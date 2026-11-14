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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The build worker (ADR111): a loop on every bex-api replica that starts
// queued builds in builder sandboxes and commits each finished one into an
// image. A claim starts one build under its workspace's advisory lock, and a
// build's worker owns it for as long as its heartbeat lands, so replicas never
// run one build twice; a build whose replica died is failed by the sweeper,
// and its builder is terminated (here, or by the Completer's inventory
// reconcile).
const (
	buildTick         = 15 * time.Second
	buildPollInterval = 30 * time.Second
	buildHeartbeat    = 30 * time.Second
	buildStaleAfter   = 5 * time.Minute
	// buildCommitAllowance is the sweeper's grace past the build limit: a
	// build that finished its steps may still be committing.
	buildCommitAllowance = 45 * time.Minute
	buildSettleTimeout   = 30 * time.Second
	// maxBuildsPerReplica builds run at once on one replica.
	maxBuildsPerReplica = 4
	// The daily refresh (ADR111 D3): each replica looks every
	// buildRefreshEvery for a repository whose newest build is a day old,
	// less a per-repository jitter so a workspace's refreshes spread out. A
	// build bex itself failed is retried sooner. A build of an older agent
	// image is rebuilt once it is buildImageSettle old, so the replicas of a
	// rolling deploy do not rebuild each other's builds, and in its
	// repository's slot of the spread, so a release does not rebuild every
	// repository at once.
	buildRefreshEvery            = 10 * time.Minute
	repoEnvironmentRefreshAge    = 24 * time.Hour
	repoEnvironmentRefreshSpread = 2 * time.Hour
	buildRefreshSlot             = 20 * time.Minute
	buildImageSettle             = time.Hour
	buildInternalRetryAfter      = 15 * time.Minute
	// Retired images leave the registry buildImageGrace after retirement, so
	// a session that booted one just before keeps it; one replica deletes
	// each, a failure backing off from buildPurgeBackoff.
	buildPurgeEvery      = 10 * time.Minute
	buildImageGrace      = 24 * time.Hour
	buildPurgeBackoff    = 10 * time.Minute
	buildPurgeMaxBackoff = 24 * time.Hour
	buildPurgeBatch      = 20
	// Bounds on the untrusted driver's report.
	maxBuildSteps       = 16
	maxBuildStepName    = 64
	maxBuildStepOutput  = 8 << 10
	maxBuildReasonRunes = 1000
)

// RepoEnvironmentBuilder runs repository environment builds. Run it on every
// replica.
type RepoEnvironmentBuilder struct {
	Service *Service
	// Interval is the tick (default 15 s); PollInterval how often a running
	// build's status is read (30 s); HeartbeatInterval how often its worker
	// says it is alive (30 s).
	Interval          time.Duration
	PollInterval      time.Duration
	HeartbeatInterval time.Duration
	// StatusReadFailureTTL is the Completer's retry window: how long a
	// builder's status may stay unreadable before its build fails.
	StatusReadFailureTTL time.Duration
	// BuildTimeout overrides the 3 h build limit (tests).
	BuildTimeout time.Duration
	// runner runs one claimed build; nil starts a goroutine. Tests run builds
	// inline.
	runner      func(func())
	mu          sync.Mutex
	running     int
	lastRefresh time.Time
	lastPurge   time.Time
}

// RepoEnvironmentBuildsRunnable reports whether this deployment can run
// builds: the tables and the sandbox side are wired.
func (s *Service) RepoEnvironmentBuildsRunnable() bool { return s != nil && s.buildsEnabled() }

// Run ticks until ctx ends.
func (w *RepoEnvironmentBuilder) Run(ctx context.Context) {
	core.Poll(ctx, "agent-repo-environment-builds", cmp.Or(w.Interval, buildTick), w.RunOnce)
}

