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
	ID            string          `json:"id"`
	BuildSourceID string          `json:"buildSourceId"`
	BuildRun      *BuildRunView   `json:"buildRun,omitempty"`
	Image         *BuildImageView `json:"image,omitempty"`
	CreatedAt     string          `json:"createdAt"`
	UpdatedAt     string          `json:"updatedAt"`

	deployID string
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
// source builds.
func (s *Service) TriggerBuild(ctx context.Context, id string) (BuildView, error) {
	ctx, carrier, err := s.buildSourceApp(ctx, core.LifecycleOrCreate(false), id)
	if err != nil {
		return BuildView{}, err
	}
	if carrier.Spec.Repo == "" {
		return BuildView{}, fmt.Errorf("%w: only a git build source can be built; build source %s is image-backed", core.ErrBadRequest, id)
	}
	if err := s.RequireBillingMutation(ctx, carrier.Labels[core.LabelTenant]); err != nil {
		return BuildView{}, err
	}
	d, err := s.triggerFetched(ctx, id, carrier, TriggerParams{}, store.TriggerAPI)
	if err != nil {
		return BuildView{}, err
	}
	return buildView(carrier, core.AppPublicID(carrier), d), nil
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
	wrap := func(target string, err error) error {
		log.Printf("deploys: fan out build of %s to %s: %v", b.Carrier, target, err)
		return fmt.Errorf("fan out build of %s to %s: %w", b.Carrier, target, err)
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
		_, err := s.triggerFetched(ctx, core.AppPublicName(&a), &a, TriggerParams{build: build}, store.TriggerNewCommit)
		if err != nil && !errors.Is(err, errAlreadyOnBuild) {
			errs = append(errs, fanoutFailure(wrap(row.ID, err)))
		}
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
		return s.TriggerBuild(r.Context(), r.PathValue("buildSourceId"))
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

var buildGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Build",
	Fields: graphql.Fields{
		"id":            gqlutil.StrField(func(b BuildView) any { return b.ID }),
		"buildSourceId": gqlutil.StrField(func(b BuildView) any { return b.BuildSourceID }),
		"buildRun":      gqlutil.Typed(buildRunGQLType, func(b BuildView) any { return b.BuildRun }),
		"image":         gqlutil.Typed(buildImageGQLType, func(b BuildView) any { return b.Image }),
		"cursor":        gqlutil.StrField(func(b BuildView) any { return b.deployID }),
		"createdAt":     gqlutil.StrField(func(b BuildView) any { return b.CreatedAt }),
		"updatedAt":     gqlutil.StrField(func(b BuildView) any { return b.UpdatedAt }),
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
		"triggerBuild": gqlutil.KeyVerb(buildGQLType, "buildSourceId", s.TriggerBuild),
	}
}

type buildSourceArgs struct {
	BuildSourceID string `json:"buildSourceId" jsonschema:"the build source id (bds-…)"`
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
		Description: "Build a git build source's branch head, even when an up-to-date build exists. When it succeeds, services linked to the source with auto-deploy on deploy the result. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in buildSourceArgs) (*mcp.CallToolResult, BuildView, error) {
		b, err := s.TriggerBuild(ctx, in.BuildSourceID)
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
