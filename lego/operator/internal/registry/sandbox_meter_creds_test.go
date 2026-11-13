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
	"testing"

	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func meterTestCreds(t *testing.T) *Creds {
	t.Helper()
	return newTestCreds(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "bex-registry"},
		// bex-builder and a per-namespace snapshot user already exist.
		Data: map[string][]byte{htpasswdKey: []byte("bex-builder:$2a$10$somehash\nsnap-tea-a-sandbox:$2a$10$otherhash\n")},
	})
}

func getSecret(t *testing.T, c *Creds, name string) corev1.Secret {
	t.Helper()
	var sec corev1.Secret
	if err := c.Client.Get(context.Background(), client.ObjectKey{Namespace: "bex-registry", Name: name}, &sec); err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return sec
}

// The meter's read user can read the snapshot repositories and nothing else; the
// reaper can also delete; the importer can push but not delete (w2/m358); all
// three are scoped to snapshots/**.
func TestSandboxMeterCredsMintScopedUsers(t *testing.T) {
	ctx := context.Background()
	c := meterTestCreds(t)
	if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
		t.Fatalf("EnsureSandboxMeterCreds: %v", err)
	}

	sec := getSecret(t, c, SandboxMeterSecretName)
	if sec.Type != corev1.SecretTypeOpaque {
		t.Errorf("secret type = %v, want Opaque (bex-api reads plain keys)", sec.Type)
	}
	if string(sec.Data[SandboxMeterReaderUserKey]) != SandboxMeterReaderUser || string(sec.Data[SandboxMeterReaperUserKey]) != SandboxMeterReaperUser {
		t.Errorf("usernames = %q / %q", sec.Data[SandboxMeterReaderUserKey], sec.Data[SandboxMeterReaperUserKey])
	}
	if string(sec.Data[SandboxImporterUserKey]) != SandboxImporterUser {
		t.Errorf("importer username = %q", sec.Data[SandboxImporterUserKey])
	}
	reader, reaper, importer := string(sec.Data[SandboxMeterReaderPasswordKey]), string(sec.Data[SandboxMeterReaperPasswordKey]), string(sec.Data[SandboxImporterPasswordKey])
	if reader == "" || reaper == "" || importer == "" || reader == reaper || importer == reader || importer == reaper {
		t.Fatalf("passwords must be set and distinct, got %q, %q and %q", reader, reaper, importer)
	}

	ht := getSecret(t, c, "zot-htpasswd")
	for _, user := range []string{SandboxMeterReaderUser, SandboxMeterReaperUser, SandboxImporterUser, "bex-builder", "snap-tea-a-sandbox"} {
		if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], user); !ok {
			t.Errorf("htpasswd entry for %s missing (the existing users must survive)", user)
		}
	}

	data := mustDecode(t, getSecret(t, c, "zot-config").Data[zotConfigKey])
	if !zotRepoUserGrants(data, snapshotsRepoGlob, SandboxMeterReaderUser, zotReadOnlyActions) {
		t.Error("the meter user lacks read on snapshots/**")
	}
	if !zotRepoUserGrants(data, snapshotsRepoGlob, SandboxMeterReaperUser, zotReadDeleteActions) {
		t.Error("the reaper lacks read and delete on snapshots/**")
	}
	if !zotRepoUserGrants(data, snapshotsRepoGlob, SandboxImporterUser, zotImportActions) {
		t.Error("the importer lacks read, create and update on snapshots/**")
	}
	// The reader must never gain delete, only the importer writes, and it never
	// deletes.
	if zotRepoUserGrants(data, snapshotsRepoGlob, SandboxImporterUser, zotReadWriteActions) {
		t.Error("the importer must not be able to delete")
	}
	if zotRepoUserGrants(data, snapshotsRepoGlob, SandboxMeterReaderUser, zotReadDeleteActions) {
		t.Error("the meter user must not be able to delete")
	}
	if zotRepoUserGrants(data, snapshotsRepoGlob, SandboxMeterReaperUser, zotReadWriteActions) {
		t.Error("the reaper must not be able to push or update")
	}
	// Scope: the users appear on snapshots/** and on no other repository pattern.
	for repo, entry := range zotRepos(data) {
		if repo == snapshotsRepoGlob {
			continue
		}
		policies, _ := entry.(map[string]any)["policies"].([]any)
		for _, p := range policies {
			users, _ := p.(map[string]any)["users"].([]any)
			for _, u := range users {
				if u == SandboxMeterReaderUser || u == SandboxMeterReaperUser || u == SandboxImporterUser {
					t.Errorf("%v is granted on %s, outside snapshots/**", u, repo)
				}
			}
		}
	}
}

