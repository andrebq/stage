package ubank

import (
	"context"

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

	AccountInfo struct {
		Name string
		PID  stage.Identity
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

func (l *Ledger) RegisterAccount(ctx context.Context, _ stage.Identity, ai *AccountInfo, media stage.Media) error {
	l.accounts[ai.Name] = ai.PID
	return nil
}

func (l *Ledger) NumPendingTransaction(ctx context.Context, from stage.Identity, _ *struct{}, media stage.Media) error {
	media.Send(ctx, from, "Reply", uint64(len(l.pendingCredits)+len(l.pendingDebits)))
	return nil
}

func (l *Ledger) Schedule(ctx context.Context, _ stage.Identity, t *Transfer, media stage.Media) error {
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

func (l *Ledger) ConfirmDebit(ctx context.Context, _ stage.Identity, t *Transfer, media stage.Media) error {
	if _, ok := l.pendingDebits[*t]; !ok {
		return nil
	}
	delete(l.pendingDebits, *t)
	l.pendingCredits[*t] = empty{}
	creditor, _ := l.accounts[t.To], l.accounts[t.From]
	media.Send(ctx, creditor, "Credit", *t)
	return nil
}

func (l *Ledger) ConfirmCredit(ctx context.Context, _ stage.Identity, t *Transfer, media stage.Media) error {
	if _, ok := l.pendingCredits[*t]; !ok {
		return nil
	}
	delete(l.pendingCredits, *t)
	return nil
}
