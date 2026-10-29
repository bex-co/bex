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
	"strconv"
	"sync"
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// appWrites observes what the operator would: every App state the API server
// actually stores, in order. Dry-runs store nothing and are not recorded.
type appWrites struct {
	mu     sync.Mutex
	stored []appv1alpha1.App
	// beforePatch, when set, runs ahead of each real App patch — a racing
	// writer the test controls.
	beforePatch func(ctx context.Context, cl client.WithWatch) error
}

func observeAppWrites(svc *Service, inner client.Client) *appWrites {
	w := &appWrites{}
	record := func(ctx context.Context, cl client.WithWatch, obj client.Object) {
		a, ok := obj.(*appv1alpha1.App)
		if !ok {
			return
		}
		var stored appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKeyFromObject(a), &stored); err == nil {
			w.mu.Lock()
			w.stored = append(w.stored, stored)
			w.mu.Unlock()
		}
	}
	svc.Client = interceptor.NewClient(inner.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			var po client.PatchOptions
			po.ApplyOptions(opts)
			if _, ok := obj.(*appv1alpha1.App); ok && len(po.DryRun) == 0 && w.beforePatch != nil {
				if err := w.beforePatch(ctx, cl); err != nil {
					return err
				}
			}
			if err := cl.Patch(ctx, obj, patch, opts...); err != nil {
				return err
			}
			if len(po.DryRun) == 0 {
				record(ctx, cl, obj)
			}
			return nil
		},
		Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			var uo client.UpdateOptions
			uo.ApplyOptions(opts)
			if err := cl.Update(ctx, obj, opts...); err != nil {
				return err
			}
			if len(uo.DryRun) == 0 {
				record(ctx, cl, obj)
			}
			return nil
		},
	})
	return w
}

func (w *appWrites) states() []appv1alpha1.App {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]appv1alpha1.App(nil), w.stored...)
}

// specChanges is the stored states that moved the spec — the ones the operator
// can release. A metadata-only write (a touch, the pending-source marker) is
// not one of them.
func (w *appWrites) specChanges(from *appv1alpha1.App) []appv1alpha1.App {
	var changed []appv1alpha1.App
	prev := from.Spec
	for _, a := range w.states() {
		if !apiequality.Semantic.DeepEqual(prev, a.Spec) {
			changed = append(changed, a)
		}
		prev = a.Spec
	}
	return changed
}

// preDeployApp is a managed web service with a health check and a pre-deploy
// command — the fixture the w8/m56 live repro created.
func preDeployApp() *appv1alpha1.App {
	a := managedRepoApp("web")
	a.Spec.HealthCheckPath = "/healthz"
	a.Spec.PreDeployCommand = "printf marker-old"
	return a
}

// TestCompoundClearNeverStoresTheRemovedCommandBesideTheOtherChange is w8/m56:
// `services update --pre-deploy-command ” --health-check-path ”` stored
// "health cleared, old pre-deploy command" first, the operator released it,
// and the release ran the command the same request removed. Every App state
// the API server stores must be the complete requested configuration.
func TestCompoundClearNeverStoresTheRemovedCommandBesideTheOtherChange(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, preDeployApp())
	writes := observeAppWrites(svc, cl)
	empty := ""
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		HealthCheckPath: &empty, PreDeployCommand: &empty,
	}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	stored := writes.specChanges(preDeployApp())
	for i, a := range stored {
		if a.Spec.PreDeployCommand != "" || a.Spec.HealthCheckPath != "" {
			t.Errorf("stored state %d: health=%q preDeploy=%q — a partial configuration the operator can release",
				i, a.Spec.HealthCheckPath, a.Spec.PreDeployCommand)
		}
	}
	if len(stored) != 1 {
		t.Fatalf("the patch stored %d spec changes, want exactly 1", len(stored))
	}
	// Render's Update Service saves only (w8/m135): the change is held for
	// the next deploy under the generation the publish produced.
	assertHeldForNextDeploy(t, &stored[0], "5")
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %+v, want none: a settings PATCH deploys nothing", st.deployCalls)
	}
}

// assertHeldForNextDeploy checks a stored App holds its release change for
// the next deploy at pending, without stamping a release of its own.
func assertHeldForNextDeploy(t *testing.T, a *appv1alpha1.App, pending string) {
	t.Helper()
	if got := a.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; got != pending {
		t.Errorf("pending generation = %q, want %s: the change waits for the next deploy", got, pending)
	}
	if got, ok := a.Annotations[appv1alpha1.AnnotationReleaseGeneration]; ok {
		t.Errorf("release generation = %q, want none: a save-only change stamps no release", got)
	}
}

