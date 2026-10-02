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
	"reflect"
	"strconv"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A served release with an exact immutable file snapshot. A nil map means this
// release has never referenced the service's files Secret.
func savedConfigurationFixture(t *testing.T, serviceType string, files map[string][]byte) (*AppReconciler, *appv1alpha1.App) {
	t.Helper()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: snapshotTestNS, UID: "api-uid", Generation: 1,
			Labels:      map[string]string{labelWorkspace: snapshotTestNS},
			Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
		Spec: appv1alpha1.AppSpec{Type: serviceType, Image: "image:v1", Replicas: 1, Port: 3000, Schedule: "0 * * * *"},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, ReleaseGeneration: 1,
			ConfigSnapshotGeneration: 1, ActiveRevision: "rev-1", Image: "image:v1", ObservedGeneration: 1},
	}
	if files != nil {
		app.Spec.FilesFromSecrets = []string{"api-files"}
	}
	sch := deletionScheme(t)
	cl := fake.NewClientBuilder().WithScheme(sch).WithStatusSubresource(app).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: sch, Mode: ModeKubernetes}
	if files != nil {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "api-files", Namespace: app.Namespace}, Data: files}
		if err := cl.Create(t.Context(), s); err != nil {
			t.Fatal(err)
		}
		if err := r.copyConfigSecret(t.Context(), app, s.Name, 1); err != nil {
			t.Fatal(err)
		}
	}
	var tmpl corev1.PodTemplateSpec
	if serviceType == appv1alpha1.TypeCronJob {
		tmpl = r.cronPodSpec(app, app.Status.Image, 3000, map[string]string{labelApp: app.Name})
		cj := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.CronJobName(app.Name), Namespace: app.Namespace},
			Spec: batchv1.CronJobSpec{Schedule: app.Spec.Schedule, JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{Template: tmpl}}}}
		if err := controllerutil.SetControllerReference(app, cj, sch); err != nil {
			t.Fatal(err)
		}
		if err := cl.Create(t.Context(), cj); err != nil {
			t.Fatal(err)
		}
	} else {
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		applyDeploymentSpec(dep, app, deploymentParams{image: app.Status.Image, port: 3000, replicas: 1,
			worker: serviceType == appv1alpha1.TypeBackgroundWorker})
		tmpl = dep.Spec.Template
		if err := controllerutil.SetControllerReference(app, dep, sch); err != nil {
			t.Fatal(err)
		}
		if err := cl.Create(t.Context(), dep); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.recordReleasePodTemplate(t.Context(), app, tmpl); err != nil {
		t.Fatal(err)
	}
	return r, savedConfigurationApp(t, r, app)
}

func savedConfigurationApp(t *testing.T, r *AppReconciler, app *appv1alpha1.App) *appv1alpha1.App {
	t.Helper()
	current := &appv1alpha1.App{}
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), current); err != nil {
		t.Fatal(err)
	}
	return current
}

func notifySavedConfiguration(t *testing.T, r *AppReconciler, app *appv1alpha1.App, token string) *appv1alpha1.App {
	t.Helper()
	current := savedConfigurationApp(t, r, app)
	current.Annotations[appv1alpha1.AnnotationSavedConfigRevision] = token
	if err := r.Update(t.Context(), current); err != nil {
		t.Fatal(err)
	}
	return current
}

func reconcileSavedConfiguration(t *testing.T, r *AppReconciler, app *appv1alpha1.App, want bool) *appv1alpha1.App {
	t.Helper()
	if _, err := r.reconcileSavedConfigurationStatus(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(app)}); err != nil {
		t.Fatal(err)
	}
	current := savedConfigurationApp(t, r, app)
	if current.Status.UndeployedChanges != want {
		t.Fatalf("undeployedChanges=%v, want %v", current.Status.UndeployedChanges, want)
	}
	return current
}

func savedRuntimeTemplate(t *testing.T, r *AppReconciler, app *appv1alpha1.App) corev1.PodTemplateSpec {
	t.Helper()
	if app.Spec.Type == appv1alpha1.TypeCronJob {
		tmpl, err := r.cronJobTemplate(t.Context(), app)
		if err != nil {
			t.Fatal(err)
		}
		return tmpl
	}
	dep := &appsv1.Deployment{}
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), dep); err != nil {
		t.Fatal(err)
	}
	return dep.Spec.Template
}

