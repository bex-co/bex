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

	"github.com/bex-co/bex/lego/types/renderlog"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
	ID     string
	Status string
	// Generation is the release generation the deploy ran as — the revision its
	// build and pre-deploy Jobs are named by (appv1alpha1.BuildRevision), so a
	// Deploy-scoped read can tell one deploy's pods from another's.
	Generation       int64
	Image            string // image-backed deploys; "" for repo builds
	Commit           string // resolved commit sha; "" when unresolved
	DetectedHTTPPort int32  // verified listener saved on this deploy, never borrowed from the current App
	FailureReason    string // human-actionable cause on a failed deploy (w5/064); "" when none
	// Built reports that a build actually ran for this deploy — the same
	// evidence the Events tab shows as Build started/ended (a `build_started`
	// service event fact). Deploys that run no build (rollbacks, and any
	// rollout that reuses the active artifact) are false, and earn rollout
	// narration instead of a build story (w4/m110 t004).
	Built      bool
	CreatedAt  time.Time
	StartedAt  time.Time // zero until the deploy starts
	FinishedAt time.Time // zero until terminal
	// PortScanSince is when the rollout's stall first said no port was open,
	// PortScanUntil when a later stall said the scan was over; zero when not
	// (w8/385). ReadAt is when the row was read: an open scan's lines run up
	// to it.
	PortScanSince time.Time
	PortScanUntil time.Time
	// PortScanFound is the finding Render words apart from "no open ports"
	// (appv1alpha1.PortScanFound*), recorded at PortScanFoundAt; "" when none
	// (w8/388). Lines from that moment on carry its wording.
	PortScanFound   appv1alpha1.PortScanFinding
	PortScanFoundAt time.Time
	// TimedOut reports that the deploy failed because its rollout ran out of
	// its budget (the close's DeployEnd.TimedOutSeconds, w8/m146), which the
	// log closes with Render's timeout lines (w8/391).
	TimedOut bool
	// ExitCode is how the app container last exited, as the close recorded
	// it (DeployEnd.ExitCode, w8/m146); nil when nothing exited. OOMKilled
	// reports that the kernel killed it for memory. A start command that
	// exited closes the log with Render's exit lines (w8/393).
	ExitCode  *int32
	OOMKilled bool
	// RolloutStartedAt is when the operator first wrote the release's serving
	// template, AppStartedAt each start of its app container while the
	// rollout ran (oldest first), StartCommand the start command that
	// container ran on a native image, InstanceAnnounces whether the
	// instance writes that command's `==> Running` line itself (w32/m575),
	// and WebConcurrencyDefault the WEB_CONCURRENCY default it got
	// (status.releaseRollout, w32/m560). Zero values are none. They time
	// Render's rollout lines (rolloutLines).
	RolloutStartedAt      time.Time
	AppStartedAt          []time.Time
	StartCommand          string
	InstanceAnnounces     bool
	WebConcurrencyDefault int32
	// LiveURLs are the URLs the deploy went live at, primary first, as its
	// row recorded them when it closed live (w32/m567); nil is no record.
	// They write Render's primary URL block after the live line.
	LiveURLs []string
	// PreDeploy is the deploy's pre-deploy step as its row recorded it
	// (w32/m572); the zero value is no step, or one an older operator ran.
	PreDeploy PreDeployRun
	ReadAt    time.Time
}

// PreDeployRun is a deploy's pre-deploy step as its row recorded it from the
// operator's status.preDeploy (w32/m572). Zero values are none.
type PreDeployRun struct {
	// Status is the row's pre_deploy_status: running, succeeded, failed or
	// canceled.
	Status string
	// StartedAt is when the operator launched the step (microseconds),
	// ContainerStartedAt when its container started (whole seconds), and
	// FinishedAt when the operator settled its verdict (microseconds).
	StartedAt          time.Time
	ContainerStartedAt time.Time
	FinishedAt         time.Time
	// Command is the command the step ran, as configured; Native reports that
	// a native image's entrypoint ran it.
	Command string
	Native  bool
	// ExitCode is the exit status of a command that ended on its own or was
	// killed for memory (OOMKilled); nil when none was recorded.
	ExitCode  *int32
	OOMKilled bool
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
	deployStatusPreDeployFailed = "pre_deploy_failed"
	// store.PreDeploySucceeded, a row's pre_deploy_status.
	preDeploySucceeded = "succeeded"
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
	hook        hookRun
	oidc        oidcRefusal
	namespace   string
	name        string
}

// oidcRefusal is the release whose environment asked for Managed OIDC tokens
// it does not get (status.managedOIDC): its workspace plan does not include
// them (w2/m477), or its GCP variables are not ones Google can take (w2/m479).
// lines are the deploy log's explanations; the zero value is none.
type oidcRefusal struct {
	generation int64
	lines      []string
}

