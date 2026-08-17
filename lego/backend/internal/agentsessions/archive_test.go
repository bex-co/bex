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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// --- ADR065 D1: archive/unarchive + the AGENT_SESSION_ARCHIVED gate ---------

// Archive is idempotent, orthogonal to phase, and reversible; while archived
// the mutation verbs refuse with the coded 409 and reads keep working.
func TestArchiveUnarchiveLifecycle(t *testing.T) {
	svc, _, _, id := steerableFixture(t)

	view, err := svc.Archive(caller("alice"), id)
	if err != nil || view.ArchivedAt == nil {
		t.Fatalf("archive = %+v err=%v", view, err)
	}
	if view.Phase != PhaseCompleted {
		t.Fatalf("archive must not touch phase, got %q", view.Phase)
	}
	again, err := svc.Archive(caller("alice"), id)
	if err != nil || again.ArchivedAt == nil || !again.ArchivedAt.Equal(*view.ArchivedAt) {
		t.Fatalf("re-archive must be idempotent: %+v err=%v", again, err)
	}

	// Mutation verbs refuse while archived — one identical coded 409 everywhere.
	if _, err := svc.Resume(caller("alice"), id); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("resume archived = %v, want AGENT_SESSION_ARCHIVED", err)
	}
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "go"}); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("steer archived = %v, want AGENT_SESSION_ARCHIVED", err)
	}
	if _, err := svc.Pin(caller("alice"), id); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("pin archived = %v, want AGENT_SESSION_ARCHIVED", err)
	}
	if _, err := svc.Unpin(caller("alice"), id); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("unpin archived = %v, want AGENT_SESSION_ARCHIVED", err)
	}

	// Reads always work: viewable is the point of the archive.
	if got, err := svc.Get(caller("alice"), id); err != nil || got.ArchivedAt == nil {
		t.Fatalf("get archived = %+v err=%v", got, err)
	}

	// Unarchive restores the mutation verbs (the steer now fails on its own
	// merits — it dispatches — so just assert the archived gate is gone).
	restored, err := svc.Unarchive(caller("alice"), id)
	if err != nil || restored.ArchivedAt != nil {
		t.Fatalf("unarchive = %+v err=%v", restored, err)
	}
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "go"}); isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("steer after unarchive still gated: %v", err)
	}
}

// Cancel is the safety verb: it stays allowed on an archived session.
func TestCancelAllowedWhileArchived(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	if _, err := svc.Archive(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	view, err := svc.Cancel(caller("alice"), id)
	if err != nil || view.Phase != PhaseCanceled {
		t.Fatalf("cancel archived = %+v err=%v", view, err)
	}
	_ = st
}

// A live-turn ticket is refused on an archived session at mint time, and the
// gateway revalidator refuses it at redemption time too (an archive landing
// inside the ticket TTL). Read tickets stay available — viewable is the point.
func TestArchivedRefusesTurnTicketsMintAndRedemption(t *testing.T) {
	svc, st, fga, _ := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(caller("alice"), created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionTurn); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("turn ticket on archived = %v, want AGENT_SESSION_ARCHIVED", err)
	}
	if _, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionRead); err != nil {
		t.Fatalf("read ticket on archived = %v, want allowed", err)
	}
	reval := &AttachRevalidator{Base: svc.Base, Store: st}
	if err := reval.RevalidateAttach(context.Background(), "alice", created.ID, agentsessionticket.ActionTurn); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("redemption revalidation on archived = %v, want AGENT_SESSION_ARCHIVED", err)
	}
	if err := reval.RevalidateAttach(context.Background(), "alice", created.ID, agentsessionticket.ActionRead); err != nil {
		t.Fatalf("read revalidation on archived = %v, want allowed", err)
	}
	_ = fga
}

