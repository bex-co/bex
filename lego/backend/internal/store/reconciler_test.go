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
	"fmt"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func newTestReconciler(t *testing.T) (*Reconciler, *memStore, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appv1alpha1.App{}).Build()
	store := newMemStore()
	return NewReconciler(cl, store, "default"), store, cl
}

func TestObservedImagePullFailureIsImmediateAndGenerationScoped(t *testing.T) {
	at := metav1.NewTime(time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC))
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 7},
		Spec:       appv1alpha1.AppSpec{Image: "registry.invalid/missing:never"},
		Status: appv1alpha1.AppStatus{
			ReleaseGeneration: 7,
			Conditions: []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionFalse, Reason: "ImagePullBackOff",
				ObservedGeneration: 7, LastTransitionTime: at,
			}},
		},
	}
	open := Deploy{ID: "dep-image", AppID: "srv-image", Image: app.Spec.Image, Generation: 7}
	fact, ok := observedImagePullFailure(open, app)
	if !ok || fact.SourceKey != "deploy:dep-image:image_pull_failed" || fact.Type != EventFactImagePullFailed ||
		fact.DeployID != open.ID || fact.Image != open.Image || fact.ReasonCode != EventReasonImagePullBackoff || !fact.At.Equal(at.Time) {
		t.Fatalf("image-pull fact = %+v, %v", fact, ok)
	}
	open.Generation = 6
	if _, ok := observedImagePullFailure(open, app); ok {
		t.Fatal("stale release generation emitted an image-pull fact")
	}
}

// TestBuildEndedStatus pins the build-outcome table (w7/m66): where the deploy
// has reached decides whether the build is over and with what status.
func TestBuildEndedStatus(t *testing.T) {
	cases := []struct{ name, current, next, want string }{
		{"created, still building", DeployCreated, "", ""},
		{"queued, still building", DeployQueued, "", ""},
		{"build in progress", DeployBuildInProgress, "", ""},
		{"build failed", DeployBuildInProgress, DeployBuildFailed, EventStatusFailed},
		{"advanced to pre-deploy", DeployBuildInProgress, DeployPreDeployInProgress, EventStatusSucceeded},
		{"advanced to update", DeployQueued, DeployUpdateInProgress, EventStatusSucceeded},
		{"fast path straight to live", DeployCreated, DeployLive, EventStatusSucceeded},
		{"pre-deploy failure ⇒ build still succeeded", DeployBuildInProgress, DeployPreDeployFailed, EventStatusSucceeded},
		{"update failure ⇒ build still succeeded", DeployUpdateInProgress, DeployUpdateFailed, EventStatusSucceeded},
		{"canceled during build", DeployBuildInProgress, DeployCanceled, EventStatusCanceled},
		{"canceled after build", DeployUpdateInProgress, DeployCanceled, EventStatusSucceeded},
	}
	for _, c := range cases {
		if got := buildEndedStatus(c.current, c.next); got != c.want {
			t.Errorf("%s: buildEndedStatus(%q, %q) = %q, want %q", c.name, c.current, c.next, got, c.want)
		}
	}
}

// TestBuildLifecycleFacts proves build_started rides the deploy's creation time
// and build_ended appears (with an outcome) only once the build is over.
func TestBuildLifecycleFacts(t *testing.T) {
	created := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	open := Deploy{ID: "dep-1", AppID: "srv-1", Image: "reg/x:1", Status: DeployBuildInProgress, CreatedAt: created}

	building := buildLifecycleFacts(open, "")
	if len(building) != 1 || building[0].Type != EventFactBuildStarted ||
		building[0].SourceKey != "deploy:dep-1:build_started" || !building[0].At.Equal(created) ||
		building[0].Image != "reg/x:1" {
		t.Fatalf("still-building facts = %+v, want a single build_started at created_at", building)
	}

	failed := buildLifecycleFacts(open, DeployBuildFailed)
	if len(failed) != 2 || failed[1].Type != EventFactBuildEnded ||
		failed[1].SourceKey != "deploy:dep-1:build_ended" || failed[1].Status != EventStatusFailed {
		t.Fatalf("failed-build facts = %+v, want build_started + build_ended(failed)", failed)
	}
}

// TestPreDeployLifecycleFacts proves the pair rides status.preDeploy: nothing
// without a step, started+ended (with the CR's stamps and outcome) with one.
func TestPreDeployLifecycleFacts(t *testing.T) {
	open := Deploy{ID: "dep-1", AppID: "srv-1"}

	if facts := preDeployLifecycleFacts(open, &appv1alpha1.App{}); facts != nil {
		t.Fatalf("no pre-deploy step should emit nothing, got %+v", facts)
	}

	started, finished := "2026-07-31T10:00:00Z", "2026-07-31T10:01:00Z"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
		Status: appv1alpha1.AppStatus{
			ReleaseGeneration: 3,
			PreDeploy: &appv1alpha1.PreDeployStatus{
				Generation: 3, Status: appv1alpha1.PreDeploySucceeded,
				StartedAt: started, FinishedAt: finished,
			},
		},
	}
	facts := preDeployLifecycleFacts(open, app)
	if len(facts) != 2 || facts[0].Type != EventFactPreDeployStarted ||
		facts[1].Type != EventFactPreDeployEnded || facts[1].Status != EventStatusSucceeded {
		t.Fatalf("succeeded pre-deploy facts = %+v, want started + ended(succeeded)", facts)
	}
	wantStart, _ := time.Parse(time.RFC3339, started)
	wantFinish, _ := time.Parse(time.RFC3339, finished)
	if !facts[0].At.Equal(wantStart) || !facts[1].At.Equal(wantFinish) {
		t.Errorf("pre-deploy fact times = %s / %s, want %s / %s", facts[0].At, facts[1].At, wantStart, wantFinish)
	}
}

