package stats

import (
	"errors"
	"github.com/ipchama/dhammer/config"
)

type Stat struct {
	Name                string  `json:"stat_name"`
	Value               int     `json:"stat_value"`
	PreviousTickerValue int     `json:"stat_previous_ticker_value"`
	RatePerSecond       float64 `json:"stat_rate_per_second"`
}

type StatValue int

type Stats interface {
	AddStat(s StatValue) bool
	Init() error
	Run()
	String() string
	Stop() error
	DeInit() error
}

type StatsInitParams struct {
	options *config.Options
	logFunc func(string) bool
	errFunc func(error) bool
}

var Statters map[string]func(StatsInitParams) Stats = make(map[string]func(StatsInitParams) Stats)

func New(o *config.Options, logFunc func(string) bool, errFunc func(error) bool) (error, Stats) {
	sip := StatsInitParams{
		options: o,
		logFunc: logFunc,
		errFunc: errFunc,
	}

	sf, ok := Statters[*o.HammerType]

	if !ok {
		return errors.New("Statters - Hammer type not found: " + *o.HammerType), nil
	}

	return nil, sf(sip)
}
