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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprintApp is a service created from a render.yaml whose manifest declares
// MESSAGE as a plain literal. Blueprint literals deliberately stay on spec.Env
// rather than moving into the mutable store (the w6/m45 carve-out), and the
// operator feeds spec.Env straight to the container.
func blueprintApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Env = []appv1alpha1.EnvVar{
		{Name: "MESSAGE", Value: "hello from bex"},
		// A Secret-key reference has no value to show and no view to live in —
		// it must be skipped, not rendered as an empty variable.
		{Name: "FROM_SECRET", ValueFrom: &appv1alpha1.EnvVarSource{
			SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "other", Key: "k"},
		}},
	}
	return a
}

// TestManifestEnvIsVisibleOnEveryRead is w4/m120's read half. Live on
// 2026-09-21 a blueprint service whose manifest declared MESSAGE answered
// GET env-vars with [] (twice, five minutes after a successful sync) and
// GET env-vars/MESSAGE with 404 — while its process was running the manifest's
// value the whole time, because Kubernetes `env` beats the `envFrom`
// projection the store materializes into.
func TestManifestEnvIsVisibleOnEveryRead(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, blueprintApp("bp"))
	ctx := context.Background()

	// A store-owned variable alongside it: both must show, and only the
	// manifest one may be flagged.
	if _, err := svc.SetEnvVar(ctx, "bp", "TOKEN", EnvVarWrite{Value: "t0ken"}); err != nil {
		t.Fatalf("SetEnvVar: %v", err)
	}

	list, err := svc.ListEnvVars(ctx, "bp")
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	byKey := map[string]EnvVarView{}
	for _, v := range list {
		if _, dup := byKey[v.Key]; dup {
			t.Fatalf("%s appears twice in one list: %+v", v.Key, list)
		}
		byKey[v.Key] = v
	}
	if len(byKey) != 2 {
		t.Fatalf("list = %+v, want MESSAGE (manifest) and TOKEN (store)", list)
	}
	if got := byKey["MESSAGE"]; got.Value != "hello from bex" || got.ManagedBy != core.ManagedByBlueprint {
		t.Errorf("manifest var = %+v, want the manifest value flagged blueprint", got)
	}
	if got := byKey["TOKEN"]; got.Value != "t0ken" || got.ManagedBy != "" {
		t.Errorf("store var = %+v, want an unflagged editable variable", got)
	}
	if _, leaked := byKey["FROM_SECRET"]; leaked {
		t.Error("a ValueFrom reference has no value to show and must not appear as a variable")
	}

	// The single-key read agreed with the list only by 404ing before this fix.
	one, err := svc.GetEnvVar(ctx, "bp", "MESSAGE")
	if err != nil {
		t.Fatalf("GetEnvVar(MESSAGE): %v", err)
	}
	if one.Value != "hello from bex" || one.ManagedBy != core.ManagedByBlueprint {
		t.Errorf("GetEnvVar = %+v, want the manifest value flagged blueprint", one)
	}

	// The dashboard seam has to know a row is read-only BEFORE anyone reveals
	// its value, or it renders an editable field for a variable no write can
	// change — so the flag rides the key list too.
	keys, err := svc.EnvVarKeys(ctx, "bp")
	if err != nil {
		t.Fatalf("EnvVarKeys: %v", err)
	}
	flags := map[string]string{}
	for _, k := range keys {
		flags[k.Key] = k.ManagedBy
		if k.Value != "" {
			t.Errorf("key list must not carry values: %+v", k)
		}
	}
	if flags["MESSAGE"] != core.ManagedByBlueprint || flags["TOKEN"] != "" {
		t.Errorf("key-list flags = %+v", flags)
	}
	value, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
	if err != nil || value.Value != "hello from bex" || value.ManagedBy != core.ManagedByBlueprint {
		t.Errorf("EnvVarValue = %+v (err %v)", value, err)
	}
}

// TestNonBlueprintServiceReadsAreUnchanged is the regression target: an
// ordinary service's spec.Env is empty after w6/m45, so the merge must be a
// no-op — same keys, same values, and no managedBy on the wire at all.
func TestNonBlueprintServiceReadsAreUnchanged(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()
	if _, err := svc.SetEnvVars(ctx, "web", []EnvVarView{{Key: "FOO", Value: "bar"}}); err != nil {
		t.Fatalf("SetEnvVars: %v", err)
	}
	list, err := svc.ListEnvVars(ctx, "web")
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	if len(list) != 1 || list[0].Key != "FOO" || list[0].Value != "bar" || list[0].ManagedBy != "" {
		t.Fatalf("ordinary service list = %+v, want one unflagged FOO=bar", list)
	}
	if _, err := svc.GetEnvVar(ctx, "web", "NOPE"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown key on an ordinary service = %v, want ErrNotFound", err)
	}
}

