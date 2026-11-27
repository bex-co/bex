#!/usr/bin/env bash
# Unit tests for render-pricing-drift.sh. Network-free: the two pages are
# file:// URLs to copies of render.com's markdown read on 2026-11-26
# (scripts/fixtures/render-pricing/), so what is under test is the script's own
# logic: the parser reproduces every fixture in
# lego/backend/internal/pricing/testdata/ from the pages, a changed, added or
# removed row fails naming what bex keeps for it, and a page it cannot read
# fails with a message instead of passing. The comparison is the backend's
# TestRenderPricingDrift, so this needs the backend's Go.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/render-pricing-drift.sh"
pages="$repo_root/scripts/fixtures/render-pricing"
testdata="$repo_root/lego/backend/internal/pricing/testdata"

failures=0
check() { # check <name> <expected exit> <expected text> <command...>
  local name="$1" expected_exit="$2" expected_text="$3" actual_exit=0 output
  shift 3
  output="$("$@" 2>&1)" || actual_exit=$?
  if [[ "$actual_exit" != "$expected_exit" ]]; then
    echo "FAIL: $name — exit $actual_exit, want $expected_exit" >&2
    echo "$output" | head -12 >&2
    failures=$((failures + 1))
    return
  fi
  if [[ -n "$expected_text" ]] && ! grep -qF -- "$expected_text" <<<"$output"; then
    echo "FAIL: $name — output missing '$expected_text'" >&2
    echo "$output" | head -12 >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}

sandbox="$(mktemp -d -t bex-pricing-drift-test.XXXXXX)"
trap 'rm -rf "$sandbox"' EXIT
mkdir "$sandbox/tmp"

# run_with <pricing.md> <features.md> [mode args…]: the script against these
# pages, with its temporary directory inside the sandbox so the cleanup can be
# asserted.
run_with() {
  local pricing="$1" features="$2"
  shift 2
  TMPDIR="$sandbox/tmp" RENDER_PRICING_URL="file://$pricing" RENDER_FEATURES_URL="file://$features" "$script" "$@"
}

pricing="$pages/pricing.md"
features="$pages/platform-features-by-plan.md"

# A variant of a page with one sed expression applied.
page_with() { # page_with <name> <page> <sed expression>
  sed -e "$3" "$2" >"$sandbox/$1.md"
  echo "$sandbox/$1.md"
}

check "the pages as read pass" 0 "all 202 rows match the fixtures" run_with "$pricing" "$features"

# parse writes each table exactly as its fixture holds it, header aside: the
# fixtures are the parser's reading of the page, not a hand copy.
check "parse writes the live rows" 0 "" run_with "$pricing" "$features" parse "$sandbox/rows"
for fixture in "$testdata"/render-*.tsv; do
  name="$(basename "$fixture")"
  if ! diff <(grep -v '^#' "$fixture") "$sandbox/rows/$name" >"$sandbox/diff" 2>&1; then
    echo "FAIL: parse output differs from $name:" >&2
    head -8 "$sandbox/diff" >&2
    failures=$((failures + 1))
  fi
done
if [[ "$(find "$sandbox/rows" -name '*.tsv' | wc -l | tr -d ' ')" != "$(find "$testdata" -name 'render-*.tsv' | wc -l | tr -d ' ')" ]]; then
  echo "FAIL: parse writes a table with no fixture, or misses one" >&2
  failures=$((failures + 1))
else
  echo "ok: every fixture is the parser's reading of the page"
fi

# A changed price names the row, the pricing.yaml tier and its new price.
# One run, three assertions: replay prints its output and exits with its code.
replay() { printf '%s\n' "$1"; return "$2"; }
moved="$(page_with moved-price "$pricing" 's#| Less than 1 CPU | $7/month | 512 MB RAM | 0.5c-512mb |#| Less than 1 CPU | $8/month | 512 MB RAM | 0.5c-512mb |#')"
moved_exit=0
moved_output="$(run_with "$moved" "$features" 2>&1)" || moved_exit=$?
check "a changed price fails naming the row" 1 'changed: compute: 0.5c-512mb was' replay "$moved_output" "$moved_exit"
check "a changed price names the pricing.yaml tier" 1 '(tier: pricing.yaml compute starter)' replay "$moved_output" "$moved_exit"
check "a changed price names the tier's new price" 1 'Render $8/month × 0.70 = $5.60 a month, but pricing.yaml compute starter is $4.90' \
  replay "$moved_output" "$moved_exit"

