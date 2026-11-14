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
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/id"
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
// With nothing staged that is the running spec, in its own order. A staged
// change ships in a release, which puts the group refs in creation order
// (PrepareGroupRefsForRelease), so the result already carries that order.
func EffectiveGroupSecrets(a *appv1alpha1.App) (env, files []string) {
	g := readGroupLinks(a)
	if g.empty() {
		return slices.Clone(a.Spec.EnvFromSecrets), slices.Clone(a.Spec.FilesFromSecrets)
	}
	return SortGroupRefs(g.Env.apply(a.Spec.EnvFromSecrets)), SortGroupRefs(g.Files.apply(a.Spec.FilesFromSecrets))
}

// PrepareGroupRefsForRelease moves a's staged links into its spec, clears
// them, and puts its group refs in creation order (w4/m355). A writer that is
// already minting a release calls it, so the release carries the links. It
// must not be called for a write that does not roll one, nor by a rollback,
// which restores the old release as it was. This is how a service linked
// before w4/m355, in link order, adopts creation order: only at a release its
// owner started, never by a reconcile or a write that ships nothing.
func PrepareGroupRefsForRelease(a *appv1alpha1.App) {
	if a == nil {
		return
	}
	g := readGroupLinks(a)
	a.Spec.EnvFromSecrets = SortGroupRefs(g.Env.apply(a.Spec.EnvFromSecrets))
	a.Spec.FilesFromSecrets = SortGroupRefs(g.Files.apply(a.Spec.FilesFromSecrets))
	delete(a.Annotations, appv1alpha1.AnnotationPendingGroupLinks)
}

// Between two linked environment groups that define the same env var or secret
// file, a service runs the value of the most recently CREATED group, as Render
// documents: "Currently, Render uses the value from the most recently created
// environment group" (render.com/docs/configure-environment-variables, read
// 2026-11-14). The operator lets the LAST ref win a collision, and puts the
// service's own Secret after every group, so the writers keep the group refs
// oldest-created first, whatever order they were linked in (w4/m355).

// InsertGroupRef returns list with ref present once and its group refs in
// creation order. A link writer calls it in place of an append.
func InsertGroupRef(list []string, ref string) []string {
	if !slices.Contains(list, ref) {
		list = append(slices.Clone(list), ref)
	}
	return SortGroupRefs(list)
}

// SortGroupRefs returns a copy of refs with its env-group refs
// ("<evg-id>-env", "<evg-id>-files") in group creation order, oldest first.
// They are sorted within the slots group refs already hold, so every other ref
// keeps its position.
//
// A group id is an xid that id.New mints when the group is created, at the
// same instant as its CreatedAt, and an xid's string sorts by its embedded
// creation second first (then machine, process and counter). Ordering by id is
// therefore creation order with the id as tie-break, and it needs no store
// read: the release path that applies it (Snapshot.Stamp) is pure, and one
// rule on every path means a release never reorders what a link wrote.
func SortGroupRefs(refs []string) []string {
	out := slices.Clone(refs)
	var slots []int
	var groups []string
	for i, ref := range out {
		if isGroupRef(ref) {
			slots = append(slots, i)
			groups = append(groups, ref)
		}
	}
	slices.Sort(groups)
	for i, slot := range slots {
		out[slot] = groups[i]
	}
	return out
}

// isGroupRef reports whether ref names an env group's env or files Secret.
func isGroupRef(ref string) bool {
	gid, ok := strings.CutSuffix(ref, "-env")
	if !ok {
		gid, ok = strings.CutSuffix(ref, "-files")
	}
	kind, isID := id.KindOf(gid)
	return ok && isID && kind == id.EnvGroup
}
