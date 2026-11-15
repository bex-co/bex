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
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR047 D12 (w22/m413) against the real store: a turn a person stopped keeps
// its disk as a retained image, and the session's next message boots from it.
// The sandbox side is a stand-in behind the same seam; the pause, the copy and
// the stale-file removal are tested in the sandbox package.

type fakeRetainers struct {
	mu   sync.Mutex
	base string
	// blockDelete makes a registry delete hang until its context ends.
	blockDelete bool
	// egressDeleted are the sessions whose egress policy was deleted.
	egressDeleted []string
	// retainErr fails the save; commit is what a good one returns.
	retainErr error
	commit    sandbox.RetainedCommit
	retained  []retainCall
	deleted   []string
	deleteErr error
	// onRetain runs inside the save, before it answers.
	onRetain func(retainCall)
	// events is the order of the saves, boots and deletes, for the races.
	events []string
	// transcriptReads reads the harvest count when a save starts, to show the
	// transcript was read before the pause.
	transcriptReads func() int
}

type retainCall struct {
	workspace, session, sandbox, image string
	transcriptReads                    int
}

func (f *fakeRetainers) RetainAgentSessionSandbox(_ context.Context, ws, session, sandboxID, imageID string) (sandbox.RetainedCommit, error) {
	call := retainCall{workspace: ws, session: session, sandbox: sandboxID, image: imageID}
	if f.transcriptReads != nil {
		call.transcriptReads = f.transcriptReads()
	}
	f.mu.Lock()
	f.retained = append(f.retained, call)
	f.events = append(f.events, "save:"+imageID)
	hook, err, commit := f.onRetain, f.retainErr, f.commit
	f.mu.Unlock()
	if hook != nil {
		hook(call)
	}
	if err != nil {
		return sandbox.RetainedCommit{}, err
	}
	if commit.Digest == "" {
		commit = sandbox.RetainedCommit{AgentImageCommit: sandbox.AgentImageCommit{Digest: testImageDigest, SizeBytes: 4096}, SandboxEnded: true}
	}
	return commit, nil
}

func (f *fakeRetainers) DeleteAgentSessionEgress(_ context.Context, _, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.egressDeleted = append(f.egressDeleted, sessionID)
	f.events = append(f.events, "egress:"+sessionID)
	return nil
}

func (f *fakeRetainers) egressDeletes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.egressDeleted)
}

func (f *fakeRetainers) setBlockDelete(block bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blockDelete = block
}

func (f *fakeRetainers) DeleteAgentRetainedImage(ctx context.Context, _, imageID string) error {
	f.mu.Lock()
	block := f.blockDelete
	f.mu.Unlock()
	if block {
		<-ctx.Done()
		return ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, imageID)
	f.events = append(f.events, "delete:"+imageID)
	return nil
}

func (f *fakeRetainers) AgentTemplateImage(string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.base
}

func (f *fakeRetainers) setBase(base string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.base = base
}

func (f *fakeRetainers) deletions(imageID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, id := range f.deleted {
		if id == imageID {
			n++
		}
	}
	return n
}

func (f *fakeRetainers) log() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.events)
}

func (f *fakeRetainers) mark(event string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
}

type retainEnv struct {
	queueEnv
	clock *schedClock
	rs    *fakeRetainers
	r     *Retention
	c     *Completer
}

const retentionTTL = 24 * time.Hour

func newRetainEnv(t *testing.T) *retainEnv {
	t.Helper()
	prevPoll := retentionWaitPoll
	retentionWaitPoll = time.Millisecond
	t.Cleanup(func() { retentionWaitPoll = prevPoll })
	e := &retainEnv{queueEnv: newQueueEnv(t)}
	e.clock = &schedClock{t: time.Now().UTC().Truncate(time.Millisecond)}
	e.svc.Clock = e.clock.Now
	e.rs = &fakeRetainers{base: "agent:1"}
	e.rs.transcriptReads = func() int { return e.lc.readTranscript }
	e.r = &Retention{Store: e.pg, Sandboxes: e.rs, TTL: retentionTTL}
	e.svc.Retention = e.r
	e.c = &Completer{Store: e.pg, Sandbox: e.lc, Retention: e.r, Now: e.clock.Now, Metrics: NewCompletionMetrics(prometheus.NewRegistry()),
		retainRunner: func(fn func()) { fn() }}
	return e
}

