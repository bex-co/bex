#!/usr/bin/env bash
# Reconcile public, DNS-only A/AAAA records with the public addresses on
# Traefik's LoadBalancer. The BEX_SSH_* interface remains the SSH entrypoint;
# BEX_EDGE_DNS_* lets other raw-TCP edges reuse the same safety gates.
#
# Usage:
#   BEX_SSH_HOST=ssh.bex.co CLOUDFLARE_API_TOKEN=... \
#     scripts/ssh-dns-cloudflare.sh [--check]
#
# The token needs Zone / DNS / Edit for the one DNS zone. If neither
# CLOUDFLARE_ZONE_ID nor the legacy CF_ZONE_ID is set, it also needs Zone / Zone
# / Read so the script can discover the zone id from BEX_SSH_DNS_ZONE.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/ssh-edge.sh
source "$repo_root/scripts/lib/ssh-edge.sh"

mode="${1:-}"
if [[ -n "$mode" && "$mode" != "--check" ]]; then
  echo "usage: $0 [--check]" >&2
  exit 2
fi

: "${CLOUDFLARE_API_TOKEN:?set a zone-scoped Cloudflare API token}"

for command in curl jq; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done

edge_label="${BEX_EDGE_DNS_LABEL:-SSH}"
hostname="$(printf '%s' "${BEX_EDGE_DNS_HOST:-${BEX_SSH_HOST:-}}" | tr '[:upper:]' '[:lower:]')"
[[ -n "$hostname" ]] || { echo "set BEX_SSH_HOST or BEX_EDGE_DNS_HOST" >&2; exit 1; }
validation_host="$hostname"
if [[ "$hostname" == \*.* ]]; then
  validation_host="${hostname:2}"
