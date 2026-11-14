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
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// twofactor_test.go covers the setting half of w22/m402 (docs/ADR012-auth.md
// §9a): an admin turns "require two-factor authentication" on and off, a
// non-admin cannot, a no-op writes no audit row, enabling is refused while the
// acting admin has no second factor (or no lookup exists to enforce it), and a
// blocked member can still list their workspaces and see the setting.

// roleChecker grants each subject the relations of its role, like OpenFGA's
// workspace model does for the ones this feature reads.
type roleChecker map[string]string

var roleRelations = map[string][]string{
	"admin":     {core.RelCanView, core.RelCanManage, core.RelCanCreate},
	"developer": {core.RelCanView, core.RelCanCreate},
	"viewer":    {core.RelCanView},
}

func (c roleChecker) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	user := subject[len("user:"):]
	for _, r := range roleRelations[c[user]] {
		if r == relation {
			return true, nil
		}
	}
	return false, nil
}

type auditCapture struct {
	mu     sync.Mutex
	events []core.AuditEvent
}

func (a *auditCapture) Record(_ context.Context, ev core.AuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, ev)
	return nil
}

func (a *auditCapture) verbs(outcome core.AuditOutcome) []core.AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []core.AuditEvent
	for _, e := range a.events {
		if e.Verb == core.AuditVerbSetRequireTwoFactor && e.Outcome == outcome {
			out = append(out, e)
		}
	}
	return out
}

// twoFactorFixture is a service over a fake store with an enrolled admin
// (admin-1), an unenrolled admin (admin-2), a developer and a viewer, all
// members of one workspace.
type twoFactorFixture struct {
	svc        *Service
	store      *fakeStore
	audit      *auditCapture
	identities fakeIdentities
	changed    []string
	workspace  string
}

func newTwoFactorFixture(t *testing.T) *twoFactorFixture {
	t.Helper()
	f := &twoFactorFixture{
		store: newFakeStore(),
		audit: &auditCapture{},
		identities: fakeIdentities{
			"admin-1": {MFAEnabled: true}, "admin-2": {}, "dev-1": {MFAEnabled: true}, "viewer-1": {MFAEnabled: true},
		},
	}
	f.svc = &Service{
		Base: &core.Base{
			Authz:     roleChecker{"admin-1": "admin", "admin-2": "admin", "dev-1": "developer", "viewer-1": "viewer"},
			Workspace: &fakeResolver{store: f.store},
			Audit:     f.audit,
		},
		Store:            f.store,
		Identities:       f.identities,
		TwoFactorChanged: func(id string) { f.changed = append(f.changed, id) },
	}
	w, err := f.store.CreateWorkspace(context.Background(), "acme", "pro", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	f.workspace = w.ID
	return f
}

func isCoded(err error, sentinel error, code string) bool {
	var coded *core.CodedError
	return errors.Is(err, sentinel) && errors.As(err, &coded) && coded.Code == code
}

func TestSetRequireTwoFactor_AnAdminTogglesItAndTheViewReflectsIt(t *testing.T) {
	f := newTwoFactorFixture(t)
	ctx := ctxAs("admin-1")

	w, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, true)
	if err != nil || !w.RequireTwoFactor || w.ID != f.workspace {
		t.Fatalf("enable = (%+v, %v), want the view with requireTwoFactor", w, err)
	}
	if got, _ := f.store.GetTenant(ctx, f.workspace); !got.RequireTwoFactor {
		t.Fatal("the store row did not change")
	}
	got, err := f.svc.GetWorkspace(ctx, f.workspace)
	if err != nil || !got.RequireTwoFactor {
		t.Fatalf("GetWorkspace = (%+v, %v), want requireTwoFactor", got, err)
	}
	list, err := f.svc.List(ctx)
	if err != nil || len(list) != 1 || !list[0].RequireTwoFactor {
		t.Fatalf("List = (%+v, %v), want the flag on the listed workspace", list, err)
	}
	if len(f.changed) != 1 || f.changed[0] != f.workspace {
		t.Fatalf("local cache invalidations = %v, want exactly the changed workspace", f.changed)
	}

	// The allowed row carries Render's verb source and the value set.
	rows := f.audit.verbs(core.AuditAllowed)
	if len(rows) != 1 || rows[0].TwoFactorRequiredTo == nil || !*rows[0].TwoFactorRequiredTo ||
		rows[0].Resource != core.WorkspaceObject(f.workspace) || rows[0].Caller != "admin-1" {
		t.Fatalf("audit rows = %+v, want one allowed row recording enabled=true", rows)
	}

	off, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, false)
	if err != nil || off.RequireTwoFactor {
		t.Fatalf("disable = (%+v, %v)", off, err)
	}
	rows = f.audit.verbs(core.AuditAllowed)
	if len(rows) != 2 || *rows[1].TwoFactorRequiredTo {
		t.Fatalf("audit rows = %+v, want a second row recording enabled=false", rows)
	}
}

