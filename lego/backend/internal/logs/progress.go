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

package logs

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Platform progress lines (w1/m48): Render's deploy feed is never silent — the
// platform itself narrates (`==> Cloning from…`, `==> Your service is live 🎉`)
// while the build pod schedules and pulls. bex synthesizes the equivalent from
// the deploy row's observed lifecycle (created/started/finished + terminal
// status), so a cold node's minutes-long pre-build window shows the deploy's
// own narration instead of nothing. The contract table lives in
// docs/render-artifacts/live-deploy-following.md § Platform progress-line
// contract. Lines carry type=build (they narrate the build/deploy), the deploy
// id as `instance`, and container "platform"; ids derive from the existing
// logID (instance+timestamp+message), so identical lines are deterministic
// across reads and dedupe against the live tail for free.

// progressContainer is the `container` label on synthesized lines — no pod
// container is called this (buildkit / sign / kpack step names), so the
// platform's own narration stays distinguishable from real build stdout.
const progressContainer = "platform"

// DeployProgress is one deploy row's lifecycle facts, the input to line
// synthesis — the logs domain's own shape so it stays store-free (the
// composition root adapts store.Deploy, the way PodLogSource keeps the
// clientset out).
type DeployProgress struct {
	ID            string
	Status        string
	Image         string // image-backed deploys; "" for repo builds
	Commit        string // resolved commit sha; "" when unresolved
	FailureReason string // human-actionable cause on a failed deploy (w5/064); "" when none
	// Built reports that a build actually ran for this deploy — the same
	// evidence the Events tab shows as Build started/ended (a `build_started`
	// service event fact). Deploys that run no build (rollbacks, and any
	// rollout that reuses the active artifact) are false, and earn rollout
	// narration instead of a build story (w4/m110 t004).
	Built      bool
	CreatedAt  time.Time
	StartedAt  time.Time // zero until the deploy starts
	FinishedAt time.Time // zero until terminal
}

// DeployProgressSource lists an App's deploy rows, newest-first, created
// before `end` (zero = no upper bound), bounded to a reasonable page by the
// implementation. nil => no synthesis (byte-identical prior behavior). The
// resource key is the public srv- id — the store's app_id — so legacy
// label-less Apps simply resolve to no rows.
type DeployProgressSource func(ctx context.Context, resource string, end time.Time) ([]DeployProgress, error)

// These are store.DeployCreated/DeployQueued/DeployBuildInProgress. The logs
// domain stays store-free (DeployProgress carries the status as a plain
// string), so the values the tail branches on are restated here rather than
// imported.
const (
	deployStatusCreated         = "created"
	deployStatusQueued          = "queued"
	deployStatusBuildInProgress = "build_in_progress"
)

// buildWait is the App's CURRENT reason for a not-yet-running build, read from
// the `Ready` condition the operator writes (`BuildQueued` for every
// dispatched-but-not-running wait, `RegistryCredsPending` for the pre-dispatch
// registry window). Zero value = nothing to narrate.
type buildWait struct {
	Reason  string
	Message string
}

// progressContext is the App-derived input to line synthesis: the fields that
// shape the lines (repo/branch/type), the App's current queued-wait reason, and
// the object key a live tail re-reads that reason from each tick. Bundled into
// one value so the read path and the tail path cannot drift apart, and so
// neither verb grows a sixth positional string.
type progressContext struct {
	repo        string
	branch      string
	serviceType string
	wait        buildWait
	namespace   string
	name        string
}

// newProgressContext snapshots an App CR for narration.
func newProgressContext(app *appv1alpha1.App) progressContext {
	if app == nil {
		return progressContext{}
	}
	return progressContext{
		repo:        app.Spec.Repo,
		branch:      app.Spec.Branch,
		serviceType: app.Spec.Type,
		wait:        appBuildWait(app),
		namespace:   app.Namespace,
		name:        app.Name,
	}
}

