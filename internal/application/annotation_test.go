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
}
