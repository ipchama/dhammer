package config

import (
	"net"
)

type DhcpV4Options struct {
	Handshake         bool
	DhcpInfo          bool
	EthernetBroadcast bool
	DhcpBroadcast     bool
	DhcpRelease       bool
	DhcpDecline       bool

	Arp        bool
	ArpFakeMAC bool
	Bind       bool

	DhcpRelay           bool
	RelaySourceIP       net.IP
	RelayGatewayIP      net.IP
	RelayTargetServerIP net.IP
	TargetPort          int

	AdditionalDhcpOptions []string

	RequestsPerSecond int
	MaxLifetime       int

	MacCount      int
	SpecifiedMacs []string
	MacSeed       int64

	StatsRate int
}

func (o *DhcpV4Options) HammerType() string {
	return "dhcpv4"
}
