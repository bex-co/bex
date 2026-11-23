#!/usr/bin/env bash
# Real-server gate for one published operand image (ADR092 D3).
#
# Runs inside the candidate image as its postgres user, with networking
# disabled, against a throwaway cluster configured the way the operator
# projects it (database_operand.go pgExtensionParameters, kept equal by
# TestCheckShConfiguresTheProjectedSettings). Usage:
#
#   docker run --rm --network none -v "$PWD/deploy/pg-operand:/check:ro" \
#     <image> bash /check/check.sh <major> <carried> <admitted> <exact-server-version>
#
# <carried> is every extension the image carries for the major (matrix.json
# "extensions"); <admitted> is the extwlist.extensions value the operator
# projects (matrix.json "admitted", pgExtensionAllowlist). Property 5 is
# judged per extension: each carried extension prints
# owner_restore_<ext>=PASS|FAIL with its exact error, and the gate fails when
# an admitted extension fails or a passing one is left out (ADR092 D3, w21/m584).
set -euo pipefail

major="$1"
carried="$2"
admitted="$3"
expected_version="${4:?exact base server patch is required}"
bin="/usr/lib/postgresql/${major}/bin"
test "$("$bin/postgres" --version | awk '{print $3}')" = "$expected_version"
data="$(mktemp -d)/data"
work="${data%/data}"
scripts="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)"
export PGHOST=/tmp PGPORT=5432 PGOPTIONS="-c client_min_messages=warning"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}
listed() { case ",$1," in *",$2,"*) return 0 ;; esac; return 1; }
has() { listed "$carried" "$1"; }
admits() { listed "$admitted" "$1"; }
for ext in ${admitted//,/ }; do has "$ext" || fail "admitted ${ext} is not carried by the image"; done

# The cgroup's anonymous memory, sampled at the gate's busiest points:
# memory.peak counts page cache too, so it says little about Free's headroom.
anon_max=0
sample_memory() {
  local anon
  anon="$(awk '$1 == "anon" {print $2}' /sys/fs/cgroup/memory.stat 2>/dev/null || true)"
  if [ -n "$anon" ] && [ "$anon" -gt "$anon_max" ]; then anon_max="$anon"; fi
}

"$bin/initdb" -D "$data" -U postgres --locale=C -E UTF8 >/dev/null
# dblink's peer: a non-superuser's dblink must authenticate with a password.
sed -i '1i local all dblink_peer scram-sha-256' "$data/pg_hba.conf"
cat >>"$data/postgresql.conf" <<EOF
listen_addresses = ''
unix_socket_directories = '/tmp'
# CloudNativePG's default; wal2json's logical slots need it.
wal_level = logical
EOF
# Production's projection, with production's allowlist.
sh "$scripts/config.sh" "$major" "$admitted" app 256 >>"$data/postgresql.conf"
"$bin/pg_ctl" -D "$data" -w -l "$work/log" start >/dev/null || {
  cat "$work/log"
  exit 1
}
trap '"$bin/pg_ctl" -D "$data" -m immediate stop >/dev/null 2>&1 || true' EXIT

# SQL goes on stdin so each statement autocommits (CREATE DATABASE cannot run
# in the implicit transaction of a multi-statement -c).
super() { psql -X -q -v ON_ERROR_STOP=1 -At -U postgres -d "${2:-postgres}" <<<"$1"; }
as() { psql -X -q -v ON_ERROR_STOP=1 -At -U "$1" -d "$2" <<<"$3"; }
refused() { # role db sql pattern
  local out
  if out="$(as "$1" "$2" "$3" 2>&1)"; then fail "$1 ran '$3' (want refusal): $out"; fi
  grep -q -- "$4" <<<"$out" || fail "'$3' failed differently: $out"
}
expect() { # want sql
  local got
  got="$(as app appdb "$2")"
  [ "$got" = "$1" ] || fail "'$2' = '$got', want '$1'"
}

super "CREATE ROLE app LOGIN; CREATE ROLE other LOGIN; CREATE DATABASE appdb OWNER app"
# The operator makes every owner a pg_monitor member for query insights
# (insightsMonitorRole), which opens the read-only inspection modules.
super "GRANT pg_monitor TO app"
super "SET password_encryption = 'scram-sha-256'; CREATE ROLE dblink_peer LOGIN PASSWORD 'gate-peer'"
super "GRANT CREATE ON DATABASE appdb TO other"
as app appdb "GRANT CREATE ON SCHEMA public TO other"

# Property 1: the list is the platform's. The owner cannot widen it.
refused app appdb "SET extwlist.extensions = 'postgres_fdw'" "permission denied"
refused app appdb "ALTER ROLE app SET extwlist.extensions = 'postgres_fdw'" "permission denied"
# A non-trusted extension Render does not list keeps Postgres's normal refusal
# (postgres_fdw is in every major's image), and so does one the image carries
# that failed its own gate.
refused app appdb "CREATE EXTENSION postgres_fdw" "permission denied to create extension"
for ext in ${carried//,/ }; do
  admits "$ext" || refused app appdb "CREATE EXTENSION ${ext}" "permission denied to create extension"
done
# Trusted extensions keep working exactly as before.
as app appdb "CREATE EXTENSION hstore" >/dev/null
# Only the database owner gets the allowlist path.
refused other appdb "CREATE EXTENSION ${admitted%%,*}" "restricted to the database owner"
# pgaudit refuses to load anywhere but shared_preload_libraries, so it is in
# the image's preload from the start.
case ",$(super 'SHOW shared_preload_libraries' | tr -d ' ')," in *,pgaudit,*) ;; *) fail "pgaudit is not preloaded" ;; esac

