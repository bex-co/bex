#!/usr/bin/env bash
# Verify the live GitHub environment gates match the shape w3/m173 set
# (ADR083 obligation 5 as rewritten 2026-10-20, .pm/DO_NOT_DO.md #ENV-REVIEWERS):
# every environment named in scripts/production-secrets.txt exists and allows
# deployments from `main` only. That branch policy is what keeps a workflow on
# any other branch from reaching an environment-scoped production secret, and it
# lives in repository settings — invisible to github-actions-validate.sh — so a
# settings change could silently remove it. It also reports any manual gate —
# a required-reviewers rule or a wait timer — because production automation
# must never wait on a human.
#
# Secret scope (no production secret left at repository scope) needs an admin
# token to list and is proven by w3/m173 t007's branch probe instead.
#
# Exit: 0 every environment matches · 1 drift (named per environment) ·
#       2 unusable (missing tool, unreadable table, API failure other than 404).
#
# Env:
#   GITHUB_REPOSITORY                owner/repo (default: from `gh repo view`)
#   BEX_PRODUCTION_SECRETS_FILE      table to read (default: scripts/production-secrets.txt)
#   BEX_ENV_DRIFT_FIXTURE_DIR        read <env>.json and <env>.policies.json from
#                                    here instead of the API; a missing <env>.json
#                                    means the environment does not exist (self-test)
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
TABLE="${BEX_PRODUCTION_SECRETS_FILE:-$here/production-secrets.txt}"
FIXTURES="${BEX_ENV_DRIFT_FIXTURE_DIR:-}"

unusable() { echo "UNUSABLE: $*" >&2; exit 2; }
command -v jq >/dev/null || unusable "missing required command: jq"
[ -r "$TABLE" ] || unusable "production-secret table not readable: $TABLE"

environments="$(sed -e 's/#.*//' "$TABLE" | awk 'NF >= 2 { print $2 }' | tr ',' '\n' | sed '/^$/d' | sort -u)"
[ -n "$environments" ] || unusable "no environments declared in $TABLE"

if [ -z "$FIXTURES" ]; then
  command -v gh >/dev/null || unusable "missing required command: gh"
  repo="${GITHUB_REPOSITORY:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"
fi

# fetch <env> <kind: env|policies> — prints JSON, or nothing for a missing environment.
fetch() {
  local env="$1" kind="$2" path out
  if [ -n "$FIXTURES" ]; then
    path="$FIXTURES/$env.json"
    [ "$kind" = env ] || path="$FIXTURES/$env.policies.json"
    [ -f "$path" ] && cat "$path"
    return 0
  fi
  path="repos/$repo/environments/$env"
  [ "$kind" = env ] || path="$path/deployment-branch-policies"
  if out="$(gh api "$path" 2>&1)"; then
    printf '%s' "$out"
  elif printf '%s' "$out" | grep -q 'HTTP 404'; then
    return 0
  else
    unusable "GitHub API $path: $out"
  fi
}

drift=0
report() { echo "DRIFT: $1: $2"; drift=1; }

for env in $environments; do
  meta="$(fetch "$env" env)"
  if [ -z "$meta" ]; then
    report "$env" "environment does not exist (a job referencing it would auto-create it with no branch policy)"
    continue
  fi
  manual="$(printf '%s' "$meta" | jq -r '[.protection_rules[]? | select(.type == "required_reviewers" or (.type == "wait_timer" and (.wait_timer // 0) > 0)) | .type] | join(",")')"
  if [ -n "$manual" ]; then
    report "$env" "manual protection rule(s) '$manual' would hold every run for a human; remove them (#ENV-REVIEWERS)"
  fi
  custom="$(printf '%s' "$meta" | jq -r '.deployment_branch_policy.custom_branch_policies // false')"
  protected="$(printf '%s' "$meta" | jq -r '.deployment_branch_policy.protected_branches // false')"
  if [ "$custom" != true ] || [ "$protected" != false ]; then
    report "$env" "deployment branches are not restricted to a custom policy (custom_branch_policies=$custom, protected_branches=$protected); any branch can deploy"
    continue
  fi
  policies="$(fetch "$env" policies)"
  branches="$(printf '%s' "$policies" | jq -r '[.branch_policies[]? | select((.type // "branch") == "branch") | .name] | sort | join(",")')"
  others="$(printf '%s' "$policies" | jq -r '[.branch_policies[]? | select((.type // "branch") != "branch") | .name] | join(",")')"
  if [ "$branches" != main ] || [ -n "$others" ]; then
    report "$env" "deployment branch policy is '${branches:-<none>}'${others:+ plus tag policies '$others'}, want exactly 'main'"
  fi
done

if [ "$drift" -eq 0 ]; then
  echo "OK: every production environment ($(printf '%s' "$environments" | tr '\n' ' ' | sed 's/ $//')) allows deployments from main only, with no manual gate"
fi
exit "$drift"
