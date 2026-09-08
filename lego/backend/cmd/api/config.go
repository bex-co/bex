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

package main

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/usage"
	"github.com/bex-co/bex/lego/backend/internal/webhooks"
)

// Config is every environment-derived value bex-api runs on, parsed and
// validated by loadConfig BEFORE main performs any irreversible side effect
// (store.Migrate, background loops, the listeners). Before this existed,
// main.go interleaved env parsing with startup: migrations had already run and
// workers were already started when a later parse could still log.Fatalf, so a
// typo'd knob crashlooped the pod repeating migrations and partial serving on
// every restart (.pm/w1/070.md). cmd/ is the only env reader — packages under
// internal/ receive values, never call os.Getenv.
//
// Fields gated behind a feature (control plane, Stripe, dunning, OpenSandbox,
// stdio) are parsed only when their gate is on, exactly as the inline reads
// they replaced were: a deployment with a stale, malformed value for an inert
// feature keeps starting.
type Config struct {
	// Mode: `api mcp-stdio` (or BEX_MCP_STDIO=1) serves only the MCP adapter
	// over stdio for a local agent (DB-free).
	MCPStdio bool

	// Core surface.
	APIAddr      string // BEX_API_ADDR, default ":8090"
	Namespace    string // BEX_API_NAMESPACE, default "default"
	CPDBURI      string // BEX_CP_DB_URI; set => bex-api also runs the control plane
	DashboardURL string
	CORSOrigin   string
	// BEX_BASE_DOMAIN names custom-domain DNS targets `<app>.<base>`
	// (docs/ADR005-custom-domain.md); unset falls back to deriving the platform
	// host from an App's status URLs.
	BaseDomain string
	// BEX_REGION is the explicit platform placement surfaced in Render resource
	// metadata. Empty is honestly omitted.
	Region       string
	APIPublicURL string // BEX_API_PUBLIC_URL (deploy-hook base URL)
	SSHHost      string
	// ADR047 D2 / m33: the gateway-shared HMAC secret for sandbox exec and the
	// internal mint verbs.
	SandboxExecSecret string
	// w1/m53 + w1/m65 F13: gate login-time invite redemption on a
	// Kratos-verified email. Secure by DEFAULT (true); BEX_REQUIRE_VERIFIED_INVITE_EMAIL=0
	// only for local dev without a verification UX (docs/ADR024-members.md).
	RequireVerifiedInviteEmail bool

	// Push transports (ADR048): validated at construction in main, which runs
	// before any side effect.
	PushProvider           string
	ExpoPushAccessToken    string
	ExpoPushURL            string
	WebPushVAPIDPublicKey  string
	WebPushVAPIDPrivateKey string
	WebPushSubscriber      string

	// Observability sources (docs/ADR010-observability.md).
	LokiURL string
	PromURL string

	// Auth (docs/ADR012-auth.md).
	HydraAdminURL  string
	KratosURL      string
	KratosAdminURL string
	OpenFGAURL     string
	OpenFGAToken   string
	OAuthIssuer    string
	OAuthResource  string
	// w1/m67 F1: narrow the empty-audience token exception to bex-provisioned
	// OAuth clients. Opt-in — see the F6 warning loadConfig emits when off.
	OAuthRequireAudience bool
	// BEX_OAUTH_PLATFORM_CLIENTS is the operator-owned comma-separated registry
	// of platform client IDs. It is deliberately outside Hydra client records.
	OAuthPlatformClients []string
	OAuthAPIScope        string

	// Ops-workspace pin (docs/ADR088-platform-observability-ui.md §4). The
	// workspace id alone arms the lifecycle guards (delete/suspend refusal,
	// invite seat/plan-gate exemption); the internal ops-role verb on the
	// cluster-internal listener additionally needs the static bearer — either
	// unset leaves that route unregistered (the internal mux's normal 404).
	OpsWorkspace string // BEX_OPS_WORKSPACE — the pinned tea-* ops workspace id
	OpsRoleToken string // BEX_OPS_ROLE_TOKEN — static bearer for the ops-role verb

	// Tenant secrets (docs/ADR013-secrets.md).
	OpenBaoURL string
	// BEX_OPENBAO_JWT_PATH overrides the pod's projected ServiceAccount token
	// path so bex-api can run off-cluster (local dev, scripts/secrets-verify.sh).
	OpenBaoJWTPath string

	// GitHub App (docs/ADR026-github-integration.md). Key parse happens at
	// construction in wireGitHubApp, before any side effect.
	GitHubAppID           string
	GitHubAppPrivateKey   string
	GitHubAppSlug         string
	GitHubAppClientID     string
	GitHubAppClientSecret string

	// Invite delivery (w4/m12): the same SMTP relay Kratos's courier uses.
	SMTPAddr     string
	SMTPFrom     string
	SMTPUsername string
	SMTPPassword string

	// Billing (w7/m47–m50, ADR040/ADR046/ADR075). Stripe secondary knobs are
	// parsed only when the gate (BEX_STRIPE_SECRET_KEY) is on, keeping a
	// deployment with stale BEX_STRIPE_* values completely inert.
	RequirePaymentMethod        paymentMethodMode
	StripeSecretKey             string
	StripeEpoch                 time.Time
	StripeEnabled               bool
	StripePublishableKey        string
	StripeAPIURL                string
	StripeCompCouponID          string
	StripePortalConfigurationID string
	StripeTaxCode               string
	StripeTaxBehavior           string
	StripeSealHours             int
	StripeSealHoursSet          bool
	StripeDunningEnabled        bool
	StripeGracePeriod           time.Duration
	StripeReconcileInterval     time.Duration
	StripeWebhookSecret         string

	// Control plane (w1/m2), parsed only when BEX_CP_DB_URI is set outside
	// stdio mode.
	CPAppsNamespace                  string        // BEX_CP_APPS_NAMESPACE, falls back to Namespace
	CPResync                         time.Duration // valid only when CPResyncSet
	CPResyncSet                      bool
	CPToken                          string
	CPAddr                           string // BEX_CP_ADDR, default ":8091"
	CPIdentity                       string // validated as a Kubernetes label value (ADR043 D9)
	BuildNamespace                   string
	UsageRetention                   int // BEX_USAGE_RETENTION_MONTHS; valid only when UsageRetentionSet
	UsageRetentionSet                bool
	AuditRetention                   int // BEX_AUDIT_RETENTION_DAYS; valid only when AuditRetentionSet
	AuditRetentionSet                bool
	WebhookBackoff                   []time.Duration // BEX_WEBHOOK_BACKOFF (dev/verification knob)
	WebhookRetention                 int             // BEX_WEBHOOK_RETENTION_DAYS (w1/m67 F3)
	WebhookKeep                      int             // BEX_WEBHOOK_RETENTION_KEEP
	MaxWebhookDeliveriesPerWorkspace int

	// Git webhooks (codex round-8 #9: replay protection needs the durable store).
	WebhookSecret       string
	GitHubWebhookSecret string

	// One-shot env-group operations (w2/m80 + name-claim audit); modes are
	// validated here, executed in main after the server is built.
	EnvGroupNameClaimAudit string
	EnvGroupPathMigration  string

	// Hosted agent sandboxes + sessions (ADR042/ADR047/ADR059/ADR062).
	AgentModelProxyURL string
	OpenSandboxURL     string
	// Derived from AgentModelProxyURL's origin (ADR062) so egress narrowing and
	// the agentsessions Service always agree on the port; validated only when
	// the OpenSandbox leg is on, like the inline parse it replaced.
	ModelProxyPort       uint16
	SandboxImage         string
	AgentSessionImage    string
	AgentSetupRegistries string
	SandboxExecURL       string
	// The Hibernated-tier object-store host the sandbox curl PUT/GET must
	// reach; empty when the tier is off. A set-but-unparseable endpoint is
	// fatal so a typo cannot arm hibernation with a policy that still
	// NXDOMAINs the presigned URL (w2/m77 live walk, curl 6).
	SnapshotEgressDomains []string

	ShellTicketSecret      string
	ShellWSURL             string
	AgentSessionGatewayURL string
	AgentGitProxyURL       string
	AgentSandboxIdleTTL    time.Duration
	AgentTurnTimeout       time.Duration

	AgentMaxLiveSandboxesPerWorkspace  int
	MaxBlueprintGroupings              int
	MaxEnvGroupsPerWorkspace           int
	MaxGitConnectionsPerWorkspace      int
	MaxRegistryCredentialsPerWorkspace int
	MaxCustomDomainsPerService         int
	MaxCustomDomainsPerWorkspace       int

	// ADR059 D3/D5 hibernation (w2/m68, armed w2/m77): all four required
	// coordinates unset ⇒ the whole tier is off. A partial set is fatal — a
	// typo'd Secret key must not silently disable hibernation.
	AgentSnapshot                       agentsessions.S3SnapshotConfig
	AgentSnapshotRetentionTTL           time.Duration
	AgentMaxPinnedSandboxesPerWorkspace int

	// Disk snapshots (docs/ADR082-persistent-disks.md D5).
	DiskSnapshot apps.S3DiskSnapshotConfig

	// Serving knobs (skipped entirely in stdio mode, like the inline reads
	// they replaced — a local agent's leftover env must not fail a subprocess).
	MaxBodyBytes                int64
	MaxQueryHours               int
	MaxSSEConns                 int64
	MaxSSEConnsPerSubject       int
	MaxSSEConnsPerWorkspace     int
	LogStreamRevalidateInterval time.Duration
	TrustedProxies              core.TrustedProxies
	RateLimitRPM                float64
	RateLimitBurst              int
	DeviceRateRPM               float64
	DeviceRateBurst             int
	WebhookRateRPM              float64
	WebhookRateBurst            int
	DeployHookLookupRPM         float64
	DeployHookLookupBurst       int
	AuthFailureRPM              float64
	AuthFailureBurst            int
	AuthMaxInflight             int
}

