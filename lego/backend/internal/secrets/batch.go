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
	"sort"
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
type EnvVarPatch struct {
	Key           string `json:"key"`
	FromKey       string `json:"fromKey,omitempty"`
	Value         string `json:"value,omitempty"`
	GenerateValue bool   `json:"generateValue,omitempty"`
	Delete        bool   `json:"delete,omitempty"`
}

// SecretFilePatch is one explicit secret-file mutation. Omitted files are
// preserved, and contents never appear in PatchEnvironment's result.
type SecretFilePatch struct {
	Name     string `json:"name"`
	FromName string `json:"fromName,omitempty"`
	Content  string `json:"content,omitempty"`
	Delete   bool   `json:"delete,omitempty"`
}

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
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return EnvironmentPatchResult{}, core.ErrSecretsUnavailable
	}
	if patch.SaveMode != SaveModeOnly && patch.SaveMode != SaveModeDeploy {
		return EnvironmentPatchResult{}, fmt.Errorf("%w: saveMode must be %q or %q", core.ErrBadRequest, SaveModeOnly, SaveModeDeploy)
	}

	var (
		oldEnv          map[string]string
		expectedVersion uint64
		versionedStore  core.VersionedSecretKV
		casMode         = patch.ExpectedEnvRevision != nil
	)
	if casMode {
		if err := validateCASPatch(patch); err != nil {
			return EnvironmentPatchResult{}, err
		}
		expectedVersion, err = decodeEnvRevision(*patch.ExpectedEnvRevision)
		if err != nil {
			return EnvironmentPatchResult{}, core.NewBadRequestError(
				"ENVIRONMENT_REVISION_INVALID",
				"expectedEnvRevision is invalid",
				nil,
			)
		}
		var ok bool
		versionedStore, ok = s.Store.(core.VersionedSecretKV)
		if !ok {
			return EnvironmentPatchResult{}, fmt.Errorf("%w: environment revisions require a versioned secret store", core.ErrSecretsUnavailable)
		}
		snapshot, snapshotErr := versionedStore.GetVersioned(ctx, envPath(service))
		if snapshotErr != nil {
			return EnvironmentPatchResult{}, envSourceUnavailable()
		}
		oldEnv = snapshot.Data
		if snapshot.Version != expectedVersion {
			return EnvironmentPatchResult{}, envRevisionConflict()
		}
		if _, exists := oldEnv[strings.TrimSpace(patch.EnvVars[0].Key)]; !exists {
			return EnvironmentPatchResult{}, core.NewNotFoundError(
				"ENVIRONMENT_VARIABLE_NOT_FOUND",
				"environment variable was not found",
				nil,
			)
		}
	} else {
		oldEnv, err = s.readMap(ctx, envPath(service))
		if err != nil {
			return EnvironmentPatchResult{}, err
		}
	}
	oldFiles := map[string]string{}
	if !casMode {
		oldFiles, err = s.readMap(ctx, filesPath(service))
		if err != nil {
			return EnvironmentPatchResult{}, err
		}
	}
	env := cloneStringMap(oldEnv)
	files := cloneStringMap(oldFiles)
	if err := applyEnvPatch(env, patch.EnvVars); err != nil {
		return EnvironmentPatchResult{}, err
	}
	if !casMode {
		if err := applyFilePatch(files, patch.SecretFiles); err != nil {
			return EnvironmentPatchResult{}, err
		}
	}

	envChanged := !maps.Equal(oldEnv, env)
	filesChanged := !maps.Equal(oldFiles, files)
	result := environmentPatchResult(env, files, false)
	if !casMode && !envChanged && !filesChanged {
		return result, nil
	}

	var casWriteVersion *uint64
	if casMode {
		newVersion, putErr := versionedStore.PutCAS(ctx, envPath(service), env, expectedVersion)
		if putErr != nil {
			if errors.Is(putErr, core.ErrConflict) {
				return EnvironmentPatchResult{}, envRevisionConflict()
			}
			return EnvironmentPatchResult{}, envSourceUnavailable()
		}
		casWriteVersion = &newVersion
	} else if envChanged {
		if err := s.storeMap(ctx, envPath(service), env); err != nil {
			return EnvironmentPatchResult{}, err
		}
	}
	if filesChanged {
		if err := s.storeMap(ctx, filesPath(service), files); err != nil {
			return EnvironmentPatchResult{}, errors.Join(err, s.restoreSourceMaps(ctx, service, oldEnv, oldFiles, envChanged, false))
		}
	}
	// A compare-and-set of an unchanged value still advances the opaque
	// revision, which is what makes two submissions from one observed revision
	// resolve to exactly one success and one conflict. Claim the derived Secret
	// for that new source version as well, but do not persist an App change or
	// roll pods because the effective environment did not change.
	if !envChanged && !filesChanged {
		if casWriteVersion != nil {
			originalApp := a.DeepCopy()
			projection, projectionErr := s.projectCASEnv(ctx, service, a, env, *casWriteVersion)
			if projectionErr != nil {
				return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, service, originalApp, oldEnv, oldFiles, false, false, casWriteVersion, projection, projectionErr)
			}
		}
		return result, nil
	}

	originalApp := a.DeepCopy()
	base := client.MergeFrom(a.DeepCopy())
	var casProjection casEnvProjection
	undo := func(cause error) (EnvironmentPatchResult, error) {
		return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, service, originalApp, oldEnv, oldFiles, envChanged, filesChanged, casWriteVersion, casProjection, cause)
	}
	if envChanged {
		if casWriteVersion != nil {
			casProjection, err = s.projectCASEnv(ctx, service, a, env, *casWriteVersion)
		} else {
			err = s.projectEnv(ctx, a, env)
		}
		if err != nil {
			return undo(err)
		}
	}
	if filesChanged {
		if err := s.projectFiles(ctx, a, files); err != nil {
			return undo(err)
		}
	}
	rolledOut := patch.SaveMode == SaveModeDeploy
	if rolledOut {
		activatePendingProjectionReferences(a)
		s.bumpRestart(a)
	} else {
		stagePendingProjectionReferences(a, originalApp, env, files, envChanged, filesChanged)
	}
	if apiequality.Semantic.DeepEqual(originalApp, a) {
		return result, nil
	}
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return undo(err)
	}
	result.RolledOut = rolledOut
	return result, nil
}

