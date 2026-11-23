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

package deploys

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render Build Source builds (w2/m254 t006, docs/ADR097-build-sources.md). A
// build is a deploy row of the build source's carrier App: triggering one is a
// deploy of the carrier, and its public id is that row's id as a bld- id.

// BuildRunView is Render's buildRun: the run that produced a git build.
type BuildRunView struct {
	ID              string `json:"id"`
	Status          string `json:"status,omitempty"`
	BuildStartedAt  string `json:"buildStartedAt,omitempty"`
	BuildFinishedAt string `json:"buildFinishedAt,omitempty"`
	Runtime         string `json:"runtime,omitempty"`
	CommitID        string `json:"commitId,omitempty"`
	CommitURL       string `json:"commitUrl,omitempty"`
}

// BuildImageView is Render's image: an image-backed build's identity.
type BuildImageView struct {
	SHA                  string  `json:"SHA,omitempty"`
	Ref                  string  `json:"ref,omitempty"`
	ImageURL             string  `json:"imageUrl,omitempty"`
	RegistryCredentialID *string `json:"registryCredentialId,omitempty"`
}

// imageRef is the tag or digest an image URL names ("" when it names none).
func imageRef(image string) string {
	if _, digest, ok := strings.Cut(image, "@"); ok {
		return digest
	}
	if i := strings.LastIndex(image, ":"); i > strings.LastIndex(image, "/") {
		return image[i+1:]
	}
	return ""
}

// BuildView is Render's build object.
type BuildView struct {
	ID              string                `json:"id"`
	BuildSourceID   string                `json:"buildSourceId"`
	BuildRun        *BuildRunView         `json:"buildRun,omitempty"`
	Image           *BuildImageView       `json:"image,omitempty"`
	CreatedAt       string                `json:"createdAt"`
	UpdatedAt       string                `json:"updatedAt"`
	Deploys         []BuildDeployRef      `json:"-"`
	SkippedServices []SkippedBuildService `json:"-"`

	deployID string
}

// Render's trigger-build-source-build deploy.serviceScope values (round 9,
// w4/m596 t002).
const (
	BuildDeployScopeAutoDeployOnly = store.BuildDeployScopeAutoDeployOnly
	BuildDeployScopeNone           = store.BuildDeployScopeNone
	BuildDeployScopeCustom         = store.BuildDeployScopeCustom
)

// Render's trigger-build-source-build skippedServices[].reason values.
// bex never emits deploys_shut_off because it has no per-plan-tier deploy
// shut-off; a billing-enforced workspace refuses the build trigger itself.
const (
	SkipReasonSuspended      = "suspended"
	SkipReasonDeploysShutOff = "deploys_shut_off"
)

// BuildDeployScope is Render's trigger-build-source-build deploy body object.
type BuildDeployScope struct {
	ServiceScope string    `json:"serviceScope"`
	ServiceIDs   *[]string `json:"serviceIds,omitempty"`
}

// BuildDeployRef is one entry of Render's trigger-build-source-build 201
// deploys array.
type BuildDeployRef struct {
	ServiceID string `json:"serviceId"`
	DeployID  string `json:"deployId"`
}

// SkippedBuildService is one entry of Render's trigger-build-source-build 201
// skippedServices array.
type SkippedBuildService struct {
	ServiceID string `json:"serviceId"`
	Reason    string `json:"reason"`
}

// TriggerBuildResult is Render's 201 for trigger-build-source-build: the build
// plus the deploys it started and the services in scope it skipped.
type TriggerBuildResult struct {
	BuildView
	Deploys         []BuildDeployRef      `json:"deploys"`
	SkippedServices []SkippedBuildService `json:"skippedServices"`
}

// buildAwaitingDeployStore is the store surface for pre-created service deploy
// rows waiting on a build source build (w4/m596 t002). *store.PGStore satisfies
// it.
type buildAwaitingDeployStore interface {
	CreateAwaitingBuildDeploy(ctx context.Context, appID, trigger, buildID string, commit store.CommitInfo, triggeredBy string) (store.Deploy, error)
	ListBuildAwaitingDeploys(ctx context.Context, buildID string) ([]store.Deploy, error)
	CancelAwaitingBuildDeploy(ctx context.Context, id, reason string) (bool, error)
	CheckAwaitingBuildDeploy(ctx context.Context, id, appID string) error
	DeferAwaitingBuildDeploy(ctx context.Context, id, appID string, req store.DeferredRequest) (store.Deploy, bool, error)
}

var _ buildAwaitingDeployStore = (*store.PGStore)(nil)