// loadConfig parses and validates the complete environment contract. It is
// pure over getenv/now/args (table-testable), collects every fatal problem
// instead of dying on the first, and returns startup warnings for main to log
// once. No side effects happen until it has returned nil error.
func loadConfig(getenv func(string) string, now time.Time, args []string) (*Config, []string, error) {
	p := &parser{getenv: getenv}
	cfg := &Config{}

	cfg.MCPStdio = getenv("BEX_MCP_STDIO") == "1" || (len(args) > 1 && args[1] == "mcp-stdio")
	cfg.CPDBURI = getenv("BEX_CP_DB_URI")
	// The control plane runs only outside stdio mode; every CP-gated knob below
	// follows this, matching the pre-Load inline reads.
	cpOn := cfg.CPDBURI != "" && !cfg.MCPStdio

	// Core surface.
	cfg.APIAddr = p.str("BEX_API_ADDR", ":8090")
	cfg.Namespace = p.str("BEX_API_NAMESPACE", "default")
	cfg.DashboardURL = getenv("BEX_DASHBOARD_URL")
	cfg.CORSOrigin = getenv("BEX_API_CORS_ORIGIN")
	cfg.BaseDomain = getenv("BEX_BASE_DOMAIN")
	cfg.Region = getenv("BEX_REGION")
	cfg.APIPublicURL = getenv("BEX_API_PUBLIC_URL")
	cfg.SSHHost = getenv("BEX_SSH_HOST")
	cfg.SandboxExecSecret = getenv("BEX_SANDBOX_EXEC_SECRET")
	verifiedInvite := getenv("BEX_REQUIRE_VERIFIED_INVITE_EMAIL")
	cfg.RequireVerifiedInviteEmail = verifiedInvite != "0" && !strings.EqualFold(verifiedInvite, "false")

	// Push transports.
	cfg.PushProvider = getenv("BEX_PUSH_PROVIDER")
	cfg.ExpoPushAccessToken = getenv("BEX_EXPO_PUSH_ACCESS_TOKEN")
	cfg.ExpoPushURL = getenv("BEX_EXPO_PUSH_URL")
	cfg.WebPushVAPIDPublicKey = getenv("BEX_WEBPUSH_VAPID_PUBLIC_KEY")
	cfg.WebPushVAPIDPrivateKey = getenv("BEX_WEBPUSH_VAPID_PRIVATE_KEY")
	cfg.WebPushSubscriber = getenv("BEX_WEBPUSH_SUBSCRIBER")

	// Observability.
	cfg.LokiURL = getenv("BEX_LOKI_URL")
	cfg.PromURL = getenv("BEX_PROM_URL")

	// Auth.
	cfg.HydraAdminURL = getenv("BEX_HYDRA_ADMIN_URL")
	cfg.KratosURL = getenv("BEX_KRATOS_URL")
	cfg.KratosAdminURL = getenv("BEX_KRATOS_ADMIN_URL")
	cfg.OpenFGAURL = getenv("BEX_OPENFGA_URL")
	cfg.OpenFGAToken = getenv("BEX_OPENFGA_TOKEN")
	// w1/m53 + w1/m65 F16: with the store on but OpenFGA off, checkAuthz allows
	// every relation (fail-open) AND the explicit-workspace verbs bypass
	// membership resolution, so cross-tenant isolation does not hold. A
	// multi-tenant API must FAIL CLOSED (refuse to start) rather than warn.
	// BEX_ALLOW_INSECURE_AUTHZ=1 is the documented single-member/local-dev
	// override (mirrors BEX_CP_INSECURE for BEX_CP_TOKEN).
	if cfg.OpenFGAURL == "" && cpOn {
		if getenv("BEX_ALLOW_INSECURE_AUTHZ") != "1" {
			p.errorf("BEX_OPENFGA_URL is unset while the control-plane store is on (BEX_CP_DB_URI set): authorization would be FAIL-OPEN — every workspace member gets admin-equivalent rights AND explicit-workspace verbs bypass membership resolution, so cross-tenant isolation does not hold. Refusing to start a multi-tenant API without enforced authorization. Set BEX_OPENFGA_URL, or set BEX_ALLOW_INSECURE_AUTHZ=1 to override in single-member/local dev only (docs/ADR012-auth.md).")
		} else {
			p.warnf("WARNING: BEX_OPENFGA_URL is unset while the control-plane store is on and BEX_ALLOW_INSECURE_AUTHZ=1 — authorization is FAIL-OPEN (every member admin-equivalent; explicit-workspace verbs bypass membership isolation). Safe ONLY for a single-member workspace / local dev.")
		}
	}
	cfg.OAuthIssuer = getenv("BEX_OAUTH_ISSUER")
	cfg.OAuthResource = getenv("BEX_OAUTH_RESOURCE")
	cfg.OAuthRequireAudience = getenv("BEX_OAUTH_REQUIRE_AUDIENCE") == "1"
	for _, clientID := range strings.Split(getenv("BEX_OAUTH_PLATFORM_CLIENTS"), ",") {
		if clientID = strings.TrimSpace(clientID); clientID != "" {
			cfg.OAuthPlatformClients = append(cfg.OAuthPlatformClients, clientID)
		}
	}
	cfg.OAuthAPIScope = getenv("BEX_OAUTH_API_SCOPE")
	// codex F6: an opt-in security control that ships off is invisible — a LOUD
	// WARNING on every start, not a fail-closed refusal (a hard refusal would
	// crashloop the API or force BEX_ALLOW_INSECURE_AUTHZ=1). Track: docs/ADR055.
	if cfg.OAuthResource != "" && !cfg.OAuthRequireAudience {
		p.warnf("WARNING: BEX_OAUTH_REQUIRE_AUDIENCE is off while BEX_OAUTH_RESOURCE=%q — an audience-less token "+
			"from ANY self-registered OAuth client a user consents to carries that user's full workspace rights here (codex F6, cross-tenant). "+
			"Activate: configure BEX_OAUTH_PLATFORM_CLIENTS with the operator-provisioned client IDs, then set it to 1 "+
			"(docs/ADR012-auth.md §7)", cfg.OAuthResource)
	}

	// Ops-workspace pin (docs/ADR088-platform-observability-ui.md §4).
	cfg.OpsWorkspace = getenv("BEX_OPS_WORKSPACE")
	cfg.OpsRoleToken = getenv("BEX_OPS_ROLE_TOKEN")
	if (cfg.OpsWorkspace == "") != (cfg.OpsRoleToken == "") {
		p.warnf("WARNING: exactly one of BEX_OPS_WORKSPACE/BEX_OPS_ROLE_TOKEN is set — the internal ops-role verb (docs/ADR088-platform-observability-ui.md §4) stays disabled until both are; the ops-workspace lifecycle guards key on BEX_OPS_WORKSPACE alone")
	}

	// Secrets.
	cfg.OpenBaoURL = getenv("BEX_OPENBAO_URL")
	cfg.OpenBaoJWTPath = getenv("BEX_OPENBAO_JWT_PATH")

	// GitHub App.
	cfg.GitHubAppID = getenv("BEX_GITHUB_APP_ID")
	cfg.GitHubAppPrivateKey = getenv("BEX_GITHUB_APP_PRIVATE_KEY")
	cfg.GitHubAppSlug = getenv("BEX_GITHUB_APP_SLUG")
	cfg.GitHubAppClientID = getenv("BEX_GITHUB_APP_CLIENT_ID")
	cfg.GitHubAppClientSecret = getenv("BEX_GITHUB_APP_CLIENT_SECRET")

	// SMTP.
	cfg.SMTPAddr = getenv("BEX_SMTP_ADDR")
	cfg.SMTPFrom = getenv("BEX_SMTP_FROM")
	cfg.SMTPUsername = getenv("BEX_SMTP_USERNAME")
	cfg.SMTPPassword = getenv("BEX_SMTP_PASSWORD")

	// Payment gate (ADR046/ADR075 D7): validated up front regardless of mode,
	// as before.
	mode, err := paymentMethodGate(getenv)
	p.err(err)
	cfg.RequirePaymentMethod = mode

	// Control-plane block.
	if cpOn {
		cfg.CPAppsNamespace = p.str("BEX_CP_APPS_NAMESPACE", cfg.Namespace)
		if d := getenv("BEX_CP_RESYNC"); d != "" {
			v, err := time.ParseDuration(d)
			if err != nil {
				p.errorf("bad BEX_CP_RESYNC %q: %v", d, err)
			}
			cfg.CPResync, cfg.CPResyncSet = v, err == nil
		}
		cfg.CPToken = getenv("BEX_CP_TOKEN")
		// w1/m53: the internal control-plane API grants workspace-admin and
		// cross-tenant writes — an empty BEX_CP_TOKEN must abort startup rather
		// than silently serve open. requireCPAuth logs the loud BEX_CP_INSECURE=1
		// local-dev warning itself.
		if err := requireCPAuth(cfg.CPToken, getenv("BEX_CP_INSECURE")); err != nil {
			p.errs = append(p.errs, fmt.Errorf("control plane: %w", err))
		}
		// ADR043 D9: BEX_CP_IDENTITY is projected into a label; an invalid value
		// would fail every namespace apply while ReconcileOnce collects those
		// errors per-workspace without exiting — the process would come up
		// healthy, provision nothing, and keep pruning.
		cfg.CPIdentity = getenv("BEX_CP_IDENTITY")
		if cfg.CPIdentity != "" && len(validation.IsValidLabelValue(cfg.CPIdentity)) != 0 {
			p.errorf("BEX_CP_IDENTITY %q is not a valid Kubernetes label value", cfg.CPIdentity)
		}
		cfg.UsageRetention, cfg.UsageRetentionSet = p.positiveInt("BEX_USAGE_RETENTION_MONTHS", usage.DefaultRetentionMonths)
		cfg.AuditRetention, cfg.AuditRetentionSet = p.positiveInt("BEX_AUDIT_RETENTION_DAYS", audit.DefaultRetentionDays)

		// Stripe (w7/m47–m50, ADR040): the gate resolves before any secondary
		// knob so a deployment with no runtime key stays completely inert.
		secret, epoch, stripeEnabled, err := stripeBillingGate(getenv, now)
		p.err(err)
		cfg.StripeSecretKey, cfg.StripeEpoch, cfg.StripeEnabled = secret, epoch, stripeEnabled
		if stripeEnabled {
			pk, err := stripePublishableKey(getenv, secret, cfg.RequirePaymentMethod != paymentMethodOff)
			p.err(err)
			cfg.StripePublishableKey = pk
			cfg.StripeAPIURL = getenv("BEX_STRIPE_API_URL")
			cfg.StripeCompCouponID = getenv("BEX_STRIPE_COMP_COUPON_ID")
			cfg.StripePortalConfigurationID = getenv("BEX_STRIPE_PORTAL_CONFIGURATION_ID")
			cfg.StripeTaxCode = getenv("BEX_STRIPE_TAX_CODE")
			cfg.StripeTaxBehavior = getenv("BEX_STRIPE_TAX_BEHAVIOR")
			cfg.StripeSealHours, cfg.StripeSealHoursSet = p.positiveInt("BEX_STRIPE_SEAL_HOURS", billing.DefaultSealHours)
			// The livemode parameter mirrors billing.NewStripe's derivation; the
			// gate deliberately ignores it (any mode is supported since w4/m81
			// t002 — the contract is pinned by TestDunningGate so a reintroduced
			// live fence must fail there, not in an untestable Fatal here).
			cfg.StripeDunningEnabled = dunningGate(getenv, !strings.Contains(secret, "_test_"))
			if cfg.StripeDunningEnabled {
				cfg.StripeGracePeriod = p.minuteDuration("BEX_STRIPE_GRACE_PERIOD", "168h")
				cfg.StripeReconcileInterval = p.minuteDuration("BEX_STRIPE_RECONCILE_INTERVAL", "5m")
			}
			cfg.StripeWebhookSecret = getenv("BEX_STRIPE_WEBHOOK_SECRET")
		}

		// Outbound-webhook delivery worker knobs (w3/m11 + w1/m67 F3).
		backoff, err := webhooks.ParseBackoff(getenv("BEX_WEBHOOK_BACKOFF"))
		p.err(err)
		cfg.WebhookBackoff = backoff
		cfg.WebhookRetention = p.quietInt("BEX_WEBHOOK_RETENTION_DAYS", "0")
		cfg.WebhookKeep = p.quietInt("BEX_WEBHOOK_RETENTION_KEEP", "0")
		cfg.MaxWebhookDeliveriesPerWorkspace = p.zeroableInt(
			"BEX_MAX_WEBHOOK_DELIVERIES_PER_WORKSPACE", webhooks.DefaultMaxDeliveriesPerWorkspace)
	}
	// The cluster-internal listener address serves the control plane and/or the
	// ADR088 ops-role verb; parse it whenever either will listen (a plain
	// default read with no failure mode — an inert deployment's stale value
	// still can't fail startup).
	if cpOn || (cfg.OpsWorkspace != "" && cfg.OpsRoleToken != "" && !cfg.MCPStdio) {
		cfg.CPAddr = p.str("BEX_CP_ADDR", ":8091")
	}
	cfg.BuildNamespace = getenv("BEX_BUILD_NAMESPACE")

	// Git webhook secrets: codex round-8 #9 — a configured webhook without the
	// durable replay-claim store must be rejected up front.
	cfg.WebhookSecret = getenv("BEX_WEBHOOK_SECRET")
	cfg.GitHubWebhookSecret = getenv("BEX_GITHUB_WEBHOOK_SECRET")
	p.err(validateWebhookReplayConfig(cfg.WebhookSecret, cfg.GitHubWebhookSecret, cpOn))

	// One-shot env-group operation modes (values checked here; execution stays
	// in main, deliberately after the server is built).
	cfg.EnvGroupNameClaimAudit = strings.TrimSpace(getenv("BEX_ENV_GROUP_NAME_CLAIM_AUDIT"))
	if m := cfg.EnvGroupNameClaimAudit; m != "" && m != "dry-run" && m != "apply" {
		p.errorf("BEX_ENV_GROUP_NAME_CLAIM_AUDIT must be dry-run or apply")
	}
	cfg.EnvGroupPathMigration = strings.TrimSpace(getenv("BEX_ENV_GROUP_PATH_MIGRATION"))
	if m := cfg.EnvGroupPathMigration; m != "" && m != "dry-run" && m != "apply" {
		p.errorf("BEX_ENV_GROUP_PATH_MIGRATION must be dry-run or apply")
	}

	// Agent-session snapshot store (ADR059 D3/D5): a partial coordinate set is
	// fatal — a typo'd Secret key must not silently disable hibernation.
	cfg.AgentSnapshot = agentsessions.S3SnapshotConfig{
		Endpoint:  getenv("BEX_AGENT_SNAPSHOT_S3_ENDPOINT"),
		Bucket:    getenv("BEX_AGENT_SNAPSHOT_S3_BUCKET"),
		Region:    getenv("BEX_AGENT_SNAPSHOT_S3_REGION"),
		Prefix:    getenv("BEX_AGENT_SNAPSHOT_S3_PREFIX"),
		AccessKey: getenv("BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY"),
		SecretKey: getenv("BEX_AGENT_SNAPSHOT_S3_SECRET_KEY"),
	}
	if err := cfg.AgentSnapshot.Validate(); err != nil {
		p.errorf("agent-session hibernation config: %v", err)
	}

	// Hosted sandboxes (ADR042/ADR047/ADR062).
	cfg.AgentModelProxyURL = getenv("BEX_AGENT_MODEL_PROXY_URL")
	cfg.OpenSandboxURL = getenv("BEX_OPENSANDBOX_URL")
	if cfg.OpenSandboxURL != "" {
		// Both defaults are digest-pinned (w7/m85): a sandbox template names an
		// image bex RUNS. BEX_AGENT_SESSION_IMAGE is normally supplied by
		// lego/operator/config/api/deployment.yaml — this default is the floor
		// for a deployment that omits it, not the production value.
		cfg.SandboxImage = p.str("BEX_SANDBOX_IMAGE", "docker.io/library/alpine:3@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b")
		cfg.AgentSessionImage = p.str("BEX_AGENT_SESSION_IMAGE", "ghcr.io/bex-co/bex-agent-sandbox@sha256:4faafb4ad14e6d76be076fecffe1b02c06d21ec23d9ce1bba780da2af37c698a")
		cfg.AgentSetupRegistries = getenv("BEX_AGENT_SETUP_REGISTRIES")
		cfg.SandboxExecURL = getenv("BEX_SANDBOX_EXEC_URL")
		port, err := modelProxyPort(cfg.AgentModelProxyURL)
		p.err(err)
		cfg.ModelProxyPort = port
		if strings.TrimSpace(cfg.AgentSnapshot.Bucket) != "" {
			host, err := sessionegress.SnapshotEndpointHost(cfg.AgentSnapshot.Endpoint)
			switch {
			case err != nil:
				p.errorf("agent-session snapshot egress: %v", err)
			case host == "":
				p.errorf("agent-session snapshot egress: snapshot endpoint host is empty")
			default:
				cfg.SnapshotEgressDomains = []string{host}
			}
		}
		// These are exactly the inputs sessionegress.NewManager will normalize
		// at wiring time — reject a malformed registry catalog here, before
		// any side effect.
		if err := (sessionegress.Config{
			SetupRegistryDomains: sessionegress.RegistryConfig(cfg.AgentSetupRegistries),
			ModelProxyPort:       cfg.ModelProxyPort,
			SnapshotStoreDomains: cfg.SnapshotEgressDomains,
		}).Validate(); err != nil {
			p.err(err)
		}
	}

	// Agent sessions / Browser Web Shell (docs/ADR035-ssh.md, ADR047, ADR059).
	cfg.ShellTicketSecret = getenv("BEX_SHELL_TICKET_SECRET")
	cfg.ShellWSURL = getenv("BEX_SHELL_WS_URL")
	cfg.AgentSessionGatewayURL = getenv("BEX_AGENT_SESSION_GATEWAY_URL")
	cfg.AgentGitProxyURL = getenv("BEX_AGENT_GIT_PROXY_URL")
	cfg.AgentSandboxIdleTTL = p.zeroableDuration("BEX_AGENT_SANDBOX_IDLE_TTL", 30*time.Minute)
	// w5/m80 t002: one turn's wall-clock bound. A 0/invalid value falls back to
	// the Service's 30m default (the bound is never disabled — that is the
	// 4h-hang bug this fixes).
	cfg.AgentTurnTimeout = p.zeroableDuration("BEX_AGENT_TURN_TIMEOUT", 30*time.Minute)
	cfg.AgentMaxLiveSandboxesPerWorkspace = p.zeroableInt("BEX_AGENT_MAX_LIVE_SANDBOXES_PER_WORKSPACE", 5)
	cfg.MaxBlueprintGroupings = p.zeroableInt("BEX_MAX_BLUEPRINT_GROUPINGS", 1000)
	// Round-11 #3, ADR075 §2, codex-security geyRc8 F1 + round 18 quotas.
	cfg.MaxEnvGroupsPerWorkspace = p.zeroableInt("BEX_MAX_ENV_GROUPS_PER_WORKSPACE", 100)
	cfg.MaxGitConnectionsPerWorkspace = p.zeroableInt("BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE", 10)
	cfg.MaxRegistryCredentialsPerWorkspace = p.zeroableInt("BEX_MAX_REGISTRY_CREDS_PER_WORKSPACE", 50)
	cfg.MaxCustomDomainsPerService = p.zeroableInt("BEX_MAX_CUSTOM_DOMAINS_PER_SERVICE", 100)
	cfg.MaxCustomDomainsPerWorkspace = p.zeroableInt("BEX_MAX_CUSTOM_DOMAINS_PER_WORKSPACE", 500)
	cfg.AgentSnapshotRetentionTTL = p.zeroableDuration("BEX_AGENT_SNAPSHOT_RETENTION", 7*24*time.Hour)
	cfg.AgentMaxPinnedSandboxesPerWorkspace = p.zeroableInt("BEX_AGENT_MAX_PINNED_SANDBOXES_PER_WORKSPACE", 10)

	// Disk snapshots (ADR082 D5).
	cfg.DiskSnapshot = apps.S3DiskSnapshotConfig{
		Endpoint:  getenv("BEX_DISK_SNAPSHOT_ENDPOINT"),
		Bucket:    getenv("BEX_DISK_SNAPSHOT_BUCKET"),
		Prefix:    getenv("BEX_DISK_SNAPSHOT_PREFIX"),
		Region:    getenv("BEX_DISK_SNAPSHOT_REGION"),
		AccessKey: getenv("BEX_DISK_SNAPSHOT_ACCESS_KEY"),
		SecretKey: getenv("BEX_DISK_SNAPSHOT_SECRET_KEY"),
	}

	// Serving knobs — skipped in stdio mode (the HTTP surface never starts).
	if !cfg.MCPStdio {
		cfg.MaxBodyBytes = int64(p.quietInt("BEX_MAX_BODY_BYTES", "2097152"))
		cfg.MaxQueryHours = p.quietInt("BEX_MAX_QUERY_HOURS", "720")
		cfg.MaxSSEConns = int64(p.quietInt("BEX_MAX_SSE_CONNS", "100"))
		cfg.MaxSSEConnsPerSubject = p.quietInt("BEX_MAX_SSE_CONNS_PER_SUBJECT", "5")
		cfg.MaxSSEConnsPerWorkspace = p.quietInt("BEX_MAX_SSE_CONNS_PER_WORKSPACE", "20")
		// w4/034: the live log tail's authorization watchdog cadence. Negative
		// disables.
		cfg.LogStreamRevalidateInterval = p.signedDuration("BEX_LOG_STREAM_REVALIDATE_INTERVAL", logs.DefaultRevalidateInterval)
		// Trusted-proxy CIDRs for rate-limit identity (w4/m33 P2 register).
		// Malformed ⇒ fail closed.
		proxies, err := core.ParseTrustedProxies(getenv("BEX_TRUSTED_PROXY_CIDRS"))
		if err != nil {
			p.errorf("bad BEX_TRUSTED_PROXY_CIDRS: %v", err)
		}
		cfg.TrustedProxies = proxies
		// Rate limiting + request caps (w7/m3, w4/m31, w7/m60, w1/m67 F1). A
		// rate of 0 disables the limiter in question.
		cfg.RateLimitRPM, cfg.RateLimitBurst = p.rateLimit("BEX_RATE_LIMIT", "500", "BEX_RATE_BURST", "0")
		cfg.DeviceRateRPM, cfg.DeviceRateBurst = p.rateLimit("BEX_DEVICE_RATE_LIMIT", "30", "BEX_DEVICE_RATE_BURST", "0")
		cfg.WebhookRateRPM, cfg.WebhookRateBurst = p.rateLimit("BEX_WEBHOOK_RATE_LIMIT", "600", "BEX_WEBHOOK_RATE_BURST", "0")
		cfg.DeployHookLookupRPM, cfg.DeployHookLookupBurst = p.rateLimit("BEX_DEPLOY_HOOK_LOOKUP_RATE_LIMIT", "60", "BEX_DEPLOY_HOOK_LOOKUP_RATE_BURST", "10")
		cfg.AuthFailureRPM, cfg.AuthFailureBurst = p.rateLimit("BEX_AUTH_FAILURE_LIMIT", "60", "BEX_AUTH_FAILURE_BURST", "0")
		inflight, err := strconv.Atoi(p.str("BEX_AUTH_MAX_INFLIGHT", "64"))
		if err != nil {
			p.errorf("bad BEX_AUTH_MAX_INFLIGHT: %v", err)
		}
		cfg.AuthMaxInflight = inflight
	}

	return cfg, p.warnings, errors.Join(p.errs...)
}

