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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestBuildSourceLogsEndToEndThroughComposedServer verifies w2/m611 end to end
// through NewServer, server.go's DeployProgress + BuiltDeployIDs adapter,
// Render's OpenAPI request validator, Loki query/label-values builders,
// GraphQL, MCP, and scoped API key permissions (KeyMonitoringLogs).
func TestBuildSourceLogsEndToEndThroughComposedServer(t *testing.T) {
	at := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	ws := "tea-a"
	bdsID := ids.New(ids.BuildSource)
	dep1 := ids.New(ids.Deploy)
	dep2 := ids.New(ids.Deploy)
	bld2, _ := ids.Rekind(ids.Build, dep2)

	carrier := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws + "-" + bdsID,
			Namespace: "default",
			Labels: map[string]string{
				core.LabelTenant:      ws,
				store.LabelAppID:      bdsID,
				core.LabelServiceName: bdsID,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeBuildSource,
			DisplayName: "monorepo-src",
			Repo:        "https://github.com/bex-co/monorepo",
			Branch:      "main",
		},
	}

	t1Start, t1End := at.Add(-4*time.Minute), at.Add(-3*time.Minute)
	t2Start, t2End := at.Add(-2*time.Minute), at.Add(-time.Minute)
	deployStore := &builtConformDeployStore{
		conformDeployStore: conformDeployStore{
			byApp: map[string][]store.Deploy{
				bdsID: {
					{
						ID: dep2, AppID: bdsID, Generation: 2, Status: store.DeployLive,
						Repo: carrier.Spec.Repo, Branch: "main", Commit: "2222222bbbb",
						CreatedAt: t2Start.Add(-5 * time.Second), StartedAt: &t2Start, FinishedAt: &t2End,
						// Runtime-shaped fields must never leak into Build Source narration.
						DetectedHTTPPort: 8080,
						LiveURLs:         []string{"https://must-not-appear.onbex.co"},
					},
					{
						ID: dep1, AppID: bdsID, Generation: 1, Status: store.DeployBuildFailed,
						Repo: carrier.Spec.Repo, Branch: "main", Commit: "1111111aaaa",
						FailureReason: "build timed out",
						CreatedAt:     t1Start.Add(-5 * time.Second), StartedAt: &t1Start, FinishedAt: &t1End,
					},
				},
			},
		},
		built: map[string]bool{dep1: true, dep2: true},
	}

	job1 := appv1alpha1.BuildJobName(carrier.Name, appv1alpha1.BuildRevision(1))
	job2 := appv1alpha1.BuildJobName(carrier.Name, appv1alpha1.BuildRevision(2))
	pod2 := job2 + "-x7k2p"

	var lastRangeQuery, lastValuesQuery string
	lokiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/label/") {
			lastValuesQuery = r.URL.Query().Get("query")
			if strings.Contains(r.URL.Path, "/label/pod/values") {
				_, _ = fmt.Fprintf(w, `{"status":"success","data":[%q]}`, pod2)
				return
			}
			_, _ = w.Write([]byte(`{"status":"success","data":["build"]}`))
			return
		}
		lastRangeQuery = r.URL.Query().Get("query")
		ts := fmt.Sprintf("%d", t2Start.Add(20*time.Second).UnixNano())
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"streams","result":[{"stream":{"type":"build","pod":%q,"app":%q},"values":[[%q,"building monorepo artifact"]]}]}}`, pod2, carrier.Name, ts)
	}))
	t.Cleanup(lokiSrv.Close)

	allowKey := keyOf("key-with-logs")
	denyKey := keyOf("key-without-logs")
	grants := &grantStore{grants: map[string]core.KeyGrant{
		allowKey.Subject: grantOf("monitoring:read"),
		denyKey.Subject:  grantOf("services:read"),
	}}
	base := &core.Base{
		Client:    fakeClient(eventsApp(), carrier),
		Namespace: "default",
		Authz:     &fakeChecker{allow: true},
		KeyGrants: grants,
	}
	h, srv := serverWith(t, base, Deps{
		DeployStore:    deployStore,
		LogHistory:     logs.NewLokiSource(lokiSrv.URL, lokiSrv.Client()),
		LogLabelValues: logs.NewLokiLabelValuesSource(lokiSrv.URL, lokiSrv.Client()),
	})

	startParam := at.Add(-10 * time.Minute).Format(time.RFC3339)
	endParam := at.Format(time.RFC3339)

	// 1. REST GET /v1/logs?resource=bds-…&build=bld2 passes OpenAPI validation,
	//    scopes Loki to job2 only, and emits build=bld2 with build-only narration.
	q := url.Values{
		"ownerId":   {ws},
		"resource":  {bdsID},
		"build":     {bld2},
		"startTime": {startParam},
		"endTime":   {endParam},
		"limit":     {"100"},
	}
	res := do(t, h, http.MethodGet, "/v1/logs?"+q.Encode(), testToken, "")
	if res.Code != http.StatusOK {
		t.Fatalf("GET /v1/logs?resource=%s&build=%s: %d %s", bdsID, bld2, res.Code, res.Body.String())
	}
	if !strings.Contains(lastRangeQuery, job2) || strings.Contains(lastRangeQuery, job1) {
		t.Errorf("Loki range query = %q, want scoped to %s and not %s", lastRangeQuery, job2, job1)
	}
	var logBody struct {
		Logs []struct {
			Message string `json:"message"`
			Labels  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"labels"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &logBody); err != nil {
		t.Fatal(err)
	}
	var messages []string
	for _, l := range logBody.Logs {
		messages = append(messages, l.Message)
		var gotRes, gotBuild, gotType string
		for _, lb := range l.Labels {
			switch lb.Name {
			case "resource":
				gotRes = lb.Value
			case logs.LabelBuild:
				gotBuild = lb.Value
			case logs.LabelType:
				gotType = lb.Value
			}
		}
		if gotRes != bdsID || gotBuild != bld2 || gotType != logs.LogTypeBuild {
			t.Errorf("REST line %q labels = %+v, want resource=%s build=%s type=build", l.Message, l.Labels, bdsID, bld2)
		}
		if strings.Contains(l.Message, "Your service is live") || strings.Contains(l.Message, "Detected service") || strings.Contains(l.Message, "must-not-appear") {
			t.Errorf("Build Source narration leaked runtime line: %q", l.Message)
		}
	}
	for _, want := range []string{"==> Build queued", "==> Building from https://github.com/bex-co/monorepo@2222222", "building monorepo artifact", "==> Build succeeded 🎉"} {
		if !slices.Contains(messages, want) {
			t.Errorf("REST logs missing %q: %v", want, messages)
		}
	}

	// 2. REST GET /v1/logs/values?resource=bds-…&label=instance&build=bld2 passes OpenAPI validation.
	valQ := url.Values{
		"ownerId":   {ws},
		"resource":  {bdsID},
		"label":     {logs.LabelInstance},
		"build":     {bld2},
		"startTime": {startParam},
		"endTime":   {endParam},
	}
	valRes := do(t, h, http.MethodGet, "/v1/logs/values?"+valQ.Encode(), testToken, "")
	if valRes.Code != http.StatusOK {
		t.Fatalf("GET /v1/logs/values: %d %s", valRes.Code, valRes.Body.String())
	}
	if !strings.Contains(lastValuesQuery, job2) {
		t.Errorf("Loki label-values query = %q, want scoped to %s", lastValuesQuery, job2)
	}
	var vals []string
	if err := json.Unmarshal(valRes.Body.Bytes(), &vals); err != nil {
		t.Fatal(err)
	}
	if len(vals) != 2 {
		t.Errorf("REST /v1/logs/values?label=instance = %v, want 2 (pod2 + narration)", vals)
	}

	// 3. GraphQL logs + logLabelValues agree with REST.
	gqlBody, _ := json.Marshal(map[string]string{
		"query": fmt.Sprintf(`{
			logs(resource: %q, build: [%q], startTime: %q, endTime: %q) { logs { resource type build message } }
			logLabelValues(resource: %q, label: "instance", build: [%q], startTime: %q, endTime: %q)
		}`, bdsID, bld2, startParam, endParam, bdsID, bld2, startParam, endParam),
	})
	gqlRes := do(t, h, http.MethodPost, "/graphql", testToken, string(gqlBody))
	if gqlRes.Code != http.StatusOK {
		t.Fatalf("GraphQL = %d %s", gqlRes.Code, gqlRes.Body.String())
	}
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
		Errors []any `json:"errors"`
	}
	if err := json.Unmarshal(gqlRes.Body.Bytes(), &gqlResp); err != nil {
		t.Fatal(err)
	}
	if len(gqlResp.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", gqlResp.Errors)
	}
	if len(gqlResp.Data.Logs.Logs) != len(logBody.Logs) {
		t.Errorf("GraphQL logs = %d, REST = %d", len(gqlResp.Data.Logs.Logs), len(logBody.Logs))
	}
	if !slices.Equal(gqlResp.Data.LogLabelValues, vals) {
		t.Errorf("GraphQL logLabelValues = %v, REST = %v", gqlResp.Data.LogLabelValues, vals)
	}

	// 4. MCP list_logs + list_log_label_values agree with REST.
	session := mcpSessionAs(t, srv, "alice")
	mcpLogs, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "list_logs",
		Arguments: map[string]any{
			"resource":  []string{bdsID},
			"build":     []string{bld2},
			"startTime": startParam,
			"endTime":   endParam,
		},
	})
	if err != nil || mcpLogs.IsError {
		t.Fatalf("MCP list_logs = %v %+v", err, mcpLogs)
	}
	if !strings.Contains(mcpLogs.Content[0].(*mcp.TextContent).Text, bld2) {
		t.Errorf("MCP list_logs text lacks %s: %s", bld2, mcpLogs.Content[0].(*mcp.TextContent).Text)
	}
	mcpVals, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "list_log_label_values",
		Arguments: map[string]any{
			"resource":  []string{bdsID},
			"label":     logs.LabelInstance,
			"build":     []string{bld2},
			"startTime": startParam,
			"endTime":   endParam,
		},
	})
	if err != nil || mcpVals.IsError {
		t.Fatalf("MCP list_log_label_values = %v %+v", err, mcpVals)
	}
	var gotMCPVals []string
	if err := json.Unmarshal([]byte(mcpVals.Content[0].(*mcp.TextContent).Text), &gotMCPVals); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotMCPVals, vals) {
		t.Errorf("MCP list_log_label_values = %v, REST = %v", gotMCPVals, vals)
	}

	// 5. Scoped API key enforcement: monitoring:read (KeyMonitoringLogs) allows bds-… reads on REST/GraphQL/MCP; services:read alone gets 403.
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema
	if status, body := restAs(t, srv, allowKey, http.MethodGet, "/v1/logs?"+q.Encode(), ""); status != http.StatusOK {
		t.Errorf("scoped key with monitoring:read REST = %d %v, want 200", status, body)
	}
	if status, body := restAs(t, srv, denyKey, http.MethodGet, "/v1/logs?"+q.Encode(), ""); !grantRefused(status, body) {
		t.Errorf("scoped key without monitoring:read REST = %d %v, want 403 %s", status, body, core.KeyPermissionDeniedCode)
	}
	gqlDoc := fmt.Sprintf(`{ logs(resource: %q, build: [%q], startTime: %q, endTime: %q) { logs { build message } } }`, bdsID, bld2, startParam, endParam)
	if out := graphqlDo(t, srv, allowKey, gqlDoc); graphqlCode(out) == core.KeyPermissionDeniedCode {
		t.Errorf("scoped key with monitoring:read GraphQL refused: %v", out)
	}
	if out := graphqlDo(t, srv, denyKey, gqlDoc); graphqlCode(out) != core.KeyPermissionDeniedCode {
		t.Errorf("scoped key without monitoring:read GraphQL = %v, want %s", out, core.KeyPermissionDeniedCode)
	}
	mcpAllow := mcpSessionIdentity(t, srv, allowKey)
	if res, err := mcpAllow.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{"resource": []string{bdsID}, "build": []string{bld2}, "startTime": startParam, "endTime": endParam}}); err != nil || res.IsError {
		t.Errorf("scoped key with monitoring:read MCP list_logs = %v %+v", err, res)
	}
	mcpDeny := mcpSessionIdentity(t, srv, denyKey)
	if res, err := mcpDeny.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{"resource": []string{bdsID}, "build": []string{bld2}, "startTime": startParam, "endTime": endParam}}); err == nil && !res.IsError {
		t.Errorf("scoped key without monitoring:read MCP list_logs succeeded, want error")
	}
}
