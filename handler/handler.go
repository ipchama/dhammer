package handler

import (
	//"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/message"
	"github.com/ipchama/dhammer/stats"
	"net"
	"runtime"
)

type HandlerV4 struct {
	options      *config.Options
	iface        *net.Interface
	addLog       func(string) bool
	addError     func(error) bool
	sendPayload  func([]byte) bool
	addStat      func(stats.StatValue) bool
	inputChannel chan message.Message
	doneChannel  chan struct{}
}

func NewV4(o *config.Options, iface *net.Interface, logFunc func(string) bool, errFunc func(error) bool, payloadFunc func([]byte) bool, statFunc func(stats.StatValue) bool) *HandlerV4 {

	h := HandlerV4{
		options:      o,
		iface:        iface,
		addLog:       logFunc,
		addError:     errFunc,
		sendPayload:  payloadFunc,
		addStat:      statFunc,
		inputChannel: make(chan message.Message, 10000),
		doneChannel:  make(chan struct{}),
	}

	return &h
}

func (h *HandlerV4) ReceiveMessage(msg message.Message) bool {

	select {
	case h.inputChannel <- msg:
		return true
	default:
	}

	return false

}

func (h *HandlerV4) Init() error {
	return nil
}

func (h *HandlerV4) DeInit() error {
	return nil
}

func (h *HandlerV4) Stop() error {
	close(h.inputChannel)
	<-h.doneChannel
	return nil
}

func (h *HandlerV4) Run() {

	var msg message.Message
	var dhcpReply *layers.DHCPv4

	ethernetLayer := &layers.Ethernet{
		DstMAC:       layers.EthernetBroadcast,
		SrcMAC:       h.iface.HardwareAddr,
		EthernetType: layers.EthernetTypeIPv4,
		Length:       0,
	}

	if !*h.options.EthernetBroadcast {
		ethernetLayer.DstMAC = h.options.GatewayMAC
	}

	ipLayer := &layers.IPv4{
		Version:  4, // IPv4
		TTL:      64,
		Protocol: 17, // UDP
		SrcIP:    net.IPv4(0, 0, 0, 0),
		DstIP:    net.IPv4(255, 255, 255, 255),
	}

	udpLayer := &layers.UDP{
		SrcPort: layers.UDPPort(68),
		DstPort: layers.UDPPort(67),
	}

	outDhcpLayer := &layers.DHCPv4{
		Operation:    layers.DHCPOpRequest,
		HardwareType: layers.LinkTypeEthernet,
		HardwareLen:  6,
		Flags:        0x8000, // Broadcast
	}

	if !*h.options.DhcpBroadcast {
		outDhcpLayer.Flags = 0x0
	}

	if h.options.DhcpRelay {
		ipLayer.SrcIP = h.options.RelaySourceIP
		ipLayer.DstIP = h.options.RelayTargetServerIP

		ethernetLayer.SrcMAC = h.iface.HardwareAddr
		ethernetLayer.DstMAC = h.options.GatewayMAC

		outDhcpLayer.RelayAgentIP = h.options.RelaySourceIP
	}

	goPacketSerializeOpts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	ok := true

	for {

		if msg, ok = <-h.inputChannel; !ok {
			h.doneChannel <- struct{}{}
			return
		}

		if msg.Packet.Layer(layers.LayerTypeDHCPv4) == nil {
			runtime.Gosched()
			continue
		}

		dhcpReply = msg.Packet.Layer(layers.LayerTypeDHCPv4).(*layers.DHCPv4)

		//h.addLog(fmt.Sprintf("[REPLY] %v %v %v %v %v", dhcpReply.Options[0].String(), dhcpReply.YourClientIP.String(), string(dhcpReply.ServerName), dhcpReply.ClientIP.String(), dhcpReply.ClientHWAddr))

		// Should do as I've always done and loop through options and not assume that the first is the one we want.
		if dhcpReply.Options[0].Data[0] == (byte)(layers.DHCPMsgTypeOffer) {

			h.addStat(stats.OfferReceivedStat)

			if *h.options.Handshake {

				buf := gopacket.NewSerializeBuffer()

				outDhcpLayer.Xid = dhcpReply.Xid

				outDhcpLayer.Options = make(layers.DHCPOptions, 3)

				outDhcpLayer.Options[0] = layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(layers.DHCPMsgTypeRequest)})
				outDhcpLayer.Options[1] = layers.NewDHCPOption(layers.DHCPOptRequestIP, dhcpReply.YourClientIP)
				outDhcpLayer.Options[2] = layers.NewDHCPOption(layers.DHCPOptEnd, []byte{})

				outDhcpLayer.ClientHWAddr = dhcpReply.ClientHWAddr

				udpLayer.SetNetworkLayerForChecksum(ipLayer)

				gopacket.SerializeLayers(buf, goPacketSerializeOpts,
					ethernetLayer,
					ipLayer,
					udpLayer,
					outDhcpLayer,
				)

				if h.sendPayload(buf.Bytes()) {
					h.addStat(stats.RequestSentStat)
				}
			}
		} else if dhcpReply.Options[0].Data[0] == (byte)(layers.DHCPMsgTypeAck) {

			h.addStat(stats.AckReceivedStat)

			if *h.options.Handshake && *h.options.DhcpRelease {

				buf := gopacket.NewSerializeBuffer()

				outDhcpLayer.Xid = dhcpReply.Xid

				/* We have to unicast DHCPRELEASE - https://tools.ietf.org/html/rfc2131#section-4.4.4 */

				dhcpReplyEtherFrame := msg.Packet.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)

				/*
					The "next server" value of the DHCP reply might not actually be the server issuing the IP.
					Not seeing another sure option for grabbing the DHCP server IP aside from yanking it out of the IP header.
				*/

				dhcpReplyIpHeader := msg.Packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)

				releaseEthernetLayer := &layers.Ethernet{
					DstMAC:       dhcpReplyEtherFrame.SrcMAC,
					SrcMAC:       h.iface.HardwareAddr,
					EthernetType: layers.EthernetTypeIPv4,
					Length:       0,
				}

				releaseIpLayer := &layers.IPv4{
					Version:  4, // IPv4
					TTL:      64,
					Protocol: 17, // UDP
					SrcIP:    dhcpReply.YourClientIP,
					DstIP:    dhcpReplyIpHeader.SrcIP,
				}

				previousClientIP := outDhcpLayer.ClientIP
				outDhcpLayer.ClientIP = dhcpReply.YourClientIP

				outDhcpLayer.Options = make(layers.DHCPOptions, 3)

				outDhcpLayer.Options[0] = layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(layers.DHCPMsgTypeRelease)})
				outDhcpLayer.Options[1] = layers.NewDHCPOption(layers.DHCPOptEnd, []byte{})

				outDhcpLayer.ClientHWAddr = dhcpReply.ClientHWAddr

				udpLayer.SetNetworkLayerForChecksum(ipLayer)

				gopacket.SerializeLayers(buf, goPacketSerializeOpts,
					releaseEthernetLayer,
					releaseIpLayer,
					udpLayer,
					outDhcpLayer,
				)

				// Reset ClientIP to what it was.  It might have been an IP or it might have been 0.0.0.0, depending what options were used.
				outDhcpLayer.ClientIP = previousClientIP

				if h.sendPayload(buf.Bytes()) {
					h.addStat(stats.ReleaseSentStat)
				}
			}

		} else if dhcpReply.Options[0].Data[0] == (byte)(layers.DHCPMsgTypeNak) {
			h.addStat(stats.NakReceivedStat)
		}
	}
}
