#!/usr/bin/env bash
# Acceptance for platform alerting (docs/ADR010-observability.md, w3/m6): prove the
# fire → notify → resolve loop end-to-end against the current kubeconfig cluster,
# WITHOUT waiting for a real outage. Two independent proofs, layered cheapest-first:
#
#   1. RULE LOGIC (offline, deterministic): `promtool test rules` over the pack
#      embedded in deploy/gitops/base/prometheus.yaml — the same check CI runs via
#      scripts/gitops-validate.sh. This is the synthetic-break proof at the rule
#      level: the bad-rollout not-ready series fires PlatformDeploymentNotReady,
#      the stale-timestamp fixture fires BackupCronJobStale, both clear on fix.
#
#   2. NOTIFICATION PATH (live): stand up a throwaway Alertmanager from the rendered
#      chart config into a scratch namespace, with the prod email receiver swapped
#      for a webhook pointed at an in-cluster capture sink (so no real mail is sent
#      and no SMTP secret is needed), hand-fire an alert via the Alertmanager v2
#      API, and observe the sink receive first the firing then — after resolving
#      the alert — the resolved notification. This exercises the route + group/
#      resolve mechanics the GitOps Alertmanager uses (the receiver transport is
#      the one deliberate test-only substitution).
#
# The two together cover the DoD: a broken invariant produces a channel
# notification within an evaluation window, and recovery resolves it. Full Argo
# firing against live cluster series is the post-ship smoke (see the milestone
# README); this script is the pre-ship gate that needs no committed secret.
#
# Usage: scripts/alerts-verify.sh            # respects $KUBECONFIG; exits 0 on pass
# Requires: kubectl, helm, curl, jq, yq, promtool; a reachable cluster.
set -euo pipefail
cd "$(dirname "$0")/.."

NS=alerts-verify
REL=amverify
PROM_CHART_VERSION="25.30.2" # the minor pinned in prometheus.yaml (targetRevision 25.*)
CAPTURE=capture
PF_PID=""
TMP="$(mktemp -d)"

cleanup() {
  [ -n "$PF_PID" ] && kill "$PF_PID" 2>/dev/null || true
  kubectl delete ns "$NS" --wait=false >/dev/null 2>&1 || true
  rm -rf "$TMP"
}
trap cleanup EXIT

say() { printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
fail() { printf '\033[1;31mFAIL: %s\033[0m\n' "$*" >&2; exit 1; }

# ── 1. Rule logic (offline) ──────────────────────────────────────────────────
say "1. promtool: rule pack checks + unit tests"
if command -v promtool >/dev/null 2>&1; then
  # Same single-yq extraction gitops-validate.sh runs (from_yaml re-parses the
  # helm block-scalar string in-process); keep the two in sync if the path moves.
  yq '.spec.source.helm.values | from_yaml | {"groups": .serverFiles."alerting_rules.yml".groups}' \
    deploy/gitops/base/prometheus.yaml >"$TMP/alerting_rules.yml"
  cp deploy/gitops/base/rules/alerts_test.yml "$TMP/alerts_test.yml"
  ( cd "$TMP" && promtool check rules alerting_rules.yml && promtool test rules alerts_test.yml ) \
    || fail "rule pack failed promtool"
else
  fail "promtool not installed — install it (see .github/workflows/gitops.yml) or run scripts/gitops-validate.sh"
fi

# ── 2. Notification path (live) ──────────────────────────────────────────────
say "2. live Alertmanager: fire → notify, resolve → resolved"
# A prior run's namespace may still be Terminating — wait it out before recreating.
kubectl delete ns "$NS" --ignore-not-found --wait=true --timeout=120s >/dev/null 2>&1 || true
kubectl create ns "$NS" >/dev/null

# The capture sink: logs every webhook POST body to stdout. A tiny python http
# server keeps this image-free of anything exotic.
kubectl -n "$NS" apply -f - >/dev/null <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: capture-src
data:
  sink.py: |
    import http.server, sys
    class H(http.server.BaseHTTPRequestHandler):
        def do_POST(self):
            n = int(self.headers.get('Content-Length', 0))
            body = self.rfile.read(n).decode()
            sys.stdout.write("WEBHOOK " + body + "\n"); sys.stdout.flush()
            self.send_response(200); self.end_headers()
        def log_message(self, *a): pass
    http.server.HTTPServer(('', 9099), H).serve_forever()
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: capture
  labels: { app: capture }
spec:
  replicas: 1
  selector: { matchLabels: { app: capture } }
  template:
    metadata: { labels: { app: capture } }
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile: { type: RuntimeDefault }
      containers:
        - name: sink
          image: python:3.12-alpine
          command: ["python", "/src/sink.py"]
          ports: [{ containerPort: 9099 }]
          securityContext:
            allowPrivilegeEscalation: false
            capabilities: { drop: ["ALL"] }
          volumeMounts: [{ name: src, mountPath: /src }]
      volumes:
        - name: src
          configMap: { name: capture-src }
---
apiVersion: v1
kind: Service
metadata:
  name: capture
spec:
  selector: { app: capture }
  ports: [{ port: 9099, targetPort: 9099 }]
YAML

# Render JUST the alertmanager subchart with the SAME config prometheus.yaml uses,
# then install it into the scratch namespace (release name avoids the GitOps one).
# The prometheus.yaml values live nested under spec.source.helm.values — extract
# them so helm sees a plain values file, then disable everything but Alertmanager.
say "rendering + installing throwaway Alertmanager"
yq '.spec.source.helm.values' deploy/gitops/base/prometheus.yaml >"$TMP/values.yaml"
# For the test we (a) disable everything but Alertmanager, (b) shorten ONLY this
# throwaway AM's route timers so the resolved notification (the real config paces
# it at group_interval: 5m) lands within the wait window, and (c) SWAP the prod
# email receiver for a webhook pointed at the in-cluster capture sink — so the
# test observes fire/resolve directly, sends no real mail, and needs no SMTP
# secret (extraSecretMounts dropped). The committed config is untouched — we edit
# the extracted copy. The "null" black-hole receiver is preserved alongside the
# swapped webhook because the committed route sends severity=info there; dropping
# it would leave that sub-route referencing an undefined receiver and Alertmanager
# would reject the config on load.
CAPTURE_URL="http://${CAPTURE}.${NS}.svc:9099/" \
yq -i '
  .server.enabled=false
  | (.["kube-state-metrics"].enabled)=false
  | .alertmanager.extraSecretMounts=[]
  | .alertmanager.config.route.group_wait="5s"
  | .alertmanager.config.route.group_interval="10s"
  | .alertmanager.config.route.repeat_interval="30s"
  | .alertmanager.config.receivers=[{"name":"platform","webhook_configs":[{"url":env(CAPTURE_URL),"send_resolved":true}]},{"name":"null"}]
' "$TMP/values.yaml"
helm template "$REL" prometheus \
  --repo https://prometheus-community.github.io/helm-charts \
  --version "$PROM_CHART_VERSION" -n "$NS" -f "$TMP/values.yaml" \
  | kubectl -n "$NS" apply -f - >/dev/null

kubectl -n "$NS" rollout status deploy/capture --timeout=120s >/dev/null
kubectl -n "$NS" rollout status statefulset/"$REL"-alertmanager --timeout=180s >/dev/null 2>&1 \
  || kubectl -n "$NS" rollout status deploy/"$REL"-alertmanager --timeout=180s >/dev/null

# Port-forward the Alertmanager API and hand-fire an alert.
kubectl -n "$NS" port-forward "svc/${REL}-alertmanager" 29093:9093 >/dev/null 2>&1 &
PF_PID=$!
sleep 3
AM=http://127.0.0.1:29093

fire() { # $1 = endsAt (RFC3339) — future => firing, past => resolved
  curl -sf -X POST "$AM/api/v2/alerts" -H 'Content-Type: application/json' -d @- >/dev/null <<JSON
[{"labels":{"alertname":"VerifyProbe","severity":"critical","namespace":"bex-system"},
  "annotations":{"summary":"synthetic verify probe"},
  "endsAt":"$1"}]
JSON
}

now_plus() { python3 -c "import datetime,sys;print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=int(sys.argv[1]))).strftime('%Y-%m-%dT%H:%M:%SZ'))" "$1"; }

wait_sink() { # $1 = firing|resolved — poll the capture pod for that status
  for _ in $(seq 1 40); do
    kubectl -n "$NS" logs deploy/capture 2>/dev/null \
      | grep -q "\"status\":\"$1\".*VerifyProbe" && { printf '%s notification received\n' "$1"; return; }
    sleep 2
  done
  fail "no $1 notification reached the sink"
}

say "firing alert (endsAt +5m) — expect a WEBHOOK firing at the sink"
fire "$(now_plus 300)"
wait_sink firing

say "resolving alert (endsAt now) — expect a WEBHOOK resolved at the sink"
fire "$(now_plus -1)"
wait_sink resolved

say "PASS: rules test clean; Alertmanager delivered firing + resolved to the channel"
