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

package apps

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render Build Sources (w2/m254, docs/ADR097-build-sources.md). A build source
// is a carrier App the operator builds and never runs. It is created through
// the service create path (so it gets the same validation, clone secret,
// registry credential and release stamping) and edited through the service
// setters, resolved under core.WithBuildSources after the id is checked to
// name a carrier.

// buildSourceCreate is a carrier create's internal half of a CreateRequest.
type buildSourceCreate struct {
	id, name, scopeProjectID string
}

// shape turns the background-worker spec the create path derived into a
// carrier's: it builds, exposes nothing and is labeled with its name.
func (b *buildSourceCreate) shape(spec *appv1alpha1.AppSpec) {
	spec.Type = appv1alpha1.TypeBuildSource
	spec.Expose = false
	spec.DisplayName = b.name
}

func (b *buildSourceCreate) rowID() string {
	if b == nil {
		return ""
	}
	return b.id
}

func (b *buildSourceCreate) displayName() string {
	if b == nil {
		return ""
	}
	return b.name
}

func (b *buildSourceCreate) scope() string {
	if b == nil {
		return ""
	}
	return b.scopeProjectID
}

// BuildSourceGit is Render's git-backed build source configuration.
type BuildSourceGit struct {
	RepoURL              string           `json:"repoUrl,omitempty"`
	Branch               string           `json:"branch,omitempty"`
	BuildCommand         string           `json:"buildCommand,omitempty"`
	BuildFilter          *BuildFilterView `json:"buildFilter,omitempty"`
	DockerContext        string           `json:"dockerContext,omitempty"`
	DockerfilePath       string           `json:"dockerfilePath,omitempty"`
	Runtime              string           `json:"runtime,omitempty"`
	RegistryCredentialID *string          `json:"registryCredentialId,omitempty"`
	Region               string           `json:"region,omitempty"`
	RootDir              string           `json:"rootDir,omitempty"`
}

// BuildSourceImage is Render's image-backed build source configuration.
type BuildSourceImage struct {
	OwnerID              string  `json:"ownerId,omitempty"`
	ImageURL             string  `json:"imageUrl,omitempty"`
	RegistryCredentialID *string `json:"registryCredentialId,omitempty"`
}

// BuildSourceServiceLink names a service linked to a build source.
type BuildSourceServiceLink struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// BuildSourceView is Render's build source object.
type BuildSourceView struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	OwnerID      string                   `json:"ownerId"`
	ProjectID    string                   `json:"projectId,omitempty"`
	Git          *BuildSourceGit          `json:"git,omitempty"`
	Image        *BuildSourceImage        `json:"image,omitempty"`
	CreatedAt    string                   `json:"createdAt"`
	UpdatedAt    string                   `json:"updatedAt"`
	ServiceLinks []BuildSourceServiceLink `json:"serviceLinks"`
}

// CreateBuildSourceRequest is Render's create-build-source body.
type CreateBuildSourceRequest struct {
	OwnerID   string            `json:"ownerId"`
	Name      string            `json:"name"`
	ProjectID string            `json:"projectId,omitempty"`
	Git       *BuildSourceGit   `json:"git,omitempty"`
	Image     *BuildSourceImage `json:"image,omitempty"`
	// EnvVars and SecretFiles seed the build-time environment, as a service
	// create seeds its own (w2/m254 t004).
	EnvVars     []envVarInput     `json:"envVars,omitempty"`
	SecretFiles []secretFileInput `json:"secretFiles,omitempty"`
	// ServiceIDs are linked to the new build source, and EnvGroupIDs linked
	// into its build-time environment (git sources only), as Render's create
	// does (w9/m180 t001). Every link is checked before anything is written.
	ServiceIDs  []string `json:"serviceIds,omitempty" jsonschema:"service ids (srv-…) to link to the new build source; each is checked as a PATCH buildSourceId is, and none deploys until the deploy API is called"`
	EnvGroupIDs []string `json:"envGroupIds,omitempty" jsonschema:"environment group ids (evg-…) to link into the build-time environment; git sources only, and not groups scoped to an environment"`
}

// UpdateBuildSourceRequest is Render's update-build-source body: each
// top-level field is a true patch.
type UpdateBuildSourceRequest struct {
	Name  *string           `json:"name,omitempty"`
	Git   *BuildSourceGit   `json:"git,omitempty"`
	Image *BuildSourceImage `json:"image,omitempty"`
}

// buildSourceRuntimes is Render's git build source runtime enum.
var buildSourceRuntimes = []string{"docker", "elixir", "go", "node", "python", "ruby", "rust"}

// BuildSourceLimitCode refuses a build source past the workspace's ceiling.
const BuildSourceLimitCode = "BUILD_SOURCE_LIMIT"

// BuildSourceNameTakenCode refuses a name another build source in the
// workspace has: Render's Blueprint schema makes it unique, and a sync finds a
// carrier by it (ADR097 Blueprints §1, w9/m180 t004). A 409 conflict, as an
// env group's ENV_GROUP_NAME_EXISTS is; Render's create and update document
// no code for it.
const BuildSourceNameTakenCode = "BUILD_SOURCE_NAME_TAKEN"

