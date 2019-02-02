package config

import (
	"fmt"
	"net"
)

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

type Options struct {
	Handshake         *bool
	DhcpInfo          *bool
	EthernetBroadcast *bool
	DhcpBroadcast     *bool
	DhcpRelease       *bool

	DhcpRelay           bool
	RelaySourceIP       net.IP
	RelayTargetServerIP net.IP

	AdditionalDhcpOptions FlagArrayString

	RequestsPerSecond *int
	MaxLifetime       *int
	MacCount          *int

	StatsRate *int

	InterfaceName *string
	GatewayMAC    net.HardwareAddr
}
