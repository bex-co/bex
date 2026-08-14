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

package deploys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// filter_test.go covers w2/m31: the ListFilter translation (FilterOf), the
// REST query params, the GraphQL arguments, the MCP limit/cursor knobs, and
// the cross-surface agreement between them.

// seedHistory fills a fakeStore with a deterministic newest-first history for
// srv-1: dep-5 (open, newest) > dep-4 (canceled) > dep-3 (live) > dep-2
// (update_failed) > dep-1 (live, oldest), one minute apart from base.
func seedHistory(ds *fakeStore) (base time.Time) {
	base = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	statuses := []string{
		store.DeployLive, store.DeployUpdateFailed, store.DeployLive,
		store.DeployCanceled, store.DeployUpdateInProgress,
	}
	for i, status := range statuses {
		created := base.Add(time.Duration(i) * time.Minute)
		updated := created.Add(30 * time.Second)
		d := store.Deploy{
			ID: fmt.Sprintf("dep-%d", i+1), AppID: "srv-1", Trigger: store.TriggerAPI,
			Status: status, CreatedAt: created, UpdatedAt: updated,
		}
		if store.IsTerminalDeployStatus(status) {
			d.FinishedAt = &updated
		}
		ds.byApp["srv-1"] = append([]store.Deploy{d}, ds.byApp["srv-1"]...)
	}
	return base
}

func viewIDs(deploys []DeployView) []string {
	out := make([]string, len(deploys))
	for i, d := range deploys {
		out[i] = d.ID
	}
	return out
}

// --- FilterOf -----------------------------------------------------------------

func TestFilterOfRejectsMalformedParams(t *testing.T) {
	for _, tc := range []struct {
		before, after string
		limit         int
	}{
		{"yesterday", "", 0},
		{"", "2026-13-45", 0},
		{"1699999999", "", 0}, // an epoch, not RFC3339
		{"", "", -1},          // a negative limit must never read as "unbounded"
	} {
		if _, err := FilterOf(nil, tc.before, tc.after, "", "", "", "", "", tc.limit); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("FilterOf(before=%q, after=%q, limit=%d): want core.ErrBadRequest, got %v", tc.before, tc.after, tc.limit, err)
		}
	}
}

func TestFilterOfParses(t *testing.T) {
	f, err := FilterOf([]string{"live"}, "2026-07-01T00:05:00Z", "2026-07-01T00:00:00Z", "", "", "", "", "dep-3", 7)
	if err != nil {
		t.Fatalf("FilterOf: %v", err)
	}
	if f.CreatedBefore.IsZero() || f.CreatedAfter.IsZero() || f.Cursor != "dep-3" || len(f.Statuses) != 1 || f.Limit != 7 {
		t.Errorf("FilterOf = %+v", f)
	}
	if empty, err := FilterOf(nil, "", "", "", "", "", "", "", 0); err != nil ||
		len(empty.Statuses) != 0 || !empty.CreatedBefore.IsZero() || !empty.CreatedAfter.IsZero() ||
		empty.Cursor != "" || empty.Limit != 0 {
		t.Errorf("all-absent params must be the zero filter (the full history), got %+v (err %v)", empty, err)
	}
}

// --- REST ---------------------------------------------------------------------

// newRESTHarness registers svc's REST fragment on a fresh mux and returns a
// request driver — the one REST test harness this package's suites share.
func newRESTHarness(t *testing.T, svc *Service) func(method, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}
}

func decodeList(t *testing.T, rec *httptest.ResponseRecorder) []deployWithCursor {
	t.Helper()
	var list []deployWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list %s: %v", rec.Body, err)
	}
	return list
}