func parseBuildDeployScope(d *BuildDeployScope) (scope string, serviceIDs []string, err error) {
	if d == nil {
		return BuildDeployScopeAutoDeployOnly, nil, nil
	}
	switch d.ServiceScope {
	case BuildDeployScopeAutoDeployOnly, BuildDeployScopeNone:
		if d.ServiceIDs != nil {
			return "", nil, fmt.Errorf("%w: deploy.serviceIds is only allowed when deploy.serviceScope is %q", core.ErrBadRequest, BuildDeployScopeCustom)
		}
		return d.ServiceScope, nil, nil
	case BuildDeployScopeCustom:
		if d.ServiceIDs == nil || len(*d.ServiceIDs) == 0 {
			return "", nil, fmt.Errorf("%w: deploy.serviceIds is required when deploy.serviceScope is %q", core.ErrBadRequest, BuildDeployScopeCustom)
		}
		seen := make(map[string]bool, len(*d.ServiceIDs))
		out := make([]string, 0, len(*d.ServiceIDs))
		for _, sid := range *d.ServiceIDs {
			sid = strings.TrimSpace(sid)
			if sid == "" {
				return "", nil, fmt.Errorf("%w: deploy.serviceIds must not contain an empty service id", core.ErrBadRequest)
			}
			if !seen[sid] {
				seen[sid] = true
				out = append(out, sid)
			}
		}
		return BuildDeployScopeCustom, out, nil
	default:
		return "", nil, fmt.Errorf("%w: deploy.serviceScope must be one of %s, %s, %s", core.ErrBadRequest, BuildDeployScopeAutoDeployOnly, BuildDeployScopeNone, BuildDeployScopeCustom)
	}
}

// BuildID is the public id of the build a carrier deploy row is.
func BuildID(deployID string) string {
	id, _ := ids.Rekind(ids.Build, deployID)
	return id
}

// buildDeployID is BuildID's inverse: the carrier deploy row buildID names.
func buildDeployID(buildID string) (string, bool) {
	if k, ok := ids.KindOf(buildID); !ok || k != ids.Build {
		return "", false
	}
	return ids.Rekind(ids.Deploy, buildID)
}

// buildSourceApp authorizes relation on build source id. Unexported so the
// audit verb walk names the exported verb calling it.
func (s *Service) buildSourceApp(ctx context.Context, relation, id string) (context.Context, *appv1alpha1.App, error) {
	ctx = core.WithBuildSources(ctx)
	a, err := s.AuthorizeApp(ctx, relation, id)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, a, core.BuildSourceOrNotFound(a)
}

// TriggerBuild is Render's trigger-build-source-build: a new build of the
// source's branch head even when an up-to-date build exists. Only a git
// source builds. clearCache is Render's optional "clear" | "do_not_clear"
// (round 7, w3/m367), honored as a service deploy's is; "" is do_not_clear.
// deploy is Render's optional deploy scope (round 9, w4/m596 t002): omitted
// defaults to autodeploy_only. Each targeted linked service gets a readable
// deploy row immediately (awaiting_build), promoted in place when the build
// succeeds or closed terminal if the build fails or is canceled.
func (s *Service) TriggerBuild(ctx context.Context, id, clearCache string, deploy ...*BuildDeployScope) (TriggerBuildResult, error) {
	origCtx := ctx
	ctx, carrier, err := s.buildSourceApp(ctx, core.LifecycleOrCreate(false), id)
	if err != nil {
		return TriggerBuildResult{}, err
	}
	if carrier.Spec.Repo == "" {
		return TriggerBuildResult{}, fmt.Errorf("%w: only a git build source can be built; build source %s is image-backed", core.ErrBadRequest, id)
	}
	var deployOpt *BuildDeployScope
	if len(deploy) > 0 {
		deployOpt = deploy[0]
	}
	scope, serviceIDs, err := parseBuildDeployScope(deployOpt)
	if err != nil {
		return TriggerBuildResult{}, err
	}
	if err := s.RequireBillingMutation(ctx, carrier.Labels[core.LabelTenant]); err != nil {
		return TriggerBuildResult{}, err
	}

	targets, skipped, err := s.resolveBuildDeployTargets(origCtx, carrier, scope, serviceIDs)
	if err != nil {
		return TriggerBuildResult{}, err
	}

	d, err := s.triggerFetched(store.WithBuildDeployScope(ctx, scope), id, carrier, TriggerParams{ClearCache: clearCache}, store.TriggerAPI)
	if err != nil {
		return TriggerBuildResult{}, err
	}

	buildID := BuildID(d.ID)
	refs := make([]BuildDeployRef, 0, len(targets))
	if len(targets) > 0 {
		aw, ok := s.Store.(buildAwaitingDeployStore)
		if !ok {
			return TriggerBuildResult{}, fmt.Errorf("%w: build deploy scope requires the control plane", core.ErrUnavailable)
		}
		commit := store.CommitInfo{Hash: d.CommitID, Message: d.CommitMessage, AuthorAt: d.CommitAuthorAt}
		triggeredBy := core.SubjectFrom(ctx)
		for _, target := range targets {
			planned, err := aw.CreateAwaitingBuildDeploy(ctx, appStoreID(target), store.TriggerAPI, buildID, commit, triggeredBy)
			if err != nil {
				return TriggerBuildResult{}, err
			}
			refs = append(refs, BuildDeployRef{
				ServiceID: core.AppPublicID(target),
				DeployID:  planned.ID,
			})
		}
	}
	if skipped == nil {
		skipped = []SkippedBuildService{}
	}
	bv := buildView(carrier, core.AppPublicID(carrier), d)
	bv.Deploys = refs
	bv.SkippedServices = skipped
	return TriggerBuildResult{
		BuildView:       bv,
		Deploys:         refs,
		SkippedServices: skipped,
	}, nil
}

