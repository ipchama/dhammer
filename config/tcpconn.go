package config

import (
	"net"
)

type TcpConnOptions struct {
	Handshake int
	Syn       bool
	SynAck    bool
	Ack       bool

	IPVersion int

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
