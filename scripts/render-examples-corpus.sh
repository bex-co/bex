#!/usr/bin/env bash
# Fetch every render.yaml in github.com/render-examples into a directory, for
# lego/backend/internal/apps' opt-in TestRenderExamplesCorpus (w5/m143). The
# files are third-party, so they are fetched, never vendored. The weekly
# render-examples job in .github/workflows/render-schema-drift.yml runs both
# (w33/001).
#
#   bash scripts/render-examples-corpus.sh /tmp/render-examples
#   BEX_RENDER_EXAMPLES_DIR=/tmp/render-examples go test -run TestRenderExamplesCorpus ./internal/apps/
#
# Only a 404 means a repository has no such file. Any other failure (a rate
# limit, a 5xx) stops the fetch, so a partial corpus never passes for a whole one.
set -euo pipefail
out=${1:?usage: $0 <output-dir>}
mkdir -p "$out"
err=$(mktemp)
trap 'rm -f "$err"' EXIT
gh api --paginate 'orgs/render-examples/repos?per_page=100' --jq '.[] | select(.archived|not) | .name' |
  while read -r repo; do
    for path in render.yaml render.yml; do
      if content=$(gh api "repos/render-examples/$repo/contents/$path" --jq .content 2>"$err"); then
        printf '%s' "$content" | base64 --decode > "$out/$repo.yaml"
        break
      fi
      if ! grep -q 'HTTP 404' "$err"; then
        echo "render-examples/$repo/$path: $(cat "$err")" >&2
        exit 1
      fi
    done
  done
echo "fetched $(ls "$out" | wc -l | tr -d ' ') render.yaml files into $out"
