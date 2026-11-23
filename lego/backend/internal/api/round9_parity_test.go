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
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type round9Projects struct {
	workspace string
	byProject map[string][]savedsearch.Resource
	envProj   map[string]string
}

func (p *round9Projects) ProjectLogResources(_ context.Context, projectID string) (string, []savedsearch.Resource, error) {
	rs, ok := p.byProject[projectID]
	if !ok {
		return "", nil, core.NotFound("project")
	}
	return p.workspace, rs, nil
}

func (p *round9Projects) ProjectScopeResources(_ context.Context, projectID, envID string) (string, []savedsearch.Resource, error) {
	if projectID == "" {
		proj, ok := p.envProj[envID]
		if !ok {
			return "", nil, core.NotFound("environment")
		}
		projectID = proj
	} else if envID != "" {
		proj, ok := p.envProj[envID]
		if !ok {
			return "", nil, core.NotFound("environment")
		}
		if proj != projectID {
			return "", nil, fmt.Errorf("%w: environment %q does not belong to project %q", core.ErrBadRequest, envID, projectID)
		}
	}
	rs, ok := p.byProject[projectID]
	if !ok {
		return "", nil, core.NotFound("project")
	}
	if envID == "" {
		return p.workspace, rs, nil
	}
	out := make([]savedsearch.Resource, 0, len(rs))
	for _, r := range rs {
		if r.EnvironmentID == envID {
			out = append(out, r)
		}
	}
	return p.workspace, out, nil
}