# wal2json is an output plugin, not an extension: a role with REPLICATION
# reads a database's changes through a logical slot.
super "SELECT 'created' FROM pg_create_logical_replication_slot('gate_wal2json', 'wal2json')" appdb >/dev/null
as app appdb "CREATE TABLE w2j (id int PRIMARY KEY); INSERT INTO w2j VALUES (1)"
changes="$(super "SELECT data FROM pg_logical_slot_get_changes('gate_wal2json', NULL, NULL)" appdb)"
grep -q '"kind":"insert".*"table":"w2j"' <<<"$changes" || fail "wal2json returned no insert for w2j: $changes"
super "SELECT pg_drop_replication_slot('gate_wal2json')" appdb >/dev/null

# Every carried extension is judged below, so the platform widens the list to
# all of them with a reload, as a later admission does in production: no
# restart (ADR092 D4).
echo "extwlist.extensions = '${carried}'" >>"$data/postgresql.conf"
"$bin/pg_ctl" -D "$data" reload >/dev/null
for ((attempt = 0; attempt < 50; attempt++)); do
  [ "$(as app appdb 'SHOW extwlist.extensions')" = "$carried" ] && break
  sleep 0.1
done
[ "$(as app appdb 'SHOW extwlist.extensions')" = "$carried" ] || fail "a reload did not widen extwlist.extensions"

# TimescaleDB first: pg_duckdb defines public.time_bucket only when no
# conflicting function exists (duckdb/pg_duckdb#934), so the reverse order makes
# TimescaleDB's install fail with "function time_bucket already exists".
create_order="$(has timescaledb && echo timescaledb; for e in ${carried//,/ }; do [ "$e" = timescaledb ] || echo "$e"; done)"
for ext in $create_order; do
  as app appdb "CREATE EXTENSION ${ext} CASCADE" >/dev/null
  echo "created ${ext}"