// Create with resumeSessionId routes through Resume, so it hits the same gate.
func TestCreateResumeOfArchivedRefused(t *testing.T) {
	svc, _, _, id := steerableFixture(t)
	if _, err := svc.Archive(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(caller("alice"), CreateRequest{ResumeSessionID: id}); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("create-resume archived = %v, want AGENT_SESSION_ARCHIVED", err)
	}
}

// --- ADR065 D3: the filtered, page-bounded list ------------------------------

// The default list is the unarchived working set; archived=true/all widen it;
// invalid filter vocabulary is a named 400, never a silent full or empty list.
func TestListDefaultsToWorkingSetAndValidatesFilters(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	other, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(caller("alice"), other.ID); err != nil {
		t.Fatal(err)
	}

	working, err := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a"})
	if err != nil || len(working) != 1 || working[0].ID != id {
		t.Fatalf("default list = %+v err=%v, want only the unarchived session", working, err)
	}
	archivedOnly, err := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a", Archived: ArchivedTrue})
	if err != nil || len(archivedOnly) != 1 || archivedOnly[0].ID != other.ID {
		t.Fatalf("archived=true list = %+v err=%v", archivedOnly, err)
	}
	all, err := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a", Archived: ArchivedAll})
	if err != nil || len(all) != 2 {
		t.Fatalf("archived=all list = %d rows err=%v", len(all), err)
	}

	if _, err := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a", Archived: "bogus"}); !isCode(err, "AGENT_SESSION_FILTER_INVALID") {
		t.Fatalf("bogus archived filter = %v, want AGENT_SESSION_FILTER_INVALID", err)
	}
	if _, err := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a", Phases: []string{"exploded"}}); !isCode(err, "AGENT_SESSION_FILTER_INVALID") {
		t.Fatalf("bogus phase filter = %v, want AGENT_SESSION_FILTER_INVALID", err)
	}
	if _, err := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a", Limit: -1}); !isCode(err, "AGENT_SESSION_FILTER_INVALID") {
		t.Fatalf("negative limit = %v, want AGENT_SESSION_FILTER_INVALID", err)
	}
	_ = st
}

// The normalized store query carries the default and the clamp: omitted limit
// becomes defaultListLimit, an oversized one clamps to maxListLimit.
func TestListLimitNormalization(t *testing.T) {
	req := ListRequest{}
	q, err := req.storeQuery()
	if err != nil || q.Limit != defaultListLimit || q.Archived != store.ArchivedExclude {
		t.Fatalf("default query = %+v err=%v", q, err)
	}
	req = ListRequest{Limit: 10_000}
	if q, err = req.storeQuery(); err != nil || q.Limit != maxListLimit {
		t.Fatalf("oversized limit = %+v err=%v", q, err)
	}
}

// --- ADR065 D4: delete ------------------------------------------------------

// freshDenyFGA allows the cached path but refuses the fresh re-check — the
// revoked-inside-the-cache-TTL scenario the destructive sink must fail on.
type freshDenyFGA struct{ *fakeFGA }

