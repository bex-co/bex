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

package registry

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestSnapshotNamingConventions pins the per-namespace naming scheme.
func TestSnapshotNamingConventions(t *testing.T) {
	if got := SnapshotZotUsername("tea-abc-sandbox"); got != "snap-tea-abc-sandbox" {
		t.Errorf("SnapshotZotUsername = %q; want %q", got, "snap-tea-abc-sandbox")
	}
	if got := SnapshotRepoGlob("tea-abc-sandbox"); got != "snapshots/tea-abc-sandbox/**" {
		t.Errorf("SnapshotRepoGlob = %q; want %q", got, "snapshots/tea-abc-sandbox/**")
	}
	if SnapshotPullSecretName != "bex-snapshot-pull" {
		t.Errorf("SnapshotPullSecretName = %q; want bex-snapshot-pull", SnapshotPullSecretName)
	}
}

// TestSnapshotACLIsReadOnly proves the minted policy carries exactly the read
// action — a snapshot consumer must never gain write on any repository.
func TestSnapshotACLIsReadOnly(t *testing.T) {
	const glob = "snapshots/tea-a-sandbox/**"
	const user = "snap-tea-a-sandbox"
	data := mustDecode(t, (&Creds{}).baseZotConfig())
	setZotRepoPolicy(data, glob, user, zotReadOnlyActions)

	if !zotRepoGrants(data, glob, user, zotReadOnlyActions) {
		t.Fatal("read-only policy not detected after add")
	}
	// The read/write grant used for per-App repos must NOT be satisfied by the
	// read-only entry (guards accidental privilege widening).
	if zotRepoGrants(data, glob, user, zotReadWriteActions) {
		t.Fatal("snapshot ACL must not satisfy the read/write policy check")
	}

	entry, _ := zotRepos(data)[glob].(map[string]any)
	policies, _ := entry["policies"].([]any)
	policy, _ := policies[0].(map[string]any)
	actions, _ := policy["actions"].([]any)
	if len(actions) != 1 || actions[0] != zotActionRead {
		t.Fatalf("actions = %v; want exactly [read]", actions)
	}
}

// TestSnapshotACLRemoveRoundTrip verifies revoke removes only the namespace's
// entry and leaves its neighbor intact.
func TestSnapshotACLRemoveRoundTrip(t *testing.T) {
	data := mustDecode(t, (&Creds{}).baseZotConfig())
	setZotRepoPolicy(data, "snapshots/tea-a-sandbox/**", "snap-tea-a-sandbox", zotReadOnlyActions)
	setZotRepoPolicy(data, "snapshots/tea-b-sandbox/**", "snap-tea-b-sandbox", zotReadOnlyActions)

	delete(zotRepos(data), "snapshots/tea-a-sandbox/**")
	if zotHasRepo(data, "snapshots/tea-a-sandbox/**") {
		t.Fatal("a's entry should be gone")
	}
	if !zotRepoGrants(data, "snapshots/tea-b-sandbox/**", "snap-tea-b-sandbox", zotReadOnlyActions) {
		t.Fatal("b's entry must survive a's removal")
	}
}

// TestEnsureSnapshotCredsEndToEnd exercises the full mint against a fake
// cluster: pull Secret in the sandbox namespace, htpasswd entry, read-only
// ACL — then idempotency, then revoke.
func TestEnsureSnapshotCredsEndToEnd(t *testing.T) {
	ctx := context.Background()
	htpasswd := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "bex-registry"},
		Data:       map[string][]byte{htpasswdKey: []byte("bex-builder:$2a$10$somehash\n")},
	}
	c := newTestCreds(t, htpasswd)

	const ns = "tea-x-sandbox"
	if err := c.EnsureSnapshotCreds(ctx, ns); err != nil {
		t.Fatalf("EnsureSnapshotCreds: %v", err)
	}

	var pullSec corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: SnapshotPullSecretName}, &pullSec); err != nil {
		t.Fatalf("pull secret missing: %v", err)
	}
	if pullSec.Type != corev1.SecretTypeDockerConfigJson {
		t.Fatalf("pull secret type = %v; want dockerconfigjson", pullSec.Type)
	}
	if got := pullSec.Labels[appv1alpha1.LabelProtectedFromTenantMount]; got != appv1alpha1.ProtectedFromTenantMount {
		t.Fatalf("protected label = %q; want %q (tenant workloads must not mount resume-pull creds)", got, appv1alpha1.ProtectedFromTenantMount)
	}
	password, err := c.passwordFrom(pullSec.Data[corev1.DockerConfigJsonKey], SnapshotZotUsername(ns))
	if err != nil || password == "" {
		t.Fatalf("passwordFrom: %v (password empty=%v)", err, password == "")
	}

	var ht corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-htpasswd"}, &ht); err != nil {
		t.Fatal(err)
	}
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], SnapshotZotUsername(ns)); !ok {
		t.Fatal("htpasswd entry missing")
	}
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], "bex-builder"); !ok {
		t.Fatal("bex-builder must survive")
	}

	var cfg corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-config"}, &cfg); err != nil {
		t.Fatalf("zot-config should be bootstrapped: %v", err)
	}
	if !zotRepoGrants(mustDecode(t, cfg.Data[zotConfigKey]), SnapshotRepoGlob(ns), SnapshotZotUsername(ns), zotReadOnlyActions) {
		t.Fatal("read-only ACL missing")
	}

	// Idempotent second ensure: same password survives (no rotation).
	if err := c.EnsureSnapshotCreds(ctx, ns); err != nil {
		t.Fatalf("second EnsureSnapshotCreds: %v", err)
	}
	var pullSec2 corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: SnapshotPullSecretName}, &pullSec2); err != nil {
		t.Fatal(err)
	}
	if string(pullSec.Data[corev1.DockerConfigJsonKey]) != string(pullSec2.Data[corev1.DockerConfigJsonKey]) {
		t.Fatal("ensure must be idempotent, not rotate")
	}

	// Revoke removes the Zot identity (the namespace Secret dies with the ns).
	if err := c.RevokeSnapshotCreds(ctx, ns); err != nil {
		t.Fatalf("RevokeSnapshotCreds: %v", err)
	}
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-htpasswd"}, &ht); err != nil {
		t.Fatal(err)
	}
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], SnapshotZotUsername(ns)); ok {
		t.Fatal("htpasswd entry should be revoked")
	}
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-config"}, &cfg); err != nil {
		t.Fatal(err)
	}
	if zotHasRepo(mustDecode(t, cfg.Data[zotConfigKey]), SnapshotRepoGlob(ns)) {
		t.Fatal("ACL entry should be revoked")
	}

	// Revoking again (or a namespace that never existed) is a no-op.
	if err := c.RevokeSnapshotCreds(ctx, ns); err != nil {
		t.Fatalf("revoke must be idempotent: %v", err)
	}
	if err := c.RevokeSnapshotCreds(ctx, "never-existed-sandbox"); err != nil {
		t.Fatalf("revoke of unknown namespace must be a no-op: %v", err)
	}
}

