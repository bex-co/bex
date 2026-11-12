#!/usr/bin/env bash
# Alert → scrape keep-list guard (w6/122). An alert whose backing series a
# scrape job's metric_relabel_configs keep-list drops evaluates an absent
# series and never fires. Every other check passes in that state: promtool
# tests feed the series themselves, and obs-coverage-check.sh only matches
# alerts to dashboards. It happened three times: w2/m72's build alerts on the
# operator job, the webhooks alerts on bex-api, and w6/m334's
# UsageHourNearlyUnmeterable, whose bex_usage_* gauge was never stored.
#
# For every bex_* series an alert reads (directly, or through a recording
# rule), one of these must hold:
#   - a scrape job keeps it: a keep rule on [__name__] whose regex fully
#     matches the name (Prometheus anchors relabel regexes);
#   - its prefix is declared below as produced by a job that keeps every
#     name (no keep rule on [__name__]), and that job exists.
# A new bex_ family therefore fails closed until it is added to its job's
# keep-list, or declared here with the job that emits it.
#
#   bash scripts/obs-scrape-keep-check.sh
#   OBS_RULES_FILE=… OBS_PROMETHEUS_FILE=… bash scripts/obs-scrape-keep-check.sh
set -euo pipefail

cd "$(dirname "$0")/.."

# prefix=job: bex_* families emitted by a job that keeps every metric name.
KEEP_ALL_SOURCES=(
  "bex_ssh_=bex-ssh-gateway"   # the gateway's own registry, scraped whole
  "bex_zot_=alloy"             # log-shipper's Zot metrics branch (log-shipper.yaml)
  "bex_cnpg_=kube-state-metrics" # customResourceState metricNamePrefix (prometheus.yaml)
)

RULES_FILE="${OBS_RULES_FILE:-deploy/gitops/base/rules/alerting_rules.yml}"
PROM_FILE="${OBS_PROMETHEUS_FILE:-deploy/gitops/base/prometheus.yaml}"

for tool in yq python3; do
  command -v "$tool" >/dev/null 2>&1 || {
    echo "FAIL: scrape keep check requires $tool — refusing to pass without running" >&2
    exit 2
  }
done

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
yq -o=json '.groups' "$RULES_FILE" >"$tmp/rules.json" || {
  echo "FAIL: could not read rule groups from $RULES_FILE" >&2
  exit 2
}

python3 - "$tmp/rules.json" "$PROM_FILE" "${KEEP_ALL_SOURCES[@]}" <<'PYEOF'
import json
import re
import sys

rules_path, prom_path, *declared = sys.argv[1:]
groups = json.load(open(rules_path))
alerts, records = {}, {}
for group in groups or []:
    for rule in group.get("rules") or []:
        if "alert" in rule:
            alerts[rule["alert"]] = rule["expr"]
        elif "record" in rule:
            records[rule["record"]] = rule["expr"]
if not alerts:
    sys.exit("FAIL: no alert rules parsed — refusing to pass")

SERIES = re.compile(r"(?<![A-Za-z0-9_:])bex_[a-z0-9_]+(?![A-Za-z0-9_:])")
RECORD = re.compile(r"(?<![A-Za-z0-9_:])[a-z][a-z0-9_]*:[a-z0-9_:]+")
QUOTED = re.compile(r'"(?:[^"\\]|\\.)*"')


def series_of(expr, seen=()):
    # Label values in quotes are not series names.
    bare = QUOTED.sub('""', expr)
    names = set(SERIES.findall(bare))
    for rec in RECORD.findall(bare):
        if rec in records and rec not in seen:
            names |= series_of(records[rec], seen + (rec,))
    return names


# Scrape jobs: each "- job_name:" block up to the next one, with every keep
# rule whose source_labels is exactly [__name__].
text = open(prom_path).read()
blocks = re.split(r"\n\s*- job_name:\s*", text)[1:]
if not blocks:
    sys.exit(f"FAIL: no scrape jobs found in {prom_path} — refusing to pass")
KEEP = re.compile(
    r"source_labels:\s*\[__name__\]\s*\n(?:\s*#[^\n]*\n)*\s*regex:\s*(?P<re>[^\n]+?)\s*\n(?:\s*#[^\n]*\n)*\s*action:\s*keep"
)
jobs = {}
for block in blocks:
    name = block.split("\n", 1)[0].strip().strip("'\"")
    jobs[name] = [m.group("re").strip("'\"") for m in KEEP.finditer(block)]

keep_all = {}
problems = []
for entry in declared:
    prefix, _, job = entry.partition("=")
    if job not in jobs:
        problems.append(f"declared source {prefix}* names no scrape job {job!r}")
    elif jobs[job]:
        problems.append(f"declared source {prefix}* names {job!r}, which has a __name__ keep-list; add the family there instead")
    keep_all[prefix] = job

used = {}
for alert, expr in alerts.items():
    for name in series_of(expr):
        used.setdefault(name, []).append(alert)

print(f"alert → scrape keep map ({len(alerts)} alerts, {len(used)} bex_* series, {len(jobs)} jobs):")
for name in sorted(used):
    kept_by = sorted(job for job, regexes in jobs.items() if any(re.fullmatch(r, name) for r in regexes))
    if kept_by:
        print(f"  kept      {name:<56} -> {', '.join(kept_by)}")
        continue
    prefix = next((p for p in keep_all if name.startswith(p)), None)
    if prefix:
        print(f"  kept      {name:<56} -> {keep_all[prefix]} (keeps every name)")
        continue
    problems.append(f"{name} (read by {', '.join(sorted(used[name]))}) is kept by no scrape job: "
                    "add it to its job's __name__ keep-list in prometheus.yaml, or declare its "
                    "keep-all source in scripts/obs-scrape-keep-check.sh")
    print(f"  DROPPED   {name:<56} -> no keep-list matches")

if problems:
    print("\n".join(f"FAIL: {p}" for p in problems), file=sys.stderr)
    sys.exit(1)
print(f"PASS: every bex_* series an alert reads is kept by its scrape job ({len(used)} series)")
PYEOF
