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
	"encoding/json"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// selectedReleaseFixture is an image service whose release 3, a Restart or
// rollback, selects release 1 (image:A, echo A, /A, two instances, literal and
// Secret values A) while the saved spec holds B. Each of mutate applies to the
// saved spec and to release 1's, after release 1's own values.
func selectedReleaseFixture(t *testing.T, mutate ...func(*appv1alpha1.AppSpec)) (*AppReconciler, *appv1alpha1.App) {
	t.Helper()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: snapshotTestNS, UID: "api-uid", Generation: 3,
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "3"}},
		Spec: appv1alpha1.AppSpec{Image: "image:B", StartCommand: "echo B", HealthCheckPath: "/B", Replicas: 3, Tier: "standard", Port: 3000,
			Env: []appv1alpha1.EnvVar{{Name: "LITERAL", Value: "B"}}, EnvFromSecret: "api-env", EnvFromSecrets: []string{"evg-shared-env"}, FilesFromSecrets: []string{"api-files"},
			ReleaseConfig: &appv1alpha1.ReleaseConfigReference{Generation: 3, SourceGeneration: 1, Image: "image:A"}},
		Status: appv1alpha1.AppStatus{ReleaseGeneration: 3, ConfigSnapshotGeneration: 2, ActiveRevision: "rev-2", Image: "image:B"},
	}
	for _, m := range mutate {
		m(&app.Spec)
	}
	scheme := deletionScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(app).WithObjects(app,
		secret("api-env", "A"), secret("api-files", "file-A"), secret("evg-shared-env", "group-A")).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes}
	target := app.DeepCopy()
	target.Spec.ReleaseConfig = nil
	target.Spec.Image = "image:A"
	target.Spec.StartCommand = "echo A"
	target.Spec.HealthCheckPath = "/A"
	target.Spec.Replicas = 2
	target.Spec.Env[0].Value = "A"
	for _, m := range mutate {
		m(&target.Spec)
	}
	target.Status.ReleaseGeneration = 1
	target.Status.ConfigSnapshotGeneration = 1
	for _, source := range releaseConfigSources(target) {
		if err := r.copyConfigSecret(context.Background(), target, source, 1); err != nil {
			t.Fatal(err)
		}
	}
	dep := &appsv1.Deployment{}
	applyDeploymentSpec(dep, target, deploymentParams{image: "image:A", port: 3000, replicas: 2})
	if err := r.recordReleasePodTemplate(context.Background(), target, dep.Spec.Template); err != nil {
		t.Fatal(err)
	}
	setSelectedTestSecret(t, r, "api-env", "B")
	setSelectedTestSecret(t, r, "api-files", "file-B")
	setSelectedTestSecret(t, r, "evg-shared-env", "group-B")
	return r, app
}

func setSelectedTestSecret(t *testing.T, r *AppReconciler, name, value string) {
	t.Helper()
	s := &corev1.Secret{}
	if err := r.Get(context.Background(), client.ObjectKey{Namespace: snapshotTestNS, Name: name}, s); err != nil {
		t.Fatal(err)
	}
	s.Data = map[string][]byte{"MESSAGE": []byte(value)}
	if err := r.Update(context.Background(), s); err != nil {
		t.Fatal(err)
	}
}

func selectedTestValue(t *testing.T, r *AppReconciler, name string) string {
	t.Helper()
	s := &corev1.Secret{}
	if err := r.Get(context.Background(), client.ObjectKey{Namespace: snapshotTestNS, Name: name}, s); err != nil {
		t.Fatal(err)
	}
	return string(s.Data["MESSAGE"])
}

func materializeSelectedTestRelease(t *testing.T, r *AppReconciler, app *appv1alpha1.App) *appsv1.Deployment {
	t.Helper()
	ctx := context.Background()
	if err := r.ensureReleaseConfigSnapshot(ctx, app); err != nil {
		t.Fatal(err)
	}
	runtime, err := r.selectedRuntimeApp(ctx, app)
	if err != nil {
		t.Fatal(err)
	}
	image, ok := reusableArtifactImage(app, appReleaseDecision{})
	if !ok {
		t.Fatal("selected release has no artifact")
	}
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if err := r.applyServingDeployment(ctx, app, dep, deploymentParams{image: image, port: 3000, replicas: effectiveReplicas(runtime)}, nil); err != nil {
		t.Fatal(err)
	}
	// Publish the serving revision before comparing status: dispatch alone is
	// not proof the replacement is live.
	app.Status.ActiveRevision = releaseRevision(app)
	app.Status.Image = image
	app.Status.UndeployedChanges, _, err = r.servingConfigurationDiffers(ctx, app)
	if err != nil {
		t.Fatal(err)
	}
	if err := updateStatusIfChanged(ctx, r.Client, app); err != nil {
		t.Fatal(err)
	}
	return dep
}

