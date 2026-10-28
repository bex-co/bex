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

// Package egressproxy is the Pod-bound egress proxy of the isolated SSH
// gateway (w10/m19, ADR047 D5 amendment 2026-10-28). A sandbox of a workspace
// whose agent environment runs the `full` network tier has no direct public
// egress at all; its HTTPS_PROXY/HTTP_PROXY point here. The proxy relays HTTP
// CONNECT tunnels and absolute-form plain-HTTP requests to any public host on
// the allowed ports, and refuses the model providers (by name and by TLS SNI,
// so model traffic keeps going through the bex model proxy) and every private,
// loopback, link-local, CGNAT, cluster and metadata address (SSRF guard).
//
// Trust model, identical to the Git and model proxies: the source Pod is
// resolved from the TCP source IP and bound to the claimed session by its
// immutable labels; bex-api then confirms the live session's workspace runs
// the `full` tier. The proxy URL's userinfo is only the session claim.
package egressproxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentcred"
)

// Defaults used when the corresponding Broker field is zero; cmd/ssh-gateway
// supplies the configured values.
const (
	defaultMaxConns          = 256
	defaultMaxConnsPerPod    = 32
	defaultIdleTimeout       = 5 * time.Minute
	defaultMaxDuration       = time.Hour
	defaultDialTimeout       = 10 * time.Second
	defaultHelloTimeout      = 10 * time.Second
	defaultGrantTTL          = 30 * time.Second
	defaultDeniedTTL         = 5 * time.Second
	defaultAuditInterval     = time.Minute
	defaultResponseHeaderTTL = time.Minute
	usageIdleTTL             = 24 * time.Hour
	maxClientHelloBytes      = 16 << 10
)

// DefaultPorts are the destination ports the proxy relays when none are
// configured: HTTPS and HTTP.
var DefaultPorts = []int{443, 80}

// Granter is bex-api's egress grant verb (*agentsession.EgressGrantClient).
type Granter interface {
	Grant(context.Context, agentsession.ModelMintRequest) (agentsession.EgressGrantResponse, error)
}

// Resolver resolves a public host name. *net.Resolver satisfies it.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Broker is the egress proxy. It is disabled unless both the Pod resolver and
// the grant client are wired, so an unconfigured gateway never starts it.
type Broker struct {
	Pods    agentcred.SessionPodResolver
	API     Granter
	Audit   core.AuditSink
	Metrics *sshgateway.Metrics
	// Limits bounds concurrent requests/tunnels globally and per source Pod
	// IP, held for a tunnel's whole life. nil gets 256 global / 32 per Pod.
	Limits *sshgateway.SessionLimiter
	// Resolver and Dial are seams; nil uses the gateway's resolver and a
	// plain TCP dialer. The proxy resolves once and dials the checked IP, so a
	// second (rebinding) resolution can never pick the address.
	Resolver Resolver
	Dial     func(ctx context.Context, network, address string) (net.Conn, error)
	// Ports are the allowed destination ports; empty means DefaultPorts.
	Ports []int
	// DenyPrefixes adds deployment-specific ranges (the cluster Pod/Service
	// CIDRs when they fall outside the built-in private ranges).
	DenyPrefixes []netip.Prefix
	// Cumulative budgets; <= 0 disables a dimension. Process-local: a gateway
	// restart resets them (they bound runaway loops, not billing).
	MaxConnectionsPerSession   int
	MaxConnectionsPerWorkspace int
	MaxBytesPerSession         int64
	// Per-tunnel bounds; zero uses the defaults (5m idle, 1h total).
	IdleTimeout time.Duration
	MaxDuration time.Duration
	// GrantTTL caches an allowed grant per Pod; DeniedTTL a refusal. Zero uses
	// 30s / 5s, so switching a workspace off `full` takes effect within 30s.
	GrantTTL  time.Duration
	DeniedTTL time.Duration
	// AuditInterval is how often per-session usage counters are flushed to the
	// audit log; zero uses one minute.
	AuditInterval time.Duration
	Now           func() time.Time

	limitsOnce sync.Once
	limits     *sshgateway.SessionLimiter

	grantsMu sync.Mutex
	grants   map[string]grantEntry

	usageMu   sync.Mutex
	usage     map[string]*sessionUsage
	auditOnce sync.Once
}

type grantEntry struct {
	allowed bool
	expires time.Time
}