func TestObservedServiceStateTreatsConcreteOpenDeployFailureAsInstanceFailure(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 4},
		Status: appv1alpha1.AppStatus{
			Phase:          appv1alpha1.PhaseDeploying,
			ActiveRevision: "rev-3",
			Conditions: []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionFalse, Reason: "ImagePullBackOff",
				ObservedGeneration: 4,
			}},
		},
	}
	obs := observedServiceStateFor("srv-image", app, true)
	if !obs.AvailabilityObserved || obs.Availability != "unhealthy" || obs.ReasonCode != EventReasonReadinessFailed {
		t.Fatalf("image-pull observation = %+v, want unhealthy instance", obs)
	}
	app.Status.Conditions[0].Reason = "Deploying"
	obs = observedServiceStateFor("srv-image", app, true)
	if obs.AvailabilityObserved {
		t.Fatalf("ordinary rollout progress = %+v, must not be an instance failure", obs)
	}
}

// getApp fetches the one public-name "web" CR every test projects. The object
// name intentionally uses the immutable tenant id, not the mutable tenant name.
func getApp(t *testing.T, cl client.Client) *appv1alpha1.App {
	t.Helper()
	var apps appv1alpha1.AppList
	if err := cl.List(context.Background(), &apps, client.InNamespace("default"), client.MatchingLabels{core.LabelServiceName: "web"}); err != nil {
		t.Fatalf("list App web: %v", err)
	}
	if len(apps.Items) != 1 {
		t.Fatalf("App web count = %d, want 1", len(apps.Items))
	}
	return &apps.Items[0]
}

func TestReconcileCreatesAppCR(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Image: "traefik/whoami", Branch: "main",
		Port: 80, Replicas: 2, Tier: "starter",
	})

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	app := getApp(t, cl)
	if app.Labels[LabelManagedBy] != ManagedByValue || app.Labels[LabelAppID] != row.ID || app.Labels[LabelTenant] != ten.ID {
		t.Errorf("labels = %v", app.Labels)
	}
	if app.Spec.Image != "traefik/whoami" || app.Spec.Port != 80 || app.Spec.Replicas != 2 ||
		app.Spec.Tier != "starter" || !app.Spec.Expose {
		t.Errorf("spec = %+v", app.Spec)
	}
}

func TestProjectAppUsesTenantIDNotMutableTenantName(t *testing.T) {
	rec := &Reconciler{Namespace: "default"}
	d := DesiredApp{App: App{TenantID: "tea-stable", Name: "web"}, TenantName: "renamed-workspace"}
	app := rec.projectApp(context.Background(), d)
	if app.Name != "tea-stable-web" {
		t.Fatalf("projected name = %q, want stable tenant-id name", app.Name)
	}
	if app.Labels[LabelTenant] != "tea-stable" || app.Labels[LabelAppID] != d.ID {
		t.Fatalf("projected labels = %v", app.Labels)
	}
}

func TestReconcileProjectsExplicitRegistryCredential(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	credentialID := "rgc-private"
	row, _ := store.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Image: "ghcr.io/acme/private:1", Branch: "main",
		RegistryCredentialID: &credentialID, Port: 80, Replicas: 1, Tier: "starter",
	})

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	app := getApp(t, cl)
	if app.Spec.RegistryCredentialID == nil || *app.Spec.RegistryCredentialID != credentialID {
		t.Fatalf("registry credential id = %v, want %q", app.Spec.RegistryCredentialID, credentialID)
	}
	wantPullSecret := core.CRName(ten.ID, "web") + "-registry-pull"
	if app.Spec.ExternalRegistryPullSecret != wantPullSecret {
		t.Errorf("pull secret = %q, want %q", app.Spec.ExternalRegistryPullSecret, wantPullSecret)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	app = getApp(t, cl)
	if app.Spec.ExternalRegistryPullSecret != wantPullSecret {
		t.Errorf("second reconcile cleared pull secret = %q", app.Spec.ExternalRegistryPullSecret)
	}

	empty := ""
	if err := store.SetAppSource(ctx, row.ID, "", "ghcr.io/acme/private:2", "main", &empty); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile clear: %v", err)
	}
	app = getApp(t, cl)
	if app.Spec.RegistryCredentialID == nil || *app.Spec.RegistryCredentialID != "" {
		t.Fatalf("cleared registry credential id = %v, want explicit empty", app.Spec.RegistryCredentialID)
	}
	if app.Spec.ExternalRegistryPullSecret != "" {
		t.Errorf("cleared pull secret reference = %q, want empty", app.Spec.ExternalRegistryPullSecret)
	}
}

func TestReconcileUpdatesOwnedFieldsOnly(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Simulate a field the control plane doesn't own (set by bex-api / defaulting).
	app := getApp(t, cl)
	app.Spec.Builder = "dockerfile"
	app.Spec.RestartedAt = "2026-07-05T00:00:00Z"
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}

	// Change owned state in the DB: scale up + add domains.
	store.mu.Lock()
	a := store.apps[row.ID]
	a.Replicas, a.Image = 3, "img:2"
	store.apps[row.ID] = a
	store.mu.Unlock()
	if _, err := store.CreateDomain(ctx, row.ID, "extra.example.com", false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateDomain(ctx, row.ID, "web.example.com", true); err != nil {
		t.Fatal(err)
	}

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	app = getApp(t, cl)
	if app.Spec.Replicas != 3 || app.Spec.Image != "img:2" {
		t.Errorf("owned fields not updated: %+v", app.Spec)
	}
	if app.Spec.Host != "web.example.com" || len(app.Spec.Hosts) != 1 || app.Spec.Hosts[0] != "extra.example.com" {
		t.Errorf("hosts: host=%q hosts=%v", app.Spec.Host, app.Spec.Hosts)
	}
	if app.Spec.Builder != "dockerfile" || app.Spec.RestartedAt != "2026-07-05T00:00:00Z" {
		t.Errorf("unowned fields stomped: builder=%q restartedAt=%q", app.Spec.Builder, app.Spec.RestartedAt)
	}
}

