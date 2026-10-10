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
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	jsonpatch "github.com/evanphx/json-patch/v5"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Stage holds one App's writes in memory so a request that changes several of
// its settings publishes them as ONE operator-visible configuration (w8/m56).
//
// A compound PATCH /v1/services/{id} applies each field through its own setter,
// and each setter used to commit its own App patch. The operator reconciles
// every stored generation, so a patch clearing the health check and the
// pre-deploy command together first published "health cleared, old pre-deploy
// command" — a release that ran the command the same request removed. Batch
// collapsed the deploy-history ROWS of such a request; it never held the App
// writes the rows describe.
//
// Stage is a client.Client: reads of the staged App answer the staged copy, and
// writes to it are applied to that copy instead of the API server. Every other
// object, every dry-run, and every status write passes straight through, so
// each setter keeps its own validation, audit and store effects unchanged.
// Publish then writes the accumulated change once.
type Stage struct {
	client.Client

	mu       sync.Mutex
	key      client.ObjectKey
	original *appv1alpha1.App
	staged   *appv1alpha1.App
}

// NewStage stages writes to a (as read) on top of cl.
func NewStage(cl client.Client, a *appv1alpha1.App) *Stage {
	return &Stage{
		Client:   cl,
		key:      client.ObjectKeyFromObject(a),
		original: a.DeepCopy(),
		staged:   a.DeepCopy(),
	}
}

func (s *Stage) holds(obj client.Object) (*appv1alpha1.App, bool) {
	a, ok := obj.(*appv1alpha1.App)
	return a, ok && client.ObjectKeyFromObject(a) == s.key
}

// Get answers the staged App for its key.
func (s *Stage) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if a, ok := obj.(*appv1alpha1.App); ok && key == s.key {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.staged.DeepCopyInto(a)
		return nil
	}
	return s.Client.Get(ctx, key, obj, opts...)
}

// List answers the API server's list with the staged App in place of the live one.
func (s *Stage) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := s.Client.List(ctx, list, opts...); err != nil {
		return err
	}
	apps, ok := list.(*appv1alpha1.AppList)
	if !ok {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range apps.Items {
		if client.ObjectKeyFromObject(&apps.Items[i]) == s.key {
			s.staged.DeepCopyInto(&apps.Items[i])
		}
	}
	return nil
}

// Patch applies a write to the staged App to the staged copy and answers obj
// with the result, as the API server would. Dry-runs go to the API server, and
// so does a write that leaves the spec alone: it moves nothing the operator
// releases, and a setter may need it durable before its store write — the
// source row's pending-source marker must reach the App before the projector
// can see the new source row (SetSourceAndRegistryCredential). The stage takes
// s.mu, then the Batch's lock (refile, drop); never the reverse.
func (s *Stage) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	a, ok := s.holds(obj)
	if !ok || isDryRun(opts) {
		return s.Client.Patch(ctx, obj, patch, opts...)
	}
	data, err := patch.Data(obj)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next, err := applyPatch(s.staged, patch.Type(), data)
	if err != nil {
		return err
	}
	return s.stageOrWrite(next, a, func() error { return s.Client.Patch(ctx, obj, patch, opts...) })
}

// stageOrWrite stages next unless it leaves the spec alone, in which case write
// sends it to the API server and the stage rebases onto the App it answered —
// so the publish neither repeats that write nor loses its lock to it.
func (s *Stage) stageOrWrite(next, a *appv1alpha1.App, write func() error) error {
	if !apiequality.Semantic.DeepEqual(next.Spec, s.staged.Spec) {
		s.staged = next
		next.DeepCopyInto(a)
		return nil
	}
	if err := write(); err != nil {
		return err
	}
	live := a.DeepCopy()
	rebased, err := s.rebase(live)
	if err != nil {
		return err
	}
	s.original, s.staged = live, rebased
	rebased.DeepCopyInto(a)
	return nil
}

func applyPatch(a *appv1alpha1.App, patchType types.PatchType, data []byte) (*appv1alpha1.App, error) {
	current, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	var patched []byte
	switch patchType {
	case types.MergePatchType:
		patched, err = jsonpatch.MergePatch(current, data)
	case types.JSONPatchType:
		var p jsonpatch.Patch
		if p, err = jsonpatch.DecodePatch(data); err == nil {
			patched, err = p.Apply(current)
		}
	default:
		return nil, fmt.Errorf("rollout: cannot stage a %s patch to service %q", patchType, a.Name)
	}
	if err != nil {
		return nil, err
	}
	next := &appv1alpha1.App{}
	if err := json.Unmarshal(patched, next); err != nil {
		return nil, err
	}
	next.ResourceVersion, next.Generation, next.UID = a.ResourceVersion, a.Generation, a.UID
	return next, nil
}

