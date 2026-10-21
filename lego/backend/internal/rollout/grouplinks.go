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

package rollout

import (
	"encoding/json"
	"slices"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Env-group links on an Auto-Deploy-off service (w4/m239).
//
// spec.envFromSecrets and spec.filesFromSecrets are release identity, so
// writing a group's Secret refs into them makes the operator adopt a new
// release — one no deploy row records, and one that strands the owner's next
// deploy. Render deploys an environment-group change only for services with
// Auto-Deploy on, so for the others the change waits: it is staged in the
// AnnotationPendingGroupLinks annotation (metadata, outside the identity) and
// folded into the spec by the next rolling write, which is a recorded deploy.

// refDelta is the pending change to one Secret-ref list.
type refDelta struct {
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

// groupLinks is the annotation's value.
type groupLinks struct {
	Env   refDelta `json:"env"`
	Files refDelta `json:"files"`
}

func (g groupLinks) empty() bool {
	return len(g.Env.Add)+len(g.Env.Remove)+len(g.Files.Add)+len(g.Files.Remove) == 0
}

func readGroupLinks(a *appv1alpha1.App) groupLinks {
	var g groupLinks
	if raw := a.Annotations[appv1alpha1.AnnotationPendingGroupLinks]; raw != "" {
		_ = json.Unmarshal([]byte(raw), &g) // a malformed value stages nothing
	}
	return g
}

func (d refDelta) apply(list []string) []string {
	out := slices.DeleteFunc(slices.Clone(list), func(v string) bool { return slices.Contains(d.Remove, v) })
	for _, name := range d.Add {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

// settle drops entries the spec already satisfies, so a link then unlink of an
// unmounted group (or the reverse) leaves nothing staged.
func (d refDelta) settle(list []string) refDelta {
	return refDelta{
		Add:    slices.DeleteFunc(slices.Clone(d.Add), func(v string) bool { return slices.Contains(list, v) }),
		Remove: slices.DeleteFunc(slices.Clone(d.Remove), func(v string) bool { return !slices.Contains(list, v) }),
	}
}

func (d refDelta) stage(name string, link bool) refDelta {
	d.Add = slices.DeleteFunc(slices.Clone(d.Add), func(v string) bool { return v == name })
	d.Remove = slices.DeleteFunc(slices.Clone(d.Remove), func(v string) bool { return v == name })
	if link {
		d.Add = append(d.Add, name)
	} else {
		d.Remove = append(d.Remove, name)
	}
	return d
}

func writeGroupLinks(a *appv1alpha1.App, g groupLinks) {
	g.Env, g.Files = g.Env.settle(a.Spec.EnvFromSecrets), g.Files.settle(a.Spec.FilesFromSecrets)
	if g.empty() {
		delete(a.Annotations, appv1alpha1.AnnotationPendingGroupLinks)
		return
	}
	raw, _ := json.Marshal(g)
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	a.Annotations[appv1alpha1.AnnotationPendingGroupLinks] = string(raw)
}

// StageGroupLink records that a's group Secrets (envSecret, filesSecret) are to
// be linked, or unlinked, by the next release instead of by this write. It
// touches only metadata.
func StageGroupLink(a *appv1alpha1.App, envSecret, filesSecret string, link bool) {
	g := readGroupLinks(a)
	g.Env, g.Files = g.Env.stage(envSecret, link), g.Files.stage(filesSecret, link)
	writeGroupLinks(a, g)
}

// EffectiveGroupSecrets returns the env and files Secret refs a will carry once
// its staged links ship: what the owner asked for, not just what is running.
func EffectiveGroupSecrets(a *appv1alpha1.App) (env, files []string) {
	g := readGroupLinks(a)
	return g.Env.apply(a.Spec.EnvFromSecrets), g.Files.apply(a.Spec.FilesFromSecrets)
}

// FoldStagedGroupLinks moves a's staged links into its spec and clears them. A
// writer that is already minting a release calls it, so the release carries the
// links; it must not be called for a write that does not roll one.
func FoldStagedGroupLinks(a *appv1alpha1.App) {
	if a == nil || a.Annotations[appv1alpha1.AnnotationPendingGroupLinks] == "" {
		return
	}
	a.Spec.EnvFromSecrets, a.Spec.FilesFromSecrets = EffectiveGroupSecrets(a)
	delete(a.Annotations, appv1alpha1.AnnotationPendingGroupLinks)
}
