#!/usr/bin/env bash
# Unit tests for cloudflare-ranges-drift.sh. Network-free: Cloudflare's two
# lists are files served over file://, and the Go files are the committed
# plugin and backend files, edited here only where a case needs a copy to
# differ, so the script's parsing of the real files is under test too. What is
# checked: an added or removed range fails naming the range and both files, and
# a list or file it cannot read fails with a message instead of passing.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/cloudflare-ranges-drift.sh"
committed_plugin="$repo_root/deploy/gitops/charts/traefik-plugins/clientipheaders/clientipheaders.go"
committed_backend="$repo_root/lego/backend/internal/core/proxiedhosts.go"

failures=0
check() { # check <name> <expected exit> <expected text> <command...>
  local name="$1" expected_exit="$2" expected_text="$3" actual_exit=0 output
  shift 3
  output="$("$@" 2>&1)" || actual_exit=$?
  if [[ "$actual_exit" != "$expected_exit" ]]; then
    echo "FAIL: $name — exit $actual_exit, want $expected_exit" >&2
    echo "$output" | head -12 >&2
    failures=$((failures + 1))
    return
  fi
  if [[ -n "$expected_text" ]] && ! grep -qF -- "$expected_text" <<<"$output"; then
    echo "FAIL: $name — output missing '$expected_text'" >&2
    echo "$output" | head -12 >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}
refute() { # refute <name> <text that must be absent> <command...>
  local name="$1" text="$2" output
  shift 2
  output="$("$@" 2>&1)" || true
  if grep -qF -- "$text" <<<"$output"; then
    echo "FAIL: $name — output contains '$text'" >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}

sandbox="$(mktemp -d -t bex-cloudflare-ranges-test.XXXXXX)"
trap 'rm -rf "$sandbox"' EXIT
mkdir "$sandbox/tmp"

# Cloudflare's lists as the committed plugin holds them, in Cloudflare's
# format: one range per line, the last with no trailing newline (as
# www.cloudflare.com/ips-v4 serves it).
plugin_cidrs="$(grep -oE '"[0-9a-f.:]+/[0-9]+"' "$committed_plugin" | tr -d '"')"
printf '%s' "$(grep -v : <<<"$plugin_cidrs")" >"$sandbox/v4"
printf '%s' "$(grep : <<<"$plugin_cidrs")" >"$sandbox/v6"
if [[ ! -s "$sandbox/v4" || ! -s "$sandbox/v6" || -z "$(tail -c 1 "$sandbox/v4")" ]]; then
  echo "FAIL: the fixture lists could not be built from $committed_plugin" >&2
  exit 1
fi

run_with() { # run_with <v4 file> <v6 file> [plugin] [backend]
  env TMPDIR="$sandbox/tmp" CLOUDFLARE_IPS_V4_URL="file://$1" CLOUDFLARE_IPS_V6_URL="file://$2" \
    CLOUDFLARE_RANGES_PLUGIN="${3:-$committed_plugin}" CLOUDFLARE_RANGES_BACKEND="${4:-$committed_backend}" \
    "$script"
}
plugin_rel="deploy/gitops/charts/traefik-plugins/clientipheaders/clientipheaders.go"
backend_rel="lego/backend/internal/core/proxiedhosts.go"

check "the committed files equal a list holding the same ranges" 0 \
  "Cloudflare's published ranges (15 IPv4, 7 IPv6) equal cloudflareRanges in $plugin_rel and $backend_rel" \
  run_with "$sandbox/v4" "$sandbox/v6"

# Order, case, CRLF and blank lines are not drift.
{
  printf '\r\n'
  sort -r "$sandbox/v4" | sed 's/$/\r/'
} >"$sandbox/v4-shuffled"
tr 'a-f' 'A-F' <"$sandbox/v6" >"$sandbox/v6-upper"
check "reordered, upper-case and CRLF lists pass" 0 "equal cloudflareRanges" \
  run_with "$sandbox/v4-shuffled" "$sandbox/v6-upper"

