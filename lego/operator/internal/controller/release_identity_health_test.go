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
	"context"
	"errors"
	"strconv"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestHealthModeIsReleaseIdentity pins w4/m191: an empty health path (TCP
// check) and "/" (HTTP GET /) project different probes, so a switch between
// them must be a release of its own — in the operator's fingerprint and in
// the backend's deploy-history gate alike — while the artifact stays the same.
// The release-v1 identity folded empty into "/", so the TCP <-> "/" rows of
// this table failed against it.
func TestHealthModeIsReleaseIdentity(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		rolls    bool
	}{
		{"", "/", true},
		{"/", "", true},
		{"", "/healthz", true},
		{"/healthz", "/", true},
		{"/healthz", "", true},
		{"", "", false},
		{"/", "/", false},
		{"/healthz", "/healthz", false},
	} {
		t.Run(strconv.Quote(tc.from)+"->"+strconv.Quote(tc.to), func(t *testing.T) {
			before := appv1alpha1.AppSpec{Image: "example:v1", Port: 3000, HealthCheckPath: tc.from}
			after := before
			after.HealthCheckPath = tc.to
			was, now := desiredAppReleaseIdentity(before), desiredAppReleaseIdentity(after)
			if was.artifact != now.artifact {
				t.Fatal("a health-mode change must reuse the artifact")
			}
			if operatorRolls := was.release != now.release; operatorRolls != tc.rolls {
				t.Fatalf("operator rolls = %v, want %v", operatorRolls, tc.rolls)
			}
			if backendRolls := appv1alpha1.SpecRollsRelease(before, after); backendRolls != tc.rolls {
				t.Fatalf("SpecRollsRelease = %v, want %v", backendRolls, tc.rolls)
			}
			if releaseSettingsFingerprint(before) != releaseSettingsFingerprint(after) != tc.rolls {
				t.Fatal("the settings fingerprint must follow the release identity")
			}
		})
	}
}

// TestHealthModeSwitchAdvancesToTheOwedGeneration drives the decision for the
// observed journey: a served TCP release, then a Settings save of "/" that
// bex-api stamped as release generation 4 (after an operational scale moved
// metadata.generation past it). The switch must adopt generation 4, keep the
// served artifact image, and do the same for the reverse switch.
func TestHealthModeSwitchAdvancesToTheOwedGeneration(t *testing.T) {
	for _, tc := range []struct{ from, to string }{{"", "/"}, {"/", ""}} {
		t.Run(strconv.Quote(tc.from)+"->"+strconv.Quote(tc.to), func(t *testing.T) {
			served := appv1alpha1.AppSpec{Image: "example:v1", Port: 3000, HealthCheckPath: tc.from}
			identity := desiredAppReleaseIdentity(served)
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Generation: 4,
					Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "4"}},
				Spec: served,
				Status: appv1alpha1.AppStatus{
					Phase: appv1alpha1.PhaseRunning, Image: "example:v1", ArtifactImage: "example:v1",
					ArtifactFingerprint: identity.artifact, ReleaseFingerprint: identity.release,
					ReleaseArtifactFingerprint: identity.artifact,
					ReleaseGeneration:          2, ActiveRevision: "rev-2", ObservedGeneration: 3,
				},
			}
			app.Spec.HealthCheckPath = tc.to

			decision := prepareAppReleaseDecision(app)
			if !decision.releaseChanged || decision.artifactChanged {
				t.Fatalf("decision = %+v, want a release change on the same artifact", decision)
			}
			if app.Status.ReleaseGeneration != 4 || releaseRevision(app) != "rev-4" {
				t.Fatalf("release generation = %d, want the stamped generation 4", app.Status.ReleaseGeneration)
			}
			if image, ok := reusableArtifactImage(app, decision); !ok || image != "example:v1" {
				t.Fatalf("reusable artifact = %q (%v), want the served image", image, ok)
			}
			// The next pass is a no-op: the generation is now adopted.
			if again := prepareAppReleaseDecision(app); again.releaseChanged || app.Status.ReleaseGeneration != 4 {
				t.Fatalf("second pass = %+v gen %d, want a settled release", again, app.Status.ReleaseGeneration)
			}
		})
	}
}

const legacyAdoptNS = "tea-m191"

