// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package clientipheaders is a Traefik local HTTP middleware on the tenant
// entrypoints (w8/m123). On Render every web service sits behind Cloudflare,
// which overwrites CF-Connecting-IP and True-Client-IP with the real client,
// and Render tells apps to trust them. Traefik rewrites only its own
// X-Forwarded-*/X-Real-IP set, so without this an app trusted whatever the
// caller sent. Every request leaves here with True-Client-IP naming the real
// client: the TCP peer (after PROXY protocol), or Cloudflare's CF-Connecting-IP
// when the peer is a Cloudflare edge — a tenant custom domain behind its own
// Cloudflare proxy.
//
// A request from any other peer gets the Cloudflare headers a Render app
// receives (w4/m687, docs/render-artifacts/edge-headers.md): CF-Connecting-IP,
// CF-IPCountry (country.go), CF-Ray, CF-Visitor and CDN-Loop, whatever the
// caller sent, and its answer carries the same cf-ray. Render's edge refuses a
// caller's own CF-Connecting-IP from outside Cloudflare with 403 "error code:
// 1000"; this edge replaces it, because tenants' reverse proxies on bex pass
// Cloudflare's header on to other bex services (w4/m687 t004).
//
// It also gives every request Render's Rndr-Id (w8/m125): a fresh id on the
// request to the app and on the response, which the access log keeps, so a
// request line and the app lines that log the header share one searchable
// value. A caller's Rndr-Id is replaced, never trusted.
//
// And Render's X-Request-Start (w8/196): the time the request reached the edge,
// in microseconds since the Unix epoch, which New Relic, AppSignal, Scout and
// Judoscale read to report request-queue time (Judoscale autoscales on it). A
// caller's value is replaced, never trusted.
//
// Traefik's entrypoints keep a caller's X-Forwarded-* (w8/402), so that
// X-Forwarded-For reaches the app as the caller's list with the client
// appended, as on Render. This sanitizer rewrites or removes every other
// forwarded header, so the app sees what Traefik set for an untrusted caller.
//
// A second instance, configured with originServer, sits in each web service's
// own route (w8/321): Render's edge never hands a web service's Server header
// to clients. It moves the value to X-Render-Origin-Server, and names "Render"
// there when the app sent none. Static sites and platform hosts carry no
// such instance, as Render's static sites carry no such header. It also
// answers with the request's cf-ray whatever the app sent, and with
// cf-cache-status DYNAMIC when the service's edge cache did not set one, as
// Render's edge does (w4/m687).
//
// A third, configured with httpsRedirect, leads every tenant host's route
// and the unknown-host fallback (w8/357): it redirects plain HTTP to HTTPS
// with Render's status codes, which Traefik's own redirectScheme cannot send.
//
// A fourth, configured with staticSite, sits in each static site's route
// (w8/363): it tells the static server which requests came with no
// Accept-Encoding, which Traefik's forwarder otherwise hides by asking for
// gzip itself.
package bex_client_ip_headers

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// cloudflareRanges is Cloudflare's published edge address space. It must equal
// lego/backend/internal/core/proxiedhosts.go's cloudflareRanges;
// scripts/gitops-validate.sh fails when the two drift.
var cloudflareRanges = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
	"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
}

// cloudflareHeaders are the client-describing headers Cloudflare sets; only a
// Cloudflare peer may pass them on, and the edge sets its own for any other.
// Keys are lowercase with "-" for "_": CGI-style frameworks (Rack, PHP, WSGI)
// read True_Client_IP and True-Client-IP as the same HTTP_TRUE_CLIENT_IP, so
// both spellings count. CDN-Loop is not here: Cloudflare keeps a caller's
// list and adds itself (cdnLoop).
var cloudflareHeaders = map[string]bool{
	"cf-connecting-ip": true, "cf-connecting-ipv6": true, "cf-ipcountry": true,
	"cf-ray": true, "cf-visitor": true, "cf-worker": true,
}