fi
zone_name="$(printf '%s' "${BEX_EDGE_DNS_ZONE:-${BEX_SSH_DNS_ZONE:-${validation_host#*.}}}" | tr '[:upper:]' '[:lower:]')"
zone_id="${CLOUDFLARE_ZONE_ID:-${CF_ZONE_ID:-}}"

hostname_pattern='^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]*[a-z0-9])?$'
if [[ ! "$validation_host" =~ $hostname_pattern || ! "$zone_name" =~ $hostname_pattern ]]; then
  echo "edge host and DNS zone must be DNS hostnames (an initial *. wildcard is allowed)" >&2
  exit 1
fi
if [[ "$validation_host" != "$zone_name" && "$validation_host" != *".$zone_name" ]]; then
  echo "edge host must belong to its DNS zone" >&2
  exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
desired="$tmp/desired.tsv"
records="$tmp/records.tsv"
used="$tmp/used.ids"
: >"$used"

if ! bex_ssh_public_ingress_addresses |
  awk '
    /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/ { print "A\t" $0; next }
    /:/ { print "AAAA\t" $0; next }
    { exit 1 }
  ' >"$desired"; then
  echo "the Terraform edge reported an invalid public LoadBalancer address" >&2
  exit 1
fi
if [[ ! -s "$desired" ]]; then
  echo "the Terraform edge has no public LoadBalancer addresses" >&2
  exit 1
fi

api_base="https://api.cloudflare.com/client/v4"
cloudflare_request() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"
  local response
  local args=(
    --silent
    --show-error
    --fail-with-body
    --request "$method"
    --header "Authorization: Bearer $CLOUDFLARE_API_TOKEN"
    --header 'Content-Type: application/json'
  )
  if [[ -n "$payload" ]]; then
    args+=(--data "$payload")
  fi

  if ! response="$(curl "${args[@]}" "$api_base$path")"; then
    echo "Cloudflare API request failed: $method $path" >&2
    return 1
  fi
  if ! jq -e '.success == true' >/dev/null 2>&1 <<<"$response"; then
    local errors
    errors="$(jq -r '[.errors[]? | ((.code | tostring) + ": " + .message)] | join("; ")' <<<"$response" 2>/dev/null || true)"
    echo "Cloudflare API rejected $method $path${errors:+: $errors}" >&2
    return 1
  fi
  printf '%s\n' "$response"
}

if [[ -z "$zone_id" ]]; then
  zone_response="$(cloudflare_request GET "/zones?name=$zone_name&status=active&per_page=50")"
  if [[ "$(jq '.result | length' <<<"$zone_response")" != "1" ]]; then
    echo "Cloudflare zone discovery did not return exactly one active zone for $zone_name" >&2
    exit 1
  fi
  zone_id="$(jq -r '.result[0].id' <<<"$zone_response")"
fi
if [[ ! "$zone_id" =~ ^[A-Za-z0-9_-]+$ ]]; then
  echo "invalid Cloudflare zone id" >&2
  exit 1
fi

records_path="/zones/$zone_id/dns_records?name.exact=$hostname&per_page=100"
load_records() {
  local response
  response="$(cloudflare_request GET "$records_path")"
  if jq -e '.result[]? | select(.type == "CNAME" or .type == "NS")' >/dev/null <<<"$response"; then
    echo "refusing to replace a CNAME or NS record at $hostname" >&2
    return 1
  fi
  jq -r '.result[]? | select(.type == "A" or .type == "AAAA") | [.id, .type, .content, ((.proxied // false) | tostring)] | @tsv' \
    <<<"$response" >"$records"
}

normalized_records() {
  awk -F '\t' '{ print $2 "\t" $3 "\t" $4 }' "$records" | sort
}

normalized_desired() {
  awk -F '\t' '{ print $1 "\t" $2 "\tfalse" }' "$desired" | sort
}

records_match() {
  [[ "$(normalized_records)" == "$(normalized_desired)" ]]
}

print_mismatch() {
  echo "Cloudflare A/AAAA records for $hostname do not match the Terraform edge Load Balancer addresses" >&2
  echo "desired (DNS-only):" >&2
  normalized_desired | sed 's/^/  /' >&2
  echo "current:" >&2
  if [[ -s "$records" ]]; then
    normalized_records | sed 's/^/  /' >&2
  else
    echo "  <none>" >&2
  fi
}

load_records
addresses="$(cut -f2 "$desired" | paste -sd, -)"
if [[ "$mode" == "--check" ]]; then
  if ! records_match; then
    print_mismatch
    exit 1
  fi
  echo "PASS Cloudflare $edge_label DNS host=$hostname addresses=$addresses"
  exit 0
fi

while IFS=$'\t' read -r type content; do
  exact_id=""
  fallback_id=""
  existing_content=""
  existing_proxied=""

  while IFS=$'\t' read -r record_id record_type record_content record_proxied; do
    [[ -n "$record_id" ]] || continue
    if grep -Fxq "$record_id" "$used"; then
      continue
    fi
    if [[ "$record_type" == "$type" && "$record_content" == "$content" ]]; then
      exact_id="$record_id"
      existing_content="$record_content"
      existing_proxied="$record_proxied"
      break
    fi
    if [[ "$record_type" == "$type" && -z "$fallback_id" ]]; then
      fallback_id="$record_id"
    fi
  done <"$records"

  record_id="${exact_id:-$fallback_id}"
  comment_label="$(printf '%s' "$edge_label" | tr '[:upper:]' '[:lower:]')"
  payload="$(jq -cn \
    --arg type "$type" \
    --arg name "$hostname" \
    --arg content "$content" \
    --arg comment "bex $comment_label raw TCP edge" \
    '{type: $type, name: $name, content: $content, ttl: 300, proxied: false, comment: $comment}')"

  if [[ -n "$record_id" ]]; then
    if [[ ! "$record_id" =~ ^[A-Za-z0-9_-]+$ ]]; then
      echo "Cloudflare returned an invalid DNS record id" >&2
      exit 1
    fi
    printf '%s\n' "$record_id" >>"$used"
    if [[ "$record_id" != "$exact_id" || "$existing_proxied" != "false" || "$existing_content" != "$content" ]]; then
      cloudflare_request PATCH "/zones/$zone_id/dns_records/$record_id" "$payload" >/dev/null
    fi
  else
    cloudflare_request POST "/zones/$zone_id/dns_records" "$payload" >/dev/null
  fi
done <"$desired"

while IFS=$'\t' read -r record_id _; do
  [[ -n "$record_id" ]] || continue
  if ! grep -Fxq "$record_id" "$used"; then
    if [[ ! "$record_id" =~ ^[A-Za-z0-9_-]+$ ]]; then
      echo "Cloudflare returned an invalid DNS record id" >&2
      exit 1
    fi
    cloudflare_request DELETE "/zones/$zone_id/dns_records/$record_id" >/dev/null
  fi
done <"$records"

load_records
if ! records_match; then
  print_mismatch
  exit 1
fi
echo "reconciled Cloudflare $edge_label DNS host=$hostname addresses=$addresses"
