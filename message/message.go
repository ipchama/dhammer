package message

import (
	"github.com/google/gopacket"
	"net"
)

type Message struct {
	RemoteAddress net.IP
	RemotePort    int
	Packet        gopacket.Packet
}