func TestRollbackSelectionPreservesSavedConfiguration(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	dep := materializeSelectedTestRelease(t, r, app)
	c := dep.Spec.Template.Spec.Containers[0]
	if c.Image != "image:A" || strings.Join(c.Command, " ") != "echo A" {
		t.Fatalf("runtime image/command = %s %v", c.Image, c.Command)
	}
	if c.ReadinessProbe.HTTPGet.Path != "/A" || *dep.Spec.Replicas != 2 {
		t.Fatalf("runtime health/count = %s %d", c.ReadinessProbe.HTTPGet.Path, *dep.Spec.Replicas)
	}
	for name, want := range map[string]string{"api-env-r3": "A", "api-files-r3": "file-A", "api-evg-shared-env-r3": "group-B", "api-env": "B", "api-files": "file-B", "evg-shared-env": "group-B"} {
		if got := selectedTestValue(t, r, name); got != want {
			t.Errorf("%s=%q, want %q", name, got, want)
		}
	}
	saved := &appv1alpha1.App{}
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(app), saved); err != nil {
		t.Fatal(err)
	}
	if saved.Spec.StartCommand != "echo B" || saved.Spec.Image != "image:B" || saved.Spec.HealthCheckPath != "/B" || saved.Spec.Replicas != 3 {
		t.Fatalf("saved state changed: %+v", saved.Spec)
	}
	if !app.Status.UndeployedChanges {
		t.Fatal("saved-vs-running divergence hidden")
	}
	rec, err := r.readRuntimeConfigRecord(context.Background(), app, 3)
	if err != nil {
		t.Fatal(err)
	}
	if rec.spec.StartCommand != "echo A" {
		t.Fatal("new record contains saved B instead of runtime A")
	}
	// Reconciliation and controller restart consume the newly materialized record,
	// even after the original source record and its snapshots are reclaimed.
	for _, name := range []string{appv1alpha1.ReleaseRecordName(app.Name, 1), "api-env-r1", "api-files-r1", "api-evg-shared-env-r1"} {
		if err := r.Delete(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}); err != nil {
			t.Fatal(err)
		}
	}
	r = &AppReconciler{Client: r.Client, Scheme: r.Scheme, Mode: ModeKubernetes}
	if again := materializeSelectedTestRelease(t, r, app); strings.Join(again.Spec.Template.Spec.Containers[0].Command, " ") != "echo A" {
		t.Fatal("reconciliation lost the selected command")
	}
}

func TestRestartSelectionRetainsRunningGroupsThenNormalDeployUsesSavedValues(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	materializeSelectedTestRelease(t, r, app)
	setSelectedTestSecret(t, r, "evg-shared-env", "group-C")
	app.Generation = 4
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "4"
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 4, SourceGeneration: 3, Image: "image:A", PreserveGroupValues: true}
	app.Status.ReleaseGeneration = 4
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	app.Status.ReleaseGeneration = 4
	materializeSelectedTestRelease(t, r, app)
	if got := selectedTestValue(t, r, "api-evg-shared-env-r4"); got != "group-B" {
		t.Fatalf("restart picked up new group values: %s", got)
	}
	if got := selectedTestValue(t, r, "api-env-r4"); got != "A" {
		t.Fatalf("restart picked up saved B: %s", got)
	}
	// A generic config deployment expires the generation-bound override even
	// before a backend clears the stale reference field.
	app.Generation = 5
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "5"
	app.Status.ReleaseGeneration = 5
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	app.Status.ReleaseGeneration = 5
	dep := materializeSelectedTestRelease(t, r, app)
	c := dep.Spec.Template.Spec.Containers[0]
	if c.Image != "image:B" || strings.Join(c.Command, " ") != "echo B" {
		t.Fatalf("normal deploy retained rollback: %s %v", c.Image, c.Command)
	}
	if got := selectedTestValue(t, r, "api-env-r5"); got != "B" {
		t.Fatalf("normal deploy env=%q", got)
	}
	if app.Status.UndeployedChanges {
		t.Fatal("normal deploy retained undeployed flag")
	}
}

func TestSelectedConfigurationMissingSnapshotFailsClosed(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	materializeSelectedTestRelease(t, r, app)
	if err := r.Delete(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "api-env-r3", Namespace: app.Namespace}}); err != nil {
		t.Fatal(err)
	}
	if err := r.ensureReleaseConfigSnapshot(context.Background(), app); err == nil || !strings.Contains(err.Error(), "materialized configuration") {
		t.Fatalf("missing selected snapshot error=%v", err)
	}
}

func TestSelectedConfigurationChecksSourcesOncePerRuntimeReconcile(t *testing.T) {
	for _, serviceType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeCronJob} {
		t.Run(serviceType, func(t *testing.T) {
			r, app := selectedReleaseFixture(t)
			materializeSelectedTestRelease(t, r, app)
			app.Spec.Type = serviceType
			app.Spec.Schedule = "0 0 1 1 *"
			if err := r.Update(context.Background(), app); err != nil {
				t.Fatal(err)
			}
			reads := map[string]int{"api-env": 0, "api-env-r3": 0, "api-files": 0, "api-files-r3": 0, "evg-shared-env": 0, "api-evg-shared-env-r3": 0}
			r.Client = interceptor.NewClient(r.Client.(client.WithWatch), interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, tracked := reads[key.Name]; tracked {
						reads[key.Name]++
					}
					return c.Get(ctx, key, obj, opts...)
				},
			})
			if _, err := r.reconcileKubernetes(context.Background(), app, "image:A", 3000); err != nil {
				t.Fatal(err)
			}
			for name, count := range reads {
				want := 0
				if appv1alpha1.IsReleaseSnapshotName(name) {
					want = 1
				}
				if count != want {
					t.Errorf("%s fetched %d times in one runtime pass, want %d for snapshot validation", name, count, want)
				}
			}
			// Validation is bounded within a pass, not cached across passes. Losing
			// the optional env snapshot must still stop the next reconciliation.
			if err := r.Delete(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "api-env-r3", Namespace: app.Namespace}}); err != nil {
				t.Fatal(err)
			}
			if _, err := r.reconcileKubernetes(context.Background(), app, "image:A", 3000); err == nil || !strings.Contains(err.Error(), "materialized configuration") {
				t.Fatalf("next pass ignored a missing selected snapshot: %v", err)
			}
		})
	}
}

