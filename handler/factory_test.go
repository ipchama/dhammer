package handler_test

import (
	"github.com/ipchama/dhammer/handler"
	"github.com/ipchama/dhammer/message"
	"github.com/ipchama/dhammer/stats"
	"testing"
)

type TestHammerConfig struct {
	hType string
}

func (t *TestHammerConfig) HammerType() string {
	return t.hType
}

type TestHandler struct {
}

func (t *TestHandler) Init() error {
	return nil
}

func (t *TestHandler) ReceiveMessage(m message.Message) bool {
	return true
}

func (t *TestHandler) Run() {
}

func (t *TestHandler) Stop() error {
	return nil
}

func (t *TestHandler) DeInit() error {
	return nil
}

func TestNew(t *testing.T) {

	o := &TestHammerConfig{
		hType: "__TEST__",
	}

	if _, err := handler.New(nil, o, func(string) bool { return true }, func(error) bool { return true }, func(stats.StatValue) bool { return true }); err == nil {
		t.Errorf("Handler factory did not return error for unknown type.")
	}

	if err := handler.AddHandler(o.hType, func(h handler.HandlerInitParams) handler.Handler { return &TestHandler{} }); err != nil {
		t.Errorf("Handler factory failed to add new type.")
	}

	if err := handler.AddHandler(o.hType, func(h handler.HandlerInitParams) handler.Handler { return &TestHandler{} }); err == nil {
		t.Errorf("Handler factory allowed duplicate type.")
	}

	if _, err := handler.New(nil, o, func(string) bool { return true }, func(error) bool { return true }, func(stats.StatValue) bool { return true }); err != nil {
		t.Errorf("Handler factory failed to return known type.")
	}

}
