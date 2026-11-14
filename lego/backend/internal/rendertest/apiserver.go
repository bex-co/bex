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

package rendertest

import (
	"context"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// stampedAPIVersion fills the stamped managedFields entry's apiVersion, which
// the fake client requires to be non-empty. A typed fixture carries no
// TypeMeta to read one from, and nothing bex reads uses it.
const stampedAPIVersion = "v1"

// CreationEpoch is the creationTimestamp the fake API server stamps.
var CreationEpoch = time.Unix(1_700_000_000, 0).UTC()

// Stamped gives every obj without them a creationTimestamp and a managedFields
// entry, as a real API server guarantees for every stored object, and returns
// objs. It stamps in place, as the fake client builder already sets
// resourceVersion on the fixtures it is given, so a test updating its own
// fixture keeps working.
// controller-runtime's fake client leaves the timestamp zero, and bex then
// omits the createdAt Render requires, so a response from an unstamped
// fixture fails validation for a reason production never sees.
//
// A seeded App bex-api wrote (no writer recorded, or bex-api) also gets the
// tier bex-api's create sets when no plan is asked for: every service it or a
// Blueprint creates has one, and without it the read omits the plan Render
// requires. A static site's plan is optional on Render and a build source has
// none, so those are left as given, as is a KubectlApplied CR.
func Stamped(objs ...client.Object) []client.Object {
	for _, obj := range objs {
		if a, ok := obj.(*appv1alpha1.App); ok && writtenByBexAPI(a) && a.Spec.Tier == "" &&
			a.Spec.Type != appv1alpha1.TypeStaticSite && !core.IsBuildSource(a) {
			a.Spec.Tier = core.DefaultPaidComputeTier()
		}
		stamp(obj)
	}
	return objs
}

// StampOnCreate is the interceptor half of Stamped: an object a test creates
// through the fake client gets its creationTimestamp and managedFields too.
// It fills no tier, so a create that stopped setting one still fails its test.
var StampOnCreate = interceptor.Funcs{
	Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
		stamp(obj)
		return c.Create(ctx, obj, opts...)
	},
}

func stamp(obj client.Object) {
	created := obj.GetCreationTimestamp()
	if created.IsZero() {
		created = metav1.NewTime(CreationEpoch)
		obj.SetCreationTimestamp(created)
	}
	// A real API server records the writer of every stored object in
	// managedFields, and bex reads an unannotated object's updatedAt from
	// there (resourcemeta.UpdatedAt). The fake client records none, so without
	// this an unstamped fixture omits the updatedAt Render requires, which
	// production never does.
	if len(obj.GetManagedFields()) == 0 {
		setManager(obj, bexAPIManager, created)
	}
}

// bexAPIManager is the managedFields writer Stamped records for a fixture.
const bexAPIManager = "bex-api"

func writtenByBexAPI(a *appv1alpha1.App) bool {
	fields := a.GetManagedFields()
	return len(fields) == 0 || slices.ContainsFunc(fields, func(f metav1.ManagedFieldsEntry) bool { return f.Manager == bexAPIManager })
}

// KubectlApplied marks a as written by kubectl rather than bex-api, so
// Stamped leaves it as given: a hand-applied App can lack the tier every
// bex-api create sets.
func KubectlApplied(a *appv1alpha1.App) *appv1alpha1.App {
	setManager(a, "kubectl", metav1.NewTime(CreationEpoch))
	return a
}

func setManager(obj client.Object, manager string, at metav1.Time) {
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager: manager, Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: stampedAPIVersion, Time: &at,
		FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte("{}")},
	}})
}
