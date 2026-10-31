#!/usr/bin/env bash
# Real-server gate for one published operand image (ADR092 D3 property 5).
#
# Runs inside the candidate image as its postgres user, with networking
# disabled, against a throwaway cluster configured the way the operator
# projects it (database_operand.go pgExtensionParameters, kept equal by
# TestCheckShConfiguresTheProjectedSettings). Usage:
#
#   docker run --rm --network none -v "$PWD/deploy/pg-operand:/check:ro" \
#     <image> bash /check/check.sh <major> <allowlist> <exact-server-version>
#
# <allowlist> is the comma-separated extwlist.extensions value for the major.
set -euo pipefail

major="$1"
allowlist="$2"
expected_version="${3:?exact base server patch is required}"
bin="/usr/lib/postgresql/${major}/bin"
test "$("$bin/postgres" --version | awk '{print $3}')" = "$expected_version"
data="$(mktemp -d)/data"
scripts="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)"
export PGHOST=/tmp PGPORT=5432 PGOPTIONS="-c client_min_messages=warning"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
has() { case ",$allowlist," in *",$1,"*) return 0 ;; esac; return 1; }

"$bin/initdb" -D "$data" -U postgres --locale=C -E UTF8 >/dev/null
cat >>"$data/postgresql.conf" <<EOF
listen_addresses = ''
unix_socket_directories = '/tmp'
EOF
sh "$scripts/config.sh" "$major" "$allowlist" app 256 >>"$data/postgresql.conf"
"$bin/pg_ctl" -D "$data" -w -l "$data/../log" start >/dev/null || {
  cat "$data/../log"
  exit 1
}
trap '"$bin/pg_ctl" -D "$data" -m immediate stop >/dev/null 2>&1 || true' EXIT

# SQL goes on stdin so each statement autocommits (CREATE DATABASE cannot run
# in the implicit transaction of a multi-statement -c).
super() { psql -X -q -v ON_ERROR_STOP=1 -At -U postgres -d postgres <<<"$1"; }
as() { psql -X -q -v ON_ERROR_STOP=1 -At -U "$1" -d "$2" <<<"$3"; }
refused() { # role db sql pattern
  local out
  if out="$(as "$1" "$2" "$3" 2>&1)"; then fail "$1 ran '$3' (want refusal): $out"; fi
  grep -q -- "$4" <<<"$out" || fail "'$3' failed differently: $out"
}

super "CREATE ROLE app LOGIN; CREATE ROLE other LOGIN; CREATE DATABASE appdb OWNER app"
super "GRANT CREATE ON DATABASE appdb TO other"
as app appdb "GRANT CREATE ON SCHEMA public TO other"

# Property 1: the list is the platform's. The owner cannot widen it.
refused app appdb "SET extwlist.extensions = 'file_fdw'" "permission denied"
refused app appdb "ALTER ROLE app SET extwlist.extensions = 'file_fdw'" "permission denied"
# Every other non-trusted extension keeps Postgres's normal refusal.
refused app appdb "CREATE EXTENSION file_fdw" "permission denied"
refused app appdb "CREATE EXTENSION dblink" "permission denied"
# Trusted extensions keep working exactly as before.
as app appdb "CREATE EXTENSION hstore" >/dev/null
# Only the database owner gets the allowlist path.
first="${allowlist%%,*}"
refused other appdb "CREATE EXTENSION ${first}" "restricted to the database owner"

expect() { # want sql
  local got
  got="$(as app appdb "$2")"
  [ "$got" = "$1" ] || fail "'$2' = '$got', want '$1'"
}

