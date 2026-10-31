#!/bin/sh
# Restore an extracted operand export; PGDATABASE selects the existing target.
# The sibling IMMV sidecar belongs to the same snapshot as the directory dump.
set -eu
if test "$#" -ne 1; then
  echo 'Usage: PGDATABASE=target bex-postgres-restore /path/to/export/source_database' >&2
  exit 2
fi
: "${PGDATABASE:?}"
dump=$(CDPATH= cd -- "$1" && pwd -P)
metadata="$(dirname "$dump")/pg_ivm_restore.sql"
if ! test -f "$dump/toc.dat" || ! test -r "$dump/toc.dat"; then
  echo 'The source must be a readable directory-format operand dump.' >&2
  exit 2
fi
if ! test -f "$metadata" || ! test -r "$metadata"; then
  echo 'The export is missing its readable pg_ivm_restore.sql sidecar.' >&2
  exit 2
fi

umask 077
restore_work=$(mktemp -d "${TMPDIR:-/tmp}/bex-postgres-restore.XXXXXX")
cleanup() {
  result=$?
  trap - EXIT
  rm -rf -- "$restore_work"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

# Inspect the archive without connecting to the target. Neither raw Timescale
# catalog COPY nor pre/post_restore is an ordinary-owner restore protocol.
pg_restore --list "$dump" > "$restore_work/toc.list"
if LC_ALL=C grep -Eq '^[0-9]+;[[:space:]]+[0-9]+[[:space:]]+[0-9]+[[:space:]]+EXTENSION[[:space:]]+-[[:space:]]+timescaledb([[:space:]]|$)' "$restore_work/toc.list"; then
  echo 'TimescaleDB archives require a supported owner restore protocol; no database changes were made.' >&2
  exit 1
fi
# pg_restore emits separate transactions for large-object data in SQL-file
# mode. Those commits would escape the transaction containing the IMMV sidecar.
if LC_ALL=C grep -Eq '^[0-9]+;[[:space:]]+[0-9]+[[:space:]]+[0-9]+[[:space:]]+BLOBS?([[:space:]]|$)' "$restore_work/toc.list"; then
  echo 'Large-object archives need an atomic restore protocol with the IMMV sidecar; no database changes were made.' >&2
  exit 1
fi

# No --create, --single-transaction, or actor override is passed to pg_restore:
# for the accepted archive it emits SQL without reconnecting or committing.
# Keep pg_dump's restrict/unrestrict commands intact for psql's own parser.
pg_restore --no-owner --no-privileges --file="$restore_work/restore.sql" "$dump"
# One transaction includes both files. A failed sidecar must roll back the dump
# rather than leave the original IMMV as an unmaintained ordinary table.
psql -X --dbname="$PGDATABASE" --single-transaction --set=ON_ERROR_STOP=1 \
  --file="$restore_work/restore.sql" --file="$metadata"