// TestSingleFieldClearKeepsTheOtherSetting is the control the live repro ran:
// clearing only the pre-deploy command leaves the health check alone.
func TestSingleFieldClearKeepsTheOtherSetting(t *testing.T) {
	svc, cl := newService(&recordingStore{}, preDeployApp())
	writes := observeAppWrites(svc, cl)
	empty := ""
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{PreDeployCommand: &empty}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	stored := writes.specChanges(preDeployApp())
	if len(stored) != 1 || stored[0].Spec.PreDeployCommand != "" || stored[0].Spec.HealthCheckPath != "/healthz" {
		t.Fatalf("stored states = %+v, want one with the command cleared and /healthz kept", stored)
	}
}

// TestWideCompoundPatchIsOneAppWrite: every row the web service accepts writes
// through the stage — a setter that reached the API server directly would
// publish its own partial configuration again.
func TestWideCompoundPatchIsOneAppWrite(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, preDeployApp())
	writes := observeAppWrites(svc, cl)
	display, root, health, pre := "Web", "services/api", "/livez", "printf marker-new"
	build, start, notify := "go build -o app ./cmd", "./app --serve", "notify"
	delay, port := int32(45), int32(9000)
	autoDeploy := false
	allow := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}}
	view, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		DisplayName: &display, MaxShutdownDelaySeconds: &delay, RootDir: &root, AutoDeploy: &autoDeploy,
		HealthCheckPath: &health, PreDeployCommand: &pre, BuildCommand: &build, StartCommand: &start,
		Port: &port, NotifyOnFail: &notify, IPAllowList: &allow,
	})
	if err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	stored := writes.specChanges(preDeployApp())
	if len(stored) != 1 {
		t.Fatalf("an eleven-field patch stored %d spec changes, want exactly 1", len(stored))
	}
	got := stored[0].Spec
	if got.DisplayName != display || got.RootDir != root || got.AutoDeploy || got.HealthCheckPath != health ||
		got.PreDeployCommand != pre || got.BuildCommand != build || got.StartCommand != start || got.Port != port ||
		got.MaxShutdownDelaySeconds == nil || *got.MaxShutdownDelaySeconds != delay || len(got.IPAllowListEntries) != 1 {
		t.Fatalf("published spec = %+v, want every requested field", got)
	}
	assertHeldForNextDeploy(t, &stored[0], "5")
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %d, want none: a settings PATCH deploys nothing", len(st.deployCalls))
	}
	if view.HealthCheckPath != health || view.StartCommand != start {
		t.Errorf("answered view = health %q start %q, want the published configuration", view.HealthCheckPath, view.StartCommand)
	}
}

// TestCompoundPatchRebasesOnALostRace: a writer that lands between the read and
// the publish keeps its change, the request keeps its own, and the release
// stamp and history row name the generation the publish actually produced.
func TestCompoundPatchRebasesOnALostRace(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, preDeployApp())
	writes := observeAppWrites(svc, cl)
	raced := false
	writes.beforePatch = func(ctx context.Context, inner client.WithWatch) error {
		if raced {
			return nil
		}
		raced = true
		var live appv1alpha1.App
		if err := inner.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &live); err != nil {
			return err
		}
		base := client.MergeFrom(live.DeepCopy())
		live.Spec.Replicas = 3
		live.Generation = 7 // what a real API server's intervening writes leave
		return inner.Patch(ctx, &live, base)
	}
	empty := ""
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		HealthCheckPath: &empty, PreDeployCommand: &empty,
	}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	final := getApp(t, cl, "web")
	if final.Spec.Replicas != 3 || final.Spec.HealthCheckPath != "" || final.Spec.PreDeployCommand != "" {
		t.Fatalf("final spec = %+v, want the racing scale kept and both settings cleared", final.Spec)
	}
	assertHeldForNextDeploy(t, final, strconv.Itoa(8))
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %+v, want none", st.deployCalls)
	}
}

