package generator

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/ipchama/dhammer/stats"
	"math/rand"
	"net"
	"runtime"
	"time"
)

type GeneratorTcpConn struct {
	options       *config.TcpConnOptions
	socketeer     *socketeer.RawSocketeer
	iface         *net.Interface
	addLog        func(string) bool
	addError      func(error) bool
	sendPayload   func([]byte) bool
	addStat       func(stats.StatValue) bool
	finishChannel chan struct{}
	doneChannel   chan struct{}
	rpsChannel    chan int
}

func init() {
	if err := AddGenerator("tcpconn", NewTcpConn); err != nil {
		panic(err)
	}
}

func NewTcpConn(gip GeneratorInitParams) Generator {

	g := GeneratorTcpConn{
		options:       gip.options.(*config.TcpConnOptions),
		socketeer:     gip.socketeer,
		iface:         gip.socketeer.IfInfo,
		addLog:        gip.logFunc,
		addError:      gip.errFunc,
		sendPayload:   gip.socketeer.AddPayload,
		addStat:       gip.statFunc,
		finishChannel: make(chan struct{}, 1),
		doneChannel:   make(chan struct{}),
		rpsChannel:    make(chan int, 1),
	}

	return &g
}

func (g *GeneratorTcpConn) Init() error {
	return nil
}

func (g *GeneratorTcpConn) DeInit() error {
	return nil
}

func (g *GeneratorTcpConn) Stop() error {
	g.finishChannel <- struct{}{}
	_, _ = <-g.doneChannel
	return nil
}

func (g *GeneratorTcpConn) Update(details interface{}) error {

	if d, ok := details.(map[string]interface{}); ok {
		if v, ok := d["rps"].(float64); ok {
			g.rpsChannel <- int(v)
			return nil
		}
	}

	return fmt.Errorf("Update request failed.  Data was %v", details)
}

func (g *GeneratorTcpConn) Run() {

	targetPorts := generatePortList(g.options.TargetPortRangeStart, g.options.TargetPortRangeEnd)
	sourcePorts := generatePortList(5000, 60000)

	//nS := rand.NewSource(time.Now().Unix())
	//nRand := rand.New(nS)

	socketeerOptions := g.socketeer.Options()

	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	ethernetLayer := &layers.Ethernet{
		DstMAC:       socketeerOptions.GatewayMAC,
		SrcMAC:       g.iface.HardwareAddr,
		EthernetType: layers.EthernetTypeIPv4,
		Length:       0,
	}

	ipLayer := &layers.IPv4{
		Version:  4, // IPv4
		TTL:      64,
		Protocol: 6,                          // TCP
		SrcIP:    net.IPv4(192, 168, 1, 143), // TODO: Set to one of the spoof IPs.
		DstIP:    g.options.TargetServerIP,
	}

	tcpLayer := &layers.TCP{}

	iSport := 0 // Increment later
	iDport := 0 // Increment later

	sent := 0

	start := time.Now()
	time.Sleep(1)

	mRps := g.options.RequestsPerSecond

	var t time.Time
	var elapsed float64
	var rps int

	nS := rand.NewSource(time.Now().Unix())
	nRand := rand.New(nS)
	var unsolicitedPerms int

	g.addLog("Finished preparing packet headers.")

	for g.options.MaxLifetime == 0 || int(elapsed) <= g.options.MaxLifetime {
		select {
		case _, _ = <-g.finishChannel:
			close(g.doneChannel)
			return
		default:
		}

		select {
		case mRps, _ = <-g.rpsChannel:
			sent = 0
			start = time.Now()
			time.Sleep(1)
		default:
		}

		if mRps > 0 {
			t = time.Now()
			elapsed = t.Sub(start).Seconds()
			rps = int(float64(sent) / elapsed)

			if rps >= mRps {
				runtime.Gosched()
				continue
			}
		}

		tcpLayer.SetNetworkLayerForChecksum(ipLayer) // Do I need to set this on every interation?  I think this can be set once outside the loop.

		tcpLayer.SrcPort = layers.TCPPort(sourcePorts[iSport])
		tcpLayer.DstPort = layers.TCPPort(targetPorts[iDport])

		if g.options.Handshake > 0 {
			tcpLayer.SYN = true
			if g.options.RequestCongestionManagement {
				tcpLayer.ECE = true
				tcpLayer.CWR = true
			}
		} else {
			// Pregenerating a random list of the possible combinations and then doing a look up is almost an order of magnitude faster,
			// but it's 12 ns vs 1 ns, and both are so fast that they get lost in the time it takes to push packets out.
			// Even though there are only 63 possible combinations of flags (since we exclude the 0/empty case), the order in which they'll emerge
			// is harder to identify if we just call rand rather than pregenerate, and it's also way more readable in this case.
			unsolicitedPerms = nRand.Intn(64) + 1
			tcpLayer.SYN = g.options.UnsolicitedSynAck && (unsolicitedPerms&1 == 1)
			tcpLayer.ACK = (g.options.UnsolicitedSynAck && (unsolicitedPerms&1 == 1)) || (g.options.UnsolicitedAck && (unsolicitedPerms&2 == 2))

			tcpLayer.RST = g.options.UnsolicitedReset && (unsolicitedPerms&4 == 4)
			tcpLayer.FIN = g.options.UnsolicitedFin && (unsolicitedPerms&8 == 8)
			tcpLayer.URG = g.options.UnsolicitedUrgent && (unsolicitedPerms&16 == 16)
			tcpLayer.PSH = g.options.UnsolicitedPush && (unsolicitedPerms&32 == 32)
		}

		buf := gopacket.NewSerializeBuffer()
		gopacket.SerializeLayers(buf, opts,
			ethernetLayer,
			ipLayer,
			tcpLayer,
		)

		// TODO: This and the actual payload will depend on options chosen
		if g.sendPayload(buf.Bytes()) {
			g.addStat(stats.TcpSynSentStat)
		}

		sent++

		if iSport++; iSport > len(sourcePorts)-1 { // TODO: Rotate through target port, sourc port, and spoof IP.  Set up a compare-and-swap to randomize the lists every few seconds.
			iSport = 0
		}

		if iDport++; iDport > len(targetPorts)-1 { // TODO: Rotate through target port, sourc port, and spoof IP.  Set up a compare-and-swap to randomize the lists every few seconds.
			iDport = 0
		}
	}

}

func (g *GeneratorTcpConn) loadSpoofList() []net.IP {
	return nil
}

func generatePortList(s, e int) []int {

	if e == 0 {
		p := make([]int, 1, 1)
		p[0] = s
		return p
	}

	nS := rand.NewSource(time.Now().Unix())
	nRand := rand.New(nS)

	portList := make([]int, e-s, e-s)

	for i := 0; i < e-s; i++ {
		portList[i] = nRand.Intn(e-s) + s
	}

	return portList
}