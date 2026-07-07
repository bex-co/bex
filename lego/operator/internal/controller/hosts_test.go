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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func mkApp(name, host string, expose bool, hosts ...string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       appv1alpha1.AppSpec{Host: host, Expose: expose, Hosts: hosts},
	}
}

func TestEffectiveHosts(t *testing.T) {
	cases := []struct {
		name       string
		app        *appv1alpha1.App
		baseDomain string
		want       []string
	}{
		// The live-App invariant: host-only spec must resolve to exactly that host,
		// first, regardless of base domain being configured on the controller.
		{"legacy host only", mkApp("beancount-cms", "beancount-cms-v2.onbex.co", false), "onbex.co",
			[]string{"beancount-cms-v2.onbex.co"}},
		{"no exposure", mkApp("a", "", false), "onbex.co", nil},
		{"expose without base domain", mkApp("a", "", true), "", nil},
		{"expose computes platform host", mkApp("a", "", true), "onbex.co",
			[]string{"a.onbex.co"}},
		{"host stays first when exposed", mkApp("a", "x.example.com", true), "onbex.co",
			[]string{"x.example.com", "a.onbex.co"}},
		{"custom domains appended", mkApp("a", "a.onbex.co", false, "www.example.com", "example.net"), "onbex.co",
			[]string{"a.onbex.co", "www.example.com", "example.net"}},
		{"dedup across sources", mkApp("a", "a.onbex.co", true, "a.onbex.co", "www.example.com"), "onbex.co",
			[]string{"a.onbex.co", "www.example.com"}},
		{"empty entries dropped", mkApp("a", "", false, "", "www.example.com"), "",
			[]string{"www.example.com"}},
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

func TestTLSSecretName(t *testing.T) {
	// The live-App invariant: the first host keeps the legacy name.
	if got := tlsSecretName("beancount-cms", 0, "beancount-cms-v2.onbex.co"); got != "beancount-cms-tls" {
		t.Fatalf("first host secret = %q, want beancount-cms-tls", got)
	}
	if got := tlsSecretName("a", 1, "www.example.com"); got != "a-tls-www.example.com" {
		t.Fatalf("second host secret = %q", got)
	}
	if got := tlsSecretName("a", 1, "*.example.com"); strings.Contains(got, "*") {
		t.Fatalf("wildcard not sanitized: %q", got)
	}
	long := strings.Repeat("x", 300) + ".example.com"
	if got := tlsSecretName("a", 1, long); len(got) > 253 {
		t.Fatalf("secret name exceeds 253 chars: %d", len(got))
	}
}