done
sample_memory

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
  # DuckDB execution runs in a database without TimescaleDB. pg_duckdb plans
  # its Postgres scans with standard_planner(), skipping the planner hook that
  # sets up TimescaleDB's per-query state, so with both extensions in one
  # database the scan segfaults inside ts_classify_relation (duckdb/pg_duckdb#845,
  # open upstream; ADR092 records the limitation, w8/236 the follow-up).
  super "CREATE DATABASE duckdb_scans OWNER app"
  as app duckdb_scans 'CREATE EXTENSION pg_duckdb' >/dev/null
  as app duckdb_scans 'CREATE TABLE duck_values AS SELECT generate_series(1,100000) AS v' >/dev/null
  [ "$(as app duckdb_scans 'SET duckdb.force_execution = true; SELECT sum(v) FROM duck_values')" = 5000050000 ] ||
    fail 'forced DuckDB scan returned the wrong sum'
  refused other duckdb_scans 'SET duckdb.force_execution = true; SELECT sum(g) FROM generate_series(1,10) g' 'permission denied\|not allowed'
  # Both instances live inside the same actual Free-plan cgroup. Observe only
  # these two controlled sessions after a real forced DuckDB scan.
  PGAPPNAME=operand-scan-a as app duckdb_scans 'SET duckdb.force_execution=true; SELECT sum(v) FROM duck_values; SET duckdb.force_execution=false; SELECT pg_sleep(5)' >"$work/scan-a" &
  scan_a=$!
  PGAPPNAME=operand-scan-b as app duckdb_scans 'SET duckdb.force_execution=true; SELECT sum(v) FROM duck_values; SET duckdb.force_execution=false; SELECT pg_sleep(5)' >"$work/scan-b" &
  scan_b=$!
  backends=''
  for ((attempt=0; attempt<100; attempt++)); do
    backends="$(as app duckdb_scans "SELECT pid FROM pg_stat_activity WHERE application_name IN ('operand-scan-a','operand-scan-b') AND wait_event='PgSleep'")"
    [ "$(wc -w <<<"$backends")" -ne 2 ] || break
    sleep 0.1
  done
  [ "$(wc -w <<<"$backends")" -eq 2 ] || fail 'two DuckDB scan sessions did not finish together'
  for backend in $backends; do
    awk '/^Private_(Clean|Dirty):/ {kb += $2} END {printf "duckdb_backend_private_bytes=%d\n", kb * 1024}' "/proc/$backend/smaps_rollup"
  done
  sample_memory
  wait "$scan_a"; wait "$scan_b"
  grep -qx 5000050000 "$work/scan-a"
  grep -qx 5000050000 "$work/scan-b"
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
if has pg_repack; then
  as app appdb "CREATE TABLE repacked (id int PRIMARY KEY, v text);
    INSERT INTO repacked SELECT g, md5(g::text) FROM generate_series(1, 1000) g; DELETE FROM repacked WHERE id % 2 = 0" >/dev/null
  filenode="$(as app appdb "SELECT pg_relation_filenode('repacked')")"
  checksum="SELECT md5(string_agg(id || v, ',' ORDER BY id)) FROM repacked"
  rows="$(as app appdb "$checksum")"
  size="$(as app appdb "SELECT pg_relation_size('repacked')")"
  # A table's owner repacks it with the client's superuser check skipped
  # (pg_repack 1.5.2+), as Render's guide runs it; extwlist/pg_repack/
  # after-create.sql is what lets the owner do that when the extension belongs
  # to the bootstrap superuser.
  out="$("$bin/pg_repack" --no-superuser-check -U app -d appdb --table=public.repacked 2>&1)" || fail "owner pg_repack: $out"
  expect "$rows" "$checksum"
  [ "$(as app appdb "SELECT pg_relation_filenode('repacked')")" != "$filenode" ] || fail "pg_repack did not rewrite the table: $out"
  [ "$(as app appdb "SELECT pg_relation_size('repacked') < ${size}")" = t ] || fail "pg_repack left the deleted rows' space: $out"
  # Property 2: the SECURITY DEFINER trigger inserts into a log table the owner
  # created, so it must run as the owner, never as the bootstrap superuser.
  expect app "SELECT proowner::regrole FROM pg_proc WHERE oid = 'repack.repack_trigger()'::regprocedure"
  # Only a table's owner repacks it. The client reports the refusal and still
  # exits 0, so the table itself is the evidence.
  as other appdb "CREATE TABLE others (id int PRIMARY KEY); INSERT INTO others SELECT generate_series(1, 10)" >/dev/null
  filenode="$(as app appdb "SELECT pg_relation_filenode('others')")"
  out="$("$bin/pg_repack" --no-superuser-check -U app -d appdb --table=public.others 2>&1 || true)"
  grep -q 'permission denied for table others' <<<"$out" || fail "repacking another role's table: $out"
  [ "$(as app appdb "SELECT pg_relation_filenode('others')")" = "$filenode" ] || fail "the database owner repacked another role's table: $out"
  as other appdb "DROP TABLE others" >/dev/null
