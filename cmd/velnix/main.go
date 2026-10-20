package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"velnix/internal/platform"
	"velnix/internal/transport/cli"
)

var version = "0.1.0-dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	command := cli.CLI{Out: os.Stdout, Err: os.Stderr, Detector: platform.Detector{Root: "/"}, Version: version}
	code := command.Run(ctx, os.Args[1:])
	cancel()
	os.Exit(code)
}
