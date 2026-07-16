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
	"encoding/json"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestNamingConventions pins the per-App naming scheme so refactors don't drift.
func TestNamingConventions(t *testing.T) {
	if got := PullSecretName("hello"); got != "reg-pull-hello" {
		t.Errorf("PullSecretName = %q; want %q", got, "reg-pull-hello")
	}
	if got := ZotUsername("hello"); got != "app-hello" {
		t.Errorf("ZotUsername = %q; want %q", got, "app-hello")
	}
}

// TestHTPasswdRoundTrip verifies add → detect → remove cycle.
func TestHTPasswdRoundTrip(t *testing.T) {
	// Start with a non-empty htpasswd (bex-builder line, as in production).
	initial := []byte("bex-builder:$2a$10$somehashhere\n")

	// Add new user.
	updated, err := addHTPasswdLine(initial, "app-foo", "mysecret")
	if err != nil {
		t.Fatalf("addHTPasswdLine: %v", err)
	}
	if !htpasswdHasUser(updated, "app-foo") {
		t.Fatal("app-foo should be present after add")
	}
	if !htpasswdHasUser(updated, "bex-builder") {
		t.Fatal("bex-builder should survive after adding app-foo")
	}

	// Verify the hash is valid bcrypt for the password.
	var fooHash string
	for _, line := range strings.Split(string(updated), "\n") {
		if strings.HasPrefix(line, "app-foo:") {
			fooHash = strings.TrimPrefix(line, "app-foo:")
		}
	}
	if fooHash == "" {
		t.Fatal("could not find app-foo hash in htpasswd")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(fooHash), []byte("mysecret")); err != nil {
		t.Errorf("bcrypt.CompareHashAndPassword: %v", err)
	}

	// Remove the user; bex-builder must remain.
	removed := removeHTPasswdLine(updated, "app-foo")
	if htpasswdHasUser(removed, "app-foo") {
		t.Fatal("app-foo should be absent after remove")
	}
	if !htpasswdHasUser(removed, "bex-builder") {
		t.Fatal("bex-builder should remain after removing app-foo")
	}

	// Idempotent remove on non-existent user is a no-op.
	again := removeHTPasswdLine(removed, "app-foo")
	if string(again) != string(removed) {
		t.Error("remove of non-existent user should be a no-op")
	}
}

// TestHTPasswdUpdateReplaces verifies that adding an existing user replaces the
// old hash (handles rotation / credential regeneration).
func TestHTPasswdUpdateReplaces(t *testing.T) {
	first, _ := addHTPasswdLine(nil, "app-bar", "pass1")
	second, _ := addHTPasswdLine(first, "app-bar", "pass2")

	var count int
	for _, line := range strings.Split(string(second), "\n") {
		if strings.HasPrefix(line, "app-bar:") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 app-bar entry after update, got %d", count)
	}
}

// TestZotConfigACLRoundTrip verifies add → detect → remove cycle for the Zot config.
func TestZotConfigACLRoundTrip(t *testing.T) {
	base := baseZotConfig()

	// Add per-App entry.
	updated, err := addZotACLEntry(base, "myapp", "app-myapp")
	if err != nil {
		t.Fatalf("addZotACLEntry: %v", err)
	}
	if !zotConfigHasRepo(updated, "myapp") {
		t.Fatal("myapp repo should be present after add")
	}
	// bex-builder's ** wildcard must remain.
	if !zotConfigHasRepo(updated, "**") {
		t.Fatal("** wildcard must survive after adding myapp")
	}

	// Verify the ACL content.
	repos := zotRepos(updated)
	myappRaw, ok := repos["myapp"]
	if !ok {
		t.Fatal("myapp not in parsed repos")
	}
	myappMap, _ := myappRaw.(map[string]interface{})
	policies, _ := myappMap["policies"].([]interface{})
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy for myapp, got %d", len(policies))
	}
	pol, _ := policies[0].(map[string]interface{})
	users, _ := pol["users"].([]interface{})
	if len(users) != 1 || users[0] != "app-myapp" {
		t.Errorf("unexpected users %v, want [app-myapp]", users)
	}
	actions, _ := pol["actions"].([]interface{})
	if len(actions) != 1 || actions[0] != "read" {
		t.Errorf("unexpected actions %v, want [read]", actions)
	}

	// Remove entry; ** must remain.
	final, err := removeZotACLEntry(updated, "myapp")
	if err != nil {
		t.Fatalf("removeZotACLEntry: %v", err)
	}
	if zotConfigHasRepo(final, "myapp") {
		t.Fatal("myapp should be absent after remove")
	}
	if !zotConfigHasRepo(final, "**") {
		t.Fatal("** wildcard should remain after remove")
	}
}

