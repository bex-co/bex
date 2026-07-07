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
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func testScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return scheme
}

// podFor makes a pod labeled as one of app's instances (as the controller does).
func podFor(app, name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      name,
		Namespace: "default",
		Labels:    map[string]string{podLabelApp: app},
	}}
}

// staticLogs is a PodLogSource serving canned, timestamped lines per pod.
func staticLogs(lines map[string][]string) PodLogSource {
	return func(_ context.Context, _, pod, _ string, _ int64) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(strings.Join(lines[pod], "\n"))), nil
	}
}

// --- Core.Logs verb ---

func TestCore_LogsAggregatesAndSorts(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).
		WithObjects(sampleApp("web"), podFor("web", webInst), podFor("web", "web-2")).Build()
	core := &Core{Client: cl, Namespace: "default", PodLogs: staticLogs(map[string][]string{
		webInst: {"2026-07-05T00:00:01Z hello from 1", "2026-07-05T00:00:03Z later from 1"},
		"web-2": {"2026-07-05T00:00:02Z hello from 2"},
	})}

	entries, err := core.Logs(context.Background(), "web", 0)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 lines across 2 instances, got %d", len(entries))
	}
	// Sorted by timestamp so instances interleave chronologically.
	wantMsg := []string{"hello from 1", "hello from 2", "later from 1"}
	for i, w := range wantMsg {
		if entries[i].Message != w {
			t.Errorf("entry %d message = %q, want %q", i, entries[i].Message, w)
		}
	}
	// Render log-label shape: each entry is tagged with its instance + service.
	if entries[0].Labels["instance"] != webInst || entries[0].Labels["service"] != "web" {
		t.Errorf("missing render log labels: %+v", entries[0].Labels)
	}
}

func TestCore_LogsErrors(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(sampleApp("web")).Build()

	// No source wired => ErrLogsUnavailable (not a 404).
	nolog := &Core{Client: cl, Namespace: "default"}
	if _, err := nolog.Logs(context.Background(), "web", 0); !errors.Is(err, ErrLogsUnavailable) {
		t.Errorf("nil PodLogs should give ErrLogsUnavailable, got %v", err)
	}

	// Unknown app => ErrNotFound, exactly like Get.
	core := &Core{Client: cl, Namespace: "default", PodLogs: staticLogs(nil)}
	if _, err := core.Logs(context.Background(), "nope", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown app should give ErrNotFound, got %v", err)
	}
}

// --- MCP adapter (same Core, Render-consistent tool names) ---

func mcpClient(t *testing.T, core *Core) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	srv := (&Server{Core: core}).MCPServer()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if out != nil && !res.IsError {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("decode %s result: %v", name, err)
		}
	}
	return res
}

func TestMCP_ExposesRenderConsistentTools(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(sampleApp("web")).Build()
	cs := mcpClient(t, &Core{Client: cl, Namespace: "default"})

	got := map[string]bool{}
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tl := range tools.Tools {
		got[tl.Name] = true
	}
	// Names track Render's official MCP server (render-oss/render-mcp-server).
	for _, want := range []string{
		"list_services", "get_service", "list_logs", "get_metrics",
		"restart_service", "suspend_service", "resume_service",
	} {
		if !got[want] {
			t.Errorf("missing Render-consistent tool %q (have %v)", want, got)
		}
	}
}

