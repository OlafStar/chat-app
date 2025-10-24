#!/usr/bin/env bash
# Fix permission issues for the DynamoDB Local volume to allow table creation.

set -euo pipefail

SERVICE_NAME="${DYNAMODB_SERVICE:-dynamodb}"
DATA_PATH="${DYNAMODB_DATA_PATH:-/home/dynamodblocal/data}"
DATA_USER="${DYNAMODB_USER:-dynamodblocal}"
DATA_GROUP="${DYNAMODB_GROUP:-$DATA_USER}"
RESTART_SERVICE="${RESTART_DYNAMODB_AFTER_FIX:-true}"

if command -v docker-compose >/dev/null 2>&1; then
  compose() {
    docker-compose "$@"
  }
else
  compose() {
    docker compose "$@"
  }
fi

echo "Ensuring DynamoDB service '${SERVICE_NAME}' is running..."
container_id="$(compose ps -q "${SERVICE_NAME}" || true)"
if [[ -z "${container_id}" ]]; then
  echo "Starting '${SERVICE_NAME}' via docker compose..."
  compose up -d "${SERVICE_NAME}"
  container_id="$(compose ps -q "${SERVICE_NAME}" || true)"
  if [[ -z "${container_id}" ]]; then
    echo "Failed to start '${SERVICE_NAME}'. Exiting." >&2
    exit 1
  fi
fi

echo "Adjusting ownership of '${DATA_PATH}' to ${DATA_USER}:${DATA_GROUP}..."
compose exec --user root "${SERVICE_NAME}" \
  chown -R "${DATA_USER}:${DATA_GROUP}" "${DATA_PATH}"

if [[ "${RESTART_SERVICE}" == "true" ]]; then
  echo "Restarting '${SERVICE_NAME}' to apply changes..."
  compose restart "${SERVICE_NAME}"
else
  echo "Skipping restart as RESTART_DYNAMODB_AFTER_FIX=${RESTART_SERVICE}."
fi

echo "Permission fix complete."
