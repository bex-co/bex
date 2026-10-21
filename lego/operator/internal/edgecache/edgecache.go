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

// Package edgecache is Render's web-service edge caching
// (render.com/docs/web-service-caching) as a per-pod caching reverse proxy in
// front of the app container (w5/m151). It caches what Render caches and
// reports it the way Render does:
//
//   - Profiles: ProfileStatic caches Render's "common static files" list,
//     ProfileAll caches every eligible response.
//   - Eligibility: a GET (a HEAD is answered from a stored GET) with no
//     Authorization, Range or Upgrade; a response without Set-Cookie, with a
//     Vary of at most Accept-Encoding, not an event stream, and a status
//     Render caches (200, 206, 301, 302, 303, 404, 410).
//   - TTL: CDN-Cache-Control beats Cache-Control, s-maxage beats max-age;
//     no-store, private and no-cache disable caching, and without a directive
//     the status's default applies (200/206/301: 120 minutes, 302/303: 20
//     minutes, 404/410: 3 minutes).
//   - CF-Cache-Status: HIT, MISS, EXPIRED, BYPASS (the response opted out)
//     or DYNAMIC (not eligible).
//   - Concurrent misses for one key collapse into a single upstream request.
//
// Every request, cached or not, goes upstream through one
// httputil.ReverseProxy, so the app sees the same headers either way. The
// cache lives in the pod's memory, so every deploy starts empty: Render purges
// a service's cache on each deploy, and so does this.
package edgecache