// validateCASPatch keeps the revision-aware surface deliberately narrow: one
// ordinary key/value assignment and no file, rename, delete, or generation
// operation. Legacy callers that omit ExpectedEnvRevision retain the complete
// sparse batch contract above.
func validateCASPatch(patch EnvironmentPatch) error {
	if len(patch.EnvVars) != 1 || len(patch.SecretFiles) != 0 {
		return core.NewBadRequestError(
			"INVALID_ENVIRONMENT_CAS_PATCH",
			"expectedEnvRevision requires exactly one environment variable update and no secret files",
			nil,
		)
	}
	write := patch.EnvVars[0]
	if !core.ValidEnvKey(strings.TrimSpace(write.Key)) {
		return core.NewBadRequestError(
			"ENVIRONMENT_VARIABLE_INVALID",
			"environment variable key is invalid",
			nil,
		)
	}
	if strings.TrimSpace(write.FromKey) != "" || write.Delete || write.GenerateValue {
		return core.NewBadRequestError(
			"INVALID_ENVIRONMENT_CAS_PATCH",
			"expectedEnvRevision supports only an ordinary environment variable value update",
			nil,
		)
	}
	return nil
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
	if len(data) != len(env) {
		return false
	}
	for key, value := range env {
		stored, ok := data[key]
		if !ok || string(stored) != value {
			return false
		}
	}
	return true
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
	seen := make(map[string]struct{}, len(writes))
	for _, write := range writes {
		key := strings.TrimSpace(write.Key)
		if !core.ValidEnvKey(key) {
			return fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		fromKey := strings.TrimSpace(write.FromKey)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("%w: duplicate environment variable operation for %q", core.ErrBadRequest, key)
		}
		seen[key] = struct{}{}
		if fromKey != "" {
			if !core.ValidEnvKey(fromKey) {
				return fmt.Errorf("%w: invalid source environment variable name %q", core.ErrBadRequest, fromKey)
			}
			if write.Delete || write.GenerateValue || write.Value != "" {
				return fmt.Errorf("%w: environment variable rename %q cannot combine with delete, value, or generateValue", core.ErrBadRequest, key)
			}
			if _, duplicate := seen[fromKey]; duplicate && fromKey != key {
				return fmt.Errorf("%w: conflicting environment variable operation for %q", core.ErrBadRequest, fromKey)
			}
			value, ok := env[fromKey]
			if !ok {
				return fmt.Errorf("%w: source environment variable %q", core.ErrNotFound, fromKey)
			}
			if _, occupied := env[key]; occupied && key != fromKey {
				return fmt.Errorf("%w: environment variable rename destination %q already exists", core.ErrBadRequest, key)
			}
			seen[fromKey] = struct{}{}
			delete(env, fromKey)
			env[key] = value
			continue
		}
		if write.Delete {
			if write.GenerateValue || write.Value != "" {
				return fmt.Errorf("%w: environment variable %q cannot combine delete with a value or generateValue", core.ErrBadRequest, key)
			}
			delete(env, key)
			continue
		}
		value, err := resolveValue(key, write.Value, write.GenerateValue)
		if err != nil {
			return err
		}
		env[key] = value
	}
	return nil
}

