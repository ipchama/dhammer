package stats

import (
	"encoding/json"
	//"fmt"
	"github.com/ipchama/dhammer/config"
	"sync"
	"time"
)

const (
	DiscoverSentStat = iota
	InfoSentStat
	RequestSentStat
	DeclineSentStat
	ReleaseSentStat

	OfferReceivedStat
	AckReceivedStat
	NakReceivedStat

	ArpReplySentStat
	ArpRequestReceivedStat
)

type StatsV4 struct {
	options *config.DhcpV4Options

	countersMux *sync.RWMutex
	counters    [10]Stat

	addLog   func(string) bool
	addError func(error) bool

	statChannel chan StatValue
	doneChannel chan struct{}
}

func init() {
	if err := AddStatter("dhcpv4", NewStatsDhcpV4); err != nil {
		panic(err)
	}
}

func NewStatsDhcpV4(sip StatsInitParams) Stats {
	s := StatsV4{
		options:     sip.options.(*config.DhcpV4Options),
		addLog:      sip.logFunc,
		addError:    sip.errFunc,
		statChannel: make(chan StatValue, 10000),
		doneChannel: make(chan struct{}, 1),
		countersMux: &sync.RWMutex{},
	}

	return &s
}

func (s *StatsV4) AddStat(sv StatValue) bool {
	select {
	case s.statChannel <- sv:
		return true
	default:
	}
	return false
}

func (s *StatsV4) Init() error {

	s.counters[0].Name = "DiscoverSent"
	s.counters[1].Name = "InfoSent"
	s.counters[2].Name = "RequestSent"
	s.counters[3].Name = "DeclineSent"
	s.counters[4].Name = "ReleaseSent"
	s.counters[5].Name = "OfferReceived"
	s.counters[6].Name = "AckReceived"
	s.counters[7].Name = "NakReceived"

	s.counters[8].Name = "ArpReplySent"
	s.counters[9].Name = "ArpRequestReceived"

	return nil
}

func (s *StatsV4) DeInit() error {
	return nil
}

func (s *StatsV4) Run() {

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

func (s *StatsV4) calculateStats() error {

	var StatsTickerRate float64 = float64(s.options.StatsRate)

	s.countersMux.Lock()
	for i := 0; i < len(s.counters); i++ {
		s.counters[i].RatePerSecond = float64((s.counters[i].Value - s.counters[i].PreviousTickerValue)) / StatsTickerRate
		s.counters[i].PreviousTickerValue = s.counters[i].Value
	}
	s.countersMux.Unlock()

	return nil
}

func (s *StatsV4) String() string {

	s.countersMux.RLock()
	defer s.countersMux.RUnlock()

	if jsonData, err := json.MarshalIndent(s.counters, "", "  "); err != nil {
		s.addError(err)
		return ""
	} else {
		return string(jsonData)
	}
}

func (s *StatsV4) Stop() error {
	close(s.statChannel)
	_, _ = <-s.doneChannel

	return nil
}
