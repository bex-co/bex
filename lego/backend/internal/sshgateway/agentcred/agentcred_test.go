package agentcred

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type credentialPodResolver struct{ pods map[string]SessionPod }

func (r credentialPodResolver) ResolveSessionPod(_ context.Context, _, sourceIP string) (SessionPod, error) {
	return r.pods[sourceIP], nil
}

type credentialConnections struct{}

func (credentialConnections) GetGitConnection(_ context.Context, workspace string) (store.GitConnection, error) {
	return store.GitConnection{WorkspaceID: workspace, InstallationID: 42, AccountLogin: "octo"}, nil
}

type credentialSessions struct{}

func (credentialSessions) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	return store.AgentSession{ID: id, WorkspaceID: "tea-a", SandboxID: "sbx-a", Phase: "running"}, nil
}

type credentialGitHub struct{ calls int }

func (g *credentialGitHub) MintSessionInstallationToken(context.Context, int64, string) (github.InstallationToken, error) {
	g.calls++
	return github.InstallationToken{Token: "ghs_gateway_secret", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

type credentialAudit struct{ events []core.AuditEvent }

func (a *credentialAudit) Record(_ context.Context, event core.AuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

func credentialLabels(t *testing.T, workspace string) map[string]string {
	t.Helper()
	labels, err := agentsession.BindingLabels("ags-one", "octo/repo", "bex-agent/task-1")
	if err != nil {
		t.Fatal(err)
	}
	labels[agentsession.LabelWorkspace] = workspace
	labels[agentsession.LabelRegime] = agentsession.RegimeSandbox
	return labels
}

func TestGitProxyKeepsTokenOutOfSandboxAndBindsSourcePod(t *testing.T) {
	secret := []byte("shared-domain-separated-secret")
	gh := &credentialGitHub{}
	apiHandler := &agentsession.Handler{Secret: secret, Minter: &agentsession.Minter{
		GitHub: gh, Connections: credentialConnections{}, Sessions: credentialSessions{},
	}}
	apiServer := httptest.NewServer(apiHandler)
	defer apiServer.Close()

	var upstreamAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		_, _ = io.WriteString(w, "0000")
	}))
	defer upstream.Close()

	audit := &credentialAudit{}
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{
			"10.0.0.1": {Name: "sandbox-a", UID: "uid-a", Labels: credentialLabels(t, "tea-a")},
			"10.0.0.2": {Name: "sandbox-b", UID: "uid-b", Labels: credentialLabels(t, "tea-b")},
		}},
		API:   &agentsession.Client{URL: apiServer.URL, Secret: secret, HTTP: apiServer.Client()},
		Audit: audit, UpstreamOrigin: upstream.URL, HTTP: upstream.Client(),
	}
	repoURL, err := agentsession.ProxyRepositoryURL("http://gateway.internal:8082", "tea-a-sandbox", "ags-one", "octo/repo", "bex-agent/task-1")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, repoURL+"/info/refs?service=git-upload-pack", nil)
	request.RemoteAddr = "10.0.0.1:43210"
	recorder := httptest.NewRecorder()
	broker.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "0000" {
		t.Fatalf("proxy status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "ghs_gateway_secret") || strings.Contains(fmt.Sprint(recorder.Header()), "ghs_gateway_secret") {
		t.Fatal("raw GitHub token escaped into the sandbox response")
	}
	if upstreamAuthorization == "" || !strings.HasPrefix(upstreamAuthorization, "Basic ") {
		t.Fatalf("trusted upstream hop did not receive injected auth: %q", upstreamAuthorization)
	}
	if gh.calls != 1 || len(audit.events) != 1 {
		t.Fatalf("github calls=%d audits=%d", gh.calls, len(audit.events))
	}

	request = httptest.NewRequest(http.MethodGet, repoURL+"/info/refs?service=git-upload-pack", nil)
	request.RemoteAddr = "10.0.0.2:43210"
	recorder = httptest.NewRecorder()
	broker.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || gh.calls != 1 {
		t.Fatalf("cross-workspace status=%d mint calls=%d", recorder.Code, gh.calls)
	}
}

func TestGitProxyReceivePackAllowsOnlyBoundBranch(t *testing.T) {
	secret := []byte("secret")
	gh := &credentialGitHub{}
	apiServer := httptest.NewServer(&agentsession.Handler{Secret: secret, Minter: &agentsession.Minter{
		GitHub: gh, Connections: credentialConnections{}, Sessions: credentialSessions{},
	}})
	defer apiServer.Close()
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { upstreamCalls++; _, _ = io.WriteString(w, "0000") }))
	defer upstream.Close()
	broker := &Broker{Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{"10.0.0.1": {Name: "sandbox-a", UID: "uid-a", Labels: credentialLabels(t, "tea-a")}}},
		API:  &agentsession.Client{URL: apiServer.URL, Secret: secret, HTTP: apiServer.Client()}, UpstreamOrigin: upstream.URL, HTTP: upstream.Client()}
	base, _ := agentsession.ProxyRepositoryURL("http://gateway", "tea-a-sandbox", "ags-one", "octo/repo", "bex-agent/task-1")
	push := func(ref string) int {
		old := strings.Repeat("1", 40)
		next := strings.Repeat("2", 40)
		line := old + " " + next + " " + ref + "\x00report-status\n"
		body := fmt.Sprintf("%04x%s0000PACK", len(line)+4, line)
		req := httptest.NewRequest(http.MethodPost, base+"/git-receive-pack", strings.NewReader(body))
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
		rec := httptest.NewRecorder()
		broker.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	if got := push("refs/heads/main"); got != http.StatusForbidden {
		t.Fatalf("wrong branch status=%d", got)
	}
	if gh.calls != 0 || upstreamCalls != 0 {
		t.Fatal("wrong-branch push reached credential mint or GitHub")
	}
	if got := push("refs/heads/bex-agent/task-1"); got != http.StatusOK {
		t.Fatalf("bound branch status=%d", got)
	}
	if gh.calls != 1 || upstreamCalls != 1 {
		t.Fatalf("valid push mints=%d upstream=%d", gh.calls, upstreamCalls)
	}
}

