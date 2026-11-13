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

package postgres

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestInsightStatementsCarryTheirTag pins w4/557: the two activity reads leave
// Insights' own tagged statements (runAuthorizedInsight tags each) and every
// reserved role out, so a quiet database's panel never lists the panel itself
// or CNPG's exporter.
func TestInsightStatementsCarryTheirTag(t *testing.T) {
	if got := tagInsight("\nSELECT 1"); got != "SELECT "+insightsTag+" 1" {
		t.Errorf("tagInsight = %q", got)
	}
	for name, sql := range map[string]string{
		"processes": sqlProcesses, "top queries": sqlTopQueries, "database size": sqlDatabaseSize,
		"table sizes": sqlTableSizes, "table scans": sqlTableScans, "parameter overrides": sqlParameterOverrides,
	} {
		if !strings.HasPrefix(strings.TrimLeft(sql, " \t\n"), "SELECT ") {
			t.Errorf("%s statement does not lead with SELECT, so tagInsight would mangle it", name)
		}
	}
	for name, sql := range map[string]string{"processes": sqlProcesses, "top queries": sqlTopQueries} {
		if !strings.Contains(sql, "NOT LIKE '%"+insightsTag+"%'") || !strings.Contains(sql, "starts_with(") {
			t.Errorf("%s statement keeps bex's own or CNPG's statements:\n%s", name, sql)
		}
	}
}

// TestInsightInputGuards covers the paths that don't require a live DB: unknown
// database — all resolved before any dial.
func TestInsightInputGuards(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	if _, err := svc.Processes(ctx, "no-db"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Processes unknown db => %v, want ErrNotFound", err)
	}
	if _, err := svc.Sizes(ctx, "no-db"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Sizes unknown db => %v, want ErrNotFound", err)
	}
	if _, err := svc.TableScans(ctx, "no-db"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("TableScans unknown db => %v, want ErrNotFound", err)
	}
	if _, err := svc.ParameterOverrides(ctx, "no-db"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ParameterOverrides unknown db => %v, want ErrNotFound", err)
	}
	if _, err := svc.TopQueries(ctx, "no-db"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("TopQueries unknown db => %v, want ErrNotFound", err)
	}

}

// TestRESTInsightEndpoints verifies routing for the five GET endpoints. The
// seeded DB uses a non-routable URI so TopQueries returns 503 on a failed dial.
// The bex-only PUT parameter-overrides route was retired at w2/066.
func TestRESTInsightEndpoints(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "ins-db")

	for _, c := range []struct {
		method string
		path   string
		body   string
		want   int
	}{
		// DB exists but URI is a fake internal hostname — dial fails → 500.
		{"GET", "/v1/postgres/ins-db/processes", "", 500},
		{"GET", "/v1/postgres/ins-db/top-queries", "", 503},
		{"GET", "/v1/postgres/ins-db/sizes", "", 500},
		{"GET", "/v1/postgres/ins-db/table-scans", "", 500},
		{"GET", "/v1/postgres/ins-db/parameter-overrides", "", 500},
		// Retired write route (w2/066): the path only serves GET now.
		{"PUT", "/v1/postgres/ins-db/parameter-overrides", `{"parameters":{"work_mem":"16MB"}}`, 405},
		// Unknown database → 404.
		{"GET", "/v1/postgres/nope/processes", "", 404},
		{"GET", "/v1/postgres/nope/sizes", "", 404},
	} {
		code := serveREST(svc, c.method, c.path, c.body).Code
		if code != c.want {
			t.Errorf("%s %s => %d, want %d", c.method, c.path, code, c.want)
		}
	}
}

// TestInsightHelpers unit-tests the strVal/intVal/floatVal extraction helpers.
func TestInsightHelpers(t *testing.T) {
	if strVal(nil) != "" {
		t.Error("strVal(nil) != \"\"")
	}
	if strVal("hello") != "hello" {
		t.Error("strVal(string)")
	}
	if intVal(nil) != 0 {
		t.Error("intVal(nil) != 0")
	}
	if intVal(int64(42)) != 42 {
		t.Error("intVal(int64)")
	}
	if intVal(int32(7)) != 7 {
		t.Error("intVal(int32)")
	}
	if intVal(float64(3.9)) != 3 {
		t.Error("intVal(float64) truncation")
	}
	if floatVal(nil) != 0 {
		t.Error("floatVal(nil) != 0")
	}
	if floatVal(float64(1.5)) != 1.5 {
		t.Error("floatVal(float64)")
	}
	if floatVal(int64(10)) != 10.0 {
		t.Error("floatVal(int64)")
	}
}

