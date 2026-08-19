#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${EDGE_ROLLOUT_DEV_PORT:-18081}"
BIN="${TMPDIR:-/tmp}/edge-rollout-control-dev"
DB="${TMPDIR:-/tmp}/edge-rollout-control-dev.db"
LOG="${TMPDIR:-/tmp}/edge-rollout-control-dev.log"
PID=""
cleanup() {
  if [[ -n "$PID" ]] && kill -0 "$PID" 2>/dev/null; then kill -TERM "$PID" 2>/dev/null || true; wait "$PID" 2>/dev/null || true; fi
  rm -f "$BIN" "$DB" "$DB-wal" "$DB-shm" "$LOG"
}
trap cleanup EXIT
cd "$ROOT"
go build -o "$BIN" ./cmd/edge-rollout
DATABASE_DSN="$DB" SERVER_ADDR=":$PORT" "$BIN" -config configs/config.yaml >"$LOG" 2>&1 & PID=$!
for _ in $(seq 1 100); do curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1 && break; sleep 0.1; done
curl -fsS "http://127.0.0.1:$PORT/healthz"; echo
curl -fsS "http://127.0.0.1:$PORT/readyz"; echo
device_json=$(curl -fsS -X POST "http://127.0.0.1:$PORT/v1/devices" -H 'Content-Type: application/json' -d '{"name":"pump-01","hardware_model":"pump-v1","software_version":"1.0.0","labels":{"area":"east"}}')
device_id=$(printf '%s' "$device_json" | sed -n 's/.*"ID":"\([^"]*\)".*/\1/p')
config_json=$(curl -fsS -X POST "http://127.0.0.1:$PORT/v1/configurations" -H 'Content-Type: application/json' -d '{"name":"pump-config","format":"json","content":"{\"sampling\":30}","compatible_models":["pump-v1"],"change_summary":"initial"}')
config_id=$(printf '%s' "$config_json" | sed -n 's/.*"ID":"\([^"]*\)".*/\1/p')
curl -fsS -X POST "http://127.0.0.1:$PORT/v1/configurations/$config_id/publish" -H 'X-Actor: verifier'; echo
rollout_json=$(curl -fsS -X POST "http://127.0.0.1:$PORT/v1/rollouts" -H 'Content-Type: application/json' -d "{\"name\":\"initial-rollout\",\"configuration_id\":\"$config_id\",\"device_ids\":[\"$device_id\"],\"strategy\":\"immediate\",\"batch_size\":1}")
rollout_id=$(printf '%s' "$rollout_json" | sed -n 's/.*"ID":"\([^"]*\)".*/\1/p')
curl -fsS -X POST "http://127.0.0.1:$PORT/v1/rollouts/$rollout_id/start" -H 'X-Actor: verifier'; echo
pending=$(curl -fsS "http://127.0.0.1:$PORT/v1/devices/$device_id/pending-configuration")
printf '%s\n' "$pending"
curl -fsS -X POST "http://127.0.0.1:$PORT/v1/devices/$device_id/receipts" -H 'Content-Type: application/json' -H 'Idempotency-Key: receipt-1' -d "{\"idempotency_key\":\"receipt-1\",\"rollout_id\":\"$rollout_id\",\"configuration_id\":\"$config_id\",\"status\":\"succeeded\",\"message\":\"applied\"}"; echo
curl -fsS -X POST "http://127.0.0.1:$PORT/v1/devices/$device_id/receipts" -H 'Content-Type: application/json' -H 'Idempotency-Key: receipt-1' -d "{\"idempotency_key\":\"receipt-1\",\"rollout_id\":\"$rollout_id\",\"configuration_id\":\"$config_id\",\"status\":\"succeeded\"}"; echo
curl -fsS "http://127.0.0.1:$PORT/v1/rollouts/$rollout_id"; echo
curl -fsS "http://127.0.0.1:$PORT/v1/audit-events?resource_id=$rollout_id"; echo
kill -TERM "$PID"; wait "$PID"; PID=""
echo "service stopped"
