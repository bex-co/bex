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

package apps

// blueprint_e2e_test.go is the w1/m35 DoD proof: a five-field bex.yml applied
// through the REAL env-groups + env-vars feature services (wired as the two
// blueprint seams over one shared store) materializes end-to-end, a re-sync is
// idempotent (no re-mint, no roll), and sync:false honors a live edit.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/envgroups/envgroupstest"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// memKV is a minimal in-memory core.SecretKV shared by the env-groups + env-vars
// services in the end-to-end test (the OpenBao seam both features write through).
type memKV struct {
	mu       sync.Mutex
	m        map[string]map[string]string
	versions map[string]uint64
}

func newMemKV() *memKV {
	return &memKV{m: map[string]map[string]string{}, versions: map[string]uint64{}}
}

func (k *memKV) Get(_ context.Context, path string) (map[string]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.get(path), nil
}

func (k *memKV) get(path string) map[string]string {
	out := map[string]string{}
	for kk, v := range k.m[path] {
		out[kk] = v
	}
	return out
}

func (k *memKV) Put(_ context.Context, path string, data map[string]string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.put(path, data)
	return nil
}

func (k *memKV) put(path string, data map[string]string) {
	cp := map[string]string{}
	for kk, v := range data {
		cp[kk] = v
	}
	k.m[path] = cp
	k.versions[path]++
}

func (k *memKV) Delete(_ context.Context, path string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, path)
	delete(k.versions, path)
	return nil
}

func (k *memKV) List(_ context.Context, path string) ([]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	prefix := path + "/"
	seen := map[string]bool{}
	var out []string
	for kk := range k.m {
		if len(kk) <= len(prefix) || kk[:len(prefix)] != prefix {
			continue
		}
		rest := kk[len(prefix):]
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' {
				rest = rest[:i]
				break
			}
		}
		if !seen[rest] {
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out, nil
}

func (k *memKV) GetVersioned(_ context.Context, path string) (core.SecretKVSnapshot, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return core.SecretKVSnapshot{Data: k.get(path), Version: k.versions[path]}, nil
}

func (k *memKV) PutCAS(_ context.Context, path string, data map[string]string, expectedVersion uint64) (uint64, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.versions[path] != expectedVersion {
		return 0, core.ErrConflict
	}
	k.put(path, data)
	return k.versions[path], nil
}

func TestBlueprintFiveFieldEndToEnd(t *testing.T) {
	cl := fakeClient()
	base := &core.Base{Client: cl, Namespace: "default", Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() },
		Workspace: coretest.Workspaces{core.DefaultTenant}}
	kv := newMemKV()
	eg := &envgroups.Service{Base: base, Store: kv, Groups: envgroupstest.New()}
	sec := &secrets.Service{Base: base, Store: kv}
	svc := &Service{Base: base, EnvGroups: eg, EnvSeeder: sec}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "owner", Method: "session"})

	// --- apply ---
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: fiveFieldManifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}

	// envVarGroups materialized a group named "shared" with its literal + generate.
	groups, err := eg.ListEnvGroups(ctx, "")
	if err != nil {
		t.Fatalf("ListEnvGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "shared" {
		t.Fatalf("env groups = %+v, want one named shared", groups)
	}
	gid := groups[0].ID
	groupSecret := kv.m["env-groups/"+gid+"/env"]["GROUP_SECRET"]
	if len(groupSecret) != 44 {
		t.Errorf("group GROUP_SECRET = %q, want a 44-char base64 value", groupSecret)
	}
	if kv.m["env-groups/"+gid+"/env"]["LOG_LEVEL"] != "info" {
		t.Errorf("group LOG_LEVEL not materialized")
	}

	// fromGroup linked the group onto web (its env secret is on the spec).
	web := getApp(t, cl, core.CRName(core.DefaultTenant, "web"))
	linked := false
	for _, s := range web.Spec.EnvFromSecrets {
		if s == gid+"-env" {
			linked = true
		}
	}
	if !linked {
		t.Errorf("web not linked to the group: %v", web.Spec.EnvFromSecrets)
	}

	// generateValue + sync:false seeded web's mutable env store; envVarKey copied a
	// sibling literal into spec.Env; a plain literal is on spec.Env too.
	sessionSecret, err := sec.GetEnvVar(ctx, "web", "SESSION_SECRET")
	if err != nil || len(sessionSecret.Value) != 44 {
		t.Fatalf("SESSION_SECRET = %+v err=%v, want a 44-char base64 value", sessionSecret, err)
	}
	if seeded, _ := sec.GetEnvVar(ctx, "web", "SEEDED"); seeded.Value != "once" {
		t.Errorf("SEEDED = %q, want once", seeded.Value)
	}
	if dp := findEnv(t, getApp(t, cl, core.CRName(core.DefaultTenant, "web")).Spec.Env, "DB_PASS"); dp.Value != "supersecret" {
		t.Errorf("fromService.envVarKey copy DB_PASS = %q, want supersecret", dp.Value)
	}

	// --- re-sync: idempotent (no re-mint, no roll) ---
	firstRA := getApp(t, cl, core.CRName(core.DefaultTenant, "web")).Spec.RestartedAt
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: fiveFieldManifest}); err != nil {
		t.Fatalf("re-DeployStack: %v", err)
	}
	if got := kv.m["env-groups/"+gid+"/env"]["GROUP_SECRET"]; got != groupSecret {
		t.Errorf("group generated value re-minted on sync: %q -> %q", groupSecret, got)
	}
	if got, _ := sec.GetEnvVar(ctx, "web", "SESSION_SECRET"); got.Value != sessionSecret.Value {
		t.Errorf("service generated value re-minted on sync: %q -> %q", sessionSecret.Value, got.Value)
	}
	if got := getApp(t, cl, core.CRName(core.DefaultTenant, "web")).Spec.RestartedAt; got != firstRA {
		t.Errorf("idempotent re-sync rolled web: RestartedAt %q -> %q", firstRA, got)
	}

	// --- sync:false honors a live edit ---
	if _, err := sec.SetEnvVar(ctx, "web", "SEEDED", secrets.EnvVarWrite{Value: "edited-in-dashboard"}); err != nil {
		t.Fatalf("SetEnvVar: %v", err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: fiveFieldManifest}); err != nil {
		t.Fatalf("re-DeployStack after edit: %v", err)
	}
	if got, _ := sec.GetEnvVar(ctx, "web", "SEEDED"); got.Value != "edited-in-dashboard" {
		t.Errorf("sync:false overwrote a live edit: SEEDED = %q, want edited-in-dashboard", got.Value)
	}
}

