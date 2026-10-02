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

package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPreDeployInstanceFiltersAcrossSurfaces(t *testing.T) {
	resource := ids.New(ids.Service)
	app := logTenantNSApp("web", logTestWS, resource)
	a, b := "migration-a", "migration-b"
	podA, podB := preDeployPodIn(app.Name, a, app.Namespace), preDeployPodIn(app.Name, b, app.Namespace)
	podA.UID, podB.UID = "migration-uid-a", "migration-uid-b"
	foreignNamespace := preDeployPodIn(app.Name, a, "default")
	foreignNamespace.UID = "foreign-migration-uid"
	appPod := podFor(app.Name, "app-pod")
	appPod.Namespace = app.Namespace
	svc := newService(map[string][]string{
		a:                   {"2026-07-05T00:00:00Z keep early", "2026-07-05T00:00:01Z keep A", "2026-07-05T00:00:02Z unrelated", "2026-07-05T00:00:04Z keep late"},
		b:                   {"2026-07-05T00:00:02Z keep B"},
		"app-pod":           {"2026-07-05T00:00:01Z keep app"},
		"foreign-migration": {"2026-07-05T00:00:01Z keep foreign"},
	}, app, appPod, podA, podB, foreignNamespace, preDeployPodIn("other", "foreign-migration", app.Namespace))
	podsByMessage := map[string]string{"keep A": a, "keep B": b}
	var reads []string
	source := svc.PodLogs
	svc.PodLogs = func(ctx context.Context, ns, pod, container string, tail int64) (io.ReadCloser, error) {
		reads = append(reads, ns+"/"+pod)
		if ns != app.Namespace || container != core.PreDeployContainer {
			t.Errorf("read %s/%s container=%s, want tenant predeploy source", ns, pod, container)
		}
		return source(ctx, ns, pod, container, tail)
	}
	svc.LabelValues = func(context.Context, string, string, LogQuery) ([]string, error) {
		t.Error("predeploy must not discover historical App instances")
		return nil, nil
	}
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	for _, tc := range []struct {
		name                  string
		selectors, want, pods []string
	}{
		{"omitted", nil, []string{"keep A", "keep B"}, []string{a, b}},
		{"raw-a", []string{a}, []string{"keep A"}, []string{a}},
		{"raw-b", []string{b}, []string{"keep B"}, []string{b}},
		{"public-a", []string{ids.ServiceInstanceID(resource, a)}, []string{"keep A"}, []string{a}},
		{"public-b", []string{ids.ServiceInstanceID(resource, b)}, []string{"keep B"}, []string{b}},
		{"legacy-a", []string{ids.LegacyServiceInstanceID(resource, string(podA.UID))}, []string{"keep A"}, []string{a}},
		{"legacy-b", []string{ids.LegacyServiceInstanceID(resource, string(podB.UID))}, []string{"keep B"}, []string{b}},
		{"unknown", []string{"does-not-exist"}, nil, nil},
		{"app-pod", []string{"app-pod"}, nil, nil},
		{"foreign-pod", []string{"foreign-migration"}, nil, nil},
		{"foreign-resource", []string{ids.ServiceInstanceID(ids.New(ids.Service), a)}, nil, nil},
		{"foreign-namespace-uid", []string{ids.LegacyServiceInstanceID(resource, string(foreignNamespace.UID))}, nil, nil},
		{"mixed", []string{"does-not-exist", a, a}, []string{"keep A"}, []string{a}},
		{"mixed-two", []string{ids.ServiceInstanceID(resource, b), "does-not-exist", a, ids.LegacyServiceInstanceID(resource, string(podA.UID))}, []string{"keep A", "keep B"}, []string{a, b}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, surface := range []string{"REST", "GraphQL", "MCP"} {
				t.Run(surface, func(t *testing.T) {
					reads = nil
					var entries []LogEntry
					resourceLabel := "service"
					switch surface {
					case "REST":
						params := url.Values{"resource": {resource}, "type": {"predeploy"}, "text": {"keep"}, "startTime": {"2026-07-05T00:00:01Z"}, "endTime": {"2026-07-05T00:00:03Z"}, "instance": tc.selectors}
						rec := serveREST(svc, http.MethodGet, "/v1/logs?"+params.Encode())
						page := decodeLogList(t, rec)
						for _, row := range page.Logs {
							entry := LogEntry{Message: row.Message, Labels: map[string]string{}}
							for _, label := range row.Labels {
								entry.Labels[label.Name] = label.Value
							}
							entries = append(entries, entry)
						}
						resourceLabel = "resource"
					case "GraphQL":
						selectors := tc.selectors
						if selectors == nil {
							selectors = []string{}
						}
						raw, _ := json.Marshal(selectors)
						data := runQuery(t, schema, fmt.Sprintf(`{ logs(resource:%q,type:"predeploy",text:"keep",instance:%s,startTime:"2026-07-05T00:00:01Z",endTime:"2026-07-05T00:00:03Z"){logs{message instance type}}}`, resource, raw))
						for _, raw := range data["logs"].(map[string]any)["logs"].([]any) {
							row := raw.(map[string]any)
							entries = append(entries, LogEntry{Message: row["message"].(string), Labels: map[string]string{
								LabelInstance: row["instance"].(string), LabelType: row["type"].(string),
							}})
						}
						resourceLabel = ""
					case "MCP":
						result, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{"resource": []string{resource}, "type": []string{"predeploy"}, "text": []string{"keep"}, "instance": tc.selectors, "startTime": "2026-07-05T00:00:01Z", "endTime": "2026-07-05T00:00:03Z"}})
						if e != nil {
							t.Fatal(e)
						}
						if result.IsError {
							t.Fatalf("MCP: %+v", result)
						}
						var page listLogsResult
						if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &page); err != nil {
							t.Fatal(err)
						}
						entries = page.Logs
					}
					var messages []string
					for _, entry := range entries {
						messages = append(messages, entry.Message)
						wantInstance := ids.ServiceInstanceID(resource, podsByMessage[entry.Message])
						if entry.Labels[LabelInstance] != wantInstance || entry.Labels[LabelType] != LogTypePreDeploy {
							t.Fatalf("entry %q labels=%v, want canonical instance=%s and predeploy type", entry.Message, entry.Labels, wantInstance)
						}
						if resourceLabel != "" && entry.Labels[resourceLabel] != resource {
							t.Fatalf("entry %q resource=%q, want canonical %q", entry.Message, entry.Labels[resourceLabel], resource)
						}
					}
					if !slices.Equal(messages, tc.want) {
						t.Fatalf("messages=%q, want %q", messages, tc.want)
					}
					slices.Sort(reads)
					want := slices.Clone(tc.pods)
					for i := range want {
						want[i] = app.Namespace + "/" + want[i]
					}
					slices.Sort(want)
					if !slices.Equal(reads, want) {
						t.Fatalf("pod reads=%q, want %q", reads, want)
					}
				})
			}
		})
	}
}
