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

package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestInvalidFieldsErrorNamesTheFields pins the 400 a CRD refusal becomes,
// against the status a live API server returned for a server-side dry-run of
// an App whose startCommand was 5,000 bytes (w5/m116): the field cause, without
// the object's name or the fieldless note that CEL rules went unchecked.
func TestInvalidFieldsErrorNamesTheFields(t *testing.T) {
	live := &apierrors.StatusError{ErrStatus: metav1.Status{
		Status: metav1.StatusFailure, Code: 422, Reason: metav1.StatusReasonInvalid,
		Message: `App.app.bex.co "tea-x-web" is invalid: [spec.startCommand: Too long: may not be more than 4096 bytes, <nil>: Invalid value: "null": some validation rules were not checked because the object was invalid; correct the existing errors to complete validation]`,
		Details: &metav1.StatusDetails{Name: "tea-x-web", Group: "app.bex.co", Kind: "App", Causes: []metav1.StatusCause{
			{Type: metav1.CauseType("FieldValueTooLong"), Message: "Too long: may not be more than 4096 bytes", Field: "spec.startCommand"},
			{Type: metav1.CauseTypeFieldValueInvalid, Message: "Invalid value: null: some validation rules were not checked because the object was invalid; correct the existing errors to complete validation", Field: "<nil>"},
		}},
	}}
	mapped, ok := InvalidFieldsError(fmt.Errorf("creating App: %w", live))
	if !ok || !errors.Is(mapped, ErrBadRequest) {
		t.Fatalf("mapped = %v, %v; want a bad request", mapped, ok)
	}
	if want := "bad request: spec.startCommand: Too long: may not be more than 4096 bytes"; mapped.Error() != want {
		t.Fatalf("message = %q\nwant      %q", mapped.Error(), want)
	}
	if _, ok := InvalidFieldsError(apierrors.NewForbidden(schema.GroupResource{Group: "app.bex.co", Resource: "apps"}, "x", errors.New("quota"))); ok {
		t.Fatal("a Forbidden is not an invalid object")
	}
}

// TestDryRunCreateValidatesAFirstResourceInTheBootstrapNamespace: before a
// workspace's namespace exists, the dry-run validates the object in the
// bootstrap namespace without its workspace label (the only form the app
// admission policy accepts there), a name taken only there is no refusal, and
// the caller's object is never touched.
func TestDryRunCreateValidatesAFirstResourceInTheBootstrapNamespace(t *testing.T) {
	var seen []string
	cl := interceptor.NewClient(fakeAppClient().(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			var o client.CreateOptions
			o.ApplyOptions(opts)
			if !slices.Contains(o.DryRun, metav1.DryRunAll) {
				t.Fatalf("DryRunCreate persisted %s/%s", obj.GetNamespace(), obj.GetName())
			}
			seen = append(seen, obj.GetNamespace())
			if obj.GetNamespace() == "tea-a" {
				return apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "tea-a")
			}
			if _, labeled := obj.GetLabels()[LabelWorkspace]; labeled {
				t.Fatal("the stand-in carried the workspace label the admission policy refuses there")
			}
			return apierrors.NewAlreadyExists(schema.GroupResource{Group: "app.bex.co", Resource: "apps"}, obj.GetName())
		},
	})
	b := &Base{Client: cl, Namespace: "default"}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "tea-a-web", Namespace: "tea-a", Labels: TenantLabels("tea-a")}}
	if err := b.DryRunCreate(context.Background(), app); err != nil {
		t.Fatalf("DryRunCreate = %v, want nil", err)
	}
	if !slices.Equal(seen, []string{"tea-a", "default"}) {
		t.Fatalf("dry-run namespaces = %v, want the workspace's, then the bootstrap one", seen)
	}
	if app.Namespace != "tea-a" || app.Labels[LabelWorkspace] != "tea-a" {
		t.Fatalf("caller's object changed: %s %v", app.Namespace, app.Labels)
	}
}
