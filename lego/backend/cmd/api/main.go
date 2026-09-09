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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/accounts"
	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/api"
	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/cliauth"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/deploys"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/mailer"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/notifications"
	pushtransport "github.com/bex-co/bex/lego/backend/internal/notifications/push"
	"github.com/bex-co/bex/lego/backend/internal/opsrole"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/registrycreds"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/serve"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/usage"
	"github.com/bex-co/bex/lego/backend/internal/webhooks"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func sandboxTemplateRegistry(baseImage, agentImage string) map[string]sandbox.Template {
	return map[string]sandbox.Template{
		"base": {
			Image: baseImage, Entrypoint: []string{"sleep", "infinity"},
			CPU: "500m", Memory: "512Mi",
		},
		"agent": {
			Image: agentImage, Entrypoint: []string{"bex-agent-driver"},
			CPU: "2", Memory: "4Gi",
		},
	}
}

// requireCPAuth fails closed for the internal control-plane API (:8091): it
// grants workspace-admin and cross-tenant writes, so an empty BEX_CP_TOKEN
// (no bearer gate) must abort startup rather than silently serve open. The
// BEX_CP_INSECURE=1 escape hatch keeps local dev working and logs loudly; prod
// never sets it. Extracted so the guard is unit-testable without booting main.
func requireCPAuth(token, insecure string) error {
	if token != "" {
		return nil
	}
	if insecure == "1" {
		log.Printf("WARNING: BEX_CP_TOKEN is empty and BEX_CP_INSECURE=1 — the control-plane API (:8091) serves UNAUTHENTICATED. Never do this outside local dev.")
		return nil
	}
	return errors.New("BEX_CP_TOKEN is required when the control plane is enabled (BEX_CP_DB_URI set): the internal API grants workspace-admin and cross-tenant writes; refusing to serve it unauthenticated. Set BEX_CP_TOKEN, or set BEX_CP_INSECURE=1 to override in local dev only")
}

// sandboxKeyProvider adapts the control-plane store to the sandbox feature's
// KeyProvider: it mints/returns each workspace's opaque OpenSandbox tenant key
// (m32 t006). The reverse lookup (key → `<ws>-sandbox` namespace) is served by
// the CP tenant-lookup endpoint the OpenSandbox server calls back to.
type sandboxKeyProvider struct{ st *store.PGStore }

func (p sandboxKeyProvider) WorkspaceKey(ctx context.Context, workspaceID string) (string, error) {
	return p.st.SandboxKeyForWorkspace(ctx, workspaceID)
}

// drainWindow is how long a SIGTERM'd bex-api keeps serving NEW requests while
// /readyz reports draining (w1/m52): long enough for the readiness probe
// (periodSeconds 5, failureThreshold 1) plus endpoint/ingress propagation to
// stop routing here, and — with serve's 10s in-flight shutdown budget — inside
// the default 30s terminationGracePeriodSeconds. A second signal force-exits
// immediately (ctrl.SetupSignalHandler), so local Ctrl-C isn't held hostage.
const drainWindow = 15 * time.Second