// countingSeeder records how often the seed-once path runs.
type countingSeeder struct {
	EnvSeeder
	calls int
}

func (c *countingSeeder) SeedEnvVars(ctx context.Context, service string, literals map[string]string, generates []string) error {
	c.calls++
	return c.EnvSeeder.SeedEnvVars(ctx, service, literals, generates)
}

// w8/296: a service a Blueprint creates gets its generateValue and sync:false
// values, and its copies of a created sibling's, with its first write, as a
// direct create does, so no seed or copy opens a second deploy that
// supersedes the first. A service that already exists is still seeded
// seed-once, and a re-apply re-mints nothing.
func TestBlueprintCreateSeedsWithTheFirstWrite(t *testing.T) {
	cl := fakeClient()
	base := &core.Base{Client: cl, Namespace: "default", Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() },
		Workspace: coretest.Workspaces{core.DefaultTenant}}
	kv := newMemKV()
	sec := &secrets.Service{Base: base, Store: kv}
	seeds := &countingSeeder{EnvSeeder: sec}
	svc := &Service{Base: base, EnvSeeder: seeds, EnvCopier: secrets.NewEnvCopier(sec), CreateSecrets: secrets.NewCreateSecretsSeeder(sec)}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "owner", Method: "session"})
	// The consumer comes first: the copy does not depend on file order.
	const manifest = `
services:
  - name: worker
    type: worker
    runtime: image
    image: {url: worker:1}
    envVars:
      - {key: WEB_TOKEN, fromService: {name: web, type: web, envVarKey: TOKEN}}
  - name: web
    type: web
    runtime: image
    image: {url: web:1}
    envVars:
      - {key: PLAIN, value: hello}
      - {key: TOKEN, generateValue: true}
      - {key: SEEDED, value: once, sync: false}
`
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if seeds.calls != 0 {
		t.Errorf("a created service was seeded after its first write (%d calls); its seed would supersede the first deploy", seeds.calls)
	}
	token, err := sec.GetEnvVar(ctx, "web", "TOKEN")
	if err != nil || len(token.Value) != 44 {
		t.Fatalf("TOKEN = %+v err=%v, want a 44-char generated value", token, err)
	}
	if seeded, _ := sec.GetEnvVar(ctx, "web", "SEEDED"); seeded.Value != "once" {
		t.Errorf("SEEDED = %q, want once", seeded.Value)
	}
	if copied, _ := sec.GetEnvVar(ctx, "worker", "WEB_TOKEN"); copied.Value != token.Value {
		t.Errorf("worker WEB_TOKEN = %q, want web's TOKEN", copied.Value)
	}
	if plain := findEnv(t, getApp(t, cl, core.CRName(core.DefaultTenant, "web")).Spec.Env, "PLAIN"); plain.Value != "hello" {
		t.Errorf("PLAIN = %q, want the manifest literal on spec.Env", plain.Value)
	}
	for _, name := range []string{"web", "worker"} {
		app := getApp(t, cl, core.CRName(core.DefaultTenant, name))
		if app.Spec.RestartedAt != "" || app.Spec.EnvFromSecret != app.Name+"-env" {
			t.Errorf("%s: RestartedAt %q, EnvFromSecret %q; want its first deploy alone, with its env store attached", name, app.Spec.RestartedAt, app.Spec.EnvFromSecret)
		}
	}

	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("re-DeployStack: %v", err)
	}
	if seeds.calls != 1 {
		t.Errorf("seed calls after a re-apply = %d, want the existing service seeded once", seeds.calls)
	}
	if got, _ := sec.GetEnvVar(ctx, "web", "TOKEN"); got.Value != token.Value {
		t.Errorf("TOKEN re-minted on re-apply: %q -> %q", token.Value, got.Value)
	}
	for _, name := range []string{"web", "worker"} {
		if got := getApp(t, cl, core.CRName(core.DefaultTenant, name)).Spec.RestartedAt; got != "" {
			t.Errorf("an idempotent re-apply rolled %s: RestartedAt %q", name, got)
		}
	}

	// A consumer added later copies its existing sibling's value with its
	// first write too.
	const withAPI = manifest + `  - name: api
    type: worker
    runtime: image
    image: {url: api:1}
    envVars:
      - {key: WEB_TOKEN, fromService: {name: web, type: web, envVarKey: TOKEN}}
`
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: withAPI}); err != nil {
		t.Fatalf("DeployStack with api: %v", err)
	}
	if copied, _ := sec.GetEnvVar(ctx, "api", "WEB_TOKEN"); copied.Value != token.Value {
		t.Errorf("api WEB_TOKEN = %q, want web's TOKEN", copied.Value)
	}
	if got := getApp(t, cl, core.CRName(core.DefaultTenant, "api")).Spec.RestartedAt; got != "" {
		t.Errorf("api's copy rolled it after its first write: RestartedAt %q", got)
	}
}
