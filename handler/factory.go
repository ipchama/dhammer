package handler

import (
	"errors"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/message"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/ipchama/dhammer/stats"
)

type Handler interface {
	ReceiveMessage(m message.Message) bool
	Init() error
	Run()
	Stop() error
	DeInit() error
}

type HandlerInitParams struct {
	options   config.HammerConfig
	socketeer *socketeer.RawSocketeer
	logFunc   func(string) bool
	errFunc   func(error) bool
	statFunc  func(stats.StatValue) bool
}

var handlers map[string]func(HandlerInitParams) Handler = make(map[string]func(HandlerInitParams) Handler)

func AddHandler(s string, f func(HandlerInitParams) Handler) error {
	if _, found := handlers[s]; found {
		return errors.New("Handler type already exists: " + s)
	}

	handlers[s] = f

	return nil
}

func New(s *socketeer.RawSocketeer, o config.HammerConfig, logFunc func(string) bool, errFunc func(error) bool, statFunc func(stats.StatValue) bool) (Handler, error) {
	hip := HandlerInitParams{
		options:   o,
		socketeer: s,
		logFunc:   logFunc,
		errFunc:   errFunc,
		statFunc:  statFunc,
	}

	hf, ok := handlers[o.HammerType()]

	if !ok {
		return nil, errors.New("Handlers - Hammer type not found: " + o.HammerType())
	}

	return hf(hip), nil
}