func TestReconcileStampsWorkspaceLabel(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}) //nolint:errcheck

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	app := getApp(t, cl)
	if got := app.Labels[LabelWorkspace]; got != ten.ID {
		t.Errorf("LabelWorkspace = %q; want %q", got, ten.ID)
	}
	// workspace label must equal the tenant label — same value, two selectors
	if app.Labels[LabelWorkspace] != app.Labels[LabelTenant] {
		t.Errorf("workspace label %q != tenant label %q", app.Labels[LabelWorkspace], app.Labels[LabelTenant])
	}
}

func TestTenantNamespacesProjectAppsIntoWorkspaceNamespace(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	rec.TenantNamespaces = true // t002: project into <ws>, not the shared ns
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}) //nolint:errcheck

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// The App CR must land in the workspace's hosting namespace, not "default".
	var apps appv1alpha1.AppList
	if err := cl.List(ctx, &apps, client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 1 {
		t.Fatalf("App count = %d, want 1", len(apps.Items))
	}
	if got, want := apps.Items[0].Namespace, WorkspaceNamespace(ten.ID); got != want {
		t.Errorf("App namespace = %q, want %q", got, want)
	}

	// A second pass is a stable no-op: the cluster-wide list finds it by app-id
	// and updates in place (no duplicate, no move).
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if err := cl.List(ctx, &apps, client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 1 {
		t.Fatalf("App count after resync = %d, want 1 (no duplicate)", len(apps.Items))
	}
}

func TestSharedNamespaceProjectionIsUnchangedWhenGateOff(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t) // TenantNamespaces defaults false
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}) //nolint:errcheck
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// Byte-identical to pre-m31: the App lands in the shared "default" namespace.
	app := getApp(t, cl)
	if app.Namespace != "default" {
		t.Errorf("App namespace = %q, want default (gate off must be unchanged)", app.Namespace)
	}
}

func TestReconcileWorkspaceLabelSurvivesResync(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	// Simulate an external party removing the workspace label.
	app := getApp(t, cl)
	delete(app.Labels, LabelWorkspace)
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}

	// A resync after a spec change re-stamps the label.
	store.mu.Lock()
	a := store.apps[row.ID]
	a.Image = "img:2"
	store.apps[row.ID] = a
	store.mu.Unlock()

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	app = getApp(t, cl)
	if got := app.Labels[LabelWorkspace]; got != ten.ID {
		t.Errorf("LabelWorkspace after resync = %q; want %q", got, ten.ID)
	}
}

func TestReconcileProjectsServiceAssociationsForRESTClients(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	store.mu.Lock()
	a := store.apps[row.ID]
	a.ProjectID, a.EnvironmentID = "prj-example", "env-staging"
	store.apps[row.ID] = a
	store.mu.Unlock()

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile association labels: %v", err)
	}
	app := getApp(t, cl)
	if app.Labels[core.LabelProject] != "prj-example" || app.Labels[core.LabelEnvironment] != "env-staging" {
		t.Fatalf("association labels = %v", app.Labels)
	}

	store.mu.Lock()
	a = store.apps[row.ID]
	a.ProjectID, a.EnvironmentID = "", ""
	store.apps[row.ID] = a
	store.mu.Unlock()
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("clear association labels: %v", err)
	}
	app = getApp(t, cl)
	if _, ok := app.Labels[core.LabelProject]; ok {
		t.Fatalf("project label survived clear: %v", app.Labels)
	}
	if _, ok := app.Labels[core.LabelEnvironment]; ok {
		t.Fatalf("environment label survived clear: %v", app.Labels)
	}
}

// TestReconcileWorkspaceLabelDistinct verifies two tenants get distinct workspace
// labels — a cross-workspace NetworkPolicy selector must not accidentally match.
func TestReconcileWorkspaceLabelDistinct(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)

	tenA, _ := store.CreateTenant(ctx, "alpha", "free")
	tenB, _ := store.CreateTenant(ctx, "beta", "free")
	store.CreateApp(ctx, App{TenantID: tenA.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})  //nolint:errcheck
	store.CreateApp(ctx, App{TenantID: tenB.ID, Name: "api", Image: "img2", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}) //nolint:errcheck

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var appA, appB appv1alpha1.App
	if err := cl.Get(ctx, types.NamespacedName{Namespace: "default", Name: core.CRName(tenA.ID, "web")}, &appA); err != nil {
		t.Fatalf("get alpha web: %v", err)
	}
	if err := cl.Get(ctx, types.NamespacedName{Namespace: "default", Name: core.CRName(tenB.ID, "api")}, &appB); err != nil {
		t.Fatalf("get beta api: %v", err)
	}
	wsA := appA.Labels[LabelWorkspace]
	wsB := appB.Labels[LabelWorkspace]
	if wsA == "" || wsB == "" {
		t.Errorf("workspace labels missing: A=%q B=%q", wsA, wsB)
	}
	if wsA == wsB {
		t.Errorf("workspace labels must differ: both = %q", wsA)
	}
}

