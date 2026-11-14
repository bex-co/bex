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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const testBuildID = "arb-0123456789abcdefghij"

type fakeBuilds struct {
	build store.AgentRepoEnvironmentBuildBinding
}

func (f fakeBuilds) GetAgentRepoEnvironmentBuildBinding(_ context.Context, id string) (store.AgentRepoEnvironmentBuildBinding, error) {
	if f.build.ID != id {
		return store.AgentRepoEnvironmentBuildBinding{}, store.ErrNotFound
	}
	return f.build, nil
}

func runningBuild() store.AgentRepoEnvironmentBuildBinding {
	return store.AgentRepoEnvironmentBuildBinding{ID: testBuildID, WorkspaceID: "tea-a", Repo: "octo/repo", Status: store.AgentRepoEnvironmentBuildBuilding, SandboxID: "builder-1"}
}

func buildMintRequest() MintRequest {
	return MintRequest{SessionID: testBuildID, Workspace: "tea-a", Repository: "octo/repo", Branch: BuildBranch(testBuildID), PodName: "builder-1-0", PodUID: "uid-b"}
}

// ADR111: a build principal gets a contents-read token for its own
// repository, only while the build runs and only from its builder; never a
// session's write token.
func TestMinterBuildPrincipalIsReadOnlyAndBoundToItsBuilder(t *testing.T) {
	connections := fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "octo"}}
	gh := &fakeSessionGitHub{}
	audit := &auditRecorder{}
	m := &Minter{GitHub: gh, Connections: connections, Sessions: fakeSessions{}, Builds: fakeBuilds{build: runningBuild()}, Audit: audit}
	got, err := m.Mint(context.Background(), buildMintRequest())
	if err != nil || got.Token != "ghs_read" || gh.readCalls != 1 || gh.calls != 0 || gh.repository != "repo" {
		t.Fatalf("build mint = %+v / %v (read %d, write %d)", got, err, gh.readCalls, gh.calls)
	}
	if ev := audit.events[0]; ev.Target != RepoEnvironmentBuildAuditTarget+":"+testBuildID || ev.TargetName != "octo/repo" || ev.Outcome != core.AuditAllowed {
		t.Fatalf("audit = %+v", ev)
	}
	finished := runningBuild()
	finished.Status = store.AgentRepoEnvironmentBuildSuccess
	for name, tc := range map[string]struct {
		builds BuildStore
		mutate func(*MintRequest)
	}{
		"finished build":   {fakeBuilds{build: finished}, nil},
		"unknown build":    {fakeBuilds{}, nil},
		"no build store":   {nil, nil},
		"another pod":      {fakeBuilds{build: runningBuild()}, func(r *MintRequest) { r.PodName = "builder-2-0" }},
		"another repo":     {fakeBuilds{build: runningBuild()}, func(r *MintRequest) { r.Repository = "octo/other" }},
		"a session branch": {fakeBuilds{build: runningBuild()}, func(r *MintRequest) { r.Branch = "bex-agent/main" }},
		"another tenant":   {fakeBuilds{build: runningBuild()}, func(r *MintRequest) { r.Workspace = "tea-b" }},
	} {
		gh := &fakeSessionGitHub{}
		m := &Minter{GitHub: gh, Connections: connections, Sessions: fakeSessions{}, Builds: tc.builds, Audit: &auditRecorder{}}
		req := buildMintRequest()
		if tc.mutate != nil {
			tc.mutate(&req)
		}
		if _, err := m.Mint(context.Background(), req); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: mint = %v, want forbidden", name, err)
		}
		if gh.calls+gh.readCalls != 0 {
			t.Errorf("%s: a refused build mint reached GitHub", name)
		}
	}
}

// ADR111 D11: a running build's builder reaches the egress proxy in a
// full-tier workspace, as its setup steps would in a session; nothing else
// about a build does.
func TestEgressGrantBuildPrincipal(t *testing.T) {
	full := fakeEnvironments{row: store.AgentEnvironment{NetworkTier: sessionegress.NetworkTierFull}}
	req := ModelMintRequest{SessionID: testBuildID, Workspace: "tea-a", PodName: "builder-1-0", PodUID: "uid-b"}
	granter := &EgressGranter{Sessions: fakeSessions{}, Environments: full, Builds: fakeBuilds{build: runningBuild()}}
	if got, err := granter.Grant(context.Background(), req); err != nil || !got.Allowed || got.SessionID != testBuildID {
		t.Fatalf("grant = %+v / %v", got, err)
	}
	finished := runningBuild()
	finished.Status = store.AgentRepoEnvironmentBuildCancelled
	for name, g := range map[string]*EgressGranter{
		"finished build": {Sessions: fakeSessions{}, Environments: full, Builds: fakeBuilds{build: finished}},
		"no build store": {Sessions: fakeSessions{}, Environments: full},
		"registries":     {Sessions: fakeSessions{}, Environments: fakeEnvironments{row: store.AgentEnvironment{NetworkTier: "registries"}}, Builds: fakeBuilds{build: runningBuild()}},
	} {
		if _, err := g.Grant(context.Background(), req); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: grant = %v, want forbidden", name, err)
		}
	}
}

func TestBuildPrincipalAndBranch(t *testing.T) {
	if !IsBuildPrincipal(testBuildID) || IsBuildPrincipal("ags-0123456789abcdefghij") || IsBuildPrincipal("arb-short") {
		t.Fatal("IsBuildPrincipal misclassifies")
	}
	if err := ValidateBranch(BuildBranch(testBuildID)); err != nil {
		t.Fatalf("the build branch fails the branch gate: %v", err)
	}
}
