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
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Service turns user intent (restart / suspend / resume) into App CR spec
// patches, and reads Apps as the Render service shape. It is a thin policy layer
// — the operator does the mechanism.
type Service struct {
	*core.Base
	// BaseDomain is the platform wildcard domain (BEX_BASE_DOMAIN, e.g. "onbex.co")
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
	// Store is the Postgres source of truth for store-managed Apps (those carrying
	// the bex.co/app-id label). Suspend/Resume write the row first — the row owns
	// spec.suspended, and the projection loop reverts CR patches it didn't
	// originate — then patch the CR as the fast-converge path. Nil (tests, DB-less
	// mode) falls back to CR-only patches, safe only for hand-applied Apps.
	Store IntentStore
	// Selections is the shared MCP per-session workspace selection (w6/m2/t005):
	// list_services falls back to the caller's selected workspace when its
	// ownerId argument is omitted. Read-only (apps never selects a workspace,
	// only workspaces.Service's select_workspace does). Nil => no fallback.
	Selections core.WorkspaceSelectionReader
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
	// MaxServices, when positive, caps how many services a workspace may own.
	// Enforced on new-service creates only (not redeploys of existing services).
	// 0 = unlimited (the default; byte-identical to before). Only enforced when
	// the caller's tenant is resolvable (store on + bound caller) — a store-off
	// operator or an unbound caller skips the check, consistent with the
	// per-workspace design (w7/m9).
	MaxServices int
	// Blueprints, when set (the control-plane store is wired), persists blueprint
	// rows (w2/m15): auto-upserted on every repo-backed deploy, and queried by the
	// list/sync verbs. nil => list/sync return ErrBlueprintsUnavailable; validate
	// is always available (stateless).
	Blueprints BlueprintStore
	// SecretsEraser, when set, purges the app's OpenBao env-var and secret-file
	// paths on delete. nil => OpenBao paths are not purged on service delete
	// (they are purged on workspace delete via WorkspacePurger). Satisfied
	// structurally by *secrets.WorkspacePurger so apps never imports secrets.
	SecretsEraser AppSecretsEraser
	// EnvGroups, when set (OpenBao is wired), materializes a bex.yml's
	// envVarGroups: and links them to services via fromGroup (w1/m35), riding the
	// env-groups feature through a narrow seam. nil => a manifest using
	// envVarGroups/fromGroup is rejected before any write (env groups unavailable),
	// never silently dropped.
	EnvGroups EnvGroupApplier
	// EnvSeeder, when set (OpenBao is wired), seeds a bex.yml's sync:false and
	// generateValue vars into the mutable env-vars store SEED-ONCE (w1/m35), so a
	// later dashboard edit wins and a re-sync never overwrites/re-mints. nil => a
	// manifest using those forms is rejected before any write.
	EnvSeeder EnvSeeder
}

// AppSecretsEraser clears per-app secrets from the external store on service
// delete. Satisfied structurally by *secrets.WorkspacePurger.
type AppSecretsEraser interface {
	PurgeApp(ctx context.Context, name string) error
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
	// CreateDeploy opens a new deploy row (status update_in_progress) — called
	// on redeploy of a store-managed App so the deploys API reflects the push.
	// generation is the App CR's metadata.generation this deploy runs under,
	// captured after the redeploy's own patch (w2/m10: Cancel's build-Job
	// identity is derived from this stored value, not a fresh re-fetch, so it
	// must be the generation this deploy actually runs under). commit is the
	// resolved commit this deploy runs (w9/001), zero when unresolvable. The
	// reconciler's write-back closes the row once the CR reaches Running/Failed.
	CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo) (store.Deploy, error)
	// DeleteApp removes the apps row — the single writer of intent for a
	// store-managed App's existence. The projector deletes the orphaned App CR
	// on its next pass, so the row delete (not a bare CR delete) is what keeps
	// the deletion from being resurrected on resync. ErrNotFound for unknown ids.
	DeleteApp(ctx context.Context, id string) error
	SetAppSuspended(ctx context.Context, id string, suspended bool) error
	SetAppTier(ctx context.Context, id string, tier string) error
	SetAppReplicas(ctx context.Context, id string, replicas int32) error
	// SetAppIdleTTL updates the row's idle-TTL — the single write path for the
	// idle-timeout verb on store-managed Apps, same row-first rationale as
	// SetAppReplicas (the projector owns spec.idleTTLSeconds).
	SetAppIdleTTL(ctx context.Context, id string, seconds int32) error
	// AddDomain appends a custom domain row. Idempotent — conflict silently
	// ignored. The projector carries it into spec.hosts[] on the next resync.
	AddDomain(ctx context.Context, appID, host string) error
	// RemoveDomain removes a custom domain row. Idempotent — not-found silently
	// ignored.
	RemoveDomain(ctx context.Context, appID, host string) error
	// GetAppProtectedStatus resolves an App's protectedStatus via its
	// Environment (w6/m19) — "unprotected" when it has none. The read side of
	// the destructive-verb guard, apps/protection.go.
	GetAppProtectedStatus(ctx context.Context, appID string) (string, error)
}

