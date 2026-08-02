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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestLiveAcceptance is w2/m5's t005: create -> deploy #1 recorded -> live;
// trigger -> deploy #2 recorded -> polled to live; a broken image -> the
// deploy truthfully reads update_failed. It drives the REAL PGStore, the
// REAL Reconciler, and a REAL App CR against a REAL cluster + REAL operator —
// a manual/local-only run, not CI (needs BEX_TEST_DB_URI + KUBECONFIG
// pointing at a running mock cluster with the operator reconciling it).
func TestLiveAcceptance(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	if dbURI == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	// This drives a REAL App CR against a REAL cluster + operator, so it needs a
	// KUBECONFIG pointing at a running mock cluster (see the doc comment above).
	// CI sets BEX_TEST_DB_URI for the other integration tests but has no such
	// cluster, so gate on KUBECONFIG too — otherwise ctrl.GetConfig() below
	// fatals in CI. Manual/local runs export KUBECONFIG to opt in.
	if os.Getenv("KUBECONFIG") == "" {
		t.Skip("KUBECONFIG not set — TestLiveAcceptance is manual/local-only (needs a mock cluster + operator)")
	}
	ctx := context.Background()

	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	st := store.NewPGStore(pool)

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cfg, err := ctrl.GetConfig()
	if err != nil {
		t.Fatalf("kubeconfig: %v (set KUBECONFIG to the mock cluster)", err)
	}
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("kube client: %v", err)
	}

	rec := store.NewReconciler(cl, st)
	rec.DeployGateTimeout = 6 * time.Second // short so the broken-image case doesn't take 3 real minutes

	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: st}

	ten, err := st.CreateTenant(ctx, "m5acc", store.PlanHobby)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	// --- 1: create -> deploy #1 recorded -> live -----------------------------
	app, err := st.CreateApp(ctx, store.App{
		TenantID: ten.ID, Name: "web", Image: "traefik/whoami", Branch: "main",
		Port: 80, Replicas: 1, Tier: "free",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	name := store.CRName("m5acc", "web")

	waitForPhase(t, ctx, rec, cl, name, appv1alpha1.PhaseRunning, 60*time.Second)

	list, err := svc.List(ctx, name, ListFilter{})
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	if len(list) != 1 || list[0].Status != store.DeployLive {
		t.Fatalf("deploy #1 = %+v, want exactly one, live", list)
	}
	t.Logf("deploy #1 recorded live: %+v", list[0])

	// --- 2: trigger -> deploy #2 recorded -> polled to live -------------------
	triggered, err := svc.Trigger(ctx, name, TriggerParams{})
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if triggered.Status != store.DeployCreated || triggered.Trigger != "api" {
		t.Fatalf("triggered deploy = %+v", triggered)
	}
	waitForDeployStatus(t, ctx, rec, svc, name, triggered.ID, store.DeployLive, 60*time.Second)

	list, err = svc.List(ctx, name, ListFilter{})
	if err != nil {
		t.Fatalf("list after trigger: %v", err)
	}
	if len(list) != 2 || list[0].ID != triggered.ID {
		t.Fatalf("list after trigger (want newest-first, 2 entries) = %+v", list)
	}
	t.Logf("deploy #2 (triggered) recorded live: %+v", list[0])

	// --- 3: a broken image reads update_failed, never live --------------------
	badApp, err := st.CreateApp(ctx, store.App{
		TenantID: ten.ID, Name: "bad", Image: "typo-registry.invalid/no/such/image:latest", Branch: "main",
		Port: 80, Replicas: 1, Tier: "free",
	})
	if err != nil {
		t.Fatalf("create bad app: %v", err)
	}
	badName := store.CRName("m5acc", "bad")
	waitForDeployStatus(t, ctx, rec, svc, badName, "", store.DeployUpdateFailed, 30*time.Second)

	badList, err := svc.List(ctx, badName, ListFilter{})
	if err != nil {
		t.Fatalf("list bad deploys: %v", err)
	}
	if len(badList) != 1 || badList[0].Status != store.DeployUpdateFailed {
		t.Fatalf("broken-image deploy = %+v, want exactly one, update_failed", badList)
	}
	t.Logf("broken-image deploy truthfully update_failed: %+v", badList[0])

	// --- 4: bad-image deploy -> rollback -> previous digest live (w2/m10/t005) ---
	// "web" is live on deploy #2's image (traefik/whoami, from step 2's
	// trigger); simulate a bad deploy by pointing spec.image at a bogus tag
	// directly (bex-api has no "change image" verb yet, so this mirrors what
	// one would eventually drive) and opening a matching deploy row, the same
	// bookkeeping Trigger would do for an image change.
	goodDeploys, err := svc.List(ctx, name, ListFilter{})
	if err != nil || len(goodDeploys) == 0 {
		t.Fatalf("list web deploys before rollback: %+v (err %v)", goodDeploys, err)
	}
	lastGood := goodDeploys[0] // newest-first; this is deploy #2, live on traefik/whoami
	webApp := getApp(t, cl, name)
	webApp.Spec.Image = "typo-registry.invalid/no/such/image:latest"
	webApp.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	// Update first (as Trigger itself now does) so the deploy row is stamped
	// with the CR's post-bump generation, not a value a race could move past.
	if err := cl.Update(ctx, webApp); err != nil {
		t.Fatalf("patch web to a bad image: %v", err)
	}
	badDeploy, err := st.CreateDeploy(ctx, app.ID, "api", "typo-registry.invalid/no/such/image:latest", webApp.Generation, store.CommitInfo{})
	if err != nil {
		t.Fatalf("open bad deploy on web: %v", err)
	}
	waitForDeployStatus(t, ctx, rec, svc, name, badDeploy.ID, store.DeployUpdateFailed, 30*time.Second)
	t.Logf("web's bad-image redeploy truthfully update_failed: %s", badDeploy.ID)

	rolled, err := svc.Rollback(ctx, name, lastGood.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolled.Trigger != "rollback" || rolled.RollbackOf != lastGood.ID {
		t.Fatalf("rolled-back deploy = %+v, want trigger=rollback rollbackOf=%s", rolled, lastGood.ID)
	}
	waitForDeployStatus(t, ctx, rec, svc, name, rolled.ID, store.DeployLive, 60*time.Second)
	waitForPhase(t, ctx, rec, cl, name, appv1alpha1.PhaseRunning, 60*time.Second)
	restored := getApp(t, cl, name)
	if restored.Spec.Image != lastGood.Image {
		t.Fatalf("web's image after rollback = %q, want %q (the rolled-back-to deploy's image)", restored.Spec.Image, lastGood.Image)
	}
	t.Logf("rollback restored web to %s, live: %+v", restored.Spec.Image, rolled)

	// --- 5: mid-build cancel -> job gone (w2/m10/t005) --------------------------
	// A fresh deploy on the now-healthy "web" is still update_in_progress the
	// instant it opens; Cancel closes it canceled before it ever converges,
	// and a further reconcile pass must not clobber that back to live — the
	// CAS guard (store.CloseDeploy) is what makes that race safe.
	toCancel, err := svc.Trigger(ctx, name, TriggerParams{})
	if err != nil {
		t.Fatalf("trigger a deploy to cancel: %v", err)
	}
	canceled, err := svc.Cancel(ctx, name, toCancel.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if canceled.Status != store.DeployCanceled {
		t.Fatalf("canceled deploy = %+v, want status canceled", canceled)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile after cancel: %v", err)
	}
	stillCanceled, err := svc.Get(ctx, name, toCancel.ID)
	if err != nil || stillCanceled.Status != store.DeployCanceled {
		t.Fatalf("deploy after a further reconcile = %+v (err %v), want it to stay canceled (CAS guard)", stillCanceled, err)
	}
	t.Logf("canceled deploy survives a further reconcile pass: %+v", stillCanceled)

	// Cleanup.
	_ = st.DeleteApp(ctx, app.ID)
	_ = st.DeleteApp(ctx, badApp.ID)
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Logf("cleanup reconcile: %v", err)
	}
}

// waitForPhase drives the reconciler and polls the CR until it reaches phase
// (or the deadline passes) — the reconciler alone converges spec; the real
// operator running against the same cluster is what turns that into a
// running Deployment and writes status back onto the CR.
func waitForPhase(t *testing.T, ctx context.Context, rec *store.Reconciler, cl client.Client, name string, phase appv1alpha1.AppPhase, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Logf("reconcile: %v", err)
		}
		var a appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, &a); err == nil && a.Status.Phase == phase {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never reached phase %s within %s", name, phase, timeout)
		}
		time.Sleep(2 * time.Second)
	}
}

// waitForDeployStatus drives the reconciler and polls get_deploy (or list, if
// deployID is empty — the broken-image case, where the id isn't known ahead
// of the poll) until the (a) deploy reaches status, or the deadline passes.
func waitForDeployStatus(t *testing.T, ctx context.Context, rec *store.Reconciler, svc *Service, service, deployID, status string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Logf("reconcile: %v", err)
		}
		got, ok := currentStatus(ctx, svc, service, deployID)
		if ok && got == status {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s's deploy never reached %s within %s (last seen %q)", service, status, timeout, got)
		}
		time.Sleep(2 * time.Second)
	}
}

func currentStatus(ctx context.Context, svc *Service, service, deployID string) (string, bool) {
	if deployID != "" {
		d, err := svc.Get(ctx, service, deployID)
		if err != nil {
			return "", false
		}
		return d.Status, true
	}
	list, err := svc.List(ctx, service, ListFilter{})
	if err != nil || len(list) == 0 {
		return "", false
	}
	return list[0].Status, true
}
