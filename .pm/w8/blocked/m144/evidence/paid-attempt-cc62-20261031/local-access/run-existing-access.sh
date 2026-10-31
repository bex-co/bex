#!/usr/bin/env bash
set -euo pipefail
umask 077
cd /Users/tianpan/projects/bex7
work=/private/tmp/bex-w8-m144-live-20261031-cc62f8e3a1ee8b8c/local-existing-access
. scripts/lib/env-file.sh
bex_load_env ./.env
printf 'existing-env-loaded\n' > "$work/phase.txt"
[ -n "${HCLOUD_TOKEN:-}" ] || { printf 'HCLOUD_TOKEN absent\n' >&2; exit 21; }
[ -r "$HOME/.ssh/bex" ] || { printf 'existing SSH key absent\n' >&2; exit 22; }
export BEX_SSH_KEY_PATH="$HOME/.ssh/bex"
if [ -n "${BEX_SSH_KNOWN_HOSTS_FILE:-}" ]; then
 [ -s "$BEX_SSH_KNOWN_HOSTS_FILE" ] || { printf 'configured authoritative host pin unavailable\n' >&2; exit 23; }
elif [ -n "${BEX_SSH_KNOWN_HOSTS:-}" ]; then
 printf '%s\n' "$BEX_SSH_KNOWN_HOSTS" > "$work/known_hosts"
 chmod 600 "$work/known_hosts"
 export BEX_SSH_KNOWN_HOSTS_FILE="$work/known_hosts"
else
 printf 'authoritative host pin absent; refusing TOFU\n' >&2; exit 24
fi
export BEX_SSH_REQUIRE_KNOWN_HOSTS=true
export BEX_KUBECONFIG_FETCH_ATTEMPTS=1
printf 'existing-credential-material-present-strict-pinned\n' > "$work/phase.txt"
bash scripts/fetch-app-kubeconfig.sh "$work/app.kubeconfig" > "$work/fetch.stdout.txt" 2> "$work/fetch.stderr.txt"
chmod 600 "$work/app.kubeconfig"
printf 'fetched-private-kubeconfig\n' > "$work/phase.txt"
export KUBECONFIG="$work/app.kubeconfig"
bash scripts/service-disk-inventory.sh > "$work/inventory.json" 2> "$work/inventory.stderr.txt"
printf 'fixed-readonly-inventory-complete\n' > "$work/phase.txt"
