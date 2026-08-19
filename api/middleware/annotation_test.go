package middleware

import (
	"context"
	"errors"
	"testing"
)

func TestRunWithRequestContextStopsOnClientAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RunWithRequestContext(ctx, func(workContext context.Context) error {
		<-workContext.Done()
		return workContext.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected cancellation: %v", err)
	}
	if !errors.Is(RequestContextError(ctx), context.Canceled) {
		t.Fatal("request cancellation disappeared")
	}
	if RequestContextState(ctx) != "cancelled" {
		t.Fatal("request state was not cancelled")
	}
}
