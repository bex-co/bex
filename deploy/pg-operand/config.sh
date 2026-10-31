#!/bin/sh
# The real SQL gate's operand settings. Go's drift test executes this script
# and compares them with the actual Free-plan operator projection.
set -eu
major=$1
allowlist=$2
owner=$3
memory_mb=$4
case "$major" in 13|14|15|16|17|18) ;; *) exit 2 ;; esac
case "$memory_mb" in ''|*[!0-9]*) exit 2 ;; esac
case "$owner" in ''|*[!a-zA-Z0-9_-]*) exit 2 ;; esac
buffers=$((memory_mb / 4))
if test "$buffers" -lt 16; then buffers=16; fi
preloads=pg_stat_statements,timescaledb,pg_ivm
if test "$major" -ge 16; then preloads=$preloads,pg_duckdb; fi
cat <<EOF
shared_buffers = '${buffers}MB'
shared_preload_libraries = '$preloads'
local_preload_libraries = 'pgextwlist'
extwlist.extensions = '$allowlist'
extwlist.restrict_to_database_owner = 'on'
extwlist.custom_path = '/usr/share/bex/extwlist'
timescaledb.license = 'apache'
timescaledb.telemetry_level = 'off'
EOF
if test "$major" -ge 16; then
  memory=$((memory_mb / 8))
  if test "$memory" -lt 1; then memory=1; fi
  cat <<EOF
duckdb.postgres_role = '$owner'
duckdb.max_memory = '$memory'
duckdb.threads = '1'
duckdb.enable_external_access = 'false'
duckdb.autoinstall_known_extensions = 'false'
duckdb.autoload_known_extensions = 'false'
EOF
fi
