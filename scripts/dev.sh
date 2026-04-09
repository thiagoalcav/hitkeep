#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

SEED_DB="${HITKEEP_DB:-$REPO_DIR/hitkeep.db}"
SEED_DATA_PATH="${HITKEEP_DATA_PATH:-$REPO_DIR/data}"
SEED_EMAIL="${HITKEEP_SEED_EMAIL:-demo@example.com}"
SEED_PASSWORD="${HITKEEP_SEED_PASSWORD:-demo1234}"
SEED_DOMAIN="${HITKEEP_SEED_DOMAIN:-acme-analytics.io}"
SEED_DAYS="${HITKEEP_SEED_DAYS:-90}"
RUN_SEED=0

usage() {
  cat <<'EOF'
Usage:
  make dev
  make dev DEV_ARGS=--seed
  make dev-seed

Options:
  --seed    Seed the development database before starting backend/frontend
  --help    Show this help

Environment overrides:
  HITKEEP_DB              Database path to seed (default: ./hitkeep.db)
  HITKEEP_DATA_PATH       Tenant data directory (default: ./data)
  HITKEEP_SEED_EMAIL      Seed user email (default: demo@example.com)
  HITKEEP_SEED_PASSWORD   Seed user password (default: demo1234)
  HITKEEP_SEED_DOMAIN     Seed site domain (default: acme-analytics.io)
  HITKEEP_SEED_DAYS       Days of seed traffic (default: 90)

Backend mail defaults for local dev:
  HITKEEP_MAIL_DRIVER=smtp
  HITKEEP_MAIL_HOST=localhost
  HITKEEP_MAIL_PORT=1025
  HITKEEP_MAIL_ENCRYPTION=none
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
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

echo "Starting development environment..."
echo "Mail defaults: smtp via Mailpit on localhost:1025 (no TLS)"

exec env \
  HITKEEP_DB_PATH="$SEED_DB" \
  HITKEEP_DATA_PATH="$SEED_DATA_PATH" \
  HITKEEP_PUBLIC_URL="${HITKEEP_PUBLIC_URL:-http://localhost:4200}" \
  HITKEEP_MAIL_DRIVER="${HITKEEP_MAIL_DRIVER:-smtp}" \
  HITKEEP_MAIL_HOST="${HITKEEP_MAIL_HOST:-localhost}" \
  HITKEEP_MAIL_PORT="${HITKEEP_MAIL_PORT:-1025}" \
  HITKEEP_MAIL_ENCRYPTION="${HITKEEP_MAIL_ENCRYPTION:-none}" \
  make -j2 dev-backend dev-frontend
