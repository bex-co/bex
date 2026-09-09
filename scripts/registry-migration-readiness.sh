#!/usr/bin/env bash
# Read-only ADR055 Phase-4 clean-window readiness (w2/m93).
#
# Combines the current tombstone/scoped inventory with retained Loki evidence
# of legacy Zot pulls and static-server legacy-prefix GETs. Never mutates
# grants, flags, or artifacts.
#
# Verdicts (exactly one RESULT line):
#   RESULT status=clean
#   RESULT status=legacy_reads_detected
#   RESULT status=insufficient_evidence
#
# Fail-closed: missing history, dark collection, source errors, truncated
# windows, and inventory failures never report clean.
#
# Usage:
#   KUBECONFIG=… bash scripts/registry-migration-readiness.sh
#   LOKI_ADDR=127.0.0.1:3100 bash scripts/registry-migration-readiness.sh
#   READINESS_FIXTURE_DIR=… bash scripts/registry-migration-readiness.sh   # hermetic
#
# Env:
#   WINDOW_DAYS          required continuous clean days (default 14)
#   COVERAGE_GRACE_HOURS max allowed gap inside the window (default 2)
#   EVIDENCE_START       RFC3339 / epoch override for coverage start (optional)
#   LOKI_ADDR            host:port of reachable Loki (skip port-forward)
#   KUBECONFIG           cluster for inventory + Loki port-forward
#   READINESS_FIXTURE_DIR  if set, read inventory.json + evidence.json instead
#                          of live cluster/Loki (unit tests)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

WINDOW_DAYS="${WINDOW_DAYS:-14}"
COVERAGE_GRACE_HOURS="${COVERAGE_GRACE_HOURS:-2}"
MON_NS=monitoring
WINDOW_SECS=$((WINDOW_DAYS * 86400))

PIDS=()
cleanup() { for p in "${PIDS[@]:-}"; do kill "$p" 2>/dev/null || true; done; }
trap cleanup EXIT

emit() { printf '%s\n' "$*"; }

# Shared fail-closed evaluator. Args: inventory.json evidence.json
evaluate() {
  python3 - "$1" "$2" "$WINDOW_DAYS" "$COVERAGE_GRACE_HOURS" "${EVIDENCE_START:-}" <<'PY'
import json, sys, time
from datetime import datetime

inv_path, ev_path, window_days, grace_hours, evidence_start_override = sys.argv[1:6]
window_secs = int(window_days) * 86400
grace_secs = int(grace_hours) * 3600

def parse_ts(v):
    if v is None or v == "":
        return None
    if isinstance(v, (int, float)):
        return int(v)
    s = str(v).strip()
    if s.isdigit():
        return int(s)
    if s.endswith("Z"):
        s = s[:-1] + "+00:00"
    return int(datetime.fromisoformat(s).timestamp())

inv = json.load(open(inv_path))
ev = json.load(open(ev_path))
now = parse_ts(ev.get("now")) or int(time.time())

if ev.get("source_error"):
    print(f"RESULT status=insufficient_evidence reason=source_error detail={ev['source_error']}")
    sys.exit(1)

migrated = []
for row in inv.get("apps", []):
    ws = (row.get("workspace") or "").strip()
    tomb = str(row.get("tombstone") or "").lower() in ("true", "1", "yes")
    name = row.get("name") or ""
    if not ws:
        continue  # unlabeled — excluded from migration debt
    if not tomb:
        print(f"RESULT status=insufficient_evidence reason=untombstoned_labeled name={name}")
        sys.exit(1)
    image = row.get("image") or ""
    if image:
        want = f"{ws}/{name}"
        if want not in image:
            print(f"RESULT status=insufficient_evidence reason=unscoped_image name={name}")
            sys.exit(1)
    migrated.append(name)

required = set(ev.get("required_services") or ["zot", "static-server"])
live = set(ev.get("live_services") or [])
missing = sorted(required - live)
if missing:
    print("RESULT status=insufficient_evidence reason=collection_dark services=" + ",".join(missing))
    sys.exit(1)

cov_start = parse_ts(evidence_start_override) or parse_ts(ev.get("coverage_start"))
cov_end = parse_ts(ev.get("coverage_end")) or now
if cov_start is None:
    print("RESULT status=insufficient_evidence reason=missing_coverage_start")
    sys.exit(1)

for g in ev.get("gaps") or []:
    gs, ge = parse_ts(g.get("start")), parse_ts(g.get("end"))
    if gs is None or ge is None:
        print("RESULT status=insufficient_evidence reason=malformed_gap")
        sys.exit(1)
    if ge <= cov_start or gs >= cov_end:
        continue
    if (min(ge, cov_end) - max(gs, cov_start)) > grace_secs:
        print(f"RESULT status=insufficient_evidence reason=coverage_gap start={gs} end={ge}")
        sys.exit(1)

span = cov_end - cov_start
if span < window_secs:
    print(f"RESULT status=insufficient_evidence reason=truncated_window have_secs={span} need_secs={window_secs}")
    sys.exit(1)

win_start = cov_end - window_secs
if win_start < cov_start:
    print("RESULT status=insufficient_evidence reason=window_before_coverage")
    sys.exit(1)

reads = []
for r in ev.get("legacy_reads") or []:
    at = parse_ts(r.get("at"))
    name = r.get("name") or ""
    if at is None or not name:
        print("RESULT status=insufficient_evidence reason=malformed_read")
        sys.exit(1)
    if name not in migrated:
        continue
    if win_start <= at <= cov_end:
        reads.append(r)

if reads:
    first = reads[0]
    print(
        "RESULT status=legacy_reads_detected"
        f" name={first.get('name')}"
        f" kind={first.get('kind', 'unknown')}"
        f" at={first.get('at')}"
        f" count={len(reads)}"
    )
    sys.exit(1)

print(
    f"RESULT status=clean"
    f" window_days={window_days}"
    f" coverage_start={cov_start}"
    f" coverage_end={cov_end}"
    f" migrated={len(migrated)}"
)
sys.exit(0)
PY
}

