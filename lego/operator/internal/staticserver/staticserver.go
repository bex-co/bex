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

// Package staticserver is the shared static-site origin proxy (w1/m21): one
// always-on HTTP server, behind Traefik, that serves every static_site App's
// built output from the object-store origin. It dispatches by request Host to
// the App's current revision prefix, fetches objects with signed GETs (the
// bucket stays private), and applies the App's edge rules — redirects/rewrites
// (Spec.Routes, Render's /routes) and custom response headers (Spec.Headers,
// Render's /headers) — plus index.html defaulting and SPA fallback. Objects are
// cached in memory: a revision prefix is immutable, so a hit is never stale, and
// caching keeps egress inside the object store's fair-use budget.
//
// The handler is decoupled from Kubernetes and S3 via two seams — Resolver
// (host → Site) and Origin (key → bytes) — so the edge-rule behavior is unit
// tested with fakes (staticserver_test.go); the real wiring lives in resolver.go
// (a controller-runtime cache over static_site Apps) and s3origin.go.
package staticserver

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

	"golang.org/x/sync/semaphore"
	"golang.org/x/sync/singleflight"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ErrNotFound is what an Origin returns when the requested key does not exist.
var ErrNotFound = errors.New("staticserver: object not found")

// ErrObjectTooLarge is what an Origin returns when a tenant-controlled object
// exceeds maxOriginObjectBytes, so a single oversized asset can't be allocated
// whole and OOM the shared single-replica server (codex-security #10).
var ErrObjectTooLarge = errors.New("staticserver: object too large")

// ErrOverloaded is returned when the origin-fetch admission gate is at capacity;
// the handler sheds it as 503 rather than buffering unbounded misses (finding 12).
var ErrOverloaded = errors.New("staticserver: origin fetch capacity reached")

// maxOriginObjectBytes bounds a single origin object read into memory. Generous
// for real static assets, far below the pod memory budget.
const maxOriginObjectBytes = 32 << 20 // 32 MiB

// maxObjectKeyBytes is the object store's hard cap on an object key's length
// (S3 caps keys at 1024 bytes, counting the site's revision prefix). A longer
// derived key can never name an existing object, so the handler answers 404 up
// front rather than letting the store's key-length client error surface as a
// 502 "origin error" (w6/047).
const maxObjectKeyBytes = 1024

// DefaultCacheBytes is the default in-memory object cache budget (main.go may
// override it via BEX_STATIC_CACHE_BYTES).
const DefaultCacheBytes = 256 << 20 // 256 MiB

// defaultMaxLiveBodyBytes bounds the summed size of response bodies held live
// while being written to clients. The fetch gate releases its reservation when
// the origin read completes, but the fetched body stays allocated until the
// response write finishes — and a cache entry can be evicted mid-write, taking
// the body out of cache accounting — so distinct large objects served to slow
// clients need their own budget. Weighted by actual body size; on exhaustion
// the handler sheds with 503 rather than writing past the budget.
//
// Aggregate memory budget (codex-security round 18): the three independent
// ceilings must sum well below the 2 GiB pod limit in
// config/staticserver/deployment.yaml:
//
//	cache            256 MiB  (DefaultCacheBytes)
//	live bodies      256 MiB  (defaultMaxLiveBodyBytes)
//	fetch gate       512 MiB  (16 fetches x 32 MiB, fetchgate.go)
//	─────────────────────────
//	worst case     1024 MiB = 50% of the 2 GiB cgroup limit
//
// leaving ~1 GiB for slice-capacity growth, io.ReadAll transients, the Go
// runtime/GC (further softened by GOMEMLIMIT=1500MiB), the S3 SDK, and the
// resolver. budget_test.go asserts this invariant so a knob bump trips CI.
const defaultMaxLiveBodyBytes = 256 << 20 // 256 MiB

// Object is a fetched origin object.
type Object struct {
	Body        []byte
	ContentType string // from the origin; may be empty (then inferred from extension)
	// negative marks a cached confirmed miss. A revision prefix is immutable,
	// so "this key does not exist" is as permanent as any object within the
	// revision — caching it keeps the lookup-before-rules order (w4/m94) from
	// re-asking the origin for the same SPA deep link or redirect path on
	// every request, and from letting those misses exhaust the per-site fetch
	// slots real objects need.
	negative bool
}

// Origin fetches objects from the static-site object store by key.
type Origin interface {
	// Get returns the object at key, or ErrNotFound if it does not exist.
	Get(ctx context.Context, key string) (Object, error)
}

