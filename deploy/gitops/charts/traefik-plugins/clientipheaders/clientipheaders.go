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
// Cloudflare proxy. CF-* headers survive only from a Cloudflare peer.
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
// A second instance, configured with originServer, sits in each web service's
// own route (w8/321): Render's edge never hands a web service's Server header
// to clients. It moves the value to X-Render-Origin-Server, and names "Render"
// there when the app sent none. Static sites and platform hosts carry no
// such instance, as Render's static sites carry no such header.
package bex_client_ip_headers

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"mime"
	"net"
	"net/http"
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
// Cloudflare peer may pass them on. Keys are lowercase with "-" for "_":
// CGI-style frameworks (Rack, PHP, WSGI) read True_Client_IP and
// True-Client-IP as the same HTTP_TRUE_CLIENT_IP, so both spellings count.
var cloudflareHeaders = map[string]bool{
	"cf-connecting-ip": true, "cf-connecting-ipv6": true, "cf-ipcountry": true,
	"cf-ray": true, "cf-visitor": true, "cf-worker": true,
}

// alwaysStripped never comes from a party bex trusts; True-Client-IP, Rndr-Id
// and X-Request-Start are rewritten below.
var alwaysStripped = map[string]bool{"x-client-ip": true, "true-client-ip": true, "rndr-id": true, "x-request-start": true}

type Config struct {
	// OriginServer makes this instance the per-web-service Server rename
	// (w8/321) instead of the entrypoint's request sanitizer.
	OriginServer bool `json:"originServer,omitempty"`
}

func CreateConfig() *Config { return &Config{} }

type sanitizer struct {
	next       http.Handler
	cloudflare []*net.IPNet
}

func New(_ context.Context, next http.Handler, config *Config, _ string) (http.Handler, error) {
	if config != nil && config.OriginServer {
		return &originServer{next: next}, nil
	}
	s := &sanitizer{next: next}
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
	if v := h.Get("CF-Connecting-IP"); v != "" && peer != nil && s.fromCloudflare(peer) {
		client = net.ParseIP(v)
	}
	for name := range h {
		key := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		// A trusted edge's CF-* stay only in Cloudflare's own hyphenated form.
		if alwaysStripped[key] || (cloudflareHeaders[key] && (client == nil || strings.Contains(name, "_"))) {
			delete(h, name)
		}
	}
	switch {
	case client != nil:
		h.Set("CF-Connecting-IP", client.String())
		h.Set("True-Client-IP", client.String())
	case peer != nil:
		h.Set("True-Client-IP", peer.String())
	}
	// Set before the app answers: an app that sends its own Rndr-Id adds a
	// second value after ours, and Header.Get still reads ours.
	id := requestID()
	h.Set("Rndr-Id", id)
	w.Header().Set("Rndr-Id", id)
	// Render's format: bare microseconds since the epoch, no "t=" prefix. Taken
	// at the edge, before any activator wake, so queue time includes the
	// spin-up, as on Render.
	h.Set("X-Request-Start", strconv.FormatInt(time.Now().UnixMicro(), 10))
	s.next.ServeHTTP(w, r)
}

// requestID is a random id in Render's Rndr-Id shape: 8-4-4 lowercase hex,
// 18 characters, as in its "303516bf-4995-4cb9" (w4/583).
func requestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // never fails on supported platforms (Go 1.24+)
	x := hex.EncodeToString(b[:])
	return x[0:8] + "-" + x[8:12] + "-" + x[12:16]
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

// originServerHeader is where Render's edge puts a web service's own Server
// header, and "Render" when the app sent none (captured 2026-11-14: a uvicorn
// app answers "x-render-origin-server: uvicorn", an Express app "Render").
const originServerHeader = "X-Render-Origin-Server"

// proxyFlushInterval is Traefik's default responseForwarding.flushInterval:
// how often its proxy flushes a body of known length.
const proxyFlushInterval = 100 * time.Millisecond

type originServer struct{ next http.Handler }

func (o *originServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ow := &originServerWriter{ResponseWriter: w, rc: http.NewResponseController(w)}
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