func main() {
	ctx := ctrl.SetupSignalHandler()
	// Parse + validate the ENTIRE environment contract first (.pm/w1/070.md):
	// a malformed knob aborts HERE — before store.Migrate runs, any worker
	// loop starts, or either listener binds — instead of crashlooping the pod
	// through repeated migrations and partial serving. loadConfig collects
	// every problem, so one restart shows the whole fix list; cmd/ is the only
	// env reader.
	cfg, warnings, err := loadConfig(os.Getenv, time.Now(), os.Args)
	if err != nil {
		log.Fatalf("bex-api: %v", err)
	}
	for _, w := range warnings {
		log.Print(w)
	}
	cpDBURI := cfg.CPDBURI
	dashboardURL := cfg.DashboardURL
	mobilePush, err := pushtransport.New(pushtransport.Config{
		Provider: cfg.PushProvider, AccessToken: cfg.ExpoPushAccessToken,
		Endpoint: cfg.ExpoPushURL,
	})
	if err != nil {
		log.Fatalf("bex-api: push config: %v", err)
	}
	webPush, err := pushtransport.NewWebPush(pushtransport.WebPushConfig{
		PublicKey: cfg.WebPushVAPIDPublicKey, PrivateKey: cfg.WebPushVAPIDPrivateKey,
		Subscriber: cfg.WebPushSubscriber,
	})
	if err != nil {
		log.Fatalf("bex-api: web push config: %v", err)
	}
	metricRegistry := prometheus.NewRegistry()
	metricRegistry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	billingMetrics := billing.NewMetrics(metricRegistry)
	pushMetrics := notifications.NewPushMetrics(metricRegistry)
	webhookMetrics := webhooks.NewMetrics(metricRegistry)
	// Shared by Completer, Service, and ModelAuthFailer (w5/m81 + w5/m88). Created
	// early so the control-plane auth-failure verb can observe vendor rejections.
	agentMetrics := agentsessions.NewCompletionMetrics(metricRegistry)
	pushMetrics.SetEnabled(mobilePush != nil || webPush != nil)

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appv1alpha1.AddToScheme(scheme))

	kubeCfg := ctrl.GetConfigOrDie() // in-cluster, or KUBECONFIG for local dev
	cl, err := client.New(kubeCfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("kube client: %v", err)
	}
	// Clientset just for the pod-log + metrics-server subresources (the reads
	// controller-runtime's client can't serve); wired into the logs/metrics deps.
	cs, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		log.Fatalf("kube clientset: %v", err)
	}

	base := &core.Base{Client: cl, Namespace: cfg.Namespace}

	// One readiness flag for the whole pod (w1/m52): the public server's
	// /readyz answers 200 until SIGTERM, then 503 while both servers keep
	// serving through the drain window. The readiness probe points here;
	// liveness stays on /healthz, which never flips.
	ready := &serve.Readiness{}

	deps := api.Deps{
		PushAvailable:    mobilePush != nil,
		WebPushAvailable: webPush != nil,
		WebhookMetrics:   webhookMetrics,
		// BEX_BASE_DOMAIN names custom-domain DNS targets `<app>.<base>` (docs/ADR005-custom-domain.md);
		// unset falls back to deriving the platform host from an App's status URLs.
		BaseDomain: cfg.BaseDomain,
		// BEX_REGION is the explicit platform placement surfaced in Render
		// resource metadata. Empty is honestly omitted.
		Region:        cfg.Region,
		PodLogs:       logs.NewPodLogSource(cs),
		PodLogsFollow: logs.NewPodLogStream(cs), // live tail for GET /v1/logs/subscribe (always pod logs)
		// Resource metrics (cpu/memory) via metrics-server — the snapshot fallback
		// when Prometheus isn't wired below; instance count then needs no source.
		// Left nil if metrics-server is absent => those metrics report 503.
		ResourceMetrics: metrics.NewResourceMetricsSource(cs),
	}
	if webPush != nil {
		deps.WebPushVAPIDPublicKey = webPush.PublicKey()
	}
	wireObservability(&deps, cfg)
	// Auth (docs/ADR012-auth.md): OAuth2 API keys introspected at Hydra's admin API,
	// Kratos sessions optional. Handler() fails fast without the Hydra URL. nil key
	// store (stdio mode without a Hydra URL) keeps the api-key verbs answering
	// ErrAPIKeysUnavailable instead of dialing nowhere.
	hydraAdminURL := cfg.HydraAdminURL
	if hydraAdminURL != "" {
		deps.APIKeys = apikeys.NewHydraAPIKeys(hydraAdminURL)
	}
	// Tenant secrets (docs/ADR013-secrets.md): the env-vars API stores values in OpenBao
	// KV v2, wired only when BEX_OPENBAO_URL is set — else the env-vars verbs 503
	// and the rest of the API is byte-for-byte unchanged.
	if cfg.OpenBaoURL != "" {
		deps.Secrets = secrets.NewOpenBaoStore(cfg.OpenBaoURL, cfg.OpenBaoJWTPath)
	}
	ghClient := wireGitHubApp(&deps, cfg)
	// Owner/member identity attributes (w6/m2): Kratos' admin API, distinct from
	// the public BEX_KRATOS_URL session whoami above — looking up OTHER members'
	// email/MFA needs the admin API, not a session. Unset => those fields omitted.
	kratosAdminURL := cfg.KratosAdminURL
	if kratosAdminURL != "" {
		deps.Identities = workspaces.NewKratosIdentities(kratosAdminURL)
	}
	oryAccountCleaner := accounts.NewOryCleaner(hydraAdminURL, kratosAdminURL)
	deps.AccountOAuth = oryAccountCleaner
	deps.AccountKratos = oryAccountCleaner
	authzChecker := wireAuthz(base, cfg)
	// Ops-workspace pin (docs/ADR088-platform-observability-ui.md §4): the id
	// arms the delete guard (workspaces service, via Deps) and the store-level
	// guards (invite seat-cap exemption + account-deletion disposition, set on
	// st below); the internal ops-role verb needs the bearer too and is nil —
	// never mounted — until both are configured.
	deps.OpsWorkspaceID = cfg.OpsWorkspace
	opsRole := opsRoleHandler(cfg, authzChecker, deps.Identities)

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
	// st is hoisted like rec: the outbound-webhook delivery worker (w3/m11) is
	// constructed after NewServer (it shares deps.Mailer + the identity email
	// lookup) but reads/writes the store directly.
	var st *store.PGStore
	var stripeLifecycleWorker *billing.Worker
	var stripeLifecycleReconciler *billing.Reconciler
	var stripeBillingAdmin store.BillingAdmin
	if cpDBURI != "" && !cfg.MCPStdio {
		// The datastore (Database/KeyValue) projection namespace; falls back to
		// BEX_API_NAMESPACE so the two agree unless explicitly split.
		appsNS := cfg.CPAppsNamespace
		pool := openControlPlaneDB(ctx, cpDBURI)
		defer pool.Close()

		st = store.NewPGStore(pool)
		// ADR088 §4 store-level guards: invite redemption into the pinned ops
		// workspace skips seat/plan gating, and account-deletion disposition
		// classifies a sole-member ops workspace blocked instead of delete.
		st.OpsWorkspaceID = cfg.OpsWorkspace
		rec = store.NewReconciler(cl, st)
		rec.Metrics = store.NewReconcilerMetrics(metricRegistry)
		if cfg.CPResyncSet {
			rec.Resync = cfg.CPResync
		}
		// rec.Run is started after NewServer below, so CloneSecrets is set before
		// the first reconcile pass (w2/m11).
		granter := wireControlPlaneFeatures(cfg, &deps, base, st, rec, authzChecker)

		// Usage metering (w8/m1) + retention (m4): the loop rolls usage_hourly
		// rows up every hour (needs Prometheus; skipped without it) and compacts
		// months older than the hot window into usage_monthly daily. The hot
		// window is BEX_USAGE_RETENTION_MONTHS calendar months (current month
		// included; default 3, minimum 1) — docs/ADR023-usage-metering.md.
		usageSvc := usage.NewService(base, st, cfg.PromURL, nil)
		if cfg.UsageRetentionSet {
			usageSvc.RetentionMonths = cfg.UsageRetention
		}
		// build_seconds counts Jobs where the operator runs them — must match the
		// manager's own BEX_BUILD_NAMESPACE, the same way Cancel's Job identity
		// does (deps.DeployBuildNamespace below).
		usageSvc.BuildNamespace = cfg.BuildNamespace
		deps.Usage = usageSvc

		stripeLifecycleWorker, stripeLifecycleReconciler, stripeBillingAdmin = wireStripeBilling(ctx, cfg, &deps, base, cl, st, appsNS, usageSvc, billingMetrics)

		go usageSvc.Run(ctx)

		// Audit log retention (w4/m10 + w2/m39): purges audit_events and SSH
		// session metadata older than BEX_AUDIT_RETENTION_DAYS daily, same
		// cadence/shape as usage's compaction loop above. The write side is
		// base.Audit (wired above); this Service is the read verb + sweep only.
		auditSvc := &audit.Service{Base: base, Store: st}
		if cfg.AuditRetentionSet {
			auditSvc.RetentionDays = cfg.AuditRetention
		}
		deps.Audit = auditSvc
		go auditSvc.Run(ctx)

		startControlPlaneServer(ctx, cfg, st, rec, granter, stripeBillingAdmin, metricRegistry, agentMetrics, ghClient, deps.Secrets, opsRole, ready)
	} else if opsRole != nil && !cfg.MCPStdio {
		// ADR088 §4: without the control plane there is no :8091 mux to share,
		// so a configured ops-role verb gets its own minimal cluster-internal
		// listener (local dev / e2e without BEX_CP_DB_URI). Production always
		// runs the control plane and takes the branch above.
		startOpsRoleServer(ctx, cfg, opsRole, ready)
	}

	// Invite delivery (w4/m12): the members feature emails invites over the same
	// SMTP relay Kratos's courier uses (SendGrid in prod, Mailpit locally). Unset
	// BEX_SMTP_ADDR/BEX_SMTP_FROM => mailer nil, invites recorded but not emailed.
	// BEX_DASHBOARD_URL is the origin the invite link points at.
	if m := mailer.New(cfg.SMTPAddr, cfg.SMTPFrom,
		cfg.SMTPUsername, cfg.SMTPPassword); m != nil {
		deps.Mailer = m
	}
	deps.InviteBaseURL = dashboardURL
	// The GitHub install callback (docs/ADR026-github-integration.md) redirects the
	// browser back to dashboard settings on success and with a bounded error code
	// on state/install failures.
	deps.DashboardURL = dashboardURL
	deps.DeployHookBaseURL = cfg.APIPublicURL
	deps.SSHHost = cfg.SSHHost
	// ADR062/ADR064 model proxy: the internal gateway origin agent model traffic is
	// routed through so the BYO key never enters the sandbox. Unset keeps session
	// mutation unavailable. Set before wireSandboxes so session egress derives its
	// port from this one value (single source of truth).
	deps.AgentModelProxyURL = cfg.AgentModelProxyURL
	wireSandboxes(ctx, cfg, &deps, cl, st)
	wireAgentSessions(&deps, cfg)
	wireDiskSnapshots(&deps, cfg)
	srv := api.NewServer(base, deps)
	if mode := cfg.EnvGroupNameClaimAudit; mode != "" {
		report, auditErr := envgroups.AuditNameClaims(ctx, deps.Secrets, mode == "dry-run")
		if auditErr != nil {
			log.Fatalf("bex-api: environment-group name-claim audit: %v", auditErr)
		}
		log.Printf("bex-api: environment-group name-claim audit mode=%s scanned=%d missing=%d created=%d existing=%d conflicts=%d duplicates=%v",
			mode, report.Scanned, report.Missing, report.Created, report.Existing, report.Conflicts, report.Duplicates)
	}
	// w2/m80: the explicit, opt-in move of env-groups off the shared legacy
	// OpenBao tenant onto their own workspace-prefixed tenants. See
	// docs/runbooks/env-group-path-migration.md before ever running apply
	// mode against a production store.
	if mode := cfg.EnvGroupPathMigration; mode != "" {
		report, migrateErr := envgroups.MigratePaths(ctx, deps.Secrets, mode == "dry-run")
		if migrateErr != nil {
			log.Fatalf("bex-api: environment-group path migration: %v", migrateErr)
		}
		log.Printf("bex-api: environment-group path migration mode=%s scanned=%d migrated=%d alreadyMigrated=%d skippedNoWorkspace=%d failed=%v",
			mode, report.Scanned, report.Migrated, report.AlreadyMigrated, report.SkippedNoWorkspace, report.Failed)
	}
	// One CompletionMetrics registration shared by the Completer (turn duration,
	// convergence) and the Service (provisioning latency) so the agent-session
	// lifecycle timing lives under one set of series (w5/m81). Created above with
	// the other registries so ModelAuthFailer on :8091 can observe too (w5/m88).
	srv.AgentSessionCompleter.Metrics = agentMetrics
	srv.AgentSessions.Metrics = agentMetrics
	// codex round-8 #9: the signed git webhook durably claims each processed
	// delivery body so a captured (body, signature) pair cannot be replayed into
	// repeated deploys. A configured webhook without this durable store is
	// rejected below; store-less deployments must leave webhook secrets unset.
	if st != nil {
		srv.WebhookReplays = st
		srv.CLIRefreshes = st
		// Replay claims are partitioned by the authenticated signing-secret epoch.
		// Every replica leases the epochs it still accepts; only after all leases
		// retire may maintenance purge that epoch without making an accepted old
		// signature replayable again (codex-security U9MUKo finding 4).
		epochs := store.GitWebhookReplayEpochs(cfg.WebhookSecret, cfg.GitHubWebhookSecret)
		if _, err := st.MaintainGitWebhookReplayEpochs(ctx, epochs, time.Now()); err != nil {
			log.Fatalf("bex-api: register git webhook replay epochs: %v", err)
		}
		go st.RunGitWebhookReplayMaintenance(ctx, epochs)
	}
	// Membership rows and exact OpenFGA roles are joined by a transactional
	// Postgres outbox. Drain it independently of request retries so an invite or
	// downgrade survives transient OpenFGA failures and process restarts.
	go srv.Members.RunRoleReconciler(ctx)
	go srv.Accounts.Run(ctx)
	// The agent-session Completer finalizes fire-and-forget sessions: it opens the
	// draft PR + records evidence for completed turns (ADR047 D4, w3/m41). It is a
	// no-op unless the store, OpenSandbox, and GitHub App are all wired.
	go srv.AgentSessionCompleter.Run(ctx)
	if stripeLifecycleWorker != nil {
		stripeLifecycleWorker.Notifier = notifications.BillingNotifier{Service: srv.Notifications}
		go stripeLifecycleWorker.Run(ctx)
		go stripeLifecycleReconciler.Run(ctx)
	}

	wireReconcilers(ctx, srv, rec, st, cl, base, cfg)
	startDeliveryWorkers(ctx, cfg, srv, st, deps.Mailer, mobilePush, webPush, pushMetrics, webhookMetrics)
	srv.CORSOrigin = cfg.CORSOrigin
	srv.HydraAdminURL = hydraAdminURL
	srv.KratosURL = cfg.KratosURL
	configureServerAuthOptions(srv, cfg)

	// stdio MCP mode: `api mcp-stdio` (or BEX_MCP_STDIO=1) serves only the MCP
	// adapter over stdin/stdout — how a local agent launches bex as a subprocess.
	if cfg.MCPStdio {
		log.Printf("bex-api: serving MCP over stdio (namespace %s)", base.Namespace)
		if err := srv.RunStdio(ctx); err != nil {
			log.Fatalf("bex-api mcp stdio: %v", err)
		}
		return
	}

	// Deploy-hook credentials created before the digest index existed must be
	// indexed before the public HTTP route can serve. This is the sole intentional
	// cluster-wide migration list; requests thereafter use an exact label query.
	if err := deploys.BackfillDeployHookTokenDigests(ctx, base.Client); err != nil {
		log.Fatalf("bex-api: deploy-hook token index backfill: %v", err)
	}

	configureRateLimiters(srv, cfg)

	srv.MaxBodyBytes = cfg.MaxBodyBytes

	maxQueryHours := cfg.MaxQueryHours
	srv.Logs.MaxQueryHours = maxQueryHours
	srv.Logs.MaxSSEConns = cfg.MaxSSEConns
	srv.Logs.MaxSSEConnsPerSubject = cfg.MaxSSEConnsPerSubject
	srv.Logs.MaxSSEConnsPerWorkspace = cfg.MaxSSEConnsPerWorkspace
	// w4/034: the live log tail's authorization watchdog cadence — every
	// established SSE/WebSocket/NDJSON subscription re-runs a FRESH
	// can_view_logs check on this interval so a revocation ends the stream
	// within one interval, not at the next admission. Negative disables.
	srv.Logs.RevalidateInterval = cfg.LogStreamRevalidateInterval
	srv.Metrics.MaxQueryHours = maxQueryHours
	srv.Events.MaxQueryHours = maxQueryHours

	handler, err := srv.Handler()
	if err != nil {
		log.Fatalf("bex-api: %v", err)
	}

	// /readyz mounts OUTSIDE the product handler (w1/m52): it is a deployment
	// lifecycle endpoint for the readiness probe, not part of the Render-shaped
	// API surface, so internal/api stays untouched and parity-clean. Everything
	// else falls through to the product handler unchanged.
	root := http.NewServeMux()
	root.Handle("GET /readyz", ready.Handler())
	root.Handle("/", handler)

	addr := cfg.APIAddr
	httpSrv := newHTTPServer(addr, root)
	log.Printf("bex-api listening on %s (namespace %s)", addr, base.Namespace)
	// Serve in a goroutine and block on ctx (SIGTERM/SIGINT via
	// ctrl.SetupSignalHandler above) so the process shuts the server down
	// gracefully instead of serving for the whole termination grace period with
	// every background loop's context already cancelled (w1/m30, .pm/w1/019.md).
	// On SIGTERM: /readyz flips to 503, the server keeps serving new requests
	// through drainWindow, then the in-flight drain runs (w1/m52).
	if err := serve.UntilShutdown(ctx, httpSrv, serve.Options{Readiness: ready, DrainWindow: drainWindow}); err != nil {
		log.Fatalf("bex-api: %v", err)
	}
}

