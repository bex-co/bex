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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// countingEnvGroupIndex counts the workspace-wide membership reads the full
// environment view makes.
type countingEnvGroupIndex struct {
	fakeEnvGroupIndex
	lists int
}

func (c *countingEnvGroupIndex) ListEnvironmentMemberships(ctx context.Context, ownerID string) ([]envgroups.EnvironmentMembership, error) {
	c.lists++
	return c.fakeEnvGroupIndex.ListEnvironmentMemberships(ctx, ownerID)
}

// w4/m206: placing an env group in an environment asked for the environment's
// owner through Get, which lists every database, Key Value and env group of
// the workspace to build a view the caller threw away — on every create and
// every single move. OwnerWorkspace answers the same question with the same
// scoping and authorization, and reads no membership.
func TestOwnerWorkspaceAnswersGetsOwnerWithoutReadingMembership(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	groups := &countingEnvGroupIndex{}
	svc := newService(st)
	svc.EnvGroups = groups
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatal(err)
	}
	groups.lists = 0

	owner, err := svc.OwnerWorkspace(ctxAs("user-a"), e.ID)
	if err != nil || owner != "tea-a" {
		t.Fatalf("OwnerWorkspace = %q, %v; want tea-a", owner, err)
	}
	if groups.lists != 0 {
		t.Fatalf("OwnerWorkspace read workspace membership %d time(s), want none", groups.lists)
	}
	view, err := svc.Get(ctxAs("user-a"), e.ID)
	if err != nil || view.OwnerID != owner {
		t.Fatalf("Get owner = %q, %v; want the same %q", view.OwnerID, err, owner)
	}
	if groups.lists == 0 {
		t.Fatal("Get stopped reading membership: its public view must stay complete")
	}
}

// The narrow lookup refuses exactly what Get refuses: another workspace's
// environment is forbidden (never "not found"), and a missing one is not found.
func TestOwnerWorkspaceRefusesWhatGetRefuses(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-other", TenantID: "tea-other", Name: "other"})
	st.envs["env-other"] = store.Environment{ID: "env-other", ProjectID: "prj-other", TenantID: "tea-other", Name: "production"}
	foreign := &Service{Base: &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-other"))}, Store: st}
	_, getErr := foreign.Get(ctxAs("user-a"), "env-other")
	_, err := foreign.OwnerWorkspace(ctxAs("user-a"), "env-other")
	if !errors.Is(err, core.ErrForbidden) || errors.Is(err, core.ErrNotFound) || !errors.Is(getErr, core.ErrForbidden) {
		t.Fatalf("foreign environment: OwnerWorkspace = %v, Get = %v; want both forbidden", err, getErr)
	}

	if _, err := newService(newFakeStore()).OwnerWorkspace(ctxAs("user-a"), "env-missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("missing environment: OwnerWorkspace = %v, want not found", err)
	}
	if _, err := (&Service{Base: &core.Base{Authz: allowChecker{}}}).OwnerWorkspace(ctxAs("user-a"), "env-any"); !errors.Is(err, ErrEnvironmentsUnavailable) {
		t.Fatalf("no store: OwnerWorkspace = %v, want environments unavailable", err)
	}
}

// failingEnvGroupIndex refuses to move one named group.
type failingEnvGroupIndex struct {
	fakeEnvGroupIndex
	refuse string
}

func (f *failingEnvGroupIndex) SetEnvironmentID(ctx context.Context, id, environmentID string) error {
	if id == f.refuse {
		return errors.New("secret store: connection reset by 10.0.0.5")
	}
	return f.fakeEnvGroupIndex.SetEnvironmentID(ctx, id, environmentID)
}

// w4/m206: a two-group move that failed on its second group left the first one
// moved and answered with the bare failure, so a caller could not tell that
// placement had changed. The refusal now names what changed and what failed;
// a failure before any change still stands alone.
func TestSetEnvGroupsReportsHowFarAnInterruptedReplacementGot(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	groups := &failingEnvGroupIndex{refuse: "evg-second"}
	groups.groups = []envgroups.EnvGroupView{
		{ID: "evg-first", OwnerID: "tea-a"},
		{ID: "evg-second", OwnerID: "tea-a"},
	}
	svc := newService(st)
	svc.EnvGroups = groups
	left, _ := svc.Create(ctxAs("user-a"), "prj-1", "left")
	right, _ := svc.Create(ctxAs("user-a"), "prj-1", "right")
	groups.refuse = ""
	if _, err := svc.SetEnvGroups(ctxAs("user-a"), left.ID, []string{"evg-first", "evg-second"}); err != nil {
		t.Fatal(err)
	}

	groups.refuse = "evg-second"
	_, err := svc.SetEnvGroups(ctxAs("user-a"), right.ID, []string{"evg-first", "evg-second"})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_MEMBERSHIP_PARTIAL" || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("interrupted move = %v, want the partial-membership conflict", err)
	}
	if changed, _ := coded.Params["changedIds"].([]string); len(changed) != 1 || changed[0] != "evg-first" || coded.Params["failedId"] != "evg-second" {
		t.Fatalf("partial outcome = %v, want evg-first changed and evg-second failed", coded.Params)
	}
	if strings.Contains(err.Error(), "10.0.0.5") {
		t.Fatalf("the refusal leaks its internal cause: %v", err)
	}
	// The outcome it names is the one a read finds.
	leftNow, _ := svc.Get(ctxAs("user-a"), left.ID)
	rightNow, _ := svc.Get(ctxAs("user-a"), right.ID)
	if len(leftNow.EnvGroupIDs) != 1 || leftNow.EnvGroupIDs[0] != "evg-second" || len(rightNow.EnvGroupIDs) != 1 || rightNow.EnvGroupIDs[0] != "evg-first" {
		t.Fatalf("placement = left %v right %v, want the split the refusal described", leftNow.EnvGroupIDs, rightNow.EnvGroupIDs)
	}
	// Retrying once the failure clears completes the move.
	groups.refuse = ""
	done, err := svc.SetEnvGroups(ctxAs("user-a"), right.ID, []string{"evg-first", "evg-second"})
	if err != nil || len(done.EnvGroupIDs) != 2 {
		t.Fatalf("retry = %v, %v; want both groups in the target", done.EnvGroupIDs, err)
	}

	// A failure on the first change is not partial: its own error stands.
	groups.refuse = "evg-first"
	if _, err := svc.SetEnvGroups(ctxAs("user-a"), left.ID, []string{"evg-first"}); err == nil || errors.As(err, &coded) && coded.Code == "ENVIRONMENT_MEMBERSHIP_PARTIAL" {
		t.Fatalf("first-change failure = %v, want the plain failure", err)
	}
}
