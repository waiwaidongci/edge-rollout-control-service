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
}
