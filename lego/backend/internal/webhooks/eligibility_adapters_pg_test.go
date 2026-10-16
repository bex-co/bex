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

package webhooks

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestEveryEnabledStateAdapterShapesTheSameEligibility (w4/m212 t004): REST
// create(enabled:false) and PATCH, GraphQL setWebhookEndpointEnabled and
// updateWebhookEndpoint, and MCP update_webhook_endpoint all change an
// endpoint's eligibility history through the one store rule — over real
// Postgres, so the trigger behind every writer is what is exercised. A save
// that omits enabled, and a toggle the caller is refused, leave it unchanged.
func TestEveryEnabledStateAdapterShapesTheSameEligibility(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	ten, err := st.CreateTenant(ctx, fmt.Sprintf("wh-m212-adapters-%d", time.Now().UnixNano()%1e9), store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ten.ID) })
	svc := &Service{Base: &core.Base{Namespace: "default", Workspace: coretest.Workspaces{ten.ID}}, Store: st}
	ctx = core.WithIdentity(ctx, core.Identity{Subject: "user-1", Method: "session"})
	rest := func(svc *Service, method, target, body string) *httptest.ResponseRecorder {
		t.Helper()
		return webhookRESTAs(t, ctx, svc, method, target, body, nil)
	}

	var id string
	intervals := func(wantOpen bool, wantTotal int) {
		t.Helper()
		var open, total int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FILTER (WHERE disabled_at IS NULL), count(*)
			   FROM webhook_endpoint_eligibility WHERE endpoint_id = $1`, id).Scan(&open, &total); err != nil {
			t.Fatal(err)
		}
		if (open == 1) != wantOpen || open > 1 || total != wantTotal {
			t.Fatalf("intervals: open=%d total=%d, want open=%v total=%d", open, total, wantOpen, wantTotal)
		}
	}

	created := rest(svc, http.MethodPost, "/v1/webhooks",
		`{"ownerId":"`+ten.ID+`","name":"m212","url":"https://hooks.example.com/m212","enabled":false,"eventFilter":["deploy_ended"]}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("REST create = %d %s", created.Code, created.Body.String())
	}
	id = decodeObject(t, created)["id"].(string)
	intervals(false, 0)

	schema := webhookGraphQLSchema(t, svc)
	gql := func(query string) {
		t.Helper()
		if res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: query}); len(res.Errors) > 0 {
			t.Fatalf("GraphQL %s: %v", query, res.Errors)
		}
	}
	gql(`mutation { setWebhookEndpointEnabled(id:"` + id + `", enabled:true) { id enabled } }`)
	intervals(true, 1)

	if rr := rest(svc, http.MethodPatch, "/v1/webhooks/"+id, `{"enabled":false}`); rr.Code != http.StatusOK {
		t.Fatalf("REST disable = %d %s", rr.Code, rr.Body.String())
	}
	intervals(false, 1)
	if rr := rest(svc, http.MethodPatch, "/v1/webhooks/"+id, `{"name":"m212-renamed"}`); rr.Code != http.StatusOK {
		t.Fatalf("REST rename = %d %s", rr.Code, rr.Body.String())
	}
	intervals(false, 1)

	client := webhookMCPClient(t, ctx, svc)
	if res, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name: "update_webhook_endpoint", Arguments: map[string]any{"id": id, "enabled": true},
	}); err != nil || res.IsError {
		t.Fatalf("MCP enable: %v %+v", err, res)
	}
	intervals(true, 2)

	gql(`mutation { updateWebhookEndpoint(id:"` + id + `", enabled:false) { id enabled } }`)
	intervals(false, 2)

	outsider := &Service{Base: &core.Base{Namespace: "default", Workspace: coretest.Workspaces{"tea-elsewhere"}}, Store: st}
	if rr := rest(outsider, http.MethodPatch, "/v1/webhooks/"+id, `{"enabled":true}`); rr.Code < 400 {
		t.Fatalf("outsider enable = %d %s, want a refusal", rr.Code, rr.Body.String())
	}
	intervals(false, 2)
}
