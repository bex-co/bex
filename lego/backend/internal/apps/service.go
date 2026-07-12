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
}

// IntentStore is the slice of the source of truth Service writes through — kept
// to the methods the lifecycle verbs need, so the service can't grow into a
// second store client and tests fake a single method. *store.PGStore satisfies it.
type IntentStore interface {
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
	// Type is the Render serviceType (web_service | private_service |
	// background_worker | cron_job); empty spec.type projects as web_service.
	Type      string   `json:"type"`
	Phase     string   `json:"phase"`
	URL       string   `json:"url"`
	URLs      []string `json:"urls"`
	Image     string   `json:"image"`
	Replicas  int32    `json:"replicas"`
	Suspended bool     `json:"suspended"`
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
	RootDir string `json:"rootDir,omitempty"`
	// Repo and Branch are the build-from-git source (spec.repo/spec.branch),
	// empty for an image-backed App. The dashboard's Settings → Build & Deploy
	// section reads all three; only RootDir is editable after create
	// (SetRootDir) — Repo/Branch are fixed at create time.
	Repo   string `json:"repo,omitempty"`
	Branch string `json:"branch,omitempty"`
	// Autoscaling is the current per-service autoscaling config (nil when
	// spec.autoscaling is unset, i.e. disabled and unconfigured).
	Autoscaling *AutoscalingView `json:"autoscaling,omitempty"`
	// AutoDeploy is whether a signed git push to Branch redeploys this App
	// (spec.autoDeploy, Render's Auto-Deploy toggle). The Settings → Build &
	// Deploy section reads it to render the toggle and writes it via SetAutoDeploy.
	AutoDeploy bool `json:"autoDeploy"`
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
	return AppView{
		Name:           a.Name,
		Type:           svcType,
		Phase:          string(a.Status.Phase),
		URL:            a.Status.URL,
		URLs:           a.Status.URLs,
		Image:          a.Status.Image,
		Replicas:       a.Spec.Replicas,
		Suspended:      a.Spec.Suspended,
		Schedule:       a.Spec.Schedule,
		Command:        a.Spec.Command,
		Runs:           cronRunViews(a.Status.Runs),
		Plan:           plan,
		Revision:       a.Status.ActiveRevision,
		CreatedAt:      created,
		IdleTTLSeconds: a.Spec.IdleTTLSeconds,
		OwnerID:        a.Labels[core.LabelTenant],
		RootDir:        a.Spec.RootDir,
		Repo:           a.Spec.Repo,
		Branch:         a.Spec.Branch,
		Autoscaling:    asView,
		AutoDeploy:     a.Spec.AutoDeploy,
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
// additionally authorizes can_view on that exact workspace explicitly — so a
// caller who belongs to more than one workspace can pick one (an ownerId the
// caller can't access is ErrForbidden), the same override Render's real API
// supports for a multi-workspace key.
func (s *Service) List(ctx context.Context, ownerID string) ([]AppView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	opts := []client.ListOption{client.InNamespace(s.Namespace)}
	switch {
	case ownerID != "":
		if err := s.AuthorizeOn(ctx, core.RelCanView, core.WorkspaceObject(ownerID)); err != nil {
			return nil, err
		}
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
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return AppView{}, err
	}
	a, err := s.GetApp(ctx, name)
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
	Name string
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
	Branch  string
	Builder string
	// RootDir scopes build-from-git to a subdirectory of Repo (Render's Root
	// Directory setting, for monorepos; spec.rootDir). Empty is the repo root.
	RootDir         string
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
}

// Create writes the App CR for a new service, or updates it in place when one
// of the same name already exists — the same verb "deploy this" rides (Deploy
// maps a repo + bex.yml onto a CreateRequest, docs/ADR006-bex-api.md). Repeating the
// call for an existing service is a redeploy, not a duplicate: the spec fields
// the request carries are re-applied and spec.restartedAt is bumped, so a
// repo-backed App re-runs its build-from-git. Intent only — the operator
// converges the CR into a running service with a live URL.
//
// This writes the CR directly (the hand-applied path scripts/app-apply.sh
// uses), not through a store row: the public surface has no tenant context, and
// the row-backed, multi-tenant create is the internal control-plane API's job
// (store/api.go POST /v1/apps). The projector never touches a CR it didn't
// create (it lists only its own managed-by label), so the two coexist.
func (s *Service) Create(ctx context.Context, req CreateRequest) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return AppView{}, err
	}
	return s.create(ctx, req)
}

