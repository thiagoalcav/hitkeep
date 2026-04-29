#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

SEED_DB="${HITKEEP_DB:-${HITKEEP_DB_PATH:-$REPO_DIR/hitkeep.db}}"
SEED_DATA_PATH="${HITKEEP_DATA_PATH:-$REPO_DIR/data}"
SEED_EMAIL="${HITKEEP_SEED_EMAIL:-demo@example.com}"
SEED_PASSWORD="${HITKEEP_SEED_PASSWORD:-demo1234}"
SEED_DOMAIN="${HITKEEP_SEED_DOMAIN:-acme-analytics.io}"
SEED_DAYS="${HITKEEP_SEED_DAYS:-90}"
BACKEND_ADDR="${HITKEEP_HTTP_ADDR:-:8080}"
RUN_SEED=0
RUN_CLOUD=0

usage() {
  cat <<'EOF'
Usage:
  make dev
  make dev DEV_ARGS=--seed
  make dev DEV_ARGS="--cloud --seed"
  make dev-seed
  make dev-cloud
  make dev-cloud-seed

Options:
  --cloud   Start the backend with cloud/billing build tags and local cloud defaults
  --seed    Seed the development database before starting backend/frontend
  --help    Show this help

Environment overrides:
  HITKEEP_DB              Database path to seed (default: ./hitkeep.db)
  HITKEEP_DATA_PATH       Tenant data directory (default: ./data)
  HITKEEP_SEED_EMAIL      Seed user email (default: demo@example.com)
  HITKEEP_SEED_PASSWORD   Seed user password (default: demo1234)
  HITKEEP_SEED_DOMAIN     Seed site domain (default: acme-analytics.io)
  HITKEEP_SEED_DAYS       Days of seed traffic (default: 90)
  HITKEEP_HTTP_ADDR        Backend listen address (default: :8080)

Backend mail defaults for local dev:
  HITKEEP_MAIL_DRIVER=smtp
  HITKEEP_MAIL_HOST=localhost
  HITKEEP_MAIL_PORT=1025
  HITKEEP_MAIL_ENCRYPTION=none

Cloud defaults for local dev:
  HITKEEP_CLOUD_HOSTED=true
  HITKEEP_CLOUD_SIGNUP_ENABLED=true
  HITKEEP_CLOUD_JURISDICTION=EU
  HITKEEP_CLOUD_REGION=eu-central-1
EOF
}

backend_port() {
  local addr="$1"
  local port="${addr##*:}"
  if [[ "$port" =~ ^[0-9]+$ ]]; then
    printf '%s\n' "$port"
  fi
}

ensure_backend_port_available() {
  local port occupant
  port="$(backend_port "$BACKEND_ADDR")"
  if [[ -z "$port" ]] || ! command -v lsof >/dev/null 2>&1; then
    return
  fi

  occupant="$(lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | awk 'NR == 2 { print $1 " PID " $2 }' || true)"
  if [[ -z "$occupant" ]]; then
    return
  fi

  cat >&2 <<EOF
Port $port is already in use by $occupant.
HitKeep dev needs the backend on $BACKEND_ADDR so the Angular proxy can reach /api.

Stop the process using port $port, or start HitKeep on another port and update
frontend/dashboard/proxy.conf.json to match.
EOF
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cloud)
      RUN_CLOUD=1
      shift
      ;;
    --seed)
      RUN_SEED=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

cd "$REPO_DIR"

ensure_backend_port_available

if ! command -v air >/dev/null 2>&1; then
  echo "Air is not installed. Installing..."
  go install github.com/air-verse/air@latest
fi

if (( RUN_SEED )); then
  echo "Seeding development data into $SEED_DB (tenant data: $SEED_DATA_PATH)..."
  go run ./cmd/seed \
    -db "$SEED_DB" \
    -data-path "$SEED_DATA_PATH" \
    -email "$SEED_EMAIL" \
    -password "$SEED_PASSWORD" \
    -domain "$SEED_DOMAIN" \
    -days "$SEED_DAYS"
fi

backend_target="dev-backend"
env_args=(
  "HITKEEP_DB_PATH=$SEED_DB"
  "HITKEEP_DATA_PATH=$SEED_DATA_PATH"
  "HITKEEP_JWT_SECRET=${HITKEEP_JWT_SECRET:-hitkeep-dev-jwt-secret}"
  "HITKEEP_PUBLIC_URL=${HITKEEP_PUBLIC_URL:-http://localhost:4200}"
  "HITKEEP_MAIL_DRIVER=${HITKEEP_MAIL_DRIVER:-smtp}"
  "HITKEEP_MAIL_HOST=${HITKEEP_MAIL_HOST:-localhost}"
  "HITKEEP_MAIL_PORT=${HITKEEP_MAIL_PORT:-1025}"
  "HITKEEP_MAIL_ENCRYPTION=${HITKEEP_MAIL_ENCRYPTION:-none}"
  "HITKEEP_MCP_ENABLED=${HITKEEP_MCP_ENABLED:-true}"
)

if (( RUN_CLOUD )); then
  backend_target="dev-cloud-backend"
  env_args+=(
    "HITKEEP_CLOUD_HOSTED=${HITKEEP_CLOUD_HOSTED:-true}"
    "HITKEEP_CLOUD_SIGNUP_ENABLED=${HITKEEP_CLOUD_SIGNUP_ENABLED:-true}"
    "HITKEEP_CLOUD_JURISDICTION=${HITKEEP_CLOUD_JURISDICTION:-EU}"
    "HITKEEP_CLOUD_REGION=${HITKEEP_CLOUD_REGION:-eu-central-1}"
    "HITKEEP_CLOUD_UPGRADE_URL=${HITKEEP_CLOUD_UPGRADE_URL:-http://localhost:4200/admin/team}"
    "HITKEEP_CLOUD_SUPPORT_URL=${HITKEEP_CLOUD_SUPPORT_URL:-https://hitkeep.com/support/help/}"
    "HITKEEP_CLOUD_CHECKOUT_SUCCESS_URL=${HITKEEP_CLOUD_CHECKOUT_SUCCESS_URL:-http://localhost:4200/admin/team?checkout=success}"
    "HITKEEP_CLOUD_CHECKOUT_CANCEL_URL=${HITKEEP_CLOUD_CHECKOUT_CANCEL_URL:-http://localhost:4200/admin/team?checkout=cancelled}"
  )
fi

if (( RUN_CLOUD )); then
  echo "Starting development environment (cloud/billing)..."
else
  echo "Starting development environment..."
fi
echo "Mail defaults: smtp via Mailpit on localhost:1025 (no TLS)"

if (( RUN_CLOUD )); then
  echo "Cloud defaults: hosted signup in EU / eu-central-1"
fi

exec env "${env_args[@]}" make -j2 "$backend_target" dev-frontend
