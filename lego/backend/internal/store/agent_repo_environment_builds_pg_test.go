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

package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type buildFixture struct {
	t    *testing.T
	s    *PGStore
	pool *pgxpool.Pool
	ws   string
}

func newBuildFixture(t *testing.T, name string) *buildFixture {
	t.Helper()
	s, pool, tenant := agentSessionPGStore(t, name)
	// Claims and purges scan every workspace; rows a failed earlier test left
	// would take this test's.
	if _, err := pool.Exec(context.Background(), `UPDATE agent_repo_environment_builds SET status = 'cancelled', image_digest = NULL
		WHERE workspace_id <> $1 AND (status = 'queued' OR image_digest IS NOT NULL)`, tenant.ID); err != nil {
		t.Fatal(err)
	}
	return &buildFixture{t: t, s: s, pool: pool, ws: tenant.ID}
}

func (f *buildFixture) env(repo, setup string) (AgentRepoEnvironment, bool) {
	f.t.Helper()
	row, changed, err := f.s.UpsertAgentRepoEnvironment(context.Background(), f.ws, repo, []byte(setup), []byte(`[]`), 50)
	if err != nil {
		f.t.Fatal(err)
	}
	return row, changed
}

func (f *buildFixture) queue(repo string) AgentRepoEnvironmentBuild {
	f.t.Helper()
	b, err := f.s.QueueAgentRepoEnvironmentBuild(context.Background(), NewAgentRepoEnvironmentBuild{WorkspaceID: f.ws, Repo: repo, Trigger: "manual"})
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}

func (f *buildFixture) claim(limits AgentRepoEnvironmentBuildLimits) (AgentRepoEnvironmentBuild, bool) {
	f.t.Helper()
	claimed, err := f.s.ClaimAgentRepoEnvironmentBuild(context.Background(), limits)
	if errors.Is(err, ErrNotFound) {
		return AgentRepoEnvironmentBuild{}, false
	}
	if err != nil {
		f.t.Fatal(err)
	}
	if claimed.Build.WorkspaceID != f.ws {
		f.t.Fatalf("claimed another workspace's build %+v", claimed.Build)
	}
	return claimed.Build, true
}

func (f *buildFixture) get(id string) AgentRepoEnvironmentBuild {
	f.t.Helper()
	b, err := f.s.GetAgentRepoEnvironmentBuild(context.Background(), id)
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}