// alwaysStripped never comes from a party bex trusts; True-Client-IP, Rndr-Id
// and X-Request-Start are rewritten below.
var alwaysStripped = map[string]bool{"x-client-ip": true, "true-client-ip": true, "rndr-id": true, "x-request-start": true}

// forwardedHeaders are the headers Traefik's entrypoint manages (its
// forwardedheaders.XHeadersSet), which the tenant entrypoints now take from
// any caller (forwardedHeaders.insecure, w8/402) so that X-Forwarded-For
// keeps the caller's list. false marks a header removed, as Traefik removed it
// from an untrusted caller; true one kept, because it is X-Forwarded-For,
// setForwarded rewrites it, or Traefik always overwrites it
// (X-Forwarded-Server). Every underscore spelling is removed.
var forwardedHeaders = map[string]bool{
	"x-forwarded-for": true, "x-forwarded-proto": true, "x-forwarded-host": true,
	"x-forwarded-port": true, "x-forwarded-server": true, "x-real-ip": true,
	"x-forwarded-uri": false, "x-forwarded-method": false, "x-forwarded-prefix": false,
	"x-forwarded-tls-client-cert": false, "x-forwarded-tls-client-cert-info": false,
}

type Config struct {
	// Colo is the three-letter data-center code that ends every CF-Ray the
	// entrypoint's sanitizer mints, as Cloudflare names the data center that
	// handled a request: the IATA code of Cloudflare's nearest to the cluster.
	Colo string `json:"colo,omitempty"`
	// OriginServer makes this instance the per-web-service Server rename
	// (w8/321) instead of the entrypoint's request sanitizer.
	OriginServer bool `json:"originServer,omitempty"`
	// HTTPSRedirect makes this instance a route's HTTP-to-HTTPS redirect
	// (w8/357) instead.
	HTTPSRedirect bool `json:"httpsRedirect,omitempty"`
	// StaticSite makes this instance a static site's no-Accept-Encoding
	// marker (w8/363) instead.
	StaticSite bool `json:"staticSite,omitempty"`
}

// defaultColo is Frankfurt, Cloudflare's nearest data center to bex's
// Falkenstein and Nuremberg regions.
const defaultColo = "FRA"

func CreateConfig() *Config { return &Config{Colo: defaultColo} }

type sanitizer struct {
	next       http.Handler
	cloudflare []*net.IPNet
	colo       string
}

func New(_ context.Context, next http.Handler, config *Config, _ string) (http.Handler, error) {
	if config != nil && config.HTTPSRedirect {
		return &httpsRedirect{next: next}, nil
	}
	if config != nil && config.OriginServer {
		return &originServer{next: next}, nil
	}
	if config != nil && config.StaticSite {
		return &staticSite{next: next}, nil
	}
	s := &sanitizer{next: next, colo: defaultColo}
	if config != nil && config.Colo != "" {
		s.colo = config.Colo
	}
	startCountries()
	for _, cidr := range cloudflareRanges {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		s.cloudflare = append(s.cloudflare, n)
	}
	return s, nil
}

