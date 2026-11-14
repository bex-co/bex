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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR111 against the real control-plane Postgres: the builds table, the claim
// and the triggers are the real ones; the builder sandbox and its commit are
// stand-ins behind the same seams.

type fakeBuilder struct {
	buildID, repo string
	egress        sessionegress.Egress
	env           map[string]string
}

// fakeBuilders is the sandbox side of builds. status is every builder's
// status file; onRead runs before each read.
type fakeBuilders struct {
	mu         sync.Mutex
	base       string
	created    []fakeBuilder
	createErr  error
	status     string
	statusErr  error
	onRead     func()
	reads      int
	commitErr  error
	committed  []string
	terminated []string
	deleted    []string
}

func (f *fakeBuilders) CreateAgentBuildSandbox(_ context.Context, _, buildID, repo string, egress sessionegress.Egress, env map[string]string) (sandbox.Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return sandbox.Sandbox{}, f.createErr
	}
	f.created = append(f.created, fakeBuilder{buildID: buildID, repo: repo, egress: egress, env: env})
	return sandbox.Sandbox{ID: "builder-" + buildID, Status: sandbox.StatusRunning}, nil
}

func (f *fakeBuilders) ReadAgentBuildStatus(context.Context, string, string, string) (string, error) {
	if f.onRead != nil {
		f.onRead()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.status, f.statusErr
}

func (f *fakeBuilders) CommitAgentBuildSandbox(_ context.Context, _, buildID, _ string) (sandbox.AgentImageCommit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.commitErr != nil {
		return sandbox.AgentImageCommit{}, f.commitErr
	}
	f.committed = append(f.committed, buildID)
	return sandbox.AgentImageCommit{Digest: testImageDigest, SizeBytes: 4096}, nil
}

func (f *fakeBuilders) TerminateAgentBuildSandbox(_ context.Context, _, buildID, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.terminated = append(f.terminated, buildID)
	return nil
}

func (f *fakeBuilders) DeleteAgentBuildImage(_ context.Context, _, buildID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, buildID)
	return nil
}

func (f *fakeBuilders) AgentTemplateImage(string) string { return f.base }

func (f *fakeBuilders) last() fakeBuilder {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.created[len(f.created)-1]
}

func (f *fakeBuilders) terminations(buildID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, id := range f.terminated {
		if id == buildID {
			n++
		}
	}
	return n
}

const testImageDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000001"

const succeededBuild = `{"state":"succeeded","build":{"head":"0123456789abcdef0123456789abcdef01234567","steps":[{"name":"deps","exitCode":0,"durationMs":1200,"output":"added 12 packages"}]}}`

type buildEnv struct {
	*schedEnv
	lc       *fakeLifecycle
	builders *fakeBuilders
}

func newBuildEnv(t *testing.T, name string) *buildEnv {
	t.Helper()
	e := newSchedEnv(t, name)
	ctx := context.Background()
	// The claim and the sweeper see every workspace; another test's leftovers
	// would take this one's claims.
	if _, err := e.pool.Exec(ctx, `UPDATE agent_repo_environment_builds SET status = 'cancelled', finished_at = now()
		WHERE status IN ('queued', 'building') AND workspace_id <> $1`, e.ws); err != nil {
		t.Fatal(err)
	}
	e.clock.t = time.Now().UTC()
	lc := &fakeLifecycle{}
	builders := &fakeBuilders{base: "agent:1", status: succeededBuild}
	e.svc.Sandbox = lc
	e.svc.RepoEnvs, e.svc.RepoBuilds, e.svc.Builders, e.svc.SecretScopes = e.pg, e.pg, builders, e.pg
	e.svc.dispatchRunner = func(fn func()) { fn() }
	return &buildEnv{schedEnv: e, lc: lc, builders: builders}
}

func (e *buildEnv) builder() *RepoEnvironmentBuilder {
	return &RepoEnvironmentBuilder{Service: e.svc, runner: func(fn func()) { fn() }, PollInterval: time.Millisecond}
}

func (e *buildEnv) runOnce() {
	e.t.Helper()
	if err := e.builder().RunOnce(context.Background()); err != nil {
		e.t.Fatalf("builder tick: %v", err)
	}
}

