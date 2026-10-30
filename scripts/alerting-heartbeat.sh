#!/usr/bin/env bash
# Out-of-cluster heartbeat for the alert pipeline (w3/m174; ADR010
# §Alerting heartbeat, ADR088 §1's external check). Prometheus and Alertmanager
# cannot report their own death, and a notification path that fails silently
# looks like a quiet night. From outside the cluster this confirms:
#   1. the cluster API answers (via the admin kubeconfig the probes fetch),
#   2. the always-firing Watchdog is active in Alertmanager,
#   3. no Alertmanager notification failed in the last hour,
#   4. https://api.bex.co and https://dashboard.bex.co answer below 500.
# Alertmanager and Prometheus are read through `kubectl port-forward`, so nothing
# in the cluster is exposed for this. Not the API server's service proxy: the
# apiserver dials Pods from the control-plane host network, which `monitoring`'s
# deny-tenant-ingress (Cilium: remote-node) refuses, so every proxied read timed
# out (w3/m174 t011, #99). port-forward enters the Pod's netns via the kubelet.
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
AM_SVC="svc/prometheus-alertmanager"
AM_PORT=9093
PROM_SVC="svc/prometheus-server"
PROM_PORT=80

unusable() { echo "UNUSABLE: $*" >&2; exit 2; }
command -v jq >/dev/null || unusable "missing required command: jq"
if [ -z "$FIXTURES" ]; then
  command -v kubectl >/dev/null || unusable "missing required command: kubectl"
  command -v curl >/dev/null || unusable "missing required command: curl"
fi

failed=0
fail() { echo "FAIL: $*"; failed=1; }

# forward <service> <port> — start a port-forward in the background and set
# $forward_url to its local base URL; on failure leave $forward_url empty and the
# first error line in $forward_err. Runs in this shell (never inside $(...)) so
# the pid reaches the exit trap. Every forward is stopped on exit.
pf_dir="$(mktemp -d)"
pf_pids=()
cleanup() {
  local pid
  for pid in "${pf_pids[@]+"${pf_pids[@]}"}"; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$pf_dir"
}
trap cleanup EXIT
forward_url="" forward_err=""
forward() {
  local log="$pf_dir/${1##*/}.log" i port
  forward_url="" forward_err=""
  kubectl -n monitoring port-forward "$1" ":$2" --request-timeout=20s >"$log" 2>&1 &
  pf_pids+=("$!")
  for i in $(seq 1 40); do
    port="$(sed -n 's/^Forwarding from 127\.0\.0\.1:\([0-9]*\) .*/\1/p' "$log" | head -1)"
    [ -n "$port" ] && { forward_url="http://127.0.0.1:$port"; return 0; }
    kill -0 "${pf_pids[${#pf_pids[@]}-1]}" 2>/dev/null || break
    sleep 0.5
  done
  forward_err="$(grep -v '^Forwarding from' "$log" | head -1)"
  forward_err="${forward_err:-port-forward to $1 did not start within 20s}"
  return 0
}

# cluster_get <base-url> <path> <fixture> — GET through a forward, or the fixture
# file. Leaves the first error line in $get_err.
get_err=""
cluster_get() {
  get_err=""
  if [ -n "$FIXTURES" ]; then
    local f="$FIXTURES/$3"
    [ -f "$f" ] && cat "$f"
    return 0
  fi
  [ -n "$1" ] || return 0
  curl -sS --fail --max-time 20 "$1$2" 2>"$pf_dir/curl.err" || get_err="$(head -1 "$pf_dir/curl.err")"
  return 0
}

am_url="" prom_url="" am_err="" prom_err=""
if [ -z "$FIXTURES" ]; then
  if [ -z "${KUBECONFIG:-}" ] || [ ! -f "$KUBECONFIG" ]; then
    am_err="no kubeconfig (fetch failed)"
    prom_err="$am_err"
  else
    forward "$AM_SVC" "$AM_PORT"
    am_url="$forward_url" am_err="$forward_err"
    forward "$PROM_SVC" "$PROM_PORT"
    prom_url="$forward_url" prom_err="$forward_err"
  fi
fi
# why <error> — " (<error>)" or nothing.
why() { [ -n "$1" ] && printf ' (%s)' "$1"; return 0; }

alerts="$(cluster_get "$am_url" "/api/v2/alerts?active=true&filter=alertname%3D%22Watchdog%22" alerts.json)"
[ -n "$am_err" ] || am_err="$get_err"
if [ -z "$alerts" ]; then
  fail "cluster API or Alertmanager unreachable: no answer from Alertmanager (kubeconfig fetch failed, control plane down, or Alertmanager down)$(why "$am_err")"
else
  active="$(printf '%s' "$alerts" | jq '[.[] | select(.labels.alertname == "Watchdog" and .status.state == "active")] | length' 2>/dev/null || echo 0)"
  [ "${active:-0}" -gt 0 ] || fail "Watchdog is not active in Alertmanager: Prometheus is not evaluating rules or not reaching Alertmanager"
fi

query='sum(increase(alertmanager_notifications_failed_total{job="alertmanager"}[1h]))'
failures="$(cluster_get "$prom_url" "/api/v1/query?query=$(jq -rn --arg q "$query" '$q|@uri')" failures.json)"
[ -n "$prom_err" ] || prom_err="$get_err"
if [ -z "$failures" ]; then
  fail "Prometheus unreachable: notification-failure count unknown$(why "$prom_err")"
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
