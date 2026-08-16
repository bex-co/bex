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

// modelProxyPort derives the sessionegress model-proxy port from the
// BEX_AGENT_MODEL_PROXY_URL origin (ADR062), so the egress narrowing and the
// agentsessions Service always agree on the port. Empty ⇒ 0 and agent-session
// mutation stays unavailable; a URL without an explicit port defaults to the
// gateway's :8084 listener. A malformed URL or port fails startup.
func modelProxyPort(raw string) uint16 {
	if raw == "" {
		return 0
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		log.Fatalf("bex-api: bad BEX_AGENT_MODEL_PROXY_URL %q: %v", raw, err)
	}
	port := u.Port()
	if port == "" {
		return 8084
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		log.Fatalf("bex-api: bad BEX_AGENT_MODEL_PROXY_URL port %q", port)
	}
	return uint16(n)
}

// rateLimitEnv parses one of the requests/min rate-limit env pairs: the fill
// rate (startup-fatal when malformed) plus its best-effort burst companion.
func rateLimitEnv(limitVar, limitDef, burstVar, burstDef string) (float64, int) {
	raw := envOr(limitVar, limitDef)
	rpm, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		log.Fatalf("bex-api: bad %s %q: %v", limitVar, raw, err)
	}
	return rpm, intEnv(burstVar, burstDef)
}

// intEnv parses an integer tuning knob best-effort: unset ⇒ the default,
// malformed ⇒ 0 (each such knob's documented disabled/default sentinel).
func intEnv(name, def string) int {
	n, _ := strconv.Atoi(envOr(name, def))
	return n
}

// positiveIntEnv parses an integer ≥ 1 tuning knob, warning (so the caller
// keeps its default) when the variable is set but invalid; unset is silently
// ok=false.
func positiveIntEnv(name string, def any) (int, bool) {
	v := os.Getenv(name)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		log.Printf("%s=%q invalid (want integer ≥ 1); using default %v", name, v, def)
		return 0, false
	}
	return n, true
}

// zeroableDurationEnv parses a duration tuning knob that accepts an explicit
// disabling zero: unset ⇒ def; a valid non-negative duration ⇒ that value (0 is
// a legal "disabled"); malformed or negative ⇒ warn and keep def.
func zeroableDurationEnv(name string, def time.Duration) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		log.Printf("%s=%q invalid (want a non-negative Go duration, 0 to disable); using default %s", name, v, def)
		return def
	}
	return d
}

// zeroableIntEnv parses an integer tuning knob that accepts an explicit
// disabling zero: unset ⇒ def; a valid n ≥ 0 ⇒ n (0 disables); invalid ⇒ def.
func zeroableIntEnv(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		log.Printf("%s=%q invalid (want an integer >= 0, 0 to disable); using default %d", name, v, def)
		return def
	}
	return n
}