func TestRESTListFiltersAndPaginates(t *testing.T) {
	ds := newFakeStore()
	seedHistory(ds)
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	do := newRESTHarness(t, svc)
	get := func(path string) *httptest.ResponseRecorder { return do("GET", path) }

	rec := get("/v1/services/web/deploys?status=live&limit=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status+limit: code=%d body=%s", rec.Code, rec.Body)
	}
	list := decodeList(t, rec)
	if len(list) != 1 || list[0].Deploy.ID != "dep-3" || list[0].Cursor != "dep-3" {
		t.Fatalf("status=live&limit=1 = %+v, want the newest live deploy dep-3", list)
	}

	// The item cursor resumes the SAME filtered listing on the next call.
	rec = get("/v1/services/web/deploys?status=live&limit=1&cursor=" + list[0].Cursor)
	if list = decodeList(t, rec); len(list) != 1 || list[0].Deploy.ID != "dep-1" {
		t.Fatalf("next live page = %+v, want dep-1", list)
	}

	// Repeated status params OR together.
	rec = get("/v1/services/web/deploys?status=update_failed&status=canceled")
	if list = decodeList(t, rec); len(list) != 2 || list[0].Deploy.ID != "dep-4" || list[1].Deploy.ID != "dep-2" {
		t.Fatalf("two statuses = %+v, want [dep-4 dep-2]", list)
	}

	// Exclusive time bounds, spelled RFC3339 like every other bex window param.
	rec = get("/v1/services/web/deploys?createdAfter=2026-07-01T00:01:00Z&createdBefore=2026-07-01T00:04:00Z")
	if list = decodeList(t, rec); len(list) != 2 || list[0].Deploy.ID != "dep-4" || list[1].Deploy.ID != "dep-3" {
		t.Fatalf("window = %+v, want [dep-4 dep-3]", list)
	}

	// No params returns the newest core.MaxPageLimit page (all 5 here fit;
	// the store bounds an absent limit — codex-security round-6 #7).
	rec = get("/v1/services/web/deploys")
	if list = decodeList(t, rec); len(list) != 5 {
		t.Fatalf("no params = %d rows, want all 5 (under the page cap)", len(list))
	}
}

func TestRESTListRejectsMalformedParams(t *testing.T) {
	ds := newFakeStore()
	seedHistory(ds)
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	do := newRESTHarness(t, svc)
	get := func(path string) *httptest.ResponseRecorder { return do("GET", path) }

	for _, path := range []string{
		"/v1/services/web/deploys?createdBefore=yesterday",
		"/v1/services/web/deploys?createdAfter=not-a-time",
		"/v1/services/web/deploys?updatedBefore=not-a-time",
		"/v1/services/web/deploys?finishedAfter=not-a-time",
		"/v1/services/web/deploys?limit=abc",
		"/v1/services/web/deploys?limit=0",
		"/v1/services/web/deploys?limit=-3",
	} {
		if rec := get(path); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: code=%d body=%s, want 400 — a malformed param must never be a silently-dropped filter", path, rec.Code, rec.Body)
		}
	}
}

