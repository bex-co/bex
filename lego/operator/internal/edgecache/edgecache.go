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
//   - Conditional requests (w3/m488): a client whose If-None-Match or
//     If-Modified-Since matches the page gets 304 Not Modified, from a stored
//     page and from the fill alike, so caching never turns an app's 304s into
//     full 200s. Every answer from a stored page carries its Age.
//
// Every request, cached or not, goes upstream through one
// httputil.ReverseProxy, so the app sees the same headers either way. The
// cache lives in the pod's memory, so every deploy starts empty: Render purges
// a service's cache on each deploy, and so does this. A manual purge and a
// profile switch reach a running proxy through its control file (control.go,
// w3/m486), so neither restarts the instance, as on Render's CDN.
package edgecache

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	// Profile is the profile the proxy starts with (SetProfile replaces it).
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
	cfg     Config
	proxy   *httputil.ReverseProxy
	fills   chan struct{}
	profile atomic.Pointer[string]

	mu      sync.Mutex
	entries map[string]*list.Element
	lru     *list.List
	size    int64
	flights map[string]*flight
	// generation counts purges: a fill started under an older one is not
	// stored, so a purge empties the cache of responses fetched before it.
	generation uint64
}

type entry struct {
	key    string
	status int
	header http.Header
	body   []byte
	// born is when the upstream generated the response: the time it was
	// stored less the Age the upstream gave it (RFC 9111 §4.2.3).
	born, expires time.Time
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
	// client is the leader's own request header, validators and all; the
	// upstream sees none, so the body it sends suits every client.
	client http.Header
	keep   bool
	ttl    time.Duration
	status int
	header http.Header
	// body is the copy kept for storing, at most maxBody bytes; a longer
	// body overflows and nothing is stored.
	body     bytes.Buffer
	maxBody  int64
	overflow bool
}

