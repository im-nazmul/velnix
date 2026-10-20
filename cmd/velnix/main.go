package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"velnix/internal/platform"
	"velnix/internal/process"
	"velnix/internal/transport/cli"
)

var version = "0.1.0-dev"

func main() {
	ctx, cancel := context.WithCancelCause(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-signals:
			cancel(&process.SignalCause{Signal: sig})
		case <-ctx.Done():
		}
	}()
	command := cli.CLI{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Detector: platform.Detector{Root: "/"}, Version: version}
	code := command.Run(ctx, os.Args[1:])
	signal.Stop(signals)
	cancel(nil)
	os.Exit(code)
}
