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

package modelproxy_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentcred"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/modelproxy"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

var secret = []byte("shared-gateway-secret")

// --- fakes ---------------------------------------------------------------

type fakeKV struct{ data map[string]map[string]string }

func (f fakeKV) Get(_ context.Context, path string) (map[string]string, error) {
	return f.data[path], nil
}
func (fakeKV) Put(context.Context, string, map[string]string) error { return nil }
func (fakeKV) Delete(context.Context, string) error                 { return nil }
func (fakeKV) List(context.Context, string) ([]string, error)       { return nil, nil }

type fakeStore struct {
	session store.AgentSession
	mu      sync.Mutex
	calls   int
}

func (f *fakeStore) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.session.ID != "" && f.session.ID != id {
		return store.AgentSession{}, store.ErrNotFound
	}
	return f.session, nil
}

type fakePods struct {
	pod agentcred.SessionPod
	err error
}

func (f fakePods) ResolveSessionPod(context.Context, string, string) (agentcred.SessionPod, error) {
	return f.pod, f.err
}

type countingPods struct{ calls int }

func (p *countingPods) ResolveSessionPod(context.Context, string, string) (agentcred.SessionPod, error) {
	p.calls++
	return sandboxPod("ags-one"), nil
}

// captureTransport records the outgoing upstream request and returns a canned
// SSE response, so tests assert what reached the vendor without any network.
type captureTransport struct {
	mu   sync.Mutex
	last *http.Request
}

func (c *captureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.last = r.Clone(context.Background())
	c.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: {\"ok\":true}\n\n")),
	}, nil
}

// --- harness -------------------------------------------------------------

func sandboxPod(session string) agentcred.SessionPod {
	return agentcred.SessionPod{Name: "sbx-1-0", UID: "uid-1", Labels: map[string]string{
		agentsession.LabelWorkspace: "tea-a",
		agentsession.LabelRegime:    agentsession.RegimeSandbox,
		agentsession.LabelSession:   session,
	}}
}

func liveSession(endpoint string) store.AgentSession {
	agent := map[string]string{
		"https://api.anthropic.com/v1":                     "claude",
		"https://api.openai.com/v1":                        "codex",
		"https://generativelanguage.googleapis.com/v1beta": "gemini",
	}[endpoint]
	return store.AgentSession{
		ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running",
		AgentConfig: json.RawMessage(`{"agent":"` + agent + `","modelEndpoint":"` + endpoint + `"}`),
	}
}

// harness wires a real bex-api ModelHandler (fake KV + session store) behind a
// real ModelClient, and a Broker whose upstream client is the capture transport.
func harness(t *testing.T, pods fakePods, sess store.AgentSession, key string) (*httptest.Server, *captureTransport, *fakeStore) {
	t.Helper()
	st := &fakeStore{session: sess}
	minter := &agentsession.ModelMinter{
		Keys:     fakeKV{data: map[string]map[string]string{agentsession.ModelKeySecretPath("tea-a"): {agentsession.ModelKeyField: key}}},
		Sessions: st,
	}
	api := httptest.NewServer(&agentsession.ModelHandler{Secret: secret, Minter: minter})
	t.Cleanup(api.Close)

	capture := &captureTransport{}
	broker := &modelproxy.Broker{
		Pods: pods,
		API:  &agentsession.ModelClient{URL: api.URL + agentsession.InternalModelMintPath, Secret: secret, HTTP: api.Client()},
		HTTP: &http.Client{Transport: capture},
	}
	proxy := httptest.NewServer(broker.Handler())
	t.Cleanup(proxy.Close)
	return proxy, capture, st
}