// Write keeps p for storing while the response is kept and still fits.
func (st *fill) Write(p []byte) (int, error) {
	if st.keep && !st.overflow {
		if int64(st.body.Len()+len(p)) > st.maxBody {
			st.overflow = true
			st.body = bytes.Buffer{}
		} else {
			st.body.Write(p)
		}
	}
	return len(p), nil
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
	h.profile.Store(&cfg.Profile)
	h.proxy = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.Out.Host = pr.In.Host
			// ReverseProxy strips the inbound X-Forwarded-*. Traefik sets them
			// and is the only edge that reaches this sidecar, so the app sees
			// the client address and scheme the edge saw, as through the
			// activator's proxy (ADR004; w21/m394 t015, R25-07): the inbound
			// X-Forwarded-For is copied before SetXForwarded, which appends the
			// peer, and an http or https X-Forwarded-Proto replaces its "http".
			if forwardedFor := pr.In.Header.Values("X-Forwarded-For"); len(forwardedFor) > 0 {
				pr.Out.Header["X-Forwarded-For"] = slices.Clone(forwardedFor)
			}
			pr.SetXForwarded()
			if proto := strings.ToLower(pr.In.Header.Get("X-Forwarded-Proto")); proto == "https" || proto == "http" {
				pr.Out.Header.Set("X-Forwarded-Proto", proto)
			}
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
		h.writeEntry(w, r, e)
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

// Profile is the profile the proxy caches by.
func (h *Handler) Profile() string { return *h.profile.Load() }

// SetProfile replaces the profile from the next request on, and purges the
// entries stored under the old one, as a switch did when it rolled the pods.
// Any other value than ProfileStatic or ProfileAll passes every request
// through (DYNAMIC): caching turned off before the next deploy removes the
// proxy. It reports whether the profile changed.
func (h *Handler) SetProfile(profile string) bool {
	if h.Profile() == profile {
		return false
	}
	h.profile.Store(&profile)
	h.Purge()
	return true
}

// Purge empties the cache: every stored entry leaves, and a fill already in
// flight is served to its own client but not stored.
func (h *Handler) Purge() {
	h.mu.Lock()
	defer h.mu.Unlock()
	clear(h.entries)
	h.lru.Init()
	h.size = 0
	h.generation++
}

// requestEligible is the request half of Render's rules.
func (h *Handler) requestEligible(r *http.Request) bool {
	profile := h.Profile()
	if profile != ProfileStatic && profile != ProfileAll {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.Header.Get("Authorization") != "" || r.Header.Get("Upgrade") != "" || r.Header.Get("Range") != "" {
		return false
	}
	if profile == ProfileStatic {
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
			h.writeEntry(w, r, f.stored)
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
	generation := h.generation
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
	// a page); decide answers the leader's own validators.
	out := r.Clone(context.WithValue(r.Context(), fillKey{}, nil))
	st := &fill{key: key, fetched: fetched, flight: f, client: r.Header, maxBody: h.cfg.MaxObjectBytes}
	out = out.WithContext(context.WithValue(out.Context(), fillKey{}, st))
	out.Header.Del("If-None-Match")
	out.Header.Del("If-Modified-Since")
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		out.Header.Set("Accept-Encoding", "gzip")
	} else {
		out.Header.Set("Accept-Encoding", "identity")
	}
	h.proxy.ServeHTTP(&teeWriter{ResponseWriter: w, st: st}, out)
	if st.keep && !st.overflow {
		now := h.cfg.Now()
		e := &entry{
			key: key, status: st.status, header: st.header, body: st.body.Bytes(),
			born: now.Add(-upstreamAge(st.header)), expires: now.Add(st.ttl),
		}
		if h.store(e, generation) {
			f.settle(e)
		}
	}
}

// decide is the ReverseProxy's ModifyResponse: for a leader's fetch it
// decides whether to keep the response, and releases waiters at once when
// not, so a stream or an uncacheable page never holds them up. A leader whose
// own validators match the response gets a 304, as the app would have sent
// it, and a kept body is read into the store here, since the leader is sent
// none of it.
func (h *Handler) decide(resp *http.Response) error {
	st, _ := resp.Request.Context().Value(fillKey{}).(*fill)
	if st == nil {
		return nil
	}
	ttl, status := responseTTL(resp)
	if ttl > 0 && resp.ContentLength <= h.cfg.MaxObjectBytes {
		st.keep, st.ttl, st.status, st.header = true, ttl, resp.StatusCode, resp.Header.Clone()
		status = st.fetched
	} else {
		if status == "" {
			status = statusDynamic
		}
		st.flight.settle(nil)
	}
	resp.Header.Set(StatusHeader, status)
	if !notModified(st.client, resp.StatusCode, resp.Header) {
		return nil
	}
	if st.keep {
		// The limit stops reading a body that overflows: nothing is stored.
		if _, err := io.Copy(st, io.LimitReader(resp.Body, st.maxBody+1)); err != nil {
			st.keep = false
		}
	}
	_ = resp.Body.Close()
	resp.StatusCode, resp.Body, resp.ContentLength = http.StatusNotModified, http.NoBody, 0
	notModifiedHeader(resp.Header)
	return nil
}

// notModified reports whether a GET or HEAD carrying reqHeader is answered 304
// Not Modified by a response with status and header, as RFC 9110 §13.2.2
// orders the validators: If-None-Match by weak comparison, and, only without
// it, If-Modified-Since against Last-Modified. Only a 2xx is ever replaced
// (§13.2.1).
func notModified(reqHeader http.Header, status int, header http.Header) bool {
	if status < 200 || status > 299 {
		return false
	}
	if inm := strings.Join(reqHeader.Values("If-None-Match"), ","); inm != "" {
		return noneMatch(inm, header.Get("ETag"))
	}
	ims := reqHeader.Values("If-Modified-Since")
	if len(ims) != 1 {
		return false
	}
	since, err := http.ParseTime(ims[0])
	if err != nil {
		return false
	}
	modified, err := http.ParseTime(header.Get("Last-Modified"))
	return err == nil && !modified.After(since)
}

// noneMatch reports whether an If-None-Match value is "*" or names etag by weak
// comparison (RFC 9110 §8.8.3.2, §13.1.2), scanning entity-tags as net/http's
// file server does: an entity-tag may hold a comma, and a malformed one ends
// the value.
func noneMatch(tags, etag string) bool {
	etag = strings.TrimPrefix(etag, "W/")
	for {
		tags = strings.TrimLeft(tags, " \t,")
		if tags == "" {
			return false
		}
		if tags[0] == '*' {
			return true
		}
		tag := strings.TrimPrefix(tags, "W/")
		if !strings.HasPrefix(tag, `"`) {
			return false
		}
		end := strings.IndexByte(tag[1:], '"') + 2
		if end < 2 {
			return false
		}
		if tag[:end] == etag {
			return true
		}
		tags = tag[end:]
	}
}

// notModifiedHeader trims a 2xx's header to its 304's, as net/http's file
// server does (RFC 9110 §15.4.5): no representation metadata beyond what
// updates the client's stored copy, so Last-Modified stays only without an
// ETag.
func notModifiedHeader(header http.Header) {
	header.Del("Content-Type")
	header.Del("Content-Length")
	header.Del("Content-Encoding")
	if header.Get("ETag") != "" {
		header.Del("Last-Modified")
	}
}

// upstreamAge is the Age the upstream gave a response (RFC 9111 §5.1): none,
// or one that is not a count of seconds, is zero, and one too large counts as
// 2^31 - 1 seconds (§1.2.2).
func upstreamAge(header http.Header) time.Duration {
	seconds, err := strconv.ParseUint(strings.TrimSpace(header.Get("Age")), 10, 31)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0
	}
	return time.Duration(seconds) * time.Second
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

// teeWriter passes a leader's response through and gives its fill a copy of
// the body.
type teeWriter struct {
	http.ResponseWriter
	st *fill
}

func (t *teeWriter) Write(p []byte) (int, error) {
	_, _ = t.st.Write(p)
	return t.ResponseWriter.Write(p)
}

func (t *teeWriter) Flush() {
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (t *teeWriter) Unwrap() http.ResponseWriter { return t.ResponseWriter }

// store keeps e, fetched under generation, unless it is larger than the whole
// cache. current is false, and nothing is kept, when a purge came after the
// fetch began: e is then no answer for the requests waiting on it either.
func (h *Handler) store(e *entry, generation uint64) (current bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if generation != h.generation {
		return false
	}
	if old, ok := h.entries[e.key]; ok {
		h.removeLocked(old)
	}
	cost := entryCost(e)
	if cost > h.cfg.MaxBytes {
		return true
	}
	// Expired entries leave first, from the cold end, whether or not their key
	// is ever asked for again; then the least recently used make room.
	now := h.cfg.Now()
	for el := h.lru.Back(); el != nil && !now.Before(el.Value.(*entry).expires); el = h.lru.Back() {
		h.removeLocked(el)
	}
	for h.size+cost > h.cfg.MaxBytes && h.lru.Len() > 0 {
		h.removeLocked(h.lru.Back())
	}
	h.entries[e.key] = h.lru.PushFront(e)
	h.size += cost
	return true
}

func (h *Handler) removeLocked(el *list.Element) {
	e := el.Value.(*entry)
	h.lru.Remove(el)
	delete(h.entries, e.key)
	h.size -= entryCost(e)
}

// entryOverhead is the fixed cost charged for an entry's own structure, beyond
// its body, key and headers: the list element, the two map slots, the entry and
// the cloned header map. It was measured at about 756 bytes per entry (w21/m394
// t015, R25-06) and is rounded up. Charging it makes the byte budget bound how
// many entries the cache holds too, so a flood of unique tiny 404s cannot
// exhaust the process's memory behind an honest-looking byte count.
const entryOverhead = 1024

// entryCost is what an entry is charged against the byte budget: everything the
// cache keeps for it. store and removeLocked share it, so an entry is released
// for exactly what it was charged.
func entryCost(e *entry) int64 {
	// Capacity, not length: the body comes from a bytes.Buffer, whose spare
	// capacity stays allocated while the entry lives.
	cost := int64(cap(e.body)+len(e.key)) + entryOverhead
	for name, values := range e.header {
		cost += int64(len(name))
		for _, value := range values {
			cost += int64(len(value))
		}
	}
	return cost
}

// writeEntry answers r from a stored page: 304 Not Modified when r's
// validators match it, the page otherwise. Either carries the page's Age, in
// place of any it was stored with (RFC 9111 §4, §5.1).
func (h *Handler) writeEntry(w http.ResponseWriter, r *http.Request, e *entry) {
	header := w.Header()
	for k, vs := range e.header {
		header[k] = append([]string(nil), vs...)
	}
	header.Set(StatusHeader, statusHit)
	header.Set("Age", strconv.FormatInt(int64(max(h.cfg.Now().Sub(e.born), 0)/time.Second), 10))
	if notModified(r.Header, e.status, header) {
		notModifiedHeader(header)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	header.Set("Content-Length", strconv.Itoa(len(e.body)))
	w.WriteHeader(e.status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(e.body)
	}
}
