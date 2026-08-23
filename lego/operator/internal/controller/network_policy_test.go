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

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestExecutionNetworkPolicyIsSweptFromBuildNamespace pins the post-co-location
// shape: the pre-deploy Job runs in the App's own namespace (ADR043 D8), so the
// per-App execution-egress exception in the shared build namespace is never
// created anymore — reconcileExecutionNetworkPolicy only sweeps the stale
// object a pre-co-location operator left behind, and only when THIS App owns
// it (a same-named App's policy in another workspace must survive).
func TestExecutionNetworkPolicyIsSweptFromBuildNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "apps", UID: "uid-web",
			Labels: map[string]string{labelWorkspace: "tea-workspace"},
		},
		Spec: appv1alpha1.AppSpec{PreDeployCommand: "migrate"},
	}
	key := client.ObjectKey{Namespace: "bex-build", Name: build.JobName("web", "execution-egress")}
	stale := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: key.Name, Namespace: key.Namespace,
			Labels: map[string]string{labelApp: "web", "app.bex.co/app-uid": "uid-web"},
		},
	}
	foreign := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: key.Name, Namespace: key.Namespace,
			Labels: map[string]string{labelApp: "web", "app.bex.co/app-uid": "uid-other"},
		},
	}

	// A stale policy this App owns is removed even though the App still carries
	// a pre-deploy command — the grant is simply unnecessary now.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, stale).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl, BuildNamespace: "bex-build"}
	if err := r.reconcileExecutionNetworkPolicy(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), key, &networkingv1.NetworkPolicy{}); !apierrors.IsNotFound(err) {
		t.Fatalf("stale execution policy = %v, want swept", err)
	}

	// A same-named FOREIGN policy is left intact (owner-gated delete), and the
	// reconcile creates nothing in the build namespace.
	cl2 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, foreign).Build()
	r2 := &AppReconciler{Client: cl2, BuildClient: cl2, BuildNamespace: "bex-build"}
	if err := r2.reconcileExecutionNetworkPolicy(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	if err := cl2.Get(context.Background(), key, &networkingv1.NetworkPolicy{}); err != nil {
		t.Fatalf("foreign execution policy was touched: %v", err)
	}

	// No build namespace configured: nothing to sweep, no error.
	if err := (&AppReconciler{}).reconcileExecutionNetworkPolicy(context.Background(), app); err != nil {
		t.Errorf("co-located sweep = %v, want nil", err)
	}
}