// wireObservability wires the optional log and metric history sources.
func wireObservability(deps *api.Deps, cfg *Config) {
	// Durable log history, wired only when BEX_LOKI_URL is set: QueryLogs/Logs
	// then read Loki (history survives pod restarts) instead of live pod logs.
	// Unset => the pod-log path runs byte-identical to before (docs/ADR010-observability.md).
	// The SSE live tail stays on pod logs either way.
	// It also backs the request-log split (type=request) and the structured
	// filters/label discovery — the labels live in the store, not in a pod's
	// stdout, so unset means those are refused (503), never silently ignored.
	if lokiURL := cfg.LokiURL; lokiURL != "" {
		deps.LogHistory = logs.NewLokiSource(lokiURL, nil)
		deps.LogLabelValues = logs.NewLokiLabelValuesSource(lokiURL, nil)
		// Host/path-filtered request metrics are served from the same Traefik
		// access log in Loki (w5/m58) — the only store with a per-request
		// host/path axis; unset => a host/path-filtered metrics read 503s.
		deps.RequestLogMetrics = metrics.NewLokiRequestMetricsSource(lokiURL, nil)
	}
	// Prometheus-backed history, wired only when BEX_PROM_URL is set: request
	// metrics (http_requests/latency/bandwidth via Traefik's counters — unwired
	// they 503) and resource-metrics history (cpu/memory/instance_count via
	// cAdvisor, preferred over the metrics-server snapshot; Prometheus set but
	// unreachable surfaces the query error, it does not silently fall back).
	promURL := cfg.PromURL
	if promURL != "" {
		deps.RequestMetrics = metrics.NewPrometheusRequestSource(promURL, nil)
		deps.ResourceMetricsRange = metrics.NewPrometheusResourceSource(promURL, nil)
		deps.ResourceLimitRange = metrics.NewPrometheusResourceLimitSource(promURL, nil)
		deps.MonthToDateBandwidth = metrics.NewMonthToDateBandwidthSource(promURL, nil)
		deps.MetricsFilterValues = metrics.NewPrometheusFilterValuesSource(promURL, nil)
		// Datastore metrics (w3/m10): PVC usage (kubelet, already scraped
		// cluster-wide) and CNPG's postgres_exporter (connections, replication
		// lag — deploy/gitops/base/prometheus.yaml's cnpg-tenant-db job).
		deps.DiskUsage = metrics.NewPrometheusDiskUsageSource(promURL, nil)
		deps.DBConnections = metrics.NewPrometheusDBConnectionsSource(promURL, nil)
		deps.ReplicationLag = metrics.NewPrometheusReplicationLagSource(promURL, nil)
		deps.KeyValueStats = metrics.NewPrometheusKeyValueStatsSource(promURL, nil)
	}
}

// wireAuthz installs the OpenFGA checker on base; the fail-closed posture a
// multi-tenant API requires is validated in loadConfig.
//
// Authorization (docs/ADR012-auth.md): unset => authz disabled (every verb allowed,
// the pre-m4 behavior); set => every verb checks OpenFGA, fail closed. NOT
// wired in stdio mode: that transport's trust boundary is the subprocess itself
// (no auth gate, so no identity — a wired checker would deny all).
func wireAuthz(base *core.Base, cfg *Config) core.Checker {
	var authzChecker core.Checker
	if cfg.OpenFGAURL != "" && !cfg.MCPStdio {
		authzChecker = authz.NewOpenFGAChecker(cfg.OpenFGAURL, cfg.OpenFGAToken)
		base.Authz = authzChecker
	}
	// w1/m53 + w1/m65 F16: the store-on/OpenFGA-off FAIL-CLOSED posture (and
	// its BEX_ALLOW_INSECURE_AUTHZ=1 single-member/local-dev override) is
	// enforced by loadConfig, before any side effect.
	return authzChecker
}

// openControlPlaneDB dials bex-db and converges its schema before anything
// serves. CNPG may still be coming up when the pod starts — wait for the DB
// rather than crash-looping. The caller owns closing the returned pool.
func openControlPlaneDB(ctx context.Context, cpDBURI string) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, cpDBURI)
	if err != nil {
		log.Fatalf("bex-api: db config: %v", err)
	}
	if err := waitForDB(ctx, pool); err != nil {
		log.Fatalf("bex-api: database unreachable: %v", err)
	}
	if err := store.Migrate(cpDBURI); err != nil {
		log.Fatalf("bex-api: %v", err)
	}
	if err := store.CheckOwnership(ctx, pool); err != nil {
		log.Fatalf("bex-api: %v", err)
	}
	return pool
}