func TestSetRequireTwoFactor_ANoOpWritesNoAuditRowAndNoInvalidation(t *testing.T) {
	f := newTwoFactorFixture(t)
	ctx := ctxAs("admin-1")
	if _, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, false); err != nil {
		t.Fatalf("disabling an already-off setting = %v", err)
	}
	if _, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, true); err != nil {
		t.Fatalf("enabling an already-on setting = %v", err)
	}
	if rows := f.audit.verbs(core.AuditAllowed); len(rows) != 1 {
		t.Fatalf("audit rows = %d after one real change and two no-ops, want 1", len(rows))
	}
	if len(f.changed) != 1 {
		t.Fatalf("invalidations = %v, want one (the real change)", f.changed)
	}
}

func TestSetRequireTwoFactor_OnlyAnAdminMayChangeIt(t *testing.T) {
	f := newTwoFactorFixture(t)
	for _, subject := range []string{"dev-1", "viewer-1", "stranger"} {
		_, err := f.svc.SetRequireTwoFactor(ctxAs(subject), f.workspace, true)
		if !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s enable = %v, want ErrForbidden", subject, err)
		}
	}
	if got, _ := f.store.GetTenant(context.Background(), f.workspace); got.RequireTwoFactor {
		t.Fatal("a refused mutation changed the setting")
	}
	if rows := f.audit.verbs(core.AuditAllowed); len(rows) != 0 {
		t.Fatalf("allowed rows = %+v, want none", rows)
	}
	// Every refusal is on the record as a denial.
	if rows := f.audit.verbs(core.AuditDenied); len(rows) != 3 {
		t.Fatalf("denied rows = %d, want one per refused caller", len(rows))
	}
	if len(f.changed) != 0 {
		t.Fatalf("invalidations = %v, want none", f.changed)
	}
}

func TestSetRequireTwoFactor_AnAdminWithoutASecondFactorCannotLockThemselvesOut(t *testing.T) {
	f := newTwoFactorFixture(t)
	_, err := f.svc.SetRequireTwoFactor(ctxAs("admin-2"), f.workspace, true)
	if !isCoded(err, core.ErrConflict, core.CodeWorkspaceTwoFactorAdminNotEnrolled) {
		t.Fatalf("enable as an unenrolled admin = %v, want the coded 409 %s", err, core.CodeWorkspaceTwoFactorAdminNotEnrolled)
	}
	if got, _ := f.store.GetTenant(context.Background(), f.workspace); got.RequireTwoFactor {
		t.Fatal("the guard let the setting change")
	}
	if rows := f.audit.verbs(core.AuditAllowed); len(rows) != 0 || len(f.changed) != 0 {
		t.Fatalf("a refused enable recorded %v / invalidated %v", rows, f.changed)
	}

	// Once they have a second factor, the same call succeeds.
	f.identities["admin-2"] = IdentityAttrs{MFAEnabled: true}
	if w, err := f.svc.SetRequireTwoFactor(ctxAs("admin-2"), f.workspace, true); err != nil || !w.RequireTwoFactor {
		t.Fatalf("enable after enrolling = (%+v, %v)", w, err)
	}
}

