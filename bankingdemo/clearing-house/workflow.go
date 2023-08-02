package main

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var moneyLaunderingThresholdAmount = 1000.0

type Request struct {
	SourceBank, DestinationBank string
	SourceAcc, DestinationAcc   string
	Amount                      float64
	Ref                         string
}

type BankTransaction struct {
	AccountID string
	Amount    float64
	Reference string
	IsRefund  bool
}

type Response struct {
	StartTime                      time.Time
	MoneyLaunderingCheckFinishTime time.Time
	WithdrawDoneTime               time.Time
	RefundDoneTime                 time.Time
	DepositDoneTime                time.Time
}

func MoneyTransfer(ctx workflow.Context, req Request) (Response, error) {
	var startTime time.Time
	if err := workflow.SideEffect(ctx, currentTime).Get(&startTime); err != nil {
		return Response{}, err
	}

	var moneyLaunderingFinishTime time.Time
	if req.Amount >= moneyLaunderingThresholdAmount {
		actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			TaskQueue:           "money-laundering",
			StartToCloseTimeout: time.Hour * 24 * 5,
		})
		var moneyLaunderingCheckResponse string
		if err := workflow.ExecuteActivity(actx, "MoneyLaunderingCheck", req).Get(actx, &moneyLaunderingCheckResponse); err != nil {
			return Response{}, err
		}
		_ = workflow.SideEffect(ctx, currentTime).Get(&moneyLaunderingFinishTime)
		if moneyLaunderingCheckResponse == "reject" {
			return Response{}, temporal.NewNonRetryableApplicationError("money laundering check failed", "validation", nil)
		}
	}

	// Withdrawal
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           req.SourceBank,
		StartToCloseTimeout: time.Minute,
	})
	txn := BankTransaction{
		AccountID: req.SourceAcc,
		Amount:    req.Amount,
		Reference: req.Ref,
	}
	if err := workflow.ExecuteActivity(actx, "Withdraw", txn).Get(actx, nil); err != nil {
		return Response{}, err
	}

	var withdrawDoneTime time.Time
	_ = workflow.SideEffect(ctx, currentTime).Get(&withdrawDoneTime)

	// Deposit
	actx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           req.DestinationBank,
		StartToCloseTimeout: time.Minute,
	})
	txn = BankTransaction{
		AccountID: req.DestinationAcc,
		Amount:    req.Amount,
		Reference: req.Ref,
	}
	if err := workflow.ExecuteActivity(actx, "Deposit", txn).Get(actx, nil); err != nil {
		// Refund
		actx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			TaskQueue:           req.SourceBank,
			StartToCloseTimeout: time.Minute,
		})
		txn = BankTransaction{
			AccountID: req.SourceAcc,
			Amount:    req.Amount,
			Reference: fmt.Sprintf("REFUND: %s", req.Ref),
			IsRefund:  true,
		}
		if refundErr := workflow.ExecuteActivity(actx, "Deposit", txn).Get(actx, nil); refundErr != nil {
			resp := Response{StartTime: startTime, WithdrawDoneTime: withdrawDoneTime}
			return resp, fmt.Errorf("error refunding withdrawn amount. Deposit error: %w. Refund error: %w", err, refundErr)
		}
		var refundDoneTime time.Time
		_ = workflow.SideEffect(ctx, currentTime).Get(&refundDoneTime)
		resp := Response{StartTime: startTime, WithdrawDoneTime: withdrawDoneTime, RefundDoneTime: refundDoneTime}
		return resp, err
	}

	var depositDoneTime time.Time
	_ = workflow.SideEffect(ctx, currentTime).Get(&depositDoneTime)

	return Response{
		StartTime:                      startTime,
		MoneyLaunderingCheckFinishTime: moneyLaunderingFinishTime,
		WithdrawDoneTime:               withdrawDoneTime,
		DepositDoneTime:                depositDoneTime,
	}, nil
}

func currentTime(ctx workflow.Context) interface{} {
	return time.Now()
}
