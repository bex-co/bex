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
	"fmt"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/m234: "Save and deploy" on a suspended service answered rolledOut:true
// and filed a deploy the reconciler then canceled. A deploying save on a
// suspended service now stages like Save only and says why, in one sentence,
// for both deploy and rebuild; the save stands.
func TestADeployingSaveOnASuspendedServiceStagesAndSaysWhy(t *testing.T) {
	for _, mode := range []SaveMode{SaveModeDeploy, SaveModeRebuild} {
		t.Run(string(mode), func(t *testing.T) {
			store := newFakeSecretStore()
			app := sampleApp("web")
			app.Spec.Suspended = true
			svc := newService(store, app)
			rebuilt := 0
			svc.RebuildService = func(context.Context, string) error { rebuilt++; return nil }

			result, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
				SaveMode: mode,
				EnvVars:  []EnvVarPatch{{Key: "GREETING", Value: "hello"}},
			})
			if err != nil {
				t.Fatalf("PatchEnvironment: %v", err)
			}
			if result.RolledOut || result.RolloutError != suspendedSaveMessage || rebuilt != 0 {
				t.Fatalf("result %+v (rebuilt %d), want staged with the suspended sentence", result, rebuilt)
			}
			if strings.Contains(result.RolloutError, "conflict") || strings.Contains(result.RolloutError, "srv-") {
				t.Fatalf("rolloutError is not product English: %q", result.RolloutError)
			}
			if store.m[envPath("web")]["GREETING"] != "hello" {
				t.Fatal("the save did not stand")
			}
			if got := getApp(t, svc.Client, "web"); got.Spec.RestartedAt != "" {
				t.Fatalf("a suspended save bumped restartedAt to %q", got.Spec.RestartedAt)
			}
		})
	}
}

// A rebuild refused because a suspend landed after the read gets the same
// sentence, not the trigger's raw "conflict: service \"srv-…\" is suspended".
func TestARebuildRefusedByARacingSuspendSaysTheSameSentence(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	svc.RebuildService = func(ctx context.Context, _ string) error {
		a := getApp(t, svc.Client, "web")
		a.Spec.Suspended = true
		if err := svc.Client.Update(ctx, a); err != nil {
			t.Fatal(err)
		}
		return fmt.Errorf("%w: service %q is suspended", core.ErrConflict, "srv-c185th5c2rvvnhbfiltg")
	}
	result, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeRebuild,
		EnvVars:  []EnvVarPatch{{Key: "GREETING", Value: "hello"}},
	})
	if err != nil || result.RolledOut || result.RolloutError != suspendedSaveMessage {
		t.Fatalf("result %+v, %v; want the suspended sentence", result, err)
	}
}

// Control: a running service's deploy save still rolls out.
func TestADeploySaveOnARunningServiceStillRollsOut(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	result, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy,
		EnvVars:  []EnvVarPatch{{Key: "GREETING", Value: "hello"}},
	})
	if err != nil || !result.RolledOut || result.RolloutError != "" {
		t.Fatalf("result %+v, %v; want rolled out", result, err)
	}
}

// The locked write decides from the App it writes: a suspend that lands
// between PatchEnvironment's read and the write stages too.
func TestTheLockedWriteStagesForASuspendThatRacedTheRead(t *testing.T) {
	store := newFakeSecretStore()
	app := sampleApp("web")
	svc := newService(store, app)
	suspended := getApp(t, svc.Client, "web")
	suspended.Spec.Suspended = true
	if err := svc.Client.Update(context.Background(), suspended); err != nil {
		t.Fatal(err)
	}
	// app is the stale, running read; the stored App is suspended.
	result, err := svc.patchEnvironmentSparse(context.Background(), "web", app, EnvironmentPatch{
		SaveMode: SaveModeDeploy,
		EnvVars:  []EnvVarPatch{{Key: "GREETING", Value: "hello"}},
	})
	if err != nil || result.RolledOut || result.RolloutError != suspendedSaveMessage {
		t.Fatalf("result %+v, %v; want staged with the suspended sentence", result, err)
	}
}

// Each adapter projects the result itself, so the sentence is checked on all
// three, and a caller without can_create is refused before suspension matters.
func TestTheSuspendedSaveAnswersAlikeOnEverySurface(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			app := sampleApp("web")
			app.Spec.Suspended = true
			svc := newService(newFakeSecretStore(), app)
			result := environmentPatchSurfaceCall(t, svc, surface)(map[string]any{
				"saveMode": string(SaveModeDeploy), "envVars": []map[string]any{{"key": "TOKEN", "value": "staged"}},
			})
			if result["rolledOut"] != false || result["rolloutError"] != suspendedSaveMessage {
				t.Fatalf("%s answered %#v, want rolledOut false with the suspended sentence", surface, result)
			}
		})
	}

	app := sampleApp("web")
	app.Spec.Suspended = true
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user", Method: "oauth2"})
	svc := &Service{Base: &core.Base{Client: fakeClient(app), Namespace: "default", Authz: &fakeChecker{allow: false}}, Store: newFakeSecretStore()}
	if _, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeDeploy}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a caller without can_create got %v, want the authorization error", err)
	}
}
