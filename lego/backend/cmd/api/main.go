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
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/mailer"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/notifications"
	pushtransport "github.com/bex-co/bex/lego/backend/internal/notifications/push"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
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

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// agentRecordURL derives the gateway's headless-transcript-recorder URL from its
// sandbox-exec URL (ADR051): both endpoints live on the same internal gateway
// listener and differ only in the final path segment. Empty in => empty out.
func agentRecordURL(execURL string) string {
	if execURL == "" {
		return ""
	}
	return strings.TrimSuffix(execURL, "/sandbox-exec") + "/agent-record"
}

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
	requirePaymentMethod, err := paymentMethodGate(os.Getenv)
	if err != nil {
		log.Fatalf("bex-api: %v", err)
	}
	mobilePush, err := pushtransport.New(pushtransport.Config{
		Provider: os.Getenv("BEX_PUSH_PROVIDER"), AccessToken: os.Getenv("BEX_EXPO_PUSH_ACCESS_TOKEN"),
		Endpoint: os.Getenv("BEX_EXPO_PUSH_URL"),
	})
	if err != nil {
		log.Fatalf("bex-api: push config: %v", err)
	}
	metricRegistry := prometheus.NewRegistry()
	metricRegistry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	billingMetrics := billing.NewMetrics(metricRegistry)
	pushMetrics := notifications.NewPushMetrics(metricRegistry)
	pushMetrics.SetEnabled(mobilePush != nil)

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

	// One readiness flag for the whole pod (w1/m52): the public server's
	// /readyz answers 200 until SIGTERM, then 503 while both servers keep
	// serving through the drain window. The readiness probe points here;
	// liveness stays on /healthz, which never flips.
	ready := &serve.Readiness{}

	deps := api.Deps{
		PushAvailable: mobilePush != nil,
		// BEX_BASE_DOMAIN names custom-domain DNS targets `<app>.<base>` (docs/ADR005-custom-domain.md);
		// unset falls back to deriving the platform host from an App's status URLs.
		BaseDomain: os.Getenv("BEX_BASE_DOMAIN"),
		// BEX_REGION is the explicit platform placement surfaced in Render
		// resource metadata. Empty is honestly omitted.
		Region:        os.Getenv("BEX_REGION"),
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
	// promURL is also used by the usage metering block below.
	promURL := os.Getenv("BEX_PROM_URL")
	if promURL != "" {
		deps.RequestMetrics = metrics.NewPrometheusRequestSource(promURL, nil)
		deps.ResourceMetricsRange = metrics.NewPrometheusResourceSource(promURL, nil)
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
	var ghClient *github.Client
	if appID, key, slug := os.Getenv("BEX_GITHUB_APP_ID"), os.Getenv("BEX_GITHUB_APP_PRIVATE_KEY"), os.Getenv("BEX_GITHUB_APP_SLUG"); appID != "" && key != "" && slug != "" {
		var err error
		ghClient, err = github.NewClient(github.Config{
			AppID: appID, PrivateKey: key, Slug: slug,
			// F2: optional OAuth credentials enable the installation-admin verifier
			// on the browser connect callback. Both set (and the App configured to
			// request user authorization on install) => a workspace admin can only
			// connect an installation they actually administer.
			ClientID:     os.Getenv("BEX_GITHUB_APP_CLIENT_ID"),
			ClientSecret: os.Getenv("BEX_GITHUB_APP_CLIENT_SECRET"),
		})
		if err != nil {
			log.Fatalf("bex-api: github app config: %v", err)
		}
		deps.GitHubClient = ghClient
		if ghClient.InstallVerificationConfigured() {
			deps.GitHubInstallVerifier = ghClient
		}
		// The same out-of-band private key also HMAC-signs the short-lived
		// workspace state carried through GitHub's browser install redirect.
		deps.GitHubStateSecret = []byte(key)
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
	// w1/m53 + w1/m65 F16: with the store on but OpenFGA off, checkAuthz allows
	// every relation (fail-open). This is worse than "roles unenforced within a
	// workspace": the explicit-workspace verbs (workspaces/members/projects) call
	// AuthorizeOn/AuthorizeOnTarget with an arbitrary workspace object and do NOT
	// pass through the membership-resolving WithWorkspace path, so cross-tenant
	// isolation does NOT hold either — any authenticated caller can read/mutate
	// another workspace. A multi-tenant API must therefore FAIL CLOSED (refuse to
	// start) rather than warn. BEX_ALLOW_INSECURE_AUTHZ=1 is the documented
	// single-member/local-dev override (mirrors BEX_CP_INSECURE for BEX_CP_TOKEN).
	if base.Authz == nil && os.Getenv("BEX_CP_DB_URI") != "" && !mcpStdio() {
		if os.Getenv("BEX_ALLOW_INSECURE_AUTHZ") != "1" {
			log.Fatal("BEX_OPENFGA_URL is unset while the control-plane store is on (BEX_CP_DB_URI set): authorization would be FAIL-OPEN — every workspace member gets admin-equivalent rights AND explicit-workspace verbs bypass membership resolution, so cross-tenant isolation does not hold. Refusing to start a multi-tenant API without enforced authorization. Set BEX_OPENFGA_URL, or set BEX_ALLOW_INSECURE_AUTHZ=1 to override in single-member/local dev only (docs/ADR012-auth.md).")
		}
		log.Printf("WARNING: BEX_OPENFGA_URL is unset while the control-plane store is on and BEX_ALLOW_INSECURE_AUTHZ=1 — authorization is FAIL-OPEN (every member admin-equivalent; explicit-workspace verbs bypass membership isolation). Safe ONLY for a single-member workspace / local dev.")
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
	// st is hoisted like rec: the outbound-webhook delivery worker (w3/m11) is
	// constructed after NewServer (it shares deps.Mailer + the identity email
	// lookup) but reads/writes the store directly.
	var st *store.PGStore
	var stripeLifecycleWorker *billing.Worker
	var stripeLifecycleReconciler *billing.Reconciler
	var stripeBillingAdmin store.BillingAdmin
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
		if err := store.CheckOwnership(ctx, pool); err != nil {
			log.Fatalf("bex-api: %v", err)
		}

		st = store.NewPGStore(pool)
		rec = store.NewReconciler(cl, st)
		if d := os.Getenv("BEX_CP_RESYNC"); d != "" {
			v, err := time.ParseDuration(d)
			if err != nil {
				log.Fatalf("bex-api: bad BEX_CP_RESYNC %q: %v", d, err)
			}
			rec.Resync = v
		}
		// rec.Run is started after NewServer below, so CloneSecrets is set before
		// the first reconcile pass (w2/m11).
		deps.Store = st        // single writer of intent: suspend/resume write the row first
		deps.SSHKeysStore = st // identity-scoped SSH public-key registry
		deps.DeployStore = st  // deploy history (w2/m5): list/get/trigger read+write the same rows
		// Cancel (w2/m10) needs to compute a repo-backed App's in-flight build
		// Job's identity — must match the operator's own BEX_BUILD_NAMESPACE.
		deps.DeployBuildNamespace = os.Getenv("BEX_BUILD_NAMESPACE")
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
		tenantSvc.RequireVerifiedInviteEmail = os.Getenv("BEX_REQUIRE_VERIFIED_INVITE_EMAIL") != "0" &&
			!strings.EqualFold(os.Getenv("BEX_REQUIRE_VERIFIED_INVITE_EMAIL"), "false")
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

		// Billing (w7/m47–m50, ADR040): one Stripe client drives both the
		// seal-then-emit meter-event sidecar and the usage surface's invoice
		// read-back. BEX_STRIPE_SECRET_KEY unset means no client, emitter, reader,
		// or public Stripe webhook: estimate-only behavior stays unchanged.
		stripeSecretKey, billingEpoch, stripeEnabled, err := stripeBillingGate(os.Getenv, time.Now())
		if err != nil {
			log.Fatalf("bex-api: %v", err)
		}
		if stripeEnabled {
			billingMetrics.SetEnabled(true)
			stripeClient := billing.NewStripe(billing.StripeConfig{
				SecretKey:             stripeSecretKey,
				BaseURL:               os.Getenv("BEX_STRIPE_API_URL"),
				BillingEpoch:          billingEpoch,
				CompCouponID:          os.Getenv("BEX_STRIPE_COMP_COUPON_ID"),
				DashboardURL:          os.Getenv("BEX_DASHBOARD_URL"),
				PortalConfigurationID: os.Getenv("BEX_STRIPE_PORTAL_CONFIGURATION_ID"),
				TaxCode:               os.Getenv("BEX_STRIPE_TAX_CODE"),
				TaxBehavior:           os.Getenv("BEX_STRIPE_TAX_BEHAVIOR"),
				State:                 st,
				Metrics:               billingMetrics,
			})
			usageSvc.Billing = stripeClient
			deps.Billing = stripeClient
			deps.BillingState = st
			if requirePaymentMethod {
				base.Payment = &billing.PaymentGate{Store: st}
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
			emitter.RequirePaymentMethod = requirePaymentMethod
			if v := os.Getenv("BEX_STRIPE_SEAL_HOURS"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n >= 1 {
					emitter.SealHours = time.Duration(n) * time.Hour
				} else {
					log.Printf("BEX_STRIPE_SEAL_HOURS=%q invalid (want integer ≥ 1); using default %s", v, billing.DefaultSealHours)
				}
			}
			// w7/m57: never seal shorter than the usage rollup's catch-up window, or
			// an exported row could still be rewritten and never re-emitted.
			if clamped := billing.ClampSealHours(emitter.SealHours, usage.CatchupWindow); clamped != emitter.SealHours {
				log.Printf("BEX_STRIPE_SEAL_HOURS=%s is below the usage catch-up window; raising the seal horizon to %s so exported rows are final", emitter.SealHours, clamped)
				emitter.SealHours = clamped
			}
			var lifecycle *billing.Lifecycle
			if os.Getenv("BEX_STRIPE_DUNNING_ENABLED") == "1" {
				if stripeClient.ExpectedLivemode() {
					log.Fatal("bex-api: BEX_STRIPE_DUNNING_ENABLED=1 is test-mode only at w7/m52; refusing live enforcement")
				}
				grace, err := time.ParseDuration(envOr("BEX_STRIPE_GRACE_PERIOD", "168h"))
				if err != nil || grace < time.Minute {
					log.Fatalf("bex-api: BEX_STRIPE_GRACE_PERIOD must be a duration >= 1m: %v", err)
				}
				reconcileEvery, err := time.ParseDuration(envOr("BEX_STRIPE_RECONCILE_INTERVAL", "5m"))
				if err != nil || reconcileEvery < time.Minute {
					log.Fatalf("bex-api: BEX_STRIPE_RECONCILE_INTERVAL must be a duration >= 1m: %v", err)
				}
				lifecycle = &billing.Lifecycle{Store: st, GracePeriod: grace, ExpectedLivemode: false}
				enforcer := &billing.KubernetesEnforcer{Client: cl, Store: st, Namespace: envOr("BEX_CP_APPS_NAMESPACE", base.Namespace)}
				stripeLifecycleWorker = &billing.Worker{Store: st, Enforcer: enforcer}
				stripeLifecycleReconciler = &billing.Reconciler{Store: st, Provider: stripeClient, GracePeriod: grace, Interval: reconcileEvery, Metrics: billingMetrics}
				log.Printf("bex-api Stripe test-mode dunning enabled (grace %s, reconcile %s)", grace, reconcileEvery)
			}
			if secret := os.Getenv("BEX_STRIPE_WEBHOOK_SECRET"); secret != "" {
				handler := &billing.StripeWebhook{Secret: secret, ExpectedLivemode: stripeClient.ExpectedLivemode(), OnCheckoutCompleted: stripeClient.CompleteCheckoutSession, Metrics: billingMetrics}
				if lifecycle != nil {
					handler.OnLifecycle = lifecycle.HandleStripeEvent
				}
				deps.StripeWebhook = handler
			}
			log.Printf("bex-api Stripe Billing enabled (seal horizon %s, epoch %s, webhook %t)", emitter.SealHours, billingEpoch.Format(time.RFC3339), deps.StripeWebhook != nil)
			go emitter.Run(ctx)
		}

		go usageSvc.Run(ctx)

		// Audit log retention (w4/m10 + w2/m39): purges audit_events and SSH
		// session metadata older than BEX_AUDIT_RETENTION_DAYS daily, same
		// cadence/shape as usage's compaction loop above. The write side is
		// base.Audit (wired above); this Service is the read verb + sweep only.
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

		// The internal control-plane API (:8091) grants workspace-admin and
		// cross-tenant writes, so it must never serve unauthenticated. Fail
		// closed at startup when BEX_CP_TOKEN is empty (w1/m53: the token was
		// set nowhere in prod, so the API had been serving open behind the
		// NetworkPolicy alone). BEX_CP_INSECURE=1 is a loud local-dev override.
		cpToken := os.Getenv("BEX_CP_TOKEN")
		if err := requireCPAuth(cpToken, os.Getenv("BEX_CP_INSECURE")); err != nil {
			log.Fatalf("control plane: %v", err)
		}
		internal := &store.API{Store: st, Kick: rec.Kick, Health: st.Ping, Token: cpToken, Grant: granter, Billing: stripeBillingAdmin, BillingOperations: st, SandboxTenants: st}
		internalRoot := http.NewServeMux()
		internalRoot.Handle("GET /metrics", promhttp.HandlerFor(metricRegistry, promhttp.HandlerOpts{}))
		// ADR047 D2: a gateway-authenticated, internal-only mint verb. The same
		// sandbox-exec HMAC secret is reused with protocol domain separation; the
		// route is not mounted on :8090 and never enters the public surface.
		if secret := os.Getenv("BEX_SANDBOX_EXEC_SECRET"); secret != "" {
			internalRoot.Handle(agentsession.InternalMintPath, &agentsession.Handler{
				Secret: []byte(secret),
				Minter: &agentsession.Minter{GitHub: ghClient, Connections: st, Audit: st},
			})
		}
		internalRoot.Handle("/", internal.Handler())
		cpAddr := envOr("BEX_CP_ADDR", ":8091")
		cpSrv := &http.Server{
			Addr:              cpAddr,
			Handler:           internalRoot,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		}
		log.Printf("bex-api control plane (source of truth) on %s (Apps project into per-tenant <ws> namespaces; Databases/KeyValues stay in %q)", cpAddr, appsNS)
		go func() {
			// Same serve-then-graceful-shutdown pattern as the public server
			// below: on SIGTERM (ctx cancelled) the internal API drains instead
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
	// browser back to dashboard settings on success and with a bounded error code
	// on state/install failures.
	deps.DashboardURL = os.Getenv("BEX_DASHBOARD_URL")
	deps.DeployHookBaseURL = os.Getenv("BEX_API_PUBLIC_URL")
	deps.SSHHost = os.Getenv("BEX_SSH_HOST")
	// Hosted agent sandboxes (pillar 5, ADR042/w3/m32): wire the OpenSandbox
	// lifecycle client only when BEX_OPENSANDBOX_URL is set — unset => the
	// sandbox verbs 503 and the feature is not registered (byte-identical). The
	// per-workspace tenant-key provider (m32 t006) and template registry are
	// wired as they land; for now a single default "base" template.
	if osURL := os.Getenv("BEX_OPENSANDBOX_URL"); osURL != "" {
		deps.SandboxClient = sandbox.NewClient(osURL)
		deps.SandboxTemplates = sandboxTemplateRegistry(
			envOr("BEX_SANDBOX_IMAGE", "docker.io/library/alpine:3"),
			envOr("BEX_AGENT_SESSION_IMAGE", "ghcr.io/bex-co/bex-agent-sandbox:latest"),
		)
		deps.SandboxDefaultPlan = sandbox.PlanStarter
		// The Render CLI's `ea sandbox create` sends no template (no such flag), so
		// an empty template resolves to this registered default (w3/m32 t009).
		deps.SandboxDefaultTemplate = "base"
		// Agent-session egress (ADR047 D5): policy is installed before sandbox
		// creation. The registry catalog is platform config; the model endpoint is
		// selected per session by its agent/provider config and validated at create.
		egress, err := sessionegress.NewManager(cl, sessionegress.Config{
			SetupRegistryDomains: sessionegress.RegistryConfig(os.Getenv("BEX_AGENT_SETUP_REGISTRIES")),
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
		if secret := os.Getenv("BEX_SANDBOX_EXEC_SECRET"); secret != "" {
			if gwURL := os.Getenv("BEX_SANDBOX_EXEC_URL"); gwURL != "" {
				deps.SandboxExec = &sandbox.ExecConfig{
					Secret:     []byte(secret),
					GatewayURL: gwURL,
					// The headless transcript recorder (ADR051) shares the exec secret
					// and the same internal gateway listener; its URL is the exec URL
					// with the path swapped, so no new env var is needed. An explicit
					// BEX_AGENT_RECORD_URL overrides it if the paths ever diverge.
					RecordURL: envOr("BEX_AGENT_RECORD_URL", agentRecordURL(gwURL)),
					Client:    &http.Client{}, // no timeout: the exec stream is long-lived
					TTL:       60 * time.Second,
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
	// Browser Web Shell (docs/ADR035-ssh.md § Browser Web Shell): the HMAC key
	// shared only with the isolated gateway and the browser-reachable gateway
	// WebSocket origin. Either unset => the ticket verb returns 503 and native
	// `ssh` is unaffected.
	if secret := os.Getenv("BEX_SHELL_TICKET_SECRET"); secret != "" {
		deps.ShellTicketSecret = []byte(secret)
		// Agent attach deliberately reuses the Browser Web Shell trust key and
		// DB-backed nonce design. The claims are a distinct ticket type and bind
		// the session sandbox pod/workspace instead of a service instance.
		deps.AgentSessionTicketSecret = []byte(secret)
	}
	deps.ShellWSURL = os.Getenv("BEX_SHELL_WS_URL")
	deps.AgentSessionGatewayURL = os.Getenv("BEX_AGENT_SESSION_GATEWAY_URL")
	// Optional override of the in-cluster gateway git-credential broker URL the
	// sandbox helper calls (ADR047 D2); empty => the default internal gateway URL.
	deps.AgentCredentialURL = os.Getenv("BEX_AGENT_CREDENTIAL_URL")

	srv := api.NewServer(base, deps)
	// The agent-session Completer finalizes fire-and-forget sessions: it opens the
	// draft PR + records evidence for completed turns (ADR047 D4, w3/m41). It is a
	// no-op unless the store, OpenSandbox, and GitHub App are all wired.
	go srv.AgentSessionCompleter.Run(ctx)
	if stripeLifecycleWorker != nil {
		stripeLifecycleWorker.Notifier = notifications.BillingNotifier{Service: srv.Notifications}
		go stripeLifecycleWorker.Run(ctx)
		go stripeLifecycleReconciler.Run(ctx)
	}

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
		nsRec := store.NewNamespaceReconciler(cl, st)
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
	// Outbound event webhooks (w3/m11): the delivery worker tails the composed
	// event feed (deploys + audit_events + service_event_facts — the same rows the events feed reads)
	// through a durable watermark and POSTs signed notifications to subscribed
	// endpoints, with retry/auto-disable. Store off => no worker (the CRUD verbs
	// already 503 via deps.WebhookStore above). Failure notices reuse the SMTP
	// relay + Kratos email lookup the notifications feature uses.
	// BEX_WEBHOOK_BACKOFF ("5s,10s,1m") overrides the documented retry schedule
	// — a dev/verification knob, unset in production.
	if st != nil {
		backoff, err := webhooks.ParseBackoff(os.Getenv("BEX_WEBHOOK_BACKOFF"))
		if err != nil {
			log.Fatalf("bex-api: %v", err)
		}
		whWorker := &webhooks.Worker{Store: st, Mailer: deps.Mailer, Emails: srv.Notifications.Identities, Backoff: backoff}
		go whWorker.Run(ctx)
	}
	// Native push (ADR048 D2): only an explicitly configured, startup-validated
	// transport starts the event consumer. With config absent there is no client,
	// no worker, no feed advancement, and no provider network traffic.
	if st != nil && mobilePush != nil {
		pushWorker := &notifications.PushWorker{
			Store: st, Sender: notifications.PushTransportSender{Transport: mobilePush},
			Receipts: mobilePush, Metrics: pushMetrics, Evidence: notifications.PushEvidenceLogger{},
		}
		go pushWorker.Run(ctx)
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

	// Device-flow rate limiting (w4/m31/t002). BEX_DEVICE_RATE_LIMIT=0
	// disables it. Set here, before Handler(), like every other cap: Handler()
	// wires this onto the cliauth.Service it constructs internally (it can't
	// be built any earlier, since it needs the auth gate's invalidate
	// callback, which is also Handler()-scoped) — but the limiter itself only
	// needs the env-derived rpm/burst, so there's no reason its configuration
	// should wait until after Handler() the way its consumer's construction
	// must.
	deviceRPMStr := envOr("BEX_DEVICE_RATE_LIMIT", "30")
	deviceRPM, err := strconv.ParseFloat(deviceRPMStr, 64)
	if err != nil {
		log.Fatalf("bex-api: bad BEX_DEVICE_RATE_LIMIT %q: %v", deviceRPMStr, err)
	}
	deviceBurst, _ := strconv.Atoi(envOr("BEX_DEVICE_RATE_BURST", "0"))
	srv.DeviceRateLimiter = cliauth.NewDeviceRateLimiter(deviceRPM, deviceBurst)

	// Webhook intake rate limiting (w7/m60). The two unauthenticated intakes
	// (POST /v1/webhooks/git, /v1/webhooks/stripe) mount outside the auth gate, so
	// neither BEX_RATE_LIMIT nor BEX_DEVICE_RATE_LIMIT reaches them; this IP-keyed
	// limiter sheds a flood with 429 before the body read + HMAC verification.
	// Default 600/min per IP is deliberately generous: no legitimate GitHub/Stripe
	// delivery pattern hits 10 req/s from one source IP, and both senders retry, so
	// an unlucky burst-shed self-heals — only an abusive flood is turned away.
	// BEX_WEBHOOK_RATE_LIMIT=0 disables it (byte-identical to before m60).
	webhookRPMStr := envOr("BEX_WEBHOOK_RATE_LIMIT", "600")
	webhookRPM, err := strconv.ParseFloat(webhookRPMStr, 64)
	if err != nil {
		log.Fatalf("bex-api: bad BEX_WEBHOOK_RATE_LIMIT %q: %v", webhookRPMStr, err)
	}
	webhookBurst, _ := strconv.Atoi(envOr("BEX_WEBHOOK_RATE_BURST", "0"))
	srv.WebhookRateLimiter = api.NewRateLimiter(webhookRPM, webhookBurst) // nil when rpm=0

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

	// /readyz mounts OUTSIDE the product handler (w1/m52): it is a deployment
	// lifecycle endpoint for the readiness probe, not part of the Render-shaped
	// API surface, so internal/api stays untouched and parity-clean. Everything
	// else falls through to the product handler unchanged.
	root := http.NewServeMux()
	root.Handle("GET /readyz", ready.Handler())
	root.Handle("/", handler)

	addr := envOr("BEX_API_ADDR", ":8090")
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
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