// appBuildWait extracts the queued-wait reason from an App's `Ready` condition,
// matching the same currency rule the deploy projector uses
// (store.failureReasonFor): only the condition observed for THIS generation
// counts, so a stale reason from a superseded spec is never narrated. Any
// reason other than the two wait reasons means the App is not waiting to build,
// which is the empty (silent) answer.
func appBuildWait(app *appv1alpha1.App) buildWait {
	if app == nil {
		return buildWait{}
	}
	for i := range app.Status.Conditions {
		c := &app.Status.Conditions[i]
		if c.Type != appv1alpha1.ConditionReady || c.ObservedGeneration != app.Generation {
			continue
		}
		switch c.Reason {
		case appv1alpha1.ReasonBuildQueued, appv1alpha1.ReasonRegistryCredsPending:
			return buildWait{Reason: c.Reason, Message: c.Message}
		}
		return buildWait{}
	}
	return buildWait{}
}

// The queued-wait narration vocabulary. Exactly two shapes reach a tenant:
// its OWN workspace's slot usage, and a countless, tenant-neutral line for
// every wait whose cause lives outside this workspace.
//
// SECURITY (m99 t002): the operator's `Ready` message is platform-internal
// text. The cluster-wide cap message ("cluster has 4/4 concurrent builds
// active…") counts OTHER tenants' builds, and the scheduler wait ("waiting for
// build capacity: <kube scheduler message>") names nodes, taints and quotas.
// Neither may be echoed. waitLine is therefore an allow-list that renders
// operator text only for the one message shape that is provably this
// workspace's own, and falls back to the neutral line for everything else —
// including any wait message a newer operator invents.
const (
	waitLineNeutral          = "==> Waiting for platform build capacity"
	waitLineRegistryCreds    = "==> Preparing registry credentials"
	workspaceCapMessageStart = "workspace has "
	workspaceCapMessageEnd   = " concurrent builds active; waiting for a slot"
)

// waitLine renders the tenant-visible line for a queued wait; "" when there is
// nothing to say.
func waitLine(w buildWait) string {
	switch w.Reason {
	case appv1alpha1.ReasonRegistryCredsPending:
		return waitLineRegistryCreds
	case appv1alpha1.ReasonBuildQueued:
		if counts, ok := workspaceCapCounts(w.Message); ok {
			return "==> Waiting for a build slot: this workspace has " + counts + " builds running"
		}
		return waitLineNeutral
	}
	return ""
}

