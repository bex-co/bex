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
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// managedRepoApp is a store-managed, repo-backed service — the shape every
// Settings verb below is exercised against. Only a managed App has a
// control-plane row, and therefore deploy history to keep.
func managedRepoApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				store.LabelManagedBy: store.ManagedByValue,
				store.LabelAppID:     "srv-test",
			},
			Generation: 4,
		},
		Spec: appv1alpha1.AppSpec{
			Repo:         "https://github.com/bex-co/hello.git",
			Branch:       "main",
			Runtime:      "go",
			BuildCommand: "go build -o app .",
			StartCommand: "./app",
			Tier:         "starter",
			Replicas:     2,
		},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning},
	}
}

// TestSettingsVerbsOpenDeployHistory is w6/m51's core guarantee for the
// save-and-deploy verbs (the dashboard's GraphQL Settings saves): a Settings
// edit that forces the operator to rebuild or roll new pods opens the same
// deploy-history row an explicit deploy does, as settings_change (w8/m135).
// Before w6/m51, `setStartCommand` persisted, the service genuinely rebuilt,
// and the Deploys tab showed nothing.
func TestSettingsVerbsOpenDeployHistory(t *testing.T) {
	ctx := context.Background()
	start := "./app --serve"
	build := "go build -tags prod -o app ."
	cases := []struct {
		name string
		verb func(svc *Service) error
	}{
		{"start command", func(svc *Service) error {
			_, err := svc.SetCommands(ctx, "web", nil, &start)
			return err
		}},
		{"build command", func(svc *Service) error {
			_, err := svc.SetCommands(ctx, "web", &build, nil)
			return err
		}},
		{"root dir", func(svc *Service) error {
			_, err := svc.SetRootDir(ctx, "web", "services/api")
			return err
		}},
		{"pre-deploy command", func(svc *Service) error {
			_, err := svc.SetPreDeployCommand(ctx, "web", "./migrate")
			return err
		}},
		{"health check path", func(svc *Service) error {
			_, err := svc.SetHealthCheckPath(ctx, "web", "/healthz")
			return err
		}},
		{"max shutdown delay", func(svc *Service) error {
			_, err := svc.SetMaxShutdownDelay(ctx, "web", 45)
			return err
		}},
		{"plan", func(svc *Service) error {
			_, err := svc.SetPlan(ctx, "web", "standard")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := &recordingStore{}
			svc, _ := newService(st, managedRepoApp("web"))
			if err := tc.verb(svc); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if len(st.deployCalls) != 1 {
				t.Fatalf("deploy rows = %d, want exactly 1 for a rollout-forcing edit", len(st.deployCalls))
			}
			d := st.deployCalls[0]
			if d.Trigger != store.TriggerSettingsChange {
				t.Errorf("trigger = %q, want %q", d.Trigger, store.TriggerSettingsChange)
			}
			if d.AppID != "srv-test" {
				t.Errorf("appID = %q, want the control-plane row id", d.AppID)
			}
			// The row must name the release the operator will reconcile, not the
			// one it just left, or the reconciler cannot close it.
			if d.Generation <= 4 {
				t.Errorf("generation = %d, want the post-patch release generation (> 4)", d.Generation)
			}
		})
	}
}

// TestSettingsVerbStampsReleaseGeneration pins the annotation that keeps the
// deploy row and the operator's release decision on the same generation when an
// operational mutation races in between (the same guard deploys.Trigger uses).
func TestSettingsVerbStampsReleaseGeneration(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, managedRepoApp("web"))
	start := "./app --serve"
	if _, err := svc.SetCommands(context.Background(), "web", nil, &start); err != nil {
		t.Fatalf("SetCommands: %v", err)
	}
	a := getApp(t, cl, "web")
	stamped := a.Annotations[appv1alpha1.AnnotationReleaseGeneration]
	if stamped == "" {
		t.Fatal("a tracked rollout must stamp the release generation the row was filed under")
	}
	if got := st.deployCalls[0].Generation; stamped != "5" || got != 5 {
		t.Fatalf("annotation=%q row generation=%d, want both to name generation 5", stamped, got)
	}
}

