package ubank

import (
	"context"
	"time"

	"github.com/andrebq/stage"
)

const (
	accountActorName = "Account"
)

type (
	Balance struct {
		Current int64
		Time    time.Time
	}

	AccountState struct {
		AccountID string
		Balance   int64
	}
	Account struct {
		stage.BaseActor
		stage.Dispatcher
		state *AccountState
	}

	Transfer struct {
		From, To string
		Seq      int64
		Total    int64
	}
)

// business methods

func NewAccount() *Account {
	ac := &Account{}
	ac.BaseActor = stage.BasicActor(accountActorName, func() interface{} {
		return &AccountState{}
	}, func() interface{} {
		return ac.state
	})
	ac.Dispatcher = stage.DispatchByReflection(ac)
	return ac
}

func (a *Account) Init(ctx context.Context, state interface{}) error {
	a.state = state.(*AccountState)
	return nil
}

func (a *Account) Debit(_ context.Context, _ stage.Identity, t *Transfer, _ stage.Media) error {
	a.state.Balance -= t.Total
	return nil
}

func (a *Account) Credit(_ context.Context, _ stage.Identity, t *Transfer, _ stage.Media) error {
	a.state.Balance += t.Total
	return nil
}

func (a *Account) GetBalance(ctx context.Context, from stage.Identity, _ *struct{}, output stage.Media) error {
	output.Send(ctx, from, "Reply", Balance{Current: a.state.Balance, Time: time.Now().Truncate(time.Millisecond)})
	return nil
}