// TestCompoundPatchWithPlanDeploysEveryHeldChange: a plan change still
// deploys at once, as on Render, and the one release it opens carries the
// settings the same request saved — the stamp reaches the pending generation,
// so nothing stays held behind it (w8/m135).
func TestCompoundPatchWithPlanDeploysEveryHeldChange(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, preDeployApp())
	writes := observeAppWrites(svc, cl)
	plan, health := "standard", "/livez"
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{Plan: &plan, HealthCheckPath: &health}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	stored := writes.specChanges(preDeployApp())
	if len(stored) != 1 || stored[0].Spec.HealthCheckPath != health {
		t.Fatalf("stored spec changes = %+v, want one carrying the plan and the health path", stored)
	}
	release, _ := strconv.ParseInt(stored[0].Annotations[appv1alpha1.AnnotationReleaseGeneration], 10, 64)
	pending, _ := strconv.ParseInt(stored[0].Annotations[appv1alpha1.AnnotationPendingSourceGeneration], 10, 64)
	if release != 5 || pending > release {
		t.Errorf("release = %d pending = %d, want release 5 covering every held change", release, pending)
	}
	if len(st.deployCalls) != 1 || st.deployCalls[0].Generation != 5 || st.deployCalls[0].Trigger != store.TriggerSettingsChange {
		t.Fatalf("deploy rows = %+v, want one settings_change row filed under generation 5", st.deployCalls)
	}
}

// TestCompoundPatchRacedBySuspendIsHeld: a suspend that lands between the
// staged setters and the publish must hold the change for the next deploy.
// Stamping the release with no row would let Resume roll it unrecorded
// (w4/m234, w5/m114).
func TestCompoundPatchRacedBySuspendIsHeld(t *testing.T) {
	st := &recordingStore{}
	app := preDeployApp()
	svc, cl := newService(st, app)
	writes := observeAppWrites(svc, cl)
	raced := false
	writes.beforePatch = func(ctx context.Context, inner client.WithWatch) error {
		if raced {
			return nil
		}
		raced = true
		var live appv1alpha1.App
		if err := inner.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &live); err != nil {
			return err
		}
		base := client.MergeFrom(live.DeepCopy())
		live.Spec.Suspended = true
		live.Generation = 7
		return inner.Patch(ctx, &live, base)
	}
	empty := ""
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		HealthCheckPath: &empty, PreDeployCommand: &empty,
	}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	final := getApp(t, cl, "web")
	if !final.Spec.Suspended || final.Spec.HealthCheckPath != "" || final.Spec.PreDeployCommand != "" {
		t.Fatalf("final spec = %+v, want suspended with both settings cleared", final.Spec)
	}
	if got, want := final.Annotations[appv1alpha1.AnnotationReleaseGeneration], app.Annotations[appv1alpha1.AnnotationReleaseGeneration]; got != want {
		t.Errorf("release generation = %q, want the prior %q: nothing deploys while suspended", got, want)
	}
	if got := final.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; got != "8" {
		t.Errorf("pending generation = %q, want 8: the change waits for the next deploy", got)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %+v, want none for a suspended service", st.deployCalls)
	}
}

// TestCompoundPatchStopsAfterRepeatedConflicts: a publish that keeps losing
// answers a retryable conflict rather than overwriting the winner.
func TestCompoundPatchStopsAfterRepeatedConflicts(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, preDeployApp())
	writes := observeAppWrites(svc, cl)
	writes.beforePatch = func(context.Context, client.WithWatch) error {
		return apierrors.NewConflict(schema.GroupResource{Group: "app.bex.co", Resource: "apps"}, "web", nil)
	}
	empty := ""
	_, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		HealthCheckPath: &empty, PreDeployCommand: &empty,
	})
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("err = %v, want core.ErrConflict", err)
	}
	if a := getApp(t, cl, "web"); a.Spec.PreDeployCommand == "" || a.Spec.HealthCheckPath == "" {
		t.Fatalf("a refused publish changed the service: %+v", a.Spec)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("a refused publish filed %d deploy rows for a release that never rolled", len(st.deployCalls))
	}
}

// TestSaveOnlySourceInACompoundPatchStaysDeferred: Update Source is saved for
// the next deploy (Render's contract); publishing it with an operational field
// in one write must not turn it into a rollout.
func TestSaveOnlySourceInACompoundPatchStaysDeferred(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, preDeployApp())
	writes := observeAppWrites(svc, cl)
	branch, autoDeploy := "release", false
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{Branch: &branch, AutoDeploy: &autoDeploy}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	// The pending-source marker lands first, on its own: it must be durable
	// before the store's source row, which the projector may project at any
	// moment. It changes no spec.
	all := writes.states()
	if len(all) == 0 || all[0].Annotations[appv1alpha1.AnnotationPendingSourceGeneration] == "" || all[0].Spec.Branch != "main" {
		t.Fatalf("first stored state = %+v, want the pending-source marker on the unchanged spec", all)
	}
	stored := writes.specChanges(preDeployApp())
	if len(stored) != 1 || stored[0].Spec.Branch != branch || stored[0].Spec.AutoDeploy {
		t.Fatalf("stored spec changes = %d, want one carrying the new branch and auto-deploy off", len(stored))
	}
	if got := stored[0].Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; got != "5" {
		t.Errorf("pending source generation = %q, want 5", got)
	}
	if _, ok := stored[0].Annotations[appv1alpha1.AnnotationReleaseGeneration]; ok {
		t.Errorf("a save-only source update stamped a release generation")
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("a save-only source patch opened %d deploy rows, want none", len(st.deployCalls))
	}
}

