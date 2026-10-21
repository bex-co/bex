#!/usr/bin/env bash
# Self-test for scripts/github-environments-drift.sh (w3/m173 t004): every way
# the live gate can silently open — a missing environment, no branch policy, a
# protected-branches policy, an extra branch, a tag rule — must be reported, and
# the intended shape must pass. Fixtures stand in for the GitHub API.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/github-environments-drift.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
fails=0

printf '%s\n' '# fixture' 'TOKEN_A prod-a,prod-b' 'TOKEN_B prod-b' >"$tmp/table.txt"
good_env='{"deployment_branch_policy":{"protected_branches":false,"custom_branch_policies":true}}'
main_only='{"branch_policies":[{"name":"main","type":"branch"}]}'

# case <label> <want-rc> <needle> ; fixture dir prepared by caller in $dir
case_run() {
  local label="$1" want="$2" needle="$3" out got
  set +e
  out="$(BEX_PRODUCTION_SECRETS_FILE="$tmp/table.txt" BEX_ENV_DRIFT_FIXTURE_DIR="$dir" "$SCRIPT" 2>&1)"
  got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want: $out" >&2; fails=$((fails + 1)); return
  fi
  if [ -n "$needle" ] && ! printf '%s' "$out" | grep -qF "$needle"; then
    echo "FAIL: $label — output did not contain '$needle': $out" >&2; fails=$((fails + 1)); return
  fi
  echo "ok: $label"
}
fresh() {
  dir="$tmp/f$RANDOM$RANDOM"; mkdir -p "$dir"
  for e in prod-a prod-b; do
    printf '%s' "$good_env" >"$dir/$e.json"
    printf '%s' "$main_only" >"$dir/$e.policies.json"
  done
}

fresh; case_run "main-only on every environment passes" 0 "allows deployments from main only"
fresh; rm "$dir/prod-b.json"; case_run "missing environment drifts" 1 "prod-b: environment does not exist"
fresh; printf '%s' '{"deployment_branch_policy":null}' >"$dir/prod-a.json"
case_run "no branch policy drifts" 1 "prod-a: deployment branches are not restricted"
fresh; printf '%s' '{"deployment_branch_policy":{"protected_branches":true,"custom_branch_policies":false}}' >"$dir/prod-a.json"
case_run "protected-branches policy drifts" 1 "any branch can deploy"
fresh; printf '%s' '{"branch_policies":[{"name":"main","type":"branch"},{"name":"release/*","type":"branch"}]}' >"$dir/prod-b.policies.json"
case_run "extra branch drifts" 1 "'main,release/*', want exactly 'main'"
fresh; printf '%s' '{"branch_policies":[{"name":"main","type":"branch"},{"name":"v*","type":"tag"}]}' >"$dir/prod-a.policies.json"
case_run "tag rule drifts" 1 "plus tag policies 'v*'"
fresh; printf '%s' '{"branch_policies":[]}' >"$dir/prod-a.policies.json"
case_run "empty custom policy drifts" 1 "'<none>'"
dir="$tmp/none"; mkdir -p "$dir"
set +e; BEX_PRODUCTION_SECRETS_FILE="$tmp/absent.txt" BEX_ENV_DRIFT_FIXTURE_DIR="$dir" "$SCRIPT" >/dev/null 2>&1; rc=$?; set -e
if [ "$rc" -eq 2 ]; then echo "ok: unreadable table is unusable (exit 2)"; else echo "FAIL: unreadable table exit $rc, want 2" >&2; fails=$((fails + 1)); fi

# The real table must name only production-* environments.
bad="$(sed -e 's/#.*//' "$here/production-secrets.txt" | awk 'NF >= 2 { print $2 }' | tr ',' '\n' | sed '/^$/d' | grep -v '^production-' || true)"
if [ -z "$bad" ]; then echo "ok: production-secrets.txt names only production-* environments"; else echo "FAIL: non-production environments in table: $bad" >&2; fails=$((fails + 1)); fi

[ "$fails" -eq 0 ] && echo "PASS: github-environments-drift.sh" || { echo "FAIL: $fails case(s)" >&2; exit 1; }
