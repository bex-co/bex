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

package staticserver

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestEffectiveHostsPrefersSubdomain mirrors controller.TestEffectiveHosts'
// w4/m19 cases: the static-site resolver must agree with the controller's own
// Ingress-host resolution, or a site's Ingress and its serving proxy would
// route different hostnames for the same App.
func TestEffectiveHostsPrefersSubdomain(t *testing.T) {
	cases := []struct {
		name       string
		app        *appv1alpha1.App
		baseDomain string
		want       []string
	}{
		{"subdomain preferred over name", &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "beancount-cms"},
			Spec:       appv1alpha1.AppSpec{Expose: true, Subdomain: "beancount-cms-bkxk"},
		}, "onbex.co", []string{"beancount-cms-bkxk.onbex.co"}},
		{"empty subdomain falls back to name", &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "a"},
			Spec:       appv1alpha1.AppSpec{Expose: true},
		}, "onbex.co", []string{"a.onbex.co"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := effectiveHosts(tc.app, tc.baseDomain)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}
