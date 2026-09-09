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

// Package apps is the App-lifecycle feature: the list/get read side and the
// restart/suspend/resume write side, projected as Render's "service" shape. The
// Service holds the business logic once; the rest/graphql/mcp files are thin
// registration fragments over it, so the three surfaces cannot drift.
package apps

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"golang.org/x/net/http/httpguts"
	"golang.org/x/sync/singleflight"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Service turns user intent (restart / suspend / resume) into App CR spec
// patches, and reads Apps as the Render service shape. It is a thin policy layer
// — the operator does the mechanism.
type Service struct {
	*core.Base
	// DomainOwnership verifies an app-bound DNS TXT challenge before any custom
	// host enters App.spec and becomes routable. nil uses the system resolver;
	// tests inject a deterministic verifier.
	DomainOwnership DomainOwnershipVerifier
	// DiskSnapshots lists the objects the operator's nightly snapshot Job
	// writes, and SnapshotSecret signs the 24-hour keys a listing hands out and
	// a restore hands back (docs/ADR082-persistent-disks.md D5). Either unset ⇒
	// the two snapshot verbs report unavailable; disks themselves still work.
	DiskSnapshots  DiskSnapshotLister
	SnapshotSecret []byte
	// Owners and Metadata are the shared Render resource-metadata dependencies.
	// The neutral AppView stays independent of nested REST owner wire shapes.
	Owners   resourcemeta.OwnerResolver
	Metadata resourcemeta.Config
	// BaseDomain is the PSL-safe platform wildcard domain (BEX_BASE_DOMAIN)
	// — the same value the operator computes app URLs from. The custom-domain DNS
	// instructions need it to name the CNAME/ALIAS target `<app>.<BaseDomain>` the
	// tenant points their record at. Empty falls back to deriving the platform host
	// from the App's status URLs (docs/ADR005-custom-domain.md).
	BaseDomain string
	// DashboardHost is the bare hostname of BEX_DASHBOARD_URL (e.g.
	// "dashboard.bex.co"), reserved so no tenant can claim the control-plane
	// dashboard host as a custom domain (w7/m6). Empty => not reserved (the
	// base-domain guard still covers the `*.<BaseDomain>` platform namespace).
	DashboardHost string
	// MaxCustomDomainsPerService and MaxCustomDomainsPerWorkspace cap
	// custom-domain cardinality (codex-security round 18;
	// BEX_MAX_CUSTOM_DOMAINS_PER_SERVICE / BEX_MAX_CUSTOM_DOMAINS_PER_WORKSPACE,
	// defaults 100/500; 0 disables). Every verified host fans out into an
	// Ingress rule, a cert-manager TLS entry, and activator/static-server
	// host-cache entries, so unbounded churn would exhaust shared
	// certificate/routing/control-plane resources. A claim beyond either cap is
	// refused with the coded CUSTOM_DOMAIN_LIMIT (409), identical across
	// REST/GraphQL/MCP; a create/blueprint host set over the per-service cap is
	// a 400 (the CRD schema's MaxItems is the admission backstop). The
	// per-workspace count needs the control-plane store; storeless mode keeps
	// only the per-service cap.
	MaxCustomDomainsPerService   int
	MaxCustomDomainsPerWorkspace int
	// SSHHost is the public SSH gateway hostname (BEX_SSH_HOST). When set,
	// eligible paid, long-running services advertise Render-compatible
	// serviceDetails.sshAddress values. Authorization and runtime readiness are
	// still checked by the gateway for every connection.
	SSHHost string
	// ShellTicketSecret and ShellWSURL wire the Browser Web Shell
	// (docs/ADR035-ssh.md § Browser Web Shell). Secret is the HMAC key
	// (BEX_SHELL_TICKET_SECRET) shared only with the isolated gateway;
	// ShellWSURL (BEX_SHELL_WS_URL) is the browser-reachable gateway WebSocket
	// origin handed to the terminal. Either empty => CreateShellSession returns
	// ErrShellUnavailable (503) and native `ssh` is unaffected. bex-api mints the
	// ticket but never gains pods/exec.
	ShellTicketSecret []byte
	ShellWSURL        string
	// Store is the Postgres source of truth for store-managed Apps (those carrying
	// both the managed-by and app-id labels). Suspend/Resume write the row first — the row owns
	// spec.suspended, and the projection loop reverts CR patches it didn't
	// originate — then patch the CR as the fast-converge path. Nil (tests, DB-less
	// mode) falls back to CR-only patches, safe only for hand-applied Apps.
	Store IntentStore
	// EventFacts persists closed, non-secret service activity that cannot be
	// represented by an ordinary authorization audit row. nil in store-less mode.
	EventFacts store.EventFactWriter
	// GitHub, when set (the GitHub App + control-plane store are wired), mints a
	// fresh installation token and writes the <app>-clone Secret on every deploy
	// trigger whose repo belongs to the workspace's connection, so private repos
	// clone (docs/ADR026-github-integration.md). nil => public-clone only, unchanged.
	GitHub CloneTokenSource
	// Commits resolves a repo-backed deploy's triggering ref to the exact
	// commit it points at (w9/001) — github.Service's DeployCommitSource, the
	// same seam deploys.Service.Commits wires. nil => the deploy rows this
	// package opens carry no commit metadata.
	Commits CommitResolver
	// StartedNotifier is invoked asynchronously after a signed git push has
	// successfully started the redeploy. nil leaves push-to-deploy unchanged.
	StartedNotifier DeployStartedNotifier
	// RegistryCreds, when set (registrycreds.Service is wired), materializes a
	// dockerconfigjson pull Secret for an image-backed App whose image's
	// registry host matches a workspace-stored credential (w2/m14). nil =>
	// registry credentials are off, unchanged (no external pull secret is
	// ever set).
	RegistryCreds PullSecretSource
	// Kick, when set (the reconciler is wired), schedules an immediate projection
	// pass after a store-managed create/redeploy so intent reaches the cluster in
	// milliseconds instead of on the next resync period. nil => no nudge (store off
	// or tests).
	Kick func()
	// Blueprints, when set (the control-plane store is wired), persists blueprint
	// rows (w2/m15): auto-upserted on every repo-backed deploy, and queried by the
	// list/sync verbs. nil => list/sync return ErrBlueprintsUnavailable; validate
	// is always available (stateless).
	Blueprints BlueprintStore
	// GitFetcher, when set (the GitHub App is wired), fetches the render.yaml manifest
	// from a Git repository so CreateBlueprint and SyncBlueprint can pull the
	// latest file without a local clone (w2/m62 — Git-connected Blueprints). nil
	// => create and sync fall back to the supplied/stored manifest.
	GitFetcher BlueprintFetcher
	// BlueprintGroups resolves/creates Render Blueprint projects and
	// environments by name for projects[].environments[] manifests.
	BlueprintGroups BlueprintGroupingStore
	// BlueprintGroupsTx, when set (*store.PGStore), makes the grouping apply
	// atomic: every grouping row from one sync commits together or rolls back
	// together (w8/m20 t001). nil (test fakes) applies non-transactionally.
	BlueprintGroupsTx BlueprintGroupingTxRunner
	// MaxGroupings caps a workspace's durable project and environment counts
	// at Blueprint-apply time (BEX_MAX_BLUEPRINT_GROUPINGS, default 1000) — an
	// abuse bound, not a plan tier (w1/049 #5). 0 disables.
	MaxGroupings int
	// GroupingReclaim, when set (*store.PGStore), lets DisconnectBlueprint
	// sweep the empty grouping rows the blueprint minted (w8/m20 t004).
	GroupingReclaim GroupingReclaimer
	// SecretsEraser, when set, purges the app's OpenBao env-var and secret-file
	// paths on delete. nil => OpenBao paths are not purged on service delete
	// (they are purged on workspace delete via WorkspacePurger). Satisfied
	// structurally by *secrets.WorkspacePurger so apps never imports secrets.
	SecretsEraser AppSecretsEraser
	// EnvGroups, when set (OpenBao is wired), materializes a render.yaml's
	// envVarGroups: and links them to services via fromGroup (w1/m35), riding the
	// env-groups feature through a narrow seam. nil => a manifest using
	// envVarGroups/fromGroup is rejected before any write (env groups unavailable),
	// never silently dropped.
	EnvGroups EnvGroupApplier
	// EnvSeeder, when set (OpenBao is wired), seeds a render.yaml's sync:false and
	// generateValue vars into the mutable env-vars store SEED-ONCE (w1/m35), so a
	// later dashboard edit wins and a re-sync never overwrites/re-mints. nil => a
	// manifest using those forms is rejected before any write.
	EnvSeeder EnvSeeder
	// EnvNames lists a service's mutable-store env var NAMES for Blueprint
	// generation (w8/m22) — never values. nil ⇒ those vars are omitted from
	// generated manifests.
	EnvNames EnvNameSource
	// EnvGroupExport lists selected env groups' names + env key names for
	// Blueprint generation (w4/040) — never values. nil ⇒ env-group selection
	// and fromGroup linkage are omitted from generated manifests.
	EnvGroupExport EnvGroupExportSource
	// CreateSecrets, when set (OpenBao is wired), persists and materializes what
	// a service is born with: the official CLI's create-time secretFiles payload
	// and the create request's literal env vars (w6/m45). nil makes a create
	// carrying secretFiles fail before the App is written rather than silently
	// discarding it, and leaves literal env vars on spec.Env exactly as before.
	CreateSecrets CreateSecretsSeeder
	// Environments is the shared create-time assignment resolver. It owns the
	// unknown-versus-foreign classification and project lookup for all resource
	// kinds, so service/Postgres/Key Value creates cannot drift.
	Environments core.EnvironmentResolver
	// pushDelivery memoizes the per-repo GitHub-grant answer behind
	// AppView.PushDeliveryMethod (w6/m99) so projecting a whole workspace's
	// services doesn't cost a GitHub round-trip per service. Unexported and
	// built on first use (pushDeliveryMemo): it is a read-path cache, never
	// configuration, so no Service literal has to construct it.
	pushDeliveryOnce   sync.Once
	pushDelivery       *core.TTLCache[string]
	pushDeliveryFlight singleflight.Group
}

// managedAppID distinguishes a projected control-plane App from a direct CR.
// Every App has a stable public app-id, including store-less API creates; only
// the managed-by label proves that the id names a Postgres source row.
func managedAppID(a *appv1alpha1.App) string {
	if a == nil {
		return ""
	}
	return store.ManagedAppID(a.Labels)
}

// AppSecretsEraser clears per-app secrets from the external store on service
// delete. Satisfied structurally by *secrets.WorkspacePurger.
type AppSecretsEraser interface {
	PurgeApp(ctx context.Context, a *appv1alpha1.App) error
}

// CreateSecretsSeeder is the narrow create-time seam onto the secrets feature:
// the secret files and literal env vars a new service is born with, written
// (and referenced from the spec) before the App CR so the first reconcile
// already carries them. *secrets.Service's adapter satisfies it structurally,
// avoiding an apps -> secrets dependency cycle.
type CreateSecretsSeeder interface {
	PrepareCreateSecrets(ctx context.Context, service string, app *appv1alpha1.App, files []core.SecretFile, env map[string]string) error
	CommitCreateSecrets(ctx context.Context, service string, app *appv1alpha1.App) error
	AbortCreateSecrets(ctx context.Context, service string, app *appv1alpha1.App) error
}

// IntentStore is the slice of the source of truth Service writes through — kept
// to the methods the create + lifecycle verbs need, so the service can't grow
// into a second store client and tests fake a minimal set. *store.PGStore satisfies it.
type IntentStore interface {
	// CreateApp writes the app row and opens its first deploy row in one
	// transaction — so every store-managed App has a deploy record from the
	// moment the public-surface create returns (unified create path, w2/m11).
	// ErrConflict if (tenant_id, name) already exists.
	CreateApp(ctx context.Context, a store.App) (store.App, error)
	// CreateDeploy opens a new deploy row (created when idle, queued behind an
	// active release) — called on redeploy of a store-managed App so the deploys
	// API reflects the push.
	// generation is the App CR's metadata.generation this deploy runs under,
	// captured after the redeploy's own patch (w2/m10: Cancel's build-Job
	// identity is derived from this stored value, not a fresh re-fetch, so it
	// must be the generation this deploy actually runs under). commit is the
	// resolved commit this deploy runs (w9/001), zero when unresolvable. The
	// reconciler's write-back closes the row once the CR reaches Running/Failed.
	CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo) (store.Deploy, error)
	// DeleteApp removes the apps row — the single writer of intent for a
	// store-managed App's existence. Delete keeps this durable row until every
	// external, name-keyed secret has been purged, then removes it before the CR.
	// ErrNotFound for unknown ids.
	DeleteApp(ctx context.Context, id string) error
	// Persistent service disks (docs/ADR082-persistent-disks.md). The row is
	// intent — the projector turns it into spec.disk — and simultaneously the
	// billing record the provisioned-capacity meter integrates.
	CreateDisk(ctx context.Context, tenantID, appID, name, mountPath string, sizeGB int32) (store.Disk, error)
	GetDisk(ctx context.Context, id string) (store.Disk, error)
	ListDisks(ctx context.Context, tenantID, appID string) ([]store.Disk, error)
	UpdateDisk(ctx context.Context, id string, name, mountPath *string, sizeGB *int32) (store.Disk, error)
	DeleteDisk(ctx context.Context, id string) error
	SetAppSuspended(ctx context.Context, id string, suspended bool) error
	SetAppTier(ctx context.Context, id string, tier string) error
	SetAppReplicas(ctx context.Context, id string, replicas int32) error
	// SetAppIdleTTL updates the row's idle-TTL — the single write path for the
	// idle-timeout verb on store-managed Apps, same row-first rationale as
	// SetAppReplicas (the projector owns spec.idleTTLSeconds).
	SetAppIdleTTL(ctx context.Context, id string, seconds int32) error
	// SetAppDisplayName mirrors spec.displayName onto the row. The CR stays the
	// writer of truth (the projector doesn't own the field) — this is the read
	// projection the workspace-wide webhook feed joins, so a renamed service's
	// deliveries carry the same label the dashboard shows (w6/m101).
	SetAppDisplayName(ctx context.Context, id string, displayName string) error
	// SetAppSource updates the projector-owned repo/image/branch tuple in one
	// row write so a source PATCH cannot be reverted on the next resync.
	SetAppSource(ctx context.Context, id, repo, image, branch string, registryCredentialID *string) error
	// SetAppImage updates the projector-owned image field. Source redeploys use
	// it to clear the exact-image override a prior rollback installed.
	SetAppImage(ctx context.Context, id string, image string) error
	// AddDomain appends or updates a custom-domain row. redirectForName is empty
	// for a directly-served host and names the canonical host for an auto-paired
	// redirect. The projector carries both into the App spec on the next resync.
	AddDomain(ctx context.Context, appID, host, redirectForName string) error
	// RemoveDomain removes a custom domain row and any platform-generated row
	// that redirects to it. A generated redirect row is identified by its
	// redirectForName; an explicitly re-added sibling has that field cleared and
	// is therefore preserved. Idempotent — not-found is silently ignored.
	RemoveDomain(ctx context.Context, appID, host string) error
	// ReplaceDomains atomically makes the database's globally-unique domain rows
	// match the requested primary + additional hosts. The unique host constraint
	// is the authoritative concurrent-claim boundary used by Blueprint re-sync.
	ReplaceDomains(ctx context.Context, appID, primary string, hosts []string) error
	// GetAppProtectedStatus resolves an App's protectedStatus via its
	// Environment (w6/m19) — "unprotected" when it has none. The read side of
	// the destructive-verb guard, apps/protection.go.
	GetAppProtectedStatus(ctx context.Context, appID string) (string, error)
}

// CronRunView is one execution of a cron_job — the neutral projection of the
// App's status run history the adapters render (Render exposes cron runs at
// /cron-jobs/{id}/runs). Newest first.
type CronRunView struct {
	// ID is a stable crr- id derived from Name through internal/id. Name remains
	// an internal/legacy projection for the nested Service.runs field; the
	// first-class run APIs expose ID and never leak the Kubernetes Job name.
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Status     string `json:"status"` // pending | successful | unsuccessful | canceled
}

// Stable machine-readable cron action failures. REST exposes these in `code`,
// GraphQL in `extensions.code`, and the MCP adapter prefixes them to error text.
const (
	CronErrorSuspended   = "CRON_SUSPENDED"
	CronErrorRunNotFound = "CRON_RUN_NOT_FOUND"
	CronErrorRunTerminal = "CRON_RUN_TERMINAL"
)

// ServiceInstanceView is Render's complete public service-instance shape. It
// deliberately carries no Pod name, namespace, node, IP, phase, labels, or
// container state: Kubernetes is the mechanism behind the two-field contract,
// not part of it.
type ServiceInstanceView struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

