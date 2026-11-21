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
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// retainedApp is a Blueprint service whose first manifest declared MESSAGE and
// KEEPME and whose latest sync declares only KEEPME: the sync left MESSAGE on
// spec.Env, as Render keeps an omitted variable, and recorded what the
// manifest still declares. Production, 2026-11-16: DELETE and PUT of such a
// MESSAGE answered 409 ENV_VAR_MANIFEST_MANAGED ("declared in this service's
// render.yaml manifest") although the manifest no longer named it, and nothing
// the caller could do freed it. The Blueprint is still connected (it carries
// the label).
func retainedApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{core.LabelBlueprint: "exs-connected"}
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

// retainedManagedApp is retainedApp as a control-plane service, whose rolling
// writes open the deploy rows they owe.
func retainedManagedApp(name string) *appv1alpha1.App {
	a, r := managedApp(name), retainedApp(name)
	a.Annotations, a.Spec.Env = r.Annotations, r.Spec.Env
	maps.Copy(a.Labels, r.Labels)
	return a
}

// patchRetained is the save of a client that checks the revision it listed (the
// mobile app, MCP, REST and GraphQL callers of expectedEnvRevision; the web
// dashboard's editor sends none): one value edit of MESSAGE.
func patchRetained(svc *Service, revision *string, mode SaveMode) (EnvironmentPatchResult, error) {
	return svc.PatchEnvironment(context.Background(), "bp", EnvironmentPatch{
		SaveMode: mode, ExpectedEnvRevision: revision,
		EnvVars: []EnvVarPatch{{Key: "MESSAGE", Value: "edited", ValueSet: true}},
	})
}

func conflictCode(t *testing.T, err error) string {
	t.Helper()
	var coded *core.CodedError
	if !errors.Is(err, core.ErrConflict) || !errors.As(err, &coded) {
		t.Fatalf("error = %v, want a coded conflict", err)
	}
	return coded.Code
}

// A client that checks the revision it listed (the mobile app, MCP, REST and
// GraphQL callers of expectedEnvRevision) lists a retained literal at the
// store's revision. Its first save of the variable lands: the store takes the
// literal with the edit in ONE new revision, checked against the listed one,
// and the same App patch removes the literal from spec.Env (w21/103). Moving
// the literal in a write of its own advanced the revision under the caller and
// refused that save as stale, and only the second try landed.
func TestASaveAgainstTheListedRevisionNamingARetainedLiteral(t *testing.T) {
	ctx := context.Background()
	kv := newVersionedFakeSecretStore()
	svc := newService(kv, retainedApp("bp"))
	counting := &patchCountingClient{Client: svc.Client}
	svc.Client = counting
	listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
	if err != nil {
		t.Fatal(err)
	}
	result, err := patchRetained(svc, &listed.Revision, SaveModeOnly)
	if err != nil {
		t.Fatalf("the first save against the listed revision: %v", err)
	}
	if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "edited" || got.ManagedBy != "" {
		t.Fatalf("after the save MESSAGE = %+v (err %v), want %q with no owner", got, err, "edited")
	}
	app := getApp(t, svc.Client, "bp")
	if got := specEnvNames(app); len(got) != 1 || got["KEEPME"] != "1" {
		t.Errorf("spec.Env = %v, want the literal moved off and the declared KEEPME untouched", got)
	}
	// One store write, one revision, one App patch: the move rides the edit.
	if got := kv.versions[envPath("bp")]; got != 1 || kv.casCalls != 1 {
		t.Errorf("the save advanced the store to revision %d in %d writes, want 1 in 1", got, kv.casCalls)
	}
	if counting.patches != 1 {
		t.Errorf("the save wrote the App %d times, want once", counting.patches)
	}
	if result.Revision == nil || *result.Revision != encodeEnvRevision(1) {
		t.Errorf("result revision = %v, want the one revision the save committed", result.Revision)
	}
	if got := kv.m[envPath("bp")]; !maps.Equal(got, map[string]string{"MESSAGE": "edited"}) {
		t.Errorf("the store holds %v, want exactly the edited MESSAGE", got)
	}
	if got := string(getSecret(t, svc.Client, "bp-env").Data["MESSAGE"]); got != "edited" {
		t.Errorf("the projected Secret carries %q, want the edited value", got)
	}
	// The returned token chains: MESSAGE is an ordinary stored variable now.
	if _, err := patchRetained(svc, result.Revision, SaveModeOnly); err != nil {
		t.Fatalf("a second save with the returned revision: %v", err)
	}
}

