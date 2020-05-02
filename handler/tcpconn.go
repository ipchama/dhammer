package handler

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/message"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/ipchama/dhammer/stats"
	"github.com/vishvananda/netlink"
	"net"
	"time"
)

type LeaseTcpConn struct {
	Packet   gopacket.Packet
	LinkAddr *netlink.Addr
	Acquired time.Time
	HwAddr   net.HardwareAddr
}

type HandlerTcpConn struct {
	options      *config.TcpConnOptions
	socketeer    *socketeer.RawSocketeer
	iface        *net.Interface
	link         netlink.Link
	acquiredIPs  map[string]*LeaseTcpConn
	addLog       func(string) bool
	addError     func(error) bool
	sendPayload  func([]byte) bool
	addStat      func(stats.StatValue) bool
	inputChannel chan message.Message
	doneChannel  chan struct{}
}

func init() {
	if err := AddHandler("tcpconn", NewTcpConn); err != nil {
		panic(err)
	}
}

func NewTcpConn(hip HandlerInitParams) Handler {

	h := HandlerTcpConn{
		options:      hip.options.(*config.TcpConnOptions),
		socketeer:    hip.socketeer,
		iface:        hip.socketeer.IfInfo,
		acquiredIPs:  make(map[string]*LeaseTcpConn),
		addLog:       hip.logFunc,
		addError:     hip.errFunc,
		sendPayload:  hip.socketeer.AddPayload,
		addStat:      hip.statFunc,
		inputChannel: make(chan message.Message, 10000),
		doneChannel:  make(chan struct{}),
	}

	return &h
}

func (h *HandlerTcpConn) ReceiveMessage(msg message.Message) bool {

	select {
	case h.inputChannel <- msg:
		return true
	default:
	}

	return false

}

func (h *HandlerTcpConn) Init() error {

	var err error = nil

	h.link, err = netlink.LinkByName("lo")

	return err
}

func (h *HandlerTcpConn) DeInit() error {

	return nil
}

func (h *HandlerTcpConn) Stop() error {
	close(h.inputChannel)
	<-h.doneChannel
	return nil
}

func (h *HandlerTcpConn) Run() {

	var msg message.Message
	var tcpReply *layers.TCP

	//socketeerOptions := h.socketeer.Options()

	ethernetLayer := &layers.Ethernet{
		DstMAC:       layers.EthernetBroadcast,
		SrcMAC:       h.iface.HardwareAddr,
		EthernetType: layers.EthernetTypeIPv4,
		Length:       0,
	}

	ipLayer := &layers.IPv4{
		Version:  4, // IPv4
		TTL:      64,
		Protocol: 6, // UDP
		SrcIP:    net.IPv4(0, 0, 0, 0),
		DstIP:    net.IPv4(255, 255, 255, 255),
	}

	tcpLayer := &layers.TCP{
		SrcPort: layers.TCPPort(0), // TODO: Generate a randomized port list and set this later in the loop
		DstPort: layers.TCPPort(0), // TODO: Set to one of the target port IPs.
	}

	goPacketSerializeOpts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	for msg = range h.inputChannel {

		if msg.Packet.Layer(layers.LayerTypeTCP) == nil {
			continue
		}

		tcpReply = msg.Packet.Layer(layers.LayerTypeTCP).(*layers.TCP)

		var replyOptions [16]layers.TCPOption

		for _, option := range tcpReply.Options { // Assuming that we'll expand on usage of options in the reply later and just doing this now.
			replyOptions[option.OptionType] = option
		}

		//h.addLog(fmt.Sprintf("[REPLY] %v %v %v %v %v", dhcpReply.Options[0].String(), dhcpReply.YourClientIP.String(), string(dhcpReply.ServerName), dhcpReply.ClientIP.String(), dhcpReply.ClientHWAddr))
		// TODO: Switch on the bools here https://github.com/google/gopacket/blob/1d829e51f0c85294eeedb06477a6d369fb5be0ea/layers/tcp.go#L26

		if tcpReply.FIN {

			h.addStat(stats.OfferReceivedStat)
			// TODO switch on Handshake option
			if h.options.Handshake == 1 {

				buf := gopacket.NewSerializeBuffer()

				tcpLayer.SetNetworkLayerForChecksum(ipLayer)

				gopacket.SerializeLayers(buf, goPacketSerializeOpts,
					ethernetLayer,
					ipLayer,
					tcpLayer,
					// TODO:  Do I want to play with random data here?
				)

				if h.sendPayload(buf.Bytes()) {
					// The stat added needs to dependon on the HANDSHAKE/COMM step used.
					h.addStat(stats.DeclineSentStat)
				}
			}
		}
	}

	h.doneChannel <- struct{}{}
}
