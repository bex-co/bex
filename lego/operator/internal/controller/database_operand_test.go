/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPGExtensionAllowlistMatchesTheImageGate keeps the operator's allowlist
// and the publish workflow's real-server gate (deploy/pg-operand/matrix.json
// "admitted") on one list per major, for every major the CRD permits: the gate
// fails when an admitted extension fails its own verdict or a passing one is
// left out, so this equality is what carries those verdicts to production.
// Every admitted extension is one the image carries.
func TestPGExtensionAllowlistMatchesTheImageGate(t *testing.T) {
	raw, err := os.ReadFile("../../../../deploy/pg-operand/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var matrix map[string]struct {
		Base       string   `json:"base"`
		Extensions []string `json:"extensions"`
		Admitted   []string `json:"admitted"`
	}
	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatal(err)
	}
	versions := permittedPostgresVersions(t)
	if got := slices.Sorted(maps.Keys(matrix)); !slices.Equal(got, versions) {
		t.Errorf("matrix.json majors = %v, want the CRD's %v", got, versions)
	}
	if got := slices.Sorted(maps.Keys(pgExtensionAllowlist)); !slices.Equal(got, versions) {
		t.Errorf("pgExtensionAllowlist majors = %v, want the CRD's %v", got, versions)
	}
	for _, v := range versions {
		t.Logf("PG%s exact operand server_version_num=%s", v, operandServerVersionNumber(t, v))
		if !slices.Equal(matrix[v].Admitted, pgExtensionAllowlist[v]) {
			t.Errorf("PG%s: matrix.json admitted %v != pgExtensionAllowlist %v", v, matrix[v].Admitted, pgExtensionAllowlist[v])
		}
		for _, ext := range matrix[v].Admitted {
			if !slices.Contains(matrix[v].Extensions, ext) {
				t.Errorf("PG%s: admits %s, which the image does not carry (matrix.json extensions %v)", v, ext, matrix[v].Extensions)
			}
		}
		if !strings.Contains(matrix[v].Base, ":"+v+".") || !strings.Contains(matrix[v].Base, "@sha256:") {
			t.Errorf("PG%s: base %q is not a digest-pinned PG%s image", v, matrix[v].Base, v)
		}
	}
}

// TestCheckShConfiguresTheProjectedSettings: the publish gate's server must be
// configured exactly as the operator projects a bex operand cluster, or the
// gate proves a different allowlist than production runs.
// TestAllowlistIsRendersUntrustedListPerMajor: the owner may create exactly
// the untrusted extensions Render lists, on exactly the majors it lists them
// for (render.com/docs/postgresql-extensions, read 2026-11-23; PostgreSQL 17
// removed adminpack and old_snapshot), less TimescaleDB, whose own gate fails
// (ADR092 D1, D3 property 1). pgvector (`vector`, PostgreSQL 13+) is among
// them since a new database no longer gets it preinstalled (w21/m586), and
// so is pg_stat_statements, which CloudNativePG still creates in every
// database: without the allowlist the owner's restore of its own export stops
// at COMMENT ON EXTENSION pg_stat_statements (w21/124). Nothing else gets the
// allowlist path.
func TestAllowlistIsRendersUntrustedListPerMajor(t *testing.T) {
	all := [2]int{13, 18}
	render := map[string][2]int{
		"postgis": all, "postgis_raster": all, "postgis_topology": all, "postgis_tiger_geocoder": all,
		"pg_partman": {14, 18}, "pg_ivm": all, "pg_similarity": {13, 15}, "pg_duckdb": {16, 18}, "pg_repack": {16, 18},
		"amcheck": all, "autoinc": all, "bloom": all, "dblink": all, "dict_xsyn": all, "earthdistance": all,
		"file_fdw": all, "insert_username": all, "intagg": all, "moddatetime": all, "pageinspect": all,
		"pg_buffercache": all, "pg_freespacemap": all, "pg_prewarm": all, "pg_stat_statements": all, "pg_surgery": {14, 18}, "pg_visibility": all,
		"pgaudit": all, "pgrowlocks": all, "pgstattuple": all, "refint": all, "sslinfo": all, "xml2": all,
		"adminpack": {13, 16}, "old_snapshot": {14, 16}, "timescaledb": all, "vector": all,
	}
	for _, version := range permittedPostgresVersions(t) {
		major, _ := strconv.Atoi(version)
		list := pgExtensionAllowlist[version]
		for name, majors := range render {
			// TimescaleDB is on Render's list, but its own owner restore fails.
			want := majors[0] <= major && major <= majors[1] && name != "timescaledb"
			if slices.Contains(list, name) != want {
				t.Errorf("PG%s: %s admitted = %t, want %t (Render lists it on %d-%d)", version, name, !want, want, majors[0], majors[1])
			}
		}
		for _, name := range list {
			if _, listed := render[name]; !listed {
				t.Errorf("PG%s: admits %s, which Render does not list as an untrusted extension", version, name)
			}
		}
	}
}

