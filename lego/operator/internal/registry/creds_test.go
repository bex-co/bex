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
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"golang.org/x/crypto/bcrypt"

	"github.com/bex-co/bex/lego/operator/internal/identity"
)

// -- fixtures -----------------------------------------------------------------

const (
	testZotNS   = "bex-registry"
	testAppNS   = "default"
	testAppName = "myapp"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

// newTestCreds wires a Creds to a fake cluster preloaded with objs. It does not
// create the Zot Secrets; tests add the ones their path requires.
func newTestCreds(t *testing.T, objs ...client.Object) *Creds {
	t.Helper()
	return &Creds{
		Client:       fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objs...).Build(),
		ZotNamespace: testZotNS,
		HTPasswdName: "zot-htpasswd",
		ConfigName:   "zot-config",
		Registry:     "zot.bex-registry.svc:5000",
	}
}

// htpasswdSecret is the out-of-band Secret the Zot chart mounts, seeded with
// the platform builder entry production always carries.
func htpasswdSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: testZotNS},
		Data:       map[string][]byte{htpasswdKey: []byte("bex-builder:$2a$10$somehashhere\n")},
	}
}

func zotConfigSecret(configJSON []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-config", Namespace: testZotNS},
		Data:       map[string][]byte{zotConfigKey: configJSON},
	}
}

func mustDecode(t *testing.T, configJSON []byte) map[string]any {
	t.Helper()
	data, err := decodeZotConfig(configJSON)
	if err != nil {
		t.Fatalf("decode zot config: %v", err)
	}
	return data
}

func readSecret(t *testing.T, c *Creds, ns, name string) *corev1.Secret {
	t.Helper()
	var sec corev1.Secret
	if err := c.Client.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: name}, &sec); err != nil {
		t.Fatalf("get %s/%s: %v", ns, name, err)
	}
	return &sec
}

func storedConfig(t *testing.T, c *Creds) map[string]any {
	t.Helper()
	return mustDecode(t, readSecret(t, c, testZotNS, "zot-config").Data[zotConfigKey])
}

// -- naming -------------------------------------------------------------------

// TestNamingConventions pins the per-App naming scheme so refactors don't drift.
func TestNamingConventions(t *testing.T) {
	if got := PullSecretName("hello"); got != "reg-pull-hello" {
		t.Errorf("PullSecretName = %q; want %q", got, "reg-pull-hello")
	}
	if got := ZotUsername("hello"); got != "app-hello" {
		t.Errorf("ZotUsername = %q; want %q", got, "app-hello")
	}
}

// -- htpasswd -----------------------------------------------------------------

// TestHTPasswdRoundTrip verifies add → detect → remove cycle.
func TestHTPasswdRoundTrip(t *testing.T) {
	initial := []byte("bex-builder:$2a$10$somehashhere\n")

	updated, err := setHTPasswdLine(initial, "app-foo", "mysecret")
	if err != nil {
		t.Fatalf("setHTPasswdLine: %v", err)
	}
	hash, found := htpasswdUserHash(updated, "app-foo")
	if !found {
		t.Fatal("app-foo should be present after add")
	}
	if _, ok := htpasswdUserHash(updated, "bex-builder"); !ok {
		t.Fatal("bex-builder should survive after adding app-foo")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("mysecret")); err != nil {
		t.Errorf("bcrypt.CompareHashAndPassword: %v", err)
	}

	removed := removeHTPasswdLine(updated, "app-foo")
	if _, ok := htpasswdUserHash(removed, "app-foo"); ok {
		t.Fatal("app-foo should be absent after remove")
	}
	if _, ok := htpasswdUserHash(removed, "bex-builder"); !ok {
		t.Fatal("bex-builder should remain after removing app-foo")
	}

	again := removeHTPasswdLine(removed, "app-foo")
	if string(again) != string(removed) {
		t.Error("remove of non-existent user should be a no-op")
	}
}

// TestHTPasswdUpdateReplaces verifies that adding an existing user replaces the
// old hash (handles rotation / credential regeneration).
func TestHTPasswdUpdateReplaces(t *testing.T) {
	first, _ := setHTPasswdLine(nil, "app-bar", "pass1")
	second, _ := setHTPasswdLine(first, "app-bar", "pass2")

	var count int
	for line := range strings.SplitSeq(string(second), "\n") {
		if strings.HasPrefix(line, "app-bar:") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 app-bar entry after update, got %d", count)
	}
	hash, _ := htpasswdUserHash(second, "app-bar")
	if err := bcrypt.CompareHashAndPassword(hash, []byte("pass2")); err != nil {
		t.Errorf("surviving hash must match the newest password: %v", err)
	}
}