// AppView is the neutral, bex-native projection of an App — spec intent +
// observed status. Service returns this; each adapter maps it to its own wire
// format (the REST/GraphQL adapters render it in Render's Service shape).
type AppView struct {
	// ID is the Render-shaped typed service id (srv-<xid>). Store-managed Apps
	// carry it in core.LabelAppID; store-less API creates mint the same label.
	// Legacy hand-applied CRs fall back to Name for backwards compatibility.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Slug is the globally-unique platform-host segment (Render's "slug"
	// field on the service object; bex's spec.subdomain, minted w4/m19) —
	// the bare service name when free platform-wide, or "<name>-<4-char
	// suffix>" when a cross-tenant collision required one. Distinct from
	// Name: Name is workspace-unique (what the tenant called it), Slug is
	// platform-unique (what the public host is built from,
	// AppSpec.PlatformSubdomain) — previously only observable by parsing it
	// out of URL/URLs (w4/m20/t002).
	Slug string `json:"slug"`
	// DisplayName is the App's mutable, human-facing label (spec.displayName).
	// Empty means clients should display Name, preserving existing Apps while
	// keeping Name available as the immutable resource id.
	DisplayName string `json:"displayName"`
	// Type is the Render serviceType (web_service | private_service |
	// background_worker | cron_job); empty spec.type projects as web_service.
	Type  string   `json:"type"`
	Phase string   `json:"phase"`
	URL   string   `json:"url"`
	URLs  []string `json:"urls"`
	// PublicRoutingNotice explains why a service that asked to be publicly
	// reachable has no public address (bex extra, w7/m79). Empty when the
	// service is routed, or is not the kind that carries a public URL.
	//
	// Read from the operator's PublicRouting condition rather than recomputed
	// here on purpose: bex-api and the operator each carry their own
	// BEX_BASE_DOMAIN and can disagree (.pm/w7/026.md), and the truth the user
	// needs is what the component that actually writes Ingresses decided.
	PublicRoutingNotice string `json:"publicRoutingNotice,omitempty"`
	Image               string `json:"image"`
	// SourceImage is the configured prebuilt image. Image above is observed
	// deployment state (status.image), which can be empty until the operator has
	// reconciled; Render's imagePath is configuration and must be available
	// immediately so clients can clone an image-backed service.
	SourceImage string `json:"sourceImage,omitempty"`
	// RegistryCredentialID is the durable explicit private-registry binding.
	// nil means legacy host auto-resolution; pointer-to-empty means explicitly
	// unbound. Adapters expose the string value and treat nil as omitted.
	RegistryCredentialID *string `json:"registryCredentialId,omitempty"`
	// Runtime/build/start mirror Render's native source-build contract.
	Runtime      string `json:"runtime,omitempty"`
	BuildCommand string `json:"buildCommand,omitempty"`
	StartCommand string `json:"startCommand,omitempty"`
	// Builder selects the internal repo build strategy (auto | buildpack |
	// dockerfile | native). Render-facing clients normally use Runtime instead.
	// Empty only occurs on legacy Apps created before the CRD default existed.
	Builder   string `json:"builder,omitempty"`
	Replicas  int32  `json:"replicas"`
	Suspended bool   `json:"suspended"`
	// Schedule is the cron expression for a cron_job (spec.schedule), empty otherwise.
	Schedule string `json:"schedule,omitempty"`
	// Command overrides a cron_job's default entrypoint (spec.command), empty
	// otherwise (the image's own command runs unmodified).
	Command string `json:"command,omitempty"`
	// Runs is a cron_job's recent run history (status.runs), newest first.
	Runs []CronRunView `json:"runs,omitempty"`
	// LastSuccessfulRunAt is Render's cronJobDetails.lastSuccessfulRunAt,
	// derived from the newest successful status.runs entry.
	LastSuccessfulRunAt string `json:"lastSuccessfulRunAt,omitempty"`
	// NextRunAt is the next scheduled fire time of a cron_job (RFC3339 UTC),
	// computed from spec.schedule with the same parser the Kubernetes CronJob
	// controller uses. A bex extension (Render has no next-run field); omitted
	// for a suspended cron, a non-cron service, or an unparseable schedule.
	NextRunAt string `json:"nextRunAt,omitempty"`
	// Plan is Render's public spelling of the App's tier (e.g. "pro_plus" for
	// spec.tier "pro-plus"), sourced from lego/types/tiers. Omitted — not
	// faked as "" — when spec.tier is empty or not a recognized tier, so a
	// Render-shaped client sees a real superset rather than a bogus plan.
	Plan     string `json:"plan,omitempty"`
	Revision string `json:"revision"`
	// SSHAddress is Render's raw OpenSSH target (`srv-…@host`). It is populated
	// only by Service projections because it depends on the configured gateway.
	SSHAddress string `json:"sshAddress,omitempty"`
	// InternalAddress is the private-network address sibling services connect
	// to: "<slug>:<port>", scheme-less — the exact string Render's Connect →
	// Internal tab shows (docs/ADR041-service-addresses.md D4). Addressable
	// types only (web/private); empty otherwise. A documented bex extension:
	// Render's REST has no internal-address field (consumers derive it from
	// slug + port), so surfacing it is additive on the Render-compatible
	// surfaces.
	InternalAddress string `json:"internalAddress,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
	// DashboardURL is the control-plane detail route, in Render's
	// `/{web|worker|pserv|static|cron}/{id}` shape (docs/render-artifacts/
	// dashboard-routes.md). URL above remains the hosted data-plane endpoint;
	// the two are never interchangeable.
	DashboardURL string `json:"dashboardUrl,omitempty"`
	Region       string `json:"region,omitempty"`
	// IdleTTLSeconds is how long a free-tier App may be idle before it
	// auto-hibernates ("sleep = free", spec.idleTTLSeconds). 0 (or unset) means
	// the platform default idle window (15 min); a positive value overrides it.
	// A bex extension with no Render counterpart (Render's spin-down window is
	// fixed) — the dashboard's Settings tab reads/writes it.
	IdleTTLSeconds int32 `json:"idleTTLSeconds"`
	// OwnerID is the workspace (tenant) this App belongs to — Render's `ownerId`
	// scoping field (w6/m2/t004), read from the App CR's core.LabelTenant label
	// (the same label List uses to scope to the caller's own tenant). Omitted
	// for Apps the control-plane projector didn't stamp (the hand-applied path,
	// scripts/app-apply.sh) — an honest superset rather than a faked id.
	OwnerID string `json:"ownerId,omitempty"`
	// ProjectID/EnvironmentID are projected from the control-plane row onto
	// labels so Render REST clients can hydrate and filter service membership.
	ProjectID     string `json:"projectId,omitempty"`
	EnvironmentID string `json:"environmentId,omitempty"`
	// RootDir is the subdirectory of the repo this App builds from (Render's
	// Root Directory setting, for monorepos; spec.rootDir). Empty is the repo root.
	RootDir        string `json:"rootDir,omitempty"`
	DockerfilePath string `json:"dockerfilePath,omitempty"`
	// DockerContext is Render's Docker Build Context Directory
	// (repo-root-relative, independent of RootDir); empty means the RootDir
	// context (w8/m19).
	DockerContext string `json:"dockerContext,omitempty"`
	// Repo and Branch are the build-from-git source (spec.repo/spec.branch),
	// empty for an image-backed App. The dashboard's Settings → Build & Deploy
	// section reads all three; only RootDir is editable after create
	// (SetRootDir) — Repo/Branch are fixed at create time.
	Repo   string `json:"repo,omitempty"`
	Branch string `json:"branch,omitempty"`
	// BuildFilter is Render's Build Filters object (spec.buildFilter): the glob
	// patterns gating git-push auto-deploys. nil when unset. The Settings → Build
	// & Deploy section reads it and writes it via SetBuildFilter.
	BuildFilter *BuildFilterView `json:"buildFilter,omitempty"`
	// Autoscaling is the current per-service autoscaling config (nil when
	// spec.autoscaling is unset, i.e. disabled and unconfigured).
	Autoscaling *AutoscalingView `json:"autoscaling,omitempty"`
	// Disk is the attached persistent disk, nil when the service has none
	// (docs/ADR082-persistent-disks.md). Projected from spec.disk so it reflects
	// what the operator is actually running, not only what was requested; the
	// disk's own id comes from the control-plane row via the /v1/disks surface.
	Disk *ServiceDiskView `json:"disk,omitempty"`
	// AutoDeploy is whether a signed git push to Branch redeploys this App
	// (spec.autoDeploy, Render's Auto-Deploy toggle). The Settings → Build &
	// Deploy section reads it to render the toggle and writes it via SetAutoDeploy.
	AutoDeploy bool `json:"autoDeploy"`
	// PushDeliveryMethod is how a push to Branch can REACH bex for this specific
	// repo — github_app | manual_webhook | none | unknown (see pushdelivery.go).
	// AutoDeploy above is the on/off setting; this is whether the setting has a
	// delivery path at all, which the setting alone cannot express (w6/m99).
	//
	// Answering it costs GitHub round-trips, so only Get computes it — the one
	// verb REST's by-id GET, GraphQL server(id)/service(id) and MCP get_service
	// all route through. It is EMPTY on every other projection (List, and the
	// views mutations and previews return): absent means "not computed on this
	// projection", never "no" — read the service by id for the answer.
	PushDeliveryMethod string `json:"pushDeliveryMethod,omitempty"`
	// NotifyOnFail is Render's per-service deploy-failure notification override
	// (spec.notifyOnFail): default | notify | ignore (docs/render-artifacts/
	// notify-on-fail.md). Empty is reported as "default". The Settings →
	// Notifications section reads it and writes it via SetNotifyOnFail.
	NotifyOnFail string `json:"notifyOnFail"`
	// NotificationsToSend is Render's richer service notification policy:
	// default | none | failure | all.
	NotificationsToSend string `json:"notificationsToSend"`
	// RenderSubdomainPolicy is Render's renderSubdomainPolicy field
	// (enabled|disabled): whether the platform subdomain under BEX_BASE_DOMAIN is
	// active for this service. "enabled" (default) keeps it; "disabled" drops
	// it so only custom domains in spec.hosts[] serve the App. The Settings →
	// Custom Domains section reads it and writes it via SetSubdomainPolicy.
	RenderSubdomainPolicy string `json:"renderSubdomainPolicy"`
	// HealthCheckPath is the HTTP path the health probes GET (spec.healthCheckPath,
	// Render's healthCheckPath). Empty selects the TCP default — the platform
	// checks only that the process is listening. The Settings →
	// Health & Alerts section reads/writes it via SetHealthCheckPath (w5/009).
	HealthCheckPath string `json:"healthCheckPath,omitempty"`
	// MaxShutdownDelaySeconds is the effective SIGTERM grace window for a
	// long-running service. Existing CRs leave the underlying pointer unset, so
	// view() reports Render/Kubernetes' shared 30-second default without mutating
	// their spec. Zero means not applicable (cron_job/static_site).
	MaxShutdownDelaySeconds int32 `json:"maxShutdownDelaySeconds,omitempty"`
	// PreDeployCommand is Render's Pre-Deploy Command (spec.preDeployCommand): a
	// command run to completion against the new revision's image before it serves
	// traffic (typically a DB migration); a non-zero exit fails the deploy. Empty
	// means no pre-deploy step. The Settings → Build & Deploy section reads/writes
	// it via SetPreDeployCommand.
	PreDeployCommand string `json:"preDeployCommand,omitempty"`
	// InitialDeployHook is the blueprint-only one-time first-deploy command (Render's
	// initialDeployHook, w2/m45). Echoed from the bex.co/initial-deploy-hook
	// annotation; empty on services not created from a blueprint or where no hook
	// was declared.
	InitialDeployHook string `json:"initialDeployHook,omitempty"`
	// PublishPath is the built output directory a static_site serves as its
	// document root (spec.publishPath, Render's "Publish Directory"). Empty for
	// every other type.
	PublishPath string `json:"publishPath,omitempty"`
	// Routes are a static_site's ordered redirect/rewrite rules (spec.routes,
	// Render's /routes). Empty for every other type.
	Routes []StaticRouteView `json:"routes,omitempty"`
	// Headers are a static_site's custom response-header rules (spec.headers,
	// Render's /headers). Empty for every other type.
	Headers []StaticHeaderView `json:"headers,omitempty"`
	// IPAllowList is Render's inbound IP allowlist for web_service and
	// static_site. Both CIDR and description persist; enforcement projects only
	// CIDRs. Empty means open to all source IPs.
	IPAllowList []core.IPAllowListEntry `json:"ipAllowList,omitempty"`
	// MaintenanceMode is Render's maintenanceMode object (spec.maintenanceMode):
	// {enabled, uri}. web_service only — every other type reports the zero
	// value. The Settings → Maintenance Mode section reads it and writes it via
	// SetMaintenanceMode. See docs/render-artifacts/maintenance-mode.md.
	MaintenanceMode MaintenanceModeView `json:"maintenanceMode"`
	// LatestDeployID is the id of the first deploy row, populated by Create only
	// (w3/m14). The dashboard uses it to navigate to the in-flight deploy page
	// immediately after a git-sourced service is created. Empty on Get/List.
	LatestDeployID string `json:"latestDeployId,omitempty"`
}

// StaticRouteView is the neutral projection of one static_site redirect/rewrite
// rule — Render's route shape (type/source/destination).
type StaticRouteView struct {
	Type        string `json:"type"` // redirect | rewrite
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// StaticHeaderView is the neutral projection of one static_site custom response
// header — Render's header shape (path/name/value).
type StaticHeaderView struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// staticRouteViews / staticHeaderViews project the CR spec lists onto the neutral
// view shapes; the fromView helpers convert the surface input back to CR types.
func staticRouteViews(routes []appv1alpha1.StaticRoute) []StaticRouteView {
	if len(routes) == 0 {
		return nil
	}
	out := make([]StaticRouteView, len(routes))
	for i, r := range routes {
		out[i] = StaticRouteView{Type: r.Type, Source: r.Source, Destination: r.Destination}
	}
	return out
}

func staticHeaderViews(headers []appv1alpha1.StaticHeader) []StaticHeaderView {
	if len(headers) == 0 {
		return nil
	}
	out := make([]StaticHeaderView, len(headers))
	for i, h := range headers {
		out[i] = StaticHeaderView{Path: h.Path, Name: h.Name, Value: h.Value}
	}
	return out
}

// MaintenanceModeView is the neutral projection of Render's maintenanceMode
// object (spec.maintenanceMode): {enabled, uri}, byte-identical across every
// surface. Unlike BuildFilterView, a value type (never nil) — nil/unset
// spec.maintenanceMode reads back as the zero value {enabled:false, uri:""},
// matching docs/render-artifacts/maintenance-mode.md ("nil/unset is the same
// as {enabled: false, uri: \"\"}"), so a read never has to distinguish "never
// configured" from "explicitly disabled" the way BuildFilter does.
type MaintenanceModeView struct {
	Enabled bool   `json:"enabled"`
	URI     string `json:"uri"`
}

// maintenanceModeView projects the CR spec onto the neutral view; nil => the
// zero value.
func maintenanceModeView(m *appv1alpha1.MaintenanceModeSpec) MaintenanceModeView {
	if m == nil {
		return MaintenanceModeView{}
	}
	return MaintenanceModeView{Enabled: m.Enabled, URI: m.URI}
}

// normalizeMaintenanceMode validates Render's maintenanceMode input and
// projects it onto the CRD spec type. nil input means unset (create didn't
// mention it) => nil spec, the same as never having enabled it. A non-empty
// uri must be an absolute http(s) URL (docs/render-artifacts/
// maintenance-mode.md). Service-level validation additionally rejects a URI
// that points back at any host owned by the same service, avoiding a fetch
// loop before the change reaches the CR.
func normalizeMaintenanceMode(in *MaintenanceModeView) (*appv1alpha1.MaintenanceModeSpec, error) {
	if in == nil {
		return nil, nil
	}
	return validatedMaintenanceModeSpec(in.Enabled, in.URI)
}

func validatedMaintenanceModeSpec(enabled bool, rawURI string) (*appv1alpha1.MaintenanceModeSpec, error) {
	uri := strings.TrimSpace(rawURI)
	if uri != "" && !core.ValidAbsoluteHTTPURL(uri) {
		return nil, core.ErrNotAbsoluteHTTPURL("maintenanceMode.uri")
	}
	return &appv1alpha1.MaintenanceModeSpec{Enabled: enabled, URI: uri}, nil
}

func validateMaintenanceEligibility(serviceType, tier string, mode *MaintenanceModeView) error {
	if mode == nil {
		return nil
	}
	if serviceType != appv1alpha1.TypeWebService {
		return fmt.Errorf("%w: maintenanceMode is available only for web services", core.ErrBadRequest)
	}
	if tier == "" || tier == "free" {
		return fmt.Errorf("%w: maintenanceMode requires a paid web service plan", core.ErrBadRequest)
	}
	return nil
}

func (s *Service) validateMaintenanceMode(ctx context.Context, a *appv1alpha1.App, mode MaintenanceModeView) error {
	if err := validateMaintenanceEligibility(effectiveType(a.Spec.Type), a.Spec.Tier, &mode); err != nil {
		return err
	}
	u, err := parseMaintenanceURI(mode.URI)
	if err != nil || u == nil {
		return err
	}
	host := canonicalHost(u.Hostname())
	for owned := range s.maintenanceHosts(a) {
		if host == owned {
			return fmt.Errorf("%w: maintenanceMode.uri cannot point to the same service", core.ErrBadRequest)
		}
	}
	// Cross-service loop guard: every maintenance-page fetch is a synchronous
	// public request through the shared activator, so a URI pointing at ANY
	// platform-routed host (not just this service's own) can close an
	// amplifying fetch cycle between two services. Reject platform-reserved
	// hosts (`*.<base>`, dashboard — reservedHost) and any custom host claimed
	// by another App cluster-wide (the same sweep AddDomain enforces).
	if s.reservedHost(s.ownPlatformHost(a), host) {
		return fmt.Errorf("%w: maintenanceMode.uri cannot point to a platform hostname", core.ErrBadRequest)
	}
	if claimed, err := s.hostClaimedElsewhere(ctx, a, host); err != nil {
		return err
	} else if claimed {
		return fmt.Errorf("%w: maintenanceMode.uri cannot point to another service on this platform", core.ErrBadRequest)
	}
	return nil
}

// validateNewSpecMaintenanceMode runs validateMaintenanceMode over a NEW
// service's desired spec — before any App exists — via a name-only probe
// object. No-ops when the spec carries no maintenanceMode; shared by create
// and the Blueprint validate/apply paths so the probe shape can't drift.
func (s *Service) validateNewSpecMaintenanceMode(ctx context.Context, name string, desired appv1alpha1.AppSpec) error {
	if desired.MaintenanceMode == nil {
		return nil
	}
	probe := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: desired}
	return s.validateMaintenanceMode(ctx, probe, maintenanceModeView(desired.MaintenanceMode))
}

func parseMaintenanceURI(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("%w: maintenanceMode.uri must be an absolute HTTP(S) URL", core.ErrBadRequest)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: maintenanceMode.uri must not contain credentials", core.ErrBadRequest)
	}
	return u, nil
}

func (s *Service) maintenanceHosts(a *appv1alpha1.App) map[string]struct{} {
	hosts := make(map[string]struct{})
	add := func(value string) {
		if u, err := url.Parse(value); err == nil && u.Hostname() != "" {
			hosts[canonicalHost(u.Hostname())] = struct{}{}
			return
		}
		if host := canonicalHost(value); host != "" {
			hosts[host] = struct{}{}
		}
	}
	add(a.Spec.Host)
	for _, host := range a.Spec.Hosts {
		add(host)
	}
	add(a.Status.URL)
	for _, value := range a.Status.URLs {
		add(value)
	}
	if s.BaseDomain != "" {
		add(a.Spec.PlatformSubdomain(a.Name) + "." + s.BaseDomain)
	}
	return hosts
}

func canonicalHost(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	return strings.ToLower(strings.TrimSuffix(value, "."))
}

// BuildFilterView is the neutral projection of Render's Build Filters object
// (spec.buildFilter): the repository-root-relative glob patterns that gate
// git-push auto-deploys. Its JSON shape ({paths, ignoredPaths}) is byte-identical
// across every surface (Render create/PATCH body, service response, render.yaml
// blueprint), so the one type serves as create input, PATCH input, read-back
// output, and blueprint decode — no per-surface duplicate. Both arrays are
// present (never null) when the object is, matching Render's required fields.
type BuildFilterView struct {
	Paths        []string `json:"paths"`
	IgnoredPaths []string `json:"ignoredPaths"`
}

// buildFilterView projects the CR spec onto the neutral view. A nil or all-empty
// filter projects as nil (the canonical "unset" — every matching push deploys),
// so a read-back never shows an empty-but-present object. The slices are copied
// (and forced non-nil when present) so the response marshals arrays, not null.
func buildFilterView(bf *appv1alpha1.BuildFilterSpec) *BuildFilterView {
	if bf == nil || (len(bf.Paths) == 0 && len(bf.IgnoredPaths) == 0) {
		return nil
	}
	return &BuildFilterView{
		Paths:        append([]string{}, bf.Paths...),
		IgnoredPaths: append([]string{}, bf.IgnoredPaths...),
	}
}

// normalizeBuildFilter validates and canonicalizes Render's Build Filters input:
// each glob must pass store.ValidGlob (a compilable, repo-root-relative pattern,
// ≤100 per list); empty/whitespace entries are dropped. When both lists end up
// empty the whole filter is nil (the canonical "unset"), so setting an all-empty
// filter clears it. Shared by create (specFromCreate) and SetBuildFilter so the
// two entry points enforce one rule.
func normalizeBuildFilter(in *BuildFilterView) (*appv1alpha1.BuildFilterSpec, error) {
	if in == nil {
		return nil, nil
	}
	paths, err := cleanGlobs("paths", in.Paths)
	if err != nil {
		return nil, err
	}
	ignored, err := cleanGlobs("ignoredPaths", in.IgnoredPaths)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 && len(ignored) == 0 {
		return nil, nil
	}
	return &appv1alpha1.BuildFilterSpec{Paths: paths, IgnoredPaths: ignored}, nil
}

// cleanGlobs trims and validates one buildFilter list, dropping empty entries and
// rejecting a malformed or over-long glob at the API boundary (so the webhook
// matcher never sees a pattern it can't compile).
func cleanGlobs(field string, globs []string) ([]string, error) {
	var out []string
	for _, g := range globs {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if !store.ValidGlob(g) {
			return nil, fmt.Errorf("%w: buildFilter.%s has an invalid glob pattern %q", core.ErrBadRequest, field, g)
		}
		out = append(out, g)
	}
	if len(out) > 100 {
		return nil, fmt.Errorf("%w: buildFilter.%s has too many patterns (max 100)", core.ErrBadRequest, field)
	}
	return out, nil
}

// effectiveType resolves spec.type to Render's serviceType, defaulting an
// empty value to web_service — the one place that default is decided, so
// view()/toRenderService() can't drift on it.
func effectiveType(specType string) string {
	if specType == "" {
		return appv1alpha1.TypeWebService
	}
	return specType
}

// effectiveRuntime resolves the Render-facing source runtime a read surface
// reports (serviceDetails.runtime/env, the runtime-keyed envSpecificDetails, and
// the GraphQL/MCP runtime field). Every Render service HAS a runtime, but bex's
// App CR leaves spec.runtime empty for the common Dockerfile build it expresses
// through spec.builder instead — a hand-applied App, a Blueprint, or a dashboard
// create that never set Render's `runtime` alias. The official CLI reads
// serviceDetails.runtime to round-trip a partial `services update`: with none
// returned it rejects the empty value client-side ("unsupported runtime") or
// treats an explicit --runtime as a forbidden switch ("cannot switch runtimes
// via the CLI"), so a Dockerfile web service could not repoint its own
// healthCheckPath (w4/052). Deriving it here — the one place the read runtime is
// decided — keeps that string consistent across every surface.
//
// The mapping mirrors the operator's effectiveBuilder in reverse and names only
// a runtime bex can determine unambiguously: an explicit spec.runtime wins; a
// prebuilt image (image set, no repo build) is "image"; and a repo build under
// the default/auto/dockerfile builder is "docker" (all three resolve to a
// Dockerfile build). A static site has no App runtime (bex's runtime enum has no
// "static"), and a bare "buildpack"/"native" builder without an explicit runtime
// names no runtime bex can pin here, so each reads back empty as it did before.
func effectiveRuntime(spec appv1alpha1.AppSpec, svcType string) string {
	if spec.Runtime != "" {
		return spec.Runtime
	}
	if svcType == appv1alpha1.TypeStaticSite {
		return ""
	}
	if spec.Image != "" && spec.Repo == "" {
		return "image"
	}
	if spec.Repo != "" {
		switch spec.Builder {
		case "", "auto", "dockerfile":
			return "docker"
		}
	}
	return ""
}

func view(a *appv1alpha1.App) AppView {
	name := publicName(a)
	appID := publicID(a)
	created := ""
	if !a.CreationTimestamp.IsZero() {
		created = a.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	plan := ""
	if t, ok := tiers.Compute.ByID(a.Spec.Tier); ok {
		plan = t.RenderPlan
	}
	svcType := effectiveType(a.Spec.Type)
	var asView *AutoscalingView
	if a.Spec.Autoscaling != nil {
		v := autoscalingView(a)
		asView = &v
	}
	policy := a.Spec.EffectiveNotificationsToSend()
	notifyOnFail := appv1alpha1.NotifyOnFailForNotificationsToSend(policy)
	// normalizeSubdomainPolicy's error is unreachable: the CRD enum guarantees
	// a.Spec.SubdomainPolicy is "", "enabled", or "disabled".
	subdomainPolicy, _ := normalizeSubdomainPolicy(a.Spec.SubdomainPolicy)
	// renderSubdomainPolicy describes whether the platform subdomain serves this
	// service — a property only a web service or static site HAS. Render declares
	// it solely on webServiceDetails/staticSiteDetails; a private service, worker
	// or cron job has no ingress, so the field is not applicable and reads back
	// empty (omitted on REST, null on GraphQL), never "enabled" while the same
	// payload has no url and the host 404s at the edge (w6/m130). Emptying the
	// VALUE here is what lets GraphQL's flat field agree with REST/MCP's omission
	// — GraphQL reads this AppView, not renderServiceDetails (which gates again at
	// the emission site, belt-and-suspenders like ipAllowList).
	if !appv1alpha1.TypePubliclyRoutable(svcType) {
		subdomainPolicy = ""
	}
	phase := string(a.Status.Phase)
	url := a.Status.URL
	urls := a.Status.URLs
	if !a.DeletionTimestamp.IsZero() {
		// A deleting App's route and certificate are withdrawn by the ownerRef
		// cascade within seconds, so its serving URL is dead. Never project it.
		// By-id verbs already 404 in this state (core.NotFoundIfDeleting), so this
		// is defense-in-depth that keeps the shared projection itself honest — a
		// deleting App, if ever projected, reads Deleting with no URL, never the
		// stale Running/dead-URL pair the w3/m81 fixture served for 2+ hours.
		phase = "Deleting"
		url = ""
		urls = nil
	}
	return AppView{
		ID:                  appID,
		Name:                name,
		Slug:                a.Spec.PlatformSubdomain(a.Name),
		DisplayName:         a.Spec.DisplayName,
		Type:                svcType,
		Phase:               phase,
		URL:                 url,
		PublicRoutingNotice: publicRoutingNotice(a),
		// The contract-level derivation (types/v1alpha1) the operator's slug
		// Service answers — surfaced string and resolvable hostname cannot
		// drift (ADR041 D2/D4).
		InternalAddress:      a.Spec.InternalAddress(a.Name),
		URLs:                 urls,
		Image:                a.Status.Image,
		SourceImage:          a.Spec.Image,
		RegistryCredentialID: clonePtr(a.Spec.RegistryCredentialID),
		Runtime:              effectiveRuntime(a.Spec, svcType),
		BuildCommand:         a.Spec.BuildCommand,
		StartCommand:         a.Spec.StartCommand,
		Builder:              a.Spec.Builder,
		Replicas:             a.Spec.Replicas,
		Suspended:            a.Spec.Suspended,
		Schedule:             a.Spec.Schedule,
		Command:              a.Spec.Command,
		Runs:                 cronRunViews(a.Status.Runs),
		LastSuccessfulRunAt:  lastSuccessfulCronRunAt(a.Status.Runs),
		Plan:                 plan,
		Revision:             a.Status.ActiveRevision,
		CreatedAt:            created,
		UpdatedAt:            resourcemeta.UpdatedAt(a),
		IdleTTLSeconds:       a.Spec.IdleTTLSeconds,
		OwnerID:              a.Labels[core.LabelTenant],
		ProjectID:            a.Labels[core.LabelProject],
		EnvironmentID:        a.Labels[core.LabelEnvironment],
		RootDir:              a.Spec.RootDir,
		DockerfilePath:       a.Spec.DockerfilePath,
		DockerContext:        a.Spec.DockerContext,
		// Redact any legacy embedded userinfo before the URL reaches a viewer
		// (round-6 #13); create/update now reject such URLs outright.
		Repo:                  store.RedactRepoURL(a.Spec.Repo),
		Branch:                a.Spec.Branch,
		BuildFilter:           buildFilterView(a.Spec.BuildFilter),
		Autoscaling:           asView,
		Disk:                  serviceDiskView(a.Spec.Disk),
		AutoDeploy:            a.Spec.AutoDeploy,
		NotifyOnFail:          notifyOnFail,
		NotificationsToSend:   policy,
		RenderSubdomainPolicy: subdomainPolicy,
		HealthCheckPath:       a.Spec.HealthCheckPath,
		MaxShutdownDelaySeconds: effectiveMaxShutdownDelaySeconds(
			svcType, a.Spec.MaxShutdownDelaySeconds,
		),
		PreDeployCommand:  a.Spec.PreDeployCommand,
		InitialDeployHook: a.Annotations[initialDeployHookAnnotation],
		PublishPath:       a.Spec.PublishPath,
		Routes:            staticRouteViews(a.Spec.Routes),
		Headers:           staticHeaderViews(a.Spec.Headers),
		IPAllowList:       core.AllowListFromSpec(a.Spec.EffectiveIPAllowListEntries()),
		MaintenanceMode:   maintenanceModeView(a.Spec.MaintenanceMode),
	}
}

// suspenders projects spec.suspended into Render's suspenders array. The
// user-driven suspend verb is the only way an App becomes suspended in bex,
// so a suspended App always reports exactly ["user"]; enum values bex has no
// source for (admin, billing, parent_service, …) are omitted, never faked.
//
// Free-tier idle auto-hibernation is NOT suspension and never reaches here: it
// scales the Deployment to 0 and observes phase Hibernated without touching
// spec.suspended, so an auto-slept service correctly reports no suspenders.
// The event vocabulary keeps the same split — service_hibernated/service_woken
// for the idle cycle, service_suspended/service_resumed for this verb (w6/m47).
func suspenders(suspended bool) []string {
	if suspended {
		return []string{"user"}
	}
	return []string{}
}

func (s *Service) view(a *appv1alpha1.App) AppView {
	v := view(a)
	v.NextRunAt = s.nextCronRunAt(a)
	// Don't synthesize the pending intent URL for a deleting App — its host is
	// being torn down, so the derived URL would be as dead as the withdrawn
	// status.URL that view() already blanks (w3/m81).
	if v.URL == "" && a.DeletionTimestamp.IsZero() {
		v.URL = s.pendingPublicURL(a)
	}
	v.DashboardURL = s.Metadata.DashboardURL(resourcemeta.ServiceDashboardRoute(v.Type), v.ID)
	v.Region = s.Metadata.PlatformRegion()
	host := strings.ToLower(strings.TrimSpace(s.SSHHost))
	if strings.Contains(host, ".") && len(validation.IsDNS1123Subdomain(host)) == 0 && sshEligible(v) {
		if kind, ok := ids.KindOf(v.ID); ok && kind == ids.Service {
			v.SSHAddress = v.ID + "@" + host
		}
	}
	return v
}

// pendingPublicURL derives the URL the operator will serve a public service at
// before the first successful deploy has materialized status.url — Render shows
// a service's URL from the moment it's created (w5/m48/t003), so the API's url
// field must not wait for a publish. The layer decision is deliberate: the CR's
// status stays the operator's observed truth (what is actually served), while
// the API projects the deterministic intent — the same host the operator's
// status derivation will pick, via the one precedence rule on the CRD contract
// (types.AppSpec.EffectiveHosts). Only web_service and static_site carry a
// public URL. Returns "" when no public host is derivable (e.g. the platform
// subdomain is disabled and no custom host is set, or BEX_BASE_DOMAIN is unset).
func (s *Service) pendingPublicURL(a *appv1alpha1.App) string {
	if !a.Spec.PubliclyRoutable() {
		return ""
	}
	if hosts := a.Spec.EffectiveHosts(a.Name, s.BaseDomain); len(hosts) > 0 {
		return "https://" + hosts[0]
	}
	return ""
}

// publicRoutingNotice surfaces the operator's PublicRouting condition when it
// reports that an exposed service has no public address. The operator sets it
// only for services meant to carry a public URL and never for a state the owner
// chose (a worker, a private service, or renderSubdomainPolicy: disabled), so
// this needs no second policy — reproducing that judgement here would be a
// place for the two to drift.
func publicRoutingNotice(a *appv1alpha1.App) string {
	c := meta.FindStatusCondition(a.Status.Conditions, appv1alpha1.ConditionPublicRouting)
	if c == nil || c.Status != metav1.ConditionFalse {
		return ""
	}
	return c.Message
}

func publicID(a *appv1alpha1.App) string {
	if appID := a.Labels[core.LabelAppID]; appID != "" {
		return appID
	}
	return publicName(a)
}

func sshEligible(v AppView) bool {
	// Render only offers SSH on paid services. Treat a missing/unknown plan as
	// ineligible instead of accidentally turning an unclassified service into a
	// paid one.
	plan, knownPlan := tiers.Compute.ByRenderPlan(v.Plan)
	if v.Suspended || !knownPlan || plan.ID == "free" {
		return false
	}
	switch v.Type {
	case appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker:
		return true
	default:
		return false
	}
}

// sshSessionReady is the shared runtime gate both pods/exec transports
// (ResolveSSHSession's native SSH and CreateShellSession's browser shell)
// apply on top of sshEligible: the service must be Running with a live
// revision and image before a session can target one of its pods.
func sshSessionReady(v AppView) error {
	if !sshEligible(v) || v.Phase != string(appv1alpha1.PhaseRunning) || v.Revision == "" || v.Image == "" {
		return fmt.Errorf("%w: service is not eligible and running", core.ErrConflict)
	}
	return nil
}

// cronRunViews projects the CR's status run history onto the neutral view shape.
func cronRunViews(runs []appv1alpha1.CronRun) []CronRunView {
	if len(runs) == 0 {
		return nil
	}
	out := make([]CronRunView, len(runs))
	for i, r := range runs {
		out[i] = cronRunView(r)
	}
	return out
}

func cronRunView(r appv1alpha1.CronRun) CronRunView {
	return CronRunView{
		ID:         ids.Derive(ids.CronRun, r.Name),
		Name:       r.Name,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Status:     renderCronRunStatus(r.Status),
	}
}

// The Render-facing cron-run status vocabulary (CronRunView.Status), produced
// only by renderCronRunStatus and the cancel/trigger verbs.
const (
	cronRunPending      = "pending"
	cronRunSuccessful   = "successful"
	cronRunUnsuccessful = "unsuccessful"
	cronRunCanceled     = "canceled"
)

func renderCronRunStatus(status string) string {
	switch strings.ToLower(status) {
	case "succeeded", cronRunSuccessful:
		return cronRunSuccessful
	case "failed", cronRunUnsuccessful:
		return cronRunUnsuccessful
	case cronRunCanceled, "cancelled":
		return cronRunCanceled
	default:
		return cronRunPending
	}
}

func lastSuccessfulCronRunAt(runs []appv1alpha1.CronRun) string {
	for _, run := range runs {
		if renderCronRunStatus(run.Status) == cronRunSuccessful && run.FinishedAt != "" {
			return run.FinishedAt
		}
	}
	return ""
}

// nextCronRunAt computes a cron_job's next scheduled fire time (RFC3339 UTC)
// from spec.schedule, using the same parser the Kubernetes CronJob controller
// evaluates the schedule with (cron.ParseStandard) so the projection matches
// what the cluster will actually do. A bex extension — Render exposes no
// next-run field. Empty for a non-cron service, a suspended cron (the CronJob
// is paused, so there is no next run), an empty schedule, or one that does not
// parse (which the create/update validation already refuses, but a legacy or
// hand-edited App could still carry).
func (s *Service) nextCronRunAt(a *appv1alpha1.App) string {
	if effectiveType(a.Spec.Type) != appv1alpha1.TypeCronJob || a.Spec.Suspended {
		return ""
	}
	sched, err := cron.ParseStandard(strings.TrimSpace(a.Spec.Schedule))
	if err != nil {
		return ""
	}
	return sched.Next(s.Now().UTC()).UTC().Format(time.RFC3339)
}

// List returns the caller's Apps, optionally narrowed to a single owning
// workspace via ownerID — Render's `ownerId` list-filter contract (w6/m2/t004).
// ownerID == "" is the caller's own resolved tenant: with the store on, only
// its projected Apps (labeled core.LabelTenant=<id>) are visible, and a caller
// with no resolvable tenant sees an empty list rather than an unfiltered one;
// store off lists every App in the namespace, as before. A non-empty ownerID
// names the workspace to list (core.WithWorkspace), authorized+membership-
// checked by the same resolveWorkspace mechanism every other verb uses
// (w6/m17 — previously an OpenFGA-only AuthorizeOn check with no IsMember,
// weaker than the resource-scoped gates) — so a caller who belongs to more
// than one workspace can pick one (an ownerId the caller can't access, or
// isn't a MEMBER of, is ErrForbidden), the same override Render's real API
// supports for a multi-workspace key.
func (s *Service) List(ctx context.Context, ownerID string) ([]AppView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	// Resolve the workspace this list targets — the one named by ownerID or, when
	// unnamed, the caller's own — BEFORE picking the namespace: with per-tenant
	// namespaces (ADR043) each workspace's Apps live in AppNamespace(tenantID), so
	// the list must be scoped there, not the shared s.Namespace.
	tenantID := ownerID
	if tenantID == "" && s.Workspace != nil {
		t, ok := s.Tenant(ctx)
		if !ok {
			return []AppView{}, nil
		}
		tenantID = t
	}
	opts := []client.ListOption{client.InNamespace(s.AppNamespace(tenantID))}
	if tenantID != "" {
		opts = append(opts, client.MatchingLabels{core.LabelTenant: tenantID})
	}
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, opts...); err != nil {
		return nil, err
	}
	out := make([]AppView, 0, len(list.Items))
	for i := range list.Items {
		// A deleting App is dropped from the list the moment its deletion is
		// requested (w3/m46), matching Render (a deleted service leaves the list
		// at once) and the by-id Get, which already 404s once the store row is
		// gone. Without this, an App lingers in the list through its finalizer
		// teardown (a static site's S3-prefix cleanup Job can run for tens of
		// seconds) rendered as "Deleting" — which the dashboard shows as the
		// meaningless "Unknown" status. Trade-off: a delete stuck on a failing
		// finalizer becomes invisible HERE, but not unaccounted for — an
		// object-count ResourceQuota keeps holding its quota until finalizers
		// clear, so the usage surface (workspaces.ResourceLimits / GraphQL
		// workspaceLimits) reports it under `terminating` and the tenant can
		// reconcile the shorter list against the quota it consumes (w6/m129);
		// the operator's own alerts/audit still surface a genuinely stuck one.
		// The detail view is already 404 in that state, so hiding the list row
		// keeps the two consistent.
		if !list.Items[i].DeletionTimestamp.IsZero() {
			continue
		}
		out = append(out, s.view(&list.Items[i]))
	}
	return out, nil
}

// InstanceType is the display-shaped projection of one lego/types/tiers
// compute tier — the bex extension backing the dashboard's plan picker.
// Render's own dashboard hardcodes its instance-type list (no public REST/MCP
// equivalent exists to mirror byte-for-byte), so this is new surface, not a
// captured-live shape; ID is Render's plan spelling (what SetPlan accepts),
// matching the picker's other fields.
type InstanceType struct {
	ID         string
	Name       string
	CPU        string
	Memory     string
	MonthlyUSD string
}

// InstanceTypes lists every tier in the shared compute catalog, in ladder
// order — never a hardcoded copy (the ladder already existed in three such
// copies before w1/m8 collapsed them; the dashboard must not become a fourth).
func (s *Service) InstanceTypes(ctx context.Context) ([]InstanceType, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	ids := tiers.Compute.IDs()
	out := make([]InstanceType, len(ids))
	for i, id := range ids {
		t, _ := tiers.Compute.ByID(id)
		monthlyUSD, _ := pricing.Default.InstanceMonthlyUSD(id, store.ResourceKindService)
		out[i] = InstanceType{
			ID: t.RenderPlan, Name: tierDisplayName(id), CPU: t.CPU,
			Memory: t.Memory, MonthlyUSD: monthlyUSD,
		}
	}
	return out, nil
}

// tierDisplayName turns a hyphenated tier id into Render's display spelling,
// e.g. "pro-plus" -> "Pro Plus" (matches the names captured live from
// Render's plan picker: Free, Starter, Standard, Pro, Pro Plus, Pro Max, Pro Ultra).
func tierDisplayName(id string) string {
	words := strings.Split(id, "-")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// Get returns one App, or core.ErrNotFound. With the store on, a cross-tenant
// Get is core.ErrForbidden (the App exists, the caller just doesn't own it) —
// not ErrNotFound, matching the existing error convention — enforced by the
// shared GetApp, not a re-check here.
func (s *Service) Get(ctx context.Context, name string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return AppView{}, err
	}
	// A deleting App is absent from every by-id surface, matching List and
	// Render's GET 404 (w3/m81): the moment a DeletionTimestamp is set the detail
	// read must not keep serving `phase: Deleting` plus a now-withdrawn URL while
	// the finalizer tears down the resource. The `terminating` quota counter
	// (workspaceLimits, w6/m129) is the aggregate signal that cleanup is still in
	// flight; a stuck finalizer surfaces to operators as a DeletionStalled App
	// condition (docs/ADR029 § Deletion and finalizer bound).
	if err := core.NotFoundIfDeleting(a); err != nil {
		return AppView{}, err
	}
	v := s.view(a)
	// Push deliverability is computed HERE and nowhere else (w6/m99): this one
	// verb backs REST GET /v1/services/{id}, GraphQL server(id)/service(id), and
	// MCP get_service, so all three agree — while List and the projections
	// mutations and previews return, which the field is no part of the contract
	// for, keep paying nothing for a lookup that costs GitHub round-trips.
	v.PushDeliveryMethod = s.pushDeliveryMethod(ctx, a)
	return v, nil
}

// ListInstances projects a long-running App's live replica Pods into Render's
// service-instance contract. Authorization and App resolution happen before
// the Pod list, against the App's own workspace through AuthorizeApp.
//
// Inclusion policy is explicit:
//   - Pending, Running, and Unknown Pods are included, so both active sides of
//     a Deployment rollout remain observable while Kubernetes converges.
//   - terminating Pods and terminal Succeeded/Failed Pods are excluded.
//   - cron_job and static_site Apps return no service instances; their Job Pods
//     or shared static server are not replicas of the service.
//   - suspended and observed-Hibernated Apps return [] immediately, even during
//     the short interval before Kubernetes finishes deleting old Pods. The
//     latter includes auto-sleep's scale-to-zero state.
func (s *Service) ListInstances(ctx context.Context, name string) ([]ServiceInstanceView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}

	out := make([]ServiceInstanceView, 0)
	serviceType := effectiveType(a.Spec.Type)
	if a.Spec.Suspended || a.Status.Phase == appv1alpha1.PhaseHibernated {
		return out, nil
	}
	switch serviceType {
	case appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker:
		// These are the Deployment-backed service types.
	default:
		return out, nil
	}

	pods, err := s.AppPodsIn(ctx, a.Namespace, a.Name)
	if err != nil {
		return nil, err
	}
	serviceID := publicID(a)
	for i := range pods {
		pod := &pods[i]
		if pod.DeletionTimestamp != nil || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		out = append(out, ServiceInstanceView{
			ID:        ids.ServiceInstanceID(serviceID, pod.Name),
			CreatedAt: pod.CreationTimestamp.Time,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// SSHInstanceTarget is one Ready pod selected for an interactive session.
// PodName remains internal; ID is the Render-compatible public instance id.
type SSHInstanceTarget struct {
	ID        string
	CreatedAt string
	ServiceID string
	OwnerID   string
	Namespace string
	PodName   string
	Container string
	// podUID is retained only so ResolveSSHSession can accept pre-m87
	// UID-derived selectors; it is never serialized on the wire.
	podUID string
	// Sandbox marks an agent-session sandbox target (ADR054): the transport then
	// permits Zed's multi-channel remoting and the sftp-server subsystem bridge,
	// which stay banned for ordinary App-instance (srv-…) targets. Resolved
	// server-side (never from caller input) by the agent-session SSH resolver.
	Sandbox bool `json:"Sandbox,omitempty"`
}

// ResolveSSHSession performs resource-scoped authorization and resolves a raw
// SSH username to a Ready instance. A bare service id picks a random Ready
// replica; a compound id returned by ListInstances targets that exact replica.
func (s *Service) ResolveSSHSession(ctx context.Context, username string) (SSHInstanceTarget, error) {
	serviceID, instanceID, parseErr := parseSSHUsername(username)
	lookup := serviceID
	if parseErr != nil {
		// AuthorizeApp remains the first policy boundary even for a malformed
		// username, preserving the repository-wide 403-before-400/404 rule.
		lookup = strings.TrimSpace(username)
	}
	// SECURITY (codex round-4 #8): native SSH and the Browser Web Shell reach the
	// SAME pods/exec sink, so they must enforce the SAME relation. CreateShellSession
	// below requires can_view_sensitive because a shell is `printenv` on the pod's
	// env vars and mounted secrets; gating this transport on the weaker can_operate
	// let a contributor — who holds can_operate AND can_manage_ssh_keys — enroll a
	// key and walk around that boundary with `ssh`. The relation belongs to the sink,
	// not the transport.
	a, err := s.AuthorizeApp(ctx, core.RelCanViewSensitive, lookup)
	if err != nil {
		return SSHInstanceTarget{}, err
	}
	// codex round-7 F7: this resolver runs on the separately deployed gateway
	// and converts a verified credential (SSH key or browser ticket) into a NEW
	// hours-long pods/exec session, so the relation is re-asserted UNCACHED —
	// a member revoked on another replica within PositiveTTL must not open one
	// last shell off this gateway's stale cached positive (the agent-attach
	// revalidator's pattern, ADR057 #11).
	if err := s.AuthorizeAppFresh(ctx, core.RelCanViewSensitive, a); err != nil {
		return SSHInstanceTarget{}, err
	}
	if parseErr != nil {
		return SSHInstanceTarget{}, parseErr
	}
	v := s.view(a)
	if err := sshSessionReady(v); err != nil {
		return SSHInstanceTarget{}, err
	}
	targets, err := s.readySSHInstances(ctx, a, v)
	if err != nil {
		return SSHInstanceTarget{}, err
	}
	if instanceID == "" {
		if len(targets) == 0 {
			return SSHInstanceTarget{}, fmt.Errorf("%w: service has no Ready instance", core.ErrConflict)
		}
		return targets[rand.IntN(len(targets))], nil
	}
	var matched *SSHInstanceTarget
	for i := range targets {
		// Canonical id equals target.ID; legacy UID-derived selectors still
		// resolve to the same Ready pod via MatchServiceInstance.
		if !ids.MatchServiceInstance(instanceID, targets[i].ServiceID, targets[i].PodName, targets[i].podUID) {
			continue
		}
		if matched != nil {
			// Ambiguous (e.g. two pods sharing a legacy UID hash) — never
			// fall through to an arbitrary replica.
			return SSHInstanceTarget{}, core.ErrNotFound
		}
		matched = &targets[i]
	}
	if matched == nil {
		return SSHInstanceTarget{}, core.ErrNotFound
	}
	return *matched, nil
}

// shellTicketTTL bounds a Browser Web Shell exec ticket. It is short because
// the dashboard opens the gateway WebSocket immediately after minting it; a
// tight window limits the value of a leaked ticket and the single-use nonce
// makes reuse a no-op on a given gateway replica.
const shellTicketTTL = 90 * time.Second

// ShellSessionView is the minted exec ticket the dashboard terminal presents to
// the gateway WebSocket to open a Browser Web Shell (docs/ADR035-ssh.md
// § Browser Web Shell). It carries no terminal content and no SSH key.
type ShellSessionView struct {
	Ticket    string `json:"ticket"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expiresAt"`
}