// workspaceCapCounts recognizes the per-workspace concurrency-cap message and
// returns its `<active>/<limit>` fragment. The match is exact on both ends and
// the fragment must be two digit runs around a single slash, so the only
// operator-authored bytes that can ever reach a tenant are that workspace's own
// two numbers — never free-form text, and never the `cluster` noun's counts.
func workspaceCapCounts(message string) (string, bool) {
	rest, ok := strings.CutPrefix(message, workspaceCapMessageStart)
	if !ok {
		return "", false
	}
	counts, ok := strings.CutSuffix(rest, workspaceCapMessageEnd)
	if !ok {
		return "", false
	}
	active, limit, ok := strings.Cut(counts, "/")
	if !ok || !digitsOnly(active) || !digitsOnly(limit) {
		return "", false
	}
	return counts, true
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// progressLines renders the platform lines a deploy row has earned so far.
// Only observed moments produce lines — no invented clone/checkout stages
// (those arrive as real BuildKit stdout once the pod runs). `deactivated`
// keeps its live line: finished_at records when this deploy went live; its
// later replacement is outside this deploy's own story.
//
// A row still `queued` also earns the wait line for pc.wait: the whole point of
// m99 is that a capacity wait reads differently from a stuck deploy. It is
// timestamped at CreatedAt (the moment the wait began) so the line is
// deterministic across reads — identical reasons collapse to one id, and a
// CHANGED reason is a new id — which is what makes the tail's dedupe "re-emit
// on change, never per tick" for free.
func progressLines(d DeployProgress, pc progressContext) []LogEntry {
	repo := pc.repo
	// A build story is owed only by a deploy that actually built (w4/m110
	// t004). Every repo-backed deploy used to get one, so a rollback and a
	// config-change rollout that reused the active artifact each read as a
	// 14-30s build: `==> Build queued` / `==> Building from <repo>@<sha>` with
	// no build steps between them and no Build events behind them. The lines
	// were synthesized from the row's own timestamps, which are observed —
	// but their WORDING asserted a build nobody ran.
	//
	// Absence of evidence counts only once the deploy is OVER: build_started is
	// written by the reconciler as it observes the dispatch, so an in-flight
	// row may legitimately have no fact yet (a still-queued deploy has none by
	// design — there is no build to report until a slot opens). A terminal row
	// with no build fact, though, is settled: nothing will arrive later, and a
	// missing fact for a build that did run costs only the two synthetic lines
	// — which is the safe direction to be wrong in.
	built := repo != ""
	if !d.FinishedAt.IsZero() {
		built = built && d.Built
	}
	var out []LogEntry
	add := func(t time.Time, msg string) {
		out = append(out, LogEntry{
			Timestamp: t.UTC().Format(time.RFC3339Nano),
			Message:   msg,
			Labels: map[string]string{
				LabelType:     LogTypeBuild,
				LabelInstance: d.ID,
				"container":   progressContainer,
			},
		})
	}
	if !d.CreatedAt.IsZero() {
		if built {
			add(d.CreatedAt, "==> Build queued")
		} else {
			add(d.CreatedAt, "==> Deploy queued")
		}
		if d.Status == deployStatusQueued && d.StartedAt.IsZero() && d.FinishedAt.IsZero() {
			if line := waitLine(pc.wait); line != "" {
				add(d.CreatedAt, line)
			}
		}
	}
	if !d.StartedAt.IsZero() {
		switch {
		case built:
			ref := d.Commit
			if len(ref) > 7 {
				ref = ref[:7]
			}
			if ref == "" {
				ref = pc.branch
			}
			if ref != "" {
				add(d.StartedAt, fmt.Sprintf("==> Building from %s@%s", repo, ref))
			} else {
				add(d.StartedAt, fmt.Sprintf("==> Building from %s", repo))
			}
		case d.Image != "":
			// Image-backed service, or a rollback — the row carries the exact
			// image it restores, so name it.
			add(d.StartedAt, fmt.Sprintf("==> Deploying image %s", d.Image))
		default:
			// Repo-backed, but no build ran and the row names no image: a
			// rollout of the artifact already active. Narrate the rollout.
			add(d.StartedAt, "==> Rolling out the current release")
		}
	}
	if !d.FinishedAt.IsZero() {
		for _, msg := range terminalLines(d.Status, pc.serviceType, d.FailureReason) {
			add(d.FinishedAt, msg)
		}
	}
	return out
}

// terminalLines maps a terminal deploy status to its closing line(s). An empty
// reason keeps the historical bare line (w1/m48); a non-empty reason is
// appended so the CLI's type=build narration carries the same cause the events
// feed already exposes as failureReason (w5/064). Multi-line reasons become
// one stream line per segment (the feed is line-oriented).
func terminalLines(status, serviceType, reason string) []string {
	base := terminalLine(status, serviceType)
	if base == "" {
		return nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return []string{base}
	}
	switch status {
	case "build_failed", "pre_deploy_failed", "update_failed":
		// only failure statuses carry a failureReason into the narration
	default:
		return []string{base}
	}
	parts := strings.Split(reason, "\n")
	out := make([]string, 0, len(parts))
	first := strings.TrimSpace(parts[0])
	out = append(out, base+": "+first)
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, "==> "+part)
	}
	return out
}

// terminalLine maps a terminal deploy status to its closing line; "" for the
// open states (no line yet) and unknown values (never invent an outcome). A
// cron_job runs its command to completion on a schedule — it is never a served
// "live" service — so a successful deploy narrates that it is scheduled, not
// live (background workers keep the "live" line: they run continuously).
func terminalLine(status, serviceType string) string {
	switch status {
	case "live", "deactivated":
		if serviceType == "cron_job" {
			return "==> Cron job deployed — runs on schedule 🎉"
		}
		return "==> Your service is live 🎉"
	case "build_failed":
		return "==> Build failed"
	case "pre_deploy_failed":
		return "==> Pre-deploy failed"
	case "update_failed":
		return "==> Deploy failed"
	case "canceled":
		return "==> Deploy canceled"
	}
	return ""
}