import (
	"bytes"
	"container/list"
	"context"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Profiles, named as Render's API names them (serviceDetails.cache.profile).
const (
	ProfileNone   = "no-cache"
	ProfileStatic = "origin-controlled"
	ProfileAll    = "origin-controlled-all"
)

// StatusHeader is the response header Render reports cache status in, and
// these its values.
const (
	StatusHeader  = "CF-Cache-Status"
	statusHit     = "HIT"
	statusMiss    = "MISS"
	statusExpired = "EXPIRED"
	statusBypass  = "BYPASS"
	statusDynamic = "DYNAMIC"
)

// staticExtensions is Render's "common static files" list.
var staticExtensions = map[string]bool{}

func init() {
	for ext := range strings.FieldsSeq("7z avi avif bin bmp bz2 class css csv dmg doc docx eot ejs eps exe flac gif gz ico iso jar jpeg jpg js mid midi mkv mp3 mp4 ogg otf pdf pict pls png ppt pptx ps rar svg svgz swf tar tif tiff ttf webm webp woff woff2 xls xlsx zip zst") {
		staticExtensions[ext] = true
	}
}

// defaultTTL is Render's TTL for a cache-eligible status without a directive;
// zero means the status is never cached, whatever its directives say.
func defaultTTL(status int) time.Duration {
	switch status {
	case http.StatusOK, http.StatusPartialContent, http.StatusMovedPermanently:
		return 120 * time.Minute
	case http.StatusFound, http.StatusSeeOther:
		return 20 * time.Minute
	case http.StatusNotFound, http.StatusGone:
		return 3 * time.Minute
	}
	return 0
}

// Config bounds one cache.
type Config struct {
	Profile string
	// MaxBytes bounds the whole cache; MaxObjectBytes one response body;
	// MaxFills how many responses may be buffered for storing at once.
	MaxBytes, MaxObjectBytes int64
	MaxFills                 int
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Handler is the caching proxy in front of upstream.
type Handler struct {
	cfg   Config
	proxy *httputil.ReverseProxy
	fills chan struct{}

	mu      sync.Mutex
	entries map[string]*list.Element
	lru     *list.List
	size    int64
	flights map[string]*flight
}

type entry struct {
	key     string
	status  int
	header  http.Header
	body    []byte
	expires time.Time
}

// flight is one upstream fetch other requests for the key wait on. done
// closes as soon as the leader knows whether it will store the response;
// stored then holds the entry, or nil when the response is not kept.
type flight struct {
	done   chan struct{}
	once   sync.Once
	stored *entry
}

func (f *flight) settle(e *entry) {
	f.once.Do(func() {
		f.stored = e
		close(f.done)
	})
}

type fillKey struct{}

// fill is one leader's fetch in progress, carried on the request context.
type fill struct {
	key, fetched string
	flight       *flight
	keep         bool
	ttl          time.Duration
	status       int
	header       http.Header
}

// New returns a Handler proxying to upstream (e.g. http://127.0.0.1:10000).
func New(upstream *url.URL, cfg Config) *Handler {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.MaxFills <= 0 {
		cfg.MaxFills = 4
	}
	h := &Handler{
		cfg: cfg, fills: make(chan struct{}, cfg.MaxFills),
		entries: map[string]*list.Element{}, lru: list.New(), flights: map[string]*flight{},
	}
	h.proxy = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.Out.Host = pr.In.Host
			pr.SetXForwarded()
		},
		// Flush as bytes arrive, so streamed and long-poll responses flow.
		FlushInterval:  -1,
		ModifyResponse: h.decide,
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.requestEligible(r) {
		w.Header().Set(StatusHeader, statusDynamic)
		h.proxy.ServeHTTP(w, r)
		return
	}
	key := cacheKey(r)
	e, expired := h.lookup(key)
	if e != nil {
		writeEntry(w, r, e, statusHit)
		return
	}
	if r.Method != http.MethodGet {
		// Only a GET fills: a HEAD response has no body to store.
		w.Header().Set(StatusHeader, statusMiss)
		h.proxy.ServeHTTP(w, r)
		return
	}
	fetched := statusMiss
	if expired {
		fetched = statusExpired
	}
	h.fetch(w, r, key, fetched)
}

// requestEligible is the request half of Render's rules.
func (h *Handler) requestEligible(r *http.Request) bool {
	if h.cfg.Profile != ProfileStatic && h.cfg.Profile != ProfileAll {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.Header.Get("Authorization") != "" || r.Header.Get("Upgrade") != "" || r.Header.Get("Range") != "" {
		return false
	}
	if h.cfg.Profile == ProfileStatic {
		ext := strings.TrimPrefix(strings.ToLower(path.Ext(r.URL.Path)), ".")
		return staticExtensions[ext]
	}
	return true
}

// acceptsGzip reports a client that takes gzip: a gzip (or *) coding with a
// nonzero quality.
func acceptsGzip(header string) bool {
	for part := range strings.SplitSeq(header, ",") {
		coding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if c := strings.ToLower(strings.TrimSpace(coding)); c != "gzip" && c != "*" {
			continue
		}
		if q, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if v, err := strconv.ParseFloat(q, 64); err == nil && v == 0 {
				continue
			}
		}
		return true
	}
	return false
}

func cacheKey(r *http.Request) string {
	return r.Host + "\x00" + r.URL.RequestURI() + "\x00" + strconv.FormatBool(acceptsGzip(r.Header.Get("Accept-Encoding")))
}

func (h *Handler) lookup(key string) (*entry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	el, ok := h.entries[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*entry)
	if !h.cfg.Now().Before(e.expires) {
		h.removeLocked(el)
		return nil, true
	}
	h.lru.MoveToFront(el)
	return e, false
}

// fetch fetches key once, however many requests miss it at the same time.
func (h *Handler) fetch(w http.ResponseWriter, r *http.Request, key, fetched string) {
	h.mu.Lock()
	if f, ok := h.flights[key]; ok {
		h.mu.Unlock()
		select {
		case <-f.done:
		case <-r.Context().Done():
			return
		}
		if f.stored != nil {
			writeEntry(w, r, f.stored, statusHit)
			return
		}
		w.Header().Set(StatusHeader, statusMiss)
		h.proxy.ServeHTTP(w, r)
		return
	}
	select {
	case h.fills <- struct{}{}:
	default:
		// Too many responses buffering already: serve this one uncached.
		h.mu.Unlock()
		w.Header().Set(StatusHeader, statusMiss)
		h.proxy.ServeHTTP(w, r)
		return
	}
	f := &flight{done: make(chan struct{})}
	h.flights[key] = f
	h.mu.Unlock()
	defer func() {
		<-h.fills
		h.mu.Lock()
		delete(h.flights, key)
		h.mu.Unlock()
		f.settle(nil)
	}()

	// The stored body must suit every client the key covers, so the upstream
	// sees one canonical encoding preference and no validators (a 304 is not
	// a page).
	out := r.Clone(context.WithValue(r.Context(), fillKey{}, nil))
	st := &fill{key: key, fetched: fetched, flight: f}
	out = out.WithContext(context.WithValue(out.Context(), fillKey{}, st))
	out.Header.Del("If-None-Match")
	out.Header.Del("If-Modified-Since")
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		out.Header.Set("Accept-Encoding", "gzip")
	} else {
		out.Header.Set("Accept-Encoding", "identity")
	}
	tee := &teeWriter{ResponseWriter: w, st: st, max: h.cfg.MaxObjectBytes}
	h.proxy.ServeHTTP(tee, out)
	if st.keep && !tee.overflow {
		e := &entry{key: key, status: st.status, header: st.header, body: tee.buf.Bytes(), expires: h.cfg.Now().Add(st.ttl)}
		h.store(e)
		f.settle(e)
	}
}

