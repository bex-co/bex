#!/usr/bin/env bash
# Production liveness for the tenant's own view of a running service (w3/m83/t003).
#
# Why this exists, and why it is not scripts/request-logs-liveness.sh:
#
#   request-logs-liveness.sh reads Loki's label index directly. That catches a
#   dead SHIPPER, but it is blind to the failure class that has actually bitten
#   twice: bex-api asking Loki or Prometheus the WRONG QUESTION. In w6/m110 the
#   metric series existed and the query named them wrong, so every tenant saw an
#   empty chart while the platform's own probes were green. A 200 with no rows
#   is a valid response shape, so nothing errored and no alert fired.
#
#   The only way to catch that is to ask the API the way a tenant does, with a
#   tenant's credential, about a service whose answer is KNOWN to be non-empty.
#   So this probe generates the traffic itself and then requires bex-api to
#   report it back through all three ADR010 pipelines.
#
# The fixture is a permanent first-party canary: one free web service from
# examples/hello-go in the `bex-canary` workspace, plus a workspace-scoped API
# key. Free-tier hibernation is fine and in fact desirable — the probe's own
# request wakes it, which exercises the activator path too, and the wake shows
# up in the events feed.
#
# Four stages, each with its own exit code so the workflow can name the broken
# pipeline in the issue it opens:
#
#   1 (exit 3) WAKE     GET the canary's public host with a unique query param;
#                       require 200 within the wake budget.
#   2 (exit 4) LOGS     GET /v1/logs?type=request must return that exact
#                       request line (matched on the unique param, so a stale
#                       line from an earlier run cannot satisfy it), and
#                       type=app must return the service's own output.
#   3 (exit 5) METRICS  GET /v1/metrics/cpu + /memory over the last hour must
#                       carry at least one sample (a woken service always does).
#   4 (exit 6) EVENTS   GET /v1/services/{id}/events must carry this run's
#                       service_woken when stage 1 woke the service, else an
#                       event newer than the event window (both polled).
#
# Usage:   scripts/tenant-view-liveness.sh
# Env:     BEX_API_URL              default https://api.bex.co
#          BEX_OAUTH_ISSUER         default https://oauth.bex.co
#          BEX_CANARY_API_KEY       <key-id>:<key-secret>, canary workspace only
#          BEX_CANARY_SERVICE_ID    srv-… of the canary web service
#          BEX_CANARY_URL           its public host (https://…onbex.co)
#          BEX_CANARY_WAKE_BUDGET   seconds for stage 1 (default 60)
#          BEX_CANARY_LOG_BUDGET    seconds for stage 2 (default 180)
#          BEX_CANARY_METRIC_BUDGET seconds for stage 3 (default 120)
#          BEX_CANARY_EVENT_WINDOW  seconds an event may be older than (default 3600)
#          BEX_CANARY_WOKEN_BUDGET  seconds stage 4 polls the events feed (default 120)
#          BEX_CANARY_WAKE_THRESHOLD stage 1 seconds that count as a wake (default 3)
#          BEX_CANARY_POLL_INTERVAL seconds between polls in stages 2-4 (default 5)
#          BEX_CANARY_REQUIRE_NONCE 1 (default) = stage 2 must find THIS run's
#                       request line. The nonce rides in the URL PATH, never in a
#                       query parameter: the edge drops every query parameter
#                       before the access-log sink on purpose (deploy-hook `key=`
#                       redaction — deploy/gitops/base/values/traefik.values.yaml),
#                       so a `?nonce=…` marker never reaches Loki and this stage
#                       could never pass. Set to 0 only if a future edge
#                       configuration also strips the path, and record why (the
#                       probe then degrades to "some request line exists for this
#                       service", which is weaker).
# Exit:    0 pass · 2 config error · 3 wake · 4 logs · 5 metrics · 6 events
#
# Nothing secret is printed: the API key is exchanged through a private file and
# the bearer rides in curl's -K header file (scripts/lib/canary-api.sh).
set -euo pipefail
cd "$(dirname "$0")/.."