func (f *freshDenyFGA) CheckFresh(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func TestDeleteGuardsAndOrdering(t *testing.T) {
	svc, st, lc, id := steerableFixture(t)
	snaps := newFakeSnapshots()
	svc.Snapshots = snaps

	// A live-phase session must cancel first.
	row := st.rows[id]
	row.Phase = PhaseRunning
	st.rows[id] = row
	if err := svc.Delete(caller("alice"), id); !isCode(err, "AGENT_SESSION_NOT_DELETABLE") {
		t.Fatalf("delete running = %v, want AGENT_SESSION_NOT_DELETABLE", err)
	}

	// A hibernated session: the snapshot blob is deleted BEFORE the row.
	seedHibernated(st, id)
	ref := st.rows[id].SnapshotRef
	if err := svc.Delete(caller("alice"), id); err != nil {
		t.Fatalf("delete hibernated = %v", err)
	}
	if len(snaps.deleted) != 1 || snaps.deleted[0] != ref {
		t.Fatalf("snapshot not deleted: %v", snaps.deleted)
	}
	if _, ok := st.rows[id]; ok {
		t.Fatalf("row survived delete")
	}
	_ = lc
}

// A terminal session still holding its idle-grace sandbox is torn down before
// the row is deleted, so the reaper can never be left with an orphaned pod.
func TestDeleteTearsDownIdleGraceSandboxFirst(t *testing.T) {
	svc, st, lc, id := steerableFixture(t)
	row := st.rows[id]
	row.SandboxID = "sandbox-1" // completed, sandbox retained by the idle grace
	st.rows[id] = row
	before := lc.canceled
	if err := svc.Delete(caller("alice"), id); err != nil {
		t.Fatalf("delete = %v", err)
	}
	if lc.canceled != before+1 {
		t.Fatalf("sandbox not terminated before delete (canceled=%d)", lc.canceled)
	}
	if _, ok := st.rows[id]; ok {
		t.Fatalf("row survived delete")
	}
}

// The irreversible sink is gated by the FRESH re-check: a cached allow with a
// fresh deny refuses, and the row survives.
func TestDeleteRefusedByFreshRecheck(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	fga, _ := svc.Base.Authz.(*fakeFGA)
	svc.Base.Authz = &freshDenyFGA{fakeFGA: fga}
	if err := svc.Delete(caller("alice"), id); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("delete with fresh deny = %v, want ErrForbidden", err)
	}
	if _, ok := st.rows[id]; !ok {
		t.Fatalf("row deleted despite fresh denial")
	}
}

// --- ADR065 D2: replay-only attach tickets ----------------------------------

// A reaped terminal/hibernated session (empty sandbox id) mints a replay-only
// read ticket whose pod triple is empty; a provisioning session still refuses,
// and a turn ticket can never be pod-less.
func TestAttachTicketReplayOnlyForReapedSessions(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	row := st.rows[id]
	row.SandboxID = "" // reaped: idle grace elapsed, pod gone
	st.rows[id] = row

	view, err := svc.AttachTicket(caller("alice"), id, agentsessionticket.ActionRead)
	if err != nil || view.Ticket == "" {
		t.Fatalf("replay-only mint = %+v err=%v", view, err)
	}
	claims, err := agentsessionticket.Verify(svc.TicketSecret, view.Ticket, st.now)
	if err != nil {
		t.Fatalf("verify replay-only ticket: %v", err)
	}
	if claims.Pod != "" || claims.SandboxID != "" || claims.Namespace != "" {
		t.Fatalf("replay-only ticket must carry an empty pod triple: %+v", claims)
	}
	if claims.SessionID != id || claims.Action != agentsessionticket.ActionRead {
		t.Fatalf("replay-only claims = %+v", claims)
	}

	// Turn tickets on a reaped session stay refused.
	if _, err := svc.AttachTicket(caller("alice"), id, agentsessionticket.ActionTurn); !isCode(err, "AGENT_SESSION_NOT_ATTACHABLE") {
		t.Fatalf("turn on reaped = %v, want AGENT_SESSION_NOT_ATTACHABLE", err)
	}

	// A provisioning session (creating, no sandbox yet) keeps the retryable refusal.
	row = st.rows[id]
	row.Phase = PhaseCreating
	st.rows[id] = row
	if _, err := svc.AttachTicket(caller("alice"), id, agentsessionticket.ActionRead); !isCode(err, "AGENT_SESSION_NOT_ATTACHABLE") {
		t.Fatalf("read on provisioning = %v, want AGENT_SESSION_NOT_ATTACHABLE", err)
	}

	// Hibernated is replayable too.
	seedHibernated(st, id)
	if _, err := svc.AttachTicket(caller("alice"), id, agentsessionticket.ActionRead); err != nil {
		t.Fatalf("read on hibernated = %v, want allowed", err)
	}
}

// --- ADR065 D2: the poll-shaped transcript read ------------------------------

