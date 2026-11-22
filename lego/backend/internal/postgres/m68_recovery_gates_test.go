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

package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/m68 F5 — recovery is a create, and pays a create's price.
//
// Recover provisions a NEW managed Postgres on a caller-selected (or inherited)
// plan. It was written as its own create path and reached Client.Create with
// neither RequirePaymentMethod nor RequireBillingMutation, while CreatePostgres
// and SetPlan enforce both for the identical resource cost — so a developer
// could provision paid capacity in a workspace whose billing state should have
// refused it, and could do so repeatedly.
//
// This is the third entry in what TestPaidIntentGuardCoversPostgresCreateAndBothPlanUpdatePaths
// calls the create-and-plan-update set. It is here rather than folded into that
// test because recovery's preconditions differ (a backed-up source instance).

// refusingBillingGate refuses every mutation, standing in for a workspace whose
// billing state is enforced (unpaid invoice, suspended contract).
type refusingBillingGate struct{ calls []string }

func (g *refusingBillingGate) CheckBillingMutationAllowed(_ context.Context, workspaceID string) error {
	g.calls = append(g.calls, workspaceID)
	return core.ErrBillingEnforced
}

// backedUpSource is a Ready, backed-up Database owned by tea-a — the only shape
// Recover accepts as a source.
func backedUpSource(t *testing.T, plan string) (*Service, *appv1alpha1.Database) {
	t.Helper()
	svc, cl := newServicePostgresResources()
	src := seedDatabaseSpec(t, cl, "dpg-src", appv1alpha1.DatabaseSpec{Name: "orders", Plan: plan}, true)
	// The PITR window is established: these tests pin the billing gates, and
	// since w6/m117 Recover refuses before ever reaching them when the source
	// has no restore point.
	seedBarmanRecoveryWindow(t, cl, "dpg-src", "2026-08-01T00:00:00Z", "2026-08-02T00:00:00Z")
	src.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelWorkspace: "tea-a"}
	if err := cl.Update(context.Background(), src); err != nil {
		t.Fatalf("label source: %v", err)
	}
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	return svc, src
}

func TestRecoveryEnforcesPaymentMethodForPaidPlans(t *testing.T) {
	t.Run("caller-selected paid plan", func(t *testing.T) {
		svc, _ := backedUpSource(t, "free")
		gate := &rejectingPaymentGate{}
		svc.Payment = gate

		_, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored", Plan: "basic-256mb"})
		if !errors.Is(err, core.ErrPaymentRequired) {
			t.Fatalf("paid recovery err=%v, want ErrPaymentRequired", err)
		}
		if len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
			t.Fatalf("payment gate calls=%v, want one for the source's workspace", gate.calls)
		}
	})

	t.Run("plan INHERITED from a paid source", func(t *testing.T) {
		// The inherited case is the one a plan-only reading of the code misses:
		// omitting `plan` does not make the recovery free, it makes it cost
		// whatever the source costs.
		svc, _ := backedUpSource(t, "basic-256mb")
		gate := &rejectingPaymentGate{}
		svc.Payment = gate

		_, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored"})
		if !errors.Is(err, core.ErrPaymentRequired) {
			t.Fatalf("inherited-paid recovery err=%v, want ErrPaymentRequired", err)
		}
	})

	t.Run("free plan does not consult the paid gate", func(t *testing.T) {
		// A free source has no backups, so a free recovery is a paid source's
		// copy on the free plan (the workspace's only free database).
		svc, _ := backedUpSource(t, "basic-256mb")
		gate := &rejectingPaymentGate{}
		svc.Payment = gate

		if _, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored", Plan: "free"}); err != nil {
			t.Fatalf("free recovery => %v, want success", err)
		}
		if len(gate.calls) != 0 {
			t.Fatalf("free recovery consulted the paid gate: %v", gate.calls)
		}
	})
}

