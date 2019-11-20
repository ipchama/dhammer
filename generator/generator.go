package generator

import (
	"errors"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/stats"
	"net"
)

type Generator interface {
	Init() error
	Update(interface{}) error
	Run()
	Stop() error
	DeInit() error
}

type GeneratorInitParams struct {
	options     *config.Options
	iface       *net.Interface
	logFunc     func(string) bool
	errFunc     func(error) bool
	payloadFunc func([]byte) bool
	statFunc    func(stats.StatValue) bool
}

var generators map[string]func(GeneratorInitParams) Generator = make(map[string]func(GeneratorInitParams) Generator)

func AddGenerator(s string, f func(GeneratorInitParams) Generator) error {
	if _, found := generators[s]; found {
		return errors.New("Generator type already exists: " + s)
	}

	generators[s] = f

	return nil
}

func New(o *config.Options, iface *net.Interface, logFunc func(string) bool, errFunc func(error) bool, payloadFunc func([]byte) bool, statFunc func(stats.StatValue) bool) (error, Generator) {

	gip := GeneratorInitParams{
		options:     o,
		iface:       iface,
		logFunc:     logFunc,
		errFunc:     errFunc,
		payloadFunc: payloadFunc,
		statFunc:    statFunc,
	}

	gf, ok := generators[*o.HammerType]

	if !ok {
		return errors.New("Generators - Hammer type not found: " + *o.HammerType), nil
	}

	return nil, gf(gip)
}
