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

// TestSettingsVerbsOpenDeployHistory is w6/m51's core guarantee: a Settings
// edit that forces the operator to rebuild or roll new pods opens the same
// deploy-history row an explicit deploy does. Before this, `setStartCommand`
// persisted, the service genuinely rebuilt, and the Deploys tab showed nothing.
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
			if d.Trigger != store.TriggerConfigChange {
				t.Errorf("trigger = %q, want %q", d.Trigger, store.TriggerConfigChange)
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

// TestSurfacesAgreeOnDeployHistory is w6/m51's Render-parity check (t004): the
// same edit must be equally visible whichever surface made it. GraphQL drives
// each field through its own mutation; REST PATCH and the update_service MCP
// tool share one ordered op table. All three land on the same Service verbs and
// must produce the same, single, visible deploy.
func TestSurfacesAgreeOnDeployHistory(t *testing.T) {
	ctx := context.Background()
	start := "./app --serve"
	surfaces := []struct {
		name string
		edit func(svc *Service) error
	}{
		{"graphql setStartCommand", func(svc *Service) error {
			_, err := svc.SetCommands(ctx, "web", nil, &start)
			return err
		}},
		{"rest/mcp servicePatch", func(svc *Service) error {
			_, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{StartCommand: &start})
			return err
		}},
	}
	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			st := &recordingStore{}
			svc, _ := newService(st, managedRepoApp("web"))
			if err := surface.edit(svc); err != nil {
				t.Fatalf("%s: %v", surface.name, err)
			}
			if len(st.deployCalls) != 1 || st.deployCalls[0].Trigger != store.TriggerConfigChange {
				t.Fatalf("%s produced %d rows %+v, want exactly one config_change deploy",
					surface.name, len(st.deployCalls), st.deployCalls)
			}
		})
	}
}

// TestMultiFieldPatchIsOneDeploy: a Settings save that changes several fields is
// one rollout. Without batching, the op table's per-field setters would each
// open a row and the history would show one deploy per field, all but the last
// immediately canceled.
func TestMultiFieldPatchIsOneDeploy(t *testing.T) {
	st := &recordingStore{}
	svc, _ := newService(st, managedRepoApp("web"))
	start, build, health := "./app --serve", "go build -o app ./cmd", "/healthz"
	display := "Web"
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		StartCommand: &start, BuildCommand: &build, HealthCheckPath: &health, DisplayName: &display,
	}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	if len(st.deployCalls) != 1 {
		t.Fatalf("a four-field patch opened %d deploy rows, want exactly 1", len(st.deployCalls))
	}
	// The row must name the release the LAST setter produced — the one the
	// operator will reconcile — or the reconciler never closes it. (The fake
	// client does not increment metadata.generation, so the monotonic fallback
	// is what is observable here; a real API server advances it per patch.)
	if got := st.deployCalls[0].Generation; got <= 4 {
		t.Fatalf("generation = %d, want the post-patch release generation (> 4)", got)
	}
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
