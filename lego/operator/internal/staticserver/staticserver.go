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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"
	"golang.org/x/sync/singleflight"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/types/netutil"
	"github.com/bex-co/bex/lego/types/routepattern"
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
	// etag is the strong validator over Body, set by Handler.get when the
	// object is fetched (w4/341); a cached object is hashed once for all its
	// revalidations.
	etag string
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
	// Incarnation is the App's UID. Object keys restart at rev-1 when a site is
	// deleted and recreated under its name, so the cache keys by it too
	// (cacheKey, w8/146).
	Incarnation string
	// Unavailable marks a published site that is suspended in the last
	// snapshot. Its host stays known so a request reaching the static-server
	// across the suspend/resume Ingress handoff reads "try again", never the
	// unknown-host 404 that says the site is gone (w4/307). It serves no bytes.
	Unavailable bool
	// NoSPAFallback is Render's miss handling (w8/183): a path that matches no
	// file and no rule answers 404 with the site's own /404.html when it has
	// one. Unset, a site keeps the implicit single-page-app fallback it was
	// created with (ADR029 § Default SPA fallback); the resolver sets it for
	// sites created on or after SPAFallbackCutoff.
	NoSPAFallback bool
}

// SPAFallbackCutoff is when the static server stopped serving a site's root
// index.html with 200 for every extension-less miss (w8/183). Render answers a
// miss with 404 and the site's 404.html, and client-side routing is an explicit
// `/* → /index.html` rewrite. Sites created before it keep the implicit
// fallback, so a deploy never turns a running single-page app's deep links into
// 404s; they opt in by adding the rewrite.
var SPAFallbackCutoff = time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC)

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

// cacheKey is the cache's key for a site-relative path: the object key under
// the App's incarnation, so a recreated site never reads its predecessor's
// cached bytes, misses or encodings for a reused revision key.
func (s Site) cacheKey(reqPath string) string {
	return s.Incarnation + "\x00" + s.keyFor(reqPath)
}

// legacyServePrefix is the prefix string used for legacy-shape detection /
// evidence logging. Empty Prefix synthesizes "<app>/<rev>/"; does not force a
// trailing slash onto an explicit Prefix (status may omit it).
func legacyServePrefix(s Site) string {
	if s.Prefix != "" {
		return s.Prefix
	}
	if s.AppID == "" || s.Revision == "" {
		return ""
	}
	return s.AppID + "/" + s.Revision + "/"
}

// isLegacyStaticPrefix reports whether the Site still serves from the pre-m75
// "<app>/<rev>/" object-key shape (empty status.staticPrefix dual-read). A
// workspace-scoped prefix is "<workspace>/<app>/<rev>/" and does not match.
func isLegacyStaticPrefix(s Site) bool {
	if s.AppID == "" {
		return false
	}
	prefix := legacyServePrefix(s)
	if prefix == "" {
		return false
	}
	return strings.HasPrefix(prefix, s.AppID+"/")
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
	// clients bounds the origin fetches one client holds at once, so it cannot
	// take the gate's slots from other tenants' sites.
	clients *keyedLimiter
	// admission lets a miss over the client, site or replica cap wait briefly
	// for a slot before it is shed.
	admission *admission
	// liveBodies bounds the total bytes of response bodies held live while being
	// written to clients, extending memory accounting from fetch until the
	// response write completes (the fetch gate alone releases at read time).
	liveBodies *semaphore.Weighted
	// encodes bounds concurrent Brotli encodes (CPU), globally and per site;
	// their working memory is leased from liveBodies (encoding.go).
	encodes *fetchGate
	// external fetches external rewrite destinations (proxy.go, w8/184).
	external *externalProxy
}