// appOIDCRefusal reads the App's current release decision. Only the running
// release's is kept, so an older deploy's log stops carrying the lines once a
// newer release replaces the decision.
func appOIDCRefusal(app *appv1alpha1.App) oidcRefusal {
	st := app.Status.ManagedOIDC
	var lines []string
	var triggers []string
	for _, p := range appv1alpha1.ManagedOIDCProvidersNamed(st.Refused()) {
		triggers = append(triggers, p.Trigger)
	}
	if len(triggers) > 0 {
		lines = append(lines, oidcRefusalLine(triggers))
	}
	if st != nil && st.GCP != nil && st.GCP.Refused != "" {
		lines = append(lines, "==> "+st.GCP.Refused)
	}
	if len(lines) == 0 {
		return oidcRefusal{}
	}
	return oidcRefusal{generation: st.Generation, lines: lines}
}

// oidcRefusalLine is the deploy log's explanation, in Render's own words for
// the plan rule (render.com/docs/oidc: "With a Pro workspace or higher").
func oidcRefusalLine(triggers []string) string {
	named, verb := triggers[0], "is"
	if n := len(triggers); n > 1 {
		named, verb = strings.Join(triggers[:n-1], ", ")+" and "+triggers[n-1], "are"
	}
	return "==> Managed OIDC requires a Pro workspace or higher: " + named + " " + verb + " set, but no identity token is mounted"
}

// hookRun is the App's latest initialDeployHook run (status.initialDeployHook,
// w33/m435): the one-time command the operator starts after a release is
// live. The zero value is no run.
type hookRun struct {
	generation int64 // the release generation that was live when it started
	status     string
	message    string
	startedAt  time.Time
	finishedAt time.Time // zero while it runs
}

// appHookRun reads the App's recorded hook run; a record with no start time
// narrates nothing.
func appHookRun(app *appv1alpha1.App) hookRun {
	run := app.Status.InitialDeployHook
	if run == nil {
		return hookRun{}
	}
	started, err := time.Parse(time.RFC3339, run.StartedAt)
	if err != nil {
		return hookRun{}
	}
	out := hookRun{generation: run.Generation, status: run.Status, message: run.Message, startedAt: started}
	if finished, err := time.Parse(time.RFC3339, run.FinishedAt); err == nil {
		out.finishedAt = finished
	}
	return out
}

// ranFor reports whether the run belongs to d: the hook starts only once d's
// release is live, so only a row that went live under the run's generation.
func (h hookRun) ranFor(d DeployProgress) bool {
	return h.generation > 0 && h.generation == d.Generation && !d.FinishedAt.IsZero() &&
		(d.Status == "live" || d.Status == "deactivated")
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
		hook:        appHookRun(app),
		oidc:        appOIDCRefusal(app),
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

// progressEntry is one platform line of the deploy deployID, stamped at t. The
// deploy is also its build (LabelBuild).
func progressEntry(deployID string, t time.Time, msg string) LogEntry {
	return LogEntry{
		Timestamp: t.UTC().Format(time.RFC3339Nano),
		Message:   msg,
		Labels: map[string]string{
			LabelType:     LogTypeBuild,
			LabelInstance: deployID,
			LabelBuild:    deployID,
			"container":   progressContainer,
		},
	}
}

// styled is line, a plain `==> <text>` narration line, written in Render's
// form f (w32/m576). Each line is given its form where it is written: Render's
// own where a capture of Render's log API shows the line, renderlog.Plain for
// bex's own lines and for Render's lines no capture shows with their codes
// (w32/085). The constants keep the plain words, which displayText gives back.
func styled(f renderlog.Form, line string) string {
	return f.Line(strings.TrimPrefix(line, renderlog.Plain.Open))
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
	add := func(t time.Time, msg string) { out = append(out, progressEntry(d.ID, t, msg)) }
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
		// The operator decides a release's token mounts as it projects
		// the release, after the deploy started.
		if pc.oidc.generation > 0 && pc.oidc.generation == d.Generation {
			for _, line := range pc.oidc.lines {
				add(d.StartedAt, line)
			}
		}
	}
	out = append(out, preDeployLines(d)...)
	out = append(out, rolloutLines(d, repo != "" && d.Built)...)
	out = append(out, portScanLines(d)...)
	if !d.FinishedAt.IsZero() {
		if (d.Status == "live" || d.Status == "deactivated") && d.DetectedHTTPPort > 0 && d.DetectedHTTPPort <= 65535 {
			add(d.FinishedAt, "==> "+appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortDetected,
				appv1alpha1.DeployReasonParam{Name: "port", Value: fmt.Sprint(d.DetectedHTTPPort)}))
		}
		for _, msg := range closingLines(d, pc.serviceType) {
			add(d.FinishedAt, msg)
		}
		// Each line of the block has its own moment: a line's id is its
		// instance, moment and text, and the block repeats two of its lines.
		for i, msg := range primaryURLLines(d) {
			add(d.FinishedAt.Add(time.Duration(i+1)*primaryURLLineStep), msg)
		}
	}
	return out
}

