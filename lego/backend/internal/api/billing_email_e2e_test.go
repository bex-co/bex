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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// customerEmails stands in for the Stripe Customer: the email each
// workspace's Customer was last given.
type customerEmails struct {
	mu   sync.Mutex
	sets map[string][]string
}

func (c *customerEmails) SetCustomerEmail(_ context.Context, workspaceID, email string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets[workspaceID] = append(c.sets[workspaceID], email)
	return nil
}

func (c *customerEmails) of(workspaceID string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.sets[workspaceID])
}

// billing_email_e2e_test.go is w6/m627's proof against REAL Postgres and REAL
// OpenFGA through the composed REST, GraphQL and MCP surfaces. In a Scale
// workspace the Billing member and the Admin set its billing email on each
// surface, the Stripe Customer gets each address, and Admin, Developer and
// Billing read it back on each; the Developer cannot set it, and a
// Contributor and a Viewer cannot read it. A Hobby workspace's billing email
// is its owner's account email: every surface refuses the write with
// PLAN_FEATURE_REQUIRED (feature billingEmail, requiredPlan pro) and changes
// nothing. Upgraded, it sets its own; moved back to Hobby, the address is
// cleared and the Customer gets the owner's back.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestBillingEmailOnEverySurfaceE2E -v
func TestBillingEmailOnEverySurfaceE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
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

	run := uniqueRunSuffix()
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	workspace := func(name, plan, admin string) store.Tenant {
		t.Helper()
		ws, err := st.CreateWorkspace(ctx, name+"-"+run, plan, admin)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+admin, "admin"); err != nil {
			t.Fatal(err)
		}
		return ws
	}
	admin, hobbyAdmin := "admin-"+run, "hobbyadmin-"+run
	scale := workspace("billingemail", store.PlanScale, admin)
	hobby := workspace("billingemailhobby", store.PlanHobby, hobbyAdmin)
	members := map[string]string{"admin": admin}
	for _, role := range []string{"developer", "contributor", "viewer", "billing"} {
		subject := role + "-" + run
		if err := st.AddMember(ctx, subject, scale.ID, role); err != nil {
			t.Fatal(err)
		}
		if err := roles.GrantWorkspaceRole(ctx, scale.ID, "user:"+subject, role); err != nil {
			t.Fatal(err)
		}
		members[role] = subject
	}

	stripe := &customerEmails{sets: map[string][]string{}}
	resolver := tenancy.NewTenantService(st, roles)
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: checker, Workspace: resolver, KeyGrants: resolver}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: roles,
		Identities:             metadataIdentities{}, // <subject>@example.com
		WorkspaceCustomerEmail: stripe,
	})
	mux := srv.restHandler()
	gated := srv.withScopeClassREST(mux, mux)
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema

	session := func(subject string) core.Identity { return core.Identity{Subject: subject, Method: "session"} }
	rest := func(id core.Identity, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, id)))
		return rec
	}
	path := func(ws store.Tenant) string { return "/v1/workspaces/" + ws.ID + "/billing/email" }
	// read answers the billing email each surface gives id, refusing the test
	// when the surfaces disagree.
	read := func(who string, id core.Identity, ws store.Tenant) (email string, configurable, refused bool) {
		t.Helper()
		rec := rest(id, http.MethodGet, path(ws), "")
		gql := graphqlDo(t, srv, id, fmt.Sprintf(`{ workspaceBillingEmail(workspaceId: %q) { workspaceId email configurable } }`, ws.ID))
		res := mcpCall(t, srv, id, "get_billing_email", map[string]any{})
		if rec.Code == http.StatusForbidden {
			if graphqlCode(gql) != "FORBIDDEN" || !strings.Contains(strings.ToLower(mcpText(res)), "forbidden") {
				t.Fatalf("%s: REST refused but GraphQL %v, MCP %s", who, gql, mcpText(res))
			}
			return "", false, true
		}
		var views []map[string]any
		views = append(views, jsonObject(rec.Body.Bytes()))
		data, _ := gql["data"].(map[string]any)
		gqlView, _ := data["workspaceBillingEmail"].(map[string]any)
		views = append(views, gqlView, jsonObject(mustJSON(t, res.StructuredContent)))
		for i, v := range views {
			if v["workspaceId"] != ws.ID || v["email"] != views[0]["email"] || v["configurable"] != views[0]["configurable"] {
				t.Fatalf("%s: surface %d answered %v, REST %d %s, GraphQL %v", who, i, v, rec.Code, rec.Body, gql)
			}
		}
		email, _ = views[0]["email"].(string)
		configurable, _ = views[0]["configurable"].(bool)
		return email, configurable, false
	}

	// Before anything is set, the Scale workspace's billing email is its
	// owner's account email, and it may set its own.
	if email, configurable, _ := read("admin", session(admin), scale); email != admin+"@example.com" || !configurable {
		t.Fatalf("the Scale workspace's billing email = %q (configurable %v), want the owner's, configurable", email, configurable)
	}

	// The Billing member sets it over REST, the Admin over GraphQL, then MCP;
	// each reaches the Stripe Customer and reads back for everyone who may
	// view billing.
	writes := []struct {
		who, email string
		write      func(id core.Identity, email string) string // "" on success
	}{
		{"billing", "rest@example.com", func(id core.Identity, email string) string {
			if rec := rest(id, http.MethodPut, path(scale), `{"email":"`+strings.ToUpper(email)+`"}`); rec.Code != http.StatusOK {
				return fmt.Sprintf("%d %s", rec.Code, rec.Body)
			}
			return ""
		}},
		{"admin", "graphql@example.com", func(id core.Identity, email string) string {
			if out := graphqlDo(t, srv, id, fmt.Sprintf(`mutation { setWorkspaceBillingEmail(workspaceId: %q, email: %q) { email } }`, scale.ID, email)); out["errors"] != nil {
				return fmt.Sprint(out)
			}
			return ""
		}},
		{"billing", "mcp@example.com", func(id core.Identity, email string) string {
			if res := mcpCall(t, srv, id, "update_billing_email", map[string]any{"email": email}); res.IsError {
				return mcpText(res)
			}
			return ""
		}},
	}
	for _, w := range writes {
		if failed := w.write(session(members[w.who]), w.email); failed != "" {
			t.Fatalf("the %s sets %s: %s", w.who, w.email, failed)
		}
		if got := stripe.of(scale.ID); got[len(got)-1] != w.email {
			t.Fatalf("the Stripe Customer's email after %s = %v, want %s", w.email, got, w.email)
		}
		for role, subject := range members {
			email, configurable, refused := read(role, session(subject), scale)
			viewsBilling := role == "admin" || role == "developer" || role == "billing"
			if refused == viewsBilling || (viewsBilling && (email != w.email || !configurable)) {
				t.Fatalf("the %s reads %q (configurable %v, refused %v) after %s was set", role, email, configurable, refused, w.email)
			}
		}
	}

	// The Developer views billing details but does not change them.
	developer := session(members["developer"])
	if rec := rest(developer, http.MethodPut, path(scale), `{"email":"dev@example.com"}`); rec.Code != http.StatusForbidden {
		t.Errorf("the Developer's PUT = %d %s, want 403", rec.Code, rec.Body)
	}
	if out := graphqlDo(t, srv, developer, fmt.Sprintf(`mutation { setWorkspaceBillingEmail(workspaceId: %q, email: "dev@example.com") { email } }`, scale.ID)); graphqlCode(out) != "FORBIDDEN" {
		t.Errorf("the Developer's GraphQL write = %v, want FORBIDDEN", out)
	}
	if res := mcpCall(t, srv, developer, "update_billing_email", map[string]any{"email": "dev@example.com"}); !strings.Contains(strings.ToLower(mcpText(res)), "forbidden") {
		t.Errorf("the Developer's MCP write = %s, want forbidden", mcpText(res))
	}
	if email, _, _ := read("admin", session(admin), scale); email != "mcp@example.com" || len(stripe.of(scale.ID)) != 3 {
		t.Fatalf("after the refused writes = %q, Stripe %v; want mcp@example.com and three Stripe writes", email, stripe.of(scale.ID))
	}

	// Hobby: the owner's account email, and every surface refuses the write
	// with the same coded refusal.
	owner := session(hobbyAdmin)
	if email, configurable, _ := read("hobby admin", owner, hobby); email != hobbyAdmin+"@example.com" || configurable {
		t.Fatalf("the Hobby workspace's billing email = %q (configurable %v), want the owner's, not configurable", email, configurable)
	}
	rec := rest(owner, http.MethodPut, path(hobby), `{"email":"finance@example.com"}`)
	var refusal struct {
		Code   string         `json:"code"`
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &refusal); rec.Code != http.StatusBadRequest || err != nil || refusal.Code != core.PlanFeatureRequiredCode ||
		refusal.Params["feature"] != "billingEmail" || refusal.Params["requiredPlan"] != "pro" || refusal.Params["plan"] != "hobby" {
		t.Errorf("the Hobby REST write = %d %s, want 400 PLAN_FEATURE_REQUIRED billingEmail/pro", rec.Code, rec.Body)
	}
	out := graphqlDo(t, srv, owner, fmt.Sprintf(`mutation { setWorkspaceBillingEmail(workspaceId: %q, email: "finance@example.com") { email } }`, hobby.ID))
	if graphqlCode(out) != core.PlanFeatureRequiredCode || !strings.Contains(fmt.Sprint(out["errors"]), "billingEmail") {
		t.Errorf("the Hobby GraphQL write = %v, want PLAN_FEATURE_REQUIRED billingEmail", out)
	}
	if res := mcpCall(t, srv, owner, "update_billing_email", map[string]any{"email": "finance@example.com"}); !res.IsError || !strings.HasPrefix(mcpText(res), core.PlanFeatureRequiredCode) {
		t.Errorf("the Hobby MCP write = %s, want a PLAN_FEATURE_REQUIRED refusal", mcpText(res))
	}
	if _, stored, err := st.WorkspaceBillingEmail(ctx, hobby.ID); err != nil || stored != "" || len(stripe.of(hobby.ID)) != 0 {
		t.Fatalf("the refused Hobby writes changed stored %q (%v), Stripe %v", stored, err, stripe.of(hobby.ID))
	}

	// Upgraded to Pro, the same workspace sets an address of its own; moved
	// back to Hobby, the address is cleared and the Customer gets the owner's
	// account email back.
	if _, err := st.UpdateTenantPlan(ctx, hobby.ID, store.PlanPro); err != nil {
		t.Fatal(err)
	}
	if out := graphqlDo(t, srv, owner, fmt.Sprintf(`mutation { setWorkspaceBillingEmail(workspaceId: %q, email: "finance@example.com") { email } }`, hobby.ID)); out["errors"] != nil {
		t.Fatalf("the Pro write: %v", out)
	}
	if out := graphqlDo(t, srv, owner, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "hobby") { plan } }`, hobby.ID)); out["errors"] != nil {
		t.Fatalf("the downgrade to Hobby: %v", out)
	}
	if _, stored, err := st.WorkspaceBillingEmail(ctx, hobby.ID); err != nil || stored != "" {
		t.Fatalf("after the downgrade stored %q, %v; want cleared", stored, err)
	}
	if got := stripe.of(hobby.ID); !slices.Equal(got, []string{"finance@example.com", hobbyAdmin + "@example.com"}) {
		t.Fatalf("the Stripe Customer's emails = %v, want the Pro address then the owner's", got)
	}
	if email, configurable, _ := read("hobby admin", owner, hobby); email != hobbyAdmin+"@example.com" || configurable {
		t.Fatalf("after the downgrade = %q (configurable %v), want the owner's, not configurable", email, configurable)
	}
}

// TestNewCustomersTakeTheBillingEmailE2E is w6/179's proof against REAL
// Postgres, with the real Stripe client over a stand-in and the billing email
// read composed as cmd/api composes it (workspaces.BillingEmailsInEffect). The
// Customer a workspace's first Checkout
// will create, and the one EnsureCustomer creates for its usage, take its
// billing email in effect: on Hobby its owner's account email, whatever
// address is stored; on Pro the address it set. A workspace whose owner's
// email cannot be read still opens Checkout, which then asks for one.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	  go test ./internal/api -run TestNewCustomersTakeTheBillingEmailE2E -v
func TestNewCustomersTakeTheBillingEmailE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	if dbURI == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
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

	run := uniqueRunSuffix()
	workspace := func(name, plan, owner, stored string) store.Tenant {
		t.Helper()
		ws, err := st.CreateWorkspace(ctx, name+"-"+run, plan, owner)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
		if err := st.SetWorkspaceBillingEmail(ctx, ws.ID, stored); err != nil {
			t.Fatal(err)
		}
		return ws
	}
	hobbyOwner := "hobbyowner-" + run
	hobby := workspace("newcustomerhobby", store.PlanHobby, hobbyOwner, "stale@example.com")
	pro := workspace("newcustomerpro", store.PlanPro, "proowner-"+run, "finance@example.com")
	// Its owner has no identity the provider can read, and it stored none.
	unknown := workspace("newcustomerunknown", store.PlanPro, "unknownowner-"+run, "")

	var mu sync.Mutex
	checkouts, creates := map[string]url.Values{}, map[string]url.Values{}
	stripeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/customers/search":
			fmt.Fprint(w, `{"object":"search_result","data":[],"has_more":false,"url":"/v1/customers/search"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/checkout/sessions":
			checkouts[r.PostForm.Get("client_reference_id")] = r.PostForm
			fmt.Fprint(w, `{"id":"cs_test_1","object":"checkout.session","mode":"setup","livemode":false,"expires_at":1785200000,"url":"https://checkout.stripe.com/c/pay/cs_test_1"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/customers":
			creates[r.PostForm.Get("metadata[bex_workspace]")] = r.PostForm
			fmt.Fprint(w, `{"id":"cus_`+r.PostForm.Get("metadata[bex_workspace]")+`","object":"customer"}`)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":{"type":"api_error","message":"unexpected `+r.Method+" "+r.URL.Path+`"}}`)
		}
	}))
	defer stripeAPI.Close()
	stripeClient := billing.NewStripe(billing.StripeConfig{
		SecretKey: "sk_test_x", HTTPClient: stripeAPI.Client(), BaseURL: stripeAPI.URL, MaxNetworkRetries: stripe.Int64(0),
		DashboardURL: "https://dashboard.bex.co",
		BillingEmail: workspaces.BillingEmailsInEffect(st, ownerIdentities{unreadable: "unknownowner-" + run}),
	})

	for _, tc := range []struct {
		name string
		ws   store.Tenant
		want string // "" when none is sent
	}{
		{"Hobby: its owner's account email", hobby, hobbyOwner + "@example.com"},
		{"Pro: the address it set", pro, "finance@example.com"},
		{"an owner whose email cannot be read", unknown, ""},
	} {
		if _, err := stripeClient.CreateCheckoutSession(ctx, tc.ws.ID, billing.CheckoutRequest{
			SuccessURL: "https://dashboard.bex.co/billing?billing=success", CancelURL: "https://dashboard.bex.co/billing?billing=cancelled",
		}); err != nil {
			t.Fatalf("%s: Checkout: %v", tc.name, err)
		}
		if err := stripeClient.EnsureCustomer(ctx, tc.ws.ID); err != nil {
			t.Fatalf("%s: EnsureCustomer: %v", tc.name, err)
		}
		mu.Lock()
		checkout, created := checkouts[tc.ws.ID], creates[tc.ws.ID]
		mu.Unlock()
		if checkout.Get("customer_creation") != "always" || checkout.Has("customer_email") != (tc.want != "") || checkout.Get("customer_email") != tc.want {
			t.Errorf("%s: Checkout = %v, want customer_email %q", tc.name, checkout, tc.want)
		}
		if created.Has("email") != (tc.want != "") || created.Get("email") != tc.want {
			t.Errorf("%s: the created Customer = %v, want email %q", tc.name, created, tc.want)
		}
	}
}

// ownerIdentities is metadataIdentities (<subject>@example.com) for every
// subject but unreadable, which the identity provider cannot read.
type ownerIdentities struct{ unreadable string }

func (o ownerIdentities) Lookup(ctx context.Context, subject string) (workspaces.IdentityAttrs, bool) {
	if subject == o.unreadable {
		return workspaces.IdentityAttrs{}, false
	}
	return metadataIdentities{}.Lookup(ctx, subject)
}

func (o ownerIdentities) LookupMany(ctx context.Context, subjects []string) map[string]workspaces.IdentityAttrs {
	out := metadataIdentities{}.LookupMany(ctx, subjects)
	delete(out, o.unreadable)
	return out
}

func (o ownerIdentities) LookupEmails(ctx context.Context, subjects []string) (map[string]string, error) {
	out, err := metadataIdentities{}.LookupEmails(ctx, subjects)
	delete(out, o.unreadable)
	return out, err
}

func (o ownerIdentities) MFAEnrolled(ctx context.Context, id string) (bool, bool, error) {
	return metadataIdentities{}.MFAEnrolled(ctx, id)
}
