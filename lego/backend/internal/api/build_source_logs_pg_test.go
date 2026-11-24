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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestBuildSourceLogsPGAndOpenFGAE2E proves w2/m611 against real PostgreSQL
// (ListDeploys, BuiltDeployIDs, ListProjectAppIDs) and real OpenFGA (viewer
// can_view_logs vs foreign-workspace refusal) across REST, GraphQL, and MCP.
func TestBuildSourceLogsPGAndOpenFGAE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()
	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	run := uniqueRunSuffix()
	admin := "bds-logs-admin-" + run
	contributor := "bds-logs-contrib-" + run
	viewer := "bds-logs-viewer-" + run
	outsider := "bds-logs-outsider-" + run

	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	ws, err := st.CreateWorkspace(ctx, "bds-logs-"+run, store.PlanHobby, admin)
	if err != nil {
		t.Fatal(err)
	}
	otherWS, err := st.CreateWorkspace(ctx, "bds-logs-other-"+run, store.PlanHobby, outsider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id IN ($1, $2)`, ws.ID, otherWS.ID)
		_ = st.DeleteTenant(context.Background(), ws.ID)
		_ = st.DeleteTenant(context.Background(), otherWS.ID)
	})
	if err := st.AddMember(ctx, contributor, ws.ID, "contributor"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddMember(ctx, viewer, ws.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	for _, g := range []struct{ ws, sub, role string }{
		{ws.ID, admin, "admin"},
		{ws.ID, contributor, "contributor"},
		{ws.ID, viewer, "viewer"},
		{otherWS.ID, outsider, "admin"},
	} {
		if err := roles.GrantWorkspaceRole(ctx, g.ws, "user:"+g.sub, g.role); err != nil {
			t.Fatalf("grant %s %s: %v", g.sub, g.role, err)
		}
	}

	prj, err := st.CreateProject(ctx, ws.ID, "proj-"+run)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	bdsID := ids.New(ids.BuildSource)
	bdsCRName := ws.ID + "-" + bdsID
	repoURL := "https://github.com/bex-co/monorepo"

	if _, err := st.CreateApp(ctx, store.App{
		ID:                bdsID,
		DisplayName:       "MonoSource",
		TenantID:          ws.ID,
		ScopeProjectID:    prj.ID,
		Name:              bdsID,
		Type:              store.TypeBuildSource,
		Repo:              repoURL,
		Branch:            "main",
		Port:              8080,
		Replicas:          1,
		Tier:              "free",
		FirstDeployCommit: store.CommitInfo{Hash: "1111111aaaa"},
	}); err != nil {
		t.Fatalf("CreateApp carrier: %v", err)
	}
	siblingRow, err := st.CreateApp(ctx, store.App{
		TenantID:  ws.ID,
		ProjectID: prj.ID,
		Name:      "api-" + run,
		Type:      appv1alpha1.TypeWebService,
		Image:     "nginx:alpine",
		Port:      8080,
		Replicas:  1,
		Tier:      "free",
	})
	if err != nil {
		t.Fatalf("CreateApp sibling: %v", err)
	}
	srvID := siblingRow.ID
	srvCRName := ws.ID + "-" + srvID

	initialDeploys, err := st.ListDeploys(ctx, bdsID, store.DeployFilter{Limit: 10})
	if err != nil || len(initialDeploys) != 1 {
		t.Fatalf("ListDeploys initial = %v (err %v), want 1 initial deploy", initialDeploys, err)
	}
	d1 := initialDeploys[0]
	if ok, err := st.TransitionDeploy(ctx, d1.ID, store.DeployBuildInProgress, store.RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("TransitionDeploy 1 build: ok=%v err=%v", ok, err)
	}
	if ok, err := st.TransitionDeploy(ctx, d1.ID, store.DeployBuildFailed, store.RanImage{}, "build error", "", "", nil); err != nil || !ok {
		t.Fatalf("TransitionDeploy 1 fail: ok=%v err=%v", ok, err)
	}
	if _, err := st.InsertServiceEventFact(ctx, store.ServiceEventFact{
		SourceKey: "deploy:" + d1.ID + ":build_started",
		AppID:     bdsID,
		Type:      store.EventFactBuildStarted,
		At:        d1.CreatedAt.Add(time.Second),
		DeployID:  d1.ID,
	}); err != nil {
		t.Fatalf("InsertServiceEventFact 1: %v", err)
	}

	d2, err := st.CreateDeploy(ctx, bdsID, store.TriggerManual, "", 2, store.CommitInfo{
		Hash: "2222222bbbb",
	}, admin)
	if err != nil {
		t.Fatalf("CreateDeploy 2: %v", err)
	}
	if ok, err := st.TransitionDeploy(ctx, d2.ID, store.DeployBuildInProgress, store.RanImage{}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("TransitionDeploy 2 build: ok=%v err=%v", ok, err)
	}
	if ok, err := st.TransitionDeploy(ctx, d2.ID, store.DeployLive, store.RanImage{Ref: "zot.test/mono@sha256:abc"}, "", "", "", nil); err != nil || !ok {
		t.Fatalf("TransitionDeploy 2 live: ok=%v err=%v", ok, err)
	}
	if _, err := st.InsertServiceEventFact(ctx, store.ServiceEventFact{
		SourceKey: "deploy:" + d2.ID + ":build_started",
		AppID:     bdsID,
		Type:      store.EventFactBuildStarted,
		At:        d2.CreatedAt.Add(time.Second),
		DeployID:  d2.ID,
	}); err != nil {
		t.Fatalf("InsertServiceEventFact 2: %v", err)
	}
	bld2, _ := ids.Rekind(ids.Build, d2.ID)

	carrier := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bdsCRName,
			Namespace: "default",
			Labels: map[string]string{
				core.LabelTenant:      ws.ID,
				store.LabelAppID:      bdsID,
				core.LabelServiceName: bdsID,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeBuildSource,
			DisplayName: "MonoSource",
			Repo:        repoURL,
			Branch:      "main",
		},
	}
	sibling := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      srvCRName,
			Namespace: "default",
			Labels: map[string]string{
				core.LabelTenant:      ws.ID,
				store.LabelAppID:      srvID,
				core.LabelServiceName: srvID,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeWebService,
			DisplayName: "ApiService",
			Image:       "nginx:alpine",
		},
	}

	job1 := appv1alpha1.BuildJobName(carrier.Name, appv1alpha1.BuildRevision(1))
	job2 := appv1alpha1.BuildJobName(carrier.Name, appv1alpha1.BuildRevision(2))
	pod2 := job2 + "-pg2a"
	now := time.Now().UTC()

	loki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/loki/api/v1/query_range") {
			q := r.URL.Query().Get("query")
			var streams []map[string]any
			if strings.Contains(q, job2) && !strings.Contains(q, job1) {
				streams = append(streams, map[string]any{
					"stream": map[string]string{
						"app": carrier.Name, "type": logs.LogTypeBuild, "pod": pod2, "container": "build",
					},
					"values": [][]string{
						{fmt.Sprintf("%d", now.Add(-10*time.Second).UnixNano()), "pg-e2e compiling bld2"},
					},
				})
			}
			if strings.Contains(q, sibling.Name) {
				streams = append(streams, map[string]any{
					"stream": map[string]string{
						"app": sibling.Name, "type": logs.LogTypeApp, "pod": sibling.Name + "-pod1", "container": "app",
					},
					"values": [][]string{
						{fmt.Sprintf("%d", now.Add(-5*time.Second).UnixNano()), "sibling runtime line"},
					},
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"resultType": "streams", "result": streams},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/label/type/values") {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{logs.LogTypeBuild}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{}})
	}))
	t.Cleanup(loki.Close)

	resolver := tenancy.NewTenantService(st, roles)
	base := &core.Base{
		Client:    fakeClient(carrier, sibling),
		Namespace: "default",
		Authz:     checker,
		Workspace: resolver,
	}
	srv := NewServer(base, Deps{
		Store:          st,
		DeployStore:    st,
		ProjectsStore:  st,
		WorkspaceStore: st,
		LogHistory:     logs.NewLokiSource(loki.URL, loki.Client()),
		LogLabelValues: logs.NewLokiLabelValuesSource(loki.URL, loki.Client()),
	})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema

	restMux, err := newRenderRequestValidator(srv.restHandler())
	if err != nil {
		t.Fatalf("newRenderRequestValidator: %v", err)
	}
	contribID := core.Identity{Subject: contributor, Method: "session"}
	viewerID := core.Identity{Subject: viewer, Method: "session"}
	outsiderID := core.Identity{Subject: outsider, Method: "session"}

	startTime := now.Add(-5 * time.Minute).Format(time.RFC3339)
	endTime := now.Add(5 * time.Minute).Format(time.RFC3339)

	// 1. Contributor with real OpenFGA can_view_logs reads bld2 over REST, GraphQL, MCP.
	restPath := "/v1/logs?resource=" + bdsID + "&build=" + bld2 +
		"&startTime=" + url.QueryEscape(startTime) + "&endTime=" + url.QueryEscape(endTime)
	req := httptest.NewRequest(http.MethodGet, restPath, nil)
	rec := httptest.NewRecorder()
	restMux.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, contribID)))
	if rec.Code != http.StatusOK {
		t.Fatalf("contributor REST GET /v1/logs = %d: %s", rec.Code, rec.Body.String())
	}
	var restPage struct {
		Logs []struct {
			Message string              `json:"message"`
			Labels  []map[string]string `json:"labels"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &restPage); err != nil {
		t.Fatalf("decode REST page: %v", err)
	}
	if len(restPage.Logs) == 0 {
		t.Fatalf("expected logs for bld2, got empty page")
	}
	var sawPod, sawSucceeded bool
	for _, entry := range restPage.Logs {
		if strings.Contains(entry.Message, "pg-e2e compiling bld2") {
			sawPod = true
		}
		if strings.Contains(entry.Message, "Build succeeded") {
			sawSucceeded = true
		}
		if strings.Contains(entry.Message, "Your service is live") {
			t.Errorf("service-only rollout line leaked: %q", entry.Message)
		}
		var buildVal string
		for _, kv := range entry.Labels {
			if kv["name"] == "build" {
				buildVal = kv["value"]
			}
		}
		if buildVal != bld2 {
			t.Errorf("entry %q build label = %q, want %q", entry.Message, buildVal, bld2)
		}
	}
	if !sawPod || !sawSucceeded {
		t.Fatalf("expected pod line (%v) and Build succeeded narration (%v), got %+v", sawPod, sawSucceeded, restPage.Logs)
	}

	// Render OpenAPI refuses label=build with 400; label=type&build=bld2 answers ["build"].
	badLabelRec := httptest.NewRecorder()
	restMux.ServeHTTP(badLabelRec, httptest.NewRequest(http.MethodGet,
		"/v1/logs/values?resource="+bdsID+"&label=build", nil).WithContext(core.WithIdentity(ctx, contribID)))
	if badLabelRec.Code != http.StatusBadRequest {
		t.Fatalf("label=build status = %d, want 400: %s", badLabelRec.Code, badLabelRec.Body.String())
	}
	typeValRec := httptest.NewRecorder()
	restMux.ServeHTTP(typeValRec, httptest.NewRequest(http.MethodGet,
		"/v1/logs/values?resource="+bdsID+"&label=type&build="+bld2, nil).WithContext(core.WithIdentity(ctx, contribID)))
	if typeValRec.Code != http.StatusOK {
		t.Fatalf("label=type status = %d: %s", typeValRec.Code, typeValRec.Body.String())
	}
	var typeVals []string
	if err := json.Unmarshal(typeValRec.Body.Bytes(), &typeVals); err != nil || !slices.Equal(typeVals, []string{"build"}) {
		t.Fatalf("label=type values = %v (err %v), want [build]", typeVals, err)
	}

	// GraphQL parity as contributor.
	gqlDoc := fmt.Sprintf(`{
		logs(resource: %q, build: [%q], startTime: %q, endTime: %q) { logs { resource type build message } }
		logLabelValues(resource: %q, label: "type", build: [%q])
	}`, bdsID, bld2, startTime, endTime, bdsID, bld2)
	gqlOut := graphqlDo(t, srv, contribID, gqlDoc)
	if errs, _ := gqlOut["errors"].([]any); len(errs) > 0 {
		t.Fatalf("GraphQL contributor errors: %v", errs)
	}
	gqlRaw, _ := json.Marshal(gqlOut)
	var gqlResp struct {
		Data struct {
			Logs struct {
				Logs []struct {
					Resource string `json:"resource"`
					Type     string `json:"type"`
					Build    string `json:"build"`
					Message  string `json:"message"`
				} `json:"logs"`
			} `json:"logs"`
			LogLabelValues []string `json:"logLabelValues"`
		} `json:"data"`
	}
	if err := json.Unmarshal(gqlRaw, &gqlResp); err != nil {
		t.Fatalf("decode GraphQL response: %v", err)
	}
	if len(gqlResp.Data.Logs.Logs) != len(restPage.Logs) {
		t.Errorf("GraphQL logs = %d, want REST count %d", len(gqlResp.Data.Logs.Logs), len(restPage.Logs))
	}
	if !slices.Equal(gqlResp.Data.LogLabelValues, []string{"build"}) {
		t.Errorf("GraphQL logLabelValues = %v, want [build]", gqlResp.Data.LogLabelValues)
	}

	// MCP parity as contributor.
	contribMCP := mcpSessionIdentity(t, srv, contribID)
	mcpRes, err := contribMCP.CallTool(ctx, &mcp.CallToolParams{
		Name: "list_logs",
		Arguments: map[string]any{
			"resource":  []string{bdsID},
			"build":     []string{bld2},
			"startTime": startTime,
			"endTime":   endTime,
		},
	})
	if err != nil || mcpRes.IsError {
		t.Fatalf("MCP list_logs contributor failed: err=%v res=%+v", err, mcpRes)
	}

	// 2. Both a workspace viewer (lacking can_view_logs in OpenFGA) and a foreign
	// workspace outsider are refused on REST, GraphQL, and MCP.
	for _, denied := range []struct {
		name string
		id   core.Identity
	}{
		{"viewer", viewerID},
		{"outsider", outsiderID},
	} {
		outREST := httptest.NewRecorder()
		restMux.ServeHTTP(outREST, httptest.NewRequest(http.MethodGet, restPath, nil).WithContext(core.WithIdentity(ctx, denied.id)))
		if outREST.Code != http.StatusForbidden {
			t.Fatalf("%s REST status = %d, want 403: %s", denied.name, outREST.Code, outREST.Body.String())
		}
		outGQL := graphqlDo(t, srv, denied.id, fmt.Sprintf(`{ logs(resource: %q) { logs { message } } }`, bdsID))
		if errs, _ := outGQL["errors"].([]any); len(errs) == 0 {
			t.Fatalf("%s GraphQL unexpectedly succeeded: %v", denied.name, outGQL)
		}
		deniedMCP := mcpSessionIdentity(t, srv, denied.id)
		outMCPRes, err := deniedMCP.CallTool(ctx, &mcp.CallToolParams{
			Name:      "list_logs",
			Arguments: map[string]any{"resource": []string{bdsID}},
		})
		if err != nil {
			t.Fatalf("%s MCP transport error: %v", denied.name, err)
		}
		if !outMCPRes.IsError {
			t.Fatalf("%s MCP unexpectedly succeeded: %+v", denied.name, outMCPRes)
		}
	}

	// 3. Real PGStore.ListProjectAppIDs excludes Build Source carriers from
	// project-wide log queries and rejects explicit project + bds-… combos.
	projRec := httptest.NewRecorder()
	restMux.ServeHTTP(projRec, httptest.NewRequest(http.MethodGet,
		"/v1/logs?project="+prj.ID+"&startTime="+url.QueryEscape(startTime)+"&endTime="+url.QueryEscape(endTime),
		nil).WithContext(core.WithIdentity(ctx, contribID)))
	if projRec.Code != http.StatusOK {
		t.Fatalf("project logs status = %d: %s", projRec.Code, projRec.Body.String())
	}
	if !strings.Contains(projRec.Body.String(), "sibling runtime line") {
		t.Errorf("project logs missed sibling service line: %s", projRec.Body.String())
	}
	if strings.Contains(projRec.Body.String(), bdsID) || strings.Contains(projRec.Body.String(), "pg-e2e compiling") {
		t.Errorf("project logs leaked Build Source carrier: %s", projRec.Body.String())
	}
	projAndBDSRec := httptest.NewRecorder()
	restMux.ServeHTTP(projAndBDSRec, httptest.NewRequest(http.MethodGet,
		"/v1/logs?project="+prj.ID+"&resource="+bdsID, nil).WithContext(core.WithIdentity(ctx, contribID)))
	if projAndBDSRec.Code != http.StatusBadRequest {
		t.Fatalf("project + bds status = %d, want 400: %s", projAndBDSRec.Code, projAndBDSRec.Body.String())
	}
}