// CreateShellSession authorizes can_operate on the service and mints a
// short-lived exec ticket the isolated gateway validates to open a browser
// terminal. bex-api never gains pods/exec — it only authorizes and signs; the
// gateway re-authorizes and re-resolves the instance for every connection. The
// eligibility gate mirrors sshAddress (paid, non-suspended web/private/worker,
// Running with a live revision and image); a specific instanceID, when given,
// must belong to the service (the gateway still re-validates it is Ready).
func (s *Service) CreateShellSession(ctx context.Context, name, instanceID string) (ShellSessionView, error) {
	// SECURITY (codex #1): an interactive shell yields arbitrary code execution in
	// the running pod, including reading its env vars and secret files — exactly
	// what can_view_sensitive gates on the API. Gating the shell on the weaker
	// can_operate would let a contributor `printenv` their way around that boundary,
	// so shell mint requires can_view_sensitive (developer and up).
	a, err := s.AuthorizeApp(ctx, core.RelCanViewSensitive, name)
	if err != nil {
		return ShellSessionView{}, err
	}
	if len(s.ShellTicketSecret) == 0 || strings.TrimSpace(s.ShellWSURL) == "" {
		return ShellSessionView{}, core.ErrShellUnavailable
	}
	v := s.view(a)
	if err := sshSessionReady(v); err != nil {
		return ShellSessionView{}, err
	}
	instanceID = strings.TrimSpace(instanceID)
	if instanceID != "" && !strings.HasPrefix(instanceID, v.ID+"-") {
		return ShellSessionView{}, fmt.Errorf("%w: instance does not belong to this service", core.ErrBadRequest)
	}
	id, _ := core.IdentityFrom(ctx)
	if id.Subject == "" {
		return ShellSessionView{}, core.ErrForbidden
	}
	now := s.Now()
	expires := now.Add(shellTicketTTL)
	token, err := shellticket.Mint(s.ShellTicketSecret, shellticket.Claims{
		Subject:    id.Subject,
		ServiceID:  v.ID,
		InstanceID: instanceID,
		IssuedAt:   now.Unix(),
		ExpiresAt:  expires.Unix(),
	})
	if err != nil {
		return ShellSessionView{}, err
	}
	return ShellSessionView{Ticket: token, URL: s.ShellWSURL, ExpiresAt: expires.UTC().Format(time.RFC3339)}, nil
}

func parseSSHUsername(username string) (serviceID, instanceID string, err error) {
	parts := strings.Split(strings.TrimSpace(username), "-")
	if len(parts) < 2 {
		return "", "", core.ErrBadRequest
	}
	serviceID = strings.Join(parts[:2], "-")
	if kind, ok := ids.KindOf(serviceID); !ok || kind != ids.Service {
		return "", "", core.ErrBadRequest
	}
	if len(parts) > 2 {
		instanceID = strings.Join(parts, "-")
	}
	return serviceID, instanceID, nil
}

func (s *Service) readySSHInstances(ctx context.Context, a *appv1alpha1.App, v AppView) ([]SSHInstanceTarget, error) {
	pods, err := s.AppPodsIn(ctx, a.Namespace, a.Name)
	if err != nil {
		return nil, err
	}
	candidates := make([]SSHInstanceTarget, 0, len(pods))
	instanceCounts := make(map[string]int, len(pods))
	for i := range pods {
		pod := &pods[i]
		if !readyCurrentAppPod(pod, v.Image, v.Revision) {
			continue
		}
		target := SSHInstanceTarget{
			ID:        ids.ServiceInstanceID(v.ID, pod.Name),
			CreatedAt: pod.CreationTimestamp.UTC().Format(time.RFC3339),
			ServiceID: v.ID,
			OwnerID:   v.OwnerID,
			Namespace: pod.Namespace,
			PodName:   pod.Name,
			podUID:    string(pod.UID),
			Container: core.AppContainer,
		}
		candidates = append(candidates, target)
		instanceCounts[target.ID]++
	}
	targets := make([]SSHInstanceTarget, 0, len(candidates))
	for _, target := range candidates {
		if instanceCounts[target.ID] == 1 {
			targets = append(targets, target)
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].CreatedAt == targets[j].CreatedAt {
			return targets[i].ID < targets[j].ID
		}
		return targets[i].CreatedAt < targets[j].CreatedAt
	})
	return targets, nil
}

func readyCurrentAppPod(pod *corev1.Pod, activeImage, activeRevision string) bool {
	if activeImage == "" || activeRevision == "" || pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
		return false
	}
	if pod.Labels[core.PodLabelRevision] != activeRevision {
		return false
	}
	ready := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}
	if !ready {
		return false
	}
	for _, container := range pod.Spec.Containers {
		if container.Name == core.AppContainer {
			return container.Image == activeImage
		}
	}
	return false
}

