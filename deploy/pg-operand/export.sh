#!/bin/sh
# A directory-format pg_dump and pg_ivm's restore commands from one snapshot.
# The existing Job still uploads one tar.gz; credentials remain its Secret env.
set -eu
: "${PGDATABASE:?}" "${EXPORT_DIRECTORY:?}" "${EXPORT_FILENAME:?}"
work_root=${BEX_POSTGRES_EXPORT_ROOT:-/work}
archive="$work_root/$EXPORT_FILENAME"
mkdir -p "$work_root/$EXPORT_DIRECTORY"
BEX_EXPORT_WORK=$(mktemp -d "$work_root/.bex-postgres-export.XXXXXX")
BEX_EXPORT_DESTINATION="$work_root/$EXPORT_DIRECTORY/$PGDATABASE"
export BEX_EXPORT_WORK BEX_EXPORT_DESTINATION
cleanup() {
  result=$?
  trap - EXIT
  rm -rf -- "$BEX_EXPORT_WORK"
  if test "$result" -ne 0; then rm -f -- "$archive"; fi
  exit "$result"
}
trap cleanup EXIT
metadata="$work_root/$EXPORT_DIRECTORY/pg_ivm_restore.sql"
: > "$metadata"
psql -X -qAt -v ON_ERROR_STOP=1 -v snapshot_file="$BEX_EXPORT_WORK/snapshot" -v metadata_file="$metadata" <<'SQL'
BEGIN ISOLATION LEVEL REPEATABLE READ;
SELECT pg_export_snapshot() \g :snapshot_file
SELECT EXISTS (SELECT FROM pg_extension WHERE extname = 'pg_ivm') AS bex_has_ivm \gset
\if :bex_has_ivm
SELECT pgivm.get_restore_immv_commands() \g :metadata_file
\endif
\! bex-postgres-dump-snapshot
\set bex_dump_ok `test -f "$BEX_EXPORT_WORK/dump.complete" && printf true || printf false`
\if :bex_dump_ok
COMMIT;
\else
ROLLBACK;
SELECT 1 / 0;
\endif
SQL
tar -C "$work_root" -czf "$archive" "$EXPORT_DIRECTORY"
