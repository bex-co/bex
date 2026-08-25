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

package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand/v2"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Typed resource ids ("tea-…"/"srv-…"/"cdm-…") are minted through the one id
// package (docs/ADR020-identifiers.md) — never hand-concatenated here, so the format
// and its DNS-safety stay guarded in one place. A rename never breaks a
// reference because ids, not names, are the keys (docs/ADR009-postgresql-management.md §4).

// MaxReplicas is the shared upper bound on an App's replica count, enforced by
// both the create path (store/api.go) and the apps scale verb so the two can't
// disagree about what a valid App is. The lower bounds legitimately differ
// (create treats 0 as "default 1"; scale rejects 0 — see apps.Service.Scale).
// The value is the CRD/operator ceiling (types.MaxReplicas) so autoscaling,
// explicit scale, and create cannot drift from what Kubernetes will admit.
const MaxReplicas = appv1alpha1.MaxReplicas

// Error taxonomy shared by the store and the API: the store classifies
// Postgres failures into these, the API maps them to status codes
// (ErrInvalid→400, ErrNotFound→404, ErrConflict→409).
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
	ErrInvalid  = errors.New("invalid")
)

// MapError translates this taxonomy into core's, so a feature service can
// return a store error straight out of a verb and have every surface map it to
// the right status. The store error stays in the message (%v, not %w) — the
// core sentinel is what callers match on.
//
// Features whose mapping differs deliberately (workspaces folds Conflict into
// ErrBadRequest; registrycreds and webhooks return bare sentinels) keep their
// own — this is the common case, not a mandate.
func MapError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return fmt.Errorf("%w: %v", core.ErrNotFound, err)
	case errors.Is(err, ErrConflict):
		return fmt.Errorf("%w: %v", core.ErrConflict, err)
	case errors.Is(err, ErrInvalid):
		return fmt.Errorf("%w: %v", core.ErrBadRequest, err)
	default:
		return err
	}
}

// Tenant is a row of `tenants` — who owns apps; plan names the tier ladder row.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"createdAt"`
}

// App is a row of `apps` — the source-of-truth service definition the
// reconciler projects into an App CR. Observed state (phase, url) is NOT stored
// here: it lives on the App CR's status, which bex-api reads at query time.
type App struct {
	ID       string `json:"id"`
	TenantID string `json:"tenantId"`
	Name     string `json:"name"`
	// Slug is the globally-unique public subdomain (Render's "slug", distinct
	// from Name which is only workspace-unique): the bare Name when free
	// platform-wide, or "<name>-<4-char suffix>" when CreateApp had to mint one
	// to avoid a cross-tenant collision (w4/m19). The operator reads this —
	// never Name — to derive the platform host.
	Slug string `json:"slug"`
	// Type is the App's service kind (types.AppSpec.Type's vocabulary:
	// web_service | private_service | background_worker | cron_job |
	// static_site). The projector derives spec.expose from it, so a row that
	// does not carry it cannot be projected correctly. Immutable for the app's
	// whole life, so it is recorded once at create. Empty is a pre-w6/m46 row
	// and reads as web_service, exactly as the CRD contract defines the empty
	// type; the projector backfills the real value from the live CR on resync.
	Type  string `json:"type,omitempty"`
	Repo  string `json:"repo,omitempty"`
	Image string `json:"image,omitempty"`
	// RegistryCredentialID preserves Render's tri-state image credential
	// binding: nil is legacy host auto-resolution, pointer-to-empty explicitly
	// selects no credential, and a non-empty value pins one workspace credential.
	RegistryCredentialID *string   `json:"registryCredentialId,omitempty"`
	Branch               string    `json:"branch"`
	Port                 int32     `json:"port"`
	Replicas             int32     `json:"replicas"`
	Tier                 string    `json:"tier"`
	IdleTTLSeconds       int32     `json:"idleTTLSeconds"`
	Suspended            bool      `json:"suspended"`
	ProjectID            string    `json:"projectId,omitempty"`
	EnvironmentID        string    `json:"environmentId,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
	// FirstDeployCommit is CreateApp INPUT only (w9/001): the resolved commit
	// stamped onto the first deploy row (trigger "create") CreateApp opens in
	// the same transaction. Not an apps column — never persisted on, or read
	// back from, the app row itself. Zero value ⇒ the first deploy carries no
	// commit metadata (image-backed app, or no GitHub connection to resolve
	// the branch through).
	FirstDeployCommit CommitInfo `json:"-"`
	// FirstDeployID is CreateApp OUTPUT only (w3/m14): the id minted for the
	// first deploy row. Callers use it to navigate straight to the deploy page.
	// Not stored on the app row — only populated by CreateApp, never read back.
	FirstDeployID string `json:"-"`
}

// Deploy status vocabulary — Render's complete eleven-state enum. The backend
// persists only states supported by current App/Job evidence; it never invents
// an unobserved intermediate phase merely to make the sequence look complete.
const (
	DeployCreated             = "created"
	DeployQueued              = "queued"
	DeployBuildInProgress     = "build_in_progress"
	DeployBuildFailed         = "build_failed"
	DeployPreDeployInProgress = "pre_deploy_in_progress"
	DeployPreDeployFailed     = "pre_deploy_failed"
	DeployUpdateInProgress    = "update_in_progress"
	DeployLive                = "live"
	DeployUpdateFailed        = "update_failed"
	DeployDeactivated         = "deactivated"
	DeployCanceled            = "canceled"
)

// Pre-deploy step status vocabulary — the deploy row's pre_deploy_status column
// (w1/m33). The lowercase projection of the App CR's status.preDeploy.Status,
// distinct from the overall deploy status: a deploy can be update_failed with
// pre_deploy_status 'failed' (its migration failed) or ” (its health check
// failed). Empty means no pre-deploy step ran.
const (
	PreDeployRunning   = "running"
	PreDeploySucceeded = "succeeded"
	PreDeployFailed    = "failed"
)

// RenderDeployStatus maps a TERMINAL bex deploy status onto Render's
// deployStatus enum (succeeded|failed|canceled) — one mapping next to the
// vocabulary it interprets, shared by the events feed's deploy_ended details
// and the webhook payload's data.status so the same deploy can never be
// reported differently by the two (the drift w2/m10's `canceled` addition
// showed is possible).
func RenderDeployStatus(status string) string {
	switch status {
	case DeployLive:
		return "succeeded"
	case DeployDeactivated:
		// A deactivated deploy previously reached live; a newer successful
		// deploy replaced it. Its deploy_ended outcome remains succeeded.
		return "succeeded"
	case DeployCanceled:
		return "canceled"
	default:
		return "failed"
	}
}

// The deploy `trigger` vocabulary — what caused a rollout. One place, since
// both the writers (CreateApp, deploys.Trigger/Rollback) and the readers (the
// events feed's deploy_started trigger flags, internal/events) spell it.
const (
	TriggerCreate     = "create"      // the app's first deploy, opened by CreateApp
	TriggerAPI        = "api"         // an authenticated POST .../deploys
	TriggerDeployHook = "deploy_hook" // the App's unauthenticated secret-URL trigger
	TriggerRollback   = "rollback"    // a deploy created by Rollback (w2/m10), restoring an earlier image
	TriggerNewCommit  = "new_commit"  // a git-push redeploy via the HMAC webhook (Render's spelling)
	// TriggerConfigChange is a rollout a configuration write forced rather than
	// an explicit deploy verb: a Settings-page field, an env var or secret file,
	// an env-group link/unlink. bex cannot apply any of these without rolling a
	// new release, so each is a real deploy the user must be able to see, retry,
	// and roll back — spelled distinctly so the history still tells a settings
	// save apart from a manual deploy (w6/m51).
	TriggerConfigChange = "config_change"
)

// CommitInfo is the git commit a build-from-git deploy runs — the resolved
// SHA, message, and author timestamp (w9/001 + w2/m42). Zero value = unknown:
// image-backed app, no GitHub connection to resolve through, or resolution
// failed. Commit metadata is best-effort provenance, never load-bearing — a
// deploy with an empty CommitInfo is still a fully valid deploy.
type CommitInfo struct {
	Hash     string     `json:"hash,omitempty"`
	Message  string     `json:"message,omitempty"`
	AuthorAt *time.Time `json:"authorAt,omitempty"`
}

// Deploy is a row of `deploys` — one rollout attempt of an app, Render's
// deploy history (list_deploys/get_deploy). Trigger is "create" (the app's
// first deploy, opened by CreateApp), "api" (an explicit POST .../deploys),
// "deploy_hook" (the service's secret URL), "new_commit" (a git-push redeploy
// via the HMAC webhook), or "rollback" (w2/m10: a deploy created by Rollback,
// RollbackOf naming the source deploy it restores).
// Commit/CommitMessage (w9/001) are the resolved commit a build-from-git
// deploy ran, captured once at open time via the workspace's GitHub App
// connection — "" when unresolvable (omitted by the views, not faked).
// ResolvedImage is the image this deploy
// actually put into the cluster (backfilled by CloseDeploy once the deploy
// reaches live — "" until then, and forever for one that never does): the
// only field Rollback (w2/m10) trusts as a restore target, since Image alone
// is "" for a build-from-git deploy until the build resolves it. Generation
// is the App CR's metadata.generation this deploy runs under, captured once
// at open time — Cancel (w2/m10) derives its build-Job identity from this
// stored value rather than the App's current generation, which a later,
// unrelated spec write could have already moved past.
type Deploy struct {
	ID             string     `json:"id"`
	AppID          string     `json:"appId"`
	Trigger        string     `json:"trigger"`
	Image          string     `json:"image,omitempty"`
	ResolvedImage  string     `json:"-"`
	RollbackOf     string     `json:"rollbackOf,omitempty"`
	Generation     int64      `json:"-"`
	Commit         string     `json:"commit,omitempty"`
	CommitMessage  string     `json:"commitMessage,omitempty"`
	CommitAuthorAt *time.Time `json:"commitAuthorAt,omitempty"`
	Status         string     `json:"status"`
	// OverlapPending distinguishes the one latest-pending overlap slot from an
	// active release that may itself report Render status queued while waiting
	// for build capacity. It is store-internal and never exposed on a surface.
	OverlapPending bool       `json:"-"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
	// PreDeployStatus is the pre-deploy command's outcome for this deploy
	// (w1/m33): '' (no step) | 'running' | 'succeeded' | 'failed'. The reconciler
	// projects it from the App CR's status.preDeploy so a migration failure is
	// distinguishable from a health-check failure (both close as update_failed).
	PreDeployStatus string `json:"preDeployStatus,omitempty"`
	// FailureReason is the human-actionable cause stamped when the reconciler
	// closes this deploy failed (w9/011) — the App CR's Ready-condition message
	// (crash loop with the $PORT hint, image-pull failure, build error) or a
	// synthesized health-gate-timeout line. Empty on non-failed deploys. A bex
	// extension beyond Render's deploy shape, like RollbackOf and
	// PreDeployStatus.
	FailureReason string `json:"failureReason,omitempty"`
}

