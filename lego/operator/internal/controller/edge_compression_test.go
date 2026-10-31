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
	"slices"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1 "k8s.io/api/networking/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A public web service's route compresses its responses at the edge, as
// Render's load balancer does (w8/m83): one App-owned Traefik compress
// Middleware (gzip, compressible types only, no event stream) sits after the
// access layers and before the WebSocket meter. Unexposing the service
// removes it with its Ingress.
func TestWebServiceEdgeCompressionProjection(t *testing.T) {
	scheme := newIPAllowListScheme()
	app := appWithAllowList("edge", nil, appv1alpha1.TypeWebService)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod"}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "edge", Namespace: "default"}
	middleware := func(name string) (*unstructured.Unstructured, error) {
		o := &unstructured.Unstructured{}
		o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
		return o, cl.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, o)
	}
	reconcileTwice(t, r, nn)

	mw, err := middleware("edge-compress")
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if owners := mw.GetOwnerReferences(); len(owners) != 1 || owners[0].Name != "edge" || owners[0].Controller == nil || !*owners[0].Controller {
		t.Errorf("compress owners = %+v, want the App as controller", owners)
	}
	encodings, _, _ := unstructured.NestedStringSlice(mw.Object, "spec", "compress", "encodings")
	contentTypes, _, _ := unstructured.NestedStringSlice(mw.Object, "spec", "compress", "includedContentTypes")
	if !slices.Equal(encodings, []string{"gzip"}) || !slices.Contains(contentTypes, "text/html") || !slices.Contains(contentTypes, "application/json") ||
		slices.Contains(contentTypes, "text/event-stream") || slices.ContainsFunc(contentTypes, func(t string) bool { return strings.HasPrefix(t, "image/") && t != "image/svg+xml" }) {
		t.Errorf("compress spec = encodings %v, types %v", encodings, contentTypes)
	}

	ing := &networkingv1.Ingress{}
	if err := cl.Get(ctx, nn, ing); err != nil {
		t.Fatal(err)
	}
	want := "default-edge-compress@kubernetescrd,default-edge-ws-egress@kubernetescrd"
	if got := ing.Annotations[traefikRouterMiddlewaresAnnotation]; got != want {
		t.Fatalf("router middlewares = %q, want %q", got, want)
	}

	updateApp(t, cl, nn, func(a *appv1alpha1.App) { a.Spec.Expose = false })
	reconcileOnce(t, r, nn)
	if _, err := middleware("edge-compress"); !apierrors.IsNotFound(err) {
		t.Errorf("compress after unexposing: %v, want removed", err)
	}
}