func TestDeployLifecycleFiltersAndUpdatedAtMatchEverySurface(t *testing.T) {
	ds := newFakeStore()
	seedHistory(ds)
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	// dep-3 is the only live deploy updated after 00:01 and finished before
	// 00:03. REST returns the authoritative updatedAt on the same object.
	do := newRESTHarness(t, svc)
	rest := decodeList(t, do("GET", "/v1/services/web/deploys?status=live&updatedAfter=2026-07-01T00:01:00Z&finishedBefore=2026-07-01T00:03:00Z"))
	if len(rest) != 1 || rest[0].Deploy.ID != "dep-3" || rest[0].Deploy.UpdatedAt != "2026-07-01T00:02:30Z" {
		t.Fatalf("REST lifecycle filter = %+v, want dep-3 with stored updatedAt", rest)
	}

	schema := testSchema(t, svc)
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{
		deploys(serviceId: "web", status: ["live"], updatedAfter: "2026-07-01T00:01:00Z", finishedBefore: "2026-07-01T00:03:00Z") { id updatedAt }
	}`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL lifecycle filter: %v", res.Errors)
	}
	gql := res.Data.(map[string]any)["deploys"].([]any)
	if len(gql) != 1 || gql[0].(map[string]any)["id"] != "dep-3" || gql[0].(map[string]any)["updatedAt"] != "2026-07-01T00:02:30Z" {
		t.Fatalf("GraphQL lifecycle filter = %+v, want dep-3 with stored updatedAt", gql)
	}

	page := callListDeploys(t, newMCPSession(t, svc), map[string]any{
		"serviceId":      "web",
		"status":         []string{"live"},
		"updatedAfter":   "2026-07-01T00:01:00Z",
		"finishedBefore": "2026-07-01T00:03:00Z",
	})
	if len(page.Deploys) != 1 || page.Deploys[0].ID != "dep-3" || page.Deploys[0].UpdatedAt != "2026-07-01T00:02:30Z" {
		t.Fatalf("MCP lifecycle filter = %+v, want dep-3 with stored updatedAt", page)
	}
}

func TestDeployTimestampFormattingPreservesSubsecondTransitionOrder(t *testing.T) {
	created := time.Date(2026, 7, 15, 12, 0, 0, 100, time.UTC)
	updated := created.Add(time.Microsecond)
	got := toRenderDeploy(DeployView{CreatedAt: created, UpdatedAt: updated})
	if got.CreatedAt == got.UpdatedAt {
		t.Fatalf("createdAt=%q updatedAt=%q, want distinct serialized transition instants", got.CreatedAt, got.UpdatedAt)
	}
}

func TestEveryDeployStatusFilterMatchesEverySurface(t *testing.T) {
	ds := newFakeStore()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for i, status := range store.DeployStatuses() {
		at := base.Add(time.Duration(i) * time.Minute)
		ds.byApp["srv-1"] = append([]store.Deploy{{
			ID: fmt.Sprintf("dep-%02d", i), AppID: "srv-1", Trigger: store.TriggerAPI,
			Status: status, CreatedAt: at, UpdatedAt: at,
		}}, ds.byApp["srv-1"]...)
	}
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	do := newRESTHarness(t, svc)
	schema := testSchema(t, svc)
	cs := newMCPSession(t, svc)

	for i, status := range store.DeployStatuses() {
		wantID := fmt.Sprintf("dep-%02d", i)
		t.Run(status, func(t *testing.T) {
			rest := decodeList(t, do("GET", "/v1/services/web/deploys?status="+status))
			if len(rest) != 1 || rest[0].Deploy.ID != wantID {
				t.Fatalf("REST status %s = %+v, want %s", status, rest, wantID)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
				RequestString: fmt.Sprintf(`{ deploys(serviceId: "web", status: ["%s"]) { id } }`, status)})
			if len(res.Errors) > 0 {
				t.Fatalf("GraphQL status %s: %v", status, res.Errors)
			}
			gql := res.Data.(map[string]any)["deploys"].([]any)
			if len(gql) != 1 || gql[0].(map[string]any)["id"] != wantID {
				t.Fatalf("GraphQL status %s = %+v, want %s", status, gql, wantID)
			}
			mcpPage := callListDeploys(t, cs, map[string]any{"serviceId": "web", "status": []string{status}})
			if len(mcpPage.Deploys) != 1 || mcpPage.Deploys[0].ID != wantID {
				t.Fatalf("MCP status %s = %+v, want %s", status, mcpPage, wantID)
			}
		})
	}
}

// --- GraphQL ------------------------------------------------------------------

func testSchema(t *testing.T, s *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return schema
}

func TestGraphQLDeploysFiltersMatchREST(t *testing.T) {
	ds := newFakeStore()
	seedHistory(ds)
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	schema := testSchema(t, svc)
	ctx := context.Background()

	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ deploys(serviceId: "web", status: ["live"], createdBefore: "2026-07-01T00:04:00Z", limit: 1) { id } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("deploys query: %v", res.Errors)
	}
	list := res.Data.(map[string]any)["deploys"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["id"] != "dep-3" {
		t.Fatalf("filtered deploys = %+v, want [dep-3]", list)
	}

	// The same filter through the Service (what REST renders) — identical set.
	want, err := svc.List(ctx, "web", ListFilter{
		Statuses: []string{"live"}, CreatedBefore: time.Date(2026, 7, 1, 0, 4, 0, 0, time.UTC), Limit: 1,
	})
	if err != nil || len(want) != 1 || want[0].ID != "dep-3" {
		t.Fatalf("Service.List = %+v (err %v) — GraphQL and REST must agree", want, err)
	}

	// A malformed timestamp is a resolver error (the REST 400), never a
	// silently-dropped filter — and an explicit limit below 1 is rejected the
	// same way REST rejects ?limit=0 (only an ABSENT limit gets the default
	// store-bounded page).
	for _, q := range []string{
		`{ deploys(serviceId: "web", createdBefore: "yesterday") { id } }`,
		`{ deploys(serviceId: "web", limit: 0) { id } }`,
		`{ deploys(serviceId: "web", limit: -2) { id } }`,
	} {
		if res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: q}); len(res.Errors) == 0 {
			t.Errorf("%s: want a resolver error, got none", q)
		}
	}

	// Cursor pages the same keyset REST pages.
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ deploys(serviceId: "web", cursor: "dep-3") { id } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("cursor query: %v", res.Errors)
	}
	list = res.Data.(map[string]any)["deploys"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["id"] != "dep-2" || list[1].(map[string]any)["id"] != "dep-1" {
		t.Fatalf("cursor page = %+v, want [dep-2 dep-1]", list)
	}
}

// TestGraphQLDeploySingleRead covers w9/m1/t001: the deploy(serviceId,
// deployId) by-id field REST's GET .../deploys/{deployId} and MCP's
// get_deploy already had — found, unknown, and cross-service-owned-id, all
// through the same s.Get verb REST/MCP call, so the three surfaces cannot
// drift on fields or the not-found shape.
func TestGraphQLDeploySingleRead(t *testing.T) {
	ds := newFakeStore()
	seedHistory(ds)
	svc, _ := newService(ds, sampleApp("web", "srv-1"), sampleApp("other", "srv-2"))
	schema := testSchema(t, svc)
	ctx := context.Background()

	// Found: the same DeployView fields the REST detail endpoint serves.
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ deploy(serviceId: "web", deployId: "dep-3") { id status preDeployStatus } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("deploy query: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["deploy"].(map[string]any)
	if got["id"] != "dep-3" || got["status"] != store.DeployLive {
		t.Fatalf("deploy = %+v, want id=dep-3 status=live", got)
	}

	// Cross-checked against Service.Get (what REST/MCP render) — identical.
	want, err := svc.Get(ctx, "web", "dep-3")
	if err != nil || want.ID != "dep-3" || want.Status != store.DeployLive {
		t.Fatalf("Service.Get = %+v (err %v) — GraphQL and REST/MCP must agree", want, err)
	}

	// Unknown deployId -> not-found (a resolver error), never a phantom deploy.
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ deploy(serviceId: "web", deployId: "dep-nope") { id } }`})
	if len(res.Errors) == 0 {
		t.Fatal("unknown deployId: want a not-found error, got none")
	}
	if !strings.Contains(res.Errors[0].Message, "not found") {
		t.Errorf("unknown deployId error = %q, want it to name not-found", res.Errors[0].Message)
	}

	// A deployId belonging to a DIFFERENT service must not resolve — never a
	// cross-app leak through the id alone (deploys.Service.Get's doc comment).
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ deploy(serviceId: "other", deployId: "dep-3") { id } }`})
	if len(res.Errors) == 0 {
		t.Fatal("cross-service deployId: want a not-found error, got none")
	}
}

// --- MCP ----------------------------------------------------------------------

func newMCPSession(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func callListDeploys(t *testing.T, cs *mcp.ClientSession, args map[string]any) listDeploysResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_deploys", Arguments: args})
	if err != nil || res.IsError {
		t.Fatalf("list_deploys(%v): %v isErr=%v", args, err, res.IsError)
	}
	var out listDeploysResult
	if err := decodeStructured(res.StructuredContent, &out); err != nil {
		t.Fatalf("decode list_deploys result: %v", err)
	}
	return out
}

// TestMCPListDeploysPagination covers t004's intentional behavior change: the
// tool now defaults to a 10-deploy page (Render's own MCP tool's default)
// instead of the full history, and pages via the result's cursor.
func TestMCPListDeploysPagination(t *testing.T) {
	ds := newFakeStore()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 12; i++ {
		d := store.Deploy{
			ID: fmt.Sprintf("dep-%02d", i), AppID: "srv-1", Trigger: store.TriggerAPI,
			Status: store.DeployLive, CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}
		ds.byApp["srv-1"] = append([]store.Deploy{d}, ds.byApp["srv-1"]...)
	}
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	cs := newMCPSession(t, svc)

	// No limit ⇒ Render's default page of 10, newest first, cursor = last item.
	page := callListDeploys(t, cs, map[string]any{"serviceId": "web"})
	if len(page.Deploys) != 10 || page.Deploys[0].ID != "dep-12" || page.Cursor != "dep-03" {
		t.Fatalf("default page = %d deploys, cursor %q; want 10 ending at cursor dep-03", len(page.Deploys), page.Cursor)
	}

	// The cursor fetches the remainder; its own cursor then ends the walk with
	// an empty page and an empty cursor.
	page = callListDeploys(t, cs, map[string]any{"serviceId": "web", "cursor": page.Cursor})
	if len(page.Deploys) != 2 || page.Deploys[0].ID != "dep-02" || page.Cursor != "dep-01" {
		t.Fatalf("second page = %+v, want [dep-02 dep-01]", page)
	}
	page = callListDeploys(t, cs, map[string]any{"serviceId": "web", "cursor": page.Cursor})
	if len(page.Deploys) != 0 || page.Cursor != "" {
		t.Fatalf("end of history = %+v, want an empty page with an empty cursor", page)
	}

	// An explicit limit is honored; out-of-range values fall back safely
	// (below 1 ⇒ the default 10, above 100 ⇒ the 100 cap) rather than erroring
	// an agent mid-loop.
	if page = callListDeploys(t, cs, map[string]any{"serviceId": "web", "limit": 3}); len(page.Deploys) != 3 {
		t.Errorf("limit 3 = %d deploys", len(page.Deploys))
	}
	if page = callListDeploys(t, cs, map[string]any{"serviceId": "web", "limit": -1}); len(page.Deploys) != 10 {
		t.Errorf("limit -1 = %d deploys, want the default 10", len(page.Deploys))
	}
	if page = callListDeploys(t, cs, map[string]any{"serviceId": "web", "limit": 5000}); len(page.Deploys) != 12 {
		t.Errorf("limit 5000 = %d deploys, want all 12 (clamped to 100, which covers them)", len(page.Deploys))
	}
}

// --- cross-surface agreement (t005) --------------------------------------------

// TestDeployPaginationSurfaceParity drives the same paged query through REST,
// GraphQL, and MCP and asserts all three return the identical id sequence —
// the one-Service-three-adapters invariant, extended to w2/m31's paging.
func TestDeployPaginationSurfaceParity(t *testing.T) {
	ds := newFakeStore()
	seedHistory(ds)
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	// REST: limit 2, then follow the last item's cursor.
	do := newRESTHarness(t, svc)
	var restIDs []string
	cursor := ""
	for {
		path := "/v1/services/web/deploys?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		list := decodeList(t, do("GET", path))
		if len(list) == 0 {
			break
		}
		for _, item := range list {
			restIDs = append(restIDs, item.Deploy.ID)
		}
		cursor = list[len(list)-1].Cursor
	}

	// GraphQL: the same walk through deploys(serviceId, cursor, limit).
	schema := testSchema(t, svc)
	var gqlIDs []string
	cursor = ""
	for {
		q := `{ deploys(serviceId: "web", limit: 2` + map[bool]string{true: `, cursor: "` + cursor + `"`, false: ""}[cursor != ""] + `) { id } }`
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q})
		if len(res.Errors) > 0 {
			t.Fatalf("graphql page: %v", res.Errors)
		}
		list := res.Data.(map[string]any)["deploys"].([]any)
		if len(list) == 0 {
			break
		}
		for _, item := range list {
			gqlIDs = append(gqlIDs, item.(map[string]any)["id"].(string))
		}
		cursor = gqlIDs[len(gqlIDs)-1]
	}

	// MCP: the same walk through list_deploys' limit/cursor.
	cs := newMCPSession(t, svc)
	var mcpIDs []string
	args := map[string]any{"serviceId": "web", "limit": 2}
	for {
		page := callListDeploys(t, cs, args)
		if len(page.Deploys) == 0 {
			break
		}
		for _, d := range page.Deploys {
			mcpIDs = append(mcpIDs, d.ID)
		}
		args = map[string]any{"serviceId": "web", "limit": 2, "cursor": page.Cursor}
	}

	want := []string{"dep-5", "dep-4", "dep-3", "dep-2", "dep-1"}
	for name, got := range map[string][]string{"REST": restIDs, "GraphQL": gqlIDs, "MCP": mcpIDs} {
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s walk = %v, want %v", name, got, want)
		}
	}
}
