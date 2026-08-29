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

package environments

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// moves_test.go covers w6/m134's emission half in this funnel: a successful
// SetServices records exactly one service_moved audit row per service whose
// placement actually changed — and records none for no-ops or failed writes.
// The audit row IS the service event (internal/events maps
// environments.MoveService onto service_moved), so the sink assertions here
// are feed assertions.

type recordingSink struct{ events []core.AuditEvent }

func (r *recordingSink) Record(_ context.Context, ev core.AuditEvent) error {
	r.events = append(r.events, ev)
	return nil
}

func (r *recordingSink) moveRows(verb string) []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range r.events {
		if ev.Verb == verb {
			out = append(out, ev)
		}
	}
	return out
}

func TestSetServices_RecordsOneMoveEventPerChangedService(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st, sampleApp("web"), sampleApp("worker"))
	sink := &recordingSink{}
	svc.Audit = sink

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	moves := sink.moveRows(core.AuditVerbEnvironmentServiceMoved)
	if len(moves) != 1 {
		t.Fatalf("assign recorded %d move rows, want exactly 1: %+v", len(moves), moves)
	}
	mv := moves[0]
	if mv.Target != core.ServiceTarget("web") || mv.Outcome != core.AuditAllowed {
		t.Errorf("move row target/outcome = %q/%q, want service:web allowed", mv.Target, mv.Outcome)
	}
	if mv.ProjectFrom != nil || mv.EnvironmentFrom != nil ||
		mv.ProjectTo == nil || *mv.ProjectTo != "prj-1" ||
		mv.EnvironmentTo == nil || *mv.EnvironmentTo != e.ID {
		t.Errorf("assign placement = %+v, want nil→(prj-1,%s)", mv, e.ID)
	}

	// Replaying the same membership is a no-op: no false move event.
	sink.events = nil
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"}); err != nil {
		t.Fatalf("no-op SetServices: %v", err)
	}
	if moves := sink.moveRows(core.AuditVerbEnvironmentServiceMoved); len(moves) != 0 {
		t.Fatalf("no-op replacement recorded %d move rows, want none: %+v", len(moves), moves)
	}

	// Unassigning records the departure with the from side populated.
	sink.events = nil
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, nil); err != nil {
		t.Fatalf("unassign SetServices: %v", err)
	}
	moves = sink.moveRows(core.AuditVerbEnvironmentServiceMoved)
	if len(moves) != 1 {
		t.Fatalf("unassign recorded %d move rows, want exactly 1: %+v", len(moves), moves)
	}
	mv = moves[0]
	if mv.EnvironmentFrom == nil || *mv.EnvironmentFrom != e.ID || mv.EnvironmentTo != nil {
		t.Errorf("unassign placement = %+v, want environment %s→nil", mv, e.ID)
	}
}

// failingMembershipStore fails the authoritative membership write itself — the
// case that must leave the feed untouched.
type failingMembershipStore struct{ EnvironmentStore }

func (failingMembershipStore) SetEnvironmentServices(context.Context, string, string, string, []string) ([]core.ServicePlacementChange, error) {
	return nil, errors.New("boom")
}

func TestSetServices_FailedWriteRecordsNoMoveEvent(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st, sampleApp("web"))
	sink := &recordingSink{}
	svc.Audit = sink

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	svc.Store = failingMembershipStore{st}
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"}); err == nil {
		t.Fatal("SetServices over a failing store should error")
	}
	if moves := sink.moveRows(core.AuditVerbEnvironmentServiceMoved); len(moves) != 0 {
		t.Fatalf("failed write recorded %d move rows, want none: %+v", len(moves), moves)
	}
}

// A reported change whose App CR does not exist (a stale store row, or a race
// with delete) is skipped rather than failing the verb or inventing a target —
// mirroring patchApps' own stale-name tolerance.
func TestSetServices_MissingAppCRSkipsRecordingWithoutFailing(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st) // no App CRs at all
	sink := &recordingSink{}
	svc.Audit = sink

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"ghost"}); err != nil {
		t.Fatalf("SetServices with a CR-less member: %v", err)
	}
	if moves := sink.moveRows(core.AuditVerbEnvironmentServiceMoved); len(moves) != 0 {
		t.Fatalf("CR-less member recorded %d move rows, want none: %+v", len(moves), moves)
	}
}