func TestCheckShConfiguresTheProjectedSettings(t *testing.T) {
	plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{})
	for _, version := range permittedPostgresVersions(t) {
		t.Run(version, func(t *testing.T) {
			list := strings.Join(pgExtensionAllowlist[version], ",")
			raw, err := exec.Command("sh", "../../../../deploy/pg-operand/config.sh", version, list, "app", "256").CombinedOutput()
			if err != nil {
				t.Fatalf("actual gate config: %v %s", err, raw)
			}
			got := map[string]any{}
			for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
				k, v, ok := strings.Cut(line, " = ")
				if !ok {
					t.Fatalf("unexpected gate config: %s", line)
				}
				got[k] = strings.Trim(v, "'")
			}
			want := pgExtensionParameters(clusterParams{version: version, plan: plan, owner: "app"})
			preloads := []string{"pg_stat_statements"}
			for _, lib := range pgExtensionPreloadLibraries(version) {
				preloads = append(preloads, lib.(string))
			}
			want["shared_preload_libraries"] = strings.Join(preloads, ",")
			if !maps.Equal(got, want) {
				t.Errorf("gate settings = %v; want actual Free projection %v", got, want)
			}
		})
	}
}

// TestEveryOperandMajorPreloadsWhatItCarries: every library the image
// carries that must be preloaded is preloaded from the start on every major,
// whether or not its extension is admitted, so admitting one later is a
// reload, never a restart (ADR092 D4). pgaudit loads from nowhere else.
func TestEveryOperandMajorPreloadsWhatItCarries(t *testing.T) {
	for _, v := range permittedPostgresVersions(t) {
		preload := pgExtensionPreloadLibraries(v)
		want := []any{"timescaledb", "pg_ivm", "pgaudit"}
		if major, _ := strconv.Atoi(v); major >= 16 {
			want = []any{"timescaledb", "pg_ivm", "pg_duckdb", "pgaudit"}
		}
		if !slices.Equal(preload, want) {
			t.Errorf("PG%s preloads %v, want %v", v, preload, want)
		}
		if slices.Contains(preload, any("pg_partman_bgw")) {
			t.Errorf("PG%s preloads pg_partman's background worker, which Render runs without", v)
		}
	}
}

// TestWal2jsonIsABlessedOutputPluginWhereTheServerAsks: on 14-18 the server
// refuses a logical slot whose plugin is not in output_plugin_libraries, so
// the operand lists wal2json beside the defaults; PostgreSQL 13 has no such
// setting and would not start with it. A tenant value never replaces it.
func TestWal2jsonIsABlessedOutputPluginWhereTheServerAsks(t *testing.T) {
	plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{})
	for _, v := range permittedPostgresVersions(t) {
		got, ok := pgExtensionParameters(clusterParams{version: v, plan: plan, owner: "app"})["output_plugin_libraries"]
		if v == "13" {
			if ok {
				t.Errorf("PG13 projects output_plugin_libraries=%v, a setting its server does not have", got)
			}
			continue
		}
		if got != "pgoutput,test_decoding,wal2json" {
			t.Errorf("PG%s output_plugin_libraries = %v, want pgoutput,test_decoding,wal2json", v, got)
		}
	}
	withBexOperands(t, map[string]string{"18": "ghcr.io/bex-co/bex-operator:18.7-postgres-abc@sha256:0123"})
	_, _, params := clusterPostgresql(t, "18", map[string]string{" Output_Plugin_Libraries ": "pgoutput,evil"})
	if params[" Output_Plugin_Libraries "] != nil || params["output_plugin_libraries"] != "pgoutput,test_decoding,wal2json" {
		t.Errorf("a tenant value reached output_plugin_libraries: %v", params)
	}
}

