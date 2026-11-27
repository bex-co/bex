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
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// commitMessages is a Git host that knows each commit's message.
type commitMessages map[string]string

func (m commitMessages) ResolveCommit(_ context.Context, _, _, ref string) (store.CommitInfo, bool, error) {
	msg, ok := m[ref]
	return store.CommitInfo{Hash: ref, Message: msg}, ok, nil
}

const syncPageCommitMessage = "Serve the api from nginx"

// syncActionSummary is a stored sync's actions as "operation kind name"
// lines, in the order they were planned.
func syncActionSummary(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var actions []BlueprintPlanAction
	if err := json.Unmarshal(raw, &actions); err != nil {
		t.Fatalf("actions %s: %v", raw, err)
	}
	lines := make([]string, 0, len(actions))
	for _, a := range actions {
		lines = append(lines, fmt.Sprintf("%s %s %s", a.Operation, a.Kind, a.Name))
	}
	return strings.Join(lines, "; ")
}

// w3/m678 t002 on real Postgres: a first sync, a manual sync, an automatic
// sync and an approved plan each leave a row carrying the actions it planned
// and its commit's message, which the sync's page lists. A row recorded before
// syncs kept their actions still reads, with none.
func TestEverySyncPathRecordsItsActionsAndCommitMessagePG(t *testing.T) {
	svc, st, _, ctx, wsID := newRecreatePGService(t)
	svc.Commits = commitMessages{testCommitSHA: syncPageCommitMessage}
	repo := "https://github.com/acme/sync-record-" + wsID

	row := func(bp, sync string) store.PlannedBlueprintSync {
		t.Helper()
		p, err := st.BlueprintPlannedSync(ctx, ids.BlueprintStorageID(bp), ids.BlueprintSyncStorageID(sync))
		if err != nil {
			t.Fatalf("read sync %s: %v", sync, err)
		}
		return p
	}
	newest := func(bp string) store.BlueprintSync {
		t.Helper()
		runs, err := st.ListBlueprintSyncs(ctx, ids.BlueprintStorageID(bp), "", 1)
		if err != nil || len(runs) != 1 {
			t.Fatalf("syncs of %s = %+v, %v", bp, runs, err)
		}
		return runs[0]
	}
	recorded := func(step string, p store.PlannedBlueprintSync, state, actions string) {
		t.Helper()
		if p.State != state || p.CommitID != testCommitSHA || p.CommitMessage != syncPageCommitMessage {
			t.Errorf("%s: sync %s is %s at %s %q, want %s at %s %q", step, p.ID, p.State, p.CommitID, p.CommitMessage, state, testCommitSHA, syncPageCommitMessage)
		}
		if len(p.PlanActions) == 0 {
			t.Fatalf("%s: sync %s recorded no actions", step, p.ID)
		}
		if got := syncActionSummary(t, p.PlanActions); got != actions {
			t.Errorf("%s: actions = %q, want %q", step, got, actions)
		}
	}

	// The first sync of a one-step create.
	created, err := svc.CreateBlueprint(ctx, wsID, CreateBlueprintRequest{Repo: repo, Branch: "main"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	bp := created.ID
	first := newest(bp)
	recorded("first sync", row(bp, first.ID), store.BlueprintSyncStateSuccess, "create service api")
	if first.CommitMessage != syncPageCommitMessage {
		t.Errorf("the history lists the first sync with message %q", first.CommitMessage)
	}

	// A manual sync of a file that renamed the service: the new one is
	// created and the old one detached, and both are recorded.
	svc.GitFetcher = fakeBlueprintFetcher{contents: manifestNaming("web")}
	if _, err := svc.SyncBlueprint(ctx, bp, wsID, "", "", nil); err != nil {
		t.Fatalf("manual sync: %v", err)
	}
	manual := newest(bp)
	recorded("manual sync", row(bp, manual.ID), store.BlueprintSyncStateSuccess, "create service web; detach service api")

	// An automatic sync, delivered by a push and run by the worker.
	svc.GitFetcher = fakeBlueprintFetcher{contents: strings.Replace(manifestNaming("web"), "nginx:1.27", "nginx:1.28", 1)}
	if _, err := st.EnqueueBlueprintAutoSyncIntent(ctx, store.BlueprintAutoSyncIntent{TenantID: wsID, BlueprintID: ids.BlueprintStorageID(bp),
		DeliveryDigest: "sync-record-" + wsID, CommitSHA: testCommitSHA, Path: CanonicalBlueprintFilename}); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimBlueprintAutoSyncIntents(ctx, 50, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(claimed, func(c store.BlueprintAutoSyncIntent) bool { return c.TenantID == wsID })
	if i < 0 {
		t.Fatalf("claimed %+v, none for %s", claimed, wsID)
	}
	(&BlueprintAutoSyncWorker{Svc: svc}).process(ctx, claimed[i])
	auto := newest(bp)
	if auto.ID == manual.ID {
		t.Fatal("the automatic sync recorded no run")
	}
	recorded("automatic sync", row(bp, auto.ID), store.BlueprintSyncStateSuccess, "update service web")

	// A planned create, then its plan approved: the plan's row keeps the
	// actions and the message it was planned with.
	svc.GitFetcher = fakeBlueprintFetcher{contents: manifestNaming("planned")}
	plan, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "planned", OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo + "-planned"}})
	if err != nil || plan.PlanningOutcome != PlanningPlanned || plan.Sync == nil {
		t.Fatalf("planned create = %+v, %v", plan, err)
	}
	recorded("planned sync", row(plan.Blueprint.ID, plan.Sync.ID), store.BlueprintSyncStateCreated, "create service planned")
	approved, err := svc.ApproveBlueprint(ctx, plan.Blueprint.ID, wsID, plan.Sync.ID, nil)
	if err != nil || approved.ID != plan.Sync.ID {
		t.Fatalf("approve = %+v, %v", approved, err)
	}
	recorded("approved plan", row(plan.Blueprint.ID, approved.ID), store.BlueprintSyncStateSuccess, "create service planned")

	// A row from before syncs kept their actions still reads, with none.
	before := time.Now().Add(-time.Hour)
	old, err := st.InsertBlueprintSync(ctx, store.BlueprintSync{BlueprintID: ids.BlueprintStorageID(bp), CommitID: testCommitSHA,
		State: store.BlueprintSyncStateSuccess, StartedAt: before, CompletedAt: &before})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetBlueprintSync(ctx, bp, wsID, old.ID)
	if err != nil || detail.ActionsRecorded || len(detail.Actions) != 0 || detail.CommitMessage != "" {
		t.Fatalf("an old sync reads %+v, %v; want it with no actions recorded", detail, err)
	}
}