func TestReconcileDeletesRemovedRowsOnly(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	// A hand-applied App (no managed-by label) must never be touched.
	manual := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "hand-applied", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "img"},
	}
	if err := cl.Create(ctx, manual); err != nil {
		t.Fatal(err)
	}

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if err := store.DeleteApp(ctx, row.ID); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var apps appv1alpha1.AppList
	if err := cl.List(ctx, &apps, client.InNamespace("default")); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 1 || apps.Items[0].Name != "hand-applied" {
		names := make([]string, 0, len(apps.Items))
		for _, a := range apps.Items {
			names = append(names, a.Name)
		}
		t.Errorf("want only hand-applied to survive, got %v", names)
	}
}

// TestRecordDeployClosesLiveOnHealthy exercises t001's core acceptance
// criterion: a rollout produces exactly one deploy row that transitions to
// live once the projected App CR reports Running. The reconciler only reads
// this pass's already-observed status (cur.Status), so the test drives it the
// same way the real operator would — a status subresource write between two
// ReconcileOnce passes.
func TestRecordDeployClosesLiveOnHealthy(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	if err := rec.ReconcileOnce(ctx); err != nil { // pass 1: creates the CR, no status yet
		t.Fatalf("reconcile: %v", err)
	}
	if open, ok, _ := openDeployFor(ctx, store, row.ID); !ok || open.Status != DeployCreated {
		t.Fatalf("deploy after create = %+v ok=%v, want one open row", open, ok)
	}

	app := getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseRunning
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil { // pass 2: observes Running, closes the deploy
		t.Fatalf("reconcile: %v", err)
	}

	deploys, err := store.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	if len(deploys) != 1 || deploys[0].Status != DeployLive || deploys[0].FinishedAt == nil {
		t.Fatalf("deploys = %+v, want exactly one, live, with finished_at set", deploys)
	}
}

// TestRecordDeployClosesLiveBackfillsResolvedImage covers w2/m10's t001: the
// deploy row's ResolvedImage is backfilled from the CR's own Status.Image the
// moment it reaches live — the field Rollback later trusts as a restore
// target, since Image alone stays "" for a build-from-git deploy until a
// build resolves it (recordDeploy has no build here, but the write-back path
// is identical either way — see reconciler.go's own comment).
func TestRecordDeployClosesLiveBackfillsResolvedImage(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	app := getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.Image = "img:1@sha256:resolved"
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	deploys, err := store.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 || deploys[0].Status != DeployLive || deploys[0].ResolvedImage != "img:1@sha256:resolved" {
		t.Fatalf("deploys = %+v (err %v), want exactly one, live, with resolved_image backfilled", deploys, err)
	}
}

// TestRecordDeployProjectsPreDeployStatus covers w1/m33: the reconciler projects
// the App CR's status.preDeploy onto the open deploy row so a client can watch
// the pre-deploy step and, crucially, tell a migration failure apart from a
// health-check failure — a failed pre-deploy closes the deploy update_failed
// AND carries pre_deploy_status "failed".
func TestRecordDeployProjectsPreDeployStatus(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	if err := rec.ReconcileOnce(ctx); err != nil { // pass 1: creates the CR + opens a deploy
		t.Fatalf("reconcile: %v", err)
	}

	// The migration is running: status.preDeploy for the CR's current generation.
	app := getApp(t, cl)
	app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
		Job: "predeploy-acme-web-gen-1", Generation: app.Generation,
		Status: appv1alpha1.PreDeployRunning,
	}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil { // projects "running"; deploy stays open
		t.Fatalf("reconcile: %v", err)
	}
	open, ok, _ := openDeployFor(ctx, store, row.ID)
	if !ok || open.PreDeployStatus != PreDeployRunning {
		t.Fatalf("open deploy = %+v ok=%v, want pre_deploy_status=running, still open", open, ok)
	}

	// The migration fails: the CR reaches Failed with status.preDeploy failed.
	app = getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseFailed
	app.Status.PreDeploy.Status = appv1alpha1.PreDeployFailed
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil { // closes update_failed, records pre_deploy_status failed
		t.Fatalf("reconcile: %v", err)
	}

	deploys, err := store.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 {
		t.Fatalf("deploys = %+v (err %v), want exactly one", deploys, err)
	}
	d := deploys[0]
	if d.Status != DeployPreDeployFailed || d.PreDeployStatus != PreDeployFailed {
		t.Errorf("deploy = %+v, want pre_deploy_failed with pre_deploy_status=failed (migration failure, not health check)", d)
	}
}

// TestPreDeployStatusForIgnoresStaleGeneration guards the projection's
// generation gate: a status.preDeploy left over from a superseded revision must
// not be projected onto the current rollout's deploy.
func TestPreDeployStatusForIgnoresStaleGeneration(t *testing.T) {
	app := &appv1alpha1.App{}
	app.Generation = 5
	app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{Generation: 4, Status: appv1alpha1.PreDeploySucceeded}
	if got := preDeployStatusFor(app); got != "" {
		t.Errorf("stale-generation pre-deploy projected %q, want empty", got)
	}
	app.Status.PreDeploy.Generation = 5
	if got := preDeployStatusFor(app); got != PreDeploySucceeded {
		t.Errorf("current-generation pre-deploy = %q, want succeeded", got)
	}

	// An operational spec update can advance metadata generation while the same
	// release's pre-deploy remains authoritative.
	app.Generation = 6
	app.Status.ReleaseGeneration = 5
	if got := preDeployStatusFor(app); got != PreDeploySucceeded {
		t.Errorf("release-generation pre-deploy = %q, want succeeded", got)
	}
}