// BuildSourcePartiallyLinkedCode reports a create that wrote its build source
// and linked some of its serviceIds before a later link failed.
const BuildSourcePartiallyLinkedCode = "BUILD_SOURCE_PARTIALLY_LINKED"

func errBuildSourceNameTaken(name string) error {
	return core.NewConflictError(BuildSourceNameTakenCode,
		fmt.Sprintf("a build source named %q already exists in this workspace", name), map[string]any{"name": name})
}

// checkBuildSourceName refuses name when another build source than selfID in
// the acting workspace has it, so a rename is refused before its CR patch. A
// create relies on the store's locked check (store.ErrBuildSourceNameTaken),
// which refuses before the row is written.
func (s *Service) checkBuildSourceName(ctx context.Context, name, selfID string) error {
	tenantID, _ := s.Tenant(ctx)
	var list appv1alpha1.AppList
	if err := s.ListByTenant(ctx, &list, tenantID); err != nil {
		return err
	}
	for i := range list.Items {
		a := &list.Items[i]
		if core.IsBuildSource(a) && core.NotFoundIfDeleting(a) == nil && a.Name != selfID && buildSourceName(a) == name {
			return errBuildSourceNameTaken(name)
		}
	}
	return nil
}

// CreateBuildSource is Render's create: a carrier built from git or named by
// an image. It builds once it exists, as a new service does.
func (s *Service) CreateBuildSource(ctx context.Context, req CreateBuildSourceRequest) (BuildSourceView, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return BuildSourceView{}, err
	}
	req.Name = strings.TrimSpace(req.Name)
	switch {
	case req.Name == "":
		return BuildSourceView{}, fmt.Errorf("%w: name is required", core.ErrBadRequest)
	case (req.Git == nil) == (req.Image == nil):
		return BuildSourceView{}, fmt.Errorf("%w: exactly one of git or image is required", core.ErrBadRequest)
	case req.Image != nil && (len(req.EnvVars) > 0 || len(req.SecretFiles) > 0 || len(req.EnvGroupIDs) > 0):
		// Render: the build-time environment is only valid for git sources;
		// the request fails if any is provided with image.
		return BuildSourceView{}, fmt.Errorf("%w: envVars, secretFiles and envGroupIds are only valid for git build sources", core.ErrBadRequest)
	}
	env, err := toEnvVars(req.EnvVars)
	if err != nil {
		return BuildSourceView{}, err
	}
	create := CreateRequest{OwnerID: req.OwnerID, Type: appv1alpha1.TypeBackgroundWorker, Env: env}
	for _, f := range req.SecretFiles {
		create.SecretFiles = append(create.SecretFiles, core.SecretFile{Name: f.Name, Content: f.Content})
	}
	if g := req.Git; g != nil {
		if !slices.Contains(buildSourceRuntimes, g.Runtime) {
			return BuildSourceView{}, fmt.Errorf("%w: git.runtime must be one of %s", core.ErrBadRequest, strings.Join(buildSourceRuntimes, "|"))
		}
		if strings.TrimSpace(g.RepoURL) == "" {
			return BuildSourceView{}, fmt.Errorf("%w: git.repoUrl is required", core.ErrBadRequest)
		}
		if g.DockerContext != "" && g.Runtime != "docker" {
			return BuildSourceView{}, fmt.Errorf("%w: git.dockerContext is only valid for the docker runtime", core.ErrBadRequest)
		}
		create.Repo, create.Branch, create.Runtime, create.BuildCommand = g.RepoURL, g.Branch, g.Runtime, g.BuildCommand
		create.DockerfilePath, create.DockerContext, create.RootDir = g.DockerfilePath, g.DockerContext, g.RootDir
		create.BuildFilter, create.RegistryCredentialID, create.Region = g.BuildFilter, g.RegistryCredentialID, g.Region
	} else {
		if strings.TrimSpace(req.Image.ImageURL) == "" {
			return BuildSourceView{}, fmt.Errorf("%w: image.imageUrl is required", core.ErrBadRequest)
		}
		if req.Image.OwnerID != "" && req.OwnerID != "" && req.Image.OwnerID != req.OwnerID {
			return BuildSourceView{}, fmt.Errorf("%w: image.ownerId must be the build source's ownerId", core.ErrBadRequest)
		}
		create.Image, create.RegistryCredentialID = req.Image.ImageURL, req.Image.RegistryCredentialID
	}
	native := req.Git != nil && blueprintNativeRuntime(req.Git.Runtime)
	links, err := s.checkBuildSourceCreateLinks(ctx, req.ServiceIDs, native)
	if err != nil {
		return BuildSourceView{}, err
	}
	groups, err := s.buildSourceCreateGroups(ctx, req.EnvGroupIDs)
	if err != nil {
		return BuildSourceView{}, err
	}
	id, err := s.createBuildSourceCarrier(ctx, create, req.Name, req.ProjectID)
	if err != nil {
		return BuildSourceView{}, err
	}
	// The group links only touch the carrier: a refusal there (an env group
	// scoped to an environment, which Render refuses) deletes it again, so the
	// create writes nothing. The service links were checked above.
	for _, g := range groups {
		if err := s.EnvGroups.LinkEnvGroup(core.WithBuildSources(ctx), g, id); err != nil {
			if undo := s.DeleteBuildSource(ctx, id); undo != nil {
				log.Printf("apps: delete build source %s after a refused env group link: %v", id, undo)
			}
			return BuildSourceView{}, fmt.Errorf("linking env group %q: %w", g, err)
		}
	}
	// Every service link was checked above, so a failure here is transient.
	// Before any link lands, the carrier is deleted again and a retry starts
	// clean; a link rewrites the service's own source, which no undo can
	// restore, so after one the source stays and the error names what linked.
	for i, service := range links {
		if _, err := s.LinkBuildSource(ctx, service, id, nil, false); err != nil {
			if i == 0 {
				if undo := s.DeleteBuildSource(ctx, id); undo != nil {
					log.Printf("apps: delete build source %s after a failed service link: %v", id, undo)
				}
				return BuildSourceView{}, fmt.Errorf("linking service %s: %w", service, err)
			}
			return BuildSourceView{}, core.NewConflictError(BuildSourcePartiallyLinkedCode,
				fmt.Sprintf("build source %s was created and linked to %s; linking service %s failed: %v", id, strings.Join(links[:i], ", "), service, err),
				map[string]any{"buildSourceId": id, "linkedServiceIds": links[:i], "failedServiceId": service})
		}
	}
	return s.GetBuildSource(ctx, id)
}

