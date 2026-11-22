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

package sandboxbroker

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type e2eStore struct {
	mu      sync.Mutex
	records map[string]store.SandboxRecord
	members map[string]store.SandboxGroupMember
	groups  map[string]store.SandboxGroup
	creds   map[string][]store.SandboxCredential
}

func (s *e2eStore) GetSandboxRecord(_ context.Context, ws, id string) (store.SandboxRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[ws+"|"+id]
	if !ok {
		return store.SandboxRecord{}, store.ErrNotFound
	}
	return r, nil
}

func (s *e2eStore) GetSandboxGroupMember(_ context.Context, ws, id string) (store.SandboxGroupMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[ws+"|"+id]
	if !ok {
		return store.SandboxGroupMember{}, store.ErrNotFound
	}
	return m, nil
}

func (s *e2eStore) SandboxGroup(_ context.Context, ws, id string) (store.SandboxGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[ws+"|"+id]
	if !ok {
		return store.SandboxGroup{}, store.ErrNotFound
	}
	return g, nil
}

func (s *e2eStore) ListSandboxGroupCredentials(_ context.Context, ws, gid string) ([]store.SandboxCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.SandboxCredential(nil), s.creds[ws+"|"+gid]...), nil
}

type e2eKV struct {
	mu   sync.Mutex
	data map[string]map[string]string
}

func (k *e2eKV) Put(_ context.Context, p string, v map[string]string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.data == nil {
		k.data = map[string]map[string]string{}
	}
	cp := make(map[string]string, len(v))
	for kk, vv := range v {
		cp[kk] = vv
	}
	k.data[p] = cp
	return nil
}

func (k *e2eKV) Get(_ context.Context, p string) (map[string]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.data[p], nil
}

func (k *e2eKV) Delete(_ context.Context, p string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.data, p)
	return nil
}

func (k *e2eKV) List(_ context.Context, _ string) ([]string, error) { return nil, nil }

type e2eNonce struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func (n *e2eNonce) ClaimShellNonce(_ context.Context, nonce string, _ time.Time) (bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.seen == nil {
		n.seen = map[string]struct{}{}
	}
	if _, exists := n.seen[nonce]; exists {
		return false, nil
	}
	n.seen[nonce] = struct{}{}
	return true, nil
}

type capturedUpstreamReq struct {
	Host   string            `json:"host"`
	Method string            `json:"method"`
	Path   string            `json:"path"`
	Query  string            `json:"query"`
	Header map[string]string `json:"header"`
}

