#!/usr/bin/env bash
# Verify the repository's own security settings match the shape w3/m365 set:
#   - secret scanning and push protection are enabled, so a credential in a
#     format GitHub recognizes is refused at push time instead of being found
#     after it is public (the gitleaks job in deploy.yml only runs post-push);
#   - `main` carries active `deletion` and `non_fast_forward` rules, from
#     rulesets with no bypass actor, so it cannot be deleted or force-pushed.
# Both live in repository settings, invisible to every file in this tree, so a
# settings change could silently remove them. A deliberate history rewrite
# disables the ruleset first (docs/runbooks/main-history-rewrite.md), and this
# check is how its re-enable is confirmed.
#
# What a token can read differs. `security_and_analysis` is returned only to
# admins; without it the check falls back to the secret-scanning alerts
# endpoint (200 = enabled, 404 = disabled) and says push protection was not
# read. Rules on `main` are public. Non-provider patterns and validity checks
# need a paid GitHub Secret Protection licence (the org is on the free plan,
# 2026-11-13), so they are not required.
#
# Exit: 0 matches (NOTE lines name anything this token could not read) ·
#       1 drift (one DRIFT line per finding) · 2 unusable (missing tool, a
#       failed or unparseable API answer — never a silent pass).
#
# Env:
#   GITHUB_REPOSITORY                  owner/repo (default: gh's {owner}/{repo}
#                                      from the git remote, no network call)
#   GH_RETRY_ATTEMPTS, GH_RETRY_DELAY  see scripts/lib/gh-retry.sh
#   BEX_REPO_SECURITY_FIXTURE_DIR      read fixtures instead of the API (self-test):
#                                      repo.json, rules-main.json,
#                                      ruleset-<id>.json, and
#                                      secret-scanning-alerts.status (an HTTP code)
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
FIXTURES="${BEX_REPO_SECURITY_FIXTURE_DIR:-}"
repo="${GITHUB_REPOSITORY:-}"
[ -n "$repo" ] || repo='{owner}/{repo}' # gh fills these in from the git remote

unusable() { echo "UNUSABLE: $*" >&2; exit 2; }
command -v jq >/dev/null || unusable "missing required command: jq"
if [ -z "$FIXTURES" ]; then
  command -v gh >/dev/null || unusable "missing required command: gh"
  # shellcheck source=lib/gh-retry.sh
  . "$here/lib/gh-retry.sh"
fi

# fetch <fixture file> <api path> <jq type> — the JSON body, or unusable.
fetch() {
  local out
  if [ -n "$FIXTURES" ]; then
    out="$(cat "$FIXTURES/$1" 2>/dev/null)" || unusable "fixture missing: $1"
  else
    out="$(gh_retry gh api "$2" 2>/dev/null)" || unusable "GitHub API $2 failed"
  fi
  printf '%s' "$out" | jq -e --arg t "$3" 'type == $t' >/dev/null 2>&1 || unusable "$2 did not answer a JSON $3"
  printf '%s' "$out"
}

# alerts_status — 200, 404 or 403 for the secret-scanning alerts list; any
# other answer, after retries, is unusable.
alerts_status() {
  local out attempt attempts="${GH_RETRY_ATTEMPTS:-3}"
  if [ -n "$FIXTURES" ]; then
    out="$(cat "$FIXTURES/secret-scanning-alerts.status" 2>/dev/null)" || unusable "fixture missing: secret-scanning-alerts.status"
    case "$out" in 200 | 404 | 403) echo "$out" ;; *) unusable "secret-scanning alerts answered $out" ;; esac
    return 0
  fi
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if out="$(gh api "repos/$repo/secret-scanning/alerts?per_page=1" 2>&1)"; then
      echo 200
      return 0
    fi
    case "$out" in
      *"HTTP 404"*) echo 404 && return 0 ;;
      *"HTTP 401"* | *"HTTP 403"*) echo 403 && return 0 ;;
    esac
    [ "$attempt" -ge "$attempts" ] || sleep $((attempt * ${GH_RETRY_DELAY:-10}))
  done
  unusable "secret-scanning alerts: $out"
}

drift=0 notes=0
report() { echo "DRIFT: $1"; drift=1; }
note() { echo "NOTE: $1"; notes=1; }

repo_json="$(fetch repo.json "repos/$repo" object)"
settings="$(printf '%s' "$repo_json" | jq -r 'if (.security_and_analysis | type) == "object"
  then .security_and_analysis | "\(.secret_scanning.status // "absent") \(.secret_scanning_push_protection.status // "absent")"
  else "unread" end')"
if [ "$settings" != unread ]; then
  read -r scanning push <<<"$settings"
  [ "$scanning" = enabled ] || report "secret_scanning is $scanning; enable it in Settings → Advanced Security (w3/m365)"
  [ "$push" = enabled ] || report "secret_scanning_push_protection is $push; enable it in Settings → Advanced Security (w3/m365)"
else
  # Assigned first: a `case "$(…)"` would swallow the subshell's unusable exit.
  status="$(alerts_status)"
  case "$status" in
    200) note "secret scanning is enabled; push protection needs an admin token to read and was not checked" ;;
    404) report "secret_scanning is disabled (the alerts endpoint answers 404); enable it and push protection in Settings → Advanced Security (w3/m365)" ;;
    403) note "this token can read neither security_and_analysis nor secret-scanning alerts; secret scanning and push protection were not checked" ;;
  esac
fi

rules="$(fetch rules-main.json "repos/$repo/rules/branches/main" array)"
# One verdict per ruleset carrying either rule, each fetched once:
# "<id> clean", "<id> unread" or "<id> bypass <who>".
verdicts=""
for id in $(printf '%s' "$rules" | jq -r '[.[] | select(.type == "deletion" or .type == "non_fast_forward") | .ruleset_id // empty] | unique | .[]'); do
  verdict="$(fetch "ruleset-$id.json" "repos/$repo/rulesets/$id" object | jq -r '
    if (.bypass_actors | type) != "array" then "unread"
    elif (.bypass_actors | length) == 0 then "clean"
    else "bypass \(.name // .id): " + ([.bypass_actors[] | "\(.actor_type)\(if .actor_id then "#\(.actor_id)" else "" end)"] | join(","))
    end')"
  verdicts="$verdicts$id $verdict"$'\n'
done
for rule in deletion non_fast_forward; do
  ids="$(printf '%s' "$rules" | jq -r --arg r "$rule" '[.[] | select(.type == $r) | .ruleset_id // empty] | unique | .[]')"
  if [ -z "$ids" ]; then
    report "main has no active '$rule' rule; restore the 'main history' ruleset (docs/runbooks/main-history-rewrite.md)"
    continue
  fi
  mine="$(grep -E "^($(printf '%s' "$ids" | paste -sd'|' -)) " <<<"$verdicts" | cut -d' ' -f2-)"
  if grep -qx clean <<<"$mine"; then
    continue # one unbypassable ruleset enforces the rule
  elif grep -qx unread <<<"$mine"; then
    note "main's '$rule' rule is active; this token cannot read its bypass list"
  else
    report "main's '$rule' rule can be bypassed by$(sed 's/^bypass / /' <<<"$mine" | paste -sd';' -); remove the bypass actors"
  fi
done

if [ "$drift" -eq 0 ]; then
  suffix=""
  [ "$notes" = 0 ] || suffix=" (except what the NOTE lines say was not read)"
  echo "OK: secret scanning settings and main's deletion/non-fast-forward rules match w3/m365$suffix"
fi
exit "$drift"
