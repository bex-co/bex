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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestStampedGivesABexAPIWrittenServiceItsCreateTier(t *testing.T) {
	app := func(svcType, tier string) *appv1alpha1.App {
		return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: appv1alpha1.AppSpec{Type: svcType, Tier: tier}}
	}
	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
		want string
	}{
		{"an unattributed tierless service gets create's default", app(appv1alpha1.TypeBackgroundWorker, ""), core.DefaultPaidComputeTier()},
		{"an untyped (web) service too", app("", ""), core.DefaultPaidComputeTier()},
		{"a tier the fixture names is kept", app(appv1alpha1.TypeWebService, "pro"), "pro"},
		{"a static site has no plan to fill", app(appv1alpha1.TypeStaticSite, ""), ""},
		{"a build source has no plan to fill", app(appv1alpha1.TypeBuildSource, ""), ""},
		{"a kubectl-applied CR is left as given", KubectlApplied(app(appv1alpha1.TypeWebService, "")), ""},
		{"a fixture naming bex-api as its writer is filled", withWriter(app(appv1alpha1.TypeWebService, ""), bexAPIManager), core.DefaultPaidComputeTier()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			Stamped(tc.app)
			if tc.app.Spec.Tier != tc.want {
				t.Errorf("tier = %q, want %q", tc.app.Spec.Tier, tc.want)
			}
			if fields := tc.app.GetManagedFields(); len(fields) != 1 || fields[0].Time == nil {
				t.Errorf("managedFields = %+v, want one timed writer entry", fields)
			}
		})
	}
}

func withWriter(a *appv1alpha1.App, manager string) *appv1alpha1.App {
	setManager(a, manager, metav1.NewTime(CreationEpoch))
	return a
}
