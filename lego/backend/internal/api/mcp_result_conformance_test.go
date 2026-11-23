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
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// mcp_result_conformance_test.go — composed-server MCP result-shape and
// structuredContent conformance suite covering all 28 shared Render tools
// through Server.MCPServer() (w5/m601 t003). Every answer runs through the
// full MCP middleware, workspace binding, and JSON-RPC transport, and is
// asserted against rendertest.AssertRenderMCPResult. A coverage guard fails
// if any shared tool in render-mcp-results.json is untested or if any
// form:"object" tool lacks a recorded type.

type conformMCPDeployStore struct {
	*conformMutableDeployStore
	queueNext bool
}

func (s *conformMCPDeployStore) CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, triggeredBy string, clearCache ...bool) (store.Deploy, error) {
	d, err := s.conformMutableDeployStore.CreateDeploy(ctx, appID, trigger, image, generation, commit, triggeredBy, clearCache...)
	if err != nil {
		return d, err
	}
	if s.queueNext {
		s.queueNext = false
		s.mu.Lock()
		d.Status = store.DeployQueued
		if len(s.byApp[appID]) > 0 && s.byApp[appID][0].ID == d.ID {
			s.byApp[appID][0].Status = store.DeployQueued
		}
		s.mu.Unlock()
	}
	return d, nil
}

type conformMCPBlueprintStore struct {
	*conformBlueprintStore
	planned map[string]store.PlannedBlueprintSync
	order   []string
	claims  map[string][]store.BlueprintResourceClaim
}

func newConformMCPBlueprintStore(ws string) *conformMCPBlueprintStore {
	base, _ := newConformBlueprintStore(ws)
	return &conformMCPBlueprintStore{
		conformBlueprintStore: base,
		planned:               map[string]store.PlannedBlueprintSync{},
		claims:                map[string][]store.BlueprintResourceClaim{},
	}
}

func (s *conformMCPBlueprintStore) CreatePlannedBlueprint(_ context.Context, b store.Blueprint) (store.Blueprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.ID == "" {
		b.ID = ids.New(ids.Blueprint)
	}
	b.Status = store.BlueprintStatusCreated
	b.ExecutionGeneration = 1
	b.CreatedAt = conformEpoch
	b.UpdatedAt = conformEpoch
	if b.ExistingResources == store.BlueprintExistingCreateNew && b.ResourceSuffix == "" {
		b.ResourceSuffix = "mcp1"
	}
	s.bps[b.ID] = b
	return b, nil
}

func (s *conformMCPBlueprintStore) UndoPlannedBlueprintCreate(_ context.Context, created store.Blueprint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.bps, created.ID)
	return nil
}

func (s *conformMCPBlueprintStore) InsertPlannedBlueprintSync(_ context.Context, p store.PlannedBlueprintSync) (store.PlannedBlueprintSync, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = ids.New(ids.BlueprintSync)
	}
	p.CreatedAt = conformEpoch
	if p.StartedAt.IsZero() {
		p.StartedAt = conformEpoch
	}
	s.planned[p.ID] = p
	s.order = append(s.order, p.ID)
	s.syncs[p.BlueprintID] = append(s.syncs[p.BlueprintID], p.BlueprintSync)
	return p, nil
}

func (s *conformMCPBlueprintStore) LatestBlueprintPlan(_ context.Context, blueprintID string, _ time.Time) (store.PlannedBlueprintSync, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.order) - 1; i >= 0; i-- {
		p := s.planned[s.order[i]]
		if p.BlueprintID == blueprintID {
			return p, nil
		}
	}
	return store.PlannedBlueprintSync{}, store.ErrNotFound
}

func (s *conformMCPBlueprintStore) BlueprintPlannedSync(_ context.Context, blueprintID, syncID string) (store.PlannedBlueprintSync, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.planned[syncID]
	if !ok || p.BlueprintID != blueprintID {
		return store.PlannedBlueprintSync{}, store.ErrNotFound
	}
	return p, nil
}

func (s *conformMCPBlueprintStore) SettleNoChangesPlan(_ context.Context, blueprintID, tenantID, manifest string) (store.Blueprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bps[blueprintID]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, store.ErrNotFound
	}
	now := conformEpoch
	b.NoChangesPlannedAt = &now
	b.Manifest = manifest
	b.UpdatedAt = conformEpoch
	s.bps[blueprintID] = b
	return b, nil
}

