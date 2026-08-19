package device

import (
	"testing"
	"time"
)

func TestEvaluateHealth(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entity := Device{Status: StatusOnline, LastHeartbeatAt: now.Add(-2 * time.Minute)}
	if got := EvaluateHealth(entity, now, HealthPolicy{StaleAfter: time.Minute, OfflineAfter: 5 * time.Minute}); got != HealthStale {
		t.Fatalf("health: %s", got)
	}
}

func TestSelectorExpression(t *testing.T) {
	selector, err := ParseSelectorExpression("area=east,role=prod")
	if err != nil {
		t.Fatal(err)
	}
	if !selector.Match(map[string]string{"area": "east", "role": "prod"}) {
		t.Fatal("expected selector match")
	}
}