func (s *sanitizer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := r.Header
	peer := peerIP(r.RemoteAddr)
	// Ordinary traffic carries no CF-Connecting-IP, so it skips the range scan.
	var client net.IP
	claimed := h.Get("CF-Connecting-IP")
	fromCloudflare := claimed != "" && peer != nil && s.fromCloudflare(peer)
	if fromCloudflare {
		client = net.ParseIP(claimed)
	}
	// Render's edge also refuses a CF-Connecting-IP from outside Cloudflare.
	// This one replaces it: a tenant's Caddy in front of other bex services
	// forwards Cloudflare's header, and refusing it took those sites down on
	// 2026-11-27 (w4/m687 t004, parked for the user's decision).
	refused := false
	for name := range h {
		key := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		// A trusted edge's CF-* stay only in Cloudflare's own hyphenated form.
		if alwaysStripped[key] || (cloudflareHeaders[key] && (client == nil || strings.Contains(name, "_"))) {
			delete(h, name)
		}
		if keep, ok := forwardedHeaders[key]; ok && (!keep || strings.Contains(name, "_")) {
			delete(h, name)
		}
	}
	setForwarded(r)
	id, ray := requestIDs(s.colo)
	switch {
	case client != nil:
		// Cloudflare's own CF-* and CDN-Loop stay as it sent them.
		h.Set("CF-Connecting-IP", client.String())
		h.Set("True-Client-IP", client.String())
		ray = h.Get("CF-Ray")
	default:
		if peer != nil {
			h.Set("True-Client-IP", peer.String())
			h.Set("CF-Connecting-IP", peer.String())
			h.Set("CF-IPCountry", country(peer))
		}
		h.Set("CF-Ray", ray)
		// The scheme the client reached the edge with, never a caller's.
		visitor := `{"scheme":"http"}`
		if r.TLS != nil {
			visitor = `{"scheme":"https"}`
		}
		h.Set("CF-Visitor", visitor)
		loop, looped := cdnLoop(h.Values("CDN-Loop"))
		h.Set("CDN-Loop", loop)
		refused = refused || looped
	}
	// Set before the app answers: an app that sends its own Rndr-Id adds a
	// second value after ours, and Header.Get still reads ours.
	h.Set("Rndr-Id", id)
	// Render's format: bare microseconds since the epoch, no "t=" prefix. Taken
	// at the edge, before any activator wake, so queue time includes the
	// spin-up, as on Render.
	h.Set("X-Request-Start", strconv.FormatInt(time.Now().UnixMicro(), 10))
	if ray != "" {
		w.Header().Set("Cf-Ray", ray)
	}
	// Cloudflare refuses a loop before Render's edge sees the request, so its
	// answer has no Rndr-Id. The access log still records the request's, and
	// the answer's cf-ray carries the same hex.
	if refused {
		refuse(w)
		return
	}
	w.Header().Set("Rndr-Id", id)
	s.next.ServeHTTP(w, r)
}

// refusalBody is Cloudflare's answer to a request it refuses as a loop,
// error 1000, byte for byte as Render's edge sent it (captured 2026-11-27).
const refusalBody = "error code: 1000\n"

// refuse answers as Render's edge answered a CDN-Loop past maxCDNLoops (and a
// forged CF-Connecting-IP): 403, plain text, never cached. Traefik's entrypoint pre-sets a nil Content-Type
// to stop net/http's sniffing, which Set replaces.
func refuse(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=UTF-8")
	h.Set("Content-Length", strconv.Itoa(len(refusalBody)))
	h.Set("Cache-Control", "private, max-age=0, no-store, no-cache, must-revalidate, post-check=0, pre-check=0")
	h.Set("Expires", "Thu, 01 Jan 1970 00:00:01 GMT")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("X-Frame-Options", "SAMEORIGIN")
	w.WriteHeader(http.StatusForbidden)
	_, _ = io.WriteString(w, refusalBody)
}

// maxCDNLoops is the most Cloudflare hops Render's edge let a request claim:
// a caller's "cloudflare; loops=15" reached the app as loops=16, and loops=16
// was refused with error 1000 (captured 2026-11-27).
const maxCDNLoops = 16

