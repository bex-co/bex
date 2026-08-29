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

package billing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type billingTenant struct{ id string }

func (b billingTenant) Tenant(context.Context, core.Identity) (string, bool) { return b.id, true }
func (b billingTenant) IsMember(_ context.Context, _ core.Identity, id string) (bool, error) {
	return id == b.id, nil
}

type billingProviderFake struct {
	workspace string
	status    Readiness
	checkout  HostedSession
	portal    HostedSession
	err       error
}

func (f *billingProviderFake) Readiness(_ context.Context, workspaceID string) (Readiness, error) {
	f.workspace = workspaceID
	return f.status, f.err
}
func (f *billingProviderFake) CreateCheckoutSession(_ context.Context, workspaceID string, _ CheckoutRequest) (HostedSession, error) {
	f.workspace = workspaceID
	return f.checkout, f.err
}
func (f *billingProviderFake) CreatePortalSession(_ context.Context, workspaceID string, _ PortalRequest) (HostedSession, error) {
	f.workspace = workspaceID
	return f.portal, f.err
}

type billingDeny struct{}

func (billingDeny) Check(context.Context, string, string, string) (bool, error) { return false, nil }

func billingTestService(provider HostedProvider) *Service {
	return &Service{
		Base:     &core.Base{Workspace: billingTenant{id: "tea-a"}},
		Provider: provider,
	}
}

func billingIdentity(ctx context.Context) context.Context {
	return core.WithIdentity(ctx, core.Identity{Subject: "user:admin"})
}

func TestBillingStatusRESTGraphQLMCPParity(t *testing.T) {
	want := Readiness{
		Mode:               "test",
		CustomerReady:      true,
		SubscriptionReady:  true,
		PaymentMethodReady: true,
		Tax: TaxReadiness{
			Configured: true, Enabled: true, ProductTaxCode: "txcd_confirmed", TaxBehavior: "exclusive", RegistrationCount: 1,
		},
	}
	provider := &billingProviderFake{status: want}
	svc := billingTestService(provider)
	ctx := billingIdentity(context.Background())

	// REST.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/tea-a/billing", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("REST status=%d body=%s", w.Code, w.Body.String())
	}
	var rest Readiness
	if err := json.NewDecoder(w.Body).Decode(&rest); err != nil {
		t.Fatal(err)
	}

	// GraphQL.
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{
		Schema: schema,
		RequestString: `{ workspaceBillingReadiness(workspaceId:"tea-a") {
			workspaceId mode customerReady subscriptionReady paymentMethodReady paymentMethodBrand paymentMethodLast4 paymentMethodRequired paymentMethodOnboardingRequired
			tax { configured enabled reason productTaxCode taxBehavior registrationCount }
			lifecycle { status reason graceDeadline enforcementOwned recoveryPending allowedActions updatedAt }
		} }`,
		Context: ctx,
	})
	if len(gql.Errors) != 0 {
		t.Fatalf("GraphQL errors: %v", gql.Errors)
	}
	gqlJSON, _ := json.Marshal(gql.Data.(map[string]any)["workspaceBillingReadiness"])
	var graph Readiness
	if err := json.Unmarshal(gqlJSON, &graph); err != nil {
		t.Fatal(err)
	}

	// MCP.
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "billing-test", Version: "0"}, nil)
	svc.RegisterMCP(mcpServer)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpCtx := core.WithWorkspace(ctx, "tea-a")
	if _, err := mcpServer.Connect(mcpCtx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "billing-client", Version: "0"}, nil).Connect(mcpCtx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	result, err := client.CallTool(mcpCtx, &mcp.CallToolParams{Name: "get_billing_readiness"})
	if err != nil || result.IsError {
		t.Fatalf("MCP result=%+v err=%v", result, err)
	}
	mcpJSON, _ := json.Marshal(result.StructuredContent)
	var agent Readiness
	if err := json.Unmarshal(mcpJSON, &agent); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(rest, graph) || !reflect.DeepEqual(rest, agent) {
		t.Fatalf("surface drift:\nREST  %+v\nGraph %+v\nMCP   %+v", rest, graph, agent)
	}
	if rest.WorkspaceID != "tea-a" || provider.workspace != "tea-a" {
		t.Fatalf("workspace scoping rest=%q provider=%q", rest.WorkspaceID, provider.workspace)
	}
	if rest.PaymentMethodRequired {
		t.Fatal("paymentMethodRequired must be false when the paid-intent gate is unwired")
	}
}

