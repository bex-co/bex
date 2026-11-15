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

package objectgw

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

const (
	// StreamDeadline is the longest one response may take to write, and
	// WriteStall how long a reader may go without taking a byte. The server's own
	// 120 s write timeout would cut a large object going to a slow reader, so each
	// streamed GET extends it as bytes are taken (a reader that stops is cut after
	// WriteStall, one that keeps up runs until StreamDeadline, the bound bex-api's
	// streams use too, ADR112 D3).
	StreamDeadline = 30 * time.Minute
	WriteStall     = time.Minute
)

// entityHeaders are the headers of Wasabi's answer that reach the visitor: what
// the object was stored with (ADR112 D4 serves the stored Content-Type and
// Cache-Control) and what describes the answer's bytes. Nothing else of Wasabi's
// is relayed: no x-amz-* header and no request id.
var entityHeaders = []string{
	"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified",
	"Cache-Control", "Content-Disposition", "Content-Encoding", "Content-Language", "Expires",
}

// copyBufs holds the buffers relayed bodies are copied through, so a stream does
// not allocate one of its own.
var copyBufs = sync.Pool{New: func() any { b := make([]byte, 32<<10); return &b }}

// Handler is the gateway: GET and HEAD of a public bucket's objects, and CORS
// preflight. Everything else is 404 or 405.
type Handler struct {
	Resolver Resolver
	Origin   Origin
	Limits   *Limits
	Meter    *Meter
	Metrics  *Metrics

	cache     *objectCache
	buffering chan struct{}
}

// NewHandler builds a handler over its parts, with a cache of the production size.
func NewHandler(resolver Resolver, origin Origin, limits *Limits, meter *Meter, metrics *Metrics) *Handler {
	return &Handler{
		Resolver: resolver, Origin: origin, Limits: limits, Meter: meter, Metrics: metrics,
		cache: newObjectCache(), buffering: make(chan struct{}, MaxBufferedFetches),
	}
}

// reqState is what one request learns on its way, for the telemetry written
// after it ends.
type reqState struct {
	workspaceID string
	origin      responseOrigin
	result      requestResult
}

// countingWriter counts the body bytes written, which is what the meters and the
// byte counter report: exactly what went to the visitor. A streamed response
// also keeps its write deadline ahead of the reader (see WriteStall).
type countingWriter struct {
	http.ResponseWriter
	n      int64
	status int

	rc       *http.ResponseController // set by stream
	hardStop time.Time
	deadline time.Time
}

func (c *countingWriter) WriteHeader(code int) {
	if c.status == 0 {
		c.status = code
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	if c.rc != nil {
		c.extendDeadline()
	}
	n, err := c.ResponseWriter.Write(p)
	c.n += int64(n)
	return n, err
}

func (c *countingWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// stream starts the rolling write deadline of a response that may be large.
func (c *countingWriter) stream() {
	c.rc = http.NewResponseController(c.ResponseWriter)
	c.hardStop = time.Now().Add(StreamDeadline)
	c.extendDeadline()
}

// extendDeadline pushes the write deadline WriteStall ahead, never past the hard
// stop, and only when half of it has gone, so a fast stream does not set a timer
// per write. A writer that cannot take a deadline (a test's) keeps its own.
func (c *countingWriter) extendDeadline() {
	now := time.Now()
	if c.deadline.Sub(now) > WriteStall/2 {
		return
	}
	c.deadline = now.Add(WriteStall)
	if c.deadline.After(c.hardStop) {
		c.deadline = c.hardStop
	}
	_ = c.rc.SetWriteDeadline(c.deadline)
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w := &countingWriter{ResponseWriter: rw}
	// Every answer, errors included, forbids sniffing: content is the tenant's.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	st := &reqState{result: resultOK, origin: originWasabi}
	h.serve(w, r, st)
	// What an object's bytes cost is what the visitor received of the object:
	// the short text of an error is not an object's, whoever wrote it.
	if w.status >= http.StatusBadRequest {
		w.n = 0
	}
	if st.result == resultOK {
		switch w.status {
		case http.StatusNotModified:
			st.result = resultNotModified
		case http.StatusRequestedRangeNotSatisfiable:
			st.result = resultRangeNotSatisfiable
		}
	}
	if h.Metrics != nil {
		h.Metrics.requests.WithLabelValues(string(st.result)).Inc()
		if w.n > 0 {
			h.Metrics.responseBytes.WithLabelValues(string(st.origin)).Add(float64(w.n))
		}
	}
	h.Meter.Add(st.workspaceID, w.n)
}

func (h *Handler) serve(w *countingWriter, r *http.Request, st *reqState) {
	// A host that is not shaped like a bucket's costs no lookup.
	if !isBucketHost(r.Host) {
		plain(w, st, http.StatusNotFound, resultNotFound, "not found")
		return
	}
	bucket, err := h.Resolver.Lookup(r.Context(), r.Host)
	switch {
	case errors.Is(err, ErrNotFound):
		plain(w, st, http.StatusNotFound, resultNotFound, "not found")
		return
	case errors.Is(err, ErrBusy):
		unavailable(w, st, 1, "try again shortly")
		return
	case err != nil:
		log.Printf("objectgw: lookup: %v", err)
		unavailable(w, st, 5, "temporarily unavailable")
		return
	}
	st.workspaceID = bucket.WorkspaceID

	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
	default:
		methodNotAllowed(w, st)
		return
	}

	release, refusal := h.Limits.Admit(bucket.ID, bucket.WorkspaceID)
	if refusal != nil {
		w.Header().Set("Retry-After", strconv.Itoa(refusal.RetryAfterSeconds))
		plain(w, st, refusal.Status, refusal.Result, http.StatusText(refusal.Status))
		return
	}
	defer release()

	if r.Method == http.MethodOptions {
		preflight(w, r, bucket, st)
		return
	}

	// The object is whatever follows the slash; the query is ignored, so no
	// parameter of S3's (a listing, a version, a response override) is ever
	// passed on. The root, and any path that is not a valid key, is 404.
	key := strings.TrimPrefix(r.URL.Path, "/")
	wasabiKey, err := objectstorage.ObjectKey(bucket.WorkspaceID, bucket.ID, key)
	if err != nil {
		plain(w, st, http.StatusNotFound, resultNotFound, "not found")
		return
	}
	addCORS(w, r, bucket)
	h.serveObject(w, r, bucket, key, wasabiKey, st)
}

// writePlain answers with a short plain-text body, which is never cached: a
// missing object may be uploaded a moment later.
func writePlain(w http.ResponseWriter, status int, message string) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, message+"\n")
}

