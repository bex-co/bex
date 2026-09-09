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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStaticServePrefixRecordedThenLegacy(t *testing.T) {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web"}}
	app.Status.ActiveRevision = "rev-1"
	if got, want := staticServePrefix(app), "web/rev-1/"; got != want {
		t.Errorf("legacy fallback = %q, want %q", got, want)
	}
	app.Labels = map[string]string{"app.bex.co/workspace": "tea-aaaaaaaaaaaaaaaaaaaa"}
	if got, want := staticServePrefix(app), "web/rev-1/"; got != want {
		t.Errorf("labeled but empty status still dual-reads legacy = %q, want %q", got, want)
	}
	app.Status.StaticPrefix = "tea-aaaaaaaaaaaaaaaaaaaa/web/rev-1/"
	if got, want := staticServePrefix(app), "tea-aaaaaaaaaaaaaaaaaaaa/web/rev-1/"; got != want {
		t.Errorf("recorded prefix = %q, want %q", got, want)
	}
}

func TestIsLegacyStaticPrefix(t *testing.T) {
	cases := []struct {
		name string
		site Site
		want bool
	}{
		{"explicit legacy", Site{AppID: "web", Prefix: "web/rev-1/"}, true},
		{"empty prefix synthesizes legacy", Site{AppID: "web", Revision: "rev-1"}, true},
		{"scoped prefix", Site{AppID: "web", Prefix: "tea-aaaaaaaaaaaaaaaaaaaa/web/rev-1/"}, false},
		{"missing app", Site{Prefix: "web/rev-1/"}, false},
	}
	for _, tc := range cases {
		if got := isLegacyStaticPrefix(tc.site); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
