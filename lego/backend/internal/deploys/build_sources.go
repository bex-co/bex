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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
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
	k, ok := ids.KindOf(buildID)
	if !ok || k != ids.Build {
		return BuildView{}, core.NotFound("build")
	}
	deployID, _ := ids.Rekind(ids.Deploy, buildID)
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
	v.BuildRun = &BuildRunView{ID: d.ID, Status: store.BuildRunStatus(d.Status), Runtime: core.EffectiveRuntime(carrier.Spec, carrier.Spec.Type), CommitID: d.CommitID,
		BuildStartedAt: formatTimePtr(d.StartedAt), BuildFinishedAt: formatTimePtr(d.FinishedAt)}
	return v
}

// linkedBuild is the build source link a deploy of linked service a ships:
// the link with the source's current build selected. nil for a service with
// no build source.
func (s *Service) linkedBuild(ctx context.Context, a *appv1alpha1.App) (*appv1alpha1.BuildSourceLink, error) {
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
	link := core.BuildSourceLinkTo(&carrier)
	if link.Image == "" {
		return nil, fmt.Errorf("%w: build source %s has not built yet; trigger a build first", core.ErrConflict, core.BuildSourceIDOf(a))
	}
	return link, nil
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

// selectLinkedBuild records image as the build linked service appID deploys,
// in the row the projector owns spec.buildSource from. The write holds only
// while the row still links carrierID, so a deploy racing a re-link can never
// pair one source's build with another source.
func (s *Service) selectLinkedBuild(ctx context.Context, appID, carrierID, image string) error {
	st, err := s.buildSourceStore()
	if err != nil {
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
// caller identity; failures are logged per service, never returned. Every
// service ships b.Image, even if a newer build lands mid fan-out (that build
// fans out itself); a service already on b.Image is left alone.
func (s *Service) BuildSucceeded(ctx context.Context, b store.BuiltSource) {
	fail := func(target string, err error) {
		log.Printf("deploys: fan out build of %s to %s: %v", b.Carrier, target, err)
	}
	if b.Image == "" {
		return
	}
	st, err := s.buildSourceStore()
	if err != nil {
		fail("its services", err)
		return
	}
	links, err := st.ListBuildSourceLinks(ctx, b.CarrierID)
	if err != nil || len(links) == 0 {
		if err != nil {
			fail("its services", err)
		}
		return
	}
	if err := s.RequireBillingMutation(ctx, b.TenantID); err != nil {
		fail("its services", err)
		return
	}
	link := &appv1alpha1.BuildSourceLink{Name: b.Carrier, Image: b.Image}
	for _, row := range links {
		waiting := row.BuildSourceImage == ""
		if row.Suspended || row.BuildSourceImage == b.Image {
			continue
		}
		var a appv1alpha1.App
		if err := s.Client.Get(ctx, client.ObjectKey{Namespace: b.Namespace, Name: store.CRName(row.TenantID, row.Name)}, &a); err != nil {
			fail(row.ID, err)
			continue
		}
		if (!a.Spec.AutoDeploy && !waiting) || a.Spec.Suspended || a.DeletionTimestamp != nil {
			continue
		}
		if _, err := s.triggerFetched(ctx, core.AppPublicName(&a), &a, TriggerParams{buildSource: link}, store.TriggerNewCommit); err != nil {
			fail(row.ID, err)
		}
	}
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
