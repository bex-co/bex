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
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

const (
	defaultMaxConns               = 256
	defaultMaxConnsPerPod         = 32
	defaultIdleTimeout            = 5 * time.Minute
	defaultMaxDuration            = time.Hour
	defaultDialTimeout            = 10 * time.Second
	defaultTLSHandshakeTimeout    = 10 * time.Second
	defaultResponseHeaderTimeout  = time.Minute
	defaultMaxResponseHeaderBytes = 1 << 20

	outcomeInjected    = "injected"
	outcomePassthrough = "passthrough"
	outcomeDenied      = "denied"
	outcomeError       = "error"
)

// DefaultPorts are the destination ports allowed on CONNECT when Broker.Ports
// is empty (HTTPS port 443 only).
var DefaultPorts = []int{443}

// LookupClient fetches a verified sandbox Pod's networkPolicy and enabled group
// credentials from bex-api's internal :8091 broker lookup route.
// *sandbox.BrokerLookupClient satisfies it.
type LookupClient interface {
	Lookup(ctx context.Context, req sandbox.BrokerLookupRequest) (sandbox.BrokerLookupResponse, error)
}

// Broker is the Pod-bound outbound HTTPS credential broker for /v1/sandboxes
// Pods (w2/m534, docs/ADR122-sandbox-credential-brokering.md).
//
// Security invariants:
//   - Identity comes strictly from direct TCP source Pod IP + immutable sandbox
//     labels (`app.bex.co/regime=sandbox`, `bex.co/sandbox-id`,
//     `bex.co/sandbox-group`, non-terminating, `bex.co/agent-session` absent).
//     Caller-supplied `Proxy-Authorization` headers are never trusted.
//   - Plain HTTP requests and non-443 ports never receive injected credentials.
//   - Uncredentialed hosts pass through raw TCP/TLS unmodified (real upstream
//     server certificate preserved).
//   - Credentialed hosts on port 443 terminate TLS with a short-lived exact-host
//     Broker CA leaf cert, reject SNI and Host smuggling, inject the single
//     winning credential on matching (method, path), forward over verified
//     public-root TLS to the resolve-once SSRF-checked IP, and return 3xx
//     responses unfollowed.
//   - Zero secret bytes ever enter logs, error bodies, or metrics.
type Broker struct {
	Pods    PodResolver
	API     LookupClient
	CA      *CA
	Metrics *sshgateway.Metrics
	Limits  *sshgateway.SessionLimiter

	// Resolver and Dial are seams; nil uses net.DefaultResolver and net.Dialer.
	// The proxy resolves once and dials the verified public IP directly.
	Resolver Resolver
	Dial     func(ctx context.Context, network, address string) (net.Conn, error)

	// Ports are allowed CONNECT destination ports; empty defaults to [443].
	// Regardless of Ports, selective MITM injection only ever runs on port 443.
	Ports []int
	// DenyPrefixes adds deployment-specific cluster CIDRs to the SSRF floor.
	DenyPrefixes []netip.Prefix

	IdleTimeout time.Duration
	MaxDuration time.Duration

	// UpstreamRootCAs overrides system roots for upstream TLS verification
	// (nil in production uses OS system roots; tests inject a loopback root CA).
	UpstreamRootCAs *x509.CertPool
	// Logf overrides log.Printf for secret-free operational logging.
	Logf func(format string, args ...any)

	limitsOnce sync.Once
	limits     *sshgateway.SessionLimiter
}

// Enabled reports whether all required broker dependencies are wired.
func (b *Broker) Enabled() bool {
	return b != nil && b.Pods != nil && b.API != nil && b.CA != nil
}

// Handler serves the broker listener (CONNECT + GET /healthz).
func (b *Broker) Handler() http.Handler { return http.HandlerFunc(b.serve) }