// resolveBuildDeployTargets selects the linked services to deploy and the
// services in scope to skip, verifying per-service deploy authorization for
// custom serviceIds before the carrier build is opened.
func (s *Service) resolveBuildDeployTargets(origCtx context.Context, carrier *appv1alpha1.App, scope string, serviceIDs []string) ([]*appv1alpha1.App, []SkippedBuildService, error) {
	switch scope {
	case BuildDeployScopeNone:
		return nil, []SkippedBuildService{}, nil
	case BuildDeployScopeCustom:
		var targets []*appv1alpha1.App
		skipped := []SkippedBuildService{}
		for _, sid := range serviceIDs {
			a, err := s.AuthorizeApp(origCtx, core.LifecycleOrCreate(false), sid)
			if err != nil {
				return nil, nil, err
			}
			if err := core.NotFoundIfDeleting(a); err != nil {
				return nil, nil, err
			}
			if a.Spec.BuildSource == nil || a.Spec.BuildSource.Name != carrier.Name {
				return nil, nil, fmt.Errorf("%w: deploy.serviceIds: service %s is not linked to build source %s", core.ErrBadRequest, sid, core.AppPublicID(carrier))
			}
			if a.Spec.Suspended {
				skipped = append(skipped, SkippedBuildService{
					ServiceID: core.AppPublicID(a),
					Reason:    SkipReasonSuspended,
				})
				continue
			}
			targets = append(targets, a)
		}
		return targets, skipped, nil
	case BuildDeployScopeAutoDeployOnly:
		st, ok := s.Store.(buildSourceStore)
		if !ok {
			return nil, []SkippedBuildService{}, nil
		}
		links, err := st.ListBuildSourceLinks(origCtx, appStoreID(carrier))
		if err != nil {
			return nil, nil, err
		}
		var targets []*appv1alpha1.App
		for _, row := range links {
			// For autodeploy_only, Render does not list suspended services in
			// skippedServices.
			if row.Suspended {
				continue
			}
			var a appv1alpha1.App
			if err := s.Client.Get(origCtx, client.ObjectKey{Namespace: carrier.Namespace, Name: row.AppObjectName()}, &a); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return nil, nil, err
			}
			if !a.Spec.AutoDeploy || a.Spec.Suspended || a.DeletionTimestamp != nil {
				continue
			}
			cp := a
			targets = append(targets, &cp)
		}
		return targets, []SkippedBuildService{}, nil
	default:
		return nil, nil, fmt.Errorf("%w: unknown deploy.serviceScope %q", core.ErrBadRequest, scope)
	}
}

// ListBuilds lists a page of a build source's builds, newest first. cursor
// is the last page's final cursor (that build's carrier deploy id).
func (s *Service) ListBuilds(ctx context.Context, id, cursor string, limit int) ([]BuildView, error) {
	ctx, carrier, err := s.buildSourceApp(ctx, core.RelCanView, id)
	if err != nil {
		return nil, err
	}
	deploys, err := s.listFetched(ctx, carrier, ListFilter{Cursor: cursor, Limit: limit})
	if err != nil {
		return nil, err
	}
	sourceID := core.AppPublicID(carrier)
	out := make([]BuildView, len(deploys))
	for i, d := range deploys {
		out[i] = buildView(carrier, sourceID, d)
	}
	return out, nil
}

// GetBuild returns one build of a build source.
func (s *Service) GetBuild(ctx context.Context, id, buildID string) (BuildView, error) {
	ctx, carrier, err := s.buildSourceApp(ctx, core.RelCanView, id)
	if err != nil {
		return BuildView{}, err
	}
	deployID, ok := buildDeployID(buildID)
	if !ok {
		return BuildView{}, core.NotFound("build")
	}
	d, err := s.getFetched(ctx, carrier, deployID)
	if errors.Is(err, core.ErrNotFound) {
		return BuildView{}, core.NotFound("build")
	}
	if err != nil {
		return BuildView{}, err
	}
	return buildView(carrier, core.AppPublicID(carrier), d), nil
}

