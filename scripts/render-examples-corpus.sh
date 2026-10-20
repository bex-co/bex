#!/usr/bin/env bash
# Fetch every render.yaml in github.com/render-examples into a directory, for
# lego/backend/internal/apps' opt-in TestRenderExamplesCorpus (w5/m143). The
# files are third-party, so they are fetched, never vendored.
#
#   bash scripts/render-examples-corpus.sh /tmp/render-examples
#   BEX_RENDER_EXAMPLES_DIR=/tmp/render-examples go test -run TestRenderExamplesCorpus ./internal/apps/
set -euo pipefail
out=${1:?usage: $0 <output-dir>}
mkdir -p "$out"
gh api --paginate 'orgs/render-examples/repos?per_page=100' --jq '.[] | select(.archived|not) | .name' |
  while read -r repo; do
    for path in render.yaml render.yml; do
      if content=$(gh api "repos/render-examples/$repo/contents/$path" --jq .content 2>/dev/null); then
        printf '%s' "$content" | base64 --decode > "$out/$repo.yaml"
        break
      fi
    done
  done
echo "fetched $(ls "$out" | wc -l | tr -d ' ') render.yaml files into $out"