func plain(w *countingWriter, st *reqState, status int, result requestResult, message string) {
	st.result = result
	writePlain(w, status, message)
}

// unavailable answers 503 and tells the visitor when to come back.
func unavailable(w *countingWriter, st *reqState, retryAfterSeconds int, message string) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	plain(w, st, http.StatusServiceUnavailable, resultUnavailable, message)
}

func methodNotAllowed(w *countingWriter, st *reqState) {
	w.Header().Set("Allow", "GET, HEAD, OPTIONS")
	plain(w, st, http.StatusMethodNotAllowed, resultMethodNotAllowed, "method not allowed")
}

// addCORS adds the headers of the first CORS rule that admits the request's
// origin, and Vary: Origin whenever the bucket has rules, so a shared cache does
// not hand one origin's answer to another.
func addCORS(w http.ResponseWriter, r *http.Request, b Bucket) {
	if len(b.CORS) == 0 {
		return
	}
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if m, ok := matchCORS(b.CORS, origin, r.Method, nil); ok {
		m.setActual(w.Header(), origin)
	}
}

// preflight answers a CORS preflight from the bucket's rules. An OPTIONS that is
// not a preflight, and one no rule admits, is refused with no CORS header.
func preflight(w *countingWriter, r *http.Request, b Bucket, st *reqState) {
	origin := r.Header.Get("Origin")
	method := r.Header.Get("Access-Control-Request-Method")
	if origin == "" || method == "" {
		methodNotAllowed(w, st)
		return
	}
	requested := splitHeaderList(r.Header.Get("Access-Control-Request-Headers"))
	hdr := w.Header()
	hdr.Add("Vary", "Origin")
	hdr.Add("Vary", "Access-Control-Request-Method")
	hdr.Add("Vary", "Access-Control-Request-Headers")
	m, ok := matchCORS(b.CORS, origin, method, requested)
	if !ok {
		plain(w, st, http.StatusForbidden, resultPreflight, "this origin, method or header is not allowed by the bucket's CORS rules")
		return
	}
	m.setPreflight(hdr, origin, requested)
	st.result = resultPreflight
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) serveObject(w *countingWriter, r *http.Request, b Bucket, key, wasabiKey string, st *reqState) {
	cacheKey := b.ID + "/" + key
	if obj, ok := h.cache.get(cacheKey); ok {
		st.origin = originCache
		h.serveCached(w, r, b, obj)
		return
	}

	w.stream()
	forwarded := forwardHeaders(r.Header)
	resp, err := h.Origin.Fetch(r.Context(), b.Region, wasabiKey, r.Method, forwarded)
	if err != nil {
		if r.Context().Err() == nil {
			log.Printf("objectgw: origin: %v", err)
		}
		unavailable(w, st, 5, "temporarily unavailable")
		return
	}
	// Read to its end before it is closed, so the connection goes back to the pool.
	defer drainClose(resp)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent, http.StatusNotModified:
	case http.StatusPreconditionFailed:
		// The store's own body is an XML error with its request id; the visitor
		// gets the status and a plain line instead.
		plain(w, st, http.StatusPreconditionFailed, resultPreconditionFailed, "precondition failed")
		return
	case http.StatusRequestedRangeNotSatisfiable:
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			w.Header().Set("Content-Range", cr)
		}
		plain(w, st, http.StatusRequestedRangeNotSatisfiable, resultRangeNotSatisfiable, "range not satisfiable")
		return
	case http.StatusNotFound:
		plain(w, st, http.StatusNotFound, resultNotFound, "not found")
		return
	default:
		// Anything else (the store's 403 for our own credential, a 5xx) is the
		// gateway's problem, not the visitor's, and its detail stays in the log.
		log.Printf("objectgw: origin answered %d for %s", resp.StatusCode, b.ID)
		plain(w, st, http.StatusBadGateway, resultError, "bad gateway")
		return
	}

	if cacheable(r, forwarded, resp) {
		if obj, filled, ok := h.tryFill(cacheKey, resp); filled {
			if !ok {
				// A body that did not read whole is not served half: the headers
				// are still unsent, so the visitor gets an error, not a truncated object.
				plain(w, st, http.StatusBadGateway, resultError, "bad gateway")
				return
			}
			h.serveCached(w, r, b, obj)
			return
		}
	}
	h.relay(w, r, b, resp)
}