// checkBuildSourceCreateLinks checks each service a create links before the
// carrier exists, so a refused link writes nothing: linkableService's checks,
// plus Render's start-command rule for a source that is native or not.
func (s *Service) checkBuildSourceCreateLinks(ctx context.Context, serviceIDs []string, native bool) ([]string, error) {
	seen := map[string]bool{}
	for _, id := range serviceIDs {
		if seen[id] {
			return nil, fmt.Errorf("%w: serviceIds lists %s twice", core.ErrBadRequest, id)
		}
		seen[id] = true
		a, err := s.linkableService(ctx, id)
		if err != nil {
			return nil, err
		}
		existing := linkStartCommand(a.Spec.Type, a.Spec.StartCommand, a.Spec.Command)
		if _, err := buildSourceLinkCommand(native, nil, existing, "being created"); err != nil {
			return nil, fmt.Errorf("service %s: %w", id, err)
		}
	}
	return serviceIDs, nil
}

// linkableService authorizes linking service name to a build source and
// checks what every link requires of it: it is not in a protected
// environment (linking repoints what it runs, the rule a repo or image change
// follows, w4/m126, w8/187) and its type builds from a build source.
func (s *Service) linkableService(ctx context.Context, name string) (*appv1alpha1.App, error) {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, name)
	if err != nil {
		return nil, err
	}
	if err := s.requireUnprotected(ctx, a, "repoint"); err != nil {
		return nil, err
	}
	if err := linkableType(a.Spec.Type); err != nil {
		return nil, err
	}
	return a, nil
}

func linkableType(svcType string) error {
	if !slices.Contains(buildSourceLinkTypes, svcType) {
		return fmt.Errorf("%w: a %s cannot build from a build source", core.ErrBadRequest, svcType)
	}
	return nil
}

