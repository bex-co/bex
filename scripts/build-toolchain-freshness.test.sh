#!/usr/bin/env bash
# Self-test for scripts/build-toolchain-freshness.sh. Fixture trees prove
# fail-closed inventory validation, byte-identical resolve, exact old→new
# drift, and issue open/update/close/noop — without contacting a registry
# or GitHub.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/build-toolchain-freshness.sh"
root="$(cd "$here/.." && pwd)"
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable" >&2; exit 1; }

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

DIGEST_A='sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
DIGEST_B='sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
DIGEST_C='sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'

inventory_json() {
  local committed="${1:-$DIGEST_A}"
  cat <<EOF
{
  "schema": "bex.build-toolchain-freshness/v1",
  "images": [
    {
      "id": "cnb-builder",
      "kind": "builder",
      "upstream": "docker.io/example/builder:latest",
      "committed": "example/builder@${committed}",
      "resolved_at": "2026-08-18T00:00:00Z",
      "source": "fixture",
      "files": ["pin.go"]
    }
  ]
}
EOF
}

write_tree() {
  local dir="$1" committed="${2:-$DIGEST_A}"
  mkdir -p "$dir"
  inventory_json "$committed" >"$dir/inv.json"
  printf 'image := "example/builder@%s"\n' "$committed" >"$dir/pin.go"
}

assert() {
  local label="$1" want="$2"
  shift 2
  set +e
  local err
  err="$("$@" 2>&1 >/dev/null)"
  local got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
    echo "$err" >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label (exit $got)"
}

assert_stdout() {
  local label="$1" needle="$2"
  shift 2
  local out
  out="$("$@")"
  if ! printf '%s' "$out" | grep -qF "$needle"; then
    echo "FAIL: $label — stdout did not contain '$needle'" >&2
    echo "$out" >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label"
}

# GREEN: the committed tree validates.
assert "canonical inventory validates" 0 env -u FRESHNESS_INVENTORY -u FRESHNESS_ROOT -u FRESHNESS_PIN_SITES "$SCRIPT" validate

# RED: missing coverage of a pin-site digest.
write_tree "$tmp/uncovered"
printf 'extra := "other@%s"\n' "$DIGEST_C" >>"$tmp/uncovered/pin.go"
assert "uncovered pin-site digest fails" 1 \
  env FRESHNESS_ROOT="$tmp/uncovered" FRESHNESS_INVENTORY="$tmp/uncovered/inv.json" \
      FRESHNESS_PIN_SITES="pin.go" "$SCRIPT" validate

# RED: listed file lacks the committed digest.
write_tree "$tmp/mismatch"
printf 'image := "example/builder@%s"\n' "$DIGEST_B" >"$tmp/mismatch/pin.go"
assert "committed digest missing from file fails" 1 \
  env FRESHNESS_ROOT="$tmp/mismatch" FRESHNESS_INVENTORY="$tmp/mismatch/inv.json" \
      FRESHNESS_PIN_SITES="pin.go" "$SCRIPT" validate

# RED: malformed metadata.
printf '{"schema":"nope","images":[]}\n' >"$tmp/bad.json"
assert "malformed schema fails" 1 \
  env FRESHNESS_INVENTORY="$tmp/bad.json" FRESHNESS_ROOT="$tmp" FRESHNESS_PIN_SITES="" "$SCRIPT" validate

printf '{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"cnb-builder","kind":"builder","upstream":"docker.io/example/builder:latest","committed":"example/builder@sha256:dead","resolved_at":"2026-08-18T00:00:00Z","source":"x","files":["pin.go"]}]}\n' >"$tmp/shortdigest.json"
assert "short digest fails" 1 \
  env FRESHNESS_INVENTORY="$tmp/shortdigest.json" FRESHNESS_ROOT="$tmp" FRESHNESS_PIN_SITES="" "$SCRIPT" validate