# --- fixture mode ------------------------------------------------------------
if [[ -n "${READINESS_FIXTURE_DIR:-}" ]]; then
  inv_file="$READINESS_FIXTURE_DIR/inventory.json"
  ev_file="$READINESS_FIXTURE_DIR/evidence.json"
  [[ -f "$inv_file" && -f "$ev_file" ]] || {
    emit "RESULT status=insufficient_evidence reason=fixture_missing"
    exit 1
  }
  evaluate "$inv_file" "$ev_file"
  exit $? # never fall through to live cluster/Loki probes
fi

# --- live mode ---------------------------------------------------------------
command -v kubectl >/dev/null || { emit "RESULT status=insufficient_evidence reason=kubectl_missing"; exit 1; }
command -v jq >/dev/null || { emit "RESULT status=insufficient_evidence reason=jq_missing"; exit 1; }
command -v python3 >/dev/null || { emit "RESULT status=insufficient_evidence reason=python3_missing"; exit 1; }
command -v curl >/dev/null || { emit "RESULT status=insufficient_evidence reason=curl_missing"; exit 1; }

# Loki query_range helper. Empty / transport failure => "".
loki_query_range() {
  local q="$1" start_ns="$2" end_ns="$3" limit="${4:-50}" dir="${5:-backward}"
  curl -sfG "http://$LOKI/loki/api/v1/query_range" \
    --data-urlencode "query=$q" \
    --data-urlencode "start=$start_ns" \
    --data-urlencode "end=$end_ns" \
    --data-urlencode "limit=$limit" \
    --data-urlencode "direction=$dir" || true
}

# Fail-closed: empty body or Loki status=error.
loki_query_ok() {
  local out="$1"
  [[ -n "$out" ]] || return 1
  ! echo "$out" | jq -e '.status == "error"' >/dev/null 2>&1
}

if [[ -n "${LOKI_ADDR:-}" ]]; then
  LOKI="$LOKI_ADDR"
else
  LOKI=127.0.0.1:23102
  kubectl -n "$MON_NS" port-forward svc/loki 23102:3100 >/dev/null 2>&1 &
  PIDS+=($!)
  ready=0
  for _ in $(seq 1 30); do
    if curl -sf "http://$LOKI/ready" >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done
  if [[ "$ready" -ne 1 ]]; then
    emit "RESULT status=insufficient_evidence reason=loki_unreachable"
    exit 1
  fi
fi

tmp="$(mktemp -d)"
trap 'cleanup; rm -rf "$tmp"' EXIT

emit "== inventory (labeled Apps) =="
if ! kubectl get apps.app.bex.co -A -o json >"$tmp/apps.json" 2>"$tmp/apps.err"; then
  emit "RESULT status=insufficient_evidence reason=inventory_error"
  exit 1
fi

python3 - "$tmp/apps.json" >"$tmp/inventory.json" <<'PY'
import json, sys
raw = json.load(open(sys.argv[1]))
apps = []
for item in raw.get("items", []):
    md = item.get("metadata") or {}
    labels = md.get("labels") or {}
    ann = md.get("annotations") or {}
    st = item.get("status") or {}
    apps.append({
        "namespace": md.get("namespace", ""),
        "name": md.get("name", ""),
        "workspace": labels.get("app.bex.co/workspace", ""),
        "tombstone": ann.get("app.bex.co/identity-tombstone", ""),
        "image": st.get("image", "") or "",
        "staticPrefix": st.get("staticPrefix", "") or "",
    })
json.dump({"apps": apps}, sys.stdout)
PY