fi
# Render's untrusted contrib modules keep PostgreSQL's own grants (ADR092 D1):
# each does its ordinary job for the owner, what PostgreSQL keeps for a
# superuser stays refused, and none reads a server file or borrows the local
# trust.
as app appdb "CREATE TABLE contrib_rows (id int PRIMARY KEY, who text, modified timestamptz NOT NULL DEFAULT 'epoch');
  INSERT INTO contrib_rows (id) SELECT generate_series(1, 10)" >/dev/null
has earthdistance && expect 111320 "SELECT round(earth_distance(ll_to_earth(0, 0), ll_to_earth(0, 1)))"
if has moddatetime; then
  as app appdb "CREATE TRIGGER contrib_modified BEFORE UPDATE ON contrib_rows FOR EACH ROW EXECUTE PROCEDURE moddatetime(modified);
    UPDATE contrib_rows SET id = id WHERE id = 1" >/dev/null
  expect 1 "SELECT count(*) FROM contrib_rows WHERE modified > 'epoch'"
fi
if has insert_username; then
  as app appdb "CREATE TRIGGER contrib_who BEFORE INSERT ON contrib_rows FOR EACH ROW EXECUTE PROCEDURE insert_username(who);
    INSERT INTO contrib_rows (id) VALUES (11)" >/dev/null
  expect app "SELECT who FROM contrib_rows WHERE id = 11"
fi
has bloom && as app appdb "CREATE INDEX contrib_bloom ON contrib_rows USING bloom (id)" >/dev/null
has intagg && expect "{1,2,3}" "SELECT int_array_aggregate(g) FROM generate_series(1, 3) g"
has dict_xsyn && expect "{supernova,sn,sne,1987a}" "CREATE TEXT SEARCH DICTIONARY contrib_xsyn (TEMPLATE = xsyn_template, RULES = 'xsyn_sample');
  SELECT ts_lexize('contrib_xsyn', 'supernova')"
has sslinfo && expect f "SELECT ssl_is_used()"
has pg_prewarm && expect t "SELECT pg_prewarm('contrib_rows') > 0"
has old_snapshot && expect 0 "SELECT count(*) FROM pg_old_snapshot_time_mapping()"
if has xml2; then
  expect x "SELECT xpath_string('<a><b>x</b></a>', '/a/b')"
  expect 0 "SELECT length(xpath_string('<!DOCTYPE a [<!ENTITY e SYSTEM \"file:///etc/passwd\">]><a>&e;</a>', '/a'))"
  refused app appdb "SELECT xslt_process('<a/>', '<xsl:stylesheet version=\"1.0\" xmlns:xsl=\"http://www.w3.org/1999/XSL/Transform\"><xsl:template match=\"/\"><xsl:copy-of select=\"document(''/etc/passwd'')\"/></xsl:template></xsl:stylesheet>')" "failed to apply stylesheet"
fi
if has dblink; then
  expect 1 "SELECT x FROM dblink('host=/tmp dbname=appdb user=dblink_peer password=gate-peer', 'SELECT 1') AS r(x int)"
  refused app appdb "SELECT dblink_connect('host=/tmp dbname=appdb user=postgres')" "password"
  refused app appdb "SELECT dblink_connect_u('host=/tmp dbname=appdb user=postgres')" "permission denied for function dblink_connect_u"