// decide is the ReverseProxy's ModifyResponse: for a leader's fetch it
// decides whether to keep the response, and releases waiters at once when
// not, so a stream or an uncacheable page never holds them up.
func (h *Handler) decide(resp *http.Response) error {
	st, _ := resp.Request.Context().Value(fillKey{}).(*fill)
	if st == nil {
		return nil
	}
	ttl, status := responseTTL(resp)
	if ttl > 0 && resp.ContentLength <= h.cfg.MaxObjectBytes {
		st.keep, st.ttl, st.status, st.header = true, ttl, resp.StatusCode, resp.Header.Clone()
		resp.Header.Set(StatusHeader, st.fetched)
		return nil
	}
	if status == "" {
		status = statusDynamic
	}
	resp.Header.Set(StatusHeader, status)
	st.flight.settle(nil)
	return nil
}

// responseTTL is the response half of Render's rules: how long to keep it,
// and the status to report when it is not kept.
func responseTTL(resp *http.Response) (time.Duration, string) {
	if defaultTTL(resp.StatusCode) == 0 {
		return 0, statusDynamic
	}
	if len(resp.Header.Values("Set-Cookie")) > 0 {
		return 0, statusBypass
	}
	for _, v := range resp.Header.Values("Vary") {
		for field := range strings.SplitSeq(v, ",") {
			if f := strings.TrimSpace(field); f != "" && !strings.EqualFold(f, "Accept-Encoding") {
				return 0, statusBypass
			}
		}
	}
	if ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); ct == "text/event-stream" {
		return 0, statusDynamic
	}
	directives := resp.Header.Values("CDN-Cache-Control")
	if len(directives) == 0 {
		directives = resp.Header.Values("Cache-Control")
	}
	maxAge, sMaxAge, haveMax, haveS := 0, 0, false, false
	for d := range strings.SplitSeq(strings.Join(directives, ","), ",") {
		name, value, _ := strings.Cut(strings.ToLower(strings.TrimSpace(d)), "=")
		switch name {
		case "no-store", "private", "no-cache":
			return 0, statusBypass
		case "max-age":
			maxAge, _ = strconv.Atoi(strings.Trim(value, `"`))
			haveMax = true
		case "s-maxage":
			sMaxAge, _ = strconv.Atoi(strings.Trim(value, `"`))
			haveS = true
		}
	}
	switch {
	case haveS:
		return time.Duration(sMaxAge) * time.Second, statusBypass
	case haveMax:
		return time.Duration(maxAge) * time.Second, statusBypass
	}
	return defaultTTL(resp.StatusCode), ""
}

// teeWriter passes a leader's response through and keeps a copy of a body
// the leader decided to store, up to max bytes.
type teeWriter struct {
	http.ResponseWriter
	st       *fill
	max      int64
	buf      bytes.Buffer
	overflow bool
}

func (t *teeWriter) Write(p []byte) (int, error) {
	if t.st.keep && !t.overflow {
		if int64(t.buf.Len()+len(p)) > t.max {
			t.overflow = true
			t.buf = bytes.Buffer{}
		} else {
			t.buf.Write(p)
		}
	}
	return t.ResponseWriter.Write(p)
}

func (t *teeWriter) Flush() {
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (t *teeWriter) Unwrap() http.ResponseWriter { return t.ResponseWriter }

func (h *Handler) store(e *entry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, ok := h.entries[e.key]; ok {
		h.removeLocked(old)
	}
	cost := int64(len(e.body))
	if cost > h.cfg.MaxBytes {
		return
	}
	for h.size+cost > h.cfg.MaxBytes && h.lru.Len() > 0 {
		h.removeLocked(h.lru.Back())
	}
	h.entries[e.key] = h.lru.PushFront(e)
	h.size += cost
}

func (h *Handler) removeLocked(el *list.Element) {
	e := el.Value.(*entry)
	h.lru.Remove(el)
	delete(h.entries, e.key)
	h.size -= int64(len(e.body))
}

func writeEntry(w http.ResponseWriter, r *http.Request, e *entry, status string) {
	for k, vs := range e.header {
		w.Header()[k] = append([]string(nil), vs...)
	}
	w.Header().Set(StatusHeader, status)
	w.Header().Set("Content-Length", strconv.Itoa(len(e.body)))
	w.WriteHeader(e.status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(e.body)
	}
}