func TestTranscriptPagesVerbatimParts(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	parts := []store.AgentSessionTranscriptPart{
		{Seq: 0, Turn: 1, Part: []byte(`{"type":"start"}`)},
		{Seq: 1, Turn: 1, Part: []byte(`{"type":"text-delta","delta":"hi"}`)},
		{Seq: 2, Turn: 2, Part: []byte(`{"type":"finish"}`)},
	}
	if err := st.AppendAgentSessionTranscript(context.Background(), id, parts); err != nil {
		t.Fatal(err)
	}

	page, err := svc.Transcript(caller("alice"), id, -1, 2)
	if err != nil || len(page.Parts) != 2 || page.NextAfterSeq != 1 {
		t.Fatalf("page 1 = %+v err=%v", page, err)
	}
	if string(page.Parts[1].Part) != `{"type":"text-delta","delta":"hi"}` {
		t.Fatalf("part not verbatim: %s", page.Parts[1].Part)
	}
	tail, err := svc.Transcript(caller("alice"), id, page.NextAfterSeq, 0)
	if err != nil || len(tail.Parts) != 1 || tail.Parts[0].Seq != 2 || tail.NextAfterSeq != 2 {
		t.Fatalf("tail page = %+v err=%v", tail, err)
	}
	end, err := svc.Transcript(caller("alice"), id, tail.NextAfterSeq, 0)
	if err != nil || len(end.Parts) != 0 || end.NextAfterSeq != 2 {
		t.Fatalf("end page = %+v err=%v", end, err)
	}

	// Archived sessions stay readable (viewable is the point of the archive).
	if _, err := svc.Archive(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	if page, err := svc.Transcript(caller("alice"), id, -1, 0); err != nil || len(page.Parts) != 3 {
		t.Fatalf("archived transcript read = %+v err=%v", page, err)
	}

	// Unknown session is a 404; cross-workspace caller is denied.
	if _, err := svc.Transcript(caller("alice"), "ags-doesnotexist00000000", -1, 0); !errors.Is(err, core.ErrForbidden) && !isCode(err, "AGENT_SESSION_NOT_FOUND") {
		t.Fatalf("unknown session = %v", err)
	}
	if _, err := svc.Transcript(caller("bob"), id, -1, 0); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-workspace transcript = %v, want forbidden", err)
	}
}

// --- ADR065 D1: archive zeroes the Completer idle grace ----------------------

// An archived finished session skips the idle grace: the reaper reclaims it at
// the very next tick, while an unarchived twin would still be deferred.
func TestCompleterReclaimsArchivedImmediately(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	c.IdleTTL = 30 * time.Minute
	base := st.now
	c.Now = func() time.Time { return base }

	c.Reconcile(context.Background()) // finalize; idle ≈ 0 < 30m ⇒ deferred
	if lc.canceled != 0 || st.rows[id].SandboxID == "" {
		t.Fatalf("sandbox reaped inside the idle grace (canceled=%d)", lc.canceled)
	}

	// The tenant archives the finished session: explicit disinterest.
	if _, err := st.SetAgentSessionArchived(context.Background(), id, true); err != nil {
		t.Fatal(err)
	}
	c.Reconcile(context.Background()) // same instant — grace skipped
	if lc.canceled != 1 {
		t.Fatalf("archived session not reclaimed at next tick (canceled=%d)", lc.canceled)
	}
	if st.rows[id].SandboxID != "" {
		t.Fatalf("sandbox id not cleared: %q", st.rows[id].SandboxID)
	}
}

// --- REST adapter plumbing (ADR065 D3/D4) ------------------------------------