// Domain is a row of `domains` — a BYOD custom domain attached to an app.
// The primary domain becomes the App CR's spec.host, the rest spec.hosts.
// ClaimState is durable ownership admission; only verified rows are projected
// into App CRs. TLS/cert status remains observed from the cluster.
type Domain struct {
	ID                   string     `json:"id"`
	AppID                string     `json:"appId"`
	Host                 string     `json:"host"`
	Primary              bool       `json:"primary"`
	RedirectForName      string     `json:"redirectForName,omitempty"`
	ClaimState           string     `json:"claimState"`
	Challenge            string     `json:"challenge,omitempty"`
	ChallengeVersion     int64      `json:"challengeVersion"`
	VerificationAttempts int64      `json:"verificationAttempts"`
	LastVerificationAt   *time.Time `json:"lastVerificationAt,omitempty"`
	VerifiedAt           *time.Time `json:"verifiedAt,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
}

// DomainDeclaration is one desired host plus projection metadata. Replacing a
// declaration set preserves an unchanged row's claim state and challenge;
// newly declared hosts enter pending.
type DomainDeclaration struct {
	Host            string
	Primary         bool
	RedirectForName string
}

// DesiredApp is an apps row joined with everything projection needs: the
// owning tenant's name (part of the CR name) and the app's domains,
// primary first.
// Disk is a persistent volume attached to one service
// (docs/ADR082-persistent-disks.md). Size is PROVISIONED capacity: it is what
// the tenant is billed for whether the service is running, suspended, or
// scaled to zero, exactly as Render bills its disks and as Hetzner bills the
// volume underneath.
type Disk struct {
	ID        string     `json:"id"`
	TenantID  string     `json:"tenantId"`
	AppID     string     `json:"appId"`
	Name      string     `json:"name"`
	MountPath string     `json:"mountPath"`
	SizeGB    int32      `json:"sizeGB"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// DiskUsageRow is one service's provisioned disk GB-seconds within a metering
// window, attributed to the owning service (ADR082 D9 — the disk is a property
// of its service, so the meter keys on srv-, not dsk-).
type DiskUsageRow struct {
	TenantID  string
	AppID     string
	GBSeconds int64
}

type DesiredApp struct {
	App
	// Disk is the service's live disk, nil when it has none. The projector
	// turns it into spec.disk; clearing it detaches the volume.
	Disk          *Disk
	TenantName    string
	PrimaryHost   string
	Hosts         []string
	HostRedirects map[string]string
	// EnvironmentIPAllowList is the member's environment inbound-IP layer
	// (w4/m28), projected from the environments row at read time (the
	// EnvironmentLayerCIDRs shape member CRs carry) so a projector-created CR
	// starts consistent with the environments fan-out. Update-path syncs stay
	// fan-out-owned: this field is deliberately NOT in applyOwnedSpec.
	EnvironmentIPAllowList []string
}

// Store is the persistence boundary. The API writes through it, the reconciler
// reads desired state and projects it into App CRs. Observed state (phase/url)
// is not persisted — it stays on the CR. The production implementation is
// PGStore; tests use an in-memory fake.
type Store interface {
	CreateTenant(ctx context.Context, name, plan string) (Tenant, error)
	ListTenants(ctx context.Context) ([]Tenant, error)
	// GetTenant and CountAppsForTenant back the per-plan service cap the create
	// path enforces (w6/m1): a Hobby workspace is capped at 25 apps.
	GetTenant(ctx context.Context, id string) (Tenant, error)
	CountAppsForTenant(ctx context.Context, tenantID string) (int, error)
	// SetTenantBillingExcluded flips a workspace's Stripe billing-exclusion
	// flag (docs/ADR040-billing-metronome.md §7), auditing the change. Admin-only
	// — the control-plane internal API is its sole caller. Returns whether the
	// value changed; ErrNotFound for an unknown workspace.
	SetTenantBillingExcluded(ctx context.Context, tenantID string, excluded bool, actor string, at time.Time) (bool, error)
	// TenantForIdentity returns the subject's DEFAULT workspace (w6/m14): the
	// tenant of its OLDEST membership in tenant_members, or ErrNotFound. One
	// lookup serves both human (Kratos identity id) and machine (Hydra client
	// id) callers since tenant_members.subject covers both kinds of id.
	//
	// The default-workspace contract: a subject may belong to several
	// workspaces (w6/m1 multi-workspace, w4/m12 invites), so this is the
	// implicit resolution used when a caller names no workspace — deterministic
	// by membership created_at (tenant id as the tie-break), never an arbitrary
	// row of the join. Oldest-first makes it the workspace the caller has had
	// longest, which for a human is the personal tenant minted on first login,
	// matching Render ("passing a user id returns that user's default
	// workspace"). A caller that wants another of its workspaces names it
	// explicitly (REST ownerId / GraphQL ownerId / MCP workspaceId →
	// core.WithWorkspace), which is membership-checked at core.Base.
	TenantForIdentity(ctx context.Context, subject string) (Tenant, error)
	// IsMember reports whether a subject belongs to a workspace — the check
	// core.Base runs before honoring an explicit workspace override, so naming
	// a workspace the caller is not a member of is refused (ErrForbidden)
	// rather than silently redirected to the caller's own (w6/m14).
	IsMember(ctx context.Context, subject, tenantID string) (bool, error)
	// CreateTenantWithMember mints a personal tenant for an identity on first
	// login: a tenant row owned by the identity plus an admin membership. It is
	// idempotent and race-safe — concurrent first logins for the same identity
	// yield exactly one tenant (the partial unique index on owner_identity_id
	// is the gate, not a check-then-insert). The tenant name is a placeholder
	// (its id) pending a future rename API.
	CreateTenantWithMember(ctx context.Context, identityID, plan string) (Tenant, error)
	// AddMember records a subject's membership in a tenant (idempotent). The
	// platform tenant-create path uses it to make the Admin identity a member —
	// without the row the resolver can't map that identity to its workspace.
	AddMember(ctx context.Context, subject, tenantID, role string) error
	// BindClient records that an API key belongs to a tenant — a tenant_members
	// row keyed by the key's client_id, the same table TenantForIdentity reads
	// (idempotent — a re-bind to the same or another tenant upserts). The
	// api-keys mint calls this after creating the Hydra client.
	BindClient(ctx context.Context, clientID, tenantID string) error
	// UnbindClient removes an API key's tenant binding (idempotent — a key that
	// was never bound is not an error). The api-keys revoke calls this.
	UnbindClient(ctx context.Context, clientID string) error
	CreateApp(ctx context.Context, a App) (App, error)
	GetApp(ctx context.Context, id string) (App, error)
	ListApps(ctx context.Context) ([]App, error)
	DeleteApp(ctx context.Context, id string) error
	CreateDomain(ctx context.Context, appID, host string, primary bool) (Domain, error)
	ReplaceDomains(ctx context.Context, appID, primary string, hosts []string) error
	// DeleteDomain removes a custom domain row. Not-found is ErrNotFound.
	DeleteDomain(ctx context.Context, appID, host string) error
	// Persistent service disks (docs/ADR082-persistent-disks.md). The row is
	// the source of truth for both the projected spec.disk and the provisioned
	// GB-seconds meter, which is why every write also closes or opens a size
	// period — see service_disk_sizes in migration 0096.
	//
	// CreateDisk returns ErrConflict when the service already has a live disk:
	// a service can carry at most one, and the unique index enforces it even
	// against a racing second create.
	CreateDisk(ctx context.Context, tenantID, appID, name, mountPath string, sizeGB int32) (Disk, error)
	GetDisk(ctx context.Context, id string) (Disk, error)
	// ListDisks returns the workspace's live disks, newest first. An empty
	// appID lists them all; a non-empty one narrows to a single service.
	ListDisks(ctx context.Context, tenantID, appID string) ([]Disk, error)
	// UpdateDisk applies a grow and/or a rename/remount. sizeGB below the
	// current size is ErrInvalid — a disk never shrinks (ADR082 D2).
	UpdateDisk(ctx context.Context, id string, name, mountPath *string, sizeGB *int32) (Disk, error)
	// DeleteDisk soft-deletes the row, which both detaches the volume (the
	// projector clears spec.disk) and stops the meter by closing the open size
	// period. The row is retained so an already-billed period stays auditable.
	DeleteDisk(ctx context.Context, id string) error
	// DiskUsageForWindow integrates provisioned disk GB-seconds per service
	// over [from, to); LatestUsageWindowForKind is that meter's cursor.
	DiskUsageForWindow(ctx context.Context, from, to time.Time) ([]DiskUsageRow, error)
	LatestUsageWindowForKind(ctx context.Context, kind string) (time.Time, bool, error)
	ListDesiredApps(ctx context.Context) ([]DesiredApp, error)
	// SetAppSuspended flips the row's suspended flag — the single write path
	// for suspend/resume on store-managed Apps. bex-api's lifecycle verbs call
	// this (row first, then the CR fast-path) so the projection loop never
	// reverts a suspend it didn't know about.
	SetAppSuspended(ctx context.Context, id string, suspended bool) error
	// SetAppTier updates the row's tier — the single write path for plan
	// changes on store-managed Apps, same row-first rationale as
	// SetAppSuspended.
	SetAppTier(ctx context.Context, id string, tier string) error
	// SetAppReplicas updates the row's replica count — the single write path
	// for the manual-scale verb on store-managed Apps, same row-first
	// rationale as SetAppSuspended (the projector owns spec.replicas).
	SetAppReplicas(ctx context.Context, id string, replicas int32) error
	// SetAppIdleTTL updates the row's idle-TTL — the single write path for the
	// idle-timeout verb on store-managed Apps (the projector owns
	// spec.idleTTLSeconds), same row-first rationale as SetAppReplicas.
	SetAppIdleTTL(ctx context.Context, id string, seconds int32) error
	// SetAppSource atomically updates the projector-owned source tuple and its
	// context-sensitive registry credential binding.
	SetAppSource(ctx context.Context, id, repo, image, branch string, registryCredentialID *string) error
	// BackfillAppType records the service type of a row created before the type
	// column existed (w6/m46 t001), read off the row's own live App CR. It is a
	// one-way heal, never a rename: the type is immutable, so it only ever
	// writes a row whose type is still empty and reports whether it wrote. Not a
	// lifecycle verb — the projector is its sole caller.
	BackfillAppType(ctx context.Context, id, serviceType string) (bool, error)
	// SetAppImage updates the row's image — the single write path for a
	// rollback's restored image on store-managed Apps (the projector owns
	// spec.image), same row-first rationale as SetAppReplicas (w2/m10).
	SetAppImage(ctx context.Context, id string, image string) error
	// GetEnvironmentProtectedStatus resolves an Environment's destructive-verb
	// protection state for Database/KeyValue members whose membership is stored
	// as a CR label rather than in a control-plane resource row.
	GetEnvironmentProtectedStatus(ctx context.Context, environmentID string) (string, error)

	// UpsertUsageHourly writes one window row idempotently (ON CONFLICT DO
	// UPDATE) — the write path for the metering loop (w8/m1). Re-processing
	// the same window is safe.
	UpsertUsageHourly(ctx context.Context, row HourlyRow) error
	// RecordUsageSourceHealth persists explicit source evidence for a window
	// that could not produce a usage row. Missing evidence remains unknown.
	RecordUsageSourceHealth(ctx context.Context, records []UsageSourceRecord) error
	// ReconcileUsageSourceStreams closes streams for resources no longer in the
	// collector inventory, without deleting their current-month evidence.
	ReconcileUsageSourceStreams(ctx context.Context, active []UsageResourceRef, through time.Time) error
	// CurrentUsageCoverage aggregates explicit current-month source evidence.
	CurrentUsageCoverage(ctx context.Context, workspaceID string, now time.Time) (UsageCoverage, error)
	// LatestUsageWindow returns the most-recent window_start for one resource
	// and meter kind so each meter can catch up independently after a restart.
	// Returns zero time when no matching rows exist yet.
	LatestUsageWindow(ctx context.Context, resourceKind, serviceID, kind string) (time.Time, error)
	// UsageMonthToDate returns month-to-date aggregates (grouped by service /
	// kind / tier) for a workspace, bounded by the caller-supplied now so tests
	// don't depend on wall time. Sums usage_hourly and usage_monthly together,
	// so the result is exact whether or not the month has been compacted.
	UsageMonthToDate(ctx context.Context, workspaceID string, now time.Time) ([]UsageSummaryRow, error)
	// CompactUsage folds hourly rows older than before into usage_monthly and
	// purges them — atomic and idempotent; the retention loop (w8/m4) calls it
	// daily with the hot-window boundary.
	CompactUsage(ctx context.Context, before time.Time) (UsageCompaction, error)

	// CreateDeploy opens a new deploy row for appID (status DeployCreated) —
	// CreateApp calls this for an app's first deploy
	// (trigger "create"); the deploys feature's Trigger verb calls it for an
	// explicit redeploy (trigger "api"). generation is the App CR's
	// metadata.generation this deploy runs under, captured once at open time
	// (w2/m10) — Cancel's build-Job identity is derived from this stored
	// value, not a fresh re-fetch, so a later unrelated spec write can't make
	// it compute the wrong Job name. commit is the resolved commit this deploy
	// runs (w9/001), zero when unresolvable. The reconciler's write-back
	// closes it.
	CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit CommitInfo) (Deploy, error)
	// CreateRollbackDeploy opens a new deploy row for appID whose trigger is
	// "rollback" and whose image/resolvedImage are the target being restored
	// — Render models rollback as a fresh deploy, never a history rewrite
	// (w2/m10). rollbackOf records provenance: the source deploy id being
	// rolled back to; commit is the target's own commit metadata (w9/001),
	// copied rather than re-resolved against a branch that has since moved.
	CreateRollbackDeploy(ctx context.Context, appID, image, rollbackOf string, generation int64, commit CommitInfo) (Deploy, error)
	// ListDeploys returns an app's deploy history, newest first, narrowed by
	// filter (w2/m31) — a zero DeployFilter returns the full history, the
	// pre-m31 contract.
	ListDeploys(ctx context.Context, appID string, filter DeployFilter) ([]Deploy, error)
	// GetDeploy fetches one deploy scoped to appID — a deployID belonging to a
	// different app is ErrNotFound, not a cross-app leak.
	GetDeploy(ctx context.Context, appID, deployID string) (Deploy, error)
	// ListOpenDeploys returns every non-terminal deploy across all apps in one
	// query — the reconciler's write-back hook
	// calls this once per ReconcileOnce pass and looks apps up in the result,
	// rather than one query per app in its per-app loop.
	ListOpenDeploys(ctx context.Context) ([]Deploy, error)
	// TransitionDeploy applies one legal lifecycle transition, advances
	// updated_at only when the status actually changes, stamps started_at on
	// the first executing phase and finished_at on terminal transitions, and
	// atomically deactivates a prior live deploy when this one reaches live.
	// failureReason (w9/011) is stored with the same transition when non-empty
	// — pass it only alongside a failure status. A stale/repeated/invalid
	// transition returns false without changing data.
	TransitionDeploy(ctx context.Context, id, status, resolvedImage, failureReason, failureCode string) (bool, error)
	// CloseDeploy is the terminal-transition compatibility seam used by the
	// deploy service's Cancel path. It delegates to TransitionDeploy.
	CloseDeploy(ctx context.Context, id, status, resolvedImage string) (bool, error)
	// SetDeployPreDeployStatus records the pre-deploy step's outcome ('' |
	// 'running' | 'succeeded' | 'failed') on a deploy row (w1/m33), projected from
	// the App CR's status.preDeploy by the reconciler. No-op when unchanged;
	// returns whether a row was updated.
	SetDeployPreDeployStatus(ctx context.Context, id, status string) (bool, error)
	// RecordObservedServiceState persists level-triggered App status edges through
	// a typed checkpoint; repeated reconciler observations are no-ops.
	RecordObservedServiceState(ctx context.Context, obs ObservedServiceState) ([]ServiceEventFact, error)
	// LastHealthyTransitionAt returns the Ready=True transition time recorded
	// with the service's current healthy checkpoint — the reference the
	// reconciler's stale-conclusion guard (w6/m41) orders an unhealthy edge
	// against. Zero when there is no healthy checkpoint or its transition time
	// is unknown; callers must fail open toward recording, never silence.
	LastHealthyTransitionAt(ctx context.Context, appID string) (time.Time, error)
	InsertServiceEventFact(ctx context.Context, fact ServiceEventFact) (bool, error)
	InsertServiceEventFacts(ctx context.Context, facts []ServiceEventFact) error
}

