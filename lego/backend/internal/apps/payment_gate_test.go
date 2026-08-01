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

package apps

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type rejectingPaymentGate struct{ calls []string }

func (g *rejectingPaymentGate) RequirePaymentMethod(_ context.Context, workspaceID string) error {
	g.calls = append(g.calls, workspaceID)
	return core.NewPaymentRequiredError()
}

func paidGateContext() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})
}

func TestPaidIntentGuardCoversServiceCreatePlanAndBlueprintBeforeWrites(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		gate := &rejectingPaymentGate{}
		svc.Payment = gate
		_, err := svc.Create(paidGateContext(), CreateRequest{Name: "paid", Image: "nginx:alpine", Plan: "starter"})
		if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
			t.Fatalf("paid create err=%v calls=%v", err, gate.calls)
		}
		var apps appv1alpha1.AppList
		if err := cl.List(context.Background(), &apps); err != nil || len(apps.Items) != 0 {
			t.Fatalf("paid refusal wrote Apps: %+v err=%v", apps.Items, err)
		}

		gate.calls = nil
		if _, err := svc.Create(paidGateContext(), CreateRequest{Name: "free", Image: "nginx:alpine", Plan: "free"}); err != nil {
			t.Fatalf("free create: %v", err)
		}
		if len(gate.calls) != 0 {
			t.Fatalf("free create consulted paid gate: %v", gate.calls)
		}
	})

	t.Run("plan change", func(t *testing.T) {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Labels: map[string]string{core.LabelTenant: "tea-a"}},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:alpine", Tier: "free"},
		}
		svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"}, app)
		gate := &rejectingPaymentGate{}
		svc.Payment = gate
		_, err := svc.SetPlan(paidGateContext(), "web", "starter")
		if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 {
			t.Fatalf("paid plan err=%v calls=%v", err, gate.calls)
		}
		if got := getApp(t, svc.Client, "web").Spec.Tier; got != "free" {
			t.Fatalf("refused plan persisted tier %q", got)
		}
	})

	t.Run("Blueprint", func(t *testing.T) {
		svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		gate := &rejectingPaymentGate{}
		svc.Payment = gate
		_, err := svc.DeployStack(paidGateContext(), DeployRequest{Manifest: stackManifest})
		if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 {
			t.Fatalf("paid Blueprint err=%v calls=%v", err, gate.calls)
		}
		var apps appv1alpha1.AppList
		var databases appv1alpha1.DatabaseList
		if err := cl.List(context.Background(), &apps); err != nil {
			t.Fatal(err)
		}
		if err := cl.List(context.Background(), &databases); err != nil {
			t.Fatal(err)
		}
		if len(apps.Items) != 0 || len(databases.Items) != 0 {
			t.Fatalf("paid Blueprint refusal wrote apps=%d databases=%d", len(apps.Items), len(databases.Items))
		}

		gate.calls = nil
		const freeManifest = `services:
  - name: free-web
    type: web
    runtime: image
    image: {url: nginx:alpine}
    plan: free
databases:
  - name: free-db
    plan: free
`
		if _, err := svc.DeployStack(paidGateContext(), DeployRequest{Manifest: freeManifest}); err != nil {
			t.Fatalf("free Blueprint: %v", err)
		}
		if len(gate.calls) != 0 {
			t.Fatalf("free Blueprint consulted paid gate: %v", gate.calls)
		}
	})

	t.Run("Blueprint sync replacement", func(t *testing.T) {
		const originalManifest = `services:
  - name: free-web
    type: web
    runtime: image
    image: {url: nginx:alpine}
    plan: free
`
		blueprints := newFakeBlueprintStore(store.Blueprint{
			ID: "blp-paid-gate", TenantID: "tea-a", Manifest: originalManifest, Status: "active",
		})
		svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		gate := &rejectingPaymentGate{}
		svc.Payment = gate
		svc.Blueprints = blueprints
		_, err := svc.SyncBlueprint(paidGateContext(), "blp-paid-gate", "tea-a", stackManifest, "")
		if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 {
			t.Fatalf("paid Blueprint sync err=%v calls=%v", err, gate.calls)
		}
		if got := blueprints.blueprints["blp-paid-gate"].Manifest; got != originalManifest {
			t.Fatalf("refused Blueprint sync persisted replacement manifest %q", got)
		}
	})
}
