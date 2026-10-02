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
expected_sha256="8c044390d89fc2a2129714ba39e786de201af9d05deccb8f751e204c1c15ed2b"
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
  echo "and expected_sha256 in this script; if it was not a deliberate capture refresh, restore it from git" >&2
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

pinned_names="$(jq -r '.tools[].name' "$pinned" | sort)"
upstream_names="$(jq -r '.[].name' "$upstream" | sort)"
added="$(comm -13 <(echo "$pinned_names") <(echo "$upstream_names") | paste -sd' ' -)"
removed="$(comm -23 <(echo "$pinned_names") <(echo "$upstream_names") | paste -sd' ' -)"
# A tool present on both sides whose argument or required set moved. Named
# separately because it is the easiest drift to read past: before w1/m165 the
# headline listed only added/removed tools, so upstream's Dockerfile arguments
# on create_web_service/create_cron_job (bc94f8d) rode along unnamed behind
# `added=[list_events]`.
changed="$(jq -rn --slurpfile p "$pinned" --slurpfile u "$upstream" '
  def surface: {args: (.args | sort), required: (.required | sort)};
  ($p[0].tools | map({key: .name, value: surface}) | from_entries) as $pin
  | ($u[0] | map({key: .name, value: surface}) | from_entries) as $up
  | [$pin | keys[] | select($up[.] != null and $pin[.] != $up[.])] | join(" ")')"

echo "::error title=Render MCP tool drift::upstream ref=$ref added=[${added:-none}] removed=[${removed:-none}] changed=[${changed:-none}]" >&2
cat >&2 <<MSG

Upstream's MCP tool surface moved away from the reviewed pin
($pinned). The diff above is exact; what each kind of drift needs:

  added=[...]    a tool bex neither implements nor declines. Decide it:
                 implement it with upstream's argument names, or record why bex
                 declines it in mcpKnownUpstreamOnly
                 (lego/backend/internal/api/mcp_parity.go).
  removed=[...]  upstream dropped a tool. Delete any mcpKnownUpstreamOnly entry
                 for it; a bex tool of that name becomes an Extension.
  changed=[...]  an argument or required set moved on a shared tool. The bex
                 tool may now be Divergent: fix its arguments, or record the
                 reason in mcpAcceptedDivergences (same file).

Then refresh the pin — never by hand-editing its tools array:

  scripts/render-mcp-capture.py --ref <upstream commit> --out /tmp/render-mcp.json
  # copy .source and .tools from /tmp/render-mcp.json into the pin, bump
  # pin.capturedAt, and set its new sha256 in BOTH renderMCPToolsSHA256
  # (lego/backend/internal/api/render_mcp.go) and expected_sha256 (this script)
  cd lego/backend && go test ./internal/api/ -run 'TestMCPParity|TestRenderMCP|TestScopeMatrix'

TestMCPParityUpstreamToolsAreImplementedOrAcknowledged and
TestMCPParityEveryDivergenceIsAccepted fail until each item above is decided,
which is why this job never refreshes the pin itself.
MSG
exit 1
