#!/usr/bin/env bash
# Check, but never update, that bex's copies of Cloudflare's published edge
# ranges still equal Cloudflare's list (w4/737).
#
# The edge's client-identity plugin keeps Cloudflare's CF-Connecting-IP,
# CF-IPCountry and CF-Ray only from a peer inside its cloudflareRanges, and
# bex-api's proxied-host warning resolves custom domains against the same list
# in lego/backend/internal/core/proxiedhosts.go. Both were fetched once, on
# 2026-10-02, and scripts/gitops-validate.sh checks only that the two copies
# agree. If Cloudflare adds a range, every request through it to bex.co,
# api.bex.co, dashboard.bex.co or a tenant's Cloudflare-proxied custom domain
# reads the edge's own address as the client. So this reads
# https://www.cloudflare.com/ips-v4 and /ips-v6 every day and fails, naming each
# added or removed range and both files, when either copy differs from them.
#
# To clear it: set cloudflareRanges in both files to Cloudflare's list (order is
# free: both this job and gitops-validate.sh compare sets) and ship them in one
# commit, so the edge and the warning never disagree.
#
# Config (the self-test's seams; never set in CI):
#   CLOUDFLARE_IPS_V4_URL, CLOUDFLARE_IPS_V6_URL      the lists (file:// works)
#   CLOUDFLARE_RANGES_PLUGIN, CLOUDFLARE_RANGES_BACKEND  the two Go files
#
# Exit 0 when both copies equal Cloudflare's list, 1 when either differs, 2
# when a list or a file cannot be read (never a silent pass).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
v4_url="${CLOUDFLARE_IPS_V4_URL:-https://www.cloudflare.com/ips-v4}"
v6_url="${CLOUDFLARE_IPS_V6_URL:-https://www.cloudflare.com/ips-v6}"
plugin="${CLOUDFLARE_RANGES_PLUGIN:-$repo_root/deploy/gitops/charts/traefik-plugins/clientipheaders/clientipheaders.go}"
backend="${CLOUDFLARE_RANGES_BACKEND:-$repo_root/lego/backend/internal/core/proxiedhosts.go}"

unusable() { # unusable <title> <message>
  echo "::error title=$1::$2" >&2
  exit 2
}

work="$(mktemp -d "${TMPDIR:-/tmp}/bex-cloudflare-ranges.XXXXXX")"
trap 'rm -rf "$work"' EXIT

# published <url> <family> <pattern>: Cloudflare's list, one lowercase CIDR per
# line. A body that is not a CIDR list (a challenge page, an error, nothing)
# would otherwise read as "every range removed".
published() {
  local url="$1" family="$2" pattern="$3" body="$work/$2" bad
  if ! curl -fsSL --max-time 30 --retry 3 --retry-delay 5 --retry-all-errors \
    -o "$body" "$url" 2>"$work/$family.err"; then
    unusable "Cloudflare $family list unreachable" "could not fetch $url: $(head -c 300 "$work/$family.err" | tr '\n' ' ')"
  fi
  tr -d '\r' <"$body" | tr 'A-F' 'a-f' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e '/^$/d' >"$body.cidrs"
  if [[ ! -s "$body.cidrs" ]]; then
    unusable "Cloudflare $family list empty" "$url returned no ranges"
  fi
  bad="$(grep -vxE "$pattern" "$body.cidrs" | head -n 1 || true)"
  if [[ -n "$bad" ]]; then
    unusable "Cloudflare $family list unreadable" "$url is not a list of $family CIDRs; first other line: ${bad:0:80}"
  fi
  cat "$body.cidrs"
}

# ours <file>: the CIDR literals of the file's `var cloudflareRanges = ...`
# block, which ends at the first line that is only `}` or `)`.
ours() {
  local file="$1" cidrs
  [[ -r "$file" ]] || unusable "cloudflareRanges file missing" "$file is not readable"
  cidrs="$(awk '/^var cloudflareRanges = /{on=1} on{print} on && /^[)}]$/{exit}' "$file" |
    { grep -oE '"[0-9a-fA-F.:]+/[0-9]+"' || true; } | tr -d '"' | tr 'A-F' 'a-f' | LC_ALL=C sort -u)"
  [[ -n "$cidrs" ]] || unusable "cloudflareRanges unreadable" "no 'var cloudflareRanges = ' block of quoted CIDRs in $file"
  printf '%s\n' "$cidrs"
}

v4="$(published "$v4_url" IPv4 '[0-9]{1,3}(\.[0-9]{1,3}){3}/[0-9]{1,2}')"
v6="$(published "$v6_url" IPv6 '[0-9a-f]{0,4}(:[0-9a-f]{0,4}){2,7}/[0-9]{1,3}')"
printf '%s\n%s\n' "$v4" "$v6" | LC_ALL=C sort -u >"$work/published"
ours "$plugin" >"$work/plugin"
ours "$backend" >"$work/backend"

drift=0
for copy in plugin backend; do
  file="${!copy}"
  added="$(LC_ALL=C comm -23 "$work/published" "$work/$copy")"
  removed="$(LC_ALL=C comm -13 "$work/published" "$work/$copy")"
  [[ -z "$added$removed" ]] && continue
  drift=1
  echo "::error title=Cloudflare ranges drift::cloudflareRanges in ${file#"$repo_root/"} differs from Cloudflare's published list" >&2
  [[ -z "$added" ]] || sed 's/^/  added by Cloudflare, missing here: /' <<<"$added" >&2
  [[ -z "$removed" ]] || sed 's/^/  removed by Cloudflare, still trusted here: /' <<<"$removed" >&2
done

if [[ "$drift" -ne 0 ]]; then
  cat >&2 <<EOF

Action: set cloudflareRanges in both
  ${plugin#"$repo_root/"}
  ${backend#"$repo_root/"}
to Cloudflare's list ($v4_url and $v6_url) and ship them in one commit;
scripts/gitops-validate.sh fails while the two copies differ. A range missing
from the plugin makes a request through it read the edge's address as the
client. This job never edits either file.
EOF
  exit 1
fi
echo "Cloudflare's published ranges ($(wc -l <<<"$v4" | tr -d ' ') IPv4, $(wc -l <<<"$v6" | tr -d ' ') IPv6) equal cloudflareRanges in ${plugin#"$repo_root/"} and ${backend#"$repo_root/"}"