// TestBexOperandImagesArePinnedPerMajor guards each adoption commit: a bex
// operand is digest-pinned, for a permitted major, and tagged with that
// major's version first, which is where CloudNativePG reads it from.
func TestBexOperandImagesArePinnedPerMajor(t *testing.T) {
	versions := permittedPostgresVersions(t)
	for v, image := range bexOperandImages {
		if !slices.Contains(versions, v) {
			t.Errorf("bexOperandImages[%q]: not a permitted major", v)
		}
		if !strings.Contains(image, ":"+v+".") || !strings.Contains(image, "@sha256:") {
			t.Errorf("bexOperandImages[%q] = %q, want <repo>:%s.<patch>-…@sha256:…", v, image, v)
		}
	}
}

func withBexOperands(t *testing.T, operands map[string]string) {
	t.Helper()
	saved := bexOperandImages
	bexOperandImages = operands
	t.Cleanup(func() { bexOperandImages = saved })
}

func clusterPostgresql(t *testing.T, version string, tenant map[string]string) (image string, preload []any, params map[string]any) {
	t.Helper()
	plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{})
	spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: 1, version: version, dbname: "app", owner: "app", parameters: tenant})
	pg := spec["postgresql"].(map[string]any)
	image, _ = spec["imageName"].(string)
	return image, pg["shared_preload_libraries"].([]any), pg["parameters"].(map[string]any)
}

// TestClusterSpecProjectsTheAllowlistOnlyWithABexOperand: a bex operand gets
// pgextwlist, the closed per-major list and TimescaleDB's preload and Apache
// license, none of which a tenant parameter can replace (ADR092 D3 properties
// 1, 3 and 4). Any other image keeps today's projection exactly, because
// those libraries do not exist there and the server would not start.
func TestClusterSpecProjectsTheAllowlistOnlyWithABexOperand(t *testing.T) {
	tenant := map[string]string{
		"work_mem":                "8MB",
		"extwlist.extensions":     "postgres_fdw",
		"local_preload_libraries": "",
		"timescaledb.license":     "timescale",
	}

	withBexOperands(t, map[string]string{})
	image, preload, _ := clusterPostgresql(t, "16", tenant)
	if image != cnpgExportImages["16"] || !slices.Equal(preload, []any{"pg_stat_statements"}) {
		t.Errorf("upstream operand: image %q preload %v, want %q [pg_stat_statements]", image, preload, cnpgExportImages["16"])
	}

	const bex = "ghcr.io/bex-co/bex-operator:16.15-postgres-abc@sha256:0123"
	withBexOperands(t, map[string]string{"16": bex})
	image, preload, params := clusterPostgresql(t, "16", tenant)
	if image != bex {
		t.Errorf("imageName = %q, want the bex operand %q", image, bex)
	}
	if !slices.Equal(preload, []any{"pg_stat_statements", "timescaledb", "pg_ivm", "pg_duckdb", "pgaudit"}) {
		t.Errorf("shared_preload_libraries = %v, want pg_stat_statements + timescaledb, pg_ivm, pg_duckdb, pgaudit (no pg_partman_bgw)", preload)
	}
	want := map[string]any{
		"local_preload_libraries": "pgextwlist",
		"extwlist.extensions": "postgis,postgis_raster,postgis_topology,postgis_tiger_geocoder,pg_partman,pg_ivm,pg_duckdb,pg_repack,vector," +
			"amcheck,autoinc,bloom,dblink,dict_xsyn,earthdistance,file_fdw,insert_username,intagg,moddatetime,pageinspect," +
			"pg_buffercache,pg_freespacemap,pg_prewarm,pg_stat_statements,pg_surgery,pg_visibility,pgaudit,pgrowlocks,pgstattuple,refint,sslinfo,xml2," +
			"adminpack,old_snapshot",
		"extwlist.restrict_to_database_owner": "on",
		"extwlist.custom_path":                "/usr/share/bex/extwlist",
		"timescaledb.license":                 "apache",
		"timescaledb.telemetry_level":         "off",
		"work_mem":                            "8MB",
	}
	for k, v := range want {
		if params[k] != v {
			t.Errorf("parameters[%q] = %v, want %v", k, params[k], v)
		}
	}

	// Another major in the same reconcile keeps its upstream image untouched.
	if image, preload, params := clusterPostgresql(t, "18", nil); image != cnpgExportImages["18"] || len(preload) != 1 || params["local_preload_libraries"] != nil {
		t.Errorf("PG18 without a bex operand: image %q preload %v local_preload %v", image, preload, params["local_preload_libraries"])
	}
}

