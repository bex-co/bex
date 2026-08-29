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

package projects

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// moves_test.go covers w6/m134's emission half in this funnel: a successful
// SetServices records exactly one service_moved audit row per service whose
// placement actually changed — and none for no-ops or failed writes. The
// audit row IS the service event (internal/events maps projects.MoveService
// onto service_moved), so the sink assertions here are feed assertions.

type recordingSink struct{ events []core.AuditEvent }

func (r *recordingSink) Record(_ context.Context, ev core.AuditEvent) error {
	r.events = append(r.events, ev)
	return nil
}

func (r *recordingSink) moveRows() []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range r.events {
		if ev.Verb == core.AuditVerbProjectServiceMoved {
			out = append(out, ev)
		}
	}
	return out
}

// newMoveService is the projects sibling of environments' newServiceWithClient:
// the fake project store plus a fake k8s client seeded with App CRs, so
// RecordServiceMoves can resolve each moved service's canonical audit target.
func newMoveService(st ProjectStore, sink *recordingSink, appNames ...string) *Service {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, name := range appNames {
		builder = builder.WithObjects(&appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}})
	}
	return &Service{
		Base:  &core.Base{Authz: allowChecker{}, Client: builder.Build(), Namespace: "default", Audit: sink},
		Store: st,
	}
}

func TestSetServices_RecordsOneMoveEventPerChangedService(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	sink := &recordingSink{}
	svc := newMoveService(st, sink, "web", "worker")

	if _, err := svc.SetServices(ctxAs("user-a"), "prj-1", []string{"web"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	moves := sink.moveRows()
	if len(moves) != 1 {
		t.Fatalf("assign recorded %d move rows, want exactly 1: %+v", len(moves), moves)
	}
	mv := moves[0]
	if mv.Target != core.ServiceTarget("web") || mv.Outcome != core.AuditAllowed {
		t.Errorf("move row target/outcome = %q/%q, want service:web allowed", mv.Target, mv.Outcome)
	}
	if mv.ProjectFrom != nil || mv.ProjectTo == nil || *mv.ProjectTo != "prj-1" {
		t.Errorf("assign placement = %+v, want project nil→prj-1", mv)
	}

	// Replaying the same membership is a no-op: no false move event.
	sink.events = nil
	if _, err := svc.SetServices(ctxAs("user-a"), "prj-1", []string{"web"}); err != nil {
		t.Fatalf("no-op SetServices: %v", err)
	}
	if moves := sink.moveRows(); len(moves) != 0 {
		t.Fatalf("no-op replacement recorded %d move rows, want none: %+v", len(moves), moves)
	}

	// Unassigning records the departure with the from side populated.
	sink.events = nil
	if _, err := svc.SetServices(ctxAs("user-a"), "prj-1", nil); err != nil {
		t.Fatalf("unassign SetServices: %v", err)
	}
	moves = sink.moveRows()
	if len(moves) != 1 {
		t.Fatalf("unassign recorded %d move rows, want exactly 1: %+v", len(moves), moves)
	}
	if mv := moves[0]; mv.ProjectFrom == nil || *mv.ProjectFrom != "prj-1" || mv.ProjectTo != nil {
		t.Errorf("unassign placement = %+v, want project prj-1→nil", mv)
	}
}

// failingProjectMembershipStore fails the authoritative membership write — the
// case that must leave the feed untouched.
type failingProjectMembershipStore struct{ ProjectStore }

func (failingProjectMembershipStore) SetProjectServices(context.Context, string, string, []string) ([]core.ServicePlacementChange, error) {
	return nil, store.ErrConflict
}

func TestSetServices_FailedWriteRecordsNoMoveEvent(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	sink := &recordingSink{}
	svc := newMoveService(failingProjectMembershipStore{st}, sink, "web")

	if _, err := svc.SetServices(ctxAs("user-a"), "prj-1", []string{"web"}); err == nil {
		t.Fatal("SetServices over a failing store should error")
	}
	if moves := sink.moveRows(); len(moves) != 0 {
		t.Fatalf("failed write recorded %d move rows, want none: %+v", len(moves), moves)
	}
}
