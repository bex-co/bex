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
	if !errors.Is(err, core.ErrConflict) || err.Error() != errServiceReplaced.Error() {
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
