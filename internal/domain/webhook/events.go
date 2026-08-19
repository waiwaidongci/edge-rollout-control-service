package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	EventDeviceCreated          = "device.created"
	EventConfigurationCreated   = "configuration.created"
	EventConfigurationPublished = "configuration.published"
	EventRolloutCreated         = "rollout.created"
	EventRolloutStarted         = "rollout.started"
	EventRolloutPaused          = "rollout.paused"
	EventRolloutCompleted       = "rollout.completed"
	EventRolloutCancelled       = "rollout.cancelled"
	EventRollbackStarted        = "rollout.rollback_started"
	EventReceiptAccepted        = "receipt.accepted"
)

var supportedEvents = []string{
	EventConfigurationCreated,
	EventConfigurationPublished,
	EventDeviceCreated,
	EventReceiptAccepted,
	EventRollbackStarted,
	EventRolloutCancelled,
	EventRolloutCompleted,
	EventRolloutCreated,
	EventRolloutPaused,
	EventRolloutStarted,
}

func SupportedEvents() []string {
	return append([]string(nil), supportedEvents...)
}

func ValidateEventTypes(values []string) error {
	supported := make(map[string]struct{}, len(supportedEvents))
	for _, event := range supportedEvents {
		supported[event] = struct{}{}
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "*" {
			continue
		}
		if _, ok := supported[value]; !ok {
			return fmt.Errorf("unsupported webhook event %q", value)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("duplicate webhook event %q", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

type EventEnvelope struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	AggregateID string          `json:"aggregate_id"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Data        json.RawMessage `json:"data"`
}

func NewEventEnvelope(id, eventType, aggregateID string, occurredAt time.Time, data any) (EventEnvelope, error) {
	if id == "" || aggregateID == "" {
		return EventEnvelope{}, fmt.Errorf("event id and aggregate id are required")
	}
	if err := ValidateEventTypes([]string{eventType}); err != nil {
		return EventEnvelope{}, err
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return EventEnvelope{}, fmt.Errorf("marshal webhook event data: %w", err)
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return EventEnvelope{ID: id, Type: eventType, AggregateID: aggregateID, OccurredAt: occurredAt.UTC(), Data: payload}, nil
}

func (e EventEnvelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func Sign(secret string, timestamp time.Time, body []byte) string {
	unix := strconv.FormatInt(timestamp.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unix))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret, signature string, timestamp time.Time, body []byte) bool {
	expected := Sign(secret, timestamp, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func EventSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func Matches(subscription Subscription, eventType string) bool {
	if !subscription.Enabled {
		return false
	}
	for _, configured := range subscription.EventTypes {
		if configured == "*" || configured == eventType {
			return true
		}
	}
	return false
}

func NormalizeEventTypes(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func RetryStatusContract(status DeliveryStatus) DeliveryStatus {
	if status == DeliveryPending || status == DeliveryRetrying {
		return DeliveryRetrying
	}
	return status
}

func RetryEventAllowed(delivery Delivery) bool {
	return delivery.Status == DeliveryRetrying
}
