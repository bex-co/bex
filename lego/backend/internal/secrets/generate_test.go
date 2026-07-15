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

// generate_test.go covers generateValue on the env-var write verb (w8/m10/t001)
// and the blueprint seed-once seam (w1/m35): SeedEnvVars.

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestSetEnvVarsGenerateValue(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	got, err := svc.SetEnvVars(ctx, "web", []EnvVarView{{Key: "SESSION_SECRET", Generate: true}})
	if err != nil {
		t.Fatalf("SetEnvVars generate: %v", err)
	}
	// A minted value comes back in the response (the caller must see what was
	// generated) and matches Render's shape: base64 256-bit (44 chars).
	if len(got) != 1 || len(got[0].Value) != 44 {
		t.Fatalf("generated value = %+v, want one 44-char base64 value", got)
	}
	if store.m[envPath("web")]["SESSION_SECRET"] != got[0].Value {
		t.Errorf("stored value != returned value")
	}
}

func TestSetEnvVarGenerateValueIsFresh(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	first, err := svc.SetEnvVar(ctx, "web", "TOKEN", EnvVarWrite{GenerateValue: true})
	if err != nil {
		t.Fatalf("first generated SetEnvVar: %v", err)
	}
	second, err := svc.SetEnvVar(ctx, "web", "TOKEN", EnvVarWrite{GenerateValue: true})
	if err != nil {
		t.Fatalf("second generated SetEnvVar: %v", err)
	}
	if len(first.Value) != 44 || len(second.Value) != 44 || first.Value == second.Value {
		t.Fatalf("generated values = %q, %q; want distinct 44-char values", first.Value, second.Value)
	}
}

func TestSetEnvVarValuePlusGenerateRejected(t *testing.T) {
	svc := newService(newFakeSecretStore(), sampleApp("web"))
	_, err := svc.SetEnvVar(context.Background(), "web", "TOKEN", EnvVarWrite{Value: "literal", GenerateValue: true})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("single value + generateValue => ErrBadRequest, got %v", err)
	}
}

func TestSetEnvVarsValuePlusGenerateRejected(t *testing.T) {
	svc := newService(newFakeSecretStore(), sampleApp("web"))
	_, err := svc.SetEnvVars(context.Background(), "web", []EnvVarView{{Key: "X", Value: "v", Generate: true}})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("value + generateValue => ErrBadRequest, got %v", err)
	}
}

func TestSeedEnvVarsSeedsOnce(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	// First seed: a sync:false literal + a generateValue key both land.
	if err := svc.SeedEnvVars(ctx, "web", map[string]string{"API_KEY": "seed-1"}, []string{"TOKEN"}); err != nil {
		t.Fatalf("SeedEnvVars: %v", err)
	}
	env := store.m[envPath("web")]
	if env["API_KEY"] != "seed-1" {
		t.Errorf("literal not seeded: %q", env["API_KEY"])
	}
	minted := env["TOKEN"]
	if len(minted) != 44 {
		t.Errorf("generated value = %q, want 44-char base64", minted)
	}

	// Simulate a dashboard edit through the normal env-vars API.
	if _, err := svc.SetEnvVar(ctx, "web", "API_KEY", EnvVarWrite{Value: "edited-live"}); err != nil {
		t.Fatalf("SetEnvVar: %v", err)
	}

	// Re-seed (a later blueprint sync): seed-once means the live edit survives and
	// the generated value is NOT re-minted.
	if err := svc.SeedEnvVars(ctx, "web", map[string]string{"API_KEY": "seed-2"}, []string{"TOKEN"}); err != nil {
		t.Fatalf("SeedEnvVars re-seed: %v", err)
	}
	env = store.m[envPath("web")]
	if env["API_KEY"] != "edited-live" {
		t.Errorf("re-seed overwrote a live edit: %q, want edited-live", env["API_KEY"])
	}
	if env["TOKEN"] != minted {
		t.Errorf("re-seed re-minted the generated value: %q -> %q", minted, env["TOKEN"])
	}
}

func TestSeedEnvVarsAllPresentIsNoOp(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()
	if err := svc.SeedEnvVars(ctx, "web", map[string]string{"K": "v"}, nil); err != nil {
		t.Fatalf("SeedEnvVars: %v", err)
	}
	firstRA := getApp(t, svc.Client, "web").Spec.RestartedAt
	// Re-seeding the same, already-present key must not roll the pod.
	if err := svc.SeedEnvVars(ctx, "web", map[string]string{"K": "v"}, nil); err != nil {
		t.Fatalf("SeedEnvVars re-seed: %v", err)
	}
	if got := getApp(t, svc.Client, "web").Spec.RestartedAt; got != firstRA {
		t.Errorf("no-op re-seed bumped RestartedAt: %q -> %q", firstRA, got)
	}
}

func TestSeedEnvVarsNoStore(t *testing.T) {
	svc := &Service{Base: newService(newFakeSecretStore(), sampleApp("web")).Base} // Store nil
	if err := svc.SeedEnvVars(context.Background(), "web", map[string]string{"K": "v"}, nil); !errors.Is(err, core.ErrSecretsUnavailable) {
		t.Errorf("SeedEnvVars no store => ErrSecretsUnavailable, got %v", err)
	}
}