func buildView(carrier *appv1alpha1.App, sourceID string, d DeployView) BuildView {
	v := BuildView{ID: BuildID(d.ID), BuildSourceID: sourceID, deployID: d.ID,
		CreatedAt: formatTime(d.CreatedAt), UpdatedAt: formatTime(d.UpdatedAt)}
	if carrier.Spec.Repo == "" {
		v.Image = &BuildImageView{ImageURL: d.Image, SHA: d.ImageSHA, Ref: imageRef(d.Image), RegistryCredentialID: carrier.Spec.RegistryCredentialID}
		return v
	}
	v.BuildRun = &BuildRunView{ID: d.ID, Status: store.BuildRunStatus(d.Status), Runtime: core.EffectiveRuntime(carrier.Spec, carrier.Spec.Type), CommitID: d.CommitID, CommitURL: core.CommitURL(carrier.Spec.Repo, d.CommitID),
		BuildStartedAt: formatTimePtr(d.StartedAt), BuildFinishedAt: formatTimePtr(d.FinishedAt)}
	return v
}

// sourceBuild is the build source build a deploy of a linked service ships:
// the link selecting its image, and its public bld- id ("" when unknown).
type sourceBuild struct {
	link *appv1alpha1.BuildSourceLink
	id   string
	// fanout marks a build source fan-out's deploy: it ships only when the
	// service is not already on this build, decided under the trigger lock.
	fanout bool
	// commit is what the build built, so the service's deploy reports the
	// build's commit as Render's does.
	commit store.CommitInfo
}

// Render's create-deploy refusals for buildId.
const (
	buildNotFoundCode     = "build_not_found"
	buildNotSucceededCode = "build_not_succeeded"
)

// linkedBuild is the build a deploy of linked service a ships: buildID when
// the caller chose one (Render's create-deploy buildId), else the source's
// current build. nil for a service with no build source.
func (s *Service) linkedBuild(ctx context.Context, a *appv1alpha1.App, buildID string) (*sourceBuild, error) {
	if a.Spec.BuildSource == nil {
		return nil, nil
	}
	var carrier appv1alpha1.App
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: a.Spec.BuildSource.Name}, &carrier)
	if apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("%w: the build source of service %s no longer exists", core.ErrConflict, a.Name)
	}
	if err != nil {
		return nil, fmt.Errorf("read build source of %s: %w", a.Name, err)
	}
	if buildID != "" {
		return s.chosenBuild(ctx, &carrier, buildID)
	}
	link := core.BuildSourceLinkTo(&carrier)
	if link.Image == "" {
		return nil, fmt.Errorf("%w: build source %s has not built yet; trigger a build first", core.ErrConflict, core.BuildSourceIDOf(a))
	}
	// The current build is the carrier deploy row serving it; the id is
	// provenance only, so an unresolved row ships the build without one.
	b := &sourceBuild{link: link}
	if d, err := s.servingDeploy(ctx, carrier.Name, &carrier); err == nil && d.ID != "" {
		b.id, b.commit = BuildID(d.ID), d.CommitInfo()
	}
	return b, nil
}

// chosenBuild resolves buildID as a succeeded build of carrier.
func (s *Service) chosenBuild(ctx context.Context, carrier *appv1alpha1.App, buildID string) (*sourceBuild, error) {
	notFound := core.NewNotFoundError(buildNotFoundCode, fmt.Sprintf("build %s not found in this service's build source", buildID), map[string]any{"buildId": buildID})
	deployID, ok := buildDeployID(buildID)
	if !ok {
		return nil, notFound
	}
	d, err := s.Store.GetDeploy(ctx, appStoreID(carrier), deployID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, notFound
	}
	if err != nil {
		return nil, err
	}
	// A succeeded build is one a rollback could restore: it went live with
	// a resolved image.
	if !RollbackEligible(carrier, d) {
		return nil, core.NewConflictError(buildNotSucceededCode, fmt.Sprintf("build %s has not succeeded", buildID), map[string]any{"buildId": buildID})
	}
	return &sourceBuild{link: &appv1alpha1.BuildSourceLink{Name: carrier.Name, Image: d.ResolvedImage}, id: buildID, commit: d.CommitInfo()}, nil
}

// buildSourceStore is the control-plane store surface linked services need.
type buildSourceStore interface {
	SetAppBuildSourceImage(ctx context.Context, id, carrierID, image string) error
	ListBuildSourceLinks(ctx context.Context, carrierID string) ([]store.App, error)
}

