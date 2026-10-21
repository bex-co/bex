#!/usr/bin/env bash
# Out-of-cluster heartbeat for the alert pipeline (w3/m174; ADR010
# §Alerting heartbeat, ADR088 §1's external check). Prometheus and Alertmanager
# cannot report their own death, and a notification path that fails silently
# looks like a quiet night. From outside the cluster this confirms:
#   1. the cluster API answers (via the admin kubeconfig the probes fetch),
#   2. the always-firing Watchdog is active in Alertmanager,
#   3. no Alertmanager notification failed in the last hour,
#   4. https://api.bex.co and https://dashboard.bex.co answer below 500.
# Alertmanager and Prometheus are read through the API server's service proxy,
# so nothing in the cluster is exposed for this.
#
# Exit: 0 healthy · 1 one or more named failures · 2 unusable (missing tool).
#
# Env:
#   KUBECONFIG                    admin kubeconfig; unset/missing => "cluster unreachable"
#   BEX_API_URL / BEX_DASHBOARD_URL   public endpoints (defaults: https://api.bex.co, https://dashboard.bex.co)
#   BEX_HEARTBEAT_FIXTURE_DIR     self-test: read alerts.json, failures.json,
#                                 api.code, dashboard.code from here instead of
#                                 the cluster and the network; a missing
#                                 alerts.json means the cluster is unreachable
set -euo pipefail
API_URL="${BEX_API_URL:-https://api.bex.co}"
DASHBOARD_URL="${BEX_DASHBOARD_URL:-https://dashboard.bex.co}"
FIXTURES="${BEX_HEARTBEAT_FIXTURE_DIR:-}"
AM_PROXY="/api/v1/namespaces/monitoring/services/prometheus-alertmanager:9093/proxy"
PROM_PROXY="/api/v1/namespaces/monitoring/services/prometheus-server:80/proxy"

unusable() { echo "UNUSABLE: $*" >&2; exit 2; }
command -v jq >/dev/null || unusable "missing required command: jq"
if [ -z "$FIXTURES" ]; then
  command -v kubectl >/dev/null || unusable "missing required command: kubectl"
  command -v curl >/dev/null || unusable "missing required command: curl"
fi

failed=0
fail() { echo "FAIL: $*"; failed=1; }

# cluster_get <path> — raw API-server GET, or the fixture file.
cluster_get() {
  if [ -n "$FIXTURES" ]; then
    local f="$FIXTURES/$2"
    [ -f "$f" ] && cat "$f"
    return 0
  fi
  [ -n "${KUBECONFIG:-}" ] && [ -f "$KUBECONFIG" ] || return 0
  kubectl get --raw "$1" --request-timeout=20s 2>/dev/null || true
}

alerts="$(cluster_get "$AM_PROXY/api/v2/alerts?active=true&filter=alertname%3D%22Watchdog%22" alerts.json)"
if [ -z "$alerts" ]; then
  fail "cluster API or Alertmanager unreachable: no answer from Alertmanager through the API server (kubeconfig fetch failed, control plane down, or Alertmanager down)"
else
  active="$(printf '%s' "$alerts" | jq '[.[] | select(.labels.alertname == "Watchdog" and .status.state == "active")] | length' 2>/dev/null || echo 0)"
  [ "${active:-0}" -gt 0 ] || fail "Watchdog is not active in Alertmanager: Prometheus is not evaluating rules or not reaching Alertmanager"
fi

query='sum(increase(alertmanager_notifications_failed_total{job="alertmanager"}[1h]))'
failures="$(cluster_get "$PROM_PROXY/api/v1/query?query=$(jq -rn --arg q "$query" '$q|@uri')" failures.json)"
if [ -z "$failures" ]; then
  fail "Prometheus unreachable through the API server: notification-failure count unknown"
else
  count="$(printf '%s' "$failures" | jq -r '[.data.result[]?.value[1] | tonumber] | add // 0' 2>/dev/null || echo unknown)"
  [[ "$count" =~ ^[0-9]+(\.[0-9]+)?$ ]] || count=unknown
  case "$count" in
    unknown) fail "Prometheus answered without a readable notification-failure count" ;;
    *) awk -v c="$count" 'BEGIN { exit !(c > 0) }' && fail "Alertmanager failed $count notification(s) in the last hour: alerts may be firing unseen (check alertmanager-smtp and SendGrid activity)" ;;
  esac
fi

# probe <name> <url> — any HTTP answer below 500 is up; three tries.
probe() {
  local name="$1" url="$2" code=000 i
  if [ -n "$FIXTURES" ]; then
    code="$(cat "$FIXTURES/$name.code" 2>/dev/null || echo 000)"
  else
    for i in 1 2 3; do
      code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 20 "$url" 2>/dev/null || echo 000)"
      [ "$code" != 000 ] && [ "$code" -lt 500 ] && break
      sleep 5
    done
  fi
  [[ "$code" =~ ^[0-9]{3}$ ]] || code=000
  if [ "$code" = 000 ] || [ "$code" -ge 500 ]; then
    fail "$url is down (HTTP $code after retries)"
  fi
}
probe api "$API_URL"
probe dashboard "$DASHBOARD_URL"

[ "$failed" -eq 0 ] && echo "OK: Watchdog active, no failed notifications in 1h, $API_URL and $DASHBOARD_URL answering"
exit "$failed"