func TestSetRequireTwoFactor_TheGuardSpeaksForAKeysCreator(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.svc.KeyOwners = fakeKeyOwners{"key-by-enrolled": "admin-1", "key-by-unenrolled": "admin-2"}
	f.svc.Base.Authz = roleChecker{"key-by-enrolled": "admin", "key-by-unenrolled": "admin", "orphan-key": "admin"}

	if _, err := f.svc.SetRequireTwoFactor(ctxAsAPIKey("key-by-unenrolled"), f.workspace, true); !isCoded(err, core.ErrConflict, core.CodeWorkspaceTwoFactorAdminNotEnrolled) {
		t.Fatalf("a key whose creator has no second factor = %v, want the coded 409", err)
	}
	if _, err := f.svc.SetRequireTwoFactor(ctxAsAPIKey("orphan-key"), f.workspace, true); !isCoded(err, core.ErrConflict, core.CodeWorkspaceTwoFactorAdminNotEnrolled) {
		t.Fatalf("a key with no recorded creator = %v, want the coded 409", err)
	}
	if w, err := f.svc.SetRequireTwoFactor(ctxAsAPIKey("key-by-enrolled"), f.workspace, true); err != nil || !w.RequireTwoFactor {
		t.Fatalf("a key created by an enrolled admin = (%+v, %v), want allowed", w, err)
	}
}

func TestSetRequireTwoFactor_WithoutAnIdentityLookupEnablingIsRefusedAndDisablingIsNot(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.svc.Identities = nil // BEX_KRATOS_ADMIN_URL unset
	_, err := f.svc.SetRequireTwoFactor(ctxAs("admin-1"), f.workspace, true)
	if !isCoded(err, core.ErrConflict, core.CodeWorkspaceTwoFactorUnavailable) {
		t.Fatalf("enable without a lookup = %v, want the coded 409 %s", err, core.CodeWorkspaceTwoFactorUnavailable)
	}
	// A workspace that already has it on (a lookup was configured earlier) must
	// still be able to turn it off.
	f.store.tenants[f.workspace] = store.Tenant{ID: f.workspace, Name: "acme", Plan: "pro", RequireTwoFactor: true}
	if w, err := f.svc.SetRequireTwoFactor(ctxAs("admin-1"), f.workspace, false); err != nil || w.RequireTwoFactor {
		t.Fatalf("disable without a lookup = (%+v, %v), want allowed", w, err)
	}
}

// The legacy default workspace is never enforced (twofactor.Gate skips it), so
// the setting must not be switchable there: a requirement nothing enforces
// would be shown to its members as if it were.
func TestSetRequireTwoFactor_TheDefaultWorkspaceCannotRequireIt(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.store.tenants[core.DefaultTenant] = store.Tenant{ID: core.DefaultTenant, Name: core.DefaultTenant, Plan: "free"}
	f.svc.Base.Authz = roleChecker{"admin-1": "admin"}

	_, err := f.svc.SetRequireTwoFactor(ctxAs("admin-1"), core.DefaultTenant, true)
	if !isCoded(err, core.ErrConflict, core.CodeWorkspaceTwoFactorUnavailable) {
		t.Fatalf("enable on the default workspace = %v, want the coded 409 %s", err, core.CodeWorkspaceTwoFactorUnavailable)
	}
	if got, _ := f.store.GetTenant(context.Background(), core.DefaultTenant); got.RequireTwoFactor {
		t.Fatal("the default workspace's setting changed")
	}
	if rows := f.audit.verbs(core.AuditAllowed); len(rows) != 0 || len(f.changed) != 0 {
		t.Fatalf("a refused enable recorded %v / invalidated %v", rows, f.changed)
	}
	// Disabling an already-off setting there is the no-op it is everywhere.
	if w, err := f.svc.SetRequireTwoFactor(ctxAs("admin-1"), core.DefaultTenant, false); err != nil || w.RequireTwoFactor {
		t.Fatalf("disable on the default workspace = (%+v, %v), want the no-op", w, err)
	}
}

// meetingStore holds every GetTenant caller until n of them have read the row,
// so two admins deterministically both see the old value before either writes:
// the interleaving that makes a read-then-write toggle record itself twice.
type meetingStore struct {
	*fakeStore
	arrived sync.WaitGroup
}

func newMeetingStore(inner *fakeStore, n int) *meetingStore {
	m := &meetingStore{fakeStore: inner}
	m.arrived.Add(n)
	return m
}