END="$(date -u +%s)"
END_NS="${END}000000000"
PROBE_START_NS="$((END - 6 * 3600))000000000"
WIN_START_NS="$((END - WINDOW_SECS))000000000"

live_services=()
for svc in zot static-server; do
  vals="$(curl -sfG "http://$LOKI/loki/api/v1/label/service/values" \
    --data-urlencode "query={type=\"platform\",service=\"$svc\"}" \
    --data-urlencode "start=$PROBE_START_NS" \
    --data-urlencode "end=$END_NS" || true)"
  if [[ -z "$vals" ]]; then
    emit "RESULT status=insufficient_evidence reason=loki_label_query_failed"
    exit 1
  fi
  if echo "$vals" | jq -e --arg s "$svc" '.data | index($s)' >/dev/null 2>&1; then
    live_services+=("$svc")
    emit "collection live: service=$svc"
  else
    emit "collection dark: service=$svc"
  fi
done

# coverage_start: EVIDENCE_START override, else oldest retained zot line (≤21d).
cov_start_arg="${EVIDENCE_START:-}"
if [[ -z "$cov_start_arg" ]]; then
  oldest="$(loki_query_range '{type="platform",service="zot"}' "$((END - 21 * 86400))000000000" "$END_NS" 1 forward)"
  if [[ -n "${oldest:-}" ]]; then
    ns="$(echo "$oldest" | jq -r '[.data.result[]?.values[]?[0]] | map(tonumber) | min // empty' 2>/dev/null || true)"
    if [[ -n "${ns:-}" ]]; then
      cov_start_arg=$((ns / 1000000000))
    fi
  fi
fi

python3 - "$tmp/inventory.json" >"$tmp/migrated_names.txt" <<'PY'
import json, sys
inv = json.load(open(sys.argv[1]))
for a in inv["apps"]:
    if (a.get("workspace") or "") and str(a.get("tombstone", "")).lower() in ("true", "1", "yes"):
        print(a["name"])
PY

: >"$tmp/reads.jsonl"
if [[ -s "$tmp/migrated_names.txt" ]]; then
  while IFS= read -r name; do
    [[ -n "$name" ]] || continue
    zot_out="$(loki_query_range "{type=\"platform\",service=\"zot\"} |= \"/v2/${name}/\"" "$WIN_START_NS" "$END_NS")"
    if ! loki_query_ok "$zot_out"; then
      emit "RESULT status=insufficient_evidence reason=loki_query_error"
      exit 1
    fi
    echo "$zot_out" | jq -c --arg n "$name" '
      .data.result[]?.values[]? | {name:$n, kind:"registry", at:(.[0]|tonumber/1e9|floor), line:.[1]}
    ' >>"$tmp/reads.jsonl" || true

    st_out="$(loki_query_range "{type=\"platform\",service=\"static-server\"} |~ \"static_legacy_origin_get\" |~ \"${name}\"" "$WIN_START_NS" "$END_NS")"
    if ! loki_query_ok "$st_out"; then
      emit "RESULT status=insufficient_evidence reason=loki_query_error"
      exit 1
    fi
    echo "$st_out" | jq -c --arg n "$name" '
      .data.result[]?.values[]?
      | select(.[1] | test("static_legacy_origin_get"))
      | select(.[1] | (test("app=" + $n) or test("\"app\":\"" + $n + "\"")))
      | {name:$n, kind:"static", at:(.[0]|tonumber/1e9|floor), line:.[1]}
    ' >>"$tmp/reads.jsonl" || true
  done <"$tmp/migrated_names.txt"
fi

python3 - "$tmp/inventory.json" "$tmp/reads.jsonl" "$END" "${cov_start_arg:-}" \
  "$(IFS=,; echo "${live_services[*]-}")" >"$tmp/evidence.json" <<'PY'
import json, sys
from datetime import datetime

inv_path, reads_path, end_s, cov_raw, live_csv = sys.argv[1:6]
end = int(end_s)

def parse_ts(v):
    if v is None or v == "":
        return None
    if isinstance(v, (int, float)):
        return int(v)
    s = str(v).strip()
    if s.isdigit():
        return int(s)
    if s.endswith("Z"):
        s = s[:-1] + "+00:00"
    return int(datetime.fromisoformat(s).timestamp())

cov_start = parse_ts(cov_raw)
live = [s for s in live_csv.split(",") if s]
reads = []
with open(reads_path) as f:
    for line in f:
        line = line.strip()
        if line:
            reads.append(json.loads(line))
ev = {
    "now": end,
    "coverage_start": cov_start,
    "coverage_end": end,
    "required_services": ["zot", "static-server"],
    "live_services": live,
    "gaps": [],
    "legacy_reads": reads,
}
if cov_start is None:
    ev["source_error"] = "missing_coverage_start"
json.dump(ev, sys.stdout)
PY

evaluate "$tmp/inventory.json" "$tmp/evidence.json"
