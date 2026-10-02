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
	"fmt"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// runtimeConfigRecord keeps the selected input description beside the template
// that consumed it. The record for the NEW release owns its own copied Secrets,
// so retaining a rollback never depends on the target surviving later GC.
type runtimeConfigRecord struct {
	spec     appv1alpha1.ReleaseRecordSpec
	template corev1.PodTemplateSpec
}

func (r *AppReconciler) readRuntimeConfigRecord(ctx context.Context, app *appv1alpha1.App, generation int64) (*runtimeConfigRecord, error) {
	rec := &corev1.Secret{}
	if err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.ReleaseRecordName(app.Name, generation)}, rec); err != nil {
		return nil, err
	}
	result := &runtimeConfigRecord{}
	if err := json.Unmarshal(rec.Data[appv1alpha1.ReleaseRecordSpecKey], &result.spec); err != nil {
		return nil, fmt.Errorf("decode release %d configuration: %w", generation, err)
	}
	if err := json.Unmarshal(rec.Data[appv1alpha1.ReleaseRecordPodTemplateKey], &result.template); err != nil {
		return nil, fmt.Errorf("decode release %d template: %w", generation, err)
	}
	if result.spec.Version == 0 {
		result.adoptLegacySpec(app, generation)
	}
	return result, nil
}

func (rec *runtimeConfigRecord) adoptLegacySpec(app *appv1alpha1.App, generation int64) {
	if app.Spec.Type == appv1alpha1.TypeCronJob {
		rec.spec.Command = new("")
	}
	for _, c := range rec.template.Spec.Containers {
		if c.Name != "app" {
			continue
		}
		for _, env := range c.Env {
			if env.Name == portEnvName {
				continue
			}
			v := appv1alpha1.EnvVar{Name: env.Name, Value: env.Value}
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				ref := env.ValueFrom.SecretKeyRef
				v.ValueFrom = &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: ref.Name, Key: ref.Key}}
			}
			rec.spec.Env = append(rec.spec.Env, v)
		}
		for _, from := range c.EnvFrom {
			if from.SecretRef == nil {
				continue
			}
			source := historicalSourceName(app, from.SecretRef.Name, generation)
			if strings.HasPrefix(source, app.Name+"-") {
				rec.spec.EnvFromSecret = source
			} else {
				rec.spec.EnvFromSecrets = append(rec.spec.EnvFromSecrets, source)
			}
		}
		if c.ReadinessProbe != nil {
			if c.ReadinessProbe.HTTPGet != nil {
				rec.spec.HealthCheckPath = new(c.ReadinessProbe.HTTPGet.Path)
			} else if c.ReadinessProbe.TCPSocket != nil {
				rec.spec.HealthCheckPath = new("")
			}
		}
		if app.Spec.Type == appv1alpha1.TypeCronJob && len(c.Command) == 3 && c.Command[1] == "-c" {
			rec.spec.Command = new(c.Command[2])
		}
	}
	for _, volume := range rec.template.Spec.Volumes {
		if volume.Name != secretFilesVolumeName || volume.Projected == nil {
			continue
		}
		for _, from := range volume.Projected.Sources {
			if from.Secret != nil {
				rec.spec.FilesFromSecrets = append(rec.spec.FilesFromSecrets, historicalSourceName(app, from.Secret.Name, generation))
			}
		}
	}
	rec.spec.Version = 1
}

// historicalSourceName also handles the pre-scoping group snapshots retained by
// legacy Apps. New records store the exact source membership directly.
func historicalSourceName(app *appv1alpha1.App, name string, generation int64) string {
	source := strings.TrimSuffix(name, appv1alpha1.ReleaseSnapshotSuffix+strconv.FormatInt(generation, 10))
	if source != app.Name+"-env" && source != app.Name+"-files" {
		source = strings.TrimPrefix(source, app.Name+"-")
	}
	return source
}

