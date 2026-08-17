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

// bex-pg-sni-proxy is the Postgres-aware SNI front-door for external
// managed-database connections. Traefik's TCP/SNI routing reads hostname from
// the TLS ClientHello, but the libpq default (preamble-mode) sends a cleartext
// SSLRequest BEFORE the TLS ClientHello — so Traefik can't route it.
//
// The proxy sits on :5432, handles the SSLRequest negotiation, peeks at the
// TLS ClientHello to extract the SNI hostname, and dials the correct CNPG
// backend directly. Direct-TLS clients (PG 17+ sslnegotiation=direct) are also
// handled transparently. See docs/ADR009-postgresql-management.md.
package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
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
	sslRequestMagic = int32(80877103) // 0x04D2162F — PostgreSQL SSLRequest magic
	sslRequestLen   = int32(8)
	sslByte         = byte('S') // server replies 'S' to accept SSL

	defaultAddr        = ":5432"
	defaultMetricsAddr = ":9092"
	dialTimeout        = 10 * time.Second
	// handshakeTimeout bounds the PROXY/TLS preamble so a slow- or no-byte
	// connection cannot pin a goroutine + file descriptor indefinitely (the
	// sibling kv-sni-proxy sets the same control). Cleared once SNI is resolved.
	handshakeTimeout = 10 * time.Second

	// Admission + copy-loop bounds (finding 6). Defaults are non-zero and
	// generous so real connection pools are unaffected; 0 disables a dimension.
	defaultMaxConns          = 1024      // BEX_PROXY_MAX_CONNS
	defaultMaxConnsPerSource = 128       // BEX_PROXY_MAX_CONNS_PER_SOURCE
	defaultMaxConnsWorkspace = 256       // BEX_PROXY_MAX_CONNS_PER_WORKSPACE
	defaultMaxConnsResource  = 64        // BEX_PROXY_MAX_CONNS_PER_RESOURCE
	defaultIdleTimeout       = time.Hour // BEX_PROXY_IDLE_TIMEOUT
	defaultMaxLifetime       = 24 * time.Hour
	labelWorkspace           = "app.bex.co/workspace"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(appv1alpha1.AddToScheme(scheme))
}

// dbRouter maps `<dbname>` → `<namespace>` for all Database CRs in the cluster.
// SNI hostname `<dbname>.<domain>` is resolved at connection time. The router is
// kept up-to-date by a controller-runtime reconciler watching Database events.
type dbRouter struct {
	mu      sync.RWMutex
	domain  string                    // BEX_DB_DOMAIN, e.g. "db.bex.co"
	table   map[string]dbRoutingEntry // dbname → exact public routing intent
	invalid map[string]bool           // malformed CRs keep global source health false
}

type dbRoutingEntry struct {
	workspace string
	namespace string
	pooler    bool
	replicas  map[string]bool
	allow     []netip.Prefix
	envAllow  []netip.Prefix
}

type dbRoute struct {
	Database  string
	Workspace string
	Backend   string
}

func newRouter(domain string) *dbRouter {
	return &dbRouter{
		domain:  strings.ToLower(strings.TrimSuffix(domain, ".")),
		table:   make(map[string]dbRoutingEntry),
		invalid: make(map[string]bool),
	}
}

func (r *dbRouter) set(db *appv1alpha1.Database) error {
	if !db.Spec.Public || r.domain == "" {
		r.delete(db.Name)
		return nil
	}
	workspace := db.Labels[labelWorkspace]
	if workspace == "" {
		workspace = db.Namespace
	}
	entry := dbRoutingEntry{workspace: workspace, namespace: db.Namespace, pooler: db.Spec.Pooler, replicas: map[string]bool{}}
	for _, replica := range db.Spec.ReadReplicas {
		entry.replicas[replica.Name] = true
	}
	allow, envAllow, err := sniproxy.ParseAllowList(db.Spec.IPAllowList, db.Spec.EnvironmentIPAllowList)
	if err != nil {
		r.mu.Lock()
		delete(r.table, db.Name)
		r.invalid[db.Name] = true
		r.mu.Unlock()
		return err
	}
	entry.allow, entry.envAllow = allow, envAllow
	r.mu.Lock()
	r.table[db.Name] = entry
	delete(r.invalid, db.Name)
	r.mu.Unlock()
	return nil
}