// minuteDurationEnv parses a duration env var with a floor of one minute,
// startup-fatal when malformed or shorter.
func minuteDurationEnv(name, def string) time.Duration {
	d, err := time.ParseDuration(envOr(name, def))
	if err != nil || d < time.Minute {
		log.Fatalf("bex-api: %s must be a duration >= 1m: %v", name, err)
	}
	return d
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
	cpDBURI := os.Getenv("BEX_CP_DB_URI")
	dashboardURL := os.Getenv("BEX_DASHBOARD_URL")
	sandboxExecSecret := os.Getenv("BEX_SANDBOX_EXEC_SECRET")
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
	promURL := wireObservability(&deps)
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
	ghClient := wireGitHubApp(&deps)
	// Owner/member identity attributes (w6/m2): Kratos' admin API, distinct from
	// the public BEX_KRATOS_URL session whoami above — looking up OTHER members'
	// email/MFA needs the admin API, not a session. Unset => those fields omitted.
	if kratosAdmin := os.Getenv("BEX_KRATOS_ADMIN_URL"); kratosAdmin != "" {
		deps.Identities = workspaces.NewKratosIdentities(kratosAdmin)
	}
	authzChecker := wireAuthz(base, cpDBURI)

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
	if cpDBURI != "" && !mcpStdio() {
		// The datastore (Database/KeyValue) projection namespace; falls back to
		// BEX_API_NAMESPACE so the two agree unless explicitly split.
		appsNS := envOr("BEX_CP_APPS_NAMESPACE", base.Namespace)
		pool := openControlPlaneDB(ctx, cpDBURI)
		defer pool.Close()

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
		granter := wireControlPlaneFeatures(&deps, base, st, rec, authzChecker)

		// Usage metering (w8/m1) + retention (m4): the loop rolls usage_hourly
		// rows up every hour (needs Prometheus; skipped without it) and compacts
		// months older than the hot window into usage_monthly daily. The hot
		// window is BEX_USAGE_RETENTION_MONTHS calendar months (current month
		// included; default 3, minimum 1) — docs/ADR023-usage-metering.md.
		usageSvc := usage.NewService(base, st, promURL, nil)
		if n, ok := positiveIntEnv("BEX_USAGE_RETENTION_MONTHS", usage.DefaultRetentionMonths); ok {
			usageSvc.RetentionMonths = n
		}
		deps.Usage = usageSvc

		stripeLifecycleWorker, stripeLifecycleReconciler, stripeBillingAdmin = wireStripeBilling(ctx, &deps, base, cl, st, appsNS, usageSvc, billingMetrics, requirePaymentMethod, dashboardURL)

		go usageSvc.Run(ctx)

		// Audit log retention (w4/m10 + w2/m39): purges audit_events and SSH
		// session metadata older than BEX_AUDIT_RETENTION_DAYS daily, same
		// cadence/shape as usage's compaction loop above. The write side is
		// base.Audit (wired above); this Service is the read verb + sweep only.
		auditSvc := &audit.Service{Base: base, Store: st}
		if n, ok := positiveIntEnv("BEX_AUDIT_RETENTION_DAYS", audit.DefaultRetentionDays); ok {
			auditSvc.RetentionDays = n
		}
		deps.Audit = auditSvc
		go auditSvc.Run(ctx)

		startControlPlaneServer(ctx, st, rec, granter, stripeBillingAdmin, metricRegistry, ghClient, deps.Secrets, sandboxExecSecret, appsNS, ready)
	}

	// Invite delivery (w4/m12): the members feature emails invites over the same
	// SMTP relay Kratos's courier uses (SendGrid in prod, Mailpit locally). Unset
	// BEX_SMTP_ADDR/BEX_SMTP_FROM => mailer nil, invites recorded but not emailed.
	// BEX_DASHBOARD_URL is the origin the invite link points at.
	if m := mailer.New(os.Getenv("BEX_SMTP_ADDR"), os.Getenv("BEX_SMTP_FROM"),
		os.Getenv("BEX_SMTP_USERNAME"), os.Getenv("BEX_SMTP_PASSWORD")); m != nil {
		deps.Mailer = m
	}
	deps.InviteBaseURL = dashboardURL
	// The GitHub install callback (docs/ADR026-github-integration.md) redirects the
	// browser back to dashboard settings on success and with a bounded error code
	// on state/install failures.
	deps.DashboardURL = dashboardURL
	deps.DeployHookBaseURL = os.Getenv("BEX_API_PUBLIC_URL")
	deps.SSHHost = os.Getenv("BEX_SSH_HOST")
	// ADR062/ADR064 model proxy: the internal gateway origin agent model traffic is
	// routed through so the BYO key never enters the sandbox. Unset keeps session
	// mutation unavailable. Set before wireSandboxes so session egress derives its
	// port from this one value (single source of truth).
	deps.AgentModelProxyURL = os.Getenv("BEX_AGENT_MODEL_PROXY_URL")
	wireSandboxes(ctx, &deps, cl, st, sandboxExecSecret)
	wireAgentSessions(&deps)
	srv := api.NewServer(base, deps)
	// codex round-8 #9: the signed git webhook durably claims each processed
	// delivery body so a captured (body, signature) pair cannot be replayed into
	// repeated deploys. Store-less operation keeps the prior behavior.
	if st != nil {
		srv.WebhookReplays = st
	}
	// Membership rows and exact OpenFGA roles are joined by a transactional
	// Postgres outbox. Drain it independently of request retries so an invite or
	// downgrade survives transient OpenFGA failures and process restarts.
	go srv.Members.RunRoleReconciler(ctx)
	// The agent-session Completer finalizes fire-and-forget sessions: it opens the
	// draft PR + records evidence for completed turns (ADR047 D4, w3/m41). It is a
	// no-op unless the store, OpenSandbox, and GitHub App are all wired.
	go srv.AgentSessionCompleter.Run(ctx)
	if stripeLifecycleWorker != nil {
		stripeLifecycleWorker.Notifier = notifications.BillingNotifier{Service: srv.Notifications}
		go stripeLifecycleWorker.Run(ctx)
		go stripeLifecycleReconciler.Run(ctx)
	}

	wireReconcilers(ctx, srv, rec, st, cl)
	startDeliveryWorkers(ctx, srv, st, deps.Mailer, mobilePush, pushMetrics)
	srv.CORSOrigin = os.Getenv("BEX_API_CORS_ORIGIN")
	srv.HydraAdminURL = hydraAdminURL
	srv.KratosURL = os.Getenv("BEX_KRATOS_URL")
	configureServerAuthOptions(srv, cpDBURI)

	// stdio MCP mode: `api mcp-stdio` (or BEX_MCP_STDIO=1) serves only the MCP
	// adapter over stdin/stdout — how a local agent launches bex as a subprocess.
	if mcpStdio() {
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

	configureRateLimiters(srv)

	srv.MaxBodyBytes = int64(intEnv("BEX_MAX_BODY_BYTES", "2097152"))

	maxQueryHours := intEnv("BEX_MAX_QUERY_HOURS", "720")
	srv.Logs.MaxQueryHours = maxQueryHours
	srv.Logs.MaxSSEConns = int64(intEnv("BEX_MAX_SSE_CONNS", "100"))
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

// wireObservability wires the optional log and metric history sources, and
// returns the Prometheus base URL the usage-metering block reuses.
func wireObservability(deps *api.Deps) string {
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
	return promURL
}

// wireAuthz installs the OpenFGA checker on base and enforces the fail-closed
// posture a multi-tenant API requires.
//
// Authorization (docs/ADR012-auth.md): unset => authz disabled (every verb allowed,
// the pre-m4 behavior); set => every verb checks OpenFGA, fail closed. NOT
// wired in stdio mode: that transport's trust boundary is the subprocess itself
// (no auth gate, so no identity — a wired checker would deny all).
func wireAuthz(base *core.Base, cpDBURI string) core.Checker {
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
	if base.Authz == nil && cpDBURI != "" && !mcpStdio() {
		if os.Getenv("BEX_ALLOW_INSECURE_AUTHZ") != "1" {
			log.Fatal("BEX_OPENFGA_URL is unset while the control-plane store is on (BEX_CP_DB_URI set): authorization would be FAIL-OPEN — every workspace member gets admin-equivalent rights AND explicit-workspace verbs bypass membership resolution, so cross-tenant isolation does not hold. Refusing to start a multi-tenant API without enforced authorization. Set BEX_OPENFGA_URL, or set BEX_ALLOW_INSECURE_AUTHZ=1 to override in single-member/local dev only (docs/ADR012-auth.md).")
		}
		log.Printf("WARNING: BEX_OPENFGA_URL is unset while the control-plane store is on and BEX_ALLOW_INSECURE_AUTHZ=1 — authorization is FAIL-OPEN (every member admin-equivalent; explicit-workspace verbs bypass membership isolation). Safe ONLY for a single-member workspace / local dev.")
	}
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
func configureServerAuthOptions(srv *api.Server, cpDBURI string) {
	// OAuth 2.1 discovery for MCP/agent clients (w4/m9, docs/ADR012-auth.md): the Hydra
	// public issuer + this API's canonical resource URI. Both unset => no
	// metadata endpoint, no audience check — behavior identical to before.
	srv.OAuthIssuer = os.Getenv("BEX_OAUTH_ISSUER")
	srv.OAuthResource = os.Getenv("BEX_OAUTH_RESOURCE")
	// w1/m67 F1: narrow the empty-audience token exception to bex-provisioned
	// OAuth clients. Opt-in because it must not precede the operator step that
	// stamps the platform-client marker (scripts/auth-bootstrap-client.sh) — see
	// docs/ADR012-auth.md §7.
	srv.OAuthRequireAudience = os.Getenv("BEX_OAUTH_REQUIRE_AUDIENCE") == "1"
	// codex F6: an opt-in security control that ships off is invisible, and this
	// one stayed off through three remediation rounds while the fail-open posture
	// remained the deployed default. The narrowed audience check (auth.go) is
	// implemented; ENABLING it (BEX_OAUTH_REQUIRE_AUDIENCE=1) is an operator step
	// gated on scripts/auth-bootstrap-client.sh having stamped the
	// bex.co/platform-client marker first — flipping it before that 401s the
	// official Render CLI + bex-mobile device-flow logins, which legitimately
	// request no audience. So this is a LOUD WARNING on every start, not a
	// fail-closed refusal: a hard refusal would either crashloop the API when
	// the flag is off, or force BEX_ALLOW_INSECURE_AUTHZ=1 (which would also
	// disable the OpenFGA fail-closed above). Track: docs/ADR055 F6 disposition.
	if srv.OAuthResource != "" && !srv.OAuthRequireAudience {
		log.Printf("WARNING: BEX_OAUTH_REQUIRE_AUDIENCE is off while BEX_OAUTH_RESOURCE=%q — an audience-less token "+
			"from ANY self-registered OAuth client a user consents to carries that user's full workspace rights here (codex F6, cross-tenant). "+
			"Activate: re-run scripts/auth-bootstrap-client.sh (stamps bex.co/platform-client), then set it to 1 "+
			"(docs/ADR012-auth.md §7)", srv.OAuthResource)
	}
	srv.WebhookSecret = os.Getenv("BEX_WEBHOOK_SECRET")
	// The GitHub App's app-wide webhook signs pushes with its own secret — a
	// second accepted key so installed repos redeploy hands-free
	// (docs/ADR026-github-integration.md).
	srv.GitHubWebhookSecret = os.Getenv("BEX_GITHUB_WEBHOOK_SECRET")
	// codex #4: in multitenant mode (control-plane store active), reject the shared
	// manual webhook secret because it carries no per-workspace binding and would
	// authorize cross-tenant deployment mutations. The GitHub App key is unaffected.
	srv.MultitenantWebhook = cpDBURI != ""
}

// wireGitHubApp wires the GitHub App integration (docs/ADR026-github-integration.md): private-repo deploys +
// zero-config push-to-deploy. Wired only when all three BEX_GITHUB_APP_* vars
// are set (and the key parses) — else the git-connect verbs 503. The store
// half (git_connections) is wired inside the BEX_CP_DB_URI block below.
func wireGitHubApp(deps *api.Deps) *github.Client {
	var ghClient *github.Client
	if appID, key, slug := os.Getenv("BEX_GITHUB_APP_ID"), os.Getenv("BEX_GITHUB_APP_PRIVATE_KEY"), os.Getenv("BEX_GITHUB_APP_SLUG"); appID != "" && key != "" && slug != "" {
		var err error
		ghClient, err = github.NewClient(github.Config{
			AppID: appID, PrivateKey: key, Slug: slug,
			// F2: both OAuth credentials enable the mandatory installation-admin
			// proof on new browser bindings. Both absent leaves existing connections
			// usable but makes every callback fail closed; a partial pair is fatal.
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
	return ghClient
}

// wireControlPlaneFeatures wires the control-plane store into the feature deps
// and adapts the authz checker's role grant/revoke sides, returning the
// membership granter shared by the tenant service and the internal CP API.
func wireControlPlaneFeatures(deps *api.Deps, base *core.Base, st *store.PGStore, rec *store.Reconciler, authzChecker core.Checker) store.MembershipGranter {
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
	return granter
}

// wireStripeBilling wires Stripe Billing (w7/m47–m50, ADR040): one Stripe
// client drives both the seal-then-emit meter-event sidecar and the usage
// surface's invoice read-back. BEX_STRIPE_SECRET_KEY unset means no client,
// emitter, reader, or public Stripe webhook: estimate-only behavior stays
// unchanged.
func wireStripeBilling(ctx context.Context, deps *api.Deps, base *core.Base, cl client.Client, st *store.PGStore, appsNS string, usageSvc *usage.Service, billingMetrics *billing.Metrics, requirePaymentMethod bool, dashboardURL string) (*billing.Worker, *billing.Reconciler, store.BillingAdmin) {
	var stripeLifecycleWorker *billing.Worker
	var stripeLifecycleReconciler *billing.Reconciler
	var stripeBillingAdmin store.BillingAdmin
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
			DashboardURL:          dashboardURL,
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
		if n, ok := positiveIntEnv("BEX_STRIPE_SEAL_HOURS", billing.DefaultSealHours); ok {
			emitter.SealHours = time.Duration(n) * time.Hour
		}
		// w7/m57: never seal shorter than the usage rollup's catch-up window, or
		// an exported row could still be rewritten and never re-emitted.
		if clamped := billing.ClampSealHours(emitter.SealHours, usage.CatchupWindow); clamped != emitter.SealHours {
			log.Printf("BEX_STRIPE_SEAL_HOURS=%s is below the usage catch-up window; raising the seal horizon to %s so exported rows are final", emitter.SealHours, clamped)
			emitter.SealHours = clamped
		}
		var lifecycle *billing.Lifecycle
		if dunningGate(os.Getenv, stripeClient.ExpectedLivemode()) {
			// The mode threads through so webhook events from the other mode
			// are rejected; BEX_STRIPE_ALLOW_LIVE at the installer remains the
			// single deliberate live gate.
			grace := minuteDurationEnv("BEX_STRIPE_GRACE_PERIOD", "168h")
			reconcileEvery := minuteDurationEnv("BEX_STRIPE_RECONCILE_INTERVAL", "5m")
			lifecycle = &billing.Lifecycle{Store: st, GracePeriod: grace, ExpectedLivemode: stripeClient.ExpectedLivemode()}
			enforcer := &billing.KubernetesEnforcer{Client: cl, Store: st, Namespace: appsNS}
			stripeLifecycleWorker = &billing.Worker{Store: st, Enforcer: enforcer}
			stripeLifecycleReconciler = &billing.Reconciler{Store: st, Provider: stripeClient, GracePeriod: grace, Interval: reconcileEvery, Metrics: billingMetrics}
			log.Printf("bex-api Stripe dunning enabled (livemode %t, grace %s, reconcile %s)", stripeClient.ExpectedLivemode(), grace, reconcileEvery)
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
	return stripeLifecycleWorker, stripeLifecycleReconciler, stripeBillingAdmin
}

// startControlPlaneServer starts the internal control-plane API (:8091): it
// grants workspace-admin and cross-tenant writes, so it must never serve
// unauthenticated. Fail closed at startup when BEX_CP_TOKEN is empty (w1/m53:
// the token was set nowhere in prod, so the API had been serving open behind
// the NetworkPolicy alone). BEX_CP_INSECURE=1 is a loud local-dev override.
func startControlPlaneServer(ctx context.Context, st *store.PGStore, rec *store.Reconciler, granter store.MembershipGranter, stripeBillingAdmin store.BillingAdmin, metricRegistry *prometheus.Registry, ghClient *github.Client, modelKeys core.SecretKV, sandboxExecSecret, appsNS string, ready *serve.Readiness) {
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
	if sandboxExecSecret != "" {
		internalRoot.Handle(agentsession.InternalMintPath, &agentsession.Handler{
			Secret: []byte(sandboxExecSecret),
			Minter: &agentsession.Minter{GitHub: ghClient, Connections: st, Sessions: st, Audit: st},
		})
		// ADR062: the model-credential mint. Same gateway-only HMAC + internal-only
		// listener as the Git mint, path-domain-separated. Wired only when OpenBao is
		// reachable (modelKeys non-nil), so a deployment without the BYO key store
		// simply 503s the model proxy and never falls back to sandbox key injection.
		if modelKeys != nil {
			internalRoot.Handle(agentsession.InternalModelMintPath, &agentsession.ModelHandler{
				Secret: []byte(sandboxExecSecret),
				Minter: &agentsession.ModelMinter{Keys: modelKeys, Sessions: st, Audit: st},
			})
		}
	}
	internalRoot.Handle("/", internal.Handler())
	cpAddr := envOr("BEX_CP_ADDR", ":8091")
	cpSrv := newHTTPServer(cpAddr, internalRoot)
	log.Printf("bex-api control plane (source of truth) on %s (Apps project into per-tenant <ws> namespaces; Databases/KeyValues stay in %q)", cpAddr, appsNS)
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

// wireSandboxes wires hosted agent sandboxes (pillar 5, ADR042/w3/m32): the
// OpenSandbox lifecycle client only when BEX_OPENSANDBOX_URL is set — unset =>
// the sandbox verbs 503 and the feature is not registered (byte-identical). The
// per-workspace tenant-key provider (m32 t006) and template registry are
// wired as they land; for now a single default "base" template.
func wireSandboxes(ctx context.Context, deps *api.Deps, cl client.Client, st *store.PGStore, sandboxExecSecret string) {
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
			// ADR062: when the model proxy is on, narrow the session policy to admit
			// the gateway proxy port instead of the vendor host. The port is derived
			// from the same deps.AgentModelProxyURL the agentsessions Service uses, so
			// the two can never disagree on which port to open.
			ModelProxyPort: modelProxyPort(deps.AgentModelProxyURL),
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
		if sandboxExecSecret != "" {
			if gwURL := os.Getenv("BEX_SANDBOX_EXEC_URL"); gwURL != "" {
				deps.SandboxExec = &sandbox.ExecConfig{
					Secret:     []byte(sandboxExecSecret),
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
func wireAgentSessions(deps *api.Deps) {
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
	// Optional override of the in-cluster Git smart-HTTP proxy origin. The
	// sandbox receives this non-secret URL, never a GitHub credential.
	deps.AgentGitProxyURL = os.Getenv("BEX_AGENT_GIT_PROXY_URL")
	// Active-tier sandbox lifecycle (ADR059 D2/D6, w2/m67): idle grace before a
	// finished session's sandbox is reaped (default 30m; 0 ⇒ ADR054 D6 immediate
	// reap), and the per-workspace concurrent live-sandbox cap (default 5; 0 ⇒
	// uncapped).
	deps.AgentSandboxIdleTTL = zeroableDurationEnv("BEX_AGENT_SANDBOX_IDLE_TTL", 30*time.Minute)
	deps.AgentMaxLiveSandboxesPerWorkspace = zeroableIntEnv("BEX_AGENT_MAX_LIVE_SANDBOXES_PER_WORKSPACE", 5)
	// ADR059 D3/D5 hibernation (w2/m68): the object store enables the Hibernated
	// tier (reclaim → snapshot, resume → rehydrate). Unset ⇒ the whole tier is off
	// and reclaim stays Terminate (byte-identical to w2/m67).
	if store := agentsessions.NewS3SnapshotStore(agentsessions.S3SnapshotConfig{
		Endpoint:  os.Getenv("BEX_AGENT_SNAPSHOT_S3_ENDPOINT"),
		Bucket:    os.Getenv("BEX_AGENT_SNAPSHOT_S3_BUCKET"),
		Region:    os.Getenv("BEX_AGENT_SNAPSHOT_S3_REGION"),
		Prefix:    os.Getenv("BEX_AGENT_SNAPSHOT_S3_PREFIX"),
		AccessKey: os.Getenv("BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("BEX_AGENT_SNAPSHOT_S3_SECRET_KEY"),
	}); store != nil {
		deps.AgentSnapshotStore = store
		log.Printf("bex-api: agent-session hibernation enabled (object store %s)", os.Getenv("BEX_AGENT_SNAPSHOT_S3_BUCKET"))
	}
	deps.AgentSnapshotRetentionTTL = zeroableDurationEnv("BEX_AGENT_SNAPSHOT_RETENTION", 7*24*time.Hour)
	deps.AgentMaxPinnedSandboxesPerWorkspace = zeroableIntEnv("BEX_AGENT_MAX_PINNED_SANDBOXES_PER_WORKSPACE", 10)
}

func wireReconcilers(ctx context.Context, srv *api.Server, rec *store.Reconciler, st *store.PGStore, cl client.Client) {
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
}

// startDeliveryWorkers starts the outbound-webhook delivery worker and the
// native-push consumer; each runs only when its wiring is present.
func startDeliveryWorkers(ctx context.Context, srv *api.Server, st *store.PGStore, m members.Mailer, mobilePush pushtransport.Transport, pushMetrics *notifications.PushMetrics) {
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
		// Terminal deliveries are purged on an age + per-endpoint-count policy
		// (w1/m67 F3): webhook_deliveries doubles as the dashboard's history view,
		// so without retention a tenant's ordinary activity grew shared storage
		// forever. 0 keeps the documented defaults.
		whRetentionDays := intEnv("BEX_WEBHOOK_RETENTION_DAYS", "0")
		whKeepPerEndpoint := intEnv("BEX_WEBHOOK_RETENTION_KEEP", "0")
		whWorker := &webhooks.Worker{
			Store: st, Mailer: m, Emails: srv.Notifications.Identities, Backoff: backoff,
			RetentionDays: whRetentionDays, RetentionKeepPerEndpoint: whKeepPerEndpoint,
		}
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
}

// configureRateLimiters wires the trusted-proxy-aware rate limiters and the
// pre-auth admission budget onto the server.
func configureRateLimiters(srv *api.Server) {
	// Trusted-proxy CIDRs for rate-limit identity (w4/m33 P2 register,
	// .pm/w4/029.md report #10). In production every public request's TCP peer
	// is a Traefik pod, so without this every IP-keyed limiter below keys all
	// anonymous Internet clients into ONE shared bucket (the device flow's
	// 30/min held platform-wide breaks `render login` for everyone). Set ⇒ a
	// limiter derives the client IP from X-Forwarded-For/X-Real-IP only when
	// the immediate peer is inside one of these CIDRs; unset ⇒ peer IP only,
	// headers ignored (byte-identical to before). Malformed ⇒ fail closed.
	trustedProxies, err := core.ParseTrustedProxies(os.Getenv("BEX_TRUSTED_PROXY_CIDRS"))
	if err != nil {
		log.Fatalf("bex-api: bad BEX_TRUSTED_PROXY_CIDRS: %v", err)
	}

	// Rate limiting + request caps (w7/m3). BEX_RATE_LIMIT=0 disables the limiter.
	rpm, burst := rateLimitEnv("BEX_RATE_LIMIT", "500", "BEX_RATE_BURST", "0")
	srv.RateLimiter = api.NewRateLimiter(rpm, burst) // nil when rpm=0 (disabled)

	// Device-flow rate limiting (w4/m31/t002). BEX_DEVICE_RATE_LIMIT=0
	// disables it. Set here, before Handler(), like every other cap: Handler()
	// wires this onto the cliauth.Service it constructs internally (it can't
	// be built any earlier, since it needs the auth gate's invalidate
	// callback, which is also Handler()-scoped) — but the limiter itself only
	// needs the env-derived rpm/burst, so there's no reason its configuration
	// should wait until after Handler() the way its consumer's construction
	// must.
	deviceRPM, deviceBurst := rateLimitEnv("BEX_DEVICE_RATE_LIMIT", "30", "BEX_DEVICE_RATE_BURST", "0")
	srv.DeviceRateLimiter = cliauth.NewDeviceRateLimiter(deviceRPM, deviceBurst)

	// Webhook intake rate limiting (w7/m60). The two unauthenticated intakes
	// (POST /v1/webhooks/git, /v1/webhooks/stripe) mount outside the auth gate, so
	// neither BEX_RATE_LIMIT nor BEX_DEVICE_RATE_LIMIT reaches them; this IP-keyed
	// limiter sheds a flood with 429 before the body read + HMAC verification.
	// Default 600/min per IP is deliberately generous: no legitimate GitHub/Stripe
	// delivery pattern hits 10 req/s from one source IP, and both senders retry, so
	// an unlucky burst-shed self-heals — only an abusive flood is turned away.
	// BEX_WEBHOOK_RATE_LIMIT=0 disables it (byte-identical to before m60).
	webhookRPM, webhookBurst := rateLimitEnv("BEX_WEBHOOK_RATE_LIMIT", "600", "BEX_WEBHOOK_RATE_BURST", "0")
	srv.WebhookRateLimiter = api.NewRateLimiter(webhookRPM, webhookBurst) // nil when rpm=0

	// Deploy-hook lookup limiting is a distinct IP budget outside the auth gate.
	// The hook's inner 6/min token bucket handles a leaked valid credential; this
	// outer cap sheds random-token enumeration before any Kubernetes API lookup.
	deployHookLookupRPM, deployHookLookupBurst := rateLimitEnv("BEX_DEPLOY_HOOK_LOOKUP_RATE_LIMIT", "60", "BEX_DEPLOY_HOOK_LOOKUP_RATE_BURST", "10")
	srv.DeployHookLookupRateLimiter = api.NewRateLimiter(deployHookLookupRPM, deployHookLookupBurst)

	// Pre-auth admission (w1/m67 F1). The per-caller limiter above runs INSIDE
	// the auth gate, keyed on the resolved identity, so it cannot bound the work
	// of resolving an identity: with no negative cache, every unique invalid
	// bearer/session costs one Hydra or Kratos round trip. Invalid credentials
	// spend a source-IP budget; every credential also has its own HMAC-keyed
	// request/concurrency partition, so one stolen valid session is shed before
	// upstream I/O without throttling unrelated SSR users behind the same IP.
	// BEX_AUTH_FAILURE_LIMIT=0 + BEX_AUTH_MAX_INFLIGHT=0 disables both.
	authFailureRPM, authFailureBurst := rateLimitEnv("BEX_AUTH_FAILURE_LIMIT", "60", "BEX_AUTH_FAILURE_BURST", "0")
	authMaxInflight, err := strconv.Atoi(envOr("BEX_AUTH_MAX_INFLIGHT", "64"))
	if err != nil {
		log.Fatalf("bex-api: bad BEX_AUTH_MAX_INFLIGHT: %v", err)
	}
	srv.AuthAdmission = api.NewAuthAdmission(authFailureRPM, authFailureBurst, authMaxInflight)
	if srv.AuthAdmission != nil {
		srv.AuthAdmission.TrustedProxies = trustedProxies
	}

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

// mcpStdio reports whether the binary should run as a stdio MCP server rather
// than the HTTP service: subcommand `mcp-stdio` or BEX_MCP_STDIO=1.
func mcpStdio() bool {
	if os.Getenv("BEX_MCP_STDIO") == "1" {
		return true
	}
	return len(os.Args) > 1 && os.Args[1] == "mcp-stdio"
}
