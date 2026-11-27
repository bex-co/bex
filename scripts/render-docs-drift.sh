#!/usr/bin/env bash
# Check, but never update, that every page in Render's docs index has a row in
# docs/render-artifacts/docs-pages.md, and that no page changed since its row
# was written (w4/m688).
#
# The changelog (render-changelog-drift.sh) sees what Render announces and the
# dashboard bundle (render-dashboard-ops.sh) what its dashboard can do. Neither
# sees Render rewriting a docs page, which is where it states its limits,
# defaults and behaviours: it moved runtime defaults on its version pages six
# times between 2026-09-09 and 11-20 without a changelog entry, and a
# brainstorm's reading of three uncited pages found two unrecorded gaps (the
# Cloudflare headers, w4/m687; the plan-fee waiver, w4/m689). So this job reads
# every page llms.txt lists and fails, naming the page and its row's pointer,
# when one is new, gone or rewritten.
#
#   render-docs-drift.sh [check]          compare every page with its row
#   render-docs-drift.sh hashes [page…]   print "<page><TAB><hash>" per page
#   render-docs-drift.sh validate         check the ledger's rows, offline
#
# RENDER_DOCS_CACHE names a directory that keeps each page's normalized text
# under its hash; with it, a changed page's failure quotes the paragraphs that
# changed since the fetch its row recorded. The page text never enters git: the
# ledger holds a hash per page. The parsing, hashing and comparison live in
# scripts/lib/render_docs_pages.py.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
lib="$repo_root/scripts/lib/render_docs_pages.py"
docs_url="${RENDER_DOCS_URL:-https://render.com/docs}"
ledger="${RENDER_DOCS_LEDGER:-$repo_root/docs/render-artifacts/docs-pages.md}"

mode="${1:-check}"
if [[ "$mode" == check || "$mode" == validate ]] && [[ ! -f "$ledger" ]]; then
  echo "::error title=Render docs ledger missing::$ledger does not exist" >&2
  exit 1
fi
case "$mode" in
  check)
    exec python3 -I "$lib" check "$docs_url" "$ledger" ${RENDER_DOCS_CACHE:+"$RENDER_DOCS_CACHE"}
    ;;
  validate)
    exec python3 -I "$lib" validate "$ledger"
    ;;
  hashes)
    shift
    exec python3 -I "$lib" hashes "$docs_url" "$@"
    ;;
  *)
    echo "usage: $0 [check | hashes [page…] | validate]" >&2
    exit 2
    ;;
esac
