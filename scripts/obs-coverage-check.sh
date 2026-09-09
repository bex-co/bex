#!/usr/bin/env bash
# Alert→panel coverage drift guard (ADR088 §6 "every alert gets a panel";
# .pm/w5/done/054.md). Every alert rule in prometheus.yaml's embedded pack
# (serverFiles.alerting_rules.yml) must have at least one backing metric series
# drawn on at least one committed Grafana dashboard
# (deploy/gitops/base/values/grafana.values.yaml) — an alert only says
# "broken"; the panel answers how bad / since when / trending which way.
#
# Mechanics: a python3 helper extracts each alert's backing series OCCURRENCES
# (series name + the label matchers attached to that selector) from its PromQL
# expr, follows recorded rules to their base series (matching the
# recorded-rule name itself in a dashboard also counts — the builds dashboard
# graphs bex:build_infra_success_ratio:rate1h directly), and matches against
# the series occurrences named by every dashboard JSON `expr`. The full
# alert → matched-dashboard-uid(s) audit map is printed on every run (PASS
# too), so the ADR088 §6 audit is generated, never hand-maintained.
#
# Round-3 review strengthening (item 3, P2):
#   1. CONTEXT series (list below) never establish coverage on their own —
#      they are guards/joins/dimensions (`up`, enable flags, node-role joins),
#      not failure signals, so a panel sharing only those proves nothing about
#      the alert. Coverage needs a NON-context backing series to match.
#   2. Label-matcher compatibility: a panel occurrence covers an alert
#      occurrence only if, for every =/=~ matcher on the alert side, the panel
#      either has no matcher on that key (superset) or one whose value set
#      overlaps. E.g. the crashloop panel (reason="CrashLoopBackOff") does not
#      cover StrandedNodeLocalImages (reason=~"ImagePullBackOff|...").
#
# Round-4 review strengthening (item 2, P2):
#   3. `unless` awareness: an occurrence on the RHS of `unless` is subtracted
#      from the panel, not displayed, so it never establishes coverage; and a
#      surviving LHS occurrence carries the RHS matcher sets as EXCLUSIONS —
#      an alert whose selection provably lies inside a subtracted population
#      is not covered by that panel (the platform cert panel excludes exactly
#      TenantCustomDomainCertNotReady's tenant-cert population).
#
# Fail closed: an empty alert list, an empty dashboard series set, an alert
# whose expr yields no series, an alert backed ONLY by context series, or a
# missing tool is a loud failure — never a silent pass. Called by
# scripts/gitops-validate.sh; exit 0 = PASS. Fixture/test runs may override
# the inputs via OBS_PROM_FILE / OBS_VALUES_FILE.
set -euo pipefail

cd "$(dirname "$0")/.."

# ── Waiver list ──────────────────────────────────────────────────────────────
# Alerts whose backing series is intentionally panel-less (e.g. an alert whose
# only series never exists by design, or one backed only by context series —
# see CONTEXT_SERIES below). Every entry MUST carry a trailing `# reason:`
# comment, and an entry whose series later gains a panel should be removed
# (the audit map flags stale waivers). Intentionally EMPTY today: ADR088 §6
# grants no exceptions.
WAIVED_ALERTS=(
  # "SomeAlertName" # reason: <why this alert can never have a panel>
  #
  # reason: signal is composed entirely of context-class series
  # (kube_node_info, up) + unless-excluded dimensions (taint/role); visual
  # coverage = bex-billing-metering "Egress meter fleet" panel (verbatim expr
  # mirror of the alert's node arithmetic); series matching cannot express
  # this without weakening the context rule (round-4 addendum).
  "EgressMeterTargetMissing"
)

# ── Context series ───────────────────────────────────────────────────────────
# Series that appear in alert exprs as guards, joins, or bounded dimensions —
# never as the failure signal — so a dashboard drawing one of them on an
# unrelated tile must NOT count as the alert's panel (round-3 item 3: deleting
# the only outbox-age panel used to stay green because BillingExportBacklog
# still shared bex_billing_enabled with another billing tile). Each entry
# carries the reason it can never establish coverage on its own.
CONTEXT_SERIES=(
  "up"                        # scrape-target liveness guard/absence clause, shared by most jobs
  "bex_billing_enabled"       # feature gate ANDed into every billing alert, not a signal
  "bex_push_enabled"          # push-transport-configured gate ANDed into PushDeliveryStale, not a signal
  "kube_node_role"            # node-role join (control-plane vs worker), not a signal
  "kube_node_info"            # node-inventory join used for fleet counts, not a signal
  "kube_cronjob_spec_suspend" # suspension guard ANDed into BackupCronJobStale, not a signal
)

