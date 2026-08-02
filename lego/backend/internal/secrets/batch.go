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
	"sort"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
	EnvVars     []EnvVarPatch     `json:"envVars,omitempty"`
	SecretFiles []SecretFilePatch `json:"secretFiles,omitempty"`
	SaveMode    SaveMode          `json:"saveMode"`
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

	oldEnv, err := s.readMap(ctx, envPath(service))
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	oldFiles, err := s.readMap(ctx, filesPath(service))
	if err != nil {
		return EnvironmentPatchResult{}, err
	}
	env := cloneStringMap(oldEnv)
	files := cloneStringMap(oldFiles)
	if err := applyEnvPatch(env, patch.EnvVars); err != nil {
		return EnvironmentPatchResult{}, err
	}
	if err := applyFilePatch(files, patch.SecretFiles); err != nil {
		return EnvironmentPatchResult{}, err
	}

	envChanged := !equalStringMaps(oldEnv, env)
	filesChanged := !equalStringMaps(oldFiles, files)
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

	originalApp := a.DeepCopy()
	base := client.MergeFrom(a.DeepCopy())
	if envChanged {
		if err := s.projectEnv(ctx, a, env); err != nil {
			return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, service, originalApp, oldEnv, oldFiles, envChanged, filesChanged, err)
		}
	}
	if filesChanged {
		if err := s.projectFiles(ctx, a, files); err != nil {
			return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, service, originalApp, oldEnv, oldFiles, envChanged, filesChanged, err)
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
		return environmentPatchResult(env, files, false), nil
	}
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return EnvironmentPatchResult{}, s.compensateEnvironment(ctx, service, originalApp, oldEnv, oldFiles, envChanged, filesChanged, err)
	}
	return environmentPatchResult(env, files, rolledOut), nil
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
		if !containsString(original.Spec.FilesFromSecrets, name) && len(files) > 0 {
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

func (s *Service) compensateEnvironment(ctx context.Context, service string, originalApp *appv1alpha1.App, oldEnv, oldFiles map[string]string, envChanged, filesChanged bool, cause error) error {
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
		if containsString(originalApp.Spec.FilesFromSecrets, name) {
			if err := s.upsertSecret(ctx, originalApp, name, oldFiles); err != nil {
				compensation = append(compensation, fmt.Errorf("restore secret-file projection: %w", err))
			}
		} else if err := s.deleteSecret(ctx, originalApp.Namespace, name); err != nil {
			compensation = append(compensation, fmt.Errorf("remove secret-file projection: %w", err))
		}
	}
	return errors.Join(append([]error{cause}, compensation...)...)
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

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func equalStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		other, ok := b[key]
		if !ok || other != value {
			return false
		}
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