// RunOnce is one tick: fail builds whose worker stopped, queue due
// refreshes, start the builds that may start, and delete retired images.
func (w *RepoEnvironmentBuilder) RunOnce(ctx context.Context) error {
	sweepErr := w.sweep(ctx)
	refreshErr := w.refresh(ctx)
	claimErr := w.claim(ctx)
	return errors.Join(sweepErr, refreshErr, claimErr, w.purge(ctx))
}

// sweep fails the builds whose worker went silent or that ran past the build
// limit, and terminates their builders.
func (w *RepoEnvironmentBuilder) sweep(ctx context.Context) error {
	s := w.Service
	stale, err := s.RepoBuilds.FailStaleAgentRepoEnvironmentBuilds(ctx, buildStaleAfter, repoEnvironmentBuildTimeout+buildCommitAllowance)
	for _, b := range stale {
		log.Printf("agent repo environment builds: swept build=%s code=%s", b.ID, b.FailureCode)
		if b.SandboxID != "" {
			s.terminateBuilder(ctx, b.WorkspaceID, b.ID, b.SandboxID)
		}
	}
	return err
}

// due reports whether a pass paced every interval runs now, and records it.
func due(last *time.Time, now time.Time, interval time.Duration) bool {
	if !last.IsZero() && now.Sub(*last) < interval {
		return false
	}
	*last = now
	return true
}

// refresh queues the daily refresh of every repository due one, at most every
// buildRefreshEvery on this replica.
func (w *RepoEnvironmentBuilder) refresh(ctx context.Context) error {
	s := w.Service
	now := s.Now()
	if !due(&w.lastRefresh, now, buildRefreshEvery) {
		return nil
	}
	rows, err := s.RepoBuilds.AgentRepoEnvironmentRefreshes(ctx)
	if err != nil {
		return err
	}
	base := s.Builders.AgentTemplateImage("")
	for _, r := range rows {
		if !refreshDue(r, base, now) {
			continue
		}
		// A workspace that may not start compute is not built for until it may.
		if s.RequireBillingMutation(ctx, r.WorkspaceID) != nil || s.RequirePaymentMethod(ctx, r.WorkspaceID) != nil {
			continue
		}
		// Busy, rate-limited or gone since the read: the next pass decides again.
		if _, err := s.queueBuild(ctx, r.WorkspaceID, r.Repo, buildTriggerRefresh, true); err == nil {
			log.Printf("agent repo environment builds: refresh queued (workspace=%s repo=%s)", r.WorkspaceID, r.Repo)
		}
	}
	return nil
}

// refreshDue reports whether a repository needs a build: it never built, its
// newest build is of another version (a save whose build was refused), its
// newest success ran on an older agent image, bex itself failed its newest
// build a while ago, or its newest build is about a day old.
func refreshDue(r store.AgentRepoEnvironmentRefresh, base string, now time.Time) bool {
	b := r.Newest
	switch {
	case b == nil:
		return true
	case b.Status == store.AgentRepoEnvironmentBuildFailed && b.FailureCode == buildFailedInternalError:
		return b.FinishedAt == nil || now.Sub(*b.FinishedAt) >= buildInternalRetryAfter
	case b.Version != r.Version:
		return true
	case b.Status == store.AgentRepoEnvironmentBuildSuccess && base != "" && b.BaseImage != base &&
		now.Sub(b.CreatedAt) >= buildImageSettle && inRefreshSlot(r.WorkspaceID, r.Repo, now):
		return true
	}
	return now.Sub(b.CreatedAt) >= repoEnvironmentRefreshAge-refreshJitter(r.WorkspaceID, r.Repo)
}

// refreshJitter is a repository's fixed share of the refresh spread.
func refreshJitter(workspaceID, repo string) time.Duration {
	h := fnv.New64a()
	_, _ = h.Write([]byte(workspaceID + "/" + repo))
	return time.Duration(h.Sum64() % uint64(repoEnvironmentRefreshSpread))
}

