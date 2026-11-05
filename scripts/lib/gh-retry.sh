#!/usr/bin/env bash
# gh_retry — run a gh command up to three times. api.github.com TLS handshake
# timeouts from the runner host are transient, and a probe's issue
# bookkeeping must not turn a healthy run red (isolation-matrix run
# 42019120521). Usage: . scripts/lib/gh-retry.sh; gh_retry gh issue list ...
gh_retry() {
  local attempt
  for attempt in 1 2 3; do
    "$@" && return 0
    [ "$attempt" = 3 ] || sleep $((attempt * 10))
  done
  return 1
}
