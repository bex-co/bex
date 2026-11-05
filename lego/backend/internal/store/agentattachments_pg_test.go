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
	"slices"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func insertUpload(t *testing.T, s *PGStore, workspaceID, caller string, size int64, expires time.Time) AgentAttachment {
	t.Helper()
	ctx := context.Background()
	id := ids.New(ids.AgentAttachment)
	if err := s.BeginAgentAttachment(ctx, AgentAttachment{
		ID: id, WorkspaceID: workspaceID, Direction: AgentAttachmentInput, Name: "log.txt",
		MediaType: "text/plain", SizeBytes: size, ObjectKey: workspaceID + "/" + id, CreatedBy: caller,
	}); err != nil {
		t.Fatal(err)
	}
	a, err := s.ReadyAgentAttachment(ctx, id, "sum", expires)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func retiredIDs(t *testing.T, s *PGStore) []string {
	t.Helper()
	batch, err := s.ClaimRetiredAgentAttachments(context.Background(), 10_000, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(batch))
	for i, a := range batch {
		out[i] = a.ID
	}
	return out
}

func TestAgentAttachmentPendingRowsAreNotReadable(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "attach-pending")
	ctx := context.Background()
	id := ids.New(ids.AgentAttachment)
	if err := s.BeginAgentAttachment(ctx, AgentAttachment{
		ID: id, WorkspaceID: tenant.ID, Direction: AgentAttachmentInput, Name: "x", MediaType: "text/plain",
		SizeBytes: 1, ObjectKey: tenant.ID + "/" + id, CreatedBy: "alice",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetAgentAttachment(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a pending upload is readable: %v", err)
	}
	// A crash before confirmation leaves the row to expire into the purge queue.
	if _, err := s.ExpireAgentAttachments(ctx, time.Now().Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(retiredIDs(t, s), id) {
		t.Fatal("an abandoned pending upload never reaches the sweep")
	}
}

func TestAgentAttachmentBindingIsAtomicWithTheTurn(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "attach-bind")
	ctx := context.Background()
	future := time.Now().Add(time.Hour)
	mine := insertUpload(t, s, tenant.ID, "alice", 10, future)
	theirs := insertUpload(t, s, tenant.ID, "bob", 10, future)

	// Another member's unbound upload refuses the whole create: no session row.
	_, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, InitialPrompt: "task",
		InitialAttachments: AgentAttachmentBinding{IDs: []string{mine.ID, theirs.ID}, Caller: "alice"},
	})
	var invalid *AgentAttachmentInvalidError
	if !errors.As(err, &invalid) || invalid.ID != theirs.ID {
		t.Fatalf("err = %v, want invalid %s", err, theirs.ID)
	}
	if got, _ := s.GetAgentAttachment(ctx, mine.ID); got.SessionID != "" {
		t.Fatal("a refused binding still bound the caller's own upload")
	}

	record, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, InitialPrompt: "task",
		InitialAttachments: AgentAttachmentBinding{IDs: []string{mine.ID}, Caller: "alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	bound, err := s.GetAgentAttachment(ctx, mine.ID)
	if err != nil || bound.SessionID != record.ID || bound.Turn != 1 || bound.ExpiresAt != nil {
		t.Fatalf("bound = %+v, %v", bound, err)
	}
	turns, err := s.AgentSessionTurns(ctx, record.ID)
	if err != nil || len(turns) != 1 || len(turns[0].Attachments) != 1 ||
		turns[0].Attachments[0] != (TurnAttachment{ID: mine.ID, Name: "log.txt", MediaType: "text/plain", Size: 10}) {
		t.Fatalf("turns = %+v, %v", turns, err)
	}
}

func TestAgentAttachmentQuotas(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "attach-quota")
	ctx := context.Background()
	pool := s.Pool
	big := insertUpload(t, s, tenant.ID, "alice", MaxAgentSessionAttachmentBytes, time.Now().Add(time.Hour))
	record, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, InitialPrompt: "task",
		InitialAttachments: AgentAttachmentBinding{IDs: []string{big.ID}, Caller: "alice"},
	})
	if err != nil {
		t.Fatalf("binding exactly the session cap = %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, record.ID); err != nil {
		t.Fatal(err)
	}
	one := insertUpload(t, s, tenant.ID, "alice", 1, time.Now().Add(time.Hour))
	_, err = s.BeginAgentSessionTurn(ctx, record.ID, "more", "redispatch", "redispatching", "redispatching",
		AgentAttachmentBinding{IDs: []string{one.ID}, Caller: "alice"}, AgentTurnOutput{})
	if !errors.Is(err, ErrAgentSessionAttachmentQuota) {
		t.Fatalf("err = %v, want the session quota", err)
	}
	turns, _ := s.AgentSessionTurns(ctx, record.ID)
	if len(turns) != 1 {
		t.Fatalf("a refused binding recorded turn %d", len(turns))
	}
	// The uploader's unbound bytes are capped too.
	id := ids.New(ids.AgentAttachment)
	err = s.BeginAgentAttachment(ctx, AgentAttachment{
		ID: id, WorkspaceID: tenant.ID, Direction: AgentAttachmentInput, Name: "x", MediaType: "text/plain",
		SizeBytes: MaxUnboundAttachmentBytes + 1, ObjectKey: tenant.ID + "/" + id, CreatedBy: "dave",
	})
	if !errors.Is(err, ErrUnboundAttachmentQuota) {
		t.Fatalf("err = %v, want the uploader quota", err)
	}
}

