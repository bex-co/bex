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

package secrets

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// tenantFakeSecretStore is a tenant-aware in-memory core.SecretKV that keys each
// path under the ctx tenant (tenantFromCtx), mirroring the real OpenBao store's
// w7/m70 keying. fakeSecretStore (tenant-ignorant) hides the prefix, so these
// tests use this one to assert cross-workspace isolation + lazy migration.
type tenantFakeSecretStore struct {
	m map[string]map[string]string // key: tenantFromCtx(ctx) + "/" + path
}

func newTenantFakeSecretStore() *tenantFakeSecretStore {
	return &tenantFakeSecretStore{m: map[string]map[string]string{}}
}

func (f *tenantFakeSecretStore) key(ctx context.Context, path string) string {
	return tenantFromCtx(ctx) + "/" + path
}

func (f *tenantFakeSecretStore) Get(ctx context.Context, path string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range f.m[f.key(ctx, path)] {
		out[k] = v
	}
	return out, nil
}

func (f *tenantFakeSecretStore) Put(ctx context.Context, path string, data map[string]string) error {
	cp := map[string]string{}
	for k, v := range data {
		cp[k] = v
	}
	f.m[f.key(ctx, path)] = cp
	return nil
}

func (f *tenantFakeSecretStore) Delete(ctx context.Context, path string) error {
	delete(f.m, f.key(ctx, path))
	return nil
}

func (f *tenantFakeSecretStore) List(context.Context, string) ([]string, error) {
	return nil, nil
}

// TestOpenBaoStore_TenantScopedPaths proves the w7/m70 fix for codex-security #1
// at the store level: the same logical path under two different ctx tenants maps
// to two disjoint KV paths, so two same-named services in different workspaces
// cannot read or overwrite each other's values.
func TestOpenBaoStore_TenantScopedPaths(t *testing.T) {
	stub := &baoStub{t: t, data: map[string]map[string]string{}}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)
	s := newOpenBaoStoreForTest(t, srv.URL)

	ctxA := withTenant(context.Background(), "tea-a")
	ctxB := withTenant(context.Background(), "tea-b")

	if err := s.Put(ctxA, "services/web/env", map[string]string{"TOKEN": "a-secret"}); err != nil {
		t.Fatalf("Put under tea-a: %v", err)
	}
	// tea-b's "web" must not see tea-a's value.
	if got, err := s.Get(ctxB, "services/web/env"); err != nil || len(got) != 0 {
		t.Fatalf("tea-b must not see tea-a's value: %+v err=%v", got, err)
	}
	// tea-a reads its own value back.
	if got, err := s.Get(ctxA, "services/web/env"); err != nil || got["TOKEN"] != "a-secret" {
		t.Fatalf("tea-a should see its own value: %+v err=%v", got, err)
	}
	// The stub holds them under distinct tenant prefixes (no collision).
	if stub.data["tea-a/services/web/env"]["TOKEN"] != "a-secret" {
		t.Fatalf("tea-a KV path missing/wrong: %+v", stub.data)
	}
	if _, ok := stub.data["tea-b/services/web/env"]; ok {
		t.Fatalf("tea-b KV path should not exist: %+v", stub.data)
	}
	// An unset ctx still falls back to the legacy default tenant (env-groups parity).
	if got, err := s.Get(context.Background(), "services/web/env"); err != nil || len(got) != 0 {
		t.Fatalf("legacy default tenant should be empty here: %+v err=%v", got, err)
	}
}

// TestReadMap_DoesNotClaimAmbiguousLegacyData proves a same-named service cannot
// acquire a legacy default-tenant value merely by reading first. Those paths
// require an explicit ownership-backed operator migration.
func TestReadMap_DoesNotClaimAmbiguousLegacyData(t *testing.T) {
	store := newTenantFakeSecretStore()
	svc := newService(store)

	// Pre-w7/m70 data lives under the legacy default tenant (no ctx tenant set).
	legacyCtx := withTenant(context.Background(), baoTenant)
	if err := store.Put(legacyCtx, "services/web/env", map[string]string{"TOKEN": "legacy"}); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	ctx := withTenant(context.Background(), "tea-a")

	got, err := svc.readMap(ctx, "services/web/env")
	if err != nil || len(got) != 0 {
		t.Fatalf("tenant read must not surface legacy data: %+v err=%v", got, err)
	}
	if _, ok := store.m["tea-a/services/web/env"]; ok {
		t.Fatalf("tenant path must not be populated from ambiguous legacy data: %+v", store.m)
	}
	if store.m["default/services/web/env"]["TOKEN"] != "legacy" {
		t.Fatalf("legacy path must remain for explicit migration: %+v", store.m)
	}
	got2, err := svc.readMap(ctx, "services/web/env")
	if err != nil || len(got2) != 0 {
		t.Fatalf("second readMap: %+v err=%v", got2, err)
	}
}