// inRefreshSlot reports whether now falls in the repository's slot of the
// refresh spread: each repository has one buildRefreshSlot in every spread,
// wider than the refresh pass, so a pass lands in it.
func inRefreshSlot(workspaceID, repo string, now time.Time) bool {
	slots := int64(repoEnvironmentRefreshSpread / buildRefreshSlot)
	return (now.UnixNano()/int64(buildRefreshSlot))%slots == int64(refreshJitter(workspaceID, repo)/buildRefreshSlot)
}

// purge deletes the retired images past their grace from the registry, at
// most every buildPurgeEvery on this replica. The claim defers each one's next
// attempt, so replicas share the work and a failed delete backs off.
func (w *RepoEnvironmentBuilder) purge(ctx context.Context) error {
	s := w.Service
	if !due(&w.lastPurge, s.Now(), buildPurgeEvery) {
		return nil
	}
	rows, err := s.RepoBuilds.ClaimAgentRepoEnvironmentBuildPurges(ctx, store.AgentRepoEnvironmentBuildPurge{
		Grace: buildImageGrace, Backoff: buildPurgeBackoff, MaxBackoff: buildPurgeMaxBackoff, Limit: buildPurgeBatch})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.Builders.DeleteAgentBuildImage(ctx, row.WorkspaceID, row.ID); err != nil {
			log.Printf("agent repo environment builds: delete image (build=%s): %v", row.ID, err)
			continue
		}
		if err := s.RepoBuilds.ClearAgentRepoEnvironmentBuildImage(ctx, row.ID); err != nil {
			log.Printf("agent repo environment builds: record image deleted (build=%s): %v", row.ID, err)
		}
	}
	return nil
}

// claim starts every build that may start, up to this replica's share.
func (w *RepoEnvironmentBuilder) claim(ctx context.Context) error {
	s := w.Service
	limits := store.AgentRepoEnvironmentBuildLimits{PerWorkspace: maxRepoEnvironmentBuildsPerWorkspace, LiveSandboxes: s.MaxLiveSandboxes}
	for w.reserve() {
		claimed, err := s.RepoBuilds.ClaimAgentRepoEnvironmentBuild(ctx, limits)
		if err != nil {
			w.release()
			if errors.Is(err, store.ErrNotFound) {
				return nil
			}
			return err
		}
		w.run(func() {
			defer w.release()
			w.build(ctx, claimed)
		})
	}
	return nil
}

func (w *RepoEnvironmentBuilder) reserve() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running >= maxBuildsPerReplica {
		return false
	}
	w.running++
	return true
}

func (w *RepoEnvironmentBuilder) release() {
	w.mu.Lock()
	w.running--
	w.mu.Unlock()
}

func (w *RepoEnvironmentBuilder) run(fn func()) {
	if w.runner != nil {
		w.runner(fn)
		return
	}
	go fn()
}

// buildFailure is a build's failure code and reason.
type buildFailure struct {
	code, reason string
	steps        json.RawMessage
}