// TestExportJobsUseTheServingOperand: pg_dump of a database that uses one of
// the operand's extensions runs from the same image (ADR092 D1).
func TestExportJobsUseTheServingOperand(t *testing.T) {
	const bex = "ghcr.io/bex-co/bex-operator:18.6-postgres-abc@sha256:0123"
	withBexOperands(t, map[string]string{"18": bex})
	for _, v := range []string{"18", "", "19"} {
		if got := cnpgExportImage(v); got != bex {
			t.Errorf("cnpgExportImage(%q) = %q, want the PG18 bex operand", v, got)
		}
		if command := postgresExportCommand(v); command != "exec /usr/local/bin/bex-postgres-export" {
			t.Errorf("operand export must retain coherent IVM metadata on version %q: %s", v, command)
		}
	}
	if got := cnpgExportImage("17"); got != cnpgExportImages["17"] {
		t.Errorf("cnpgExportImage(17) = %q, want the upstream pin", got)
	}
	if command := postgresExportCommand("17"); strings.Contains(command, "bex-postgres-export") || !strings.Contains(command, "pg_dump --format=directory") {
		t.Fatalf("upstream export lost its supported command: %s", command)
	}
}

func TestOperandGuardsDirectParametersAndBoundsDuckDB(t *testing.T) {
	withBexOperands(t, map[string]string{"18": "ghcr.io/bex-co/bex-operator:18.6-postgres-candidate@sha256:0123"})
	for _, adopted := range []bool{false, true} {
		t.Run(strconv.FormatBool(adopted), func(t *testing.T) {
			version := "17"
			if adopted {
				version = "18"
			}
			_, _, params := clusterPostgresql(t, version, map[string]string{
				" ExTwLiSt.extensions ": "postgres_fdw", "DuckDB.memory_limit": "9999",
				"session_preload_libraries": "unsafe", "work_mem": "8MB",
			})
			for _, key := range []string{" ExTwLiSt.extensions ", "DuckDB.memory_limit", "session_preload_libraries"} {
				if _, ok := params[key]; ok {
					t.Errorf("direct CR bypassed platform configuration: %s", key)
				}
			}
			if params["work_mem"] != "8MB" {
				t.Fatal("ordinary parameter was lost")
			}
			if adopted && (params["duckdb.max_memory"] != "32" || params["duckdb.threads"] != "1" || params["duckdb.postgres_role"] != "app" || params["shared_buffers"] != "64MB") {
				t.Fatalf("Free operand resource/owner projection: %v", params)
			}
		})
	}
}