# shellcheck source=scripts/lib/canary-api.sh
. scripts/lib/canary-api.sh

API="$(canary_api_url)"
SERVICE_ID="${BEX_CANARY_SERVICE_ID:-}"
CANARY_URL="${BEX_CANARY_URL:-}"
WAKE_BUDGET="${BEX_CANARY_WAKE_BUDGET:-60}"
LOG_BUDGET="${BEX_CANARY_LOG_BUDGET:-180}"
# Must exceed TWO cAdvisor scrape intervals (15s each, deploy/gitops/base/
# prometheus.yaml), because cpu is a rate: bex-api asks Prometheus for
# rate(container_cpu_usage_seconds_total[60s]) and a rate needs two samples
# inside its lookback before it yields any point at all. A just-woken pod has
# 0-1. Memory is a plain gauge and needs only one. 120s leaves room for a slow
# first scrape; anything under 30s cannot work no matter how it is spelled.
METRIC_BUDGET="${BEX_CANARY_METRIC_BUDGET:-120}"
EVENT_WINDOW="${BEX_CANARY_EVENT_WINDOW:-3600}"
# The events projection records a wake when its reconciler next observes the
# service Running (store/reconciler.go resyncs every 30s), so allow a few.
WOKEN_BUDGET="${BEX_CANARY_WOKEN_BUDGET:-120}"
REQUIRE_NONCE="${BEX_CANARY_REQUIRE_NONCE:-1}"
# A hibernated service is woken by the activator HOLDING the first request until
# the pod is Ready (~14s, w4/517), and answering it 200 — no retry happens. So a
# wake is measured by elapsed time, not by retries: anything this slow was not
# served by an already-running pod.
WAKE_THRESHOLD="${BEX_CANARY_WAKE_THRESHOLD:-3}"
POLL="${BEX_CANARY_POLL_INTERVAL:-5}"

canary_require_tools

case "$SERVICE_ID" in
  srv-*) ;;
  *)
    echo "error: BEX_CANARY_SERVICE_ID must be the canary service id (srv-…)" >&2
    exit 2
    ;;
esac
case "$CANARY_URL" in
  https://*) ;;
  *)
    echo "error: BEX_CANARY_URL must be the canary service's public https host" >&2
    exit 2
    ;;
esac
CANARY_URL="${CANARY_URL%/}"

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf -- "$TMP_DIR"; }
trap cleanup EXIT

canary_login "$TMP_DIR"

# One value that exists nowhere else, so a matching log line can only have come
# from this run. `date +%s` alone is not enough — two runs in the same second
# (a dispatch racing the cron) would alias.
NONCE="tvl$(date -u +%s)$(head -c 6 /dev/urandom | od -An -tx1 | tr -d ' \n')"
# In the path, not the query string: Traefik is configured with
# `accessLog.fields.queryParameters.defaultMode: drop` so deploy-hook `key=`
# credentials never reach the log sink, which strips a `?marker=…` too. RequestPath
# is kept verbatim, so `/bexProbe/<nonce>` survives into the line stage 2 reads.
# The canary (examples/hello-go) answers 200 on every path, so this is still a
# plain successful request.
PROBE_PATH="/bexProbe/$NONCE"
# Loki/Prometheus ingestion is not instantaneous and clocks are not identical;
# start the query window before the request so a few seconds of skew cannot
# hide the line the probe is looking for.
WINDOW_START="$(date -u -d '-10 minutes' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null \
  || date -u -v-10M +%Y-%m-%dT%H:%M:%SZ)"
# Stage 4 reads the events feed over EVENT_WINDOW, not the 10-minute log window:
# the window it asserts and the window it asks for must be the same one.
EVENTS_START="$(date -u -d "-${EVENT_WINDOW} seconds" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null \
  || date -u -v-"${EVENT_WINDOW}"S +%Y-%m-%dT%H:%M:%SZ)"

echo "== w3/m83 tenant-view canary: $SERVICE_ID =="