// TestOperationalVerbsOpenNoDeploy is the other direction, and the reason this
// milestone gates on the operator's identity classification instead of "any
// spec change": scale, autoDeploy, idle TTL and display name are reconciled
// against the RUNNING release, so minting a deploy for them would fill the
// history with rollouts that never happened.
func TestOperationalVerbsOpenNoDeploy(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		verb func(svc *Service) error
	}{
		{"scale", func(svc *Service) error { _, err := svc.Scale(ctx, "web", 3); return err }},
		{"idle ttl", func(svc *Service) error { _, err := svc.SetIdleTTL(ctx, "web", 300); return err }},
		{"display name", func(svc *Service) error { _, err := svc.SetDisplayName(ctx, "web", "Web"); return err }},
		{"auto deploy", func(svc *Service) error { _, err := svc.SetAutoDeploy(ctx, "web", false); return err }},
		{"notify on fail", func(svc *Service) error { _, err := svc.SetNotifyOnFail(ctx, "web", "default"); return err }},
		{"suspend", func(svc *Service) error { _, err := svc.Suspend(ctx, "web"); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := &recordingStore{}
			svc, _ := newService(st, managedRepoApp("web"))
			if err := tc.verb(svc); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if len(st.deployCalls) != 0 {
				t.Fatalf("deploy rows = %d, want none for an operational change", len(st.deployCalls))
			}
		})
	}
}

// TestSourceUpdateDefersDeploy covers the exceptional release-field contract:
// Update Source changes future release intent but does not itself deploy. The
// pending marker lets the operator preserve the active release until a deploy
// verb consumes the new source.
func TestSourceUpdateDefersDeploy(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, managedRepoApp("web"))
	repo := "https://github.com/bex-co/other.git"
	if _, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", sourcePatch{Repo: &repo}); err != nil {
		t.Fatalf("SetSourceAndRegistryCredential: %v", err)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %d, want none for a deferred source update", len(st.deployCalls))
	}
	app := getApp(t, cl, "web")
	if app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] == "" {
		t.Fatal("source update did not mark its generation pending")
	}
}

// TestUnchangedSettingsValueOpensNoDeploy: re-saving the value a service
// already has changes no spec field, so it is not a rollout and must not cost
// the user a deploy — Settings pages re-submit the whole form.
func TestUnchangedSettingsValueOpensNoDeploy(t *testing.T) {
	st := &recordingStore{}
	svc, _ := newService(st, managedRepoApp("web"))
	same := "./app"
	if _, err := svc.SetCommands(context.Background(), "web", nil, &same); err != nil {
		t.Fatalf("SetCommands: %v", err)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %d, want none when the value did not change", len(st.deployCalls))
	}
}

// TestUnmanagedAppOpensNoDeploy: a hand-applied App CR has no control-plane row
// and so no deploy history to keep. Tracking must degrade, not fail the verb.
func TestUnmanagedAppOpensNoDeploy(t *testing.T) {
	st := &recordingStore{}
	a := managedRepoApp("web")
	a.Labels = nil
	svc, _ := newService(st, a)
	start := "./app --serve"
	if _, err := svc.SetCommands(context.Background(), "web", nil, &start); err != nil {
		t.Fatalf("SetCommands: %v", err)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %d, want none for an App with no control-plane row", len(st.deployCalls))
	}
}

