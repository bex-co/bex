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

package deploys

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m117: a cancel ends one deploy. Canceling the deploy waiting in the
// latest-pending slot names the older one ahead of it as the release left
// running, and the commit its row records as the one to build. Canceling
// anything else names none, and the cancel settles as it always has: the older
// deploy itself, a pending deploy that brought settings of its own, one behind
// a rollback, or one of an image-backed service.
func TestCancelOfThePendingDeployLeavesTheOlderOneRunning(t *testing.T) {
	ctx := context.Background()
	repoApp := func() *appv1alpha1.App {
		a := sampleApp("web", "srv-1")
		a.Spec.Image, a.Spec.Repo, a.Spec.BuildCommit = "", "https://example.invalid/acme/web.git", "bbb"
		a.Annotations = map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}
		return a
	}
	for name, tc := range map[string]struct {
		app                    *appv1alpha1.App
		olderTrigger, olderImg string
		pendingTrigger         string
		cancelOlder            bool
		wantCanceled, wantKept string
		wantCommit             string
	}{
		"a duplicate deploy":         {repoApp(), store.TriggerCreate, "", store.TriggerAPI, false, "2", "1", "aaa"},
		"a pushed commit":            {repoApp(), store.TriggerAPI, "", store.TriggerNewCommit, false, "2", "1", "aaa"},
		"the older deploy itself":    {repoApp(), store.TriggerCreate, "", store.TriggerAPI, true, "1", "", "bbb"},
		"a configuration change":     {repoApp(), store.TriggerCreate, "", store.TriggerConfigChange, false, "2", "", "bbb"},
		"a blueprint sync":           {repoApp(), store.TriggerCreate, "", "blueprint", false, "2", "", "bbb"},
		"a deploy behind a rollback": {repoApp(), store.TriggerRollback, "web@sha256:old", store.TriggerAPI, false, "2", "", "bbb"},
		"an image-backed service":    {sampleApp("web", "srv-1"), store.TriggerCreate, "", store.TriggerAPI, false, "2", "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			ds := newFakeStore()
			older, _ := ds.CreateDeploy(ctx, "srv-1", tc.olderTrigger, tc.olderImg, 1, store.CommitInfo{Hash: "aaa"}, "")
			pending, _ := ds.CreateDeploy(ctx, "srv-1", tc.pendingTrigger, "", 2, store.CommitInfo{Hash: "bbb"}, "")
			if !pending.OverlapPending {
				t.Fatalf("precondition: the newer deploy waits in the pending slot, got %+v", pending)
			}
			svc, cl := newService(ds, tc.app)
			target, other := pending, older
			if tc.cancelOlder {
				target, other = older, pending
			}

			got, err := svc.Cancel(ctx, "web", target.ID)
			if err != nil || got.Status != store.DeployCanceled {
				t.Fatalf("Cancel = %+v, %v; want the deploy canceled", got, err)
			}
			if row, _ := ds.GetDeploy(ctx, "srv-1", other.ID); !store.IsOpenDeployStatus(row.Status) {
				t.Errorf("the other deploy is %s, want it left open", row.Status)
			}
			a := getApp(t, cl, "web")
			if canceled, kept := a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration],
				a.Annotations[appv1alpha1.AnnotationSurvivingReleaseGeneration]; canceled != tc.wantCanceled || kept != tc.wantKept {
				t.Errorf("canceled=%q left running=%q, want %q %q", canceled, kept, tc.wantCanceled, tc.wantKept)
			}
			if a.Spec.BuildCommit != tc.wantCommit {
				t.Errorf("saved commit = %q, want %q", a.Spec.BuildCommit, tc.wantCommit)
			}
		})
	}
}