func (m *meetingStore) GetTenant(ctx context.Context, id string) (store.Tenant, error) {
	t, err := m.fakeStore.GetTenant(ctx, id)
	m.arrived.Done()
	done := make(chan struct{})
	go func() { m.arrived.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second): // never hang a test on a serialized caller
	}
	return t, err
}

// Two admins turning the setting on at the same moment is one change: one
// write, one audit row, one cache invalidation, and neither of them an error.
func TestSetRequireTwoFactor_TwoAdminsTogglingAtOnceChangeItOnce(t *testing.T) {
	f := newTwoFactorFixture(t)
	f.identities["admin-3"] = IdentityAttrs{MFAEnabled: true}
	f.svc.Base.Authz = roleChecker{"admin-1": "admin", "admin-3": "admin"}
	f.svc.Store = newMeetingStore(f.store, 2)
	var mu sync.Mutex
	f.svc.TwoFactorChanged = func(id string) { mu.Lock(); f.changed = append(f.changed, id); mu.Unlock() }

	var wg sync.WaitGroup
	results := make([]error, 2)
	views := make([]WorkspaceView, 2)
	for i, admin := range []string{"admin-1", "admin-3"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			views[i], results[i] = f.svc.SetRequireTwoFactor(ctxAs(admin), f.workspace, true)
		}()
	}
	wg.Wait()

	for i := range results {
		if results[i] != nil || !views[i].RequireTwoFactor {
			t.Fatalf("admin %d = (%+v, %v), want the enabled view and no error", i, views[i], results[i])
		}
	}
	if got, _ := f.store.GetTenant(context.Background(), f.workspace); !got.RequireTwoFactor {
		t.Fatal("neither admin's write took")
	}
	if rows := f.audit.verbs(core.AuditAllowed); len(rows) != 1 {
		t.Fatalf("audit rows = %d for two simultaneous enables, want 1 (one change)", len(rows))
	}
	if len(f.changed) != 1 {
		t.Fatalf("cache invalidations = %v, want 1", f.changed)
	}
}

func TestSetRequireTwoFactor_StoreErrors(t *testing.T) {
	f := newTwoFactorFixture(t)
	if _, err := f.svc.SetRequireTwoFactor(ctxAs("admin-1"), "tea-missing", true); !errors.Is(err, core.ErrNotFound) && !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("unknown workspace = %v, want not found (or forbidden for a non-member)", err)
	}
	f.svc.Store = nil
	if _, err := f.svc.SetRequireTwoFactor(ctxAs("admin-1"), f.workspace, true); !errors.Is(err, core.ErrWorkspacesUnavailable) {
		t.Fatalf("no store = %v, want ErrWorkspacesUnavailable", err)
	}
}

// refusingGate refuses every workspace, standing in for a blocked member.
type refusingGate struct{}

func (refusingGate) Check(_ context.Context, workspaceID string, _ core.Identity) error {
	return core.NewWorkspaceTwoFactorRequiredError(workspaceID)
}

func TestABlockedMemberCanStillListTheirWorkspacesAndWhoTheyAre(t *testing.T) {
	f := newTwoFactorFixture(t)
	if _, err := f.svc.SetRequireTwoFactor(ctxAs("admin-1"), f.workspace, true); err != nil {
		t.Fatal(err)
	}
	f.svc.Base.Authz = roleChecker{"viewer-1": "viewer", "admin-1": "admin"}
	f.svc.Base.TwoFactor = refusingGate{}
	f.store.members[f.workspace] = append(f.store.members[f.workspace], store.TenantMember{TenantID: f.workspace, Subject: "viewer-1", Role: "viewer"})

	// The two exempt reads work, and the list says why the member is blocked.
	list, err := f.svc.List(ctxAs("viewer-1"))
	if err != nil || len(list) != 1 || !list[0].RequireTwoFactor {
		t.Fatalf("List as a blocked member = (%+v, %v), want the workspace with requireTwoFactor", list, err)
	}
	if _, err := f.svc.CurrentUser(ctxAsSession("viewer-1", "v@example.com", "V")); err != nil {
		t.Fatalf("CurrentUser as a blocked member = %v", err)
	}
	owners, err := f.svc.ListOwners(ctxAs("viewer-1"), OwnerFilter{})
	if err != nil || len(owners) != 1 {
		t.Fatalf("ListOwners as a blocked member = (%+v, %v)", owners, err)
	}

	// Everything else of the workspace is refused with the coded 403.
	_, err = f.svc.GetWorkspace(ctxAs("viewer-1"), f.workspace)
	if !isCoded(err, core.ErrForbidden, core.CodeWorkspaceTwoFactorRequired) {
		t.Fatalf("GetWorkspace as a blocked member = %v, want the coded 403", err)
	}
	if _, err := f.svc.Rename(ctxAs("admin-1"), f.workspace, "x"); !isCoded(err, core.ErrForbidden, core.CodeWorkspaceTwoFactorRequired) {
		t.Fatalf("Rename as a blocked admin = %v, want the coded 403", err)
	}
}

