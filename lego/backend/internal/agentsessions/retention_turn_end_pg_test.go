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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR047 D12 widened (w10/m491) against the real store: a turn that ends with
// undelivered work keeps its disk however it ended (it completed and was
// followed by a message, it completed and idled out, it failed), and the next
// message boots from it. A checkout with nothing undelivered keeps nothing.

// undeliveredGit is a checkout with one unpushed commit and uncommitted
// changes, as the driver reports it.
var undeliveredGit = statusGit{Dirty: true, UnpushedCommits: 1, Undelivered: []string{"the working tree has uncommitted changes"}}

func failedStatus(git *statusGit) string {
	raw := map[string]any{"state": driverStateFailed, "error": "ACP turn made no progress"}
	if git != nil {
		raw["git"] = git
	}
	out, _ := json.Marshal(raw)
	return string(out)
}

// finished settles the running turn with the driver status given.
func (e *retainEnv) finished(t *testing.T, status string) {
	t.Helper()
	e.lc.status = status
	e.settle(t)
}

// idle passes the idle grace and runs the reaper.
func (e *retainEnv) idleOut(t *testing.T) {
	t.Helper()
	e.clock.Advance(e.c.IdleTTL + time.Minute)
	e.c.reapIdleSandboxes(context.Background())
}

func (e *retainEnv) assertContinuedFrom(t *testing.T, img store.AgentRetainedImage, cause string) {
	t.Helper()
	if e.lc.image.ID != img.ID || e.lc.image.Digest != testImageDigest {
		t.Fatalf("the next turn booted %+v, want the kept disk %s", e.lc.image, img.ID)
	}
	turn := e.turn(t, 1)
	if turn.Retention != store.TurnRetentionContinued || turn.RetentionCause != cause {
		t.Fatalf("turn 1 = retention %q cause %q, want continued after %s", turn.Retention, turn.RetentionCause, cause)
	}
	if c, ok := turn.Checkout(); !ok || !c.Dirty || c.UnpushedCommits != 1 {
		t.Fatalf("turn 1's stake = %+v (%t), want the unpushed commit and the dirty tree", c, ok)
	}
	if env := e.lc.driverEnv[envCheckoutNote]; !strings.Contains(env, "turn 1 left") || !strings.Contains(env, "1 unpushed commit and the uncommitted changes") {
		t.Fatalf("the agent was told %q, want the restored checkout", env)
	}
}

// DoD 1: a completed turn with an unpushed commit and an uncommitted file,
// followed by a message within the idle grace, runs that message in a sandbox
// booted from its disk. Before w10/m491 the steer tore the sandbox down and
// booted a fresh clone.
func TestAMessageAfterACompletedTurnContinuesFromItsUndeliveredWork(t *testing.T) {
	e := newRetainEnv(t)
	e.c.IdleTTL = time.Hour
	before := e.session(t)
	e.finished(t, gitStatus(undeliveredGit))
	if len(e.rs.retained) != 0 || e.lc.canceled != 0 {
		t.Fatalf("saves %+v canceled %d: the idle grace should still hold the sandbox", e.rs.retained, e.lc.canceled)
	}
	e.send(t, "alice", "carry on")
	if len(e.rs.retained) != 1 || e.rs.retained[0].sandbox != before.SandboxID {
		t.Fatalf("saves = %+v, want the replaced sandbox %s saved once", e.rs.retained, before.SandboxID)
	}
	if e.rs.checkoutReads != 0 {
		t.Fatalf("the checkout was read %d times: the turn's own facts decide", e.rs.checkoutReads)
	}
	// The continued image is spent and purged; the boot names it.
	e.assertContinuedFrom(t, store.AgentRetainedImage{ID: e.rs.retained[0].image}, store.TurnRetentionCauseReplaced)
	if e.handles(t) != 0 || e.rs.deletions(e.rs.retained[0].image) != 1 {
		t.Fatalf("handles %d: the spent image was not deleted", e.handles(t))
	}
	if e.phase(t, e.id) != PhaseRunning {
		t.Fatalf("phase = %s", e.phase(t, e.id))
	}
}