// TestGQLInsightFields verifies that the five insight query fields are
// registered in the GraphQL schema, and that the retired
// setDatabaseParameterOverrides mutation (w2/066) is not.
// GraphQLQuery/GraphQLMutation return field maps; no database object is needed.
func TestGQLInsightFields(t *testing.T) {
	svc, _ := newService()
	fields := svc.GraphQLQuery()
	for _, name := range []string{
		"databaseProcesses",
		"databaseTopQueries",
		"databaseSizes",
		"databaseTableScans",
		"databaseParameterOverrides",
	} {
		if _, ok := fields[name]; !ok {
			t.Errorf("GraphQLQuery missing field %q", name)
		}
	}
	if _, ok := svc.GraphQLMutation()["setDatabaseParameterOverrides"]; ok {
		t.Error("GraphQLMutation still registers the retired setDatabaseParameterOverrides")
	}
}

// TestMCPInsightTools verifies that the six insight tools are registered in the
// MCP server under their expected names.
func TestMCPInsightTools(t *testing.T) {
	svc, _ := newService()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)

	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	byName := map[string]bool{}
	for _, tl := range res.Tools {
		byName[tl.Name] = true
	}
	for _, want := range []string{
		"list_postgres_processes",
		"list_postgres_top_queries",
		"get_postgres_sizes",
		"list_postgres_table_scans",
		"list_postgres_parameter_overrides",
		// The parameter-override WRITE folded into update_postgres in w1/m71;
		// the read tool above kept its own name.
		"update_postgres",
	} {
		if !byName[want] {
			t.Errorf("MCP tool %q not registered", want)
		}
	}
}

// --- Live-DB integration (hermetic-by-default) --------------------------------
//
// BEX_TEST_DB_URI points at a throwaway database, e.g.
// `docker run --rm -e POSTGRES_PASSWORD=pw -p 5433:5432 postgres:17`
// → postgres://postgres:pw@localhost:5433/postgres?sslmode=disable

// TestProcessesIntegration asserts that at least one process appears in
// pg_stat_activity. The platform's own sessions are filtered out (w21/026), so
// the list may be empty on a bare test database: it only has to answer cleanly;
// TestProcessesHidePlatformActivityIntegration pins what it shows.
func TestProcessesIntegration(t *testing.T) {
	uri := testDBURI(t)
	svc, _ := newService()
	seedDatabaseAt(t, svc, "proc-live", uri)

	out, err := svc.Processes(context.Background(), "proc-live")
	if err != nil {
		t.Fatalf("Processes => %v", err)
	}
	t.Logf("Processes returned %d rows", len(out))
}

// TestProcessesHidePlatformActivityIntegration pins w21/026: the platform's own
// sessions (role postgres, background workers) never reach the customer's
// Active processes, while a customer role's session does.
func TestProcessesHidePlatformActivityIntegration(t *testing.T) {
	uri := testDBURI(t)
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("admin connect: %v", err)
	}
	defer admin.Close(ctx)
	const role = "w21_026_app"
	_, _ = admin.Exec(ctx, `DROP ROLE IF EXISTS `+role)
	if _, err := admin.Exec(ctx, `CREATE ROLE `+role+` LOGIN PASSWORD 'pw'`); err != nil {
		t.Fatalf("create role: %v", err)
	}
	defer func() { _, _ = admin.Exec(ctx, `DROP ROLE IF EXISTS `+role) }()

	cfg, err := pgx.ParseConfig(uri)
	if err != nil {
		t.Fatal(err)
	}
	cfg.User, cfg.Password = role, "pw"
	app, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("app connect: %v", err)
	}
	defer app.Close(ctx)
	if _, err := app.Exec(ctx, `SELECT 1`); err != nil {
		t.Fatal(err)
	}

	// w4/557: CNPG's exporter has its own cnpg_ login, and Insights' own
	// statements run as the owner; neither is the customer's work.
	const exporter = "cnpg_w4_557_exporter"
	_, _ = admin.Exec(ctx, `DROP ROLE IF EXISTS `+exporter)
	if _, err := admin.Exec(ctx, `CREATE ROLE `+exporter+` LOGIN PASSWORD 'pw'`); err != nil {
		t.Fatalf("create exporter role: %v", err)
	}
	defer func() { _, _ = admin.Exec(ctx, `DROP ROLE IF EXISTS `+exporter) }()
	exporterCfg := cfg.Copy()
	exporterCfg.User = exporter
	scraper, err := pgx.ConnectConfig(ctx, exporterCfg)
	if err != nil {
		t.Fatalf("exporter connect: %v", err)
	}
	defer scraper.Close(ctx)
	if _, err := scraper.Exec(ctx, `SELECT 2`); err != nil {
		t.Fatal(err)
	}
	insight, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("second app connect: %v", err)
	}
	defer insight.Close(ctx)
	if _, err := insight.Exec(ctx, tagInsight(`SELECT 3`)); err != nil {
		t.Fatal(err)
	}

	svc, _ := newService()
	seedDatabaseAt(t, svc, "proc-platform", uri)
	out, err := svc.Processes(ctx, "proc-platform")
	if err != nil {
		t.Fatalf("Processes => %v", err)
	}
	var sawApp bool
	for _, p := range out {
		switch {
		case strings.Contains(p.Query, insightsTag):
			t.Errorf("Insights' own statement leaked into Processes: %+v", p)
		case p.UserName == role:
			sawApp = true
		case p.UserName == "" || appv1alpha1.ReservedPostgresRole(p.UserName):
			t.Errorf("platform activity leaked into Processes: %+v", p)
		}
	}
	if !sawApp {
		t.Errorf("the customer role's session is missing: %+v", out)
	}
}

