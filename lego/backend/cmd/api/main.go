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
	"strconv"
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
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/mailer"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/usage"
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
		// BEX_BASE_DOMAIN names custom-domain DNS targets `<app>.<base>` (docs/ADR005-custom-domain.md);
		// unset falls back to deriving the platform host from an App's status URLs.
		BaseDomain:    os.Getenv("BEX_BASE_DOMAIN"),
		PodLogs:       logs.NewPodLogSource(cs),
		PodLogsFollow: logs.NewPodLogStream(cs), // live tail for GET /v1/logs/subscribe (always pod logs)
		// Resource metrics (cpu/memory) via metrics-server — the snapshot fallback
		// when Prometheus isn't wired below; instance count then needs no source.
		// Left nil if metrics-server is absent => those metrics report 503.
		ResourceMetrics: metrics.NewResourceMetricsSource(cs),
	}
	// Durable log history, wired only when BEX_LOKI_URL is set: QueryLogs/Logs
	// then read Loki (history survives pod restarts) instead of live pod logs.
	// Unset => the pod-log path runs byte-identical to before (docs/ADR010-observability.md).
	// The SSE live tail stays on pod logs either way.
	// It also backs the request-log split (type=request) and the structured
	// filters/label discovery — the labels live in the store, not in a pod's
	// stdout, so unset means those are refused (503), never silently ignored.
	if lokiURL := os.Getenv("BEX_LOKI_URL"); lokiURL != "" {
		deps.LogHistory = logs.NewLokiSource(lokiURL, nil)
		deps.LogLabelValues = logs.NewLokiLabelValuesSource(lokiURL, nil)
	}
	// Prometheus-backed history, wired only when BEX_PROM_URL is set: request
	// metrics (http_requests/latency/bandwidth via Traefik's counters — unwired
	// they 503) and resource-metrics history (cpu/memory/instance_count via
	// cAdvisor, preferred over the metrics-server snapshot; Prometheus set but
	// unreachable surfaces the query error, it does not silently fall back).
	// promURL is also used by the usage metering block below.
	promURL := os.Getenv("BEX_PROM_URL")
	if promURL != "" {
		deps.RequestMetrics = metrics.NewPrometheusRequestSource(promURL, nil)
		deps.ResourceMetricsRange = metrics.NewPrometheusResourceSource(promURL, nil)
		deps.MonthToDateBandwidth = metrics.NewMonthToDateBandwidthSource(promURL, nil)
		deps.MetricsFilterValues = metrics.NewPrometheusFilterValuesSource(promURL, nil)
	}
	// Auth (docs/ADR012-auth.md): OAuth2 API keys introspected at Hydra's admin API,
	// Kratos sessions optional. Handler() fails fast without the Hydra URL. nil key
	// store (stdio mode without a Hydra URL) keeps the api-key verbs answering
	// ErrAPIKeysUnavailable instead of dialing nowhere.
	hydraAdminURL := os.Getenv("BEX_HYDRA_ADMIN_URL")
	if hydraAdminURL != "" {
		deps.APIKeys = apikeys.NewHydraAPIKeys(hydraAdminURL)
	}
	// Tenant secrets (docs/ADR013-secrets.md): the env-vars API stores values in OpenBao
	// KV v2, wired only when BEX_OPENBAO_URL is set — else the env-vars verbs 503
	// and the rest of the API is byte-for-byte unchanged.
	if bao := os.Getenv("BEX_OPENBAO_URL"); bao != "" {
		deps.Secrets = secrets.NewOpenBaoStore(bao)
	}
	// GitHub App integration (docs/ADR026-github-integration.md): private-repo deploys +
	// zero-config push-to-deploy. Wired only when all three BEX_GITHUB_APP_* vars
	// are set (and the key parses) — else the git-connect verbs 503. The store
	// half (git_connections) is wired inside the BEX_CP_DB_URI block below.
	if appID, key, slug := os.Getenv("BEX_GITHUB_APP_ID"), os.Getenv("BEX_GITHUB_APP_PRIVATE_KEY"), os.Getenv("BEX_GITHUB_APP_SLUG"); appID != "" && key != "" && slug != "" {
		ghClient, err := github.NewClient(github.Config{AppID: appID, PrivateKey: key, Slug: slug})
		if err != nil {
			log.Fatalf("bex-api: github app config: %v", err)
		}
		deps.GitHubClient = ghClient
	}
	// Owner/member identity attributes (w6/m2): Kratos' admin API, distinct from
	// the public BEX_KRATOS_URL session whoami above — looking up OTHER members'
	// email/MFA needs the admin API, not a session. Unset => those fields omitted.
	if kratosAdmin := os.Getenv("BEX_KRATOS_ADMIN_URL"); kratosAdmin != "" {
		deps.Identities = workspaces.NewKratosIdentities(kratosAdmin)
	}
	// Authorization (docs/ADR012-auth.md): unset => authz disabled (every verb allowed,
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
	//
	// rec is hoisted outside the if-block so NewServer can wire the apps.Service
	// as the reconciler's CloneSecreter (and vice-versa) after both are constructed
	// (w2/m11): rec.Run is deferred until after NewServer for the same reason.
	var rec *store.Reconciler
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
		rec = store.NewReconciler(cl, st, appsNS)
		if d := os.Getenv("BEX_CP_RESYNC"); d != "" {
			v, err := time.ParseDuration(d)
			if err != nil {
				log.Fatalf("bex-api: bad BEX_CP_RESYNC %q: %v", d, err)
			}
			rec.Resync = v
		}
		// rec.Run is started after NewServer below, so CloneSecrets is set before
		// the first reconcile pass (w2/m11).
		deps.Store = st       // single writer of intent: suspend/resume write the row first
		deps.DeployStore = st // deploy history (w2/m5): list/get/trigger read+write the same rows
		// Cancel (w2/m10) needs to compute a repo-backed App's in-flight build
		// Job's identity — must match the operator's own BEX_BUILD_NAMESPACE.
		deps.DeployBuildNamespace = os.Getenv("BEX_BUILD_NAMESPACE")
		deps.GitHubStore = st // git connections (w2/m8): connect/disconnect/list read+write git_connections
		deps.EventStore = st  // service events (w3/m7): the feed composes deploys + audit_events, writing neither

		// Audit log (w4/m10): *store.PGStore structurally satisfies
		// core.AuditSink, so every write verb's Authorize/AuthorizeOn call
		// starts recording the instant the store is wired — no extra plumbing.
		base.Audit = st

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
		// Out-of-cascade teardown (w6/m4/t005): a deleted workspace's OpenBao
		// secrets and managed Databases/KeyValue stores live outside the tenant
		// row's FK cascade, so Delete runs these after the row is gone. Order
		// doesn't matter — each purger only touches its own resource type.
		//
		// apps.WorkspacePurger (w6/m11, live-verification finding): an App
		// created through the public REST/GraphQL/MCP surface carries
		// core.LabelTenant only, never store.LabelManagedBy, so the row-backed
		// cascade + reconciler prune that tears down *row-backed* Apps on
		// workspace delete never sees it — it would otherwise survive forever,
		// still running and permanently unreachable (its tenant is gone, so
		// core.Base's tenant gate forbids everyone, including its creator).
		// Redundant-but-harmless for row-backed Apps the reconciler already
		// pruned (delete of an already-gone object is a no-op).
		deps.WorkspacePurgers = []workspaces.WorkspacePurger{
			&secrets.WorkspacePurger{Service: &secrets.Service{Base: base, Store: deps.Secrets}},
			&postgres.WorkspacePurger{Service: &postgres.Service{Base: base}},
			&keyvalue.WorkspacePurger{Service: &keyvalue.Service{Base: base}},
			&apps.WorkspacePurger{Service: &apps.Service{Base: base}},
		}

		// Workspace members & roles (w4/m12): the team surface writes through the
		// same store (tenant_members + tenant_invites) and keeps OpenFGA role
		// tuples in step. Granter/Revoker are the OpenFGA checker when authz is
		// wired; a nil pair means the store records roles while authz isn't yet
		// enforcing them (store on, BEX_OPENFGA_URL off).
		deps.MembersStore = st
		if g, ok := authzChecker.(members.RoleGranter); ok {
			deps.MembersGranter = g
		}
		if rv, ok := authzChecker.(members.RoleRevoker); ok {
			deps.MembersRevoker = rv
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

		// Usage metering (w8/m1) + retention (m4): the loop rolls usage_hourly
		// rows up every hour (needs Prometheus; skipped without it) and compacts
		// months older than the hot window into usage_monthly daily. The hot
		// window is BEX_USAGE_RETENTION_MONTHS calendar months (current month
		// included; default 3, minimum 1) — docs/ADR023-usage-metering.md.
		usageSvc := usage.NewService(base, st, promURL, nil)
		if v := os.Getenv("BEX_USAGE_RETENTION_MONTHS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 1 {
				usageSvc.RetentionMonths = n
			} else {
				log.Printf("BEX_USAGE_RETENTION_MONTHS=%q invalid (want integer ≥ 1); using default %d", v, usage.DefaultRetentionMonths)
			}
		}
		deps.Usage = usageSvc
		go usageSvc.Run(ctx)

		// Audit log retention (w4/m10): purges audit_events rows older than
		// BEX_AUDIT_RETENTION_DAYS daily, same cadence/shape as usage's
		// compaction loop above. The write side is base.Audit (wired above);
		// this Service is the read verb + the sweep only.
		auditSvc := &audit.Service{Base: base, Store: st}
		if v := os.Getenv("BEX_AUDIT_RETENTION_DAYS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 1 {
				auditSvc.RetentionDays = n
			} else {
				log.Printf("BEX_AUDIT_RETENTION_DAYS=%q invalid (want integer ≥ 1); using default %d", v, audit.DefaultRetentionDays)
			}
		}
		deps.Audit = auditSvc
		go auditSvc.Run(ctx)

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

	// Invite delivery (w4/m12): the members feature emails invites over the same
	// SMTP relay Kratos's courier uses (SendGrid in prod, Mailpit locally). Unset
	// BEX_SMTP_ADDR/BEX_SMTP_FROM => mailer nil, invites recorded but not emailed.
	// BEX_DASHBOARD_URL is the origin the invite link points at.
	if m := mailer.New(os.Getenv("BEX_SMTP_ADDR"), os.Getenv("BEX_SMTP_FROM"),
		os.Getenv("BEX_SMTP_USERNAME"), os.Getenv("BEX_SMTP_PASSWORD")); m != nil {
		deps.Mailer = m
	}
	deps.InviteBaseURL = os.Getenv("BEX_DASHBOARD_URL")
	// The GitHub install callback (docs/ADR026-github-integration.md) redirects the
	// browser back to the dashboard settings page on success.
	deps.DashboardURL = os.Getenv("BEX_DASHBOARD_URL")

	// Per-workspace resource caps (w7/m9): 0 (unset) = unlimited, byte-identical.
	// Render-Hobby defaults: BEX_MAX_SERVICES=25, BEX_MAX_POSTGRES=1, BEX_MAX_KEYVALUES=1.
	deps.MaxServices, _ = strconv.Atoi(os.Getenv("BEX_MAX_SERVICES"))
	deps.MaxPostgres, _ = strconv.Atoi(os.Getenv("BEX_MAX_POSTGRES"))
	deps.MaxKeyValues, _ = strconv.Atoi(os.Getenv("BEX_MAX_KEYVALUES"))

	srv := api.NewServer(base, deps)
	// Wire the reconciler ↔ apps.Service now that both exist (w2/m11):
	// - CloneSecrets: the projector mints clone Secrets for private-repo rows
	//   created via the internal CP API (store/api.go POST /v1/apps).
	// - Kick on the apps.Service: after a store-managed create/redeploy the
	//   projector runs immediately instead of waiting the next resync period.
	// rec.Run is started here — after the wiring — so CloneSecrets is already
	// set before the first reconcile pass runs.
	if rec != nil {
		rec.CloneSecrets = srv.Apps.ReconcilerCloneSecreter()
		srv.Apps.Kick = rec.Kick
		go rec.Run(ctx)
	}
	srv.CORSOrigin = os.Getenv("BEX_API_CORS_ORIGIN")
	srv.HydraAdminURL = hydraAdminURL
	srv.KratosURL = os.Getenv("BEX_KRATOS_URL")
	// OAuth 2.1 discovery for MCP/agent clients (w4/m9, docs/ADR012-auth.md): the Hydra
	// public issuer + this API's canonical resource URI. Both unset => no
	// metadata endpoint, no audience check — behavior identical to before.
	srv.OAuthIssuer = os.Getenv("BEX_OAUTH_ISSUER")
	srv.OAuthResource = os.Getenv("BEX_OAUTH_RESOURCE")
	srv.WebhookSecret = os.Getenv("BEX_WEBHOOK_SECRET")
	// The GitHub App's app-wide webhook signs pushes with its own secret — a
	// second accepted key so installed repos redeploy hands-free
	// (docs/ADR026-github-integration.md).
	srv.GitHubWebhookSecret = os.Getenv("BEX_GITHUB_WEBHOOK_SECRET")

	// stdio MCP mode: `api mcp-stdio` (or BEX_MCP_STDIO=1) serves only the MCP
	// adapter over stdin/stdout — how a local agent launches bex as a subprocess.
	if mcpStdio() {
		log.Printf("bex-api: serving MCP over stdio (namespace %s)", base.Namespace)
		if err := srv.RunStdio(ctx); err != nil {
			log.Fatalf("bex-api mcp stdio: %v", err)
		}
		return
	}

	// Rate limiting + request caps (w7/m3). BEX_RATE_LIMIT=0 disables the limiter.
	rpmStr := envOr("BEX_RATE_LIMIT", "500")
	rpm, err := strconv.ParseFloat(rpmStr, 64)
	if err != nil {
		log.Fatalf("bex-api: bad BEX_RATE_LIMIT %q: %v", rpmStr, err)
	}
	burstStr := envOr("BEX_RATE_BURST", "0")
	burst, _ := strconv.Atoi(burstStr)
	srv.RateLimiter = api.NewRateLimiter(rpm, burst) // nil when rpm=0 (disabled)

	maxBody, _ := strconv.ParseInt(envOr("BEX_MAX_BODY_BYTES", "2097152"), 10, 64)
	srv.MaxBodyBytes = maxBody

	maxQueryHours, _ := strconv.Atoi(envOr("BEX_MAX_QUERY_HOURS", "720"))
	maxSSEConns, _ := strconv.ParseInt(envOr("BEX_MAX_SSE_CONNS", "100"), 10, 64)
	srv.Logs.MaxQueryHours = maxQueryHours
	srv.Logs.MaxSSEConns = maxSSEConns
	srv.Metrics.MaxQueryHours = maxQueryHours
	srv.Events.MaxQueryHours = maxQueryHours

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
