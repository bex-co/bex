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
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestMapPGError checks the driver-error → value-free sentinel mapping without a
// database: read-only and timeout classes get their own sentinels; any other DB
// error surfaces only its SQLSTATE (never the Postgres message).
func TestMapPGError(t *testing.T) {
	if got := mapPGError(nil); got != nil {
		t.Fatalf("nil => %v", got)
	}
	// Caller-fault classes carry their own sentinel and also wrap core.ErrBadRequest
	// so WriteErr maps them to 400 (not the unclassified 500 default).
	ro := &pgconn.PgError{Code: pgerrcode.ReadOnlySQLTransaction, Message: `cannot execute INSERT in a read-only transaction`}
	if got := mapPGError(ro); !errors.Is(got, errQueryReadOnly) || !errors.Is(got, core.ErrBadRequest) {
		t.Errorf("read-only => %v, want errQueryReadOnly wrapping ErrBadRequest", got)
	}
	to := &pgconn.PgError{Code: pgerrcode.QueryCanceled, Message: "canceling statement due to statement timeout"}
	if got := mapPGError(to); !errors.Is(got, errQueryTimeout) || !errors.Is(got, core.ErrBadRequest) {
		t.Errorf("canceled => %v, want errQueryTimeout wrapping ErrBadRequest", got)
	}
	if got := mapPGError(context.DeadlineExceeded); !errors.Is(got, errQueryTimeout) {
		t.Errorf("deadline => %v, want errQueryTimeout", got)
	}
	// Syntax error echoes a query token in .Message; the mapped error must not.
	syn := &pgconn.PgError{Code: pgerrcode.SyntaxError, Message: `syntax error at or near "SECRETVALUE"`}
	got := mapPGError(syn)
	if !errors.Is(got, core.ErrBadRequest) {
		t.Errorf("syntax => %v, want ErrBadRequest", got)
	}
	if strings.Contains(got.Error(), "SECRETVALUE") {
		t.Errorf("mapped error leaked the Postgres message: %q", got.Error())
	}
	if !strings.Contains(got.Error(), pgerrcode.SyntaxError) {
		t.Errorf("mapped error should carry the SQLSTATE: %q", got.Error())
	}
}

// TestQueryInputGuards covers the paths that don't need a live DB: empty SQL
// and an unknown database — all resolved before any dial. Query's shared fetch
// (w6/m17's AuthorizeDatabase) now authorizes-and-fetches before any other
// check, so the empty-SQL case needs a database that actually exists — an
// unknown name would report ErrNotFound first, exactly like the second case,
// and never exercise the SQL guard at all.
func TestQueryInputGuards(t *testing.T) {
	svc, _ := newService()
	seedDatabaseAt(t, svc, "any", "postgres://unused/placeholder")
	ctx := context.Background()

	if _, err := svc.Query(ctx, "any", "   "); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("empty sql => %v, want ErrBadRequest", err)
	}
	if _, err := svc.Query(ctx, "no-such-db", "SELECT 1"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown db => %v, want ErrNotFound", err)
	}
}

// TestMCPQueryToolAdvertised asserts the tool is exposed under Render's exact
// name with the postgresId + sql arguments — the drop-in-compat contract.
func TestMCPQueryToolAdvertised(t *testing.T) {
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
	var tool *mcp.Tool
	for _, tl := range res.Tools {
		if tl.Name == "query_render_postgres" {
			tool = tl
		}
	}
	if tool == nil {
		t.Fatal("query_render_postgres not advertised")
	}
	// Render's argument names must be present in the input schema.
	schema, _ := json.Marshal(tool.InputSchema)
	for _, want := range []string{"postgresId", "sql"} {
		if !strings.Contains(string(schema), want) {
			t.Errorf("input schema missing %q: %s", want, schema)
		}
	}
}

// --- Live-Postgres integration (hermetic-by-default) ---------------------------

// BEX_TEST_DB_URI points at a throwaway database, e.g.
// `docker run --rm -e POSTGRES_PASSWORD=pw -p 5433:5432 postgres:17`
// → postgres://postgres:pw@localhost:5433/postgres?sslmode=disable
func testDBURI(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	return uri
}

// seedDatabaseAt seeds a Ready Database whose "<name>-app" Secret uri points at
// the given target, so svc.Query exercises the full secret→dial→execute stack.
func seedDatabaseAt(t *testing.T, svc *Service, name, uri string) {
	t.Helper()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free"},
		Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady, SecretName: name + "-app"},
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-app", Namespace: "default"},
		Data:       map[string][]byte{"uri": []byte(uri)},
	}
	ctx := context.Background()
	if err := svc.Client.Create(ctx, db); err != nil {
		t.Fatalf("seed db: %v", err)
	}
	if err := svc.Client.Create(ctx, sec); err != nil {
		t.Fatalf("seed secret: %v", err)
	}
}

func TestQueryReadOnlyIntegration(t *testing.T) {
	uri := testDBURI(t)
	ctx := context.Background()

	// Set up a real table outside the read-only path so writes have a target that
	// exists (a missing relation would mask the read-only rejection).
	admin, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("admin connect: %v", err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, `DROP TABLE IF EXISTS q_test; CREATE TABLE q_test(id int); INSERT INTO q_test VALUES (1),(2),(3)`); err != nil {
		t.Fatalf("setup table: %v", err)
	}

	svc, _ := newService()
	seedDatabaseAt(t, svc, "live-db", uri)

	// SELECT returns columns + rows.
	res, err := svc.Query(ctx, "live-db", "SELECT id FROM q_test ORDER BY id")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(res.Columns) != 1 || res.Columns[0] != "id" || res.RowCount != 3 || res.Truncated {
		t.Fatalf("select result = %+v", res)
	}

	// Every write shape is rejected inside the read-only transaction.
	for _, w := range []string{
		"INSERT INTO q_test VALUES (9)",
		"UPDATE q_test SET id = 9",
		"DELETE FROM q_test",
		"CREATE TABLE q_evil(id int)",
		"DROP TABLE q_test",
	} {
		if _, err := svc.Query(ctx, "live-db", w); !errors.Is(err, errQueryReadOnly) {
			t.Errorf("write %q => %v, want errQueryReadOnly", w, err)
		}
	}

	// Multi-statement escape (incl. the SET read-write trick) is rejected by the
	// extended protocol, not by parsing.
	for _, esc := range []string{
		"SELECT 1; INSERT INTO q_test VALUES (9)",
		"SET transaction_read_only = off; INSERT INTO q_test VALUES (9)",
	} {
		if _, err := svc.Query(ctx, "live-db", esc); err == nil {
			t.Errorf("multi-statement %q was not rejected", esc)
		}
	}

	// Statement timeout fires (short limit via the helper so the test stays fast).
	_, err = runReadOnlyQuery(ctx, uri, "SELECT pg_sleep(3)", queryLimits{statementTimeout: 500 * time.Millisecond, rowCap: 500})
	if !errors.Is(err, errQueryTimeout) {
		t.Errorf("pg_sleep => %v, want errQueryTimeout", err)
	}

	// Row cap truncates and flags.
	capped, err := runReadOnlyQuery(ctx, uri, "SELECT g FROM generate_series(1, 50) g", queryLimits{statementTimeout: queryStatementTimeout, rowCap: 10})
	if err != nil {
		t.Fatalf("generate_series: %v", err)
	}
	if capped.RowCount != 10 || !capped.Truncated {
		t.Fatalf("cap result = %+v, want 10 rows truncated", capped)
	}
}