// configureServerAuthOptions applies the OAuth discovery and webhook-key
// settings, warning loudly about the audience check that ships off.
func configureServerAuthOptions(srv *api.Server, cfg *Config) {
	// OAuth 2.1 discovery for MCP/agent clients (w4/m9, docs/ADR012-auth.md): the Hydra
	// public issuer + this API's canonical resource URI. Both unset => no
	// metadata endpoint, no audience check — behavior identical to before.
	srv.OAuthIssuer = cfg.OAuthIssuer
	srv.OAuthResource = cfg.OAuthResource
	// w1/m67 F1: narrow the empty-audience token exception to bex-provisioned
	// OAuth clients from the operator-owned registry — see docs/ADR012-auth.md §7.
	srv.OAuthRequireAudience = cfg.OAuthRequireAudience
	srv.OAuthPlatformClients = cfg.OAuthPlatformClients
	// w8/m27: BEX_OAUTH_API_SCOPE is retained for deployment compatibility but
	// is ignored as a second semantic matrix. Third-party human API-audience
	// tokens must carry the closed granular vocabulary (bex.read / bex.write /
	// bex.sensitive). bex.api remains a platform-client compatibility alias
	// only. See docs/ADR012-auth.md §7.
	srv.OAuthAPIScope = cfg.OAuthAPIScope
	// codex F6: an opt-in security control that ships off is invisible, and this
	// one stayed off through three remediation rounds while the fail-open posture
	// remained the deployed default. The narrowed audience check (auth.go) is
	// implemented; ENABLING it (BEX_OAUTH_REQUIRE_AUDIENCE=1) is an operator step
	// gated on BEX_OAUTH_PLATFORM_CLIENTS naming the official Render CLI and
	// bex-mobile IDs first — omitting them 401s their legitimately audience-less
	// device-flow logins. So this is a LOUD WARNING on every start, not a
	// fail-closed refusal: a hard refusal would either crashloop the API when
	// the flag is off, or force BEX_ALLOW_INSECURE_AUTHZ=1 (which would also
	// disable the OpenFGA fail-closed above). Track: docs/ADR055 F6 disposition.
	// (That warning is emitted by loadConfig with the other startup warnings.)
	srv.WebhookSecret = cfg.WebhookSecret
	// The GitHub App's app-wide webhook signs pushes with its own secret — a
	// second accepted key so installed repos redeploy hands-free
	// (docs/ADR026-github-integration.md).
	srv.GitHubWebhookSecret = cfg.GitHubWebhookSecret
	if err := validateWebhookReplayConfig(srv.WebhookSecret, srv.GitHubWebhookSecret, srv.WebhookReplays != nil); err != nil {
		log.Fatal(err)
	}
	// codex #4: in multitenant mode (control-plane store active), reject the shared
	// manual webhook secret because it carries no per-workspace binding and would
	// authorize cross-tenant deployment mutations. The GitHub App key is unaffected.
	srv.MultitenantWebhook = cfg.CPDBURI != ""
}

func validateWebhookReplayConfig(manualSecret, githubSecret string, durableReplays bool) error {
	if (manualSecret != "" || githubSecret != "") && !durableReplays {
		return errors.New("Git webhooks require BEX_CP_DB_URI for durable replay protection")
	}
	return nil
}

// wireGitHubApp wires the GitHub App integration (docs/ADR026-github-integration.md): private-repo deploys +
// zero-config push-to-deploy. Wired only when all three BEX_GITHUB_APP_* vars
// are set (and the key parses) — else the git-connect verbs 503. The store
// half (git_connections) is wired inside the BEX_CP_DB_URI block below.
func wireGitHubApp(deps *api.Deps, cfg *Config) *github.Client {
	var ghClient *github.Client
	if appID, key, slug := cfg.GitHubAppID, cfg.GitHubAppPrivateKey, cfg.GitHubAppSlug; appID != "" && key != "" && slug != "" {
		var err error
		ghClient, err = github.NewClient(github.Config{
			AppID: appID, PrivateKey: key, Slug: slug,
			// F2: both OAuth credentials enable the mandatory installation-admin
			// proof on new browser bindings. Both absent leaves existing connections
			// usable but makes every callback fail closed; a partial pair is fatal.
			ClientID:     cfg.GitHubAppClientID,
			ClientSecret: cfg.GitHubAppClientSecret,
		})
		if err != nil {
			log.Fatalf("bex-api: github app config: %v", err)
		}
		deps.GitHubClient = ghClient
		if ghClient.InstallVerificationConfigured() {
			deps.GitHubInstallVerifier = ghClient
		} else {
			// ADR075 §7: the half-configured state (app keys present, OAuth pair
			// absent) is a deployment mistake, never meaningful — every new
			// binding would fail closed at the callback. Production ran this way
			// undetected because the Secret simply lacked the two keys and the
			// manifest's optional env refs made it silent. Say so loudly; the
			// connect/claim start verbs also refuse up front.
			log.Printf("bex-api: WARNING: GitHub App is configured but BEX_GITHUB_APP_CLIENT_ID/BEX_GITHUB_APP_CLIENT_SECRET are not — every new GitHub connection will be refused until they are set")
		}
		// The same out-of-band private key also HMAC-signs the short-lived
		// workspace state carried through GitHub's browser install redirect.
		deps.GitHubStateSecret = []byte(key)
	}
	return ghClient
}

