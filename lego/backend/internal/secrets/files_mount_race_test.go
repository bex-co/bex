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
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// beforeFirstAppPatch runs hook once, just before the first App patch goes
// out: a concurrent write landing between a write's projection and its patch.
type beforeFirstAppPatch struct {
	client.Client
	hook func()
}

func (c *beforeFirstAppPatch) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*appv1alpha1.App); ok && c.hook != nil {
		hook := c.hook
		c.hook = nil
		hook()
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func emptyFiles(svc *Service) error {
	_, err := emptyFilesResult(svc)
	return err
}

func emptyFilesResult(svc *Service) (EnvironmentPatchResult, error) {
	return svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SecretFiles: []SecretFilePatch{{Name: "f", Delete: true}}, SaveMode: SaveModeDeploy,
	})
}

func setFile(svc *Service) error {
	_, err := svc.SetSecretFile(context.Background(), "web", "g", "v")
	return err
}

// TestAnEmptyingWriteThatLosesTheRaceKeepsTheLaterMount (w5/m131): a
// deploying batch empties the files map, projects the empty Secret at
// revision 2 and stalls before its App patch. SetSecretFile(g) then commits
// {g} at revision 3, projects it and patches the reference in. The batch's
// patch, computed from its stale read, used to land without the reference, so
// the pod stopped mounting the {g} the store and the Secret held.
func TestAnEmptyingWriteThatLosesTheRaceKeepsTheLaterMount(t *testing.T) {
	c := projectionCases[1]
	var hooked *beforeFirstAppPatch
	svc, store := c.setup(func(cl client.Client) client.Client {
		hooked = &beforeFirstAppPatch{Client: cl}
		return hooked
	}, map[string]string{"f": "1"})
	var concurrent error
	var concurrentStamp string
	hooked.hook = func() {
		concurrent = setFile(svc)
		concurrentStamp = getApp(t, svc.Client, "web").Spec.RestartedAt
	}
	// Each restart stamp differs, so the batch's patch, retried after the
	// conflict, is a real write.
	now := fixedNow()
	svc.Clock = func() time.Time {
		now = now.Add(time.Second)
		return now
	}

	result, err := emptyFilesResult(svc)
	if err != nil || !result.RolledOut {
		t.Fatalf("emptying batch = rolledOut %v, %v; want it rolled out", result.RolledOut, err)
	}
	if concurrent != nil {
		t.Fatalf("SetSecretFile: %v", concurrent)
	}
	if store.versions[c.path] != 3 || len(store.m[c.path]) != 1 {
		t.Fatalf("store = %v at %d, want {g} at 3", store.m[c.path], store.versions[c.path])
	}
	c.expectMatchesStore(t, svc, store)
	if got := getApp(t, svc.Client, "web").Spec.RestartedAt; got <= concurrentStamp {
		t.Fatalf("restartedAt = %s, want the batch's retried stamp, after SetSecretFile's %s", got, concurrentStamp)
	}
}

// TestAnAddThatLosesTheRaceRestoresTheMount (w5/m131): SetSecretFile(g) reads
// the App with the reference present. An emptying batch then commits,
// projects and removes the reference before SetSecretFile commits {g} at
// revision 3. SetSecretFile's patch, which saw no reference to add, used to
// leave the removal in place.
func TestAnAddThatLosesTheRaceRestoresTheMount(t *testing.T) {
	c := projectionCases[1]
	svc, store := c.setup(nil, map[string]string{"f": "1"})
	var concurrent error
	store.afterGet = func() { concurrent = emptyFiles(svc) }

	if err := setFile(svc); err != nil {
		t.Fatalf("SetSecretFile: %v", err)
	}
	if concurrent != nil {
		t.Fatalf("emptying batch: %v", concurrent)
	}
	if store.versions[c.path] != 3 || len(store.m[c.path]) != 1 {
		t.Fatalf("store = %v at %d, want {g} at 3", store.m[c.path], store.versions[c.path])
	}
	c.expectMatchesStore(t, svc, store)
}

// conflictingAppPatches answers every App patch with a conflict, counting them.
type conflictingAppPatches struct {
	client.Client
	attempts int
}

func (c *conflictingAppPatches) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*appv1alpha1.App); ok {
		c.attempts++
		return apierrors.NewConflict(schema.GroupResource{Group: "app.bex.co", Resource: "apps"}, obj.GetName(), errors.New("changed"))
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

// TestABatchThatKeepsLosingTheAppRaceCompensates (w5/m131): a batch re-reads
// and retries its App patch a bounded number of times, and only then
// compensates. The store goes back to the map it replaced, and the call
// answers a conflict, not an internal error.
func TestABatchThatKeepsLosingTheAppRaceCompensates(t *testing.T) {
	c := projectionCases[1]
	var conflicting *conflictingAppPatches
	svc, store := c.setup(func(cl client.Client) client.Client {
		conflicting = &conflictingAppPatches{Client: cl}
		return conflicting
	}, map[string]string{"f": "1"})

	if err := emptyFiles(svc); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a batch whose App patch always conflicts = %v, want the conflict refusal", err)
	}
	if conflicting.attempts < 2 {
		t.Fatalf("App patch attempts = %d, want the patch retried", conflicting.attempts)
	}
	if got := store.m[c.path]; len(got) != 1 || got["f"] != "1" {
		t.Fatalf("store = %v, want the replaced {f} restored", got)
	}
	c.expectMatchesStore(t, svc, store)
}

// TestACASSaveThatLosesTheAppRaceToAnEnvWriteStillLands (w5/m131): a CAS save
// commits TOKEN, projects it, and before its App patch a SetEnvVar commits
// and patches the App. The save's patch conflicts and runs again. It must not
// project the env map again, which a later env write would refuse; it keeps
// the save it already stored.
func TestACASSaveThatLosesTheAppRaceToAnEnvWriteStillLands(t *testing.T) {
	c := projectionCases[0]
	var hooked *beforeFirstAppPatch
	svc, store := c.setup(func(cl client.Client) client.Client {
		hooked = &beforeFirstAppPatch{Client: cl}
		return hooked
	}, map[string]string{"TOKEN": "old"})
	var concurrent error
	hooked.hook = func() { concurrent = c.write(svc, "OTHER") }
	revision := encodeEnvRevision(1)

	result, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, ExpectedEnvRevision: &revision,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "new"}},
	})
	if err != nil || !result.RolledOut {
		t.Fatalf("CAS save = rolledOut %v, %v; want it rolled out", result.RolledOut, err)
	}
	if concurrent != nil {
		t.Fatalf("SetEnvVar: %v", concurrent)
	}
	if got := store.m[c.path]; got["TOKEN"] != "new" || got["OTHER"] != "v" {
		t.Fatalf("store = %v, want both writes kept", got)
	}
	c.expectMatchesStore(t, svc, store)
}
