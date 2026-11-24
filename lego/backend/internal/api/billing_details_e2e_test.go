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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/bex-co/bex/lego/backend/internal/usage"
)

// billing_details_e2e_test.go is w9/m620's proof against REAL Postgres and
// REAL OpenFGA, through the composed REST, GraphQL and MCP surfaces. Render's
// role table (render.com/docs/team-members, read 2026-11-23) gives "View
// billing details" to Admin, Developer and Billing and "Edit payment method"
// to Admin and Billing only. So in a Scale workspace an Admin, a Developer and
// a Billing member, and an API key each of them minted, read the usage with
// its estimate and real cost, the billing readiness and the pipeline spend
// limit; a Contributor and a Viewer, and their keys, are refused each one
// with the same 403; and the Developer still cannot open Checkout or the
// Portal or change the spend limit, which leaves the limit unchanged.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestBillingDetailsPerRoleE2E -v
func TestBillingDetailsPerRoleE2E(t *testing.T) {
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
	admin := "admin-" + run
	// Contributor, Viewer and Billing are Scale roles (store.LimitsFor).
	ws, err := st.CreateWorkspace(ctx, "billingdetails-"+run, store.PlanScale, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+admin, "admin"); err != nil {
		t.Fatal(err)
	}
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

	keys := &creatorKeys{run: run, keys: map[string]apikeys.APIKey{}}
	resolver := tenancy.NewTenantService(st, roles)
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz: checker, Workspace: resolver, KeyCreators: keys, KeyGrants: resolver,
	}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: roles,
		APIKeys: keys, KeyBinder: resolver,
		Billing: e2eBillingProvider{},
		Usage:   &usage.Service{Base: base, Store: st, Billing: e2eInvoiceReader{}},
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

	// The Admin sets a spend limit, which the refused writes below must leave.
	if out := graphqlDo(t, srv, session(admin), fmt.Sprintf(`mutation { setPipelineSpendLimit(workspaceId: %q, spendLimitUsd: "25") { spendLimitUsd } }`, ws.ID)); out["errors"] != nil {
		t.Fatalf("the Admin sets the spend limit: %v", out)
	}

	// Every member mints an All key (any role may, w9/m514); each acts with
	// its creator's role (w9/m481).
	callers := map[string]core.Identity{}
	for role, subject := range members {
		callers[role] = session(subject)
		rec := rest(session(subject), http.MethodPost, "/v1/api-keys", `{"name":"`+role+`-key","ownerId":"`+ws.ID+`"}`)
		var key apikeys.APIKey
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &key) != nil || !key.Permissions.All {
			t.Fatalf("the %s mints an All key: %d %s", role, rec.Code, rec.Body)
		}
		callers[role+"'s key"] = core.Identity{Subject: key.ID, ClientID: key.ID, Method: core.MethodOAuth2}
	}

	billingPath := "/v1/workspaces/" + ws.ID + "/billing"
	for who, id := range callers {
		role, _, _ := strings.Cut(who, "'")
		views := role == "admin" || role == "developer" || role == "billing"
		manages := role == "admin" || role == "billing"

		// REST: the usage with its real cost, and the billing readiness.
		usageRec := rest(id, http.MethodGet, "/v1/usage?ownerId="+ws.ID, "")
		readiness := rest(id, http.MethodGet, billingPath, "")
		// GraphQL: the same two reads, and the pipeline spend limit.
		gqlUsage := graphqlDo(t, srv, id, fmt.Sprintf(`{ usage(ownerId: %q) { estimatedCost { totalUsd } billing { currentCost { amountUsd } } } }`, ws.ID))
		gqlReadiness := graphqlDo(t, srv, id, fmt.Sprintf(`{ workspaceBillingReadiness(workspaceId: %q) { mode } }`, ws.ID))
		gqlPipeline := graphqlDo(t, srv, id, fmt.Sprintf(`{ pipelineSettings(workspaceId: %q) { spendLimitUsd } }`, ws.ID))
		// MCP: get_usage and get_billing_readiness.
		mcpUsage := mcpCall(t, srv, id, "get_usage", map[string]any{})
		mcpReadiness := mcpCall(t, srv, id, "get_billing_readiness", map[string]any{})

		if views {
			if usageRec.Code != http.StatusOK || !strings.Contains(usageRec.Body.String(), `"amountUsd":"12.34"`) {
				t.Errorf("%s's GET /v1/usage = %d %s, want 200 with the real cost", who, usageRec.Code, usageRec.Body)
			}
			if readiness.Code != http.StatusOK {
				t.Errorf("%s's GET %s = %d %s, want 200", who, billingPath, readiness.Code, readiness.Body)
			}
			if gqlUsage["errors"] != nil || !strings.Contains(fmt.Sprint(gqlUsage["data"]), "12.34") {
				t.Errorf("%s's GraphQL usage = %v, want the real cost", who, gqlUsage)
			}
			for name, out := range map[string]map[string]any{"workspaceBillingReadiness": gqlReadiness, "pipelineSettings": gqlPipeline} {
				if out["errors"] != nil {
					t.Errorf("%s's GraphQL %s = %v, want data", who, name, out)
				}
			}
			if !strings.Contains(fmt.Sprint(gqlPipeline["data"]), "25.00") {
				t.Errorf("%s's GraphQL pipelineSettings = %v, want the 25.00 limit", who, gqlPipeline)
			}
			if mcpUsage.IsError || !strings.Contains(mcpText(mcpUsage), "12.34") {
				t.Errorf("%s's MCP get_usage = %s, want the real cost", who, mcpText(mcpUsage))
			}
			if mcpReadiness.IsError {
				t.Errorf("%s's MCP get_billing_readiness = %s", who, mcpText(mcpReadiness))
			}
		} else {
			for name, rec := range map[string]*httptest.ResponseRecorder{"GET /v1/usage": usageRec, "GET " + billingPath: readiness} {
				if rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), "12.34") {
					t.Errorf("%s's %s = %d %s, want 403 and no figure", who, name, rec.Code, rec.Body)
				}
			}
			for name, out := range map[string]map[string]any{"usage": gqlUsage, "workspaceBillingReadiness": gqlReadiness, "pipelineSettings": gqlPipeline} {
				if graphqlCode(out) != "FORBIDDEN" || strings.Contains(fmt.Sprint(out["data"]), "12.34") {
					t.Errorf("%s's GraphQL %s = %v, want FORBIDDEN and no figure", who, name, out)
				}
			}
			for name, res := range map[string]string{"get_usage": mcpText(mcpUsage), "get_billing_readiness": mcpText(mcpReadiness)} {
				if !strings.Contains(strings.ToLower(res), "forbidden") || strings.Contains(res, "12.34") {
					t.Errorf("%s's MCP %s = %s, want forbidden and no figure", who, name, res)
				}
			}
		}

		// The capability read says the same, so the dashboard gates on it.
		caps := graphqlDo(t, srv, id, fmt.Sprintf(`{ viewerCapabilities(ownerId: %q) { canViewBilling canManageBilling } }`, ws.ID))
		got, _ := caps["data"].(map[string]any)["viewerCapabilities"].(map[string]any)
		if got["canViewBilling"] != views || got["canManageBilling"] != manages {
			t.Errorf("%s's viewerCapabilities = %v, want canViewBilling %v and canManageBilling %v", who, caps, views, manages)
		}

		// Editing stays Admin and Billing: Checkout, the Portal and the spend
		// limit refuse everyone else, the Developer included.
		if manages {
			continue
		}
		for name, rec := range map[string]*httptest.ResponseRecorder{
			"checkout-session": rest(id, http.MethodPost, billingPath+"/checkout-session", `{"successUrl":"https://dashboard.bex.co/billing","cancelUrl":"https://dashboard.bex.co/billing"}`),
			"portal-session":   rest(id, http.MethodPost, billingPath+"/portal-session", `{"returnUrl":"https://dashboard.bex.co/billing"}`),
		} {
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s's POST %s = %d %s, want 403", who, name, rec.Code, rec.Body)
			}
		}
		if out := graphqlDo(t, srv, id, fmt.Sprintf(`mutation { setPipelineSpendLimit(workspaceId: %q, spendLimitUsd: "1") { spendLimitUsd } }`, ws.ID)); graphqlCode(out) != "FORBIDDEN" {
			t.Errorf("%s's setPipelineSpendLimit = %v, want FORBIDDEN", who, out)
		}
		if res := mcpCall(t, srv, id, "create_billing_portal_session", map[string]any{"returnUrl": "https://dashboard.bex.co/billing"}); !strings.Contains(strings.ToLower(mcpText(res)), "forbidden") {
			t.Errorf("%s's MCP create_billing_portal_session = %s, want forbidden", who, mcpText(res))
		}
	}

	// The refused spend-limit changes wrote nothing.
	if out := graphqlDo(t, srv, session(admin), fmt.Sprintf(`{ pipelineSettings(workspaceId: %q) { spendLimitUsd } }`, ws.ID)); !strings.Contains(fmt.Sprint(out["data"]), "25.00") {
		t.Errorf("the spend limit after the refused changes = %v, want 25.00", out)
	}
}

// e2eInvoiceReader is the Stripe projection's stand-in: every workspace has a
// current cost of 12.34, so a response that carries it carried real billing.
type e2eInvoiceReader struct{}

func (e2eInvoiceReader) BillingFor(context.Context, string, time.Time, time.Time) (*billing.Billing, error) {
	return &billing.Billing{CurrentCost: &billing.Amount{AmountUSD: "12.34", Currency: "usd"}, Invoices: []billing.Invoice{}}, nil
}