for ext in ${allowlist//,/ }; do
  as app appdb "CREATE EXTENSION ${ext} CASCADE" >/dev/null
  echo "created ${ext}"
done

if has postgis; then
  expect 5 "SELECT ST_Distance('SRID=3857;POINT(0 0)'::geometry, 'SRID=3857;POINT(3 4)'::geometry)"
  as app appdb "CREATE TABLE places (id int, geom geometry(Point, 4326));
    INSERT INTO places SELECT g, ST_SetSRID(ST_MakePoint(g, g), 4326) FROM generate_series(1, 50) g;
    CREATE INDEX ON places USING gist (geom)" >/dev/null
  expect 3 "SELECT count(*) FROM places WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint(10, 10), 4326), 1.5)"
fi
has postgis_raster && expect 2 "SELECT ST_Width(ST_MakeEmptyRaster(2, 3, 0, 0, 1))"
if has postgis_topology; then
  as app appdb "SELECT topology.CreateTopology('t1', 4326);
    CREATE TABLE parcels (id int); SELECT topology.AddTopoGeometryColumn('t1', 'public', 'parcels', 'topo', 'POLYGON')" >/dev/null
fi
if has postgis_tiger_geocoder; then
  expect 1 "SELECT count(*) FROM pg_extension WHERE extname = 'fuzzystrmatch'"
  expect "123 Main St" "SET search_path = public, tiger; SELECT address || ' ' || streetname || ' ' || streettypeabbrev FROM tiger.normalize_address('123 Main Street, Boston, MA 02110')"
fi
has postgis && as app appdb "INSERT INTO spatial_ref_sys (srid, auth_name, auth_srid, proj4text) VALUES (990001, 'bex', 990001, '+proj=longlat +datum=WGS84 +no_defs')" >/dev/null
if has pg_ivm; then
  ivm="$(as app appdb "SELECT pronamespace::regnamespace FROM pg_proc WHERE proname = 'create_immv' LIMIT 1")"
  as app appdb "CREATE TABLE base (id int PRIMARY KEY, v int); INSERT INTO base VALUES (1, 10), (2, 20);
    SELECT ${ivm}.create_immv('base_sum', 'SELECT sum(v) AS s FROM base'); INSERT INTO base VALUES (3, 30)" >/dev/null
  expect 60 "SELECT s FROM base_sum"
fi
has pg_similarity && expect 0.57 "SELECT round(lev('kitten', 'sitting')::numeric, 2)"
if has pg_duckdb; then
  expect t "SELECT setting='32' AND unit='MB' FROM pg_settings WHERE name='duckdb.max_memory'"
  expect 1 "SHOW duckdb.threads"
  refused app appdb "SET duckdb.max_memory = '4096'" 'permission denied'
  [ "$(super "SELECT context FROM pg_settings WHERE name='duckdb.postgres_role'")" = postmaster ] || fail 'DuckDB role is not restart-only'
  refused app appdb "SET duckdb.postgres_role = 'other'" 'cannot be changed without restarting the server'
  [ "$(super 'SHOW duckdb.postgres_role')" = app ] || fail 'owner changed DuckDB role'
  refused app appdb "SET duckdb.enable_external_access = true" 'permission denied'
  as app appdb 'CREATE TABLE duck_values AS SELECT generate_series(1,100000) AS v' >/dev/null
  expect 5000050000 'SET duckdb.force_execution = true; SELECT sum(v) FROM duck_values'
  refused other appdb 'SET duckdb.force_execution = true; SELECT sum(g) FROM generate_series(1,10) g' 'permission denied\|not allowed'
  # Both instances live inside the same actual Free-plan cgroup. Observe only
  # these two controlled sessions after a real forced DuckDB scan.
  PGAPPNAME=operand-scan-a as app appdb 'SET duckdb.force_execution=true; SELECT sum(v) FROM duck_values; SET duckdb.force_execution=false; SELECT pg_sleep(5)' >"$data/../scan-a" &
  scan_a=$!
  PGAPPNAME=operand-scan-b as app appdb 'SET duckdb.force_execution=true; SELECT sum(v) FROM duck_values; SET duckdb.force_execution=false; SELECT pg_sleep(5)' >"$data/../scan-b" &
  scan_b=$!
  backends=''
  for ((attempt=0; attempt<100; attempt++)); do
    backends="$(as app appdb "SELECT pid FROM pg_stat_activity WHERE application_name IN ('operand-scan-a','operand-scan-b') AND wait_event='PgSleep'")"
    [ "$(wc -w <<<"$backends")" -ne 2 ] || break
    sleep 0.1
  done
  [ "$(wc -w <<<"$backends")" -eq 2 ] || fail 'two DuckDB scan sessions did not finish together'
  for backend in $backends; do
    awk '/^Private_(Clean|Dirty):/ {kb += $2} END {printf "duckdb_backend_private_bytes=%d\n", kb * 1024}' "/proc/$backend/smaps_rollup"
  done
  wait "$scan_a"; wait "$scan_b"
  grep -qx 5000050000 "$data/../scan-a"
  grep -qx 5000050000 "$data/../scan-b"
fi
if has pg_partman; then
  as app appdb "CREATE TABLE events (id bigint, at timestamptz NOT NULL) PARTITION BY RANGE (at)" >/dev/null
  schema="$(as app appdb "SELECT extnamespace::regnamespace FROM pg_extension WHERE extname = 'pg_partman'")"
  as app appdb "SELECT ${schema}.create_parent('public.events', 'at', '1 day');
    UPDATE ${schema}.part_config SET retention = '30 days' WHERE parent_table = 'public.events';
    CALL ${schema}.run_maintenance_proc()" >/dev/null
  [ "$(as app appdb "SELECT count(*) > 1 FROM pg_inherits WHERE inhparent = 'events'::regclass")" = t ] ||
    fail "pg_partman created no partitions"
  # Property 3: Render runs pg_partman without its background worker.
  [ "$(as app appdb "SELECT count(*) FROM pg_stat_activity WHERE backend_type LIKE 'pg_partman%'")" = 0 ] ||
    fail "pg_partman background worker is running"
fi
if has timescaledb; then
  as app appdb "CREATE TABLE metrics (at timestamptz NOT NULL, v float8);
    SELECT create_hypertable('metrics', 'at'); INSERT INTO metrics VALUES (now(), 1), (now() - interval '9 days', 2)" >/dev/null
  expect 2 "SELECT count(*) FROM metrics"
  # Property 3: Community (TSL) features stay unavailable. The owner cannot
  # switch the license, and the image carries no TSL module even for a
  # superuser who does.
  refused app appdb "ALTER TABLE metrics SET (timescaledb.compress)" "apache"
  refused app appdb "SET timescaledb.license = 'timescale'" "permission denied"
  if out="$(psql -X -q -At -U postgres -d appdb -c "SET timescaledb.license = 'timescale'" 2>&1)"; then
    fail "the TSL module loaded: $out"
  fi
  grep -q 'invalid value' <<<"$out" || fail "superuser license switch failed differently: $out"
fi

# Property 2: no superuser leaks to the owner.
expect "f|off" "SELECT rolsuper, current_setting('is_superuser') FROM pg_roles WHERE rolname = current_user"
as app appdb "DO \$check\$ BEGIN BEGIN CREATE EXTENSION postgis; EXCEPTION WHEN duplicate_object THEN NULL; END; IF current_user <> 'app' OR current_setting('is_superuser') <> 'off' THEN RAISE EXCEPTION 'extension exception leaked privileges'; END IF; END \$check\$" >/dev/null
refused app appdb "CREATE EXTENSION adminpack" "permission denied\|not available\|could not open"
# The owner manages the extensions it created.
as app appdb "ALTER EXTENSION hstore UPDATE" >/dev/null

# Property 5: the owner's complete logical export, with the same snapshot for
# IMMV metadata and pg_dump. Never delete a fixture to make its restore pass.
export PGUSER=app PGDATABASE=appdb BEX_POSTGRES_EXPORT_ROOT="${data%/data}/export"
export EXPORT_DIRECTORY=operand EXPORT_FILENAME=appdb.dir.tar.gz
bex-postgres-export
archive="$BEX_POSTGRES_EXPORT_ROOT/$EXPORT_DIRECTORY/$PGDATABASE"
test -s "$BEX_POSTGRES_EXPORT_ROOT/$EXPORT_DIRECTORY/pg_ivm_restore.sql"
super "CREATE DATABASE owner_restored OWNER app;"
owner_restore_failed=false
owner_restore_log="${data%/data}/owner-restore.log"
# Keep the original ordinary-owner attempt and its exact errors. Platform
# recovery below is separate proof and cannot turn this criterion green.
if ! {
  as app owner_restored 'CREATE EXTENSION timescaledb; SELECT timescaledb_pre_restore()' &&
  pg_restore -U app -d owner_restored --no-owner --no-privileges --exit-on-error "$archive" &&
  as app owner_restored 'SELECT timescaledb_post_restore()' &&
  psql -X -q -v ON_ERROR_STOP=1 -U app -d owner_restored \
    -f "$BEX_POSTGRES_EXPORT_ROOT/$EXPORT_DIRECTORY/pg_ivm_restore.sql" &&
  as app owner_restored 'INSERT INTO base VALUES (4,40); INSERT INTO metrics VALUES (now(),3)' &&
  test "$(as app owner_restored 'SELECT s FROM base_sum')" = 100 &&
  test "$(as app owner_restored 'SELECT sum(v) FROM metrics')" = 6;
} >"$owner_restore_log" 2>&1; then
  owner_restore_failed=true
  cat "$owner_restore_log"
  echo 'ordinary_owner_complete_restore=FAIL; platform recovery is separate evidence'
else
  echo 'ordinary_owner_complete_restore=PASS'
fi
super "CREATE DATABASE restored OWNER app;"
# Timescale's internal catalogs require the platform restore actor. Ordinary
# schemas are restored as the original owner; no tenant role gains superuser.
psql -X -q -v ON_ERROR_STOP=1 -U postgres -d restored \
  -c 'CREATE EXTENSION timescaledb' -c 'SELECT timescaledb_pre_restore()' >/dev/null
pg_restore -U postgres -d restored --exit-on-error "$archive" || fail "pg_restore failed"
psql -X -q -v ON_ERROR_STOP=1 -U postgres -d restored -c 'SELECT timescaledb_post_restore()' >/dev/null
psql -X -q -v ON_ERROR_STOP=1 -U app -d restored \
  -f "$BEX_POSTGRES_EXPORT_ROOT/$EXPORT_DIRECTORY/pg_ivm_restore.sql" >/dev/null
for ext in ${allowlist//,/ }; do
  [ "$(as app restored "SELECT count(*) FROM pg_extension WHERE extname = '${ext}'")" = 1 ] ||
    fail "restore lost extension ${ext}"
done
if has postgis; then
  [ "$(as app restored "SELECT count(*) FROM places")" = 50 ] || fail "PostGIS rows did not restore"
  [ "$(as app restored "SELECT count(*) FROM spatial_ref_sys WHERE srid = 990001")" = 1 ] || fail "custom SRID did not restore"
fi
if has pg_ivm; then
  [ "$(as app restored "SELECT s FROM base_sum")" = 60 ] || fail "pg_ivm rows did not restore"
  as app restored "INSERT INTO base VALUES (4, 40)" >/dev/null
  [ "$(as app restored "SELECT s FROM base_sum")" = 100 ] || fail "original IMMV is not incremental after restore"
fi
expect 2 'SELECT count(*) FROM metrics'
[ "$(as app restored 'SELECT count(*) FROM metrics')" = 2 ] || fail 'hypertable rows did not restore'
as app restored 'INSERT INTO metrics VALUES (now(), 3)' >/dev/null
[ "$(as app restored 'SELECT sum(v) FROM metrics')" = 6 ] || fail 'restored hypertable is not writable'
[ "$(as app restored "SELECT rolsuper, current_setting('is_superuser') FROM pg_roles WHERE rolname = current_user")" = 'f|off' ] || fail 'restore elevated the owner'
if [ -r /sys/fs/cgroup/memory.peak ]; then echo "cgroup_memory_peak_bytes=$(cat /sys/fs/cgroup/memory.peak)"; fi

# The owner can drop what it created.
for ext in ${allowlist//,/ }; do as app appdb "DROP EXTENSION IF EXISTS ${ext} CASCADE" >/dev/null; done
if "$owner_restore_failed"; then fail 'ordinary-owner complete Timescale/IMMV logical restore remains unsupported'; fi
echo "PASS PG${major}: ${allowlist}"