func TestSavedConfigurationExistingFilesNoopRevertAndSecondSave(t *testing.T) {
	r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, map[string][]byte{"MESSAGE": []byte("v1")})
	beforeTemplate, beforeSpec := savedRuntimeTemplate(t, r, app), app.Spec.DeepCopy()
	for _, step := range []struct {
		value string
		want  bool
	}{{"v2", true}, {"v2", true}, {"v1", false}, {"v3", true}} {
		setSelectedTestSecret(t, r, "api-files", step.value)
		app = notifySavedConfiguration(t, r, app, "saved-"+step.value)
		app = reconcileSavedConfiguration(t, r, app, step.want)
		version := app.ResourceVersion
		app = reconcileSavedConfiguration(t, r, app, step.want)
		if app.ResourceVersion != version {
			t.Fatal("unchanged comparison issued another status write")
		}
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		if err := r.applyServingDeployment(t.Context(), app, dep, deploymentParams{image: app.Status.Image, port: 3000, replicas: 1}, nil); err != nil {
			t.Fatal(err)
		}
		if err := updateStatusIfChanged(t.Context(), r.Client, app); err != nil {
			t.Fatal(err)
		}
		app = savedConfigurationApp(t, r, app)
		if app.Status.UndeployedChanges != step.want {
			t.Fatal("ordinary runtime reconciliation erased the saved/runtime difference")
		}
		if !reflect.DeepEqual(beforeTemplate, savedRuntimeTemplate(t, r, app)) || !reflect.DeepEqual(*beforeSpec, app.Spec) ||
			app.Generation != 1 || app.Status.ReleaseGeneration != 1 || app.Status.ConfigSnapshotGeneration != 1 {
			t.Fatal("Save-only status reconciliation changed the serving template or release intent")
		}
		if got := selectedTestValue(t, r, "api-files-r1"); got != "v1" {
			t.Fatal("Save-only comparison rewrote the serving file snapshot")
		}
	}
}

