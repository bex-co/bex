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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w21/105: a list that returns a download URL of a logical backup records one
// Render DownloadDatabaseBackupEvent, on REST, GraphQL and MCP alike; a list that
// returns none records none; a refused one records its denial.

const exportAuditSignedURL = "https://objects.example.com/signed?X-Amz-Signature=deadbeef"

type exportAuditFixture struct {
	svc    *Service
	cl     client.Client
	sink   *webhookAuditSink
	signer *fakeExportSigner
	now    time.Time
}

// newExportAuditFixture is a database in workspace tea-export with one available
// export, and a signer that counts the URLs it mints.
func newExportAuditFixture(t *testing.T) *exportAuditFixture {
	t.Helper()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	svc, cl := newService()
	db := seedLogicalExport(t, cl, now, appv1alpha1.DatabaseExportAvailable)
	db.Labels = map[string]string{core.LabelTenant: "tea-export"}
	if err := cl.Update(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	f := &exportAuditFixture{svc: svc, cl: cl, sink: &webhookAuditSink{}, signer: &fakeExportSigner{url: exportAuditSignedURL}, now: now}
	svc.ExportSigner = f.signer
	svc.Audit = f.sink
	svc.Clock = func() time.Time { return now }
	return f
}

// setExports replaces the database's exports with one request per phase, newest
// last, each created an hour ago and expiring in six days (an available one
// expires at expiresAt when given).
func (f *exportAuditFixture) setExports(t *testing.T, phases ...appv1alpha1.DatabaseExportPhase) {
	t.Helper()
	var db appv1alpha1.Database
	if err := f.cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "export-db"}, &db); err != nil {
		t.Fatal(err)
	}
	db.Spec.Exports, db.Status.Exports = nil, nil
	for i, phase := range phases {
		id := "exp-audit-" + string(rune('a'+i))
		created := f.now.Add(-time.Duration(len(phases)-i) * time.Hour)
		db.Spec.Exports = append(db.Spec.Exports, appv1alpha1.DatabaseExportRequest{ID: id, RequestedAt: created.Format(time.RFC3339)})
		db.Status.Exports = append(db.Status.Exports, appv1alpha1.DatabaseExportStatus{
			ID: id, Phase: phase, CreatedAt: created.Format(time.RFC3339),
			ExpiresAt: f.now.Add(6 * 24 * time.Hour).Format(time.RFC3339), Filename: id + ".dir.tar.gz",
		})
	}
	if err := f.cl.Update(context.Background(), &db); err != nil {
		t.Fatal(err)
	}
}

