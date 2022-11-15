package ubank

import (
	"context"
	"errors"

	"github.com/andrebq/stage"
)

type (
	Ledger struct {
		stage.Dispatcher
		accounts map[string]stage.Identity

		transfers      []Transfer
		pendingDebits  map[Transfer]empty
		pendingCredits map[Transfer]empty
	}

	empty struct{}
)

func NewLedger() *Ledger {
	l := &Ledger{}
	l.Dispatcher = stage.DispatchByReflection(l)
	return l
}

func (l *Ledger) Zero(_ context.Context) error {
	l.accounts = make(map[string]stage.Identity)
	l.pendingCredits = make(map[Transfer]empty)
	l.pendingDebits = make(map[Transfer]empty)
	return nil
}

func (l *Ledger) Schedule(ctx context.Context, t *Transfer, media stage.Media) error {
	for _, v := range l.transfers {
		if v == *t {
			// dedup a transaction
			return nil
		}
	}
	creditor, debitor := l.accounts[t.To], l.accounts[t.From]
	if creditor.IsZero() || debitor.IsZero() {
		return nil
	}
	l.transfers = append(l.transfers, *t)
	l.pendingDebits[*t] = empty{}
	media.Send(ctx, debitor, "Debit", t)
	return nil
}

func (l *Ledger) ConfirmDebit(ctx context.Context, t *Transfer, media stage.Media) error {
	return errors.New("not implemented")
}
