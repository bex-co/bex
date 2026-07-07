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
	find := func(env []corev1.EnvVar, name string) (string, bool) {
		val, ok := "", false
		for _, e := range env {
			if e.Name == name {
				val, ok = e.Value, true // keep scanning: last wins, mirroring kubelet
			}
		}
		return val, ok
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
		if v, _ := find(env, "BAR"); v != "" {
			t.Fatalf("empty value should be preserved, got %q", v)
		}
	})

	t.Run("PORT cannot be shadowed", func(t *testing.T) {
		env := appEnv(mk(appv1alpha1.EnvVar{Name: "PORT", Value: "1"}), 3000)
		if v, _ := find(env, "PORT"); v != "3000" {
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
}

func TestEnvFromSources(t *testing.T) {
	if envFromSources(&appv1alpha1.App{}) != nil {
		t.Fatal("no envFromSecret => nil (unchanged behavior)")
	}
	got := envFromSources(&appv1alpha1.App{Spec: appv1alpha1.AppSpec{EnvFromSecret: "web-env"}})
	if len(got) != 1 || got[0].SecretRef == nil || got[0].SecretRef.Name != "web-env" {
		t.Fatalf("want one SecretRef envFrom to web-env, got %v", got)
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
