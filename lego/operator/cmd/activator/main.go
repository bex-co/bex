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

// bex-activator is the shared public interstitial responder. For an
// auto-hibernated free-tier App it wakes the Deployment and returns 503 with a
// retry hint. For an App in maintenance mode it serves the default or bounded
// custom maintenance page without touching the App or workload.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/singleflight"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/bex-co/bex/lego/types/netutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const annotLastActive = "app.bex.co/last-active"

const (
	customPageTimeout = 5 * time.Second
	customPageMaxSize = 1 << 20 // 1 MiB
	maxRedirects      = 5

	// Custom-page snapshot cache (round-11 #4): a maintenance host's custom
	// page is fetched at most once per TTL no matter the public request volume,
	// one fetch is shared by concurrent requests (singleflight), the process
	// holds a bounded number of origin fetches in flight, and a failing origin
	// keeps serving the last good snapshot for a short stale window instead of
	// turning every public request into a fresh 5s timeout. A tenant pointing
	// the page at a slow origin can therefore no longer convert unauthenticated
	// traffic into unbounded activator sockets/goroutines.
	customPageTTL        = 30 * time.Second
	customPageMaxStale   = 10 * time.Minute
	customPageMaxEntries = 256
	customPageMaxFetches = 16

	// Public-server timeouts bound slow-header/slowloris connections so a trickle
	// client cannot hold a goroutine/fd open (codex-security #21).
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 20 // 1 MiB

	// hostCacheRefresh is how often the host→App index is rebuilt. Steady-state
	// requests then resolve from the cache (O(1)) instead of a cluster-wide App
	// List + linear scan per request (codex-security #7).
	hostCacheRefresh = 5 * time.Second
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appv1alpha1.AddToScheme(scheme))
}

func main() {
	ctrl.SetLogger(zap.New())
	log := ctrl.Log.WithName("activator")

	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "activator: k8s client: %v\n", err)
		os.Exit(1)
	}

	addr := ":8888"
	if v := os.Getenv("BEX_ACTIVATOR_ADDR"); v != "" {
		addr = v
	}

	// Build + prime the host→App index once (background-refreshed) so a request
	// flood resolves from memory instead of a cluster-wide App List + linear scan
	// per request (codex-security #7).
	cache := newHostCache(c, log)
	cache.refreshOnce(context.Background())
	go cache.run(context.Background())

	http.Handle("/", newHandler(c, cache, log))

	log.Info("listening", "addr", addr)
	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "activator: %v\n", err)
		os.Exit(1)
	}
}

func newHandler(c client.Client, cache *hostCache, log logr.Logger) http.Handler {
	var wakeGroup singleflight.Group
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}

		host := requestHost(r.Host)
		ctx := r.Context()
		app, ok := cache.lookup(host)
		if !ok {
			log.Info("no routed app found for host", "host", host)
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		// Maintenance routing is a public interstitial only. Do not touch the App,
		// Deployment, or wake annotations: its workload remains exactly as it was.
		if app.Spec.MaintenanceMode != nil && app.Spec.MaintenanceMode.Enabled {
			serveMaintenance(w, r, app, cache.claims)
			return
		}

		// Coalesce concurrent wake requests per App so a retry flood issues one
		// last-active patch + scale-up instead of N.
		key := app.Namespace + "/" + app.Name
		_, _, _ = wakeGroup.Do(key, func() (any, error) {
			wakeApp(logf.IntoContext(ctx, log), c, app, host)
			return nil, nil
		})
		writeWakeResponse(w, r)
	})
}

// wakeApp patches last-active and scales the Deployment to 1 so the operator
// stops auto-hibernating and the workload can come back.
func wakeApp(ctx context.Context, c client.Client, app *appv1alpha1.App, host string) {
	log := logf.FromContext(ctx)
	// Touch last-active so the operator's shouldAutoHibernate returns false on
	// the next reconcile, allowing the Ingress to switch back to the app.
	base := app.DeepCopy()
	if app.Annotations == nil {
		app.Annotations = map[string]string{}
	}
	app.Annotations[annotLastActive] = time.Now().UTC().Format(time.RFC3339)
	if err := c.Patch(ctx, app, client.MergeFrom(base)); err != nil {
		log.Error(err, "patching last-active annotation", "app", app.Name)
	}

	dep := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, dep); err == nil {
		if dep.Spec.Replicas == nil || *dep.Spec.Replicas < 1 {
			one := int32(1)
			depBase := dep.DeepCopy()
			dep.Spec.Replicas = &one
			if err := c.Patch(ctx, dep, client.MergeFrom(depBase)); err != nil {
				log.Error(err, "patching deployment replicas", "app", app.Name)
			}
		}
	}
	log.Info("waking app", "app", app.Name, "host", host)
}

func writeWakeResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "5")
	w.Header().Set("Cache-Control", "no-store")
	if acceptsHTML(r.Header.Values("Accept")) {
		writeHTMLPage(w, r, http.StatusServiceUnavailable, wakePage)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, `{"error":"service hibernated","retryAfter":5}`)
	}
}

// acceptsHTML requires an explicit, non-zero text/html media range. Browsers
// send one; API clients that omit Accept or send only */* keep the historical
// JSON response instead of unexpectedly receiving a document.
func acceptsHTML(values []string) bool {
	for _, value := range values {
		for item := range strings.SplitSeq(value, ",") {
			mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(item))
			if err != nil || !strings.EqualFold(mediaType, "text/html") {
				continue
			}
			if rawQ, ok := params["q"]; ok {
				q, err := strconv.ParseFloat(rawQ, 64)
				if err != nil || q <= 0 {
					continue
				}
			}
			return true
		}
	}
	return false
}

// writeHTMLPage is the shared default-page seam for the maintenance and wake
// responders. Custom maintenance pages keep their bounded origin response.
func writeHTMLPage(w http.ResponseWriter, r *http.Request, status int, page string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, page)
	}
}

func serveMaintenance(w http.ResponseWriter, r *http.Request, app *appv1alpha1.App, platformHost func(string) bool) {
	fetch := maintenancePages.fetcher(platformHost)
	serveMaintenanceWithFetcher(w, r, app, fetch)
}

// maintenancePages is the process-wide custom-page snapshot cache shared by
// every maintenance request (round-11 #4).
var maintenancePages = newCustomPageCache()

// customPageEntry is one cached origin snapshot. ok=false marks a fetch
// failure, which is never cached — the stale window serves the previous good
// entry instead.
type customPageEntry struct {
	body        []byte
	contentType string
	status      int
	fetchedAt   time.Time
}

// customPageCache bounds the custom-page origin fan-out (round-11 #4): per-URI
// singleflight + TTL snapshots, a process-wide cap on concurrent origin
// fetches, a bounded entry map, and stale-on-error serving. All SSRF/redirect
// validation stays in validateCustomPageURL/fetchOrigin — the cache sits above
// it and stores only validated, size-bounded bytes.
type customPageCache struct {
	ttl        time.Duration
	maxStale   time.Duration
	maxEntries int
	fetchSem   chan struct{}
	inflight   singleflight.Group
	transport  *http.Transport // pooled across requests (was: fresh per request)

	mu      sync.Mutex
	entries map[string]customPageEntry
	// now and origin are injectable seams for tests.
	now    func() time.Time
	origin func(context.Context, *appv1alpha1.App, *url.URL, func(string) bool) (*http.Response, error)
}

func newCustomPageCache() *customPageCache {
	c := &customPageCache{
		ttl:        customPageTTL,
		maxStale:   customPageMaxStale,
		maxEntries: customPageMaxEntries,
		fetchSem:   make(chan struct{}, customPageMaxFetches),
		transport: &http.Transport{
			ForceAttemptHTTP2:     true,
			ResponseHeaderTimeout: customPageTimeout,
			TLSHandshakeTimeout:   customPageTimeout,
			DialContext:           netutil.SafeDialContext(customPageTimeout),
		},
		entries: map[string]customPageEntry{},
		now:     time.Now,
	}
	c.origin = c.fetchOrigin
	return c
}

// fetcher returns the fetch func serveMaintenanceWithFetcher consumes. Cache
// hits synthesize an *http.Response over the snapshot, so the size cap and
// status mapping in the responder stay exactly as they were.
func (c *customPageCache) fetcher(
	platformHost func(string) bool,
) func(context.Context, *appv1alpha1.App, string) (*http.Response, error) {
	return func(ctx context.Context, app *appv1alpha1.App, rawURI string) (*http.Response, error) {
		u, err := validateCustomPageURL(rawURI, app, platformHost)
		if err != nil {
			return nil, err
		}
		key := u.String()
		if e, ok := c.snapshot(key); ok {
			return e.response(), nil
		}
		v, err, _ := c.inflight.Do(key, func() (any, error) {
			// Another goroutine may have refreshed the entry while this caller
			// waited on the flight group.
			if e, ok := c.snapshot(key); ok {
				return e, nil
			}
			select {
			case c.fetchSem <- struct{}{}:
			case <-ctx.Done():
				return customPageEntry{}, ctx.Err()
			}
			defer func() { <-c.fetchSem }()
			resp, err := c.origin(ctx, app, u, platformHost)
			if err != nil {
				return customPageEntry{}, err
			}
			defer func() { _ = resp.Body.Close() }()
			body, err := io.ReadAll(io.LimitReader(resp.Body, customPageMaxSize+1))
			if err != nil || len(body) > customPageMaxSize {
				return customPageEntry{}, errors.New("custom page oversized or unreadable")
			}
			e := customPageEntry{
				body:        body,
				contentType: resp.Header.Get("Content-Type"),
				status:      resp.StatusCode,
				fetchedAt:   c.now(),
			}
			c.store(key, e)
			return e, nil
		})
		if err != nil {
			// Origin failed: serve the last good snapshot while it is younger
			// than TTL+maxStale, so a flapping origin does not flap every
			// public request.
			if e, ok := c.staleSnapshot(key); ok {
				return e.response(), nil
			}
			return nil, err
		}
		return v.(customPageEntry).response(), nil
	}
}