// cdnLoop is the CDN-Loop (RFC 8586) the app receives, as Cloudflare writes
// it on Render: the caller's list with each member re-joined by "; ", every
// "cloudflare" member's loops counted up by one (a missing or unreadable
// count reads as none), or "cloudflare; loops=1" added when no member names
// Cloudflare. looped reports a count past maxCDNLoops, which Cloudflare
// refuses.
func cdnLoop(values []string) (loop string, looped bool) {
	if len(values) == 0 { // nearly every request: spare the interpreter the parse
		return "cloudflare; loops=1", false
	}
	var members []string
	found := false
	for _, member := range strings.Split(strings.Join(values, ","), ",") {
		params := strings.Split(member, ";")
		for i := range params {
			params[i] = strings.TrimSpace(params[i])
		}
		if params[0] == "" && len(params) == 1 {
			continue
		}
		if strings.EqualFold(params[0], "cloudflare") {
			found = true
			loops, at := 0, len(params)
			for i, p := range params[1:] {
				if name, value, ok := strings.Cut(p, "="); ok && strings.EqualFold(strings.TrimSpace(name), "loops") {
					loops, _ = strconv.Atoi(strings.TrimSpace(value))
					at = i + 1
					break
				}
			}
			if loops < 0 {
				loops = 0
			}
			loops++
			looped = looped || loops > maxCDNLoops
			count := "loops=" + strconv.Itoa(loops)
			if at == len(params) {
				params = append(params, count)
			} else {
				params[at] = count
			}
		}
		members = append(members, strings.Join(params, "; "))
	}
	if !found {
		members = append(members, "cloudflare; loops=1")
	}
	return strings.Join(members, ", "), looped
}

// setForwarded writes what Traefik's entrypoint wrote for an untrusted caller
// (forwardedheaders rewrite, v3.7.17), now that it keeps a caller's values:
// the scheme from the TLS state, ws or wss on a WebSocket upgrade; the port
// from the Host, else the scheme's; the Host; and the TCP peer, after PROXY
// protocol, as X-Real-Ip, in Traefik's raw form (peerIP's normalizing would
// change it for a v4-mapped peer).
func setForwarded(r *http.Request) {
	h := r.Header
	proto, port := "http", "80"
	if r.TLS != nil {
		proto, port = "https", "443"
	}
	if hasToken(h.Get("Connection"), "upgrade") && hasToken(h.Get("Upgrade"), "websocket") {
		proto = strings.Replace(proto, "http", "ws", 1)
	}
	h.Set("X-Forwarded-Proto", proto)
	if _, p, err := net.SplitHostPort(r.Host); err == nil && p != "" {
		port = p
	}
	h.Set("X-Forwarded-Port", port)
	if r.Host != "" {
		h.Set("X-Forwarded-Host", r.Host)
	} else {
		h.Del("X-Forwarded-Host")
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host, _, _ = strings.Cut(host, "%")
		h.Set("X-Real-Ip", host)
	} else {
		h.Del("X-Real-Ip")
	}
}

// hasToken reports whether the comma-separated list holds token, ignoring
// case, as Traefik's isWebsocketRequest reads Connection and Upgrade.
func hasToken(list, token string) bool {
	for _, v := range strings.Split(list, ",") {
		if strings.EqualFold(strings.TrimSpace(v), token) {
			return true
		}
	}
	return false
}

// requestIDs mints one random value in two shapes: Render's Rndr-Id, 8-4-4
// lowercase hex, 18 characters, as in its "303516bf-4995-4cb9" (w4/583), and
// Cloudflare's CF-Ray, the same 16 hex characters, "-" and the data center, as
// in "a60d86a06ea867d9-CMH". So a cf-ray a client quotes names the Rndr-Id the
// request log keeps.
func requestIDs(colo string) (id, ray string) {
	var b [8]byte
	_, _ = rand.Read(b[:]) // never fails on supported platforms (Go 1.24+)
	x := hex.EncodeToString(b[:])
	return x[0:8] + "-" + x[8:12] + "-" + x[12:16], x + "-" + colo
}

