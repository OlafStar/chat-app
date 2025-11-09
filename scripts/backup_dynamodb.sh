#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/backup_dynamodb.sh [output_root]

Examples:
  scripts/backup_dynamodb.sh
  TABLES="Tenants Users" scripts/backup_dynamodb.sh ./my-backups

When output_root is omitted, backups/dynamodb is used. A timestamped folder is
created on each run (e.g. backups/dynamodb/2024-05-01T10-00-00Z). The script
talks to the local DynamoDB container exposed via docker compose (port 8000);
ensure `docker compose up dynamodb` is running beforehand.
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI is required but not found in PATH." >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required but not found in PATH." >&2
  exit 1
fi

AWS_REGION="${AWS_REGION:-eu-central-1}"
ENDPOINT_URL="${DYNAMODB_ENDPOINT:-http://localhost:8000}"
OUTPUT_ROOT="${1:-backups/dynamodb}"
TIMESTAMP=$(date -u +"%Y-%m-%dT%H-%M-%SZ")
BACKUP_DIR="${OUTPUT_ROOT%/}/$TIMESTAMP"

mkdir -p "$BACKUP_DIR"

AWS_CMD=(aws --region "$AWS_REGION" --no-cli-pager --endpoint-url "$ENDPOINT_URL")

TABLE_LIST=()
if [[ -n "${TABLES:-}" ]]; then
  read -r -a TABLE_LIST <<<"${TABLES}"
else
  TMP_ERR=$(mktemp)
  if ! LIST_TABLES_JSON=$("${AWS_CMD[@]}" dynamodb list-tables --output json 2>"$TMP_ERR"); then
    echo "Failed to list tables via local DynamoDB (endpoint $ENDPOINT_URL)." >&2
    cat "$TMP_ERR" >&2 || true
    rm -f "$TMP_ERR"
    exit 1
  fi
  rm -f "$TMP_ERR"
  while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    TABLE_LIST+=("$name")
  done < <(
    python3 - "$LIST_TABLES_JSON" <<'PY'
import json
import sys

try:
    data = json.loads(sys.argv[1])
except json.JSONDecodeError as exc:
    sys.stderr.write(f"Failed to parse list-tables output: {exc}\n")
    sys.exit(1)

for name in data.get("TableNames", []):
    if name:
        print(name)
PY
  )
fi

if [[ ${#TABLE_LIST[@]} -eq 0 ]]; then
  echo "No tables found to back up. Either specify TABLES or ensure the source account has tables." >&2
  exit 1
fi

echo "Backing up ${#TABLE_LIST[@]} table(s) to $BACKUP_DIR"
for table in "${TABLE_LIST[@]}"; do
  echo "→ $table"
  "${AWS_CMD[@]}" dynamodb scan \
    --table-name "$table" \
    --consistent-read \
    --output json >"$BACKUP_DIR/$table.json"
done

MANIFEST="$BACKUP_DIR/manifest.json"
python3 - "$MANIFEST" "$TIMESTAMP" "$AWS_REGION" "$ENDPOINT_URL" "${TABLE_LIST[@]}" <<'PY'
import json
import sys

out = sys.argv[1]
timestamp = sys.argv[2]
region = sys.argv[3]
endpoint = sys.argv[4] or None
tables = sys.argv[5:]

manifest = {
    "createdAt": timestamp,
    "awsRegion": region,
    "sourceEndpoint": endpoint,
    "tableCount": len(tables),
    "tables": tables,
}

with open(out, "w", encoding="utf-8") as fh:
    json.dump(manifest, fh, indent=2)
PY

cat <<EOF
Backup complete.
  Folder : $BACKUP_DIR
  Tables : ${TABLE_LIST[*]}
EOF