// TestReservedRoleSQLMatchesTheOperatorRuleIntegration pins w4/557: the SQL
// predicate Insights filters on says exactly what ReservedPostgresRole says
// (a LIKE 'pg_%' would also have hidden `pgxuser`).
func TestReservedRoleSQLMatchesTheOperatorRuleIntegration(t *testing.T) {
	uri := testDBURI(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	for _, name := range []string{
		"postgres", "streaming_replica", "cnpg_metrics_exporter", "cnpg_pooler_pgbouncer", "pg_monitor",
		"pgxuser", "cnpgxuser", "app_user", "dpg_x_user",
	} {
		var got bool
		if err := conn.QueryRow(ctx, "SELECT "+reservedRoleSQL("$1::text"), name).Scan(&got); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := appv1alpha1.ReservedPostgresRole(name); got != want {
			t.Errorf("reservedRoleSQL(%q) = %v, ReservedPostgresRole = %v", name, got, want)
		}
	}
}

// TestTopQueriesIntegration exercises pg_stat_statements (may be absent on the
// test DB — empty list without error is the expected outcome).
func TestTopQueriesIntegration(t *testing.T) {
	uri := testDBURI(t)
	// Touch the DB so pg_stat_statements has at least one entry if it's installed.
	admin, err := pgx.Connect(context.Background(), uri)
	if err != nil {
		t.Fatalf("admin connect: %v", err)
	}
	_, _ = admin.Exec(context.Background(), `SELECT 1`)
	var hasStatements bool
	if err := admin.QueryRow(context.Background(), `SELECT to_regclass('pg_stat_statements') IS NOT NULL`).Scan(&hasStatements); err != nil {
		t.Fatal(err)
	}
	admin.Close(context.Background())

	svc, _ := newService()
	seedDatabaseAt(t, svc, "topq-live", uri)

	out, err := svc.TopQueries(context.Background(), "topq-live")
	if !hasStatements {
		if !errors.Is(err, core.ErrUnavailable) {
			t.Fatalf("missing extension error = %v", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("TopQueries => %v", err)
	}
	t.Logf("TopQueries returned %d rows", len(out))
}

// TestTopQueriesLeavesTheReadOnlyEnvelopeOut pins w21/058: on a database with no
// application traffic, Top queries listed Insights' own `begin read only` and
// `rollback`. The test database has no pg_stat_statements (it is not preloaded),
// so a table of the same columns stands in for the view; the statement under test
// is the real sqlTopQueries, run through the real read path.
func TestTopQueriesLeavesTheReadOnlyEnvelopeOut(t *testing.T) {
	uri := testDBURI(t)
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("admin connect: %v", err)
	}
	defer admin.Close(ctx)
	var present bool
	if err := admin.QueryRow(ctx, `SELECT to_regclass('pg_stat_statements') IS NOT NULL`).Scan(&present); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Skip("the database has a real pg_stat_statements; the stand-in table would not be the one read")
	}
	if _, err := admin.Exec(ctx, `CREATE TABLE public.pg_stat_statements (
		userid oid, query text, calls bigint, total_exec_time double precision,
		mean_exec_time double precision, rows bigint, shared_blks_hit bigint, shared_blks_read bigint)`); err != nil {
		t.Fatalf("create stand-in: %v", err)
	}
	// The test login is a reserved role (postgres), which the read leaves out by
	// design, so the rows belong to a customer-like role.
	if _, err := admin.Exec(ctx, `DROP ROLE IF EXISTS topq_customer`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `CREATE ROLE topq_customer`); err != nil {
		t.Fatalf("create role: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, uri)
		if err != nil {
			return
		}
		defer c.Close(ctx)
		_, _ = c.Exec(ctx, `DROP TABLE IF EXISTS public.pg_stat_statements`)
		_, _ = c.Exec(ctx, `DROP ROLE IF EXISTS topq_customer`)
	})
	// Largest total time first: the envelope statements outrank the customer's.
	if _, err := admin.Exec(ctx, `INSERT INTO public.pg_stat_statements
		SELECT (SELECT oid FROM pg_roles WHERE rolname = 'topq_customer'), q, 1, t, t, 0, 0, 0
		FROM (VALUES
			('begin read only', 90.0), ('rollback', 80.0),
			('SELECT id FROM orders', 70.0), ('ROLLBACK', 60.0), ('BEGIN', 50.0),
			('SELECT `+insightsTag+` 1', 40.0)) AS v(q, t)`); err != nil {
		t.Fatalf("seed stand-in: %v", err)
	}

	svc, _ := newService()
	seedDatabaseAt(t, svc, "topq-envelope", uri)
	rows, err := svc.TopQueries(ctx, "topq-envelope")
	if err != nil {
		t.Fatalf("TopQueries => %v", err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Query)
	}
	// The customer's statement stays, in every spelling that is not the envelope's;
	// the envelope's two and Insights' own tagged SELECT do not.
	want := []string{"SELECT id FROM orders", "ROLLBACK", "BEGIN"}
	if !slices.Equal(got, want) {
		t.Fatalf("Top queries = %q, want %q", got, want)
	}
}

// TestSizesIntegration exercises pg_database_size / pg_total_relation_size.
func TestSizesIntegration(t *testing.T) {
	uri := testDBURI(t)
	svc, _ := newService()
	seedDatabaseAt(t, svc, "size-live", uri)

	v, err := svc.Sizes(context.Background(), "size-live")
	if err != nil {
		t.Fatalf("Sizes => %v", err)
	}
	if v.Database.SizeBytes <= 0 || v.Database.SizePretty == "" {
		t.Errorf("database size missing: %+v", v.Database)
	}
	t.Logf("db=%q size=%s tables=%d", v.Database.Name, v.Database.SizePretty, len(v.Tables))
}

// TestTableScansIntegration exercises pg_stat_user_tables.
func TestTableScansIntegration(t *testing.T) {
	uri := testDBURI(t)
	admin, err := pgx.Connect(context.Background(), uri)
	if err != nil {
		t.Fatalf("admin connect: %v", err)
	}
	_, _ = admin.Exec(context.Background(),
		`DROP TABLE IF EXISTS scan_test; CREATE TABLE scan_test(id int); SELECT * FROM scan_test`)
	admin.Close(context.Background())

	svc, _ := newService()
	seedDatabaseAt(t, svc, "scan-live", uri)

	out, err := svc.TableScans(context.Background(), "scan-live")
	if err != nil {
		t.Fatalf("TableScans => %v", err)
	}
	for _, r := range out {
		if r.Name == "scan_test" {
			t.Logf("scan_test: seq=%d idx=%d", r.SeqScans, r.IndexScans)
			return
		}
	}
	t.Log("scan_test not in pg_stat_user_tables (ok if stats haven't flushed yet)")
}

// TestParameterOverridesIntegration exercises pg_settings on a real DB.
func TestParameterOverridesIntegration(t *testing.T) {
	uri := testDBURI(t)
	svc, _ := newService()
	seedDatabaseAt(t, svc, "pov-live", uri)

	out, err := svc.ParameterOverrides(context.Background(), "pov-live")
	if err != nil {
		t.Fatalf("ParameterOverrides => %v", err)
	}
	t.Logf("ParameterOverrides returned %d rows", len(out))
	// w4/185: the probe's own session rails must not read back as the
	// database's settings — a declared statement_timeout looked unapplied
	// because the probe reported its own 10000 ms from source "client".
	for _, row := range out {
		if row.Source == "client" && (row.Name == "statement_timeout" || row.Name == "default_transaction_read_only") {
			t.Errorf("ParameterOverrides published the probe session's own %s=%s (source client)", row.Name, row.Setting)
		}
	}
}

// The pg_settings read dials without the session rails; every other insight
// and the console keep them (TestInsightConnectionKeepsItsReadOnlyRails).
func TestServerSettingsReadDialsWithoutSessionRails(t *testing.T) {
	cfg, err := buildQueryConnConfig("postgres://u:p@localhost:5432/db?sslmode=disable",
		queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap, serverSettings: true}, true)
	if err != nil {
		t.Fatalf("buildQueryConnConfig => %v", err)
	}
	for _, k := range []string{"statement_timeout", "default_transaction_read_only"} {
		if v, ok := cfg.RuntimeParams[k]; ok {
			t.Errorf("server-settings read pins %s=%q; pg_settings would report it as the database's", k, v)
		}
	}
}