// The two exempt reads answer a blocked member with membership-scoped facts and
// nothing of any workspace's resources, and the exemption lives only inside the
// verb: the context the caller keeps is still refused afterwards.
func TestExemptReadsLeakNothingAboutAWorkspaceToABlockedMember(t *testing.T) {
	f := newTwoFactorFixture(t)
	other, err := f.store.CreateWorkspace(context.Background(), "other-co", "pro", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	f.store.members[f.workspace] = append(f.store.members[f.workspace], store.TenantMember{TenantID: f.workspace, Subject: "viewer-1", Role: "viewer"})
	f.svc.Base.Authz = roleChecker{"viewer-1": "viewer", "admin-1": "admin"}
	f.svc.Base.TwoFactor = refusingGate{}

	ctx := ctxAs("viewer-1")
	list, err := f.svc.List(ctx)
	if err != nil {
		t.Fatalf("List as a blocked member = %v", err)
	}
	var ids []string
	for _, w := range list {
		ids = append(ids, w.ID)
	}
	if !slices.Equal(ids, []string{f.workspace}) {
		t.Fatalf("a blocked member's workspaces = %v, want only the one they belong to (not %s)", ids, other.ID)
	}

	// The exemption does not escape the verb: the same context is still refused
	// on a workspace verb, for the workspace they are in and the one they are not.
	for _, id := range []string{f.workspace, other.ID} {
		if _, err := f.svc.GetWorkspace(ctx, id); err == nil {
			t.Fatalf("GetWorkspace(%s) on the context that just listed = nil, want a refusal", id)
		}
	}
	if _, err := f.svc.GetWorkspace(ctx, f.workspace); !isCoded(err, core.ErrForbidden, core.CodeWorkspaceTwoFactorRequired) {
		t.Fatalf("GetWorkspace on the context that just listed = %v, want the coded 403", err)
	}

	// What the exempt reads return is pinned to non-resource fields, so adding a
	// field that exposes workspace contents is a decision, not an accident.
	fieldNames := func(v any) []string {
		var names []string
		for field := range reflect.TypeOf(v).Fields() {
			names = append(names, field.Name)
		}
		return names
	}
	if got, want := fieldNames(WorkspaceView{}), []string{"ID", "Name", "Slug", "Plan", "Role", "CreatedAt", "RequireTwoFactor"}; !slices.Equal(got, want) {
		t.Errorf("WorkspaceView fields = %v, want %v: List is exempt from the 2FA requirement", got, want)
	}
	if got, want := fieldNames(UserView{}), []string{"ID", "Email", "Name"}; !slices.Equal(got, want) {
		t.Errorf("UserView fields = %v, want %v: CurrentUser is exempt from the 2FA requirement", got, want)
	}
}

// storeSettings reads the workspace setting like cmd/api's adapter over the store.
type storeSettings struct{ *fakeStore }

func (s storeSettings) RequireTwoFactor(ctx context.Context, id string) (bool, error) {
	t, err := s.GetTenant(ctx, id)
	return t.RequireTwoFactor, err
}

// ADR012 §9a: an admin who removes their last second factor on an enforcing
// workspace is blocked like any member, so cannot disable the setting until they
// enroll again; the block shows up within the gate's TTL, and lifting it is
// enrolling again. Run through the real gate, not a stub.
func TestAnAdminWhoLosesTheirSecondFactorAfterEnablingIsBlockedUntilTheyEnrollAgain(t *testing.T) {
	f := newTwoFactorFixture(t)
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	gate := twofactor.New(twofactor.Config{Settings: storeSettings{f.store}, Enrollment: f.identities, Now: func() time.Time { return now }})
	f.svc.Base.TwoFactor = gate
	f.svc.TwoFactorChanged = gate.Invalidate
	ctx := ctxAs("admin-1")

	if _, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, true); err != nil {
		t.Fatalf("enable as an enrolled admin = %v", err)
	}
	// While enrolled they pass the gate on the workspace they just locked down.
	if _, err := f.svc.GetWorkspace(ctx, f.workspace); err != nil {
		t.Fatalf("an enrolled admin on an enforcing workspace = %v", err)
	}

	f.identities["admin-1"] = IdentityAttrs{} // removes their last factor
	now = now.Add(twofactor.TTL + time.Second)
	if _, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, false); !isCoded(err, core.ErrForbidden, core.CodeWorkspaceTwoFactorRequired) {
		t.Fatalf("disable as an admin who lost their factor = %v, want the coded 403 (intended: they must enroll again)", err)
	}
	if got, _ := f.store.GetTenant(context.Background(), f.workspace); !got.RequireTwoFactor {
		t.Fatal("a blocked admin changed the setting")
	}

	f.identities["admin-1"] = IdentityAttrs{MFAEnabled: true}
	now = now.Add(twofactor.TTL + time.Second)
	if w, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, false); err != nil || w.RequireTwoFactor {
		t.Fatalf("disable after enrolling again = (%+v, %v), want allowed", w, err)
	}
}

