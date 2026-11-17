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
	"github.com/bex-co/bex/lego/backend/internal/rollout"
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
// Every write that names one makes that move inside its own write: the store
// takes the literal with the write's edit, and the same locked App patch removes
// the literal from spec.Env (dropMovedRetained). The environment patch
// (PatchEnvironment) does it in its compare-and-set revision, so a caller that
// checked the revision it listed lands its first save (w21/103); the
// single-variable verbs and the bulk replace do it in their own store write and
// App patch (w21/108). Nothing is moved before the write is accepted: a write
// the API refuses (a value that is both given and generated, an aggregate-quota
// breach, a name that is not there to delete) leaves the literal where it was and
// no saved change waiting for a deploy, and each verb makes one store write and
// one App patch.

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

// retainedEdit is the App edit a verb's write makes inside its own locked patch:
// removing the literals it moved into the store from spec.Env.
func retainedEdit(moved map[string]string) func(*appv1alpha1.App) error {
	return func(a *appv1alpha1.App) error { return dropMovedRetained(a, moved) }
}

// holdMovedRetained is the context a verb's App patch runs in. Removing a
// literal from spec.Env changes the pod template, which rolls a release. A staged
// write (REST: Render's env-var writes "will not be deployed automatically")
// asks for no rollout, so the removal is held for the next deploy
// (rollout.WithSaveOnly), which takes it with the stored value; a deploying write
// (GraphQL, MCP) rolls once, in the patch that carries the removal.
func holdMovedRetained(ctx context.Context, moved map[string]string) context.Context {
	if len(moved) > 0 && stagedWrite(ctx) {
		return rollout.WithSaveOnly(ctx)
	}
	return ctx
}