func (r *dbRouter) delete(name string) {
	r.mu.Lock()
	delete(r.table, name)
	delete(r.invalid, name)
	r.mu.Unlock()
}

func (r *dbRouter) healthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.invalid) == 0
}

// resolve maps an SNI hostname to a backend TCP address.
// Supported patterns (all on port 5432):
//   - <dbname>.<domain>           → <dbname>-rw.<ns>.svc.cluster.local:5432
//   - <dbname>-pool.<domain>      → <dbname>-pooler.<ns>.svc.cluster.local:5432
//   - <dbname>-ro-<rep>.<domain>  → <dbname>-ro.<ns>.svc.cluster.local:5432
func (r *dbRouter) resolve(sni string, source netip.Addr) (dbRoute, bool) {
	if r.domain == "" {
		return dbRoute{}, false
	}
	base, ok := strings.CutSuffix(strings.ToLower(strings.TrimSuffix(sni, ".")), "."+r.domain)
	if !ok {
		return dbRoute{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, entry := range r.table {
		var backend string
		switch {
		case base == name:
			backend = fmt.Sprintf("%s-rw.%s.svc.cluster.local:5432", name, entry.namespace)
		case base == name+"-pool" && entry.pooler:
			backend = fmt.Sprintf("%s-pooler.%s.svc.cluster.local:5432", name, entry.namespace)
		case strings.HasPrefix(base, name+"-ro-") && entry.replicas[strings.TrimPrefix(base, name+"-ro-")]:
			backend = fmt.Sprintf("%s-ro.%s.svc.cluster.local:5432", name, entry.namespace)
		default:
			continue
		}
		// round-5 finding 15: scan the (tenant-controlled, now count-capped)
		// allowlist ONLY for the entry whose exact SNI name matched — a name
		// compare is O(1), an allowlist scan is not, so the prior "check every
		// database's allowlist on every handshake" turned cheap TLS churn into
		// shared-proxy CPU + router-lock pressure. Fall-through is preserved: a
		// name match the source can't use lets a differently-keyed entry still win.
		if !sniproxy.AllowedBy(source, entry.allow) || !sniproxy.AllowedBy(source, entry.envAllow) {
			continue
		}
		return dbRoute{Database: name, Workspace: entry.workspace, Backend: backend}, true
	}
	return dbRoute{}, false
}

// dbWatcher is a controller-runtime reconciler that keeps dbRouter in sync with
// Database CRs. It does no patching — read only. Runs in every namespace.
type dbWatcher struct {
	client.Client
	router *dbRouter
	meter  *sniproxy.ByteMeter
}

func (w *dbWatcher) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var db appv1alpha1.Database
	if err := w.Get(ctx, req.NamespacedName, &db); err != nil {
		if apierrors.IsNotFound(err) {
			w.router.delete(req.Name)
			w.meter.SetHealthy(w.router.healthy())
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if !db.DeletionTimestamp.IsZero() {
		w.router.delete(db.Name)
	} else if err := w.router.set(&db); err != nil {
		w.meter.SetHealthy(w.router.healthy())
		return reconcile.Result{}, err
	}
	w.meter.SetHealthy(w.router.healthy())
	return reconcile.Result{}, nil
}

func main() {
	ctrl.SetLogger(zap.New())
	log := ctrl.Log.WithName("pg-sni-proxy")

	addr := sniproxy.EnvOr("BEX_PROXY_ADDR", defaultAddr)
	domain := os.Getenv("BEX_DB_DOMAIN")
	if domain == "" {
		log.Info("BEX_DB_DOMAIN is unset — proxy will refuse all connections (no routing table)")
	}
	trustedProxyCIDRs, err := sniproxy.ParseTrustedProxyCIDRs(os.Getenv("BEX_PROXY_PROTOCOL_TRUSTED_CIDRS"))
	if err != nil {
		log.Error(err, "invalid BEX_PROXY_PROTOCOL_TRUSTED_CIDRS")
		os.Exit(1)
	}
	limiter := sniproxy.NewLimiter(
		sniproxy.EnvInt(log, "BEX_PROXY_MAX_CONNS", defaultMaxConns),
		sniproxy.EnvInt(log, "BEX_PROXY_MAX_CONNS_PER_SOURCE", defaultMaxConnsPerSource),
	)
	limiter.SetRouteLimits(
		sniproxy.EnvInt(log, "BEX_PROXY_MAX_CONNS_PER_WORKSPACE", defaultMaxConnsWorkspace),
		sniproxy.EnvInt(log, "BEX_PROXY_MAX_CONNS_PER_RESOURCE", defaultMaxConnsResource),
	)
	idleTimeout := sniproxy.EnvDuration(log, "BEX_PROXY_IDLE_TIMEOUT", defaultIdleTimeout)
	maxLifetime := sniproxy.EnvDuration(log, "BEX_PROXY_MAX_LIFETIME", defaultMaxLifetime)

	router := newRouter(domain)
	registry := prometheus.NewRegistry()
	meter := sniproxy.NewByteMeter(registry, "pg_proxy", "postgres")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		// Dedicated plaintext Prometheus listener below; disable controller-runtime's.
		Metrics:        metricsserver.Options{BindAddress: "0"},
		LeaderElection: false,
	})
	if err != nil {
		log.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.Database{}).
		Complete(&dbWatcher{Client: mgr.GetClient(), router: router, meter: meter}); err != nil {
		log.Error(err, "unable to set up Database watcher")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	defer cancel()

	// Start the manager (and its cache) in the background.
	mgrErrCh := make(chan error, 1)
	go func() { mgrErrCh <- mgr.Start(ctx) }()

	// Wait for the cache to sync before accepting connections, so the first
	// connection doesn't race with the initial list.
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		log.Error(fmt.Errorf("cache sync timed out"), "failed to sync Database cache")
		os.Exit(1)
	}
	meter.SetHealthy(true)
	metricsAddr := sniproxy.EnvOr("BEX_PROXY_METRICS_ADDR", defaultMetricsAddr)
	metricsServer := &http.Server{
		Addr: metricsAddr, Handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, "metrics listener failed", "addr", metricsAddr)
			cancel()
		}
	}()
	defer func() { _ = metricsServer.Close() }()
	log.Info("cache synced, starting TCP listener", "addr", addr)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error(err, "failed to listen", "addr", addr)
		os.Exit(1)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		select {
		case <-ctx.Done():
		case err := <-mgrErrCh:
			if err != nil {
				log.Error(err, "manager exited")
			}
			cancel() // stop Serve cleanly when the manager exits without a signal
		}
		_ = ln.Close()
	}()

	// Serve admits each connection through the global cap BEFORE dispatching a
	// handler goroutine, shedding overload at accept time (finding 6).
	sniproxy.Serve(ctx, ln, limiter, meter, func(conn net.Conn) {
		handleConn(conn, router, meter, trustedProxyCIDRs, limiter, idleTimeout, maxLifetime, log)
	}, func(err error) {
		log.Error(err, "accept error")
	})
}