func (s *sanitizer) fromCloudflare(ip net.IP) bool {
	for _, n := range s.cloudflare {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// peerIP is the request's TCP peer, which Traefik has already replaced with
// the PROXY-protocol client on the load-balanced entrypoints. nil when
// RemoteAddr is not an address.
func peerIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}

// httpsRedirect sends a plain-HTTP request to the same URL on https with
// Render's codes (captured 2026-11-20): 301 for GET and HEAD, and a
// temporary 307 for every other method, which keeps the method and body and
// is not cached. Render's answer is Go's http.Redirect, body included.
// Traefik's redirectScheme answers every method but GET with 308.
//
// Like that middleware, it reads the scheme from the TLS state, overridden
// by X-Forwarded-Proto, which the entrypoint's sanitizer sets from the TLS
// state in place of a caller's (setForwarded). The Location drops any port,
// so HTTPS whose Host names a port other than 443 redirects too.
type httpsRedirect struct{ next http.Handler }

func (h *httpsRedirect) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, port := r.Host, ""
	if i := strings.LastIndexByte(host, ':'); i > strings.LastIndexByte(host, ']') {
		host, port = host[:i], host[i+1:]
	}
	if !plainHTTP(r) && (port == "" || port == "443") {
		h.next.ServeHTTP(w, r)
		return
	}
	uri := r.RequestURI
	if !strings.HasPrefix(uri, "/") { // absolute-form: keep its path and query
		uri = r.URL.RequestURI()
	}
	target := "https://" + host + uri
	if u, err := url.Parse(target); err == nil {
		target = u.String()
	}
	code := http.StatusTemporaryRedirect
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		code = http.StatusMovedPermanently
	}
	// Traefik's entrypoint pre-sets a nil Content-Type to stop net/http's
	// sniffing, which http.Redirect would take for one already chosen and
	// leave out its own, with its body.
	delete(w.Header(), "Content-Type")
	http.Redirect(w, r, target, code)
}

func plainHTTP(r *http.Request) bool {
	switch strings.ToLower(r.Header.Get("X-Forwarded-Proto")) {
	case "http", "ws":
		return true
	case "https", "wss":
		return false
	}
	return r.TLS == nil
}

// noAcceptEncodingHeader marks a static-site request whose client sent no
// Accept-Encoding. It must equal lego/operator/internal/staticserver's
// noAcceptEncodingHeader; scripts/gitops-validate.sh fails when the two drift.
const noAcceptEncodingHeader = "X-Bex-No-Accept-Encoding"

// staticSite marks a request with no Accept-Encoding for the static server,
// and drops a caller's own marker. Traefik's forwarder, Go's http.Transport,
// asks for gzip on a GET with no Accept-Encoding and no Range and decodes the
// answer itself, so the static server sees a gzip client. On Render such a
// client gets an icon identity with its strong tag, Content-Length and
// Accept-Ranges (captured 2026-11-21, w8/363), and the static server needs
// the marker to answer it that way.
type staticSite struct{ next http.Handler }

func (s *staticSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Header.Del(noAcceptEncodingHeader)
	if r.Header.Get("Accept-Encoding") == "" {
		r.Header.Set(noAcceptEncodingHeader, "1")
	}
	s.next.ServeHTTP(w, r)
}

// originServerHeader is where Render's edge puts a web service's own Server
// header, and "Render" when the app sent none (captured 2026-11-14: a uvicorn
// app answers "x-render-origin-server: uvicorn", an Express app "Render").
const originServerHeader = "X-Render-Origin-Server"

// proxyFlushInterval is Traefik's default responseForwarding.flushInterval:
// how often its proxy flushes a body of known length.
const proxyFlushInterval = 100 * time.Millisecond

type originServer struct{ next http.Handler }

func (o *originServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ow := &originServerWriter{ResponseWriter: w, rc: http.NewResponseController(w), ray: r.Header.Get("Cf-Ray")}
	o.next.ServeHTTP(ow, r)
	ow.mu.Lock()
	defer ow.mu.Unlock()
	// A handler that wrote nothing gets its response from net/http after it
	// returns, so the move still lands; after a write it changes nothing.
	if !ow.moved {
		ow.moveServer(http.StatusOK)
	}
	ow.stop()
}