// TestSnapshotBackfillPatchDeniedIsNonFatal pins the w2/m82 t001 contract: a
// legacy snapshot Secret (minted before the protected label shipped) whose
// backfill patch is DENIED — the pre-patch RoleBinding state prod ran with —
// must not fail EnsureSnapshotCreds. The credential itself is already usable;
// failing the whole path here is what ERROR-spammed sandboxnamespaceregistry.
func TestSnapshotBackfillPatchDeniedIsNonFatal(t *testing.T) {
	ctx := context.Background()
	const ns = "tea-legacy-sandbox"
	zotUser := SnapshotZotUsername(ns)
	// A pre-round-11 Secret: valid credential, no protected label.
	c := newTestCreds(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "bex-registry"},
		Data:       map[string][]byte{htpasswdKey: []byte("bex-builder:$2a$10$somehash\n")},
	})
	legacy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns, Name: SnapshotPullSecretName,
			Labels: map[string]string{"app.bex.co/component": "snapshot-pull"},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: c.dockerConfig(zotUser, "legacy-password")},
	}
	patched := false
	c.Client = fake.NewClientBuilder().WithScheme(testScheme(t)).
		WithObjects(legacy, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "bex-registry"},
			Data:       map[string][]byte{htpasswdKey: []byte("bex-builder:$2a$10$somehash\n")},
		}).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if obj.GetName() != SnapshotPullSecretName {
					return cl.Patch(ctx, obj, patch, opts...)
				}
				patched = true
				return apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "secrets"}, obj.GetName(), errors.New("cannot patch"))
			},
		}).Build()

	if err := c.EnsureSnapshotCreds(ctx, ns); err != nil {
		t.Fatalf("EnsureSnapshotCreds must survive a denied backfill patch: %v", err)
	}
	if !patched {
		t.Fatal("the backfill patch was never attempted; this test no longer exercises the backfill path")
	}
	// The Mint still completes its Zot-side halves with the EXISTING password.
	var ht corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-htpasswd"}, &ht); err != nil {
		t.Fatal(err)
	}
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], zotUser); !ok {
		t.Fatal("htpasswd entry missing: the denied label backfill must not stop the credential mint")
	}
}

// TestSnapshotCredsCrossNamespaceIsolation proves two workspaces get distinct
// users, distinct passwords, and ACLs that never reference each other.
func TestSnapshotCredsCrossNamespaceIsolation(t *testing.T) {
	ctx := context.Background()
	htpasswd := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "bex-registry"},
		Data:       map[string][]byte{htpasswdKey: []byte("")},
	}
	c := newTestCreds(t, htpasswd)

	for _, ns := range []string{"tea-a-sandbox", "tea-b-sandbox"} {
		if err := c.EnsureSnapshotCreds(ctx, ns); err != nil {
			t.Fatalf("ensure %s: %v", ns, err)
		}
	}
	var secA, secB corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "tea-a-sandbox", Name: SnapshotPullSecretName}, &secA); err != nil {
		t.Fatal(err)
	}
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "tea-b-sandbox", Name: SnapshotPullSecretName}, &secB); err != nil {
		t.Fatal(err)
	}
	passA, _ := c.passwordFrom(secA.Data[corev1.DockerConfigJsonKey], "snap-tea-a-sandbox")
	passB, _ := c.passwordFrom(secB.Data[corev1.DockerConfigJsonKey], "snap-tea-b-sandbox")
	if passA == "" || passA == passB {
		t.Fatal("workspaces must hold distinct credentials")
	}

	var cfg corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-config"}, &cfg); err != nil {
		t.Fatal(err)
	}
	// a's user must not appear in b's ACL entry and vice versa.
	if zotRepoGrants(mustDecode(t, cfg.Data[zotConfigKey]), SnapshotRepoGlob("tea-a-sandbox"), "snap-tea-b-sandbox", zotReadOnlyActions) {
		t.Fatal("b's user must not read a's snapshots")
	}
	if zotRepoGrants(mustDecode(t, cfg.Data[zotConfigKey]), SnapshotRepoGlob("tea-b-sandbox"), "snap-tea-a-sandbox", zotReadOnlyActions) {
		t.Fatal("a's user must not read b's snapshots")
	}
}
