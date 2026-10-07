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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

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
	// The takeover dates its own claim: the Secret keeps the deleted
	// service's creationTimestamp (w5/135).
	if got := sec.Annotations[preparedAtAnnotation]; got != fixedNow().Format(time.RFC3339) {
		t.Fatalf("prepared Secret claimed at %q, want now", got)
	}
	if err := svc.Client.Create(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := seeder.CommitCreateSecrets(ctx, "web", app); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if sec := getSecret(t, svc.Client, "web-files"); len(sec.OwnerReferences) != 1 || sec.OwnerReferences[0].UID != app.UID {
		t.Fatalf("committed Secret owners = %v, want the new App", sec.OwnerReferences)
	} else if _, dated := sec.Annotations[preparedAtAnnotation]; dated {
		t.Fatal("the adopted Secret still carries its claim's date")
	}
}

// preparedSecret is a create's ownerless files Secret, holding data, claimed
// at claimedAt (unset when zero), the object itself created at createdAt.
func preparedSecret(claimedAt, createdAt time.Time, data map[string]string) *corev1.Secret {
	sec := projectedSecret(filesProjection, committedAt(1), data)
	sec.CreationTimestamp = metav1.NewTime(createdAt)
	if !claimedAt.IsZero() {
		setPreparedAt(sec, claimedAt)
	}
	return sec
}

// TestACreateTakesOverAnAbandonedPreparation (w5/135): a create that crashed
// between prepare and commit left its Secret ownerless for good, so every
// later create of the name was refused as "still being created", whatever
// the wait. A preparation claimed longer ago than any create runs is now
// taken over, with the map the crashed create left. One that may still be
// running is not, nor one at a live App's name, nor an ownerless Secret that
// is no create's.
func TestACreateTakesOverAnAbandonedPreparation(t *testing.T) {
	long, recent := fixedNow().Add(-time.Hour), fixedNow().Add(-time.Minute)
	theirs := map[string]string{"theirs": "x"}
	notAProjection := preparedSecret(time.Time{}, long, theirs) // an env group's, say
	delete(notAProjection.Annotations, filesProjection.annotation)
	controller := false
	referenced := preparedSecret(long, long, theirs)
	referenced.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "holder", UID: "uid-holder", Controller: &controller}}
	for name, tc := range map[string]struct {
		objs      []client.Object
		takenOver bool
	}{
		"abandoned":                          {[]client.Object{preparedSecret(long, long, theirs)}, true},
		"abandoned before claims were dated": {[]client.Object{preparedSecret(time.Time{}, long, theirs)}, true},
		"still preparing":                    {[]client.Object{preparedSecret(recent, recent, theirs)}, false},
		// A takeover's claim keeps the Secret's first creationTimestamp.
		"claimed recently, created long ago": {[]client.Object{preparedSecret(recent, long, theirs)}, false},
		"abandoned at a live App's name":     {[]client.Object{sampleApp("web"), preparedSecret(long, long, theirs)}, false},
		"old, but no create's":               {[]client.Object{notAProjection}, false},
		"old, with another owner":            {[]client.Object{referenced}, false},
	} {
		t.Run(name, func(t *testing.T) {
			store := newFakeSecretStore()
			store.m[filesPath("web")] = map[string]string{"theirs": "x"}
			svc := newService(store, tc.objs...)
			err := NewCreateSecretsSeeder(svc).PrepareCreateSecrets(context.Background(), "web", sampleApp("web"), []core.SecretFile{{Name: "token", Content: "new"}}, nil)
			sec := getSecret(t, svc.Client, "web-files")
			if tc.takenOver {
				if err != nil || len(sec.Data) != 1 || string(sec.Data["token"]) != "new" || sec.Annotations[preparedAtAnnotation] != fixedNow().Format(time.RFC3339) {
					t.Fatalf("prepare = %v with files %v, want the abandoned Secret taken over and claimed now", err, keysOf(sec.Data))
				}
				if got := store.m[filesPath("web")]; len(got) != 1 || got["token"] != "new" {
					t.Fatalf("store map = %v, want the crashed create's replaced", got)
				}
				return
			}
			if !errors.Is(err, core.ErrConflict) || string(sec.Data["theirs"]) != "x" || len(sec.OwnerReferences) != len(getSecretOwners(tc.objs)) {
				t.Fatalf("prepare = %v with files %v, want a conflict and the Secret untouched", err, keysOf(sec.Data))
			}
		})
	}
}

// getSecretOwners is the owner references the fixture's files Secret had.
func getSecretOwners(objs []client.Object) []metav1.OwnerReference {
	for _, obj := range objs {
		if sec, ok := obj.(*corev1.Secret); ok {
			return sec.OwnerReferences
		}
	}
	return nil
}

