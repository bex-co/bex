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
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// projectedSecret is web's kind Secret holding data at revision.
func projectedSecret(kind projectionKind, revision sourceRevision, data map[string]string) *corev1.Secret {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: kind.secretName("web"), Namespace: "default"},
		Data:       envBytes(data),
	}
	setProjectedRevision(sec, kind, revision)
	return sec
}

// TestAProjectionNeverMovesItsSecretToAnEarlierRevision pins projectSource's
// order (w5/m127). A delete projects before it commits, at the revision its
// commit will create, marked provisional.
func TestAProjectionNeverMovesItsSecretToAnEarlierRevision(t *testing.T) {
	v1 := map[string]string{"A": "1", "B": "1", "C": "1"}
	v2 := map[string]string{"A": "1", "B": "1", "C": "1", "D": "1"} // a write committed over v1
	withoutA := map[string]string{"B": "1", "C": "1"}               // a delete of A, read at v1
	withoutB := map[string]string{"A": "1", "C": "1"}               // a delete of B, read at v1
	for _, tc := range []struct {
		name           string
		current        *corev1.Secret // nil: not projected yet
		data           map[string]string
		revision       sourceRevision
		remove         bool
		wantData       map[string]string // nil: no Secret
		wantRevision   sourceRevision
		wantSuperseded bool
		wantConflict   bool
		untouched      bool
	}{{
		name: "not projected yet: created at the revision",
		data: v1, revision: committedAt(1), wantData: v1, wantRevision: committedAt(1),
	}, {
		name:    "a Secret without a revision is replaced",
		current: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-env", Namespace: "default"}, Data: envBytes(withoutA)},
		data:    v1, revision: committedAt(1), wantData: v1, wantRevision: committedAt(1),
	}, {
		name:    "a later revision replaces an earlier one",
		current: projectedSecret(envProjection, committedAt(1), v1),
		data:    v2, revision: committedAt(2), wantData: v2, wantRevision: committedAt(2),
	}, {
		name:    "an earlier revision is left to a later one",
		current: projectedSecret(envProjection, committedAt(2), v2),
		data:    v1, revision: committedAt(1), wantData: v2, wantRevision: committedAt(2), wantSuperseded: true, untouched: true,
	}, {
		name:    "the same revision and data changes nothing",
		current: projectedSecret(envProjection, committedAt(2), v2),
		data:    v2, revision: committedAt(2), wantData: v2, wantRevision: committedAt(2), untouched: true,
	}, {
		name:    "the same revision with other data is a conflict",
		current: projectedSecret(envProjection, committedAt(2), v2),
		data:    v1, revision: committedAt(2), wantData: v2, wantRevision: committedAt(2), wantConflict: true, untouched: true,
	}, {
		name:    "a delete's provisional revision replaces the revision it read",
		current: projectedSecret(envProjection, committedAt(1), v1),
		data:    withoutA, revision: provisionalAt(2), wantData: withoutA, wantRevision: provisionalAt(2),
	}, {
		name:    "an earlier revision is left to a delete's provisional one",
		current: projectedSecret(envProjection, provisionalAt(2), withoutA),
		data:    v1, revision: committedAt(1), wantData: withoutA, wantRevision: provisionalAt(2), wantSuperseded: true, untouched: true,
	}, {
		name:    "a write that committed the revision replaces a delete's provisional projection of it",
		current: projectedSecret(envProjection, provisionalAt(2), withoutA),
		data:    v2, revision: committedAt(2), wantData: v2, wantRevision: committedAt(2),
	}, {
		name:    "a delete's provisional revision is refused by a write that committed it",
		current: projectedSecret(envProjection, committedAt(2), v2),
		data:    withoutA, revision: provisionalAt(2), wantData: v2, wantRevision: committedAt(2), wantSuperseded: true, wantConflict: true, untouched: true,
	}, {
		name:    "of two deletes that read one revision, the Secret keeps what both keep",
		current: projectedSecret(envProjection, provisionalAt(2), withoutA),
		data:    withoutB, revision: provisionalAt(2), wantData: map[string]string{"C": "1"}, wantRevision: provisionalAt(2),
	}, {
		name:     "a removal deletes an earlier revision",
		current:  projectedSecret(envProjection, committedAt(1), v1),
		revision: committedAt(2), remove: true,
	}, {
		name:     "a removal leaves a later revision",
		current:  projectedSecret(envProjection, committedAt(2), v2),
		revision: committedAt(1), remove: true, wantData: v2, wantRevision: committedAt(2), wantSuperseded: true, untouched: true,
	}, {
		name:     "a removal of a Secret never projected changes nothing",
		revision: committedAt(1), remove: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			objs := []client.Object{sampleApp("web")}
			if tc.current != nil {
				objs = append(objs, tc.current)
			}
			svc := newService(newVersionedFakeSecretStore(), objs...)
			var before string
			if tc.current != nil {
				before = getSecret(t, svc.Client, "web-env").ResourceVersion
			}

			got, err := svc.projectSource(context.Background(), sampleApp("web"), envProjection, tc.data, tc.revision, tc.remove)
			if tc.wantConflict {
				if !errors.Is(err, errProjectionConflict) {
					t.Fatalf("projectSource error = %v, want errProjectionConflict", err)
				}
			} else if err != nil {
				t.Fatalf("projectSource: %v", err)
			}
			if got.Superseded != tc.wantSuperseded {
				t.Fatalf("superseded = %v, want %v", got.Superseded, tc.wantSuperseded)
			}
			if tc.wantData == nil {
				if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web-env"}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
					t.Fatalf("the Secret is still there (get error %v)", err)
				}
				return
			}
			if mounted := secretData(t, svc.Client, "web-env"); !maps.Equal(mounted, tc.wantData) {
				t.Fatalf("the Secret mounts %v, want %v", mounted, tc.wantData)
			}
			sec := getSecret(t, svc.Client, "web-env")
			if revision, known := projectedRevision(sec, envProjection); !known || revision != tc.wantRevision {
				t.Fatalf("the Secret records %+v (known %v), want %+v", revision, known, tc.wantRevision)
			}
			if tc.untouched && sec.ResourceVersion != before {
				t.Fatalf("the Secret was written (resourceVersion %s -> %s), want it untouched", before, sec.ResourceVersion)
			}
		})
	}
}

