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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/types/deploybudgets"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/m572: the reconciler copies the operator's record of a release's
// pre-deploy step onto that release's own open row: the launch, command and
// native launch once, the container's start when it comes, and the end with
// its exit status on the pass that closes a failed step's row. A record of
// another release never stamps a row, and a closed row keeps what it had.
func TestReconcilerCopiesTheReleasesPreDeployRunOntoItsOwnRow(t *testing.T) {
	ctx := context.Background()
	m := newMemStore()
	m.deploys["dep-2"] = Deploy{ID: "dep-2", Generation: 2, Status: DeployPreDeployInProgress, CreatedAt: time.Now()}
	m.deploys["dep-3"] = Deploy{ID: "dep-3", Generation: 3, Status: DeployQueued, CreatedAt: time.Now()}
	r := &Reconciler{Store: m, DeployGateTimeout: deploybudgets.DeployGateTimeout, PreDeployGateTimeout: deploybudgets.PreDeployGateTimeout}
	launched := time.Date(2026, 11, 22, 11, 29, 14, 990123000, time.UTC)
	container := metav1.NewTime(time.Date(2026, 11, 22, 11, 29, 27, 0, time.UTC))
	settled := time.Date(2026, 11, 22, 11, 29, 31, 490456000, time.UTC)
	app := &appv1alpha1.App{}
	app.Generation = 3
	app.Spec.PreDeployCommand = "npx prisma migrate deploy"
	app.Status.ReleaseGeneration = 2
	app.Status.Phase = appv1alpha1.PhaseDeploying
	app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
		Job: "pre-web-gen-2", Generation: 2, Status: appv1alpha1.PreDeployRunning, StartedAt: launched.Format(time.RFC3339),
		Command: "npx prisma migrate deploy", Native: true, LaunchedAt: new(metav1.NewMicroTime(launched)),
	}
	pass := func(id string) Deploy {
		t.Helper()
		r.recordDeploy(ctx, DesiredApp{}, m.deploys[id], app)
		return m.deploys[id]
	}

	run := pass("dep-2").PreDeployRun
	if run.StartedAt == nil || !run.StartedAt.Equal(launched) || run.Command != "npx prisma migrate deploy" || !run.Native ||
		run.ContainerStartedAt != nil || run.FinishedAt != nil || run.ExitCode != nil {
		t.Fatalf("first pass: run = %+v, want the record's launch, command and native launch", run)
	}
	if got := pass("dep-3").PreDeployRun; got != (DeployPreDeployRun{}) {
		t.Fatalf("release 2's record stamped release 3's row: %+v", got)
	}

	app.Status.PreDeploy.ContainerStartedAt = &container
	app.Status.PreDeploy.Command = "other"
	run = pass("dep-2").PreDeployRun
	if run.ContainerStartedAt == nil || !run.ContainerStartedAt.Equal(container.Time) || run.Command != "npx prisma migrate deploy" {
		t.Fatalf("the container's start: run = %+v, want it added and the command kept", run)
	}

	// The pass that sees the step fail closes the row, and copies its end and
	// exit status first.
	code := int32(1)
	pd := app.Status.PreDeploy
	pd.Status, pd.Message, pd.FinishedAt = appv1alpha1.PreDeployFailed, "the pre-deploy command exited with code 1; check the pre-deploy logs", settled.Format(time.RFC3339)
	pd.SettledAt, pd.ExitCode = new(metav1.NewMicroTime(settled)), &code
	d := pass("dep-2")
	if d.Status != DeployPreDeployFailed {
		t.Fatalf("status = %q, want the row closed pre_deploy_failed on this pass", d.Status)
	}
	if run := d.PreDeployRun; run.FinishedAt == nil || !run.FinishedAt.Equal(settled) || run.ExitCode == nil || *run.ExitCode != 1 || run.OOMKilled {
		t.Fatalf("closing pass: run = %+v, want the end and exit status 1", run)
	}

	// A closed row keeps what it had.
	later, other := settled.Add(time.Minute), int32(2)
	if err := m.SetDeployPreDeployRun(ctx, "dep-2", DeployPreDeployRun{StartedAt: &later, FinishedAt: &later, ExitCode: &other, Command: "x"}); err != nil {
		t.Fatal(err)
	}
	if run := m.deploys["dep-2"].PreDeployRun; !run.StartedAt.Equal(launched) || !run.FinishedAt.Equal(settled) || *run.ExitCode != 1 {
		t.Fatalf("a closed row changed: %+v", run)
	}
}

// Only a record of the row's own release, holding a moment the row keeps,
// reaches it; and a row that already holds the record is not written again.
func TestPreDeployRunForReadsTheRowsOwnRecord(t *testing.T) {
	at := time.Date(2026, 11, 22, 11, 29, 14, 0, time.UTC)
	app := &appv1alpha1.App{}
	if _, ok := preDeployRunFor(app, 2); ok {
		t.Error("no step: nothing to copy")
	}
	app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{Generation: 2, Status: appv1alpha1.PreDeployFailed, Message: "old operator"}
	if _, ok := preDeployRunFor(app, 2); ok {
		t.Error("a record an older operator wrote holds no moment the row keeps")
	}
	app.Status.PreDeploy.LaunchedAt = new(metav1.NewMicroTime(at))
	if _, ok := preDeployRunFor(app, 3); ok {
		t.Error("release 2's record must not reach release 3's row")
	}
	if _, ok := preDeployRunFor(app, 0); ok {
		t.Error("a row with no generation has no record")
	}
	rec, ok := preDeployRunFor(app, 2)
	if !ok || rec.StartedAt == nil || !rec.StartedAt.Equal(at) {
		t.Fatalf("own record = %+v, %v", rec, ok)
	}

	if (DeployPreDeployRun{}).holds(rec) {
		t.Error("an unstamped row holds nothing")
	}
	row := DeployPreDeployRun{StartedAt: &at}
	if !row.holds(rec) {
		t.Error("the same launch must not be written again")
	}
	if row.holds(DeployPreDeployRun{StartedAt: &at, ContainerStartedAt: &at}) {
		t.Error("a container's start must be written")
	}
	if row.holds(DeployPreDeployRun{StartedAt: &at, FinishedAt: &at}) {
		t.Error("an end must be written")
	}
}
