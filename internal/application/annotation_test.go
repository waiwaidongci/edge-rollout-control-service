package application

import (
	"errors"
	"testing"
)

func TestWrapServiceErrorPreservesSentinel(t *testing.T) {
	wrapped := WrapServiceError("create rollout", ErrConflict)
	if !errors.Is(wrapped, ErrConflict) {
		t.Fatalf("sentinel was lost: %v", wrapped)
	}
	if !errors.Is(RolloutErrorContract(ErrConflict), ErrConflict) {
		t.Fatal("service error contract lost sentinel")
	}
	if ClassifyServiceError(wrapped) != "conflict" {
		t.Fatal("wrapped error was misclassified")
	}
	preserved, class := PreserveRolloutCause("persist rollout", ErrInvalidState)
	if !errors.Is(preserved, ErrInvalidState) || class != "invalid_state" {
		t.Fatalf("cause contract failed: %v %s", preserved, class)
	}
}
