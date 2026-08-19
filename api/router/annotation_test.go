package router

import (
	"context"
	"errors"
	"testing"
)

func TestRunRouterWorkStopsOnClientAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RunRouterWork(ctx, func(workContext context.Context) error { return workContext.Err() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected cancellation: %v", err)
	}
	if !errors.Is(RouterContextError(ctx), context.Canceled) {
		t.Fatal("router cancellation disappeared")
	}
	if RouterContextState(ctx) != "cancelled" {
		t.Fatal("router state was not cancelled")
	}
}
