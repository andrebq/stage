package ubank

import (
	"context"
	"testing"
	"time"

	"github.com/andrebq/stage"
)

func TestDistributedBank(t *testing.T) {
	t.Parallel()
	// node1 and node2 are a point-to-point stage
	// every remote message from node2 will go to node1
	// and every remote message from node1 will go to node2
	//
	// In most scenarios, this would be a start stage,
	// where one central location is connected to all
	// other locations
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	node1, node2 := pipeUpstream("n1", "n2")
	s1, err := stage.New(node1)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Close()
	s2, err := stage.New(node2)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	alice, err := s1.Spawn(ctx, NewAccount())
	if err != nil {
		t.Fatal(err)
	}
	bob, err := s2.Spawn(ctx, NewAccount())
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := s1.Spawn(ctx, NewLedger())
	if err != nil {
		t.Fatal(err)
	}
	// Now from s2 we try to register 2 accounts onto the ledger running on s1
	s2.Inject(ctx, ledger, "RegisterAccount", AccountInfo{Name: "Alice-Check-Account", PID: alice})
	s2.Inject(ctx, ledger, "RegisterAccount", AccountInfo{Name: "Bob-Check-Account", PID: bob})

	for {
		var status ScheduleStatus
		err := s1.Request(ctx, &status, time.Second, ledger, "Schedule", Transfer{From: "Alice-Check-Account", To: "Bob-Check-Account", Total: 10, Seq: 1})
		if err != nil {
			t.Fatal(err)
		}
		// loop until we managed to schedule the transaciton, since the account registration happens
		// in the background, the Schedule message might arrive before the RegisterAccount
		// so, if an account does not have a valid PID associated with it
		// the ledger cannot schedule it
		if !status.Valid {
			time.Sleep(time.Millisecond * 10)
			continue
		}
		break
	}

	for {
		var stats LedgerStats
		err := s2.Request(ctx, &stats, time.Second, ledger, "NumPendingTransaction", struct{}{})
		if err != nil {
			t.Fatal(err)
		}
		// loop until the ledger has scheduled at least one transaction
		// and until there are no pending transactions.
		//
		// Even though both loops are sequential, this arrangement allows them
		// to run in concurrently.
		//
		// And this is what makes stage (and the whole Actor model) so
		// good.
		if stats.Total != 0 && stats.Pending == 0 {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}

	var balance Balance
	err = s2.Request(ctx, &balance, time.Second, bob, "GetBalance", nil)
	if err != nil {
		t.Fatal(err)
	} else if balance.Current != 10 {
		t.Fatalf("Invalid balance, got: %#v", balance)
	}
}
