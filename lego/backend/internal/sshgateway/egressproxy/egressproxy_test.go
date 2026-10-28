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

package egressproxy_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentcred"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/egressproxy"
)

const (
	namespace = "tea-aaaaaaaaaaaaaaaaaaaa-sandbox"
	sessionID = "ags-one"
)

// --- fakes ---------------------------------------------------------------

type fakePods struct {
	pod agentcred.SessionPod
	err error
}

func (f fakePods) ResolveSessionPod(context.Context, string, string) (agentcred.SessionPod, error) {
	return f.pod, f.err
}

func sandboxPod(session string) agentcred.SessionPod {
	return agentcred.SessionPod{Name: "sbx-1-0", UID: "uid-1", Labels: map[string]string{
		agentsession.LabelWorkspace: "tea-aaaaaaaaaaaaaaaaaaaa",
		agentsession.LabelRegime:    agentsession.RegimeSandbox,
		agentsession.LabelSession:   session,
	}}
}

type fakeGranter struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (g *fakeGranter) Grant(_ context.Context, req agentsession.ModelMintRequest) (agentsession.EgressGrantResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if g.err != nil {
		return agentsession.EgressGrantResponse{}, g.err
	}
	return agentsession.EgressGrantResponse{Allowed: true, SessionID: req.SessionID}, nil
}

type fakeResolver map[string][]string

func (r fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	raw, ok := r[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	out := make([]netip.Addr, 0, len(raw))
	for _, value := range raw {
		out = append(out, netip.MustParseAddr(value))
	}
	return out, nil
}

var resolver = fakeResolver{
	"example.com.":                       {"93.184.215.14"},
	"plain.example.com.":                 {"93.184.215.15"},
	"mirror.example.net.":                {"160.79.104.10"}, // shares a provider address
	"internal.example.com.":              {"10.1.2.3"},
	"rebind.example.com.":                {"93.184.215.16", "169.254.169.254"},
	"api.anthropic.com.":                 {"160.79.104.10"},
	"api.openai.com.":                    {"162.159.140.245"},
	"generativelanguage.googleapis.com.": {"142.250.74.106"},
	"openrouter.ai.":                     {"104.18.2.115", "2606:4700::6812:273"},
}

type auditRecorder struct {
	mu     sync.Mutex
	events []core.AuditEvent
}

func (a *auditRecorder) Record(_ context.Context, event core.AuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, event)
	return nil
}

// harness wires a Broker whose every dial lands on one local upstream, and
// records the address the proxy asked to dial.
type harness struct {
	broker  *egressproxy.Broker
	proxy   *httptest.Server
	granter *fakeGranter
	audit   *auditRecorder

	mu     sync.Mutex
	dialed []string
}

func newHarness(t *testing.T, upstream string, configure func(*egressproxy.Broker)) *harness {
	t.Helper()
	h := &harness{granter: &fakeGranter{}, audit: &auditRecorder{}}
	h.broker = &egressproxy.Broker{
		Pods:     fakePods{pod: sandboxPod(sessionID)},
		API:      h.granter,
		Audit:    h.audit,
		Metrics:  sshgateway.NewMetrics(prometheus.NewRegistry()),
		Resolver: resolver,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			h.mu.Lock()
			h.dialed = append(h.dialed, address)
			h.mu.Unlock()
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, upstream)
		},
	}
	if configure != nil {
		configure(h.broker)
	}
	h.proxy = httptest.NewServer(h.broker.Handler())
	t.Cleanup(h.proxy.Close)
	return h
}

func (h *harness) dials() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.dialed...)
}

func proxyAuth() string {
	encode := func(v string) string { return base64.RawURLEncoding.EncodeToString([]byte(v)) }
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(encode(namespace)+":"+encode(sessionID)))
}

// connect sends a raw CONNECT and returns the status and the open connection.
func (h *harness) connect(t *testing.T, target, auth string) (int, net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", h.proxy.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	request := "CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n"
	if auth != "" {
		request += "Proxy-Authorization: " + auth + "\r\n"
	}
	if _, err := io.WriteString(conn, request+"\r\n"); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("CONNECT %s: %v", target, err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
	}
	return response.StatusCode, conn, reader
}

func tlsUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "hello %s", r.Host)
	}))
	t.Cleanup(server.Close)
	return server
}

