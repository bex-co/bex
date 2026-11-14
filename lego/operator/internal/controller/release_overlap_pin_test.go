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

// overlapApp is a repo-backed App whose generation-2 release is in flight in
// phase, with generation 3 saved and requested behind it (w8/m119): the spec
// already carries generation 3's start command, the status still generation
// 2's identity.
func overlapApp(phase appv1alpha1.AppPhase, activeRevision string) *appv1alpha1.App {
	executing := appv1alpha1.AppSpec{
		Repo: "https://github.com/acme/app", Branch: "main", Builder: "dockerfile", Port: 3000,
		StartCommand: "serve A",
	}
	pinned := desiredAppReleaseIdentity(executing)
	newer := executing
	newer.StartCommand = "serve B"
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Generation:  3,
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "3"},
		},
		Spec: newer,
		Status: appv1alpha1.AppStatus{
			Phase:                      phase,
			ArtifactFingerprint:        pinned.artifact,
			ReleaseArtifactFingerprint: pinned.artifact,
			ReleaseFingerprint:         pinned.release,
			ReleaseGeneration:          2,
			ActiveRevision:             activeRevision,
			ArtifactImage:              "registry.example/app:gen-2",
			Image:                      "registry.example/app:gen-2",
		},
	}
}

// Render's Wait overlap policy: the deploy in progress finishes, then the most
// recently triggered one proceeds. A repo-backed release stays pinned through
// its rollout until it serves, so the release whose pods take traffic is the
// one status.activeRevision and the deploy history report. It lets the newer
// release proceed as soon as it serves, fails, sleeps or is canceled.
func TestPendingReleaseWaitsUntilTheReleaseInFlightServes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		app    func() *appv1alpha1.App
		pinned bool
	}{
		{"building over a served release", func() *appv1alpha1.App { return overlapApp(appv1alpha1.PhaseBuilding, "rev-1") }, true},
		{"rolling out over a served release", func() *appv1alpha1.App { return overlapApp(appv1alpha1.PhaseDeploying, "rev-1") }, true},
		{"rolling out a first release", func() *appv1alpha1.App { return overlapApp(appv1alpha1.PhaseDeploying, "") }, true},
		{"served", func() *appv1alpha1.App { return overlapApp(appv1alpha1.PhaseRunning, "rev-2") }, false},
		{"rollout failed over the prior release", func() *appv1alpha1.App { return overlapApp(appv1alpha1.PhaseRunning, "rev-1") }, false},
		{"first release failed", func() *appv1alpha1.App { return overlapApp(appv1alpha1.PhaseFailed, "") }, false},
		{"parked asleep", func() *appv1alpha1.App { return overlapApp(appv1alpha1.PhaseHibernated, "rev-1") }, false},
		{"its own deploy canceled", func() *appv1alpha1.App {
			app := overlapApp(appv1alpha1.PhaseDeploying, "rev-1")
			app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "2"
			return app
		}, false},
		{"rolling out a rollback a newer deploy deselected", func() *appv1alpha1.App {
			// A normal deploy clears spec.releaseConfig while the rollback it
			// replaces still rolls out; that release was never a source build.
			app := overlapApp(appv1alpha1.PhaseDeploying, "rev-1")
			app.Status.SelectedReleaseGeneration = 2
			return app
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := tc.app()
			decision := prepareAppReleaseDecision(app, nil)
			if tc.pinned {
				if app.Status.ReleaseGeneration != 2 || app.Status.PendingReleaseGeneration != 3 || decision.releaseChanged {
					t.Fatalf("release=%d pending=%d changed=%v, want release 2 pinned with 3 pending",
						app.Status.ReleaseGeneration, app.Status.PendingReleaseGeneration, decision.releaseChanged)
				}
				return
			}
			if app.Status.ReleaseGeneration != 3 || app.Status.PendingReleaseGeneration != 0 || !decision.releaseChanged {
				t.Fatalf("release=%d pending=%d changed=%v, want release 3 adopted",
					app.Status.ReleaseGeneration, app.Status.PendingReleaseGeneration, decision.releaseChanged)
			}
		})
	}
}