func (s *Service) buildSourceStore() (buildSourceStore, error) {
	st, ok := s.Store.(buildSourceStore)
	if !ok {
		return nil, fmt.Errorf("%w: build sources need the control plane", core.ErrUnavailable)
	}
	return st, nil
}

// errAlreadyOnBuild skips a fan-out deploy of a service already on the build:
// another fan-out of the same build got there first.
var errAlreadyOnBuild = errors.New("the service is already on this build")

// linkedBuildSelector is the store's conditional build write (PGStore): it
// reports whether the row changed, so a fan-out deploys a service once.
type linkedBuildSelector interface {
	SelectAppBuildSourceImage(ctx context.Context, id, carrierID, image string) (bool, error)
}

// selectLinkedBuild records image as the build linked service appID deploys,
// in the row the projector owns spec.buildSource from. The write holds only
// while the row still links carrierID, so a deploy racing a re-link can never
// pair one source's build with another source. A fan-out's write (onlyNew)
// runs under the trigger lock and returns errAlreadyOnBuild when the row is
// already on image: two fan-outs of one build, a resync overlapping a slow
// first run, open one deploy per service between them.
func (s *Service) selectLinkedBuild(ctx context.Context, appID, carrierID, image string, onlyNew bool) error {
	st, err := s.buildSourceStore()
	if err != nil {
		return err
	}
	if sel, ok := s.Store.(linkedBuildSelector); ok && onlyNew {
		changed, err := sel.SelectAppBuildSourceImage(ctx, appID, carrierID, image)
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: the service's build source changed during the deploy; deploy again", core.ErrConflict)
		}
		if err == nil && !changed {
			return errAlreadyOnBuild
		}
		return err
	}
	err = st.SetAppBuildSourceImage(ctx, appID, carrierID, image)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("%w: the service's build source changed during the deploy; deploy again", core.ErrConflict)
	}
	return err
}

// BuildSucceeded deploys a carrier's new build to every service linked to it
// with auto-deploy on (Render: "services linked to the build source with
// autodeploy enabled are deployed with the result"), and to any linked service
// still waiting for the source's first build, whose first deploy this
// completes. The reconciler calls it when a carrier deploy goes live, with no
// caller identity. Every service ships b.Image, even if a newer build lands
// mid fan-out (that build fans out itself); a service already on b.Image is
// left alone, checked again under its trigger lock, so a re-run of an
// interrupted fan-out, or one overlapping it, opens no second deploy. The
// error joins every service that failed transiently, so the reconciler
// retries the fan-out (store.BuildFanoutLedger); a refusal that a retry would
// meet again (billing enforcement, a missing or re-linked service) is logged
// and the fan-out completes without it.
func (s *Service) BuildSucceeded(ctx context.Context, b store.BuiltSource) error {
	if b.Image == "" {
		return nil
	}
	if b.BuildDeployScope == BuildDeployScopeNone {
		return nil
	}
	wrap := func(target string, err error) error {
		log.Printf("deploys: fan out build of %s to %s: %v", b.Carrier, target, err)
		return fmt.Errorf("fan out build of %s to %s: %w", b.Carrier, target, err)
	}
	if b.BuildDeployScope == BuildDeployScopeCustom || b.BuildDeployScope == BuildDeployScopeAutoDeployOnly {
		return s.fanoutPlannedBuildDeploys(ctx, b, wrap)
	}
	st, err := s.buildSourceStore()
	if err != nil {
		return wrap("its services", err)
	}
	links, err := st.ListBuildSourceLinks(ctx, b.CarrierID)
	if err != nil {
		return wrap("its services", err)
	}
	if len(links) == 0 {
		return nil
	}
	if err := s.RequireBillingMutation(ctx, b.TenantID); err != nil {
		if permanentFanoutError(err) {
			log.Printf("deploys: fan out build of %s skipped: %v", b.Carrier, err)
			return nil
		}
		return wrap("its services", err)
	}
	build := &sourceBuild{link: &appv1alpha1.BuildSourceLink{Name: b.Carrier, Image: b.Image}, id: BuildID(b.DeployID), commit: b.Commit, fanout: true}
	var errs []error
	for _, row := range links {
		waiting := row.BuildSourceImage == ""
		if row.Suspended || row.BuildSourceImage == b.Image {
			continue
		}
		var a appv1alpha1.App
		if err := s.Client.Get(ctx, client.ObjectKey{Namespace: b.Namespace, Name: row.AppObjectName()}, &a); err != nil {
			errs = append(errs, fanoutFailure(wrap(row.ID, err)))
			continue
		}
		if (!a.Spec.AutoDeploy && !waiting) || a.Spec.Suspended || a.DeletionTimestamp != nil {
			continue
		}
		_, err := s.triggerFetched(ctx, core.AppPublicName(&a), &a, TriggerParams{build: build, firstBuild: waiting}, store.TriggerNewCommit)
		if err != nil && !errors.Is(err, errAlreadyOnBuild) {
			errs = append(errs, fanoutFailure(wrap(row.ID, err)))
		}
	}
	return errors.Join(errs...)
}

