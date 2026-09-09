#!/usr/bin/env bash
# Red/green self-test for scripts/registry-migration-readiness.sh (w2/m93 t006).
# Hermetic: every case uses READINESS_FIXTURE_DIR; no cluster required.
set -euo pipefail
cd "$(dirname "$0")/.."

guard="$PWD/scripts/registry-migration-readiness.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
failures=0

NOW=1890000000 # fixed epoch far in the future of any real clock skew in CI
DAY=86400

run_case() {
  local dir="$1"
  READINESS_FIXTURE_DIR="$dir" WINDOW_DAYS=14 COVERAGE_GRACE_HOURS=2 bash "$guard" 2>&1 || true
}

expect() {
  local name="$1" want="$2" out
  out="$(run_case "$tmp/$name")"
  if [[ "$out" != *"$want"* ]]; then
    echo "FAIL [$name]: want substring '$want', got:" >&2
    printf '%s\n' "$out" >&2
    failures=$((failures + 1))
  else
    echo "PASS [$name]"
  fi
}

write_pair() {
  local name="$1" inv="$2" ev="$3"
  mkdir -p "$tmp/$name"
  printf '%s\n' "$inv" >"$tmp/$name/inventory.json"
  printf '%s\n' "$ev" >"$tmp/$name/evidence.json"
}

MIGRATED_INV=$(cat <<EOF
{"apps":[
  {"namespace":"tea-aaaaaaaaaaaaaaaaaaaa","name":"tea-aaaaaaaaaaaaaaaaaaaa-web","workspace":"tea-aaaaaaaaaaaaaaaaaaaa","tombstone":"true","image":"zot.example/tea-aaaaaaaaaaaaaaaaaaaa/tea-aaaaaaaaaaaaaaaaaaaa-web:gen-1"},
  {"namespace":"default","name":"hello-static","workspace":"","tombstone":"","image":"zot.example/hello-static:1"}
]}
EOF
)

# --- clean: full 14d coverage, live collection, zero migrated legacy reads ---
write_pair clean "$MIGRATED_INV" "$(cat <<EOF
{"now":$NOW,"coverage_start":$((NOW - 16 * DAY)),"coverage_end":$NOW,
 "required_services":["zot","static-server"],"live_services":["zot","static-server"],
 "gaps":[],"legacy_reads":[{"name":"hello-static","kind":"static","at":$((NOW - DAY))}]}
EOF
)"
expect clean "RESULT status=clean"
# Fixture mode must not fall through into live probes after a clean verdict.
out_clean="$(run_case "$tmp/clean")"
if [[ "$out_clean" == *"kubectl_missing"* ]] || [[ "$out_clean" == *$'\n'"RESULT "* ]]; then
  echo "FAIL [clean]: fixture mode leaked past evaluate:" >&2
  printf '%s\n' "$out_clean" >&2
  failures=$((failures + 1))
else
  echo "PASS [clean_no_fallthrough]"
fi

# --- legacy read on migrated App fails readiness ---
write_pair legacy_read "$MIGRATED_INV" "$(cat <<EOF
{"now":$NOW,"coverage_start":$((NOW - 16 * DAY)),"coverage_end":$NOW,
 "required_services":["zot","static-server"],"live_services":["zot","static-server"],
 "gaps":[],"legacy_reads":[{"name":"tea-aaaaaaaaaaaaaaaaaaaa-web","kind":"registry","at":$((NOW - DAY))}]}
EOF
)"
expect legacy_read "RESULT status=legacy_reads_detected"

# --- truncated history ---
write_pair truncated "$MIGRATED_INV" "$(cat <<EOF
{"now":$NOW,"coverage_start":$((NOW - 5 * DAY)),"coverage_end":$NOW,
 "required_services":["zot","static-server"],"live_services":["zot","static-server"],
 "gaps":[],"legacy_reads":[]}
