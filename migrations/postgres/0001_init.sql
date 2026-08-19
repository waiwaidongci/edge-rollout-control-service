CREATE TABLE devices (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    hardware_model TEXT NOT NULL,
    software_version TEXT NOT NULL,
    labels JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL,
    current_configuration_id UUID,
    last_heartbeat_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX devices_name_key ON devices(name);
CREATE INDEX devices_labels_idx ON devices USING GIN(labels);

CREATE TABLE device_groups (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    selector TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE configurations (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    version INTEGER NOT NULL,
    format TEXT NOT NULL,
    content TEXT NOT NULL,
    content_sha256 TEXT NOT NULL,
    compatible_models JSONB NOT NULL DEFAULT '[]',
    variables JSONB NOT NULL DEFAULT '{}',
    change_summary TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(name, version)
);

CREATE TABLE rollout_rules (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    max_failure_rate DOUBLE PRECISION NOT NULL,
    max_offline_rate DOUBLE PRECISION NOT NULL,
    max_timeout_rate DOUBLE PRECISION NOT NULL,
    critical_labels JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE rollouts (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    configuration_id UUID NOT NULL REFERENCES configurations(id),
    group_id UUID REFERENCES device_groups(id),
    strategy TEXT NOT NULL,
    batch_size INTEGER NOT NULL,
    batch_percent INTEGER NOT NULL,
    scheduled_at TIMESTAMPTZ,
    status TEXT NOT NULL,
    current_batch INTEGER NOT NULL DEFAULT 0,
    target_count INTEGER NOT NULL,
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    timeout_count INTEGER NOT NULL DEFAULT 0,
    pause_reason TEXT NOT NULL DEFAULT '',
    rollback_configuration_id UUID REFERENCES configurations(id),
    rule_id UUID REFERENCES rollout_rules(id),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);
CREATE INDEX rollouts_status_schedule_idx ON rollouts(status, scheduled_at);

CREATE TABLE rollout_targets (
    id UUID PRIMARY KEY,
    rollout_id UUID NOT NULL REFERENCES rollouts(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id),
    batch_number INTEGER NOT NULL,
    status TEXT NOT NULL,
    desired_configuration_id UUID NOT NULL REFERENCES configurations(id),
    delivered_at TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(rollout_id, device_id)
);
CREATE INDEX rollout_targets_device_status_idx ON rollout_targets(device_id, status);

CREATE TABLE receipts (
    id UUID PRIMARY KEY,
    idempotency_key TEXT NOT NULL UNIQUE,
    rollout_id UUID NOT NULL REFERENCES rollouts(id),
    device_id UUID NOT NULL REFERENCES devices(id),
    configuration_id UUID NOT NULL REFERENCES configurations(id),
    status TEXT NOT NULL,
    error_code TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    device_timestamp TIMESTAMPTZ,
    received_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE audit_events (
    id UUID PRIMARY KEY,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    details JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX audit_events_resource_idx ON audit_events(resource_type, resource_id, created_at DESC);

CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    secret TEXT NOT NULL,
    event_types JSONB NOT NULL DEFAULT '[]',
    enabled BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY,
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id),
    event_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    last_status_code INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX webhook_deliveries_due_idx ON webhook_deliveries(status, next_attempt_at);
