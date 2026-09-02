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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type deleteFailureClient struct {
	client.Client
	failSecretUpdates int
	failAppPatches    int
}

func (c *deleteFailureClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*corev1.Secret); ok && c.failSecretUpdates > 0 {
		c.failSecretUpdates--
		return errors.New("injected Kubernetes Secret update failure")
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *deleteFailureClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*appv1alpha1.App); ok && c.failAppPatches > 0 {
		c.failAppPatches--
		return errors.New("injected App patch failure")
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func TestDeleteEnvVarRetryConvergesAfterProjectionFailure(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"KEEP": "yes", "REVOKED": "old"}
	app := sampleApp("web")
	app.Spec.EnvFromSecret = "web-env"
	projection := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "web-env", Namespace: "default"},
		Data:       map[string][]byte{"KEEP": []byte("yes"), "REVOKED": []byte("old")},
	}
	cl := &deleteFailureClient{Client: fakeClient(app, projection), failSecretUpdates: 1}
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: store}

	if err := svc.DeleteEnvVar(context.Background(), "web", "REVOKED"); err == nil {
		t.Fatal("DeleteEnvVar unexpectedly succeeded")
	}
	if store.m[envPath("web")]["REVOKED"] != "old" {
		t.Fatal("source key was deleted before its Kubernetes projection converged")
	}
	if _, ok := getSecret(t, cl, "web-env").Data["REVOKED"]; !ok {
		t.Fatal("failed Secret update unexpectedly changed the projection")
	}

	if err := svc.DeleteEnvVar(context.Background(), "web", "REVOKED"); err != nil {
		t.Fatalf("DeleteEnvVar retry: %v", err)
	}
	if _, ok := store.m[envPath("web")]["REVOKED"]; ok {
		t.Fatal("retry left revoked source key behind")
	}
	if _, ok := getSecret(t, cl, "web-env").Data["REVOKED"]; ok {
		t.Fatal("retry left revoked projection key behind")
	}
}

func TestDeleteSecretFileRetryRollsAfterAppPatchFailure(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[filesPath("web")] = map[string]string{"keep.txt": "yes", "revoked.txt": "old"}
	app := sampleApp("web")
	app.Spec.FilesFromSecrets = []string{"web-files"}
	projection := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "web-files", Namespace: "default"},
		Data:       map[string][]byte{"keep.txt": []byte("yes"), "revoked.txt": []byte("old")},
	}
	cl := &deleteFailureClient{Client: fakeClient(app, projection), failAppPatches: 1}
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: store}

	if err := svc.DeleteSecretFile(context.Background(), "web", "revoked.txt"); err == nil {
		t.Fatal("DeleteSecretFile unexpectedly succeeded")
	}
	if store.m[filesPath("web")]["revoked.txt"] != "old" {
		t.Fatal("source file was deleted before the App rollout converged")
	}
	if _, ok := getSecret(t, cl, "web-files").Data["revoked.txt"]; ok {
		t.Fatal("derived Secret retained the revoked file after its update succeeded")
	}

	if err := svc.DeleteSecretFile(context.Background(), "web", "revoked.txt"); err != nil {
		t.Fatalf("DeleteSecretFile retry: %v", err)
	}
	if _, ok := store.m[filesPath("web")]["revoked.txt"]; ok {
		t.Fatal("retry left revoked source file behind")
	}
	if getApp(t, cl, "web").Spec.RestartedAt == "" {
		t.Fatal("retry did not persist the workload rollout")
	}
}

func TestDeleteEnvVarAlreadyAbsentRepairsStaleProjection(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"KEEP": "yes"}
	app := sampleApp("web")
	app.Spec.EnvFromSecret = "web-env"
	projection := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "web-env", Namespace: "default"},
		Data:       map[string][]byte{"KEEP": []byte("yes"), "REVOKED": []byte("stale")},
	}
	svc := newService(store, app, projection)

	err := svc.DeleteEnvVar(context.Background(), "web", "REVOKED")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("DeleteEnvVar missing key = %v, want ErrNotFound after repair", err)
	}
	if _, ok := getSecret(t, svc.Client, "web-env").Data["REVOKED"]; ok {
		t.Fatal("already-absent retry left stale projection behind")
	}
	if getApp(t, svc.Client, "web").Spec.RestartedAt == "" {
		t.Fatal("already-absent retry did not roll the workload")
	}
}
