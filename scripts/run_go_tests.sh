#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"

# Provide sane defaults for required environment variables so tests can run locally.
export AWS_REGION="${AWS_REGION:-us-east-1}"
export AWS_ID="${AWS_ID:-local}"
export AWS_SECRET="${AWS_SECRET:-local-secret}"
export USER_SECRET="${USER_SECRET:-local-user-secret}"
export AUTH_REDIS_URL="${AUTH_REDIS_URL:-redis://localhost:6379/0}"
export CHAT_REDIS_URL="${CHAT_REDIS_URL:-redis://localhost:6379/1}"
export WEB_URL="${WEB_URL:-http://localhost:3000}"

# Optional variables with safe defaults for local testing.
export DYNAMODB_ENDPOINT="${DYNAMODB_ENDPOINT:-http://localhost:8000}"
export AWS_TOKEN="${AWS_TOKEN:-}"
export AUTH_REDIS_PASS="${AUTH_REDIS_PASS:-}"
export CHAT_REDIS_PASS="${CHAT_REDIS_PASS:-}"

cd "$BACKEND_DIR"
go test "${@:-./...}"
