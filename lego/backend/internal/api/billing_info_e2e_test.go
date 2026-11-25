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
	"maps"
	"net/http"
	"net/http/httptest"
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
)

// fakeStripeCustomers is a Stripe stand-in holding Customers and their tax
// IDs, as much of the API as billing info and a workspace rename use.
type fakeStripeCustomers struct {
	mu        sync.Mutex
	customers map[string]*fakeCustomer // by Customer id
	nextTaxID int
}

type fakeCustomer struct {
	id, workspace, name string
	metadata            map[string]string
	address             map[string]string
	taxIDs              []map[string]string // id, type, value; newest first
}

func (f *fakeStripeCustomers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_ = r.ParseForm()
	reply := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/customers"), "/")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/customers/search":
		var data []any
		for _, c := range f.customers {
			if strings.Contains(r.Form.Get("query"), "'"+c.workspace+"'") {
				data = append(data, map[string]any{"id": c.id, "object": "customer"})
			}
		}
		reply(200, map[string]any{"object": "search_result", "data": data, "has_more": false, "url": "/v1/customers/search"})
		return
	case len(parts) < 2 || f.customers[parts[1]] == nil:
		reply(404, map[string]any{"error": map[string]any{"type": "invalid_request_error", "code": "resource_missing", "message": "No such customer"}})
		return
	}
	c := f.customers[parts[1]]
	switch {
	case r.Method == http.MethodGet && len(parts) == 2:
		reply(200, c.json())
	case r.Method == http.MethodPost && len(parts) == 2:
		if r.Form.Has("name") {
			c.name = r.Form.Get("name")
		}
		for key, values := range r.Form {
			if field, ok := strings.CutPrefix(key, "metadata["); ok {
				field = strings.TrimSuffix(field, "]")
				if values[0] == "" {
					delete(c.metadata, field)
				} else {
					c.metadata[field] = values[0]
				}
			}
			if field, ok := strings.CutPrefix(key, "address["); ok {
				c.address[strings.TrimSuffix(field, "]")] = values[0]
			}
		}
		if r.Form.Has("address") && r.Form.Get("address") == "" {
			c.address = map[string]string{}
		}
		reply(200, c.json())
	case r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "tax_ids":
		if strings.HasPrefix(r.Form.Get("value"), "INVALID") {
			reply(400, map[string]any{"error": map[string]any{"type": "invalid_request_error", "code": "tax_id_invalid", "param": "value", "message": "Invalid value for " + r.Form.Get("type") + "."}})
			return
		}
		f.nextTaxID++
		t := map[string]string{"id": fmt.Sprintf("txi_%d", f.nextTaxID), "object": "tax_id", "type": r.Form.Get("type"), "value": strings.ToUpper(r.Form.Get("value"))}
		c.taxIDs = append([]map[string]string{t}, c.taxIDs...)
		reply(200, t)
	case r.Method == http.MethodDelete && len(parts) == 4 && parts[2] == "tax_ids":
		for i, t := range c.taxIDs {
			if t["id"] == parts[3] {
				c.taxIDs = append(c.taxIDs[:i], c.taxIDs[i+1:]...)
			}
		}
		reply(200, map[string]any{"id": parts[3], "object": "tax_id", "deleted": true})
	default:
		reply(500, map[string]any{"error": map[string]any{"type": "api_error", "message": "unexpected " + r.Method + " " + r.URL.Path}})
	}
}

func (c *fakeCustomer) json() map[string]any {
	meta := map[string]string{"bex_workspace": c.workspace}
	for k, v := range c.metadata {
		meta[k] = v
	}
	taxIDs := make([]any, 0, len(c.taxIDs))
	for _, t := range c.taxIDs {
		taxIDs = append(taxIDs, t)
	}
	out := map[string]any{
		"id": c.id, "object": "customer", "name": c.name, "metadata": meta,
		"tax_ids": map[string]any{"object": "list", "data": taxIDs, "has_more": false},
	}
	if len(c.address) > 0 {
		out["address"] = c.address
	}
	return out
}

func (f *fakeStripeCustomers) customer(id string) fakeCustomer {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := *f.customers[id]
	return c
}