func TestSavedConfigurationFirstReferencesRemainDeferredAcrossRuntimeTypes(t *testing.T) {
	for _, serviceType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob} {
		t.Run(serviceType, func(t *testing.T) {
			r, app := savedConfigurationFixture(t, serviceType, nil)
			before := savedRuntimeTemplate(t, r, app)
			for _, name := range []string{"api-env", "api-files"} {
				if err := r.Create(t.Context(), secret(name, "not-live")); err != nil {
					t.Fatal(err)
				}
			}
			app.Annotations[appv1alpha1.PendingEnvSecretAnnotation] = "api-env"
			app.Annotations[appv1alpha1.PendingFilesSecretAnnotation] = "api-files"
			app.Annotations[appv1alpha1.AnnotationSavedConfigRevision] = "saved-first"
			if err := r.Update(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			app = reconcileSavedConfiguration(t, r, app, true)
			for range 2 {
				if serviceType == appv1alpha1.TypeCronJob {
					runtime, err := r.selectedRuntimeApp(t.Context(), app)
					if err != nil {
						t.Fatal(err)
					}
					tmpl := r.cronPodSpec(runtime, app.Status.Image, 3000, before.Labels)
					if _, err := r.applyServingCronJob(t.Context(), app, tmpl, false); err != nil {
						t.Fatal(err)
					}
				} else {
					dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
					if err := r.applyServingDeployment(t.Context(), app, dep, deploymentParams{image: app.Status.Image,
						port: 3000, replicas: 1, worker: serviceType == appv1alpha1.TypeBackgroundWorker}, nil); err != nil {
						t.Fatal(err)
					}
				}
				if !reflect.DeepEqual(before, savedRuntimeTemplate(t, r, app)) {
					t.Fatal("a routine runtime reconcile applied pending first references")
				}
				app = reconcileSavedConfiguration(t, r, app, true)
			}
			// Reverting a first save removes its pending references. The leftover
			// unreferenced Secret is not a runtime difference.
			delete(app.Annotations, appv1alpha1.PendingEnvSecretAnnotation)
			delete(app.Annotations, appv1alpha1.PendingFilesSecretAnnotation)
			app.Annotations[appv1alpha1.AnnotationSavedConfigRevision] = "reverted-first"
			if err := r.Update(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			reconcileSavedConfiguration(t, r, app, false)
		})
	}
}

func TestSavedConfigurationClearsOnlyAfterCandidateServes(t *testing.T) {
	r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, map[string][]byte{"MESSAGE": []byte("v1")})
	setSelectedTestSecret(t, r, "api-files", "v2")
	app = notifySavedConfiguration(t, r, app, "saved-v2")
	app = reconcileSavedConfiguration(t, r, app, true)
	app.Generation = 2
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	app.Spec.RestartedAt = "deploy-v2"
	if err := r.Update(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	app.Status.ReleaseGeneration, app.Status.ConfigSnapshotGeneration = 2, 2
	app.Status.Phase = appv1alpha1.PhaseDeploying
	if err := r.Status().Update(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	if err := r.copyConfigSecret(t.Context(), app, "api-files", 2); err != nil {
		t.Fatal(err)
	}
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if err := r.applyServingDeployment(t.Context(), app, dep, deploymentParams{image: app.Status.Image, port: 3000, replicas: 1}, nil); err != nil {
		t.Fatal(err)
	}
	app = reconcileSavedConfiguration(t, r, app, true)
	if app.Status.ActiveRevision != "rev-1" || selectedTestValue(t, r, "api-files-r2") != "v2" {
		t.Fatal("fixture must contain a snapshotted candidate over the older serving release")
	}
	app.Status.ActiveRevision, app.Status.Phase = "rev-2", appv1alpha1.PhaseRunning
	if err := r.Status().Update(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	app = reconcileSavedConfiguration(t, r, app, false)
	setSelectedTestSecret(t, r, "api-files", "v3")
	app = notifySavedConfiguration(t, r, app, "saved-v3")
	reconcileSavedConfiguration(t, r, app, true)
}

func TestSavedConfigurationMissingRecordUsesAlreadyAppliedMembership(t *testing.T) {
	for _, serviceType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeCronJob} {
		t.Run(serviceType, func(t *testing.T) {
			r, app := savedConfigurationFixture(t, serviceType, map[string][]byte{"MESSAGE": []byte("v1")})
			before := savedRuntimeTemplate(t, r, app)
			if err := r.Delete(t.Context(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ReleaseRecordName(app.Name, 1), Namespace: app.Namespace}}); err != nil {
				t.Fatal(err)
			}
			// The live file was originally deployed from a pending annotation.
			// Losing its record must not remove that already-consumed reference;
			// adding an unrelated first env reference must not activate the env.
			app.Spec.FilesFromSecrets = nil
			app.Annotations[appv1alpha1.PendingFilesSecretAnnotation] = "api-files"
			app.Annotations[appv1alpha1.PendingEnvSecretAnnotation] = "api-env"
			if err := r.Create(t.Context(), secret("api-env", "not-live")); err != nil {
				t.Fatal(err)
			}
			runtime, err := r.selectedRuntimeApp(t.Context(), app)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.Spec.EnvFromSecret != "" || !reflect.DeepEqual(runtime.Spec.FilesFromSecrets, []string{"api-files"}) {
				t.Fatal("missing-record fallback changed already-applied source membership")
			}
			var projected corev1.PodTemplateSpec
			if serviceType == appv1alpha1.TypeCronJob {
				projected = r.cronPodSpec(runtime, app.Status.Image, 3000, before.Labels)
			} else {
				dep := &appsv1.Deployment{}
				applyDeploymentSpec(dep, runtime, deploymentParams{image: app.Status.Image, port: 3000, replicas: 1})
				projected = dep.Spec.Template
			}
			if !reflect.DeepEqual(before, projected) {
				t.Fatal("fallback would roll a served release after losing its configuration record")
			}
		})
	}
}

func TestSavedConfigurationMissingCandidateRecordDoesNotApplyLaterSave(t *testing.T) {
	r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, nil)
	app.Generation = 2
	app.Status.ReleaseGeneration, app.Status.ConfigSnapshotGeneration = 2, 2
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	dep := &appsv1.Deployment{}
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), dep); err != nil {
		t.Fatal(err)
	}
	applyDeploymentSpec(dep, app, deploymentParams{image: app.Status.Image, port: 3000, replicas: 1})
	if err := r.Update(t.Context(), dep); err != nil {
		t.Fatal(err)
	}
	before := dep.Spec.Template.DeepCopy()
	// The candidate template has been written, but its release-record write
	// was lost and Ready has not advanced ActiveRevision from rev-1.
	app.Annotations[appv1alpha1.PendingFilesSecretAnnotation] = "api-files"
	if err := r.Create(t.Context(), secret("api-files", "saved-after-dispatch")); err != nil {
		t.Fatal(err)
	}
	runtime, err := r.selectedRuntimeApp(t.Context(), app)
	if err != nil {
		t.Fatal(err)
	}
	applyDeploymentSpec(dep, runtime, deploymentParams{image: app.Status.Image, port: 3000, replicas: 1})
	if !reflect.DeepEqual(*before, dep.Spec.Template) {
		t.Fatal("missing in-flight record treated a retry as first dispatch and activated a later Save-only reference")
	}
}