// TestHTPasswdDrainedFileIsEmpty pins that revoking the last user leaves no
// bytes rather than a bare newline, so a drained registry holds no entries.
func TestHTPasswdDrainedFileIsEmpty(t *testing.T) {
	only, err := setHTPasswdLine(nil, "app-solo", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if got := removeHTPasswdLine(only, "app-solo"); len(got) != 0 {
		t.Errorf("drained htpasswd = %q; want empty", got)
	}
}

// -- Zot config: ACL round trips ----------------------------------------------

// TestZotConfigACLRoundTrip verifies add → detect → remove for a tenant repo.
func TestZotConfigACLRoundTrip(t *testing.T) {
	data := mustDecode(t, (&Creds{}).baseZotConfig())

	setZotRepoPolicy(data, "myapp", "app-myapp", zotReadWriteActions)
	if !zotHasRepo(data, "myapp") {
		t.Fatal("myapp repo should be present after add")
	}
	if !zotHasRepo(data, "**") {
		t.Fatal("** wildcard must survive after adding myapp")
	}
	if !zotHasBuilderAdminPolicy(data) {
		t.Fatal("bex-builder admin policy must survive after adding myapp")
	}
	if !zotRepoGrants(data, "myapp", "app-myapp", zotReadWriteActions) {
		t.Fatal("myapp must grant its own user read/create/update/delete")
	}

	entry, _ := zotRepos(data)["myapp"].(map[string]any)
	policies, _ := entry["policies"].([]any)
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy for myapp, got %d", len(policies))
	}
	policy, _ := policies[0].(map[string]any)
	actions, _ := policy["actions"].([]any)
	if len(actions) != 4 || actions[0] != "read" || actions[1] != "create" || actions[2] != "update" || actions[3] != "delete" {
		t.Errorf("unexpected actions %v, want repository-scoped read/create/update/delete", actions)
	}

	delete(zotRepos(data), "myapp")
	if zotHasRepo(data, "myapp") {
		t.Fatal("myapp should be absent after remove")
	}
	if !zotHasRepo(data, "**") {
		t.Fatal("** wildcard should remain after remove")
	}
}

// TestZotRepoGrantsIsExact proves the convergence predicate rejects a widened
// policy. Tolerating an extra user would leave a cross-tenant grant in place
// forever, since the operator only rewrites entries it considers unconverged.
func TestZotRepoGrantsIsExact(t *testing.T) {
	widenedUsers := mustDecode(t, (&Creds{}).baseZotConfig())
	zotReposFor(widenedUsers)["myapp"] = map[string]any{
		"policies": []any{map[string]any{
			"users":   []any{"app-myapp", "app-attacker"},
			"actions": anySlice(zotReadWriteActions),
		}},
	}
	if zotRepoGrants(widenedUsers, "myapp", "app-myapp", zotReadWriteActions) {
		t.Error("a policy naming a second user must not count as converged")
	}

	widenedActions := mustDecode(t, (&Creds{}).baseZotConfig())
	zotReposFor(widenedActions)["snapshots/tea-a-sandbox/**"] = map[string]any{
		"policies": []any{map[string]any{
			"users":   []any{"snap-tea-a-sandbox"},
			"actions": anySlice(zotReadWriteActions),
		}},
	}
	if zotRepoGrants(widenedActions, "snapshots/tea-a-sandbox/**", "snap-tea-a-sandbox", zotReadOnlyActions) {
		t.Error("a read-only grant must not be satisfied by a read/write policy")
	}

	// Order must not matter — Zot does not care, and neither should convergence.
	shuffled := mustDecode(t, (&Creds{}).baseZotConfig())
	zotReposFor(shuffled)["myapp"] = map[string]any{
		"policies": []any{map[string]any{
			"users":   []any{"app-myapp"},
			"actions": []any{"delete", "read", "update", "create"},
		}},
	}
	if !zotRepoGrants(shuffled, "myapp", "app-myapp", zotReadWriteActions) {
		t.Error("action order must not trigger a needless rewrite")
	}
}

// TestZotConfigBuilderAdminPolicyMigration verifies that existing configs are
// upgraded even when the per-App repository entry already exists. This is the
// production regression path: Zot's exact repo policy shadows the ** policy.
func TestZotConfigBuilderAdminPolicyMigration(t *testing.T) {
	data := mustDecode(t, (&Creds{}).baseZotConfig())
	delete(zotAccessControl(data), "adminPolicy")
	setZotRepoPolicy(data, "myapp", "app-myapp", zotReadWriteActions)
	if zotHasBuilderAdminPolicy(data) {
		t.Fatal("historical config unexpectedly has builder admin policy")
	}

	setZotBuilderAdminPolicy(data)
	if !zotHasBuilderAdminPolicy(data) {
		t.Fatal("builder admin policy missing after migration")
	}
	if !zotHasRepo(data, "myapp") {
		t.Fatal("migration removed existing per-App repository ACL")
	}
}

// TestZotConfigStorageMigration verifies that an existing production config
// receives new GC/storage policy without losing tenant ACLs or other extensions.
func TestZotConfigStorageMigration(t *testing.T) {
	c := &Creds{}
	data := mustDecode(t, c.baseZotConfig())
	setZotRepoPolicy(data, "myapp", "app-myapp", zotReadWriteActions)

	storage, _ := data["storage"].(map[string]any)
	storage["gcInterval"] = "24h"
	delete(storage, "dedupe")
	data["extensions"] = map[string]any{"sentinel": true}

	if !setZotStorage(data, c.canonicalStorage()) {
		t.Fatal("historical storage policy was not migrated")
	}
	if !zotRepoGrants(data, "myapp", "app-myapp", zotReadWriteActions) {
		t.Fatal("storage migration removed the existing per-App ACL")
	}
	if !zotHasBuilderAdminPolicy(data) {
		t.Fatal("storage migration removed the builder admin policy")
	}
	migrated, _ := data["storage"].(map[string]any)
	if migrated["gcInterval"] != "1h" || migrated["dedupe"] != true {
		t.Errorf("migrated storage = %v; want hourly GC with dedupe", migrated)
	}
	extensions, _ := data["extensions"].(map[string]any)
	if extensions["sentinel"] != true {
		t.Fatal("storage migration removed an unrelated extension")
	}

	if setZotStorage(data, c.canonicalStorage()) {
		t.Fatal("canonical storage migration must be idempotent")
	}
}

