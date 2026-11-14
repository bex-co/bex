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
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/accounts"
	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/bandwidth"
	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/buildminutes"
	"github.com/bex-co/bex/lego/backend/internal/cliauth"
	"github.com/bex-co/bex/lego/backend/internal/clitelemetry"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/crashloop"
	"github.com/bex-co/bex/lego/backend/internal/datastorelogs"
	"github.com/bex-co/bex/lego/backend/internal/deploys"
	"github.com/bex-co/bex/lego/backend/internal/diskusage"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/environments"
	"github.com/bex-co/bex/lego/backend/internal/events"
	"github.com/bex-co/bex/lego/backend/internal/freehours"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
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
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/router"
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
	Accounts      *accounts.Service
	Router        *router.Service
	SSHKeys       *sshkeys.Service
	Sandbox       *sandbox.Service
	AgentSessions *agentsessions.Service
	// AgentSessionCompleter is the trusted background loop that finalizes
	// fire-and-forget agent sessions (ADR047 D4, w3/m41). nil (or its deps
	// unwired) ⇒ the loop is a no-op; main.go starts it with the serve context.
	AgentSessionCompleter *agentsessions.Completer
	// BlueprintRecovery is the background loop that settles Blueprint sync
	// runs abandoned by process loss (w8/m37 t004). nil (or its store
	// unwired) ⇒ the loop is a no-op; main.go starts it with the serve
	// context.
	BlueprintRecovery *apps.BlueprintRecoverer
	// BlueprintAutoSync is the bounded worker that executes durable
	// auto-sync intents accepted at the signed git webhook (w8/m38).
	// nil / store-off ⇒ no-op.
	BlueprintAutoSync *apps.BlueprintAutoSyncWorker
	CheckedDeploys    *apps.CheckedDeployWorker
	// DeferredRestarts runs each Restart that waited, under Render's Wait
	// overlap policy, for the deploy in flight (w5/m284). Store-off ⇒ no-op.
	DeferredRestarts *deploys.DeferredRestartWorker
	// DomainVerification is the sweep that promotes a pending custom-domain
	// claim once its traffic DNS points at the service — Render's automatic
	// verification (w5/m137). nil / store-off ⇒ no-op.
	DomainVerification *apps.DomainVerificationWorker
	// ServiceRefs keeps Blueprint fromService port/hostport references equal to
	// their targets' serving ports (w6/m148). main.go starts it with the serve
	// context.
	ServiceRefs *apps.ServiceRefPropagator
	Postgres    *postgres.Service
	// PostgresExpiry expires and deletes free Postgres databases (ADR009
	// §Free-plan expiry, w8/m126). main.go starts it with the serve context.
	PostgresExpiry *postgres.FreeExpirySweeper
	// FreeHours suspends Free web services once a workspace has used its
	// monthly Free instance hours (w8/m139). main.go starts it with the serve
	// context.
	FreeHours *freehours.Sweeper
	// BuildMinutes stops builds of a workspace out of included pipeline
	// minutes with no way to pay for more (w6/m308); nil without billing.
	BuildMinutes *buildminutes.Sweeper
	// Bandwidth spins down the services of a workspace out of included
	// outbound bandwidth with no payment method (w6/m319); nil without billing.
	Bandwidth *bandwidth.Sweeper
	// CrashLoop suspends services that keep failing to start (w2/m253).
	// main.go starts it with the serve context.
	CrashLoop *crashloop.Sweeper
	// DiskUsageAlerts watches service disks cross 80% of their size
	// (w2/m255). main.go starts it with the serve context.
	DiskUsageAlerts *diskusage.Sweeper
	KeyValue        *keyvalue.Service
	Secrets         *secrets.Service
	EnvGroups       *envgroups.Service
	Workspaces      *workspaces.Service
	Members         *members.Service
	Billing         *billing.Service
	Usage           *usage.Service
	Deploys         *deploys.Service
	Events          *events.Service
	Audit           *audit.Service
	GitHub          *github.Service
	Notifications   *notifications.Service
	Projects        *projects.Service
	Environments    *environments.Service
	RegistryCreds   *registrycreds.Service
	Webhooks        *webhooks.Service
	Jobs            *jobs.Service
	CLITelemetry    *clitelemetry.Service
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
	// CLIRefreshes collapses concurrent official-CLI refreshes through the
	// control-plane Postgres. nil retains the DB-free direct-proxy path.
	CLIRefreshes     cliauth.RefreshIdempotencyStore
	OAuthRevocations RevocationStore

	// TrustedProxies (BEX_TRUSTED_PROXY_CIDRS) are the edge proxies whose
	// forwarding headers this process may believe. Set by the composition root
	// alongside the IP-keyed limiters; withSecurityHeaders consults it so HSTS
	// is only emitted for a genuinely-TLS request and never from a spoofed
	// X-Forwarded-Proto (codex-security target #10). nil ⇒ trust nobody.
	TrustedProxies core.TrustedProxies

	CORSOrigin string // comma-separated allowed origins; empty => no CORS

	HydraAdminURL string // Hydra admin base URL (introspection); required
	KratosURL     string // Kratos public base URL (whoami); empty disables sessions

	// RequireVerifiedEmail (BEX_REQUIRE_VERIFIED_EMAIL, on unless 0) refuses
	// human callers with an unverified email (ADR075 D8 revision, w2/m168).
	// EmailVerified resolves a human OAuth subject through Kratos admin; nil
	// with the gate on fails every human bearer closed (503).
	RequireVerifiedEmail bool
	EmailVerified        func(ctx context.Context, subject string) (verified, ok bool)

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
	// token exception to bex-provisioned OAuth clients (w1/m67 F1).
	OAuthRequireAudience bool
	// OAuthPlatformClients is the operator-owned client-ID registry used for
	// platform-only compatibility and durable-credential minting. It must never
	// be derived from a Hydra client record.
	OAuthPlatformClients []string
	// OAuthAPIScope is retained for constructor/env compatibility
	// (BEX_OAUTH_API_SCOPE). The closed vocabulary is not overridable: a
	// deployment cannot invent a second semantic matrix. Discovery advertises
	// bex.read / bex.write / bex.sensitive; bex.api is a platform-client
	// compatibility alias only (w8/m27).
	OAuthAPIScope string
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

	// WebhookReplays durably claims each processed signed git-webhook body so the
	// exact bytes cannot be replayed into repeated deploys (codex round-8 #9).
	// The control-plane store implements it; nil (store-less operation) keeps the
	// prior replayable behavior — there is nothing durable to claim with.
	WebhookReplays apps.WebhookReplayGuard
	// PRFeedback turns comments, reviews and failed CI on agent-session pull
	// requests into steering turns (w9/m344, ADR047 D11); nil leaves those
	// GitHub App events ignored.
	PRFeedback *agentsessions.PRFeedback
	// PRStates reads every agent-session pull request's state from GitHub
	// and applies pull_request deliveries (w9/m386); nil leaves states unread.
	PRStates *agentsessions.PRStates

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

	// OriginMetrics, when set, records bex-api's own request telemetry — the
	// per-route HTTP histogram plus the GraphQL-operation and MCP-tool
	// dimensions (docs/ADR088 §6). nil (tests, stdio mode) observes nothing;
	// the HTTP families are recorded by the middleware cmd/api wraps each
	// listener with, not from here. See httpmetrics.go.
	OriginMetrics *OriginMetrics

	schema graphql.Schema
	// gqlBudget overrides gqlExecTimeout in tests; zero means gqlExecTimeout.
	gqlBudget time.Duration
	// restBudget overrides core.RequestBudget in tests; zero means it.
	restBudget time.Duration

	mcpOnce   sync.Once
	mcpServer *mcp.Server
	// toolsetServers holds the narrow per-toolset MCP servers (ADR094 D1),
	// built once on first use like mcpServer.
	toolsetMu      sync.Mutex
	toolsetServers map[string]*mcp.Server
}