// create is the unauthorized core of Create — shared with Deploy, which
// authorizes once before mapping the bex.yml.
func (s *Service) create(ctx context.Context, req CreateRequest) (AppView, error) {
	desired, err := specFromCreate(req)
	if err != nil {
		return AppView{}, err
	}
	existing, err := s.GetApp(ctx, req.Name)
	switch {
	case errors.Is(err, core.ErrNotFound):
		a := &appv1alpha1.App{}
		a.Name = req.Name
		a.Namespace = s.Namespace
		a.Spec = desired
		// With the store on, stamp the caller's tenant so it can see/manage what
		// it just created — GetApp's tenant gate and List's tenant filter both key
		// off this label, and a label-less CR would be invisible to its own owner.
		if s.Workspace != nil {
			if tenantID, ok := s.Tenant(ctx); ok {
				a.Labels = map[string]string{core.LabelTenant: tenantID}
			}
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
		if err := s.Client.Create(ctx, a); err != nil {
			return AppView{}, err
		}
		return view(a), nil
	case err != nil:
		return AppView{}, err
	default:
		// Update-in-place = redeploy: re-apply the request's owned fields onto the
		// live spec (leaving operator/other-feature fields like EnvFromSecret
		// intact), then bump restartedAt so a repo-backed App rebuilds even when
		// no other field changed.
		base := client.MergeFrom(existing.DeepCopy())
		applyCreateToSpec(&existing.Spec, desired)
		// Refresh the clone token so the rebuild starts with a token minted
		// seconds ago. ensureCloneSecret reads the just-applied repo; it returns
		// "" for an unconnected repo, so a hand-set cloneSecret pointing elsewhere
		// is left untouched.
		secretName, err := s.ensureCloneSecret(ctx, existing)
		if err != nil {
			return AppView{}, err
		}
		if secretName != "" {
			existing.Spec.CloneSecret = secretName
		}
		existing.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
		if err := s.Client.Patch(ctx, existing, base); err != nil {
			return AppView{}, err
		}
		return view(existing), nil
	}
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
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	a, err := s.GetApp(ctx, name)
	if err != nil {
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
	// AutoDeploy: default on for a repo-backed service (a push should redeploy,
	// Render's default), off for an image-backed one (no repo to rebuild from).
	// An explicit request value wins.
	autoDeploy := req.Repo != ""
	if req.AutoDeploy != nil {
		autoDeploy = *req.AutoDeploy
	}
	spec := appv1alpha1.AppSpec{
		Type:            svcType,
		Repo:            req.Repo,
		Image:           req.Image,
		Branch:          branch,
		Builder:         req.Builder,
		RootDir:         req.RootDir,
		Port:            port,
		Replicas:        replicas,
		Tier:            tier,
		HealthCheckPath: req.HealthCheckPath,
		Env:             req.Env,
		AutoDeploy:      autoDeploy,
		// Only a web service is public: expose it at <name>.<BEX_BASE_DOMAIN> so the
		// caller gets a live URL with no custom domain. Every other type opts out
		// (private has no platform host; worker/cron have no HTTP endpoint at all).
		Expose: svcType == appv1alpha1.TypeWebService,
	}
	if svcType == appv1alpha1.TypeCronJob {
		spec.Schedule = strings.TrimSpace(req.Schedule)
		spec.Command = strings.TrimSpace(req.Command)
	}
	if len(req.Hosts) > 0 {
		spec.Host = req.Hosts[0]
		spec.Hosts = append([]string(nil), req.Hosts[1:]...)
	}
	return spec, nil
}

// normalizeType resolves the requested service type, tracking Render's
// serviceType vocabulary. Empty defaults to web_service; an unrecognized type is
// rejected.
func normalizeType(t string) (string, error) {
	switch t {
	case "":
		return appv1alpha1.TypeWebService, nil
	case appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob:
		return t, nil
	default:
		return "", fmt.Errorf("%w: type must be one of web_service|private_service|background_worker|cron_job", core.ErrBadRequest)
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
	dst.Builder = want.Builder
	dst.RootDir = want.RootDir
	dst.Port = want.Port
	dst.Replicas = want.Replicas
	dst.Tier = want.Tier
	dst.HealthCheckPath = want.HealthCheckPath
	dst.Env = want.Env
	dst.AutoDeploy = want.AutoDeploy
	dst.Expose = want.Expose
	dst.Host = want.Host
	dst.Hosts = want.Hosts
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
	a, err := s.GetApp(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	// Refresh the clone token so the push-triggered rebuild clones the private
	// repo with a token minted seconds ago.
	secretName, err := s.ensureCloneSecret(ctx, a)
	if err != nil {
		return AppView{}, err
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		if secretName != "" {
			a.Spec.CloneSecret = secretName
		}
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
}

// Restart requests a rolling restart (spec.restartedAt = now). The operator
// stamps the pod template and Kubernetes rolls the pods with no downtime.
func (s *Service) Restart(ctx context.Context, name string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	return s.patch(ctx, name, func(a *appv1alpha1.App) {
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
}

// TriggerCronRun requests a one-off run of a cron_job now (Render's
// POST /cron-jobs/{id}/runs): it bumps spec.runAt (verb-as-timestamp, like
// Restart), and the operator materializes a single Job the run history then
// shows. Rejected for a non-cron service. Intent only — the run appears in
// status.runs once the operator reconciles, not synchronously.
func (s *Service) TriggerCronRun(ctx context.Context, name string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	a, err := s.GetApp(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		return AppView{}, fmt.Errorf("%w: service %q is not a cron_job", core.ErrBadRequest, name)
	}
	return s.patch(ctx, name, func(a *appv1alpha1.App) {
		a.Spec.RunAt = s.Now().UTC().Format(time.RFC3339Nano)
	})
}

// Suspend parks the App (spec.suspended = true): scaled to 0, host/certs kept.
func (s *Service) Suspend(ctx context.Context, name string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	return s.setSuspended(ctx, name, true)
}

// Resume brings a suspended App back (spec.suspended = false); the operator
// restores spec.replicas.
func (s *Service) Resume(ctx context.Context, name string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	return s.setSuspended(ctx, name, false)
}

// SetPlan changes the App's instance size (Render's `plan`, spelled per
// lego/types/tiers). Unknown plans are rejected before any write — the
// caller maps core.ErrInvalid to 400/a GraphQL error, listing the valid
// plans. A plan change resizes the pod (new requests==limits), which is a
// Deployment rollout — the same restart-shaped cost as Render's own plan
// changes.
func (s *Service) SetPlan(ctx context.Context, name, plan string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	t, ok := tiers.Compute.ByRenderPlan(plan)
	if !ok {
		return AppView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Compute.RenderPlans(), "|"))
	}
	tier := t.ID
	return s.writeThroughStore(ctx, name,
		func(ctx context.Context, id string) error { return s.Store.SetAppTier(ctx, id, tier) },
		func(a *appv1alpha1.App) { a.Spec.Tier = tier })
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
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	if replicas < 1 || replicas > store.MaxReplicas {
		return AppView{}, fmt.Errorf("%w: numInstances must be 1-%d", core.ErrBadRequest, store.MaxReplicas)
	}
	return s.writeThroughStore(ctx, name,
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
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	if seconds < 0 || seconds > MaxIdleTTLSeconds {
		return AppView{}, fmt.Errorf("%w: idleTTLSeconds must be 0-%d", core.ErrBadRequest, MaxIdleTTLSeconds)
	}
	return s.writeThroughStore(ctx, name,
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
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	a, err := s.GetApp(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	if a.Spec.Repo == "" {
		return AppView{}, fmt.Errorf("%w: service %q has no repo to build (root directory only applies to build-from-git)", core.ErrBadRequest, name)
	}
	return s.patchFetched(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.RootDir = rootDir
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
}

// SetAutoDeploy flips whether a signed git push to the tracked branch redeploys
// this App (spec.autoDeploy, Render's Auto-Deploy toggle). A direct CR patch,
// not projection-owned (mirrors Builder/RootDir), and no restartedAt bump —
// flipping the toggle changes future push behavior, it does not itself redeploy.
func (s *Service) SetAutoDeploy(ctx context.Context, name string, enabled bool) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	return s.patch(ctx, name, func(a *appv1alpha1.App) {
		a.Spec.AutoDeploy = enabled
	})
}

// setSuspended flips suspension with the row as the single writer of intent.
// Restart needs no row write: spec.restartedAt is not projection-owned.
func (s *Service) setSuspended(ctx context.Context, name string, suspended bool) (AppView, error) {
	return s.writeThroughStore(ctx, name,
		func(ctx context.Context, id string) error { return s.Store.SetAppSuspended(ctx, id, suspended) },
		func(a *appv1alpha1.App) { a.Spec.Suspended = suspended })
}

// writeThroughStore is the shared shape of every intent-field verb with a row
// as the single writer of truth (suspend/resume, plan): for store-managed Apps
// the row is updated first — the projection loop owns the field and would
// revert a bare CR patch on the next resync — then the CR patch after it makes
// the change converge immediately; if the row write fails, the CR is left
// untouched (the row is already wrong, so retrying is safe). Unmanaged
// (bare-CR) Apps skip the row entirely and go straight to the CR patch. One
// GetApp serves both: it is the shared fetch (gated by the caller's tenant,
// see core.Base.GetApp) and the row write's source for the store's app-id.
func (s *Service) writeThroughStore(
	ctx context.Context, name string,
	writeRow func(ctx context.Context, id string) error,
	mutate func(*appv1alpha1.App),
) (AppView, error) {
	a, err := s.GetApp(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	if s.Store != nil {
		if id := a.Labels[store.LabelAppID]; id != "" {
			if err := writeRow(ctx, id); err != nil {
				return AppView{}, fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	return s.patchFetched(ctx, a, mutate)
}

// patch fetches the App by name (gated by the caller's tenant, see
// core.Base.GetApp) then applies mutate. Restart/redeploy's single-fetch shape;
// writeThroughStore instead reuses the App it already fetched via patchFetched.
func (s *Service) patch(ctx context.Context, name string, mutate func(*appv1alpha1.App)) (AppView, error) {
	a, err := s.GetApp(ctx, name)
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
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return AutoscalingView{}, err
	}
	a, err := s.GetApp(ctx, name)
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
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
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
	a, err := s.GetApp(ctx, name)
	if err != nil {
		return AutoscalingView{}, err
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
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return err
	}
	a, err := s.GetApp(ctx, name)
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
