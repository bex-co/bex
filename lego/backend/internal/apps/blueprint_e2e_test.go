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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// memKV is a minimal in-memory core.SecretKV shared by the env-groups + env-vars
// services in the end-to-end test (the OpenBao seam both features write through).
type memKV struct{ m map[string]map[string]string }

func newMemKV() *memKV { return &memKV{m: map[string]map[string]string{}} }

func (k *memKV) Get(_ context.Context, path string) (map[string]string, error) {
	out := map[string]string{}
	for kk, v := range k.m[path] {
		out[kk] = v
	}
	return out, nil
}

func (k *memKV) Put(_ context.Context, path string, data map[string]string) error {
	cp := map[string]string{}
	for kk, v := range data {
		cp[kk] = v
	}
	k.m[path] = cp
	return nil
}

func (k *memKV) Delete(_ context.Context, path string) error { delete(k.m, path); return nil }

func (k *memKV) List(_ context.Context, path string) ([]string, error) {
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

func TestBlueprintFiveFieldEndToEnd(t *testing.T) {
	cl := fakeClient()
	base := &core.Base{Client: cl, Namespace: "default", Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}
	kv := newMemKV()
	eg := &envgroups.Service{Base: base, Store: kv}
	sec := &secrets.Service{Base: base, Store: kv}
	svc := &Service{Base: base, EnvGroups: eg, EnvSeeder: sec}
	ctx := context.Background()

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
	web := getApp(t, cl, "web")
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
	if dp := findEnv(t, getApp(t, cl, "web").Spec.Env, "DB_PASS"); dp.Value != "supersecret" {
		t.Errorf("fromService.envVarKey copy DB_PASS = %q, want supersecret", dp.Value)
	}

	// --- re-sync: idempotent (no re-mint, no roll) ---
	firstRA := getApp(t, cl, "web").Spec.RestartedAt
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: fiveFieldManifest}); err != nil {
		t.Fatalf("re-DeployStack: %v", err)
	}
	if got := kv.m["env-groups/"+gid+"/env"]["GROUP_SECRET"]; got != groupSecret {
		t.Errorf("group generated value re-minted on sync: %q -> %q", groupSecret, got)
	}
	if got, _ := sec.GetEnvVar(ctx, "web", "SESSION_SECRET"); got.Value != sessionSecret.Value {
		t.Errorf("service generated value re-minted on sync: %q -> %q", sessionSecret.Value, got.Value)
	}
	if got := getApp(t, cl, "web").Spec.RestartedAt; got != firstRA {
		t.Errorf("idempotent re-sync rolled web: RestartedAt %q -> %q", firstRA, got)
	}

	// --- sync:false honors a live edit ---
	if _, err := sec.SetEnvVar(ctx, "web", "SEEDED", "edited-in-dashboard"); err != nil {
		t.Fatalf("SetEnvVar: %v", err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: fiveFieldManifest}); err != nil {
		t.Fatalf("re-DeployStack after edit: %v", err)
	}
	if got, _ := sec.GetEnvVar(ctx, "web", "SEEDED"); got.Value != "edited-in-dashboard" {
		t.Errorf("sync:false overwrote a live edit: SEEDED = %q, want edited-in-dashboard", got.Value)
	}
}
