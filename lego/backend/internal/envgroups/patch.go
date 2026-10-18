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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
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
	// PendingServiceIDs are linked services the write deliberately did NOT
	// deploy because their Auto-Deploy is off (Render's documented gate). Their
	// copy of the group is current — the group's Secret is shared — but they
	// keep serving their existing release until their owner deploys them. This
	// is a subset of AffectedServiceIDs, and NOT a failure: unlike
	// FailedServiceIDs it must not be retried, so RolledOut stays true.
	PendingServiceIDs []string `json:"pendingServiceIds,omitempty"`
	RolledOut         bool     `json:"rolledOut"`
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

const codeRestorationFailed = "ENV_GROUP_RESTORATION_FAILED"

func envGroupRestorationFailed() error {
	return core.NewConflictError(
		codeRestorationFailed,
		"the environment group update failed and could not be safely restored; an operator must repair it",
		nil,
	)
}

// groupPatchTxn is one claimed save: the group as claimed, its operation and
// claim version, and what compensation needs to restore.
type groupPatchTxn struct {
	g            store.EnvGroup
	op           store.EnvGroupOp
	claimVersion int64
	prior        groupContents
	projections  [2]*coreSecret // indexed by contentSlot
}

// coreSecret is the minimal projection snapshot compensation needs. Keeping a
// copy of the Kubernetes object also preserves metadata set outside this feature.
type coreSecret struct{ object *corev1.Secret }

// PatchEnvironment applies one group draft. The group's row serializes the two
// OpenBao maps and their two Kubernetes projections across API replicas: a save
// claims the row's content version busy, writes, and releases it with the next
// generation, so a file-only update takes part in the same revision protocol.
func (s *Service) PatchEnvironment(ctx context.Context, gid string, patch EnvironmentPatch) (EnvironmentPatchResult, error) {
	g, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	return s.patchEnvironmentAuthorized(ctx, g, patch)
}

