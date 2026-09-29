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
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RestoreEnvironment makes a service's SAVED env vars and secret files equal env
// and files — a rollback target's per-release snapshot (w1/m152 t009).
//
// Render: on a rollback "environment variables … match the target deploy"
// (render.com/docs/rollbacks), and the milestone decided the restore lands in the
// saved state, so the Environment tab and GET …/env-vars read the target's values
// afterwards rather than a newer save the running release no longer has.
//
// It is one save_only PatchEnvironment, so it opens no deploy and rolls nothing of
// its own: the rollback's single dispatch rolls once, with these values. Every
// validation, quota, audit and OpenBao-then-projection ordering PatchEnvironment
// already enforces therefore applies unchanged. Manifest-owned keys are left alone —
// a render.yaml owns their value, not a rollback — and a snapshot cannot normally
// contain one, since manifest literals render inline rather than into the store map.
// Returns whether anything changed.
func (s *Service) RestoreEnvironment(ctx context.Context, service string, env, files map[string]string) (bool, error) {
	a, ctx, service, err := s.scopeForWrite(ctx, core.RelCanCreate, service)
	if err != nil {
		return false, err
	}
	currentEnv, err := s.readMap(ctx, envPath(service))
	if err != nil {
		return false, err
	}
	currentFiles, err := s.readMap(ctx, filesPath(service))
	if err != nil {
		return false, err
	}
	owned := manifestEnv(a)

	var envPatch []EnvVarPatch
	for _, key := range sortedUnion(currentEnv, env) {
		if _, manifest := owned[key]; manifest {
			continue
		}
		want, keep := env[key]
		have, exists := currentEnv[key]
		switch {
		case !keep:
			envPatch = append(envPatch, EnvVarPatch{Key: key, Delete: true})
		case !exists || have != want:
			envPatch = append(envPatch, EnvVarPatch{Key: key, Value: want, ValueSet: true})
		}
	}
	var filesPatch []SecretFilePatch
	for _, name := range sortedUnion(currentFiles, files) {
		want, keep := files[name]
		have, exists := currentFiles[name]
		switch {
		case !keep:
			filesPatch = append(filesPatch, SecretFilePatch{Name: name, Delete: true})
		case !exists || have != want:
			filesPatch = append(filesPatch, SecretFilePatch{Name: name, Content: want})
		}
	}
	if len(envPatch) == 0 && len(filesPatch) == 0 {
		return false, nil
	}
	if _, err := s.PatchEnvironment(ctx, service, EnvironmentPatch{
		EnvVars:     envPatch,
		SecretFiles: filesPatch,
		SaveMode:    SaveModeOnly,
	}); err != nil {
		return false, err
	}
	return true, nil
}

func sortedUnion(a, b map[string]string) []string {
	out := make([]string, 0, len(a)+len(b))
	for k := range a {
		out = append(out, k)
	}
	for k := range b {
		if _, dup := a[k]; !dup {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}