// fanoutPlannedBuildDeploys promotes each pre-created awaiting-build service
// deploy for a trigger-build-source-build run (autodeploy_only or custom).
// Retries from BuildFanoutLedger only see rows still awaiting_build, so the
// scope chosen at trigger time is preserved across restarts and partial runs.
func (s *Service) fanoutPlannedBuildDeploys(ctx context.Context, b store.BuiltSource, wrap func(string, error) error) error {
	aw, ok := s.Store.(buildAwaitingDeployStore)
	if !ok {
		return nil
	}
	planned, err := aw.ListBuildAwaitingDeploys(ctx, BuildID(b.DeployID))
	if err != nil {
		return wrap("its planned deploys", err)
	}
	if len(planned) == 0 {
		return nil
	}
	cancelPlanned := func(id, reason string) {
		if _, err := aw.CancelAwaitingBuildDeploy(ctx, id, reason); err != nil {
			log.Printf("deploys: cancel planned build deploy %s: %v", id, err)
		}
	}
	if err := s.RequireBillingMutation(ctx, b.TenantID); err != nil {
		if permanentFanoutError(err) {
			log.Printf("deploys: fan out build of %s skipped: %v", b.Carrier, err)
			for _, wd := range planned {
				cancelPlanned(wd.ID, err.Error())
			}
			return nil
		}
		return wrap("its planned deploys", err)
	}
	build := &sourceBuild{link: &appv1alpha1.BuildSourceLink{Name: b.Carrier, Image: b.Image}, id: BuildID(b.DeployID), commit: b.Commit, fanout: true}
	var errs []error
	for _, wd := range planned {
		var list appv1alpha1.AppList
		opts := []client.ListOption{client.MatchingLabels{core.LabelAppID: wd.AppID}}
		if b.Namespace != "" {
			opts = append(opts, client.InNamespace(b.Namespace))
		}
		if err := s.Client.List(ctx, &list, opts...); err != nil {
			errs = append(errs, fanoutFailure(wrap(wd.AppID, err)))
			continue
		}
		items := core.ServiceApps(list.Items)
		if len(items) != 1 || items[0].DeletionTimestamp != nil {
			cancelPlanned(wd.ID, "the service was deleted")
			continue
		}
		a := &items[0]
		if a.Spec.Suspended {
			cancelPlanned(wd.ID, "the service is suspended")
			continue
		}
		if a.Spec.BuildSource == nil || a.Spec.BuildSource.Name != b.Carrier {
			cancelPlanned(wd.ID, "the service was unlinked from the build source")
			continue
		}
		trigger := wd.Trigger
		if trigger == "" {
			trigger = store.TriggerAPI
		}
		_, err := s.triggerFetched(store.WithAwaitingBuildDeploy(ctx, wd.ID), core.AppPublicName(a), a, TriggerParams{build: build}, trigger)
		if err == nil || errors.Is(err, store.ErrAwaitingBuildDeployGone) || errors.Is(err, errAlreadyOnBuild) {
			continue
		}
		if permanentFanoutError(err) {
			log.Printf("deploys: cancel planned build deploy %s for %s: %v", wd.ID, wd.AppID, err)
			cancelPlanned(wd.ID, err.Error())
			continue
		}
		errs = append(errs, wrap(wd.AppID, err))
	}
	return errors.Join(errs...)
}

// fanoutFailure is err when a retry could succeed, nil when it would meet the
// same refusal: the fan-out then completes without that service (logged by
// wrap), instead of re-running every lease period until a newer build lands.
func fanoutFailure(err error) error {
	if permanentFanoutError(err) {
		return nil
	}
	return err
}

// permanentFanoutError reports a refusal that does not change on retry: the
// service or its CR is gone, it was re-linked, a request-shape refusal, or the
// workspace's billing is enforced (its owner deploys by hand once settled).
func permanentFanoutError(err error) bool {
	return apierrors.IsNotFound(err) ||
		errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrConflict) || errors.Is(err, core.ErrBadRequest) ||
		errors.Is(err, core.ErrForbidden) || errors.Is(err, core.ErrGone) ||
		errors.Is(err, core.ErrBillingEnforced) || errors.Is(err, core.ErrPaymentRequired)
}

type buildWithCursor struct {
	Build  BuildView `json:"build"`
	Cursor string    `json:"cursor"`
}

func toBuildList(builds []BuildView) []buildWithCursor {
	out := make([]buildWithCursor, 0, len(builds))
	for _, b := range builds {
		out = append(out, buildWithCursor{Build: b, Cursor: b.deployID})
	}
	return out
}

