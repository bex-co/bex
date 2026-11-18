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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m480, docs/ADR116-managed-oidc.md D12: when the observed native build
// ends, its decision leaves status (bex-api stops minting) and both token
// Secrets go; a build still running keeps them, and a Secret of another App
// lifetime stays.
func TestEndBuildManagedOIDCEndsTheBuildsTokensOnlyOnceItIsOver(t *testing.T) {
	ctx := context.Background()
	scheme := nativeEnvScheme(t)
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: types.UID("uid-web")}}
	app.Status.BuildManagedOIDC = &appv1alpha1.ManagedOIDCStatus{Generation: 4, Providers: []string{"aws"}, PlanAllowed: true}
	copied := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: buildManagedOIDCSecretName("web"), Namespace: "bex-build", Labels: artifactLabels(app, "managed-oidc-secret")}}
	minted := func(uid types.UID) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ManagedOIDCBuildSecretName("web"), Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "app.bex.co/v1alpha1", Kind: "App", Name: "web", UID: uid}}}}
	}
	exists := func(cl client.Client, key client.ObjectKey) bool {
		t.Helper()
		err := cl.Get(ctx, key, &corev1.Secret{})
		if err != nil && !apierrors.IsNotFound(err) {
			t.Fatal(err)
		}
		return err == nil
	}
	copyKey := client.ObjectKeyFromObject(copied)
	srcKey := client.ObjectKey{Namespace: "default", Name: appv1alpha1.ManagedOIDCBuildSecretName("web")}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app.DeepCopy(), copied, minted(app.UID)).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	live := &appv1alpha1.App{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), live); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []build.Phase{build.PhaseWaiting, build.PhaseBuilding} {
		r.endBuildManagedOIDC(ctx, live, "bex-build", phase)
		if live.Status.BuildManagedOIDC == nil || !exists(cl, copyKey) || !exists(cl, srcKey) {
			t.Fatalf("phase %v: a build still waiting or running lost its tokens", phase)
		}
	}
	r.endBuildManagedOIDC(ctx, live, "bex-build", build.PhaseSucceeded)
	stored := &appv1alpha1.App{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.BuildManagedOIDC != nil || exists(cl, copyKey) || exists(cl, srcKey) {
		t.Fatalf("after the build: decision %+v, copy %v, minted %v; want all gone",
			stored.Status.BuildManagedOIDC, exists(cl, copyKey), exists(cl, srcKey))
	}

	// A failed build ends them too; a Secret another App lifetime owns stays.
	cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(app.DeepCopy(), minted("uid-other")).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r = &AppReconciler{Client: cl, BuildClient: cl}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), live); err != nil {
		t.Fatal(err)
	}
	r.endBuildManagedOIDC(ctx, live, "bex-build", build.PhaseFailed)
	if live.Status.BuildManagedOIDC != nil || !exists(cl, srcKey) {
		t.Fatalf("a failed build: decision %+v, foreign Secret kept %v", live.Status.BuildManagedOIDC, exists(cl, srcKey))
	}
}
