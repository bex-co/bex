#!/usr/bin/env bash
# Production liveness for the Traefik REQUEST-log pipeline (w6/m131/t004).
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
#   it is not enough that SOME request stream exists, it must exist for a
#   TENANT namespace.
#
# What it asserts, cheaply, from Loki's own label index (no line scan, no API
# credentials, no fixture, no traffic generation — the platform's real traffic
# is the fixture):
#
#   1. LIVE:   at least one `type=request` stream exists in the window.
#   2. TENANT: at least one of those streams is in a `tea-<xid>` namespace —
#              not just `default` or the `dashboard` host-allowlist bucket
#              (w4/m88), both of which survived the outage this guards.
#
# A quiet platform is indistinguishable from a broken one for a single service
# (w6/m131/t002 records why that distinction is not knowable per-resource), but
# it is decidable in aggregate: across every tenant app, over hours, zero
# request lines means the pipeline, not the traffic.
#
# Usage:   scripts/request-logs-liveness.sh [window]      # default 6h
# Env:     KUBECONFIG   cluster to probe (port-forwards svc/loki in monitoring)
#          LOKI_ADDR    host:port of an already-reachable Loki; set => no
#                       port-forward and kubectl is not required
# Exit:    0 = the request pipeline is producing tenant lines
#          1 = it is not (the failure this milestone was filed for)
set -euo pipefail
cd "$(dirname "$0")/.."

WINDOW="${1:-6h}"
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

# Loki's label index, scoped to request streams: which namespaces are actually
# producing access lines? This is the decisive read — an empty set means the
# shipper never attached the label, not that one query missed.
namespaces="$(curl -sf --get "http://$LOKI/loki/api/v1/label/namespace/values" \
  --data-urlencode 'query={type="request"}' \
  --data-urlencode "start=${START}000000000" \
  --data-urlencode "end=${END}000000000" \
  | jq -r '.data[]?' | sort)" || {
  echo "FAIL: could not read Loki's namespace label index for {type=\"request\"}" >&2
  exit 2
}

if [[ -z "$namespaces" ]]; then
  echo "FAIL: no type=request streams exist anywhere in the last $WINDOW."
  echo "      The Traefik access-log pipeline is not producing lines at all."
  echo "      Check the log-shipper DaemonSet (deploy/gitops/base/log-shipper.yaml)"
  echo "      and that Traefik's access log is enabled and JSON"
  echo "      (deploy/gitops/base/values/traefik.values.yaml, logs.access)."
  exit 1
fi

tenant_ns="$(echo "$namespaces" | grep -E "$TENANT_RE" || true)"
if [[ -z "$tenant_ns" ]]; then
  echo "FAIL: type=request streams exist, but NONE in a tenant (tea-<xid>) namespace."
  echo "      Namespaces producing request lines in the last $WINDOW:"
  echo "$namespaces" | sed 's/^/        /'
  echo
  echo "      This is exactly the w6/m131 failure mode: the shipper's ServiceName"
  echo "      attribution regex (log-shipper.yaml) is not matching per-tenant"
  echo "      namespaces, so every tenant access line is dropped as"
  echo "      not_a_tenant_app while \`default\`/\`dashboard\` keep flowing."
  echo "      TestShipperRegexAttributesTenantNamespaceServices guards the regex."
  exit 1
fi

count="$(echo "$tenant_ns" | wc -l | tr -d ' ')"
echo "PASS: the request-log pipeline is live and attributing tenant lines."
echo "      $count tenant namespace(s) produced type=request streams in the last $WINDOW:"
echo "$tenant_ns" | sed 's/^/        /'