// TestApplicationDatabaseExtensionsOnCNPG is ADR092 D3's actual-CNPG gate (w8/m92
// t008): bex's own Cluster projection, on a real CloudNativePG
// (BEX_CNPG_KUBECONFIG) and a candidate operand image per major
// (BEX_PG_OPERAND_IMAGES="18=<image>,13=<image>", loaded on the nodes), lets
// the application owner run m92's original PostGIS migration and every other
// allowlisted extension, while untrusted extensions stay refused and the owner
// stays non-superuser. Opt-in: needs the cluster, kubectl and the images.
func TestApplicationDatabaseExtensionsOnCNPG(t *testing.T) {
	cl, kubeconfig, operands := operandQualificationCluster(t)
	for _, major := range slices.Sorted(maps.Keys(operands)) {
		t.Run("PG"+major, func(t *testing.T) {
			// A tenant's own parameter override rides along, as it would in
			// production, and cannot widen the list.
			namespace, asOwner := startCNPGDatabase(t, cl, kubeconfig, "bex-operand-it", major, "geo_prod", "geo_app",
				map[string]string{"extwlist.extensions": "postgres_fdw", "work_mem": "8MB"})
			if out, err := asOwner("SELECT current_setting('server_version_num')"); err != nil || out != operandServerVersionNumber(t, major) {
				t.Fatalf("actual PG%s server patch differs from reviewed base: %v %s", major, err, out)
			}

			// m92's original migration, unchanged, from the owner's connection.
			migration, err := os.ReadFile("testdata/postgis-migration.sql")
			if err != nil {
				t.Fatal(err)
			}
			out, err := asOwner(string(migration))
			if err != nil || !strings.Contains(out, `"distance_m": 111319.491`) || !strings.Contains(out, `"wkt": "POINT(0 0)"`) {
				t.Fatalf("original PostGIS migration = %v %q", err, out)
			}

			// DuckDB executes before TimescaleDB exists in this database, then
			// leaves: a forced DuckDB scan in a database that also has
			// TimescaleDB segfaults the server (duckdb/pg_duckdb#845, ADR092),
			// and pg_duckdb created first would take TimescaleDB's time_bucket.
			if major == "16" || major == "17" || major == "18" {
				if out, err := asOwner("CREATE EXTENSION pg_duckdb; CREATE TABLE duck_values AS SELECT generate_series(1,100000) AS v; SET duckdb.force_execution=true; SELECT sum(v) FROM duck_values"); err != nil || !strings.HasSuffix(out, "5000050000") {
					t.Fatalf("actual Free DuckDB execution: %v %s", err, out)
				}
				if out, err := asOwner("DROP EXTENSION pg_duckdb"); err != nil {
					t.Fatalf("drop pg_duckdb before TimescaleDB: %v %s", err, out)
				}
			}
			steps := []struct{ sql, want string }{
				{"SELECT rolsuper FROM pg_roles WHERE rolname = current_user", "f"},
				{"SHOW extwlist.extensions", strings.Join(pgExtensionAllowlist[major], ",")},
				{"SHOW work_mem", "8MB"},
				{"CREATE EXTENSION postgis; CREATE TABLE places (id int, geom geometry(Point, 4326));" +
					" INSERT INTO places SELECT g, ST_SetSRID(ST_MakePoint(g, g), 4326) FROM generate_series(1, 50) g;" +
					" CREATE INDEX ON places USING gist (geom);" +
					" SELECT count(*) FROM places WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint(10, 10), 4326), 1.5)", "3"},
				{"CREATE EXTENSION hstore; SELECT 'a=>1'::hstore -> 'a'", "1"},
				// Render's pgvector command as written: nothing preinstalled it,
				// and the owner drops it again (w21/m586).
				{"CREATE EXTENSION vector; CREATE TABLE items (id int PRIMARY KEY, embedding vector(3));" +
					" INSERT INTO items VALUES (1, '[1,2,3]'), (2, '[4,5,6]'), (3, '[1,1,1]');" +
					" CREATE INDEX ON items USING hnsw (embedding vector_l2_ops);" +
					" SET enable_seqscan = off; SELECT id FROM items ORDER BY embedding <-> '[1,1,2]' LIMIT 1", "3"},
				{"DROP TABLE items; DROP EXTENSION vector; SELECT count(*) FROM pg_extension WHERE extname = 'vector'", "0"},
				// CloudNativePG created pg_stat_statements as the superuser
				// (pg_stat_statements.track); through the allowlist the owner
				// replaces it by Render's command (w21/124).
				{"DROP EXTENSION pg_stat_statements; CREATE EXTENSION pg_stat_statements; SELECT count(*) > 0 FROM pg_stat_statements", "t"},
				{"SELECT rolsuper FROM pg_roles WHERE rolname = current_user", "f"},
			}
			timescale := ownerTimescaleOnCNPG(t, major, asOwner)
			if slices.Contains(pgExtensionAllowlist[major], "pg_partman") {
				steps = append(steps, struct{ sql, want string }{
					"CREATE SCHEMA partman; CREATE EXTENSION pg_partman SCHEMA partman;" +
						" CREATE TABLE events (id bigint, at timestamptz NOT NULL) PARTITION BY RANGE (at);" +
						" SELECT partman.create_parent('public.events', 'at', '1 day');" +
						" SELECT count(*) > 1 FROM pg_inherits WHERE inhparent = 'events'::regclass", "t"})
			}
			for _, step := range steps {
				if out, err := asOwner(step.sql); err != nil || !strings.HasSuffix(out, step.want) {
					t.Fatalf("%s = %v %q, want suffix %q", step.sql, err, out, step.want)
				}
			}
			if out, err := asOwner("CREATE EXTENSION pg_ivm; CREATE TABLE ivm_base (id int PRIMARY KEY,v int); INSERT INTO ivm_base VALUES (1,10),(2,20); SELECT pgivm.create_immv('ivm_sum','SELECT sum(v) AS s FROM ivm_base'); INSERT INTO ivm_base VALUES (3,30); SELECT s FROM ivm_sum"); err != nil || !strings.HasSuffix(out, "60") {
				t.Fatalf("actual IMMV maintenance: %v %s", err, out)
			}
			verifyCNPGOwnerExportRestore(t, cl, kubeconfig, namespace, timescale)

			ownerContribOnCNPG(t, asOwner)
			for _, refused := range []string{"CREATE EXTENSION postgres_fdw", "SET extwlist.extensions = 'postgres_fdw'"} {
				if out, err := asOwner(refused); err == nil || !strings.Contains(out, "permission denied") {
					t.Errorf("%s = %v %q, want permission denied", refused, err, out)
				}
			}
			ownerPgauditAcrossRestartOnCNPG(t, cl, kubeconfig, namespace, "geo_prod", asOwner)
		})
	}
}

