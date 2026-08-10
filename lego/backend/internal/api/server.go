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

// Package api is the composition root of bex-api: it wires the feature services
// (apps, logs, metrics, apikeys, postgres) behind one auth gate and assembles
// the three transports as SINGLE artifacts — one REST router, one GraphQL
// schema, one MCP registry. Each feature contributes registration fragments
// (RegisterREST / GraphQLQuery+GraphQLMutation / RegisterMCP); the root merges
// them, so a verb reachable over one surface is reachable over all three and the
// surfaces cannot drift. The root imports the features + core; features never
// import the root (no cycle).
package api

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/cliauth"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/deploys"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/environments"
	"github.com/bex-co/bex/lego/backend/internal/events"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/jobs"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/notifications"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/projects"
	"github.com/bex-co/bex/lego/backend/internal/registrycreds"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/sshkeys"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/usage"
	"github.com/bex-co/bex/lego/backend/internal/webhooks"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

const (
	mcpServerName = "bex"
	mcpVersion    = "0.1.0"
)

const errNoHydraURL = "bex-api: BEX_HYDRA_ADMIN_URL must be set (refusing to serve without a token validator)"

// Server wires the feature services over one auth gate and assembles the three
// surfaces. All surfaces mount on the same mux, share the auth middleware, and
// call identical Service methods — so they cannot diverge in behavior.
type Server struct {
	Apps          *apps.Service
	Logs          *logs.Service
	Metrics       *metrics.Service
	APIKeys       *apikeys.Service
	SSHKeys       *sshkeys.Service
	Sandbox       *sandbox.Service
	AgentSessions *agentsessions.Service
	// AgentSessionCompleter is the trusted background loop that finalizes
	// fire-and-forget agent sessions (ADR047 D4, w3/m41). nil (or its deps
	// unwired) ⇒ the loop is a no-op; main.go starts it with the serve context.
	AgentSessionCompleter *agentsessions.Completer
	Postgres              *postgres.Service
	KeyValue              *keyvalue.Service
	Secrets               *secrets.Service
	EnvGroups             *envgroups.Service
	Workspaces            *workspaces.Service
	Members               *members.Service
	Billing               *billing.Service
	Usage                 *usage.Service
	Deploys               *deploys.Service
	Events                *events.Service
	Audit                 *audit.Service
	GitHub                *github.Service
	Notifications         *notifications.Service
	Projects              *projects.Service
	Environments          *environments.Service
	RegistryCreds         *registrycreds.Service
	Webhooks              *webhooks.Service
	Jobs                  *jobs.Service
	// StripeWebhook is the signature-verifying public billing callback. It is
	// nil unless BEX_STRIPE_WEBHOOK_SECRET is configured.
	StripeWebhook http.Handler
	// DeviceRateLimiter guards the official Render CLI's three
	// credential-less device-flow routes (w4/m31/t002) — nil disables
	// limiting. Set by the composition root once BEX_DEVICE_RATE_LIMIT is
	// known, BEFORE calling Handler(), the same convention RateLimiter
	// itself uses; Handler() wires it onto the cliauth.Service it
	// constructs internally (which needs the auth gate's invalidate
	// callback, so it can't itself be built any earlier than Handler()).
	DeviceRateLimiter *cliauth.DeviceRateLimiter

	CORSOrigin string // comma-separated allowed origins; empty => no CORS

	HydraAdminURL string // Hydra admin base URL (introspection); required
	KratosURL     string // Kratos public base URL (whoami); empty disables sessions

	// OAuth 2.1 resource-server discovery (w4/m9, MCP authorization spec).
	// OAuthIssuer is Hydra's public issuer (e.g. https://oauth.bex.co);
	// OAuthResource is this API's canonical resource URI (e.g.
	// https://api.bex.co/mcp) — advertised via RFC 9728 protected-resource
	// metadata. When set, the introspected token's `aud` must include the
	// resource and its `iss` must match the issuer (w6/m6). Both unset =>
	// behavior is byte-identical to before (no metadata endpoint, bare
	// WWW-Authenticate, no audience/issuer check).
	OAuthIssuer   string
	OAuthResource string
	// OAuthRequireAudience (BEX_OAUTH_REQUIRE_AUDIENCE=1) narrows the empty-audience
	// token exception to bex-provisioned OAuth clients (w1/m67 F1). Off by default:
	// it must not be enabled before auth-bootstrap-client.sh has stamped the
	// platform-client marker on the Render CLI + mobile clients, or their
	// legitimately audience-less logins would be refused. See auth.go's
	// requireAudience field and docs/ADR012-auth.md §7.
	OAuthRequireAudience bool
	// AuthAdmission, when set, bounds the upstream identity-provider work an
	// unauthenticated caller can impose (w1/m67 F1, authadmission.go): a
	// per-client-IP budget on INVALID credentials plus a global in-flight cap on
	// upstream auth calls. It is deliberately not a request-rate limiter in front
	// of the gate — that would throttle the dashboard's SSR traffic, which
	// arrives from one pod IP with every user's forwarded session. nil disables
	// both bounds.
	AuthAdmission *AuthAdmission

	// WebhookSecret is the shared HMAC-SHA256 key the git push webhook verifies
	// signatures against; empty disables the endpoint (it 503s). The webhook sits
	// OUTSIDE the OAuth gate — its signature is its authentication.
	WebhookSecret string
	// GitHubWebhookSecret is the GitHub App's webhook HMAC key
	// (BEX_GITHUB_WEBHOOK_SECRET) — a second accepted key on the same endpoint so
	// app-signed pushes redeploy hands-free. The endpoint 503s only when both
	// this and WebhookSecret are empty (docs/ADR026-github-integration.md).
	GitHubWebhookSecret string

	// MultitenantWebhook, when true, rejects the shared manual webhook secret
	// (BEX_WEBHOOK_SECRET) on the git push endpoint because it carries no
	// per-workspace binding and would authorize cross-tenant deployment mutations
	// (codex #4). The GitHub App key is unaffected — its deliveries are confined
	// via Installations. Set when the control-plane store is active.
	MultitenantWebhook bool

	// RateLimiter, when set, enforces per-caller token-bucket limits on the three
	// auth-gated surfaces (REST, GraphQL, MCP). nil disables rate limiting. The
	// healthz endpoint is intentionally exempt; the webhook intakes get their own
	// WebhookRateLimiter below (they mount outside this gate).
	RateLimiter *RateLimiter
	// WebhookRateLimiter, when set, meters the two unauthenticated webhook intakes
	// (POST /v1/webhooks/git, /v1/webhooks/stripe) with an IP-keyed token bucket so
	// a flood sheds with 429 BEFORE the body read and HMAC/signature verification
	// (w7/m60). These routes mount outside the auth gate, so the identity-keyed
	// RateLimiter never sees them; a separate instance keeps a webhook flood from
	// draining an IP's API budget and vice-versa. Requests here carry no Identity,
	// so RateLimiter.callerKey degrades to pure IP keying. nil disables (the
	// default is generous — see cmd/api). The device-flow routes have their own
	// DeviceRateLimiter because they need the OAuth slow_down 429 dialect; a
	// webhook sender only needs a plain 429 + Retry-After, so it reuses this one.
	WebhookRateLimiter *RateLimiter
	// DeployHookLookupRateLimiter is the IP-keyed outer budget for the public
	// deploy-hook prefix. It runs before token lookup, complementing the inner
	// per-token limiter: random well-formed credentials cannot force unbounded
	// Kubernetes API reads or grow arbitrary token buckets. nil disables it.
	DeployHookLookupRateLimiter *RateLimiter
	// MaxBodyBytes, when positive, caps non-GET request bodies at this many bytes,
	// returning 413 for oversized payloads. 0 disables the check.
	MaxBodyBytes int64

	// Onboard, when set (the control-plane store is wired), mints a personal
	// tenant for a human identity on first login. nil => store off: no mint.
	Onboard Onboarding

	schema graphql.Schema
}

