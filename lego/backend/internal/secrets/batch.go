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
	"crypto/rand"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// SaveMode controls whether a coherent environment patch only updates stored
// and projected configuration or also rolls the service once. Rebuilding source
// is intentionally composed by the deploys feature after a successful save.
type SaveMode string

const (
	SaveModeOnly   SaveMode = "save_only"
	SaveModeDeploy SaveMode = "deploy"
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

// EnvironmentPatchResult contains names and an opaque revision, never secret material.
type EnvironmentPatchResult struct {
	EnvVarKeys      []string `json:"envVarKeys"`
	SecretFileNames []string `json:"secretFileNames"`
	RolledOut       bool     `json:"rolledOut"`
	// Revision is null for sparse patches, which make no caller-visible CAS promise.
	Revision *string `json:"revision"`
}

// PatchEnvironment applies a mixed, sparse environment patch after validating
// the complete request. It writes both OpenBao maps, projects both Kubernetes
// Secrets, and persists one App patch with either zero or one restartedAt bump.
// If a later write or projection fails, already-written source/projection state
// is restored best-effort and a compensation error is joined to the cause. A
// write a newer one superseded is left to it instead, and the call answers
// ENVIRONMENT_RESTORATION_FAILED, since its change may survive in that write.
func (s *Service) PatchEnvironment(ctx context.Context, service string, patch EnvironmentPatch) (EnvironmentPatchResult, error) {
	a, ctx, service, err := s.scopeForWrite(ctx, core.RelCanCreate, service)
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	// A manifest-owned key refuses the whole patch before anything is written
	// (w4/m120) — including the rename source, since a rename deletes it.
	keys := make([]string, 0, len(patch.EnvVars)*2)
	for _, v := range patch.EnvVars {
		keys = append(keys, v.Key, v.FromKey)
	}
	if err := refuseManifestKeys(a, keys...); err != nil {
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
// write already wrote: the App before its projection, each source map's write,
// and — for a revision-aware patch (cas), whose env write always advances the
// revision — the projection it claimed.
type envPatchTxn struct {
	service       string
	originalApp   *appv1alpha1.App
	env, files    mapWrite
	cas           bool
	casProjection casEnvProjection
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
	if err := patchWithinQuota(oldEnv, env, envMapWithinQuota); err != nil {
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
	revision := encodeEnvRevision(newVersion)
	result.Revision = &revision
	txn := envPatchTxn{
		service:     service,
		originalApp: a.DeepCopy(),
		env:         mapWrite{prior: oldEnv, version: newVersion, changed: envChanged},
		cas:         true,
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
		// The effective environment did not change: no App write, no roll, and
		// so no event (w4/m122).
		return result, nil
	}
	return s.finalizeEnvironmentPatch(ctx, a, txn, patch.SaveMode, env, nil, result)
}

// patchEnvironmentSparse is the batch protocol: any mix of env-var and
// secret-file operations applied to the current maps with no caller-supplied
// revision check.
//
// codex-security round-19 #7: each map is written through updateMapCAS's
// GetVersioned/PutCAS retry loop instead of a bare readMap+storeMap, so a
// concurrent single-field writer (SetEnvVar, SetSecretFile, ...) between the
// read and this write is never silently discarded — the patch re-applies
// against the latest committed map on every CAS retry. This keeps the sparse
// contract (no ExpectedEnvRevision, no conflict surfaced to the caller) while
// closing the lost-update race; only patchEnvironmentCAS exposes revisions.
func (s *Service) patchEnvironmentSparse(ctx context.Context, service string, a *appv1alpha1.App, patch EnvironmentPatch) (EnvironmentPatchResult, error) {
	var applyErr error

	// Each mutate checks the quota on the map it is about to write, so the check
	// re-runs against the latest committed map on every CAS retry (ADR066 #6).
	// A refused map is not written: the mutate reports "unchanged".
	env, envWrite, err := s.updateMapCAS(ctx, envPath(service), func(current map[string]string) bool {
		before := core.CloneStringMap(current)
		if err := applyEnvPatch(current, patch.EnvVars); err != nil {
			applyErr = err
			return false
		}
		if err := patchWithinQuota(before, current, envMapWithinQuota); err != nil {
			applyErr = err
			return false
		}
		return !maps.Equal(before, current)
	})
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	if applyErr != nil {
		return EnvironmentPatchResult{}, applyErr
	}

	files, filesWrite, err := s.updateMapCAS(ctx, filesPath(service), func(current map[string]string) bool {
		before := core.CloneStringMap(current)
		if err := applyFilePatch(current, patch.SecretFiles); err != nil {
			applyErr = err
			return false
		}
		if err := patchWithinQuota(before, current, filesMapWithinQuota); err != nil {
			applyErr = err
			return false
		}
		return !maps.Equal(before, current)
	})
	// The files write failed after the env write committed, before anything
	// was projected: put the env map back. A newer write that superseded it
	// keeps this patch's env change, so the call cannot say nothing was saved.
	undoEnv := func(cause error) error {
		if !envWrite.changed {
			return cause
		}
		_, superseded, err := s.restoreMap(ctx, envPath(service), envWrite)
		switch {
		case err != nil:
			return errors.Join(cause, err)
		case superseded:
			return envRestorationFailed()
		}
		return cause
	}
	if err != nil {
		return EnvironmentPatchResult{}, undoEnv(err)
	}
	if applyErr != nil {
		return EnvironmentPatchResult{}, undoEnv(applyErr)
	}

	result := environmentPatchResult(env, files, false)
	if !envWrite.changed && !filesWrite.changed {
		return result, nil
	}
	txn := envPatchTxn{service: service, originalApp: a.DeepCopy(), env: envWrite, files: filesWrite}
	return s.finalizeEnvironmentPatch(ctx, a, txn, patch.SaveMode, env, files, result)
}

// finalizeEnvironmentPatch projects the changed maps onto the derived Secrets
// and persists at most one App patch, staging or activating the projection
// references per saveMode. Any failure compensates through txn.
func (s *Service) finalizeEnvironmentPatch(ctx context.Context, a *appv1alpha1.App, txn envPatchTxn, saveMode SaveMode, env, files map[string]string, result EnvironmentPatchResult) (EnvironmentPatchResult, error) {
	before := rollout.Before(a)
	base := client.MergeFrom(txn.originalApp)
	if txn.env.changed {
		var err error
		if txn.cas {
			txn.casProjection, err = s.projectCASEnv(ctx, txn.service, a, env, txn.env.version)
		} else {
			err = s.projectEnv(ctx, a, env, committedAt(txn.env.version))
		}
		if err != nil {
			return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, txn, err)
		}
	}
	if txn.files.changed {
		if err := s.projectFiles(ctx, a, files, committedAt(txn.files.version)); err != nil {
			return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, txn, err)
		}
	}
	rolledOut := saveMode == SaveModeDeploy
	if rolledOut {
		activatePendingProjectionReferences(a)
		s.bumpRestart(a)
	} else {
		stagePendingProjectionReferences(a, txn.originalApp, env, files, txn.env.changed, txn.files.changed)
		if a.Annotations == nil {
			a.Annotations = map[string]string{}
		}
		// Existing references leave spec unchanged, but the saved Secret bytes
		// still need comparison with the serving release. A fresh, value-free
		// notification wakes only the operator's status reconciliation. Do not
		// use a timestamp: consecutive saves can share a clock tick.
		a.Annotations[appv1alpha1.AnnotationSavedConfigRevision] = rand.Text()
	}
	if apiequality.Semantic.DeepEqual(txn.originalApp, a) {
		// No runtime identity changed. Effective Save-only writes always carry
		// a fresh notification above; source no-ops return before finalization.
		return result, nil
	}
	tracked := before.Stamp(a)
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, txn, err)
	}
	s.RecordAppConfigChanged(ctx, a, core.AuditVerbPatchEnvironment)
	// save_only stages the projection references without touching release
	// identity, so only the deploying mode opens a row — exactly the rollout the
	// user just asked for, and nothing for the save they explicitly deferred.
	if tracked {
		s.Rollout.Open(ctx, before, a, store.TriggerConfigChange)
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
	if err := core.CheckEnvKey(key); err != nil {
		return "", err
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
	projection, err := s.projectSource(ctx, a, envProjection, env, committedAt(ownerVersion), false)
	ownership.ExistedBefore = projection.ExistedBefore
	if err != nil {
		return ownership, safeCASProjectionError(err)
	}
	if projection.Superseded {
		return ownership, envRevisionConflict()
	}
	a.Spec.EnvFromSecret = envSecretName(a.Name)
	return ownership, nil
}

func safeCASProjectionError(err error) error {
	if apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) || errors.Is(err, core.ErrConflict) || errors.Is(err, errProjectionConflict) {
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
// status controller observes its notification, while the runtime controller
// consumes the pending name only on the next release. Existing references stay
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

// compensateEnvironment undoes a write whose projection or App patch failed:
// the batch patch's and each single-map setter's (w5/m119). A changed map goes
// back to what the write replaced only while the write is still the store's
// latest (restoreMap), and its Secret then goes back with it, at the revision
// the restore committed (w5/m127). A map a newer committed write superseded is
// left to that write, and the call answers ENVIRONMENT_RESTORATION_FAILED: its
// own change may survive there.
func (s *Service) compensateEnvironment(ctx context.Context, txn envPatchTxn, cause error) error {
	app := txn.originalApp
	if txn.cas {
		return s.compensateCASEnvironment(ctx, txn.service, app, txn.env.prior, txn.env.version, txn.casProjection)
	}
	envSecret, filesSecret := envSecretName(app.Name), filesSecretName(app.Name)
	var compensation []error
	superseded := false
	for _, m := range []struct {
		write      mapWrite
		path       string
		kind       projectionKind
		label      string
		referenced bool
	}{{
		write: txn.env, path: envPath(txn.service), kind: envProjection, label: "environment",
		referenced: app.Spec.EnvFromSecret == envSecret || app.Annotations[appv1alpha1.PendingEnvSecretAnnotation] == envSecret,
	}, {
		write: txn.files, path: filesPath(txn.service), kind: filesProjection, label: "secret-file",
		referenced: slices.Contains(app.Spec.FilesFromSecrets, filesSecret) || app.Annotations[appv1alpha1.PendingFilesSecretAnnotation] == filesSecret,
	}} {
		if !m.write.changed {
			continue
		}
		restored, gone, err := s.restoreMap(ctx, m.path, m.write)
		if err != nil {
			compensation = append(compensation, fmt.Errorf("restore secret store: %w", err))
			continue
		}
		if gone {
			superseded = true
			continue
		}
		// The restored map goes back at the revision its restore committed, so
		// it cannot overwrite a projection a newer write landed since. Only a
		// Secret this write created for an empty map is removed. The App this
		// call read decides no more than that: a concurrent first write may
		// have referenced the Secret since, and the restored map is its own.
		remove := len(m.write.prior) == 0 && !m.referenced
		if _, err := s.projectSource(ctx, app, m.kind, m.write.prior, committedAt(restored), remove); err != nil {
			compensation = append(compensation, fmt.Errorf("restore %s projection: %w", m.label, err))
		}
	}
	switch {
	case len(compensation) > 0:
		return errors.Join(append([]error{cause}, compensation...)...)
	case superseded:
		return envRestorationFailed()
	case errors.Is(cause, errProjectionConflict):
		return secretChanged()
	}
	return refusedProjection(cause, app.Name)
}

// compensateCASEnvironment restores source first, then rolls the projection
// back only while the failed write's exact revision still owns it. Once the
// source write was accepted, a failed rollback is a restoration failure, never
// a revision refusal: the submitted value may still be stored. Return only the
// coded error so store/Kubernetes details do not cross the API.
func (s *Service) compensateCASEnvironment(ctx context.Context, service string, originalApp *appv1alpha1.App, oldEnv map[string]string, casWriteVersion uint64, projection casEnvProjection) error {
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return envRestorationFailed()
	}
	restoredVersion, err := versioned.PutCAS(ctx, envPath(service), oldEnv, casWriteVersion)
	if err != nil {
		return envRestorationFailed()
	}
	if err := s.rollbackCASEnvProjection(ctx, originalApp, oldEnv, restoredVersion, projection); err != nil {
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
	// Only this write's own committed revision is its to roll back: a delete's
	// provisional projection of that version is the delete's.
	if current, known := projectedRevision(sec, envProjection); !known || current != committedAt(projection.OwnerVersion) {
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
	if err := s.stampProjection(originalApp, sec, envProjection, oldEnv, committedAt(restoredVersion)); err != nil {
		return safeCASProjectionError(err)
	}
	// Update is conditional on the exact UID/resourceVersion returned by Get.
	if err := s.Client.Update(ctx, sec); err != nil {
		return safeCASProjectionError(err)
	}
	return nil
}

// patchWithinQuota is the quota rule for a batch patch (w1/m147): the map it
// produces must fit, unless the map was already over quota and the patch does
// not grow it. Deleting or shrinking entries frees room and stays possible
// (ADR066 #6); adding an entry or bytes to an over-quota map does not. Before
// w1/m147 the batch patch ran no check at all, so the dashboard's Environment
// editor stored and deployed a 614,400-byte secret file past the 512 KiB cap.
func patchWithinQuota(before, after map[string]string, within func(map[string]string) error) error {
	err := within(after)
	if err == nil || (len(after) <= len(before) && mapBytes(after) <= mapBytes(before)) {
		return nil
	}
	return err
}

func environmentPatchResult(env, files map[string]string, rolledOut bool) EnvironmentPatchResult {
	return EnvironmentPatchResult{
		EnvVarKeys:      slices.Sorted(maps.Keys(env)),
		SecretFileNames: slices.Sorted(maps.Keys(files)),
		RolledOut:       rolledOut,
	}
}