for tool in yq python3; do
  command -v "$tool" >/dev/null 2>&1 || {
    echo "FAIL: obs coverage check requires $tool — refusing to pass without running (ADR088 §6)" >&2
    exit 2
  }
done

PROM_FILE="${OBS_PROM_FILE:-deploy/gitops/base/prometheus.yaml}"
GRAFANA_FILE="${OBS_VALUES_FILE:-deploy/gitops/base/values/grafana.values.yaml}"

tmp="$(mktemp -d)"
# Paths originate from mktemp and are always task-scoped.
trap 'rm -rf "$tmp"' EXIT

# helm `values:` is a block-scalar string — from_yaml re-parses it in-process
# (same extraction the promtool step in gitops-validate.sh uses).
yq -o=json '.spec.source.helm.values | from_yaml | .serverFiles."alerting_rules.yml".groups' \
  "$PROM_FILE" >"$tmp/rules.json" || {
  echo "FAIL: could not extract alerting rule groups from $PROM_FILE" >&2
  exit 2
}
yq -o=json '.dashboards' "$GRAFANA_FILE" >"$tmp/dashboards.json" || {
  echo "FAIL: could not extract dashboards from $GRAFANA_FILE" >&2
  exit 2
}

python3 - "$tmp/rules.json" "$tmp/dashboards.json" \
  --context "${CONTEXT_SERIES[@]}" \
  --waived ${WAIVED_ALERTS[@]+"${WAIVED_ALERTS[@]}"} <<'PYEOF'
import json
import re
import sys

rules_path, dashboards_path = sys.argv[1], sys.argv[2]
context, waived = set(), set()
bucket = None
for arg in sys.argv[3:]:
    if arg == "--context":
        bucket = context
    elif arg == "--waived":
        bucket = waived
    else:
        bucket.add(arg)

# ── PromQL series-occurrence extraction (pragmatic, per .pm/w5/done/054.md) ──
# An occurrence is (series name, [label matchers attached to that selector]).
# A series name is any identifier that is not a function call (followed by
# `(`), not a PromQL keyword/modifier, and not a grouping label — plus the
# literal value of any __name__ matcher (absent()-style bare selectors still
# name their series that way, e.g. {__name__=~"(vault|bao)_core_unsealed"}).
KEYWORDS = {
    "and", "or", "unless", "by", "on", "ignoring", "without",
    "group_left", "group_right", "bool", "offset", "atan2",
    "start", "end", "inf", "nan",
}
QUOTED = re.compile(r'"(?:[^"\\]|\\.)*"')
GROUPING = re.compile(r'\b(?:by|on|ignoring|without|group_left|group_right)\s*\([^)]*\)')
LABEL_BLOCK = re.compile(r'\{(?:[^{}"]|"(?:[^"\\]|\\.)*")*\}')
MATCHER = re.compile(r'([a-zA-Z_][a-zA-Z0-9_]*)\s*(=~|!=|!~|=)\s*"((?:[^"\\]|\\.)*)"')
IDENT = re.compile(r'(?<![A-Za-z0-9_:])[A-Za-z_][A-Za-z0-9_:]*')
REGEX_META = re.compile(r'[.^$*+?()\[\]{}\\|]')
UNLESS = re.compile(r'(?<![A-Za-z0-9_:])unless(?![A-Za-z0-9_:])')
SET_OP = re.compile(r'(?<![A-Za-z0-9_:])(?:and|or|unless)(?![A-Za-z0-9_:])')


def blank(text, start, end):
    return text[:start] + " " * (end - start) + text[end:]


def unless_rhs_spans(scan):
    # Round-4 item 2: a series occurrence on the RIGHT-hand side of `unless`
    # is SUBTRACTED from the result, not displayed, so it must never establish
    # coverage. Each RHS span runs from after the keyword to the close of the
    # enclosing paren group or the next set operator at the same nesting level
    # (PromQL set ops are left-associative and equal-precedence, so in
    # `A unless B or C` the RHS is just B). Pragmatic limits: this is a
    # token-level split, not a PromQL parse — operator precedence inside the
    # RHS and vector-matching keywords are not modelled beyond this rule; the
    # committed rules/dashboards only use the simple shapes it handles.
    spans = []
    for m in UNLESS.finditer(scan):
        i, depth, end = m.end(), 0, len(scan)
        while i < len(scan):
            char = scan[i]
            if char == "(":
                depth += 1
            elif char == ")":
                if depth == 0:
                    end = i
                    break
                depth -= 1
            elif depth == 0 and SET_OP.match(scan, i):
                end = i
                break
            i += 1
        spans.append((m.end(), end))
    return spans


