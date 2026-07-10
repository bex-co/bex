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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func tenantApp(name, tenantID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{core.LabelTenant: tenantID}
	return a
}

func TestWorkspacePurger_DeletesOnlyTheGivenTenantsSecrets(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, tenantApp("web", "tea-a"), tenantApp("other", "tea-b"))
	if err := store.Put(context.Background(), envPath("web"), map[string]string{"FOO": "bar"}); err != nil {
		t.Fatalf("seed web env: %v", err)
	}
	if err := store.Put(context.Background(), filesPath("web"), map[string]string{"cert.pem": "..."}); err != nil {
		t.Fatalf("seed web files: %v", err)
	}
	if err := store.Put(context.Background(), envPath("other"), map[string]string{"BAZ": "qux"}); err != nil {
		t.Fatalf("seed other env: %v", err)
	}
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("PurgeWorkspace: %v", err)
	}

	if env, err := store.Get(context.Background(), envPath("web")); err != nil || len(env) != 0 {
		t.Fatalf("web env after purge = %+v, err=%v; want empty", env, err)
	}
	if files, err := store.Get(context.Background(), filesPath("web")); err != nil || len(files) != 0 {
		t.Fatalf("web files after purge = %+v, err=%v; want empty", files, err)
	}
	// tea-b's "other" secrets are untouched.
	if env, err := store.Get(context.Background(), envPath("other")); err != nil || len(env) != 1 {
		t.Fatalf("other env after purge = %+v, err=%v; want untouched", env, err)
	}
}

func TestWorkspacePurger_NilStoreIsANoOp(t *testing.T) {
	svc := newService(nil, tenantApp("web", "tea-a"))
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("PurgeWorkspace with secrets disabled should be a no-op, got: %v", err)
	}
}

func TestWorkspacePurger_SecondPurgeIsANoOp(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, tenantApp("web", "tea-a"))
	if err := store.Put(context.Background(), envPath("web"), map[string]string{"FOO": "bar"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("first purge: %v", err)
	}
	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("second purge (already-gone) should be a no-op, got: %v", err)
	}
}