func TestRollbackSelectionSkipsDeletedGroups(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	if err := r.Delete(context.Background(), secret("evg-shared-env", "")); err != nil {
		t.Fatal(err)
	}
	dep := materializeSelectedTestRelease(t, r, app)
	for _, from := range dep.Spec.Template.Spec.Containers[0].EnvFrom {
		if strings.Contains(from.SecretRef.Name, "evg-shared") {
			t.Fatal("deleted group remained linked")
		}
	}
}

func TestIdenticalRestartDoesNotReportUndeployedChanges(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	// Runtime A is now also the saved configuration. This ordinary restart has
	// no saved/runtime divergence; merely selecting a record is not a change.
	target, err := r.readRuntimeConfigRecord(context.Background(), app, 1)
	if err != nil {
		t.Fatal(err)
	}
	app.Spec.Image = "image:A"
	app.Spec.StartCommand = target.spec.StartCommand
	app.Spec.HealthCheckPath = *target.spec.HealthCheckPath
	app.Spec.Replicas = *target.spec.Replicas
	app.Spec.Env = target.spec.Env
	app.Spec.ReleaseConfig.PreserveGroupValues = true
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	setSelectedTestSecret(t, r, "api-env", "A")
	setSelectedTestSecret(t, r, "api-files", "file-A")
	setSelectedTestSecret(t, r, "evg-shared-env", "group-A")
	materializeSelectedTestRelease(t, r, app)
	if app.Status.UndeployedChanges {
		t.Fatal("identical restart falsely says settings are undeployed")
	}
}

func TestEnvOnlyRollbackReportsUndeployedChanges(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	app.Spec.Image = "image:A"
	app.Spec.StartCommand = "echo A"
	app.Spec.HealthCheckPath = "/A"
	app.Spec.Replicas = 2
	app.Spec.Env[0].Value = "A"
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	materializeSelectedTestRelease(t, r, app)
	if !app.Status.UndeployedChanges {
		t.Fatal("different saved Secret values were invisible")
	}
}