def occurrences_of(expr, subtract_unless=False):
    # -> [(name, matchers, exclusions)]. Length-preserving masking keeps spans
    # aligned with the original text. (Pragmatic limit: a grouping-clause-
    # shaped substring INSIDE a label value would be masked too; no rule or
    # panel writes one.) With subtract_unless (the DASHBOARD side, round-4
    # item 2): occurrences inside an `unless` RHS are dropped — subtracted,
    # not displayed — and their matcher sets attach as EXCLUSIONS to every
    # same-name surviving occurrence, so a panel like
    # `X{a} unless X{a, tenant-pop}` does not cover an alert selecting exactly
    # the subtracted tenant population. The ALERT side keeps RHS occurrences:
    # backing stays "every series the rule's logic reads" (round-1 contract),
    # and only the panel side must prove it actually DISPLAYS one of them.
    masked = expr
    for m in GROUPING.finditer(expr):
        masked = blank(masked, m.start(), m.end())
    blocks = []  # (start, [(key, op, value), ...])
    scan = masked
    for m in LABEL_BLOCK.finditer(masked):
        blocks.append((m.start(), MATCHER.findall(m.group(0))))
        scan = blank(scan, m.start(), m.end())
    for m in QUOTED.finditer(scan):
        scan = blank(scan, m.start(), m.end())
    block_at = dict(blocks)
    attached = set()
    raw = []  # (position, name, matchers)
    for m in IDENT.finditer(scan):
        tok = m.group(0)
        # Look ahead in MASKED (label blocks intact), not in scan where the
        # block is already blanked to spaces — skipping whitespace there would
        # walk over the entire block and silently drop its matchers, turning
        # every occurrence into a match-anything superset.
        j = m.end()
        while j < len(masked) and masked[j] in " \t\r\n":
            j += 1
        if j < len(masked) and masked[j] == "(":
            continue  # function call, not a series
        if tok in KEYWORDS:
            continue
        matchers = []
        if j in block_at:
            attached.add(j)
            matchers = [t for t in block_at[j] if t[0] != "__name__"]
        raw.append((m.start(), tok, matchers))
    for start, matchers in blocks:  # bare selectors: {__name__=~"...", ...}
        if start in attached:
            continue
        names = [v for k, op, v in matchers if k == "__name__" and op in ("=", "=~")]
        if names:
            raw.append((start, names[0],
                        [t for t in matchers if t[0] != "__name__"]))
    spans = unless_rhs_spans(scan) if subtract_unless else []
    kept, subtracted = [], []
    for pos, name, matchers in raw:
        target = subtracted if any(s <= pos < e for s, e in spans) else kept
        target.append((name, matchers))
    return [(name, matchers,
             [s_matchers for s_name, s_matchers in subtracted
              if names_match(name, s_name)])
            for name, matchers in kept]


def safe_fullmatch(pattern, literal):
    try:
        return re.fullmatch(pattern, literal) is not None
    except re.error:
        return False


def names_match(a, b):
    if a == b:
        return True
    # One side may be a __name__ regex (e.g. "(vault|bao)_core_unsealed")
    # matching the other side's bare series name.
    for pat, lit in ((a, b), (b, a)):
        if REGEX_META.search(pat) and safe_fullmatch(pat, lit):
            return True
    return False


def literal_alternatives(regex):
    # "a|b|c" -> ["a", "b", "c"] when every alternative is metacharacter-free.
    alts = regex.split("|")
    if any(REGEX_META.search(a) for a in alts):
        return None
    return alts


def matcher_overlap(a_op, a_val, p_op, p_val):
    if a_op == "=" and p_op == "=":
        return a_val == p_val
    if a_op == "=" and p_op == "=~":
        return safe_fullmatch(p_val, a_val)
    if a_op == "=~" and p_op == "=":
        return safe_fullmatch(a_val, p_val)
    # regex vs regex: identical patterns overlap; an alternation of literals
    # on either side is tested alternative-by-alternative; two opaque regexes
    # are conservatively treated as overlapping (pragmatic limit — a true
    # intersection test needs automata this guard does not warrant).
    if a_val == p_val:
        return True
    for pattern, other in ((a_val, p_val), (p_val, a_val)):
        lits = literal_alternatives(other)
        if lits is not None:
            return any(safe_fullmatch(pattern, lit) for lit in lits)
    return True


