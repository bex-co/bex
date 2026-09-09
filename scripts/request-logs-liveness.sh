#!/usr/bin/env bash
# Production liveness for the TENANT log pipelines (w6/m131/t004, w3/m83/t002).
#
# Why this exists, and why it is not scripts/logs-verify.sh:
#
#   w6/m131 found type=request silently empty for EVERY service in production —
#   the shipper's ServiceName regex was anchored to the literal namespace
#   `default`, so under ADR043 (a tenant App lives in its workspace's own
#   `tea-<xid>` namespace) every tenant access line missed the regex, had `app`
#   left empty, and was dropped as not_a_tenant_app. Zero rows is a valid
#   response shape, so nothing errored and no alert fired.
#
#   scripts/logs-verify.sh — the acceptance script w3/m8 closed with owed —
#   would NOT have caught it: it deploys its fixture into APP_NS=default and
#   asserts type=request there, and `default` is precisely the one namespace the
#   broken regex still matched. Running it in production would have gone green
#   while every real tenant service was dark. The lesson is the assertion below:
#   it is not enough that SOME stream exists, it must exist for a TENANT
#   namespace.
#
#   w3/m36 was the same silent-zero on the OTHER pipeline: `type=app` empty for
#   every tenant. That is why this probe is parameterised over stream types
#   rather than hardcoding `request` — one dark pipeline must never be masked by
#   a healthy sibling (w3/m83/t002).
#
# What it asserts, cheaply, from Loki's own label index (no line scan, no API
# credentials, no fixture, no traffic generation — the platform's real traffic
# is the fixture), once PER STREAM TYPE:
#
#   1. LIVE:   at least one stream of that type exists in the window.
#   2. TENANT: at least one of those streams is in a `tea-<xid>` namespace —
#              not just `default` or the `dashboard` host-allowlist bucket
#              (w4/m88), both of which survived the outage this guards.
#
# A quiet platform is indistinguishable from a broken one for a single service
# (w6/m131/t002 records why that distinction is not knowable per-resource), but
# it is decidable in aggregate: across every tenant app, over hours, zero lines
# means the pipeline, not the traffic.
#
# Usage:   scripts/request-logs-liveness.sh [window]      # default 6h
# Env:     STREAM_TYPES  space-separated Loki `type` values to assert
#                        (default "request app")
#          KUBECONFIG    cluster to probe (port-forwards svc/loki in monitoring)
#          LOKI_ADDR     host:port of an already-reachable Loki; set => no
#                        port-forward and kubectl is not required
# Exit:    0 = every requested pipeline is producing tenant lines
#          1 = at least one is not (the failure this milestone was filed for)
#          2 = the probe could not run at all (no kubectl/Loki, bad window)
#
# WHICH type failed is read from the machine-readable summary lines, one per
# requested type, not from the exit code:
#
#   RESULT type=request status=live
#   RESULT type=app status=dark
#
# A per-type bit in the exit status was rejected: `2` is already the
# could-not-run code, so a bitmask would collide with it, and the caller
# (.github/workflows/ssh-edge-liveness.yml) needs the per-type answer to pick
# between the `request-logs-down` and `app-logs-down` issues anyway. When the
# probe cannot run, NO RESULT line is emitted for the types it never reached —
# the run still fails loudly, but neither pipeline's issue is opened on evidence
# the probe never gathered.
set -euo pipefail
cd "$(dirname "$0")/.."

WINDOW="${1:-6h}"
STREAM_TYPES="${STREAM_TYPES:-request app}"
MON_NS=monitoring
# A tenant namespace is `tea-` + a 20-char xid (ADR043) — the same shape the
# shipper's attribution regex matches and bex-api's LogQL selector queries by.
TENANT_RE='^tea-[a-z0-9]{20}$'

PIDS=()
cleanup() { for p in "${PIDS[@]:-}"; do kill "$p" 2>/dev/null || true; done; }
trap cleanup EXIT

if [[ -n "${LOKI_ADDR:-}" ]]; then
  LOKI="$LOKI_ADDR"
else
  command -v kubectl >/dev/null || { echo "kubectl is required (or set LOKI_ADDR)" >&2; exit 2; }
  LOKI=127.0.0.1:23101
  kubectl -n "$MON_NS" port-forward svc/loki 23101:3100 >/dev/null 2>&1 &
  PIDS+=($!)
  for _ in $(seq 1 30); do
    curl -sf "http://$LOKI/ready" >/dev/null 2>&1 && break
    sleep 1
  done
  curl -sf "http://$LOKI/ready" >/dev/null 2>&1 \
    || { echo "FAIL: Loki (svc/loki in $MON_NS) never became reachable" >&2; exit 2; }
