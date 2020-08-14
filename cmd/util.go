package cmd

import (
	"errors"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/message"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/vishvananda/netlink"

	"net"
	"sync"
	"time"
)

func getVal(i interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}

	return i
}

func getGatewayV4(i string) (net.HardwareAddr, error) {
	link := getVal(netlink.LinkByName(i)).(netlink.Link)
	routes := getVal(netlink.RouteList(link, netlink.FAMILY_V4)).([]netlink.Route)

	for _, r := range routes {
		if r.Dst == nil && r.Src == nil { // We've found the default route.
			return getVal(arp(i, link, r.Gw)).(net.HardwareAddr), nil
			break
		}
	}
	return nil, errors.New("Failed to get gateway details via ARP.")
}

func arp(n string, l netlink.Link, i net.IP) (net.HardwareAddr, error) {

	srcAddr := getVal(netlink.AddrList(l, netlink.FAMILY_V4)).([]netlink.Addr)[0]

	s := socketeer.NewRawSocketeer(&config.SocketeerOptions{InterfaceName: n}, func(s string) bool { return true }, func(e error) bool { println(e); return true })

	if err := s.Init(); err != nil {
		return nil, err
	}

	arpReplies := make(chan net.HardwareAddr)

	s.SetReceiver(func(msg message.Message) bool {
		if msg.Packet.Layer(layers.LayerTypeARP) != nil {
			arpMsg := msg.Packet.Layer(layers.LayerTypeARP).(*layers.ARP)
			if arpMsg.Operation == layers.ARPReply {
				if net.IP(arpMsg.SourceProtAddress).String() == i.String() {
					select {
					case arpReplies <- arpMsg.SourceHwAddress:
					default:
					}
				}
			}
		}

		return true
	})

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		s.RunWriter()
		wg.Done()

	}()
	wg.Add(1)
	go func() {
		s.RunListener()
		wg.Done()
	}()

	goPacketSerializeOpts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	ethernetLayer := &layers.Ethernet{
		DstMAC:       layers.EthernetBroadcast,
		SrcMAC:       s.IfInfo.HardwareAddr,
		EthernetType: layers.EthernetTypeARP,
		Length:       0,
	}

	arpLayer := &layers.ARP{
		Operation:         layers.ARPRequest,
		DstHwAddress:      layers.EthernetBroadcast, // Broadcast
		DstProtAddress:    i,
		HwAddressSize:     6,
		AddrType:          1, // Netlink type: ethernet
		ProtAddressSize:   4,
		Protocol:          0x800, // Ipv4
		SourceHwAddress:   s.IfInfo.HardwareAddr,
		SourceProtAddress: srcAddr.IP,
	}
	buf := gopacket.NewSerializeBuffer()

	if err := gopacket.SerializeLayers(buf, goPacketSerializeOpts,
		ethernetLayer,
		arpLayer,
	); err != nil {
		panic(err)
	}

	s.AddPayload(buf.Bytes())

	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		close(arpReplies)
	}()

	gwMac, ok := <-arpReplies

	timer.Stop()

	/*	Many things could happen here that we really don't need to care about and that could cause "false-positive" failures.
		Either we got an arp response or we didn't.  The !ok test below is all that should matter.  We just want output for debugging purposes.
		Still, revisit becasue this could still be handled better..
	*/
	if err := s.StopListener(); err != nil {
		println(err)
	}

	if err := s.StopWriter(); err != nil {
		println(err)
	}

	wg.Wait()

	if !ok {
		return nil, errors.New("failed to get ARP response for default gateway probe during init")
	}

	return gwMac, nil
}
