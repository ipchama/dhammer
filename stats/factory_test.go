package stats_test

import (
	"github.com/ipchama/dhammer/stats"
	"testing"
)

type TestHammerConfig struct {
	hType string
}

func (t *TestHammerConfig) HammerType() string {
	return t.hType
}

type TestStats struct {
}

func (t *TestStats) Init() error {
	return nil
}

func (t *TestStats) AddStat(s stats.StatValue) bool {
	return true
}

func (t *TestStats) Run() {
}

func (t *TestStats) String() string {
	return ""
}

func (t *TestStats) Stop() error {
	return nil
}

func (t *TestStats) DeInit() error {
	return nil
}

func TestNew(t *testing.T) {

	o := &TestHammerConfig{
		hType: "__TEST__",
	}

	if _, err := stats.New(o, func(string) bool { return true }, func(error) bool { return true }); err == nil {
		t.Errorf("Stats factory did not return error for unknown type.")
	}

	if err := stats.AddStatter(o.hType, func(h stats.StatsInitParams) stats.Stats { return &TestStats{} }); err != nil {
		t.Errorf("Stats factory failed to add new type.")
	}

	if err := stats.AddStatter(o.hType, func(h stats.StatsInitParams) stats.Stats { return &TestStats{} }); err == nil {
		t.Errorf("Stats factory allowed duplicate type.")
	}

	if _, err := stats.New(o, func(string) bool { return true }, func(error) bool { return true }); err != nil {
		t.Errorf("Stats factory failed to return known type.")
	}

}
