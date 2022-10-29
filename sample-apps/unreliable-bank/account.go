package ubank

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/andrebq/stage"
)

type (
	Balance struct {
		Current int64
		Time    time.Time
	}
	Account struct {
		state struct {
			AccountID string
			Balance   int64
		}
	}

	Transfer struct {
		From, To string
		Seq      int64
		Total    int64
	}
)

// business methods

func (a Account) Debit(_ stage.Message) (Account, error) {
	a.state.Balance -= 1
	return a, nil
}

func (a Account) Credit(msg stage.Message) (Account, error) {
	var t Transfer
	err := json.NewDecoder(bytes.NewBuffer(msg.Content)).Decode(&t)
	if err != nil {
		return a, err
	}
	a.state.Balance += t.Total
	return a, nil
}

func (a Account) GetBalance(ctx context.Context, msg stage.Message, output stage.Media) (Account, error) {
	output.Send(ctx, msg.From, "Reply", Balance{Current: a.state.Balance, Time: time.Now().Truncate(time.Millisecond)})
	return a, nil
}

// methods related to stage dispatch

func (a Account) Dispatch(ctx context.Context, msg stage.Message, output stage.Media) (stage.Actor, error) {
	switch msg.Method {
	case "Debit":
		return a.Debit(msg)
	case "Credit":
		return a.Credit(msg)
	case "GetBalance":
		return a.GetBalance(ctx, msg, output)
	}
	return a, stage.ErrMethodNotFound
}

func (a Account) Hibernate(ctx context.Context) (interface{}, error) {
	return a.state, nil
}

func (a Account) Init(ctx context.Context, state interface{}) error {
	return nil
}

func (a Account) Template() string { return "Account" }
