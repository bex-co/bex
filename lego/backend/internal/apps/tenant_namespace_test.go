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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// tenant_namespace_test.go is the regression guard for the ADR043 per-tenant
// namespace read-path bug: once BEX_TENANT_NAMESPACES was enabled the projector
// moved every store-managed App CR into its workspace's own `<ws>` namespace
// (== the workspace id), but bex-api's App-CR reads still listed only the shared
// BEX_API_NAMESPACE ("default"), so `services(ownerId=…)` came back empty and the
// dashboard showed no services for any workspace. These tests pin the fix: with
// core.Base.TenantNamespaces set, App-CR reads resolve into the tenant namespace.

const testWS = "tea-d98210cbbpdc73dcrkvg"

// tenantNSApp builds a store-managed App CR exactly as the projector projects it
// under per-tenant namespaces: in the `<ws>` namespace, object-named
// CRName(ws, svc), and carrying the tenant + public-name + app-id labels the read
// path resolves against.
func tenantNSApp(svc, ws, appID string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      core.CRName(ws, svc),
			Namespace: ws,
			Labels: map[string]string{
				core.LabelTenant:      ws,
				core.LabelServiceName: svc,
				core.LabelAppID:       appID,
			},
		},
		Spec:   appv1alpha1.AppSpec{Image: svc + ":v1", Replicas: 1},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, URL: "https://" + svc + ".onbex.co"},
	}
}

// TestList_TenantNamespaceMode is the direct reproduction of the prod incident:
// an App projected into `<ws>` is invisible to a shared-namespace List but
// surfaces once TenantNamespaces is on.
func TestList_TenantNamespaceMode(t *testing.T) {
	app := tenantNSApp("agentmarketcap-1", testWS, "srv-1")
	cl := fakeClient(app)

	// The bug: a shared-namespace Service scoped to "default" finds nothing,
	// because the App lives only in the `<ws>` namespace.
	shared := &Service{Base: &core.Base{Client: cl, Namespace: "default", Authz: &fakeChecker{allow: true}}}
	if got, err := shared.List(ctxAs("user-a"), testWS); err != nil {
		t.Fatalf("shared List: %v", err)
	} else if len(got) != 0 {
		t.Fatalf("shared-namespace List unexpectedly found %d apps in default; the App lives only in %s", len(got), testWS)
	}

	// The fix: with per-tenant namespaces, List resolves into the `<ws>` namespace.
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true, Authz: &fakeChecker{allow: true}}}
	got, err := svc.List(ctxAs("user-a"), testWS)
	if err != nil {
		t.Fatalf("tenant-ns List: %v", err)
	}
	if len(got) != 1 || got[0].Name != "agentmarketcap-1" || got[0].OwnerID != testWS {
		t.Fatalf("tenant-ns List = %+v, want one app agentmarketcap-1 owned by %s", got, testWS)
	}
}

// TestWebhookRedeploy_TenantNamespaceMode reproduces the production failure in
// which GitHub received 200 {"redeployed":[]} for an auto-deploy App projected
// outside BEX_API_NAMESPACE. The signed webhook must list across tenant
// namespaces and patch the exact App it matched instead of re-fetching its CR
// name from the shared namespace.
func TestWebhookRedeploy_TenantNamespaceMode(t *testing.T) {
	const repo = "https://github.com/bex-co/eden-cms-v2"
	app := tenantNSApp("eden-cms-v2", testWS, "srv-1")
	app.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}
	cl := fakeClient(app)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true}}
	h := &GitWebhook{Svc: svc, Secret: "shh"}

	redeployed, err := h.redeployMatching(context.Background(), newPush(repo, []string{"docs/cli.md"}), "main")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if !contains(redeployed, app.Name) {
		t.Fatalf("tenant App was not redeployed: got %v, want %s", redeployed, app.Name)
	}

	var got appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: testWS, Name: app.Name}, &got); err != nil {
		t.Fatalf("get tenant App: %v", err)
	}
	if got.Spec.RestartedAt == "" {
		t.Error("tenant App restart timestamp was not bumped")
	}
}

// TestWebhookBranchDelete_TenantNamespaceMode keeps the sibling delete path in
// sync: deleting a tracked branch must find the tenant App and disable its
// auto-deploy even though no matching App exists in the shared namespace.
func TestWebhookBranchDelete_TenantNamespaceMode(t *testing.T) {
	const repo = "https://github.com/bex-co/eden-cms-v2"
	app := tenantNSApp("eden-cms-v2", testWS, "srv-1")
	app.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}
	cl := fakeClient(app)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true}}
	h := &GitWebhook{Svc: svc, Secret: "shh"}

	affected, err := h.recordBranchDeleted(context.Background(), []string{repo}, "main", "delivery-1")
	if err != nil {
		t.Fatalf("recordBranchDeleted: %v", err)
	}
	if !contains(affected, app.Name) {
		t.Fatalf("tenant App was not affected: got %v, want %s", affected, app.Name)
	}

	var got appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: testWS, Name: app.Name}, &got); err != nil {
		t.Fatalf("get tenant App: %v", err)
	}
	if got.Spec.AutoDeploy {
		t.Error("tenant App auto-deploy remained enabled after tracked branch deletion")
	}
}

