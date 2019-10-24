package handler

import (
	"errors"
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/message"
	"github.com/ipchama/dhammer/stats"
	"net"
)

type Handler interface {
	ReceiveMessage(m message.Message) bool
	Init() error
	Run()
	Stop() error
	DeInit() error
}

type HandlerInitParams struct {
	options     *config.Options
	iface       *net.Interface
	logFunc     func(string) bool
	errFunc     func(error) bool
	payloadFunc func([]byte) bool
	statFunc    func(stats.StatValue) bool
}

var handlers map[string]func(HandlerInitParams) Handler = make(map[string]func(HandlerInitParams) Handler)

func AddHandler(s string, f func(HandlerInitParams) Handler) error {
	if _, found := handlers[s]; found {
		return errors.New("Handler type already exists: " + s)
	}

	handlers[s] = f

	return nil
}

func New(o *config.Options, iface *net.Interface, logFunc func(string) bool, errFunc func(error) bool, payloadFunc func([]byte) bool, statFunc func(stats.StatValue) bool) (error, Handler) {
	hip := HandlerInitParams{
		options:     o,
		iface:       iface,
		logFunc:     logFunc,
		errFunc:     errFunc,
		payloadFunc: payloadFunc,
		statFunc:    statFunc,
	}

	hf, ok := handlers[*o.HammerType]

	if !ok {
		return errors.New("Handlers - Hammer type not found: " + *o.HammerType), nil
	}

	return nil, hf(hip)
}