func (s *Service) patchEnvironmentAuthorized(ctx context.Context, g store.EnvGroup, patch EnvironmentPatch) (EnvironmentPatchResult, error) {
	if patch.SaveMode != SaveModeOnly && patch.SaveMode != SaveModeDeploy && patch.SaveMode != SaveModeRebuild {
		return EnvironmentPatchResult{}, core.NewBadRequestError(
			"ENV_GROUP_SAVE_MODE_INVALID",
			"saveMode must be save_only, deploy, or rebuild",
			nil,
		)
	}
	g, err := s.operableGroup(ctx, g)
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	gid, workspace, generation := g.ID, g.TenantID, g.Content.Generation
	if patch.ExpectedRevision != nil {
		expected, decodeErr := decodeEnvGroupRevision(*patch.ExpectedRevision)
		if decodeErr != nil {
			return EnvironmentPatchResult{}, envGroupRevisionInvalid()
		}
		if expected != generation {
			return EnvironmentPatchResult{}, envGroupRevisionConflict()
		}
	}
	// Preflight the complete merged maps before claiming the revision, so quota
	// failures never leave a busy group behind and are independent of which
	// patch form was used.
	var prior groupContents
	if prior.env, err = s.getGroupMap(ctx, workspace, envPath(gid)); err != nil {
		return EnvironmentPatchResult{}, err
	}
	if prior.files, err = s.getGroupMap(ctx, workspace, filesPath(gid)); err != nil {
		return EnvironmentPatchResult{}, err
	}
	next := groupContents{env: core.CloneStringMap(prior.env), files: core.CloneStringMap(prior.files)}
	if err := core.ApplyEnvVarPatch(next.env, patch.EnvVars); err != nil {
		return EnvironmentPatchResult{}, err
	}
	if err := core.ApplySecretFilePatch(next.files, patch.SecretFiles); err != nil {
		return EnvironmentPatchResult{}, err
	}
	// Same rule the service batch patch got in w1/m147: a group already over the
	// cap can shrink but not grow. Checking the patched map alone refused the
	// delete-only patch that is the only way back under the cap.
	if err := secrets.ValidatePatchEnvMapQuota(prior.env, next.env); err != nil {
		return EnvironmentPatchResult{}, err
	}
	if err := secrets.ValidatePatchFilesMapQuota(prior.files, next.files); err != nil {
		return EnvironmentPatchResult{}, err
	}
	op := store.EnvGroupOp{Kind: opKindPatch, EnvChanged: !maps.Equal(prior.env, next.env), FilesChanged: !maps.Equal(prior.files, next.files)}
	if op.EnvChanged || op.FilesChanged {
		// A save that changes nothing (the empty patch that retries a rollout
		// included) has nothing to serialize and goes straight to the rollout.
		if generation, err = s.saveContents(ctx, g, op, prior, next); err != nil {
			return EnvironmentPatchResult{}, err
		}
	}

	// The links to roll are the ones the group holds now.
	if current, err := s.readGroup(ctx, gid); err == nil {
		g = current
	}
	result := EnvironmentPatchResult{
		EnvVarKeys:         slices.Sorted(maps.Keys(next.env)),
		SecretFileNames:    slices.Sorted(maps.Keys(next.files)),
		Revision:           encodeEnvGroupRevision(generation),
		AffectedServiceIDs: slices.Clone(g.ServiceLinks),
	}
	if patch.SaveMode == SaveModeOnly || len(g.ServiceLinks) == 0 {
		return result, nil
	}
	// Roll (or rebuild) every currently-linked service. A service that no longer
	// exists (core.ErrNotFound from rollLink's GetApp or RebuildService's own
	// AuthorizeApp) is NOT a rollout failure: it can never be retried into
	// existence, so counting it against FailedServiceIDs would pin a permanent,
	// un-retryable failure on the group and make RolledOut false forever —
	// breaking ADR006's "rollout-only failures are retryable" guarantee. It is
	// dropped from both the reported affected set and the group's persisted links
	// (self-heal below), leaving only real, still-existing services — a genuine
	// failure on one of those stays in FailedServiceIDs and retryable, unchanged.
	affected := make([]string, 0, len(g.ServiceLinks))
	var stale []string
	for _, serviceID := range g.ServiceLinks {
		skipped, actionErr := s.rollLink(ctx, gid, serviceID, patch.SaveMode)
		if errors.Is(actionErr, core.ErrServiceReplaced) || apierrors.IsNotFound(actionErr) {
			// Deleted, or deleted and recreated, since its read: judge the
			// link again against the App now at serviceID (w5/158).
			skipped, actionErr = s.rollLink(ctx, gid, serviceID, patch.SaveMode)
		}
		if errors.Is(actionErr, core.ErrNotFound) {
			stale = append(stale, serviceID)
			continue
		}
		affected = append(affected, serviceID)
		if skipped {
			result.PendingServiceIDs = append(result.PendingServiceIDs, serviceID)
		}
		if actionErr != nil {
			result.FailedServiceIDs = append(result.FailedServiceIDs, serviceID)
		}
	}
	result.AffectedServiceIDs = affected
	result.RolledOut = len(result.FailedServiceIDs) == 0
	s.pruneStaleLinks(ctx, gid, stale)
	return result, nil
}

// saveContents is the claimed part of a save that changes content: record the
// operation, claim the group, write each changed map and its projection, and
// release at the next generation. It returns that generation.
func (s *Service) saveContents(ctx context.Context, g store.EnvGroup, op store.EnvGroupOp, prior, next groupContents) (uint64, error) {
	gid, workspace := g.ID, g.TenantID
	opID, err := s.preparePatchOperation(ctx, op, workspace, gid, prior, next)
	if err != nil {
		return 0, core.ErrSecretsUnavailable
	}
	op.ID = opID
	claimVersion, err := s.claimContent(ctx, g, op)
	if err != nil {
		_ = s.clearOpMaps(context.WithoutCancel(ctx), workspace, gid, opID)
		return 0, err
	}
	txn := groupPatchTxn{g: g, op: op, claimVersion: claimVersion, prior: prior}
	sides := []struct {
		changed bool
		slot    contentSlot
		data    map[string]string
		phase   string
	}{
		{op.EnvChanged, envSlot, next.env, opPhaseEnvWritten},
		{op.FilesChanged, filesSlot, next.files, opPhaseFilesWritten},
	}
	// Snapshot every projection this save replaces before writing any, so a
	// failure at any later step can restore all of them.
	for _, side := range sides {
		if !side.changed {
			continue
		}
		if txn.projections[side.slot], err = s.snapshotGroupSecret(ctx, workspace, side.slot.secret(gid)); err != nil {
			return 0, s.abortGroupPatch(ctx, txn, false)
		}
	}
	for _, side := range sides {
		if !side.changed {
			continue
		}
		if err := s.writeSlot(ctx, workspace, gid, side.slot, side.data); err != nil {
			return 0, s.abortGroupPatch(ctx, txn, true)
		}
		if err := s.advanceOpPhase(ctx, gid, opID, side.phase); err != nil {
			return 0, s.abortGroupPatch(ctx, txn, true)
		}
	}
	if !op.FilesChanged {
		// Past env, files_written is the phase recovery finalizes from.
		if err := s.advanceOpPhase(ctx, gid, opID, opPhaseFilesWritten); err != nil {
			return 0, s.abortGroupPatch(ctx, txn, true)
		}
	}
	if err := s.releaseContent(ctx, gid, claimVersion, s.commit(next)); err != nil {
		return 0, envGroupRestorationFailed()
	}
	_ = s.clearOpMaps(context.WithoutCancel(ctx), workspace, gid, opID)
	return g.Content.Generation + 1, nil
}