// ownerPgauditAcrossRestartOnCNPG: the pgaudit the owner created (in
// ownerContribOnCNPG) is still there after the instance pod restarts, as on
// Render, and the owner can drop it. bex preloads pgaudit and sets no pgaudit.*
// parameter, so CloudNativePG 1.30.1's instance manager drops it on its first
// pass after the start; bex's build keeps it (w21/m606). No other database
// gains it either way.
func ownerPgauditAcrossRestartOnCNPG(t *testing.T, cl client.Client, kubeconfig, namespace, dbname string, asOwner func(string) (string, error)) {
	t.Helper()
	ctx := context.Background()
	superuser := func(database, sql string) (string, error) {
		cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "-n", namespace, "exec", "-i", "pg-1", "-c", "postgres", "--",
			"psql", "-X", "-v", "ON_ERROR_STOP=1", "-At", "-d", database)
		cmd.Stdin = strings.NewReader(sql)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	// The pass that would drop pgaudit creates pg_stat_statements (bex sets
	// pg_stat_statements.track) in the same transaction, so its return marks
	// that pass as committed.
	if out, err := superuser(dbname, "DROP EXTENSION pg_stat_statements"); err != nil {
		t.Fatalf("drop pg_stat_statements before the restart: %v %s", err, out)
	}
	var pod corev1.Pod
	key := client.ObjectKey{Namespace: namespace, Name: "pg-1"}
	if err := cl.Get(ctx, key, &pod); err != nil {
		t.Fatal(err)
	}
	before := pod.UID
	if err := cl.Delete(ctx, &pod); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Minute, "a new pg-1 to be ready", func() bool {
		var current corev1.Pod
		return cl.Get(ctx, key, &current) == nil && current.UID != before && podReady(&current)
	})
	waitFor(t, 3*time.Minute, "the instance manager's extension pass after the restart", func() bool {
		out, err := superuser(dbname, "SELECT count(*) FROM pg_extension WHERE extname = 'pg_stat_statements'")
		return err == nil && out == "1"
	})
	if out, err := asOwner("SELECT count(*) FROM pg_extension WHERE extname = 'pgaudit'"); err != nil || out != "1" {
		t.Fatalf("after the restart the owner's pgaudit = %v %q, want it still installed", err, out)
	}
	databases, err := superuser("postgres", "SELECT datname FROM pg_database WHERE datallowconn ORDER BY 1")
	if err != nil {
		t.Fatalf("list databases: %v %s", err, databases)
	}
	for database := range strings.FieldsSeq(databases) {
		if database == dbname {
			continue
		}
		if out, err := superuser(database, "SELECT count(*) FROM pg_extension WHERE extname = 'pgaudit'"); err != nil || out != "0" {
			t.Errorf("after the restart %s has pgaudit = %v %q, want none: its owner never created it", database, err, out)
		}
	}
	if out, err := asOwner("DROP EXTENSION pgaudit; SELECT count(*) FROM pg_extension WHERE extname = 'pgaudit'"); err != nil || !strings.HasSuffix(out, "0") {
		t.Fatalf("the owner's DROP EXTENSION pgaudit after the restart = %v %q", err, out)
	}
}

