package agentcred

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type credentialPodResolver struct{ pods map[string]SessionPod }

func (r credentialPodResolver) ResolveSessionPod(_ context.Context, _, sourceIP string) (SessionPod, error) {
	return r.pods[sourceIP], nil
}

type credentialConnections struct{}

func (credentialConnections) GetGitConnectionByOwner(_ context.Context, workspace, accountLogin string) (store.GitConnection, error) {
	return store.GitConnection{WorkspaceID: workspace, InstallationID: 42, AccountLogin: accountLogin}, nil
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
			"10.0.0.1": {Name: "sbx-a-0", UID: "uid-a", Labels: credentialLabels(t, "tea-a")},
			"10.0.0.2": {Name: "sbx-a-0", UID: "uid-b", Labels: credentialLabels(t, "tea-b")},
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

// gitProxyHarness wires a broker against a caller-supplied upstream handler,
// returning the broker, the bound proxy repo URL (session ags-one, repo
// octo/repo, branch bex-agent/task-1, source pod 10.0.0.1), and the mint
// counter.
func gitProxyHarness(t *testing.T, upstreamHandler http.HandlerFunc) (*Broker, string, *credentialGitHub) {
	t.Helper()
	secret := []byte("harness-secret")
	gh := &credentialGitHub{}
	apiServer := httptest.NewServer(&agentsession.Handler{Secret: secret, Minter: &agentsession.Minter{
		GitHub: gh, Connections: credentialConnections{}, Sessions: credentialSessions{},
	}})
	t.Cleanup(apiServer.Close)
	upstream := httptest.NewServer(upstreamHandler)
	t.Cleanup(upstream.Close)
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{
			"10.0.0.1": {Name: "sbx-a-0", UID: "uid-a", Labels: credentialLabels(t, "tea-a")},
		}},
		API:            &agentsession.Client{URL: apiServer.URL, Secret: secret, HTTP: apiServer.Client()},
		UpstreamOrigin: upstream.URL, HTTP: upstream.Client(),
	}
	repoURL, err := agentsession.ProxyRepositoryURL("http://gateway", "tea-a-sandbox", "ags-one", "octo/repo", "bex-agent/task-1")
	if err != nil {
		t.Fatal(err)
	}
	return broker, repoURL, gh
}

func gitBudgetHarness(t *testing.T, perSession, perWorkspace int) (*Broker, string, *credentialGitHub) {
	t.Helper()
	broker, repoURL, gh := gitProxyHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "0000")
	})
	// Safe post-construction: budgets() memoizes lazily on the first request.
	broker.MaxRequestsPerSession, broker.MaxRequestsPerWorkspace = perSession, perWorkspace
	return broker, repoURL, gh
}

func gitBudgetRequest(broker *Broker, repoURL string) int {
	req := httptest.NewRequest(http.MethodGet, repoURL+"/info/refs?service=git-upload-pack", nil)
	req.RemoteAddr = "10.0.0.1:43210"
	rec := httptest.NewRecorder()
	broker.Handler().ServeHTTP(rec, req)
	return rec.Code
}

func TestGitProxySessionRequestBudget(t *testing.T) {
	broker, repoURL, gh := gitBudgetHarness(t, 2, 0)
	if got := gitBudgetRequest(broker, repoURL); got != http.StatusOK {
		t.Fatalf("request 1 = %d", got)
	}
	if got := gitBudgetRequest(broker, repoURL); got != http.StatusOK {
		t.Fatalf("request 2 = %d", got)
	}
	if got := gitBudgetRequest(broker, repoURL); got != http.StatusTooManyRequests {
		t.Fatalf("request 3 = %d, want 429", got)
	}
	if gh.calls != 2 {
		t.Fatalf("credential mints = %d, want 2", gh.calls)
	}
}

