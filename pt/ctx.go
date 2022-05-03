package pt

import (
	"context"
	"testing"
)

func TestContext(t *testing.T) context.Context {
	t.Helper()

	var ctx context.Context
	var cancel func()

	if deadline, ok := t.Deadline(); ok {
		ctx, cancel = context.WithDeadline(context.Background(), deadline)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	t.Cleanup(cancel)

	return ctx
}