func (b *Broker) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && !r.URL.IsAbs() && r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !b.Enabled() {
		b.Metrics.SandboxBrokerRequest(outcomeError)
		http.Error(w, "bex sandbox broker unavailable", http.StatusServiceUnavailable)
		return
	}
	// HTTPS CONNECT only: plain HTTP requests are never injected or relayed.
	if r.Method != http.MethodConnect {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		http.Error(w, "bex sandbox broker: HTTPS CONNECT required", http.StatusMethodNotAllowed)
		return
	}

	sourceIP, err := sshgateway.RemoteIP(r.RemoteAddr)
	if err != nil {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	limiter := b.limiter()
	if ok, scope := limiter.Acquire(sourceIP); !ok {
		b.Metrics.LimitRejected("sandbox_broker_" + scope)
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		http.Error(w, "too many concurrent broker connections", http.StatusTooManyRequests)
		return
	}
	defer limiter.Release(sourceIP)

	// Authenticate strictly by direct source Pod IP + immutable labels.
	pod, err := b.Pods.ResolveSandboxPod(r.Context(), sourceIP)
	if err != nil {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	rawHost, rawPort, err := net.SplitHostPort(r.Host)
	if err != nil {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		http.Error(w, "bex sandbox broker: CONNECT requires host:port", http.StatusBadRequest)
		return
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port <= 0 || port > 65535 || !slices.Contains(b.ports(), port) {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		http.Error(w, "bex sandbox broker: destination port not allowed", http.StatusForbidden)
		return
	}

	lookupReq := sandbox.BrokerLookupRequest{
		WorkspaceID:    pod.Workspace,
		SandboxID:      pod.SandboxID,
		SandboxGroupID: pod.GroupID,
		PodName:        pod.Name,
		PodUID:         pod.UID,
	}
	lookup, err := b.API.Lookup(r.Context(), lookupReq)
	if err != nil {
		if errors.Is(err, sandbox.ErrBrokerForbidden) {
			b.Metrics.SandboxBrokerRequest(outcomeDenied)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		b.logf("sandbox broker: lookup unavailable (sandbox=%s): %s", pod.SandboxID, scrubErrorCause(err))
		b.Metrics.SandboxBrokerRequest(outcomeError)
		http.Error(w, "bex sandbox broker unavailable", http.StatusServiceUnavailable)
		return
	}

	// Enforce sandbox networkPolicy BEFORE DNS resolution or dialing.
	normHost := NormalizeHost(rawHost)
	_, isLiteral := netip.ParseAddr(strings.Trim(normHost, "[]"))
	if !b.policyAllowsConnect(lookup.NetworkPolicy, normHost, port, isLiteral == nil) {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		http.Error(w, "bex sandbox broker: destination denied by sandbox networkPolicy", http.StatusForbidden)
		return
	}

	// Resolve-once SSRF enforcement BEFORE dialing.
	checkedAddr, canonicalHost, literalIP, err := b.guard().ResolveAndCheck(r.Context(), rawHost, port)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidTarget):
			b.Metrics.SandboxBrokerRequest(outcomeDenied)
			http.Error(w, "bex sandbox broker: invalid destination", http.StatusBadRequest)
		case errors.Is(err, ErrSSRFBlocked):
			b.Metrics.SandboxBrokerRequest(outcomeDenied)
			http.Error(w, "bex sandbox broker: destination blocked by security policy", http.StatusForbidden)
		default:
			b.Metrics.SandboxBrokerRequest(outcomeError)
			http.Error(w, "bex sandbox broker: could not resolve destination", http.StatusBadGateway)
		}
		return
	}

	// Selective MITM: ONLY on HTTPS port 443 for a validated public DNS host
	// that currently has an enabled group credential. Everything else passes
	// through raw TCP/TLS untouched.
	if port == 443 && !literalIP && HasCredentialForHost(lookup.Credentials, canonicalHost) {
		b.serveMITM(w, r, lookupReq, canonicalHost, checkedAddr)
		return
	}
	b.servePassthrough(w, r, checkedAddr)
}

func (b *Broker) policyAllowsConnect(policy sandbox.NetworkPolicy, host string, port int, isLiteralIP bool) bool {
	switch policy.Default {
	case sandbox.NetworkPolicyAllowAll:
		return true
	case sandbox.NetworkPolicyAllowList:
		if port != 443 || isLiteralIP {
			return false
		}
		return AllowsDestination(policy, host)
	default:
		return false
	}
}

func (b *Broker) servePassthrough(w http.ResponseWriter, r *http.Request, addr netip.AddrPort) {
	upstream, err := b.dial(r.Context(), addr)
	if err != nil {
		b.Metrics.SandboxBrokerRequest(outcomeError)
		http.Error(w, "bex sandbox broker: upstream connection failed", http.StatusBadGateway)
		return
	}
	defer upstream.Close()

	client, buffered, err := http.NewResponseController(w).Hijack()
	if err != nil {
		b.Metrics.SandboxBrokerRequest(outcomeError)
		http.Error(w, "bex sandbox broker unavailable", http.StatusInternalServerError)
		return
	}
	defer client.Close()
	_ = client.SetDeadline(time.Time{})
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	b.Metrics.SandboxBrokerRequest(outcomePassthrough)
	b.splice(client, buffered.Reader, upstream)
}

