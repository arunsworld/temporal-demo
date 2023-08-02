package main

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	c, err := client.Dial(client.Options{
		Namespace: "default",
	})
	if err != nil {
		return err
	}
	defer c.Close()

	w := worker.New(c, "clearing-house", worker.Options{})

	w.RegisterWorkflow(MoneyTransfer)

	return w.Run(worker.InterruptCh())
}
