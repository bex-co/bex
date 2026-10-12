package resolve

import (
	"context"
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/owner"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/validate"
)

type Resolver struct {
	ownerRepo       *owner.Repo
	projectRepo     *project.Repo
	environmentRepo *environment.Repo
}

// ScopeInput contains optional CLI resource selectors. Each field may be a
// Render ID or a human-readable name.
type ScopeInput struct {
	WorkspaceIDOrName   string
	ProjectIDOrName     *string
	EnvironmentIDOrName *string
}

// ActiveWorkspaceScopeInput contains optional CLI resource selectors that
// should be resolved within the configured active workspace.
type ActiveWorkspaceScopeInput struct {
	ProjectIDOrName     *string
	EnvironmentIDOrName *string
}

// Scope is the resolved workspace, project, and environment context implied by
// a command's resource selectors.
type Scope struct {
	WorkspaceID string
	Project     *client.Project
	Environment *client.Environment
}

// ProjectID returns the resolved project ID, if a project was resolved.
func (s *Scope) ProjectID() *string {
	if s == nil || s.Project == nil {
		return nil
	}
	return &s.Project.Id
}

// EnvironmentID returns the resolved environment ID, if an environment was
// resolved.
func (s *Scope) EnvironmentID() *string {
	if s == nil || s.Environment == nil {
		return nil
	}
	return &s.Environment.Id
}

// New builds a Resolver from existing repos. Prefer this so callers can share
// cached repo instances from a dependency container instead of constructing a
// parallel set of repos for the same HTTP client.
func New(ownerRepo *owner.Repo, projectRepo *project.Repo, environmentRepo *environment.Repo) *Resolver {
	return &Resolver{
		ownerRepo:       ownerRepo,
		projectRepo:     projectRepo,
		environmentRepo: environmentRepo,
	}
}

// NewFromClient builds a Resolver with repos backed by the given HTTP client.
// Prefer New when a dependency container or shared repos are already available.
func NewFromClient(c *client.ClientWithResponses) *Resolver {
	return New(owner.NewRepo(c), project.NewRepo(c), environment.NewRepo(c))
}

// ResolveScope resolves explicitly supplied resources and their ancestors.
// Project input resolves workspace and project; environment input resolves
// workspace, project, and environment. It does not infer descendant resources
// that were not supplied. The configured active workspace is only used when no
// scope input is provided.
func (r *Resolver) ResolveScope(ctx context.Context, input ScopeInput) (*Scope, error) {
	scope := &Scope{}

	if input.WorkspaceIDOrName != "" {
		workspaceID, err := r.ResolveWorkspaceID(ctx, input.WorkspaceIDOrName)
		if err != nil {
			return nil, err
		}
		scope.WorkspaceID = workspaceID
	}

	if input.ProjectIDOrName != nil {
		project, err := r.ResolveProject(ctx, *input.ProjectIDOrName, scope.WorkspaceID)
		if err != nil {
			return nil, err
		}
		if err := validateProjectBelongsToWorkspace(project, scope.WorkspaceID); err != nil {
			return nil, err
		}
		scope.Project = project
		if scope.WorkspaceID == "" {
			scope.WorkspaceID = project.Owner.Id
		}
	}

	if input.EnvironmentIDOrName != nil {
		env, project, err := r.resolveEnvironmentAndProject(ctx, *input.EnvironmentIDOrName, scope.WorkspaceID, scope.Project)
		if err != nil {
			return nil, err
		}
		scope.Environment = env
		if scope.Project == nil {
			scope.Project = project
		}
		if scope.WorkspaceID == "" {
			scope.WorkspaceID = project.Owner.Id
		}
	}

	// No input implied a workspace; fall back to the active workspace.
	if scope.WorkspaceID == "" {
		workspaceID, err := r.ResolveWorkspaceID(ctx, "")
		if err != nil {
			return nil, err
		}
		scope.WorkspaceID = workspaceID
	}

	return scope, nil
}

// ResolveEnvironment is a convenience wrapper around ResolveScope for callers
// that have only an environment selector and want the full *client.Environment
// (with Name, Id, ProjectId, etc.) for downstream use such as scoping a
// resource lookup or building user-facing error messages. Ancestor resolution
// matches ResolveScope. Callers decide whether to invoke this — pass a real
// selector or skip the call entirely.
func (r *Resolver) ResolveEnvironment(ctx context.Context, envIDOrName string) (*client.Environment, error) {
	scope, err := r.ResolveScope(ctx, ScopeInput{EnvironmentIDOrName: &envIDOrName})
	if err != nil {
		return nil, err
	}
	return scope.Environment, nil
}