fi
has file_fdw && refused app appdb "CREATE SERVER contrib_files FOREIGN DATA WRAPPER file_fdw" "permission denied for foreign-data wrapper file_fdw"
has pageinspect && refused app appdb "SELECT get_raw_page('contrib_rows', 0)" "must be superuser"
has amcheck && refused app appdb "SELECT bt_index_check('contrib_rows_pkey')" "permission denied for function bt_index_check"
has pg_surgery && refused app appdb "SELECT heap_force_kill('contrib_rows', ARRAY['(0,1)']::tid[])" "permission denied for function heap_force_kill"
has adminpack && refused app appdb "SELECT pg_file_write('contrib', 'x', false)" "permission denied for function pg_file_write"
has pgaudit && refused app appdb "SET pgaudit.log = 'all'" "permission denied to set parameter"
has pg_buffercache && expect t "SELECT count(*) > 0 FROM pg_buffercache"
has pg_freespacemap && expect t "SELECT count(*) > 0 FROM pg_freespace('contrib_rows')"
has pg_visibility && expect t "SELECT count(*) > 0 FROM pg_visibility('contrib_rows')"
has pgstattuple && expect 11 "SELECT tuple_count FROM pgstattuple('contrib_rows')"
has pgrowlocks && expect 0 "SELECT count(*) FROM pgrowlocks('contrib_rows')"
# pgvector's fixture: three vectors with an HNSW index, whose nearest
# neighbour of [1,1,2] is item 3 (here and in property 5 below).
vector_items="CREATE TABLE items (id int PRIMARY KEY, embedding vector(3));
  INSERT INTO items VALUES (1, '[1,2,3]'), (2, '[4,5,6]'), (3, '[1,1,1]');
  CREATE INDEX items_embedding ON items USING hnsw (embedding vector_l2_ops)"
nearest_query="SELECT id FROM items ORDER BY embedding <-> '[1,1,2]' LIMIT 1"
nearest="SET enable_seqscan = off; $nearest_query"
if has vector; then
  # Render's documented command, as written (render.com/docs/postgresql-extensions):
  # no IF NOT EXISTS, so it fails wherever vector is already there, and a
  # migration run with ON_ERROR_STOP stops at that error. A new database has
  # none preinstalled, and the owner creates, uses and drops it (w21/m586).
  super "CREATE DATABASE vectors OWNER app"
  as app vectors "CREATE EXTENSION vector;" >/dev/null
  as app vectors "$vector_items" >/dev/null
  out="$(as app vectors "SET enable_seqscan = off; EXPLAIN (COSTS OFF) $nearest_query")"
  grep -q 'Index Scan using items_embedding' <<<"$out" || fail "the nearest-neighbour query does not use the HNSW index: $out"
  [ "$(as app vectors "$nearest")" = 3 ] || fail "the HNSW nearest neighbour of [1,1,2] is not item 3"
  as app vectors "DROP TABLE items; DROP EXTENSION vector" >/dev/null
  [ "$(as app vectors "SELECT count(*) FROM pg_extension WHERE extname = 'vector'")" = 0 ] || fail "the owner's DROP EXTENSION vector left it installed"
  # A database from before w21/m586 holds the vector CloudNativePG installed
  # as the superuser. Render's command meets it as PostgreSQL meets any
  # installed extension, IF NOT EXISTS is a no-op, its data keeps working, and
  # the owner can drop it through the allowlist and create its own.
  super "CREATE DATABASE vectors_preinstalled OWNER app"
  super "CREATE EXTENSION vector" vectors_preinstalled
  as app vectors_preinstalled "CREATE EXTENSION IF NOT EXISTS vector; $vector_items" >/dev/null
  refused app vectors_preinstalled "CREATE EXTENSION vector;" 'extension "vector" already exists'
  [ "$(as app vectors_preinstalled "$nearest")" = 3 ] || fail "the preinstalled vector's HNSW nearest neighbour is not item 3"
  as app vectors_preinstalled "DROP TABLE items; DROP EXTENSION vector; CREATE EXTENSION vector;" >/dev/null
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
refused app appdb "CREATE EXTENSION postgres_fdw" "permission denied to create extension"
# The owner manages the extensions it created.
as app appdb "ALTER EXTENSION hstore UPDATE" >/dev/null

