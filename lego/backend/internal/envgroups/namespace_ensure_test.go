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
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// newWorkspaceWithoutNamespaces is a service in which dana belongs to tea-a
// and tea-b, whose namespaces the NamespaceReconciler has not created yet: the
// client refuses a namespaced create into a missing namespace, as the API
// server does, and EnsureNamespaces creates the workspace's namespace, as
// NamespaceReconciler.EnsureWorkspace does. It returns the workspaces ensured.
func newWorkspaceWithoutNamespaces(t *testing.T) (*Service, context.Context, *[]string) {
	t.Helper()
	cl := fakeClient().(client.WithWatch)
	var ensured []string
	svc := withGroups(&Service{
		Base: &core.Base{
			Client: interceptor.NewClient(cl, interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if ns := obj.GetNamespace(); ns != "" {
						if err := c.Get(ctx, client.ObjectKey{Name: ns}, &corev1.Namespace{}); apierrors.IsNotFound(err) {
							return apierrors.NewNotFound(corev1.Resource("namespaces"), ns)
						}
					}
					return c.Create(ctx, obj, opts...)
				},
			}),
			Namespace: "default",
			Workspace: coretest.Members{"dana": {"tea-a", "tea-b"}},
			Clock:     func() time.Time { return time.Unix(1_000_000, 0).UTC() },
			EnsureNamespaces: func(ctx context.Context, ws string) error {
				ensured = append(ensured, ws)
				return cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ws}})
			},
		},
		Store: newFakeStore(),
	})
	return svc, core.WithIdentity(tctx(t), core.Identity{Subject: "dana", Method: "session"}), &ensured
}

// requireProjection fails unless group gid's env projection Secret stands in
// workspace's own namespace.
func requireProjection(t *testing.T, svc *Service, workspace, gid string) {
	t.Helper()
	var sec corev1.Secret
	if err := svc.Client.Get(t.Context(), client.ObjectKey{Namespace: workspace, Name: envSecretName(gid)}, &sec); err != nil {
		t.Fatalf("projection Secret in %s: %v", workspace, err)
	}
}

// TestEnvGroupCreatesEnsureABrandNewWorkspacesNamespace (w4/733): a group made
// before the NamespaceReconciler's resync had created its workspace's
// namespace answered an internal error, `namespaces "tea-…" not found`, since
// no create path ensured it as service, Postgres and Key Value create do.
// Every way a group comes to exist — create, clone into another workspace, a
// Blueprint's new group — now succeeds on the first try.
func TestEnvGroupCreatesEnsureABrandNewWorkspacesNamespace(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		svc, ctx, ensured := newWorkspaceWithoutNamespaces(t)
		g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
			Name: "shared", OwnerID: "tea-a", EnvVars: []CreateEnvVarInput{{Key: "A", Value: "1"}},
		})
		if err != nil {
			t.Fatalf("create in a workspace without its namespace: %v", err)
		}
		requireProjection(t, svc, "tea-a", g.ID)
		if !slices.Equal(*ensured, []string{"tea-a"}) {
			t.Fatalf("ensured %v, want [tea-a]", *ensured)
		}
	})

	t.Run("clone into another workspace", func(t *testing.T) {
		svc, ctx, ensured := newWorkspaceWithoutNamespaces(t)
		// The source workspace is provisioned; only the target is brand new.
		if err := svc.Client.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tea-a"}}); err != nil {
			t.Fatal(err)
		}
		source, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
			Name: "source", OwnerID: "tea-a", EnvVars: []CreateEnvVarInput{{Key: "A", Value: "1"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		clone, err := svc.CloneEnvGroup(ctx, source.ID, CloneEnvGroupRequest{Name: "copy", OwnerID: "tea-b"})
		if err != nil {
			t.Fatalf("clone into a workspace without its namespace: %v", err)
		}
		requireProjection(t, svc, "tea-b", clone.ID)
		if !slices.Equal(*ensured, []string{"tea-b"}) {
			t.Fatalf("ensured %v, want [tea-b]", *ensured)
		}
	})

	t.Run("blueprint", func(t *testing.T) {
		svc, ctx, ensured := newWorkspaceWithoutNamespaces(t)
		ctx = core.WithWorkspace(ctx, "tea-a")
		if _, err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"A": "1"}, nil); err != nil {
			t.Fatalf("blueprint group in a workspace without its namespace: %v", err)
		}
		requireProjection(t, svc, "tea-a", idNamed(ctx, svc, "shared"))
		if !slices.Equal(*ensured, []string{"tea-a"}) {
			t.Fatalf("ensured %v, want [tea-a]", *ensured)
		}
	})
}