func (b *Broker) serveMITM(w http.ResponseWriter, r *http.Request, lookupReq sandbox.BrokerLookupRequest, connectHost string, checkedAddr netip.AddrPort) {
	client, buffered, err := http.NewResponseController(w).Hijack()
	if err != nil {
		b.Metrics.SandboxBrokerRequest(outcomeError)
		http.Error(w, "bex sandbox broker unavailable", http.StatusInternalServerError)
		return
	}
	defer client.Close()
	_ = client.SetDeadline(time.Time{})
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}

	var sniMismatch bool
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1"},
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			if hello.ServerName != "" && NormalizeHost(hello.ServerName) != connectHost {
				sniMismatch = true
				return nil, errors.New("sni host mismatch")
			}
			leaf, certErr := b.CA.CertForHost(connectHost)
			if certErr != nil {
				return nil, certErr
			}
			return &tls.Config{
				MinVersion:   tls.VersionTLS12,
				NextProtos:   []string{"http/1.1"},
				Certificates: []tls.Certificate{*leaf},
			}, nil
		},
	}

	baseConn := &bufferedConn{Conn: client, r: buffered.Reader}
	tlsConn := tls.Server(baseConn, tlsCfg)
	defer tlsConn.Close()

	_ = tlsConn.SetDeadline(time.Now().Add(defaultTLSHandshakeTimeout))
	if err := tlsConn.HandshakeContext(r.Context()); err != nil {
		if sniMismatch {
			b.Metrics.SandboxBrokerRequest(outcomeDenied)
		} else {
			b.Metrics.SandboxBrokerRequest(outcomeError)
		}
		return
	}
	_ = tlsConn.SetDeadline(time.Time{})

	lifetime := time.AfterFunc(b.maxDuration(), func() {
		_ = tlsConn.Close()
	})
	defer lifetime.Stop()

	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return b.dial(ctx, checkedAddr)
		},
		TLSClientConfig: &tls.Config{
			ServerName: connectHost,
			MinVersion: tls.VersionTLS12,
			RootCAs:    b.UpstreamRootCAs,
		},
		DisableCompression:     true,
		ResponseHeaderTimeout:  defaultResponseHeaderTimeout,
		MaxResponseHeaderBytes: defaultMaxResponseHeaderBytes,
	}
	defer transport.CloseIdleConnections()

	br := bufio.NewReader(tlsConn)
	idle := b.idleTimeout()
	for {
		_ = tlsConn.SetReadDeadline(time.Now().Add(idle))
		inReq, err := http.ReadRequest(br)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				b.Metrics.SandboxBrokerRequest(outcomeError)
				writeSimpleError(tlsConn, http.StatusBadRequest, "bex sandbox broker: malformed HTTP request")
			}
			return
		}
		_ = tlsConn.SetReadDeadline(time.Time{})

		keepAlive := b.handleMITMRequest(r.Context(), tlsConn, transport, lookupReq, connectHost, inReq)
		if !keepAlive {
			return
		}
	}
}

