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
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Between two linked groups the most recently CREATED one wins, as Render
// documents (w4/m355). The ids are round 247's: A was created at 00:56:09,
// B at 00:58:50, and an xid sorts by its creation second.
const (
	olderGroup = "evg-dbr60a95t55c738j9nmg" // A
	newerGroup = "evg-dbr61inoj40s73fktaig" // B
)

func TestInsertGroupRefKeepsCreationOrderWhateverTheLinkOrder(t *testing.T) {
	for _, suffix := range []string{"-env", "-files"} {
		a, b := olderGroup+suffix, newerGroup+suffix
		want := []string{a, b}

		// B linked first, then A (round 247's relink): A must not land last.
		if got := InsertGroupRef(InsertGroupRef(nil, b), a); !slices.Equal(got, want) {
			t.Fatalf("link B then A = %v, want %v", got, want)
		}
		linked := InsertGroupRef(InsertGroupRef(nil, a), b)
		if !slices.Equal(linked, want) {
			t.Fatalf("link A then B = %v, want %v", linked, want)
		}
		// Unlink A, relink A: the winner does not change.
		relinked := InsertGroupRef(slices.DeleteFunc(slices.Clone(linked), func(v string) bool { return v == a }), a)
		if !slices.Equal(relinked, want) {
			t.Fatalf("unlink then relink A = %v, want %v", relinked, want)
		}
		// Linking a group already there is a no-op.
		if got := InsertGroupRef(relinked, b); !slices.Equal(got, want) {
			t.Fatalf("relink of a linked group = %v, want %v", got, want)
		}
	}
}

// Only group refs move, within the slots they hold. The service's own files
// Secret and any ref that is not a well-formed group id keep their position,
// and the input is never mutated.
func TestSortGroupRefsMovesOnlyGroupRefs(t *testing.T) {
	in := []string{"web-files", newerGroup + "-files", "evg-legacy-files", olderGroup + "-files"}
	got := SortGroupRefs(in)
	want := []string{"web-files", olderGroup + "-files", "evg-legacy-files", newerGroup + "-files"}
	if !slices.Equal(got, want) {
		t.Fatalf("SortGroupRefs = %v, want %v", got, want)
	}
	if in[1] != newerGroup+"-files" {
		t.Fatalf("SortGroupRefs mutated its input: %v", in)
	}
}

// A staged link on an Auto-Deploy-off service folds into creation order at the
// next release, and the effective refs already report that order.
func TestStagedLinkFoldsIntoCreationOrder(t *testing.T) {
	cl, a := lockedPatchFixture(t, interceptor.Funcs{})
	a.Spec.EnvFromSecrets = []string{newerGroup + "-env"}
	a.Spec.FilesFromSecrets = []string{newerGroup + "-files"}
	StageGroupLink(a, olderGroup+"-env", olderGroup+"-files", true)
	wantEnv := []string{olderGroup + "-env", newerGroup + "-env"}
	wantFiles := []string{olderGroup + "-files", newerGroup + "-files"}
	if env, files := EffectiveGroupSecrets(a); !slices.Equal(env, wantEnv) || !slices.Equal(files, wantFiles) {
		t.Fatalf("effective refs = %v %v, want %v %v", env, files, wantEnv, wantFiles)
	}
	if err := cl.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	tr := &Tracker{Store: &fakeDeployStore{}}
	if err := tr.PatchLocked(context.Background(), cl, a, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
		a.Spec.HealthCheckPath = "/healthz" // rolls a release
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(a.Spec.EnvFromSecrets, wantEnv) || !slices.Equal(a.Spec.FilesFromSecrets, wantFiles) {
		t.Fatalf("folded refs = %v %v, want %v %v", a.Spec.EnvFromSecrets, a.Spec.FilesFromSecrets, wantEnv, wantFiles)
	}
}

// A service linked before w4/m355 holds its refs in link order. That order
// changes only in a write that mints a release: never in a save-only write, an
// operational write, or by staging an unrelated link.
func TestLegacyLinkOrderChangesOnlyAtARelease(t *testing.T) {
	cl, a := lockedPatchFixture(t, interceptor.Funcs{})
	legacy := []string{newerGroup + "-env", olderGroup + "-env"}
	a.Spec.EnvFromSecrets = slices.Clone(legacy)
	if err := cl.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	ds := &fakeDeployStore{}
	tr := &Tracker{Store: ds}
	patch := func(ctx context.Context, mutate func(*appv1alpha1.App)) {
		t.Helper()
		if err := tr.PatchLocked(ctx, cl, a, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
			mutate(a)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	patch(WithSaveOnly(context.Background()), func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/saved" })
	patch(context.Background(), func(a *appv1alpha1.App) { a.Spec.Replicas = 2 })
	patch(context.Background(), func(a *appv1alpha1.App) {
		StageGroupLink(a, "evg-vvvvvvvvvvvvvvvvvvvv-env", "evg-vvvvvvvvvvvvvvvvvvvv-files", true)
	})
	if !slices.Equal(a.Spec.EnvFromSecrets, legacy) || len(ds.created) != 0 {
		t.Fatalf("a write that ships nothing reordered the running refs: %v (rows %d)", a.Spec.EnvFromSecrets, len(ds.created))
	}

	patch(context.Background(), func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/healthz" })
	want := []string{olderGroup + "-env", newerGroup + "-env", "evg-vvvvvvvvvvvvvvvvvvvv-env"}
	if !slices.Equal(a.Spec.EnvFromSecrets, want) || len(ds.created) != 1 {
		t.Fatalf("the release refs = %v (rows %d), want %v", a.Spec.EnvFromSecrets, len(ds.created), want)
	}
}