// synthesizeProgress merges platform progress lines into a windowed read's
// entries. Applies only when the query explicitly asks for build logs (the
// same condition under which the store selector includes build streams) and a
// source is wired. Additive by design: a source error degrades to the plain
// read (narration must never break a log query), and it runs only on the
// store-backed path — a missing Loki still reports buildStoreUnavailable, so
// platform lines never masquerade as a successful empty build history.
func (s *Service) synthesizeProgress(ctx context.Context, q LogQuery, resource string, pc progressContext, entries []LogEntry) []LogEntry {
	if s.DeployProgress == nil || !slices.Contains(q.Types, LogTypeBuild) {
		return entries
	}
	rows, err := s.DeployProgress(ctx, resource, q.End)
	if err != nil {
		return entries
	}
	merged := entries
	for i, d := range rows {
		// Skip rows that ended before the window opened; per-line keep()
		// applies the exact bounds below.
		if !q.Since.IsZero() && !d.FinishedAt.IsZero() && d.FinishedAt.Before(q.Since) {
			continue
		}
		if !q.keepPod(d.ID) {
			continue
		}
		rowPC := pc
		if i > 0 {
			// The App's `Ready` condition describes the CURRENT wait, which can
			// only belong to the newest row; an older row must never borrow it.
			rowPC.wait = buildWait{}
		}
		for _, e := range progressLines(d, rowPC) {
			if q.keep(e) {
				merged = append(merged, e)
			}
		}
	}
	if len(merged) == len(entries) {
		return entries
	}
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].Timestamp < merged[j].Timestamp })
	return q.capToLimit(merged)
}

// progressFollower tracks which platform lines a live build tail has already
// emitted, so subscribe-time catch-up, wait-loop transitions, and the
// post-stream terminal check each emit a line exactly once.
type progressFollower struct {
	s        *Service
	q        LogQuery
	resource string
	pc       progressContext
	emitted  map[string]bool
	status   string // the followed deploy row's status at the last read
}

// buildOpen reports that the followed deploy has not finished building: its
// row is still created/queued/build_in_progress. A build tail with no pod yet
// (the registry-credentials wait, the workspace/cluster slot caps, the gap
// before the Job exists) waits on it rather than calling the build over.
func (f *progressFollower) buildOpen() bool {
	if f == nil {
		return false
	}
	switch f.status {
	case deployStatusCreated, deployStatusQueued, deployStatusBuildInProgress:
		return true
	}
	return false
}

// newProgressFollower returns nil when no source is wired — every method is
// nil-safe, so the tail path stays a straight line.
func (s *Service) newProgressFollower(q LogQuery, resource string, pc progressContext) *progressFollower {
	if s.DeployProgress == nil {
		return nil
	}
	return &progressFollower{s: s, q: q, resource: resource, pc: pc, emitted: map[string]bool{}}
}

// refreshWait re-reads the App's `Ready` condition so a tail narrates the
// CURRENT reason rather than the one that held at subscribe. It runs only while
// the row is still queued — the one window in which the reason can change and
// matter — so a normal build tail costs no extra read. A failed read keeps the
// last known reason: narration must never tear down (or stall) a tail, and the
// dedupe below makes a repeated reason a no-op anyway.
func (f *progressFollower) refreshWait(ctx context.Context, d DeployProgress) {
	if d.Status != deployStatusQueued || f.pc.name == "" || f.s.Client == nil {
		return
	}
	var fresh appv1alpha1.App
	if err := f.s.Client.Get(ctx, client.ObjectKey{Namespace: f.pc.namespace, Name: f.pc.name}, &fresh); err != nil {
		return
	}
	f.pc.wait = appBuildWait(&fresh)
}

// emitReached sends every not-yet-emitted line the newest deploy row has
// earned. Source errors are swallowed (narration never tears down a tail);
// emit errors propagate (the client is gone).
func (f *progressFollower) emitReached(ctx context.Context, emit func(LogEntry) error) error {
	if f == nil {
		return nil
	}
	rows, err := f.s.DeployProgress(ctx, f.resource, time.Time{})
	if err != nil || len(rows) == 0 {
		return nil
	}
	d := rows[0] // newest — the deploy this tail is following
	if !f.q.keepPod(d.ID) {
		return nil
	}
	f.status = d.Status
	f.refreshWait(ctx, d)
	for _, e := range progressLines(d, f.pc) {
		// logID is the adapters' stable line identity — reusing it here keeps
		// the follower's dedupe in lockstep with what clients see.
		key := logID(e)
		if f.emitted[key] || !f.q.keep(e) {
			continue
		}
		f.emitted[key] = true
		setLogEntryResource(&e, f.resource)
		if err := emit(e); err != nil {
			return err
		}
	}
	return nil
}