func (f *buildFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *buildFixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

const testImageDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

// succeed runs a build from queue to success with an image.
func (f *buildFixture) succeed(repo string) AgentRepoEnvironmentBuild {
	f.t.Helper()
	ctx := context.Background()
	b := f.queue(repo)
	if claimed, ok := f.claim(AgentRepoEnvironmentBuildLimits{}); !ok || claimed.ID != b.ID {
		f.t.Fatalf("claim = %+v %v, want %s", claimed, ok, b.ID)
	}
	if ok, err := f.s.SetAgentRepoEnvironmentBuilder(ctx, b.ID, "sb-"+b.ID, "agent:1"); err != nil || !ok {
		f.t.Fatalf("set builder = %v %v", ok, err)
	}
	if ok, err := f.s.SucceedAgentRepoEnvironmentBuild(ctx, b.ID, AgentRepoEnvironmentBuildImage{Digest: testImageDigest, SizeBytes: 100}); err != nil || !ok {
		f.t.Fatalf("succeed = %v %v", ok, err)
	}
	return f.get(b.ID)
}

const buildSetup = `[{"name":"deps","run":"npm ci"}]`

// ADR111 D3, D7: the version hashes the setup steps' jsonb text, as the
// migration's backfill does; the upsert reports a change of it, and checks do
// not move it.
func TestPGAgentRepoEnvironmentVersion(t *testing.T) {
	f := newBuildFixture(t, "build-version")
	row, changed := f.env("acme/api", buildSetup)
	sum := sha256.Sum256([]byte(`[{"run": "npm ci", "name": "deps"}]`))
	if row.Version != hex.EncodeToString(sum[:]) || !changed || row.SetupSteps != 1 {
		t.Fatalf("version = %s changed %v steps %d, want the hash of the jsonb text on a new environment", row.Version, changed, row.SetupSteps)
	}
	if _, changed := f.env("acme/api", buildSetup); changed {
		t.Fatal("an identical save reported a change")
	}
	checked, changed, err := f.s.UpsertAgentRepoEnvironment(context.Background(), f.ws, "acme/api", []byte(buildSetup), []byte(`[{"name":"test","run":"npm test"}]`), 50)
	if err != nil || checked.Version != row.Version || changed {
		t.Fatalf("a checks-only edit moved the version: %s %v / %v", checked.Version, changed, err)
	}
	if edited, changed := f.env("acme/api", `[{"name":"deps","run":"yarn"}]`); edited.Version == row.Version || !changed {
		t.Fatal("a setup edit kept the version")
	}
}

// Two concurrent saves of the same new steps report the change once, so one
// build is queued for it.
func TestPGAgentRepoEnvironmentConcurrentSavesChangeOnce(t *testing.T) {
	f := newBuildFixture(t, "build-version-race")
	f.env("acme/api", buildSetup)
	var wg sync.WaitGroup
	var mu sync.Mutex
	changes := 0
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, changed, err := f.s.UpsertAgentRepoEnvironment(context.Background(), f.ws, "acme/api", []byte(`[{"name":"deps","run":"pnpm i"}]`), []byte(`[]`), 50)
			if err != nil {
				t.Error(err)
			}
			if changed {
				mu.Lock()
				changes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if changes != 1 {
		t.Fatalf("%d saves reported the change, want 1", changes)
	}
}

// ADR111 D1, D3: only an environment with setup steps builds; a newer trigger
// replaces a queued build; the refresh queues nothing over a busy repository.
func TestPGAgentRepoEnvironmentBuildQueue(t *testing.T) {
	f := newBuildFixture(t, "build-queue")
	ctx := context.Background()
	if _, err := f.s.QueueAgentRepoEnvironmentBuild(ctx, NewAgentRepoEnvironmentBuild{WorkspaceID: f.ws, Repo: "acme/none", Trigger: "manual"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("queue without an environment = %v", err)
	}
	f.env("acme/checks", `[]`)
	if _, err := f.s.QueueAgentRepoEnvironmentBuild(ctx, NewAgentRepoEnvironmentBuild{WorkspaceID: f.ws, Repo: "acme/checks", Trigger: "manual"}); !errors.Is(err, ErrAgentRepoEnvironmentNothingToBuild) {
		t.Fatalf("queue without setup steps = %v", err)
	}
	env, _ := f.env("acme/api", buildSetup)
	first := f.queue("acme/api")
	if first.Status != AgentRepoEnvironmentBuildQueued || first.Version != env.Version || first.Trigger != "manual" {
		t.Fatalf("queued = %+v", first)
	}
	second := f.queue("acme/api")
	if old := f.get(first.ID); old.Status != AgentRepoEnvironmentBuildCancelled || old.FailureReason == "" || old.FinishedAt == nil {
		t.Fatalf("the replaced build = %+v", old)
	}
	if _, err := f.s.QueueAgentRepoEnvironmentBuild(ctx, NewAgentRepoEnvironmentBuild{WorkspaceID: f.ws, Repo: "acme/api", Trigger: "refresh", IfIdle: true}); !errors.Is(err, ErrAgentRepoEnvironmentBuildBusy) {
		t.Fatalf("an if-idle queue over a queued build = %v", err)
	}
	if got := f.get(second.ID); got.Status != AgentRepoEnvironmentBuildQueued {
		t.Fatalf("the if-idle queue touched the queued build: %+v", got)
	}
	// A running build is never replaced: a new trigger queues behind it.
	if claimed, ok := f.claim(AgentRepoEnvironmentBuildLimits{}); !ok || claimed.ID != second.ID {
		t.Fatalf("claim = %+v", claimed)
	}
	third := f.queue("acme/api")
	if got := f.get(second.ID); got.Status != AgentRepoEnvironmentBuildBuilding {
		t.Fatalf("a trigger stopped the running build: %+v", got)
	}
	if _, ok := f.claim(AgentRepoEnvironmentBuildLimits{}); ok {
		t.Fatal("a second build of the repository started while one runs")
	}
	if got := f.get(third.ID); got.Status != AgentRepoEnvironmentBuildQueued {
		t.Fatalf("the build behind the running one = %+v", got)
	}
}

// ADR111 D3: at most 12 starts per repository in 24 h; the refusal names when
// the next may start.
func TestPGAgentRepoEnvironmentBuildRateLimit(t *testing.T) {
	f := newBuildFixture(t, "build-rate")
	ctx := context.Background()
	f.env("acme/api", buildSetup)
	base := time.Now().Add(-20 * time.Hour).UTC().Truncate(time.Second)
	for i := range 3 {
		b := f.queue("acme/api")
		f.exec(`UPDATE agent_repo_environment_builds SET status = 'failed', started_at = $2, finished_at = $2 WHERE id = $1`, b.ID, base.Add(time.Duration(i)*time.Hour))
	}
	in := NewAgentRepoEnvironmentBuild{WorkspaceID: f.ws, Repo: "acme/api", Trigger: "manual", RateLimit: 3, RateWindow: 24 * time.Hour}
	_, err := f.s.QueueAgentRepoEnvironmentBuild(ctx, in)
	var rate *AgentRepoEnvironmentBuildRateError
	if !errors.As(err, &rate) || rate.Limit != 3 || !rate.NextAt.Equal(base.Add(24*time.Hour)) {
		t.Fatalf("past the limit = %v (%+v)", err, rate)
	}
	// A start older than the window no longer counts.
	f.exec(`UPDATE agent_repo_environment_builds SET started_at = started_at - interval '5 hours' WHERE workspace_id = $1 AND started_at = $2`, f.ws, base)
	if _, err := f.s.QueueAgentRepoEnvironmentBuild(ctx, in); err != nil {
		t.Fatalf("under the limit = %v", err)
	}
}

// ADR111 D3: a claim starts at most one build per repository and two per
// workspace, never past the workspace's agent sandbox limit, which running
// builds and live sessions share; concurrent claims cannot pass the limits.
func TestPGAgentRepoEnvironmentBuildClaimLimits(t *testing.T) {
	f := newBuildFixture(t, "build-claim")
	ctx := context.Background()
	for _, repo := range []string{"acme/a", "acme/b", "acme/c"} {
		f.env(repo, buildSetup)
		f.queue(repo)
	}
	limits := AgentRepoEnvironmentBuildLimits{PerWorkspace: 2}
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.s.ClaimAgentRepoEnvironmentBuild(ctx, limits); err != nil && !errors.Is(err, ErrNotFound) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	building := `SELECT count(*) FROM agent_repo_environment_builds WHERE workspace_id = $1 AND status = 'building'`
	if n := f.count(building, f.ws); n > 2 {
		t.Fatalf("concurrent claims started %d builds, the workspace limit is 2", n)
	}
	// A try-lock miss leaves the rest for the next tick.
	for range 4 {
		f.claim(limits)
	}
	if n := f.count(building, f.ws); n != 2 {
		t.Fatalf("building = %d", n)
	}
	// The running builds hold two of the workspace's agent sandbox slots.
	live, err := f.s.CountActiveAgentSessions(ctx, f.ws, AgentSessionCount{WithSandbox: true})
	if err != nil || live != 2 {
		t.Fatalf("live sandboxes = %d / %v, want the two builders", live, err)
	}
	if all, err := f.s.CountActiveAgentSessions(ctx, f.ws, AgentSessionCount{}); err != nil || all != 0 {
		t.Fatalf("active sessions = %d / %v: builds count only in the live-sandbox set", all, err)
	}
	// With a free per-workspace slot but every sandbox slot held, the third waits.
	f.exec(`UPDATE agent_repo_environment_builds SET status = 'success', finished_at = now() WHERE workspace_id = $1 AND status = 'building' AND repo = (SELECT min(repo) FROM agent_repo_environment_builds WHERE workspace_id = $1 AND status = 'building')`, f.ws)
	session, err := f.s.CreateAgentSession(ctx, AgentSession{WorkspaceID: f.ws, Repo: "acme/a", AgentConfig: []byte(`{}`), InitialPrompt: "go"})
	if err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE agent_sessions SET phase = 'running', sandbox_id = 'sb-live' WHERE id = $1`, session.ID)
	if _, ok := f.claim(AgentRepoEnvironmentBuildLimits{PerWorkspace: 2, LiveSandboxes: 2}); ok {
		t.Fatal("a build started with every agent sandbox slot held")
	}
	if _, ok := f.claim(AgentRepoEnvironmentBuildLimits{PerWorkspace: 2, LiveSandboxes: 3}); !ok {
		t.Fatal("a build waited with a free agent sandbox slot")
	}
}

// A queued build whose environment lost its setup steps is cancelled by the
// claim, not started; the claim returns the environment it builds.
func TestPGAgentRepoEnvironmentBuildClaimReadsEnvironment(t *testing.T) {
	f := newBuildFixture(t, "build-claim-env")
	f.env("acme/api", buildSetup)
	stale := f.queue("acme/api")
	f.env("acme/api", `[]`)
	if _, ok := f.claim(AgentRepoEnvironmentBuildLimits{}); ok {
		t.Fatal("a build of an environment without setup steps started")
	}
	if got := f.get(stale.ID); got.Status != AgentRepoEnvironmentBuildCancelled {
		t.Fatalf("the stale build = %+v", got)
	}
	f.env("acme/api", buildSetup)
	b := f.queue("acme/api")
	changed, _ := f.env("acme/api", `[{"name":"deps","run":"pnpm i"}]`)
	claimed, err := f.s.ClaimAgentRepoEnvironmentBuild(context.Background(), AgentRepoEnvironmentBuildLimits{})
	if err != nil || claimed.Build.ID != b.ID || claimed.Build.Version != changed.Version || claimed.Environment.Version != changed.Version {
		t.Fatalf("claim = %+v / %v, want the current version %s", claimed, err, changed.Version)
	}
}

// Heartbeats and settles touch only a running build; the sweeper fails a
// silent one (internal_error) and one past the build limit (build_timeout);
// a failure releases the storage the build reserved.
func TestPGAgentRepoEnvironmentBuildSettleAndSweep(t *testing.T) {
	f := newBuildFixture(t, "build-sweep")
	ctx := context.Background()
	f.env("acme/a", buildSetup)
	f.env("acme/b", buildSetup)
	a := f.queue("acme/a")
	b := f.queue("acme/b")
	f.claim(AgentRepoEnvironmentBuildLimits{})
	f.claim(AgentRepoEnvironmentBuildLimits{})
	if ok, err := f.s.HeartbeatAgentRepoEnvironmentBuild(ctx, a.ID); err != nil || !ok {
		t.Fatalf("heartbeat = %v %v", ok, err)
	}
	f.exec(`UPDATE agent_repo_environment_builds SET heartbeat_at = now() - interval '10 minutes' WHERE id = $1`, a.ID)
	f.exec(`UPDATE agent_repo_environment_builds SET started_at = now() - interval '4 hours' WHERE id = $1`, b.ID)
	swept, err := f.s.FailStaleAgentRepoEnvironmentBuilds(ctx, 5*time.Minute, 3*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]string{}
	for _, s := range swept {
		if s.WorkspaceID == f.ws {
			codes[s.ID] = s.FailureCode
		}
	}
	if codes[a.ID] != AgentRepoEnvironmentBuildInternalError || codes[b.ID] != AgentRepoEnvironmentBuildTimeout {
		t.Fatalf("swept = %v", codes)
	}
	if ok, err := f.s.HeartbeatAgentRepoEnvironmentBuild(ctx, a.ID); err != nil || ok {
		t.Fatalf("a failed build's heartbeat = %v %v", ok, err)
	}
	if ok, err := f.s.SucceedAgentRepoEnvironmentBuild(ctx, a.ID, AgentRepoEnvironmentBuildImage{Digest: testImageDigest}); err != nil || ok {
		t.Fatalf("a failed build succeeded: %v %v", ok, err)
	}
	f.queue("acme/a")
	failing, _ := f.claim(AgentRepoEnvironmentBuildLimits{})
	if err := f.s.ReserveAgentRepoEnvironmentBuildImage(ctx, f.ws, failing.ID, 70, 1000); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.s.FailAgentRepoEnvironmentBuild(ctx, failing.ID, "setup_failed", "deps exited 1", []byte(`[{"name":"deps","exitCode":1}]`)); err != nil || !ok {
		t.Fatalf("fail = %v %v", ok, err)
	}
	if got := f.get(failing.ID); got.Status != AgentRepoEnvironmentBuildFailed || got.FailureCode != "setup_failed" || got.SizeBytes != nil || string(got.Steps) != `[{"name": "deps", "exitCode": 1}]` {
		t.Fatalf("failed = %+v %s", got, got.Steps)
	}
	if used, err := f.s.SandboxStorageBytes(ctx, f.ws); err != nil || used != 0 {
		t.Fatalf("storage after a failed build = %d / %v", used, err)
	}
}

// ADR111 D9: a build's image is reserved against the workspace's storage cap
// under the lock imports take, so two builds cannot both pass it, and the
// reservation counts while the build runs.
func TestPGAgentRepoEnvironmentBuildImageReservation(t *testing.T) {
	f := newBuildFixture(t, "build-reserve")
	ctx := context.Background()
	f.env("acme/a", buildSetup)
	f.env("acme/b", buildSetup)
	f.queue("acme/a")
	f.queue("acme/b")
	a, _ := f.claim(AgentRepoEnvironmentBuildLimits{})
	b, _ := f.claim(AgentRepoEnvironmentBuildLimits{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	refused := 0
	for _, id := range []string{a.ID, b.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := f.s.ReserveAgentRepoEnvironmentBuildImage(ctx, f.ws, id, 60, 100)
			var limit *SandboxStorageLimitError
			if errors.As(err, &limit) {
				mu.Lock()
				refused++
				mu.Unlock()
			} else if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if refused != 1 {
		t.Fatalf("%d of two 60-byte reservations under a 100-byte cap were refused, want 1", refused)
	}
	if used, err := f.s.SandboxStorageBytes(ctx, f.ws); err != nil || used != 60 {
		t.Fatalf("storage = %d / %v, want the one reservation", used, err)
	}
}

// ADR111 D4, D8: sessions boot from the newest success of the current version
// and agent image; each repository keeps its two newest images and 20
// records; a retired image leaves the registry only a day later, claimed by
// one replica and backed off after a failure; an environment delete cancels
// its builds and retires every image.
func TestPGAgentRepoEnvironmentBuildImagesAndRetention(t *testing.T) {
	f := newBuildFixture(t, "build-retention")
	ctx := context.Background()
	env, _ := f.env("acme/api", buildSetup)
	var builds []AgentRepoEnvironmentBuild
	for range 3 {
		builds = append(builds, f.succeed("acme/api"))
	}
	newest, err := f.s.NewestAgentRepoEnvironmentBuildImage(ctx, f.ws, "acme/api", env.Version, "agent:1")
	if err != nil || newest.ID != builds[2].ID || newest.ImageDigest != testImageDigest {
		t.Fatalf("newest image = %+v / %v", newest, err)
	}
	if _, err := f.s.NewestAgentRepoEnvironmentBuildImage(ctx, f.ws, "acme/api", env.Version, "agent:2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an image of an older agent = %v", err)
	}
	if _, err := f.s.NewestAgentRepoEnvironmentBuildImage(ctx, f.ws, "acme/api", "other", "agent:1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an image of another version = %v", err)
	}
	if used, err := f.s.SandboxStorageBytes(ctx, f.ws); err != nil || used != 300 {
		t.Fatalf("storage = %d / %v, want the three images' bytes", used, err)
	}
	if err := f.s.RetainAgentRepoEnvironmentBuilds(ctx, f.ws, "acme/api", 2, 20); err != nil {
		t.Fatal(err)
	}
	if got := f.get(builds[0].ID); got.ImageDeletedAt == nil || got.HasImage() || got.ImageDigest == "" {
		t.Fatalf("the retired image = %+v, want retired and still named", got)
	}
	purge := AgentRepoEnvironmentBuildPurge{Grace: 24 * time.Hour, Backoff: 10 * time.Minute, MaxBackoff: 24 * time.Hour, Limit: 10}
	ownPurges := func() []AgentRepoEnvironmentBuild {
		rows, err := f.s.ClaimAgentRepoEnvironmentBuildPurges(ctx, purge)
		if err != nil {
			t.Fatal(err)
		}
		var own []AgentRepoEnvironmentBuild
		for _, r := range rows {
			if r.WorkspaceID == f.ws {
				own = append(own, r)
			}
		}
		return own
	}
	if rows := ownPurges(); len(rows) != 0 {
		t.Fatalf("a just-retired image was claimed for deletion: %+v", rows)
	}
	f.exec(`UPDATE agent_repo_environment_builds SET image_deleted_at = now() - interval '25 hours' WHERE id = $1`, builds[0].ID)
	if rows := ownPurges(); len(rows) != 1 || rows[0].ID != builds[0].ID {
		t.Fatalf("purges past the grace = %+v", rows)
	}
	if rows := ownPurges(); len(rows) != 0 {
		t.Fatalf("a claimed purge was claimed again before its backoff: %+v", rows)
	}
	f.exec(`UPDATE agent_repo_environment_builds SET purge_after = now() - interval '1 second' WHERE id = $1`, builds[0].ID)
	if rows := ownPurges(); len(rows) != 1 {
		t.Fatalf("a failed purge was not retried after its backoff: %+v", rows)
	}
	var attempts int
	var wait time.Duration
	if err := f.pool.QueryRow(ctx, `SELECT purge_attempts, extract(epoch FROM purge_after - now())::bigint * 1000000000
		FROM agent_repo_environment_builds WHERE id = $1`, builds[0].ID).Scan(&attempts, &wait); err != nil || attempts != 2 || wait < 19*time.Minute {
		t.Fatalf("after two attempts: attempts %d, next in %s / %v, want a doubled backoff", attempts, wait, err)
	}
	if err := f.s.ClearAgentRepoEnvironmentBuildImage(ctx, builds[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := f.get(builds[0].ID); got.ImageDigest != "" {
		t.Fatalf("the cleared image = %+v", got)
	}
	if err := f.s.ClearAgentRepoEnvironmentBuildImage(ctx, builds[2].ID); err != nil || !f.get(builds[2].ID).HasImage() {
		t.Fatal("a kept image was cleared without its retirement")
	}
	if ids, err := f.s.AgentRepoEnvironmentBuildImageIDs(ctx, f.ws); err != nil || len(ids) != 2 {
		t.Fatalf("workspace images = %v / %v", ids, err)
	}

	// Records: the 20 newest stay, and an older one with an image stays too.
	for range 20 {
		b := f.queue("acme/api")
		f.exec(`UPDATE agent_repo_environment_builds SET status = 'failed', finished_at = now() WHERE id = $1`, b.ID)
	}
	f.exec(`UPDATE agent_repo_environment_builds SET created_at = created_at - interval '1 day' WHERE id = ANY($1)`, []string{builds[0].ID, builds[1].ID, builds[2].ID})
	if err := f.s.RetainAgentRepoEnvironmentBuilds(ctx, f.ws, "acme/api", 2, 20); err != nil {
		t.Fatal(err)
	}
	list, err := f.s.AgentRepoEnvironmentBuilds(ctx, f.ws, "acme/api", 100)
	if err != nil || len(list) != 22 || list[0].Steps != nil {
		t.Fatalf("records = %d / %v, want 20 plus the two with images, without their logs", len(list), err)
	}
	latest, err := f.s.LatestAgentRepoEnvironmentBuilds(ctx, f.ws)
	if err != nil || len(latest) != 1 || latest[0].ID != list[0].ID {
		t.Fatalf("latest = %+v / %v", latest, err)
	}

	queued := f.queue("acme/api")
	if err := f.s.DeleteAgentRepoEnvironment(ctx, f.ws, "acme/api"); err != nil {
		t.Fatal(err)
	}
	if got := f.get(queued.ID); got.Status != AgentRepoEnvironmentBuildCancelled {
		t.Fatalf("the deleted environment's queued build = %+v", got)
	}
	if n := f.count(`SELECT count(*) FROM agent_repo_environment_builds WHERE workspace_id = $1 AND image_deleted_at IS NULL AND image_digest IS NOT NULL`, f.ws); n != 0 {
		t.Fatalf("%d images still live after the environment delete", n)
	}
	if _, err := f.s.NewestAgentRepoEnvironmentBuildImage(ctx, f.ws, "acme/api", env.Version, "agent:1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a session would boot a deleted environment's image: %v", err)
	}
}

// The daily refresh considers environments with setup steps and no build
// queued or running, with their newest build's status, version, agent image
// and times.
func TestPGAgentRepoEnvironmentRefreshes(t *testing.T) {
	f := newBuildFixture(t, "build-refresh")
	ctx := context.Background()
	f.env("acme/new", buildSetup)
	f.env("acme/checks", `[]`)
	f.env("acme/done", buildSetup)
	done := f.succeed("acme/done")
	f.env("acme/busy", buildSetup)
	f.queue("acme/busy")
	rows, err := f.s.AgentRepoEnvironmentRefreshes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*AgentRepoEnvironmentBuild{}
	for _, r := range rows {
		if r.WorkspaceID == f.ws {
			got[r.Repo] = r.Newest
		}
	}
	newest := got["acme/done"]
	if len(got) != 2 || got["acme/new"] != nil || newest == nil || newest.Status != AgentRepoEnvironmentBuildSuccess ||
		newest.Version != done.Version || newest.BaseImage != "agent:1" || !newest.CreatedAt.Equal(done.CreatedAt) || newest.FinishedAt == nil {
		t.Fatalf("refreshes = %+v (newest %+v)", got, newest)
	}
}

// ADR111 D4: a session records the build its dispatch booted from; the
// gateway reads a build's binding.
func TestPGAgentSessionEnvironmentBuildAndBinding(t *testing.T) {
	f := newBuildFixture(t, "build-session")
	ctx := context.Background()
	session, err := f.s.CreateAgentSession(ctx, AgentSession{WorkspaceID: f.ws, Repo: "acme/api", AgentConfig: []byte(`{}`), InitialPrompt: "go"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetAgentSessionEnvironmentBuild(ctx, session.ID, "arb-0123456789abcdefghij"); err != nil {
		t.Fatal(err)
	}
	got, err := f.s.GetAgentSession(ctx, session.ID)
	if err != nil || got.EnvironmentBuildID != "arb-0123456789abcdefghij" {
		t.Fatalf("session = %q / %v", got.EnvironmentBuildID, err)
	}
	f.env("acme/api", buildSetup)
	built := f.succeed("acme/api")
	binding, err := f.s.GetAgentRepoEnvironmentBuildBinding(ctx, built.ID)
	if err != nil || binding != (AgentRepoEnvironmentBuildBinding{ID: built.ID, WorkspaceID: f.ws, Repo: "acme/api", Status: AgentRepoEnvironmentBuildSuccess, SandboxID: "sb-" + built.ID}) {
		t.Fatalf("binding = %+v / %v", binding, err)
	}
	if _, err := f.s.GetAgentRepoEnvironmentBuildBinding(ctx, "arb-0000000000000000000a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a missing build's binding = %v", err)
	}
}