// build runs one claimed build to its end: the builder, the driver's verdict
// or the build limit, the commit, retention. A build cancelled or failed
// elsewhere meanwhile stops at its next heartbeat; its builder is terminated
// however it ends.
func (w *RepoEnvironmentBuilder) build(ctx context.Context, claimed store.ClaimedAgentRepoEnvironmentBuild) {
	s := w.Service
	b := claimed.Build
	alive, stop := context.WithCancel(ctx)
	defer stop()
	go core.Heartbeat(alive, "agent repo environment build "+b.ID, cmp.Or(w.HeartbeatInterval, buildHeartbeat), stop, func(ctx context.Context) (bool, error) {
		return s.RepoBuilds.HeartbeatAgentRepoEnvironmentBuild(ctx, b.ID)
	})
	settle := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.WithoutCancel(ctx), buildSettleTimeout)
	}
	fail := func(f buildFailure) {
		settleCtx, cancel := settle()
		defer cancel()
		if _, err := s.RepoBuilds.FailAgentRepoEnvironmentBuild(settleCtx, b.ID, f.code, f.reason, f.steps); err != nil {
			log.Printf("agent repo environment builds: record failure (build=%s): %v", b.ID, err)
		}
		log.Printf("agent repo environment builds: failed build=%s code=%s reason=%q", b.ID, f.code, f.reason)
		s.retainBuilds(settleCtx, b)
	}
	sb, err := s.startBuilder(alive, claimed)
	if err != nil {
		log.Printf("agent repo environment builds: builder start failed (build=%s): %v", b.ID, err)
		fail(buildFailure{code: buildFailedInternalError, reason: builderStartFailure(err)})
		return
	}
	defer func() {
		settleCtx, cancel := settle()
		defer cancel()
		s.terminateBuilder(settleCtx, b.WorkspaceID, b.ID, sb.ID)
	}()
	if ok, err := s.RepoBuilds.SetAgentRepoEnvironmentBuilder(alive, b.ID, sb.ID, s.Builders.AgentTemplateImage("")); err != nil || !ok {
		if err != nil {
			fail(buildFailure{code: buildFailedInternalError, reason: "the builder could not be recorded"})
		}
		return
	}
	report, failure := w.awaitBuild(alive, b, sb.ID)
	if alive.Err() != nil {
		return
	}
	if failure != nil {
		fail(*failure)
		return
	}
	commit, err := s.Builders.CommitAgentBuildSandbox(alive, b.WorkspaceID, b.ID, sb.ID)
	if alive.Err() != nil {
		return
	}
	if err != nil {
		log.Printf("agent repo environment builds: commit failed (build=%s): %v", b.ID, err)
		f := buildFailure{code: buildFailedInternalError, reason: "the build's image could not be saved", steps: report.steps}
		if errors.Is(err, sandbox.ErrAgentImageStorageLimit) {
			f = buildFailure{code: buildFailedStorageLimit, reason: boundedText(err.Error(), maxBuildReasonRunes), steps: report.steps}
		}
		fail(f)
		return
	}
	settleCtx, cancel := settle()
	defer cancel()
	ok, err := s.RepoBuilds.SucceedAgentRepoEnvironmentBuild(settleCtx, b.ID, store.AgentRepoEnvironmentBuildImage{
		Digest: commit.Digest, SizeBytes: commit.SizeBytes, HeadSHA: report.head, Steps: report.steps})
	if err != nil || !ok {
		// Cancelled while it committed: no session ever saw the image it just
		// wrote, so it goes at once.
		log.Printf("agent repo environment builds: build settled elsewhere, deleting its image (build=%s): %v", b.ID, err)
		if err := s.Builders.DeleteAgentBuildImage(settleCtx, b.WorkspaceID, b.ID); err != nil {
			log.Printf("agent repo environment builds: delete orphaned image (build=%s): %v", b.ID, err)
		}
		return
	}
	log.Printf("agent repo environment builds: succeeded build=%s repo=%s bytes=%d", b.ID, b.Repo, commit.SizeBytes)
	s.retainBuilds(settleCtx, b)
}

// retainBuilds applies the repository's retention once a build settled
// (ADR111 D8); the images it retires leave the registry in a later purge.
func (s *Service) retainBuilds(ctx context.Context, b store.AgentRepoEnvironmentBuild) {
	if err := s.RepoBuilds.RetainAgentRepoEnvironmentBuilds(ctx, b.WorkspaceID, b.Repo, repoEnvironmentBuildKeepImages, repoEnvironmentBuildKeepRecords); err != nil {
		log.Printf("agent repo environment builds: retention (workspace=%s repo=%s): %v", b.WorkspaceID, b.Repo, err)
	}
}

