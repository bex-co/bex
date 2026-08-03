#!/usr/bin/env bash
# Check, but never update, the Render Blueprint JSON Schema pinned by m63/t001.
# The repository bytes remain the only runtime contract; this script's temporary
# download is removed on exit even when Render has changed upstream.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pinned="$repo_root/lego/backend/internal/apps/schema/render.yaml.json"
expected_sha256="665539cb0c191856ba38d292b985a963880bb69b030d666e5fe7788e78e7e696"
schema_url="${RENDER_BLUEPRINT_SCHEMA_URL:-https://render.com/schema/render.yaml.json}"

if [[ ! -f "$pinned" ]]; then
  echo "pinned Render Blueprint schema is missing: $pinned" >&2
  exit 1
fi

sha256() {
  shasum -a 256 "$1" | awk '{print $1}'
}

pinned_sha256="$(sha256 "$pinned")"
if [[ "$pinned_sha256" != "$expected_sha256" ]]; then
  echo "pinned Render Blueprint schema integrity mismatch: got $pinned_sha256, want $expected_sha256" >&2
  exit 1
fi

temporary_schema="$(mktemp -t bex-render-schema.XXXXXX)"
trap 'rm -f "$temporary_schema"' EXIT
curl --fail --location --retry 2 --silent --show-error "$schema_url" --output "$temporary_schema"

upstream_sha256="$(sha256 "$temporary_schema")"
if [[ "$upstream_sha256" == "$pinned_sha256" ]]; then
  echo "Render Blueprint schema matches pinned $pinned_sha256"
  exit 0
fi

echo "::error title=Render Blueprint schema drift::pinned=$pinned_sha256 upstream=$upstream_sha256" >&2
diff -u "$pinned" "$temporary_schema" || true
exit 1
