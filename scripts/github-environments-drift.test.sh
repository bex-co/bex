#!/usr/bin/env bash
# Self-test for scripts/github-environments-drift.sh (w3/m173 t004): every way
# the live gate can silently open — a missing environment, no branch policy, a
# protected-branches policy, an extra branch, a tag rule — and every manual gate
# (required reviewers, a wait timer) must be reported, and the intended shape
# must pass. Fixtures stand in for the GitHub API.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/github-environments-drift.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
fails=0

printf '%s\n' '# fixture' 'TOKEN_A prod-a,prod-b' 'TOKEN_B prod-b' >"$tmp/table.txt"
good_env='{"deployment_branch_policy":{"protected_branches":false,"custom_branch_policies":true}}'
main_only='{"branch_policies":[{"name":"main","type":"branch"}]}'
# with_rules <protection_rules JSON> — good_env plus the given protection rules.
with_rules() { printf '{"protection_rules":%s,%s' "$1" "${good_env#\{}"; }

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
fresh; with_rules '[{"type":"required_reviewers","reviewers":[{"type":"User"}]},{"type":"branch_policy"}]' >"$dir/prod-b.json"
case_run "required reviewers drift" 1 "prod-b: manual protection rule(s) 'required_reviewers'"
fresh; with_rules '[{"type":"wait_timer","wait_timer":5}]' >"$dir/prod-a.json"
case_run "wait timer drifts" 1 "'wait_timer'"
fresh; with_rules '[{"type":"wait_timer","wait_timer":0},{"type":"branch_policy"}]' >"$dir/prod-a.json"
case_run "zero wait timer and branch rule pass" 0 "with no manual gate"
# production-release also carries the release tags its tag-triggered workflows
# push — exactly those, and no other environment may carry any.
printf '%s\n' 'TOKEN_R production-release' >"$tmp/release.txt"
release_tags='{"branch_policies":[{"name":"main","type":"branch"},{"name":"bex-mobile/v*","type":"tag"},{"name":"bex-cli/v*","type":"tag"}]}'
release_run() { # <label> <want-rc> <needle> <policies JSON>
  dir="$tmp/r$RANDOM$RANDOM"; mkdir -p "$dir"
  printf '%s' "$good_env" >"$dir/production-release.json"
  printf '%s' "$4" >"$dir/production-release.policies.json"
  local out got
  set +e; out="$(BEX_PRODUCTION_SECRETS_FILE="$tmp/release.txt" BEX_ENV_DRIFT_FIXTURE_DIR="$dir" "$SCRIPT" 2>&1)"; got=$?; set -e
  if [ "$got" -ne "$2" ] || ! printf '%s' "$out" | grep -qF "$3"; then echo "FAIL: $1 — exit $got, want $2, needle '$3': $out" >&2; fails=$((fails + 1)); return; fi
  echo "ok: $1"
}
release_run "production-release with main + both release tags passes" 0 "allows deployments from main only" "$release_tags"
release_run "production-release without its release tags drifts (releases would be refused)" 1 "want exactly 'main' plus tag policies 'bex-cli/v*,bex-mobile/v*'" "$main_only"
release_run "production-release with a broader tag drifts" 1 "plus tag policies 'bex-cli/v*,v*'" '{"branch_policies":[{"name":"main","type":"branch"},{"name":"bex-cli/v*","type":"tag"},{"name":"v*","type":"tag"}]}'
# The allowed tags must equal the tag triggers of the workflows that deploy to
# production-release, or a new release workflow would be refused (or an old
# pattern left open).
wf_tags="$(grep -l 'environment: production-release' "$here"/../.github/workflows/*.yml \
  | xargs sed -n 's/^ *tags: *\[\(.*\)\].*/\1/p' | tr ',' '\n' | tr -d ' "' | sed '/^$/d' | sort | paste -sd, -)"
script_tags="$(sed -n 's/^ *production-release) echo "\(.*\)" ;;$/\1/p' "$SCRIPT")"
if [ "$wf_tags" = "$script_tags" ]; then echo "ok: release tag rules match the production-release workflows' tag triggers ($wf_tags)"
else echo "FAIL: allowed_tags production-release='$script_tags' but workflows trigger on '$wf_tags'" >&2; fails=$((fails + 1)); fi
dir="$tmp/none"; mkdir -p "$dir"
set +e; BEX_PRODUCTION_SECRETS_FILE="$tmp/absent.txt" BEX_ENV_DRIFT_FIXTURE_DIR="$dir" "$SCRIPT" >/dev/null 2>&1; rc=$?; set -e
if [ "$rc" -eq 2 ]; then echo "ok: unreadable table is unusable (exit 2)"; else echo "FAIL: unreadable table exit $rc, want 2" >&2; fails=$((fails + 1)); fi

# The real table must name only production-* environments.
bad="$(sed -e 's/#.*//' "$here/production-secrets.txt" | awk 'NF >= 2 { print $2 }' | tr ',' '\n' | sed '/^$/d' | grep -v '^production-' || true)"
if [ -z "$bad" ]; then echo "ok: production-secrets.txt names only production-* environments"; else echo "FAIL: non-production environments in table: $bad" >&2; fails=$((fails + 1)); fi

[ "$fails" -eq 0 ] && echo "PASS: github-environments-drift.sh" || { echo "FAIL: $fails case(s)" >&2; exit 1; }
