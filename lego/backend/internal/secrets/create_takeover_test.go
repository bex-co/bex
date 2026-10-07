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
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestACreateTakesOverItsDeletedNamesakesSecret (w5/128): a service recreated
// under a deleted service's name, before garbage collection reached the old
// service's Secret, takes that Secret over. The create succeeds, and the
// Secret holds only the new files at the new store's revision, not the
// predecessor's 7, which would outrank every later write. The new App owns it
// once created.
func TestACreateTakesOverItsDeletedNamesakesSecret(t *testing.T) {
	ctx := context.Background()
	svc := newService(newVersionedFakeSecretStore(), predecessorSecret(filesProjection, "uid-deleted", map[string]string{"stale": "old"}))
	seeder := NewCreateSecretsSeeder(svc)
	app := sampleApp("web")
	if err := seeder.PrepareCreateSecrets(ctx, "web", app, []core.SecretFile{{Name: "token", Content: "new"}}, nil); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	sec := getSecret(t, svc.Client, "web-files")
	if len(sec.OwnerReferences) != 0 || len(sec.Data) != 1 || string(sec.Data["token"]) != "new" {
		t.Fatalf("prepared Secret = owners %v, files %v; want ownerless with only the new file", sec.OwnerReferences, keysOf(sec.Data))
	}
	if revision, _ := projectedRevision(sec, filesProjection); revision != committedAt(1) {
		t.Fatalf("prepared Secret revision = %+v, want the new store's first", revision)
	}
	if err := svc.Client.Create(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := seeder.CommitCreateSecrets(ctx, "web", app); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if sec := getSecret(t, svc.Client, "web-files"); len(sec.OwnerReferences) != 1 || sec.OwnerReferences[0].UID != app.UID {
		t.Fatalf("committed Secret owners = %v, want the new App", sec.OwnerReferences)
	}
}

// TestACreateRefusesASecretAnotherServiceHolds (w5/128): a Secret a live App
// controls, or one another create prepared and has not adopted yet, refuses
// the create with a conflict, not a 500. The Secret is left as it was, and the
// store write is rolled back.
func TestACreateRefusesASecretAnotherServiceHolds(t *testing.T) {
	live := sampleApp("web")
	theirs := map[string]string{"theirs": "x"}
	// A deleted namesake's Secret that a finalizer holds in deletion would
	// vanish under the create that took it over.
	terminating := predecessorSecret(filesProjection, "uid-deleted", theirs)
	terminating.Finalizers = []string{"example.test/hold"}
	terminating.DeletionTimestamp = &metav1.Time{Time: time.Unix(1_000_000, 0)}
	otherName := predecessorSecret(filesProjection, "uid-other", theirs)
	otherName.OwnerReferences[0].Name = "other"
	for holder, objs := range map[string][]client.Object{
		"a live App":                   {live, predecessorSecret(filesProjection, live.UID, theirs)},
		"another create":               {projectedSecret(filesProjection, committedAt(1), theirs)},
		"a deleted namesake, deleting": {terminating},
		"an App of another name":       {otherName},
	} {
		t.Run(holder, func(t *testing.T) {
			store := newFakeSecretStore()
			svc := newService(store, objs...)
			err := NewCreateSecretsSeeder(svc).PrepareCreateSecrets(context.Background(), "web", sampleApp("web"), []core.SecretFile{{Name: "token", Content: "new"}}, nil)
			if !errors.Is(err, core.ErrConflict) {
				t.Fatalf("prepare = %v, want a conflict", err)
			}
			if sec := getSecret(t, svc.Client, "web-files"); string(sec.Data["theirs"]) != "x" || len(sec.Data) != 1 {
				t.Errorf("the held Secret changed: files %v", keysOf(sec.Data))
			}
			if _, ok := store.m[filesPath("web")]; ok {
				t.Error("the refused create left its files in the store")
			}
		})
	}
}

// failingDeletes is a store whose deletes fail, naming its backend.
type failingDeletes struct{ *fakeSecretStore }

func (failingDeletes) Delete(context.Context, string) error {
	return errors.New("delete http://bao.internal:8200/v1/secret/data/services/web/files: connection refused")
}

// TestARefusedCreateHidesItsRollbackFailure (w5/128): the refusal is the
// caller's whole answer. A rollback that fails is logged, never joined into it,
// since its text names the store's internal address.
func TestARefusedCreateHidesItsRollbackFailure(t *testing.T) {
	svc := newService(failingDeletes{newFakeSecretStore()}, projectedSecret(filesProjection, committedAt(1), map[string]string{"theirs": "x"}))
	err := NewCreateSecretsSeeder(svc).PrepareCreateSecrets(context.Background(), "web", sampleApp("web"), []core.SecretFile{{Name: "token", Content: "new"}}, nil)
	if !errors.Is(err, core.ErrConflict) || err.Error() != errSecretNameHeld.Error() {
		t.Fatalf("prepare = %q, want the refusal alone", err)
	}
}

// TestAFailedCreatePrepareRemovesOnlyWhatItWrote (w5/128): when the env leg is
// refused after the files leg wrote, prepare removes the files leg, and leaves
// the env Secret it was refused on, which a live App holds.
func TestAFailedCreatePrepareRemovesOnlyWhatItWrote(t *testing.T) {
	ctx := context.Background()
	live := sampleApp("web")
	store := newFakeSecretStore()
	svc := newService(store, live, predecessorSecret(envProjection, live.UID, map[string]string{"THEIRS": "x"}))
	err := NewCreateSecretsSeeder(svc).PrepareCreateSecrets(ctx, "web", sampleApp("web"),
		[]core.SecretFile{{Name: "token", Content: "new"}}, map[string]string{"MESSAGE": "new"})
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("prepare = %v, want the env leg's conflict", err)
	}
	if err := svc.Client.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web-files"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Errorf("the files leg's Secret outlived the failed prepare: %v", err)
	}
	if _, ok := store.m[filesPath("web")]; ok {
		t.Error("the files leg's store map outlived the failed prepare")
	}
	if sec := getSecret(t, svc.Client, "web-env"); string(sec.Data["THEIRS"]) != "x" {
		t.Errorf("the live App's env Secret changed: %v", keysOf(sec.Data))
	}
}

func keysOf(data map[string][]byte) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	return keys
}
