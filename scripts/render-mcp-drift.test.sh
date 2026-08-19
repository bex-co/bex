#!/usr/bin/env bash
# Unit tests for render-mcp-drift.sh's failure modes. Network-free: the capture
# step is stubbed, so what is under test is the drift script's own logic —
# integrity checking, diffing, and whether its failure message is actionable.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/render-mcp-drift.sh"
pinned="$repo_root/lego/backend/internal/api/openapi/render-mcp-tools.json"

failures=0
check() {
  local name="$1" expected_exit="$2" expected_text="$3" actual_exit=0 output
  shift 3
  output="$("$@" 2>&1)" || actual_exit=$?
  if [[ "$actual_exit" != "$expected_exit" ]]; then
    echo "FAIL: $name — exit $actual_exit, want $expected_exit" >&2
    echo "$output" | head -5 >&2
    failures=$((failures + 1))
    return
  fi
  if [[ -n "$expected_text" ]] && ! grep -qF "$expected_text" <<<"$output"; then
    echo "FAIL: $name — output missing '$expected_text'" >&2
    echo "$output" | head -5 >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}

sandbox="$(mktemp -d -t bex-mcp-drift-test.XXXXXX)"
trap 'rm -rf "$sandbox"' EXIT

# A stub capture that emits whatever tools array the test wants, so the drift
# path is exercised without building the upstream server.
make_stub() {
  local body="$1" dest="$sandbox/capture.py"
  cat >"$dest" <<STUB
#!/usr/bin/env python3
import sys, json
out = sys.argv[sys.argv.index("--out") + 1]
open(out, "w").write(json.dumps($body))
STUB
  chmod +x "$dest"
  echo "$dest"
}

run_with() { # run_with <stub-body> [env...]
  local stub; stub="$(make_stub "$1")"
  shift
  local root="$sandbox/repo"
  rm -rf "$root"
  mkdir -p "$root/scripts" "$root/lego/backend/internal/api/openapi"
  cp "$script" "$root/scripts/render-mcp-drift.sh"
  cp "$stub" "$root/scripts/render-mcp-capture.py"
  cp "$pinned" "$root/lego/backend/internal/api/openapi/render-mcp-tools.json"
  ( cd "$root" && env "$@" ./scripts/render-mcp-drift.sh )
}

pinned_tools="$(jq -c '.tools' "$pinned")"

check "matching upstream passes" 0 "matches the pin" \
  run_with "$pinned_tools"

check "added upstream tool fails and is named" 1 "added=[brand_new_tool]" \
  run_with "$(jq -c '. + [{"name":"brand_new_tool","args":[],"required":[]}]' <<<"$pinned_tools")"

check "removed upstream tool fails and is named" 1 "removed=[list_services]" \
  run_with "$(jq -c 'map(select(.name != "list_services"))' <<<"$pinned_tools")"

check "changed argument surface fails" 1 "Render MCP tool drift" \
  run_with "$(jq -c 'map(if .name == "list_services" then .args += ["invented"] else . end)' <<<"$pinned_tools")"

check "failure message says how to refresh" 1 "render-mcp-capture.py" \
  run_with "$(jq -c 'map(select(.name != "list_services"))' <<<"$pinned_tools")"

check "failure message names the guard test" 1 "TestMCPParity" \
  run_with "$(jq -c 'map(select(.name != "list_services"))' <<<"$pinned_tools")"

# A hand-edited pin must fail on integrity before any capture runs.
edited_root="$sandbox/edited"
mkdir -p "$edited_root/scripts" "$edited_root/lego/backend/internal/api/openapi"
cp "$script" "$edited_root/scripts/render-mcp-drift.sh"
cp "$(make_stub "$pinned_tools")" "$edited_root/scripts/render-mcp-capture.py"
jq '.tools[0].name = "tampered"' "$pinned" >"$edited_root/lego/backend/internal/api/openapi/render-mcp-tools.json"
check "hand-edited pin fails integrity" 1 "integrity mismatch" \
  bash -c "cd '$edited_root' && ./scripts/render-mcp-drift.sh"

check "hand-edited pin names the Go constant" 1 "renderMCPToolsSHA256" \
  bash -c "cd '$edited_root' && ./scripts/render-mcp-drift.sh"

# A missing pin must fail loudly rather than silently passing.
missing_root="$sandbox/missing"
mkdir -p "$missing_root/scripts" "$missing_root/lego/backend/internal/api/openapi"
cp "$script" "$missing_root/scripts/render-mcp-drift.sh"
cp "$(make_stub "$pinned_tools")" "$missing_root/scripts/render-mcp-capture.py"
check "missing pin fails loudly" 1 "is missing" \
  bash -c "cd '$missing_root' && ./scripts/render-mcp-drift.sh"

if ((failures > 0)); then
  echo "$failures test(s) failed" >&2
  exit 1
fi
echo "all render-mcp-drift.sh tests passed"
