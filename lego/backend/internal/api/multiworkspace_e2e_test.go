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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// multiworkspace_e2e_test.go is w6/m14's end-to-end proof (t007), against REAL
// infrastructure — a live Postgres (the membership source of truth) and a live
// OpenFGA (enforced, per-workspace roles) — driving the REAL REST surface
// through the REAL store-backed workspace resolver (api.tenantService, not a
// fake): the whole chain the field bug ran through.
//
// It is the same harness TestWorkspaceLifecycleE2E uses, and skipped the same
// way unless both point at throwaway infra:
//
//	BEX_TEST_DB_URI=postgres://postgres:pw@localhost:5433/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api/ -run TestMultiWorkspaceTargetingE2E -v
//
// The OpenFGA at that URL must have a store named "bex" carrying
// deploy/gitops/authz/model.json (scripts/authz-model.sh).
func TestMultiWorkspaceTargetingE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		t.Skip("BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()

	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	// This test is ADDITIVE — it never truncates. `go test ./...` runs packages
	// concurrently, so a TRUNCATE here would race internal/store's tests against
	// the same throwaway database and fail them from the outside. Instead every
	// workspace name and every subject is unique to this run, so the assertions
	// (notably "dana's oldest membership") hold no matter what else is in the DB.
	// A short, unique-per-run suffix. The TRAILING chars of an xid are its
	// counter/pid; the leading ones are a timestamp, so two runs in the same
	// second would collide on a prefix. Short because an App name is a DNS label
	// (30 chars max) and these names are embedded in one.
	full := ids.New(ids.Workspace)
	run := full[len(full)-8:]
	dana, carl, erin := "dana-"+run, "carl-"+run, "erin-"+run
	ws := func(kind string) string { return kind + "-" + run }

	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	granter := checker.(workspaces.WorkspaceGranter)
	// The full role-granting seam (store.MembershipGranter) — needed to make carl a
	// VIEWER of bravo, a role WorkspaceGranter's admin-only grant cannot express.
	roles := checker.(store.MembershipGranter)

	// dana's two workspaces, in the order she got them: A first (her default), B
	// second. Real tenants + tenant_members rows; real FGA admin tuples.
	wsA, err := st.CreateWorkspace(ctx, ws("alpha"), store.PlanHobby, dana)
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	wsB, err := st.CreateWorkspace(ctx, ws("bravo"), store.PlanHobby, dana)
	if err != nil {
		t.Fatalf("create bravo: %v", err)
	}
	// erin's workspace — one dana has no membership in at all.
	wsE, err := st.CreateWorkspace(ctx, ws("echo"), store.PlanHobby, erin)
	if err != nil {
		t.Fatalf("create echo: %v", err)
	}
	for _, w := range []store.Tenant{wsA, wsB} {
		if err := granter.GrantWorkspaceAdmin(ctx, w.ID, "user:"+dana); err != nil {
			t.Fatalf("grant admin on %s: %v", w.Name, err)
		}
	}
	if err := granter.GrantWorkspaceAdmin(ctx, wsE.ID, "user:"+erin); err != nil {
		t.Fatalf("grant admin on echo: %v", err)
	}
	// carl is a VIEWER of bravo and an ADMIN of his own workspace — the caller
	// who makes membership-alone-is-enough a privilege escalation.
	wsC, err := st.CreateWorkspace(ctx, ws("charlie"), store.PlanHobby, carl)
	if err != nil {
		t.Fatalf("create charlie: %v", err)
	}
	if err := granter.GrantWorkspaceAdmin(ctx, wsC.ID, "user:"+carl); err != nil {
		t.Fatalf("grant admin on charlie: %v", err)
	}
	if err := st.AddMember(ctx, carl, wsB.ID, "viewer"); err != nil {
		t.Fatalf("add carl to bravo: %v", err)
	}
	if err := roles.GrantWorkspaceRole(ctx, wsB.ID, "user:"+carl, "viewer"); err != nil {
		t.Fatalf("grant carl viewer on bravo: %v", err)
	}

	// The REAL resolver: store-backed TenantForIdentity (the ORDER BY) + IsMember.
	cl := fakeClient()
	base := &core.Base{
		Client: cl, Namespace: "default",
		Authz:     checker,
		Workspace: NewTenantService(st, roles),
	}
	// A fake HostedProvider so an AUTHORIZED billing verb returns a real 2xx, not
	// the 503 a nil provider yields — making the w1/m60 billing allow-vs-deny below
	// unambiguous (a denied caller still 403s before the provider is ever reached).
	srv := NewServer(base, Deps{Store: st, WorkspaceStore: st, WorkspaceGranter: granter, Billing: e2eBillingProvider{}})
	mux := srv.restHandler()

	// call drives the real REST router as `subject` (the auth middleware's job —
	// attaching the Identity — is done here, as the other e2e tests do).
	call := func(subject, method, path, body string) *httptest.ResponseRecorder {
		return e2eCall(mux, ctx, subject, method, path, body)
	}
	// appTenantOf reads back an App by its exact tenant-scoped object name
	// (core.CRName, w4/m19) — a store-managed App's object name is no longer
	// its public name, since two workspaces may now legitimately share one.
	appTenantOf := func(t *testing.T, tenantID, name string) string {
		t.Helper()
		var a appv1alpha1.App
		if err := cl.Get(ctx, k8sKey(tenantID, name), &a); err != nil {
			t.Fatalf("App %s/%s: %v", tenantID, name, err)
		}
		return a.Labels[core.LabelTenant]
	}

	bravoWeb, stolen := "bravo-web-"+run, "stolen-"+run

	// (0) dana's DEFAULT workspace is alpha — her OLDEST membership, not a coin
	// flip. This is the t001 contract, read through the production resolver.
	if got, err := st.TenantForIdentity(ctx, dana); err != nil || got.ID != wsA.ID {
		t.Fatalf("default workspace = %+v (err %v), want alpha %s", got, err, wsA.ID)
	}

	// (1) A create NAMING bravo lands in bravo — not in alpha, which is what the
	// implicit resolution would have picked.
	rec := call(dana, "POST", "/v1/services", `{"name":"`+bravoWeb+`","image":{"imagePath":"nginx"},"ownerId":"`+wsB.ID+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/services ownerId=bravo: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Service struct{ OwnerID string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Service.OwnerID != wsB.ID {
		t.Errorf("response ownerId = %q, want bravo %q", created.Service.OwnerID, wsB.ID)
	}
	if got := appTenantOf(t, wsB.ID, bravoWeb); got != wsB.ID {
		t.Errorf("App landed in %q, want bravo %q", got, wsB.ID)
	}

	// (2) THE w6/m11 REGRESSION: dana reads her bravo service with NO ownerId, so
	// the implicit resolution picks alpha. It must serve her own service, not 403.
	if rec := call(dana, "GET", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusOK {
		t.Errorf("GET her own bravo service (implicit resolution = alpha): %d %s — want 200; an owner must never be 403'd by which of her workspaces resolved",
			rec.Code, rec.Body.String())
	}
	// ...and she can operate on it, too (the write path through the same gate).
	if rec := call(dana, "POST", "/v1/services/"+bravoWeb+"/restart", ""); rec.Code != http.StatusOK {
		t.Errorf("restart her own bravo service: %d %s — want 200", rec.Code, rec.Body.String())
	}

	// (3) A create naming a workspace dana does NOT belong to is refused — and
	// crucially, refused rather than silently redirected into one of her own.
	rec = call(dana, "POST", "/v1/services", `{"name":"`+stolen+`","image":{"imagePath":"nginx"},"ownerId":"`+wsE.ID+`"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST ownerId=echo (not dana's): %d %s, want 403", rec.Code, rec.Body.String())
	}
	var stray appv1alpha1.App
	if err := cl.Get(ctx, k8sKey(wsE.ID, stolen), &stray); err == nil {
		t.Errorf("the refused create still wrote an App, labeled %q — it must write nothing", stray.Labels[core.LabelTenant])
	}

	// (4) NO CROSS-WORKSPACE PRIVILEGE ESCALATION. carl is an ADMIN of charlie and
	// only a VIEWER of bravo. He may READ bravo's service (viewer ⇒ can_view)...
	if rec := call(carl, "GET", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusOK {
		t.Errorf("carl (viewer of bravo) GET bravo-web: %d %s, want 200", rec.Code, rec.Body.String())
	}
	// ...but he must NOT be able to delete it: can_create does not hold for a
	// viewer in bravo, even though he is an admin in his own default workspace,
	// which is the workspace his implicit resolution picks.
	if rec := call(carl, "DELETE", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusForbidden {
		t.Errorf("carl (viewer of bravo, admin of charlie) DELETE bravo-web: %d %s, want 403 — a role must not leak across workspaces",
			rec.Code, rec.Body.String())
	}
	if err := cl.Get(ctx, k8sKey(wsB.ID, bravoWeb), &stray); err != nil {
		t.Errorf("bravo-web was deleted by a viewer: %v", err)
	}
	// ...and he cannot read its env vars either (can_view_sensitive: developer+).
	if rec := call(carl, "GET", "/v1/services/"+bravoWeb+"/env-vars", ""); rec.Code == http.StatusOK {
		t.Errorf("carl (viewer of bravo) read bravo-web's env vars: %d — want a refusal", rec.Code)
	}

	// (5) erin, a member of NEITHER of dana's workspaces, cannot reach her
	// service at all — the isolation that predates m14 and must still hold.
	if rec := call(erin, "GET", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusForbidden {
		t.Errorf("erin GET dana's service: %d %s, want 403", rec.Code, rec.Body.String())
	}

	// (6) PER-SERVICE SAMPLE against the REAL FGA model (w7/m55/t004). The
	// fake-checker matrices (crossworkspace_*_test.go) catch route/verb WIRING
	// regressions but use a fake checker, so an FGA MODEL regression — a schema
	// change that let a viewer's can_view resolve to can_create, or made a
	// relation public — would pass them. These cases drive one representative
	// read and one representative write per resource OBJECT TYPE (Database,
	// KeyValue — beyond the App the cases above already cover) through the real
	// OpenFGA model, so such a regression fails CI. For each: erin (non-member)
	// is denied both; carl (VIEWER of bravo) may read (can_view) but not delete
	// (can_create) — the exact per-relation resolution only the real model
	// exercises. bravo's Database and KeyValue are seeded straight into the fake
	// apiserver, labeled bravo, since this test drives the authz path, not the
	// operator's provisioning.
	bravoDB, bravoKV := "bravo-db-"+run, "bravo-kv-"+run
	if err := cl.Create(ctx, ownedDB(bravoDB, wsB.ID)); err != nil {
		t.Fatalf("seed bravo Database: %v", err)
	}
	if err := cl.Create(ctx, ownedKV(bravoKV, wsB.ID)); err != nil {
		t.Fatalf("seed bravo KeyValue: %v", err)
	}
	perObject := []struct {
		object, readPath, writePath string
	}{
		{"Database", "/v1/postgres/" + bravoDB, "/v1/postgres/" + bravoDB},
		{"KeyValue", "/v1/key-value/" + bravoKV, "/v1/key-value/" + bravoKV},
	}
	for _, o := range perObject {
		// erin (non-member) — denied read AND write, the isolation floor.
		if rec := call(erin, "GET", o.readPath, ""); rec.Code != http.StatusForbidden {
			t.Errorf("erin (non-member) GET %s [%s]: %d %s, want 403", o.readPath, o.object, rec.Code, rec.Body.String())
		}
		if rec := call(erin, "DELETE", o.writePath, ""); rec.Code != http.StatusForbidden {
			t.Errorf("erin (non-member) DELETE %s [%s]: %d %s, want 403", o.writePath, o.object, rec.Code, rec.Body.String())
		}
		// carl (viewer of bravo) — can_view holds, can_create does not: the model
		// regression catch. A read that 403s would be a false negative; a delete
		// that 2xxes would be the role leak this asserts against.
		if rec := call(carl, "GET", o.readPath, ""); rec.Code != http.StatusOK {
			t.Errorf("carl (viewer of bravo) GET %s [%s]: %d %s, want 200 — can_view must hold for a viewer",
				o.readPath, o.object, rec.Code, rec.Body.String())
		}
		if rec := call(carl, "DELETE", o.writePath, ""); rec.Code != http.StatusForbidden {
			t.Errorf("carl (viewer of bravo) DELETE %s [%s]: %d %s, want 403 — can_create must NOT hold for a viewer",
				o.writePath, o.object, rec.Code, rec.Body.String())
		}
	}

	// (7) Members read/write against the real model on the WORKSPACE object: carl,
	// a viewer of bravo, may LIST its members (can_view) but not INVITE (can_manage
	// — admin only). This exercises the AuthorizeOn(workspace) seam the resource
	// verbs above don't touch.
	if rec := call(carl, "GET", "/v1/owners/"+wsB.ID+"/members", ""); rec.Code != http.StatusOK {
		t.Errorf("carl (viewer of bravo) GET bravo members: %d %s, want 200 — can_view must hold", rec.Code, rec.Body.String())
	}
	if rec := call(carl, "POST", "/v1/workspaces/"+wsB.ID+"/members", `{"email":"x@example.com","role":"viewer"}`); rec.Code != http.StatusForbidden {
		t.Errorf("carl (viewer of bravo) invite a member: %d %s, want 403 — can_manage must NOT hold for a viewer", rec.Code, rec.Body.String())
	}
	if rec := call(erin, "GET", "/v1/owners/"+wsB.ID+"/members", ""); rec.Code != http.StatusForbidden {
		t.Errorf("erin (non-member) GET bravo members: %d %s, want 403", rec.Code, rec.Body.String())
	}

	// (8) w7/m61 — the INSIDER role ladder against the REAL model, one verb per
	// relation class per lower rung. The fake-checker matrix (roleladder_test.go)
	// proves the ladder for every verb; these prove the deployed model resolves the
	// per-role relations correctly end-to-end (a schema change that let a
	// contributor's can_operate resolve to can_create, say, fails here). carl
	// (viewer) is covered above; add a CONTRIBUTOR and a BILLING member of bravo,
	// each with their own home workspace (mirroring carl) so the resolver always has
	// a default membership.
	fred, gwen := "fred-"+run, "gwen-"+run
	wsF, err := st.CreateWorkspace(ctx, ws("foxtrot"), store.PlanHobby, fred)
	if err != nil {
		t.Fatalf("create foxtrot: %v", err)
	}
	wsG, err := st.CreateWorkspace(ctx, ws("golf"), store.PlanHobby, gwen)
	if err != nil {
		t.Fatalf("create golf: %v", err)
	}
	for subj, w := range map[string]store.Tenant{fred: wsF, gwen: wsG} {
		if err := granter.GrantWorkspaceAdmin(ctx, w.ID, "user:"+subj); err != nil {
			t.Fatalf("grant admin on %s: %v", w.Name, err)
		}
	}
	for subj, role := range map[string]string{fred: "contributor", gwen: "billing"} {
		if err := st.AddMember(ctx, subj, wsB.ID, role); err != nil {
			t.Fatalf("add %s to bravo as %s: %v", subj, role, err)
		}
		if err := roles.GrantWorkspaceRole(ctx, wsB.ID, "user:"+subj, role); err != nil {
			t.Fatalf("grant %s %s on bravo: %v", subj, role, err)
		}
	}

	// CONTRIBUTOR (fred): can_operate holds (restart 2xx); can_create does not
	// (delete 403); can_view_sensitive does not (env vars refused).
	if rec := call(fred, "POST", "/v1/services/"+bravoWeb+"/restart", ""); rec.Code != http.StatusOK {
		t.Errorf("fred (contributor of bravo) restart bravo-web: %d %s, want 200 — can_operate must hold", rec.Code, rec.Body.String())
	}
	if rec := call(fred, "DELETE", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusForbidden {
		t.Errorf("fred (contributor) DELETE bravo-web: %d %s, want 403 — can_create must NOT hold for a contributor", rec.Code, rec.Body.String())
	}
	if rec := call(fred, "GET", "/v1/services/"+bravoWeb+"/env-vars", ""); rec.Code == http.StatusOK {
		t.Errorf("fred (contributor) read bravo-web env vars: %d — want a refusal (can_view_sensitive is developer+)", rec.Code)
	}

	// BILLING (gwen): can_view holds (read 2xx); resource mutation denied (delete
	// 403); sensitive read denied (env vars refused). The billing MANAGEMENT verbs
	// (Status/Checkout/Portal) she IS allowed — they gate on can_manage_billing
	// (billing or admin), asserted against the real model in (9) just below (w1/m60).
	if rec := call(gwen, "GET", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusOK {
		t.Errorf("gwen (billing of bravo) GET bravo-web: %d %s, want 200 — can_view must hold", rec.Code, rec.Body.String())
	}
	if rec := call(gwen, "DELETE", "/v1/services/"+bravoWeb, ""); rec.Code != http.StatusForbidden {
		t.Errorf("gwen (billing) DELETE bravo-web: %d %s, want 403 — can_create must NOT hold for a billing member", rec.Code, rec.Body.String())
	}
	if rec := call(gwen, "GET", "/v1/services/"+bravoWeb+"/env-vars", ""); rec.Code == http.StatusOK {
		t.Errorf("gwen (billing) read bravo-web env vars: %d — want a refusal (can_view_sensitive is developer+)", rec.Code)
	}

	// Neither lower rung's denied mutation actually deleted the service.
	if err := cl.Get(ctx, k8sKey(wsB.ID, bravoWeb), &stray); err != nil {
		t.Errorf("bravo-web was deleted by a contributor/billing member: %v", err)
	}

	// (9) w1/m60 — the billing MANAGEMENT ladder against the REAL model. The billing
	// verbs (Status/Checkout/Portal, all funnelling through billing.authorize) gate
	// on can_manage_billing, which model.fga resolves as `billing or admin`. So the
	// BILLING role reaches them and every lower/other rung is refused — the exact
	// per-relation resolution only the deployed model exercises, and the closeout of
	// the dead-relation finding (w7/014) this milestone fixes. Each verb targets the
	// NAMED workspace (bravo via the {workspaceId} path), so this checks can_manage_billing
	// on bravo — where gwen is billing and carl/fred are not — never a caller's home.
	billingBase := "/v1/workspaces/" + wsB.ID + "/billing"
	checkoutBody := `{"successUrl":"https://dashboard.bex.co/usage","cancelUrl":"https://dashboard.bex.co/usage"}`
	portalBody := `{"returnUrl":"https://dashboard.bex.co/usage"}`

	// gwen (BILLING) — allowed all three billing verbs (can_manage_billing holds).
	if rec := call(gwen, "GET", billingBase, ""); rec.Code != http.StatusOK {
		t.Errorf("gwen (billing) GET billing readiness: %d %s, want 200 — can_manage_billing must hold for the billing role", rec.Code, rec.Body.String())
	}
	if rec := call(gwen, "POST", billingBase+"/checkout-session", checkoutBody); rec.Code != http.StatusCreated {
		t.Errorf("gwen (billing) POST checkout-session: %d %s, want 201 — the billing role manages billing", rec.Code, rec.Body.String())
	}
	if rec := call(gwen, "POST", billingBase+"/portal-session", portalBody); rec.Code != http.StatusCreated {
		t.Errorf("gwen (billing) POST portal-session: %d %s, want 201 — the billing role opens the portal", rec.Code, rec.Body.String())
	}

	// dana (ADMIN of bravo) — still reaches billing (can_manage_billing: `billing or admin`).
	if rec := call(dana, "GET", billingBase, ""); rec.Code != http.StatusOK {
		t.Errorf("dana (admin of bravo) GET billing readiness: %d %s, want 200 — admin retains billing management", rec.Code, rec.Body.String())
	}

	// Every OTHER rung is refused all three — can_manage_billing does NOT hold. This
	// is the "denials unchanged" half of the DoD: only the billing rows flipped.
	for _, d := range []struct{ subject, role string }{
		{carl, "viewer"}, {fred, "contributor"}, {erin, "non-member"},
	} {
		if rec := call(d.subject, "GET", billingBase, ""); rec.Code != http.StatusForbidden {
			t.Errorf("%s (%s of bravo) GET billing readiness: %d %s, want 403 — can_manage_billing must NOT hold", d.subject, d.role, rec.Code, rec.Body.String())
		}
		if rec := call(d.subject, "POST", billingBase+"/checkout-session", checkoutBody); rec.Code != http.StatusForbidden {
			t.Errorf("%s (%s of bravo) POST checkout-session: %d %s, want 403 — billing management is not theirs", d.subject, d.role, rec.Code, rec.Body.String())
		}
	}
}

// e2eBillingProvider is a stand-in billing.HostedProvider for the real-FGA E2E:
// it returns fixed hosted URLs so an AUTHORIZED billing verb yields a real 2xx,
// isolating the authorization decision (403 vs allowed) from Stripe availability.
type e2eBillingProvider struct{}

func (e2eBillingProvider) Readiness(_ context.Context, workspaceID string) (billing.Readiness, error) {
	return billing.Readiness{WorkspaceID: workspaceID, Mode: "test"}, nil
}

func (e2eBillingProvider) CreateCheckoutSession(_ context.Context, _ string, _ billing.CheckoutRequest) (billing.HostedSession, error) {
	return billing.HostedSession{URL: "https://checkout.stripe.test/session"}, nil
}

func (e2eBillingProvider) CreatePortalSession(_ context.Context, _ string, _ billing.PortalRequest) (billing.HostedSession, error) {
	return billing.HostedSession{URL: "https://billing.stripe.test/portal"}, nil
}

// k8sKey is the fake apiserver's object key for a store-managed App: object-
// named core.CRName(tenantID, name), living in the tenant's own `<ws>`
// namespace (ADR043) — store.WorkspaceNamespace is the identity function, so
// that namespace is the tenant id itself.
func k8sKey(tenantID, name string) client.ObjectKey {
	return client.ObjectKey{Namespace: tenantID, Name: core.CRName(tenantID, name)}
}

// e2eCall drives mux's REST router as `subject` (the auth middleware's job —
// attaching the Identity — is done here, as the auth gate would). Shared by
// every real-infrastructure e2e test in this package (multiworkspace, w6013)
// so the request-building dance lives in one place.
func e2eCall(mux http.Handler, ctx context.Context, subject, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	ictx := core.WithIdentity(ctx, core.Identity{Subject: subject, Method: "session"})
	mux.ServeHTTP(rec, req.WithContext(ictx))
	return rec
}
