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

package secrets

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// SaveMode controls whether a coherent environment patch only updates stored
// and projected configuration or also rolls the service once. Rebuilding source
// is intentionally composed by the deploys feature after a successful save.
type SaveMode string

const (
	SaveModeOnly                    SaveMode = "save_only"
	SaveModeDeploy                  SaveMode = "deploy"
	envProjectionRevisionAnnotation          = "app.bex.co/env-source-revision"
)

// EnvVarPatch is one explicit mutation in a service environment draft. Omitted
// keys are preserved without being read by or returned to the caller.
type EnvVarPatch = core.EnvVarPatch

// SecretFilePatch is one explicit secret-file mutation. Omitted files are
// preserved, and contents never appear in PatchEnvironment's result.
type SecretFilePatch = core.SecretFilePatch

// EnvironmentPatch applies env-var and secret-file changes as one logical save.
type EnvironmentPatch struct {
	EnvVars             []EnvVarPatch     `json:"envVars,omitempty"`
	SecretFiles         []SecretFilePatch `json:"secretFiles,omitempty"`
	SaveMode            SaveMode          `json:"saveMode"`
	ExpectedEnvRevision *string           `json:"expectedEnvRevision,omitempty"`
}

// EnvironmentPatchResult deliberately contains names only. Generated or
// supplied secret material remains write-only on this batch surface.
type EnvironmentPatchResult struct {
	EnvVarKeys      []string `json:"envVarKeys"`
	SecretFileNames []string `json:"secretFileNames"`
	RolledOut       bool     `json:"rolledOut"`
}

// PatchEnvironment applies a mixed, sparse environment patch after validating
// the complete request. It writes both OpenBao maps, projects both Kubernetes
// Secrets, and persists one App patch with either zero or one restartedAt bump.
// If a later write or projection fails, already-written source/projection state
// is restored best-effort and the compensation error is joined to the cause.
func (s *Service) PatchEnvironment(ctx context.Context, service string, patch EnvironmentPatch) (EnvironmentPatchResult, error) {
	a, ctx, service, err := s.scope(ctx, core.RelCanCreate, service)
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	if patch.SaveMode != SaveModeOnly && patch.SaveMode != SaveModeDeploy {
		return EnvironmentPatchResult{}, fmt.Errorf("%w: saveMode must be %q or %q", core.ErrBadRequest, SaveModeOnly, SaveModeDeploy)
	}
	if patch.ExpectedEnvRevision != nil {
		return s.patchEnvironmentCAS(ctx, service, a, patch)
	}
	return s.patchEnvironmentSparse(ctx, service, a, patch)
}

// envPatchTxn carries what compensation needs to restore the state a failed
// patch already wrote: the pre-patch App and source maps, which maps changed,
// and — for revision-aware patches — the CAS write and its projection.
type envPatchTxn struct {
	service         string
	originalApp     *appv1alpha1.App
	oldEnv          map[string]string
	oldFiles        map[string]string
	envChanged      bool
	filesChanged    bool
	casWriteVersion *uint64
	casProjection   casEnvProjection
}

// patchEnvironmentCAS is the revision-aware protocol: exactly one ordinary
// env-var value update, compare-and-set against the caller's observed revision,
// and no secret-file operations.
func (s *Service) patchEnvironmentCAS(ctx context.Context, service string, a *appv1alpha1.App, patch EnvironmentPatch) (EnvironmentPatchResult, error) {
	casKey, err := validateCASPatch(patch)
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	expectedVersion, err := decodeEnvRevision(*patch.ExpectedEnvRevision)
	if err != nil {
		return EnvironmentPatchResult{}, core.NewBadRequestError(
			"ENVIRONMENT_REVISION_INVALID",
			"expectedEnvRevision is invalid",
			nil,
		)
	}
	versionedStore, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return EnvironmentPatchResult{}, fmt.Errorf("%w: environment revisions require a versioned secret store", core.ErrSecretsUnavailable)
	}
	snapshot, err := versionedStore.GetVersioned(ctx, envPath(service))
	if err != nil {
		return EnvironmentPatchResult{}, envSourceUnavailable()
	}
	oldEnv := snapshot.Data
	if snapshot.Version != expectedVersion {
		return EnvironmentPatchResult{}, envRevisionConflict()
	}
	if _, exists := oldEnv[casKey]; !exists {
		return EnvironmentPatchResult{}, core.NewNotFoundError(
			"ENVIRONMENT_VARIABLE_NOT_FOUND",
			"environment variable was not found",
			nil,
		)
	}
	env := core.CloneStringMap(oldEnv)
	if err := applyEnvPatch(env, patch.EnvVars); err != nil {
		return EnvironmentPatchResult{}, err
	}
	envChanged := !maps.Equal(oldEnv, env)
	result := environmentPatchResult(env, nil, false)

	newVersion, putErr := versionedStore.PutCAS(ctx, envPath(service), env, expectedVersion)
	if putErr != nil {
		if errors.Is(putErr, core.ErrConflict) {
			return EnvironmentPatchResult{}, envRevisionConflict()
		}
		return EnvironmentPatchResult{}, envSourceUnavailable()
	}
	txn := envPatchTxn{
		service:         service,
		originalApp:     a.DeepCopy(),
		oldEnv:          oldEnv,
		envChanged:      envChanged,
		casWriteVersion: &newVersion,
	}
	// A compare-and-set of an unchanged value still advances the opaque
	// revision, which is what makes two submissions from one observed revision
	// resolve to exactly one success and one conflict. Claim the derived Secret
	// for that new source version as well, but do not persist an App change or
	// roll pods because the effective environment did not change.
	if !envChanged {
		projection, projectionErr := s.projectCASEnv(ctx, service, a, env, newVersion)
		if projectionErr != nil {
			txn.casProjection = projection
			return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, txn, projectionErr)
		}
		return result, nil
	}
	return s.finalizeEnvironmentPatch(ctx, a, txn, patch.SaveMode, env, nil, result)
}