// TestZotConfigIsolation verifies that per-App users are only added to their
// own repo — not to the global ** wildcard.
func TestZotConfigIsolation(t *testing.T) {
	data := mustDecode(t, (&Creds{}).baseZotConfig())
	for _, app := range []string{"alpha", "beta"} {
		setZotRepoPolicy(data, app, ZotUsername(app), zotReadWriteActions)
	}
	repos := zotRepos(data)

	wildcard, _ := repos["**"].(map[string]any)
	policies, _ := wildcard["policies"].([]any)
	for _, p := range policies {
		pm, _ := p.(map[string]any)
		users, _ := pm["users"].([]any)
		for _, u := range users {
			if u == "app-alpha" || u == "app-beta" {
				t.Errorf("per-app user %v must not appear in ** wildcard policy", u)
			}
		}
	}

	for _, app := range []string{"alpha", "beta"} {
		other := "beta"
		if app == "beta" {
			other = "alpha"
		}
		if zotRepoGrants(data, app, ZotUsername(other), zotReadWriteActions) {
			t.Errorf("%s's user must not be granted %s's repository — cross-tenant leak", other, app)
		}
	}
}

// TestZotConfigPreservesUnmodeledFields is the load-bearing invariant of this
// package's map-based edits: zot-config carries Zot settings the operator does
// not model (extensions, TLS, alternate auth backends, storage drivers), and a
// full read-modify-write cycle must return every one of them untouched. A typed
// round-trip through the canonical document would silently erase them on the
// next reconcile of any App.
func TestZotConfigPreservesUnmodeledFields(t *testing.T) {
	ctx := context.Background()
	data := mustDecode(t, (&Creds{}).baseZotConfig())
	foreign := map[string]any{
		"extensions": map[string]any{"search": map[string]any{"enable": true}},
		"scheduler":  map[string]any{"numWorkers": float64(3)},
		"cluster":    map[string]any{"members": []any{"zot-0", "zot-1"}},
	}
	maps.Copy(data, foreign)
	httpBlock, _ := data["http"].(map[string]any)
	httpBlock["realm"] = "bex"
	httpBlock["tls"] = map[string]any{"cert": "/certs/tls.crt"}
	seeded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret(seeded))
	if _, err := c.ensureZotConfigEntry(ctx, "app-myapp", zotReadWriteActions, "myapp"); err != nil {
		t.Fatalf("ensureZotConfigEntry: %v", err)
	}
	if _, err := c.removeZotConfigEntry(ctx, "myapp"); err != nil {
		t.Fatalf("removeZotConfigEntry: %v", err)
	}

	got := storedConfig(t, c)
	for key, want := range foreign {
		if !jsonEqual(got[key], want) {
			t.Errorf("top-level %q = %#v after a full ACL cycle; want %#v", key, got[key], want)
		}
	}
	gotHTTP, _ := got["http"].(map[string]any)
	if gotHTTP["realm"] != "bex" {
		t.Errorf("http.realm = %#v; want preserved", gotHTTP["realm"])
	}
	if !jsonEqual(gotHTTP["tls"], map[string]any{"cert": "/certs/tls.crt"}) {
		t.Errorf("http.tls = %#v; want preserved", gotHTTP["tls"])
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// -- docker config ------------------------------------------------------------

// TestDockerConfigRoundTrip verifies dockerConfig + passwordFrom.
func TestDockerConfigRoundTrip(t *testing.T) {
	c := &Creds{Registry: "zot.bex-registry.svc:5000"}
	cfg := c.dockerConfig("app-foo", "supersecret")

	pass, err := c.passwordFrom(cfg, "app-foo")
	if err != nil {
		t.Fatalf("passwordFrom: %v", err)
	}
	if pass != "supersecret" {
		t.Errorf("got %q; want %q", pass, "supersecret")
	}

	var parsed struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(cfg, &parsed); err != nil {
		t.Fatal(err)
	}
	wantAuth := base64.StdEncoding.EncodeToString([]byte("app-foo:supersecret"))
	if parsed.Auths[c.Registry].Auth != wantAuth {
		t.Error("docker config must include the standard auth field consumed by skopeo/containers-image")
	}

	if _, err := c.passwordFrom(cfg, "app-other"); err == nil {
		t.Error("a credential minted for another user must not be accepted")
	}
	if _, err := (&Creds{Registry: "elsewhere:5000"}).passwordFrom(cfg, "app-foo"); err == nil {
		t.Error("a config without an entry for the canonical registry must error")
	}
}

// TestDockerConfigIncludesKpackAlias verifies both registries are included when
// the kpack alias differs from the canonical registry.
func TestDockerConfigIncludesKpackAlias(t *testing.T) {
	c := &Creds{Registry: "zot.bex-registry.svc:5000", KpackRegistry: "zot.local:5000"}
	cfg := c.dockerConfig("app-foo", "pass")

	var parsed struct {
		Auths map[string]struct {
			Username string `json:"username"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(cfg, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed.Auths[c.Registry]; !ok {
		t.Errorf("canonical registry %q not in docker config", c.Registry)
	}
	if _, ok := parsed.Auths[c.KpackRegistry]; !ok {
		t.Errorf("kpack alias %q not in docker config", c.KpackRegistry)
	}
}

// -- base config --------------------------------------------------------------

// TestBaseZotConfigNoBexPuller asserts the base Zot config no longer includes
// the shared bex-puller user, closing ADR022:204's shared-credential residual.
func TestBaseZotConfigNoBexPuller(t *testing.T) {
	data := mustDecode(t, (&Creds{}).baseZotConfig())
	wildcard, _ := zotRepos(data)["**"].(map[string]any)
	policies, _ := wildcard["policies"].([]any)
	if len(policies) == 0 {
		t.Fatal("** wildcard missing from base config")
	}
	for _, p := range policies {
		pm, _ := p.(map[string]any)
		users, _ := pm["users"].([]any)
		for _, u := range users {
			if u == "bex-puller" {
				t.Error("bex-puller shared user must not appear in base config ** wildcard (w7/m36 closes ADR022:204)")
			}
		}
	}
}

// TestBaseZotConfigBuilderAdminPolicy pins the global builder authorization.
// Zot exact repository rules override **, so this must remain an adminPolicy.
func TestBaseZotConfigBuilderAdminPolicy(t *testing.T) {
	if !zotHasBuilderAdminPolicy(mustDecode(t, (&Creds{}).baseZotConfig())) {
		t.Fatal("base config must grant bex-builder global read/create/update/delete through adminPolicy")
	}
}

// TestBaseZotConfigIsByteStable pins the exact canonical document. The typed
// shape that produces it is only ever marshalled, never used to parse a stored
// config, so this is the one place a field rename or an omitempty change would
// otherwise slip through to a running registry unnoticed.
func TestBaseZotConfigIsByteStable(t *testing.T) {
	const want = `{"distSpecVersion":"1.1.0","storage":{"rootDirectory":"/var/lib/registry","dedupe":true,"gc":true,"gcDelay":"1h","gcInterval":"1h","retention":{"dryRun":false,"policies":[{"repositories":["**"],"deleteUntagged":true,"keepTags":[{"patterns":[".*"],"mostRecentlyPushedCount":5}]}]}},"http":{"address":"0.0.0.0","port":"5000","compat":["docker2s2"],"readTimeout":"60s","writeTimeout":"60s","auth":{"htpasswd":{"path":"/secret/htpasswd"}},"accessControl":{"repositories":{"**":{"policies":[{"users":["bex-builder"],"actions":["read","create","update","delete"]}]},"bex-cnb-builder":{"policies":null,"defaultPolicy":["read"]}},"adminPolicy":{"users":["bex-builder"],"actions":["read","create","update","delete"]}}},"log":{"level":"info"},"extensions":{"scrub":{"enable":true,"interval":"24h"}}}`
	if got := string((&Creds{}).baseZotConfig()); got != want {
		t.Errorf("canonical zot config drifted.\n got: %s\nwant: %s", got, want)
	}
}

func TestPlatformBuilderRepositoryIsReadOnlyForAuthenticatedApps(t *testing.T) {
	ctx := context.Background()
	if !zotHasPlatformBuilderPolicy(mustDecode(t, (&Creds{}).baseZotConfig())) {
		t.Fatal("base config must grant authenticated builds read-only access to the shared kpack builder")
	}

	legacy := []byte(`{"http":{"accessControl":{"repositories":{"**":{"defaultPolicy":[]}},"adminPolicy":{"users":["bex-builder"],"actions":["read","create","update","delete"]}}}}`)
	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret(legacy))

	// A tenant service whose public name collides with the platform repository
	// must gain neither write permission nor the power to strip the shared rule.
	if _, err := c.ensureZotConfigEntry(ctx, ZotUsername(platformBuilderRepository), zotReadWriteActions, platformBuilderRepository); err != nil {
		t.Fatal(err)
	}
	data := storedConfig(t, c)
	if !zotHasPlatformBuilderPolicy(data) {
		t.Fatal("legacy config did not gain the shared builder read policy")
	}
	if zotRepoGrants(data, platformBuilderRepository, ZotUsername(platformBuilderRepository), zotReadWriteActions) {
		t.Fatal("colliding App replaced the platform builder read-only policy")
	}

	if _, err := c.removeZotConfigEntry(ctx, platformBuilderRepository); err != nil {
		t.Fatal(err)
	}
	if !zotHasPlatformBuilderPolicy(storedConfig(t, c)) {
		t.Fatal("colliding App deletion removed the platform builder policy")
	}
}

// TestBaseZotConfigContractValues pins the Zot config contract so drift is
// detected at test time, not when the Zot pod fails to start.
func TestBaseZotConfigContractValues(t *testing.T) {
	data := mustDecode(t, (&Creds{RetentionCount: 0}).baseZotConfig())

	httpBlock, _ := data["http"].(map[string]any)
	if port, _ := httpBlock["port"].(string); port != zotHTTPPort {
		t.Errorf("http.port = %q; want %q (contract with Helm chart)", port, zotHTTPPort)
	}
	auth, _ := httpBlock["auth"].(map[string]any)
	htpasswd, _ := auth["htpasswd"].(map[string]any)
	if path, _ := htpasswd["path"].(string); path != zotHTPasswdPath {
		t.Errorf("http.auth.htpasswd.path = %q; want %q (contract with Helm chart)", path, zotHTPasswdPath)
	}

	storage, _ := data["storage"].(map[string]any)
	if dedupe, _ := storage["dedupe"].(bool); !dedupe {
		t.Error("storage.dedupe must be true")
	}
	if gc, _ := storage["gc"].(bool); !gc {
		t.Error("storage.gc must be true")
	}
	if delay, _ := storage["gcDelay"].(string); delay != "1h" {
		t.Errorf("storage.gcDelay = %q; want 1h", delay)
	}
	if interval, _ := storage["gcInterval"].(string); interval != "1h" {
		t.Errorf("storage.gcInterval = %q; want 1h", interval)
	}
	if got := retentionCount(t, data); got != defaultRetentionCount {
		t.Errorf("default mostRecentlyPushedCount = %d; want %d", got, defaultRetentionCount)
	}
}

// TestBaseZotConfigRetentionCountOverride verifies BEX_ZOT_RETENTION_COUNT
// (via Creds.RetentionCount) overrides the default.
func TestBaseZotConfigRetentionCountOverride(t *testing.T) {
	if got := retentionCount(t, mustDecode(t, (&Creds{RetentionCount: 10}).baseZotConfig())); got != 10 {
		t.Errorf("mostRecentlyPushedCount = %d; want 10", got)
	}
}

func retentionCount(t *testing.T, data map[string]any) int {
	t.Helper()
	storage, _ := data["storage"].(map[string]any)
	retention, _ := storage["retention"].(map[string]any)
	policies, _ := retention["policies"].([]any)
	if len(policies) == 0 {
		t.Fatal("storage.retention.policies is empty")
	}
	policy, _ := policies[0].(map[string]any)
	keepTags, _ := policy["keepTags"].([]any)
	if len(keepTags) == 0 {
		t.Fatal("keepTags is empty")
	}
	kt, _ := keepTags[0].(map[string]any)
	count, _ := kt["mostRecentlyPushedCount"].(float64)
	return int(count)
}

// -- EnsureCreds / RevokeCreds against a cluster ------------------------------

// TestEnsureCredsEndToEnd exercises the full per-App mint: pull Secret in the
// App namespace, htpasswd entry, read/write ACL — then idempotency.
func TestEnsureCredsEndToEnd(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())

	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("EnsureCreds: %v", err)
	}

	pullSec := readSecret(t, c, testAppNS, PullSecretName(testAppName))
	if pullSec.Type != corev1.SecretTypeDockerConfigJson {
		t.Fatalf("pull secret type = %v; want dockerconfigjson", pullSec.Type)
	}
	if pullSec.Labels["app.bex.co/app"] != testAppName {
		t.Errorf("pull secret labels = %v; want the App identity stamped", pullSec.Labels)
	}
	password, err := c.passwordFrom(pullSec.Data[corev1.DockerConfigJsonKey], ZotUsername(testAppName))
	if err != nil || password == "" {
		t.Fatalf("passwordFrom: %v (empty=%v)", err, password == "")
	}

	ht := readSecret(t, c, testZotNS, "zot-htpasswd")
	hash, found := htpasswdUserHash(ht.Data[htpasswdKey], ZotUsername(testAppName))
	if !found {
		t.Fatal("htpasswd entry missing")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Errorf("htpasswd hash must verify the pull Secret's password: %v", err)
	}
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], "bex-builder"); !ok {
		t.Fatal("bex-builder must survive")
	}

	// zot-config is bootstrapped from the canonical base, ACL included.
	data := storedConfig(t, c)
	if !zotRepoGrants(data, testAppName, ZotUsername(testAppName), zotReadWriteActions) {
		t.Fatal("per-App ACL missing after bootstrap")
	}
	if !zotHasBuilderAdminPolicy(data) || !zotHasPlatformBuilderPolicy(data) {
		t.Fatal("bootstrapped config must carry the platform grants")
	}

	// A converged reconcile must not write: no new bcrypt hash, no Secret churn.
	htRV := readSecret(t, c, testZotNS, "zot-htpasswd").ResourceVersion
	cfgRV := readSecret(t, c, testZotNS, "zot-config").ResourceVersion
	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("second EnsureCreds: %v", err)
	}
	if got := readSecret(t, c, testZotNS, "zot-htpasswd").ResourceVersion; got != htRV {
		t.Errorf("converged reconcile rewrote zot-htpasswd (rv %s -> %s)", htRV, got)
	}
	if got := readSecret(t, c, testZotNS, "zot-config").ResourceVersion; got != cfgRV {
		t.Errorf("converged reconcile rewrote zot-config (rv %s -> %s)", cfgRV, got)
	}
}

// TestEnsureCredsResyncsStaleHash covers the pull-Secret-recreated path: a new
// password was generated behind the operator's back, so the htpasswd hash no
// longer verifies and must be rewritten.
func TestEnsureCredsResyncsStaleHash(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())
	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("EnsureCreds: %v", err)
	}

	// Recreate the pull Secret with a different password, as a manual delete
	// followed by the next reconcile would.
	pullSec := readSecret(t, c, testAppNS, PullSecretName(testAppName))
	pullSec.Data[corev1.DockerConfigJsonKey] = c.dockerConfig(ZotUsername(testAppName), "rotated-behind-our-back")
	if err := c.Client.Update(ctx, pullSec); err != nil {
		t.Fatal(err)
	}

	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("resync EnsureCreds: %v", err)
	}
	hash, found := htpasswdUserHash(readSecret(t, c, testZotNS, "zot-htpasswd").Data[htpasswdKey], ZotUsername(testAppName))
	if !found {
		t.Fatal("htpasswd entry vanished")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("rotated-behind-our-back")); err != nil {
		t.Errorf("stale htpasswd hash was not re-synced to the current password: %v", err)
	}
}

// TestEnsureCredsRepairsWidenedACL proves a cross-tenant grant that appears in
// zot-config is actively removed, not merely never created.
func TestEnsureCredsRepairsWidenedACL(t *testing.T) {
	ctx := context.Background()
	tampered := mustDecode(t, (&Creds{}).baseZotConfig())
	zotReposFor(tampered)[testAppName] = map[string]any{
		"policies": []any{map[string]any{
			"users":   []any{ZotUsername(testAppName), "app-attacker"},
			"actions": anySlice(zotReadWriteActions),
		}},
	}
	seeded, err := json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret(seeded))

	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("EnsureCreds: %v", err)
	}
	data := storedConfig(t, c)
	if !zotRepoGrants(data, testAppName, ZotUsername(testAppName), zotReadWriteActions) {
		t.Fatal("the App's own grant must survive the repair")
	}
	entry, _ := zotRepos(data)[testAppName].(map[string]any)
	policies, _ := entry["policies"].([]any)
	policy, _ := policies[0].(map[string]any)
	users, _ := policy["users"].([]any)
	for _, u := range users {
		if u == "app-attacker" {
			t.Fatal("a foreign user on the App's repository must be repaired away")
		}
	}
}

// TestRevokeCredsRemovesBothAndIsIdempotent covers the delete path.
func TestRevokeCredsRemovesBothAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())
	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("EnsureCreds: %v", err)
	}

	if err := c.RevokeCreds(ctx, testAppName); err != nil {
		t.Fatalf("RevokeCreds: %v", err)
	}
	ht := readSecret(t, c, testZotNS, "zot-htpasswd")
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], ZotUsername(testAppName)); ok {
		t.Fatal("htpasswd entry should be revoked")
	}
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], "bex-builder"); !ok {
		t.Fatal("revoking one App must not disturb the builder entry")
	}
	if zotHasRepo(storedConfig(t, c), testAppName) {
		t.Fatal("ACL entry should be revoked")
	}

	if err := c.RevokeCreds(ctx, testAppName); err != nil {
		t.Fatalf("revoke must be idempotent: %v", err)
	}
	if err := c.RevokeCreds(ctx, "never-existed"); err != nil {
		t.Fatalf("revoke of an unknown App must be a no-op: %v", err)
	}
}

// TestRevokeCredsMissingZotSecretsIsNoOp pins that a torn-down registry does
// not block App finalizers.
func TestRevokeCredsMissingZotSecretsIsNoOp(t *testing.T) {
	c := newTestCreds(t)
	if err := c.RevokeCreds(context.Background(), testAppName); err != nil {
		t.Fatalf("revoke without the zot Secrets must be a no-op: %v", err)
	}
}

// TestRevokeCredsMalformedConfigErrors proves a corrupt zot-config surfaces
// instead of reporting a successful revoke while the ACL entry stays live.
func TestRevokeCredsMalformedConfigErrors(t *testing.T) {
	c := newTestCreds(t, htpasswdSecret(), zotConfigSecret([]byte("{ not json")))
	err := c.RevokeCreds(context.Background(), testAppName)
	if err == nil {
		t.Fatal("a malformed zot-config must not report a successful revoke")
	}
	if !strings.Contains(err.Error(), "parse zot config") {
		t.Errorf("error = %v; want it to name the parse failure", err)
	}
}

// TestEnsureCredsMissingHTPasswdSecretErrors pins that the out-of-band Secret's
// absence is a surfaced misconfiguration, not a silent success that would leave
// the App unable to pull.
func TestEnsureCredsMissingHTPasswdSecretErrors(t *testing.T) {
	c := newTestCreds(t)
	err := c.EnsureCreds(context.Background(), testAppName, testAppNS)
	if err == nil {
		t.Fatal("EnsureCreds must fail when zot-htpasswd is absent")
	}
	if !apierrors.IsNotFound(errors.Unwrap(err)) {
		t.Errorf("error = %v; want a wrapped NotFound", err)
	}
}

// TestEnsurePullSecretAdoptsRaceWinnerPassword covers two reconciles racing to
// create the same pull Secret. The loser must hash the winner's password: the
// pull Secret kubelet reads and the htpasswd entry zot checks have to agree.
func TestEnsurePullSecretAdoptsRaceWinnerPassword(t *testing.T) {
	ctx := context.Background()
	const winnerPassword = "the-winners-password"
	scheme := testScheme(t)
	c := &Creds{
		ZotNamespace: testZotNS,
		HTPasswdName: "zot-htpasswd",
		ConfigName:   "zot-config",
		Registry:     "zot.bex-registry.svc:5000",
	}

	var raced bool
	c.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(htpasswdSecret()).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				sec, ok := obj.(*corev1.Secret)
				if !ok || sec.Name != PullSecretName(testAppName) || raced {
					return cl.Create(ctx, obj, opts...)
				}
				raced = true
				winner := sec.DeepCopy()
				winner.Data = map[string][]byte{
					corev1.DockerConfigJsonKey: c.dockerConfig(ZotUsername(testAppName), winnerPassword),
				}
				if err := cl.Create(ctx, winner); err != nil {
					return err
				}
				return apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, sec.Name)
			},
		}).Build()

	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("EnsureCreds: %v", err)
	}
	if !raced {
		t.Fatal("the create race never fired; the test is not exercising the path")
	}
	hash, found := htpasswdUserHash(readSecret(t, c, testZotNS, "zot-htpasswd").Data[htpasswdKey], ZotUsername(testAppName))
	if !found {
		t.Fatal("htpasswd entry missing")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(winnerPassword)); err != nil {
		t.Errorf("htpasswd must hash the surviving pull Secret's password: %v", err)
	}
}

// TestConflictRequeues proves a persistent write conflict surfaces as
// ErrConflictRequeue — through the caller's error wrapping — so the controller
// requeues instead of failing the App permanently.
func TestConflictRequeues(t *testing.T) {
	ctx := context.Background()
	var patches int
	c := &Creds{
		ZotNamespace: testZotNS,
		HTPasswdName: "zot-htpasswd",
		ConfigName:   "zot-config",
		Registry:     "zot.bex-registry.svc:5000",
	}
	c.Client = fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(htpasswdSecret()).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(_ context.Context, _ client.WithWatch, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
				patches++
				return apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, obj.GetName(), errors.New("stale"))
			},
		}).Build()

	err := c.EnsureCreds(ctx, testAppName, testAppNS)
	if !errors.Is(err, ErrConflictRequeue) {
		t.Fatalf("EnsureCreds error = %v; want ErrConflictRequeue", err)
	}
	if patches != secretWriteAttempts {
		t.Errorf("patch attempts = %d; want the full retry budget of %d", patches, secretWriteAttempts)
	}
}

// TestRotateCredsPasswordChange verifies rotation moves both halves of the
// credential to a new password.
func TestRotateCredsPasswordChange(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())
	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("EnsureCreds: %v", err)
	}
	before := readSecret(t, c, testAppNS, PullSecretName(testAppName))
	oldPassword, err := c.passwordFrom(before.Data[corev1.DockerConfigJsonKey], ZotUsername(testAppName))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.RotateCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("RotateCreds: %v", err)
	}

	after := readSecret(t, c, testAppNS, PullSecretName(testAppName))
	newPassword, err := c.passwordFrom(after.Data[corev1.DockerConfigJsonKey], ZotUsername(testAppName))
	if err != nil {
		t.Fatal(err)
	}
	if newPassword == oldPassword {
		t.Fatal("rotation must mint a new password")
	}

	ht := readSecret(t, c, testZotNS, "zot-htpasswd")
	hash, found := htpasswdUserHash(ht.Data[htpasswdKey], ZotUsername(testAppName))
	if !found {
		t.Fatal("htpasswd entry vanished during rotation")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(newPassword)); err != nil {
		t.Errorf("new password doesn't match new hash: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(oldPassword)); err == nil {
		t.Error("old password incorrectly authenticates after rotation")
	}

	var count int
	for line := range strings.SplitSeq(string(ht.Data[htpasswdKey]), "\n") {
		if strings.HasPrefix(line, ZotUsername(testAppName)+":") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 entry after rotation, got %d", count)
	}
}

// TestEnsureCredsMemoizesHashVerification proves the bcrypt memo makes a
// converged re-ensure free without making it blind. The reconciler is
// level-triggered with no generation guard, so EnsureCreds runs for every App
// every ~30s; at bcrypt.DefaultCost the comparison alone would dominate a
// reconcile worker.
func TestEnsureCredsMemoizesHashVerification(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())
	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("EnsureCreds: %v", err)
	}
	zotUser := ZotUsername(testAppName)
	if _, memoized := c.verified[zotUser]; !memoized {
		t.Fatal("a converged EnsureCreds must leave the verification memoized")
	}

	before := readSecret(t, c, testZotNS, "zot-htpasswd").ResourceVersion
	for range 3 {
		if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
			t.Fatalf("re-ensure: %v", err)
		}
	}
	if after := readSecret(t, c, testZotNS, "zot-htpasswd").ResourceVersion; after != before {
		t.Errorf("converged re-ensure rewrote the htpasswd Secret (%s -> %s)", before, after)
	}
}

// TestEnsureCredsMemoDoesNotMaskTamperedHash is the memo's safety property: a
// hash swapped for one that verifies a DIFFERENT password must still be
// repaired. The memo is keyed by the exact (hash, password) pair, so a tampered
// hash cannot hit it — were it keyed by username alone, this App would keep a
// credential the pull Secret no longer matches until the process restarted.
func TestEnsureCredsMemoDoesNotMaskTamperedHash(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())
	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("EnsureCreds: %v", err)
	}
	zotUser := ZotUsername(testAppName)
	password, err := c.passwordFrom(
		readSecret(t, c, testAppNS, PullSecretName(testAppName)).Data[corev1.DockerConfigJsonKey], zotUser)
	if err != nil {
		t.Fatalf("passwordFrom: %v", err)
	}

	// Overwrite the entry with a well-formed hash of an attacker-chosen password.
	tampered, err := setHTPasswdLine(
		readSecret(t, c, testZotNS, "zot-htpasswd").Data[htpasswdKey], zotUser, "not-the-real-password")
	if err != nil {
		t.Fatalf("setHTPasswdLine: %v", err)
	}
	ht := readSecret(t, c, testZotNS, "zot-htpasswd")
	ht.Data[htpasswdKey] = tampered
	if err := c.Client.Update(ctx, ht); err != nil {
		t.Fatal(err)
	}

	if err := c.EnsureCreds(ctx, testAppName, testAppNS); err != nil {
		t.Fatalf("repair EnsureCreds: %v", err)
	}
	hash, found := htpasswdUserHash(readSecret(t, c, testZotNS, "zot-htpasswd").Data[htpasswdKey], zotUser)
	if !found {
		t.Fatal("htpasswd entry vanished")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Errorf("tampered hash survived the memo — it must be re-synced to the pull Secret's password: %v", err)
	}
}

func TestEnsureCredsForDisjointWorkspaces(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())
	c.DualReadEnabled = true // supervised migration window (round-21 finding 4)
	a := identity.ForApp("web", "tea-aaaaaaaaaaaaaaaaaaaa")
	b := identity.ForApp("web", "tea-bbbbbbbbbbbbbbbbbbbb")
	if err := c.EnsureCredsFor(ctx, a, "ns-a"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureCredsFor(ctx, b, "ns-b"); err != nil {
		t.Fatal(err)
	}
	if a.PullSecretName() == b.PullSecretName() || a.ZotUsername() == b.ZotUsername() || a.Repo() == b.Repo() {
		t.Fatal("same App name in two workspaces collided")
	}
	_ = readSecret(t, c, "ns-a", a.PullSecretName())
	_ = readSecret(t, c, "ns-b", b.PullSecretName())
	ht := readSecret(t, c, testZotNS, "zot-htpasswd")
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], a.ZotUsername()); !ok {
		t.Fatal("workspace A user missing")
	}
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], b.ZotUsername()); !ok {
		t.Fatal("workspace B user missing")
	}
	data := storedConfig(t, c)
	if !zotRepoGrants(data, a.Repo(), a.ZotUsername(), zotReadWriteActions) {
		t.Fatal("workspace A repo ACL missing")
	}
	if !zotRepoGrants(data, b.Repo(), b.ZotUsername(), zotReadWriteActions) {
		t.Fatal("workspace B repo ACL missing")
	}
	if !zotRepoUserGrants(data, a.LegacyRepo(), a.ZotUsername(), zotReadOnlyActions) {
		t.Fatal("dual-read grant for A missing on legacy repo")
	}
	if !zotRepoUserGrants(data, b.LegacyRepo(), b.ZotUsername(), zotReadOnlyActions) {
		t.Fatal("dual-read grant for B missing on legacy repo")
	}
}

func TestEnsureCredsForDualReadDisabledByDefault(t *testing.T) {
	// round-21 finding 4: with dual-read off (the default), a scoped App must NOT
	// gain a read grant on the bare-name legacy repo. Otherwise workspace B could
	// name a service after workspace A's pre-migration repo and read A's image,
	// since the legacy key drops the workspace and grantZotRepoUser checks no
	// ownership. Its exclusive RW on its OWN workspace-scoped repo is unaffected.
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())
	scoped := identity.ForApp("web", "tea-bbbbbbbbbbbbbbbbbbbb")
	if err := c.EnsureCredsFor(ctx, scoped, "ns-b"); err != nil {
		t.Fatal(err)
	}
	data := storedConfig(t, c)
	if zotRepoHasUser(data, scoped.LegacyRepo(), scoped.ZotUsername()) {
		t.Fatal("dual-read grant on the legacy repo was issued with dual-read disabled")
	}
	if !zotRepoGrants(data, scoped.Repo(), scoped.ZotUsername(), zotReadWriteActions) {
		t.Fatal("scoped exclusive RW on its own repo missing")
	}
}

func TestEnsureCredsForPreservesUnlabeledSiblingACL(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())
	c.DualReadEnabled = true // supervised migration window (round-21 finding 4)
	if err := c.EnsureCreds(ctx, "web", "legacy-ns"); err != nil {
		t.Fatal(err)
	}
	scoped := identity.ForApp("web", "tea-aaaaaaaaaaaaaaaaaaaa")
	if err := c.EnsureCredsFor(ctx, scoped, "ns-a"); err != nil {
		t.Fatal(err)
	}
	data := storedConfig(t, c)
	if !zotRepoUserGrants(data, "web", ZotUsername("web"), zotReadWriteActions) {
		t.Fatal("unlabeled sibling exclusive RW was replaced")
	}
	if !zotRepoUserGrants(data, "web", scoped.ZotUsername(), zotReadOnlyActions) {
		t.Fatal("dual-read READ grant missing")
	}
	if !zotRepoGrants(data, scoped.Repo(), scoped.ZotUsername(), zotReadWriteActions) {
		t.Fatal("scoped exclusive RW missing")
	}
}

func TestRevokeCredsForScopedLeavesLegacySibling(t *testing.T) {
	ctx := context.Background()
	c := newTestCreds(t, htpasswdSecret())
	c.DualReadEnabled = true // grant the dual-read so revoke has something to drop
	if err := c.EnsureCreds(ctx, "web", "legacy-ns"); err != nil {
		t.Fatal(err)
	}
	scoped := identity.ForApp("web", "tea-aaaaaaaaaaaaaaaaaaaa")
	if err := c.EnsureCredsFor(ctx, scoped, "ns-a"); err != nil {
		t.Fatal(err)
	}
	if err := c.RevokeCredsFor(ctx, scoped); err != nil {
		t.Fatal(err)
	}
	ht := readSecret(t, c, testZotNS, "zot-htpasswd")
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], scoped.ZotUsername()); ok {
		t.Fatal("scoped user should be revoked")
	}
	if _, ok := htpasswdUserHash(ht.Data[htpasswdKey], ZotUsername("web")); !ok {
		t.Fatal("unlabeled sibling user was revoked")
	}
	data := storedConfig(t, c)
	if zotHasRepo(data, scoped.Repo()) {
		t.Fatal("scoped repo ACL should be gone")
	}
	if !zotRepoGrants(data, "web", ZotUsername("web"), zotReadWriteActions) {
		t.Fatal("unlabeled sibling ACL was removed")
	}
	if zotRepoHasUser(data, "web", scoped.ZotUsername()) {
		t.Fatal("dual-read grant should be dropped on scoped revoke")
	}
}

func TestRetentionPolicyGlobsNestedRepos(t *testing.T) {
	data := mustDecode(t, (&Creds{}).baseZotConfig())
	storage, _ := data["storage"].(map[string]any)
	retention, _ := storage["retention"].(map[string]any)
	policies, _ := retention["policies"].([]any)
	policy, _ := policies[0].(map[string]any)
	repos, _ := policy["repositories"].([]any)
	if len(repos) != 1 || repos[0] != "**" {
		t.Fatalf("retention repositories = %v; want [**] (Zot doublestar matches nested tea-x/hello)", repos)
	}
}
