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
	"net/http"
	"strings"

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
