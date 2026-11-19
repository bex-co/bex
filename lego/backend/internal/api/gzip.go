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

package api

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// gzipWriterPool reuses gzip.Writers across requests — each compressed response
// borrows one and returns it on close, so a high request rate doesn't churn the
// allocator (w9/m61).
var gzipWriterPool = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

// withGzip compresses read-path responses (GraphQL/REST JSON compresses 70–85%)
// when the client offers `Accept-Encoding: gzip`, while leaving streaming
// responses untouched. The compress/skip decision is made from the response the
// handler actually produced (its `Content-Type`), not a route allowlist, so a
// future SSE route can never silently regress into a buffered, latency-added
// stream — any `text/event-stream` (or already-encoded) response passes through
// byte-for-byte with per-event flushing intact.
func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !clientAcceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		// Advertise that the representation varies by Accept-Encoding so shared
		// caches don't hand a gzip body to an identity-only client.
		w.Header().Add("Vary", "Accept-Encoding")
		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

// clientAcceptsGzip reports whether the request's Accept-Encoding lists gzip
// (ignoring any q-value; a q=0 refusal is rare and the cost of honoring the
// coarse check is nil).
func clientAcceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		token := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if strings.EqualFold(token, "gzip") {
			return true
		}
	}
	return false
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	decided     bool
	compress    bool
	wroteHeader bool
}

// decide runs exactly once, at the first WriteHeader/Write, and chooses whether
// to compress from the headers the handler set.
func (g *gzipResponseWriter) decide() {
	if g.decided {
		return
	}
	g.decided = true
	h := g.Header()
	ct := h.Get("Content-Type")
	// Never compress an event stream (per-event flush latency) or a body the
	// handler already encoded itself.
	if strings.HasPrefix(ct, "text/event-stream") || h.Get("Content-Encoding") != "" {
		return
	}
	g.compress = true
	// Length is unknown after compression, and a stale identity length would be
	// wrong on the wire.
	h.Del("Content-Length")
	h.Set("Content-Encoding", "gzip")
	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(g.ResponseWriter)
	g.gz = gz
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	g.decide()
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.compress {
		return g.gz.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

// Flush keeps streaming responses live: for a compressed body it flushes the
// gzip framing first; either way it forwards to the underlying flusher so SSE
// events reach the client per message.
func (g *gzipResponseWriter) Flush() {
	if g.compress && g.gz != nil {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack passes through so any hijacking handler (never compressed) still works.
func (g *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := g.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Unwrap lets http.ResponseController reach the underlying writer's
// Flusher/Hijacker/etc. when a handler uses the controller API.
func (g *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}

// close finishes the gzip stream and returns the writer to the pool. Safe when
// the response was never compressed (gz stays nil).
func (g *gzipResponseWriter) close() {
	if g.gz == nil {
		return
	}
	_ = g.gz.Close()
	gzipWriterPool.Put(g.gz)
	g.gz = nil
}