// CreateRequest is the neutral create-or-update input the three surfaces (and
// the render.yaml deploy mapper) share — Render's create body projected onto the
// App CR spec. One of Repo/Image is required. Zero values fall back to the
// platform defaults the operator would apply (branch main, port 3000, one
// replica, the catalog's default tier). Plan accepts either Render's spelling
// ("pro_plus") or a bex tier id ("pro-plus"). Hosts are external FQDNs, the
// first canonical (spec.host); only a web_service is additionally exposed at the
// platform hostname <name>.<BEX_BASE_DOMAIN>.
type CreateRequest struct {
	// Disk attaches a persistent volume at create time — the Blueprint path's
	// only way in, since render.yaml declares a disk inline with its service.
	// nil means no disk is being declared, which on a sync PRESERVES whatever
	// the service already has (ADR082 D7).
	Disk *ServiceDiskView `json:"disk,omitempty"`

	// OwnerID is the workspace to create the service IN — Render's `ownerId`
	// (w6/m14). Empty means the caller's default workspace (their oldest
	// membership), so a single-workspace client never has to say it; a workspace
	// the caller is not a member of is core.ErrForbidden, never a create that
	// silently lands somewhere else. All three surfaces fill it (REST body,
	// GraphQL arg, or MCP's per-call workspaceId), so the workspace a
	// create targets is decided in exactly one place: Create.
	OwnerID string
	// EnvironmentID optionally assigns the new service to an existing
	// environment (and therefore that environment's project).
	EnvironmentID string
	// EnvironmentSpecified distinguishes a Blueprint resource explicitly nested
	// under ungrouped: (clear membership) from a create that simply omitted the
	// field (preserve current behavior on idempotent stack re-apply).
	EnvironmentSpecified bool
	Name                 string
	// Type is the Render serviceType: web_service (default), private_service,
	// background_worker, cron_job. Empty defaults to web_service.
	Type string
	// Schedule is the cron expression, required when Type is cron_job.
	Schedule string
	// Command overrides a cron_job's default entrypoint (spec.command); empty
	// runs the image's own command. Ignored for every other type.
	Command string
	Repo    string
	Image   string
	// RegistryCredentialID follows Render's registryCredentialId semantics.
	// nil means omitted (preserve bex's host auto-resolution); a pointer to an
	// empty string explicitly selects no credential; any other value binds that
	// exact credential after workspace validation and, for image-backed sources,
	// image-host validation. Dockerfile builds use the stored credential host.
	RegistryCredentialID *string
	Branch               string
	Builder              string
	Runtime              string
	BuildCommand         string
	StartCommand         string
	// RootDir scopes build-from-git to a subdirectory of Repo (Render's Root
	// Directory setting, for monorepos; spec.rootDir). Empty is the repo root.
	RootDir string
	// BuildFilter is Render's Build Filters at create time (spec.buildFilter):
	// glob patterns gating git-push auto-deploys. nil means unset (every matching
	// push deploys). Validated + canonicalized by normalizeBuildFilter; editable
	// later via SetBuildFilter.
	BuildFilter    *BuildFilterView
	DockerfilePath string
	// DockerContext is Render's Docker build context directory, relative to
	// the repo root and independent of RootDir (docker builds only).
	DockerContext   string
	Port            int32
	Replicas        int32
	Plan            string
	HealthCheckPath string
	// MaxShutdownDelaySeconds is Render's per-service SIGTERM grace window.
	// nil means omitted (the shared Render/Kubernetes default of 30 seconds);
	// non-nil values must be 1-300 and only apply to web/private/worker services.
	MaxShutdownDelaySeconds *int32
	Env                     []appv1alpha1.EnvVar
	// SecretFiles are Render's create-time secret files. They live in OpenBao,
	// not the App spec, and are materialized by CreateSecrets before the App CR
	// is written, so the first pod already mounts them.
	SecretFiles []core.SecretFile
	Hosts       []string
	// AutoDeploy controls whether a signed git push to Branch redeploys this App
	// (the webhook honors spec.autoDeploy). nil => the default: on for a
	// repo-backed service (Render's default too), off for an image-backed one
	// (nothing to rebuild on push).
	AutoDeploy *bool
	// NotifyOnFail is Render's per-service deploy-failure notification override
	// (spec.notifyOnFail): default | notify | ignore, docs/render-artifacts/
	// notify-on-fail.md. Empty means "default" (defer to each member's own
	// w3/m9 preference). An unrecognized value is core.ErrBadRequest.
	NotifyOnFail string
	// SubdomainPolicy is Render's renderSubdomainPolicy: "enabled" (default,
	// platform host active) or "disabled" (platform host dropped; only custom
	// hosts in Hosts[] serve the App). Cannot be "disabled" with no Hosts —
	// that would leave the service unreachable. An unrecognized value is
	// core.ErrBadRequest.
	SubdomainPolicy string
	// PreDeployCommand is Render's Pre-Deploy Command (spec.preDeployCommand): a
	// command run to completion against the new revision's image before it serves
	// traffic (typically a DB migration); a non-zero exit fails the deploy. Empty
	// means no pre-deploy step. Ignored for cron_job/static_site.
	PreDeployCommand string
	// InitialDeployHook is Render's initialDeployHook (w2/m45): a blueprint-only
	// command that runs exactly once, on the first successful deploy. REST/GraphQL/MCP
	// callers leave this empty; it is populated only by the blueprint parse path.
	// Gated by the bex.co/initial-deploy-hook-ran annotation on the App CR; after it
	// runs, subsequent deploys and blueprint re-syncs use PreDeployCommand instead.
	InitialDeployHook string
	// PublishPath is the built output directory a static_site serves (Render's
	// "Publish Directory", spec.publishPath). Required when Type is static_site,
	// ignored otherwise.
	PublishPath string
	// Routes / Headers are a static_site's edge rules at create time (spec.routes /
	// spec.headers). Ignored for every other type; editable later via
	// SetRoutes/SetHeaders.
	Routes  []StaticRouteView
	Headers []StaticHeaderView
	// IPAllowList is Render's inbound IP allowlist. Both CIDR and description
	// persist on the App CR; enforcement consumes CIDRs only. Empty means open
	// to all source IPs (Render's default). Only meaningful for web_service and
	// static_site.
	IPAllowList []core.IPAllowListEntry
	// MaintenanceMode is Render's maintenanceMode object at create time
	// (spec.maintenanceMode): {enabled, uri}. nil means unset (disabled).
	// web_service only — a non-nil value for any other type is core.ErrBadRequest.
	// A later toggle is SetMaintenanceMode, never re-applied by a redeploy
	// (applyCreateToSpec deliberately leaves it alone — it's a runtime state,
	// not desired build config; see that function's doc comment).
	MaintenanceMode *MaintenanceModeView
	// Autoscaling is Render's scaling block at create time (w2/m49): only the
	// Blueprint path sets this field (render.yaml `scaling:`); direct create
	// callers leave it nil and configure autoscaling later via SetAutoscaling.
	// Non-nil enables autoscaling immediately — same validation as SetAutoscaling.
	Autoscaling *SetAutoscalingRequest
	// DryRun, when true, resolves the spec and returns a preview without any
	// Kubernetes or control-plane-store writes — zero side effects (w2/m29).
	// The response shape is identical to a live create; the caller knows it is a
	// dry-run because they set this flag. Validation (specFromCreate) still runs,
	// so an invalid request still returns an error.
	DryRun bool
}

// stampEnvironmentMembership applies an environment assignment onto a newborn
// App CR: project/environment labels, the inherited inbound-IP layer
// (w4/m28), and the isolation label when the environment demands it — one
// helper for every create path so a new environment-derived field can't be
// stamped in some paths and forgotten in others. Callers initialize Labels
// before calling; a zero assignment is a no-op.
func stampEnvironmentMembership(a *appv1alpha1.App, environment core.EnvironmentAssignment) {
	if environment.ID == "" {
		return
	}
	a.Labels[core.LabelProject] = environment.ProjectID
	a.Labels[core.LabelEnvironment] = environment.ID
	// Newborn members inherit the environment's inbound-IP layer.
	a.Spec.EnvironmentIPAllowList = core.EnvironmentLayerCIDRs(environment.IPAllowList)
	if environment.NetworkIsolationEnabled {
		a.Labels[core.LabelNetworkIsolation] = environment.ID
	}
}

// Create writes the App CR for a new service, or updates it in place when one
// of the same name already exists — the same verb "deploy this" rides (Deploy
// maps a repo + render.yaml onto a CreateRequest, docs/ADR006-bex-api.md). Repeating the
// call for an existing service is a redeploy, not a duplicate: the spec fields
// the request carries are re-applied and spec.restartedAt is bumped, so a
// repo-backed App re-runs its build-from-git. Intent only — the operator
// converges the CR into a running service with a live URL.
//
// When the control-plane store is configured and the caller has a resolved
// tenant, it writes a store row first (unified create path, w2/m11): the row
// opens a deploy record so history is populated from the first create, and the
// store labels (managed-by + app-id) are stamped on the CR so the projector
// and lifecycle verbs recognise it as store-managed. Without the store, or when
// no tenant resolves, the CR is written directly (the hand-applied path
// scripts/app-apply.sh uses) — behaviour unchanged from before.
// req.OwnerID names the workspace to create in. It is bound to the context
// BEFORE the authorization check, which is what makes it safe: Authorize then
// checks can_create against THAT workspace (403 for a caller who is not a
// member of it), and the same resolution flows through s.Tenant below into the
// App's tenant label and its store row — so a create can never be authorized
// against one workspace and land in another.
func (s *Service) Create(ctx context.Context, req CreateRequest) (AppView, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return AppView{}, err
	}
	if len(req.SecretFiles) > 0 {
		for i := range req.SecretFiles {
			req.SecretFiles[i].Name = strings.TrimSpace(req.SecretFiles[i].Name)
			if !core.ValidSecretFileName(req.SecretFiles[i].Name) {
				return AppView{}, fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, req.SecretFiles[i].Name)
			}
		}
		if s.CreateSecrets == nil {
			return AppView{}, core.ErrSecretsUnavailable
		}
	}
	return s.create(ctx, req)
}

// create is the unauthorized core of Create — a same-workspace duplicate name
// is rejected (Render-style, w4/m19), never silently redeployed; that's what
// the deploy/restart verbs and the stack path's applyCreate (deploy.go, an
// idempotent upsert by design) are for.
func (s *Service) create(ctx context.Context, req CreateRequest) (AppView, error) {
	desired, err := specFromCreate(req)
	if err != nil {
		return AppView{}, err
	}
	if err := s.validateNewSpecMaintenanceMode(ctx, req.Name, desired); err != nil {
		return AppView{}, err
	}
	// The workspace this create acts in: the one named by req.OwnerID (already
	// membership-checked by Create's Authorize) or the caller's default. Empty
	// when the store is off or the caller is unbound. Resolved BEFORE the
	// existence probe, because it is what makes that probe workspace-correct.
	tenantID, _ := s.Tenant(ctx)
	environment, err := core.ResolveEnvironmentForCreate(ctx, s.Environments, req.EnvironmentID, tenantID)
	if err != nil {
		return AppView{}, err
	}

	// Dry-run: return the resolved spec preview without any k8s or store writes.
	if req.DryRun {
		a := &appv1alpha1.App{}
		a.Name = req.Name
		a.Namespace = s.AppNamespace(tenantID)
		a.Spec = desired
		if tenantID != "" {
			a.Labels = map[string]string{core.LabelTenant: tenantID}
		}
		stampEnvironmentMembership(a, environment)
		return s.view(a), nil
	}
	if err := s.RequirePlanBilling(ctx, tenantID, desired.Tier); err != nil {
		return AppView{}, err
	}

	// Duplicate check, scoped to exactly the target workspace (w4/m19) —
	// deliberately NOT GetApp, whose cross-workspace fallback exists so a
	// caller can reach one of their OTHER workspaces' Apps by bare name; reused
	// here it would make a create in workspace B see workspace A's same-named
	// App as "taken" merely because the caller also belongs to A, refusing a
	// creation the milestone exists to allow (two workspaces both owning
	// "web").
	taken, err := s.nameTaken(ctx, tenantID, req.Name)
	if err != nil {
		return AppView{}, err
	}
	if taken {
		return AppView{}, core.NewConflictError("CONFLICT", fmt.Sprintf("name %q is already in use", req.Name), nil)
	}

	a := &appv1alpha1.App{}
	a.Name = req.Name
	if tenantID != "" {
		// Collision-free object name (w4/m19): all tenants' Apps share one
		// namespace, so two tenants both naming a service "web" must not
		// collide on the bare name the way the pre-migration scheme did.
		a.Name = core.CRName(tenantID, req.Name)
		// LabelServiceName is what lets GetApp find this App from one of the
		// caller's OTHER workspaces by its public name (w4/m19) — metadata.Name
		// alone no longer serves that purpose once it's tenant-prefixed.
		a.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelServiceName: req.Name}
	}
	a.Namespace = s.AppNamespace(tenantID)
	a.Spec = desired
	stampEnvironmentMembership(a, environment)
	seed := createSeed{files: req.SecretFiles}
	// A nil seeder (OpenBao off) keeps the pre-w6/m45 spec-only behavior.
	if s.CreateSecrets != nil {
		seed.env = takeCreateEnvLiterals(&a.Spec)
	}
	return s.materializeNewApp(ctx, req, a, tenantID, environment, seed)
}

// createSeed is what a new service is born with that does NOT live on its App
// spec: the create request's secret files and its literal env vars, both
// written to the mutable stores (and referenced from the spec) before the CR
// exists. Bundled so the two always travel together through the create tail.
type createSeed struct {
	files []core.SecretFile
	env   map[string]string
}

func (s createSeed) empty() bool { return len(s.files) == 0 && len(s.env) == 0 }

// takeCreateEnvLiterals removes a newborn spec's literal env vars from spec.Env
// and returns them as the map the create-time seeder writes into the mutable
// env store. Moving rather than copying is the whole point (w6/m45): the store
// becomes the single writer, so the Environment tab and the envVars/envVar
// REST/GraphQL/MCP reads list, reveal, edit, export and delete a var set at
// creation time exactly like one added later. A copy left on spec.Env would
// also SHADOW the projected Secret in the running container — Kubernetes `env`
// beats `envFrom` — so every later edit or delete would be silently ignored by
// the process.
//
// Entries the env store cannot represent stay exactly where they are: a
// ValueFrom entry is a Secret key reference, not a literal (the shape a bex.yml
// fromDatabase reference resolves to), and a name outside core.ValidEnvKey
// would fail the projection Secret's write — both keep their spec-only
// behavior rather than newly failing a create that works today.
func takeCreateEnvLiterals(spec *appv1alpha1.AppSpec) map[string]string {
	var literals map[string]string
	kept := make([]appv1alpha1.EnvVar, 0, len(spec.Env))
	for _, item := range spec.Env {
		if item.ValueFrom != nil || !core.ValidEnvKey(item.Name) {
			kept = append(kept, item)
			continue
		}
		if literals == nil {
			literals = map[string]string{}
		}
		literals[item.Name] = item.Value
	}
	if literals == nil {
		return nil
	}
	if len(kept) == 0 {
		kept = nil
	}
	spec.Env = kept
	return literals
}

// nameTaken reports whether name is already claimed in the exactly-one
// workspace this create targets (w4/m19): tenantID's own object name
// (CRName(tenantID, name)) first, then the bare name IF it belongs to that
// SAME tenant — a store-managed App created before this scheme shipped is
// still object-named bare (never renamed in place), so its own workspace must
// still see it as taken; a bare-named App belonging to a DIFFERENT tenant (or
// none) must not. tenantID == "" (store off / unbound caller) falls back to
// the single flat bare-name probe, the pre-migration behavior for that mode.
func (s *Service) nameTaken(ctx context.Context, tenantID, name string) (bool, error) {
	get := func(objName string) (*appv1alpha1.App, error) {
		var a appv1alpha1.App
		err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.AppNamespace(tenantID), Name: objName}, &a)
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &a, nil
	}
	if tenantID == "" {
		a, err := get(name)
		return a != nil, err
	}
	if a, err := get(core.CRName(tenantID, name)); a != nil || err != nil {
		return a != nil, err
	}
	a, err := get(name)
	if err != nil {
		return false, err
	}
	return a != nil && a.Labels[core.LabelTenant] == tenantID, nil
}

// createNewApp writes a brand-new App CR (the not-found path both create and the
// stack applyCreate share): stamps the tenant + store labels, opens the store
// row + its first deploy record when the store is on, mints a clone secret for a
// private repo, and creates the CR. Shared so the stack path creates services
// identically to the interactive create (w1/m24).
func (s *Service) createNewApp(ctx context.Context, req CreateRequest, desired appv1alpha1.AppSpec) (AppView, error) {
	a := &appv1alpha1.App{}
	a.Name = req.Name
	a.Namespace = s.Namespace
	a.Spec = desired
	// Resolve the caller's tenant; used both for the tenant label and the
	// store row. Empty when the store is off or the caller is unbound.
	tenantID := ""
	if s.Workspace != nil {
		if t, ok := s.Tenant(ctx); ok {
			tenantID = t
		}
	}
	if tenantID != "" {
		// Collision-free object name (w4/m19) — see create's identical stamp;
		// the stack path shares this helper so it never re-introduces the
		// bare-name collision create() just closed.
		a.Name = core.CRName(tenantID, req.Name)
		a.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelServiceName: req.Name}
		// Land the CR (and its clone/pull Secrets, written to a.Namespace) in the
		// workspace's own namespace under per-tenant isolation (ADR043), matching
		// where the projector projects it; AppNamespace == s.Namespace when off.
		a.Namespace = s.AppNamespace(tenantID)
	}
	environment, err := core.ResolveEnvironmentForCreate(ctx, s.Environments, req.EnvironmentID, tenantID)
	if err != nil {
		return AppView{}, err
	}
	stampEnvironmentMembership(a, environment)
	// Persist the initialDeployHook command for echo-back on reads (w2/m45).
	// The ran-once annotation is added by applyCreate once the first pre-deploy
	// Job succeeds; createNewApp only stores the command.
	if req.InitialDeployHook != "" {
		if a.Annotations == nil {
			a.Annotations = map[string]string{}
		}
		a.Annotations[initialDeployHookAnnotation] = req.InitialDeployHook
	}
	// No env-literal move here: this is the Blueprint/stack create path, where
	// spec.Env is manifest-owned and re-applied on every sync
	// (applyCreateToSpec's `dst.Env = want.Env`). Seeding those into the mutable
	// store would make two writers of the same key and revert a dashboard edit
	// on the next sync — which is exactly what render.yaml's `sync: false` (and
	// bex's existing EnvSeeder for it) exists to opt out of.
	return s.materializeNewApp(ctx, req, a, tenantID, environment, createSeed{files: req.SecretFiles})
}

// materializeNewApp is the shared write tail of create and createNewApp, run
// once the caller has shaped the CR (object name, namespace, spec, tenant +
// environment labels, and any annotations): the store row + its first deploy
// record when the store is on — with the UNIQUE(tenant_id, name) ErrConflict
// backstop behind both callers' duplicate pre-checks (a concurrent duplicate
// create that slips past them still surfaces as ErrConflict here, never an
// unclassified 500) and the managed-by/app-id/workspace/slug stamps — then the
// LabelAppID fallback mint for a store-less create, the clone + external-
// registry pull Secrets, and Touch → writeNewApp → Kick → view.
//
// The source-of-truth row is written before the CR so the CR can be stamped
// with the row id and its initial deploy history exists as soon as create
// succeeds; if any later step fails, remove that row again — otherwise the
// projector can resurrect a service the API reported as failed, potentially
// from the store's narrower projection of the desired spec (for example after
// a stale CRD rejects a newly added field). That applies to the stack path
// too: "the next apply re-converges" only helps if a next apply ever comes.
// provisionAppIdentity opens the control-plane row (when the store is on and a
// tenant is resolved) and stamps the CR's identity: the managed-by/app-id/
// workspace labels the projector and lifecycle verbs key on, plus the globally
// unique slug that drives the platform host. It returns the created row's id
// (empty when no row was written, so the caller knows whether to roll back) and
// the first deploy id the create opened.
func (s *Service) provisionAppIdentity(ctx context.Context, req CreateRequest, a *appv1alpha1.App, tenantID string, environment core.EnvironmentAssignment) (createdRowID, firstDeployID string, err error) {
	if a.Labels == nil {
		a.Labels = map[string]string{}
	}
	if s.Store != nil && tenantID != "" {
		row, err := s.Store.CreateApp(ctx, store.App{
			TenantID: tenantID,
			Name:     req.Name,
			// The service type the projector needs to derive spec.expose
			// (w6/m46 t001). Recorded once here — spec.type is immutable, so
			// no later write path revisits it.
			Type:                 a.Spec.Type,
			Repo:                 req.Repo,
			Image:                req.Image,
			RegistryCredentialID: clonePtr(a.Spec.RegistryCredentialID),
			Branch:               a.Spec.Branch,
			Port:                 a.Spec.Port,
			Replicas:             a.Spec.Replicas,
			Tier:                 a.Spec.Tier,
			ProjectID:            environment.ProjectID,
			EnvironmentID:        environment.ID,
			// Provenance for the first deploy row CreateApp opens (w9/001):
			// the branch tip this create will build, resolved best-effort.
			FirstDeployCommit: s.resolveDeployCommit(ctx, tenantID, req.Repo, a.Spec.Branch),
		})
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				return "", "", core.NewConflictError("CONFLICT", fmt.Sprintf("name %q is already in use", req.Name), nil)
			}
			return "", "", fmt.Errorf("creating service record: %w", err)
		}
		// Stamp the managed-by + app-id labels so the projector's byID index
		// finds this CR on its next pass (avoiding a duplicate create) and
		// lifecycle verbs (suspend/scale/plan) have an app-id to write through.
		a.Labels[store.LabelManagedBy] = store.ManagedByValue
		a.Labels[store.LabelAppID] = row.ID
		a.Labels[store.LabelWorkspace] = tenantID
		// The globally-unique slug (w4/m19) drives the platform host
		// (operator effectiveHosts) — never req.Name, which is only
		// workspace-unique and can collide across tenants.
		a.Spec.Subdomain = row.Slug
		createdRowID, firstDeployID = row.ID, row.FirstDeployID
	}
	// The control-plane row already supplied its canonical id above. A direct
	// API create has no row, so persist an equally Render-shaped service id on
	// the CR; all later reads and lifecycle verbs can resolve it by label.
	if a.Labels[core.LabelAppID] == "" {
		a.Labels[core.LabelAppID] = ids.New(ids.Service)
	}
	return createdRowID, firstDeployID, nil
}

func (s *Service) materializeNewApp(ctx context.Context, req CreateRequest, a *appv1alpha1.App, tenantID string, environment core.EnvironmentAssignment, seed createSeed) (AppView, error) {
	// A freshly minted workspace's tea-* namespace may not exist yet (the
	// NamespaceReconciler only converges it on its resync tick) — ensure it
	// before any write below (projection Secrets included) lands there (w2/026).
	if err := s.EnsureWorkspaceNamespace(ctx, tenantID); err != nil {
		return AppView{}, err
	}
	var createdRowID string
	rollbackStoreRow := func(cause error) error {
		if createdRowID == "" || s.Store == nil {
			return cause
		}
		if err := s.Store.DeleteApp(ctx, createdRowID); err != nil && !errors.Is(err, store.ErrNotFound) {
			return errors.Join(cause, fmt.Errorf("rolling back service record: %w", err))
		}
		if s.Kick != nil {
			s.Kick()
		}
		return cause
	}
	rowID, firstDeployID, err := s.provisionAppIdentity(ctx, req, a, tenantID, environment)
	if err != nil {
		return AppView{}, err
	}
	createdRowID = rowID
	if err := s.ensureHostsClaimable(ctx, a); err != nil {
		return AppView{}, rollbackStoreRow(err)
	}
	if createdRowID != "" {
		if claims, _, ok := s.managedDomainClaims(a); ok {
			declarations := domainDeclarations(a.Spec.Host, a.Spec.Hosts, a.Spec.HostRedirects)
			rows, err := claims.ReplaceDomainClaims(ctx, createdRowID, declarations)
			if err != nil {
				if errors.Is(err, store.ErrConflict) {
					err = errDomainInUse()
				}
				return AppView{}, rollbackStoreRow(fmt.Errorf("claim service domains: %w", err))
			}
			applyVerifiedDomainClaims(&a.Spec, rows)
		} else if err := s.Store.ReplaceDomains(ctx, createdRowID, a.Spec.Host, a.Spec.Hosts); err != nil {
			if errors.Is(err, store.ErrConflict) {
				err = errDomainInUse()
			}
			return AppView{}, rollbackStoreRow(fmt.Errorf("claim service domains: %w", err))
		}
	}
	// A private-connection repo gets a fresh clone token + spec.cloneSecret so
	// its first build authenticates. create-owned only: never set on a spec
	// that already hand-pointed cloneSecret elsewhere.
	if a.Spec.CloneSecret == "" {
		secretName, err := s.ensureCloneSecret(ctx, a)
		if err != nil {
			return AppView{}, rollbackStoreRow(err)
		}
		a.Spec.CloneSecret = secretName
	}
	pullSecretName, err := s.ensureExternalRegistryPullSecret(ctx, a)
	if err != nil {
		return AppView{}, rollbackStoreRow(err)
	}
	a.Spec.ExternalRegistryPullSecret = pullSecretName
	// Every other deploy-opening path stamps this; create was the one that left
	// the operator to infer the release from whatever metadata generation it
	// read first — so a write racing its first reconcile made it report a
	// release the deploy row had never heard of, which reads as a supersede and
	// closed brand-new deploys canceled (w6/m46 t004).
	stampReleaseGeneration(a, store.FirstDeployGeneration)
	resourcemeta.Touch(a, s.Now())
	if err := s.writeNewApp(ctx, req.Name, a, seed); err != nil {
		return AppView{}, rollbackStoreRow(err)
	}
	if s.Kick != nil {
		s.Kick()
	}
	v := s.view(a)
	v.LatestDeployID = firstDeployID
	return v, nil
}

// stampReleaseGeneration pins an App's open deploy to the release generation
// whose work it represents, so the operator adopts that identity instead of
// inferring one from whatever metadata generation it observes first (see
// AnnotationReleaseGeneration's own contract).
func stampReleaseGeneration(a *appv1alpha1.App, generation int64) {
	metav1.SetMetaDataAnnotation(&a.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration,
		strconv.FormatInt(generation, 10))
}

