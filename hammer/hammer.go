package hammer

import (
	"github.com/ipchama/dhammer/config"
	"github.com/ipchama/dhammer/generator"
	"github.com/ipchama/dhammer/handler"
	"github.com/ipchama/dhammer/message"
	"github.com/ipchama/dhammer/socketeer"
	"github.com/ipchama/dhammer/stats"
	"log"
	"sync"
)

/*
	TODO:
		Review MAC generation.
		Factory for generator/handler/stats based on future ipv option.
		Info/decline options.
		Options and functionality for "binding" IPs and ARPing.
		Option to automatically select gateway MAC from default route gateway.
*/

type Handler interface {
	ReceiveMessage(m message.Message) bool
	Init() error
	Run()
	Stop() error
	DeInit() error
}

type Generator interface {
	Init() error
	Run()
	Stop() error
	DeInit() error
}

type Stats interface {
	AddStat(s stats.StatValue) bool
	Init() error
	Run()
	String() string
	Stop() error
	DeInit() error
}

type Hammer struct {
	options      *config.Options
	logChannel   chan string
	errorChannel chan error

	handler   Handler
	generator Generator
	stats     Stats
	socketeer *socketeer.RawSocketeer
}

func New(o *config.Options) *Hammer {

	h := Hammer{
		options:      o,
		logChannel:   make(chan string, 1000),
		errorChannel: make(chan error, 1000),
	}

	return &h
}

func (h *Hammer) Init() error {

	var err error

	log.SetFlags(log.LstdFlags | log.LUTC)

	h.stats = stats.NewV4(h.options, h.addLog, h.addError)
	if err = h.stats.Init(); err != nil {
		return err
	}

	h.socketeer = socketeer.NewRawSocketeer(h.options, h.addLog, h.addError)
	if err = h.socketeer.Init(); err != nil {
		return err
	}

	h.handler = handler.NewV4(h.options, h.socketeer.IfInfo, h.addLog, h.addError, h.socketeer.AddPayload, h.stats.AddStat)
	if err := h.handler.Init(); err != nil {
		return err
	}

	h.socketeer.SetReceiver(h.handler.ReceiveMessage)

	h.generator = generator.NewV4(h.options, h.socketeer.IfInfo, h.addLog, h.addError, h.socketeer.AddPayload, h.stats.AddStat)
	if err = h.generator.Init(); err != nil {
		return err
	}

	return nil
}

func (h *Hammer) deInit() {
	var err error

	if err = h.socketeer.DeInit(); err != nil {
		h.addError(err)
	}

	if err = h.handler.DeInit(); err != nil {
		h.addError(err)
	}

	if err = h.generator.DeInit(); err != nil {
		h.addError(err)
	}

	if err = h.stats.DeInit(); err != nil {
		h.addError(err)
	}
}
func (h *Hammer) Run() error {

	var wg sync.WaitGroup

	log.Print("INFO: Starting error channel reader.")
	wg.Add(1)
	go func() {
		var err error

		for ok := true; ok; {
			if err, ok = <-h.errorChannel; ok {
				log.Print("ERROR: " + err.Error())
			}
		}
		wg.Done()
		log.Print("INFO: Stopped error channel reader.")
	}()

	log.Print("INFO: Starting stats.")
	wg.Add(1)
	go func() {
		h.stats.Run()
		wg.Done()
		log.Print("INFO: Stopped stats.")
	}()

	log.Print("INFO: Starting writer.")
	wg.Add(1)
	go func() {
		h.socketeer.RunWriter()
		wg.Done()
		log.Print("INFO: Stopped writer.")
	}()

	log.Print("INFO: Starting handler.")
	wg.Add(1)
	go func() {
		h.handler.Run()
		wg.Done()
		log.Print("INFO: Stopped handler.")
	}()

	log.Print("INFO: Starting listener.")
	wg.Add(1)
	go func() {
		h.socketeer.RunListener()
		wg.Done()
		log.Print("INFO: Stopped listener.")
	}()

	log.Print("INFO: Starting log channel reader.")
	wg.Add(1)
	go func() {
		var msg string

		for ok := true; ok; {
			if msg, ok = <-h.logChannel; ok {
				log.Print("INFO: " + msg)
			}
		}
		wg.Done()
		log.Print("INFO: Stopped log channel reader.")
	}()

	log.Print("INFO: Starting generator.")
	wg.Add(1)
	go func() {
		h.generator.Run()
		log.Print("INFO: Stopped generator.")
		log.Print("INFO: Going to stop everything else...")
		h.stop()
		wg.Done()
	}()

	wg.Wait()

	return nil
}

func (h *Hammer) addError(e error) bool {
	select {
	case h.errorChannel <- e:
		return true
	default:
	}
	return false
}

func (h *Hammer) addLog(s string) bool {
	select {
	case h.logChannel <- s:
		return true
	default:
	}

	return false
}

func (h *Hammer) Stop() {
	// All "stop" calls should block.
	// This will make sure no new payloads go TO the writer FROM the generator.
	h.generator.Stop()
}

func (h *Hammer) stop() {
	var err error

	// All "stop" calls should block.
	if err = h.socketeer.StopListener(); err != nil { // This will make sure no new messages are sent TO the handler.
		h.addError(err)
	}

	if err = h.handler.Stop(); err != nil { // This will make sure no new payloads go TO the writer FROM the handler.
		h.addError(err)
	}

	if err = h.socketeer.StopWriter(); err != nil { // This will stop any writing to the underlying socket and stop any potential error or message logging.
		h.addError(err)
	}

	if err = h.stats.Stop(); err != nil { // This should be the last place that could send errors or logs.
		h.addError(err)
	}

	/*
	 Err doesn't get returned here because it uses h.addError directly.
	 There may eventually be multiple points in it where err != nil, but I want all the DeInit steps to complete, so I won't return err.
	 Instead, I addError so it gets reported and I keep going.
	*/
	h.deInit()

	close(h.errorChannel)
	close(h.logChannel)
}
