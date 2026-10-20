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
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestManagedPostgresPodLevels(t *testing.T) {
	pod := postgresID + "-1"
	var lines []string
	for i, severity := range []string{"DEBUG1", "LOG", "INFO", "NOTICE", "WARNING", "FATAL", "ERROR", "PANIC"} {
		lines = append(lines, fmt.Sprintf(`2026-07-05T00:00:%02dZ {"logger":"postgres","msg":"record","record":{"log_time":"2026-07-05 UTC","process_id":7,"error_severity":%q,"message":"event %s"}}`, i+1, severity, severity))
	}
	lines = append(lines, "2026-07-05T00:00:09Z event plain")
	svc := newService(nil, sampleDatabase(postgresID), databasePod(postgresID, pod))
	svc.PodLogs = func(_ context.Context, namespace, name, container string, tail int64) (io.ReadCloser, error) {
		if namespace != "default" || name != pod || container != core.CNPGPostgresContainer {
			t.Fatalf("pod source = %s/%s/%s", namespace, name, container)
		}
		// Like the kubelet, this source caps raw records before severity parsing.
		start := max(0, len(lines)-int(tail))
		return io.NopCloser(strings.NewReader(strings.Join(lines[start:], "\n"))), nil
	}
	for _, tc := range []struct {
		name string
		q    LogQuery
		want []string
	}{
		{"unfiltered", LogQuery{}, []string{"DEBUG1", "LOG", "INFO", "NOTICE", "WARNING", "FATAL", "ERROR", "PANIC", "plain"}},
		{"error includes fatal and panic", LogQuery{Level: []string{"error"}}, []string{"FATAL", "ERROR", "PANIC"}},
		{"warning alias", LogQuery{Level: []string{"warn"}}, []string{"WARNING"}},
		{"notice bucket", LogQuery{Level: []string{"notice"}}, []string{"LOG", "INFO", "NOTICE"}},
		{"critical alias", LogQuery{Level: []string{"critical"}}, []string{"FATAL", "ERROR", "PANIC"}},
		{"multiple values OR", LogQuery{Level: []string{"warning", "error"}}, []string{"WARNING", "FATAL", "ERROR", "PANIC"}},
		{"wildcards", LogQuery{Level: []string{"warn*", "err*"}}, []string{"WARNING", "FATAL", "ERROR", "PANIC"}},
		{"regex metacharacters stay literal", LogQuery{Level: []string{"err.r", "info|error"}}, nil},
		{"unknown level", LogQuery{Level: []string{"unknown"}}, nil},
		{"text AND level", LogQuery{Level: []string{"error"}, Search: []string{"fatal"}}, []string{"FATAL"}},
		{"time AND level", LogQuery{Level: []string{"error"}, Since: time.Date(2026, 7, 5, 0, 0, 7, 0, time.UTC)}, []string{"ERROR", "PANIC"}},
		{"bounded raw pod buffer", LogQuery{Level: []string{"error"}, Limit: 2}, []string{"PANIC"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.q
			q.App = postgresID
			entries, err := svc.QueryLogs(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, entry := range entries {
				parts := strings.Fields(entry.Message)
				severity := parts[len(parts)-1]
				got = append(got, severity)
				level := map[string]string{"DEBUG1": "debug", "LOG": "info", "INFO": "info", "NOTICE": "info", "WARNING": "warning", "FATAL": "error", "ERROR": "error", "PANIC": "error"}[severity]
				if entry.Labels[LabelLevel] != level || entry.Labels[LabelType] != "postgres" || entry.Labels["service"] != postgresID {
					t.Errorf("entry labels = %v, want level %q and Postgres attribution", entry.Labels, level)
				}
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("events = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestManagedPostgresLokiLevels(t *testing.T) {
	for _, tc := range []struct {
		name   string
		levels []string
		want   string
	}{
		{"error", []string{"error"}, `level="error"`},
		{"warning and critical", []string{"warning", "critical"}, `level=~"^(warning|warn|error|critical)$"`},
		{"wildcards", []string{"err*", "warn*"}, `level=~"^(err.*|warn.*)$"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Loki performs stream filtering; pin the actual HTTP selector and
			// ensure a matched normalized FATAL reaches the public response.
			f := newFakeLoki(lokiResp(map[string]any{
				"stream": `{"database":"` + postgresID + `","pod":"` + postgresID + `-1","container":"postgres","type":"postgres","level":"error"}`,
				"values": `[["1751673601000000000","2026-07-05 UTC [7] FATAL:  password authentication failed"]]`,
			}))
			defer f.srv.Close()
			svc := newService(nil, sampleDatabase(postgresID))
			svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
			entries, err := svc.QueryLogs(context.Background(), LogQuery{App: postgresID, Level: tc.levels})
			if err != nil || len(entries) != 1 || entries[0].Labels[LabelLevel] != "error" || !strings.Contains(entries[0].Message, "FATAL:") {
				t.Fatalf("history = %+v, err=%v", entries, err)
			}
			if want, got := `{namespace="default", database="`+postgresID+`", `+tc.want+`}`, f.lastValues.Get("query"); got != want {
				t.Fatalf("Loki query = %q, want %q", got, want)
			}
		})
	}
}

func TestManagedPostgresLevelFilterGuards(t *testing.T) {
	for _, history := range []bool{false, true} {
		t.Run(fmt.Sprintf("history=%t", history), func(t *testing.T) {
			svc := newService(nil, sampleDatabase(postgresID))
			if history {
				svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) {
					t.Fatal("rejected query reached history")
					return nil, nil
				}
			}
			for _, q := range []LogQuery{
				{Types: []string{LogTypeBuild}}, {Types: []string{LogTypeRequest}},
				{Host: []string{"db.example"}}, {StatusCode: []string{"500"}},
				{Method: []string{"GET"}}, {Path: []string{"/"}},
				{Level: []string{string([]byte{0xff})}}, // Invalid UTF-8 must never panic in regexp compilation.
			} {
				q.App = postgresID
				if _, err := svc.QueryLogs(context.Background(), q); !errors.Is(err, core.ErrBadRequest) {
					t.Errorf("query %+v: %v, want bad request", q, err)
				}
			}
			// Authorization precedes both filter validation and compilation.
			svc.Base.Authz = denyLogChecker{}
			if _, err := svc.QueryLogs(context.Background(), LogQuery{App: postgresID, Level: []string{string([]byte{0xff})}}); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("denied query = %v, want forbidden", err)
			}
		})
	}
}

func TestManagedPostgresLevelValues(t *testing.T) {
	f := newFakeLoki(`{"status":"success","data":["warning","error","error"]}`)
	defer f.srv.Close()
	svc := newService(nil, sampleDatabase(postgresID))
	svc.LabelValues = NewLokiLabelValuesSource(f.srv.URL, f.srv.Client())
	values, err := svc.LogLabelValues(context.Background(), LabelLevel, LogQuery{App: postgresID, Level: []string{"warning", "error"}})
	if err != nil || !slices.Equal(values, []string{"error", "warning"}) {
		t.Fatalf("level values = %v, err=%v", values, err)
	}
	if f.lastPath != "/loki/api/v1/label/level/values" || f.lastValues.Get("query") != `{namespace="default", database="`+postgresID+`", level=~"^(warning|warn|error)$"}` {
		t.Fatalf("label source = %s %v", f.lastPath, f.lastValues)
	}
	if _, err := svc.LogLabelValues(context.Background(), LabelMethod, LogQuery{App: postgresID}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Postgres method values = %v, want bad request", err)
	}
	svc.LabelValues = nil
	if _, err := svc.LogLabelValues(context.Background(), LabelLevel, LogQuery{App: postgresID}); !errors.Is(err, core.ErrLogStoreUnavailable) {
		t.Fatalf("level values without store = %v, want unavailable", err)
	}
}

// w5/m139: a Key Value's level filter is Postgres's — the pod read keeps the
// lines whose Valkey marker names a matching level, and the store read selects
// the level label the shipper attaches.
func TestKeyValueLevelFilter(t *testing.T) {
	svc := newManagedTailService()
	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: keyValueID, Level: []string{"warning"}})
	if err != nil || len(entries) != 1 || !strings.Contains(entries[0].Message, "Memory overcommit") || entries[0].Labels[LabelLevel] != "warning" {
		t.Fatalf("pod read = %+v, err=%v; want only the warning line", entries, err)
	}

	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"keyvalue":"` + keyValueID + `","pod":"` + keyValueID + `-0","container":"valkey","type":"keyvalue","level":"warning"}`,
		"values": `[["1751673606000000000","1:M 05 Jul 2026 00:00:06.000 # WARNING Memory overcommit must be enabled!"]]`,
	}))
	defer f.srv.Close()
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	entries, err = svc.QueryLogs(context.Background(), LogQuery{App: keyValueID, Level: []string{"warning"}})
	if err != nil || len(entries) != 1 {
		t.Fatalf("store read = %+v, err=%v", entries, err)
	}
	query := f.lastValues.Get("query")
	if !strings.Contains(query, `keyvalue="`+keyValueID+`"`) || !strings.Contains(query, "level") || !strings.Contains(query, "warning") {
		t.Fatalf("Loki query = %q, want the keyvalue stream narrowed by level", query)
	}
}

// w5/m139: the generic GraphQL `logs` and MCP `list_logs` adapters carry a Key
// Value level filter exactly as REST does — one core verb behind all three.
func TestKeyValueLevelOnEveryAdapter(t *testing.T) {
	svc := newManagedTailService()
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	data := runQuery(t, schema, `{ logs(resource:"`+keyValueID+`", level:["warning"]) { logs { message } } }`)
	raw, _ := json.Marshal(data["logs"])
	if !strings.Contains(string(raw), "Memory overcommit") || strings.Contains(string(raw), "Ready to accept") {
		t.Fatalf("GraphQL logs(level:warning) = %s", raw)
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
	defer func() { _ = cs.Close() }()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{keyValueID}, "level": []string{"warning"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("MCP list_logs = %+v, err=%v", result, err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Memory overcommit") || strings.Contains(text, "Ready to accept") {
		t.Fatalf("MCP list_logs(level:warning) = %s", text)
	}
}

// w5/m139: Key Value lines carry a level now, so level discovery reads the
// keyvalue stream's level index as Postgres's does.
func TestKeyValueLevelValues(t *testing.T) {
	f := newFakeLoki(`{"status":"success","data":["info","warning"]}`)
	defer f.srv.Close()
	svc := newManagedTailService()
	svc.LabelValues = NewLokiLabelValuesSource(f.srv.URL, f.srv.Client())
	values, err := svc.LogLabelValues(context.Background(), LabelLevel, LogQuery{App: keyValueID})
	if err != nil || !slices.Equal(values, []string{"info", "warning"}) {
		t.Fatalf("level values = %v, err=%v", values, err)
	}
	if f.lastPath != "/loki/api/v1/label/level/values" || f.lastValues.Get("query") != `{namespace="default", keyvalue="`+keyValueID+`"}` {
		t.Fatalf("label source = %s %v", f.lastPath, f.lastValues)
	}
}