// A Restart or rollback runs its target's build, so a saved build command,
// branch or root directory that build was not made from waits for the next
// deploy, as Render's next standard deploy "uses the service's current
// configuration" (render.com/docs/rollbacks); the target's put back waits for
// nothing. Render's rollback keeps the current plan, so a plan changed since
// the target is not a change. The selection's record named the saved build
// settings, so a pending one read as deployed and the target's put back as
// undeployed (w32/058).
func TestASelectionKeepsSavedBuildSettingsUndeployed(t *testing.T) {
	builds := map[string]func(*appv1alpha1.AppSpec){
		"a Dockerfile service": func(s *appv1alpha1.AppSpec) {
			s.Repo, s.Image, s.Branch, s.BuildCommand, s.Builder = "https://github.com/bex-co/hello", "", "main", "make a", "dockerfile"
		},
		// A native build bakes its start command and environment in too.
		"a native service": func(s *appv1alpha1.AppSpec) {
			s.Repo, s.Image, s.Branch, s.BuildCommand, s.Runtime = "https://github.com/bex-co/hello", "", "main", "make a", "node"
		},
	}
	// selectTheTarget makes the saved selected settings release 1's, as an
	// identical Restart's are, applies save, and serves release 3.
	selectTheTarget := func(t *testing.T, build func(*appv1alpha1.AppSpec), restart bool, save func(*appv1alpha1.AppSpec)) (*AppReconciler, *appv1alpha1.App) {
		t.Helper()
		r, app := selectedReleaseFixture(t, build)
		target, err := r.readRuntimeConfigRecord(t.Context(), app, 1)
		if err != nil {
			t.Fatal(err)
		}
		app.Spec.StartCommand, app.Spec.HealthCheckPath = target.spec.StartCommand, *target.spec.HealthCheckPath
		app.Spec.Replicas, app.Spec.Env = *target.spec.Replicas, target.spec.Env
		if target.spec.BuildSettingsFingerprint != buildSettingsFingerprint(app.Spec) {
			t.Fatal("precondition: release 1's record names the build settings it was built from")
		}
		app.Spec.ReleaseConfig.PreserveGroupValues = restart
		save(&app.Spec)
		if err := r.Update(t.Context(), app); err != nil {
			t.Fatal(err)
		}
		setSelectedTestSecret(t, r, "api-env", "A")
		setSelectedTestSecret(t, r, "api-files", "file-A")
		setSelectedTestSecret(t, r, "evg-shared-env", "group-A")
		materializeSelectedTestRelease(t, r, app)
		return r, app
	}
	undeployed := func(t *testing.T, r *AppReconciler, app *appv1alpha1.App) bool {
		t.Helper()
		different, _, err := r.servingConfigurationDiffers(t.Context(), app)
		if err != nil {
			t.Fatal(err)
		}
		return different
	}
	for kind, build := range builds {
		for _, tc := range []struct {
			name    string
			restart bool
			save    func(*appv1alpha1.AppSpec)
			want    bool
		}{
			{"an identical Restart", true, func(*appv1alpha1.AppSpec) {}, false},
			{"an identical rollback", false, func(*appv1alpha1.AppSpec) {}, false},
			{"a Restart, a saved build command", true, func(s *appv1alpha1.AppSpec) { s.BuildCommand = "make b" }, true},
			{"a rollback, a saved build command", false, func(s *appv1alpha1.AppSpec) { s.BuildCommand = "make b" }, true},
			{"a Restart, a saved branch", true, func(s *appv1alpha1.AppSpec) { s.Branch = "b" }, true},
			{"a rollback, a saved root directory", false, func(s *appv1alpha1.AppSpec) { s.RootDir = "b" }, true},
			{"a rollback, a plan changed since the target", false, func(s *appv1alpha1.AppSpec) { s.Tier = "pro" }, false},
		} {
			t.Run(kind+", "+tc.name, func(t *testing.T) {
				r, app := selectTheTarget(t, build, tc.restart, tc.save)
				if app.Status.UndeployedChanges != tc.want {
					t.Fatalf("undeployedChanges = %v after the selection served, want %v", app.Status.UndeployedChanges, tc.want)
				}
				target, err := r.readRuntimeConfigRecord(t.Context(), app, 1)
				if err != nil {
					t.Fatal(err)
				}
				rec, err := r.readRuntimeConfigRecord(t.Context(), app, 3)
				if err != nil {
					t.Fatal(err)
				}
				if rec.spec.BuildSettingsFingerprint != target.spec.BuildSettingsFingerprint {
					t.Fatal("the selection's record must name its target's build settings, not the saved ones")
				}
			})
		}

		t.Run(kind+", the target's build settings put back wait for nothing, and a later save waits", func(t *testing.T) {
			r, app := selectTheTarget(t, build, true, func(s *appv1alpha1.AppSpec) { s.BuildCommand = "make b" })
			if !app.Status.UndeployedChanges {
				t.Fatal("precondition: the saved build command waits")
			}
			app.Spec.BuildCommand = "make a"
			if undeployed(t, r, app) {
				t.Fatal("the build command the Restart runs, put back, still reads undeployed")
			}
			app.Spec.Branch = "b"
			if !undeployed(t, r, app) {
				t.Fatal("a saved branch after the Restart reads deployed")
			}
		})

		t.Run(kind+", a Restart of a rollback keeps the rollback target's build settings", func(t *testing.T) {
			r, app := selectTheTarget(t, build, false, func(s *appv1alpha1.AppSpec) { s.BuildCommand = "make b" })
			app.Generation = 4
			app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "4"
			app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 4, SourceGeneration: 3, Image: "image:A", PreserveGroupValues: true}
			if err := r.Update(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			app.Status.ReleaseGeneration = 4
			materializeSelectedTestRelease(t, r, app)
			if !app.Status.UndeployedChanges {
				t.Fatal("a Restart of the rollback reads the saved build command deployed")
			}
			app.Spec.BuildCommand = "make a"
			if undeployed(t, r, app) {
				t.Fatal("the rollback target's build command put back reads undeployed after the Restart")
			}
		})
	}

	t.Run("a target recorded before build settings keeps the saved comparison", func(t *testing.T) {
		r, app := selectedReleaseFixture(t, builds["a Dockerfile service"])
		target, err := r.readRuntimeConfigRecord(t.Context(), app, 1)
		if err != nil {
			t.Fatal(err)
		}
		target.spec.BuildSettingsFingerprint = ""
		if err := r.persistRuntimeConfigRecord(t.Context(), app, 1, target); err != nil {
			t.Fatal(err)
		}
		app.Spec.StartCommand, app.Spec.HealthCheckPath = target.spec.StartCommand, *target.spec.HealthCheckPath
		app.Spec.Replicas, app.Spec.Env, app.Spec.BuildCommand = *target.spec.Replicas, target.spec.Env, "make b"
		app.Spec.ReleaseConfig.PreserveGroupValues = true
		if err := r.Update(t.Context(), app); err != nil {
			t.Fatal(err)
		}
		setSelectedTestSecret(t, r, "api-env", "A")
		setSelectedTestSecret(t, r, "api-files", "file-A")
		setSelectedTestSecret(t, r, "evg-shared-env", "group-A")
		materializeSelectedTestRelease(t, r, app)
		rec, err := r.readRuntimeConfigRecord(t.Context(), app, 3)
		if err != nil {
			t.Fatal(err)
		}
		if rec.spec.BuildSettingsFingerprint != "" || rec.spec.SettingsFingerprint != releaseSettingsFingerprint(app.Spec) {
			t.Fatal("a selection of a record with no build settings must name the saved settings and claim no build")
		}
		if app.Status.UndeployedChanges {
			t.Fatal("with no recorded build settings the comparison is the saved one, as before")
		}
	})
}