// TestDeployRowFailureDoesNotFailTheEdit: the spec patch has already landed and
// the rebuild is already rolling by the time the row is written, so a store
// failure is logged, not reported as a failed edit the user would retry.
func TestDeployRowFailureDoesNotFailTheEdit(t *testing.T) {
	st := &recordingStore{err: errors.New("control plane unavailable")}
	svc, cl := newService(st, managedRepoApp("web"))
	start := "./app --serve"
	if _, err := svc.SetCommands(context.Background(), "web", nil, &start); err != nil {
		t.Fatalf("SetCommands must survive a deploy-row write failure: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.StartCommand; got != start {
		t.Fatalf("startCommand = %q, want the edit to have persisted", got)
	}
}

// TestSettingsPatchCarriesPriorCommit is w4/m100 t005: apps.patchFetched must
// open a settings_change row that inherits the running release's commit.
func TestSettingsPatchCarriesPriorCommit(t *testing.T) {
	prior := store.CommitInfo{Hash: "5ef5e18799fa7edbc6477ca3128c78686833b06b", Message: "update"}
	st := &recordingStore{priorCommit: prior}
	svc, _ := newService(st, managedRepoApp("web"))
	start := "./app --serve"
	if _, err := svc.SetCommands(context.Background(), "web", nil, &start); err != nil {
		t.Fatalf("SetCommands: %v", err)
	}
	if len(st.deployCalls) != 1 {
		t.Fatalf("deploy rows = %d, want 1", len(st.deployCalls))
	}
	got := st.deployCalls[0]
	if got.Trigger != store.TriggerSettingsChange {
		t.Errorf("trigger = %q, want settings_change", got.Trigger)
	}
	if got.Commit != prior.Hash || got.CommitMessage != prior.Message {
		t.Fatalf("commit = %q/%q, want prior %q/%q", got.Commit, got.CommitMessage, prior.Hash, prior.Message)
	}
}

// TestSurfaceSaveSemantics pins each surface's save mode (w8/m135). Render's
// Update Service API saves and deploys nothing ("you must call the deploy
// API"), so REST PATCH and the update_service MCP tool — one ordered op table
// — hold the change for the next deploy. The dashboard's GraphQL Settings
// save is an explicit save-and-deploy: one visible settings_change deploy.
func TestSurfaceSaveSemantics(t *testing.T) {
	ctx := context.Background()
	start := "./app --serve"

	t.Run("graphql setStartCommand saves and deploys", func(t *testing.T) {
		st := &recordingStore{}
		svc, _ := newService(st, managedRepoApp("web"))
		if _, err := svc.SetCommands(ctx, "web", nil, &start); err != nil {
			t.Fatalf("SetCommands: %v", err)
		}
		if len(st.deployCalls) != 1 || st.deployCalls[0].Trigger != store.TriggerSettingsChange {
			t.Fatalf("produced %d rows %+v, want exactly one settings_change deploy", len(st.deployCalls), st.deployCalls)
		}
	})
	t.Run("rest/mcp servicePatch saves only", func(t *testing.T) {
		st := &recordingStore{}
		svc, cl := newService(st, managedRepoApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{StartCommand: &start}); err != nil {
			t.Fatalf("ApplyServicePatch: %v", err)
		}
		if len(st.deployCalls) != 0 {
			t.Fatalf("produced %d rows %+v, want none", len(st.deployCalls), st.deployCalls)
		}
		a := getApp(t, cl, "web")
		if a.Spec.StartCommand != start {
			t.Fatalf("startCommand = %q, want the saved %q", a.Spec.StartCommand, start)
		}
		assertHeldForNextDeploy(t, a, "5")
	})
}

// TestServicePatchHoldsEveryReleaseField is w8/m135 t001 per field: each
// release-relevant setting a Render Update Service PATCH carries is saved,
// held for the next deploy, and opens no deploy row — the sweep-6e1b repro,
// where each of these PATCHes started a deploy of its own.
func TestServicePatchHoldsEveryReleaseField(t *testing.T) {
	str := func(v string) *string { return &v }
	i32 := func(v int32) *int32 { return &v }
	docker := func() *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Spec.Runtime, a.Spec.BuildCommand, a.Spec.StartCommand = "docker", "", ""
		return a
	}
	image := func() *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Spec.Repo, a.Spec.Branch, a.Spec.Runtime, a.Spec.BuildCommand, a.Spec.StartCommand = "", "", "image", "", ""
		a.Spec.Image = "docker.io/ealen/echo-server:0.9.2"
		return a
	}
	cases := []struct {
		name  string
		app   *appv1alpha1.App
		patch ServicePatch
		saved func(appv1alpha1.AppSpec) bool
	}{
		{"healthCheckPath", image(), ServicePatch{HealthCheckPath: str("/hz2")},
			func(s appv1alpha1.AppSpec) bool { return s.HealthCheckPath == "/hz2" }},
		{"maxShutdownDelaySeconds", image(), ServicePatch{MaxShutdownDelaySeconds: i32(40)},
			func(s appv1alpha1.AppSpec) bool { return s.MaxShutdownDelaySeconds != nil && *s.MaxShutdownDelaySeconds == 40 }},
		{"preDeployCommand", image(), ServicePatch{PreDeployCommand: str("echo x")},
			func(s appv1alpha1.AppSpec) bool { return s.PreDeployCommand == "echo x" }},
		{"dockerCommand on an image", image(), ServicePatch{StartCommand: str("node index.js")},
			func(s appv1alpha1.AppSpec) bool { return s.StartCommand == "node index.js" }},
		{"startCommand", managedRepoApp("web"), ServicePatch{StartCommand: str("./app --serve")},
			func(s appv1alpha1.AppSpec) bool { return s.StartCommand == "./app --serve" }},
		{"buildCommand", managedRepoApp("web"), ServicePatch{BuildCommand: str("make")},
			func(s appv1alpha1.AppSpec) bool { return s.BuildCommand == "make" }},
		{"rootDir", managedRepoApp("web"), ServicePatch{RootDir: str("services/api")},
			func(s appv1alpha1.AppSpec) bool { return s.RootDir == "services/api" }},
		{"dockerfilePath", docker(), ServicePatch{DockerfilePath: str("./build/Dockerfile")},
			func(s appv1alpha1.AppSpec) bool { return s.DockerfilePath != "" }},
		{"port", image(), ServicePatch{Port: i32(9000)},
			func(s appv1alpha1.AppSpec) bool { return s.Port == 9000 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := &recordingStore{}
			svc, cl := newService(st, tc.app)
			if _, err := svc.ApplyServicePatch(context.Background(), "web", tc.patch); err != nil {
				t.Fatalf("ApplyServicePatch: %v", err)
			}
			a := getApp(t, cl, "web")
			if !tc.saved(a.Spec) {
				t.Fatalf("spec = %+v, want the %s saved", a.Spec, tc.name)
			}
			if !appv1alpha1.SpecRollsRelease(tc.app.Spec, a.Spec) {
				t.Fatalf("%s is not a release field; the case proves nothing", tc.name)
			}
			assertHeldForNextDeploy(t, a, "5")
			if len(st.deployCalls) != 0 {
				t.Fatalf("deploy rows = %+v, want none: the PATCH saves only", st.deployCalls)
			}
		})
	}
}

