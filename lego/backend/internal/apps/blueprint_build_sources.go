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
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Blueprint Build Sources (ADR097 § Blueprints, w3/m270). A buildSources entry
// is a build source carrier, identified by its name within the workspace, as
// on Render. It compiles into the build-only service declaration the carrier
// already is: buildSourceGit's fields carry the same names as a service's
// (repo, branch, runtime, buildCommand, …), so the carrier is planned, created
// and synchronized by the presence-aware service machinery, and a changed
// carrier rebuilds the way a changed service redeploys. services[].buildSource
// links a service to one by name.

// bexBuildSource is one buildSources entry.
type bexBuildSource struct {
	Name  string             `json:"name"`
	Git   *bexBuildSourceGit `json:"git"`
	Image *bexImage          `json:"image"`
}

// bexBuildSourceGit is Render's buildSourceGit.
type bexBuildSourceGit struct {
	Repo               string                    `json:"repo"`
	Branch             string                    `json:"branch"`
	Runtime            string                    `json:"runtime"`
	BuildCommand       string                    `json:"buildCommand"`
	BuildFilter        *BuildFilterView          `json:"buildFilter"`
	EnvVars            []bexEnvVar               `json:"envVars"`
	RootDir            string                    `json:"rootDir"`
	DockerContext      string                    `json:"dockerContext"`
	DockerfilePath     string                    `json:"dockerfilePath"`
	Region             string                    `json:"region"`
	RegistryCredential *bexRegistryCredentialRef `json:"registryCredential"`
}

// parsedBuildSource is a validated buildSources entry.
type parsedBuildSource struct {
	// name is the build source's identity, its carrier's display name.
	name string
	// project names the project a projects[].buildSources entry is scoped to;
	// "" for the root and ungrouped.
	project string
	region  string
	// native is a git source built by a native runtime, whose linked services
	// need their own start command (Render).
	native bool
	// svc is the carrier as a build-only service declaration; svc.req.Name is
	// the build source's name until the create mints its id.
	svc parsedService
	// refVars are its fromDatabase / Key Value references, classified with
	// the services' in parse pass 2.
	refVars []envRef
}

// blueprintBuildSourceEnvError refuses the one build-time env var form a
// build source cannot hold: a reference to a service (its address or one of
// its values). Build sources apply before services, so the build would run
// before what it references exists; a Key Value's fromService is a datastore
// reference and is served.
func blueprintBuildSourceEnvError(name string, i int, e bexEnvVar) error {
	if e.FromService != nil && !isKeyValueType(e.FromService.Type) {
		return atEnvEntry(i, fmt.Errorf("%w: build source %q envVars[%q]: fromService a service is not available on a build source, which builds before the services it would reference", core.ErrBadRequest, name, e.Key))
	}
	return nil
}

// parseBuildSource compiles a buildSources entry into its carrier declaration.
func parseBuildSource(overrides blueprintParseOverrides, b bexBuildSource, project string, fields map[string]BlueprintField) (parsedBuildSource, error) {
	name := strings.TrimSpace(b.Name)
	if name == "" {
		return parsedBuildSource{}, fmt.Errorf("%w: a build source entry is missing its name", core.ErrBadRequest)
	}
	decl := bexService{Name: name, Type: "worker"}
	region := ""
	native := false
	switch {
	case b.Git != nil:
		g := b.Git
		decl.Repo, decl.Branch, decl.Runtime, decl.BuildCommand = g.Repo, g.Branch, g.Runtime, g.BuildCommand
		decl.BuildFilter, decl.RootDir, decl.DockerContext, decl.DockerfilePath = g.BuildFilter, g.RootDir, g.DockerContext, g.DockerfilePath
		decl.RegistryCredential, decl.EnvVars = g.RegistryCredential, g.EnvVars
		region = g.Region
		native = blueprintNativeRuntime(g.Runtime)
	case b.Image != nil:
		decl.Runtime, decl.Image = "image", b.Image
	default: // the compiler requires exactly one
		return parsedBuildSource{}, fmt.Errorf("%w: build source %q needs git or image", core.ErrBadRequest, name)
	}
	for i, e := range decl.EnvVars {
		if err := blueprintBuildSourceEnvError(name, i, e); err != nil {
			return parsedBuildSource{}, err
		}
	}
	req, se, err := parseService(overrides, decl)
	if err != nil {
		return parsedBuildSource{}, err
	}
	credName, err := blueprintRegistryCredentialName(decl)
	if err != nil {
		return parsedBuildSource{}, err
	}
	req.buildSource = &buildSourceCreate{name: name}
	return parsedBuildSource{
		name: name, project: project, region: region, native: native, refVars: se.refVars,
		svc: parsedService{req: req, fields: blueprintBuildSourceFields(fields), registryCredentialName: credName,
			groupLinks: se.groupLinks, seedLiterals: se.seedLiterals, seedGenerates: se.seedGenerates, promptKeys: se.promptKeys, env: se},
	}, nil
}

