#!/usr/bin/env bash
set -euo pipefail

mobile_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
repo_dir=$(cd "$mobile_dir/.." && pwd)
schema_tmp=$(mktemp "${TMPDIR:-/tmp}/bex-mobile-schema.XXXXXX.json")
trap 'rm -f -- "$schema_tmp"' EXIT

(
  cd "$repo_dir/lego/backend"
  SCHEMA_DUMP_PATH="$schema_tmp" \
    go test ./internal/api -run '^TestDumpGraphQLSchema$' -count=1
)

(
  cd "$mobile_dir"
  SCHEMA_JSON="$schema_tmp" yarn codegen
)