# Cloudflare adds a range: both copies miss it, and both are named.
{
  cat "$sandbox/v4"
  printf '\n198.51.100.0/24\n'
} >"$sandbox/v4-added"
check "an added IPv4 range fails" 1 "added by Cloudflare, missing here: 198.51.100.0/24" \
  run_with "$sandbox/v4-added" "$sandbox/v6"
check "an added range names the plugin" 1 "::error title=Cloudflare ranges drift::cloudflareRanges in $plugin_rel differs" \
  run_with "$sandbox/v4-added" "$sandbox/v6"
check "an added range names the backend copy" 1 "::error title=Cloudflare ranges drift::cloudflareRanges in $backend_rel differs" \
  run_with "$sandbox/v4-added" "$sandbox/v6"
check "the action names both files" 1 "  $backend_rel" run_with "$sandbox/v4-added" "$sandbox/v6"

# Cloudflare removes a range: both copies still trust it.
grep -v '^2a06:98c0::/29$' "$sandbox/v6" >"$sandbox/v6-removed"
check "a removed IPv6 range fails" 1 "removed by Cloudflare, still trusted here: 2a06:98c0::/29" \
  run_with "$sandbox/v4" "$sandbox/v6-removed"
refute "a removed range is not reported as added" "added by Cloudflare" \
  run_with "$sandbox/v4" "$sandbox/v6-removed"

# Only one copy differs: only it is reported, and the action still names both.
sed 's#"131.0.72.0/22",##' "$committed_backend" >"$sandbox/backend-short.go"
check "a copy missing a range is named" 1 "cloudflareRanges in $sandbox/backend-short.go differs" \
  run_with "$sandbox/v4" "$sandbox/v6" "$committed_plugin" "$sandbox/backend-short.go"
refute "the copy that matches is not reported" "cloudflareRanges in $plugin_rel differs" \
  run_with "$sandbox/v4" "$sandbox/v6" "$committed_plugin" "$sandbox/backend-short.go"

# CIDRs elsewhere in a file are not cloudflareRanges.
{
  cat "$committed_plugin"
  printf '\nvar otherRanges = []string{"192.0.2.0/24"}\n'
} >"$sandbox/plugin-extra.go"
check "a CIDR outside the block is ignored" 0 "equal cloudflareRanges" \
  run_with "$sandbox/v4" "$sandbox/v6" "$sandbox/plugin-extra.go"

# Anything it cannot read fails with exit 2, never a pass.
printf '<!DOCTYPE html>\n<title>Just a moment...</title>\n' >"$sandbox/challenge"
check "a challenge page instead of the list fails" 2 "Cloudflare IPv4 list unreadable" \
  run_with "$sandbox/challenge" "$sandbox/v6"
check "an IPv4 range on the IPv6 list fails" 2 "Cloudflare IPv6 list unreadable" \
  run_with "$sandbox/v4" "$sandbox/v4"
: >"$sandbox/empty"
check "an empty list fails" 2 "Cloudflare IPv6 list empty" run_with "$sandbox/v4" "$sandbox/empty"
check "an unreachable list fails" 2 "Cloudflare IPv4 list unreachable" run_with "$sandbox/absent" "$sandbox/v6"
sed 's/^var cloudflareRanges = /var edgeRanges = /' "$committed_plugin" >"$sandbox/plugin-renamed.go"
check "a renamed block fails" 2 "no 'var cloudflareRanges = ' block" \
  run_with "$sandbox/v4" "$sandbox/v6" "$sandbox/plugin-renamed.go"
check "a missing file fails" 2 "cloudflareRanges file missing" \
  run_with "$sandbox/v4" "$sandbox/v6" "$committed_plugin" "$sandbox/absent.go"

if [[ -n "$(ls -A "$sandbox/tmp")" ]]; then
  echo "FAIL: the script left files in its temporary directory" >&2
  failures=$((failures + 1))
else
  echo "ok: the temporary directory is removed"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "$failures check(s) failed" >&2
  exit 1
fi
echo "all cloudflare-ranges-drift checks passed"
