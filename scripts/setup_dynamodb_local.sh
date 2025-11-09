#!/usr/bin/env bash
set -euo pipefail

# Configure endpoint/region for local DynamoDB; override via env when needed.
ENDPOINT_URL="${DYNAMODB_ENDPOINT:-http://localhost:8000}"
AWS_REGION="${AWS_REGION:-eu-central-1}"

aws_dynamodb() {
  aws --region "$AWS_REGION" --endpoint-url "$ENDPOINT_URL" --no-cli-pager dynamodb "$@"
}

create_table() {
  local table_name=$1
  local payload=$2

  if aws_dynamodb describe-table --table-name "$table_name" >/dev/null 2>&1; then
    echo "Table $table_name already exists; skipping."
    return
  fi

  echo "Creating table $table_name..."
  aws_dynamodb create-table --cli-input-json "$payload" >/dev/null
  echo "Table $table_name created."
}

ensure_ttl() {
  local table_name=$1
  local attribute=$2

  local status
  status=$(aws_dynamodb describe-time-to-live --table-name "$table_name" \
    --query "TimeToLiveDescription.TimeToLiveStatus" --output text 2>/dev/null || echo "DISABLED")

  if [[ "$status" == "ENABLED" ]]; then
    echo "TTL already enabled on $table_name ($attribute)."
    return
  fi

  echo "Enabling TTL on $table_name ($attribute)..."
  aws_dynamodb update-time-to-live \
    --table-name "$table_name" \
    --time-to-live-specification "Enabled=true,AttributeName=$attribute" >/dev/null
  echo "TTL enabled on $table_name."
}

echo "Using DynamoDB endpoint $ENDPOINT_URL (region $AWS_REGION)"

tenants_table=$(cat <<'JSON'
{
  "TableName": "Tenants",
  "AttributeDefinitions": [
    {"AttributeName": "tenantId", "AttributeType": "S"}
  ],
  "KeySchema": [
    {"AttributeName": "tenantId", "KeyType": "HASH"}
  ],
  "BillingMode": "PAY_PER_REQUEST"
}
JSON
)

users_table=$(cat <<'JSON'
{
  "TableName": "Users",
  "AttributeDefinitions": [
    {"AttributeName": "pk", "AttributeType": "S"},
    {"AttributeName": "email", "AttributeType": "S"},
    {"AttributeName": "tenantId", "AttributeType": "S"},
    {"AttributeName": "createdAt", "AttributeType": "S"}
  ],
  "KeySchema": [
    {"AttributeName": "pk", "KeyType": "HASH"}
  ],
  "BillingMode": "PAY_PER_REQUEST",
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "byEmail",
      "KeySchema": [
        {"AttributeName": "email", "KeyType": "HASH"},
        {"AttributeName": "tenantId", "KeyType": "RANGE"}
      ],
      "Projection": {"ProjectionType": "ALL"}
    },
    {
      "IndexName": "byTenant",
      "KeySchema": [
        {"AttributeName": "tenantId", "KeyType": "HASH"},
        {"AttributeName": "createdAt", "KeyType": "RANGE"}
      ],
      "Projection": {"ProjectionType": "ALL"}
    }
  ]
}
JSON
)

tenant_invites_table=$(cat <<'JSON'
{
  "TableName": "TenantInvites",
  "AttributeDefinitions": [
    {"AttributeName": "token", "AttributeType": "S"},
    {"AttributeName": "tenantEmail", "AttributeType": "S"},
    {"AttributeName": "createdAt", "AttributeType": "S"}
  ],
  "KeySchema": [
    {"AttributeName": "token", "KeyType": "HASH"}
  ],
  "BillingMode": "PAY_PER_REQUEST",
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "byTenantEmail",
      "KeySchema": [
        {"AttributeName": "tenantEmail", "KeyType": "HASH"},
        {"AttributeName": "createdAt", "KeyType": "RANGE"}
      ],
      "Projection": {"ProjectionType": "ALL"}
    }
  ]
}
JSON
)

conversations_table=$(cat <<'JSON'
{
  "TableName": "Conversations",
  "AttributeDefinitions": [
    {"AttributeName": "pk", "AttributeType": "S"},
    {"AttributeName": "tenantId", "AttributeType": "S"},
    {"AttributeName": "tenantVisitor", "AttributeType": "S"},
    {"AttributeName": "lastMessageAt", "AttributeType": "S"}
  ],
  "KeySchema": [
    {"AttributeName": "pk", "KeyType": "HASH"}
  ],
  "BillingMode": "PAY_PER_REQUEST",
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "byTenant",
      "KeySchema": [
        {"AttributeName": "tenantId", "KeyType": "HASH"},
        {"AttributeName": "lastMessageAt", "KeyType": "RANGE"}
      ],
      "Projection": {"ProjectionType": "ALL"}
    },
    {
      "IndexName": "byVisitor",
      "KeySchema": [
        {"AttributeName": "tenantVisitor", "KeyType": "HASH"},
        {"AttributeName": "lastMessageAt", "KeyType": "RANGE"}
      ],
      "Projection": {"ProjectionType": "ALL"}
    }
  ]
}
JSON
)

messages_table=$(cat <<'JSON'
{
  "TableName": "Messages",
  "AttributeDefinitions": [
    {"AttributeName": "pk", "AttributeType": "S"},
    {"AttributeName": "conversationId", "AttributeType": "S"},
    {"AttributeName": "createdAt", "AttributeType": "S"}
  ],
  "KeySchema": [
    {"AttributeName": "pk", "KeyType": "HASH"}
  ],
  "BillingMode": "PAY_PER_REQUEST",
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "byConversation",
      "KeySchema": [
        {"AttributeName": "conversationId", "KeyType": "HASH"},
        {"AttributeName": "createdAt", "KeyType": "RANGE"}
      ],
      "Projection": {"ProjectionType": "ALL"}
    }
  ]
}
JSON
)

visitors_table=$(cat <<'JSON'
{
  "TableName": "Visitors",
  "AttributeDefinitions": [
    {"AttributeName": "pk", "AttributeType": "S"}
  ],
  "KeySchema": [
    {"AttributeName": "pk", "KeyType": "HASH"}
  ],
  "BillingMode": "PAY_PER_REQUEST"
}
JSON
)

tenant_api_keys_table=$(cat <<'JSON'
{
  "TableName": "TenantAPIKeys",
  "AttributeDefinitions": [
    { "AttributeName": "tenantId", "AttributeType": "S" },
    { "AttributeName": "keyId", "AttributeType": "S" },
    { "AttributeName": "apiKey", "AttributeType": "S" },
    { "AttributeName": "createdAt", "AttributeType": "S" }
  ],
  "KeySchema": [
    { "AttributeName": "tenantId", "KeyType": "HASH" },
    { "AttributeName": "keyId", "KeyType": "RANGE" }
  ],
  "BillingMode": "PAY_PER_REQUEST",
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "byApiKey",
      "KeySchema": [
        { "AttributeName": "apiKey", "KeyType": "HASH" },
        {"AttributeName": "createdAt", "KeyType": "RANGE"}
      ],
      "Projection": { "ProjectionType": "ALL" }
    }
  ]
}
JSON
)

create_table "Tenants" "$tenants_table"
create_table "Users" "$users_table"
create_table "TenantInvites" "$tenant_invites_table"
create_table "Conversations" "$conversations_table"
create_table "Messages" "$messages_table"
create_table "Visitors" "$visitors_table"
create_table "TenantAPIKeys" "$tenant_api_keys_table"

ensure_ttl "Messages" "expireAt"

echo "DynamoDB local setup complete."