// modelProxyPort derives the sessionegress model-proxy port from the
// BEX_AGENT_MODEL_PROXY_URL origin (ADR062), so the egress narrowing and the
// agentsessions Service always agree on the port. Empty ⇒ 0 and agent-session
// mutation stays unavailable; a URL without an explicit port defaults to the
// gateway's :8084 listener. A malformed URL or port fails startup.
func modelProxyPort(raw string) (uint16, error) {
	if raw == "" {
		return 0, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return 0, fmt.Errorf("bad BEX_AGENT_MODEL_PROXY_URL %q: %v", raw, err)
	}
	port := u.Port()
	if port == "" {
		return 8084, nil
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("bad BEX_AGENT_MODEL_PROXY_URL port %q", port)
	}
	return uint16(n), nil
}

// parser accumulates the three failure policies every env knob resolves to
// (.pm/w1/070.md — previously seven near-duplicate parsers with silently
// different failure semantics):
//
//   - fatal: the error is collected and startup aborts before any side effect;
//   - warn: the default is kept and the message is logged once at startup;
//   - quiet: best-effort — unset/malformed silently become the knob's
//     documented disabled/default sentinel.
type parser struct {
	getenv   func(string) string
	warnings []string
	errs     []error
}

func (p *parser) errorf(format string, args ...any) {
	p.errs = append(p.errs, fmt.Errorf(format, args...))
}