// A release held through its rollout records, on a pass that renders the newer
// saved spec, the stamps it was adopted with: its settings, and the build
// settings a Restart or rollback of it runs (w8/m120, w32/058).
func TestAHeldReleaseRecordKeepsItsAdoptedStamps(t *testing.T) {
	adopted := appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/hello", Branch: "main", BuildCommand: "make a", Port: 3000, Replicas: 1}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: snapshotTestNS, UID: "api-uid", Generation: 2},
		Spec:       adopted,
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseBuilding, ReleaseGeneration: 2, ActiveRevision: "rev-1",
			ReleaseFingerprint: desiredAppReleaseIdentity(adopted).release},
	}
	scheme := deletionScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(app).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes}
	if err := r.recordAdoptedRelease(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	app.Spec.BuildCommand, app.Spec.Replicas = "make b", 2
	if !heldRelease(app) {
		t.Fatal("precondition: the saved spec moved on past the release in flight")
	}
	if err := r.recordReleasePodTemplate(t.Context(), app, corev1.PodTemplateSpec{}); err != nil {
		t.Fatal(err)
	}
	rec, err := r.readRuntimeConfigRecord(t.Context(), app, 2)
	if err != nil {
		t.Fatal(err)
	}
	if rec.spec.SettingsFingerprint != releaseSettingsFingerprint(adopted) || *rec.spec.SavedReplicas != 1 {
		t.Fatal("the held release's record took the newer saved settings")
	}
	if rec.spec.BuildSettingsFingerprint != buildSettingsFingerprint(adopted) {
		t.Fatal("the held release's record lost the build settings it was adopted with")
	}
}

// A record that names a spec's settings on their own build, as a Restart that
// runs the saved settings' own build does, matches them; on another build's
// settings it does not. The build settings leave out the start command and
// environment a release applies when it starts the image, so a rerun that runs
// a saved start command or group on the served build still names that build
// (w32/058, w32/060).
func TestSelectedSettingsFingerprintOfTheSavedBuildIsTheSavedSettings(t *testing.T) {
	for name, spec := range map[string]appv1alpha1.AppSpec{
		"image": {Image: "nginx:1", Port: 80, Tier: "starter"},
		"native": {Repo: "https://github.com/bex-co/hello", Runtime: "node", BuildCommand: "npm ci", StartCommand: "node .",
			Env: []appv1alpha1.EnvVar{{Name: "K", Value: "v"}}, EnvFromSecret: "api-env", RestartedAt: "now", BuildCommit: "abc"},
		"docker": {Repo: "https://github.com/bex-co/hello", Branch: "dev", DockerfilePath: "Dockerfile",
			ReleaseConfig: &appv1alpha1.ReleaseConfigReference{Generation: 2, SourceGeneration: 1, Image: "x"}},
	} {
		named := func(built appv1alpha1.AppSpec) appv1alpha1.ReleaseRecordSpec {
			return appv1alpha1.ReleaseRecordSpec{SettingsFingerprint: selectedSettingsFingerprint(spec, buildSettingsFingerprint(built))}
		}
		if !recordedSettingsMatch(named(spec), spec) {
			t.Errorf("%s: its own build's settings do not name the saved ones", name)
		}
		other := spec
		other.BuildCommand = "other"
		if recordedSettingsMatch(named(other), spec) {
			t.Errorf("%s: another build's settings name the saved ones", name)
		}
		started := spec
		started.StartCommand, started.Env, started.EnvFromSecret = "node other.js", nil, ""
		started.EnvFromSecrets, started.FilesFromSecrets = []string{"evg-shared-env"}, []string{"api-files"}
		if buildSettingsFingerprint(started) != buildSettingsFingerprint(spec) {
			t.Errorf("%s: the start command and environment change the build settings", name)
		}
		if recordedSettingsMatch(appv1alpha1.ReleaseRecordSpec{SettingsFingerprint: selectedSettingsFingerprint(started, buildSettingsFingerprint(spec))}, spec) {
			t.Errorf("%s: another start command and environment name the saved ones", name)
		}
	}
}

func TestSelectedReplicaCountAllowsLaterOperationalScale(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	materializeSelectedTestRelease(t, r, app)
	app.Spec.Replicas = 4
	app.Generation = 4
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	dep := materializeSelectedTestRelease(t, r, app)
	if *dep.Spec.Replicas != 4 {
		t.Fatalf("explicit scale was overwritten by historical count: %d", *dep.Spec.Replicas)
	}
	if strings.Join(dep.Spec.Template.Spec.Containers[0].Command, " ") != "echo A" {
		t.Fatal("scale expired historical runtime configuration")
	}
}