func TestGitProxyWorkspaceRequestBudgetAndDisabled(t *testing.T) {
	broker, repoURL, gh := gitBudgetHarness(t, 0, 2)
	for i := 0; i < 2; i++ {
		if got := gitBudgetRequest(broker, repoURL); got != http.StatusOK {
			t.Fatalf("workspace request %d = %d", i+1, got)
		}
	}
	if got := gitBudgetRequest(broker, repoURL); got != http.StatusTooManyRequests {
		t.Fatalf("workspace request 3 = %d, want 429", got)
	}
	if gh.calls != 2 {
		t.Fatalf("workspace credential mints = %d, want 2", gh.calls)
	}

	disabled, disabledURL, disabledGH := gitBudgetHarness(t, 0, 0)
	for i := 0; i < 5; i++ {
		if got := gitBudgetRequest(disabled, disabledURL); got != http.StatusOK {
			t.Fatalf("disabled request %d = %d", i+1, got)
		}
	}
	if disabledGH.calls != 5 {
		t.Fatalf("disabled credential mints = %d, want 5", disabledGH.calls)
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
		Pods: credentialPodResolver{pods: map[string]SessionPod{"10.0.0.1": {Name: "sbx-a-0", UID: "uid-a", Labels: credentialLabels(t, "tea-a")}}},
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

func TestValidateReceivePackAcceptsBoundedShallowMetadata(t *testing.T) {
	pkt := func(line string) string {
		return fmt.Sprintf("%04x%s", len(line)+4, line)
	}
	old := strings.Repeat("1", 40)
	next := strings.Repeat("2", 40)
	shallow := "shallow " + strings.Repeat("a", 40) + "\n"
	command := old + " " + next + " refs/heads/bex-agent/task-1\x00report-status\n"
	body := pkt(shallow) + pkt(command) + "0000PACK"

	validated, err := validateReceivePack(strings.NewReader(body), "bex-agent/task-1")
	if err != nil {
		t.Fatalf("valid shallow push rejected: %v", err)
	}
	got, err := io.ReadAll(validated)
	if err != nil || string(got) != body {
		t.Fatalf("validated body changed: err=%v got=%q", err, got)
	}

	for name, invalid := range map[string]string{
		"invalid oid":         pkt("shallow not-an-oid\n") + pkt(command) + "0000",
		"metadata after ref":  pkt(command) + pkt(shallow) + "0000",
		"metadata capability": pkt("shallow "+strings.Repeat("a", 40)+"\x00cap\n") + pkt(command) + "0000",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := validateReceivePack(strings.NewReader(invalid), "bex-agent/task-1"); !errors.Is(err, agentsession.ErrForbidden) {
				t.Fatalf("error=%v, want forbidden", err)
			}
		})
	}
}

// TestValidateReceivePackRejectsWhitespacePaddedShallow is the codex-security
// 2026-08 F3 shape regression: the shallow-line check must be the exact
// single-space git wire form, not a strings.Fields two-field test — "shallow"
// plus ~65,400 spaces plus a valid oid passed every prior check at the maximum
// packet size, letting padding reach the replay buffer.
func TestValidateReceivePackRejectsWhitespacePaddedShallow(t *testing.T) {
	pkt := func(line string) string {
		return fmt.Sprintf("%04x%s", len(line)+4, line)
	}
	old := strings.Repeat("1", 40)
	next := strings.Repeat("2", 40)
	command := old + " " + next + " refs/heads/bex-agent/task-1\x00report-status\n"
	for name, padded := range map[string]string{
		// Multiple interior spaces: a two-field line to strings.Fields, malformed on the wire.
		"interior spaces": pkt("shallow  "+strings.Repeat("a", 40)+"\n") + pkt(command) + "0000",
		// Maximal padding sized to the per-packet cap — the shape the buffer
		// budget must never see.
		"padding to packet cap": pkt("shallow "+strings.Repeat(" ", maxPacketBytes-4-8-41-len("\n"))+strings.Repeat("a", 40)+"\n") + pkt(command) + "0000",
		// Tab separator — same class.
		"tab separator": pkt("shallow\t"+strings.Repeat("a", 40)+"\n") + pkt(command) + "0000",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := validateReceivePack(strings.NewReader(padded), "bex-agent/task-1"); !errors.Is(err, agentsession.ErrForbidden) {
				t.Fatalf("padded shallow line accepted: err=%v, want forbidden", err)
			}
		})
	}
}