// ResolveScopeInActiveWorkspace is like ResolveScope but constrains all
// supplied project and environment selectors to the configured active
// workspace. Use this when the command's primary resources are active-workspace
// scoped and project/environment inputs should only narrow within that scope.
func (r *Resolver) ResolveScopeInActiveWorkspace(ctx context.Context, input ActiveWorkspaceScopeInput) (*Scope, error) {
	workspaceID, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}
	return r.ResolveScope(ctx, ScopeInput{
		WorkspaceIDOrName:   workspaceID,
		ProjectIDOrName:     input.ProjectIDOrName,
		EnvironmentIDOrName: input.EnvironmentIDOrName,
	})
}

// ResolveWorkspaceID resolves an optional workspace ID or name to an owner ID.
// If idOrName is empty, it returns the configured workspace ID.
func (r *Resolver) ResolveWorkspaceID(ctx context.Context, idOrName string) (string, error) {
	if idOrName == "" {
		return config.WorkspaceID()
	}

	if validate.IsWorkspaceID(idOrName) {
		o, err := r.ownerRepo.RetrieveOwner(ctx, idOrName)
		if err != nil {
			return "", fmt.Errorf("workspace %q not found: %w", idOrName, err)
		}
		return o.Id, nil
	}

	owners, err := r.ownerRepo.ListOwners(ctx, owner.ListInput{Name: idOrName})
	if err != nil {
		return "", err
	}
	if len(owners) == 0 {
		return "", fmt.Errorf("no workspace found with name %q", idOrName)
	}
	if len(owners) > 1 {
		return "", fmt.Errorf("multiple workspaces found with name %q — please use the workspace ID instead", idOrName)
	}
	return owners[0].Id, nil
}

func validateProjectBelongsToWorkspace(project *client.Project, workspaceID string) error {
	if project == nil || workspaceID == "" || project.Owner.Id == workspaceID {
		return nil
	}
	return fmt.Errorf("project %q belongs to workspace %q, not workspace %q", project.Id, project.Owner.Id, workspaceID)
}

func validateEnvironmentBelongsToProject(env *client.Environment, resolvedProject *client.Project) error {
	if resolvedProject == nil || env.ProjectId == resolvedProject.Id {
		return nil
	}
	return fmt.Errorf("environment %q belongs to project %q, not project %q", env.Id, env.ProjectId, resolvedProject.Id)
}

func validateEnvironmentBelongsToWorkspace(env *client.Environment, project *client.Project, workspaceID string) error {
	if project == nil {
		return fmt.Errorf("environment %q has no resolved project", env.Id)
	}
	if err := validateEnvironmentBelongsToProject(env, project); err != nil {
		return err
	}
	if workspaceID == "" || project.Owner.Id == workspaceID {
		return nil
	}
	return fmt.Errorf("environment %q belongs to workspace %q, not workspace %q", env.Id, project.Owner.Id, workspaceID)
}

// resolveEnvironmentAndProject resolves an environment selector and returns the
// environment plus its owning project. workspaceID and resolvedProject are
// optional constraints: when present, the resolved environment must belong to
// them; when absent, this helper resolves the missing ancestors from the
// environment itself.
func (r *Resolver) resolveEnvironmentAndProject(ctx context.Context, envIDOrName string, workspaceID string, resolvedProject *client.Project) (*client.Environment, *client.Project, error) {
	if validate.IsEnvironmentID(envIDOrName) {
		env, err := r.environmentRepo.GetEnvironment(ctx, envIDOrName)
		if err != nil {
			return nil, nil, fmt.Errorf("environment %q not found: %w", envIDOrName, err)
		}
		project, err := r.projectRepo.GetProject(ctx, env.ProjectId)
		if err != nil {
			return nil, nil, err
		}
		if err := validateEnvironmentBelongsToProject(env, resolvedProject); err != nil {
			return nil, nil, err
		}
		if err := validateEnvironmentBelongsToWorkspace(env, project, workspaceID); err != nil {
			return nil, nil, err
		}
		return env, project, nil
	}

	projects, err := r.candidateProjectsForEnvironmentLookup(ctx, resolvedProject, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	projectIDs := make([]string, 0, len(projects))
	projectByID := make(map[string]*client.Project, len(projects))
	for _, p := range projects {
		projectIDs = append(projectIDs, p.Id)
		projectByID[p.Id] = p
	}
	if len(projectIDs) == 0 {
		return nil, nil, fmt.Errorf("no environment found with name %q", envIDOrName)
	}

	nameFilter := client.NameParam{envIDOrName}
	matches, err := r.environmentRepo.ListEnvironments(ctx, &client.ListEnvironmentsParams{
		ProjectId: projectIDs,
		Name:      &nameFilter,
	})
	if err != nil {
		return nil, nil, err
	}

	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("no environment found with name %q", envIDOrName)
	}
	if len(matches) > 1 {
		return nil, nil, fmt.Errorf("multiple environments found with name %q across projects — please use the environment ID instead", envIDOrName)
	}

	env := matches[0]
	project := projectByID[env.ProjectId]
	if project == nil {
		project, err = r.projectRepo.GetProject(ctx, env.ProjectId)
		if err != nil {
			return nil, nil, err
		}
	}
	if err := validateEnvironmentBelongsToProject(env, resolvedProject); err != nil {
		return nil, nil, err
	}
	if err := validateEnvironmentBelongsToWorkspace(env, project, workspaceID); err != nil {
		return nil, nil, err
	}
	return env, project, nil
}

