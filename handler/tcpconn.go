package handler

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/message"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/ipchama/dhammer/stats"
	"net"
)

type HandlerTcpConn struct {
	options      *config.TcpConnOptions
	socketeer    *socketeer.RawSocketeer
	iface        *net.Interface
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

	return nil
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
	var ipReply *layers.IPv4

	socketeerOptions := h.socketeer.Options()

	ethernetLayer := &layers.Ethernet{
		DstMAC:       socketeerOptions.GatewayMAC,
		SrcMAC:       h.iface.HardwareAddr,
		EthernetType: layers.EthernetTypeIPv4,
		Length:       0,
	}

	ipLayer := &layers.IPv4{
		Version:  4, // IPv4
		TTL:      64,
		Protocol: 6, // TCP
		SrcIP:    net.IPv4(0, 0, 0, 0),
		DstIP:    net.IPv4(255, 255, 255, 255),
	}

	tcpLayer := &layers.TCP{}

	if h.options.Handshake == 2 { // Full-handshake
		tcpLayer.URG = h.options.UseUrgent
		tcpLayer.PSH = h.options.UsePush

		tcpLayer.ECE = h.options.RequestCongestionManagement
		tcpLayer.CWR = h.options.RequestCongestionManagement
	}

	goPacketSerializeOpts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	for msg = range h.inputChannel {

		/* if h.options.Ipv6 && msg.Packet.Layer(layers.LayerTypeIPv6) == nil {
			continue
		} else */if !h.options.Ipv6 && msg.Packet.Layer(layers.LayerTypeIPv4) == nil {
			continue
		}

		if msg.Packet.Layer(layers.LayerTypeTCP) == nil {
			continue
		}

		tcpReply = msg.Packet.Layer(layers.LayerTypeTCP).(*layers.TCP)

		if tcpReply.SYN && tcpReply.ACK { // A new connection is starting

			h.addStat(stats.TcpHandshakeSynAckRecvStat)
			h.addStat(stats.TcpRecvStat)

			if h.options.Handshake == 2 { // We only care about full handshake.  If == 0 then we are just spewing garbage. If == 1 then we're making half open connections.

				ipReply = msg.Packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)

				// Need to start tracking connections here.  Probably a good idea to just go with with full 4-tuple tracking from the start.
				// Probably also need to start thinking about a separate go-routine to spew data packets for live connections.

				buf := gopacket.NewSerializeBuffer()

				ipLayer.SrcIP = ipReply.DstIP
				ipLayer.DstIP = ipReply.SrcIP

				tcpLayer.SetNetworkLayerForChecksum(ipLayer)

				tcpLayer.ACK = true
				tcpLayer.Ack = tcpReply.Seq + 1

				tcpLayer.DstPort = tcpReply.SrcPort
				tcpLayer.SrcPort = tcpReply.DstPort
				tcpLayer.Seq = 1
				tcpLayer.Window = tcpReply.Window // TODO: Actually set this to something valid.

				// If we can statically set this, do so outside the loop, or at least build the parts that we can outside the loop.
				tcpLayer.Options = []layers.TCPOption{
					layers.TCPOption{layers.TCPOptionKindSACKPermitted, 0, []byte{}},
				}

				/*
					for o := range tcpReply.Options {
						if o.OptionType == layers.TCPOptionKindSACKPermitted {

						}
					}
				*/

				gopacket.SerializeLayers(buf, goPacketSerializeOpts,
					ethernetLayer,
					ipLayer,
					tcpLayer,
					// TODO:  Do I want to play with random data here?
				)

				if h.sendPayload(buf.Bytes()) {
					h.addStat(stats.TcpSentStat)
					h.addStat(stats.TcpHandshakeAckSentStat)
					h.addStat(stats.TcpConnSuccessStat)
				}
			}
		}
	}

	h.doneChannel <- struct{}{}
}
