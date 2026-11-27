#!/usr/bin/env bash
# Check, but never update, that Render's pricing pages still show every row bex
# captured from them (w4/m690).
#
# bex prices compute, Postgres, Key Value, cron jobs, sandboxes and its plan
# fees at Render's prices less 30% (ADR030 §2), and its plan limits and gates
# follow Render's plan tables (ADR030 §9). The tests in
# lego/backend/internal/pricing hold bex to fixtures captured by hand, not to
# the page, so a change Render made waited for someone to re-read it. Render
# changed its plans on 2026-04-23 and 2026-08-26, and runs a Starter price
# experiment in its dashboard (enable_starter_price). So every week this reads
# render.com/pricing.md and render.com/docs/platform-features-by-plan.md, and
# fails on any row of the plan comparison table, the features page, the seven
# price tables or the FAQ that changed, appeared or vanished, naming what bex
# keeps for it (the pricing.yaml tier, the gate, the limit) and, for a moved
# price, the price the tier needs.
#
#   render-pricing-drift.sh [check]      compare the pages with the fixtures
#   render-pricing-drift.sh parse <dir>  write the pages' rows in the fixtures' form
#
# Two halves. scripts/lib/render_pricing_page.py reads the markdown into the
# fixtures' form and fails, naming the table, on a section, table or column it
# does not recognise. TestRenderPricingDrift in lego/backend/internal/pricing
# compares those rows with the fixtures and holds each to its disposition.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
parser="$repo_root/scripts/lib/render_pricing_page.py"
pricing_url="${RENDER_PRICING_URL:-https://render.com/pricing.md}"
features_url="${RENDER_FEATURES_URL:-https://render.com/docs/platform-features-by-plan.md}"
fixtures="lego/backend/internal/pricing/testdata"

usage() {
  echo "usage: $0 [check|parse <dir>]" >&2
  exit 2
}
mode="${1:-check}"
case "$mode" in
  check) [[ $# -le 1 ]] || usage ;;
  parse) [[ $# -eq 2 ]] || usage ;;
  *) usage ;;
esac

# Under $TMPDIR on every platform (macOS mktemp -t ignores it), so the test can
# assert the cleanup.
work="$(mktemp -d "${TMPDIR:-/tmp}/bex-render-pricing.XXXXXX")"
trap 'rm -rf "$work"' EXIT

fetch() { # fetch <url> <dest>
  if ! curl --fail --location --retry 2 --silent --show-error --max-time 60 "$1" --output "$2"; then
    echo "::error title=Render pricing page unreachable::could not fetch $1" >&2
    exit 1
  fi
}
fetch "$pricing_url" "$work/pricing.md"
fetch "$features_url" "$work/features.md"

if [[ "$mode" == parse ]]; then
  python3 -I "$parser" "$work/pricing.md" "$work/features.md" "$2"
  exit 0
fi
python3 -I "$parser" "$work/pricing.md" "$work/features.md" "$work/rows"

# GOWORK=off: the backend module on its own go.mod, as backend-test.yml and the
# image build it. -v, so a test that was renamed or skipped cannot pass as a
# comparison that ran.
status=0
output="$(cd "$repo_root/lego/backend" && GOWORK=off BEX_RENDER_PRICING_DIR="$work/rows" \
  go test -count=1 -run '^TestRenderPricingDrift$' -v ./internal/pricing/ 2>&1)" || status=$?
if [[ "$status" -eq 0 ]] && grep -qF -- '--- PASS: TestRenderPricingDrift (' <<<"$output"; then
  echo "Render pricing: all $(cat "$work"/rows/*.tsv | wc -l | tr -d ' ') rows match the fixtures in $fixtures"
  exit 0
fi
differences="$(sed -n 's/^ *[a-z_]*_test\.go:[0-9]*: //p' <<<"$output")"
if [[ -z "$differences" ]]; then
  echo "::error title=Render pricing comparison did not run::TestRenderPricingDrift did not pass and named no difference" >&2
else
  while IFS= read -r line; do
    echo "::error title=Render pricing drift::$line" >&2
  done <<<"$differences"
  cat >&2 <<EOF

Action: take Render's change in, then make bex follow it.
  1. scripts/render-pricing-drift.sh parse <dir> writes the live rows; copy each
     changed table's rows over its fixture's in $fixtures and update the read
     date in its header.
  2. cd lego/backend && go test ./internal/pricing/ names every disposition the
     change moves: a pricing.yaml tier (and its Stripe Price,
     scripts/stripe-billing-setup.py), a gate, a limit. An added row needs a
     disposition (a tier, a gap with the board item that owns it, a non-goal);
     a removed row's disposition is deleted.
  3. Record the change in docs/ADR030-pricing.md §9. This job never writes the
     fixtures.
EOF
fi
printf '%s\n' "$output" >&2
exit 1
