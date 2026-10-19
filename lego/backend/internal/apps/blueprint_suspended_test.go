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

package apps

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestBlueprintSyncOnSuspendedServiceHoldsTheDeploy (w4/366): a Blueprint
// sync that changes a suspended service persists the intent and spec and
// writes the pending-source hold — no release stamp, no deploy row, no
// commit resolution, no restartedAt bump — like every other release-rolling
// writer since w4/m234. The running-service control still stamps the
// release and opens exactly one Blueprint deploy row.
func TestBlueprintSyncOnSuspendedServiceHoldsTheDeploy(t *testing.T) {
	for name, tc := range map[string]struct {
		suspended  bool
		wantRows   int
		wantHold   string
		wantStamp  string
		wantCommit string
	}{
		"suspended is held": {suspended: true, wantRows: 0, wantHold: "8", wantStamp: "7", wantCommit: "oldsha"},
		"running deploys":   {suspended: false, wantRows: 1, wantHold: "", wantStamp: "8", wantCommit: ""},
	} {
		t.Run(name, func(t *testing.T) {
			existing := blueprintIntentApp(true)
			existing.Spec.Suspended = tc.suspended
			existing.Spec.StartCommand = "node old.js"
			existing.Spec.BuildCommit = "oldsha"
			existing.Spec.RestartedAt = "2026-01-01T00:00:00Z"
			existing.Annotations = map[string]string{
				appv1alpha1.AnnotationReleaseGeneration: "7",
			}
			cl := fakeClient(existing)
			rec := &recordingStore{}
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: rec}
			final := existing.DeepCopy()
			final.Spec.StartCommand = "node new.js"

			if _, err := svc.patchChangedStackService(context.Background(), CreateRequest{Name: existing.Name}, existing, final, stackChanges{specChanged: true}); err != nil {
				t.Fatal(err)
			}
			if len(rec.deployCalls) != tc.wantRows {
				t.Errorf("deploy rows = %d, want %d", len(rec.deployCalls), tc.wantRows)
			}
			stored := &appv1alpha1.App{}
			if err := cl.Get(context.Background(), client.ObjectKeyFromObject(existing), stored); err != nil {
				t.Fatal(err)
			}
			if stored.Spec.StartCommand != "node new.js" {
				t.Errorf("start command = %q, want the synced value persisted", stored.Spec.StartCommand)
			}
			if got := stored.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; got != tc.wantHold {
				t.Errorf("pending-source hold = %q, want %q", got, tc.wantHold)
			}
			if got := stored.Annotations[appv1alpha1.AnnotationReleaseGeneration]; got != tc.wantStamp {
				t.Errorf("release stamp = %q, want %q", got, tc.wantStamp)
			}
			if tc.suspended {
				if stored.Spec.BuildCommit != tc.wantCommit {
					t.Errorf("build commit = %q, want %q (no commit resolution while held)", stored.Spec.BuildCommit, tc.wantCommit)
				}
				if stored.Spec.RestartedAt != "2026-01-01T00:00:00Z" {
					t.Errorf("restartedAt = %q, want the prior value (no bump while held)", stored.Spec.RestartedAt)
				}
			}
		})
	}
}
