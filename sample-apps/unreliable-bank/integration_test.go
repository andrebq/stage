package ubank

import (
	"context"
	"testing"
	"time"

	"github.com/andrebq/stage"
)

func TestSimpleTransaction(t *testing.T) {
	ctx := context.Background()
	s, stop := acquireStage(t)
	bob, err := s.Spawn(ctx, NewAccount())
	if err != nil {
		t.Fatal(err)
	}
	err = s.Inject(ctx, bob, "Credit", Transfer{Total: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	var balance Balance
	err = s.Request(ctx, &balance, time.Second, bob, "GetBalance", nil)
	if err != nil {
		t.Fatal(err)
	} else if balance.Current != 10 {
		t.Fatalf("Invalid balance, got: %#v", balance)
	}
}

func TestLedgerTransaction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s, stop := acquireStage(t)
	defer stop()
	bob, err := s.Spawn(ctx, NewAccount())
	if err != nil {
		t.Fatal(err)
	}
	alice, err := s.Spawn(ctx, NewAccount())
	if err != nil {
		t.Fatal(err)
	}

	ledger, err := s.Spawn(ctx, NewLedger())
	if err != nil {
		t.Fatal(err)
	}
	s.Inject(ctx, ledger, "RegisterAccount", AccountInfo{Name: "Alice-Check-Account", PID: alice})
	s.Inject(ctx, ledger, "RegisterAccount", AccountInfo{Name: "Bob-Check-Account", PID: bob})

	s.Inject(ctx, ledger, "Schedule", Transfer{From: "Alice-Check-Account", To: "Bob-Check-Account", Total: 10, Seq: 1})
	for {
		var pendingCount int
		err := s.Request(ctx, &pendingCount, time.Second, ledger, "NumPendingTransaction", struct{}{})
		if err != nil {
			t.Fatal(err)
		}
		if pendingCount == 0 {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}

	var balance Balance
	err = s.Request(ctx, &balance, time.Second, bob, "GetBalance", nil)
	if err != nil {
		t.Fatal(err)
	} else if balance.Current != 10 {
		t.Fatalf("Invalid balance, got: %#v", balance)
	}
}

func acquireStage(t interface{ Fatal(...interface{}) }) (*stage.S, func()) {
	s, err := stage.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, func() {
		s.Close()
	}
}