// TestMultiFieldPatchHoldsEveryFieldForOneDeploy: a PATCH that changes several
// fields saves all of them under one pending generation and deploys none —
// the next deploy adopts them together instead of rolling one intermediate
// configuration per field.
func TestMultiFieldPatchHoldsEveryFieldForOneDeploy(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, managedRepoApp("web"))
	start, build, health := "./app --serve", "go build -o app ./cmd", "/healthz"
	display := "Web"
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		StartCommand: &start, BuildCommand: &build, HealthCheckPath: &health, DisplayName: &display,
	}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("a four-field patch opened %d deploy rows, want none", len(st.deployCalls))
	}
	a := getApp(t, cl, "web")
	if a.Spec.StartCommand != start || a.Spec.BuildCommand != build || a.Spec.HealthCheckPath != health {
		t.Fatalf("spec = %+v, want every field saved", a.Spec)
	}
	assertHeldForNextDeploy(t, a, "5")
}

// TestOperationalOnlyPatchIsNoDeploy: a patch that touches only operational
// fields rolls nothing, so it opens nothing even though the batch is armed.
func TestOperationalOnlyPatchIsNoDeploy(t *testing.T) {
	st := &recordingStore{}
	svc, _ := newService(st, managedRepoApp("web"))
	display, autoDeploy := "Web", false
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		DisplayName: &display, AutoDeploy: &autoDeploy,
	}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("an operational-only patch opened %d deploy rows, want none", len(st.deployCalls))
	}
}

// TestSourceUpdateClearsTheBuildPin (w5/138): a pinned commit belongs to the
// repository and branch it was resolved from. Update Source is saved for the
// next deploy, and a configuration rollout counts as one, so a pin left behind
// would build the old commit against the new branch, or fail to fetch it from
// the new repository.
func TestSourceUpdateClearsTheBuildPin(t *testing.T) {
	repo, branch := "https://github.com/bex-co/other.git", "release"
	for name, patch := range map[string]sourcePatch{"repository": {Repo: &repo}, "branch": {Branch: &branch}} {
		t.Run(name, func(t *testing.T) {
			pinned := managedRepoApp("web")
			pinned.Spec.BuildCommit = "1111111111111111111111111111111111111111"
			svc, cl := newService(&recordingStore{}, pinned)
			if _, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", patch); err != nil {
				t.Fatalf("SetSourceAndRegistryCredential: %v", err)
			}
			if got := getApp(t, cl, "web").Spec.BuildCommit; got != "" {
				t.Errorf("spec.buildCommit = %q after a %s change, want it cleared", got, name)
			}
		})
	}
}
