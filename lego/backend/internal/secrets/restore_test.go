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
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// restore_test.go pins RestoreEnvironment (w1/m152 t009): a rollback makes the
// SAVED environment equal its target's snapshot, without opening a rollout of its
// own.

func envMap(t *testing.T, s *Service) map[string]string {
	t.Helper()
	vars, err := s.ListEnvVars(context.Background(), "web")
	if err != nil {
		t.Fatalf("list env: %v", err)
	}
	out := map[string]string{}
	for _, v := range vars {
		out[v.Key] = v.Value
	}
	return out
}

func TestRestoreEnvironmentMakesSavedStateEqualTheTarget(t *testing.T) {
	ctx := context.Background()
	s := newService(newFakeSecretStore(), tenantApp("web", "tea-a"))
	// An advancing clock: the harness default is frozen, under which a bumped
	// restartedAt is indistinguishable from an untouched one.
	now := fixedNow()
	s.Clock = func() time.Time { now = now.Add(time.Second); return now }
	// D2's saved state: MESSAGE=v2 added, KEEP changed.
	if _, err := s.SetEnvVars(ctx, "web", []EnvVarView{{Key: "MESSAGE", Value: "v2"}, {Key: "KEEP", Value: "new"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSecretFile(ctx, "web", "extra.pem", "later"); err != nil {
		t.Fatal(err)
	}
	restartedBefore := getApp(t, s.Client, "web").Spec.RestartedAt

	// Roll back to D1, which had no MESSAGE, KEEP=old, and a different file set.
	changed, err := s.RestoreEnvironment(ctx, "web",
		map[string]string{"KEEP": "old"},
		map[string]string{"ca.pem": "CERT"})
	if err != nil || !changed {
		t.Fatalf("RestoreEnvironment = (%v, %v), want a change", changed, err)
	}

	if got := envMap(t, s); len(got) != 1 || got["KEEP"] != "old" {
		t.Errorf("saved env = %v, want exactly the target's {KEEP: old}", got)
	}
	files, err := s.ListSecretFiles(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "ca.pem" {
		t.Errorf("saved files = %v, want exactly the target's [ca.pem]", files)
	}
	// save_only: the rollback's own dispatch rolls; restoring must not request a
	// second rollout by bumping restartedAt.
	if after := getApp(t, s.Client, "web").Spec.RestartedAt; after != restartedBefore {
		t.Errorf("restore bumped restartedAt %q -> %q; the rollback must roll exactly once", restartedBefore, after)
	}
}

func TestRestoreEnvironmentIsANoOpWhenAlreadyEqual(t *testing.T) {
	ctx := context.Background()
	s := newService(newFakeSecretStore(), tenantApp("web", "tea-a"))
	if _, err := s.SetEnvVars(ctx, "web", []EnvVarView{{Key: "KEEP", Value: "v"}}); err != nil {
		t.Fatal(err)
	}
	changed, err := s.RestoreEnvironment(ctx, "web", map[string]string{"KEEP": "v"}, nil)
	if err != nil || changed {
		t.Fatalf("RestoreEnvironment = (%v, %v), want no change", changed, err)
	}
}

// A manifest owns its keys' values; a rollback must not delete or rewrite them.
func TestRestoreEnvironmentLeavesManifestKeysAlone(t *testing.T) {
	ctx := context.Background()
	app := tenantApp("web", "tea-a")
	app.Spec.Env = []appv1alpha1.EnvVar{{Name: "FROM_MANIFEST", Value: "m"}}
	s := newService(newFakeSecretStore(), app)
	if _, err := s.RestoreEnvironment(ctx, "web", map[string]string{"FROM_MANIFEST": "rolled-back", "OTHER": "x"}, nil); err != nil {
		t.Fatalf("RestoreEnvironment refused over a manifest key it should have skipped: %v", err)
	}
	got := envMap(t, s)
	if got["FROM_MANIFEST"] != "m" {
		t.Errorf("manifest key = %q, want the manifest's value m", got["FROM_MANIFEST"])
	}
	if got["OTHER"] != "x" {
		t.Errorf("store key OTHER = %q, want x", got["OTHER"])
	}
}
