package util

import (
	"os"
	"os/signal"
	"syscall"
)

type Closer interface {
	Close()
}

func WaitCloseSignals(closer Closer) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	<-signals
	closer.Close()
}
