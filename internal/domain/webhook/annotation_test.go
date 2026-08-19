package webhook

import (
	"errors"
	"testing"
	"time"
)

func TestScheduleRetryPublishesRetryingState(t *testing.T) {
	delivery := Delivery{Status: DeliveryPending}
	now := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	delivery.ScheduleRetry(now, errors.New("temporary"))
	if delivery.Status != DeliveryRetrying || delivery.Attempts != 1 || !delivery.NextAttemptAt.After(now) {
		t.Fatalf("unexpected retry state: %#v", delivery)
	}
}
