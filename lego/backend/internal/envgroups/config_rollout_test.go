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

package envgroups

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// recordingDeploys captures the deploy-history rows an env-group write opens.
type recordingDeploys struct{ rows []store.Deploy }

func (r *recordingDeploys) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, _ store.CommitInfo) (store.Deploy, error) {
	d := store.Deploy{ID: "dep-test", AppID: appID, Trigger: trigger, Image: image, Generation: generation}
	r.rows = append(r.rows, d)
	return d, nil
}

// managedApp is sampleApp with the control-plane labels a store-managed service
// carries — the only kind with deploy history to keep.
func managedApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{
		store.LabelManagedBy:  store.ManagedByValue,
		store.LabelAppID:      "srv-" + name,
		core.LabelServiceName: name,
	}
	return a
}

func trackedService(kv core.SecretKV, rec *recordingDeploys, name string) *Service {
	svc := newService(kv, managedApp(name))
	svc.Rollout = &rollout.Tracker{Store: rec}
	return svc
}

// TestEnvGroupWritesOpenDeployHistory is w6/m51's t008 half: linking a group,
// changing its values, and unlinking it each rewrite the linked service's
// release identity, so the operator rebuilds and redeploys — and each was
// invisible in the Deploys tab before this milestone. Reproduced live on
// qa-20260824-envprec: phase went Running -> Building -> Running and the
// revision bumped rev-1 -> rev-2 with no second deploy row.
func TestEnvGroupWritesOpenDeployHistory(t *testing.T) {
	ctx := context.Background()
	rec := &recordingDeploys{}
	svc := trackedService(newFakeStore(), rec, "web")

	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatalf("CreateEnvGroup: %v", err)
	}
	if _, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "DB_URL", Value: "postgres://x"}}); err != nil {
		t.Fatalf("SetEnvGroupVars: %v", err)
	}
	if len(rec.rows) != 0 {
		t.Fatalf("deploy rows = %d before any service is linked, want none", len(rec.rows))
	}

	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatalf("LinkService: %v", err)
	}
	if len(rec.rows) != 1 {
		t.Fatalf("link opened %d deploy rows, want 1", len(rec.rows))
	}
	if rec.rows[0].Trigger != store.TriggerConfigChange {
		t.Errorf("trigger = %q, want %q", rec.rows[0].Trigger, store.TriggerConfigChange)
	}
	if rec.rows[0].AppID != "srv-web" {
		t.Errorf("appID = %q, want the linked service's row id", rec.rows[0].AppID)
	}

	// Re-linking is idempotent and changes no spec field, so it is not a rollout.
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatalf("re-link: %v", err)
	}
	if len(rec.rows) != 1 {
		t.Fatalf("an idempotent re-link opened %d rows, want it to open none", len(rec.rows)-1)
	}

	// Changing the group's values rolls every linked service.
	svc.Base.Clock = func() time.Time { return time.Unix(2_000_000, 0).UTC() }
	if _, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "DB_URL", Value: "postgres://y"}}); err != nil {
		t.Fatalf("update group vars: %v", err)
	}
	if len(rec.rows) != 2 {
		t.Fatalf("a group-value change opened %d rows total, want 2", len(rec.rows))
	}

	if err := svc.UnlinkService(ctx, g.ID, "web"); err != nil {
		t.Fatalf("UnlinkService: %v", err)
	}
	if len(rec.rows) != 3 {
		t.Fatalf("unlink opened %d rows total, want 3", len(rec.rows))
	}
}

// TestEnvGroupWritesWithoutTrackerStillApply: no control-plane store wired
// (CR-only mode) must degrade to the pre-w6/m51 behavior, never panic.
func TestEnvGroupWritesWithoutTrackerStillApply(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore(), managedApp("web"))
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatalf("CreateEnvGroup: %v", err)
	}
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatalf("LinkService with no rollout tracker: %v", err)
	}
	if got := getApp(t, svc.Client, "web").Spec.RestartedAt; got == "" {
		t.Fatal("the link must still roll the service")
	}
}
