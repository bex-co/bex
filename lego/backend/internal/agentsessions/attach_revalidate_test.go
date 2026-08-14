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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// relationChecker allows exactly the relations in allow, and (when freshDeny
// is set) denies everything on the FRESH path while still allowing on the
// cached path — modelling a just-revoked member whose positive decision is
// still warm in the cache.
type relationChecker struct {
	allow     map[string]bool
	freshDeny bool
}

func (c relationChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return c.allow[relation], nil
}

func (c relationChecker) CheckFresh(_ context.Context, _, relation, _ string) (bool, error) {
	if c.freshDeny {
		return false, nil
	}
	return c.allow[relation], nil
}

func revalidator(c core.Checker, st *fakeStore) *AttachRevalidator {
	return &AttachRevalidator{Base: &core.Base{Authz: c}, Store: st}
}

// TestAttachRevalidatorUsesFreshAuthorization pins codex-security round-6 #11:
// redemption is a privilege exercise, so it must consult the source of truth —
// a revoked member whose cached positive is still warm is refused.
func TestAttachRevalidatorUsesFreshAuthorization(t *testing.T) {
	st := newFakeStore()
	id := liveSession(st, "tea-a", "os-abc", PhaseRunning)
	r := revalidator(relationChecker{allow: map[string]bool{core.RelCanOperate: true}, freshDeny: true}, st)
	if err := r.RevalidateAttach(context.Background(), "alice", id, agentsessionticket.ActionRead); err == nil {
		t.Fatal("revoked-on-fresh-path subject must be refused at redemption")
	}
}

// TestAttachRevalidatorMirrorsMintRelations pins the relation ladder: read
// redeems with can_operate (contributor), turn requires can_create (developer)
// — the same relations AttachTicket mints under.
func TestAttachRevalidatorMirrorsMintRelations(t *testing.T) {
	st := newFakeStore()
	id := liveSession(st, "tea-a", "os-abc", PhaseRunning)

	contributor := revalidator(relationChecker{allow: map[string]bool{core.RelCanOperate: true}}, st)
	if err := contributor.RevalidateAttach(context.Background(), "alice", id, agentsessionticket.ActionRead); err != nil {
		t.Fatalf("contributor read redemption: %v", err)
	}
	if err := contributor.RevalidateAttach(context.Background(), "alice", id, agentsessionticket.ActionTurn); err == nil {
		t.Fatal("contributor must not redeem a turn ticket (can_create required)")
	}

	developer := revalidator(relationChecker{allow: map[string]bool{core.RelCanCreate: true}}, st)
	if err := developer.RevalidateAttach(context.Background(), "alice", id, agentsessionticket.ActionTurn); err != nil {
		t.Fatalf("developer turn redemption: %v", err)
	}
}

// TestAttachRevalidatorTurnRequiresLivePhase pins the lifecycle re-check: a
// turn ticket minted moments before the session went terminal must not run a
// last off-lifecycle model turn; a read ticket replays terminal sessions by
// design.
func TestAttachRevalidatorTurnRequiresLivePhase(t *testing.T) {
	st := newFakeStore()
	id := liveSession(st, "tea-a", "os-abc", PhaseCompleted)
	r := revalidator(relationChecker{allow: map[string]bool{core.RelCanOperate: true, core.RelCanCreate: true}}, st)
	if err := r.RevalidateAttach(context.Background(), "alice", id, agentsessionticket.ActionTurn); err == nil {
		t.Fatal("turn redemption on a terminal session must be refused")
	}
	if err := r.RevalidateAttach(context.Background(), "alice", id, agentsessionticket.ActionRead); err != nil {
		t.Fatalf("read redemption on a terminal session must stay allowed: %v", err)
	}
}