# --- Stage 1: the tenant's own request reaches the service ---------------------
# A free service may be hibernated, so a slow first response is expected, not a
# failure: this budget is the activator's wake path, which the probe deliberately
# exercises.
echo "==> 1/4 wake: request the public host (budget ${WAKE_BUDGET}s)"
# A service_woken for this run's wake is stamped after this, give or take the
# runner/API clock skew the slack covers.
WAKE_START="$(date -u -d '-30 seconds' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null \
  || date -u -v-30S +%Y-%m-%dT%H:%M:%SZ)"
status=000
wake_t0="$(date -u +%s)"
while [ $(($(date -u +%s) - wake_t0)) -lt "$WAKE_BUDGET" ]; do
  status="$(curl -sS -o /dev/null -w '%{http_code}' \
    --connect-timeout 5 --max-time 30 "$CANARY_URL$PROBE_PATH" 2>/dev/null || echo 000)"
  [ "$status" = "200" ] && break
  sleep 3
done
if [ "$status" != "200" ]; then
  echo "FAIL(1/4 wake): $CANARY_URL answered $status, not 200, within ${WAKE_BUDGET}s."
  echo "      The canary service is down, or the activator never woke it."
  echo "      This is the tenant-visible symptom: a service that exists but does not serve."
  exit 3
fi
wake_seconds=$(($(date -u +%s) - wake_t0))
echo "  ok: 200 in ${wake_seconds}s (nonce recorded, not a secret: $NONCE)"

# --- Stage 2: bex-api reports that request back through the logs pipeline ------
echo "==> 2/4 logs: bex-api must return this run's request line and the app stream (budget ${LOG_BUDGET}s)"
request_hit=""
app_hit=""
waited=0
while [ "$waited" -lt "$LOG_BUDGET" ]; do
  if [ -z "$request_hit" ]; then
    if acurl -o "$TMP_DIR/request.json" --get "$API/v1/logs" \
      --data-urlencode "resource=$SERVICE_ID" \
      --data-urlencode "type=request" \
      --data-urlencode "startTime=$WINDOW_START" \
      --data-urlencode "limit=100" 2>/dev/null; then
      if [ "$REQUIRE_NONCE" = "1" ]; then
        # Match the nonce in the line's `path` label (w4/m325: the message is
        # Render's summary, which has no path) or, from a bex-api older than
        # that, in the raw access line. The nonce is unique to this run, so
        # neither match can be satisfied by a different request.
        jq -e --arg n "$NONCE" '[.logs[]? | select((.message | contains($n)) or any(.labels[]?; .name == "path" and (.value | contains($n))))] | length > 0' \
          "$TMP_DIR/request.json" >/dev/null 2>&1 && request_hit=1
      else
        jq -e '(.logs | length) > 0' "$TMP_DIR/request.json" >/dev/null 2>&1 && request_hit=1
      fi
    fi
  fi
  if [ -z "$app_hit" ]; then
    if acurl -o "$TMP_DIR/app.json" --get "$API/v1/logs" \
      --data-urlencode "resource=$SERVICE_ID" \
      --data-urlencode "type=app" \
      --data-urlencode "startTime=$WINDOW_START" \
      --data-urlencode "limit=20" 2>/dev/null; then
      jq -e '(.logs | length) > 0' "$TMP_DIR/app.json" >/dev/null 2>&1 && app_hit=1
    fi
  fi
  [ -n "$request_hit" ] && [ -n "$app_hit" ] && break
  sleep "$POLL"
  waited=$((waited + POLL))
done
if [ -z "$request_hit" ] || [ -z "$app_hit" ]; then
  echo "FAIL(2/4 logs): after ${waited}s bex-api still does not report this service's own traffic."
  echo "      type=request line for this run: ${request_hit:+found}${request_hit:-MISSING}"
  echo "      type=app lines for this service: ${app_hit:+found}${app_hit:-MISSING}"
  echo
  echo "      The request DID reach the service (stage 1 got 200), so this is a"
  echo "      read-path failure, not a quiet service — the w6/m131 (shipper"
  echo "      attribution) or w6/m110 (bex-api queries the wrong selector) class."
  echo "      Compare with scripts/request-logs-liveness.sh: if that is green and"
  echo "      this is red, the shipper is fine and the API's query is wrong."
  exit 4
