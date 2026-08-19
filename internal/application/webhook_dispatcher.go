package application

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/example/edge-rollout-control/internal/domain/webhook"
)

type WebhookDispatcher struct {
	repository  UnitOfWork
	clock       Clock
	client      *http.Client
	logger      *slog.Logger
	maxAttempts int
}

func NewWebhookDispatcher(repository UnitOfWork, clock Clock, logger *slog.Logger, maxAttempts int) *WebhookDispatcher {
	return &WebhookDispatcher{repository: repository, clock: clock, client: &http.Client{Timeout: 10 * time.Second}, logger: logger, maxAttempts: maxAttempts}
}

func (d *WebhookDispatcher) Process(ctx context.Context, limit int) error {
	deliveries, err := d.repository.ListDueDeliveries(ctx, d.clock.Now(), limit)
	if err != nil {
		return fmt.Errorf("list due webhook deliveries: %w", err)
	}
	for _, delivery := range deliveries {
		if err := d.deliver(ctx, delivery); err != nil {
			d.logger.WarnContext(ctx, "webhook delivery failed", "delivery_id", delivery.ID, "error", err)
		}
	}
	return nil
}

func (d *WebhookDispatcher) deliver(ctx context.Context, delivery webhook.Delivery) error {
	subscription, err := d.repository.GetSubscription(ctx, delivery.SubscriptionID)
	if err != nil {
		return fmt.Errorf("get webhook subscription: %w", err)
	}
	now := d.clock.Now()
	delivery.Attempts++
	delivery.UpdatedAt = now
	if !subscription.Enabled {
		delivery.Status = webhook.DeliveryAbandoned
		delivery.LastError = "subscription disabled"
		return d.repository.UpdateDelivery(ctx, delivery)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, subscription.URL, bytes.NewReader([]byte(delivery.Payload)))
	if err != nil {
		return d.recordFailure(ctx, delivery, 0, err)
	}
	timestamp := strconv.FormatInt(now.Unix(), 10)
	signature := signWebhook(subscription.Secret, timestamp, []byte(delivery.Payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "edge-rollout-control/1.0")
	request.Header.Set("X-Edge-Event", delivery.EventType)
	request.Header.Set("X-Edge-Delivery", delivery.ID)
	request.Header.Set("X-Edge-Timestamp", timestamp)
	request.Header.Set("X-Edge-Signature", "sha256="+signature)
	response, err := d.client.Do(request)
	if err != nil {
		return d.recordFailure(ctx, delivery, 0, err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return d.recordFailure(ctx, delivery, response.StatusCode, fmt.Errorf("unexpected status %d", response.StatusCode))
	}
	delivery.Status = webhook.DeliverySucceeded
	delivery.LastStatusCode = response.StatusCode
	delivery.LastError = ""
	delivery.DeliveredAt = &now
	if err := d.repository.UpdateDelivery(ctx, delivery); err != nil {
		return fmt.Errorf("mark webhook delivered: %w", err)
	}
	return nil
}

func (d *WebhookDispatcher) recordFailure(ctx context.Context, delivery webhook.Delivery, statusCode int, deliveryError error) error {
	delivery.LastStatusCode = statusCode
	delivery.LastError = deliveryError.Error()
	if delivery.Attempts >= d.maxAttempts {
		delivery.Status = webhook.DeliveryAbandoned
	} else {
		delivery.Status = webhook.DeliveryRetrying
		delivery.NextAttemptAt = d.clock.Now().Add(webhookBackoff(delivery.Attempts))
	}
	if err := d.repository.UpdateDelivery(ctx, delivery); err != nil {
		return fmt.Errorf("record webhook failure: %w", err)
	}
	return deliveryError
}

func signWebhook(secret, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func webhookBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	duration := time.Second << min(attempt-1, 10)
	if duration > 30*time.Minute {
		return 30 * time.Minute
	}
	return duration
}