func (p *parser) err(e error) {
	if e != nil {
		p.errs = append(p.errs, e)
	}
}

func (p *parser) warnf(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

// str: unset ⇒ def (no failure mode).
func (p *parser) str(name, def string) string {
	if v := p.getenv(name); v != "" {
		return v
	}
	return def
}

// quietInt — policy "quiet": an integer tuning knob parsed best-effort; unset
// ⇒ the default, malformed ⇒ 0 (each such knob's documented disabled/default
// sentinel).
func (p *parser) quietInt(name, def string) int {
	n, _ := strconv.Atoi(p.str(name, def))
	return n
}

// positiveInt — policy "warn": an integer ≥ 1 knob; set-but-invalid keeps the
// caller's default and warns; unset is silently ok=false.
func (p *parser) positiveInt(name string, def any) (int, bool) {
	v := p.getenv(name)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		p.warnf("%s=%q invalid (want integer ≥ 1); using default %v", name, v, def)
		return 0, false
	}
	return n, true
}

// zeroableDuration — policy "warn": a duration knob that accepts an explicit
// disabling zero; unset ⇒ def; malformed or negative ⇒ warn and keep def.
func (p *parser) zeroableDuration(name string, def time.Duration) time.Duration {
	v := p.getenv(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		p.warnf("%s=%q invalid (want a non-negative Go duration, 0 to disable); using default %s", name, v, def)
		return def
	}
	return d
}