// w3/m678 t003 on real Postgres: GraphQL's blueprintSync reads one sync with
// its actions, a state for each, its located errors and the sync:false values
// approving it needs, authorized as blueprintSyncs is: another Blueprint's sync
// is not found, and another workspace's Blueprint is refused as its history
// is. blueprintSyncs lists the waiting plan on its first page, as Render's
// dashboard does, where REST and MCP still leave it out.
func TestBlueprintSyncReadOnEveryStatePG(t *testing.T) {
	svc, st, _, ctx, wsID := newRecreatePGService(t)
	svc.Commits = commitMessages{testCommitSHA: syncPageCommitMessage}
	svc.Metadata = resourcemeta.Config{DashboardBaseURL: "https://dashboard.example"}
	schema := blueprintSchema(t, svc)
	repo := "https://github.com/acme/sync-read-" + wsID
	read := func(bp, sync string) (map[string]any, []string) {
		t.Helper()
		out := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: fmt.Sprintf(`{ blueprintSync(id:%q, syncId:%q) {
			id commitId commitMessage state actionsRecorded dashboardUrl
			actions { operation kind name } actionStates { state }
			errors { severity message path line column } requiredManualEnvVars { serviceType serviceName key } } }`, bp, sync)})
		var codes []string
		for _, e := range out.Errors {
			code, _ := e.Extensions["code"].(string)
			codes = append(codes, code+": "+e.Message)
		}
		data, _ := out.Data.(map[string]any)
		got, _ := data["blueprintSync"].(map[string]any)
		return got, codes
	}

	withSecret := strings.Replace(recreateManifest, "url: docker.io/library/nginx:1.27", "url: docker.io/library/nginx:1.27\n    envVars:\n      - key: API_TOKEN\n        sync: false", 1)
	svc.GitFetcher = fakeBlueprintFetcher{contents: withSecret}
	plan, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "read", OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo}})
	if err != nil || plan.Sync == nil {
		t.Fatalf("planned create = %+v, %v", plan, err)
	}
	bp, waiting := plan.Blueprint.ID, plan.Sync.ID

	got, errs := read(bp, waiting)
	if len(errs) > 0 {
		t.Fatalf("read the waiting plan: %v", errs)
	}
	want := fmt.Sprintf(`map[actionStates:[map[state:created]] actions:[map[kind:service name:api operation:create]] actionsRecorded:true commitId:%s commitMessage:%s dashboardUrl:https://dashboard.example/blueprint/%s/sync/%s errors:[] id:%s requiredManualEnvVars:[map[key:API_TOKEN serviceName:api serviceType:web]] state:created]`,
		testCommitSHA, syncPageCommitMessage, bp, waiting, waiting)
	if fmt.Sprint(got) != want {
		t.Fatalf("the waiting plan reads\n%v\nwant\n%s", got, want)
	}

	// Syncs on bex's GraphQL lists the waiting plan; REST and MCP do not.
	listed := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: fmt.Sprintf(`{ blueprintSyncs(id:%q) { id state commitMessage } }`, bp)})
	if fmt.Sprint(listed.Data) != fmt.Sprintf("map[blueprintSyncs:[map[commitMessage:%s id:%s state:created]]]", syncPageCommitMessage, waiting) || len(listed.Errors) > 0 {
		t.Fatalf("GraphQL blueprintSyncs = %v %v, want the waiting plan", listed.Data, listed.Errors)
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/blueprints/"+bp+"/syncs?ownerId="+wsID, nil).WithContext(ctx))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("REST syncs = %d %s, want none", rec.Code, rec.Body)
	}
	session := blueprintMCPSession(t, svc, ctx)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_blueprint_syncs", Arguments: map[string]any{"id": bp}})
	if raw, _ := json.Marshal(res.StructuredContent); err != nil || res.IsError || !strings.Contains(string(raw), `"syncs":[]`) {
		t.Fatalf("MCP list_blueprint_syncs = %s, %v; want none", raw, err)
	}

	// Approved, it reads as a success; each action succeeded with it.
	token := "s3cret"
	if _, err := svc.ApproveBlueprint(ctx, bp, wsID, waiting, []ManualEnvVar{{ServiceType: "web", ServiceName: "api", Key: "API_TOKEN", Value: &token}}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	got, _ = read(bp, waiting)
	if got["state"] != store.BlueprintSyncStateSuccess || fmt.Sprint(got["actionStates"]) != "[map[state:success]]" || fmt.Sprint(got["requiredManualEnvVars"]) != "[]" {
		t.Fatalf("the approved plan reads %v", got)
	}

	// A plan of a broken file fails with its located notice.
	svc.GitFetcher = fakeBlueprintFetcher{contents: "services:\n  - type: web\n    name: api\n    runtime: rocket\n"}
	failed, err := svc.PlanBlueprint(ctx, bp, wsID)
	if err != nil || failed.PlanningOutcome != PlanningFailed || failed.Sync == nil {
		t.Fatalf("plan a broken file = %+v, %v", failed, err)
	}
	got, _ = read(bp, failed.Sync.ID)
	notices, _ := got["errors"].([]any)
	first, _ := slices.Concat(notices, []any{nil})[0].(map[string]any)
	if got["state"] != store.BlueprintSyncStateError || got["actionsRecorded"] != false || len(notices) == 0 ||
		first["severity"] != "error" || first["line"] == nil || first["message"] == "" || strings.HasPrefix(fmt.Sprint(first["message"]), "planning failed") {
		t.Fatalf("the failed plan reads %v, want its located notices", got)
	}

	// A sync of another Blueprint is not found from this one.
	svc.GitFetcher = fakeBlueprintFetcher{contents: manifestNaming("other")}
	other, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "other", OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo + "-other"}})
	if err != nil || other.Sync == nil {
		t.Fatalf("second planned create = %+v, %v", other, err)
	}
	if got, errs := read(bp, other.Sync.ID); got != nil || len(errs) != 1 || !strings.HasPrefix(errs[0], "NOT_FOUND") && !strings.Contains(errs[0], "not found") {
		t.Fatalf("another Blueprint's sync reads %v %v, want not found", got, errs)
	}

	// Another workspace's Blueprint answers as its history does.
	outsider, err := st.CreateWorkspace(ctx, "sync-read-outsider-"+wsID, store.PlanHobby, "sync-read-outsider-"+wsID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), outsider.ID) })
	svc.Workspace = fakeWorkspace{"recreate-owner": wsID, "sync-read-outsider": outsider.ID}
	stranger := core.WithIdentity(context.Background(), core.Identity{Subject: "sync-read-outsider", Method: "oauth2"})
	_, historyErr := svc.ListBlueprintSyncs(stranger, bp, "", "", 0)
	_, readErr := svc.GetBlueprintSync(stranger, bp, "", waiting)
	if historyErr == nil || readErr == nil || readErr.Error() != historyErr.Error() {
		t.Fatalf("a stranger's read = %v, the history's = %v; want the same refusal", readErr, historyErr)
	}
}

