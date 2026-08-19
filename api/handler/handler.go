package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/edge-rollout-control/api"
	"github.com/example/edge-rollout-control/api/middleware"
	"github.com/example/edge-rollout-control/internal/application"
	"github.com/example/edge-rollout-control/internal/domain/audit"
	"github.com/example/edge-rollout-control/internal/domain/configuration"
	"github.com/example/edge-rollout-control/internal/domain/device"
	"github.com/example/edge-rollout-control/internal/domain/receipt"
	"github.com/example/edge-rollout-control/internal/domain/rollout"
	"github.com/example/edge-rollout-control/internal/domain/rule"
	"github.com/example/edge-rollout-control/internal/domain/webhook"
)

type Handler struct {
	service   *application.Service
	health    func() error
	logger    *slog.Logger
	startedAt time.Time
	version   string
	commit    string
}

func New(service *application.Service, health func() error, logger *slog.Logger, version, commit string) *Handler {
	return &Handler{service: service, health: health, logger: logger, startedAt: time.Now().UTC(), version: version, commit: commit}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /console", h.console)
	mux.HandleFunc("GET /console/", h.console)
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /readyz", h.readyz)
	mux.HandleFunc("GET /metrics", h.metrics)
	mux.HandleFunc("GET /openapi.yaml", h.openapi)

	mux.HandleFunc("POST /v1/devices", h.createDevice)
	mux.HandleFunc("GET /v1/devices", h.listDevices)
	mux.HandleFunc("GET /v1/devices/{deviceID}", h.getDevice)
	mux.HandleFunc("PATCH /v1/devices/{deviceID}", h.updateDevice)
	mux.HandleFunc("POST /v1/devices/{deviceID}/heartbeat", h.heartbeat)
	mux.HandleFunc("GET /v1/devices/{deviceID}/pending-configuration", h.pullConfiguration)
	mux.HandleFunc("POST /v1/devices/{deviceID}/receipts", h.submitReceipt)

	mux.HandleFunc("POST /v1/device-groups", h.createGroup)
	mux.HandleFunc("GET /v1/device-groups", h.listGroups)
	mux.HandleFunc("GET /v1/device-groups/{groupID}/members", h.resolveGroup)

	mux.HandleFunc("POST /v1/configurations", h.createConfiguration)
	mux.HandleFunc("GET /v1/configurations", h.listConfigurations)
	mux.HandleFunc("GET /v1/configurations/{configurationID}", h.getConfiguration)
	mux.HandleFunc("POST /v1/configurations/{configurationID}/publish", h.publishConfiguration)
	mux.HandleFunc("POST /v1/configurations/{configurationID}/deprecate", h.deprecateConfiguration)

	mux.HandleFunc("POST /v1/rollout-rules", h.createRule)
	mux.HandleFunc("GET /v1/rollout-rules", h.listRules)
	mux.HandleFunc("POST /v1/rollouts", h.createRollout)
	mux.HandleFunc("GET /v1/rollouts", h.listRollouts)
	mux.HandleFunc("GET /v1/rollouts/{rolloutID}", h.getRollout)
	mux.HandleFunc("GET /v1/rollouts/{rolloutID}/targets", h.listTargets)
	mux.HandleFunc("POST /v1/rollouts/{rolloutID}/start", h.startRollout)
	mux.HandleFunc("POST /v1/rollouts/{rolloutID}/pause", h.pauseRollout)
	mux.HandleFunc("POST /v1/rollouts/{rolloutID}/resume", h.startRollout)
	mux.HandleFunc("POST /v1/rollouts/{rolloutID}/cancel", h.cancelRollout)
	mux.HandleFunc("POST /v1/rollouts/{rolloutID}/rollback", h.rollbackRollout)
	mux.HandleFunc("POST /v1/rollouts/{rolloutID}/reconcile", h.reconcileRollout)

	mux.HandleFunc("GET /v1/receipts", h.listReceipts)
	mux.HandleFunc("GET /v1/audit-events", h.listAudit)
	mux.HandleFunc("POST /v1/webhooks", h.createSubscription)
	mux.HandleFunc("GET /v1/webhooks", h.listSubscriptions)
	mux.HandleFunc("GET /v1/webhook-deliveries", h.listDeliveries)
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	api.WriteData(w, http.StatusOK, map[string]any{"status": "ok", "version": h.version, "commit": h.commit, "uptime_seconds": int(time.Since(h.startedAt).Seconds())})
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	if err := h.health(); err != nil {
		api.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorEnvelope{Error: api.APIError{Code: "not_ready", Message: err.Error()}})
		return
	}
	api.WriteData(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = io.WriteString(w, "# HELP edge_rollout_up Whether the process is running.\n# TYPE edge_rollout_up gauge\nedge_rollout_up 1\n")
}

func (h *Handler) openapi(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	http.ServeFile(w, nil, "api/openapi.yaml")
}