// blueprintBuildSourceFields lifts a build source's git (or image) block to
// the service field names the presence-aware appliers read: git.branch is the
// carrier's branch, image is its image.
func blueprintBuildSourceFields(fields map[string]BlueprintField) map[string]BlueprintField {
	out := map[string]BlueprintField{}
	if image, ok := fields["image"]; ok {
		out["image"] = image
		out["runtime"] = BlueprintField{Value: "image", Location: image.Location}
	}
	git, ok := fields["git"]
	if !ok {
		return out
	}
	values, _ := git.Value.(map[string]any)
	for name, value := range values {
		if name == "region" {
			continue // one region per installation (ADR097 §7)
		}
		out[name] = BlueprintField{Null: value == nil, Value: value, Location: git.Location}
	}
	return out
}

// buildSourceFieldPath is a carrier field's path in the build source entry,
// for the plan's changed fields.
func buildSourceFieldPath(fields map[string]BlueprintField, name string) string {
	if name == "image" || (name == "runtime" && fields["image"].Value != nil) {
		return "image"
	}
	return "git." + name
}

// parsedBlueprintBuildSource finds the declared build source name.
func parsedBlueprintBuildSource(st parsedStack, name string) (parsedBuildSource, bool) {
	for _, b := range st.buildSources {
		if b.name == name {
			return b, true
		}
	}
	return parsedBuildSource{}, false
}

// checkDeclaredBuildSourceLink applies Render's start-command rule to a
// service linked to a build source this Blueprint declares: a native source
// needs the service's start command, a Docker or image one takes none.
func checkDeclaredBuildSourceLink(svc parsedService, b parsedBuildSource) error {
	start := linkStartCommand(svc.req.Type, svc.req.StartCommand, svc.req.Command)
	if err := buildSourceStartRule(b.native, strings.TrimSpace(start), b.name); err != nil {
		return fmt.Errorf("service %q: %w", svc.req.Name, err)
	}
	return nil
}

// buildSources keys the workspace's build source carriers by name, the
// identity a Blueprint declares them under. Two carriers sharing a name are
// refused: list order would otherwise pick which one a Blueprint manages.
func (snap *workspaceSnapshot) buildSources(ctx context.Context) (map[string]*appv1alpha1.App, error) {
	if snap.buildSourceMap != nil {
		return snap.buildSourceMap, nil
	}
	list, err := snap.allApps(ctx)
	if err != nil {
		return nil, err
	}
	byName := map[string]*appv1alpha1.App{}
	for i := range list.Items {
		a := &list.Items[i]
		if !core.IsBuildSource(a) || a.Labels[core.LabelTenant] != snap.tenantID || core.NotFoundIfDeleting(a) != nil {
			continue
		}
		name := buildSourceName(a)
		// Of a carrier's copies mid-namespace move (ADR043 D8), the one in
		// the workspace's own namespace answers, as for a service.
		if other := byName[name]; other != nil {
			if other.Name != a.Name {
				return nil, fmt.Errorf("%w: build source name %q is already used more than once in this workspace", core.ErrConflict, name)
			}
			if !core.AppInOwnWorkspaceNamespace(a) {
				continue
			}
		}
		byName[name] = a
	}
	snap.buildSourceMap = byName
	return byName, nil
}