// stop asks to stop the running turn, as a person does, and has the driver
// report it stopped.
func (e *retainEnv) stop(t *testing.T) {
	t.Helper()
	if _, err := e.svc.Interrupt(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	e.lc.status = interruptedStatus(statusGit{})
}

func (e *retainEnv) session(t *testing.T) store.AgentSession {
	t.Helper()
	row, err := e.pg.GetAgentSession(context.Background(), e.id)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

// settle has the Completer read the driver's status and settle the turn, which
// for a stopped turn runs its save inline.
func (e *retainEnv) settle(t *testing.T) {
	t.Helper()
	e.c.finalize(context.Background(), e.session(t))
}

// stopped is a session whose turn a person stopped and whose disk is kept.
func (e *retainEnv) stopped(t *testing.T) store.AgentRetainedImage {
	t.Helper()
	e.stop(t)
	e.settle(t)
	return e.live(t)
}

func (e *retainEnv) live(t *testing.T) store.AgentRetainedImage {
	t.Helper()
	img, err := e.pg.AgentSessionRetainedImage(context.Background(), e.id)
	if err != nil {
		t.Fatalf("the session's retained image: %v", err)
	}
	return img
}

func (e *retainEnv) noImage(t *testing.T) {
	t.Helper()
	if img, err := e.pg.AgentSessionRetainedImage(context.Background(), e.id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("retained image = %+v / %v, want none", img, err)
	}
}

func (e *retainEnv) handles(t *testing.T) int {
	t.Helper()
	all, err := e.pg.AgentSessionRetainedImages(context.Background(), e.id)
	if err != nil {
		t.Fatal(err)
	}
	return len(all)
}

func (e *retainEnv) turn(t *testing.T, n int) store.AgentSessionTurn {
	t.Helper()
	turns, err := e.pg.AgentSessionTurns(context.Background(), e.id)
	if err != nil || len(turns) < n {
		t.Fatalf("turns = %v / %v", turns, err)
	}
	return turns[n-1]
}

// sweep is the Completer's pass over retained images: expiry, then the purge.
func (e *retainEnv) sweep() { e.c.reapRetainedImages(context.Background()) }

// holdSave makes the next save block inside the sandbox seam, run on a
// goroutine as in production, and returns a channel that closes once it is
// there and a function that lets it finish and waits for it.
func (e *retainEnv) holdSave() (entered <-chan struct{}, finish func()) {
	in, out := make(chan struct{}), make(chan struct{})
	e.rs.onRetain = func(retainCall) {
		close(in)
		<-out
	}
	e.c.retainRunner = nil // a goroutine, tracked by retainWG
	return in, func() {
		close(out)
		e.c.retainWG.Wait()
	}
}

func (e *retainEnv) results(result retentionResult) float64 {
	return testutil.ToFloat64(e.c.Metrics.retentions.WithLabelValues(string(result)))
}

// A stopped turn settles as completed at once, and its disk is then saved:
// the transcript is read before the pause, the paused sandbox is ended by the
// save and not by the idle reaper, and the image is recorded as kept for the
// retention time, on the session and on the turn.
func TestAStoppedTurnKeepsItsDisk(t *testing.T) {
	e := newRetainEnv(t)
	e.lc.transcriptLog = `{"turn":1,"partIndex":0,"part":{"type":"text","text":"half an answer"}}`
	e.stop(t)
	before := e.session(t)
	e.settle(t)

	row := e.session(t)
	if row.Phase != PhaseCompleted || !steerable(row.Phase, false) || !e.turn(t, 1).Interrupted {
		t.Fatalf("the stopped turn settled as %+v", row)
	}
	img := e.live(t)
	if img.State != store.RetainedImageKept || img.ImageDigest != testImageDigest || img.Turn != 1 || img.BaseImage != "agent:1" ||
		img.RetainedUntil == nil || !img.RetainedUntil.Equal(e.clock.Now().Add(retentionTTL)) {
		t.Fatalf("retained image = %+v", img)
	}
	if len(e.rs.retained) != 1 {
		t.Fatalf("saves = %+v", e.rs.retained)
	}
	call := e.rs.retained[0]
	if call.session != e.id || call.sandbox != before.SandboxID || call.image != img.ID || call.workspace != e.ws {
		t.Fatalf("the save = %+v, want the session's sandbox %q saved under %s", call, before.SandboxID, img.ID)
	}
	if call.transcriptReads != 1 {
		t.Fatalf("the transcript was read %d times before the pause, want once", call.transcriptReads)
	}
	if parts, err := e.pg.AgentSessionTranscript(context.Background(), e.id, -1, 1<<30, 0); err != nil || len(parts) != 1 {
		t.Fatalf("the partial reply is not in the transcript: %v %v", parts, err)
	}
	// The save ended the sandbox: no ordinary teardown ran, and its id is gone.
	if e.lc.canceled != 0 || row.SandboxID != "" {
		t.Fatalf("canceled=%d sandboxID=%q, want the save to have ended the sandbox", e.lc.canceled, row.SandboxID)
	}
	turn := e.turn(t, 1)
	if turn.Retention != store.TurnRetentionKept || turn.RetainedUntil == nil || !turn.RetainedUntil.Equal(*img.RetainedUntil) {
		t.Fatalf("turn = %+v, want its disk recorded as kept until %v", turn, img.RetainedUntil)
	}
	if e.results(retentionKept) != 1 {
		t.Fatalf("kept saves counted %v", e.results(retentionKept))
	}
	// The next pass finds nothing more to do.
	e.c.Reconcile(context.Background())
	if len(e.rs.retained) != 1 {
		t.Fatalf("a settled turn was saved again: %d", len(e.rs.retained))
	}
}

// A turn that ended on its own, completed or failed, keeps nothing: its
// sandbox follows the ordinary teardown.
func TestACompletedOrFailedTurnKeepsNothing(t *testing.T) {
	for name, status := range map[string]string{
		"completed": `{"state":"succeeded"}`,
		"failed":    `{"state":"failed","error":"the agent crashed"}`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetainEnv(t)
			e.lc.status = status
			e.settle(t)
			if len(e.rs.retained) != 0 || e.handles(t) != 0 {
				t.Fatalf("a %s turn was saved: %+v", name, e.rs.retained)
			}
			if e.lc.canceled != 1 {
				t.Fatalf("canceled = %d, want the ordinary teardown", e.lc.canceled)
			}
			if turn := e.turn(t, 1); turn.Retention != "" {
				t.Fatalf("turn = %+v", turn)
			}
		})
	}
}

// With retention off, or not wired, a stopped turn behaves as it did before.
func TestAStoppedTurnKeepsNothingWhenRetentionIsOff(t *testing.T) {
	for name, set := range map[string]func(e *retainEnv){
		"a zero retention time": func(e *retainEnv) { e.r.TTL = 0 },
		"no retention":          func(e *retainEnv) { e.c.Retention, e.svc.Retention = nil, nil },
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetainEnv(t)
			set(e)
			e.stop(t)
			e.settle(t)
			if row := e.session(t); row.Phase != PhaseCompleted || !e.turn(t, 1).Interrupted {
				t.Fatalf("session = %+v", row)
			}
			if len(e.rs.retained) != 0 || e.handles(t) != 0 || e.lc.canceled != 1 {
				t.Fatalf("saves %+v handles %d canceled %d, want today's teardown", e.rs.retained, e.handles(t), e.lc.canceled)
			}
		})
	}
}

