#!/usr/bin/env bash
# Register a customer's custom domain as a Cloudflare for SaaS "custom hostname",
# so the (proxied) Cloudflare edge accepts traffic for it, issues/renews its edge
# certificate, and forwards to bex. Companion to adding the domain to the app's
# render.yaml `domains:` list (then scripts/app-apply.sh) — the operator side needs
# both: CF admits the hostname at the edge, the Ingress rule routes it inside.
#
# One-time zone prereqs (Cloudflare dashboard, once ever):
#   1. SSL/TLS -> Custom Hostnames: enable Cloudflare for SaaS (free <= 100 hostnames).
#   2. Set the fallback origin (a proxied record in the zone the edge forwards to).
#
# The customer's only job: CNAME www.theirdomain.com -> <app>.<base-domain>.
# (Apex domains can't CNAME — they need ALIAS/flattening at their DNS provider,
# or a registrar redirect to www.)
#
# Usage:   CLOUDFLARE_API_TOKEN=... CLOUDFLARE_ZONE_ID=... scripts/domain-add.sh www.example.com
# Status:  same env vars, scripts/domain-add.sh www.example.com --status
# Token scope: Zone -> SSL and Certificates -> Edit (for the base-domain zone).
set -euo pipefail

domain="${1:?usage: domain-add.sh <customer-domain> [--status]}"
: "${CLOUDFLARE_API_TOKEN:?set CLOUDFLARE_API_TOKEN}"
: "${CLOUDFLARE_ZONE_ID:?set CLOUDFLARE_ZONE_ID (zone id of the base domain)}"

api="https://api.cloudflare.com/client/v4/zones/$CLOUDFLARE_ZONE_ID/custom_hostnames"
auth=(-H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" -H "Content-Type: application/json")

if [ "${2:-}" = "--status" ]; then
  curl -fsS "${auth[@]}" "$api?hostname=$domain" |
    yq -p=json -o=yaml '.result[0] | {"hostname": .hostname, "status": .status, "ssl": .ssl.status}'
  exit 0
fi

curl -fsS -X POST "${auth[@]}" "$api" \
  --data "{\"hostname\":\"$domain\",\"ssl\":{\"method\":\"http\",\"type\":\"dv\"}}" |
  yq -p=json -o=yaml '.result | {"hostname": .hostname, "status": .status, "ssl": .ssl.status}'

echo "next: add $domain to the app's render.yaml domains: and run scripts/app-apply.sh" >&2
echo "      (activates once the customer's CNAME resolves; --status to check)" >&2
