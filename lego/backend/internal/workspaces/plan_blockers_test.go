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

package workspaces

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// plan_blockers_test.go pins the plan page's blockers (w3/m708) to
// ChangePlan's own refusals on a standalone workspace: each scenario lists
// what ChangePlan refuses, and a workspace with no blocker changes plan.
// TestOrganizationDowngradePreconditionsE2E and
// TestOrganizationDowngradeNamesPeopleE2E compare them on an org's primary
// through GraphQL on a real Postgres.

func TestPlanChangeBlockersMatchChangePlansRefusals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		from  string
		to    string
		setup func(st *fakeStore, svc *Service, id string)
		want  []core.DowngradePrecondition
	}{
		{name: "upgrade", from: store.PlanHobby, to: store.PlanPro, want: []core.DowngradePrecondition{}},
		{name: "same plan", from: store.PlanPro, to: store.PlanPro, want: []core.DowngradePrecondition{}},
		{name: "downgrade that fits", from: store.PlanPro, to: store.PlanHobby, want: []core.DowngradePrecondition{}},
		{
			// Hobby seats one: everyone beyond the caller is named, members
			// first, with their roles; the Developer's role is not a second
			// blocker, since clearing the seat clears it.
			name: "people over Hobby's seat", from: store.PlanPro, to: store.PlanHobby,
			setup: func(st *fakeStore, _ *Service, id string) {
				st.members[id] = append(st.members[id], store.TenantMember{TenantID: id, Subject: "user-b", Role: "developer"})
				st.invite(id, "carol@example.com", "developer")
			},
			want: []core.DowngradePrecondition{{Kind: core.PreconditionMemberLimit, Items: []string{"user-b:developer", "carol@example.com:developer"}, Limit: 1}},
		},
		{
			name: "only an invite over the seat", from: store.PlanPro, to: store.PlanHobby,
			setup: func(st *fakeStore, _ *Service, id string) { st.invite(id, "carol@example.com", "developer") },
			want:  []core.DowngradePrecondition{{Kind: core.PreconditionMemberLimit, Items: []string{"carol@example.com:developer"}, Limit: 1}},
		},
		{
			name: "services over Hobby's cap", from: store.PlanPro, to: store.PlanHobby,
			setup: func(st *fakeStore, _ *Service, id string) { st.apps = map[string]int{id: 26} },
			want:  []core.DowngradePrecondition{{Kind: core.PreconditionServiceLimit, Items: []string{}, Limit: 25}},
		},
		{
			name: "at the Hobby services cap", from: store.PlanPro, to: store.PlanHobby,
			setup: func(st *fakeStore, _ *Service, id string) { st.apps = map[string]int{id: 25} },
			want:  []core.DowngradePrecondition{},
		},
		{
			name: "five Hobby workspaces already", from: store.PlanPro, to: store.PlanHobby,
			setup: func(_ *fakeStore, svc *Service, _ string) {
				for i := range 5 {
					if _, err := svc.Create(ctxAs("user-a"), fmt.Sprintf("h%d", i), store.PlanHobby); err != nil {
						t.Fatal(err)
					}
				}
			},
			want: []core.DowngradePrecondition{{Kind: core.PreconditionWorkspaceLimit, Items: []string{}, Limit: 5}},
		},
		{
			name: "roles Pro does not offer", from: store.PlanScale, to: store.PlanPro,
			setup: func(st *fakeStore, _ *Service, id string) {
				st.members[id] = append(st.members[id], store.TenantMember{TenantID: id, Subject: "user-v", Role: "viewer"})
				st.invite(id, "bill@example.com", "billing")
				st.invite(id, "dev@example.com", "developer")
			},
			want: []core.DowngradePrecondition{
				{Kind: core.PreconditionMemberRoles, Items: []string{"user-v:viewer"}},
				{Kind: core.PreconditionInviteRoles, Items: []string{"bill@example.com:billing"}},
			},
		},
		{
			name: "every kind at once", from: store.PlanPro, to: store.PlanHobby,
			setup: func(st *fakeStore, svc *Service, id string) {
				st.invite(id, "carol@example.com", "developer")
				st.apps = map[string]int{id: 30}
				for i := range 5 {
					if _, err := svc.Create(ctxAs("user-a"), fmt.Sprintf("h%d", i), store.PlanHobby); err != nil {
						t.Fatal(err)
					}
				}
			},
			want: []core.DowngradePrecondition{
				{Kind: core.PreconditionMemberLimit, Items: []string{"carol@example.com:developer"}, Limit: 1},
				{Kind: core.PreconditionServiceLimit, Items: []string{}, Limit: 25},
				{Kind: core.PreconditionWorkspaceLimit, Items: []string{}, Limit: 5},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			svc := allowSvc(st, &fakeGranter{}, nil, nil)
			w, err := svc.Create(ctxAs("user-a"), "acme", tc.from)
			if err != nil {
				t.Fatal(err)
			}
			if tc.setup != nil {
				tc.setup(st, svc, w.ID)
			}
			got, err := svc.PlanChangeBlockers(ctxAs("user-a"), w.ID, tc.to)
			if err != nil {
				t.Fatalf("PlanChangeBlockers: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("blockers = %+v, want %+v", got, tc.want)
			}
			// The page's promise: a blocker it lists is a refusal, and no
			// blocker means the change goes through.
			_, err = svc.ChangePlan(ctxAs("user-a"), w.ID, tc.to)
			if len(got) > 0 && !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("ChangePlan with blockers %+v = %v, want a refusal", got, err)
			}
			if len(got) == 0 && err != nil {
				t.Fatalf("ChangePlan with no blocker = %v", err)
			}
		})
	}
}

func TestPlanChangeBlockersRefusals(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", store.PlanPro)
	if _, err := svc.PlanChangeBlockers(ctxAs("user-a"), w.ID, "platinum"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("unknown plan: want ErrBadRequest, got %v", err)
	}
	denied := &Service{Base: &core.Base{Authz: &fakeChecker{allow: false}}, Store: st}
	if _, err := denied.PlanChangeBlockers(ctxAs("user-b"), w.ID, store.PlanHobby); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-admin: want forbidden, got %v", err)
	}
	if _, err := (&Service{Base: svc.Base}).PlanChangeBlockers(ctxAs("user-a"), w.ID, store.PlanHobby); !errors.Is(err, core.ErrWorkspacesUnavailable) {
		t.Fatalf("no store: want ErrWorkspacesUnavailable, got %v", err)
	}
}
