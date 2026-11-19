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
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// ADR047 D12, w22/m413: the retained-image handle of a stopped turn, against
// the real control-plane Postgres.

type retainedFixture struct {
	t    *testing.T
	s    *PGStore
	pool *pgxpool.Pool
	ws   string
	now  time.Time
}

func newRetainedFixture(t *testing.T, name string) *retainedFixture {
	t.Helper()
	s, pool, tenant := agentSessionPGStore(t, name)
	return &retainedFixture{t: t, s: s, pool: pool, ws: tenant.ID, now: time.Now().UTC().Truncate(time.Millisecond)}
}

// running is a session whose first turn is running.
func (f *retainedFixture) running() AgentSession {
	f.t.Helper()
	row, err := f.s.CreateAgentSession(context.Background(), AgentSession{WorkspaceID: f.ws, InitialPrompt: "do the task"})
	if err != nil {
		f.t.Fatal(err)
	}
	return row
}

func (f *retainedFixture) stop(session AgentSession) {
	f.t.Helper()
	if err := f.s.MarkAgentSessionTurnInterrupted(context.Background(), session.ID, 1); err != nil {
		f.t.Fatal(err)
	}
}

// settle is the Completer's write for a settled turn: the session completes
// and, for a stopped turn, its retention begins in the same transaction.
func (f *retainedFixture) settle(session AgentSession) (AgentSession, *AgentRetainedImage, error) {
	f.t.Helper()
	out, _, img, err := f.s.FinalizeAgentSessionRetaining(context.Background(), session.ID, "", "", NewAgentRetainedImage{
		ID: ids.New(ids.AgentRetainedImage), BaseImage: "agent:1", SaveDeadline: f.now.Add(5 * time.Minute)})
	return out, img, err
}

// saving is a completed session whose only turn a person stopped, with its
// retained image saving.
func (f *retainedFixture) saving() (AgentSession, AgentRetainedImage) {
	f.t.Helper()
	session := f.running()
	f.stop(session)
	out, img, err := f.settle(session)
	if err != nil || img == nil {
		f.t.Fatalf("settle = %+v / %v", img, err)
	}
	return out, *img
}

// kept takes a stopped session's retention through reserve and keep.
func (f *retainedFixture) kept(size int64, ttl time.Duration) (AgentSession, AgentRetainedImage) {
	f.t.Helper()
	ctx := context.Background()
	session, img := f.saving()
	if err := f.s.ReserveAgentSessionRetainedImage(ctx, f.ws, img.ID, size, 1<<40); err != nil {
		f.t.Fatal(err)
	}
	if ok, err := f.s.KeepAgentSessionRetainedImage(ctx, img.ID, testImageDigest, f.now.Add(ttl)); err != nil || !ok {
		f.t.Fatalf("keep = %v / %v", ok, err)
	}
	return session, img
}

