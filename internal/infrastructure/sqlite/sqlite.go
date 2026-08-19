package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS devices(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE,hardware_model TEXT NOT NULL,software_version TEXT NOT NULL,labels TEXT NOT NULL DEFAULT '{}',status TEXT NOT NULL,current_configuration_id TEXT NOT NULL DEFAULT '',last_heartbeat_at TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS device_groups(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE,selector TEXT NOT NULL,description TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS configurations(id TEXT PRIMARY KEY,name TEXT NOT NULL,version INTEGER NOT NULL,format TEXT NOT NULL,content TEXT NOT NULL,content_sha256 TEXT NOT NULL,compatible_models TEXT NOT NULL DEFAULT '[]',variables TEXT NOT NULL DEFAULT '{}',change_summary TEXT NOT NULL DEFAULT '',status TEXT NOT NULL,created_by TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(name,version));
CREATE TABLE IF NOT EXISTS rollout_rules(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE,max_failure_rate REAL NOT NULL,max_offline_rate REAL NOT NULL,max_timeout_rate REAL NOT NULL,critical_labels TEXT NOT NULL DEFAULT '{}',enabled INTEGER NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS rollouts(id TEXT PRIMARY KEY,name TEXT NOT NULL,configuration_id TEXT NOT NULL,group_id TEXT NOT NULL DEFAULT '',strategy TEXT NOT NULL,batch_size INTEGER NOT NULL,batch_percent INTEGER NOT NULL,scheduled_at TEXT,status TEXT NOT NULL,current_batch INTEGER NOT NULL,target_count INTEGER NOT NULL,success_count INTEGER NOT NULL,failure_count INTEGER NOT NULL,timeout_count INTEGER NOT NULL,pause_reason TEXT NOT NULL,rollback_configuration_id TEXT NOT NULL,rule_id TEXT NOT NULL,created_by TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,started_at TEXT,completed_at TEXT);
CREATE TABLE IF NOT EXISTS rollout_targets(id TEXT PRIMARY KEY,rollout_id TEXT NOT NULL,device_id TEXT NOT NULL,batch_number INTEGER NOT NULL,status TEXT NOT NULL,desired_configuration_id TEXT NOT NULL,delivered_at TEXT,acknowledged_at TEXT,error_code TEXT NOT NULL,error_message TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(rollout_id,device_id));
CREATE TABLE IF NOT EXISTS receipts(id TEXT PRIMARY KEY,idempotency_key TEXT NOT NULL UNIQUE,rollout_id TEXT NOT NULL,device_id TEXT NOT NULL,configuration_id TEXT NOT NULL,status TEXT NOT NULL,error_code TEXT NOT NULL,message TEXT NOT NULL,device_timestamp TEXT,received_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS audit_events(id TEXT PRIMARY KEY,actor TEXT NOT NULL,action TEXT NOT NULL,resource_type TEXT NOT NULL,resource_id TEXT NOT NULL,request_id TEXT NOT NULL,details TEXT NOT NULL,created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS webhook_subscriptions(id TEXT PRIMARY KEY,name TEXT NOT NULL,url TEXT NOT NULL,secret TEXT NOT NULL,event_types TEXT NOT NULL,enabled INTEGER NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS webhook_deliveries(id TEXT PRIMARY KEY,subscription_id TEXT NOT NULL,event_type TEXT NOT NULL,aggregate_id TEXT NOT NULL,payload TEXT NOT NULL,status TEXT NOT NULL,attempts INTEGER NOT NULL,last_status_code INTEGER NOT NULL,next_attempt_at TEXT NOT NULL,last_error TEXT NOT NULL,delivered_at TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_rollouts_status_schedule ON rollouts(status,scheduled_at); CREATE INDEX IF NOT EXISTS idx_deliveries_due ON webhook_deliveries(status,next_attempt_at);
`

func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		dsn = ":memory:"
	}
	if dsn != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dsn), 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", dsn+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}
	return db, nil
}
