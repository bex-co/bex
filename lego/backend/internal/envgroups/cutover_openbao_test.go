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

package envgroups

// The cutover and the store-backed service against a real OpenBao KV v2 engine
// and a real control-plane Postgres (w8/m114): the KV semantics the fakes model
// — nested folders, a path that is both a secret and a folder, deletes of
// absent paths — and the env_groups row round trip. Leaves with cutover.go.

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func realStores(t *testing.T) (core.SecretKV, *store.PGStore) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	addr, token := os.Getenv("BEX_TEST_OPENBAO_KV_URL"), os.Getenv("BEX_TEST_OPENBAO_KV_TOKEN")
	if uri == "" || addr == "" || token == "" {
		testenv.Skip(t, "requires BEX_TEST_DB_URI, BEX_TEST_OPENBAO_KV_URL and BEX_TEST_OPENBAO_KV_TOKEN")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	kv, remove, err := secrets.NewOpenBaoTestStore(context.Background(), addr, token)
	if err != nil {
		t.Fatalf("openbao: %v", err)
	}
	t.Cleanup(remove)
	return kv, store.NewPGStore(pool)
}

// allKeys lists every secret under root in prefix, depth first.
func allKeys(t *testing.T, kv core.SecretKV, prefix, root string) []string {
	t.Helper()
	ctx := secrets.WithTenant(context.Background(), prefix)
	var out []string
	var walk func(path string)
	walk = func(path string) {
		children, err := kv.List(ctx, path)
		if err != nil {
			t.Fatalf("list %s: %v", path, err)
		}
		if len(children) == 0 {
			if data, _ := kv.Get(ctx, path); len(data) > 0 {
				out = append(out, path)
			}
			return
		}
		if data, _ := kv.Get(ctx, path); len(data) > 0 {
			out = append(out, path) // a path that is a secret and a folder
		}
		for _, child := range children {
			walk(path + "/" + child)
		}
	}
	walk(root)
	slices.Sort(out)
	return out
}