// patchEnvironmentSparse is the batch protocol: any mix of env-var and
// secret-file operations applied to the current maps with no revision check.
func (s *Service) patchEnvironmentSparse(ctx context.Context, service string, a *appv1alpha1.App, patch EnvironmentPatch) (EnvironmentPatchResult, error) {
	oldEnv, err := s.readMap(ctx, envPath(service))
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	oldFiles, err := s.readMap(ctx, filesPath(service))
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	env := core.CloneStringMap(oldEnv)
	files := core.CloneStringMap(oldFiles)
	if err := applyEnvPatch(env, patch.EnvVars); err != nil {
		return EnvironmentPatchResult{}, err
	}
	if err := applyFilePatch(files, patch.SecretFiles); err != nil {
		return EnvironmentPatchResult{}, err
	}
	envChanged := !maps.Equal(oldEnv, env)
	filesChanged := !maps.Equal(oldFiles, files)
	result := environmentPatchResult(env, files, false)
	if !envChanged && !filesChanged {
		return result, nil
	}
	if envChanged {
		if err := s.storeMap(ctx, envPath(service), env); err != nil {
			return EnvironmentPatchResult{}, err
		}
	}
	if filesChanged {
		if err := s.storeMap(ctx, filesPath(service), files); err != nil {
			return EnvironmentPatchResult{}, errors.Join(err, s.restoreSourceMaps(ctx, service, oldEnv, oldFiles, envChanged, false))
		}
	}
	txn := envPatchTxn{
		service:      service,
		originalApp:  a.DeepCopy(),
		oldEnv:       oldEnv,
		oldFiles:     oldFiles,
		envChanged:   envChanged,
		filesChanged: filesChanged,
	}
	return s.finalizeEnvironmentPatch(ctx, a, txn, patch.SaveMode, env, files, result)
}

// finalizeEnvironmentPatch projects the changed maps onto the derived Secrets
// and persists at most one App patch, staging or activating the projection
// references per saveMode. Any failure compensates through txn.
func (s *Service) finalizeEnvironmentPatch(ctx context.Context, a *appv1alpha1.App, txn envPatchTxn, saveMode SaveMode, env, files map[string]string, result EnvironmentPatchResult) (EnvironmentPatchResult, error) {
	base := client.MergeFrom(txn.originalApp)
	if txn.envChanged {
		var err error
		if txn.casWriteVersion != nil {
			txn.casProjection, err = s.projectCASEnv(ctx, txn.service, a, env, *txn.casWriteVersion)
		} else {
			err = s.projectEnv(ctx, a, env)
		}
		if err != nil {
			return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, txn, err)
		}
	}
	if txn.filesChanged {
		if err := s.projectFiles(ctx, a, files); err != nil {
			return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, txn, err)
		}
	}
	rolledOut := saveMode == SaveModeDeploy
	if rolledOut {
		activatePendingProjectionReferences(a)
		s.bumpRestart(a)
	} else {
		stagePendingProjectionReferences(a, txn.originalApp, env, files, txn.envChanged, txn.filesChanged)
	}
	if apiequality.Semantic.DeepEqual(txn.originalApp, a) {
		return result, nil
	}
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, txn, err)
	}
	result.RolledOut = rolledOut
	return result, nil
}