// snapshot returns the cached entry when one exists and is younger than ttl.
func (c *customPageCache) snapshot(key string) (customPageEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || c.now().Sub(e.fetchedAt) > c.ttl {
		return customPageEntry{}, false
	}
	return e, true
}

// staleSnapshot returns the cached entry however old, up to ttl+maxStale.
func (c *customPageCache) staleSnapshot(key string) (customPageEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || c.now().Sub(e.fetchedAt) > c.ttl+c.maxStale {
		return customPageEntry{}, false
	}
	return e, true
}

func (c *customPageCache) store(key string, e customPageEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.maxEntries {
		// Bound the map by dropping the oldest entry (distinct URIs track
		// maintenance-enabled Apps; 256 covers them with headroom).
		var oldestKey string
		var oldest time.Time
		first := true
		for k, v := range c.entries {
			if first || v.fetchedAt.Before(oldest) {
				oldestKey, oldest, first = k, v.fetchedAt, false
			}
		}
		if !first {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[key] = e
}

// fetchOrigin performs the actual origin fetch on the shared pooled transport.
func (c *customPageCache) fetchOrigin(
	ctx context.Context,
	app *appv1alpha1.App,
	u *url.URL,
	platformHost func(string) bool,
) (*http.Response, error) {
	hc := &http.Client{
		Transport:     c.transport,
		Timeout:       customPageTimeout,
		CheckRedirect: customPageRedirectPolicy(app, platformHost),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "bex-maintenance-responder/1.0")
	return hc.Do(req)
}

// response materializes the cached snapshot as the *http.Response shape the
// responder consumes.
func (e customPageEntry) response() *http.Response {
	header := http.Header{}
	if e.contentType != "" {
		header.Set("Content-Type", e.contentType)
	}
	return &http.Response{
		StatusCode: e.status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(e.body)),
	}
}

func serveMaintenanceWithFetcher(
	w http.ResponseWriter,
	r *http.Request,
	app *appv1alpha1.App,
	fetch func(context.Context, *appv1alpha1.App, string) (*http.Response, error),
) {
	w.Header().Set("Cache-Control", "no-store")
	if app.Spec.MaintenanceMode.URI == "" {
		writeHTMLPage(w, r, http.StatusServiceUnavailable, maintenancePage)
		return
	}

	resp, err := fetch(r.Context(), app, app.Spec.MaintenanceMode.URI)
	if err != nil {
		http.Error(w, "custom maintenance page unavailable", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, customPageMaxSize+1))
	if err != nil || len(body) > customPageMaxSize {
		http.Error(w, "custom maintenance page unavailable", http.StatusBadGateway)
		return
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	status := resp.StatusCode
	if status >= 200 && status < 400 {
		status = http.StatusServiceUnavailable
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func customPageRedirectPolicy(
	app *appv1alpha1.App, platformHost func(string) bool,
) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return errors.New("too many redirects")
		}
		_, err := validateCustomPageURL(req.URL.String(), app, platformHost)
		return err
	}
}

func validateCustomPageURL(rawURI string, app *appv1alpha1.App, platformHost func(string) bool) (*url.URL, error) {
	u, err := url.Parse(rawURI)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("maintenance uri must be an absolute HTTP(S) URL")
	}
	if u.User != nil {
		return nil, errors.New("maintenance uri must not contain credentials")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	for owned := range appHosts(app) {
		if host == owned {
			return nil, errors.New("maintenance uri must not point to this service")
		}
	}
	// Cross-service recursion guard (defense in depth — the activator does not
	// trust backend validation): every custom-page fetch is a synchronous
	// public request answered by this shared activator, so a URI whose host is
	// routed to ANY App on the platform — not just this one — can close an
	// amplifying fetch cycle between two services pointing at each other.
	if platformHost != nil && platformHost(host) {
		return nil, errors.New("maintenance uri must not point to a platform-routed host")
	}
	return u, nil
}

func appHosts(app *appv1alpha1.App) map[string]struct{} {
	hosts := make(map[string]struct{})
	add := func(value string) {
		if u, err := url.Parse(value); err == nil && u.Hostname() != "" {
			hosts[strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))] = struct{}{}
			return
		}
		value = requestHost(value)
		if value != "" {
			hosts[value] = struct{}{}
		}
	}
	add(app.Spec.Host)
	for _, host := range app.Spec.Hosts {
		add(host)
	}
	add(app.Status.URL)
	for _, value := range app.Status.URLs {
		add(value)
	}
	return hosts
}

