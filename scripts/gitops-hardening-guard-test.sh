#!/usr/bin/env bash
set -euo pipefail

# w7/m58/t005 — self-test for the platform-pod hardening guard. Runs the SAME
# single-sourced yq expression the CI guard uses (scripts/gitops-validate.sh reads
# lib/platform-pod-hardening-guard.yq) against inline fixtures, proving it:
#   (a) flags a workload with NO securityContext at all (the static-server bug),
#   (b) flags a workload missing exactly ONE baseline field (each field in turn),
#   (c) does NOT flag the egress-meter exemption,
#   (d) passes a fully-hardened workload.
# So a regression that weakens the guard — or a future unhardened platform pod —
# turns this red, not just the one-time manual revert in t003. Run directly or via
# CI alongside gitops-validate.sh.

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
guard="$(cat "$here/lib/platform-pod-hardening-guard.yq")"
command -v yq >/dev/null || { echo "missing required command: yq" >&2; exit 1; }

# flagged prints the names the guard reports non-compliant for the manifest on stdin.
flagged() { yq ea "$guard" - | sed '/^$/d' | sort -u | tr '\n' ' ' | sed 's/ $//'; }

# hardened emits a fully-compliant Deployment named $1.
hardened() {
  cat <<YAML
kind: Deployment
metadata: { name: $1 }
spec:
  template:
    spec:
      securityContext: { runAsNonRoot: true, seccompProfile: { type: RuntimeDefault } }
      containers:
        - name: c
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: { drop: [ALL] }
YAML
}

fail=0
expect() { # $1 = case, $2 = expected flagged, $3 = actual flagged
  if [ "$2" != "$3" ]; then
    echo "FAIL [$1]: guard flagged '$3', expected '$2'" >&2
    fail=1
  else
    echo "ok   [$1]: flagged '$3'"
  fi
}

# (d) fully hardened -> not flagged.
expect "hardened passes" "" "$(hardened good | flagged)"

# (a) no securityContext at all -> flagged.
expect "no securityContext" "bad" "$(cat <<'YAML' | flagged
kind: Deployment
metadata: { name: bad }
spec: { template: { spec: { containers: [ { name: c } ] } } }
YAML
)"

# (b) each single missing/wrong baseline field in turn -> flagged.
expect "missing pod runAsNonRoot" "bad" "$(hardened bad | yq ea 'del(.spec.template.spec.securityContext.runAsNonRoot)' - | flagged)"
expect "wrong seccompProfile"     "bad" "$(hardened bad | yq ea '.spec.template.spec.securityContext.seccompProfile.type = "Unconfined"' - | flagged)"
expect "allowPrivEsc true"        "bad" "$(hardened bad | yq ea '.spec.template.spec.containers[0].securityContext.allowPrivilegeEscalation = true' - | flagged)"
expect "readOnlyRootFilesystem missing" "bad" "$(hardened bad | yq ea 'del(.spec.template.spec.containers[0].securityContext.readOnlyRootFilesystem)' - | flagged)"
expect "drop ALL missing"         "bad" "$(hardened bad | yq ea '.spec.template.spec.containers[0].securityContext.capabilities.drop = ["NET_RAW"]' - | flagged)"

# (c) egress-meter is exempt even when totally unhardened; a hardened sibling in
# the same stream is still checked (the exemption is scoped to one name).
expect "egress-meter exempt, sibling checked" "" "$( { cat <<'YAML'
kind: DaemonSet
metadata: { name: bex-egress-meter }
spec: { template: { spec: { hostNetwork: true, containers: [ { name: c } ] } } }
YAML
hardened good; } | flagged)"

if [ "$fail" -ne 0 ]; then
  echo "platform-pod hardening guard self-test FAILED" >&2
  exit 1
fi
echo "PASS: platform-pod hardening guard self-test"
