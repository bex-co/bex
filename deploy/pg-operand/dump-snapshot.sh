#!/bin/sh
# Called synchronously by the export's open psql transaction. No resume/append:
# any pg_dump failure leaves no completion marker and fails the parent export.
set -eu
: "${BEX_EXPORT_WORK:?}" "${BEX_EXPORT_DESTINATION:?}"
snapshot=$(cat "$BEX_EXPORT_WORK/snapshot")
pg_dump --format=directory --jobs=2 --no-owner --no-privileges \
  --snapshot="$snapshot" --file="$BEX_EXPORT_DESTINATION"
: > "$BEX_EXPORT_WORK/dump.complete"
