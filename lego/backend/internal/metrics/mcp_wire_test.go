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

// TestMCPGetMetricsWireIsRendersText pins get_metrics at the transport. It
// used to pin w4/377's typed contract (structuredContent plus the same JSON as
// text); w4/m378 made the answer Render's instead: one non-empty text block
// carrying Render's MetricsResponse JSON and no structuredContent, which
// Render's server never sends.
func TestMCPGetMetricsWireIsRendersText(t *testing.T) {
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
	if len(env.Result.StructuredContent) != 0 {
		t.Fatalf("structuredContent = %s, want none (Render sends text only)", env.Result.StructuredContent)
	}
	if len(env.Result.Content) != 1 || env.Result.Content[0].Type != "text" {
		t.Fatalf("content = %+v, want one text block", env.Result.Content)
	}
	text := env.Result.Content[0].Text
	if !strings.HasPrefix(text, `{"resourceId":"web","timeRange":{},"metrics":[{"type":"cpu_usage","data":[`) ||
		!strings.Contains(text, `"value":0.5`) {
		t.Fatalf("text = %s, want Render's MetricsResponse carrying the 0.5 cpu point", text)
	}
}