// interleavedClient runs between once, just before the next Secret update or
// delete: another projection landing between projectSource's read and its
// write.
type interleavedClient struct {
	client.Client
	between func()
}

func (c *interleavedClient) interleave(obj client.Object) {
	if _, ok := obj.(*corev1.Secret); ok && c.between != nil {
		between := c.between
		c.between = nil
		between()
	}
}

func (c *interleavedClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.interleave(obj)
	return c.Client.Update(ctx, obj, opts...)
}

func (c *interleavedClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.interleave(obj)
	return c.Client.Delete(ctx, obj, opts...)
}

// TestAProjectionRejudgesASecretChangedSinceItsRead (w5/m127): an update or a
// delete carries the resourceVersion projectSource read, so a later revision
// projected in between is judged, and kept, rather than overwritten or removed.
func TestAProjectionRejudgesASecretChangedSinceItsRead(t *testing.T) {
	for name, remove := range map[string]bool{"update": false, "delete": true} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			cl := &interleavedClient{Client: fakeClient(sampleApp("web"), projectedSecret(envProjection, committedAt(1), map[string]string{"A": "1"}))}
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: newVersionedFakeSecretStore()}
			later := map[string]string{"A": "3"}
			cl.between = func() {
				if _, err := svc.projectSource(ctx, sampleApp("web"), envProjection, later, committedAt(3), false); err != nil {
					t.Fatalf("the later projection: %v", err)
				}
			}
			var data map[string]string
			if !remove {
				data = map[string]string{"A": "2"}
			}

			got, err := svc.projectSource(ctx, sampleApp("web"), envProjection, data, committedAt(2), remove)
			if err != nil || !got.Superseded {
				t.Fatalf("projectSource = %+v, %v; want superseded by the later revision", got, err)
			}
			if mounted := secretData(t, cl, "web-env"); !maps.Equal(mounted, later) {
				t.Fatalf("the Secret mounts %v, want the later revision's %v", mounted, later)
			}
		})
	}
}

// afterCASAt runs fn once, right after the store commits version at.
func afterCASAt(store *versionedFakeSecretStore, at uint64, fn func()) {
	var hook func(string, uint64)
	hook = func(_ string, version uint64) {
		if version != at {
			store.afterCAS = hook
			return
		}
		fn()
	}
	store.afterCAS = hook
}

// projectionCase is one derived Secret, with the single-key write and delete
// that project it.
type projectionCase struct {
	name   string
	path   string
	kind   projectionKind
	write  func(svc *Service, key string) error
	delete func(svc *Service, key string) error
}