// An interrupt nobody requested is not believed, so it keeps nothing either.
func TestAnUnrequestedInterruptKeepsNothing(t *testing.T) {
	e := newRetainEnv(t)
	e.lc.status = interruptedStatus(statusGit{})
	e.settle(t)
	if len(e.rs.retained) != 0 || e.handles(t) != 0 {
		t.Fatalf("an unrequested interrupt was saved: %+v", e.rs.retained)
	}
}

// A workspace at its storage cap keeps no disk, and the user is shown no
// error: the loss is recorded on the turn, the session is settled, and the
// sandbox follows today's lifecycle (the idle grace, then the reaper).
func TestAFullStorageCapDegradesToTodaysBehavior(t *testing.T) {
	e := newRetainEnv(t)
	e.rs.retainErr = fmt.Errorf("%w: full", sandbox.ErrAgentImageStorageLimit)
	e.stop(t)
	before := e.session(t)
	e.settle(t)
	row := e.session(t)
	if row.Phase != PhaseCompleted || row.FailureReason != "" || !e.turn(t, 1).Interrupted {
		t.Fatalf("a refused save changed the turn's outcome: %+v", row)
	}
	e.noImage(t)
	if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionLost {
		t.Fatalf("turn = %+v, want the loss recorded", turn)
	}
	if e.results(retentionStorageFull) != 1 {
		t.Fatalf("storage_limit saves counted %v", e.results(retentionStorageFull))
	}
	// The sandbox was left as it is today: the idle reaper owns it.
	if row.SandboxID != before.SandboxID || e.lc.canceled != 0 {
		t.Fatalf("sandbox %q canceled %d, want it left to the idle reaper", row.SandboxID, e.lc.canceled)
	}
	e.c.reapIdleSandboxes(context.Background())
	if e.lc.canceled != 1 || e.session(t).SandboxID != "" {
		t.Fatalf("canceled = %d: the idle reaper did not end it", e.lc.canceled)
	}
	// The handle that reserved nothing is removed by the purge.
	e.sweep()
	if e.handles(t) != 0 {
		t.Fatalf("the released handle was left behind")
	}
	// The next message starts fresh.
	e.send(t, "alice", "carry on")
	if e.lc.image != (sandbox.AgentImage{}) {
		t.Fatalf("the next turn booted from %+v with nothing kept", e.lc.image)
	}
}

// A pause that fails after it began ends the sandbox, which cannot run its
// turn again, and records the loss; the turn itself is unaffected.
func TestAFailedPauseEndsTheSandboxAndRecordsTheLoss(t *testing.T) {
	e := newRetainEnv(t)
	e.rs.retainErr = fmt.Errorf("%w: the commit did not finish", sandbox.ErrAgentSandboxPaused)
	e.stop(t)
	e.settle(t)
	row := e.session(t)
	if row.Phase != PhaseCompleted || row.SandboxID != "" {
		t.Fatalf("session = %+v, want completed with the ended sandbox's id cleared", row)
	}
	e.noImage(t)
	if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionLost {
		t.Fatalf("turn = %+v", turn)
	}
	if e.results(retentionFailed) != 1 {
		t.Fatalf("failed saves counted %v", e.results(retentionFailed))
	}
}

// A transcript that cannot be read keeps nothing: the pod is about to go, and
// the reply is in no other place.
func TestATranscriptThatCannotBeReadKeepsNothing(t *testing.T) {
	e := newRetainEnv(t)
	e.lc.transcriptErr = errors.New("the pod did not answer")
	e.stop(t)
	e.settle(t)
	if len(e.rs.retained) != 0 {
		t.Fatalf("the sandbox was paused without its transcript: %+v", e.rs.retained)
	}
	e.noImage(t)
	if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionLost {
		t.Fatalf("turn = %+v", turn)
	}
	if e.lc.canceled != 0 {
		t.Fatalf("canceled = %d: the sandbox is left to the idle reaper", e.lc.canceled)
	}
}

// The retention time is the clock the image lives by: past it the sweep
// releases and deletes the image, the handle goes, and the turn records the
// loss. Before it nothing happens.
func TestAKeptDiskExpiresAndIsDeleted(t *testing.T) {
	e := newRetainEnv(t)
	img := e.stopped(t)
	e.clock.Advance(retentionTTL - time.Minute)
	e.sweep()
	if e.rs.deletions(img.ID) != 0 || e.live(t).ID != img.ID {
		t.Fatalf("an image was deleted before its time")
	}
	e.clock.Advance(2 * time.Minute)
	e.sweep()
	e.noImage(t)
	if e.rs.deletions(img.ID) != 1 || e.handles(t) != 0 {
		t.Fatalf("deleted %d times, handles %d, want the image deleted once and its handle gone", e.rs.deletions(img.ID), e.handles(t))
	}
	if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionLost {
		t.Fatalf("turn = %+v", turn)
	}
	// The next message simply starts fresh.
	e.send(t, "alice", "carry on")
	if e.lc.image != (sandbox.AgentImage{}) || e.phase(t, e.id) != PhaseRunning {
		t.Fatalf("after the expiry: image %+v phase %s", e.lc.image, e.phase(t, e.id))
	}
}

// The setting never reaches Devin's 14 days: a longer one is cut to the cap.
func TestTheRetentionTimeIsCapped(t *testing.T) {
	e := newRetainEnv(t)
	e.r.TTL = 30 * 24 * time.Hour
	img := e.stopped(t)
	if got := img.RetainedUntil.Sub(e.clock.Now()); got != MaxRetentionTTL {
		t.Fatalf("kept for %v, want the %v cap", got, MaxRetentionTTL)
	}
	if MaxRetentionTTL >= 14*24*time.Hour {
		t.Fatalf("the cap %v is not shorter than Devin's 14 days", MaxRetentionTTL)
	}
}

