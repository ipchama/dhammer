package cmd

import (
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/hammer"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"net"
)

func prepareTcpCmd(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Bool("syn", true, "Send only SYN. (same as handhake=1)")
	cmd.Flags().Bool("syn-ack", true, "Send SYN-ACK")
	cmd.Flags().Bool("ack", true, "Send only acks")

	cmd.Flags().Int("handshake", 2, "Handshake stages: 1 == syn only; 2 == full handshake")

	cmd.Flags().Int("rps", 0, "Max number of packets per second. 0 == unlimited.")
	cmd.Flags().Int("maxlife", 0, "How long to run. 0 == forever")

	cmd.Flags().Int("stats-rate", 5, "How frequently to update stat calculations. (seconds).")

	cmd.Flags().String("spoof-sources-file", "", "List of source IPs.  This will only work for with SYN-only.")

	cmd.Flags().String("target-server-ip", "127.0.0.1", "Target to load test.")
	cmd.Flags().Int("target-port-range-start", 80, "Start of port range to test.")
	cmd.Flags().Int("target-port-range-end", 80, "End of port range to test.")

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
		Long:  `Run a tcp-based load test.`,
		Run: func(cmd *cobra.Command, args []string) {

			options := &config.TcpConnOptions{}
			socketeerOptions := &config.SocketeerOptions{}

			var err error

			options.Handshake = getVal(cmd.Flags().GetInt("handshake")).(int)

			options.Syn = getVal(cmd.Flags().GetBool("syn")).(bool)
			options.SynAck = getVal(cmd.Flags().GetBool("syn-ack")).(bool)
			options.Ack = getVal(cmd.Flags().GetBool("ack")).(bool)

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
