#!/usr/bin/env bash
# Push bex CI secrets from .env into THIS repo's GitHub Actions environments.
# scripts/production-secrets.txt is the authority (w3/m173, ADR083 obligation 5,
# .pm/DO_NOT_DO.md #ENV-REVIEWERS): each listed secret is pushed into exactly the
# environments it names and NEVER at repository scope, where any branch's
# workflow could read it. github-actions-validate.sh check 9 reads the same
# table, so the job allowed to read a secret and the environment holding it
# cannot disagree. DRY_RUN=1 prints the plan without calling gh.
#   1. cp .env.example .env     2. fill .env (gitignored)     3. bash scripts/gh-secrets.sh
# Requires the gh CLI, authed (`gh auth login`). Repo is inferred from the origin remote.
# Secrets go in via stdin/file — never as command-line args (which would leak in ps).
set -euo pipefail
cd "$(dirname "$0")/.."

[ -f .env ] || { echo "ERROR: .env not found — run: cp .env.example .env  (then fill it)"; exit 1; }
[ "${DRY_RUN:-0}" = 1 ] || command -v gh >/dev/null || { echo "ERROR: gh CLI not found — brew install gh && gh auth login"; exit 1; }
TABLE="${BEX_PRODUCTION_SECRETS_FILE:-scripts/production-secrets.txt}"
[ -r "$TABLE" ] || { echo "ERROR: production-secret table not readable: $TABLE"; exit 1; }

. scripts/lib/env-file.sh && bex_load_env ./.env || exit 1

set_secret_value() {
  local name="$1" environment="$2"
  if [ "${DRY_RUN:-0}" = 1 ]; then
    cat >/dev/null
  else
    gh secret set "$name" --env "$environment"
  fi
}

set_scalar() {
  local name="$1" environment="$2" val="${!1:-}"
  [ "$#" -eq 2 ] || { echo "ERROR: set_scalar accepts only NAME ENVIRONMENT" >&2; return 2; }
  [ -n "$val" ] || { echo "skip  $name (empty in .env)"; return; }
  printf '%s' "$val" | set_secret_value "$name" "$environment"
  echo "set   $name (environment: $environment)"
}
set_file() {
  local name="$1" path="$2" environment="$3"
  [ "$#" -eq 3 ] || { echo "ERROR: set_file accepts only NAME PATH ENVIRONMENT" >&2; return 2; }
  [ -f "$path" ] || { echo "ERROR: $name file not found: $path"; exit 1; }
  set_secret_value "$name" "$environment" < "$path"
  echo "set   $name  (from $path) (environment: $environment)"
}

# Every production-class secret, into each environment the table names. A
# value comes from $NAME in .env, or from the file at $NAME_FILE (keys, certs,
# multi-line material). REQUIRED names abort when unset; every other name is
# optional and skipped when empty (its consuming step then soft-skips or fails
# loudly by its own design).
REQUIRED=" BEX_ONBEX_TLS_CERT BEX_ONBEX_TLS_KEY BEX_SSH_PUBLIC_KEY BEX_SSH_PRIVATE_KEY "
push_production_secret() {
  local name="$1" envs="$2" file_var="${1}_FILE" env value_file
  value_file="${!file_var:-}"
  if [ -z "$value_file" ] && [ -z "${!name:-}" ]; then
    case "$REQUIRED" in
      *" $name "*) echo "ERROR: $name is required: set $name or ${name}_FILE in .env" >&2; exit 1 ;;
    esac
    echo "skip  $name (empty in .env)"
    return
  fi
  for env in ${envs//,/ }; do
    if [ -n "$value_file" ]; then
      set_file "$name" "$value_file" "$env"
    else
      set_scalar "$name" "$env"
    fi
  done
}
while read -r name envs; do
  push_production_secret "$name" "$envs"
done < <(sed -e 's/#.*//' "$TABLE" | awk 'NF >= 2 { print $1, $2 }')

# Not pushed: BEX_AGENT_SNAPSHOT_S3_* and BEX_DISK_SNAPSHOT_* are installed into
# the cluster from .env by scripts/agent-snapshot-secret.sh and
# scripts/disk-snapshot-secret.sh; no workflow reads them, so a GitHub copy was
# only exposure (w3/m173 t001).

# Repository-scope copies left over from before the move stay readable by any
# branch until deleted. Name them; deleting is a deliberate operator step.
if [ "${DRY_RUN:-0}" = 1 ]; then
  echo "done (DRY_RUN — nothing was pushed)."
else
  leftover="$(comm -12 <(gh secret list --json name --jq '.[].name' | sort) \
    <(sed -e 's/#.*//' "$TABLE" | awk 'NF >= 2 { print $1 }' | sort))"
  if [ -n "$leftover" ]; then
    echo "WARNING: production secrets still at repository scope (readable by any branch):"
    printf '  %s\n' $leftover
    echo "  after verifying the environment copies, delete each: gh secret delete NAME"
  fi
  echo "done."
fi
