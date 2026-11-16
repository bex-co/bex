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
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Blueprint sync keeps a variable its manifest used to declare and no longer
// does, on the service with its value: Render "preserves existing environment
// variables, even if you omit them from the Blueprint file"
// (https://render.com/docs/blueprint-spec). The literal stays on spec.Env, but
// it is no longer the manifest's: reading it as manifest-owned refused every
// delete and edit of it with "declared in this service's render.yaml
// manifest", forever (w21/m421, production 2026-11-08 and 2026-11-16). The
// Blueprint's sync records what it declares (core.AnnotationBlueprintEnv); a
// literal outside that record is retained, listed as an ordinary variable, and
// adopted into the mutable store when a write names it.

// declaredBlueprintEnv is the set of names the App's Blueprint declared at its
// last sync, and whether a sync has recorded one at all.
func declaredBlueprintEnv(a *appv1alpha1.App) (map[string]bool, bool) {
	raw, ok := a.GetAnnotations()[core.AnnotationBlueprintEnv]
	if !ok {
		return nil, false
	}
	declared := map[string]bool{}
	for name := range strings.SplitSeq(raw, ",") {
		if name != "" {
			declared[name] = true
		}
	}
	return declared, true
}

// retainedEnv returns the spec.Env literals the App's Blueprint no longer
// declares, by name: the ordinary variables a sync left in place. An App no
// sync has recorded a declaration for has none (every literal is declared).
// Names no env verb accepts are the older debris manifestEnv already lists as
// deletable, not retained variables.
func retainedEnv(a *appv1alpha1.App) map[string]string {
	declared, tracked := declaredBlueprintEnv(a)
	if !tracked {
		return nil
	}
	var retained map[string]string
	for _, item := range a.Spec.Env {
		if item.ValueFrom != nil || item.Name == "" || declared[item.Name] || !core.ValidEnvKey(item.Name) {
			continue
		}
		if retained == nil {
			retained = map[string]string{}
		}
		retained[item.Name] = item.Value
	}
	return retained
}

// dropSpecEnvNames removes the named literals from spec.Env, inside a locked
// patch that has just stored them.
func dropSpecEnvNames(a *appv1alpha1.App, names map[string]string) {
	a.Spec.Env = slices.DeleteFunc(a.Spec.Env, func(item appv1alpha1.EnvVar) bool {
		_, drop := names[item.Name]
		return drop && item.ValueFrom == nil
	})
	if len(a.Spec.Env) == 0 {
		a.Spec.Env = nil
	}
}

// adoptRetainedEnv moves the retained literals a write names into the mutable
// store, so the write then meets an ordinary stored variable: the literal wins
// over the store at runtime (the container's env beats envFrom), so a stored
// edit or delete beside a surviving literal would change nothing. only names
// the keys to adopt; none adopts every retained literal (a whole-set replace).
// The value the process holds, the literal's, is what the store keeps. It
// refreshes a, and does nothing for a service with no retained literal.
func (s *Service) adoptRetainedEnv(ctx context.Context, a *appv1alpha1.App, service string, only ...string) error {
	retained := retainedEnv(a)
	if len(only) > 0 {
		for name := range retained {
			if !slices.Contains(only, name) {
				delete(retained, name)
			}
		}
	}
	if len(retained) == 0 {
		return nil
	}
	var quota error
	env, envWrite, err := s.updateMapCAS(ctx, envPath(service), func(current map[string]string) bool {
		changed := false
		for name, value := range retained {
			if v, ok := current[name]; !ok || v != value {
				current[name], changed = value, true
			}
		}
		if err := envMapWithinQuota(current); err != nil {
			quota = err
			return false
		}
		return changed
	})
	if err != nil {
		return err
	}
	if quota != nil {
		return quota
	}
	original := a.DeepCopy()
	if err := s.materializeEnvEditing(ctx, a, env, committedAt(envWrite.version), func(a *appv1alpha1.App) {
		dropSpecEnvNames(a, retained)
	}); err != nil {
		return s.compensateEnvironment(ctx, envPatchTxn{service: service, originalApp: original, env: envWrite}, err)
	}
	return nil
}
