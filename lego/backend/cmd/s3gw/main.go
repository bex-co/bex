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

// Command s3gw is bex-s3, the S3-compatible API (ADR112 D3): the REST protocol
// aws-cli, rclone and boto3 speak, at objects.<BEX_BASE_DOMAIN>. It runs from the
// bex-api image as a Deployment of its own, so a stream of up to 5 GiB for up to
// 30 minutes does not share pods with REST, GraphQL and MCP, and a rollout waits
// for it: on SIGTERM it stops taking new requests and lets its streams finish.
//
// Its environment is what the S3 surface reads and nothing else: the control
// plane's database, OpenFGA, OpenBao and Kratos' admin API (to authorize as an
// access key's creator and hold the workspace's login requirements), and the
// read-write Wasabi pair. Without the database or the Wasabi pair it stays up
// and answers 503 ServiceUnavailable, as objectgw does, so a cluster without
// them does not crash-loop. It mounts no REST, GraphQL, MCP or OAuth route.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/s3gw"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/serve"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("bex-s3: %v", err)
	}
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	var gw *s3gw.Gateway
	if reason := cfg.degradedReason(); reason != "" {
		log.Printf("bex-s3: %s; serving 503 until it is installed", reason)
		gw = s3gw.NewDegraded()
	} else {
		var closeDB func()
		gw, closeDB = build(ctx, cfg, registry)
		defer closeDB()
	}

	ready := &serve.Readiness{}
	public := s3gw.NewServer(s3gw.Addr, gw)
	probe := probeServer(s3gw.ProbeAddr, ready, registry)
	probeDone := make(chan error, 1)
	go func() { probeDone <- probe.ListenAndServe() }()
	log.Printf("bex-s3: S3 on %s, probes and metrics on %s", s3gw.Addr, s3gw.ProbeAddr)

	// The S3 listener keeps serving through the drain window while readiness
	// says draining, then Shutdown waits for every stream to finish, up to the
	// stream deadline plus a margin that fits the pod's grace period.
	err = runUntilStopped(ctx, public, ready, drainWindow)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("bex-s3: serve: %v", err)
	}
	probeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = probe.Shutdown(probeCtx)
	if err := <-probeDone; err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("bex-s3: probe server: %v", err)
	}
}

// runUntilStopped serves srv until ctx ends and then drains it: readiness flips to
// draining, srv keeps taking new requests for window so the edge stops routing
// here, then Shutdown refuses new connections and waits for every stream in
// flight to finish, up to the stream deadline plus the margin.
func runUntilStopped(ctx context.Context, srv *http.Server, ready *serve.Readiness, window time.Duration) error {
	return serve.UntilShutdown(ctx, srv, serve.Options{
		Readiness:       ready,
		DrainWindow:     window,
		ShutdownTimeout: objectstorage.StreamDeadline + shutdownMargin,
	})
}

// drainWindow is how long the S3 listener keeps taking new requests after
// SIGTERM, while readiness reports draining, so Traefik and kube-proxy stop
// routing here before the listener closes. shutdownMargin is what the stream
// deadline gets on top: the pod's terminationGracePeriodSeconds (2,100) covers
// the window, the deadline (1,800) and this.
const (
	drainWindow    = 10 * time.Second
	shutdownMargin = 60 * time.Second
)

// build wires the S3 surface over the control plane, OpenFGA, OpenBao, Kratos
// and Wasabi. It returns the closer of the database pool.
func build(ctx context.Context, cfg *config, registry *prometheus.Registry) (*s3gw.Gateway, func()) {
	pool, err := pgxpool.New(ctx, cfg.DBURI)
	if err != nil {
		log.Fatalf("bex-s3: database config: %v", err)
	}
	st := store.NewPGStore(pool)

	authzChecker := authz.NewOpenFGAChecker(cfg.OpenFGAURL, cfg.OpenFGAToken)
	base := &core.Base{Namespace: cfg.Namespace, Authz: authzChecker, Audit: st}
	var granter store.MembershipGranter
	if g, ok := authzChecker.(store.MembershipGranter); ok {
		granter = g
	}
	base.Workspace = tenancy.NewTenantService(st, granter)

	// The workspace's login requirements (two-factor, Google login) hold for an
	// S3 key as they do for an SSH key. They need Kratos' admin API; without it a
	// workspace that requires one fails closed.
	tf := twofactor.Config{Settings: st, Sessions: st, Metrics: twofactor.NewMetrics(registry)}
	if cfg.KratosAdminURL != "" {
		ids := workspaces.NewKratosIdentities(cfg.KratosAdminURL)
		tf.Enrollment, tf.Google = ids, ids
	}
	base.TwoFactor = twofactor.New(tf)

	svc := objectstorage.NewService(st, objectstorage.NewStores(cfg.ObjectStorage))
	svc.SetUsageSink(st)
	go svc.RunMeter(ctx, objectstorage.TransferFlushInterval)
	keys := objectstorage.NewAccessKeys(st, secrets.NewOpenBaoStore(cfg.OpenBaoURL, cfg.OpenBaoJWTPath))
	return s3gw.New(s3gw.Config{Service: svc, Keys: keys, Base: base, TrustedProxies: cfg.TrustedProxies}), pool.Close
}

// probeServer serves /healthz (liveness, always 200 once the process is up),
// /readyz (200 until draining) and /metrics, on the port nothing else uses, so a
// bucket named healthz or metrics is never shadowed.
func probeServer(addr string, ready *serve.Readiness, registry *prometheus.Registry) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /readyz", ready.Handler())
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}
