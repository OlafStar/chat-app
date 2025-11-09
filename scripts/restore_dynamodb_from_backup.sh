#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/restore_dynamodb_from_backup.sh <backup_dir>

Examples:
  scripts/restore_dynamodb_from_backup.sh backups/dynamodb/2024-05-01T10-00-00Z
  DYNAMODB_ENDPOINT="http://localhost:9000" scripts/restore_dynamodb_from_backup.sh ./tmp/backup

The script removes the target tables, re-creates them via scripts/setup_dynamodb_local.sh,
and populates them with the backed-up data. It expects the docker compose
`dynamodb` service to be running (port 8000 exposed to the host).
USAGE
}

if [[ $# -eq 0 || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit $([[ $# -eq 0 ]] && echo 1 || echo 0)
fi

if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI is required but not found in PATH." >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required but not found in PATH." >&2
  exit 1
fi

BACKUP_DIR="${1%/}"
if [[ ! -d "$BACKUP_DIR" ]]; then
  echo "Backup directory $BACKUP_DIR does not exist." >&2
  exit 1
fi

AWS_REGION="${AWS_REGION:-eu-central-1}"
ENDPOINT_URL="${DYNAMODB_ENDPOINT:-http://localhost:8000}"

AWS_CMD=(aws --region "$AWS_REGION" --no-cli-pager)
if [[ -n "$ENDPOINT_URL" ]]; then
  AWS_CMD+=(--endpoint-url "$ENDPOINT_URL")
fi

read_manifest_tables() {
  local manifest="$BACKUP_DIR/manifest.json"
  if [[ -f "$manifest" ]]; then
    python3 - "$manifest" <<'PY'
import json
import sys

manifest_path = sys.argv[1]
with open(manifest_path, encoding="utf-8") as fh:
    manifest = json.load(fh)

for table in manifest.get("tables", []):
    print(table)
PY
    return
  fi
  # Fallback to file discovery (ignores manifest.json if present).
  for file in "$BACKUP_DIR"/*.json; do
    [[ ! -f "$file" ]] && continue
    [[ $(basename "$file") == "manifest.json" ]] && continue
    basename "$file" .json
  done
}

TABLES=()
while IFS= read -r table; do
  [[ -z "$table" ]] && continue
  TABLES+=("$table")
done < <(read_manifest_tables)
if [[ ${#TABLES[@]} -eq 0 ]]; then
  echo "No tables found in $BACKUP_DIR. Make sure it was produced by backup_dynamodb.sh." >&2
  exit 1
fi

echo "Restoring ${#TABLES[@]} table(s) from $BACKUP_DIR into $ENDPOINT_URL"

delete_table_if_exists() {
  local table=$1
  if "${AWS_CMD[@]}" dynamodb describe-table --table-name "$table" >/dev/null 2>&1; then
    echo "Removing existing table $table..."
    "${AWS_CMD[@]}" dynamodb delete-table --table-name "$table" >/dev/null
    "${AWS_CMD[@]}" dynamodb wait table-not-exists --table-name "$table"
  fi
}

for table in "${TABLES[@]}"; do
  delete_table_if_exists "$table"
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SETUP_SCRIPT="$SCRIPT_DIR/setup_dynamodb_local.sh"
if [[ ! -x "$SETUP_SCRIPT" ]]; then
  echo "Expected setup script at $SETUP_SCRIPT but could not execute it." >&2
  exit 1
fi

echo "Recreating tables via $SETUP_SCRIPT"
DYNAMODB_ENDPOINT="$ENDPOINT_URL" AWS_REGION="$AWS_REGION" "$SETUP_SCRIPT" >/dev/null

restore_table() {
  local table=$1
  local backup_file="$BACKUP_DIR/$table.json"
  if [[ ! -f "$backup_file" ]]; then
    echo "Skipping $table (missing $backup_file)"
    return
  fi
  echo "Restoring data for $table..."
  python3 - "$table" "$backup_file" "${AWS_CMD[@]}" <<'PY'
import json
import sys
import subprocess
import time

table = sys.argv[1]
backup_path = sys.argv[2]
aws_cmd = sys.argv[3:]

with open(backup_path, encoding="utf-8") as fh:
    blob = json.load(fh)

items = blob.get("Items", [])
if not items:
    print("  No rows to import.")
    sys.exit(0)

batch_size = 25
total_written = 0

for index in range(0, len(items), batch_size):
    pending = items[index:index + batch_size]
    retries = 0
    while pending:
        request = {table: [{"PutRequest": {"Item": item}} for item in pending]}
        cmd = aws_cmd + ["dynamodb", "batch-write-item", "--request-items", json.dumps(request)]
        proc = subprocess.run(cmd, capture_output=True, text=True)
        if proc.returncode != 0:
            sys.stderr.write(proc.stderr)
            raise SystemExit(proc.returncode)
        resp = json.loads(proc.stdout or "{}")
        unprocessed = resp.get("UnprocessedItems", {}).get(table, [])
        pending = [item["PutRequest"]["Item"] for item in unprocessed]
        if pending:
            retries += 1
            time.sleep(min(2 ** retries, 5))
        else:
            total_written += len(request[table])

print(f"  Imported {total_written} item(s).")
PY
}

for table in "${TABLES[@]}"; do
  restore_table "$table"
done

echo "Done. Local DynamoDB now mirrors $BACKUP_DIR."
