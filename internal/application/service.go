package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/example/edge-rollout-control/internal/domain/audit"
	"github.com/example/edge-rollout-control/internal/domain/configuration"
	"github.com/example/edge-rollout-control/internal/domain/device"
	"github.com/example/edge-rollout-control/internal/domain/receipt"
	"github.com/example/edge-rollout-control/internal/domain/rollout"
	"github.com/example/edge-rollout-control/internal/domain/rule"
	"github.com/example/edge-rollout-control/internal/domain/webhook"
)

type Service struct {
	repository UnitOfWork
	clock      Clock
	ids        IDGenerator
	logger     *slog.Logger
}

func NewService(repository UnitOfWork, clock Clock, ids IDGenerator, logger *slog.Logger) *Service {
	return &Service{repository: repository, clock: clock, ids: ids, logger: logger}
}

type CreateDeviceCommand struct {
	Name            string            `json:"name"`
	HardwareModel   string            `json:"hardware_model"`
	SoftwareVersion string            `json:"software_version"`
	Labels          map[string]string `json:"labels"`
}

type UpdateDeviceCommand struct {
	Name            *string           `json:"name"`
	SoftwareVersion *string           `json:"software_version"`
	Labels          map[string]string `json:"labels"`
}

type HeartbeatCommand struct {
	SoftwareVersion        string    `json:"software_version"`
	CurrentConfigurationID string    `json:"current_configuration_id"`
	Timestamp              time.Time `json:"timestamp"`
}

func (s *Service) CreateDevice(ctx context.Context, command CreateDeviceCommand, actor string) (device.Device, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.HardwareModel = strings.TrimSpace(command.HardwareModel)
	command.SoftwareVersion = strings.TrimSpace(command.SoftwareVersion)
	if command.Name == "" {
		return device.Device{}, ValidationError{Field: "name", Message: "is required"}
	}
	if command.HardwareModel == "" {
		return device.Device{}, ValidationError{Field: "hardware_model", Message: "is required"}
	}
	if command.SoftwareVersion == "" {
		return device.Device{}, ValidationError{Field: "software_version", Message: "is required"}
	}
	if command.Labels == nil {
		command.Labels = map[string]string{}
	}
	for key, value := range command.Labels {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return device.Device{}, ValidationError{Field: "labels", Message: "keys and values must not be empty"}
		}
	}
	now := s.clock.Now()
	entity := device.Device{
		ID: s.ids.New(), Name: command.Name, HardwareModel: command.HardwareModel,
		SoftwareVersion: command.SoftwareVersion, Labels: cloneLabels(command.Labels),
		Status: device.StatusOffline, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repository.CreateDevice(ctx, entity); err != nil {
		return device.Device{}, fmt.Errorf("create device: %w", err)
	}
	s.audit(ctx, actor, "device.created", "device", entity.ID, map[string]any{"name": entity.Name, "hardware_model": entity.HardwareModel})
	s.emit(ctx, "device.created", entity.ID, entity)
	return entity, nil
}

func (s *Service) UpdateDevice(ctx context.Context, id string, command UpdateDeviceCommand, actor string) (device.Device, error) {
	entity, err := s.repository.GetDevice(ctx, id)
	if err != nil {
		return device.Device{}, err
	}
	if command.Name != nil {
		name := strings.TrimSpace(*command.Name)
		if name == "" {
			return device.Device{}, ValidationError{Field: "name", Message: "must not be empty"}
		}
		entity.Name = name
	}
	if command.SoftwareVersion != nil {
		version := strings.TrimSpace(*command.SoftwareVersion)
		if version == "" {
			return device.Device{}, ValidationError{Field: "software_version", Message: "must not be empty"}
		}
		entity.SoftwareVersion = version
	}
	if command.Labels != nil {
		entity.Labels = cloneLabels(command.Labels)
	}
	entity.UpdatedAt = s.clock.Now()
	if err := s.repository.UpdateDevice(ctx, entity); err != nil {
		return device.Device{}, fmt.Errorf("update device: %w", err)
	}
	s.audit(ctx, actor, "device.updated", "device", entity.ID, map[string]any{"name": entity.Name})
	return entity, nil
}

func (s *Service) Heartbeat(ctx context.Context, id string, command HeartbeatCommand) (device.Device, error) {
	entity, err := s.repository.GetDevice(ctx, id)
	if err != nil {
		return device.Device{}, err
	}
	now := command.Timestamp
	if now.IsZero() {
		now = s.clock.Now()
	}
	if command.SoftwareVersion != "" {
		entity.SoftwareVersion = command.SoftwareVersion
	}
	if command.CurrentConfigurationID != "" {
		entity.CurrentConfigurationID = command.CurrentConfigurationID
	}
	entity.Status = device.StatusOnline
	entity.LastHeartbeatAt = now.UTC()
	entity.UpdatedAt = s.clock.Now()
	if err := s.repository.UpdateDevice(ctx, entity); err != nil {
		return device.Device{}, fmt.Errorf("record heartbeat: %w", err)
	}
	return entity, nil
}

