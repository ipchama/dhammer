package generator

import (
	"encoding/base64"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/stats"
	"math/rand"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type GeneratorV4 struct {
	options       *config.Options
	iface         *net.Interface
	addLog        func(string) bool
	addError      func(error) bool
	sendPayload   func([]byte) bool
	addStat       func(stats.StatValue) bool
	finishChannel chan struct{}
	doneChannel   chan struct{}
	rpsChannel    chan int
}

func NewV4(o *config.Options, iface *net.Interface, logFunc func(string) bool, errFunc func(error) bool, payloadFunc func([]byte) bool, statFunc func(stats.StatValue) bool) *GeneratorV4 {

	g := GeneratorV4{
		options:       o,
		iface:         iface,
		addLog:        logFunc,
		addError:      errFunc,
		sendPayload:   payloadFunc,
		addStat:       statFunc,
		finishChannel: make(chan struct{}, 1),
		doneChannel:   make(chan struct{}),
		rpsChannel:    make(chan int, 1),
	}

	return &g
}

func (g *GeneratorV4) Init() error {
	return nil
}

func (g *GeneratorV4) DeInit() error {
	return nil
}

func (g *GeneratorV4) Stop() error {
	g.finishChannel <- struct{}{}
	_, _ = <-g.doneChannel
	return nil
}

func (g *GeneratorV4) Update(attr interface{}, val interface{}) error {

	if a, ok := attr.(string); ok {
		if a == "rps" {
			if v, ok := val.(string); ok {
				if vI, err := strconv.Atoi(v); err != nil {
					return err
				} else {
					g.rpsChannel <- vI
				}
			}
		}
	}

	return nil
}

func (g *GeneratorV4) Run() {

	macs := g.generateMacList()
	nS := rand.NewSource(time.Now().Unix())
	nRand := rand.New(nS)

	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	outDhcpLayer := &layers.DHCPv4{
		Operation:    layers.DHCPOpRequest,
		HardwareType: layers.LinkTypeEthernet,
		HardwareLen:  6,
		//HardwareOpts // Used by relay agents
		Flags: 0x8000, // Broadcast
	}

	if !*g.options.DhcpBroadcast {
		outDhcpLayer.Flags = 0x0
	}

	baseOptionCount := 2
	additionalOptionCount := g.options.AdditionalDhcpOptions.Len()

	outDhcpLayer.Options = make(layers.DHCPOptions, baseOptionCount+additionalOptionCount+1) // +1 for DHCPOptEnd

	outDhcpLayer.Options[0] = layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(layers.DHCPMsgTypeDiscover)})
	outDhcpLayer.Options[1] = layers.NewDHCPOption(layers.DHCPOptParamsRequest, []byte{byte(0x01), byte(0x28), byte(0x03), byte(0x0f), byte(0x06)})

	// Add in any additional DHCP options that were passed in the CLI
	for i := 0; i < additionalOptionCount; i++ {

		optionValCombo := strings.Split(g.options.AdditionalDhcpOptions[i], ":")

		aOption, err := strconv.Atoi(optionValCombo[0])
		if err != nil {
			g.addError(err)
			continue
		} else if aOption > 255 {
			g.addLog("DHCP option codes greater than 255 are not supported. Skipping " + optionValCombo[0])
			continue
		}

		aValue, err := base64.StdEncoding.DecodeString(optionValCombo[1])

		if err != nil {
			g.addError(err)
			continue
		}

		// Finish the DHCP options.
		outDhcpLayer.Options[baseOptionCount+i] = layers.NewDHCPOption(layers.DHCPOpt(aOption), aValue)
	}

	outDhcpLayer.Options[baseOptionCount+additionalOptionCount] = layers.NewDHCPOption(layers.DHCPOptEnd, []byte{})

	ethernetLayer := &layers.Ethernet{
		DstMAC:       layers.EthernetBroadcast,
		SrcMAC:       g.iface.HardwareAddr,
		EthernetType: layers.EthernetTypeIPv4,
		Length:       0,
	}

	if !*g.options.EthernetBroadcast {
		ethernetLayer.DstMAC = g.options.GatewayMAC
	}

	ipLayer := &layers.IPv4{
		Version:  4, // IPv4
		TTL:      64,
		Protocol: 17, // UDP
		SrcIP:    net.IPv4(0, 0, 0, 0),
		DstIP:    net.IPv4(255, 255, 255, 255),
	}

	if g.options.DhcpRelay {
		ipLayer.SrcIP = g.options.RelaySourceIP
		ipLayer.DstIP = g.options.RelayTargetServerIP

		ethernetLayer.SrcMAC = g.iface.HardwareAddr
		ethernetLayer.DstMAC = g.options.GatewayMAC

		outDhcpLayer.RelayAgentIP = g.options.RelayGatewayIP
	}

	udpLayer := &layers.UDP{
		SrcPort: layers.UDPPort(68),
		DstPort: layers.UDPPort(*g.options.TargetPort),
	}

	i := 0 // Increment later

	sent := 0

	start := time.Now()
	time.Sleep(1)

	mRps := *g.options.RequestsPerSecond

	var t time.Time
	var elapsed float64
	var rps int

	g.addLog("Finished generating MACs and preparing packet headers.")

	for *g.options.MaxLifetime == 0 || int(elapsed) <= *g.options.MaxLifetime {

		select {
		case _, _ = <-g.finishChannel:
			close(g.doneChannel)
			return
		default:
		}

		select {
		case mRps, _ = <-g.rpsChannel:
			sent = 0
			start = time.Now()
			time.Sleep(1)
		default:
		}

		t = time.Now()
		elapsed = t.Sub(start).Seconds()
		rps = int(float64(sent) / elapsed)

		if rps >= mRps {
			runtime.Gosched()
			continue
		}

		outDhcpLayer.Xid = nRand.Uint32()
		outDhcpLayer.ClientHWAddr = macs[i]

		//ethernetLayer.SrcMAC = macs[i]

		udpLayer.SetNetworkLayerForChecksum(ipLayer)

		buf := gopacket.NewSerializeBuffer()
		gopacket.SerializeLayers(buf, opts,
			ethernetLayer,
			ipLayer,
			udpLayer,
			outDhcpLayer,
		)

		if g.sendPayload(buf.Bytes()) {
			g.addStat(stats.DiscoverSentStat)
		}

		sent++

		if i++; i > len(macs)-1 {
			i = 0
		}
	}

}

func (g *GeneratorV4) generateMacList() []net.HardwareAddr {
	nS := rand.NewSource(time.Now().Unix())
	nRand := rand.New(nS)

	macs := make([]net.HardwareAddr, 0)
	for i := 0; i < *g.options.MacCount; i++ {
		if mac, err := net.ParseMAC(fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", nRand.Intn(256), nRand.Intn(256), nRand.Intn(256), nRand.Intn(256), nRand.Intn(256), nRand.Intn(256))); err == nil {
			macs = append(macs, mac)
		} else {
			g.addError(err)
		}
	}
	return macs
}
