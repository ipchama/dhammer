package config

import (
	"net"
)

type TcpConnOptions struct {
	AddDropRules bool

	Handshake                   int
	RequestCongestionManagement bool
	UnsolicitedSynAck           bool
	UnsolicitedAck              bool
	UnsolicitedReset            bool
	UnsolicitedFin              bool
	UnsolicitedUrgent           bool
	UnsolicitedPush             bool

	UsePush   bool
	UseUrgent bool
	UseFin    bool
	UseReset  bool

	Ipv6 bool

	TargetServerIP       net.IP
	TargetPortRangeStart int
	TargetPortRangeEnd   int

	SpoofSourcesFile string

	RequestsPerSecond int
	MaxLifetime       int

	StatsRate int
}

func (o *TcpConnOptions) HammerType() string {
	return "tcpconn"
}
