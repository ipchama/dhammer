package config

import (
	"net"
)

//test
type SocketeerOptions struct {
	InterfaceName   string
	GatewayMAC      net.HardwareAddr
	PromiscuousMode bool
}