// request drives one model call through the proxy the way an agent's SDK would:
// base URL = ModelProxyURL, then the SDK appends `subpath`.
func request(t *testing.T, proxy *httptest.Server, sessionID, subpath, placeholderHeader, placeholderValue string) *http.Response {
	t.Helper()
	base, err := agentsession.ModelProxyURL(proxy.URL, "tea-a-sandbox", sessionID)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+subpath, strings.NewReader(`{"model":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if placeholderHeader != "" {
		req.Header.Set(placeholderHeader, placeholderValue) // the sandbox's placeholder credential
	}
	resp, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// --- tests ---------------------------------------------------------------

func TestProxyInjectsRealKeyAndPinsHost(t *testing.T) {
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.anthropic.com/v1"), "sk-ant-api03-REAL")
	resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder-inside-sandbox")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ok") {
		t.Fatalf("upstream body not streamed back: %q", body)
	}
	got := capture.last
	if got == nil {
		t.Fatal("no upstream request captured")
	}
	if got.URL.Scheme != "https" || got.URL.Host != "api.anthropic.com" || got.URL.Path != "/v1/messages" {
		t.Fatalf("upstream url = %s://%s%s, want https://api.anthropic.com/v1/messages", got.URL.Scheme, got.URL.Host, got.URL.Path)
	}
	if got.Header.Get("X-Api-Key") != "sk-ant-api03-REAL" {
		t.Fatalf("injected x-api-key = %q, want the real key", got.Header.Get("X-Api-Key"))
	}
	if got.Header.Get("Authorization") != "" {
		t.Fatal("Authorization must be absent for the anthropic-key scheme")
	}
	if strings.Contains(got.Header.Get("X-Api-Key"), "placeholder") {
		t.Fatal("the sandbox placeholder reached the vendor")
	}
}

func TestProxyOAuthAndOpenAISchemes(t *testing.T) {
	t.Run("anthropic oauth", func(t *testing.T) {
		proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.anthropic.com/v1"), "sk-ant-oat01-REAL")
		resp := request(t, proxy, "ags-one", "/v1/messages", "Authorization", "Bearer placeholder")
		resp.Body.Close()
		if got := capture.last.Header.Get("Authorization"); got != "Bearer sk-ant-oat01-REAL" {
			t.Fatalf("Authorization = %q, want the real oauth token", got)
		}
		if capture.last.Header.Get("X-Api-Key") != "" {
			t.Fatal("x-api-key must be absent for the oauth scheme")
		}
	})
	t.Run("openai bearer", func(t *testing.T) {
		proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.openai.com/v1"), "sk-proj-REAL")
		resp := request(t, proxy, "ags-one", "/v1/responses", "Authorization", "Bearer placeholder")
		resp.Body.Close()
		if got := capture.last; got.URL.Host != "api.openai.com" || got.URL.Path != "/v1/responses" || got.Header.Get("Authorization") != "Bearer sk-proj-REAL" {
			t.Fatalf("openai upstream = %s%s auth=%q", got.URL.Host, got.URL.Path, got.Header.Get("Authorization"))
		}
	})
}

func TestProxyGeminiStripsKeyQuery(t *testing.T) {
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://generativelanguage.googleapis.com/v1beta"), "AIzaREAL")
	resp := request(t, proxy, "ags-one", "/v1beta/models/gemini:streamGenerateContent?key=placeholder&alt=sse", "", "")
	resp.Body.Close()
	got := capture.last
	if got.URL.Host != "generativelanguage.googleapis.com" || got.Header.Get("X-Goog-Api-Key") != "AIzaREAL" {
		t.Fatalf("gemini upstream host=%q x-goog-api-key=%q", got.URL.Host, got.Header.Get("X-Goog-Api-Key"))
	}
	if got.URL.Query().Get("key") != "" {
		t.Fatalf("the client-supplied ?key= placeholder was forwarded: %q", got.URL.RawQuery)
	}
	if got.URL.Query().Get("alt") != "sse" {
		t.Fatal("a non-credential query parameter must be preserved")
	}
}

func TestProxyRefusesForeignOrMislabeledPod(t *testing.T) {
	cases := map[string]fakePods{
		"unresolved source pod": {err: agentsession.ErrForbidden},
		"sibling session pod":   {pod: sandboxPod("ags-other")},
	}
	for name, pods := range cases {
		t.Run(name, func(t *testing.T) {
			proxy, capture, _ := harness(t, pods, liveSession("https://api.anthropic.com/v1"), "sk-ant-api03-REAL")
			resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", resp.StatusCode)
			}
			if capture.last != nil {
				t.Fatal("a refused request must never reach the vendor")
			}
		})
	}
}

func TestProxyRefusesNonLiveSession(t *testing.T) {
	// The session is terminal, so bex-api's mint returns 403 → the proxy refuses,
	// and the credential is never released even though the pod identity is intact.
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")},
		store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "completed",
			AgentConfig: json.RawMessage(`{"agent":"claude","modelEndpoint":"https://api.anthropic.com/v1"}`)}, "sk-ant-api03-REAL")
	resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if capture.last != nil {
		t.Fatal("a non-live session must never reach the vendor")
	}
}

func TestProxyRevalidatesCredentialForEveryExchange(t *testing.T) {
	proxy, _, st := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.anthropic.com/v1"), "sk-ant-api03-REAL")
	for i := 0; i < 3; i++ {
		resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
		resp.Body.Close()
	}
	if st.calls != 3 {
		t.Fatalf("session store reads = %d, want 3 (fresh lifecycle check per exchange)", st.calls)
	}
}

func TestProxyRejectsProviderControlPlaneOperations(t *testing.T) {
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.openai.com/v1"), "sk-proj-REAL")
	for _, path := range []string{"/v1/files", "/v1/batches", "/v1/fine_tuning/jobs", "/v1/models"} {
		resp := request(t, proxy, "ags-one", path, "Authorization", "Bearer placeholder")
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s status = %d, want 403", path, resp.StatusCode)
		}
	}
	if capture.last != nil {
		t.Fatal("a refused provider operation reached the upstream")
	}
}

// Claude Code reaches the very same inference operations through the Anthropic
// SDK's beta namespace, which addresses them as `?beta=true`. Refusing a
// non-empty query therefore 403'd every real Claude turn ("403 model operation
// is not allowed" in the agent's own log) while the operation itself was
// allowed — the whole point of the boundary is WHICH operation, not the flag.
func TestProxyAllowsAnthropicBetaNamespaceInferenceCalls(t *testing.T) {
	for _, subpath := range []string{
		"/v1/messages?beta=true",
		"/v1/messages/count_tokens?beta=true",
	} {
		proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.anthropic.com/v1"), "sk-ant-api03-REAL")
		resp := request(t, proxy, "ags-one", subpath, "X-Api-Key", "placeholder-inside-sandbox")
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", subpath, resp.StatusCode)
		}
		if capture.last == nil || capture.last.URL.RawQuery != "beta=true" {
			t.Fatalf("%s: the beta flag must reach the vendor verbatim; got %v", subpath, capture.last)
		}
	}
}

// Admitting the beta flag must not admit arbitrary query parameters: only the
// exact name/value pairs each provider's inference operations legitimately use.
func TestProxyRejectsUnknownAnthropicQueryParameters(t *testing.T) {
	proxy, capture, _ := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.anthropic.com/v1"), "sk-ant-api03-REAL")
	for _, subpath := range []string{
		"/v1/messages?beta=false",
		"/v1/messages?beta=true&redirect=https%3A%2F%2Fevil.example",
		"/v1/messages?beta=true&beta=true",
	} {
		resp := request(t, proxy, "ags-one", subpath, "X-Api-Key", "placeholder-inside-sandbox")
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s status = %d, want 403", subpath, resp.StatusCode)
		}
	}
	if capture.last != nil {
		t.Fatal("a refused provider operation reached the upstream")
	}
}

// The proxy forwards the agent's own Accept-Encoding to the vendor (disabling
// Go's transparent gzip handling), so a compressed response streams back
// verbatim — and its Content-Encoding header MUST travel with it. Dropping the
// header handed the agent's JSON parser raw gzip bytes: every Claude turn died
// with `Unexpected token '\x1f'` (the gzip magic number) and the session
// failed as "sandbox terminated before completion".
func TestProxyRelaysContentEncodingWithCompressedBody(t *testing.T) {
	plain := `{"id":"msg_1","content":[{"type":"text","text":"ok"}]}`
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write([]byte(plain)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	gzipUpstream := &headerTransport{response: func(r *http.Request) *http.Response {
		if r.Header.Get("Accept-Encoding") != "gzip" {
			t.Fatalf("Accept-Encoding = %q did not reach the vendor", r.Header.Get("Accept-Encoding"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}, "Content-Encoding": {"gzip"}},
			Body:       io.NopCloser(bytes.NewReader(compressed.Bytes())),
		}
	}}
	proxy, _ := gzipHarness(t, gzipUpstream)
	base, err := agentsession.ModelProxyURL(proxy.URL, "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(`{"model":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	// An explicit Accept-Encoding (what undici/fetch sends) means the Go test
	// client will NOT transparently decompress either — exactly the agent's view.
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip — without it the agent parses raw gzip bytes as JSON", got)
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("body is not the vendor's gzip stream: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != plain {
		t.Fatalf("decompressed body = %q, want %q", body, plain)
	}
}

type headerTransport struct {
	response func(*http.Request) *http.Response
}

func (h *headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return h.response(r), nil
}

// gzipHarness mirrors harness but lets the test supply the upstream transport.
func gzipHarness(t *testing.T, transport http.RoundTripper) (*httptest.Server, *fakeStore) {
	t.Helper()
	st := &fakeStore{session: liveSession("https://api.anthropic.com/v1")}
	minter := &agentsession.ModelMinter{
		Keys:     fakeKV{data: map[string]map[string]string{agentsession.ModelKeySecretPath("tea-a"): {agentsession.ModelKeyField: "sk-ant-api03-REAL"}}},
		Sessions: st,
	}
	api := httptest.NewServer(&agentsession.ModelHandler{Secret: secret, Minter: minter})
	t.Cleanup(api.Close)
	broker := &modelproxy.Broker{
		Pods: fakePods{pod: sandboxPod("ags-one")},
		API:  &agentsession.ModelClient{URL: api.URL + agentsession.InternalModelMintPath, Secret: secret, HTTP: api.Client()},
		HTTP: &http.Client{Transport: transport},
	}
	proxy := httptest.NewServer(broker.Handler())
	t.Cleanup(proxy.Close)
	return proxy, st
}

func TestProxyRejectsOversizedRequestBeforePodOrCredentialLookup(t *testing.T) {
	proxy, capture, st := harness(t, fakePods{pod: sandboxPod("ags-one")}, liveSession("https://api.openai.com/v1"), "sk-proj-REAL")
	base, err := agentsession.ModelProxyURL(proxy.URL, "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/responses", strings.NewReader(strings.Repeat("x", (4<<20)+1)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
	if st.calls != 0 || capture.last != nil {
		t.Fatalf("oversized request crossed admission boundary: session reads=%d upstream=%v", st.calls, capture.last != nil)
	}
}

func TestProxyConcurrencyLimitPrecedesPodAndCredentialLookup(t *testing.T) {
	limits := sshgateway.NewSessionLimiter(1, 1)
	if ok, _ := limits.Acquire("10.0.0.7"); !ok {
		t.Fatal("failed to seed limiter")
	}
	defer limits.Release("10.0.0.7")
	pods := &countingPods{}
	broker := &modelproxy.Broker{
		Pods:   pods,
		API:    &agentsession.ModelClient{URL: "http://unused", Secret: secret},
		Limits: limits,
	}
	path, err := agentsession.ModelProxyURL("http://proxy", "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path+"/v1/responses", strings.NewReader(`{}`))
	req.RemoteAddr = "10.0.0.7:12345"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	broker.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	if pods.calls != 0 {
		t.Fatalf("over-limit request performed %d Pod lookups", pods.calls)
	}
}
