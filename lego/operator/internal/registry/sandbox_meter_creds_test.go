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
// reaper can also delete; neither can push, and both are scoped to snapshots/**.
func TestSandboxMeterCredsMintTwoScopedUsers(t *testing.T) {
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
	reader, reaper := string(sec.Data[SandboxMeterReaderPasswordKey]), string(sec.Data[SandboxMeterReaperPasswordKey])
	if reader == "" || reaper == "" || reader == reaper {
		t.Fatalf("passwords must be set and distinct, got %q and %q", reader, reaper)
	}

	ht := getSecret(t, c, "zot-htpasswd")
	for _, user := range []string{SandboxMeterReaderUser, SandboxMeterReaperUser, "bex-builder", "snap-tea-a-sandbox"} {
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
	// The reader must never gain delete, and nobody gains write.
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
				if u == SandboxMeterReaderUser || u == SandboxMeterReaperUser {
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
	for user, key := range map[string]string{SandboxMeterReaderUser: SandboxMeterReaderPasswordKey, SandboxMeterReaperUser: SandboxMeterReaperPasswordKey} {
		hash, ok := htpasswdUserHash(ht.Data[htpasswdKey], user)
		if !ok {
			t.Fatalf("htpasswd entry for %s missing", user)
		}
		if bcrypt.CompareHashAndPassword(hash, fresh.Data[key]) != nil {
			t.Errorf("htpasswd hash of %s does not verify the Secret's password", user)
		}
	}
}
