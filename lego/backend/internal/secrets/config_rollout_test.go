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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type recordingDeploys struct{ rows []store.Deploy }

func (r *recordingDeploys) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, _ store.CommitInfo) (store.Deploy, error) {
	d := store.Deploy{ID: "dep-test", AppID: appID, Trigger: trigger, Image: image, Generation: generation}
	r.rows = append(r.rows, d)
	return d, nil
}

func managedApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{
		store.LabelManagedBy:  store.ManagedByValue,
		store.LabelAppID:      "srv-" + name,
		core.LabelServiceName: name,
	}
	return a
}

func trackedService(name string) (*Service, *recordingDeploys) {
	rec := &recordingDeploys{}
	svc := newService(newFakeSecretStore(), managedApp(name))
	svc.Rollout = &rollout.Tracker{Store: rec}
	return svc, rec
}

// TestEnvWritesOpenDeployHistory: materializing a changed env map rewrites the
// projection Secret and bumps spec.restartedAt — release identity, so the
// operator rebuilds and rolls new pods. That rollout is a deploy the user must
// be able to see and roll back, not an invisible restart (w6/m51).
func TestEnvWritesOpenDeployHistory(t *testing.T) {
	ctx := context.Background()
	svc, rec := trackedService("web")

	if _, err := svc.SetEnvVars(ctx, "web", []EnvVarView{{Key: "MYVAR", Value: "one"}}); err != nil {
		t.Fatalf("SetEnvVars: %v", err)
	}
	if len(rec.rows) != 1 {
		t.Fatalf("setting env vars opened %d deploy rows, want 1", len(rec.rows))
	}
	if rec.rows[0].Trigger != store.TriggerConfigChange {
		t.Errorf("trigger = %q, want %q", rec.rows[0].Trigger, store.TriggerConfigChange)
	}
	if rec.rows[0].AppID != "srv-web" {
		t.Errorf("appID = %q, want the service's control-plane row id", rec.rows[0].AppID)
	}

	if _, err := svc.SetSecretFile(ctx, "web", "ca.pem", "CERT"); err != nil {
		t.Fatalf("SetSecretFile: %v", err)
	}
	if len(rec.rows) != 2 {
		t.Fatalf("writing a secret file opened %d rows total, want 2", len(rec.rows))
	}
}

// TestSaveOnlyEnvPatchOpensNoDeploy: save_only deliberately stages the changed
// values without rolling, so nothing rebuilt and there is no deploy to show.
// The deploying mode opens exactly one row for the rollout it did request.
func TestSaveOnlyEnvPatchOpensNoDeploy(t *testing.T) {
	ctx := context.Background()
	svc, rec := trackedService("web")

	if _, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{
		EnvVars: []EnvVarPatch{{Key: "STAGED", Value: "later", ValueSet: true}}, SaveMode: SaveModeOnly,
	}); err != nil {
		t.Fatalf("PatchEnvironment(save_only): %v", err)
	}
	if len(rec.rows) != 0 {
		t.Fatalf("save_only opened %d deploy rows, want none — it rolls nothing", len(rec.rows))
	}

	if _, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{
		EnvVars: []EnvVarPatch{{Key: "NOW", Value: "applied", ValueSet: true}}, SaveMode: SaveModeDeploy,
	}); err != nil {
		t.Fatalf("PatchEnvironment(deploy): %v", err)
	}
	if len(rec.rows) != 1 {
		t.Fatalf("a deploying env patch opened %d rows, want 1", len(rec.rows))
	}
}
