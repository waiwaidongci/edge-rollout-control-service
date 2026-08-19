package application

import (
	"context"
	"time"

	"github.com/example/edge-rollout-control/internal/domain/audit"
	"github.com/example/edge-rollout-control/internal/domain/configuration"
	"github.com/example/edge-rollout-control/internal/domain/device"
	"github.com/example/edge-rollout-control/internal/domain/receipt"
	"github.com/example/edge-rollout-control/internal/domain/rollout"
	"github.com/example/edge-rollout-control/internal/domain/rule"
	"github.com/example/edge-rollout-control/internal/domain/webhook"
)

type DeviceRepository interface {
	CreateDevice(context.Context, device.Device) error
	UpdateDevice(context.Context, device.Device) error
	GetDevice(context.Context, string) (device.Device, error)
	ListDevices(context.Context, device.Filter) ([]device.Device, int, error)
	CreateGroup(context.Context, device.Group) error
	GetGroup(context.Context, string) (device.Group, error)
	ListGroups(context.Context, int, int) ([]device.Group, int, error)
	ResolveGroup(context.Context, string) ([]device.Device, error)
}

type ConfigurationRepository interface {
	CreateConfiguration(context.Context, configuration.Configuration) error
	GetConfiguration(context.Context, string) (configuration.Configuration, error)
	ListConfigurations(context.Context, configuration.Filter) ([]configuration.Configuration, int, error)
	UpdateConfigurationStatus(context.Context, string, configuration.Status, time.Time) error
}

type RolloutRepository interface {
	CreateRollout(context.Context, rollout.Rollout, []rollout.Target) error
	GetRollout(context.Context, string) (rollout.Rollout, error)
	UpdateRollout(context.Context, rollout.Rollout) error
	ListRollouts(context.Context, rollout.Filter) ([]rollout.Rollout, int, error)
	ListRunnableRollouts(context.Context, time.Time, int) ([]rollout.Rollout, error)
	ListTargets(context.Context, string, rollout.TargetFilter) ([]rollout.Target, int, error)
	GetTarget(context.Context, string, string) (rollout.Target, error)
	UpdateTarget(context.Context, rollout.Target) error
	CountTargetStates(context.Context, string, int) (map[rollout.TargetStatus]int, error)
}

type ReceiptRepository interface {
	CreateReceipt(context.Context, receipt.Receipt) (bool, error)
	ListReceipts(context.Context, receipt.Filter) ([]receipt.Receipt, int, error)
}

type RuleRepository interface {
	CreateRule(context.Context, rule.Rule) error
	UpdateRule(context.Context, rule.Rule) error
	GetRule(context.Context, string) (rule.Rule, error)
	ListRules(context.Context, rule.Filter) ([]rule.Rule, int, error)
}

type AuditRepository interface {
	AppendAudit(context.Context, audit.Event) error
	ListAudit(context.Context, audit.Filter) ([]audit.Event, int, error)
}

type WebhookRepository interface {
	CreateSubscription(context.Context, webhook.Subscription) error
	UpdateSubscription(context.Context, webhook.Subscription) error
	GetSubscription(context.Context, string) (webhook.Subscription, error)
	ListSubscriptions(context.Context, webhook.SubscriptionFilter) ([]webhook.Subscription, int, error)
	EnqueueDelivery(context.Context, webhook.Delivery) error
	ListDueDeliveries(context.Context, time.Time, int) ([]webhook.Delivery, error)
	UpdateDelivery(context.Context, webhook.Delivery) error
	ListDeliveries(context.Context, webhook.DeliveryFilter) ([]webhook.Delivery, int, error)
}

type UnitOfWork interface {
	DeviceRepository
	ConfigurationRepository
	RolloutRepository
	ReceiptRepository
	RuleRepository
	AuditRepository
	WebhookRepository
	Ping(context.Context) error
	Close() error
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	New() string
}