// buildSourceCreateGroups resolves a create's envGroupIds to the names the
// link writes by, refusing an id that names no env group in the workspace
// before anything is written.
func (s *Service) buildSourceCreateGroups(ctx context.Context, groupIDs []string) ([]string, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	if s.EnvGroups == nil {
		return nil, fmt.Errorf("%w: env groups need the secret store", core.ErrUnavailable)
	}
	byName, err := s.EnvGroups.GroupIDsByName(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(byName))
	for name, id := range byName {
		names[id] = name
	}
	var out []string
	for _, id := range groupIDs {
		name, ok := names[id]
		if !ok {
			return nil, fmt.Errorf("%w: envGroupIds: env group %s is not in this workspace", core.ErrBadRequest, id)
		}
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out, nil
}

// createBuildSourceCarrier creates the carrier of build source name, scoped to
// projectID, from create's source and build fields, and returns its id.
func (s *Service) createBuildSourceCarrier(ctx context.Context, create CreateRequest, name, projectID string) (string, error) {
	id := ids.New(ids.BuildSource)
	create.Name = id
	create.buildSource = &buildSourceCreate{id: id, name: name, scopeProjectID: projectID}
	if err := s.Metadata.CheckRequestedRegion(create.Region); err != nil {
		return "", err
	}
	if err := s.checkBuildSourceProject(ctx, projectID); err != nil {
		return "", err
	}
	if _, err := s.create(ctx, create); err != nil {
		if errors.Is(err, store.ErrBuildSourceNameTaken) {
			return "", errBuildSourceNameTaken(name)
		}
		return "", err
	}
	return id, nil
}

// checkBuildSourceProject refuses a project scope outside the workspace.
func (s *Service) checkBuildSourceProject(ctx context.Context, projectID string) error {
	if projectID == "" {
		return nil
	}
	getter, ok := s.Store.(interface {
		GetProject(ctx context.Context, id string) (store.Project, error)
	})
	if !ok {
		return fmt.Errorf("%w: projectId is not supported without the control plane", core.ErrBadRequest)
	}
	tenantID, _ := s.Tenant(ctx)
	if p, err := getter.GetProject(ctx, projectID); err != nil || p.TenantID != tenantID {
		return fmt.Errorf("%w: project %s is not in this workspace", core.ErrBadRequest, projectID)
	}
	return nil
}

// checkBuildSourceCeiling refuses a build source past the plan's ceiling.
func (s *Service) checkBuildSourceCeiling(ctx context.Context, tenantID string) error {
	counter, ok := s.Store.(interface {
		CountBuildSourcesForTenant(context.Context, string) (int, error)
		GetTenant(context.Context, string) (store.Tenant, error)
	})
	if !ok || tenantID == "" {
		return nil
	}
	tenant, err := counter.GetTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	n, err := counter.CountBuildSourcesForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	if limit := store.QuotaCapsForPlan(tenant.Plan).BuildSources; int64(n) >= limit {
		return core.NewBadRequestError(BuildSourceLimitCode,
			fmt.Sprintf("this workspace already has %d build sources, the limit for its plan", limit), map[string]any{"limit": limit})
	}
	return nil
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

// GetBuildSource returns one build source.
func (s *Service) GetBuildSource(ctx context.Context, id string) (BuildSourceView, error) {
	ctx, a, err := s.buildSourceApp(ctx, core.RelCanView, id)
	if err != nil {
		return BuildSourceView{}, err
	}
	return s.buildSourceView(ctx, a)
}

// parseBuildSourceFilter reads Render's list-build-sources filters, get
// returning each by its Render name (projectId, createdBefore, …).
func parseBuildSourceFilter(names []string, get func(string) string) (BuildSourceFilter, error) {
	f := BuildSourceFilter{Names: names, ProjectID: get("projectId")}
	for field, dst := range map[string]*time.Time{
		"createdBefore": &f.CreatedBefore, "createdAfter": &f.CreatedAfter,
		"updatedBefore": &f.UpdatedBefore, "updatedAfter": &f.UpdatedAfter,
	} {
		t, err := core.ParseTime(field, get(field))
		if err != nil {
			return BuildSourceFilter{}, err
		}
		*dst = t
	}
	return f, nil
}

// listBuildSourcesPage is one page of ListBuildSources, after the build
// source id after.
func (s *Service) listBuildSourcesPage(ctx context.Context, ownerID string, f BuildSourceFilter, after string, limit int) ([]BuildSourceView, error) {
	views, err := s.ListBuildSources(ctx, ownerID, f)
	if err != nil {
		return nil, err
	}
	return core.StablePage(views, after, limit, true, func(v BuildSourceView) string { return v.ID }), nil
}

func toBuildSourceList(views []BuildSourceView) []buildSourceWithCursor {
	out := make([]buildSourceWithCursor, len(views))
	for i, v := range views {
		out[i] = buildSourceWithCursor{BuildSource: v, Cursor: v.ID}
	}
	return out
}

// BuildSourceFilter is Render's list-build-sources query.
type BuildSourceFilter struct {
	Names                                                    []string
	ProjectID                                                string
	CreatedBefore, CreatedAfter, UpdatedBefore, UpdatedAfter time.Time
}

// ListBuildSources lists the workspace's build sources matching f.
func (s *Service) ListBuildSources(ctx context.Context, ownerID string, f BuildSourceFilter) ([]BuildSourceView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	tenantID, _ := s.Tenant(ctx)
	var list appv1alpha1.AppList
	if err := s.ListByTenant(ctx, &list, tenantID); err != nil {
		return nil, err
	}
	var views []BuildSourceView
	for i := range list.Items {
		a := &list.Items[i]
		if !core.IsBuildSource(a) || core.NotFoundIfDeleting(a) != nil {
			continue
		}
		v, err := s.buildSourceView(ctx, a)
		if err != nil {
			return nil, err
		}
		if buildSourceMatches(v, f) {
			views = append(views, v)
		}
	}
	return views, nil
}

func buildSourceMatches(v BuildSourceView, f BuildSourceFilter) bool {
	if len(f.Names) > 0 && !slices.Contains(f.Names, v.Name) {
		return false
	}
	if f.ProjectID != "" && v.ProjectID != f.ProjectID {
		return false
	}
	created, _ := time.Parse(time.RFC3339, v.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, v.UpdatedAt)
	switch {
	case !f.CreatedBefore.IsZero() && !created.Before(f.CreatedBefore),
		!f.CreatedAfter.IsZero() && !created.After(f.CreatedAfter),
		!f.UpdatedBefore.IsZero() && !updated.Before(f.UpdatedBefore),
		!f.UpdatedAfter.IsZero() && !updated.After(f.UpdatedAfter):
		return false
	}
	return true
}

// buildSourceView renders a carrier as Render's build source object.
func (s *Service) buildSourceView(ctx context.Context, a *appv1alpha1.App) (BuildSourceView, error) {
	v := s.view(a)
	out := BuildSourceView{
		ID: v.ID, Name: buildSourceName(a), OwnerID: v.OwnerID,
		CreatedAt: v.CreatedAt, UpdatedAt: cmp.Or(v.UpdatedAt, v.CreatedAt),
		ServiceLinks: []BuildSourceServiceLink{},
	}
	if a.Spec.Image != "" {
		out.Image = &BuildSourceImage{OwnerID: v.OwnerID, ImageURL: a.Spec.Image, RegistryCredentialID: v.RegistryCredentialID}
	} else {
		out.Git = &BuildSourceGit{RepoURL: v.Repo, Branch: v.Branch, BuildCommand: v.BuildCommand, BuildFilter: v.BuildFilter,
			DockerContext: v.DockerContext, DockerfilePath: v.DockerfilePath, Runtime: v.Runtime,
			RegistryCredentialID: v.RegistryCredentialID, Region: v.Region, RootDir: v.RootDir}
	}
	lister, ok := s.Store.(interface {
		ListBuildSourceLinks(context.Context, string) ([]store.App, error)
		GetApp(context.Context, string) (store.App, error)
	})
	if s.Store == nil || !ok {
		return out, nil
	}
	if row, err := lister.GetApp(ctx, out.ID); err == nil {
		out.ProjectID = row.ScopeProjectID
	}
	links, err := lister.ListBuildSourceLinks(ctx, out.ID)
	if err != nil {
		return BuildSourceView{}, err
	}
	for _, l := range links {
		out.ServiceLinks = append(out.ServiceLinks, BuildSourceServiceLink{ID: l.ID, Name: cmp.Or(l.DisplayName, l.Name)})
	}
	return out, nil
}

// UpdateBuildSource is Render's update: each top-level field is a true patch,
// saved without building (a build is triggered separately). Every field is
// validated before anything is written, and the edit lands as one untracked
// patch marked source-pending, so the operator keeps the current build until
// a build is triggered — no deploy row, no build.
func (s *Service) UpdateBuildSource(ctx context.Context, id string, req UpdateBuildSourceRequest) (BuildSourceView, error) {
	if req.Git != nil && req.Image != nil {
		return BuildSourceView{}, fmt.Errorf("%w: git and image are mutually exclusive", core.ErrBadRequest)
	}
	ctx, a, err := s.buildSourceApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, id)
	if err != nil {
		return BuildSourceView{}, err
	}
	var name string
	if req.Name != nil {
		if name = strings.TrimSpace(*req.Name); name == "" {
			return BuildSourceView{}, fmt.Errorf("%w: name cannot be empty", core.ErrBadRequest)
		}
		if name != buildSourceName(a) {
			if err := s.checkBuildSourceName(ctx, name, a.Name); err != nil {
				return BuildSourceView{}, err
			}
		}
	}
	patch, build, err := buildSourcePatch(a, req)
	if err != nil {
		return BuildSourceView{}, err
	}
	if req.Git != nil && req.Git.Region != "" {
		if err := s.Metadata.CheckRequestedRegion(req.Git.Region); err != nil {
			return BuildSourceView{}, err
		}
	}
	next, sourceChanged, err := s.checkSourcePatch(ctx, a, patch)
	if err != nil {
		return BuildSourceView{}, err
	}
	probe := a.DeepCopy()
	if sourceChanged {
		next.applySourceChange(probe)
	}
	if err := build.apply(probe); err != nil {
		return BuildSourceView{}, err
	}
	if !sourceChanged && probe.Spec.DisplayName == a.Spec.DisplayName && name == "" &&
		equalBuildSettings(probe.Spec, a.Spec) {
		return s.buildSourceView(ctx, a)
	}
	if err := s.markSourcePending(ctx, a); err != nil {
		return BuildSourceView{}, err
	}
	if s.Store != nil {
		if sourceChanged {
			if err := s.Store.SetAppSource(ctx, id, next.repo, next.image, next.branch, clonePtr(next.registryCredentialID)); err != nil {
				return BuildSourceView{}, fmt.Errorf("update source of truth: %w", err)
			}
		}
		if name != "" {
			if err := s.Store.SetAppDisplayName(ctx, id, name); err != nil {
				if errors.Is(err, store.ErrBuildSourceNameTaken) {
					return BuildSourceView{}, errBuildSourceNameTaken(name)
				}
				return BuildSourceView{}, err
			}
		}
	}
	oldCloneSecret, repoChanged := a.Spec.CloneSecret, sourceChanged && next.repo != a.Spec.Repo
	if _, err := s.patchUntracked(ctx, a, func(app *appv1alpha1.App) {
		if sourceChanged {
			next.applySourceChange(app)
		}
		_ = build.apply(app) // validated on the probe above
		if name != "" {
			app.Spec.DisplayName = name
		}
	}); err != nil {
		return BuildSourceView{}, err
	}
	if repoChanged && oldCloneSecret != "" {
		if err := s.deleteCloneSecret(ctx, a.Namespace, a.Name); err != nil {
			log.Printf("apps: clear stale clone secret for build source %s: %v", id, err)
		}
	}
	s.RecordAppConfigChanged(ctx, a, core.AuditVerbSetSourceAndRegistryCredential)
	return s.GetBuildSource(ctx, id)
}

// buildSettings are the git build fields a build source patch may change.
type buildSettings struct {
	runtime, buildCommand, dockerfilePath, dockerContext, rootDir *string
	buildFilter                                                   *BuildFilterView
}

// apply writes the settings onto a's spec, resolving a runtime to its builder
// as create does.
func (b buildSettings) apply(a *appv1alpha1.App) error {
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = *v
		}
	}
	set(&a.Spec.BuildCommand, b.buildCommand)
	set(&a.Spec.DockerfilePath, b.dockerfilePath)
	set(&a.Spec.DockerContext, b.dockerContext)
	if b.rootDir != nil {
		a.Spec.RootDir = store.NormalizeRootDir(*b.rootDir)
	}
	if b.buildFilter != nil {
		filter, err := normalizeBuildFilter(b.buildFilter)
		if err != nil {
			return err
		}
		a.Spec.BuildFilter = filter
	}
	if b.runtime != nil {
		runtime, builder, err := resolveBuildStrategy(appv1alpha1.TypeBackgroundWorker, CreateRequest{
			Runtime: *b.runtime, Repo: a.Spec.Repo, Image: a.Spec.Image, BuildCommand: a.Spec.BuildCommand,
			buildSource: &buildSourceCreate{},
		})
		if err != nil {
			return err
		}
		a.Spec.Runtime, a.Spec.Builder = runtime, builder
	}
	if a.Spec.DockerContext != "" && a.Spec.Runtime != "docker" {
		return fmt.Errorf("%w: git.dockerContext is only valid for the docker runtime", core.ErrBadRequest)
	}
	return nil
}

