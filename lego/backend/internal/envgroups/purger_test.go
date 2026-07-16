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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
)

func TestWorkspacePurgerDeletesOnlyTheGivenWorkspacesGroups(t *testing.T) {
	store := newFakeStore()
	owned := id.New(id.EnvGroup)
	other := id.New(id.EnvGroup)
	legacy := id.New(id.EnvGroup)
	for gid, owner := range map[string]string{owned: "tea-a", other: "tea-b", legacy: ""} {
		if err := store.Put(context.Background(), metaPath(gid), map[string]string{"name": gid, "workspace": owner}); err != nil {
			t.Fatal(err)
		}
		_ = store.Put(context.Background(), envPath(gid), map[string]string{"TOKEN": "secret"})
		_ = store.Put(context.Background(), filesPath(gid), map[string]string{"cert.pem": "secret"})
	}
	objects := []client.Object{}
	for _, gid := range []string{owned, other, legacy} {
		objects = append(objects,
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: envSecretName(gid), Namespace: "default"}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: filesSecretName(gid), Namespace: "default"}},
		)
	}
	svc := newService(store, objects...)
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("PurgeWorkspace tea-a: %v", err)
	}
	assertGroupAbsent(t, svc, store, owned)
	assertGroupPresent(t, svc, store, other)
	assertGroupPresent(t, svc, store, legacy)

	// Ownerless legacy groups have the same deterministic owner as readMeta's
	// lazy migration, so deleting the bootstrap workspace cannot strand them.
	if err := purger.PurgeWorkspace(context.Background(), core.DefaultTenant); err != nil {
		t.Fatalf("PurgeWorkspace default: %v", err)
	}
	assertGroupAbsent(t, svc, store, legacy)
	assertGroupPresent(t, svc, store, other)

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("second purge should be idempotent: %v", err)
	}
}

func TestWorkspacePurgerNilStoreIsNoOp(t *testing.T) {
	purger := &WorkspacePurger{Service: newService(nil)}
	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("nil-store purge: %v", err)
	}
}

func assertGroupAbsent(t *testing.T, svc *Service, store *fakeStore, gid string) {
	t.Helper()
	if raw, _ := store.Get(context.Background(), metaPath(gid)); len(raw) != 0 {
		t.Fatalf("group %s meta survived: %+v", gid, raw)
	}
	for _, name := range []string{envSecretName(gid), filesSecretName(gid)} {
		var secret corev1.Secret
		err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &secret)
		if !apierrors.IsNotFound(err) {
			t.Fatalf("projection Secret %s survived: %v", name, err)
		}
	}
}

func assertGroupPresent(t *testing.T, svc *Service, store *fakeStore, gid string) {
	t.Helper()
	if raw, _ := store.Get(context.Background(), metaPath(gid)); len(raw) == 0 {
		t.Fatalf("group %s was deleted", gid)
	}
	for _, name := range []string{envSecretName(gid), filesSecretName(gid)} {
		var secret corev1.Secret
		if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &secret); err != nil {
			t.Fatalf("projection Secret %s was deleted: %v", name, err)
		}
	}
}