func requestHost(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	return strings.ToLower(strings.TrimSuffix(value, "."))
}

// hostCache is a periodically-refreshed host→App index that lets the public
// handler route a request with an O(1) map lookup instead of a cluster-wide App
// List + linear scan per request (codex-security #7). A background refresher
// rebuilds the map every hostCacheRefresh; a failed refresh keeps serving the
// last good map rather than emptying it. New/changed hosts are visible within one
// refresh interval of when the activator started.
type hostCache struct {
	client client.Client
	log    logr.Logger
	mu     sync.RWMutex
	byHost map[string]*appv1alpha1.App
}

func newHostCache(c client.Client, log logr.Logger) *hostCache {
	return &hostCache{client: c, log: log, byHost: map[string]*appv1alpha1.App{}}
}

// refreshOnce rebuilds the host→App map from one cluster-wide List. On error it
// leaves the existing map in place (stale-but-useful) and logs.
func (h *hostCache) refreshOnce(ctx context.Context) {
	var list appv1alpha1.AppList
	if err := h.client.List(ctx, &list); err != nil {
		h.log.Error(err, "refreshing app host cache")
		return
	}
	next := make(map[string]*appv1alpha1.App, len(list.Items))
	for i := range list.Items {
		app := &list.Items[i]
		for host := range appHosts(app) {
			next[host] = app
		}
	}
	h.mu.Lock()
	h.byHost = next
	h.mu.Unlock()
}

// run refreshes the index on a ticker until ctx is cancelled.
func (h *hostCache) run(ctx context.Context) {
	t := time.NewTicker(hostCacheRefresh)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.refreshOnce(ctx)
		}
	}
}

// claims reports whether host (canonicalized) is routed to ANY App in the
// current host map — the platform-wide denylist the maintenance-page fetch
// path validates against, so two services cannot point their custom
// maintenance pages at each other through this shared activator. Unlike
// lookup it returns no App (the fetch path only needs membership), so no
// DeepCopy is taken.
func (h *hostCache) claims(host string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.byHost[requestHost(host)]
	return ok
}

// lookup returns the App that owns host (canonicalized), or ok=false. It never
// hits the API — the public request path is List-free (codex-security #7). It
// returns a private DeepCopy, never the cache-owned pointer: the wake path
// mutates annotations, the same *App is shared across concurrent requests (an App
// with multiple hosts resolves to one object), and refreshOnce swaps the map
// underneath. Handing out the shared pointer let concurrent wakes race on the
// same annotations map — a fatal "concurrent map writes" crash (finding 5).
func (h *hostCache) lookup(host string) (*appv1alpha1.App, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	app, ok := h.byHost[requestHost(host)]
	if !ok {
		return nil, false
	}
	return app.DeepCopy(), true
}

const maintenancePage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Under maintenance</title>
  <style>
    body{font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0;
      background:#111827;color:#f9fafb}
    main{max-width:36rem;padding:2rem;text-align:center}h1{font-size:1.75rem}p{color:#d1d5db;line-height:1.6}
  </style>
</head>
<body><main><h1>This site is currently under maintenance.</h1>
<p>The owner will restore service as soon as possible. Please try again later.</p></main></body>
</html>`

const wakePage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>bex &mdash; Application loading</title>
  <style>
    body{font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0;
      background:#111827;color:#f9fafb}
    main{max-width:36rem;padding:2rem;text-align:center}h1{font-size:1.75rem;margin-bottom:.75rem}
    p{color:#d1d5db;line-height:1.6}.s{display:inline-block;width:24px;height:24px;
      border:3px solid #4b5563;border-top-color:#f9fafb;border-radius:50%;animation:spin 1s linear infinite}
    @keyframes spin{to{transform:rotate(360deg)}}
  </style>
</head>
<body><main><div class="s" aria-hidden="true"></div><h1>Application loading</h1>
<p>Incoming HTTP request detected. This service is waking up and will be ready shortly.</p>
<p><small>Checking again every 5 seconds&hellip;</small></p></main>
<script>
  const probeIntervalMs = 5000;
  const fallbackReloadMs = 45000;
  async function testIfServerIsUp() {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 4000);
    try {
      const response = await fetch(window.location.href, {
        method: "HEAD", cache: "no-store", signal: controller.signal
      });
      if (response.status !== 503) window.location.replace(window.location.href);
    } catch (_) {
      // A cold workload can remain unreachable between probes.
    } finally {
      clearTimeout(timeout);
    }
  }
  setInterval(testIfServerIsUp, probeIntervalMs);
  setTimeout(() => window.location.reload(), fallbackReloadMs);
</script></body>
</html>`