// imageOverlapApp is a saved-image App whose generation-2 deploy is in flight
// in phase, with a redeploy of the same image (generation 3) requested behind
// it: only spec.restartedAt differs, as on every API, hook or dashboard
// trigger of an image-backed service.
func imageOverlapApp(phase appv1alpha1.AppPhase, activeRevision string) *appv1alpha1.App {
	executing := appv1alpha1.AppSpec{Image: "docker.io/library/busybox:1.37", Port: 8080, RestartedAt: "2026-11-13T11:47:30Z"}
	pinned := desiredAppReleaseIdentity(executing)
	newer := executing
	newer.RestartedAt = "2026-11-13T11:47:40Z"
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Generation:  3,
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "3"},
		},
		Spec: newer,
		Status: appv1alpha1.AppStatus{
			Phase:                      phase,
			ArtifactFingerprint:        pinned.artifact,
			ReleaseArtifactFingerprint: pinned.artifact,
			ReleaseFingerprint:         pinned.release,
			ReleaseGeneration:          2,
			ActiveRevision:             activeRevision,
			Image:                      executing.Image,
		},
	}
}

// Render's Wait overlap policy covers every runtime (w4/635): a prebuilt
// image's deploy that is already rolling out finishes before the newer one
// starts, where it used to be replaced and its row canceled. A deploy the newer
// trigger reaches before the operator adopted it never started, so it is still
// replaced, and a selected release (Rollback, Restart, an image override) still
// replaces or is replaced as before.
func TestImageReleaseRollingOutWaitsForTheNewerTrigger(t *testing.T) {
	for _, tc := range []struct {
		name   string
		app    func() *appv1alpha1.App
		pinned bool
	}{
		{"rolling out over a served release", func() *appv1alpha1.App { return imageOverlapApp(appv1alpha1.PhaseDeploying, "rev-1") }, true},
		{"rolling out a first release", func() *appv1alpha1.App { return imageOverlapApp(appv1alpha1.PhaseDeploying, "") }, true},
		{"served", func() *appv1alpha1.App { return imageOverlapApp(appv1alpha1.PhaseRunning, "rev-2") }, false},
		{"rollout failed", func() *appv1alpha1.App { return imageOverlapApp(appv1alpha1.PhaseFailed, "rev-1") }, false},
		{"not adopted before the newer trigger", func() *appv1alpha1.App {
			app := imageOverlapApp(appv1alpha1.PhaseRunning, "rev-1")
			app.Status.ReleaseGeneration = 1
			return app
		}, false},
		{"its own deploy canceled", func() *appv1alpha1.App {
			app := imageOverlapApp(appv1alpha1.PhaseDeploying, "rev-1")
			app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "2"
			return app
		}, false},
		{"rolling out a selected release", func() *appv1alpha1.App {
			app := imageOverlapApp(appv1alpha1.PhaseDeploying, "rev-1")
			app.Status.SelectedReleaseGeneration = 2
			return app
		}, false},
		{"the newer trigger overrides the image", func() *appv1alpha1.App {
			app := imageOverlapApp(appv1alpha1.PhaseDeploying, "rev-1")
			app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 3, Image: "docker.io/library/busybox:1.36"}
			return app
		}, false},
		{"suspended", func() *appv1alpha1.App {
			app := imageOverlapApp(appv1alpha1.PhaseDeploying, "rev-1")
			app.Spec.Suspended = true
			return app
		}, false},
		{"the workspace policy is Override", func() *appv1alpha1.App {
			app := imageOverlapApp(appv1alpha1.PhaseDeploying, "rev-1")
			app.Labels = map[string]string{appv1alpha1.LabelOverlappingDeployPolicy: appv1alpha1.OverlappingDeployPolicyOverride}
			return app
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := tc.app()
			decision := prepareAppReleaseDecision(app, nil)
			if tc.pinned {
				if app.Status.ReleaseGeneration != 2 || app.Status.PendingReleaseGeneration != 3 || decision.releaseChanged || !decision.pinned {
					t.Fatalf("release=%d pending=%d changed=%v pinned=%v, want release 2 pinned with 3 pending",
						app.Status.ReleaseGeneration, app.Status.PendingReleaseGeneration, decision.releaseChanged, decision.pinned)
				}
				return
			}
			if app.Status.ReleaseGeneration != 3 || app.Status.PendingReleaseGeneration != 0 || !decision.releaseChanged {
				t.Fatalf("release=%d pending=%d changed=%v, want release 3 adopted",
					app.Status.ReleaseGeneration, app.Status.PendingReleaseGeneration, decision.releaseChanged)
			}
		})
	}
}