EOF
)"
expect truncated "RESULT status=insufficient_evidence reason=truncated_window"

# --- collection dark ---
write_pair dark "$MIGRATED_INV" "$(cat <<EOF
{"now":$NOW,"coverage_start":$((NOW - 16 * DAY)),"coverage_end":$NOW,
 "required_services":["zot","static-server"],"live_services":["zot"],
 "gaps":[],"legacy_reads":[]}
EOF
)"
expect dark "RESULT status=insufficient_evidence reason=collection_dark"

# --- coverage gap larger than grace ---
write_pair gap "$MIGRATED_INV" "$(cat <<EOF
{"now":$NOW,"coverage_start":$((NOW - 16 * DAY)),"coverage_end":$NOW,
 "required_services":["zot","static-server"],"live_services":["zot","static-server"],
 "gaps":[{"start":$((NOW - 10 * DAY)),"end":$((NOW - 10 * DAY + 4 * 3600))}],
 "legacy_reads":[]}
EOF
)"
expect gap "RESULT status=insufficient_evidence reason=coverage_gap"

# --- missing coverage start / source error ---
write_pair missing_start "$MIGRATED_INV" "$(cat <<EOF
{"now":$NOW,"coverage_end":$NOW,
 "required_services":["zot","static-server"],"live_services":["zot","static-server"],
 "gaps":[],"legacy_reads":[],"source_error":"missing_coverage_start"}
EOF
)"
expect missing_start "RESULT status=insufficient_evidence reason=source_error"

# --- malformed read ---
write_pair malformed "$MIGRATED_INV" "$(cat <<EOF
{"now":$NOW,"coverage_start":$((NOW - 16 * DAY)),"coverage_end":$NOW,
 "required_services":["zot","static-server"],"live_services":["zot","static-server"],
 "gaps":[],"legacy_reads":[{"name":"tea-aaaaaaaaaaaaaaaaaaaa-web","kind":"registry"}]}
EOF
)"
expect malformed "RESULT status=insufficient_evidence reason=malformed_read"

# --- untombstoned labeled App ---
write_pair untomb "$(cat <<EOF
{"apps":[{"namespace":"tea-aaaaaaaaaaaaaaaaaaaa","name":"tea-aaaaaaaaaaaaaaaaaaaa-web","workspace":"tea-aaaaaaaaaaaaaaaaaaaa","tombstone":"","image":"zot.example/tea-aaaaaaaaaaaaaaaaaaaa/tea-aaaaaaaaaaaaaaaaaaaa-web:gen-1"}]}
EOF
)" "$(cat <<EOF
{"now":$NOW,"coverage_start":$((NOW - 16 * DAY)),"coverage_end":$NOW,
 "required_services":["zot","static-server"],"live_services":["zot","static-server"],
 "gaps":[],"legacy_reads":[]}
EOF
)"
expect untomb "RESULT status=insufficient_evidence reason=untombstoned_labeled"

# --- unscoped image on tombstoned App ---
write_pair unscoped "$(cat <<EOF
{"apps":[{"namespace":"tea-aaaaaaaaaaaaaaaaaaaa","name":"tea-aaaaaaaaaaaaaaaaaaaa-web","workspace":"tea-aaaaaaaaaaaaaaaaaaaa","tombstone":"true","image":"zot.example/tea-aaaaaaaaaaaaaaaaaaaa-web:gen-1"}]}
EOF
)" "$(cat <<EOF
{"now":$NOW,"coverage_start":$((NOW - 16 * DAY)),"coverage_end":$NOW,
 "required_services":["zot","static-server"],"live_services":["zot","static-server"],
 "gaps":[],"legacy_reads":[]}
EOF
)"
expect unscoped "RESULT status=insufficient_evidence reason=unscoped_image"

if [ "$failures" -ne 0 ]; then
  echo "$failures failure(s)" >&2
  exit 1
fi
echo "registry-migration-readiness.test.sh: ok"