func TestRecoveryIsRefusedDuringABillingMutationLock(t *testing.T) {
	// Unlike the payment gate, this one applies on EVERY plan: a locked
	// workspace may not provision at all, free or not.
	for _, plan := range []string{"free", "basic-256mb"} {
		svc, _ := backedUpSource(t, plan)
		gate := &refusingBillingGate{}
		svc.Billing = gate

		_, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored"})
		if !errors.Is(err, core.ErrBillingEnforced) {
			t.Errorf("recovery on %s during a billing lock => %v, want ErrBillingEnforced", plan, err)
		}
		if len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
			t.Errorf("billing gate calls=%v, want one for the source's workspace", gate.calls)
		}
	}
}

func TestPermittedRecoveryStillCreatesTheIntendedInstance(t *testing.T) {
	// The gates must not change what a permitted recovery produces.
	svc, src := backedUpSource(t, "basic-256mb")
	svc.Payment = &switchablePaymentGate{bound: true}

	out, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{
		Name:       "restored",
		TargetTime: "2026-08-10T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("permitted recovery => %v", err)
	}
	if out.Name != "restored" || out.Plan != "basic-256mb" {
		t.Fatalf("view = %+v, want the inherited plan under the new name", out)
	}

	var created appv1alpha1.Database
	if err := svc.Client.Get(context.Background(), clientKey(out.ID), &created); err != nil {
		t.Fatalf("get recovered db: %v", err)
	}
	if created.Spec.Recovery == nil ||
		created.Spec.Recovery.SourceDatabase != src.Name ||
		created.Spec.Recovery.TargetTime != "2026-08-10T00:00:00Z" {
		t.Fatalf("recovery spec = %+v, want the source + PITR target preserved", created.Spec.Recovery)
	}
	if created.Labels[core.LabelTenant] != "tea-a" {
		t.Fatalf("recovered db labels = %v, want the source's workspace", created.Labels)
	}
}

// TestRecoveryValidatesOnlyCallerSuppliedPlanAndVersion pins the boundary the
// gates sit behind. Validating the RESOLVED values instead would refuse to
// recover an instance whose plan has since left the catalog, or whose version is
// empty because it takes the operator's default — a real instance, unrecoverable.
func TestRecoveryValidatesOnlyCallerSuppliedPlanAndVersion(t *testing.T) {
	t.Run("caller-supplied garbage is refused", func(t *testing.T) {
		svc, _ := backedUpSource(t, "free")
		if _, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "r", Plan: "not-a-plan"}); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("bogus plan => %v, want ErrBadRequest", err)
		}
		if _, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "r", Version: "99"}); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("bogus version => %v, want ErrBadRequest", err)
		}
	})

	t.Run("an inherited empty version still recovers", func(t *testing.T) {
		svc, _ := backedUpSource(t, "basic-256mb") // seeded with no Version at all
		if _, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored"}); err != nil {
			t.Fatalf("recovery of a default-version instance => %v, want success", err)
		}
	})
}

// A recovery onto the free plan is a free create: beside the workspace's free
// database it is refused with FREE_INSTANCE_LIMIT and writes nothing, as
// CreatePostgres refuses it.
func TestARecoveryOntoFreeBesideAFreeDatabaseIsRefused(t *testing.T) {
	svc, _ := backedUpSource(t, "basic-256mb")
	holder := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-free", Namespace: "default", Labels: core.TenantLabels("tea-a")},
		Spec:       appv1alpha1.DatabaseSpec{Name: "hobby", Plan: core.FreePlanID},
	}
	if err := svc.Client.Create(context.Background(), holder); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored", Plan: "free"})
	if errorCode(err) != core.FreeInstanceLimitCode || !strings.Contains(err.Error(), "(hobby)") {
		t.Fatalf("free recovery beside a free database = %v, want FREE_INSTANCE_LIMIT naming hobby", err)
	}
	var databases appv1alpha1.DatabaseList
	if err := svc.Client.List(context.Background(), &databases); err != nil || len(databases.Items) != 2 {
		t.Fatalf("a refused recovery left %d databases (%v), want the source and the free one", len(databases.Items), err)
	}
	if _, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored"}); err != nil {
		t.Fatalf("a recovery on the source's paid plan => %v, want success", err)
	}
}