// originServerWriter moves the app's Server header just before the response
// head is written. A WebSocket upgrade passes through untouched: the proxy
// writes its 101 on the hijacked connection, and Render's answer there was
// not captured.
//
// Under Traefik's Yaegi interpreter, compiled code sees an interpreted
// ResponseWriter only through Yaegi's wrapper, which keeps Hijack but drops
// Flush and Unwrap (yaegi stdlib/wrapper-composed.go). The proxy behind this
// writer therefore cannot flush, so the writer flushes for it, by the
// proxy's own rules (httputil.ReverseProxy.flushInterval, maxLatencyWriter):
// an event stream or a body of unknown length after the head and every
// write, and any other body within proxyFlushInterval of its head and of
// each write. It must be the last middleware: a compiled one after it would
// see it without Flush too.
type originServerWriter struct {
	http.ResponseWriter
	rc *http.ResponseController
	// mu serializes the proxy's writes with the delayed flush's.
	mu     sync.Mutex
	moved  bool
	stream bool
	// server is the app's Server header taken off an informational answer.
	server []string
	// ray is the request's CF-Ray, which the entrypoint's sanitizer set.
	ray string
	// delayed is the pending flush of a body of known length; done stops
	// it once the handler returns or the connection is hijacked.
	delayed *time.Timer
	done    bool
}

// moveServer runs once, under mu, as the head is about to be written with
// code. The proxy copies the app's headers in canonical form, so plain map
// access finds them, and costs the interpreter no reflective method call.
func (w *originServerWriter) moveServer(code int) {
	w.moved = true
	h := w.ResponseWriter.Header()
	server := h["Server"]
	if len(server) == 0 {
		server = w.server
	}
	delete(h, "Server")
	if len(server) == 0 || server[0] == "" {
		server = []string{"Render"}
	}
	h[originServerHeader] = server
	if w.ray != "" {
		h["Cf-Ray"] = []string{w.ray}
	}
	// Set by the service's edge cache when caching is on (lego/operator's
	// edgecache); Render's edge reports every other answer as DYNAMIC.
	if len(h["Cf-Cache-Status"]) == 0 {
		h["Cf-Cache-Status"] = []string{"DYNAMIC"}
	}
	// No body follows a 204 or 304, which the proxy does not flush either.
	if code == http.StatusNoContent || code == http.StatusNotModified {
		w.done = true
		return
	}
	w.stream = len(h["Content-Length"]) == 0 || isEventStream(h["Content-Type"])
}

func isEventStream(contentType []string) bool {
	if len(contentType) == 0 {
		return false
	}
	mediaType, _, _ := mime.ParseMediaType(contentType[0])
	return mediaType == "text/event-stream"
}

func (w *originServerWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// An informational answer (100, 103) precedes the real one. It goes out
	// with the header map as it stands, so the app's Server moves now too.
	if code >= 100 && code <= 199 && code != http.StatusSwitchingProtocols {
		h := w.ResponseWriter.Header()
		if server := h["Server"]; len(server) > 0 {
			w.server = server
			delete(h, "Server")
			h[originServerHeader] = server
		}
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if !w.moved {
		w.moveServer(code)
	}
	w.ResponseWriter.WriteHeader(code)
	w.flushed()
}

func (w *originServerWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.moved {
		w.moveServer(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.flushed()
	return n, err
}

// flushed flushes a stream at once and schedules the delayed flush of any
// other body, under mu.
func (w *originServerWriter) flushed() {
	switch {
	case w.done:
	case w.stream:
		_ = w.rc.Flush()
	case w.delayed == nil:
		w.delayed = time.AfterFunc(proxyFlushInterval, w.delayedFlush)
	}
}

func (w *originServerWriter) delayedFlush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.delayed = nil
	if !w.done {
		_ = w.rc.Flush()
	}
}

// stop ends the delayed flush, under mu: nothing may write to the response
// once the handler has returned.
func (w *originServerWriter) stop() {
	w.done = true
	if w.delayed != nil {
		w.delayed.Stop()
		w.delayed = nil
	}
}

func (w *originServerWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *originServerWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.moved {
		w.moveServer(http.StatusOK)
	}
	_ = w.rc.Flush()
}

func (w *originServerWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying response writer does not support hijacking")
	}
	w.stop()
	return hijacker.Hijack()
}
