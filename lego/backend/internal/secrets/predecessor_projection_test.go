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
	"fmt"
	"maps"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// predecessorSecret is kind's Secret for "web" as a deleted namesake left it:
// controlled by its App's UID and stamped with its store's revision 7.
func predecessorSecret(kind projectionKind, uid types.UID, data map[string]string) *corev1.Secret {
	sec := projectedSecret(kind, committedAt(7), data)
	controller := true
	sec.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "app.bex.co/v1alpha1", Kind: "App", Name: "web", UID: uid, Controller: &controller,
	}}
	return sec
}

// TestARecreatedServiceReplacesItsPredecessorsProjection (w5/118): a service
// deleted and recreated under the same name, before garbage collection reaches
// the old App's Secret, found it stamped with the old store's revision. The
// new store restarts at version 1, so its first write lost to the dead
// service's data, and the files case mounted the predecessor's files. A delete
// facing it ran out of retries. The new service now replaces the Secret and
// owns it.
func TestARecreatedServiceReplacesItsPredecessorsProjection(t *testing.T) {
	for _, c := range projectionCases {
		t.Run(c.name, func(t *testing.T) {
			for name, step := range map[string]struct {
				stored map[string]string
				run    func(*Service) error
			}{
				"its first write": {nil, func(svc *Service) error { return c.write(svc, "new") }},
				"a delete":        {map[string]string{"keep": "1", "gone": "1"}, func(svc *Service) error { return c.delete(svc, "gone") }},
			} {
				t.Run(name, func(t *testing.T) {
					store := newVersionedFakeSecretStore()
					if step.stored != nil {
						store.m[c.path], store.versions[c.path] = maps.Clone(step.stored), 1
					}
					app := sampleApp("web")
					svc := newService(store, app, predecessorSecret(c.kind, "uid-old", map[string]string{"old": "secret"}))

					if err := step.run(svc); err != nil {
						t.Fatal(err)
					}
					c.expectMatchesStore(t, svc, store)
					if owner := metav1.GetControllerOf(getSecret(t, svc.Client, c.kind.secretName("web"))); owner == nil || owner.UID != app.UID {
						t.Fatalf("the Secret is controlled by %+v, want the recreated App", owner)
					}
				})
			}
		})
	}
}

// TestARequestThatSpansARecreateLeavesTheNewServicesSecretAlone (w5/118): a
// write whose App was deleted and recreated under the same name since the
// request read it must not take the new service's Secret over as its
// predecessor's. It refuses with a conflict instead.
func TestARequestThatSpansARecreateLeavesTheNewServicesSecretAlone(t *testing.T) {
	c := projectionCases[1]
	svc, store := c.setup(nil, map[string]string{"f": "1"})
	var recreated error
	store.afterGet = func() {
		recreated = func() error {
			ctx := context.Background()
			if err := svc.Client.Delete(ctx, sampleApp("web")); err != nil {
				return err
			}
			successor := sampleApp("web")
			successor.UID = "uid-successor"
			if err := svc.Client.Create(ctx, successor); err != nil {
				return err
			}
			sec := getSecret(t, svc.Client, c.kind.secretName("web"))
			sec.OwnerReferences = predecessorSecret(c.kind, successor.UID, nil).OwnerReferences
			sec.Data = map[string][]byte{"successor": []byte("1")}
			return svc.Client.Update(ctx, sec)
		}()
	}

	err := c.write(svc, "stale")
	if recreated != nil {
		t.Fatalf("recreate: %v", recreated)
	}
	if !errors.Is(err, core.ErrConflict) || err.Error() != core.ErrServiceReplaced.Error() {
		t.Fatalf("the stale write = %v, want only the conflict that the service changed", err)
	}
	sec := getSecret(t, svc.Client, c.kind.secretName("web"))
	if owner := metav1.GetControllerOf(sec); owner == nil || owner.UID != "uid-successor" || string(sec.Data["successor"]) != "1" || len(sec.Data) != 1 {
		t.Fatalf("the new service's Secret = %v controlled by %+v, want it untouched", sec.Data, owner)
	}
}

