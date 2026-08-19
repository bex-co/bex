#!/usr/bin/env bash
# Check, but never update, the pinned render-oss/render-mcp-server tool surface.
#
# The MCP counterpart of render-schema-drift.sh. Until w1/m70 the MCP adapter
# had no pin at all — parity was asserted in hand-written comments carrying
# manual check dates, which is how bex reached 213 tools against upstream's 22
# without anyone deciding to. This job is what keeps that from recurring.
#
# Only the `tools` array is compared: names and argument names are the
# contractual surface, while descriptions and annotations drift editorially and
# would otherwise fire this job on non-contractual churn. Refreshing the pin's
# own `pin`/`source` metadata therefore never registers as drift.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pinned="$repo_root/lego/backend/internal/api/openapi/render-mcp-tools.json"
capture="$repo_root/scripts/render-mcp-capture.py"
expected_sha256="28ac990ade694df68502b9d7e5a79473691b0f2ba2d09cca181d6bcad75fdca1"
ref="${RENDER_MCP_REF:-main}"

if [[ ! -f "$pinned" ]]; then
  echo "pinned Render MCP tool surface is missing: $pinned" >&2
  exit 1
fi
if [[ ! -x "$capture" ]]; then
  echo "capture script is missing or not executable: $capture" >&2
  exit 1
fi

sha256() {
  shasum -a 256 "$1" | awk '{print $1}'
}

pinned_sha256="$(sha256 "$pinned")"
if [[ "$pinned_sha256" != "$expected_sha256" ]]; then
  echo "pinned Render MCP tool surface integrity mismatch: got $pinned_sha256, want $expected_sha256" >&2
  echo "the pin was edited without updating renderMCPToolsSHA256 in lego/backend/internal/api/render_mcp.go" >&2
  exit 1
fi

upstream="$(mktemp -t bex-render-mcp.XXXXXX)"
trap 'rm -f "$upstream"' EXIT

# Builds upstream at $ref and reads its own tools/list over the MCP stdio
# handshake, so the comparison is against what the server actually registers.
"$capture" --ref "$ref" --tools-only --out "$upstream"

if diff -u \
  <(jq -S '.tools' "$pinned") \
  <(jq -S '.' "$upstream"); then
  echo "Render MCP tool surface matches the pin ($(jq '.tools | length' "$pinned") tools at $ref)"
  exit 0
fi

added="$(comm -13 \
  <(jq -r '.tools[].name' "$pinned" | sort) \
  <(jq -r '.[].name' "$upstream" | sort) | paste -sd' ' -)"
removed="$(comm -23 \
  <(jq -r '.tools[].name' "$pinned" | sort) \
  <(jq -r '.[].name' "$upstream" | sort) | paste -sd' ' -)"

echo "::error title=Render MCP tool drift::upstream ref=$ref added=[${added:-none}] removed=[${removed:-none}]" >&2
cat >&2 <<MSG

Upstream's MCP tool surface moved. To refresh the pin:

  scripts/render-mcp-capture.py --ref $ref --out $pinned
  # then re-add the human 'pin' block, update renderMCPToolsSHA256 in
  # lego/backend/internal/api/render_mcp.go, and re-run:
  cd lego/backend && go test ./internal/api/ -run TestMCPParity

A tool bex neither implements nor declines will fail
TestMCPParityUpstreamToolsAreImplementedOrAcknowledged, and a changed argument
surface may reclassify a bex tool as Divergent — both are decisions to make
deliberately, which is why this job never refreshes the pin itself.
MSG
exit 1