func TestObservedDeployStatusUsesCurrentGenerationEvidence(t *testing.T) {
	gen := int64(7)
	ready := func(reason string) []metav1.Condition {
		return []metav1.Condition{{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: reason,
			ObservedGeneration: gen,
		}}
	}
	app := func(phase appv1alpha1.AppPhase, reason string) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Generation: gen},
			Status: appv1alpha1.AppStatus{
				Phase: phase, Conditions: ready(reason),
			},
		}
	}

	tests := []struct {
		name     string
		open     Deploy
		app      *appv1alpha1.App
		timedOut bool
		want     string
	}{
		{"queued build", Deploy{Generation: gen, Status: DeployCreated}, app(appv1alpha1.PhaseBuilding, "BuildQueued"), false, DeployQueued},
		{"running build", Deploy{Generation: gen, Status: DeployQueued}, app(appv1alpha1.PhaseBuilding, "Building"), false, DeployBuildInProgress},
		{"build failed", Deploy{Generation: gen, Status: DeployBuildInProgress}, app(appv1alpha1.PhaseFailed, "BuildFailed"), false, DeployBuildFailed},
		{"build succeeded and rollout began", Deploy{Generation: gen, Status: DeployBuildInProgress}, app(appv1alpha1.PhaseDeploying, "Deploying"), false, DeployUpdateInProgress},
		{"rollout failed", Deploy{Generation: gen, Status: DeployUpdateInProgress}, app(appv1alpha1.PhaseFailed, "IngressFailed"), false, DeployUpdateFailed},
		{"build timed out", Deploy{Generation: gen, Status: DeployBuildInProgress}, app(appv1alpha1.PhasePending, ""), true, DeployBuildFailed},
		{"pre-deploy timed out", Deploy{Generation: gen, Status: DeployPreDeployInProgress}, app(appv1alpha1.PhasePending, ""), true, DeployPreDeployFailed},
		{"rollout timed out", Deploy{Generation: gen, Status: DeployUpdateInProgress}, app(appv1alpha1.PhaseDeploying, ""), true, DeployUpdateFailed},
	}

	preRunning := app(appv1alpha1.PhaseDeploying, "PreDeploy")
	preRunning.Status.PreDeploy = &appv1alpha1.PreDeployStatus{Generation: gen, Status: appv1alpha1.PreDeployRunning}
	tests = append(tests, struct {
		name     string
		open     Deploy
		app      *appv1alpha1.App
		timedOut bool
		want     string
	}{"pre-deploy running", Deploy{Generation: gen, Status: DeployBuildInProgress}, preRunning, false, DeployPreDeployInProgress})

	preFailed := app(appv1alpha1.PhaseFailed, "PreDeployFailed")
	preFailed.Status.PreDeploy = &appv1alpha1.PreDeployStatus{Generation: gen, Status: appv1alpha1.PreDeployFailed}
	tests = append(tests, struct {
		name     string
		open     Deploy
		app      *appv1alpha1.App
		timedOut bool
		want     string
	}{"pre-deploy failed", Deploy{Generation: gen, Status: DeployPreDeployInProgress}, preFailed, false, DeployPreDeployFailed})

	live := app(appv1alpha1.PhaseRunning, "Deployed")
	live.Status.ObservedGeneration = gen
	tests = append(tests, struct {
		name     string
		open     Deploy
		app      *appv1alpha1.App
		timedOut bool
		want     string
	}{"live", Deploy{Generation: gen, Status: DeployUpdateInProgress}, live, false, DeployLive})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := observedDeployStatus(tc.open, tc.app, tc.timedOut); got != tc.want {
				t.Errorf("observedDeployStatus = %q, want %q", got, tc.want)
			}
		})
	}

	stale := app(appv1alpha1.PhaseFailed, "BuildFailed")
	stale.Status.Conditions[0].ObservedGeneration = gen - 1
	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployBuildInProgress}, stale, false); got != "" {
		t.Errorf("stale condition emitted %q, want no transition", got)
	}
	newer := app(appv1alpha1.PhaseDeploying, "Deploying")
	newer.Generation = gen + 1
	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployCreated}, newer, false); got != "" {
		t.Errorf("different App generation emitted %q, want no transition", got)
	}

	operational := app(appv1alpha1.PhaseBuilding, "Building")
	operational.Generation = gen + 1
	operational.Status.Conditions[0].ObservedGeneration = gen + 1
	operational.Status.ReleaseGeneration = gen
	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployQueued}, operational, false); got != DeployBuildInProgress {
		t.Errorf("operational generation during build emitted %q, want %q", got, DeployBuildInProgress)
	}
	operational.Status.Phase = appv1alpha1.PhaseRunning
	operational.Status.ObservedGeneration = gen + 1
	operational.Status.ActiveRevision = fmt.Sprintf("rev-%d", gen)
	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployUpdateInProgress}, operational, false); got != DeployLive {
		t.Errorf("operational generation after rollout emitted %q, want %q", got, DeployLive)
	}
}