// TestEveryRowWritesThroughTheStage covers the rows the eleven-field patch
// cannot carry on one service: each case is a compound patch for the kind that
// accepts it, and each must store exactly one App state.
func TestEveryRowWritesThroughTheStage(t *testing.T) {
	str := func(v string) *string { return &v }
	i32 := func(v int32) *int32 { return &v }
	cron := func() *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Spec.Type = appv1alpha1.TypeCronJob
		a.Spec.Schedule = "0 * * * *"
		a.Spec.Command = "./job"
		a.Spec.Replicas = 0
		return a
	}
	static := func() *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Spec.Type = appv1alpha1.TypeStaticSite
		a.Spec.Replicas = 0
		a.Spec.PublishPath = "dist"
		return a
	}
	cases := []struct {
		name  string
		app   *appv1alpha1.App
		patch ServicePatch
	}{
		{"plan, maintenance, autoscaling, subdomain, filter, notifications", preDeployApp(), ServicePatch{
			Plan: str("standard"), MaintenanceMode: &MaintenanceModeView{Enabled: true},
			Autoscaling:           &SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3, TargetCPUPercent: i32(70)},
			RenderSubdomainPolicy: str("enabled"), BuildFilter: &BuildFilterView{Paths: []string{"src/**"}},
			NotificationsToSend: str("all"), HealthCheckPath: str(""), PreDeployCommand: str(""),
		}},
		{"source with build settings", func() *appv1alpha1.App {
			a := preDeployApp()
			a.Spec.Runtime = "docker"
			return a
		}(), ServicePatch{
			Branch: str("release"), DockerfilePath: str("./Dockerfile"), PreDeployCommand: str(""),
		}},
		{"free downgrade disabling maintenance first", func() *appv1alpha1.App {
			a := preDeployApp()
			a.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
			a.Spec.Replicas = 1
			return a
		}(), ServicePatch{
			MaintenanceBeforeFreeDowngrade: true, PreDeployBeforeFreeDowngrade: true, MaintenanceMode: &MaintenanceModeView{}, Plan: str("free"),
			IdleTTLSeconds: i32(600), PreDeployCommand: str(""),
		}},
		{"cron schedule and command", cron(), ServicePatch{Schedule: str("*/5 * * * *"), Command: str("./job --all"), BuildCommand: str("make")}},
		{"static publish path", static(), ServicePatch{PublishPath: str("public"), BuildCommand: str("npm run build")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(&recordingStore{}, tc.app)
			writes := observeAppWrites(svc, cl)
			if _, err := svc.ApplyServicePatch(context.Background(), "web", tc.patch); err != nil {
				t.Fatalf("ApplyServicePatch: %v", err)
			}
			if n := len(writes.specChanges(tc.app)); n != 1 {
				t.Fatalf("stored %d spec changes, want exactly 1", n)
			}
		})
	}
}

// portFailingStore fails only the port row's store write — an unexpected
// failure past the preflight, partway through the table.
type portFailingStore struct{ *recordingStore }

func (portFailingStore) SetAppPort(context.Context, string, int32, string) error {
	return errors.New("ledger down")
}

// TestFailurePartwayStillPublishesTheRowsBeforeIt: the preflight closes the
// deterministic refusals, not a mid-table store outage. The rows ahead of the
// failure already recorded their effects, so they are published — still as one
// write — and the request answers the failure (w4/123's truthful partial).
func TestFailurePartwayStillPublishesTheRowsBeforeIt(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(portFailingStore{st}, preDeployApp())
	writes := observeAppWrites(svc, cl)
	empty, port := "", int32(9000)
	_, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		HealthCheckPath: &empty, PreDeployCommand: &empty, Port: &port,
	})
	if err == nil {
		t.Fatal("a failed port row answered success")
	}
	stored := writes.specChanges(preDeployApp())
	if len(stored) != 1 || stored[0].Spec.HealthCheckPath != "" || stored[0].Spec.PreDeployCommand != "" || stored[0].Spec.Port == port {
		t.Fatalf("stored spec changes = %+v, want one carrying both clears and not the failed port", stored)
	}
	assertHeldForNextDeploy(t, &stored[0], "5")
	if len(st.deployCalls) != 0 {
		t.Fatalf("deploy rows = %d, want none: the published rows are held for the next deploy", len(st.deployCalls))
	}
}
