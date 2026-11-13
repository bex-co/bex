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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// imageOnlyManifest plans one create and needs no manual env var, so its plan
// approves without supplied values.
const imageOnlyManifest = `
services:
  - type: web
    name: api
    runtime: image
    image:
      url: docker.io/library/nginx:1.27
`

// renderSyncID is Render's blueprintSync id pattern in the pinned OpenAPI.
var renderSyncID = regexp.MustCompile(`^exe-[0-9a-z]{20}$`)

// w2/m288: every surface shows a Blueprint sync as Render's exe- spelling of
// its stored bsr- key — the Render-shaped create, plan and approve bodies, the
// approve refusal's syncId, and the sync history on REST, GraphQL and MCP —
// and a sync-id input (the history cursor) given either spelling reaches the
// store as the durable key.
func TestBlueprintSyncPublicIDOnEverySurface(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, imageOnlyManifest)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rest := func(method, path, body string) (int, string) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	// public reports a body's sync id after checking it is the exe- alias of
	// a stored run and that no durable bsr- key leaks beside it.
	public := func(t *testing.T, surface, id, body string) string {
		t.Helper()
		if !renderSyncID.MatchString(id) {
			t.Fatalf("%s sync id = %q, want Render's exe- spelling", surface, id)
		}
		durable := ids.BlueprintSyncStorageID(id)
		if _, ok := st.syncs[durable]; !ok {
			t.Fatalf("%s sync id %q names no stored run (%s)", surface, id, durable)
		}
		if strings.Contains(body, durable) {
			t.Fatalf("%s = %s leaks the durable key %s", surface, body, durable)
		}
		return id
	}

	code, body := rest(http.MethodPost, "/v1/blueprints", `{"name":"stack","ownerId":"tea-a","source":{"repo":"https://github.com/a/app","branch":"main"}}`)
	var created struct {
		Blueprint struct {
			ID string `json:"id"`
		} `json:"blueprint"`
		Sync struct {
			ID string `json:"id"`
		} `json:"sync"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || code != http.StatusCreated {
		t.Fatalf("create = %d %s (%v)", code, body, err)
	}
	public(t, "REST create", created.Sync.ID, body)
	bp := created.Blueprint.ID

	code, body = rest(http.MethodPost, "/v1/blueprints/"+bp+"/syncs", "")
	var planned struct {
		Sync struct {
			ID string `json:"id"`
		} `json:"sync"`
	}
	if err := json.Unmarshal([]byte(body), &planned); err != nil || code != http.StatusCreated {
		t.Fatalf("plan = %d %s (%v)", code, body, err)
	}
	plan := public(t, "REST plan", planned.Sync.ID, body)

	// The dashboard's planned-sync card and GraphQL's mirror of the plan.
	out := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: ctx,
		RequestString: fmt.Sprintf(`{ blueprintPlannedSync(id:%q, ownerId:"tea-a") { id } }`, bp)})
	if len(out.Errors) > 0 {
		t.Fatalf("GraphQL blueprintPlannedSync: %v", out.Errors)
	}
	if got := out.Data.(map[string]any)["blueprintPlannedSync"].(map[string]any)["id"]; got != plan {
		t.Fatalf("GraphQL blueprintPlannedSync id = %v, want %s", got, plan)
	}

	// w5/m384: an approval bound to a plan that a re-plan replaced is refused
	// and applies nothing; bound to the newest plan, it applies.
	if created.Sync.ID == plan {
		t.Fatalf("the re-plan kept the create's sync id %s", plan)
	}
	code, body = rest(http.MethodPost, "/v1/blueprints/"+bp+"/approve", fmt.Sprintf(`{"syncId":%q}`, created.Sync.ID))
	if code != http.StatusConflict || !strings.Contains(body, "BLUEPRINT_PLAN_CHANGED") || !strings.Contains(body, plan) {
		t.Fatalf("approve of a replaced plan = %d %s, want 409 BLUEPRINT_PLAN_CHANGED naming %s", code, body, plan)
	}
	if run := st.syncs[ids.BlueprintSyncStorageID(plan)]; run.State != store.BlueprintSyncStateCreated {
		t.Fatalf("a refused approval moved the newest plan to %s", run.State)
	}
	// Either spelling of the reviewed plan's id binds it.
	code, body = rest(http.MethodPost, "/v1/blueprints/"+bp+"/approve", fmt.Sprintf(`{"syncId":%q}`, ids.BlueprintSyncStorageID(plan)))
	var approved struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(body), &approved); err != nil || code != http.StatusOK || approved.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %d %s (%v)", code, body, err)
	}
	if public(t, "REST approve", approved.ID, body) != plan {
		t.Fatalf("approve id = %s, want the planned sync %s", approved.ID, plan)
	}

	// Approving again is refused; the refusal names the newest sync by its
	// public id, in the message and in details.syncId.
	_, err := svc.ApproveBlueprint(ctx, bp, "tea-a", "", nil)
	var refusal *core.CodedError
	if !errors.As(err, &refusal) || refusal.Params["syncId"] != plan || !strings.Contains(refusal.Error(), plan) {
		t.Fatalf("second approve = %v, want a refusal naming %s", err, plan)
	}

	// The history: the fake returns its stored runs (bsr- keys) in order.
	durable := ids.BlueprintStorageID(bp)
	for _, id := range []string{ids.BlueprintSyncStorageID(plan), ids.BlueprintSyncStorageID(created.Sync.ID)} {
		st.syncsToReturn = append(st.syncsToReturn, st.syncs[id])
	}
	cursor := ids.BlueprintSyncStorageID(created.Sync.ID)
	for _, spelling := range []string{ids.BlueprintSyncPublicID(cursor), cursor} {
		t.Run("cursor "+spelling[:3], func(t *testing.T) {
			code, body := rest(http.MethodGet, "/v1/blueprints/"+bp+"/syncs?ownerId=tea-a&cursor="+spelling, "")
			var rows []struct {
				Sync struct {
					ID string `json:"id"`
				} `json:"sync"`
				Cursor string `json:"cursor"`
			}
			if err := json.Unmarshal([]byte(body), &rows); err != nil || code != http.StatusOK || len(rows) != 2 {
				t.Fatalf("REST syncs = %d %s (%v)", code, body, err)
			}
			for _, r := range rows {
				if public(t, "REST syncs", r.Sync.ID, body) != r.Cursor {
					t.Errorf("REST sync %s pages by cursor %s, want its own id", r.Sync.ID, r.Cursor)
				}
			}
			if st.gotSyncCursor != cursor {
				t.Errorf("REST cursor %s reached the store as %q, want %s", spelling, st.gotSyncCursor, cursor)
			}

			st.gotSyncCursor = ""
			out := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: ctx,
				RequestString: fmt.Sprintf(`{ blueprintSyncs(id:%q, ownerId:"tea-a", cursor:%q) { id } }`, durable, spelling)})
			if len(out.Errors) > 0 {
				t.Fatalf("GraphQL blueprintSyncs: %v", out.Errors)
			}
			raw, _ := json.Marshal(out.Data)
			for _, row := range out.Data.(map[string]any)["blueprintSyncs"].([]any) {
				public(t, "GraphQL blueprintSyncs", row.(map[string]any)["id"].(string), string(raw))
			}
			if st.gotSyncCursor != cursor {
				t.Errorf("GraphQL cursor %s reached the store as %q, want %s", spelling, st.gotSyncCursor, cursor)
			}

			st.gotSyncCursor = ""
			session := blueprintMCPSession(t, svc, ctx)
			res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_blueprint_syncs", Arguments: map[string]any{"id": bp, "cursor": spelling}})
			if err != nil || res.IsError {
				t.Fatalf("MCP list_blueprint_syncs = %+v, %v", res, err)
			}
			raw, _ = json.Marshal(res.StructuredContent)
			var listed listBlueprintSyncsResult
			if err := json.Unmarshal(raw, &listed); err != nil || len(listed.Syncs) != 2 {
				t.Fatalf("MCP list_blueprint_syncs = %s (%v)", raw, err)
			}
			for _, run := range listed.Syncs {
				public(t, "MCP list_blueprint_syncs", run.ID, string(raw))
			}
			if st.gotSyncCursor != cursor {
				t.Errorf("MCP cursor %s reached the store as %q, want %s", spelling, st.gotSyncCursor, cursor)
			}
		})
	}
}

// blueprintMCPSession connects an in-memory MCP client to svc's tools.
func blueprintMCPSession(t *testing.T, svc *Service, ctx context.Context) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "m288", Version: "0"}, nil)
	svc.RegisterMCP(server)
	a, b := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, a, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "m288", Version: "0"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}