type readyPaymentGate struct{}

func (readyPaymentGate) RequirePaymentMethod(context.Context, string) error { return nil }

func TestBillingStatusReportsPaymentMethodRequiredWhenGateWired(t *testing.T) {
	svc := billingTestService(&billingProviderFake{status: Readiness{Mode: "test"}})
	svc.Payment = readyPaymentGate{}
	got, err := svc.Status(billingIdentity(context.Background()), "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if !got.PaymentMethodRequired {
		t.Fatal("expected paymentMethodRequired when core.Payment is set")
	}
}

type refusingPaymentGate struct{ calls int }

func (g *refusingPaymentGate) RequirePaymentMethod(context.Context, string) error {
	g.calls++
	return core.NewPaymentRequiredError()
}

type failingPaymentGate struct{}

func (failingPaymentGate) RequirePaymentMethod(context.Context, string) error {
	return errors.New("marker read failed")
}

// The sign-up wall's input mirrors the create gate exactly: it is true only in
// `all` mode AND only when the very gate a create consults refuses the
// workspace. Paid-intent-only mode never walls (the free tier is usable
// card-less), a bound/excluded/comped workspace never walls, and a marker read
// failure degrades the whole readiness read the way every other store failure
// in Status does — never a silent false that would let an unbound workspace
// through the wall.
func TestBillingStatusOnboardingRequiredMirrorsAllModeGate(t *testing.T) {
	cases := []struct {
		name     string
		gate     core.PaymentGate
		allPlans bool
		want     bool
		wantErr  bool
	}{
		{name: "gate unwired", gate: nil, allPlans: true, want: false},
		{name: "paid-intent-only mode never walls", gate: &refusingPaymentGate{}, allPlans: false, want: false},
		{name: "all mode, workspace refused", gate: &refusingPaymentGate{}, allPlans: true, want: true},
		{name: "all mode, workspace allowed", gate: readyPaymentGate{}, allPlans: true, want: false},
		{name: "all mode, marker read failed", gate: failingPaymentGate{}, allPlans: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := billingTestService(&billingProviderFake{status: Readiness{Mode: "test"}})
			svc.Payment = tc.gate
			svc.PaymentAllPlans = tc.allPlans
			got, err := svc.Status(billingIdentity(context.Background()), "tea-a")
			if tc.wantErr {
				if !errors.Is(err, core.ErrBillingUnavailable) {
					t.Fatalf("err=%v, want ErrBillingUnavailable", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.PaymentMethodOnboardingRequired != tc.want {
				t.Fatalf("paymentMethodOnboardingRequired=%v, want %v", got.PaymentMethodOnboardingRequired, tc.want)
			}
			if gate, ok := tc.gate.(*refusingPaymentGate); ok && tc.allPlans && gate.calls != 1 {
				t.Fatalf("gate consulted %d times, want exactly once", gate.calls)
			}
		})
	}
}

func TestBillingRESTCreatesOnlyHostedURLs(t *testing.T) {
	provider := &billingProviderFake{
		checkout: HostedSession{URL: "https://checkout.stripe.com/test", ExpiresAt: "2026-07-27T17:00:00Z"},
		portal:   HostedSession{URL: "https://billing.stripe.com/test"},
	}
	svc := billingTestService(provider)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	ctx := billingIdentity(context.Background())
	cases := []struct {
		path string
		body string
	}{
		{"/v1/workspaces/tea-a/billing/checkout-session", `{"successUrl":"https://dashboard.bex.co/usage","cancelUrl":"https://dashboard.bex.co/usage"}`},
		{"/v1/workspaces/tea-a/billing/portal-session", `{"returnUrl":"https://dashboard.bex.co/usage"}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)).WithContext(ctx)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "https://") || strings.Contains(w.Body.String(), "_test_") {
			t.Errorf("POST %s status=%d body=%s", tc.path, w.Code, w.Body.String())
		}
	}
}

func TestBillingAdminAuthorizationAndDegradedProvider(t *testing.T) {
	provider := &billingProviderFake{}
	svc := billingTestService(provider)
	svc.Authz = billingDeny{}
	_, err := svc.Status(billingIdentity(context.Background()), "tea-a")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("denied Status error=%v, want forbidden", err)
	}
	if provider.workspace != "" {
		t.Fatalf("provider called after denial for %q", provider.workspace)
	}

	svc = billingTestService(&billingProviderFake{err: errors.New("Stripe down")})
	_, err = svc.Status(billingIdentity(context.Background()), "tea-a")
	if !errors.Is(err, core.ErrBillingUnavailable) {
		t.Fatalf("provider failure error=%v, want billing unavailable", err)
	}
}

// billingRecordChecker is a core.Checker that records every relation asked for
// and answers from a fixed grant set — the seam for pinning WHICH relation each
// billing verb gates on. It also answers the uncached FreshChecker path from
// the same grant set (recording separately). Write relations now enter through
// the fresh path centrally; Checkout/Portal retain their sink-adjacent second
// check from round 8.
type billingRecordChecker struct {
	grant          map[string]bool
	relations      []string
	freshRelations []string
}

func (c *billingRecordChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	c.relations = append(c.relations, relation)
	return c.grant[relation], nil
}

func (c *billingRecordChecker) CheckFresh(_ context.Context, _, relation, _ string) (bool, error) {
	c.freshRelations = append(c.freshRelations, relation)
	return c.grant[relation], nil
}

// TestBillingVerbsGateOnCanManageBilling pins the w1/m60 flip: every billing
// verb (Status/Checkout/Portal) must authorize on exactly can_manage_billing —
// NOT the admin-only can_manage it replaced. Reverting billing.authorize() to
// core.RelCanManage fails this (the "granted can_manage_billing" leg denies, and
// the "only can_manage" leg wrongly allows), so the dead-relation regression
// (w7/014) cannot silently return.
func TestBillingVerbsGateOnCanManageBilling(t *testing.T) {
	verbs := []struct {
		name string
		call func(*Service, context.Context) error
	}{
		{"Status", func(s *Service, ctx context.Context) error { _, err := s.Status(ctx, "tea-a"); return err }},
		{"Checkout", func(s *Service, ctx context.Context) error {
			_, err := s.Checkout(ctx, "tea-a", CheckoutRequest{})
			return err
		}},
		{"Portal", func(s *Service, ctx context.Context) error {
			_, err := s.Portal(ctx, "tea-a", PortalRequest{})
			return err
		}},
	}
	for _, v := range verbs {
		t.Run(v.name, func(t *testing.T) {
			// Granting ONLY can_manage_billing must allow, and it must be the sole
			// relation the verb checks (one or more fresh assertions are fine).
			rec := &billingRecordChecker{grant: map[string]bool{core.RelCanManageBilling: true}}
			svc := billingTestService(&billingProviderFake{})
			svc.Authz = rec
			if err := v.call(svc, billingIdentity(context.Background())); err != nil {
				t.Fatalf("%s with can_manage_billing granted: %v — want allowed", v.name, err)
			}
			if len(rec.relations) != 0 || len(rec.freshRelations) == 0 {
				t.Fatalf("%s cached=%v fresh=%v, want fresh %s checks only", v.name, rec.relations, rec.freshRelations, core.RelCanManageBilling)
			}
			for _, relation := range rec.freshRelations {
				if relation != core.RelCanManageBilling {
					t.Fatalf("%s checked relation %q, want only %s", v.name, relation, core.RelCanManageBilling)
				}
			}

			// Granting the admin-only can_manage but NOT can_manage_billing must DENY —
			// proving the verb no longer gates on can_manage (the w7/014 fix).
			recAdmin := &billingRecordChecker{grant: map[string]bool{core.RelCanManage: true}}
			svcAdmin := billingTestService(&billingProviderFake{})
			svcAdmin.Authz = recAdmin
			if err := v.call(svcAdmin, billingIdentity(context.Background())); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("%s with only can_manage granted: %v — want ErrForbidden (must NOT gate on admin-only can_manage)", v.name, err)
			}
		})
	}
}