// mapServiceCapError translates a per-namespace ResourceQuota rejection of an
// App CR create (the count/apps.app.bex.co cap that replaced the app-code
// BEX_MAX_SERVICES check, ADR043 D3, w3/m34) into the same Render-shaped cap
// error the deleted check used to return (docs/ADR006-bex-api.md § Per-
// workspace resource caps), so create-past-cap stays a 400 with a readable
// message instead of a raw Kubernetes admission error leaking through as a 500.
// Any other error (including an unrelated Forbidden) passes through unchanged.
func mapServiceCapError(err error) error {
	if err == nil {
		return nil
	}
	if mapped, ok := core.QuotaCapError(err, store.AppsQuotaCountKey, "service"); ok {
		return mapped
	}
	return err
}

// writeNewApp makes create-time secret files AND literal env vars visible to
// the very first pod. Both projection Secrets and their App references are
// prepared before the App exists; after Kubernetes assigns the App UID, Commit
// adopts them. Every failure removes the pre-created projections and their
// OpenBao paths.
func (s *Service) writeNewApp(ctx context.Context, publicName string, a *appv1alpha1.App, seed createSeed) error {
	if seed.empty() {
		return mapServiceCapError(s.Client.Create(ctx, a))
	}
	if s.CreateSecrets == nil {
		return core.ErrSecretsUnavailable
	}
	if err := s.CreateSecrets.PrepareCreateSecrets(ctx, publicName, a, seed.files, seed.env); err != nil {
		return fmt.Errorf("prepare create secrets: %w", err)
	}
	abort := func(cause error) error {
		if err := s.CreateSecrets.AbortCreateSecrets(ctx, publicName, a); err != nil {
			return errors.Join(cause, fmt.Errorf("abort create secrets: %w", err))
		}
		return cause
	}
	if err := s.Client.Create(ctx, a); err != nil {
		return abort(mapServiceCapError(err))
	}
	if err := s.CreateSecrets.CommitCreateSecrets(ctx, publicName, a); err != nil {
		cause := fmt.Errorf("commit create secrets: %w", err)
		if deleteErr := s.Client.Delete(ctx, a); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
			cause = errors.Join(cause, fmt.Errorf("roll back App: %w", deleteErr))
		}
		return abort(cause)
	}
	return nil
}

// Delete removes a service — the single implementation the three adapters
// delegate to. With the store on and the App store-managed (it carries the
// store's app-id label), it keeps the apps row as a durable retry anchor while
// deleting every external, name-keyed secret. The still-present row also blocks
// a same-name replacement from inheriting a partially purged OpenBao path. Only
// after cleanup succeeds is the row removed, followed by the CR; once the row is
// gone the projector makes a failed CR delete converge without resurrection.
// Store-less
// (or a hand-applied App with no row) deletes the CR directly, the same split
// suspend/resume follow. The operator's ownerRefs cascade everything it derived
// (Deployment/Service/Ingress/CronJob/NetworkPolicy); the one orphan left is the
// cert-manager TLS Secret (documented in docs/ADR006-bex-api.md). Unknown id =>
// core.ErrNotFound; unauthorized => core.ErrForbidden.
func (s *Service) Delete(ctx context.Context, name string) error {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	if err := s.requireUnprotected(ctx, a, "delete"); err != nil {
		return err
	}
	// codex round-8 #8: deletion is irreversible (store row + CR cascade + OpenBao
	// purge) — re-assert can_create uncached so a member revoked inside
	// PositiveTTL cannot tear down one last service.
	if err := s.AuthorizeAppFresh(ctx, core.RelCanCreate, a); err != nil {
		return err
	}
	// Remove the private-repo clone Secret bex-api wrote for it (docs/github-
	// integration.md) — the operator doesn't own it (no ownerRef), so the CR
	// delete cascade wouldn't. Best-effort: an absent Secret (public app) is fine.
	if err := s.deleteCloneSecret(ctx, a.Namespace, a.Name); err != nil {
		return fmt.Errorf("delete clone secret: %w", err)
	}
	// Same for the external-registry pull Secret (w2/m14) — no ownerRef (see
	// pullsecret.go), so it needs the same explicit delete.
	if err := s.deleteExternalRegistryPullSecret(ctx, a.Namespace, a.Name); err != nil {
		return fmt.Errorf("delete registry pull secret: %w", err)
	}
	// Purge OpenBao env-var and secret-file paths — not owned by the App CR, so
	// they don't cascade with the CR delete and would otherwise linger forever.
	if s.SecretsEraser != nil {
		if err := s.SecretsEraser.PurgeApp(ctx, a); err != nil {
			return fmt.Errorf("purge app secrets: %w", err)
		}
	}
	if s.Store != nil {
		if id := managedAppID(a); id != "" {
			// An already-gone row is the intended end state, not an error (a
			// prior attempt may have completed the cleanup and row delete) —
			// fall through to delete the orphaned CR.
			if err := s.Store.DeleteApp(ctx, id); err != nil && !errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("delete source of truth: %w", err)
			}
		}
	}
	// IgnoreNotFound: the CR may already be gone (a racing projector pass, or a
	// store-managed App whose row delete triggered a Kick) — the end state is
	// what matters.
	return client.IgnoreNotFound(s.Client.Delete(ctx, a))
}

// specFromCreate validates a CreateRequest and projects it onto a fresh App
// spec. Validation mirrors the internal create API (store/api.go) so both paths
// agree on what a valid App is: DNS-label name, one of repo/image, a known
// plan, sane port/replica bounds.
func specFromCreate(req CreateRequest) (appv1alpha1.AppSpec, error) {
	if !store.ValidAppName(req.Name) {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: name must be a DNS label of 1-30 chars ([a-z0-9-])", core.ErrBadRequest)
	}
	if err := validateCreateSource(req); err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	svcType, err := normalizeType(req.Type)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	if err := validateTypeSpecificCreate(svcType, req); err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	tier, err := normalizeTierForType(svcType, req.Plan)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	if err := validateMaintenanceEligibility(svcType, tier, req.MaintenanceMode); err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	port, replicas, branch, autoDeploy, err := normalizeCreateDefaults(req)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	// A Blueprint declares a disk inline with its service, so eligibility is
	// checked here rather than on a later attach — there is no moment when the
	// service exists without it.
	disk, err := validateCreateDisk(svcType, tier, replicas, req.Disk)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	// Create sets replicas directly, without going through Scale — gate it by the
	// same plan cap (w6/m118 t003) so a caller cannot simply create at N instead
	// of scaling to N. Autoscaling min/max is capped separately in autoscalingSpec.
	if err := checkInstanceCap(tier, replicas); err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	runtime, builder, err := resolveBuildStrategy(req)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	// A static site that declares buildCommand builds through the native
	// toolchain so the declared command actually runs (Render's static build
	// environment); the auto/CNB path would silently ignore it (ADR029). The
	// toolchain default is the build plane's choice — runtime stays empty so
	// Render-shaped reads never surface an internal toolchain name.
	if svcType == appv1alpha1.TypeStaticSite && strings.TrimSpace(req.BuildCommand) != "" &&
		strings.TrimSpace(req.DockerfilePath) == "" && runtime == "" &&
		(builder == "" || builder == "auto") {
		builder = "native"
	}
	buildFilter, err := normalizeBuildFilter(req.BuildFilter)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	maintenanceMode, err := normalizeMaintenanceMode(req.MaintenanceMode)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	notifyOnFail, err := normalizeNotifyOnFail(req.NotifyOnFail)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	subdomainPolicy, err := resolveCreateSubdomainPolicy(req)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	if err := core.ValidateAllowList(req.IPAllowList); err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	spec := appv1alpha1.AppSpec{
		Type:            svcType,
		Repo:            req.Repo,
		Image:           req.Image,
		Branch:          branch,
		Runtime:         runtime,
		BuildCommand:    req.BuildCommand,
		StartCommand:    req.StartCommand,
		Builder:         builder,
		RootDir:         req.RootDir,
		BuildFilter:     buildFilter,
		MaintenanceMode: maintenanceMode,
		DockerfilePath:  req.DockerfilePath,
		DockerContext:   req.DockerContext,
		Port:            port,
		Replicas:        replicas,
		Tier:            tier,
		HealthCheckPath: req.HealthCheckPath,
		Disk:            disk,
		MaxShutdownDelaySeconds: clonePtr(
			req.MaxShutdownDelaySeconds,
		),
		Env:              req.Env,
		AutoDeploy:       autoDeploy,
		NotifyOnFail:     notifyOnFail,
		SubdomainPolicy:  subdomainPolicy,
		PreDeployCommand: strings.TrimSpace(req.PreDeployCommand),
		// A web service and a static site are public: expose them at
		// <name>.<BEX_BASE_DOMAIN> so the caller gets a live URL with no custom
		// domain. Every other type opts out (private has no platform host;
		// worker/cron have no HTTP endpoint at all). On the CRD contract's
		// helper rather than a second copy of the rule — the projector kept its
		// own copy and got it wrong (w6/m46 t001).
		Expose: appv1alpha1.TypePubliclyRoutable(svcType),
	}
	spec.SetIPAllowListEntries(core.AllowListToSpec(req.IPAllowList))
	if err := applyOptionalCreateSpec(&spec, svcType, req); err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	return spec, nil
}

// validateCreateSource checks the create's source inputs: one of repo/image,
// no build-from-git field alongside a prebuilt image, and — because the
// operator never forwards an unchecked repo/branch/rootDirectory into the
// BuildKit context string (w6/m6 t003) — the git inputs' shape. A bare image
// deploy skips those (no build).
func validateCreateSource(req CreateRequest) error {
	if req.Repo == "" && req.Image == "" {
		return fmt.Errorf("%w: one of repo or image is required", core.ErrBadRequest)
	}
	if req.Image != "" {
		for _, sourceField := range prebuiltImageSourceFields {
			if sourceField.declaredInCreate != nil && sourceField.declaredInCreate(req) {
				return fmt.Errorf("%w: %s", core.ErrBadRequest, prebuiltImageSourceFieldMessage(sourceField.createName))
			}
		}
	}
	if req.Repo != "" && !store.ValidRepo(req.Repo) {
		return fmt.Errorf("%w: repo must be an https/ssh/git URL", core.ErrBadRequest)
	}
	if req.Branch != "" && !store.ValidGitRef(req.Branch) {
		return fmt.Errorf("%w: branch must be a git ref (no shell metacharacters)", core.ErrBadRequest)
	}
	if req.Image != "" && !store.ValidImage(req.Image) {
		return fmt.Errorf("%w: image must be an OCI reference (no whitespace or shell metacharacters)", core.ErrBadRequest)
	}
	if req.RootDir != "" && !store.ValidRootDir(req.RootDir) {
		return fmt.Errorf("%w: rootDirectory must be a relative path with no '..' components", core.ErrBadRequest)
	}
	if req.DockerfilePath != "" && !store.ValidRootDir(req.DockerfilePath) {
		return fmt.Errorf("%w: dockerfilePath must be a relative path with no '..' components", core.ErrBadRequest)
	}
	if req.DockerContext != "" && !store.ValidRootDir(req.DockerContext) {
		return fmt.Errorf("%w: dockerContext must be a relative path with no '..' components", core.ErrBadRequest)
	}
	return nil
}

// validateTypeSpecificCreate enforces the create fields tied to the service
// type: the shutdown grace window only applies to types that run continuously;
// a cron_job needs a schedule; a private/worker/cron has no public ingress, so it can't carry
// custom domains (same rule the deploy manifest enforces for private
// services); a static_site builds from a Git repo — a prebuilt image is
// rejected (ADR029; Render parity) — with publish/edge fields required for it
// and rejected for every other type.
func validateTypeSpecificCreate(svcType string, req CreateRequest) error {
	if err := validateMaxShutdownDelaySeconds(svcType, req.MaxShutdownDelaySeconds); err != nil {
		return err
	}
	if svcType == appv1alpha1.TypeCronJob {
		sched := strings.TrimSpace(req.Schedule)
		if sched == "" {
			return fmt.Errorf("%w: schedule is required for a cron_job", core.ErrBadRequest)
		}
		if !validCronSchedule(sched) {
			return fmt.Errorf("%w: schedule must be a valid 5-field cron expression (e.g. '0 * * * *')", core.ErrBadRequest)
		}
	}
	// Only a type served at a public host can carry custom domains. This named
	// background_worker and cron_job explicitly and so let a private_service
	// through, accepting a domain the platform will never serve (w6/m46 t002).
	if !appv1alpha1.TypePubliclyRoutable(svcType) && len(req.Hosts) > 0 {
		return errNoPublicIngress(svcType)
	}
	// A static_site needs a publish directory, and its edge rules must be valid.
	if svcType == appv1alpha1.TypeStaticSite {
		if req.Image != "" {
			return fmt.Errorf("%w: a static_site builds from a Git repo; a prebuilt image is not supported", core.ErrBadRequest)
		}
		if strings.TrimSpace(req.PublishPath) == "" {
			return fmt.Errorf("%w: publishPath is required for a static_site", core.ErrBadRequest)
		}
		if err := validateRoutes(req.Routes); err != nil {
			return err
		}
		return validateHeaders(req.Headers)
	}
	if strings.TrimSpace(req.PublishPath) != "" || len(req.Routes) > 0 || len(req.Headers) > 0 {
		return fmt.Errorf("%w: publishPath/routes/headers only apply to a static_site", core.ErrBadRequest)
	}
	return nil
}

// normalizeCreateDefaults bounds-checks the create's port/replica values and
// resolves the fields with request-shape defaults: port (DefaultPort),
// replicas (1), branch ("main" for a repo-backed service), and autoDeploy.
func normalizeCreateDefaults(req CreateRequest) (port, replicas int32, branch string, autoDeploy bool, err error) {
	if req.Port < 0 || req.Port > 65535 {
		return 0, 0, "", false, fmt.Errorf("%w: port must be 1-65535", core.ErrBadRequest)
	}
	if req.Replicas < 0 || req.Replicas > store.MaxReplicas {
		return 0, 0, "", false, fmt.Errorf("%w: replicas must be 0-%d", core.ErrBadRequest, store.MaxReplicas)
	}
	port = req.Port
	if port == 0 {
		port = appv1alpha1.DefaultPort
	}
	replicas = req.Replicas
	if replicas == 0 {
		replicas = 1
	}
	branch = req.Branch
	if branch == "" && req.Repo != "" {
		branch = appv1alpha1.DefaultBranch
	}
	// AutoDeploy: default on for a repo-backed service (a push should redeploy,
	// Render's default), off for an image-backed one (no repo to rebuild from).
	// An explicit request value wins.
	autoDeploy = req.Repo != ""
	if req.AutoDeploy != nil {
		autoDeploy = *req.AutoDeploy
	}
	return port, replicas, branch, autoDeploy, nil
}

// resolveBuildStrategy folds the create's runtime/builder pair into the
// effective builder: runtime wins (docker → dockerfile, image → auto, a
// Blueprint-native runtime → native with its command requirements), and a
// bare builder value is validated as-is.
func resolveBuildStrategy(req CreateRequest) (string, string, error) {
	builder := req.Builder
	runtime := strings.ToLower(strings.TrimSpace(req.Runtime))
	if runtime != "" && builder != "" && builder != "auto" {
		return "", "", fmt.Errorf("%w: runtime and builder cannot both select a build strategy", core.ErrBadRequest)
	}
	switch runtime {
	case "":
		if builder != "" && builder != "auto" && builder != "buildpack" && builder != "dockerfile" {
			return "", "", fmt.Errorf("%w: builder must be auto, buildpack, or dockerfile", core.ErrBadRequest)
		}
	case "docker":
		builder = "dockerfile"
	case "image":
		if req.Image == "" || req.Repo != "" {
			return "", "", fmt.Errorf("%w: runtime image requires image and no repo", core.ErrBadRequest)
		}
		builder = "auto"
	default:
		if !blueprintNativeRuntime(runtime) {
			return "", "", fmt.Errorf("%w: unsupported runtime %q", core.ErrBadRequest, runtime)
		}
		if req.Repo == "" {
			return "", "", fmt.Errorf("%w: native runtime %s requires repo", core.ErrBadRequest, runtime)
		}
		if strings.TrimSpace(req.BuildCommand) == "" || strings.TrimSpace(req.StartCommand) == "" {
			return "", "", fmt.Errorf("%w: native runtime %s requires buildCommand and startCommand", core.ErrBadRequest, runtime)
		}
		builder = "native"
	}
	return runtime, builder, nil
}

// resolveCreateSubdomainPolicy normalizes the create's renderSubdomainPolicy
// and refuses to disable the platform subdomain when no custom domain would
// remain to serve the app from.
func resolveCreateSubdomainPolicy(req CreateRequest) (string, error) {
	subdomainPolicy, err := normalizeSubdomainPolicy(req.SubdomainPolicy)
	if err != nil {
		return "", err
	}
	if subdomainPolicy == appv1alpha1.SubdomainPolicyDisabled && len(req.Hosts) == 0 {
		return "", fmt.Errorf("%w: renderSubdomainPolicy cannot be disabled without at least one custom domain", core.ErrBadRequest)
	}
	return subdomainPolicy, nil
}

// applyOptionalCreateSpec projects the create's type-specific and optional
// fields onto the spec: the trimmed registry credential, cron
// schedule/command, static-site publish/edge rules, custom domains, and
// autoscaling.
func applyOptionalCreateSpec(spec *appv1alpha1.AppSpec, svcType string, req CreateRequest) error {
	if registryCredentialID := clonePtr(req.RegistryCredentialID); registryCredentialID != nil {
		*registryCredentialID = strings.TrimSpace(*registryCredentialID)
		spec.RegistryCredentialID = registryCredentialID
	}
	if svcType == appv1alpha1.TypeCronJob {
		spec.Schedule = strings.TrimSpace(req.Schedule)
		spec.Command = strings.TrimSpace(req.Command)
	}
	if svcType == appv1alpha1.TypeStaticSite {
		spec.PublishPath = strings.TrimSpace(req.PublishPath)
		spec.Routes = routesFromViews(req.Routes)
		spec.Headers = headersFromViews(req.Headers)
	}
	if len(req.Hosts) > 0 {
		hosts, err := canonicalHosts(req.Hosts)
		if err != nil {
			return err
		}
		spec.Host = hosts[0]
		spec.Hosts = hosts[1:]
	}
	if req.Autoscaling != nil {
		as, err := autoscalingSpec(*req.Autoscaling, spec.Tier)
		if err != nil {
			return err
		}
		spec.Autoscaling = &as
	}
	return nil
}

// canonicalHosts canonicalizes every requested custom hostname (trimmed,
// terminal dot dropped, lowercased, DNS-1123 validated) so the cross-App
// uniqueness sweep and every downstream consumer compare like with like — a
// case/trailing-dot variant of another tenant's host must collapse to the
// same value here instead of slipping past as a distinct string.
func canonicalHosts(raw []string) ([]string, error) {
	hosts := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, r := range raw {
		h, err := canonicalHostname(r)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		hosts = append(hosts, h)
	}
	return hosts, nil
}

// notifyOnFailDefault is Render's own default enum value for spec.notifyOnFail
// — "defer to the workspace/member notification preference" (docs/render-
// artifacts/notify-on-fail.md). Empty is normalized to this both at create
// time and by view(), so an App created before this field existed reports the
// same "default" behavior it always had.
const notifyOnFailDefault = "default"

// normalizeSubdomainPolicy validates Render's exact renderSubdomainPolicy enum
// (enabled|disabled); empty means "enabled" (the platform subdomain is always
// present unless explicitly opted out). An unrecognized value is a named 400.
func normalizeSubdomainPolicy(v string) (string, error) {
	switch v {
	case "", appv1alpha1.SubdomainPolicyEnabled:
		return appv1alpha1.SubdomainPolicyEnabled, nil
	case appv1alpha1.SubdomainPolicyDisabled:
		return appv1alpha1.SubdomainPolicyDisabled, nil
	default:
		return "", fmt.Errorf("%w: renderSubdomainPolicy must be enabled or disabled", core.ErrBadRequest)
	}
}

// normalizeNotifyOnFail validates Render's exact notifyOnFail enum
// (default|notify|ignore, docs/render-artifacts/notify-on-fail.md); empty
// means "default". An unrecognized value is a named 400, matching
// normalizeType/SetPlan's enum-validation convention.
func normalizeNotifyOnFail(v string) (string, error) {
	switch v {
	case "":
		return notifyOnFailDefault, nil
	case notifyOnFailDefault, "notify", "ignore":
		return v, nil
	default:
		return "", fmt.Errorf("%w: notifyOnFail must be one of default|notify|ignore", core.ErrBadRequest)
	}
}

// normalizeType resolves the requested service type, tracking Render's
// serviceType vocabulary. Empty defaults to web_service; an unrecognized type is
// rejected.
func normalizeType(t string) (string, error) {
	switch t {
	case "":
		return appv1alpha1.TypeWebService, nil
	case appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob, appv1alpha1.TypeStaticSite:
		return t, nil
	default:
		return "", fmt.Errorf("%w: type must be one of web_service|private_service|background_worker|cron_job|static_site", core.ErrBadRequest)
	}
}

const (
	defaultMaxShutdownDelaySeconds int32 = 30
	minMaxShutdownDelaySeconds     int32 = 1
	maxMaxShutdownDelaySeconds     int32 = 300
)

// supportsMaxShutdownDelay reports whether the service owns a continuously
// running pod whose process receives SIGTERM during rollout/scale-down. Empty
// is the legacy spelling of web_service.
func supportsMaxShutdownDelay(serviceType string) bool {
	switch serviceType {
	case "", appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker:
		return true
	default:
		return false
	}
}

func validateMaxShutdownDelaySeconds(serviceType string, seconds *int32) error {
	if seconds == nil {
		return nil
	}
	if !supportsMaxShutdownDelay(serviceType) {
		return fmt.Errorf("%w: maxShutdownDelaySeconds is not applicable to a %s", core.ErrBadRequest, serviceType)
	}
	if *seconds < minMaxShutdownDelaySeconds || *seconds > maxMaxShutdownDelaySeconds {
		return fmt.Errorf("%w: maxShutdownDelaySeconds must be %d-%d", core.ErrBadRequest, minMaxShutdownDelaySeconds, maxMaxShutdownDelaySeconds)
	}
	return nil
}

func effectiveMaxShutdownDelaySeconds(serviceType string, seconds *int32) int32 {
	if !supportsMaxShutdownDelay(serviceType) {
		return 0
	}
	if seconds == nil {
		return defaultMaxShutdownDelaySeconds
	}
	return *seconds
}

func clonePtr[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// applyCreateToSpec copies the create-owned fields of want onto an existing
// spec, leaving fields owned by the operator or other features (EnvFromSecret,
// Suspended, IdleTTLSeconds, RestartedAt) untouched. MaintenanceMode is
// Blueprint-owned only when explicitly present; omission preserves an active
// maintenance window across ordinary deploys.
func applyCreateToSpec(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
	dst.Type = want.Type
	dst.Schedule = want.Schedule
	dst.Command = want.Command
	dst.Repo = want.Repo
	dst.Image = want.Image
	dst.Branch = want.Branch
	dst.Runtime = want.Runtime
	dst.BuildCommand = want.BuildCommand
	dst.StartCommand = want.StartCommand
	dst.Builder = want.Builder
	dst.RootDir = want.RootDir
	dst.BuildFilter = want.BuildFilter
	dst.DockerfilePath = want.DockerfilePath
	dst.DockerContext = want.DockerContext
	dst.Port = want.Port
	dst.Replicas = want.Replicas
	dst.Tier = want.Tier
	dst.HealthCheckPath = want.HealthCheckPath
	dst.MaxShutdownDelaySeconds = clonePtr(want.MaxShutdownDelaySeconds)
	dst.Env = want.Env
	dst.AutoDeploy = want.AutoDeploy
	dst.NotifyOnFail = want.NotifyOnFail
	dst.NotificationsToSend = want.NotificationsToSend
	dst.SubdomainPolicy = want.SubdomainPolicy
	dst.PreDeployCommand = want.PreDeployCommand
	if want.MaintenanceMode != nil {
		dst.MaintenanceMode = want.MaintenanceMode.DeepCopy()
	}
	dst.SetIPAllowListEntries(want.EffectiveIPAllowListEntries())
	dst.Expose = want.Expose
	dst.Host = want.Host
	dst.Hosts = want.Hosts
	dst.PublishPath = want.PublishPath
	dst.Routes = want.Routes
	dst.Headers = want.Headers
}

// normalizeTierOrPlan resolves a plan/tier string to a bex tier id, accepting
// either Render's plan spelling or the bex id. Empty => the catalog's default
// tier, matching the internal create API.
func normalizeTierOrPlan(v string) (string, error) {
	if v == "" {
		return tiers.Compute.Default().ID, nil
	}
	// render.yaml spells multi-word plans with spaces ("pro plus") while the
	// REST plan spelling uses underscores; the capability registry marks the
	// schema spelling translated, so accept both (w8/m22 round-trip fix).
	if t, ok := tiers.Compute.ByRenderPlan(strings.ReplaceAll(v, " ", "_")); ok {
		return t.ID, nil
	}
	if t, ok := tiers.Compute.ByRenderPlan(v); ok {
		return t.ID, nil
	}
	if _, ok := tiers.Compute.ByID(v); ok {
		return v, nil
	}
	return "", fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Compute.RenderPlans(), "|"))
}