// PGStore is the Postgres-backed Store over a pgx pool. It holds no business
// logic — validation happens in the API layer, classification of Postgres
// errors happens here.
type PGStore struct {
	Pool *pgxpool.Pool

	// cliRefreshOnce/cliRefreshLocal back IdempotentCLIRefresh's replica-local
	// response cache (codex-security 2026-08 F2): token responses are never
	// persisted, so the bytes a duplicate caller must receive byte-identically
	// live only in the process that minted them.
	cliRefreshOnce  sync.Once
	cliRefreshLocal *cliRefreshTTLCache
}

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{Pool: pool} }

// Ping reports whether the database is reachable — the /healthz check.
func (s *PGStore) Ping(ctx context.Context) error { return s.Pool.Ping(ctx) }

// withTx runs fn inside one transaction: begin, deferred best-effort rollback,
// commit iff fn returns nil. The commit here is the transaction's ONLY commit —
// fn returns nil early for fast paths and carries results out via closure
// captures. The commit error is fn's caller's error.
func (s *PGStore) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PGStore) CreateTenant(ctx context.Context, name, plan string) (Tenant, error) {
	t := Tenant{ID: ids.New(ids.Workspace), Name: name, Plan: plan}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO tenants (id, name, plan) VALUES ($1, $2, $3) RETURNING created_at`,
		t.ID, name, plan,
	).Scan(&t.CreatedAt)
	if err != nil {
		return Tenant{}, classify("tenant", err)
	}
	return t, nil
}

func (s *PGStore) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, plan, created_at FROM tenants ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// scanTenant reads the four public tenant columns off a row (id/name/plan/
// created_at — owner_identity_id is internal and never surfaced in the Tenant
// shape the API returns).
func scanTenant(row pgx.Row) (Tenant, error) {
	var t Tenant
	err := row.Scan(&t.ID, &t.Name, &t.Plan, &t.CreatedAt)
	return t, err
}

// TenantForIdentity resolves the subject's default workspace — see the Store
// interface for the contract. ORDER BY m.created_at is what makes it a
// *default* rather than an arbitrary row: the bare join returned whichever
// membership Postgres happened to yield first, so a caller with two workspaces
// could resolve to a different one call to call (w6/m11 hit this live). The
// tenant id is the tie-break for two memberships written in the same instant
// (the same-transaction case: a workspace create inserts tenant + membership
// together), so the answer is stable even then.
func (s *PGStore) TenantForIdentity(ctx context.Context, subject string) (Tenant, error) {
	t, err := scanTenant(s.Pool.QueryRow(ctx, `
		SELECT t.id, t.name, t.plan, t.created_at FROM tenants t
		JOIN tenant_members m ON m.tenant_id = t.id
		WHERE m.subject = $1
		ORDER BY m.created_at, t.id
		LIMIT 1`, subject))
	if err != nil {
		return Tenant{}, classify("tenant", err)
	}
	return t, nil
}

// IsMember reports whether the subject belongs to the workspace — the
// membership gate for an explicitly named workspace (w6/m14). A plain existence
// check, so it answers false (not ErrNotFound) for a non-member and for a
// workspace id that does not exist at all: both are "you may not act there".
func (s *PGStore) IsMember(ctx context.Context, subject, tenantID string) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM tenant_members WHERE subject = $1 AND tenant_id = $2)`,
		subject, tenantID,
	).Scan(&exists)
	if err != nil {
		return false, classify("workspace", err)
	}
	return exists, nil
}