// Every way a kept disk can end deletes the image from the registry and
// removes its handle: cancel, delete, archive, expiry and the continuation
// itself each have a case, and each is deleted once.
func TestEveryEndOfAKeptDiskDeletesTheImage(t *testing.T) {
	ctx := caller("alice")
	cases := map[string]func(e *retainEnv, t *testing.T){
		"cancel": func(e *retainEnv, t *testing.T) {
			if _, err := e.svc.Cancel(ctx, e.id); err != nil {
				t.Fatal(err)
			}
		},
		"archive": func(e *retainEnv, t *testing.T) {
			if _, err := e.svc.Archive(ctx, e.id); err != nil {
				t.Fatal(err)
			}
		},
		"delete": func(e *retainEnv, t *testing.T) {
			if err := e.svc.Delete(ctx, e.id); err != nil {
				t.Fatal(err)
			}
		},
		"expiry": func(e *retainEnv, t *testing.T) {
			e.clock.Advance(retentionTTL + time.Minute)
			e.sweep()
		},
		"continuation": func(e *retainEnv, t *testing.T) {
			e.send(t, "alice", "carry on")
		},
		"a turn that started fresh": func(e *retainEnv, t *testing.T) {
			// The image is not used (a different agent image), and what
			// remains of it is deleted, not left to expire.
			e.rs.setBase("agent:2")
			e.send(t, "alice", "carry on")
		},
	}
	for name, end := range cases {
		t.Run(name, func(t *testing.T) {
			e := newRetainEnv(t)
			img := e.stopped(t)
			end(e, t)
			// The verb itself deletes it, except for the delete, which removes
			// the handle with the session; a sweep then finds nothing more.
			if got := e.rs.deletions(img.ID); got != 1 {
				t.Fatalf("the image was deleted %d times by the verb, want once", got)
			}
			e.sweep()
			if got := e.rs.deletions(img.ID); got != 1 {
				t.Fatalf("the image was deleted %d times after a sweep, want once", got)
			}
			if name != "delete" && e.handles(t) != 0 {
				t.Fatalf("the handle outlived its image")
			}
		})
	}
}

// A failed delete leaves the handle, which names an image the registry still
// holds, and the sweep tries again; the Delete verb refuses to remove the
// session while it would orphan one.
func TestAFailedImageDeleteKeepsTheHandle(t *testing.T) {
	e := newRetainEnv(t)
	img := e.stopped(t)
	e.rs.deleteErr = errors.New("the registry is down")
	if err := e.svc.Delete(caller("alice"), e.id); err == nil {
		t.Fatalf("the session was deleted while its image could not be")
	}
	if _, err := e.pg.GetAgentSession(context.Background(), e.id); err != nil {
		t.Fatalf("the session is gone: %v", err)
	}
	if e.handles(t) != 1 {
		t.Fatalf("the handle went before its image")
	}
	e.rs.deleteErr = nil
	if err := e.svc.Delete(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if e.rs.deletions(img.ID) != 1 {
		t.Fatalf("the image was deleted %d times", e.rs.deletions(img.ID))
	}

	// The sweep backs a failed delete off and then completes it.
	e = newRetainEnv(t)
	img = e.stopped(t)
	e.rs.deleteErr = errors.New("the registry is down")
	if _, err := e.svc.Cancel(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if e.handles(t) != 1 {
		t.Fatalf("the handle went with a failed delete")
	}
	e.rs.deleteErr = nil
	e.clock.Advance(time.Hour)
	e.sweep()
	if e.rs.deletions(img.ID) != 1 || e.handles(t) != 0 {
		t.Fatalf("after the registry came back: deleted %d, handles %d", e.rs.deletions(img.ID), e.handles(t))
	}
}

// A sandbox whose save is in progress is not ended by the idle reaper, even
// with no idle grace, and the save finishes the job.
func TestTheIdleReaperLeavesASandboxBeingSaved(t *testing.T) {
	e := newRetainEnv(t)
	e.stop(t)
	entered, finish := e.holdSave()
	e.settle(t)
	<-entered
	// The turn has settled, and its image is being saved.
	if row := e.session(t); row.Phase != PhaseCompleted || row.SandboxID == "" {
		t.Fatalf("session = %+v", row)
	}
	if img := e.live(t); img.State != store.RetainedImageSaving {
		t.Fatalf("image = %+v, want saving", img)
	}
	e.c.reapIdleSandboxes(context.Background())
	if e.lc.canceled != 0 {
		t.Fatalf("the idle reaper ended a sandbox that is being saved")
	}
	finish()
	if img := e.live(t); img.State != store.RetainedImageKept {
		t.Fatalf("image = %+v, want kept", img)
	}
}

// Stop shows at once: the session is completed and flagged while the save
// still runs, and a message sent meanwhile is not queued behind it but waits
// for the save and then continues from the image.
func TestAMessageSentWhileTheDiskIsSavedWaitsForItAndContinuesFromIt(t *testing.T) {
	e := newRetainEnv(t)
	e.stop(t)
	entered, finish := e.holdSave()
	e.settle(t)
	<-entered
	saving := e.live(t)

	// The settle did not wait for the pause.
	if row := e.session(t); row.Phase != PhaseCompleted || !e.turn(t, 1).Interrupted {
		t.Fatalf("the stop did not show while the save ran: %+v", row)
	}
	var sendErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, sendErr = e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "continue from where you were"})
	}()
	// The dispatch is waiting: nothing has been created or torn down.
	time.Sleep(50 * time.Millisecond)
	if e.lc.created != 1 || e.lc.canceled != 0 {
		t.Fatalf("created=%d canceled=%d while the save ran: the next turn did not wait", e.lc.created, e.lc.canceled)
	}
	finish()
	<-done
	if sendErr != nil {
		t.Fatal(sendErr)
	}
	if e.lc.image.ID != saving.ID || e.lc.image.Digest != testImageDigest {
		t.Fatalf("the next turn booted from %+v, want the saved image %s", e.lc.image, saving.ID)
	}
	if e.lc.created != 2 || e.phase(t, e.id) != PhaseRunning {
		t.Fatalf("created=%d phase=%s", e.lc.created, e.phase(t, e.id))
	}
}

