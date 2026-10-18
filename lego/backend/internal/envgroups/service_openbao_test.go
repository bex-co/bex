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

// The store-backed service against a real OpenBao KV v2 engine and a real
// control-plane Postgres (w8/m114): the row carries every non-secret fact, so
// OpenBao holds only the group's value maps, and the unique index answers a
// duplicate name.

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
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
	ctx := groupCtx(context.Background(), prefix)
	var out []string
	var walk func(path string)
	walk = func(path string) {
		data, err := kv.Get(ctx, path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		if len(data) > 0 {
			out = append(out, path) // a path may be a secret and a folder
		}
		children, err := kv.List(ctx, path)
		if err != nil {
			t.Fatalf("list %s: %v", path, err)
		}
		for _, child := range children {
			walk(path + "/" + child)
		}
	}
	walk(root)
	slices.Sort(out)
	return out
}

func TestServiceAgainstRealOpenBaoAndPostgres(t *testing.T) {
	kv, pg := realStores(t)
	ctx := context.Background()
	tenant, err := pg.CreateTenant(ctx, "envgroups-real-"+time.Now().Format("150405.000000000"), store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), tenant.ID) })
	svc := &Service{
		Base:   &core.Base{Client: fakeClient(), Namespace: "default", Workspace: coretest.Workspaces{tenant.ID}},
		Store:  kv,
		Groups: pg,
	}
	sctx := tctx(t)
	created, err := svc.CreateEnvGroup(sctx, CreateEnvGroupRequest{
		Name:        "real",
		EnvVars:     []CreateEnvVarInput{{Key: "A", Value: "1"}},
		SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "PEM"}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var coded *core.CodedError
	if _, err := svc.CreateEnvGroup(sctx, CreateEnvGroupRequest{Name: "real"}); !errors.As(err, &coded) || coded.Code != "ENV_GROUP_NAME_EXISTS" {
		t.Fatalf("duplicate create = %v, want ENV_GROUP_NAME_EXISTS", err)
	}

	before, err := pg.GetEnvGroup(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetEnvGroupVar(sctx, created.ID, "B", "2"); err != nil {
		t.Fatalf("save: %v", err)
	}
	row, err := pg.GetEnvGroup(ctx, created.ID)
	if err != nil || row.TenantID != tenant.ID || !slices.Equal(row.EnvKeys, []string{"A", "B"}) ||
		!slices.Equal(row.FileNames, []string{"cert.pem"}) || row.Content.State != store.EnvGroupIdle {
		t.Fatalf("row after save = %+v, %v", row, err)
	}
	view, err := svc.GetEnvGroup(sctx, created.ID)
	if err != nil || view.Revision != encodeEnvGroupRevision(before.Content.Generation+1) {
		t.Fatalf("view after save = %+v, %v; want generation %d", view, err, before.Content.Generation+1)
	}
	if got, err := svc.GetEnvGroupVar(sctx, created.ID, "B"); err != nil || got.Value != "2" {
		t.Fatalf("reveal = %+v, %v", got, err)
	}
	// The save's recovery maps are gone; only the two value maps remain. The
	// paths are spelled out to pin ADR013's layout, not the package's helpers.
	want := []string{"env-groups/" + created.ID + "/env", "env-groups/" + created.ID + "/files"}
	if got := allKeys(t, kv, tenant.ID, "env-groups"); !slices.Equal(got, want) {
		t.Fatalf("OpenBao keys = %v, want only the value maps %v", got, want)
	}

	if err := svc.DeleteEnvGroup(sctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := pg.GetEnvGroup(ctx, created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("row after delete = %v, want ErrNotFound", err)
	}
	if got := allKeys(t, kv, tenant.ID, "env-groups"); len(got) != 0 {
		t.Fatalf("OpenBao keys after delete = %v, want none", got)
	}
}
