package ubank

import (
	"context"
	"os"
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

func acquireStage(t interface{ Fatal(...interface{}) }) (*stage.S, func()) {
	s, err := stage.New(os.Getenv("STAGE_TEST_NATS_SERVER"))
	if err != nil {
		t.Fatal(err)
	}
	return s, func() {
		s.Close()
	}
}