// CreateTenantWithMember mints a personal tenant for an identity in one
// transaction. The INSERT ... ON CONFLICT (owner_identity_id) DO UPDATE is the
// race-safe idempotent gate: a concurrent first login that already inserted a
// tenant for this identity makes this a no-op that returns the winner's row
// (DO UPDATE exists only to surface RETURNING for the existing row). The
// membership upsert is idempotent for the same reason. The tenant name is its
// id — a unique DNS-safe placeholder.
func (s *PGStore) CreateTenantWithMember(ctx context.Context, identityID, plan string) (Tenant, error) {
	var t Tenant
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		id := ids.New(ids.Workspace)
		var err error
		t, err = scanTenant(tx.QueryRow(ctx, `
		INSERT INTO tenants (id, name, plan, owner_identity_id)
		VALUES ($1, $1, $2, $3)
		ON CONFLICT (owner_identity_id) WHERE owner_identity_id IS NOT NULL
		DO UPDATE SET owner_identity_id = EXCLUDED.owner_identity_id
		RETURNING id, name, plan, created_at`,
			id, plan, identityID))
		if err != nil {
			return classify("tenant", err)
		}
		if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_members (tenant_id, subject, role)
		VALUES ($1, $2, 'admin')
		ON CONFLICT DO NOTHING`, t.ID, identityID); err != nil {
			return classify("tenant_member", err)
		}
		return nil
	})
	if err != nil {
		return Tenant{}, err
	}
	return t, nil
}

// AddMember records a subject's membership in a tenant. Used both by the
// platform tenant-create path (store/api.go, an explicit Admin) and by
// BindClient (a minted API key is "membership" too — same table, same shape).
func (s *PGStore) AddMember(ctx context.Context, subject, tenantID, role string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO tenant_members (tenant_id, subject, role)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, tenantID, subject, role)
	if err != nil {
		return classify("tenant_member", err)
	}
	return nil
}

// BindClient records that an API key belongs to a tenant: a tenant_members row
// with the key's client_id as subject and role "developer" — least privilege
// that still covers every resource verb, matching the FGA grant apikeys mints
// (w1/m9). The PK is (tenant_id, subject), so a rebind to a DIFFERENT tenant
// is not a PK conflict — delete any existing binding for this client first, in
// the same transaction, so a client is bound to at most one tenant.
func (s *PGStore) BindClient(ctx context.Context, clientID, tenantID string) error {
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM tenant_members WHERE subject = $1`, clientID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO tenant_members (tenant_id, subject, role)
			VALUES ($1, $2, 'developer')`, tenantID, clientID)
		return err
	})
	if err != nil {
		return classify("tenant_member", err)
	}
	return nil
}

// UnbindClient removes an API key's tenant_members row across every tenant it
// might be bound to (idempotent — a key that was never bound is not an error).
func (s *PGStore) UnbindClient(ctx context.Context, clientID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM tenant_members WHERE subject = $1`, clientID)
	if err != nil {
		return classify("tenant_member", err)
	}
	return nil // idempotent — a key that was never bound is not an error
}

// FirstDeployGeneration is the release generation a service's create-triggered
// deploy runs under: a freshly created Kubernetes object always starts at
// metadata.generation 1, and the reconciler projects this row into exactly such
// an object. bex-api stamps the same value on the CR's release-generation
// annotation, and the two MUST agree — a row pinned to a release the operator
// never reports reads as superseded and closes canceled (w6/m46 t004).
const FirstDeployGeneration int64 = 1

// maxSlugMintAttempts bounds CreateApp's collision-retry loop for a fresh
// random slug suffix (w4/m19 t002). Each attempt draws from a 36^4 space, so
// exhausting this many is a practical impossibility short of a bug — treat it
// as a real conflict rather than loop forever.
const maxSlugMintAttempts = 5

const slugSuffixAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// randomSlugSuffix mints a 4-char base36 suffix ("-xxxx" once joined), the
// same shape Render's own onrender.com suffix uses
// (docs/render-artifacts/duplicate-service-names.md). Not a resource id — the
// slug is a hostname fragment, so it is minted here rather than through
// internal/id (docs/ADR020-identifiers.md scopes that package to id columns).
func randomSlugSuffix() string {
	b := make([]byte, 4)
	for i := range b {
		b[i] = slugSuffixAlphabet[mathrand.IntN(len(slugSuffixAlphabet))]
	}
	return string(b)
}

// isSlugConflict reports whether err is a unique-violation on apps_slug_idx
// specifically — the signal CreateApp retries on with a suffixed slug, as
// opposed to a tenant_id+name conflict (a real same-workspace duplicate,
// which must surface to the caller, not be silently retried away).
func isSlugConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "apps_slug_idx"
}

// CreateApp inserts the app row and opens its first deploy row (trigger
// "create") in one transaction — every store-managed app has exactly one
// deploy the instant it exists, so ListDeploys is never truthfully empty for
// an app the reconciler is about to project (t001: "creating a service
// records deploy #1"). Bundling a second, related insert into the row's own
// creation is the same precedent CreateTenantWithMember already sets (a
// tenant plus its owner membership, one transaction) — an invariant the type
// itself guarantees is safer here than trusting every future caller to
// remember a second call.
//
// The slug (the globally-unique public subdomain, w4/m19 t002) starts as the
// bare name; a apps_slug_idx collision (some other tenant already claimed
// that bare name) makes this retry with a random "-xxxx" suffix, matching
// Render's own onrender.com behavior. A tenant_id+name collision (the
// caller's own workspace already has this name) is a real conflict and is
// never retried — it classifies straight to ErrConflict.
func (s *PGStore) CreateApp(ctx context.Context, a App) (App, error) {
	a.ID = ids.New(ids.Service)
	a.Slug = a.Name
	for attempt := 0; ; attempt++ {
		deployID := ids.New(ids.Deploy)
		err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
			if err := tx.QueryRow(ctx,
				`INSERT INTO apps (id, tenant_id, name, slug, type, repo, image, registry_credential_id, branch, port, replicas, tier, idle_ttl_seconds, suspended, project_id, environment_id)
				 VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), NULLIF($7,''), $8, $9, $10, $11, $12, $13, $14, NULLIF($15,''), NULLIF($16,''))
				 RETURNING created_at`,
				a.ID, a.TenantID, a.Name, a.Slug, a.Type, a.Repo, a.Image, a.RegistryCredentialID, a.Branch, a.Port, a.Replicas, a.Tier, a.IdleTTLSeconds, a.Suspended, a.ProjectID, a.EnvironmentID,
			).Scan(&a.CreatedAt); err != nil {
				return err
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO deploys (id, app_id, trigger, image, generation, commit, commit_message, status) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				deployID, a.ID, TriggerCreate, a.Image, FirstDeployGeneration, a.FirstDeployCommit.Hash, a.FirstDeployCommit.Message, DeployCreated)
			return err
		})
		if err == nil {
			a.FirstDeployID = deployID
			return a, nil
		}
		if isSlugConflict(err) && attempt < maxSlugMintAttempts {
			a.Slug = a.Name + "-" + randomSlugSuffix()
			continue
		}
		return App{}, classify("app", err)
	}
}

const appColumns = `a.id, a.tenant_id, a.name, a.slug, a.type, COALESCE(a.repo,''), COALESCE(a.image,''), a.registry_credential_id,
	a.branch, a.port, a.replicas, a.tier, a.idle_ttl_seconds, a.suspended,
	COALESCE(a.project_id::text,''), COALESCE(a.environment_id::text,''), a.created_at`

func scanApp(row pgx.Row) (App, error) {
	var a App
	err := row.Scan(&a.ID, &a.TenantID, &a.Name, &a.Slug, &a.Type, &a.Repo, &a.Image,
		&a.RegistryCredentialID, &a.Branch, &a.Port, &a.Replicas, &a.Tier, &a.IdleTTLSeconds, &a.Suspended,
		&a.ProjectID, &a.EnvironmentID, &a.CreatedAt)
	return a, err
}

func (s *PGStore) GetApp(ctx context.Context, id string) (App, error) {
	a, err := scanApp(s.Pool.QueryRow(ctx,
		`SELECT `+appColumns+` FROM apps a WHERE a.id = $1`, id))
	if err != nil {
		return App{}, classify("app", err)
	}
	return a, nil
}

// GetAppProtectedStatus resolves a store-managed App's protectedStatus via
// its Environment (w6/m19, apps.Service's destructive-verb guard,
// apps/protection.go): "unprotected" when the App has no environment_id, or
// its environment's own protected_status column otherwise. "unprotected" is
// the same literal 0024_environment_acl.up.sql defaults protected_status to
// — an App outside any environment behaves exactly like one in a freshly
// created, unprotected environment.
func (s *PGStore) GetAppProtectedStatus(ctx context.Context, appID string) (string, error) {
	var protectedStatus *string
	err := s.Pool.QueryRow(ctx,
		`SELECT e.protected_status FROM apps a LEFT JOIN environments e ON e.id = a.environment_id WHERE a.id = $1`,
		appID,
	).Scan(&protectedStatus)
	if err != nil {
		return "", classify("app", err)
	}
	if protectedStatus == nil {
		return "unprotected", nil
	}
	return *protectedStatus, nil
}

// GetEnvironmentProtectedStatus returns an Environment's protectedStatus.
// Missing, NULL, and empty values are all unprotected: protection is opt-in,
// and a resource may race with deletion of its Environment row.
func (s *PGStore) GetEnvironmentProtectedStatus(ctx context.Context, environmentID string) (string, error) {
	var protectedStatus *string
	err := s.Pool.QueryRow(ctx,
		`SELECT protected_status FROM environments WHERE id = $1`,
		environmentID,
	).Scan(&protectedStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return "unprotected", nil
	}
	if err != nil {
		return "", err
	}
	if protectedStatus == nil || *protectedStatus == "" {
		return "unprotected", nil
	}
	return *protectedStatus, nil
}

func (s *PGStore) ListApps(ctx context.Context) ([]App, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+appColumns+` FROM apps a ORDER BY a.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []App
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *PGStore) DeleteApp(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM apps WHERE id = $1`, id)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app %s: %w", id, ErrNotFound)
	}
	return nil
}

