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
	"net/url"
	"os"
	"strings"
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
	"github.com/bex-co/bex/lego/backend/internal/identityclaims"
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
	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/serve"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
	"github.com/bex-co/bex/lego/backend/internal/usage"
	"github.com/bex-co/bex/lego/backend/internal/webhooks"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// baseEntrypoint links /bin/bash to the official bash image's
// /usr/local/bin/bash, so a script with a `#!/bin/bash` line runs as it would
// on Render (w32/m348), creates the home directory (baseHome), then sleeps.
// An image that already has /bin/bash, or has no /usr/local/bin/bash, is left
// alone.
const baseEntrypoint = `if [ ! -e /bin/bash ] && [ -x /usr/local/bin/bash ]; then ln -s /usr/local/bin/bash /bin/bash; fi
mkdir -p -- "${HOME:-/root}"
exec sleep infinity`

// baseHome is a base sandbox's home directory, Render's: relative remote
// paths "resolve under /home/render" (render.com/docs/sandboxes-sdk-python,
// read 2026-11-12), and a command starts there (w32/019). The user stays
// root: Render documents only the path.
const baseHome = "/home/render"

func sandboxTemplateRegistry(baseImage, agentImage string) map[string]sandbox.Template {
	// A create without a plan keeps its template's size and is labelled with
	// the default plan, so the base template is the default plan's size: a
	// sandbox created with no flags is Render's 1c-2g in size, meter weight
	// and name (w8/247).
	baseCPU, baseMemory, _ := sandbox.PlanLimits(sandbox.DefaultPlan)
	// An agent session clones and builds a whole repository, which needs more
	// than the sandbox namespace's 10Gi ephemeral storage default (ADR105
	// § Until the AX42-1 arrives); 30Gi stays within the LimitRange's 32Gi max.
	agentTemplate := func(plan sandbox.Plan) sandbox.Template {
		cpu, memory, _ := sandbox.PlanLimits(plan)
		return sandbox.Template{Image: agentImage, Entrypoint: []string{"bex-agent-driver"}, CPU: cpu, Memory: memory, EphemeralStorage: "30Gi", SessionOnly: true}
	}
	return map[string]sandbox.Template{
		"base": {
			Image: baseImage, Entrypoint: []string{"/bin/sh", "-c", baseEntrypoint},
			Env: map[string]string{"HOME": baseHome},
			CPU: baseCPU, Memory: baseMemory,
		},
		// Customers' sessions keep the 2c-4g agent; a session or automation
		// that builds a large repository names agent-4c-8g, and the sandbox
		// meter bills the larger shape.
		"agent":       agentTemplate(sandbox.Plan2C4G),
		"agent-4c-8g": agentTemplate(sandbox.Plan4C8G),
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

// SandboxKeyLookup is the lookup-only seam (never mints) the by-id sandbox
// verbs use to find which of the caller's workspaces owns a sandbox (w4/m172).
func (p sandboxKeyProvider) SandboxKeyLookup(ctx context.Context, workspaceID string) (string, bool, error) {
	return p.st.SandboxKeyLookup(ctx, workspaceID)
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
	if len(os.Args) > 1 && os.Args[1] == platformRebuildCommand {
		os.Exit(runPlatformRebuild(ctx, cfg, os.Args[2:], os.Stdout, os.Stderr))
	}
	// loadConfig already refused this route on a production control plane.
	if err := agentsession.EnableDevModelRoute(cfg.AgentDevModelRoute, cfg.AgentDevModel, cfg.CPIdentity); err != nil {
		log.Fatalf("bex-api: %v", err)
	}
	if cfg.AgentDevModelRoute != "" {
		log.Printf("bex-api: DEV model route %q active for the claude agent profile (dev control plane %q)", cfg.AgentDevModelRoute, cfg.CPIdentity)
	}
	// loadConfig already validated the route and cleared it when the key is
	// missing; the key itself is never logged.
	if err := agentsession.EnablePlatformModelRoute(cfg.AgentPlatformModelRoute, cfg.AgentPlatformModel, cfg.AgentPlatformModelWorkspaces, cfg.AgentPlatformModelKey); err != nil {
		log.Fatalf("bex-api: %v", err)
	}
	if cfg.AgentPlatformModelRoute != "" {
		log.Printf("bex-api: PLATFORM model route %q active for the claude agent profile in %d allowlisted workspace(s): %s",
			cfg.AgentPlatformModelRoute, len(cfg.AgentPlatformModelWorkspaces), strings.Join(cfg.AgentPlatformModelWorkspaces, ","))
	}
	// The muse profile (ADR108) runs only on this bex-owned Meta key, in the
	// same allowlisted workspaces; the key is never logged.
	if agentsession.EnablePlatformMetaRoute(cfg.AgentPlatformMetaKey, cfg.AgentPlatformModelWorkspaces) {
		log.Printf("bex-api: PLATFORM Meta route active for the muse agent profile in %d allowlisted workspace(s)", len(cfg.AgentPlatformModelWorkspaces))
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
	// bex-api's own request telemetry (w3/m84, docs/ADR088 §6): one instance
	// shared by both listeners and the GraphQL/MCP observation points, so the
	// origin-side view of an incident lives on the same registry /metrics serves.
	originMetrics := api.NewOriginMetrics(metricRegistry)
	if cfg.AgentPlatformModelRoute != "" && cfg.AgentPlatformModelKey != "" {
		// ADR091 D5 global backstop: export the OpenRouter account balance
		// behind the platform key every 10 minutes (PlatformModelCreditLow
		// warns below $25, PlatformModelCreditCritical below $5). The
		// account balance is the cap; the key itself has no limit.
		go agentsession.NewPlatformCreditMonitor(metricRegistry, cfg.AgentPlatformModelKey).Run(ctx)
	}
	pushMetrics.SetEnabled(mobilePush != nil || webPush != nil)

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appv1alpha1.AddToScheme(scheme))

	kubeCfg := ctrl.GetConfigOrDie() // in-cluster, or KUBECONFIG for local dev
	// WithWatch: the App status watch (w5/m369) watches through it too.
	cl, err := client.NewWithWatch(kubeCfg, client.Options{Scheme: scheme})
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
	if len(os.Args) > 1 && os.Args[1] == migrateContainerPolicyCommand {
		os.Exit(runMigrateContainerPolicy(ctx, cpDBURI, base, os.Args[2:], os.Stdout, os.Stderr))
	}

	// One readiness flag for the whole pod (w1/m52): the public server's
	// /readyz answers 200 until SIGTERM, then 503 while both servers keep
	// serving through the drain window. The readiness probe points here;
	// liveness stays on /healthz, which never flips.
	ready := &serve.Readiness{}

	// Before any request is served: which registries an image may name (w5/171).
	store.PublicImageRegistries = cfg.PublicImageRegistries
	deps := api.Deps{
		RouterURL:        cfg.RouterURL,
		RouterSecret:     cfg.RouterSecret,
		PushAvailable:    mobilePush != nil,
		WebPushAvailable: webPush != nil,
		WebhookMetrics:   webhookMetrics,
		// BEX_BASE_DOMAIN names custom-domain DNS targets `<app>.<base>` (docs/ADR005-custom-domain.md);
		// unset falls back to deriving the platform host from an App's status URLs.
		BaseDomain:         cfg.BaseDomain,
		ReservedDomains:    cfg.ReservedDomains,
		ACMEDelegationZone: cfg.ACMEDelegationZone,
		// One shared detector (and DNS cache) for the allowlist's
		// Cloudflare-proxied-domain warning on services and environments.
		ProxiedHosts: core.NewProxiedHostDetector(),
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
	platformRebuild := &lateHandler{}
	shareFiles := &lateHandler{}
	agentResults := &lateHandler{}
	agentSpawns := &lateHandler{}
	identityClaims := identityClaimsHandler(cfg, deps.Identities)

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
		rec.ProductObserver = func(ctx context.Context, desired store.DesiredApp, app *appv1alpha1.App) {
			apps.ObserveProductApp(ctx, st, cl, cfg.BaseDomain, desired, app)
		}
		rec.Metrics = store.NewReconcilerMetrics(metricRegistry)
		// A release a suspend ended has its build deleted where Cancel's is.
		rec.BuildNamespace = cfg.BuildNamespace
		if cfg.CPResyncSet {
			rec.Resync = cfg.CPResync
		}
		// rec.Run is started after NewServer below, so CloneSecrets is set before
		// the first reconcile pass (w2/m11).
		granter := wireControlPlaneFeatures(cfg, &deps, base, st, rec, authzChecker)
		twoFactorGate := wireTwoFactor(&deps, base, st, metricRegistry)

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
		usageSvc.FreeInstanceHours = cfg.FreeInstanceHours
		usageSvc.Metrics = usage.NewMetrics(metricRegistry)
		deps.Usage = usageSvc

		var thresholdCharges agentsession.ThresholdCharger
		stripeLifecycleWorker, stripeLifecycleReconciler, stripeBillingAdmin, thresholdCharges = wireStripeBilling(ctx, cfg, &deps, base, cl, st, appsNS, usageSvc, billingMetrics)

		go usageSvc.Run(ctx)

		// Audit log retention (w4/m10 + w2/m39): purges audit_events and SSH
		// session metadata older than BEX_AUDIT_RETENTION_DAYS daily, same
		// cadence/shape as usage's compaction loop above. The write side is
		// base.Audit (wired above); this Service is the read verb + sweep only.
		auditSvc := &audit.Service{Base: base, Store: st, UserIDs: st}
		if cfg.AuditRetentionSet {
			auditSvc.RetentionDays = cfg.AuditRetention
		}
		deps.Audit = auditSvc
		go auditSvc.Run(ctx)

		// ADR062: the model-credential mint, wired only when OpenBao is
		// reachable. Spend: the platform route's monthly agent-model spend
		// cap (ADR091 D5: min(admin override or pricing.yaml plan default,
		// the trust tier's ceiling) on month-to-date cost) and the BYO key
		// flag (D7), read through one cache that this process's key writes
		// invalidate. Charges: the threshold charge made in the request that
		// reaches it (D10.2a; nil without Stripe billing).
		var modelMinter *agentsession.ModelMinter
		if deps.Secrets != nil {
			modelMinter = &agentsession.ModelMinter{Keys: deps.Secrets, Sessions: st, Audit: st, Spend: st, SessionSpend: st, Charges: thresholdCharges}
			deps.AgentModelKeyChanged = modelMinter.ForgetSpend
			go backfillModelKeyFlags(ctx, deps.Secrets, st)
		}

		go backfillSkillVersionContent(ctx, st)

		startControlPlaneServer(ctx, cfg, st, rec, granter, stripeBillingAdmin, metricRegistry, originMetrics, agentMetrics, ghClient, modelMinter, twoFactorGate, opsRole, identityClaims, platformRebuild, shareFiles, agentResults, agentSpawns, ready)
	} else if opsRole != nil && !cfg.MCPStdio {
		// ADR088 §4: without the control plane there is no :8091 mux to share,
		// so a configured ops-role verb gets its own minimal cluster-internal
		// listener (local dev / e2e without BEX_CP_DB_URI). Production always
		// runs the control plane and takes the branch above.
		startOpsRoleServer(ctx, cfg, opsRole, identityClaims, ready)
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
	deps.AgentEgressProxyURL = cfg.AgentEgressProxyURL
	wireSandboxes(ctx, cfg, &deps, cl, st, metricRegistry)
	wireAgentSessions(&deps, cfg)
	wireDiskSnapshots(&deps, cfg)
	srv := api.NewServer(base, deps)
	platformRebuild.set(deploys.PlatformRebuildHandler(srv.Deploys, cfg.CPToken))
	if srv.AgentSessions != nil && st != nil && cfg.SandboxExecSecret != "" {
		shareFiles.set(&agentsession.ShareFileHandler{Secret: []byte(cfg.SandboxExecSecret), Files: srv.AgentSessions, Nonce: st})
		agentResults.set(&agentsession.ResultHandler{Secret: []byte(cfg.SandboxExecSecret), Results: srv.AgentSessions, Nonce: st})
		agentSpawns.set(&agentsession.SpawnHandler{Secret: []byte(cfg.SandboxExecSecret), Spawns: srv.AgentSessions, Nonce: st})
	}
	srv.OriginMetrics = originMetrics
	srv.CLITelemetry.Metrics = originMetrics
	// One CompletionMetrics registration shared by the Completer (turn duration,
	// convergence) and the Service (provisioning latency) so the agent-session
	// lifecycle timing lives under one set of series (w5/m81). Created above with
	// the other registries so ModelAuthFailer on :8091 can observe too (w5/m88).
	srv.AgentSessionCompleter.Metrics = agentMetrics
	srv.AgentSessions.Metrics = agentMetrics
	// The completer records the session branch's head as GitHub has it, not as
	// the sandbox reports it (w5/m382).
	if ghClient != nil {
		srv.AgentSessionCompleter.Heads = agentsession.BranchHeads{GitHub: ghClient, Connections: st}
	}
	// The /v1/sandboxes surface reports separately from agent sessions above:
	// they share a runtime, but only this series answers whether the sandbox API
	// itself is healthy — the gap ADR088's coverage table left unfiled (w5/m95).
	if srv.Sandbox != nil {
		m := sandbox.NewMetrics(metricRegistry)
		srv.Sandbox.Metrics = m
		if srv.AgentSessionCompleter != nil {
			srv.AgentSessionCompleter.SandboxMetrics = m
		}
		if srv.AgentSessions != nil {
			srv.AgentSessions.SandboxMetrics = m
		}
		// Idle suspension (w2/m278): pause sandboxes past idleTimeoutSeconds
		// with no exec or file transfer. Every replica sweeps; the store's
		// claim makes one of them pause each sandbox. Store-off has no
		// activity record and no tenant keys to sweep.
		if st != nil {
			go (&sandbox.IdleSuspender{Service: srv.Sandbox, Tenants: st}).Run(ctx)
			// Imports whose replica died are failed, so a caller is not left
			// polling a build no one advances (w2/m358).
			go srv.Sandbox.RunImageSweeper(ctx)
			// Group deletes a request could not finish (w2/m359).
			go srv.Sandbox.RunGroupDeleteSweeper(ctx)
		}
	}
	// codex round-8 #9: the signed git webhook durably claims each processed
	// delivery body so a captured (body, signature) pair cannot be replayed into
	// repeated deploys. A configured webhook without this durable store is
	// rejected below; store-less deployments must leave webhook secrets unset.
	if st != nil {
		srv.WebhookReplays = st
		// PR feedback (w9/m344, ADR047 D11) needs the App to read who
		// commented and Kratos to link their GitHub sign-in to a member. A
		// `/bex` takeover's reply links the session's dashboard page.
		if srv.AgentSessions != nil && ghClient != nil && kratosAdminURL != "" {
			srv.PRFeedback = &agentsessions.PRFeedback{Sessions: srv.AgentSessions, Store: st, GitHub: ghClient,
				Identities: workspaces.NewKratosIdentities(kratosAdminURL), DashboardURL: cfg.DashboardURL}
		}
		// Pull request states (w9/m386) are read with the App's installation
		// tokens; no extra webhook subscription is needed.
		if srv.AgentSessions != nil && ghClient != nil {
			srv.PRStates = &agentsessions.PRStates{Store: st, GitHub: ghClient}
		}
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
		// One-shot ADR051 item-model backfill (w10/m16): rewrite legacy raw-chunk
		// transcript turns into completed items, in the background so startup
		// never waits on it. Idempotent; once drained it is one index probe.
		go compactAgentTranscripts(ctx, st)
	}
	// Membership rows and exact OpenFGA roles are joined by a transactional
	// Postgres outbox. Drain it independently of request retries so an invite or
	// downgrade survives transient OpenFGA failures and process restarts.
	go srv.Members.RunRoleReconciler(ctx)
	go srv.Accounts.Run(ctx)
	// The agent-session Completer finalizes fire-and-forget sessions from the
	// driver's status file and the PR the agent already opened (ADR090). It is a
	// no-op unless the store and OpenSandbox are wired.
	go srv.AgentSessionCompleter.Run(ctx)
	// ADR095 D2/D7: expire unbound uploads and purge deleted attachments.
	go agentsessions.RunAttachmentSweep(ctx, srv.AgentSessions, time.Hour)
	// The Blueprint recovery sweep settles sync runs abandoned by process
	// loss (w8/m37 t004). No-op unless the control-plane store is wired.
	go srv.BlueprintRecovery.Run(ctx)
	// Free Postgres expiry (w8/m126): warn, suspend at expiresAt, delete after
	// the grace period.
	go srv.PostgresExpiry.Run(ctx)
	// Monthly Free instance hours (w8/m139): warn at 80%, suspend Free web
	// services when spent, resume at the month boundary.
	go srv.FreeHours.Run(ctx)
	// Included build minutes (w6/m308): warn at 80%; out of them with no way
	// to pay, builds stop until the month turns.
	go srv.BuildMinutes.Run(ctx)
	// Included outbound bandwidth (w6/m319): warn at 80%; out of it with no
	// payment method, the workspace's services spin down until the month turns.
	go srv.Bandwidth.Run(ctx)
	// Crash-loop suspension (w2/m253): a service failing to start and become
	// healthy for 24h is suspended (stuck_crashlooping) and its owners emailed.
	go srv.CrashLoop.Run(ctx)
	// Service disk usage (w2/m255): crossing 80% of a disk's size, and dropping
	// back below, records Render's events and emails the workspace.
	go srv.DiskUsageAlerts.Run(ctx)
	// Durable Blueprint auto-sync worker (w8/m38): claims intents accepted at
	// the signed git webhook. No-op unless Blueprints store is wired.
	go srv.BlueprintAutoSync.Run(ctx)
	go srv.CheckedDeploys.Run(ctx)
	go srv.DeferredRestarts.Run(ctx)
	// Custom-domain verification sweep (w5/m137): promotes a pending domain
	// once its traffic DNS propagates. No-op store-less.
	go srv.DomainVerification.Run(ctx)
	go srv.ServiceRefs.Run(ctx)
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
	srv.RequireVerifiedEmail = cfg.RequireVerifiedEmail
	if deps.Identities != nil {
		identities := deps.Identities
		srv.EmailVerified = func(ctx context.Context, subject string) (bool, bool) {
			attrs, ok := identities.Lookup(ctx, subject)
			return attrs.EmailVerified, ok
		}
	} else if cfg.RequireVerifiedEmail {
		log.Printf("bex-api: BEX_REQUIRE_VERIFIED_EMAIL is on but BEX_KRATOS_ADMIN_URL is unset; human OAuth tokens will fail closed (503)")
	}
	configureServerAuthOptions(srv, cfg)

	// stdio MCP mode: `api mcp-stdio` (or BEX_MCP_STDIO=1) serves only the MCP
	// adapter over stdin/stdout — how a local agent launches bex as a subprocess.
	if cfg.MCPStdio {
		log.Printf("bex-api: serving MCP over stdio (namespace %s)", base.Namespace)
		run := srv.RunStdio
		if cfg.MCPToolset != "" {
			run = func(ctx context.Context) error { return srv.RunStdioToolset(ctx, cfg.MCPToolset) }
		}
		if err := run(ctx); err != nil {
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

	// API-key bindings written before w5/m103 added tenant_members.kind default
	// to 'user', so they still look like members. Hydra is the only registry of
	// client ids, so the reclassification cannot live in the SQL migration —
	// reconcile once here, against the live client list.
	//
	// Deliberately NOT fatal: the failure mode is cosmetic (a key keeps
	// appearing on the Team surface and holding a seat, exactly as it did
	// before), and refusing to serve the API because Hydra was briefly
	// unreachable would be a far worse trade. It retries on the next start.
	backfillMachineMemberships(ctx, st, deps.APIKeys)

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
	// The request-telemetry middleware is the OUTERMOST wrapper (w3/m84): a
	// request shed by the auth gate, a rate limiter, or the body cap is part of
	// the origin's error picture, so it has to be counted too.
	// Origin classification sits inside the metrics wrapper but outside
	// everything else, and unlike the metrics wrapper it is unconditional:
	// product-analytics attribution must not depend on whether metrics are
	// enabled (w5/m97).
	httpSrv := newHTTPServer(addr, originMetrics.Middleware(api.RequestOriginMiddleware(root)))
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
		deps.DatastoreResource = metrics.NewPrometheusDatastoreResourceSource(promURL, nil)
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
	if srv.Apps != nil {
		srv.Apps.ManualWebhookDisabled = srv.MultitenantWebhook
	}
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

// Wires the gate onto core.Base and returns it so :8091 can serve the gateway
// verb. A nil Enrollment fails a requiring workspace closed.
func wireTwoFactor(deps *api.Deps, base *core.Base, st *store.PGStore, registerer prometheus.Registerer) *twofactor.Gate {
	cfg := twofactor.Config{Settings: st, Sessions: st, Metrics: twofactor.NewMetrics(registerer)}
	if deps.Identities != nil {
		cfg.Enrollment = deps.Identities
	}
	if keys, ok := deps.APIKeys.(twofactor.KeyCreators); ok {
		cfg.Keys = keys
	} else if deps.APIKeys != nil {
		// Without it every API-key call on a workspace that requires 2FA is a 503.
		log.Printf("WARNING: the API key store (%T) has no KeyCreator, so API keys cannot be checked against a workspace's two-factor requirement and fail closed there", deps.APIKeys)
	}
	gate := twofactor.New(cfg)
	base.TwoFactor = gate
	deps.TwoFactorChanged = gate.Invalidate
	return gate
}

// wireControlPlaneFeatures wires the control-plane store into the feature deps
// and adapts the authz checker's role grant/revoke sides, returning the
// membership granter shared by the tenant service and the internal CP API.
func wireControlPlaneFeatures(cfg *Config, deps *api.Deps, base *core.Base, st *store.PGStore, rec *store.Reconciler, authzChecker core.Checker) store.MembershipGranter {
	deps.AppPlacements = st
	deps.Store = st // single writer of intent: suspend/resume write the row first
	deps.OAuthRevocations = st
	deps.AccountStore = st
	deps.RouterMembers = st
	deps.SSHKeysStore = st // identity-scoped SSH public-key registry
	deps.DeployStore = st  // deploy history (w2/m5): list/get/trigger read+write the same rows
	// Cancel (w2/m10) needs to compute a repo-backed App's in-flight build
	// Job's identity — must match the operator's own BEX_BUILD_NAMESPACE.
	deps.DeployBuildNamespace = cfg.BuildNamespace
	deps.GitHubStore = st               // git connections (w2/m8): connect/disconnect/list read+write git_connections
	deps.EventStore = st                // service events: deploy + audit + typed observed/Git facts
	deps.EventFacts = st                // typed observed/Git event facts (w3/m19), written outside the operator
	deps.CronRunHistory = st            // cron runs past the status window, from their facts (w3/m366)
	deps.CronRunActors = st             // who triggered and canceled each cron run (w4/631)
	deps.NotificationsStore = st        // deploy notifications (w3/m9): settings read/write + the reconciler's recipient fan-out
	deps.ProjectsStore = st             // project groupings (w1/m31): project CRUD + service-assignment
	deps.EnvironmentsStore = st         // environment groupings (layered on w1/m31): environment CRUD + service-assignment
	deps.RegistryCredsStore = st        // registry credentials (w2/m14): CRUD metadata rows; secrets live in OpenBao (deps.Secrets)
	deps.EnvGroupsStore = st            // environment-group metadata (w8/m114); values stay in OpenBao
	deps.CLITelemetryStore = st         // CLI telemetry ingest (w5/m92): one row per cli-telemetry-events delivery (nil-safe: *store.PGStore stays a comparable nil when the store is off)
	deps.BlueprintsStore = st           // blueprint registry (w2/m15): auto-upserted on deploy, list+sync read it
	deps.WebhookStore = st              // outbound webhooks (w3/m11): endpoint CRUD + delivery history; the worker below delivers
	deps.JobStore = st                  // one-off jobs (Render's /services/{id}/jobs): job CRUD + k8s Job tracking
	deps.AgentSessionStore = st         // ADR047 D3: durable agent-session lifecycle
	deps.AgentAttachmentStore = st      // ADR095: attachment metadata
	deps.AgentSkillStore = st           // ADR099: workspace skills
	deps.AgentSecretScopeStore = st     // ADR098 D9: agent secret scopes
	deps.AgentRepoEnvironmentStore = st // ADR109: repository environments
	deps.AgentAutomationStore = st      // ADR101: agent automations
	deps.AgentFeedbackStore = st        // ADR047 D11: PR feedback turn origins
	// w10/m17: the per-workspace agent environment (network tier, hosts, model).
	deps.AgentEnvironmentStore = st
	// ADR091 D5, D10.1: agent-model spend caps + trust tiers.
	deps.AgentSpendStore = st
	// ADR091 D6: per-session, per-turn model cost from the usage events.
	deps.AgentModelCostStore = st
	// w9/m178: the sandbox compute each session's sandboxes accrued.
	deps.AgentComputeStore = st
	if writer, ok := authzChecker.(agentsessions.TupleWriter); ok {
		deps.AgentSessionTuples = writer
	}

	// Audit log (w4/m10): *store.PGStore structurally satisfies
	// core.AuditSink, so every write verb's Authorize/AuthorizeOn call
	// starts recording the instant the store is wired — no extra plumbing.
	base.Audit = st
	base.ProductActivity = st.RecordProductActivity
	base.Billing = st
	// The plan, with the log floor an upgrade kept in the same read (w9/m389).
	base.Plans = st
	// The workspace inbound-IP layer every create path stamps (w9/m388).
	base.WorkspaceIPRules = st

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
		&envgroups.WorkspacePurger{Service: &envgroups.Service{Base: base, Store: deps.Secrets, Groups: st}},
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
// unchanged. The returned ThresholdCharger is the model mint's agent-model
// threshold charger (ADR091 D10.2a), nil while billing or the agent-model
// meter is off.
func wireStripeBilling(ctx context.Context, cfg *Config, deps *api.Deps, base *core.Base, cl client.Client, st *store.PGStore, appsNS string, usageSvc *usage.Service, billingMetrics *billing.Metrics) (*billing.Worker, *billing.Reconciler, store.BillingAdmin, agentsession.ThresholdCharger) {
	var stripeLifecycleWorker *billing.Worker
	var stripeLifecycleReconciler *billing.Reconciler
	var stripeBillingAdmin store.BillingAdmin
	var thresholdCharges agentsession.ThresholdCharger
	requirePaymentMethod := cfg.RequirePaymentMethod
	stripeSecretKey, billingEpoch, stripeEnabled := cfg.StripeSecretKey, cfg.StripeEpoch, cfg.StripeEnabled
	if stripeEnabled {
		publishableKey := cfg.StripePublishableKey
		billingMetrics.SetEnabled(true)
		// ADR091 D3: until the Stripe catalog carries the agent model cost
		// Price, hold that meter so neither Subscriptions nor the outbox
		// reference a Price that does not exist; held rows ship once released.
		var heldMeters []string
		if !cfg.StripeAgentModelMeter {
			heldMeters = []string{store.UsageKindAgentModelCostMicroUSD}
		}
		thresholds := &billing.AgentModelThresholds{Store: st, Interval: cfg.StripeReconcileInterval, Metrics: billingMetrics}
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
			Cards:                 st,
			Audit:                 st,
			Metrics:               billingMetrics,
			HeldMeters:            heldMeters,
			AgentModelThreshold:   thresholds.Want,
		})
		thresholds.Stripe = stripeClient
		go thresholds.Run(ctx)
		usageSvc.Billing = stripeClient
		deps.Billing = stripeClient
		deps.BillingState = st
		deps.BillingEligibility = st
		if publishableKey != "" {
			deps.WorkspaceCreationBilling = stripeClient
			go (&billing.WorkspaceCreationCleaner{Store: st, Provider: stripeClient, Metrics: billingMetrics}).Run(ctx)
		}
		if requirePaymentMethod != paymentMethodOff {
			base.Payment = &billing.PaymentGate{Store: st}
			// ADR075 D7: "all" widens RequireBillingFor to the free tier too.
			base.PaymentAllPlans = requirePaymentMethod == paymentMethodAllPlans
			// Reclaim Customers that never bound a card. Gate-on only —
			// with the gate off the emitter provisions cardless Customers on
			// purpose and would immediately re-mint them.
			go (&billing.CheckoutReclaimer{Store: st, Provider: stripeClient, Metrics: billingMetrics}).Run(ctx)
		}
		stripeBillingAdmin = &billing.Admin{Store: st, Provider: stripeClient}
		// Workspace-delete Stripe teardown (w1/m61): cancel the workspace's
		// metered Subscription when its workspace is deleted (keeping the Customer
		// for invoice history). Pre-cascade, so the billing_provider_mappings row
		// still resolves the subscription id, and LAST among them (NewServer
		// appends it after every other pre-cascade purger): once it has run, a
		// retried delete finds no live Subscription and its settle is a no-op
		// (ADR091 D10.4), so no other purger may fail after it. Only wired when
		// Stripe is enabled — byte-identical to before m61 otherwise.
		deps.WorkspaceBillingPurger = &billing.WorkspacePurger{Canceller: stripeClient}

		emitter := billing.NewEmitter(st, stripeClient)
		emitter.Metrics = billingMetrics
		emitter.Epoch = billingEpoch
		emitter.RequirePaymentMethod = requirePaymentMethod != paymentMethodOff
		// ADR091 D3: the agent model cost meter's hourly row is final once its
		// hour closes, so it ships ~5 min after the hour instead of after the
		// 48h seal — what lets a Stripe usage threshold charge promptly.
		emitter.FastKinds = []string{store.UsageKindAgentModelCostMicroUSD}
		emitter.HoldKinds = heldMeters
		// docs/ADR030 §13: each plan's included pipeline minutes are free, so
		// only the build minutes above them reach Stripe.
		emitter.Plans = base
		if cfg.StripeSealHoursSet {
			emitter.SealHours = time.Duration(cfg.StripeSealHours) * time.Hour
		}
		// w7/m57: never seal shorter than the usage rollup's catch-up window, or
		// an exported row could still be rewritten and never re-emitted.
		if clamped := billing.ClampSealHours(emitter.SealHours, usage.CatchupWindow); clamped != emitter.SealHours {
			log.Printf("BEX_STRIPE_SEAL_HOURS=%s is below the usage catch-up window; raising the seal horizon to %s so exported rows are final", emitter.SealHours, clamped)
			emitter.SealHours = clamped
		}
		// ADR091 D10.4: workspace and account deletion settle the workspace's
		// unbilled usage first and refuse on a declined charge.
		deps.WorkspaceSettler = &billing.Settler{Emitter: emitter, Usage: st, Invoices: stripeClient}
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
		// ADR091 D10.1–D10.3: invoice and dispute events feed the agent-model
		// trust tiers, bad debt and the threshold charge ledger; the
		// threshold charger records its own charges in the same trust record.
		trust := &billing.Trust{Store: st, Charges: stripeClient,
			OnTierChanged: thresholds.OnTierChanged,
			WriteOffs:     st, Invoices: stripeClient, ModelCharges: st}
		// ADR091 D10.2a: the mint charges a workspace's unbilled agent-model
		// spend in the request that reaches its tier's threshold. Netting the
		// charge out of the cycle invoice assumes the meter bills that usage,
		// so a held meter has no charger (a T0 workspace stops at $10).
		if cfg.StripeAgentModelMeter {
			modelCharges := &billing.AgentModelCharger{Store: st, Stripe: stripeClient, Trust: trust}
			go modelCharges.Run(ctx)
			thresholdCharges = thresholdCharger{modelCharges}
		}
		if secret := cfg.StripeWebhookSecret; secret != "" {
			handler := &billing.StripeWebhook{Secret: secret, ExpectedLivemode: stripeClient.ExpectedLivemode(), OnCheckoutCompleted: stripeClient.CompleteCheckoutSession, OnCustomerUpdated: stripeClient.RecordPortalPaymentMethodChange, Metrics: billingMetrics}
			if lifecycle != nil {
				handler.OnLifecycle = lifecycle.HandleStripeEvent
			}
			handler.OnTrust = trust.HandleStripeEvent
			go backfillBillingTrust(ctx, st, stripeClient, trust)
			deps.StripeWebhook = handler
		}
		log.Printf("bex-api Stripe Billing enabled (seal horizon %s, epoch %s, webhook %t)", emitter.SealHours, billingEpoch.Format(time.RFC3339), deps.StripeWebhook != nil)
		go emitter.Run(ctx)
	}
	return stripeLifecycleWorker, stripeLifecycleReconciler, stripeBillingAdmin, thresholdCharges
}

// thresholdCharger adapts the billing charger to the mint's
// agentsession.ThresholdCharger, which keeps Stripe out of agentsession (and
// so out of the SSH gateway that imports it).
type thresholdCharger struct{ charger *billing.AgentModelCharger }

func (t thresholdCharger) ChargeThreshold(ctx context.Context, charge agentsession.ThresholdCharge) (agentsession.ThresholdChargeOutcome, error) {
	result, err := t.charger.ChargeThreshold(ctx, charge)
	if err != nil {
		return agentsession.ThresholdChargePending, err
	}
	switch result.Outcome {
	case billing.ChargePaid:
		return agentsession.ThresholdChargePaid, nil
	case billing.ChargeDeclined:
		return agentsession.ThresholdChargeDeclined, nil
	case billing.ChargeNotDue:
		return agentsession.ThresholdChargeNotDue, nil
	}
	return agentsession.ThresholdChargePending, nil
}

// startControlPlaneServer starts the internal control-plane API (:8091): it
// grants workspace-admin and cross-tenant writes, so it must never serve
// unauthenticated. Fail closed at startup when BEX_CP_TOKEN is empty (w1/m53:
// the token was set nowhere in prod, so the API had been serving open behind
// the NetworkPolicy alone). BEX_CP_INSECURE=1 is a loud local-dev override.
func startControlPlaneServer(ctx context.Context, cfg *Config, st *store.PGStore, rec *store.Reconciler, granter store.MembershipGranter, stripeBillingAdmin store.BillingAdmin, metricRegistry *prometheus.Registry, originMetrics *api.OriginMetrics, agentMetrics *agentsessions.CompletionMetrics, ghClient *github.Client, modelMinter *agentsession.ModelMinter, twoFactor *twofactor.Gate, opsRole *opsrole.Handler, identityClaims *identityclaims.Handler, platformRebuild, shareFiles, agentResults, agentSpawns http.Handler, ready *serve.Readiness) {
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
		// ADR090: the agent's own draft-PR verb. Same gateway-only HMAC and
		// internal-only listener; owner/repo/base/head come from the durable
		// session, never the request. Unwired without the GitHub App.
		if ghClient != nil {
			pullRequests := &agentsession.PullRequester{
				GitHub: ghClient, Connections: st, Sessions: st, Audit: st,
				DashboardURL: cfg.DashboardURL,
			}
			internalRoot.Handle(agentsession.InternalPullRequestPath, &agentsession.PullRequestHandler{
				Secret:       []byte(cfg.SandboxExecSecret),
				PullRequests: pullRequests,
				Nonce:        st,
			})
			// ADR047 D11: the agent's reply on its own PR. Same HMAC under a
			// `pull-request-comment` MAC domain; the PR number comes from the
			// durable session, never the request.
			internalRoot.Handle(agentsession.InternalPullRequestCommentPath, &agentsession.PullRequestCommentHandler{
				Secret:       []byte(cfg.SandboxExecSecret),
				PullRequests: pullRequests,
				Nonce:        st,
			})
		}
		// w2/m168: native public-key SSH never passes the auth middleware, so the
		// gateway asks here whether a key owner's email is verified. Unwired
		// Kratos admin leaves Resolve nil, so the verb answers 503 and the
		// gateway refuses: closed, never open.
		verification := &agentsession.EmailVerificationHandler{Secret: []byte(cfg.SandboxExecSecret), Nonce: st}
		if cfg.KratosAdminURL != "" {
			verification.Resolve = workspaces.NewKratosIdentities(cfg.KratosAdminURL).EmailVerification
		}
		internalRoot.Handle(agentsession.InternalEmailVerificationPath, verification)
		// The same door for a workspace's two-factor requirement. The gateway
		// cannot read the setting or Kratos, so native SSH and every ticket it
		// redeems or revalidates ask here, and the real gate decides. It answers
		// only for the identity classes the gateway authenticates
		// (agentsession.TwoFactorHandler); an unwired gate leaves the verb 503, so
		// the gateway refuses.
		twoFactorVerb := &agentsession.TwoFactorHandler{Secret: []byte(cfg.SandboxExecSecret), Nonce: st}
		if twoFactor != nil {
			twoFactorVerb.Policy = twoFactor
		}
		internalRoot.Handle(agentsession.InternalTwoFactorPath, twoFactorVerb)
		// w10/m19: the gateway egress proxy asks here whether a Pod-bound
		// session's workspace runs the `full` network tier. Same gateway-only
		// HMAC + internal-only listener; it releases no credential.
		// ADR095 D6: files the agent shares reach the session's output
		// attachments here. Same gateway-only HMAC + internal-only listener; the
		// envelope names the gateway-verified session and binds the body's
		// digest. Late-bound: the agent-session service is built after :8091.
		internalRoot.Handle(agentsession.InternalShareFilePath, shareFiles)
		// ADR094 D5: the structured result the agent submits for a turn. Same
		// gateway-only HMAC (a `submit-result` MAC domain) and late binding.
		internalRoot.Handle(agentsession.InternalResultPath, agentResults)
		// ADR102 D9: a running agent starts and reads its child sessions. Same
		// gateway-only HMAC (a `spawn-session` MAC domain) and late binding.
		internalRoot.Handle(agentsession.InternalSpawnPath, agentSpawns)
		internalRoot.Handle(agentsession.InternalEgressGrantPath, &agentsession.EgressGrantHandler{
			Secret:  []byte(cfg.SandboxExecSecret),
			Granter: &agentsession.EgressGranter{Sessions: st, Environments: st, Audit: st},
			Nonce:   st,
		})
		// ADR062: the model-credential mint. Same gateway-only HMAC + internal-only
		// listener as the Git mint, path-domain-separated. Wired only when OpenBao is
		// reachable (modelMinter non-nil), so a deployment without the BYO key store
		// simply 503s the model proxy and never falls back to sandbox key injection.
		if modelMinter != nil {
			internalRoot.Handle(agentsession.InternalModelMintPath, &agentsession.ModelHandler{
				Secret: []byte(cfg.SandboxExecSecret),
				Minter: modelMinter,
				Nonce:  st,
			})
			// The gateway reports each platform-route exchange's token usage
			// and OpenRouter cost here; it lands in usage_hourly (ADR023) once
			// per request id, priced at cost × agentModel.markup (ADR091 D1).
			// An exchange without a reported cost is priced from OpenRouter's
			// cached model list (D2.2), fetched in the background from startup.
			// Same gateway-only HMAC + internal-only listener.
			prices := &agentsession.OpenRouterPriceTable{}
			prices.Start(ctx)
			internalRoot.Handle(agentsession.InternalModelUsagePath, &agentsession.ModelUsageHandler{
				Secret:   []byte(cfg.SandboxExecSecret),
				Recorder: &agentsession.ModelUsageRecorder{Store: st, Prices: prices},
				Nonce:    st,
			})
			// w5/m80 t003: the gateway reports a vendor auth rejection (401/403 on the
			// gateway→vendor hop) here so bex-api terminalizes the session fast instead
			// of the agent CLI burning its full retry/backoff. Same gateway-only HMAC +
			// internal-only listener as the mint; wired whatever the model keys,
			// since it only needs the store to CAS a live session to failed.
			internalRoot.Handle(agentsession.InternalModelAuthFailurePath, &agentsession.ModelAuthFailureHandler{
				Secret: []byte(cfg.SandboxExecSecret),
				Failer: &agentsession.ModelAuthFailer{
					Sessions: st, Audit: st, Spend: st,
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
	// The identity-claims verb rides the same internal-only rule and bearer.
	identityclaims.Register(internalRoot, identityClaims)
	// The fleet rebuild verb (platform-rebuild) shares this listener and the
	// control-plane bearer: like the projection API it writes across tenants.
	internalRoot.Handle("POST "+deploys.PlatformRebuildPath, platformRebuild)
	internalRoot.Handle("/", internal.Handler())
	cpAddr := cfg.CPAddr
	// Metered as surface="internal": the projection/mint/ops-role verbs are not
	// product API and must never be averaged into the public SLIs (w3/m84).
	cpSrv := newHTTPServer(cpAddr, originMetrics.InternalMiddleware(internalRoot))
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

// identityClaimsHandler builds the identity-claims verb (internal/identityclaims)
// whenever the internal bearer BEX_OPS_ROLE_TOKEN is set; nil otherwise, so the
// route is never mounted. It reads the same Kratos admin reader as the ops-role
// verb and fails closed (503) when that reader is unwired or misses.
func identityClaimsHandler(cfg *Config, identities workspaces.IdentityReader) *identityclaims.Handler {
	if cfg.OpsRoleToken == "" {
		return nil
	}
	h := &identityclaims.Handler{Token: cfg.OpsRoleToken}
	if identities != nil {
		h.Identity = func(ctx context.Context, subject string) (identityclaims.Claims, bool) {
			attrs, ok := identities.Lookup(ctx, subject)
			return identityclaims.Claims{Email: attrs.Email, EmailVerified: attrs.EmailVerified, Name: attrs.Name}, ok
		}
	}
	return h
}

// startOpsRoleServer starts the cluster-internal listener with ONLY the ADR088
// ops-role verb (and the identity-claims verb beside it) mounted — the control-plane-less shape (local dev / e2e
// without BEX_CP_DB_URI). With the control plane on, the verb instead shares
// the :8091 mux (startControlPlaneServer); either way it never touches the
// public :8090 surface.
func startOpsRoleServer(ctx context.Context, cfg *Config, h *opsrole.Handler, identityClaims *identityclaims.Handler, ready *serve.Readiness) {
	// loadConfig parses BEX_CP_ADDR under its own copy of the "internal
	// listener will run" predicate; if the two ever drift, an empty addr here
	// would make net/http bind ":http" (port 80) — a silent misbind on a
	// privileged port. Refuse loudly instead.
	if cfg.CPAddr == "" {
		log.Fatalf("bex-api ops-role listener: BEX_CP_ADDR empty (loadConfig's listener predicate drifted from startup's)")
	}
	internalRoot := http.NewServeMux()
	opsrole.Register(internalRoot, h)
	identityclaims.Register(internalRoot, identityClaims)
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
func wireSandboxes(ctx context.Context, cfg *Config, deps *api.Deps, cl client.Client, st *store.PGStore, reg prometheus.Registerer) {
	if cfg.OpenSandboxURL != "" {
		deps.SandboxClient = sandbox.NewClient(cfg.OpenSandboxURL)
		deps.SandboxMaxGroups = cfg.SandboxMaxGroupsPerWorkspace
		deps.SandboxMaxGroupConcurrency = cfg.SandboxMaxGroupConcurrency
		deps.SandboxMaxActive = cfg.SandboxMaxActivePerWorkspace
		deps.SandboxMaxStorageBytes = int64(cfg.SandboxMaxStorageGBPerWorkspace) * store.BytesPerDecimalGB
		// Both defaults are digest-pinned (w7/m85): a sandbox template names an
		// image bex RUNS, so a floating tag let two creates months apart start
		// different code with no record of the change. The tag is retained ahead
		// of the digest for legibility; the digest is what resolves.
		// BEX_AGENT_SESSION_IMAGE is normally supplied by
		// lego/operator/config/api/deployment.yaml, which deploy.yml rewrites to
		// the digest it just pushed — this default is the floor for a deployment
		// that omits it, not the production value.
		deps.SandboxTemplates = sandboxTemplateRegistry(cfg.SandboxImage, cfg.AgentSessionImage)
		deps.SandboxDefaultPlan = sandbox.DefaultPlan
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
			ModelProxyPort: cfg.ModelProxyPort,
			// w10/m19: the full-internet posture admits the gateway egress proxy
			// on the port the sandbox's HTTPS_PROXY names (same source URL).
			EgressProxyPort: cfg.EgressProxyPort,
		})
		if err != nil {
			log.Fatalf("bex-api: %v", err)
		}
		deps.SandboxSessionEgress = egress
		// Render allow-list sandboxes (w32/m283) share the same policy client.
		deps.SandboxEgress = &sessionegress.SandboxManager{Client: cl}
		deps.AgentRegistryHosts = egress.RegistryDomains()
		// `render ea sandbox exec` (m33): bex-api authorizes can_operate and mints a
		// signed ticket, then reverse-proxies the SSE stream from the isolated SSH
		// gateway (which alone holds pods/exec, Option A). Both the shared HMAC
		// secret and the gateway's internal exec URL must be set, else the exec verb
		// 503s (create/list/stop are unaffected).
		if cfg.SandboxExecSecret != "" {
			if gwURL := cfg.SandboxExecURL; gwURL != "" {
				fileURL, err := url.Parse(gwURL)
				if err != nil {
					log.Fatal("bex-api: invalid sandbox gateway URL")
				}
				fileURL.Path, fileURL.RawPath = sandboxfiles.GatewayPath, ""
				fileURL.RawQuery, fileURL.Fragment = "", ""
				deps.SandboxExec = &sandbox.ExecConfig{
					Secret:         []byte(cfg.SandboxExecSecret),
					GatewayURL:     gwURL,
					FileGatewayURL: fileURL.String(),
					// No overall timeout (an exec stream is long-lived); dialing
					// and the gateway's answer are bounded. The gateway answers an
					// exec's headers on admission and a file transfer's once it is
					// written or refused (a refused upload destination is checked
					// before the body is streamed, w8/313), so 2m never cuts either
					// short.
					Client: sshgateway.NewUpstreamClient(2 * time.Minute),
					TTL:    60 * time.Second,
					// The pinned CLI's run connect-token handshake (w7/m147): the
					// minted `uri` is on the public API origin, and the token is
					// single-use across both replicas through the shared store (the
					// same shell_ticket_nonces claim the gateway uses; st nil ⇒
					// process-local, as for shells).
					PublicURL: cfg.APIPublicURL,
					Nonces:    &sshgateway.NonceGuard{Store: nonceStoreOrNil(st)},
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
			deps.SandboxStorage = st
			deps.SandboxMeter = &sandbox.Meter{Client: deps.SandboxClient, Store: st, StorageMetrics: sandbox.NewStorageMetrics(reg)}
			// A suspended sandbox pays for its retained image, sized from its manifest
			// in the registry. Unset, nothing is sized and nothing billed.
			if cfg.SandboxSnapshotRegistryURL != "" {
				registry := sandbox.RegistryConfig{
					BaseURL:   cfg.SandboxSnapshotRegistryURL,
					Prefix:    cfg.SandboxSnapshotRepoPrefix,
					Container: cfg.SandboxSnapshotContainer,
					User:      cfg.SandboxSnapshotRegistryUser,
					Password:  cfg.SandboxSnapshotRegistryPassword,
				}
				reaper := registry
				// The operator-minted Secret wins over the plain variables, and is read
				// again as it rotates. Deleting a gone sandbox's images needs its own
				// credential with the delete action; without one they stay (Zot
				// retention keeps five tags).
				reap := cfg.SandboxSnapshotRegistryGCUser != ""
				if cfg.SandboxSnapshotRegistrySecret != "" {
					key, _ := parseSecretRef(cfg.SandboxSnapshotRegistrySecret) // validated in config
					registry.Credentials = secretCredentials(cl, key, registrySecretReaderUserKey, registrySecretReaderPasswordKey, time.Now)
					reaper.Credentials = secretCredentials(cl, key, registrySecretReaperUserKey, registrySecretReaperPasswordKey, time.Now)
					reap = true
				} else {
					reaper.User, reaper.Password = cfg.SandboxSnapshotRegistryGCUser, cfg.SandboxSnapshotRegistryGCPass
				}
				deps.SandboxMeter.Sizer = &sandbox.RegistrySizer{RegistryConfig: registry}
				if reap {
					deps.SandboxMeter.Reaper = &sandbox.RegistryReaper{RegistryConfig: reaper}
				}
				// Imported sandbox images (w2/m358) are written by the operator's
				// importer user, so they need the operator-minted Secret; the
				// reaper deletes a deleted group's images (w2/m359).
				if cfg.SandboxSnapshotRegistrySecret != "" {
					key, _ := parseSecretRef(cfg.SandboxSnapshotRegistrySecret) // validated in config
					importer := registry
					importer.Credentials = secretCredentials(cl, key, registrySecretImporterUserKey, registrySecretImporterPasswordKey, time.Now)
					deps.SandboxImages = sandbox.NewImages(st, importer)
					deps.SandboxImages.Reaper = &sandbox.RegistryReaper{RegistryConfig: reaper}
					deps.SandboxImages.PlatformBudgetBytes = int64(cfg.SandboxImportPlatformBudgetGB) * store.BytesPerDecimalGB
				}
			}
			go deps.SandboxMeter.Run(ctx)
			// Idle suspension (w2/m278): exec/file activity is recorded in the
			// store; the sweeper itself starts once the Service exists.
			deps.SandboxActivity = st
			deps.SandboxNames = st
			deps.SandboxRecords = st
			deps.SandboxExecutions = st
			// Sandbox groups and their concurrency limits (w22/m311).
			deps.SandboxGroups = st
			// Workspace-delete sandbox teardown (w1/m61): stop the workspace's
			// running OpenSandbox sandboxes before the tenant row (and its sandbox
			// key, migration 0056) cascade away. It joins the PRE-cascade purgers so
			// the key still resolves; st satisfies sandbox.PurgeKeyLookup (lookup-only,
			// never minting). Only meaningful with the store on (workspace delete
			// needs it) and the client wired — both true in this branch.
			deps.WorkspacePreCascadePurgers = append(deps.WorkspacePreCascadePurgers,
				&sandbox.WorkspacePurger{Client: deps.SandboxClient, Keys: st, Images: deps.SandboxImages})
		}
	}
}

// wireReconcilers wires the apps.Service ↔ reconciler seams and starts the
// projector + namespace reconciler loops.
// wireAgentSessions reads the agent-session and Browser Web Shell environment
// contract onto deps: the shared gateway trust key, the browser-reachable
// origins, and the idle-close and live-sandbox lifecycle bounds.
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
	}
	deps.ShellWSURL = cfg.ShellWSURL
	deps.AgentSessionGatewayURL = cfg.AgentSessionGatewayURL
	// Optional override of the in-cluster Git smart-HTTP proxy origin. The
	// sandbox receives this non-secret URL, never a GitHub credential.
	deps.AgentGitProxyURL = cfg.AgentGitProxyURL
	// ADR095: nil when BEX_AGENT_ATTACHMENTS_S3_* is unset.
	deps.AgentAttachments = agentsessions.NewS3AttachmentObjects(cfg.AgentAttachments)
	// Sandbox lifecycle (w2/m67, w10/m15): idle grace before a finished
	// session's sandbox is terminated (default 2h; 0 ⇒ ADR054 D6 immediate
	// reap), and the per-workspace concurrent live-sandbox cap (default 5; 0 ⇒
	// uncapped).
	deps.AgentSandboxIdleTTL = cfg.AgentSandboxIdleTTL
	// w5/m80 t002: one turn's wall-clock bound, injected into the sandbox as
	// BEX_AGENT_TURN_TIMEOUT_MS. Default 30m; a 0/invalid value falls back to the
	// Service's 30m default (the bound is never disabled — that is the 4h-hang bug
	// this fixes). It is the ceiling over the inactivity bound, which one tool
	// call may pause only so long, and never outlives the sandbox.
	deps.AgentTurnTimeout = cfg.AgentTurnTimeout
	deps.AgentMetaRouteMaxSessions = cfg.AgentMetaRouteMaxSessions
	deps.AgentTurnIdleTimeout = cfg.AgentTurnIdleTimeout
	deps.AgentToolCallTimeout = cfg.AgentToolCallTimeout
	// Transient status-read failures fail a session only after this.
	deps.AgentStatusRetryWindow = cfg.AgentStatusRetryWindow
	deps.FreeInstanceHours = cfg.FreeInstanceHours
	deps.CrashLoopSuspendAfter = cfg.CrashLoopSuspendAfter
	deps.AgentMaxLiveSandboxesPerWorkspace = cfg.AgentMaxLiveSandboxesPerWorkspace
	deps.MaxBlueprintGroupings = cfg.MaxBlueprintGroupings
	deps.ImageCompatibilityWorkspaces = cfg.ImageCompatibilityWorkspaces
	deps.PrivateImagePortsDisabled = cfg.PrivateImagePortsDisabled
	// Round-11 #3: per-workspace env-group quota (default 100; 0 disables).
	deps.MaxEnvGroupsPerWorkspace = cfg.MaxEnvGroupsPerWorkspace
	// ADR075 §2: per-workspace GitHub-connection quota (default 10; 0 disables).
	deps.MaxGitConnectionsPerWorkspace = cfg.MaxGitConnectionsPerWorkspace
	deps.MaxWorkspacesPerGitInstallation = cfg.MaxWorkspacesPerGitInstallation
	// codex-security geyRc8 F1: per-workspace registry-credential quota.
	deps.MaxRegistryCredentialsPerWorkspace = cfg.MaxRegistryCredentialsPerWorkspace
	// codex-security round 18: custom-domain cardinality quotas (default 100
	// per service — the round-12 #3 routes/headers scale — and 500 per
	// workspace; 0 disables). Beyond either, claims are refused with
	// CUSTOM_DOMAIN_LIMIT.
	deps.MaxCustomDomainsPerService = cfg.MaxCustomDomainsPerService
	deps.MaxCustomDomainsPerWorkspace = cfg.MaxCustomDomainsPerWorkspace
}

func wireReconcilers(ctx context.Context, srv *api.Server, rec *store.Reconciler, st *store.PGStore, cl client.WithWatch, base *core.Base, cfg *Config) {
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
		// BuildFanout (w2/m254, ADR097 §5): a build source's live build deploys
		// to its linked auto-deploy services.
		rec.BuildFanout = srv.Deploys

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
		base.EnforceIsolation = nsRec.EnsureEnvironmentIsolation
		// Kick BOTH reconcilers on workspace create/delete for the same
		// low-latency reason the projector is kicked on app writes: the
		// projector prunes the deleted workspace's orphaned App CRs, the
		// namespace reconciler provisions/prunes its `<ws>` namespace(s).
		srv.Workspaces.Kick = func() {
			rec.Kick()
			nsRec.Kick()
		}
		go rec.Run(ctx)
		// The operator's status writes kick the projector (w5/m369): a deploy's
		// verdict reaches its row, events and webhooks within seconds instead of
		// on the next resync. Every replica watches, as every replica projects;
		// the store's transitions are idempotent across replicas.
		go (&store.AppStatusWatch{Client: cl, Kick: rec.KickFromWatch, Metrics: rec.Metrics}).Run(ctx)
		go nsRec.Run(ctx)
		go (&store.ProductInventoryCollector{Store: st, Client: cl, Identity: cpIdentity}).Run(ctx)
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
		if srv.AgentSessions != nil {
			whWorker.AgentOutcomes = agentsessions.TurnOutcomes{Service: srv.AgentSessions}
		}
		go whWorker.Run(ctx)
		// Agent automations (ADR101 D3): every replica ticks; the claim, the
		// run-row key and the idempotent session create keep a fire single.
		// It runs only when a session create could succeed.
		if srv.AgentSessions.AutomationsRunnable() {
			go (&agentsessions.AutomationScheduler{Service: srv.AgentSessions}).Run(ctx)
		}
		// Queued PR feedback is delivered as soon as its session is idle; the
		// lease keeps replicas from steering one session twice.
		if srv.PRFeedback != nil {
			go srv.PRFeedback.Run(ctx)
		}
		// Every replica reads due pull request states; the claim's SKIP LOCKED
		// keeps two from reading the same one.
		if srv.PRStates != nil {
			go srv.PRStates.Run(ctx)
		}
	}
	// Failure alert emails (w2/m252): a crashing service, a failed cron run, a
	// failed image pull or a failed Blueprint sync email the workspace, as
	// Render does. Without SMTP there is nothing to send, so no worker runs and
	// its cursors are seeded only once mail is configured.
	if n := srv.Notifications; st != nil && n != nil && n.Mailer != nil && n.Identities != nil {
		var cl client.Client
		if n.Base != nil {
			cl = n.Base.Client
		}
		go (&notifications.AlertEmailWorker{
			Store: st, Policy: notifications.AppNotificationPolicy(cl), Owners: notifications.StoredOwnerPolicy(st),
			Mailer: n.Mailer, Identities: n.Identities, DashboardBaseURL: n.DashboardBaseURL,
		}).Run(ctx)
	}
	// Agent automation emails (w9/m343, ADR101 D7): each run's outcome per the
	// automation's email setting, and every platform auto-pause. It runs even
	// without SMTP, so unmailed notices are consumed rather than mailed in bulk
	// once mail is configured.
	if st != nil {
		w := &notifications.AutomationEmailWorker{Store: st}
		if n := srv.Notifications; n != nil {
			w.Mailer, w.Identities, w.DashboardBaseURL = n.Mailer, n.Identities, n.DashboardBaseURL
		}
		go w.Run(ctx)
	}
	// Native push (ADR048 D2): only an explicitly configured, startup-validated
	// transport starts the event consumer. With config absent there is no client,
	// no worker, no feed advancement, and no provider network traffic.
	if st != nil && (mobilePush != nil || webPush != nil) {
		var cl client.Client
		if n := srv.Notifications; n != nil && n.Base != nil {
			cl = n.Base.Client
		}
		pushWorker := &notifications.PushWorker{
			Store: st, Sender: notifications.PushTransportSender{Transport: mobilePush, WebPush: webPush},
			Receipts: mobilePush, Metrics: pushMetrics, Evidence: notifications.PushEvidenceLogger{},
			// A service's notification level (its override, else the workspace
			// policy) applies to its pushes too (w3/m271).
			Owners: notifications.StoredOwnerPolicy(st), Policy: notifications.AppNotificationPolicy(cl),
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
	// BEX_AUTH_FAILURE_LIMIT=0 + BEX_AUTH_MAX_INFLIGHT=0 disables both rate
	// and global concurrency; BEX_AUTH_MAX_INFLIGHT_PER_CREDENTIAL=0 leaves
	// per-credential concurrency unbounded (default 64).
	authFailureRPM, authFailureBurst := cfg.AuthFailureRPM, cfg.AuthFailureBurst
	srv.AuthAdmission = api.NewAuthAdmission(authFailureRPM, authFailureBurst, cfg.AuthMaxInflight, cfg.AuthMaxInflightPerCredential)
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

// edgeUpstreamIdleTimeout is how long Traefik keeps an idle upstream
// connection to bex-api in its pool: its forwardingTimeouts.idleConnTimeout
// default, which deploy/gitops does not override (no ServersTransport).
const edgeUpstreamIdleTimeout = 90 * time.Second

// newHTTPServer builds an HTTP server with the header/read/write/idle timeouts
// shared by the public and internal control-plane listeners.
//
// IdleTimeout must outlive the edge's upstream pool. Left zero, net/http uses
// ReadTimeout (30s), so bex-api closed idle keep-alive connections that Traefik
// still considered reusable; a POST written onto one as it closed failed with
// no retry (GraphQL is always POST) and reached the browser as a CORS-less 502
// or a closed connection — the intermittent dropped mutations of w3/044.
func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      api.HTTPWriteTimeout,
		IdleTimeout:       edgeUpstreamIdleTimeout + 30*time.Second,
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

// nonceStoreOrNil keeps a nil *store.PGStore a nil NonceStore interface: a typed
// nil boxed into the interface would be non-nil and every claim would panic.
func nonceStoreOrNil(st *store.PGStore) sshgateway.NonceStore {
	if st == nil {
		return nil
	}
	return st
}

// machineMembershipStore is the slice of the control-plane store the
// machine-binding reconcile needs (w5/m103). *store.PGStore satisfies it; a
// DB-less run leaves it nil and the reconcile is skipped.
type machineMembershipStore interface {
	MarkMachineMemberships(ctx context.Context, clientIDs []string) (int64, error)
}

// backfillMachineMemberships reclassifies the tenant_members rows of existing
// API keys as kind='machine' — the one-time reconcile w5/m103's migration
// cannot do in SQL, because Hydra is the only registry that knows which
// subjects are client ids.
//
// Best-effort by design: without it a pre-existing key keeps appearing on the
// Team surface and holding a seat, which is precisely the pre-m103 behavior, so
// a failure here degrades to the status quo rather than to anything broken. It
// is logged, never fatal, and retried on the next start.
func backfillMachineMemberships(ctx context.Context, st any, keys apikeys.APIKeyStore) {
	marker, ok := st.(machineMembershipStore)
	if !ok || keys == nil {
		return
	}
	list, err := keys.List(ctx)
	if err != nil {
		log.Printf("bex-api: machine-membership backfill: list API keys: %v (retrying next start)", err)
		return
	}
	ids := make([]string, 0, len(list))
	for _, k := range list {
		ids = append(ids, k.ID)
	}
	n, err := marker.MarkMachineMemberships(ctx, ids)
	if err != nil {
		log.Printf("bex-api: machine-membership backfill: %v (retrying next start)", err)
		return
	}
	if n > 0 {
		log.Printf("bex-api: machine-membership backfill: reclassified %d API-key binding(s) as machine", n)
	}
}

// compactAgentTranscripts runs the transcript item-model backfill: a dry run
// first (its counts are the logged plan), then the rewrite. A failure is logged
// and retried on the next start; the legacy rows it leaves still replay
// unchanged.
func compactAgentTranscripts(ctx context.Context, st *store.PGStore) {
	report := func(phase string, stats store.TranscriptCompactionStats) {
		log.Printf("bex-api: agent transcript compaction %s: sessions=%d turns=%d skipped_open_turns=%d rows %d→%d bytes %d→%d truncation_flags_cleared=%d",
			phase, stats.Sessions, stats.Turns, stats.SkippedOpenTurns, stats.RowsBefore, stats.RowsAfter, stats.BytesBefore, stats.BytesAfter, stats.FlagsCleared)
	}
	plan, err := st.CompactAgentSessionTranscripts(ctx, store.TranscriptCompactionOptions{DryRun: true})
	if err != nil {
		log.Printf("bex-api: agent transcript compaction dry run failed (retrying next start): %v", err)
		return
	}
	if plan.Turns == 0 {
		return
	}
	report("dry run", plan)
	done, err := st.CompactAgentSessionTranscripts(ctx, store.TranscriptCompactionOptions{Logf: log.Printf})
	if err != nil {
		log.Printf("bex-api: agent transcript compaction stopped (retrying next start): %v", err)
	}
	report("applied", done)
}

// backfillModelKeyFlags is the one-shot startup reconcile that records the
// flag for model keys put in OpenBao by hand before the write-only key field
// existed (agentsessions.BackfillModelKeyFlags). It runs in the background
// and logs counts only; a failure is retried on the next start.
func backfillModelKeyFlags(ctx context.Context, keys core.SecretKV, st *store.PGStore) {
	r, err := agentsessions.BackfillModelKeyFlags(ctx, keys, st)
	if err != nil {
		log.Printf("bex-api: agent model key flag backfill failed (retrying next start): %v", err)
	}
	log.Printf("bex-api: agent model key flag backfill: workspaces=%d keys=%d flags_set=%d platform_routed=%d failed=%d", r.Workspaces, r.Keys, r.Marked, r.PlatformRouted, r.Failed)
}

// backfillBillingTrust is the startup pass that counts the paid invoices the
// invoice.paid webhook never recorded (billing.Trust.Backfill), on one replica
// at a time; a failure is retried on the next start.
func backfillBillingTrust(ctx context.Context, st *store.PGStore, stripeClient *billing.StripeClient, trust *billing.Trust) {
	_, err := st.RunExclusive(ctx, "billing:trust-backfill", func(ctx context.Context) error {
		const limit = 500
		mappings, err := st.ListBillingProviderMappings(ctx, stripeClient.ExpectedLivemode(), limit)
		if err != nil {
			return err
		}
		if len(mappings) == limit {
			log.Printf("bex-api: billing trust backfill reads only the first %d billing mappings", limit)
		}
		counted, err := trust.Backfill(ctx, mappings, stripeClient)
		log.Printf("bex-api: billing trust backfill: counted %d paid invoices across %d workspaces", counted, len(mappings))
		return err
	})
	if err != nil {
		log.Printf("bex-api: billing trust backfill failed (retrying next start): %v", err)
	}
}

// backfillSkillVersionContent is the one-shot startup pass that re-renders the
// skill versions migration 0210 backfilled in SQL with the Go renderer's YAML
// quoting (agentsessions.BackfillSkillVersionContent). It runs in the
// background and logs a count; a failure is retried on the next start.
func backfillSkillVersionContent(ctx context.Context, st *store.PGStore) {
	n, err := agentsessions.BackfillSkillVersionContent(ctx, st)
	if err != nil {
		log.Printf("bex-api: skill version content backfill failed (retrying next start): %v", err)
	}
	log.Printf("bex-api: skill version content backfill: rewritten=%d", n)
}