func (e *buildEnv) setEnvironment(repo string, setup, checks []RepoStep) RepoEnvironment {
	e.t.Helper()
	env, err := e.svc.SetRepoEnvironment(caller("root"), e.ws, repo, RepoEnvironmentInput{Setup: setup, Checks: checks})
	if err != nil {
		e.t.Fatal(err)
	}
	return env
}

func (e *buildEnv) builds(repo string) []store.AgentRepoEnvironmentBuild {
	e.t.Helper()
	rows, err := e.pg.AgentRepoEnvironmentBuilds(context.Background(), e.ws, repo, 100)
	if err != nil {
		e.t.Fatal(err)
	}
	return rows
}

func (e *buildEnv) build(id string) store.AgentRepoEnvironmentBuild {
	e.t.Helper()
	row, err := e.pg.GetAgentRepoEnvironmentBuild(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return row
}

func (e *buildEnv) startSession(repo string) View {
	e.t.Helper()
	view, err := e.svc.Create(caller("alice"), CreateRequest{OwnerID: e.ws, Repo: repo, AgentConfig: AgentConfig{Agent: "codex", Model: "gpt-5", ModelEndpoint: "https://api.openai.com/v1", Task: "fix the tests"}})
	if err != nil {
		e.t.Fatal(err)
	}
	got, err := e.svc.Get(caller("alice"), view.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	return got
}

var depsStep = []RepoStep{{Name: "deps", Run: "npm ci"}}

// ADR111 D1, D3, D4, D6, D7: a save that changes the setup steps queues a
// build; the builder runs the driver in build mode with the repository's
// unnamed secrets and a build-principal clone URL; its success commits an
// image, and the repository's next session boots from it until the steps
// change.
func TestRepoEnvironmentBuildToPrebuiltSession(t *testing.T) {
	e := newBuildEnv(t, "build-flow")
	ctx := caller("root")
	for _, secret := range []struct {
		name  string
		scope string
		repo  string
	}{{"WS_TOKEN", store.AgentSecretScopeWorkspace, ""}, {"NPM_TOKEN", store.AgentSecretScopeRepo, "acme/api"}, {"OTHER_TOKEN", store.AgentSecretScopeRepo, "acme/other"}, {"NAMED_TOKEN", store.AgentSecretScopeExplicit, ""}} {
		scope := secret.scope
		if _, err := e.svc.SetAgentSecret(ctx, e.ws, secret.name, AgentSecretInput{Value: "value-of-" + secret.name, Scope: &scope, Repo: secret.repo}); err != nil {
			t.Fatal(err)
		}
	}
	env := e.setEnvironment("acme/api", depsStep, []RepoStep{{Name: "test", Run: "npm test"}})
	builds := e.builds("acme/api")
	if len(builds) != 1 || builds[0].Status != store.AgentRepoEnvironmentBuildQueued || builds[0].Trigger != buildTriggerEnvironmentSaved || builds[0].Version != env.Version {
		t.Fatalf("builds after the save = %+v", builds)
	}
	if env.LatestBuild == nil || env.LatestBuild.ID != builds[0].ID {
		t.Fatalf("the saved environment's latestBuild = %+v", env.LatestBuild)
	}

	e.runOnce()
	got := e.build(builds[0].ID)
	if got.Status != store.AgentRepoEnvironmentBuildSuccess || got.ImageDigest != testImageDigest || got.HeadSHA != "0123456789abcdef0123456789abcdef01234567" ||
		got.SandboxID != "builder-"+got.ID || got.BaseImage != "agent:1" || *got.SizeBytes != 4096 {
		t.Fatalf("the built build = %+v", got)
	}
	var steps []RepoEnvironmentBuildStep
	if err := json.Unmarshal(got.Steps, &steps); err != nil || len(steps) != 1 || steps[0].Name != "deps" || *steps[0].ExitCode != 0 || steps[0].Output != "added 12 packages" {
		t.Fatalf("steps = %s / %v", got.Steps, err)
	}
	if n := e.builders.terminations(got.ID); n != 1 {
		t.Fatalf("the builder was terminated %d times, want once", n)
	}
	builder := e.builders.last()
	wantURL, _ := agentsession.ProxyRepositoryURL(defaultCredentialURL, store.SandboxNamespace(e.ws), got.ID, "acme/api", agentsession.BuildBranch(got.ID))
	if builder.env[buildModeEnv] != "build" || builder.env["BEX_AGENT_REPOSITORY"] != "acme/api" || builder.env["BEX_AGENT_REPO_URL"] != wantURL ||
		builder.env[repoEnvironmentEnv] == "" || builder.env["BEX_AGENT_NETWORK_TIER"] != NetworkTierRegistries {
		t.Fatalf("builder env = %v", builder.env)
	}
	if builder.env["WS_TOKEN"] != "value-of-WS_TOKEN" || builder.env["NPM_TOKEN"] != "value-of-NPM_TOKEN" || builder.env["OTHER_TOKEN"] != "" || builder.env["NAMED_TOKEN"] != "" {
		t.Fatalf("builder secrets = %v, want the workspace and this repository's secrets only", builder.env)
	}
	if names := builder.env[secretNamesEnv]; !strings.Contains(names, "WS_TOKEN") || !strings.Contains(names, "NPM_TOKEN") || strings.Contains(names, "NAMED") {
		t.Fatalf("scrub list = %q", names)
	}
	for _, key := range []string{"BEX_AGENT_PROMPT", "BEX_AGENT_MODEL_PROXY_URL", "BEX_AGENT_ARGS", sandbox.ModelAPIKeyEnvVar} {
		if _, ok := builder.env[key]; ok {
			t.Fatalf("builder env carries %s", key)
		}
	}
	if builder.repo != "acme/api" || builder.buildID != got.ID {
		t.Fatalf("builder = %+v", builder)
	}

	session := e.startSession("acme/api")
	if e.lc.image != (sandbox.AgentImage{BuildID: got.ID, Digest: testImageDigest}) || e.lc.driverEnv[prebuiltEnv] != got.ID || e.lc.driverEnv[repoEnvironmentEnv] == "" {
		t.Fatalf("session dispatch = image %+v env %v", e.lc.image, e.lc.driverEnv)
	}
	if session.EnvironmentBuildID != got.ID {
		t.Fatalf("session environmentBuildId = %q", session.EnvironmentBuildID)
	}

	// A checks-only edit keeps the version: no build, sessions keep the image.
	e.setEnvironment("acme/api", depsStep, []RepoStep{{Name: "lint", Run: "npm run lint"}})
	if n := len(e.builds("acme/api")); n != 1 {
		t.Fatalf("a checks-only edit queued a build (%d builds)", n)
	}
	// A setup edit makes the build stale: phase 1 until the new build succeeds.
	e.setEnvironment("acme/api", []RepoStep{{Name: "deps", Run: "npm install"}}, nil)
	if builds := e.builds("acme/api"); len(builds) != 2 || builds[0].Status != store.AgentRepoEnvironmentBuildQueued {
		t.Fatalf("builds after a setup edit = %+v", builds)
	}
	session = e.startSession("acme/api")
	if e.lc.image != (sandbox.AgentImage{}) || e.lc.driverEnv[prebuiltEnv] != "" || session.EnvironmentBuildID != "" {
		t.Fatalf("a stale build was used: image %+v env %v view %q", e.lc.image, e.lc.driverEnv[prebuiltEnv], session.EnvironmentBuildID)
	}
	// A build run on another agent image is not used either.
	e.runOnce()
	e.builders.base = "agent:2"
	e.startSession("acme/api")
	if e.lc.image != (sandbox.AgentImage{}) {
		t.Fatalf("a build of an older agent image was used: %+v", e.lc.image)
	}
}

// ADR111 D2, D5: the driver's failure codes, a code it may not report, a lost
// builder, a status unreadable past the retry window, the build limit only
// bex-api enforces, a failed start and both commit failures each fail the
// build with their code, and every builder that started is terminated.
func TestRepoEnvironmentBuildFailures(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		readErr  error
		create   error
		commit   error
		code     string
		reason   string
		withStep bool
	}{
		{name: "step failed", status: `{"state":"failed","code":"setup_failed","error":"setup step \"deps\" exited with code 1","build":{"steps":[{"name":"deps","exitCode":1,"durationMs":5,"output":"npm ERR!"}]}}`, code: buildFailedSetup, reason: `setup step "deps" exited with code 1`, withStep: true},
		{name: "step timeout", status: `{"state":"failed","code":"step_timeout","error":"deps passed 3h","build":{"steps":[{"name":"deps","exitCode":null,"durationMs":5,"output":""}]}}`, code: buildFailedStepTimeout, withStep: true},
		{name: "checkout", status: `{"state":"failed","code":"checkout_failed","error":"could not clone","build":{"steps":[]}}`, code: buildFailedCheckout},
		{name: "driver internal", status: `{"state":"failed","code":"internal_error","error":"scrub failed","build":{"steps":[]}}`, code: buildFailedInternalError},
		{name: "a build timeout from the driver", status: `{"state":"failed","code":"build_timeout","error":"lies","build":{"steps":[]}}`, code: buildFailedInternalError},
		{name: "unknown code", status: `{"state":"failed","code":"storage_limit_reached","error":"lies","build":{"steps":[]}}`, code: buildFailedInternalError},
		{name: "builder lost", readErr: fmt.Errorf("%w: gone", core.ErrNotFound), code: buildFailedInternalError, reason: "the builder ended before the build finished"},
		{name: "status unreadable", readErr: errors.New("gateway restarting"), code: buildFailedInternalError, reason: "the builder's status could not be read"},
		{name: "build limit", code: buildFailedTimeout, reason: "the build did not finish within"},
		{name: "builder start", create: errors.New("pod schedule timeout"), code: buildFailedInternalError, reason: "the builder sandbox could not start"},
		{name: "storage limit", status: succeededBuild, commit: fmt.Errorf("%w: too big", sandbox.ErrAgentImageStorageLimit), code: buildFailedStorageLimit, withStep: true},
		{name: "commit", status: succeededBuild, commit: errors.New("registry down"), code: buildFailedInternalError, reason: "the build's image could not be saved", withStep: true},
	}
	e := newBuildEnv(t, "build-fail")
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := fmt.Sprintf("acme/fail-%d", i)
			e.builders.status, e.builders.statusErr = tc.status, tc.readErr
			e.builders.createErr, e.builders.commitErr = tc.create, tc.commit
			// Each status read moves the clock 2 minutes: reads failing for the
			// configured 3-minute retry window (not the 10-minute default)
			// fail the build on the third.
			e.builders.onRead = func() { e.clock.Advance(2 * time.Minute) }
			e.builders.reads = 0
			e.setEnvironment(repo, depsStep, nil)
			b := e.builder()
			b.StatusReadFailureTTL = 3 * time.Minute
			b.BuildTimeout = 50 * time.Millisecond
			if err := b.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if tc.name == "status unreadable" && e.builders.reads != 3 {
				t.Fatalf("the build failed after %d unreadable reads, want 3 under the 3-minute window", e.builders.reads)
			}
			got := e.builds(repo)[0]
			if got.Status != store.AgentRepoEnvironmentBuildFailed || got.FailureCode != tc.code || !strings.HasPrefix(got.FailureReason, tc.reason) || got.ImageDigest != "" {
				t.Fatalf("build = %+v, want failed %s %q", got, tc.code, tc.reason)
			}
			if tc.withStep && string(e.build(got.ID).Steps) == "[]" {
				t.Fatalf("the failed build kept no log")
			}
			if wantTerminated := tc.create == nil; (e.builders.terminations(got.ID) == 1) != wantTerminated {
				t.Fatalf("terminations = %d", e.builders.terminations(got.ID))
			}
		})
	}
}

