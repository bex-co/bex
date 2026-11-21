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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w32/077 on real Postgres, through REST, GraphQL and MCP: a Blueprint's sync
// history never lists a plan waiting for approval, as Render's leaves out
// every `created` sync ("Syncs that are planned but not yet approved (in the
// `created` state) are not included", the pinned CLI's ListBlueprintSyncs and
// api-docs.render.com's list-blueprint-syncs). A cursor naming the waiting
// plan pages from where it stands, and an approved plan is listed under the
// same id.
func TestBlueprintSyncHistoryLeavesOutWaitingPlansOnEverySurfacePG(t *testing.T) {
	svc, _, _, ctx, wsID := newRecreatePGService(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema := blueprintSchema(t, svc)
	session := blueprintMCPSession(t, svc, ctx)

	created, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "history", OwnerID: wsID, Source: RenderBlueprintSource{Repo: "https://github.com/acme/history-" + wsID}})
	if err != nil || created.PlanningOutcome != PlanningPlanned || created.Sync == nil {
		t.Fatalf("create = %+v err=%v, want a planned sync", created, err)
	}
	bp, first := created.Blueprint.ID, created.Sync.ID

	// history is what every surface lists from cursor, newest first, once the
	// three agree and none lists a `created` sync.
	history := func(cursor string) []string {
		t.Helper()
		path := "/v1/blueprints/" + bp + "/syncs?ownerId=" + wsID
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx))
		var rows []struct {
			Sync BlueprintSyncView `json:"sync"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("REST GET %s = %d %s (%v)", path, rec.Code, rec.Body, err)
		}
		restRuns := make([]BlueprintSyncView, len(rows))
		for i, r := range rows {
			restRuns[i] = r.Sync
		}

		out := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
			RequestString: fmt.Sprintf(`{ blueprintSyncs(id:%q, ownerId:%q, cursor:%q) { id state } }`, bp, wsID, cursor)})
		if len(out.Errors) > 0 {
			t.Fatalf("GraphQL blueprintSyncs: %v", out.Errors)
		}
		raw, _ := json.Marshal(out.Data.(map[string]any)["blueprintSyncs"])
		var gqlRuns []BlueprintSyncView
		if err := json.Unmarshal(raw, &gqlRuns); err != nil {
			t.Fatalf("GraphQL blueprintSyncs = %s (%v)", raw, err)
		}

		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_blueprint_syncs", Arguments: map[string]any{"id": bp, "cursor": cursor}})
		if err != nil || res.IsError {
			t.Fatalf("MCP list_blueprint_syncs = %+v, %v", res, err)
		}
		raw, _ = json.Marshal(res.StructuredContent)
		var listed listBlueprintSyncsResult
		if err := json.Unmarshal(raw, &listed); err != nil {
			t.Fatalf("MCP list_blueprint_syncs = %s (%v)", raw, err)
		}

		var ids []string
		for surface, runs := range map[string][]BlueprintSyncView{"REST": restRuns, "GraphQL": gqlRuns, "MCP": listed.Syncs} {
			got := make([]string, len(runs))
			for i, r := range runs {
				if r.State == store.BlueprintSyncStateCreated {
					t.Errorf("%s lists %s, a plan waiting for approval", surface, r.ID)
				}
				got[i] = r.ID
			}
			if ids == nil {
				ids = got
			} else if !slices.Equal(got, ids) {
				t.Fatalf("%s lists %v, another surface %v", surface, got, ids)
			}
		}
		return ids
	}
	want := func(step string, got []string, runs ...string) {
		t.Helper()
		if !slices.Equal(got, runs) {
			t.Fatalf("%s: history = [%s], want [%s]", step, strings.Join(got, " "), strings.Join(runs, " "))
		}
	}

	want("the first plan waiting", history(""))
	if _, err := svc.ApproveBlueprint(ctx, bp, wsID, first, nil); err != nil {
		t.Fatalf("approve the first plan: %v", err)
	}
	want("the first plan approved", history(""), first)

	svc.GitFetcher = fakeBlueprintFetcher{contents: manifestNaming("web")}
	replan, err := svc.PlanBlueprint(ctx, bp, wsID)
	if err != nil || replan.PlanningOutcome != PlanningPlanned || replan.Sync == nil {
		t.Fatalf("re-plan = %+v err=%v, want a planned sync", replan, err)
	}
	second := replan.Sync.ID
	want("a re-plan waiting", history(""), first)
	want("a cursor naming the waiting re-plan", history(second), first)
	if _, err := svc.ApproveBlueprint(ctx, bp, wsID, second, nil); err != nil {
		t.Fatalf("approve the re-plan: %v", err)
	}
	want("the re-plan approved", history(""), second, first)
}
