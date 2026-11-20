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

// bex-kv-sni-proxy is the TLS/SNI pass-through public front door for managed
// Valkey. Public rediss:// clients keep end-to-end TLS to the existing Valkey
// server while this process owns source allowlists and wire-byte accounting.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
	"github.com/bex-co/bex/lego/types/env"
	"github.com/bex-co/bex/lego/types/tiers"
)

const (
	defaultAddr        = ":6379"
	defaultMetricsAddr = ":9093"
	dialTimeout        = 10 * time.Second
	handshakeTimeout   = 10 * time.Second

	// Admission bounds (finding 6); 0 disables a dimension. The pg-sni-proxy
	// mirror: each store's route admits twice its plan's limit, and these
	// protect the process only (w6/m457, docs/ADR021 § 9).
	defaultMaxConns          = 4096 // BEX_KV_PROXY_MAX_CONNS
	defaultMaxConnsPerSource = 0    // BEX_KV_PROXY_MAX_CONNS_PER_SOURCE
	// Connections still in their handshake (PROXY header to SNI) are bounded
	// per network, apart from established ones, so a botnet inside a few
	// networks cannot hold the global cap for the whole handshake deadline. One
	// network is an IPv4 /24 or an IPv6 /64, and 32 of them fill the process.
	defaultMaxHandshakesPerNetwork = 128 // BEX_KV_PROXY_MAX_HANDSHAKES_PER_NETWORK

	// drainSecondsEnv bounds the drain a release's SIGTERM starts, in whole
	// seconds; unset or 0 closes every stream at once (w6/m515).
	drainSecondsEnv = "BEX_KV_PROXY_DRAIN_SECONDS"
	// idleTimeoutEnv and maxLifetimeEnv are the copy loop's idle and lifetime
	// bounds (Go durations), both unset: a routed stream stays open however
	// long it is silent and however old, as on Valkey (w6/149). Each is an
	// incident lever, and the drain bound never exceeds a set idle timeout.
	idleTimeoutEnv = "BEX_KV_PROXY_IDLE_TIMEOUT"
	maxLifetimeEnv = "BEX_KV_PROXY_MAX_LIFETIME"
)

var scheme = runtime.NewScheme()

// The scheme holds sniproxy's routing views, never lego/types/v1alpha1, so a
// CRD edit leaves this binary byte-identical and rolls no pod (w6/150).
func init() { utilruntime.Must(sniproxy.AddToScheme(scheme)) }

type kvRoute struct {
	ResourceID string
	Backend    string
	Allow      sniproxy.AllowLayers
	// Route and Capacity are the store's admission bucket and its size,
	// twice its plan's connection limit (sniproxy.RouteCapacity).
	Route    string
	Capacity int
}

type kvRouter struct {
	mu      sync.RWMutex
	domain  string
	table   map[string]kvRoute
	invalid map[string]bool
}

func newRouter(domain string) *kvRouter {
	return &kvRouter{domain: domain, table: map[string]kvRoute{}, invalid: map[string]bool{}}
}

func (r *kvRouter) set(kv *sniproxy.KeyValue) error {
	expectedHost := kv.Name + "." + r.domain
	if !kv.Spec.Public || r.domain == "" || !strings.EqualFold(kv.Status.ExternalHost, expectedHost) {
		r.delete(kv.Name)
		return nil
	}
	// Backend is the KeyValue's ClusterIP Service: its A record is the stable
	// VIP, so CoreDNS caching cannot hand a dial a restarted pod's old IP or a
	// cached NXDOMAIN (w1/m166 — it did while the Service was headless).
	route := kvRoute{
		ResourceID: kv.Name,
		Backend:    fmt.Sprintf("%s.%s.svc.cluster.local:6380", kv.Name, kv.Namespace),
		Route:      kv.Namespace + "/" + kv.Name,
		Capacity:   sniproxy.RouteCapacity(tiers.Valkey.Resolve(kv.Spec.Plan).ConnectionLimit),
	}
	allow, err := sniproxy.ParseAllowList(
		kv.Spec.IPAllowList, kv.Spec.EnvironmentIPAllowList, kv.Spec.WorkspaceIPAllowList)
	if err != nil {
		r.mu.Lock()
		delete(r.table, kv.Name)
		r.invalid[kv.Name] = true
		r.mu.Unlock()
		return err
	}
	route.Allow = allow
	r.mu.Lock()
	r.table[kv.Name] = route
	delete(r.invalid, kv.Name)
	r.mu.Unlock()
	return nil
}

func (r *kvRouter) delete(name string) {
	r.mu.Lock()
	delete(r.table, name)
	delete(r.invalid, name)
	r.mu.Unlock()
}