// w34/006: Render's API spells a recovery's plan basic_256mb, pro_4gb or
// 1c-4g. The recovered copy's spec.plan is the catalog id on every surface,
// because the CRD's enum refuses the alias (production answered 500 with the
// CR rejected); an unknown plan is still a 400 and creates nothing.
func TestRecoveryWritesTheCanonicalPlanForARenderAliasOnEverySurface(t *testing.T) {
	recoverWithPlan := func(t *testing.T, svc *Service, surface, plan string) (ok bool) {
		t.Helper()
		rest := func(path, body string, created int) bool {
			res := serveREST(svc, http.MethodPost, path, body)
			if res.Code != created && res.Code != http.StatusBadRequest {
				t.Fatalf("%s with plan %q answered %d %s", path, plan, res.Code, res.Body)
			}
			return res.Code == created
		}
		switch surface {
		case "REST": // Render's endpoint answers 200
			return rest("/v1/postgres/dpg-src/recovery", `{"restoreName":"restored","plan":"`+plan+`"}`, http.StatusOK)
		case "REST /recover": // bex's native alias answers 201
			return rest("/v1/postgres/dpg-src/recover", `{"name":"restored","plan":"`+plan+`"}`, http.StatusCreated)
		case "GraphQL":
			schema, err := pgGQLSchema(svc)
			if err != nil {
				t.Fatal(err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("user-a"),
				VariableValues: map[string]any{"p": plan},
				RequestString:  `mutation($p:String) { recoverDatabase(id:"dpg-src", name:"restored", plan:$p) { id } }`})
			return len(res.Errors) == 0
		case "MCP":
			session, cleanup := pgMCPSession(t, svc)
			defer cleanup()
			res, err := session.CallTool(ctxAs("user-a"), &mcp.CallToolParams{Name: "recover_postgres",
				Arguments: map[string]any{"postgresId": "dpg-src", "name": "restored", "plan": plan}})
			if err != nil {
				t.Fatal(err)
			}
			return !res.IsError
		}
		t.Fatalf("unknown surface %q", surface)
		return false
	}
	// recoveredPlans is the spec.plan of every recovered copy.
	recoveredPlans := func(t *testing.T, cl client.Client) []string {
		t.Helper()
		var list appv1alpha1.DatabaseList
		if err := cl.List(context.Background(), &list); err != nil {
			t.Fatal(err)
		}
		var plans []string
		for _, d := range list.Items {
			if d.Spec.Recovery != nil {
				plans = append(plans, d.Spec.Plan)
			}
		}
		return plans
	}

	for _, surface := range []string{"REST", "REST /recover", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			for alias, want := range map[string]string{"basic_256mb": "basic-256mb", "pro_4gb": "pro-4gb", "1c-4g": "pro-4gb", "basic-1gb": "basic-1gb"} {
				svc, _ := backedUpSource(t, "basic-256mb")
				svc.Payment = &switchablePaymentGate{bound: true}
				if !recoverWithPlan(t, svc, surface, alias) {
					t.Errorf("recovery naming %q was refused", alias)
					continue
				}
				if got := recoveredPlans(t, svc.Client); len(got) != 1 || got[0] != want {
					t.Errorf("recovery naming %q wrote copies on plans %q, want one on %q", alias, got, want)
				}
			}

			svc, _ := backedUpSource(t, "basic-256mb")
			svc.Payment = &switchablePaymentGate{bound: true}
			if recoverWithPlan(t, svc, surface, "basic_512mb") {
				t.Errorf("recovery naming an unknown plan was accepted")
			}
			if got := recoveredPlans(t, svc.Client); len(got) != 0 {
				t.Errorf("a refused recovery created %d copies", len(got))
			}
		})
	}
}

func clientKey(name string) client.ObjectKey {
	return client.ObjectKey{Name: name, Namespace: "default"}
}
