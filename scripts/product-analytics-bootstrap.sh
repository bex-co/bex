#!/usr/bin/env bash
# Credential handling is shared by the closed CLI/product reader profiles.
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec bash "$script_dir/lib/analytics-bootstrap.sh" product