var projectionCases = []projectionCase{{
	name: "env var", path: envPath("web"), kind: envProjection,
	write: func(svc *Service, key string) error {
		_, err := svc.SetEnvVar(context.Background(), "web", key, EnvVarWrite{Value: "v"})
		return err
	},
	delete: func(svc *Service, key string) error { return svc.DeleteEnvVar(context.Background(), "web", key) },
}, {
	name: "secret file", path: filesPath("web"), kind: filesProjection,
	write: func(svc *Service, key string) error {
		_, err := svc.SetSecretFile(context.Background(), "web", key, "v")
		return err
	},
	delete: func(svc *Service, key string) error { return svc.DeleteSecretFile(context.Background(), "web", key) },
}}

// setup is a service whose store holds data at version 1, projected and
// referenced, behind wrap's client when wrap is set.
func (c projectionCase) setup(wrap func(client.Client) client.Client, data map[string]string) (*Service, *versionedFakeSecretStore) {
	store := newVersionedFakeSecretStore()
	store.m[c.path] = maps.Clone(data)
	store.versions[c.path] = 1
	app := sampleApp("web")
	app.Spec.EnvFromSecret = envSecretName("web")
	app.Spec.FilesFromSecrets = []string{filesSecretName("web")}
	cl := fakeClient(app, projectedSecret(c.kind, committedAt(1), data))
	if wrap != nil {
		cl = wrap(cl)
	}
	return &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: store}, store
}

// expectMatchesStore checks the Secret mounts the store's map at the store's
// version, and the App references it. A delete that committed keeps its
// provisional mark, so the mark is not checked.
func (c projectionCase) expectMatchesStore(t *testing.T, svc *Service, store *versionedFakeSecretStore) {
	t.Helper()
	name := c.kind.secretName("web")
	if mounted, stored := secretData(t, svc.Client, name), store.m[c.path]; !maps.Equal(mounted, stored) {
		t.Fatalf("the Secret mounts %v while the store lists %v", mounted, stored)
	}
	if got, known := projectedRevision(getSecret(t, svc.Client, name), c.kind); !known || got.version != store.versions[c.path] {
		t.Fatalf("the Secret records version %d (known %v), want the store's %d", got.version, known, store.versions[c.path])
	}
	spec := getApp(t, svc.Client, "web").Spec
	if referenced := spec.EnvFromSecret == name || slices.Contains(spec.FilesFromSecrets, name); !referenced {
		t.Fatalf("the App does not reference %s: %+v", name, spec)
	}
}

// TestAStaleProjectionNeverOverwritesANewerOne (w5/m127): writer A commits
// version 2, then writer B commits 3 and projects it before A projects. A's
// projection used to land over B's, so the Secret mounted A's older map while
// the store listed B's, until the next write.
func TestAStaleProjectionNeverOverwritesANewerOne(t *testing.T) {
	for _, c := range projectionCases {
		t.Run(c.name, func(t *testing.T) {
			svc, store := c.setup(nil, map[string]string{"keep": "1"})
			var concurrent error
			afterCASAt(store, 2, func() { concurrent = c.write(svc, "b") })

			if err := c.write(svc, "a"); err != nil {
				t.Fatalf("writer A: %v", err)
			}
			if concurrent != nil {
				t.Fatalf("writer B: %v", concurrent)
			}
			c.expectMatchesStore(t, svc, store)
		})
	}
}

// TestARestoreNeverOverwritesANewerProjection (w5/m127): writer A's App patch
// fails, and its compensation restores the store at version 3. Writer B
// commits 4 and projects it before A's restore projects 3. The restore used to
// land over B's projection: w5/m119's remaining window. A's change is gone from
// the store, so A answers its own failure, not a failed restoration.
func TestARestoreNeverOverwritesANewerProjection(t *testing.T) {
	for _, c := range projectionCases {
		t.Run(c.name, func(t *testing.T) {
			svc, store := c.setup(func(cl client.Client) client.Client {
				return &deleteFailureClient{Client: cl, failAppPatches: 1}
			}, map[string]string{"keep": "1"})
			var concurrent error
			afterCASAt(store, 3, func() { concurrent = c.write(svc, "b") })

			err := c.write(svc, "a")
			var coded *core.CodedError
			if err == nil || errors.As(err, &coded) {
				t.Fatalf("writer A = %v, want its App patch failure", err)
			}
			if concurrent != nil {
				t.Fatalf("writer B: %v", concurrent)
			}
			if want := map[string]string{"keep": "1", "b": "v"}; !maps.Equal(store.m[c.path], want) {
				t.Fatalf("store = %v, want %v", store.m[c.path], want)
			}
			c.expectMatchesStore(t, svc, store)
		})
	}
}

