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

// retainedApp is a Blueprint service whose first manifest declared MESSAGE and
// KEEPME and whose latest sync declares only KEEPME: the sync left MESSAGE on
// spec.Env, as Render keeps an omitted variable, and recorded what the
// manifest still declares. Production, 2026-11-16: DELETE and PUT of such a
// MESSAGE answered 409 ENV_VAR_MANIFEST_MANAGED ("declared in this service's
// render.yaml manifest") although the manifest no longer named it, and nothing
// the caller could do freed it.
func retainedApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Annotations = map[string]string{core.AnnotationBlueprintEnv: "KEEPME"}
	a.Spec.Env = []appv1alpha1.EnvVar{
		{Name: "MESSAGE", Value: "hello from bex"},
		{Name: "KEEPME", Value: "1"},
	}
	return a
}

func specEnvNames(a *appv1alpha1.App) map[string]string {
	out := map[string]string{}
	for _, item := range a.Spec.Env {
		out[item.Name] = item.Value
	}
	return out
}

// A variable the manifest dropped is listed as an ordinary one, while the one
// it still declares stays the manifest's.
func TestARetainedLiteralListsAsAnOrdinaryVariable(t *testing.T) {
	svc := newService(newFakeSecretStore(), retainedApp("bp"))
	list, err := svc.ListEnvVars(context.Background(), "bp")
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	by := map[string]EnvVarView{}
	for _, v := range list {
		by[v.Key] = v
	}
	if got := by["MESSAGE"]; got.Value != "hello from bex" || got.ManagedBy != "" {
		t.Errorf("retained MESSAGE = %+v, want its value with no owner", got)
	}
	if got := by["KEEPME"]; got.ManagedBy != core.ManagedByBlueprint {
		t.Errorf("declared KEEPME = %+v, want it flagged blueprint", got)
	}
}

// Delete, set, bulk replace and the dashboard's patch all succeed on a
// retained literal and leave nothing of it on spec.Env, where it would keep
// shadowing the store.
func TestARetainedLiteralCanBeChangedAndDeleted(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name  string
		act   func(t *testing.T, svc *Service) error
		check func(t *testing.T, svc *Service)
	}{
		{"delete", func(t *testing.T, svc *Service) error { return svc.DeleteEnvVar(ctx, "bp", "MESSAGE") },
			func(t *testing.T, svc *Service) {
				if _, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); !errors.Is(err, core.ErrNotFound) {
					t.Errorf("after the delete GetEnvVar(MESSAGE) = %v, want not found", err)
				}
			}},
		{"set one", func(t *testing.T, svc *Service) error {
			_, err := svc.SetEnvVar(ctx, "bp", "MESSAGE", EnvVarWrite{Value: "changed"})
			return err
		}, func(t *testing.T, svc *Service) {
			if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "changed" {
				t.Errorf("after the write MESSAGE = %+v (err %v), want %q", got, err, "changed")
			}
		}},
		{"patch", func(t *testing.T, svc *Service) error {
			_, err := svc.PatchEnvironment(ctx, "bp", EnvironmentPatch{
				SaveMode: SaveModeDeploy,
				EnvVars:  []EnvVarPatch{{Key: "MESSAGE", Value: "patched", ValueSet: true}},
			})
			return err
		}, func(t *testing.T, svc *Service) {
			if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "patched" {
				t.Errorf("after the patch MESSAGE = %+v (err %v), want %q", got, err, "patched")
			}
		}},
		{"replace all keeps what it names", func(t *testing.T, svc *Service) error {
			_, err := svc.SetEnvVars(ctx, "bp", []EnvVarView{{Key: "MESSAGE", Value: "kept"}, {Key: "OTHER", Value: "2"}})
			return err
		}, func(t *testing.T, svc *Service) {
			if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "kept" {
				t.Errorf("after the replace MESSAGE = %+v (err %v), want %q", got, err, "kept")
			}
		}},
		{"replace all drops what it omits", func(t *testing.T, svc *Service) error {
			_, err := svc.SetEnvVars(ctx, "bp", []EnvVarView{{Key: "OTHER", Value: "2"}})
			return err
		}, func(t *testing.T, svc *Service) {
			if _, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); !errors.Is(err, core.ErrNotFound) {
				t.Errorf("after a replace that omits it GetEnvVar(MESSAGE) = %v, want not found", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(newFakeSecretStore(), retainedApp("bp"))
			if err := tc.act(t, svc); err != nil {
				t.Fatalf("the verb on a retained literal: %v", err)
			}
			tc.check(t, svc)
			got := specEnvNames(getApp(t, svc.Client, "bp"))
			if _, still := got["MESSAGE"]; still {
				t.Errorf("spec.Env still carries MESSAGE (%v): the literal would shadow the store", got)
			}
			if got["KEEPME"] != "1" {
				t.Errorf("the declared KEEPME was touched: %v", got)
			}
		})
	}
}

// What the sync still declares stays refused, and an App no sync has recorded
// a declaration for keeps treating every literal as declared.
func TestOnlyAnUndeclaredLiteralIsFreed(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeSecretStore(), retainedApp("bp"))
	err := svc.DeleteEnvVar(ctx, "bp", "KEEPME")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != CodeEnvVarManifestManaged {
		t.Fatalf("deleting the still-declared KEEPME = %v, want %s", err, CodeEnvVarManifestManaged)
	}

	legacy := retainedApp("old")
	delete(legacy.Annotations, core.AnnotationBlueprintEnv)
	svc = newService(newFakeSecretStore(), legacy)
	err = svc.DeleteEnvVar(ctx, "old", "MESSAGE")
	if !errors.As(err, &coded) || coded.Code != CodeEnvVarManifestManaged {
		t.Fatalf("an App with no recorded declaration: delete(MESSAGE) = %v, want %s", err, CodeEnvVarManifestManaged)
	}
}

// The dashboard saves against the revision it listed. Adopting a retained
// literal writes the store, so the first save that names one meets a newer
// revision and is refused as stale; nothing is half-applied, and a save
// against the refreshed revision lands. This pins that order so a later change
// cannot turn the refusal into a silent overwrite or a lost value.
func TestASaveAgainstTheListedRevisionNamingARetainedLiteral(t *testing.T) {
	ctx := context.Background()
	svc := newService(newVersionedFakeSecretStore(), retainedApp("bp"))
	listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
	if err != nil {
		t.Fatal(err)
	}
	patch := func(revision string) error {
		_, err := svc.PatchEnvironment(ctx, "bp", EnvironmentPatch{
			SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision,
			EnvVars: []EnvVarPatch{{Key: "MESSAGE", Value: "edited", ValueSet: true}},
		})
		return err
	}
	if err := patch(listed.Revision); err == nil {
		t.Fatal("a save against the pre-adoption revision was accepted")
	} else if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("the stale save = %v, want the conflict class", err)
	}
	if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "hello from bex" {
		t.Fatalf("after the refused save MESSAGE = %+v (err %v), want its value kept", got, err)
	}
	refreshed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
	if err != nil {
		t.Fatal(err)
	}
	if err := patch(refreshed.Revision); err != nil {
		t.Fatalf("the save against the refreshed revision: %v", err)
	}
	if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "edited" {
		t.Fatalf("after the refreshed save MESSAGE = %+v (err %v), want %q", got, err, "edited")
	}
	if _, still := specEnvNames(getApp(t, svc.Client, "bp"))["MESSAGE"]; still {
		t.Error("spec.Env still carries MESSAGE after the save")
	}
}
