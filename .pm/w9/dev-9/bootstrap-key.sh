#!/usr/bin/env bash
# One-shot: mint a real tenant-bound bex API key for the CLI-compat harness
# (t001) via Kratos native registration -> first authenticated call
# (EnsureTenant) -> POST /v1/api-keys, then save id+secret + the tenant id to
# .pm/w9/dev-9/.cli-key.env. Idempotent-ish: registers a fresh throwaway user
# each run (Kratos requires unique emails), so re-running mints a new tenant +
# key rather than reusing one.
set -euo pipefail
cd "$(dirname "$0")"
# shellcheck disable=SC1091
source ports.env
KRATOS="http://localhost:$KRATOS_PUBLIC_PORT"
BEXAPI="http://localhost:$BEX_API_PORT"

# jget FIELD-EXPR — pull one field from a JSON blob on stdin, e.g. jget '["id"]'.
jget() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)"; }

EMAIL="cli-compat-$(date +%s)-$$@example.com"
FLOW=$(curl -s "$KRATOS/self-service/registration/api" | jget '["id"]')
RESP=$(curl -s -X POST "$KRATOS/self-service/registration?flow=$FLOW" \
  -H 'Content-Type: application/json' \
  -d "{\"method\":\"password\",\"traits\":{\"email\":\"$EMAIL\"},\"password\":\"Sup3rSecret!9x\"}")
TOK=$(echo "$RESP" | jget '["session_token"]')

curl -s -o /dev/null -H "X-Session-Token: $TOK" "$BEXAPI/v1/services"

KEY=$(curl -s -X POST -H "X-Session-Token: $TOK" -H 'Content-Type: application/json' \
  -d '{"name":"cli-compat-harness"}' "$BEXAPI/v1/api-keys")
read -r KEY_ID KEY_SECRET <<<"$(echo "$KEY" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["id"], d["secret"])')"

OWNER=$(curl -s -H "X-Session-Token: $TOK" "$BEXAPI/v1/owners")
TENANT_ID=$(echo "$OWNER" | jget '[0]["owner"]["id"]')

cat > .cli-key.env <<EOF
CLI_COMPAT_KEY_ID=$KEY_ID
CLI_COMPAT_KEY_SECRET=$KEY_SECRET
CLI_COMPAT_TENANT_ID=$TENANT_ID
CLI_COMPAT_EMAIL=$EMAIL
EOF
echo "minted key $KEY_ID bound to tenant $TENANT_ID (see .cli-key.env)"