// TestADeleteProjectsTheRevisionItsCommitWillCreate (w5/m127): a delete
// projects before it commits, so a failed delete still leaves its key revoked.
// It stamps the version its commit will create, marked provisional, and a write
// that commits that version owns it.
func TestADeleteProjectsTheRevisionItsCommitWillCreate(t *testing.T) {
	for _, c := range projectionCases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("one that loses the race to a write re-reads before it rolls", func(t *testing.T) {
				var counting *patchCountingClient
				svc, store := c.setup(func(cl client.Client) client.Client {
					counting = &patchCountingClient{Client: cl}
					return counting
				}, map[string]string{"keep": "1", "gone": "1"})
				var concurrent error
				store.afterGet = func() { concurrent = c.write(svc, "w") }

				if err := c.delete(svc, "gone"); err != nil {
					t.Fatalf("delete: %v", err)
				}
				if concurrent != nil {
					t.Fatalf("the racing write: %v", concurrent)
				}
				if want := map[string]string{"keep": "1", "w": "v"}; !maps.Equal(store.m[c.path], want) {
					t.Fatalf("store = %v, want %v", store.m[c.path], want)
				}
				c.expectMatchesStore(t, svc, store)
				if counting.patches != 2 {
					t.Fatalf("%d App patches, want 2: the write's, and the delete's after it re-read", counting.patches)
				}
			})

			t.Run("one that loses the race never unmounts the write's key", func(t *testing.T) {
				svc, store := c.setup(nil, map[string]string{"keep": "1", "gone": "1"})
				var concurrent error
				store.afterGet = func() { concurrent = c.write(svc, "w") }
				store.failCASCall = 2 // the delete's commit, after the write's

				if err := c.delete(svc, "gone"); err == nil {
					t.Fatal("the delete whose commit failed succeeded")
				}
				if concurrent != nil {
					t.Fatalf("the racing write: %v", concurrent)
				}
				if mounted, want := secretData(t, svc.Client, c.kind.secretName("web")), map[string]string{"keep": "1", "w": "v"}; !maps.Equal(mounted, want) {
					t.Fatalf("the Secret mounts %v, want %v: the write's key, without the failed delete's", mounted, want)
				}
			})

			t.Run("one whose commit fails keeps its key revoked until the next write", func(t *testing.T) {
				svc, store := c.setup(nil, map[string]string{"keep": "1", "gone": "1"})
				store.failCASCall = 1

				if err := c.delete(svc, "gone"); err == nil {
					t.Fatal("the delete whose commit failed succeeded")
				}
				if _, mounted := secretData(t, svc.Client, c.kind.secretName("web"))["gone"]; mounted {
					t.Fatal("the failed delete's key is mounted again")
				}
				if err := c.write(svc, "w"); err != nil {
					t.Fatalf("the next write, which commits the delete's version: %v", err)
				}
				c.expectMatchesStore(t, svc, store)
			})

			t.Run("one whose commit fails leaves the next delete free to land", func(t *testing.T) {
				svc, store := c.setup(nil, map[string]string{"keep": "1", "gone": "1", "other": "1"})
				store.failCASCall = 1

				if err := c.delete(svc, "gone"); err == nil {
					t.Fatal("the delete whose commit failed succeeded")
				}
				if err := c.delete(svc, "other"); err != nil {
					t.Fatalf("the next delete, which read the same version: %v", err)
				}
				if mounted, want := secretData(t, svc.Client, c.kind.secretName("web")), map[string]string{"keep": "1"}; !maps.Equal(mounted, want) {
					t.Fatalf("the Secret mounts %v, want %v: both deletes' keys revoked", mounted, want)
				}
				if err := c.write(svc, "w"); err != nil {
					t.Fatalf("the next write: %v", err)
				}
				c.expectMatchesStore(t, svc, store)
			})

			t.Run("a delete that read the same version never mounts the committed one's key again", func(t *testing.T) {
				svc, store := c.setup(nil, map[string]string{"keep": "1", "first": "1", "second": "1"})
				var first error
				store.afterGet = func() { first = c.delete(svc, "first") }
				store.failCASCall = 2 // the second delete's commit, after the first's

				if err := c.delete(svc, "second"); err == nil {
					t.Fatal("the second delete, whose commit failed, succeeded")
				}
				if first != nil {
					t.Fatalf("the first delete: %v", first)
				}
				if _, mounted := secretData(t, svc.Client, c.kind.secretName("web"))["first"]; mounted {
					t.Fatal("the first delete's key, deleted and committed, is mounted again")
				}
			})

			t.Run("an absent key's repair leaves a first write's projection", func(t *testing.T) {
				store := newVersionedFakeSecretStore() // nothing written yet: version 0
				svc := newService(store, sampleApp("web"))
				var concurrent error
				store.afterGet = func() { concurrent = c.write(svc, "w") }

				if err := c.delete(svc, "absent"); !errors.Is(err, core.ErrNotFound) {
					t.Fatalf("delete of an absent key = %v, want not found", err)
				}
				if concurrent != nil {
					t.Fatalf("the racing first write: %v", concurrent)
				}
				c.expectMatchesStore(t, svc, store)
			})
		})
	}
}