// A checkout with nothing undelivered keeps nothing: the message starts
// fresh as before, without a pause.
func TestAMessageAfterACleanTurnStartsFresh(t *testing.T) {
	e := newRetainEnv(t)
	e.c.IdleTTL = time.Hour
	e.finished(t, gitStatus(statusGit{RemoteHead: "0123456789abcdef0123456789abcdef01234567"}))
	e.send(t, "alice", "carry on")
	if len(e.rs.retained) != 0 || e.handles(t) != 0 || e.rs.checkoutReads != 0 {
		t.Fatalf("saves %+v handles %d reads %d, want a fresh start", e.rs.retained, e.handles(t), e.rs.checkoutReads)
	}
	if e.lc.image != (sandbox.AgentImage{}) || e.lc.driverEnv[envCheckoutNote] != "" {
		t.Fatalf("image %+v note %q, want a fresh checkout and nothing to say", e.lc.image, e.lc.driverEnv[envCheckoutNote])
	}
	if turn := e.turn(t, 1); turn.Retention != "" || turn.RetentionCause != "" {
		t.Fatalf("turn 1 = %+v, want nothing decided", turn)
	}
}

// Facts the turn did not report (a session with no repository, an older
// driver) are read in the sandbox before it is replaced.
func TestAMessageReadsAnUnreportedCheckoutInTheSandbox(t *testing.T) {
	for name, tc := range map[string]struct {
		checkout store.TurnCheckout
		err      error
		saved    bool
	}{
		"files in it":    {checkout: store.TurnCheckout{Dirty: true}, saved: true},
		"an empty one":   {},
		"a failed read":  {err: errors.New("exec failed"), saved: false},
		"a gone sandbox": {err: fmt.Errorf("%w: gone", core.ErrNotFound), saved: false},
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetainEnv(t)
			e.c.IdleTTL = time.Hour
			e.rs.checkout, e.rs.checkoutErr = tc.checkout, tc.err
			e.finished(t, `{"state":"succeeded"}`)
			e.send(t, "alice", "carry on")
			if e.rs.checkoutReads != 1 {
				t.Fatalf("checkout reads = %d, want one", e.rs.checkoutReads)
			}
			if saved := len(e.rs.retained) == 1; saved != tc.saved {
				t.Fatalf("saved = %t, want %t", saved, tc.saved)
			}
			if tc.saved {
				turn := e.turn(t, 1)
				if c, ok := turn.Checkout(); !ok || !c.Dirty || turn.Retention != store.TurnRetentionContinued {
					t.Fatalf("turn 1 = %+v, want the read recorded and continued", turn)
				}
			}
		})
	}
}

// An editor connected since the turn ended may have changed the checkout, so
// it is read again; one open now keeps nothing, as for a Stop, and says so.
func TestAnEditorDecidesTheReplacedTurnsCheckout(t *testing.T) {
	t.Run("connected since: read again", func(t *testing.T) {
		e := newRetainEnv(t)
		e.c.IdleTTL = time.Hour
		e.finished(t, gitStatus(statusGit{}))
		ended := e.clock.Now().Add(time.Minute)
		e.c.Store = sshEndedStore{Store: e.pg, ended: ended}
		e.rs.setCheckout(store.TurnCheckout{UnpushedCommits: 2})
		e.send(t, "alice", "carry on")
		if e.rs.checkoutReads != 1 || len(e.rs.retained) != 1 {
			t.Fatalf("reads %d saves %d, want the edited checkout read and kept", e.rs.checkoutReads, len(e.rs.retained))
		}
		if c, _ := e.turn(t, 1).Checkout(); c.UnpushedCommits != 2 {
			t.Fatalf("stake = %+v, want what the sandbox holds now", c)
		}
	})
	t.Run("open now: lost", func(t *testing.T) {
		e := newRetainEnv(t)
		e.c.IdleTTL = time.Hour
		e.finished(t, gitStatus(undeliveredGit))
		e.c.Store = sshOpenStore{e.pg}
		e.rs.setCheckout(store.TurnCheckout{Dirty: true, UnpushedCommits: 1})
		e.send(t, "alice", "carry on")
		if len(e.rs.retained) != 0 || e.lc.image != (sandbox.AgentImage{}) {
			t.Fatalf("saves %+v image %+v, want a fresh start", e.rs.retained, e.lc.image)
		}
		turn := e.turn(t, 1)
		if turn.Retention != store.TurnRetentionLost || turn.RetentionReason != store.RetainedNotSavedEditorOpen || turn.RetentionCause != store.TurnRetentionCauseReplaced {
			t.Fatalf("turn 1 = %+v, want lost to the open editor", turn)
		}
		want := "The 1 unpushed commit and the uncommitted changes from turn 1 were lost (an editor was open on the sandbox), so this turn started from a fresh checkout of the branch."
		if note := e.turn(t, 2).StartNote; note != want {
			t.Fatalf("start note = %q\nwant %q", note, want)
		}
		if e.lc.driverEnv[envCheckoutNote] != want {
			t.Fatalf("the agent was told %q", e.lc.driverEnv[envCheckoutNote])
		}
	})
}

