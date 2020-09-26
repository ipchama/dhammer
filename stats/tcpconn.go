package stats

import (
	"encoding/json"
	//"fmt"

	"github.com/ipchama/dhammer/config"
	"sync"
	"time"
)

// The stats should probably speak more to connection and bandwidth/data-transmission and not
// Individual packet properties, particularily because these aren't separate packets: a single packet could have a combination of them.
const (
	TcpSentStat = iota
	TcpRecvStat

	TcpHandshakeSynSentStat
	TcpHandshakeSynAckRecvStat
	TcpHandshakeAckSentStat

	TcpConnAttemptStat
	TcpConnSuccessStat
)

type StatsTcpConn struct {
	options *config.TcpConnOptions

	countersMux *sync.RWMutex
	counters    [7]Stat

	addLog   func(string) bool
	addError func(error) bool

	statChannel chan StatValue
	doneChannel chan struct{}
}

func init() {
	if err := AddStatter("tcpconn", NewStatsTcpConn); err != nil {
		panic(err)
	}
}

func NewStatsTcpConn(sip StatsInitParams) Stats {
	s := StatsTcpConn{
		options:     sip.options.(*config.TcpConnOptions),
		addLog:      sip.logFunc,
		addError:    sip.errFunc,
		statChannel: make(chan StatValue, 10000),
		doneChannel: make(chan struct{}, 1),
		countersMux: &sync.RWMutex{},
	}

	return &s
}

func (s *StatsTcpConn) AddStat(sv StatValue) bool {
	select {
	case s.statChannel <- sv:
		return true
	default:
	}
	return false
}

func (s *StatsTcpConn) Init() error {

	s.counters[0].Name = "PacketSent"
	s.counters[1].Name = "PacketReceived"
	s.counters[2].Name = "HandshakeSynSent"
	s.counters[3].Name = "HandshakeSynAckReceived"
	s.counters[4].Name = "HandshakeAckSent"
	s.counters[5].Name = "ConnectionAttempted"
	s.counters[6].Name = "ConnectionSucceeded"

	return nil
}

func (s *StatsTcpConn) DeInit() error {
	return nil
}

func (s *StatsTcpConn) Run() {

	var wg sync.WaitGroup

	wg.Add(1)

	stopTicker := make(chan struct{})

	ticker := time.NewTicker(time.Duration(s.options.StatsRate) * time.Second)
	go func() {
		for {
			select {
			case <-stopTicker:
				ticker.Stop()
				wg.Done()
				return
			case <-ticker.C:
			}

			if err := s.calculateStats(); err != nil {
				s.addError(err)
			}
		}
	}()

	for sv := range s.statChannel {
		s.countersMux.Lock()
		s.counters[sv].Value++
		s.countersMux.Unlock()
	}

	stopTicker <- struct{}{}
	wg.Wait()

	close(s.doneChannel)
}

func (s *StatsTcpConn) calculateStats() error {

	var StatsTickerRate float64 = float64(s.options.StatsRate)

	s.countersMux.Lock()
	for i := 0; i < len(s.counters); i++ {
		s.counters[i].RatePerSecond = float64((s.counters[i].Value - s.counters[i].PreviousTickerValue)) / StatsTickerRate
		s.counters[i].PreviousTickerValue = s.counters[i].Value
	}
	s.countersMux.Unlock()

	return nil
}

func (s *StatsTcpConn) String() string {

	s.countersMux.RLock()
	defer s.countersMux.RUnlock()

	if jsonData, err := json.MarshalIndent(s.counters, "", "  "); err != nil {
		s.addError(err)
		return ""
	} else {
		return string(jsonData)
	}
}

func (s *StatsTcpConn) Stop() error {
	close(s.statChannel)
	_, _ = <-s.doneChannel

	return nil
}
