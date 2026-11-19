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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The regression for w5/195: a hand-applied App in `default` with no
// workspace label served on the platform domain for four months and nothing
// listed it. Each App outside its workspace's namespace gets a series; an App
// in its own workspace namespace gets none.
func TestAppPlacementMetricsNameAppsOutsideTheirWorkspace(t *testing.T) {
	scheme := newTestScheme(t)
	app := func(namespace, name, workspace string) *appv1alpha1.App {
		a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		if workspace != "" {
			a.Labels = map[string]string{labelWorkspace: workspace}
		}
		return a
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		app("tea-ws1", "web", "tea-ws1"),                // where bex-api puts every App
		app("default", "hand-applied", ""),              // the w5/195 shape
		app("tea-ws1", "mislabeled", "tea-ws2"),         // another workspace's label
		app("tea-ws2", "unlabeled-in-workspace", ""),    // a workspace namespace, no label
		app("default", "labeled-in-default", "tea-ws1"), // a label, the wrong namespace
	).Build()

	got := renderMetric(t, NewAppPlacementCollector(cl), "bex_app_placement_outside_workspace")
	for _, want := range []string{
		`bex_app_placement_outside_workspace{name="hand-applied",namespace="default",workspace=""} 1`,
		`bex_app_placement_outside_workspace{name="mislabeled",namespace="tea-ws1",workspace="tea-ws2"} 1`,
		`bex_app_placement_outside_workspace{name="unlabeled-in-workspace",namespace="tea-ws2",workspace=""} 1`,
		`bex_app_placement_outside_workspace{name="labeled-in-default",namespace="default",workspace="tea-ws1"} 1`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing series:\n  want %s\n  got:\n%s", want, got)
		}
	}
	if strings.Contains(got, `name="web"`) {
		t.Errorf("an App in its own workspace namespace must not be reported, got:\n%s", got)
	}
}

// A listing failure must not read as "no App outside a workspace": the alert
// on the outside series would go quiet exactly when observation breaks.
func TestAppPlacementMetricsCountObservationFailure(t *testing.T) {
	got := renderMetric(t, NewAppPlacementCollector(failingLister{}), "bex_app_placement_observe_errors_total")
	if !strings.Contains(got, "bex_app_placement_observe_errors_total 1") {
		t.Errorf("a failed listing must be counted, got:\n%s", got)
	}
	if strings.Contains(got, "bex_app_placement_outside_workspace{") {
		t.Error("a failed listing must emit no placement series at all")
	}
}
