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

package envgroups

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type SaveMode string

const (
	SaveModeOnly    SaveMode = "save_only"
	SaveModeDeploy  SaveMode = "deploy"
	SaveModeRebuild SaveMode = "rebuild"
)

type EnvVarPatch = core.EnvVarPatch
type SecretFilePatch = core.SecretFilePatch

// EnvironmentPatch is one sparse, opaque-preserving Environment Group draft
// commit. ExpectedRevision is the group-level token returned by list/get.
type EnvironmentPatch struct {
	EnvVars          []EnvVarPatch     `json:"envVars,omitempty"`
	SecretFiles      []SecretFilePatch `json:"secretFiles,omitempty"`
	SaveMode         SaveMode          `json:"saveMode"`
	ExpectedRevision *string           `json:"expectedRevision,omitempty"`
}

// EnvironmentPatchResult never includes values or file contents. A source
// commit remains successful when one or more later service actions fail; those
// ids are explicit so the client can retry an empty patch at Revision.
type EnvironmentPatchResult struct {
	EnvVarKeys         []string `json:"envVarKeys"`
	SecretFileNames    []string `json:"secretFileNames"`
	Revision           string   `json:"revision"`
	AffectedServiceIDs []string `json:"affectedServiceIds"`
	FailedServiceIDs   []string `json:"failedServiceIds"`
	RolledOut          bool     `json:"rolledOut"`
}

func revisionPath(gid string) string { return "env-groups/" + gid + "/revision" }

func revisionGeneration(data map[string]string) uint64 {
	generation, _ := strconv.ParseUint(data["generation"], 10, 64)
	return generation
}

func envGroupRevisionConflict() error {
	return core.NewConflictError(
		"ENV_GROUP_REVISION_CONFLICT",
		"the environment group changed; refresh it before saving again",
		nil,
	)
}

func envGroupRevisionInvalid() error {
	return core.NewBadRequestError(
		"ENV_GROUP_REVISION_INVALID",
		"expectedRevision is invalid",
		nil,
	)
}

func envGroupUpdateRestored() error {
	return core.NewConflictError(
		"ENV_GROUP_UPDATE_RESTORED",
		"the environment group update failed and its previous state was restored",
		nil,
	)
}

func envGroupRestorationFailed() error {
	return core.NewConflictError(
		"ENV_GROUP_RESTORATION_FAILED",
		"the environment group update failed and could not be safely restored; an operator must repair it",
		nil,
	)
}

type groupPatchTxn struct {
	gid                string
	versioned          core.VersionedSecretKV
	claimVersion       uint64
	generation         uint64
	oldEnv             map[string]string
	oldFiles           map[string]string
	oldMeta            meta
	envChanged         bool
	filesChanged       bool
	oldEnvProjection   *coreSecret
	oldFilesProjection *coreSecret
}

// coreSecret is the minimal projection snapshot compensation needs. Keeping a
// copy of the Kubernetes object also preserves metadata set outside this feature.
type coreSecret struct{ object *corev1.Secret }

// PatchEnvironment applies one group draft. A dedicated versioned lock path
// serializes the two OpenBao maps and their two Kubernetes projections across
// API replicas; unlike using either content path as a lock, a file-only update
// participates in the same revision protocol.
func (s *Service) PatchEnvironment(ctx context.Context, gid string, patch EnvironmentPatch) (EnvironmentPatchResult, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	return s.patchEnvironmentAuthorized(ctx, gid, m, patch)
}