fi

END="$(date -u +%s)"
case "$WINDOW" in
  *h) START=$((END - ${WINDOW%h} * 3600)) ;;
  *m) START=$((END - ${WINDOW%m} * 60)) ;;
  *)  echo "unsupported window \"$WINDOW\": use e.g. 6h or 90m" >&2; exit 2 ;;
esac

# Per-type remediation. Each pipeline fails for its own reason, and an issue
# body that names the wrong one costs the responder the whole first hour.
remediation() {
  case "$1" in
    request)
      echo "Suspects, in order (Traefik access-log pipeline):"
      echo "First is the ServiceName attribution regex in"
      echo "deploy/gitops/base/log-shipper.yaml drifting from where App CRs"
      echo "actually live (ADR043 tea-<xid> namespaces) — the w6/m131 root"
      echo "cause, guarded in CI by"
      echo "TestShipperRegexAttributesTenantNamespaceServices. Then check that"
      echo "Traefik's access log is enabled and JSON"
      echo "(deploy/gitops/base/values/traefik.values.yaml, logs.access)."
      ;;
    app)
      echo "Suspects, in order (App container-log pipeline):"
      echo "Unlike request logs, this pipeline reads the namespace straight from"
      echo "pod metadata, so a regex is not the suspect: check the Alloy"
      echo "DaemonSet itself (deploy/gitops/base/log-shipper.yaml app_pods"
      echo "pipeline) — it must run on every tenant node (it tolerates all"
      echo "taints for exactly that reason), its app.bex.co/app pod selector"
      echo "must still match what the operator stamps, and Loki must be"
      echo "accepting pushes (kubectl -n monitoring logs ds/log-shipper)."
      ;;
    *)
      echo "Suspects: none are recorded for stream type \"$1\" — start at the"
      echo "Alloy pipeline that produces it (deploy/gitops/base/log-shipper.yaml)."
      ;;
  esac
}

# Loki's label index, scoped to one stream type: which namespaces are actually
# producing lines? This is the decisive read — an empty set means the shipper
# never attached the label, not that one query missed.
namespaces_for() {
  curl -sf --get "http://$LOKI/loki/api/v1/label/namespace/values" \
    --data-urlencode "query={type=\"$1\"}" \
    --data-urlencode "start=${START}000000000" \
    --data-urlencode "end=${END}000000000" \
    | jq -r '.data[]?' | sort
}

dark=0
for type in $STREAM_TYPES; do
  namespaces="$(namespaces_for "$type")" || {
    echo "FAIL: could not read Loki's namespace label index for {type=\"$type\"}" >&2
    exit 2
  }

  if [[ -z "$namespaces" ]]; then
    echo "RESULT type=$type status=dark"
    echo "FAIL[$type]: no type=$type streams exist anywhere in the last $WINDOW."
    echo "      The pipeline is not producing lines at all."
    remediation "$type" | sed 's/^/      /'
    echo
    dark=1
    continue
  fi

  tenant_ns="$(echo "$namespaces" | grep -E "$TENANT_RE" || true)"
  if [[ -z "$tenant_ns" ]]; then
    echo "RESULT type=$type status=dark"
    echo "FAIL[$type]: type=$type streams exist, but NONE in a tenant (tea-<xid>) namespace."
    echo "      Namespaces producing $type lines in the last $WINDOW:"
    echo "$namespaces" | sed 's/^/        /'
    echo "      This is the w6/m131 / w3/m36 failure mode: the shared/platform"
    echo "      buckets (\`default\`, \`dashboard\`) keep flowing while every"
    echo "      tenant line is dropped, so aggregate volume looks healthy."
    remediation "$type" | sed 's/^/      /'
    echo
    dark=1
    continue
  fi

  count="$(echo "$tenant_ns" | wc -l | tr -d ' ')"
  echo "RESULT type=$type status=live"
  echo "PASS[$type]: the $type-log pipeline is live and attributing tenant lines."
  echo "      $count tenant namespace(s) produced type=$type streams in the last $WINDOW:"
  echo "$tenant_ns" | sed 's/^/        /'
  echo
done

if (( dark )); then
  exit 1
fi
echo "PASS: every requested tenant log pipeline ($STREAM_TYPES) is live."