func equalBuildSettings(a, b appv1alpha1.AppSpec) bool {
	return a.Runtime == b.Runtime && a.Builder == b.Builder && a.BuildCommand == b.BuildCommand &&
		a.DockerfilePath == b.DockerfilePath && a.DockerContext == b.DockerContext && a.RootDir == b.RootDir &&
		reflect.DeepEqual(a.BuildFilter, b.BuildFilter)
}

// buildSourcePatch splits a patch into its source change (repo/image/branch/
// credential, switching backing when git or image is supplied) and its build
// settings, refusing what Render refuses.
func buildSourcePatch(a *appv1alpha1.App, req UpdateBuildSourceRequest) (sourcePatch, buildSettings, error) {
	var patch sourcePatch
	var build buildSettings
	opt := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	imageBacked := a.Spec.Image != ""
	if g := req.Git; g != nil {
		if g.Runtime != "" && !slices.Contains(buildSourceRuntimes, g.Runtime) {
			return patch, build, fmt.Errorf("%w: git.runtime must be one of %s", core.ErrBadRequest, strings.Join(buildSourceRuntimes, "|"))
		}
		if g.Region != "" && !imageBacked {
			return patch, build, fmt.Errorf("%w: git.region can only be set when switching an image build source to git", core.ErrBadRequest)
		}
		if g.RepoURL == "" && imageBacked {
			return patch, build, fmt.Errorf("%w: git.repoUrl is required to switch an image build source to git", core.ErrBadRequest)
		}
		patch.Repo, patch.Branch, patch.RegistryCredentialID = opt(g.RepoURL), opt(g.Branch), g.RegistryCredentialID
		if imageBacked {
			empty := ""
			patch.Image = &empty
		}
		build = buildSettings{runtime: opt(g.Runtime), buildCommand: opt(g.BuildCommand), dockerfilePath: opt(g.DockerfilePath),
			dockerContext: opt(g.DockerContext), rootDir: opt(g.RootDir), buildFilter: g.BuildFilter}
	}
	if img := req.Image; img != nil {
		if img.ImageURL == "" && !imageBacked {
			return patch, build, fmt.Errorf("%w: image.imageUrl is required to switch a git build source to an image", core.ErrBadRequest)
		}
		patch.Image, patch.RegistryCredentialID = opt(img.ImageURL), img.RegistryCredentialID
		if !imageBacked {
			empty := ""
			patch.Repo, patch.Branch = &empty, &empty
		}
	}
	return patch, build, nil
}