// ADR111 D3: Build now is admins' and rate-limited per repository; a
// repository secret set or changed rebuilds, a workspace secret does not; an
// environment without setup steps has nothing to build; the daily refresh
// queues a day-old repository's build.
func TestRepoEnvironmentBuildTriggers(t *testing.T) {
	e := newBuildEnv(t, "build-triggers")
	root := caller("root")
	if _, err := e.svc.BuildRepoEnvironment(root, e.ws, "acme/none"); !isCode(err, "AGENT_REPO_ENVIRONMENT_NOT_FOUND") {
		t.Fatalf("build of no environment = %v", err)
	}
	e.setEnvironment("acme/checks", nil, []RepoStep{{Name: "test", Run: "go test ./..."}})
	if _, err := e.svc.BuildRepoEnvironment(root, e.ws, "acme/checks"); !isCode(err, "AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD") {
		t.Fatalf("build of a checks-only environment = %v", err)
	}
	if n := len(e.builds("acme/checks")); n != 0 {
		t.Fatalf("a checks-only environment queued %d builds", n)
	}
	saved := e.setEnvironment("acme/api", depsStep, nil).LatestBuild
	if _, err := e.svc.BuildRepoEnvironment(caller("alice"), e.ws, "acme/api"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a developer's Build now = %v", err)
	}
	manual, err := e.svc.BuildRepoEnvironment(root, e.ws, "Acme/API")
	if err != nil || manual.Trigger != buildTriggerManual || manual.Status != store.AgentRepoEnvironmentBuildQueued || manual.Repo != "acme/api" {
		t.Fatalf("Build now = %+v / %v", manual, err)
	}
	if replaced := e.build(saved.ID); replaced.Status != store.AgentRepoEnvironmentBuildCancelled {
		t.Fatalf("the save's queued build after Build now = %+v", replaced)
	}
	workspace, repoScope := store.AgentSecretScopeWorkspace, store.AgentSecretScopeRepo
	if _, err := e.svc.SetAgentSecret(root, e.ws, "WS_TOKEN", AgentSecretInput{Value: "w", Scope: &workspace}); err != nil {
		t.Fatal(err)
	}
	if latest := e.builds("acme/api")[0]; latest.ID != manual.ID {
		t.Fatalf("a workspace secret queued %+v", latest)
	}
	if _, err := e.svc.SetAgentSecret(root, e.ws, "NPM_TOKEN", AgentSecretInput{Value: "n", Scope: &repoScope, Repo: "acme/api"}); err != nil {
		t.Fatal(err)
	}
	if latest := e.builds("acme/api")[0]; latest.Trigger != buildTriggerSecretChanged || latest.Status != store.AgentRepoEnvironmentBuildQueued {
		t.Fatalf("a repository secret's build = %+v", latest)
	}

	// Twelve starts in 24 hours; the thirteenth is refused with its time.
	if _, err := e.pool.Exec(context.Background(), `UPDATE agent_repo_environment_builds SET status = 'failed', started_at = now() - interval '1 hour', finished_at = now()
		WHERE workspace_id = $1 AND repo = 'acme/api'`, e.ws); err != nil {
		t.Fatal(err)
	}
	for range repoEnvironmentBuildRateLimit - 3 {
		b, err := e.svc.BuildRepoEnvironment(root, e.ws, "acme/api")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.pool.Exec(context.Background(), `UPDATE agent_repo_environment_builds SET status = 'failed', started_at = now(), finished_at = now() WHERE id = $1`, b.ID); err != nil {
			t.Fatal(err)
		}
	}
	_, err = e.svc.BuildRepoEnvironment(root, e.ws, "acme/api")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != CodeRepoEnvironmentBuildRateLimited || coded.Params["nextBuildAt"] == nil || !errors.Is(err, core.ErrTooManyRequests) {
		t.Fatalf("the thirteenth build = %v", err)
	}

	// The daily refresh: a repository whose newest build is a day old.
	e.setEnvironment("acme/web", depsStep, nil)
	e.runOnce()
	web := e.builds("acme/web")
	if len(web) != 1 || web[0].Status != store.AgentRepoEnvironmentBuildSuccess {
		t.Fatalf("web builds = %+v", web)
	}
	if _, err := e.pool.Exec(context.Background(), `UPDATE agent_repo_environment_builds SET created_at = now() - interval '25 hours' WHERE id = $1`, web[0].ID); err != nil {
		t.Fatal(err)
	}
	b := e.builder()
	if err := b.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	web = e.builds("acme/web")
	if len(web) != 2 || web[0].Trigger != buildTriggerRefresh || web[0].Status != store.AgentRepoEnvironmentBuildSuccess {
		t.Fatalf("web builds after the refresh = %+v", web)
	}
	if err := b.RunOnce(context.Background()); err != nil || len(e.builds("acme/web")) != 2 {
		t.Fatalf("a fresh build was refreshed again: %v", err)
	}
}