type accountStore interface {
	accounts.Store
	members.AccountMemberStore
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
	// ResourceLimitRange, when set (BEX_PROM_URL, same Prometheus as the usage
	// history), backs percentage mode's per-timestamp denominators
	// (kube-state-metrics limit history, w5/m90). nil => percentage mode
	// divides by the pods' current spec limits.
	ResourceLimitRange metrics.ResourceLimitRangeSource
	RequestMetrics     metrics.RequestMetricsSource
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
	// DatastoreResource backs a datastore's own cpu/memory and their limits
	// (w1/m180). nil => ErrMetricsUnavailable.
	DatastoreResource metrics.DatastoreResourceSource
	APIKeys           apikeys.APIKeyStore
	// Account deletion is durable control-plane orchestration. These seams are
	// nil in DB-less mode, leaving the stable API available but unavailable.
	AccountStore  accountStore
	AccountOAuth  accounts.SubjectCleaner
	AccountKratos accounts.IdentityCleaner
	// SSHKeysStore persists identity-scoped public keys and resolves their
	// fingerprints for the separately deployed SSH gateway. nil => management
	// verbs report ErrSSHKeysUnavailable.
	SSHKeysStore  sshkeys.Store
	RouterURL     string
	RouterSecret  string
	RouterMembers router.MemberStore
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
	// SandboxActivity is the exec/file activity record behind sandbox idle
	// suspension (w2/m278); nil (store off) disables idle suspension.
	SandboxActivity sandbox.ActivityStore
	// SandboxNames holds Render sandbox names (w5/m287); nil refuses --name.
	SandboxNames sandbox.NameStore
	// SandboxRecords keeps each sandbox's record past its end (w3/m318); nil
	// answers an ended sandbox as not found.
	SandboxRecords sandbox.RecordStore
	// SandboxExecutions holds Render's sandbox execution history (w32/m298);
	// nil answers the /execs reads as unavailable.
	SandboxExecutions sandbox.ExecutionStore
	// SandboxGroups holds Render's sandbox groups and each sandbox's group
	// (w22/m311); nil (store off) serves no group route, refuses a create that
	// names a group and enforces no concurrency limit.
	SandboxGroups sandbox.GroupStore
	// SandboxImages imports Render's sandbox images into the sandbox registry
	// (w2/m358); nil answers the image routes 503.
	SandboxImages *sandbox.Images
	// SandboxMaxGroups caps a workspace's groups (BEX_SANDBOX_MAX_GROUPS_PER_WORKSPACE,
	// Render's 10; 0 = uncapped), SandboxMaxGroupConcurrency is the ceiling and
	// default of a group's concurrencyLimit (BEX_SANDBOX_MAX_GROUP_CONCURRENCY,
	// Render's 1,000; 0 = none) and SandboxMaxActive caps a workspace's active
	// sandboxes across its groups (BEX_SANDBOX_MAX_ACTIVE_PER_WORKSPACE; 0 = none).
	SandboxMaxGroups           int
	SandboxMaxGroupConcurrency int
	SandboxMaxActive           int
	// SandboxStorage reads a workspace's retained sandbox image bytes and
	// SandboxMaxStorageBytes caps them (BEX_SANDBOX_MAX_STORAGE_GB_PER_WORKSPACE,
	// ADR104 D11; 0 = none): past it a suspend answers 429 SANDBOX_STORAGE_LIMIT.
	SandboxStorage         sandbox.StorageUsage
	SandboxMaxStorageBytes int64
	// SandboxExec wires `render ea sandbox exec` (w3/m33) — bex-api authorizes and
	// reverse-proxies the SSE from the isolated gateway; nil => the exec verb 503s.
	SandboxExec *sandbox.ExecConfig
	// SandboxSessionEgress renders the namespaced Cilium policy before an agent
	// session sandbox starts and narrows it at the setup→agent boundary.
	SandboxSessionEgress sandbox.SessionEgress
	// SandboxEgress installs a Render allow-list sandbox's per-sandbox Cilium
	// policy (w32/m283); nil refuses allow-list creates.
	SandboxEgress sandbox.SandboxEgress
	// Agent sessions are durable control-plane resources that delegate runtime
	// mechanism to SandboxClient. All five dependencies are mandatory: the DB
	// row, first-class OpenFGA tuple writer, sandbox lifecycle, HMAC signer, and
	// browser-reachable gateway origin form one fail-closed feature.
	AgentSessionStore agentsessions.Store
	// AgentEnvironmentStore persists each workspace's agent environment
	// (w10/m17); nil ⇒ defaults only.
	AgentEnvironmentStore agentsessions.EnvironmentStore
	// AgentMetaRouteMaxSessions caps live muse sessions across every workspace
	// on the one Meta key (BEX_AGENT_META_ROUTE_MAX_SESSIONS); 0 ⇒ default.
	AgentMetaRouteMaxSessions int
	// AgentSpendStore reads the agent-model spend controls (ADR091 D5,
	// D10.1): the trust tier's session limit and the environment's spend
	// fields; nil ⇒ neither.
	AgentSpendStore agentsessions.SpendStore
	// AgentModelCostStore reads a session's platform-route model cost per
	// turn (ADR091 D6); nil ⇒ every session reads zero cost.
	AgentModelCostStore agentsessions.ModelCostStore
	// AgentComputeStore reads the sandbox compute attributed to a session
	// (w9/m178); nil ⇒ outcome.cost carries model cost only.
	AgentComputeStore agentsessions.ComputeStore
	// AgentModelKeyChanged is told when a workspace's own model key is set or
	// cleared (agentsessions.Service.ModelKeyChanged); nil ⇒ nobody listens.
	AgentModelKeyChanged func(workspaceID string)
	// AgentAttachments and AgentAttachmentStore are the ADR095 attachment
	// bucket and table; either nil ⇒ attachments are unavailable.
	AgentAttachments     agentsessions.AttachmentObjects
	AgentAttachmentStore agentsessions.AttachmentStore
	// AgentSkillStore is the ADR099 workspace skills table; nil ⇒ skills
	// unavailable.
	AgentSkillStore agentsessions.SkillStore
	// AgentSecretScopeStore holds workspace agent secret scopes (w9/m385,
	// ADR098 D9); nil ⇒ every agent secret is explicit.
	AgentSecretScopeStore agentsessions.SecretScopeStore
	// AgentAutomationStore is the ADR101 agent automations table; nil =>
	// automations unavailable.
	AgentAutomationStore agentsessions.AutomationStore
	// AgentFeedbackStore reads the PR feedback behind steering turns
	// (w9/m344, ADR047 D11); nil leaves turnOrigins out.
	AgentFeedbackStore agentsessions.FeedbackReader
	// AgentRegistryHosts is the session-egress package-registry catalog,
	// reported by the agent environment for display.
	AgentRegistryHosts       []string
	AgentSessionTuples       agentsessions.TupleWriter
	AgentSessionTicketSecret []byte
	AgentSessionGatewayURL   string
	AgentGitProxyURL         string
	// AgentModelProxyURL (BEX_AGENT_MODEL_PROXY_URL) is the mandatory internal
	// gateway model-proxy origin (ADR062/ADR064). Sessions receive a placeholder
	// model credential + this per-session base URL, and the session egress policy
	// admits the proxy in place of the vendor host. Empty disables session mutation.
	AgentModelProxyURL string
	// AgentEgressProxyURL (BEX_AGENT_EGRESS_PROXY_URL) is the gateway egress
	// proxy origin full-tier sandboxes use (w10/m19); empty uses the default.
	AgentEgressProxyURL string
	// AgentSandboxIdleTTL is the idle grace (w2/m67, w10/m15): a finished
	// session's sandbox is terminated once it has been idle this long. Zero ⇒
	// no grace (reap as soon as no editor is connected, ADR054 D6 behavior).
	AgentSandboxIdleTTL time.Duration
	// AgentTurnTimeout bounds one agent turn (w5/m80 t002, BEX_AGENT_TURN_TIMEOUT).
	// Injected into the sandbox as BEX_AGENT_TURN_TIMEOUT_MS so a hung turn is
	// bounded here, not by the driver's 4h fallback. Zero ⇒ the 30m default.
	AgentTurnTimeout time.Duration
	// AgentTurnIdleTimeout bounds how long one turn may make no progress
	// (BEX_AGENT_TURN_IDLE_TIMEOUT), injected as BEX_AGENT_TURN_IDLE_TIMEOUT_MS;
	// AgentTurnTimeout is its ceiling. Zero ⇒ the 30m default.
	AgentTurnIdleTimeout time.Duration
	// AgentToolCallTimeout bounds how long one tool call may pause the idle
	// bound (BEX_AGENT_TOOL_CALL_TIMEOUT), injected as
	// BEX_AGENT_TOOL_CALL_TIMEOUT_MS. Zero ⇒ the 2h default.
	AgentToolCallTimeout time.Duration
	// AgentStatusRetryWindow is the Completer's window for transient
	// status-read failures (BEX_AGENT_STATUS_RETRY_WINDOW). Zero ⇒ the 10m
	// default.
	AgentStatusRetryWindow time.Duration
	// FreeInstanceHours overrides the monthly Free instance-hours allowance
	// (BEX_FREE_INSTANCE_HOURS, w8/m139); 0 means 750.
	FreeInstanceHours float64
	// CrashLoopSuspendAfter overrides the crash-loop suspension window
	// (BEX_CRASHLOOP_SUSPEND_AFTER, w2/m253); 0 means 24h.
	CrashLoopSuspendAfter time.Duration
	// MaxBlueprintGroupings caps a workspace's durable Blueprint project and
	// environment counts (BEX_MAX_BLUEPRINT_GROUPINGS, default 1000; 0
	// disables) — the w1/049 #5 abuse bound, refused with
	// BLUEPRINT_GROUPING_LIMIT identically across REST/GraphQL/MCP.
	MaxBlueprintGroupings int
	// ImageCompatibilityWorkspaces opts newly created Docker/image apps in these
	// workspaces ("*" for all) into the render-v1 container policy (w6/m149).
	ImageCompatibilityWorkspaces map[string]bool
	// PrivateImagePortsDisabled is BEX_PRIVATE_IMAGE_PORTS=off: the rollback
	// switch for image-derived private ports (ADR089 § Port slice decisions).
	PrivateImagePortsDisabled bool
	// MaxEnvGroupsPerWorkspace caps a workspace's environment-group count
	// (BEX_MAX_ENV_GROUPS_PER_WORKSPACE, default 100; 0 disables) — round-11 #3:
	// group metadata shares one index, so the cap bounds every tenant's
	// list/name-resolution sweep; refused with ENV_GROUP_LIMIT across surfaces.
	MaxEnvGroupsPerWorkspace int

	// AgentMaxLiveSandboxesPerWorkspace caps the concurrent live agent-session
	// sandboxes one workspace may hold (ADR059 D6 / w2/m67). Zero ⇒ uncapped.
	AgentMaxLiveSandboxesPerWorkspace int
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
	// DiskSnapshots lists the objects the operator's nightly disk-snapshot Job
	// writes, and DiskSnapshotSecret signs the 24-hour keys a listing hands out
	// (docs/ADR082-persistent-disks.md D5). Either unset ⇒ the snapshot verbs
	// report unavailable; disks themselves are unaffected.
	DiskSnapshots      apps.DiskSnapshotLister
	DiskSnapshotSecret []byte
	ShellWSURL         string
	Store              apps.IntentStore
	AppPlacements      apps.PlacementReader
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
	// CronRunHistory reads cron runs past the App.status.runs window (w3/m366).
	CronRunHistory store.CronRunHistory
	// BaseDomain is BEX_BASE_DOMAIN (the PSL-safe platform wildcard domain)
	// — the apps service names custom-domain DNS targets `<app>.<BaseDomain>` from it.
	// Empty falls back to deriving the platform host from an App's status URLs.
	BaseDomain string
	// ReservedDomains is BEX_RESERVED_DOMAINS: the zones no tenant may claim a
	// custom domain at or under, replacing the default around the platform
	// hosts. Empty keeps the default.
	ReservedDomains []string
	// ACMEDelegationZone is BEX_ACME_DELEGATION_ZONE: where a wildcard custom
	// domain's `_acme-challenge` CNAME points (w4/m326). Empty refuses
	// wildcards.
	ACMEDelegationZone string
	// ProxiedHosts detects Cloudflare-proxied custom domains so the service
	// and environment allowlist responses can warn that the allowlist sees
	// Cloudflare's edge there (w1/m171). nil (tests) detects nothing.
	ProxiedHosts *core.ProxiedHostDetector
	// Region is BEX_REGION, the explicit platform placement name projected onto
	// Render resource metadata. Empty means region is unavailable and omitted;
	// it is never inferred from an object-store or provider setting.
	Region string
	// Workspace lifecycle (w6/m1): the control-plane store seam + the OpenFGA
	// grant/revoke sides + the projector nudge. All nil when BEX_CP_DB_URI is
	// unset — the workspace verbs then answer ErrWorkspacesUnavailable.
	WorkspaceStore           workspaces.WorkspaceStore
	WorkspaceCreationStore   workspaces.WorkspaceCreationStore
	WorkspaceCreationBilling workspaces.WorkspaceCreationBilling
	WorkspaceGranter         workspaces.WorkspaceGranter
	WorkspaceRevoker         workspaces.WorkspaceRevoker
	// EnterpriseEntitlement is the operator-approved allow-list for the custom
	// Enterprise workspace tier. Nil keeps that tier unavailable by default.
	EnterpriseEntitlement workspaces.EnterpriseEntitlement
	// OpsWorkspaceID pins the platform ops workspace (BEX_OPS_WORKSPACE,
	// docs/ADR088-platform-observability-ui.md §4): the workspaces service
	// refuses to delete it (its membership is the observability-UI operator
	// ACL). Empty (unset) => no pin, byte-identical behavior.
	OpsWorkspaceID string
	WorkspaceKick  func()
	// WorkspacePreCascadePurgers run in workspaces.Service.Delete BEFORE the
	// tenant row is dropped (retry-safe teardown of secrets, env groups,
	// Databases/KeyValues, sandboxes, Stripe); WorkspacePostCascadePurgers run
	// after it (the App CRs, which the projector would otherwise resurrect). See
	// workspaces.WorkspacePurger.
	WorkspacePreCascadePurgers  []workspaces.WorkspacePurger
	WorkspacePostCascadePurgers []workspaces.WorkspacePurger
	// WorkspaceBillingPurger cancels the Stripe Subscription; NewServer runs it
	// after every other pre-cascade purger (ADR091 D10.4: a retry after it has
	// run settles nothing). Nil when Stripe is disabled.
	WorkspaceBillingPurger workspaces.WorkspacePurger
	// WorkspaceSettler charges a workspace's unbilled usage before workspace
	// or account deletion (ADR091 D10.4); nil when Stripe is disabled.
	WorkspaceSettler workspaces.WorkspaceSettler
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
	Onboard          Onboarding
	OAuthRevocations RevocationStore
	// Usage is the month-to-date usage verb (w8/m2), built with the
	// control-plane store. nil => NewServer mounts it store-less, answering
	// core.ErrUsageUnavailable (503).
	Usage *usage.Service
	// Billing is the Stripe-hosted customer-onboarding provider. nil preserves a
	// stable REST/GraphQL/MCP contract whose verbs fail with 503 while Stripe is
	// disabled, instead of making the schema depend on process environment.
	Billing      billing.HostedProvider
	BillingState billing.LifecycleReader
	// BillingEligibility is the payment-eligibility read billing readiness
	// takes the card refusal at binding from (ADR091 D10.5–D10.6); nil
	// reports none.
	BillingEligibility billing.PaymentEligibilityStore
	// Identities resolves owner/member email + MFA for the owners/members read
	// API (w6/m2) — Kratos' admin API (BEX_KRATOS_ADMIN_URL). Nil omits those
	// fields (honest subset) rather than failing the request.
	Identities workspaces.IdentityReader
	// Audit backs the audit-log read verb (w4/m10). cmd/api/main.go builds it
	// and starts its retention loop when BEX_CP_DB_URI is set. nil => NewServer
	// mounts it store-less, answering core.ErrAuditUnavailable (503).
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
	// MaxGitConnectionsPerWorkspace caps how many GitHub installations one
	// workspace may connect (BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE, ADR078 §2;
	// default 10, 0 disables).
	MaxGitConnectionsPerWorkspace int
	// MaxWorkspacesPerGitInstallation is its mirror under N:N: how many
	// workspaces ONE installation may serve (BEX_MAX_WORKSPACES_PER_GIT_INSTALLATION,
	// ADR078 §2; default 10, 0 disables), which also bounds the push webhook's
	// fan-out (§4a).
	MaxWorkspacesPerGitInstallation int

	// RegistryCredsStore, when set (the control-plane store is wired), backs
	// the registry-credentials feature (w2/m14) — CRUD for a workspace's
	// stored external-registry credentials. nil => those verbs report
	// core.ErrRegistryCredentialsUnavailable. The secret store is the shared
	// `Secrets` field above (same OpenBao instance the env-vars feature uses).
	RegistryCredsStore registrycreds.CredentialStore
	// MaxRegistryCredentialsPerWorkspace caps stored external-registry
	// credentials per workspace (default 50, 0 disables). Over-cap creates are
	// refused with REGISTRY_CREDENTIAL_LIMIT across all surfaces.
	MaxRegistryCredentialsPerWorkspace int
	// CLITelemetryStore, when set (the control-plane store is wired), backs
	// the CLI-telemetry ingest feature (w5/m92) — one row per
	// `POST /v1/cli-telemetry-events` delivery. Concrete *store.PGStore (not
	// the interface) so a store-off nil stays a comparable nil instead of a
	// typed-nil trap; nil => the verb reports
	// core.ErrCLITelemetryUnavailable.
	CLITelemetryStore *store.PGStore

	// MaxCustomDomainsPerService and MaxCustomDomainsPerWorkspace cap
	// custom-domain cardinality (codex-security round 18; defaults 100/500, 0
	// disables). Over-cap claims are refused with CUSTOM_DOMAIN_LIMIT across
	// all surfaces.
	MaxCustomDomainsPerService   int
	MaxCustomDomainsPerWorkspace int

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
	// WebPushAvailable is true only when VAPID composition passed startup
	// validation. Independent of Expo so native availability stays byte-identical.
	WebPushAvailable bool
	// WebPushVAPIDPublicKey is the browser applicationServerKey. Empty when unset.
	WebPushVAPIDPublicKey string
	// ProjectsStore, when set (the control-plane store is wired), backs the
	// project grouping verbs (w1/m31). nil => those verbs report
	// projects.ErrProjectsUnavailable (503).
	ProjectsStore projects.ProjectStore
	// EnvironmentsStore, when set (the control-plane store is wired), backs the
	// environment grouping verbs layered on top of Projects. nil => those
	// verbs report environments.ErrEnvironmentsUnavailable (503).
	EnvironmentsStore environments.EnvironmentStore
	// EnvGroupsStore, when set (the control-plane store is wired), holds every
	// non-secret fact about an environment group (w8/m114). nil => the env-group
	// verbs report core.ErrSecretsUnavailable (503), as with OpenBao unset.
	EnvGroupsStore envgroups.GroupStore
	// BlueprintsStore, when set (the control-plane store is wired), backs the
	// blueprint list/sync verbs (w2/m15). nil => list/sync report
	// ErrBlueprintsUnavailable (503); validate is always available (stateless).
	BlueprintsStore apps.BlueprintStore
	// WebhookStore, when set (the control-plane store is wired), backs the
	// outbound-webhook endpoint CRUD + delivery-history verbs (w3/m11). nil =>
	// those verbs report core.ErrWebhooksUnavailable (503). The delivery
	// worker itself is constructed and started in cmd/api/main.go (a
	// background loop, the usage/audit-sweep shape), not here.
	WebhookStore   webhooks.EndpointStore
	WebhookMetrics *webhooks.Metrics
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

// datastoreLogsAdapter projects the shared log service onto the datastore log
// seam. Postgres and Key Value both alias datastorelogs.Query/Entry, so one
// concrete adapter serves both fields rather than a copy per datastore.
func datastoreLogsAdapter(logSvc *logs.Service) func(context.Context, string, datastorelogs.Query) ([]datastorelogs.Entry, error) {
	return func(ctx context.Context, name string, q datastorelogs.Query) ([]datastorelogs.Entry, error) {
		entries, err := logSvc.QueryLogs(ctx, logs.LogQuery{
			App:       name,
			Search:    []string{q.Search},
			Since:     q.Since,
			End:       q.End,
			Limit:     q.Limit,
			Direction: q.Direction,
			Instance:  q.Instance,
		})
		if err != nil {
			return nil, err
		}
		out := make([]datastorelogs.Entry, len(entries))
		for i := range entries {
			out[i] = datastorelogs.Entry{
				Timestamp: entries[i].Timestamp,
				Message:   entries[i].Message,
				Labels:    entries[i].Labels,
			}
		}
		return out, nil
	}
}

// NewServer wires the five feature services over one core.Base + deps. Callers
// set the HTTP config fields (CORSOrigin/HydraAdminURL/KratosURL) on the result.
func NewServer(base *core.Base, d Deps) *Server {
	if base != nil && d.WorkspaceStore != nil {
		base.MemberWorkspaceIDs = func(ctx context.Context, identity core.Identity) ([]string, error) {
			members, err := d.WorkspaceStore.ListTenantsForSubject(ctx, identity.Subject)
			if err != nil {
				return nil, err
			}
			ids := make([]string, len(members))
			for i, member := range members {
				ids[i] = member.ID
			}
			return ids, nil
		}
	}
	workspaceSvc := &workspaces.Service{
		Base:                  base,
		Store:                 d.WorkspaceStore,
		CreationStore:         d.WorkspaceCreationStore,
		CreationBilling:       d.WorkspaceCreationBilling,
		Granter:               d.WorkspaceGranter,
		Revoker:               d.WorkspaceRevoker,
		EnterpriseEntitlement: d.EnterpriseEntitlement,
		OpsWorkspaceID:        d.OpsWorkspaceID,
		Kick:                  d.WorkspaceKick,
		PreCascadePurgers:     preCascadePurgers(d),
		PostCascadePurgers:    d.WorkspacePostCascadePurgers,
		Settler:               d.WorkspaceSettler,
		Identities:            d.Identities,
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
		Base:                         base,
		GitHub:                       d.GitHubClient,
		Store:                        d.GitHubStore,
		Verifier:                     d.GitHubInstallVerifier,
		StateSecret:                  d.GitHubStateSecret,
		DashboardURL:                 d.DashboardURL,
		MaxConnections:               d.MaxGitConnectionsPerWorkspace,
		MaxWorkspacesPerInstallation: d.MaxWorkspacesPerGitInstallation,
	}
	// The registry-credentials service is also the apps deploy path's
	// pull-secret seam (w2/m14), so build it once and share it, same as gh
	// above. Always non-nil; its verbs 503 until the control-plane store +
	// OpenBao are both wired.
	rc := &registrycreds.Service{
		Base: base, Store: d.RegistryCredsStore, Secret: d.Secrets,
		MaxCredentials: d.MaxRegistryCredentialsPerWorkspace,
	}
	// The CLI-telemetry ingest service is always non-nil; its verb 503s until
	// the control-plane store is wired (same convention as rc above).
	var telStore clitelemetry.TelemetryStore
	if d.CLITelemetryStore != nil {
		telStore = d.CLITelemetryStore
	}
	telSvc := &clitelemetry.Service{Base: base, Store: telStore}
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
		// w4/m110 t004: the narrator must not claim a build the Events tab has
		// no record of, and a deploy row's timestamps cannot tell a build from
		// a reuse rollout. Optional capability — a store without it (tests,
		// CR-only mode) simply narrates no builds rather than fabricating them.
		builtIDs, _ := ds.(interface {
			BuiltDeployIDs(ctx context.Context, appID string, limit int) (map[string]bool, error)
		})
		logSvc.DeployProgress = func(ctx context.Context, resource string, end time.Time) ([]logs.DeployProgress, error) {
			rows, err := ds.ListDeploys(ctx, resource, store.DeployFilter{CreatedBefore: end, Limit: core.MaxPageLimit})
			if err != nil {
				return nil, err
			}
			built := map[string]bool{}
			if builtIDs != nil {
				if got, err := builtIDs.BuiltDeployIDs(ctx, resource, core.MaxPageLimit); err != nil {
					// Provenance, not content: a failed lookup must not fail the
					// log read. Narrate the rollout instead of inventing a build.
					log.Printf("logs: build evidence for %s: %v", resource, err)
				} else {
					built = got
				}
			}
			out := make([]logs.DeployProgress, 0, len(rows))
			for _, r := range rows {
				p := logs.DeployProgress{ID: r.ID, Generation: r.Generation, Status: r.Status, Image: store.PublicImageRef(r.Image), Commit: r.Commit, FailureReason: r.FailureReason, DetectedHTTPPort: r.DetectedHTTPPort, CreatedAt: r.CreatedAt, Built: built[r.ID]}
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
	// Datastores have no store row to retire on deletion, so their delete verbs
	// record it for usage metering (w4/m173).
	var retirements store.UsageRetirementRecorder
	if candidate, ok := d.Store.(store.UsageRetirementRecorder); ok {
		retirements = candidate
	}
	pg := &postgres.Service{
		Base:         base,
		Retirements:  retirements,
		Protection:   protectionStore,
		ExportSigner: exportSigner,
		Owners:       workspaceSvc,
		Metadata:     resourceMetadata,
		PodLogs:      d.PodLogs,
	}
	pg.DatabaseLogs = datastoreLogsAdapter(logSvc)
	kv := &keyvalue.Service{Base: base, Retirements: retirements, Protection: protectionStore, Owners: workspaceSvc, Metadata: resourceMetadata}
	kv.PodLogs = d.PodLogs
	kv.KeyValueLogs = datastoreLogsAdapter(logSvc)
	// secrets + env-groups are also the blueprint apply path's seams (w1/m35:
	// apps.EnvSeeder / apps.EnvGroupApplier) — built once and shared. They are
	// wired onto Apps ONLY when OpenBao (d.Secrets) is on, so a nil seam is the
	// honest "unavailable" signal the stack pre-flight rejects against (a bex.yml
	// using envVarGroups/fromGroup/sync:false/generateValue then fails before any
	// write, never silently drops the var).
	// One recorder shared by every writer that patches an App CR outside the
	// deploy verbs (w6/m51): a config write that rolls a new release opens the
	// same deploy-history row an explicit deploy does.
	rollouts := &rollout.Tracker{Store: d.DeployStore}
	secretsSvc := &secrets.Service{Base: base, Store: d.Secrets, Rollout: rollouts}
	envGroupsSvc := &envgroups.Service{Base: base, Store: d.Secrets, Groups: d.EnvGroupsStore, Rollout: rollouts, MaxEnvGroups: d.MaxEnvGroupsPerWorkspace}
	if apps, ok := d.Store.(interface {
		GetApp(context.Context, string) (store.App, error)
	}); ok {
		envGroupsSvc.ServiceEnvironment = func(ctx context.Context, serviceID string) (string, error) {
			a, err := apps.GetApp(ctx, serviceID)
			if err != nil {
				return "", store.MapError(err)
			}
			return a.EnvironmentID, nil
		}
	}
	var envSeeder apps.EnvSeeder
	var envCopier apps.EnvCopier
	var envNames apps.EnvNameSource
	var createSecrets apps.CreateSecretsSeeder
	var envGroupApplier apps.EnvGroupApplier
	var envGroupExport apps.EnvGroupExportSource
	if d.Secrets != nil {
		envSeeder = secretsSvc
		envCopier = secrets.NewEnvCopier(secretsSvc)
		envNames = secretsSvc
		createSecrets = secrets.NewCreateSecretsSeeder(secretsSvc)
		envGroupApplier = envGroupsSvc
		envGroupExport = envGroupsSvc
	}
	billingSvc := &billing.Service{Base: base, Provider: d.Billing, State: d.BillingState, Eligibility: d.BillingEligibility}
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
		// Environment deletes remove contained datastores (w6/m290).
		PostgresDeleter: pg,
		KeyValueDeleter: kv,
		// The direct-create grouping quota shares the Blueprint bound
		// (codex-security round 12, finding 5).
		MaxGroupings: d.MaxBlueprintGroupings,
		ProxiedHosts: d.ProxiedHosts,
	}
	// Placement writers outside environments check the environments their
	// write leaves through its one seam (w4/m232); projects reach it through
	// ProjectMemberClearer.
	envGroupsSvc.ExitGuards = environments.ExitGuards(environmentsSvc)
	environmentCreateResolver := environments.NewCreateResolver(environmentsSvc)
	pg.Environments = environmentCreateResolver
	kv.Environments = environmentCreateResolver
	envGroupsSvc.Environments = environmentCreateResolver
	var blueprintGroups apps.BlueprintGroupingStore
	if groups, ok := d.Store.(apps.BlueprintGroupingStore); ok {
		blueprintGroups = groups
	}
	// The same store, when it can, also supplies the transactional grouping
	// runner (w8/m20 t001) so a blueprint sync's grouping writes are atomic.
	var blueprintGroupsTx apps.BlueprintGroupingTxRunner
	if runner, ok := d.Store.(apps.BlueprintGroupingTxRunner); ok {
		blueprintGroupsTx = runner
	}
	var groupingReclaim apps.GroupingReclaimer
	if reclaimer, ok := d.Store.(apps.GroupingReclaimer); ok {
		groupingReclaim = reclaimer
	}
	// Environment membership is stored with the env group, while validation
	// belongs to environments.Service. Wire the two narrow seams once here so
	// create_env_group(environmentId) and update_environment(envGroupIds) share the
	// same workspace authorization and lookup behavior without a package cycle.
	envGroupsSvc.EnvironmentWorkspace = environmentsSvc.OwnerWorkspace
	pg.EnvironmentWorkspace = environmentsSvc.OwnerWorkspace
	kv.EnvironmentWorkspace = environmentsSvc.OwnerWorkspace
	notificationsSvc := &notifications.Service{
		Base:   base,
		Store:  d.NotificationsStore,
		Mailer: d.Mailer,
		// Unwrapped, so no identity provider (BEX_KRATOS_ADMIN_URL unset)
		// leaves a nil lookup: the notifier logs each notice it cannot
		// address instead of retrying it forever (w5/m136).
		Identities:       d.Identities,
		DashboardBaseURL: d.DashboardURL,
		PushAvailable:    &d.PushAvailable,
		WebPushAvailable: &d.WebPushAvailable,
		VAPIDPublicKey:   d.WebPushVAPIDPublicKey,
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
	var agentCopier agentsessions.AttachmentCopier
	if sandboxSvc != nil {
		lifecycle := sandbox.NewAgentSessionLifecycle(sandboxSvc)
		agentLifecycle, agentCopier = lifecycle, lifecycle
	}
	deploysSvc := &deploys.Service{
		Base:              base,
		Store:             d.DeployStore,
		Kick:              d.WorkspaceKick,
		Commits:           gh.DeployCommitSource(),
		BuildNamespace:    d.DeployBuildNamespace,
		DeployHookBaseURL: d.DeployHookBaseURL,
		DeployHookLimiter: deploys.NewDeployHookRateLimiter(deploys.DefaultDeployHookRPM, deploys.DefaultDeployHookBurst),
	}
	// An environment save's rebuild is a configuration change, not a Manual
	// Deploy, for a group and a service alike (w4/347).
	rebuildForConfig := func(ctx context.Context, serviceID string) error {
		_, err := deploysSvc.Rebuild(ctx, serviceID)
		return err
	}
	envGroupsSvc.RebuildService = rebuildForConfig
	secretsSvc.RebuildService = func(ctx context.Context, serviceID string) (json.RawMessage, error) {
		d, err := deploysSvc.Rebuild(ctx, serviceID)
		if err != nil {
			return nil, err
		}
		return deploys.RenderJSON(d)
	}
	// REST and MCP restart (in apps) run the same deploys verb as GraphQL
	// restartServer, so every surface keeps the running commit (w1/m148).
	// Without a deploy store there is no live commit, so apps keeps its CR-only
	// restart.
	var restartDeploy func(ctx context.Context, service string) error
	if d.DeployStore != nil {
		restartDeploy = func(ctx context.Context, service string) error {
			_, err := deploysSvc.Restart(ctx, service)
			return err
		}
	}
	apiKeysSvc := &apikeys.Service{Base: base, APIKeys: d.APIKeys, Binding: d.KeyBinder, CreationLimiter: apikeys.NewCreationRateLimiter()}
	// One machine-credential teardown adapter, shared by the account worker and
	// the membership-exit path (w2/m163) so the fail-closed unbind-before-delete
	// order cannot drift between them. Nil when no key registry is wired.
	var keyTeardown *apikeys.AccountTeardown
	if d.APIKeys != nil && d.KeyBinder != nil {
		keyTeardown = &apikeys.AccountTeardown{Store: d.APIKeys, Binding: d.KeyBinder}
	}
	membersSvc := &members.Service{
		Base: base, Store: d.MembersStore, Granter: d.MembersGranter,
		Revoker: d.MembersRevoker, Mailer: d.Mailer, InviteBaseURL: d.InviteBaseURL,
		Identities: identityEmailLookup{d.Identities},
	}
	if keyTeardown != nil {
		membersSvc.Keys = keyTeardown
	}
	workspaceSvc.Members = memberMutator{membersSvc}
	var workspaceResolver core.WorkspaceResolver
	if base != nil {
		workspaceResolver = base.Workspace
	}
	var accountKeys accounts.MachineCredentials
	if keyTeardown != nil {
		accountKeys = *keyTeardown
	}
	accountSvc := &accounts.Service{
		Base: base, Store: d.AccountStore,
		Workspaces: workspaces.AccountTeardown{Service: workspaceSvc},
		APIKeys:    accountKeys,
		OAuth:      d.AccountOAuth, Kratos: d.AccountKratos,
		Settler: d.WorkspaceSettler, Mailer: d.Mailer, Emails: identityEmailLookup{d.Identities},
	}
	appsSvc := &apps.Service{Base: base, ProxiedHosts: d.ProxiedHosts, ImageCompatibilityWorkspaces: d.ImageCompatibilityWorkspaces, PrivateImagePortsDisabled: d.PrivateImagePortsDisabled, Store: d.Store, Placements: d.AppPlacements, EventFacts: d.EventFacts, CronRunHistory: d.CronRunHistory, BaseDomain: d.BaseDomain, DashboardHost: hostOf(d.DashboardURL), PlatformHosts: []string{hostOf(d.DeployHookBaseURL), sshHost}, ReservedDomains: d.ReservedDomains, ACMEDelegationZone: d.ACMEDelegationZone, MaxCustomDomainsPerService: d.MaxCustomDomainsPerService, MaxCustomDomainsPerWorkspace: d.MaxCustomDomainsPerWorkspace, SSHHost: sshHost, ShellTicketSecret: d.ShellTicketSecret, ShellWSURL: d.ShellWSURL, DiskSnapshots: d.DiskSnapshots, SnapshotSecret: d.DiskSnapshotSecret, GitHub: gh.DeployTokenSource(), Commits: gh.DeployCommitSource(), Checks: gh.DeployChecksSource(), RegistryCreds: rc.DeployPullSecretSource(), Blueprints: d.BlueprintsStore, GitFetcher: gh.BlueprintFileFetcher(), BlueprintGroups: blueprintGroups, BlueprintGroupsTx: blueprintGroupsTx, MaxGroupings: d.MaxBlueprintGroupings, GroupingReclaim: groupingReclaim, EnvGroups: envGroupApplier, EnvGroupLinks: &envgroups.WorkspacePurger{Service: envGroupsSvc}, EnvSeeder: envSeeder, EnvCopier: envCopier, EnvNames: envNames, EnvGroupExport: envGroupExport, CreateSecrets: createSecrets, Environments: environmentCreateResolver, EnvironmentWorkspace: environmentsSvc.OwnerWorkspace, ExitGuards: environments.ExitGuards(environmentsSvc), Owners: workspaceSvc, Metadata: resourceMetadata, RestartDeploy: restartDeploy}
	// Environment deletes call the normal service delete (w4/m264); the apps
	// service is built after environments, so the seam is closed here.
	environmentsSvc.Services = appsSvc
	if envGroupApplier != nil {
		appsSvc.EnvGroupPlacements = envgroups.GroupPlacements(envGroupsSvc)
	}
	agentSessionsSvc := &agentsessions.Service{
		Base: base, Store: d.AgentSessionStore, Tuples: d.AgentSessionTuples,
		Sandbox: agentLifecycle, TicketSecret: d.AgentSessionTicketSecret,
		GatewayURL: d.AgentSessionGatewayURL, APIPublicURL: d.DeployHookBaseURL,
		GitProxyURL:    d.AgentGitProxyURL,
		ModelProxyURL:  d.AgentModelProxyURL,
		EgressProxyURL: d.AgentEgressProxyURL,
		SSHHost:        sshHost, ModelKeys: d.Secrets, GitHub: gh,
		MaxLiveSandboxes: d.AgentMaxLiveSandboxesPerWorkspace,
		TurnTimeout:      d.AgentTurnTimeout,
		TurnIdleTimeout:  d.AgentTurnIdleTimeout,
		ToolCallTimeout:  d.AgentToolCallTimeout,
		Environments:     d.AgentEnvironmentStore,
		Spend:            d.AgentSpendStore,
		ModelCosts:       d.AgentModelCostStore,
		Compute:          d.AgentComputeStore,
		ModelKeyChanged:  d.AgentModelKeyChanged,
		RegistryHosts:    d.AgentRegistryHosts,
		Attachments:      d.AgentAttachments,
		AttachmentRows:   d.AgentAttachmentStore,
		AttachmentCopier: agentCopier,
		Skills:           d.AgentSkillStore,
		SecretScopes:     d.AgentSecretScopeStore,
		Automations:      d.AgentAutomationStore,
		Feedback:         d.AgentFeedbackStore,

		MetaRouteMaxSessions: d.AgentMetaRouteMaxSessions,
	}
	// A membership exit ends the sessions the member started (w5/186).
	memberExit := agentsessions.MemberExit{Service: agentSessionsSvc}
	membersSvc.Sessions = memberExit
	accountSvc.Members = members.AccountOffboarder{
		Store: d.AccountStore, Revoker: d.MembersRevoker, Workspace: workspaceResolver, Sessions: memberExit,
	}
	srv := &Server{
		Apps: appsSvc,
		Logs: logSvc,
		Metrics: &metrics.Service{
			Base:                       base,
			ResourceMetrics:            d.ResourceMetrics,
			ResourceMetricsRange:       d.ResourceMetricsRange,
			ResourceLimitRange:         d.ResourceLimitRange,
			RequestMetrics:             d.RequestMetrics,
			RequestLogMetrics:          d.RequestLogMetrics,
			MonthToDateBandwidthSource: d.MonthToDateBandwidth,
			MetricsFilterValuesSource:  d.MetricsFilterValues,
			DiskUsage:                  d.DiskUsage,
			DBConnections:              d.DBConnections,
			ReplicationLag:             d.ReplicationLag,
			KeyValueStats:              d.KeyValueStats,
			DatastoreResource:          d.DatastoreResource,
		},
		APIKeys:            apiKeysSvc,
		Accounts:           accountSvc,
		Router:             &router.Service{Base: base, URL: d.RouterURL, Secret: d.RouterSecret, Members: d.RouterMembers},
		SSHKeys:            &sshkeys.Service{Base: base, Store: d.SSHKeysStore},
		Sandbox:            sandboxSvc,
		AgentSessions:      agentSessionsSvc,
		BlueprintRecovery:  &apps.BlueprintRecoverer{Store: d.BlueprintsStore},
		PostgresExpiry:     &postgres.FreeExpirySweeper{Service: pg, Notifier: notifications.DatabaseExpiryNotifier{Service: notificationsSvc}},
		FreeHours:          newFreeHours(appsSvc, d, notificationsSvc),
		BuildMinutes:       newBuildMinutes(appsSvc, d, notificationsSvc),
		Bandwidth:          newBandwidth(appsSvc, d, notificationsSvc),
		CrashLoop:          newCrashLoop(appsSvc, d),
		DiskUsageAlerts:    newDiskUsage(appsSvc, d),
		BlueprintAutoSync:  &apps.BlueprintAutoSyncWorker{Svc: appsSvc},
		CheckedDeploys:     &apps.CheckedDeployWorker{Svc: appsSvc},
		DeferredRestarts:   &deploys.DeferredRestartWorker{Svc: deploysSvc},
		DomainVerification: &apps.DomainVerificationWorker{Svc: appsSvc},
		ServiceRefs:        &apps.ServiceRefPropagator{Svc: appsSvc},
		AgentSessionCompleter: &agentsessions.Completer{
			Store: d.AgentSessionStore, Sandbox: agentLifecycle,
			IdleTTL:              d.AgentSandboxIdleTTL,
			StatusReadFailureTTL: d.AgentStatusRetryWindow,
			TurnTimeout:          d.AgentTurnTimeout,
			InventoryInterval:    time.Minute,
		},
		Postgres:      pg,
		KeyValue:      kv,
		Secrets:       secretsSvc,
		EnvGroups:     envGroupsSvc,
		Deploys:       deploysSvc,
		Events:        &events.Service{Base: base, Store: d.EventStore, Identities: identityEmailLookup{d.Identities}, UserIDs: userIDLookup(d.EventStore)},
		Workspaces:    workspaceSvc,
		Members:       membersSvc,
		Billing:       billingSvc,
		Notifications: notificationsSvc,
		Projects: &projects.Service{
			Base:            base,
			Store:           d.ProjectsStore,
			Databases:       pg,
			KeyValues:       kv,
			Environments:    &environments.ProjectMemberClearer{Service: environmentsSvc},
			Services:        appsSvc,
			PostgresDeleter: pg,
			KeyValueDeleter: kv,
			ExitGuards:      environments.ExitGuards(environmentsSvc),
			Owners:          workspaceSvc,
			// The direct-create grouping quota shares the Blueprint bound
			// (codex-security round 12, finding 5).
			MaxGroupings: d.MaxBlueprintGroupings,
		},
		Environments:     environmentsSvc,
		GitHub:           gh,
		RegistryCreds:    rc,
		Webhooks:         &webhooks.Service{Base: base, Store: d.WebhookStore, Metrics: d.WebhookMetrics},
		Jobs:             &jobs.Service{Base: base, Store: d.JobStore, EventFacts: d.EventFacts},
		CLITelemetry:     telSvc,
		Onboard:          d.Onboard,
		OAuthRevocations: d.OAuthRevocations,
		// main.go builds Usage and Audit only with the control-plane store.
		// Without it they mount store-less and answer 503 (w5/113).
		Usage:         cmp.Or(d.Usage, &usage.Service{Base: base}),
		Audit:         cmp.Or(d.Audit, &audit.Service{Base: base}),
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
	// A protected environment restricts variables and secret files to Admins (w8/193).
	secretsSvc.Protection = srv.Apps.ProtectionGuard()
	srv.Deploys.PullSecrets = srv.Apps.DeployPullSecretPreparer()
	return srv
}

// preCascadePurgers is the workspace teardown's pre-cascade order: the Stripe
// purger last, so nothing can fail after the Subscription is cancelled.
func preCascadePurgers(d Deps) []workspaces.WorkspacePurger {
	purgers := slices.Clone(d.WorkspacePreCascadePurgers)
	if d.WorkspaceBillingPurger != nil {
		purgers = append(purgers, d.WorkspaceBillingPurger)
	}
	return purgers
}

// identityEmailLookup adapts workspaces.IdentityReader to
// members.IdentityLookup and notifications.EmailLookup (the packages can't
// share the interface directly — IdentityReader.LookupMany returns
// workspaces.IdentityAttrs, a type members doesn't import). A nil Identities
// (BEX_KRATOS_ADMIN_URL unset) degrades to an honest miss rather than
// panicking on a nil interface call.
type identityEmailLookup struct {
	Identities workspaces.IdentityReader
}

func (a identityEmailLookup) LookupIdentities(ctx context.Context, subjects []string) map[string]members.IdentityAttrs {
	if a.Identities == nil {
		return nil
	}
	found := a.Identities.LookupMany(ctx, subjects)
	out := make(map[string]members.IdentityAttrs, len(found))
	for subject, attrs := range found {
		out[subject] = memberAttrs(attrs)
	}
	return out
}

func (a identityEmailLookup) LookupEmails(ctx context.Context, subjects []string) (map[string]string, error) {
	if a.Identities == nil {
		return nil, nil
	}
	return a.Identities.LookupEmails(ctx, subjects)
}

func memberAttrs(attrs workspaces.IdentityAttrs) members.IdentityAttrs {
	return members.IdentityAttrs{Email: attrs.Email, MFAEnabled: attrs.MFAEnabled}
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
		SandboxEgress:   d.SandboxEgress,
		Region:          resourcemeta.RenderRegion(d.Region),
		Activity:        d.SandboxActivity,
		Names:           d.SandboxNames,
		Records:         d.SandboxRecords,
		Executions:      d.SandboxExecutions,
		UserIDs:         userIDLookup(d.SandboxExecutions),

		Groups:              d.SandboxGroups,
		Images:              d.SandboxImages,
		MaxGroups:           d.SandboxMaxGroups,
		MaxGroupConcurrency: d.SandboxMaxGroupConcurrency,
		MaxActive:           d.SandboxMaxActive,
		Storage:             d.SandboxStorage,
		MaxStorageBytes:     d.SandboxMaxStorageBytes,
	}
}

// appendFeature adds a wired feature service to the registration list. It takes
// the concrete *T so the nil test happens BEFORE boxing: a nil *apps.Service
// stored in an `any` is a non-nil interface, so collecting the fields into an
// []any first and filtering on != nil would register every unwired feature and
// hand the surface type-assertions a nil receiver.
func appendFeature[T any](out []any, feature *T) []any {
	if feature == nil {
		return out
	}
	return append(out, feature)
}

// features lists the wired feature services in Server field order. The three
// surface builders each type-assert every entry for the fragments it implements
// (restRegistrar, gqlQueryProvider, mcpRegistrar), so a feature appears on
// exactly the surfaces it registers for.
func (s *Server) features() []any {
	var out []any
	out = appendFeature(out, s.Apps)
	out = appendFeature(out, s.Logs)
	out = appendFeature(out, s.Metrics)
	out = appendFeature(out, s.APIKeys)
	out = appendFeature(out, s.Accounts)
	out = appendFeature(out, s.SSHKeys)
	out = appendFeature(out, s.Router)
	out = appendFeature(out, s.Sandbox)
	out = appendFeature(out, s.AgentSessions)
	out = appendFeature(out, s.Postgres)
	out = appendFeature(out, s.KeyValue)
	out = appendFeature(out, s.Secrets)
	out = appendFeature(out, s.EnvGroups)
	out = appendFeature(out, s.Workspaces)
	out = appendFeature(out, s.Members)
	out = appendFeature(out, s.Billing)
	out = appendFeature(out, s.Usage)
	out = appendFeature(out, s.Deploys)
	out = appendFeature(out, s.Events)
	out = appendFeature(out, s.Audit)
	out = appendFeature(out, s.GitHub)
	out = appendFeature(out, s.Notifications)
	out = appendFeature(out, s.Projects)
	out = appendFeature(out, s.Environments)
	out = appendFeature(out, s.RegistryCreds)
	out = appendFeature(out, s.CLITelemetry)
	out = appendFeature(out, s.Webhooks)
	out = appendFeature(out, s.Jobs)
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
	muxes, err := s.composedMuxes()
	if err != nil {
		return nil, err
	}
	// withBodyLimit is applied PER ROUTE inside composedMuxes, never as a blanket outer
	// wrapper (codex F11): buffering every non-GET body at the top read up to
	// MaxBodyBytes for each request BEFORE the route's cheap admission (the
	// unauthenticated webhook/deploy-hook IP limiters, the auth gate) could shed
	// it. Each mounted body-bearing handler now runs its limiter/auth first and
	// caps the body last.
	// withGzip is outermost so it observes Accept-Encoding and compresses the
	// final body; it self-exempts streaming (text/event-stream) responses so the
	// SSE live-tail / agent-session / sandbox-exec proxies still flush per event
	// (w9/m61).
	return s.wrapMuxes(muxes), nil
}

const restMountPattern = "/v1/"

type serverMuxes struct {
	root *http.ServeMux
	rest *http.ServeMux
}

func (m serverMuxes) corsRoutes() corsRoutes {
	return corsRoutes{
		root: m.root,
		delegated: map[string]*http.ServeMux{
			restMountPattern: m.rest,
		},
	}
}

func (s *Server) wrapMuxes(m serverMuxes) http.Handler {
	// The route recorder is outermost so the telemetry label is a registered
	// pattern of the product mux even when CORS answers a preflight itself.
	return recordRoutePattern(m.root, withGzip(withSecurityHeaders(s.TrustedProxies, withCORS(s.CORSOrigin, m.corsRoutes()))))
}

// rootMux exposes the raw top-level mux to completeness guards. The assembly
// lives in composedMuxes; Handler only adds the CORS/security/gzip wrappers.
func (s *Server) rootMux() (*http.ServeMux, error) {
	muxes, err := s.composedMuxes()
	return muxes.root, err
}

// composedMuxes returns both routing layers needed by Handler: the root mux it
// serves and the method-qualified REST mux mounted behind root's /v1/ wildcard.
// CORS consults both so its answer comes from the actual registered routes.
func (s *Server) composedMuxes() (serverMuxes, error) {
	authGate, err := s.newAuthGate()
	if err != nil {
		return serverMuxes{}, err
	}
	auth := authGate.middleware
	schema, err := s.newSchema()
	if err != nil {
		return serverMuxes{}, err
	}
	s.schema = schema

	mux := http.NewServeMux()
	// Per-route body cap (codex F11): mounted AFTER each route's cheap admission
	// so an over-budget or unauthenticated caller is shed before its body is
	// buffered. Zero/disabled MaxBodyBytes makes this a pass-through.
	bodyLimit := withBodyLimit(s.MaxBodyBytes)
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
	cliAuth := cliauth.New(s.OAuthIssuer, s.APIKeys, authGate.invalidate)
	cliAuth.RateLimiter = s.DeviceRateLimiter
	cliAuth.Refreshes = s.CLIRefreshes
	cliAuth.Revocations = s.OAuthRevocations
	cliAuth.RegisterPublic(mux, bodyLimit)
	// The git push webhook authenticates by HMAC signature, not the OAuth gate,
	// so it mounts directly (ahead of the /v1/ wildcard — a more specific pattern
	// wins in net/http's mux). A git host can't present a bearer token. The
	// IP-keyed webhook limiter wraps the handler so a flood sheds with 429 before
	// the body read + HMAC verification (w7/m60).
	webhookLimit := s.webhookRateLimitMiddleware()
	if s.Apps != nil {
		wh := &apps.GitWebhook{Svc: s.Apps, Secret: s.WebhookSecret, GitHubSecret: s.GitHubWebhookSecret, Multitenant: s.MultitenantWebhook, Replays: s.WebhookReplays}
		// codex #7: confine a GitHub-App-signed delivery to its installation's
		// workspace. Only wire the resolver when the GitHub service is present —
		// assigning a nil *github.Service would trap a typed-nil in the interface.
		if s.GitHub != nil {
			wh.Installations = s.GitHub.InstallationResolver()
		}
		if s.PRStates != nil {
			wh.PRStates = s.PRStates
		}
		if s.PRFeedback != nil {
			wh.PRFeedback = s.PRFeedback
		}
		mux.Handle("POST /v1/webhooks/git", webhookLimit(bodyLimit(wh)))
	}
	// Stripe cannot present a bex bearer token; its timestamped HMAC signature
	// is the route's authentication. The injected handler verifies it before
	// decoding or acting on any event; the limiter sheds a flood ahead of that.
	if s.StripeWebhook != nil {
		mux.Handle("POST /v1/webhooks/stripe", webhookLimit(bodyLimit(s.StripeWebhook)))
	}
	// Deploy hooks authenticate with the unguessable URL token itself. Mount the
	// whole prefix outside OAuth so malformed credentials containing an extra
	// slash still reach the hook handler's uniform 404 instead of falling through
	// to the authenticated /v1 wildcard and becoming a distinguishing 401.
	if s.Deploys != nil {
		hook := s.deployHookLookupRateLimitMiddleware()(bodyLimit(s.Deploys.DeployHookHandler()))
		mux.Handle("/v1/deploy-hooks", hook)
		mux.Handle("/v1/deploy-hooks/", hook)
	}
	// The pinned Render CLI redeems a sandbox run connect token (minted by the
	// gated `POST /v1/sandboxes/{id}/runs/{operation}/token`) at the `uri` bex
	// returned, carrying that HMAC token — not an OAuth credential — as its
	// Bearer, so the redeem route mounts outside the gate (w7/m147). The same
	// IP-keyed pre-lookup limiter as deploy hooks sheds a flood before the
	// token is verified; the token itself binds workspace, sandbox, execution,
	// and command, is single-use, and expires within 60s.
	if s.Sandbox != nil {
		mux.Handle(sandbox.ConnectStreamPattern, s.deployHookLookupRateLimitMiddleware()(bodyLimit(s.Sandbox.ConnectStreamHandler())))
		// File bodies have their own streaming bounds; the JSON request limit
		// would truncate otherwise valid uploads before they reach the gateway.
		files := s.deployHookLookupRateLimitMiddleware()(s.Sandbox.ConnectFileHandler())
		mux.Handle(sandbox.ConnectFileUploadPattern, files)
		mux.Handle(sandbox.ConnectFileDownloadPattern, files)
	}
	// All three adapters sit behind the same auth gate, with rate limiting inside
	// the auth wrapper so the limiter keys on the resolved caller Identity. The
	// gate itself recognizes github's one exact signed-state callback exception;
	// keeping the mount here ensures every other /v1 route stays covered.
	rl := s.rateLimitMiddleware()
	restMux := s.restHandler(cliAuth)
	rest, err := newRenderRequestValidator(restMux)
	if err != nil {
		return serverMuxes{}, err
	}
	// bodyLimit is innermost: auth (bearer/cookie — never the body) and the
	// identity-keyed rate limiter both admit the request before its body is
	// buffered, so an unauthenticated or throttled caller is rejected without the
	// server reading up to MaxBodyBytes (codex F11). The operation-class gate
	// sits outside the OpenAPI validator so a read-only token is refused
	// before schema work runs.
	// recordRoutePattern sits innermost, right against the REST mux: the
	// wrappers outside it clone the request, so reading r.Pattern after the
	// fact would only ever see the outer `/v1/` mount, not the specific route
	// the telemetry needs (w3/m84).
	mux.Handle(restMountPattern, auth(rl(withValidQueryText(restBodyLimit(bodyLimit)(s.withScopeClassREST(restMux, s.withCLIWorkspace(recordRoutePattern(restMux, s.withRequestBudget(rest)))))))))
	// GraphQL is body-bearing JSON and supports POST only. A method-qualified
	// pattern makes ServeMux return 405 before auth/body decoding for GET (whose
	// generic body limiter intentionally skips bodies).
	mux.Handle("POST /graphql", auth(rl(bodyLimit(s.graphqlHandler()))))
	mux.Handle("/mcp", auth(rl(bodyLimit(s.mcpHTTPHandler()))))
	// The cloud-agents toolset (ADR094 D1): the same server, auth gate, rate
	// limiter, workspace binding and scope gate, serving only the nine
	// cloud-agent tools plus list_workspaces and list_repos.
	mux.Handle(cloudAgentsMountPath, auth(rl(bodyLimit(s.mcpToolsetHTTPHandler(ToolsetCloudAgents)))))

	// RFC 9728 protected-resource metadata (w4/m9): open by design — it's how an
	// unauthenticated MCP client discovers the authorization server (the MCP
	// authorization spec requires it). One predicate decides both this mount and
	// the 401 WWW-Authenticate enrichment (resourceMetadataURL), so the hint and
	// the endpoint can't drift. scopes_supported advertises the closed
	// least-privilege vocabulary (w8/m27): discovery-driven clients request
	// exactly these; bex.api is a platform-client compatibility alias and is
	// not advertised.
	if s.resourceMetadataURL() != "" {
		mux.HandleFunc("GET /.well-known/oauth-protected-resource", func(w http.ResponseWriter, _ *http.Request) {
			core.WriteJSON(w, http.StatusOK, map[string]any{
				"resource":                 s.OAuthResource,
				"authorization_servers":    []string{s.OAuthIssuer},
				"bearer_methods_supported": []string{"header"},
				"scopes_supported":         core.AdvertisedScopes(),
			})
		})
		// RFC 9728 path-inserted metadata for the cloud-agents mount. Tokens are
		// audience-bound to the parent resource (BEX_OAUTH_RESOURCE), which the
		// MCP authorization spec lets a client accept for a deeper path, so the
		// document names that same resource rather than minting a second audience.
		mux.HandleFunc("GET /.well-known/oauth-protected-resource"+cloudAgentsMountPath, func(w http.ResponseWriter, _ *http.Request) {
			core.WriteJSON(w, http.StatusOK, map[string]any{
				"resource":                 s.OAuthResource,
				"authorization_servers":    []string{s.OAuthIssuer},
				"bearer_methods_supported": []string{"header"},
				"scopes_supported":         core.AdvertisedScopes(),
			})
		})
	}

	return serverMuxes{root: mux, rest: restMux}, nil
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
	gate := newOryAuth(s.HydraAdminURL, s.KratosURL, s.OAuthResource, s.OAuthIssuer, s.resourceMetadataURL(), s.OAuthRequireAudience, s.AuthAdmission, s.Onboard, touch, s.apiScope())
	gate.setPlatformClientIDs(s.OAuthPlatformClients)
	gate.revocations = s.OAuthRevocations
	gate.requireVerifiedEmail = s.RequireVerifiedEmail
	gate.emailVerified = s.EmailVerified
	// The durable-credential mint verbs' class gate (round-7 F3) resolves
	// platform clients through the same operator-owned registry the audience rule
	// uses. Every feature service shares this one Base.
	if s.APIKeys != nil && s.APIKeys.Base != nil && s.APIKeys.Base.PlatformClients == nil {
		s.APIKeys.Base.PlatformClients = gate
	}
	return gate, nil
}

// apiScope is retained so existing constructors and round-14 tests keep
// compiling. Discovery no longer advertises it; the closed vocabulary is
// core.AdvertisedScopes.
func (s *Server) apiScope() string {
	return cmp.Or(s.OAuthAPIScope, DefaultAPIScope)
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
	// Render routes bex deliberately does not serve each answer a named 404
	// rather than the router's bare one (a Render CLI prints a 404's message).
	mountNotOffered(mux, notOfferedRefusals)
	return mux
}

func mountNotOffered(mux *http.ServeMux, routes map[string]core.NotOffered) {
	for pattern, refusal := range routes {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) {
			core.WriteErr(w, core.NewNotFoundError(refusal.Code, refusal.Message, nil))
		})
	}
}

// notOfferedRefusals maps every not-offered Render route pattern to its named
// 404:
//   - Render's early-access object storage (`render ea objects`) is not
//     offered (ADR018, user decision 2026-10-29, w6/097).
//   - Render's sandbox groups, imported images and snapshots are not served
//     (ADR018 § Render operation omissions, w2/m263).
var notOfferedRefusals = func() map[string]core.NotOffered {
	routes := maps.Clone(sandbox.NotOfferedRoutes)
	for _, pattern := range []string{
		"GET /v1/objects/{ownerId}/{region}",
		"PUT /v1/objects/{ownerId}/{region}/{key...}",
		"GET /v1/objects/{ownerId}/{region}/{key...}",
		"DELETE /v1/objects/{ownerId}/{region}/{key...}",
	} {
		routes[pattern] = core.NotOffered{Code: core.NotFoundCode, Message: objectStorageNotOffered}
	}
	return routes
}()

// notOfferedRoutes are answered before the Render request gate runs: a body
// the gate would refuse must still get the named 404.
var notOfferedRoutes = func() map[string]bool {
	set := make(map[string]bool, len(notOfferedRefusals))
	for pattern := range notOfferedRefusals {
		set[pattern] = true
	}
	return set
}()

// objectStorageNotOffered is the answer to every Render object storage call.
const objectStorageNotOffered = "bex does not offer Render's early-access object storage yet"

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
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: query}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: mutation}),
	})
	if err != nil {
		return schema, err
	}
	// An errored field must resolve to null. graphql-go leaks the resolver's
	// un-completed return value into the response instead, which turned every
	// `(SomeView, error)` read verb's not-found into a zero-value object rather
	// than null (w6/m44) — see gqlutil.NilOnError.
	gqlutil.NilOnError(&schema)
	return schema, nil
}

// graphqlHandler serves POST /graphql over the compiled schema, timing each
// document into the operation histogram (w3/m84 — GraphQL answers 200 with an
// `errors` array, so the HTTP metrics alone cannot say whether a mutation
// worked).
func (s *Server) graphqlHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		operation, opType, outcome := s.serveGraphQL(w, r)
		s.OriginMetrics.observeGraphQL(operation, opType, outcome, time.Since(start))
	})
}

// graphqlRequestBody is one GraphQL-over-HTTP operation (single or batch element).
type graphqlRequestBody struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

// serveGraphQL answers one GraphQL request and returns its bounded telemetry
// dimensions. The request context already carries the caller Identity (attached
// by the auth middleware), which the feature resolvers' authorize gate reads.
//
// Bodies may be a single operation object or a JSON array of operations (the
// Apollo BatchHttpLink shape). A batch shares one auth admission slot — which
// is what stops the Metrics page's fan-out from shedding the session (w4/m100).
func (s *Server) serveGraphQL(w http.ResponseWriter, r *http.Request) (operation, opType, outcome string) {
	operation, opType = gqlOperationOther, gqlTypeQuery
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, "bad request")
		return operation, opType, gqlOutcomeInvalid
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		core.WriteErrStatus(w, http.StatusBadRequest, "bad request")
		return operation, opType, gqlOutcomeInvalid
	}
	if raw[0] == '[' {
		var bodies []graphqlRequestBody
		if err := json.Unmarshal(raw, &bodies); err != nil || len(bodies) == 0 {
			core.WriteErrStatus(w, http.StatusBadRequest, "bad request")
			return operation, opType, gqlOutcomeInvalid
		}
		// Cap batch size so one POST cannot amplify into unbounded resolver work.
		const maxGraphQLBatch = 32
		if len(bodies) > maxGraphQLBatch {
			core.WriteErrStatus(w, http.StatusBadRequest, "bad request")
			return operation, opType, gqlOutcomeInvalid
		}
		// One execution deadline for the whole array: the HTTP write deadline
		// covers the response, not each member, so per-member budgets could sum
		// past it and lose every result to an edge 502 (w4/m211). Each member
		// runs within what is left.
		ctx, cancel := context.WithTimeout(r.Context(), s.execBudget())
		defer cancel()
		results := make([]*graphql.Result, len(bodies))
		for i := range bodies {
			op, typ, out, result := s.executeGraphQL(ctx, bodies[i])
			results[i] = result
			if i == 0 {
				operation, opType, outcome = op, typ, out
			} else if out != gqlOutcomeOK && outcome == gqlOutcomeOK {
				outcome = out
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(results)
		return operation, opType, outcome
	}
	var body graphqlRequestBody
	if err := json.Unmarshal(raw, &body); err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, "bad request")
		return operation, opType, gqlOutcomeInvalid
	}
	operation, opType, outcome, result := s.executeGraphQL(r.Context(), body)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
	return operation, opType, outcome
}

// executeGraphQL runs one document against the compiled schema.
func (s *Server) executeGraphQL(parent context.Context, body graphqlRequestBody) (operation, opType, outcome string, result *graphql.Result) {
	operation, opType = gqlOperationOther, gqlTypeQuery
	if strings.TrimSpace(body.Query) == "" {
		return operation, opType, gqlOutcomeInvalid, graphqlErrorResult(errors.New("query is required"))
	}
	// One parse feeds telemetry dims, the cost gate, and the scope check;
	// graphql.Do still re-parses for execution (its own AST).
	if fragments, ops, ok := parseGraphQLDocument(body.Query); ok {
		operation, opType = graphqlOperationDimsFrom(fragments, ops, body.OperationName)
		// Cost gate (w1/m65 F9): reject an over-budget document before execution.
		if err := validateGraphQLComplexityParsed(fragments, ops); err != nil {
			return operation, opType, gqlOutcomeInvalid, graphqlErrorResult(err)
		}
		if err := s.requireGraphQLScopeFrom(parent, fragments, ops, body.OperationName); err != nil {
			return operation, opType, graphqlOutcome(err), graphqlErrorResult(err)
		}
		if name, bad := invalidGraphQLText(fragments, ops, body.Variables); bad {
			err := core.NewInvalidTextError("argument", name)
			return operation, opType, graphqlOutcome(err), graphqlErrorResult(err)
		}
	}
	// Env-var reads nest under the apps Service type but live in the secrets
	// feature; inject the reader so those resolvers reach it via context (the
	// shared Service GraphQL type stays stateless — no per-server closure).
	ctx := parent
	if s.Secrets != nil {
		ctx = core.WithEnvVars(ctx, s.Secrets)
		ctx = core.WithSecretFiles(ctx, s.Secrets)
	}
	// The outboundIps field nests the apps Service's own outbound-IPs verb
	// under the same stateless type through the matching reader seam.
	if s.Apps != nil {
		ctx = core.WithOutboundIPs(ctx, s.Apps)
	}
	// graphql-go starts resolving before it looks at ctx, so an expired
	// context alone does not keep a write from starting: refuse here, at the
	// last point before dispatch (w4/m211).
	if parent.Err() != nil {
		return operation, opType, gqlOutcomeUnavailable, graphqlErrorResult(errOperationNotStarted)
	}
	// Bound execution time so a single expensive document can't tie up a
	// resolver goroutine indefinitely (F9).
	ctx, cancel := context.WithTimeout(ctx, s.execBudget())
	defer cancel()
	result = graphql.Do(graphql.Params{
		Schema:         s.schema,
		RequestString:  body.Query,
		OperationName:  body.OperationName,
		VariableValues: body.Variables,
		Context:        ctx,
	})
	result.Errors = sanitizeGraphQLErrors(result.Errors, opType == gqlTypeMutation)
	return operation, opType, graphqlResultOutcome(result.Errors), result
}

// execBudget is the GraphQL execution budget: per document, and shared by
// every member of a batch.
func (s *Server) execBudget() time.Duration {
	if s.gqlBudget > 0 {
		return s.gqlBudget
	}
	return gqlExecTimeout
}

// errOperationNotStarted answers a batch member that never ran. Earlier
// members may have committed, so it points at a re-read.
var errOperationNotStarted = core.NewUnavailableError(core.CodeOperationNotStarted,
	"this operation did not start because the request ran out of time; earlier operations in the same request may have completed, so reload to see their outcome before trying again", nil)

// graphqlErrorResult is the pre-execution error envelope (input / cost gate / scope)
// for both single-document and batch execute paths.
func graphqlErrorResult(err error) *graphql.Result {
	fe := gqlerrors.FormattedError{Message: err.Error()}
	var coded *core.CodedError
	if errors.As(err, &coded) {
		if ext := coded.Extensions(); len(ext) > 0 {
			fe.Extensions = ext
		}
	}
	return &graphql.Result{Errors: []gqlerrors.FormattedError{fe}}
}

// graphqlResultOutcome classifies a completed document by its FIRST error —
// graphql-go reports one entry per failed field, and the first is the one a
// client acts on. A parse/validation error (no wrapped resolver error) is an
// invalid document, not a server fault.
func graphqlResultOutcome(errs []gqlerrors.FormattedError) string {
	if len(errs) == 0 {
		return gqlOutcomeOK
	}
	resolverErr := resolverError(errs[0].OriginalError())
	if resolverErr == nil {
		return gqlOutcomeInvalid
	}
	return graphqlOutcome(resolverErr)
}

// queryTimeoutMessage is what a GraphQL client reads when a resolver hits
// its execution budget.
const queryTimeoutMessage = "query timed out; narrow the time range or the search and try again"

// writeOutcomeUnknownMessage is what a GraphQL client reads when a mutation
// hits its execution budget. The work is not rolled back and may still be finishing,
// so the query advice ("narrow it and try again") would be wrong here.
const writeOutcomeUnknownMessage = "the change ran out of time and may have been applied in part or in full; reload to see its outcome before trying again"

// sanitizeGraphQLErrors applies core.WriteErr's redaction policy on the GraphQL
// surface: a resolver error that is not a public/classified error (a raw
// pgx/Kubernetes failure) has its message replaced with a generic string and the
// real cause logged, so internal detail — constraint names, connection strings,
// paths — never reaches the client. Pure parse/validation errors (which wrap no
// resolver error) keep their client-needed message, as do coded and sentinel
// errors. Shares core.IsPublicError with WriteErr so REST and GraphQL redact the
// exact same set.
func sanitizeGraphQLErrors(errs []gqlerrors.FormattedError, mutation bool) []gqlerrors.FormattedError {
	for i := range errs {
		resolverErr := resolverError(errs[i].OriginalError())
		// nil → a parse/validation error that wraps no resolver error: keep it,
		// minus any variable value it echoes.
		if resolverErr == nil {
			errs[i].Message = redactInvalidValue(errs[i].Message)
			continue
		}
		// graphql-go's operation selection fails with plain errors (no path, no
		// locations) that wrap no resolver; they are the caller's to fix (w4/484).
		if isOperationSelectionError(errs[i]) {
			continue
		}
		if core.IsPublicError(resolverErr) {
			// graphql-go reads extensions off the returned error only, so a
			// CodedError a resolver wrapped (fmt.Errorf("…: %w", coded)) would
			// lose its code on GraphQL alone: REST and MCP find it with
			// errors.As (w5/m118).
			var coded *core.CodedError
			if _, has := errs[i].Extensions["code"]; !has && errors.As(resolverErr, &coded) {
				errs[i].Extensions = coded.Extensions()
			}
			continue
		}
		// A resolver that ran out of the execution budget is the caller's
		// signal to narrow the request, not an internal fault (w4/m140).
		if errors.Is(resolverErr, context.DeadlineExceeded) {
			errs[i].Message = queryTimeoutMessage
			errs[i].Extensions = map[string]any{"code": core.CodeQueryTimeout}
			// A write that timed out is not known to have done nothing.
			if mutation {
				errs[i].Message = writeOutcomeUnknownMessage
				errs[i].Extensions = map[string]any{"code": core.CodeWriteOutcomeUnknown}
			}
			continue
		}
		log.Printf("bex-api graphql: internal error: %v", resolverErr)
		errs[i].Message = "internal error"
		errs[i].Extensions = map[string]any{"code": core.CodeInternal}
	}
	return errs
}

// invalidValuePrefix matches the head of graphql-go's two value-echoing
// errors: variable coercion (`Variable "$name" got invalid value <JSON>.`)
// and an inline argument literal (`Argument "name" has invalid value
// <literal>.`), each followed by one reason per line.
var invalidValuePrefix = regexp.MustCompile(`^(Variable "\$|Argument ")([_A-Za-z][_0-9A-Za-z]*)" (?:got|has) invalid value `)

// The reason lines graphql-go's value checks produce, behind any number of
// `In field "x": ` / `In element #n: ` prefixes: kept whole, or kept up to
// the `, found <value>.` tail that echoes the value.
var (
	invalidValueReasonPath  = `^((?:In (?:field "[_A-Za-z][_0-9A-Za-z]*"|element #[0-9]+): )*)`
	invalidValueReasonWhole = regexp.MustCompile(invalidValueReasonPath + `(?:Unknown field\.|Expected "[_A-Za-z][_0-9A-Za-z]*!", found null\.|Expected "[_A-Za-z][_0-9A-Za-z]*", found not an object\.)$`)
	invalidValueReasonTyped = regexp.MustCompile(invalidValueReasonPath + `(Expected type "[_A-Za-z][_0-9A-Za-z]*"), found `)
)

// redactInvalidValue drops the value from a graphql-go variable or argument
// value error, keeping the variable or argument name and each reason: the
// value may be a write-only secret (an agent environment's modelApiKey) and
// a client never needs it echoed. A reason line in any other shape —
// including a fragment of a multi-line value — is dropped. Every other
// message is returned as is.
func redactInvalidValue(msg string) string {
	head := invalidValuePrefix.FindStringSubmatch(msg)
	if head == nil {
		return msg
	}
	out := head[1] + head[2] + `" has an invalid value.`
	_, reasons, _ := strings.Cut(msg, "\n")
	for _, line := range strings.Split(reasons, "\n") {
		if invalidValueReasonWhole.MatchString(line) {
			out += "\n" + line
		} else if m := invalidValueReasonTyped.FindStringSubmatch(line); m != nil {
			out += "\n" + m[1] + m[2] + "."
		}
	}
	return out
}

// isOperationSelectionError matches the two errors graphql-go raises before
// executing anything when operationName does not select exactly one operation.
// The wording is pinned by a test against the real executor, so an upgrade that
// rewords it fails loudly instead of reverting to "internal error".
func isOperationSelectionError(e gqlerrors.FormattedError) bool {
	if len(e.Path) > 0 || len(e.Locations) > 0 {
		return false
	}
	return e.Message == "Must provide operation name if query contains multiple operations." ||
		strings.HasPrefix(e.Message, "Unknown operation named ")
}

// resolverError peels graphql-go's *gqlerrors.Error wrapper to recover the error
// a resolver actually returned. That wrapper does not implement Unwrap, so
// errors.Is/As cannot see the underlying core sentinel/CodedError through it.
// Returns nil for a pure parse/validation error (its OriginalError field is nil).
func resolverError(orig error) error {
	var ge *gqlerrors.Error
	if errors.As(orig, &ge) {
		return ge.OriginalError
	}
	return orig
}

// MCPServer returns the MCP server with every feature's tools registered. It is
// stateless w.r.t. sessions, so one instance per Server serves stdio, every
// HTTP session and in-memory test sessions alike (w5/107). It is built once, on
// first use, which Handler makes at startup: a broken tool schema still fails
// there, and the Server's fields must be final by then.
func (s *Server) MCPServer() *mcp.Server {
	s.mcpOnce.Do(func() { s.mcpServer = s.buildMCPServer() })
	return s.mcpServer
}

// mcpSchemaCache shares tools' inferred and resolved JSON schemas across the
// MCP servers one process builds: they depend only on each tool's Go types.
// Production builds one server, but the api tests build one per test server,
// and resolving every tool's schemas each time was most of their CPU (w5/107).
var mcpSchemaCache = mcp.NewSchemaCache()

func (s *Server) buildMCPServer() *mcp.Server { return s.buildMCPServerFor(nil) }

// buildMCPServerFor builds the MCP server with every feature registered; a
// non-nil toolset narrows it to those tool names (list and call), so a narrow
// mount can never drift from /mcp: it is the same registration, filtered.
func (s *Server) buildMCPServerFor(toolset map[string]struct{}) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: mcpServerName, Version: mcpVersion}, &mcp.ServerOptions{SchemaCache: mcpSchemaCache})
	for _, f := range s.features() {
		if r, ok := f.(mcpRegistrar); ok {
			r.RegisterMCP(srv)
		}
	}
	var base *core.Base
	if s.Apps != nil {
		base = s.Apps.Base
	}
	srv.AddReceivingMiddleware(mcpArgumentValueRedaction)
	srv.AddReceivingMiddleware(mcpWorkspaceMiddleware(base, s.requireMCPScope))
	// Added last => outermost (the SDK wraps the current handler), so a call the
	// workspace/scope gate above refuses is still timed, as denied.
	if s.OriginMetrics != nil {
		srv.AddReceivingMiddleware(mcpMetricsMiddleware(s.OriginMetrics))
	}
	if toolset != nil {
		// Outermost of all: a tool outside the toolset is neither listed nor
		// reachable, and never counts as a call.
		srv.AddReceivingMiddleware(mcpToolsetMiddleware(toolset))
	}
	return srv
}

// mcpArgumentValueRedaction drops the argument value the SDK's input schema
// check echoes ("validating \"arguments\": … type: <value> has type …"),
// the MCP side of redactInvalidValue: the value may be a write-only secret
// (an agent environment's modelApiKey) put in the wrong field.
func mcpArgumentValueRedaction(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, req)
		if res, ok := result.(*mcp.CallToolResult); ok && res != nil && res.IsError {
			for _, c := range res.Content {
				if text, ok := c.(*mcp.TextContent); ok {
					text.Text = redactArgumentValue(text.Text)
				}
			}
		}
		return result, err
	}
}

// argumentValueKeyword matches the first jsonschema keyword whose message
// prints the instance; argumentTypeTail keeps a type refusal's expectation.
var (
	argumentValueKeyword = regexp.MustCompile(`(?s)\b(type|enum|const|multipleOf|minimum|maximum|exclusiveMinimum|exclusiveMaximum|minLength|maxLength|pattern|contains): .*$`)
	argumentTypeTail     = regexp.MustCompile(`(?s) (has type "[^"]*", want [^\n]*)$`)
)

// redactArgumentValue rewrites an SDK argument-validation message so it keeps
// the path and the keyword but never the value; any other text is returned
// as is.
func redactArgumentValue(text string) string {
	if !strings.HasPrefix(text, `validating "arguments": `) {
		return text
	}
	return argumentValueKeyword.ReplaceAllStringFunc(text, func(tail string) string {
		keyword, _, _ := strings.Cut(tail, ":")
		if m := argumentTypeTail.FindStringSubmatch(tail); keyword == "type" && m != nil {
			return "type: the value " + m[1]
		}
		return keyword + ": the value is not allowed"
	})
}

// MCPToolsetServer returns the narrow MCP server for a named toolset, or nil
// for an unknown name.
func (s *Server) MCPToolsetServer(name string) *mcp.Server {
	tools, ok := mcpToolsets[name]
	if !ok {
		return nil
	}
	s.toolsetMu.Lock()
	defer s.toolsetMu.Unlock()
	if srv, ok := s.toolsetServers[name]; ok {
		return srv
	}
	if s.toolsetServers == nil {
		s.toolsetServers = map[string]*mcp.Server{}
	}
	srv := s.buildMCPServerFor(tools)
	s.toolsetServers[name] = srv
	return srv
}

func (s *Server) mcpToolsetHTTPHandler(name string) http.Handler {
	srv := s.MCPToolsetServer(name)
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
}

// RunStdioToolset serves one toolset over stdio (`api mcp-stdio --toolset X`).
func (s *Server) RunStdioToolset(ctx context.Context, name string) error {
	srv := s.MCPToolsetServer(name)
	if srv == nil {
		return fmt.Errorf("unknown MCP toolset %q", name)
	}
	return srv.Run(ctx, &mcp.StdioTransport{})
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

// Blueprint validation owns a manifest limit and a JSON/multipart envelope
// allowance. Enforce its named refusal before OpenAPI reads the body, even
// when the configurable global limit is lower, higher, or disabled.
func restBodyLimit(global func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		narrow := global(next)
		blueprint := withBodyLimitMessage(apps.MaxBlueprintValidationBodyBytes, apps.ErrBlueprintTooLarge.Error())(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == apps.BlueprintValidationPath {
				blueprint.ServeHTTP(w, r)
				return
			}
			// The attachment upload streams and bounds its own body (ADR095 D2).
			if r.Method == http.MethodPost && r.URL.Path == agentsessions.AttachmentUploadPath {
				next.ServeHTTP(w, r)
				return
			}
			narrow.ServeHTTP(w, r)
		})
	}
}

// memberMutator drops members.MemberView: the Render member routes answer
// workspaces' teamMember shape.
type memberMutator struct{ svc *members.Service }

func (m memberMutator) ChangeRole(ctx context.Context, workspaceID, subject, role string) error {
	_, err := m.svc.ChangeRole(ctx, workspaceID, subject, role)
	return err
}

func (m memberMutator) Remove(ctx context.Context, workspaceID, subject string) error {
	return m.svc.Remove(ctx, workspaceID, subject)
}

// userIDLookup is the store's non-minting user-id read when it has one
// (*store.PGStore), else nil (w5/m277).
func userIDLookup(st any) core.UserIDLookup {
	if lookup, ok := st.(core.UserIDLookup); ok {
		return lookup
	}
	return nil
}