func (r *AppReconciler) ensureSelectedReleaseConfig(ctx context.Context, app *appv1alpha1.App, ref *appv1alpha1.ReleaseConfigReference) error {
	rec, err := r.readRuntimeConfigRecord(ctx, app, ref.Generation)
	if apierrors.IsNotFound(err) {
		rec, err = r.readRuntimeConfigRecord(ctx, app, ref.SourceGeneration)
		if err != nil {
			return fmt.Errorf("read selected release %d: %w", ref.SourceGeneration, err)
		}
		if err := r.copySelectedSources(ctx, app, ref, rec); err != nil {
			return err
		}
		rec.spec.SavedReplicas = new(app.Spec.Replicas)
		if err := r.writeRuntimeConfigRecord(ctx, app, ref.Generation, rec); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// The persisted record is the commit point. On retry its copies are reused;
	// changed saved/group values cannot rewrite an already selected release.
	app.Status.ConfigSnapshotGeneration = ref.Generation
	effectiveApp := rec.projectRuntimeApp(app, ref)
	app.Status.UndeployedChanges, err = r.selectedConfigurationDiffers(ctx, app, effectiveApp)
	if err != nil {
		return err
	}
	return updateStatusIfChanged(ctx, r.Client, app)
}

// Compare values as well as source names: saved Secret contents can change
// without changing any App spec field. Every selected copy must still exist;
// an optional envFrom must not turn a missing snapshot into a silent empty env.
func (r *AppReconciler) selectedConfigurationDiffers(ctx context.Context, saved, runtime *appv1alpha1.App) (bool, error) {
	different := !equality.Semantic.DeepEqual(releaseRecordSpec(saved), releaseRecordSpec(runtime))
	if ref := saved.ActiveReleaseConfig(); saved.Spec.Repo == "" && ref != nil && saved.Spec.Image != ref.Image {
		different = true
	}
	for _, source := range releaseConfigSources(runtime) {
		snapshot := &corev1.Secret{}
		if err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: saved.Namespace, Name: snapshotOrSource(runtime, source)}, snapshot); err != nil {
			return false, fmt.Errorf("read materialized configuration %s: %w", source, err)
		}
		current := &corev1.Secret{}
		err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: saved.Namespace, Name: source}, current)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		if !equality.Semantic.DeepEqual(snapshot.Data, current.Data) {
			different = true
		}
	}
	return different, nil
}

func (r *AppReconciler) copySelectedSources(ctx context.Context, app *appv1alpha1.App, ref *appv1alpha1.ReleaseConfigReference, rec *runtimeConfigRecord) error {
	var err error
	rec.spec.EnvFromSecrets, err = r.copySelectedSourceList(ctx, app, ref, rec, rec.spec.EnvFromSecrets)
	if err != nil {
		return err
	}
	if rec.spec.EnvFromSecret != "" {
		if _, err := r.copySelectedSource(ctx, app, ref, rec, rec.spec.EnvFromSecret); err != nil {
			return err
		}
	}
	rec.spec.FilesFromSecrets, err = r.copySelectedSourceList(ctx, app, ref, rec, rec.spec.FilesFromSecrets)
	return err
}

func (r *AppReconciler) copySelectedSourceList(ctx context.Context, app *appv1alpha1.App, ref *appv1alpha1.ReleaseConfigReference, rec *runtimeConfigRecord, sources []string) ([]string, error) {
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		exists, err := r.copySelectedSource(ctx, app, ref, rec, source)
		if err != nil {
			return nil, err
		}
		if exists {
			out = append(out, source)
		}
	}
	return out, nil
}

func (r *AppReconciler) copySelectedSource(ctx context.Context, app *appv1alpha1.App, ref *appv1alpha1.ReleaseConfigReference, rec *runtimeConfigRecord, source string) (bool, error) {
	destination := appv1alpha1.AppReleaseSnapshotName(app.Name, source, ref.Generation)
	existing := &corev1.Secret{}
	err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: destination}, existing)
	if err == nil {
		return true, nil
	}
	if !apierrors.IsNotFound(err) {
		return false, err
	}
	name := selectedSourceName(app, ref, rec, source)
	selected := &corev1.Secret{}
	if err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, selected); err != nil {
		// Rollback uses current values from the target's surviving associations.
		// A restart uses its exact snapshots even if the shared source was deleted.
		if apierrors.IsNotFound(err) && !ref.PreserveGroupValues && !strings.HasPrefix(source, app.Name+"-") {
			return false, nil
		}
		return false, fmt.Errorf("read selected configuration %s: %w", name, err)
	}
	snapshot := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: destination, Namespace: app.Namespace,
		Labels: map[string]string{snapshotOfLabel: source, snapshotGenerationLabel: strconv.FormatInt(ref.Generation, 10)}}, Type: selected.Type, Data: selected.DeepCopy().Data}
	if err := controllerutil.SetControllerReference(app, snapshot, r.Scheme); err != nil {
		return false, err
	}
	if err := r.uncachedSecretClient().Create(ctx, snapshot); err != nil && !apierrors.IsAlreadyExists(err) {
		return false, err
	}
	return true, nil
}

