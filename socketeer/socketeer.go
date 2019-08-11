package socketeer

import (
	"encoding/binary"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/message"
	"net"
	"runtime"
	"syscall"
)

type RawSocketeer struct {
	socketFd      int
	IfInfo        *net.Interface
	outputChannel chan []byte

	options *config.Options

	addLog   func(string) bool
	addError func(error) bool

	handleMessage func(msg message.Message) bool

	finishChannel chan struct{}
	doneChannel   chan struct{}
}

func NewRawSocketeer(o *config.Options, logFunc func(string) bool, errFunc func(error) bool) *RawSocketeer {
	s := RawSocketeer{
		options:       o,
		addLog:        logFunc,
		addError:      errFunc,
		outputChannel: make(chan []byte),
		finishChannel: make(chan struct{}, 1),
		doneChannel:   make(chan struct{}, 1),
	}

	return &s
}

func (s *RawSocketeer) SetReceiver(receiverFunc func(msg message.Message) bool) {
	s.handleMessage = receiverFunc
}

func (s *RawSocketeer) Init() error {
	var err error

	if s.socketFd, err = syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, syscall.ETH_P_ALL); err != nil {
		return err
	}

	s.IfInfo, err = net.InterfaceByName(*s.options.InterfaceName)

	if err != nil {
		return err
	}

	protocolBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(protocolBytes, syscall.ETH_P_ALL)
	protocol := binary.LittleEndian.Uint16(protocolBytes)

	var haddr [8]byte
	copy(haddr[0:7], s.IfInfo.HardwareAddr[0:7])
	addr := syscall.SockaddrLinklayer{
		Protocol: protocol,
		Ifindex:  s.IfInfo.Index,
		Halen:    uint8(len(s.IfInfo.HardwareAddr)),
		Addr:     haddr,
	}

	if err = syscall.Bind(s.socketFd, &addr); err != nil {
		return err
	}

	if *s.options.PromiscuousMode {
		if err = syscall.SetLsfPromisc(*s.options.InterfaceName, true); err != nil {
			return err
		}
	}

	return nil
}

func (s *RawSocketeer) DeInit() error {

	if *s.options.PromiscuousMode {
		if err := syscall.SetLsfPromisc(*s.options.InterfaceName, false); err != nil {
			return err
		}
	}

	if err := syscall.Close(s.socketFd); err != nil {
		return err
	}

	return nil
}

func (s *RawSocketeer) RunListener() {

	data := make([]byte, 4096)

	for {

		select {
		case _, _ = <-s.finishChannel:
			close(s.doneChannel)
			return
		default:
		}

		/*
			TODO: Should make this non-blocking.
			In reality, this socket is so raw that there's almost certainly going to be a high enough
			frequency of packets that it won't block for long, but why leave that to chance.
		*/
		read, ifrom, err := syscall.Recvfrom(s.socketFd, data, 0)
		if err != nil {
			s.addError(err)
			continue
		} else if sll := ifrom.(*syscall.SockaddrLinklayer); sll.Pkttype == syscall.PACKET_OUTGOING {
			runtime.Gosched()
			continue
		} else if read == 0 {
			runtime.Gosched()
			continue
		}

		p := gopacket.NewPacket(data[:read], layers.LayerTypeEthernet, gopacket.Lazy)

		msg := message.Message{
			Packet: p,
		}

		s.handleMessage(msg) // Such an urge to use a reference here...
	}

}

func (s *RawSocketeer) RunWriter() {

	var payload []byte

	for ok := true; ok; {
		if payload, ok = <-s.outputChannel; ok {

			_, err := syscall.Write(s.socketFd, payload)

			if err != nil {
				s.addError(err)
			}
		}
	}
}

func (s *RawSocketeer) StopListener() error {
	s.finishChannel <- struct{}{}
	_, _ = <-s.doneChannel
	return nil
}

func (s *RawSocketeer) StopWriter() error {
	close(s.outputChannel)
	return nil
}

func (s *RawSocketeer) AddPayload(payload []byte) bool {
	s.outputChannel <- payload
	return true
}
