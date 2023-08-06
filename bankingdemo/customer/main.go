package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arunsworld/nursery"
	temporalgolibs "github.com/arunsworld/temporal-demo/temporal-golibs"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

type Request struct {
	SourceBank, DestinationBank string
	SourceAcc, DestinationAcc   string
	Amount                      float64
	Ref                         string
}

type Response struct {
	Status                         string // success, failure, refunded
	FailureReason                  string // populated for failure and refunded
	StartTime                      time.Time
	MoneyLaunderingCheckFinishTime time.Time
	WithdrawDoneTime               time.Time
	RefundDoneTime                 time.Time
	DepositDoneTime                time.Time
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	c, err := temporalgolibs.NewClient(ctx, "default")
	if err != nil {
		return err
	}
	defer c.Close()

	newService(ctx, c)

	server := &http.Server{Addr: ":9399"}

	return nursery.RunConcurrentlyWithContext(ctx,
		func(context.Context, chan error) {
			log.Println("serving on http://localhost:9399/")
			if err := server.ListenAndServe(); err != nil {
				log.Printf("unable to serve on port 9399")
			}
		},
		func(ctx context.Context, errCh chan error) {
			<-ctx.Done()
			server.Close()
		},
	)

	// amount := 20000.0
	// if os.Getenv("AMOUNT") != "" {
	// 	v, err := strconv.ParseFloat(os.Getenv("AMOUNT"), 64)
	// 	if err != nil {
	// 		log.Printf("AMOUNT variable is not a valid number")
	// 	} else {
	// 		amount = v
	// 	}
	// }
	// ref := "test transaction"
	// if os.Getenv("REF") != "" {
	// 	ref = os.Getenv("REF")
	// }

	// req := Request{
	// 	SourceBank:      "abbank",
	// 	SourceAcc:       "12345",
	// 	DestinationBank: "bcbank",
	// 	DestinationAcc:  "99999",
	// 	Amount:          amount,
	// 	Ref:             ref,
	// }

	// options := client.StartWorkflowOptions{
	// 	TaskQueue: "clearing-house",
	// 	ID:        fmt.Sprintf("MT: %s", req.Ref),
	// }
	// workflowRun, err := c.ExecuteWorkflow(ctx, options, "MoneyTransfer", req)
	// if err != nil {
	// 	return err
	// }
	// log.Printf("workflow started with ID: %s and RunID: %s", workflowRun.GetID(), workflowRun.GetRunID())

	// resp := Response{}
	// if err := workflowRun.Get(ctx, &resp); err != nil {
	// 	return err
	// }
	// fmt.Println(resp.String())

	// return nil
}

func (r Response) String() string {
	if r.StartTime.IsZero() {
		return ""
	}
	var withdrawDuration time.Duration
	var depositDuration time.Duration
	var refundDuration time.Duration
	var moneyLaunderingCheckDuration time.Duration
	if !r.WithdrawDoneTime.IsZero() {
		if r.MoneyLaunderingCheckFinishTime.IsZero() {
			withdrawDuration = r.WithdrawDoneTime.Sub(r.StartTime)
		} else {
			withdrawDuration = r.WithdrawDoneTime.Sub(r.MoneyLaunderingCheckFinishTime)
		}
	}
	if !r.RefundDoneTime.IsZero() {
		refundDuration = r.RefundDoneTime.Sub(r.WithdrawDoneTime)
	}
	if !r.DepositDoneTime.IsZero() {
		depositDuration = r.DepositDoneTime.Sub(r.WithdrawDoneTime)
	}
	if !r.MoneyLaunderingCheckFinishTime.IsZero() {
		moneyLaunderingCheckDuration = r.MoneyLaunderingCheckFinishTime.Sub(r.StartTime)
	}
	switch r.Status {
	case "success":
		return fmt.Sprintf(`SUCCESS:
	Start: %v
	MoneyLaunderingCheckFinishTime: [%v] %v
	Withdraw: [%v] %v
	Deposit: [%v] %v`, r.StartTime, moneyLaunderingCheckDuration, r.MoneyLaunderingCheckFinishTime,
			withdrawDuration, r.WithdrawDoneTime, depositDuration, r.DepositDoneTime)
	case "failure":
		return fmt.Sprintf(`FAILURE: %s
	Start: %v
	MoneyLaunderingCheckFinishTime: [%v] %v
	Withdraw: [%v] %v
	Refund: [%v] %v
	Deposit: [%v] %v`, r.FailureReason, r.StartTime, moneyLaunderingCheckDuration, r.MoneyLaunderingCheckFinishTime,
			withdrawDuration, r.WithdrawDoneTime, refundDuration, r.RefundDoneTime, depositDuration, r.DepositDoneTime)
	case "refunded":
		return fmt.Sprintf(`REFUNDED: %s
	Start: %v
	MoneyLaunderingCheckFinishTime: [%v] %v
	Withdraw: [%v] %v
	Refund: [%v] %v`, r.FailureReason, r.StartTime, moneyLaunderingCheckDuration, r.MoneyLaunderingCheckFinishTime,
			withdrawDuration, r.WithdrawDoneTime, refundDuration, r.RefundDoneTime)
	default:
		return ""
	}
}