// validateCASPatch keeps the revision-aware surface deliberately narrow: one
// ordinary key/value assignment and no file, rename, delete, or generation
// operation. Legacy callers that omit ExpectedEnvRevision retain the complete
// sparse batch contract above. It returns the trimmed target key.
func validateCASPatch(patch EnvironmentPatch) (string, error) {
	if len(patch.EnvVars) != 1 || len(patch.SecretFiles) != 0 {
		return "", core.NewBadRequestError(
			"INVALID_ENVIRONMENT_CAS_PATCH",
			"expectedEnvRevision requires exactly one environment variable update and no secret files",
			nil,
		)
	}
	write := patch.EnvVars[0]
	key := strings.TrimSpace(write.Key)
	if !core.ValidEnvKey(key) {
		return "", core.NewBadRequestError(
			"ENVIRONMENT_VARIABLE_INVALID",
			"environment variable key is invalid",
			nil,
		)
	}
	if strings.TrimSpace(write.FromKey) != "" || write.Delete || write.GenerateValue {
		return "", core.NewBadRequestError(
			"INVALID_ENVIRONMENT_CAS_PATCH",
			"expectedEnvRevision supports only an ordinary environment variable value update",
			nil,
		)
	}
	return key, nil
}

func envRevisionConflict() error {
	return core.NewConflictError(
		"ENVIRONMENT_REVISION_CONFLICT",
		"the service environment changed; refresh it before saving again",
		nil,
	)
}

func envUpdateRestored() error {
	return core.NewConflictError(
		"ENVIRONMENT_UPDATE_RESTORED",
		"the environment update failed and its previous state was restored",
		nil,
	)
}

func envRestorationFailed() error {
	return core.NewConflictError(
		"ENVIRONMENT_RESTORATION_FAILED",
		"the environment update failed and could not be safely restored; refresh before retrying",
		nil,
	)
}

// casEnvProjection records whether the revision-owned projection replaced an
// existing Secret or created a new one. Compensation uses that distinction only
// after proving that OwnerVersion still owns the current object.
type casEnvProjection struct {
	OwnerVersion  uint64
	ExistedBefore bool
}

// projectCASEnv publishes a version-owned derived Secret. The source version is
// checked immediately before Kubernetes mutation, and the Secret's native
// resourceVersion/UID make a concurrent update or delete lose safely instead of
// overwriting a newer projection.
func (s *Service) projectCASEnv(ctx context.Context, service string, a *appv1alpha1.App, env map[string]string, ownerVersion uint64) (casEnvProjection, error) {
	ownership := casEnvProjection{OwnerVersion: ownerVersion}
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return ownership, core.ErrSecretsUnavailable
	}
	snapshot, err := versioned.GetVersioned(ctx, envPath(service))
	if err != nil {
		return ownership, safeCASProjectionError(err)
	}
	if snapshot.Version != ownerVersion || !maps.Equal(snapshot.Data, env) {
		return ownership, envRevisionConflict()
	}

	name := envSecretName(a.Name)
	sec := &corev1.Secret{}
	err = s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: name}, sec)
	if apierrors.IsNotFound(err) {
		sec = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   a.Namespace,
			Annotations: map[string]string{envProjectionRevisionAnnotation: encodeEnvRevision(ownerVersion)},
		}}
		sec.Type = corev1.SecretTypeOpaque
		sec.Data = envBytes(env)
		if ownerErr := controllerutil.SetControllerReference(a, sec, s.Client.Scheme()); ownerErr != nil {
			return ownership, safeCASProjectionError(ownerErr)
		}
		if createErr := s.Client.Create(ctx, sec); createErr != nil {
			return ownership, safeCASProjectionError(createErr)
		}
		a.Spec.EnvFromSecret = name
		return ownership, nil
	}
	if err != nil {
		return ownership, safeCASProjectionError(err)
	}
	ownership.ExistedBefore = true
	if current := sec.Annotations[envProjectionRevisionAnnotation]; current != "" {
		currentVersion, decodeErr := decodeEnvRevision(current)
		if decodeErr != nil || currentVersion > ownerVersion {
			return ownership, envRevisionConflict()
		}
		if currentVersion == ownerVersion {
			if !equalSecretData(sec.Data, env) {
				return ownership, envRevisionConflict()
			}
			a.Spec.EnvFromSecret = name
			return ownership, nil
		}
	}
	if sec.Annotations == nil {
		sec.Annotations = map[string]string{}
	}
	sec.Annotations[envProjectionRevisionAnnotation] = encodeEnvRevision(ownerVersion)
	sec.Type = corev1.SecretTypeOpaque
	sec.Data = envBytes(env)
	if ownerErr := controllerutil.SetControllerReference(a, sec, s.Client.Scheme()); ownerErr != nil {
		return ownership, safeCASProjectionError(ownerErr)
	}
	// Update carries the exact UID and resourceVersion read above. Kubernetes
	// refuses it if the object was replaced or changed after our ownership check.
	if updateErr := s.Client.Update(ctx, sec); updateErr != nil {
		return ownership, safeCASProjectionError(updateErr)
	}
	a.Spec.EnvFromSecret = name
	return ownership, nil
}

