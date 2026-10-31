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
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPGExtensionAllowlistMatchesTheImageGate keeps the operator's allowlist
// and the publish workflow's real-server gate (deploy/pg-operand/matrix.json)
// on one list per major, for every major the CRD permits: an extension the
// gate never created must not be admitted, and one the gate proved must not
// be silently left out.
func TestPGExtensionAllowlistMatchesTheImageGate(t *testing.T) {
	raw, err := os.ReadFile("../../../../deploy/pg-operand/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var matrix map[string]struct {
		Base       string   `json:"base"`
		Extensions []string `json:"extensions"`
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
		if !slices.Equal(matrix[v].Extensions, pgExtensionAllowlist[v]) {
			t.Errorf("PG%s: matrix.json extensions %v != pgExtensionAllowlist %v", v, matrix[v].Extensions, pgExtensionAllowlist[v])
		}
		if !strings.Contains(matrix[v].Base, ":"+v+".") || !strings.Contains(matrix[v].Base, "@sha256:") {
			t.Errorf("PG%s: base %q is not a digest-pinned PG%s image", v, matrix[v].Base, v)
		}
	}
}

// TestCheckShConfiguresTheProjectedSettings: the publish gate's server must be
// configured exactly as the operator projects a bex operand cluster, or the
// gate proves a different allowlist than production runs.
func TestCheckShConfiguresTheProjectedSettings(t *testing.T) {
	raw, err := os.ReadFile("../../../../deploy/pg-operand/check.sh")
	if err != nil {
		t.Fatal(err)
	}
	conf := string(raw)
	conf = conf[strings.Index(conf, ">>\"$data/postgresql.conf\" <<EOF\n"):]
	conf = conf[strings.Index(conf, "\n")+1 : strings.Index(conf, "\nEOF\n")]
	got := map[string]string{}
	for line := range strings.SplitSeq(conf, "\n") {
		k, v, _ := strings.Cut(line, " = ")
		got[k] = strings.Trim(v, "'")
	}
	want := map[string]string{"listen_addresses": "", "unix_socket_directories": "/tmp"}
	preload := "pg_stat_statements"
	for _, lib := range pgExtensionPreloadLibraries {
		preload += "," + lib.(string)
	}
	want["shared_preload_libraries"] = preload
	for k, v := range pgExtensionParameters("18") {
		want[k] = v.(string)
	}
	want["extwlist.extensions"] = "${allowlist}"
	if !maps.Equal(got, want) {
		t.Errorf("check.sh postgresql.conf = %v\nwant the operator's projection %v", got, want)
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
		"extwlist.extensions":     "file_fdw",
		"local_preload_libraries": "",
		"timescaledb.license":     "timescale",
	}

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
	if !slices.Equal(preload, []any{"pg_stat_statements", "timescaledb"}) {
		t.Errorf("shared_preload_libraries = %v, want pg_stat_statements + timescaledb (no pg_partman_bgw)", preload)
	}
	want := map[string]any{
		"local_preload_libraries":             "pgextwlist",
		"extwlist.extensions":                 "postgis,postgis_raster,postgis_topology,postgis_tiger_geocoder,pg_partman,pg_ivm,timescaledb",
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
	}
	if got := cnpgExportImage("17"); got != cnpgExportImages["17"] {
		t.Errorf("cnpgExportImage(17) = %q, want the upstream pin", got)
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
	operands := map[string]string{}
	for pair := range strings.SplitSeq(images, ",") {
		major, image, ok := strings.Cut(pair, "=")
		if !ok {
			t.Fatalf("BEX_PG_OPERAND_IMAGES entry %q is not <major>=<image>", pair)
		}
		operands[major] = image
	}
	withBexOperands(t, operands)

	for _, major := range slices.Sorted(maps.Keys(operands)) {
		t.Run("PG"+major, func(t *testing.T) {
			// A tenant's own parameter override rides along, as it would in
			// production, and cannot widen the list.
			_, asOwner := startCNPGDatabase(t, cl, kubeconfig, "bex-operand-it", major, "geo_prod", "geo_app",
				map[string]string{"extwlist.extensions": "file_fdw", "work_mem": "8MB"})

			// m92's original migration, unchanged, from the owner's connection.
			migration, err := os.ReadFile("testdata/postgis-migration.sql")
			if err != nil {
				t.Fatal(err)
			}
			out, err := asOwner(string(migration))
			if err != nil || !strings.Contains(out, `"distance_m": 111319.491`) || !strings.Contains(out, `"wkt": "POINT(0 0)"`) {
				t.Fatalf("original PostGIS migration = %v %q", err, out)
			}

			steps := []struct{ sql, want string }{
				{"SELECT rolsuper FROM pg_roles WHERE rolname = current_user", "f"},
				{"SHOW extwlist.extensions", strings.Join(pgExtensionAllowlist[major], ",")},
				{"SHOW work_mem", "8MB"},
				{"CREATE EXTENSION postgis; CREATE TABLE places (id int, geom geometry(Point, 4326));" +
					" INSERT INTO places SELECT g, ST_SetSRID(ST_MakePoint(g, g), 4326) FROM generate_series(1, 50) g;" +
					" CREATE INDEX ON places USING gist (geom);" +
					" SELECT count(*) FROM places WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint(10, 10), 4326), 1.5)", "3"},
				{"CREATE EXTENSION timescaledb; CREATE TABLE metrics (at timestamptz NOT NULL, v float8);" +
					" SELECT count(*) FROM create_hypertable('metrics', 'at'); INSERT INTO metrics VALUES (now(), 1);" +
					" SELECT count(*) FROM metrics", "1"},
				{"CREATE EXTENSION hstore; SELECT 'a=>1'::hstore -> 'a'", "1"},
				{"SELECT rolsuper FROM pg_roles WHERE rolname = current_user", "f"},
			}
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
			for _, refused := range []string{"CREATE EXTENSION file_fdw", "CREATE EXTENSION dblink", "SET extwlist.extensions = 'file_fdw'"} {
				if out, err := asOwner(refused); err == nil || !strings.Contains(out, "permission denied") {
					t.Errorf("%s = %v %q, want permission denied", refused, err, out)
				}
			}
		})
	}
}