// TestObservedDeployStatusCancelsSupersededRow guards the stranded-row fix: a
// git-push redeploy, env-var change, or restart can mint a newer release
// identity without adopting the open row, after which the operator reports
// releaseGeneration past the row's. Generations are monotonic, so that row can
// never advance again — it closes canceled (the CR-side mirror of
// CreateDeploy's newest-wins cancel) instead of stranding open forever. An
// operator merely lagging BEHIND the row keeps waiting, and a legacy operator
// (no status.releaseGeneration) keeps today's conservative wait: its metadata
// generation also moves for operational churn, which must never cancel.
func TestObservedDeployStatusCancelsSupersededRow(t *testing.T) {
	gen := int64(184)
	superseded := &appv1alpha1.App{}
	superseded.Generation = gen + 2
	superseded.Status.Phase = appv1alpha1.PhaseBuilding
	superseded.Status.ReleaseGeneration = gen + 2

	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployBuildInProgress}, superseded, false); got != DeployCanceled {
		t.Errorf("superseded release => %q, want %q", got, DeployCanceled)
	}
	// Timed out or not: superseded is canceled, never repainted as a failure.
	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployBuildInProgress}, superseded, true); got != DeployCanceled {
		t.Errorf("superseded release (timed out) => %q, want %q", got, DeployCanceled)
	}

	// Fresh trigger, operator's first reconcile still pending: releaseGeneration
	// is BEHIND the row — nothing is superseded, keep waiting.
	lagging := &appv1alpha1.App{}
	lagging.Generation = gen
	lagging.Status.Phase = appv1alpha1.PhaseRunning
	lagging.Status.ReleaseGeneration = gen - 1
	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployCreated}, lagging, false); got != "" {
		t.Errorf("lagging operator => %q, want no transition", got)
	}

	// Legacy operator: metadata generation ahead, no releaseGeneration — the
	// pre-fix behavior (wait, don't guess) is preserved.
	legacy := &appv1alpha1.App{}
	legacy.Generation = gen + 2
	legacy.Status.Phase = appv1alpha1.PhaseBuilding
	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployBuildInProgress}, legacy, false); got != "" {
		t.Errorf("legacy generation churn => %q, want no transition", got)
	}
}

// TestObservedDeployStatusCancelsOrphanedRowAfterTimeout guards the recreated-CR
// case: an App CR deleted and recreated (e.g. the ADR043 per-tenant-namespace
// migration re-applying every CR) restarts status.releaseGeneration at 1, far
// BELOW the stored generation of any deploy row opened against the prior CR.
// The row can never be adopted, and a healthy PhaseRunning App never trips the
// phase-switch timeout, so before the fix it stayed build_in_progress forever
// (prod dep-d9kge9usnfpc73ajp8a0: generation 495 vs a recreated CR at rev-1).
// Once the row sits past its gate timeout it closes canceled — but only for a
// release-generation-aware operator, and never before the timeout (an operator
// merely lagging one generation behind must keep waiting).
func TestObservedDeployStatusCancelsOrphanedRowAfterTimeout(t *testing.T) {
	orphaned := &appv1alpha1.App{}
	orphaned.Generation = 1
	orphaned.Status.Phase = appv1alpha1.PhaseRunning
	orphaned.Status.ReleaseGeneration = 1
	orphaned.Status.ActiveRevision = "rev-1"
	open := Deploy{Generation: 495, Status: DeployBuildInProgress}

	// Not yet timed out: the operator might still be catching up — keep waiting.
	if got := observedDeployStatus(open, orphaned, false); got != "" {
		t.Errorf("orphaned row before timeout => %q, want no transition", got)
	}
	// Timed out against a recreated CR whose releaseGeneration can never reach the
	// row's: close it canceled instead of stranding it open forever.
	if got := observedDeployStatus(open, orphaned, true); got != DeployCanceled {
		t.Errorf("orphaned row after timeout => %q, want %q", got, DeployCanceled)
	}

	// A legacy operator (no status.releaseGeneration) still keeps the conservative
	// wait even when timed out: its metadata generation moves for operational
	// churn, so a mismatch is not trustworthy evidence of an orphaned release.
	legacy := &appv1alpha1.App{}
	legacy.Generation = 1
	legacy.Status.Phase = appv1alpha1.PhaseRunning
	if got := observedDeployStatus(open, legacy, true); got != "" {
		t.Errorf("legacy orphaned row after timeout => %q, want no transition", got)
	}
}

// TestRecordDeployCancelsSupersededRow runs the same scenario through the full
// Reconciler pass: an open create-row (generation 1) whose App the operator
// reports at releaseGeneration 3 closes canceled — finished, no failure
// reason, no failure notification semantics (canceled is exempt).
func TestRecordDeployCancelsSupersededRow(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	app := getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseBuilding
	app.Status.ReleaseGeneration = 3
	app.Status.Conditions = []metav1.Condition{{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: "Building",
		ObservedGeneration: 3,
	}}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	deploys, err := store.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 {
		t.Fatalf("deploys = %+v (err %v), want exactly one", deploys, err)
	}
	if deploys[0].Status != DeployCanceled || deploys[0].FinishedAt == nil {
		t.Errorf("deploy = {Status:%s FinishedAt:%v}, want canceled with finished_at set", deploys[0].Status, deploys[0].FinishedAt)
	}
	if deploys[0].FailureReason != "" {
		t.Errorf("failure_reason = %q, want empty — canceled is not a failure", deploys[0].FailureReason)
	}
}

