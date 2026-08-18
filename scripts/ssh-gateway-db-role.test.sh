#!/usr/bin/env bash
set -euo pipefail

# Hermetic regression for w5/m72: routine deploy reconciliation must apply the
# exact grant surface without touching credentials, Secrets, or Deployments.
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin"
capture="$tmp/sql"

cat >"$tmp/bin/kubectl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *" get pod "* ]]; then
  printf 'bex-db-1'
  exit 0
fi
if [[ "$*" == *" exec -i bex-db-1 -- psql "* ]]; then
  cat >"$BEX_TEST_SQL_CAPTURE"
  exit 0
fi
echo "unexpected kubectl mutation in grants-only mode: $*" >&2
exit 97
SH
chmod +x "$tmp/bin/kubectl"

PATH="$tmp/bin:/usr/bin:/bin" \
  BEX_TEST_SQL_CAPTURE="$capture" \
  GRANTS_ONLY=1 \
  bash "$root/scripts/ssh-gateway-db-role.sh" >"$tmp/output"

grep -q "REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM bex_ssh_gateway" "$capture"
grep -q "GRANT SELECT ON agent_session_turns TO bex_ssh_gateway" "$capture"
grep -q "GRANT SELECT, INSERT ON agent_session_transcripts TO bex_ssh_gateway" "$capture"
if grep -Eq "ALTER ROLE|CREATE ROLE|PASSWORD" "$capture"; then
  echo "grant-only SQL unexpectedly changes credentials" >&2
  exit 1
fi
grep -q "credential unchanged" "$tmp/output"

if GRANTS_ONLY=1 BEX_SSH_GATEWAY_ROLE='bad;role' bash "$root/scripts/ssh-gateway-db-role.sh" >"$tmp/bad.out" 2>&1; then
  echo "invalid role name was accepted" >&2
  exit 1
fi
grep -q "invalid PostgreSQL role name" "$tmp/bad.out"

echo "PASS: ssh-gateway grant-only reconciliation"