func TestSelectedReplicaCountAllowsExplicitAlreadySavedCount(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	materializeSelectedTestRelease(t, r, app)
	app.Annotations[appv1alpha1.AnnotationReleaseConfigScaled] = "3"
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	dep := materializeSelectedTestRelease(t, r, app)
	if *dep.Spec.Replicas != 3 {
		t.Fatalf("same-saved scale left historical count %d", *dep.Spec.Replicas)
	}
	if strings.Join(dep.Spec.Template.Spec.Containers[0].Command, " ") != "echo A" {
		t.Fatal("scale expired historical runtime configuration")
	}
}

func TestLegacySelectionDerivesTCPHealthAndKeepsUnknownReplicas(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	rec := &corev1.Secret{}
	if err := r.Get(context.Background(), client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.ReleaseRecordName(app.Name, 1)}, rec); err != nil {
		t.Fatal(err)
	}
	var template corev1.PodTemplateSpec
	if err := json.Unmarshal(rec.Data[appv1alpha1.ReleaseRecordPodTemplateKey], &template); err != nil {
		t.Fatal(err)
	}
	template.Spec.Containers[0].ReadinessProbe.HTTPGet = nil
	template.Spec.Containers[0].ReadinessProbe.TCPSocket = &corev1.TCPSocketAction{}
	raw, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	rec.Data[appv1alpha1.ReleaseRecordPodTemplateKey] = raw
	rec.Data[appv1alpha1.ReleaseRecordSpecKey] = []byte(`{"startCommand":"echo A"}`)
	if err := r.Update(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	dep := materializeSelectedTestRelease(t, r, app)
	if dep.Spec.Template.Spec.Containers[0].ReadinessProbe.TCPSocket == nil {
		t.Fatal("legacy TCP target inherited saved HTTP health path")
	}
	if *dep.Spec.Replicas != 3 {
		t.Fatal("legacy record invented unavailable target replicas")
	}
}

func TestCancelAfterRollbackKeepsTheSelectedServingTemplate(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	live := materializeSelectedTestRelease(t, r, app)
	app.Status.ActiveRevision = "rev-3"
	app.Status.Image = "image:A"
	app.Generation = 4
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "4"
	app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "4"
	app.Spec.StartCommand = "echo canceled"
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	app.Status.ActiveRevision = "rev-3"
	app.Status.ReleaseGeneration = 3
	app.Status.Image = "image:A"
	restore, err := r.servedPodTemplateForCancel(context.Background(), app)
	if err != nil || restore == nil {
		t.Fatalf("restore=%v err=%v", restore, err)
	}
	if err := r.applyServingDeployment(context.Background(), app, live, deploymentParams{image: "image:A", port: 3000, replicas: 2}, restore); err != nil {
		t.Fatal(err)
	}
	if strings.Join(live.Spec.Template.Spec.Containers[0].Command, " ") != "echo A" {
		t.Fatal("cancel applied saved configuration over the serving rollback")
	}
	if app.Spec.StartCommand != "echo canceled" {
		t.Fatal("cancel rewrote saved configuration")
	}
}

// imageUrl selects only an artifact: commands, health checks and configuration
// snapshots still come from saved settings, and the next release uses the saved
// image. This is also how a failed override stops affecting later config deploys.
func TestImageOverrideUsesSavedConfigurationAndExpiresOnNextRelease(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 3, Image: "image:C"}
	if err := r.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	dep := materializeSelectedTestRelease(t, r, app)
	c := dep.Spec.Template.Spec.Containers[0]
	if c.Image != "image:C" || strings.Join(c.Command, " ") != "echo B" || c.ReadinessProbe.HTTPGet.Path != "/B" {
		t.Fatalf("runtime = image %q, command %v, probe %q", c.Image, c.Command, c.ReadinessProbe.HTTPGet.Path)
	}
	if selectedTestValue(t, r, "api-env-r3") != "B" || selectedTestValue(t, r, "api-evg-shared-env-r3") != "group-B" {
		t.Fatal("image override selected historical environment")
	}
	if app.Spec.Image != "image:B" {
		t.Fatal("override changed saved image")
	}
	app.Generation = 4
	app.Status.ReleaseGeneration = 4
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "4"
	if image, ok := reusableArtifactImage(app, appReleaseDecision{}); !ok || image != "image:B" {
		t.Fatalf("next release image = %q, reusable %v; want saved image:B", image, ok)
	}
}

// TestASelectedConfigsAutoscalerWritesThePassApp (w5/110): a release running
// an earlier release's configuration plans its replicas from a copy of the
// App. The autoscaler wrote to that copy: the pass's own App kept its old
// resourceVersion, so its status write conflicted, and the Started record the
// autoscaler set went with the copy.
func TestASelectedConfigsAutoscalerWritesThePassApp(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	if err := r.ensureReleaseConfigSnapshot(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	app.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 1, MaxReplicas: 5, TargetCPUPercent: new(int32(80))}
	limit, _ := tierLimits(app.Spec.Tier)
	r.MetricsReader = func(context.Context, string, string) ([]PodUsage, error) {
		// Two pods at their CPU limit: 125% of the 80% target, so three replicas.
		return []PodUsage{{Pod: "a", CPUCores: limit}, {Pod: "b", CPUCores: limit}}, nil
	}

	plan, _, halted, err := r.planReplicas(context.Background(), app, releaseObservation{}, false)
	if err != nil || halted {
		t.Fatalf("planReplicas halted=%v err=%v", halted, err)
	}
	if plan.replicas != 3 {
		t.Fatalf("planned %d replicas, want 3 from the selected config's 2", plan.replicas)
	}
	if got := app.Status.Autoscaling; got == nil || got.State != appv1alpha1.AutoscalingTransitionStarted || got.FromReplicas != 2 || got.ToReplicas != 3 {
		t.Fatalf("the pass's App records autoscaling %+v, want Started 2→3", got)
	}
	if err := r.Status().Update(context.Background(), app); err != nil {
		t.Fatalf("the pass's status write: %v", err)
	}
}

