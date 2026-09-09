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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type CloneEnvGroupRequest struct {
	Name          string `json:"name"`
	OwnerID       string `json:"ownerId,omitempty"`
	EnvironmentID string `json:"environmentId,omitempty"`
}

// CloneEnvGroup copies a consistent source snapshot entirely server-side. It
// freshly authorizes sensitive source access, separately authorizes target
// creation, takes the source revision lock across the copy, and intentionally
// supplies no service ids to the target create transaction.
func (s *Service) CloneEnvGroup(ctx context.Context, sourceID string, req CloneEnvGroupRequest) (EnvGroupView, error) {
	source, err := s.authorizeGroup(ctx, core.RelCanViewSensitive, sourceID)
	if err != nil {
		return EnvGroupView{}, err
	}
	if source.workspace != "" {
		if err := s.AuthorizeFreshOn(ctx, core.RelCanViewSensitive, core.WorkspaceObject(source.workspace)); err != nil {
			return EnvGroupView{}, err
		}
	} else if err := s.AuthorizeFresh(ctx, core.RelCanViewSensitive); err != nil {
		return EnvGroupView{}, err
	}
	targetCtx := core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(targetCtx, core.RelCanCreate); err != nil {
		return EnvGroupView{}, err
	}
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	if err := s.ensureGroupOperable(ctx, source.workspace, sourceID); err != nil {
		return EnvGroupView{}, err
	}
	revision, err := s.getRevisionSnapshot(ctx, versioned, source.workspace, sourceID)
	if err != nil || revision.Data["state"] == "repair_required" {
		return EnvGroupView{}, envGroupRestorationFailed()
	}
	if revision.Data["state"] == "busy" {
		return EnvGroupView{}, envGroupRevisionConflict()
	}
	generation := revisionGeneration(revision.Data)
	opID, err := s.prepareCloneOperation(ctx, source.workspace, sourceID, generation)
	if err != nil {
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	claimVersion, err := versioned.PutCAS(groupCtx(ctx, source.workspace), revisionPath(sourceID), revisionClaimData("busy", generation, opID), revision.Version)
	if err != nil {
		_ = s.clearOpArtifacts(ctx, source.workspace, sourceID, opID)
		if errors.Is(err, core.ErrConflict) {
			return EnvGroupView{}, envGroupRevisionConflict()
		}
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	if err := s.commitOpRecord(ctx, source.workspace, sourceID, groupOpRecord{
		id: opID, kind: opKindClone, phase: opPhaseAdmitted, generation: generation,
		leaseUntil: s.Now().UTC().Add(s.opLease()),
	}); err != nil {
		_, _ = s.releaseGroupPatch(ctx, source.workspace, sourceID, versioned, claimVersion, generation, "idle")
		_ = s.clearOpArtifacts(ctx, source.workspace, sourceID, opID)
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	release := func() error {
		_, releaseErr := s.releaseGroupPatch(ctx, source.workspace, sourceID, versioned, claimVersion, generation, "idle")
		_ = s.clearOpArtifacts(ctx, source.workspace, sourceID, opID)
		return releaseErr
	}
	env, err := s.getGroupMap(ctx, source.workspace, envPath(sourceID))
	if err != nil {
		_ = release()
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	files, err := s.getGroupMap(ctx, source.workspace, filesPath(sourceID))
	if err != nil {
		_ = release()
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	create := CreateEnvGroupRequest{
		Name: req.Name, OwnerID: req.OwnerID, EnvironmentID: req.EnvironmentID,
		EnvVars:     make([]CreateEnvVarInput, 0, len(env)),
		SecretFiles: make([]SecretFileView, 0, len(files)),
	}
	for _, key := range core.SortedKeys(env) {
		create.EnvVars = append(create.EnvVars, CreateEnvVarInput{Key: key, Value: env[key], ValueSet: true})
	}
	for _, name := range core.SortedKeys(files) {
		create.SecretFiles = append(create.SecretFiles, SecretFileView{Name: name, Content: files[name]})
	}
	cloned, err := s.createEnvGroupAuthorized(targetCtx, create)
	if err != nil {
		_ = release()
		return EnvGroupView{}, err
	}
	if err := release(); err != nil {
		cleanupErr := s.DeleteEnvGroup(targetCtx, cloned.ID)
		return EnvGroupView{}, errors.Join(envGroupRestorationFailed(), cleanupErr)
	}
	return cloned, nil
}