// A revision that really is stale is still refused when the save names a
// retained literal, whether the environment changed after the caller listed it
// or between this save's read and its write; only the move that bumped the
// revision under the caller is gone. A refused save writes nothing: the literal
// stays where it was and the value is kept.
func TestAStaleRevisionIsStillRefusedForARetainedLiteral(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		// race makes the environment change, from listed, before the save lands.
		race func(t *testing.T, svc *Service, kv *versionedFakeSecretStore)
	}{
		{"changed after the list", func(t *testing.T, svc *Service, _ *versionedFakeSecretStore) {
			if _, err := svc.SetEnvVar(ctx, "bp", "OTHER", EnvVarWrite{Value: "2"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"changed between the save's read and its write", func(t *testing.T, svc *Service, kv *versionedFakeSecretStore) {
			kv.afterGet = func() {
				if _, err := svc.SetEnvVar(ctx, "bp", "OTHER", EnvVarWrite{Value: "2"}); err != nil {
					t.Error(err)
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kv := newVersionedFakeSecretStore()
			svc := newService(kv, retainedApp("bp"))
			listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
			if err != nil {
				t.Fatal(err)
			}
			tc.race(t, svc, kv)
			wrote := kv.versions[envPath("bp")]
			_, err = patchRetained(svc, &listed.Revision, SaveModeDeploy)
			if code := conflictCode(t, err); code != "ENVIRONMENT_REVISION_CONFLICT" {
				t.Fatalf("the stale save answered %s, want ENVIRONMENT_REVISION_CONFLICT", code)
			}
			if tc.name == "changed after the list" && kv.versions[envPath("bp")] != wrote {
				t.Errorf("the refused save wrote the store: revision %d -> %d", wrote, kv.versions[envPath("bp")])
			}
			if got := specEnvNames(getApp(t, svc.Client, "bp")); got["MESSAGE"] != "hello from bex" {
				t.Errorf("spec.Env = %v, want the literal left where it was", got)
			}
			if _, stored := kv.m[envPath("bp")]["MESSAGE"]; stored {
				t.Errorf("the refused save stored MESSAGE: %v", kv.m[envPath("bp")])
			}
			if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "hello from bex" {
				t.Errorf("after the refused save MESSAGE = %+v (err %v), want its value kept", got, err)
			}
			// A save against the revision the listing now reports lands.
			refreshed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := patchRetained(svc, &refreshed.Revision, SaveModeDeploy); err != nil {
				t.Fatalf("the save against the refreshed revision: %v", err)
			}
			if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "edited" {
				t.Errorf("after the refreshed save MESSAGE = %+v (err %v), want %q", got, err, "edited")
			}
		})
	}
}

// A save that leaves the stored map as it was still moves the literal: the
// store already held the literal's value (a stored duplicate beside the literal
// that shadowed it), and the edit sets that same value. The revision advances
// as every revision-checked save does, and the literal leaves spec.Env.
func TestASaveThatChangesNothingStillMovesTheLiteral(t *testing.T) {
	ctx := context.Background()
	kv := newVersionedFakeSecretStore()
	kv.m[envPath("bp")] = map[string]string{"MESSAGE": "hello from bex"}
	kv.versions[envPath("bp")] = 3
	svc := newService(kv, retainedApp("bp"))
	listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.PatchEnvironment(ctx, "bp", EnvironmentPatch{
		SaveMode: SaveModeDeploy, ExpectedEnvRevision: &listed.Revision,
		EnvVars: []EnvVarPatch{{Key: "MESSAGE", Value: "hello from bex", ValueSet: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision == nil || *result.Revision != encodeEnvRevision(4) {
		t.Errorf("result revision = %v, want the advanced one", result.Revision)
	}
	if got := specEnvNames(getApp(t, svc.Client, "bp")); len(got) != 1 || got["KEEPME"] != "1" {
		t.Errorf("spec.Env = %v, want the literal gone", got)
	}
	if got := string(getSecret(t, svc.Client, "bp-env").Data["MESSAGE"]); got != "hello from bex" {
		t.Errorf("the projected Secret carries %q, want the value the literal held", got)
	}
}

// The web dashboard's editor sends a sparse patch, with no revision. An edit
// and a delete of a retained variable each land first try as ONE write: the
// store takes the move with the change, and the App is patched once with the
// literal gone (the move used to be a write and an App patch of its own).
func TestASparsePatchOfARetainedLiteralIsOneWrite(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		patch  EnvVarPatch
		stored map[string]string
	}{
		{"edit", EnvVarPatch{Key: "MESSAGE", Value: "edited", ValueSet: true}, map[string]string{"MESSAGE": "edited", "OTHER": "2"}},
		{"delete", EnvVarPatch{Key: "MESSAGE", Delete: true}, map[string]string{"OTHER": "2"}},
		// The move is the whole change: the edit sets the value the literal held.
		{"edit to the literal's own value", EnvVarPatch{Key: "MESSAGE", Value: "hello from bex", ValueSet: true}, map[string]string{"MESSAGE": "hello from bex", "OTHER": "2"}},
		{"rename", EnvVarPatch{Key: "RENAMED", FromKey: "MESSAGE"}, map[string]string{"RENAMED": "hello from bex", "OTHER": "2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kv := newVersionedFakeSecretStore()
			kv.m[envPath("bp")] = map[string]string{"OTHER": "2"}
			kv.versions[envPath("bp")] = 4
			svc := newService(kv, retainedApp("bp"))
			counting := &patchCountingClient{Client: svc.Client}
			svc.Client = counting
			if _, err := svc.PatchEnvironment(ctx, "bp", EnvironmentPatch{SaveMode: SaveModeDeploy, EnvVars: []EnvVarPatch{tc.patch}}); err != nil {
				t.Fatalf("the first patch of a retained literal: %v", err)
			}
			if got := kv.m[envPath("bp")]; !maps.Equal(got, tc.stored) {
				t.Errorf("the store holds %v, want %v", got, tc.stored)
			}
			if got, want := kv.versions[envPath("bp")], uint64(5); got != want || kv.casCalls != 1 {
				t.Errorf("revision %d after %d writes, want %d after 1", got, kv.casCalls, want)
			}
			if counting.patches != 1 {
				t.Errorf("the App was patched %d times, want once", counting.patches)
			}
			if got := specEnvNames(getApp(t, svc.Client, "bp")); len(got) != 1 || got["KEEPME"] != "1" {
				t.Errorf("spec.Env = %v, want only the declared KEEPME", got)
			}
		})
	}
}

// Save only keeps what it saves for the next deploy. Moving a literal off
// spec.Env changes the pod template, which rolls a release, so the save-only
// patch holds that change as every other save-only write does and opens no
// deploy; a deploying save opens exactly one. Neither loses the variable: the
// store keeps it and the Secret is wired for the release that takes it.
func TestASaveOfARetainedLiteralRollsOnlyWhenItDeploys(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name       string
		mode       SaveMode
		revision   bool
		wantDeploy int
	}{
		{"save only, revision-checked", SaveModeOnly, true, 0},
		{"deploy, revision-checked", SaveModeDeploy, true, 1},
		{"save only, sparse", SaveModeOnly, false, 0},
		{"deploy, sparse", SaveModeDeploy, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingDeploys{}
			svc := newService(newVersionedFakeSecretStore(), retainedManagedApp("bp"))
			svc.Rollout = &rollout.Tracker{Store: rec}
			var revision *string
			if tc.revision {
				listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
				if err != nil {
					t.Fatal(err)
				}
				revision = &listed.Revision
			}
			result, err := patchRetained(svc, revision, tc.mode)
			if err != nil {
				t.Fatal(err)
			}
			if len(rec.rows) != tc.wantDeploy || result.RolledOut != (tc.wantDeploy == 1) {
				t.Errorf("%s opened %d deploys (rolledOut %v), want %d", tc.mode, len(rec.rows), result.RolledOut, tc.wantDeploy)
			}
			app := getApp(t, svc.Client, "bp")
			held := app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != ""
			if held != (tc.mode == SaveModeOnly) {
				t.Errorf("the literal's removal held for the next deploy = %v, want %v", held, tc.mode == SaveModeOnly)
			}
			if _, still := specEnvNames(app)["MESSAGE"]; still {
				t.Error("spec.Env still carries MESSAGE")
			}
			if tc.mode == SaveModeDeploy && app.Spec.EnvFromSecret != "bp-env" {
				t.Errorf("EnvFromSecret = %q, want the Secret holding MESSAGE wired", app.Spec.EnvFromSecret)
			}
			if tc.mode == SaveModeOnly && app.Annotations[appv1alpha1.PendingEnvSecretAnnotation] != "bp-env" {
				t.Errorf("the Secret holding MESSAGE is not staged for the next deploy: %v", app.Annotations)
			}
			if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "edited" {
				t.Errorf("after the save MESSAGE = %+v (err %v), want %q", got, err, "edited")
			}
		})
	}
}

// What the manifest declares is still refused before anything is written, and a
// sync that re-declares the variable after the save read the App but before the
// App patch lands is not overwritten: the save is refused and rolled back, the
// literal stays the manifest's, and the store does not hold a value the process
// would never see. Nothing is half-applied on either protocol.
func TestARetainedLiteralTheManifestRedeclaresIsNeitherStoredNorDropped(t *testing.T) {
	ctx := context.Background()
	for _, protocol := range []string{"revision-checked", "sparse"} {
		t.Run("declared at the read/"+protocol, func(t *testing.T) {
			kv := newVersionedFakeSecretStore()
			app := retainedApp("bp")
			app.Annotations[core.AnnotationBlueprintEnv] = "KEEPME,MESSAGE"
			svc := newService(kv, app)
			var revision *string
			if protocol == "revision-checked" {
				listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
				if err != nil {
					t.Fatal(err)
				}
				revision = &listed.Revision
			}
			_, err := patchRetained(svc, revision, SaveModeDeploy)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != CodeEnvVarManifestManaged {
				t.Fatalf("a save of the declared MESSAGE = %v, want %s", err, CodeEnvVarManifestManaged)
			}
			if kv.casCalls != 0 || len(kv.m[envPath("bp")]) != 0 {
				t.Errorf("the refused save wrote the store (%d writes, %v)", kv.casCalls, kv.m[envPath("bp")])
			}
		})
		t.Run("re-declared after the read/"+protocol, func(t *testing.T) {
			kv := newVersionedFakeSecretStore()
			kv.m[envPath("bp")] = map[string]string{"OTHER": "2"}
			kv.versions[envPath("bp")] = 1
			svc := newService(kv, retainedApp("bp"))
			var revision *string
			if protocol == "revision-checked" {
				listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
				if err != nil {
					t.Fatal(err)
				}
				revision = &listed.Revision
			}
			// The sync lands as soon as the store has taken the save's write.
			kv.afterCAS = func(string, uint64) {
				a := getApp(t, svc.Client, "bp")
				a.Annotations[core.AnnotationBlueprintEnv] = "KEEPME,MESSAGE"
				if err := svc.Client.Update(ctx, a); err != nil {
					t.Error(err)
				}
			}
			_, err := patchRetained(svc, revision, SaveModeDeploy)
			if !errors.Is(err, core.ErrConflict) {
				t.Fatalf("a save the manifest overtook = %v, want a refusal in the conflict class", err)
			}
			app := getApp(t, svc.Client, "bp")
			if got := specEnvNames(app); got["MESSAGE"] != "hello from bex" || got["KEEPME"] != "1" {
				t.Errorf("spec.Env = %v, want the manifest's literals as the sync left them", got)
			}
			if got := kv.m[envPath("bp")]; !maps.Equal(got, map[string]string{"OTHER": "2"}) {
				t.Errorf("the store holds %v, want what it held before the refused save", got)
			}
		})
	}
}

// A failure after the store write rolls the whole save back: the store holds
// what it held, the literal is still on spec.Env, and the variable still lists
// with its value. The failure here is the App patch, the last step.
func TestAFailedSaveOfARetainedLiteralLeavesItWhereItWas(t *testing.T) {
	ctx := context.Background()
	for _, protocol := range []string{"revision-checked", "sparse"} {
		t.Run(protocol, func(t *testing.T) {
			kv := newVersionedFakeSecretStore()
			kv.m[envPath("bp")] = map[string]string{"OTHER": "2"}
			kv.versions[envPath("bp")] = 1
			svc := newService(kv, retainedApp("bp"))
			var revision *string
			if protocol == "revision-checked" {
				listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
				if err != nil {
					t.Fatal(err)
				}
				revision = &listed.Revision
			}
			svc.Client = &patchCountingClient{Client: svc.Client, fail: errors.New("injected App patch failure")}
			if _, err := patchRetained(svc, revision, SaveModeDeploy); err == nil {
				t.Fatal("the save succeeded although the App patch failed")
			}
			if got := kv.m[envPath("bp")]; !maps.Equal(got, map[string]string{"OTHER": "2"}) {
				t.Errorf("the store holds %v, want what it held before the failed save", got)
			}
			if got := specEnvNames(getApp(t, svc.Client.(*patchCountingClient).Client, "bp")); got["MESSAGE"] != "hello from bex" {
				t.Errorf("spec.Env = %v, want the literal kept", got)
			}
			svc.Client = svc.Client.(*patchCountingClient).Client
			if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "hello from bex" || got.ManagedBy != "" {
				t.Errorf("after the failed save MESSAGE = %+v (err %v), want it listed with its value", got, err)
			}
		})
	}
}

// The single-variable verbs and the bulk replace of REST check no revision and
// move the literal inside their own write: each lands, and takes the literal off
// spec.Env, whatever store backs them.
func TestRESTWritesOfARetainedLiteralStillLand(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		want   map[string]string
	}{
		{"PUT one", http.MethodPut, "/v1/services/bp/env-vars/MESSAGE", `{"value":"changed"}`, map[string]string{"MESSAGE": "changed"}},
		{"DELETE one", http.MethodDelete, "/v1/services/bp/env-vars/MESSAGE", ``, map[string]string{}},
		{"PUT all", http.MethodPut, "/v1/services/bp/env-vars", `[{"key":"MESSAGE","value":"kept"},{"key":"OTHER","value":"2"}]`, map[string]string{"MESSAGE": "kept", "OTHER": "2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kv := newVersionedFakeSecretStore()
			svc := newService(kv, retainedApp("bp"))
			w := serveREST(svc, tc.method, tc.path, tc.body)
			if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
				t.Fatalf("%s %s = %d: %s", tc.method, tc.path, w.Code, w.Body.String())
			}
			if got := kv.m[envPath("bp")]; !maps.Equal(got, tc.want) {
				t.Errorf("the store holds %v, want %v", got, tc.want)
			}
			if got := specEnvNames(getApp(t, svc.Client, "bp")); len(got) != 1 || got["KEEPME"] != "1" {
				t.Errorf("spec.Env = %v, want only the declared KEEPME", got)
			}
		})
	}
}

// Both revision-aware wire surfaces carry the save: REST's PATCH and
// GraphQL's patchServiceEnvironment, each with the revision the list returned.
func TestARetainedLiteralSaveLandsFirstTryOnTheWire(t *testing.T) {
	ctx := context.Background()
	t.Run("REST", func(t *testing.T) {
		svc := newService(newVersionedFakeSecretStore(), retainedApp("bp"))
		listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(map[string]any{"saveMode": "save_only", "expectedEnvRevision": listed.Revision,
			"envVars": []map[string]any{{"key": "MESSAGE", "value": "edited"}}})
		w := serveREST(svc, http.MethodPatch, "/v1/services/bp/environment", string(body))
		if w.Code != http.StatusOK {
			t.Fatalf("PATCH = %d: %s", w.Code, w.Body.String())
		}
		if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "edited" {
			t.Fatalf("MESSAGE = %+v (err %v), want %q", got, err, "edited")
		}
	})
	t.Run("GraphQL", func(t *testing.T) {
		svc := newService(newVersionedFakeSecretStore(), retainedApp("bp"))
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{"ok": &graphql.Field{Type: graphql.Boolean}}}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		listed, err := svc.EnvVarValue(ctx, "bp", "MESSAGE")
		if err != nil {
			t.Fatal(err)
		}
		result := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
			RequestString:  `mutation($revision:String) { patchServiceEnvironment(serviceId:"bp", saveMode:"save_only", expectedEnvRevision:$revision, envVars:[{key:"MESSAGE",value:"edited"}]) { revision } }`,
			VariableValues: map[string]any{"revision": listed.Revision}})
		if len(result.Errors) != 0 {
			t.Fatalf("GraphQL: %v", result.Errors)
		}
		if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != "edited" {
			t.Fatalf("MESSAGE = %+v (err %v), want %q", got, err, "edited")
		}
	})
}