// zeroableInt — policy "warn": an integer ≥ 0 knob (0 disables); invalid ⇒
// warn and keep def.
func (p *parser) zeroableInt(name string, def int) int {
	v := p.getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		p.warnf("%s=%q invalid (want an integer >= 0, 0 to disable); using default %d", name, v, def)
		return def
	}
	return n
}

// signedDuration — policy "fatal": a duration knob whose NEGATIVE value
// disables the feature rather than being invalid (the revalidation watchdogs);
// malformed is fatal.
func (p *parser) signedDuration(name string, def time.Duration) time.Duration {
	v := p.getenv(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		p.errorf("bad %s %q: %v", name, v, err)
	}
	return d
}

// minuteDuration — policy "fatal": a duration knob with a floor of one minute.
func (p *parser) minuteDuration(name, def string) time.Duration {
	d, err := time.ParseDuration(p.str(name, def))
	if err != nil || d < time.Minute {
		p.errorf("%s must be a duration >= 1m: %v", name, err)
	}
	return d
}

// rateLimit — one of the requests/min rate-limit env pairs: the fill rate
// (fatal when malformed) plus its best-effort burst companion (policy "quiet").
func (p *parser) rateLimit(limitVar, limitDef, burstVar, burstDef string) (float64, int) {
	raw := p.str(limitVar, limitDef)
	rpm, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		p.errorf("bad %s %q: %v", limitVar, raw, err)
	}
	return rpm, p.quietInt(burstVar, burstDef)
}