func (s *Service) registerBuildSourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/build-sources/{buildSourceId}/builds", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		var body struct {
			ClearCache string            `json:"clearCache"`
			Deploy     *BuildDeployScope `json:"deploy"`
		}
		if err := core.DecodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: %v", core.ErrBadRequest, err)
		}
		return s.TriggerBuild(r.Context(), r.PathValue("buildSourceId"), body.ClearCache, body.Deploy)
	}))
	mux.HandleFunc("GET /v1/build-sources/{buildSourceId}/builds", core.HandleMapped(http.StatusOK, func(r *http.Request) ([]BuildView, error) {
		cursor, limit := core.PageParams(r.URL.Query())
		return s.ListBuilds(r.Context(), r.PathValue("buildSourceId"), cursor, limit)
	}, toBuildList))
	mux.HandleFunc("GET /v1/build-sources/{buildSourceId}/builds/{buildId}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetBuild(r.Context(), r.PathValue("buildSourceId"), r.PathValue("buildId"))
	}))
}

var buildRunGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildRun",
	Fields: graphql.Fields{
		"id":              gqlutil.StrField(func(r *BuildRunView) any { return r.ID }),
		"status":          gqlutil.StrField(func(r *BuildRunView) any { return r.Status }),
		"buildStartedAt":  gqlutil.OptionalStrField(func(r *BuildRunView) any { return r.BuildStartedAt }),
		"buildFinishedAt": gqlutil.OptionalStrField(func(r *BuildRunView) any { return r.BuildFinishedAt }),
		"runtime":         gqlutil.OptionalStrField(func(r *BuildRunView) any { return r.Runtime }),
		"commitId":        gqlutil.OptionalStrField(func(r *BuildRunView) any { return r.CommitID }),
		"commitUrl":       gqlutil.OptionalStrField(func(r *BuildRunView) any { return r.CommitURL }),
	},
})

var buildImageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildImage",
	Fields: graphql.Fields{
		"sha":                  gqlutil.OptionalStrField(func(i *BuildImageView) any { return i.SHA }),
		"ref":                  gqlutil.OptionalStrField(func(i *BuildImageView) any { return i.Ref }),
		"imageUrl":             gqlutil.OptionalStrField(func(i *BuildImageView) any { return i.ImageURL }),
		"registryCredentialId": gqlutil.StrField(func(i *BuildImageView) any { return i.RegistryCredentialID }),
	},
})

var buildDeployRefGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildDeployRef",
	Fields: graphql.Fields{
		"serviceId": gqlutil.StrField(func(r BuildDeployRef) any { return r.ServiceID }),
		"deployId":  gqlutil.StrField(func(r BuildDeployRef) any { return r.DeployID }),
	},
})

var skippedBuildServiceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SkippedBuildService",
	Fields: graphql.Fields{
		"serviceId": gqlutil.StrField(func(s SkippedBuildService) any { return s.ServiceID }),
		"reason":    gqlutil.StrField(func(s SkippedBuildService) any { return s.Reason }),
	},
})

var buildDeployScopeInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "BuildDeployScopeInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"serviceScope": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"serviceIds":   &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
	},
})

var buildGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Build",
	Fields: graphql.Fields{
		"id":              gqlutil.StrField(func(b BuildView) any { return b.ID }),
		"buildSourceId":   gqlutil.StrField(func(b BuildView) any { return b.BuildSourceID }),
		"buildRun":        gqlutil.Typed(buildRunGQLType, func(b BuildView) any { return b.BuildRun }),
		"image":           gqlutil.Typed(buildImageGQLType, func(b BuildView) any { return b.Image }),
		"deploys":         gqlutil.Typed(graphql.NewList(buildDeployRefGQLType), func(b BuildView) any { return b.Deploys }),
		"skippedServices": gqlutil.Typed(graphql.NewList(skippedBuildServiceGQLType), func(b BuildView) any { return b.SkippedServices }),
		"cursor":          gqlutil.StrField(func(b BuildView) any { return b.deployID }),
		"createdAt":       gqlutil.StrField(func(b BuildView) any { return b.CreatedAt }),
		"updatedAt":       gqlutil.StrField(func(b BuildView) any { return b.UpdatedAt }),
	},
})

func (s *Service) buildGQLQueryFields() graphql.Fields {
	return graphql.Fields{
		"builds": &graphql.Field{
			Type: graphql.NewList(buildGQLType),
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{"buildSourceId": gqlutil.ReqArg(graphql.String)}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListBuilds(p.Context, p.Args["buildSourceId"].(string), gqlutil.Str(p.Args, "cursor"), core.PageLimitOrDefault(gqlutil.Int(p.Args, "limit")))
			},
		},
		"build": &graphql.Field{
			Type: buildGQLType,
			Args: graphql.FieldConfigArgument{
				"buildSourceId": gqlutil.ReqArg(graphql.String),
				"id":            gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetBuild(p.Context, p.Args["buildSourceId"].(string), p.Args["id"].(string))
			},
		},
	}
}