// rollLink rolls (or rebuilds) the service one link names. A service that is
// gone, or no longer mounts the group, answers core.ErrNotFound.
func (s *Service) rollLink(ctx context.Context, gid, serviceID string, saveMode SaveMode) (skipped bool, err error) {
	a, err := s.GetApp(ctx, core.RelCanCreate, serviceID)
	if err != nil {
		return false, err
	}
	if !mountsGroup(a, gid) {
		return false, core.ErrNotFound
	}
	if saveMode == SaveModeRebuild {
		// An explicit rebuild is the caller's deliberate choice, not the
		// implicit fan-out Render gates on Auto-Deploy, so it is ungated.
		if s.RebuildService == nil {
			return false, core.ErrDeploysUnavailable
		}
		return false, s.RebuildService(ctx, serviceID)
	}
	return s.rollOne(ctx, a, s.now())
}

// pruneStaleLinks removes stale links from the group's persisted link set:
// since-deleted services, and entries answering for a service that does not
// mount the group (both surfaced as core.ErrNotFound while rolling). It runs
// after the content commit has already succeeded and the revision was
// released, so it is best-effort self-heal: a failure to persist the pruned
// set must never turn a successful rollout into an error response — the write
// error is swallowed and the next patch re-discovers and re-prunes. Removals
// apply to the *current* link set so a concurrently added valid link is
// preserved (w4/m97).
func (s *Service) pruneStaleLinks(ctx context.Context, gid string, stale []string) {
	if len(stale) == 0 {
		return
	}
	_ = s.dropLinks(context.WithoutCancel(ctx), gid, stale...)
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

// abortGroupPatch undoes a save that failed after its claim. Only the content
// maps and projections are this save's to restore: a concurrent rename, link
// or move committed on the row meanwhile survives (w4/m97). A failed restore
// leaves the group repair_required with its operation, which recovery retries.
func (s *Service) abortGroupPatch(ctx context.Context, txn groupPatchTxn, restore bool) error {
	ctx = context.WithoutCancel(ctx)
	workspace, gid := txn.g.TenantID, txn.g.ID
	var restoreErr error
	if restore {
		var restoreErrors []error
		for slot, changed := range [2]bool{txn.op.EnvChanged, txn.op.FilesChanged} {
			if !changed {
				continue
			}
			slot := contentSlot(slot)
			prior := txn.prior.env
			if slot == filesSlot {
				prior = txn.prior.files
			}
			restoreErrors = append(restoreErrors,
				s.storeMap(ctx, workspace, slot.path(gid), prior),
				s.restoreGroupSecret(ctx, workspace, slot.secret(gid), txn.projections[slot]),
			)
		}
		restoreErr = errors.Join(restoreErrors...)
	}
	releaseErr := s.releaseContent(ctx, gid, txn.claimVersion, store.EnvGroupRelease{RepairRequired: restoreErr != nil})
	if restoreErr != nil || releaseErr != nil {
		return envGroupRestorationFailed()
	}
	_ = s.clearOpMaps(ctx, workspace, gid, txn.op.ID)
	return envGroupUpdateRestored()
}