// ADR111 D3: builds share the workspace's agent sandbox limit with sessions,
// in both directions.
func TestRepoEnvironmentBuildsShareTheSandboxLimit(t *testing.T) {
	e := newBuildEnv(t, "build-limit")
	e.svc.MaxLiveSandboxes = 1
	session := e.startSession("acme/web")
	e.setEnvironment("acme/api", depsStep, nil)
	e.runOnce()
	if got := e.builds("acme/api")[0]; got.Status != store.AgentRepoEnvironmentBuildQueued {
		t.Fatalf("a build started with every agent sandbox slot held: %+v", got)
	}
	if _, err := e.pool.Exec(context.Background(), `UPDATE agent_sessions SET phase = 'completed', sandbox_id = '' WHERE id = $1`, session.ID); err != nil {
		t.Fatal(err)
	}
	claimed, err := e.pg.ClaimAgentRepoEnvironmentBuild(context.Background(), store.AgentRepoEnvironmentBuildLimits{PerWorkspace: 2, LiveSandboxes: 1})
	if err != nil {
		t.Fatalf("claim with a free slot = %v", err)
	}
	if _, err := e.svc.Create(caller("alice"), CreateRequest{OwnerID: e.ws, Repo: "acme/web", AgentConfig: AgentConfig{Agent: "codex", Model: "gpt-5", ModelEndpoint: "https://api.openai.com/v1", Task: "go"}}); !isCode(err, "AGENT_SESSION_LIVE_LIMIT") {
		t.Fatalf("a session start while a build holds the slot = %v", err)
	}
	if claimed.Build.WorkspaceID != e.ws {
		t.Fatalf("claimed %+v", claimed.Build)
	}
}

