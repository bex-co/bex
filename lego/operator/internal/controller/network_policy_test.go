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
	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestExecutionNetworkPolicyScopesToPreDeploy(t *testing.T) {
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
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl, BuildNamespace: "bex-build"}

	if err := r.reconcileExecutionNetworkPolicy(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	np := &networkingv1.NetworkPolicy{}
	key := client.ObjectKey{Namespace: "bex-build", Name: build.JobName("web", "execution-egress")}
	if err := cl.Get(context.Background(), key, np); err != nil {
		t.Fatal(err)
	}
	if got := np.Spec.PodSelector.MatchLabels[execution.LabelComponent]; got != predeploy.ComponentValue {
		t.Fatalf("execution source component = %q, want %q", got, predeploy.ComponentValue)
	}
	if got := np.Spec.PodSelector.MatchLabels[labelWorkspace]; got != "tea-workspace" {
		t.Fatalf("execution source workspace = %q", got)
	}

	app.Spec.PreDeployCommand = ""
	if err := r.reconcileExecutionNetworkPolicy(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), key, &networkingv1.NetworkPolicy{}); !apierrors.IsNotFound(err) {
		t.Fatalf("execution policy after command removal = %v, want not found", err)
	}
}