// New builds a Handler over a Resolver and Origin. cacheBytes caps the in-memory
// object cache (0 disables caching).
func New(resolver Resolver, origin Origin, cacheBytes int64) *Handler {
	return &Handler{
		resolver:   resolver,
		origin:     origin,
		cache:      newCache(cacheBytes),
		gate:       newFetchGate(defaultMaxConcurrentFetches, defaultMaxInflightBytes, defaultMaxSiteFetches),
		clients:    newKeyedLimiter(defaultMaxClientFetches),
		admission:  newAdmission(),
		liveBodies: semaphore.NewWeighted(defaultMaxLiveBodyBytes),
		encodes:    newFetchGate(defaultMaxConcurrentEncodes, 0, defaultMaxSiteEncodes),
		external:   newExternalProxy(netutil.UnsafeOriginIP),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	site, ok := h.resolver.Resolve(hostOnly(r.Host))
	if !ok {
		http.Error(w, "no static site for host", http.StatusNotFound)
		return
	}
	r = r.WithContext(withClient(r.Context(), clientKey(r)))

	// requestPath is the visitor's normalized path and stays immutable: custom
	// headers match against it on every response (Render's request-path
	// contract, w4/m94), while the (possibly rewritten) served path drives
	// origin keys and content type.
	requestPath := normalizePath(r.URL.Path)
	if site.Unavailable {
		serveSiteUnavailable(w, site, requestPath)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.serveOtherMethod(w, r, site, requestPath)
		return
	}
	// Git metadata is never part of a site (w4/565). Sites published before
	// the publish step dropped .git still hold it.
	if hasGitSegment(requestPath) {
		h.serveNotFound(w, r, site, requestPath)
		return
	}

	// w2/m93: evidence line when this host still dual-reads the legacy
	// "<app>/<rev>/" prefix. Path/prefix stay line fields (never Loki labels).
	// Unlabeled sites are legitimate forever; readiness filters by tombstone.
	if isLegacyStaticPrefix(site) {
		log.FromContext(r.Context()).Info("static_legacy_origin_get",
			"app", site.AppID,
			"prefix", legacyServePrefix(site),
			"path", requestPath,
		)
	}

	// An existing published object, or the page an extensionless path names,
	// wins over both redirect and rewrite rules, matching Render's documented
	// order. Only a genuine miss advances to the ordered rules; permission
	// denials, timeouts, capacity and size failures keep their own classes
	// below rather than degrading into a rule.
	//
	// The store cannot hold a key past maxObjectKeyBytes, so a path whose
	// derived key (after index.html defaulting) is longer can never match an
	// object — nor plausibly be an SPA route. fetchPage treats it as a miss
	// without an origin round trip instead of misreporting the store's
	// key-length client error as an origin failure (w6/047).
	overlong := overlongKey(site, requestPath)
	obj, servedPath, err := h.fetchPage(r.Context(), site, requestPath)
	if err == nil {
		h.serveObject(w, r, site, obj, servedPath, requestPath)
		return
	}
	if !errors.Is(err, ErrNotFound) {
		serveOriginError(w, site, requestPath, err)
		return
	}

	// Miss: edge rules run in order, first match wins.
	act, target, routeErr := matchRoutes(site.Routes, requestPath)
	if act == actRedirect {
		serveRedirect(w, r, site, requestPath, target, routeErr)
		return
	}
	if routeErr != nil {
		serveSiteError(w, site, requestPath, http.StatusBadRequest, "invalid rewrite target")
		return
	}

	if act == actNone && overlong {
		// No rule claimed the over-long path; it can never be an object and is
		// not plausibly an SPA route either.
		h.serveNotFound(w, r, site, requestPath)
		return
	}

	fallbackPath := requestPath
	if isExternalRewrite(act, target) {
		// An external rewrite serves the destination URL's content here; Render
		// does not rewrite the root path.
		if requestPath == "/" {
			h.serveNotFound(w, r, site, requestPath)
			return
		}
		h.serveExternalRewrite(w, r, site, requestPath, target)
		return
	}
	if act == actRewrite {
		fallbackPath = normalizePath(target.Path)
		if overlongKey(site, fallbackPath) {
			h.serveNotFound(w, r, site, requestPath)
			return
		}
		// A rewrite target resolves as a request for it would, page included.
		obj, servedPath, err = h.fetchPage(r.Context(), site, fallbackPath)
		if err == nil {
			h.serveObject(w, r, site, obj, servedPath, requestPath)
			return
		}
		if !errors.Is(err, ErrNotFound) {
			serveOriginError(w, site, requestPath, err)
			return
		}
	}

	// Sites created before SPAFallbackCutoff keep the implicit SPA fallback: if
	// the miss (after any rewrite) is a bare path with no extension and an
	// index.html exists at the root, serve it so client-side routing works even
	// without an explicit route. A missing asset (has an extension) stays a 404.
	if !site.NoSPAFallback {
		if fallback, fok := h.spaFallback(r.Context(), site, fallbackPath); fok {
			h.serveObject(w, r, site, fallback, "/index.html", requestPath)
			return
		}
	}
	h.serveNotFound(w, r, site, requestPath)
}

// serveOtherMethod answers a method other than GET and HEAD on a live site. A
// site's objects are read with those two; any other method reaches content
// only through an external rewrite, Render's way to reach an API from a
// static site (w8/184). A matched redirect rule answers with GET's 301
// (w8/324), file or not: no lookup runs, as for an external rewrite, and
// Render's answer for a file under a redirect rule is uncaptured. Anything
// else gets Render's edge answer: a bare 200 with no body and none of the
// site's headers, on any path (w8/319, captured on four Render static sites
// 2026-11-13).
func (h *Handler) serveOtherMethod(w http.ResponseWriter, r *http.Request, site Site, requestPath string) {
	act, target, err := matchRoutes(site.Routes, requestPath)
	switch {
	case act == actRedirect && !hasGitSegment(requestPath):
		serveRedirect(w, r, site, requestPath, target, err)
		return
	case err == nil && isExternalRewrite(act, target) && requestPath != "/":
		// The root path is never rewritten, as on the GET path.
		h.serveExternalRewrite(w, r, site, requestPath, target)
		return
	}
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusOK)
}

