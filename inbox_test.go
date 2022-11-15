package stage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInbox(t *testing.T) {
	ib := NewInbox[int]()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go ib.run(ctx)
	defer ib.Close()
	ib.Push(ctx, 1)

	val, err := ib.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if val != 1 {
		t.Fatal("should be 1 got", val)
	}
	ib.Close()
	_, err = ib.Next(ctx)
	if !errors.Is(err, errClosedInbox) {
		t.Fatal("error should be closed inbox")
	}
}
