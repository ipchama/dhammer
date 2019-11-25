package config

import (
	"net"
)

type SocketeerOptions struct {
	InterfaceName   *string
	GatewayMAC      net.HardwareAddr
	PromiscuousMode *bool
}