func TestWorkspaceGraphQL_RequireTwoFactorFieldAndMutation(t *testing.T) {
	f := newTwoFactorFixture(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: f.svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: f.svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	run := func(subject, query string) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: ctxAs(subject)})
	}
	mutation := fmt.Sprintf(`mutation { setWorkspaceRequireTwoFactor(workspaceId: %q, enabled: true) { id requireTwoFactor } }`, f.workspace)

	denied := run("viewer-1", mutation)
	if len(denied.Errors) != 1 || denied.Errors[0].Extensions["code"] != "FORBIDDEN" {
		t.Fatalf("a viewer's mutation = %+v, want a FORBIDDEN error", denied.Errors)
	}
	// An admin with no second factor of their own is refused with the coded 409.
	lockout := run("admin-2", mutation)
	if len(lockout.Errors) != 1 || lockout.Errors[0].Extensions["code"] != core.CodeWorkspaceTwoFactorAdminNotEnrolled {
		t.Fatalf("an unenrolled admin's mutation = %+v, want the %s error", lockout.Errors, core.CodeWorkspaceTwoFactorAdminNotEnrolled)
	}
	res := run("admin-1", mutation)
	if len(res.Errors) != 0 {
		t.Fatalf("admin mutation errors = %v", res.Errors)
	}
	set := res.Data.(map[string]any)["setWorkspaceRequireTwoFactor"].(map[string]any)
	if set["id"] != f.workspace || set["requireTwoFactor"] != true {
		t.Fatalf("mutation result = %v", set)
	}

	// Any member reads the field: a blocked member must be able to see why.
	read := run("admin-1", `{ workspaces { id requireTwoFactor } }`)
	if len(read.Errors) != 0 {
		t.Fatalf("read errors = %v", read.Errors)
	}
	ws := read.Data.(map[string]any)["workspaces"].([]any)
	if len(ws) != 1 || ws[0].(map[string]any)["requireTwoFactor"] != true {
		t.Fatalf("workspaces = %v, want the flag", ws)
	}
	// The non-null Boolean argument is mandatory.
	missing := run("admin-1", fmt.Sprintf(`mutation { setWorkspaceRequireTwoFactor(workspaceId: %q) { id } }`, f.workspace))
	if len(missing.Errors) == 0 {
		t.Fatal("enabled is required: a call without it must be a validation error")
	}
}
