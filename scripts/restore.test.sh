#!/usr/bin/env bash
# Hermetic tests for restore tooling. Live recovery belongs in production drill
# records; CI only tests pure selection/validation/integrity and DRY_RUN safety.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/bex-restore-test.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT
FAKE="$TMP/bin"
mkdir -p "$FAKE"

pass=0
fail() { echo "not ok: $*" >&2; exit 1; }
ok() { pass=$((pass + 1)); echo "ok $pass - $*"; }

cat >"$FAKE/aws" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'aws %s\n' "$*" >>"$RESTORE_TEST_CALLS"
if [[ " $* " == *" s3api list-objects-v2 "* ]]; then
  printf '%s\t%s\t%s\n' \
    'fixture/2026-07-31T03:00:00Z.rdb.gz' \
    'fixture/2026-08-01T03:00:00Z.rdb.gz' \
    'fixture/2026-07-30T03:00:00Z.rdb.gz'
  printf '%s\t%s\n' \
    'etcd-snapshots/etcd-2026-07-31T03:00:00Z.db.gz' \
    'etcd-snapshots/etcd-2026-08-01T03:00:00Z.db.gz'
  printf '%s\t%s\n' \
    'openbao-snapshots/openbao-2026-07-31T03:00:00Z.snap.gz' \
    'openbao-snapshots/openbao-2026-08-01T03:00:00Z.snap.gz'
  printf '%s\n' 'keyvalue/red-fixture/2026-08-01T03:00:00Z.rdb.gz'
  # ADR050: an age-encrypted object under an isolated prefix, so the pre-existing
  # plain-object selections above are unaffected.
  printf '%s\t%s\n' \
    'agefix/2026-07-31T03:00:00Z.rdb.gz' \
    'agefix/2026-08-01T03:00:00Z.rdb.gz.age'
  exit 0
fi
if [[ " $* " == *" s3 cp "* ]]; then
  destination="${!#}"
  /bin/cp "$RESTORE_TEST_ARCHIVE" "$destination"
  exit 0
fi
exit 1
EOF

cat >"$FAKE/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'docker %s\n' "$*" >>"$RESTORE_TEST_CALLS"
if [[ " $* " == *" snapshot restore "* ]]; then
  for arg in "$@"; do
    if [[ "$arg" == *:/work ]]; then
      /bin/mkdir -p "${arg%%:*}/restored"
      break
    fi
  done
  exit 0
fi
if [[ " $* " == *" --keys-only "* ]]; then
  echo '/registry/app.bex.co/apps/default/fixture-app'
  exit 0
fi
if [[ " $* " == *" --print-value-only "* ]]; then
  echo '{"apiVersion":"app.bex.co/v1alpha1","kind":"App","metadata":{"name":"fixture-app","namespace":"default","uid":"strip-me"},"spec":{"image":"example.invalid/fixture"},"status":{"phase":"Running"}}'
  exit 0
fi
if [[ " $* " == *" run -d "* ]]; then
  echo fake-container-id
fi
exit 0
EOF

cat >"$FAKE/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'kubectl %s\n' "$*" >>"$RESTORE_TEST_CALLS"
if [[ " $* " == *" get clusters.postgresql.cnpg.io "* ]]; then
  cat <<'JSON'
{"apiVersion":"postgresql.cnpg.io/v1","kind":"Cluster","metadata":{"name":"source-pg","namespace":"source"},"spec":{"instances":2,"imageName":"ghcr.io/cloudnative-pg/postgresql:18.4-system-trixie","affinity":{"nodeSelector":{"bex.co/pool":"platform"}},"storage":{"size":"5Gi","storageClass":"hcloud-volumes"},"resources":{"requests":{"cpu":"100m","memory":"256Mi"},"limits":{"cpu":"1","memory":"1Gi"}}}}
JSON
  exit 0
fi
if [[ " $* " == *" get objectstores.barmancloud.cnpg.io "* ]]; then
  cat <<'JSON'
{"apiVersion":"barmancloud.cnpg.io/v1","kind":"ObjectStore","metadata":{"name":"source-store","namespace":"source"},"spec":{"configuration":{"destinationPath":"s3://fixture/postgres","endpointURL":"https://s3.invalid","s3Credentials":{"accessKeyId":{"name":"backup-s3","key":"AWS_ACCESS_KEY_ID"},"secretAccessKey":{"name":"backup-s3","key":"AWS_SECRET_ACCESS_KEY"}}}}}
JSON
  exit 0
fi
if [[ " $* " == *" apply -f "* ]]; then
  exit 0
fi
exit 1
EOF

cat >"$FAKE/age" <<'EOF'
#!/usr/bin/env bash
# Fake age: records the call and "decrypts" by copying input -> output so the
# downstream gzip integrity check runs on the real fixture bytes.
set -euo pipefail
printf 'age %s\n' "$*" >>"$RESTORE_TEST_CALLS"
out=""; in=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -d) shift ;;
    -i) shift 2 ;;
    -o) out="$2"; shift 2 ;;
    *) in="$1"; shift ;;
  esac