// TestInsightQueryTimeout verifies that the statement-timeout envelope applies
// to insight SQL (same infrastructure as Query but exercised separately).
func TestInsightQueryTimeout(t *testing.T) {
	uri := testDBURI(t)
	_, err := runReadOnlyQuery(context.Background(), uri, "SELECT pg_sleep(5)",
		queryLimits{statementTimeout: 200 * time.Millisecond, rowCap: 10})
	if !errors.Is(err, errQueryTimeout) {
		t.Errorf("insight timeout => %v, want errQueryTimeout", err)
	}
}

// TestParameterSpecIsTheDeclaredSet covers the read w6/m133 added: the tenant's
// declared overrides, sorted, empty for a database nobody has configured. The
// set is read-only since parameterOverrides was retired (w2/066), so the test
// seeds it on the CR the way a pre-retirement database carries it.
func TestParameterSpecIsTheDeclaredSet(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "spec-db")
	ctx := context.Background()

	// A database nobody has configured declares nothing. The pg_settings view
	// would report ~48 rows for the same database.
	fresh, err := svc.ParameterSpec(ctx, "spec-db")
	if err != nil {
		t.Fatalf("ParameterSpec => %v", err)
	}
	if len(fresh) != 0 {
		t.Fatalf("ParameterSpec on a fresh database = %v, want empty", fresh)
	}

	db := getDatabase(t, cl, "spec-db")
	db.Spec.Parameters = map[string]string{
		"work_mem":                  "8MB",
		"effective_cache_size":      "1GB",
		"default_statistics_target": "200",
	}
	if err := cl.Update(ctx, db); err != nil {
		t.Fatalf("seed spec.parameters => %v", err)
	}

	got, err := svc.ParameterSpec(ctx, "spec-db")
	if err != nil {
		t.Fatalf("ParameterSpec => %v", err)
	}
	// Name-sorted, so a reader does not reshuffle between reads.
	want := []ParameterSpecView{
		{Name: "default_statistics_target", Value: "200", ObservationStatus: "unavailable"},
		{Name: "effective_cache_size", Value: "1GB", ObservationStatus: "unavailable"},
		{Name: "work_mem", Value: "8MB", ObservationStatus: "unavailable"},
	}
	if len(got) != len(want) {
		t.Fatalf("ParameterSpec = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ParameterSpec[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	if _, err := svc.ParameterSpec(ctx, "no-db"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ParameterSpec unknown db => %v, want ErrNotFound", err)
	}
}