// TestValidateReceivePackBoundsReplayBuffer is the codex-security 2026-08 F3
// budget regression: per-packet (64 KiB) and per-kind (1024 shallows) caps
// bound nothing until their product is checked, and the product is ~64 MiB of
// resident buffer in the shared gateway. The accumulated prefix must fail
// closed at maxReceivePackPrefixBytes even when every individual line is valid.
func TestValidateReceivePackBoundsReplayBuffer(t *testing.T) {
	pkt := func(line string) string {
		return fmt.Sprintf("%04x%s", len(line)+4, line)
	}
	var body strings.Builder
	// Distinct oids so no two packets are identical (nothing dedupes; this is
	// purely about volume). ~65 bytes/packet × ~16k packets ≈ 1 MiB budget.
	for i := 0; body.Len() <= maxReceivePackPrefixBytes+4096; i++ {
		oid := fmt.Sprintf("%040x", i)
		body.WriteString(pkt("shallow " + oid + "\n"))
	}
	// Every line so far is individually valid: if the budget check were
	// missing, validation would keep accepting until the 1024-shallow cap —
	// but 1024 × 64 KiB of *padded* lines is the OOM shape this bounds.
	if _, err := validateReceivePack(strings.NewReader(body.String()), "bex-agent/task-1"); !errors.Is(err, agentsession.ErrForbidden) {
		t.Fatal("volume of individually-valid shallow lines exceeded the byte budget but was accepted")
	}
	// Sanity: the budget is what fired, not the shallow-count cap.
	if strings.Count(body.String(), "shallow") <= maxShallowBoundaries {
		t.Fatalf("test bug: only %d shallow lines built; need > %d to prove the byte budget fires",
			strings.Count(body.String(), "shallow"), maxShallowBoundaries)
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

// upstreamRequest is one exchange as the recording upstream saw it.
type upstreamRequest struct {
	encoding string
	body     []byte
}

// recordingUpstream captures each proxied request's Content-Encoding and body
// and answers a minimal git response.
func recordingUpstream(got *[]upstreamRequest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*got = append(*got, upstreamRequest{encoding: r.Header.Get("Content-Encoding"), body: body})
		_, _ = io.WriteString(w, "0000")
	}
}

func gzipBytes(t *testing.T, plain string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(plain)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// git gzips any smart-HTTP RPC body over 1 KiB (remote-curl.c post_rpc), so the
// pack-fetch POST of a many-ref repo arrives compressed. The proxy must forward
// the encoding header WITH the compressed bytes — stripping it while passing the
// body made GitHub 400 every clone of bex-co/bex-security (887 refs) as an
// opaque 502 (w5/m82, session ags-da9l9e5a801s739cb2ig).
func TestGitProxyForwardsGzipEncodedUploadPack(t *testing.T) {
	negotiation := "0011command=fetch0001000fno-progress" + strings.Repeat("0032want 1111111111111111111111111111111111111111\n", 40) + "0009done\n0000"
	compressed := gzipBytes(t, negotiation)

	var got []upstreamRequest
	broker, repoURL, _ := gitProxyHarness(t, recordingUpstream(&got))

	post := func(body []byte, gzipped bool) int {
		req := httptest.NewRequest(http.MethodPost, repoURL+"/git-upload-pack", bytes.NewReader(body))
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
		if gzipped {
			req.Header.Set("Content-Encoding", "gzip")
		}
		rec := httptest.NewRecorder()
		broker.Handler().ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post(compressed, true); code != http.StatusOK {
		t.Fatalf("gzipped upload-pack status=%d", code)
	}
	if len(got) != 1 || got[0].encoding != "gzip" || !bytes.Equal(got[0].body, compressed) {
		t.Fatalf("upstream saw encoding=%q body-intact=%t; the compressed body must travel with its header",
			got[0].encoding, bytes.Equal(got[0].body, compressed))
	}
	if code := post([]byte(negotiation), false); code != http.StatusOK {
		t.Fatalf("plain upload-pack status=%d", code)
	}
	if len(got) != 2 || got[1].encoding != "" || !bytes.Equal(got[1].body, []byte(negotiation)) {
		t.Fatalf("plain request changed: encoding=%q intact=%t", got[1].encoding, bytes.Equal(got[1].body, []byte(negotiation)))
	}
}

// A gzipped receive-pack body (any push in git's 1 KiB–postBuffer gzip band) is
// validated against the plaintext pkt-lines and forwarded DECOMPRESSED, so
// branch confinement keeps working and the upstream never sees an encoding
// header the body no longer matches.
func TestGitProxyReceivePackAcceptsGzippedBody(t *testing.T) {
	plainPush := func(ref string) string {
		old := strings.Repeat("1", 40)
		next := strings.Repeat("2", 40)
		line := old + " " + next + " " + ref + "\x00report-status\n"
		return fmt.Sprintf("%04x%s0000PACK", len(line)+4, line)
	}

	var got []upstreamRequest
	broker, repoURL, _ := gitProxyHarness(t, recordingUpstream(&got))

	push := func(body []byte, encoding string) int {
		req := httptest.NewRequest(http.MethodPost, repoURL+"/git-receive-pack", bytes.NewReader(body))
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
		if encoding != "" {
			req.Header.Set("Content-Encoding", encoding)
		}
		rec := httptest.NewRecorder()
		broker.Handler().ServeHTTP(rec, req)
		return rec.Code
	}

	bound := plainPush("refs/heads/bex-agent/task-1")
	if code := push(gzipBytes(t, bound), "gzip"); code != http.StatusOK {
		t.Fatalf("gzipped bound-branch push status=%d", code)
	}
	if len(got) != 1 || got[0].encoding != "" || !bytes.Equal(got[0].body, []byte(bound)) {
		t.Fatalf("upstream must receive the decompressed body with no encoding header: encoding=%q decompressed=%t",
			got[0].encoding, bytes.Equal(got[0].body, []byte(bound)))
	}
	// Branch confinement is enforced on the DECOMPRESSED stream: a cross-branch
	// push cannot smuggle itself past validation inside gzip.
	if code := push(gzipBytes(t, plainPush("refs/heads/main")), "gzip"); code != http.StatusForbidden {
		t.Fatalf("gzipped cross-branch push status=%d, want 403", code)
	}
	// A body that claims gzip but is not fails closed before validation.
	if code := push([]byte(bound), "gzip"); code != http.StatusBadRequest {
		t.Fatalf("corrupt gzip status=%d, want 400", code)
	}
	if len(got) != 1 {
		t.Fatalf("rejected pushes reached upstream: %d calls", len(got))
	}
}

// The forwarded-header set is a CONTRACT, not a best-effort copy: exactly the
// allowlisted headers (plus Content-Encoding for upload-pack) cross the trust
// boundary, the injected installation-token auth cannot be overridden by the
// sandbox, and hostile extras never reach the forge. A future hardening pass
// that "narrows" the list again (the e91d5be8 regression class) or a loosening
// that reflects sandbox headers both fail here.
func TestGitProxyForwardsOnlyAllowlistedHeaders(t *testing.T) {
	var gotHeaders http.Header
	var gotAuth string
	broker, repoURL, _ := gitProxyHarness(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, "0000")
	})

	body := gzipBytes(t, "0011command=fetch0009done\n0000")
	req := httptest.NewRequest(http.MethodPost, repoURL+"/git-upload-pack", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
	req.Header.Set("Git-Protocol", "version=2")
	req.Header.Set("Content-Encoding", "gzip")
	// Hostile extras a compromised sandbox could attach.
	req.Header.Set("Authorization", "Bearer sandbox-evil")
	req.Header.Set("Cookie", "session=stolen")
	req.Header.Set("X-Evil", "1")
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	rec := httptest.NewRecorder()
	broker.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") || strings.Contains(gotAuth, "sandbox-evil") {
		t.Fatalf("upstream Authorization must be the injected installation token, got %q", gotAuth)
	}
	for _, name := range []string{"Cookie", "X-Evil", "X-Forwarded-For"} {
		if gotHeaders.Get(name) != "" {
			t.Fatalf("hostile header %s crossed the trust boundary: %q", name, gotHeaders.Get(name))
		}
	}
	for name, want := range map[string]string{
		"Accept": "application/x-git-upload-pack-result", "Content-Type": "application/x-git-upload-pack-request",
		"Git-Protocol": "version=2", "Content-Encoding": "gzip", "User-Agent": "bex-agent-git-proxy/1",
	} {
		if gotHeaders.Get(name) != want {
			t.Fatalf("upstream %s = %q, want %q", name, gotHeaders.Get(name), want)
		}
	}
}

// Every admitted-then-failed upstream exchange logs an attributable reason and
// counts a bounded-cause metric — the ags-da9l9e5a801s739cb2ig 502 was invisible
// in gateway logs and needed a DB audit-trail reconstruction (w5/m82 t003).
func TestGitProxyUpstreamRefusalLogsAndCounts(t *testing.T) {
	broker, repoURL, _ := gitProxyHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	})
	registry := prometheus.NewRegistry()
	broker.Metrics = sshgateway.NewMetrics(registry)

	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)

	req := httptest.NewRequest(http.MethodGet, repoURL+"/info/refs?service=git-upload-pack", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	broker.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d, want 502", rec.Code)
	}
	// The sandbox keeps seeing only the generic refusal — the upstream error
	// body must not be reflected across the trust boundary.
	if body := rec.Body.String(); strings.Contains(body, "Bad Request") || !strings.Contains(body, "git upstream refused request") {
		t.Fatalf("sandbox-visible 502 body reflected upstream content: %q", body)
	}
	line := logged.String()
	for _, want := range []string{"refused", "session=ags-one", "repo=octo/repo", "status 400"} {
		if !strings.Contains(line, want) {
			t.Fatalf("log line missing %q: %q", want, line)
		}
	}
	if strings.Contains(line, "ghs_gateway_secret") || strings.Contains(line, "Authorization") {
		t.Fatalf("credential material leaked into the log: %q", line)
	}
	if v := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_git_proxy_upstream_failures_total", map[string]string{"cause": "refused"}); v != 1 {
		t.Fatalf(`git_proxy_upstream_failures_total{cause="refused"} = %v, want 1`, v)
	}

	// An unreachable upstream is the other silent-502 class: same generic
	// sandbox answer, distinct bounded cause.
	broker.UpstreamOrigin = "http://127.0.0.1:1"
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, repoURL+"/info/refs?service=git-upload-pack", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	broker.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unreachable upstream status=%d, want 502", rec.Code)
	}
	if v := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_git_proxy_upstream_failures_total", map[string]string{"cause": "network"}); v != 1 {
		t.Fatalf(`git_proxy_upstream_failures_total{cause="network"} = %v, want 1`, v)
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
