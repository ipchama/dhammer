package generator_test

import (
	"github.com/ipchama/dhammer/generator"
	"github.com/ipchama/dhammer/stats"
	"testing"
)

type TestHammerConfig struct {
	hType string
}

func (t *TestHammerConfig) HammerType() string {
	return t.hType
}

type TestGenerator struct {
}

func (t *TestGenerator) Init() error {
	return nil
}

func (t *TestGenerator) Update(i interface{}) error {
	return nil
}

func (t *TestGenerator) Run() {
}

func (t *TestGenerator) Stop() error {
	return nil
}

func (t *TestGenerator) DeInit() error {
	return nil
}

func TestNew(t *testing.T) {

	o := &TestHammerConfig{
		hType: "__TEST__",
	}

	if _, err := generator.New(nil, o, func(string) bool { return true }, func(error) bool { return true }, func(stats.StatValue) bool { return true }); err == nil {
		t.Errorf("Generator factory did not return error for unknown type.")
	}

	if err := generator.AddGenerator(o.hType, func(g generator.GeneratorInitParams) generator.Generator { return &TestGenerator{} }); err != nil {
		t.Errorf("Generator factory failed to add new type.")
	}

	if err := generator.AddGenerator(o.hType, func(g generator.GeneratorInitParams) generator.Generator { return &TestGenerator{} }); err == nil {
		t.Errorf("Generator factory allowed duplicate type.")
	}

	if _, err := generator.New(nil, o, func(string) bool { return true }, func(error) bool { return true }, func(stats.StatValue) bool { return true }); err != nil {
		t.Errorf("Generator factory failed to return known type.")
	}

}