func (s *conformMCPBlueprintStore) AdmitBlueprintSyncRun(_ context.Context, blueprintID, tenantID string, run store.BlueprintSync) (store.Blueprint, store.BlueprintSync, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bps[blueprintID]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, store.BlueprintSync{}, store.ErrNotFound
	}
	b.ExecutionGeneration++
	if run.ID == "" {
		run.ID = ids.New(ids.BlueprintSync)
	}
	run.BlueprintID = blueprintID
	run.State = store.BlueprintSyncStateRunning
	run.StartedAt = conformEpoch
	b.ActiveRunID = run.ID
	b.Status = store.BlueprintStatusSyncing
	s.bps[blueprintID] = b
	if p, exists := s.planned[run.ID]; exists {
		p.BlueprintSync = run
		s.planned[run.ID] = p
	}
	return b, run, nil
}

func (s *conformMCPBlueprintStore) StageBlueprintManifest(_ context.Context, id, tenantID string, _ int64, _, manifest string) (store.Blueprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bps[id]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, store.ErrNotFound
	}
	b.Manifest = manifest
	s.bps[id] = b
	return b, nil
}

func (s *conformMCPBlueprintStore) AssertBlueprintExecution(context.Context, string, string, int64, string) error {
	return nil
}

func (s *conformMCPBlueprintStore) CompleteBlueprintSync(_ context.Context, id, tenantID, runID string, _ int64, state string, completedAt time.Time, errMsg *string) (store.Blueprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bps[id]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, store.ErrNotFound
	}
	b.ActiveRunID = ""
	b.LastSyncAt = &completedAt
	b.UpdatedAt = completedAt
	if state == store.BlueprintSyncStateSuccess {
		if b.AutoSync {
			b.Status = store.BlueprintStatusInSync
		} else {
			b.Status = store.BlueprintStatusPaused
		}
	} else {
		b.Status = store.BlueprintStatusError
	}
	s.bps[id] = b
	if p, exists := s.planned[runID]; exists {
		p.State = state
		p.CompletedAt = &completedAt
		p.ErrorMessage = errMsg
		s.planned[runID] = p
	}
	return b, nil
}

func (s *conformMCPBlueprintStore) FailAdmittedSync(context.Context, string, string, string, int64, time.Time, *string) error {
	return nil
}

func (s *conformMCPBlueprintStore) ClaimBlueprintResource(_ context.Context, tenantID string, kind store.BlueprintClaimKind, name, blueprintID, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.claims[blueprintID] {
		if c.Kind == kind && c.Name == name {
			return nil
		}
	}
	s.claims[blueprintID] = append(s.claims[blueprintID], store.BlueprintResourceClaim{
		TenantID: tenantID, Kind: kind, Name: name, BlueprintID: blueprintID,
	})
	return nil
}

func (s *conformMCPBlueprintStore) ListBlueprintResourceClaims(_ context.Context, _, blueprintID string) ([]store.BlueprintResourceClaim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.claims[blueprintID]), nil
}

func (s *conformMCPBlueprintStore) GetBlueprintResourceOwner(_ context.Context, _ string, kind store.BlueprintClaimKind, name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for bpID, list := range s.claims {
		for _, c := range list {
			if c.Kind == kind && c.Name == name {
				return bpID, nil
			}
		}
	}
	return "", nil
}

func (s *conformMCPBlueprintStore) BlueprintResourceOwners(_ context.Context, tenantID string) (map[store.BlueprintResourceKey]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[store.BlueprintResourceKey]string{}
	for bpID, list := range s.claims {
		for _, c := range list {
			if c.TenantID == tenantID {
				out[c.Key()] = bpID
			}
		}
	}
	return out, nil
}

func (s *conformMCPBlueprintStore) BlueprintEnvGroupClaims(context.Context, string, string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *conformMCPBlueprintStore) RecordBlueprintEnvGroupClaims(context.Context, string, string, map[string]string) error {
	return nil
}

