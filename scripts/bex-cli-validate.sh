#!/usr/bin/env bash
# Verify the deliberately narrow boundary between bex and render-oss/cli.
# This script is hermetic: it checks the resolved module graph only and does
# not contact Bex or Render.
set -euo pipefail
cd "$(dirname "$0")/.."

readonly UPSTREAM_MODULE=github.com/render-oss/cli
readonly EXPECTED_RELEASE=v2.22.0
readonly EXPECTED_COMMIT=d8fd7c2bb09d56beaca5df15ac2aefcb5ae5f427
readonly EXPECTED_VERSION=v1.1.3-0.20260721145337-d8fd7c2bb09d
readonly PIN_RECORD=cli/UPSTREAM_RENDER_CLI.md

fail() {
  echo "error: $*" >&2
  exit 1
}

[ -f "$PIN_RECORD" ] || fail "missing $PIN_RECORD"
grep -Fq "**Render release:** \`$EXPECTED_RELEASE\`" "$PIN_RECORD" \
  || fail "pin record must name Render release $EXPECTED_RELEASE"
grep -Fq "**Upstream commit:** \`$EXPECTED_COMMIT\`" "$PIN_RECORD" \
  || fail "pin record must name upstream commit $EXPECTED_COMMIT"
grep -Fq "**Go module version:** \`$EXPECTED_VERSION\`" "$PIN_RECORD" \
  || fail "pin record must name Go module version $EXPECTED_VERSION"
grep -Fq "readonly upstream_version=${EXPECTED_RELEASE#v}" scripts/bex-cli-build.sh \
  || fail "build script must embed upstream version ${EXPECTED_RELEASE#v}"

actual_version="$(cd cli && go list -m -f '{{.Version}}' "$UPSTREAM_MODULE")"
[ "$actual_version" = "$EXPECTED_VERSION" ] \
  || fail "$UPSTREAM_MODULE version is $actual_version, want $EXPECTED_VERSION; update $PIN_RECORD and review compatibility"

grep -Fq '"github.com/render-oss/cli/cmd"' cli/main.go \
  || fail "cli/main.go no longer imports upstream cmd package"
if find cli -type f -path '*/cmd/*' -print -quit | grep -q .; then
  fail "cli contains a copied cmd tree; keep command implementation upstream"
fi

echo "✓ bex CLI uses $UPSTREAM_MODULE $EXPECTED_VERSION ($EXPECTED_RELEASE) through an import boundary"
