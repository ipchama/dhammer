package generator

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/ipchama/dhammer/stats"
	//	"math/rand"
	"net"
	"runtime"
	"time"
)

type GeneratorTcpConn struct {
	options       *config.TcpConnOptions
	socketeer     *socketeer.RawSocketeer
	iface         *net.Interface
	addLog        func(string) bool
	addError      func(error) bool
	sendPayload   func([]byte) bool
	addStat       func(stats.StatValue) bool
	finishChannel chan struct{}
	doneChannel   chan struct{}
	rpsChannel    chan int
}

func init() {
	if err := AddGenerator("tcpconn", NewTcpConn); err != nil {
		panic(err)
	}
}

func NewTcpConn(gip GeneratorInitParams) Generator {

	g := GeneratorTcpConn{
		options:       gip.options.(*config.TcpConnOptions),
		socketeer:     gip.socketeer,
		iface:         gip.socketeer.IfInfo,
		addLog:        gip.logFunc,
		addError:      gip.errFunc,
		sendPayload:   gip.socketeer.AddPayload,
		addStat:       gip.statFunc,
		finishChannel: make(chan struct{}, 1),
		doneChannel:   make(chan struct{}),
		rpsChannel:    make(chan int, 1),
	}

	return &g
}

func (g *GeneratorTcpConn) Init() error {
	return nil
}

func (g *GeneratorTcpConn) DeInit() error {
	return nil
}

func (g *GeneratorTcpConn) Stop() error {
	g.finishChannel <- struct{}{}
	_, _ = <-g.doneChannel
	return nil
}

func (g *GeneratorTcpConn) Update(details interface{}) error {

	if d, ok := details.(map[string]interface{}); ok {
		if v, ok := d["rps"].(float64); ok {
			g.rpsChannel <- int(v)
			return nil
		}
	}

	return fmt.Errorf("Update request failed.  Data was %v", details)
}

func (g *GeneratorTcpConn) Run() {

	// targetPorts := generatePortList(PASS PORT RANGE OPTIONS)
	// sourcePorts := generatePortList()

	//nS := rand.NewSource(time.Now().Unix())
	//nRand := rand.New(nS)

	socketeerOptions := g.socketeer.Options()

	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	ethernetLayer := &layers.Ethernet{
		DstMAC:       socketeerOptions.GatewayMAC,
		SrcMAC:       g.iface.HardwareAddr,
		EthernetType: layers.EthernetTypeIPv4,
		Length:       0,
	}

	ipLayer := &layers.IPv4{
		Version:  4, // IPv4
		TTL:      64,
		Protocol: 6,                            // TCP
		SrcIP:    net.IPv4(0, 0, 0, 0),         // TODO: Set to one of the spoof IPs.
		DstIP:    net.IPv4(255, 255, 255, 255), // TODO: Set to the target IP from options
	}

	tcpLayer := &layers.TCP{
		SrcPort: layers.TCPPort(0), // TODO: Generate a randomized port list and set this later in the loop
		DstPort: layers.TCPPort(0), // TODO: Set to one of the target port IPs.
	}

	//i := 0 // Increment later

	sent := 0

	start := time.Now()
	time.Sleep(1)

	mRps := g.options.RequestsPerSecond

	var t time.Time
	var elapsed float64
	var rps int

	g.addLog("Finished generating MACs and preparing packet headers.")

	for g.options.MaxLifetime == 0 || int(elapsed) <= g.options.MaxLifetime {

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

		tcpLayer.SetNetworkLayerForChecksum(ipLayer)

		buf := gopacket.NewSerializeBuffer()
		gopacket.SerializeLayers(buf, opts,
			ethernetLayer,
			ipLayer,
			tcpLayer,
		)

		// TODO: This and the actual payload will depend on options chosen
		if g.sendPayload(buf.Bytes()) {
			g.addStat(stats.TcpSynSentStat)
		}

		sent++

		/*		if i++; i > len(macs)-1 { // TODO: Rotate through target port, sourc port, and target IP.  Set up a compare-and-swap to randomize the lists every few seconds.
					i = 0
				}
		*/
	}

}

func (g *GeneratorTcpConn) loadSpoofList() []net.IP {
	return nil
}