def value_subset(a_op, a_val, e_op, e_val):
    # Is the alert matcher's value set provably a subset of the exclusion's?
    if a_op == "=" and e_op == "=":
        return a_val == e_val
    if a_op == "=" and e_op == "=~":
        return safe_fullmatch(e_val, a_val)
    if a_op == "=~" and e_op == "=~":
        if a_val == e_val:
            return True
        lits = literal_alternatives(a_val)
        return lits is not None and all(safe_fullmatch(e_val, lit) for lit in lits)
    if a_op == "=~" and e_op == "=":
        return literal_alternatives(a_val) == [e_val]
    return False


def selection_contained(alert_labels, excl_labels):
    # True when the alert's selection provably lies INSIDE an `unless`-
    # subtracted population, i.e. the panel removes from display exactly what
    # the alert fires on. Positives only, per key: every =/=~ matcher on the
    # exclusion side needs an alert matcher on the same key whose value set is
    # a subset; an unconstrained alert key is wider than the exclusion, so not
    # contained. Negative matchers are ignored on both sides — that models the
    # exclusion as LARGER than it is, so an uncertain case suppresses coverage
    # (fail-closed: worst case a truly-shown series reads uncovered and gets a
    # dedicated panel, never the reverse).
    a_pos = {}
    for key, op, val in alert_labels:
        if op in ("=", "=~"):
            a_pos.setdefault(key, []).append((op, val))
    for key, e_op, e_val in excl_labels:
        if e_op in ("!=", "!~"):
            continue
        candidates = a_pos.get(key)
        if not candidates:
            return False
        if not any(value_subset(a_op, a_val, e_op, e_val)
                   for a_op, a_val in candidates):
            return False
    return True


def matchers_compatible(alert_labels, panel_labels):
    # A panel occurrence covers an alert occurrence only if every =/=~ matcher
    # on the ALERT side is satisfied: the panel has no matcher on that key
    # (superset — a broader panel still draws the alert's series), or a
    # positive matcher whose value set overlaps the alert's. A panel-side
    # negative matcher that excludes exactly what the alert selects also
    # rejects. Negative matchers on the alert side are skipped (pragmatic:
    # they only widen the alert's selection).
    positive, negative = {}, {}
    for key, op, val in panel_labels:
        (positive if op in ("=", "=~") else negative).setdefault(key, []).append((op, val))
    for key, a_op, a_val in alert_labels:
        if a_op in ("!=", "!~"):
            continue
        pos = positive.get(key, [])
        if pos:
            if not any(matcher_overlap(a_op, a_val, p_op, p_val) for p_op, p_val in pos):
                return False
            continue
        for p_op, p_val in negative.get(key, []):
            if a_op == "=" and ((p_op == "!=" and p_val == a_val)
                                or (p_op == "!~" and safe_fullmatch(p_val, a_val))):
                return False
            if a_op == "=~" and p_op == "!~" and p_val == a_val:
                return False
    return True


# ── Alerts + recorded rules (fail closed on an empty/undecodable pack) ───────
try:
    with open(rules_path, encoding="utf-8") as f:
        groups = json.load(f)
except (OSError, json.JSONDecodeError) as e:
    sys.exit(f"FAIL: cannot parse extracted alerting rules: {e}")
if not isinstance(groups, list) or not groups:
    sys.exit("FAIL: extracted alerting rule pack is empty — parse error, refusing to pass")

alerts = []   # (name, expr) in file order
records = {}  # recorded-rule name -> its base occurrences
for group in groups:
    for rule in group.get("rules") or []:
        if "record" in rule:
            records[rule["record"]] = occurrences_of(rule.get("expr", ""))
        elif "alert" in rule:
            alerts.append((rule["alert"], rule.get("expr", "")))
if not alerts:
    sys.exit("FAIL: no alert rules found in the extracted pack — parse error, refusing to pass")

# ── Dashboard occurrences, per uid (Prometheus targets only; Loki = logs) ────
try:
    with open(dashboards_path, encoding="utf-8") as f:
        providers = json.load(f)
except (OSError, json.JSONDecodeError) as e:
    sys.exit(f"FAIL: cannot parse extracted dashboards: {e}")
if not isinstance(providers, dict) or not providers:
    sys.exit("FAIL: no dashboards found in grafana values — parse error, refusing to pass")


def collect_exprs(node, ds_type, exprs):
    if isinstance(node, dict):
        ds = node.get("datasource")
        if isinstance(ds, dict) and isinstance(ds.get("type"), str):
            ds_type = ds["type"]
        expr = node.get("expr")
        if isinstance(expr, str) and ds_type in (None, "prometheus"):
            exprs.append(expr)
        for key, value in node.items():
            if key != "expr":
                collect_exprs(value, ds_type, exprs)
    elif isinstance(node, list):
        for value in node:
            collect_exprs(value, ds_type, exprs)


