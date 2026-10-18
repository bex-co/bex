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

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestWorkspacePurgerDeletesOnlyTheGivenWorkspacesGroups(t *testing.T) {
	store := newFakeStore()
	owned := id.New(id.EnvGroup)
	other := id.New(id.EnvGroup)
	def := id.New(id.EnvGroup)
	orphan := id.New(id.EnvGroup) // a crashed create's maps: no row
	now := time.Unix(1_000_000, 0).UTC()
	for gid, owner := range map[string]string{owned: "tea-a", other: "tea-b", def: core.DefaultTenant} {
		groupsOf(store).Seed(storeEnvGroup(gid, owner, now))
		_ = store.Put(groupCtx(testCtx(), owner), envPath(gid), map[string]string{"TOKEN": "secret"})
		_ = store.Put(groupCtx(testCtx(), owner), filesPath(gid), map[string]string{"cert.pem": "secret"})
	}
	_ = store.Put(groupCtx(testCtx(), "tea-a"), envPath(orphan), map[string]string{"TOKEN": "secret"})
	objects := []client.Object{}
	// Projection Secrets live in their OWNING workspace's namespace (ADR043);
	// the default workspace's live in the shared one.
	for gid, ns := range map[string]string{owned: "tea-a", other: "tea-b", def: "default"} {
		objects = append(objects,
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: envSecretName(gid), Namespace: ns}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: filesSecretName(gid), Namespace: ns}},
		)
	}
	svc := newService(store, objects...)
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(testCtx(), "tea-a"); err != nil {
		t.Fatalf("PurgeWorkspace tea-a: %v", err)
	}
	assertGroupAbsent(t, svc, store, owned, "tea-a")
	if env, _ := store.Get(groupCtx(testCtx(), "tea-a"), envPath(orphan)); len(env) != 0 {
		t.Fatalf("an unreferenced map under the purged workspace survived: %v", env)
	}
	assertGroupPresent(t, svc, store, other, "tea-b")
	assertGroupPresent(t, svc, store, def, core.DefaultTenant)

	if err := purger.PurgeWorkspace(testCtx(), core.DefaultTenant); err != nil {
		t.Fatalf("PurgeWorkspace default: %v", err)
	}
	assertGroupAbsent(t, svc, store, def, core.DefaultTenant)
	assertGroupPresent(t, svc, store, other, "tea-b")

	if err := purger.PurgeWorkspace(testCtx(), "tea-a"); err != nil {
		t.Fatalf("second purge should be idempotent: %v", err)
	}
}

func storeEnvGroup(gid, owner string, at time.Time) store.EnvGroup {
	return store.EnvGroup{ID: gid, TenantID: owner, Name: gid, CreatedAt: at, UpdatedAt: at,
		EnvKeys: []string{"TOKEN"}, FileNames: []string{"cert.pem"}}
}

func TestWorkspacePurgerNilStoreIsNoOp(t *testing.T) {
	purger := &WorkspacePurger{Service: newService(nil)}
	if err := purger.PurgeWorkspace(testCtx(), "tea-a"); err != nil {
		t.Fatalf("nil-store purge: %v", err)
	}
}

func assertGroupAbsent(t *testing.T, svc *Service, store *fakeStore, gid, ns string) {
	t.Helper()
	if _, ok := groupsOf(store).Row(gid); ok {
		t.Fatalf("group %s row survived", gid)
	}
	if env, _ := store.Get(groupCtx(testCtx(), ns), envPath(gid)); len(env) != 0 {
		t.Fatalf("group %s maps survived: %+v", gid, env)
	}
	for _, name := range []string{envSecretName(gid), filesSecretName(gid)} {
		var secret corev1.Secret
		err := svc.Client.Get(testCtx(), client.ObjectKey{Namespace: ns, Name: name}, &secret)
		if !apierrors.IsNotFound(err) {
			t.Fatalf("projection Secret %s survived: %v", name, err)
		}
	}
}

func assertGroupPresent(t *testing.T, svc *Service, store *fakeStore, gid, ns string) {
	t.Helper()
	if _, ok := groupsOf(store).Row(gid); !ok {
		t.Fatalf("group %s was deleted", gid)
	}
	if env, _ := store.Get(groupCtx(testCtx(), ns), envPath(gid)); len(env) == 0 {
		t.Fatalf("group %s lost its maps", gid)
	}
	for _, name := range []string{envSecretName(gid), filesSecretName(gid)} {
		var secret corev1.Secret
		if err := svc.Client.Get(testCtx(), client.ObjectKey{Namespace: ns, Name: name}, &secret); err != nil {
			t.Fatalf("projection Secret %s was deleted: %v", name, err)
		}
	}
}

// A purger with OpenBao but no metadata store is a wiring error: it refuses,
// rather than report a workspace purged while its values remain.
func TestWorkspacePurgerWithoutGroupsFailsClosed(t *testing.T) {
	svc := newService(newFakeStore())
	svc.Groups = nil
	if err := (&WorkspacePurger{Service: svc}).PurgeWorkspace(testCtx(), "tea-a"); err == nil {
		t.Fatal("purge without the metadata store succeeded")
	}
}