// Deps bundles the injected backends the feature services need — the seams that
// keep the domain layer clientset/HTTP-free (nil leaves a verb reporting its
// "…Unavailable" sentinel). NewServer wires them onto the services in one place.
type Deps struct {
	PodLogs       logs.PodLogSource
	PodLogsFollow logs.PodLogStream
	// LogHistory, when set (BEX_LOKI_URL), backs QueryLogs/Logs with durable Loki
	// history that survives pod restarts. nil => those reads use live pod logs
	// (byte-identical to before). The SSE tail always stays on pod logs.
	LogHistory logs.LogHistorySource
	// LogLabelValues, when set (BEX_LOKI_URL), backs list_log_label_values /
	// GET /v1/logs/values — filter-value discovery over the store's labels.
	LogLabelValues       logs.LogLabelValuesSource
	ResourceMetrics      metrics.ResourceMetricsSource
	ResourceMetricsRange metrics.ResourceMetricsRangeSource
	RequestMetrics       metrics.RequestMetricsSource
	// RequestLogMetrics, when set (BEX_LOKI_URL), serves host/path-filtered
	// request metrics from the Traefik access log in Loki — the only store with a
	// per-request host/path axis (w5/m58). nil => a host/path-filtered read
	// reports core.ErrLogStoreUnavailable (the Logs-tab 503 pattern).
	RequestLogMetrics    metrics.RequestMetricsSource
	MonthToDateBandwidth metrics.MonthToDateBandwidthSource
	MetricsFilterValues  metrics.MetricsFilterValuesSource
	// DiskUsage/DBConnections/ReplicationLag (w3/m10) back the datastore-scoped
	// metrics (Database/KeyValue disk usage; Postgres connections + replication
	// lag). nil => those metrics report core.ErrMetricsUnavailable, same as the
	// App-scoped Prometheus-backed metrics above.
	DiskUsage      metrics.DiskUsageSource
	DBConnections  metrics.DBConnectionsSource
	ReplicationLag metrics.ReplicationLagSource
	KeyValueStats  metrics.KeyValueStatsSource
	APIKeys        apikeys.APIKeyStore
	// SSHKeysStore persists identity-scoped public keys and resolves their
	// fingerprints for the separately deployed SSH gateway. nil => management
	// verbs report ErrSSHKeysUnavailable.
	SSHKeysStore sshkeys.Store
	// SandboxClient is the OpenSandbox lifecycle client (pillar 5, ADR042/w3/m32);
	// nil (BEX_OPENSANDBOX_URL unset) => the sandbox verbs report
	// ErrSandboxesUnavailable and the feature is not registered. SandboxTemplates
	// fixes the registered image(s); SandboxKeys mints per-workspace tenant keys
	// (nil => single-tenant OpenSandbox).
	SandboxClient          *sandbox.Client
	SandboxTemplates       map[string]sandbox.Template
	SandboxKeys            sandbox.KeyProvider
	SandboxDefaultPlan     sandbox.Plan
	SandboxDefaultTemplate string
	SandboxMeter           *sandbox.Meter
	// SandboxExec wires `render ea sandbox exec` (w3/m33) — bex-api authorizes and
	// reverse-proxies the SSE from the isolated gateway; nil => the exec verb 503s.
	SandboxExec *sandbox.ExecConfig
	// SandboxSessionEgress renders the namespaced Cilium policy before an agent
	// session sandbox starts and narrows it at the setup→agent boundary.
	SandboxSessionEgress sandbox.SessionEgress
	// Agent sessions are durable control-plane resources that delegate runtime
	// mechanism to SandboxClient. All five dependencies are mandatory: the DB
	// row, first-class OpenFGA tuple writer, sandbox lifecycle, HMAC signer, and
	// browser-reachable gateway origin form one fail-closed feature.
	AgentSessionStore        agentsessions.Store
	AgentSessionTuples       agentsessions.TupleWriter
	AgentSessionTicketSecret []byte
	AgentSessionGatewayURL   string
	AgentCredentialURL       string
	// SSHHost is the public gateway hostname advertised through Render's
	// serviceDetails.sshAddress field. Empty disables SSH address advertising.
	SSHHost string
	// ShellTicketSecret and ShellWSURL wire the Browser Web Shell
	// (docs/ADR035-ssh.md § Browser Web Shell): the HMAC key shared only with the
	// isolated gateway (BEX_SHELL_TICKET_SECRET) and the browser-reachable gateway
	// WebSocket origin (BEX_SHELL_WS_URL) handed to the terminal. Either empty =>
	// the ticket verb returns 503; native `ssh` is unaffected. bex-api never gains
	// pods/exec.
	ShellTicketSecret []byte
	ShellWSURL        string
	Store             apps.IntentStore
	// Secrets is the shared OpenBao-backed store both the env-vars/secret-files
	// feature and the env-groups feature read/write through (docs/ADR013-secrets.md). One
	// instance, wired into both services below. nil => those verbs 503.
	Secrets core.SecretKV
	// DeployStore, when set (the control-plane store is wired), backs the
	// deploy-history feature (w2/m5). nil => its verbs answer
	// core.ErrDeploysUnavailable (deploy history has no CR-only equivalent).
	DeployStore deploys.DeployStore
	// DeployBuildNamespace is BEX_BUILD_NAMESPACE (w2/m10) — the namespace
	// Cancel looks for a repo-backed App's in-flight build Job in; must match
	// the operator's own BEX_BUILD_NAMESPACE for the Job identity to resolve.
	// Empty falls back to the App's own namespace (the operator's own default).
	DeployBuildNamespace string
	// DeployHookBaseURL is BEX_API_PUBLIC_URL — the externally reachable API
	// origin used to make secret deploy-hook URLs copy-ready for CI systems.
	DeployHookBaseURL string
	// EventStore, when set (the control-plane store is wired), backs the
	// per-service events feed — a view over deploys, audit_events, and typed
	// service_event_facts. nil => the feed answers core.ErrEventsUnavailable.
	EventStore events.EventStore
	EventFacts store.EventFactWriter
	// BaseDomain is BEX_BASE_DOMAIN (the PSL-safe platform wildcard domain)
	// — the apps service names custom-domain DNS targets `<app>.<BaseDomain>` from it.
	// Empty falls back to deriving the platform host from an App's status URLs.
	BaseDomain string
	// Region is BEX_REGION, the explicit platform placement name projected onto
	// Render resource metadata. Empty means region is unavailable and omitted;
	// it is never inferred from an object-store or provider setting.
	Region string
	// Workspace lifecycle (w6/m1): the control-plane store seam + the OpenFGA
	// grant/revoke sides + the projector nudge. All nil when BEX_CP_DB_URI is
	// unset — the workspace verbs then answer ErrWorkspacesUnavailable.
	WorkspaceStore   workspaces.WorkspaceStore
	WorkspaceGranter workspaces.WorkspaceGranter
	WorkspaceRevoker workspaces.WorkspaceRevoker
	WorkspaceKick    func()
	// WorkspacePreCascadePurgers run in workspaces.Service.Delete BEFORE the
	// tenant row is dropped (retry-safe teardown of secrets, env groups,
	// Databases/KeyValues, sandboxes, Stripe); WorkspacePostCascadePurgers run
	// after it (the App CRs, which the projector would otherwise resurrect). See
	// workspaces.WorkspacePurger.
	WorkspacePreCascadePurgers  []workspaces.WorkspacePurger
	WorkspacePostCascadePurgers []workspaces.WorkspacePurger
	// Workspace members & roles (w4/m12): the same control-plane store + OpenFGA
	// role grant/revoke sides, plus the invite Mailer and the dashboard origin the
	// invite email links to. Store nil (BEX_CP_DB_URI unset) => the member verbs
	// answer ErrMembersUnavailable; Mailer nil (no SMTP) => invites are recorded
	// but not emailed.
	MembersStore   members.MembersStore
	MembersGranter members.RoleGranter
	MembersRevoker members.RoleRevoker
	Mailer         members.Mailer
	InviteBaseURL  string
	// KeyBinder, when set (the control-plane store is wired), ties each minted
	// API key to the caller's tenant (w1/m9). nil => keys mint unbound (store off).
	KeyBinder apikeys.KeyBinder
	// Onboard, when set (the control-plane store is wired), mints a personal
	// tenant for a human identity on first login (w1/m9). nil => store off: no mint.
	Onboard Onboarding
	// Usage, when set (store + Prom wired), provides the month-to-date usage
	// verb (w8/m2). nil => the verb reports ErrUsageUnavailable (503).
	Usage *usage.Service
	// Billing is the Stripe-hosted customer-onboarding provider. nil preserves a
	// stable REST/GraphQL/MCP contract whose verbs fail with 503 while Stripe is
	// disabled, instead of making the schema depend on process environment.
	Billing      billing.HostedProvider
	BillingState billing.LifecycleReader
	// Identities resolves owner/member email + MFA for the owners/members read
	// API (w6/m2) — Kratos' admin API (BEX_KRATOS_ADMIN_URL). Nil omits those
	// fields (honest subset) rather than failing the request.
	Identities workspaces.IdentityReader
	// Audit, when set (store + Prom-independent — only BEX_CP_DB_URI is
	// needed), backs the audit-log read verb (w4/m10). Constructed and its
	// retention loop started in cmd/api/main.go, same as Usage. nil => the
	// verb reports core.ErrAuditUnavailable (503).
	Audit *audit.Service
	// StripeWebhook mounts outside OAuth because Stripe authenticates with its
	// webhook signature. The handler itself must fail closed on a bad signature.
	StripeWebhook http.Handler
	// GitHub App integration (docs/ADR026-github-integration.md). GitHubClient is the
	// GitHub REST client (nil when BEX_GITHUB_APP_* unset); GitHubStore is the
	// git_connections store (nil when BEX_CP_DB_URI unset). Either nil => the
	// git-connect verbs report core.ErrGitHubUnavailable. GitHubStateSecret is
	// the app private key's PEM bytes, reused to HMAC-sign the short-lived browser
	// callback state. DashboardURL is where the install callback redirects.
	GitHubClient      github.APIClient
	GitHubStore       github.ConnectionStore
	GitHubStateSecret []byte
	// GitHubInstallVerifier makes the browser callback prove installation
	// administration (F2). nil leaves existing connections available but makes
	// every new installation binding fail closed.
	GitHubInstallVerifier github.InstallationVerifier
	DashboardURL          string

	// RegistryCredsStore, when set (the control-plane store is wired), backs
	// the registry-credentials feature (w2/m14) — CRUD for a workspace's
	// stored external-registry credentials. nil => those verbs report
	// core.ErrRegistryCredentialsUnavailable. The secret store is the shared
	// `Secrets` field above (same OpenBao instance the env-vars feature uses).
	RegistryCredsStore registrycreds.CredentialStore

	// NotificationsStore, when set (the control-plane store is wired), backs
	// the deploy-notification settings verbs (w3/m9). nil => those verbs
	// report core.ErrNotificationsUnavailable (503). Delivery reuses Mailer
	// above (mailer.SMTP satisfies both members.Mailer and
	// notifications.Mailer structurally) — nil Mailer leaves NotifyDeploy a
	// silent no-op, same degrade-quietly shape as invite delivery.
	NotificationsStore notifications.NotificationsStore
	// PushAvailable is true only when an explicitly configured transport passed
	// startup validation. It lets clients distinguish permission denial from a
	// server-disabled channel without exposing provider configuration.
	PushAvailable bool
	// ProjectsStore, when set (the control-plane store is wired), backs the
	// project grouping verbs (w1/m31). nil => those verbs report
	// projects.ErrProjectsUnavailable (503).
	ProjectsStore projects.ProjectStore
	// EnvironmentsStore, when set (the control-plane store is wired), backs the
	// environment grouping verbs layered on top of Projects. nil => those
	// verbs report environments.ErrEnvironmentsUnavailable (503).
	EnvironmentsStore environments.EnvironmentStore
	// BlueprintsStore, when set (the control-plane store is wired), backs the
	// blueprint list/sync verbs (w2/m15). nil => list/sync report
	// ErrBlueprintsUnavailable (503); validate is always available (stateless).
	BlueprintsStore apps.BlueprintStore
	// WebhookStore, when set (the control-plane store is wired), backs the
	// outbound-webhook endpoint CRUD + delivery-history verbs (w3/m11). nil =>
	// those verbs report core.ErrWebhooksUnavailable (503). The delivery
	// worker itself is constructed and started in cmd/api/main.go (a
	// background loop, the usage/audit-sweep shape), not here.
	WebhookStore webhooks.EndpointStore
	// JobStore, when set (the control-plane store is wired), backs the
	// one-off jobs feature (Render's /services/{id}/jobs). nil => every job
	// verb reports jobs.ErrJobsUnavailable (503).
	JobStore jobs.JobStore
}

