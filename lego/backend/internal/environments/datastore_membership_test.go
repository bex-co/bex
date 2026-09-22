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
package environments

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func snapshotMembershipResources(t *testing.T, cl client.Client) []client.Object {
	t.Helper()
	var out []client.Object
	for _, name := range []string{"member", "other", "foreign"} {
		for _, obj := range []client.Object{&appv1alpha1.Database{}, &appv1alpha1.KeyValue{}} {
			if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "default", Name: name}, obj); err != nil {
				t.Fatal(err)
			}
			out = append(out, obj)
		}
	}
	return out
}

func TestDatastoreMembershipRejectsInvalidReplacementBeforeWrites(t *testing.T) {
	for _, kind := range []string{"postgres", "keyvalue"} {
		for _, invalid := range []string{"missing", "foreign", ""} {
			for _, mixed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/id=%q/mixed=%t", kind, invalid, mixed), func(t *testing.T) {
					svc, cl, _ := datastorePlacementService(t, interceptor.Funcs{})
					before := snapshotMembershipResources(t, cl)
					set := svc.SetDatabases
					if kind == "keyvalue" {
						set = svc.SetKeyValues
					}
					wanted := []string{invalid}
					if mixed {
						wanted = append([]string{"other"}, wanted...)
					}
					_, err := set(ctxAs("user-a"), "evm-delete", wanted)
					if !errors.Is(err, core.ErrForbidden) {
						t.Errorf("invalid replacement error=%v, want forbidden", err)
					} else if want := fmt.Sprintf("%v: %q does not belong to workspace %q", core.ErrForbidden, invalid, "tea-a"); err.Error() != want {
						t.Errorf("error=%q, want non-disclosing refusal %q", err, want)
					}
					if got := snapshotMembershipResources(t, cl); !reflect.DeepEqual(got, before) {
						t.Error("invalid replacement changed existing datastore labels, rules, or metadata")
					}
				})
			}
		}
	}
}

func TestDatastoreMembershipAuthorizedEmptyReplacementClears(t *testing.T) {
	for _, kind := range []string{"postgres", "keyvalue"} {
		t.Run(kind, func(t *testing.T) {
			svc, cl, _ := datastorePlacementService(t, interceptor.Funcs{})
			before := snapshotMembershipResources(t, cl)
			set := svc.SetDatabases
			if kind == "keyvalue" {
				set = svc.SetKeyValues
			}
			if _, err := set(ctxAs("user-a"), "evm-delete", nil); err != nil {
				t.Fatal(err)
			}
			got := snapshotMembershipResources(t, cl)
			for i, previous := range before {
				_, postgres := previous.(*appv1alpha1.Database)
				target := previous.GetName() == "member" && (postgres == (kind == "postgres"))
				if !target {
					if !reflect.DeepEqual(got[i], previous) {
						t.Errorf("empty replacement changed unrelated resource %T %s", previous, previous.GetName())
					}
					continue
				}
				want := previous.DeepCopyObject().(client.Object)
				delete(want.GetLabels(), core.LabelEnvironment)

				switch obj := want.(type) {
				case *appv1alpha1.Database:
					obj.Spec.EnvironmentIPAllowList = nil
				case *appv1alpha1.KeyValue:
					obj.Spec.EnvironmentIPAllowList = nil
				}
				// Placement writes advance these two fields; all resource state must match.
				want.SetResourceVersion(got[i].GetResourceVersion())
				annotations := want.GetAnnotations()
				if annotations == nil {
					annotations = map[string]string{}
				}
				annotations[resourcemeta.UpdatedAtAnnotation] = got[i].GetAnnotations()[resourcemeta.UpdatedAtAnnotation]
				want.SetAnnotations(annotations)
				if !reflect.DeepEqual(got[i], want) {
					t.Errorf("empty replacement failed to clear membership while preserving own rules: got=%+v want=%+v", got[i], want)
				}
			}
		})
	}
}