// Site is the serving config for one static_site App, keyed by request host.
type Site struct {
	AppID    string                     // first object-key segment (legacy sites)
	Revision string                     // last key segment (e.g. "rev-7"); immutable per revision
	Prefix   string                     // full object-key prefix including trailing slash when known
	Routes   []appv1alpha1.StaticRoute  // ordered redirect/rewrite rules
	Headers  []appv1alpha1.StaticHeader // custom response headers by path
}

// keyFor is the object-store key for a site-relative request path.
func (s Site) keyFor(reqPath string) string {
	prefix := s.Prefix
	if prefix == "" {
		prefix = s.AppID + "/" + s.Revision + "/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix + strings.TrimPrefix(reqPath, "/")
}

// Resolver maps a request host to its Site config.
type Resolver interface {
	// Resolve returns the Site for host, or ok=false when no static_site App
	// serves that host.
	Resolve(host string) (Site, bool)
}

// Handler is the static-site origin proxy. Construct with New.
type Handler struct {
	resolver Resolver
	origin   Origin
	cache    *cache
	// group collapses concurrent misses for the same object key into one origin
	// fetch (finding 12): a viral asset is fetched once, not once per request.
	group singleflight.Group
	// gate bounds concurrent origin fetches (count + in-flight bytes) so a burst
	// of distinct misses can't buffer an unbounded amount of memory (finding 12).
	gate *fetchGate
	// liveBodies bounds the total bytes of response bodies held live while being
	// written to clients, extending memory accounting from fetch until the
	// response write completes (the fetch gate alone releases at read time).
	liveBodies *semaphore.Weighted
}

// New builds a Handler over a Resolver and Origin. cacheBytes caps the in-memory
// object cache (0 disables caching).
func New(resolver Resolver, origin Origin, cacheBytes int64) *Handler {
	return &Handler{
		resolver:   resolver,
		origin:     origin,
		cache:      newCache(cacheBytes),
		gate:       newFetchGate(defaultMaxConcurrentFetches, defaultMaxInflightBytes, defaultMaxSiteFetches),
		liveBodies: semaphore.NewWeighted(defaultMaxLiveBodyBytes),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	site, ok := h.resolver.Resolve(hostOnly(r.Host))
	if !ok {
		http.Error(w, "no static site for host", http.StatusNotFound)
		return
	}

	// requestPath is the visitor's normalized path and stays immutable: custom
	// headers match against it on every response (Render's request-path
	// contract, w4/m94), while the (possibly rewritten) served path drives
	// origin keys, content type, and the default cache policy.
	requestPath := normalizePath(r.URL.Path)

	// An existing published object wins over both redirect and rewrite rules,
	// matching Render's documented order. Only a genuine miss advances to the
	// ordered rules; permission denials, timeouts, capacity and size failures
	// keep their own classes below rather than degrading into a rule.
	//
	// The store cannot hold a key past maxObjectKeyBytes, so a path whose
	// derived key (after index.html defaulting) is longer can never match an
	// object — nor plausibly be an SPA route. Treat it as a miss without an
	// origin round trip instead of misreporting the store's key-length client
	// error as an origin failure (w6/047).
	var obj Object
	var servedPath string
	err := ErrNotFound
	overlong := overlongKey(site, requestPath)
	if !overlong {
		obj, servedPath, err = h.fetch(r.Context(), site, requestPath)
	}
	if err == nil {
		h.serveObject(w, r, site, obj, servedPath, requestPath)
		return
	}
	if !errors.Is(err, ErrNotFound) {
		serveOriginError(w, err)
		return
	}

	// Miss: edge rules run in order, first match wins.
	act, target := matchRoutes(site.Routes, requestPath)
	if act == actRedirect {
		if !safeRedirectTarget(target) {
			http.Error(w, "invalid redirect target", http.StatusBadRequest)
			return
		}
		applyHeaders(w.Header(), site.Headers, requestPath)
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}

	if act == actNone && overlong {
		// No rule claimed the over-long path; it can never be an object and is
		// not plausibly an SPA route either.
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	fallbackPath := requestPath
	if act == actRewrite {
		fallbackPath = normalizePath(target)
		if overlongKey(site, fallbackPath) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		obj, servedPath, err = h.fetch(r.Context(), site, fallbackPath)
		if err != nil && !errors.Is(err, ErrNotFound) {
			serveOriginError(w, err)
			return
		}
		if err == nil {
			h.serveObject(w, r, site, obj, servedPath, requestPath)
			return
		}
	}

	// SPA fallback: if the miss (after any rewrite) is a bare path with no
	// extension and an index.html exists at the root, serve it so client-side
	// routing works even without an explicit route. A missing asset (has an
	// extension) stays a 404.
	if fallback, fok := h.spaFallback(r.Context(), site, fallbackPath); fok {
		h.serveObject(w, r, site, fallback, "/index.html", requestPath)
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

// serveOriginError maps a non-miss fetch failure onto its response class:
// oversize 413, overload 503 + Retry-After, anything else 502.
func serveOriginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrObjectTooLarge):
		http.Error(w, "object too large", http.StatusRequestEntityTooLarge)
	case errors.Is(err, ErrOverloaded):
		w.Header().Set("Retry-After", "1")
		http.Error(w, "server busy", http.StatusServiceUnavailable)
	default:
		http.Error(w, "origin error", http.StatusBadGateway)
	}
}

// serveObject writes a successful object response. servedPath (the resolved
// object) drives content type and cache policy; requestPath (the visitor's
// path) drives custom header matching.
func (h *Handler) serveObject(w http.ResponseWriter, r *http.Request, site Site, obj Object, servedPath, requestPath string) {
	// Hold a live-body lease for the whole write: the body stays allocated
	// until the response reaches the client, so slow clients must count
	// against the in-flight memory budget even after the fetch gate released
	// its reservation. HEAD responses carry no body and need no lease. On
	// exhaustion shed with 503 (before any success header is written) rather
	// than buffering past the budget.
	if r.Method != http.MethodHead {
		lease := int64(len(obj.Body))
		if !h.liveBodies.TryAcquire(lease) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "server busy", http.StatusServiceUnavailable)
			return
		}
		defer h.liveBodies.Release(lease)
	}

	hdr := w.Header()
	hdr.Set("Content-Type", contentType(servedPath, obj.ContentType))
	hdr.Set("Cache-Control", cacheControl(servedPath))
	// Custom headers win over the defaults above (last-write for a given key).
	applyHeaders(hdr, site.Headers, requestPath)

	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(obj.Body)
}

