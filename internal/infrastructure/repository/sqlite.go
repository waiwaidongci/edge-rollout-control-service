package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/example/edge-rollout-control/internal/application"
	"github.com/example/edge-rollout-control/internal/domain/configuration"
	"github.com/example/edge-rollout-control/internal/domain/device"
	"github.com/example/edge-rollout-control/internal/domain/receipt"
)

var ErrNotFound = errors.New("repository: not found")

type UnitOfWork struct{ db *sql.DB }

func OperationContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func ContextCancellation(ctx context.Context) error {
	return OperationContext(ctx).Err()
}

func New(db *sql.DB) *UnitOfWork                     { return &UnitOfWork{db: db} }
func (u *UnitOfWork) Ping(ctx context.Context) error { return u.db.PingContext(ctx) }
func (u *UnitOfWork) Close() error                   { return u.db.Close() }
func enc(v any) string                               { b, _ := json.Marshal(v); return string(b) }
func dec(s string, v any) error                      { return json.Unmarshal([]byte(s), v) }
func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func parse(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil {
		return nil
	}
	return &t
}
func lim(v int) int {
	if v <= 0 || v > 1000 {
		return 100
	}
	return v
}
func offset(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func (u *UnitOfWork) CreateDevice(ctx context.Context, d device.Device) error {
	if err := d.Validate(); err != nil {
		return err
	}
	_, e := u.db.ExecContext(ctx, `INSERT INTO devices(id,name,hardware_model,software_version,labels,status,current_configuration_id,last_heartbeat_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, d.ID, d.Name, d.HardwareModel, d.SoftwareVersion, enc(d.Labels), d.Status, d.CurrentConfigurationID, ts(d.LastHeartbeatAt), ts(d.CreatedAt), ts(d.UpdatedAt))
	return e
}
func (u *UnitOfWork) UpdateDevice(ctx context.Context, d device.Device) error {
	_, e := u.db.ExecContext(ctx, `UPDATE devices SET name=?,hardware_model=?,software_version=?,labels=?,status=?,current_configuration_id=?,last_heartbeat_at=?,updated_at=? WHERE id=?`, d.Name, d.HardwareModel, d.SoftwareVersion, enc(d.Labels), d.Status, d.CurrentConfigurationID, ts(d.LastHeartbeatAt), ts(d.UpdatedAt), d.ID)
	return e
}
func (u *UnitOfWork) GetDevice(ctx context.Context, id string) (d device.Device, e error) {
	var labels, heartbeat, created, updated string
	e = u.db.QueryRowContext(ctx, `SELECT id,name,hardware_model,software_version,labels,status,current_configuration_id,last_heartbeat_at,created_at,updated_at FROM devices WHERE id=?`, id).Scan(&d.ID, &d.Name, &d.HardwareModel, &d.SoftwareVersion, &labels, &d.Status, &d.CurrentConfigurationID, &heartbeat, &created, &updated)
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotFound
		return
	}
	_ = dec(labels, &d.Labels)
	if p := parse(heartbeat); p != nil {
		d.LastHeartbeatAt = *p
	}
	if p := parse(created); p != nil {
		d.CreatedAt = *p
	}
	if p := parse(updated); p != nil {
		d.UpdatedAt = *p
	}
	return
}
func (u *UnitOfWork) ListDevices(ctx context.Context, f device.Filter) (out []device.Device, total int, e error) {
	where := []string{"1=1"}
	args := []any{}
	if f.Status != "" {
		where = append(where, "status=?")
		args = append(args, f.Status)
	}
	if f.HardwareModel != "" {
		where = append(where, "hardware_model=?")
		args = append(args, f.HardwareModel)
	}
	if f.Query != "" {
		where = append(where, "(name LIKE ? OR id LIKE ?)")
		q := "%" + f.Query + "%"
		args = append(args, q, q)
	}
	cl := strings.Join(where, " AND ")
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices WHERE "+cl, args...).Scan(&total)
	rows, err := u.db.QueryContext(ctx, "SELECT id FROM devices WHERE "+cl+" ORDER BY name LIMIT ? OFFSET ?", append(args, lim(f.Limit), offset(f.Offset))...)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	for _, id := range ids {
		d, err := u.GetDevice(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, nil
}
func (u *UnitOfWork) CreateGroup(ctx context.Context, g device.Group) error {
	if err := g.Validate(); err != nil {
		return err
	}
	_, e := u.db.ExecContext(ctx, `INSERT INTO device_groups(id,name,selector,description,created_at,updated_at) VALUES(?,?,?,?,?,?)`, g.ID, g.Name, g.Selector, g.Description, ts(g.CreatedAt), ts(g.UpdatedAt))
	return e
}
func (u *UnitOfWork) GetGroup(ctx context.Context, id string) (g device.Group, e error) {
	e = u.db.QueryRowContext(ctx, `SELECT id,name,selector,description,created_at,updated_at FROM device_groups WHERE id=?`, id).Scan(&g.ID, &g.Name, &g.Selector, &g.Description, new(string), new(string))
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotFound
	}
	return
}
func (u *UnitOfWork) ListGroups(ctx context.Context, limit, off int) (out []device.Group, total int, e error) {
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_groups").Scan(&total)
	rows, e := u.db.QueryContext(ctx, "SELECT id,name,selector,description FROM device_groups ORDER BY name LIMIT ? OFFSET ?", lim(limit), offset(off))
	if e != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var g device.Group
		if e = rows.Scan(&g.ID, &g.Name, &g.Selector, &g.Description); e != nil {
			return
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}
func (u *UnitOfWork) ResolveGroup(ctx context.Context, id string) (out []device.Device, e error) {
	g, e := u.GetGroup(ctx, id)
	if e != nil {
		return
	}
	f := device.Filter{Query: g.Selector, Limit: 1000}
	out, _, e = u.ListDevices(ctx, f)
	return
}

func (u *UnitOfWork) CreateConfiguration(ctx context.Context, c configuration.Configuration) error {
	if err := c.Validate(); err != nil {
		return err
	}
	_, e := u.db.ExecContext(ctx, `INSERT INTO configurations(id,name,version,format,content,content_sha256,compatible_models,variables,change_summary,status,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.ID, c.Name, c.Version, c.Format, c.Content, c.ContentSHA256, enc(c.CompatibleModels), enc(c.Variables), c.ChangeSummary, c.Status, c.CreatedBy, ts(c.CreatedAt), ts(c.UpdatedAt))
	return e
}
func (u *UnitOfWork) GetConfiguration(ctx context.Context, id string) (c configuration.Configuration, e error) {
	var models, vars, created, updated string
	e = u.db.QueryRowContext(ctx, `SELECT id,name,version,format,content,content_sha256,compatible_models,variables,change_summary,status,created_by,created_at,updated_at FROM configurations WHERE id=?`, id).Scan(&c.ID, &c.Name, &c.Version, &c.Format, &c.Content, &c.ContentSHA256, &models, &vars, &c.ChangeSummary, &c.Status, &c.CreatedBy, &created, &updated)
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotFound
		return
	}
	_ = dec(models, &c.CompatibleModels)
	_ = dec(vars, &c.Variables)
	if p := parse(created); p != nil {
		c.CreatedAt = *p
	}
	if p := parse(updated); p != nil {
		c.UpdatedAt = *p
	}
	return
}
func (u *UnitOfWork) ListConfigurations(ctx context.Context, f configuration.Filter) (out []configuration.Configuration, total int, e error) {
	where := []string{"1=1"}
	args := []any{}
	if f.Status != "" {
		where = append(where, "status=?")
		args = append(args, f.Status)
	}
	if f.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+f.Name+"%")
	}
	cl := strings.Join(where, " AND ")
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM configurations WHERE "+cl, args...).Scan(&total)
	rows, e := u.db.QueryContext(ctx, "SELECT id FROM configurations WHERE "+cl+" ORDER BY name,version DESC LIMIT ? OFFSET ?", append(args, lim(f.Limit), offset(f.Offset))...)
	if e != nil {
		return
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if er := rows.Scan(&id); er != nil {
			_ = rows.Close()
			return nil, 0, er
		}
		ids = append(ids, id)
	}
	if er := rows.Close(); er != nil {
		return nil, 0, er
	}
	for _, id := range ids {
		c, er := u.GetConfiguration(ctx, id)
		if er != nil {
			return nil, 0, er
		}
		out = append(out, c)
	}
	return out, total, nil
}
func (u *UnitOfWork) UpdateConfigurationStatus(ctx context.Context, id string, s configuration.Status, at time.Time) error {
	res, e := u.db.ExecContext(ctx, "UPDATE configurations SET status=?,updated_at=? WHERE id=?", s, ts(at), id)
	if e == nil {
		n, _ := res.RowsAffected()
		if n == 0 {
			e = ErrNotFound
		}
	}
	return e
}

