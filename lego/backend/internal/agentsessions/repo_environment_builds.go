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
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Repository environment builds (ADR111): a repository's setup steps baked
// into an image its sessions boot from, so no session waits for them.
const (
	// maxRepoEnvironmentBuildsPerWorkspace builds run at once in a workspace;
	// one runs per repository (ADR111 D3).
	maxRepoEnvironmentBuildsPerWorkspace = 2
	// repoEnvironmentBuildRateLimit builds start per repository in
	// repoEnvironmentBuildRateWindow (ADR111 D3).
	repoEnvironmentBuildRateLimit  = 12
	repoEnvironmentBuildRateWindow = 24 * time.Hour
	// repoEnvironmentBuildTimeout is a whole build's limit (ADR111 D5), which
	// bex-api enforces: the driver bounds only each step, by the same 3 h.
	repoEnvironmentBuildTimeout = 3 * time.Hour
	// A repository keeps its two newest successful builds' images and its 20
	// newest build records (ADR111 D8).
	repoEnvironmentBuildKeepImages  = 2
	repoEnvironmentBuildKeepRecords = 20
	// buildModeEnv selects the driver's build mode; prebuiltEnv tells a
	// session's driver which build its image came from (ADR111 D1, D4).
	buildModeEnv = "BEX_AGENT_MODE"
	prebuiltEnv  = "BEX_REPO_PREBUILT"

	// CodeRepoEnvironmentBuildRateLimited refuses a build past the
	// repository's starts in 24 hours (ADR111 D3).
	CodeRepoEnvironmentBuildRateLimited = "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED"
)

// What queued a build (ADR111 D3).
const (
	buildTriggerEnvironmentSaved = "environment_saved"
	buildTriggerSecretChanged    = "secret_changed"
	buildTriggerManual           = "manual"
	buildTriggerRefresh          = "refresh"
)

// The failure codes of a failed build (ADR111 D2).
const (
	buildFailedSetup         = "setup_failed"
	buildFailedStepTimeout   = "step_timeout"
	buildFailedTimeout       = store.AgentRepoEnvironmentBuildTimeout
	buildFailedCheckout      = "checkout_failed"
	buildFailedStorageLimit  = "storage_limit_reached"
	buildFailedInternalError = store.AgentRepoEnvironmentBuildInternalError
)

// RepoEnvironmentBuildStep is one setup step of a build as the driver
// reported it: its exit (null for a step killed or never started), duration
// and redacted output tail.
type RepoEnvironmentBuildStep struct {
	Name       string `json:"name"`
	ExitCode   *int   `json:"exitCode"`
	DurationMs int64  `json:"durationMs"`
	Output     string `json:"output"`
}

