package config

import (
	"fmt"
	"net"
)

type HammerConfig interface {
	HammerType() string
}

type FlagArrayString []string

func (a *FlagArrayString) String() string {
	return fmt.Sprint([]string(*a))
}

func (a *FlagArrayString) Set(value string) error {
	*a = append(*a, value)
	return nil
}

func (a *FlagArrayString) Len() int {
	return len([]string(*a))
}

type DhcpV4Options struct {
	Handshake         *bool
	DhcpInfo          *bool
	EthernetBroadcast *bool
	DhcpBroadcast     *bool
	DhcpRelease       *bool
	DhcpDecline       *bool

	Arp        *bool
	ArpFakeMAC *bool
	Bind       *bool

	DhcpRelay           bool
	RelaySourceIP       net.IP
	RelayGatewayIP      net.IP
	RelayTargetServerIP net.IP
	TargetPort          *int

	AdditionalDhcpOptions FlagArrayString

	RequestsPerSecond *int
	MaxLifetime       *int
	MacCount          *int

	StatsRate *int

	InterfaceName   *string
	GatewayMAC      net.HardwareAddr
	PromiscuousMode *bool
}

func (o *DhcpV4Options) HammerType() string {
	return "dhcpv4"
}
