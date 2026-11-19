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
// (render.com/docs/web-service-caching) as a caching reverse proxy in front of
// the app (w5/m151). It caches what Render caches and reports it the way
// Render does:
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
// httputil.ReverseProxy, so the app sees the same headers either way.
//
// It runs in one of two shapes. As a sidecar in each app pod (w5/m151) it
// keeps bodies in memory (Config.Dir empty) and has no peers, so the cache
// lives and dies with the pod. As a service's cache tier (w3/m489), a small
// Deployment in front of the app's Service, it keeps bodies on its pod's
// bounded disk (Config.Dir) and only their metadata in memory, and the
// replicas share one cache by key affinity (peers.go): each key has one owner
// replica, the others forward it there, so every key is filled once for the
// whole service and a restart, a scale change or a crash of the app keeps it.
// A manual purge, a profile switch and, for the tier, a successful deploy reach
// a running proxy through its control file (control.go, w3/m486), so none
// restarts an instance, as on Render's CDN. A proxy process always starts
// empty: it cannot know which purges it missed while it was down.
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
	"os"
	"path"
	"path/filepath"
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
	// MaxFills how many responses may be kept for storing at once, each in
	// memory or in a temporary file under Dir.
	MaxBytes, MaxObjectBytes int64
	MaxFills                 int
	// Dir, when set, keeps response bodies as files in this directory, never
	// whole in memory; New creates it and empties it. MaxBytes then bounds the
	// stored bodies on disk, and MaxIndexBytes the metadata kept in memory for
	// them (key, header and each entry's own structure; MaxBytes/32 when not
	// positive). Empty, everything an entry holds is in memory and MaxBytes
	// bounds it all.
	Dir           string
	MaxIndexBytes int64
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Handler is the caching proxy in front of upstream.
type Handler struct {
	cfg     Config
	proxy   *httputil.ReverseProxy
	fills   chan struct{}
	profile atomic.Pointer[string]
	// peers is the tier this replica shares its cache with (peers.go); nil or
	// empty, every request is served here.
	peers     atomic.Pointer[peerSet]
	forwarder *httputil.ReverseProxy

	mu      sync.Mutex
	entries map[string]*list.Element
	lru     *list.List
	// size is what the entries are charged against MaxBytes, index what they
	// are charged against MaxIndexBytes (disk mode only): see charge.
	size, index int64
	flights     map[string]*flight
	// unlinked holds the body files of entries that left while mu was held;
	// unlock removes them.
	unlinked []string
	// generation counts purges: a fill started under an older one is not
	// stored, so a purge empties the cache of responses fetched before it. It
	// changes only under mu, and a fill in flight reads it without the lock to
	// drop its temporary file at once.
	generation atomic.Uint64
}

type entry struct {
	key    string
	status int
	header http.Header
	// The body is in body, or, in disk mode, in the file at path file (none
	// for an empty body); size is its length either way.
	body []byte
	file string
	size int64
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
	// The copy kept for storing, at most maxBody bytes: in body, or in disk
	// mode in a temporary file under dir, created at the first byte. A longer
	// body, a purge or a disk failure discards it, and nothing is stored.
	body      bytes.Buffer
	dir       string
	file      *os.File
	written   int64
	maxBody   int64
	discarded bool
	// generation is the purge count the fill began under; purges is the
	// handler's, which moves past it when a purge comes.
	generation uint64
	purges     *atomic.Uint64
}

// Write keeps p for storing while the response is kept and still fits.
func (st *fill) Write(p []byte) (int, error) {
	if !st.keep || st.discarded || len(p) == 0 {
		return len(p), nil
	}
	if st.written+int64(len(p)) > st.maxBody || st.purges.Load() != st.generation {
		st.discard()
		return len(p), nil
	}
	if st.dir == "" {
		st.body.Write(p)
	} else if err := st.writeFile(p); err != nil {
		st.discard()
		return len(p), nil
	}
	st.written += int64(len(p))
	return len(p), nil
}

func (st *fill) writeFile(p []byte) error {
	if st.file == nil {
		f, err := os.CreateTemp(st.dir, "fill-*")
		if err != nil {
			return err
		}
		st.file = f
	}
	_, err := st.file.Write(p)
	return err
}

// discard drops the kept copy, removing its temporary file, and releases the
// requests waiting on the fill to fetch for themselves: nothing will be
// stored for them.
func (st *fill) discard() {
	if st.file != nil {
		_ = st.file.Close()
		_ = os.Remove(st.file.Name())
		st.file = nil
	}
	st.body = bytes.Buffer{}
	st.discarded = true
	st.flight.settle(nil)
}