func TestCutoverAgainstRealOpenBaoAndPostgres(t *testing.T) {
	kv, pg := realStores(t)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000000")
	tenant, err := pg.CreateTenant(ctx, "cutover-a-"+suffix, store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	other, err := pg.CreateTenant(ctx, "cutover-b-"+suffix, store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	put := func(prefix, path string, data map[string]string) {
		t.Helper()
		if err := kv.Put(secrets.WithTenant(ctx, prefix), path, data); err != nil {
			t.Fatalf("put %s %s: %v", prefix, path, err)
		}
	}
	ts := "2026-08-01T10:00:00Z"
	migrated, unmigrated, fenced, orphan := id.New(id.EnvGroup), id.New(id.EnvGroup), id.New(id.EnvGroup), id.New(id.EnvGroup)
	// w2/m80 layout: workspace copy, legacy locator, name claim, a finished
	// op record and a stranded op's maps under the same "op" path.
	put(tenant.ID, "env-groups/"+migrated+"/meta", map[string]string{"name": "migrated", "workspace": tenant.ID, "linksByID": "1", "createdAt": ts, "updatedAt": ts})
	put(tenant.ID, "env-groups/"+migrated+"/revision", map[string]string{"state": "idle", "generation": "4"})
	put(tenant.ID, "env-groups/"+migrated+"/env", map[string]string{"A": "1"})
	put(tenant.ID, "env-groups/"+migrated+"/op", map[string]string{"id": "ego-old", "kind": "patch", "phase": "committed"})
	put(tenant.ID, "env-groups/"+migrated+"/op/ego-old/old-env", map[string]string{"A": "0"})
	put(tenant.ID, "env-group-name-claims/digest", map[string]string{"id": migrated, "name": "migrated"})
	put(secrets.LegacyTenant, "env-groups/"+migrated+"/meta", map[string]string{"locator": "1", "workspace": tenant.ID})
	// Never migrated: everything at the legacy tenant, owned by the other workspace.
	put(secrets.LegacyTenant, "env-groups/"+unmigrated+"/meta", map[string]string{"name": "unmigrated", "workspace": other.ID, "linksByID": "1", "createdAt": ts, "updatedAt": ts})
	put(secrets.LegacyTenant, "env-groups/"+unmigrated+"/revision", map[string]string{"state": "idle", "generation": "2"})
	put(secrets.LegacyTenant, "env-groups/"+unmigrated+"/files", map[string]string{"cert.pem": "PEM"})
	// Deleted under w4/m97, with residue.
	put(tenant.ID, "env-groups/"+fenced+"/meta", map[string]string{"tombstoned": "1"})
	put(tenant.ID, "env-groups/"+fenced+"/op/ego-x/new-env", map[string]string{"X": "1"})
	// A create of the new code in flight: maps, no row, no KV metadata.
	put(tenant.ID, "env-groups/"+orphan+"/env", map[string]string{"Z": "9"})

	cutover := &Cutover{Secrets: kv, Store: pg, Client: fakeClient(), Now: time.Now}
	if report, err := cutover.Run(ctx, "dry-run"); err != nil || report.Migrated != 0 || len(report.Refusals) != 0 {
		t.Fatalf("dry-run = %+v, %v", report, err)
	}
	report, err := cutover.Run(ctx, "apply")
	if err != nil {
		t.Fatalf("apply: %v (%+v)", err, report)
	}
	if report.Migrated < 2 {
		t.Fatalf("apply migrated %d groups, want at least this test's 2: %+v", report.Migrated, report)
	}
	row, err := pg.GetEnvGroup(ctx, migrated)
	if err != nil || row.TenantID != tenant.ID || row.Content.Generation != 4 || !slices.Equal(row.EnvKeys, []string{"A"}) {
		t.Fatalf("migrated row = %+v, %v", row, err)
	}
	if row, err := pg.GetEnvGroup(ctx, unmigrated); err != nil || row.TenantID != other.ID || !slices.Equal(row.FileNames, []string{"cert.pem"}) {
		t.Fatalf("unmigrated row = %+v, %v", row, err)
	}

	// The store-backed service serves the moved group and saves through it.
	svc := withGroups(&Service{
		Base:   &core.Base{Client: fakeClient(), Namespace: "default", Workspace: coretest.Workspaces{tenant.ID}},
		Store:  kv,
		Groups: pg,
	})
	sctx := testCtx()
	if _, err := svc.SetEnvGroupVar(sctx, migrated, "B", "2"); err != nil {
		t.Fatalf("save through the migrated group: %v", err)
	}
	view, err := svc.GetEnvGroup(sctx, migrated)
	if err != nil || len(view.EnvVars) != 2 || view.Revision != encodeEnvGroupRevision(5) {
		t.Fatalf("view after save = %+v, %v", view, err)
	}
	created, err := svc.CreateEnvGroup(sctx, CreateEnvGroupRequest{Name: "after-cutover", EnvVars: []CreateEnvVarInput{{Key: "N", Value: "new"}}})
	if err != nil {
		t.Fatalf("create after cutover: %v", err)
	}
	// A save in flight keeps its recovery maps through prune.
	live, _ := pg.GetEnvGroup(ctx, created.ID)
	if _, err := pg.ClaimEnvGroupContent(ctx, created.ID, live.Content.Version, store.EnvGroupOp{
		ID: "ego-live", Kind: opKindPatch, Phase: opPhaseAdmitted, LeaseUntil: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	put(tenant.ID, opOldEnvPath(created.ID, "ego-live"), map[string]string{"N": "new"})

	if report, err = cutover.Run(ctx, "prune"); err != nil {
		t.Fatalf("prune: %v (%+v)", err, report)
	}
	gotTenant := allKeys(t, kv, tenant.ID, "env-groups")
	wantTenant := []string{
		"env-groups/" + migrated + "/env",
		"env-groups/" + orphan + "/env",
		"env-groups/" + created.ID + "/env",
		opOldEnvPath(created.ID, "ego-live"),
	}
	slices.Sort(wantTenant)
	if !slices.Equal(gotTenant, wantTenant) {
		t.Fatalf("tenant keys after prune = %v\nwant %v", gotTenant, wantTenant)
	}
	if claims := allKeys(t, kv, tenant.ID, "env-group-name-claims"); len(claims) != 0 {
		t.Fatalf("name claims survived: %v", claims)
	}
	if gotOther := allKeys(t, kv, other.ID, "env-groups"); !slices.Equal(gotOther, []string{"env-groups/" + unmigrated + "/files"}) {
		t.Fatalf("other tenant keys = %v, want only the copied files map", gotOther)
	}
	for _, key := range allKeys(t, kv, secrets.LegacyTenant, "env-groups") {
		if strings.Contains(key, migrated) || strings.Contains(key, unmigrated) {
			t.Fatalf("legacy tenant kept state of a group living elsewhere: %s", key)
		}
	}
	if got, err := svc.GetEnvGroupVar(sctx, migrated, "A"); err != nil || got.Value != "1" {
		t.Fatalf("value after prune = %+v, %v", got, err)
	}
	if again, err := cutover.Run(ctx, "prune"); err != nil || again.PrunedPaths != 0 {
		t.Fatalf("second prune = %+v, %v; want nothing left to delete", again, err)
	}
}