func selectedSourceName(app *appv1alpha1.App, ref *appv1alpha1.ReleaseConfigReference, rec *runtimeConfigRecord, source string) string {
	if !ref.PreserveGroupValues && !strings.HasPrefix(source, app.Name+"-") {
		return source
	}
	// The recorded references are authoritative for legacy unscoped copies.
	for _, c := range rec.template.Spec.Containers {
		for _, from := range c.EnvFrom {
			if from.SecretRef != nil && historicalSourceName(app, from.SecretRef.Name, ref.SourceGeneration) == source {
				return from.SecretRef.Name
			}
		}
	}
	for _, volume := range rec.template.Spec.Volumes {
		if volume.Name != secretFilesVolumeName || volume.Projected == nil {
			continue
		}
		for _, from := range volume.Projected.Sources {
			if from.Secret != nil && historicalSourceName(app, from.Secret.Name, ref.SourceGeneration) == source {
				return from.Secret.Name
			}
		}
	}
	return appv1alpha1.AppReleaseSnapshotName(app.Name, source, ref.SourceGeneration)
}

func (r *AppReconciler) selectedRuntimeApp(ctx context.Context, app *appv1alpha1.App) (*appv1alpha1.App, error) {
	ref := app.ActiveReleaseConfig()
	if ref == nil || ref.SourceGeneration == 0 || canceledOverServed(app) {
		return app, nil
	}
	rec, err := r.readRuntimeConfigRecord(ctx, app, ref.Generation)
	if err != nil {
		return nil, fmt.Errorf("read materialized release %d: %w", ref.Generation, err)
	}
	return rec.projectRuntimeApp(app, ref), nil
}

func (rec *runtimeConfigRecord) projectRuntimeApp(app *appv1alpha1.App, ref *appv1alpha1.ReleaseConfigReference) *appv1alpha1.App {
	effectiveApp := app.DeepCopy()
	effectiveApp.Spec.StartCommand = rec.spec.StartCommand
	if rec.spec.Command != nil {
		effectiveApp.Spec.Command = *rec.spec.Command
	}
	if rec.spec.HealthCheckPath != nil {
		effectiveApp.Spec.HealthCheckPath = *rec.spec.HealthCheckPath
	}
	explicitScale := app.Annotations[appv1alpha1.AnnotationReleaseConfigScaled] == strconv.FormatInt(ref.Generation, 10)
	if rec.spec.Replicas != nil && !explicitScale && (rec.spec.SavedReplicas == nil || *rec.spec.SavedReplicas == app.Spec.Replicas) {
		effectiveApp.Spec.Replicas = *rec.spec.Replicas
	}
	effectiveApp.Spec.Env = rec.spec.Env
	effectiveApp.Spec.EnvFromSecret = rec.spec.EnvFromSecret
	effectiveApp.Spec.EnvFromSecrets = rec.spec.EnvFromSecrets
	effectiveApp.Spec.FilesFromSecrets = rec.spec.FilesFromSecrets
	delete(effectiveApp.Annotations, appv1alpha1.PendingEnvSecretAnnotation)
	delete(effectiveApp.Annotations, appv1alpha1.PendingFilesSecretAnnotation)
	return effectiveApp
}

func releaseRecordSpec(app *appv1alpha1.App) appv1alpha1.ReleaseRecordSpec {
	files := slices.Clone(app.Spec.FilesFromSecrets)
	if name := app.Annotations[appv1alpha1.PendingFilesSecretAnnotation]; name != "" && !slices.Contains(files, name) {
		files = append(files, name)
	}
	return appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: app.Spec.StartCommand, Command: new(app.Spec.Command),
		HealthCheckPath: new(app.Spec.HealthCheckPath), Replicas: new(app.Spec.Replicas), Env: app.Spec.Env,
		EnvFromSecret: runtimeEnvSecret(app), EnvFromSecrets: app.Spec.EnvFromSecrets, FilesFromSecrets: files}
}

func (r *AppReconciler) writeRuntimeConfigRecord(ctx context.Context, app *appv1alpha1.App, generation int64, record *runtimeConfigRecord) error {
	raw, err := json.Marshal(record.template)
	if err != nil {
		return err
	}
	spec, err := json.Marshal(record.spec)
	if err != nil {
		return err
	}
	rec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ReleaseRecordName(app.Name, generation), Namespace: app.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.uncachedSecretClient(), rec, func() error {
		if rec.Labels == nil {
			rec.Labels = map[string]string{}
		}
		rec.Labels[snapshotOfLabel] = app.Name + "-podtemplate"
		rec.Labels[snapshotGenerationLabel] = strconv.FormatInt(generation, 10)
		rec.Data = map[string][]byte{appv1alpha1.ReleaseRecordPodTemplateKey: raw, appv1alpha1.ReleaseRecordSpecKey: spec}
		return controllerutil.SetControllerReference(app, rec, r.Scheme)
	})
	return err
}
