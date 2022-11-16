package ubank

import (
	"context"
	"time"

	"github.com/andrebq/stage"
)

type (
	Balance struct {
		Current int64
		Time    time.Time
	}

	AccountState struct {
		AccountID string
		Balance   int64

		Transfers map[Transfer]empty
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
	ac.Dispatcher = stage.DispatchByReflection(ac)
	return ac
}

func (a *Account) Zero(_ context.Context) error {
	a.state = &AccountState{Transfers: make(map[Transfer]empty)}
	return nil
}

func (a *Account) Debit(ctx context.Context, from stage.Identity, t *Transfer, media stage.Media) error {
	// dedup transactions
	if _, ok := a.state.Transfers[*t]; !ok {
		a.state.Balance -= t.Total
	}
	media.Send(ctx, from, "ConfirmDebit", *t)
	return nil
}

func (a *Account) Credit(ctx context.Context, from stage.Identity, t *Transfer, media stage.Media) error {
	// dedup transactions
	if _, ok := a.state.Transfers[*t]; !ok {
		a.state.Balance += t.Total
	}
	media.Send(ctx, from, "ConfirmCredit", *t)
	return nil
}

func (a *Account) GetBalance(ctx context.Context, from stage.Identity, _ *struct{}, output stage.Media) error {
	output.Send(ctx, from, "Reply", Balance{Current: a.state.Balance, Time: time.Now().Truncate(time.Millisecond)})
	return nil
}
