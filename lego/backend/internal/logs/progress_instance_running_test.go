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
	"maps"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// w32/m575: the instance's own `==> Running '<command>'` is an app line. REST
// /v1/logs (`bex logs -r <srv>`), GraphQL logs (the service's Logs page and
// the deploy page) and MCP list_logs read it at each start with the same ids
// and the instance's labels, and a build read of a release whose instance
// writes it carries no narrated copy.
func TestTheInstancesRunningLineReadsTheSameOnRESTGraphQLAndMCP(t *testing.T) {
	second := firstAppStart.Add(4 * time.Second)
	instanceLine := func(at time.Time) LogEntry {
		e := typedLine(at, LogTypeApp, rendersRunning)
		e.Labels[LabelLevel] = "info"
		return e
	}
	entries := []LogEntry{
		instanceLine(firstAppStart.Add(150 * time.Millisecond)),
		typedLine(firstAppStart.Add(900*time.Millisecond), LogTypeApp, "listening on 10000"),
		instanceLine(second.Add(150 * time.Millisecond)),
	}
	d := nativeRollout("live")
	d.AppStartedAt, d.InstanceAnnounces = []time.Time{firstAppStart, second}, true
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	svc := newService(nil, app, podFor("web", "web-1"))
	svc.PodLogs, svc.PodLogsFollow = nil, nil
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		var in []LogEntry
		for _, e := range entries {
			if q.keep(e) && (len(q.Types) == 0 || slices.Contains(q.Types, e.Labels[LabelType])) {
				e.Labels = maps.Clone(e.Labels) // each read projects its own copy
				in = append(in, e)
			}
		}
		return in, nil
	}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }
	start, end := rolloutCreated.Format(time.RFC3339), rolloutClose.Add(time.Minute).Format(time.RFC3339)

	// running is the ids of the read's Running lines, oldest first.
	running := func(surface string, messages, ids, instances []string) []string {
		t.Helper()
		var out []string
		for i, m := range messages {
			if m != rendersRunning {
				continue
			}
			if instances != nil && instances[i] == "" {
				t.Errorf("%s: Running line %s has no instance label", surface, ids[i])
			}
			out = append(out, ids[i])
		}
		slices.Sort(out)
		return out
	}

	rest := func(typ string) []string {
		t.Helper()
		rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&type="+typ+"&startTime="+start+"&endTime="+end)
		var env renderLogList
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("REST = %d %s (%v)", rec.Code, rec.Body, err)
		}
		var messages, ids, instances []string
		for _, l := range env.Logs {
			instance := ""
			for _, label := range l.Labels {
				if label.Name == LabelInstance {
					instance = label.Value
				}
			}
			messages, ids, instances = append(messages, l.Message), append(ids, l.ID), append(instances, instance)
		}
		if typ == LogTypeBuild {
			instances = nil
		}
		return running("REST "+typ, messages, ids, instances)
	}
	restIDs := rest(LogTypeApp)
	if len(restIDs) != 2 {
		t.Fatalf("REST app read: %d Running lines, want one per start", len(restIDs))
	}
	if built := rest(LogTypeBuild); len(built) != 0 {
		t.Fatalf("REST build read: %d narrated Running lines, want none beside the instance's", len(built))
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", type:"app", startTime:"`+start+`", endTime:"`+end+`") { logs { id message } } }`)
	var messages, ids []string
	for _, row := range data["logs"].(map[string]any)["logs"].([]any) {
		m := row.(map[string]any)
		messages, ids = append(messages, m["message"].(string)), append(ids, m["id"].(string))
	}
	if gql := running("GraphQL", messages, ids, nil); !slices.Equal(gql, restIDs) {
		t.Fatalf("GraphQL ids %q, REST %q", gql, restIDs)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web"}, "type": []string{LogTypeApp}, "startTime": start, "endTime": end,
	}})
	if err != nil || res.IsError {
		t.Fatalf("list_logs: %v %+v", err, res)
	}
	var got renderLogList
	rendertest.DecodeMCPText(t, res, &got)
	messages, ids = nil, nil
	for _, l := range got.Logs {
		messages, ids = append(messages, l.Message), append(ids, l.ID)
	}
	if mcpIDs := running("MCP", messages, ids, nil); !slices.Equal(mcpIDs, restIDs) {
		t.Fatalf("MCP ids %q, REST %q", mcpIDs, restIDs)
	}
}