// TestASupersededFilesRemovalKeepsTheMount (w5/m127): a batch patch deletes a
// service's last file and commits, then a write commits a new file and projects
// it before the patch projects its removal. The removal is superseded, and the
// patch's App update must not drop the reference the newer Secret is mounted
// through.
func TestASupersededFilesRemovalKeepsTheMount(t *testing.T) {
	c := projectionCases[1]
	svc, store := c.setup(nil, map[string]string{"old": "1"})
	var concurrent error
	afterCASAt(store, 2, func() { concurrent = c.write(svc, "new") })

	if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, SecretFiles: []SecretFilePatch{{Name: "old", Delete: true}},
	}); err != nil {
		t.Fatalf("PatchEnvironment: %v", err)
	}
	if concurrent != nil {
		t.Fatalf("the racing write: %v", concurrent)
	}
	c.expectMatchesStore(t, svc, store)
}

// restoreKeepsLosing fails the next App patch with an error naming internal
// endpoints, then makes every later Secret update conflict: a write whose
// compensation cannot project its restore.
type restoreKeepsLosing struct {
	client.Client
	patched bool
}

func (c *restoreKeepsLosing) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*appv1alpha1.App); ok && !c.patched {
		c.patched = true
		return errors.New(`Patch "https://10.0.0.1:6443/apis/app.bex.co/v1alpha1/namespaces/ws-internal/apps/web": connection refused`)
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *restoreKeepsLosing) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*corev1.Secret); ok && c.patched {
		return apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, obj.GetName(), errors.New("changed"))
	}
	return c.Client.Update(ctx, obj, opts...)
}

// TestAFailedRestoreProjectionStaysPrivate (w5/m127): a compensation whose
// restore projection kept losing joins that failure with the write's own cause.
// The projection's error must not make the join a public answer, which would
// send the cause's internal endpoint to the caller.
func TestAFailedRestoreProjectionStaysPrivate(t *testing.T) {
	c := projectionCases[0]
	svc, _ := c.setup(func(cl client.Client) client.Client {
		return &restoreKeepsLosing{Client: cl}
	}, map[string]string{"keep": "1"})

	err := c.write(svc, "a")
	if err == nil {
		t.Fatal("the write whose App patch failed succeeded")
	}
	if core.IsPublicError(err) {
		t.Fatalf("the compensation failure is a public answer: %v", err)
	}
}

// TestACASRollbackLeavesADeleteProvisionalProjection (w5/m127): the
// revision-aware patch rolls back only a Secret its own committed revision
// holds. A delete's provisional projection of that version is the delete's,
// and putting the patch's prior map over it would mount the revoked key again.
func TestACASRollbackLeavesADeleteProvisionalProjection(t *testing.T) {
	revoked := map[string]string{"KEEP": "1"}
	svc := newService(newVersionedFakeSecretStore(), sampleApp("web"), projectedSecret(envProjection, provisionalAt(2), revoked))

	err := svc.rollbackCASEnvProjection(context.Background(), sampleApp("web"), map[string]string{"KEEP": "1", "GONE": "1"}, 3, casEnvProjection{OwnerVersion: 2, ExistedBefore: true})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_REVISION_CONFLICT" {
		t.Fatalf("rollback = %v, want ENVIRONMENT_REVISION_CONFLICT", err)
	}
	if mounted := secretData(t, svc.Client, "web-env"); !maps.Equal(mounted, revoked) {
		t.Fatalf("the Secret mounts %v, want the delete's %v", mounted, revoked)
	}
}