func (s *PGStore) CreateDomain(ctx context.Context, appID, host string, primary bool) (Domain, error) {
	now := time.Now().UTC()
	d := Domain{ID: ids.New(ids.Domain), AppID: appID, Host: host, Primary: primary, ClaimState: "verified", ChallengeVersion: 1, VerifiedAt: &now}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO domains (id, app_id, host, is_primary, claim_state, verified_at)
		 VALUES ($1, $2, $3, $4, 'verified', $5)
		 RETURNING created_at`,
		d.ID, appID, host, primary, now,
	).Scan(&d.CreatedAt)
	if err != nil {
		return Domain{}, classify("domain", err)
	}
	return d, nil
}

const domainClaimColumns = `id, app_id, host, is_primary, COALESCE(redirect_for_name, ''),
	claim_state, COALESCE(challenge, ''), challenge_version, verification_attempts,
	last_verification_at, verified_at, created_at`

func scanDomainClaim(row rowScanner) (Domain, error) {
	var d Domain
	err := row.Scan(&d.ID, &d.AppID, &d.Host, &d.Primary, &d.RedirectForName,
		&d.ClaimState, &d.Challenge, &d.ChallengeVersion, &d.VerificationAttempts,
		&d.LastVerificationAt, &d.VerifiedAt, &d.CreatedAt)
	return d, err
}

func newDomainChallenge() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "bex-domain-verification=" + base64.RawURLEncoding.EncodeToString(raw), nil
}

// AddDomainClaim creates one globally-unique pending claim. A same-App retry
// returns the exact existing row and challenge, updating only redirect intent;
// a claim owned by another App remains a conflict regardless of claim state.
func (s *PGStore) AddDomainClaim(ctx context.Context, appID, host, redirectForName string) (Domain, bool, error) {
	challenge, err := newDomainChallenge()
	if err != nil {
		return Domain{}, false, err
	}
	var redirect any
	if redirectForName != "" {
		redirect = redirectForName
	}
	var d Domain
	created := false
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		// SECURITY (finding-6): expired pending claims are reclaimed atomically
		// before evaluating the global uniqueness conflict. Without expiry a
		// tenant can squat any hostname indefinitely without proving DNS
		// ownership. 48h matches the typical DNS challenge window and prevents
		// indefinite reservation while keeping a fresh pending claim conflicting
		// during its proof window. Verified rows are never reclaimed.
		if _, err := tx.Exec(ctx, `DELETE FROM domains WHERE host = $1 AND claim_state = 'pending' AND created_at < now() - interval '48 hours'`, host); err != nil {
			return err
		}
		var insertErr error
		d, insertErr = scanDomainClaim(tx.QueryRow(ctx,
			`INSERT INTO domains (id, app_id, host, is_primary, redirect_for_name, claim_state, challenge)
			 VALUES ($1, $2, $3, false, $4, 'pending', $5)
			 ON CONFLICT (host) DO NOTHING
			 RETURNING `+domainClaimColumns,
			ids.New(ids.Domain), appID, host, redirect, challenge))
		if insertErr == nil {
			created = true
			return nil
		}
		if !errors.Is(insertErr, pgx.ErrNoRows) {
			return insertErr
		}
		var owner string
		var claimState string
		var createdAt time.Time
		if err := tx.QueryRow(ctx, `SELECT app_id, claim_state, created_at FROM domains WHERE host = $1 FOR UPDATE`, host).Scan(&owner, &claimState, &createdAt); err != nil {
			return err
		}
		if owner != appID {
			// Expired pending owned by another app was already deleted above;
			// a remaining row here is either verified (never reclaim) or fresh
			// pending within the proof window — both remain conflicts.
			return ErrConflict
		}
		d, insertErr = scanDomainClaim(tx.QueryRow(ctx,
			`UPDATE domains SET redirect_for_name = $3 WHERE app_id = $1 AND host = $2 RETURNING `+domainClaimColumns,
			appID, host, redirect))
		return insertErr
	})
	if err != nil {
		return Domain{}, false, classify("domain", err)
	}
	return d, created, nil
}

// GetDomainClaim returns one exact host claim owned by appID.
func (s *PGStore) GetDomainClaim(ctx context.Context, appID, host string) (Domain, error) {
	d, err := scanDomainClaim(s.Pool.QueryRow(ctx,
		`SELECT `+domainClaimColumns+` FROM domains WHERE app_id = $1 AND host = $2`, appID, host))
	if errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, ErrNotFound
	}
	return d, err
}

// ListDomainClaims returns pending and verified claims in stable projection
// order, primary first and then creation order.
func (s *PGStore) ListDomainClaims(ctx context.Context, appID string) ([]Domain, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+domainClaimColumns+` FROM domains WHERE app_id = $1 ORDER BY is_primary DESC, created_at, id`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Domain
	for rows.Next() {
		d, err := scanDomainClaim(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CountWorkspaceDomainClaims returns how many claims (pending and verified) one
// workspace holds across all its apps — the per-workspace custom-domain quota
// check (BEX_MAX_CUSTOM_DOMAINS_PER_WORKSPACE, codex-security round 18),
// mirroring CountGitConnections.
func (s *PGStore) CountWorkspaceDomainClaims(ctx context.Context, workspaceID string) (int, error) {
	var n int
	if err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM domains d JOIN apps a ON a.id = d.app_id WHERE a.tenant_id = $1`, workspaceID,
	).Scan(&n); err != nil {
		return 0, classify("domain", err)
	}
	return n, nil
}

// RecordDomainVerificationAttempt records a conservative failed-or-successful
// resolver attempt without changing ownership state.
func (s *PGStore) RecordDomainVerificationAttempt(ctx context.Context, appID, id string, at time.Time) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE domains SET verification_attempts = verification_attempts + 1, last_verification_at = $3
		 WHERE app_id = $1 AND id = $2 AND claim_state = 'pending'`, appID, id, at)
	return err
}

// PromoteDomainClaim atomically promotes the same pending row that supplied
// expectedChallenge. A concurrent delete/recreate cannot promote the new row.
// Already-verified rows are idempotent only when the id still matches.
func (s *PGStore) PromoteDomainClaim(ctx context.Context, appID, id, expectedChallenge string, at time.Time) (Domain, error) {
	d, err := scanDomainClaim(s.Pool.QueryRow(ctx,
		`UPDATE domains
		 SET claim_state = 'verified', verified_at = $4,
		     verification_attempts = verification_attempts + 1, last_verification_at = $4
		 WHERE app_id = $1 AND id = $2 AND claim_state = 'pending' AND challenge = $3
		 RETURNING `+domainClaimColumns, appID, id, expectedChallenge, at))
	if err == nil {
		return d, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, err
	}
	d, err = scanDomainClaim(s.Pool.QueryRow(ctx,
		`SELECT `+domainClaimColumns+` FROM domains WHERE app_id = $1 AND id = $2`, appID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, ErrConflict
	}
	if err != nil {
		return Domain{}, err
	}
	if d.ClaimState != "verified" {
		return Domain{}, ErrConflict
	}
	return d, nil
}

// ReplaceDomainClaims atomically reconciles a declaration set while preserving
// unchanged rows' ownership evidence. Newly introduced hosts start pending.
func (s *PGStore) ReplaceDomainClaims(ctx context.Context, appID string, declarations []DomainDeclaration) ([]Domain, error) {
	seen := make(map[string]struct{}, len(declarations))
	for _, declaration := range declarations {
		if declaration.Host == "" {
			continue
		}
		if _, exists := seen[declaration.Host]; exists {
			return nil, fmt.Errorf("domain: %w", ErrInvalid)
		}
		seen[declaration.Host] = struct{}{}
	}
	var out []Domain
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var locked string
		if err := tx.QueryRow(ctx, `SELECT id FROM apps WHERE id = $1 FOR UPDATE`, appID).Scan(&locked); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE domains SET is_primary = false WHERE app_id = $1`, appID); err != nil {
			return err
		}
		hosts := make([]string, 0, len(seen))
		for _, declaration := range declarations {
			if declaration.Host == "" {
				continue
			}
			hosts = append(hosts, declaration.Host)
			challenge, err := newDomainChallenge()
			if err != nil {
				return err
			}
			var redirect any
			if declaration.RedirectForName != "" {
				redirect = declaration.RedirectForName
			}
			tag, err := tx.Exec(ctx,
				`INSERT INTO domains (id, app_id, host, is_primary, redirect_for_name, claim_state, challenge)
				 VALUES ($1, $2, $3, $4, $5, 'pending', $6)
				 ON CONFLICT (host) DO UPDATE
				 SET is_primary = EXCLUDED.is_primary, redirect_for_name = EXCLUDED.redirect_for_name
				 WHERE domains.app_id = EXCLUDED.app_id`,
				ids.New(ids.Domain), appID, declaration.Host, declaration.Primary, redirect, challenge)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrConflict
			}
		}
		if len(hosts) == 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM domains WHERE app_id = $1`, appID); err != nil {
				return err
			}
		} else if _, err := tx.Exec(ctx,
			`DELETE FROM domains WHERE app_id = $1 AND NOT (host = ANY($2::text[]))`, appID, hosts); err != nil {
			return err
		}
		rows, err := tx.Query(ctx,
			`SELECT `+domainClaimColumns+` FROM domains WHERE app_id = $1 ORDER BY is_primary DESC, created_at, id`, appID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scanDomainClaim(rows)
			if err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, classify("domain", err)
	}
	return out, nil
}

func (s *PGStore) DeleteDomain(ctx context.Context, appID, host string) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM domains WHERE app_id = $1 AND host = $2`, appID, host)
	if err != nil {
		return classify("domain", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("domain: %w", ErrNotFound)
	}
	return nil
}