func (s *Service) patchEnvironmentAuthorized(ctx context.Context, gid string, m meta, patch EnvironmentPatch) (EnvironmentPatchResult, error) {
	if patch.SaveMode != SaveModeOnly && patch.SaveMode != SaveModeDeploy && patch.SaveMode != SaveModeRebuild {
		return EnvironmentPatchResult{}, core.NewBadRequestError(
			"ENV_GROUP_SAVE_MODE_INVALID",
			"saveMode must be save_only, deploy, or rebuild",
			nil,
		)
	}
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return EnvironmentPatchResult{}, core.ErrSecretsUnavailable
	}
	revision, err := s.getRevisionSnapshot(ctx, versioned, m.workspace, gid)
	if err != nil || revision.Data["state"] == "repair_required" {
		return EnvironmentPatchResult{}, envGroupRestorationFailed()
	}
	if revision.Data["state"] == "busy" {
		return EnvironmentPatchResult{}, envGroupRevisionConflict()
	}
	generation := revisionGeneration(revision.Data)
	if patch.ExpectedRevision != nil {
		expected, decodeErr := decodeEnvGroupRevision(*patch.ExpectedRevision)
		if decodeErr != nil {
			return EnvironmentPatchResult{}, envGroupRevisionInvalid()
		}
		if expected != generation {
			return EnvironmentPatchResult{}, envGroupRevisionConflict()
		}
	}
	claimVersion, err := versioned.PutCAS(groupCtx(ctx, m.workspace), revisionPath(gid), map[string]string{
		"state": "busy", "generation": strconv.FormatUint(generation, 10),
	}, revision.Version)
	if err != nil {
		if errors.Is(err, core.ErrConflict) {
			return EnvironmentPatchResult{}, envGroupRevisionConflict()
		}
		return EnvironmentPatchResult{}, core.ErrSecretsUnavailable
	}

	oldEnv, err := s.getGroupMap(ctx, m.workspace, envPath(gid))
	if err != nil {
		return EnvironmentPatchResult{}, s.abortGroupPatch(ctx, groupPatchTxn{gid: gid, versioned: versioned, claimVersion: claimVersion, generation: generation, oldMeta: m}, false)
	}
	oldFiles, err := s.getGroupMap(ctx, m.workspace, filesPath(gid))
	if err != nil {
		return EnvironmentPatchResult{}, s.abortGroupPatch(ctx, groupPatchTxn{gid: gid, versioned: versioned, claimVersion: claimVersion, generation: generation, oldMeta: m}, false)
	}
	env, files := core.CloneStringMap(oldEnv), core.CloneStringMap(oldFiles)
	if err := core.ApplyEnvVarPatch(env, patch.EnvVars); err != nil {
		if _, releaseErr := s.releaseGroupPatch(ctx, m.workspace, gid, versioned, claimVersion, generation, "idle"); releaseErr != nil {
			return EnvironmentPatchResult{}, envGroupRestorationFailed()
		}
		return EnvironmentPatchResult{}, err
	}
	if err := core.ApplySecretFilePatch(files, patch.SecretFiles); err != nil {
		if _, releaseErr := s.releaseGroupPatch(ctx, m.workspace, gid, versioned, claimVersion, generation, "idle"); releaseErr != nil {
			return EnvironmentPatchResult{}, envGroupRestorationFailed()
		}
		return EnvironmentPatchResult{}, err
	}
	changedEnv := !maps.Equal(oldEnv, env)
	changedFiles := !maps.Equal(oldFiles, files)

	txn := groupPatchTxn{
		gid: gid, versioned: versioned, claimVersion: claimVersion, generation: generation,
		oldEnv: core.CloneStringMap(oldEnv), oldFiles: core.CloneStringMap(oldFiles), oldMeta: m,
		envChanged: changedEnv, filesChanged: changedFiles,
	}
	if changedEnv {
		txn.oldEnvProjection, err = s.snapshotGroupSecret(ctx, m.workspace, envSecretName(gid))
		if err != nil {
			return EnvironmentPatchResult{}, s.abortGroupPatch(ctx, txn, false)
		}
	}
	if changedFiles {
		txn.oldFilesProjection, err = s.snapshotGroupSecret(ctx, m.workspace, filesSecretName(gid))
		if err != nil {
			return EnvironmentPatchResult{}, s.abortGroupPatch(ctx, txn, false)
		}
	}

	if changedEnv {
		if err := s.storeMap(ctx, m.workspace, envPath(gid), env); err != nil {
			return EnvironmentPatchResult{}, s.abortGroupPatch(ctx, txn, true)
		}
		if err := s.upsertSecret(ctx, m.workspace, envSecretName(gid), env); err != nil {
			return EnvironmentPatchResult{}, s.abortGroupPatch(ctx, txn, true)
		}
	}
	if changedFiles {
		if err := s.storeMap(ctx, m.workspace, filesPath(gid), files); err != nil {
			return EnvironmentPatchResult{}, s.abortGroupPatch(ctx, txn, true)
		}
		if err := s.upsertSecret(ctx, m.workspace, filesSecretName(gid), files); err != nil {
			return EnvironmentPatchResult{}, s.abortGroupPatch(ctx, txn, true)
		}
	}
	if changedEnv || changedFiles {
		if _, err := s.touch(ctx, gid, m); err != nil {
			return EnvironmentPatchResult{}, s.abortGroupPatch(ctx, txn, true)
		}
	}
	newGeneration := generation
	if changedEnv || changedFiles {
		newGeneration++
	}
	_, err = s.releaseGroupPatch(ctx, m.workspace, gid, versioned, claimVersion, newGeneration, "idle")
	if err != nil {
		return EnvironmentPatchResult{}, envGroupRestorationFailed()
	}

	result := EnvironmentPatchResult{
		EnvVarKeys:         slices.Sorted(maps.Keys(env)),
		SecretFileNames:    slices.Sorted(maps.Keys(files)),
		Revision:           encodeEnvGroupRevision(newGeneration),
		AffectedServiceIDs: slices.Clone(m.links),
	}
	if patch.SaveMode == SaveModeOnly || len(m.links) == 0 {
		return result, nil
	}
	for _, serviceID := range m.links {
		var actionErr error
		if patch.SaveMode == SaveModeRebuild {
			if s.RebuildService == nil {
				actionErr = core.ErrDeploysUnavailable
			} else {
				actionErr = s.RebuildService(ctx, serviceID)
			}
		} else {
			actionErr = s.rollOne(ctx, serviceID, s.now())
		}
		if actionErr != nil {
			result.FailedServiceIDs = append(result.FailedServiceIDs, serviceID)
		}
	}
	result.RolledOut = len(result.FailedServiceIDs) == 0
	return result, nil
}