func safeCASProjectionError(err error) error {
	if apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) || errors.Is(err, core.ErrConflict) {
		return envRevisionConflict()
	}
	// Kubernetes and transport errors can contain object names or request paths.
	// The revision-aware mobile surface returns a constant public failure instead.
	return errors.New("environment projection is unavailable")
}

func envBytes(env map[string]string) map[string][]byte {
	out := make(map[string][]byte, len(env))
	for key, value := range env {
		out[key] = []byte(value)
	}
	return out
}

func equalSecretData(data map[string][]byte, env map[string]string) bool {
	return maps.EqualFunc(data, env, func(stored []byte, value string) bool {
		return string(stored) == value
	})
}

// stagePendingProjectionReferences keeps a save-only write metadata-only when
// the App has never consumed its conventional service-local Secret. The
// operator's generation-only predicate ignores this patch, then reads the
// pending name on the next deliberate App reconcile. Existing references stay
// untouched, so updating a projected Secret never changes the pod template.
func stagePendingProjectionReferences(a, original *appv1alpha1.App, env, files map[string]string, envChanged, filesChanged bool) {
	a.Spec.EnvFromSecret = original.Spec.EnvFromSecret
	a.Spec.FilesFromSecrets = append([]string(nil), original.Spec.FilesFromSecrets...)
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	if envChanged {
		if original.Spec.EnvFromSecret == "" && len(env) > 0 {
			a.Annotations[appv1alpha1.PendingEnvSecretAnnotation] = envSecretName(a.Name)
		} else {
			delete(a.Annotations, appv1alpha1.PendingEnvSecretAnnotation)
		}
	}
	if filesChanged {
		name := filesSecretName(a.Name)
		if !slices.Contains(original.Spec.FilesFromSecrets, name) && len(files) > 0 {
			a.Annotations[appv1alpha1.PendingFilesSecretAnnotation] = name
		} else {
			delete(a.Annotations, appv1alpha1.PendingFilesSecretAnnotation)
		}
	}
	if len(a.Annotations) == 0 {
		a.Annotations = nil
	}
}

// activatePendingProjectionReferences folds prior save-only configuration into
// the same spec patch that requests a rollout. It is especially important when
// this patch changes files while an earlier env-only save is still pending (or
// vice versa): both projections become active in the one requested rollout.
func activatePendingProjectionReferences(a *appv1alpha1.App) {
	if name := a.Annotations[appv1alpha1.PendingEnvSecretAnnotation]; a.Spec.EnvFromSecret == "" && name != "" {
		a.Spec.EnvFromSecret = name
	}
	if name := a.Annotations[appv1alpha1.PendingFilesSecretAnnotation]; name != "" {
		a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, name)
	}
	delete(a.Annotations, appv1alpha1.PendingEnvSecretAnnotation)
	delete(a.Annotations, appv1alpha1.PendingFilesSecretAnnotation)
	if len(a.Annotations) == 0 {
		a.Annotations = nil
	}
}

func applyEnvPatch(env map[string]string, writes []EnvVarPatch) error {
	return core.ApplyEnvVarPatch(env, writes)
}

func applyFilePatch(files map[string]string, writes []SecretFilePatch) error {
	return core.ApplySecretFilePatch(files, writes)
}

