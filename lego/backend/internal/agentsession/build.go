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

package agentsession

import (
	"context"
	"errors"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A repository environment build (ADR111) stands in a session's place on the
// gateway: its builder sandbox carries the build id in the session label, and
// its Git proxy URL names the build id where a session's names the session.
// The build principal is read-only — a clone or fetch of the build's own
// repository, only while the build runs, only from its builder — and reaches
// nothing else a session does.

// IsBuildPrincipal reports whether a gateway principal id is a repository
// environment build's rather than a session's.
func IsBuildPrincipal(principal string) bool {
	kind, ok := id.KindOf(principal)
	return ok && kind == id.AgentRepoEnvironmentBuild
}

// BuildBranch is the branch a builder's binding labels and proxy URL carry.
// It satisfies ValidateBranch but is never pushed: a build may not push, and
// its clone checks out the repository's default branch.
func BuildBranch(buildID string) string { return BranchPrefix + "build-" + buildID }

// RepoEnvironmentBuildAuditTarget is the audit target of a repository
// environment build: Build now, and its principal's gateway verbs.
const RepoEnvironmentBuildAuditTarget = "agent-repo-environment-build"

// BuildStore reads a build's binding for the gateway verbs' build principal;
// *store.PGStore satisfies it.
type BuildStore interface {
	GetAgentRepoEnvironmentBuildBinding(ctx context.Context, id string) (store.AgentRepoEnvironmentBuildBinding, error)
}

// currentBuilder resolves a build principal's request to its build, or
// ErrForbidden unless the build is running and the request comes from its
// builder.
func currentBuilder(ctx context.Context, builds BuildStore, buildID, workspace, podName, podUID string) (store.AgentRepoEnvironmentBuildBinding, error) {
	if builds == nil {
		return store.AgentRepoEnvironmentBuildBinding{}, ErrForbidden
	}
	build, err := builds.GetAgentRepoEnvironmentBuildBinding(ctx, buildID)
	if errors.Is(err, store.ErrNotFound) {
		return store.AgentRepoEnvironmentBuildBinding{}, ErrForbidden
	}
	if err != nil {
		return store.AgentRepoEnvironmentBuildBinding{}, err
	}
	if build.Status != store.AgentRepoEnvironmentBuildBuilding || !boundPod(build.WorkspaceID, build.SandboxID, workspace, podName, podUID) {
		return store.AgentRepoEnvironmentBuildBinding{}, ErrForbidden
	}
	return build, nil
}
