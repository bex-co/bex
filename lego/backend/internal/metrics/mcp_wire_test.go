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

package metrics

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPCallReturnsStructuredContent pins the w4/377 contract at the
// transport: a tools/call for a typed tool returns BOTH structuredContent and
// a text block carrying the same JSON — an MCP client must never have to
// scrape text (or find it empty). This probe is exactly what the filed bug
// lacked on the deployed build.
func TestMCPCallReturnsStructuredContent(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))
	srv := mcp.NewServer(&mcp.Implementation{Name: "bex-test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, &mcp.StreamableHTTPOptions{Stateless: true})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_metrics","arguments":{"resource":["web"],"metricTypes":["cpu_usage"]}}}`
	req, err := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call => %d %s", resp.StatusCode, raw)
	}
	_, data, ok := strings.Cut(string(raw), "data: ")
	if !ok {
		t.Fatalf("no SSE data frame in %s", raw)
	}
	var env struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			StructuredContent json.RawMessage `json:"structuredContent"`
			IsError           bool            `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &env); err != nil {
		t.Fatal(err)
	}
	if env.Result.IsError {
		t.Fatalf("tool error: %s", raw)
	}
	if len(env.Result.StructuredContent) == 0 ||
		!strings.Contains(string(env.Result.StructuredContent), `"series"`) ||
		!strings.Contains(string(env.Result.StructuredContent), `"value":0.5`) {
		t.Fatalf("structuredContent missing or wrong: %s", env.Result.StructuredContent)
	}
	if len(env.Result.Content) != 1 || env.Result.Content[0].Type != "text" ||
		env.Result.Content[0].Text == "" ||
		env.Result.Content[0].Text != string(env.Result.StructuredContent) {
		t.Fatalf("text block must carry the same JSON, got %+v", env.Result.Content)
	}
}
