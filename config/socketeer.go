package config

import (
	"golang.org/x/sys/unix"
	"net"
)

type SocketeerOptions struct {
	InterfaceName   string
	GatewayMAC      net.HardwareAddr
	PromiscuousMode bool
	EbpfFilter      *unix.SockFprog
}
