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

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestAppEnv(t *testing.T) {
	mk := func(env ...appv1alpha1.EnvVar) *appv1alpha1.App {
		return &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Env: env}}
	}
	// last() is what k8s uses: within a container's env, a duplicate name is
	// resolved to the last entry — so PORT being appended last is what makes it win.
	find := func(env []corev1.EnvVar, name string) string {
		val := ""
		for _, e := range env {
			if e.Name == name {
				val = e.Value // keep scanning: last wins, mirroring kubelet
			}
		}
		return val
	}

	t.Run("empty env still injects PORT", func(t *testing.T) {
		env := appEnv(mk(), 8080)
		if len(env) != 1 || env[0].Name != "PORT" || env[0].Value != "8080" {
			t.Fatalf("want single PORT=8080, got %v", env)
		}
	})

	t.Run("user vars precede PORT, in order", func(t *testing.T) {
		env := appEnv(mk(
			appv1alpha1.EnvVar{Name: "FOO", Value: "1"},
			appv1alpha1.EnvVar{Name: "BAR", Value: ""},
		), 3000)
		if got := []string{env[0].Name, env[1].Name, env[2].Name}; got[0] != "FOO" || got[1] != "BAR" || got[2] != "PORT" {
			t.Fatalf("order = %v, want [FOO BAR PORT]", got)
		}
		if v := find(env, "BAR"); v != "" {
			t.Fatalf("empty value should be preserved, got %q", v)
		}
	})

	t.Run("PORT cannot be shadowed", func(t *testing.T) {
		env := appEnv(mk(appv1alpha1.EnvVar{Name: "PORT", Value: "1"}), 3000)
		if v := find(env, "PORT"); v != "3000" {
			t.Fatalf("PORT = %q, want 3000 (user shadow dropped)", v)
		}
		// The dropped entry must not linger earlier in the slice (kubelet's
		// last-wins would otherwise resolve it to the user value on a reorder).
		for _, e := range env[:len(env)-1] {
			if e.Name == "PORT" {
				t.Fatalf("a user PORT entry survived: %v", env)
			}
		}
	})

	t.Run("ValueFrom secretRef is materialized (fromDatabase, w1/m24)", func(t *testing.T) {
		// A bex.yml fromDatabase reference resolves to an EnvVar.ValueFrom.SecretKeyRef;
		// the operator projects it onto a corev1 SecretKeySelector (non-optional, so a
		// service waits on its Database's CNPG connection Secret) — no plaintext value.
		env := appEnv(mk(appv1alpha1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{
				Name: "db-app", Key: "uri",
			}},
		}), 3000)
		var du *corev1.EnvVar
		for i := range env {
			if env[i].Name == "DATABASE_URL" {
				du = &env[i]
			}
		}
		if du == nil || du.Value != "" || du.ValueFrom == nil || du.ValueFrom.SecretKeyRef == nil {
			t.Fatalf("DATABASE_URL must be a secretKeyRef with no plaintext value, got %+v", du)
		}
		ref := du.ValueFrom.SecretKeyRef
		if ref.Name != "db-app" || ref.Key != "uri" {
			t.Errorf("secretKeyRef = %+v, want {db-app, uri}", ref)
		}
		if ref.Optional != nil {
			t.Errorf("fromDatabase secretRef must be non-optional so the service waits on the DB, got optional=%v", *ref.Optional)
		}
	})
}

func TestEnvFromSources(t *testing.T) {
	if envFromSources(&appv1alpha1.App{}) != nil {
		t.Fatal("no envFromSecret => nil (unchanged behavior)")
	}
	got := envFromSources(&appv1alpha1.App{Spec: appv1alpha1.AppSpec{EnvFromSecret: "web-env"}})
	if len(got) != 1 || got[0].SecretRef == nil || got[0].SecretRef.Name != "web-env" {
		t.Fatalf("want one SecretRef envFrom to web-env, got %v", got)
	}

	// Env groups (spec.envFromSecrets) come BEFORE the service's own set, so the
	// service's own env var wins on a collision (kubelet applies envFrom in order,
	// last wins). Group refs are optional; the service's own is not.
	got = envFromSources(&appv1alpha1.App{Spec: appv1alpha1.AppSpec{
		EnvFromSecret:  "web-env",
		EnvFromSecrets: []string{"evg-1-env", "evg-2-env"},
	}})
	if len(got) != 3 {
		t.Fatalf("want 3 envFrom sources, got %d: %v", len(got), got)
	}
	if got[0].SecretRef.Name != "evg-1-env" || got[1].SecretRef.Name != "evg-2-env" || got[2].SecretRef.Name != "web-env" {
		t.Fatalf("order = %v, want [evg-1-env evg-2-env web-env]", got)
	}
	if got[0].SecretRef.Optional == nil || !*got[0].SecretRef.Optional {
		t.Error("group env sources should be optional")
	}
	if got[2].SecretRef.Optional != nil {
		t.Error("the service's own env source should not be optional")
	}
}

func TestSecretFileMounts(t *testing.T) {
	if vol, mount := secretFileMounts(&appv1alpha1.App{}); vol != nil || mount != nil {
		t.Fatal("no filesFromSecrets => no volume/mount (unchanged behavior)")
	}
	vol, mount := secretFileMounts(&appv1alpha1.App{Spec: appv1alpha1.AppSpec{
		FilesFromSecrets: []string{"web-files", "evg-1-files"},
	}})
	if vol == nil || mount == nil {
		t.Fatal("filesFromSecrets should yield a volume + mount")
	}
	if mount.MountPath != "/etc/secrets" || !mount.ReadOnly {
		t.Fatalf("mount should be read-only at /etc/secrets, got %+v", mount)
	}
	if vol.Projected == nil || len(vol.Projected.Sources) != 2 {
		t.Fatalf("want a projected volume with 2 secret sources, got %+v", vol.VolumeSource)
	}
	for i, src := range vol.Projected.Sources {
		if src.Secret == nil || src.Secret.Optional == nil || !*src.Secret.Optional {
			t.Fatalf("source %d should be an optional secret projection: %+v", i, src)
		}
	}
	if vol.Projected.Sources[0].Secret.Name != "web-files" || vol.Projected.Sources[1].Secret.Name != "evg-1-files" {
		t.Fatalf("projected sources = %v, want [web-files evg-1-files]", vol.Projected.Sources)
	}
}

func TestEffectiveReplicas(t *testing.T) {
	mk := func(replicas int32, suspended bool) *appv1alpha1.App {
		return &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Replicas: replicas, Suspended: suspended}}
	}
	cases := []struct {
		name string
		app  *appv1alpha1.App
		want int32
	}{
		{"default is 1", mk(0, false), 1},
		{"explicit count", mk(3, false), 3},
		{"suspended overrides default", mk(0, true), 0},
		// The ADR invariant: suspend derives 0 without rewriting spec.replicas.
		{"suspended overrides explicit count", mk(3, true), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectiveReplicas(tc.app); got != tc.want {
				t.Fatalf("effectiveReplicas = %d, want %d", got, tc.want)
			}
			// suspend must never mutate the stored count
			if tc.app.Spec.Suspended && tc.app.Spec.Replicas == 0 && tc.name == "suspended overrides explicit count" {
				t.Fatalf("spec.replicas was mutated")
			}
		})
	}
}