// TestACASRollbackRemovesAPredecessorsSecretItReplaced (w5/118): a CAS save
// that replaced a deleted namesake's Secret and then failed removes the
// Secret rather than restoring it, as for one it created: the Secret was
// never this service's, and the predecessor's data must not come back.
func TestACASRollbackRemovesAPredecessorsSecretItReplaced(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
	revision := encodeEnvRevision(0)
	failing := &patchCountingClient{
		Client: fakeClient(sampleApp("web"), predecessorSecret(envProjection, "uid-old", map[string]string{"OLD": "secret"})),
		fail:   errors.New("injected App patch failure"),
	}
	svc := &Service{Base: &core.Base{Client: failing, Namespace: "default", Clock: fixedNow}, Store: store}

	if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, ExpectedEnvRevision: &revision,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "failed-writer-secret"}},
	}); err == nil {
		t.Fatal("a save whose App patch failed succeeded")
	}
	sec := &corev1.Secret{}
	if err := failing.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web-env"}, sec); !apierrors.IsNotFound(err) {
		t.Fatalf("the replaced predecessor's Secret = %v (%v), want it removed", sec.Data, err)
	}
}

// TestADeletedServicesWriteLeavesItsNamesakesPreparedSecretAlone (w5/147): a
// write in flight for a deleted service, landing after a create under its name
// prepared the projection Secret, made the dead App that Secret's controller
// and replaced the create's data. Garbage collection then removed the new
// service's Secret, or the new service adopted the dead one's values. The
// write now refuses as the service having changed, and the prepared Secret
// stays ownerless with the create's data, its revision stamped or not; an
// unstamped one still takes the create's stamp.
func TestADeletedServicesWriteLeavesItsNamesakesPreparedSecretAlone(t *testing.T) {
	for _, c := range projectionCases {
		for _, stamped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, stamped %v", c.name, stamped), func(t *testing.T) {
				svc, store := c.setup(nil, map[string]string{"old": "1"})
				ctx := context.Background()
				name := c.kind.secretName("web")
				prepared := map[string]string{"new": "1"}
				claim := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Data: envBytes(prepared)}
				var namesake error
				store.afterGet = func() {
					namesake = func() error {
						// The service is deleted, garbage collection removes its
						// Secret, and a create under its name claims the name.
						if err := svc.Client.Delete(ctx, sampleApp("web")); err != nil {
							return err
						}
						if err := svc.Client.Delete(ctx, getSecret(t, svc.Client, name)); err != nil {
							return err
						}
						setPreparedAt(claim, svc.Now())
						if err := svc.createPreparedSecret(ctx, sampleApp("web"), c.kind, claim); err != nil || !stamped {
							return err
						}
						return svc.stampPreparedRevision(ctx, claim, c.kind, committedAt(1))
					}()
				}

				err := c.write(svc, "stale")
				if namesake != nil {
					t.Fatalf("the namesake's create: %v", namesake)
				}
				if !errors.Is(err, core.ErrConflict) || err.Error() != core.ErrServiceReplaced.Error() {
					t.Fatalf("the deleted service's write = %v, want only the conflict that the service changed", err)
				}
				if sec := getSecret(t, svc.Client, name); len(sec.OwnerReferences) != 0 || !equalSecretData(sec.Data, prepared) {
					t.Fatalf("the prepared Secret holds %v, owned by %+v, want the create's %v and no owner", sec.Data, sec.OwnerReferences, prepared)
				}
				if !stamped {
					if err := svc.stampPreparedRevision(ctx, claim, c.kind, committedAt(1)); err != nil {
						t.Fatalf("the create's stamp after the deleted service's write: %v", err)
					}
				}
			})
		}
	}
}