// normalizeTierForType resolves a create's plan for its (already normalized)
// service type. A background worker is paid-only (w6/025, matching Render,
// whose worker picker has no free rung): an omitted plan lands on the cheapest
// paid tier instead of the catalog's free default, and an explicit free plan is
// refused. Every other type keeps normalizeTierOrPlan's defaulting.
func normalizeTierForType(svcType, plan string) (string, error) {
	if svcType != appv1alpha1.TypeBackgroundWorker {
		return normalizeTierOrPlan(plan)
	}
	if plan == "" {
		return defaultPaidTierID(), nil
	}
	tier, err := normalizeTierOrPlan(plan)
	if err != nil {
		return "", err
	}
	if !core.PaidPlan(tier) {
		return "", errWorkerFreePlan()
	}
	return tier, nil
}

// defaultPaidTierID is the cheapest paid rung of the compute ladder — the
// catalog is ordered smallest-first, so the first non-free id is the tier a
// plan-less background worker falls to. The catalog-default fallback is
// unreachable with the reviewed tiers.yaml, which always carries paid rungs.
func defaultPaidTierID() string {
	for _, tierID := range tiers.Compute.IDs() {
		if core.PaidPlan(tierID) {
			return tierID
		}
	}
	return tiers.Compute.Default().ID
}

// errWorkerFreePlan refuses the free plan on a background worker — workers are
// paid-only (w6/025). Built in one place so the create path and the plan-change
// verbs word the refusal identically, listing only the plans a worker may use.
func errWorkerFreePlan() error {
	paid := make([]string, 0, len(tiers.Compute.RenderPlans()))
	for _, plan := range tiers.Compute.RenderPlans() {
		if core.PaidPlan(plan) {
			paid = append(paid, plan)
		}
	}
	return fmt.Errorf("%w: a background_worker requires a paid plan; plan must be one of %s", core.ErrBadRequest, strings.Join(paid, "|"))
}

// redeploy bumps spec.restartedAt to force the operator to roll a new revision
// — for a repo-backed App this changes artifact/release identity and re-runs the
// build-from-git. Unauthorized on purpose: its only
// caller is the HMAC-verified git webhook, whose signature check is the
// authorization (there is no OpenFGA identity on a git-host callback).
//
// A store-managed App also gets a deploy row (trigger "new_commit"), stamped
// with the release generation this bump requests — the same discipline as
// deploys.Trigger. That both puts push-to-deploy in the deploy history and lets
// CreateDeploy's latest-pending slot replace any older queued trigger without
// preempting the release already executing. Without the row, the pending
// release would have no history record to adopt after the active release
// settles. commit is the pushed head
// commit from the webhook payload; empty falls back to the GitHub resolver.
// A CR-only App (no store id) keeps the plain bump.
func (s *Service) redeploy(ctx context.Context, name string, commit store.CommitInfo) (AppView, error) {
	a, err := s.GetApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	return s.redeployFetched(ctx, a, commit)
}

// redeployFetched applies a signed-webhook redeploy to an App the caller has
// already resolved. Besides avoiding a redundant API lookup, this preserves the
// App's tenant namespace: webhook matching lists cluster-wide when ADR043 is on,
// while an identity-less name lookup would otherwise fall back to the shared
// BEX_API_NAMESPACE and miss the matched App.
func (s *Service) redeployFetched(ctx context.Context, a *appv1alpha1.App, commit store.CommitInfo) (AppView, error) {
	appID := managedAppID(a)
	// A rollback of a repo-backed App installs the selected deploy's resolved
	// image as a temporary override. Push-to-deploy is a source build, so clear
	// that override in the projector's row before patching the CR. Leaving it in
	// place makes the operator prefer an old (eventually retained-away) tag over
	// every newly built artifact.
	if s.Store != nil && appID != "" && a.Spec.Repo != "" {
		if err := s.Store.SetAppImage(ctx, appID, ""); err != nil {
			return AppView{}, fmt.Errorf("clear rollback image override: %w", err)
		}
	}
	// Refresh the clone token so the push-triggered rebuild clones the private
	// repo with a token minted seconds ago.
	secretName, err := s.ensureCloneSecret(ctx, a)
	if err != nil {
		return AppView{}, err
	}
	pullSecretName, err := s.ensureExternalRegistryPullSecret(ctx, a)
	if err != nil {
		return AppView{}, err
	}
	previousGeneration := a.Generation
	// Untracked patch: this path opens its OWN deploy row below, with pushed-
	// commit provenance that a generic config_change row cannot carry.
	v, err := s.patchUntracked(ctx, a, func(a *appv1alpha1.App) {
		stampReleaseGeneration(a, previousGeneration+1)
		if secretName != "" {
			a.Spec.CloneSecret = secretName
		}
		a.Spec.ExternalRegistryPullSecret = pullSecretName
		if a.Spec.Repo != "" {
			a.Spec.Image = ""
			// A webhook's authenticated `after` is the immutable source of
			// provenance. Replace the previous one-shot pin on every source
			// deploy; never let the operator silently follow a moving branch.
			a.Spec.BuildCommit = commit.Hash
		}
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
	if err != nil {
		return AppView{}, err
	}
	if s.Store != nil {
		if appID != "" {
			if commit.Hash == "" {
				commit = s.resolveDeployCommit(ctx, s.AppWorkspace(ctx, a), a.Spec.Repo, a.Spec.Branch)
			}
			// max(after, before+1): a real API server has already incremented
			// metadata.generation on the patch; the fake client has not
			// (deploys.patchedGeneration, the same monotonic fallback).
			releaseGeneration := max(a.Generation, previousGeneration+1)
			// Row failure is logged, not returned: the CR bump above already
			// succeeded (the rebuild is rolling), and propagating a 5xx would
			// make the git host redeliver the push, re-bumping every matched App.
			// The reconciler's superseded-row cancel is the backstop for the
			// open row this row would have replaced.
			if _, err := s.Store.CreateDeploy(ctx, appID, store.TriggerNewCommit, a.Spec.Image, releaseGeneration, commit); err != nil {
				log.Printf("webhook: recording redeploy of %s: %v", a.Name, err)
			}
		}
	}
	s.notifyDeployStarted(ctx, a, a.Name)
	return v, nil
}

// notifyDeployStarted keeps identity-provider and SMTP I/O off the signed
// webhook request path. The App's owner label is authoritative because a git
// callback has no caller identity from which to resolve a workspace.
func (s *Service) notifyDeployStarted(ctx context.Context, a *appv1alpha1.App, name string) {
	if s.StartedNotifier == nil {
		return
	}
	tenantID := a.Labels[core.LabelTenant]
	if tenantID == "" {
		return
	}
	go s.StartedNotifier.NotifyDeployStarted(context.WithoutCancel(ctx), tenantID, name, a.Spec.NotificationsToSend)
}

// Restart requests a rolling restart (spec.restartedAt = now). The operator
// stamps the pod template and Kubernetes rolls the pods with no downtime.
func (s *Service) Restart(ctx context.Context, name string) (AppView, error) {
	return s.patch(ctx, core.RelCanOperate, name, func(a *appv1alpha1.App) {
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
		a.Spec.BuildCommit = "" // clear any commitId pin so the restart uses Branch HEAD
	})
}

// TriggerCronRun requests a one-off run of a cron_job now (Render's
// POST /cron-jobs/{id}/runs). Render guarantees one active cron execution: a
// manual trigger cancels the current run before starting its replacement, so
// the same spec patch carries both pieces of intent when status has an active
// run. The operator materializes the deterministic Job and owns cancellation.
// The pending run object can be returned synchronously because both its public
// crr- id and backing Job name are deterministic functions of runAt.
func (s *Service) TriggerCronRun(ctx context.Context, name string) (CronRunView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return CronRunView{}, err
	}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		return CronRunView{}, fmt.Errorf("%w: service %q is not a cron_job", core.ErrBadRequest, name)
	}
	if a.Spec.Suspended {
		return CronRunView{}, core.NewConflictError(
			CronErrorSuspended,
			fmt.Sprintf("cron job %q is suspended", name),
			map[string]any{"serviceId": name},
		)
	}
	if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		return CronRunView{}, err
	}
	now := s.Now().UTC()
	runAt := now.Format(time.RFC3339Nano)
	jobName := appv1alpha1.ManualCronRunJobName(a.Name, runAt)
	_, err = s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		if run, ok := pendingCronRun(a); ok {
			a.Spec.CancelRun = &appv1alpha1.CronRunCancellation{Name: run.Name, RequestedAt: runAt}
		}
		a.Spec.RunAt = runAt
	})
	if err != nil {
		return CronRunView{}, err
	}
	return CronRunView{ID: ids.Derive(ids.CronRun, jobName), Name: jobName, Status: cronRunPending}, nil
}

// ListCronRuns returns a cron_job's first-class run history, newest first. The
// status slice is already bounded by the operator; Page supplies Render-style
// cursor/limit semantics and an unknown cursor yields an empty tail.
func (s *Service) ListCronRuns(ctx context.Context, name, cursor string, limit int) ([]CronRunView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		return nil, core.ErrNotFound
	}
	runs := cronRunViews(a.Status.Runs)
	return core.Page(runs, cursor, core.PageLimitOrAbsent(limit), func(run CronRunView) string { return run.ID }), nil
}

// GetCronRun fetches one run by its stable derived id, scoped to its cron_job.
// A run from another service, an unknown id, or a non-cron service is the same
// 404 shape so ids cannot be used to probe across resources.
func (s *Service) GetCronRun(ctx context.Context, name, runID string) (CronRunView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return CronRunView{}, err
	}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		return CronRunView{}, cronRunNotFound(runID)
	}
	return cronRunByID(a.Status.Runs, runID)
}

// CancelCronRun records cancellation intent for one pending run. Terminal runs
// return 409; cancellation is never a successful no-op. The operator deletes
// the backing Job and owns the authoritative status write-back. The returned
// object reflects the accepted terminal intent immediately, while subsequent
// reads settle on the operator-written identical status/timestamp.
func (s *Service) CancelCronRun(ctx context.Context, name, runID string) (CronRunView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return CronRunView{}, err
	}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		return CronRunView{}, cronRunNotFound(runID)
	}
	run, err := cronRunByID(a.Status.Runs, runID)
	if err != nil {
		return CronRunView{}, err
	}
	return s.cancelCronRunFetched(ctx, a, run)
}

// CancelCurrentCronRun implements Render's current DELETE
// /cron-jobs/{id}/runs contract. It selects the sole pending run; the operator
// configures CronJob concurrency as Forbid and manual triggers cancel before
// replacement, so more than one active run is not a supported steady state.
func (s *Service) CancelCurrentCronRun(ctx context.Context, name string) (CronRunView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return CronRunView{}, err
	}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		return CronRunView{}, core.ErrNotFound
	}
	if run, ok := pendingCronRun(a); ok {
		return s.cancelCronRunFetched(ctx, a, run)
	}
	return CronRunView{}, fmt.Errorf("%w: cron job %q has no active run", core.ErrConflict, name)
}

// pendingCronRun selects the sole pending run (the operator forbids
// concurrency, so more than one is not a supported steady state) — the
// predicate CancelCurrentCronRun and the capability projection share.
func pendingCronRun(a *appv1alpha1.App) (CronRunView, bool) {
	for _, raw := range a.Status.Runs {
		if run := cronRunView(raw); run.Status == cronRunPending {
			return run, true
		}
	}
	return CronRunView{}, false
}

func (s *Service) cancelCronRunFetched(ctx context.Context, a *appv1alpha1.App, run CronRunView) (CronRunView, error) {
	if run.Status != cronRunPending {
		return CronRunView{}, core.NewConflictError(
			CronErrorRunTerminal,
			fmt.Sprintf("cron run %q is already %s", run.ID, run.Status),
			map[string]any{"runId": run.ID, "status": run.Status},
		)
	}
	now := s.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.CancelRun = &appv1alpha1.CronRunCancellation{Name: run.Name, RequestedAt: now}
	}); err != nil {
		return CronRunView{}, err
	}
	run.Status = cronRunCanceled
	run.FinishedAt = now
	return run, nil
}

func cronRunByID(runs []appv1alpha1.CronRun, runID string) (CronRunView, error) {
	if kind, ok := ids.KindOf(runID); !ok || kind != ids.CronRun {
		return CronRunView{}, cronRunNotFound(runID)
	}
	for _, raw := range runs {
		run := cronRunView(raw)
		if run.ID == runID {
			return run, nil
		}
	}
	return CronRunView{}, cronRunNotFound(runID)
}

func cronRunNotFound(runID string) error {
	return core.NewNotFoundError(
		CronErrorRunNotFound,
		fmt.Sprintf("cron run %q was not found", runID),
		map[string]any{"runId": runID},
	)
}

// Suspend parks the App (spec.suspended = true): scaled to 0, host/certs kept.
func (s *Service) Suspend(ctx context.Context, name string) (AppView, error) {
	return s.setSuspended(ctx, name, true)
}

// Resume brings a suspended App back (spec.suspended = false); the operator
// restores spec.replicas.
func (s *Service) Resume(ctx context.Context, name string) (AppView, error) {
	return s.setSuspended(ctx, name, false)
}

// SetPlan changes the App's instance size (Render's `plan`, spelled per
// lego/types/tiers). Unknown plans are rejected before any write — the
// caller maps core.ErrInvalid to 400/a GraphQL error, listing the valid
// plans. A plan change resizes the pod (new requests==limits), which is a
// Deployment rollout — the same restart-shaped cost as Render's own plan
// changes.
func (s *Service) SetPlan(ctx context.Context, name, plan string) (AppView, error) {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	t, ok := tiers.Compute.ByRenderPlan(plan)
	if !ok {
		return AppView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Compute.RenderPlans(), "|"))
	}
	tier := t.ID
	if a.Spec.Type == appv1alpha1.TypeBackgroundWorker && !core.PaidPlan(tier) {
		return AppView{}, errWorkerFreePlan()
	}
	if err := s.RequirePlanBilling(ctx, a.Labels[core.LabelTenant], tier); err != nil {
		return AppView{}, err
	}
	if tier == "free" && a.Spec.MaintenanceMode != nil && a.Spec.MaintenanceMode.Enabled {
		return AppView{}, fmt.Errorf("%w: disable maintenance mode before changing to the free plan", core.ErrBadRequest)
	}
	if err := planDowngradeError(a, tier, plan); err != nil {
		return AppView{}, err
	}
	fromPlan := ""
	if ft, ok := tiers.Compute.ByID(a.Spec.Tier); ok {
		fromPlan = ft.RenderPlan
	}
	result, err := s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppTier(ctx, id, tier) },
		func(a *appv1alpha1.App) { a.Spec.Tier = tier })
	if err != nil {
		return AppView{}, err
	}
	// Same no-op guard as Scale: a plan change to the current plan emits no
	// plan_changed audit row or webhook (the allowed-write audit is deferred here).
	if fromPlan != plan {
		s.RecordPlanChanged(ctx, a, fromPlan, plan)
	}
	return result, nil
}

// PreviewSetPlan returns what SetPlan would produce — the same validation and
// in-memory spec update — without writing to Kubernetes or the store (w2/m29
// dry-run). Requires can_view on the named service (no audit event, no write).
func (s *Service) PreviewSetPlan(ctx context.Context, name, plan string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return AppView{}, err
	}
	t, ok := tiers.Compute.ByRenderPlan(plan)
	if !ok {
		return AppView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Compute.RenderPlans(), "|"))
	}
	if a.Spec.Type == appv1alpha1.TypeBackgroundWorker && !core.PaidPlan(t.ID) {
		return AppView{}, errWorkerFreePlan()
	}
	if t.ID == "free" && a.Spec.MaintenanceMode != nil && a.Spec.MaintenanceMode.Enabled {
		return AppView{}, fmt.Errorf("%w: disable maintenance mode before changing to the free plan", core.ErrBadRequest)
	}
	if err := planDowngradeError(a, t.ID, plan); err != nil {
		return AppView{}, err
	}
	preview := a.DeepCopy()
	preview.Spec.Tier = t.ID
	return s.view(preview), nil
}

// errInstanceCap rejects an instance count above the plan's ceiling (w6/m118),
// naming the plan and the limit — the shape errNoPublicIngress established for
// the sibling plan-type refusal, not a bare 400. plan is the public Render
// spelling, cap the plan's maximum, requested the count the caller asked for.
func errInstanceCap(plan string, cap, requested int32) error {
	return fmt.Errorf("%w: the %s plan is limited to %d instance(s) and cannot scale to %d", core.ErrBadRequest, plan, cap, requested)
}

// checkInstanceCap returns errInstanceCap when a requested instance count
// exceeds the plan's ceiling, and nil for no-cap tiers (every paid plan, and an
// untiered/bare-CR App). The limit lives in the reviewed tier catalog, so every
// write path that can set an instance count — Scale, create, autoscaling — reads
// the same bound rather than testing for the literal "free" (see t003).
func checkInstanceCap(tier string, requested int32) error {
	cap, ok := tiers.Compute.InstanceCap(tier)
	if !ok || requested <= cap {
		return nil
	}
	plan := tier
	if t, found := tiers.Compute.ByID(tier); found {
		plan = t.RenderPlan
	}
	return errInstanceCap(plan, cap, requested)
}

// planDowngradeError refuses a plan change that would strand a service above the
// target plan's instance cap (w6/m118 t003) — the same "reduce first" shape as
// the maintenance-mode guard, telling the caller to scale down rather than
// silently shrinking a running service (the mutation-succeeds-and-does-nothing
// shape). It considers the fixed replica count and, when autoscaling is on, its
// max — either can exceed the cap. plan is the target's public Render spelling.
func planDowngradeError(a *appv1alpha1.App, tier, plan string) error {
	cap, ok := tiers.Compute.InstanceCap(tier)
	if !ok {
		return nil
	}
	current := a.Spec.Replicas
	if current < 1 {
		current = 1
	}
	if as := a.Spec.Autoscaling; as != nil && as.Enabled && as.MaxReplicas > current {
		current = as.MaxReplicas
	}
	if current > cap {
		return fmt.Errorf("%w: scale to %d instance(s) or fewer before changing to the %s plan (currently %d)", core.ErrBadRequest, cap, plan, current)
	}
	return nil
}

// Scale sets the App's desired running instance count (Render's manual-scaling
// verb; the REST body field is numInstances). It writes spec.replicas the same
// row-first way as Suspend/SetPlan — the projector owns the field. The count is
// what the operator runs when the App is active: suspend still wins (it forces
// 0 in the operator's effectiveReplicas without rewriting spec.replicas), so
// scaling a suspended App takes visible effect on resume. This is the
// degenerate, human-driven case of m3 (bin-pack/autoscale); the field
// semantics settled here must stay compatible with it.
//
// The count must be 1..store.MaxReplicas (the shared upper bound the create
// path also enforces). 0 is rejected, not scale-to-zero: today the operator
// maps spec.replicas 0 to 1 (the default), so 0 is ambiguous — scale-to-zero
// (m4) owns redefining that, and will keep this 1-based verb valid.
func (s *Service) Scale(ctx context.Context, name string, replicas int32) (AppView, error) {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		return AppView{}, err
	}
	if replicas < 1 || replicas > store.MaxReplicas {
		return AppView{}, fmt.Errorf("%w: numInstances must be 1-%d", core.ErrBadRequest, store.MaxReplicas)
	}
	if err := checkInstanceCap(a.Spec.Tier, replicas); err != nil {
		return AppView{}, err
	}
	fromReplicas := a.Spec.Replicas
	result, err := s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppReplicas(ctx, id, replicas) },
		func(a *appv1alpha1.App) { a.Spec.Replicas = replicas })
	if err != nil {
		return AppView{}, err
	}
	// Only record the effect when the count actually changed. A scale to the
	// current replica count is a no-op, and Render's instance_count_changed
	// semantics are "changed" — recording it would emit a spurious audit row and
	// webhook delivery. Because the allowed-write audit is deferred to this call,
	// skipping it means a no-op scale produces no event at all, closing a cheap
	// repeatable amplifier of the shared outbound-webhook queue (scan finding #3).
	if fromReplicas != replicas {
		s.RecordScaleEffect(ctx, a, fromReplicas, replicas)
	}
	return result, nil
}

// MaxIdleTTLSeconds bounds the idle-timeout a caller may set (7 days). Free-tier
// Apps auto-hibernate after this many idle seconds; 0 (or unset) means the
// platform default idle window (15 min). A generous ceiling — the point is
// "sleep quickly to save money", not an indefinite keep-alive that would defeat
// the free tier.
const MaxIdleTTLSeconds int32 = 7 * 24 * 60 * 60

// SetIdleTTL sets how long the App may idle before it auto-hibernates
// (spec.idleTTLSeconds; "sleep = free", w1/m4). 0 (or unset) selects the
// platform default idle window (15 min); a positive value overrides it. The
// literal 0 is stored unchanged — the operator resolves it — so writing 0 never
// rewrites the field to the default. A bex extension with no Render counterpart
// — Render's free spin-down window is fixed — so it writes spec.idleTTLSeconds
// the same row-first way as
// Scale (the projector owns the field). Only free web services ever sleep, but
// the value is stored for every type and tier for wire compatibility; it takes
// effect only if the App is both a web service and free. The dashboard gates
// the control on that same policy and the operator is the final authority.
func (s *Service) SetIdleTTL(ctx context.Context, name string, seconds int32) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if seconds < 0 || seconds > MaxIdleTTLSeconds {
		return AppView{}, fmt.Errorf("%w: idleTTLSeconds must be 0-%d", core.ErrBadRequest, MaxIdleTTLSeconds)
	}
	return s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppIdleTTL(ctx, id, seconds) },
		func(a *appv1alpha1.App) { a.Spec.IdleTTLSeconds = seconds })
}