// w4/m184: "saved changes aren't live — deploy to apply them" showed for the
// whole of the very deploy applying them. UndeployedChangesApplying says so
// only for a rolling release that runs the saved configuration.
// parkedBuildFailure records what a failed build leaves over a parked prior
// release: the service's sleeping or suspended Ready beside the build verdict
// of release generation gen (settlePriorRelease + fail).
func parkedBuildFailure(a *appv1alpha1.App, parkedReason, buildReason string, gen int64) {
	a.Status.Phase = appv1alpha1.PhaseHibernated
	a.Status.Conditions = append(a.Status.Conditions,
		metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: parkedReason, ObservedGeneration: 3,
			Message: "the latest build failed; the previously deployed release stays parked"},
		metav1.Condition{Type: appv1alpha1.ConditionBuild, Status: metav1.ConditionFalse, Reason: buildReason, ObservedGeneration: gen})
}

func TestInFlightReleaseCarriesSavedConfiguration(t *testing.T) {
	ctx := context.Background()
	check := func(t *testing.T, r *AppReconciler, app *appv1alpha1.App, want bool) {
		t.Helper()
		got, err := r.inFlightCarriesSavedConfiguration(ctx, app)
		if err != nil || got != want {
			t.Fatalf("carries saved = %v, %v; want %v", got, err, want)
		}
	}
	ordinary := func(t *testing.T) (*AppReconciler, *appv1alpha1.App) {
		r, app := selectedReleaseFixture(t)
		app.Spec.ReleaseConfig = nil // a standard deploy of the saved settings
		app.Status.Phase = appv1alpha1.PhaseDeploying
		return r, app
	}
	t.Run("a deploy not dispatched yet snapshots the saved values", func(t *testing.T) {
		r, app := ordinary(t)
		check(t, r, app, true)
	})
	// w4/529: the requested deploy carries the saved settings before the
	// operator dispatches it (still Running) and while it builds.
	for _, phase := range []appv1alpha1.AppPhase{appv1alpha1.PhaseRunning, appv1alpha1.PhasePending, appv1alpha1.PhaseBuilding} {
		t.Run("a requested deploy not yet rolling, "+string(phase), func(t *testing.T) {
			r, app := ordinary(t)
			app.Status.Phase = phase
			check(t, r, app, true)
		})
	}
	t.Run("a Save only stamps no release and carries nothing", func(t *testing.T) {
		r, app := ordinary(t)
		app.Status.Phase = appv1alpha1.PhaseRunning
		delete(app.Annotations, appv1alpha1.AnnotationReleaseGeneration)
		check(t, r, app, false)
	})
	t.Run("nothing is rolling once the release served", func(t *testing.T) {
		r, app := ordinary(t)
		app.Status.Phase = appv1alpha1.PhaseRunning
		app.Status.ActiveRevision = "rev-3"
		check(t, r, app, false)
	})
	t.Run("a failed release carries nothing", func(t *testing.T) {
		for name, fail := range map[string]func(*appv1alpha1.App){
			"failed phase": func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseFailed },
			"rollout verdict": func(a *appv1alpha1.App) {
				a.Status.Phase = appv1alpha1.PhaseRunning
				a.Status.Conditions = append(a.Status.Conditions, metav1.Condition{Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: "ProgressDeadlineExceeded", ObservedGeneration: 3})
			},
			"prior release serving": func(a *appv1alpha1.App) {
				a.Status.Phase = appv1alpha1.PhaseRunning
				a.Status.Conditions = append(a.Status.Conditions, metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: reasonPriorReleaseServing})
			},
			"pre-deploy": func(a *appv1alpha1.App) {
				a.Status.Phase = appv1alpha1.PhaseRunning
				a.Status.PreDeploy = &appv1alpha1.PreDeployStatus{Status: appv1alpha1.PreDeployFailed, Generation: 3}
			},
			// w21/041: over a parked release the failure is written as the
			// service's sleeping (or suspended) Ready, so only the build verdict
			// says the release failed.
			"build failed over a sleeping service": func(a *appv1alpha1.App) {
				parkedBuildFailure(a, reasonAutoHibernated, appv1alpha1.ReasonBuildFailedUserError, 3)
			},
			"build failed over a suspended service": func(a *appv1alpha1.App) {
				parkedBuildFailure(a, reasonSuspended, appv1alpha1.ReasonBuildFailed, 3)
			},
		} {
			r, app := ordinary(t)
			fail(app)
			if got, err := r.inFlightCarriesSavedConfiguration(ctx, app); err != nil || got {
				t.Errorf("%s: carries saved = %v, %v; want false", name, got, err)
			}
		}
	})
	// w21/041: the controls on a parked service. A requested deploy that is
	// still building, or that follows an earlier release's failed build, goes
	// live with the saved settings, so the notice must keep saying so.
	t.Run("a deploy building on a sleeping service still carries the saved settings", func(t *testing.T) {
		r, app := ordinary(t)
		app.Status.Phase = appv1alpha1.PhaseHibernated
		app.Status.Conditions = append(app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reasonAutoHibernated, ObservedGeneration: 3})
		check(t, r, app, true)
	})
	t.Run("an earlier release's failed build does not condemn the next deploy", func(t *testing.T) {
		r, app := ordinary(t)
		parkedBuildFailure(app, reasonAutoHibernated, appv1alpha1.ReasonBuildFailedUserError, 2)
		check(t, r, app, true)
	})
	t.Run("a canceled release carries nothing", func(t *testing.T) {
		r, app := ordinary(t)
		app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "3"
		check(t, r, app, false)
	})
	t.Run("a rollback or Restart selects a recorded configuration", func(t *testing.T) {
		r, app := selectedReleaseFixture(t)
		app.Status.Phase = appv1alpha1.PhaseDeploying
		check(t, r, app, false)
		app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 3, Image: "image:A", PreserveGroupValues: true}
		check(t, r, app, false)
	})
	t.Run("a dispatched release is compared with the saved configuration", func(t *testing.T) {
		r, app := ordinary(t)
		// Release 1's record holds configuration A, while B is saved.
		app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "1"
		app.Status.ReleaseGeneration, app.Status.ActiveRevision = 0, ""
		check(t, r, app, false)
		rec, err := r.readRuntimeConfigRecord(ctx, app, 1)
		if err != nil {
			t.Fatal(err)
		}
		app.Spec.Image, app.Spec.StartCommand = "image:A", rec.spec.StartCommand
		app.Spec.HealthCheckPath, app.Spec.Replicas, app.Spec.Env = *rec.spec.HealthCheckPath, *rec.spec.Replicas, rec.spec.Env
		setSelectedTestSecret(t, r, "api-env", "A")
		setSelectedTestSecret(t, r, "api-files", "file-A")
		setSelectedTestSecret(t, r, "evg-shared-env", "group-A")
		check(t, r, app, true)
	})
}