// Update is Patch for a whole-object write.
func (s *Stage) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	a, ok := s.holds(obj)
	if !ok || isDryRunUpdate(opts) {
		return s.Client.Update(ctx, obj, opts...)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := a.DeepCopy()
	next.ResourceVersion, next.Generation, next.UID = s.staged.ResourceVersion, s.staged.Generation, s.staged.UID
	next.Status = s.staged.Status
	return s.stageOrWrite(next, a, func() error { return s.Client.Update(ctx, obj, opts...) })
}

func isDryRun(opts []client.PatchOption) bool {
	var po client.PatchOptions
	po.ApplyOptions(opts)
	return len(po.DryRun) > 0
}

func isDryRunUpdate(opts []client.UpdateOption) bool {
	var uo client.UpdateOptions
	uo.ApplyOptions(opts)
	return len(uo.DryRun) > 0
}

// Publish writes the staged App in one patch under an optimistic lock, and
// re-files a held Batch rollout under the generation that write produced. It
// answers nil when nothing was staged.
//
// A lost race re-reads the live App and re-applies this request's own change on
// top of it — the same field-level outcome the setters' individual merge
// patches had — re-stamping the generation annotations against the new read.
// Publish is safe to call more than once; later calls publish what was staged
// since the last.
func (s *Stage) Publish(ctx context.Context) (*appv1alpha1.App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if apiequality.Semantic.DeepEqual(s.original.ObjectMeta, s.staged.ObjectMeta) &&
		apiequality.Semantic.DeepEqual(s.original.Spec, s.staged.Spec) {
		return nil, nil
	}
	var live *appv1alpha1.App
	next := s.original.DeepCopy()
	if err := PatchAppLocked(ctx, s.Client, next, func(a *appv1alpha1.App) error {
		live = a.DeepCopy()
		rebased, err := s.rebase(live)
		if err != nil {
			return err
		}
		*a = *rebased
		return nil
	}); err != nil {
		// Nothing was published, so no release is rolling to record.
		drop(ctx)
		return nil, err
	}
	refile(ctx, live, next)
	s.original, s.staged = next.DeepCopy(), next.DeepCopy()
	return next, nil
}

// rebase re-applies the staged change on top of live — on the first attempt
// live is original, so the result is exactly the staged App. The two
// generation annotations a setter stamps name "the generation my write
// produces", which after a lost race is live's next one, not original's.
func (s *Stage) rebase(live *appv1alpha1.App) (*appv1alpha1.App, error) {
	original, err := json.Marshal(s.original)
	if err != nil {
		return nil, err
	}
	staged, err := json.Marshal(s.staged)
	if err != nil {
		return nil, err
	}
	current, err := json.Marshal(live)
	if err != nil {
		return nil, err
	}
	diff, err := jsonpatch.CreateMergePatch(original, staged)
	if err != nil {
		return nil, err
	}
	merged, err := jsonpatch.MergePatch(current, diff)
	if err != nil {
		return nil, err
	}
	next := &appv1alpha1.App{}
	if err := json.Unmarshal(merged, next); err != nil {
		return nil, err
	}
	following := strconv.FormatInt(live.Generation+1, 10)
	if next.Annotations == nil {
		next.Annotations = map[string]string{}
	}
	if s.staged.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != s.original.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] {
		next.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] = following
	}
	if s.staged.Annotations[appv1alpha1.AnnotationReleaseGeneration] != s.original.Annotations[appv1alpha1.AnnotationReleaseGeneration] {
		if appv1alpha1.SpecRollsRelease(live.Spec, next.Spec) {
			next.Annotations[appv1alpha1.AnnotationReleaseGeneration] = following
		} else if prior, ok := live.Annotations[appv1alpha1.AnnotationReleaseGeneration]; ok {
			next.Annotations[appv1alpha1.AnnotationReleaseGeneration] = prior
		} else {
			delete(next.Annotations, appv1alpha1.AnnotationReleaseGeneration)
		}
	}
	return next, nil
}

// drop discards a Batch's held rollout.
func drop(ctx context.Context) {
	if b, ok := ctx.Value(batchKey{}).(*batch); ok {
		b.mu.Lock()
		b.tracker, b.app = nil, nil
		b.mu.Unlock()
	}
}

// refile points a Batch's held rollout at the write that actually published
// it: from live (the App the publish patched) to published. The staged setters
// held snapshots of in-memory steps; the operator reconciles only this one.
// A held rollout the published change no longer rolls (a racing writer already
// made it) is dropped. Nothing is held for an untracked change — a saved source
// update stays deferred.
func refile(ctx context.Context, live, published *appv1alpha1.App) {
	b, ok := ctx.Value(batchKey{}).(*batch)
	if !ok {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tracker == nil {
		return
	}
	before := Before(live)
	if !before.Stamp(published.DeepCopy()) {
		b.tracker, b.app = nil, nil
		return
	}
	b.snapshot, b.app = before, published
}