// A save that does not finish by its deadline is not waited for: the next turn
// starts fresh. The save, when it does end, touches nothing of that turn: its
// image is superseded and deleted, the new turn's sandbox id is not blanked
// and its egress policy is not deleted with the sandbox the save ended.
func TestAMessageDoesNotWaitForeverForASave(t *testing.T) {
	e := newRetainEnv(t)
	e.stop(t)
	entered, finish := e.holdSave()
	e.settle(t)
	<-entered
	saving := e.live(t)
	// The deadline passes with the save still running.
	e.clock.Advance(retentionSaveWindow() + time.Second)
	e.send(t, "alice", "continue")
	if e.lc.image != (sandbox.AgentImage{}) || e.phase(t, e.id) != PhaseRunning {
		t.Fatalf("the next turn booted from %+v, want a fresh sandbox", e.lc.image)
	}
	fresh := e.session(t)
	if fresh.SandboxID == "" || fresh.Turns != 2 {
		t.Fatalf("the fresh turn's session = %+v", fresh)
	}
	// The save ends afterwards: the turn that started fresh superseded it, so
	// it is not kept, and the image it wrote is deleted.
	finish()
	e.noImage(t)
	if e.rs.deletions(saving.ID) < 1 {
		t.Fatalf("the late save's image %s was not deleted", saving.ID)
	}
	if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionLost {
		t.Fatalf("turn = %+v", turn)
	}
	// The save ended the first turn's sandbox and has nothing to do with the
	// fresh one.
	if after := e.session(t); after.SandboxID != fresh.SandboxID || after.Phase != PhaseRunning {
		t.Fatalf("the late save changed the fresh turn's session: %+v, was %+v", after, fresh)
	}
	if got := e.rs.egressDeletes(); len(got) != 0 {
		t.Fatalf("the late save deleted the session's egress policy %v under the fresh turn", got)
	}
	// The fresh turn is still read by the Completer: its sandbox id is there.
	e.lc.status = `{"state":"succeeded"}`
	e.settle(t)
	if row := e.session(t); row.Phase != PhaseCompleted {
		t.Fatalf("the fresh turn was stranded: %+v", row)
	}
}

// The next turn stops waiting only after the save and everything it does once
// it has ended: the pause and the copy, the end of the sandbox, and the
// records written afterwards. A deadline any shorter lets a Steer start fresh
// while the save is still cleaning up the session it left.
func TestTheSaveDeadlineOutlastsTheSaveAndItsCleanup(t *testing.T) {
	e := newRetainEnv(t)
	e.stop(t)
	entered, finish := e.holdSave()
	e.settle(t)
	<-entered
	img := e.live(t)
	finish()
	want := retentionSaveTimeout + sandbox.RetainEndTimeout + retentionSettleTimeout
	if got := img.SaveDeadline.Sub(img.CreatedAt); got < want-time.Second || retentionSaveWindow() != want {
		t.Fatalf("the save deadline is %v after it began, window %v, want %v", got, retentionSaveWindow(), want)
	}
}

// A stopped session's egress policy is deleted with its sandbox when no newer
// turn has started, as the ordinary teardown does, so none is left behind.
func TestTheSaveDeletesTheEgressPolicyOfASessionNoOneContinued(t *testing.T) {
	e := newRetainEnv(t)
	e.stopped(t)
	if got := e.rs.egressDeletes(); len(got) != 1 || got[0] != e.id {
		t.Fatalf("egress policy deletes = %v, want the session's once", got)
	}
}

