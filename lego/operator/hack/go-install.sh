#!/usr/bin/env bash
set -euo pipefail
[ "$#" -eq 1 ] || { echo 'usage: go-install.sh package@version' >&2; exit 2; }
log=$(mktemp)
trap 'rm -f -- "$log"' EXIT
for attempt in 1 2 3; do
  if go install "$1" 2>"$log"; then
    cat "$log" >&2
    exit 0
  else
    status=$?
  fi
  cat "$log" >&2
  # A truncated module response is retryable; compile, version, checksum and
  # certificate failures must still fail immediately under the same Go policy.
  if [ "$attempt" -eq 3 ] \
    || grep -Eiq 'SECURITY ERROR|checksum mismatch' "$log" \
    || ! grep -Eq '(Get|Head) "https?://[^"]+": .*(unexpected EOF|EOF|i/o timeout|context deadline exceeded|TLS handshake timeout|connection reset by peer|connection refused|network is unreachable|no route to host)|reading https?://[^[:space:]]+: (429|5[0-9][0-9])([[:space:]]|$)' "$log"; then
    exit "$status"
  fi
  echo "go install $1 download attempt $attempt failed; retrying in $((attempt * 2))s..." >&2
  sleep "$((attempt * 2))"
done