func (f *retainedFixture) live(session AgentSession) AgentRetainedImage {
	f.t.Helper()
	img, err := f.s.AgentSessionRetainedImage(context.Background(), session.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return img
}

func (f *retainedFixture) none(session AgentSession) {
	f.t.Helper()
	if img, err := f.s.AgentSessionRetainedImage(context.Background(), session.ID); !errors.Is(err, ErrNotFound) {
		f.t.Fatalf("live retained image = %+v / %v, want none", img, err)
	}
}

func (f *retainedFixture) turn(session AgentSession) AgentSessionTurn {
	f.t.Helper()
	turns, err := f.s.AgentSessionTurns(context.Background(), session.ID)
	if err != nil || len(turns) == 0 {
		f.t.Fatalf("turns = %v / %v", turns, err)
	}
	return turns[len(turns)-1]
}

// apiTurn is the session's last turn as the API reads it: a save in progress
// reads "saving".
func (f *retainedFixture) apiTurn(session AgentSession) AgentSessionTurn {
	f.t.Helper()
	turns, err := f.s.AgentSessionTurnsWithRetention(context.Background(), session.ID)
	if err != nil || len(turns) == 0 {
		f.t.Fatalf("turns = %v / %v", turns, err)
	}
	return turns[len(turns)-1]
}

func (f *retainedFixture) rows(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// The save is begun by the write that settles the turn: only a stopped,
// unarchived turn gets one, once, so two replicas never pause one sandbox, and
// the session is steerable at the very instant its retained image exists.
func TestPGRetainedImageBeginsWithTheSettleOfAStoppedTurn(t *testing.T) {
	f := newRetainedFixture(t, "retain-begin")
	ctx := context.Background()

	// A turn that ended on its own was not stopped: settled, nothing to save.
	finished := f.running()
	out, img, err := f.settle(finished)
	if err != nil || img != nil || out.Phase != "completed" {
		t.Fatalf("settle of an unstopped turn = %+v / %+v / %v", out, img, err)
	}
	f.none(finished)
	// An archived session takes no more messages, so it is settled and kept
	// nothing.
	archived := f.running()
	f.stop(archived)
	if _, err := f.s.SetAgentSessionArchived(ctx, archived.ID, true); err != nil {
		t.Fatal(err)
	}
	out, img, err = f.settle(archived)
	if err != nil || img != nil || out.Phase != "completed" {
		t.Fatalf("settle of an archived session = %+v / %+v / %v", out, img, err)
	}
	f.none(archived)

	stopped := f.running()
	f.stop(stopped)
	out, img, err = f.settle(stopped)
	if err != nil || img == nil {
		t.Fatalf("settle of a stopped turn = %+v / %v", img, err)
	}
	if out.Phase != "completed" || img.State != RetainedImageSaving || img.WorkspaceID != f.ws || img.SessionID != stopped.ID ||
		img.Turn != 1 || img.BaseImage != "agent:1" || img.ImageDigest != "" {
		t.Fatalf("settled = %+v, saving = %+v", out, img)
	}
	// A second settle finds the session no longer live: it starts no second save.
	if _, img2, err := f.settle(stopped); !errors.Is(err, ErrNotFound) || img2 != nil {
		t.Fatalf("a second settle = %+v / %v, want ErrNotFound", img2, err)
	}
	// One live retention per session, even for a write that bypasses the check.
	_, err = f.pool.Exec(ctx, `INSERT INTO agent_session_retained_images (id, session_id, workspace_id, turn, save_deadline)
		VALUES ($1, $2, $3, 1, now())`, ids.New(ids.AgentRetainedImage), stopped.ID, f.ws)
	if err == nil {
		t.Fatalf("a second live retention was inserted for one session")
	}
}

// The retained bytes are part of what the workspace's cap reads, from the
// moment they are reserved until the handle is gone; two copies that would
// together pass the cap cannot both be reserved.
func TestPGRetainedImageCountsAgainstTheStorageCap(t *testing.T) {
	f := newRetainedFixture(t, "retain-cap")
	ctx := context.Background()
	_, a := f.saving()
	_, b := f.saving()
	if used, err := f.s.SandboxStorageBytes(ctx, f.ws); err != nil || used != 0 {
		t.Fatalf("storage before any reservation = %d / %v", used, err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	refused := 0
	for _, id := range []string{a.ID, b.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := f.s.ReserveAgentSessionRetainedImage(ctx, f.ws, id, 60, 100)
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
	// A released image still counts until the registry has dropped it and the
	// handle is removed.
	for _, id := range []string{a.ID, b.ID} {
		if _, err := f.s.ReleaseAgentSessionRetainedImage(ctx, id, RetainedReleaseSaveFailed, f.now); err != nil {
			t.Fatal(err)
		}
	}
	if used, _ := f.s.SandboxStorageBytes(ctx, f.ws); used != 60 {
		t.Fatalf("storage after release = %d, want the 60 bytes the registry still holds", used)
	}
	for _, id := range []string{a.ID, b.ID} {
		if err := f.s.DeleteAgentSessionRetainedImage(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if used, _ := f.s.SandboxStorageBytes(ctx, f.ws); used != 0 {
		t.Fatalf("storage after the handles went = %d, want 0", used)
	}
	// A reservation for a retention released meanwhile is refused.
	if err := f.s.ReserveAgentSessionRetainedImage(ctx, f.ws, a.ID, 1, 100); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reserve on a gone retention = %v, want ErrNotFound", err)
	}
}

// saving → kept → in_use → released(continued), each a compare-and-set, and
// the turn records what became of its disk.
func TestPGRetainedImageStateMachine(t *testing.T) {
	f := newRetainedFixture(t, "retain-states")
	ctx := context.Background()
	session, img := f.saving()
	// Nothing is kept, so nothing can be used, while it is still saving.
	if _, err := f.s.ClaimAgentSessionRetainedImage(ctx, session.ID, f.now, time.Minute); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim while saving = %v, want ErrNotFound", err)
	}
	if err := f.s.ReserveAgentSessionRetainedImage(ctx, f.ws, img.ID, 4096, 1<<40); err != nil {
		t.Fatal(err)
	}
	until := f.now.Add(24 * time.Hour)
	if ok, err := f.s.KeepAgentSessionRetainedImage(ctx, img.ID, testImageDigest, until); err != nil || !ok {
		t.Fatalf("keep = %v / %v", ok, err)
	}
	if ok, _ := f.s.KeepAgentSessionRetainedImage(ctx, img.ID, testImageDigest, until); ok {
		t.Fatalf("a retention was kept twice")
	}
	if turn := f.turn(session); turn.Retention != TurnRetentionKept || turn.RetainedUntil == nil || !turn.RetainedUntil.Equal(until) {
		t.Fatalf("turn after keep = %+v", turn)
	}
	got := f.live(session)
	if got.State != RetainedImageKept || got.ImageDigest != testImageDigest || got.SizeBytes != 4096 || got.RetainedUntil == nil {
		t.Fatalf("kept = %+v", got)
	}

	claimed, err := f.s.ClaimAgentSessionRetainedImage(ctx, session.ID, f.now, 10*time.Minute)
	if err != nil || claimed.ID != img.ID || claimed.State != RetainedImageInUse {
		t.Fatalf("claim = %+v / %v", claimed, err)
	}
	// One dispatch uses it: a second claim finds nothing.
	if _, err := f.s.ClaimAgentSessionRetainedImage(ctx, session.ID, f.now, 10*time.Minute); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a second claim = %v, want ErrNotFound", err)
	}
	// A dispatch that did not use it lets it go, and a later message can.
	if err := f.s.UnclaimAgentSessionRetainedImage(ctx, img.ID); err != nil {
		t.Fatal(err)
	}
	if again, err := f.s.ClaimAgentSessionRetainedImage(ctx, session.ID, f.now, 10*time.Minute); err != nil || again.ID != img.ID {
		t.Fatalf("claim after unclaim = %+v / %v", again, err)
	}
	if err := f.s.FinishAgentSessionRetainedImages(ctx, session.ID, img.ID, f.now); err != nil {
		t.Fatal(err)
	}
	f.none(session)
	if turn := f.turn(session); turn.Retention != TurnRetentionContinued {
		t.Fatalf("turn after the continuation = %+v", turn)
	}
	var state, reason string
	if err := f.pool.QueryRow(ctx, `SELECT state, release_reason FROM agent_session_retained_images WHERE id = $1`, img.ID).Scan(&state, &reason); err != nil ||
		state != RetainedImageReleased || reason != RetainedReleaseContinued {
		t.Fatalf("row after the continuation = %s / %s / %v", state, reason, err)
	}
}

// A claim is decided against the clock in the statement that makes it, and an
// expiry in the one that releases: a kept image past its time cannot be
// claimed, and a claimed one cannot be expired out from under its dispatch.
func TestPGRetainedImageExpiryAndClaimExcludeEachOther(t *testing.T) {
	f := newRetainedFixture(t, "retain-race")
	ctx := context.Background()

	past, _ := f.kept(10, time.Hour)
	// The claim and the sweep read the same later instant.
	later := f.now.Add(2 * time.Hour)
	if _, err := f.s.ClaimAgentSessionRetainedImage(ctx, past.ID, later, time.Minute); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim of an image past its time = %v, want ErrNotFound", err)
	}
	if n, err := f.s.ExpireAgentSessionRetainedImages(ctx, later); err != nil || n < 1 {
		t.Fatalf("expire = %d / %v", n, err)
	}
	f.none(past)
	if turn := f.turn(past); turn.Retention != TurnRetentionLost {
		t.Fatalf("turn after the expiry = %+v, want lost", turn)
	}

	// A claim whose dispatch died lapses: the sweep lets the image go while
	// it is still within its time, and expires it only once that has passed.
	lapsed, lapsedImg := f.kept(10, time.Hour)
	if _, err := f.s.ClaimAgentSessionRetainedImage(ctx, lapsed.ID, f.now, 30*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.ExpireAgentSessionRetainedImages(ctx, f.now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := f.live(lapsed); got.ID != lapsedImg.ID || got.State != RetainedImageInUse {
		t.Fatalf("a claim still within its lease was let go: %+v", got)
	}
	if _, err := f.s.ExpireAgentSessionRetainedImages(ctx, f.now.Add(45*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := f.live(lapsed); got.State != RetainedImageKept || got.ClaimedUntil != nil {
		t.Fatalf("a lapsed claim = %+v, want let go to kept", got)
	}
	if _, err := f.s.ExpireAgentSessionRetainedImages(ctx, f.now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	f.none(lapsed)

	// A claim that is still held is not expired, even past the retention time:
	// the image is not released from under the dispatch booting from it.
	held, heldImg := f.kept(10, time.Hour)
	if _, err := f.s.ClaimAgentSessionRetainedImage(ctx, held.ID, f.now, 4*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.ExpireAgentSessionRetainedImages(ctx, f.now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := f.live(held); got.ID != heldImg.ID || got.State != RetainedImageInUse {
		t.Fatalf("an image in use was expired: %+v", got)
	}

	// Racing on the retention boundary, exactly one of the claim (read just
	// before it) and the expiry (read just after it) wins, and the loser finds
	// nothing to take.
	for i := range 25 {
		session, _ := f.kept(1, time.Hour)
		boundary := f.now.Add(time.Hour)
		var claimed bool
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := f.s.ClaimAgentSessionRetainedImage(ctx, session.ID, boundary.Add(-time.Second), time.Minute)
			claimed = err == nil
		}()
		go func() {
			defer wg.Done()
			if _, err := f.s.ExpireAgentSessionRetainedImages(ctx, boundary.Add(time.Nanosecond)); err != nil {
				t.Error(err)
			}
		}()
		wg.Wait()
		var state string
		if err := f.pool.QueryRow(ctx, `SELECT state FROM agent_session_retained_images WHERE session_id = $1`, session.ID).Scan(&state); err != nil {
			t.Fatal(err)
		}
		switch {
		case claimed && state != RetainedImageInUse:
			t.Fatalf("round %d: the claim won but the row is %s", i, state)
		case !claimed && state != RetainedImageReleased:
			t.Fatalf("round %d: the claim lost but the row is %s, not released by the expiry", i, state)
		}
	}
}

// Cancel and archive release the retained image in the transaction that
// changes the session, so the handle is never left naming an image nothing
// can use; a lapsed save is abandoned; and the purge hands each released
// image to one claimant, backing off after a failure.
func TestPGRetainedImageIsReleasedWithItsSession(t *testing.T) {
	f := newRetainedFixture(t, "retain-release")
	ctx := context.Background()

	canceled, _ := f.kept(5, time.Hour)
	if _, _, err := f.s.SetAgentSessionCanceled(ctx, canceled.ID); err != nil {
		t.Fatal(err)
	}
	f.none(canceled)
	if turn := f.turn(canceled); turn.Retention != TurnRetentionLost {
		t.Fatalf("a canceled session's stopped turn = %+v, want lost", turn)
	}

	archived, _ := f.kept(5, time.Hour)
	if _, err := f.s.SetAgentSessionArchived(ctx, archived.ID, true); err != nil {
		t.Fatal(err)
	}
	f.none(archived)
	// Unarchiving does not bring it back.
	if _, err := f.s.SetAgentSessionArchived(ctx, archived.ID, false); err != nil {
		t.Fatal(err)
	}
	f.none(archived)

	// A save whose Completer is gone is abandoned once its deadline passes.
	lapsed, _ := f.saving()
	if _, err := f.s.ExpireAgentSessionRetainedImages(ctx, f.now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	f.live(lapsed) // still saving: the deadline is five minutes out
	if _, err := f.s.ExpireAgentSessionRetainedImages(ctx, f.now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	f.none(lapsed)
	var reason string
	if err := f.pool.QueryRow(ctx, `SELECT release_reason FROM agent_session_retained_images WHERE session_id = $1`, lapsed.ID).Scan(&reason); err != nil || reason != RetainedReleaseAbandoned {
		t.Fatalf("a lapsed save was released as %q / %v", reason, err)
	}

	// The three released images are claimed once each; a failed delete backs
	// off, doubling per attempt.
	p := RetainedImagePurge{Backoff: time.Minute, MaxBackoff: time.Hour, Limit: 10}
	claimedIDs := func(at time.Time, session string) map[string]bool {
		q := p
		q.SessionID = session
		rows, err := f.s.ClaimAgentSessionRetainedImagePurges(ctx, at, q)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, r := range rows {
			if r.WorkspaceID == f.ws {
				out[r.SessionID] = true
			}
		}
		return out
	}
	got := claimedIDs(f.now, "")
	if len(got) != 3 || !got[canceled.ID] || !got[archived.ID] || !got[lapsed.ID] {
		t.Fatalf("purge claim = %v", got)
	}
	if again := claimedIDs(f.now, ""); len(again) != 0 {
		t.Fatalf("an image claimed for deletion was claimed again at once: %v", again)
	}
	if later := claimedIDs(f.now.Add(61*time.Second), ""); len(later) != 3 {
		t.Fatalf("after its backoff the failed deletes were claimed %v", later)
	}
	// Doubled: the second attempt waits two minutes.
	if soon := claimedIDs(f.now.Add(61*time.Second+90*time.Second), ""); len(soon) != 0 {
		t.Fatalf("the second backoff did not double: %v", soon)
	}
	if only := claimedIDs(f.now.Add(10*time.Minute), canceled.ID); len(only) != 1 || !only[canceled.ID] {
		t.Fatalf("a session's own claim = %v", only)
	}
}

// The workspace delete lists every handle before its rows cascade away, and
// the idle reaper asks which sessions are saving.
func TestPGRetainedImageListings(t *testing.T) {
	f := newRetainedFixture(t, "retain-list")
	ctx := context.Background()
	saving, savingImg := f.saving()
	kept, keptImg := f.kept(7, time.Hour)
	got, err := f.s.AgentRetainedImageIDs(ctx, f.ws)
	if err != nil || len(got) != 2 {
		t.Fatalf("ids = %v / %v", got, err)
	}
	have := map[string]bool{got[0]: true, got[1]: true}
	if !have[savingImg.ID] || !have[keptImg.ID] {
		t.Fatalf("ids = %v, want both handles", got)
	}
	savingNow, err := f.s.AgentSessionsSavingRetainedImage(ctx, []string{saving.ID, kept.ID, "ags-none"})
	if err != nil || len(savingNow) != 1 || !savingNow[saving.ID] {
		t.Fatalf("saving = %v / %v, want only the one still being saved", savingNow, err)
	}
	if none, err := f.s.AgentSessionsSavingRetainedImage(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("saving for no sessions = %v / %v", none, err)
	}
	all, err := f.s.AgentSessionRetainedImages(ctx, kept.ID)
	if err != nil || len(all) != 1 || all[0].ID != keptImg.ID {
		t.Fatalf("a session's handles = %+v / %v", all, err)
	}
	// Deleting the session drops its handle with it: the service deletes the
	// image first.
	if err := f.s.ReleaseAgentSessionRetainedImages(ctx, kept.ID, RetainedReleaseDeleted, f.now); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteAgentSession(ctx, kept.ID); err != nil {
		t.Fatal(err)
	}
	if n := f.rows(`SELECT count(*) FROM agent_session_retained_images WHERE id = $1`, keptImg.ID); n != 0 {
		t.Fatalf("a deleted session left its handle behind")
	}
	// A turn that starts fresh, in a sandbox that is not the retained disk,
	// supersedes every image of the session, a saving one included.
	if err := f.s.FinishAgentSessionRetainedImages(ctx, saving.ID, "", f.now); err != nil {
		t.Fatal(err)
	}
	f.none(saving)
	if ok, err := f.s.KeepAgentSessionRetainedImage(ctx, savingImg.ID, testImageDigest, f.now.Add(time.Hour)); err != nil || ok {
		t.Fatalf("a superseded save was kept: %v / %v", ok, err)
	}
	if turn := f.turn(saving); turn.Retention != TurnRetentionLost {
		t.Fatalf("a superseded turn = %+v", turn)
	}
}

// A save that ends the sandbox clears the session's sandbox id only while that
// is still the sandbox it ended: a session whose next turn already bound
// another keeps it.
func TestPGClearRetainedSandboxIsConditional(t *testing.T) {
	f := newRetainedFixture(t, "retain-clear")
	ctx := context.Background()
	session, _ := f.saving()
	read := func() string {
		row, err := f.s.GetAgentSession(ctx, session.ID)
		if err != nil {
			t.Fatal(err)
		}
		return row.SandboxID
	}
	if _, err := f.pool.Exec(ctx, `UPDATE agent_sessions SET sandbox_id = 'sb-new' WHERE id = $1`, session.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ClearAgentSessionRetainedSandbox(ctx, session.ID, "sb-old"); err != nil || read() != "sb-new" {
		t.Fatalf("a clear for another sandbox changed the id: %q / %v", read(), err)
	}
	if err := f.s.ClearAgentSessionRetainedSandbox(ctx, session.ID, "sb-new"); err != nil || read() != "" {
		t.Fatalf("the clear for the session's sandbox left %q / %v", read(), err)
	}
}

// A stopped turn's record carries what the API says about its disk
// (w22/m413 t004): "saving" for as long as the save runs, which the read
// derives from the image's row, and, once the image is released, why it was
// lost, copied onto the turn because the row is deleted after the purge. A
// disk that was continued keeps its outcome and no reason.
func TestPGTurnRecordsWhyItsDiskWasLost(t *testing.T) {
	f := newRetainedFixture(t, "retain-turn-reason")
	ctx := context.Background()

	// While the save runs the turn reads "saving"; a turn nobody saves reads
	// "" and so does one not stopped.
	session, img := f.saving()
	if turn := f.apiTurn(session); turn.Retention != TurnRetentionSaving || turn.RetentionReason != "" {
		t.Fatalf("turn while saving = %+v, want saving", turn)
	}
	// The plain read, the SSH gateway's, says what is recorded.
	if turn := f.turn(session); turn.Retention != "" {
		t.Fatalf("the plain read says %q while saving, want the recorded empty retention", turn.Retention)
	}
	var stored string
	if err := f.pool.QueryRow(ctx, `SELECT retention FROM agent_session_turns WHERE session_id = $1`, session.ID).Scan(&stored); err != nil || stored != "" {
		t.Fatalf("the stored retention while saving = %q / %v: \"saving\" is derived, never stored", stored, err)
	}
	if turn := f.apiTurn(f.running()); turn.Retention != "" {
		t.Fatalf("a running turn reads %+v", turn)
	}
	if err := f.s.ReserveAgentSessionRetainedImage(ctx, f.ws, img.ID, 4096, 1<<40); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.s.KeepAgentSessionRetainedImage(ctx, img.ID, testImageDigest, f.now.Add(time.Hour)); err != nil || !ok {
		t.Fatalf("keep = %v / %v", ok, err)
	}
	if turn := f.turn(session); turn.Retention != TurnRetentionKept || turn.RetentionReason != "" {
		t.Fatalf("turn once kept = %+v", turn)
	}

	// Each reason a release gives is copied onto the turn with the loss.
	for _, reason := range []string{RetainedReleaseExpired, RetainedReleaseStorageFull, RetainedReleaseSaveFailed, RetainedReleaseCanceled} {
		stopped, image := f.kept(4096, time.Hour)
		if ok, err := f.s.ReleaseAgentSessionRetainedImage(ctx, image.ID, reason, f.now); err != nil || !ok {
			t.Fatalf("release %s = %v / %v", reason, ok, err)
		}
		if turn := f.turn(stopped); turn.Retention != TurnRetentionLost || turn.RetentionReason != reason {
			t.Fatalf("turn released as %s = %+v", reason, turn)
		}
	}

	// A continuation records no reason, and a later release of another image
	// does not turn it into a loss.
	continued, used := f.kept(4096, time.Hour)
	if _, err := f.s.ClaimAgentSessionRetainedImage(ctx, continued.ID, f.now, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := f.s.FinishAgentSessionRetainedImages(ctx, continued.ID, used.ID, f.now); err != nil {
		t.Fatal(err)
	}
	if turn := f.turn(continued); turn.Retention != TurnRetentionContinued || turn.RetentionReason != "" {
		t.Fatalf("continued turn = %+v", turn)
	}
	if err := f.s.ReleaseAgentSessionRetainedImages(ctx, continued.ID, RetainedReleaseCanceled, f.now); err != nil {
		t.Fatal(err)
	}
	if turn := f.turn(continued); turn.Retention != TurnRetentionContinued || turn.RetentionReason != "" {
		t.Fatalf("a continued turn changed after a later release: %+v", turn)
	}

}

// A stopped turn that keeps nothing because an editor is open on its sandbox is
// settled and recorded lost, with the reason, by ONE transaction: the API never
// reads it settled without the reason, and a settle that does not happen
// records nothing. No image is begun.
func TestPGStoppedTurnSettlesAndRecordsWhyNothingIsKeptTogether(t *testing.T) {
	f := newRetainedFixture(t, "retain-editor-open")
	ctx := context.Background()
	editor := f.running()
	f.stop(editor)

	out, fact, err := f.s.FinalizeAgentSessionNotRetaining(ctx, editor.ID, "", "a delivery note", RetainedNotSavedEditorOpen)
	if err != nil || out.Phase != "completed" || fact.Turn != 1 || out.DeliveryNote != "a delivery note" {
		t.Fatalf("settle = %+v / %+v / %v", out, fact, err)
	}
	// One read: the settled session and the turn's reason agree.
	if turn := f.apiTurn(editor); turn.Retention != TurnRetentionLost || turn.RetentionReason != RetainedNotSavedEditorOpen || turn.CompletedAt == nil {
		t.Fatalf("turn with an open editor = %+v, want it settled and lost for editor_open", turn)
	}
	f.none(editor)
	if n := f.rows(`SELECT count(*) FROM agent_session_retained_images WHERE session_id = $1`, editor.ID); n != 0 {
		t.Fatalf("%d retained images for a turn that kept nothing", n)
	}

	// A session that has already left its live phase settles nothing and so
	// records nothing: ErrNotFound, and the first record stands.
	if _, _, err := f.s.FinalizeAgentSessionNotRetaining(ctx, editor.ID, "", "", RetainedReleaseExpired); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a second settle = %v, want ErrNotFound", err)
	}
	if turn := f.turn(editor); turn.RetentionReason != RetainedNotSavedEditorOpen {
		t.Fatalf("a refused settle changed the record: %+v", turn)
	}
	// A turn nobody stopped records nothing either.
	running := f.running()
	if _, _, err := f.s.FinalizeAgentSessionNotRetaining(ctx, running.ID, "", "", RetainedNotSavedEditorOpen); err != nil {
		t.Fatal(err)
	}
	if turn := f.turn(running); turn.Retention != "" || turn.RetentionReason != "" {
		t.Fatalf("an unstopped turn was recorded %+v", turn)
	}
}

// Whatever ended a turn (w10/m491: a stop, a failure, the idle close, the
// next message), a saving row for that turn reads as saving while the turn's
// own record is still undecided; another turn's saving row says nothing about
// it, and a decided record reads as recorded.
func TestPGATurnIsReadAsSavingBesideItsOwnSavingRow(t *testing.T) {
	f := newRetainedFixture(t, "retain-probe")
	ctx := context.Background()
	session := f.running() // not stopped
	if _, err := f.pool.Exec(ctx, `INSERT INTO agent_session_retained_images (id, session_id, workspace_id, turn, save_deadline)
		VALUES ($1, $2, $3, 2, now() + interval '5 minutes')`, ids.New(ids.AgentRetainedImage), session.ID, f.ws); err != nil {
		t.Fatal(err)
	}
	if turn := f.apiTurn(session); turn.Retention != "" {
		t.Fatalf("turn 1 reads %q beside turn 2's saving row, want it as recorded", turn.Retention)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE agent_session_retained_images SET turn = 1 WHERE session_id = $1`, session.ID); err != nil {
		t.Fatal(err)
	}
	if turn := f.apiTurn(session); turn.Retention != TurnRetentionSaving {
		t.Fatalf("a turn nobody stopped reads %q beside its own saving row, want saving", turn.Retention)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE agent_session_turns SET retention = 'lost' WHERE session_id = $1`, session.ID); err != nil {
		t.Fatal(err)
	}
	if turn := f.apiTurn(session); turn.Retention != TurnRetentionLost {
		t.Fatalf("a decided turn reads %q, want it as recorded", turn.Retention)
	}
}

// The SSH gateway's scoped database role reads agent_session_turns and no other
// table (sshgateway/dbrole, ADR035), and it calls AgentSessionTurns. That read
// must stay join-free: run under a role that holds SELECT on that one table, it
// still answers (with the retention as recorded), while the API's
// retention-aware read, which derives "saving" from the retained image's row, is
// refused by Postgres, so the derivation cannot leak into the gateway's path.
func TestPGAgentSessionTurnsTouchesNoOtherTable(t *testing.T) {
	f := newRetainedFixture(t, "retain-turns-role")
	ctx := context.Background()
	const role = "bex_turns_only_test"
	if _, err := f.pool.Exec(ctx, `DO $$ BEGIN
		IF EXISTS (SELECT FROM pg_roles WHERE rolname = '`+role+`') THEN
			EXECUTE 'DROP OWNED BY `+role+`';
			EXECUTE 'DROP ROLE `+role+`';
		END IF;
	END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `CREATE ROLE `+role+` NOLOGIN`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `GRANT USAGE ON SCHEMA public TO `+role+`; GRANT SELECT ON agent_session_turns TO `+role); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DROP OWNED BY `+role)
		_, _ = f.pool.Exec(context.Background(), `DROP ROLE IF EXISTS `+role)
	})

	cfg, err := pgxpool.ParseConfig(os.Getenv("BEX_TEST_DB_URI"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE `+role)
		return err
	}
	scoped, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(scoped.Close)
	gateway := NewPGStore(scoped)

	// A stopped turn whose save is running: the row the derivation would read.
	session, _ := f.saving()
	turns, err := gateway.AgentSessionTurns(ctx, session.ID)
	if err != nil || len(turns) != 1 {
		t.Fatalf("the gateway's read under a role that sees only agent_session_turns = %v / %v", turns, err)
	}
	if turns[0].Retention != "" || !turns[0].Interrupted {
		t.Fatalf("turn = %+v, want the retention as recorded (empty while saving), not derived", turns[0])
	}
	_, err = gateway.AgentSessionTurnsWithRetention(ctx, session.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("the retention-aware read under that role = %v, want permission denied (42501): it is the API's, never the gateway's", err)
	}
	// Under the API's own pool it says what the save is doing.
	if got, err := f.s.AgentSessionTurnsWithRetention(ctx, session.ID); err != nil || got[0].Retention != TurnRetentionSaving {
		t.Fatalf("the API's read = %+v / %v, want saving", got, err)
	}
}

// w10/m491: a settled turn's disk is kept when it failed, idled out or is
// replaced by the next message. The begin is the claim on pausing the sandbox:
// it takes only the sandbox the session still holds (or the one its new turn
// just replaced), never an archived session's, and never a second live row.
func TestPGBeginRetentionOfASettledTurn(t *testing.T) {
	f := newRetainedFixture(t, "retain-begin")
	ctx := context.Background()
	img := func() NewAgentRetainedImage {
		return NewAgentRetainedImage{ID: ids.New(ids.AgentRetainedImage), BaseImage: "agent:1", SaveDeadline: f.now.Add(5 * time.Minute)}
	}
	// Each session's sandbox id is its own (a live sandbox belongs to one
	// session); sbx names it.
	sbx := func(session AgentSession) string { return "sbx-" + session.ID }
	settled := func(phase string) AgentSession {
		session := f.running()
		if _, err := f.pool.Exec(ctx, `UPDATE agent_sessions SET phase = $2, sandbox_id = $3 WHERE id = $1`, session.ID, phase, sbx(session)); err != nil {
			t.Fatal(err)
		}
		return session
	}
	stake := TurnCheckout{Dirty: true, UnpushedCommits: 2}

	t.Run("the facts, then one claim on the held sandbox", func(t *testing.T) {
		session := settled("failed")
		if err := f.s.RecordAgentSessionTurnCheckout(ctx, session.ID, 1, TurnCheckout{UnpushedCommits: 1}); err != nil {
			t.Fatal(err)
		}
		if c, ok := f.turn(session).Checkout(); !ok || c.Dirty || c.UnpushedCommits != 1 {
			t.Fatalf("recorded facts = %+v (%t)", c, ok)
		}
		if got, err := f.s.BeginAgentSessionRetention(ctx, session.ID, 1, "sbx-other", TurnRetentionCauseFailed, stake, img()); err != nil || got != nil {
			t.Fatalf("a sandbox the session does not hold was claimed: %+v / %v", got, err)
		}
		got, err := f.s.BeginAgentSessionRetention(ctx, session.ID, 1, sbx(session), TurnRetentionCauseFailed, stake, img())
		if err != nil || got == nil || got.State != RetainedImageSaving || got.Turn != 1 {
			t.Fatalf("begin = %+v / %v", got, err)
		}
		turn := f.apiTurn(session)
		if turn.Retention != TurnRetentionSaving || turn.RetentionCause != TurnRetentionCauseFailed {
			t.Fatalf("turn = %+v, want saving for the failed turn", turn)
		}
		if c, _ := turn.Checkout(); c != stake {
			t.Fatalf("stake = %+v, want the one that decided it", c)
		}
		if again, err := f.s.BeginAgentSessionRetention(ctx, session.ID, 1, sbx(session), TurnRetentionCauseIdleClosed, stake, img()); err != nil || again != nil {
			t.Fatalf("a second begin = %+v / %v, want none", again, err)
		}
	})
	t.Run("the sandbox the next turn replaced", func(t *testing.T) {
		session := settled("completed")
		if _, err := f.s.BeginAgentSessionTurn(ctx, session.ID, "more", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		if got, err := f.s.BeginAgentSessionRetention(ctx, session.ID, 1, "sbx-2", TurnRetentionCauseReplaced, stake, img()); err != nil || got != nil {
			t.Fatalf("another sandbox = %+v / %v", got, err)
		}
		if got, err := f.s.BeginAgentSessionRetention(ctx, session.ID, 2, sbx(session), TurnRetentionCauseReplaced, stake, img()); err != nil || got != nil {
			t.Fatalf("the running turn = %+v / %v", got, err)
		}
		got, err := f.s.BeginAgentSessionRetention(ctx, session.ID, 1, sbx(session), TurnRetentionCauseReplaced, stake, img())
		if err != nil || got == nil || got.Turn != 1 {
			t.Fatalf("begin = %+v / %v", got, err)
		}
	})
	t.Run("an archived session keeps nothing", func(t *testing.T) {
		session := settled("completed")
		if _, err := f.pool.Exec(ctx, `UPDATE agent_sessions SET archived_at = now() WHERE id = $1`, session.ID); err != nil {
			t.Fatal(err)
		}
		if got, err := f.s.BeginAgentSessionRetention(ctx, session.ID, 1, sbx(session), TurnRetentionCauseIdleClosed, stake, img()); err != nil || got != nil {
			t.Fatalf("begin = %+v / %v", got, err)
		}
	})
	t.Run("a loss with no image, and a later keep clears it", func(t *testing.T) {
		session := settled("completed")
		if err := f.s.LoseAgentSessionTurnRetention(ctx, session.ID, 1, TurnRetentionCauseReplaced, RetainedNotSavedEditorOpen, &stake); err != nil {
			t.Fatal(err)
		}
		turn := f.turn(session)
		if turn.Retention != TurnRetentionLost || turn.RetentionReason != RetainedNotSavedEditorOpen || turn.RetentionCause != TurnRetentionCauseReplaced {
			t.Fatalf("turn = %+v", turn)
		}
		// A decided loss is not overwritten by a second one.
		if err := f.s.LoseAgentSessionTurnRetention(ctx, session.ID, 1, TurnRetentionCauseFailed, RetainedNotSavedSandboxLost, nil); err != nil {
			t.Fatal(err)
		}
		if turn := f.turn(session); turn.RetentionReason != RetainedNotSavedEditorOpen {
			t.Fatalf("turn = %+v, want the first loss", turn)
		}
		// The editor closed and the idle close kept the disk after all.
		got, err := f.s.BeginAgentSessionRetention(ctx, session.ID, 1, sbx(session), TurnRetentionCauseIdleClosed, stake, img())
		if err != nil || got == nil {
			t.Fatalf("begin = %+v / %v", got, err)
		}
		if ok, err := f.s.KeepAgentSessionRetainedImage(ctx, got.ID, testImageDigest, f.now.Add(time.Hour)); err != nil || !ok {
			t.Fatalf("keep = %v / %v", ok, err)
		}
		turn = f.turn(session)
		if turn.Retention != TurnRetentionKept || turn.RetentionReason != "" || turn.RetentionCause != TurnRetentionCauseIdleClosed {
			t.Fatalf("turn = %+v, want kept with no loss reason", turn)
		}
	})
}
