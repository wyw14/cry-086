package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/wyw14/cry-086/internal/config"
)

func RunProcess() int {
	settings, err := config.Load()
	if err != nil {
		slog.Error("invalid crane safety configuration", "error", err)
		return 2
	}
	processContext, cancelProcess := context.WithCancel(context.Background())
	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(interrupts)
		cancelProcess()
	}()
	go func() {
		select {
		case received := <-interrupts:
			slog.Info("shutdown requested", "signal", received.String())
			cancelProcess()
		case <-processContext.Done():
		}
	}()

	application, err := New(processContext, settings)
	if err != nil {
		slog.Error("crane safety platform did not initialize", "error", err)
		return 1
	}
	defer application.Close()
	if err := application.Run(processContext); err != nil {
		slog.Error("crane safety platform stopped unexpectedly", "error", err)
		return 1
	}
	return 0
}
