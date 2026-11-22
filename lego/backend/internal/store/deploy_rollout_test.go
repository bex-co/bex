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

package store

import (
	"context"
	"slices"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/types/deploybudgets"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/m560: the reconciler copies the operator's record of a release's
// rollout onto that release's own open row: the start, command and default
// once, the app starts as they grow, and on the pass that closes the row too.
// A record of another release never stamps a row.
func TestReconcilerCopiesTheReleasesRolloutOntoItsOwnRow(t *testing.T) {
	ctx := context.Background()
	m := newMemStore()
	m.deploys["dep-2"] = Deploy{ID: "dep-2", Generation: 2, Status: DeployUpdateInProgress, CreatedAt: time.Now()}
	m.deploys["dep-3"] = Deploy{ID: "dep-3", Generation: 3, Status: DeployQueued, CreatedAt: time.Now()}
	r := &Reconciler{Store: m, DeployGateTimeout: deploybudgets.DeployGateTimeout}
	started := time.Date(2026, 11, 22, 22, 14, 46, 270000000, time.UTC)
	first, second := metav1.NewTime(started.Add(12*time.Second).Truncate(time.Second)), metav1.NewTime(started.Add(20*time.Second).Truncate(time.Second))
	app := &appv1alpha1.App{}
	app.Generation = 3
	app.Status.ReleaseGeneration = 2
	app.Status.Phase = appv1alpha1.PhaseDeploying
	app.Status.ReleaseRollout = &appv1alpha1.ReleaseRolloutStatus{
		Generation: 2, StartedAt: metav1.NewMicroTime(started), Command: "npm start", WebConcurrency: 1,
	}
	pass := func(id string) Deploy {
		t.Helper()
		r.recordDeploy(ctx, DesiredApp{}, m.deploys[id], app)
		return m.deploys[id]
	}

	d := pass("dep-2")
	if d.Rollout.StartedAt == nil || !d.Rollout.StartedAt.Equal(started) || d.Rollout.Command != "npm start" ||
		d.Rollout.WebConcurrency != 1 || len(d.Rollout.AppStarts) != 0 {
		t.Fatalf("first pass: rollout = %+v, want the record's start, command and default", d.Rollout)
	}
	if got := pass("dep-3").Rollout; got.StartedAt != nil {
		t.Fatalf("release 2's record stamped release 3's row: %+v", got)
	}

	app.Status.ReleaseRollout.AppStarts = []metav1.Time{first}
	app.Status.ReleaseRollout.Command = "node other.js"
	if d := pass("dep-2"); !slices.Equal(d.Rollout.AppStarts, []time.Time{first.Time}) || d.Rollout.Command != "npm start" {
		t.Fatalf("a start: rollout = %+v, want the start added and the command kept", d.Rollout)
	}

	// The pass that sees the release serving closes the row, and copies the
	// last start first.
	app.Status.ReleaseRollout.AppStarts = []metav1.Time{first, second}
	app.Status.Phase, app.Status.ActiveRevision = appv1alpha1.PhaseRunning, "rev-2"
	app.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Deployed", ObservedGeneration: 3}}
	d = pass("dep-2")
	if d.Status != DeployLive {
		t.Fatalf("status = %q, want the row closed live on this pass", d.Status)
	}
	if !slices.Equal(d.Rollout.AppStarts, []time.Time{first.Time, second.Time}) {
		t.Fatalf("closing pass: starts = %v, want both", d.Rollout.AppStarts)
	}

	// A start the operator records after the close is not the deploy's.
	app.Status.ReleaseRollout.AppStarts = append(app.Status.ReleaseRollout.AppStarts, metav1.NewTime(second.Add(time.Minute)))
	if err := m.SetDeployRollout(ctx, "dep-2", DeployRollout{StartedAt: &started, AppStarts: []time.Time{second.Add(time.Minute)}}); err != nil {
		t.Fatal(err)
	}
	if got := m.deploys["dep-2"].Rollout.AppStarts; len(got) != 2 {
		t.Fatalf("a closed row grew: %v", got)
	}
}

// A row that already holds the record is not written again on each pass.
func TestDeployRolloutHoldsWhatTheRecordWouldAdd(t *testing.T) {
	started := time.Date(2026, 11, 22, 22, 14, 46, 0, time.UTC)
	a, b := started.Add(time.Second), started.Add(2*time.Second)
	row := DeployRollout{StartedAt: &started, AppStarts: []time.Time{a}}
	if (DeployRollout{}).holds(DeployRollout{StartedAt: &started}) {
		t.Error("an unstamped row holds nothing")
	}
	if !row.holds(DeployRollout{StartedAt: &started, AppStarts: []time.Time{a}}) {
		t.Error("the same record must not be written again")
	}
	if row.holds(DeployRollout{StartedAt: &started, AppStarts: []time.Time{a, b}}) {
		t.Error("a new start must be written")
	}
	full := DeployRollout{StartedAt: &started}
	for i := range appv1alpha1.MaxReleaseRolloutAppStarts {
		full.AppStarts = append(full.AppStarts, started.Add(time.Duration(i)*time.Second))
	}
	if !full.holds(DeployRollout{StartedAt: &started, AppStarts: []time.Time{started.Add(time.Hour)}}) {
		t.Error("a row at its bound takes no more starts")
	}
}