// TestACreateRefusesASecretAnotherServiceHolds (w5/128): a Secret a live App
// controls, or one another create prepared and has not adopted yet, refuses
// the create with a conflict, not a 500. The Secret is left as it was, and so
// is the holder's store map: the refused create used to write over it and
// then delete it, leaving the holder's Secret with no source of truth
// (w5/134).
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
		"a live App": {live, predecessorSecret(filesProjection, live.UID, theirs)},
		// No claim date and no creation time: of unknown age, never abandoned.
		"another create":               {projectedSecret(filesProjection, committedAt(1), theirs)},
		"a deleted namesake, deleting": {terminating},
		"an App of another name":       {otherName},
	} {
		t.Run(holder, func(t *testing.T) {
			store := newFakeSecretStore()
			store.m[filesPath("web")] = map[string]string{"theirs": "x"}
			svc := newService(store, objs...)
			err := NewCreateSecretsSeeder(svc).PrepareCreateSecrets(context.Background(), "web", sampleApp("web"), []core.SecretFile{{Name: "token", Content: "new"}}, nil)
			if !errors.Is(err, core.ErrConflict) {
				t.Fatalf("prepare = %v, want a conflict", err)
			}
			if sec := getSecret(t, svc.Client, "web-files"); string(sec.Data["theirs"]) != "x" || len(sec.Data) != 1 {
				t.Errorf("the held Secret changed: files %v", keysOf(sec.Data))
			}
			if got := store.m[filesPath("web")]; len(got) != 1 || got["theirs"] != "x" {
				t.Errorf("the refused create changed the holder's store map: %v", got)
			}
		})
	}
}

// TestAFailedPrepareReleasesTheName (w5/134): prepare claims the name with its
// Secret, then writes the store and stamps the revision it committed. When the
// write or the stamp fails, both halves go, so a retry can create the service.
func TestAFailedPrepareReleasesTheName(t *testing.T) {
	refused := errors.New("refused")
	for name, setup := range map[string]func() (core.SecretKV, func(*Service), map[string]map[string]string){
		"store write": func() (core.SecretKV, func(*Service), map[string]map[string]string) {
			store := newFakeSecretStore()
			store.failPut = refused
			return store, func(*Service) {}, store.m
		},
		"versioned store write": func() (core.SecretKV, func(*Service), map[string]map[string]string) {
			store := newVersionedFakeSecretStore()
			store.failCASCall = 1
			return store, func(*Service) {}, store.m
		},
		"revision stamp": func() (core.SecretKV, func(*Service), map[string]map[string]string) {
			store := newVersionedFakeSecretStore()
			return store, func(svc *Service) {
				svc.Client = interceptor.NewClient(svc.Client.(client.WithWatch), interceptor.Funcs{
					Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
						return refused
					},
				})
			}, store.m
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			store, wire, maps := setup()
			svc := newService(store)
			wire(svc)
			if err := NewCreateSecretsSeeder(svc).PrepareCreateSecrets(ctx, "web", sampleApp("web"), []core.SecretFile{{Name: "token", Content: "new"}}, nil); err == nil {
				t.Fatal("prepare succeeded")
			}
			if err := svc.Client.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web-files"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
				t.Errorf("the failed prepare kept its claim on the name: %v", err)
			}
			if _, ok := maps[filesPath("web")]; ok {
				t.Error("the failed prepare left its map in the store")
			}
		})
	}
}

// claimCheckingStore records, at each store delete, whether the files Secret
// still held the name.
type claimCheckingStore struct {
	*fakeSecretStore
	cl           client.Client
	heldAtDelete []bool
}

func (c *claimCheckingStore) Delete(ctx context.Context, path string) error {
	c.heldAtDelete = append(c.heldAtDelete, c.cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web-files"}, &corev1.Secret{}) == nil)
	return c.fakeSecretStore.Delete(ctx, path)
}

// TestAnAbortDeletesTheMapWhileStillHoldingTheName (w5/134): an abort released
// the name before it deleted the store map, so a create that claimed the name
// in between could have its fresh map deleted. The map now goes first.
func TestAnAbortDeletesTheMapWhileStillHoldingTheName(t *testing.T) {
	live := sampleApp("web")
	store := &claimCheckingStore{fakeSecretStore: newFakeSecretStore()}
	svc := newService(store, live, predecessorSecret(envProjection, live.UID, map[string]string{"THEIRS": "x"}))
	store.cl = svc.Client
	err := NewCreateSecretsSeeder(svc).PrepareCreateSecrets(context.Background(), "web", sampleApp("web"),
		[]core.SecretFile{{Name: "token", Content: "new"}}, map[string]string{"MESSAGE": "new"})
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("prepare = %v, want the env leg's conflict", err)
	}
	if len(store.heldAtDelete) != 1 || !store.heldAtDelete[0] {
		t.Fatalf("the files leg's map was deleted with the name held = %v, want held", store.heldAtDelete)
	}
}

// failingDeletes is a store whose deletes fail, naming its backend.
type failingDeletes struct{ *fakeSecretStore }

func (failingDeletes) Delete(context.Context, string) error {
	return errors.New("delete http://bao.internal:8200/v1/secret/data/services/web/files: connection refused")
}

// TestARefusedCreateHidesItsRollbackFailure (w5/128): the refusal is the
// caller's whole answer. An env leg refused after the files leg wrote rolls
// the files leg back; a rollback that fails is logged, never joined into the
// answer, since its text names the store's internal address.
func TestARefusedCreateHidesItsRollbackFailure(t *testing.T) {
	live := sampleApp("web")
	svc := newService(failingDeletes{newFakeSecretStore()}, live, predecessorSecret(envProjection, live.UID, map[string]string{"THEIRS": "x"}))
	err := NewCreateSecretsSeeder(svc).PrepareCreateSecrets(context.Background(), "web", sampleApp("web"),
		[]core.SecretFile{{Name: "token", Content: "new"}}, map[string]string{"MESSAGE": "new"})
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
