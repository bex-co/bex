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

package agentsessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Repository environments (ADR109): per-repository setup steps the driver runs
// before the agent and checks the overlay names. Limits are ADR109 D3.
const (
	maxRepoEnvironments  = 50
	maxRepoSetupSteps    = 10
	maxRepoChecks        = 10
	maxRepoSetupRunBytes = 8 << 10
	maxRepoCheckRunBytes = 2 << 10
	// maxRepoEnvironmentBytes bounds the encoded environment, which reaches
	// the sandbox as one env var (Linux caps one at 128 KiB).
	maxRepoEnvironmentBytes = 64 << 10
	// repoEnvironmentEnv carries the session repository's environment to the
	// driver (lego/agent-image/driver/src/repo-setup.ts).
	repoEnvironmentEnv = "BEX_REPO_ENVIRONMENT"
	// FailureCodeSetupFailed is the failureReasonCode of a turn whose setup
	// step failed (ADR109 D5): the one code bex-api accepts from the driver.
	FailureCodeSetupFailed = "AGENT_SESSION_SETUP_FAILED"
	// RepoEnvironmentAuditTarget is the audit target of an environment write.
	RepoEnvironmentAuditTarget = "agent-repo-environment"
)

var repoEnvironmentReadFailure = store.AgentSessionFailure{Reason: "the repository's environment could not be read"}

var repoStepNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,63}$`)

// RepoStep is one setup step or check: a name and a shell command.
type RepoStep struct {
	Name string `json:"name" jsonschema:"a name unique in its list: a letter or digit, then letters, digits, spaces, '.', '_' or '-' (at most 64)"`
	Run  string `json:"run" jsonschema:"the shell command, run with bash from the repository root"`
}

// RepoEnvironmentInput is a whole environment write.
type RepoEnvironmentInput struct {
	Setup  []RepoStep `json:"setup" jsonschema:"ordered setup steps run before the agent, at most 10, each at most 8 KiB"`
	Checks []RepoStep `json:"checks" jsonschema:"checks the agent runs before it delivers (by convention lint, test, build), at most 10, each at most 2 KiB"`
}

// RepoEnvironment is a repository's environment as every read shows it.
// Version hashes its setup steps (ADR111 D7): a build whose version differs
// is stale. LatestBuild is its newest build, when it has one.
type RepoEnvironment struct {
	OwnerID     string                `json:"ownerId"`
	Repo        string                `json:"repo"`
	Setup       []RepoStep            `json:"setup"`
	Checks      []RepoStep            `json:"checks"`
	Version     string                `json:"version"`
	LatestBuild *RepoEnvironmentBuild `json:"latestBuild,omitempty"`
	UpdatedAt   time.Time             `json:"updatedAt"`
}

// RepoEnvironmentList is a workspace's repository environments.
type RepoEnvironmentList struct {
	OwnerID      string            `json:"ownerId"`
	Environments []RepoEnvironment `json:"environments"`
}

// RepoEnvironmentStore is the agent_repo_environments table (*store.PGStore).
type RepoEnvironmentStore interface {
	ListAgentRepoEnvironments(ctx context.Context, workspaceID string) ([]store.AgentRepoEnvironment, error)
	GetAgentRepoEnvironment(ctx context.Context, workspaceID, repo string) (store.AgentRepoEnvironment, error)
	UpsertAgentRepoEnvironment(ctx context.Context, workspaceID, repo string, setup, checks json.RawMessage, maxRepos int) (store.AgentRepoEnvironment, bool, error)
	DeleteAgentRepoEnvironment(ctx context.Context, workspaceID, repo string) error
}

func repoEnvironmentInvalid(field, msg string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	details["field"] = field
	return core.NewBadRequestError("AGENT_REPO_ENVIRONMENT_INVALID", msg, details)
}

func errRepoEnvironmentsUnavailable() error {
	return core.NewUnavailableError("AGENT_REPO_ENVIRONMENTS_UNAVAILABLE", "repository environments need the control-plane store, which this deployment does not configure", nil)
}

func errRepoEnvironmentNotFound(repo string) error {
	return core.NewNotFoundError("AGENT_REPO_ENVIRONMENT_NOT_FOUND", "this repository has no environment", map[string]any{"repo": repo})
}

func normalizeEnvironmentRepo(raw string) (string, error) {
	repo, err := agentsession.NormalizeRepository(raw)
	if err != nil {
		return "", repoEnvironmentInvalid("repo", "a repository is owner/name", nil)
	}
	return repo, nil
}

func validateRepoSteps(field string, steps []RepoStep, maxSteps, maxRun int) error {
	if len(steps) > maxSteps {
		return repoEnvironmentInvalid(field, fmt.Sprintf("at most %d %s", maxSteps, field), map[string]any{"limit": maxSteps})
	}
	seen := map[string]bool{}
	for i, step := range steps {
		if !repoStepNamePattern.MatchString(step.Name) {
			return repoEnvironmentInvalid(field, "a name starts with a letter or digit and has at most 64 letters, digits, spaces, '.', '_' or '-'", map[string]any{"index": i})
		}
		if seen[step.Name] {
			return repoEnvironmentInvalid(field, "a name appears twice", map[string]any{"name": step.Name})
		}
		seen[step.Name] = true
		if step.Run == "" || len(step.Run) > maxRun || !core.ValidEnvValue(step.Run) {
			return repoEnvironmentInvalid(field, fmt.Sprintf("a command is 1 B to %d KiB with no NUL byte", maxRun>>10), map[string]any{"name": step.Name, "maxBytes": maxRun})
		}
	}
	return nil
}

func (in RepoEnvironmentInput) validate() error {
	if err := validateRepoSteps("setup", in.Setup, maxRepoSetupSteps, maxRepoSetupRunBytes); err != nil {
		return err
	}
	if err := validateRepoSteps("checks", in.Checks, maxRepoChecks, maxRepoCheckRunBytes); err != nil {
		return err
	}
	if len(in.Setup) == 0 && len(in.Checks) == 0 {
		return repoEnvironmentInvalid("environment", "an environment has at least one setup step or check; delete it instead", nil)
	}
	return nil
}

func repoEnvironmentView(row store.AgentRepoEnvironment, latest map[string]RepoEnvironmentBuild) RepoEnvironment {
	out := RepoEnvironment{OwnerID: row.WorkspaceID, Repo: row.Repo, Setup: []RepoStep{}, Checks: []RepoStep{}, Version: row.Version, UpdatedAt: row.UpdatedAt}
	_ = json.Unmarshal(row.Setup, &out.Setup)
	_ = json.Unmarshal(row.Checks, &out.Checks)
	if build, ok := latest[row.Repo]; ok {
		out.LatestBuild = &build
	}
	return out
}

// RepoEnvironments lists the workspace's repository environments. Members
// who can start sessions read them: they shape those sessions.
func (s *Service) RepoEnvironments(ctx context.Context, ownerID string) (RepoEnvironmentList, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanCreate, ownerID)
	if err != nil {
		return RepoEnvironmentList{}, err
	}
	if s.RepoEnvs == nil {
		return RepoEnvironmentList{}, errRepoEnvironmentsUnavailable()
	}
	rows, err := s.RepoEnvs.ListAgentRepoEnvironments(ctx, workspaceID)
	if err != nil {
		return RepoEnvironmentList{}, core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
	}
	latest := s.latestBuilds(ctx, workspaceID)
	out := RepoEnvironmentList{OwnerID: workspaceID, Environments: make([]RepoEnvironment, 0, len(rows))}
	for _, row := range rows {
		out.Environments = append(out.Environments, repoEnvironmentView(row, latest))
	}
	return out, nil
}

// SetRepoEnvironment creates or replaces one repository's environment
// (admins only). Sessions pick it up from their next dispatch. A save that
// changes the setup steps queues a build of them (ADR111 D3).
func (s *Service) SetRepoEnvironment(ctx context.Context, ownerID, repo string, in RepoEnvironmentInput) (RepoEnvironment, error) {
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, core.RelCanManage, ownerID, RepoEnvironmentAuditTarget)
	if err != nil {
		return RepoEnvironment{}, err
	}
	repo, err = normalizeEnvironmentRepo(repo)
	if err != nil {
		return RepoEnvironment{}, err
	}
	if in.Setup == nil {
		in.Setup = []RepoStep{}
	}
	if in.Checks == nil {
		in.Checks = []RepoStep{}
	}
	if err := in.validate(); err != nil {
		return RepoEnvironment{}, err
	}
	if s.RepoEnvs == nil {
		return RepoEnvironment{}, errRepoEnvironmentsUnavailable()
	}
	setup, _ := json.Marshal(in.Setup)
	checks, _ := json.Marshal(in.Checks)
	if len(setup)+len(checks) > maxRepoEnvironmentBytes {
		return RepoEnvironment{}, repoEnvironmentInvalid("environment",
			fmt.Sprintf("an environment's commands total at most %d KiB", maxRepoEnvironmentBytes>>10), map[string]any{"maxBytes": maxRepoEnvironmentBytes})
	}
	row, changed, err := s.RepoEnvs.UpsertAgentRepoEnvironment(ctx, workspaceID, repo, setup, checks, maxRepoEnvironments)
	if errors.Is(err, store.ErrAgentRepoEnvironmentLimit) {
		return RepoEnvironment{}, core.NewBadRequestError("AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED",
			fmt.Sprintf("a workspace has environments for at most %d repositories", maxRepoEnvironments), map[string]any{"limit": maxRepoEnvironments})
	}
	if err != nil {
		return RepoEnvironment{}, core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
	}
	if changed {
		s.triggerBuild(ctx, workspaceID, repo, buildTriggerEnvironmentSaved)
	}
	return repoEnvironmentView(row, s.latestBuilds(ctx, workspaceID)), nil
}

// DeleteRepoEnvironment removes one repository's environment (admins only),
// cancelling its builds and retiring their images (ADR111 D8).
func (s *Service) DeleteRepoEnvironment(ctx context.Context, ownerID, repo string) error {
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, core.RelCanManage, ownerID, RepoEnvironmentAuditTarget)
	if err != nil {
		return err
	}
	repo, err = normalizeEnvironmentRepo(repo)
	if err != nil {
		return err
	}
	if s.RepoEnvs == nil {
		return errRepoEnvironmentsUnavailable()
	}
	if err := s.RepoEnvs.DeleteAgentRepoEnvironment(ctx, workspaceID, repo); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errRepoEnvironmentNotFound(repo)
		}
		return core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
	}
	return nil
}

// withRepoEnvironment adds the session repository's environment to the
// dispatch env (ADR109 D7) and returns it; a session with no repository, or a
// repository with no environment, gets none (the zero environment).
func (s *Service) withRepoEnvironment(ctx context.Context, record store.AgentSession, env map[string]string) (map[string]string, store.AgentRepoEnvironment, error) {
	if s.RepoEnvs == nil || record.Repo == "" {
		return env, store.AgentRepoEnvironment{}, nil
	}
	row, err := s.RepoEnvs.GetAgentRepoEnvironment(ctx, record.WorkspaceID, record.Repo)
	if errors.Is(err, store.ErrNotFound) {
		return env, store.AgentRepoEnvironment{}, nil
	}
	if err != nil {
		return nil, store.AgentRepoEnvironment{}, err
	}
	return withEnv(env, repoEnvironmentEnv, repoEnvironmentJSON(row)), row, nil
}

// repoEnvironmentJSON is BEX_REPO_ENVIRONMENT: the stored JSON as it is, since
// re-encoding would only escape shell's & < >.
func repoEnvironmentJSON(row store.AgentRepoEnvironment) string {
	return `{"setup":` + string(row.Setup) + `,"checks":` + string(row.Checks) + `}`
}
