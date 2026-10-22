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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's env-var and secret-file REST verbs save the change and deploy
// nothing: "Changes will not be deployed automatically. Instead you must call
// the deploy API" (api-docs.render.com update-env-vars-for-service, w8/186).
// The REST adapters mark their context with WithStagedWrite, and the single-item
// write paths then end in the same Save-only staging PatchEnvironment's
// save_only mode uses instead of bumping spec.restartedAt. GraphQL and MCP keep
// the immediate rollout their callers expect.

type stagedWriteKey struct{}

// WithStagedWrite marks ctx so env-var and secret-file writes save without a
// rollout.
func WithStagedWrite(ctx context.Context) context.Context {
	return context.WithValue(ctx, stagedWriteKey{}, true)
}

func stagedWrite(ctx context.Context) bool {
	staged, _ := ctx.Value(stagedWriteKey{}).(bool)
	return staged
}

// stageEnvWrite is materializeEnv's Save-only counterpart: the projected
// Secret is updated, the App's references stay as they are (a release that
// never consumed the Secret gets it on its next deploy), and a fresh
// notification wakes the status comparison with the serving release. edit
// adjusts the App inside the locked patch.
func (s *Service) stageEnvWrite(ctx context.Context, a *appv1alpha1.App, env map[string]string, revision sourceRevision, edit func(*appv1alpha1.App)) error {
	// Projected once, before the locked patch, which can run again on a
	// re-read App (see finalizeEnvironmentPatch).
	if err := s.projectEnv(ctx, a.DeepCopy(), env, revision); err != nil {
		return err
	}
	return s.rollApp(ctx, a, func(a *appv1alpha1.App) error {
		read := a.DeepCopy()
		a.Spec.EnvFromSecret = envSecretName(a.Name)
		if edit != nil {
			edit(a)
		}
		stagePendingProjectionReferences(a, read, env, true, false)
		notifySavedConfig(a)
		return nil
	})
}

// stageFilesWrite is materializeFiles' Save-only counterpart.
func (s *Service) stageFilesWrite(ctx context.Context, a *appv1alpha1.App, files map[string]string, revision sourceRevision) error {
	return s.rollApp(ctx, a, func(a *appv1alpha1.App) error {
		read := a.DeepCopy()
		if err := s.projectFiles(ctx, a, files, revision); err != nil {
			return err
		}
		stagePendingProjectionReferences(a, read, nil, false, true)
		notifySavedConfig(a)
		return nil
	})
}

// notifySavedConfig stamps the value-free, never-repeating notification the
// operator's status reconciliation compares saved bytes on. A timestamp would
// collide when two saves share a clock tick.
func notifySavedConfig(a *appv1alpha1.App) {
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	a.Annotations[appv1alpha1.AnnotationSavedConfigRevision] = rand.Text()
}