func TestMCP_ToolsDelegateToCore(t *testing.T) {
	web := sampleApp("web")
	cl := fake.NewClientBuilder().WithScheme(testScheme()).
		WithObjects(web, podFor("web", webInst)).Build()
	core := &Core{
		Client: cl, Namespace: "default",
		Now:     func() time.Time { return time.Unix(1_000_000, 0).UTC() },
		PodLogs: staticLogs(map[string][]string{webInst: {"2026-07-05T00:00:01Z booting"}}),
	}
	cs := mcpClient(t, core)

	// list_services returns the Render service shape.
	var list listServicesResult
	callTool(t, cs, "list_services", nil, &list)
	if len(list.Services) != 1 || list.Services[0].ID != "web" {
		t.Fatalf("list_services: %+v", list.Services)
	}
	if list.Services[0].Type != renderWebService || list.Services[0].Suspended != renderNotSuspended {
		t.Errorf("list_services shape not Render-consistent: %+v", list.Services[0])
	}

	// suspend_service travels the SAME Core write path as REST/GraphQL. Render's
	// arg name is serviceId.
	var svc renderService
	callTool(t, cs, "suspend_service", map[string]any{"serviceId": "web"}, &svc)
	if svc.Suspended != renderSuspended {
		t.Errorf("suspend_service should report suspended, got %q", svc.Suspended)
	}
	if got := getApp(t, cl, "web"); !got.Spec.Suspended || got.Spec.Replicas != 2 {
		t.Errorf("suspend_service must suspend and keep replicas: %+v", got.Spec)
	}

	// list_logs takes Render's `resource` array and delegates to Core.Logs.
	var logs listLogsResult
	callTool(t, cs, "list_logs", map[string]any{"resource": []string{"web"}}, &logs)
	if len(logs.Logs) != 1 || logs.Logs[0].Message != "booting" {
		t.Fatalf("list_logs: %+v", logs.Logs)
	}
	if logs.Logs[0].Labels["instance"] != webInst {
		t.Errorf("list_logs missing instance label: %+v", logs.Logs[0].Labels)
	}

	// Unknown serviceId surfaces as a tool error, not a crash.
	if res := callTool(t, cs, "get_service", map[string]any{"serviceId": "nope"}, nil); !res.IsError {
		t.Error("get_service on unknown serviceId should be a tool error")
	}
}

// TestMCP_GetMetrics checks the metrics tool delegates to Core.Metrics and tags
// each series with its metric — instance_count needs no metrics source.
func TestMCP_GetMetrics(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).
		WithObjects(sampleApp("web"), podFor("web", webInst)).Build()
	core := &Core{Client: cl, Namespace: "default", Now: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}
	cs := mcpClient(t, core)

	var out getMetricsResult
	callTool(t, cs, "get_metrics", map[string]any{
		"resource": []string{"web"}, "metricTypes": []string{"instance_count"},
	}, &out)
	if len(out.Series) != 1 || out.Series[0].Unit != unitCount {
		t.Fatalf("get_metrics instance_count: %+v", out.Series)
	}
	if out.Series[0].Labels["metric"] != "instance_count" || out.Series[0].Points[0].Value != 1 {
		t.Errorf("series should be metric-tagged with one running instance: %+v", out.Series[0])
	}
}

// TestMCP_ListLogsResourceArrayAndLimit checks Render's list_logs shape: the
// `resource` array aggregates multiple services, re-sorted by timestamp, and
// `limit` caps the total to the newest N lines.
func TestMCP_ListLogsResourceArrayAndLimit(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"), podFor("web", webInst),
		sampleApp("api"), podFor("api", "api-1"),
	).Build()
	core := &Core{Client: cl, Namespace: "default", PodLogs: staticLogs(map[string][]string{
		webInst: {"2026-07-05T00:00:01Z w1", "2026-07-05T00:00:04Z w2"},
		"api-1": {"2026-07-05T00:00:02Z a1", "2026-07-05T00:00:03Z a2"},
	})}
	cs := mcpClient(t, core)

	// Two resources, limit 3 => newest 3 across both, chronologically ordered.
	var logs listLogsResult
	callTool(t, cs, "list_logs", map[string]any{"resource": []string{"web", "api"}, "limit": 3}, &logs)
	got := make([]string, len(logs.Logs))
	for i, e := range logs.Logs {
		got[i] = e.Message
	}
	want := []string{"a1", "a2", "w2"} // w1 (oldest) dropped by limit
	if len(got) != len(want) {
		t.Fatalf("limit=3 should cap to 3 lines, got %d: %v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("merged logs[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