// sessionUsage is one session's cumulative counters (pod is guarded by
// Broker.usageMu: a later sandbox generation of the session replaces it). connections and bytes
// enforce the budgets; the pending* pair is what the next audit flush reports.
type sessionUsage struct {
	workspace, sessionID, pod string
	connections               atomic.Int64
	bytes                     atomic.Int64
	pendingConns              atomic.Int64
	pendingBytes              atomic.Int64
	lastUsed                  atomic.Int64
}

// refusal is a request the proxy turns away: the HTTP status, the bounded
// metric reason, and the short message the sandbox sees.
type refusal struct {
	status  int
	reason  string
	message string
}

func (e *refusal) Error() string { return e.message }

func refuse(status int, reason, message string) *refusal {
	return &refusal{status: status, reason: reason, message: message}
}

func (b *Broker) Enabled() bool { return b != nil && b.Pods != nil && b.API != nil }

// Handler serves the egress listener. Origin-form requests are not proxy
// requests: only GET /healthz answers; everything else is a 400 naming the
// proxy, so a misconfigured client fails loudly.
func (b *Broker) Handler() http.Handler { return http.HandlerFunc(b.serve) }

func (b *Broker) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect && !r.URL.IsAbs() {
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "bex egress proxy: send proxy requests (CONNECT or absolute-form HTTP)", http.StatusBadRequest)
		return
	}
	if !b.Enabled() {
		http.Error(w, "bex egress proxy unavailable", http.StatusServiceUnavailable)
		return
	}
	namespace, sessionID, err := agentsession.ParseEgressProxyAuthorization(r.Header.Get("Proxy-Authorization"))
	if err != nil {
		b.Metrics.EgressProxyConnection("unauthenticated")
		w.Header().Set("Proxy-Authenticate", `Basic realm="bex-egress"`)
		http.Error(w, "bex egress proxy: proxy credentials required (use the HTTPS_PROXY URL as given)", http.StatusProxyAuthRequired)
		return
	}
	sourceIP, err := agentcred.RemoteIP(r.RemoteAddr)
	if err != nil {
		b.fail(w, refuse(http.StatusForbidden, "source", "forbidden"))
		return
	}
	limiter := b.limiter()
	if ok, scope := limiter.Acquire(sourceIP); !ok {
		b.Metrics.LimitRejected("egress_" + scope)
		b.fail(w, refuse(http.StatusTooManyRequests, "concurrency", "too many concurrent egress connections from this sandbox"))
		return
	}
	defer limiter.Release(sourceIP)

	usage, refused := b.admit(r.Context(), namespace, sessionID, sourceIP)
	if refused != nil {
		b.fail(w, refused)
		return
	}
	if r.Method == http.MethodConnect {
		b.serveConnect(w, r, usage)
		return
	}
	b.serveHTTP(w, r, usage)
}

// admit binds the request to its session sandbox and its `full` grant, then
// charges the cumulative connection budgets.
func (b *Broker) admit(ctx context.Context, namespace, sessionID, sourceIP string) (*sessionUsage, *refusal) {
	pod, err := b.Pods.ResolveSessionPod(ctx, namespace, sourceIP)
	if err != nil {
		b.Metrics.Authentication("rejected_key")
		return nil, refuse(http.StatusForbidden, "pod", "forbidden")
	}
	verified, err := agentsession.AuthorizeSessionPod(namespace, pod.Name, pod.UID, pod.Labels, sessionID)
	if err != nil {
		b.Metrics.Authentication("rejected_target")
		return nil, refuse(http.StatusForbidden, "pod", "forbidden")
	}
	if refused := b.checkGrant(ctx, namespace, verified); refused != nil {
		return nil, refused
	}
	usage := b.sessionUsage(namespace, verified)
	if max := b.MaxConnectionsPerSession; max > 0 && usage.connections.Load() >= int64(max) {
		b.Metrics.LimitRejected("egress_session_budget")
		return nil, refuse(http.StatusTooManyRequests, "budget", "egress connection budget exhausted for this session")
	}
	if max := b.MaxBytesPerSession; max > 0 && usage.bytes.Load() >= max {
		b.Metrics.LimitRejected("egress_session_bytes")
		return nil, refuse(http.StatusTooManyRequests, "budget", "egress byte budget exhausted for this session")
	}
	if max := b.MaxConnectionsPerWorkspace; max > 0 && b.workspaceConnections(verified.Workspace) >= int64(max) {
		b.Metrics.LimitRejected("egress_workspace_budget")
		return nil, refuse(http.StatusTooManyRequests, "budget", "egress connection budget exhausted for this workspace")
	}
	usage.connections.Add(1)
	usage.pendingConns.Add(1)
	return usage, nil
}

