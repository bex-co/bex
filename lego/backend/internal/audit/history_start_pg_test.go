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

package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w9/098: Render's audit history "starts only when you upgrade to Pro or
// higher. Earlier data is unavailable." A Hobby workspace's events are still
// recorded, but after the upgrade REST (which the dashboard's CSV export
// reads) and GraphQL list only what happened since; a downgrade and a later
// upgrade start it again; a workspace created on Pro keeps its whole history.
func TestPGAuditHistoryStartsAtTheUpgradeToPro(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}, Plans: st}, Store: st}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	callerCtx := core.WithIdentity(ctx, core.Identity{Subject: "admin-1", Method: "session"})

	record := func(ws, id string, at time.Time) {
		t.Helper()
		if err := st.Record(ctx, core.AuditEvent{ID: id, Verb: "apps.Create", Resource: "workspace:" + ws, Outcome: core.AuditAllowed, At: at}); err != nil {
			t.Fatal(err)
		}
	}
	listed := func(ws string) []string {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/owners/"+ws+"/audit-logs?direction=forward", nil)
		mux.ServeHTTP(rec, req.WithContext(callerCtx))
		if rec.Code != http.StatusOK {
			t.Fatalf("REST %s: %d %s", ws, rec.Code, rec.Body)
		}
		var page []struct {
			AuditLog struct {
				ID string `json:"id"`
			} `json:"auditLog"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		var rest []string
		for _, row := range page {
			rest = append(rest, row.AuditLog.ID)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: callerCtx,
			RequestString: `{ auditLogs(ownerId:"` + ws + `", direction:"forward") { id } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL %s: %v", ws, res.Errors)
		}
		var gql []string
		for _, row := range res.Data.(map[string]any)["auditLogs"].([]any) {
			gql = append(gql, row.(map[string]any)["id"].(string))
		}
		if !slices.Equal(rest, gql) {
			t.Fatalf("%s: REST lists %v, GraphQL %v", ws, rest, gql)
		}
		return rest
	}
	newWorkspace := func(name, plan string) string {
		t.Helper()
		ws, err := st.CreateTenant(ctx, name, plan)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM audit_events WHERE workspace_id = $1`, ws.ID)
			_ = st.DeleteTenant(ctx, ws.ID)
		})
		return ws.ID
	}
	setPlan := func(ws, plan string) {
		t.Helper()
		if _, err := st.UpdateTenantPlan(ctx, ws, plan); err != nil {
			t.Fatal(err)
		}
	}

	hobby := newWorkspace("ws-audit-history-hobby", store.PlanHobby)
	record(hobby, "aud-098-hobby-old", time.Now().Add(-time.Hour))
	record(hobby, "aud-098-hobby-recent", time.Now().Add(-time.Second))
	setPlan(hobby, store.PlanPro)
	record(hobby, "aud-098-after-pro", time.Now())
	if got := listed(hobby); !slices.Equal(got, []string{"aud-098-after-pro"}) {
		t.Fatalf("after hobby->pro the log lists %v, want only the event after the upgrade", got)
	}
	// A caller's earlier startTime cannot reach past the start either.
	if events, err := svc.List(callerCtx, hobby, Filter{Since: time.Now().Add(-24 * time.Hour)}); err != nil || len(events) != 1 {
		t.Fatalf("an earlier startTime lists %d events, %v; want the one after the upgrade", len(events), err)
	}

	// Pro -> Scale keeps the start; Scale -> Hobby -> Pro moves it forward.
	setPlan(hobby, store.PlanScale)
	if got := listed(hobby); !slices.Equal(got, []string{"aud-098-after-pro"}) {
		t.Fatalf("after pro->scale the log lists %v, want the same start", got)
	}
	setPlan(hobby, store.PlanHobby)
	record(hobby, "aud-098-hobby-again", time.Now())
	setPlan(hobby, store.PlanPro)
	record(hobby, "aud-098-pro-again", time.Now())
	if got := listed(hobby); !slices.Equal(got, []string{"aud-098-pro-again"}) {
		t.Fatalf("after a downgrade and a second upgrade the log lists %v, want only the event after it", got)
	}

	pro := newWorkspace("ws-audit-history-pro", store.PlanPro)
	record(pro, "aud-098-pro-old", time.Now().Add(-time.Hour))
	record(pro, "aud-098-pro-new", time.Now())
	if got := listed(pro); !slices.Equal(got, []string{"aud-098-pro-old", "aud-098-pro-new"}) {
		t.Fatalf("a workspace created on Pro lists %v, want its whole history", got)
	}
}
