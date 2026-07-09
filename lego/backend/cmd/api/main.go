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

// Command api is bex-api: the product front door exposing REST, GraphQL and MCP
// for the App lifecycle verbs (list / get / restart / suspend / resume / logs /
// metrics) plus authz and API keys. It is a pure apiserver client — it patches
// App CRs and reads pod logs; the operator does the mechanism.
//
// When BEX_CP_DB_URI is set it ALSO runs the control plane (the Postgres source
// of truth, w1/m2): migrations on bex-db, the projector that turns `apps` rows
// into App CRs, the cluster-internal tenant API on :8091, and the single-writer
// wiring so suspend/resume update the row (not just the CR). Unset => bex-api is
// the Render surface alone. `api mcp-stdio` (or BEX_MCP_STDIO=1) serves only the
// MCP adapter over stdio for a local agent (DB-free).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/api"
	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	ctx := ctrl.SetupSignalHandler()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appv1alpha1.AddToScheme(scheme))

	cfg := ctrl.GetConfigOrDie() // in-cluster, or KUBECONFIG for local dev
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("kube client: %v", err)
	}
	// Clientset just for the pod-log + metrics-server subresources (the reads
	// controller-runtime's client can't serve); wired into the logs/metrics deps.
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("kube clientset: %v", err)
	}

	base := &core.Base{Client: cl, Namespace: envOr("BEX_API_NAMESPACE", "default")}

	deps := api.Deps{
		// BEX_BASE_DOMAIN names custom-domain DNS targets `<app>.<base>` (docs/custom-domain.md);
		// unset falls back to deriving the platform host from an App's status URLs.
		BaseDomain:    os.Getenv("BEX_BASE_DOMAIN"),
		PodLogs:       logs.NewPodLogSource(cs),
		PodLogsFollow: logs.NewPodLogStream(cs), // live tail for GET /v1/logs/subscribe
		// Resource metrics (cpu/memory) via metrics-server — the snapshot fallback
		// when Prometheus isn't wired below; instance count then needs no source.
		// Left nil if metrics-server is absent => those metrics report 503.
		ResourceMetrics: metrics.NewResourceMetricsSource(cs),
	}
	// Prometheus-backed history, wired only when BEX_PROM_URL is set: request
	// metrics (http_requests/latency/bandwidth via Traefik's counters — unwired
	// they 503) and resource-metrics history (cpu/memory/instance_count via
	// cAdvisor, preferred over the metrics-server snapshot; Prometheus set but
	// unreachable surfaces the query error, it does not silently fall back).
	if prom := os.Getenv("BEX_PROM_URL"); prom != "" {
		deps.RequestMetrics = metrics.NewPrometheusRequestSource(prom, nil)
		deps.ResourceMetricsRange = metrics.NewPrometheusResourceSource(prom, nil)
		deps.MonthToDateBandwidth = metrics.NewMonthToDateBandwidthSource(prom, nil)
		deps.MetricsFilterValues = metrics.NewPrometheusFilterValuesSource(prom, nil)
	}
	// Auth (docs/auth.md): OAuth2 API keys introspected at Hydra's admin API,
	// Kratos sessions optional. Handler() fails fast without the Hydra URL. nil key
	// store (stdio mode without a Hydra URL) keeps the api-key verbs answering
	// ErrAPIKeysUnavailable instead of dialing nowhere.
	hydraAdminURL := os.Getenv("BEX_HYDRA_ADMIN_URL")
	if hydraAdminURL != "" {
		deps.APIKeys = apikeys.NewHydraAPIKeys(hydraAdminURL)
	}
	// Tenant secrets (docs/secrets.md): the env-vars API stores values in OpenBao
	// KV v2, wired only when BEX_OPENBAO_URL is set — else the env-vars verbs 503
	// and the rest of the API is byte-for-byte unchanged.
	if bao := os.Getenv("BEX_OPENBAO_URL"); bao != "" {
		deps.Secrets = secrets.NewOpenBaoStore(bao)
	}
	// Authorization (docs/auth.md): unset => authz disabled (every verb allowed,
	// the pre-m4 behavior); set => every verb checks OpenFGA, fail closed. NOT
	// wired in stdio mode: that transport's trust boundary is the subprocess itself
	// (no auth gate, so no identity — a wired checker would deny all).
	var authzChecker core.Checker
	if fga := os.Getenv("BEX_OPENFGA_URL"); fga != "" && !mcpStdio() {
		authzChecker = authz.NewOpenFGAChecker(fga, os.Getenv("BEX_OPENFGA_TOKEN"))
		base.Authz = authzChecker
	}

	// Control plane (source of truth, w1/m2): opt-in via BEX_CP_DB_URI. When set,
	// bex-api owns bex-db — run migrations, the projector (apps rows -> App CRs),
	// the single-writer wiring, and the cluster-internal tenant API on :8091. Built
	// before NewServer so the store is wired into the apps service. DB-free in
	// stdio mode.
	if dbURI := os.Getenv("BEX_CP_DB_URI"); dbURI != "" && !mcpStdio() {
		appsNS := envOr("BEX_CP_APPS_NAMESPACE", "default")
		pool, err := pgxpool.New(ctx, dbURI)
		if err != nil {
			log.Fatalf("bex-api: db config: %v", err)
		}
		defer pool.Close()
		// CNPG may still be coming up when the pod starts — wait for the DB rather
		// than crash-looping, then converge the schema before serving.
		if err := waitForDB(ctx, pool); err != nil {
			log.Fatalf("bex-api: database unreachable: %v", err)
		}
		if err := store.Migrate(dbURI); err != nil {
			log.Fatalf("bex-api: %v", err)
		}

		st := store.NewPGStore(pool)
		rec := store.NewReconciler(cl, st, appsNS)
		if d := os.Getenv("BEX_CP_RESYNC"); d != "" {
			v, err := time.ParseDuration(d)
			if err != nil {
				log.Fatalf("bex-api: bad BEX_CP_RESYNC %q: %v", d, err)
			}
			rec.Resync = v
		}
		go rec.Run(ctx)
		deps.Store = st // single writer of intent: suspend/resume write the row first

		// Workspace lifecycle (w6/m1): the workspaces feature writes through the
		// same store and nudges the same projector to prune a deleted workspace's
		// App CRs. The OpenFGA checker (when wired) is both the grant and revoke
		// side, keeping workspace:tea-<id> tuples in step with tenant_members.
		deps.WorkspaceStore = st
		deps.WorkspaceKick = rec.Kick
		if g, ok := authzChecker.(workspaces.WorkspaceGranter); ok {
			deps.WorkspaceGranter = g
		}
		if rv, ok := authzChecker.(workspaces.WorkspaceRevoker); ok {
			deps.WorkspaceRevoker = rv
		}

		// Tenant onboarding + workspace scoping (w1/m9): one tenantService is the
		// store-backed resolver (core.Base.Workspace, every verb's Authorize
		// targets workspace:tea-<id>), the onboarding seam (mints a personal
		// tenant on a human's first login), and the key-binder (ties minted keys
		// to their tenant). The OpenFGA checker is the membership granter when
		// authz is wired; a nil granter means the store isolates tenants by
		// filtering while authz isn't yet enforcing roles (store on,
		// BEX_OPENFGA_URL off).
		var granter store.MembershipGranter
		if g, ok := authzChecker.(store.MembershipGranter); ok {
			granter = g
		}
		tenantSvc := api.NewTenantService(st, granter)
		base.Workspace = tenantSvc
		deps.Onboard = tenantSvc
		deps.KeyBinder = tenantSvc

		internal := &store.API{Store: st, Kick: rec.Kick, Health: st.Ping, Token: os.Getenv("BEX_CP_TOKEN"), Grant: granter}
		cpAddr := envOr("BEX_CP_ADDR", ":8091")
		cpSrv := &http.Server{
			Addr:              cpAddr,
			Handler:           internal.Handler(),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		}
		go func() {
			log.Printf("bex-api control plane (source of truth) on %s (projecting Apps into %q)", cpAddr, appsNS)
			if err := cpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("bex-api control plane: %v", err)
			}
		}()
	}

	srv := api.NewServer(base, deps)
	srv.CORSOrigin = os.Getenv("BEX_API_CORS_ORIGIN")
	srv.HydraAdminURL = hydraAdminURL
	srv.KratosURL = os.Getenv("BEX_KRATOS_URL")
	// OAuth 2.1 discovery for MCP/agent clients (w4/m9, docs/auth.md): the Hydra
	// public issuer + this API's canonical resource URI. Both unset => no
	// metadata endpoint, no audience check — behavior identical to before.
	srv.OAuthIssuer = os.Getenv("BEX_OAUTH_ISSUER")
	srv.OAuthResource = os.Getenv("BEX_OAUTH_RESOURCE")
	srv.WebhookSecret = os.Getenv("BEX_WEBHOOK_SECRET")

	// stdio MCP mode: `api mcp-stdio` (or BEX_MCP_STDIO=1) serves only the MCP
	// adapter over stdin/stdout — how a local agent launches bex as a subprocess.
	if mcpStdio() {
		log.Printf("bex-api: serving MCP over stdio (namespace %s)", base.Namespace)
		if err := srv.RunStdio(ctx); err != nil {
			log.Fatalf("bex-api mcp stdio: %v", err)
		}
		return
	}

	handler, err := srv.Handler()
	if err != nil {
		log.Fatalf("bex-api: %v", err)
	}

	addr := envOr("BEX_API_ADDR", ":8090")
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	log.Printf("bex-api listening on %s (namespace %s)", addr, base.Namespace)
	log.Fatal(httpSrv.ListenAndServe())
}

// waitForDB pings until the pool answers, for up to ~2 minutes.
func waitForDB(ctx context.Context, pool *pgxpool.Pool) error {
	var err error
	for range 60 {
		if err = pool.Ping(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return err
}

// mcpStdio reports whether the binary should run as a stdio MCP server rather
// than the HTTP service: subcommand `mcp-stdio` or BEX_MCP_STDIO=1.
func mcpStdio() bool {
	if os.Getenv("BEX_MCP_STDIO") == "1" {
		return true
	}
	return len(os.Args) > 1 && os.Args[1] == "mcp-stdio"
}
