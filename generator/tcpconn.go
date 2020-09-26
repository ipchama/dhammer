package generator

import (
	"bufio"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/ipchama/dhammer/stats"
	"math/rand"
	"net"
	"os"
	"runtime"
	"strings"
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

	spoofIps []net.IP
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

// Init should probably also be where we handle the iptables changes (dropping RST in output) if g.options.AddDropRules is true

func (g *GeneratorTcpConn) Init() error {

	var err error

	g.spoofIps, err = loadIpsFromFile(g.options.SpoofSourcesFile)

	if err != nil {
		return err
	}

	if len(g.spoofIps) == 0 {
		err = fmt.Errorf("empty IP list passed to tcpconn generator after reading %s", g.options.SpoofSourcesFile)
	}

	return err
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
	sourcePorts := generatePortList(10000, 60000)

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
		Protocol: 6, // TCP
		DstIP:    g.options.TargetServerIP,
	}

	tcpLayer := &layers.TCP{}

	if g.options.Handshake > 0 {

		tcpLayer.SYN = true
		tcpLayer.URG = g.options.UseUrgent
		tcpLayer.PSH = g.options.UsePush

		tcpLayer.ECE = g.options.RequestCongestionManagement
		tcpLayer.CWR = g.options.RequestCongestionManagement
	}

	iSport := 0 // Increment later
	iDport := 0 // Increment later
	sIp := 0

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

		tcpLayer.SetNetworkLayerForChecksum(ipLayer) // Do I need to set this on every iteration?  I think this can be set once outside the loop.

		tcpLayer.SrcPort = layers.TCPPort(sourcePorts[iSport])
		tcpLayer.DstPort = layers.TCPPort(targetPorts[iDport])
		ipLayer.SrcIP = g.spoofIps[sIp]

		if g.options.Handshake == 0 {
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

		if g.sendPayload(buf.Bytes()) {
			g.addStat(stats.TcpSentStat)
			g.addStat(stats.TcpHandshakeSynSentStat)
			g.addStat(stats.TcpConnAttemptStat)
		}

		sent++

		if iSport++; iSport >= len(sourcePorts) { // TODO: Set up a compare-and-swap to randomize the lists every few seconds.
			iSport = 0
		}

		if iDport++; iDport >= len(targetPorts) { // TODO: Set up a compare-and-swap to randomize the lists every few seconds.
			iDport = 0
		}

		if sIp++; sIp >= len(g.spoofIps) {
			sIp = 0
		}
	}

}

// Move this junk in the config package and just import or something...
func loadIpsFromFile(f string) ([]net.IP, error) {
	list := make([]net.IP, 1, 1)

	file, err := os.Open(f)

	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		ip := net.ParseIP(strings.TrimSpace(scanner.Text()))

		if ip != nil {
			list = append(list, ip)
		}
	}

	return list, file.Close()
}

func generatePortList(start, end int) []int {

	if end == 0 {
		p := make([]int, 1, 1)
		p[0] = start
		return p
	}

	nS := rand.NewSource(time.Now().Unix())
	nRand := rand.New(nS)

	portList := make([]int, end-start, end-start)

	for i := 0; i < end-start; i++ {
		portList[i] = nRand.Intn(end-start) + start
	}

	return portList
}
