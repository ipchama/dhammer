package cmd

import (
	"errors"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/hammer"
	"github.com/ipchama/dhammer/message"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
	"net"
	"sync"
	"time"
)

func prepareCmd(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Bool("handshake", true, "Attempt full handshakes")
	cmd.Flags().Bool("info", false, "Send DHCPINFO packets. This requires a full handshake.")
	cmd.Flags().Bool("dhcp-broadcast", true, "Set the broadcast bit.")
	cmd.Flags().Bool("ethernet-broadcast", true, "Use ethernet broadcasting.")
	cmd.Flags().Bool("release", false, "Release leases after acquiring them.")

	cmd.Flags().Bool("decline", false, "Decline offers.")

	cmd.Flags().Int("rps", 0, "Max number of packets per second. 0 == unlimited.")
	cmd.Flags().Int("maxlife", 0, "How long to run. 0 == forever")
	cmd.Flags().Int("mac-count", 1, "Total number of MAC addresses to use. If the 'mac' option is used, mac-count - number of mac will be used to pad with additional pre-generated MAC addresses.")
	cmd.Flags().StringArray("mac", []string{}, "Optionally specified MAC address to be used for requesting leases. Can be used multiple times.")

	cmd.Flags().Int("stats-rate", 5, "How frequently to update stat calculations. (seconds).")

	cmd.Flags().Bool("arp", false, "Respond to arp requests for assigned IPs.")
	cmd.Flags().Bool("arp-fake-mac", false, "Respond to ARP requests with the generated MAC used to originally obtain the lease.  You might want to set arp_ignore to 1 or 3 for the interface sending packets. For full functionality, the --promisc option is needed.")
	cmd.Flags().Bool("bind", false, "Bind acquired IPs to the loopback device.  Combined with the --arp option, this will result in fully functioning IPs.")

	cmd.Flags().String("relay-source-ip", "", "Source IP for relayed requests.  relay-source-ip AND relay-target-server-ip must be set for relay mode.")
	cmd.Flags().String("relay-gateway-ip", "", "Gateway (giaddr) IP for relayed requests.  If not set, it will default to the relay source IP.")
	cmd.Flags().String("relay-target-server-ip", "", "Target/Destination IP for relayed requests.  relay-source-ip AND relay-target-server-ip must be set for relay mode.")
	cmd.Flags().Int("target-port", 67, "Target port for special cases.  Rarely would you want to use this.")

	cmd.Flags().StringArray("dhcp-option", []string{}, "Additional DHCP option to send out in the discover. Can be used multiple times. Format: <option num>:<RFC4648-base64-encoded-value>")

	cmd.Flags().String("interface", "eth0", "Interface name for listening and sending.")
	cmd.Flags().String("gateway-mac", "auto", "MAC of the gateway.")
	cmd.Flags().Bool("promisc", false, "Turn on promiscuous mode for the listening interface.")

	cmd.Flags().String("api-address", "", "IP for the API server to listen on.")
	cmd.Flags().Int("api-port", 8080, "Port for the API server to listen on.")

	return cmd
}

func getVal(i interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}

	return i
}

