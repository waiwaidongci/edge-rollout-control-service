package middleware

import (
	"context"
	"errors"
	"testing"
)

func TestRunWithRequestContextReturnsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RunWithRequestContext(ctx, func(workContext context.Context) error {
		<-workContext.Done()
		return workContext.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected cancellation: %v", err)
	}
}