// hostOf extracts the bare hostname (no scheme/port) from a URL like
// BEX_DASHBOARD_URL, for the apps service's reserved-host guard. Empty in,
// unparseable, or hostless => "" (guard inert for the dashboard host).
func hostOf(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// NewServer wires the five feature services over one core.Base + deps. Callers
// set the HTTP config fields (CORSOrigin/HydraAdminURL/KratosURL) on the result.
func NewServer(base *core.Base, d Deps) *Server {
	workspaceSvc := &workspaces.Service{
		Base:               base,
		Store:              d.WorkspaceStore,
		Granter:            d.WorkspaceGranter,
		Revoker:            d.WorkspaceRevoker,
		Kick:               d.WorkspaceKick,
		PreCascadePurgers:  d.WorkspacePreCascadePurgers,
		PostCascadePurgers: d.WorkspacePostCascadePurgers,
		Identities:         d.Identities,
		// APIKeys satisfies workspaces.KeyOwnerReader structurally (KeyOwner has
		// the identical signature) — no adapter, and no cache: the lookup runs at
		// most once per GET /v1/users (cold path), only for API-key callers.
		KeyOwners: d.APIKeys,
	}
	resourceMetadata := resourcemeta.Config{Region: d.Region, DashboardBaseURL: d.DashboardURL}
	// The GitHub-connect service is also the apps deploy path's clone-token seam
	// (docs/ADR026-github-integration.md), so build it once and share it. Always
	// non-nil; its verbs 503 until BEX_GITHUB_APP_* + the store are wired.
	gh := &github.Service{
		Base:         base,
		GitHub:       d.GitHubClient,
		Store:        d.GitHubStore,
		Verifier:     d.GitHubInstallVerifier,
		StateSecret:  d.GitHubStateSecret,
		DashboardURL: d.DashboardURL,
	}
	// The registry-credentials service is also the apps deploy path's
	// pull-secret seam (w2/m14), so build it once and share it, same as gh
	// above. Always non-nil; its verbs 503 until the control-plane store +
	// OpenBao are both wired.
	rc := &registrycreds.Service{Base: base, Store: d.RegistryCredsStore, Secret: d.Secrets}
	// pg and kv are also the projects and environments features' Database/
	// KeyValue grouping seam (w1/m31 extension, internal/projects.DatabaseIndex/
	// KeyValueIndex; w6/m20 extension, internal/environments' counterparts) —
	// built once and shared, same as gh/rc above.
	var exportSigner postgres.ExportURLSigner
	if base != nil {
		exportSigner = postgres.NewS3ExportSigner(base.Client)
	}
	logSvc := &logs.Service{Base: base, PodLogs: d.PodLogs, PodLogsFollow: d.PodLogsFollow, History: d.LogHistory, LabelValues: d.LogLabelValues, BuildNamespace: d.DeployBuildNamespace}
	if d.DeployStore != nil {
		// Platform progress lines (w1/m48): adapt the deploy store into the logs
		// domain's own shape — newest-first, coarse created-before bound, page-
		// capped; the logs service applies the exact window per line.
		ds := d.DeployStore
		logSvc.DeployProgress = func(ctx context.Context, resource string, end time.Time) ([]logs.DeployProgress, error) {
			rows, err := ds.ListDeploys(ctx, resource, store.DeployFilter{CreatedBefore: end, Limit: core.MaxPageLimit})
			if err != nil {
				return nil, err
			}
			out := make([]logs.DeployProgress, 0, len(rows))
			for _, r := range rows {
				p := logs.DeployProgress{ID: r.ID, Status: r.Status, Image: r.Image, Commit: r.Commit, CreatedAt: r.CreatedAt}
				if r.StartedAt != nil {
					p.StartedAt = *r.StartedAt
				}
				if r.FinishedAt != nil {
					p.FinishedAt = *r.FinishedAt
				}
				out = append(out, p)
			}
			return out, nil
		}
	}
	var protectionStore core.EnvironmentProtectionStore
	if candidate, ok := d.Store.(core.EnvironmentProtectionStore); ok {
		protectionStore = candidate
	}
	pg := &postgres.Service{
		Base:         base,
		Protection:   protectionStore,
		ExportSigner: exportSigner,
		Owners:       workspaceSvc,
		Metadata:     resourceMetadata,
		PodLogs:      d.PodLogs,
	}
	pg.DatabaseLogs = func(ctx context.Context, name string, q postgres.DatabaseLogQuery) ([]postgres.DatabaseLogEntry, error) {
		entries, err := logSvc.QueryLogs(ctx, logs.LogQuery{
			App:       name,
			Search:    q.Search,
			Since:     q.Since,
			End:       q.End,
			Limit:     q.Limit,
			Direction: q.Direction,
			Instance:  q.Instance,
		})
		if err != nil {
			return nil, err
		}
		out := make([]postgres.DatabaseLogEntry, len(entries))
		for i := range entries {
			out[i] = postgres.DatabaseLogEntry{
				Timestamp: entries[i].Timestamp,
				Message:   entries[i].Message,
				Labels:    entries[i].Labels,
			}
		}
		return out, nil
	}
	kv := &keyvalue.Service{Base: base, Protection: protectionStore, Owners: workspaceSvc, Metadata: resourceMetadata}
	kv.PodLogs = d.PodLogs
	kv.KeyValueLogs = func(ctx context.Context, name string, q keyvalue.KeyValueLogQuery) ([]keyvalue.KeyValueLogEntry, error) {
		entries, err := logSvc.QueryLogs(ctx, logs.LogQuery{
			App:       name,
			Search:    q.Search,
			Since:     q.Since,
			End:       q.End,
			Limit:     q.Limit,
			Direction: q.Direction,
			Instance:  q.Instance,
		})
		if err != nil {
			return nil, err
		}
		out := make([]keyvalue.KeyValueLogEntry, len(entries))
		for i := range entries {
			out[i] = keyvalue.KeyValueLogEntry{
				Timestamp: entries[i].Timestamp,
				Message:   entries[i].Message,
				Labels:    entries[i].Labels,
			}
		}
		return out, nil
	}
	// secrets + env-groups are also the blueprint apply path's seams (w1/m35:
	// apps.EnvSeeder / apps.EnvGroupApplier) — built once and shared. They are
	// wired onto Apps ONLY when OpenBao (d.Secrets) is on, so a nil seam is the
	// honest "unavailable" signal the stack pre-flight rejects against (a bex.yml
	// using envVarGroups/fromGroup/sync:false/generateValue then fails before any
	// write, never silently drops the var).
	secretsSvc := &secrets.Service{Base: base, Store: d.Secrets}
	envGroupsSvc := &envgroups.Service{Base: base, Store: d.Secrets}
	var envSeeder apps.EnvSeeder
	var secretFileSeeder apps.SecretFileSeeder
	var envGroupApplier apps.EnvGroupApplier
	if d.Secrets != nil {
		envSeeder = secretsSvc
		secretFileSeeder = secrets.NewCreateSecretFileSeeder(secretsSvc)
		envGroupApplier = envGroupsSvc
	}
	billingSvc := &billing.Service{Base: base, Provider: d.Billing, State: d.BillingState}
	var environmentEnvGroups environments.EnvGroupIndex
	if d.Secrets != nil {
		environmentEnvGroups = envGroupsSvc
	}
	environmentsSvc := &environments.Service{
		Base:      base,
		Store:     d.EnvironmentsStore,
		Databases: pg,
		KeyValues: kv,
		EnvGroups: environmentEnvGroups,
	}
	environmentCreateResolver := environments.NewCreateResolver(environmentsSvc)
	pg.Environments = environmentCreateResolver
	kv.Environments = environmentCreateResolver
	var blueprintGroups apps.BlueprintGroupingStore
	if groups, ok := d.Store.(apps.BlueprintGroupingStore); ok {
		blueprintGroups = groups
	}
	// Environment membership is stored with the env group, while validation
	// belongs to environments.Service. Wire the two narrow seams once here so
	// create_env_group(environmentId) and set_environment_env_groups share the
	// same workspace authorization and lookup behavior without a package cycle.
	envGroupsSvc.EnvironmentWorkspace = func(ctx context.Context, environmentID string) (string, error) {
		e, err := environmentsSvc.Get(ctx, environmentID)
		if err != nil {
			return "", err
		}
		return e.OwnerID, nil
	}
	notificationsSvc := &notifications.Service{
		Base:             base,
		Store:            d.NotificationsStore,
		Mailer:           d.Mailer,
		Identities:       identityEmailLookup{d.Identities},
		DashboardBaseURL: d.DashboardURL,
		PushAvailable:    &d.PushAvailable,
	}
	sshHost := ""
	if d.SSHKeysStore != nil {
		// Advertising an SSH address without the identity-key store would send
		// every client to a gateway they cannot enroll a key for. The activation
		// ConfigMap is necessary, but the backing key feature must be live too.
		sshHost = d.SSHHost
	}
	sandboxSvc := sandboxService(base, d)
	// Keep the agent-session lifecycle a genuine nil interface when OpenSandbox is
	// off, so enabled() checks are honest instead of holding a typed-nil pointer.
	var agentLifecycle agentsessions.SandboxLifecycle
	if sandboxSvc != nil {
		agentLifecycle = sandbox.NewAgentSessionLifecycle(sandboxSvc)
	}
	srv := &Server{
		Apps: &apps.Service{Base: base, Store: d.Store, EventFacts: d.EventFacts, BaseDomain: d.BaseDomain, DashboardHost: hostOf(d.DashboardURL), SSHHost: sshHost, ShellTicketSecret: d.ShellTicketSecret, ShellWSURL: d.ShellWSURL, GitHub: gh.DeployTokenSource(), Commits: gh.DeployCommitSource(), RegistryCreds: rc.DeployPullSecretSource(), Blueprints: d.BlueprintsStore, GitFetcher: gh.BlueprintFileFetcher(), BlueprintGroups: blueprintGroups, EnvGroups: envGroupApplier, EnvSeeder: envSeeder, SecretFileSeeder: secretFileSeeder, Environments: environmentCreateResolver, Owners: workspaceSvc, Metadata: resourceMetadata},
		Logs: logSvc,
		Metrics: &metrics.Service{
			Base:                       base,
			ResourceMetrics:            d.ResourceMetrics,
			ResourceMetricsRange:       d.ResourceMetricsRange,
			RequestMetrics:             d.RequestMetrics,
			RequestLogMetrics:          d.RequestLogMetrics,
			MonthToDateBandwidthSource: d.MonthToDateBandwidth,
			MetricsFilterValuesSource:  d.MetricsFilterValues,
			DiskUsage:                  d.DiskUsage,
			DBConnections:              d.DBConnections,
			ReplicationLag:             d.ReplicationLag,
			KeyValueStats:              d.KeyValueStats,
		},
		APIKeys: &apikeys.Service{Base: base, APIKeys: d.APIKeys, Binding: d.KeyBinder},
		SSHKeys: &sshkeys.Service{Base: base, Store: d.SSHKeysStore},
		Sandbox: sandboxSvc,
		AgentSessions: &agentsessions.Service{
			Base: base, Store: d.AgentSessionStore, Tuples: d.AgentSessionTuples,
			Sandbox: agentLifecycle, TicketSecret: d.AgentSessionTicketSecret,
			GatewayURL: d.AgentSessionGatewayURL, CredentialURL: d.AgentCredentialURL,
			SSHHost: sshHost, ModelKeys: d.Secrets, GitHub: gh,
		},
		AgentSessionCompleter: &agentsessions.Completer{
			Store: d.AgentSessionStore, Sandbox: agentLifecycle,
			GitHub: d.GitHubClient, Connections: d.GitHubStore, APIPublicURL: d.DeployHookBaseURL,
		},
		Postgres:  pg,
		KeyValue:  kv,
		Secrets:   secretsSvc,
		EnvGroups: envGroupsSvc,
		Deploys: &deploys.Service{
			Base:              base,
			Store:             d.DeployStore,
			Commits:           gh.DeployCommitSource(),
			BuildNamespace:    d.DeployBuildNamespace,
			DeployHookBaseURL: d.DeployHookBaseURL,
			DeployHookLimiter: deploys.NewDeployHookRateLimiter(deploys.DefaultDeployHookRPM, deploys.DefaultDeployHookBurst),
		},
		Events:     &events.Service{Base: base, Store: d.EventStore},
		Workspaces: workspaceSvc,
		Members: &members.Service{
			Base:          base,
			Store:         d.MembersStore,
			Granter:       d.MembersGranter,
			Revoker:       d.MembersRevoker,
			Mailer:        d.Mailer,
			InviteBaseURL: d.InviteBaseURL,
			Identities:    identityEmailLookup{d.Identities},
		},
		Billing:       billingSvc,
		Notifications: notificationsSvc,
		Projects: &projects.Service{
			Base:         base,
			Store:        d.ProjectsStore,
			Databases:    pg,
			KeyValues:    kv,
			Environments: &environments.ProjectMemberClearer{Service: environmentsSvc},
		},
		Environments:  environmentsSvc,
		GitHub:        gh,
		RegistryCreds: rc,
		Webhooks:      &webhooks.Service{Base: base, Store: d.WebhookStore},
		Jobs:          &jobs.Service{Base: base, Store: d.JobStore, EventFacts: d.EventFacts},
		Onboard:       d.Onboard,
		Usage:         d.Usage,
		Audit:         d.Audit,
		StripeWebhook: d.StripeWebhook,
	}
	// Request-time deploy-start notifications use the same feature service as
	// the reconciler's close-time success/failure fan-out, but are wired at the
	// two seams that actually initiate deploys.
	srv.Apps.StartedNotifier = notificationsSvc
	srv.Deploys.StartedNotifier = notificationsSvc
	// Manual triggers + deploy hooks refresh the private-repo clone credential
	// through the same bridge the control-plane reconciler uses — installation
	// tokens live an hour, so every deploy-initiating seam must remint, never
	// build with the previous deploy's token.
	srv.Deploys.CloneSecrets = srv.Apps.ReconcilerCloneSecreter()
	return srv
}

// identityEmailLookup adapts workspaces.IdentityReader to
// members.IdentityLookup (the two packages can't share the interface directly
// — IdentityReader.Lookup returns workspaces.IdentityAttrs, a type members
// doesn't import). A nil Identities (BEX_KRATOS_ADMIN_URL unset) degrades to
// an honest miss rather than panicking on a nil interface call.
type identityEmailLookup struct {
	Identities workspaces.IdentityReader
}

func (a identityEmailLookup) LookupIdentity(ctx context.Context, subject string) (members.IdentityAttrs, bool) {
	if a.Identities == nil {
		return members.IdentityAttrs{}, false
	}
	attrs, ok := a.Identities.Lookup(ctx, subject)
	return members.IdentityAttrs{Email: attrs.Email, MFAEnabled: attrs.MFAEnabled}, ok
}

// LookupEmail keeps the same adapter serving notifications.EmailLookup (the
// notification-settings feature needs only the address).
func (a identityEmailLookup) LookupEmail(ctx context.Context, subject string) (string, bool) {
	attrs, ok := a.LookupIdentity(ctx, subject)
	return attrs.Email, ok
}

// Feature registration contracts. A feature implements the fragments it has; the
// root type-asserts each service against these when assembling the surfaces, so a
// feature with no mutations (logs, metrics) simply omits GraphQLMutation.
type (
	restRegistrar       interface{ RegisterREST(*http.ServeMux) }
	gqlQueryProvider    interface{ GraphQLQuery() graphql.Fields }
	gqlMutationProvider interface{ GraphQLMutation() graphql.Fields }
	mcpRegistrar        interface{ RegisterMCP(*mcp.Server) }
)

// features lists the wired (non-nil) feature services in a stable order. A typed
// nil stored in an interface is not == nil, so each is checked explicitly.
// sandboxService constructs the sandbox feature when an OpenSandbox client is
// wired (BEX_OPENSANDBOX_URL set), else nil so features() skips it and the verbs
// report ErrSandboxesUnavailable — byte-identical to before pillar 5 (ADR042).
func sandboxService(base *core.Base, d Deps) *sandbox.Service {
	if d.SandboxClient == nil {
		return nil
	}
	return &sandbox.Service{
		Base:            base,
		Client:          d.SandboxClient,
		Keys:            d.SandboxKeys,
		Templates:       d.SandboxTemplates,
		DefaultPlan:     d.SandboxDefaultPlan,
		DefaultTemplate: d.SandboxDefaultTemplate,
		Exec:            d.SandboxExec,
		Meter:           d.SandboxMeter,
		SessionEgress:   d.SandboxSessionEgress,
	}
}

func (s *Server) features() []any {
	var out []any
	if s.Apps != nil {
		out = append(out, s.Apps)
	}
	if s.Logs != nil {
		out = append(out, s.Logs)
	}
	if s.Metrics != nil {
		out = append(out, s.Metrics)
	}
	if s.APIKeys != nil {
		out = append(out, s.APIKeys)
	}
	if s.SSHKeys != nil {
		out = append(out, s.SSHKeys)
	}
	if s.Sandbox != nil {
		out = append(out, s.Sandbox)
	}
	if s.AgentSessions != nil {
		out = append(out, s.AgentSessions)
	}
	if s.Postgres != nil {
		out = append(out, s.Postgres)
	}
	if s.KeyValue != nil {
		out = append(out, s.KeyValue)
	}
	if s.Secrets != nil {
		out = append(out, s.Secrets)
	}
	if s.EnvGroups != nil {
		out = append(out, s.EnvGroups)
	}
	if s.Workspaces != nil {
		out = append(out, s.Workspaces)
	}
	if s.Members != nil {
		out = append(out, s.Members)
	}
	if s.Billing != nil {
		out = append(out, s.Billing)
	}
	if s.Usage != nil {
		out = append(out, s.Usage)
	}
	if s.Deploys != nil {
		out = append(out, s.Deploys)
	}
	if s.Events != nil {
		out = append(out, s.Events)
	}
	if s.Audit != nil {
		out = append(out, s.Audit)
	}
	if s.GitHub != nil {
		out = append(out, s.GitHub)
	}
	if s.Notifications != nil {
		out = append(out, s.Notifications)
	}
	if s.Projects != nil {
		out = append(out, s.Projects)
	}
	if s.Environments != nil {
		out = append(out, s.Environments)
	}
	if s.RegistryCreds != nil {
		out = append(out, s.RegistryCreds)
	}
	if s.Webhooks != nil {
		out = append(out, s.Webhooks)
	}
	if s.Jobs != nil {
		out = append(out, s.Jobs)
	}
	return out
}

// Handler returns the fully wired http.Handler, or an error if misconfigured
// (missing Hydra URL, or an invalid GraphQL schema). Routes:
//
//	GET  /healthz                              (open)
//	GET  /v1/services, /v1/services/{id}       (auth)   REST
//	POST /v1/services/{id}/{suspend|resume|restart}  (auth)   REST
//	POST /graphql                              (auth)   GraphQL
//	     /mcp                                  (auth)   MCP (streamable-http)
func (s *Server) Handler() (http.Handler, error) {
	mux, err := s.rootMux()
	if err != nil {
		return nil, err
	}
	return withSecurityHeaders(withBodyLimit(s.MaxBodyBytes)(withCORS(s.CORSOrigin, mux))), nil
}

// rootMux builds the composed top-level mux: the directly-mounted always-public
// routes (healthz, the CLI device-flow routes, the two webhook intakes, deploy
// hooks, RFC 9728 discovery) plus the three auth-gated wildcards (/v1/, /graphql,
// /mcp). Split out of Handler() (which only adds the body-limit/CORS/security
// wrappers) so the completeness guard (alwayspublic_guard_test.go, w7/m60) can
// enumerate the raw mux and force every directly-mounted route to be a classified
// always-public inventory entry — the CI census internal/api/CLAUDE.md documents.
func (s *Server) rootMux() (*http.ServeMux, error) {
	authGate, err := s.newAuthGate()
	if err != nil {
		return nil, err
	}
	auth := authGate.middleware
	schema, err := s.newSchema()
	if err != nil {
		return nil, err
	}
	s.schema = schema

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// The official Render CLI starts unauthenticated and speaks Render's wrapper
	// endpoints rather than Hydra's RFC 8628 paths directly. Mount only those
	// exact protocol routes outside the gate; they can mint tokens but cannot
	// reach a bex resource without completing Hydra's browser authorization.
	// Its authenticated revoke route registers into the one REST router below;
	// authGate.invalidate lets logout evict this pod's introspection cache.
	cliAuth := cliauth.New(s.OAuthIssuer, s.HydraAdminURL, s.APIKeys, authGate.invalidate)
	cliAuth.RateLimiter = s.DeviceRateLimiter
	cliAuth.RegisterPublic(mux)
	// The git push webhook authenticates by HMAC signature, not the OAuth gate,
	// so it mounts directly (ahead of the /v1/ wildcard — a more specific pattern
	// wins in net/http's mux). A git host can't present a bearer token. The
	// IP-keyed webhook limiter wraps the handler so a flood sheds with 429 before
	// the body read + HMAC verification (w7/m60).
	webhookLimit := s.webhookRateLimitMiddleware()
	if s.Apps != nil {
		wh := &apps.GitWebhook{Svc: s.Apps, Secret: s.WebhookSecret, GitHubSecret: s.GitHubWebhookSecret, Multitenant: s.MultitenantWebhook}
		// codex #7: confine a GitHub-App-signed delivery to its installation's
		// workspace. Only wire the resolver when the GitHub service is present —
		// assigning a nil *github.Service would trap a typed-nil in the interface.
		if s.GitHub != nil {
			wh.Installations = s.GitHub.InstallationResolver()
		}
		mux.Handle("POST /v1/webhooks/git", webhookLimit(wh))
	}
	// Stripe cannot present a bex bearer token; its timestamped HMAC signature
	// is the route's authentication. The injected handler verifies it before
	// decoding or acting on any event; the limiter sheds a flood ahead of that.
	if s.StripeWebhook != nil {
		mux.Handle("POST /v1/webhooks/stripe", webhookLimit(s.StripeWebhook))
	}
	// Deploy hooks authenticate with the unguessable URL token itself. Mount the
	// whole prefix outside OAuth so malformed credentials containing an extra
	// slash still reach the hook handler's uniform 404 instead of falling through
	// to the authenticated /v1 wildcard and becoming a distinguishing 401.
	if s.Deploys != nil {
		hook := s.deployHookLookupRateLimitMiddleware()(s.Deploys.DeployHookHandler())
		mux.Handle("/v1/deploy-hooks", hook)
		mux.Handle("/v1/deploy-hooks/", hook)
	}
	// All three adapters sit behind the same auth gate, with rate limiting inside
	// the auth wrapper so the limiter keys on the resolved caller Identity. The
	// gate itself recognizes github's one exact signed-state callback exception;
	// keeping the mount here ensures every other /v1 route stays covered.
	rl := s.rateLimitMiddleware()
	rest, err := newRenderRequestValidator(s.restHandler(cliAuth))
	if err != nil {
		return nil, err
	}
	mux.Handle("/v1/", auth(rl(rest)))
	mux.Handle("/graphql", auth(rl(s.graphqlHandler())))
	mux.Handle("/mcp", auth(rl(s.mcpHTTPHandler())))

	// RFC 9728 protected-resource metadata (w4/m9): open by design — it's how an
	// unauthenticated MCP client discovers the authorization server (the MCP
	// authorization spec requires it). One predicate decides both this mount and
	// the 401 WWW-Authenticate enrichment (resourceMetadataURL), so the hint and
	// the endpoint can't drift.
	if s.resourceMetadataURL() != "" {
		mux.HandleFunc("GET /.well-known/oauth-protected-resource", func(w http.ResponseWriter, _ *http.Request) {
			core.WriteJSON(w, http.StatusOK, map[string]any{
				"resource":                 s.OAuthResource,
				"authorization_servers":    []string{s.OAuthIssuer},
				"bearer_methods_supported": []string{"header"},
			})
		})
	}

	return mux, nil
}

// rateLimitMiddleware returns the rate-limiting middleware when a RateLimiter
// is configured, or a no-op pass-through when it is nil (disabled).
func (s *Server) rateLimitMiddleware() func(http.Handler) http.Handler {
	if s.RateLimiter == nil {
		return func(h http.Handler) http.Handler { return h }
	}
	return s.RateLimiter.Middleware
}

// webhookRateLimitMiddleware returns the IP-keyed limiter that wraps the two
// unauthenticated webhook intakes, or a no-op pass-through when disabled. It
// runs OUTSIDE the auth gate, so RateLimiter.Middleware keys on IP (no Identity
// is ever attached to a webhook request) — shedding a flood before the handler
// reads the body or verifies the HMAC.
func (s *Server) webhookRateLimitMiddleware() func(http.Handler) http.Handler {
	if s.WebhookRateLimiter == nil {
		return func(h http.Handler) http.Handler { return h }
	}
	return s.WebhookRateLimiter.Middleware
}

// deployHookLookupRateLimitMiddleware returns the IP-keyed outer limiter for
// deploy hooks. It wraps the complete handler so shedding happens before token
// validation, digest-index lookup, and the inner per-token bucket.
func (s *Server) deployHookLookupRateLimitMiddleware() func(http.Handler) http.Handler {
	if s.DeployHookLookupRateLimiter == nil {
		return func(h http.Handler) http.Handler { return h }
	}
	return s.DeployHookLookupRateLimiter.Middleware
}

func (s *Server) newAuthGate() (*oryAuth, error) {
	if s.HydraAdminURL == "" {
		return nil, core.Err(errNoHydraURL)
	}
	// Feed the gate the api-key last-used recorder when the feature is wired, so a
	// successful API-key introspection stamps the key off the request path (w4/m13).
	var touch func(string)
	if s.APIKeys != nil {
		touch = s.APIKeys.TouchAPIKey
	}
	return newOryAuth(s.HydraAdminURL, s.KratosURL, s.OAuthResource, s.OAuthIssuer, s.resourceMetadataURL(), s.OAuthRequireAudience, s.AuthAdmission, s.Onboard, touch), nil
}

// resourceMetadataURL derives the public URL of this API's RFC 9728 metadata
// endpoint from the resource URI (same scheme+host, well-known path) — what the
// 401 WWW-Authenticate header advertises. Empty when discovery isn't configured
// or the resource URI doesn't parse.
func (s *Server) resourceMetadataURL() string {
	if s.OAuthIssuer == "" || s.OAuthResource == "" {
		return ""
	}
	u, err := url.Parse(s.OAuthResource)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/.well-known/oauth-protected-resource"
}

// restHandler mounts every feature's REST fragment on one mux — the single REST
// router (Render-public-API compatible), served under /v1.
// restHandler assembles the single REST router from every feature's fragment.
// extra carries registrars constructed inside Handler() itself (cliauth's
// authenticated revoke route needs the auth gate's cache hook), so even those
// stay inside the one router rather than becoming parallel root mounts.
func (s *Server) restHandler(extra ...restRegistrar) *http.ServeMux {
	mux := http.NewServeMux()
	for _, f := range s.features() {
		if r, ok := f.(restRegistrar); ok {
			r.RegisterREST(mux)
		}
	}
	for _, r := range extra {
		r.RegisterREST(mux)
	}
	// Render supports "workflows" as a resource type; the official CLI probes
	// GET /v1/workflows when resolving --resources for multi-resource log queries.
	// bex does not implement workflows, so stub the list endpoint to return an
	// empty array rather than 404 (which the CLI treats as a fatal resolution
	// error). The {id} sub-routes are omitted — only the list is probed.
	mux.HandleFunc("GET /v1/workflows", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]")) //nolint:errcheck
	})
	return mux
}

