#!/usr/bin/env bash
set -euo pipefail

# Live pause/resume verification for the w3/m42 snapshot-job-namespace fix
# (docs/ADR042-sandbox-cluster-substrate.md § D5): prove on the real gVisor
# substrate that a sandbox survives a rootfs snapshot round-trip —
#   create → write marker state → pause (pod gone, commit Job ran in the
#   dedicated namespace, snapshot Succeeded) → resume → marker state present
# — with a disposable sandbox that is terminated on exit.
#
# Run AFTER: the patched controller image is deployed (post-/ship Argo
# rollout), scripts/snapshot-registry-secrets.sh has created the push
# credential, and the prod snapshot transport values are enabled (registry +
# snapshotPushSecret + resumePullSecret; see the prod values file comment —
# do NOT run push-only). This script asserts honestly and fails loudly: a
# Pausing hang, a PSS-rejected commit pod, or a resume image-pull failure all
# surface as named failures with the relevant kubectl evidence.
#
# Required:
#   BEX_LIVE_VERIFY=1              explicit authorization for live fixtures
#   BEX_API_URL=https://api.…      bex-api origin
#   BEX_BEARER=…                   bearer authorized for sandbox verbs in the
#                                  target workspace (never echoed)
#   KUBECONFIG=/path/…             app-cluster kubeconfig (read-only ops used)
# Optional:
#   BEX_OWNER_ID=tea-…             explicit workspace id for the sandbox verbs
#                                  (defaults to the bearer's workspace)
#   BEX_SNAPSHOT_JOB_NS=opensandbox-snapshot   dedicated commit-Job namespace
#   BEX_PAUSE_TIMEOUT=300          seconds to wait for pause/resume phases
#
# ADR042 D6: this cannot run against scripts/mock-cluster.sh (cri-dockerd has
# no BatchSandbox snapshot path) — production/k3s substrate only.

cd "$(dirname "$0")/.."

[ "${BEX_LIVE_VERIFY:-0}" = 1 ] || {
  echo "error: set BEX_LIVE_VERIFY=1 to authorize a disposable live sandbox fixture" >&2
  exit 2
}
: "${BEX_API_URL:?set BEX_API_URL to the bex-api origin}"
: "${BEX_BEARER:?set BEX_BEARER to an authorized bearer (not echoed)}"
: "${KUBECONFIG:?set KUBECONFIG to the app-cluster kubeconfig}"

for command in curl jq kubectl; do
  command -v "$command" >/dev/null || { echo "error: missing required command: $command" >&2; exit 2; }
done

snapshot_ns="${BEX_SNAPSHOT_JOB_NS:-opensandbox-snapshot}"
timeout="${BEX_PAUSE_TIMEOUT:-300}"
marker="m42-$(date +%s)-$RANDOM"

api() { # METHOD PATH [JSON_BODY] — bearer never appears in argv (config via stdin)
  local method="$1" path="$2" body="${3:-}" sep='?'
  case "$path" in *\?*) sep='&' ;; esac
  [ -n "${BEX_OWNER_ID:-}" ] && path="$path${sep}ownerId=$BEX_OWNER_ID"
  curl -fsS -X "$method" "$BEX_API_URL$path" \
    --config <(printf 'header = "Authorization: Bearer %s"\n' "$BEX_BEARER") \
    -H 'Content-Type: application/json' ${body:+--data "$body"}
}

fail() { echo "FAIL: $1" >&2; shift; for cmd in "$@"; do echo "--- $cmd" >&2; eval "$cmd" >&2 || true; done; exit 1; }

sandbox_id=""
cleanup() {
  [ -n "$sandbox_id" ] && api POST "/v1/sandboxes/$sandbox_id/terminate" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> create disposable sandbox"
sandbox_id="$(api POST /v1/sandboxes '{}' | jq -re '.id')"
echo "    id=$sandbox_id"

wait_status() { # want-status
  local want="$1" status="" deadline=$((SECONDS + timeout))
  while ((SECONDS < deadline)); do
    status="$(api GET "/v1/sandboxes/$sandbox_id" | jq -re '.status' || true)"
    [ "$status" = "$want" ] && return 0
    sleep 5
  done
  echo "timed out waiting for status=$want (last=$status)" >&2
  return 1
}
wait_status running || fail "sandbox never reached running" \
  "kubectl get pods -A -l batch-sandbox.sandbox.opensandbox.io/name --show-labels | tail -5"

# The marker must live on the container ROOTFS: gVisor mounts /tmp as an
# internal tmpfs, which no rootfs snapshot can capture by design (and the
# sandbox nodes run runsc with overlay2=none so rootfs writes reach the
# committable host rw layer — infra sandbox-pool bootstrap, w3/m42).
echo "==> write marker state inside the sandbox"
api POST "/v1/sandboxes/$sandbox_id/exec" \
  "$(jq -cn --arg m "$marker" '{command: ("echo " + $m + " > /root/m42-marker")}')" >/dev/null

echo "==> pause (rootfs snapshot)"
api POST "/v1/sandboxes/$sandbox_id/pause" >/dev/null
wait_status suspended || fail "pause did not reach suspended — the D5 hang shape" \
  "kubectl get sandboxsnapshots -A -o wide | tail -5" \
  "kubectl get jobs -n '$snapshot_ns' | tail -5" \
  "kubectl get events -n '$snapshot_ns' --sort-by=.lastTimestamp | tail -10"

echo "==> assert the commit Job ran in the dedicated namespace (not a tenant ns)"
kubectl get jobs -n "$snapshot_ns" -l sandbox.opensandbox.io/sandbox-snapshot-name -o name | grep -q . \
  || fail "no commit Job found in $snapshot_ns — snapshot-job-namespace mode not active" \
    "kubectl get jobs -A -l sandbox.opensandbox.io/sandbox-snapshot-name"
kubectl get sandboxsnapshots -A -o json \
  | jq -re '[.items[].status.phase] | index("Succeed") != null' >/dev/null \
  || fail "no SandboxSnapshot reached Succeed" "kubectl get sandboxsnapshots -A -o wide"

echo "==> resume and read the marker back"
api POST "/v1/sandboxes/$sandbox_id/resume" >/dev/null
wait_status running || fail "resume did not reach running — check resume-pull credentials" \
  "kubectl get pods -A -l batch-sandbox.sandbox.opensandbox.io/name -o wide | tail -5" \
  "kubectl get events -A --field-selector reason=Failed --sort-by=.lastTimestamp | tail -10"
readback="$(api POST "/v1/sandboxes/$sandbox_id/exec" '{"command":"cat /root/m42-marker"}' | tr -d '\r')"
grep -qF "$marker" <<<"$readback" \
  || fail "marker not present after resume — rootfs did not survive the round-trip" \
    "printf '%s\n' \"\$readback\" | tail -5"

echo "PASS: pause/resume round-trip preserved rootfs state (marker $marker); commit Job confined to $snapshot_ns"