// A second run changes nothing: no rotation, and neither Secret is rewritten.
func TestSandboxMeterCredsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	c := meterTestCreds(t)
	if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
		t.Fatal(err)
	}
	secBefore, htBefore, cfgBefore := getSecret(t, c, SandboxMeterSecretName), getSecret(t, c, "zot-htpasswd"), getSecret(t, c, "zot-config")
	if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
		t.Fatal(err)
	}
	for name, before := range map[string]corev1.Secret{SandboxMeterSecretName: secBefore, "zot-htpasswd": htBefore, "zot-config": cfgBefore} {
		if after := getSecret(t, c, name); after.ResourceVersion != before.ResourceVersion {
			t.Errorf("%s was rewritten by a second run (resourceVersion %s -> %s)", name, before.ResourceVersion, after.ResourceVersion)
		}
	}
}

// A Secret that lost a password, or that was deleted, heals: the htpasswd hash
// is re-synced to whatever password the Secret ends up holding, so bex-api's
// credential and Zot's never disagree.
func TestSandboxMeterCredsHealAMissingPasswordAndAMissingSecret(t *testing.T) {
	ctx := context.Background()
	c := meterTestCreds(t)
	if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
		t.Fatal(err)
	}
	sec := getSecret(t, c, SandboxMeterSecretName)
	keep := string(sec.Data[SandboxMeterReaderPasswordKey])
	delete(sec.Data, SandboxMeterReaperPasswordKey)
	if err := c.Client.Update(ctx, &sec); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
		t.Fatal(err)
	}
	healed := getSecret(t, c, SandboxMeterSecretName)
	if string(healed.Data[SandboxMeterReaderPasswordKey]) != keep {
		t.Error("a password that was present must not rotate")
	}
	if len(healed.Data[SandboxMeterReaperPasswordKey]) == 0 {
		t.Fatal("the missing password was not regenerated")
	}

	// Delete the whole Secret: new passwords, and the htpasswd hashes verify them.
	if err := c.Client.Delete(ctx, &healed); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
		t.Fatal(err)
	}
	fresh, ht := getSecret(t, c, SandboxMeterSecretName), getSecret(t, c, "zot-htpasswd")
	for user, key := range map[string]string{SandboxMeterReaderUser: SandboxMeterReaderPasswordKey, SandboxMeterReaperUser: SandboxMeterReaperPasswordKey, SandboxImporterUser: SandboxImporterPasswordKey} {
		hash, ok := htpasswdUserHash(ht.Data[htpasswdKey], user)
		if !ok {
			t.Fatalf("htpasswd entry for %s missing", user)
		}
		if bcrypt.CompareHashAndPassword(hash, fresh.Data[key]) != nil {
			t.Errorf("htpasswd hash of %s does not verify the Secret's password", user)
		}
	}
}

