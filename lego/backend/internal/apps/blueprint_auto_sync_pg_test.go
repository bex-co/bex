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
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This walkthrough uses the signed intake, real PostgreSQL claims/lifecycle,
// actual shared apply engine and a controlled Kubernetes client. It does not
// claim a live GitHub delivery or running tenant workload.
func TestPGBlueprintAutoSyncWorkerOrderingRecoveryAndPolicy(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	workspace, err := st.CreateWorkspace(ctx, "auto-order-"+id.New(id.Owner), store.PlanHobby, "auto-order-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, workspace.ID) })
	repo := "https://github.com/acme/" + workspace.ID
	bp, err := st.UpsertBlueprint(ctx, store.Blueprint{TenantID: workspace.ID, Name: "ordering", Repo: repo, Branch: "main", Path: CanonicalBlueprintFilename, AutoSync: true, Status: store.BlueprintStatusInSync, Manifest: "services: []\n"})
	if err != nil {
		t.Fatal(err)
	}
	git := &advancingBlueprintFetcher{headSHA: shaB, byCommit: map[string]string{shaA: strings.ReplaceAll(ownershipManifest, "nginx:1", "nginx:old"), shaB: strings.ReplaceAll(ownershipManifest, "nginx:1", "nginx:current"), strings.Repeat("c", 40): strings.ReplaceAll(ownershipManifest, "nginx:1", "nginx:later")}}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"auto-order-owner": workspace.ID}}, Store: st, Blueprints: st, GitFetcher: git, DomainTraffic: allowDomainTraffic{}}
	h := &GitWebhook{Svc: svc, Secret: "auto-order-key"}
	push := func(commit, event string) {
		t.Helper()
		body := strings.Replace(pushBodyWithPaths(repo, "main", commit, []string{CanonicalBlueprintFilename}), "msg", event, 1)
		rec := postSignedPush(t, h, "auto-order-key", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("signed intake => %d: %s", rec.Code, rec.Body)
		}
	}
	push(shaB, "current-B")
	push(shaA, "delayed-A")
	// Replica 1 dies after accepting its claim. Replica 2 starts with no memory
	// of that request, reclaims it, and cannot resurrect its old token.
	claims, err := st.ClaimBlueprintAutoSyncIntents(ctx, 50, time.Minute)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	old := claims[0]
	if _, err := pool.Exec(ctx, `UPDATE blueprint_auto_sync_intents SET claimed_at=now()-interval '6 minutes' WHERE id=$1`, old.ID); err != nil {
		t.Fatal(err)
	}
	restarted := &BlueprintAutoSyncWorker{Svc: svc}
	if err := restarted.tick(ctx); err != nil {
		t.Fatal(err)
	}
	if err := (&BlueprintAutoSyncWorker{Svc: svc}).tick(ctx); err != nil {
		t.Fatal(err)
	}
	assertImage := func(image string) {
		t.Helper()
		if a := getTenantApp(t, svc.Client, workspace.ID, "web"); a.Spec.Image != image {
			t.Fatalf("image=%q want %q", a.Spec.Image, image)
		}
	}
	assertImage("nginx:current")
	apps, err := st.ListApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, app := range apps {
		if app.TenantID != workspace.ID {
			continue
		}
		deploys, err := st.ListDeploys(ctx, app.ID, store.DeployFilter{})
		if err != nil || len(deploys) != 1 {
			t.Fatalf("coalesced identical manifest churned releases: %+v, %v", deploys, err)
		}
	}
	syncs, err := st.ListBlueprintSyncs(ctx, bp.ID, "", 20)
	if err != nil || len(syncs) != 2 {
		t.Fatalf("syncs=%+v err=%v", syncs, err)
	}
	for _, run := range syncs {
		if run.CommitID != shaB || run.State != store.BlueprintSyncStateSuccess {
			t.Fatalf("untruthful current-branch history: %+v", run)
		}
	}
	if !strings.Contains(syncs[0].Note, shaA) || !strings.Contains(syncs[0].Note, shaB) {
		t.Fatalf("coalescing note=%q", syncs[0].Note)
	}
	(&BlueprintAutoSyncWorker{Svc: svc}).process(ctx, old)
	again, err := st.ListBlueprintSyncs(ctx, bp.ID, "", 20)
	if err != nil || len(again) != len(syncs) {
		t.Fatal("stale pre-restart token admitted another run")
	}
	assertImage("nginx:current")
	// The same lower SHA is a genuine current force push; no SHA/timestamp
	// ordering can distinguish it from the delayed event above.
	git.headSHA = shaA
	push(shaA, "force-push-A")
	if err := restarted.tick(ctx); err != nil {
		t.Fatal(err)
	}
	assertImage("nginx:old")
	// Policy is read at execution, after durable acceptance.
	git.headSHA = strings.Repeat("c", 40)
	push(git.headSHA, "accepted-before-off")
	off := false
	if _, err := st.UpdateBlueprint(ctx, bp.ID, workspace.ID, nil, &off, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := restarted.tick(ctx); err != nil {
		t.Fatal(err)
	}
	assertImage("nginx:old")
	on := true
	if _, err := st.UpdateBlueprint(ctx, bp.ID, workspace.ID, nil, &on, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	push(git.headSHA, "accepted-before-disconnect")
	if err := st.DisconnectBlueprint(ctx, bp.ID, workspace.ID); err != nil {
		t.Fatal(err)
	}
	if err := restarted.tick(ctx); err != nil {
		t.Fatal(err)
	}
	assertImage("nginx:old")
	var open int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blueprint_auto_sync_intents WHERE blueprint_id=$1 AND state IN ('pending','claimed')`, bp.ID).Scan(&open); err != nil || open != 0 {
		t.Fatalf("unsettled policy work=%d err=%v", open, err)
	}
}