// TestOperatorManagedParameterNames pins the platform-owned name table w6/m133
// introduced. It once refused tenant overrides; since parameterOverrides was
// retired (w2/066) it keeps a recovery from carrying a legacy source's
// platform-managed settings into the restore (tenantParameters, w4/360).
func TestOperatorManagedParameterNames(t *testing.T) {
	managed := []string{
		"restore_command", "ssl_key_file", "archive_command", "archive_mode", "wal_level",
		// Prefix-matched families the exact map never names.
		"ssl_ciphers", "recovery_target_time", "archive_cleanup_command", "syslog_facility",
		// Replication / pod control.
		"listen_addresses", "port", "restart_after_crash", "allow_alter_system",
		"pg_stat_statements.track",
		// The managed extension allowlist (ADR092 D3).
		"extwlist.extensions", "extwlist.restrict_to_database_owner",
		"local_preload_libraries", "session_preload_libraries",
		"timescaledb.license", "duckdb.enable_external_access",
	}
	for _, name := range managed {
		if got := operatorManagedParameterNames(map[string]string{name: "x", "work_mem": "8MB"}); len(got) != 1 || got[0] != name {
			t.Errorf("operatorManagedParameterNames(%q) = %v, want [%s]", name, got, name)
		}
	}
	// Case and whitespace fold to the canonical name.
	if got := operatorManagedParameterNames(map[string]string{"  ARCHIVE_COMMAND  ": "x"}); len(got) != 1 || got[0] != "archive_command" {
		t.Errorf("padded upper-case archive_command => %v", got)
	}
	// Ordinary tuning is the tenant's and must not be stripped.
	for _, name := range []string{"work_mem", "maintenance_work_mem", "shared_buffers", "effective_cache_size",
		"max_connections", "random_page_cost", "statement_timeout", "idle_in_transaction_session_timeout",
		"default_statistics_target", "autovacuum_vacuum_scale_factor", "max_parallel_workers", "timezone"} {
		if got := operatorManagedParameterNames(map[string]string{name: "x"}); len(got) != 0 {
			t.Errorf("%q wrongly treated as platform-managed: %v", name, got)
		}
	}
}

// TestMaskedQueryTextNeverReachesASurface pins w4/m115's honest-degradation
// half. PostgreSQL substitutes `<insufficient privilege>` for the query text of
// backends owned by another role, and before this fix the placeholder was
// carried through `strVal` untouched — the dashboard rendered it verbatim in 34
// cells on 2026-09-19. The mapping must fold it into Masked with an empty Query
// so no surface (REST/GraphQL/MCP/dashboard) can mistake it for SQL, while the
// statistics beside it stay real.
func TestMaskedQueryTextNeverReachesASurface(t *testing.T) {
	procs := processViews([][]any{
		{int32(11), "other_user", "psql", "active", pgInsufficientPrivilege, "", "", int32(4)},
		{int32(12), "d_user", "bex", "active", "SELECT 1", "Client", "ClientRead", int32(0)},
	})
	if len(procs) != 2 {
		t.Fatalf("processViews => %d rows, want 2", len(procs))
	}
	if !procs[0].Masked || procs[0].Query != "" {
		t.Errorf("masked process = %+v, want Masked with empty Query", procs[0])
	}
	// The rest of a masked row is genuine and must survive.
	if procs[0].PID != 11 || procs[0].UserName != "other_user" || procs[0].DurationSeconds != 4 {
		t.Errorf("masked process lost its real columns: %+v", procs[0])
	}
	if procs[1].Masked || procs[1].Query != "SELECT 1" {
		t.Errorf("visible process = %+v, want the real query unflagged", procs[1])
	}

	top := topQueryViews([][]any{
		{pgInsufficientPrivilege, int64(7), 12.5, 1.75, int64(3), int64(9), int64(1)},
		{"SELECT 2", int64(1), 1.0, 1.0, int64(1), int64(0), int64(0)},
	})
	if len(top) != 2 {
		t.Fatalf("topQueryViews => %d rows, want 2", len(top))
	}
	if !top[0].Masked || top[0].Query != "" {
		t.Errorf("masked top query = %+v, want Masked with empty Query", top[0])
	}
	if top[0].Calls != 7 || top[0].MeanTimeMs != 1.75 {
		t.Errorf("masked top query lost its real statistics: %+v", top[0])
	}
	if top[1].Masked || top[1].Query != "SELECT 2" {
		t.Errorf("visible top query = %+v, want the real query unflagged", top[1])
	}

	// A query that merely CONTAINS the placeholder is real SQL, not a mask.
	embedded := topQueryViews([][]any{
		{"SELECT '" + pgInsufficientPrivilege + "'", int64(1), 1.0, 1.0, int64(1), int64(0), int64(0)},
	})
	if embedded[0].Masked {
		t.Errorf("embedded placeholder must not be treated as masked: %+v", embedded[0])
	}
}