func (s *Service) buildGQLMutationFields() graphql.Fields {
	return graphql.Fields{
		"triggerBuild": &graphql.Field{
			Type: buildGQLType,
			Args: graphql.FieldConfigArgument{
				"buildSourceId": gqlutil.ReqArg(graphql.String),
				"clearCache":    gqlutil.Arg(graphql.String),
				"deploy":        gqlutil.Arg(buildDeployScopeInputGQLType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				var deployOpt *BuildDeployScope
				if raw, ok := p.Args["deploy"].(map[string]any); ok && raw != nil {
					scope, _ := raw["serviceScope"].(string)
					deployOpt = &BuildDeployScope{ServiceScope: scope}
					if idsRaw, present := raw["serviceIds"]; present && idsRaw != nil {
						if list, ok := idsRaw.([]any); ok {
							sids := make([]string, 0, len(list))
							for _, item := range list {
								if s, ok := item.(string); ok {
									sids = append(sids, s)
								}
							}
							deployOpt.ServiceIDs = &sids
						}
					}
				}
				res, err := s.TriggerBuild(p.Context, p.Args["buildSourceId"].(string), gqlutil.Str(p.Args, "clearCache"), deployOpt)
				if err != nil {
					return nil, err
				}
				return res.BuildView, nil
			},
		},
	}
}

type buildSourceArgs struct {
	BuildSourceID string `json:"buildSourceId" jsonschema:"the build source id (bds-…)"`
}

type triggerBuildArgs struct {
	buildSourceArgs
	// ClearCache is a boolean, as on trigger_deploy (Render's MCP shape), or
	// Render's REST strings clear and do_not_clear.
	ClearCache any `json:"clearCache,omitempty" jsonschema:"true (or the REST string clear) builds without importing prior cached layers, as trigger_deploy's clearCache does"`
	// Deploy selects which linked services the build deploys to (Render's
	// deploy.serviceScope: autodeploy_only, none, or custom with serviceIds).
	Deploy *BuildDeployScope `json:"deploy,omitempty" jsonschema:"optional deploy scope: {serviceScope: autodeploy_only|none|custom, serviceIds?: [...]}; omitted defaults to autodeploy_only"`
}

type listBuildsArgs struct {
	BuildSourceID string `json:"buildSourceId" jsonschema:"the build source id (bds-…)"`
	Cursor        string `json:"cursor,omitempty" jsonschema:"the cursor of the previous page's last build"`
	Limit         int    `json:"limit,omitempty" jsonschema:"page size, default 20, at most 100"`
}

type listBuildsResult struct {
	Builds []buildWithCursor `json:"builds"`
}

type getBuildArgs struct {
	BuildSourceID string `json:"buildSourceId" jsonschema:"the build source id (bds-…)"`
	BuildID       string `json:"buildId" jsonschema:"the build id (bld-…)"`
}

func (s *Service) registerBuildTools(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "trigger_build_source_build",
		Description: "Build a git build source's branch head, even when an up-to-date build exists. clearCache: true (or the REST string clear) builds without prior cached layers, as Render's trigger-build-source-build. Optional deploy.serviceScope (autodeploy_only, none, or custom with serviceIds) chooses which linked services deploy; returns deploys and skippedServices. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in triggerBuildArgs) (*mcp.CallToolResult, TriggerBuildResult, error) {
		clearCache, err := mcputil.BoolOrEnum("clearCache", in.ClearCache, "clear", "do_not_clear")
		if err != nil {
			return nil, TriggerBuildResult{}, err
		}
		b, err := s.TriggerBuild(ctx, in.BuildSourceID, clearCache, in.Deploy)
		return nil, b, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_build_source_builds",
		Description: "List a build source's builds, newest first, with a cursor per build for paging. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listBuildsArgs) (*mcp.CallToolResult, listBuildsResult, error) {
		builds, err := s.ListBuilds(ctx, in.BuildSourceID, in.Cursor, core.PageLimitOrDefault(in.Limit))
		return nil, listBuildsResult{Builds: toBuildList(builds)}, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_build_source_build",
		Description: "Get one build of a build source: its run status, commit and timing, or the image for an image-backed source. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getBuildArgs) (*mcp.CallToolResult, BuildView, error) {
		b, err := s.GetBuild(ctx, in.BuildSourceID, in.BuildID)
		return nil, b, err
	})
}