// A Cancel that arrives while the save runs releases the retention; the save
// then finds it released, and deletes the image it just wrote.
func TestACancelDuringTheSaveDeletesTheSavedImage(t *testing.T) {
	e := newRetainEnv(t)
	e.stop(t)
	entered, finish := e.holdSave()
	e.settle(t)
	<-entered
	saving := e.live(t)
	if _, err := e.svc.Cancel(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	finish()
	e.noImage(t)
	e.sweep()
	if e.rs.deletions(saving.ID) < 1 || e.handles(t) != 0 {
		t.Fatalf("deleted %d times, handles %d, want the image gone and no handle", e.rs.deletions(saving.ID), e.handles(t))
	}
}

// --- the next turn continues from the image (w22/m413 t003) -----------------

// A direct send on a session whose stopped turn's disk is kept boots its
// sandbox from that image, with the new turn's own prompt and none of the
// repository environment's prebuilt marker; once the sandbox is up the image
// is spent and deleted, and the turn records that it continued.
func TestASendContinuesFromTheKeptDisk(t *testing.T) {
	e := newRetainEnv(t)
	e.lc.transcriptLog = `{"turn":1,"partIndex":0,"part":{"type":"text","text":"half an answer"}}`
	img := e.stopped(t)
	view := e.send(t, "alice", "continue from where you were")
	if view.Phase != PhaseRedispatching && view.Phase != PhaseRunning {
		t.Fatalf("view = %+v", view)
	}
	if e.lc.image != (sandbox.AgentImage{ID: img.ID, Digest: testImageDigest}) {
		t.Fatalf("the sandbox booted from %+v, want the kept image %s", e.lc.image, img.ID)
	}
	if e.lc.driverEnv["BEX_AGENT_PROMPT"] != "continue from where you were" || e.lc.driverEnv[prebuiltEnv] != "" {
		t.Fatalf("driver env = %v", e.lc.driverEnv)
	}
	if e.lc.created != 2 || e.phase(t, e.id) != PhaseRunning {
		t.Fatalf("created=%d phase=%s", e.lc.created, e.phase(t, e.id))
	}
	e.noImage(t)
	if e.rs.deletions(img.ID) != 1 || e.handles(t) != 0 {
		t.Fatalf("after the continuation the image was deleted %d times with %d handles", e.rs.deletions(img.ID), e.handles(t))
	}
	if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionContinued {
		t.Fatalf("turn 1 = %+v, want continued", turn)
	}
	if row := e.session(t); row.EnvironmentBuildID != "" {
		t.Fatalf("environmentBuildId = %q: a retained disk is no build's image", row.EnvironmentBuildID)
	}
	// The conversation context still reaches the new turn through the continuity
	// env, as for every turn.
	if e.lc.driverEnv[continuityContextEnv] == "" {
		t.Fatalf("the new turn carries no continuity: %v", e.lc.driverEnv)
	}
	// A second stop keeps the disk of the continued turn, and only that one.
	e.stop(t)
	e.settle(t)
	if second := e.live(t); second.ID == img.ID || second.Turn != 2 {
		t.Fatalf("second retained image = %+v", second)
	}
}

// A message queued while the turn ran continues from the kept disk too: the
// stop, the save and the dispatcher compose.
func TestAQueuedMessageContinuesFromTheKeptDisk(t *testing.T) {
	e := newRetainEnv(t)
	e.send(t, "alice", "now, please")
	if got := e.pending(t); len(got) != 1 {
		t.Fatalf("pending = %v", got)
	}
	e.stop(t)
	e.settle(t)
	img := e.live(t)
	if err := e.dispatcher().RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "now, please" {
		t.Fatalf("turns = %v, want the queued message to have run", got)
	}
	if e.lc.image != (sandbox.AgentImage{ID: img.ID, Digest: testImageDigest}) {
		t.Fatalf("the queued turn booted from %+v, want the kept image %s", e.lc.image, img.ID)
	}
	if e.rs.deletions(img.ID) != 1 {
		t.Fatalf("the image was deleted %d times", e.rs.deletions(img.ID))
	}
}

// Nothing to continue from is not an error: no image, an expired one, one
// saved on an older agent image, all start fresh, and the last two are deleted.
func TestAMissingExpiredOrOutdatedDiskStartsFresh(t *testing.T) {
	t.Run("nothing kept", func(t *testing.T) {
		e := newRetainEnv(t)
		e.lc.status = `{"state":"succeeded"}`
		e.settle(t)
		e.send(t, "alice", "next")
		if e.lc.image != (sandbox.AgentImage{}) || e.phase(t, e.id) != PhaseRunning {
			t.Fatalf("image %+v phase %s", e.lc.image, e.phase(t, e.id))
		}
	})
	t.Run("expired", func(t *testing.T) {
		e := newRetainEnv(t)
		img := e.stopped(t)
		// The retention time passes before the sweep runs: the claim itself reads
		// the clock, so the image is not used.
		e.clock.Advance(retentionTTL + time.Minute)
		e.send(t, "alice", "next")
		if e.lc.image != (sandbox.AgentImage{}) || e.phase(t, e.id) != PhaseRunning {
			t.Fatalf("an expired image was used: %+v", e.lc.image)
		}
		e.sweep()
		if e.rs.deletions(img.ID) != 1 || e.handles(t) != 0 {
			t.Fatalf("deleted %d times, handles %d", e.rs.deletions(img.ID), e.handles(t))
		}
	})
	t.Run("an older agent image", func(t *testing.T) {
		e := newRetainEnv(t)
		img := e.stopped(t)
		e.rs.setBase("agent:2")
		e.send(t, "alice", "next")
		if e.lc.image != (sandbox.AgentImage{}) || e.phase(t, e.id) != PhaseRunning {
			t.Fatalf("an older driver was booted: %+v", e.lc.image)
		}
		if e.rs.deletions(img.ID) != 1 || e.handles(t) != 0 {
			t.Fatalf("deleted %d times, handles %d", e.rs.deletions(img.ID), e.handles(t))
		}
		if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionLost {
			t.Fatalf("turn 1 = %+v", turn)
		}
	})
}

// A sandbox that cannot start from the kept disk is retried once from a fresh
// one and the turn says so; the turn is never failed for the optimization. If
// the fresh sandbox does not start either the disk was not at fault and it is
// kept for the next message; a capacity limit is not retried at all.
func TestASandboxThatCannotStartFromTheDiskStartsFresh(t *testing.T) {
	t.Run("the retry starts", func(t *testing.T) {
		e := newRetainEnv(t)
		img := e.stopped(t)
		e.lc.imageCreateErr = errors.New("ImagePullBackOff")
		e.send(t, "alice", "continue")
		if e.lc.image != (sandbox.AgentImage{}) || e.phase(t, e.id) != PhaseRunning || e.lc.created != 2 {
			t.Fatalf("image %+v phase %s created %d, want one fresh sandbox", e.lc.image, e.phase(t, e.id), e.lc.created)
		}
		if note := e.turn(t, 2).StartNote; note != retainedFresh {
			t.Fatalf("start note = %q", note)
		}
		if e.rs.deletions(img.ID) != 1 || e.handles(t) != 0 {
			t.Fatalf("the unusable image was deleted %d times, handles %d", e.rs.deletions(img.ID), e.handles(t))
		}
		// The turn says so when it finishes, before what it left undelivered.
		e.lc.status = `{"state":"succeeded","git":{"undelivered":["a note"]}}`
		e.settle(t)
		if got := e.session(t).DeliveryNote; got != retainedFresh+" a note" {
			t.Fatalf("delivery note = %q", got)
		}
	})
	t.Run("the retry fails too", func(t *testing.T) {
		e := newRetainEnv(t)
		img := e.stopped(t)
		e.lc.createErr = errors.New("the substrate is down")
		e.send(t, "alice", "continue")
		if e.phase(t, e.id) != PhaseFailed {
			t.Fatalf("phase = %s", e.phase(t, e.id))
		}
		// The disk was not at fault: it is still kept for the next message.
		if got := e.live(t); got.ID != img.ID || got.State != store.RetainedImageKept {
			t.Fatalf("image = %+v, want let go to kept", got)
		}
		if e.rs.deletions(img.ID) != 0 {
			t.Fatalf("a good image was deleted")
		}
		e.lc.createErr = nil
		e.send(t, "alice", "try again")
		if e.lc.image.ID != img.ID {
			t.Fatalf("the next message booted from %+v, want the still-kept %s", e.lc.image, img.ID)
		}
	})
	t.Run("a capacity limit", func(t *testing.T) {
		e := newRetainEnv(t)
		img := e.stopped(t)
		created := 0
		e.lc.onCreate = func(sandbox.AgentSessionSandbox) { created++ }
		e.lc.createErr = core.NewTooManyRequestsError(sandbox.CodeSandboxCapacityLimit, "capacity", nil)
		e.send(t, "alice", "continue")
		if created != 1 {
			t.Fatalf("a capacity refusal was retried: %d creates", created)
		}
		if got := e.live(t); got.ID != img.ID || got.State != store.RetainedImageKept {
			t.Fatalf("image = %+v", got)
		}
	})
}

// An expiry that fires while a dispatch is booting from the image does not
// release it from under the dispatch: the claim, not the clock, decides, and
// the image is deleted only after its sandbox is up.
func TestAnExpiryDuringTheBootDoesNotDeleteTheImageInUse(t *testing.T) {
	e := newRetainEnv(t)
	img := e.stopped(t)
	// The message arrives a minute before the image's time runs out, so the
	// dispatch claims it.
	e.clock.Advance(retentionTTL - time.Minute)
	e.lc.onCreate = func(spec sandbox.AgentSessionSandbox) {
		e.rs.mark("boot:" + spec.Image.ID)
		// The retention time passes mid-boot, and the sweep runs.
		e.clock.Advance(2 * time.Minute)
		e.sweep()
		if got := e.rs.deletions(img.ID); got != 0 {
			t.Errorf("the image was deleted %d times while a sandbox booted from it", got)
		}
	}
	e.send(t, "alice", "continue")
	if e.lc.image.ID != img.ID {
		t.Fatalf("the sandbox booted from %+v", e.lc.image)
	}
	// Once the sandbox is up the image is spent and gone, once.
	if e.rs.deletions(img.ID) != 1 || e.handles(t) != 0 {
		t.Fatalf("deleted %d times, handles %d", e.rs.deletions(img.ID), e.handles(t))
	}
	events := e.rs.log()
	boot, del := slices.Index(events, "boot:"+img.ID), slices.Index(events, "delete:"+img.ID)
	if boot < 0 || del < boot {
		t.Fatalf("events = %v, want the boot before the delete", events)
	}
}

// A Stop and the Steer that follows it, however they interleave with the save,
// never boot from an image the registry no longer holds: every create from an
// image happens while the image has not been deleted.
func TestNoSandboxBootsFromADeletedImage(t *testing.T) {
	for i := range 5 {
		e := newRetainEnv(t)
		img := e.stopped(t)
		var deletedAtBoot atomic.Bool
		e.lc.onCreate = func(spec sandbox.AgentSessionSandbox) {
			if spec.Image.ID != "" && e.rs.deletions(spec.Image.ID) > 0 {
				deletedAtBoot.Store(true)
			}
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			e.clock.Advance(retentionTTL - time.Millisecond)
			e.sweep()
		}()
		go func() {
			defer wg.Done()
			_, _ = e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "continue"})
		}()
		wg.Wait()
		if deletedAtBoot.Load() {
			t.Fatalf("round %d: a sandbox booted from image %s after it was deleted", i, img.ID)
		}
	}
}

