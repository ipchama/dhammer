package generator

import (
	"errors"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/ipchama/dhammer/stats"
)

type Generator interface {
	Init() error
	Update(interface{}) error
	Run()
	Stop() error
	DeInit() error
}

type GeneratorInitParams struct {
	socketeer *socketeer.RawSocketeer
	options   config.HammerConfig
	logFunc   func(string) bool
	errFunc   func(error) bool
	statFunc  func(stats.StatValue) bool
}

var generators map[string]func(GeneratorInitParams) Generator = make(map[string]func(GeneratorInitParams) Generator)

func AddGenerator(s string, f func(GeneratorInitParams) Generator) error {
	if _, found := generators[s]; found {
		return errors.New("Generator type already exists: " + s)
	}

	generators[s] = f

	return nil
}

func New(s *socketeer.RawSocketeer, o config.HammerConfig, logFunc func(string) bool, errFunc func(error) bool, statFunc func(stats.StatValue) bool) (Generator, error) {

	gip := GeneratorInitParams{
		socketeer: s,
		options:   o,
		logFunc:   logFunc,
		errFunc:   errFunc,
		statFunc:  statFunc,
	}

	gf, ok := generators[o.HammerType()]

	if !ok {
		return nil, errors.New("Generators - Hammer type not found: " + o.HammerType())
	}

	return gf(gip), nil
}