func TestSavedConfigurationDeletedSourcesClearAfterStandardDeploy(t *testing.T) {
	for _, change := range []string{"file deletion", "last env key deletion", "file deletion and env update"} {
		t.Run(change, func(t *testing.T) {
			r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, map[string][]byte{"MESSAGE": []byte("v1")})
			app.Spec.EnvFromSecret = "api-env"
			if err := r.Update(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			if err := r.Create(t.Context(), secret("api-env", "env-v1")); err != nil {
				t.Fatal(err)
			}
			if err := r.copyConfigSecret(t.Context(), app, "api-env", 1); err != nil {
				t.Fatal(err)
			}
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
			if err := r.applyServingDeployment(t.Context(), app, dep, deploymentParams{image: app.Status.Image, port: 3000, replicas: 1}, nil); err != nil {
				t.Fatal(err)
			}
			before := dep.Spec.Template.DeepCopy()
			if change != "last env key deletion" {
				// Secret-file deletion removes the mutable Secret while retaining
				// the active optional source reference on the App.
				if err := r.Delete(t.Context(), secret("api-files", "")); err != nil {
					t.Fatal(err)
				}
			}
			switch change {
			case "file deletion and env update":
				setSelectedTestSecret(t, r, "api-env", "env-v2")
			case "last env key deletion":
				env := &corev1.Secret{}
				if err := r.Get(t.Context(), client.ObjectKey{Name: "api-env", Namespace: app.Namespace}, env); err != nil {
					t.Fatal(err)
				}
				env.Data = map[string][]byte{}
				if err := r.Update(t.Context(), env); err != nil {
					t.Fatal(err)
				}
			}
			app = notifySavedConfiguration(t, r, app, "saved-deletion")
			app = reconcileSavedConfiguration(t, r, app, true)
			if !reflect.DeepEqual(*before, savedRuntimeTemplate(t, r, app)) {
				t.Fatal("saving a deletion changed the serving template")
			}
			app = deploySavedConfiguration(t, r, app)
			reconcileSavedConfiguration(t, r, app, false)
			if change != "last env key deletion" {
				absent := &corev1.Secret{}
				if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: "api-files-r2"}, absent); err != nil || len(absent.Data) != 0 {
					t.Fatalf("deleted optional source needs an empty release snapshot, err=%v", err)
				}
			}
		})
	}
}