// startBuilder starts the build's builder sandbox: the driver in build mode
// with the environment the claim read, a read-only clone URL for the build
// principal, and the secrets a session on the repository receives unnamed
// (ADR111 D6), under the workspace's egress tier.
func (s *Service) startBuilder(ctx context.Context, claimed store.ClaimedAgentRepoEnvironmentBuild) (sandbox.Sandbox, error) {
	b := claimed.Build
	workspaceEnv, err := s.workspaceEnvironment(ctx, b.WorkspaceID)
	if err != nil {
		return sandbox.Sandbox{}, err
	}
	allowlist, err := sessionegress.ExtraDestinations(effectiveHosts(workspaceEnv))
	if err != nil {
		return sandbox.Sandbox{}, err
	}
	env, err := s.builderEnv(ctx, claimed, workspaceEnv.NetworkTier)
	if err != nil {
		return sandbox.Sandbox{}, err
	}
	return s.Builders.CreateAgentBuildSandbox(ctx, b.WorkspaceID, b.ID, b.Repo, sessionEgress(workspaceEnv, allowlist), env)
}

// builderEnv is the build mode's driver env (lego/agent-image/driver
// build.ts): the repository, its proxy URL and environment, the network tier
// and the build's secrets with their scrub list.
func (s *Service) builderEnv(ctx context.Context, claimed store.ClaimedAgentRepoEnvironmentBuild, tier string) (map[string]string, error) {
	b := claimed.Build
	repoURL, err := agentsession.ProxyRepositoryURL(s.gitProxyURL(), store.SandboxNamespace(b.WorkspaceID), b.ID, b.Repo, agentsession.BuildBranch(b.ID))
	if err != nil {
		return nil, err
	}
	env := s.withNetworkEnv(map[string]string{
		buildModeEnv:       "build",
		envRepoURL:         repoURL,
		envRepository:      b.Repo,
		repoEnvironmentEnv: repoEnvironmentJSON(claimed.Environment),
	}, tier, b.WorkspaceID, b.ID)
	env, _, err = s.withSecrets(ctx, b.WorkspaceID, b.Repo, nil, nil, env)
	return env, err
}

// builderStartFailure is the reason a build failed whose builder did not
// start: the capacity or start refusal a session would name, or a coded
// refusal's own message (a billing gate).
func builderStartFailure(err error) string {
	switch {
	case sandbox.IsCapacityLimit(err):
		return sandbox.CapacityFailureReason
	case sandbox.IsStartFailure(err):
		return sandbox.StartFailureReason
	}
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return "the builder sandbox could not start: " + coded.Error()
	}
	return "the builder sandbox could not start"
}

// terminateBuilder ends a builder; a failure is left to the inventory
// reconcile.
func (s *Service) terminateBuilder(ctx context.Context, workspaceID, buildID, sandboxID string) {
	if err := s.Builders.TerminateAgentBuildSandbox(ctx, workspaceID, buildID, sandboxID); err != nil {
		log.Printf("agent repo environment builds: terminate builder (build=%s sandbox=%s): %v", buildID, sandboxID, err)
	}
}

// buildStatus is the build mode's status file, absent until the build ends:
// {"state":"succeeded","build":{"head","steps"}} or
// {"state":"failed","code","error","build":{"steps"}}. The sandbox is
// untrusted, so only the codes the driver may report are believed and every
// string is bounded.
type buildStatus struct {
	State string `json:"state"`
	Code  string `json:"code"`
	Error string `json:"error"`
	Build struct {
		Head  string                     `json:"head"`
		Steps []RepoEnvironmentBuildStep `json:"steps"`
	} `json:"build"`
}

// buildReport is a succeeded build's checkout head and redacted log.
type buildReport struct {
	head  string
	steps json.RawMessage
}

