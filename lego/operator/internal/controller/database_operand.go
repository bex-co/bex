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
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// bexOperandImages are the published bex managed-Postgres operand images
// (ADR092 D1: deploy/pg-operand, .github/workflows/pg-operand-image.yml),
// adopted per major by a reviewed commit (ADR060 D7). A major listed here
// serves and exports from this image instead of cnpgExportImages' upstream
// one, and its clusters get the extension allowlist (pgExtensionParameters).
// Adopting a digest restarts every managed cluster of that major once, which
// runs off-peak with health checks and a recorded rollback (ADR092 D4).
//
// Adopted 2026-11-23 (w21/m584): pg-operand-image.yml run 45254127793 gated
// each digest on arm64 and natively on amd64, with TimescaleDB held out of
// the allowlist. The rollback is this map emptied, which returns every major
// to cnpgExportImages and its upstream projection (one more restart).
var bexOperandImages = map[string]string{
	"13": "ghcr.io/bex-co/bex-operator:13.22-postgres-6c11b6b0f678b9db48bcba9fc3c2c8891e073c11@sha256:95f569d5374831da375b3238a36399b9724dbe8a084b694c1ef15bb679a04e1f",
	"14": "ghcr.io/bex-co/bex-operator:14.25-postgres-6c11b6b0f678b9db48bcba9fc3c2c8891e073c11@sha256:da5906d9eae2658f5505bbf9bf18cf0dd953d8d677395d674176259b13a433d5",
	"15": "ghcr.io/bex-co/bex-operator:15.20-postgres-6c11b6b0f678b9db48bcba9fc3c2c8891e073c11@sha256:4f6dc525a354a0eb3d1164bc294915505649bf70abe94abf4be69b5de4f7d9a3",
	"16": "ghcr.io/bex-co/bex-operator:16.16-postgres-6c11b6b0f678b9db48bcba9fc3c2c8891e073c11@sha256:ee398edf10728f3fb0df46025a9fdf22361b41247aa3cd70541be61c00ffd1f0",
	"17": "ghcr.io/bex-co/bex-operator:17.12-postgres-6c11b6b0f678b9db48bcba9fc3c2c8891e073c11@sha256:8a68db3ece72a3a34d5f79c7fe7a98c96c423f83f731d5dd0c9924a5418ceefa",
	"18": "ghcr.io/bex-co/bex-operator:18.7-postgres-6c11b6b0f678b9db48bcba9fc3c2c8891e073c11@sha256:be9029d668214614cbec665c3c5f9de9d526295cdb4b4201a3890787490fd8fc",
}

const postgresDisabled = "false"

// pgExtensionAllowlist is the extwlist.extensions value per major: the
// non-trusted extensions a database owner may CREATE EXTENSION, as Render
// documents them (render.com/docs/postgresql-extensions), that passed their
// own ADR092 D3 gate on that major. DuckDB access and per-connection
// resources are projected with the owner and plan below.
// deploy/pg-operand/matrix.json "admitted" carries the same lists for the
// image gate, which fails when an admitted extension fails or a passing one
// is left out; TestPGExtensionAllowlistMatchesTheImageGate keeps them equal.
var pgExtensionAllowlist = map[string][]string{
	"13": admittedExtensions(13), "14": admittedExtensions(14), "15": admittedExtensions(15),
	"16": admittedExtensions(16), "17": admittedExtensions(17), "18": admittedExtensions(18),
}

// admittedExtensions applies Render's per-major restrictions to the
// extensions the image carries, less those whose gate failed. TimescaleDB is
// carried and preloaded but held out: its ordinary owner cannot run
// timescaledb_pre_restore() ("permission denied to set parameter
// timescaledb.restoring"), so property 5 fails on every major. Admitting an
// extension later is a reload of extwlist.extensions, never a restart
// (ADR092 D3, D4). pgvector comes with CNPG's base image and is not trusted,
// so the owner creates and drops it through this list, as Render documents
// (`CREATE EXTENSION vector;`), and a new database gets none preinstalled
// (reconcileApplicationDatabase, w21/m586).
func admittedExtensions(major int) []string {
	list := []string{"postgis", "postgis_raster", "postgis_topology", "postgis_tiger_geocoder"}
	if major >= 14 {
		list = append(list, "pg_partman")
	}
	list = append(list, "pg_ivm")
	if major < 16 {
		list = append(list, "pg_similarity")
	} else {
		list = append(list, "pg_duckdb", "pg_repack")
	}
	list = append(list, "vector")
	return append(list, untrustedContribExtensions(major)...)
}

// untrustedContribExtensions are the modules on Render's list that the base
// image already carries but PostgreSQL does not mark trusted, on the majors
// where they exist: pg_surgery from 14, adminpack until 16 and old_snapshot on
// 14-16 (PostgreSQL 17 removed both). Each is admitted with PostgreSQL's own
// grants unchanged, so a superuser-only function stays superuser-only (ADR092
// D1, w21/m585). pg_stat_statements is also what CloudNativePG creates as the
// superuser in every database for query insights (pg_stat_statements.track);
// admitting it is what lets the owner's restore of its own export run the
// dump's COMMENT ON EXTENSION pg_stat_statements, and the owner drop it and run
// Render's CREATE EXTENSION pg_stat_statements (w21/124).
func untrustedContribExtensions(major int) []string {
	list := []string{"amcheck", "autoinc", "bloom", "dblink", "dict_xsyn", "earthdistance", "file_fdw",
		"insert_username", "intagg", "moddatetime", "pageinspect", "pg_buffercache", "pg_freespacemap", "pg_prewarm",
		"pg_stat_statements"}
	if major >= 14 {
		list = append(list, "pg_surgery")
	}
	list = append(list, "pg_visibility", "pgaudit", "pgrowlocks", "pgstattuple", "refint", "sslinfo", "xml2")
	if major <= 16 {
		list = append(list, "adminpack")
	}
	if major >= 14 && major <= 16 {
		list = append(list, "old_snapshot")
	}
	return list
}