// liveLinesEnd is the moment of the last line d's close wrote: its live
// line's primary URL block, else the close itself.
func liveLinesEnd(d DeployProgress) time.Time {
	return d.FinishedAt.Add(time.Duration(len(primaryURLLines(d))) * primaryURLLineStep)
}

// Render's pre-deploy narration (w32/m572), from deploy logs users quoted in
// GitHub issues (docs/ADR018-render-parity.md § Logs, Build logs): `==>
// Starting pre-deploy: <command>` a few seconds after the build's own last
// line, `==> Running '<command>'` when the step's container starts on a
// native runtime, the command's output, then `==> Pre-deploy complete!`
// (BerriAI/litellm#9549, ganjasan/mapsurvey#73) or `==> Pre-deploy has
// failed`, after which the deploy closes `==> Exited with status N` and the
// troubleshooting line (Unwrenchable/ATOMIC-FIZZ-CAPS-VAULT-77-WASTELAND-GPS#82,
// dukens11-create/Flupflap#206). Both quote the command as configured.
const (
	preDeployStartingLineFormat = "==> Starting pre-deploy: %s"
	preDeployCompleteLine       = "==> Pre-deploy complete!"
	preDeployFailedLine         = "==> Pre-deploy has failed"
)

// preDeployLines are the lines d earned from the operator's record of its
// pre-deploy step: the start at the step's launch, `==> Running '<command>'`
// at a native step container's start, and the step's own verdict at the
// moment the operator settled it, which follows the command's last output
// line. Only a step whose launch was recorded is narrated. Each line is held
// just after the one before it when its moment does not order it (a
// container's start is whole seconds), and a closed row's lines stay at or
// before its close, which records the step's failure after the operator
// settled it. The operator starts the rollout after it settles a passing
// step, so the rollout's lines follow these.
func preDeployLines(d DeployProgress) []LogEntry {
	run, last := d.PreDeploy, d.StartedAt
	if run.StartedAt.IsZero() || run.Command == "" {
		return nil
	}
	var out []LogEntry
	add := func(t time.Time, msg string) {
		if !last.IsZero() && !t.After(last) {
			t = last.Add(rolloutLineStep)
		}
		if !d.FinishedAt.IsZero() && t.After(d.FinishedAt) {
			t = d.FinishedAt
		}
		last = t
		out = append(out, progressEntry(d.ID, t, msg))
	}
	add(run.StartedAt, fmt.Sprintf(preDeployStartingLineFormat, run.Command))
	if run.Native && !run.ContainerStartedAt.IsZero() {
		add(run.ContainerStartedAt, runningLine(run.Command))
	}
	switch {
	case run.FinishedAt.IsZero():
	case run.Status == preDeploySucceeded:
		add(run.FinishedAt, preDeployCompleteLine)
	case preDeployExitStatus(d) != nil:
		add(run.FinishedAt, preDeployFailedLine)
	}
	return out
}

// preDeployExitStatus is the exit status a failed deploy's narrated
// pre-deploy step ended with, which Render's log closes with: a command that
// exited non-zero on its own. nil for every other failure, which keeps bex's
// own line: a step that never ran its command, one killed for memory, one
// stopped at its time limit, and a deploy whose step the log does not
// narrate.
func preDeployExitStatus(d DeployProgress) *int32 {
	run := d.PreDeploy
	if d.Status != deployStatusPreDeployFailed || run.StartedAt.IsZero() || run.Command == "" || run.FinishedAt.IsZero() ||
		run.ExitCode == nil || *run.ExitCode == 0 || run.OOMKilled {
		return nil
	}
	return run.ExitCode
}

// Render's rollout narration (w32/m560), from deploy logs users quoted in
// GitHub issues (docs/ADR018-render-parity.md § Logs, Build logs): `==>
// Deploying...` opens the rollout of every built service, native or Docker
// (DerekSopok/EMS-Kahoot#3, darthjee/ward#109); the WEB_CONCURRENCY line
// follows about 0.1 s later; and on a native runtime `==> Running
// '<command>'` comes each time the start command starts, quoted as configured
// with $PORT unexpanded (frayel/devagent#62).
const (
	deployingLine            = "==> Deploying..."
	webConcurrencyLineFormat = "==> Setting WEB_CONCURRENCY=%d by default, based on available CPUs in the instance"
	runningLineFormat        = "==> Running '%s'"
	// rolloutLineStep is how far a rollout line is held after the line before
	// it when their moments do not order them.
	rolloutLineStep = time.Millisecond
)

