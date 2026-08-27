#!/usr/bin/env bash
# Smoke test for the shotcrete closure service. It builds the server binary,
# starts it against a temporary persistent store, probes its local health and
# API behaviour, and cleans up every process and temporary file. It performs no
# external network access and does not call go test.
set -euo pipefail

PORT="${PORT:-18080}"
BIN_DIR="$(mktemp -d)"
SERVER_PID=""

cleanup() {
  if [ -n "${SERVER_PID}" ]; then
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
  rm -rf "${BIN_DIR}"
}
trap cleanup EXIT

echo "building server binary..."
go build -o "${BIN_DIR}/server" ./cmd/server

echo "starting server on 127.0.0.1:${PORT} ..."
ADDR="127.0.0.1:${PORT}" DATA_PATH="${BIN_DIR}/smoke.db" \
  "${BIN_DIR}/server" &
SERVER_PID=$!

# Wait up to ~5 seconds for the service to become healthy.
healthy=""
for _ in $(seq 1 50); do
  if resp="$(curl -s "http://127.0.0.1:${PORT}/healthz")"; then
    if printf '%s' "${resp}" | grep -q '"status":"ok"'; then
      healthy="${resp}"
      break
    fi
  fi
  sleep 0.1
done

if [ -z "${healthy}" ]; then
  echo "service did not become healthy" >&2
  exit 1
fi
echo "health OK: ${healthy}"

# Assert a missing cycle returns a deterministic 404.
missing="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${PORT}/v1/cycles/nope")"
if [ "${missing}" != "404" ]; then
  echo "expected 404 for missing cycle, got ${missing}" >&2
  exit 1
fi
echo "missing-cycle 404 OK"

# A malformed lock request must be rejected with a 400 and a stable error code.
lock_body="$(curl -s -w '\n%{http_code}' \
  -X POST "http://127.0.0.1:${PORT}/v1/cycles" \
  -H 'Content-Type: application/json' \
  -d '{"digest":"d1"}')"
lock_code="$(printf '%s' "${lock_body}" | tail -n1)"
if [ "${lock_code}" != "400" ]; then
  echo "expected 400 for malformed lock, got ${lock_code}" >&2
  exit 1
fi
if ! printf '%s' "${lock_body}" | grep -q 'INVALID_REQUEST'; then
  echo "expected INVALID_REQUEST error code in lock response" >&2
  exit 1
fi
echo "malformed-lock 400 OK"

echo "smoke OK"