// Run the standard snapshot/template path, then report the simulated ready
// deployment through the same success transition that publishes ActiveRevision.
func deploySavedConfiguration(t *testing.T, r *AppReconciler, app *appv1alpha1.App) *appv1alpha1.App {
	t.Helper()
	app = savedConfigurationApp(t, r, app)
	app.Generation++
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(app.Generation, 10)
	app.Spec.RestartedAt = "deploy-" + strconv.FormatInt(app.Generation, 10)
	if err := r.Update(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	app.Status.ReleaseGeneration = app.Generation
	if err := r.ensureSavedReleaseConfigSnapshot(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	if err := r.applyServingDeployment(t.Context(), app, dep, deploymentParams{image: app.Status.Image, port: 3000, replicas: 1}, nil); err != nil {
		t.Fatal(err)
	}
	dep.Status.ReadyReplicas = 1
	if err := r.markRunning(t.Context(), app, dep, 1, "test release ready"); err != nil {
		t.Fatal(err)
	}
	return savedConfigurationApp(t, r, app)
}

func TestSavedConfigurationEmptyAndUnreferencedSourcesAreNotDifferences(t *testing.T) {
	r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, map[string][]byte{})
	if err := r.Delete(t.Context(), secret("api-files", "")); err != nil {
		t.Fatal(err)
	}
	if err := r.Create(t.Context(), secret("api-unreferenced", "unused")); err != nil {
		t.Fatal(err)
	}
	app = notifySavedConfiguration(t, r, app, "saved-empty")
	reconcileSavedConfiguration(t, r, app, false)
}

func TestSavedConfigurationSourceMembershipAndRevert(t *testing.T) {
	r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, nil)
	if err := r.Create(t.Context(), secret("evg-shared-env", "group-value")); err != nil {
		t.Fatal(err)
	}
	before := savedRuntimeTemplate(t, r, app)
	app.Spec.EnvFromSecrets = []string{"evg-shared-env"}
	if err := r.Update(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	app = reconcileSavedConfiguration(t, r, app, true)
	app.Spec.EnvFromSecrets = nil
	if err := r.Update(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	app = reconcileSavedConfiguration(t, r, app, false)
	if !reflect.DeepEqual(before, savedRuntimeTemplate(t, r, app)) {
		t.Fatal("membership comparison mutated the serving template")
	}
}

func TestSavedConfigurationMissingEvidenceNeverClearsPending(t *testing.T) {
	for _, missing := range []string{"record", "snapshot", "legacy mutable record", "source read unavailable", "record malformed"} {
		t.Run(missing, func(t *testing.T) {
			r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, map[string][]byte{"MESSAGE": []byte("v1")})
			setSelectedTestSecret(t, r, "api-files", "v2")
			app = notifySavedConfiguration(t, r, app, "saved-v2")
			wantError := false
			switch missing {
			case "record":
				if err := r.Delete(t.Context(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ReleaseRecordName(app.Name, 1), Namespace: app.Namespace}}); err != nil {
					t.Fatal(err)
				}
			case "snapshot":
				if err := r.Delete(t.Context(), secret("api-files-r1", "")); err != nil {
					t.Fatal(err)
				}
				wantError = true
			case "legacy mutable record":
				app.Status.ConfigSnapshotGeneration = 0
				if err := r.Status().Update(t.Context(), app); err != nil {
					t.Fatal(err)
				}
				dep := &appsv1.Deployment{}
				applyDeploymentSpec(dep, app, deploymentParams{image: app.Status.Image, port: 3000, replicas: 1})
				if err := r.recordReleasePodTemplate(t.Context(), app, dep.Spec.Template); err != nil {
					t.Fatal(err)
				}
				if err := r.Delete(t.Context(), secret("api-files-r1", "")); err != nil {
					t.Fatal(err)
				}
			case "source read unavailable":
				base := r.Client.(client.WithWatch)
				r.BuildClient = interceptor.NewClient(base, interceptor.Funcs{Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*corev1.Secret); ok && key.Name == "api-files" {
						return errors.New("source temporarily unavailable")
					}
					return cl.Get(ctx, key, obj, opts...)
				}})
				wantError = true
			case "record malformed":
				rec := &corev1.Secret{}
				if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.ReleaseRecordName(app.Name, 1)}, rec); err != nil {
					t.Fatal(err)
				}
				rec.Data[appv1alpha1.ReleaseRecordSpecKey] = []byte("invalid")
				if err := r.Update(t.Context(), rec); err != nil {
					t.Fatal(err)
				}
				wantError = true
			}
			_, err := r.reconcileSavedConfigurationStatus(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(app)})
			if (err != nil) != wantError {
				t.Fatalf("comparison err=%v, wantError=%v", err, wantError)
			}
			if current := savedConfigurationApp(t, r, app); !current.Status.UndeployedChanges || current.Status.Phase != appv1alpha1.PhaseRunning {
				t.Fatal("unavailable historical evidence cleared pending state or changed runtime health")
			}
		})
	}
}

func TestSavedConfigurationConcurrentSaveInvalidatesStatusPatch(t *testing.T) {
	r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, map[string][]byte{"MESSAGE": []byte("v1")})
	setSelectedTestSecret(t, r, "api-files", "v2")
	app = notifySavedConfiguration(t, r, app, "saved-v2")
	base := r.Client.(client.WithWatch)
	raced := false
	r.Client = interceptor.NewClient(base, interceptor.Funcs{SubResourcePatch: func(ctx context.Context, cl client.Client, subresource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
		if subresource == "status" && !raced {
			raced = true
			// A newer save reverts to the running value between comparison and
			// publication. Its metadata resourceVersion invalidates the old result.
			setSelectedTestSecret(t, r, "api-files", "v1")
			notifySavedConfiguration(t, r, app, "reverted-v1")
		}
		return cl.SubResource(subresource).Patch(ctx, obj, patch, opts...)
	}})
	_, err := r.reconcileSavedConfigurationStatus(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(app)})
	if !raced || !apierrors.IsConflict(err) {
		t.Fatalf("stale comparison err=%v, raced=%v; want conflict", err, raced)
	}
	if savedConfigurationApp(t, r, app).Status.UndeployedChanges {
		t.Fatal("stale difference was published after the newer revert")
	}
	reconcileSavedConfiguration(t, r, app, false)
}