done
/bin/cp "$in" "$out"
EOF

chmod +x "$FAKE/aws" "$FAKE/docker" "$FAKE/kubectl" "$FAKE/age"
export PATH="$FAKE:$PATH"
export RESTORE_TEST_CALLS="$TMP/calls"
export RESTORE_SKIP_DOTENV=1
export TF_STATE_BUCKET=fixture
export TF_STATE_ENDPOINT=https://s3.invalid
export AWS_ACCESS_KEY_ID=fake
export AWS_SECRET_ACCESS_KEY=fake

printf 'synthetic snapshot bytes\n' >"$TMP/snapshot"
gzip -c "$TMP/snapshot" >"$TMP/valid.gz"
export RESTORE_TEST_ARCHIVE="$TMP/valid.gz"

# shellcheck source=scripts/lib/restore.sh
source "$HERE/lib/restore.sh"
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
export TF_STATE_ACCESS_KEY=fake-access
export TF_STATE_SECRET_KEY=fake-secret
restore_load_dotenv "$TMP"
if [ "$AWS_ACCESS_KEY_ID" != fake-access ] || [ "$AWS_SECRET_ACCESS_KEY" != fake-secret ]; then
  fail "TF_STATE credential aliasing with RESTORE_SKIP_DOTENV=1"
fi
ok "environment-only TF_STATE credentials map to AWS CLI names"
latest="$(restore_latest_s3_uri s3://fixture/fixture/ .rdb.gz)"
[ "$latest" = 's3://fixture/fixture/2026-08-01T03:00:00Z.rdb.gz' ] || \
  fail "latest snapshot selection"
ok "latest snapshot is selected lexicographically by RFC3339 key"

# ADR050: latest selection spans plain and .age objects (transition-safe).
age_latest="$(restore_latest_s3_uri s3://fixture/agefix/ .rdb.gz)"
[ "$age_latest" = 's3://fixture/agefix/2026-08-01T03:00:00Z.rdb.gz.age' ] || \
  fail "age snapshot selection: $age_latest"
ok "latest selection picks the newest object across plain and .age suffixes"

# ADR050 §3: a per-store reader credential overrides the shared root key.
export BEX_BACKUP_READER_KEYVALUE_ACCESS_KEY_ID=reader-access
export BEX_BACKUP_READER_KEYVALUE_SECRET_ACCESS_KEY=reader-secret
restore_prefer_reader_credential keyvalue
[ "$AWS_ACCESS_KEY_ID" = reader-access ] && [ "$AWS_SECRET_ACCESS_KEY" = reader-secret ] || \
  fail "per-store reader credential did not override AWS credentials"
restore_prefer_reader_credential bex-db  # no env for bex-db ⇒ leaves reader creds untouched
[ "$AWS_ACCESS_KEY_ID" = reader-access ] || fail "absent reader credential must be a no-op"
ok "per-store reader credential overrides the root key; absent one is a no-op"
unset BEX_BACKUP_READER_KEYVALUE_ACCESS_KEY_ID BEX_BACKUP_READER_KEYVALUE_SECRET_ACCESS_KEY
export AWS_ACCESS_KEY_ID=fake-access AWS_SECRET_ACCESS_KEY=fake-secret

run_dry() {
  : >"$RESTORE_TEST_CALLS"
  output="$TMP/output"
  "$@" >"$output" 2>"$TMP/error" || {
    sed -n '1,80p' "$TMP/error" >&2
    fail "DRY_RUN command failed: $*"
  }
  if grep -Eq '^kubectl .*(apply|create|delete|patch|replace|exec|cp|rollout|port-forward|wait)( |$)' "$RESTORE_TEST_CALLS"; then
    fail "DRY_RUN attempted a Kubernetes mutation: $*"
  fi
  if grep -Eq '^aws .* s3 (rm|mv|sync|put-object|delete-object)( |$)' "$RESTORE_TEST_CALLS"; then
    fail "DRY_RUN attempted an object-store mutation: $*"
  fi
  grep -q 'DRY_RUN=1' "$output" || fail "DRY_RUN declaration missing: $*"
}

run_dry env DRY_RUN=1 "$HERE/restore-etcd.sh"
ok "etcd DRY_RUN restores locally, extracts a CR, and does not mutate Kubernetes/S3"

mkdir "$TMP/reviewed"
cat >"$TMP/reviewed/app.json" <<'JSON'
{"apiVersion":"app.bex.co/v1alpha1","kind":"App","metadata":{"name":"fixture-app","namespace":"default"},"spec":{"image":"example.invalid/fixture"}}
JSON
: >"$RESTORE_TEST_CALLS"
"$HERE/restore-etcd.sh" --apply-dir "$TMP/reviewed" \
  --target-context restore-fixture --confirm APPLY-restore-fixture >/dev/null
