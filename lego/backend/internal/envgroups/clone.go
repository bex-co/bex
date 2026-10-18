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
	var env, files map[string]string
	err = s.withGroupContent(ctx, sourceID, func(ctx context.Context, source store.EnvGroup) error {
		opID, err := newGroupOpID()
		if err != nil {
			return core.ErrSecretsUnavailable
		}
		claimVersion, err := s.claimContent(ctx, source, store.EnvGroupOp{ID: opID, Kind: opKindClone})
		if err != nil {
			return err
		}
		var readErr error
		env, readErr = s.getGroupMap(ctx, source.TenantID, envPath(sourceID))
		if readErr == nil {
			files, readErr = s.getGroupMap(ctx, source.TenantID, filesPath(sourceID))
		}
		cleanupCtx, cancel := groupCleanupContext(ctx)
		defer cancel()
		if err := s.releaseContent(cleanupCtx, sourceID, claimVersion, store.EnvGroupRelease{}); err != nil {
			return envGroupRestorationFailed()
		}
		if readErr != nil {
			return core.ErrSecretsUnavailable
		}
		return nil
	})
	if err != nil {
		return EnvGroupView{}, err
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
	return s.createEnvGroupAuthorized(targetCtx, create)
}