// rolloutLines are the lines d earned from the operator's record of its
// rollout: `==> Deploying...` at the rollout's start when d built (a deploy
// that ran no build keeps bex's own rollout line at its start), the
// WEB_CONCURRENCY line when the release's instance got that default, and one
// `==> Running '<command>'` at each recorded start of a native start command
// whose instance does not write that line itself. An image built since
// w32/m575 does, from native-init, in the instance's own stream with its
// labels, where Render's line is (ivanseibel/tretta's log capture), so the
// deploy views that interleave the app's lines show it once; an older image's
// release keeps the narrated copy. A record stamped after d closed, and a
// start after it, are not d's. Each line is held just after the one before it
// when its moment does not order it: the row's start is the dispatch, in
// whole seconds for a deploy that ran no build, and the kubelet stamps a
// container's start in whole seconds too, so a start in the rollout's first
// second would read before it.
func rolloutLines(d DeployProgress, built bool) []LogEntry {
	if d.RolloutStartedAt.IsZero() || (!d.FinishedAt.IsZero() && d.RolloutStartedAt.After(d.FinishedAt)) {
		return nil
	}
	var out []LogEntry
	last := d.StartedAt
	add := func(t time.Time, msg string) {
		if !last.IsZero() && !t.After(last) {
			t = last.Add(rolloutLineStep)
		}
		last = t
		out = append(out, progressEntry(d.ID, t, msg))
	}
	if built {
		add(d.RolloutStartedAt, styled(renderlog.PlatformStep, deployingLine))
	}
	if d.WebConcurrencyDefault > 0 {
		add(d.RolloutStartedAt, styled(renderlog.PlatformStep, fmt.Sprintf(webConcurrencyLineFormat, d.WebConcurrencyDefault)))
	}
	if d.StartCommand == "" || d.InstanceAnnounces {
		return out
	}
	for _, start := range d.AppStartedAt {
		if !d.FinishedAt.IsZero() && start.After(d.FinishedAt) {
			break
		}
		add(start, runningLine(d.StartCommand))
	}
	return out
}

// runningLine is the narration's copy of the native instance's `==> Running
// '<command>'`, in the instance's own form (renderlog.Instance), as
// native-init writes it for a newer image: a start command whose image
// predates that line, and a pre-deploy step's command, which native-init
// leaves to the narration (w32/m575).
func runningLine(command string) string {
	return styled(renderlog.Instance, fmt.Sprintf(runningLineFormat, command))
}

// Render's port-scan narration (w8/385, w8/389). While a web service listens
// on no port, Render's deploy log repeats the scan line about once a minute,
// five times, then gives up with the timeout line about five minutes in; the
// docs line follows each. The deploy itself runs on to its 15-minute budget.
// The wording and the count are Render's own, read from deploy logs users
// quoted in GitHub issues (docs/ADR018-render-parity.md § Logs, Build logs).
const (
	portScanLine        = "==> No open ports detected, continuing to scan..."
	portScanDocsLine    = "==> Docs on specifying a port: https://render.com/docs/web-services#port-binding"
	portScanTimeoutLine = "==> Port scan timeout reached, no open ports detected. Bind your service to at least one port. " +
		"If you don't need to receive traffic on any port, create a background worker instead."
	// w8/388: Render's wording when the scan finds listeners only on
	// localhost (rkendel1/freq#97, 2026-01) and when its open ports answer
	// no HTTP (docusealco/docuseal#684, 2026-05; mx-space/core#2762,
	// 2026-07). The no-HTTP lines carry no docs line (langflow-ai/langflow#3151).
	portScanLocalhostLine        = "==> No open ports detected on 0.0.0.0, continuing to scan..."
	portScanLocalhostTimeoutLine = "==> Port scan timeout reached, no open ports detected on 0.0.0.0. " +
		"Detected open ports on localhost -- did you mean to bind one of these to 0.0.0.0?"
	portScanNoHTTPLine        = "==> No open HTTP ports detected on 0.0.0.0, continuing to scan..."
	portScanNoHTTPTimeoutLine = "==> Port scan timeout reached, no open HTTP ports detected. " +
		"If you don't need to receive public HTTP traffic, create a private service instead."
	// w8/416: once a web service sets PORT, Render scans that port alone and
	// names it (community.render.com/t/21740, 2024-05;
	// htdio-stg/deploy-templates#1, 2025-02), with no docs line.
	portScanEnvPortLine        = "==> Continuing to scan for open port %d (from PORT environment variable)..."
	portScanEnvPortTimeoutLine = "==> Port scan timeout reached, failed to detect open port %d from PORT environment variable. " +
		"Bind your service to port %d or update the PORT environment variable to the correct port."
	portScanInterval = time.Minute
	portScanMaxLines = 5
)

// portScanWording is one of Render's scan narrations: the line it repeats,
// its timeout line, the docs line that follows them ("" for none), and the
// form its timeout line and that line's docs line take (w32/m576). Render's
// log API returns the plain wording's timeout and docs lines in red
// (kush252/Sentry-AI, Ishan2036924/upmyrank-poc). No capture shows the codes
// on a scan line, on the docs line after it, which Render colours as the
// line before it, or on the other wordings' timeouts, so those stay plain
// (w32/085).
type portScanWording struct {
	scan, timeout, docs string
	timeoutForm         renderlog.Form
}