func TestSeedEnvVarsScopesSameNamedServicesByTenant(t *testing.T) {
	store := newTenantFakeSecretStore()
	svcA := newService(store, tenantApp("web", "tea-a"))
	svcB := newService(store, tenantApp("web", "tea-b"))

	if err := svcA.SeedEnvVars(context.Background(), "web", map[string]string{"TOKEN": "secret-a"}, nil); err != nil {
		t.Fatalf("seed tea-a: %v", err)
	}
	if err := svcB.SeedEnvVars(context.Background(), "web", map[string]string{"TOKEN": "secret-b"}, nil); err != nil {
		t.Fatalf("seed tea-b: %v", err)
	}
	if got := store.m["tea-a/services/web/env"]["TOKEN"]; got != "secret-a" {
		t.Fatalf("tea-a TOKEN = %q", got)
	}
	if got := store.m["tea-b/services/web/env"]["TOKEN"]; got != "secret-b" {
		t.Fatalf("tea-b TOKEN = %q", got)
	}
	if _, ok := store.m["default/services/web/env"]; ok {
		t.Fatalf("blueprint seeding wrote the legacy shared path: %+v", store.m)
	}
}

// TestWorkspacePurger_PurgeApp_UsesPublicServiceName is the F15 regression: a
// store-managed App's metadata.name is the tenant-prefixed CRName, but live
// secrets are keyed by the public LabelServiceName. PurgeApp must delete the
// public-name path, not the CR-name path, or the secrets survive the delete.
func TestWorkspacePurger_PurgeApp_UsesPublicServiceName(t *testing.T) {
	store := newTenantFakeSecretStore()
	// metadata.name = CRName("tea-a","web") = "tea-a-web"; public name = "web".
	a := tenantApp("tea-a-web", "tea-a")
	a.Labels[core.LabelServiceName] = "web"
	svc := newService(store, a)

	// Live secret keyed by the PUBLIC name under tea-a's tenant path.
	tenantCtx := withTenant(context.Background(), "tea-a")
	if err := store.Put(tenantCtx, "services/web/env", map[string]string{"TOKEN": "live"}); err != nil {
		t.Fatalf("seed live: %v", err)
	}
	if err := store.Put(tenantCtx, "services/web/files", map[string]string{"cert.pem": "live"}); err != nil {
		t.Fatalf("seed live files: %v", err)
	}

	purger := &WorkspacePurger{Service: svc}
	if err := purger.PurgeApp(context.Background(), a); err != nil {
		t.Fatalf("PurgeApp: %v", err)
	}
	// The public-name tenant paths must be gone (and the CR-name path was never
	// written, so there is nothing to leak on recreation).
	if _, ok := store.m["tea-a/services/web/env"]; ok {
		t.Fatalf("public-name env path should be purged: %+v", store.m)
	}
	if _, ok := store.m["tea-a/services/web/files"]; ok {
		t.Fatalf("public-name files path should be purged: %+v", store.m)
	}
}

// TestWorkspacePurger_PurgeApp_PreservesAmbiguousLegacyPath ensures deleting
// one tenant's same-named App cannot destroy data awaiting explicit migration.
func TestWorkspacePurger_PurgeApp_PreservesAmbiguousLegacyPath(t *testing.T) {
	store := newTenantFakeSecretStore()
	a := tenantApp("web", "tea-a")
	svc := newService(store, a)

	// Legacy data still under the default tenant (not yet migrated).
	legacyCtx := withTenant(context.Background(), baoTenant)
	if err := store.Put(legacyCtx, "services/web/env", map[string]string{"TOKEN": "legacy"}); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}

	purger := &WorkspacePurger{Service: svc}
	if err := purger.PurgeApp(context.Background(), a); err != nil {
		t.Fatalf("PurgeApp: %v", err)
	}
	if store.m["default/services/web/env"]["TOKEN"] != "legacy" {
		t.Fatalf("ambiguous legacy path must remain: %+v", store.m)
	}
}
