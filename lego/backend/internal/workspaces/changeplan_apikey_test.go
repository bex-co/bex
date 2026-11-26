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
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type subjectEntitlement struct {
	allowedSubjects map[string]bool
	seenSubjects    []string
}

func (e *subjectEntitlement) CheckEnterpriseEntitlement(_ context.Context, subject, _ string) error {
	e.seenSubjects = append(e.seenSubjects, subject)
	if !e.allowedSubjects[subject] {
		return core.NewBadRequestError("ENTERPRISE_ENTITLEMENT_REQUIRED", "operator approval required", nil)
	}
	return nil
}

type failingKeyCreators struct{}

func (failingKeyCreators) APIKeyCreator(context.Context, string) (string, bool, error) {
	return "", false, errors.New("hydra unavailable")
}

// fakeOrgPlanStore wraps fakeStore with the OrganizationStore slice exercised
// by ChangePlan (upgradeToOrganization and downgradePreconditions).
type fakeOrgPlanStore struct {
	*fakeStore
	OrganizationStore
	org           store.Organization
	orgWorkspaces []store.Tenant
	upgradeOwners []string
}

func (f *fakeOrgPlanStore) GetOrganization(_ context.Context, id string) (store.Organization, error) {
	if f.org.ID != id {
		return store.Organization{}, store.ErrNotFound
	}
	return f.org, nil
}

func (f *fakeOrgPlanStore) ListOrganizationWorkspaces(_ context.Context, _ string) ([]store.Tenant, error) {
	return f.orgWorkspaces, nil
}

func (f *fakeOrgPlanStore) ListWorkspaceEnvironments(_ context.Context, _ string) ([]store.Environment, error) {
	return nil, nil
}

func (f *fakeOrgPlanStore) UpgradeToOrganization(_ context.Context, workspaceID, plan string, owners []string) (store.Tenant, store.Organization, []store.OrganizationMember, error) {
	f.upgradeOwners = slices.Clone(owners)
	t, err := f.UpdateTenantPlan(context.Background(), workspaceID, plan)
	if err != nil {
		return store.Tenant{}, store.Organization{}, nil, err
	}
	o := store.Organization{ID: "org-test1", Name: t.Name, Plan: plan, PrimaryWorkspaceID: workspaceID}
	t.OrganizationID = o.ID
	f.tenants[workspaceID] = t
	f.org = o
	f.orgWorkspaces = []store.Tenant{t}
	return t, o, nil, nil
}

func TestChangePlan_APIKeyAttributesEnterpriseEntitlementToCreator(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, err := svc.Create(ctxAs("alice"), "acme", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	ent := &subjectEntitlement{allowedSubjects: map[string]bool{"alice": true}}
	svc.EnterpriseEntitlement = ent
	svc.KeyCreators = coretest.KeyCreators{"key-alice": "alice", "key-bob": "bob"}
	svc.Workspace = coretest.Members{
		"alice":     {w.ID},
		"key-alice": {w.ID},
		"key-bob":   {w.ID},
	}

	// Unentitled creator's key is refused with the same code as a human session.
	_, err = svc.ChangePlan(ctxAsMachine("key-bob"), w.ID, store.PlanEnterprise)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENTERPRISE_ENTITLEMENT_REQUIRED" {
		t.Fatalf("unentitled creator's key: err = %v, want ENTERPRISE_ENTITLEMENT_REQUIRED", err)
	}
	// Entitled creator's key passes entitlement under the creator's subject,
	// not the Hydra client UUID.
	got, err := svc.ChangePlan(ctxAsMachine("key-alice"), w.ID, store.PlanEnterprise)
	if err != nil {
		t.Fatalf("entitled creator's key: %v", err)
	}
	if got.Plan != store.PlanEnterprise {
		t.Fatalf("plan = %q, want %q", got.Plan, store.PlanEnterprise)
	}
	if !slices.Equal(ent.seenSubjects, []string{"bob", "alice"}) {
		t.Fatalf("entitlement subjects = %v, want [bob alice]", ent.seenSubjects)
	}
}

func TestChangePlan_APIKeyEnforcesCreatorsPerUserHobbyCap(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	humanCtx := ctxAs("alice")
	for i := range 5 {
		if _, err := svc.Create(humanCtx, fmt.Sprintf("h%d", i), store.PlanHobby); err != nil {
			t.Fatalf("hobby #%d: %v", i, err)
		}
	}
	sixth, err := svc.Create(humanCtx, "sixth", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	svc.KeyCreators = coretest.KeyCreators{"key-alice": "alice"}
	svc.Workspace = coretest.Members{"alice": {sixth.ID}, "key-alice": {sixth.ID}}

	if _, err := svc.ChangePlan(ctxAsMachine("key-alice"), sixth.ID, store.PlanHobby); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("6th hobby downgrade via API key: want ErrBadRequest, got %v", err)
	}
	if row, _ := st.GetTenant(context.Background(), sixth.ID); row.Plan != store.PlanPro {
		t.Fatalf("refused downgrade changed plan to %q", row.Plan)
	}
}