// TestMCPResultConformance exercises every one of the 28 shared Render MCP
// tools through Server.MCPServer() over an in-memory JSON-RPC transport,
// validating result forms, keys, nested types, and structuredContent rules
// against rendertest.AssertRenderMCPResult, and guards that zero shared tools
// are untested and zero form:"object" tools lack a recorded type.
func TestMCPResultConformance(t *testing.T) {
	const (
		subject = "user-mcp-conform"
		webName = "web"
		webID   = "srv-mcp-web"
		cronID  = "crn-mcp-nightly"
		depID   = "dep-mcp-1"
	)

	wsStore := newFakeWSStore()
	ws := mustCreate(t, wsStore, "Acme", store.PlanPro, subject)

	web := conformApp(webName, webID)
	web.Labels[core.LabelTenant] = ws.ID
	web.Spec = appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeWebService, Image: "nginx:1.27", Tier: "starter", Replicas: 1, Port: 8080,
	}

	cronApp := conformApp("nightly", cronID)
	cronApp.Labels[core.LabelTenant] = ws.ID
	cronApp.Spec = appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeCronJob, Image: "busybox:1.36", Tier: "starter", Schedule: "0 * * * *", Command: "date", Replicas: 1,
	}
	cronApp.Status.Runs = []appv1alpha1.CronRun{{
		Name:      "nightly-run-live",
		Status:    "Running",
		StartedAt: conformEpoch.Add(-2 * time.Minute).Format(time.RFC3339),
	}}

	webPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-pod-1",
			Namespace: "default",
			Labels:    map[string]string{"app.kubernetes.io/name": webName},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			}},
		},
	}

	cl := fakeClient(web, cronApp, webPod)

	depStore := &conformMCPDeployStore{conformMutableDeployStore: newConformMutableDeployStore()}
	depStore.byApp[webID] = []store.Deploy{conformDeploy(depID, webID)}

	eventStore := &conformEventStore{rows: []store.ServiceEventRow{{
		Key:      depID + ":" + store.EventPhaseStarted,
		At:       conformEpoch,
		Source:   store.EventSourceDeploy,
		Phase:    store.EventPhaseStarted,
		DeployID: depID,
		Trigger:  store.TriggerAPI,
	}}}

	bpStore := newConformMCPBlueprintStore(ws.ID)

	srv := NewServer(
		&core.Base{
			Client:    cl,
			Namespace: "default",
			Workspace: &apiFakeResolver{store: wsStore},
			Clock:     func() time.Time { return conformEpoch },
			Plans:     proPlan(),
		},
		Deps{
			WorkspaceStore:  wsStore,
			Identities:      metadataIdentities{},
			Secrets:         newMemSecretStore(),
			DeployStore:     depStore,
			EventStore:      eventStore,
			BlueprintsStore: bpStore,
			LogHistory: func(_ context.Context, _ string, _ logs.LogQuery) ([]logs.LogEntry, error) {
				return []logs.LogEntry{{
					Timestamp: conformEpoch.Format(time.RFC3339Nano),
					Message:   "server started",
					Labels:    map[string]string{"type": "app", "level": "info", "instance": "web-pod-1"},
				}}, nil
			},
			LogLabelValues: func(_ context.Context, _, _ string, _ logs.LogQuery) ([]string, error) {
				return []string{"info", "error"}, nil
			},
			ResourceMetrics: func(_ context.Context, _, _ string) ([]metrics.PodResourceUsage, error) {
				return []metrics.PodResourceUsage{{Pod: "web-pod-1", CPUCores: 0.25, MemoryBytes: 128 << 20}}, nil
			},
			BaseDomain:   "onbex.co",
			Region:       "fsn1",
			DashboardURL: "https://dashboard.bex.co",
		},
	)
	bpYAML := "services:\n  - type: web\n    name: bp-web\n    runtime: image\n    image:\n      url: nginx:1.27\n"
	srv.Apps.GitFetcher = movingBlueprintFile{contents: &bpYAML}
	srv.Postgres.SetQueryExecutorForTest(func(_ context.Context, _, _ string, _ bool) (postgres.QueryResult, error) {
		return postgres.QueryResult{
			Columns:  []string{"id", "label"},
			Rows:     [][]any{{int64(1), "ok"}},
			RowCount: 1,
		}, nil
	})

	cs := mcpSessionAs(t, srv, subject)

	tested := map[string]bool{}
	call := func(t *testing.T, tool string, args map[string]any, fixtureLacks ...string) *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("%s: protocol error: %v", tool, err)
		}
		if res.IsError {
			t.Fatalf("%s: tool error: %s", tool, rendertest.MCPText(t, res))
		}
		rendertest.AssertRenderMCPResult(t, tool, res, fixtureLacks...)
		tested[tool] = true
		return res
	}

	// 1. Workspaces.
	call(t, "list_workspaces", nil)

	// 2. Key Value: emptyText branch, create, get, non-empty list.
	emptyKV := call(t, "list_key_value", nil)
	if got := rendertest.MCPText(t, emptyKV); got != "No Key Value instances found" {
		t.Fatalf("empty list_key_value = %q, want Render's emptyText", got)
	}
	createdKV := call(t, "create_key_value", map[string]any{"name": "kv-conform", "plan": "free"}, "createdAt")
	var kvObj map[string]any
	rendertest.DecodeMCPText(t, createdKV, &kvObj)
	kvID, _ := kvObj["id"].(string)
	call(t, "get_key_value", map[string]any{"keyValueId": kvID}, "createdAt")
	call(t, "list_key_value", nil, "createdAt")

	// 3. Postgres: emptyText branch, create, get, non-empty list, query_render_postgres.
	emptyPG := call(t, "list_postgres_instances", nil)
	if got := rendertest.MCPText(t, emptyPG); got != "No Postgres instances found" {
		t.Fatalf("empty list_postgres_instances = %q, want Render's emptyText", got)
	}
	createdPG := call(t, "create_postgres", map[string]any{"name": "pg-conform", "plan": "free", "version": 16}, "createdAt")
	var pgObj map[string]any
	rendertest.DecodeMCPText(t, createdPG, &pgObj)
	pgID, _ := pgObj["id"].(string)
	call(t, "get_postgres", map[string]any{"postgresId": pgID}, "createdAt")
	call(t, "list_postgres_instances", nil, "createdAt")

	// Make the created Database CR Ready with a connection Secret so query_render_postgres runs.
	var dbList appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &dbList); err != nil || len(dbList.Items) != 1 {
		t.Fatalf("list databases: err=%v count=%d", err, len(dbList.Items))
	}
	pgCR := dbList.Items[0]
	pgCR.Status.Phase = appv1alpha1.DBPhaseReady
	pgCR.Status.SecretName = pgCR.Name + "-app"
	if err := cl.Update(context.Background(), &pgCR); err != nil {
		t.Fatalf("mark postgres ready: %v", err)
	}
	pgSecret := conformDatastoreSecret(pgCR.Status.SecretName, map[string]string{
		"uri": "postgresql://u:p@pg-conform-rw.default:5432/app",
	})
	pgSecret.Namespace = pgCR.Namespace
	if err := cl.Create(context.Background(), pgSecret); err != nil {
		t.Fatalf("create postgres secret: %v", err)
	}
	queryRes := call(t, "query_render_postgres", map[string]any{"postgresId": pgID, "sql": "SELECT 1 AS id, 'ok' AS label"})
	if got := rendertest.MCPText(t, queryRes); got != `[{"id":1,"label":"ok"}]` {
		t.Fatalf("query_render_postgres = %q, want row object array", got)
	}

	// 4. Services: create web/static/cron, list, get.
	call(t, "create_web_service", map[string]any{
		"name": "mcp-web-new", "runtime": "image", "image": "nginx:1.27", "plan": "free",
	})
	call(t, "create_static_site", map[string]any{
		"name": "mcp-static-new", "repo": "https://github.com/bex-co/site", "publishPath": "dist",
	})
	call(t, "create_cron_job", map[string]any{
		"name": "mcp-cron-new", "runtime": "image", "image": "busybox:1.36", "schedule": "0 * * * *",
	})
	call(t, "list_services", nil)
	call(t, "get_service", map[string]any{"serviceId": webName})

	// 5. Cron job runs: cancel_cron_job_run, then run_cron_job.
	call(t, "cancel_cron_job_run", map[string]any{"serviceId": "nightly"})
	call(t, "run_cron_job", map[string]any{"serviceId": "nightly", "cancelActiveRun": true})

	// 6. Deploys: list_deploys, get_deploy, trigger_deploy (live & queued 202).
	listDeploysRes := call(t, "list_deploys", map[string]any{"serviceId": webName})
	if !strings.Contains(rendertest.MCPText(t, listDeploysRes), rendertest.MCPCursorMarker) {
		t.Fatalf("list_deploys lacked cursor marker: %s", rendertest.MCPText(t, listDeploysRes))
	}
	call(t, "get_deploy", map[string]any{"serviceId": webName, "deployId": depID})
	call(t, "trigger_deploy", map[string]any{"serviceId": webName})
	depStore.queueNext = true
	queuedRes := call(t, "trigger_deploy", map[string]any{"serviceId": webName})
	if !strings.HasPrefix(rendertest.MCPText(t, queuedRes), "The deploy request was accepted") {
		t.Fatalf("queued trigger_deploy text = %q, want queued sentence", rendertest.MCPText(t, queuedRes))
	}

	// 7. Events, logs, metrics, env vars.
	listEventsRes := call(t, "list_events", map[string]any{"serviceId": webName})
	if !strings.Contains(rendertest.MCPText(t, listEventsRes), rendertest.MCPCursorMarker) {
		t.Fatalf("list_events lacked cursor marker: %s", rendertest.MCPText(t, listEventsRes))
	}
	call(t, "list_logs", map[string]any{
		"resource":  []string{webName},
		"startTime": conformEpoch.Add(-time.Hour).Format(time.RFC3339),
		"endTime":   conformEpoch.Add(time.Minute).Format(time.RFC3339),
	})
	call(t, "list_log_label_values", map[string]any{
		"resource":  []string{webName},
		"label":     "level",
		"startTime": conformEpoch.Add(-time.Hour).Format(time.RFC3339),
		"endTime":   conformEpoch.Add(time.Minute).Format(time.RFC3339),
	})
	call(t, "get_metrics", map[string]any{
		"resourceId":  webName,
		"metricTypes": []string{"cpu_usage"},
	})
	call(t, "update_environment_variables", map[string]any{
		"serviceId": webName,
		"envVars":   []map[string]any{{"key": "LOG_LEVEL", "value": "info"}},
	})

	// 8. Blueprints: create, get, approve, plan, update (no-op + changed).
	createdBP := call(t, "create_blueprint", map[string]any{
		"repoUrl": "https://github.com/bex-co/app.git",
		"branch":  "main",
		"path":    "render.yaml",
	})
	var bpCreate map[string]any
	rendertest.DecodeMCPText(t, createdBP, &bpCreate)
	bpID := bpCreate["blueprint"].(map[string]any)["id"].(string)

	call(t, "get_blueprint", map[string]any{"blueprintId": bpID})
	call(t, "approve_blueprint", map[string]any{"blueprintId": bpID})
	call(t, "plan_blueprint", map[string]any{"blueprintId": bpID})

	noOpBP := call(t, "update_blueprint", map[string]any{"blueprintId": bpID, "name": "app"})
	if len(noOpBP.Content) != 2 {
		t.Fatalf("no-op update_blueprint content blocks = %d, want 2 (JSON + no-op notice)", len(noOpBP.Content))
	}
	changedBP := call(t, "update_blueprint", map[string]any{"blueprintId": bpID, "name": "app-renamed"})
	if len(changedBP.Content) != 1 {
		t.Fatalf("changed update_blueprint content blocks = %d, want 1", len(changedBP.Content))
	}
	var updatedBP map[string]any
	rawUpdated, _ := json.Marshal(changedBP.StructuredContent)
	_ = json.Unmarshal(rawUpdated, &updatedBP)
	if updatedBP["blueprint"].(map[string]any)["name"] != "app-renamed" {
		t.Fatalf("updated blueprint = %v, want name app-renamed", updatedBP)
	}

	// Suite-wide coverage guard: every tool in render-mcp-results.json must be
	// exercised above and every form:"object" tool must carry a non-empty type.
	pin, err := rendertest.LoadMCPResults()
	if err != nil {
		t.Fatal(err)
	}
	var untested, untyped []string
	for _, name := range slices.Sorted(maps.Keys(pin.Tools)) {
		if !tested[name] {
			untested = append(untested, name)
		}
		if spec := pin.Tools[name]; spec.Form == "object" && spec.Type == "" {
			untyped = append(untyped, name)
		}
	}
	if len(untested) > 0 {
		t.Errorf("shared Render MCP tools untested in TestMCPResultConformance (%d/%d): %v", len(untested), len(pin.Tools), untested)
	}
	if len(untyped) > 0 {
		t.Errorf("shared Render MCP form:\"object\" tools without a recorded type (%d): %v", len(untyped), untyped)
	}
	if len(tested) != len(pin.Tools) {
		t.Errorf("tested %d tools, pin records %d", len(tested), len(pin.Tools))
	}
}