// safeRedirectTarget validates the final, expanded Location at the output sink.
// Route validation cannot prove this invariant because :splat is request data;
// browsers normalize reverse solidus like slash in navigation URLs.
func safeRedirectTarget(target string) bool {
	return strings.HasPrefix(target, "/") &&
		!strings.HasPrefix(target, "//") &&
		!strings.HasPrefix(target, `/\`) &&
		!strings.ContainsAny(target, "\\\r\n\x00")
}

// fetch resolves reqPath to an object, applying index.html defaulting for "/"
// and directory-style paths. Returns the served key path for content typing.
func (h *Handler) fetch(ctx context.Context, site Site, reqPath string) (Object, string, error) {
	lookup := lookupPath(reqPath)
	obj, err := h.get(ctx, site, lookup)
	return obj, lookup, err
}

// overlongKey reports whether p's derived object key (after index.html
// defaulting) exceeds the store's key cap and so can never match an object.
func overlongKey(site Site, p string) bool {
	return len(site.keyFor(lookupPath(p))) > maxObjectKeyBytes
}

// lookupPath applies index.html defaulting: "/" and directory-style paths
// resolve to the index.html beneath them.
func lookupPath(reqPath string) string {
	if reqPath == "/" || strings.HasSuffix(reqPath, "/") {
		return strings.TrimSuffix(reqPath, "/") + "/index.html"
	}
	return reqPath
}

// spaFallback fetches the site's root index.html for extension-less misses so a
// single-page app's client-side routes resolve. Returns ok=false for asset-like
// paths (those with a file extension) or when there is no root index.html.
func (h *Handler) spaFallback(ctx context.Context, site Site, reqPath string) (Object, bool) {
	if path.Ext(reqPath) != "" {
		return Object{}, false
	}
	obj, err := h.get(ctx, site, "/index.html")
	if err != nil {
		return Object{}, false
	}
	return obj, true
}

// get fetches a site-relative path through the cache. A revision prefix is
// immutable, so a cached object is never stale within a revision. Concurrent
// misses for the same key are collapsed into a single origin fetch (singleflight)
// and admitted through the fetch gate, so a burst can neither stampede the origin
// nor buffer unbounded memory (finding 12).
func (h *Handler) get(ctx context.Context, site Site, reqPath string) (Object, error) {
	key := site.keyFor(reqPath)
	if obj, ok := h.cache.get(key); ok {
		return obj, negativeErr(obj)
	}
	v, err, _ := h.group.Do(key, func() (any, error) {
		// A prior leader for this key may have just populated the cache.
		if obj, ok := h.cache.get(key); ok {
			return obj, nil
		}
		if !h.gate.acquire(site.AppID) {
			return Object{}, ErrOverloaded
		}
		defer h.gate.release(site.AppID)
		obj, gerr := h.origin.Get(ctx, key)
		if errors.Is(gerr, ErrNotFound) {
			// A confirmed miss is immutable within the revision too; cache it
			// so it stops costing origin round trips and fetch-gate slots.
			// Only ErrNotFound: a denial, timeout, or overload is transient
			// and must be re-asked.
			miss := Object{negative: true}
			h.cache.put(site.AppID, key, miss)
			return miss, nil
		}
		if gerr != nil {
			return Object{}, gerr
		}
		h.cache.put(site.AppID, key, obj)
		return obj, nil
	})
	if err != nil {
		return Object{}, err
	}
	obj := v.(Object)
	return obj, negativeErr(obj)
}

// negativeErr maps a cached confirmed miss back onto the ErrNotFound contract.
func negativeErr(obj Object) error {
	if obj.negative {
		return ErrNotFound
	}
	return nil
}

type routeAction int

const (
	actNone routeAction = iota
	actRedirect
	actRewrite
)

// matchRoutes evaluates the ordered routes against reqPath, first match wins,
// returning the action and the expanded destination. Source patterns support a
// trailing "/*" wildcard; the captured remainder (the "splat") substitutes a
// trailing "/*" or a ":splat" token in the destination.
func matchRoutes(routes []appv1alpha1.StaticRoute, reqPath string) (routeAction, string) {
	for _, rt := range routes {
		splat, ok := matchPattern(rt.Source, reqPath)
		if !ok {
			continue
		}
		dest := expandDest(rt.Destination, splat)
		switch rt.Type {
		case "redirect":
			return actRedirect, dest
		case "rewrite":
			return actRewrite, dest
		}
	}
	return actNone, ""
}

// matchPattern reports whether reqPath matches pattern and returns the splat (the
// text captured by a trailing "/*"). "/*" matches everything (splat = the path
// minus its leading slash); "/foo/*" matches "/foo" and "/foo/..."; any other
// pattern is an exact match (splat empty).
func matchPattern(pattern, reqPath string) (string, bool) {
	if pattern == "/*" {
		return strings.TrimPrefix(reqPath, "/"), true
	}
	if prefix, ok := strings.CutSuffix(pattern, "/*"); ok { // "/foo/*" -> "/foo"
		if reqPath == prefix {
			return "", true
		}
		if rest, ok := strings.CutPrefix(reqPath, prefix+"/"); ok {
			return rest, true
		}
		return "", false
	}
	return "", pattern == reqPath
}

// expandDest substitutes the splat into dest: a trailing "/*" or a ":splat"
// token is replaced with the captured remainder.
func expandDest(dest, splat string) string {
	if strings.Contains(dest, ":splat") {
		return strings.ReplaceAll(dest, ":splat", splat)
	}
	if strings.HasSuffix(dest, "/*") {
		return strings.TrimSuffix(dest, "*") + splat // ".../*" -> ".../" + splat
	}
	return dest
}

// applyHeaders adds every custom header whose path pattern matches reqPath. Set
// (not Add) so a repeated header name resolves to the last matching rule.
func applyHeaders(h http.Header, headers []appv1alpha1.StaticHeader, reqPath string) {
	for _, rule := range headers {
		if _, ok := matchPattern(rule.Path, reqPath); ok {
			h.Set(rule.Name, rule.Value)
		}
	}
}

// normalizePath cleans a request path to a rooted, slash-separated form,
// preserving a trailing slash (which drives index.html defaulting).
func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	trailing := strings.HasSuffix(p, "/") && p != "/"
	c := path.Clean("/" + strings.TrimPrefix(p, "/"))
	if trailing && !strings.HasSuffix(c, "/") {
		c += "/"
	}
	return c
}

// hostOnly strips a port from a Host header value.
func hostOnly(host string) string {
	h, _, _ := strings.Cut(host, ":")
	return h
}

// contentType prefers the origin's content type, else infers from the extension,
// else falls back to octet-stream.
func contentType(servedPath, originType string) string {
	if originType != "" && originType != "application/octet-stream" {
		return originType
	}
	if ct := mime.TypeByExtension(path.Ext(servedPath)); ct != "" {
		return ct
	}
	if originType != "" {
		return originType
	}
	return "application/octet-stream"
}

// cacheControl returns a client/CDN caching policy: HTML is revalidated often (a
// deploy swaps it), other assets are treated as immutable (build tooling
// content-hashes them, and each revision lives under its own prefix anyway).
func cacheControl(servedPath string) string {
	switch path.Ext(servedPath) {
	case ".html", ".htm", "":
		return "public, max-age=0, must-revalidate"
	default:
		return "public, max-age=31536000, immutable"
	}
}

// drain fully reads and closes an origin body (used by the S3 origin), bounded by
// maxOriginObjectBytes so a tenant object can't force an unbounded allocation.
func drain(rc io.ReadCloser) ([]byte, error) {
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(io.LimitReader(rc, maxOriginObjectBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxOriginObjectBytes {
		return nil, ErrObjectTooLarge
	}
	return b, nil
}
