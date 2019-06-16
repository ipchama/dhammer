package stats

import (
	"encoding/json"
	//"fmt"
	"github.com/ipchama/dhammer/config"
	"sync"
	"time"
)

type StatValue int

const (
	DiscoverSentStat = iota
	InfoSentStat
	RequestSentStat
	DeclineSentStat
	ReleaseSentStat

	OfferReceivedStat
	AckReceivedStat
	NakReceivedStat
)

const StatsTypeMax int = 8

type Stat struct {
	Name                string  `json:"stat_name"`
	Value               int     `json:"stat_value"`
	PreviousTickerValue int     `json:"stat_previous_ticker_value"`
	RatePerSecond       float64 `json:"stat_rate_per_second"`
}

type StatsV4 struct {
	options *config.Options

	countersMux *sync.RWMutex
	counters    [StatsTypeMax]Stat

	addLog   func(string) bool
	addError func(error) bool

	statChannel chan StatValue
	doneChannel chan struct{}
}

func NewV4(o *config.Options, logFunc func(string) bool, errFunc func(error) bool) *StatsV4 {
	s := StatsV4{
		options:     o,
		addLog:      logFunc,
		addError:    errFunc,
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

	return nil
}

func (s *StatsV4) DeInit() error {
	return nil
}

func (s *StatsV4) Run() {

	var wg sync.WaitGroup

	wg.Add(1)

	stopTicker := make(chan struct{})

	ticker := time.NewTicker(time.Duration(*s.options.StatsRate) * time.Second)
	go func() {
		for {
			select {
			case <-stopTicker:
				ticker.Stop()
				wg.Done()
				return
			case <-ticker.C:
			}

			s.calculateStats()
			//s.addLog("\n[STATS]" + s.String())
		}
	}()

	for sv, ok := <-s.statChannel; ok; sv, ok = <-s.statChannel {
		s.countersMux.Lock()
		s.counters[sv].Value++
		s.countersMux.Unlock()
	}

	stopTicker <- struct{}{}
	wg.Wait()

	close(s.doneChannel)
}

func (s *StatsV4) calculateStats() error {

	var StatsTickerRate float64 = float64(*s.options.StatsRate)

	s.countersMux.Lock()
	for i := 0; i < StatsTypeMax; i++ {
		s.counters[i].RatePerSecond = float64((s.counters[i].Value - s.counters[i].PreviousTickerValue)) / StatsTickerRate
		s.counters[i].PreviousTickerValue = s.counters[i].Value
	}
	s.countersMux.Unlock()

	return nil
}

func (s *StatsV4) String() string {

	s.countersMux.RLock()
	defer s.countersMux.RUnlock()

	if json, err := json.MarshalIndent(s.counters, "", "  "); err != nil {
		s.addError(err)
		return ""
	} else {
		return string(json)
	}
}

func (s *StatsV4) Stop() error {
	close(s.statChannel)
	_, _ = <-s.doneChannel

	return nil
}
