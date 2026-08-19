package repository

import (
	"context"
	"testing"
)

func TestOperationContextKeepsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if OperationContext(ctx).Err() == nil {
		t.Fatal("cancelled context was detached")
	}
	if ContextContract(ctx).Err() == nil {
		t.Fatal("repository context contract was detached")
	}
	if ContextCancellation(ctx) == nil {
		t.Fatal("operation cancellation disappeared")
	}
	if RolloutContextError(ctx) == nil {
		t.Fatal("rollout cancellation disappeared")
	}
}
