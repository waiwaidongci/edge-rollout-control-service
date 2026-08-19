package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/example/edge-rollout-control/internal/domain/configuration"
	"github.com/example/edge-rollout-control/internal/domain/device"
	"github.com/example/edge-rollout-control/internal/domain/receipt"
	"github.com/example/edge-rollout-control/internal/domain/rollout"
	"github.com/example/edge-rollout-control/internal/domain/rule"
)

type RolloutReport struct {
	Rollout       rollout.Rollout             `json:"rollout"`
	Targets       []rollout.Target            `json:"targets"`
	Progress      rollout.Progress            `json:"progress"`
	Configuration configuration.Configuration `json:"configuration"`
	Rule          *rule.Rule                  `json:"rule,omitempty"`
	GeneratedAt   time.Time                   `json:"generated_at"`
}

func (s *Service) BuildRolloutReport(ctx context.Context, id string) (RolloutReport, error) {
	entity, err := s.repository.GetRollout(ctx, id)
	if err != nil {
		return RolloutReport{}, err
	}
	targets, _, err := s.repository.ListTargets(ctx, id, rollout.TargetFilter{Limit: 10000})
	if err != nil {
		return RolloutReport{}, err
	}
	configurationEntity, err := s.repository.GetConfiguration(ctx, entity.ConfigurationID)
	if err != nil {
		return RolloutReport{}, err
	}
	var ruleEntity *rule.Rule
	if entity.RuleID != "" {
		value, ruleErr := s.repository.GetRule(ctx, entity.RuleID)
		if ruleErr == nil {
			ruleEntity = &value
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].BatchNumber == targets[j].BatchNumber {
			return targets[i].DeviceID < targets[j].DeviceID
		}
		return targets[i].BatchNumber < targets[j].BatchNumber
	})
	return RolloutReport{Rollout: entity, Targets: targets, Progress: rollout.BuildProgress(targets), Configuration: configurationEntity, Rule: ruleEntity, GeneratedAt: s.clock.Now()}, nil
}

type DeviceSummary struct {
	Device         device.Device                 `json:"device"`
	Rollouts       int                           `json:"rollouts"`
	Receipts       int                           `json:"receipts"`
	LastReceipt    *receipt.Receipt              `json:"last_receipt,omitempty"`
	Configurations []configuration.Configuration `json:"configurations"`
}

func (s *Service) BuildDeviceSummary(ctx context.Context, id string) (DeviceSummary, error) {
	entity, err := s.repository.GetDevice(ctx, id)
	if err != nil {
		return DeviceSummary{}, err
	}
	receipts, _, err := s.repository.ListReceipts(ctx, receipt.Filter{DeviceID: id, Limit: 1000})
	if err != nil {
		return DeviceSummary{}, err
	}
	rollouts, _, err := s.repository.ListRollouts(ctx, rollout.Filter{Limit: 1000})
	if err != nil {
		return DeviceSummary{}, err
	}
	configurationMap := map[string]configuration.Configuration{}
	for _, item := range rollouts {
		target, targetErr := s.repository.GetTarget(ctx, item.ID, id)
		if targetErr != nil {
			continue
		}
		value, configErr := s.repository.GetConfiguration(ctx, target.DesiredConfigurationID)
		if configErr == nil {
			configurationMap[value.ID] = value
		}
	}
	configurations := make([]configuration.Configuration, 0, len(configurationMap))
	for _, value := range configurationMap {
		configurations = append(configurations, value)
	}
	sort.Slice(configurations, func(i, j int) bool { return configurations[i].CreatedAt.Before(configurations[j].CreatedAt) })
	result := DeviceSummary{Device: entity, Rollouts: len(rollouts), Receipts: len(receipts), Configurations: configurations}
	if len(receipts) > 0 {
		last := receipts[0]
		for _, item := range receipts[1:] {
			if item.ReceivedAt.After(last.ReceivedAt) {
				last = item
			}
		}
		result.LastReceipt = &last
	}
	return result, nil
}

type Dashboard struct {
	GeneratedAt       time.Time `json:"generated_at"`
	DeviceCount       int       `json:"device_count"`
	OnlineDevices     int       `json:"online_devices"`
	OfflineDevices    int       `json:"offline_devices"`
	ActiveRollouts    int       `json:"active_rollouts"`
	PausedRollouts    int       `json:"paused_rollouts"`
	CompletedRollouts int       `json:"completed_rollouts"`
	PendingReceipts   int       `json:"pending_receipts"`
	FailedReceipts    int       `json:"failed_receipts"`
	FailureRate       float64   `json:"failure_rate"`
}

func (s *Service) BuildDashboard(ctx context.Context) (Dashboard, error) {
	devices, _, err := s.repository.ListDevices(ctx, device.Filter{Limit: 1000})
	if err != nil {
		return Dashboard{}, err
	}
	rollouts, _, err := s.repository.ListRollouts(ctx, rollout.Filter{Limit: 1000})
	if err != nil {
		return Dashboard{}, err
	}
	receipts, _, err := s.repository.ListReceipts(ctx, receipt.Filter{Limit: 1000})
	if err != nil {
		return Dashboard{}, err
	}
	dashboard := Dashboard{GeneratedAt: s.clock.Now(), DeviceCount: len(devices)}
	for _, entity := range devices {
		if entity.Status == device.StatusOnline {
			dashboard.OnlineDevices++
		} else if entity.Status == device.StatusOffline {
			dashboard.OfflineDevices++
		}
	}
	for _, entity := range rollouts {
		switch entity.Status {
		case rollout.StatusRunning, rollout.StatusScheduled, rollout.StatusPending:
			dashboard.ActiveRollouts++
		case rollout.StatusPaused:
			dashboard.PausedRollouts++
		case rollout.StatusCompleted:
			dashboard.CompletedRollouts++
		}
	}
	for _, entity := range receipts {
		switch entity.Status {
		case receipt.StatusTimeout:
			dashboard.PendingReceipts++
		case receipt.StatusFailed:
			dashboard.FailedReceipts++
		}
	}
	if len(receipts) > 0 {
		failed := 0
		for _, entity := range receipts {
			if entity.Status == receipt.StatusFailed || entity.Status == receipt.StatusTimeout {
				failed++
			}
		}
		dashboard.FailureRate = float64(failed) / float64(len(receipts))
	}
	return dashboard, nil
}

func MarshalReport(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}
	return append(data, '\n'), nil
}

func ValidateFilterText(value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > max {
		return "", fmt.Errorf("filter exceeds %d characters", max)
	}
	return value, nil
}

func NormalizeIDs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("id must not be empty")
		}
		if len(value) > 128 {
			return nil, fmt.Errorf("id exceeds 128 characters")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}