[ "$(grep -c '^kubectl .* apply -f ' "$RESTORE_TEST_CALLS")" -eq 1 ] || \
  fail "reviewed etcd apply did not apply exactly one validated file"
ok "etcd apply phase validates and applies only reviewed JSON files"

cat >"$TMP/reviewed/unsafe.json" <<'JSON'
{"apiVersion":"app.bex.co/v1alpha1","kind":"App","metadata":{"name":"unsafe"},"spec":{},"status":{"phase":"Running"}}
JSON
: >"$RESTORE_TEST_CALLS"
if "$HERE/restore-etcd.sh" --apply-dir "$TMP/reviewed" \
  --target-context restore-fixture --confirm APPLY-restore-fixture \
  >"$TMP/output" 2>"$TMP/error"; then
  fail "unsafe reviewed manifest was accepted"
fi
[ ! -s "$RESTORE_TEST_CALLS" ] || fail "unsafe reviewed directory was partially applied"
ok "etcd apply phase validates the full directory before any write"

run_dry env DRY_RUN=1 "$HERE/restore-openbao.sh" \
  --target-namespace restore-bao-test --verify-path tenants/data/fixture
ok "OpenBao DRY_RUN validates its snapshot and does not mutate Kubernetes/S3"

run_dry env DRY_RUN=1 "$HERE/restore-postgres.sh" \
  --source-namespace source --source-cluster source-pg --object-store source-store \
  --server-name source-pg --target-namespace restore-pg-test \
  --database postgres --query 'SELECT 1' --expect 1 \
  --target-time 2026-08-01T03:00:00Z
ok "Postgres DRY_RUN renders plugin PITR intent using read-only discovery"

run_dry env DRY_RUN=1 "$HERE/restore-keyvalue.sh" \
  --id red-fixture --target-namespace restore-kv-test --verify-key fixture --expect value
ok "KeyValue DRY_RUN checksum-validates the RDB and does not mutate Kubernetes/S3"

# ADR050 Tier A: an explicit .age snapshot is age-decrypted before the gzip check.
run_dry env DRY_RUN=1 AGE_BACKUP_PRIVATE_KEY=AGE-SECRET-KEY-1TESTONLY \
  "$HERE/restore-keyvalue.sh" --id red-fixture --target-namespace restore-kv-age \
  --verify-key fixture --expect value \
  --snapshot s3://fixture/keyvalue/red-fixture/2026-08-01T03:00:00Z.rdb.gz.age
grep -q '^age -d ' "$RESTORE_TEST_CALLS" || fail "age decrypt not invoked for an .age snapshot"
ok "encrypted .age snapshot is decrypted before the integrity check"

# Without the private key, an .age snapshot fails closed (no silent plaintext path).
if env DRY_RUN=1 "$HERE/restore-keyvalue.sh" --id red-fixture \
  --target-namespace restore-kv-age --verify-key fixture \
  --snapshot s3://fixture/keyvalue/red-fixture/2026-08-01T03:00:00Z.rdb.gz.age \
  >"$TMP/output" 2>"$TMP/error"; then
  fail "an .age snapshot decrypted without AGE_BACKUP_PRIVATE_KEY"
fi
grep -q 'AGE_BACKUP_PRIVATE_KEY is required' "$TMP/error" || fail "missing-key failure was not specific"
ok "an .age snapshot without the private key fails closed"

printf 'not a gzip stream\n' >"$TMP/corrupt"
export RESTORE_TEST_ARCHIVE="$TMP/corrupt"
if env DRY_RUN=1 "$HERE/restore-keyvalue.sh" \
  --id red-fixture --target-namespace restore-kv-test --verify-key fixture \
  >"$TMP/output" 2>"$TMP/error"; then
  fail "corrupt KeyValue snapshot was accepted"
fi
grep -q 'gzip integrity check failed' "$TMP/error" || fail "corrupt failure was not integrity-specific"
ok "corrupt snapshot fails closed before restore"

export RESTORE_TEST_ARCHIVE="$TMP/valid.gz"
if env DRY_RUN=1 "$HERE/restore-keyvalue.sh" \
  --id red_bad --target-namespace restore-kv-test --verify-key fixture \
  >"$TMP/output" 2>"$TMP/error"; then
  fail "invalid KeyValue id was accepted"
fi
ok "invalid resource parameter fails closed"

if env DRY_RUN=1 "$HERE/restore-postgres.sh" \
  --source-namespace source --source-cluster source-pg --object-store source-store \
  --server-name source-pg --target-namespace production \
  --database postgres --query 'SELECT 1' >"$TMP/output" 2>"$TMP/error"; then
  fail "non-throwaway Postgres target was accepted"
fi
grep -q 'must start with restore-' "$TMP/error" || fail "target safety failure was unclear"
ok "non-throwaway target namespace fails closed"

echo "1..$pass"