dash_occurrences = {}  # uid -> [(name, matchers, exclusions), ...]
for dashes in providers.values():
    for name, spec in (dashes or {}).items():
        raw = (spec or {}).get("json")
        if not isinstance(raw, str):
            sys.exit(f"FAIL: dashboard {name} has no json document — parse error, refusing to pass")
        try:
            dash = json.loads(raw)
        except json.JSONDecodeError as e:
            sys.exit(f"FAIL: dashboard {name} json does not parse: {e}")
        exprs = []
        collect_exprs(dash, None, exprs)
        occurrences = dash_occurrences.setdefault(dash.get("uid", name), [])
        for expr in exprs:
            occurrences.extend(occurrences_of(expr, subtract_unless=True))

all_dash_names = {name for occs in dash_occurrences.values() for name, _, _ in occs}
if not all_dash_names:
    sys.exit("FAIL: dashboard series set is empty — parse error, refusing to pass")

# ── Coverage + the generated ADR088 §6 audit map ─────────────────────────────
print(f"alert → dashboard coverage map ({len(alerts)} alerts, "
      f"{len(dash_occurrences)} dashboards, {len(all_dash_names)} dashboard series):")
uncovered, parse_errors, stale_waivers = [], [], []
backed_context = set()  # context entries seen backing some alert (typo guard)
for alert, expr in alerts:
    backing = []
    for occ in occurrences_of(expr):
        backing.append(occ)
        if occ[0] in records:
            backing.extend(records[occ[0]])  # recorded rule: name AND base series count
    if not backing:
        parse_errors.append(alert)
        print(f"  ERROR     {alert:<44} -> expr yielded no series (extractor parse failure)")
        continue
    backed_context |= {name for name, _, _ in backing if name in context}
    # Alert-side exclusion lists are always empty (subtract_unless is a
    # dashboard-side rule) and discarded here.
    signal = [(name, labels) for name, labels, _ in backing if name not in context]
    backing_names = sorted({name for name, _, _ in backing})
    if not signal:
        if alert in waived:
            print(f"  waived    {alert:<44} -> backed only by context series "
                  f"({', '.join(backing_names)}; see waiver comment)")
        else:
            uncovered.append(alert)
            print(f"  UNCOVERED {alert:<44} -> backed ONLY by context series "
                  f"({', '.join(backing_names)}) — needs a panelable signal series "
                  f"or a reasoned waiver")
        continue
    matched = {}  # uid -> matched non-context series (matcher-compatible, not unless-subtracted)
    for uid, panel_occs in sorted(dash_occurrences.items()):
        via = sorted({a_name for a_name, a_labels in signal
                      if any(names_match(a_name, p_name)
                             and matchers_compatible(a_labels, p_labels)
                             and not any(selection_contained(a_labels, excl)
                                         for excl in p_exclusions)
                             for p_name, p_labels, p_exclusions in panel_occs)})
        if via:
            matched[uid] = via
    if matched:
        uids = ", ".join(sorted(matched))
        via = ", ".join(sorted(set().union(*(set(v) for v in matched.values()))))
        note = " [stale waiver — remove it]" if alert in waived else ""
        if alert in waived:
            stale_waivers.append(alert)
        print(f"  covered   {alert:<44} -> {uids} (via {via}){note}")
    elif alert in waived:
        print(f"  waived    {alert:<44} -> no panel by design (see waiver comment; "
              f"backing: {', '.join(backing_names)})")
    else:
        uncovered.append(alert)
        print(f"  UNCOVERED {alert:<44} -> no matcher-compatible dashboard occurrence of: "
              f"{', '.join(sorted({name for name, _ in signal}))}")
for name in sorted(waived - {a for a, _ in alerts}):
    print(f"  ERROR     {name:<44} -> waiver names no existing alert (typo or delivered panel?)")
for name in sorted(context - backed_context):
    print(f"note: context series {name!r} backs no alert — stale entry or typo in CONTEXT_SERIES")

if parse_errors or waived - {a for a, _ in alerts}:
    sys.exit("FAIL: obs coverage check could not account for every alert (see ERROR lines above)")
if uncovered:
    sys.exit("FAIL: alert(s) without a dashboard panel (ADR088 §6): "
             + ", ".join(uncovered)
             + " — add a panel on the backing series, or waiver with a reason in scripts/obs-coverage-check.sh")
if stale_waivers:
    print("note: stale waiver(s) now covered — remove from WAIVED_ALERTS: "
          + ", ".join(stale_waivers))
print(f"PASS: every alert's backing series appears on a dashboard "
      f"({len(alerts) - len(waived)} covered, {len(waived)} waived)")
PYEOF
