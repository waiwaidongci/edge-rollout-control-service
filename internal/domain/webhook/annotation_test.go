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
	if RetryStatusContract(DeliveryPending) != DeliveryRetrying {
		t.Fatal("retry status contract did not publish retrying")
	}
	if !RetryEligible(delivery, delivery.NextAttemptAt) {
		t.Fatal("scheduled retry was not eligible")
	}
	if !RetryEventAllowed(delivery) {
		t.Fatal("retry event was suppressed")
	}
}
