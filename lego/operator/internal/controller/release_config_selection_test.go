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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func selectedReleaseFixture(t *testing.T) (*AppReconciler, *appv1alpha1.App) {
	t.Helper()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: snapshotTestNS, UID: "api-uid", Generation: 3,
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "3"}},
		Spec: appv1alpha1.AppSpec{Image: "image:B", StartCommand: "echo B", HealthCheckPath: "/B", Replicas: 3, Tier: "standard", Port: 3000,
			Env: []appv1alpha1.EnvVar{{Name: "LITERAL", Value: "B"}}, EnvFromSecret: "api-env", EnvFromSecrets: []string{"evg-shared-env"}, FilesFromSecrets: []string{"api-files"},
			ReleaseConfig: &appv1alpha1.ReleaseConfigReference{Generation: 3, SourceGeneration: 1, Image: "image:A"}},
		Status: appv1alpha1.AppStatus{ReleaseGeneration: 3, ConfigSnapshotGeneration: 2, ActiveRevision: "rev-2", Image: "image:B"},
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
	return dep
}

func TestRollbackSelectionPreservesSavedConfiguration(t *testing.T) {
	r, app := selectedReleaseFixture(t)
	dep := materializeSelectedTestRelease(t, r, app)
	c := dep.Spec.Template.Spec.Containers[0]
	if c.Image != "image:A" || len(c.Command) != 3 || c.Command[2] != "echo A" {
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
	if again := materializeSelectedTestRelease(t, r, app); again.Spec.Template.Spec.Containers[0].Command[2] != "echo A" {
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
	if c.Image != "image:B" || c.Command[2] != "echo B" {
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
				if count != 1 {
					t.Errorf("%s fetched %d times in one pass, want one selected/saved comparison", name, count)
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
	if dep.Spec.Template.Spec.Containers[0].Command[2] != "echo A" {
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
	if dep.Spec.Template.Spec.Containers[0].Command[2] != "echo A" {
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
	if live.Spec.Template.Spec.Containers[0].Command[2] != "echo A" {
		t.Fatal("cancel applied saved configuration over the serving rollback")
	}
	if app.Spec.StartCommand != "echo canceled" {
		t.Fatal("cancel rewrote saved configuration")
	}
}
