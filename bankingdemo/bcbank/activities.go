package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
)

type Transaction struct {
	AccountID string
	Amount    float64
	Reference string
	IsRefund  bool
}

var ErrTransient = errors.New("transient error")
var ErrInvalidAccount = errors.New("invalid account ID")

var transientErrorProbability = 0
var notEnoughFundsProbability = 0
var idIssueProbability = 0

type TxnResponse struct {
	Status        string // success, failure
	FailureReason string
}

func Withdraw(ctx context.Context, txn Transaction) (TxnResponse, error) {
	// this is where we talk to the bank; for now we just have a random implementation
	if transientErrorProbability > 0 && rand.Intn(int(1/transientErrorProbability)) == 0 {
		log.Printf("transient withdraw error for %s", txn.Reference)
		return TxnResponse{}, ErrTransient

	}
	if notEnoughFundsProbability > 0 && rand.Intn(int(1/notEnoughFundsProbability)) == 0 {
		log.Printf("funds issue for %s", txn.Reference)
		return TxnResponse{
			Status:        "failure",
			FailureReason: "Not Enough Funds",
		}, nil

	}
	log.Printf("withdrawn %.2f from %s. Ref: [%s]", txn.Amount, txn.AccountID, txn.Reference)
	return TxnResponse{Status: "success"}, nil
}

func Deposit(ctx context.Context, txn Transaction) (TxnResponse, error) {
	// this is where we talk to the bank; for now we just have a random implementation
	if transientErrorProbability > 0 && rand.Intn(int(1/transientErrorProbability)) == 0 {
		log.Printf("transient deposit error for %s", txn.Reference)
		return TxnResponse{}, ErrTransient

	}
	if !txn.IsRefund && idIssueProbability > 0 && rand.Intn(int(1/idIssueProbability)) == 0 {
		log.Printf("ID issue for %s", txn.Reference)
		return TxnResponse{
			Status:        "failure",
			FailureReason: "Account ID Not Found",
		}, nil

	}
	log.Printf("deposited %.2f to %s. Ref: [%s]", txn.Amount, txn.AccountID, txn.Reference)
	return TxnResponse{Status: "success"}, nil
}