# GREEN: resolve against a fixture map is byte-identical across runs.
write_tree "$tmp/stable"
printf '{"docker.io/example/builder:latest":"%s"}\n' "$DIGEST_A" >"$tmp/stable/digests.json"
run_resolve() {
  env FRESHNESS_ROOT="$tmp/stable" FRESHNESS_INVENTORY="$tmp/stable/inv.json" \
      FRESHNESS_PIN_SITES="pin.go" FRESHNESS_DIGESTS="$tmp/stable/digests.json" \
      "$SCRIPT" resolve
}
run_resolve >"$tmp/r1.json"
run_resolve >"$tmp/r2.json"
if ! cmp -s "$tmp/r1.json" "$tmp/r2.json"; then
  echo "FAIL: repeated resolve is not byte-identical" >&2
  fails=$((fails + 1))
else
  echo "ok: repeated resolve is byte-identical"
fi
assert_stdout "unchanged resolve reports changed=false" '"changed": false' cat "$tmp/r1.json"

# Drift: exact old → new replacement, worktree untouched.
printf '{"docker.io/example/builder:latest":"%s"}\n' "$DIGEST_B" >"$tmp/stable/digests.json"
env FRESHNESS_ROOT="$tmp/stable" FRESHNESS_INVENTORY="$tmp/stable/inv.json" \
    FRESHNESS_PIN_SITES="pin.go" FRESHNESS_DIGESTS="$tmp/stable/digests.json" \
    "$SCRIPT" resolve >"$tmp/drift.json"
assert_stdout "drift names committed digest" "$DIGEST_A" cat "$tmp/drift.json"
assert_stdout "drift names observed digest" "$DIGEST_B" cat "$tmp/drift.json"
assert_stdout "drift sets changed=true" '"changed": true' cat "$tmp/drift.json"
if ! grep -q "$DIGEST_A" "$tmp/stable/pin.go"; then
  echo "FAIL: resolve mutated the pin file" >&2
  fails=$((fails + 1))
else
  echo "ok: resolve does not edit the worktree"
fi

body="$(env "$SCRIPT" issue-body "$tmp/drift.json")"
if ! printf '%s' "$body" | grep -qF "$DIGEST_A" || ! printf '%s' "$body" | grep -qF "$DIGEST_B"; then
  echo "FAIL: issue body omitted exact digests" >&2
  fails=$((fails + 1))
else
  echo "ok: issue body carries exact replacements"
fi
if ! printf '%s' "$body" | grep -qF 'never edits, commits, or merges'; then
  echo "FAIL: issue body must say the workflow never applies a pin" >&2
  fails=$((fails + 1))
else
  echo "ok: issue body is a review request"
fi

action() { env EXISTING_ISSUE="${1:-}" "$SCRIPT" issue-action "$2"; }
[ "$(action '' "$tmp/drift.json")" = open ] || { echo "FAIL: new drift should open" >&2; fails=$((fails + 1)); }
[ "$(action '12' "$tmp/drift.json")" = update ] || { echo "FAIL: repeated drift should update" >&2; fails=$((fails + 1)); }
[ "$(action '' "$tmp/r1.json")" = noop ] || { echo "FAIL: no-drift without issue should noop" >&2; fails=$((fails + 1)); }
[ "$(action '12' "$tmp/r1.json")" = close ] || { echo "FAIL: no-drift with issue should close" >&2; fails=$((fails + 1)); }
echo "ok: issue open/update/close/noop decisions"

wf="$root/.github/workflows/build-toolchain-freshness.yml"
if grep -Eq '\$\{\{[[:space:]]*secrets\.|git (commit|push)|gh pr ' "$wf"; then
  echo "FAIL: freshness workflow must not edit the tree or reference production secrets" >&2
  fails=$((fails + 1))
else
  echo "ok: freshness workflow is issue-only and secret-free"
fi
if ! grep -q 'permissions:' "$wf" || ! grep -q 'contents: read' "$wf" || ! grep -q 'issues: write' "$wf"; then
  echo "FAIL: freshness workflow permissions must be contents:read + issues:write" >&2
  fails=$((fails + 1))
else
  echo "ok: freshness workflow permissions are least privilege"
fi

if [ "$fails" -ne 0 ]; then
  echo "FAIL: $fails build-toolchain freshness checks" >&2
  exit 1
fi
echo "PASS: build-toolchain-freshness.sh"