type sshEndedStore struct {
	Store
	ended time.Time
}

func (s sshEndedStore) AgentSessionSSHActivity(context.Context, string, time.Time) (bool, *time.Time, error) {
	ended := s.ended
	return false, &ended, nil
}

// DoD 2: past the idle grace a completed turn's undelivered work is saved
// instead of discarded, and a later message continues from it; a clean
// checkout is closed as before, and an archived session's at once.
func TestTheIdleCloseKeepsACompletedTurnsUndeliveredWork(t *testing.T) {
	t.Run("undelivered: kept", func(t *testing.T) {
		e := newRetainEnv(t)
		e.c.IdleTTL = time.Hour
		e.finished(t, gitStatus(undeliveredGit))
		e.idleOut(t)
		img := e.live(t)
		turn := e.turn(t, 1)
		if img.State != store.RetainedImageKept || turn.Retention != store.TurnRetentionKept || turn.RetentionCause != store.TurnRetentionCauseIdleClosed {
			t.Fatalf("image %+v turn %+v, want kept by the idle close", img, turn)
		}
		if e.lc.canceled != 0 || e.session(t).SandboxID != "" {
			t.Fatalf("canceled %d sandbox %q, want the save to have ended it", e.lc.canceled, e.session(t).SandboxID)
		}
		e.send(t, "alice", "carry on")
		e.assertContinuedFrom(t, img, store.TurnRetentionCauseIdleClosed)
	})
	t.Run("clean: closed", func(t *testing.T) {
		e := newRetainEnv(t)
		e.c.IdleTTL = time.Hour
		e.finished(t, gitStatus(statusGit{}))
		e.idleOut(t)
		if len(e.rs.retained) != 0 || e.handles(t) != 0 || e.lc.canceled != 1 {
			t.Fatalf("saves %+v handles %d canceled %d, want today's close", e.rs.retained, e.handles(t), e.lc.canceled)
		}
	})
	t.Run("archived: reclaimed at once", func(t *testing.T) {
		e := newRetainEnv(t)
		e.c.IdleTTL = time.Hour
		e.finished(t, gitStatus(undeliveredGit))
		if _, err := e.svc.Archive(caller("alice"), e.id); err != nil {
			t.Fatal(err)
		}
		e.c.reapIdleSandboxes(context.Background())
		if len(e.rs.retained) != 0 || e.handles(t) != 0 || e.lc.canceled != 1 {
			t.Fatalf("saves %+v handles %d canceled %d, want the archive's reclaim", e.rs.retained, e.handles(t), e.lc.canceled)
		}
	})
}

// DoD 3: a failed turn with undelivered work keeps its disk instead of being
// torn down at once, and reads failed with its reason; a clean failed turn is
// torn down at once; a Cancel releases the image.
func TestAFailedTurnKeepsItsUndeliveredWork(t *testing.T) {
	t.Run("undelivered: kept, still failed", func(t *testing.T) {
		e := newRetainEnv(t)
		e.c.IdleTTL = time.Hour
		e.finished(t, failedStatus(&undeliveredGit))
		row := e.session(t)
		if row.Phase != PhaseFailed || row.FailureReason != "ACP turn made no progress" {
			t.Fatalf("session = %s %q, want failed with its reason", row.Phase, row.FailureReason)
		}
		img := e.live(t)
		turn := e.turn(t, 1)
		if img.State != store.RetainedImageKept || turn.RetentionCause != store.TurnRetentionCauseFailed {
			t.Fatalf("image %+v turn %+v, want kept for the failed turn", img, turn)
		}
		e.send(t, "alice", "try again")
		e.assertContinuedFrom(t, img, store.TurnRetentionCauseFailed)
	})
	t.Run("clean: torn down at once", func(t *testing.T) {
		e := newRetainEnv(t)
		e.c.IdleTTL = time.Hour
		e.finished(t, failedStatus(&statusGit{}))
		if len(e.rs.retained) != 0 || e.handles(t) != 0 || e.lc.canceled != 1 {
			t.Fatalf("saves %+v handles %d canceled %d, want today's teardown", e.rs.retained, e.handles(t), e.lc.canceled)
		}
	})
	t.Run("cancel releases it", func(t *testing.T) {
		e := newRetainEnv(t)
		e.finished(t, failedStatus(&undeliveredGit))
		img := e.live(t)
		if _, err := e.svc.Cancel(caller("alice"), e.id); err != nil {
			t.Fatal(err)
		}
		e.sweep()
		if e.handles(t) != 0 || e.rs.deletions(img.ID) != 1 {
			t.Fatalf("handles %d deletions %d, want the image gone", e.handles(t), e.rs.deletions(img.ID))
		}
		if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionLost || turn.RetentionReason != store.RetainedReleaseCanceled {
			t.Fatalf("turn = %+v", turn)
		}
	})
}