func applyFilePatch(files map[string]string, writes []SecretFilePatch) error {
	seen := make(map[string]struct{}, len(writes))
	for _, write := range writes {
		name := strings.TrimSpace(write.Name)
		if !core.ValidSecretFileName(name) {
			return fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, name)
		}
		fromName := strings.TrimSpace(write.FromName)
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("%w: duplicate secret file operation for %q", core.ErrBadRequest, name)
		}
		seen[name] = struct{}{}
		if fromName != "" {
			if !core.ValidSecretFileName(fromName) {
				return fmt.Errorf("%w: invalid source secret file name %q", core.ErrBadRequest, fromName)
			}
			if write.Delete || write.Content != "" {
				return fmt.Errorf("%w: secret file rename %q cannot combine with delete or content", core.ErrBadRequest, name)
			}
			if _, duplicate := seen[fromName]; duplicate && fromName != name {
				return fmt.Errorf("%w: conflicting secret file operation for %q", core.ErrBadRequest, fromName)
			}
			content, ok := files[fromName]
			if !ok {
				return fmt.Errorf("%w: source secret file %q", core.ErrNotFound, fromName)
			}
			if _, occupied := files[name]; occupied && name != fromName {
				return fmt.Errorf("%w: secret file rename destination %q already exists", core.ErrBadRequest, name)
			}
			seen[fromName] = struct{}{}
			delete(files, fromName)
			files[name] = content
			continue
		}
		if write.Delete {
			if write.Content != "" {
				return fmt.Errorf("%w: secret file %q cannot combine delete with content", core.ErrBadRequest, name)
			}
			delete(files, name)
			continue
		}
		files[name] = write.Content
	}
	return nil
}

func (s *Service) compensateEnvironment(ctx context.Context, service string, originalApp *appv1alpha1.App, oldEnv, oldFiles map[string]string, envChanged, filesChanged bool, casWriteVersion *uint64, casProjection casEnvProjection, cause error) error {
	if casWriteVersion != nil {
		return s.compensateCASEnvironment(ctx, service, originalApp, oldEnv, *casWriteVersion, casProjection, cause)
	}
	var compensation []error
	if err := s.restoreSourceMaps(ctx, service, oldEnv, oldFiles, envChanged, filesChanged); err != nil {
		compensation = append(compensation, fmt.Errorf("restore secret store: %w", err))
	}
	if envChanged {
		if originalApp.Spec.EnvFromSecret == envSecretName(originalApp.Name) {
			if err := s.upsertSecret(ctx, originalApp, envSecretName(originalApp.Name), oldEnv); err != nil {
				compensation = append(compensation, fmt.Errorf("restore environment projection: %w", err))
			}
		} else if err := s.deleteSecret(ctx, originalApp.Namespace, envSecretName(originalApp.Name)); err != nil {
			compensation = append(compensation, fmt.Errorf("remove environment projection: %w", err))
		}
	}
	if filesChanged {
		name := filesSecretName(originalApp.Name)
		if slices.Contains(originalApp.Spec.FilesFromSecrets, name) {
			if err := s.upsertSecret(ctx, originalApp, name, oldFiles); err != nil {
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
	result := EnvironmentPatchResult{RolledOut: rolledOut}
	for key := range env {
		result.EnvVarKeys = append(result.EnvVarKeys, key)
	}
	for name := range files {
		result.SecretFileNames = append(result.SecretFileNames, name)
	}
	sort.Strings(result.EnvVarKeys)
	sort.Strings(result.SecretFileNames)
	return result
}

// cloneStringMap is deliberately not maps.Clone: a nil input must yield a
// writable empty map (the patch appliers mutate the clone in place).
func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
