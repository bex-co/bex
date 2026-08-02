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
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	base := (&Creds{Registry: "zot.example:5000"}).baseZotConfig()
	cfg, err := addZotReadOnlyACLEntry(base, "snapshots/tea-a-sandbox/**", "snap-tea-a-sandbox")
	if err != nil {
		t.Fatalf("addZotReadOnlyACLEntry: %v", err)
	}
	if !zotConfigHasRepoReadOnlyPolicy(cfg, "snapshots/tea-a-sandbox/**", "snap-tea-a-sandbox") {
		t.Fatal("read-only policy not detected after add")
	}
	// The write-policy detector used for per-App repos must NOT consider the
	// read-only entry sufficient (guards accidental privilege widening).
	if zotConfigHasRepoWritePolicy(cfg, "snapshots/tea-a-sandbox/**", "snap-tea-a-sandbox") {
		t.Fatal("snapshot ACL must not satisfy the read/write policy check")
	}

	var data map[string]any
	if err := json.Unmarshal(cfg, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	repos := zotReposMap(data)
	entry := repos["snapshots/tea-a-sandbox/**"].(map[string]any)
	policy := entry["policies"].([]any)[0].(map[string]any)
	actions := policy["actions"].([]any)
	if len(actions) != 1 || actions[0] != "read" {
		t.Fatalf("actions = %v; want exactly [read]", actions)
	}
}

// TestSnapshotACLRemoveRoundTrip verifies revoke removes only the namespace's
// entry and is idempotent.
func TestSnapshotACLRemoveRoundTrip(t *testing.T) {
	base := (&Creds{Registry: "zot.example:5000"}).baseZotConfig()
	cfg, err := addZotReadOnlyACLEntry(base, "snapshots/tea-a-sandbox/**", "snap-tea-a-sandbox")
	if err != nil {
		t.Fatalf("add a: %v", err)
	}
	cfg, err = addZotReadOnlyACLEntry(cfg, "snapshots/tea-b-sandbox/**", "snap-tea-b-sandbox")
	if err != nil {
		t.Fatalf("add b: %v", err)
	}
	cfg, err = removeZotACLEntry(cfg, "snapshots/tea-a-sandbox/**")
	if err != nil {
		t.Fatalf("remove a: %v", err)
	}
	if zotConfigHasRepo(cfg, "snapshots/tea-a-sandbox/**") {
		t.Fatal("a's entry should be gone")
	}
	if !zotConfigHasRepoReadOnlyPolicy(cfg, "snapshots/tea-b-sandbox/**", "snap-tea-b-sandbox") {
		t.Fatal("b's entry must survive a's removal")
	}
}

func snapshotTestCreds(t *testing.T, objs ...client.Object) *Creds {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return &Creds{
		Client:       fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build(),
		ZotNamespace: "bex-registry",
		HTPasswdName: "zot-htpasswd",
		ConfigName:   "zot-config",
		Registry:     "zot.bex-registry.svc:5000",
	}
}

// TestEnsureSnapshotCredsEndToEnd exercises the full mint against a fake
// cluster: pull Secret in the sandbox namespace, htpasswd entry, read-only
// ACL — then idempotency, then revoke.
func TestEnsureSnapshotCredsEndToEnd(t *testing.T) {
	ctx := context.Background()
	htpasswd := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "bex-registry"},
		Data:       map[string][]byte{"htpasswd": []byte("bex-builder:$2a$10$somehash\n")},
	}
	c := snapshotTestCreds(t, htpasswd)

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
	password, err := extractPassword(pullSec.Data[corev1.DockerConfigJsonKey], c.Registry, SnapshotZotUsername(ns))
	if err != nil || password == "" {
		t.Fatalf("extractPassword: %v (password empty=%v)", err, password == "")
	}

	var ht corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-htpasswd"}, &ht); err != nil {
		t.Fatal(err)
	}
	if !htpasswdHasUser(ht.Data["htpasswd"], SnapshotZotUsername(ns)) {
		t.Fatal("htpasswd entry missing")
	}
	if !htpasswdHasUser(ht.Data["htpasswd"], "bex-builder") {
		t.Fatal("bex-builder must survive")
	}

	var cfg corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-config"}, &cfg); err != nil {
		t.Fatalf("zot-config should be bootstrapped: %v", err)
	}
	if !zotConfigHasRepoReadOnlyPolicy(cfg.Data["config.json"], SnapshotRepoGlob(ns), SnapshotZotUsername(ns)) {
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
	if htpasswdHasUser(ht.Data["htpasswd"], SnapshotZotUsername(ns)) {
		t.Fatal("htpasswd entry should be revoked")
	}
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-config"}, &cfg); err != nil {
		t.Fatal(err)
	}
	if zotConfigHasRepo(cfg.Data["config.json"], SnapshotRepoGlob(ns)) {
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

// TestSnapshotCredsCrossNamespaceIsolation proves two workspaces get distinct
// users, distinct passwords, and ACLs that never reference each other.
func TestSnapshotCredsCrossNamespaceIsolation(t *testing.T) {
	ctx := context.Background()
	htpasswd := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "bex-registry"},
		Data:       map[string][]byte{"htpasswd": []byte("")},
	}
	c := snapshotTestCreds(t, htpasswd)

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
	passA, _ := extractPassword(secA.Data[corev1.DockerConfigJsonKey], c.Registry, "snap-tea-a-sandbox")
	passB, _ := extractPassword(secB.Data[corev1.DockerConfigJsonKey], c.Registry, "snap-tea-b-sandbox")
	if passA == "" || passA == passB {
		t.Fatal("workspaces must hold distinct credentials")
	}

	var cfg corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: "bex-registry", Name: "zot-config"}, &cfg); err != nil {
		t.Fatal(err)
	}
	// a's user must not appear in b's ACL entry and vice versa.
	if zotConfigHasRepoReadOnlyPolicy(cfg.Data["config.json"], SnapshotRepoGlob("tea-a-sandbox"), "snap-tea-b-sandbox") {
		t.Fatal("b's user must not read a's snapshots")
	}
	if zotConfigHasRepoReadOnlyPolicy(cfg.Data["config.json"], SnapshotRepoGlob("tea-b-sandbox"), "snap-tea-a-sandbox") {
		t.Fatal("a's user must not read b's snapshots")
	}
}