// w4/531: a release record keeps the tag its digest was resolved from, and a
// rollback or Restart that runs a selected release's digest inherits it, so a
// release serving the saved tag's digest is not an undeployed change.
func TestReleaseImageSourceFollowsTheSelectedRelease(t *testing.T) {
	ctx := context.Background()
	const digest = "registry.example/api@sha256:aaaa"
	r, app := selectedReleaseFixture(t)
	tmplFor := func(image string) corev1.PodTemplateSpec {
		return corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Image: image}}}}
	}

	// Release 1 resolved the saved tag to the digest.
	first := app.DeepCopy()
	first.Spec.ReleaseConfig = nil
	first.Status.ReleaseGeneration = 1
	first.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: digest, Source: "image:A"}
	if got := r.releaseImageSource(ctx, first, tmplFor(digest)); got != "image:A" {
		t.Fatalf("a tag-resolved release's source = %q, want image:A", got)
	}
	if err := r.recordReleasePodTemplate(ctx, first, tmplFor(digest)); err != nil {
		t.Fatal(err)
	}

	// Release 3 rolls back to it: its network records the digest as its own
	// source, and the record inherits release 1's tag.
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 3, SourceGeneration: 1, Image: digest}
	app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: digest, Source: digest}
	if got := r.releaseImageSource(ctx, app, tmplFor(digest)); got != "image:A" {
		t.Fatalf("a rollback's source = %q, want image:A", got)
	}
	// Another image than the selected release ran knows no source.
	if got := r.releaseImageSource(ctx, app, tmplFor("registry.example/api@sha256:bbbb")); got != "" {
		t.Fatalf("an unrelated digest's source = %q, want none", got)
	}
}

// A selection that stopped at its image read (a rollback to a tag the
// registry refused or is slow to answer, w4/m432) never materialized its own
// record. The held serving release renders without it, as a refused override
// does, instead of failing every pass with "read materialized release". Any
// other missing record still fails: a lost selection is never hidden.
func TestRuntimeRecordOfASelectionHaltedBeforeDispatch(t *testing.T) {
	ctx := context.Background()
	r, app := selectedReleaseFixture(t)
	if _, _, err := r.runtimeRecord(ctx, app); !apierrors.IsNotFound(err) {
		t.Fatalf("an unmaterialized selection in flight: err=%v, want not found", err)
	}
	app.Status.Phase = appv1alpha1.PhaseDeploying
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reasonImageRegistryUnavailable})
	rec, gen, err := r.runtimeRecord(ctx, app)
	if err != nil || rec != nil || gen != 0 {
		t.Fatalf("registry wait: rec=%v gen=%d err=%v, want the saved spec", rec, gen, err)
	}
}