// ADR111 D8, D11: deleting an environment cancels its builds and retires
// their images, which the worker deletes from the registry only a day later;
// the sweeper fails a build whose worker went silent and terminates its
// builder; the inventory reconcile keeps a running build's builder and reaps
// an ended one's.
func TestRepoEnvironmentBuildCleanup(t *testing.T) {
	e := newBuildEnv(t, "build-cleanup")
	e.setEnvironment("acme/api", depsStep, nil)
	e.runOnce()
	built := e.builds("acme/api")[0]
	c := &Completer{Store: e.pg, Sandbox: e.lc, Builds: e.pg}
	if claim, err := c.sandboxClaim(context.Background(), built.ID); err != nil || !claim.Found || !claim.Orphaned {
		t.Fatalf("a finished build's builder claim = %+v / %v, want an orphan", claim, err)
	}
	queued, err := e.svc.BuildRepoEnvironment(caller("root"), e.ws, "acme/api")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DeleteRepoEnvironment(caller("root"), e.ws, "acme/api"); err != nil {
		t.Fatal(err)
	}
	if got := e.build(queued.ID); got.Status != store.AgentRepoEnvironmentBuildCancelled {
		t.Fatalf("the deleted environment's queued build = %+v", got)
	}
	if got := e.build(built.ID); got.HasImage() || got.ImageDigest == "" || len(e.builders.deleted) != 0 {
		t.Fatalf("the deleted environment's image = %+v, deleted %v: retired, not yet deleted", got, e.builders.deleted)
	}
	e.runOnce()
	if len(e.builders.deleted) != 0 {
		t.Fatalf("an image was deleted inside its grace: %v", e.builders.deleted)
	}
	if _, err := e.pool.Exec(context.Background(), `UPDATE agent_repo_environment_builds SET image_deleted_at = now() - interval '25 hours' WHERE id = $1`, built.ID); err != nil {
		t.Fatal(err)
	}
	e.runOnce()
	if got := e.build(built.ID); got.ImageDigest != "" || len(e.builders.deleted) != 1 || e.builders.deleted[0] != built.ID {
		t.Fatalf("the image past its grace = %+v, deleted %v", got, e.builders.deleted)
	}

	e.setEnvironment("acme/web", depsStep, nil)
	claimed, err := e.pg.ClaimAgentRepoEnvironmentBuild(context.Background(), store.AgentRepoEnvironmentBuildLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.pg.SetAgentRepoEnvironmentBuilder(context.Background(), claimed.Build.ID, "builder-silent", "agent:1"); err != nil {
		t.Fatal(err)
	}
	if claim, err := c.sandboxClaim(context.Background(), claimed.Build.ID); err != nil || claim.Phase != PhaseRunning || claim.SandboxID != "builder-silent" || claim.Orphaned {
		t.Fatalf("a running build's builder claim = %+v / %v", claim, err)
	}
	if _, err := e.pool.Exec(context.Background(), `UPDATE agent_repo_environment_builds SET heartbeat_at = now() - interval '10 minutes' WHERE id = $1`, claimed.Build.ID); err != nil {
		t.Fatal(err)
	}
	e.runOnce()
	if got := e.build(claimed.Build.ID); got.Status != store.AgentRepoEnvironmentBuildFailed || got.FailureCode != buildFailedInternalError {
		t.Fatalf("the silent build = %+v", got)
	}
	if n := e.builders.terminations(claimed.Build.ID); n != 1 {
		t.Fatalf("the silent build's builder was terminated %d times", n)
	}
}

// ADR111 D10: builds on REST, GraphQL and MCP, read by members who can start
// sessions and started by admins.
func TestRepoEnvironmentBuildSurfaces(t *testing.T) {
	e := newBuildEnv(t, "build-surfaces")
	e.setEnvironment("acme/api", depsStep, nil)
	e.runOnce()
	built := e.builds("acme/api")[0]
	mux := http.NewServeMux()
	e.svc.RegisterREST(mux)
	serve := func(method, path, who string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil).WithContext(caller(who)))
		return rec
	}
	rec := serve(http.MethodGet, "/v1/agent-repo-environments/acme/api/builds?ownerId="+e.ws, "alice")
	var list RepoEnvironmentBuildList
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Builds) != 1 || list.Builds[0].ID != built.ID || list.Builds[0].Steps != nil || !list.Builds[0].ImageAvailable {
		t.Fatalf("REST list = %d %s", rec.Code, rec.Body.String())
	}
	rec = serve(http.MethodGet, "/v1/agent-repo-environment-builds/"+built.ID, "alice")
	var one RepoEnvironmentBuild
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &one) != nil || len(one.Steps) != 1 || one.Steps[0].Output != "added 12 packages" || one.HeadSHA == "" {
		t.Fatalf("REST get = %d %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodGet, "/v1/agent-repo-environment-builds/"+built.ID, "eve"); rec.Code != http.StatusForbidden {
		t.Fatalf("a viewer read a build: %d", rec.Code)
	}
	if rec := serve(http.MethodGet, "/v1/agent-repo-environment-builds/arb-0000000000000000000a?ownerId="+e.ws, "alice"); rec.Code != http.StatusNotFound {
		t.Fatalf("a missing build = %d", rec.Code)
	}
	if rec := serve(http.MethodPost, "/v1/agent-repo-environments/acme/api/builds?ownerId="+e.ws, "alice"); rec.Code != http.StatusForbidden {
		t.Fatalf("a developer's Build now = %d", rec.Code)
	}
	rec = serve(http.MethodPost, "/v1/agent-repo-environments/acme/api/builds?ownerId="+e.ws, "root")
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"status":"queued"`) {
		t.Fatalf("REST Build now = %d %s", rec.Code, rec.Body.String())
	}

	schema := environmentSchema(t, e.svc)
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: fmt.Sprintf(`{
		agentRepoEnvironments(ownerId: %q) { environments { repo version latestBuild { id status } } }
		agentRepoEnvironmentBuilds(ownerId: %q, repo: "acme/api") { builds { id status trigger imageAvailable sizeBytes steps { name } } }
		agentRepoEnvironmentBuild(id: %q) { id status headSha sizeBytes steps { name exitCode durationMs output } }
	}`, e.ws, e.ws, built.ID)})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL reads = %v", result.Errors)
	}
	data, _ := json.Marshal(result.Data)
	for _, want := range []string{`"latestBuild":{"id":"arb-`, `"status":"queued"`, `"sizeBytes":4096`, `"steps":[{"durationMs":1200,"exitCode":0,"name":"deps","output":"added 12 packages"}]`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("GraphQL data lacks %s: %s", want, data)
		}
	}
	result = graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: fmt.Sprintf(`mutation { buildAgentRepoEnvironment(ownerId: %q, repo: "acme/api") { id } }`, e.ws)})
	if len(result.Errors) == 0 {
		t.Fatal("a developer built over GraphQL")
	}
	result = graphql.Do(graphql.Params{Schema: schema, Context: caller("root"), RequestString: fmt.Sprintf(`mutation { buildAgentRepoEnvironment(ownerId: %q, repo: "acme/api") { status trigger } }`, e.ws)})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL Build now = %v", result.Errors)
	}

	client := newMCPClient(t, caller("root"), e.svc)
	for name, args := range map[string]map[string]any{
		"list_agent_repo_environment_builds": {"ownerId": e.ws, "repo": "acme/api"},
		"get_agent_repo_environment_build":   {"id": built.ID},
		"build_agent_repo_environment":       {"ownerId": e.ws, "repo": "acme/api"},
	} {
		res, err := client.CallTool(caller("root"), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("MCP %s = %+v / %v", name, res, err)
		}
	}
}