// checkGrant asks bex-api (cached per Pod) whether the live session's
// workspace runs the full tier.
func (b *Broker) checkGrant(ctx context.Context, namespace string, verified agentsession.ModelMintRequest) *refusal {
	key := namespace + "|" + verified.SessionID + "|" + verified.PodUID
	now := b.now()
	b.grantsMu.Lock()
	entry, ok := b.grants[key]
	b.grantsMu.Unlock()
	if !ok || !now.Before(entry.expires) {
		_, err := b.API.Grant(ctx, verified)
		switch {
		case err == nil:
			entry = grantEntry{allowed: true, expires: now.Add(durationOr(b.GrantTTL, defaultGrantTTL))}
		case errors.Is(err, agentsession.ErrForbidden):
			entry = grantEntry{allowed: false, expires: now.Add(durationOr(b.DeniedTTL, defaultDeniedTTL))}
		default:
			// An unreachable bex-api is not cached: the next request retries.
			log.Printf("agent egress proxy: grant unavailable (session=%s): %v", verified.SessionID, err)
			return refuse(http.StatusBadGateway, "grant_unavailable", "bex egress proxy unavailable")
		}
		b.grantsMu.Lock()
		if b.grants == nil {
			b.grants = make(map[string]grantEntry)
		}
		for k, e := range b.grants {
			if !now.Before(e.expires) {
				delete(b.grants, k)
			}
		}
		b.grants[key] = entry
		b.grantsMu.Unlock()
	}
	if !entry.allowed {
		return refuse(http.StatusForbidden, "tier", "this workspace's agent network tier does not include the internet egress proxy")
	}
	return nil
}

// target validates the requested authority and returns the one checked
// address the proxy dials. Every refusal is a 403 except a resolution failure.
func (b *Broker) target(ctx context.Context, host, rawPort string) (netip.AddrPort, string, *refusal) {
	port, err := strconv.Atoi(rawPort)
	if err != nil || port <= 0 || port > 65535 || !slices.Contains(b.ports(), port) {
		return netip.AddrPort{}, "", refuse(http.StatusForbidden, "port", "bex egress proxy: only ports "+b.portList()+" are allowed")
	}
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return netip.AddrPort{}, "", refuse(http.StatusBadRequest, "invalid", "bex egress proxy: missing host")
	}
	var candidates []netip.Addr
	if literal, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		candidates = []netip.Addr{literal.Unmap()}
		host = ""
	} else {
		if blockedName(host) {
			return netip.AddrPort{}, "", refuse(http.StatusForbidden, "provider", "bex egress proxy: model-provider APIs are reachable only through the bex model proxy")
		}
		if !strings.Contains(host, ".") || sessionegress.PrivateOrClusterHost(host) || strings.HasSuffix(host, ".svc") {
			return netip.AddrPort{}, "", refuse(http.StatusForbidden, "private", "bex egress proxy: private and cluster-local names are not reachable")
		}
		// The trailing dot makes the name absolute, so the gateway's cluster
		// search domains never expand it into an in-cluster name.
		addrs, err := b.resolver().LookupNetIP(ctx, "ip", host+".")
		if err != nil || len(addrs) == 0 {
			return netip.AddrPort{}, "", refuse(http.StatusBadGateway, "dns", "bex egress proxy: could not resolve "+host)
		}
		for _, addr := range addrs {
			candidates = append(candidates, addr.Unmap())
		}
	}
	// Every resolved address must pass, not just the one dialed: a mixed
	// answer is how a rebinding name smuggles a private address in.
	for _, addr := range candidates {
		if b.blockedAddr(addr) {
			return netip.AddrPort{}, "", refuse(http.StatusForbidden, "private", "bex egress proxy: private, cluster and metadata addresses are not reachable")
		}
	}
	chosen := candidates[0]
	for _, addr := range candidates {
		if addr.Is4() {
			chosen = addr
			break
		}
	}
	return netip.AddrPortFrom(chosen, uint16(port)), host, nil
}