func (s *Service) GetDevice(ctx context.Context, id string) (device.Device, error) {
	return s.repository.GetDevice(ctx, id)
}

func (s *Service) ListDevices(ctx context.Context, filter device.Filter) ([]device.Device, int, error) {
	normalizePage(&filter.Limit, &filter.Offset)
	return s.repository.ListDevices(ctx, filter)
}

type CreateGroupCommand struct {
	Name        string `json:"name"`
	Selector    string `json:"selector"`
	Description string `json:"description"`
}

func (s *Service) CreateGroup(ctx context.Context, command CreateGroupCommand, actor string) (device.Group, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.Selector = strings.TrimSpace(command.Selector)
	if command.Name == "" || command.Selector == "" {
		return device.Group{}, ValidationError{Message: "name and selector are required"}
	}
	if _, err := device.ParseSelector(command.Selector); err != nil {
		return device.Group{}, ValidationError{Field: "selector", Message: err.Error()}
	}
	now := s.clock.Now()
	entity := device.Group{ID: s.ids.New(), Name: command.Name, Selector: command.Selector, Description: command.Description, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.CreateGroup(ctx, entity); err != nil {
		return device.Group{}, fmt.Errorf("create group: %w", err)
	}
	s.audit(ctx, actor, "device_group.created", "device_group", entity.ID, map[string]any{"selector": entity.Selector})
	return entity, nil
}

func (s *Service) ListGroups(ctx context.Context, limit, offset int) ([]device.Group, int, error) {
	normalizePage(&limit, &offset)
	return s.repository.ListGroups(ctx, limit, offset)
}

func (s *Service) ResolveGroup(ctx context.Context, id string) (device.Group, []device.Device, error) {
	group, err := s.repository.GetGroup(ctx, id)
	if err != nil {
		return device.Group{}, nil, err
	}
	items, err := s.repository.ResolveGroup(ctx, id)
	return group, items, err
}

type CreateConfigurationCommand struct {
	Name             string               `json:"name"`
	Format           configuration.Format `json:"format"`
	Content          string               `json:"content"`
	CompatibleModels []string             `json:"compatible_models"`
	Variables        map[string]string    `json:"variables"`
	ChangeSummary    string               `json:"change_summary"`
	CreatedBy        string               `json:"created_by"`
}

func (s *Service) CreateConfiguration(ctx context.Context, command CreateConfigurationCommand, actor string) (configuration.Configuration, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.Content = strings.TrimSpace(command.Content)
	if command.Name == "" || command.Content == "" {
		return configuration.Configuration{}, ValidationError{Message: "name and content are required"}
	}
	if command.Format == "" {
		command.Format = configuration.FormatYAML
	}
	if !configuration.ValidFormat(command.Format) {
		return configuration.Configuration{}, ValidationError{Field: "format", Message: "must be yaml or json"}
	}
	if err := configuration.ValidateContent(command.Format, command.Content); err != nil {
		return configuration.Configuration{}, ValidationError{Field: "content", Message: err.Error()}
	}
	version := 1
	existing, _, err := s.repository.ListConfigurations(ctx, configuration.Filter{Name: command.Name, Limit: 1, Sort: "version_desc"})
	if err == nil && len(existing) > 0 {
		version = existing[0].Version + 1
	}
	digest := sha256.Sum256([]byte(command.Content))
	now := s.clock.Now()
	entity := configuration.Configuration{
		ID: s.ids.New(), Name: command.Name, Version: version, Format: command.Format,
		Content: command.Content, ContentSHA256: hex.EncodeToString(digest[:]),
		CompatibleModels: uniqueStrings(command.CompatibleModels), Variables: cloneLabels(command.Variables),
		ChangeSummary: strings.TrimSpace(command.ChangeSummary), Status: configuration.StatusDraft,
		CreatedBy: fallback(command.CreatedBy, actor), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repository.CreateConfiguration(ctx, entity); err != nil {
		return configuration.Configuration{}, fmt.Errorf("create configuration: %w", err)
	}
	s.audit(ctx, actor, "configuration.created", "configuration", entity.ID, map[string]any{"name": entity.Name, "version": entity.Version, "sha256": entity.ContentSHA256})
	return entity, nil
}

func (s *Service) PublishConfiguration(ctx context.Context, id, actor string) (configuration.Configuration, error) {
	entity, err := s.repository.GetConfiguration(ctx, id)
	if err != nil {
		return configuration.Configuration{}, err
	}
	if entity.Status != configuration.StatusDraft {
		return configuration.Configuration{}, fmt.Errorf("%w: configuration must be draft", ErrInvalidState)
	}
	now := s.clock.Now()
	if err := s.repository.UpdateConfigurationStatus(ctx, id, configuration.StatusPublished, now); err != nil {
		return configuration.Configuration{}, err
	}
	entity.Status = configuration.StatusPublished
	entity.UpdatedAt = now
	s.audit(ctx, actor, "configuration.published", "configuration", entity.ID, map[string]any{"version": entity.Version})
	s.emit(ctx, "configuration.published", entity.ID, entity)
	return entity, nil
}

func (s *Service) DeprecateConfiguration(ctx context.Context, id, actor string) (configuration.Configuration, error) {
	entity, err := s.repository.GetConfiguration(ctx, id)
	if err != nil {
		return configuration.Configuration{}, err
	}
	if entity.Status != configuration.StatusPublished {
		return configuration.Configuration{}, fmt.Errorf("%w: only published configurations can be deprecated", ErrInvalidState)
	}
	now := s.clock.Now()
	if err := s.repository.UpdateConfigurationStatus(ctx, id, configuration.StatusDeprecated, now); err != nil {
		return configuration.Configuration{}, err
	}
	entity.Status = configuration.StatusDeprecated
	entity.UpdatedAt = now
	s.audit(ctx, actor, "configuration.deprecated", "configuration", entity.ID, nil)
	return entity, nil
}

func (s *Service) GetConfiguration(ctx context.Context, id string) (configuration.Configuration, error) {
	return s.repository.GetConfiguration(ctx, id)
}

func (s *Service) ListConfigurations(ctx context.Context, filter configuration.Filter) ([]configuration.Configuration, int, error) {
	normalizePage(&filter.Limit, &filter.Offset)
	return s.repository.ListConfigurations(ctx, filter)
}

type CreateRuleCommand struct {
	Name           string            `json:"name"`
	MaxFailureRate float64           `json:"max_failure_rate"`
	MaxOfflineRate float64           `json:"max_offline_rate"`
	MaxTimeoutRate float64           `json:"max_timeout_rate"`
	CriticalLabels map[string]string `json:"critical_labels"`
	Enabled        *bool             `json:"enabled"`
}

func (s *Service) CreateRule(ctx context.Context, command CreateRuleCommand, actor string) (rule.Rule, error) {
	if strings.TrimSpace(command.Name) == "" {
		return rule.Rule{}, ValidationError{Field: "name", Message: "is required"}
	}
	for field, value := range map[string]float64{"max_failure_rate": command.MaxFailureRate, "max_offline_rate": command.MaxOfflineRate, "max_timeout_rate": command.MaxTimeoutRate} {
		if value < 0 || value > 1 {
			return rule.Rule{}, ValidationError{Field: field, Message: "must be between 0 and 1"}
		}
	}
	enabled := true
	if command.Enabled != nil {
		enabled = *command.Enabled
	}
	now := s.clock.Now()
	entity := rule.Rule{ID: s.ids.New(), Name: strings.TrimSpace(command.Name), MaxFailureRate: command.MaxFailureRate, MaxOfflineRate: command.MaxOfflineRate, MaxTimeoutRate: command.MaxTimeoutRate, CriticalLabels: cloneLabels(command.CriticalLabels), Enabled: enabled, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.CreateRule(ctx, entity); err != nil {
		return rule.Rule{}, err
	}
	s.audit(ctx, actor, "rule.created", "rule", entity.ID, map[string]any{"name": entity.Name})
	return entity, nil
}

func (s *Service) ListRules(ctx context.Context, filter rule.Filter) ([]rule.Rule, int, error) {
	normalizePage(&filter.Limit, &filter.Offset)
	return s.repository.ListRules(ctx, filter)
}

type CreateRolloutCommand struct {
	Name                    string           `json:"name"`
	ConfigurationID         string           `json:"configuration_id"`
	GroupID                 string           `json:"group_id"`
	DeviceIDs               []string         `json:"device_ids"`
	Strategy                rollout.Strategy `json:"strategy"`
	BatchSize               int              `json:"batch_size"`
	BatchPercent            int              `json:"batch_percent"`
	ScheduledAt             *time.Time       `json:"scheduled_at"`
	RuleID                  string           `json:"rule_id"`
	RollbackConfigurationID string           `json:"rollback_configuration_id"`
	CreatedBy               string           `json:"created_by"`
}

func (s *Service) CreateRollout(ctx context.Context, command CreateRolloutCommand, actor string) (rollout.Rollout, error) {
	if strings.TrimSpace(command.Name) == "" || command.ConfigurationID == "" {
		return rollout.Rollout{}, ValidationError{Message: "name and configuration_id are required"}
	}
	configurationEntity, err := s.repository.GetConfiguration(ctx, command.ConfigurationID)
	if err != nil {
		return rollout.Rollout{}, fmt.Errorf("load configuration: %w", err)
	}
	if configurationEntity.Status != configuration.StatusPublished {
		return rollout.Rollout{}, fmt.Errorf("%w: configuration must be published", ErrInvalidState)
	}
	devices, err := s.resolveTargets(ctx, command.GroupID, command.DeviceIDs)
	if err != nil {
		return rollout.Rollout{}, err
	}
	if len(devices) == 0 {
		return rollout.Rollout{}, ValidationError{Field: "targets", Message: "at least one device is required"}
	}
	compatible := map[string]struct{}{}
	for _, model := range configurationEntity.CompatibleModels {
		compatible[model] = struct{}{}
	}
	if len(compatible) > 0 {
		for _, target := range devices {
			if _, ok := compatible[target.HardwareModel]; !ok {
				return rollout.Rollout{}, ValidationError{Field: "targets", Message: fmt.Sprintf("device %s model %s is incompatible", target.ID, target.HardwareModel)}
			}
		}
	}
	if command.Strategy == "" {
		command.Strategy = rollout.StrategyImmediate
	}
	if !rollout.ValidStrategy(command.Strategy) {
		return rollout.Rollout{}, ValidationError{Field: "strategy", Message: "unsupported strategy"}
	}
	batchSize := command.BatchSize
	if command.BatchPercent > 0 {
		batchSize = int(math.Ceil(float64(len(devices)) * float64(command.BatchPercent) / 100))
	}
	if batchSize <= 0 || batchSize > len(devices) {
		batchSize = len(devices)
	}
	now := s.clock.Now()
	entity := rollout.Rollout{
		ID: s.ids.New(), Name: strings.TrimSpace(command.Name), ConfigurationID: command.ConfigurationID,
		GroupID: command.GroupID, Strategy: command.Strategy, BatchSize: batchSize, BatchPercent: command.BatchPercent,
		ScheduledAt: command.ScheduledAt, Status: rollout.StatusPending, CurrentBatch: 0, TargetCount: len(devices),
		RuleID: command.RuleID, RollbackConfigurationID: command.RollbackConfigurationID,
		CreatedBy: fallback(command.CreatedBy, actor), CreatedAt: now, UpdatedAt: now,
	}
	if command.Strategy == rollout.StrategyScheduled && (command.ScheduledAt == nil || command.ScheduledAt.IsZero()) {
		return rollout.Rollout{}, ValidationError{Field: "scheduled_at", Message: "is required for scheduled rollout"}
	}
	targets := make([]rollout.Target, 0, len(devices))
	for index, targetDevice := range devices {
		targets = append(targets, rollout.Target{ID: s.ids.New(), RolloutID: entity.ID, DeviceID: targetDevice.ID, BatchNumber: index/batchSize + 1, Status: rollout.TargetPending, DesiredConfigurationID: entity.ConfigurationID, CreatedAt: now, UpdatedAt: now})
	}
	if err := s.repository.CreateRollout(ctx, entity, targets); err != nil {
		return rollout.Rollout{}, WrapServiceError(strings.TrimSpace("create rollout"), err)
	}
	s.audit(ctx, actor, "rollout.created", "rollout", entity.ID, map[string]any{"target_count": len(targets), "batch_size": batchSize})
	s.emit(ctx, "rollout.created", entity.ID, entity)
	return entity, nil
}

func RolloutErrorContract(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("create rollout: %w", err)
}

func PreserveRolloutCause(operation string, err error) (error, string) {
	wrapped := fmt.Errorf("%s: %w", operation, err)
	return wrapped, ClassifyServiceError(wrapped)
}

func (s *Service) resolveTargets(ctx context.Context, groupID string, ids []string) ([]device.Device, error) {
	byID := map[string]device.Device{}
	if groupID != "" {
		items, err := s.repository.ResolveGroup(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf("resolve group: %w", err)
		}
		for _, item := range items {
			byID[item.ID] = item
		}
	}
	for _, id := range ids {
		item, err := s.repository.GetDevice(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load target device %s: %w", id, err)
		}
		byID[id] = item
	}
	items := make([]device.Device, 0, len(byID))
	for _, item := range byID {
		if item.Status == device.StatusRetired {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (s *Service) GetRollout(ctx context.Context, id string) (rollout.Rollout, error) {
	return s.repository.GetRollout(ctx, id)
}

func (s *Service) ListRollouts(ctx context.Context, filter rollout.Filter) ([]rollout.Rollout, int, error) {
	normalizePage(&filter.Limit, &filter.Offset)
	return s.repository.ListRollouts(ctx, filter)
}

func (s *Service) ListTargets(ctx context.Context, id string, filter rollout.TargetFilter) ([]rollout.Target, int, error) {
	normalizePage(&filter.Limit, &filter.Offset)
	return s.repository.ListTargets(ctx, id, filter)
}

func (s *Service) StartRollout(ctx context.Context, id, actor string) (rollout.Rollout, error) {
	entity, err := s.repository.GetRollout(ctx, id)
	if err != nil {
		return rollout.Rollout{}, err
	}
	if entity.Status != rollout.StatusPending && entity.Status != rollout.StatusPaused {
		return rollout.Rollout{}, fmt.Errorf("%w: rollout cannot be started from %s", ErrInvalidState, entity.Status)
	}
	now := s.clock.Now()
	if entity.Status == rollout.StatusPending {
		entity.StartedAt = &now
		entity.CurrentBatch = 1
	}
	entity.Status = rollout.StatusRunning
	entity.PauseReason = ""
	entity.UpdatedAt = now
	if err := s.activateBatch(ctx, &entity, entity.CurrentBatch); err != nil {
		return rollout.Rollout{}, err
	}
	if err := s.repository.UpdateRollout(ctx, entity); err != nil {
		return rollout.Rollout{}, err
	}
	s.audit(ctx, actor, "rollout.started", "rollout", entity.ID, map[string]any{"batch": entity.CurrentBatch})
	s.emit(ctx, "rollout.started", entity.ID, entity)
	return entity, nil
}

func (s *Service) PauseRollout(ctx context.Context, id, reason, actor string) (rollout.Rollout, error) {
	entity, err := s.repository.GetRollout(ctx, id)
	if err != nil {
		return rollout.Rollout{}, err
	}
	if entity.Status != rollout.StatusRunning {
		return rollout.Rollout{}, fmt.Errorf("%w: rollout is not running", ErrInvalidState)
	}
	entity.Status = rollout.StatusPaused
	entity.PauseReason = fallback(strings.TrimSpace(reason), "paused by operator")
	entity.UpdatedAt = s.clock.Now()
	if err := s.repository.UpdateRollout(ctx, entity); err != nil {
		return rollout.Rollout{}, err
	}
	s.audit(ctx, actor, "rollout.paused", "rollout", entity.ID, map[string]any{"reason": entity.PauseReason})
	s.emit(ctx, "rollout.paused", entity.ID, entity)
	return entity, nil
}

func (s *Service) CancelRollout(ctx context.Context, id, actor string) (rollout.Rollout, error) {
	entity, err := s.repository.GetRollout(ctx, id)
	if err != nil {
		return rollout.Rollout{}, err
	}
	if entity.Status.Terminal() {
		return rollout.Rollout{}, fmt.Errorf("%w: rollout is already terminal", ErrInvalidState)
	}
	entity.Status = rollout.StatusCancelled
	entity.UpdatedAt = s.clock.Now()
	if err := s.repository.UpdateRollout(ctx, entity); err != nil {
		return rollout.Rollout{}, err
	}
	s.audit(ctx, actor, "rollout.cancelled", "rollout", entity.ID, nil)
	s.emit(ctx, "rollout.cancelled", entity.ID, entity)
	return entity, nil
}

func (s *Service) RollbackRollout(ctx context.Context, id, actor string) (rollout.Rollout, error) {
	entity, err := s.repository.GetRollout(ctx, id)
	if err != nil {
		return rollout.Rollout{}, err
	}
	if entity.RollbackConfigurationID == "" {
		return rollout.Rollout{}, ValidationError{Field: "rollback_configuration_id", Message: "rollout has no rollback version"}
	}
	if entity.Status != rollout.StatusPaused && entity.Status != rollout.StatusFailed && entity.Status != rollout.StatusCompleted {
		return rollout.Rollout{}, fmt.Errorf("%w: rollout cannot be rolled back from %s", ErrInvalidState, entity.Status)
	}
	now := s.clock.Now()
	targets, _, err := s.repository.ListTargets(ctx, entity.ID, rollout.TargetFilter{Limit: 10000})
	if err != nil {
		return rollout.Rollout{}, err
	}
	for _, target := range targets {
		if target.Status == rollout.TargetSucceeded || target.Status == rollout.TargetFailed {
			target.Status = rollout.TargetRollbackPending
			target.DesiredConfigurationID = entity.RollbackConfigurationID
			target.UpdatedAt = now
			if err := s.repository.UpdateTarget(ctx, target); err != nil {
				return rollout.Rollout{}, err
			}
		}
	}
	entity.Status = rollout.StatusRollingBack
	entity.UpdatedAt = now
	if err := s.repository.UpdateRollout(ctx, entity); err != nil {
		return rollout.Rollout{}, err
	}
	s.audit(ctx, actor, "rollout.rollback_started", "rollout", entity.ID, map[string]any{"configuration_id": entity.RollbackConfigurationID})
	s.emit(ctx, "rollout.rollback_started", entity.ID, entity)
	return entity, nil
}

func (s *Service) activateBatch(ctx context.Context, entity *rollout.Rollout, batch int) error {
	targets, _, err := s.repository.ListTargets(ctx, entity.ID, rollout.TargetFilter{BatchNumber: batch, Limit: 10000})
	if err != nil {
		return fmt.Errorf("list rollout batch: %w", err)
	}
	now := s.clock.Now()
	for _, target := range targets {
		if target.Status != rollout.TargetPending {
			continue
		}
		target.Status = rollout.TargetReady
		target.UpdatedAt = now
		if err := s.repository.UpdateTarget(ctx, target); err != nil {
			return fmt.Errorf("activate target: %w", err)
		}
	}
	return nil
}

type PendingConfiguration struct {
	Rollout       rollout.Rollout             `json:"rollout"`
	Target        rollout.Target              `json:"target"`
	Configuration configuration.Configuration `json:"configuration"`
}

func (s *Service) PullPendingConfiguration(ctx context.Context, deviceID string) (*PendingConfiguration, error) {
	_, err := s.repository.GetDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	items, _, err := s.repository.ListRollouts(ctx, rollout.Filter{Status: rollout.StatusRunning, Limit: 200})
	if err != nil {
		return nil, err
	}
	for _, rolloutEntity := range items {
		target, err := s.repository.GetTarget(ctx, rolloutEntity.ID, deviceID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		if target.Status != rollout.TargetReady && target.Status != rollout.TargetRollbackPending {
			continue
		}
		configurationEntity, err := s.repository.GetConfiguration(ctx, target.DesiredConfigurationID)
		if err != nil {
			return nil, err
		}
		now := s.clock.Now()
		target.Status = rollout.TargetDelivered
		target.DeliveredAt = &now
		target.UpdatedAt = now
		if err := s.repository.UpdateTarget(ctx, target); err != nil {
			return nil, err
		}
		return &PendingConfiguration{Rollout: rolloutEntity, Target: target, Configuration: configurationEntity}, nil
	}
	return nil, nil
}

type SubmitReceiptCommand struct {
	IDempotencyKey  string         `json:"idempotency_key"`
	RolloutID       string         `json:"rollout_id"`
	ConfigurationID string         `json:"configuration_id"`
	Status          receipt.Status `json:"status"`
	ErrorCode       string         `json:"error_code"`
	Message         string         `json:"message"`
	DeviceTimestamp *time.Time     `json:"device_timestamp"`
}

func (s *Service) SubmitReceipt(ctx context.Context, deviceID string, command SubmitReceiptCommand) (receipt.Receipt, bool, error) {
	if command.IDempotencyKey == "" || command.RolloutID == "" || command.ConfigurationID == "" {
		return receipt.Receipt{}, false, ValidationError{Message: "idempotency_key, rollout_id and configuration_id are required"}
	}
	if command.Status != receipt.StatusSucceeded && command.Status != receipt.StatusFailed {
		return receipt.Receipt{}, false, ValidationError{Field: "status", Message: "must be succeeded or failed"}
	}
	target, err := s.repository.GetTarget(ctx, command.RolloutID, deviceID)
	if err != nil {
		return receipt.Receipt{}, false, err
	}
	if target.DesiredConfigurationID != command.ConfigurationID {
		return receipt.Receipt{}, false, fmt.Errorf("%w: receipt configuration does not match target", ErrConflict)
	}
	now := s.clock.Now()
	entity := receipt.Receipt{ID: s.ids.New(), IDempotencyKey: command.IDempotencyKey, RolloutID: command.RolloutID, DeviceID: deviceID, ConfigurationID: command.ConfigurationID, Status: command.Status, ErrorCode: command.ErrorCode, Message: command.Message, DeviceTimestamp: command.DeviceTimestamp, ReceivedAt: now}
	created, err := s.repository.CreateReceipt(ctx, entity)
	if err != nil {
		return receipt.Receipt{}, false, fmt.Errorf("create receipt: %w", err)
	}
	if !created {
		return entity, false, nil
	}
	if command.Status == receipt.StatusSucceeded {
		target.Status = rollout.TargetSucceeded
	} else {
		target.Status = rollout.TargetFailed
	}
	target.AcknowledgedAt = &now
	target.ErrorCode = command.ErrorCode
	target.ErrorMessage = command.Message
	target.UpdatedAt = now
	if err := s.repository.UpdateTarget(ctx, target); err != nil {
		return receipt.Receipt{}, false, err
	}
	if command.Status == receipt.StatusSucceeded {
		deviceEntity, err := s.repository.GetDevice(ctx, deviceID)
		if err == nil {
			deviceEntity.CurrentConfigurationID = command.ConfigurationID
			deviceEntity.UpdatedAt = now
			_ = s.repository.UpdateDevice(ctx, deviceEntity)
		}
	}
	s.audit(ctx, deviceID, "receipt.accepted", "rollout", command.RolloutID, map[string]any{"status": command.Status, "device_id": deviceID})
	s.emit(ctx, "receipt.accepted", command.RolloutID, entity)
	if err := s.ReconcileRollout(ctx, command.RolloutID); err != nil {
		s.logger.ErrorContext(ctx, "reconcile rollout after receipt", "error", err, "rollout_id", command.RolloutID)
	}
	return entity, true, nil
}

func (s *Service) ReconcileRollout(ctx context.Context, id string) error {
	entity, err := s.repository.GetRollout(ctx, id)
	if err != nil {
		return err
	}
	if entity.Status != rollout.StatusRunning && entity.Status != rollout.StatusRollingBack {
		return nil
	}
	counts, err := s.repository.CountTargetStates(ctx, id, entity.CurrentBatch)
	if err != nil {
		return err
	}
	total := 0
	terminal := 0
	for state, count := range counts {
		total += count
		if state == rollout.TargetSucceeded || state == rollout.TargetFailed || state == rollout.TargetTimedOut || state == rollout.TargetRolledBack {
			terminal += count
		}
	}
	entity.SuccessCount, entity.FailureCount, entity.TimeoutCount = counts[rollout.TargetSucceeded], counts[rollout.TargetFailed], counts[rollout.TargetTimedOut]
	allCounts, err := s.repository.CountTargetStates(ctx, id, 0)
	if err == nil {
		entity.SuccessCount = allCounts[rollout.TargetSucceeded]
		entity.FailureCount = allCounts[rollout.TargetFailed]
		entity.TimeoutCount = allCounts[rollout.TargetTimedOut]
	}
	if entity.RuleID != "" {
		ruleEntity, err := s.repository.GetRule(ctx, entity.RuleID)
		if err == nil && ruleEntity.Enabled {
			pause, reason, decisionErr := ruleEntity.Evaluate(entity.FailureCount, entity.SuccessCount+entity.FailureCount+entity.TimeoutCount, entity.TimeoutCount, entity.TargetCount)
			if decisionErr != nil {
				s.logger.WarnContext(ctx, "evaluate rollout rule", "error", decisionErr, "rollout_id", entity.ID)
			} else if pause {
				entity.Status = rollout.StatusPaused
				entity.PauseReason = reason
				entity.UpdatedAt = s.clock.Now()
				_ = s.repository.UpdateRollout(ctx, entity)
				s.audit(ctx, "system", "rollout.auto_paused", "rollout", entity.ID, map[string]any{"reason": reason})
				s.emit(ctx, "rollout.auto_paused", entity.ID, entity)
				return nil
			}
		}
	}
	if total == 0 || terminal < total {
		entity.UpdatedAt = s.clock.Now()
		return s.repository.UpdateRollout(ctx, entity)
	}
	maxBatch := int(math.Ceil(float64(entity.TargetCount) / float64(entity.BatchSize)))
	if entity.Status == rollout.StatusRollingBack {
		entity.Status = rollout.StatusRolledBack
		now := s.clock.Now()
		entity.CompletedAt = &now
		entity.UpdatedAt = now
		return s.repository.UpdateRollout(ctx, entity)
	}
	if entity.CurrentBatch < maxBatch {
		entity.CurrentBatch++
		entity.UpdatedAt = s.clock.Now()
		if err := s.activateBatch(ctx, &entity, entity.CurrentBatch); err != nil {
			return err
		}
		return s.repository.UpdateRollout(ctx, entity)
	}
	entity.Status = rollout.StatusCompleted
	now := s.clock.Now()
	entity.CompletedAt = &now
	entity.UpdatedAt = now
	if err := s.repository.UpdateRollout(ctx, entity); err != nil {
		return err
	}
	s.audit(ctx, "system", "rollout.completed", "rollout", entity.ID, map[string]any{"success": entity.SuccessCount, "failed": entity.FailureCount})
	s.emit(ctx, "rollout.completed", entity.ID, entity)
	return nil
}

func (s *Service) ProcessRunnableRollouts(ctx context.Context, limit int) error {
	items, err := s.repository.ListRunnableRollouts(ctx, s.clock.Now(), limit)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Status == rollout.StatusPending {
			if _, err := s.StartRollout(ctx, item.ID, "scheduler"); err != nil {
				s.logger.ErrorContext(ctx, "start scheduled rollout", "error", err, "rollout_id", item.ID)
			}
		} else if err := s.ReconcileRollout(ctx, item.ID); err != nil {
			s.logger.ErrorContext(ctx, "reconcile rollout", "error", err, "rollout_id", item.ID)
		}
	}
	return nil
}

func (s *Service) ListReceipts(ctx context.Context, filter receipt.Filter) ([]receipt.Receipt, int, error) {
	normalizePage(&filter.Limit, &filter.Offset)
	return s.repository.ListReceipts(ctx, filter)
}

func (s *Service) ListAudit(ctx context.Context, filter audit.Filter) ([]audit.Event, int, error) {
	normalizePage(&filter.Limit, &filter.Offset)
	return s.repository.ListAudit(ctx, filter)
}

type CreateSubscriptionCommand struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Secret     string   `json:"secret"`
	EventTypes []string `json:"event_types"`
	Enabled    *bool    `json:"enabled"`
}

func (s *Service) CreateSubscription(ctx context.Context, command CreateSubscriptionCommand, actor string) (webhook.Subscription, error) {
	if err := webhook.ValidateURL(command.URL); err != nil {
		return webhook.Subscription{}, ValidationError{Field: "url", Message: err.Error()}
	}
	if strings.TrimSpace(command.Name) == "" || strings.TrimSpace(command.Secret) == "" {
		return webhook.Subscription{}, ValidationError{Message: "name and secret are required"}
	}
	enabled := true
	if command.Enabled != nil {
		enabled = *command.Enabled
	}
	now := s.clock.Now()
	entity := webhook.Subscription{ID: s.ids.New(), Name: command.Name, URL: command.URL, Secret: command.Secret, EventTypes: uniqueStrings(command.EventTypes), Enabled: enabled, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.CreateSubscription(ctx, entity); err != nil {
		return webhook.Subscription{}, err
	}
	s.audit(ctx, actor, "webhook.created", "webhook", entity.ID, map[string]any{"url": entity.URL})
	return entity, nil
}

func (s *Service) ListSubscriptions(ctx context.Context, filter webhook.SubscriptionFilter) ([]webhook.Subscription, int, error) {
	normalizePage(&filter.Limit, &filter.Offset)
	return s.repository.ListSubscriptions(ctx, filter)
}

func (s *Service) ListDeliveries(ctx context.Context, filter webhook.DeliveryFilter) ([]webhook.Delivery, int, error) {
	normalizePage(&filter.Limit, &filter.Offset)
	return s.repository.ListDeliveries(ctx, filter)
}

func (s *Service) emit(ctx context.Context, eventType, aggregateID string, payload any) {
	subscriptions, _, err := s.repository.ListSubscriptions(ctx, webhook.SubscriptionFilter{EnabledOnly: true, EventType: eventType, Limit: 1000})
	if err != nil {
		s.logger.WarnContext(ctx, "list webhook subscriptions", "error", err)
		return
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		s.logger.WarnContext(ctx, "marshal webhook payload", "error", err)
		return
	}
	now := s.clock.Now()
	for _, subscription := range subscriptions {
		delivery := webhook.Delivery{ID: s.ids.New(), SubscriptionID: subscription.ID, EventType: eventType, AggregateID: aggregateID, Payload: string(encoded), Status: webhook.DeliveryPending, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now}
		if err := s.repository.EnqueueDelivery(ctx, delivery); err != nil {
			s.logger.WarnContext(ctx, "enqueue webhook", "error", err, "subscription_id", subscription.ID)
		}
	}
}

func (s *Service) audit(ctx context.Context, actor, action, resourceType, resourceID string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	event := audit.Event{ID: s.ids.New(), Actor: fallback(actor, "anonymous"), Action: action, ResourceType: resourceType, ResourceID: resourceID, Details: details, CreatedAt: s.clock.Now()}
	if err := s.repository.AppendAudit(ctx, event); err != nil {
		s.logger.WarnContext(ctx, "append audit", "error", err, "action", action, "resource_id", resourceID)
	}
}

func normalizePage(limit, offset *int) {
	if *limit <= 0 {
		*limit = 50
	}
	if *limit > 1000 {
		*limit = 1000
	}
	if *offset < 0 {
		*offset = 0
	}
}

func cloneLabels(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