func (h *Handler) createDevice(w http.ResponseWriter, r *http.Request) {
	var command application.CreateDeviceCommand
	if !decode(w, r, &command) {
		return
	}
	entity, err := h.service.CreateDevice(r.Context(), command, actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusCreated, entity)
}

func (h *Handler) updateDevice(w http.ResponseWriter, r *http.Request) {
	var command application.UpdateDeviceCommand
	if !decode(w, r, &command) {
		return
	}
	entity, err := h.service.UpdateDevice(r.Context(), r.PathValue("deviceID"), command, actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
	var command application.HeartbeatCommand
	if !decode(w, r, &command) {
		return
	}
	entity, err := h.service.Heartbeat(r.Context(), r.PathValue("deviceID"), command)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) getDevice(w http.ResponseWriter, r *http.Request) {
	entity, err := h.service.GetDevice(r.Context(), r.PathValue("deviceID"))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	filter := device.Filter{Status: device.Status(r.URL.Query().Get("status")), HardwareModel: r.URL.Query().Get("hardware_model"), Query: r.URL.Query().Get("q"), Limit: limit, Offset: offset}
	items, total, err := h.service.ListDevices(r.Context(), filter)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	var command application.CreateGroupCommand
	if !decode(w, r, &command) {
		return
	}
	entity, err := h.service.CreateGroup(r.Context(), command, actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusCreated, entity)
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	items, total, err := h.service.ListGroups(r.Context(), limit, offset)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func (h *Handler) resolveGroup(w http.ResponseWriter, r *http.Request) {
	group, items, err := h.service.ResolveGroup(r.Context(), r.PathValue("groupID"))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, map[string]any{"group": group, "members": items, "count": len(items)})
}

func (h *Handler) createConfiguration(w http.ResponseWriter, r *http.Request) {
	var command application.CreateConfigurationCommand
	if !decode(w, r, &command) {
		return
	}
	entity, err := h.service.CreateConfiguration(r.Context(), command, actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusCreated, entity)
}

func (h *Handler) getConfiguration(w http.ResponseWriter, r *http.Request) {
	entity, err := h.service.GetConfiguration(r.Context(), r.PathValue("configurationID"))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) listConfigurations(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	filter := configuration.Filter{Name: r.URL.Query().Get("name"), Status: configuration.Status(r.URL.Query().Get("status")), Limit: limit, Offset: offset, Sort: r.URL.Query().Get("sort")}
	items, total, err := h.service.ListConfigurations(r.Context(), filter)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	for index := range items {
		if r.URL.Query().Get("include_content") != "true" {
			items[index].Content = ""
		}
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func (h *Handler) publishConfiguration(w http.ResponseWriter, r *http.Request) {
	entity, err := h.service.PublishConfiguration(r.Context(), r.PathValue("configurationID"), actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) deprecateConfiguration(w http.ResponseWriter, r *http.Request) {
	entity, err := h.service.DeprecateConfiguration(r.Context(), r.PathValue("configurationID"), actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var command application.CreateRuleCommand
	if !decode(w, r, &command) {
		return
	}
	entity, err := h.service.CreateRule(r.Context(), command, actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusCreated, entity)
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	enabled := r.URL.Query().Get("enabled") == "true"
	filter := rule.Filter{Enabled: &enabled, Limit: limit, Offset: offset}
	items, total, err := h.service.ListRules(r.Context(), filter)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func (h *Handler) createRollout(w http.ResponseWriter, r *http.Request) {
	var command application.CreateRolloutCommand
	if !decode(w, r, &command) {
		return
	}
	entity, err := h.service.CreateRollout(r.Context(), command, actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusCreated, entity)
}

func (h *Handler) getRollout(w http.ResponseWriter, r *http.Request) {
	entity, err := h.service.GetRollout(r.Context(), r.PathValue("rolloutID"))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) listRollouts(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	filter := rollout.Filter{Status: rollout.Status(r.URL.Query().Get("status")), ConfigurationID: r.URL.Query().Get("configuration_id"), GroupID: r.URL.Query().Get("group_id"), Limit: limit, Offset: offset}
	items, total, err := h.service.ListRollouts(r.Context(), filter)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func (h *Handler) listTargets(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	filter := rollout.TargetFilter{Status: rollout.TargetStatus(r.URL.Query().Get("status")), BatchNumber: integer(r.URL.Query().Get("batch"), 0), Limit: limit, Offset: offset}
	items, total, err := h.service.ListTargets(r.Context(), r.PathValue("rolloutID"), filter)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func (h *Handler) startRollout(w http.ResponseWriter, r *http.Request) {
	entity, err := h.service.StartRollout(r.Context(), r.PathValue("rolloutID"), actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) pauseRollout(w http.ResponseWriter, r *http.Request) {
	var command struct {
		Reason string `json:"reason"`
	}
	if !decodeOptional(w, r, &command) {
		return
	}
	entity, err := h.service.PauseRollout(r.Context(), r.PathValue("rolloutID"), command.Reason, actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) cancelRollout(w http.ResponseWriter, r *http.Request) {
	entity, err := h.service.CancelRollout(r.Context(), r.PathValue("rolloutID"), actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) rollbackRollout(w http.ResponseWriter, r *http.Request) {
	entity, err := h.service.RollbackRollout(r.Context(), r.PathValue("rolloutID"), actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) reconcileRollout(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ReconcileRollout(r.Context(), r.PathValue("rolloutID")); err != nil {
		api.WriteError(w, r, err)
		return
	}
	entity, err := h.service.GetRollout(r.Context(), r.PathValue("rolloutID"))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WriteData(w, http.StatusOK, entity)
}

func (h *Handler) pullConfiguration(w http.ResponseWriter, r *http.Request) {
	pending, err := h.service.PullPendingConfiguration(r.Context(), r.PathValue("deviceID"))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	if pending == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	api.WriteData(w, http.StatusOK, pending)
}

func (h *Handler) submitReceipt(w http.ResponseWriter, r *http.Request) {
	var command application.SubmitReceiptCommand
	if !decode(w, r, &command) {
		return
	}
	if header := strings.TrimSpace(r.Header.Get("Idempotency-Key")); command.IDempotencyKey == "" {
		command.IDempotencyKey = header
	}
	entity, created, err := h.service.SubmitReceipt(r.Context(), r.PathValue("deviceID"), command)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	api.WriteData(w, status, map[string]any{"receipt": entity, "created": created})
}

func (h *Handler) listReceipts(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	filter := receipt.Filter{RolloutID: r.URL.Query().Get("rollout_id"), DeviceID: r.URL.Query().Get("device_id"), Status: receipt.Status(r.URL.Query().Get("status")), Limit: limit, Offset: offset}
	items, total, err := h.service.ListReceipts(r.Context(), filter)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	filter := audit.Filter{Actor: r.URL.Query().Get("actor"), ResourceType: r.URL.Query().Get("resource_type"), ResourceID: r.URL.Query().Get("resource_id"), Limit: limit, Offset: offset}
	items, total, err := h.service.ListAudit(r.Context(), filter)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func (h *Handler) createSubscription(w http.ResponseWriter, r *http.Request) {
	var command application.CreateSubscriptionCommand
	if !decode(w, r, &command) {
		return
	}
	entity, err := h.service.CreateSubscription(r.Context(), command, actor(r))
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	entity.Secret = ""
	api.WriteData(w, http.StatusCreated, entity)
}

func (h *Handler) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	filter := webhook.SubscriptionFilter{EnabledOnly: r.URL.Query().Get("enabled") == "true", EventType: r.URL.Query().Get("event_type"), Limit: limit, Offset: offset}
	items, total, err := h.service.ListSubscriptions(r.Context(), filter)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	for index := range items {
		items[index].Secret = ""
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func (h *Handler) listDeliveries(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	filter := webhook.DeliveryFilter{SubscriptionID: r.URL.Query().Get("subscription_id"), Status: r.URL.Query().Get("status"), EventType: r.URL.Query().Get("event_type"), Limit: limit, Offset: offset}
	items, total, err := h.service.ListDeliveries(r.Context(), filter)
	if err != nil {
		api.WriteError(w, r, err)
		return
	}
	api.WritePage(w, items, limit, offset, len(items), total)
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Body == nil {
		api.WriteJSON(w, http.StatusBadRequest, api.ErrorEnvelope{Error: api.APIError{Code: "invalid_json", Message: "request body is required"}})
		return false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		message := "invalid JSON body"
		var syntax *json.SyntaxError
		var typeError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntax):
			message = fmt.Sprintf("invalid JSON at byte %d", syntax.Offset)
		case errors.As(err, &typeError):
			message = fmt.Sprintf("invalid value for field %s", typeError.Field)
		case errors.Is(err, io.EOF):
			message = "request body is required"
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			message = err.Error()
		}
		api.WriteJSON(w, http.StatusBadRequest, api.ErrorEnvelope{Error: api.APIError{Code: "invalid_json", Message: message}})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		api.WriteJSON(w, http.StatusBadRequest, api.ErrorEnvelope{Error: api.APIError{Code: "invalid_json", Message: "request body must contain one JSON object"}})
		return false
	}
	return true
}

func decodeOptional(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true
	}
	return decode(w, r, target)
}

func page(r *http.Request) (int, int) {
	return integer(r.URL.Query().Get("limit"), 50), integer(r.URL.Query().Get("offset"), 0)
}

func integer(raw string, defaultValue int) int {
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return value
}

func actor(r *http.Request) string {
	return middleware.ActorFrom(r.Context())
}
