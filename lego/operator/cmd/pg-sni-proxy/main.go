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
	namespace string
	pooler    bool
	replicas  map[string]bool
	allow     []netip.Prefix
	envAllow  []netip.Prefix
}

type dbRoute struct {
	Database string
	Backend  string
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
	entry := dbRoutingEntry{namespace: db.Namespace, pooler: db.Spec.Pooler, replicas: map[string]bool{}}
	for _, replica := range db.Spec.ReadReplicas {
		entry.replicas[replica.Name] = true
	}
	for _, item := range db.Spec.IPAllowList {
		prefix, err := netip.ParsePrefix(item.CIDR)
		if err != nil {
			r.mu.Lock()
			delete(r.table, db.Name)
			r.invalid[db.Name] = true
			r.mu.Unlock()
			return fmt.Errorf("invalid allowlist CIDR %q: %w", item.CIDR, err)
		}
		entry.allow = append(entry.allow, prefix.Masked())
	}
	for _, cidr := range db.Spec.EnvironmentIPAllowList {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			r.mu.Lock()
			delete(r.table, db.Name)
			r.invalid[db.Name] = true
			r.mu.Unlock()
			return fmt.Errorf("invalid environment allowlist CIDR %q: %w", cidr, err)
		}
		entry.envAllow = append(entry.envAllow, prefix.Masked())
	}
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
		if !allowedByLayer(source, entry.allow) || !allowedByLayer(source, entry.envAllow) {
			continue
		}
		switch {
		case base == name:
			backend := fmt.Sprintf("%s-rw.%s.svc.cluster.local:5432", name, entry.namespace)
			return dbRoute{Database: name, Backend: backend}, true
		case base == name+"-pool" && entry.pooler:
			backend := fmt.Sprintf("%s-pooler.%s.svc.cluster.local:5432", name, entry.namespace)
			return dbRoute{Database: name, Backend: backend}, true
		case strings.HasPrefix(base, name+"-ro-") && entry.replicas[strings.TrimPrefix(base, name+"-ro-")]:
			backend := fmt.Sprintf("%s-ro.%s.svc.cluster.local:5432", name, entry.namespace)
			return dbRoute{Database: name, Backend: backend}, true
		}
	}
	return dbRoute{}, false
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

	addr := defaultAddr
	if v := os.Getenv("BEX_PROXY_ADDR"); v != "" {
		addr = v
	}
	domain := os.Getenv("BEX_DB_DOMAIN")
	if domain == "" {
		log.Info("BEX_DB_DOMAIN is unset — proxy will refuse all connections (no routing table)")
	}

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
	metricsAddr := os.Getenv("BEX_PROXY_METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = defaultMetricsAddr
	}
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
			_ = ln.Close()
		case err := <-mgrErrCh:
			if err != nil {
				log.Error(err, "manager exited")
			}
			_ = ln.Close()
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Error(err, "accept error")
				continue
			}
		}
		go handleConn(conn, router, meter, log)
	}
}

// handleConn handles one incoming client connection:
//  1. Detects PostgreSQL SSLRequest preamble vs direct TLS.
//  2. Buffers the TLS ClientHello and extracts the SNI hostname.
//  3. Dials the matching CNPG backend.
//  4. Re-negotiates SSL with the backend (for preamble-mode clients).
//  5. Splices the connection bidirectionally.
func handleConn(conn net.Conn, router *dbRouter, meter *sniproxy.ByteMeter, log interface {
	Info(msg string, keysAndValues ...any)
	Error(err error, msg string, keysAndValues ...any)
}) {
	defer func() { _ = conn.Close() }()

	// Peek at the first 8 bytes to distinguish SSLRequest from direct TLS.
	var peek [8]byte
	if _, err := io.ReadFull(conn, peek[:]); err != nil {
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

	clientHello, err := sniproxy.ReadTLSRecord(conn, initial)
	if err != nil {
		log.Info("failed to read TLS ClientHello", "err", err)
		return
	}

	sni, err := sniproxy.ExtractSNI(clientHello)
	if err != nil || sni == "" {
		log.Info("no SNI in ClientHello", "err", err)
		return
	}

	source, err := sniproxy.RemoteIP(conn.RemoteAddr())
	if err != nil {
		return
	}
	route, ok := router.resolve(sni, source)
	if !ok {
		log.Info("no route for SNI", "sni", sni)
		return
	}

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
	// the client and CNPG — the proxy never sees the plaintext.
	sniproxy.CopyBidirectional(conn, back, meter, route.Database, "postgres")
}