// cacheable reports whether the answer is a whole small object the cache may
// hold: a plain GET (no Range, no condition) answered 200 and not over the
// per-object bound.
func cacheable(r *http.Request, forwarded http.Header, resp *http.Response) bool {
	return r.Method == http.MethodGet && len(forwarded) == 0 && resp.StatusCode == http.StatusOK &&
		resp.ContentLength >= 0 && resp.ContentLength <= CacheMaxObjectBytes
}

// tryFill reads a cacheable object whole into the cache, if one of the
// MaxBufferedFetches fill slots is free. filled is false when none was, and the
// caller streams the object instead; ok is false when the body did not read whole.
func (h *Handler) tryFill(cacheKey string, resp *http.Response) (obj *cachedObject, filled, ok bool) {
	select {
	case h.buffering <- struct{}{}:
		defer func() { <-h.buffering }()
	default:
		return nil, false, false
	}
	body := make([]byte, resp.ContentLength)
	if _, err := io.ReadFull(resp.Body, body); err != nil {
		log.Printf("objectgw: reading %s to cache it: %v", cacheKey, err)
		return nil, true, false
	}
	obj = &cachedObject{key: cacheKey, body: body, header: entityHeadersOf(resp.Header)}
	obj.modTime, _ = http.ParseTime(resp.Header.Get("Last-Modified"))
	h.cache.put(obj)
	return obj, true, true
}

func entityHeadersOf(in http.Header) http.Header {
	out := http.Header{}
	for _, name := range entityHeaders {
		if v := in.Get(name); v != "" {
			out.Set(name, v)
		}
	}
	return out
}

// serveCached answers from a whole object in memory. http.ServeContent
// evaluates Range, If-Range and the four conditional headers and answers HEAD,
// so a hit and a miss differ only in where the bytes came from.
func (h *Handler) serveCached(w *countingWriter, r *http.Request, b Bucket, obj *cachedObject) {
	hdr := w.Header()
	for name, values := range obj.header {
		hdr[name] = values // ServeContent sets Content-Length, Content-Range and Accept-Ranges itself
	}
	if hdr.Get("Content-Type") == "" {
		// Without one, ServeContent would sniff the body.
		hdr.Set("Content-Type", "application/octet-stream")
	}
	http.ServeContent(w, r, "", obj.modTime, bytes.NewReader(obj.body))
	h.Limits.Charge(b.ID, b.WorkspaceID, w.n)
}

// writerOnly and readerOnly hide the ReaderFrom and WriterTo shortcuts of what
// they wrap, so io.CopyBuffer uses the pooled buffer.
type (
	writerOnly struct{ io.Writer }
	readerOnly struct{ io.Reader }
)

// relay streams Wasabi's answer to the visitor unchanged: its status, the entity
// headers and the body.
func (h *Handler) relay(w *countingWriter, r *http.Request, b Bucket, resp *http.Response) {
	hdr := w.Header()
	for name, values := range entityHeadersOf(resp.Header) {
		hdr[name] = values
	}
	sendsBody := r.Method == http.MethodGet && resp.StatusCode != http.StatusNotModified
	if sendsBody && resp.ContentLength > 0 {
		// Charged as it starts: the bucket owes these bytes whether or not the
		// reader stays to the end.
		h.Limits.Charge(b.ID, b.WorkspaceID, resp.ContentLength)
	}
	w.WriteHeader(resp.StatusCode)
	if !sendsBody {
		return
	}
	buf := copyBufs.Get().(*[]byte)
	defer copyBufs.Put(buf)
	if _, err := io.CopyBuffer(writerOnly{w}, readerOnly{resp.Body}, *buf); err != nil && r.Context().Err() == nil {
		log.Printf("objectgw: relaying %s: %v", b.ID, err)
	}
}