// handleMITMRequest processes one HTTP/1.1 request inside a selectively
// terminated TLS connection. Returns true if the connection may be kept alive.
func (b *Broker) handleMITMRequest(ctx context.Context, tlsConn net.Conn, transport *http.Transport, lookupReq sandbox.BrokerLookupRequest, connectHost string, inReq *http.Request) bool {
	defer inReq.Body.Close()

	if len(inReq.TransferEncoding) > 1 || (len(inReq.TransferEncoding) == 1 && !strings.EqualFold(inReq.TransferEncoding[0], "chunked")) {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		writeSimpleError(tlsConn, http.StatusBadRequest, "bex sandbox broker: unsupported transfer encoding")
		return false
	}

	// Enforce Host == negotiated CONNECT/SNI hostname (blocks Host smuggling).
	if !hostMatchesConnect(inReq.Host, connectHost) {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		writeSimpleError(tlsConn, http.StatusMisdirectedRequest, "bex sandbox broker: Host header does not match TLS target")
		return false
	}
	if inReq.URL.IsAbs() && !hostMatchesConnect(inReq.URL.Host, connectHost) {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		writeSimpleError(tlsConn, http.StatusMisdirectedRequest, "bex sandbox broker: request target does not match TLS target")
		return false
	}

	// Re-check live lookup (served from <=5s in-memory cache) so credential
	// disable/delete/update/restore or sandbox termination cuts off promptly
	// even over a reused keep-alive TLS connection.
	fresh, err := b.API.Lookup(ctx, lookupReq)
	if err != nil {
		if errors.Is(err, sandbox.ErrBrokerForbidden) {
			b.Metrics.SandboxBrokerRequest(outcomeDenied)
			writeSimpleError(tlsConn, http.StatusForbidden, "forbidden")
			return false
		}
		b.logf("sandbox broker: refresh lookup failed (sandbox=%s): %s", lookupReq.SandboxID, scrubErrorCause(err))
		b.Metrics.SandboxBrokerRequest(outcomeError)
		writeSimpleError(tlsConn, http.StatusServiceUnavailable, "bex sandbox broker unavailable")
		return false
	}
	if !b.policyAllowsConnect(fresh.NetworkPolicy, connectHost, 443, false) {
		b.Metrics.SandboxBrokerRequest(outcomeDenied)
		writeSimpleError(tlsConn, http.StatusForbidden, "bex sandbox broker: destination denied by sandbox networkPolicy")
		return false
	}

	outURL := *inReq.URL
	outURL.Scheme = "https"
	outURL.Host = connectHost

	reqCtx, cancel := context.WithTimeout(ctx, b.maxDuration())
	defer cancel()
	outReq := inReq.Clone(reqCtx)
	outReq.URL = &outURL
	outReq.RequestURI = ""
	outReq.Host = connectHost
	outReq.Body = inReq.Body
	outReq.ContentLength = inReq.ContentLength
	outReq.TransferEncoding = slices.Clone(inReq.TransferEncoding)
	sshgateway.RemoveHopHeaders(outReq.Header)

	matched, overlap := MatchCredential(fresh.Credentials, connectHost, outReq.Method, outReq.URL.Path)
	if matched != nil {
		if overlap {
			b.Metrics.SandboxBrokerOverlap()
		}
		if err := Inject(outReq, *matched); err != nil {
			b.logf("sandbox broker: injection failed (sandbox=%s): %s", lookupReq.SandboxID, scrubErrorCause(err))
			b.Metrics.SandboxBrokerRequest(outcomeError)
			writeSimpleError(tlsConn, http.StatusBadGateway, "bex sandbox broker: credential injection failed")
			return false
		}
	}

	// RoundTrip NEVER follows HTTP redirects; 3xx responses return verbatim to
	// the sandbox so injected secrets can never leak to a redirected host.
	resp, err := transport.RoundTrip(outReq)
	if err != nil {
		// Never log err.Error() directly: *url.Error contains outReq.URL, which
		// may carry an injected query-parameter secret.
		b.logf("sandbox broker: upstream roundtrip failed (sandbox=%s): %s", lookupReq.SandboxID, scrubErrorCause(err))
		b.Metrics.SandboxBrokerRequest(outcomeError)
		writeSimpleError(tlsConn, http.StatusBadGateway, "bex sandbox broker: upstream request failed")
		return false
	}
	defer resp.Body.Close()

	sshgateway.RemoveHopHeaders(resp.Header)
	resp.Request = outReq
	closeAfter := inReq.Close || resp.Close || !inReq.ProtoAtLeast(1, 1) || (resp.ContentLength < 0 && len(resp.TransferEncoding) == 0)
	if closeAfter {
		resp.Close = true
		resp.Header.Set("Connection", "close")
	}

	_ = tlsConn.SetWriteDeadline(time.Now().Add(b.idleTimeout()))
	writeErr := resp.Write(tlsConn)
	_ = tlsConn.SetWriteDeadline(time.Time{})
	if writeErr != nil {
		b.Metrics.SandboxBrokerRequest(outcomeError)
		return false
	}

	if matched != nil {
		b.Metrics.SandboxBrokerRequest(outcomeInjected)
	} else {
		b.Metrics.SandboxBrokerRequest(outcomePassthrough)
	}
	return !closeAfter
}