// awaitBuild reads the builder's status file until the driver reports, the
// builder is gone, its status stays unreadable past the retry window, or the
// build passes its limit, which only bex-api enforces (build_timeout).
func (w *RepoEnvironmentBuilder) awaitBuild(ctx context.Context, b store.AgentRepoEnvironmentBuild, sandboxID string) (buildReport, *buildFailure) {
	s := w.Service
	buildTimeout := cmp.Or(w.BuildTimeout, repoEnvironmentBuildTimeout)
	limit, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	tick := time.NewTicker(cmp.Or(w.PollInterval, buildPollInterval))
	defer tick.Stop()
	var unreadableSince time.Time
	for {
		readCtx, done := context.WithTimeout(limit, statusReadTimeout)
		raw, err := s.Builders.ReadAgentBuildStatus(readCtx, b.WorkspaceID, b.ID, sandboxID)
		done()
		switch {
		case errors.Is(err, core.ErrNotFound):
			return buildReport{}, &buildFailure{code: buildFailedInternalError, reason: "the builder ended before the build finished: " + lostSandboxReason(err)}
		case err != nil:
			if unreadableSince.IsZero() {
				unreadableSince = s.Now()
			} else if s.Now().Sub(unreadableSince) >= statusRetryWindow(w.StatusReadFailureTTL) {
				return buildReport{}, &buildFailure{code: buildFailedInternalError, reason: "the builder's status could not be read"}
			}
			log.Printf("agent repo environment builds: read status failed (build=%s): %v", b.ID, err)
		default:
			unreadableSince = time.Time{}
			if report, failure, done := parseBuildStatus(raw); done {
				return report, failure
			}
		}
		select {
		case <-limit.Done():
			if ctx.Err() != nil {
				return buildReport{}, nil
			}
			return buildReport{}, &buildFailure{code: buildFailedTimeout, reason: fmt.Sprintf("the build did not finish within %s", shortDuration(buildTimeout))}
		case <-tick.C:
		}
	}
}

// parseBuildStatus reads a builder's status file. done is false while the
// file is absent, torn or reports no verdict.
func parseBuildStatus(raw string) (buildReport, *buildFailure, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return buildReport{}, nil, false
	}
	var status buildStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		return buildReport{}, nil, false
	}
	steps := boundedBuildSteps(status.Build.Steps)
	switch status.State {
	case driverStateSucceeded:
		head := ""
		if core.ValidCommitSHA(status.Build.Head) {
			head = status.Build.Head
		}
		return buildReport{head: head, steps: steps}, nil, true
	case driverStateFailed:
		code := status.Code
		switch code {
		case buildFailedSetup, buildFailedStepTimeout, buildFailedCheckout, buildFailedInternalError:
		default:
			code = buildFailedInternalError
		}
		reason := boundedText(status.Error, maxBuildReasonRunes)
		if reason == "" {
			reason = "the build failed"
		}
		return buildReport{}, &buildFailure{code: code, reason: reason, steps: steps}, true
	}
	return buildReport{}, nil, false
}

// boundedBuildSteps is the driver's step log, bounded, as JSON.
func boundedBuildSteps(steps []RepoEnvironmentBuildStep) json.RawMessage {
	out := make([]RepoEnvironmentBuildStep, 0, min(len(steps), maxBuildSteps))
	for _, step := range steps[:min(len(steps), maxBuildSteps)] {
		step.Name = boundedText(step.Name, maxBuildStepName)
		step.DurationMs = max(step.DurationMs, 0)
		if len(step.Output) > maxBuildStepOutput {
			step.Output = strings.ToValidUTF8(step.Output[len(step.Output)-maxBuildStepOutput:], "")
		}
		out = append(out, step)
	}
	raw, _ := json.Marshal(out)
	return raw
}

// boundedText is untrusted text on one line, at most limit runes.
func boundedText(raw string, limit int) string {
	text := strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, raw))
	if r := []rune(text); len(r) > limit {
		text = strings.TrimSpace(string(r[:limit])) + "…"
	}
	return text
}