func TestDeletesRetireAttachmentsInTheSameTransaction(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "attach-delete")
	ctx := context.Background()
	future := time.Now().Add(time.Hour)
	bound := insertUpload(t, s, tenant.ID, "alice", 1, future)
	unbound := insertUpload(t, s, tenant.ID, "alice", 1, future)
	record, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, InitialPrompt: "task",
		InitialAttachments: AgentAttachmentBinding{IDs: []string{bound.ID}, Caller: "alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// A delete that finds nothing to delete keeps the files.
	if err := s.DeleteAgentSession(ctx, ids.New(ids.AgentSession)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing = %v", err)
	}
	if err := s.DeleteAgentSession(ctx, record.ID); err != nil {
		t.Fatal(err)
	}
	if retired := retiredIDs(t, s); !slices.Contains(retired, bound.ID) || slices.Contains(retired, unbound.ID) {
		t.Fatalf("retired after session delete = %v", retired)
	}
	// The workspace delete retires what is left; the rows survive the tenant
	// row so the sweep can still purge their objects.
	if err := s.DeleteTenant(ctx, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(retiredIDs(t, s), unbound.ID) {
		t.Fatal("a deleted workspace's upload never reaches the sweep")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM agent_session_attachments WHERE workspace_id=$1`, tenant.ID)
	})
}

func TestRetiredAttachmentsPurgeInBatchesThatDoNotStarve(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "attach-purge")
	ctx := context.Background()
	first := insertUpload(t, s, tenant.ID, "alice", 1, time.Now().Add(-time.Minute))
	second := insertUpload(t, s, tenant.ID, "alice", 1, time.Now().Add(-time.Minute))
	if _, err := s.ExpireAgentAttachments(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	// A row tried and left behind waits behind rows never tried.
	stuck, err := s.ClaimRetiredAgentAttachments(ctx, 1, now)
	if err != nil || len(stuck) != 1 {
		t.Fatalf("first claim = %+v, %v", stuck, err)
	}
	next, err := s.ClaimRetiredAgentAttachments(ctx, 1, now.Add(time.Second))
	if err != nil || len(next) != 1 || next[0].ID == stuck[0].ID {
		t.Fatalf("second claim = %+v, want the row not yet tried", next)
	}
	if err := s.PurgeAgentAttachments(ctx, []string{first.ID, second.ID}); err != nil {
		t.Fatal(err)
	}
	if retired := retiredIDs(t, s); slices.Contains(retired, first.ID) || slices.Contains(retired, second.ID) {
		t.Fatalf("purged rows still queued: %v", retired)
	}
}

func TestAgentAttachmentListPagesByKeyset(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "attach-list")
	ctx := context.Background()
	future := time.Now().Add(time.Hour)
	var uploads []string
	for range 3 {
		uploads = append(uploads, insertUpload(t, s, tenant.ID, "alice", 1, future).ID)
	}
	record, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, InitialPrompt: "task",
		InitialAttachments: AgentAttachmentBinding{IDs: uploads, Caller: "alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.ListAgentSessionAttachments(ctx, record.ID, AgentAttachmentListQuery{Limit: 2})
	if err != nil || len(first) != 2 {
		t.Fatalf("first page = %+v, %v", first, err)
	}
	rest, err := s.ListAgentSessionAttachments(ctx, record.ID, AgentAttachmentListQuery{Limit: 2, Cursor: first[1].ID})
	if err != nil || len(rest) != 1 || rest[0].ID == first[0].ID || rest[0].ID == first[1].ID {
		t.Fatalf("second page = %+v, %v", rest, err)
	}
	if turn1, _ := s.ListAgentSessionAttachments(ctx, record.ID, AgentAttachmentListQuery{Turn: 1}); len(turn1) != 3 {
		t.Fatalf("turn 1 = %d rows", len(turn1))
	}
	if outputs, _ := s.ListAgentSessionAttachments(ctx, record.ID, AgentAttachmentListQuery{Direction: AgentAttachmentOutput}); len(outputs) != 0 {
		t.Fatalf("outputs = %+v, want none", outputs)
	}
}

// Concurrent agent outputs race for a session's last quota slots: the session
// lock serializes them, so exactly the room left is admitted and no writer
// deadlocks.
func TestConcurrentOutputsNeverPassTheSessionQuota(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "attach-race")
	ctx := context.Background()
	record, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "task"})
	if err != nil {
		t.Fatal(err)
	}
	// Leave exactly 3 file slots in the session.
	for range MaxAgentSessionAttachments - 3 {
		id := ids.New(ids.AgentAttachment)
		if err := s.BeginAgentAttachment(ctx, AgentAttachment{
			ID: id, WorkspaceID: tenant.ID, SessionID: record.ID, Turn: 1, Direction: AgentAttachmentOutput,
			Name: "o", MediaType: "text/plain", SizeBytes: 1, ObjectKey: tenant.ID + "/" + id, CreatedBy: "agent",
		}); err != nil {
			t.Fatal(err)
		}
	}
	const writers = 12
	results := make(chan error, writers)
	for range writers {
		go func() {
			id := ids.New(ids.AgentAttachment)
			results <- s.BeginAgentAttachment(ctx, AgentAttachment{
				ID: id, WorkspaceID: tenant.ID, SessionID: record.ID, Turn: 1, Direction: AgentAttachmentOutput,
				Name: "o", MediaType: "text/plain", SizeBytes: 1, ObjectKey: tenant.ID + "/" + id, CreatedBy: "agent",
			})
		}()
	}
	admitted := 0
	for range writers {
		switch err := <-results; {
		case err == nil:
			admitted++
		case errors.Is(err, ErrAgentSessionAttachmentQuota):
		default:
			t.Fatalf("a concurrent output failed with %v (deadlock?)", err)
		}
	}
	if admitted != 3 {
		t.Fatalf("admitted %d concurrent outputs into 3 free slots", admitted)
	}
}
