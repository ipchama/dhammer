package cmd

import (
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/hammer"
	"github.com/spf13/cobra"
	"net"
	"os"
	"os/signal"
	"syscall"
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
	cmd.Flags().Int("mac-count", 1, "Number of unique MAC addresses to pre-generate.")

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
	cmd.Flags().String("gateway-mac", "de:ad:be:ef:f0:0d", "MAC of the gateway.")
	cmd.Flags().Bool("promisc", false, "Turn on promiscuous mode for the listening interface.")

	cmd.Flags().String("api-address", "", "IP for the API server to listen on.")
	cmd.Flags().Int("api-port", 8080, "Port for the API server to listen on.")

	return cmd
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

			options.Handshake, err = cmd.Flags().GetBool("handshake")
			options.DhcpInfo, err = cmd.Flags().GetBool("info")
			options.DhcpBroadcast, err = cmd.Flags().GetBool("dhcp-broadcast")
			options.EthernetBroadcast, err = cmd.Flags().GetBool("ethernet-broadcast")
			options.DhcpRelease, err = cmd.Flags().GetBool("release")

			options.DhcpDecline, err = cmd.Flags().GetBool("decline")

			options.RequestsPerSecond, err = cmd.Flags().GetInt("rps")
			options.MaxLifetime, err = cmd.Flags().GetInt("maxlife")
			options.MacCount, err = cmd.Flags().GetInt("mac-count")

			options.StatsRate, err = cmd.Flags().GetInt("stats-rate")

			options.Arp, err = cmd.Flags().GetBool("arp")
			options.ArpFakeMAC, err = cmd.Flags().GetBool("arp-fake-mac")
			options.Bind, err = cmd.Flags().GetBool("bind")

			relayIP, err := cmd.Flags().GetString("relay-source-ip")
			relayGatewayIP, err := cmd.Flags().GetString("relay-gateway-ip")
			targetServerIP, err := cmd.Flags().GetString("relay-target-server-ip")
			options.TargetPort, err = cmd.Flags().GetInt("target-port")

			options.AdditionalDhcpOptions, err = cmd.Flags().GetStringArray("dhcp-option")

			socketeerOptions.InterfaceName, err = cmd.Flags().GetString("interface")
			gatewayMAC, err := cmd.Flags().GetString("gateway-mac")
			socketeerOptions.PromiscuousMode, err = cmd.Flags().GetBool("promisc")

			ApiAddress, err := cmd.Flags().GetString("api-address")
			ApiPort, err := cmd.Flags().GetInt("api-port")

			options.RelaySourceIP = net.ParseIP(relayIP)
			options.RelayGatewayIP = net.ParseIP(relayGatewayIP)
			options.RelayTargetServerIP = net.ParseIP(targetServerIP)

			if options.RelayGatewayIP == nil {
				options.RelayGatewayIP = options.RelaySourceIP
			}

			if options.RelaySourceIP != nil && options.RelayTargetServerIP != nil {
				options.DhcpRelay = true
			}

			socketeerOptions.GatewayMAC, err = net.ParseMAC(gatewayMAC)

			if options.StatsRate <= 0 {
				options.StatsRate = 5
			}

			if err != nil {
				panic(err)
			}

			Hammer := hammer.New(socketeerOptions, options)

			err = Hammer.Init(ApiAddress, ApiPort)

			if err != nil {
				panic(err)
			}

			osSigChann := make(chan os.Signal)
			signal.Notify(osSigChann, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				_ = <-osSigChann
				Hammer.Stop()
			}()

			err = Hammer.Run()

			if err != nil {
				panic(err)
			}
		},
	}))

}
