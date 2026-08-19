package rollout

import (
	"testing"
	"time"
)

func TestApplyTargetTransitionRejectsMissingTarget(t *testing.T) {
	if err := ApplyTargetTransition(nil, TargetReady, time.Now()); err == nil {
		t.Fatal("missing target should be rejected")
	}
}

func TestApplyTargetTransitionRecordsTimestamps(t *testing.T) {
	at := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	target := Target{Status: TargetPending}
	if err := ApplyTargetTransition(&target, TargetReady, at); err != nil || target.Status != TargetReady {
		t.Fatalf("transition failed: %v %#v", err, target)
	}
}