// entries is line at at, followed by w's docs line when w has one, both in
// form f.
func (w portScanWording) entries(id string, at time.Time, line string, f renderlog.Form) []LogEntry {
	out := []LogEntry{progressEntry(id, at, styled(f, line))}
	if w.docs != "" {
		out = append(out, progressEntry(id, at, styled(f, w.docs)))
	}
	return out
}

var (
	portScanPlain    = portScanWording{portScanLine, portScanTimeoutLine, portScanDocsLine, renderlog.PlatformFailure}
	portScanWordings = map[appv1alpha1.PortScanFinding]portScanWording{
		appv1alpha1.PortScanFoundLocalhost: {portScanLocalhostLine, portScanLocalhostTimeoutLine, portScanDocsLine, renderlog.Plain},
		appv1alpha1.PortScanFoundNoHTTP:    {portScanNoHTTPLine, portScanNoHTTPTimeoutLine, "", renderlog.Plain},
	}
)

// portScanWordingAt is the narration of d's line at t: its scan's finding's
// from the moment the finding was recorded, the plain one before, so a line
// already read keeps its text.
func portScanWordingAt(d DeployProgress, t time.Time) portScanWording {
	if d.PortScanFound == "" || t.Before(d.PortScanFoundAt) {
		return portScanPlain
	}
	if port := d.PortScanFound.EnvPort(); port > 0 {
		return portScanWording{scan: fmt.Sprintf(portScanEnvPortLine, port), timeout: fmt.Sprintf(portScanEnvPortTimeoutLine, port, port),
			timeoutForm: renderlog.Plain}
	}
	if w, ok := portScanWordings[d.PortScanFound]; ok {
		return w
	}
	return portScanPlain
}

// portScanLines are the scan lines d has earned: one when its stall first
// said no port was open, then one a minute while the scan runs, at most five,
// each followed by the docs line, then the timeout line and the docs line a
// minute after the fifth. The scan ends when a stall says it is over or the
// deploy ends, and an open one runs up to the read. After the timeout line
// the rollout still serves a port found before its budget runs out; nothing
// more is written until the deploy ends. A deploy that failed with its scan
// still running before the timeout line was due gets it at the moment it
// failed, before the failure line. Each line is worded for what the scan had
// found by its moment (portScanWordingAt).
func portScanLines(d DeployProgress) []LogEntry {
	since := d.PortScanSince
	if since.IsZero() {
		return nil
	}
	due := func(t time.Time) bool { return !t.After(d.ReadAt) }
	switch {
	case !d.PortScanUntil.IsZero():
		due = func(t time.Time) bool { return t.Before(d.PortScanUntil) }
	case !d.FinishedAt.IsZero():
		due = func(t time.Time) bool { return t.Before(d.FinishedAt) }
	}
	var out []LogEntry
	for i := range portScanMaxLines {
		at := since.Add(time.Duration(i) * portScanInterval)
		if !due(at) {
			break
		}
		w := portScanWordingAt(d, at)
		out = append(out, w.entries(d.ID, at, w.scan, renderlog.Plain)...)
	}
	timeoutAt := since.Add(portScanMaxLines * portScanInterval)
	if !due(timeoutAt) {
		if d.Status != "update_failed" || !d.PortScanUntil.IsZero() || d.FinishedAt.Before(since) {
			return out
		}
		timeoutAt = d.FinishedAt
	}
	w := portScanWordingAt(d, timeoutAt)
	return append(out, w.entries(d.ID, timeoutAt, w.timeout, w.timeoutForm)...)
}

// The initial deploy hook's narration (w33/m435). Render runs the hook after
// the first successful deploy, so its lines follow the deploy's live line, and
// a failure is a line in the log, never the deploy's status.
const (
	hookLineRunning   = "==> Running the initial deploy hook"
	hookLineSucceeded = "==> Initial deploy hook succeeded"
	hookLineFailed    = "==> Initial deploy hook failed"
	hookLineRetry     = "==> The deploy stays live. The hook runs again after the next deploy goes live."
)

// hookMoments are the moments of h's lines on d. The operator stamps both in
// whole seconds, so the start is held just after the lines d's live close
// wrote and the end at the close of its second: the hook's own output, which
// the pod stamps in nanoseconds, then reads between the two. finished is zero
// while the hook runs.
func hookMoments(d DeployProgress, h hookRun) (started, finished time.Time) {
	started = h.startedAt
	if live := liveLinesEnd(d).Add(time.Millisecond); started.Before(live) {
		started = live
	}
	if h.finishedAt.IsZero() {
		return started, time.Time{}
	}
	finished = h.finishedAt.Add(time.Second - time.Nanosecond)
	if finished.Before(started) {
		finished = started
	}
	return started, finished
}

