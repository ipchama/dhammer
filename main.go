package main

import (
	"github.com/ipchama/dhammer/cmd"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	osSigChann := make(chan os.Signal)
	signal.Notify(osSigChann, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		_ = <-osSigChann
		cmd.Stop()
	}()

	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
