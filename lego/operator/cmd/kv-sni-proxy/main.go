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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	defaultAddr        = ":6379"
	defaultMetricsAddr = ":9093"
	dialTimeout        = 10 * time.Second
	handshakeTimeout   = 10 * time.Second

	// Admission + copy-loop bounds (finding 6). Non-zero, generous defaults; a
	// value of 0 disables that dimension.
	defaultMaxConns          = 1024      // BEX_KV_PROXY_MAX_CONNS
	defaultMaxConnsPerSource = 128       // BEX_KV_PROXY_MAX_CONNS_PER_SOURCE
	defaultIdleTimeout       = time.Hour // BEX_KV_PROXY_IDLE_TIMEOUT
	defaultMaxLifetime       = 24 * time.Hour
)

var scheme = runtime.NewScheme()

func init() { utilruntime.Must(appv1alpha1.AddToScheme(scheme)) }

type kvRoute struct {
	ResourceID string
	Backend    string
	Allow      []netip.Prefix
	EnvAllow   []netip.Prefix
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

func (r *kvRouter) set(kv *appv1alpha1.KeyValue) error {
	expectedHost := kv.Name + "." + r.domain
	if !kv.Spec.Public || r.domain == "" || !strings.EqualFold(kv.Status.ExternalHost, expectedHost) {
		r.delete(kv.Name)
		return nil
	}
	route := kvRoute{
		ResourceID: kv.Name,
		Backend:    fmt.Sprintf("%s.%s.svc.cluster.local:6380", kv.Name, kv.Namespace),
	}
	for _, entry := range kv.Spec.IPAllowList {
		prefix, err := netip.ParsePrefix(entry.CIDR)
		if err != nil {
			r.mu.Lock()
			delete(r.table, kv.Name)
			r.invalid[kv.Name] = true
			r.mu.Unlock()
			return fmt.Errorf("invalid allowlist CIDR %q: %w", entry.CIDR, err)
		}
		route.Allow = append(route.Allow, prefix.Masked())
	}
	for _, cidr := range kv.Spec.EnvironmentIPAllowList {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			r.mu.Lock()
			delete(r.table, kv.Name)
			r.invalid[kv.Name] = true
			r.mu.Unlock()
			return fmt.Errorf("invalid environment allowlist CIDR %q: %w", cidr, err)
		}
		route.EnvAllow = append(route.EnvAllow, prefix.Masked())
	}
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
	if !allowedByLayer(source, route.Allow) || !allowedByLayer(source, route.EnvAllow) {
		return kvRoute{}, false
	}
	return route, true
}

func allowedByLayer(source netip.Addr, prefixes []netip.Prefix) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if prefix.Contains(source.Unmap()) {
			return true
		}
	}
	return false
}

type kvWatcher struct {
	client.Client
	router *kvRouter
	meter  *sniproxy.ByteMeter
}

func (w *kvWatcher) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var kv appv1alpha1.KeyValue
	if err := w.Get(ctx, req.NamespacedName, &kv); err != nil {
		if apierrors.IsNotFound(err) {
			w.router.delete(req.Name)
			w.meter.SetHealthy(w.router.healthy())
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if !kv.DeletionTimestamp.IsZero() {
		w.router.delete(kv.Name)
		return reconcile.Result{}, nil
	}
	if err := w.router.set(&kv); err != nil {
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
	idleTimeout := sniproxy.EnvDuration(logger, "BEX_KV_PROXY_IDLE_TIMEOUT", defaultIdleTimeout)
	maxLifetime := sniproxy.EnvDuration(logger, "BEX_KV_PROXY_MAX_LIFETIME", defaultMaxLifetime)
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
	if err := ctrl.NewControllerManagedBy(mgr).For(&appv1alpha1.KeyValue{}).
		Complete(&kvWatcher{Client: mgr.GetClient(), router: router, meter: meter}); err != nil {
		logger.Error(err, "set up KeyValue watcher")
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	defer cancel()
	managerErrors := make(chan error, 1)
	go func() { managerErrors <- mgr.Start(ctx) }()
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		logger.Error(errors.New("cache sync failed"), "sync KeyValue cache")
		os.Exit(1)
	}
	meter.SetHealthy(true)

	metricsAddr := envOr("BEX_KV_PROXY_METRICS_ADDR", defaultMetricsAddr)
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

	listener, err := net.Listen("tcp", envOr("BEX_KV_PROXY_ADDR", defaultAddr))
	if err != nil {
		logger.Error(err, "listen")
		os.Exit(1)
	}
	defer func() { _ = listener.Close() }()
	go func() {
		select {
		case <-ctx.Done():
		case err := <-managerErrors:
			if err != nil {
				logger.Error(err, "manager exited")
			}
			cancel() // stop Serve cleanly when the manager exits without a signal
		}
		_ = listener.Close()
	}()
	// Serve admits each connection through the global cap BEFORE dispatching a
	// handler goroutine, shedding overload at accept time (finding 6).
	sniproxy.Serve(ctx, listener, limiter, meter, func(conn net.Conn) {
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
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
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
	_ = conn.SetReadDeadline(time.Time{})
	route, ok := router.resolve(sni, source)
	if !ok {
		logger.Info("no allowed route", "sni", sni)
		return
	}
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
	// The idle + max-lifetime bounds stop a routed connection from idling or
	// living forever after Valkey auth (finding 6).
	sniproxy.CopyBidirectional(conn, backend, meter, route.ResourceID, "key_value", idle, maxLifetime)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