// AddDomain appends or updates a non-primary domain row for apps.IntentStore.
// Re-adding a host this app already owns updates redirect_for_name (used when an
// auto-paired sibling becomes explicit); another app owning it is a real
// cross-app conflict.
func (s *PGStore) AddDomain(ctx context.Context, appID, host, redirectForName string) error {
	d := Domain{ID: ids.New(ids.Domain), AppID: appID, Host: host, RedirectForName: redirectForName}
	var redirect any
	if redirectForName != "" {
		redirect = redirectForName
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO domains (id, app_id, host, is_primary, redirect_for_name, claim_state, verified_at)
		 VALUES ($1, $2, $3, false, $4, 'verified', now())`, d.ID, appID, host, redirect)
	if err == nil {
		return nil
	}
	err = classify("domain", err)
	if errors.Is(err, ErrConflict) {
		if owner, ok, e := s.domainOwner(ctx, host); e == nil && ok && owner == appID {
			_, e = s.Pool.Exec(ctx,
				`UPDATE domains SET redirect_for_name = $3 WHERE app_id = $1 AND host = $2`,
				appID, host, redirect)
			return e
		}
		return ErrConflict
	}
	return err
}

// domainOwner returns the app id that owns host, or ok=false if the host is
// unclaimed. Backs AddDomain's same-app-vs-cross-app conflict decision.
func (s *PGStore) domainOwner(ctx context.Context, host string) (string, bool, error) {
	var appID string
	err := s.Pool.QueryRow(ctx, `SELECT app_id FROM domains WHERE host = $1`, host).Scan(&appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return appID, true, nil
}

// RemoveDomain deletes a domain row for apps.IntentStore — idempotent
// (not-found silently ignored).
func (s *PGStore) RemoveDomain(ctx context.Context, appID, host string) error {
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE domains SET redirect_for_name = NULL WHERE app_id = $1 AND redirect_for_name = $2`, appID, host); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM domains WHERE app_id = $1 AND host = $2`, appID, host)
		return err
	})
	return classify("domain", err)
}

// ReplaceDomains atomically reconciles one App's complete domain set. Locking
// the App row serializes concurrent re-syncs of the same service; the global
// UNIQUE(domains.host) index arbitrates claims between different services.
// Any conflict rolls the transaction back, preserving the former set.
func (s *PGStore) ReplaceDomains(ctx context.Context, appID, primary string, hosts []string) error {
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var locked string
		if err := tx.QueryRow(ctx, `SELECT id FROM apps WHERE id = $1 FOR UPDATE`, appID).Scan(&locked); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM domains WHERE app_id = $1`, appID); err != nil {
			return err
		}
		insert := func(host string, primary bool) error {
			if host == "" {
				return nil
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO domains (id, app_id, host, is_primary, claim_state, verified_at)
				 VALUES ($1, $2, $3, $4, 'verified', now())`,
				ids.New(ids.Domain), appID, host, primary)
			return err
		}
		if err := insert(primary, true); err != nil {
			return err
		}
		for _, host := range hosts {
			if err := insert(host, false); err != nil {
				return err
			}
		}
		return nil
	})
	return classify("domain", err)
}

func (s *PGStore) ListDesiredApps(ctx context.Context) ([]DesiredApp, error) {
	// LEFT JOIN environments so a member row carries its environment's rule
	// list (w4/m28): projectSpec projects the inbound-IP layer onto a CR the
	// projector (re)creates, keeping the fallback-create path consistent with
	// the environments fan-out.
	rows, err := s.Pool.Query(ctx,
		`SELECT `+appColumns+`, t.name, e.ip_allow_list FROM apps a JOIN tenants t ON t.id = a.tenant_id
		 LEFT JOIN environments e ON e.id = a.environment_id ORDER BY a.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DesiredApp
	index := map[string]int{}
	for rows.Next() {
		var d DesiredApp
		var envRules []byte
		err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Slug, &d.Type, &d.Repo, &d.Image,
			&d.RegistryCredentialID, &d.Branch, &d.Port, &d.Replicas, &d.Tier, &d.IdleTTLSeconds, &d.Suspended,
			&d.ProjectID, &d.EnvironmentID, &d.CreatedAt, &d.TenantName, &envRules)
		if err != nil {
			return nil, err
		}
		if d.EnvironmentID != "" && envRules != nil {
			var entries []core.IPAllowListEntry
			if err := json.Unmarshal(envRules, &entries); err != nil {
				return nil, err
			}
			d.EnvironmentIPAllowList = core.EnvironmentLayerCIDRs(entries)
		}
		index[d.ID] = len(out)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Attach the primary domain to PrimaryHost (projection's spec.host), keep
	// non-primary domains in Hosts, and carry redirect metadata for either kind.
	drows, err := s.Pool.Query(ctx,
		`SELECT d.app_id, d.host, d.is_primary, COALESCE(d.redirect_for_name, '')
		 FROM domains d
		 WHERE d.claim_state = 'verified'
		   AND (d.redirect_for_name IS NULL OR EXISTS (
		       SELECT 1 FROM domains target
		       WHERE target.app_id = d.app_id
		         AND target.host = d.redirect_for_name
		         AND target.claim_state = 'verified'))
		 ORDER BY d.is_primary DESC, d.created_at`)
	if err != nil {
		return nil, err
	}
	defer drows.Close()
	for drows.Next() {
		var appID, host, redirectForName string
		var primary bool
		if err := drows.Scan(&appID, &host, &primary, &redirectForName); err != nil {
			return nil, err
		}
		if i, ok := index[appID]; ok {
			if primary {
				out[i].PrimaryHost = host
			} else {
				out[i].Hosts = append(out[i].Hosts, host)
			}
			if redirectForName != "" {
				if out[i].HostRedirects == nil {
					out[i].HostRedirects = map[string]string{}
				}
				out[i].HostRedirects[host] = redirectForName
			}
		}
	}
	if err := drows.Err(); err != nil {
		return nil, err
	}
	// Attach each service's live disk (ADR082). Only live rows are selected, so
	// a soft-deleted disk projects as absent, which is what detaches the volume.
	krows, err := s.Pool.Query(ctx,
		`SELECT `+diskColumns+` FROM service_disks WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer krows.Close()
	for krows.Next() {
		disk, err := scanDisk(krows)
		if err != nil {
			return nil, err
		}
		if i, ok := index[disk.AppID]; ok {
			out[i].Disk = &disk
		}
	}
	return out, krows.Err()
}

const diskColumns = `id, tenant_id, app_id, name, mount_path, size_gb, created_at, updated_at, deleted_at`

func scanDisk(row rowScanner) (Disk, error) {
	var d Disk
	err := row.Scan(&d.ID, &d.TenantID, &d.AppID, &d.Name, &d.MountPath, &d.SizeGB,
		&d.CreatedAt, &d.UpdatedAt, &d.DeletedAt)
	return d, err
}

// CreateDisk inserts the disk and opens its first size period in one
// transaction, so a disk can never exist unmetered or be metered before it
// exists.
func (s *PGStore) CreateDisk(ctx context.Context, tenantID, appID, name, mountPath string, sizeGB int32) (Disk, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Disk{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	diskID := ids.New(ids.Disk)
	row := tx.QueryRow(ctx,
		`INSERT INTO service_disks (id, tenant_id, app_id, name, mount_path, size_gb)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+diskColumns,
		diskID, tenantID, appID, name, mountPath, sizeGB)
	disk, err := scanDisk(row)
	if err != nil {
		// The partial unique index on a live disk per app surfaces a second
		// disk as a duplicate-key violation, which classify maps to conflict.
		return Disk{}, classify("disk", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO service_disk_sizes (disk_id, size_gb, from_ts) VALUES ($1, $2, $3)`,
		disk.ID, sizeGB, disk.CreatedAt); err != nil {
		// from_ts is the row's own created_at (also the DB clock), so the first
		// period starts exactly when the disk starts existing.
		return Disk{}, classify("disk", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Disk{}, err
	}
	return disk, nil
}

func (s *PGStore) GetDisk(ctx context.Context, id string) (Disk, error) {
	disk, err := scanDisk(s.Pool.QueryRow(ctx,
		`SELECT `+diskColumns+` FROM service_disks WHERE id = $1 AND deleted_at IS NULL`, id))
	if err != nil {
		return Disk{}, classify("disk", err)
	}
	return disk, nil
}

func (s *PGStore) ListDisks(ctx context.Context, tenantID, appID string) ([]Disk, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+diskColumns+` FROM service_disks
		 WHERE tenant_id = $1 AND ($2 = '' OR app_id = $2) AND deleted_at IS NULL
		 ORDER BY created_at DESC, id`, tenantID, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Disk{}
	for rows.Next() {
		disk, err := scanDisk(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, disk)
	}
	return out, rows.Err()
}

// UpdateDisk applies the mutable fields. A size change closes the current
// period and opens a new one at the same instant, so the meter integrates the
// old size right up to the grow and the new size from it — a disk grown
// mid-hour bills both, which is what makes the invoice reproducible.
func (s *PGStore) UpdateDisk(ctx context.Context, id string, name, mountPath *string, sizeGB *int32) (Disk, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Disk{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanDisk(tx.QueryRow(ctx,
		`SELECT `+diskColumns+` FROM service_disks WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id))
	if err != nil {
		return Disk{}, classify("disk", err)
	}
	if sizeGB != nil && *sizeGB < current.SizeGB {
		return Disk{}, fmt.Errorf("disk: %w: size can only grow (current %dGB, requested %dGB)",
			ErrInvalid, current.SizeGB, *sizeGB)
	}

	// Every period boundary comes from the DATABASE clock, never Go's. The two
	// clocks can disagree by more than the gap between a create and a grow, and
	// a to_ts behind its own from_ts is both a constraint violation and, worse,
	// negative billed time. Inside one transaction now() is the transaction
	// timestamp, so the close and the open land on the same instant exactly.
	if sizeGB != nil && *sizeGB > current.SizeGB {
		if _, err := tx.Exec(ctx,
			`UPDATE service_disk_sizes SET to_ts = now() WHERE disk_id = $1 AND to_ts IS NULL`,
			id); err != nil {
			return Disk{}, err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO service_disk_sizes (disk_id, size_gb, from_ts) VALUES ($1, $2, now())`,
			id, *sizeGB); err != nil {
			return Disk{}, err
		}
	}
	updated, err := scanDisk(tx.QueryRow(ctx,
		`UPDATE service_disks
		    SET name = COALESCE($2, name),
		        mount_path = COALESCE($3, mount_path),
		        size_gb = GREATEST(size_gb, COALESCE($4, size_gb)),
		        updated_at = now()
		  WHERE id = $1 AND deleted_at IS NULL
		 RETURNING `+diskColumns, id, name, mountPath, sizeGB))
	if err != nil {
		return Disk{}, classify("disk", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Disk{}, err
	}
	return updated, nil
}

// DeleteDisk soft-deletes and closes the open size period in one transaction:
// the instant billing stops is the same instant the volume is released.
func (s *PGStore) DeleteDisk(ctx context.Context, id string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE service_disks SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		id)
	if err != nil {
		return classify("disk", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("disk: %w", ErrNotFound)
	}
	// Same transaction clock as the deletion above, so the moment billing stops
	// is exactly the moment the volume is released.
	if _, err := tx.Exec(ctx,
		`UPDATE service_disk_sizes SET to_ts = now() WHERE disk_id = $1 AND to_ts IS NULL`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DiskUsageForWindow returns provisioned disk GB-seconds per service for the
// half-open window [from, to).
//
// It integrates the size periods rather than sampling a current size, so a
// disk grown mid-window contributes its old size up to the change and its new
// size after it, and a disk deleted mid-window contributes only up to the
// deletion. Because it reads committed control-plane rows, the same window
// recomputed later yields the same number — which is what lets the invoice be
// re-derived, and what makes the meter independent of the app cluster.
func (s *PGStore) DiskUsageForWindow(ctx context.Context, from, to time.Time) ([]DiskUsageRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT d.tenant_id, d.app_id,
		        SUM(s.size_gb * EXTRACT(EPOCH FROM (
		            LEAST(COALESCE(s.to_ts, $2), $2) - GREATEST(s.from_ts, $1)
		        )))::bigint
		   FROM service_disk_sizes s
		   JOIN service_disks d ON d.id = s.disk_id
		  WHERE s.from_ts < $2 AND (s.to_ts IS NULL OR s.to_ts > $1)
		  GROUP BY d.tenant_id, d.app_id`, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DiskUsageRow{}
	for rows.Next() {
		var r DiskUsageRow
		if err := rows.Scan(&r.TenantID, &r.AppID, &r.GBSeconds); err != nil {
			return nil, err
		}
		if r.GBSeconds < 0 {
			r.GBSeconds = 0
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LatestUsageWindowForKind is the newest window any resource has recorded for
// one meter kind — the cursor for a meter whose rows are computed for every
// resource at once rather than resource by resource.
func (s *PGStore) LatestUsageWindowForKind(ctx context.Context, kind string) (time.Time, bool, error) {
	var window *time.Time
	if err := s.Pool.QueryRow(ctx,
		`SELECT MAX(window_start) FROM usage_hourly WHERE kind = $1`, kind).Scan(&window); err != nil {
		return time.Time{}, false, err
	}
	if window == nil {
		return time.Time{}, false, nil
	}
	return window.UTC(), true, nil
}

func (s *PGStore) SetAppSuspended(ctx context.Context, id string, suspended bool) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET suspended = $2, updated_at = now() WHERE id = $1`,
		id, suspended)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

// SetAppTier updates the row's tier (the apps feature's plan-change verb
// validates it against lego/types/tiers before calling this). The projector
// carries it onto spec.tier the same way it carries suspended.
func (s *PGStore) SetAppTier(ctx context.Context, id string, tier string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET tier = $2, updated_at = now() WHERE id = $1`,
		id, tier)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

// SetAppReplicas updates the row's replica count (the apps feature's scale
// verb validates the bound before calling this). The projector carries it
// onto spec.replicas the same way it carries suspended/tier.
func (s *PGStore) SetAppReplicas(ctx context.Context, id string, replicas int32) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET replicas = $2, updated_at = now() WHERE id = $1`,
		id, replicas)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

// SetAppIdleTTL updates the row's idle-TTL seconds (the apps feature's
// idle-timeout verb validates the bound before calling this). The projector
// carries it onto spec.idleTTLSeconds the same way it carries replicas.
func (s *PGStore) SetAppIdleTTL(ctx context.Context, id string, seconds int32) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET idle_ttl_seconds = $2, updated_at = now() WHERE id = $1`,
		id, seconds)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

// SetAppSource atomically updates repo/image/branch and the explicit registry
// credential binding so the projector never observes mismatched source/auth.
func (s *PGStore) SetAppSource(ctx context.Context, id, repo, image, branch string, registryCredentialID *string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET repo = NULLIF($2, ''), image = NULLIF($3, ''), branch = $4, registry_credential_id = $5, updated_at = now() WHERE id = $1`,
		id, repo, image, branch, registryCredentialID)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

// BackfillAppType heals a pre-w6/m46 row that predates the type column. Only
// matching an empty type is the whole safety story: it makes the write
// idempotent, race-free between concurrent projector passes, and incapable of
// changing a type the create path already recorded (spec.type is immutable, so
// a disagreement would be corruption, not an update).
func (s *PGStore) BackfillAppType(ctx context.Context, id, serviceType string) (bool, error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET type = $2, updated_at = now() WHERE id = $1 AND type = ''`,
		id, serviceType)
	if err != nil {
		return false, classify("app", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SetAppEnvironment atomically assigns an App row to an Environment and its
// owning Project. Empty ids clear both memberships (Blueprint ungrouped:).
func (s *PGStore) SetAppEnvironment(ctx context.Context, id, projectID, environmentID string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET project_id = NULLIF($2, ''), environment_id = NULLIF($3, ''), updated_at = now() WHERE id = $1`,
		id, projectID, environmentID)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

// SetAppImage updates the row's image (the deploys feature's Rollback verb,
// w2/m10). The projector carries it onto spec.image the same way it carries
// replicas/tier — the write-through-store discipline every intent field with
// a row as its single writer of truth follows.
func (s *PGStore) SetAppImage(ctx context.Context, id string, image string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET image = NULLIF($2, ''), updated_at = now() WHERE id = $1`,
		id, image)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

func (s *PGStore) CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit CommitInfo) (Deploy, error) {
	d := Deploy{ID: ids.New(ids.Deploy), AppID: appID, Trigger: trigger, Image: image, Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message, CommitAuthorAt: commit.AuthorAt, Status: DeployCreated}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		status, err := prepareDeployCreate(ctx, tx, appID, generation)
		if err != nil {
			return err
		}
		d.Status = status
		d.OverlapPending = status == DeployQueued
		return tx.QueryRow(ctx,
			`INSERT INTO deploys (id, app_id, trigger, image, generation, commit, commit_message, commit_author_at, status, overlap_pending, finished_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, CASE WHEN $9 = $11 THEN clock_timestamp() END)
			 RETURNING created_at, updated_at, finished_at`,
			d.ID, d.AppID, d.Trigger, d.Image, d.Generation, d.Commit, d.CommitMessage, d.CommitAuthorAt, d.Status, d.OverlapPending, DeployCanceled,
		).Scan(&d.CreatedAt, &d.UpdatedAt, &d.FinishedAt)
	})
	if err != nil {
		return Deploy{}, classify("deploy", err)
	}
	return d, nil
}

// prepareDeployCreate serializes creation for one App, then compares App
// generations before choosing the visible initial status. A delayed request
// for an older generation records itself canceled without disturbing the
// higher-generation open row. Otherwise the new generation replaces only the
// previous overlap-pending row: it starts queued while an active deploy is
// running, or created when the active slot is free. Partial unique indexes are
// the final one-active-plus-one-pending guard.
func prepareDeployCreate(ctx context.Context, tx pgx.Tx, appID string, generation int64) (string, error) {
	var lockedID string
	if err := tx.QueryRow(ctx, `SELECT id FROM apps WHERE id = $1 FOR UPDATE`, appID).Scan(&lockedID); err != nil {
		return "", err
	}
	var superseded bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM deploys
			WHERE app_id = $1 AND status = ANY($2) AND generation >= $3
		)`, appID, openDeployStatuses, generation,
	).Scan(&superseded); err != nil {
		return "", err
	}
	if superseded {
		return DeployCanceled, nil
	}
	if err := cancelPendingDeploys(ctx, tx, appID); err != nil {
		return "", err
	}
	var active bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM deploys
			WHERE app_id = $1 AND status = ANY($2) AND NOT overlap_pending
		)`, appID, openDeployStatuses,
	).Scan(&active); err != nil {
		return "", err
	}
	if active {
		return DeployQueued, nil
	}
	return DeployCreated, nil
}

// cancelPendingDeploys coalesces overlapping triggers into one latest-pending
// slot without preempting the deploy already executing. Keeping cancellation
// and insert in one App-row-locked transaction makes concurrent triggers
// deterministic.
func cancelPendingDeploys(ctx context.Context, tx pgx.Tx, appID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE deploys
		 SET status = $2,
		     updated_at = GREATEST(updated_at + interval '1 microsecond', clock_timestamp()),
		     finished_at = clock_timestamp(),
		     overlap_pending = false
		 WHERE app_id = $1 AND status = $3 AND overlap_pending`,
		appID, DeployCanceled, DeployQueued)
	return err
}

// CreateRollbackDeploy opens a "rollback"-triggered deploy row (w2/m10):
// unlike a normal trigger, the restored image is already fully known (it ran
// live before), so resolved_image is set immediately rather than waiting for
// the reconciler's write-back to backfill it on convergence. commit is the
// TARGET deploy's commit metadata (w9/001) — a rollback re-runs what the
// restored deploy built, so its provenance is copied, never re-resolved
// against a branch that has since moved.
func (s *PGStore) CreateRollbackDeploy(ctx context.Context, appID, image, rollbackOf string, generation int64, commit CommitInfo) (Deploy, error) {
	d := Deploy{ID: ids.New(ids.Deploy), AppID: appID, Trigger: TriggerRollback, Image: image, ResolvedImage: image, RollbackOf: rollbackOf, Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message, CommitAuthorAt: commit.AuthorAt, Status: DeployCreated}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		status, err := prepareDeployCreate(ctx, tx, appID, generation)
		if err != nil {
			return err
		}
		d.Status = status
		d.OverlapPending = status == DeployQueued
		return tx.QueryRow(ctx,
			`INSERT INTO deploys (id, app_id, trigger, image, resolved_image, rollback_of, generation, commit, commit_message, commit_author_at, status, overlap_pending, finished_at)
			 VALUES ($1, $2, $3, $4, $4, $5, $6, $7, $8, $9, $10, $11, CASE WHEN $10 = $12 THEN clock_timestamp() END)
			 RETURNING created_at, updated_at, finished_at`,
			d.ID, d.AppID, d.Trigger, image, rollbackOf, generation, d.Commit, d.CommitMessage, d.CommitAuthorAt, d.Status, d.OverlapPending, DeployCanceled,
		).Scan(&d.CreatedAt, &d.UpdatedAt, &d.FinishedAt)
	})
	if err != nil {
		return Deploy{}, classify("deploy", err)
	}
	return d, nil
}

// DeployFilter narrows ListDeploys using Render's status and exclusive
// created/updated/finished time bounds. Cursor resumes strictly after a
// previously returned deploy's id (newest-first keyset paging, stable under
// concurrent inserts); an unknown cursor yields an empty page. Limit is
// clamped here by clampPageLimit: <=0 (absent) and >core.MaxPageLimit both
// become core.MaxPageLimit — no query runs uncapped.
type DeployFilter struct {
	Statuses       []string
	CreatedBefore  time.Time
	CreatedAfter   time.Time
	UpdatedBefore  time.Time
	UpdatedAfter   time.Time
	FinishedBefore time.Time
	FinishedAfter  time.Time
	Cursor         string
	Limit          int
}

// clampPageLimit is the store's page-size invariant for the history lists
// (DeployFilter, JobListFilter): every query gets a LIMIT. An absent limit
// (<=0) becomes core.MaxPageLimit — the pre-m31 "omitted limit = full
// history" contract let any service viewer materialize an unbounded history
// (DB + memory + JSON + per-row k8s sync work, codex-security round-6 #7);
// callers wanting more page with the keyset cursor. Anything else caps at
// core.MaxPageLimit. The store is the enforcing layer — feature translators
// (deploys.FilterOf) pass the caller's value through rather than each
// keeping their own copy of the clamp.
func clampPageLimit(n int) int {
	if n <= 0 {
		return core.MaxPageLimit
	}
	return min(n, core.MaxPageLimit)
}

// pageKeyset appends the keyset-pagination tail shared by the store's list
// queries (ListDeploys, ListJobs, ListAuditEvents): the cursor resume clause,
// the (sortCol, id) total order — the id tiebreak makes the order total, so
// paging can never skip or repeat two rows created in the same instant — and
// the LIMIT (skipped when limit <= 0, the unbounded case). oldestFirst=false
// is newest-first (DESC, the default everywhere); oldestFirst=true is the
// mirror (ASC) — Render's direction=forward on audit logs (w4/013). The
// cursor resumes strictly PAST the cursor row's own (sortCol, id) in the
// paging direction — the same cursor id resumes older rows newest-first and
// newer rows oldest-first; an unknown cursor id matches no subquery row, so
// the comparison is NULL (never true) — an invalid cursor yields an empty
// page, not a crash or a leak of the unfiltered list.
func pageKeyset(query string, args []any, table, sortCol, cursor string, limit int, oldestFirst bool) (string, []any) {
	cmp, ord := "<", "DESC"
	if oldestFirst {
		cmp, ord = ">", "ASC"
	}
	if cursor != "" {
		args = append(args, cursor)
		query += fmt.Sprintf(" AND (%s, id) %s (SELECT %s, id FROM %s WHERE id = $%d)", sortCol, cmp, sortCol, table, len(args))
	}
	query += fmt.Sprintf(" ORDER BY %s %s, id %s", sortCol, ord, ord)
	if limit > 0 {
		args = append(args, limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	return query, args
}

const deployColumns = `id, app_id, trigger, image, resolved_image, rollback_of, generation, commit, commit_message, commit_author_at, status, overlap_pending, created_at, updated_at, started_at, finished_at, pre_deploy_status, failure_reason`

func scanDeploy(row pgx.Row) (Deploy, error) {
	var d Deploy
	err := row.Scan(&d.ID, &d.AppID, &d.Trigger, &d.Image, &d.ResolvedImage, &d.RollbackOf, &d.Generation, &d.Commit, &d.CommitMessage, &d.CommitAuthorAt, &d.Status, &d.OverlapPending, &d.CreatedAt, &d.UpdatedAt, &d.StartedAt, &d.FinishedAt, &d.PreDeployStatus, &d.FailureReason)
	return d, err
}

func (s *PGStore) ListDeploys(ctx context.Context, appID string, filter DeployFilter) ([]Deploy, error) {
	query := `SELECT ` + deployColumns + ` FROM deploys WHERE app_id = $1`
	args := []any{appID}
	if len(filter.Statuses) > 0 {
		args = append(args, filter.Statuses)
		query += fmt.Sprintf(" AND status = ANY($%d)", len(args))
	}
	if !filter.CreatedAfter.IsZero() {
		args = append(args, filter.CreatedAfter)
		query += fmt.Sprintf(" AND created_at > $%d", len(args))
	}
	if !filter.CreatedBefore.IsZero() {
		args = append(args, filter.CreatedBefore)
		query += fmt.Sprintf(" AND created_at < $%d", len(args))
	}
	if !filter.UpdatedAfter.IsZero() {
		args = append(args, filter.UpdatedAfter)
		query += fmt.Sprintf(" AND updated_at > $%d", len(args))
	}
	if !filter.UpdatedBefore.IsZero() {
		args = append(args, filter.UpdatedBefore)
		query += fmt.Sprintf(" AND updated_at < $%d", len(args))
	}
	if !filter.FinishedAfter.IsZero() {
		args = append(args, filter.FinishedAfter)
		query += fmt.Sprintf(" AND finished_at > $%d", len(args))
	}
	if !filter.FinishedBefore.IsZero() {
		args = append(args, filter.FinishedBefore)
		query += fmt.Sprintf(" AND finished_at < $%d", len(args))
	}
	query, args = pageKeyset(query, args, "deploys", "created_at", filter.Cursor, clampPageLimit(filter.Limit), false)
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deploy
	for rows.Next() {
		d, err := scanDeploy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PGStore) GetDeploy(ctx context.Context, appID, deployID string) (Deploy, error) {
	d, err := scanDeploy(s.Pool.QueryRow(ctx,
		`SELECT `+deployColumns+` FROM deploys WHERE id = $1 AND app_id = $2`, deployID, appID))
	if err != nil {
		return Deploy{}, classify("deploy", err)
	}
	return d, nil
}

func (s *PGStore) ListOpenDeploys(ctx context.Context) ([]Deploy, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+deployColumns+` FROM deploys WHERE status = ANY($1)
		 ORDER BY app_id, generation, created_at, id`, openDeployStatuses)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deploy
	for rows.Next() {
		d, err := scanDeploy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PGStore) TransitionDeploy(ctx context.Context, id, status, resolvedImage, failureReason, failureCode string) (bool, error) {
	transitioned := false
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var appID, current string
		var overlapPending bool
		if err := tx.QueryRow(ctx,
			`SELECT app_id, status, overlap_pending FROM deploys WHERE id = $1 FOR UPDATE`, id,
		).Scan(&appID, &current, &overlapPending); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		// Adoption can be a real internal transition without a wire-status
		// change: the latest overlap was already exposed as queued, and the
		// operator has now advanced to that release while it still reports
		// BuildQueued. Free the pending slot without churning updated_at.
		if current == status && current == DeployQueued && overlapPending {
			tag, err := tx.Exec(ctx, `UPDATE deploys SET overlap_pending = false WHERE id = $1 AND overlap_pending`, id)
			if err != nil {
				return err
			}
			transitioned = tag.RowsAffected() > 0
			return nil
		}
		if !CanTransitionDeploy(current, status) {
			return nil
		}

		starts := DeployStatusStartsExecution(status)
		terminal := IsTerminalDeployStatus(status)
		if _, err := tx.Exec(ctx,
			`UPDATE deploys
			 SET status = $2,
			     overlap_pending = CASE WHEN $2 = $7 THEN overlap_pending ELSE false END,
			     resolved_image = COALESCE(NULLIF($3, ''), resolved_image),
			     failure_reason = COALESCE(NULLIF($6, ''), failure_reason),
			     started_at = CASE WHEN $4 THEN COALESCE(started_at, clock_timestamp()) ELSE started_at END,
			     finished_at = CASE WHEN $5 THEN COALESCE(finished_at, clock_timestamp()) ELSE finished_at END,
			     updated_at = GREATEST(updated_at + interval '1 microsecond', clock_timestamp())
			 WHERE id = $1`,
			id, status, resolvedImage, starts, terminal, failureReason, DeployQueued); err != nil {
			return err
		}
		if status == DeployLive {
			if _, err := tx.Exec(ctx,
				`UPDATE deploys
				 SET status = $3,
				     updated_at = GREATEST(updated_at + interval '1 microsecond', clock_timestamp())
				 WHERE app_id = $1 AND id <> $2 AND status = $4`,
				appID, id, DeployDeactivated, DeployLive); err != nil {
				return err
			}
		}
		if failureCode == EventReasonImagePullBackoff {
			if _, err := tx.Exec(ctx,
				`INSERT INTO service_event_facts (
				     source_key, app_id, fact_type, at, deploy_id, image, reason_code
				 )
				 SELECT 'deploy:' || id || ':image_pull_failed', app_id, $2,
				        COALESCE(finished_at, clock_timestamp()), id, image, $3
				 FROM deploys WHERE id = $1
				 ON CONFLICT (source_key) DO NOTHING`,
				id, EventFactImagePullFailed, EventReasonImagePullBackoff); err != nil {
				return err
			}
		}
		transitioned = true
		return nil
	})
	return transitioned, err
}

func (s *PGStore) CloseDeploy(ctx context.Context, id, status, resolvedImage string) (bool, error) {
	if !IsTerminalDeployStatus(status) || status == DeployDeactivated {
		return false, nil
	}
	return s.TransitionDeploy(ctx, id, status, resolvedImage, "", "")
}

// SetDeployPreDeployStatus records the pre-deploy step's outcome on a deploy
// row (w1/m33), no-op when unchanged (IS DISTINCT FROM guards the write so the
// reconciler's every-pass projection doesn't churn the row). Returns whether a
// row was updated.
func (s *PGStore) SetDeployPreDeployStatus(ctx context.Context, id, status string) (bool, error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE deploys
		 SET pre_deploy_status = $2,
		     updated_at = GREATEST(updated_at + interval '1 microsecond', clock_timestamp())
		 WHERE id = $1 AND status = ANY($3) AND pre_deploy_status IS DISTINCT FROM $2`,
		id, status, openDeployStatuses)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// classify maps Postgres errors to the shared taxonomy: unique violations are
// conflicts, FK violations mean the referenced parent doesn't exist, and
// check violations are caller errors. (Ids are plain text, so a malformed id
// simply matches no rows — ErrNoRows covers it.)
func classify(entity string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", entity, ErrNotFound)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%s: %w", entity, ErrConflict)
		case "23503": // foreign_key_violation — referenced tenant/app is gone
			return fmt.Errorf("%s reference: %w", entity, ErrNotFound)
		case "23514": // check_violation
			return fmt.Errorf("%s: %w: %s", entity, ErrInvalid, pgErr.ConstraintName)
		}
	}
	return err
}