func TestAgentCredentialInternalMintRejectsUnsignedRequest(t *testing.T) {
	handler := &agentsession.Handler{Secret: []byte("secret"), Minter: &agentsession.Minter{}}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, agentsession.InternalMintPath, strings.NewReader(`{}`)))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned status=%d", recorder.Code)
	}
}

// blockingPodResolver holds each lookup open until released, counting how
// many reached it — the seam that proves the concurrency slot is acquired
// BEFORE the Kubernetes lookup (codex round-9 #4).
type blockingPodResolver struct {
	entered chan struct{} // one send per ResolveSessionPod call
	release chan struct{}
}

func (r *blockingPodResolver) ResolveSessionPod(ctx context.Context, _, _ string) (SessionPod, error) {
	r.entered <- struct{}{}
	select {
	case <-r.release:
		return SessionPod{}, agentsession.ErrForbidden
	case <-ctx.Done():
		return SessionPod{}, agentsession.ErrForbidden
	}
}

// codex round-9 #4: a hostile session Pod must not stack unbounded in-flight
// proxy requests. The per-source cap is enforced before ANY stateful work —
// the over-limit request is refused 429 without a second Pod lookup, a
// credential mint, or an upstream hop.
func TestGitProxyConcurrencySlotPrecedesPodLookup(t *testing.T) {
	resolver := &blockingPodResolver{entered: make(chan struct{}, 8), release: make(chan struct{})}
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods:    resolver,
		// Enabled() requires a non-nil API client; the blocked resolver never
		// lets a request reach the mint, so an unreachable URL is fine.
		API: &agentsession.Client{URL: "http://mint.invalid", Secret: []byte("x")},
		// Per-source cap of 1: the second concurrent request from the same
		// source IP must shed before reaching the resolver.
		Limits: sshgateway.NewSessionLimiter(64, 1),
	}
	repoURL, err := agentsession.ProxyRepositoryURL("http://gateway.internal:8082", "tea-a-sandbox", "ags-one", "octo/repo", "bex-agent/task-1")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan int, 2)
	for range 2 {
		go func() {
			req := httptest.NewRequest(http.MethodGet, repoURL+"/info/refs?service=git-upload-pack", nil)
			req.RemoteAddr = "10.0.0.1:43210"
			rec := httptest.NewRecorder()
			broker.Handler().ServeHTTP(rec, req)
			done <- rec.Code
		}()
	}

	// The first request enters the resolver and blocks, holding its slot.
	<-resolver.entered
	// The second must come back 429 without a second resolver entry.
	select {
	case code := <-done:
		if code != http.StatusTooManyRequests {
			t.Fatalf("over-limit request status=%d, want 429", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("over-limit request was not refused promptly")
	}
	select {
	case <-resolver.entered:
		t.Fatal("over-limit request reached the Pod lookup — the slot must be acquired first")
	default:
	}
	// Release the admitted request and let it finish (the fake resolver
	// answers ErrForbidden, so the admitted path is 403, never 429).
	close(resolver.release)
	if code := <-done; code == http.StatusTooManyRequests {
		t.Fatal("the admitted request was wrongly refused")
	}
}

// The default upstream client must be built once and reused: a per-request
// client discards its connection pool, so every clone/push re-dials and re-runs
// the TLS handshake to the forge. The sibling model proxy memoizes for the same
// reason; this proxy did not, and nothing failed when it didn't.
func TestGitProxyReusesOneUpstreamClient(t *testing.T) {
	b := &Broker{}
	first, second := b.httpClient(), b.httpClient()
	if first != second {
		t.Fatal("httpClient built a new client per call: the upstream connection pool is discarded between requests")
	}
	if first.Timeout != 0 {
		t.Fatalf("upstream client must not carry a total timeout (it truncates a streaming pack); got %s", first.Timeout)
	}
}

// An injected test client still gets the no-follow policy forced onto a copy —
// a redirect would replay the injected GitHub token to the redirect target.
func TestGitProxyForcesNoRedirectOnInjectedClient(t *testing.T) {
	injected := &http.Client{}
	b := &Broker{HTTP: injected}
	got := b.httpClient()
	if got == injected {
		t.Fatal("injected client must be copied, not mutated in place")
	}
	if got.CheckRedirect == nil {
		t.Fatal("injected client must still refuse to auto-follow redirects")
	}
	if err := got.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatalf("CheckRedirect = %v, want ErrUseLastResponse", err)
	}
}
