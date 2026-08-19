package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/example/edge-rollout-control/internal/domain/audit"
	"github.com/example/edge-rollout-control/internal/domain/rollout"
	"github.com/example/edge-rollout-control/internal/domain/rule"
	"github.com/example/edge-rollout-control/internal/domain/webhook"
)

func (u *UnitOfWork) CreateRollout(ctx context.Context, r rollout.Rollout, targets []rollout.Target) error {
	if err := r.Validate(); err != nil {
		return err
	}
	tx, err := u.db.BeginTx(OperationContext(ctx), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO rollouts(id,name,configuration_id,group_id,strategy,batch_size,batch_percent,scheduled_at,status,current_batch,target_count,success_count,failure_count,timeout_count,pause_reason,rollback_configuration_id,rule_id,created_by,created_at,updated_at,started_at,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.ID, r.Name, r.ConfigurationID, r.GroupID, r.Strategy, r.BatchSize, r.BatchPercent, ptrts(r.ScheduledAt), r.Status, r.CurrentBatch, r.TargetCount, r.SuccessCount, r.FailureCount, r.TimeoutCount, r.PauseReason, r.RollbackConfigurationID, r.RuleID, r.CreatedBy, ts(r.CreatedAt), ts(r.UpdatedAt), ptrts(r.StartedAt), ptrts(r.CompletedAt))
	if err != nil {
		return err
	}
	for _, t := range targets {
		if _, err = tx.ExecContext(ctx, `INSERT INTO rollout_targets(id,rollout_id,device_id,batch_number,status,desired_configuration_id,delivered_at,acknowledged_at,error_code,error_message,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, t.ID, t.RolloutID, t.DeviceID, t.BatchNumber, t.Status, t.DesiredConfigurationID, ptrts(t.DeliveredAt), ptrts(t.AcknowledgedAt), t.ErrorCode, t.ErrorMessage, ts(t.CreatedAt), ts(t.UpdatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ptrts(t *time.Time) string {
	if t == nil {
		return ""
	}
	return ts(*t)
}

func (u *UnitOfWork) GetRollout(ctx context.Context, id string) (r rollout.Rollout, err error) {
	var scheduled, started, completed string
	err = u.db.QueryRowContext(ctx, `SELECT id,name,configuration_id,group_id,strategy,batch_size,batch_percent,scheduled_at,status,current_batch,target_count,success_count,failure_count,timeout_count,pause_reason,rollback_configuration_id,rule_id,created_by,created_at,updated_at,started_at,completed_at FROM rollouts WHERE id=?`, id).Scan(&r.ID, &r.Name, &r.ConfigurationID, &r.GroupID, &r.Strategy, &r.BatchSize, &r.BatchPercent, &scheduled, &r.Status, &r.CurrentBatch, &r.TargetCount, &r.SuccessCount, &r.FailureCount, &r.TimeoutCount, &r.PauseReason, &r.RollbackConfigurationID, &r.RuleID, &r.CreatedBy, new(string), new(string), &started, &completed)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	r.ScheduledAt = parse(scheduled)
	r.StartedAt = parse(started)
	r.CompletedAt = parse(completed)
	return r, err
}
func (u *UnitOfWork) UpdateRollout(ctx context.Context, r rollout.Rollout) error {
	res, e := u.db.ExecContext(ctx, `UPDATE rollouts SET status=?,current_batch=?,success_count=?,failure_count=?,timeout_count=?,pause_reason=?,updated_at=?,started_at=?,completed_at=? WHERE id=?`, r.Status, r.CurrentBatch, r.SuccessCount, r.FailureCount, r.TimeoutCount, r.PauseReason, ts(r.UpdatedAt), ptrts(r.StartedAt), ptrts(r.CompletedAt), r.ID)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return e
}
func (u *UnitOfWork) ListRollouts(ctx context.Context, f rollout.Filter) (out []rollout.Rollout, total int, err error) {
	where := []string{"1=1"}
	args := []any{}
	if f.Status != "" {
		where = append(where, "status=?")
		args = append(args, f.Status)
	}
	if f.ConfigurationID != "" {
		where = append(where, "configuration_id=?")
		args = append(args, f.ConfigurationID)
	}
	if f.GroupID != "" {
		where = append(where, "group_id=?")
		args = append(args, f.GroupID)
	}
	cl := strings.Join(where, " AND ")
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rollouts WHERE "+cl, args...).Scan(&total)
	rows, e := u.db.QueryContext(ctx, "SELECT id FROM rollouts WHERE "+cl+" ORDER BY created_at DESC LIMIT ? OFFSET ?", append(args, lim(f.Limit), offset(f.Offset))...)
	if e != nil {
		return nil, 0, e
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	for _, id := range ids {
		item, getErr := u.GetRollout(ctx, id)
		if getErr != nil {
			return nil, 0, getErr
		}
		out = append(out, item)
	}
	return out, total, nil
}
func (u *UnitOfWork) ListRunnableRollouts(ctx context.Context, now time.Time, limit int) (out []rollout.Rollout, err error) {
	rows, e := u.db.QueryContext(ctx, `SELECT id FROM rollouts WHERE (status=? OR status=?) AND (scheduled_at IS NULL OR scheduled_at<=?) ORDER BY created_at LIMIT ?`, rollout.StatusPending, rollout.StatusRunning, ts(now), lim(limit))
	if e != nil {
		return nil, e
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if e := rows.Scan(&id); e != nil {
			_ = rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	if e := rows.Close(); e != nil {
		return nil, e
	}
	for _, id := range ids {
		r, e := u.GetRollout(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, nil
}
func (u *UnitOfWork) ListTargets(ctx context.Context, id string, f rollout.TargetFilter) (out []rollout.Target, total int, err error) {
	where := []string{"rollout_id=?"}
	args := []any{id}
	if f.Status != "" {
		where = append(where, "status=?")
		args = append(args, f.Status)
	}
	if f.BatchNumber > 0 {
		where = append(where, "batch_number=?")
		args = append(args, f.BatchNumber)
	}
	cl := strings.Join(where, " AND ")
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rollout_targets WHERE "+cl, args...).Scan(&total)
	rows, e := u.db.QueryContext(ctx, "SELECT id,rollout_id,device_id,batch_number,status,desired_configuration_id,delivered_at,acknowledged_at,error_code,error_message,created_at,updated_at FROM rollout_targets WHERE "+cl+" ORDER BY batch_number,device_id LIMIT ? OFFSET ?", append(args, lim(f.Limit), offset(f.Offset))...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	for rows.Next() {
		var t rollout.Target
		var d, a, c, u string
		if e = rows.Scan(&t.ID, &t.RolloutID, &t.DeviceID, &t.BatchNumber, &t.Status, &t.DesiredConfigurationID, &d, &a, &t.ErrorCode, &t.ErrorMessage, &c, &u); e != nil {
			return
		}
		t.DeliveredAt = parse(d)
		t.AcknowledgedAt = parse(a)
		if p := parse(c); p != nil {
			t.CreatedAt = *p
		}
		if p := parse(u); p != nil {
			t.UpdatedAt = *p
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}
func (u *UnitOfWork) GetTarget(ctx context.Context, rid, did string) (t rollout.Target, err error) {
	var d, a, c, updated string
	err = u.db.QueryRowContext(ctx, `SELECT id,rollout_id,device_id,batch_number,status,desired_configuration_id,delivered_at,acknowledged_at,error_code,error_message,created_at,updated_at FROM rollout_targets WHERE rollout_id=? AND device_id=?`, rid, did).Scan(&t.ID, &t.RolloutID, &t.DeviceID, &t.BatchNumber, &t.Status, &t.DesiredConfigurationID, &d, &a, &t.ErrorCode, &t.ErrorMessage, &c, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	t.DeliveredAt = parse(d)
	t.AcknowledgedAt = parse(a)
	if p := parse(c); p != nil {
		t.CreatedAt = *p
	}
	if p := parse(updated); p != nil {
		t.UpdatedAt = *p
	}
	return
}
func (u *UnitOfWork) UpdateTarget(ctx context.Context, t rollout.Target) error {
	res, e := u.db.ExecContext(ctx, `UPDATE rollout_targets SET status=?,desired_configuration_id=?,delivered_at=?,acknowledged_at=?,error_code=?,error_message=?,updated_at=? WHERE id=?`, t.Status, t.DesiredConfigurationID, ptrts(t.DeliveredAt), ptrts(t.AcknowledgedAt), t.ErrorCode, t.ErrorMessage, ts(t.UpdatedAt), t.ID)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return e
}
func (u *UnitOfWork) CountTargetStates(ctx context.Context, id string, batch int) (map[rollout.TargetStatus]int, error) {
	where := "rollout_id=?"
	args := []any{id}
	if batch > 0 {
		where += " AND batch_number=?"
		args = append(args, batch)
	}
	rows, e := u.db.QueryContext(ctx, "SELECT status,COUNT(*) FROM rollout_targets WHERE "+where+" GROUP BY status", args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := map[rollout.TargetStatus]int{}
	for rows.Next() {
		var s rollout.TargetStatus
		var n int
		if e = rows.Scan(&s, &n); e != nil {
			return nil, e
		}
		out[s] = n
	}
	return out, rows.Err()
}

func (u *UnitOfWork) CreateRule(ctx context.Context, r rule.Rule) error {
	_, e := u.db.ExecContext(ctx, `INSERT INTO rollout_rules(id,name,max_failure_rate,max_offline_rate,max_timeout_rate,critical_labels,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, r.ID, r.Name, r.MaxFailureRate, r.MaxOfflineRate, r.MaxTimeoutRate, enc(r.CriticalLabels), r.Enabled, ts(r.CreatedAt), ts(r.UpdatedAt))
	return e
}
func (u *UnitOfWork) UpdateRule(ctx context.Context, r rule.Rule) error {
	_, e := u.db.ExecContext(ctx, `UPDATE rollout_rules SET enabled=?,max_failure_rate=?,max_offline_rate=?,max_timeout_rate=?,critical_labels=?,updated_at=? WHERE id=?`, r.Enabled, r.MaxFailureRate, r.MaxOfflineRate, r.MaxTimeoutRate, enc(r.CriticalLabels), ts(r.UpdatedAt), r.ID)
	return e
}
func (u *UnitOfWork) GetRule(ctx context.Context, id string) (r rule.Rule, err error) {
	var labels, created, updated string
	var enabled int
	err = u.db.QueryRowContext(ctx, `SELECT id,name,max_failure_rate,max_offline_rate,max_timeout_rate,critical_labels,enabled,created_at,updated_at FROM rollout_rules WHERE id=?`, id).Scan(&r.ID, &r.Name, &r.MaxFailureRate, &r.MaxOfflineRate, &r.MaxTimeoutRate, &labels, &enabled, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	_ = dec(labels, &r.CriticalLabels)
	r.Enabled = enabled == 1
	if p := parse(created); p != nil {
		r.CreatedAt = *p
	}
	if p := parse(updated); p != nil {
		r.UpdatedAt = *p
	}
	return
}
func (u *UnitOfWork) ListRules(ctx context.Context, f rule.Filter) (out []rule.Rule, total int, err error) {
	where := "1=1"
	args := []any{}
	if f.Enabled != nil {
		where += " AND enabled=?"
		args = append(args, *f.Enabled)
	}
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rollout_rules WHERE "+where, args...).Scan(&total)
	rows, e := u.db.QueryContext(ctx, "SELECT id FROM rollout_rules WHERE "+where+" ORDER BY name LIMIT ? OFFSET ?", append(args, lim(f.Limit), offset(f.Offset))...)
	if e != nil {
		return nil, 0, e
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if e := rows.Scan(&id); e != nil {
			_ = rows.Close()
			return nil, 0, e
		}
		ids = append(ids, id)
	}
	if e := rows.Close(); e != nil {
		return nil, 0, e
	}
	for _, id := range ids {
		r, e := u.GetRule(ctx, id)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, r)
	}
	return out, total, nil
}

func (u *UnitOfWork) AppendAudit(ctx context.Context, e audit.Event) error {
	_, err := u.db.ExecContext(ctx, `INSERT INTO audit_events(id,actor,action,resource_type,resource_id,request_id,details,created_at) VALUES(?,?,?,?,?,?,?,?)`, e.ID, e.Actor, e.Action, e.ResourceType, e.ResourceID, e.RequestID, enc(e.Details), ts(e.CreatedAt))
	return err
}
func (u *UnitOfWork) ListAudit(ctx context.Context, f audit.Filter) (out []audit.Event, total int, err error) {
	where := []string{"1=1"}
	args := []any{}
	if f.Actor != "" {
		where = append(where, "actor=?")
		args = append(args, f.Actor)
	}
	if f.ResourceType != "" {
		where = append(where, "resource_type=?")
		args = append(args, f.ResourceType)
	}
	if f.ResourceID != "" {
		where = append(where, "resource_id=?")
		args = append(args, f.ResourceID)
	}
	cl := strings.Join(where, " AND ")
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE "+cl, args...).Scan(&total)
	rows, e := u.db.QueryContext(ctx, "SELECT id,actor,action,resource_type,resource_id,request_id,details,created_at FROM audit_events WHERE "+cl+" ORDER BY created_at DESC LIMIT ? OFFSET ?", append(args, lim(f.Limit), offset(f.Offset))...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	for rows.Next() {
		var x audit.Event
		var d, c string
		if e = rows.Scan(&x.ID, &x.Actor, &x.Action, &x.ResourceType, &x.ResourceID, &x.RequestID, &d, &c); e != nil {
			return
		}
		_ = dec(d, &x.Details)
		if p := parse(c); p != nil {
			x.CreatedAt = *p
		}
		out = append(out, x)
	}
	return out, total, rows.Err()
}

func (u *UnitOfWork) CreateSubscription(ctx context.Context, s webhook.Subscription) error {
	_, e := u.db.ExecContext(ctx, `INSERT INTO webhook_subscriptions(id,name,url,secret,event_types,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, s.ID, s.Name, s.URL, s.Secret, enc(s.EventTypes), s.Enabled, ts(s.CreatedAt), ts(s.UpdatedAt))
	return e
}
func (u *UnitOfWork) UpdateSubscription(ctx context.Context, s webhook.Subscription) error {
	_, e := u.db.ExecContext(ctx, `UPDATE webhook_subscriptions SET enabled=?,url=?,event_types=?,updated_at=? WHERE id=?`, s.Enabled, s.URL, enc(s.EventTypes), ts(s.UpdatedAt), s.ID)
	return e
}
func (u *UnitOfWork) GetSubscription(ctx context.Context, id string) (s webhook.Subscription, err error) {
	var events, created, updated string
	var enabled int
	err = u.db.QueryRowContext(ctx, `SELECT id,name,url,secret,event_types,enabled,created_at,updated_at FROM webhook_subscriptions WHERE id=?`, id).Scan(&s.ID, &s.Name, &s.URL, &s.Secret, &events, &enabled, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	_ = dec(events, &s.EventTypes)
	s.Enabled = enabled == 1
	if p := parse(created); p != nil {
		s.CreatedAt = *p
	}
	if p := parse(updated); p != nil {
		s.UpdatedAt = *p
	}
	return
}
func (u *UnitOfWork) ListSubscriptions(ctx context.Context, f webhook.SubscriptionFilter) (out []webhook.Subscription, total int, err error) {
	where := []string{"1=1"}
	args := []any{}
	if f.EnabledOnly {
		where = append(where, "enabled=1")
	}
	if f.Enabled != nil {
		where = append(where, "enabled=?")
		args = append(args, *f.Enabled)
	}
	cl := strings.Join(where, " AND ")
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM webhook_subscriptions WHERE "+cl, args...).Scan(&total)
	rows, e := u.db.QueryContext(ctx, "SELECT id FROM webhook_subscriptions WHERE "+cl+" ORDER BY name LIMIT ? OFFSET ?", append(args, lim(f.Limit), offset(f.Offset))...)
	if e != nil {
		return nil, 0, e
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if e := rows.Scan(&id); e != nil {
			_ = rows.Close()
			return nil, 0, e
		}
		ids = append(ids, id)
	}
	if e := rows.Close(); e != nil {
		return nil, 0, e
	}
	for _, id := range ids {
		s, e := u.GetSubscription(ctx, id)
		if e != nil {
			return nil, 0, e
		}
		if f.EventType != "" && !contains(s.EventTypes, f.EventType) {
			continue
		}
		out = append(out, s)
	}
	return out, total, nil
}
func contains(items []string, want string) bool {
	for _, x := range items {
		if x == want || x == "*" {
			return true
		}
	}
	return false
}
func (u *UnitOfWork) EnqueueDelivery(ctx context.Context, d webhook.Delivery) error {
	_, e := u.db.ExecContext(ctx, `INSERT INTO webhook_deliveries(id,subscription_id,event_type,aggregate_id,payload,status,attempts,last_status_code,next_attempt_at,last_error,delivered_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, d.ID, d.SubscriptionID, d.EventType, d.AggregateID, d.Payload, d.Status, d.Attempts, d.LastStatusCode, ts(d.NextAttemptAt), d.LastError, ptrts(d.DeliveredAt), ts(d.CreatedAt), ts(d.UpdatedAt))
	return e
}
func (u *UnitOfWork) ListDueDeliveries(ctx context.Context, now time.Time, limit int) (out []webhook.Delivery, err error) {
	rows, e := u.db.QueryContext(ctx, `SELECT id,subscription_id,event_type,aggregate_id,payload,status,attempts,last_status_code,next_attempt_at,last_error,delivered_at,created_at,updated_at FROM webhook_deliveries WHERE status IN (?,?) AND next_attempt_at<=? ORDER BY next_attempt_at LIMIT ?`, webhook.DeliveryPending, webhook.DeliveryRetrying, ts(now), lim(limit))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		d, e := scanDelivery(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func scanDelivery(row interface{ Scan(...any) error }) (d webhook.Delivery, err error) {
	var next, delivered, created, updated string
	err = row.Scan(&d.ID, &d.SubscriptionID, &d.EventType, &d.AggregateID, &d.Payload, &d.Status, &d.Attempts, &d.LastStatusCode, &next, &d.LastError, &delivered, &created, &updated)
	d.NextAttemptAt = *parse(next)
	d.DeliveredAt = parse(delivered)
	if p := parse(created); p != nil {
		d.CreatedAt = *p
	}
	if p := parse(updated); p != nil {
		d.UpdatedAt = *p
	}
	return
}
func (u *UnitOfWork) UpdateDelivery(ctx context.Context, d webhook.Delivery) error {
	_, e := u.db.ExecContext(ctx, `UPDATE webhook_deliveries SET status=?,attempts=?,last_status_code=?,next_attempt_at=?,last_error=?,delivered_at=?,updated_at=? WHERE id=?`, d.Status, d.Attempts, d.LastStatusCode, ts(d.NextAttemptAt), d.LastError, ptrts(d.DeliveredAt), ts(d.UpdatedAt), d.ID)
	return e
}
func (u *UnitOfWork) ListDeliveries(ctx context.Context, f webhook.DeliveryFilter) (out []webhook.Delivery, total int, err error) {
	where := []string{"1=1"}
	args := []any{}
	if f.SubscriptionID != "" {
		where = append(where, "subscription_id=?")
		args = append(args, f.SubscriptionID)
	}
	if f.Status != "" {
		where = append(where, "status=?")
		args = append(args, f.Status)
	}
	if f.EventType != "" {
		where = append(where, "event_type=?")
		args = append(args, f.EventType)
	}
	cl := strings.Join(where, " AND ")
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM webhook_deliveries WHERE "+cl, args...).Scan(&total)
	rows, e := u.db.QueryContext(ctx, "SELECT id,subscription_id,event_type,aggregate_id,payload,status,attempts,last_status_code,next_attempt_at,last_error,delivered_at,created_at,updated_at FROM webhook_deliveries WHERE "+cl+" ORDER BY created_at DESC LIMIT ? OFFSET ?", append(args, lim(f.Limit), offset(f.Offset))...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	for rows.Next() {
		d, e := scanDelivery(rows)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}