// TestInsightConnectionKeepsItsReadOnlyRails asserts the least-privilege half of
// w4/m115: granting the owner pg_monitor widens what it can SEE, never what the
// insights path can DO. The connection the insight verbs dial must still open
// read-only with the insights statement timeout pinned at session startup.
func TestInsightConnectionKeepsItsReadOnlyRails(t *testing.T) {
	cfg, err := buildQueryConnConfig("postgres://u:p@localhost:5432/db?sslmode=disable",
		queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap}, true)
	if err != nil {
		t.Fatalf("buildQueryConnConfig => %v", err)
	}
	if got := cfg.RuntimeParams["default_transaction_read_only"]; got != "on" {
		t.Errorf("default_transaction_read_only = %q, want on", got)
	}
	want := strconv.FormatInt(queryStatementTimeout.Milliseconds(), 10)
	if got := cfg.RuntimeParams["statement_timeout"]; got != want {
		t.Errorf("statement_timeout = %q, want %q", got, want)
	}
}

// TestMaskedIsCarriedByEveryInsightSurface pins the w4/m115 signal across the
// three bex-api surfaces. REST and MCP serialize the view structs directly, so
// their contract is the JSON tag; GraphQL needs its own registered field, which
// is the one that can silently drift.
func TestMaskedIsCarriedByEveryInsightSurface(t *testing.T) {
	for name, typ := range map[string]reflect.Type{
		"ProcessView":  reflect.TypeOf(ProcessView{}),
		"TopQueryView": reflect.TypeOf(TopQueryView{}),
	} {
		f, ok := typ.FieldByName("Masked")
		if !ok {
			t.Fatalf("%s has no Masked field", name)
		}
		if got := f.Tag.Get("json"); got != "masked,omitempty" {
			t.Errorf("%s.Masked json tag = %q, want masked,omitempty", name, got)
		}
	}
	for name, obj := range map[string]*graphql.Object{
		"DatabaseProcess":  processViewGQLType,
		"DatabaseTopQuery": topQueryViewGQLType,
	} {
		if _, ok := obj.Fields()["masked"]; !ok {
			t.Errorf("GraphQL %s is missing the masked field", name)
		}
	}
}