// TestList_TenantNamespaceMode_CallerDefaultWorkspace covers the unnamed-ownerId
// path the dashboard actually uses: the workspace is the caller's own, resolved
// through the store, and must still scope into that workspace's namespace.
func TestList_TenantNamespaceMode_CallerDefaultWorkspace(t *testing.T) {
	cl := fakeClient(tenantNSApp("web", testWS, "srv-1"), tenantNSApp("api", "tea-other", "srv-2"))
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true,
		Workspace: fakeWorkspace{"user-a": testWS},
		Authz:     &fakeChecker{allow: true}}}

	got, err := svc.List(ctxAs("user-a"), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].Name != "web" {
		t.Fatalf("caller-default List = %+v, want only this workspace's web (not tea-other's api)", got)
	}
}

// TestAuthorizeApp_TenantNamespaceMode proves the single-resource fetch that
// backs Get and every mutation resolves an App in its `<ws>` namespace — both by
// its Render srv- id (cluster-wide unique) and by its bare service name.
func TestAuthorizeApp_TenantNamespaceMode(t *testing.T) {
	cl := fakeClient(tenantNSApp("agentmarketcap-1", testWS, "srv-1"))
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true,
		Workspace: fakeWorkspace{"user-a": testWS},
		Authz:     &fakeChecker{allow: true}}}

	for _, ref := range []string{"srv-1", "agentmarketcap-1"} {
		got, err := svc.AuthorizeApp(ctxAs("user-a"), core.RelCanView, ref)
		if err != nil {
			t.Fatalf("AuthorizeApp(%q): %v", ref, err)
		}
		if got.Namespace != testWS || got.Name != core.CRName(testWS, "agentmarketcap-1") {
			t.Fatalf("AuthorizeApp(%q) = %s/%s, want %s/%s", ref, got.Namespace, got.Name, testWS, core.CRName(testWS, "agentmarketcap-1"))
		}
	}

	// A missing service is still a clean NotFound, not a namespace error.
	if _, err := svc.AuthorizeApp(ctxAs("user-a"), core.RelCanView, "nope"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("AuthorizeApp(nope) = %v, want ErrNotFound", err)
	}
}

// TestAppPods_TenantNamespaceMode proves the pod-backed reads (instances, live
// tail, metrics fallback) find replica pods in the App's `<ws>` namespace even
// though bex-api holds no cluster-wide pod-list grant — AppPods resolves the
// namespace via the cluster-wide App list it does hold.
func TestAppPods_TenantNamespaceMode(t *testing.T) {
	crName := core.CRName(testWS, "agentmarketcap-1")
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      crName + "-abc123",
		Namespace: testWS,
		Labels:    map[string]string{core.PodLabelApp: crName},
	}}
	cl := fakeClient(tenantNSApp("agentmarketcap-1", testWS, "srv-1"), pod)
	base := &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true}

	pods, err := base.AppPods(context.Background(), crName)
	if err != nil {
		t.Fatalf("AppPods: %v", err)
	}
	if len(pods) != 1 || pods[0].Name != crName+"-abc123" {
		t.Fatalf("AppPods = %+v, want the single replica pod in %s", pods, testWS)
	}
}

// TestAppNamespaceByName pins the name→namespace resolver the metrics resource
// funnel uses when it holds only the app name: it finds the App's `<ws>`
// namespace on, is the shared namespace off, and falls back (never errors) for a
// missing App so a metrics query just yields empty series.
func TestAppNamespaceByName(t *testing.T) {
	crName := core.CRName(testWS, "web")
	cl := fakeClient(tenantNSApp("web", testWS, "srv-1"))

	on := &core.Base{Client: cl, Namespace: "default", TenantNamespaces: true}
	if got := on.AppNamespaceByName(context.Background(), crName); got != testWS {
		t.Errorf("on: AppNamespaceByName(%s) = %q, want %s", crName, got, testWS)
	}
	if got := on.AppNamespaceByName(context.Background(), "does-not-exist"); got != "default" {
		t.Errorf("missing App must fall back to shared namespace, got %q", got)
	}

	off := &core.Base{Client: cl, Namespace: "default"}
	if got := off.AppNamespaceByName(context.Background(), crName); got != "default" {
		t.Errorf("off: AppNamespaceByName = %q, want default (no cluster-wide lookup)", got)
	}
}

// TestAppNamespace pins the resolver both modes turn on.
func TestAppNamespace(t *testing.T) {
	off := &core.Base{Namespace: "default"}
	if got := off.AppNamespace(testWS); got != "default" {
		t.Errorf("TenantNamespaces off: AppNamespace(%s) = %q, want default", testWS, got)
	}
	on := &core.Base{Namespace: "default", TenantNamespaces: true}
	if got := on.AppNamespace(testWS); got != testWS {
		t.Errorf("TenantNamespaces on: AppNamespace(%s) = %q, want %s", testWS, got, testWS)
	}
	if got := on.AppNamespace(""); got != "default" {
		t.Errorf("empty tenant must map to shared namespace, got %q", got)
	}
	if got := on.AppNamespace(core.DefaultTenant); got != "default" {
		t.Errorf("default tenant must map to shared namespace, got %q", got)
	}
}