// The REST list forwards the filter/pagination params and rejects a bad limit;
// DELETE answers 204 on success and the coded 409 on a live session.
func TestArchiveRESTAdapters(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	other, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(caller("alice"), other.ID); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	get := func(path string) (*httptest.ResponseRecorder, []View) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(caller("alice"))
		mux.ServeHTTP(rec, req)
		var views []View
		_ = json.Unmarshal(rec.Body.Bytes(), &views)
		return rec, views
	}

	// Default list excludes the archived session; archived=true is only it.
	if rec, views := get("/v1/agent-sessions?ownerId=tea-a"); rec.Code != http.StatusOK || len(views) != 1 || views[0].ID != id {
		t.Fatalf("default REST list = %d %+v", rec.Code, views)
	}
	if rec, views := get("/v1/agent-sessions?ownerId=tea-a&archived=true"); rec.Code != http.StatusOK || len(views) != 1 || views[0].ID != other.ID {
		t.Fatalf("archived REST list = %d %+v", rec.Code, views)
	}
	// Phase filter narrows within archived=all.
	if rec, views := get("/v1/agent-sessions?ownerId=tea-a&archived=all&phase=completed"); rec.Code != http.StatusOK || len(views) != 1 || views[0].ID != id {
		t.Fatalf("phase REST list = %d %+v", rec.Code, views)
	}
	// A present-but-invalid limit is a 400, never a silent default page.
	if rec, _ := get("/v1/agent-sessions?ownerId=tea-a&limit=0"); rec.Code != http.StatusBadRequest {
		t.Fatalf("limit=0 = %d, want 400", rec.Code)
	}
	if rec, _ := get("/v1/agent-sessions?ownerId=tea-a&archived=bogus"); rec.Code != http.StatusBadRequest {
		t.Fatalf("archived=bogus = %d, want 400", rec.Code)
	}

	// Transcript read: a malformed afterSeq is a 400.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/agent-sessions/"+id+"/transcript?afterSeq=abc", nil).WithContext(caller("alice"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad afterSeq = %d, want 400", rec.Code)
	}

	// DELETE: live session is the coded 409; a finished one deletes with 204.
	row := st.rows[other.ID]
	row.Phase = PhaseRunning
	st.rows[other.ID] = row
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/v1/agent-sessions/"+other.ID, nil).WithContext(caller("alice"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("DELETE running = %d %s, want 409", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/v1/agent-sessions/"+id, nil).WithContext(caller("alice"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE finished = %d %s, want 204", rec.Code, rec.Body.String())
	}
	if _, ok := st.rows[id]; ok {
		t.Fatalf("row survived REST delete")
	}
}

// The GraphQL surface exposes the same verbs + filters: archive via mutation,
// the archived list filter, and delete returning true.
func TestArchiveGraphQLSurface(t *testing.T) {
	svc, _, _, id := steerableFixture(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	do := func(q string) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: q})
	}

	if r := do(`mutation { archiveAgentSession(id:"` + id + `") { id archivedAt } }`); len(r.Errors) != 0 {
		t.Fatalf("archive mutation errors = %#v", r.Errors)
	}
	r := do(`{ agentSessions(ownerId:"tea-a", archived:"true") { id archivedAt } }`)
	if len(r.Errors) != 0 {
		t.Fatalf("archived list errors = %#v", r.Errors)
	}
	rows := r.Data.(map[string]any)["agentSessions"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["id"] != id || rows[0].(map[string]any)["archivedAt"] == nil {
		t.Fatalf("archived list = %#v", rows)
	}
	if r := do(`{ agentSessions(ownerId:"tea-a") { id } }`); len(r.Errors) != 0 || len(r.Data.(map[string]any)["agentSessions"].([]any)) != 0 {
		t.Fatalf("default list should exclude the archived session: %#v", r)
	}
	if r := do(`mutation { unarchiveAgentSession(id:"` + id + `") { archivedAt } }`); len(r.Errors) != 0 {
		t.Fatalf("unarchive mutation errors = %#v", r.Errors)
	}
	r = do(`mutation { deleteAgentSession(id:"` + id + `") }`)
	if len(r.Errors) != 0 || r.Data.(map[string]any)["deleteAgentSession"] != true {
		t.Fatalf("delete mutation = %#v errors=%#v", r.Data, r.Errors)
	}
}