func TestSavedConfigurationMissingServedRecordRejectsAnotherRevision(t *testing.T) {
	r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, nil)
	if err := r.Delete(t.Context(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ReleaseRecordName(app.Name, 1), Namespace: app.Namespace}}); err != nil {
		t.Fatal(err)
	}
	dep := &appsv1.Deployment{}
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), dep); err != nil {
		t.Fatal(err)
	}
	dep.Spec.Template.Labels[labelRevision] = "rev-2"
	if err := r.Update(t.Context(), dep); err != nil {
		t.Fatal(err)
	}
	app.Annotations[appv1alpha1.PendingEnvSecretAnnotation] = "api-env"
	if _, err := r.selectedRuntimeApp(t.Context(), app); err == nil {
		t.Fatal("a different candidate revision was adopted as the missing served record")
	}
}

func TestSavedConfigurationEventsDoNotDispatchRuntime(t *testing.T) {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Generation: 4, Annotations: map[string]string{}},
		Status: appv1alpha1.AppStatus{ActiveRevision: "rev-4", ConfigSnapshotGeneration: 4}}
	for _, annotation := range []string{appv1alpha1.AnnotationSavedConfigRevision, appv1alpha1.PendingEnvSecretAnnotation, appv1alpha1.PendingFilesSecretAnnotation} {
		next := app.DeepCopy()
		next.Annotations[annotation] = "new-notification"
		e := event.UpdateEvent{ObjectOld: app, ObjectNew: next}
		if !(savedConfigurationPredicate{}).Update(e) || (generationOrDeletionPredicate{}).Update(e) {
			t.Fatalf("annotation %s must queue only configuration status", annotation)
		}
	}
	next := app.DeepCopy()
	next.Status.ActiveRevision = "rev-5"
	if !(savedConfigurationPredicate{}).Update(event.UpdateEvent{ObjectOld: app, ObjectNew: next}) {
		t.Fatal("a newly serving release did not schedule its difference comparison")
	}
}

func TestSavedConfigurationScopeGuards(t *testing.T) {
	for _, boundary := range []string{"static", "opensandbox", "foreign namespace"} {
		t.Run(boundary, func(t *testing.T) {
			r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, nil)
			app = notifySavedConfiguration(t, r, app, "saved-but-unsupported")
			switch boundary {
			case "static":
				app.Spec.Type = appv1alpha1.TypeStaticSite
			case "opensandbox":
				r.Mode = ModeOpenSandbox
			case "foreign namespace":
				app.Labels[labelWorkspace] = "tea-other"
			}
			if err := r.Update(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			before := savedConfigurationApp(t, r, app)
			after := reconcileSavedConfiguration(t, r, app, false)
			if after.ResourceVersion != before.ResourceVersion {
				t.Fatal("out-of-scope status controller mutated the App")
			}
		})
	}
}

func TestRuntimeStatusRetriesPreserveSavedConfigurationStatus(t *testing.T) {
	for _, path := range []string{"phase", "failure"} {
		t.Run(path, func(t *testing.T) {
			r, app := savedConfigurationFixture(t, appv1alpha1.TypeWebService, nil)
			stale := app.DeepCopy()
			app.Status.UndeployedChanges = true
			if err := r.Status().Update(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			// The fake does not consistently enforce stale status Updates across
			// Kubernetes versions. Model the API server's resourceVersion check.
			base := r.Client.(client.WithWatch)
			r.Client = interceptor.NewClient(base, interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, cl client.Client, subresource string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				live := &appv1alpha1.App{}
				if err := cl.Get(ctx, client.ObjectKeyFromObject(obj), live); err != nil {
					return err
				}
				if live.ResourceVersion != obj.GetResourceVersion() {
					return apierrors.NewConflict(schema.GroupResource{Group: appv1alpha1.SchemeGroupVersion.Group, Resource: "apps"}, obj.GetName(), errors.New("stale status"))
				}
				return cl.SubResource(subresource).Update(ctx, obj, opts...)
			}})
			if path == "phase" {
				r.setPhase(t.Context(), stale, appv1alpha1.PhaseDeploying, "Deploying", "applying a deployment")
			} else {
				_, _ = r.fail(t.Context(), stale, "DeployFailed", errors.New("deployment unavailable"))
			}
			if current := savedConfigurationApp(t, r, app); !current.Status.UndeployedChanges {
				t.Fatal("runtime retry overwrote the concurrently published configuration difference")
			}
		})
	}
}