// Zot applies only the longest matching pattern, so a workspace's own
// "snapshots/<ns>/**" shadows "snapshots/**": production answered 403 to the
// meter for every workspace that had one (2026-11-11). The minted users must be
// on each of those patterns, beside the namespace's own, and nowhere else.
func TestSandboxMeterCredsReachEveryNamespacePattern(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "bex-registry"},
		Data:       map[string][]byte{htpasswdKey: []byte("bex-builder:$2a$10$somehash\n")},
	}, zotConfigSecret([]byte(`{"http":{"accessControl":{"repositories":{
		"snapshots/tea-a-sandbox/**":{"policies":[{"users":["snap-tea-a-sandbox"],"actions":["read"]}]},
		"snapshots/tea-b-sandbox/**":{"policies":[{"users":["snap-tea-b-sandbox"],"actions":["read"]}]},
		"tea-a/web":{"policies":[{"users":["app-tea-a-web"],"actions":["read","create","update","delete"]}]}
	}}}}`)))
	if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
		t.Fatal(err)
	}
	data := storedConfig(t, c)
	for _, ns := range []string{"tea-a-sandbox", "tea-b-sandbox"} {
		repo := SnapshotRepoGlob(ns)
		if !zotRepoGrantsWith(data, repo, SnapshotZotUsername(ns), zotReadOnlyActions, sandboxMeterGrants()) {
			t.Errorf("%s = %v, want the namespace's read user plus the meter's reader, reaper and importer", repo, zotRepos(data)[repo])
		}
	}
	if !zotRepoGrants(data, "tea-a/web", "app-tea-a-web", zotReadWriteActions) {
		t.Error("an App repository must be left exactly as it was")
	}

	before := readSecret(t, c, testZotNS, "zot-config").ResourceVersion
	if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
		t.Fatal(err)
	}
	if after := readSecret(t, c, testZotNS, "zot-config").ResourceVersion; after != before {
		t.Errorf("a second run rewrote zot-config (resourceVersion %s -> %s)", before, after)
	}
}

// A namespace provisioned after the meter users exist gets them on its own
// pattern at once, and the two writers agree on the result: neither strips what
// the other adds, so repeated reconciles never rewrite Zot's config or bounce it.
func TestSnapshotCredsGrantMeterUsersWhenEnabledAndNeverThrash(t *testing.T) {
	ctx := context.Background()
	c := meterTestCreds(t)
	c.SandboxMeterUsers = true
	const ns = "tea-c-sandbox"

	if err := c.EnsureSnapshotCreds(ctx, ns); err != nil {
		t.Fatal(err)
	}
	if !zotRepoGrantsWith(storedConfig(t, c), SnapshotRepoGlob(ns), SnapshotZotUsername(ns), zotReadOnlyActions, sandboxMeterGrants()) {
		t.Fatal("a new namespace pattern must carry the meter's users")
	}
	if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
		t.Fatal(err)
	}
	settled := readSecret(t, c, testZotNS, "zot-config").ResourceVersion
	for range 2 {
		if err := c.EnsureSnapshotCreds(ctx, ns); err != nil {
			t.Fatal(err)
		}
		if err := c.EnsureSandboxMeterCreds(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if after := readSecret(t, c, testZotNS, "zot-config").ResourceVersion; after != settled {
		t.Errorf("the two writers fight: zot-config resourceVersion %s -> %s", settled, after)
	}
}

// Until the meter users are minted the namespace pattern stays what it was: one
// read-only policy for the namespace's own user.
func TestSnapshotCredsCarryNoMeterUsersByDefault(t *testing.T) {
	c := meterTestCreds(t)
	const ns = "tea-d-sandbox"
	if err := c.EnsureSnapshotCreds(context.Background(), ns); err != nil {
		t.Fatal(err)
	}
	if !zotRepoGrants(storedConfig(t, c), SnapshotRepoGlob(ns), SnapshotZotUsername(ns), zotReadOnlyActions) {
		t.Errorf("%s = %v, want exactly the namespace's read user", SnapshotRepoGlob(ns), zotRepos(storedConfig(t, c))[SnapshotRepoGlob(ns)])
	}
}
