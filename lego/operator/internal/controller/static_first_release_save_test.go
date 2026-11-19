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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// savedAfterFirstRelease is a static site whose first release (generation 1)
// published while two saves (generations 2 and 3) waited for a deploy: the
// state production's first deploy was stranded in (w8/m460).
func savedAfterFirstRelease() *appv1alpha1.App {
	spec := appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/octocat/Spoon-Knife", Branch: "main",
		BuildCommand: "mkdir -p out && echo m > out/marker.txt", PublishPath: "out",
	}
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 3, Annotations: map[string]string{
			appv1alpha1.AnnotationReleaseGeneration:       "1",
			appv1alpha1.AnnotationPendingSourceGeneration: "3",
		}},
		Spec: spec,
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseDeploying, ReleaseGeneration: 1, ActiveRevision: "rev-1",
			ReleaseFingerprint:         desiredAppReleaseIdentity(spec).release,
			ReleaseSettingsFingerprint: releaseSettingsFingerprint(spec),
			PendingReleaseGeneration:   2,
		},
	}
}

// The pending slot coalesced the first save while the release built. Once the
// release settled, that generation is only the save, which waits for a deploy:
// left in the slot it requeued the settled release every second.
func TestSettledReleaseOwesNothingForASave(t *testing.T) {
	app := savedAfterFirstRelease()
	decision := prepareAppReleaseDecision(app, nil)
	if !decision.sourcePending || decision.pinned {
		t.Fatalf("decision = %+v, want the save to wait behind the settled release", decision)
	}
	if app.Status.PendingReleaseGeneration != 0 {
		t.Fatalf("pendingReleaseGeneration = %d, want 0: a save owes no release", app.Status.PendingReleaseGeneration)
	}
	if _, owed := owedReleaseRequeue(app); owed {
		t.Fatal("a settled release behind a save must not requeue as owed")
	}
}

func TestStaticReleaseFinishing(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*appv1alpha1.App)
		want bool
	}{
		{"first release published, host not handed over", func(*appv1alpha1.App) {}, true},
		{"served and running", func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseRunning }, true},
		{"direct first publish uploading", func(a *appv1alpha1.App) { a.Status.ActiveRevision = "" }, true},
		{"direct first publish cloning", func(a *appv1alpha1.App) {
			a.Status.ActiveRevision, a.Status.Phase = "", appv1alpha1.PhaseBuilding
		}, true},
		{"newer direct release publishing over a served one", func(a *appv1alpha1.App) { a.Status.ReleaseGeneration = 2 }, true},
		{"built release whose image is not built yet", func(a *appv1alpha1.App) {
			a.Status.ActiveRevision, a.Status.Phase = "", appv1alpha1.PhaseBuilding
			a.Status.ReleaseArtifactFingerprint = "artifact-v1:building"
		}, false},
		{"failed first release", func(a *appv1alpha1.App) {
			a.Status.ActiveRevision, a.Status.Phase = "", appv1alpha1.PhaseFailed
		}, false},
		{"newer release failed over a served one", func(a *appv1alpha1.App) {
			a.Status.ReleaseGeneration, a.Status.Phase = 2, appv1alpha1.PhaseRunning
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := savedAfterFirstRelease()
			tc.edit(app)
			if got := staticReleaseFinishing(app); got != tc.want {
				t.Fatalf("staticReleaseFinishing = %v, want %v (status %+v)", got, tc.want, app.Status)
			}
		})
	}
}

// A static site keeps no build-input records, so its undeployed changes read
// the saved settings against the release's own settings fingerprint while a
// save waits.
func TestStaticSaveWaits(t *testing.T) {
	// restamped is bex-api's root or publish directory save, which stamps a
	// rebuild (SetRootDir, SetPublishPath).
	restamped := func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-11-19T09:00:00Z" }
	for _, tc := range []struct {
		name string
		edit func(*appv1alpha1.App)
		want bool
	}{
		{"saves that net out to the release's own settings", func(*appv1alpha1.App) {}, false},
		// w8/339: a root directory set and cleared left only bex-api's stamp.
		{"a set-and-clear that left only a rebuild stamp", restamped, false},
		{"a saved publish directory", func(a *appv1alpha1.App) { a.Spec.PublishPath = "out2"; restamped(a) }, true},
		{"a saved root directory", func(a *appv1alpha1.App) { a.Spec.RootDir = "sub"; restamped(a) }, true},
		{"an operational change", func(a *appv1alpha1.App) { a.Spec.DisplayName = "renamed" }, false},
		{"a release an older operator dispatched compares its whole identity", func(a *appv1alpha1.App) {
			a.Status.ReleaseSettingsFingerprint = ""
			restamped(a)
		}, true},
		{"an older operator's release whose settings net out", func(a *appv1alpha1.App) {
			a.Status.ReleaseSettingsFingerprint = ""
		}, false},
		{"a deploy consumed the save", func(a *appv1alpha1.App) {
			a.Spec.PublishPath = "out2"
			a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "3"
		}, false},
		{"nothing served yet", func(a *appv1alpha1.App) {
			a.Spec.PublishPath = "out2"
			a.Status.ActiveRevision = ""
		}, false},
		{"a release-v1 fingerprint cannot be compared", func(a *appv1alpha1.App) {
			a.Status.ReleaseFingerprint, a.Status.ReleaseSettingsFingerprint = legacyReleaseFingerprint(a.Spec), ""
			a.Spec.PublishPath = "out2"
		}, false},
		{"a settings fingerprint of another identity version cannot be compared", func(a *appv1alpha1.App) {
			a.Status.ReleaseSettingsFingerprint = "release-v3:0"
			a.Spec.PublishPath = "out2"
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := savedAfterFirstRelease()
			tc.edit(app)
			if got := staticSaveWaits(app); got != tc.want {
				t.Fatalf("staticSaveWaits = %v, want %v", got, tc.want)
			}
		})
	}
}

// A dispatch records its release's saved settings next to its identity, and a
// deploy that only restamps them records the same settings (w8/339).
func TestDispatchRecordsReleaseSettings(t *testing.T) {
	spec := appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/octocat/Spoon-Knife", Branch: "main",
		BuildCommand: "mkdir -p out", PublishPath: "out",
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 1, Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
		Spec:       spec,
	}
	if d := prepareAppReleaseDecision(app, nil); !d.releaseChanged {
		t.Fatalf("decision = %+v, want the first release dispatched", d)
	}
	settings := app.Status.ReleaseSettingsFingerprint
	if settings != releaseSettingsFingerprint(spec) {
		t.Fatalf("releaseSettingsFingerprint = %q, want the dispatched spec's", settings)
	}

	app.Generation = 2
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	app.Spec.RestartedAt = "2026-11-19T09:00:00Z"
	first := app.Status.ReleaseFingerprint
	if d := prepareAppReleaseDecision(app, nil); !d.releaseChanged || app.Status.ReleaseFingerprint == first {
		t.Fatalf("decision = %+v, want the restamped deploy dispatched as a new release", d)
	}
	if app.Status.ReleaseSettingsFingerprint != settings {
		t.Fatal("a deploy that only restamps the release must record the same settings")
	}

	app.Generation = 3
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "3"
	app.Spec.PublishPath = "out2"
	prepareAppReleaseDecision(app, nil)
	if app.Status.ReleaseSettingsFingerprint == settings {
		t.Fatal("a deploy of a new publish directory must record its settings")
	}
}