// serveRedirect answers a matched redirect rule as Render does (w8/324,
// captured on wc3v.onrender.com 2026-11-14): a 301 to the expanded destination
// carrying the request's query, for every method, with only Content-Type and
// Location — none of the site's rules, HSTS or the cache policy. A target that
// failed to expand (expandErr) is a 400 instead. The request's query is
// checked apart from the destination: past the "?" a reverse solidus cannot
// move the Location's host or path. A raw "#" in it is escaped, so the
// visitor's text never becomes the Location's fragment.
func serveRedirect(w http.ResponseWriter, r *http.Request, site Site, requestPath string, target *url.URL, expandErr error) {
	query := r.URL.RawQuery
	var location string
	if expandErr == nil {
		location = target.String()
	}
	if expandErr != nil || !safeRedirectLocation(location, target) || hasControlByte(query) {
		serveSiteError(w, site, requestPath, http.StatusBadRequest, "invalid redirect target")
		return
	}
	if query != "" {
		location = withVisitorQuery(target, strings.ReplaceAll(query, "#", "%23")).String()
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		// http.Redirect types only a GET or HEAD answer; Render types them all.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	http.Redirect(w, r, location, http.StatusMovedPermanently)
}

// isExternalRewrite reports whether a matched rule rewrites to a full URL,
// which the static server proxies (w8/184).
func isExternalRewrite(act routeAction, target *url.URL) bool {
	return act == actRewrite && target.Host != ""
}

// serveNotFound answers a miss as Render does (w8/322, captured 2026-11-14 on
// four Render sites with /* rules): 404 with the site's root /404.html, sent
// with the headers of any published file (the default cache policy, HSTS,
// nosniff and the matching rules), else a bare 404 with nosniff and none of
// the site's rules. The page carries no validators or ranges: a conditional
// request for it would read as a cache hit on a 404.
func (h *Handler) serveNotFound(w http.ResponseWriter, r *http.Request, site Site, requestPath string) {
	if !site.NoSPAFallback {
		serveBareNotFound(w)
		return
	}
	obj, err := h.get(r.Context(), site, "/404.html")
	if err != nil {
		serveBareNotFound(w)
		return
	}
	if r.Method != http.MethodHead {
		lease := int64(len(obj.Body))
		if !h.liveBodies.TryAcquire(lease) {
			serveSiteBusy(w, site, requestPath)
			return
		}
		defer h.liveBodies.Release(lease)
	}
	hdr := w.Header()
	setFileHeaders(hdr, site, requestPath, contentType("/404.html", obj.ContentType), "")
	hdr.Set("Content-Length", strconv.Itoa(len(obj.Body)))
	w.WriteHeader(http.StatusNotFound)
	if r.Method != http.MethodHead {
		_, _ = w.Write(obj.Body)
	}
}

// serveBareNotFound is Render's answer to a miss on a site with no 404.html
// (w8/322): text/plain "Not Found" with nosniff and none of the site's rules.
func serveBareNotFound(w http.ResponseWriter) {
	http.Error(w, "Not Found", http.StatusNotFound)
}

// setFileHeaders gives a published file's response its headers: its type,
// the default cache policy, Render's static-site security defaults (w8/194)
// and an ETag when it has one, then the site's rules matching the request
// path, which win over each (last write for a key), so a saved ETag rule
// replaces the generated one (w4/272).
func setFileHeaders(hdr http.Header, site Site, requestPath, servedType, etag string) {
	hdr.Set("Content-Type", servedType)
	hdr.Set("Cache-Control", defaultCacheControl)
	hdr.Set("Strict-Transport-Security", defaultStrictTransportSecurity)
	hdr.Set("X-Content-Type-Options", "nosniff")
	if etag != "" {
		hdr.Set("Etag", etag)
	}
	applyHeaders(hdr, site.Headers, requestPath)
}

// serveOriginError maps a non-miss fetch failure onto its response class:
// oversize 413, overload 503 + Retry-After, anything else 502. Custom headers
// still apply (w4/m101) — a CORS/security rule must cover origin failures too.
func serveOriginError(w http.ResponseWriter, site Site, requestPath string, err error) {
	switch {
	case errors.Is(err, ErrObjectTooLarge):
		serveSiteError(w, site, requestPath, http.StatusRequestEntityTooLarge, "object too large")
	case errors.Is(err, ErrOverloaded):
		serveSiteBusy(w, site, requestPath)
	default:
		serveSiteError(w, site, requestPath, http.StatusBadGateway, "origin error")
	}
}

// serveObject writes a successful object response. servedPath (the resolved
// object) drives content type; requestPath (the visitor's path) drives custom
// header matching.
func (h *Handler) serveObject(w http.ResponseWriter, r *http.Request, site Site, obj Object, servedPath, requestPath string) {
	// Hold a live-body lease for the whole write: the body stays allocated
	// until the response reaches the client, so slow clients must count
	// against the in-flight memory budget even after the fetch gate released
	// its reservation. HEAD responses carry no body and need no lease. On
	// exhaustion shed with 503 (before any success header is written) rather
	// than buffering past the budget. The lease is taken before ServeContent
	// evaluates preconditions, so a bodyless 304/412 briefly holds it too and
	// is shed with the rest under exhaustion, as the 200 it replaces would
	// be; matching entity tags here would duplicate ServeContent's parser.
	if r.Method != http.MethodHead {
		lease := int64(len(obj.Body))
		if !h.liveBodies.TryAcquire(lease) {
			serveSiteBusy(w, site, requestPath)
			return
		}
		defer h.liveBodies.Release(lease)
	}

	hdr := w.Header()
	servedType := contentType(servedPath, obj.ContentType)
	setFileHeaders(hdr, site, requestPath, servedType, obj.etag)
	// A Content-Type rule changes the representation, so the generated tag
	// follows it: a 304 carries no Content-Type, and a browser holding the
	// old tag would keep the old type (w5/182).
	if final := hdr.Get("Content-Type"); obj.etag != "" && final != servedType && hdr.Get("Etag") == obj.etag {
		hdr.Set("Etag", retypedETag(obj.etag, final))
	}

	// Automatic Brotli and gzip (w8/m79, w8/m132). The representation is chosen before
	// ServeContent, so its preconditions and byte ranges apply to the bytes
	// actually sent. An eligible object varies by Accept-Encoding whichever
	// coding this request gets; anything short of a ready, useful variant
	// within budget falls back to identity.
	body := obj.Body
	if encodingCandidate(hdr, len(obj.Body)) {
		varyAcceptEncoding(hdr)
		if coding := negotiateCoding(r.Header); coding != "" {
			if encoded, ok := h.encodedVariant(coding, site.AppID, site.cacheKey(servedPath), obj.Body); ok {
				// The encoded body leases its own bytes on top of the identity
				// lease (the identity body stays referenced); HEAD writes none.
				// Without room, serve identity rather than shed.
				lease := int64(len(encoded))
				if r.Method == http.MethodHead {
					lease = 0
				}
				if h.liveBodies.TryAcquire(lease) {
					defer h.liveBodies.Release(lease)
					body = encoded
					hdr.Set("Content-Encoding", coding)
					weakenETag(hdr)
					w = &encodedResponseWriter{ResponseWriter: w, size: len(encoded)}
				}
			}
		}
	}

	// ServeContent evaluates the request against the ETag (304 for a matching
	// If-None-Match, 412 for a failed If-Match — RFC 9110 §13) and serves byte
	// ranges; HEAD gets headers only. The zero modtime keeps it from inventing
	// a Last-Modified validator (w4/272).
	http.ServeContent(w, r, servedPath, time.Time{}, bytes.NewReader(body))
}

// safeRedirectTarget validates the final, expanded Location at the output sink.
// Route validation cannot prove this invariant because :splat is request data;
// browsers normalize reverse solidus like slash in navigation URLs.
func safeRedirectTarget(target string) bool {
	return strings.HasPrefix(target, "/") &&
		!strings.HasPrefix(target, "//") &&
		!strings.HasPrefix(target, `/\`) &&
		!strings.Contains(target, "\\") &&
		!hasControlByte(target)
}

func hasControlByte(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0
}

func hasHTTPScheme(dest string) bool {
	lower := strings.ToLower(dest)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// safeAbsoluteHost accepts only a literal DNS-style host (optionally with a
// numeric port) and no userinfo: a capture expands in the path after the
// authority, so it can never change which host the Location names.
func safeAbsoluteHost(u *url.URL) bool {
	if u.User != nil || u.Opaque != "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	for _, c := range host {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.' {
			continue
		}
		return false
	}
	for _, c := range u.Port() {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// safeRedirectLocation validates the final Location at the output sink: a
// local path, or (for a redirect only) a plain http(s) URL with a literal host.
func safeRedirectLocation(location string, target *url.URL) bool {
	if safeRedirectTarget(location) {
		return true
	}
	return hasHTTPScheme(location) && safeAbsoluteHost(target) && !strings.Contains(location, "\\") && !hasControlByte(location) &&
		(target.Path == "" || safeRedirectTarget(target.Path))
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

// fetchPage resolves reqPath as Render serves a visitor's path: the object at
// it (index.html beneath "/" or a slash path), else the page an extensionless
// path names, in place with 200 and no redirect (w8/317, w8/318). `/about`
// and `/about/` serve a published about.html, and `/blog` serves
// blog/index.html. Only a miss is ErrNotFound; any other origin error is
// returned, so the caller answers its class rather than a wrong page.
func (h *Handler) fetchPage(ctx context.Context, site Site, reqPath string) (Object, string, error) {
	if !overlongKey(site, reqPath) {
		obj, servedPath, err := h.fetch(ctx, site, reqPath)
		if !errors.Is(err, ErrNotFound) {
			return obj, servedPath, err
		}
	}
	return h.resolvePage(ctx, site, reqPath)
}

// resolvePage is fetchPage's miss step. Only .html is tried, so a path with an
// extension (`/favicon`'s favicon.png, `/about.html/`) stays a miss. A
// slashless path tries <path>.html before <path>/index.html; a slash path's
// directory index was fetch's own lookup. Render's order when both exist was
// not captured.
func (h *Handler) resolvePage(ctx context.Context, site Site, reqPath string) (Object, string, error) {
	page := strings.TrimSuffix(reqPath, "/")
	if page == "" || path.Ext(page) != "" {
		return Object{}, "", ErrNotFound
	}
	candidates := []string{page + ".html"}
	if !strings.HasSuffix(reqPath, "/") {
		candidates = append(candidates, lookupPath(page+"/"))
	}
	for _, candidate := range candidates {
		if overlongKey(site, candidate) {
			continue
		}
		obj, err := h.get(ctx, site, candidate)
		if !errors.Is(err, ErrNotFound) {
			return obj, candidate, err
		}
	}
	return Object{}, "", ErrNotFound
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
	if hasGitSegment(reqPath) {
		// No rewrite destination reaches git metadata either.
		return Object{}, ErrNotFound
	}
	key, ck := site.keyFor(reqPath), site.cacheKey(reqPath)
	client := clientFrom(ctx)
	if obj, ok := h.cache.get(ck); ok {
		return obj, negativeErr(obj)
	}
	v, err, _ := h.group.Do(ck, func() (any, error) {
		// A prior leader for this key may have just populated the cache.
		if obj, ok := h.cache.get(ck); ok {
			return obj, nil
		}
		release, ok := h.admit(ctx, site.AppID, client)
		if !ok {
			return Object{}, ErrOverloaded
		}
		defer release()
		obj, gerr := h.origin.Get(ctx, key)
		if errors.Is(gerr, ErrNotFound) {
			// A confirmed miss is immutable within the revision too; cache it
			// so it stops costing origin round trips and fetch-gate slots.
			// Only ErrNotFound: a denial, timeout, or overload is transient
			// and must be re-asked.
			miss := Object{negative: true}
			h.cache.put(site.AppID, ck, miss)
			return miss, nil
		}
		if gerr != nil {
			return Object{}, gerr
		}
		obj.etag = contentETag(obj.Body, contentType(reqPath, obj.ContentType))
		h.cache.put(site.AppID, ck, obj)
		return obj, nil
	})
	if err != nil {
		return Object{}, err
	}
	obj := v.(Object)
	return obj, negativeErr(obj)
}

// admit reserves what one origin fetch needs: a slot of its client's cap (when
// the client is known) and a fetch-gate slot. A request that finds either at
// capacity waits briefly for one to free (admission.go) before it is shed.
func (h *Handler) admit(ctx context.Context, appID, client string) (release func(), ok bool) {
	if release, ok := h.tryAdmit(appID, client); ok {
		return release, true
	}
	return h.admission.await(ctx, client, func() (func(), bool) { return h.tryAdmit(appID, client) })
}

// tryAdmit is admit without waiting: the client cap first, then the gate. When
// the gate refuses, the client slot taken a moment ago is given back and the
// waiters are woken, since one of them may have been refused by that very slot.
func (h *Handler) tryAdmit(appID, client string) (release func(), ok bool) {
	if client != "" && !h.clients.acquire(client) {
		return nil, false
	}
	if !h.gate.acquire(appID) {
		if client != "" {
			h.clients.release(client)
			h.admission.notify()
		}
		return nil, false
	}
	return func() {
		h.gate.release(appID)
		if client != "" {
			h.clients.release(client)
		}
		h.admission.notify()
	}, true
}

// contentETag is the strong entity tag for an object's bytes and served type:
// changed bytes after a republish get a new tag, so revalidation returns them
// (w4/m160), and so does a corrected type, which a 304 would not carry (w4/581).
func contentETag(body []byte, servedType string) string {
	h := sha256.New()
	h.Write([]byte(servedType))
	h.Write([]byte{0})
	h.Write(body)
	return `"` + base64.RawURLEncoding.EncodeToString(h.Sum(nil)[:16]) + `"`
}

// retypedETag is the generated tag of an object served as servedType by a
// header rule, without hashing its bytes again.
func retypedETag(tag, servedType string) string {
	h := sha256.New()
	h.Write([]byte(tag))
	h.Write([]byte{0})
	h.Write([]byte(servedType))
	return `"` + base64.RawURLEncoding.EncodeToString(h.Sum(nil)[:16]) + `"`
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
// trailing "/*" wildcard and whole-segment ":name" placeholders; captures
// substitute ":splat"/trailing "/*" and ":name" tokens in the destination.
func matchRoutes(routes []appv1alpha1.StaticRoute, reqPath string) (routeAction, *url.URL, error) {
	for _, rt := range routes {
		splat, captures, ok := matchRoutePattern(rt.Source, reqPath)
		if !ok {
			continue
		}
		var action routeAction
		switch rt.Type {
		case "redirect":
			action = actRedirect
		case "rewrite":
			action = actRewrite
		default:
			continue
		}
		dest, err := expandDestCaptures(rt.Destination, splat, captures, true)
		return action, dest, err
	}
	return actNone, nil, nil
}

// matchPattern reports whether reqPath matches pattern and returns the splat (the
// text captured by a trailing "/*"). "/*" matches everything (splat = the path
// minus its leading slash); "/foo/*" matches "/foo/" and "/foo/...", never the
// bare "/foo", as on Render (w8/325); any other pattern is an exact match
// (splat empty).
func matchPattern(pattern, reqPath string) (string, bool) {
	if pattern == "/*" {
		return strings.TrimPrefix(reqPath, "/"), true
	}
	if prefix, ok := strings.CutSuffix(pattern, "/*"); ok { // "/foo/*" -> "/foo"
		if rest, ok := strings.CutPrefix(reqPath, prefix+"/"); ok {
			return rest, true
		}
		return "", false
	}
	return "", pattern == reqPath
}

// matchRoutePattern is matchPattern plus whole-segment ":name" captures. A
// placeholder matches exactly one non-empty segment and may precede a
// trailing "/*", which, as in matchPattern, needs the slash after the last
// segment: "/:lang/*" matches "/en/" but not "/en". Patterns without
// placeholders keep matchPattern semantics.
func matchRoutePattern(pattern, reqPath string) (string, map[string]string, bool) {
	base, hasSplat := strings.CutSuffix(pattern, "/*")
	hasPlaceholder := false
	for seg := range strings.SplitSeq(base, "/") {
		if _, ok := routepattern.PlaceholderName(seg); ok {
			hasPlaceholder = true
			break
		}
	}
	if !hasPlaceholder {
		splat, ok := matchPattern(pattern, reqPath)
		return splat, nil, ok
	}
	patSegs := strings.Split(base, "/")
	reqSegs := strings.Split(reqPath, "/")
	if hasSplat && len(reqSegs) <= len(patSegs) || !hasSplat && len(reqSegs) != len(patSegs) {
		return "", nil, false
	}
	captures := map[string]string{}
	for i, seg := range patSegs {
		if name, ok := routepattern.PlaceholderName(seg); ok {
			if reqSegs[i] == "" {
				return "", nil, false
			}
			captures[name] = reqSegs[i]
			continue
		}
		if seg != reqSegs[i] {
			return "", nil, false
		}
	}
	if !hasSplat {
		return "", captures, true
	}
	return strings.Join(reqSegs[len(patSegs):], "/"), captures, true
}

const (
	maxRouteDestinationBytes  = 2048     // Matches the configured destination limit.
	maxExpandedRoutePathBytes = 8 * 1024 // Bounds capture amplification before allocating.
)

// expandDest separates URL syntax before substituting decoded path data. Never
// parse the expanded path again: a captured ?/#/% is a filename character, not
// a new query, fragment, or escape. Configured query/fragment values stay literal.
// substituteCaptures expands ":splat", declared ":name" tokens and a
// trailing "/*" alias in a destination path, bounding the total first so a
// repeated token cannot amplify a capture past maxExpandedRoutePathBytes.
// Undeclared ":name" tokens stay literal (writers refuse them; a direct CR
// write must not panic).
func substituteCaptures(destPath, splat string, captures map[string]string) (string, error) {
	type token struct {
		start, end int
		value      string
	}
	var tokens []token
	expandedLen := len(destPath)
	for i := 0; i < len(destPath); {
		if destPath[i] != ':' {
			i++
			continue
		}
		j := i + 1
		for j < len(destPath) && (destPath[j] == '_' || destPath[j] >= 'a' && destPath[j] <= 'z' || destPath[j] >= 'A' && destPath[j] <= 'Z' || destPath[j] >= '0' && destPath[j] <= '9') {
			j++
		}
		name := destPath[i+1 : j]
		if _, ok := routepattern.PlaceholderName(":" + name); !ok {
			i++
			continue
		}
		if name == "splat" {
			tokens = append(tokens, token{i, j, splat})
			expandedLen += len(splat) - (j - i)
			i = j
			continue
		}
		if value, ok := captures[name]; ok {
			tokens = append(tokens, token{i, j, value})
			expandedLen += len(value) - (j - i)
			i = j
			continue
		}
		i = j
	}
	hasSplatToken := strings.Contains(destPath, ":splat")
	trailing := !hasSplatToken && strings.HasSuffix(destPath, "/*")
	if trailing {
		expandedLen += len(splat) - 1
	}
	if expandedLen > maxExpandedRoutePathBytes {
		return "", errors.New("expanded destination exceeds size limit")
	}
	if len(tokens) == 0 && !trailing {
		return destPath, nil
	}
	var b strings.Builder
	b.Grow(expandedLen)
	pos := 0
	for _, tok := range tokens {
		b.WriteString(destPath[pos:tok.start])
		b.WriteString(tok.value)
		pos = tok.end
	}
	if trailing {
		b.WriteString(destPath[pos : len(destPath)-1])
		b.WriteString(splat)
	} else {
		b.WriteString(destPath[pos:])
	}
	return b.String(), nil
}

func expandDest(dest, splat string, allowAbsolute bool) (*url.URL, error) {
	return expandDestCaptures(dest, splat, nil, allowAbsolute)
}

func expandDestCaptures(dest, splat string, captures map[string]string, allowAbsolute bool) (*url.URL, error) {
	if len(dest) > maxRouteDestinationBytes {
		return nil, errors.New("destination exceeds size limit")
	}
	// A redirect may leave the site for a fixed http(s) host, as Render allows
	// (w8/162), and so may a rewrite, which fetches it through the SSRF-guarded
	// proxy (w8/184).
	absolute := allowAbsolute && hasHTTPScheme(dest)
	if !absolute && !safeRedirectTarget(dest) {
		return nil, errors.New("destination must be local")
	}
	target, err := url.Parse(dest)
	if err != nil {
		return nil, err
	}
	if _, err := url.QueryUnescape(target.RawQuery); err != nil {
		return nil, err
	}
	if absolute {
		if !safeAbsoluteHost(target) || strings.ContainsAny(dest, "\\") || hasControlByte(dest) || (target.Path != "" && !safeRedirectTarget(target.Path)) {
			return nil, errors.New("destination must be a plain http(s) URL with a literal host")
		}
	} else if target.Host != "" || target.Scheme != "" || !safeRedirectTarget(target.Path) {
		return nil, errors.New("destination must be local")
	}
	expanded, err := substituteCaptures(target.Path, splat, captures)
	if err != nil {
		return nil, err
	}
	target.Path = expanded
	if (!absolute || target.Path != "") && !safeRedirectTarget(target.Path) {
		return nil, errors.New("expanded destination must be local")
	}
	target.RawPath = ""
	return target, nil
}

// matchHeaderPattern keeps file selectors separate from route splat captures.
// A globstar requires at least one directory: Render distinguishes /*.css from
// /**/*.css, and /**/* also matches a trailing directory slash. Unsupported
// metacharacters retain their literal meaning; matching is bounded by path size.
func matchHeaderPattern(pattern, reqPath string) bool {
	if suffix, ok := strings.CutPrefix(pattern, "/**/*"); ok && !strings.ContainsAny(suffix, "/*") {
		slash := strings.LastIndexByte(reqPath, '/')
		return slash > 0 && strings.HasSuffix(reqPath[slash+1:], suffix)
	}
	if prefix, suffix, ok := strings.Cut(pattern, "*"); ok && suffix != "" &&
		strings.HasSuffix(prefix, "/") && !strings.ContainsAny(suffix, "/*") {
		file, underPrefix := strings.CutPrefix(reqPath, prefix)
		return underPrefix && !strings.Contains(file, "/") && strings.HasSuffix(file, suffix)
	}
	_, ok := matchPattern(pattern, reqPath)
	return ok
}

// applyHeaders adds every custom header whose path pattern matches reqPath. Set
// (not Add) so a repeated header name resolves to the last matching rule. A
// reserved framing/hop-by-hop name is skipped: the API refuses it, but a rule
// saved before that check or a hand-applied CR must not abort every response
// (w4/204).
func applyHeaders(h http.Header, headers []appv1alpha1.StaticHeader, reqPath string) {
	for _, rule := range headers {
		if appv1alpha1.ReservedStaticHeader(rule.Name) {
			continue
		}
		if matchHeaderPattern(rule.Path, reqPath) {
			h.Set(rule.Name, rule.Value)
		}
	}
}

// errorBodyHeaderNames are hop/body metadata a custom rule must never rewrite
// on an error path — http.Error owns Content-Type/Length, and clobbering them
// would make the error body lie about what it is (w4/m101).
var errorBodyHeaderNames = map[string]struct{}{
	"Content-Type":      {},
	"Content-Length":    {},
	"Transfer-Encoding": {},
}

// applyErrorHeaders adds matching custom headers for a resolved-site error
// response, skipping body-identity names. Callers that need a platform
// Retry-After must set it after this helper (see serveSiteBusy).
func applyErrorHeaders(h http.Header, headers []appv1alpha1.StaticHeader, reqPath string) {
	for _, rule := range headers {
		if _, blocked := errorBodyHeaderNames[http.CanonicalHeaderKey(rule.Name)]; blocked {
			continue
		}
		if matchHeaderPattern(rule.Path, reqPath) {
			h.Set(rule.Name, rule.Value)
		}
	}
}

// serveSiteError writes an error for a resolved site. Custom headers apply
// before http.Error so every error class for a known site carries the
// visitor's matching rules (w4/m101), except a bare 404, which Render sends
// without them (serveBareNotFound, w8/322). A request that never
// resolved a site (an unknown host) stays on bare http.Error: there are no
// rules, and applying host-derived headers there would be an existence oracle.
func serveSiteError(w http.ResponseWriter, site Site, requestPath string, code int, msg string) {
	applyErrorHeaders(w.Header(), site.Headers, requestPath)
	http.Error(w, msg, code)
}

// serveSiteBusy is the shared 503 shed path (origin overload + live-body
// exhaustion). Platform Retry-After is set after custom headers so a customer
// rule cannot disable shedding backoff.
func serveSiteBusy(w http.ResponseWriter, site Site, requestPath string) {
	applyErrorHeaders(w.Header(), site.Headers, requestPath)
	w.Header().Set("Retry-After", "1")
	http.Error(w, "server busy", http.StatusServiceUnavailable)
}

// serveSiteUnavailable answers a known site the last snapshot saw suspended:
// the Ingress has switched back to the static-server on resume before the next
// Refresh observes it (or still points here on suspend before the activator
// takes over). Platform no-store + Retry-After are set after custom headers so
// a customer rule cannot make the transition cacheable or disable backoff.
func serveSiteUnavailable(w http.ResponseWriter, site Site, requestPath string) {
	applyErrorHeaders(w.Header(), site.Headers, requestPath)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "5")
	http.Error(w, "static site temporarily unavailable", http.StatusServiceUnavailable)
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

// hasGitSegment reports whether a site-relative path names git metadata: a
// ".git" directory or submodule gitfile at any depth.
func hasGitSegment(p string) bool {
	for seg := range strings.SplitSeq(p, "/") {
		if seg == ".git" {
			return true
		}
	}
	return false
}

// hostOnly strips a port from a Host header value.
func hostOnly(host string) string {
	h, _, _ := strings.Cut(host, ":")
	return h
}

// contentType serves Render's type for the extensions renderExtensionTypes
// names, else prefers the origin's content type, else infers from the
// extension, else falls back to octet-stream. An unknown origin type in either
// spelling counts as none: the publish upload sets no type for an extension
// the AWS CLI doesn't know, and the store then reports binary/octet-stream
// (w4/581).
func contentType(servedPath, originType string) string {
	ext := strings.ToLower(path.Ext(servedPath))
	if ct := renderExtensionTypes[ext]; ct != "" {
		return ct
	}
	if originType != "" && originType != "application/octet-stream" && originType != "binary/octet-stream" {
		return originType
	}
	if ct := staticExtensionTypes[ext]; ct != "" {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// staticExtensionTypes are the IANA types of web extensions Go's built-in
// table lacks; the distroless image has no /etc/mime.types to add them.
var staticExtensionTypes = map[string]string{
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".map":   "application/json",
	".yaml":  "application/yaml",
	".yml":   "application/yaml",
	".ogg":   "audio/ogg",
}

// renderExtensionTypes are the types Render's static sites serve for these
// extensions whatever the upload guessed (w8/326, captured 2026-11-14 on five
// Render sites). The publish uploader's Python 3.12 table stores text/javascript,
// text/xml and application/manifest+json instead. Render sends no charset on
// any of the three and leaves a manifest uncompressed. .mjs and .cjs were not
// captured, so they follow .js.
var renderExtensionTypes = map[string]string{
	".js":          "application/javascript",
	".mjs":         "application/javascript",
	".cjs":         "application/javascript",
	".xml":         "application/xml",
	".webmanifest": "binary/octet-stream",
}

// defaultCacheControl is the browser/CDN policy for every successful response
// and the 404.html page (w8/322) without a matching custom Cache-Control rule
// (w4/m160). Revision prefixes
// keep origin objects immutable, but browsers key responses by public URL,
// which survives a republish — so any filename must revalidate, hashed-looking
// or not. Owners with content-addressed assets opt into long freshness through
// a header rule. Responses already cached under the pre-m160 one-year
// immutable policy are not recalled: they stay until eviction, a forced reload,
// or a new asset URL.
const defaultCacheControl = "public, max-age=0, must-revalidate"

// defaultStrictTransportSecurity is what Render's static sites send on every
// successful response and the 404.html page (w8/194, w8/322): HTTPS only, ten
// years, subdomains, preloadable.
const defaultStrictTransportSecurity = "max-age=315360000; includeSubdomains; preload"

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
