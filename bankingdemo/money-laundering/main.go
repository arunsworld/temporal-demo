package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/arunsworld/nursery"
	temporalgolibs "github.com/arunsworld/temporal-demo/temporal-golibs"
	"go.temporal.io/sdk/activity"
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

	srv := newService(c)

	w := worker.New(c, "money-laundering", worker.Options{})
	w.RegisterActivityWithOptions(srv.temporalActivity, activity.RegisterOptions{
		Name: "MoneyLaunderingCheck",
	})

	server := &http.Server{Addr: "localhost:9999"}

	return nursery.RunConcurrently(
		func(context.Context, chan error) {
			w.Run(worker.InterruptCh())
		},
		func(context.Context, chan error) {
			log.Println("serving on http://localhost:9999/")
			if err := server.ListenAndServe(); err != nil {
				log.Printf("unable to serve on port 9999")
			}
		},
		func(context.Context, chan error) {
			<-ctx.Done()
			server.Close()
		},
	)
}