// TestInsightsRedactLiterals pins w4/m167 and w5/075: pg_stat_statements does
// not normalize utility statements, so the provisioning ALTER ROLE … PASSWORD
// 'SCRAM-SHA-256$…' sat verbatim in Top queries on every fresh database, and a
// keyword parser missed four other spellings of a secret. Every string literal
// leaves bex-api redacted; comments, quoted identifiers and $n parameters stay
// verbatim.
func TestInsightsRedactLiterals(t *testing.T) {
	const verifier = "SCRAM-SHA-256$4096:c2FsdA==$c3RvcmVk:c2VydmVy"
	for _, tc := range []struct{ name, in, want string }{
		{"provisioning verifier", `ALTER ROLE "dpg_x_user" WITH PASSWORD '` + verifier + `'`,
			`ALTER ROLE "dpg_x_user" WITH PASSWORD '<redacted>'`},
		{"encrypted, mixed case", `create user u encrypted Password 'hunter2' valid until 'infinity'`,
			`create user u encrypted Password '<redacted>' valid until '<redacted>'`},
		{"doubled quote inside", `ALTER ROLE u PASSWORD 'it''s' LOGIN`, `ALTER ROLE u PASSWORD '<redacted>' LOGIN`},
		{"escape string", `ALTER ROLE u PASSWORD E'a\'b' LOGIN`, `ALTER ROLE u PASSWORD '<redacted>' LOGIN`},
		{"dollar quoted", `ALTER ROLE u PASSWORD $$s3cr3t$$`, `ALTER ROLE u PASSWORD '<redacted>'`},
		{"tagged dollar quote", `ALTER ROLE u PASSWORD $p$a$$b$p$ LOGIN`, `ALTER ROLE u PASSWORD '<redacted>' LOGIN`},
		{"truncated mid-literal", `ALTER ROLE u WITH PASSWORD 'SCRAM-SHA-256$4096:c2Fs`, `ALTER ROLE u WITH PASSWORD '<redacted>'`},
		{"user mapping option", `CREATE USER MAPPING FOR u SERVER s OPTIONS (user 'a', password 'pw')`,
			`CREATE USER MAPPING FOR u SERVER s OPTIONS (user '<redacted>', password '<redacted>')`},
		{"conninfo value", `CREATE SUBSCRIPTION s CONNECTION 'host=h password=pw dbname=d' PUBLICATION p`,
			`CREATE SUBSCRIPTION s CONNECTION '<redacted>' PUBLICATION p`},
		{"two secrets", `ALTER ROLE a PASSWORD 'x'; ALTER ROLE b PASSWORD 'y'`,
			`ALTER ROLE a PASSWORD '<redacted>'; ALTER ROLE b PASSWORD '<redacted>'`},
		// w5/075: the spellings the keyword parser missed.
		{"comment before the value", `ALTER ROLE u PASSWORD /*c*/ 'x'`, `ALTER ROLE u PASSWORD /*c*/ '<redacted>'`},
		{"unicode escape string", `ALTER ROLE u PASSWORD U&'x' LOGIN`, `ALTER ROLE u PASSWORD '<redacted>' LOGIN`},
		{"quoted conninfo value", `CREATE SUBSCRIPTION s CONNECTION 'host=h password=''x'' dbname=d' PUBLICATION p`,
			`CREATE SUBSCRIPTION s CONNECTION '<redacted>' PUBLICATION p`},
		{"connection URI", `SELECT dblink_connect('postgresql://u:x@h/db')`, `SELECT dblink_connect('<redacted>')`},
		{"no space after the keyword", `ALTER ROLE u PASSWORD'x'`, `ALTER ROLE u PASSWORD'<redacted>'`},
		{"bit and hex strings", `SELECT B'1010', X'1F', N'x'`, `SELECT '<redacted>', '<redacted>', '<redacted>'`},
		{"function body", `CREATE FUNCTION f() RETURNS int AS $body$ SELECT 'k' $body$ LANGUAGE sql`,
			`CREATE FUNCTION f() RETURNS int AS '<redacted>' LANGUAGE sql`},
		{"live query text", `SELECT * FROM users WHERE email = 'a@b.co' AND id = $1`,
			`SELECT * FROM users WHERE email = '<redacted>' AND id = $1`},
		{"non-ASCII dollar tag", `ALTER ROLE u PASSWORD $é$s3cr3t$é$ LOGIN`, `ALTER ROLE u PASSWORD '<redacted>' LOGIN`},
		{"typed literal after a name ending in e", `SELECT date'2026-01-01'`, `SELECT date'<redacted>'`},
		{"escaped backslash ends an escape string", `SELECT E'a\\', 'x'`, `SELECT '<redacted>', '<redacted>'`},
		// standard_conforming_strings=off makes \' an escaped quote; the text
		// cannot say, so the rest is redacted rather than risk a leak.
		{"backslash before a quote", `ALTER ROLE u PASSWORD 'pa\'ss' LOGIN`, `ALTER ROLE u PASSWORD '<redacted>'`},
		{"line comment ends at a newline", "SELECT 1 -- don't\nFROM t WHERE p = 'x'",
			"SELECT 1 -- don't\nFROM t WHERE p = '<redacted>'"},
		{"line comment ends at a carriage return", "SELECT 1 -- c\rAND pw = 'secret'",
			"SELECT 1 -- c\rAND pw = '<redacted>'"},
		// Negatives: nothing a literal opens, or a quote that opens nothing.
		{"password column", `SELECT password FROM users WHERE id = $1`, `SELECT password FROM users WHERE id = $1`},
		{"normalized parameter", `UPDATE users SET password = $1 WHERE id = $2`, `UPDATE users SET password = $1 WHERE id = $2`},
		{"password null", `ALTER ROLE u PASSWORD NULL`, `ALTER ROLE u PASSWORD NULL`},
		{"plain ALTER ROLE", `ALTER ROLE u CONNECTION LIMIT 5`, `ALTER ROLE u CONNECTION LIMIT 5`},
		{"provisioning DDL", `CREATE DATABASE "app" OWNER "app_user"`, `CREATE DATABASE "app" OWNER "app_user"`},
		{"extension", `CREATE EXTENSION IF NOT EXISTS pg_stat_statements`, `CREATE EXTENSION IF NOT EXISTS pg_stat_statements`},
		{"quote in a quoted identifier", `SELECT "it's" FROM t WHERE x = $1`, `SELECT "it's" FROM t WHERE x = $1`},
		{"quote in a line comment", "SELECT 1 -- don't\nFROM t", "SELECT 1 -- don't\nFROM t"},
		{"quote in a nested block comment", `/* a /* it's */ 'still comment' */ SELECT $1`,
			`/* a /* it's */ 'still comment' */ SELECT $1`},
		{"identifier with dollar signs", `SELECT a$b$ FROM t$x$`, `SELECT a$b$ FROM t$x$`},
		{"prefix letter ending a name", `SELECT name FROM e WHERE e.x = $1`, `SELECT name FROM e WHERE e.x = $1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactLiterals(tc.in); got != tc.want {
				t.Errorf("redactLiterals(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}

	// Both publishers apply it, and a redacted row is readable, not masked.
	leak := `ALTER ROLE "dpg_x_user" WITH PASSWORD '` + verifier + `'`
	top := topQueryViews([][]any{{leak, int64(1), 1.0, 1.0, int64(0), int64(0), int64(0)}})
	procs := processViews([][]any{{int32(1), "postgres", "", "active", leak, "", "", int32(0)}})
	for surface, got := range map[string]struct {
		query  string
		masked bool
	}{
		"top queries": {top[0].Query, top[0].Masked},
		"processes":   {procs[0].Query, procs[0].Masked},
	} {
		if strings.Contains(got.query, "SCRAM") || !strings.Contains(got.query, redactedLiteral) {
			t.Errorf("%s published %q, want the verifier redacted", surface, got.query)
		}
		if got.masked {
			t.Errorf("%s: a redacted row must stay Masked=false (distinct from the privilege placeholder)", surface)
		}
	}
}

// TestTopQueriesLeavesInsightsQueryIDsOut pins w5/184: a database that ran an
// Insights statement untagged, before w4/557, keeps that untagged text under the
// statement's query id, and every tagged run since adds to the same row. Top
// queries leaves Insights' query ids out, read from EXPLAIN. The test database
// has no pg_stat_statements, so compute_query_id is turned on for it and a table
// stands in for the view, as in the envelope test above.
func TestTopQueriesLeavesInsightsQueryIDsOut(t *testing.T) {
	uri := testDBURI(t)
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("admin connect: %v", err)
	}
	defer admin.Close(ctx)
	var present bool
	if err := admin.QueryRow(ctx, `SELECT to_regclass('pg_stat_statements') IS NOT NULL`).Scan(&present); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Skip("the database has a real pg_stat_statements; the stand-in table would not be the one read")
	}
	var dbname string
	if err := admin.QueryRow(ctx, `SELECT current_database()`).Scan(&dbname); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`ALTER DATABASE ` + pgx.Identifier{dbname}.Sanitize() + ` SET compute_query_id = on`,
		`DROP ROLE IF EXISTS topq_ids_customer`,
		`CREATE ROLE topq_ids_customer`,
		`CREATE TABLE public.pg_stat_statements (
			userid oid, queryid bigint, query text, calls bigint, total_exec_time double precision,
			mean_exec_time double precision, rows bigint, shared_blks_hit bigint, shared_blks_read bigint)`,
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, uri)
		if err != nil {
			return
		}
		defer c.Close(ctx)
		_, _ = c.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{dbname}.Sanitize()+` RESET compute_query_id`)
		_, _ = c.Exec(ctx, `DROP TABLE IF EXISTS public.pg_stat_statements`)
		_, _ = c.Exec(ctx, `DROP ROLE IF EXISTS topq_ids_customer`)
	})

	// The query id the table-sizes insight runs under, as EXPLAIN reports it
	// on a fresh session (the setting applies to new connections).
	probe, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := probe.Query(ctx, "EXPLAIN (VERBOSE) "+tagInsight(sqlTableSizes))
	if err != nil {
		t.Fatal(err)
	}
	var plan strings.Builder
	for lines.Next() {
		var line string
		if err := lines.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(line + "\n")
	}
	lines.Close()
	_ = probe.Close(ctx)
	m := explainQueryID.FindStringSubmatch(plan.String())
	if m == nil {
		t.Fatalf("EXPLAIN reported no query id: %s", plan.String())
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.pg_stat_statements
		SELECT (SELECT oid FROM pg_roles WHERE rolname = 'topq_ids_customer'), id::bigint, q, 1, t, t, 0, 0, 0
		FROM (VALUES ($1, 'SELECT schemaname AS schema FROM pg_tables WHERE schemaname NOT IN ($1 /*, ... */) LIMIT $2', 90.0),
		             ('42', 'SELECT id FROM orders', 70.0)) AS v(id, q, t)`, m[1]); err != nil {
		t.Fatalf("seed stand-in: %v", err)
	}

	svc, _ := newService()
	seedDatabaseAt(t, svc, "topq-ids", uri)
	rows, err := svc.TopQueries(ctx, "topq-ids")
	if err != nil {
		t.Fatalf("TopQueries => %v", err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Query)
	}
	if want := []string{"SELECT id FROM orders"}; !slices.Equal(got, want) {
		t.Fatalf("Top queries = %q, want %q: the untagged Insights row left out by its query id", got, want)
	}
}
