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
type recordingDeploys struct {
	rows  []store.Deploy
	prior store.CommitInfo
}

func (r *recordingDeploys) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, triggeredBy string, clearCache ...bool) (store.Deploy, error) {
	d := store.Deploy{ID: "dep-test", AppID: appID, Trigger: trigger, Image: image, Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message, TriggeredBy: triggeredBy}
	r.rows = append(r.rows, d)
	return d, nil
}

func (r *recordingDeploys) LatestDeployCommit(_ context.Context, _ string) (store.CommitInfo, error) {
	return r.prior, nil
}

// managedApp is sampleApp with the control-plane labels a store-managed service
// carries — the only kind with deploy history to keep.
func managedApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{
		store.LabelManagedBy:  store.ManagedByValue,
		store.LabelAppID:      "srv-" + name,
		core.LabelServiceName: name,
		core.LabelTenant:      core.DefaultTenant,
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
	ctx := testCtx()
	prior := store.CommitInfo{Hash: "5ef5e18799fa7edbc6477ca3128c78686833b06b", Message: "update"}
	rec := &recordingDeploys{prior: prior}
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
	if rec.rows[0].Commit != prior.Hash {
		t.Errorf("link commit = %q, want prior %q (w4/m100)", rec.rows[0].Commit, prior.Hash)
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
	ctx := testCtx()
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

// liveDeploys answers ServedRelease's live-row read.
type liveDeploys struct {
	*recordingDeploys
	live []store.Deploy
}

func (l *liveDeploys) ListDeploys(_ context.Context, _ string, _ store.DeployFilter) ([]store.Deploy, error) {
	return l.live, nil
}

// w4/m418 (round 400): linking a group, editing its variable or secret file,
// and unlinking it each rolled a linked image service off its imageUrl
// override (busybox:1.37) back to the saved tag (1.36). Each must rerun the image serving; a service
// serving its saved tag selects nothing.
func TestEnvGroupRollsKeepTheServedImage(t *testing.T) {
	const override = "docker.io/library/busybox@sha256:a7dbc319"
	for _, tc := range []struct {
		name string
		live store.Deploy
		want string
	}{
		{name: "override served", live: store.Deploy{Status: store.DeployLive, Image: "docker.io/library/busybox:1.37", ResolvedImage: override}, want: override},
		{name: "saved tag served", live: store.Deploy{Status: store.DeployLive, Image: "web:v1", ResolvedImage: "web@sha256:73aaf090"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testCtx()
			deploys := &liveDeploys{recordingDeploys: &recordingDeploys{}, live: []store.Deploy{tc.live}}
			svc := newService(newFakeStore(), managedApp("web"))
			svc.Rollout = &rollout.Tracker{Store: deploys}
			g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			check := func(step string) {
				t.Helper()
				a := getApp(t, svc.Client, "web")
				ref := a.Spec.ReleaseConfig
				last := deploys.rows[len(deploys.rows)-1]
				if tc.want == "" {
					if ref != nil && ref.Generation == last.Generation {
						t.Fatalf("%s selected %+v, want nothing", step, ref)
					}
					return
				}
				if ref == nil || ref.Image != tc.want || ref.Generation != last.Generation || last.Image != tc.want {
					t.Fatalf("%s: releaseConfig %+v, row %+v; want the served %s for the release opened", step, ref, last, tc.want)
				}
			}
			if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
				t.Fatal(err)
			}
			check("link")
			svc.Base.Clock = func() time.Time { return time.Unix(2_000_000, 0).UTC() }
			if _, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "QA_G", Value: "2"}}); err != nil {
				t.Fatal(err)
			}
			check("variable edit")
			svc.Base.Clock = func() time.Time { return time.Unix(3_000_000, 0).UTC() }
			if _, err := svc.SetEnvGroupFile(ctx, g.ID, "app.conf", "x=1"); err != nil {
				t.Fatal(err)
			}
			check("secret file write")
			if err := svc.UnlinkService(ctx, g.ID, "web"); err != nil {
				t.Fatal(err)
			}
			check("unlink")
			if len(deploys.rows) != 4 {
				t.Fatalf("rows = %d, want one release per write", len(deploys.rows))
			}
		})
	}
}