// Render's Override policy (w4/653): a source build in flight is replaced by
// the newer release at once, where Wait pins it.
func TestOverridePolicyReplacesTheBuildInFlight(t *testing.T) {
	waiting := overlapApp(appv1alpha1.PhaseBuilding, "rev-1")
	if decision := prepareAppReleaseDecision(waiting, nil); !decision.pinned || waiting.Status.ReleaseGeneration != 2 {
		t.Fatalf("Wait: pinned=%v release=%d, want release 2 pinned", decision.pinned, waiting.Status.ReleaseGeneration)
	}
	override := overlapApp(appv1alpha1.PhaseBuilding, "rev-1")
	override.Labels = map[string]string{appv1alpha1.LabelOverlappingDeployPolicy: appv1alpha1.OverlappingDeployPolicyOverride}
	decision := prepareAppReleaseDecision(override, nil)
	if decision.pinned || !decision.releaseChanged || override.Status.ReleaseGeneration != 3 || override.Status.PendingReleaseGeneration != 0 {
		t.Fatalf("Override: pinned=%v changed=%v release=%d pending=%d, want release 3 adopted at once",
			decision.pinned, decision.releaseChanged, override.Status.ReleaseGeneration, override.Status.PendingReleaseGeneration)
	}
}

// Adopting a release that spec.releaseConfig selects records it, so a later
// deploy that clears the selection cannot make it read as a source build.
func TestAdoptingASelectedReleaseRecordsIt(t *testing.T) {
	app := overlapApp(appv1alpha1.PhaseRunning, "rev-2")
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 3, SourceGeneration: 1, Image: "registry.example/app:gen-1"}
	prepareAppReleaseDecision(app, nil)
	if app.Status.ReleaseGeneration != 3 || app.Status.SelectedReleaseGeneration != 3 {
		t.Fatalf("release=%d selected=%d, want rollback release 3 recorded as selected",
			app.Status.ReleaseGeneration, app.Status.SelectedReleaseGeneration)
	}
	plain := overlapApp(appv1alpha1.PhaseRunning, "rev-2")
	prepareAppReleaseDecision(plain, nil)
	if plain.Status.SelectedReleaseGeneration != 0 {
		t.Fatalf("a source release recorded selected=%d", plain.Status.SelectedReleaseGeneration)
	}
}

// A source update saved while a release is still in flight is not a deploy:
// the release keeps resolving and rolling its own artifact, never the
// previous release's image under its own revision (w8/124).
func TestSourceUpdateDuringAReleaseInFlightKeepsItsOwnArtifact(t *testing.T) {
	prior := appv1alpha1.AppSpec{Repo: "https://github.com/acme/app", Branch: "main", Builder: "dockerfile", Port: 3000, StartCommand: "serve 1"}
	inFlight := prior
	inFlight.StartCommand, inFlight.BuildCommand = "serve 2", "make"
	priorID, inFlightID := desiredAppReleaseIdentity(prior), desiredAppReleaseIdentity(inFlight)
	saved := inFlight
	saved.Repo = "https://github.com/acme/fork"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 3, Annotations: map[string]string{
			appv1alpha1.AnnotationReleaseGeneration:       "2",
			appv1alpha1.AnnotationPendingSourceGeneration: "3",
		}},
		Spec: saved,
		Status: appv1alpha1.AppStatus{
			Phase:                      appv1alpha1.PhaseBuilding,
			ArtifactFingerprint:        priorID.artifact,
			ReleaseArtifactFingerprint: inFlightID.artifact,
			ReleaseFingerprint:         inFlightID.release,
			ReleaseGeneration:          2,
			ActiveRevision:             "rev-1",
			ArtifactImage:              "registry.example/app:gen-1",
			Image:                      "registry.example/app:gen-1",
		},
	}
	// Not dispatched yet: a build would read the saved source, so the pending
	// source keeps deciding (w8/126).
	if decision := prepareAppReleaseDecision(app, nil); !decision.sourcePending {
		t.Fatalf("undispatched build: decision = %+v, want the pending source to decide", decision)
	}

	app.Status.BuildRun = &appv1alpha1.BuildRunStatus{Generation: 2, StartedAt: "2026-10-19T06:00:00Z"}
	decision := prepareAppReleaseDecision(app, nil)
	if decision.sourcePending || !decision.artifactChanged || decision.desired.artifact != inFlightID.artifact {
		t.Fatalf("building: decision = %+v, want release 2's own artifact still to build", decision)
	}
	if image, resolved := reusableArtifactImage(app, decision); resolved {
		t.Fatalf("building: reused %q, want release 2's artifact built, not the prior image", image)
	}

	app.Status.Phase = appv1alpha1.PhaseDeploying
	app.Status.ArtifactFingerprint, app.Status.ArtifactImage = inFlightID.artifact, "registry.example/app:gen-2"
	decision = prepareAppReleaseDecision(app, nil)
	if image, _ := reusableArtifactImage(app, decision); decision.sourcePending || image != "registry.example/app:gen-2" {
		t.Fatalf("rolling out: decision = %+v image %q, want release 2's own image", decision, image)
	}
}