func (r *kvRouter) healthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.invalid) == 0
}

func (r *kvRouter) resolve(sni string, source netip.Addr) (kvRoute, bool) {
	base, ok := strings.CutSuffix(strings.ToLower(strings.TrimSuffix(sni, ".")), "."+r.domain)
	if !ok || base == "" || strings.Contains(base, ".") {
		return kvRoute{}, false
	}
	r.mu.RLock()
	route, ok := r.table[base]
	r.mu.RUnlock()
	if !ok {
		return kvRoute{}, false
	}
	if !route.Allow.Admits(source) {
		return kvRoute{}, false
	}
	return route, true
}

type kvWatcher struct {
	client.Client
	router *kvRouter
	meter  *sniproxy.ByteMeter
}

func (w *kvWatcher) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var kv sniproxy.KeyValue
	if err := w.Get(ctx, req.NamespacedName, &kv); err != nil {
		if apierrors.IsNotFound(err) {
			w.router.delete(req.Name)
			w.meter.SetHealthy(w.router.healthy())
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	// Deletion clears the invalid mark too, so health must be re-published on
	// this path as well (w1/m76 t006 — the pg twin's ordering): an early return
	// here left bex_kv_proxy_healthy stuck at 0 after the last malformed
	// KeyValue was deleted, until an unrelated reconcile happened to fire.
	if !kv.DeletionTimestamp.IsZero() {
		w.router.delete(kv.Name)
	} else if err := w.router.set(&kv); err != nil {
		w.meter.SetHealthy(w.router.healthy())
		return reconcile.Result{}, err
	}
	w.meter.SetHealthy(w.router.healthy())
	return reconcile.Result{}, nil
}

func main() {
	ctrl.SetLogger(zap.New())
	logger := ctrl.Log.WithName("kv-sni-proxy")
	domain := strings.TrimSuffix(os.Getenv("BEX_KV_DOMAIN"), ".")
	if domain == "" {
		logger.Info("BEX_KV_DOMAIN is unset; proxy has no public routes")
	}
	trustedProxyCIDRs, err := sniproxy.ParseTrustedProxyCIDRs(os.Getenv("BEX_PROXY_PROTOCOL_TRUSTED_CIDRS"))
	if err != nil {
		logger.Error(err, "invalid BEX_PROXY_PROTOCOL_TRUSTED_CIDRS")
		os.Exit(1)
	}
	limiter := sniproxy.NewLimiter(
		sniproxy.EnvInt(logger, "BEX_KV_PROXY_MAX_CONNS", defaultMaxConns),
		sniproxy.EnvInt(logger, "BEX_KV_PROXY_MAX_CONNS_PER_SOURCE", defaultMaxConnsPerSource),
	)
	limiter.SetHandshakeLimit(
		sniproxy.EnvInt(logger, "BEX_KV_PROXY_MAX_HANDSHAKES_PER_NETWORK", defaultMaxHandshakesPerNetwork),
	)
	idleTimeout, maxLifetime := sniproxy.StreamBounds(logger, idleTimeoutEnv, maxLifetimeEnv)
	drainTimeout, err := sniproxy.DrainSeconds(drainSecondsEnv)
	if err != nil {
		logger.Error(err, "invalid drain bound")
		os.Exit(1)
	}
	registry := prometheus.NewRegistry()
	meter := sniproxy.NewByteMeter(registry, "kv_proxy", "key_value")
	router := newRouter(domain)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme, Metrics: metricsserver.Options{BindAddress: "0"}, LeaderElection: false,
	})
	if err != nil {
		logger.Error(err, "create manager")
		os.Exit(1)
	}
	if err := ctrl.NewControllerManagedBy(mgr).For(&sniproxy.KeyValue{}).
		Complete(&kvWatcher{Client: mgr.GetClient(), router: router, meter: meter}); err != nil {
		logger.Error(err, "set up KeyValue watcher")
		os.Exit(1)
	}
	// The first SIGTERM or SIGINT ends stopping: the manager stops, which
	// freezes the routing table, and the listener drains (w6/m515).
	// controller-runtime exits at once on a second signal. ctx is the hard
	// stop, for a manager or metrics listener that fails on its own.
	stopping := ctrl.SetupSignalHandler()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	managerErrors := make(chan error, 1)
	go func() { managerErrors <- mgr.Start(stopping) }()
	if !mgr.GetCache().WaitForCacheSync(stopping) {
		logger.Error(errors.New("cache sync failed"), "sync KeyValue cache")
		os.Exit(1)
	}
	meter.SetHealthy(true)

	metricsAddr := env.Or("BEX_KV_PROXY_METRICS_ADDR", defaultMetricsAddr)
	metricsServer := &http.Server{
		Addr: metricsAddr, Handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(err, "metrics listener failed")
			cancel()
		}
	}()
	defer func() { _ = metricsServer.Close() }()

	listener, err := net.Listen("tcp", env.Or("BEX_KV_PROXY_ADDR", defaultAddr))
	if err != nil {
		logger.Error(err, "listen")
		os.Exit(1)
	}
	go sniproxy.HardStopOnManagerExit(stopping, managerErrors, cancel, func(err error) {
		logger.Error(err, "manager exited")
	})
	// Serve admits each connection through the global cap BEFORE dispatching a
	// handler goroutine, shedding overload at accept time (finding 6), and
	// returns once a drain's last stream has ended.
	drain := sniproxy.Drain{Start: stopping.Done(), Timeout: drainTimeout}
	sniproxy.Serve(ctx, listener, limiter, meter, drain, func(conn net.Conn) {
		handleConn(conn, router, meter, trustedProxyCIDRs, limiter, idleTimeout, maxLifetime, logger)
	}, func(err error) {
		logger.Error(err, "accept")
	})
}