func (b *Broker) serveConnect(w http.ResponseWriter, r *http.Request, usage *sessionUsage) {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		b.fail(w, refuse(http.StatusBadRequest, "invalid", "bex egress proxy: CONNECT needs host:port"))
		return
	}
	addr, name, refused := b.target(r.Context(), host, port)
	if refused != nil {
		b.fail(w, refused)
		return
	}
	upstream, err := b.dial(r.Context(), addr)
	if err != nil {
		b.fail(w, refuse(http.StatusBadGateway, "dial", "bex egress proxy: could not connect to "+r.Host))
		return
	}
	defer upstream.Close()
	client, buffered, err := http.NewResponseController(w).Hijack()
	if err != nil {
		b.fail(w, refuse(http.StatusInternalServerError, "hijack", "bex egress proxy unavailable"))
		return
	}
	defer client.Close()
	_ = client.SetDeadline(time.Time{})
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	// The tunnel must open with a TLS ClientHello whose SNI is neither a
	// provider nor, for a named CONNECT, a different host: that closes CDN
	// domain fronting (CONNECT an allowed name on a shared edge, then ask it
	// for a provider) at the one layer the proxy can see.
	prefix, refused := b.checkClientHello(client, buffered.Reader, name)
	if refused != nil {
		b.Metrics.EgressProxyConnection(refused.reason)
		return
	}
	b.Metrics.EgressProxyConnection("allowed")
	b.splice(client, io.MultiReader(bytes.NewReader(prefix), buffered.Reader), upstream, usage)
}

// checkClientHello reads the first TLS record from the client and returns the
// bytes it consumed, for replay upstream.
func (b *Broker) checkClientHello(client net.Conn, reader *bufio.Reader, connectHost string) ([]byte, *refusal) {
	_ = client.SetReadDeadline(time.Now().Add(defaultHelloTimeout))
	defer func() { _ = client.SetReadDeadline(time.Time{}) }()
	first, err := reader.Peek(1)
	if err != nil || first[0] != 0x16 {
		return nil, refuse(0, "not_tls", "")
	}
	recorder := &recordingReader{r: io.LimitReader(reader, maxClientHelloBytes)}
	sni, sawHello := "", false
	stop := errors.New("client hello captured")
	_ = tls.Server(readOnlyConn{Conn: client, r: recorder}, &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			sni, sawHello = strings.TrimSuffix(strings.ToLower(hello.ServerName), "."), true
			return nil, stop
		},
	}).Handshake()
	if !sawHello {
		return nil, refuse(0, "not_tls", "")
	}
	if sni != "" && (blockedName(sni) || (connectHost != "" && sni != connectHost)) {
		if blockedName(sni) {
			return nil, refuse(0, "provider", "")
		}
		return nil, refuse(0, "sni", "")
	}
	return recorder.buf.Bytes(), nil
}

// serveHTTP forwards one absolute-form plain-HTTP request. The upstream Host
// is always the request URL's own authority (never a separate Host header),
// the dial goes to the checked address, and redirects are relayed, not
// followed.
func (b *Broker) serveHTTP(w http.ResponseWriter, r *http.Request, usage *sessionUsage) {
	if r.URL.Scheme != "http" {
		b.fail(w, refuse(http.StatusBadRequest, "invalid", "bex egress proxy: use CONNECT for "+r.URL.Scheme+" URLs"))
		return
	}
	port := r.URL.Port()
	if port == "" {
		port = "80"
	}
	addr, _, refused := b.target(r.Context(), r.URL.Hostname(), port)
	if refused != nil {
		b.fail(w, refused)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), b.maxDuration())
	defer cancel()
	out := r.Clone(ctx)
	out.RequestURI = ""
	out.Host = r.URL.Host
	removeHopHeaders(out.Header)
	if r.Body != nil && r.Body != http.NoBody {
		out.Body = &countingReader{r: r.Body, usage: usage, broker: b, direction: "upstream"}
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return b.dial(ctx, addr)
		},
		DisableKeepAlives:      true,
		DisableCompression:     true,
		ResponseHeaderTimeout:  defaultResponseHeaderTTL,
		MaxResponseHeaderBytes: 1 << 20,
	}
	defer transport.CloseIdleConnections()
	response, err := transport.RoundTrip(out)
	if err != nil {
		if errors.Is(err, errBudget) {
			b.fail(w, refuse(http.StatusTooManyRequests, "budget", "egress byte budget exhausted for this session"))
			return
		}
		b.fail(w, refuse(http.StatusBadGateway, "dial", "bex egress proxy: upstream request failed"))
		return
	}
	defer response.Body.Close()
	b.Metrics.EgressProxyConnection("allowed")
	removeHopHeaders(response.Header)
	for name, values := range response.Header {
		w.Header()[name] = values
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, &countingReader{r: response.Body, usage: usage, broker: b, direction: "downstream"})
}

