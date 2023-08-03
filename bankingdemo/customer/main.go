package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.temporal.io/sdk/client"
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

	c, err := client.Dial(client.Options{
		Namespace: "default",
	})
	if err != nil {
		return err
	}
	defer c.Close()

	req := Request{
		SourceBank:      "abbank",
		SourceAcc:       "12345",
		DestinationBank: "bcbank",
		DestinationAcc:  "99999",
		Amount:          2000,
		Ref:             "test transaction",
	}

	options := client.StartWorkflowOptions{
		TaskQueue: "clearing-house",
		ID:        fmt.Sprintf("MT: %s", req.Ref),
	}
	workflowRun, err := c.ExecuteWorkflow(ctx, options, "MoneyTransfer", req)
	if err != nil {
		return err
	}
	log.Printf("workflow started with ID: %s and RunID: %s", workflowRun.GetID(), workflowRun.GetRunID())

	resp := Response{}
	if err := workflowRun.Get(ctx, &resp); err != nil {
		return err
	}
	fmt.Println(resp.String())

	return nil
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
