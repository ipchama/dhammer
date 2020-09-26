package cmd

import (
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/hammer"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"net"
)

// Wondering if we can use timing with spoofing and a little luck to created spoofed connections.

func prepareTcpCmd(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Bool("ipv6", false, "Use IPv6")

	cmd.Flags().Bool("add-rst-drop-rules", true, "NOT IMPLEMENTED YET Automatically add firewall rules to drop outgoing RST packets for target IP and port ranges.")

	cmd.Flags().Int("handshake", 2, "Handshake steps attempted.  0 == Nothing, not even a SYN;  1 == SYN; 2 == full handhake.")
	cmd.Flags().Bool("request-congestion-mgmt", false, "Pretend we are capable of managing situations of congestion and see if the otherside is as well.")
	cmd.Flags().Bool("use-push", true, "Use PSH for data with established connections.")
	cmd.Flags().Bool("use-urgent", true, "Use URG for data with established connections.")
	cmd.Flags().Bool("use-fin", true, "Use FIN for data with established connections. If using -generate-data-* options, this will be a response to the first ACK received.")
	cmd.Flags().Bool("use-reset", true, "Use RST for data with established connections. If using -generate-data-* options, this will be a response to the first ACK received.")
	cmd.Flags().Int("data-initial-delay", 0, "Delay in MICROseconds before first data packet is sent.  This might affect rps.")
	cmd.Flags().Int("generate-data-burst", 1, "Once a connection is established, generate data and send <num> packets between ACKs.")
	cmd.Flags().Bool("random-sequence-numbers", false, "Randomize sequence numbers for data packets.")
	cmd.Flags().Bool("reverse-sequence-numbers", false, "Run the sequence backward, trying to force excessive reordering.")

	cmd.Flags().Bool("unsolicited-syn-ack", true, "Send SYN-ACKs without an established connection.")
	cmd.Flags().Bool("unsolicited-ack", true, "Send ACKs without an established connection.")
	cmd.Flags().Bool("unsolicited-reset", false, "Send RSTs without an established connection.")
	cmd.Flags().Bool("unsolicited-fin", false, "Send FINs without an established connection.")
	cmd.Flags().Bool("unsolicited-urgent", false, "Send URGs without an established connection.")
	cmd.Flags().Bool("unsolicited-push", false, "Send PSHs without an established connection.")

	cmd.Flags().Int("rps", 1, "Max number of initial packets to generate per second. 0 == unlimited.")
	cmd.Flags().Int("maxlife", 0, "How long to run. 0 == forever")

	cmd.Flags().Int("stats-rate", 5, "How frequently to update stat calculations. (seconds).")

	cmd.Flags().String("spoof-sources-file", "", "List of source IPs.  This will only work for with handshake=1 and unsolicited-* options.")

	cmd.Flags().String("target-server-ip", "127.0.0.1", "Target to load test.") // Make this a list
	cmd.Flags().Int("target-port-range-start", 80, "Start of port range to test.")
	cmd.Flags().Int("target-port-range-end", 0, "End of port range to test.  0 == only test the start port.")

	cmd.Flags().String("interface", "eth0", "Interface name for listening and sending.")
	cmd.Flags().String("gateway-mac", "auto", "MAC of the gateway.")

	cmd.Flags().String("api-address", "", "IP for the API server to listen on.")
	cmd.Flags().Int("api-port", 8080, "Port for the API server to listen on.")

	return cmd
}

func init() {

	rootCmd.AddCommand(prepareTcpCmd(&cobra.Command{
		Use:   "tcpconn",
		Short: "Run a tcp-based load test.",
		Long:  `unsolicited options can be stacked with 'handshake', which will cause initial packets to be randomly selected.`,
		Run: func(cmd *cobra.Command, args []string) {

			options := &config.TcpConnOptions{}
			socketeerOptions := &config.SocketeerOptions{}

			var err error

			options.Handshake = getVal(cmd.Flags().GetInt("handshake")).(int)

			options.AddDropRules = getVal(cmd.Flags().GetInt("add-rst-drop-rules")).(bool)

			options.RequestCongestionManagement = getVal(cmd.Flags().GetBool("request-congestion-mgmt")).(bool)
			options.UsePush = getVal(cmd.Flags().GetBool("use-push")).(bool)
			options.UseUrgent = getVal(cmd.Flags().GetBool("use-urgent")).(bool)
			options.UseFin = getVal(cmd.Flags().GetBool("use-fin")).(bool)
			options.UseReset = getVal(cmd.Flags().GetBool("use-reset")).(bool)

			// If Handshake == 0, then any of the selected unsolicited options below will be randomy chosen per generated packet.
			options.UnsolicitedSynAck = getVal(cmd.Flags().GetBool("unsolicited-syn-ack")).(bool)
			options.UnsolicitedAck = getVal(cmd.Flags().GetBool("unsolicited-ack")).(bool)
			options.UnsolicitedReset = getVal(cmd.Flags().GetBool("unsolicited-reset")).(bool)
			options.UnsolicitedFin = getVal(cmd.Flags().GetBool("unsolicited-fin")).(bool)
			options.UnsolicitedUrgent = getVal(cmd.Flags().GetBool("unsolicited-urgent")).(bool)
			options.UnsolicitedPush = getVal(cmd.Flags().GetBool("unsolicited-push")).(bool)

			options.RequestsPerSecond = getVal(cmd.Flags().GetInt("rps")).(int)
			options.MaxLifetime = getVal(cmd.Flags().GetInt("maxlife")).(int)

			options.StatsRate = getVal(cmd.Flags().GetInt("stats-rate")).(int)

			options.SpoofSourcesFile = getVal(cmd.Flags().GetString("spoof-sources-file")).(string)

			targetServerIp := getVal(cmd.Flags().GetString("target-server-ip")).(string)
			options.TargetPortRangeStart = getVal(cmd.Flags().GetInt("target-port-range-start")).(int)
			options.TargetPortRangeEnd = getVal(cmd.Flags().GetInt("target-port-range-end")).(int)

			socketeerOptions.InterfaceName = getVal(cmd.Flags().GetString("interface")).(string)
			gatewayMAC := getVal(cmd.Flags().GetString("gateway-mac")).(string)

			ApiAddress := getVal(cmd.Flags().GetString("api-address")).(string)
			ApiPort := getVal(cmd.Flags().GetInt("api-port")).(int)

			options.TargetServerIP = net.ParseIP(targetServerIp)

			if options.TargetServerIP == nil {
				panic("TargetServerIP is invalid.")
			}

			// netlink and arp to get the gw IP and then ARP to get the MAC
			if gatewayMAC == "auto" {
				if socketeerOptions.GatewayMAC, err = getGatewayV4(socketeerOptions.InterfaceName); err != nil {
					panic(err)
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

			filter := [28]unix.SockFilter{{0x28, 0, 0, 0x0000000c},
				{0x15, 0, 5, 0x000086dd},
				{0x30, 0, 0, 0x00000014},
				{0x15, 6, 0, 0x00000006},
				{0x15, 0, 6, 0x0000002c},
				{0x30, 0, 0, 0x00000036},
				{0x15, 3, 4, 0x00000006},
				{0x15, 0, 3, 0x00000800},
				{0x30, 0, 0, 0x00000017},
				{0x15, 0, 1, 0x00000006},
				{0x6, 0, 0, 0x00040000},
				{0x6, 0, 0, 0x00000000}}

			socketeerOptions.EbpfFilter = &unix.SockFprog{12, &filter[0]}

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
