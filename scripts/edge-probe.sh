#!/usr/bin/env bash
# Runs one public-edge liveness probe with confirmation retries and names how
# it failed (w1/131). A single ~40s blip at 2026-10-16 21:40Z opened two
# outage issues on a healthy edge, both prescribing a fix the probe output
# contradicted (rotate a valid certificate; a PROXY-protocol mismatch when no
# SSH banner ever arrived). This is the guard: an edge counts as down only
# after every attempt fails, and the issue names the failure the last attempt
# actually saw, so its remedy can fit.
#
# Usage: scripts/edge-probe.sh ssh|tls|shell-ws <target> <out-file>
#   exit 0 = healthy on some attempt; 1 = failed every attempt.
#   <out-file> holds the last attempt's output; <out-file>.class one line:
#   the failure class (see classify_edge_failure) or "ok".
# EDGE_PROBE_ATTEMPTS (default 3) and EDGE_PROBE_PAUSE_SECONDS (default 30)
# bound the confirmation window; EDGE_PROBE_CMD overrides the probe (tests).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"

# classify_edge_failure <kind> <output-file> prints one short class phrase.
# It reads only the probe's own error text, so the class says what was
# observed, never a guess at the cause.
classify_edge_failure() {
  local kind="$1" out="$2"
  case "$kind" in
    ssh)
      if grep -q "read server identification" "$out"; then
        echo "no SSH banner: the connection opened but the server never spoke"
      elif grep -qi "connection refused" "$out"; then
        echo "connection refused"
      elif grep -qE "dial tcp.*(i/o timeout|deadline exceeded)" "$out"; then
        echo "connect timed out"
      else
        echo "banner but no KEXINIT: the handshake never reached authentication"
      fi
      ;;
    tls)
      if grep -q "did not complete a publicly trusted TLS handshake" "$out"; then
        echo "TLS handshake failed or the certificate is not publicly trusted"
      elif grep -q "no parseable leaf certificate" "$out"; then
        echo "no leaf certificate served"
      elif grep -q "does not cover" "$out"; then
        echo "certificate does not cover the host"
      elif grep -q "lacks the exact" "$out"; then
        echo "certificate lacks the *.onbex.co SAN"
      elif grep -q "expires inside" "$out"; then
        echo "certificate expires inside the minimum validity window"
      elif grep -qE "curl: \(28\)" "$out"; then
        echo "HTTP request timed out after the TLS checks passed"
      elif grep -q "returned HTTP" "$out"; then
        echo "unexpected HTTP status"
      else
        echo "probe failed"
      fi
      ;;
    *)
      echo "probe failed"
      ;;
  esac
}

# certificate_failure <class> is true when the class is the certificate's own
# fault, the only case where its cert-manager renewal is the place to look.
certificate_failure() {
  case "$1" in
    "TLS handshake failed"* | "no leaf certificate"* | "certificate "*) return 0 ;;
    *) return 1 ;;
  esac
}

main() {
  local kind="${1:?kind}" target="${2:?target}" out="${3:?out-file}"
  local attempts="${EDGE_PROBE_ATTEMPTS:-3}" pause="${EDGE_PROBE_PAUSE_SECONDS:-30}"
  local -a cmd
  if [ -n "${EDGE_PROBE_CMD:-}" ]; then
    read -r -a cmd <<<"$EDGE_PROBE_CMD"
  else
    case "$kind" in
      ssh) cmd=(bash "$here/ssh-kexinit-probe.sh") ;;
      tls) cmd=(bash "$here/onbex-default-tls-verify.sh") ;;
      shell-ws) cmd=(bash "$here/shell-ws-probe.sh") ;;
      *) echo "edge-probe: unknown kind $kind" >&2; return 2 ;;
    esac
  fi
  local i
  for ((i = 1; i <= attempts; i++)); do
    if "${cmd[@]}" "$target" >"$out" 2>&1; then
      echo "ok" >"$out.class"
      return 0
    fi
    if ((i < attempts)); then
      sleep "$pause"
    fi
  done
  classify_edge_failure "$kind" "$out" >"$out.class"
  echo "failed $attempts of $attempts attempts, ${pause}s apart" >>"$out"
  return 1
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  main "$@"
fi