// candidateProjectsForEnvironmentLookup returns the projects whose environments
// may match an environment name selector under the current scope constraints.
func (r *Resolver) candidateProjectsForEnvironmentLookup(ctx context.Context, resolvedProject *client.Project, workspaceID string) ([]*client.Project, error) {
	if resolvedProject != nil {
		return []*client.Project{resolvedProject}, nil
	}

	var projects []*client.Project
	var err error
	if workspaceID != "" {
		projects, err = r.projectRepo.ListProjectsForWorkspace(ctx, workspaceID)
	} else {
		projects, err = r.projectRepo.ListAllAccessibleProjects(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	return projects, nil
}

// ResolveProject resolves a project ID or name to a project.
// Name lookup is server-side and case-sensitive; if workspaceID is empty the
// search spans every workspace the caller can access. Returns the full
// *client.Project so callers can read Owner.Id without re-fetching.
func (r *Resolver) ResolveProject(ctx context.Context, idOrName string, workspaceID string) (*client.Project, error) {
	if validate.IsProjectID(idOrName) {
		proj, err := r.projectRepo.GetProject(ctx, idOrName)
		if err != nil {
			return nil, fmt.Errorf("project %q not found: %w", idOrName, err)
		}
		return proj, nil
	}

	matches, err := r.projectRepo.ListProjectsByName(ctx, idOrName, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no project found with name %q", idOrName)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple projects found with name %q — please use the project ID instead", idOrName)
	}
	return matches[0], nil
}

// ResolveEnvironmentID resolves optional project and environment inputs to an
// environment ID. If a project is provided without an environment, it returns
// the project's single environment ID, errors if the project has multiple
// environments, and returns nil if the project has none. If project or workspace
// context is provided with an environment input, it must be consistent.
func (r *Resolver) ResolveEnvironmentID(ctx context.Context, resolvedProject *client.Project, envIDOrName *string, workspaceID string) (*string, error) {
	if envIDOrName != nil {
		env, _, err := r.resolveEnvironmentAndProject(ctx, *envIDOrName, workspaceID, resolvedProject)
		if err != nil {
			return nil, err
		}
		return &env.Id, nil
	}

	// Project given, no env name — use the single environment or error with the list.
	if resolvedProject != nil {
		env, err := r.resolveSingleProjectEnvironment(ctx, resolvedProject.Id)
		if err != nil {
			return nil, err
		}
		if env != nil {
			return &env.Id, nil
		}
	}

	return nil, nil
}

func (r *Resolver) resolveSingleProjectEnvironment(ctx context.Context, projectID string) (*client.Environment, error) {
	envs, err := r.environmentRepo.ListEnvironments(ctx, &client.ListEnvironmentsParams{
		ProjectId: []string{projectID},
	})
	if err != nil {
		return nil, err
	}
	if len(envs) == 1 {
		return envs[0], nil
	}
	if len(envs) > 1 {
		names := make([]string, len(envs))
		for i, e := range envs {
			names[i] = e.Name
		}
		return nil, fmt.Errorf("project has multiple environments (%s) — specify one with --environment", strings.Join(names, ", "))
	}
	return nil, nil
}