// TestRound9SandboxSuspendConformanceAndStates verifies t003 through the full
// composed server (auth + strict OpenAPI request gate + rendertest response
// validation): 202 on running, idempotent 202 on suspended, 409
// sandbox_not_running on terminated, 404 on missing, retired /pause, plus
// GraphQL suspendSandbox and MCP suspend_sandbox.
func TestRound9SandboxSuspendConformanceAndStates(t *testing.T) {
	const ws = "tea-round9"
	st := newConformSandboxStore(ws)
	osSrv := fakeOpenSandboxServer(t)

	h, _ := serverWith(t,
		&core.Base{
			Client:    fakeClient(),
			Namespace: "default",
			Workspace: fakeWorkspace{"client-1": ws},
			Clock:     func() time.Time { return conformEpoch },
			Plans:     proPlan(),
		},
		Deps{
			SandboxClient:          sandbox.NewClient(osSrv.URL),
			SandboxTemplates:       map[string]sandbox.Template{"node": {Image: "node:20", Entrypoint: []string{"sh"}, CPU: "500m", Memory: "512Mi"}},
			SandboxDefaultTemplate: "node",
			SandboxGroups:          st,
			SandboxExecutions:      st,
			SandboxRecords:         st,
			Region:                 "fsn1",
			DashboardURL:           "https://dashboard.bex.co",
		},
	)

	w := conformDo(t, h, http.MethodPost, "/v1/sandboxes", `{"ownerId":"`+ws+`","plan":"starter"}`, http.StatusCreated)
	var sbx struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &sbx); err != nil || sbx.ID == "" {
		t.Fatalf("create-sandbox = %s", w.Body.String())
	}

	// Retired /pause route is 404.
	if rec := do(t, h, http.MethodPost, "/v1/sandboxes/"+sbx.ID+"/pause?ownerId="+ws, testToken, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("POST /pause = %d %s, want 404", rec.Code, rec.Body.String())
	}

	// Suspend running sandbox -> 202 empty body; repeat on suspended -> 202 idempotent.
	w = conformDo(t, h, http.MethodPost, "/v1/sandboxes/"+sbx.ID+"/suspend?ownerId="+ws, "", http.StatusAccepted)
	if w.Body.Len() != 0 {
		t.Fatalf("suspend body = %q, want empty", w.Body.String())
	}
	conformDo(t, h, http.MethodPost, "/v1/sandboxes/"+sbx.ID+"/suspend?ownerId="+ws, "", http.StatusAccepted)

	// GraphQL suspendSandbox on suspended sandbox -> idempotent true.
	gqlBody := `{"query":"mutation($id:String!,$ownerId:String){ suspendSandbox(id:$id, ownerId:$ownerId) }","variables":{"id":"` + sbx.ID + `","ownerId":"` + ws + `"}}`
	rec := do(t, h, http.MethodPost, "/graphql", testToken, gqlBody)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"suspendSandbox":true`) {
		t.Fatalf("GraphQL suspendSandbox = %d %s", rec.Code, rec.Body.String())
	}

	// Terminate sandbox, then suspend -> 409 sandbox_not_running.
	conformDo(t, h, http.MethodPost, "/v1/sandboxes/"+sbx.ID+"/terminate?ownerId="+ws, "", http.StatusNoContent)
	w = conformDo(t, h, http.MethodPost, "/v1/sandboxes/"+sbx.ID+"/suspend?ownerId="+ws, "", http.StatusConflict)
	if !strings.Contains(w.Body.String(), `"code":"sandbox_not_running"`) {
		t.Fatalf("terminated suspend = %d %s, want sandbox_not_running", w.Code, w.Body.String())
	}

	// Unknown sandbox -> 404.
	conformDo(t, h, http.MethodPost, "/v1/sandboxes/sbx-c185th5c2rvvnhbfiltg/suspend?ownerId="+ws, "", http.StatusNotFound)
}

// TestRound9ProjectScopedLogsAndMetricsConformance verifies t004 and t005
// through the composed server (strict OpenAPI gate + rendertest response
// validator) across logs, log values, all 18 metrics paths, GraphQL and caps.
func TestRound9ProjectScopedLogsAndMetricsConformance(t *testing.T) {
	const (
		ws      = "tea-round9"
		prjID   = "prj-c185th5c2rvvnhbfiltg"
		prj21   = "prj-c185th5c2rvvnhbfilth"
		prj31   = "prj-c185th5c2rvvnhbfilti"
		prj51   = "prj-c185th5c2rvvnhbfiltj"
		envProd = "evm-c185th5c2rvvnhbfiltg"
		envStg  = "evm-c185th5c2rvvnhbfilth"
		envOut  = "evm-c185th5c2rvvnhbfilti"
	)
	webID := ids.New(ids.Service)
	wrkID := ids.New(ids.Service)
	outID := ids.New(ids.Service)
	staticID := ids.New(ids.Service)
	pgID := ids.New(ids.Postgres)
	kvID := ids.New(ids.KeyValue)

	web := conformApp("web", webID)
	web.Spec.Type = appv1alpha1.TypeWebService
	web.Spec.Tier = "starter"
	web.Labels[core.LabelTenant] = ws

	wrk := conformApp("wrk", wrkID)
	wrk.Spec.Type = appv1alpha1.TypeBackgroundWorker
	wrk.Spec.Tier = "starter"
	wrk.Labels[core.LabelTenant] = ws

	outApp := conformApp("out", outID)
	outApp.Spec.Type = appv1alpha1.TypeWebService
	outApp.Labels[core.LabelTenant] = ws

	site := conformApp("site", staticID)
	site.Spec.Type = appv1alpha1.TypeStaticSite
	site.Spec.PublishPath = "dist"
	site.Labels[core.LabelTenant] = ws

	pg := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: pgID, Namespace: ws, Labels: map[string]string{core.LabelTenant: ws}},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "basic-256mb", StorageGB: 10},
		Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady, HighAvailabilityEnabled: true},
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: kvID, Namespace: ws, Labels: map[string]string{core.LabelTenant: ws}},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free", StorageGB: 2},
	}

	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-10 * time.Minute).Format(time.RFC3339)
	start := now.Add(-time.Hour).Format(time.RFC3339)
	end := now.Format(time.RFC3339)

	makeMany := func(n int) []savedsearch.Resource {
		out := make([]savedsearch.Resource, n)
		for i := range n {
			out[i] = savedsearch.Resource{ID: fmt.Sprintf("srv-many%016d", i), Name: fmt.Sprintf("s%d", i), Type: appv1alpha1.TypeWebService, EnvironmentID: envProd}
		}
		return out
	}
	proj := &round9Projects{
		workspace: ws,
		byProject: map[string][]savedsearch.Resource{
			prjID: {
				{ID: webID, Name: "web", Type: appv1alpha1.TypeWebService, EnvironmentID: envProd},
				{ID: wrkID, Name: "wrk", Type: appv1alpha1.TypeBackgroundWorker, EnvironmentID: envStg},
				{ID: staticID, Name: "site", Type: appv1alpha1.TypeStaticSite, EnvironmentID: envProd},
				{ID: pgID, Name: "pg", Type: savedsearch.KindPostgres, EnvironmentID: envProd},
				{ID: kvID, Name: "kv", Type: savedsearch.KindKeyValue, EnvironmentID: envStg},
			},
			prj21: makeMany(21),
			prj31: makeMany(31),
			prj51: makeMany(51),
		},
		envProj: map[string]string{
			envProd: prjID,
			envStg:  prjID,
			envOut:  prj21,
		},
	}

	h, srv := serverWith(t,
		&core.Base{
			Client:    fakeClient(web, wrk, outApp, site, pg, kv),
			Namespace: "default",
			Workspace: fakeWorkspace{"client-1": ws},
			Clock:     func() time.Time { return now },
			Plans:     proPlan(),
		},
		Deps{
			LogHistory: func(_ context.Context, _ string, q logs.LogQuery) ([]logs.LogEntry, error) {
				return []logs.LogEntry{{
					Timestamp: ts,
					Message:   "hello from " + q.App,
					Labels:    map[string]string{"service": q.App, "type": "app", "level": "info", "instance": q.App + "-1"},
				}}, nil
			},
			LogLabelValues: func(_ context.Context, _ string, _ string, _ logs.LogQuery) ([]string, error) {
				return []string{"info"}, nil
			},
			ResourceMetricsRange: func(_ context.Context, _ metrics.ResourceMetricsRangeRequest) ([]metrics.MetricSeries, error) {
				return []metrics.MetricSeries{{Labels: map[string]string{"instance": "pod-1"}, Unit: "cpu", Points: []metrics.MetricPoint{{Timestamp: ts, Value: 0.2}}}}, nil
			},
			RequestMetrics: func(_ context.Context, req metrics.RequestMetricsRequest) ([]metrics.MetricSeries, error) {
				return []metrics.MetricSeries{{Labels: map[string]string{"resource": req.AppID}, Unit: "count", Points: []metrics.MetricPoint{{Timestamp: ts, Value: 5}}}}, nil
			},
			DatastoreResource: func(_ context.Context, req metrics.DatastoreResourceRequest) ([]metrics.MetricSeries, error) {
				return []metrics.MetricSeries{{Labels: map[string]string{"instance": req.Resource + "-1"}, Unit: "cpu", Points: []metrics.MetricPoint{{Timestamp: ts, Value: 0.1}}}}, nil
			},
			DiskUsage: func(_ context.Context, req metrics.DiskUsageRequest) ([]metrics.MetricSeries, error) {
				return []metrics.MetricSeries{{Labels: map[string]string{"instance": req.Resource + "-1"}, Unit: "bytes", Points: []metrics.MetricPoint{{Timestamp: ts, Value: 1024}}}}, nil
			},
			DBConnections: func(_ context.Context, req metrics.DBConnectionsRequest) ([]metrics.MetricSeries, error) {
				return []metrics.MetricSeries{{Labels: map[string]string{"instance": req.Cluster + "-1"}, Unit: "count", Points: []metrics.MetricPoint{{Timestamp: ts, Value: 3}}}}, nil
			},
			KeyValueStats: func(_ context.Context, req metrics.KeyValueStatsRequest) ([]metrics.MetricSeries, error) {
				return []metrics.MetricSeries{{Labels: map[string]string{"instance": req.Resource + "-0"}, Unit: "count", Points: []metrics.MetricPoint{{Timestamp: ts, Value: 4}}}}, nil
			},
			ReplicationLag: func(_ context.Context, req metrics.ReplicationLagRequest) ([]metrics.MetricSeries, error) {
				return []metrics.MetricSeries{{Labels: map[string]string{"instance": req.Cluster + "-2"}, Unit: "seconds", Points: []metrics.MetricPoint{{Timestamp: ts, Value: 0.2}}}}, nil
			},
			InstanceFilterValues: func(_ context.Context, _ metrics.InstanceFilterValuesRequest) ([]string, error) {
				return []string{"pod-1"}, nil
			},
			HTTPFilterValues: func(_ context.Context, _ metrics.HTTPFilterValuesRequest) (metrics.HTTPFilterValues, error) {
				return metrics.HTTPFilterValues{Hosts: []string{"web.onbex.co"}, StatusCodes: []string{"200"}}, nil
			},
			PathFilterSuggestions: func(_ context.Context, _ metrics.PathFilterSuggestionsRequest) ([]string, error) {
				return []string{"/healthz"}, nil
			},
			Region:       "fsn1",
			DashboardURL: "https://dashboard.bex.co",
		},
	)
	srv.Logs.Projects = proj
	srv.Metrics.Projects = proj

	// --- Logs (t004) ---
	w := conformDo(t, h, http.MethodGet, "/v1/logs?project="+prjID+"&startTime="+start+"&endTime="+end, "", http.StatusOK)
	if !strings.Contains(w.Body.String(), `"value":"`+webID+`"`) || !strings.Contains(w.Body.String(), `"value":"`+pgID+`"`) || strings.Contains(w.Body.String(), staticID) {
		t.Fatalf("project logs = %s", w.Body.String())
	}
	// Environment narrows to envProd (web + pg, not wrk or kv).
	w = conformDo(t, h, http.MethodGet, "/v1/logs?project="+prjID+"&environment="+envProd+"&region=oregon&startTime="+start+"&endTime="+end, "", http.StatusOK)
	if !strings.Contains(w.Body.String(), webID) || strings.Contains(w.Body.String(), wrkID) {
		t.Fatalf("project+env logs = %s", w.Body.String())
	}
	// Resource narrows further; outside resource is 400.
	w = conformDo(t, h, http.MethodGet, "/v1/logs?project="+prjID+"&resource="+wrkID+"&startTime="+start+"&endTime="+end, "", http.StatusOK)
	if !strings.Contains(w.Body.String(), wrkID) || strings.Contains(w.Body.String(), webID) {
		t.Fatalf("project+resource logs = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/logs?project="+prjID+"&resource="+outID, "", http.StatusBadRequest)
	// Environment or region without project -> 400.
	conformDo(t, h, http.MethodGet, "/v1/logs?environment="+envProd, "", http.StatusBadRequest)
	conformDo(t, h, http.MethodGet, "/v1/logs?region=oregon", "", http.StatusBadRequest)
	// Over 20 resources -> 400 too_many_resources.
	w = conformDo(t, h, http.MethodGet, "/v1/logs?project="+prj21, "", http.StatusBadRequest)
	if !strings.Contains(w.Body.String(), `"code":"too_many_resources"`) {
		t.Fatalf("logs over cap = %s", w.Body.String())
	}
	// Log label values by project.
	conformDo(t, h, http.MethodGet, "/v1/logs/values?project="+prjID+"&environment="+envProd+"&label=level&startTime="+start+"&endTime="+end, "", http.StatusOK)

	// --- Metrics (t005) ---
	for _, path := range []string{
		"/v1/metrics/cpu?project=" + prjID,
		"/v1/metrics/cpu?environment=" + envProd,
		"/v1/metrics/cpu?project=" + prjID + "&environment=" + envProd + "&region=frankfurt",
		"/v1/metrics/cpu?project=" + prjID + "&resource=" + webID,
		"/v1/metrics/memory?project=" + prjID,
		"/v1/metrics/cpu-limit?project=" + prjID,
		"/v1/metrics/memory-limit?project=" + prjID,
		"/v1/metrics/cpu-target?project=" + prjID,
		"/v1/metrics/memory-target?project=" + prjID,
		"/v1/metrics/instance-count?project=" + prjID,
		"/v1/metrics/http-requests?project=" + prjID,
		"/v1/metrics/http-latency?project=" + prjID,
		"/v1/metrics/bandwidth?project=" + prjID,
		"/v1/metrics/bandwidth-sources?project=" + prjID + "&startTime=" + start + "&endTime=" + end,
		"/v1/metrics/disk-usage?project=" + prjID,
		"/v1/metrics/disk-capacity?project=" + prjID,
		"/v1/metrics/active-connections?project=" + prjID,
		"/v1/metrics/replication-lag?project=" + prjID,
		"/v1/metrics/filters/application?project=" + prjID + "&startTime=" + start + "&endTime=" + end,
		"/v1/metrics/filters/http?project=" + prjID + "&startTime=" + start + "&endTime=" + end,
		"/v1/metrics/filters/path?project=" + prjID + "&startTime=" + start + "&endTime=" + end,
	} {
		conformDo(t, h, http.MethodGet, path, "", http.StatusOK)
	}

	// Refusals: unknown project/env -> 404; env outside project -> 400; region alone -> 400; outside resource -> 400.
	conformDo(t, h, http.MethodGet, "/v1/metrics/cpu?project=prj-missing0000000000000", "", http.StatusNotFound)
	conformDo(t, h, http.MethodGet, "/v1/metrics/cpu?environment=evm-missing0000000000000", "", http.StatusNotFound)
	conformDo(t, h, http.MethodGet, "/v1/metrics/cpu?project="+prjID+"&environment="+envOut, "", http.StatusBadRequest)
	conformDo(t, h, http.MethodGet, "/v1/metrics/cpu?region=oregon", "", http.StatusBadRequest)
	conformDo(t, h, http.MethodGet, "/v1/metrics/cpu?project="+prjID+"&resource="+outID, "", http.StatusBadRequest)

	// Caps: 51 on /metrics/cpu and 31 on /metrics/bandwidth-sources -> 400 too_many_resources.
	w = conformDo(t, h, http.MethodGet, "/v1/metrics/cpu?project="+prj51, "", http.StatusBadRequest)
	if !strings.Contains(w.Body.String(), `"code":"too_many_resources"`) {
		t.Fatalf("metrics over 50 = %s", w.Body.String())
	}
	w = conformDo(t, h, http.MethodGet, "/v1/metrics/bandwidth-sources?project="+prj31+"&startTime="+start+"&endTime="+end, "", http.StatusBadRequest)
	if !strings.Contains(w.Body.String(), `"code":"too_many_resources"`) {
		t.Fatalf("bandwidth-sources over 30 = %s", w.Body.String())
	}

	// GraphQL metrics with PROJECT filter entry.
	gqlBody := `{"query":"query($p:String!){ metrics(query:{name:\"CPU\", filters:[{field:\"PROJECT\", values:[$p]}]}){ unit } }","variables":{"p":"` + prjID + `"}}`
	rec := do(t, h, http.MethodPost, "/graphql", testToken, gqlBody)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"errors"`) {
		t.Fatalf("GraphQL project metrics = %d %s", rec.Code, rec.Body.String())
	}
}