func handleConn(
	conn net.Conn,
	router *kvRouter,
	meter *sniproxy.ByteMeter,
	trustedProxyCIDRs []netip.Prefix,
	limiter *sniproxy.Limiter,
	idle, maxLifetime time.Duration,
	logger interface {
		Info(msg string, keysAndValues ...any)
		Error(err error, msg string, keysAndValues ...any)
	},
) {
	defer func() { _ = conn.Close() }()
	// The first bytes get the short PreHeaderTimeout; once they arrive the
	// handshake has handshakeTimeout from accept.
	start := time.Now()
	_ = conn.SetReadDeadline(start.Add(sniproxy.PreHeaderTimeout()))
	reader := bufio.NewReader(conn)
	source, err := sniproxy.ReadProxySource(reader, conn.RemoteAddr(), trustedProxyCIDRs)
	if err != nil {
		logger.Info("invalid PROXY protocol header", "err", err)
		return
	}

	// Per-source admission (finding 6): applied after the real source is resolved
	// and BEFORE dialing the backend, so a shed connection opens no backend dial.
	releaseSource, ok := limiter.AcquireSource(source)
	if !ok {
		meter.Reject("source")
		logger.Info("connection rejected: per-source limit reached", "source", source.String())
		return
	}
	defer releaseSource()

	// Handshake admission: a connection that has not said what it wants counts
	// against its network's share of unfinished handshakes.
	releaseHandshake, ok := limiter.AcquireHandshake(source)
	if !ok {
		meter.Reject(sniproxy.ScopeHandshake)
		logger.Info("connection rejected: handshake limit reached", "source", source.String())
		return
	}
	defer releaseHandshake()

	if _, err := reader.Peek(1); err != nil {
		return
	}
	_ = conn.SetReadDeadline(start.Add(handshakeTimeout))
	record, err := sniproxy.ReadTLSRecord(reader, nil)
	if err != nil {
		logger.Info("TLS ClientHello read failed", "err", err)
		return
	}
	sni, err := sniproxy.ExtractSNI(record)
	if err != nil || sni == "" {
		logger.Info("TLS ClientHello has no usable SNI", "err", err)
		return
	}
	releaseHandshake()
	_ = conn.SetReadDeadline(time.Time{})
	route, ok := router.resolve(sni, source)
	if !ok {
		logger.Info("no allowed route", "sni", sni)
		return
	}
	// The store's route admits twice its plan's limit, so the client past the
	// limit reaches kv-internal-proxy and reads Valkey's refusal (w6/m457).
	releaseRoute, ok := limiter.AcquireRoute(route.Route, route.Capacity)
	if !ok {
		meter.Reject(sniproxy.ScopeResource)
		logger.Info("connection rejected: route at capacity", "resource", route.ResourceID, "capacity", route.Capacity)
		return
	}
	defer releaseRoute()
	backend, err := net.DialTimeout("tcp", route.Backend, dialTimeout)
	if err != nil {
		logger.Error(err, "dial backend", "resource", route.ResourceID)
		return
	}
	defer func() { _ = backend.Close() }()
	if _, err := backend.Write(record); err != nil {
		logger.Error(err, "forward TLS ClientHello", "resource", route.ResourceID)
		return
	}
	// No idle or lifetime bound by default (w6/149): the stream ends when the
	// client, the edge load balancer or Valkey ends it.
	sniproxy.CopyBidirectional(conn, backend, meter, route.ResourceID, "key_value", idle, maxLifetime)
}