// entry is the kept response as an entry to store, its temporary file closed
// and handed over, or nil when nothing is kept.
func (st *fill) entry(now time.Time) *entry {
	if !st.keep || st.discarded {
		return nil
	}
	e := &entry{
		key: st.key, status: st.status, header: st.header, size: st.written,
		born: now.Add(-upstreamAge(st.header)), expires: now.Add(st.ttl),
	}
	if st.file != nil {
		if err := st.file.Close(); err != nil {
			st.discard()
			return nil
		}
		e.file = st.file.Name()
		st.file = nil
	} else if st.dir == "" {
		e.body = st.body.Bytes()
	}
	return e
}

// New returns a Handler proxying to upstream (e.g. http://127.0.0.1:10000).
// With cfg.Dir set it creates the directory and removes everything in it.
func New(upstream *url.URL, cfg Config) (*Handler, error) {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.MaxFills <= 0 {
		cfg.MaxFills = 4
	}
	if cfg.Dir != "" {
		if cfg.MaxIndexBytes <= 0 {
			cfg.MaxIndexBytes = cfg.MaxBytes / 32
		}
		if err := emptyDir(cfg.Dir); err != nil {
			return nil, err
		}
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
			// The hop marker is between replicas; the app never sees it.
			pr.Out.Header.Del(HopHeader)
			setForwarded(pr)
		},
		// The tier reaches the app over the cluster network through one
		// Service address, so it keeps a pool of connections to it, where
		// http.DefaultTransport keeps two idle: past two requests at once every
		// request would open, and leave behind, a connection of its own.
		Transport:  upstreamTransport(),
		BufferPool: &proxyBuffers,
		// Flush as bytes arrive, so streamed and long-poll responses flow.
		FlushInterval:  -1,
		ModifyResponse: h.decide,
	}
	h.forwarder = newForwarder(h)
	return h, nil
}

// upstreamTransport is http.DefaultTransport with an idle pool sized for a
// tier's whole service.
func upstreamTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns, t.MaxIdleConnsPerHost = 256, 128
	return t
}

// proxyBuffers lends the proxies their copy buffers, so a proxied response
// allocates none.
var proxyBuffers bufferPool

type bufferPool struct{ sync.Pool }

func (p *bufferPool) Get() []byte {
	if b, ok := p.Pool.Get().(*[]byte); ok {
		return *b
	}
	return make([]byte, 32<<10)
}

func (p *bufferPool) Put(b []byte) { p.Pool.Put(&b) }

// emptyDir creates dir and removes everything in it. A process that restarts
// cannot know which purges it missed while it was down, so it starts empty,
// as a new instance does.
func emptyDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	names, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := os.RemoveAll(filepath.Join(dir, name.Name())); err != nil {
			return err
		}
	}
	return nil
}

// setForwarded sets the X-Forwarded-For, -Host and -Proto the app sees.
// ReverseProxy strips the inbound ones. Traefik sets them and is the only edge
// that reaches this proxy, so the app sees the client address and scheme the
// edge saw, as through the activator's proxy (ADR004; w21/m394 t015, R25-07):
// the inbound X-Forwarded-For is copied before SetXForwarded, which appends the
// peer, and an http or https X-Forwarded-Proto replaces its "http". A request
// another replica forwarded (an accepted hop, peers.go) carries the three as
// that replica computed them here for its own request, so they pass on
// verbatim: the app sees the same whichever replica fills.
func setForwarded(pr *httputil.ProxyRequest) {
	if hop, _ := pr.In.Context().Value(hopKey{}).(bool); hop {
		for _, name := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"} {
			if values := pr.In.Header.Values(name); len(values) > 0 {
				pr.Out.Header[name] = slices.Clone(values)
			}
		}
		return
	}
	if forwardedFor := pr.In.Header.Values("X-Forwarded-For"); len(forwardedFor) > 0 {
		pr.Out.Header["X-Forwarded-For"] = slices.Clone(forwardedFor)
	}
	pr.SetXForwarded()
	if proto := strings.ToLower(pr.In.Header.Get("X-Forwarded-Proto")); proto == "https" || proto == "http" {
		pr.Out.Header.Set("X-Forwarded-Proto", proto)
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hop := h.acceptedHop(r)
	if hop {
		r = r.WithContext(context.WithValue(r.Context(), hopKey{}, true))
	}
	if !h.requestEligible(r) {
		w.Header().Set(StatusHeader, statusDynamic)
		h.proxy.ServeHTTP(w, r)
		return
	}
	key := cacheKey(r)
	if !hop {
		if owner := h.owner(key); owner != "" {
			h.forward(w, r, key, owner)
			return
		}
	}
	h.serve(w, r, key)
}