// Two sends in a row each continue from the disk of the turn before them (a
// turn that failed before it ran leaves the kept disk for the next:
// TestAnEarlyDispatchFailureLeavesTheDiskKept).
func TestEachTurnContinuesFromTheDiskBeforeIt(t *testing.T) {
	e := newRetainEnv(t)
	first := e.stopped(t)
	e.send(t, "alice", "second")
	if e.lc.image.ID != first.ID {
		t.Fatalf("second turn booted from %+v", e.lc.image)
	}
	e.stop(t)
	e.settle(t)
	second := e.live(t)
	e.send(t, "alice", "third")
	if e.lc.image.ID != second.ID || second.ID == first.ID {
		t.Fatalf("third turn booted from %+v, want turn 2's disk %s", e.lc.image, second.ID)
	}
	if !strings.HasPrefix(second.ID, "ari-") {
		t.Fatalf("retained image id %q", second.ID)
	}
}

// A dispatch that fails before it creates a sandbox (here its repository
// environment cannot be read) lets go of the kept disk at once, instead of
// holding it until its claim lapses: the message that follows continues from
// it, and the failed turn leaves it for the next.
type failingRepoEnvs struct{ RepoEnvironmentStore }

func (failingRepoEnvs) GetAgentRepoEnvironment(context.Context, string, string) (store.AgentRepoEnvironment, error) {
	return store.AgentRepoEnvironment{}, errors.New("the store is unreachable")
}

func TestAnEarlyDispatchFailureLeavesTheDiskKept(t *testing.T) {
	e := newRetainEnv(t)
	img := e.stopped(t)
	e.svc.RepoEnvs = failingRepoEnvs{e.pg}
	e.send(t, "alice", "continue")
	if got := e.phase(t, e.id); got != PhaseFailed {
		t.Fatalf("phase = %s, want the dispatch failed", got)
	}
	if got := e.live(t); got.ID != img.ID || got.State != store.RetainedImageKept || got.ClaimedUntil != nil {
		t.Fatalf("image after the failed dispatch = %+v, want it let go to kept", got)
	}
	if e.rs.deletions(img.ID) != 0 {
		t.Fatalf("the kept disk was deleted by a turn that never ran")
	}
	e.svc.RepoEnvs = nil
	e.send(t, "alice", "try again")
	if e.lc.image.ID != img.ID || e.phase(t, e.id) != PhaseRunning {
		t.Fatalf("the resend booted from %+v, want the kept disk %s", e.lc.image, img.ID)
	}
}

// The purge is bounded and small: it never takes more than a handful of images
// at a time and never longer than its timeout, on the Completer's one loop and
// in a Cancel or an archive alike. A registry that hangs leaves the handle to
// the sweep.
type purgeSpy struct {
	RetainedImageStore
	mu        sync.Mutex
	limits    []int
	deadlines []bool
}