// RepoEnvironmentBuild is a build as every read shows it. Steps, the build's
// redacted log, is read one build at a time; lists leave it out.
type RepoEnvironmentBuild struct {
	ID      string `json:"id"`
	OwnerID string `json:"ownerId"`
	Repo    string `json:"repo"`
	// Version is the environment version the build baked (ADR111 D7).
	Version string `json:"version"`
	// Trigger is environment_saved, secret_changed, manual or refresh.
	Trigger string `json:"trigger"`
	// Status is queued, building, success, failed or cancelled (ADR111 D2).
	Status        string `json:"status"`
	FailureCode   string `json:"failureCode,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
	// HeadSHA is the default branch commit the build checked out.
	HeadSHA string `json:"headSha,omitempty"`
	// SizeBytes is the image's compressed size.
	SizeBytes *int64 `json:"sizeBytes,omitempty"`
	// ImageAvailable reports whether sessions can boot from the build's image.
	ImageAvailable bool                       `json:"imageAvailable"`
	Steps          []RepoEnvironmentBuildStep `json:"steps,omitempty"`
	CreatedAt      time.Time                  `json:"createdAt"`
	StartedAt      *time.Time                 `json:"startedAt,omitempty"`
	FinishedAt     *time.Time                 `json:"finishedAt,omitempty"`
}

// RepoEnvironmentBuildList is a repository's builds, newest first.
type RepoEnvironmentBuildList struct {
	OwnerID string                 `json:"ownerId"`
	Repo    string                 `json:"repo"`
	Builds  []RepoEnvironmentBuild `json:"builds"`
}

// RepoEnvironmentBuildStore is the agent_repo_environment_builds table
// (*store.PGStore).
type RepoEnvironmentBuildStore interface {
	agentsession.BuildStore
	QueueAgentRepoEnvironmentBuild(ctx context.Context, in store.NewAgentRepoEnvironmentBuild) (store.AgentRepoEnvironmentBuild, error)
	ClaimAgentRepoEnvironmentBuild(ctx context.Context, limits store.AgentRepoEnvironmentBuildLimits) (store.ClaimedAgentRepoEnvironmentBuild, error)
	SetAgentRepoEnvironmentBuilder(ctx context.Context, id, sandboxID, baseImage string) (bool, error)
	HeartbeatAgentRepoEnvironmentBuild(ctx context.Context, id string) (bool, error)
	SucceedAgentRepoEnvironmentBuild(ctx context.Context, id string, image store.AgentRepoEnvironmentBuildImage) (bool, error)
	FailAgentRepoEnvironmentBuild(ctx context.Context, id, code, reason string, steps json.RawMessage) (bool, error)
	FailStaleAgentRepoEnvironmentBuilds(ctx context.Context, staleAfter, maxDuration time.Duration) ([]store.AgentRepoEnvironmentBuild, error)
	GetAgentRepoEnvironmentBuild(ctx context.Context, id string) (store.AgentRepoEnvironmentBuild, error)
	AgentRepoEnvironmentBuilds(ctx context.Context, workspaceID, repo string, limit int) ([]store.AgentRepoEnvironmentBuild, error)
	LatestAgentRepoEnvironmentBuilds(ctx context.Context, workspaceID string) ([]store.AgentRepoEnvironmentBuild, error)
	NewestAgentRepoEnvironmentBuildImage(ctx context.Context, workspaceID, repo, version, baseImage string) (store.AgentRepoEnvironmentBuildRef, error)
	RetainAgentRepoEnvironmentBuilds(ctx context.Context, workspaceID, repo string, keepImages, keepRecords int) error
	ClaimAgentRepoEnvironmentBuildPurges(ctx context.Context, p store.AgentRepoEnvironmentBuildPurge) ([]store.AgentRepoEnvironmentBuild, error)
	ClearAgentRepoEnvironmentBuildImage(ctx context.Context, id string) error
	AgentRepoEnvironmentRefreshes(ctx context.Context) ([]store.AgentRepoEnvironmentRefresh, error)
	SetAgentSessionEnvironmentBuild(ctx context.Context, sessionID, buildID string) error
}

// BuildSandboxes is the sandbox side of a build (*sandbox.AgentSessionLifecycle):
// the builder, its status, its commit into an image, its teardown, and the
// image's deletion.
type BuildSandboxes interface {
	CreateAgentBuildSandbox(ctx context.Context, workspaceID, buildID, repository string, egress sessionegress.Egress, driverEnv map[string]string) (sandbox.Sandbox, error)
	ReadAgentBuildStatus(ctx context.Context, workspaceID, buildID, sandboxID string) (string, error)
	CommitAgentBuildSandbox(ctx context.Context, workspaceID, buildID, sandboxID string) (sandbox.AgentImageCommit, error)
	TerminateAgentBuildSandbox(ctx context.Context, workspaceID, buildID, sandboxID string) error
	DeleteAgentBuildImage(ctx context.Context, workspaceID, buildID string) error
	AgentTemplateImage(template string) string
}

func errRepoEnvironmentBuildsUnavailable() error {
	return core.NewUnavailableError("AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE", "repository environment builds need the control-plane store and agent sandboxes, which this deployment does not configure", nil)
}

func errRepoEnvironmentBuildNotFound(id string) error {
	return core.NewNotFoundError("AGENT_REPO_ENVIRONMENT_BUILD_NOT_FOUND", "repository environment build not found", map[string]any{"id": id})
}

// buildsEnabled reports whether builds can run here.
func (s *Service) buildsEnabled() bool {
	return s.RepoEnvs != nil && s.RepoBuilds != nil && s.Builders != nil
}

func repoEnvironmentBuildView(row store.AgentRepoEnvironmentBuild, withSteps bool) RepoEnvironmentBuild {
	out := RepoEnvironmentBuild{ID: row.ID, OwnerID: row.WorkspaceID, Repo: row.Repo, Version: row.Version, Trigger: row.Trigger,
		Status: row.Status, FailureCode: row.FailureCode, FailureReason: row.FailureReason, HeadSHA: row.HeadSHA,
		SizeBytes: row.SizeBytes, ImageAvailable: row.HasImage(), CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt}
	if withSteps {
		out.Steps = []RepoEnvironmentBuildStep{}
		_ = json.Unmarshal(row.Steps, &out.Steps)
	}
	return out
}

// RepoEnvironmentBuilds lists a repository's builds, newest first (ADR111
// D10). Members who can start sessions read them, as they read environments.
func (s *Service) RepoEnvironmentBuilds(ctx context.Context, ownerID, repo string) (RepoEnvironmentBuildList, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanCreate, ownerID)
	if err != nil {
		return RepoEnvironmentBuildList{}, err
	}
	repo, err = normalizeEnvironmentRepo(repo)
	if err != nil {
		return RepoEnvironmentBuildList{}, err
	}
	if s.RepoBuilds == nil {
		return RepoEnvironmentBuildList{}, errRepoEnvironmentBuildsUnavailable()
	}
	rows, err := s.RepoBuilds.AgentRepoEnvironmentBuilds(ctx, workspaceID, repo, repoEnvironmentBuildKeepRecords)
	if err != nil {
		return RepoEnvironmentBuildList{}, core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
	}
	out := RepoEnvironmentBuildList{OwnerID: workspaceID, Repo: repo, Builds: make([]RepoEnvironmentBuild, 0, len(rows))}
	for _, row := range rows {
		out.Builds = append(out.Builds, repoEnvironmentBuildView(row, false))
	}
	return out, nil
}

// RepoEnvironmentBuild returns one build with its redacted log (ADR111 D10),
// routed by its own workspace when ownerId is omitted.
func (s *Service) RepoEnvironmentBuild(ctx context.Context, ownerID, buildID string) (RepoEnvironmentBuild, error) {
	var row store.AgentRepoEnvironmentBuild
	var owner core.ResourceOwner
	if s.RepoBuilds != nil && agentsession.IsBuildPrincipal(buildID) {
		owner = func(ctx context.Context) ([]string, error) {
			var err error
			row, err = s.RepoBuilds.GetAgentRepoEnvironmentBuild(ctx, buildID)
			return store.OwnerWorkspaces(row.WorkspaceID, err)
		}
	}
	ctx, err := s.ScopeByID(ctx, ownerID, owner)
	if err != nil {
		return RepoEnvironmentBuild{}, err
	}
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanCreate, ownerID)
	if err != nil {
		return RepoEnvironmentBuild{}, err
	}
	if s.RepoBuilds == nil {
		return RepoEnvironmentBuild{}, errRepoEnvironmentBuildsUnavailable()
	}
	if !agentsession.IsBuildPrincipal(buildID) {
		return RepoEnvironmentBuild{}, errRepoEnvironmentBuildNotFound(buildID)
	}
	if row.ID == "" {
		row, err = s.RepoBuilds.GetAgentRepoEnvironmentBuild(ctx, buildID)
	}
	if errors.Is(err, store.ErrNotFound) || (err == nil && row.WorkspaceID != workspaceID) {
		return RepoEnvironmentBuild{}, errRepoEnvironmentBuildNotFound(buildID)
	}
	if err != nil {
		return RepoEnvironmentBuild{}, core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
	}
	return repoEnvironmentBuildView(row, true), nil
}

// BuildRepoEnvironment queues a build of the repository's environment now
// (admins only): Devin's "Build snapshot". A build already queued is
// replaced; a running one finishes first.
func (s *Service) BuildRepoEnvironment(ctx context.Context, ownerID, repo string) (RepoEnvironmentBuild, error) {
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, core.RelCanManage, ownerID, agentsession.RepoEnvironmentBuildAuditTarget)
	if err != nil {
		return RepoEnvironmentBuild{}, err
	}
	repo, err = normalizeEnvironmentRepo(repo)
	if err != nil {
		return RepoEnvironmentBuild{}, err
	}
	if !s.buildsEnabled() {
		return RepoEnvironmentBuild{}, errRepoEnvironmentBuildsUnavailable()
	}
	row, err := s.queueBuild(ctx, workspaceID, repo, buildTriggerManual, false)
	var rate *store.AgentRepoEnvironmentBuildRateError
	switch {
	case errors.Is(err, store.ErrNotFound):
		return RepoEnvironmentBuild{}, errRepoEnvironmentNotFound(repo)
	case errors.Is(err, store.ErrAgentRepoEnvironmentNothingToBuild):
		return RepoEnvironmentBuild{}, core.NewBadRequestError("AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD",
			"this repository's environment has no setup steps, so there is nothing to build", map[string]any{"repo": repo})
	case errors.As(err, &rate):
		return RepoEnvironmentBuild{}, core.NewTooManyRequestsError(CodeRepoEnvironmentBuildRateLimited,
			fmt.Sprintf("this repository already started %d builds in the last 24 hours; the next may start at %s", rate.Limit, rate.NextAt.UTC().Format(time.RFC3339)),
			map[string]any{"repo": repo, "limit": rate.Limit, "nextBuildAt": rate.NextAt.UTC().Format(time.RFC3339)})
	case err != nil:
		return RepoEnvironmentBuild{}, core.HideCause(core.NewAgentSessionDependencyUnavailableError(), err)
	}
	return repoEnvironmentBuildView(row, false), nil
}

// queueBuild queues a build of the repository's current environment under
// the repository's rate limit; ifIdle leaves a queued or running build alone.
func (s *Service) queueBuild(ctx context.Context, workspaceID, repo, trigger string, ifIdle bool) (store.AgentRepoEnvironmentBuild, error) {
	return s.RepoBuilds.QueueAgentRepoEnvironmentBuild(ctx, store.NewAgentRepoEnvironmentBuild{
		WorkspaceID: workspaceID, Repo: repo, Trigger: trigger, IfIdle: ifIdle,
		RateLimit: repoEnvironmentBuildRateLimit, RateWindow: repoEnvironmentBuildRateWindow,
	})
}

// triggerBuild queues a build for an event (an environment save, a secret
// change). It never fails the event: a repository with nothing to build is
// skipped, and a refused build is logged — the daily refresh rebuilds a
// version that never built.
func (s *Service) triggerBuild(ctx context.Context, workspaceID, repo, trigger string) {
	if !s.buildsEnabled() {
		return
	}
	_, err := s.queueBuild(ctx, workspaceID, repo, trigger, false)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrAgentRepoEnvironmentNothingToBuild) {
		log.Printf("agent repo environment builds: %s build not queued (workspace=%s repo=%s): %v", trigger, workspaceID, repo, err)
	}
}

// latestBuilds is each repository's newest build in the workspace, for the
// environments list. A read failure leaves the builds out rather than failing
// the list.
func (s *Service) latestBuilds(ctx context.Context, workspaceID string) map[string]RepoEnvironmentBuild {
	if s.RepoBuilds == nil {
		return nil
	}
	rows, err := s.RepoBuilds.LatestAgentRepoEnvironmentBuilds(ctx, workspaceID)
	if err != nil {
		log.Printf("agent repo environment builds: latest builds unreadable (workspace=%s): %v", workspaceID, err)
		return nil
	}
	out := make(map[string]RepoEnvironmentBuild, len(rows))
	for _, row := range rows {
		out[row.Repo] = repoEnvironmentBuildView(row, false)
	}
	return out
}

// prebuiltImage is the image a session's sandbox boots from (ADR111 D4): the
// newest successful build of the environment's current version, run on the
// agent image the session's template boots, whose image is not retired.
// Without one the session runs phase 1; so does a read failure, which is
// logged.
func (s *Service) prebuiltImage(ctx context.Context, env store.AgentRepoEnvironment, template string) sandbox.AgentImage {
	if s.RepoBuilds == nil || s.Builders == nil || env.SetupSteps == 0 {
		return sandbox.AgentImage{}
	}
	base := s.Builders.AgentTemplateImage(template)
	if base == "" {
		return sandbox.AgentImage{}
	}
	build, err := s.RepoBuilds.NewestAgentRepoEnvironmentBuildImage(ctx, env.WorkspaceID, env.Repo, env.Version, base)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("agent repo environment builds: build image unreadable, running setup (workspace=%s repo=%s): %v", env.WorkspaceID, env.Repo, err)
		}
		return sandbox.AgentImage{}
	}
	return sandbox.AgentImage{BuildID: build.ID, Digest: build.ImageDigest}
}

// recordEnvironmentBuild keeps the session view's environmentBuildId current.
// It is informational, so a failed write is logged and the dispatch goes on.
func (s *Service) recordEnvironmentBuild(ctx context.Context, record store.AgentSession, buildID string) {
	if s.RepoBuilds == nil || record.EnvironmentBuildID == buildID {
		return
	}
	if err := s.RepoBuilds.SetAgentSessionEnvironmentBuild(ctx, record.ID, buildID); err != nil {
		log.Printf("agent-session dispatch: environment build not recorded (session=%s): %v", record.ID, err)
	}
}