// pgExtensionPreloadLibraries join pg_stat_statements in
// shared_preload_libraries on a bex operand: every library the image carries
// that must be preloaded, whether or not its extension is admitted yet, so a
// later admission needs no restart. pg_partman_bgw is deliberately absent:
// Render runs pg_partman without its background worker.
func pgExtensionPreloadLibraries(version string) []any {
	// pg_ivm's preload is required for maintenance even in sessions that did
	// not themselves CREATE EXTENSION. pgextwlist remains a local preload.
	libraries := []any{"timescaledb", "pg_ivm"}
	if version == "16" || version == "17" || version == "18" {
		libraries = append(libraries, "pg_duckdb")
	}
	// pgaudit refuses to load from anywhere but shared_preload_libraries. It
	// is listed here rather than enabled through CloudNativePG's pgaudit.*
	// parameters, which would also CREATE EXTENSION pgaudit in every database
	// (ADR092 D2).
	return append(libraries, "pgaudit")
}

func managedExtensionParameter(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "local_preload_libraries" || name == "session_preload_libraries" || name == "output_plugin_libraries" ||
		strings.HasPrefix(name, "extwlist.") || strings.HasPrefix(name, "timescaledb.") || strings.HasPrefix(name, "duckdb.")
}

// cnpgOperandImage resolves the serving image for a major and whether it is a
// bex operand carrying the extensions. Unknown and empty majors resolve to ""
// so the caller leaves CNPG's own default in place.
func cnpgOperandImage(version string) (image string, bexOperand bool) {
	if image, ok := bexOperandImages[version]; ok {
		return image, true
	}
	return cnpgExportImages[version], false
}

func effectivePostgresVersion(spec, observed string, creating bool) string {
	version := cmp.Or(spec, observed)
	if version == "" && creating {
		version = appv1alpha1.DefaultPostgresVersion
	}
	return version
}

func adoptedOperandMajor(image string) string {
	for major, adopted := range bexOperandImages {
		if adopted != "" && adopted == image {
			return major
		}
	}
	return ""
}

// unversionedOperandImage resolves the serving image for a Database with no
// spec.version. Such a cluster runs CNPG's own default image. It moves onto a
// bex operand only for the major it is known to run: status.currentVersion,
// or CNPG's default major while the Cluster is first created. Then it gets the
// same single restart as every other cluster of that major (ADR092 D4). An
// existing cluster whose major is not yet observed is left alone, because a
// wrong-major image would ask CNPG for a major upgrade. "" leaves CNPG's
// default in place.
func unversionedOperandImage(running string, creating bool) (image, major string) {
	major = effectivePostgresVersion("", running, creating)
	if major == "" {
		return "", ""
	}
	return bexOperandImages[major], major
}

// pgExtensionParameters are the postgresql.conf settings that make a bex
// operand's allowlist work (ADR092 D3). They are the platform's, applied after
// the tenant's parameters so none can be overridden: pgextwlist's GUCs are
// superuser-only inside the server, and here they are never tenant-supplied.
func pgExtensionParameters(p clusterParams) map[string]any {
	parameters := map[string]any{
		// local_preload_libraries loads pgextwlist into every session from
		// $libdir/plugins.
		"local_preload_libraries":             "pgextwlist",
		"extwlist.extensions":                 strings.Join(pgExtensionAllowlist[p.version], ","),
		"extwlist.restrict_to_database_owner": "on",
		// The image's after-create grants that let the owner use what an
		// extension created as the bootstrap superuser.
		"extwlist.custom_path": "/usr/share/bex/extwlist",
		// Render excludes TimescaleDB's Community features. The image carries
		// no TSL module either; this keeps the server's own refusal explicit.
		"timescaledb.license":         "apache",
		"timescaledb.telemetry_level": "off",
	}
	// wal2json is the logical-decoding output plugin Render lists. The minor
	// releases the 14-18 bases carry refuse any plugin a slot names that is
	// not in output_plugin_libraries (default "pgoutput, test_decoding";
	// postgres/postgres 4e1252ee56). PostgreSQL 13 has no such setting, and
	// an unknown one would stop its server.
	if p.version != "13" {
		parameters["output_plugin_libraries"] = "pgoutput,test_decoding,wal2json"
	}
	memory := resource.MustParse(p.plan.Memory)
	memoryMB := memory.Value() / (1024 * 1024)
	// Leave headroom for the mandatory preloads in the 256Mi Free instance.
	// Preserve explicit user buffer tuning, including canonicalized names.
	buffers := false
	for name := range p.parameters {
		buffers = buffers || strings.EqualFold(strings.TrimSpace(name), "shared_buffers")
	}
	if !buffers {
		parameters["shared_buffers"] = strconv.FormatInt(max(16, memoryMB/4), 10) + "MB"
	}
	if p.version == "16" || p.version == "17" || p.version == "18" {
		// Execution uses the existing owner directly; no privilege-proxy role
		// or extra memberships. Other roles need normal owner authorization.
		if !appv1alpha1.ReservedPostgresRole(p.owner) {
			parameters["duckdb.postgres_role"] = p.owner
		}
		parameters["duckdb.max_memory"] = strconv.FormatInt(max(1, memoryMB/8), 10)
		parameters["duckdb.threads"] = "1"
		parameters["duckdb.enable_external_access"] = postgresDisabled
		parameters["duckdb.autoinstall_known_extensions"] = postgresDisabled
		parameters["duckdb.autoload_known_extensions"] = postgresDisabled
	}
	return parameters
}
