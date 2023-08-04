package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	temporalgolibs "github.com/arunsworld/temporal-demo/temporal-golibs"
	"go.temporal.io/sdk/worker"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	c, err := temporalgolibs.NewClient(ctx, "default")
	if err != nil {
		return err
	}
	defer c.Close()

	w := worker.New(c, "abbank", worker.Options{})
	w.RegisterActivity(Withdraw)
	w.RegisterActivity(Deposit)

	return w.Run(worker.InterruptCh())
}