// DeleteBuildSource deletes a build source; one that services still link is
// a conflict (unlink them first).
func (s *Service) DeleteBuildSource(ctx context.Context, id string) error {
	// The service delete's relation (Delete), re-checked by it below.
	ctx, _, err := s.buildSourceApp(ctx, core.RelCanCreate, id)
	if err != nil {
		return err
	}
	if lister, ok := s.Store.(interface {
		ListBuildSourceLinks(context.Context, string) ([]store.App, error)
	}); ok {
		links, err := lister.ListBuildSourceLinks(ctx, id)
		if err != nil {
			return err
		}
		if len(links) > 0 {
			return core.NewConflictError("BUILD_SOURCE_LINKED",
				fmt.Sprintf("build source %s is linked to %d service(s); unlink them before deleting it", id, len(links)),
				map[string]any{"serviceCount": len(links)})
		}
	}
	if err := s.Delete(ctx, id); err != nil {
		return err
	}
	if s.EnsureNamespaces != nil {
		if tenantID, _ := s.Tenant(ctx); tenantID != "" {
			// Give the carrier's App quota room back (roomForBuildSources).
			_ = s.EnsureNamespaces(ctx, tenantID)
		}
	}
	return nil
}

// --- REST: Render's /v1/build-sources (w2/m254 t003) ---