// handleConn handles one incoming client connection:
//  1. Detects PostgreSQL SSLRequest preamble vs direct TLS.
//  2. Buffers the TLS ClientHello and extracts the SNI hostname.
//  3. Dials the matching CNPG backend.
//  4. Re-negotiates SSL with the backend (for preamble-mode clients).
//  5. Splices the connection bidirectionally.
func handleConn(
	conn net.Conn,
	router *dbRouter,
	meter *sniproxy.ByteMeter,
	trustedProxyCIDRs []netip.Prefix,
	limiter *sniproxy.Limiter,
	idle, maxLifetime time.Duration,
	log interface {
		Info(msg string, keysAndValues ...any)
		Error(err error, msg string, keysAndValues ...any)
	},
) {
	defer func() { _ = conn.Close() }()
	// Bound the handshake reads (PROXY preamble + SSLRequest peek + TLS
	// ClientHello) so a slowloris connection can't hold this goroutine and fd
	// open (codex-security #2). Cleared once SNI is resolved, before relay.
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	reader := bufio.NewReader(conn)
	source, err := sniproxy.ReadProxySource(reader, conn.RemoteAddr(), trustedProxyCIDRs)
	if err != nil {
		log.Info("invalid PROXY protocol header", "err", err)
		return
	}

	// Per-source admission (finding 6): one client IP can't monopolize the proxy.
	// Applied here — after the real source is resolved from any PROXY header —
	// and BEFORE dialing the backend, so a shed connection opens no backend dial.
	releaseSource, ok := limiter.AcquireSource(source)
	if !ok {
		meter.Reject("source")
		log.Info("connection rejected: per-source limit reached", "source", source.String())
		return
	}
	defer releaseSource()

	// Peek at the first 8 bytes to distinguish SSLRequest from direct TLS.
	var peek [8]byte
	if _, err := io.ReadFull(reader, peek[:]); err != nil {
		return
	}

	msgLen := int32(binary.BigEndian.Uint32(peek[0:4]))
	msgCode := int32(binary.BigEndian.Uint32(peek[4:8]))
	isSSLRequest := msgLen == sslRequestLen && msgCode == sslRequestMagic
	isDirectTLS := peek[0] == sniproxy.TLSHandshakeType

	if !isSSLRequest && !isDirectTLS {
		log.Info("unknown protocol header, closing", "byte0", fmt.Sprintf("%02x", peek[0]))
		return
	}

	if isSSLRequest {
		// Tell the client we support SSL, then read its TLS ClientHello.
		if _, err := conn.Write([]byte{sslByte}); err != nil {
			return
		}
	}

	// initial holds bytes already peeked that belong to the TLS record.
	var initial []byte
	if isDirectTLS {
		initial = peek[:] // first 8 bytes are the start of the TLS record
	}

	clientHello, err := sniproxy.ReadTLSRecord(reader, initial)
	if err != nil {
		log.Info("failed to read TLS ClientHello", "err", err)
		return
	}

	sni, err := sniproxy.ExtractSNI(clientHello)
	if err != nil || sni == "" {
		log.Info("no SNI in ClientHello", "err", err)
		return
	}
	// Handshake consumed — clear the deadline so backend reads/relaying are
	// governed only by the upstream proxy's own timeouts, not this bound.
	_ = conn.SetReadDeadline(time.Time{})

	route, ok := router.resolve(sni, source)
	if !ok {
		log.Info("no route for SNI", "sni", sni)
		return
	}
	releaseRoute, rejectedScope, ok := limiter.AcquireRoute(route.Workspace, route.Workspace+"/"+route.Database)
	if !ok {
		meter.Reject(rejectedScope)
		log.Info("connection rejected: routed limit reached", "scope", rejectedScope, "database", route.Database)
		return
	}
	defer releaseRoute()

	back, err := net.DialTimeout("tcp", route.Backend, dialTimeout)
	if err != nil {
		log.Error(err, "dial backend failed", "backend", route.Backend)
		return
	}
	defer func() { _ = back.Close() }()

	if isSSLRequest {
		// Replicate the SSLRequest/S handshake with the backend so it upgrades
		// to TLS before we forward the ClientHello.
		if _, err := back.Write(peek[:]); err != nil {
			return
		}
		var resp [1]byte
		if _, err := io.ReadFull(back, resp[:]); err != nil {
			return
		}
		if resp[0] != sslByte {
			log.Info("backend declined SSL", "response", resp[0])
			return
		}
	}

	// Forward the buffered TLS ClientHello to the backend; the TLS handshake
	// continues transparently (the proxy is not a TLS terminator).
	if _, err := back.Write(clientHello); err != nil {
		return
	}

	// Splice the two sides bidirectionally. The TLS session is end-to-end between
	// the client and CNPG — the proxy never sees the plaintext. The idle +
	// max-lifetime bounds stop a routed connection from idling or living forever
	// after DB auth (finding 6).
	sniproxy.CopyBidirectional(conn, back, meter, route.Database, "postgres", idle, maxLifetime)
}