check "a changed Postgres price fails" 1 'changed: postgres: 1c-4g' \
  run_with "$(page_with pg "$pricing" 's#| 1 CPU | $55/month | 4GB RAM#| 1 CPU | $59/month | 4GB RAM#')" "$features"

# A new plan, and a plan Render no longer sells.
check "an added plan fails" 1 'added: compute: 16c-128g ["16 CPU" "$2,000/month" "128 GB RAM" "16c-128g"] (no disposition)' \
  run_with "$(page_with added "$pricing" 's#^| 12 CPU | $1,500/month | 96 GB RAM | 12c-96g |$#&\
| 16 CPU | $2,000/month | 128 GB RAM | 16c-128g |#')" "$features"
check "a removed row fails naming its tier" 1 'removed: keyvalue: 50g' \
  run_with "$(page_with removed "$pricing" '/| 50,000 connections | 50g |/d')" "$features"

check "a changed plan-table row names its limit" 1 'changed: pricing: Instant rollbacks' \
  run_with "$(page_with rollbacks "$pricing" 's#| 15 builds retained |#| 20 builds retained |#')" "$features"
check "a changed workspace plan price names the plan fee" 1 'Render $29/month + compute × 0.70 = $20.30 a month, but pricing.yaml workspace pro is $17.50' \
  run_with "$(page_with plan-fee "$pricing" 's#| Pro | $25/month + compute |#| Pro | $29/month + compute |#')" "$features"
check "a changed sandbox rate names the sandbox rates" 1 'pricing.yaml sandboxUsage usdPerCPUSecond' \
  run_with "$(page_with sandbox "$pricing" 's#| 1 CPU | $0.12 / active CPU hour  |#| 1 CPU | $0.14 / active CPU hour |#')" "$features"
check "a changed FAQ answer fails" 1 'changed: faq: Why was I charged $1 after adding a credit card?' \
  run_with "$(page_with faq "$pricing" 's#The charge is refunded after the transaction completes.#The charge is kept.#')" "$features"
check "a feature Render drops fails" 1 'removed: features: Webhooks' \
  run_with "$pricing" "$(page_with no-webhooks "$features" '/^##### Webhooks$/d')"
check "the Key Value limit disagreement resolving fails" 1 'render.com/pricing now shows 250 connections, where it showed 200' \
  run_with "$(page_with kv "$pricing" 's#| 256 MB storage | 200 connections | 256mb |#| 256 MB storage | 250 connections | 256mb |#')" "$features"

# Pages the parser cannot read fail by name, never as rows that all vanished.
check "a table whose columns changed fails" 1 "Postgres's columns are now" \
  run_with "$(page_with columns "$pricing" 's#^| Compute | Price | RAM | Connections | Plan ID |$#| Compute | Price | RAM | Storage | Plan ID |#')" "$features"
check "a section Render renamed fails" 1 "has no ## Compute > ### Postgres section" \
  run_with "$(page_with renamed "$pricing" 's#^\#\#\# Postgres$#\#\#\# Databases#')" "$features"
printf '<!DOCTYPE html><html><body>Service unavailable</body></html>\n' >"$sandbox/error.md"
check "an HTML error page fails" 1 "does not start with its '# Render Pricing' title" run_with "$sandbox/error.md" "$features"
check "an unreachable page fails" 1 "could not fetch file://$sandbox/absent.md" run_with "$sandbox/absent.md" "$features"
check "a features page with an unknown block fails" 1 "unrecognised feature block" \
  run_with "$pricing" "$(page_with block "$features" 's#^| Feature | Description |$#Some new layout#')"

check "an unknown mode is refused" 2 "usage:" run_with "$pricing" "$features" compare

# Every run above ran with TMPDIR in the sandbox: none may leave its downloads
# behind.
if [[ -n "$(ls -A "$sandbox/tmp")" ]]; then
  echo "FAIL: the script left temporary files behind: $(ls -A "$sandbox/tmp")" >&2
  failures=$((failures + 1))
else
  echo "ok: no temporary files left behind, failures included"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "$failures render-pricing-drift test(s) failed" >&2
  exit 1
fi
echo "all render-pricing-drift tests passed"