// hookLines renders the lines d earned from the initial deploy hook that ran
// once it was live: one when it started, and how it ended (hookMoments).
func hookLines(d DeployProgress, h hookRun) []LogEntry {
	if !h.ranFor(d) {
		return nil
	}
	var out []LogEntry
	add := func(t time.Time, msg string) { out = append(out, progressEntry(d.ID, t, msg)) }
	started, finished := hookMoments(d, h)
	add(started, hookLineRunning)
	if finished.IsZero() {
		return out
	}
	switch h.status {
	case appv1alpha1.PreDeploySucceeded:
		add(finished, hookLineSucceeded)
	case appv1alpha1.PreDeployFailed:
		line := hookLineFailed
		if msg := strings.TrimSpace(h.message); msg != "" {
			line += ": " + msg
		}
		add(finished, line)
		add(finished, hookLineRetry)
	}
	return out
}

// storyEnd is the moment of d's last line once its hook is counted: the
// hook's end line, the zero time while it runs, else the last line d's close
// wrote (zero while d is open).
func storyEnd(d DeployProgress, h hookRun) time.Time {
	if !h.ranFor(d) {
		return liveLinesEnd(d)
	}
	_, finished := hookMoments(d, h)
	return finished
}

// Render's closing lines for a failed deploy, from deploy logs users quoted
// in GitHub issues (docs/ADR018-render-parity.md § Logs, Build logs): the
// verdict carries no cause, and the troubleshooting line follows it in the
// same instant. A deploy that ran out its rollout budget reads `==> Timed Out`
// since 2026-05 (vincentbellemare90-commits/Mon-bot#1, frayel/devagent#38 and
// #62; w8/391). A failed build reads `==> Build failed 😞` after its output
// (thedimlebowski/soakup#35, thephm/the-beans#417, zerot-atomic/
// TaskManagerDevOps#20), and a start command that exited `==> Exited with
// status N` (rkendel1/freq#187 and #191, darthjee/ward#109; w8/393), as does
// a pre-deploy command that exited (w32/m572).
const (
	timedOutLine     = "==> Timed Out"
	buildFailedLine  = "==> Build failed 😞"
	exitedLineFormat = "==> Exited with status %d"
	troubleshootLine = "==> Common ways to troubleshoot your deploy: https://render.com/docs/troubleshooting-deploys"
)

// Render's primary URL block (w32/m567), from deploy logs users quoted in
// GitHub issues (docs/ADR018-render-parity.md § Logs, Build logs): six lines
// after a web service's `==> Your service is live 🎉`, a blank marker that
// keeps its trailing space, a rule of 59 slashes, the URL line between two
// more blank markers, and the rule again (falcon-Z/z0-auth#12,
// lelewithheart/Pixel-Breach#10 and ten more). The URL ends at its host, as
// Render's API reads give it (lss233/kirara-ai#1498, falcon-Z/z0-auth#12);
// the dashboard's viewer links it and adds a slash. A custom domain is the
// primary and counts the service's other domains (barouu78-wq/teranga-ai#417
// and mvryhan-web/video-variator-android#233, `+ 2 more domains`;
// bookorbit/bookorbit#31, `+ 1 more domain`). A static site's log has no
// block (muppetbrown/mana_meeples_singles_market#226).
const (
	primaryURLBlank    = "==> "
	primaryURLRule     = "==> ///////////////////////////////////////////////////////////"
	primaryURLLine     = "==> Available at your primary URL "
	primaryURLLineStep = time.Millisecond // each line follows the one before it by this
)

// primaryURLLines is Render's block after d's live line, from the URLs d's
// row recorded when it went live: the first is the primary, the rest are
// counted. A deploy that did not go live and a row with no record (a static
// site, a private service, a worker, a cron job, a row closed before the
// record existed) get none. Each line is green, as the live line is, the
// blank ones with the marker's space inside its colour (ivanseibel/tretta's
// and kush252/Sentry-AI's captures of Render's log API, w32/m576).
func primaryURLLines(d DeployProgress) []string {
	if (d.Status != "live" && d.Status != "deactivated") || d.FinishedAt.IsZero() || len(d.LiveURLs) == 0 {
		return nil
	}
	url := primaryURLLine + d.LiveURLs[0]
	switch more := len(d.LiveURLs) - 1; more {
	case 0:
	case 1:
		url += " + 1 more domain"
	default:
		url += fmt.Sprintf(" + %d more domains", more)
	}
	block := []string{primaryURLBlank, primaryURLRule, primaryURLBlank, url, primaryURLBlank, primaryURLRule}
	for i, line := range block {
		block[i] = styled(renderlog.PlatformSuccess, line)
	}
	return block
}