// serve answers an eligible request from this replica's own cache, filling it
// on a miss.
func (h *Handler) serve(w http.ResponseWriter, r *http.Request, key string) {
	e, body, expired := h.lookup(key)
	if e != nil {
		h.writeEntry(w, r, e, body)
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
// through (DYNAMIC): caching turned off, until the route leaves the proxy. It
// reports whether the profile changed.
func (h *Handler) SetProfile(profile string) bool {
	if h.Profile() == profile {
		return false
	}
	h.profile.Store(&profile)
	h.Purge()
	return true
}

// Purge empties the cache: every stored entry leaves, its body file with it,
// and a fill already in flight is served to its own client but not stored.
func (h *Handler) Purge() {
	h.mu.Lock()
	defer h.unlock()
	for el := h.lru.Front(); el != nil; el = el.Next() {
		h.unlinkLocked(el.Value.(*entry))
	}
	clear(h.entries)
	h.lru.Init()
	h.size, h.index = 0, 0
	h.generation.Add(1)
}

// unlock releases h.mu, then removes the body files of the entries that left
// while it was held: no request waits on the disk behind the lock, and a file
// out of the index is never opened again (one a request already has open
// stays readable until it closes it).
func (h *Handler) unlock() {
	files := h.unlinked
	h.unlinked = nil
	h.mu.Unlock()
	for _, name := range files {
		_ = os.Remove(name)
	}
}

func (h *Handler) unlinkLocked(e *entry) {
	if e.file != "" {
		h.unlinked = append(h.unlinked, e.file)
	}
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

// lookup is the live entry for key, its body file open when it has one, or
// nil and whether an expired entry was dropped. The file is opened under h.mu,
// so an eviction cannot unlink it first; an entry whose file will not open is
// dropped and the request is a miss.
func (h *Handler) lookup(key string) (*entry, *os.File, bool) {
	h.mu.Lock()
	defer h.unlock()
	el, ok := h.entries[key]
	if !ok {
		return nil, nil, false
	}
	e := el.Value.(*entry)
	if !h.cfg.Now().Before(e.expires) {
		h.removeLocked(el)
		return nil, nil, true
	}
	body, err := openBody(e)
	if err != nil {
		h.removeLocked(el)
		return nil, nil, false
	}
	h.lru.MoveToFront(el)
	return e, body, false
}

// openStored opens the body of e, the entry a fill stored, for a request that
// waited on the fill: under h.mu and only while e is still stored, since an
// eviction removes the file. It reports false when that body is gone.
func (h *Handler) openStored(e *entry) (*os.File, bool) {
	if e.file == "" {
		return nil, true
	}
	h.mu.Lock()
	defer h.unlock()
	if el, ok := h.entries[e.key]; !ok || el.Value.(*entry) != e {
		return nil, false
	}
	body, err := openBody(e)
	return body, err == nil
}

// openBody opens e's body file; an entry without one has nothing to open.
func openBody(e *entry) (*os.File, error) {
	if e.file == "" {
		return nil, nil
	}
	return os.Open(e.file)
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
		if e := f.stored; e != nil {
			if body, ok := h.openStored(e); ok {
				h.writeEntry(w, r, e, body)
				return
			}
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
	generation := h.generation.Load()
	h.mu.Unlock()
	st := &fill{
		key: key, fetched: fetched, flight: f, client: r.Header,
		dir: h.cfg.Dir, maxBody: h.cfg.MaxObjectBytes, generation: generation, purges: &h.generation,
	}
	defer func() {
		// A temporary file the store did not take is removed, whether the
		// body was too long, purged, or cut off by a panic in the proxy.
		if st.file != nil {
			st.discard()
		}
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
	out = out.WithContext(context.WithValue(out.Context(), fillKey{}, st))
	out.Header.Del("If-None-Match")
	out.Header.Del("If-Modified-Since")
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		out.Header.Set("Accept-Encoding", "gzip")
	} else {
		out.Header.Set("Accept-Encoding", "identity")
	}
	h.proxy.ServeHTTP(&teeWriter{ResponseWriter: w, st: st}, out)
	if e := st.entry(h.cfg.Now()); e != nil && h.store(e, generation) {
		f.settle(e)
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
			st.discard()
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

// store keeps e, fetched under generation, unless it is larger than the
// cache's bounds; a body file it does not keep it removes. It reports whether
// e answers the requests waiting on its fill: not when a purge came after the
// fetch began (nothing is kept then), and not when its body file is gone.
func (h *Handler) store(e *entry, generation uint64) bool {
	h.mu.Lock()
	defer h.unlock()
	if generation != h.generation.Load() {
		h.unlinkLocked(e)
		return false
	}
	if old, ok := h.entries[e.key]; ok {
		h.removeLocked(old)
	}
	size, index := h.charge(e)
	if !h.within(size, index) {
		h.unlinkLocked(e)
		return e.file == ""
	}
	// Expired entries leave first, from the cold end, whether or not their key
	// is ever asked for again; then the least recently used make room.
	now := h.cfg.Now()
	for el := h.lru.Back(); el != nil && !now.Before(el.Value.(*entry).expires); el = h.lru.Back() {
		h.removeLocked(el)
	}
	for !h.within(h.size+size, h.index+index) && h.lru.Len() > 0 {
		h.removeLocked(h.lru.Back())
	}
	h.entries[e.key] = h.lru.PushFront(e)
	h.size += size
	h.index += index
	return true
}

func (h *Handler) removeLocked(el *list.Element) {
	e := el.Value.(*entry)
	h.lru.Remove(el)
	delete(h.entries, e.key)
	size, index := h.charge(e)
	h.size -= size
	h.index -= index
	h.unlinkLocked(e)
}

// within reports whether size bytes against MaxBytes and index bytes against
// MaxIndexBytes fit the cache's bounds.
func (h *Handler) within(size, index int64) bool {
	return size <= h.cfg.MaxBytes && (h.cfg.Dir == "" || index <= h.cfg.MaxIndexBytes)
}

// diskBlock is the allocation unit a body file is charged in: the kubelet
// measures an emptyDir's use by the blocks its files hold, so a small body
// costs a whole block of the volume's size limit.
const diskBlock = 4096

// charge is what e is charged against the cache's two bounds, so store and
// removeLocked agree. In memory everything e holds counts against MaxBytes,
// and there is no index bound; on disk MaxBytes is charged the blocks its body
// file takes and MaxIndexBytes the metadata kept in memory.
func (h *Handler) charge(e *entry) (size, index int64) {
	if h.cfg.Dir == "" {
		return entryCost(e), 0
	}
	return (e.size + diskBlock - 1) / diskBlock * diskBlock, metaCost(e) + int64(len(e.file))
}

// entryOverhead is the fixed cost charged for an entry's own structure, beyond
// its body, key and headers: the list element, the two map slots, the entry and
// the cloned header map. It was measured at about 756 bytes per entry (w21/m394
// t015, R25-06) and is rounded up. Charging it makes the byte budget bound how
// many entries the cache holds too, so a flood of unique tiny 404s cannot
// exhaust the process's memory behind an honest-looking byte count.
const entryOverhead = 1024

// entryCost is what an entry held in memory is charged against the byte
// budget: everything the cache keeps for it.
func entryCost(e *entry) int64 {
	// Capacity, not length: the body comes from a bytes.Buffer, whose spare
	// capacity stays allocated while the entry lives.
	return int64(cap(e.body)) + metaCost(e)
}

// metaCost is what an entry's metadata costs in memory: its key, its header
// and its own structure.
func metaCost(e *entry) int64 {
	cost := int64(len(e.key)) + entryOverhead
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
// place of any it was stored with (RFC 9111 §4, §5.1). body is the page's open
// body file, when it has one; writeEntry closes it.
func (h *Handler) writeEntry(w http.ResponseWriter, r *http.Request, e *entry, body *os.File) {
	if body != nil {
		defer func() { _ = body.Close() }()
	}
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
	header.Set("Content-Length", strconv.FormatInt(e.size, 10))
	w.WriteHeader(e.status)
	switch {
	case r.Method == http.MethodHead:
	case body != nil:
		// The file was opened before any eviction could unlink it, and an
		// open file stays readable after its name is gone.
		_, _ = io.Copy(w, body)
	default:
		_, _ = w.Write(e.body)
	}
}
