#!/usr/bin/env bash
# gh_retry — run a gh command up to three times. api.github.com TLS handshake
# timeouts from the runner host are transient, and a probe's issue
# bookkeeping must not turn a healthy run red (isolation-matrix run
# 42019120521). Usage: . scripts/lib/gh-retry.sh; gh_retry gh issue list ...
# GH_RETRY_DELAY (seconds, default 10) scales the backoff; tests set it to 0.
# GH_RETRY_ATTEMPTS (default 3) caps the tries: an advisory reader on a hot
# path (scripts/ship-gates.sh) sets 1, because waiting buys it nothing.
gh_retry() {
  local attempt attempts="${GH_RETRY_ATTEMPTS:-3}"
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    "$@" && return 0
    [ "$attempt" -ge "$attempts" ] || sleep $((attempt * ${GH_RETRY_DELAY:-10}))
  done
  return 1
}