// TestAWriteRetriedAfterItsServiceWasRecreatedLeavesTheNamesakeAlone
// (w5/157): a write whose service was deleted and recreated between its
// projection and its App patch lost the patch's lock, re-read the App by name
// and ran again as the namesake. The namesake's Secret took the dead write's
// map and the write answered OK. It now refuses as the service having changed,
// restores nothing into the store paths the namesake holds now, and leaves the
// namesake's Secret, store map and App as its create left them.
func TestAWriteRetriedAfterItsServiceWasRecreatedLeavesTheNamesakeAlone(t *testing.T) {
	for _, c := range projectionCases {
		t.Run(c.name, func(t *testing.T) {
			var hooked *beforeFirstAppPatch
			svc, store := c.setup(func(cl client.Client) client.Client {
				hooked = &beforeFirstAppPatch{Client: cl}
				return hooked
			}, map[string]string{"old": "1"})
			ctx := context.Background()
			name := c.kind.secretName("web")
			prepared := map[string]string{"new": "1"}
			namesake := sampleApp("web")
			namesake.UID = "uid-namesake"
			namesake.Spec.EnvFromSecret = envSecretName("web")
			namesake.Spec.FilesFromSecrets = []string{filesSecretName("web")}
			var recreated error
			hooked.hook = func() {
				recreated = func() error {
					// The service is deleted with its store paths and Secret, and a
					// create under its name prepares, creates and adopts its own.
					if err := svc.Client.Delete(ctx, sampleApp("web")); err != nil {
						return err
					}
					delete(store.m, c.path)
					delete(store.versions, c.path)
					if err := svc.Client.Delete(ctx, getSecret(t, svc.Client, name)); err != nil {
						return err
					}
					if err := svc.prepareProjection(ctx, sampleApp("web"), c.kind, c.path, prepared); err != nil {
						return err
					}
					if err := svc.Client.Create(ctx, namesake.DeepCopy()); err != nil {
						return err
					}
					return svc.adoptPreparedSecret(ctx, namesake, name)
				}()
			}

			err := c.write(svc, "stale")
			if recreated != nil {
				t.Fatalf("the namesake's create: %v", recreated)
			}
			if !errors.Is(err, core.ErrServiceReplaced) {
				t.Fatalf("the deleted service's write = %v, want the conflict that the service changed", err)
			}
			sec := getSecret(t, svc.Client, name)
			if owner := metav1.GetControllerOf(sec); owner == nil || owner.UID != namesake.UID || !equalSecretData(sec.Data, prepared) {
				t.Fatalf("the namesake's Secret holds %v, controlled by %+v, want its create's %v", sec.Data, owner, prepared)
			}
			if stored := store.m[c.path]; !maps.Equal(stored, prepared) || store.versions[c.path] != 1 {
				t.Fatalf("the namesake's store map = %v at version %d, want its create's %v at 1", stored, store.versions[c.path], prepared)
			}
			if live := getApp(t, svc.Client, "web"); live.UID != namesake.UID || live.Spec.RestartedAt != "" {
				t.Fatalf("the namesake's App = %s restarted at %q, want it as its create left it", live.UID, live.Spec.RestartedAt)
			}
		})
	}
}

// TestAReplacedWritesMergeIntoItsNamesakesMapIsTakenBack (w5/157): a write
// whose store write retried after its service was deleted and recreated
// merged its key into the namesake's map before its projection refused. Its
// compensation takes the key back while that write is still the latest,
// projects nothing, and answers that the service changed.
func TestAReplacedWritesMergeIntoItsNamesakesMapIsTakenBack(t *testing.T) {
	for _, c := range projectionCases {
		t.Run(c.name, func(t *testing.T) {
			svc, store := c.setup(nil, map[string]string{"old": "1"})
			ctx := context.Background()
			name := c.kind.secretName("web")
			namesake := sampleApp("web")
			namesake.UID = "uid-namesake"
			var recreated error
			store.afterGet = func() {
				recreated = func() error {
					if err := svc.Client.Delete(ctx, sampleApp("web")); err != nil {
						return err
					}
					delete(store.m, c.path)
					delete(store.versions, c.path)
					if err := svc.Client.Delete(ctx, getSecret(t, svc.Client, name)); err != nil {
						return err
					}
					if err := svc.prepareProjection(ctx, sampleApp("web"), c.kind, c.path, map[string]string{"new": "1"}); err != nil {
						return err
					}
					if err := svc.Client.Create(ctx, namesake.DeepCopy()); err != nil {
						return err
					}
					if err := svc.adoptPreparedSecret(ctx, namesake, name); err != nil {
						return err
					}
					// The namesake's own write moves its map past the version the
					// deleted service's write read, so that write's compare-and-set
					// retries on the namesake's map.
					return c.write(svc, "next")
				}()
			}

			err := c.write(svc, "stale")
			if recreated != nil {
				t.Fatalf("the namesake's create and write: %v", recreated)
			}
			if !errors.Is(err, core.ErrServiceReplaced) {
				t.Fatalf("the deleted service's write = %v, want the conflict that the service changed", err)
			}
			want := map[string]string{"new": "1", "next": "v"}
			if stored := store.m[c.path]; !maps.Equal(stored, want) {
				t.Fatalf("the namesake's store map = %v, want its own %v", stored, want)
			}
			if sec := getSecret(t, svc.Client, name); !equalSecretData(sec.Data, want) {
				t.Fatalf("the namesake's Secret holds %v, want its own %v", sec.Data, want)
			}
		})
	}
}
