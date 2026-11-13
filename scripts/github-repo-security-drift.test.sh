#!/usr/bin/env bash
# Self-test for scripts/github-repo-security-drift.sh (w3/m365 t004): every way
# the repository's push-time secret refusal or main's history protection can
# silently turn off must be reported — secret scanning or push protection
# disabled, a missing deletion or non-fast-forward rule, a bypass actor — the
# intended shape must pass, and what a non-admin token cannot read must be said,
# never passed off as checked. Fixtures stand in for the GitHub API.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/github-repo-security-drift.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

admin_repo() { # admin_repo <secret_scanning> <push_protection>
  printf '{"full_name":"o/r","security_and_analysis":{"secret_scanning":{"status":"%s"},"secret_scanning_push_protection":{"status":"%s"},"secret_scanning_non_provider_patterns":{"status":"disabled"},"secret_scanning_validity_checks":{"status":"disabled"}}}' "$1" "$2"
}
both_rules='[{"type":"deletion","ruleset_id":7},{"type":"non_fast_forward","ruleset_id":7}]'
no_bypass='{"id":7,"name":"main history","bypass_actors":[]}'

fresh() { # a fixture dir holding the intended shape, read with an admin token
  dir="$(mktemp -d "$tmp/f.XXXXXX")"
  admin_repo enabled enabled >"$dir/repo.json"
  printf '%s' "$both_rules" >"$dir/rules-main.json"
  printf '%s' "$no_bypass" >"$dir/ruleset-7.json"
}

# case_run <label> <want-rc> <needle> [<absent needle>] — runs against $dir.
case_run() {
  local label="$1" want="$2" needle="$3" absent="${4:-}" out got
  set +e
  out="$(BEX_REPO_SECURITY_FIXTURE_DIR="$dir" "$SCRIPT" 2>&1)"
  got=$?
  set -e
  bad() { echo "FAIL: $label — $1: $out" >&2; fails=$((fails + 1)); }
  if [ "$got" -ne "$want" ]; then
    bad "exit $got, want $want"
  elif [ -n "$needle" ] && ! printf '%s' "$out" | grep -qF "$needle"; then
    bad "output did not contain '$needle'"
  elif [ -n "$absent" ] && printf '%s' "$out" | grep -qF "$absent"; then
    bad "output contained '$absent'"
  else
    echo "ok: $label"
  fi
}

fresh
case_run "intended shape passes" 0 "OK:" "DRIFT"

fresh
admin_repo disabled enabled >"$dir/repo.json"
case_run "secret scanning disabled" 1 "DRIFT: secret_scanning is disabled"

fresh
admin_repo enabled disabled >"$dir/repo.json"
case_run "push protection disabled" 1 "DRIFT: secret_scanning_push_protection is disabled"

fresh
printf '{"full_name":"o/r","security_and_analysis":{}}' >"$dir/repo.json"
case_run "settings missing from an admin read" 1 "DRIFT: secret_scanning_push_protection is absent"

fresh
printf '%s' '[{"type":"non_fast_forward","ruleset_id":7}]' >"$dir/rules-main.json"
case_run "deletion rule missing" 1 "DRIFT: main has no active 'deletion' rule"

fresh
printf '%s' '[{"type":"deletion","ruleset_id":7}]' >"$dir/rules-main.json"
case_run "non-fast-forward rule missing" 1 "DRIFT: main has no active 'non_fast_forward' rule"

fresh
printf '[]' >"$dir/rules-main.json"
case_run "ruleset removed or disabled" 1 "'non_fast_forward' rule"

fresh
printf '%s' '{"id":7,"name":"main history","bypass_actors":[{"actor_type":"RepositoryRole","actor_id":5,"bypass_mode":"always"}]}' >"$dir/ruleset-7.json"
case_run "bypass actor added" 1 "can be bypassed by main history: RepositoryRole#5"

fresh
printf '%s' '[{"type":"deletion","ruleset_id":7},{"type":"deletion","ruleset_id":8},{"type":"non_fast_forward","ruleset_id":7}]' >"$dir/rules-main.json"
printf '%s' '{"id":8,"name":"other","bypass_actors":[{"actor_type":"OrganizationAdmin"}]}' >"$dir/ruleset-8.json"
case_run "a second, bypassable ruleset does not hide an unbypassed one" 0 "OK:" "DRIFT"

fresh
printf '{"full_name":"o/r"}' >"$dir/repo.json"
printf '200' >"$dir/secret-scanning-alerts.status"
case_run "non-admin token: alerts readable" 0 "push protection needs an admin token" "DRIFT"

fresh
printf '{"full_name":"o/r"}' >"$dir/repo.json"
printf '404' >"$dir/secret-scanning-alerts.status"
case_run "non-admin token: secret scanning disabled" 1 "DRIFT: secret_scanning is disabled (the alerts endpoint answers 404)"

fresh
printf '{"full_name":"o/r"}' >"$dir/repo.json"
printf '403' >"$dir/secret-scanning-alerts.status"
case_run "non-admin token: nothing readable is said, not passed off" 0 "were not checked"

fresh
printf '%s' '{"id":7,"name":"main history"}' >"$dir/ruleset-7.json"
case_run "bypass list unreadable is a note" 0 "except what the NOTE lines say" "DRIFT"

fresh
rm "$dir/rules-main.json"
case_run "missing read is unusable, never a pass" 2 "UNUSABLE"

fresh
printf '{"full_name":"o/r"}' >"$dir/repo.json"
printf '500' >"$dir/secret-scanning-alerts.status"
case_run "non-admin token: a failed alerts read is unusable, never a pass" 2 "UNUSABLE"

fresh
printf '{"full_name":"o/r"}' >"$dir/repo.json"
case_run "non-admin token: a missing alerts answer is unusable" 2 "UNUSABLE"

fresh
printf '%s' '{"id":7,"name":"main history","bypass_actors":null}' >"$dir/ruleset-7.json"
case_run "a null bypass list is unread, not clean" 0 "cannot read its bypass list"

fresh
printf 'not json' >"$dir/ruleset-7.json"
case_run "a garbage ruleset read is unusable" 2 "UNUSABLE"

fresh
printf '%s' '[{"type":"deletion","ruleset_id":7},{"type":"non_fast_forward","ruleset_id":7}]' >"$dir/rules-main.json"
printf '%s' '{"id":7,"name":"main history","bypass_actors":[{"actor_type":"DeployKey","bypass_mode":"always"}]}' >"$dir/ruleset-7.json"
case_run "a bypass on the only ruleset is drift for both rules" 1 "'non_fast_forward' rule can be bypassed by main history: DeployKey"

fresh
printf 'not json' >"$dir/rules-main.json"
case_run "garbage read is unusable" 2 "UNUSABLE"

if [ "$fails" -gt 0 ]; then
  echo "$fails case(s) failed" >&2
  exit 1
fi
echo "all github-repo-security-drift cases passed"
