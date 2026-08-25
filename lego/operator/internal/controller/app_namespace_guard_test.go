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

package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCanonicalNamespaceGuard pins codex #4: the reconciler acts only on Apps in a
// canonical tenant namespace — the shared/bootstrap apps namespace or the App's
// own workspace namespace (namespace == the app.bex.co/workspace label). A
// platform namespace, or a tenant label that disagrees with the namespace
// (cross-tenant injection), is refused.
func TestCanonicalNamespaceGuard(t *testing.T) {
	r := &AppReconciler{AppsNamespace: "default"}
	withWorkspace := func(ns, workspace string) *appv1alpha1.App {
		return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
			Namespace: ns, Labels: map[string]string{labelWorkspace: workspace},
		}}
	}
	for _, c := range []struct {
		name string
		app  *appv1alpha1.App
		want bool
	}{
		{"bootstrap apps namespace", &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Namespace: "default"}}, true},
		{"own workspace namespace", withWorkspace("tea-a", "tea-a"), true},
		{"platform namespace", withWorkspace("kube-system", "tea-a"), false},
		{"cross-tenant: label A but namespace B", withWorkspace("tea-b", "tea-a"), false},
		{"foreign namespace, no workspace label", &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Namespace: "tea-x"}}, false},
		// codex-security 2026-08 F11: a workspace label in the bootstrap
		// namespace is unbound input — appIdentity() derives Zot repository,
		// htpasswd username, static-site prefix, and snapshot prefix from it
		// verbatim, and the projector never writes a labeled App there. A
		// forged label captures another workspace's registry repository ACL.
		{"labeled App in bootstrap namespace (forged identity)", withWorkspace("default", "tea-victim"), false},
	} {
		if got := r.canonicalNamespace(c.app); got != c.want {
			t.Errorf("%s: canonicalNamespace = %v, want %v", c.name, got, c.want)
		}
	}

	// An unset AppsNamespace must still admit the default bootstrap namespace (it
	// mirrors BEX_APPS_NAMESPACE's default), so a misconfiguration never refuses
	// legitimate bootstrap Apps.
	if !(&AppReconciler{}).canonicalNamespace(&appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Namespace: "default"}}) {
		t.Error("empty AppsNamespace must still admit the default bootstrap namespace")
	}
}

// TestReconcileRefusesForeignNamespaceApp proves the guard makes a confused-deputy
// App inert: no finalizer, no requeue, and no child Deployment in the victim
// namespace (codex #4).
func TestReconcileRefusesForeignNamespaceApp(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "evil", Namespace: "kube-system", UID: "uid-evil",
			Labels: map[string]string{labelWorkspace: "tea-victim"},
		},
		Spec: appv1alpha1.AppSpec{Image: "attacker/image:latest"},
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, AppsNamespace: "default"}
	nn := types.NamespacedName{Name: "evil", Namespace: "kube-system"}

	res, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
	if err != nil {
		t.Fatalf("guard returned error: %v", err)
	}
	if res != (reconcile.Result{}) {
		t.Fatalf("foreign App requeued: %+v", res)
	}

	var got appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &got); err != nil {
		t.Fatal(err)
	}
	if controllerutil.ContainsFinalizer(&got, finalizer) {
		t.Error("foreign App must not receive a finalizer")
	}
	var deps appsv1.DeploymentList
	if err := cl.List(context.Background(), &deps, client.InNamespace("kube-system")); err != nil {
		t.Fatal(err)
	}
	if len(deps.Items) != 0 {
		t.Errorf("foreign App produced %d Deployments; want 0", len(deps.Items))
	}
}
