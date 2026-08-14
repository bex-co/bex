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

package apps

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestApplyCreateRollsBackStoreRowWhenCRCreateFails is the stack-path twin of
// TestCreateRollsBackStoreRowWhenCRCreateFails: a Blueprint apply whose CR
// write fails after the store row landed must delete that row again, or the
// projector can resurrect a service the API reported as failed.
func TestApplyCreateRollsBackStoreRowWhenCRCreateFails(t *testing.T) {
	rec := &recordingStore{}
	wantErr := errors.New("CRD rejected App")
	svc := &Service{
		Base: &core.Base{
			Client:    createErrorClient{Client: fakeClient(), err: wantErr},
			Namespace: "default",
			Workspace: fakeWorkspace{"id-a": "tea-a"},
		},
		Store: rec,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

	if _, err := svc.applyCreate(ctx, CreateRequest{Name: "web", Image: "nginx:1"}); !errors.Is(err, wantErr) {
		t.Fatalf("applyCreate error = %v, want wrapped CR create error", err)
	}
	if len(rec.appCreates) != 1 {
		t.Fatalf("store creates = %d, want 1", len(rec.appCreates))
	}
	if len(rec.deleteCalls) != 1 || rec.deleteCalls[0] != "srv-test" {
		t.Fatalf("store rollback deletes = %v, want [srv-test]", rec.deleteCalls)
	}
}

// environmentAssignStore backs the stack path's environment-reassignment
// branch: it resolves one known environment and records SetAppEnvironment
// write-through calls.
type environmentAssignStore struct {
	*recordingStore
	assignment  core.EnvironmentAssignment
	assignCalls []struct{ appID, projectID, environmentID string }
}

func (s *environmentAssignStore) ResolveForCreate(_ context.Context, environmentID, workspaceID string) (core.EnvironmentAssignment, error) {
	if environmentID != s.assignment.ID || workspaceID != s.assignment.WorkspaceID {
		return core.EnvironmentAssignment{}, core.ErrNotFound
	}
	return s.assignment, nil
}

func (s *environmentAssignStore) SetAppEnvironment(_ context.Context, appID, projectID, environmentID string) error {
	s.assignCalls = append(s.assignCalls, struct{ appID, projectID, environmentID string }{appID, projectID, environmentID})
	return nil
}

// TestApplyCreateMovesExistingServiceBetweenEnvironments covers the
// EnvironmentSpecified update branch of applyCreateWithFields: joining an
// environment stamps the project/environment/isolation labels, inherits the
// environment's inbound-IP layer, and writes the assignment through to the
// store; an unchanged re-apply is a no-op; leaving sheds all of it.
func TestApplyCreateMovesExistingServiceBetweenEnvironments(t *testing.T) {
	st := &environmentAssignStore{
		recordingStore: &recordingStore{},
		assignment: core.EnvironmentAssignment{
			ID:                      "evm-1",
			ProjectID:               "prj-1",
			WorkspaceID:             "tea-a",
			NetworkIsolationEnabled: true,
			IPAllowList:             []core.IPAllowListEntry{{CIDRBlock: "10.1.0.0/16"}},
		},
	}
	svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
	svc.Environments = st
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

	req := CreateRequest{Name: "web", Image: "nginx:1"}
	if _, err := svc.applyCreate(ctx, req); err != nil {
		t.Fatalf("initial applyCreate: %v", err)
	}

	join := req
	join.EnvironmentSpecified = true
	join.EnvironmentID = "evm-1"
	if _, err := svc.applyCreate(ctx, join); err != nil {
		t.Fatalf("join environment: %v", err)
	}
	a := getTenantApp(t, cl, "tea-a", "web")
	if a.Labels[core.LabelProject] != "prj-1" || a.Labels[core.LabelEnvironment] != "evm-1" {
		t.Fatalf("environment labels = %v, want prj-1/evm-1", a.Labels)
	}
	if a.Labels[core.LabelNetworkIsolation] != "evm-1" {
		t.Fatalf("isolation label = %q, want evm-1", a.Labels[core.LabelNetworkIsolation])
	}
	if !slices.Equal(a.Spec.EnvironmentIPAllowList, []string{"10.1.0.0/16"}) {
		t.Fatalf("environment IP layer = %v, want [10.1.0.0/16]", a.Spec.EnvironmentIPAllowList)
	}
	if len(st.assignCalls) != 1 || st.assignCalls[0] != (struct{ appID, projectID, environmentID string }{"srv-test", "prj-1", "evm-1"}) {
		t.Fatalf("assignment write-through = %v, want one srv-test/prj-1/evm-1 call", st.assignCalls)
	}

	// Re-applying the identical assignment must short-circuit before any write.
	if _, err := svc.applyCreate(ctx, join); err != nil {
		t.Fatalf("idempotent re-apply: %v", err)
	}
	if len(st.assignCalls) != 1 {
		t.Fatalf("idempotent re-apply wrote the assignment again: %v", st.assignCalls)
	}

	leave := req
	leave.EnvironmentSpecified = true
	if _, err := svc.applyCreate(ctx, leave); err != nil {
		t.Fatalf("leave environment: %v", err)
	}
	a = getTenantApp(t, cl, "tea-a", "web")
	for _, label := range []string{core.LabelProject, core.LabelEnvironment, core.LabelNetworkIsolation} {
		if a.Labels[label] != "" {
			t.Errorf("label %s survived leaving the environment: %v", label, a.Labels)
		}
	}
	if a.Spec.EnvironmentIPAllowList != nil {
		t.Errorf("environment IP layer survived leaving: %v", a.Spec.EnvironmentIPAllowList)
	}
	if len(st.assignCalls) != 2 || st.assignCalls[1] != (struct{ appID, projectID, environmentID string }{"srv-test", "", ""}) {
		t.Fatalf("leave write-through = %v, want a trailing srv-test/empty/empty call", st.assignCalls)
	}
	// Environment moves alone must never open deploy records.
	if len(st.deployCalls) != 0 {
		t.Errorf("environment moves opened %d deploy records", len(st.deployCalls))
	}
}

// Compile-time guard: the fake must satisfy the intent store so it can back
// the whole applyCreate flow, not just the assignment write-through.
var _ IntentStore = (*environmentAssignStore)(nil)