func arp(n string, l netlink.Link, i net.IP) (net.HardwareAddr, error) {

	srcAddr := getVal(netlink.AddrList(l, netlink.FAMILY_V4)).([]netlink.Addr)[0]

	s := socketeer.NewRawSocketeer(&config.SocketeerOptions{InterfaceName: n}, func(s string) bool { return true }, func(e error) bool { panic(e); return true })

	if err := s.Init(); err != nil {
		return nil, err
	}

	arpReplies := make(chan net.HardwareAddr)

	s.SetReceiver(func(msg message.Message) bool {
		if msg.Packet.Layer(layers.LayerTypeARP) != nil {
			arpMsg := msg.Packet.Layer(layers.LayerTypeARP).(*layers.ARP)
			if arpMsg.Operation == layers.ARPReply {
				if net.IP(arpMsg.SourceProtAddress).String() == i.String() {
					arpReplies <- arpMsg.SourceHwAddress
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

	gopacket.SerializeLayers(buf, goPacketSerializeOpts,
		ethernetLayer,
		arpLayer,
	)

	s.AddPayload(buf.Bytes())

	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		close(arpReplies)
	}()

	gwMac, ok := <-arpReplies

	timer.Stop()

	s.StopListener()
	s.StopWriter()
	wg.Wait()

	if !ok {
		return nil, errors.New("Failed to get ARP response for default gateway probe during init.")
	}

	return gwMac, nil
}

func init() {

	rootCmd.AddCommand(prepareCmd(&cobra.Command{
		Use:   "dhcpv4",
		Short: "Run a dhcpv4 load test.",
		Long:  `Run a dhcpv4 load test.`,
		Run: func(cmd *cobra.Command, args []string) {

			options := &config.DhcpV4Options{}
			socketeerOptions := &config.SocketeerOptions{}

			var err error

			options.Handshake = getVal(cmd.Flags().GetBool("handshake")).(bool)

			options.DhcpInfo = getVal(cmd.Flags().GetBool("info")).(bool)
			options.DhcpBroadcast = getVal(cmd.Flags().GetBool("dhcp-broadcast")).(bool)
			options.EthernetBroadcast = getVal(cmd.Flags().GetBool("ethernet-broadcast")).(bool)
			options.DhcpRelease = getVal(cmd.Flags().GetBool("release")).(bool)
			options.DhcpDecline = getVal(cmd.Flags().GetBool("decline")).(bool)

			options.RequestsPerSecond = getVal(cmd.Flags().GetInt("rps")).(int)
			options.MaxLifetime = getVal(cmd.Flags().GetInt("maxlife")).(int)
			options.MacCount = getVal(cmd.Flags().GetInt("mac-count")).(int)
			options.AdditionalMacs = getVal(cmd.Flags().GetStringArray("mac")).([]string)

			if options.MacCount <= 0 && len(options.AdditionalMacs) == 0 {
				panic("At least one of mac-count or mac options must be used.")
			}

			options.StatsRate = getVal(cmd.Flags().GetInt("stats-rate")).(int)

			options.Arp = getVal(cmd.Flags().GetBool("arp")).(bool)
			options.ArpFakeMAC = getVal(cmd.Flags().GetBool("arp-fake-mac")).(bool)
			options.Bind = getVal(cmd.Flags().GetBool("bind")).(bool)

			relayIP := getVal(cmd.Flags().GetString("relay-source-ip")).(string)
			relayGatewayIP := getVal(cmd.Flags().GetString("relay-gateway-ip")).(string)

			targetServerIP := getVal(cmd.Flags().GetString("relay-target-server-ip")).(string)
			options.TargetPort = getVal(cmd.Flags().GetInt("target-port")).(int)
			options.AdditionalDhcpOptions = getVal(cmd.Flags().GetStringArray("dhcp-option")).([]string)

			socketeerOptions.InterfaceName = getVal(cmd.Flags().GetString("interface")).(string)
			gatewayMAC := getVal(cmd.Flags().GetString("gateway-mac")).(string)
			socketeerOptions.PromiscuousMode = getVal(cmd.Flags().GetBool("promisc")).(bool)

			ApiAddress := getVal(cmd.Flags().GetString("api-address")).(string)
			ApiPort := getVal(cmd.Flags().GetInt("api-port")).(int)

			options.RelaySourceIP = net.ParseIP(relayIP)
			options.RelayGatewayIP = net.ParseIP(relayGatewayIP)
			options.RelayTargetServerIP = net.ParseIP(targetServerIP)

			if options.RelayGatewayIP == nil {
				options.RelayGatewayIP = options.RelaySourceIP
			}

			if options.RelaySourceIP != nil && options.RelayTargetServerIP != nil {
				options.DhcpRelay = true
			}

			// netlink and arp to get the gw IP and then ARP to get the MAC
			if gatewayMAC == "auto" {
				link := getVal(netlink.LinkByName(socketeerOptions.InterfaceName)).(netlink.Link)
				routes := getVal(netlink.RouteList(link, netlink.FAMILY_V4)).([]netlink.Route)

				for _, r := range routes {
					if r.Dst == nil && r.Src == nil { // We've found the default route.
						socketeerOptions.GatewayMAC = getVal(arp(socketeerOptions.InterfaceName, link, r.Gw)).(net.HardwareAddr)
						break
					}
				}
			} else {
				socketeerOptions.GatewayMAC, err = net.ParseMAC(gatewayMAC)
				if err != nil {
					panic(err)
				}
			}

			if options.StatsRate <= 0 {
				options.StatsRate = 5
			}

			gHammer = hammer.New(socketeerOptions, options)

			err = gHammer.Init(ApiAddress, ApiPort)

			if err != nil {
				panic(err)
			}
			err = gHammer.Run()

			if err != nil {
				panic(err)
			}
		},
	}))

}