// closingLines is a finished deploy's closing line(s): Render's verdict and
// troubleshooting line for a failure Render's logs confirm, and bex's own
// terminalLines for the rest. Render's verdict names no cause; the deploy's
// failureReason and its page's header carry it, as Render shows its own.
//
// A static site's publish Job writes nothing to the build log, so what its
// failing step said comes first, as Render's log carries `==> Publish
// directory <dir> does not exist!` before its verdict (EthanM32122/cloaket#1;
// w8/395). A build's own output is in the log already.
func closingLines(d DeployProgress, serviceType string) []string {
	verdict, form := rendersVerdict(d)
	if verdict == "" {
		return terminalLines(d.Status, serviceType, d.FailureReason)
	}
	var out []string
	if output, ok := appv1alpha1.PublishOutput(strings.TrimSpace(d.FailureReason)); ok && verdict == buildFailedLine {
		out = narrated(strings.Split(output, "\n"))
	}
	return append(out, styled(form, verdict), styled(renderlog.PlatformStep, troubleshootLine))
}

// rendersVerdict is Render's verdict line for a failed deploy whose close
// Render's logs confirm, "" otherwise: a build whose own work failed (not one
// that timed out, hit an infrastructure fault or never ran), a rollout that
// ran out its budget, a start command that exited, not one the kernel killed
// for memory, and a pre-deploy command that exited (preDeployExitStatus).
// Render's lines for a pre-deploy step that never ran its command, an image
// pull failure and an out-of-memory kill are not observed, so those keep
// bex's. form is the line's form (w32/m576): `==> Timed Out` and `==> Exited
// with status N` are red, as Render's log API returns them
// (Ishan2036924/upmyrank-poc, kush252/Sentry-AI); no capture shows the codes
// on `==> Build failed 😞`, which stays plain (w32/085).
func rendersVerdict(d DeployProgress) (verdict string, form renderlog.Form) {
	switch d.Status {
	case "build_failed":
		if appv1alpha1.IsBuildFailed(strings.TrimSpace(d.FailureReason)) {
			return buildFailedLine, renderlog.Plain
		}
	case deployStatusPreDeployFailed:
		if code := preDeployExitStatus(d); code != nil {
			return fmt.Sprintf(exitedLineFormat, *code), renderlog.PlatformFailure
		}
	case "update_failed":
		switch {
		case d.TimedOut:
			return timedOutLine, renderlog.PlatformFailure
		case d.ExitCode != nil && !d.OOMKilled:
			return fmt.Sprintf(exitedLineFormat, *d.ExitCode), renderlog.PlatformFailure
		}
	}
	return "", renderlog.Plain
}

// terminalLines maps a terminal deploy status to bex's closing line(s). An
// empty reason keeps the historical bare line (w1/m48); a non-empty reason is
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
	if first := unmarked(parts[0]); first != "" {
		out = append(out, base+": "+first)
	} else {
		out = append(out, base)
	}
	return append(out, narrated(parts[1:])...)
}

// narrated is each non-empty line as one narration line, with one marker.
func narrated(lines []string) []string {
	var out []string
	for _, line := range lines {
		if line = unmarked(line); line != "" {
			out = append(out, "==> "+line)
		}
	}
	return out
}

// unmarked trims a reason line and drops any "==>" markers it already opens
// with: a native build's failure tail quotes bex's own narration lines
// ("==> Installing Bundler …"), and the marker terminalLines adds must be the
// line's only one (w8/369). A quoted line written in one of Render's coloured
// forms (w32/m576) opens with its marker only once its codes are read, so
// such a line is taken as the text it shows.
func unmarked(line string) string {
	line = strings.TrimSpace(line)
	if shown := strings.TrimSpace(displayText(line)); strings.HasPrefix(shown, "==>") {
		line = shown
	}
	for strings.HasPrefix(line, "==>") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "==>"))
	}
	return line
}

// terminalLine maps a terminal deploy status to its closing line; "" for the
// open states (no line yet) and unknown values (never invent an outcome). A
// cron_job runs its command to completion on a schedule — it is never a served
// "live" service — so a successful deploy narrates that it is scheduled, not
// live (background workers keep the "live" line: they run continuously). A
// static site closes with Render's site wording (w32/081): Render's static
// site logs end `==> Uploading build...` then `==> Your site is live 🎉`,
// where a web service's end `==> Your service is live 🎉`. The service's line
// is green, as Render's log API returns it (ivanseibel/tretta,
// kush252/Sentry-AI; w32/m576); no capture shows the codes on the site's
// line, and the rest are bex's own, so they stay plain.
func terminalLine(status, serviceType string) string {
	switch status {
	case "live", "deactivated":
		switch serviceType {
		case appv1alpha1.TypeCronJob:
			return "==> Cron job deployed — runs on schedule 🎉"
		case appv1alpha1.TypeStaticSite:
			return "==> Your site is live 🎉"
		}
		return styled(renderlog.PlatformSuccess, "==> Your service is live 🎉")
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
		// applies the exact bounds below. A row's primary URL block and its
		// initial deploy hook come after the row itself closed, so it ends
		// with them.
		if ended := storyEnd(d, pc.hook); !q.Since.IsZero() && !ended.IsZero() && ended.Before(q.Since) {
			continue
		}
		if !q.keepPod(d.ID) || (q.Deploy != "" && d.ID != q.Deploy) {
			continue
		}
		rowPC := pc
		if i > 0 {
			// The App's `Ready` condition describes the CURRENT wait, which can
			// only belong to the newest row; an older row must never borrow it.
			rowPC.wait = buildWait{}
		}
		for _, e := range append(progressLines(d, rowPC), hookLines(d, pc.hook)...) {
			if q.keep(e) {
				merged = append(merged, e)
			}
		}
	}
	if len(merged) == len(entries) {
		return entries
	}
	sort.SliceStable(merged, func(i, j int) bool { return core.TimestampLess(merged[i].Timestamp, merged[j].Timestamp) })
	return q.capToLimit(merged)
}

