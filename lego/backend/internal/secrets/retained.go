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
	"maps"
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
// moved into the mutable store when a write names it.
//
// The environment patch (PatchEnvironment) makes that move inside its own
// compare-and-set write: the store takes the literal with the patch's edit in
// one revision, checked against the revision the caller listed, and the same
// locked App patch removes the literal from spec.Env. A move in a write of its
// own would advance the revision under a caller that checked one, and refuse
// its first save as stale (w21/103). The single-variable verbs and the bulk
// replace check no revision, so they adopt first (adoptRetainedKeys).

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

// adoptRetainedKeys moves the named retained literals into the mutable store
// before a write decides them, so the write meets ordinary stored variables.
func (s *Service) adoptRetainedKeys(ctx context.Context, a *appv1alpha1.App, service string, keys ...string) error {
	return s.adoptRetained(ctx, a, service, retainedEnvNamed(a, keys...))
}

// retainedEnvNamed is retainedEnv narrowed to the named variables.
func retainedEnvNamed(a *appv1alpha1.App, names ...string) map[string]string {
	retained := retainedEnv(a)
	for name := range retained {
		if !slices.Contains(names, name) {
			delete(retained, name)
		}
	}
	return retained
}

// retainedForPatch is the retained literals an environment patch moves into the
// store with its own write: the ones it names (a rename's source included), or
// every one when it replaces the whole set.
func retainedForPatch(a *appv1alpha1.App, patch EnvironmentPatch) map[string]string {
	if patch.Replace {
		return retainedEnv(a)
	}
	keys := patchEnvKeys(patch)
	for i, key := range keys {
		keys[i] = strings.TrimSpace(key)
	}
	return retainedEnvNamed(a, keys...)
}

// dropMovedRetained removes the literals a write moved into the store from
// spec.Env, inside that write's locked App patch. The patch can run again on a
// re-read App, so it first refuses a name a Blueprint sync has re-declared since
// the write read the App: the manifest owns it again, its literal wins at
// runtime, and a stored edit beside it would change nothing the process sees.
// The refusal rolls the store write back with the rest of the save.
func dropMovedRetained(a *appv1alpha1.App, moved map[string]string) error {
	if len(moved) == 0 {
		return nil
	}
	if err := refuseManifestKeys(a, slices.Collect(maps.Keys(moved))...); err != nil {
		return err
	}
	dropSpecEnvNames(a, moved)
	return nil
}

// adoptAllRetained is adoptRetainedKeys for a whole-set replace, which decides
// every variable.
func (s *Service) adoptAllRetained(ctx context.Context, a *appv1alpha1.App, service string) error {
	return s.adoptRetained(ctx, a, service, retainedEnv(a))
}

// adoptRetained writes retained into the store and removes the literals from
// spec.Env in the same locked patch. The literal wins over the store at
// runtime (the container's env beats envFrom), so a stored edit or delete beside
// a surviving literal would change nothing; the value the process holds, the
// literal's, is what the store keeps. It refreshes a.
func (s *Service) adoptRetained(ctx context.Context, a *appv1alpha1.App, service string, retained map[string]string) error {
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