// TestOwnerPgauditAcrossRestartOnCNPG is the restart step alone, on the same
// real CloudNativePG and operands as TestApplicationDatabaseExtensionsOnCNPG:
// the owner runs Render's `CREATE EXTENSION pgaudit;` in a new database, the
// instance pod restarts, and the extension is still there (w21/m606). It fails
// on the release bex's build patches. cnpg-operator-qualify.yml runs
// TestApplicationDatabaseExtensionsOnCNPG, which ends with the same step
// (w21/124); BEX_CNPG_QUALIFY_TEST selects this one alone.
func TestOwnerPgauditAcrossRestartOnCNPG(t *testing.T) {
	cl, kubeconfig, operands := operandQualificationCluster(t)
	for _, major := range slices.Sorted(maps.Keys(operands)) {
		t.Run("PG"+major, func(t *testing.T) {
			const dbname = "audit_prod"
			namespace, asOwner := startCNPGDatabase(t, cl, kubeconfig, "bex-pgaudit-it", major, dbname, "audit_app", nil)
			if out, err := asOwner("CREATE EXTENSION pgaudit; SELECT rolsuper FROM pg_roles WHERE rolname = current_user"); err != nil || !strings.HasSuffix(out, "f") {
				t.Fatalf("the owner's CREATE EXTENSION pgaudit = %v %q", err, out)
			}
			ownerPgauditAcrossRestartOnCNPG(t, cl, kubeconfig, namespace, dbname, asOwner)
		})
	}
}