// billing_info_e2e_test.go is w6/m628's proof against REAL Postgres and REAL
// OpenFGA through the composed REST, GraphQL and MCP surfaces and the real
// Stripe client (over a Stripe stand-in). A workspace whose Customer still
// bills its tea- id is renamed, and the Customer follows; the Billing member,
// then the Admin, then the Billing member again set its billing info on each
// surface, and Admin, Developer and Billing read the same back on each,
// while a Contributor and a Viewer read none and the Developer writes none.
// A company name survives the next rename; a tax ID Stripe refuses names its
// field on every surface; a workspace with no Customer reads not ready and
// is refused the write.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestBillingInfoOnEverySurfaceE2E -v
func TestBillingInfoOnEverySurfaceE2E(t *testing.T) {
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
	workspace := func(name, admin string) store.Tenant {
		t.Helper()
		ws, err := st.CreateWorkspace(ctx, name+"-"+run, store.PlanPro, admin)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+admin, "admin"); err != nil {
			t.Fatal(err)
		}
		return ws
	}
	admin, bareAdmin := "admin-"+run, "bareadmin-"+run
	ws := workspace("billinginfo", admin)
	bare := workspace("billinginfobare", bareAdmin)
	members := map[string]string{"admin": admin}
	for _, role := range []string{"developer", "contributor", "viewer", "billing"} {
		subject := role + "-" + run
		if err := st.AddMember(ctx, subject, ws.ID, role); err != nil {
			t.Fatal(err)
		}
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+subject, role); err != nil {
			t.Fatal(err)
		}
		members[role] = subject
	}

	// The workspace's Customer was made before w6/m628, named by its id.
	fake := &fakeStripeCustomers{customers: map[string]*fakeCustomer{
		"cus_ws": {id: "cus_ws", workspace: ws.ID, name: ws.ID, metadata: map[string]string{}, address: map[string]string{}},
	}}
	stripeAPI := httptest.NewServer(fake)
	defer stripeAPI.Close()
	stripeClient := billing.NewStripe(billing.StripeConfig{
		SecretKey: "sk_test_x", HTTPClient: stripeAPI.Client(), BaseURL: stripeAPI.URL, MaxNetworkRetries: stripe.Int64(0),
		WorkspaceLabel: func(ctx context.Context, id string) (string, error) {
			t, err := st.GetTenant(ctx, id)
			return t.Label(), err
		},
	})

	resolver := tenancy.NewTenantService(st, roles)
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: checker, Workspace: resolver, KeyGrants: resolver}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: roles,
		Billing: stripeClient, BillingInfo: stripeClient, WorkspaceCustomerName: stripeClient,
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
	path := func(w store.Tenant) string { return "/v1/workspaces/" + w.ID + "/billing/info" }
	const fields = `workspaceId customerReady billTo companyName address { line1 line2 city state postalCode country } taxId { type value }`
	// read answers the billing info every surface gives id, refusing the test
	// when the surfaces disagree; nil when every surface refuses.
	read := func(who string, id core.Identity, w store.Tenant) map[string]any {
		t.Helper()
		rec := rest(id, http.MethodGet, path(w), "")
		gql := graphqlDo(t, srv, id, fmt.Sprintf(`{ workspaceBillingInfo(workspaceId: %q) { %s } }`, w.ID, fields))
		res := mcpCall(t, srv, id, "get_billing_info", map[string]any{})
		if rec.Code == http.StatusForbidden {
			if graphqlCode(gql) != "FORBIDDEN" || !strings.Contains(strings.ToLower(mcpText(res)), "forbidden") {
				t.Fatalf("%s: REST refused but GraphQL %v, MCP %s", who, gql, mcpText(res))
			}
			return nil
		}
		data, _ := gql["data"].(map[string]any)
		gqlView, _ := data["workspaceBillingInfo"].(map[string]any)
		views := []map[string]any{jsonObject(rec.Body.Bytes()), gqlView, jsonObject(mustJSON(t, res.StructuredContent))}
		want := string(mustJSON(t, views[0]))
		for i, v := range views {
			if v["workspaceId"] != w.ID || string(mustJSON(t, v)) != want {
				t.Fatalf("%s: surface %d answered %s, REST %d %s", who, i, mustJSON(t, v), rec.Code, want)
			}
		}
		return views[0]
	}
	// everyone reads the workspace's billing info as want, or is refused.
	everyone := func(want map[string]any) {
		t.Helper()
		for _, role := range slices.Sorted(maps.Keys(members)) {
			got := read(role, session(members[role]), ws)
			viewsBilling := role == "admin" || role == "developer" || role == "billing"
			if (got != nil) != viewsBilling {
				t.Fatalf("the %s reads %v, want a read %v", role, got, viewsBilling)
			}
			for k, v := range want {
				if got != nil && fmt.Sprint(got[k]) != fmt.Sprint(v) {
					t.Fatalf("the %s reads %s = %v, want %v (%v)", role, k, got[k], v, got)
				}
			}
		}
	}

	// Before anything: the invoices bill the tea- id the Customer was made
	// with, and nothing else is set.
	everyone(map[string]any{"customerReady": true, "billTo": ws.ID, "companyName": "", "address": nil, "taxId": nil})

	// A rename renames the Customer, so the next invoice bills the name.
	if out := graphqlDo(t, srv, session(admin), fmt.Sprintf(`mutation { renameWorkspace(id: %q, name: "Acme Prod") { name } }`, ws.ID)); out["errors"] != nil {
		t.Fatalf("rename: %v", out)
	}
	if got := fake.customer("cus_ws").name; got != "Acme Prod" {
		t.Fatalf("the Customer's name after the rename = %q, want Acme Prod", got)
	}
	everyone(map[string]any{"billTo": "Acme Prod", "companyName": ""})

	// The Billing member over REST, the Admin over GraphQL, the Billing member
	// over MCP; each lands on the Customer and reads back on every surface.
	writes := []struct {
		who   string
		write func(id core.Identity) string // "" on success
		want  map[string]any
	}{
		{"billing", func(id core.Identity) string {
			body := `{"companyName":"Acme GmbH","address":{"line1":"Hauptstr. 1","city":"Berlin","postalCode":"10115","country":"de"},"taxId":{"type":"eu_vat","value":"de123456789"}}`
			if rec := rest(id, http.MethodPut, path(ws), body); rec.Code != http.StatusOK {
				return fmt.Sprintf("%d %s", rec.Code, rec.Body)
			}
			return ""
		}, map[string]any{
			"billTo": "Acme GmbH", "companyName": "Acme GmbH",
			"address": map[string]any{"line1": "Hauptstr. 1", "line2": "", "city": "Berlin", "state": "", "postalCode": "10115", "country": "DE"},
			"taxId":   map[string]any{"type": "eu_vat", "value": "DE123456789"},
		}},
		{"admin", func(id core.Identity) string {
			q := fmt.Sprintf(`mutation { setWorkspaceBillingInfo(workspaceId: %q, companyName: "Acme SAS", address: {line1: "1 rue de Rivoli", city: "Paris", postalCode: "75001", country: "FR"}, taxId: {type: "eu_vat", value: "FR12345678901"}) { billTo } }`, ws.ID)
			if out := graphqlDo(t, srv, id, q); out["errors"] != nil {
				return fmt.Sprint(out)
			}
			return ""
		}, map[string]any{
			"billTo": "Acme SAS", "companyName": "Acme SAS",
			"address": map[string]any{"line1": "1 rue de Rivoli", "line2": "", "city": "Paris", "state": "", "postalCode": "75001", "country": "FR"},
			"taxId":   map[string]any{"type": "eu_vat", "value": "FR12345678901"},
		}},
		{"billing", func(id core.Identity) string {
			args := map[string]any{"companyName": "Acme SAS", "address": map[string]any{"line1": "1 rue de Rivoli", "city": "Paris", "postalCode": "75001", "country": "FR"}}
			if res := mcpCall(t, srv, id, "update_billing_info", args); res.IsError {
				return mcpText(res)
			}
			return ""
		}, map[string]any{"billTo": "Acme SAS", "companyName": "Acme SAS", "taxId": nil}},
	}
	for i, w := range writes {
		if failed := w.write(session(members[w.who])); failed != "" {
			t.Fatalf("write %d by the %s: %s", i, w.who, failed)
		}
		everyone(w.want)
		c := fake.customer("cus_ws")
		if c.name != w.want["billTo"] || c.metadata["bex_company_name"] != "true" || len(c.taxIDs) > 1 {
			t.Fatalf("the Customer after write %d = %+v", i, c)
		}
	}

	// A company name is the member's: the next rename leaves it.
	if out := graphqlDo(t, srv, session(admin), fmt.Sprintf(`mutation { renameWorkspace(id: %q, name: "Acme Global") { name } }`, ws.ID)); out["errors"] != nil {
		t.Fatalf("rename: %v", out)
	}
	if got := fake.customer("cus_ws").name; got != "Acme SAS" {
		t.Fatalf("the Customer's name after a rename = %q, want the company name kept", got)
	}

	// The Developer views billing details but does not change them.
	developer := session(members["developer"])
	if rec := rest(developer, http.MethodPut, path(ws), `{"companyName":"Dev Co"}`); rec.Code != http.StatusForbidden {
		t.Errorf("the Developer's PUT = %d %s, want 403", rec.Code, rec.Body)
	}
	if out := graphqlDo(t, srv, developer, fmt.Sprintf(`mutation { setWorkspaceBillingInfo(workspaceId: %q, companyName: "Dev Co") { billTo } }`, ws.ID)); graphqlCode(out) != "FORBIDDEN" {
		t.Errorf("the Developer's GraphQL write = %v, want FORBIDDEN", out)
	}
	if res := mcpCall(t, srv, developer, "update_billing_info", map[string]any{"companyName": "Dev Co"}); !strings.Contains(strings.ToLower(mcpText(res)), "forbidden") {
		t.Errorf("the Developer's MCP write = %s, want forbidden", mcpText(res))
	}

	// A tax ID Stripe refuses, and a type Stripe does not have, name their
	// field with the same coded refusal on every surface, and change nothing.
	billingMember := session(members["billing"])
	for field, taxID := range map[string]map[string]any{
		"taxId.value": {"type": "eu_vat", "value": "INVALID"},
		"taxId.type":  {"type": "xx_vat", "value": "123"},
	} {
		body, _ := json.Marshal(map[string]any{"companyName": "Acme SAS", "taxId": taxID})
		rec := rest(billingMember, http.MethodPut, path(ws), string(body))
		var refusal struct {
			Code   string         `json:"code"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &refusal); rec.Code != http.StatusBadRequest || err != nil || refusal.Code != billing.BillingInfoInvalidCode || refusal.Params["field"] != field || refusal.Params["reason"] == nil {
			t.Errorf("REST %s refusal = %d %s, want 400 %s field %s", field, rec.Code, rec.Body, billing.BillingInfoInvalidCode, field)
		}
		out := graphqlDo(t, srv, billingMember, fmt.Sprintf(`mutation { setWorkspaceBillingInfo(workspaceId: %q, companyName: "Acme SAS", taxId: {type: %q, value: %q}) { billTo } }`, ws.ID, taxID["type"], taxID["value"]))
		errs, _ := out["errors"].([]any)
		ext := map[string]any{}
		if len(errs) > 0 {
			ext, _ = errs[0].(map[string]any)["extensions"].(map[string]any)
		}
		if graphqlCode(out) != billing.BillingInfoInvalidCode || ext["field"] != field {
			t.Errorf("GraphQL %s refusal = %v", field, out)
		}
		if res := mcpCall(t, srv, billingMember, "update_billing_info", map[string]any{"companyName": "Acme SAS", "taxId": taxID}); !res.IsError || !strings.HasPrefix(mcpText(res), billing.BillingInfoInvalidCode) || !strings.Contains(mcpText(res), field) {
			t.Errorf("MCP %s refusal = %s", field, mcpText(res))
		}
	}
	if c := fake.customer("cus_ws"); c.name != "Acme SAS" || c.address["city"] != "Paris" || len(c.taxIDs) != 0 {
		t.Fatalf("the refused writes changed the Customer: %+v", c)
	}

	// Cleared, the invoices bill the workspace's name again.
	if rec := rest(billingMember, http.MethodPut, path(ws), `{}`); rec.Code != http.StatusOK {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body)
	}
	everyone(map[string]any{"billTo": "Acme Global", "companyName": "", "address": nil, "taxId": nil})
	if c := fake.customer("cus_ws"); c.metadata["bex_company_name"] != "" || len(c.address) != 0 {
		t.Fatalf("the cleared Customer = %+v", c)
	}

	// A workspace with no Customer has no billing info to read or set yet.
	owner := session(bareAdmin)
	if got := read("bare admin", owner, bare); got["customerReady"] != false || got["billTo"] != "" {
		t.Fatalf("the bare workspace reads %v, want not ready", got)
	}
	rec := rest(owner, http.MethodPut, path(bare), `{"companyName":"Acme"}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), billing.BillingCustomerNotReadyCode) {
		t.Errorf("the bare workspace's write = %d %s, want 409 %s", rec.Code, rec.Body, billing.BillingCustomerNotReadyCode)
	}
	if len(fake.customers) != 1 {
		t.Fatalf("Customers = %d, want none created", len(fake.customers))
	}
}
