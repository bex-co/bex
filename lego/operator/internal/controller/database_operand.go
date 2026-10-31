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
	"cmp"
	"strings"
)

// bexOperandImages are the published bex managed-Postgres operand images
// (ADR092 D1: deploy/pg-operand, .github/workflows/pg-operand-image.yml),
// adopted per major by a reviewed commit (ADR060 D7). A major listed here
// serves and exports from this image instead of cnpgExportImages' upstream
// one, and its clusters get the extension allowlist (pgExtensionParameters).
// Adopting a digest restarts every managed cluster of that major once, which
// runs off-peak with health checks and a recorded rollback (ADR092 D4).
var bexOperandImages = map[string]string{}

// pgExtensionAllowlist is ADR092 D1's table: the non-trusted extensions a
// database owner may CREATE EXTENSION on each major, as Render documents them
// (render.com/docs/postgresql-extensions). pg_duckdb ships in the 16+ images
// but stays off the list until w8/m109 settles its file/network access and
// memory guardrails.
// deploy/pg-operand/matrix.json carries the same lists for the image gate;
// TestPGExtensionAllowlistMatchesTheImageGate keeps them equal.
var pgExtensionAllowlist = map[string][]string{
	"13": renderExtensions(13), "14": renderExtensions(14), "15": renderExtensions(15),
	"16": renderExtensions(16), "17": renderExtensions(17), "18": renderExtensions(18),
}

// renderExtensions applies Render's per-major restrictions to the six.
func renderExtensions(major int) []string {
	list := []string{"postgis", "postgis_raster", "postgis_topology", "postgis_tiger_geocoder"}
	if major >= 14 {
		list = append(list, "pg_partman")
	}
	list = append(list, "pg_ivm")
	if major < 16 {
		list = append(list, "pg_similarity")
	}
	return append(list, "timescaledb")
}

// pgExtensionPreloadLibraries join pg_stat_statements in
// shared_preload_libraries on a bex operand. pg_partman_bgw is deliberately
// absent: Render runs pg_partman without its background worker.
var pgExtensionPreloadLibraries = []any{"timescaledb"}

// cnpgOperandImage resolves the serving image for a major and whether it is a
// bex operand carrying the extensions. Unknown and empty majors resolve to ""
// so the caller leaves CNPG's own default in place.
func cnpgOperandImage(version string) (image string, bexOperand bool) {
	if image, ok := bexOperandImages[version]; ok {
		return image, true
	}
	return cnpgExportImages[version], false
}

// cnpgDefaultMajor is the major of the default operand image of the
// CloudNativePG release bex pins (1.30: 18). An unversioned Database is created
// on it. Bump it with the CNPG chart when the default major moves.
const cnpgDefaultMajor = "18"

// unversionedOperandImage resolves the serving image for a Database with no
// spec.version. Such a cluster runs CNPG's own default image. It moves onto a
// bex operand only for the major it is known to run: status.currentVersion,
// or CNPG's default major while the Cluster is first created. Then it gets the
// same single restart as every other cluster of that major (ADR092 D4). An
// existing cluster whose major is not yet observed is left alone, because a
// wrong-major image would ask CNPG for a major upgrade. "" leaves CNPG's
// default in place.
func unversionedOperandImage(running string, creating bool) (image, major string) {
	if running == "" && !creating {
		return "", ""
	}
	major = cmp.Or(running, cnpgDefaultMajor)
	return bexOperandImages[major], major
}

// pgExtensionParameters are the postgresql.conf settings that make a bex
// operand's allowlist work (ADR092 D3). They are the platform's, applied after
// the tenant's parameters so none can be overridden: pgextwlist's GUCs are
// superuser-only inside the server, and here they are never tenant-supplied.
func pgExtensionParameters(version string) map[string]any {
	return map[string]any{
		// local_preload_libraries loads pgextwlist into every session from
		// $libdir/plugins.
		"local_preload_libraries":             "pgextwlist",
		"extwlist.extensions":                 strings.Join(pgExtensionAllowlist[version], ","),
		"extwlist.restrict_to_database_owner": "on",
		// The image's after-create grants that let the owner use what an
		// extension created as the bootstrap superuser.
		"extwlist.custom_path": "/usr/share/bex/extwlist",
		// Render excludes TimescaleDB's Community features. The image carries
		// no TSL module either; this keeps the server's own refusal explicit.
		"timescaledb.license":         "apache",
		"timescaledb.telemetry_level": "off",
	}
}