// newSchema merges every feature's GraphQL fragments into the single root Query
// and Mutation objects — the one schema the /graphql handler serves.
func (s *Server) newSchema() (graphql.Schema, error) {
	query := graphql.Fields{}
	mutation := graphql.Fields{}
	for _, f := range s.features() {
		if p, ok := f.(gqlQueryProvider); ok {
			maps.Copy(query, p.GraphQLQuery())
		}
		if p, ok := f.(gqlMutationProvider); ok {
			maps.Copy(mutation, p.GraphQLMutation())
		}
	}
	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: query}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: mutation}),
	})
}

// graphqlHandler serves POST /graphql over the compiled schema. The request
// context already carries the caller Identity (attached by the auth middleware),
// which the feature resolvers' authorize gate reads.
func (s *Server) graphqlHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query         string         `json:"query"`
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := core.DecodeJSON(r, &body); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, "bad request")
			return
		}
		// Cost gate (w1/m65 F9): reject an over-budget document (depth / alias /
		// field / operation count) BEFORE execution, so a single near-limit request
		// can't be amplified into many resolver/provider calls or large allocations.
		// Returned as a GraphQL-shaped errors response (HTTP 200), the dialect
		// clients already handle.
		if err := validateGraphQLComplexity(body.Query); err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]any{{"message": err.Error()}}})
			return
		}
		// Env-var reads nest under the apps Service type but live in the secrets
		// feature; inject the reader so those resolvers reach it via context (the
		// shared Service GraphQL type stays stateless — no per-server closure).
		ctx := r.Context()
		if s.Secrets != nil {
			ctx = core.WithEnvVars(ctx, s.Secrets)
			ctx = core.WithSecretFiles(ctx, s.Secrets)
		}
		// Bound execution time so a single expensive document can't tie up a
		// resolver goroutine indefinitely (F9).
		ctx, cancel := context.WithTimeout(ctx, gqlExecTimeout)
		defer cancel()
		result := graphql.Do(graphql.Params{
			Schema:         s.schema,
			RequestString:  body.Query,
			OperationName:  body.OperationName,
			VariableValues: body.Variables,
			Context:        ctx,
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
}

// MCPServer builds the MCP server with every feature's tools registered. The
// returned server is stateless w.r.t. sessions, so one instance is reused for
// stdio and across HTTP sessions.
func (s *Server) MCPServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: mcpServerName, Version: mcpVersion}, nil)
	for _, f := range s.features() {
		if r, ok := f.(mcpRegistrar); ok {
			r.RegisterMCP(srv)
		}
	}
	var base *core.Base
	if s.Apps != nil {
		base = s.Apps.Base
	}
	srv.AddReceivingMiddleware(mcpWorkspaceMiddleware(base))
	return srv
}

// mcpHTTPHandler serves the MCP streamable-HTTP transport (mounted at /mcp behind
// the same auth gate as REST/GraphQL).
func (s *Server) mcpHTTPHandler() http.Handler {
	srv := s.MCPServer()
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
}

// RunStdio serves the MCP adapter over stdio — the transport a local agent
// launches bex as a subprocess with. The trust boundary is the process itself
// (no bearer applies); the HTTP transport keeps the gate. Blocks until the
// client disconnects or ctx is cancelled.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.MCPServer().Run(ctx, &mcp.StdioTransport{})
}
