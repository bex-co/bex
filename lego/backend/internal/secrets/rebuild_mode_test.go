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
	"fmt"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/347: "Save, rebuild, and deploy" saved with save_only and then opened a
// Manual Deploy from the client. The rebuild save mode saves and rebuilds in
// one call, through RebuildService, so the deploy reads as the configuration
// change it is.

type rebuildCtxKey struct{}

func TestRebuildSaveStagesThenRebuildsOnce(t *testing.T) {
	store := newFakeSecretStore()
	// Addressed by name, rebuilt by id: a rename can hand the name to another
	// App, so the rebuild must name this one.
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelAppID: "srv-c185th5c2rvvnhbfiltg"}
	svc := newService(store, app)
	var rebuilt []string
	callerCtx := context.WithValue(context.Background(), rebuildCtxKey{}, "caller")
	svc.RebuildService = func(ctx context.Context, serviceID string) (json.RawMessage, error) {
		// The caller's own context: the scoped write context defers
		// allowed-write audits, which would drop the rebuild's audit row.
		if ctx != callerCtx {
			t.Error("rebuild ran with the scoped write context, not the caller's")
		}
		// The save is in place before the rebuild starts.
		if store.m[envPath("web")]["GREETING"] != "hello" {
			t.Errorf("rebuild ran before the save landed: %#v", store.m[envPath("web")])
		}
		rebuilt = append(rebuilt, serviceID)
		return nil, nil
	}

	result, err := svc.PatchEnvironment(callerCtx, "web", EnvironmentPatch{
		SaveMode: SaveModeRebuild,
		EnvVars:  []EnvVarPatch{{Key: "GREETING", Value: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RolledOut || result.RolloutError != "" || fmt.Sprint(rebuilt) != "[srv-c185th5c2rvvnhbfiltg]" {
		t.Fatalf("result %+v, rebuilt %v; want one rebuild of the App's id reported as rolled out", result, rebuilt)
	}
	// The save itself stages like save_only: the rebuild is the one rollout.
	if app := getApp(t, svc.Client, "web"); app.Spec.RestartedAt != "" {
		t.Fatalf("rebuild save also bumped restartedAt to %q", app.Spec.RestartedAt)
	}

	// The editor's retry re-sends an empty rebuild patch: nothing to write, one
	// more rebuild with the same cause.
	result, err = svc.PatchEnvironment(callerCtx, "web", EnvironmentPatch{SaveMode: SaveModeRebuild})
	if err != nil || !result.RolledOut || len(rebuilt) != 2 {
		t.Fatalf("empty rebuild retry = %+v, %v (rebuilt %v), want a second rebuild", result, err, rebuilt)
	}
}

func TestRebuildFailureKeepsTheSaveAndSaysWhy(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"public refusal": {
			fmt.Errorf("%w: billing is suspended for this workspace", core.ErrPaymentRequired),
			fmt.Errorf("%w: billing is suspended for this workspace", core.ErrPaymentRequired).Error(),
		},
		"internal fault": {errors.New("pgx: connection refused at 10.0.0.7"), ""},
	} {
		t.Run(name, func(t *testing.T) {
			store := newFakeSecretStore()
			svc := newService(store, sampleApp("web"))
			svc.RebuildService = func(context.Context, string) (json.RawMessage, error) { return nil, tc.err }

			result, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
				SaveMode: SaveModeRebuild,
				EnvVars:  []EnvVarPatch{{Key: "GREETING", Value: "hello"}},
			})
			if err != nil {
				t.Fatalf("a failed rebuild failed the save: %v", err)
			}
			if result.RolledOut || result.RolloutError != tc.want {
				t.Fatalf("result = %+v, want not rolled out with %q", result, tc.want)
			}
			if store.m[envPath("web")]["GREETING"] != "hello" {
				t.Fatal("the save did not stand")
			}
		})
	}
}

func TestRebuildSaveUnavailableWithoutDeploysWritesNothing(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeRebuild,
		EnvVars:  []EnvVarPatch{{Key: "GREETING", Value: "hello"}},
	})
	if !errors.Is(err, core.ErrDeploysUnavailable) {
		t.Fatalf("err = %v, want ErrDeploysUnavailable", err)
	}
	if _, wrote := store.m[envPath("web")]["GREETING"]; wrote {
		t.Fatal("an unavailable rebuild still wrote the save")
	}
}