# Property 5, per extension: a database that uses one extension, its owner's
# complete logical export, and the same ordinary owner's restore into a new
# database. Never delete a fixture to make its restore pass.
export PGUSER=app BEX_POSTGRES_EXPORT_ROOT="$work/export"
seed() { # ext db: what a database using only this extension holds
  case "$1" in
    postgis) as app "$2" "CREATE EXTENSION postgis; CREATE TABLE places (id int, geom geometry(Point, 4326));
      INSERT INTO places SELECT g, ST_SetSRID(ST_MakePoint(g, g), 4326) FROM generate_series(1, 50) g;
      CREATE INDEX ON places USING gist (geom);
      INSERT INTO spatial_ref_sys (srid, auth_name, auth_srid, proj4text) VALUES (990001, 'bex', 990001, '+proj=longlat +datum=WGS84 +no_defs')" ;;
    postgis_raster) as app "$2" "CREATE EXTENSION postgis_raster CASCADE; CREATE TABLE rasters (r raster);
      INSERT INTO rasters VALUES (ST_MakeEmptyRaster(2, 3, 0, 0, 1))" ;;
    postgis_topology) as app "$2" "CREATE EXTENSION postgis_topology CASCADE; SELECT topology.CreateTopology('t1', 4326);
      CREATE TABLE parcels (id int); SELECT topology.AddTopoGeometryColumn('t1', 'public', 'parcels', 'topo', 'POLYGON')" ;;
    postgis_tiger_geocoder) as app "$2" "CREATE EXTENSION postgis_tiger_geocoder CASCADE" ;;
    pg_partman) as app "$2" "CREATE EXTENSION pg_partman;
      CREATE TABLE events (id bigint, at timestamptz NOT NULL) PARTITION BY RANGE (at);
      SELECT create_parent('public.events', 'at', '1 day');
      UPDATE part_config SET retention = '30 days' WHERE parent_table = 'public.events'" ;;
    pg_ivm) as app "$2" "CREATE EXTENSION pg_ivm; CREATE TABLE base (id int PRIMARY KEY, v int);
      INSERT INTO base VALUES (1, 10), (2, 20); SELECT pgivm.create_immv('base_sum', 'SELECT sum(v) AS s FROM base');
      INSERT INTO base VALUES (3, 30)" ;;
    pg_similarity) as app "$2" "CREATE EXTENSION pg_similarity; CREATE TABLE words (w text); INSERT INTO words VALUES ('kitten')" ;;
    pg_duckdb) as app "$2" "CREATE EXTENSION pg_duckdb; CREATE TABLE duck_values AS SELECT generate_series(1, 1000) AS v" ;;
    pg_repack) as app "$2" "CREATE EXTENSION pg_repack; CREATE TABLE repacked (id int PRIMARY KEY, v text);
      INSERT INTO repacked SELECT g, md5(g::text) FROM generate_series(1, 100) g" ;;
    timescaledb) as app "$2" "CREATE EXTENSION timescaledb; CREATE TABLE metrics (at timestamptz NOT NULL, v float8);
      SELECT create_hypertable('metrics', 'at'); INSERT INTO metrics VALUES (now(), 1), (now() - interval '9 days', 2)" ;;
    # Render's untrusted contrib modules: three hold the owner's objects (a
    # domain column, a trigger, an index); the rest come back as the extension.
    earthdistance) as app "$2" "CREATE EXTENSION cube; CREATE EXTENSION earthdistance;
      CREATE TABLE spots (id int, at earth); INSERT INTO spots VALUES (1, ll_to_earth(0, 0))" ;;
    moddatetime) as app "$2" "CREATE EXTENSION moddatetime; CREATE TABLE notes (id int PRIMARY KEY, modified timestamptz NOT NULL DEFAULT 'epoch');
      CREATE TRIGGER notes_modified BEFORE UPDATE ON notes FOR EACH ROW EXECUTE PROCEDURE moddatetime(modified); INSERT INTO notes VALUES (1)" ;;
    bloom) as app "$2" "CREATE EXTENSION bloom; CREATE TABLE bloomed (a int, b int);
      INSERT INTO bloomed SELECT g, g % 7 FROM generate_series(1, 1000) g; CREATE INDEX bloomed_ab ON bloomed USING bloom (a, b)" ;;
    vector) as app "$2" "CREATE EXTENSION vector; $vector_items" ;;
    adminpack | amcheck | autoinc | dblink | dict_xsyn | file_fdw | insert_username | intagg | old_snapshot | pageinspect | \
      pg_buffercache | pg_freespacemap | pg_prewarm | pg_surgery | pg_visibility | pgaudit | pgrowlocks | pgstattuple | refint | sslinfo | xml2)
      as app "$2" "CREATE EXTENSION $1" ;;
    *) echo "no property 5 fixture for ${1}" >&2; return 1 ;;
  esac
}
restored() { # ext db: the restore gave the data back, and it still works
  case "$1" in
    postgis) [ "$(as app "$2" "SELECT count(*) FROM places WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint(10, 10), 4326), 1.5)")" = 3 ] &&
      [ "$(as app "$2" "SELECT count(*) FROM spatial_ref_sys WHERE srid = 990001")" = 1 ] ;;
    postgis_raster) [ "$(as app "$2" "SELECT ST_Width(r) FROM rasters")" = 2 ] ;;
    postgis_topology) [ "$(as app "$2" "SELECT count(*) FROM topology.layer l JOIN topology.topology t ON t.id = l.topology_id WHERE t.name = 't1' AND l.table_name = 'parcels'")" = 1 ] ;;
    postgis_tiger_geocoder) [ "$(as app "$2" "SET search_path = public, tiger; SELECT address || ' ' || streetname || ' ' || streettypeabbrev FROM tiger.normalize_address('123 Main Street, Boston, MA 02110')")" = "123 Main St" ] ;;
    pg_partman) [ "$(as app "$2" "SELECT retention FROM part_config WHERE parent_table = 'public.events'")" = "30 days" ] &&
      as app "$2" "CALL run_maintenance_proc()" &&
      [ "$(as app "$2" "SELECT count(*) > 1 FROM pg_inherits WHERE inhparent = 'events'::regclass")" = t ] ;;
    pg_ivm) [ "$(as app "$2" "SELECT s FROM base_sum")" = 60 ] && as app "$2" "INSERT INTO base VALUES (4, 40)" &&
      [ "$(as app "$2" "SELECT s FROM base_sum")" = 100 ] ;;
    pg_similarity) [ "$(as app "$2" "SELECT round(lev(w, 'sitting')::numeric, 2) FROM words")" = 0.57 ] ;;
    pg_duckdb) [ "$(as app "$2" "SET duckdb.force_execution = true; SELECT sum(v) FROM duck_values")" = 500500 ] ;;
    pg_repack) [ "$(as app "$2" "SELECT count(*) FROM repacked")" = 100 ] &&
      [ "$(as app "$2" "SELECT has_schema_privilege('repack', 'CREATE')")" = t ] ;;
    timescaledb) [ "$(as app "$2" "SELECT count(*) FROM metrics")" = 2 ] && as app "$2" "INSERT INTO metrics VALUES (now(), 3)" &&
      [ "$(as app "$2" "SELECT sum(v) FROM metrics")" = 6 ] ;;
    earthdistance) [ "$(as app "$2" "SELECT round(earth_distance(at, ll_to_earth(0, 1))) FROM spots")" = 111320 ] ;;
    moddatetime) as app "$2" "UPDATE notes SET id = id" && [ "$(as app "$2" "SELECT modified > 'epoch' FROM notes")" = t ] ;;
    bloom) [ "$(as app "$2" "SELECT amname FROM pg_class JOIN pg_am ON pg_am.oid = relam WHERE relname = 'bloomed_ab'")" = bloom ] &&
      [ "$(as app "$2" "SET enable_seqscan = off; SELECT count(*) FROM bloomed WHERE a = 7 AND b = 0")" = 1 ] ;;
    vector) [ "$(as app "$2" "SELECT amname FROM pg_class JOIN pg_am ON pg_am.oid = relam WHERE relname = 'items_embedding'")" = hnsw ] &&
      [ "$(as app "$2" "$nearest")" = 3 ] ;;
  esac
}
owner_restore() { # ext
  local dump="$BEX_POSTGRES_EXPORT_ROOT/owner-$1"
  super "CREATE DATABASE \"src_$1\" OWNER app; CREATE DATABASE \"dst_$1\" OWNER app" &&
    seed "$1" "src_$1" >/dev/null &&
    PGDATABASE="src_$1" EXPORT_DIRECTORY="owner-$1" EXPORT_FILENAME="owner-$1.tar.gz" bex-postgres-export &&
    { [ "$1" != timescaledb ] || as app "dst_$1" 'CREATE EXTENSION timescaledb; SELECT timescaledb_pre_restore()' >/dev/null; } &&
    pg_restore -U app -d "dst_$1" --no-owner --no-privileges --exit-on-error "$dump/src_$1" &&
    { [ "$1" != timescaledb ] || as app "dst_$1" 'SELECT timescaledb_post_restore()' >/dev/null; } &&
    psql -X -q -v ON_ERROR_STOP=1 -U app -d "dst_$1" -f "$dump/pg_ivm_restore.sql" >/dev/null &&
    [ "$(as app "dst_$1" "SELECT count(*) FROM pg_extension WHERE extname = '$1'")" = 1 ] &&
    restored "$1" "dst_$1" &&
    [ "$(as app "dst_$1" "SELECT rolsuper FROM pg_roles WHERE rolname = current_user")" = f ]
}
passed=""
for ext in ${carried//,/ }; do
  if owner_restore "$ext" >"$work/owner-restore-$ext.log" 2>&1; then
    passed="$passed,$ext"
    echo "owner_restore_${ext}=PASS"
  else
    echo "owner_restore_${ext}=FAIL"
    sed 's/^/  /' "$work/owner-restore-$ext.log"
  fi
  sample_memory
done

# The platform's recovery of the combined database (every extension at once,
# the coherent export the operator's Jobs take) is separate evidence: a
# stronger actor, never a substitute for an extension's owner verdict above.
export PGDATABASE=appdb EXPORT_DIRECTORY=operand EXPORT_FILENAME=appdb.dir.tar.gz
bex-postgres-export
archive="$BEX_POSTGRES_EXPORT_ROOT/$EXPORT_DIRECTORY/$PGDATABASE"
test -s "$BEX_POSTGRES_EXPORT_ROOT/$EXPORT_DIRECTORY/pg_ivm_restore.sql"
super "CREATE DATABASE restored OWNER app;"
has timescaledb && super "CREATE EXTENSION timescaledb; SELECT timescaledb_pre_restore()" restored >/dev/null
pg_restore -U postgres -d restored --exit-on-error "$archive" || fail "pg_restore failed"
has timescaledb && super "SELECT timescaledb_post_restore()" restored >/dev/null
psql -X -q -v ON_ERROR_STOP=1 -U app -d restored \
  -f "$BEX_POSTGRES_EXPORT_ROOT/$EXPORT_DIRECTORY/pg_ivm_restore.sql" >/dev/null
for ext in ${carried//,/ }; do
  [ "$(as app restored "SELECT count(*) FROM pg_extension WHERE extname = '${ext}'")" = 1 ] ||
    fail "restore lost extension ${ext}"
done
# appdb holds the same PostGIS, IMMV and hypertable fixtures as their seeds.
for ext in postgis pg_ivm timescaledb; do
  ! has "$ext" || restored "$ext" restored || fail "the platform restore did not give ${ext}'s data back working"
done
[ "$(as app restored "SELECT rolsuper, current_setting('is_superuser') FROM pg_roles WHERE rolname = current_user")" = 'f|off' ] || fail 'restore elevated the owner'
sample_memory
if [ -r /sys/fs/cgroup/memory.peak ]; then echo "cgroup_memory_peak_bytes=$(cat /sys/fs/cgroup/memory.peak)"; fi
echo "cgroup_memory_anon_max_sampled_bytes=${anon_max}"

# The owner can drop what it created.
for ext in ${carried//,/ }; do as app appdb "DROP EXTENSION IF EXISTS ${ext} CASCADE" >/dev/null; done

# The allowlist is exactly the passed set: an admitted extension that failed
# would reach production without property 5, and a passing one left out would
# be a parity gap nobody sees.
for ext in ${carried//,/ }; do
  if listed "${passed#,}" "$ext"; then
    admits "$ext" || fail "${ext} passed its owner restore on PG${major}; admit it in matrix.json and pgExtensionAllowlist"
  else
    ! admits "$ext" || fail "admitted ${ext} failed its owner restore on PG${major}"
  fi
done
echo "PASS PG${major}: admitted ${admitted}; carried ${carried}"