// TestZotConfigIsolation verifies that per-App users are only added to their
// own repo — not to the global ** wildcard.
func TestZotConfigIsolation(t *testing.T) {
	cfg := baseZotConfig()
	for _, app := range []string{"alpha", "beta"} {
		var err error
		cfg, err = addZotACLEntry(cfg, app, ZotUsername(app))
		if err != nil {
			t.Fatalf("add %s: %v", app, err)
		}
	}

	// Parse the resulting config.
	var data map[string]interface{}
	if err := json.Unmarshal(cfg, &data); err != nil {
		t.Fatal(err)
	}
	repos := zotReposMap(data)

	// ** wildcard must only have bex-builder.
	wildcardRaw, ok := repos["**"]
	if !ok {
		t.Fatal("** wildcard missing")
	}
	wm, _ := wildcardRaw.(map[string]interface{})
	policies, _ := wm["policies"].([]interface{})
	for _, p := range policies {
		pm, _ := p.(map[string]interface{})
		users, _ := pm["users"].([]interface{})
		for _, u := range users {
			if u == "app-alpha" || u == "app-beta" {
				t.Errorf("per-app user %v must not appear in ** wildcard policy", u)
			}
		}
	}

	// Each App's user must appear only in its own repo, not the other's.
	for _, app := range []string{"alpha", "beta"} {
		other := "beta"
		if app == "beta" {
			other = "alpha"
		}
		repoRaw, ok := repos[app]
		if !ok {
			t.Errorf("%s repo missing", app)
			continue
		}
		rm, _ := repoRaw.(map[string]interface{})
		rpolicies, _ := rm["policies"].([]interface{})
		for _, p := range rpolicies {
			pm, _ := p.(map[string]interface{})
			users, _ := pm["users"].([]interface{})
			for _, u := range users {
				if u == ZotUsername(other) {
					t.Errorf("%s's user appears in %s's ACL — cross-tenant leak", other, app)
				}
			}
		}
	}
}

// TestDockerConfigRoundTrip verifies buildDockerConfig + extractPassword.
func TestDockerConfigRoundTrip(t *testing.T) {
	const reg = "zot.bex-registry.svc:5000"
	cfg := buildDockerConfig("app-foo", "supersecret", reg, "")
	pass, err := extractPassword(cfg, reg, "app-foo")
	if err != nil {
		t.Fatalf("extractPassword: %v", err)
	}
	if pass != "supersecret" {
		t.Errorf("got %q; want %q", pass, "supersecret")
	}
}

// TestDockerConfigIncludesKpackAlias verifies both registries are included when
// the kpack alias differs from the canonical registry.
func TestDockerConfigIncludesKpackAlias(t *testing.T) {
	const reg = "zot.bex-registry.svc:5000"
	const kpack = "zot.local:5000"
	cfg := buildDockerConfig("app-foo", "pass", reg, kpack)
	var parsed struct {
		Auths map[string]struct {
			Username string `json:"username"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(cfg, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed.Auths[reg]; !ok {
		t.Errorf("canonical registry %q not in docker config", reg)
	}
	if _, ok := parsed.Auths[kpack]; !ok {
		t.Errorf("kpack alias %q not in docker config", kpack)
	}
}

// TestBaseZotConfigNoBexPuller asserts the base Zot config no longer includes
// the shared bex-puller user, closing ADR022:204's shared-credential residual.
func TestBaseZotConfigNoBexPuller(t *testing.T) {
	cfg := baseZotConfig()
	repos := zotRepos(cfg)
	wildcardRaw, ok := repos["**"]
	if !ok {
		t.Fatal("** wildcard missing from base config")
	}
	wm, _ := wildcardRaw.(map[string]interface{})
	policies, _ := wm["policies"].([]interface{})
	for _, p := range policies {
		pm, _ := p.(map[string]interface{})
		users, _ := pm["users"].([]interface{})
		for _, u := range users {
			if u == "bex-puller" {
				t.Error("bex-puller shared user must not appear in base config ** wildcard (w7/m36 closes ADR022:204)")
			}
		}
	}
}