var errBudget = errors.New("egress byte budget exhausted")

// splice relays both directions under the idle timeout, the total lifetime
// and the session byte budget.
func (b *Broker) splice(client net.Conn, clientReader io.Reader, upstream net.Conn, usage *sessionUsage) {
	idle := durationOr(b.IdleTimeout, defaultIdleTimeout)
	lifetime := time.AfterFunc(b.maxDuration(), func() {
		_ = client.Close()
		_ = upstream.Close()
	})
	defer lifetime.Stop()
	done := make(chan struct{}, 2)
	go func() {
		b.copyIdle(upstream, clientReader, client, idle, usage, "upstream")
		// The sandbox finished sending: half-close upstream so the response
		// can still drain.
		if closer, ok := upstream.(interface{ CloseWrite() error }); ok {
			_ = closer.CloseWrite()
		} else {
			_ = upstream.Close()
		}
		done <- struct{}{}
	}()
	go func() {
		b.copyIdle(client, upstream, upstream, idle, usage, "downstream")
		_ = client.Close()
		_ = upstream.Close()
		done <- struct{}{}
	}()
	<-done
	<-done
}

// copyIdle copies src to dst, refreshing source's read deadline before every
// read so a silent tunnel ends after idle, and stopping at the byte budget.
func (b *Broker) copyIdle(dst io.Writer, src io.Reader, source net.Conn, idle time.Duration, usage *sessionUsage, direction string) {
	buf := make([]byte, 32<<10)
	for {
		_ = source.SetReadDeadline(time.Now().Add(idle))
		n, err := src.Read(buf)
		if n > 0 {
			if !b.chargeBytes(usage, int64(n), direction) {
				_ = source.Close()
				return
			}
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// chargeBytes adds n to the session's byte counters and reports whether the
// session is still within its byte budget.
func (b *Broker) chargeBytes(usage *sessionUsage, n int64, direction string) bool {
	total := usage.bytes.Add(n)
	usage.pendingBytes.Add(n)
	usage.lastUsed.Store(b.now().Unix())
	b.Metrics.EgressProxyBytes(direction, n)
	if max := b.MaxBytesPerSession; max > 0 && total > max {
		b.Metrics.LimitRejected("egress_session_bytes")
		return false
	}
	return true
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

func (b *Broker) fail(w http.ResponseWriter, r *refusal) {
	b.Metrics.EgressProxyConnection(r.reason)
	http.Error(w, r.message, r.status)
}

// blockedName reports a model-provider host or any name under one.
func blockedName(host string) bool {
	for _, provider := range sessionegress.ModelProviderDomains() {
		if host == provider || strings.HasSuffix(host, "."+provider) {
			return true
		}
	}
	return false
}

// deniedPrefixes are never reachable through the proxy: unspecified, private,
// loopback, link-local (incl. the 169.254.169.254 metadata service), CGNAT,
// IETF/benchmark/documentation, multicast and reserved space, IPv6 ULA,
// link-local, multicast, NAT64, Teredo, 6to4 and discard prefixes (the
// tunnelling prefixes embed an IPv4 address that could be private).
var deniedPrefixes = func() []netip.Prefix {
	var out []netip.Prefix
	for _, raw := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
		"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16",
		"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
		"::/128", "::1/128", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001:db8::/32",
		"2001::/32", "2002::/16", "fc00::/7", "fe80::/10", "ff00::/8",
	} {
		out = append(out, netip.MustParsePrefix(raw))
	}
	return out
}()

func (b *Broker) blockedAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() || addr.IsInterfaceLocalMulticast() {
		return true
	}
	for _, prefix := range deniedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	for _, prefix := range b.DenyPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (b *Broker) sessionUsage(namespace string, verified agentsession.ModelMintRequest) *sessionUsage {
	key := namespace + "|" + verified.SessionID
	b.usageMu.Lock()
	defer b.usageMu.Unlock()
	if b.usage == nil {
		b.usage = make(map[string]*sessionUsage)
	}
	usage, ok := b.usage[key]
	if !ok {
		usage = &sessionUsage{workspace: verified.Workspace, sessionID: verified.SessionID}
		b.usage[key] = usage
	}
	usage.pod = verified.PodName
	usage.lastUsed.Store(b.now().Unix())
	b.auditOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(durationOr(b.AuditInterval, defaultAuditInterval))
			defer ticker.Stop()
			for range ticker.C {
				b.FlushAudit()
			}
		}()
	})
	return usage
}

