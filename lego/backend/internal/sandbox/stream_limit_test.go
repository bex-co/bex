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

package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// streamLimitMessage is bex-api's answer when the gateway refuses a sandbox
// stream for concurrency (w22/m623).
const streamLimitMessage = "too many sandbox commands and file transfers are running at once; retry in 1 second"

// streamLimitGateway answers every exec and file request as the gateway does
// at its sandbox stream bound.
func streamLimitGateway(t *testing.T) *httptest.Server {
	t.Helper()
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":"sandbox stream limit reached","message":"sandbox stream limit reached"}`)
	}))
	t.Cleanup(gw.Close)
	return gw
}

// wantStreamLimit holds a REST answer to Render's over-limit shape: 429 with
// Retry-After, which the Render SDK raises as RateLimitError (a 400 is its
// ClientError), and bex's message naming the wait, never the gateway's own
// body or the 400 it once was.
func wantStreamLimit(t *testing.T, surface string, resp *http.Response, body string) {
	t.Helper()
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "1" {
		t.Fatalf("%s: status %d Retry-After %q, want 429 and 1: %s", surface, resp.StatusCode, resp.Header.Get("Retry-After"), body)
	}
	var out struct{ Message string }
	if err := json.Unmarshal([]byte(body), &out); err != nil || out.Message != streamLimitMessage {
		t.Fatalf("%s: body %s, want message %q", surface, body, streamLimitMessage)
	}
	if strings.Contains(body, "stream limit reached") || strings.Contains(body, "too many concurrent exec sessions") {
		t.Fatalf("%s: body %s passes the gateway's refusal through", surface, body)
	}
}

// The two-step exec's redeem (the pinned CLI and the SDKs) and the
// single-step POST …/exec answer the gateway's refusal before the stream
// opens.
func TestExecStreamLimitIsA429OnREST(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	gw := streamLimitGateway(t)
	f.svc.Exec.GatewayURL, f.svc.Exec.Client = gw.URL, gw.Client()

	resp, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("mint = %d %v", resp.StatusCode, out)
	}
	resp, body := f.redeem(t, out["uri"].(string), out["token"].(string), c.Command)
	wantStreamLimit(t, "connect redeem", resp, body)

	exec, err := http.Post(f.api.URL+"/v1/sandboxes/os-1/exec?ownerId=tea-a", "application/json", strings.NewReader(`{"command":"true"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Body.Close()
	raw, _ := io.ReadAll(exec.Body)
	wantStreamLimit(t, "POST …/exec", exec, string(raw))
}

// The file redeem answers the gateway's refusal itself, in both directions,
// instead of passing the gateway's JSON through without Retry-After.
func TestFileStreamLimitIsA429OnREST(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":"sandbox stream limit reached","message":"sandbox stream limit reached"}`)
	})
	for operation, payload := range map[string]io.Reader{"upload": bytes.NewReader([]byte("data")), "download": nil} {
		resp := redeemFile(t, f.mint(t, operation, "/workspace/out.txt"), payload)
		body, _ := io.ReadAll(resp.Body)
		wantStreamLimit(t, operation+" redeem", resp, string(body))
	}
}

// MCP's sandbox_exec names the refusal as a rate limit, the way a rate
// class's refusal reads.
func TestExecStreamLimitIsARateLimitOnMCP(t *testing.T) {
	gw := streamLimitGateway(t)
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: execSandboxClient(t, "id-a"),
		Exec:   &ExecConfig{Secret: []byte("exec-secret"), GatewayURL: gw.URL, Client: gw.Client()},
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := callerCtx()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "sandbox_exec", Arguments: map[string]any{"ownerId": "tea-a", "id": "os-1", "command": "true"}})
	if err != nil {
		t.Fatal(err)
	}
	if text := res.Content[0].(*mcp.TextContent).Text; !res.IsError || text != mcpRateLimitedCode+": "+streamLimitMessage {
		t.Fatalf("MCP sandbox_exec at the stream bound = %#v, want %q", res.Content, mcpRateLimitedCode+": "+streamLimitMessage)
	}
}

// A command that waited for its sandbox's resume already has its SSE stream
// open: the refusal is the stream's error event, with the same status, code
// and message.
func TestExecStreamLimitAfterAResumeWaitIsAnErrorEvent(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-1", "Paused", f.now.Add(-time.Hour), "900", nil)
	gw := streamLimitGateway(t)
	f.svc.Exec.GatewayURL, f.svc.Exec.Client = gw.URL, gw.Client()
	rr := httptest.NewRecorder()
	if err := f.svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"}, rr, rr.Flush); err != nil {
		t.Fatalf("StreamExec after a resume wait = %v, want the error as an event", err)
	}
	body := rr.Body.String()
	_, data, ok := strings.Cut(body, "event: error\ndata: ")
	if rr.Code != http.StatusOK || !ok {
		t.Fatalf("resume-wait stream = %d %s, want an SSE error event", rr.Code, body)
	}
	var event struct {
		Status        int
		Message, Code string
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &event); err != nil {
		t.Fatalf("error event %q: %v", data, err)
	}
	if event.Status != http.StatusTooManyRequests || event.Code != mcpRateLimitedCode || event.Message != streamLimitMessage {
		t.Fatalf("error event = %+v, want 429 %s %q", event, mcpRateLimitedCode, streamLimitMessage)
	}
}

// The agent-session Completer's status read meets the refusal as a passing
// failure: it retries, and never reads the session's sandbox as gone.
func TestStreamLimitIsTransientForASessionRead(t *testing.T) {
	lifecycle := sessionReadFixture(t, "Running", &podExecutor{})
	lifecycle.service.Exec.GatewayURL = streamLimitGateway(t).URL
	_, err := lifecycle.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent")
	if !Transient(err) || !errors.Is(err, core.ErrTooManyRequests) || errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("status read at the stream bound = %v, want a transient 429", err)
	}
}