// surfaces are the three ways to ask for the list, each returning the error it
// got (nil on success). GraphQL selects url, as a client that wants to download.
func (f *exportAuditFixture) surfaces(t *testing.T) map[string]func(ctx context.Context) error {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: f.svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]func(ctx context.Context) error{
		"REST": func(ctx context.Context) error {
			rec := serveRESTAs(ctx, f.svc, http.MethodGet, "/v1/postgres/export-db/export")
			if rec.Code != http.StatusOK {
				return errors.New(rec.Body.String() + " (HTTP " + http.StatusText(rec.Code) + ")")
			}
			return nil
		},
		"GraphQL": func(ctx context.Context) error {
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ databaseExports(id:"export-db") { id status url } }`})
			if len(res.Errors) > 0 {
				return res.Errors[0]
			}
			return nil
		},
		"MCP": func(ctx context.Context) error {
			res, err := f.callMCP(ctx, "list_postgres_exports")
			if err != nil {
				return err
			}
			if res.IsError {
				return errors.New("MCP tool error")
			}
			return nil
		},
	}
}

func (f *exportAuditFixture) callMCP(ctx context.Context, tool string) (*mcp.CallToolResult, error) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	f.svc.RegisterMCP(server)
	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		return nil, err
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{"postgresId": "export-db"}})
}

// serveRESTAs is serveREST as the caller in ctx.
func serveRESTAs(ctx context.Context, svc *Service, method, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil).WithContext(ctx))
	return rec
}

func downloadRows(events []core.AuditEvent) []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range events {
		if ev.Verb == core.AuditVerbDownloadDatabaseBackup {
			out = append(out, ev)
		}
	}
	return out
}

func TestAListThatReturnsADownloadURLRecordsOneDownloadOnEverySurface(t *testing.T) {
	f := newExportAuditFixture(t)
	ctx := ctxAs("user-a")

	var first core.AuditEvent
	for _, name := range []string{"REST", "GraphQL", "MCP"} {
		f.sink.events, f.signer.calls = nil, 0
		if err := f.surfaces(t)[name](ctx); err != nil {
			t.Fatalf("%s list: %v", name, err)
		}
		if f.signer.calls != 1 {
			t.Fatalf("%s minted %d URLs, want 1", name, f.signer.calls)
		}
		if len(f.sink.events) != 1 {
			t.Fatalf("%s recorded %d rows, want exactly the download: %+v", name, len(f.sink.events), f.sink.events)
		}
		ev := f.sink.events[0]
		if ev.Verb != core.AuditVerbDownloadDatabaseBackup || ev.Outcome != core.AuditAllowed ||
			ev.Target != core.DatabaseTarget("export-db") || ev.TargetName != "export-db" ||
			ev.Resource != core.WorkspaceObject("tea-export") || ev.Caller != "user-a" {
			t.Errorf("%s recorded %+v", name, ev)
		}
		// Nothing of the URL, its signature or the export's object key is in the row.
		raw, _ := json.Marshal(ev)
		for _, secret := range []string{"objects.example.com", "X-Amz", "deadbeef", "logical-exports", "exp-c185th5c2rvvnhbfiltg"} {
			if strings.Contains(string(raw), secret) {
				t.Errorf("%s's row carries %q: %s", name, secret, raw)
			}
		}
		if name == "REST" {
			first = ev
			continue
		}
		// The three surfaces write the same row.
		ev.At, first.At = time.Time{}, time.Time{}
		if ev.Verb != first.Verb || ev.Target != first.Target || ev.TargetName != first.TargetName ||
			ev.Resource != first.Resource || ev.Outcome != first.Outcome || ev.Caller != first.Caller {
			t.Errorf("%s's row %+v differs from REST's %+v", name, ev, first)
		}
	}
}

// One request is one row, however many URLs it minted: a list is one decision to
// hand out the capability, as a list of N environment variables is one view.
func TestAListThatMintedSeveralURLsRecordsOneDownload(t *testing.T) {
	f := newExportAuditFixture(t)
	f.setExports(t, appv1alpha1.DatabaseExportAvailable, appv1alpha1.DatabaseExportAvailable, appv1alpha1.DatabaseExportAvailable)
	if _, err := f.svc.ListExports(ctxAs("user-a"), "export-db"); err != nil {
		t.Fatal(err)
	}
	if f.signer.calls != 3 || len(f.sink.events) != 1 || f.sink.events[0].Verb != core.AuditVerbDownloadDatabaseBackup {
		t.Fatalf("3 URLs minted %d, recorded %+v; want one download row", f.signer.calls, f.sink.events)
	}
}

func TestAListThatReturnsNoDownloadURLRecordsNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		phases []appv1alpha1.DatabaseExportPhase
	}{
		{"no export was ever requested", nil},
		{"an export is still running", []appv1alpha1.DatabaseExportPhase{appv1alpha1.DatabaseExportRunning}},
		{"an export was only requested", []appv1alpha1.DatabaseExportPhase{appv1alpha1.DatabaseExportCreated}},
		{"the only export failed", []appv1alpha1.DatabaseExportPhase{appv1alpha1.DatabaseExportFailed}},
		{"the only export expired", []appv1alpha1.DatabaseExportPhase{appv1alpha1.DatabaseExportExpired}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newExportAuditFixture(t)
			f.setExports(t, tc.phases...)
			for name, list := range f.surfaces(t) {
				if err := list(ctxAs("user-a")); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if len(f.sink.events) != 0 || f.signer.calls != 0 {
					t.Fatalf("%s minted %d URLs and recorded %+v for a list with no URL", name, f.signer.calls, f.sink.events)
				}
			}
		})
	}

	t.Run("an available export past its deadline", func(t *testing.T) {
		f := newExportAuditFixture(t)
		f.svc.Clock = func() time.Time { return f.now.Add(8 * 24 * time.Hour) }
		list, err := f.svc.ListExports(ctxAs("user-a"), "export-db")
		if err != nil || len(list) != 1 || list[0].Status != "expired" || list[0].URL != "" {
			t.Fatalf("list = %+v, %v", list, err)
		}
		if len(f.sink.events) != 0 || f.signer.calls != 0 {
			t.Fatalf("an expired export minted %d URLs and recorded %+v", f.signer.calls, f.sink.events)
		}
	})
}

type failingExportSigner struct{ calls int }

func (f *failingExportSigner) Presign(context.Context, *appv1alpha1.Database, appv1alpha1.DatabaseExportStatus, time.Duration) (string, error) {
	f.calls++
	return "", errors.New("the object store is unreachable")
}

// The row is written once the URLs were minted: a list whose signer failed returned
// no URL and records no download.
func TestAListWhoseSignerFailedRecordsNoDownload(t *testing.T) {
	f := newExportAuditFixture(t)
	signer := &failingExportSigner{}
	f.svc.ExportSigner = signer
	if _, err := f.svc.ListExports(ctxAs("user-a"), "export-db"); err == nil || signer.calls != 1 {
		t.Fatalf("list = %v after %d signer calls, want the signer's error", err, signer.calls)
	}
	f.svc.ExportSigner = nil
	if _, err := f.svc.ListExports(ctxAs("user-a"), "export-db"); err == nil {
		t.Fatal("a list with no signer answered")
	}
	if len(f.sink.events) != 0 {
		t.Fatalf("a list that returned no URL recorded %+v", f.sink.events)
	}
}

// A refused caller leaves the denial the authorization records, under the verb the
// list is known by, and no download and no minted URL, on every surface.
func TestARefusedListRecordsItsDenialAndNoDownload(t *testing.T) {
	f := newExportAuditFixture(t)
	f.svc.Authz = denySensitiveChecker{}
	for name, list := range f.surfaces(t) {
		f.sink.events = nil
		if err := list(ctxAs("viewer")); err == nil {
			t.Fatalf("%s: a caller without can_view_sensitive was answered", name)
		}
		if len(f.sink.events) != 1 {
			t.Fatalf("%s recorded %d rows for a refusal, want its one denial: %+v", name, len(f.sink.events), f.sink.events)
		}
		ev := f.sink.events[0]
		if ev.Verb != "postgres.ListExports" || ev.Outcome != core.AuditDenied || ev.Relation != core.RelCanViewSensitive ||
			ev.Target != core.DatabaseTarget("export-db") || ev.Caller != "viewer" {
			t.Errorf("%s denial = %+v", name, ev)
		}
		if len(downloadRows(f.sink.events)) != 0 || f.signer.calls != 0 {
			t.Errorf("%s: a refusal recorded a download or minted %d URLs", name, f.signer.calls)
		}
	}
}

// A revocation inside the positive cache's life is refused by the fresh check
// before any URL is signed, and records no download.
func TestAListRefusedByTheFreshCheckRecordsNoDownload(t *testing.T) {
	f := newExportAuditFixture(t)
	f.svc.Authz = staleAllowChecker{}
	if _, err := f.svc.ListExports(ctxAs("user-a"), "export-db"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a stale positive: %v, want ErrForbidden", err)
	}
	if len(downloadRows(f.sink.events)) != 0 || f.signer.calls != 0 {
		t.Fatalf("a stale positive minted %d URLs and recorded %+v", f.signer.calls, f.sink.events)
	}
}

// A GraphQL query decides by what it selects (the env var list's Selects(p,
// "value") is the precedent): the dashboard's status poll selects no url, mints no
// URL and records no download, and a query that selects url, under an alias or in
// a fragment, is the download.
func TestGraphQLRecordsADownloadOnlyForAQueryThatSelectsTheURL(t *testing.T) {
	f := newExportAuditFixture(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: f.svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	run := func(query string) map[string]any {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: ctxAs("user-a")})
		if len(res.Errors) != 0 {
			t.Fatal(res.Errors)
		}
		return res.Data.(map[string]any)["databaseExports"].([]any)[0].(map[string]any)
	}

	for _, tc := range []struct {
		name, query string
		recorded    bool
	}{
		{"the status poll", `{ databaseExports(id:"export-db") { id status createdAt expiresAt filename failureReason } }`, false},
		{"only the expiry of a URL", `{ databaseExports(id:"export-db") { id urlExpiresAt } }`, false},
		{"the url", `{ databaseExports(id:"export-db") { id url } }`, true},
		{"an aliased url", `{ databaseExports(id:"export-db") { id link: url } }`, true},
		{"a url in a fragment", `query { databaseExports(id:"export-db") { ...F } } fragment F on DatabaseExport { id url urlExpiresAt }`, true},
		{"a url in an inline fragment", `{ databaseExports(id:"export-db") { id ... on DatabaseExport { url } } }`, true},
	} {
		f.sink.events, f.signer.calls = nil, 0
		got := run(tc.query)
		if got["id"] == nil || (tc.name == "the status poll" && got["status"] != "available") {
			t.Errorf("%s: answered %v", tc.name, got)
		}
		wantRows, wantCalls := 0, 0
		if tc.recorded {
			wantRows, wantCalls = 1, 1
		}
		if len(f.sink.events) != wantRows || f.signer.calls != wantCalls {
			t.Errorf("%s: recorded %+v and minted %d URLs, want %d row and %d URLs", tc.name, f.sink.events, f.signer.calls, wantRows, wantCalls)
		}
		if tc.name == "only the expiry of a URL" && got["urlExpiresAt"] != nil && got["urlExpiresAt"] != "" {
			t.Errorf("%s: an expiry for a URL that was not minted: %v", tc.name, got["urlExpiresAt"])
		}
	}
}

// The status poll still needs can_view_sensitive and records the same denial.
func TestGraphQLStatusPollOfARefusedCallerRecordsItsDenial(t *testing.T) {
	f := newExportAuditFixture(t)
	f.svc.Authz = denySensitiveChecker{}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: f.svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("viewer"),
		RequestString: `{ databaseExports(id:"export-db") { id status } }`})
	if len(res.Errors) == 0 {
		t.Fatalf("a refused status poll was answered: %v", res.Data)
	}
	if len(f.sink.events) != 1 || f.sink.events[0].Verb != "postgres.ListExports" || f.sink.events[0].Outcome != core.AuditDenied {
		t.Fatalf("recorded %+v, want the one denial", f.sink.events)
	}
}