type buildSourceWithCursor struct {
	BuildSource BuildSourceView `json:"buildSource"`
	Cursor      string          `json:"cursor"`
}

func (s *Service) registerBuildSourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/build-sources", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		var req CreateBuildSourceRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			return nil, err
		}
		return s.CreateBuildSource(r.Context(), req)
	}))
	mux.HandleFunc("GET /v1/build-sources", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		f, err := parseBuildSourceFilter(q["name"], q.Get)
		if err != nil {
			return nil, err
		}
		after, limit := core.PageParams(q)
		page, err := s.listBuildSourcesPage(r.Context(), q.Get("ownerId"), f, after, limit)
		if err != nil {
			return nil, err
		}
		return toBuildSourceList(page), nil
	}))
	mux.HandleFunc("GET /v1/build-sources/{buildSourceId}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetBuildSource(r.Context(), r.PathValue("buildSourceId"))
	}))
	mux.HandleFunc("PATCH /v1/build-sources/{buildSourceId}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		var req UpdateBuildSourceRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			return nil, err
		}
		return s.UpdateBuildSource(r.Context(), r.PathValue("buildSourceId"), req)
	}))
	mux.HandleFunc("DELETE /v1/build-sources/{buildSourceId}", core.HandleJSON(http.StatusNoContent, func(r *http.Request) (any, error) {
		return nil, s.DeleteBuildSource(r.Context(), r.PathValue("buildSourceId"))
	}))
}

// --- Linking a service to a build source (w2/m254 t005) ---

// buildSourceLinkOmitted are the create fields a build source provides, which
// Render says to omit when buildSourceId is set.
var buildSourceLinkOmitted = []struct {
	name string
	set  func(CreateRequest) bool
}{
	{"repo", func(r CreateRequest) bool { return r.Repo != "" }},
	{"branch", func(r CreateRequest) bool { return r.Branch != "" }},
	{"image", func(r CreateRequest) bool { return r.Image != "" }},
	{"buildFilter", func(r CreateRequest) bool { return r.BuildFilter != nil }},
	{"runtime", func(r CreateRequest) bool { return r.Runtime != "" }},
	{"buildCommand", func(r CreateRequest) bool { return r.BuildCommand != "" }},
	{"dockerfilePath", func(r CreateRequest) bool { return r.DockerfilePath != "" }},
	{"dockerContext", func(r CreateRequest) bool { return r.DockerContext != "" }},
	{"rootDir", func(r CreateRequest) bool { return r.RootDir != "" }},
	{"registryCredentialId", func(r CreateRequest) bool { return r.RegistryCredentialID != nil }},
}

// validateBuildSourceLinkCreate refuses the source and build fields a linked
// create must omit, by name.
func validateBuildSourceLinkCreate(req CreateRequest) error {
	for _, f := range buildSourceLinkOmitted {
		if f.set(req) {
			return fmt.Errorf("%w: %s cannot be set with buildSourceId; the build source provides it", core.ErrBadRequest, f.name)
		}
	}
	return nil
}

// buildSourceLinkTypes are the service types Render links to a build source.
var buildSourceLinkTypes = []string{"", appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService,
	appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob}

// buildSourceLink resolves a svcType service's link to build source id in
// tenantID: the carrier, the build the service deploys (its current one, or
// none until it has built) and the service's start command. start nil keeps
// existing for a native source and drops it for a Docker or image one, which
// takes none; a native source requires one (Render).
//
// It also returns the carrier, whose source fields the image rules read for
// the linked service (sourceForImageRules).
func (s *Service) buildSourceLink(ctx context.Context, tenantID, id, svcType string, start *string, existing string) (*appv1alpha1.BuildSourceLink, string, *appv1alpha1.App, error) {
	if err := linkableType(svcType); err != nil {
		return nil, "", nil, err
	}
	_, carrier, err := s.buildSourceApp(ctx, core.RelCanView, id)
	if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrForbidden) {
		return nil, "", nil, fmt.Errorf("%w: buildSourceId %s names no build source in this workspace", core.ErrBadRequest, id)
	}
	if err != nil {
		return nil, "", nil, err
	}
	if tenantID != "" && carrier.Labels[core.LabelTenant] != tenantID {
		return nil, "", nil, fmt.Errorf("%w: buildSourceId %s names no build source in this workspace", core.ErrBadRequest, id)
	}
	command, err := buildSourceLinkCommand(buildSourceNative(carrier.Spec), start, existing, id)
	if err != nil {
		return nil, "", nil, err
	}
	return core.BuildSourceLinkTo(carrier), command, carrier, nil
}

// buildSourceNative reports whether a carrier builds with a native runtime,
// which needs each linked service's start command (Render).
func buildSourceNative(carrier appv1alpha1.AppSpec) bool {
	return carrier.Image == "" && blueprintNativeRuntime(carrier.Runtime)
}

// buildSourceLinkCommand is the start command a service linked to build
// source ref runs: start when given, else existing for a native source and
// none for a Docker or image one, checked against Render's start-command rule.
func buildSourceLinkCommand(native bool, start *string, existing, ref string) (string, error) {
	command := ""
	switch {
	case start != nil:
		command = strings.TrimSpace(*start)
	case native:
		command = existing
	}
	if err := buildSourceStartRule(native, command, ref); err != nil {
		return "", err
	}
	return command, nil
}