// operandQualificationCluster connects to the CloudNativePG cluster
// BEX_CNPG_KUBECONFIG names, checks it runs the qualified controller, and
// adopts BEX_PG_OPERAND_IMAGES (narrowed by BEX_CNPG_MAJORS) as the operands
// for the test. It skips the test when either variable is unset.
func operandQualificationCluster(t *testing.T) (client.Client, string, map[string]string) {
	t.Helper()
	kubeconfig := os.Getenv("BEX_CNPG_KUBECONFIG")
	images := os.Getenv("BEX_PG_OPERAND_IMAGES")
	if kubeconfig == "" || images == "" {
		t.Skip("set BEX_CNPG_KUBECONFIG and BEX_PG_OPERAND_IMAGES")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	cl, err := client.New(cfg, client.Options{})
	if err != nil {
		t.Fatal(err)
	}
	operands, err := qualificationOperands(images, os.Getenv("BEX_CNPG_MAJORS"))
	if err != nil {
		t.Fatal(err)
	}
	verifyCNPGOperandController(t, cl, cfg.Host)
	withBexOperands(t, operands)
	return cl, kubeconfig, operands
}

// ownerContribOnCNPG: Render's untrusted contrib modules, created by the
// owner as Render's guide writes them, leave the owner a non-superuser.
func ownerContribOnCNPG(t *testing.T, asOwner func(string) (string, error)) {
	t.Helper()
	for _, step := range []struct{ sql, want string }{
		{"CREATE EXTENSION cube; CREATE EXTENSION earthdistance; SELECT round(earth_distance(ll_to_earth(0,0), ll_to_earth(0,1)))", "111320"},
		{"CREATE EXTENSION moddatetime; CREATE TABLE notes (id int, modified timestamptz NOT NULL DEFAULT 'epoch');" +
			" CREATE TRIGGER notes_modified BEFORE UPDATE ON notes FOR EACH ROW EXECUTE PROCEDURE moddatetime(modified);" +
			" INSERT INTO notes VALUES (1); UPDATE notes SET id = 2; SELECT modified > 'epoch' FROM notes", "t"},
		{"CREATE EXTENSION dblink; CREATE EXTENSION bloom; CREATE EXTENSION pgaudit; SELECT rolsuper FROM pg_roles WHERE rolname = current_user", "f"},
	} {
		if out, err := asOwner(step.sql); err != nil || !strings.HasSuffix(out, step.want) {
			t.Fatalf("%s = %v %q, want suffix %q", step.sql, err, out, step.want)
		}
	}
}

// ownerTimescaleOnCNPG: TimescaleDB is carried and preloaded but admitted
// only once its own gate passes. Admitted, the owner builds a hypertable;
// until then the owner meets PostgreSQL's refusal. It reports whether the
// database now holds TimescaleDB.
func ownerTimescaleOnCNPG(t *testing.T, major string, asOwner func(string) (string, error)) bool {
	t.Helper()
	if !slices.Contains(pgExtensionAllowlist[major], "timescaledb") {
		if out, err := asOwner("CREATE EXTENSION timescaledb"); err == nil || !strings.Contains(out, "permission denied to create extension") {
			t.Fatalf("CREATE EXTENSION timescaledb while not admitted = %v %q, want PostgreSQL's permission error", err, out)
		}
		return false
	}
	out, err := asOwner("CREATE EXTENSION timescaledb; CREATE TABLE metrics (at timestamptz NOT NULL, v float8);" +
		" SELECT count(*) FROM create_hypertable('metrics', 'at'); INSERT INTO metrics VALUES (now(), 1);" +
		" SELECT count(*) FROM metrics")
	if err != nil || !strings.HasSuffix(out, "1") {
		t.Fatalf("owner hypertable = %v %q", err, out)
	}
	return true
}

// TestUnversionedDatabasesMoveOnlyOntoTheirOwnMajorsOperand: a Database with
// no spec.version runs CNPG's default image (production's unversioned
// Databases run 18.x). It joins the bex operand of the major it actually runs
// once that is adopted, so it is not left without the extensions, but it never
// moves before that and never onto another major's operand.
func TestUnversionedDatabasesMoveOnlyOntoTheirOwnMajorsOperand(t *testing.T) {
	plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{})
	project := func(running string) (string, map[string]any) {
		spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: 1, runningVersion: running, creating: running == "", dbname: "app", owner: "app"})
		image, _ := spec["imageName"].(string)
		return image, spec["postgresql"].(map[string]any)["parameters"].(map[string]any)
	}
	withBexOperands(t, map[string]string{})
	if image, params := project("18"); image != "" || params["local_preload_libraries"] != nil {
		t.Errorf("before adoption: imageName %q, local_preload %v, want CNPG's default left alone", image, params["local_preload_libraries"])
	}

	const bex = "ghcr.io/bex-co/bex-operator:18.6-postgres-abc@sha256:0123"
	withBexOperands(t, map[string]string{"18": bex})
	for _, running := range []string{"18", ""} { // "" = the Cluster is being created
		image, params := project(running)
		if image != bex || params["extwlist.extensions"] != strings.Join(pgExtensionAllowlist["18"], ",") {
			t.Errorf("running %q: imageName %q, allowlist %v, want the PG18 operand and its list", running, image, params["extwlist.extensions"])
		}
	}
	// An existing cluster whose major is not observed yet keeps its image: a
	// PG18 operand on a PG17 cluster would ask CNPG for a major upgrade.
	existing := cnpgClusterSpec(clusterParams{plan: plan, storageGB: 1, dbname: "app", owner: "app"})
	if image, _ := existing["imageName"].(string); image != "" {
		t.Errorf("existing cluster, unobserved major: imageName %q, want it left alone", image)
	}
	if image, params := project("14"); image != "" || params["local_preload_libraries"] != nil {
		t.Errorf("running 14 with only a PG18 operand: imageName %q, local_preload %v, want unchanged", image, params["local_preload_libraries"])
	}
}