func TestChangePlan_APIKeyAttributesUpgradeAndDowngradePreconditionsToCreator(t *testing.T) {
	baseStore := newFakeStore()
	st := &fakeOrgPlanStore{fakeStore: baseStore}
	svc := allowSvc(st, &fakeGranter{}, nil, nil)

	w, err := svc.Create(ctxAs("alice"), "acme", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	// bob joins after alice as a second admin; carol is a developer.
	st.members[w.ID] = append(st.members[w.ID],
		store.TenantMember{TenantID: w.ID, Subject: "bob", Role: "admin"},
		store.TenantMember{TenantID: w.ID, Subject: "carol", Role: "developer"},
	)
	svc.KeyCreators = coretest.KeyCreators{"key-bob": "bob"}
	svc.Workspace = coretest.Members{"alice": {w.ID}, "bob": {w.ID}, "key-bob": {w.ID}}

	// Upgrading via bob's API key passes bob (not key-bob) as the sole Owner.
	if _, err := svc.ChangePlan(ctxAsMachine("key-bob"), w.ID, store.PlanScale); err != nil {
		t.Fatalf("upgrade to scale via API key: %v", err)
	}
	if !slices.Equal(st.upgradeOwners, []string{"bob"}) {
		t.Fatalf("UpgradeToOrganization owners = %v, want [bob]", st.upgradeOwners)
	}

	// Downgrading the Scale org to Hobby via bob's API key keeps bob seated
	// and lists alice + carol as unseated — matching bob's human session.
	wantErr := func(ctx context.Context) []string {
		t.Helper()
		_, err := svc.ChangePlan(ctx, w.ID, store.PlanHobby)
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != core.CodeOrganizationDowngradeBlocked {
			t.Fatalf("downgrade err = %v, want %s", err, core.CodeOrganizationDowngradeBlocked)
		}
		pres, _ := coded.Params["preconditions"].([]map[string]any)
		for _, p := range pres {
			if p["kind"] == core.PreconditionMemberLimit {
				items, _ := p["items"].([]string)
				return items
			}
		}
		t.Fatalf("missing %s precondition in %+v", core.PreconditionMemberLimit, pres)
		return nil
	}
	humanItems := wantErr(ctxAs("bob"))
	keyItems := wantErr(ctxAsMachine("key-bob"))
	want := []string{"alice:admin", "carol:developer"}
	if !slices.Equal(humanItems, want) || !slices.Equal(keyItems, want) {
		t.Fatalf("member_limit items: human=%v key=%v want=%v", humanItems, keyItems, want)
	}
}

func TestChangePlan_APIKeyAttributionFailsClosedAndKeepsNonKeyClient(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, err := svc.Create(ctxAs("alice"), "acme", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	svc.Workspace = coretest.Members{
		"key-legacy":      {w.ID},
		"key-broken":      {w.ID},
		"operator-client": {w.ID},
	}

	// Key registry unavailable -> ErrAuthzUnavailable, no plan change.
	svc.KeyCreators = failingKeyCreators{}
	if _, err := svc.ChangePlan(ctxAsMachine("key-broken"), w.ID, store.PlanHobby); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("unavailable key registry: err = %v, want ErrAuthzUnavailable", err)
	}
	if row, _ := st.GetTenant(context.Background(), w.ID); row.Plan != store.PlanPro {
		t.Fatalf("unavailable registry changed plan to %q", row.Plan)
	}

	// Unattributed bex API key (isKey=true, creator="") -> ErrForbidden, no plan change.
	svc.KeyCreators = coretest.KeyCreators{"key-legacy": ""}
	if _, err := svc.ChangePlan(ctxAsMachine("key-legacy"), w.ID, store.PlanHobby); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("unattributed API key: err = %v, want ErrForbidden", err)
	}
	if row, _ := st.GetTenant(context.Background(), w.ID); row.Plan != store.PlanPro {
		t.Fatalf("unattributed key changed plan to %q", row.Plan)
	}

	// Operator-provisioned machine client (not in KeyCreators => isKey=false)
	// keeps its own id.Subject.
	ent := &subjectEntitlement{allowedSubjects: map[string]bool{"operator-client": true}}
	svc.EnterpriseEntitlement = ent
	if _, err := svc.ChangePlan(ctxAsMachine("operator-client"), w.ID, store.PlanEnterprise); err != nil {
		t.Fatalf("operator client: %v", err)
	}
	if !slices.Equal(ent.seenSubjects, []string{"operator-client"}) {
		t.Fatalf("operator client entitlement subject = %v, want [operator-client]", ent.seenSubjects)
	}
}