func (p *purgeSpy) ClaimAgentSessionRetainedImagePurges(ctx context.Context, now time.Time, q store.RetainedImagePurge) ([]store.AgentRetainedImage, error) {
	_, bounded := ctx.Deadline()
	p.mu.Lock()
	p.limits, p.deadlines = append(p.limits, q.Limit), append(p.deadlines, bounded)
	p.mu.Unlock()
	return p.RetainedImageStore.ClaimAgentSessionRetainedImagePurges(ctx, now, q)
}

func TestThePurgeIsSmallAndBounded(t *testing.T) {
	e := newRetainEnv(t)
	spy := &purgeSpy{RetainedImageStore: e.pg}
	e.r.Store = spy
	e.stopped(t)
	e.sweep()
	if _, err := e.svc.Archive(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Cancel(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.limits) < 3 {
		t.Fatalf("purge claims = %v, want the sweep, the archive and the cancel to have purged", spy.limits)
	}
	for i, limit := range spy.limits {
		if limit != 5 || !spy.deadlines[i] {
			t.Fatalf("purge claim %d: limit %d bounded %v, want 5 and a deadline", i, limit, spy.deadlines[i])
		}
	}
}

func TestAHungRegistryDoesNotBlockACancelOrTheSweep(t *testing.T) {
	prev := retentionPurgeTimeout
	retentionPurgeTimeout = 100 * time.Millisecond
	t.Cleanup(func() { retentionPurgeTimeout = prev })
	e := newRetainEnv(t)
	img := e.stopped(t)
	e.rs.setBlockDelete(true)

	start := time.Now()
	if _, err := e.svc.Cancel(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("the Cancel waited %v for a hung registry", took)
	}
	if e.handles(t) != 1 {
		t.Fatalf("the handle went while its image could not be deleted")
	}
	e.clock.Advance(2 * time.Minute)
	start = time.Now()
	e.sweep()
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("the sweep waited %v for a hung registry", took)
	}
	// The Delete verb deletes the image itself, so it is bounded too: it fails,
	// retryably, with the session and its handle in place.
	start = time.Now()
	if err := e.svc.Delete(caller("alice"), e.id); err == nil {
		t.Fatalf("the session was deleted while its image could not be")
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("the Delete waited %v for a hung registry", took)
	}
	if _, err := e.pg.GetAgentSession(context.Background(), e.id); err != nil || e.handles(t) != 1 {
		t.Fatalf("the session or its handle went with a failed delete: %v", err)
	}
	// The registry comes back: the sweep finishes the job.
	e.rs.setBlockDelete(false)
	e.clock.Advance(time.Hour)
	e.sweep()
	if e.rs.deletions(img.ID) != 1 || e.handles(t) != 0 {
		t.Fatalf("deleted %d times, handles %d", e.rs.deletions(img.ID), e.handles(t))
	}
}

// Stop does not end a live edit: with an editor SSH session open, the stopped
// turn settles and keeps nothing, and the sandbox stays up under the SSH pin as
// it always did; once the editor leaves, the ordinary teardown ends it.
type sshOpenStore struct{ Store }

func (sshOpenStore) AgentSessionSSHActivity(context.Context, string, time.Time) (bool, *time.Time, error) {
	return true, nil, nil
}

func TestAStopWithAnOpenEditorKeepsTheSandboxAndRetainsNothing(t *testing.T) {
	e := newRetainEnv(t)
	e.c.Store = sshOpenStore{e.pg}
	e.stop(t)
	before := e.session(t)
	e.settle(t)
	row := e.session(t)
	if row.Phase != PhaseCompleted || !e.turn(t, 1).Interrupted {
		t.Fatalf("the stopped turn settled as %+v", row)
	}
	if len(e.rs.retained) != 0 || e.handles(t) != 0 {
		t.Fatalf("a session with an open editor was saved: %+v, handles %d", e.rs.retained, e.handles(t))
	}
	// The turn says why nothing was kept.
	if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionLost || turn.RetentionReason != store.RetainedNotSavedEditorOpen {
		t.Fatalf("turn = %+v, want the open editor recorded as why nothing was kept", turn)
	}
	e.c.reapIdleSandboxes(context.Background())
	if e.lc.canceled != 0 || e.session(t).SandboxID != before.SandboxID {
		t.Fatalf("canceled=%d sandbox=%q: the open editor's sandbox was ended", e.lc.canceled, e.session(t).SandboxID)
	}
	// The editor leaves: the sandbox is reaped as before, and the next message
	// starts fresh.
	e.c.Store = e.pg
	e.c.reapIdleSandboxes(context.Background())
	if e.lc.canceled != 1 || e.session(t).SandboxID != "" {
		t.Fatalf("canceled=%d: the sandbox was not reaped once the editor left", e.lc.canceled)
	}
	e.send(t, "alice", "carry on")
	if e.lc.image != (sandbox.AgentImage{}) {
		t.Fatalf("the next turn booted from %+v with nothing kept", e.lc.image)
	}
}

// The records a save writes when it ends have a time of their own, started
// then: a save that takes longer than that bound still records its image.
func TestASlowSaveStillRecordsItsImage(t *testing.T) {
	prev := retentionSettleTimeout
	retentionSettleTimeout = 150 * time.Millisecond
	t.Cleanup(func() { retentionSettleTimeout = prev })
	e := newRetainEnv(t)
	e.rs.onRetain = func(retainCall) { time.Sleep(400 * time.Millisecond) }
	img := e.stopped(t)
	if img.State != store.RetainedImageKept || img.ImageDigest != testImageDigest {
		t.Fatalf("a save that outlasted the settle bound left %+v, want it kept", img)
	}
	if row := e.session(t); row.SandboxID != "" {
		t.Fatalf("the ended sandbox's id was not cleared: %q", row.SandboxID)
	}
}
