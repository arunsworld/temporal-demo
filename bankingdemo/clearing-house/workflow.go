package main

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

var moneyLaunderingThresholdAmount = 1000.0

type Request struct {
	SourceBank, DestinationBank string
	SourceAcc, DestinationAcc   string
	Amount                      float64
	Ref                         string
}

type TxnResponse struct {
	Status        string // success, failure
	FailureReason string
}

type BankTransaction struct {
	AccountID string
	Amount    float64
	Reference string
	IsRefund  bool
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
			return Response{
				Status:                         "failure",
				FailureReason:                  "Money Laundering check failed",
				StartTime:                      startTime,
				MoneyLaunderingCheckFinishTime: moneyLaunderingFinishTime,
			}, nil
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
	txnResp := TxnResponse{}
	if err := workflow.ExecuteActivity(actx, "Withdraw", txn).Get(actx, &txnResp); err != nil {
		return Response{}, err
	}
	if txnResp.Status != "success" {
		return Response{
			Status:                         "failure",
			FailureReason:                  txnResp.FailureReason,
			StartTime:                      startTime,
			MoneyLaunderingCheckFinishTime: moneyLaunderingFinishTime,
		}, nil
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
	txnResp = TxnResponse{}
	if err := workflow.ExecuteActivity(actx, "Deposit", txn).Get(actx, &txnResp); err != nil {
		return Response{}, err
	}
	if txnResp.Status != "success" {
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
		refundTxnResp := TxnResponse{}
		if refundErr := workflow.ExecuteActivity(actx, "Deposit", txn).Get(actx, &refundTxnResp); refundErr != nil {
			return Response{}, refundErr
		}
		if refundTxnResp.Status != "success" {
			return Response{
				Status:                         "failure",
				FailureReason:                  fmt.Sprintf("Deposit failed due to %s. Refund also failed: %s", txnResp.FailureReason, refundTxnResp.FailureReason),
				StartTime:                      startTime,
				MoneyLaunderingCheckFinishTime: moneyLaunderingFinishTime,
				WithdrawDoneTime:               withdrawDoneTime,
			}, nil
		}
		var refundDoneTime time.Time
		_ = workflow.SideEffect(ctx, currentTime).Get(&refundDoneTime)
		return Response{
			Status:                         "refunded",
			FailureReason:                  txnResp.FailureReason,
			StartTime:                      startTime,
			MoneyLaunderingCheckFinishTime: moneyLaunderingFinishTime,
			WithdrawDoneTime:               withdrawDoneTime,
			RefundDoneTime:                 refundDoneTime,
		}, nil
	}

	var depositDoneTime time.Time
	_ = workflow.SideEffect(ctx, currentTime).Get(&depositDoneTime)

	return Response{
		Status:                         "success",
		StartTime:                      startTime,
		MoneyLaunderingCheckFinishTime: moneyLaunderingFinishTime,
		WithdrawDoneTime:               withdrawDoneTime,
		DepositDoneTime:                depositDoneTime,
	}, nil
}

func currentTime(ctx workflow.Context) interface{} {
	return time.Now()
}