// A save on a close that fails keeps nothing, records the loss, and ends the
// sandbox as the close would have: the reaper does not try again every tick.
func TestAFailedSaveOnACloseEndsTheSandbox(t *testing.T) {
	for name, err := range map[string]error{
		"storage full": fmt.Errorf("%w: full", sandbox.ErrAgentImageStorageLimit),
		"save failed":  errors.New("pause failed"),
	} {
		t.Run(name, func(t *testing.T) {
			e := newRetainEnv(t)
			e.rs.retainErr = err
			e.finished(t, failedStatus(&undeliveredGit))
			if e.lc.canceled != 1 || e.session(t).SandboxID != "" {
				t.Fatalf("canceled %d sandbox %q, want the sandbox ended", e.lc.canceled, e.session(t).SandboxID)
			}
			if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionLost || turn.RetentionCause != store.TurnRetentionCauseFailed {
				t.Fatalf("turn = %+v, want the loss recorded", turn)
			}
			e.c.Reconcile(context.Background())
			if len(e.rs.retained) != 1 {
				t.Fatalf("saves = %d, want no retry", len(e.rs.retained))
			}
			e.sweep()
			e.send(t, "alice", "try again")
			if note := e.turn(t, 2).StartNote; !strings.HasPrefix(note, "The 1 unpushed commit and the uncommitted changes from turn 1 were lost (") {
				t.Fatalf("start note = %q", note)
			}
		})
	}
}

// A sandbox gone before its turn settled has nothing left to save: the turn
// records that whatever it held was lost with it.
func TestALostSandboxRecordsTheLoss(t *testing.T) {
	e := newRetainEnv(t)
	e.lc.statusErr = fmt.Errorf("%w: gone", core.ErrNotFound)
	e.settle(t)
	turn := e.turn(t, 1)
	if e.session(t).Phase != PhaseFailed || turn.Retention != store.TurnRetentionLost ||
		turn.RetentionReason != store.RetainedNotSavedSandboxLost || turn.RetentionCause != store.TurnRetentionCauseFailed {
		t.Fatalf("turn = %+v, want lost with the sandbox", turn)
	}
	if _, known := turn.Checkout(); known {
		t.Fatalf("a lost sandbox's stake is unknown, got %+v", turn)
	}
}

// A message that arrives while a failed turn's save runs waits for it and
// boots from the image, and the sandbox is saved once.
func TestAMessageWaitsForAFailedTurnsSave(t *testing.T) {
	e := newRetainEnv(t)
	entered, finish := e.holdSave()
	e.finished(t, failedStatus(&undeliveredGit))
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the failed turn's disk was not saved")
	}
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		e.send(t, "alice", "try again")
	}()
	select {
	case <-sent:
		t.Fatal("the message did not wait for the save")
	case <-time.After(50 * time.Millisecond):
	}
	finish()
	<-sent
	if len(e.rs.retained) != 1 {
		t.Fatalf("saves = %d, want one", len(e.rs.retained))
	}
	if e.lc.image.Digest != testImageDigest {
		t.Fatalf("the message booted %+v, want the saved disk", e.lc.image)
	}
}

// With retention off nothing is decided, read or kept on any of the three
// ends.
func TestNoTurnEndKeepsAnythingWithRetentionOff(t *testing.T) {
	e := newRetainEnv(t)
	e.r.TTL = 0
	e.c.IdleTTL = time.Hour
	e.finished(t, gitStatus(undeliveredGit))
	e.send(t, "alice", "carry on")
	e.finished(t, failedStatus(&undeliveredGit))
	if len(e.rs.retained) != 0 || e.handles(t) != 0 || e.rs.checkoutReads != 0 {
		t.Fatalf("saves %+v handles %d reads %d, want nothing", e.rs.retained, e.handles(t), e.rs.checkoutReads)
	}
}