// TestRecordDeployClosesFailedOnCRFailed mirrors the above for the CR's own
// Failed phase (a build error, say) — closes update_failed immediately, no
// need to wait out the gate window.
func TestRecordDeployClosesFailedOnCRFailed(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	app := getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseFailed
	app.Status.Conditions = []metav1.Condition{{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: "IngressFailed",
		Message:            "ingress rejected host example.com",
		ObservedGeneration: app.Generation,
	}}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	deploys, err := store.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 || deploys[0].Status != DeployUpdateFailed || deploys[0].FinishedAt == nil {
		t.Fatalf("deploys = %+v (err %v), want exactly one, update_failed, with finished_at set", deploys, err)
	}
	// w9/011: the failing close carries the CR's own diagnosis.
	if deploys[0].FailureReason != "ingress rejected host example.com" {
		t.Errorf("failure_reason = %q, want the Ready-condition message", deploys[0].FailureReason)
	}
}

// TestRecordDeployClosesFailedOnGateTimeout covers a deploy that never gates
// healthy and never reaches PhaseFailed either — a bad image stuck
// ImagePullBackOff, which the App CR's own phase machine polls PhaseDeploying
// forever (app_controller.go). Health gating (docs/ADR004-app-deployment.md) still needs
// to report failure eventually, so DeployGateTimeout is the fallback: a
// deploy open longer than it closes update_failed even with the CR still
// mid-rollout.
func TestRecordDeployClosesFailedOnGateTimeout(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	rec.DeployGateTimeout = 0 // any elapsed time trips it — deterministic without sleeping
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:bad", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	if err := rec.ReconcileOnce(ctx); err != nil { // pass 1: creates the CR, phase stays Pending
		t.Fatalf("reconcile: %v", err)
	}
	_ = getApp(t, cl)                              // the CR exists but never reports Running or Failed
	if err := rec.ReconcileOnce(ctx); err != nil { // pass 2: still not decisive by phase alone, but the gate window has elapsed
		t.Fatalf("reconcile: %v", err)
	}

	deploys, err := store.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 || deploys[0].Status != DeployUpdateFailed {
		t.Fatalf("deploys = %+v (err %v), want exactly one, update_failed via the gate timeout", deploys, err)
	}
	// w9/011: a pure-timeout close still explains itself.
	if deploys[0].FailureReason == "" {
		t.Error("failure_reason empty on a gate-timeout close, want the synthesized timeout line")
	}
}

// TestFailureReasonFor pins the w9/011 stamp choice: the operator's concrete
// diagnoses (CrashLoopBackOff / ImagePullBackOff / BuildFailed /
// PreDeployFailed) surface their condition message verbatim; anything else
// gets the synthesized health-gate line.
func TestFailureReasonFor(t *testing.T) {
	mk := func(phase appv1alpha1.AppPhase, reason, msg string) *appv1alpha1.App {
		app := &appv1alpha1.App{}
		app.Generation = 3
		app.Status.Phase = phase
		app.Status.Conditions = []metav1.Condition{{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: reason,
			Message: msg, ObservedGeneration: 3,
		}}
		return app
	}
	crash := "container exited shortly after start and is restarting repeatedly (last exit code 1) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (3000), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped)."
	if got, _ := failureReasonFor(mk(appv1alpha1.PhaseDeploying, "CrashLoopBackOff", crash)); got != crash {
		t.Errorf("CrashLoopBackOff reason = %q, want the condition message", got)
	}
	if got, code := failureReasonFor(mk(appv1alpha1.PhaseDeploying, "ImagePullBackOff", "image pull is failing: 401")); got != "image pull is failing: 401" || code != EventReasonImagePullBackoff {
		t.Errorf("ImagePullBackOff reason = %q, want the condition message", got)
	}
	// A bland in-progress condition proves nothing — synthesize the timeout line.
	if got, _ := failureReasonFor(mk(appv1alpha1.PhaseDeploying, "Deploying", "Reconciling Deployment for img")); got == "" || got == "Reconciling Deployment for img" {
		t.Errorf("bland Deploying condition reason = %q, want the synthesized timeout line", got)
	}
	// A stale-generation condition proves nothing either.
	stale := mk(appv1alpha1.PhaseDeploying, "CrashLoopBackOff", crash)
	stale.Status.Conditions[0].ObservedGeneration = 2
	if got, _ := failureReasonFor(stale); got == crash {
		t.Error("stale-generation condition message must not be stamped")
	}
}

func TestDeployTimedOutUsesPhaseSpecificBudgets(t *testing.T) {
	rec := NewReconciler(nil, nil, "default")
	now := time.Now()

	tests := []struct {
		name string
		app  DesiredApp
		open Deploy
		want bool
	}{
		{
			name: "repo build may exceed rollout gate",
			app:  DesiredApp{App: App{Repo: "https://github.com/acme/web"}},
			open: Deploy{Status: DeployBuildInProgress, UpdatedAt: now.Add(-4 * time.Minute)},
		},
		{
			name: "repo created phase uses build budget",
			app:  DesiredApp{App: App{Repo: "https://github.com/acme/web"}},
			open: Deploy{Status: DeployCreated, UpdatedAt: now.Add(-4 * time.Minute)},
		},
		{
			name: "pre-deploy may exceed rollout gate",
			open: Deploy{Status: DeployPreDeployInProgress, UpdatedAt: now.Add(-4 * time.Minute)},
		},
		{
			name: "build eventually times out",
			open: Deploy{Status: DeployBuildInProgress, UpdatedAt: now.Add(-defaultBuildGateTimeout - time.Minute)},
			want: true,
		},
		{
			name: "rollout uses short health gate",
			open: Deploy{Status: DeployUpdateInProgress, UpdatedAt: now.Add(-4 * time.Minute)},
			want: true,
		},
		{
			name: "image deploy created phase uses rollout gate",
			open: Deploy{Status: DeployCreated, UpdatedAt: now.Add(-4 * time.Minute)},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rec.deployTimedOut(tc.app, tc.open); got != tc.want {
				t.Fatalf("deployTimedOut = %v, want %v", got, tc.want)
			}
		})
	}
}

