#!/usr/bin/env bash
# metronome-reconcile.sh — w7/m47 acceptance: Metronome totals vs usage_hourly.
#
# Proves the event→metric mapping (ADR040 §5) is correct: for a chosen period,
# Metronome's computed billable-metric totals must equal the usage_hourly
# quantities bex exported. This is the gate before m48 exposes any real cost.
#
# For each meter kind it compares:
#   local     = SUM(usage_hourly.quantity) over EMITTED rows in [start, end)
#   metronome = the matching billable metric's total for the same window
# and asserts equality within a rounding tolerance, printing a per-meter table
# and a single PASS/FAIL (nonzero exit on FAIL). A correctly-mapped export
# passes; a wrong billable_metric_id / filter fails loudly.
#
# Only EMITTED rows (emitted_at IS NOT NULL) are compared — those are exactly the
# rows Metronome received. Sealed-but-unemitted rows are intentionally excluded.
#
# Usage:
#   BEX_CP_DB_URI=postgres://…            (required — the control-plane store)
#   BEX_METRONOME_TOKEN=…                 (required — bearer)
#   BEX_METRONOME_URL=https://api.metronome.com   (optional; this is the default)
#   scripts/metronome-reconcile.sh \
#       --start 2026-07-01T00:00:00Z --end 2026-07-02T00:00:00Z \
#       --metrics metrics.json \        # {"instance_seconds":"<id>","egress_bytes":"<id>",…}
#       [--customer tea-abc123] \       # restrict to one workspace (ingest alias)
#       [--tolerance 1]                 # allowed abs diff per meter (default 1)
#
# metrics.json maps each event_type to its Metronome billable_metric_id, created
# per docs/runbooks/metronome-billing-setup.md §2. Kinds absent from the map are
# reported local-only (Metronome side skipped, marked SKIP — never silently
# passed).
set -euo pipefail

API="${BEX_METRONOME_URL:-https://api.metronome.com}"
START="" END="" METRICS="" CUSTOMER="" TOL=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --start) START="$2"; shift 2 ;;
    --end) END="$2"; shift 2 ;;
    --metrics) METRICS="$2"; shift 2 ;;
    --customer) CUSTOMER="$2"; shift 2 ;;
    --tolerance) TOL="$2"; shift 2 ;;
    -h|--help) sed -n '2,40p' "$0"; exit 0 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

fail() { echo "error: $*" >&2; exit 2; }
command -v psql >/dev/null || fail "psql not found"
command -v curl >/dev/null || fail "curl not found"
command -v jq >/dev/null || fail "jq not found"
[[ -n "${BEX_CP_DB_URI:-}" ]] || fail "BEX_CP_DB_URI is required"
[[ -n "${BEX_METRONOME_TOKEN:-}" ]] || fail "BEX_METRONOME_TOKEN is required"
[[ -n "$START" && -n "$END" ]] || fail "--start and --end (RFC3339) are required"

echo "== reconcile $START .. $END ${CUSTOMER:+(customer $CUSTOMER)} =="
echo "   Metronome: $API   tolerance: ±$TOL per meter"
echo

# --- local side: usage_hourly emitted totals, grouped per (kind,resource_kind,tier)
cust_filter=""
[[ -n "$CUSTOMER" ]] && cust_filter="AND workspace_id = '${CUSTOMER//\'/}'"
LOCAL_TSV="$(psql "$BEX_CP_DB_URI" -At -F $'\t' -c "
  SELECT kind, resource_kind, tier, COALESCE(SUM(quantity),0)
  FROM usage_hourly
  WHERE emitted_at IS NOT NULL
    AND window_start >= '${START//\'/}'
    AND window_start <  '${END//\'/}'
    $cust_filter
  GROUP BY kind, resource_kind, tier
  ORDER BY kind, resource_kind, tier")"

# local total per kind (sum across resource_kind/tier) — the reconciled quantity.
declare -A LOCAL_KIND
echo "local usage_hourly (emitted):"
if [[ -z "$LOCAL_TSV" ]]; then
  echo "  (no emitted rows in window)"
else
  while IFS=$'\t' read -r kind rk tier qty; do
    printf "  %-18s %-10s %-12s %s\n" "$kind" "$rk" "${tier:-—}" "$qty"
    LOCAL_KIND[$kind]=$(( ${LOCAL_KIND[$kind]:-0} + qty ))
  done <<< "$LOCAL_TSV"
fi
echo

# --- Metronome side: each billable metric's total for the same window.
# POST /v1/usage returns the aggregated billable-metric value(s); we sum them.
metronome_total() {  # $1 = billable_metric_id -> prints integer total
  local mid="$1" body
  body=$(jq -n --arg mid "$mid" --arg s "$START" --arg e "$END" --arg c "$CUSTOMER" '
    { billable_metric_id: $mid, window_size: "none", starting_on: $s, ending_before: $e }
    + (if $c == "" then {} else { customer_ids: [$c] } end)')
  curl -sS -X POST "$API/v1/usage" \
    -H "Authorization: Bearer $BEX_METRONOME_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$body" \
  | jq '[.. | objects | .value? // empty | tonumber] | add // 0 | floor'
}

overall=PASS
[[ -n "$METRICS" ]] && { [[ -f "$METRICS" ]] || fail "--metrics file not found: $METRICS"; }

printf "%-18s %14s %14s %8s\n" "meter" "usage_hourly" "metronome" "result"
printf -- "------------------ -------------- -------------- --------\n"
for kind in instance_seconds egress_bytes build_seconds storage_gb_seconds; do
  local_total=${LOCAL_KIND[$kind]:-0}
  mid=""
  [[ -n "$METRICS" ]] && mid="$(jq -r --arg k "$kind" '.[$k] // ""' "$METRICS")"
  if [[ -z "$mid" ]]; then
    printf "%-18s %14s %14s %8s\n" "$kind" "$local_total" "-" "SKIP"
    [[ "$local_total" -ne 0 ]] && overall=FAIL   # exported usage with no metric mapping = a gap
    continue
  fi
  m_total="$(metronome_total "$mid")"
  diff=$(( local_total > m_total ? local_total - m_total : m_total - local_total ))
  if [[ "$diff" -le "$TOL" ]]; then res=ok; else res=MISMATCH; overall=FAIL; fi
  printf "%-18s %14s %14s %8s\n" "$kind" "$local_total" "$m_total" "$res"
done

echo
echo "RESULT: $overall"
[[ "$overall" == PASS ]] || exit 1
