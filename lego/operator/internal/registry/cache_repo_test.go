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

	"github.com/bex-co/bex/lego/operator/internal/identity"
)

// The build cache (docs/ADR060 D3) is a second repository per App holding
// source-derived layers. It has to inherit the App's existing tenant boundary
// rather than open one beside it, and it has to actually be writable by the
// identity the build pushes as — in per-App mode Zot matches only the longest
// repository rule, so the builder's "**" grant does not reach a repository a
// tenant user owns.

func TestCacheRepositoryGetsTheSameExclusiveGrantAsTheImageRepository(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret((&Creds{}).baseZotConfig()))
	id := identity.ForApp("hello", "tea-w1")

	if err := c.EnsureCredsFor(ctx, id, "default"); err != nil {
		t.Fatalf("EnsureCredsFor: %v", err)
	}
	data := storedConfig(t, c)
	for _, repo := range []string{id.Repo(), id.CacheRepo()} {
		if !zotRepoGrants(data, repo, id.ZotUsername(), zotReadWriteActions) {
			t.Errorf("repository %q is not granted exactly to %q; the cache push would be denied",
				repo, id.ZotUsername())
		}
	}
	// The cache must not reach the wildcard rule, which every builder holds.
	repos := zotRepos(data)
	if _, ok := repos[id.CacheRepo()]; !ok {
		t.Errorf("no ACL entry for %q at all", id.CacheRepo())
	}
}

func TestCrossWorkspaceCacheAccessIsDenied(t *testing.T) {
	// The confidentiality property. Cache layers carry build-time state and
	// source-derived content, so a workspace reading another's cache is a leak,
	// not a performance detail (docs/ADR034 §6).
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret((&Creds{}).baseZotConfig()))
	a := identity.ForApp("hello", "tea-w1")
	b := identity.ForApp("hello", "tea-w2")

	for _, id := range []identity.Identity{a, b} {
		if err := c.EnsureCredsFor(ctx, id, "default"); err != nil {
			t.Fatalf("EnsureCredsFor(%s): %v", id.Key(), err)
		}
	}
	data := storedConfig(t, c)

	// Same App name, different workspaces: neither user may appear on the
	// other's cache repository.
	for _, tc := range []struct{ owner, intruder identity.Identity }{{a, b}, {b, a}} {
		for _, actions := range [][]string{zotReadWriteActions, zotReadOnlyActions} {
			if zotRepoGrants(data, tc.owner.CacheRepo(), tc.intruder.ZotUsername(), actions) {
				t.Errorf("%s may reach %s's cache %q", tc.intruder.ZotUsername(), tc.owner.ZotUsername(), tc.owner.CacheRepo())
			}
		}
		if zotRepoHasUser(data, tc.owner.CacheRepo(), tc.intruder.ZotUsername()) {
			t.Errorf("%s appears in the policies of %q", tc.intruder.ZotUsername(), tc.owner.CacheRepo())
		}
	}
}

func TestUnlabeledAppsGetACacheRepositoryToo(t *testing.T) {
	// BEX_REGISTRY_NS set but the App unlabeled: the legacy flat column. The
	// cache follows the image into that column rather than being silently
	// skipped, so the feature behaves the same in both identity schemes.
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret((&Creds{}).baseZotConfig()))
	id := identity.ForApp("legacyapp", "")

	if err := c.EnsureCredsFor(ctx, id, "default"); err != nil {
		t.Fatalf("EnsureCredsFor: %v", err)
	}
	data := storedConfig(t, c)
	if got, want := id.CacheRepo(), "legacyapp-cache"; got != want {
		t.Errorf("legacy CacheRepo = %q, want %q", got, want)
	}
	if !zotRepoGrants(data, id.CacheRepo(), id.ZotUsername(), zotReadWriteActions) {
		t.Errorf("legacy App's cache repository %q is not granted to %q", id.CacheRepo(), id.ZotUsername())
	}
}

func TestRevokeRemovesTheCacheACLWithTheImageACL(t *testing.T) {
	// A cache repository left granted after its App is gone is a credential
	// pointing at content nobody owns — and the username is reused if an App of
	// the same name returns.
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret((&Creds{}).baseZotConfig()))
	id := identity.ForApp("hello", "tea-w1")

	if err := c.EnsureCredsFor(ctx, id, "default"); err != nil {
		t.Fatalf("EnsureCredsFor: %v", err)
	}
	if err := c.RevokeCredsFor(ctx, id); err != nil {
		t.Fatalf("RevokeCredsFor: %v", err)
	}
	repos := zotRepos(storedConfig(t, c))
	for _, repo := range []string{id.Repo(), id.CacheRepo()} {
		if _, ok := repos[repo]; ok {
			t.Errorf("ACL entry for %q survived revocation", repo)
		}
	}
}

func TestCacheACLCostsNoExtraRegistryWrite(t *testing.T) {
	// The image and cache grants are written inside one document hold. Splitting
	// them would add an optimistic-lock round-trip per App per reconcile, and
	// leave a window where one exists and the other does not — during which the
	// build's cache push is denied.
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret((&Creds{}).baseZotConfig()))
	id := identity.ForApp("hello", "tea-w1")

	wrote, err := c.ensureZotConfigEntry(ctx, id.ZotUsername(), zotReadWriteActions, id.Repo(), id.CacheRepo())
	if err != nil {
		t.Fatalf("ensureZotConfigEntry: %v", err)
	}
	if !wrote {
		t.Fatal("first grant reported no write")
	}
	// Converged: a second identical pass must not touch the Secret at all.
	wrote, err = c.ensureZotConfigEntry(ctx, id.ZotUsername(), zotReadWriteActions, id.Repo(), id.CacheRepo())
	if err != nil {
		t.Fatalf("second ensureZotConfigEntry: %v", err)
	}
	if wrote {
		t.Error("granting the same two repositories again rewrote the config; the reconcile never converges")
	}
}

func TestTombstonedAppDropsItsLegacyCacheGrant(t *testing.T) {
	// zot-config is one document shared by the whole estate, so an entry that
	// outlives its App grows it forever. An App that built while unlabeled owns a
	// "<name>-cache" grant; relabelling and tombstoning it must take that with
	// the legacy image grant rather than leaving an orphan nothing can reach.
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret((&Creds{}).baseZotConfig()))
	legacy := identity.ForApp("hello", "")
	if err := c.EnsureCredsFor(ctx, legacy, "default"); err != nil {
		t.Fatalf("EnsureCredsFor(legacy): %v", err)
	}
	scoped := identity.Identity{Name: "hello", Workspace: "tea-w1", Tombstoned: true}
	if err := c.EnsureCredsFor(ctx, scoped, "default"); err != nil {
		t.Fatalf("EnsureCredsFor(scoped): %v", err)
	}
	if err := c.RevokeCredsFor(ctx, scoped); err != nil {
		t.Fatalf("RevokeCredsFor: %v", err)
	}
	repos := zotRepos(storedConfig(t, c))
	for _, repo := range []string{
		scoped.Repo(), scoped.CacheRepo(), scoped.LegacyRepo(), scoped.LegacyCacheRepo(),
	} {
		if _, ok := repos[repo]; ok {
			t.Errorf("ACL entry for %q survived a tombstoned revocation", repo)
		}
	}
}
