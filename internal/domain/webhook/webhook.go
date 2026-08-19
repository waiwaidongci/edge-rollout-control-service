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
	d.Attempts++
	if err != nil {
		d.LastError = err.Error()
	} else {
		d.LastError = ""
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	backoff := time.Duration(d.Attempts*d.Attempts) * time.Second
	if backoff > 30*time.Minute {
		backoff = 30 * time.Minute
	}
	d.NextAttemptAt = now.UTC().Add(backoff)
	d.Status = DeliveryRetrying
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