// TestManifestKeyWritesAreRefused is w4/m120's write half, and the one that
// matters most: before it, a write to a manifest-owned key SUCCEEDED and did
// nothing observable. It landed in the store, the store was projected into the
// <name>-env Secret, and the container read that Secret through envFrom — which
// the manifest's own `env` entry beats. The API said 200, the Environment tab
// showed the new value, and the process kept the old one.
func TestManifestKeyWritesAreRefused(t *testing.T) {
	const manifestValue = "hello from bex"

	assertRefused := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("a write to a manifest-owned key must be refused, not silently shadowed")
		}
		if !errors.Is(err, core.ErrConflict) {
			t.Errorf("refusal = %v, want the conflict class", err)
		}
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != CodeEnvVarManifestManaged {
			t.Errorf("refusal must carry the stable %s code, got %v", CodeEnvVarManifestManaged, err)
		}
	}

	t.Run("SetEnvVar", func(t *testing.T) {
		store := newFakeSecretStore()
		svc := newService(store, blueprintApp("bp"))
		_, err := svc.SetEnvVar(context.Background(), "bp", "MESSAGE", EnvVarWrite{Value: "overwritten"})
		assertRefused(t, err)
		if _, wrote := store.m[envPath("bp")]["MESSAGE"]; wrote {
			t.Error("a refused write must leave nothing in the store")
		}
		// The read still reports what the process actually has.
		got, err := svc.GetEnvVar(context.Background(), "bp", "MESSAGE")
		if err != nil || got.Value != manifestValue {
			t.Errorf("after the refusal MESSAGE = %+v (err %v), want the manifest value", got, err)
		}
	})

	t.Run("DeleteEnvVar", func(t *testing.T) {
		svc := newService(newFakeSecretStore(), blueprintApp("bp"))
		assertRefused(t, svc.DeleteEnvVar(context.Background(), "bp", "MESSAGE"))
	})

	t.Run("SetEnvVars refuses the whole bulk write", func(t *testing.T) {
		store := newFakeSecretStore()
		svc := newService(store, blueprintApp("bp"))
		_, err := svc.SetEnvVars(context.Background(), "bp", []EnvVarView{
			{Key: "INNOCENT", Value: "fine"},
			{Key: "MESSAGE", Value: "overwritten"},
		})
		assertRefused(t, err)
		// All-or-nothing: the caller wrote this as one unit, so the innocent
		// half must not land either.
		if _, wrote := store.m[envPath("bp")]["INNOCENT"]; wrote {
			t.Error("a refused bulk write must not apply its innocent half")
		}
	})

	t.Run("PatchEnvironment", func(t *testing.T) {
		svc := newService(newFakeSecretStore(), blueprintApp("bp"))
		_, err := svc.PatchEnvironment(context.Background(), "bp", EnvironmentPatch{
			SaveMode: SaveModeOnly,
			EnvVars:  []EnvVarPatch{{Key: "MESSAGE", Value: "overwritten", ValueSet: true}},
		})
		assertRefused(t, err)
	})

	t.Run("PatchEnvironment rename away from a manifest key", func(t *testing.T) {
		// A rename deletes the source key, so naming a manifest key as fromKey
		// is the same shadowing hazard by another route.
		svc := newService(newFakeSecretStore(), blueprintApp("bp"))
		_, err := svc.PatchEnvironment(context.Background(), "bp", EnvironmentPatch{
			SaveMode: SaveModeOnly,
			EnvVars:  []EnvVarPatch{{Key: "RENAMED", FromKey: "MESSAGE", ValueSet: true}},
		})
		assertRefused(t, err)
	})
}

// TestNonManifestKeysStayWritable is the other regression target: the guard
// must not cost a blueprint service its ordinary env-var editing, which is how
// a `sync: false` or generateValue variable is managed.
func TestNonManifestKeysStayWritable(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, blueprintApp("bp"))
	ctx := context.Background()

	if _, err := svc.SetEnvVar(ctx, "bp", "TOKEN", EnvVarWrite{Value: "t0ken"}); err != nil {
		t.Fatalf("a key absent from the manifest must stay writable: %v", err)
	}
	// SetEnvVars is Render's whole-set REPLACE, so this drops TOKEN — and
	// leaves MESSAGE standing, because the manifest holds it on spec.Env where
	// a store replace cannot reach it. That asymmetry is the point: the
	// manifest's variables are not the store's to remove.
	if _, err := svc.SetEnvVars(ctx, "bp", []EnvVarView{{Key: "A", Value: "1"}, {Key: "B", Value: "2"}}); err != nil {
		t.Fatalf("bulk write of non-manifest keys: %v", err)
	}
	if err := svc.DeleteEnvVar(ctx, "bp", "A"); err != nil {
		t.Fatalf("deleting a non-manifest key: %v", err)
	}
	if _, err := svc.PatchEnvironment(ctx, "bp", EnvironmentPatch{
		SaveMode: SaveModeOnly,
		EnvVars:  []EnvVarPatch{{Key: "C", Value: "3", ValueSet: true}},
	}); err != nil {
		t.Fatalf("patching a non-manifest key: %v", err)
	}
	list, err := svc.ListEnvVars(ctx, "bp")
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	keys := map[string]bool{}
	for _, v := range list {
		keys[v.Key] = true
	}
	for _, want := range []string{"MESSAGE", "B", "C"} {
		if !keys[want] {
			t.Errorf("%s missing from %+v", want, list)
		}
	}
	if keys["A"] {
		t.Error("A was deleted and must not be listed")
	}
	if keys["TOKEN"] {
		t.Error("TOKEN was replaced away by the whole-set write and must not be listed")
	}
}