func makeBrokerCA(t *testing.T) (*CA, []byte) {
	t.Helper()
	now := time.Now().UTC()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(200),
		Subject:               pkix.Name{CommonName: "bex-sandbox-broker-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	ca, err := NewCA(certPEM, keyPEM, CAOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return ca, certPEM
}

func TestSandboxBrokerLoopbackE2E(t *testing.T) {
	// Secrets used across the entire test; at the end we assert ZERO occurrences
	// in logs, error bodies, and Prometheus metrics output.
	const (
		secretHeader   = "fixture-header-alpha"
		secretBasic    = "fixture-basic-bravo"
		secretCookie   = "fixture-cookie-charlie"
		secretQuery    = "fixture-query-delta"
		secretRotated  = "fixture-rotated-echo"
		secretRestored = "fixture-restored-foxtrot"
		secretOverlapA = "fixture-overlap-alpha"
		secretOverlapZ = "fixture-overlap-zulu"
	)
	allSecrets := []string{
		secretHeader, secretBasic, secretCookie, secretQuery,
		secretRotated, secretRestored, secretOverlapA, secretOverlapZ,
	}

	// 1. Two loopback HTTPS upstream servers:
	//    - upstreamA serves api.example.com, uncred.example.com, overlap.example.com
	//    - upstreamB serves redirect-target.example.com (receives 302 target requests)
	var redirectTargetHits atomic.Int32
	var redirectTargetSawSecret atomic.Bool

	echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/redirect" {
			http.Redirect(w, r, "https://redirect-target.example.com/landing", http.StatusFound)
			return
		}
		hdr := make(map[string]string, len(r.Header))
		for k := range r.Header {
			hdr[k] = strings.Join(r.Header.Values(k), "; ")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(capturedUpstreamReq{
			Host:   r.Host,
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Header: hdr,
		})
	})

	upstreams, upstreamRoots := makeUpstreamRootWithServers(t, echoHandler, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHits.Add(1)
		raw := r.URL.RawQuery + " " + r.Header.Get("Authorization") + " " + r.Header.Get("Cookie") + " " + r.Header.Get("X-Api-Key")
		for _, s := range allSecrets {
			if strings.Contains(raw, s) {
				redirectTargetSawSecret.Store(true)
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "landed")
	}))
	upstreamA := upstreams.srvA
	upstreamB := upstreams.srvB

	// 2. Control-plane store + OpenBao fake + real HMAC BrokerLookupHandler on httptest.
	st := &e2eStore{
		records: map[string]store.SandboxRecord{},
		members: map[string]store.SandboxGroupMember{},
		groups:  map[string]store.SandboxGroup{"tea-1|sbg-1": {ID: "sbg-1", WorkspaceID: "tea-1"}},
		creds:   map[string][]store.SandboxCredential{},
	}
	setPolicy := func(policy *sandbox.NetworkPolicy) {
		view, err := json.Marshal(sandbox.Sandbox{ID: "sbx-1", SandboxGroupID: "sbg-1", NetworkPolicy: policy})
		if err != nil {
			t.Fatal(err)
		}
		st.mu.Lock()
		st.members["tea-1|sbx-1"] = store.SandboxGroupMember{GroupID: "sbg-1"}
		st.records["tea-1|sbx-1"] = store.SandboxRecord{WorkspaceID: "tea-1", SandboxID: "sbx-1", View: view}
		st.mu.Unlock()
	}
	setPolicy(&sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyAllowAll})

	kv := &e2eKV{}
	vault := sandbox.NewCredentialVault(kv)
	putSecret := func(credID, ref, val string) {
		if err := vault.Put(context.Background(), "tea-1", "sbg-1", credID, ref, val); err != nil {
			t.Fatal(err)
		}
	}
	putSecret("c-header", "ref-h", secretHeader)
	putSecret("c-basic", "ref-b", secretBasic)
	putSecret("c-cookie", "ref-c", secretCookie)
	putSecret("c-query", "ref-q", secretQuery)
	putSecret("c-ovl-a", "ref-oa", secretOverlapA)
	putSecret("c-ovl-z", "ref-oz", secretOverlapZ)
	putSecret("c-live", "ref-l1", secretHeader)
	putSecret("c-live", "ref-l2", secretRotated)
	putSecret("c-live", "ref-l3", secretRestored)

	st.creds["tea-1|sbg-1"] = []store.SandboxCredential{
		{
			ID: "c-header", CurrentSecretRef: "ref-h", CurrentVersion: store.SandboxCredentialVersion{ID: 1, Fingerprint: "fp-h"},
			Config: store.SandboxCredentialConfig{
				Domain: "api.example.com", Enabled: true, Methods: []string{"GET"}, Paths: []string{"/v1/header/*", "/v1/redirect"},
				Injection: store.SandboxCredentialInjection{Type: "header", Name: "Authorization", Template: "Bearer ${SECRET}-${SECRET}"},
			},
		},
		{
			ID: "c-basic", CurrentSecretRef: "ref-b", CurrentVersion: store.SandboxCredentialVersion{ID: 1, Fingerprint: "fp-b"},
			Config: store.SandboxCredentialConfig{
				Domain: "api.example.com", Enabled: true, Methods: []string{"POST"}, Paths: []string{"/v1/basic/**"},
				Injection: store.SandboxCredentialInjection{Type: "basic", Template: "svc:${SECRET}"},
			},
		},
		{
			ID: "c-cookie", CurrentSecretRef: "ref-c", CurrentVersion: store.SandboxCredentialVersion{ID: 1, Fingerprint: "fp-c"},
			Config: store.SandboxCredentialConfig{
				Domain: "api.example.com", Enabled: true, Methods: []string{"GET"}, Paths: []string{"/v1/cookie"},
				Injection: store.SandboxCredentialInjection{Type: "cookie", Name: "sid", Template: "v1-${SECRET}"},
			},
		},
		{
			ID: "c-query", CurrentSecretRef: "ref-q", CurrentVersion: store.SandboxCredentialVersion{ID: 1, Fingerprint: "fp-q"},
			Config: store.SandboxCredentialConfig{
				Domain: "api.example.com", Enabled: true, Methods: []string{"GET"}, Paths: []string{"/v1/query"},
				Injection: store.SandboxCredentialInjection{Type: "query", Name: "api_key"},
			},
		},
		{
			ID: "c-ovl-z", CurrentSecretRef: "ref-oz", CurrentVersion: store.SandboxCredentialVersion{ID: 1, Fingerprint: "fp-oz"},
			Config: store.SandboxCredentialConfig{
				Domain: "overlap.example.com", Enabled: true,
				Injection: store.SandboxCredentialInjection{Type: "header", Name: "X-Overlap-Z"},
			},
		},
		{
			ID: "c-ovl-a", CurrentSecretRef: "ref-oa", CurrentVersion: store.SandboxCredentialVersion{ID: 1, Fingerprint: "fp-oa"},
			Config: store.SandboxCredentialConfig{
				Domain: "overlap.example.com", Enabled: true,
				Injection: store.SandboxCredentialInjection{Type: "header", Name: "X-Overlap-A"},
			},
		},
		{
			ID: "c-live", CurrentSecretRef: "ref-l1", CurrentVersion: store.SandboxCredentialVersion{ID: 1, Fingerprint: "fp-l1"},
			Config: store.SandboxCredentialConfig{
				Domain: "live.example.com", Enabled: true,
				Injection: store.SandboxCredentialInjection{Type: "header", Name: "X-Live-Token"},
			},
		},
	}

	var nowMu sync.Mutex
	now := time.Now().UTC()
	clock := func() time.Time {
		nowMu.Lock()
		defer nowMu.Unlock()
		return now
	}
	advanceClock := func(d time.Duration) {
		nowMu.Lock()
		now = now.Add(d)
		nowMu.Unlock()
	}

	hmacSecret := []byte("0123456789abcdef0123456789abcdef")
	cpServer := httptest.NewServer(&sandbox.BrokerLookupHandler{
		Secret:   hmacSecret,
		Resolver: &sandbox.BrokerLookupResolver{Store: st, Vault: vault, Now: clock},
		Nonce:    &e2eNonce{},
		Now:      clock,
	})
	defer cpServer.Close()

	// 3. Broker server with fake DNS + Dial routing public test IPs to loopback upstreams.
	registry := prometheus.NewRegistry()
	metrics := sshgateway.NewMetrics(registry)
	brokerCA, brokerCAPEM := makeBrokerCA(t)

	var logMu sync.Mutex
	var logBuf bytes.Buffer
	var dialCount atomic.Int32

	const (
		ipUpstreamA = "93.184.216.34"
		ipUpstreamB = "93.184.216.35"
	)
	resolver := staticResolver{
		"api.example.com":             {netip.MustParseAddr(ipUpstreamA)},
		"uncred.example.com":          {netip.MustParseAddr(ipUpstreamA)},
		"overlap.example.com":         {netip.MustParseAddr(ipUpstreamA)},
		"live.example.com":            {netip.MustParseAddr(ipUpstreamA)},
		"redirect-target.example.com": {netip.MustParseAddr(ipUpstreamB)},
		"ssrf-loopback.example":       {netip.MustParseAddr("127.0.0.1")},
		"ssrf-meta.example":           {netip.MustParseAddr("169.254.169.254")},
		"ssrf-private.example":        {netip.MustParseAddr("10.0.0.1")},
		"ssrf-ula.example":            {netip.MustParseAddr("fc00::1")},
		"ssrf-rebind.example":         {netip.MustParseAddr(ipUpstreamA), netip.MustParseAddr("127.0.0.1")},
	}

	lookupClient := &sandbox.BrokerLookupClient{
		URL:              cpServer.URL,
		Secret:           hmacSecret,
		CacheTTL:         2 * time.Second,
		Now:              clock,
		OnCacheEvent:     metrics.SandboxBrokerCache,
		OnLookupDuration: metrics.SandboxBrokerLookupDuration,
	}

	broker := &Broker{
		Pods: staticPodResolver{
			"127.0.0.1": {Namespace: "tea-1-sandbox", Workspace: "tea-1", Name: "sbx-1-0", UID: "uid-1", SandboxID: "sbx-1", GroupID: "sbg-1"},
			"::1":       {Namespace: "tea-1-sandbox", Workspace: "tea-1", Name: "sbx-1-0", UID: "uid-1", SandboxID: "sbx-1", GroupID: "sbg-1"},
		},
		API:             lookupClient,
		CA:              brokerCA,
		Metrics:         metrics,
		Resolver:        resolver,
		UpstreamRootCAs: upstreamRoots,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialCount.Add(1)
			var target string
			switch address {
			case ipUpstreamA + ":443":
				target = upstreamA.Listener.Addr().String()
			case ipUpstreamB + ":443":
				target = upstreamB.Listener.Addr().String()
			default:
				return nil, fmt.Errorf("unexpected dial target %s", address)
			}
			var d net.Dialer
			return d.DialContext(ctx, network, target)
		},
		Logf: func(format string, args ...any) {
			logMu.Lock()
			fmt.Fprintf(&logBuf, format+"\n", args...)
			logMu.Unlock()
		},
	}

	brokerSrv := httptest.NewServer(broker.Handler())
	defer brokerSrv.Close()
	brokerURL, _ := url.Parse(brokerSrv.URL)

	// Sandbox HTTP client trusts BOTH public/upstream CA (for pass-through) and
	// the public Broker CA (for selectively MITM'd credentialed hosts).
	clientRoots := upstreamRoots.Clone()
	if !clientRoots.AppendCertsFromPEM(brokerCAPEM) {
		t.Fatal("failed to add Broker CA to sandbox client roots")
	}
	newClient := func(followRedirects bool) *http.Client {
		c := &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyURL(brokerURL),
				TLSClientConfig: &tls.Config{RootCAs: clientRoots, MinVersion: tls.VersionTLS12},
			},
			Timeout: 10 * time.Second,
		}
		if !followRedirects {
			c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		}
		return c
	}

	decodeEcho := func(t *testing.T, resp *http.Response) capturedUpstreamReq {
		t.Helper()
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(body))
		}
		var out capturedUpstreamReq
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode echo: %v", err)
		}
		return out
	}

	client := newClient(false)

	// --- 1. Header injection (2x ${SECRET} template) + replaces caller header ---
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/header/item", nil)
	req.Header.Set("Authorization", "Bearer spoofed")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("header request: %v", err)
	}
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 || resp.TLS.PeerCertificates[0].Issuer.CommonName != "bex-sandbox-broker-ca" {
		t.Fatalf("expected Broker CA leaf for credentialed host, got issuer=%v", resp.TLS)
	}
	got := decodeEcho(t, resp)
	if got.Header["Authorization"] != "Bearer "+secretHeader+"-"+secretHeader {
		t.Fatalf("header injection got %q", got.Header["Authorization"])
	}

	// --- 2. Basic injection + replaces caller Authorization ---
	req, _ = http.NewRequest(http.MethodPost, "https://api.example.com/v1/basic/a/b", strings.NewReader("payload"))
	req.Header.Set("Authorization", "Basic c3Bvb2Y=")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("basic request: %v", err)
	}
	got = decodeEcho(t, resp)
	wantBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("svc:"+secretBasic))
	if got.Header["Authorization"] != wantBasic {
		t.Fatalf("basic injection got %q, want %q", got.Header["Authorization"], wantBasic)
	}

	// --- 3. Cookie injection + preserves unrelated cookies ---
	req, _ = http.NewRequest(http.MethodGet, "https://api.example.com/v1/cookie", nil)
	req.Header.Set("Cookie", "theme=dark; sid=spoofed; keep=yes")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("cookie request: %v", err)
	}
	got = decodeEcho(t, resp)
	if got.Header["Cookie"] != "theme=dark; keep=yes; sid=v1-"+secretCookie {
		t.Fatalf("cookie injection got %q", got.Header["Cookie"])
	}

	// --- 4. Query injection + preserves unrelated query params ---
	req, _ = http.NewRequest(http.MethodGet, "https://api.example.com/v1/query?foo=1&api_key=spoof&bar=2", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("query request: %v", err)
	}
	got = decodeEcho(t, resp)
	q, _ := url.ParseQuery(got.Query)
	if q.Get("foo") != "1" || q.Get("bar") != "2" || q.Get("api_key") != secretQuery {
		t.Fatalf("query injection got %q", got.Query)
	}

	// --- 5. Non-matching method & non-matching path send ZERO injected secret ---
	req, _ = http.NewRequest(http.MethodDelete, "https://api.example.com/v1/header/item", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("non-matching method: %v", err)
	}
	got = decodeEcho(t, resp)
	if got.Header["Authorization"] != "" {
		t.Fatalf("non-matching method injected Authorization=%q", got.Header["Authorization"])
	}

	req, _ = http.NewRequest(http.MethodGet, "https://api.example.com/v1/header", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("non-matching path: %v", err)
	}
	got = decodeEcho(t, resp)
	if got.Header["Authorization"] != "" {
		t.Fatalf("non-matching path injected Authorization=%q", got.Header["Authorization"])
	}

	// --- 6. Overlapping credentials: deterministic single-match (c-ovl-a wins, c-ovl-z NOT injected) ---
	resp, err = client.Get("https://overlap.example.com/anything")
	if err != nil {
		t.Fatalf("overlap request: %v", err)
	}
	got = decodeEcho(t, resp)
	if got.Header["X-Overlap-A"] != secretOverlapA || got.Header["X-Overlap-Z"] != "" {
		t.Fatalf("overlap headers = (A=%q, Z=%q), want only A", got.Header["X-Overlap-A"], got.Header["X-Overlap-Z"])
	}

	// --- 7. Uncredentialed host completes raw TLS pass-through with upstream's real leaf cert ---
	resp, err = client.Get("https://uncred.example.com/ping")
	if err != nil {
		t.Fatalf("uncredentialed host: %v", err)
	}
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 || resp.TLS.PeerCertificates[0].Issuer.CommonName != "upstream-test-ca" {
		t.Fatalf("expected raw upstream cert on uncredentialed host, got issuer=%v", resp.TLS.PeerCertificates[0].Issuer.CommonName)
	}
	_ = decodeEcho(t, resp)

	// --- 8. Cross-host 302 redirect: broker returns 302 unfollowed; follow carries ZERO secret ---
	resp, err = client.Get("https://api.example.com/v1/redirect")
	if err != nil {
		t.Fatalf("302 request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "https://redirect-target.example.com/landing" {
		t.Fatalf("expected unfollowed 302, got status=%d location=%q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if redirectTargetHits.Load() != 0 {
		t.Fatal("broker followed 302 redirect upstream!")
	}
	followClient := newClient(true)
	resp, err = followClient.Get("https://api.example.com/v1/redirect")
	if err != nil {
		t.Fatalf("client-followed 302: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || redirectTargetHits.Load() != 1 {
		t.Fatalf("redirect target status=%d hits=%d", resp.StatusCode, redirectTargetHits.Load())
	}
	if redirectTargetSawSecret.Load() {
		t.Fatal("redirect target received leaked secret!")
	}

	// --- 9. SNI mismatch and Host smuggling refused ---
	t.Run("SNI mismatch refused", func(t *testing.T) {
		conn, err := net.Dial("tcp", brokerSrv.Listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_, _ = io.WriteString(conn, "CONNECT api.example.com:443 HTTP/1.1\r\nHost: api.example.com:443\r\n\r\n")
		br := bufio.NewReader(conn)
		connectResp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
		if err != nil || connectResp.StatusCode != http.StatusOK {
			t.Fatalf("CONNECT failed: resp=%v err=%v", connectResp, err)
		}
		tc := tls.Client(&bufferedConn{Conn: conn, r: br}, &tls.Config{
			ServerName: "evil.example.com",
			RootCAs:    clientRoots,
			MinVersion: tls.VersionTLS12,
		})
		if err := tc.Handshake(); err == nil {
			t.Fatal("expected TLS handshake failure on SNI mismatch")
		}
	})

	t.Run("Host header smuggling refused with 421", func(t *testing.T) {
		conn, err := net.Dial("tcp", brokerSrv.Listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_, _ = io.WriteString(conn, "CONNECT api.example.com:443 HTTP/1.1\r\nHost: api.example.com:443\r\n\r\n")
		br := bufio.NewReader(conn)
		connectResp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
		if err != nil || connectResp.StatusCode != http.StatusOK {
			t.Fatalf("CONNECT failed: resp=%v err=%v", connectResp, err)
		}
		tc := tls.Client(&bufferedConn{Conn: conn, r: br}, &tls.Config{
			ServerName: "api.example.com",
			RootCAs:    clientRoots,
			MinVersion: tls.VersionTLS12,
		})
		if err := tc.Handshake(); err != nil {
			t.Fatalf("TLS handshake failed: %v", err)
		}
		_, _ = io.WriteString(tc, "GET /v1/header/item HTTP/1.1\r\nHost: evil.example.com\r\nConnection: close\r\n\r\n")
		resp, err := http.ReadResponse(bufio.NewReader(tc), nil)
		if err != nil {
			t.Fatalf("read smuggled response: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMisdirectedRequest {
			t.Fatalf("Host smuggling status = %d, want 421", resp.StatusCode)
		}
	})

	// --- 10. SSRF refused BEFORE dialing ---
	dialsBefore := dialCount.Load()
	for _, host := range []string{"ssrf-loopback.example", "ssrf-meta.example", "ssrf-private.example", "ssrf-ula.example", "ssrf-rebind.example"} {
		if _, err := newClient(false).Get("https://" + host + "/secret"); err == nil {
			t.Fatalf("SSRF host %q succeeded, want refusal", host)
		}
	}
	if dialCount.Load() != dialsBefore {
		t.Fatalf("SSRF attempts triggered upstream dial! before=%d after=%d", dialsBefore, dialCount.Load())
	}

	// --- 11. networkPolicy: deny-all refuses everything; allow-list refuses unlisted credentialed host ---
	setPolicy(&sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyDenyAll})
	advanceClock(3 * time.Second)
	dialsBefore = dialCount.Load()
	if _, err := newClient(false).Get("https://api.example.com/v1/header/item"); err == nil {
		t.Fatal("deny-all allowed credentialed host")
	}
	if dialCount.Load() != dialsBefore {
		t.Fatal("deny-all dialed upstream")
	}

	setPolicy(&sandbox.NetworkPolicy{
		Default: sandbox.NetworkPolicyAllowList,
		Rules:   []sandbox.NetworkPolicyRule{{Domain: "live.example.com", Protocol: sandbox.NetworkPolicyProtocolHTTPS}},
	})
	advanceClock(3 * time.Second)
	dialsBefore = dialCount.Load()
	if _, err := newClient(false).Get("https://api.example.com/v1/header/item"); err == nil {
		t.Fatal("allow-list allowed credentialed host absent from rules")
	}
	if dialCount.Load() != dialsBefore {
		t.Fatal("allow-list violation dialed upstream")
	}
	// Listed credentialed host succeeds.
	liveClient := newClient(false)
	resp, err = liveClient.Get("https://live.example.com/check")
	if err != nil {
		t.Fatalf("allow-list admitted host failed: %v", err)
	}
	if got := decodeEcho(t, resp); got.Header["X-Live-Token"] != secretHeader {
		t.Fatalf("live initial token = %q, want %q", got.Header["X-Live-Token"], secretHeader)
	}

	// --- 12. Live cutoff & swap within cache TTL (update, restore, disable, delete) over reused keep-alive client ---
	mutateLiveCred := func(fn func(c *store.SandboxCredential)) {
		st.mu.Lock()
		for i := range st.creds["tea-1|sbg-1"] {
			if st.creds["tea-1|sbg-1"][i].ID == "c-live" {
				fn(&st.creds["tea-1|sbg-1"][i])
			}
		}
		st.mu.Unlock()
		advanceClock(3 * time.Second)
	}

	// Update value -> rotated secret takes effect immediately past TTL.
	mutateLiveCred(func(c *store.SandboxCredential) {
		c.CurrentSecretRef = "ref-l2"
		c.CurrentVersion.ID = 2
		c.CurrentVersion.Fingerprint = "fp-l2"
	})
	resp, err = liveClient.Get("https://live.example.com/check")
	if err != nil {
		t.Fatalf("live rotated request: %v", err)
	}
	if got := decodeEcho(t, resp); got.Header["X-Live-Token"] != secretRotated {
		t.Fatalf("live rotated token = %q, want %q", got.Header["X-Live-Token"], secretRotated)
	}

	// Restore version -> restored secret takes effect past TTL.
	mutateLiveCred(func(c *store.SandboxCredential) {
		c.CurrentSecretRef = "ref-l3"
		c.CurrentVersion.ID = 3
		c.CurrentVersion.Fingerprint = "fp-l3"
	})
	resp, err = liveClient.Get("https://live.example.com/check")
	if err != nil {
		t.Fatalf("live restored request: %v", err)
	}
	if got := decodeEcho(t, resp); got.Header["X-Live-Token"] != secretRestored {
		t.Fatalf("live restored token = %q, want %q", got.Header["X-Live-Token"], secretRestored)
	}

	// Disable -> zero secret injected even on reused keep-alive TLS connection!
	mutateLiveCred(func(c *store.SandboxCredential) {
		c.Config.Enabled = false
	})
	resp, err = liveClient.Get("https://live.example.com/check")
	if err != nil {
		t.Fatalf("live disabled request: %v", err)
	}
	if got := decodeEcho(t, resp); got.Header["X-Live-Token"] != "" {
		t.Fatalf("disabled credential still injected %q", got.Header["X-Live-Token"])
	}

	// Delete -> new connection passes through raw TLS with zero secret!
	st.mu.Lock()
	filtered := st.creds["tea-1|sbg-1"][:0]
	for _, c := range st.creds["tea-1|sbg-1"] {
		if c.ID != "c-live" {
			filtered = append(filtered, c)
		}
	}
	st.creds["tea-1|sbg-1"] = filtered
	st.mu.Unlock()
	advanceClock(3 * time.Second)
	resp, err = newClient(false).Get("https://live.example.com/check")
	if err != nil {
		t.Fatalf("live deleted request: %v", err)
	}
	if resp.TLS.PeerCertificates[0].Issuer.CommonName != "upstream-test-ca" {
		t.Fatalf("deleted credential host still MITM'd (issuer=%q)", resp.TLS.PeerCertificates[0].Issuer.CommonName)
	}
	if got := decodeEcho(t, resp); got.Header["X-Live-Token"] != "" {
		t.Fatalf("deleted credential still injected %q", got.Header["X-Live-Token"])
	}

	// --- 13. Zero secret bytes in captured logs and Prometheus metrics text ---
	mfs, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var metricsText bytes.Buffer
	for _, mf := range mfs {
		if _, err := expfmt.MetricFamilyToText(&metricsText, mf); err != nil {
			t.Fatal(err)
		}
	}
	logMu.Lock()
	logs := logBuf.String()
	logMu.Unlock()
	combined := logs + "\n" + metricsText.String()
	for _, s := range allSecrets {
		if strings.Contains(combined, s) {
			t.Fatalf("secret %q leaked into logs or metrics!", s)
		}
	}
	for _, expectedMetric := range []string{
		"bex_ssh_gateway_sandbox_broker_requests_total",
		"bex_ssh_gateway_sandbox_broker_overlaps_total",
		"bex_ssh_gateway_sandbox_broker_lookup_duration_seconds",
		"bex_ssh_gateway_sandbox_broker_cache_total",
	} {
		if !strings.Contains(metricsText.String(), expectedMetric) {
			t.Errorf("expected metric %s in Prometheus output", expectedMetric)
		}
	}
}

func TestSandboxBrokerNon443PassthroughAndQueryErrorScrubbing(t *testing.T) {
	const querySecret = "fixture-upstream-fail-query"

	var sawSecretOn8443 atomic.Bool
	upstreams, upstreamRoots := makeUpstreamRootWithServers(t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.RawQuery, querySecret) || strings.Contains(r.Header.Get("Authorization"), querySecret) {
				sawSecretOn8443.Store(true)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok-8443")
		}),
		http.NotFoundHandler(),
	)

	brokerCA, brokerCAPEM := makeBrokerCA(t)
	var logMu sync.Mutex
	var logBuf bytes.Buffer
	const ipA = "93.184.216.34"
	const ipFail = "93.184.216.36"

	broker := &Broker{
		Pods: staticPodResolver{
			"127.0.0.1": {Namespace: "tea-1-sandbox", Workspace: "tea-1", Name: "sbx-1-0", UID: "u1", SandboxID: "sbx-1", GroupID: "sbg-1"},
			"::1":       {Namespace: "tea-1-sandbox", Workspace: "tea-1", Name: "sbx-1-0", UID: "u1", SandboxID: "sbx-1", GroupID: "sbg-1"},
		},
		API: &staticLookupClient{
			resp: sandbox.BrokerLookupResponse{
				WorkspaceID:    "tea-1",
				SandboxID:      "sbx-1",
				SandboxGroupID: "sbg-1",
				NetworkPolicy:  sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyAllowAll},
				Credentials: []sandbox.BrokerCredential{
					{
						ID: "c-q", Domain: "api.example.com", Value: querySecret,
						Injection: sandbox.GroupCredentialInjection{Type: "query", Name: "token"},
					},
					{
						ID: "c-fail", Domain: "fail.example.com", Value: querySecret,
						Injection: sandbox.GroupCredentialInjection{Type: "query", Name: "token"},
					},
				},
			},
		},
		CA:              brokerCA,
		Ports:           []int{443, 8443},
		UpstreamRootCAs: upstreamRoots,
		Resolver: staticResolver{
			"api.example.com":  {netip.MustParseAddr(ipA)},
			"fail.example.com": {netip.MustParseAddr(ipFail)},
		},
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address == ipA+":8443" {
				var d net.Dialer
				return d.DialContext(ctx, network, upstreams.srvA.Listener.Addr().String())
			}
			return nil, &net.OpError{Op: "dial", Net: "tcp", Err: io.EOF}
		},
		Logf: func(format string, args ...any) {
			logMu.Lock()
			fmt.Fprintf(&logBuf, format+"\n", args...)
			logMu.Unlock()
		},
	}

	srv := httptest.NewServer(broker.Handler())
	defer srv.Close()
	proxyURL, _ := url.Parse(srv.URL)
	roots := upstreamRoots.Clone()
	roots.AppendCertsFromPEM(brokerCAPEM)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		},
	}

	// 1. Even when Ports explicitly allows 8443, a credentialed domain on :8443
	// MUST pass through raw TLS with the real upstream cert and ZERO injection.
	resp, err := client.Get("https://api.example.com:8443/check")
	if err != nil {
		t.Fatalf("port 8443 request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.TLS.PeerCertificates[0].Issuer.CommonName != "upstream-test-ca" {
		t.Fatalf("port 8443 was MITM'd! issuer=%q", resp.TLS.PeerCertificates[0].Issuer.CommonName)
	}
	if sawSecretOn8443.Load() {
		t.Fatal("credential was injected on non-443 port!")
	}

	// 2. Upstream dial failure after query injection returns scrubbed 502 with
	// ZERO secret bytes in response body or gateway logs.
	resp, err = client.Get("https://fail.example.com/v1/x")
	if err != nil {
		t.Fatalf("expected scrubbed 502 response, got transport error: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	logMu.Lock()
	logs := logBuf.String()
	logMu.Unlock()
	if strings.Contains(string(body), querySecret) || strings.Contains(logs, querySecret) {
		t.Fatalf("query secret leaked in 502 body or logs! body=%q logs=%q", string(body), logs)
	}
}

type upstreamPair struct {
	srvA *httptest.Server
	srvB *httptest.Server
}

func makeUpstreamRootWithServers(t *testing.T, handlerA, handlerB http.Handler) (upstreamPair, *x509.CertPool) {
	t.Helper()
	now := time.Now().UTC()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(500),
		Subject:               pkix.Name{CommonName: "upstream-test-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	startServer := func(serial int64, hosts []string, h http.Handler) *httptest.Server {
		leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		leafTmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(serial),
			Subject:               pkix.Name{CommonName: hosts[0]},
			NotBefore:             now.Add(-time.Hour),
			NotAfter:              now.Add(24 * time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
			DNSNames:              hosts,
		}
		leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewUnstartedServer(h)
		srv.TLS = &tls.Config{
			MinVersion: tls.VersionTLS12,
			Certificates: []tls.Certificate{{
				Certificate: [][]byte{leafDER},
				PrivateKey:  leafKey,
			}},
		}
		srv.StartTLS()
		t.Cleanup(srv.Close)
		return srv
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return upstreamPair{
		srvA: startServer(501, []string{"api.example.com", "uncred.example.com", "overlap.example.com", "live.example.com"}, handlerA),
		srvB: startServer(502, []string{"redirect-target.example.com"}, handlerB),
	}, pool
}