// progressFollower tracks which platform lines a live build tail has already
// emitted, so subscribe-time catch-up, wait-loop transitions, the reads while
// the build streams and the close wait each emit a line exactly once.
type progressFollower struct {
	s        *Service
	q        LogQuery
	resource string
	pc       progressContext
	emitted  map[string]bool
	row      DeployProgress // the deploy row read last: the newest, or the followed one
	// followed is the deploy whose build pod the tail attached to; the reads
	// while the build streams and the close wait read its row, not a newer one.
	followed string
}

// buildOpen reports that the followed deploy has not finished building: its
// row is still created/queued/build_in_progress. A build tail with no pod yet
// (the registry-credentials wait, the workspace/cluster slot caps, the gap
// before the Job exists) waits on it rather than calling the build over.
func (f *progressFollower) buildOpen() bool {
	if f == nil {
		return false
	}
	switch f.row.Status {
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
	if err != nil || len(rows) == 0 || !f.q.keepPod(rows[0].ID) {
		return nil
	}
	f.row = rows[0]
	f.refreshWait(ctx, f.row)
	return f.emitRow(f.row, false, emit)
}

// beginBuild forgets the previous build's rows: one follower serves every
// build of a tail that follows them all (followBuilds), and a row left over
// from the last build would read as this one's.
func (f *progressFollower) beginBuild() {
	if f != nil {
		f.row, f.followed = DeployProgress{}, ""
	}
}

// follow pins the deploy whose build pod the tail attached to: the newest row
// read so far.
func (f *progressFollower) follow() {
	if f != nil {
		f.followed = f.row.ID
	}
}

// emitStarted sends what the followed row has earned while its build output
// is still arriving, so `==> Building from` comes when the row records the
// build's start, not after the build (w8/m159 t009). It holds the row's
// closing lines, which must not print before the build's last line, and stops
// reading once the row has recorded its start: nothing else is due before the
// row closes, which awaitClose reads.
func (f *progressFollower) emitStarted(ctx context.Context, emit func(LogEntry) error) error {
	if f == nil || f.followed == "" || !f.row.StartedAt.IsZero() {
		return nil
	}
	if !f.readFollowed(ctx) {
		return nil
	}
	return f.emitRow(f.row, true, emit)
}

// awaitClose keeps a build tail open once the build's output has ended, until
// the followed row leaves its build, and sends the lines that earns: a failed
// build's verdict, which the reconciler writes only after it sees the build
// Job end. A row that moves on to its rollout has nothing more for a build
// stream; a tail that follows every build (followBuilds) reads its closing
// line on its next pass. With no followed row it reads the newest row once.
func (f *progressFollower) awaitClose(ctx context.Context, emit func(LogEntry) error) error {
	if f == nil {
		return nil
	}
	if f.followed == "" {
		return f.emitReached(ctx, emit)
	}
	deadline := time.Now().Add(f.s.buildCloseWait())
	for {
		if f.readFollowed(ctx) {
			if err := f.emitRow(f.row, false, emit); err != nil {
				return err
			}
			if !f.buildOpen() {
				return nil
			}
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.s.buildPodWait()):
		}
	}
}

// readFollowed re-reads the followed deploy's row into f.row. A newer deploy
// does not displace it: its lines are not this build's.
func (f *progressFollower) readFollowed(ctx context.Context) bool {
	rows, err := f.s.DeployProgress(ctx, f.resource, time.Time{})
	if err != nil {
		return false
	}
	for _, d := range rows {
		if d.ID == f.followed {
			f.row = d
			return true
		}
	}
	return false
}

// emitRow sends the lines d has earned that this tail has not sent yet. With
// holdClosing it keeps back the lines d earned by closing: the verdict at its
// close and the primary URL block after it.
func (f *progressFollower) emitRow(d DeployProgress, holdClosing bool, emit func(LogEntry) error) error {
	hold := holdClosing && !d.FinishedAt.IsZero()
	for _, e := range progressLines(d, f.pc) {
		if hold {
			if at, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil && !at.Before(d.FinishedAt) {
				continue
			}
		}
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