// proxiedClient is an ordinary HTTP client configured like a sandbox tool
// reading HTTPS_PROXY (userinfo = the session claim).
func proxiedClient(t *testing.T, h *harness, upstream *httptest.Server) *http.Client {
	t.Helper()
	proxyURL, err := url.Parse(h.proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	built, err := agentsession.EgressProxyURL(proxyURL.String(), namespace, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	withClaim, _ := url.Parse(built)
	transport := &http.Transport{Proxy: http.ProxyURL(withClaim)}
	if upstream != nil {
		transport.TLSClientConfig = upstream.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

// --- tests -----------------------------------------------------------------

func TestConnectToAnAllowedHostTunnelsToTheResolvedAddress(t *testing.T) {
	upstream := tlsUpstream(t)
	h := newHarness(t, upstream.Listener.Addr().String(), nil)
	response, err := proxiedClient(t, h, upstream).Get("https://example.com/")
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "hello example.com" {
		t.Fatalf("response = %d %q", response.StatusCode, body)
	}
	if got := h.dials(); len(got) != 1 || got[0] != "93.184.215.14:443" {
		t.Fatalf("dialed %v, want the once-resolved address 93.184.215.14:443", got)
	}
	h.broker.FlushAudit()
	if len(h.audit.events) != 1 || h.audit.events[0].Verb != agentsession.AuditVerbEgressUsage ||
		!strings.HasPrefix(h.audit.events[0].TargetName, "connections=1 bytes=") || strings.Contains(h.audit.events[0].TargetName, "example.com") {
		t.Fatalf("usage audit = %+v, want one content-free connection count", h.audit.events)
	}
}

func TestPlainHTTPIsForwardedWithTheURLHostAndNoProxyClaim(t *testing.T) {
	var seen *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Clone(context.Background())
		_, _ = io.WriteString(w, "plain ok")
	}))
	t.Cleanup(upstream.Close)
	h := newHarness(t, upstream.Listener.Addr().String(), nil)
	response, err := proxiedClient(t, h, nil).Get("http://plain.example.com/path?q=1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "plain ok" {
		t.Fatalf("response = %d %q", response.StatusCode, body)
	}
	if seen.Host != "plain.example.com" || seen.URL.RequestURI() != "/path?q=1" || seen.Header.Get("Proxy-Authorization") != "" {
		t.Fatalf("upstream saw host=%q uri=%q proxy-auth=%q", seen.Host, seen.URL.RequestURI(), seen.Header.Get("Proxy-Authorization"))
	}
	if got := h.dials(); len(got) != 1 || got[0] != "93.184.215.15:80" {
		t.Fatalf("dialed %v", got)
	}
}

func TestModelProvidersAreRefusedByNameAndSubdomain(t *testing.T) {
	h := newHarness(t, "127.0.0.1:1", nil)
	for _, target := range []string{
		"api.anthropic.com:443", "API.OpenAI.com.:443", "generativelanguage.googleapis.com:443",
		"openrouter.ai:443", "eu.api.anthropic.com:443",
	} {
		if status, _, _ := h.connect(t, target, proxyAuth()); status != http.StatusForbidden {
			t.Errorf("CONNECT %s = %d, want 403", target, status)
		}
	}
	upstream := tlsUpstream(t)
	plain := newHarness(t, upstream.Listener.Addr().String(), nil)
	response, err := proxiedClient(t, plain, nil).Get("http://openrouter.ai/api/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("plain HTTP to a provider = %d, want 403", response.StatusCode)
	}
	if len(h.dials())+len(plain.dials()) != 0 {
		t.Fatalf("a refused provider target was dialed: %v %v", h.dials(), plain.dials())
	}
}

// Provider hosts are refused by name (and by TLS SNI in the tunnel test), not
// by resolved IP: providers such as openrouter.ai sit on shared CDN anycast
// addresses, so an IP denylist would over-block unrelated sites. The platform
// key never enters the sandbox, so a direct provider connection could only
// ever use a tenant's own key.
func TestUnrelatedHostsSharingAProviderAddressAreNotRefused(t *testing.T) {
	h := newHarness(t, "127.0.0.1:1", nil)
	if status, _, _ := h.connect(t, "mirror.example.net:443", proxyAuth()); status == http.StatusForbidden {
		t.Fatalf("CONNECT to a non-provider name sharing a provider address = 403; want it relayed")
	}
	if len(h.dials()) == 0 {
		t.Fatalf("the non-provider host was never dialed")
	}
}

func TestPrivateClusterAndMetadataTargetsAreRefused(t *testing.T) {
	h := newHarness(t, "127.0.0.1:1", nil)
	for _, target := range []string{
		"10.0.0.5:443", "127.0.0.1:443", "169.254.169.254:80", "100.64.0.1:443", "[::1]:443",
		"[fd00::1]:443", "[::ffff:10.0.0.1]:443", "192.168.1.1:80",
		"internal.example.com:443", // public name, private answer
		"rebind.example.com:443",   // mixed answer with the metadata address
		"localhost:443", "bex-api.bex-system.svc:443", "bex-api.bex-system.svc.cluster.local:443", "bex-api:443",
	} {
		if status, _, _ := h.connect(t, target, proxyAuth()); status != http.StatusForbidden {
			t.Errorf("CONNECT %s = %d, want 403", target, status)
		}
	}
	h.broker.DenyPrefixes = []netip.Prefix{netip.MustParsePrefix("93.184.215.0/24")}
	if status, _, _ := h.connect(t, "example.com:443", proxyAuth()); status != http.StatusForbidden {
		t.Errorf("CONNECT into a configured deny prefix = %d, want 403", status)
	}
	if got := h.dials(); len(got) != 0 {
		t.Fatalf("a refused private target was dialed: %v", got)
	}
}

func TestOnlyAllowedPortsAreRelayed(t *testing.T) {
	h := newHarness(t, "127.0.0.1:1", nil)
	for _, target := range []string{"example.com:22", "example.com:8443", "example.com:25", "example.com:0"} {
		if status, _, _ := h.connect(t, target, proxyAuth()); status != http.StatusForbidden {
			t.Errorf("CONNECT %s = %d, want 403", target, status)
		}
	}
	upstream := tlsUpstream(t)
	custom := newHarness(t, upstream.Listener.Addr().String(), func(b *egressproxy.Broker) { b.Ports = []int{8443} })
	if status, _, _ := custom.connect(t, "example.com:443", proxyAuth()); status != http.StatusForbidden {
		t.Errorf("CONNECT :443 with Ports=[8443] = %d, want 403", status)
	}
	if status, _, _ := custom.connect(t, "example.com:8443", proxyAuth()); status != http.StatusOK {
		t.Errorf("CONNECT :8443 with Ports=[8443] = %d, want 200", status)
	}
}

func TestNonFullTierSessionsAreRefusedAndTheDecisionIsCached(t *testing.T) {
	h := newHarness(t, "127.0.0.1:1", nil)
	h.granter.err = agentsession.ErrForbidden
	for range 3 {
		if status, _, _ := h.connect(t, "example.com:443", proxyAuth()); status != http.StatusForbidden {
			t.Fatalf("non-full CONNECT = %d, want 403", status)
		}
	}
	if h.granter.calls != 1 {
		t.Fatalf("grant asked %d times, want 1 (refusal cached)", h.granter.calls)
	}
	// An unreachable bex-api fails closed and is not cached.
	unavailable := newHarness(t, "127.0.0.1:1", nil)
	unavailable.granter.err = errors.New("broker down")
	for range 2 {
		if status, _, _ := unavailable.connect(t, "example.com:443", proxyAuth()); status != http.StatusBadGateway {
			t.Fatalf("grant outage CONNECT = %d, want 502", status)
		}
	}
	if unavailable.granter.calls != 2 {
		t.Fatalf("outage grant asked %d times, want 2 (never cached)", unavailable.granter.calls)
	}
}

func TestTheSourcePodMustBeTheClaimedSessionSandbox(t *testing.T) {
	for name, pods := range map[string]fakePods{
		"unknown source IP":   {err: agentsession.ErrForbidden},
		"sibling session pod": {pod: sandboxPod("ags-other")},
		"non-sandbox pod": {pod: agentcred.SessionPod{Name: "x", UID: "u", Labels: map[string]string{
			agentsession.LabelWorkspace: "tea-aaaaaaaaaaaaaaaaaaaa", agentsession.LabelSession: sessionID,
		}}},
	} {
		h := newHarness(t, "127.0.0.1:1", func(b *egressproxy.Broker) { b.Pods = pods })
		if status, _, _ := h.connect(t, "example.com:443", proxyAuth()); status != http.StatusForbidden {
			t.Errorf("%s: CONNECT = %d, want 403", name, status)
		}
		if h.granter.calls != 0 {
			t.Errorf("%s: grant asked before the Pod binding passed", name)
		}
	}
	h := newHarness(t, "127.0.0.1:1", nil)
	if status, _, _ := h.connect(t, "example.com:443", ""); status != http.StatusProxyAuthRequired {
		t.Fatalf("CONNECT without the session claim = %d, want 407", status)
	}
}

func TestTunnelsMustOpenWithAMatchingNonProviderClientHello(t *testing.T) {
	upstream := tlsUpstream(t)
	h := newHarness(t, upstream.Listener.Addr().String(), nil)
	for _, serverName := range []string{"api.anthropic.com", "other.example.org"} {
		status, conn, _ := h.connect(t, "example.com:443", proxyAuth())
		if status != http.StatusOK {
			t.Fatalf("CONNECT = %d", status)
		}
		client := tls.Client(conn, &tls.Config{ServerName: serverName, InsecureSkipVerify: true}) //nolint:gosec // the test asserts the proxy cuts the tunnel before any certificate matters
		_ = client.SetDeadline(time.Now().Add(5 * time.Second))
		if err := client.Handshake(); err == nil {
			t.Errorf("SNI %q through a CONNECT to example.com completed a handshake", serverName)
		}
	}
	// Plain bytes in a tunnel are refused too (fronting over cleartext).
	status, conn, reader := h.connect(t, "example.com:443", proxyAuth())
	if status != http.StatusOK {
		t.Fatalf("CONNECT = %d", status)
	}
	_, _ = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: api.anthropic.com\r\n\r\n")
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if n, _ := reader.Read(make([]byte, 64)); n != 0 {
		t.Fatal("a cleartext tunnel relayed a response")
	}
}

func TestBudgetsAndConcurrencyAreEnforced(t *testing.T) {
	upstream := tlsUpstream(t)
	t.Run("connections per session", func(t *testing.T) {
		h := newHarness(t, upstream.Listener.Addr().String(), func(b *egressproxy.Broker) { b.MaxConnectionsPerSession = 1 })
		if status, _, _ := h.connect(t, "example.com:443", proxyAuth()); status != http.StatusOK {
			t.Fatalf("first CONNECT = %d", status)
		}
		if status, _, _ := h.connect(t, "example.com:443", proxyAuth()); status != http.StatusTooManyRequests {
			t.Fatalf("over-budget CONNECT = %d, want 429", status)
		}
	})
	t.Run("connections per workspace", func(t *testing.T) {
		h := newHarness(t, upstream.Listener.Addr().String(), func(b *egressproxy.Broker) { b.MaxConnectionsPerWorkspace = 1 })
		if status, _, _ := h.connect(t, "example.com:443", proxyAuth()); status != http.StatusOK {
			t.Fatalf("first CONNECT = %d", status)
		}
		if status, _, _ := h.connect(t, "example.com:443", proxyAuth()); status != http.StatusTooManyRequests {
			t.Fatalf("over-budget CONNECT = %d, want 429", status)
		}
	})
	t.Run("concurrent tunnels per pod", func(t *testing.T) {
		h := newHarness(t, upstream.Listener.Addr().String(), func(b *egressproxy.Broker) {
			b.Limits = sshgateway.NewSessionLimiter(10, 1)
		})
		status, held, _ := h.connect(t, "example.com:443", proxyAuth())
		if status != http.StatusOK {
			t.Fatalf("first CONNECT = %d", status)
		}
		if status, _, _ := h.connect(t, "example.com:443", proxyAuth()); status != http.StatusTooManyRequests {
			t.Fatalf("second concurrent CONNECT = %d, want 429", status)
		}
		_ = held.Close()
		deadline := time.Now().Add(5 * time.Second)
		for {
			status, _, _ := h.connect(t, "example.com:443", proxyAuth())
			if status == http.StatusOK {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("slot never released after the tunnel closed: %d", status)
			}
			time.Sleep(20 * time.Millisecond)
		}
	})
	t.Run("bytes per session", func(t *testing.T) {
		h := newHarness(t, upstream.Listener.Addr().String(), func(b *egressproxy.Broker) { b.MaxBytesPerSession = 64 })
		client := proxiedClient(t, h, upstream)
		if _, err := client.Get("https://example.com/"); err == nil {
			t.Fatal("a TLS exchange larger than the byte budget completed")
		}
		if status, _, _ := h.connect(t, "example.com:443", proxyAuth()); status != http.StatusTooManyRequests {
			t.Fatalf("CONNECT after the byte budget = %d, want 429", status)
		}
	})
}

func TestIdleTunnelsAreClosed(t *testing.T) {
	upstream := tlsUpstream(t)
	h := newHarness(t, upstream.Listener.Addr().String(), func(b *egressproxy.Broker) { b.IdleTimeout = 200 * time.Millisecond })
	status, conn, _ := h.connect(t, "example.com:443", proxyAuth())
	if status != http.StatusOK {
		t.Fatalf("CONNECT = %d", status)
	}
	// Send nothing: the ClientHello wait (10s) is longer than the test, so
	// complete a handshake, then go silent.
	client := tls.Client(conn, &tls.Config{ServerName: "example.com", InsecureSkipVerify: true}) //nolint:gosec // test upstream uses a self-signed certificate
	if err := client.Handshake(); err != nil {
		t.Fatalf("handshake: %v", err)
	}
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	start := time.Now()
	_, err := client.Read(make([]byte, 1))
	if err == nil || time.Since(start) > 4*time.Second {
		t.Fatalf("idle tunnel stayed open (err=%v after %s)", err, time.Since(start))
	}
}

func TestOriginFormRequestsAreNotProxied(t *testing.T) {
	h := newHarness(t, "127.0.0.1:1", nil)
	response, err := http.Get(h.proxy.URL + "/healthz")
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("healthz = %v %v", response, err)
	}
	response, err = http.Get(h.proxy.URL + "/anything")
	if err != nil || response.StatusCode != http.StatusBadRequest {
		t.Fatalf("origin-form request = %v %v, want 400", response, err)
	}
}