func (u *UnitOfWork) CreateReceipt(ctx context.Context, r receipt.Receipt) (bool, error) {
	if err := r.Validate(); err != nil {
		return false, err
	}
	res, e := u.db.ExecContext(ctx, `INSERT OR IGNORE INTO receipts(id,idempotency_key,rollout_id,device_id,configuration_id,status,error_code,message,device_timestamp,received_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, r.ID, r.IDempotencyKey, r.RolloutID, r.DeviceID, r.ConfigurationID, r.Status, r.ErrorCode, r.Message, func() string {
		if r.DeviceTimestamp == nil {
			return ""
		}
		return ts(*r.DeviceTimestamp)
	}(), ts(r.ReceivedAt))
	if e != nil {
		return false, e
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
func (u *UnitOfWork) ListReceipts(ctx context.Context, f receipt.Filter) (out []receipt.Receipt, total int, e error) {
	where := []string{"1=1"}
	args := []any{}
	if f.RolloutID != "" {
		where = append(where, "rollout_id=?")
		args = append(args, f.RolloutID)
	}
	if f.DeviceID != "" {
		where = append(where, "device_id=?")
		args = append(args, f.DeviceID)
	}
	cl := strings.Join(where, " AND ")
	_ = u.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM receipts WHERE "+cl, args...).Scan(&total)
	rows, e := u.db.QueryContext(ctx, "SELECT id,idempotency_key,rollout_id,device_id,configuration_id,status,error_code,message,device_timestamp,received_at FROM receipts WHERE "+cl+" ORDER BY received_at DESC LIMIT ? OFFSET ?", append(args, lim(f.Limit), offset(f.Offset))...)
	if e != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var r receipt.Receipt
		var dt, rt string
		if e = rows.Scan(&r.ID, &r.IDempotencyKey, &r.RolloutID, &r.DeviceID, &r.ConfigurationID, &r.Status, &r.ErrorCode, &r.Message, &dt, &rt); e != nil {
			return
		}
		r.DeviceTimestamp = parse(dt)
		if p := parse(rt); p != nil {
			r.ReceivedAt = *p
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

var _ application.UnitOfWork = (*UnitOfWork)(nil)
