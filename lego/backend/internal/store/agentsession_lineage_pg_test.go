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
	"sync"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func lineageSession(t *testing.T, s *PGStore, workspaceID, parentID string, ceiling *int64) AgentSession {
	t.Helper()
	row, err := s.CreateAgentSession(context.Background(), AgentSession{
		WorkspaceID: workspaceID, AgentConfig: []byte(`{"agent":"claude","task":"x"}`), InitialPrompt: "x",
		ParentSessionID: parentID, MaxSpendMicroUSD: ceiling,
	})
	if err != nil {
		t.Fatalf("create under %q: %v", parentID, err)
	}
	return row
}

func setPhase(t *testing.T, s *PGStore, id, phase string) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `UPDATE agent_sessions SET phase = $2 WHERE id = $1`, id, phase); err != nil {
		t.Fatal(err)
	}
}

// ADR102 D1/D2: a child records its parent, root and depth; the lineage is
// derived from the locked parent row, never taken from the caller.
func TestPGAgentSessionLineageAndDepthLimit(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "lineage-depth")
	ctx := context.Background()
	root := lineageSession(t, s, tenant.ID, "", nil)
	if root.ParentSessionID != "" || root.RootSessionID != "" || root.Depth != 0 {
		t.Fatalf("a top-level session has lineage %+v", root)
	}
	child := lineageSession(t, s, tenant.ID, root.ID, nil)
	grand := lineageSession(t, s, tenant.ID, child.ID, nil)
	great := lineageSession(t, s, tenant.ID, grand.ID, nil)
	if child.ParentSessionID != root.ID || child.RootSessionID != root.ID || child.Depth != 1 ||
		grand.ParentSessionID != child.ID || grand.RootSessionID != root.ID || grand.Depth != 2 ||
		great.RootSessionID != root.ID || great.Depth != MaxAgentSessionDepth {
		t.Fatalf("lineage: child %+v grand %+v great %+v", child, grand, great)
	}
	// A caller cannot set its own root or depth.
	forged, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x", ParentSessionID: root.ID, RootSessionID: "ags-forged", Depth: 0})
	if err != nil || forged.RootSessionID != root.ID || forged.Depth != 1 {
		t.Fatalf("forged lineage = %+v / %v", forged, err)
	}
	_, err = s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x", ParentSessionID: great.ID})
	if !errors.Is(err, ErrAgentSessionDepthLimit) {
		t.Fatalf("a fourth level = %v, want ErrAgentSessionDepthLimit", err)
	}
	// The schema backs the limit up.
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET depth = 4 WHERE id = $1`, great.ID); err == nil {
		t.Fatal("the depth CHECK accepted 4")
	}
}

func TestPGAgentSessionParentRefusals(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "lineage-refusals")
	_, _, other := agentSessionPGStore(t, "lineage-refusals-other")
	ctx := context.Background()
	parent := lineageSession(t, s, tenant.ID, "", nil)
	foreign := lineageSession(t, s, other.ID, "", nil)

	create := func(workspaceID, parentID string) error {
		_, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: workspaceID, InitialPrompt: "x", ParentSessionID: parentID})
		return err
	}
	if err := create(tenant.ID, foreign.ID); !errors.Is(err, ErrAgentSessionParentNotFound) {
		t.Fatalf("a parent in another workspace = %v, want not found", err)
	}
	if err := create(other.ID, parent.ID); !errors.Is(err, ErrAgentSessionParentNotFound) {
		t.Fatalf("a child in another workspace = %v, want not found", err)
	}
	if err := create(tenant.ID, ids.New(ids.AgentSession)); !errors.Is(err, ErrAgentSessionParentNotFound) {
		t.Fatalf("an unknown parent = %v, want not found", err)
	}
	for _, phase := range []string{"completed", "failed", "canceling", "canceled"} {
		setPhase(t, s, parent.ID, phase)
		if err := create(tenant.ID, parent.ID); !errors.Is(err, ErrAgentSessionParentInactive) {
			t.Fatalf("a %s parent = %v, want ErrAgentSessionParentInactive", phase, err)
		}
	}
	setPhase(t, s, parent.ID, "running")
	if err := create(tenant.ID, parent.ID); err != nil {
		t.Fatalf("a running parent = %v", err)
	}
	if n, _ := s.CountLiveAgentSessionChildren(ctx, tenant.ID, parent.ID); n != 1 {
		t.Fatalf("refused creates left %d children, want 1", n)
	}
}

// ADR102 D2: the fan-out limit holds under concurrency. Two creates racing at
// the limit minus one admit exactly one; finished children do not count.
func TestPGAgentSessionFanOutLimitIsAtomic(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "lineage-fanout")
	ctx := context.Background()
	parent := lineageSession(t, s, tenant.ID, "", nil)
	for range MaxLiveAgentSessionChildren - 1 {
		lineageSession(t, s, tenant.ID, parent.ID, nil)
	}
	for round := range 5 {
		var wg sync.WaitGroup
		errs := make([]error, 2)
		made := make([]string, 2)
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x", ParentSessionID: parent.ID})
				errs[i], made[i] = err, row.ID
			}()
		}
		wg.Wait()
		admitted, refused := 0, 0
		for i, err := range errs {
			switch {
			case err == nil:
				admitted++
				setPhase(t, s, made[i], "completed") // make room for the next round
			case errors.Is(err, ErrAgentSessionFanOutLimit):
				refused++
			default:
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if admitted != 1 || refused != 1 {
			t.Fatalf("round %d: admitted %d refused %d, want exactly one of each", round, admitted, refused)
		}
	}
	if n, err := s.CountLiveAgentSessionChildren(ctx, tenant.ID, parent.ID); err != nil || n != MaxLiveAgentSessionChildren-1 {
		t.Fatalf("live children = %d / %v", n, err)
	}
	all, err := s.ListAgentSessionChildren(ctx, tenant.ID, parent.ID)
	if err != nil || len(all) != MaxLiveAgentSessionChildren-1+5 {
		t.Fatalf("children = %d / %v, want every phase listed", len(all), err)
	}
}

// Subtree, descendants-to-cancel, the list filter and the cancel cause.
func TestPGAgentSessionTreeQueries(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "lineage-tree")
	ctx := context.Background()
	root := lineageSession(t, s, tenant.ID, "", nil)
	a := lineageSession(t, s, tenant.ID, root.ID, nil)
	b := lineageSession(t, s, tenant.ID, root.ID, nil)
	a1 := lineageSession(t, s, tenant.ID, a.ID, nil)
	a11 := lineageSession(t, s, tenant.ID, a1.ID, nil)
	unrelated := lineageSession(t, s, tenant.ID, "", nil)
	setPhase(t, s, b.ID, "completed")
	setPhase(t, s, a11.ID, "canceling")

	tree, err := s.AgentSessionSubtree(ctx, tenant.ID, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range tree {
		got = append(got, r.ID)
	}
	if want := []string{root.ID, a.ID, b.ID, a1.ID, a11.ID}; !slices.Equal(got, want) {
		t.Fatalf("subtree = %v, want %v", got, want)
	}
	if tree[2].Phase != "completed" || tree[4].Phase != "canceling" {
		t.Fatalf("subtree phases = %q %q", tree[2].Phase, tree[4].Phase)
	}
	if other, _ := s.AgentSessionSubtree(ctx, "tea-nobody", root.ID); len(other) != 0 {
		t.Fatalf("another workspace read a tree: %d rows", len(other))
	}

	// The cascade query: unfinished descendants only, deepest first.
	live, err := s.ListUnfinishedAgentSessionDescendants(ctx, tenant.ID, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, r := range live {
		got = append(got, r.ID)
	}
	if want := []string{a11.ID, a1.ID, a.ID}; !slices.Equal(got, want) {
		t.Fatalf("unfinished descendants = %v, want %v (b is completed, the root and unrelated are not below)", got, want)
	}
	if mid, _ := s.ListUnfinishedAgentSessionDescendants(ctx, tenant.ID, a.ID); len(mid) != 2 {
		t.Fatalf("below a = %d, want 2", len(mid))
	}

	page, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{ParentSessionID: root.ID, Archived: ArchivedInclude, Limit: 10})
	if err != nil || len(page) != 2 {
		t.Fatalf("list by parent = %d / %v, want 2", len(page), err)
	}
	for _, r := range page {
		if r.ID == unrelated.ID || r.ParentSessionID != root.ID {
			t.Fatalf("list by parent returned %+v", r)
		}
	}

	// The cause is written once, onto a canceled row only.
	if err := s.SetAgentSessionCancelCause(ctx, a.ID, "r", "C"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetAgentSession(ctx, a.ID); got.FailureReason != "" {
		t.Fatalf("a running session took a cancel cause: %q", got.FailureReason)
	}
	if _, _, err := s.SetAgentSessionCanceled(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{"first", "second"} {
		if err := s.SetAgentSessionCancelCause(ctx, a.ID, reason, "C"); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := s.GetAgentSession(ctx, a.ID); got.FailureReason != "first" || got.FailureReasonCode != "C" {
		t.Fatalf("cancel cause = %q / %q, want the first one kept", got.FailureReason, got.FailureReasonCode)
	}
}

// Deleting a parent never deletes its children; they become top-level rows
// that keep their root and depth.
func TestPGAgentSessionDeleteParentKeepsChildren(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "lineage-delete")
	ctx := context.Background()
	root := lineageSession(t, s, tenant.ID, "", nil)
	mid := lineageSession(t, s, tenant.ID, root.ID, nil)
	leaf := lineageSession(t, s, tenant.ID, mid.ID, nil)
	if err := s.DeleteAgentSession(ctx, mid.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetAgentSession(ctx, leaf.ID)
	if err != nil {
		t.Fatalf("the child went with its parent: %v", err)
	}
	if got.ParentSessionID != "" || got.RootSessionID != root.ID || got.Depth != 2 {
		t.Fatalf("orphaned child lineage = %+v", got)
	}
	tree, err := s.AgentSessionSubtree(ctx, tenant.ID, root.ID)
	if err != nil || len(tree) != 2 {
		t.Fatalf("the tree after deleting a middle session = %d / %v, want root and leaf", len(tree), err)
	}
}

// ADR102 D3: the headroom is the tightest ceiling on the chain minus the
// recorded price of that ceiling's whole subtree.
func TestPGAgentSessionCeilingHeadroom(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "lineage-headroom")
	ctx := context.Background()
	record := func(sessionID string, price int64) {
		t.Helper()
		if _, err := s.RecordAgentModelUsage(ctx, AgentModelUsage{
			RequestID: ids.New(ids.AgentModelRequest), WorkspaceID: tenant.ID, SessionID: sessionID,
			Model: "m/a", At: time.Now().UTC(), CostSource: AgentModelCostReported, PriceMicroUSD: price,
		}); err != nil {
			t.Fatal(err)
		}
	}
	headroom := func(id string) (int64, bool) {
		t.Helper()
		h, bounded, err := s.AgentSessionCeilingHeadroom(ctx, tenant.ID, id)
		if err != nil {
			t.Fatal(err)
		}
		return h, bounded
	}
	rootCeiling, childCeiling := int64(10_000_000), int64(3_000_000)
	root := lineageSession(t, s, tenant.ID, "", &rootCeiling)
	child := lineageSession(t, s, tenant.ID, root.ID, &childCeiling)
	free := lineageSession(t, s, tenant.ID, root.ID, nil)
	grand := lineageSession(t, s, tenant.ID, child.ID, nil)
	plain := lineageSession(t, s, tenant.ID, "", nil)

	if _, bounded := headroom(plain.ID); bounded {
		t.Fatal("a session with no ceiling anywhere is bounded")
	}
	for _, c := range []struct {
		id   string
		want int64
	}{{root.ID, 10_000_000}, {child.ID, 3_000_000}, {free.ID, 10_000_000}, {grand.ID, 3_000_000}} {
		if h, bounded := headroom(c.id); !bounded || h != c.want {
			t.Fatalf("fresh headroom of %s = %d (bounded %v), want %d", c.id, h, bounded, c.want)
		}
	}
	// A grandchild's spend counts against its parent's ceiling AND the root's.
	record(grand.ID, 2_000_000)
	// A sibling's spend counts against the root's ceiling only.
	record(free.ID, 7_500_000)
	record(plain.ID, 99_000_000) // another tree: never counted
	for _, c := range []struct {
		id   string
		want int64
	}{
		{root.ID, 10_000_000 - 9_500_000},
		{child.ID, 500_000}, // min(3M-2M, 10M-9.5M)
		{grand.ID, 500_000},
		{free.ID, 500_000},
	} {
		if h, _ := headroom(c.id); h != c.want {
			t.Fatalf("headroom of %s = %d, want %d", c.id, h, c.want)
		}
	}
	record(grand.ID, 600_000)
	if h, _ := headroom(grand.ID); h != -100_000 {
		t.Fatalf("an overspent tree has headroom %d, want -100000 (not clamped)", h)
	}
	if h, bounded, _ := s.AgentSessionCeilingHeadroom(ctx, "tea-nobody", grand.ID); bounded || h != 0 {
		t.Fatalf("another workspace read headroom %d (bounded %v)", h, bounded)
	}
}
