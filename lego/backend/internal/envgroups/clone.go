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
	"github.com/bex-co/bex/lego/backend/internal/store"
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
	if err := s.authorizeGroupSensitiveFresh(ctx, source); err != nil {
		return EnvGroupView{}, err
	}
	targetCtx := core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(targetCtx, core.RelCanCreate); err != nil {
		return EnvGroupView{}, err
	}
	if source, err = s.operableGroup(ctx, source); err != nil {
		return EnvGroupView{}, err
	}
	opID, err := newGroupOpID()
	if err != nil {
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	// The source lock: a clone claims the source busy so no save changes its
	// maps mid-copy. It never mutates the source, so recovery only releases it.
	claimVersion, err := s.claimContent(ctx, source, store.EnvGroupOp{ID: opID, Kind: opKindClone})
	if err != nil {
		return EnvGroupView{}, err
	}
	release := func() error {
		return s.releaseContent(ctx, sourceID, claimVersion, store.EnvGroupRelease{})
	}
	env, err := s.getGroupMap(ctx, source.TenantID, envPath(sourceID))
	if err != nil {
		_ = release()
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	files, err := s.getGroupMap(ctx, source.TenantID, filesPath(sourceID))
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