// legacyHealthApp is an App whose last release (generation 2) was classified
// by a release-v1 operator from servedPath, and whose metadata.generation has
// since moved to 5 through operational edits — so failing to adopt it would
// roll a fresh rev-5.
func legacyHealthApp(servedPath string) *appv1alpha1.App {
	spec := appv1alpha1.AppSpec{Image: "example:v1", Port: 3000, Replicas: 1, HealthCheckPath: servedPath}
	identity := desiredAppReleaseIdentity(spec)
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: legacyAdoptNS, UID: "api-uid", Generation: 5,
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}},
		Spec: spec,
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning, Image: "example:v1", ArtifactImage: "example:v1",
			ArtifactFingerprint: identity.artifact, ReleaseArtifactFingerprint: identity.artifact,
			ReleaseFingerprint: legacyReleaseFingerprint(spec),
			ReleaseGeneration:  2, ActiveRevision: "rev-2", ObservedGeneration: 5,
		},
	}
}

type legacyEvidence int

const (
	evidenceNone legacyEvidence = iota
	evidenceRecord
	evidenceDeployment
)

// legacyAdoptionReconciler serves app's applied evidence for generation 2
// with appliedPath: as its release record, or (pre-record) only as the owned
// Deployment template at rev-2.
func legacyAdoptionReconciler(t *testing.T, app *appv1alpha1.App, appliedPath string, evidence legacyEvidence) *AppReconciler {
	t.Helper()
	sch := deletionScheme(t)
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(app.DeepCopy()).Build()
	r := &AppReconciler{Client: cl, Scheme: sch, Mode: ModeKubernetes}
	applied := app.DeepCopy()
	applied.Spec.HealthCheckPath = appliedPath
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	applyDeploymentSpec(dep, applied, deploymentParams{image: "example:v1", port: 3000, replicas: 1})
	switch evidence {
	case evidenceRecord:
		rec := &runtimeConfigRecord{spec: releaseRecordSpec(applied), template: dep.Spec.Template}
		if err := r.writeRuntimeConfigRecord(t.Context(), applied, 2, rec); err != nil {
			t.Fatal(err)
		}
	case evidenceDeployment:
		if err := controllerutil.SetControllerReference(app, dep, sch); err != nil {
			t.Fatal(err)
		}
		if err := cl.Create(t.Context(), dep); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// TestLegacyReleaseIdentityAdoption is t002's compatibility contract: a
// release-v1 status adopts onto release-v2 with no new generation when the
// served release is what spec still describes, and a genuine TCP <-> "/"
// switch — stamped by the backend or made directly on the CR — still rolls.
func TestLegacyReleaseIdentityAdoption(t *testing.T) {
	for _, tc := range []struct {
		name              string
		served, saved     string
		evidence          legacyEvidence
		stamp             string
		wantRelease       bool
		wantGeneration    int64
		mutate            func(*appv1alpha1.App)
		failEvidenceReads bool
	}{
		{name: "unchanged TCP, record", served: "", saved: "", evidence: evidenceRecord, wantGeneration: 2},
		{name: "unchanged root, record", served: "/", saved: "/", evidence: evidenceRecord, wantGeneration: 2},
		{name: "unchanged TCP, pre-record template", served: "", saved: "", evidence: evidenceDeployment, wantGeneration: 2},
		{name: "unchanged TCP, nothing applied yet", served: "", saved: "", evidence: evidenceNone, wantGeneration: 2},
		{name: "non-root path needs no evidence", served: "/healthz", saved: "/healthz", evidence: evidenceNone,
			failEvidenceReads: true, wantGeneration: 2},
		{name: "owed root -> TCP switch", served: "/", saved: "", evidence: evidenceRecord, stamp: "6",
			wantRelease: true, wantGeneration: 6},
		{name: "owed TCP -> root switch", served: "", saved: "/", evidence: evidenceRecord, stamp: "6",
			wantRelease: true, wantGeneration: 6},
		{name: "direct CR TCP -> root edit", served: "", saved: "/", evidence: evidenceRecord,
			wantRelease: true, wantGeneration: 5},
		{name: "direct CR root -> TCP edit, pre-record template", served: "/", saved: "", evidence: evidenceDeployment,
			wantRelease: true, wantGeneration: 5},
		{name: "another changed input is a normal release", served: "", saved: "", evidence: evidenceRecord, stamp: "6",
			mutate: func(a *appv1alpha1.App) { a.Spec.Image = "example:v2" }, wantRelease: true, wantGeneration: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := legacyHealthApp(tc.served)
			r := legacyAdoptionReconciler(t, app, tc.served, tc.evidence)
			if tc.failEvidenceReads {
				r.Client = interceptor.NewClient(r.Client.(client.WithWatch), interceptor.Funcs{
					Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
						return errors.New("adoption read evidence it does not need")
					},
				})
			}
			app.Spec.HealthCheckPath = tc.saved
			if tc.stamp != "" {
				app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = tc.stamp
			}
			if tc.mutate != nil {
				tc.mutate(app)
			}

			if err := r.adoptLegacyReleaseIdentity(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			decision := prepareAppReleaseDecision(app)
			if decision.releaseChanged != tc.wantRelease || app.Status.ReleaseGeneration != tc.wantGeneration {
				t.Fatalf("decision = %+v gen %d, want release=%v gen %d",
					decision, app.Status.ReleaseGeneration, tc.wantRelease, tc.wantGeneration)
			}
			if app.Status.ReleaseFingerprint != desiredAppReleaseIdentity(app.Spec).release {
				t.Fatal("status must carry the release-v2 identity after this pass")
			}
		})
	}
}

// TestLegacyAdoptionKeepsAnActiveRollback: a rollback runs its recorded path,
// not the saved one, and any later path edit restamps past the selector, so
// the saved path is still the one the legacy fingerprint encoded.
func TestLegacyAdoptionKeepsAnActiveRollback(t *testing.T) {
	app := legacyHealthApp("")
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 2, SourceGeneration: 1, Image: "example:v0"}
	app.Status.ReleaseFingerprint = legacyReleaseFingerprint(app.Spec)
	r := legacyAdoptionReconciler(t, app, "/", evidenceRecord) // the historical release's path
	if err := r.adoptLegacyReleaseIdentity(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	if decision := prepareAppReleaseDecision(app); decision.releaseChanged || app.Status.ReleaseGeneration != 2 {
		t.Fatalf("decision = %+v gen %d, want the rollback kept at generation 2", decision, app.Status.ReleaseGeneration)
	}
}

// TestLegacyAdoptionLeavesACanceledReleaseAlone: the canceled branch settles
// on the served generation without reading the fingerprint, so adoption must
// not judge the canceled release's identity against the served record.
func TestLegacyAdoptionLeavesACanceledReleaseAlone(t *testing.T) {
	app := legacyHealthApp("")
	app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "2"
	legacy := app.Status.ReleaseFingerprint
	r := legacyAdoptionReconciler(t, app, "", evidenceRecord)
	if err := r.adoptLegacyReleaseIdentity(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	if app.Status.ReleaseFingerprint != legacy {
		t.Fatal("a canceled latest release must keep its fingerprint for normal classification")
	}
}

// TestRecordedSettingsMatchAcceptsLegacyRecords keeps w4/m165's cancel answer
// across the identity version: a release-v1 record of identical settings is
// still identical, but only when its own recorded path proves which side of
// the TCP/"/" fold it ran.
func TestRecordedSettingsMatchAcceptsLegacyRecords(t *testing.T) {
	tcp := appv1alpha1.AppSpec{Image: "example:v1", Port: 3000, RestartedAt: "2026-10-10T00:00:00Z"}
	root := tcp
	root.HealthCheckPath = "/"
	legacyTCP := legacyReleaseFingerprint(settingsIdentitySpec(tcp))
	for _, tc := range []struct {
		name string
		rec  appv1alpha1.ReleaseRecordSpec
		spec appv1alpha1.AppSpec
		want bool
	}{
		{"current record", appv1alpha1.ReleaseRecordSpec{SettingsFingerprint: releaseSettingsFingerprint(tcp)}, tcp, true},
		{"current record, other mode", appv1alpha1.ReleaseRecordSpec{SettingsFingerprint: releaseSettingsFingerprint(tcp)}, root, false},
		{"legacy record, same path", appv1alpha1.ReleaseRecordSpec{SettingsFingerprint: legacyTCP, HealthCheckPath: new("")}, tcp, true},
		{"legacy record, folded path", appv1alpha1.ReleaseRecordSpec{SettingsFingerprint: legacyTCP, HealthCheckPath: new("")}, root, false},
		{"legacy record, no path", appv1alpha1.ReleaseRecordSpec{SettingsFingerprint: legacyTCP}, tcp, false},
		{"no fingerprint", appv1alpha1.ReleaseRecordSpec{HealthCheckPath: new("")}, tcp, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := recordedSettingsMatch(tc.rec, tc.spec); got != tc.want {
				t.Fatalf("recordedSettingsMatch = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestProbedHealthCheckPathInvertsTheProjection keeps the evidence reader and
// healthCheckHandler in step for both modes and for a probe-less worker.
func TestProbedHealthCheckPathInvertsTheProjection(t *testing.T) {
	for _, path := range []string{"", "/", "/healthz"} {
		c := corev1.Container{ReadinessProbe: &corev1.Probe{ProbeHandler: healthCheckHandler(path, 3000, "api.example")}}
		if got := probedHealthCheckPath(c); got == nil || *got != path {
			t.Fatalf("probedHealthCheckPath(%q) = %v", path, got)
		}
	}
	if got := probedHealthCheckPath(corev1.Container{}); got != nil {
		t.Fatalf("a container without a probe has no applied path, got %q", *got)
	}
}
