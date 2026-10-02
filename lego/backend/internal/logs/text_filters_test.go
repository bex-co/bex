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
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestTextFilterUnionAcrossAdaptersAndPodSources(t *testing.T) {
	lines := []string{
		"2026-07-05T00:00:01Z qa_text_A",
		"2026-07-05T00:00:02Z qa_text_B",
		"2026-07-05T00:00:03Z unrelated control",
		"2026-07-05T00:00:04Z qa_text_A qa_text_B",
	}
	pgPod, kvPod := postgresID+"-1", keyValueID+"-0"
	svc := newService(map[string][]string{"web-1": lines, pgPod: lines, kvPod: lines},
		sampleApp("web"), podFor("web", "web-1"), sampleDatabase(postgresID), databasePod(postgresID, pgPod),
		&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: keyValueID, Namespace: "default"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: kvPod, Namespace: "default", Labels: map[string]string{core.PodLabelKeyValue: keyValueID}}},
	)
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
	for _, resource := range []string{"web", postgresID, keyValueID} {
		for _, tc := range []struct {
			name        string
			terms, want []string
		}{
			{"forward", []string{"qa_text_A", "qa_text_B"}, []string{"qa_text_A", "qa_text_B", "qa_text_A qa_text_B"}},
			{"reverse", []string{"qa_text_B", "qa_text_A"}, []string{"qa_text_A", "qa_text_B", "qa_text_A qa_text_B"}},
			{"empty_and_duplicate", []string{"", "QA_TEXT_A", "qa_text_a", "qa_text_B", ""}, []string{"qa_text_A", "qa_text_B", "qa_text_A qa_text_B"}},
			{"no_match", []string{"absent", "missing"}, nil},
			{"all_empty", []string{"", ""}, []string{"qa_text_A", "qa_text_B", "unrelated control", "qa_text_A qa_text_B"}},
		} {
			t.Run(resource+"/"+tc.name, func(t *testing.T) {
				params := url.Values{"resource": {resource}, "text": tc.terms}
				rec := serveREST(svc, http.MethodGet, "/v1/logs?"+params.Encode())
				if rec.Code != http.StatusOK {
					t.Fatalf("REST: %d %s", rec.Code, rec.Body.String())
				}
				result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{"resource": []string{resource}, "text": tc.terms}})
				if err != nil {
					t.Fatal(err)
				}
				if result.IsError {
					t.Fatalf("MCP: %+v", result)
				}
				for adapter, body := range map[string][]byte{"REST": rec.Body.Bytes(), "MCP": []byte(result.Content[0].(*mcp.TextContent).Text)} {
					var page wirePage
					if err := json.Unmarshal(body, &page); err != nil {
						t.Fatalf("%s: %v", adapter, err)
					}
					var got []string
					for _, row := range page.Logs {
						got = append(got, row.Message)
					}
					if !slices.Equal(got, tc.want) {
						t.Errorf("%s messages = %q, want %q", adapter, got, tc.want)
					}
				}
			})
		}
	}
}

func TestTextFilterLiteralsAgreeWithLoki(t *testing.T) {
	terms := []string{"", `A.*`, `B|C`, `"} |= "`, `x\y`, " spaced ", `a.*`}
	q := LogQuery{App: "web", Search: terms}.normalized()
	query := lokiQueryFor("default", q)
	_, quoted, ok := strings.Cut(query, " |~ ")
	if !ok {
		t.Fatalf("no line predicate: %s", query)
	}
	pattern, err := strconv.Unquote(quoted)
	if err != nil {
		t.Fatalf("unsafe LogQL string %q: %v", quoted, err)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		message string
		want    bool
	}{
		{"prefix a.* suffix", true}, {"B|C", true}, {`literal "} |= " here`, true}, {`X\Y`, true}, {"contains spaced literal", true},
		{"alphabet", false}, {"B", false}, {"C", false}, {"spaced", false}, {"unrelated", false},
	} {
		if got := q.keep(LogEntry{Message: tc.message}); got != tc.want {
			t.Errorf("pod %q = %v, want %v", tc.message, got, tc.want)
		}
		if got := re.MatchString(tc.message); got != tc.want {
			t.Errorf("Loki %q = %v, want %v (%s)", tc.message, got, tc.want, query)
		}
	}
	for _, terms := range [][]string{nil, {"", ""}} {
		empty := LogQuery{App: "web", Search: terms}.normalized()
		if empty.hasFilters() || lokiScansLines(empty) || strings.Contains(lokiQueryFor("default", empty), " |~ ") {
			t.Fatalf("empty terms must remain unfiltered: %+v", empty)
		}
	}
	if !q.hasFilters() || !lokiScansLines(q) {
		t.Fatal("multi-term search must use filtered reads and bounded Loki scanning")
	}
}

func TestTextFilterUnionAndsLevelAndTimeBeforeCapping(t *testing.T) {
	start := time.Date(2026, 7, 5, 0, 0, 1, 0, time.UTC)
	q := LogQuery{Search: []string{"A", "B"}, levelFilter: regexp.MustCompile("^error$"), Since: start, End: start.Add(2 * time.Second), Limit: 100, Direction: DirectionForward}.normalized()
	var entries []LogEntry
	for _, row := range []struct {
		second     int
		msg, level string
	}{
		{0, "A too early", "error"}, {1, "A accepted", "error"}, {2, "B accepted", "error"}, {2, "A wrong level", "info"}, {3, "control", "error"}, {4, "B too late", "error"},
	} {
		entries = append(entries, LogEntry{Timestamp: start.Add(time.Duration(row.second-1) * time.Second).Format(time.RFC3339Nano), Message: row.msg, Labels: map[string]string{LabelLevel: row.level}})
	}
	got := q.filterAndCap(slices.Clone(entries))
	if len(got) != 2 || got[0].Message != "A accepted" || got[1].Message != "B accepted" {
		t.Fatalf("filtered page = %+v", got)
	}
	q.Limit = 1
	got = q.filterAndCap(entries)
	if len(got) != 1 || got[0].Message != "A accepted" {
		t.Fatalf("capped page = %+v", got)
	}
}

func TestTextFilterUnionOnAppTail(t *testing.T) {
	svc := newService(map[string][]string{"web-1": {
		"2026-07-05T00:00:01Z qa_text_A", "2026-07-05T00:00:02Z qa_text_B", "2026-07-05T00:00:03Z control",
	}}, sampleApp("web"), podFor("web", "web-1"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?resource=web&text=qa_text_A&text=qa_text_B", nil)
	serveSubscribe(mux, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tail = %d %s", rec.Code, rec.Body.String())
	}
	decoder := json.NewDecoder(rec.Body)
	var messages []string
	for {
		var entry renderLog
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		messages = append(messages, entry.Message)
	}
	if !slices.Equal(messages, []string{"qa_text_A", "qa_text_B"}) {
		t.Fatalf("tail messages = %q", messages)
	}
}

func TestTextFilterUnionOnBuildHistoryProgress(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{inFlightDeploy()}, nil
	}
	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}, Search: []string{"build queued", "building from"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Message != "==> Build queued" || !strings.HasPrefix(entries[1].Message, "==> Building from") {
		t.Fatalf("progress union = %+v", entries)
	}
}