// SetRootDir changes the subdirectory of Repo a build-from-git App builds
// from (spec.rootDir, Render's Root Directory) and bumps spec.restartedAt so
// the change takes effect as a fresh build scoped to the new subdirectory —
// otherwise a rootDir-only change would sit unbuilt until the next unrelated
// push. Not projection-owned (mirrors Builder): the control-plane row never
// carries it, so this is a direct CR patch like Restart, not
// writeThroughStore. Rejected for an image-backed App (nothing to build).
// Release identity (codex round-9 #8): rootDir selects which repository
// subtree BuildKit executes, so it is a can_create (developer) input like
// Builder — not an operational can_operate one a contributor holds.
func (s *Service) SetRootDir(ctx context.Context, name, rootDir string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	if err := requireRepoBacked(a, name, "root directory only applies to build-from-git"); err != nil {
		return AppView{}, err
	}
	if !store.ValidRootDir(rootDir) {
		return AppView{}, fmt.Errorf("%w: rootDirectory must be a relative path with no '..' components", core.ErrBadRequest)
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.RootDir = rootDir
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
}

// SetDockerfilePath changes the Dockerfile used by a repo-backed Docker build.
// The path is relative to spec.rootDir (or the repository root when rootDir is
// empty), matching Render's Dockerfile Path setting. Empty restores the default
// "Dockerfile" lookup. Like SetRootDir, changing it starts a fresh build by
// bumping restartedAt. Explicit native/buildpack runtimes are rejected; legacy
// auto/dockerfile Apps remain supported because they can still be Dockerfile
// builds without carrying spec.runtime="docker". Release identity (codex
// round-9 #8): the Dockerfile is the exact instruction set BuildKit executes,
// so the verb requires can_create (developer) like Builder and SetRootDir.
func (s *Service) SetDockerfilePath(ctx context.Context, name, dockerfilePath string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	if err := requireRepoBacked(a, name, "dockerfile path only applies to Dockerfile builds"); err != nil {
		return AppView{}, err
	}
	runtime := strings.ToLower(strings.TrimSpace(a.Spec.Runtime))
	builder := strings.ToLower(strings.TrimSpace(a.Spec.Builder))
	if (runtime != "" && runtime != "docker") || builder == "native" || builder == "buildpack" {
		return AppView{}, fmt.Errorf("%w: dockerfile path only applies to a Dockerfile-built service", core.ErrBadRequest)
	}
	dockerfilePath = strings.TrimSpace(dockerfilePath)
	if dockerfilePath != "" && !store.ValidRootDir(dockerfilePath) {
		return AppView{}, fmt.Errorf("%w: dockerfilePath must be a relative path with no '..' components", core.ErrBadRequest)
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.DockerfilePath = dockerfilePath
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
}

// SetBuildFilter replaces the git-push auto-deploy filter (spec.buildFilter,
// Render's Build Filters): the repository-root-relative glob patterns deciding
// whether a push redeploys this App. Passing all-empty (or all-whitespace)
// lists clears the filter, so every matching push deploys again. A direct CR
// patch, not projection-owned (mirrors Builder/RootDir), and — unlike SetRootDir
// — no restartedAt bump: the filter changes only which FUTURE pushes redeploy, it
// does not itself rebuild the current revision. Rejected for an image-backed App
// (no repo, so no push to filter).
func (s *Service) SetBuildFilter(ctx context.Context, name string, filter *BuildFilterView) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if err := requireRepoBacked(a, name, "build filters only apply to build-from-git"); err != nil {
		return AppView{}, err
	}
	bf, err := normalizeBuildFilter(filter)
	if err != nil {
		return AppView{}, err
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.BuildFilter = bf
	})
}

// SetPreDeployCommand changes the pre-deploy command (spec.preDeployCommand,
// Render's Pre-Deploy Command) — a command run to completion against the new
// revision's image before it serves traffic (typically a migration). An empty
// command clears the step. Rejected for cron_job (its own Command already runs
// the image) and static_site (no running container) — the operator ignores the
// field there, so accepting it would be a silent no-op. Direct CR patch, not
// projection-owned (mirrors Builder/RootDir); the spec change bumps generation,
// so the command runs on the resulting rollout with no explicit restart.
func (s *Service) SetPreDeployCommand(ctx context.Context, name, command string) (AppView, error) {
	// SECURITY (codex round-5 F2): a pre-deploy command is attacker-chosen code
	// the service executes with its runtime identity and secret projections, so
	// it is create-like for exactly the reason SetCommands below is — gate on
	// can_create (developer and up), not can_operate.
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	if a.Spec.Type == appv1alpha1.TypeCronJob || a.Spec.Type == appv1alpha1.TypeStaticSite {
		return AppView{}, fmt.Errorf("%w: a pre-deploy command does not apply to a %s", core.ErrBadRequest, a.Spec.Type)
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.PreDeployCommand = strings.TrimSpace(command)
	})
}

// SetCommands applies Render's runtime-specific build/start command PATCH.
// The official CLI sends these inside serviceDetails.envSpecificDetails; cron
// jobs call their start command `command`, while the other runtime-backed
// service types persist it as spec.startCommand.
func (s *Service) SetCommands(ctx context.Context, name string, buildCommand, startCommand *string) (AppView, error) {
	// SECURITY (codex #1): build/start commands are attacker-chosen code the
	// service executes with its runtime identity, so this is create-like, not
	// lifecycle — gate on can_create (developer and up), not can_operate.
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	if a.Spec.Type == appv1alpha1.TypeStaticSite && startCommand != nil {
		return AppView{}, fmt.Errorf("%w: start command is not applicable to a static_site", core.ErrBadRequest)
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		if buildCommand != nil {
			a.Spec.BuildCommand = strings.TrimSpace(*buildCommand)
		}
		if startCommand != nil {
			if a.Spec.Type == appv1alpha1.TypeCronJob {
				a.Spec.Command = strings.TrimSpace(*startCommand)
			} else {
				a.Spec.StartCommand = strings.TrimSpace(*startCommand)
			}
		}
	})
}

// sourcePatch is SetSourceAndRegistryCredential's field set — Render's PATCH
// source object plus its context-sensitive registryCredentialId. Every field
// follows the same pointer convention: nil means omitted (preserve current).
type sourcePatch struct {
	Repo                 *string
	Image                *string
	Branch               *string
	RegistryCredentialID *string
	ImageOwnerID         *string
}

// SetRegistryCredential sets, changes, or clears an image-backed service's or
// Dockerfile build's explicit registry credential. An empty id is the
// explicit-clear operation.
// All adapters call this same verb so membership, host validation, Secret
// materialization, and error classification cannot drift.
func (s *Service) SetRegistryCredential(ctx context.Context, name, credentialID string) (AppView, error) {
	return s.SetSourceAndRegistryCredential(ctx, name, sourcePatch{RegistryCredentialID: &credentialID})
}

// SetSourceAndRegistryCredential applies Render's PATCH source object and its
// context-sensitive registryCredentialId together. The combined verb matters
// for `image:{imagePath,registryCredentialId}`: the credential is validated
// against the proposed image host before either source field reaches the App.
// A nil credential pointer preserves the current binding; pointer-to-empty
// clears it. Switching to a repo clears an old image credential unless the
// request explicitly supplies one for a Dockerfile build.
func (s *Service) SetSourceAndRegistryCredential(ctx context.Context, name string, patch sourcePatch) (AppView, error) {
	// SECURITY (codex #1): repointing a service at a new repo or image chooses the
	// executable the operator runs with the service identity — create-like, so gate
	// on can_create (developer and up), not the lifecycle-oriented can_operate.
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	if patch.ImageOwnerID != nil {
		ownerID := strings.TrimSpace(*patch.ImageOwnerID)
		if ownerID != "" && ownerID != a.Labels[core.LabelTenant] {
			return AppView{}, fmt.Errorf("%w: image.ownerId does not match the service owner", core.ErrBadRequest)
		}
	}
	next, err := resolveSourcePatch(a, patch)
	if err != nil {
		return AppView{}, err
	}
	if next.repo == a.Spec.Repo &&
		next.image == a.Spec.Image &&
		next.branch == a.Spec.Branch &&
		sameStringPtr(next.registryCredentialID, a.Spec.RegistryCredentialID) {
		return s.view(a), nil
	}
	probe := a.DeepCopy()
	probe.Spec.Repo = next.repo
	probe.Spec.Image = next.image
	probe.Spec.Branch = next.branch
	probe.Spec.RegistryCredentialID = clonePtr(next.registryCredentialID)
	if err := s.validateExternalRegistryCredential(ctx, probe); err != nil {
		return AppView{}, err
	}
	if next.repo != "" && next.repo != a.Spec.Repo && s.GitHub != nil {
		if err := s.GitHub.ValidateRepo(ctx, s.AppWorkspace(ctx, a), next.repo); err != nil {
			return AppView{}, err
		}
	}
	// Stamp the pending generation before writing the projector-owned source
	// row. If the following CR patch loses a race or the API becomes unavailable,
	// a later projector resync still cannot deploy the new source accidentally.
	markerBase := client.MergeFrom(a.DeepCopy())
	metav1.SetMetaDataAnnotation(
		&a.ObjectMeta,
		appv1alpha1.AnnotationPendingSourceGeneration,
		strconv.FormatInt(a.Generation+1, 10),
	)
	if err := s.Client.Patch(ctx, a, markerBase); err != nil {
		return AppView{}, err
	}
	if s.Store != nil {
		if id := managedAppID(a); id != "" {
			if err := s.Store.SetAppSource(ctx, id, next.repo, next.image, next.branch, clonePtr(next.registryCredentialID)); err != nil {
				return AppView{}, fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	previousBranch := a.Spec.Branch
	// SECURITY (codex #5): when the repository origin changes, the retained GitHub
	// clone token is scoped to the OLD origin. Static publisher and kpack would send
	// it to the new (possibly attacker-controlled) origin. Atomically clear the old
	// clone Secret and spec.cloneSecret; a replacement (if the new repo is a
	// connected private one) is minted on the next deploy.
	repoChanged := next.repo != a.Spec.Repo
	oldCloneSecret := a.Spec.CloneSecret
	// Render saves Update Source without deploying it. Deliberately bypass
	// patchFetched's rollout tracker; the pending marker above tells the operator
	// to keep the active artifact until a later deploy verb stamps
	// AnnotationReleaseGeneration at this generation or newer.
	updated, err := s.patchUntracked(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Repo = next.repo
		a.Spec.Image = next.image
		a.Spec.Branch = next.branch
		a.Spec.RegistryCredentialID = clonePtr(next.registryCredentialID)
		if repoChanged {
			a.Spec.CloneSecret = "" // clear stale token; reminted on next deploy
		}
	})
	if err != nil {
		return AppView{}, err
	}
	if repoChanged && oldCloneSecret != "" {
		if err := s.deleteCloneSecret(ctx, a.Namespace, a.Name); err != nil {
			log.Printf("apps: clear stale clone secret for %s after repo change: %v", a.Name, err)
		}
	}
	if previousBranch != "" && previousBranch != next.branch {
		s.recordBranchChangedFact(ctx, a, previousBranch, next.branch, updated.UpdatedAt)
	}
	return updated, nil
}

func sameStringPtr(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// sourceFields is the validated source a sourcePatch resolves to — the four
// spec fields that must move together so a half-applied switch is impossible.
type sourceFields struct {
	repo                 string
	image                string
	branch               string
	registryCredentialID *string
}

// resolveSourcePatch folds a sourcePatch onto the App's current source,
// validating each supplied field. Repo and image are mutually exclusive, so
// setting either clears the other.
func resolveSourcePatch(a *appv1alpha1.App, patch sourcePatch) (sourceFields, error) {
	next := sourceFields{
		repo:                 a.Spec.Repo,
		image:                a.Spec.Image,
		branch:               a.Spec.Branch,
		registryCredentialID: clonePtr(a.Spec.RegistryCredentialID),
	}
	if patch.Repo != nil {
		next.repo = strings.TrimSpace(*patch.Repo)
		if next.repo == "" || !store.ValidRepo(next.repo) {
			return sourceFields{}, fmt.Errorf("%w: invalid repository URL", core.ErrBadRequest)
		}
		next.image = ""
	}
	if patch.Image != nil {
		// The same rule the create path enforces: a static site is built from
		// its repo and published to the object-store origin (ADR029), so
		// repointing one at a prebuilt image would strand it.
		if a.Spec.Type == appv1alpha1.TypeStaticSite {
			return sourceFields{}, fmt.Errorf("%w: a static_site builds from a Git repo; a prebuilt image is not supported", core.ErrBadRequest)
		}
		next.image = strings.TrimSpace(*patch.Image)
		if next.image == "" {
			return sourceFields{}, fmt.Errorf("%w: image path is required", core.ErrBadRequest)
		}
		if !store.ValidImage(next.image) {
			return sourceFields{}, fmt.Errorf("%w: image must be an OCI reference (no whitespace or shell metacharacters)", core.ErrBadRequest)
		}
		next.repo = ""
	}
	if patch.Branch != nil {
		next.branch = strings.TrimSpace(*patch.Branch)
		// An explicit empty means "back to the default" — the setter family's
		// convention (empty clears to the default, cf. SetCommands); the
		// fallback below applies it. Only a non-empty ref is validated.
		if next.branch != "" && !store.ValidGitRef(next.branch) {
			return sourceFields{}, fmt.Errorf("%w: invalid branch", core.ErrBadRequest)
		}
	}
	if next.branch == "" {
		next.branch = appv1alpha1.DefaultBranch
	}
	if patch.RegistryCredentialID != nil {
		value := strings.TrimSpace(*patch.RegistryCredentialID)
		next.registryCredentialID = &value
	} else if patch.Repo != nil {
		// A source-kind switch stops using the former image credential. This is
		// omission-as-preserve for ordinary PATCHes, but an explicit repo switch
		// cannot retain image-only authentication intent.
		next.registryCredentialID = nil
	}
	return next, nil
}

// recordBranchChangedFact appends the branch_changed fact after a source
// patch moved the tracked branch, stamped with the patch's own UpdatedAt so
// the feed matches the write (mirrors GitWebhook.recordBranchDeletedFact).
func (s *Service) recordBranchChangedFact(ctx context.Context, a *appv1alpha1.App, from, to, updatedAt string) {
	if s.EventFacts == nil {
		return
	}
	appID := managedAppID(a)
	if appID == "" {
		return
	}
	at := s.Now().UTC()
	if updatedAt != "" {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, updatedAt); parseErr == nil {
			at = parsed
		}
	}
	fact := store.ServiceEventFact{
		SourceKey: fmt.Sprintf("branch:%s:%d", appID, at.UnixNano()),
		AppID:     appID, Type: store.EventFactBranchChanged, At: at,
		BranchFrom: from, BranchTo: to,
	}
	if _, err := s.EventFacts.InsertServiceEventFact(ctx, fact); err != nil {
		log.Printf("events: record branch change for %s: %v", appID, err)
	}
}

// SetCronJob changes a cron_job's schedule and/or command (spec.schedule +
// spec.command). Rejected for a non-cron service. Both args are optional — nil
// means "keep existing," which lets REST PATCH update one field without
// re-reading the other. A non-nil schedule must be a non-empty 5-field crontab;
// a non-nil command of "" clears the entrypoint override. Direct CR patch, not
// projection-owned (mirrors Builder/RootDir).
func (s *Service) SetCronJob(ctx context.Context, name string, schedule, command *string) (AppView, error) {
	// SECURITY (codex round-5 F2): the command IS the code the cron runs, so
	// supplying one (including clearing it, which changes what executes) is
	// create-like. A schedule-only change is genuine lifecycle — when the cron
	// runs, not what it runs — and stays available to contributors.
	a, err := s.AuthorizeApp(ctx, core.LifecycleOrCreate(command != nil), name)
	if err != nil {
		return AppView{}, err
	}
	if schedule != nil {
		trimmed := strings.TrimSpace(*schedule)
		if trimmed == "" {
			return AppView{}, fmt.Errorf("%w: schedule is required", core.ErrBadRequest)
		}
		if !validCronSchedule(trimmed) {
			return AppView{}, fmt.Errorf("%w: schedule must be a valid 5-field cron expression (e.g. '0 * * * *')", core.ErrBadRequest)
		}
	}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		return AppView{}, fmt.Errorf("%w: service %q is not a cron_job", core.ErrBadRequest, name)
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		if schedule != nil {
			a.Spec.Schedule = strings.TrimSpace(*schedule)
		}
		if command != nil {
			a.Spec.Command = strings.TrimSpace(*command)
		}
	})
}

// validCronSchedule reports whether s is a valid standard 5-field cron
// expression. It first requires exactly 5 whitespace-separated fields (bex's
// contract, matching Render — descriptors like @daily are not accepted), then
// parses the fields with the SAME parser the Kubernetes CronJob controller uses
// (github.com/robfig/cron/v3, cron.ParseStandard). Parsing here is the whole
// point: a field-count-only check let malformed-but-5-field schedules like
// "99 99 * * *" through to convergence, where the apiserver rejected the
// CronJob (minute/hour out of range) and flipped the App to Failed with no
// caller feedback. Validating with the operator's own parser guarantees "if
// bex accepts it, the CronJob accepts it."
func validCronSchedule(s string) bool {
	if len(strings.Fields(s)) != 5 {
		return false
	}
	_, err := cron.ParseStandard(s)
	return err == nil
}

// SetHealthCheckPath changes spec.healthCheckPath — what the operator wires
// into the container's startup and readiness probes (w1/m23/t001). A direct CR
// patch, not projection-owned (mirrors Builder/RootDir). Rejected for service
// types that have no HTTP port (cron_job, background_worker) since those never
// serve a health endpoint.
//
// An empty path CLEARS the field, selecting the TCP probe — it does not reset
// to "/" (w7/m80). That coercion used to make the field one-way: once any path
// was set, no surface could express "unset" again, so no caller could reach the
// TCP mode that is now the platform default and Render's. Clearing is also the
// documented migration for a service created before m80, which still carries a
// "/" the CRD default persisted at write time.
func (s *Service) SetHealthCheckPath(ctx context.Context, name string, path string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if a.Spec.Type == appv1alpha1.TypeCronJob || a.Spec.Type == appv1alpha1.TypeBackgroundWorker {
		return AppView{}, fmt.Errorf("%w: health check path is not applicable to a %s", core.ErrBadRequest, a.Spec.Type)
	}
	trimmed := strings.TrimSpace(path)
	if trimmed != "" && !strings.HasPrefix(trimmed, "/") {
		return AppView{}, fmt.Errorf("%w: health check path must start with /", core.ErrBadRequest)
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.HealthCheckPath = trimmed
	})
}

// SetMaxShutdownDelay changes the maximum time Kubernetes gives a service's
// process to exit after SIGTERM before SIGKILL. It is a direct CR patch because
// the control-plane row does not own this field. Kubernetes rolls the Deployment
// when the pod template's terminationGracePeriodSeconds changes.
func (s *Service) SetMaxShutdownDelay(ctx context.Context, name string, seconds int32) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if err := validateMaxShutdownDelaySeconds(a.Spec.Type, &seconds); err != nil {
		return AppView{}, err
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.MaxShutdownDelaySeconds = clonePtr(&seconds)
	})
}

// SetAutoDeploy flips whether a signed git push to the tracked branch redeploys
// this App (spec.autoDeploy, Render's Auto-Deploy toggle). A direct CR patch,
// not projection-owned (mirrors Builder/RootDir), and no restartedAt bump —
// flipping the toggle changes future push behavior, it does not itself redeploy.
func (s *Service) SetAutoDeploy(ctx context.Context, name string, enabled bool) (AppView, error) {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	result, err := s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.AutoDeploy = enabled
	})
	if err != nil {
		return AppView{}, err
	}
	s.RecordAutoDeployChanged(ctx, a, enabled)
	return result, nil
}

// SetNotifyOnFail changes an App's deploy-failure notification override
// (spec.notifyOnFail, Render's exact name/enum — docs/render-artifacts/
// notify-on-fail.md). A direct CR patch, not projection-owned (mirrors
// AutoDeploy/DisplayName). Unrecognized value ⇒ core.ErrBadRequest.
func (s *Service) SetNotifyOnFail(ctx context.Context, name, value string) (AppView, error) {
	normalized, err := normalizeNotifyOnFail(value)
	if err != nil {
		return AppView{}, err
	}
	return s.patch(ctx, core.RelCanOperate, name, func(a *appv1alpha1.App) {
		a.Spec.NotifyOnFail = normalized
		a.Spec.NotificationsToSend = ""
	})
}

// SetNotificationsToSend changes Render's authoritative service notification
// override. notifyOnFail remains a compatibility projection.
func (s *Service) SetNotificationsToSend(ctx context.Context, name, value string) (AppView, error) {
	normalized, err := normalizeNotificationsToSend(value)
	if err != nil {
		return AppView{}, err
	}
	return s.patch(ctx, core.RelCanOperate, name, func(a *appv1alpha1.App) {
		a.Spec.NotificationsToSend = normalized
		a.Spec.NotifyOnFail = appv1alpha1.NotifyOnFailForNotificationsToSend(normalized)
	})
}

// SetSubdomainPolicy changes Render's renderSubdomainPolicy (enabled|disabled):
// whether the platform host <slug>.<domain> appears in the Ingress and status
// URL. Cannot be set to "disabled" without at least one custom host already
// configured on the App — that would leave the service silently unreachable.
func (s *Service) SetSubdomainPolicy(ctx context.Context, name, policy string) (AppView, error) {
	normalized, err := normalizeSubdomainPolicy(policy)
	if err != nil {
		return AppView{}, err
	}
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	// renderSubdomainPolicy toggles the platform subdomain, which only a web
	// service or static site HAS. A private service, background worker or cron
	// job has no ingress, so there is no subdomain to enable or disable — refuse
	// with exactly that reason, not the misleading custom-domain guard below
	// (which then sends the caller into an add-domain call that itself 400s
	// because the type has no ingress) and not a silent success (w6/m130).
	if !a.Spec.PubliclyRoutable() {
		return AppView{}, fmt.Errorf("%w: renderSubdomainPolicy applies only to web services and static sites; a %s has no platform subdomain to toggle", core.ErrBadRequest, effectiveType(a.Spec.Type))
	}
	if normalized == appv1alpha1.SubdomainPolicyDisabled {
		if a.Spec.Host == "" && len(a.Spec.Hosts) == 0 {
			return AppView{}, fmt.Errorf("%w: renderSubdomainPolicy cannot be disabled without at least one custom domain", core.ErrBadRequest)
		}
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.SubdomainPolicy = normalized
	})
}

// SetIPAllowList replaces the App's inbound allowlist. CIDRs are validated;
// descriptions persist as metadata and never influence enforcement. An empty
// slice clears both the structured and legacy fields (Render's default — open
// to all source IPs). Only meaningful for web_service and static_site.
func (s *Service) SetIPAllowList(ctx context.Context, name string, entries []core.IPAllowListEntry) (AppView, error) {
	if err := core.ValidateAllowList(entries); err != nil {
		return AppView{}, err
	}
	return s.patch(ctx, core.RelCanOperate, name, func(a *appv1alpha1.App) {
		a.Spec.SetIPAllowListEntries(core.AllowListToSpec(entries))
	})
}

// ConfigureMaintenanceMode is the explicit atomic-object spelling used by the
// REST and Blueprint adapters. SetMaintenanceMode remains the canonical Core
// verb name recorded by audit, activity, and webhook projections.
func (s *Service) ConfigureMaintenanceMode(ctx context.Context, name string, in MaintenanceModeView) (AppView, error) {
	return s.SetMaintenanceMode(ctx, name, in)
}

// SetMaintenanceMode validates and writes Render's two-key object in one
// patch. Field-level audit/activity/webhook effects are emitted only after the
// patch succeeds, URI first and then enabled when both changed.
func (s *Service) SetMaintenanceMode(ctx context.Context, name string, in MaintenanceModeView) (AppView, error) {
	return s.setMaintenanceMode(ctx, name, in)
}

// setMaintenanceMode is deliberately unexported so callerVerb walks through
// it to SetMaintenanceMode, including for denied writes entering through the
// Configure alias above.
func (s *Service) setMaintenanceMode(ctx context.Context, name string, in MaintenanceModeView) (AppView, error) {
	auditCtx := core.WithDeferredAllowedWriteAudit(ctx)
	a, err := s.AuthorizeApp(auditCtx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	in.URI = strings.TrimSpace(in.URI)
	if err := s.validateMaintenanceMode(ctx, a, in); err != nil {
		return AppView{}, err
	}
	current := maintenanceModeView(a.Spec.MaintenanceMode)
	if current == in {
		return s.view(a), nil
	}
	uriChanged := current.URI != in.URI
	var enabledChanged *bool
	if current.Enabled != in.Enabled {
		enabled := in.Enabled
		enabledChanged = &enabled
	}
	result, err := s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: in.Enabled, URI: in.URI}
	})
	if err != nil {
		return AppView{}, err
	}
	s.RecordMaintenanceModeEffects(ctx, a, uriChanged, enabledChanged)
	return result, nil
}

// SetDisplayName changes the human-facing label for an App without changing
// its immutable Kubernetes object name or any identity derived from that name
// (including platform hostnames and TLS secret names). Whitespace at the edges
// is presentation noise and is trimmed; an empty value clears the label so
// clients fall back to the immutable Name.
//
// The CR remains the writer of truth — displayName is not projector-owned, so
// unlike suspend/scale/plan a resync will never revert the patch. The row write
// alongside it is a read projection (w6/m101): the workspace-wide event feed
// joins apps at dispatch time and has no CR to read, so without the mirror
// every webhook and push notification for a renamed service reports the
// immutable creation-time name.
func (s *Service) SetDisplayName(ctx context.Context, name, displayName string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	trimmed := strings.TrimSpace(displayName)
	return s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppDisplayName(ctx, id, trimmed) },
		func(a *appv1alpha1.App) { a.Spec.DisplayName = trimmed })
}