// wireControlPlaneFeatures wires the control-plane store into the feature deps
// and adapts the authz checker's role grant/revoke sides, returning the
// membership granter shared by the tenant service and the internal CP API.
func wireControlPlaneFeatures(cfg *Config, deps *api.Deps, base *core.Base, st *store.PGStore, rec *store.Reconciler, authzChecker core.Checker) store.MembershipGranter {
	deps.Store = st // single writer of intent: suspend/resume write the row first
	deps.OAuthRevocations = st
	deps.AccountStore = st
	deps.SSHKeysStore = st // identity-scoped SSH public-key registry
	deps.DeployStore = st  // deploy history (w2/m5): list/get/trigger read+write the same rows
	// Cancel (w2/m10) needs to compute a repo-backed App's in-flight build
	// Job's identity — must match the operator's own BEX_BUILD_NAMESPACE.
	deps.DeployBuildNamespace = cfg.BuildNamespace
	deps.GitHubStore = st        // git connections (w2/m8): connect/disconnect/list read+write git_connections
	deps.EventStore = st         // service events: deploy + audit + typed observed/Git facts
	deps.EventFacts = st         // typed observed/Git event facts (w3/m19), written outside the operator
	deps.NotificationsStore = st // deploy notifications (w3/m9): settings read/write + the reconciler's recipient fan-out
	deps.ProjectsStore = st      // project groupings (w1/m31): project CRUD + service-assignment
	deps.EnvironmentsStore = st  // environment groupings (layered on w1/m31): environment CRUD + service-assignment
	deps.RegistryCredsStore = st // registry credentials (w2/m14): CRUD metadata rows; secrets live in OpenBao (deps.Secrets)
	deps.BlueprintsStore = st    // blueprint registry (w2/m15): auto-upserted on deploy, list+sync read it
	deps.WebhookStore = st       // outbound webhooks (w3/m11): endpoint CRUD + delivery history; the worker below delivers
	deps.JobStore = st           // one-off jobs (Render's /services/{id}/jobs): job CRUD + k8s Job tracking
	deps.AgentSessionStore = st  // ADR047 D3: durable agent-session lifecycle
	if writer, ok := authzChecker.(agentsessions.TupleWriter); ok {
		deps.AgentSessionTuples = writer
	}

	// Audit log (w4/m10): *store.PGStore structurally satisfies
	// core.AuditSink, so every write verb's Authorize/AuthorizeOn call
	// starts recording the instant the store is wired — no extra plumbing.
	base.Audit = st
	base.Billing = st

	// Workspace lifecycle (w6/m1): the workspaces feature writes through the
	// same store and nudges the same projector to prune a deleted workspace's
	// App CRs. The OpenFGA checker (when wired) is both the grant and revoke
	// side, keeping workspace:tea-<id> tuples in step with tenant_members.
	deps.WorkspaceStore = st
	deps.WorkspaceCreationStore = st
	deps.WorkspaceKick = rec.Kick
	if g, ok := authzChecker.(workspaces.WorkspaceGranter); ok {
		deps.WorkspaceGranter = g
	}
	if rv, ok := authzChecker.(workspaces.WorkspaceRevoker); ok {
		deps.WorkspaceRevoker = rv
	}
	// Out-of-cascade teardown (w6/m4/t005, w1/m61): a deleted workspace's
	// OpenBao secrets, env groups, managed Databases/KeyValue stores, sandboxes
	// (appended in the OpenSandbox block below), and Stripe subscription
	// (appended in the billing block below) live outside the tenant row's FK
	// cascade. Delete runs these PRE-cascade — while the tenant row still
	// exists — so a purger failure is retryable (the row and its confirmation
	// phrase survive) and the sandbox/Stripe purgers can still read the ids the
	// cascade is about to drop. The secrets purger must also run here, before
	// the App CRs are torn down: it enumerates them to find their OpenBao paths.
	deps.WorkspacePreCascadePurgers = []workspaces.WorkspacePurger{
		&secrets.WorkspacePurger{Service: &secrets.Service{Base: base, Store: deps.Secrets}},
		&envgroups.WorkspacePurger{Service: &envgroups.Service{Base: base, Store: deps.Secrets}},
		&registrycreds.WorkspacePurger{Service: &registrycreds.Service{Base: base, Store: st, Secret: deps.Secrets}},
		&postgres.WorkspacePurger{Service: &postgres.Service{Base: base}},
		&keyvalue.WorkspacePurger{Service: &keyvalue.Service{Base: base}},
	}
	// apps.WorkspacePurger (w6/m11, live-verification finding) runs POST-cascade
	// (w1/m61): an App created through the public REST/GraphQL/MCP surface
	// carries core.LabelTenant only, never store.LabelManagedBy, so the
	// row-backed cascade + reconciler prune that tears down *row-backed* Apps on
	// workspace delete never sees it — it would otherwise survive forever, still
	// running and permanently unreachable (its tenant is gone, so core.Base's
	// tenant gate forbids everyone, including its creator). It must run AFTER the
	// row cascade: purging App CRs while their apps rows still exist would let
	// the projector immediately re-create them. Redundant-but-harmless for
	// row-backed Apps the reconciler already pruned (delete of an already-gone
	// object is a no-op).
	deps.WorkspacePostCascadePurgers = []workspaces.WorkspacePurger{
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
	// Login-time invite redemptions record members.AcceptInvite audit rows
	// through the same store the feature verbs' sink writes (w1/m33).
	tenantSvc.Audit = st
	// w1/m53 + w1/m65 F13: gate login-time invite redemption on a Kratos-verified
	// email so an attacker can't register with a victim's not-yet-signed-up
	// invited address and claim the invite. Secure by DEFAULT now (default true):
	// email is an authorization key here, so ownership must be proven. The
	// emailed ?invite=<token> bearer link still redeems regardless, so an
	// address that genuinely can't be verified retains an explicit path. Set
	// BEX_REQUIRE_VERIFIED_INVITE_EMAIL=0 only for local dev without a
	// verification UX (docs/ADR024-members.md).
	tenantSvc.RequireVerifiedInviteEmail = cfg.RequireVerifiedInviteEmail
	base.Workspace = tenantSvc
	deps.Onboard = tenantSvc
	deps.KeyBinder = tenantSvc
	return granter
}

// wireStripeBilling wires Stripe Billing (w7/m47–m50, ADR040): one Stripe
// client drives both the seal-then-emit meter-event sidecar and the usage
// surface's invoice read-back. BEX_STRIPE_SECRET_KEY unset means no client,
// emitter, reader, or public Stripe webhook: estimate-only behavior stays
// unchanged.
func wireStripeBilling(ctx context.Context, cfg *Config, deps *api.Deps, base *core.Base, cl client.Client, st *store.PGStore, appsNS string, usageSvc *usage.Service, billingMetrics *billing.Metrics) (*billing.Worker, *billing.Reconciler, store.BillingAdmin) {
	var stripeLifecycleWorker *billing.Worker
	var stripeLifecycleReconciler *billing.Reconciler
	var stripeBillingAdmin store.BillingAdmin
	requirePaymentMethod := cfg.RequirePaymentMethod
	stripeSecretKey, billingEpoch, stripeEnabled := cfg.StripeSecretKey, cfg.StripeEpoch, cfg.StripeEnabled
	if stripeEnabled {
		publishableKey := cfg.StripePublishableKey
		billingMetrics.SetEnabled(true)
		stripeClient := billing.NewStripe(billing.StripeConfig{
			SecretKey:             stripeSecretKey,
			PublishableKey:        publishableKey,
			BaseURL:               cfg.StripeAPIURL,
			BillingEpoch:          billingEpoch,
			CompCouponID:          cfg.StripeCompCouponID,
			DashboardURL:          cfg.DashboardURL,
			PortalConfigurationID: cfg.StripePortalConfigurationID,
			TaxCode:               cfg.StripeTaxCode,
			TaxBehavior:           cfg.StripeTaxBehavior,
			State:                 st,
			Metrics:               billingMetrics,
		})
		usageSvc.Billing = stripeClient
		deps.Billing = stripeClient
		deps.BillingState = st
		if publishableKey != "" {
			deps.WorkspaceCreationBilling = stripeClient
			go (&billing.WorkspaceCreationCleaner{Store: st, Provider: stripeClient, Metrics: billingMetrics}).Run(ctx)
		}
		if requirePaymentMethod != paymentMethodOff {
			base.Payment = &billing.PaymentGate{Store: st}
			// ADR075 D7: "all" widens RequirePlanBilling to the free tier too.
			base.PaymentAllPlans = requirePaymentMethod == paymentMethodAllPlans
		}
		stripeBillingAdmin = &billing.Admin{Store: st, Provider: stripeClient}
		// Workspace-delete Stripe teardown (w1/m61): cancel the workspace's
		// metered Subscription when its workspace is deleted (keeping the Customer
		// for invoice history). Pre-cascade, so the billing_provider_mappings row
		// still resolves the subscription id. Only wired when Stripe is enabled —
		// byte-identical to before m61 otherwise.
		deps.WorkspacePreCascadePurgers = append(deps.WorkspacePreCascadePurgers,
			&billing.WorkspacePurger{Canceller: stripeClient})

		emitter := billing.NewEmitter(st, stripeClient)
		emitter.Metrics = billingMetrics
		emitter.Epoch = billingEpoch
		emitter.RequirePaymentMethod = requirePaymentMethod != paymentMethodOff
		if cfg.StripeSealHoursSet {
			emitter.SealHours = time.Duration(cfg.StripeSealHours) * time.Hour
		}
		// w7/m57: never seal shorter than the usage rollup's catch-up window, or
		// an exported row could still be rewritten and never re-emitted.
		if clamped := billing.ClampSealHours(emitter.SealHours, usage.CatchupWindow); clamped != emitter.SealHours {
			log.Printf("BEX_STRIPE_SEAL_HOURS=%s is below the usage catch-up window; raising the seal horizon to %s so exported rows are final", emitter.SealHours, clamped)
			emitter.SealHours = clamped
		}
		var lifecycle *billing.Lifecycle
		if cfg.StripeDunningEnabled {
			// The mode threads through so webhook events from the other mode
			// are rejected; BEX_STRIPE_ALLOW_LIVE at the installer remains the
			// single deliberate live gate. (dunningGate itself ran in
			// loadConfig.)
			grace := cfg.StripeGracePeriod
			reconcileEvery := cfg.StripeReconcileInterval
			lifecycle = &billing.Lifecycle{Store: st, GracePeriod: grace, ExpectedLivemode: stripeClient.ExpectedLivemode()}
			// ADR088 §4: dunning must never suspend the pinned ops workspace.
			enforcer := &billing.KubernetesEnforcer{Client: cl, Store: st, Namespace: appsNS, OpsWorkspaceID: cfg.OpsWorkspace}
			stripeLifecycleWorker = &billing.Worker{Store: st, Enforcer: enforcer}
			stripeLifecycleReconciler = &billing.Reconciler{Store: st, Provider: stripeClient, GracePeriod: grace, Interval: reconcileEvery, Metrics: billingMetrics, ExpectedLivemode: stripeClient.ExpectedLivemode()}
			log.Printf("bex-api Stripe dunning enabled (livemode %t, grace %s, reconcile %s)", stripeClient.ExpectedLivemode(), grace, reconcileEvery)
		}
		if secret := cfg.StripeWebhookSecret; secret != "" {
			handler := &billing.StripeWebhook{Secret: secret, ExpectedLivemode: stripeClient.ExpectedLivemode(), OnCheckoutCompleted: stripeClient.CompleteCheckoutSession, Metrics: billingMetrics}
			if lifecycle != nil {
				handler.OnLifecycle = lifecycle.HandleStripeEvent
			}
			deps.StripeWebhook = handler
		}
		log.Printf("bex-api Stripe Billing enabled (seal horizon %s, epoch %s, webhook %t)", emitter.SealHours, billingEpoch.Format(time.RFC3339), deps.StripeWebhook != nil)
		go emitter.Run(ctx)
	}
	return stripeLifecycleWorker, stripeLifecycleReconciler, stripeBillingAdmin
}

// startControlPlaneServer starts the internal control-plane API (:8091): it
// grants workspace-admin and cross-tenant writes, so it must never serve
// unauthenticated. Fail closed at startup when BEX_CP_TOKEN is empty (w1/m53:
// the token was set nowhere in prod, so the API had been serving open behind
// the NetworkPolicy alone). BEX_CP_INSECURE=1 is a loud local-dev override.
func startControlPlaneServer(ctx context.Context, cfg *Config, st *store.PGStore, rec *store.Reconciler, granter store.MembershipGranter, stripeBillingAdmin store.BillingAdmin, metricRegistry *prometheus.Registry, agentMetrics *agentsessions.CompletionMetrics, ghClient *github.Client, modelKeys core.SecretKV, opsRole *opsrole.Handler, ready *serve.Readiness) {
	// requireCPAuth ran in loadConfig — before migrations — so an empty
	// BEX_CP_TOKEN (without the loud BEX_CP_INSECURE=1 local-dev override)
	// never reaches this point.
	cpToken := cfg.CPToken
	internal := &store.API{Store: st, Kick: rec.Kick, Health: st.Ping, Token: cpToken, Grant: granter, Billing: stripeBillingAdmin, BillingOperations: st, SandboxTenants: st}
	internalRoot := http.NewServeMux()
	internalRoot.Handle("GET /metrics", promhttp.HandlerFor(metricRegistry, promhttp.HandlerOpts{}))
	// ADR047 D2: a gateway-authenticated, internal-only mint verb. The same
	// sandbox-exec HMAC secret is reused with protocol domain separation; the
	// route is not mounted on :8090 and never enters the public surface.
	if cfg.SandboxExecSecret != "" {
		internalRoot.Handle(agentsession.InternalMintPath, &agentsession.Handler{
			Secret: []byte(cfg.SandboxExecSecret),
			Minter: &agentsession.Minter{GitHub: ghClient, Connections: st, Sessions: st, Audit: st},
			Nonce:  st,
		})
		// ADR062: the model-credential mint. Same gateway-only HMAC + internal-only
		// listener as the Git mint, path-domain-separated. Wired only when OpenBao is
		// reachable (modelKeys non-nil), so a deployment without the BYO key store
		// simply 503s the model proxy and never falls back to sandbox key injection.
		if modelKeys != nil {
			internalRoot.Handle(agentsession.InternalModelMintPath, &agentsession.ModelHandler{
				Secret: []byte(cfg.SandboxExecSecret),
				Minter: &agentsession.ModelMinter{Keys: modelKeys, Sessions: st, Audit: st},
				Nonce:  st,
			})
			// w5/m80 t003: the gateway reports a vendor auth rejection (401/403 on the
			// gateway→vendor hop) here so bex-api terminalizes the session fast instead
			// of the agent CLI burning its full retry/backoff. Same gateway-only HMAC +
			// internal-only listener as the mint; wired even when hibernation/keys vary,
			// since it only needs the store to CAS a live session to failed.
			internalRoot.Handle(agentsession.InternalModelAuthFailurePath, &agentsession.ModelAuthFailureHandler{
				Secret: []byte(cfg.SandboxExecSecret),
				Failer: &agentsession.ModelAuthFailer{
					Sessions: st, Audit: st,
					ObserveTerminal: agentMetrics.ObserveVendorAuthRejected,
				},
				Nonce: st,
			})
		}
	}
	// ADR088 §4: the ops-role verb mounts ONLY here, on the cluster-internal
	// listener — never the public :8090 mux (api.bex.co routes the whole `/`
	// prefix straight to :8090, which would leave the static bearer as the
	// route's only protection). Register is a no-op unless BEX_OPS_WORKSPACE
	// and BEX_OPS_ROLE_TOKEN are both set (opsRole is then nil).
	opsrole.Register(internalRoot, opsRole)
	internalRoot.Handle("/", internal.Handler())
	cpAddr := cfg.CPAddr
	cpSrv := newHTTPServer(cpAddr, internalRoot)
	log.Printf("bex-api control plane (source of truth) on %s (Apps project into per-tenant <ws> namespaces; Databases/KeyValues stay in %q)", cpAddr, cfg.CPAppsNamespace)
	go func() {
		// Same serve-then-graceful-shutdown pattern as the public server
		// in main: on SIGTERM (ctx cancelled) the internal API drains instead
		// of being cut mid-request. It shares the pod's drain window — the
		// readiness flip removes BOTH ports from the Service endpoints, so
		// :8091 keeps serving late-routed internal calls through the same
		// window. Its drain is best-effort — main returns once the public
		// server finishes draining, so a slower CP drain is cut short at
		// process exit; acceptable for an internal-only API.
		if err := serve.UntilShutdown(ctx, cpSrv, serve.Options{Readiness: ready, DrainWindow: drainWindow}); err != nil {
			log.Fatalf("bex-api control plane: %v", err)
		}
	}()
}

// opsRoleHandler builds the ADR088 §4 ops-role verb when BOTH
// BEX_OPS_WORKSPACE and BEX_OPS_ROLE_TOKEN are configured; nil otherwise, so
// the route is never mounted and answers the internal mux's normal 404. The
// verb reads roles from OpenFGA and identity traits from the same Kratos admin
// reader the owners/members surface uses; the handler fails closed (503) when
// either backend is unwired or unreachable.
func opsRoleHandler(cfg *Config, authzChecker core.Checker, identities workspaces.IdentityReader) *opsrole.Handler {
	if cfg.OpsWorkspace == "" || cfg.OpsRoleToken == "" {
		return nil
	}
	h := &opsrole.Handler{Workspace: cfg.OpsWorkspace, Token: cfg.OpsRoleToken, Authz: authzChecker}
	if identities != nil {
		h.Identity = func(ctx context.Context, subject string) (string, string, bool) {
			attrs, ok := identities.Lookup(ctx, subject)
			return attrs.Email, attrs.Name, ok
		}
	}
	return h
}

// startOpsRoleServer starts the cluster-internal listener with ONLY the ADR088
// ops-role verb mounted — the control-plane-less shape (local dev / e2e
// without BEX_CP_DB_URI). With the control plane on, the verb instead shares
// the :8091 mux (startControlPlaneServer); either way it never touches the
// public :8090 surface.
func startOpsRoleServer(ctx context.Context, cfg *Config, h *opsrole.Handler, ready *serve.Readiness) {
	// loadConfig parses BEX_CP_ADDR under its own copy of the "internal
	// listener will run" predicate; if the two ever drift, an empty addr here
	// would make net/http bind ":http" (port 80) — a silent misbind on a
	// privileged port. Refuse loudly instead.
	if cfg.CPAddr == "" {
		log.Fatalf("bex-api ops-role listener: BEX_CP_ADDR empty (loadConfig's listener predicate drifted from startup's)")
	}
	internalRoot := http.NewServeMux()
	opsrole.Register(internalRoot, h)
	srv := newHTTPServer(cfg.CPAddr, internalRoot)
	log.Printf("bex-api internal ops-role verb on %s (control plane off)", cfg.CPAddr)
	go func() {
		if err := serve.UntilShutdown(ctx, srv, serve.Options{Readiness: ready, DrainWindow: drainWindow}); err != nil {
			log.Fatalf("bex-api ops-role listener: %v", err)
		}
	}()
}

// wireSandboxes wires hosted agent sandboxes (pillar 5, ADR042/w3/m32): the
// OpenSandbox lifecycle client only when BEX_OPENSANDBOX_URL is set — unset =>
// the sandbox verbs 503 and the feature is not registered (byte-identical). The
// per-workspace tenant-key provider (m32 t006) and template registry are
// wired as they land; for now a single default "base" template.
func wireSandboxes(ctx context.Context, cfg *Config, deps *api.Deps, cl client.Client, st *store.PGStore) {
	if cfg.OpenSandboxURL != "" {
		deps.SandboxClient = sandbox.NewClient(cfg.OpenSandboxURL)
		// Both defaults are digest-pinned (w7/m85): a sandbox template names an
		// image bex RUNS, so a floating tag let two creates months apart start
		// different code with no record of the change. The tag is retained ahead
		// of the digest for legibility; the digest is what resolves.
		// BEX_AGENT_SESSION_IMAGE is normally supplied by
		// lego/operator/config/api/deployment.yaml, which deploy.yml rewrites to
		// the digest it just pushed — this default is the floor for a deployment
		// that omits it, not the production value.
		deps.SandboxTemplates = sandboxTemplateRegistry(cfg.SandboxImage, cfg.AgentSessionImage)
		deps.SandboxDefaultPlan = sandbox.PlanStarter
		// The Render CLI's `ea sandbox create` sends no template (no such flag), so
		// an empty template resolves to this registered default (w3/m32 t009).
		deps.SandboxDefaultTemplate = "base"
		// Agent-session egress (ADR047 D5): policy is installed before sandbox
		// creation. The registry catalog is platform config; the model endpoint is
		// selected per session by its agent/provider config and validated at create.
		egress, err := sessionegress.NewManager(cl, sessionegress.Config{
			SetupRegistryDomains: sessionegress.RegistryConfig(cfg.AgentSetupRegistries),
			// ADR062: when the model proxy is on, narrow the session policy to admit
			// the gateway proxy port instead of the vendor host. The port is derived
			// from the same deps.AgentModelProxyURL the agentsessions Service uses, so
			// the two can never disagree on which port to open.
			ModelProxyPort:       cfg.ModelProxyPort,
			SnapshotStoreDomains: cfg.SnapshotEgressDomains,
		})
		if err != nil {
			log.Fatalf("bex-api: %v", err)
		}
		deps.SandboxSessionEgress = egress
		// `render ea sandbox exec` (m33): bex-api authorizes can_operate and mints a
		// signed ticket, then reverse-proxies the SSE stream from the isolated SSH
		// gateway (which alone holds pods/exec, Option A). Both the shared HMAC
		// secret and the gateway's internal exec URL must be set, else the exec verb
		// 503s (create/list/stop are unaffected).
		if cfg.SandboxExecSecret != "" {
			if gwURL := cfg.SandboxExecURL; gwURL != "" {
				deps.SandboxExec = &sandbox.ExecConfig{
					Secret:     []byte(cfg.SandboxExecSecret),
					GatewayURL: gwURL,
					Client:     &http.Client{}, // no timeout: the exec stream is long-lived
					TTL:        60 * time.Second,
				}
			}
		}
		// Multi-tenant OpenSandbox (m32 t006): with the control plane enabled, each
		// workspace gets an opaque tenant key the sandbox feature stamps as the
		// OPEN-SANDBOX-API-KEY header; the server resolves it back through the CP
		// tenant-lookup endpoint to the `<ws>-sandbox` namespace. Without the store
		// (st nil), Keys stays nil and OpenSandbox must run single-tenant.
		if st != nil {
			deps.SandboxKeys = sandboxKeyProvider{st}
			deps.SandboxMeter = &sandbox.Meter{Client: deps.SandboxClient, Store: st}
			go deps.SandboxMeter.Run(ctx)
			// Workspace-delete sandbox teardown (w1/m61): stop the workspace's
			// running OpenSandbox sandboxes before the tenant row (and its sandbox
			// key, migration 0056) cascade away. It joins the PRE-cascade purgers so
			// the key still resolves; st satisfies sandbox.PurgeKeyLookup (lookup-only,
			// never minting). Only meaningful with the store on (workspace delete
			// needs it) and the client wired — both true in this branch.
			deps.WorkspacePreCascadePurgers = append(deps.WorkspacePreCascadePurgers,
				&sandbox.WorkspacePurger{Client: deps.SandboxClient, Keys: st})
		}
	}
}

// wireReconcilers wires the apps.Service ↔ reconciler seams and starts the
// projector + namespace reconciler loops.
// wireAgentSessions reads the agent-session and Browser Web Shell environment
// contract onto deps: the shared gateway trust key, the browser-reachable
// origins, the Active-tier lifecycle bounds, and the ADR059 hibernation store.
// wireDiskSnapshots points bex-api at the same bucket the operator writes disk
// snapshots to (docs/ADR082-persistent-disks.md D5) and gives it the key that
// signs the 24-hour handles a listing hands out. Unset ⇒ the two snapshot verbs
// report unavailable and disks otherwise work exactly as before.
//
// The signing key is deliberately the shell-ticket secret's sibling rather than
// the age keypair: this signs a REFERENCE to an object, never its contents, so
// it has nothing to do with the key that decrypts a snapshot — which stays in
// the cluster with the restore Job and never reaches bex-api.
func wireDiskSnapshots(deps *api.Deps, cfg *Config) {
	deps.DiskSnapshots = apps.NewS3DiskSnapshotLister(cfg.DiskSnapshot)
	if cfg.ShellTicketSecret != "" {
		deps.DiskSnapshotSecret = []byte(cfg.ShellTicketSecret)
	}
}

func wireAgentSessions(deps *api.Deps, cfg *Config) {
	// Browser Web Shell (docs/ADR035-ssh.md § Browser Web Shell): the HMAC key
	// shared only with the isolated gateway and the browser-reachable gateway
	// WebSocket origin. Either unset => the ticket verb returns 503 and native
	// `ssh` is unaffected.
	if secret := cfg.ShellTicketSecret; secret != "" {
		deps.ShellTicketSecret = []byte(secret)
		// Agent attach deliberately reuses the Browser Web Shell trust key and
		// DB-backed nonce design. The claims are a distinct ticket type and bind
		// the session sandbox pod/workspace instead of a service instance.
		deps.AgentSessionTicketSecret = []byte(secret)
		if deps.SandboxExec != nil {
			deps.SandboxExec.DriverGrantSecret = []byte(secret)
		}
	}
	deps.ShellWSURL = cfg.ShellWSURL
	deps.AgentSessionGatewayURL = cfg.AgentSessionGatewayURL
	// Optional override of the in-cluster Git smart-HTTP proxy origin. The
	// sandbox receives this non-secret URL, never a GitHub credential.
	deps.AgentGitProxyURL = cfg.AgentGitProxyURL
	// Active-tier sandbox lifecycle (ADR059 D2/D6, w2/m67): idle grace before a
	// finished session's sandbox is reaped (default 30m; 0 ⇒ ADR054 D6 immediate
	// reap), and the per-workspace concurrent live-sandbox cap (default 5; 0 ⇒
	// uncapped).
	deps.AgentSandboxIdleTTL = cfg.AgentSandboxIdleTTL
	// w5/m80 t002: one turn's wall-clock bound, injected into the sandbox as
	// BEX_AGENT_TURN_TIMEOUT_MS. Default 30m; a 0/invalid value falls back to the
	// Service's 30m default (the bound is never disabled — that is the 4h-hang bug
	// this fixes).
	deps.AgentTurnTimeout = cfg.AgentTurnTimeout
	deps.AgentMaxLiveSandboxesPerWorkspace = cfg.AgentMaxLiveSandboxesPerWorkspace
	deps.MaxBlueprintGroupings = cfg.MaxBlueprintGroupings
	// Round-11 #3: per-workspace env-group quota (default 100; 0 disables).
	deps.MaxEnvGroupsPerWorkspace = cfg.MaxEnvGroupsPerWorkspace
	// ADR075 §2: per-workspace GitHub-connection quota (default 10; 0 disables).
	deps.MaxGitConnectionsPerWorkspace = cfg.MaxGitConnectionsPerWorkspace
	// codex-security geyRc8 F1: per-workspace registry-credential quota.
	deps.MaxRegistryCredentialsPerWorkspace = cfg.MaxRegistryCredentialsPerWorkspace
	// codex-security round 18: custom-domain cardinality quotas (default 100
	// per service — the round-12 #3 routes/headers scale — and 500 per
	// workspace; 0 disables). Beyond either, claims are refused with
	// CUSTOM_DOMAIN_LIMIT.
	deps.MaxCustomDomainsPerService = cfg.MaxCustomDomainsPerService
	deps.MaxCustomDomainsPerWorkspace = cfg.MaxCustomDomainsPerWorkspace
	// ADR059 D3/D5 hibernation (w2/m68, armed w2/m77): the object store enables
	// the Hibernated tier (reclaim → snapshot, resume → rehydrate). All four
	// required coordinates unset ⇒ the whole tier is off and reclaim stays
	// Terminate (byte-identical to w2/m67). A partial set is fatal — a typo'd
	// Secret key must not silently disable hibernation. Unset/delete the
	// bex-agent-snapshot Secret to roll back.
	store, err := agentsessions.NewS3SnapshotStore(cfg.AgentSnapshot)
	if err != nil {
		// loadConfig validated this config before any side effect; a failure
		// here means the two disagree, which is a bug.
		log.Fatalf("bex-api: agent-session hibernation config: %v", err)
	}
	if store != nil {
		deps.AgentSnapshotStore = store
		log.Printf("bex-api: agent-session hibernation enabled (object store %s)", cfg.AgentSnapshot.Bucket)
	}
	deps.AgentSnapshotRetentionTTL = cfg.AgentSnapshotRetentionTTL
	deps.AgentMaxPinnedSandboxesPerWorkspace = cfg.AgentMaxPinnedSandboxesPerWorkspace
}

func wireReconcilers(ctx context.Context, srv *api.Server, rec *store.Reconciler, st *store.PGStore, cl client.Client, base *core.Base, cfg *Config) {
	// Wire the reconciler ↔ apps.Service now that both exist (w2/m11):
	// - CloneSecrets: the projector mints clone Secrets for private-repo rows
	//   created via the internal CP API (store/api.go POST /v1/apps).
	// - Kick on the apps.Service: after a store-managed create/redeploy the
	//   projector runs immediately instead of waiting the next resync period.
	// rec.Run is started here — after the wiring — so CloneSecrets is already
	// set before the first reconcile pass runs.
	// Wire the secrets purger into the apps service so individual service deletes
	// purge OpenBao env-var and secret-file paths (w7/m12). Uses the same
	// WorkspacePurger that workspace delete already uses — just the per-app method.
	srv.Apps.SecretsEraser = &secrets.WorkspacePurger{Service: srv.Secrets}

	if rec != nil {
		rec.CloneSecrets = srv.Apps.ReconcilerCloneSecreter()
		srv.Apps.Kick = rec.Kick
		// DeployNotifier (w3/m9): srv.Notifications structurally satisfies
		// store.DeployNotifier (NotifyDeploy), so every deploy the reconciler
		// closes as succeeded/failed fans out to the workspace's members —
		// same wiring shape as CloneSecrets, deferred until here so it's set
		// before the first reconcile pass.
		rec.DeployNotifier = srv.Notifications

		// Per-tenant namespace isolation (ADR043): whenever the store is wired the
		// control plane also provisions each workspace's `<ws>` and `<ws>-sandbox`
		// namespaces with base ResourceQuota/LimitRange/default-deny NetworkPolicy,
		// and prunes them for deleted workspaces. App CRs project into `<ws>`.
		// BEX_CP_IDENTITY scopes both cluster-scoped prunes to this control-plane
		// instance; unset => "production" (docs/ADR043 D9). Fatal on a malformed
		// value: it is projected into a label, and an invalid one would fail every
		// namespace apply while ReconcileOnce collects those errors per-workspace
		// without ever exiting — so the process would come up healthy, provision
		// nothing, and keep pruning.
		// (Validated as a Kubernetes label value in loadConfig, before any
		// side effect.)
		cpIdentity := cfg.CPIdentity
		nsRec := store.NewNamespaceReconciler(cl, st)
		nsRec.Identity = cpIdentity
		rec.Identity = cpIdentity
		// w2/026: the reconciler only learns about a freshly minted workspace on
		// its next resync (nothing kicks it on mint), so a first create within
		// that window used to 500 on a namespace NotFound. The create paths now
		// ensure the workspace's namespaces synchronously when they are missing.
		base.EnsureNamespaces = nsRec.EnsureWorkspace
		// Kick BOTH reconcilers on workspace create/delete for the same
		// low-latency reason the projector is kicked on app writes: the
		// projector prunes the deleted workspace's orphaned App CRs, the
		// namespace reconciler provisions/prunes its `<ws>` namespace(s).
		srv.Workspaces.Kick = func() {
			rec.Kick()
			nsRec.Kick()
		}
		go rec.Run(ctx)
		go nsRec.Run(ctx)
	}
}

// startDeliveryWorkers starts the outbound-webhook delivery worker and the
// native-push consumer; each runs only when its wiring is present.
func startDeliveryWorkers(ctx context.Context, cfg *Config, srv *api.Server, st *store.PGStore, m members.Mailer, mobilePush pushtransport.Transport, webPush *pushtransport.WebPush, pushMetrics *notifications.PushMetrics, webhookMetrics *webhooks.Metrics) {
	// Outbound event webhooks (w3/m11): the delivery worker tails the composed
	// event feed (deploys + audit_events + service_event_facts — the same rows the events feed reads)
	// through a durable watermark and POSTs signed notifications to subscribed
	// endpoints, with retry/auto-disable. Store off => no worker (the CRUD verbs
	// already 503 via deps.WebhookStore above). Failure notices reuse the SMTP
	// relay + Kratos email lookup the notifications feature uses.
	// BEX_WEBHOOK_BACKOFF ("5s,10s,1m") overrides the documented retry schedule
	// — a dev/verification knob, unset in production.
	if st != nil {
		backoff := cfg.WebhookBackoff
		// Terminal deliveries are purged on an age + per-endpoint-count policy
		// (w1/m67 F3): webhook_deliveries doubles as the dashboard's history view,
		// so without retention a tenant's ordinary activity grew shared storage
		// forever. 0 keeps the documented defaults.
		whRetentionDays := cfg.WebhookRetention
		whKeepPerEndpoint := cfg.WebhookKeep
		whMaxDeliveriesPerWorkspace := cfg.MaxWebhookDeliveriesPerWorkspace
		whWorker := &webhooks.Worker{
			Store: st, Mailer: m, Emails: srv.Notifications.Identities, Backoff: backoff,
			RetentionDays: whRetentionDays, RetentionKeepPerEndpoint: whKeepPerEndpoint,
			Attempts: webhookMetrics, Admissions: webhookMetrics,
			MaxDeliveriesPerWorkspace: whMaxDeliveriesPerWorkspace,
		}
		go whWorker.Run(ctx)
	}
	// Native push (ADR048 D2): only an explicitly configured, startup-validated
	// transport starts the event consumer. With config absent there is no client,
	// no worker, no feed advancement, and no provider network traffic.
	if st != nil && (mobilePush != nil || webPush != nil) {
		pushWorker := &notifications.PushWorker{
			Store: st, Sender: notifications.PushTransportSender{Transport: mobilePush, WebPush: webPush},
			Receipts: mobilePush, Metrics: pushMetrics, Evidence: notifications.PushEvidenceLogger{},
		}
		go pushWorker.Run(ctx)
	}
}

// configureRateLimiters wires the trusted-proxy-aware rate limiters and the
// pre-auth admission budget onto the server.
func configureRateLimiters(srv *api.Server, cfg *Config) {
	// Trusted-proxy CIDRs for rate-limit identity (w4/m33 P2 register,
	// .pm/w4/029.md report #10). In production every public request's TCP peer
	// is a Traefik pod, so without this every IP-keyed limiter below keys all
	// anonymous Internet clients into ONE shared bucket (the device flow's
	// 30/min held platform-wide breaks `render login` for everyone). Set ⇒ a
	// limiter derives the client IP from X-Forwarded-For/X-Real-IP only when
	// the immediate peer is inside one of these CIDRs; unset ⇒ peer IP only,
	// headers ignored (byte-identical to before). Malformed ⇒ fail closed.
	trustedProxies := cfg.TrustedProxies

	// Rate limiting + request caps (w7/m3). BEX_RATE_LIMIT=0 disables the limiter.
	rpm, burst := cfg.RateLimitRPM, cfg.RateLimitBurst
	srv.RateLimiter = api.NewRateLimiter(rpm, burst) // nil when rpm=0 (disabled)
	if srv.RateLimiter != nil && srv.APIKeys != nil && srv.APIKeys.Base != nil {
		srv.RateLimiter.Workspace = srv.APIKeys.Base.Workspace
	}

	// Device-flow rate limiting (w4/m31/t002). BEX_DEVICE_RATE_LIMIT=0
	// disables it. Set here, before Handler(), like every other cap: Handler()
	// wires this onto the cliauth.Service it constructs internally (it can't
	// be built any earlier, since it needs the auth gate's invalidate
	// callback, which is also Handler()-scoped) — but the limiter itself only
	// needs the env-derived rpm/burst, so there's no reason its configuration
	// should wait until after Handler() the way its consumer's construction
	// must.
	deviceRPM, deviceBurst := cfg.DeviceRateRPM, cfg.DeviceRateBurst
	srv.DeviceRateLimiter = cliauth.NewDeviceRateLimiter(deviceRPM, deviceBurst)

	// Webhook intake rate limiting (w7/m60). The two unauthenticated intakes
	// (POST /v1/webhooks/git, /v1/webhooks/stripe) mount outside the auth gate, so
	// neither BEX_RATE_LIMIT nor BEX_DEVICE_RATE_LIMIT reaches them; this IP-keyed
	// limiter sheds a flood with 429 before the body read + HMAC verification.
	// Default 600/min per IP is deliberately generous: no legitimate GitHub/Stripe
	// delivery pattern hits 10 req/s from one source IP, and both senders retry, so
	// an unlucky burst-shed self-heals — only an abusive flood is turned away.
	// BEX_WEBHOOK_RATE_LIMIT=0 disables it (byte-identical to before m60).
	webhookRPM, webhookBurst := cfg.WebhookRateRPM, cfg.WebhookRateBurst
	srv.WebhookRateLimiter = api.NewRateLimiter(webhookRPM, webhookBurst) // nil when rpm=0

	// Deploy-hook lookup limiting is a distinct IP budget outside the auth gate.
	// The hook's inner 6/min token bucket handles a leaked valid credential; this
	// outer cap sheds random-token enumeration before any Kubernetes API lookup.
	deployHookLookupRPM, deployHookLookupBurst := cfg.DeployHookLookupRPM, cfg.DeployHookLookupBurst
	srv.DeployHookLookupRateLimiter = api.NewRateLimiter(deployHookLookupRPM, deployHookLookupBurst)

	// Pre-auth admission (w1/m67 F1). The per-caller limiter above runs INSIDE
	// the auth gate, keyed on the resolved identity, so it cannot bound the work
	// of resolving an identity: with no negative cache, every unique invalid
	// bearer/session costs one Hydra or Kratos round trip. Invalid credentials
	// spend a source-IP budget; every credential also has its own HMAC-keyed
	// request/concurrency partition, so one stolen valid session is shed before
	// upstream I/O without throttling unrelated SSR users behind the same IP.
	// BEX_AUTH_FAILURE_LIMIT=0 + BEX_AUTH_MAX_INFLIGHT=0 disables both.
	authFailureRPM, authFailureBurst := cfg.AuthFailureRPM, cfg.AuthFailureBurst
	srv.AuthAdmission = api.NewAuthAdmission(authFailureRPM, authFailureBurst, cfg.AuthMaxInflight)
	if srv.AuthAdmission != nil {
		srv.AuthAdmission.TrustedProxies = trustedProxies
	}

	// The security-header middleware consults the same trusted CIDRs so HSTS is
	// only emitted for a genuinely-TLS request, never from a spoofed
	// X-Forwarded-Proto (codex-security target #10).
	srv.TrustedProxies = trustedProxies

	// Trusted-proxy awareness applies to every IP-keyed budget alike: the
	// per-caller, device-flow, webhook-intake, and deploy-hook limiters all
	// derive the client IP through the same trusted CIDRs.
	for _, rl := range []*api.RateLimiter{srv.RateLimiter, srv.WebhookRateLimiter, srv.DeployHookLookupRateLimiter} {
		if rl != nil {
			rl.TrustedProxies = trustedProxies
		}
	}
	if srv.DeviceRateLimiter != nil {
		srv.DeviceRateLimiter.TrustedProxies = trustedProxies
	}
}

// newHTTPServer builds an HTTP server with the header/read/write timeouts
// shared by the public and internal control-plane listeners.
func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
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
