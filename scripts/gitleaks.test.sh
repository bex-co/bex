#!/usr/bin/env bash
# gitleaks.test.sh — scripts/gitleaks.sh with the real pinned binary and the
# repository's .gitleaks.toml, in a throwaway repository: a credential-shaped
# value in a commit is reported, the shapes .gitleaks.toml allowlists are not,
# and the exit codes separate a finding from a scanner that could not run.
# Every credential-shaped value is generated here at run time, so this file
# itself never trips the scan. Downloads the pinned release once (cached).
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
# head closing the pipe ends tr with SIGPIPE, which pipefail would report.
rand() { LC_ALL=C tr -dc "$1" </dev/urandom | head -c "$2" || true; }
# A key the generic rule must report. gitleaks drops a candidate that contains
# one of its stopwords (case-insensitive) or has no digit. An alphanumeric draw
# hit a stopword about one run in three (the "a generic API key gave exit 0"
# failures of 2026-11-16); a vowel-free one still did about one draw in 200,
# on vpn, md5, xmpp, sync or no digit at all (the "a key beside an allowlisted
# id gave exit 0" failure of 2026-11-17). A consonant alternating with a digit
# always carries a digit, and no stopword alternates letters and digits. At 40
# characters its entropy stays clear of the rule's 3.5 floor (lowest of 1.5M
# simulated draws: 3.7; at 32 it came within 0.07).
rand_key() {
  local letters digits key='' i
  letters=$(rand 'B-DF-HJ-NP-TV-Zb-df-hj-np-tv-z' "$(($1 / 2))")
  digits=$(rand 0-9 "$(($1 / 2))")
  for ((i = 0; i < $1 / 2; i++)); do key+=${letters:i:1}${digits:i:1}; done
  echo "$key"
}

repo="$tmp/repo"
mkdir -p "$repo/scripts"
cp "$root/scripts/gitleaks.sh" "$root/scripts/ci-fetch-verified.sh" "$repo/scripts/"
cp "$root/.gitleaks.toml" "$repo/"
git -C "$repo" init -q -b main
commit() { git -C "$repo" add -A && git -C "$repo" -c user.name=t -c user.email=t@t -c core.hooksPath=/dev/null -c commit.gpgsign=false commit -qm "$1"; }
scan() { (cd "$repo" && bash scripts/gitleaks.sh "$@") >"$tmp/out" 2>&1; }
commit base

# Never credentials: a bex resource id next to a credential word, a
# WebSocket handshake nonce (a header line and a header setter), and a hex
# digest named as a hash.
cat >"$repo/notes.md" <<EOF
A second bad credential, \`rgc-$(rand 0-9a-v 20)\`, was selected.
GET /ws HTTP/1.1\r\nSec-WebSocket-Key: $(rand A-Za-z0-9 22)==\r\n
req.Header.Set("Sec-WebSocket-Key", "$(rand A-Za-z0-9 22)==")
const renderOpenAPISHA256 = "$(rand 0-9a-f 64)"
EOF
commit allowlisted
scan HEAD~1..HEAD || fail "an allowlisted shape was reported: $(cat "$tmp/out")"

# A generic API key and a GitHub token are reported, redacted.
key=$(rand_key 40)
echo "api_key = \"$key\"" >"$repo/config.py"
commit generic
rc=0
scan HEAD~1..HEAD || rc=$?
[ "$rc" = 1 ] || fail "a generic API key gave exit $rc, not 1: $(cat "$tmp/out")"
if grep -qF "$key" "$tmp/out"; then fail "a finding was printed unredacted"; fi
echo "token=ghp_$(rand A-Za-z0-9 36)" >"$repo/ci.env"
commit pat
rc=0
scan HEAD~1..HEAD || rc=$?
[ "$rc" = 1 ] || fail "a GitHub token gave exit $rc, not 1: $(cat "$tmp/out")"

# The allowlisted id shape does not hide another value on the same line.
echo "credential rgc-$(rand 0-9a-v 20) api_key = \"$(rand_key 40)\"" >"$repo/mixed.md"
commit mixed
rc=0
scan HEAD~1..HEAD || rc=$?
[ "$rc" = 1 ] || fail "a key beside an allowlisted id gave exit $rc, not 1: $(cat "$tmp/out")"

# No commits: nothing runs, not even the binary.
BEX_GITLEAKS_BIN=/nonexistent scan HEAD..HEAD || fail "an empty range failed: $(cat "$tmp/out")"

# A range git cannot read is exit 2: gitleaks itself would call it clean.
rc=0
scan 0000000000000000000000000000000000000001..HEAD || rc=$?
[ "$rc" = 2 ] || fail "a bad range gave exit $rc, not 2"

# The config is the one committed at the range's tip: an uncommitted
# allowlist edit does not hide a finding.
cat >>"$repo/.gitleaks.toml" <<'EOF'
[[allowlists]]
description = "everything"
regexes = ['''.*''']
EOF
rc=0
scan HEAD~1..HEAD || rc=$?
[ "$rc" = 1 ] || fail "an uncommitted allowlist hid a finding (exit $rc)"
git -C "$repo" checkout -q -- .gitleaks.toml

# A scanner that fails is exit 2, never a pass and never a finding.
printf '#!/bin/sh\nexit 1\n' >"$tmp/broken"
chmod +x "$tmp/broken"
rc=0
BEX_GITLEAKS_BIN="$tmp/broken" scan HEAD~1..HEAD || rc=$?
[ "$rc" = 2 ] || fail "a failing scanner gave exit $rc, not 2"

echo "gitleaks.test: ok"