func hostMatchesConnect(headerHost, connectHost string) bool {
	headerHost = strings.TrimSpace(headerHost)
	if headerHost == "" {
		return false
	}
	h, p, err := net.SplitHostPort(headerHost)
	if err == nil {
		if p != "443" {
			return false
		}
		return NormalizeHost(h) == connectHost
	}
	// Reject malformed host:port (colon without valid host:port split).
	if strings.Contains(headerHost, ":") {
		return false
	}
	return NormalizeHost(headerHost) == connectHost
}

func writeSimpleError(conn net.Conn, status int, msg string) {
	body := msg + "\n"
	resp := &http.Response{
		StatusCode:    status,
		Status:        strconv.Itoa(status) + " " + http.StatusText(status),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": {"text/plain; charset=utf-8"}, "Connection": {"close"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Close:         true,
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = resp.Write(conn)
	_ = conn.SetWriteDeadline(time.Time{})
}

// scrubErrorCause extracts a secret-free cause description from err. It strips
// *url.Error.URL so query-parameter credentials can never appear in logs.
func scrubErrorCause(err error) string {
	if err == nil {
		return ""
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Op + ": " + scrubErrorCause(ue.Err)
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, sandbox.ErrBrokerUnavailable):
		return "broker_unavailable"
	case errors.Is(err, sandbox.ErrBrokerForbidden):
		return "broker_forbidden"
	case errors.Is(err, ErrSSRFBlocked):
		return "ssrf_blocked"
	case errors.Is(err, ErrDNSLookupFailed):
		return "dns_failed"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "network_timeout"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return "net_" + opErr.Op + "_failed"
	}
	var x509Unknown x509.UnknownAuthorityError
	if errors.As(err, &x509Unknown) {
		return "upstream_tls_unknown_authority"
	}
	var x509Host x509.HostnameError
	if errors.As(err, &x509Host) {
		return "upstream_tls_hostname_mismatch"
	}
	var x509Invalid x509.CertificateInvalidError
	if errors.As(err, &x509Invalid) {
		return "upstream_tls_invalid_cert"
	}
	return "internal_error"
}

func (b *Broker) splice(client net.Conn, clientReader io.Reader, upstream net.Conn) {
	idle := b.idleTimeout()
	lifetime := time.AfterFunc(b.maxDuration(), func() {
		_ = client.Close()
		_ = upstream.Close()
	})
	defer lifetime.Stop()
	done := make(chan struct{}, 2)
	go func() {
		copyWithIdle(upstream, clientReader, client, idle)
		if closer, ok := upstream.(interface{ CloseWrite() error }); ok {
			_ = closer.CloseWrite()
		} else {
			_ = upstream.Close()
		}
		done <- struct{}{}
	}()
	go func() {
		copyWithIdle(client, upstream, upstream, idle)
		_ = client.Close()
		_ = upstream.Close()
		done <- struct{}{}
	}()
	<-done
	<-done
}

func copyWithIdle(dst io.Writer, src io.Reader, source net.Conn, idle time.Duration) {
	buf := make([]byte, 32<<10)
	for {
		_ = source.SetReadDeadline(time.Now().Add(idle))
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (b *Broker) dial(ctx context.Context, addr netip.AddrPort) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()
	if b.Dial != nil {
		return b.Dial(ctx, "tcp", addr.String())
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp", addr.String())
}

func (b *Broker) guard() SSRFGuard {
	return SSRFGuard{Resolver: b.Resolver, DenyPrefixes: b.DenyPrefixes}
}

func (b *Broker) limiter() *sshgateway.SessionLimiter {
	if b.Limits != nil {
		return b.Limits
	}
	b.limitsOnce.Do(func() {
		b.limits = sshgateway.NewSessionLimiter(defaultMaxConns, defaultMaxConnsPerPod)
	})
	return b.limits
}

func (b *Broker) ports() []int {
	if len(b.Ports) > 0 {
		return b.Ports
	}
	return DefaultPorts
}

func (b *Broker) idleTimeout() time.Duration {
	if b.IdleTimeout > 0 {
		return b.IdleTimeout
	}
	return defaultIdleTimeout
}

func (b *Broker) maxDuration() time.Duration {
	if b.MaxDuration > 0 {
		return b.MaxDuration
	}
	return defaultMaxDuration
}

func (b *Broker) logf(format string, args ...any) {
	if b.Logf != nil {
		b.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

type bufferedConn struct {
	net.Conn
	r io.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }
