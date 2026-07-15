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

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/deploys"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/environments"
	"github.com/bex-co/bex/lego/backend/internal/events"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/notifications"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/projects"
	"github.com/bex-co/bex/lego/backend/internal/registrycreds"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
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
	Postgres      *postgres.Service
	KeyValue      *keyvalue.Service
	Secrets       *secrets.Service
	EnvGroups     *envgroups.Service
	Workspaces    *workspaces.Service
	Members       *members.Service
	Usage         *usage.Service
	Deploys       *deploys.Service
	Events        *events.Service
	Audit         *audit.Service
	GitHub        *github.Service
	Notifications *notifications.Service
	Projects      *projects.Service
	Environments  *environments.Service
	RegistryCreds *registrycreds.Service
	Webhooks      *webhooks.Service

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

	// WebhookSecret is the shared HMAC-SHA256 key the git push webhook verifies
	// signatures against; empty disables the endpoint (it 503s). The webhook sits
	// OUTSIDE the OAuth gate — its signature is its authentication.
	WebhookSecret string
	// GitHubWebhookSecret is the GitHub App's webhook HMAC key
	// (BEX_GITHUB_WEBHOOK_SECRET) — a second accepted key on the same endpoint so
	// app-signed pushes redeploy hands-free. The endpoint 503s only when both
	// this and WebhookSecret are empty (docs/ADR026-github-integration.md).
	GitHubWebhookSecret string

	// RateLimiter, when set, enforces per-caller token-bucket limits on the three
	// auth-gated surfaces (REST, GraphQL, MCP). nil disables rate limiting. The
	// webhook and healthz endpoints are intentionally exempt.
	RateLimiter *RateLimiter
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
	Store          apps.IntentStore
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
	// per-service events feed (w3/m7) — a VIEW over the deploys + audit_events
	// rows the store already holds, adding no writes of its own. nil => the feed
	// answers core.ErrEventsUnavailable (both its sources are control-plane tables).
	EventStore events.EventStore
	// BaseDomain is BEX_BASE_DOMAIN (the platform wildcard domain, e.g. "onbex.co")
	// — the apps service names custom-domain DNS targets `<app>.<BaseDomain>` from it.
	// Empty falls back to deriving the platform host from an App's status URLs.
	BaseDomain string
	// Workspace lifecycle (w6/m1): the control-plane store seam + the OpenFGA
	// grant/revoke sides + the projector nudge. All nil when BEX_CP_DB_URI is
	// unset — the workspace verbs then answer ErrWorkspacesUnavailable.
	WorkspaceStore   workspaces.WorkspaceStore
	WorkspaceGranter workspaces.WorkspaceGranter
	WorkspaceRevoker workspaces.WorkspaceRevoker
	WorkspaceKick    func()
	WorkspacePurgers []workspaces.WorkspacePurger
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
	// Identities resolves owner/member email + MFA for the owners/members read
	// API (w6/m2) — Kratos' admin API (BEX_KRATOS_ADMIN_URL). Nil omits those
	// fields (honest subset) rather than failing the request.
	Identities workspaces.IdentityReader
	// Audit, when set (store + Prom-independent — only BEX_CP_DB_URI is
	// needed), backs the audit-log read verb (w4/m10). Constructed and its
	// retention loop started in cmd/api/main.go, same as Usage. nil => the
	// verb reports core.ErrAuditUnavailable (503).
	Audit *audit.Service
	// GitHub App integration (docs/ADR026-github-integration.md). GitHubClient is the
	// GitHub REST client (nil when BEX_GITHUB_APP_* unset); GitHubStore is the
	// git_connections store (nil when BEX_CP_DB_URI unset). Either nil => the
	// git-connect verbs report core.ErrGitHubUnavailable. GitHubStateSecret is
	// the app private key's PEM bytes, reused to HMAC-sign the short-lived browser
	// callback state. DashboardURL is where the install callback redirects.
	GitHubClient      github.APIClient
	GitHubStore       github.ConnectionStore
	GitHubStateSecret []byte
	DashboardURL      string

	// RegistryCredsStore, when set (the control-plane store is wired), backs
	// the registry-credentials feature (w2/m14) — CRUD for a workspace's
	// stored external-registry credentials. nil => those verbs report
	// core.ErrRegistryCredentialsUnavailable. The secret store is the shared
	// `Secrets` field above (same OpenBao instance the env-vars feature uses).
	RegistryCredsStore registrycreds.CredentialStore

	// Per-workspace resource caps (w7/m9). 0 = unlimited (default; byte-identical
	// to before). Only enforced when the caller's tenant is resolvable (store on +
	// bound caller). Render-Hobby-anchored defaults set via BEX_MAX_SERVICES (25),
	// BEX_MAX_POSTGRES (1), BEX_MAX_KEYVALUES (1).
	MaxServices  int
	MaxPostgres  int
	MaxKeyValues int
	// NotificationsStore, when set (the control-plane store is wired), backs
	// the deploy-notification settings verbs (w3/m9). nil => those verbs
	// report core.ErrNotificationsUnavailable (503). Delivery reuses Mailer
	// above (mailer.SMTP satisfies both members.Mailer and
	// notifications.Mailer structurally) — nil Mailer leaves NotifyDeploy a
	// silent no-op, same degrade-quietly shape as invite delivery.
	NotificationsStore notifications.NotificationsStore
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
	// One selection store shared by the workspace-select tools (write) and the
	// apps/postgres list tools (read) — w6/m2/t005. Always wired: with no MCP
	// transport in use, it simply never gets a Get/Set call.
	selections := core.NewWorkspaceSelections()
	// The GitHub-connect service is also the apps deploy path's clone-token seam
	// (docs/ADR026-github-integration.md), so build it once and share it. Always
	// non-nil; its verbs 503 until BEX_GITHUB_APP_* + the store are wired.
	gh := &github.Service{
		Base:         base,
		GitHub:       d.GitHubClient,
		Store:        d.GitHubStore,
		StateSecret:  d.GitHubStateSecret,
		DashboardURL: d.DashboardURL,
		Selections:   selections,
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
	pg := &postgres.Service{Base: base, Selections: selections, MaxPostgres: d.MaxPostgres}
	kv := &keyvalue.Service{Base: base, Selections: selections, MaxKeyValues: d.MaxKeyValues}
	// Usage is constructed and its metering loop started in cmd/api/main.go
	// (before NewServer runs), so it can't take Selections as a constructor
	// arg like pg/kv above — wire it onto the already-built pointer instead, so
	// get_usage shares the SAME per-session selection store as every other
	// workspace-scoped MCP tool (w6/m18).
	if d.Usage != nil {
		d.Usage.Selections = selections
	}
	return &Server{
		Apps: &apps.Service{Base: base, Store: d.Store, BaseDomain: d.BaseDomain, DashboardHost: hostOf(d.DashboardURL), Selections: selections, GitHub: gh.DeployTokenSource(), RegistryCreds: rc.DeployPullSecretSource(), MaxServices: d.MaxServices, Blueprints: d.BlueprintsStore},
		Logs: &logs.Service{Base: base, PodLogs: d.PodLogs, PodLogsFollow: d.PodLogsFollow, History: d.LogHistory, LabelValues: d.LogLabelValues, BuildNamespace: d.DeployBuildNamespace},
		Metrics: &metrics.Service{
			Base:                       base,
			ResourceMetrics:            d.ResourceMetrics,
			ResourceMetricsRange:       d.ResourceMetricsRange,
			RequestMetrics:             d.RequestMetrics,
			MonthToDateBandwidthSource: d.MonthToDateBandwidth,
			MetricsFilterValuesSource:  d.MetricsFilterValues,
			DiskUsage:                  d.DiskUsage,
			DBConnections:              d.DBConnections,
			ReplicationLag:             d.ReplicationLag,
			KeyValueStats:              d.KeyValueStats,
		},
		APIKeys:   &apikeys.Service{Base: base, APIKeys: d.APIKeys, Binding: d.KeyBinder, Selections: selections},
		Postgres:  pg,
		KeyValue:  kv,
		Secrets:   &secrets.Service{Base: base, Store: d.Secrets},
		EnvGroups: &envgroups.Service{Base: base, Store: d.Secrets},
		Deploys: &deploys.Service{
			Base:              base,
			Store:             d.DeployStore,
			BuildNamespace:    d.DeployBuildNamespace,
			DeployHookBaseURL: d.DeployHookBaseURL,
			DeployHookLimiter: deploys.NewDeployHookRateLimiter(deploys.DefaultDeployHookRPM, deploys.DefaultDeployHookBurst),
		},
		Events: &events.Service{Base: base, Store: d.EventStore},
		Workspaces: &workspaces.Service{
			Base:         base,
			Store:        d.WorkspaceStore,
			Granter:      d.WorkspaceGranter,
			Revoker:      d.WorkspaceRevoker,
			Kick:         d.WorkspaceKick,
			Purgers:      d.WorkspacePurgers,
			Identities:   d.Identities,
			Selections:   selections,
			MaxServices:  d.MaxServices,
			MaxPostgres:  d.MaxPostgres,
			MaxKeyValues: d.MaxKeyValues,
		},
		Members: &members.Service{
			Base:          base,
			Store:         d.MembersStore,
			Granter:       d.MembersGranter,
			Revoker:       d.MembersRevoker,
			Mailer:        d.Mailer,
			InviteBaseURL: d.InviteBaseURL,
			Identities:    identityEmailLookup{d.Identities},
		},
		Notifications: &notifications.Service{
			Base:       base,
			Store:      d.NotificationsStore,
			Mailer:     d.Mailer,
			Identities: identityEmailLookup{d.Identities},
		},
		Projects: &projects.Service{
			Base:       base,
			Store:      d.ProjectsStore,
			Databases:  pg,
			KeyValues:  kv,
			Selections: selections,
		},
		Environments: &environments.Service{
			Base:      base,
			Store:     d.EnvironmentsStore,
			Databases: pg,
			KeyValues: kv,
		},
		GitHub:        gh,
		RegistryCreds: rc,
		Webhooks:      &webhooks.Service{Base: base, Store: d.WebhookStore, Selections: selections},
		Onboard:       d.Onboard,
		Usage:         d.Usage,
		Audit:         d.Audit,
	}
}

// identityEmailLookup adapts workspaces.IdentityReader to members.EmailLookup
// (the two packages can't share the interface directly — IdentityReader.Lookup
// returns workspaces.IdentityAttrs, a type members doesn't import). A nil
// Identities (BEX_KRATOS_ADMIN_URL unset) degrades to an honest miss rather
// than panicking on a nil interface call.
type identityEmailLookup struct {
	Identities workspaces.IdentityReader
}

func (a identityEmailLookup) LookupEmail(ctx context.Context, subject string) (string, bool) {
	if a.Identities == nil {
		return "", false
	}
	attrs, ok := a.Identities.Lookup(ctx, subject)
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
	auth, err := s.authMiddleware()
	if err != nil {
		return nil, err
	}
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
	// The git push webhook authenticates by HMAC signature, not the OAuth gate,
	// so it mounts directly (ahead of the /v1/ wildcard — a more specific pattern
	// wins in net/http's mux). A git host can't present a bearer token.
	if s.Apps != nil {
		mux.Handle("POST /v1/webhooks/git", &apps.GitWebhook{Svc: s.Apps, Secret: s.WebhookSecret, GitHubSecret: s.GitHubWebhookSecret})
	}
	// Deploy hooks authenticate with the unguessable URL token itself. Mount the
	// whole prefix outside OAuth so malformed credentials containing an extra
	// slash still reach the hook handler's uniform 404 instead of falling through
	// to the authenticated /v1 wildcard and becoming a distinguishing 401.
	if s.Deploys != nil {
		hook := s.Deploys.DeployHookHandler()
		mux.Handle("/v1/deploy-hooks", hook)
		mux.Handle("/v1/deploy-hooks/", hook)
	}
	// All three adapters sit behind the same auth gate, with rate limiting inside
	// the auth wrapper so the limiter keys on the resolved caller Identity. The
	// gate itself recognizes github's one exact signed-state callback exception;
	// keeping the mount here ensures every other /v1 route stays covered.
	rl := s.rateLimitMiddleware()
	mux.Handle("/v1/", auth(rl(s.restHandler())))
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

	return withSecurityHeaders(withBodyLimit(s.MaxBodyBytes)(withCORS(s.CORSOrigin, mux))), nil
}

// rateLimitMiddleware returns the rate-limiting middleware when a RateLimiter
// is configured, or a no-op pass-through when it is nil (disabled).
func (s *Server) rateLimitMiddleware() func(http.Handler) http.Handler {
	if s.RateLimiter == nil {
		return func(h http.Handler) http.Handler { return h }
	}
	return s.RateLimiter.Middleware
}

// authMiddleware builds the auth gate, validating its configuration up front so a
// misconfigured binary refuses to start.
func (s *Server) authMiddleware() (func(http.Handler) http.Handler, error) {
	if s.HydraAdminURL == "" {
		return nil, core.Err(errNoHydraURL)
	}
	// Feed the gate the api-key last-used recorder when the feature is wired, so a
	// successful API-key introspection stamps the key off the request path (w4/m13).
	var touch func(string)
	if s.APIKeys != nil {
		touch = s.APIKeys.TouchAPIKey
	}
	return newOryAuth(s.HydraAdminURL, s.KratosURL, s.OAuthResource, s.OAuthIssuer, s.resourceMetadataURL(), s.Onboard, touch).middleware, nil
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
func (s *Server) restHandler() http.Handler {
	mux := http.NewServeMux()
	for _, f := range s.features() {
		if r, ok := f.(restRegistrar); ok {
			r.RegisterREST(mux)
		}
	}
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
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
	return srv
}

// mcpHTTPHandler serves the MCP streamable-HTTP transport (mounted at /mcp behind
// the same auth gate as REST/GraphQL).
func (s *Server) mcpHTTPHandler() http.Handler {
	srv := s.MCPServer()
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

// RunStdio serves the MCP adapter over stdio — the transport a local agent
// launches bex as a subprocess with. The trust boundary is the process itself
// (no bearer applies); the HTTP transport keeps the gate. Blocks until the
// client disconnects or ctx is cancelled.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.MCPServer().Run(ctx, &mcp.StdioTransport{})
}