// sourceForImageRules is the spec whose source fields (repo, image, runtime,
// builder) decide a service's image port mode and container policy: the
// carrier's for a service linked to a build source, whose own spec has them
// cleared (linkBuildSourceSpec), and the service's own otherwise (w9/m180
// t003).
func sourceForImageRules(spec appv1alpha1.AppSpec, carrier *appv1alpha1.App) appv1alpha1.AppSpec {
	if carrier == nil {
		return spec
	}
	source := spec
	source.Repo, source.Image = carrier.Spec.Repo, carrier.Spec.Image
	source.Runtime, source.Builder = carrier.Spec.Runtime, carrier.Spec.Builder
	return source
}

// buildSourceStartRule is Render's start-command rule for a service linked to
// build source ref: a native source needs the service's start command, a
// Docker or image one takes none.
func buildSourceStartRule(native bool, command, ref string) error {
	switch {
	case native && command == "":
		return fmt.Errorf("%w: build source %s builds with a native runtime, so the service needs a startCommand", core.ErrBadRequest, ref)
	case !native && command != "":
		return fmt.Errorf("%w: build source %s uses a Docker or image runtime, which takes no startCommand", core.ErrBadRequest, ref)
	}
	return nil
}

// linkStartCommand is the command a service linked to a build source runs: a
// cron job's command, any other service's start command.
func linkStartCommand(svcType, startCommand, command string) string {
	if svcType == appv1alpha1.TypeCronJob {
		return command
	}
	return startCommand
}

// buildSourceLinker is the store's link write, absent without the control
// plane.
func (s *Service) buildSourceLinker() (interface {
	SetAppBuildSource(ctx context.Context, id, carrierID, image string) error
}, bool) {
	linker, ok := s.Store.(interface {
		SetAppBuildSource(ctx context.Context, id, carrierID, image string) error
	})
	return linker, ok
}

// linkBuildSourceSpec links spec to a build source: it drops the source and
// build configuration the build source now provides.
func linkBuildSourceSpec(spec *appv1alpha1.AppSpec, link appv1alpha1.BuildSourceLink) {
	spec.Repo, spec.Image, spec.Branch, spec.BuildCommit = "", "", "", ""
	spec.CloneSecret, spec.ExternalRegistryPullSecret, spec.RegistryCredentialID = "", "", nil
	spec.Runtime, spec.Builder, spec.BuildCommand, spec.BuildFilter = "", "", "", nil
	spec.DockerfilePath, spec.DockerContext, spec.RootDir = "", "", ""
	spec.BuildSource = &link
}

func buildSourceLinkImage(link *appv1alpha1.BuildSourceLink) string {
	if link == nil {
		return ""
	}
	return link.Image
}

// LinkBuildSource is Render's PATCH buildSourceId: the service now builds and
// deploys from build source buildSourceID, which provides all its source and
// build configuration. startCommand (nil: Render's default — see
// buildSourceLink) is the only other field Render accepts with it. Like Render,
// the link is saved without deploying: a deploy picks it up. dryRun runs every
// check and writes nothing.
func (s *Service) LinkBuildSource(ctx context.Context, name, buildSourceID string, startCommand *string, dryRun bool) (AppView, error) {
	a, err := s.linkableService(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	cron := a.Spec.Type == appv1alpha1.TypeCronJob
	existing := linkStartCommand(a.Spec.Type, a.Spec.StartCommand, a.Spec.Command)
	link, start, _, err := s.buildSourceLink(ctx, a.Labels[core.LabelTenant], buildSourceID, a.Spec.Type, startCommand, existing)
	if err != nil {
		return AppView{}, err
	}
	linker, ok := s.buildSourceLinker()
	id := managedAppID(a)
	if id != "" && !ok {
		return AppView{}, fmt.Errorf("%w: build sources need the control plane", core.ErrUnavailable)
	}
	if dryRun || (a.Spec.BuildSource != nil && *a.Spec.BuildSource == *link && existing == start) {
		return s.view(a), nil
	}
	if err := s.markSourcePending(ctx, a); err != nil {
		return AppView{}, err
	}
	if id != "" {
		if err := linker.SetAppBuildSource(ctx, id, buildSourceID, link.Image); err != nil {
			return AppView{}, fmt.Errorf("update source of truth: %w", err)
		}
	}
	oldCloneSecret := a.Spec.CloneSecret
	updated, err := s.patchUntracked(ctx, a, func(app *appv1alpha1.App) {
		spec := &app.Spec
		linkBuildSourceSpec(spec, *link)
		if cron {
			spec.Command = start
		} else {
			spec.StartCommand = start
		}
	})
	if err != nil {
		return AppView{}, err
	}
	if oldCloneSecret != "" {
		if err := s.deleteCloneSecret(ctx, a.Namespace, a.Name); err != nil {
			log.Printf("apps: clear clone secret for %s after linking a build source: %v", a.Name, err)
		}
	}
	s.RecordAppConfigChanged(ctx, a, core.AuditVerbSetSourceAndRegistryCredential)
	return updated, nil
}
