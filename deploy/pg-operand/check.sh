#!/usr/bin/env bash
# Real-server gate for one published operand image (ADR092 D3 property 5).
#
# Runs inside the candidate image as its postgres user, with networking
# disabled, against a throwaway cluster configured the way the operator
# projects it (database_operand.go pgExtensionParameters, kept equal by
# TestCheckShConfiguresTheProjectedSettings). Usage:
#
#   docker run --rm --network none -v "$PWD/deploy/pg-operand:/check:ro" \
#     <image> bash /check/check.sh <major> <allowlist>
#
# <allowlist> is the comma-separated extwlist.extensions value for the major.
set -euo pipefail

major="$1"
allowlist="$2"
bin="/usr/lib/postgresql/${major}/bin"
data="$(mktemp -d)/data"
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
shared_preload_libraries = 'pg_stat_statements,timescaledb'
local_preload_libraries = 'pgextwlist'
extwlist.extensions = '${allowlist}'
extwlist.restrict_to_database_owner = on
extwlist.custom_path = '/usr/share/bex/extwlist'
timescaledb.license = 'apache'
timescaledb.telemetry_level = off
EOF
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
refused app appdb "CREATE EXTENSION adminpack" "permission denied\|not available\|could not open"
# The owner manages the extensions it created.
as app appdb "ALTER EXTENSION hstore UPDATE" >/dev/null

# Property 5: pg_dump/restore as the owner, through the same allowlist.
pg_dump -U app -d appdb -Fc -f /tmp/appdb.dump
restorable="$allowlist"
if has timescaledb; then
  # The owner's pg_dump of a hypertable works (the logical-export path), but
  # restoring one writes TimescaleDB's own catalog, which only a superuser
  # may: Timescale's pre/post-restore protocol. That restore belongs to
  # w8/m112; the rest of the database must round-trip without it.
  as app appdb "DROP TABLE metrics; DROP EXTENSION timescaledb" >/dev/null
  pg_dump -U app -d appdb -Fc -f /tmp/appdb.dump
  restorable="$(tr , '\n' <<<"$allowlist" | grep -vx timescaledb | paste -sd, -)"
fi
super "CREATE DATABASE restored OWNER app;"
pg_restore -U app -d restored --no-owner --exit-on-error /tmp/appdb.dump || fail "pg_restore failed"
for ext in ${restorable//,/ }; do
  [ "$(as app restored "SELECT count(*) FROM pg_extension WHERE extname = '${ext}'")" = 1 ] ||
    fail "restore lost extension ${ext}"
done
if has postgis; then
  [ "$(as app restored "SELECT count(*) FROM places")" = 50 ] || fail "PostGIS rows did not restore"
  [ "$(as app restored "SELECT count(*) FROM spatial_ref_sys WHERE srid = 990001")" = 1 ] || fail "custom SRID did not restore"
fi
if has pg_ivm; then
  # pg_ivm 1.15+ deliberately leaves its IMMV catalog out of dumps, so an
  # IMMV restores as a plain table with its rows (upstream behavior; w8/m110
  # owns that acceptance). The extension itself keeps working there.
  [ "$(as app restored "SELECT s FROM base_sum")" = 60 ] || fail "pg_ivm rows did not restore"
  as app restored "SELECT ${ivm}.create_immv('base_sum2', 'SELECT sum(v) AS s FROM base'); INSERT INTO base VALUES (4, 40)" >/dev/null
  [ "$(as app restored "SELECT s FROM base_sum2")" = 100 ] || fail "pg_ivm is not incremental after restore"
fi

# The owner can drop what it created.
for ext in ${restorable//,/ }; do as app appdb "DROP EXTENSION IF EXISTS ${ext} CASCADE" >/dev/null; done
echo "PASS PG${major}: ${allowlist}"