func (s *Service) compensateEnvironment(ctx context.Context, txn envPatchTxn, cause error) error {
	if txn.casWriteVersion != nil {
		return s.compensateCASEnvironment(ctx, txn.service, txn.originalApp, txn.oldEnv, *txn.casWriteVersion, txn.casProjection, cause)
	}
	originalApp := txn.originalApp
	var compensation []error
	if err := s.restoreSourceMaps(ctx, txn.service, txn.oldEnv, txn.oldFiles, txn.envChanged, txn.filesChanged); err != nil {
		compensation = append(compensation, fmt.Errorf("restore secret store: %w", err))
	}
	if txn.envChanged {
		if originalApp.Spec.EnvFromSecret == envSecretName(originalApp.Name) {
			if err := s.upsertSecret(ctx, originalApp, envSecretName(originalApp.Name), txn.oldEnv); err != nil {
				compensation = append(compensation, fmt.Errorf("restore environment projection: %w", err))
			}
		} else if err := s.deleteSecret(ctx, originalApp.Namespace, envSecretName(originalApp.Name)); err != nil {
			compensation = append(compensation, fmt.Errorf("remove environment projection: %w", err))
		}
	}
	if txn.filesChanged {
		name := filesSecretName(originalApp.Name)
		if slices.Contains(originalApp.Spec.FilesFromSecrets, name) {
			if err := s.upsertSecret(ctx, originalApp, name, txn.oldFiles); err != nil {
				compensation = append(compensation, fmt.Errorf("restore secret-file projection: %w", err))
			}
		} else if err := s.deleteSecret(ctx, originalApp.Namespace, name); err != nil {
			compensation = append(compensation, fmt.Errorf("remove secret-file projection: %w", err))
		}
	}
	return errors.Join(append([]error{cause}, compensation...)...)
}

// compensateCASEnvironment restores source first, then rolls the projection
// back only while the failed write's exact revision still owns it. A conflict
// returns the coded error directly (rather than errors.Join) so graphql-go keeps
// extensions.code and no underlying store/Kubernetes detail crosses the API.
func (s *Service) compensateCASEnvironment(ctx context.Context, service string, originalApp *appv1alpha1.App, oldEnv map[string]string, casWriteVersion uint64, projection casEnvProjection, _ error) error {
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return envRestorationFailed()
	}
	restoredVersion, err := versioned.PutCAS(ctx, envPath(service), oldEnv, casWriteVersion)
	if err != nil {
		if errors.Is(err, core.ErrConflict) {
			return envRevisionConflict()
		}
		return envRestorationFailed()
	}
	if err := s.rollbackCASEnvProjection(ctx, originalApp, oldEnv, restoredVersion, projection); err != nil {
		if errors.Is(err, core.ErrConflict) {
			return envRevisionConflict()
		}
		return envRestorationFailed()
	}
	return envUpdateRestored()
}

func (s *Service) rollbackCASEnvProjection(ctx context.Context, originalApp *appv1alpha1.App, oldEnv map[string]string, restoredVersion uint64, projection casEnvProjection) error {
	name := envSecretName(originalApp.Name)
	sec := &corev1.Secret{}
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: originalApp.Namespace, Name: name}, sec)
	if apierrors.IsNotFound(err) {
		if projection.ExistedBefore {
			return envRevisionConflict()
		}
		return nil
	}
	if err != nil {
		return safeCASProjectionError(err)
	}
	if sec.Annotations[envProjectionRevisionAnnotation] != encodeEnvRevision(projection.OwnerVersion) {
		return envRevisionConflict()
	}
	if !projection.ExistedBefore {
		uid, resourceVersion := sec.UID, sec.ResourceVersion
		// UID prevents deleting a same-name replacement; resourceVersion prevents
		// deleting an object a newer writer updated after the ownership check.
		if err := s.Client.Delete(ctx, sec, client.Preconditions{UID: &uid, ResourceVersion: &resourceVersion}); err != nil {
			return safeCASProjectionError(err)
		}
		return nil
	}
	if sec.Annotations == nil {
		sec.Annotations = map[string]string{}
	}
	sec.Annotations[envProjectionRevisionAnnotation] = encodeEnvRevision(restoredVersion)
	sec.Type = corev1.SecretTypeOpaque
	sec.Data = envBytes(oldEnv)
	// Update is conditional on the exact UID/resourceVersion returned by Get.
	if err := s.Client.Update(ctx, sec); err != nil {
		return safeCASProjectionError(err)
	}
	return nil
}

func (s *Service) restoreSourceMaps(ctx context.Context, service string, oldEnv, oldFiles map[string]string, envChanged, filesChanged bool) error {
	var errs []error
	if envChanged {
		if err := s.storeMap(ctx, envPath(service), oldEnv); err != nil {
			errs = append(errs, err)
		}
	}
	if filesChanged {
		if err := s.storeMap(ctx, filesPath(service), oldFiles); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func environmentPatchResult(env, files map[string]string, rolledOut bool) EnvironmentPatchResult {
	return EnvironmentPatchResult{
		EnvVarKeys:      slices.Sorted(maps.Keys(env)),
		SecretFileNames: slices.Sorted(maps.Keys(files)),
		RolledOut:       rolledOut,
	}
}