// buildSourceName is a carrier's build source name.
func buildSourceName(a *appv1alpha1.App) string {
	return cmp.Or(a.Spec.DisplayName, core.AppPublicName(a))
}

// blueprintBuildSourceLinks resolves every service's buildSource name to its
// carrier: one this Blueprint declares (nil until the apply creates it) or an
// existing workspace build source. A name that is neither is refused at the
// service, before any write.
func blueprintBuildSourceLinks(st parsedStack, carriers map[string]*appv1alpha1.App) (map[string]*appv1alpha1.App, error) {
	links := map[string]*appv1alpha1.App{}
	var problems blueprintResourceErrors
	for _, svc := range st.services {
		name := svc.req.blueprintBuildSource
		if name == "" {
			continue
		}
		if _, declared := parsedBlueprintBuildSource(st, name); declared {
			links[name] = carriers[name]
			continue
		}
		carrier := carriers[name]
		if carrier == nil {
			problems = append(problems, blueprintResourceError{kind: BlueprintResourceService, name: svc.req.Name,
				err: fmt.Errorf("%w: service %q: buildSource %q names no build source in this Blueprint or workspace", core.ErrBadRequest, svc.req.Name, name)})
			continue
		}
		links[name] = carrier
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return links, nil
}

// pendingBuildSourceLink stands in, in a plan, for the link to a build source
// the apply has yet to create: it matches no current link, so linking plans
// as a change.
func pendingBuildSourceLink(name string) *appv1alpha1.BuildSourceLink {
	return &appv1alpha1.BuildSourceLink{Name: "\x00pending build source " + name}
}

// desiredBuildSourceLink is the link a declared service's buildSource wants:
// the carrier's, or a pending stand-in for one not created yet.
func desiredBuildSourceLink(svc parsedService, links map[string]*appv1alpha1.App) *appv1alpha1.BuildSourceLink {
	name := svc.req.blueprintBuildSource
	if name == "" {
		return nil
	}
	if carrier := links[name]; carrier != nil {
		return core.BuildSourceLinkTo(carrier)
	}
	return pendingBuildSourceLink(name)
}

// buildSourceProjectID is the project a projects[].buildSources entry is
// scoped to: the project this apply resolved for one of that project's
// environments.
func buildSourceProjectID(project string, groupings []parsedGrouping, assignments map[string]core.EnvironmentAssignment) (string, error) {
	if project == "" {
		return "", nil
	}
	if assignment := assignments[blueprintGroupingKey(project, "")]; assignment.ProjectID != "" {
		return assignment.ProjectID, nil
	}
	for _, g := range groupings {
		if g.projectName != project {
			continue
		}
		if assignment := assignments[blueprintGroupingKey(g.projectName, g.environmentName)]; assignment.ProjectID != "" {
			return assignment.ProjectID, nil
		}
	}
	return "", fmt.Errorf("blueprint apply: project %q was not resolved before its build sources", project)
}

// applyStackBuildSources creates or synchronizes each declared build source
// before the services that link it, returning every carrier a service may
// link (declared ones as applied, then the workspace's others).
func (s *Service) applyStackBuildSources(ctx context.Context, st parsedStack, assignments map[string]core.EnvironmentAssignment, snap *workspaceSnapshot, dbRefs map[string]databaseRef, kvCRNames map[string]string, res *StackResult) (map[string]*appv1alpha1.App, error) {
	if len(st.buildSources) == 0 && !st.linksBuildSources() {
		return nil, nil
	}
	existing, err := snap.buildSources(ctx)
	if err != nil {
		return nil, err
	}
	carriers := maps.Clone(existing)
	for _, b := range st.buildSources {
		if err := requireDeployAuthority(ctx, s); err != nil {
			return nil, err
		}
		projectID, err := buildSourceProjectID(b.project, st.groupings, assignments)
		if err != nil {
			return nil, err
		}
		// Datastore references become the carrier's secret-ref env, which
		// its build mounts; nothing else in a build source refers forward.
		if _, err := resolveServiceRefs(&b.svc, dbRefs, kvCRNames, func(string) *appv1alpha1.App { return nil }, s.BaseDomain); err != nil {
			return nil, err
		}
		carrier, err := s.applyStackBuildSource(ctx, b, projectID, existing[b.name])
		if err != nil {
			return nil, fmt.Errorf("build source %q: %w", b.name, err)
		}
		carriers[b.name] = carrier
		res.BuildSources = append(res.BuildSources, StackBuildSourceView{ID: core.AppPublicID(carrier), Name: b.name})
	}
	return carriers, nil
}

// applyStackBuildSource upserts one build source: a create mints its carrier
// as CreateBuildSource does (and it builds once it exists); an existing one
// takes the declared fields through the service update, so a change rebuilds
// it and an unchanged re-apply writes nothing.
func (s *Service) applyStackBuildSource(ctx context.Context, b parsedBuildSource, projectID string, current *appv1alpha1.App) (*appv1alpha1.App, error) {
	if err := s.claimBlueprintResourceName(ctx, store.BlueprintClaimBuildSource, b.name); err != nil {
		return nil, err
	}
	req := b.svc.req
	// A new carrier links its groups as it is written, so its first build
	// has them; an existing one links them before the update rebuilds it.
	req.initialEnvGroups = b.svc.groupLinks
	if current == nil {
		id, err := s.createBuildSourceCarrier(ctx, req, b.name, projectID)
		if err != nil {
			return nil, err
		}
		_, carrier, err := s.buildSourceApp(ctx, core.RelCanView, id)
		if err != nil {
			return nil, err
		}
		return carrier, s.seedBuildSourceEnv(ctx, b, carrier)
	}
	// A build source's project cannot change (ADR097 Blueprints §2), either
	// way: a projects[] entry naming an unscoped or other-project source, or a
	// root/ungrouped entry naming a project-scoped one (w9/m180 t004).
	scope, err := s.buildSourceScope(ctx, current)
	if err != nil {
		return nil, err
	}
	switch {
	case scope != projectID && b.project != "":
		return nil, fmt.Errorf("%w: build source %q is not scoped to project %q; a build source's project cannot change", core.ErrBadRequest, b.name, b.project)
	case scope != projectID:
		return nil, fmt.Errorf("%w: build source %q is scoped to a project; declare it under that project's buildSources, since a build source's project cannot change", core.ErrBadRequest, b.name)
	}
	desired, err := specFromCreate(req)
	if err != nil {
		return nil, err
	}
	req.buildSource.shape(&desired)
	ctx = core.WithBuildSources(ctx)
	verbCtx, ref := stackServiceTarget(ctx, current)
	for _, g := range b.svc.groupLinks {
		if err := s.EnvGroups.LinkEnvGroup(verbCtx, g, ref); err != nil {
			return nil, fmt.Errorf("linking env group %q: %w", g, err)
		}
	}
	if err := s.seedBuildSourceEnv(ctx, b, current); err != nil {
		return nil, err
	}
	if _, err := s.updateStackService(ctx, req, b.svc.fields, desired, current); err != nil {
		return nil, err
	}
	return s.currentStackService(ctx, current)
}

// seedBuildSourceEnv seeds a carrier's sync: false and generateValue
// build-time vars once, as a service's are: a later sync neither overwrites
// nor re-mints them.
func (s *Service) seedBuildSourceEnv(ctx context.Context, b parsedBuildSource, carrier *appv1alpha1.App) error {
	if len(b.svc.seedLiterals) == 0 && len(b.svc.seedGenerates) == 0 {
		return nil
	}
	verbCtx, ref := stackServiceTarget(core.WithBuildSources(ctx), carrier)
	if err := s.EnvSeeder.SeedEnvVars(verbCtx, ref, b.svc.seedLiterals, b.svc.seedGenerates); err != nil {
		return fmt.Errorf("seeding build-time env: %w", err)
	}
	return nil
}

// buildSourceScope is the project an existing carrier is scoped to; "" with
// no control plane, which scopes nothing.
func (s *Service) buildSourceScope(ctx context.Context, carrier *appv1alpha1.App) (string, error) {
	getter, ok := s.Store.(interface {
		GetApp(context.Context, string) (store.App, error)
	})
	if !ok {
		return "", nil
	}
	row, err := getter.GetApp(ctx, core.AppPublicID(carrier))
	if err != nil {
		return "", err
	}
	return row.ScopeProjectID, nil
}

// planBlueprintBuildSource is a build source's plan action: a create, or an
// update naming each git/image field whose declared value would change it.
func (r *blueprintActionResolver) planBlueprintBuildSource(ctx context.Context, resource BlueprintResourceIR, action BlueprintPlanAction) (BlueprintPlanAction, error) {
	b, ok := parsedBlueprintBuildSource(r.parsed, resource.Name)
	if !ok {
		return BlueprintPlanAction{}, fmt.Errorf("compiled build source is missing")
	}
	current := r.buildSources[resource.Name]
	want, err := blueprintReadSpec(ctx, b.svc.req)
	if err != nil {
		return BlueprintPlanAction{}, err
	}
	b.svc.req.buildSource.shape(&want)
	probe := *current.Spec.DeepCopy()
	changed, err := ApplyBlueprintServiceSpec(&probe, want, b.svc.fields)
	if err != nil {
		return BlueprintPlanAction{}, blueprintServiceResourceError(resource, err)
	}
	if !changed {
		action.Operation = BlueprintPlanNoop
		return action, nil
	}
	action.Operation = BlueprintPlanUpdate
	changes := blueprintPlanFieldChanges(blueprintServiceProbePaths(b.svc.fields), func(name string) (bool, error) {
		probe := *current.Spec.DeepCopy()
		return probeBlueprintServiceSpec(&probe, want, b.svc.fields, name)
	})
	seen := map[string]bool{}
	for _, change := range changes {
		if path := buildSourceFieldPath(b.svc.fields, change.Path); !seen[path] {
			seen[path] = true
			action.ChangedFields = append(action.ChangedFields, BlueprintFieldChange{Path: path})
		}
	}
	sort.Slice(action.ChangedFields, func(i, j int) bool { return action.ChangedFields[i].Path < action.ChangedFields[j].Path })
	return action, nil
}

// blueprintServiceLink is the build source link an existing service's
// buildSource wants, checked as a PATCH buildSourceId is: the service type,
// and the start-command rule against the declared or current command.
func (s *Service) blueprintServiceLink(ctx context.Context, req CreateRequest, fields map[string]BlueprintField, existing *appv1alpha1.App) (*appv1alpha1.BuildSourceLink, error) {
	current := linkStartCommand(existing.Spec.Type, existing.Spec.StartCommand, existing.Spec.Command)
	var start *string
	if _, declared := fields["startCommand"]; declared {
		command := linkStartCommand(existing.Spec.Type, req.StartCommand, req.Command)
		start = &command
	}
	link, _, _, err := s.buildSourceLink(ctx, existing.Labels[core.LabelTenant], req.BuildSourceID, existing.Spec.Type, start, current)
	if err != nil {
		return nil, fmt.Errorf("service %q: %w", req.Name, err)
	}
	return link, nil
}

// validateBlueprintBuildSources runs the create boundary over every declared
// build source and resolves every service's buildSource name, before any
// write: a source the create would refuse, or a link to no build source,
// fails the whole apply.
func (s *Service) validateBlueprintBuildSources(ctx context.Context, st parsedStack, snap *workspaceSnapshot) error {
	var problems blueprintResourceErrors
	for _, b := range st.buildSources {
		_, err := blueprintReadSpec(ctx, b.svc.req)
		if err == nil && b.region != "" {
			err = s.Metadata.CheckRequestedRegion(b.region)
		}
		if err != nil {
			problems = append(problems, blueprintResourceError{kind: BlueprintResourceBuildSource, name: b.name, err: fmt.Errorf("build source %q: %w", b.name, err)})
		}
	}
	if st.linksBuildSources() && s.Client != nil {
		carriers, err := snap.buildSources(ctx)
		if err != nil {
			return err
		}
		if _, err := blueprintBuildSourceLinks(st, carriers); err != nil {
			refused, _ := err.(blueprintResourceErrors)
			problems = append(problems, refused...)
		}
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}

// linksBuildSources reports whether any declared service names a build source.
func (st parsedStack) linksBuildSources() bool {
	return slices.ContainsFunc(st.services, func(svc parsedService) bool { return svc.req.blueprintBuildSource != "" })
}

// resolveBuildSourceIDs sets each linked service's BuildSourceID from the
// carriers its buildSource name resolved to; one not created yet stays unset.
func (st *parsedStack) resolveBuildSourceIDs(links map[string]*appv1alpha1.App) {
	for i := range st.services {
		if carrier := links[st.services[i].req.blueprintBuildSource]; carrier != nil {
			st.services[i].req.BuildSourceID = core.AppPublicID(carrier)
		}
	}
}

// applyBlueprintSourceProjects finds or creates each project that declares
// build sources and no environment, recording it under its project-only
// grouping key for buildSourceProjectID.
func (s *Service) applyBlueprintSourceProjects(ctx context.Context, names []string, assignments map[string]core.EnvironmentAssignment) error {
	if len(names) == 0 {
		return nil
	}
	tenantID, ok := s.Tenant(ctx)
	if s.BlueprintGroups == nil || !ok {
		return core.ErrWorkspacesUnavailable
	}
	projects, err := s.BlueprintGroups.ListProjects(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("listing Blueprint projects: %w", err)
	}
	byName := make(map[string]store.Project, len(projects))
	for _, p := range projects {
		byName[p.Name] = p
	}
	var audits []groupingAuditEntry
	for _, name := range names {
		project, exists := byName[name]
		if !exists {
			projectCount, environmentCount, err := s.BlueprintGroups.CountWorkspaceGroupings(ctx, tenantID)
			if err != nil {
				return fmt.Errorf("counting Blueprint groupings: %w", err)
			}
			if err := s.blueprintGroupingQuota(projectCount+1, environmentCount); err != nil {
				return err
			}
			if project, err = s.BlueprintGroups.CreateProject(ctx, tenantID, name); err != nil {
				return fmt.Errorf("creating Blueprint project %q: %w", name, err)
			}
			byName[name] = project
			audits = append(audits, groupingAuditEntry{action: "project_created", project: name})
		}
		assignments[blueprintGroupingKey(name, "")] = core.EnvironmentAssignment{ProjectID: project.ID}
	}
	s.emitGroupingAudits(ctx, tenantID, audits)
	return nil
}

// envDeclarations are the declarations whose envVars the apply materializes:
// every service, then every build source's carrier.
func (st parsedStack) envDeclarations() []parsedService {
	out := slices.Clone(st.services)
	for _, b := range st.buildSources {
		out = append(out, b.svc)
	}
	return out
}