// setSuspended flips suspension with the row as the single writer of intent.
// Restart needs no row write: spec.restartedAt is not projection-owned.
// Suspending (not resuming) a member App of a protectedStatus=protected
// Environment is guarded (w6/m19, requireUnprotected) — resume is never
// blocked, since it restores availability rather than taking it away.
func (s *Service) setSuspended(ctx context.Context, name string, suspended bool) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if suspended {
		if err := s.requireUnprotected(ctx, a, "suspend"); err != nil {
			return AppView{}, err
		}
	} else if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		return AppView{}, err
	}
	return s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppSuspended(ctx, id, suspended) },
		func(a *appv1alpha1.App) { a.Spec.Suspended = suspended })
}

// writeThroughStoreFetched is the shared shape of every intent-field verb
// with a row as the single writer of truth (suspend/resume, plan, …): for
// store-managed Apps the row is updated first — the projection loop owns the
// field and would revert a bare CR patch on the next resync — then the CR
// patch after it makes the change converge immediately; if the row write
// fails, the CR is left untouched (the row is already wrong, so retrying is
// safe). Unmanaged (bare-CR) Apps skip the row entirely and go straight to
// the CR patch. SetDisplayName reuses it for the opposite ownership — its row
// is a read projection, not intent — because the ordering matters there too: a
// half-applied rename is exactly the drift it closes. Every caller
// (setSuspended, SetPlan, Scale, SetIdleTTL, …)
// authorizes+fetches its own App first via AuthorizeApp (against the App's
// own workspace, w6/m17) — some validate the request or run a guard (e.g.
// setSuspended's requireUnprotected) against it before reaching here — then
// hands the already-fetched App to this shared write, so it's never
// authorized or fetched twice.
func (s *Service) writeThroughStoreFetched(
	ctx context.Context, a *appv1alpha1.App,
	writeRow func(ctx context.Context, id string) error,
	mutate func(*appv1alpha1.App),
) (AppView, error) {
	if s.Store != nil {
		if id := managedAppID(a); id != "" {
			if err := writeRow(ctx, id); err != nil {
				return AppView{}, fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	return s.patchFetched(ctx, a, mutate)
}

// patch authorizes and fetches the App by name (against its own workspace,
// core.Base.AuthorizeApp) then applies mutate. Restart/SetAutoDeploy's
// single-fetch shape; a row-writing verb instead reuses the App it already
// fetched via writeThroughStoreFetched/patchFetched. relation is the one the
// calling verb needs.
func (s *Service) patch(ctx context.Context, relation, name string, mutate func(*appv1alpha1.App)) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, relation, name)
	if err != nil {
		return AppView{}, err
	}
	return s.patchFetched(ctx, a, mutate)
}

// patchFetched applies mutate to an already-fetched App and merge-patches —
// only spec fields change; the operator reconciles the rest. The single write
// path every lifecycle verb ultimately shares.
//
// It is also the deploy-history gate (w6/m51). bex cannot apply a build- or
// release-relevant spec change without rolling a new release, so any mutation
// that moves the operator's release identity is a real rollout and gets its own
// deploys row — the same guarantee w2/m30 gave Restart, now covering the
// Settings-page verbs (start/build command, root dir, Dockerfile path,
// pre-deploy command, health check path, plan, disks) that used to
// rebuild invisibly. A purely operational mutation (scale, autoDeploy, custom
// domains, notification policy) changes nothing the operator rebuilds for and
// deliberately mints no row. Update Source is the exception among release
// fields: SetSourceAndRegistryCredential marks it pending and bypasses this
// path so Render's "next deploy uses it" contract remains true.
func (s *Service) patchFetched(ctx context.Context, a *appv1alpha1.App, mutate func(*appv1alpha1.App)) (AppView, error) {
	return s.patchTracked(ctx, a, store.TriggerConfigChange, mutate)
}

// patchTracked is patchFetched with an explicit deploy-history trigger.
func (s *Service) patchTracked(ctx context.Context, a *appv1alpha1.App, trigger string, mutate func(*appv1alpha1.App)) (AppView, error) {
	err := s.rollouts().Patch(ctx, s.Client, a, trigger, func(a *appv1alpha1.App) error {
		mutate(a)
		resourcemeta.Touch(a, s.Now())
		return nil
	})
	if err != nil {
		return AppView{}, err
	}
	return s.view(a), nil
}

// patchUntracked applies a CR-only mutation without opening a generic deploy
// row or stamping a release generation. Source updates use it because they are
// deferred until the next deploy; redeployFetched uses it because that caller
// records a richer deploy row itself.
func (s *Service) patchUntracked(ctx context.Context, a *appv1alpha1.App, mutate func(*appv1alpha1.App)) (AppView, error) {
	base := client.MergeFrom(a.DeepCopy())
	mutate(a)
	resourcemeta.Touch(a, s.Now())
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return AppView{}, err
	}
	return s.view(a), nil
}

// rollouts adapts the service's own store slice to the deploy-row recorder.
// Deriving it from Store rather than wiring a separate field is what keeps a
// second composition root (cmd/ssh-gateway, the workspace purger) from silently
// building an untracked Service. A nil Store (CR-only mode) yields a tracker
// that records nothing.
func (s *Service) rollouts() *rollout.Tracker {
	if s.Store == nil {
		return nil
	}
	return &rollout.Tracker{Store: s.Store}
}

// routesFromViews / headersFromViews convert surface input (neutral views) into
// the CR spec types, trimming whitespace.
func routesFromViews(views []StaticRouteView) []appv1alpha1.StaticRoute {
	if len(views) == 0 {
		return nil
	}
	out := make([]appv1alpha1.StaticRoute, len(views))
	for i, v := range views {
		out[i] = appv1alpha1.StaticRoute{
			Type:        strings.TrimSpace(v.Type),
			Source:      strings.TrimSpace(v.Source),
			Destination: strings.TrimSpace(v.Destination),
		}
	}
	return out
}

func headersFromViews(views []StaticHeaderView) []appv1alpha1.StaticHeader {
	if len(views) == 0 {
		return nil
	}
	out := make([]appv1alpha1.StaticHeader, len(views))
	for i, v := range views {
		out[i] = appv1alpha1.StaticHeader{
			Path:  strings.TrimSpace(v.Path),
			Name:  strings.TrimSpace(v.Name),
			Value: v.Value,
		}
	}
	return out
}

// Static edge-rule budgets (codex-security round 12, finding 3): the shared
// static server linearly scans a site's routes on every unauthenticated request
// and applies every matching header rule, so the durable per-site rule state
// must stay bounded before it reaches that path. The CRD schema carries the
// same numbers (MaxItems/MaxLength in lego/types/v1alpha1/app_types.go); this
// is the surface-side rejection so an over-budget config is a clean 400 across
// REST/GraphQL/MCP before any CR write.
const (
	maxStaticRoutes         = 100
	maxStaticHeaders        = 100
	maxStaticPathLen        = 2048
	maxStaticHeaderNameLen  = 256
	maxStaticHeaderValueLen = 4096
)

// validateRoutes rejects a malformed redirect/rewrite list: each rule needs a
// known type and a rooted source + destination path (Render's contract), and
// the whole list stays inside the edge-rule budgets above.
func validateRoutes(routes []StaticRouteView) error {
	if len(routes) > maxStaticRoutes {
		return fmt.Errorf("%w: routes must not exceed %d items", core.ErrBadRequest, maxStaticRoutes)
	}
	for i, r := range routes {
		t := strings.TrimSpace(r.Type)
		if t != "redirect" && t != "rewrite" {
			return fmt.Errorf("%w: routes[%d].type must be redirect or rewrite", core.ErrBadRequest, i)
		}
		if source := strings.TrimSpace(r.Source); !strings.HasPrefix(source, "/") {
			return fmt.Errorf("%w: routes[%d].source must be a path starting with /", core.ErrBadRequest, i)
		} else if len(source) > maxStaticPathLen {
			return fmt.Errorf("%w: routes[%d].source must not exceed %d characters", core.ErrBadRequest, i, maxStaticPathLen)
		}
		dest := strings.TrimSpace(r.Destination)
		if !strings.HasPrefix(dest, "/") {
			return fmt.Errorf("%w: routes[%d].destination must be a path starting with /", core.ErrBadRequest, i)
		}
		if len(dest) > maxStaticPathLen {
			return fmt.Errorf("%w: routes[%d].destination must not exceed %d characters", core.ErrBadRequest, i, maxStaticPathLen)
		}
		// round-5 finding 11: a leading "/" is not enough. "//host" (protocol-
		// relative) and "/\host" (browsers normalize backslash to slash) are
		// network-path references the static server's http.Redirect turns into an
		// off-site open redirect. Require a genuine local path.
		if isNetworkPathReference(dest) {
			return fmt.Errorf("%w: routes[%d].destination must be a local path, not a network-path reference", core.ErrBadRequest, i)
		}
	}
	return nil
}

// isNetworkPathReference reports whether a "/"-rooted redirect destination would
// actually send a browser off-site: a protocol-relative "//host" (or its "/\host"
// backslash variant), or any embedded backslash/CR/LF/NUL that a downstream
// parser could renormalize into one.
func isNetworkPathReference(p string) bool {
	if strings.HasPrefix(p, "//") || strings.HasPrefix(p, `/\`) {
		return true
	}
	return strings.ContainsAny(p, "\\\r\n\x00")
}

// validateHeaders rejects a malformed custom-header list: each rule needs a
// rooted path pattern and a well-formed header name + value. Name/value are
// token/CRLF-validated so a rule cannot inject a second header or split the
// response once the static server emits it via h.Set (round-5 finding 11), and
// the whole list stays inside the edge-rule budgets above.
// SECURITY (finding-5): on a shared registrable suffix (e.g. onbex.co, which
// is not a private PSL suffix) a tenant Set-Cookie with Domain=onbex.co is
// sent to every sibling tenant. Reject such cross-tenant cookie injection
// while the hosting suffix remains unlisted; host-only cookies remain allowed.
func validateHeaders(headers []StaticHeaderView) error {
	if len(headers) > maxStaticHeaders {
		return fmt.Errorf("%w: headers must not exceed %d items", core.ErrBadRequest, maxStaticHeaders)
	}
	for i, h := range headers {
		if !strings.HasPrefix(strings.TrimSpace(h.Path), "/") {
			return fmt.Errorf("%w: headers[%d].path must be a path starting with /", core.ErrBadRequest, i)
		}
		if len(strings.TrimSpace(h.Path)) > maxStaticPathLen {
			return fmt.Errorf("%w: headers[%d].path must not exceed %d characters", core.ErrBadRequest, i, maxStaticPathLen)
		}
		if name := strings.TrimSpace(h.Name); name == "" {
			return fmt.Errorf("%w: headers[%d].name is required", core.ErrBadRequest, i)
		} else if len(name) > maxStaticHeaderNameLen {
			return fmt.Errorf("%w: headers[%d].name must not exceed %d characters", core.ErrBadRequest, i, maxStaticHeaderNameLen)
		}
		if !httpguts.ValidHeaderFieldName(h.Name) {
			return fmt.Errorf("%w: headers[%d].name %q is not a valid HTTP header name", core.ErrBadRequest, i, h.Name)
		}
		if len(h.Value) > maxStaticHeaderValueLen {
			return fmt.Errorf("%w: headers[%d].value must not exceed %d characters", core.ErrBadRequest, i, maxStaticHeaderValueLen)
		}
		if !httpguts.ValidHeaderFieldValue(h.Value) {
			return fmt.Errorf("%w: headers[%d].value contains an invalid character", core.ErrBadRequest, i)
		}
		if strings.EqualFold(h.Name, "Set-Cookie") && isCrossTenantCookieValue(h.Value) {
			return fmt.Errorf("%w: headers[%d].value Set-Cookie with Domain attribute is not allowed on shared hosting (cross-tenant cookie injection)", core.ErrBadRequest, i)
		}
	}
	return nil
}

// isCrossTenantCookieValue reports whether a Set-Cookie value carries a Domain
// attribute (any value). On shared hosting without PSL private-suffix
// isolation, any Domain attribute lets the cookie scope to the parent domain
// and sibling tenants; host-only cookies (no Domain) remain correctly isolated
// by Same-Origin + host-only semantics.
func isCrossTenantCookieValue(value string) bool {
	parts := strings.Split(value, ";")
	for _, p := range parts[1:] {
		attr := strings.TrimSpace(p)
		if strings.HasPrefix(strings.ToLower(attr), "domain=") {
			return true
		}
	}
	return false
}

// validatePublishPath rejects an empty or control-character publishPath. The
// value reaches the operator's extract/clone Job via env — never interpolated
// into a shell — so it cannot inject a command; control bytes are rejected as
// obvious garbage. Absolute paths are intentionally ALLOWED: an image-backed
// static_site serves a known in-image directory (e.g. /usr/share/nginx/html),
// and the repo-backed checkout-escape guard already lives in the operator
// (publish.srcDir). The real authorization control for this verb is can_create
// (round-5 finding 11), enforced by the caller.
func validatePublishPath(publishPath string) error {
	pp := strings.TrimSpace(publishPath)
	if pp == "" {
		return fmt.Errorf("%w: publishPath must not be empty", core.ErrBadRequest)
	}
	if strings.ContainsAny(pp, "\r\n\x00") {
		return fmt.Errorf("%w: publishPath must not contain control characters", core.ErrBadRequest)
	}
	return nil
}

// requireRepoBacked refuses a build-settings verb on a service with no repo,
// naming which setting does not apply — the sibling of requireStaticSite.
func requireRepoBacked(a *appv1alpha1.App, name, detail string) error {
	if a.Spec.Repo == "" {
		return fmt.Errorf("%w: service %q has no repo to build (%s)", core.ErrBadRequest, name, detail)
	}
	return nil
}

// requireStaticSite returns ErrBadRequest unless a is a static_site — the edge
// verbs (routes/headers/publishPath) apply only to that type.
func requireStaticSite(a *appv1alpha1.App, name string) error {
	if a.Spec.Type != appv1alpha1.TypeStaticSite {
		return fmt.Errorf("%w: service %q is not a static_site", core.ErrBadRequest, name)
	}
	return nil
}

// ListRoutes returns a static_site's ordered redirect/rewrite rules (Render's
// GET /v1/services/{id}/routes).
func (s *Service) ListRoutes(ctx context.Context, name string) ([]StaticRouteView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	// Absent once deletion is accepted, same by-id contract as Get (w3/m81).
	if err := core.NotFoundIfDeleting(a); err != nil {
		return nil, err
	}
	if err := requireStaticSite(a, name); err != nil {
		return nil, err
	}
	return staticRouteViews(a.Spec.Routes), nil
}

// SetRoutes replaces a static_site's redirect/rewrite rules (Render's bulk
// PUT /v1/services/{id}/routes). Routes live on the CR spec and the
// static-server reads them live, so the change takes effect on the next resolver
// refresh — no rebuild/republish. Direct CR patch (not projection-owned).
func (s *Service) SetRoutes(ctx context.Context, name string, routes []StaticRouteView) (AppView, error) {
	// round-5 finding 11: routes change what the public origin serves (redirects
	// can be off-site), so this is a content/security mutation — can_create
	// (developer), not the can_operate a lifecycle-only contributor holds.
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	if err := validateRoutes(routes); err != nil {
		return AppView{}, err
	}
	if err := requireStaticSite(a, name); err != nil {
		return AppView{}, err
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Routes = routesFromViews(routes)
	})
}

// ListHeaders returns a static_site's custom response-header rules (Render's
// GET /v1/services/{id}/headers).
func (s *Service) ListHeaders(ctx context.Context, name string) ([]StaticHeaderView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	// Absent once deletion is accepted, same by-id contract as Get (w3/m81).
	if err := core.NotFoundIfDeleting(a); err != nil {
		return nil, err
	}
	if err := requireStaticSite(a, name); err != nil {
		return nil, err
	}
	return staticHeaderViews(a.Spec.Headers), nil
}

// SetHeaders replaces a static_site's custom response-header rules (Render's bulk
// PUT /v1/services/{id}/headers). Same live-read semantics as SetRoutes.
func (s *Service) SetHeaders(ctx context.Context, name string, headers []StaticHeaderView) (AppView, error) {
	// round-5 finding 11: response headers are a security control (CSP, HSTS,
	// framing) the public server emits, so require can_create (developer), not
	// the can_operate a lifecycle-only contributor holds.
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	if err := validateHeaders(headers); err != nil {
		return AppView{}, err
	}
	if err := requireStaticSite(a, name); err != nil {
		return AppView{}, err
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Headers = headersFromViews(headers)
	})
}

// SetPublishPath changes the built output directory a static_site serves
// (spec.publishPath) and bumps spec.restartedAt so the release identity changes
// and the publish plane runs again.
// Rejected for a non-static_site or an empty path.
func (s *Service) SetPublishPath(ctx context.Context, name, publishPath string) (AppView, error) {
	// round-5 finding 11: the served output directory is content, not lifecycle —
	// require can_create (developer), not the can_operate a contributor holds.
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	if err := validatePublishPath(publishPath); err != nil {
		return AppView{}, err
	}
	if err := requireStaticSite(a, name); err != nil {
		return AppView{}, err
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.PublishPath = strings.TrimSpace(publishPath)
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
}

// AutoscalingView is the neutral projection of a service's autoscaling config —
// Render's Scaling tab shape (minInstances/maxInstances/targetCPUPercent/
// targetMemoryPercent) mapped from spec.autoscaling.
type AutoscalingView struct {
	Enabled             bool   `json:"enabled"`
	MinInstances        int32  `json:"minInstances"`
	MaxInstances        int32  `json:"maxInstances"`
	TargetCPUPercent    *int32 `json:"targetCPUPercent,omitempty"`
	TargetMemoryPercent *int32 `json:"targetMemoryPercent,omitempty"`
}

// SetAutoscalingRequest is the input for SetAutoscaling — Render's PUT
// /v1/services/{id}/autoscaling request body projected onto spec.autoscaling.
type SetAutoscalingRequest struct {
	MinInstances        int32  `json:"minInstances"`
	MaxInstances        int32  `json:"maxInstances"`
	TargetCPUPercent    *int32 `json:"targetCPUPercent,omitempty"`
	TargetMemoryPercent *int32 `json:"targetMemoryPercent,omitempty"`
}

// autoscalingSpec validates a SetAutoscalingRequest and returns the
// corresponding AutoscalingSpec (Enabled:true). Shared by SetAutoscaling and
// specFromCreate (the Blueprint scaling: block path, w2/m49) so validation
// is identical regardless of entry point. tier is the App's spec.tier: the
// autoscaler drives replicas up to maxInstances, so an uncapped maxInstances
// would reintroduce the over-cap outcome by a different door (w6/m118 t003).
func autoscalingSpec(req SetAutoscalingRequest, tier string) (appv1alpha1.AutoscalingSpec, error) {
	if req.MinInstances < 0 {
		return appv1alpha1.AutoscalingSpec{}, fmt.Errorf("%w: minInstances must be ≥ 0", core.ErrBadRequest)
	}
	if req.MaxInstances < 1 {
		return appv1alpha1.AutoscalingSpec{}, fmt.Errorf("%w: maxInstances must be ≥ 1", core.ErrBadRequest)
	}
	if req.MinInstances > req.MaxInstances {
		return appv1alpha1.AutoscalingSpec{}, fmt.Errorf("%w: minInstances must be ≤ maxInstances", core.ErrBadRequest)
	}
	if req.MinInstances > store.MaxReplicas {
		return appv1alpha1.AutoscalingSpec{}, fmt.Errorf("%w: minInstances must be 0-%d", core.ErrBadRequest, store.MaxReplicas)
	}
	if req.MaxInstances > store.MaxReplicas {
		return appv1alpha1.AutoscalingSpec{}, fmt.Errorf("%w: maxInstances must be 1-%d", core.ErrBadRequest, store.MaxReplicas)
	}
	// The plan cap bounds the range the autoscaler may drive into: both ends,
	// since a minInstances above the cap is equally unrunnable.
	if err := checkInstanceCap(tier, req.MaxInstances); err != nil {
		return appv1alpha1.AutoscalingSpec{}, err
	}
	if err := checkInstanceCap(tier, req.MinInstances); err != nil {
		return appv1alpha1.AutoscalingSpec{}, err
	}
	if req.TargetCPUPercent != nil && (*req.TargetCPUPercent < 1 || *req.TargetCPUPercent > 100) {
		return appv1alpha1.AutoscalingSpec{}, fmt.Errorf("%w: targetCPUPercent must be 1–100", core.ErrBadRequest)
	}
	if req.TargetMemoryPercent != nil && (*req.TargetMemoryPercent < 1 || *req.TargetMemoryPercent > 100) {
		return appv1alpha1.AutoscalingSpec{}, fmt.Errorf("%w: targetMemoryPercent must be 1–100", core.ErrBadRequest)
	}
	if req.TargetCPUPercent == nil && req.TargetMemoryPercent == nil {
		return appv1alpha1.AutoscalingSpec{}, fmt.Errorf("%w: at least one of targetCPUPercent or targetMemoryPercent is required", core.ErrBadRequest)
	}
	return appv1alpha1.AutoscalingSpec{
		Enabled:             true,
		MinReplicas:         req.MinInstances,
		MaxReplicas:         req.MaxInstances,
		TargetCPUPercent:    req.TargetCPUPercent,
		TargetMemoryPercent: req.TargetMemoryPercent,
	}, nil
}

// autoscalingView projects spec.autoscaling onto the neutral view. Nil
// spec.autoscaling => disabled with zero bounds (the disabled state).
func autoscalingView(a *appv1alpha1.App) AutoscalingView {
	as := a.Spec.Autoscaling
	if as == nil {
		return AutoscalingView{}
	}
	return AutoscalingView{
		Enabled:             as.Enabled,
		MinInstances:        as.MinReplicas,
		MaxInstances:        as.MaxReplicas,
		TargetCPUPercent:    as.TargetCPUPercent,
		TargetMemoryPercent: as.TargetMemoryPercent,
	}
}

// GetAutoscaling returns the current autoscaling configuration for a service.
// Returns AutoscalingView{Enabled:false} when no autoscaling spec is set.
func (s *Service) GetAutoscaling(ctx context.Context, name string) (AutoscalingView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return AutoscalingView{}, err
	}
	return autoscalingView(a), nil
}

// SetAutoscaling enables autoscaling on a service (Render's PUT
// .../autoscaling). Validates min ≤ max and that at least one target is set.
// The autoscaling config is written directly to the CR spec (not row-first,
// like SetRootDir) — autoscaling is not a projection-owned field.
func (s *Service) SetAutoscaling(ctx context.Context, name string, req SetAutoscalingRequest) (AutoscalingView, error) {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanOperate, name)
	if err != nil {
		return AutoscalingView{}, err
	}
	as, err := autoscalingSpec(req, a.Spec.Tier)
	if err != nil {
		return AutoscalingView{}, err
	}
	var fromMin, fromMax *int32
	if a.Spec.Autoscaling != nil && a.Spec.Autoscaling.Enabled {
		fromMin = &a.Spec.Autoscaling.MinReplicas
		fromMax = &a.Spec.Autoscaling.MaxReplicas
	}
	_, err = s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Autoscaling = &as
	})
	if err != nil {
		return AutoscalingView{}, err
	}
	minTo, maxTo := req.MinInstances, req.MaxInstances
	s.RecordAutoscalingChanged(ctx, a, fromMin, fromMax, &minTo, &maxTo)
	return autoscalingView(a), nil
}

// DeleteAutoscaling disables autoscaling on a service (Render's DELETE
// .../autoscaling): clears spec.autoscaling so the service reverts to its
// fixed spec.replicas count. Idempotent — already-disabled is a no-op.
func (s *Service) DeleteAutoscaling(ctx context.Context, name string) error {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanOperate, name)
	if err != nil {
		return err
	}
	if a.Spec.Autoscaling == nil || !a.Spec.Autoscaling.Enabled {
		return nil // already disabled — idempotent
	}
	fromMin := a.Spec.Autoscaling.MinReplicas
	fromMax := a.Spec.Autoscaling.MaxReplicas
	_, err = s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Autoscaling = nil
	})
	if err != nil {
		return err
	}
	s.RecordAutoscalingChanged(ctx, a, &fromMin, &fromMax, nil, nil)
	return nil
}