// fakeDeployNotifier records NotifyDeploy calls. Thread-safe and signals each
// call on a channel: recordDeploy fires DeployNotifier in a goroutine (w3/m9,
// so a slow relay can't block ReconcileOnce), so a test asserting on calls
// must synchronize on that channel rather than reading the slice immediately
// after ReconcileOnce returns.
type fakeDeployNotifier struct {
	mu        sync.Mutex
	calls     []DeployNotification
	notifiedC chan struct{}
}

func newFakeDeployNotifier() *fakeDeployNotifier {
	return &fakeDeployNotifier{notifiedC: make(chan struct{}, 16)}
}

func (f *fakeDeployNotifier) NotifyDeploy(_ context.Context, n DeployNotification) {
	f.mu.Lock()
	f.calls = append(f.calls, n)
	f.mu.Unlock()
	f.notifiedC <- struct{}{}
}

func (f *fakeDeployNotifier) snapshot() []DeployNotification {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]DeployNotification(nil), f.calls...)
}

// awaitCall blocks until NotifyDeploy has been called at least once more since
// the last awaitCall/newFakeDeployNotifier, or fails the test after 2s — the
// bounded wait for the backgrounded goroutine recordDeploy launches.
func (f *fakeDeployNotifier) awaitCall(t *testing.T) {
	t.Helper()
	select {
	case <-f.notifiedC:
	case <-time.After(2 * time.Second):
		t.Fatal("NotifyDeploy was not called within 2s")
	}
}

// TestRecordDeployNotifiesExactlyOnceOnClose (w3/m9) pins two things at once:
// DeployNotifier fires with the right (tenant, app, status) the pass a deploy
// actually closes, and it does NOT fire again on a later pass over the same
// already-closed deploy — recordDeploy gates the call on CloseDeploy's own ok
// return, the same idempotency guard that protects a Cancel race.
func TestRecordDeployNotifiesExactlyOnceOnClose(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	notifier := newFakeDeployNotifier()
	rec.DeployNotifier = notifier
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	_, _ = store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	if err := rec.ReconcileOnce(ctx); err != nil { // pass 1: creates the CR, no status yet — no notify
		t.Fatalf("reconcile: %v", err)
	}
	if calls := notifier.snapshot(); len(calls) != 0 {
		t.Fatalf("calls after create = %+v, want none (deploy still open)", calls)
	}

	app := getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseRunning
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil { // pass 2: closes live — notify fires once
		t.Fatalf("reconcile: %v", err)
	}
	notifier.awaitCall(t)
	if calls := notifier.snapshot(); len(calls) != 1 || calls[0].TenantID != ten.ID || calls[0].AppName != "web" || calls[0].Status != DeployLive {
		t.Fatalf("calls after close = %+v, want exactly one (tenant=%s app=web status=%s)", calls, ten.ID, DeployLive)
	}
	// The notification carries the closing deploy's id so the email can build a
	// "View Logs" deep link (w7/m44); it's threaded from the same open row that
	// supplies the (here empty, image-backed) commit message.
	if calls := notifier.snapshot(); calls[0].DeployID == "" {
		t.Errorf("DeployNotification.DeployID = %q, want the closed deploy's id", calls[0].DeployID)
	}

	if err := rec.ReconcileOnce(ctx); err != nil { // pass 3: nothing left open — no re-notify
		t.Fatalf("reconcile: %v", err)
	}
	if calls := notifier.snapshot(); len(calls) != 1 {
		t.Fatalf("calls after a third pass = %d, want still 1 (no re-notify of an already-closed deploy)", len(calls))
	}
}

// fakeCloneSecreter records calls and returns a fixed secret name.
type fakeCloneSecreter struct {
	calls []struct{ namespace, appName, workspaceID, repo string }
}

func (f *fakeCloneSecreter) EnsureCloneSecret(_ context.Context, namespace, appName, workspaceID, repo string) (string, error) {
	f.calls = append(f.calls, struct{ namespace, appName, workspaceID, repo string }{namespace, appName, workspaceID, repo})
	return appName + "-clone", nil
}

func TestProjectAppCallsCloneSecreterForRepoApp(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	cs := &fakeCloneSecreter{}
	rec.CloneSecrets = cs

	ten, _ := st.CreateTenant(ctx, "acme", "free")
	_, _ = st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Repo: "https://github.com/acme/web", Branch: "main",
		Port: 3000, Replicas: 1, Tier: "free",
	})

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	app := getApp(t, cl)
	if app.Spec.CloneSecret != app.Name+"-clone" {
		t.Errorf("CloneSecret = %q, want %q", app.Spec.CloneSecret, app.Name+"-clone")
	}
	if len(cs.calls) != 1 {
		t.Fatalf("CloneSecreter called %d times, want 1", len(cs.calls))
	}
	if cs.calls[0].repo != "https://github.com/acme/web" {
		t.Errorf("repo = %q", cs.calls[0].repo)
	}
}

func TestProjectAppSkipsCloneSecreterForImageApp(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	cs := &fakeCloneSecreter{}
	rec.CloneSecrets = cs

	ten, _ := st.CreateTenant(ctx, "acme", "free")
	_, _ = st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Image: "nginx:1", Branch: "main",
		Port: 80, Replicas: 1, Tier: "free",
	})

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	_ = getApp(t, cl)
	if len(cs.calls) != 0 {
		t.Errorf("CloneSecreter called for an image-backed App; should be skipped")
	}
}
