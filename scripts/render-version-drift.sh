#!/usr/bin/env bash
# Check, but never update, that bex's dated default versions match Render's
# per-language version pages (w3/m637).
#
# Render moves a runtime's default for services created from a day on, and
# sometimes back-dates a row, without a changelog entry: it did so on
# 2026-09-09, 09-17, 10-12, 11-03, 11-19 and 11-20, and render-changelog-drift.sh
# saw none of them. bex found the 11-03 moves ten days late (w4/620) and the
# 11-19 Ruby move by chance (w9/138), and services created on bex from 10-12 to
# 11-14 got a uv Render never gave them (w8/328). So this job reads the pages
# themselves, every day.
#
#   render-version-drift.sh [check]   compare the pages with bex's tables
#   render-version-drift.sh parse     print the pages as JSON, without comparing
#
# Two halves, because each table lives on its own side. The parser,
# scripts/lib/render_version_pages.py, takes the page list from Render's
# llms.txt, reads each page's markdown copy and fails, naming the page, on an
# eighth version page, a page that is gone, or markup it does not recognise.
# The comparison is TestRenderVersionDrift in lego/operator/internal/build,
# which reads bex's real Go tables: every row from the one in force on
# 2026-06-05, when bex's first service could be created, must match, and it
# names the page, the row and the bex table that does not.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
parser="$repo_root/scripts/lib/render_version_pages.py"
docs_url="${RENDER_DOCS_URL:-https://render.com/docs}"

case "${1:-check}" in
  parse)
    exec python3 -I "$parser" "$docs_url"
    ;;
  check) ;;
  *)
    echo "usage: $0 [check|parse]" >&2
    exit 2
    ;;
esac

# Under $TMPDIR on every platform (macOS mktemp -t ignores it), so the test can
# assert the cleanup.
work="$(mktemp -d "${TMPDIR:-/tmp}/bex-render-versions.XXXXXX")"
trap 'rm -rf "$work"' EXIT

python3 -I "$parser" "$docs_url" >"$work/pages.json"

# GOWORK=off: the operator module on its own go.mod, as operator-test.yml and
# the image build it. -v, so a test that was renamed or skipped cannot pass as
# a comparison that ran.
status=0
output="$(cd "$repo_root/lego/operator" && GOWORK=off BEX_RENDER_VERSION_PAGES="$work/pages.json" \
  go test -count=1 -run '^TestRenderVersionDrift$' -v ./internal/build/ 2>&1)" || status=$?
if [[ "$status" -eq 0 ]] && grep -qF -- '--- PASS: TestRenderVersionDrift (' <<<"$output"; then
  echo "Render version pages: every default from 2026-06-05 on matches bex's tables ($(grep -c '"url"' "$work/pages.json") pages)"
  exit 0
fi
differences="$(sed -n 's/^ *render_version_drift_test\.go:[0-9]*: \(render\.com\/docs\/.*\)$/\1/p' <<<"$output")"
if [[ -z "$differences" ]]; then
  echo "::error title=Render version comparison did not run::TestRenderVersionDrift did not pass and named no difference" >&2
else
  while IFS= read -r line; do
    echo "::error title=Render default version drift::$line" >&2
  done <<<"$differences"
fi
printf '%s\n' "$output" >&2
exit 1
