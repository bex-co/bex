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
	"net/http"
	"net/url"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const keyValueID = "red-c185th5c2rvvnhbfiltg"

// logPage is the one envelope every adapter returns, decoded the same way.
type logPage struct {
	HasMore       bool   `json:"hasMore"`
	NextStartTime string `json:"nextStartTime"`
	NextEndTime   string `json:"nextEndTime"`
	Logs          []struct {
		Message string `json:"message"`
	} `json:"logs"`
}

// TestDatastoreLogsPageCursorsChainOnEveryAdapter proves the envelope the
// dashboard's datastore Logs tabs page through (w4/m136) is real for red- and
// dpg- resources on REST, GraphQL, and MCP alike: following nextStartTime/
// nextEndTime from the newest page reaches hasMore=false with every line of the
// window exactly once — the chain w4/m107 proved for services.
func TestDatastoreLogsPageCursorsChainOnEveryAdapter(t *testing.T) {
	const (
		start = "2026-07-05T00:00:00Z"
		end   = "2026-07-05T00:01:00Z"
		total = 11
		limit = 4
	)
	lines := make([]string, total)
	for i := range lines {
		lines[i] = fmt.Sprintf("2026-07-05T00:00:%02dZ line-%02d", i+1, i+1)
	}
	kvPod := keyValueID + "-0"
	pgPod := postgresID + "-1"
	svc := newService(map[string][]string{kvPod: lines, pgPod: lines},
		sampleDatabase(postgresID), databasePod(postgresID, pgPod),
		&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: keyValueID, Namespace: "default"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: kvPod, Namespace: "default", Labels: map[string]string{core.PodLabelKeyValue: keyValueID},
		}},
	)

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	adapters := map[string]func(resource, from, to string) logPage{
		"REST": func(resource, from, to string) logPage {
			q := url.Values{"resource": {resource}, "startTime": {from}, "endTime": {to}, "limit": {fmt.Sprint(limit)}}
			rec := serveREST(svc, http.MethodGet, "/v1/logs?"+q.Encode())
			if rec.Code != http.StatusOK {
				t.Fatalf("REST %s => %d %s", resource, rec.Code, rec.Body.String())
			}
			var page logPage
			if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
				t.Fatalf("REST decode: %v", err)
			}
			return page
		},
		"GraphQL": func(resource, from, to string) logPage {
			data := runQuery(t, schema, fmt.Sprintf(
				`{ logs(resource:%q, startTime:%q, endTime:%q, limit:%d) { hasMore nextStartTime nextEndTime logs { message } } }`,
				resource, from, to, limit))
			raw, _ := json.Marshal(data["logs"])
			var page logPage
			if err := json.Unmarshal(raw, &page); err != nil {
				t.Fatalf("GraphQL decode: %v", err)
			}
			return page
		},
		"MCP": func(resource, from, to string) logPage {
			result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
				"resource": []string{resource}, "startTime": from, "endTime": to, "limit": limit,
			}})
			if err != nil || result.IsError {
				t.Fatalf("MCP %s = %+v, err=%v", resource, result, err)
			}
			var page logPage
			if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &page); err != nil {
				t.Fatalf("MCP decode: %v", err)
			}
			return page
		},
	}

	for name, fetch := range adapters {
		for _, resource := range []string{postgresID, keyValueID} {
			t.Run(name+"/"+resource, func(t *testing.T) {
				seen := map[string]bool{}
				from, to := start, end
				for pageNo := 0; ; pageNo++ {
					if pageNo > total {
						t.Fatal("paging did not reach hasMore=false")
					}
					page := fetch(resource, from, to)
					for _, line := range page.Logs {
						if seen[line.Message] {
							t.Fatalf("page %d repeated %q", pageNo, line.Message)
						}
						seen[line.Message] = true
					}
					if !page.HasMore {
						break
					}
					if page.NextStartTime == "" || page.NextEndTime == "" {
						t.Fatalf("page %d has more but no cursor: %+v", pageNo, page)
					}
					from, to = page.NextStartTime, page.NextEndTime
				}
				if len(seen) != total {
					t.Fatalf("collected %d distinct lines, want %d: %v", len(seen), total, seen)
				}
			})
		}
	}
}