// CronRunView is one execution of a cron_job — the neutral projection of the
// App's status run history the adapters render (Render exposes cron runs at
// /cron-jobs/{id}/runs). Newest first.
type CronRunView struct {
	Name       string `json:"name"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Status     string `json:"status"` // Running | Succeeded | Failed
}

// AppView is the neutral, bex-native projection of an App — spec intent +
// observed status. Service returns this; each adapter maps it to its own wire
// format (the REST/GraphQL adapters render it in Render's Service shape).
type AppView struct {
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
	Image string   `json:"image"`
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
	// Plan is Render's public spelling of the App's tier (e.g. "pro_plus" for
	// spec.tier "pro-plus"), sourced from lego/types/tiers. Omitted — not
	// faked as "" — when spec.tier is empty or not a recognized tier, so a
	// Render-shaped client sees a real superset rather than a bogus plan.
	Plan      string `json:"plan,omitempty"`
	Revision  string `json:"revision"`
	CreatedAt string `json:"createdAt"`
	// IdleTTLSeconds is how long a free-tier App may be idle before it
	// auto-hibernates ("sleep = free", spec.idleTTLSeconds). 0 = the controller
	// default. A bex extension with no Render counterpart (Render's spin-down
	// window is fixed) — the dashboard's Settings tab reads/writes it.
	IdleTTLSeconds int32 `json:"idleTTLSeconds"`
	// OwnerID is the workspace (tenant) this App belongs to — Render's `ownerId`
	// scoping field (w6/m2/t004), read from the App CR's core.LabelTenant label
	// (the same label List uses to scope to the caller's own tenant). Omitted
	// for Apps the control-plane projector didn't stamp (the hand-applied path,
	// scripts/app-apply.sh) — an honest superset rather than a faked id.
	OwnerID string `json:"ownerId,omitempty"`
	// RootDir is the subdirectory of the repo this App builds from (Render's
	// Root Directory setting, for monorepos; spec.rootDir). Empty is the repo root.
	RootDir        string `json:"rootDir,omitempty"`
	DockerfilePath string `json:"dockerfilePath,omitempty"`
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
	// AutoDeploy is whether a signed git push to Branch redeploys this App
	// (spec.autoDeploy, Render's Auto-Deploy toggle). The Settings → Build &
	// Deploy section reads it to render the toggle and writes it via SetAutoDeploy.
	AutoDeploy bool `json:"autoDeploy"`
	// NotifyOnFail is Render's per-service deploy-failure notification override
	// (spec.notifyOnFail): default | notify | ignore (docs/render-artifacts/
	// notify-on-fail.md). Empty is reported as "default". The Settings →
	// Notifications section reads it and writes it via SetNotifyOnFail.
	NotifyOnFail string `json:"notifyOnFail"`
	// HealthCheckPath is the HTTP path the ReadinessProbe pings (spec.healthCheckPath,
	// Render's healthCheckPath). Empty means the default "/". The Settings →
	// Health & Alerts section reads/writes it via SetHealthCheckPath (w5/009).
	HealthCheckPath string `json:"healthCheckPath,omitempty"`
	// PreDeployCommand is Render's Pre-Deploy Command (spec.preDeployCommand): a
	// command run to completion against the new revision's image before it serves
	// traffic (typically a DB migration); a non-zero exit fails the deploy. Empty
	// means no pre-deploy step. The Settings → Build & Deploy section reads/writes
	// it via SetPreDeployCommand.
	PreDeployCommand string `json:"preDeployCommand,omitempty"`
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

func view(a *appv1alpha1.App) AppView {
	created := ""
	if !a.CreationTimestamp.IsZero() {
		created = a.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	plan := ""
	if t, ok := tiers.Compute.ByID(a.Spec.Tier); ok {
		plan = t.RenderPlan
	}
	svcType := a.Spec.Type
	if svcType == "" {
		svcType = appv1alpha1.TypeWebService // empty spec.type == web_service
	}
	var asView *AutoscalingView
	if a.Spec.Autoscaling != nil {
		v := autoscalingView(a)
		asView = &v
	}
	// normalizeNotifyOnFail's error is unreachable here: the CRD enum already
	// guarantees a.Spec.NotifyOnFail is "", "default", "notify", or "ignore".
	notifyOnFail, _ := normalizeNotifyOnFail(a.Spec.NotifyOnFail)
	return AppView{
		Name:             publicName(a),
		Slug:             a.Spec.PlatformSubdomain(a.Name),
		DisplayName:      a.Spec.DisplayName,
		Type:             svcType,
		Phase:            string(a.Status.Phase),
		URL:              a.Status.URL,
		URLs:             a.Status.URLs,
		Image:            a.Status.Image,
		Runtime:          a.Spec.Runtime,
		BuildCommand:     a.Spec.BuildCommand,
		StartCommand:     a.Spec.StartCommand,
		Builder:          a.Spec.Builder,
		Replicas:         a.Spec.Replicas,
		Suspended:        a.Spec.Suspended,
		Schedule:         a.Spec.Schedule,
		Command:          a.Spec.Command,
		Runs:             cronRunViews(a.Status.Runs),
		Plan:             plan,
		Revision:         a.Status.ActiveRevision,
		CreatedAt:        created,
		IdleTTLSeconds:   a.Spec.IdleTTLSeconds,
		OwnerID:          a.Labels[core.LabelTenant],
		RootDir:          a.Spec.RootDir,
		DockerfilePath:   a.Spec.DockerfilePath,
		Repo:             a.Spec.Repo,
		Branch:           a.Spec.Branch,
		BuildFilter:      buildFilterView(a.Spec.BuildFilter),
		Autoscaling:      asView,
		AutoDeploy:       a.Spec.AutoDeploy,
		NotifyOnFail:     notifyOnFail,
		HealthCheckPath:  a.Spec.HealthCheckPath,
		PreDeployCommand: a.Spec.PreDeployCommand,
		PublishPath:      a.Spec.PublishPath,
		Routes:           staticRouteViews(a.Spec.Routes),
		Headers:          staticHeaderViews(a.Spec.Headers),
	}
}

// cronRunViews projects the CR's status run history onto the neutral view shape.
func cronRunViews(runs []appv1alpha1.CronRun) []CronRunView {
	if len(runs) == 0 {
		return nil
	}
	out := make([]CronRunView, len(runs))
	for i, r := range runs {
		out[i] = CronRunView{Name: r.Name, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Status: r.Status}
	}
	return out
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
	opts := []client.ListOption{client.InNamespace(s.Namespace)}
	switch {
	case ownerID != "":
		opts = append(opts, client.MatchingLabels{core.LabelTenant: ownerID})
	case s.Workspace != nil:
		tenantID, ok := s.Tenant(ctx)
		if !ok {
			return []AppView{}, nil
		}
		opts = append(opts, client.MatchingLabels{core.LabelTenant: tenantID})
	}
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, opts...); err != nil {
		return nil, err
	}
	out := make([]AppView, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, view(&list.Items[i]))
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
	ID     string
	Name   string
	CPU    string
	Memory string
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
		out[i] = InstanceType{ID: t.RenderPlan, Name: tierDisplayName(id), CPU: t.CPU, Memory: t.Memory}
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
	return view(a), nil
}

// CreateRequest is the neutral create-or-update input the three surfaces (and
// the bex.yml deploy mapper) share — Render's create body projected onto the
// App CR spec. One of Repo/Image is required. Zero values fall back to the
// platform defaults the operator would apply (branch main, port 3000, one
// replica, the catalog's default tier). Plan accepts either Render's spelling
// ("pro_plus") or a bex tier id ("pro-plus"). Hosts are external FQDNs, the
// first canonical (spec.host); only a web_service is additionally exposed at the
// platform hostname <name>.<BEX_BASE_DOMAIN>.
type CreateRequest struct {
	// OwnerID is the workspace to create the service IN — Render's `ownerId`
	// (w6/m14). Empty means the caller's default workspace (their oldest
	// membership), so a single-workspace client never has to say it; a workspace
	// the caller is not a member of is core.ErrForbidden, never a create that
	// silently lands somewhere else. All three surfaces fill it (REST body,
	// GraphQL arg, the MCP session's selected workspace), so the workspace a
	// create targets is decided in exactly one place: Create.
	OwnerID string
	Name    string
	// Type is the Render serviceType: web_service (default), private_service,
	// background_worker, cron_job. Empty defaults to web_service.
	Type string
	// Schedule is the cron expression, required when Type is cron_job.
	Schedule string
	// Command overrides a cron_job's default entrypoint (spec.command); empty
	// runs the image's own command. Ignored for every other type.
	Command      string
	Repo         string
	Image        string
	Branch       string
	Builder      string
	Runtime      string
	BuildCommand string
	StartCommand string
	// RootDir scopes build-from-git to a subdirectory of Repo (Render's Root
	// Directory setting, for monorepos; spec.rootDir). Empty is the repo root.
	RootDir string
	// BuildFilter is Render's Build Filters at create time (spec.buildFilter):
	// glob patterns gating git-push auto-deploys. nil means unset (every matching
	// push deploys). Validated + canonicalized by normalizeBuildFilter; editable
	// later via SetBuildFilter.
	BuildFilter     *BuildFilterView
	DockerfilePath  string
	Port            int32
	Replicas        int32
	Plan            string
	HealthCheckPath string
	Env             []appv1alpha1.EnvVar
	Hosts           []string
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
	// PreDeployCommand is Render's Pre-Deploy Command (spec.preDeployCommand): a
	// command run to completion against the new revision's image before it serves
	// traffic (typically a DB migration); a non-zero exit fails the deploy. Empty
	// means no pre-deploy step. Ignored for cron_job/static_site.
	PreDeployCommand string
	// PublishPath is the built output directory a static_site serves (Render's
	// "Publish Directory", spec.publishPath). Required when Type is static_site,
	// ignored otherwise.
	PublishPath string
	// Routes / Headers are a static_site's edge rules at create time (spec.routes /
	// spec.headers). Ignored for every other type; editable later via
	// SetRoutes/SetHeaders.
	Routes  []StaticRouteView
	Headers []StaticHeaderView
	// DryRun, when true, resolves the spec and returns a preview without any
	// Kubernetes or control-plane-store writes — zero side effects (w2/m29).
	// The response shape is identical to a live create; the caller knows it is a
	// dry-run because they set this flag. Validation (specFromCreate) still runs,
	// so an invalid request still returns an error.
	DryRun bool
}

// Create writes the App CR for a new service, or updates it in place when one
// of the same name already exists — the same verb "deploy this" rides (Deploy
// maps a repo + bex.yml onto a CreateRequest, docs/ADR006-bex-api.md). Repeating the
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
	// The workspace this create acts in: the one named by req.OwnerID (already
	// membership-checked by Create's Authorize) or the caller's default. Empty
	// when the store is off or the caller is unbound. Resolved BEFORE the
	// existence probe, because it is what makes that probe workspace-correct.
	tenantID, _ := s.Tenant(ctx)

	// Dry-run: return the resolved spec preview without any k8s or store writes.
	if req.DryRun {
		a := &appv1alpha1.App{}
		a.Name = req.Name
		a.Namespace = s.Namespace
		a.Spec = desired
		if tenantID != "" {
			a.Labels = map[string]string{core.LabelTenant: tenantID}
		}
		return view(a), nil
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
		return AppView{}, fmt.Errorf("%w: name %q is already in use", core.ErrConflict, req.Name)
	}

	a := &appv1alpha1.App{}
	a.Name = req.Name
	if tenantID != "" {
		// Collision-free object name (w4/m19): all tenants' Apps share one
		// namespace, so two tenants both naming a service "web" must not
		// collide on the bare name the way the pre-migration scheme did.
		a.Name = core.CRName(tenantID, req.Name)
	}
	a.Namespace = s.Namespace
	a.Spec = desired
	// A store-managed create writes its source-of-truth row before the CR so the
	// CR can be stamped with the row id and its initial deploy history exists as
	// soon as create succeeds. If any later step fails, remove that row again:
	// otherwise the projector can resurrect a service the API reported as
	// failed, potentially from the store's narrower projection of the desired
	// spec (for example after a stale CRD rejects a newly added field).
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
	// Per-workspace service cap (w7/m9): count before creating so the (N+1)th
	// service is refused across all three surfaces. Counted against tenantID —
	// the workspace this create TARGETS (w6/m14's ownerId), not whichever one
	// the caller happens to resolve to, so naming a workspace charges its cap
	// and not another's. Skipped when cap is 0 (unlimited) or the caller has no
	// resolved tenant (store off / unbound).
	if s.MaxServices > 0 && tenantID != "" {
		var existing appv1alpha1.AppList
		if listErr := s.ListByTenant(ctx, &existing, tenantID); listErr != nil {
			return AppView{}, fmt.Errorf("checking service cap: %w", listErr)
		}
		if len(existing.Items) >= s.MaxServices {
			return AppView{}, fmt.Errorf("%w: workspace is limited to %d services; delete an existing service to create another", core.ErrBadRequest, s.MaxServices)
		}
	}
	if tenantID != "" {
		// LabelServiceName is what lets GetApp find this App from one of the
		// caller's OTHER workspaces by its public name (w4/m19) — metadata.Name
		// alone no longer serves that purpose once it's tenant-prefixed.
		a.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelServiceName: req.Name}
	}
	// Write the store row when the store is on + a tenant is resolved, so the
	// create populates deploy history and the projector recognises the CR as
	// store-managed (unified create path, w2/m11). The store's own
	// UNIQUE(tenant_id, name) constraint is the race-safe backstop behind the
	// GetApp pre-check above: a concurrent duplicate create that slips past it
	// still surfaces as ErrConflict here, never an unclassified 500.
	if s.Store != nil && tenantID != "" {
		row, err := s.Store.CreateApp(ctx, store.App{
			TenantID: tenantID,
			Name:     req.Name,
			Repo:     req.Repo,
			Image:    req.Image,
			Branch:   desired.Branch,
			Port:     desired.Port,
			Replicas: desired.Replicas,
			Tier:     desired.Tier,
			// Provenance for the first deploy row CreateApp opens (w9/001):
			// the branch tip this create will build, resolved best-effort.
			FirstDeployCommit: s.resolveDeployCommit(ctx, tenantID, req.Repo, desired.Branch),
		})
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				return AppView{}, fmt.Errorf("%w: name %q is already in use", core.ErrConflict, req.Name)
			}
			return AppView{}, fmt.Errorf("creating service record: %w", err)
		}
		// Stamp the managed-by + app-id labels so the projector's byID index
		// finds this CR on its next pass (avoiding a duplicate create) and
		// lifecycle verbs (suspend/scale/plan) have an app-id to write through.
		if a.Labels == nil {
			a.Labels = map[string]string{}
		}
		a.Labels[store.LabelManagedBy] = store.ManagedByValue
		a.Labels[store.LabelAppID] = row.ID
		createdRowID = row.ID
		a.Labels[store.LabelWorkspace] = tenantID
		// The globally-unique slug (w4/m19) drives the platform host
		// (operator effectiveHosts) — never req.Name, which is only
		// workspace-unique and can collide across tenants.
		a.Spec.Subdomain = row.Slug
	}
	// A private-connection repo gets a fresh clone token + spec.cloneSecret so
	// its first build authenticates. create-owned only: never set on a spec
	// that already hand-pointed cloneSecret elsewhere.
	if desired.CloneSecret == "" {
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
	if err := s.Client.Create(ctx, a); err != nil {
		return AppView{}, rollbackStoreRow(err)
	}
	if s.Kick != nil {
		s.Kick()
	}
	return view(a), nil
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
		err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: objName}, &a)
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
	}
	// Write the store row when the store is on + a tenant is resolved, so the
	// create populates deploy history and the projector recognises the CR as
	// store-managed (unified create path, w2/m11).
	if s.Store != nil && tenantID != "" {
		row, err := s.Store.CreateApp(ctx, store.App{
			TenantID: tenantID,
			Name:     req.Name,
			Repo:     req.Repo,
			Image:    req.Image,
			Branch:   desired.Branch,
			Port:     desired.Port,
			Replicas: desired.Replicas,
			Tier:     desired.Tier,
			// Provenance for the first deploy row CreateApp opens (w9/001) —
			// same best-effort resolution as create()'s.
			FirstDeployCommit: s.resolveDeployCommit(ctx, tenantID, req.Repo, desired.Branch),
		})
		if err != nil {
			// Same TOCTOU backstop as create() (w4/m19): applyCreate's own
			// GetApp pre-check can miss a duplicate that lands concurrently
			// between the check and this write; the store's UNIQUE(tenant_id,
			// name) constraint still catches it and must classify as
			// ErrConflict here too, or a blueprint-stack race would 500
			// instead of 409.
			if errors.Is(err, store.ErrConflict) {
				return AppView{}, fmt.Errorf("%w: name %q is already in use", core.ErrConflict, req.Name)
			}
			return AppView{}, fmt.Errorf("creating service record: %w", err)
		}
		// Stamp the managed-by + app-id labels so the projector's byID index
		// finds this CR on its next pass (avoiding a duplicate create) and
		// lifecycle verbs (suspend/scale/plan) have an app-id to write through.
		if a.Labels == nil {
			a.Labels = map[string]string{}
		}
		a.Labels[store.LabelManagedBy] = store.ManagedByValue
		a.Labels[store.LabelAppID] = row.ID
		a.Labels[store.LabelWorkspace] = tenantID
		// The globally-unique slug (w4/m19) drives the platform host (operator
		// effectiveHosts) — never req.Name, which is only workspace-unique and
		// can collide across tenants.
		a.Spec.Subdomain = row.Slug
	}
	// A private-connection repo gets a fresh clone token + spec.cloneSecret so
	// its first build authenticates. create-owned only: never set on a spec
	// that already hand-pointed cloneSecret elsewhere.
	if desired.CloneSecret == "" {
		secretName, err := s.ensureCloneSecret(ctx, a)
		if err != nil {
			return AppView{}, err
		}
		a.Spec.CloneSecret = secretName
	}
	pullSecretName, err := s.ensureExternalRegistryPullSecret(ctx, a)
	if err != nil {
		return AppView{}, err
	}
	a.Spec.ExternalRegistryPullSecret = pullSecretName
	if err := s.Client.Create(ctx, a); err != nil {
		return AppView{}, err
	}
	if s.Kick != nil {
		s.Kick()
	}
	return view(a), nil
}

// Delete removes a service — the single implementation the three adapters
// delegate to. With the store on and the App store-managed (it carries the
// store's app-id label), it deletes the apps row first: the row is the single
// writer of that App's existence, so a resync can't resurrect the CR (a bare CR
// delete would be). It then deletes the CR directly too, so the removal
// converges immediately instead of waiting a resync period — the projector is
// idempotent, a CR with no row is deleted again as a harmless no-op. Store-less
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
	if s.Store != nil {
		if id := a.Labels[store.LabelAppID]; id != "" {
			// An already-gone row is the intended end state, not an error (a
			// resync may have raced us) — treat it like RemoveDomain does and
			// fall through to delete the orphaned CR.
			if err := s.Store.DeleteApp(ctx, id); err != nil && !errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("delete source of truth: %w", err)
			}
		}
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
		if err := s.SecretsEraser.PurgeApp(ctx, a.Name); err != nil {
			return fmt.Errorf("purge app secrets: %w", err)
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
	if req.Repo == "" && req.Image == "" {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: one of repo or image is required", core.ErrBadRequest)
	}
	// Build-from-git inputs are validated at the API boundary (w6/m6 t003) so the
	// operator never forwards an unchecked repo/branch/rootDirectory into the
	// BuildKit context string. A bare image deploy skips these (no build).
	if req.Repo != "" && !store.ValidRepo(req.Repo) {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: repo must be an https/ssh/git URL", core.ErrBadRequest)
	}
	if req.Branch != "" && !store.ValidGitRef(req.Branch) {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: branch must be a git ref (no shell metacharacters)", core.ErrBadRequest)
	}
	if req.RootDir != "" && !store.ValidRootDir(req.RootDir) {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: rootDirectory must be a relative path with no '..' components", core.ErrBadRequest)
	}
	if req.DockerfilePath != "" && !store.ValidRootDir(req.DockerfilePath) {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: dockerfilePath must be a relative path with no '..' components", core.ErrBadRequest)
	}
	svcType, err := normalizeType(req.Type)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	// A cron_job needs a schedule; a worker/cron has no ingress, so it can't carry
	// custom domains (same rule the deploy manifest enforces for private services).
	if svcType == appv1alpha1.TypeCronJob && strings.TrimSpace(req.Schedule) == "" {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: schedule is required for a cron_job", core.ErrBadRequest)
	}
	if (svcType == appv1alpha1.TypeBackgroundWorker || svcType == appv1alpha1.TypeCronJob) && len(req.Hosts) > 0 {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: a %s has no ingress and cannot list domains", core.ErrBadRequest, svcType)
	}
	// A static_site needs a publish directory, and its edge rules must be valid.
	if svcType == appv1alpha1.TypeStaticSite {
		if strings.TrimSpace(req.PublishPath) == "" {
			return appv1alpha1.AppSpec{}, fmt.Errorf("%w: publishPath is required for a static_site", core.ErrBadRequest)
		}
		if err := validateRoutes(req.Routes); err != nil {
			return appv1alpha1.AppSpec{}, err
		}
		if err := validateHeaders(req.Headers); err != nil {
			return appv1alpha1.AppSpec{}, err
		}
	} else if strings.TrimSpace(req.PublishPath) != "" || len(req.Routes) > 0 || len(req.Headers) > 0 {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: publishPath/routes/headers only apply to a static_site", core.ErrBadRequest)
	}
	tier, err := normalizeTierOrPlan(req.Plan)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	if req.Port < 0 || req.Port > 65535 {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: port must be 1-65535", core.ErrBadRequest)
	}
	if req.Replicas < 0 || req.Replicas > store.MaxReplicas {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: replicas must be 0-%d", core.ErrBadRequest, store.MaxReplicas)
	}
	port := req.Port
	if port == 0 {
		port = 3000
	}
	replicas := req.Replicas
	if replicas == 0 {
		replicas = 1
	}
	branch := req.Branch
	if branch == "" && req.Repo != "" {
		branch = "main"
	}
	builder := req.Builder
	runtime := strings.ToLower(strings.TrimSpace(req.Runtime))
	if runtime != "" && builder != "" && builder != "auto" {
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: runtime and builder cannot both select a build strategy", core.ErrBadRequest)
	}
	switch runtime {
	case "":
		if builder != "" && builder != "auto" && builder != "buildpack" && builder != "dockerfile" {
			return appv1alpha1.AppSpec{}, fmt.Errorf("%w: builder must be auto, buildpack, or dockerfile", core.ErrBadRequest)
		}
	case "docker":
		builder = "dockerfile"
	case "image":
		if req.Image == "" || req.Repo != "" {
			return appv1alpha1.AppSpec{}, fmt.Errorf("%w: runtime image requires image and no repo", core.ErrBadRequest)
		}
		builder = "auto"
	case "elixir", "go", "node", "python", "ruby", "rust":
		if req.Repo == "" {
			return appv1alpha1.AppSpec{}, fmt.Errorf("%w: native runtime %s requires repo", core.ErrBadRequest, runtime)
		}
		if strings.TrimSpace(req.BuildCommand) == "" || strings.TrimSpace(req.StartCommand) == "" {
			return appv1alpha1.AppSpec{}, fmt.Errorf("%w: native runtime %s requires buildCommand and startCommand", core.ErrBadRequest, runtime)
		}
		builder = "native"
	default:
		return appv1alpha1.AppSpec{}, fmt.Errorf("%w: unsupported runtime %q", core.ErrBadRequest, runtime)
	}
	// AutoDeploy: default on for a repo-backed service (a push should redeploy,
	// Render's default), off for an image-backed one (no repo to rebuild from).
	// An explicit request value wins.
	autoDeploy := req.Repo != ""
	if req.AutoDeploy != nil {
		autoDeploy = *req.AutoDeploy
	}
	buildFilter, err := normalizeBuildFilter(req.BuildFilter)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	notifyOnFail, err := normalizeNotifyOnFail(req.NotifyOnFail)
	if err != nil {
		return appv1alpha1.AppSpec{}, err
	}
	spec := appv1alpha1.AppSpec{
		Type:             svcType,
		Repo:             req.Repo,
		Image:            req.Image,
		Branch:           branch,
		Runtime:          runtime,
		BuildCommand:     req.BuildCommand,
		StartCommand:     req.StartCommand,
		Builder:          builder,
		RootDir:          req.RootDir,
		BuildFilter:      buildFilter,
		DockerfilePath:   req.DockerfilePath,
		Port:             port,
		Replicas:         replicas,
		Tier:             tier,
		HealthCheckPath:  req.HealthCheckPath,
		Env:              req.Env,
		AutoDeploy:       autoDeploy,
		NotifyOnFail:     notifyOnFail,
		PreDeployCommand: strings.TrimSpace(req.PreDeployCommand),
		// A web service and a static site are public: expose them at
		// <name>.<BEX_BASE_DOMAIN> so the caller gets a live URL with no custom
		// domain. Every other type opts out (private has no platform host;
		// worker/cron have no HTTP endpoint at all).
		Expose: svcType == appv1alpha1.TypeWebService || svcType == appv1alpha1.TypeStaticSite,
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
		spec.Host = req.Hosts[0]
		spec.Hosts = append([]string(nil), req.Hosts[1:]...)
	}
	return spec, nil
}

// notifyOnFailDefault is Render's own default enum value for spec.notifyOnFail
// — "defer to the workspace/member notification preference" (docs/render-
// artifacts/notify-on-fail.md). Empty is normalized to this both at create
// time and by view(), so an App created before this field existed reports the
// same "default" behavior it always had.
const notifyOnFailDefault = "default"

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

// applyCreateToSpec copies the create-owned fields of want onto an existing
// spec, leaving fields owned by the operator or other features (EnvFromSecret,
// Suspended, IdleTTLSeconds, RestartedAt) untouched — the same discipline the
// store projector's applyOwnedSpec follows.
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
	dst.Port = want.Port
	dst.Replicas = want.Replicas
	dst.Tier = want.Tier
	dst.HealthCheckPath = want.HealthCheckPath
	dst.Env = want.Env
	dst.AutoDeploy = want.AutoDeploy
	dst.NotifyOnFail = want.NotifyOnFail
	dst.PreDeployCommand = want.PreDeployCommand
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
	if t, ok := tiers.Compute.ByRenderPlan(v); ok {
		return t.ID, nil
	}
	if _, ok := tiers.Compute.ByID(v); ok {
		return v, nil
	}
	return "", fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Compute.RenderPlans(), "|"))
}

// redeploy bumps spec.restartedAt to force the operator to roll a new revision
// — for a repo-backed App this re-runs the build-from-git (the generation bump
// invalidates the cached Status.Image). Unauthorized on purpose: its only
// caller is the HMAC-verified git webhook, whose signature check is the
// authorization (there is no OpenFGA identity on a git-host callback).
func (s *Service) redeploy(ctx context.Context, name string) (AppView, error) {
	a, err := s.GetApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
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
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		if secretName != "" {
			a.Spec.CloneSecret = secretName
		}
		a.Spec.ExternalRegistryPullSecret = pullSecretName
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
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
// POST /cron-jobs/{id}/runs): it bumps spec.runAt (verb-as-timestamp, like
// Restart), and the operator materializes a single Job the run history then
// shows. Rejected for a non-cron service. Intent only — the run appears in
// status.runs once the operator reconciles, not synchronously.
func (s *Service) TriggerCronRun(ctx context.Context, name string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		return AppView{}, fmt.Errorf("%w: service %q is not a cron_job", core.ErrBadRequest, name)
	}
	return s.patch(ctx, core.RelCanOperate, name, func(a *appv1alpha1.App) {
		a.Spec.RunAt = s.Now().UTC().Format(time.RFC3339Nano)
	})
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
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	t, ok := tiers.Compute.ByRenderPlan(plan)
	if !ok {
		return AppView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Compute.RenderPlans(), "|"))
	}
	tier := t.ID
	return s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppTier(ctx, id, tier) },
		func(a *appv1alpha1.App) { a.Spec.Tier = tier })
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
	preview := a.DeepCopy()
	preview.Spec.Tier = t.ID
	return view(preview), nil
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
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if replicas < 1 || replicas > store.MaxReplicas {
		return AppView{}, fmt.Errorf("%w: numInstances must be 1-%d", core.ErrBadRequest, store.MaxReplicas)
	}
	return s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppReplicas(ctx, id, replicas) },
		func(a *appv1alpha1.App) { a.Spec.Replicas = replicas })
}

// MaxIdleTTLSeconds bounds the idle-timeout a caller may set (7 days). Free-tier
// Apps auto-hibernate after this many idle seconds; 0 means the controller
// default. A generous ceiling — the point is "sleep quickly to save money", not
// an indefinite keep-alive that would defeat the free tier.
const MaxIdleTTLSeconds int32 = 7 * 24 * 60 * 60

// SetIdleTTL sets how long the App may idle before it auto-hibernates
// (spec.idleTTLSeconds; "sleep = free", w1/m4). 0 restores the controller
// default. A bex extension with no Render counterpart — Render's free spin-down
// window is fixed — so it writes spec.idleTTLSeconds the same row-first way as
// Scale (the projector owns the field). Only free-tier Apps ever sleep, but the
// value is stored regardless so it takes effect if the plan later changes to
// free; the dashboard is what gates the control per tier.
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
func (s *Service) SetRootDir(ctx context.Context, name, rootDir string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if a.Spec.Repo == "" {
		return AppView{}, fmt.Errorf("%w: service %q has no repo to build (root directory only applies to build-from-git)", core.ErrBadRequest, name)
	}
	if !store.ValidRootDir(rootDir) {
		return AppView{}, fmt.Errorf("%w: rootDirectory must be a relative path with no '..' components", core.ErrBadRequest)
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.RootDir = rootDir
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
	if a.Spec.Repo == "" {
		return AppView{}, fmt.Errorf("%w: service %q has no repo to build (build filters only apply to build-from-git)", core.ErrBadRequest, name)
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
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
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

// SetCronJob changes a cron_job's schedule and/or command (spec.schedule +
// spec.command). Rejected for a non-cron service. Both args are optional — nil
// means "keep existing," which lets REST PATCH update one field without
// re-reading the other. A non-nil schedule must be a non-empty 5-field crontab;
// a non-nil command of "" clears the entrypoint override. Direct CR patch, not
// projection-owned (mirrors Builder/RootDir).
func (s *Service) SetCronJob(ctx context.Context, name string, schedule, command *string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
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

// validCronSchedule reports whether s is a 5-field cron expression. Field
// syntax is intentionally permissive — the k8s CronJob controller validates the
// individual fields at convergence; bex only checks the field count here.
func validCronSchedule(s string) bool {
	return len(strings.Fields(s)) == 5
}

// SetHealthCheckPath changes spec.healthCheckPath — the HTTP path the operator
// wires into the container's ReadinessProbe (w1/m23/t001). A direct CR patch,
// not projection-owned (mirrors Builder/RootDir). An empty path resets to the
// default "/". Rejected for service types that have no HTTP port (cron_job,
// background_worker) since those never serve a health endpoint.
func (s *Service) SetHealthCheckPath(ctx context.Context, name string, path string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if a.Spec.Type == appv1alpha1.TypeCronJob || a.Spec.Type == appv1alpha1.TypeBackgroundWorker {
		return AppView{}, fmt.Errorf("%w: health check path is not applicable to a %s", core.ErrBadRequest, a.Spec.Type)
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		trimmed = "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		return AppView{}, fmt.Errorf("%w: health check path must start with /", core.ErrBadRequest)
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.HealthCheckPath = trimmed
	})
}

// SetAutoDeploy flips whether a signed git push to the tracked branch redeploys
// this App (spec.autoDeploy, Render's Auto-Deploy toggle). A direct CR patch,
// not projection-owned (mirrors Builder/RootDir), and no restartedAt bump —
// flipping the toggle changes future push behavior, it does not itself redeploy.
func (s *Service) SetAutoDeploy(ctx context.Context, name string, enabled bool) (AppView, error) {
	return s.patch(ctx, core.RelCanOperate, name, func(a *appv1alpha1.App) {
		a.Spec.AutoDeploy = enabled
	})
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
	})
}

// SetDisplayName changes the human-facing label for an App without changing
// its immutable Kubernetes object name or any identity derived from that name
// (including platform hostnames and TLS secret names). It is a direct CR patch:
// displayName is not owned by the control-plane row projector. Whitespace at
// the edges is presentation noise and is trimmed; an empty value clears the
// label so clients fall back to the immutable Name.
func (s *Service) SetDisplayName(ctx context.Context, name, displayName string) (AppView, error) {
	return s.patch(ctx, core.RelCanOperate, name, func(a *appv1alpha1.App) {
		a.Spec.DisplayName = strings.TrimSpace(displayName)
	})
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
// the CR patch. Every caller (setSuspended, SetPlan, Scale, SetIdleTTL, …)
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
		if id := a.Labels[store.LabelAppID]; id != "" {
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
func (s *Service) patchFetched(ctx context.Context, a *appv1alpha1.App, mutate func(*appv1alpha1.App)) (AppView, error) {
	base := client.MergeFrom(a.DeepCopy())
	mutate(a)
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return AppView{}, err
	}
	return view(a), nil
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

// validateRoutes rejects a malformed redirect/rewrite list: each rule needs a
// known type and a rooted source + destination path (Render's contract).
func validateRoutes(routes []StaticRouteView) error {
	for i, r := range routes {
		t := strings.TrimSpace(r.Type)
		if t != "redirect" && t != "rewrite" {
			return fmt.Errorf("%w: routes[%d].type must be redirect or rewrite", core.ErrBadRequest, i)
		}
		if !strings.HasPrefix(strings.TrimSpace(r.Source), "/") {
			return fmt.Errorf("%w: routes[%d].source must be a path starting with /", core.ErrBadRequest, i)
		}
		if !strings.HasPrefix(strings.TrimSpace(r.Destination), "/") {
			return fmt.Errorf("%w: routes[%d].destination must be a path starting with /", core.ErrBadRequest, i)
		}
	}
	return nil
}

// validateHeaders rejects a malformed custom-header list: each rule needs a
// rooted path pattern and a header name.
func validateHeaders(headers []StaticHeaderView) error {
	for i, h := range headers {
		if !strings.HasPrefix(strings.TrimSpace(h.Path), "/") {
			return fmt.Errorf("%w: headers[%d].path must be a path starting with /", core.ErrBadRequest, i)
		}
		if strings.TrimSpace(h.Name) == "" {
			return fmt.Errorf("%w: headers[%d].name is required", core.ErrBadRequest, i)
		}
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
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
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
	if err := requireStaticSite(a, name); err != nil {
		return nil, err
	}
	return staticHeaderViews(a.Spec.Headers), nil
}

// SetHeaders replaces a static_site's custom response-header rules (Render's bulk
// PUT /v1/services/{id}/headers). Same live-read semantics as SetRoutes.
func (s *Service) SetHeaders(ctx context.Context, name string, headers []StaticHeaderView) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
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
// (spec.publishPath) and bumps spec.restartedAt so the change republishes (a new
// generation invalidates the cached revision, re-running the publish plane).
// Rejected for a non-static_site or an empty path.
func (s *Service) SetPublishPath(ctx context.Context, name, publishPath string) (AppView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if strings.TrimSpace(publishPath) == "" {
		return AppView{}, fmt.Errorf("%w: publishPath must not be empty", core.ErrBadRequest)
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
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return AutoscalingView{}, err
	}
	if req.MinInstances < 0 {
		return AutoscalingView{}, fmt.Errorf("%w: minInstances must be ≥ 0", core.ErrBadRequest)
	}
	if req.MaxInstances < 1 {
		return AutoscalingView{}, fmt.Errorf("%w: maxInstances must be ≥ 1", core.ErrBadRequest)
	}
	if req.MinInstances > req.MaxInstances {
		return AutoscalingView{}, fmt.Errorf("%w: minInstances must be ≤ maxInstances", core.ErrBadRequest)
	}
	if req.TargetCPUPercent != nil && (*req.TargetCPUPercent < 1 || *req.TargetCPUPercent > 100) {
		return AutoscalingView{}, fmt.Errorf("%w: targetCPUPercent must be 1–100", core.ErrBadRequest)
	}
	if req.TargetMemoryPercent != nil && (*req.TargetMemoryPercent < 1 || *req.TargetMemoryPercent > 100) {
		return AutoscalingView{}, fmt.Errorf("%w: targetMemoryPercent must be 1–100", core.ErrBadRequest)
	}
	if req.TargetCPUPercent == nil && req.TargetMemoryPercent == nil {
		return AutoscalingView{}, fmt.Errorf("%w: at least one of targetCPUPercent or targetMemoryPercent is required", core.ErrBadRequest)
	}
	_, err = s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{
			Enabled:             true,
			MinReplicas:         req.MinInstances,
			MaxReplicas:         req.MaxInstances,
			TargetCPUPercent:    req.TargetCPUPercent,
			TargetMemoryPercent: req.TargetMemoryPercent,
		}
	})
	if err != nil {
		return AutoscalingView{}, err
	}
	return autoscalingView(a), nil
}

// DeleteAutoscaling disables autoscaling on a service (Render's DELETE
// .../autoscaling): clears spec.autoscaling so the service reverts to its
// fixed spec.replicas count. Idempotent — already-disabled is a no-op.
func (s *Service) DeleteAutoscaling(ctx context.Context, name string) error {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, name)
	if err != nil {
		return err
	}
	if a.Spec.Autoscaling == nil || !a.Spec.Autoscaling.Enabled {
		return nil // already disabled — idempotent
	}
	_, err = s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Autoscaling = nil
	})
	return err
}
