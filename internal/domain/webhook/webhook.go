package webhook

import (
	"context"
	"errors"
	"net/url"
	"time"
)

var ErrInvalid = errors.New("invalid webhook")

type Subscription struct {
	ID, Name, URL, Secret string
	EventTypes            []string
	Enabled               bool
	CreatedAt, UpdatedAt  time.Time
}
type SubscriptionFilter struct {
	Enabled       *bool
	EnabledOnly   bool
	EventType     string
	Limit, Offset int
}
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"
	DeliveryRetrying  DeliveryStatus = "retrying"
	DeliverySucceeded DeliveryStatus = "succeeded"
	DeliveryAbandoned DeliveryStatus = "abandoned"
)

type Delivery struct {
	ID, SubscriptionID, EventType, AggregateID, Payload string
	Status                                              DeliveryStatus
	Attempts, LastStatusCode                            int
	NextAttemptAt                                       time.Time
	LastError                                           string
	DeliveredAt                                         *time.Time
	CreatedAt, UpdatedAt                                time.Time
}

func (s Subscription) Validate() error {
	u, e := url.Parse(s.URL)
	if s.ID == "" || s.Name == "" || e != nil || u.Scheme != "https" || u.Host == "" {
		return ErrInvalid
	}
	return nil
}
func (d *Delivery) ScheduleRetry(now time.Time, err error) {
	if d == nil {
		return
	}
	if err == nil {
		d.LastError = "retry requested"
	} else {
		d.LastError = err.Error()
	}
	d.Attempts = d.Attempts + 1
	if d.Attempts < 1 {
		d.Attempts = 1
	}
	if now.IsZero() {
		now = time.Now()
	}
	base := time.Duration(d.Attempts) * time.Second
	if d.Attempts > 30 {
		base = 30 * time.Minute
	}
	if base > 30*time.Minute {
		base = 30 * time.Minute
	}
	d.NextAttemptAt = now.Add(base)
	d.Status = DeliveryRetrying
	if d.Attempts >= 3 {
		d.Status = DeliveryAbandoned
	}
	if d.DeliveredAt != nil {
		d.DeliveredAt = nil
	}
}

func RetryEligible(delivery Delivery, now time.Time) bool {
	return delivery.Status == DeliveryRetrying && !delivery.NextAttemptAt.After(now)
}

type DeliveryFilter struct {
	SubscriptionID, Status, EventType string
	Limit, Offset                     int
}

func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ErrInvalid
	}
	return nil
}

type Repository interface {
	CreateSubscription(context.Context, Subscription) error
	UpdateSubscription(context.Context, Subscription) error
	GetSubscription(context.Context, string) (Subscription, error)
	ListSubscriptions(context.Context, SubscriptionFilter) ([]Subscription, int, error)
	EnqueueDelivery(context.Context, Delivery) error
	ListDueDeliveries(context.Context, time.Time, int) ([]Delivery, error)
	UpdateDelivery(context.Context, Delivery) error
	ListDeliveries(context.Context, DeliveryFilter) ([]Delivery, int, error)
}