// w3/m678 t007: the planned and the approved sync's dashboardUrl on REST,
// GraphQL and MCP is that sync's page, Render's /blueprint/<id>/sync/<syncId>
// ("Link to this Sync in the Render Dashboard", the pinned CLI's
// SyncWithPlan; "Link to the sync in the Render Dashboard", its ApprovedSync).
func TestBlueprintSyncLinksOpenTheSyncOnEverySurfacePG(t *testing.T) {
	svc, _, _, ctx, wsID := newRecreatePGService(t)
	svc.Metadata = resourcemeta.Config{DashboardBaseURL: "https://dashboard.example"}
	schema := blueprintSchema(t, svc)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	session := blueprintMCPSession(t, svc, ctx)
	repo := "https://github.com/acme/sync-links-" + wsID
	page := func(bp, sync string) string { return "https://dashboard.example/blueprint/" + bp + "/sync/" + sync }

	plan, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "links", OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo}})
	if err != nil || plan.Sync == nil {
		t.Fatalf("planned create = %+v, %v", plan, err)
	}
	bp := plan.Blueprint.ID
	if plan.Sync.DashboardURL != page(bp, plan.Sync.ID) {
		t.Errorf("create's sync links %q", plan.Sync.DashboardURL)
	}
	type sync struct {
		ID           string `json:"id"`
		DashboardURL string `json:"dashboardUrl"`
	}
	check := func(surface string, got sync) {
		t.Helper()
		if !strings.HasPrefix(got.ID, "exe-") || got.DashboardURL != page(bp, got.ID) {
			t.Errorf("%s: sync %s links %q, want %q", surface, got.ID, got.DashboardURL, page(bp, got.ID))
		}
	}
	replan := func() {
		t.Helper()
		svc.GitFetcher = fakeBlueprintFetcher{contents: manifestNaming(fmt.Sprintf("svc%d", time.Now().UnixNano()%100000))}
	}

	// REST: plan, then approve.
	replan()
	rec := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/"+bp+"/syncs?ownerId="+wsID, nil).WithContext(ctx))
	var restPlan struct{ Sync sync }
	if err := json.Unmarshal(rec.Body.Bytes(), &restPlan); err != nil || rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("REST plan = %d %s", rec.Code, rec.Body)
	}
	check("REST plan", restPlan.Sync)
	rec = httptest.NewRecorder()
	approve := httptest.NewRequest(http.MethodPost, "/v1/blueprints/"+bp+"/approve?ownerId="+wsID, strings.NewReader(`{}`)).WithContext(ctx)
	approve.Header.Set("Content-Type", "application/json")
	validated(t, mux).ServeHTTP(rec, approve)
	var restApproved sync
	if err := json.Unmarshal(rec.Body.Bytes(), &restApproved); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("REST approve = %d %s", rec.Code, rec.Body)
	}
	check("REST approve", restApproved)

	// GraphQL: plan, then approve.
	replan()
	gql := func(query string, field string, path ...string) sync {
		t.Helper()
		out := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: query})
		if len(out.Errors) > 0 {
			t.Fatalf("GraphQL %s: %v", field, out.Errors)
		}
		v := out.Data.(map[string]any)[field]
		for _, p := range path {
			v = v.(map[string]any)[p]
		}
		raw, _ := json.Marshal(v)
		var got sync
		_ = json.Unmarshal(raw, &got)
		return got
	}
	check("GraphQL planBlueprintSync", gql(fmt.Sprintf(`mutation { planBlueprintSync(id:%q) { sync { id dashboardUrl } } }`, bp), "planBlueprintSync", "sync"))
	check("GraphQL approveBlueprint", gql(fmt.Sprintf(`mutation { approveBlueprint(id:%q) { id dashboardUrl } }`, bp), "approveBlueprint"))

	// MCP: plan_blueprint, then approve_blueprint.
	replan()
	call := func(name string) map[string]any {
		t.Helper()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: map[string]any{"blueprintId": bp}})
		if err != nil || res.IsError {
			t.Fatalf("MCP %s = %+v, %v", name, res, err)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		return out
	}
	asSync := func(v any) sync {
		raw, _ := json.Marshal(v)
		var got sync
		_ = json.Unmarshal(raw, &got)
		return got
	}
	check("MCP plan_blueprint", asSync(call("plan_blueprint")["sync"]))
	check("MCP approve_blueprint", asSync(call("approve_blueprint")))
}