func (s *Service) releaseGroupPatch(ctx context.Context, workspace, gid string, versioned core.VersionedSecretKV, claimVersion, generation uint64, state string) (uint64, error) {
	version, err := versioned.PutCAS(groupCtx(context.WithoutCancel(ctx), workspace), revisionPath(gid), map[string]string{
		"state": state, "generation": strconv.FormatUint(generation, 10),
	}, claimVersion)
	return version, err
}

func (s *Service) snapshotGroupSecret(ctx context.Context, workspace, name string) (*coreSecret, error) {
	secret := &corev1.Secret{}
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.envGroupNamespace(workspace), Name: name}, secret)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &coreSecret{object: secret.DeepCopy()}, nil
}

func (s *Service) restoreGroupSecret(ctx context.Context, workspace, name string, backup *coreSecret) error {
	if backup == nil {
		return s.deleteSecret(ctx, workspace, name)
	}
	secret := backup.object.DeepCopy()
	current := &corev1.Secret{}
	err := s.Client.Get(ctx, client.ObjectKeyFromObject(secret), current)
	if apierrors.IsNotFound(err) {
		secret.ResourceVersion = ""
		return s.Client.Create(ctx, secret)
	}
	if err != nil {
		return err
	}
	secret.ResourceVersion = current.ResourceVersion
	return s.Client.Update(ctx, secret)
}

func (s *Service) abortGroupPatch(ctx context.Context, txn groupPatchTxn, restore bool) error {
	var restoreErr error
	if restore {
		var restoreErrors []error
		if txn.envChanged {
			restoreErrors = append(restoreErrors,
				s.storeMap(ctx, txn.oldMeta.workspace, envPath(txn.gid), txn.oldEnv),
				s.restoreGroupSecret(ctx, txn.oldMeta.workspace, envSecretName(txn.gid), txn.oldEnvProjection),
			)
		}
		if txn.filesChanged {
			restoreErrors = append(restoreErrors,
				s.storeMap(ctx, txn.oldMeta.workspace, filesPath(txn.gid), txn.oldFiles),
				s.restoreGroupSecret(ctx, txn.oldMeta.workspace, filesSecretName(txn.gid), txn.oldFilesProjection),
			)
		}
		restoreErrors = append(restoreErrors, s.writeMeta(ctx, txn.gid, txn.oldMeta))
		restoreErr = errors.Join(restoreErrors...)
	}
	state := "idle"
	if restoreErr != nil {
		state = "repair_required"
	}
	_, releaseErr := s.releaseGroupPatch(ctx, txn.oldMeta.workspace, txn.gid, txn.versioned, txn.claimVersion, txn.generation, state)
	if restoreErr != nil || releaseErr != nil {
		return envGroupRestorationFailed()
	}
	return envGroupUpdateRestored()
}
