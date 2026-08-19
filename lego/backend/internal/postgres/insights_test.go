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
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

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
	// TopQueries silences errors (pg_stat_statements unavailable is the same code
	// path as connection failure) and returns an empty list — never an error.
	if out, err := svc.TopQueries(ctx, "no-db"); err != nil || len(out) != 0 {
		t.Errorf("TopQueries unknown db => (%v, %v), want ([], nil)", out, err)
	}
}

// TestSetParameterOverrides covers the write path (CR patching) without a live DB.
func TestSetParameterOverrides(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "param-db")
	ctx := context.Background()

	pg, err := svc.SetParameterOverrides(ctx, "param-db", map[string]string{
		"log_min_duration_statement": "1000",
		"work_mem":                   "8MB",
	})
	if err != nil {
		t.Fatalf("SetParameterOverrides => %v", err)
	}
	if pg.ID != "param-db" {
		t.Fatalf("view id = %q, want param-db", pg.ID)
	}

	spec, err := svc.GetParameterSpec(ctx, "param-db")
	if err != nil {
		t.Fatalf("GetParameterSpec => %v", err)
	}
	if spec["log_min_duration_statement"] != "1000" || spec["work_mem"] != "8MB" {
		t.Fatalf("GetParameterSpec = %v, want the two params", spec)
	}

	// shared_preload_libraries must be dropped — the operator owns that key.
	if _, err := svc.SetParameterOverrides(ctx, "param-db", map[string]string{
		"shared_preload_libraries": "pg_stat_statements,badlib",
		"log_statement":            "all",
	}); err != nil {
		t.Fatalf("SetParameterOverrides with shared_preload_libraries => %v", err)
	}
	spec2, _ := svc.GetParameterSpec(ctx, "param-db")
	if _, has := spec2["shared_preload_libraries"]; has {
		t.Error("shared_preload_libraries must be dropped from user-settable params")
	}
	if spec2["log_statement"] != "all" {
		t.Errorf("other params should survive: %v", spec2)
	}

	// Empty map clears parameters.
	if _, err := svc.SetParameterOverrides(ctx, "param-db", map[string]string{}); err != nil {
		t.Fatalf("SetParameterOverrides empty => %v", err)
	}
	spec3, _ := svc.GetParameterSpec(ctx, "param-db")
	if len(spec3) != 0 {
		t.Fatalf("cleared params should be empty, got %v", spec3)
	}
}

// TestRESTInsightEndpoints verifies routing for the five GET endpoints and the
// PUT parameter-overrides handler. The seeded DB uses a non-routable URI so
// GET endpoints that require a real dial return 503; TopQueries returns 200 with
// an empty list; PUT doesn't dial at all so it returns 200.
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
		{"GET", "/v1/postgres/ins-db/top-queries", "", 200}, // TopQueries never errors
		{"GET", "/v1/postgres/ins-db/sizes", "", 500},
		{"GET", "/v1/postgres/ins-db/table-scans", "", 500},
		{"GET", "/v1/postgres/ins-db/parameter-overrides", "", 500},
		// PUT only patches the CR — no DB dial required.
		{"PUT", "/v1/postgres/ins-db/parameter-overrides", `{"parameters":{"work_mem":"16MB"}}`, 200},
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

// TestGQLInsightFields verifies that the five insight query fields and the
// setDatabaseParameterOverrides mutation are registered in the GraphQL schema.
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
	if _, ok := svc.GraphQLMutation()["setDatabaseParameterOverrides"]; !ok {
		t.Error("GraphQLMutation missing setDatabaseParameterOverrides")
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
// pg_stat_activity (our own connection).
func TestProcessesIntegration(t *testing.T) {
	uri := testDBURI(t)
	svc, _ := newService()
	seedDatabaseAt(t, svc, "proc-live", uri)

	out, err := svc.Processes(context.Background(), "proc-live")
	if err != nil {
		t.Fatalf("Processes => %v", err)
	}
	if len(out) == 0 {
		t.Error("Processes: expected at least one process")
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
	admin.Close(context.Background())

	svc, _ := newService()
	seedDatabaseAt(t, svc, "topq-live", uri)

	out, err := svc.TopQueries(context.Background(), "topq-live")
	if err != nil {
		t.Fatalf("TopQueries => %v", err)
	}
	t.Logf("TopQueries returned %d rows", len(out))
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