func (b *Broker) workspaceConnections(workspace string) int64 {
	b.usageMu.Lock()
	defer b.usageMu.Unlock()
	var total int64
	for _, usage := range b.usage {
		if usage.workspace == workspace {
			total += usage.connections.Load()
		}
	}
	return total
}

// FlushAudit records one content-free audit event per session with activity
// since the last flush (connection and byte counts only, never hosts or
// URLs), and forgets sessions idle for a day.
func (b *Broker) FlushAudit() {
	now := b.now()
	b.usageMu.Lock()
	type flush struct {
		usage *sessionUsage
		pod   string
	}
	var pending []flush
	for key, usage := range b.usage {
		if usage.pendingConns.Load() > 0 || usage.pendingBytes.Load() > 0 {
			pending = append(pending, flush{usage: usage, pod: usage.pod})
		} else if now.Sub(time.Unix(usage.lastUsed.Load(), 0)) > usageIdleTTL {
			delete(b.usage, key)
		}
	}
	b.usageMu.Unlock()
	for _, item := range pending {
		usage := item.usage
		conns, bytesRelayed := usage.pendingConns.Swap(0), usage.pendingBytes.Swap(0)
		core.RecordAuditEvent(context.Background(), b.Audit, core.AuditEvent{
			Caller: item.pod, CallerMethod: "sandbox", Verb: agentsession.AuditVerbEgressUsage,
			Resource: core.WorkspaceObject(usage.workspace), Target: "agent-session:" + usage.sessionID,
			TargetName: fmt.Sprintf("connections=%d bytes=%d", conns, bytesRelayed),
			Outcome:    core.AuditAllowed, At: now,
		})
	}
}

func (b *Broker) limiter() *sshgateway.SessionLimiter {
	if b.Limits != nil {
		return b.Limits
	}
	b.limitsOnce.Do(func() { b.limits = sshgateway.NewSessionLimiter(defaultMaxConns, defaultMaxConnsPerPod) })
	return b.limits
}

func (b *Broker) resolver() Resolver {
	if b.Resolver != nil {
		return b.Resolver
	}
	return net.DefaultResolver
}

func (b *Broker) ports() []int {
	if len(b.Ports) > 0 {
		return b.Ports
	}
	return DefaultPorts
}

func (b *Broker) portList() string {
	parts := make([]string, 0, len(b.ports()))
	for _, port := range b.ports() {
		parts = append(parts, strconv.Itoa(port))
	}
	return strings.Join(parts, ", ")
}

func (b *Broker) maxDuration() time.Duration { return durationOr(b.MaxDuration, defaultMaxDuration) }

func (b *Broker) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func durationOr(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

// hopHeaders never cross the proxy (RFC 9110 §7.6.1), plus the session claim.
var hopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
	"Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

func removeHopHeaders(header http.Header) {
	for _, field := range header.Values("Connection") {
		for _, name := range strings.Split(field, ",") {
			if name = strings.TrimSpace(name); name != "" {
				header.Del(name)
			}
		}
	}
	for _, name := range hopHeaders {
		header.Del(name)
	}
}

// countingReader charges a plain-HTTP body against the session byte budget.
type countingReader struct {
	r         io.Reader
	usage     *sessionUsage
	broker    *Broker
	direction string
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 && !c.broker.chargeBytes(c.usage, int64(n), c.direction) {
		return n, errBudget
	}
	return n, err
}

func (c *countingReader) Close() error {
	if closer, ok := c.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// recordingReader keeps every byte it reads, for replaying the ClientHello.
type recordingReader struct {
	r   io.Reader
	buf bytes.Buffer
}

func (r *recordingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.buf.Write(p[:n])
	return n, err
}

// readOnlyConn feeds crypto/tls the recorded client bytes and refuses every
// write, so capturing the ClientHello can never answer the client.
type readOnlyConn struct {
	net.Conn
	r io.Reader
}

func (c readOnlyConn) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c readOnlyConn) Write(p []byte) (int, error) { return 0, io.ErrClosedPipe }