fi
echo "  ok: this run's request line and the app stream are both readable through /v1/logs"

# --- Stage 3: the metrics pipeline has samples for the same service -----------
# Budgeted, unlike the single-shot assert this used to be. The fixture is a free
# service that hibernates between six-hourly runs, so every run measures a pod
# that is seconds old — and cpu cannot answer yet even when everything is
# healthy (see METRIC_BUDGET above). Asserting once made the canary open
# "Tenant view is broken" issues for a scrape that had simply not landed.
#
# cpu and memory are polled INDEPENDENTLY and both reported, because the two
# answers mean different things: cpu empty while memory has samples is a cold
# rate, and cpu empty WITH memory empty is the w6/m110 class — an empty series
# behind a 200. The old loop exited on cpu first, so it never asked memory and
# both failures printed the same line.
echo "==> 3/4 metrics: cpu + memory must carry a sample (budget ${METRIC_BUDGET}s)"
waited=0
cpu_hit=""
mem_hit=""
while [ "$waited" -lt "$METRIC_BUDGET" ]; do
  for metric in cpu memory; do
    case "$metric" in
      cpu) [ -n "$cpu_hit" ] && continue ;;
      memory) [ -n "$mem_hit" ] && continue ;;
    esac
    if acurl -o "$TMP_DIR/$metric.json" --get "$API/v1/metrics/$metric" \
      --data-urlencode "resource=$SERVICE_ID" \
      --data-urlencode "startTime=$WINDOW_START" \
      --data-urlencode "resolutionSeconds=60" 2>/dev/null; then
      if jq -e '[.[]? | .values[]?] | length > 0' "$TMP_DIR/$metric.json" >/dev/null 2>&1; then
        case "$metric" in
          cpu) cpu_hit=1 ;;
          memory) mem_hit=1 ;;
        esac
      fi
    fi
  done
  [ -n "$cpu_hit" ] && [ -n "$mem_hit" ] && break
  sleep "$POLL"
  waited=$((waited + POLL))
done
if [ -z "$cpu_hit" ] || [ -z "$mem_hit" ]; then
  echo "FAIL(3/4 metrics): after ${waited}s a serving service still has no samples."
  echo "      cpu:    ${cpu_hit:+present}${cpu_hit:-EMPTY}"
  echo "      memory: ${mem_hit:+present}${mem_hit:-EMPTY}"
  echo
  if [ -z "$cpu_hit" ] && [ -n "$mem_hit" ]; then
    echo "      Only cpu is empty. cpu is a rate over a 60s lookback and memory is"
    echo "      a gauge, so this shape means the scrape is landing but the rate"
    echo "      still has under two points — raise BEX_CANARY_METRIC_BUDGET before"
    echo "      suspecting bex-api."
  else
    echo "      Empty series with a 200 is exactly the w6/m110 failure: the data"
    echo "      exists in Prometheus and bex-api's query names it wrong. Check the"
    echo "      cadvisor selector in lego/backend/internal/metrics against what"
    echo "      the scrape config actually produces before suspecting the scrape."
  fi
  exit 5
fi
echo "  ok: cpu and memory both carry samples (after ${waited}s)"

# --- Stage 4: the events feed advanced -----------------------------------------
fetch_events() {
  if ! acurl -o "$TMP_DIR/events.json" --get "$API/v1/services/$SERVICE_ID/events" \
    --data-urlencode "startTime=$EVENTS_START" \
    --data-urlencode "limit=20" 2>/dev/null; then
    echo "FAIL(4/4 events): GET /v1/services/$SERVICE_ID/events failed"
    exit 6
  fi
}
# poll_events <jq args…> — re-reads the feed until the jq program prints a
# timestamp or WOKEN_BUDGET runs out; leaves it in $hit and the wait in $waited.
poll_events() {
  waited=0
  while :; do
    fetch_events
    hit="$(jq -r "$@" "$TMP_DIR/events.json")"
    if [ -n "$hit" ] || [ "$waited" -ge "$WOKEN_BUDGET" ]; then return; fi
    sleep "$POLL"
    waited=$((waited + POLL))
  done
}
if [ "$wake_seconds" -ge "$WAKE_THRESHOLD" ]; then
  # A service this probe had to wake must report that wake as service_woken
  # (w1/m173): any recent event would also pass while the projection had
  # stopped recording wakes.
  echo "==> 4/4 events: this run's wake must appear as service_woken (budget ${WOKEN_BUDGET}s)"
  # shellcheck disable=SC2016 # $since is a jq variable
  poll_events --arg since "$WAKE_START" '[.[]? | objects | (.event // .)
    | select(.type == "service_woken" and .timestamp >= $since) | .timestamp]
    | sort | last // empty'
  woken="$hit"
  if [ -z "$woken" ]; then
    echo "FAIL(4/4 events): the service took ${wake_seconds}s to answer, so this run"
    echo "      woke it, but no service_woken since $WAKE_START appeared within"
    echo "      ${WOKEN_BUDGET}s: the events projection is not recording wakes, so no"
    echo "      tenant can see why their service changed state. Events in the window:"
    jq -r '.[]? | objects | (.event // .) | "        \(.timestamp) \(.type)"' "$TMP_DIR/events.json"
    exit 6
  fi
  echo "  ok: service_woken $woken (after ${waited}s)"
else
  # Stage 1 answered fast, so the pod was most likely already running — but a
  # wake just under the threshold still lands its service_woken on the
  # projection's next ~30s pass, so poll instead of reading once.
  echo "==> 4/4 events: the newest event must be within ${EVENT_WINDOW}s (budget ${WOKEN_BUDGET}s)"
  poll_events '[.[]? | objects | (.event // .).timestamp] | sort | last // empty'
  newest="$hit"
  if [ -z "$newest" ]; then
    # Events come only from state edges (store/event_facts.go): a service that
    # stayed up for the whole window has an empty feed, as on Render. w1/m173:
    # from 2026-10-07 to 10-15 the canary never slept because a cert-manager
    # ACME self-check hit its public URL every ~10s.
    echo "FAIL(4/4 events): no event in the last ${EVENT_WINDOW}s after ${waited}s of polling."
    echo "      Stage 1 answered in ${wake_seconds}s (under the ${WAKE_THRESHOLD}s wake threshold),"
    echo "      so the service was serving before this run. Either it has not"
    echo "      slept for the whole window — its request logs (GET /v1/logs?type=request)"
    echo "      name a repeating caller — or the events projection stopped recording."
    exit 6
  fi
  newest_epoch="$(date -u -d "$newest" +%s 2>/dev/null || echo 0)"
  if [ "$newest_epoch" = 0 ]; then
    # BSD date (a local operator run on macOS) needs the explicit format.
    newest_epoch="$(date -u -j -f '%Y-%m-%dT%H:%M:%S' "${newest%%.*}" +%s 2>/dev/null || echo 0)"
  fi
  age=$(($(date -u +%s) - newest_epoch))
  if [ "$newest_epoch" = 0 ] || [ "$age" -gt "$EVENT_WINDOW" ]; then
    echo "FAIL(4/4 events): newest event is $newest (${age}s old, budget ${EVENT_WINDOW}s)."
    echo "      The feed is stale: the pipeline is answering with history but not"
    echo "      recording what just happened to this service."
    exit 6
  fi
  echo "  ok: newest event $newest (${age}s old)"
fi

echo
echo "PASS: the tenant view of $SERVICE_ID is intact — the service served this"
echo "      probe's request, and bex-api reported it back through logs, metrics,"
echo "      and events with a tenant-scoped credential."
