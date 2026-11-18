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
	"slices"
	"testing"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"

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

// deployHistory answers both history reads by status, as the store does:
// ServedRelease's live row and DeployInFlight's open ones.
type deployHistory struct {
	*recordingDeploys
	rows []store.Deploy
}

func (h *deployHistory) ListDeploys(_ context.Context, _ string, filter store.DeployFilter) ([]store.Deploy, error) {
	var out []store.Deploy
	for _, d := range h.rows {
		if len(filter.Statuses) == 0 || slices.Contains(filter.Statuses, d.Status) {
			out = append(out, d)
		}
	}
	return out, nil
}

var servedBuild = store.Deploy{ID: "dep-live", Status: store.DeployLive,
	ResolvedImage: "zot.bex-registry.svc:5000/t/web:gen-2@sha256:4d8c", Commit: "936b61bab203", CommitMessage: "served build"}

// sourceBuiltGroupService is a Git-backed service of serviceType, Auto-Deploy
// on, whose deploy history answers rows.
func sourceBuiltGroupService(serviceType string, rows ...store.Deploy) (*Service, *deployHistory) {
	a := repoApp("web", true)
	a.Labels = managedApp("web").Labels
	a.Spec.Type = serviceType
	history := &deployHistory{recordingDeploys: &recordingDeploys{prior: store.CommitInfo{Hash: "5d946be30"}}, rows: rows}
	svc := newService(newFakeStore(), a)
	svc.Rollout = &rollout.Tracker{Store: history}
	return svc, history
}

// w4/708 (round 1152): a group link, a group value save in "Save and deploy"
// mode, a secret-file write and an unlink each rebuilt a linked Git-backed
// service from source (83–95 s of BuildKit per write), where the same save on
// the service's own Environment tab reruns the serving build (w4/m180). Each
// must open one release that reruns the served build with the new group
// values, recorded with that build's image and commit.
func TestEnvGroupDeploysRerunTheServedBuildOfAGitBackedService(t *testing.T) {
	ctx := testCtx()
	svc, history := sourceBuiltGroupService(appv1alpha1.TypeWebService, servedBuild)
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	check := func(step string, rows int) {
		t.Helper()
		if len(history.rows) != 1 {
			t.Fatalf("history changed: %+v", history.rows)
		}
		if len(history.recordingDeploys.rows) != rows {
			t.Fatalf("%s: %d deploy rows, want %d (one release per write)", step, len(history.recordingDeploys.rows), rows)
		}
		a := getApp(t, svc.Client, "web")
		last := history.recordingDeploys.rows[rows-1]
		ref := a.Spec.ReleaseConfig
		if ref == nil || ref.Image != servedBuild.ResolvedImage || ref.SourceGeneration != 0 || ref.Generation != last.Generation {
			t.Fatalf("%s: releaseConfig %+v, want the served build bound to the release opened (gen %d)", step, ref, last.Generation)
		}
		if last.Image != servedBuild.ResolvedImage || last.Commit != servedBuild.Commit || last.Trigger != store.TriggerConfigChange {
			t.Fatalf("%s: row %+v, want a config_change of the served build %s at %s", step, last, servedBuild.ResolvedImage, servedBuild.Commit)
		}
	}
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	check("link", 1)
	svc.Base.Clock = func() time.Time { return time.Unix(2_000_000, 0).UTC() }
	res, err := svc.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
		EnvVars: []EnvVarPatch{{Key: "MESSAGE", Value: "qa-eg-3", ValueSet: true}}, SaveMode: SaveModeDeploy,
	})
	if err != nil || !res.RolledOut || len(res.FailedServiceIDs) != 0 || len(res.PendingServiceIDs) != 0 {
		t.Fatalf("Save and deploy = %+v, %v", res, err)
	}
	check("Save and deploy", 2)
	svc.Base.Clock = func() time.Time { return time.Unix(3_000_000, 0).UTC() }
	if _, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "MESSAGE", Value: "qa-eg-4"}}); err != nil {
		t.Fatal(err)
	}
	check("variable PUT", 3)
	svc.Base.Clock = func() time.Time { return time.Unix(4_000_000, 0).UTC() }
	if _, err := svc.SetEnvGroupFile(ctx, g.ID, "app.conf", "x=1"); err != nil {
		t.Fatal(err)
	}
	check("secret file write", 4)
	if err := svc.UnlinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	check("unlink", 5)
}

// The group's "Save, rebuild, and deploy" is the one mode that builds: it goes
// through RebuildService and selects nothing.
func TestEnvGroupRebuildStillBuildsAGitBackedService(t *testing.T) {
	ctx := testCtx()
	svc, history := sourceBuiltGroupService(appv1alpha1.TypeWebService, servedBuild)
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", ServiceIDs: []string{"web"}})
	if err != nil {
		t.Fatal(err)
	}
	linked := getApp(t, svc.Client, "web")
	var rebuilt []string
	svc.RebuildService = func(_ context.Context, serviceID string) error {
		rebuilt = append(rebuilt, serviceID)
		return nil
	}
	res, err := svc.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
		EnvVars: []EnvVarPatch{{Key: "MESSAGE", Value: "v2", ValueSet: true}}, SaveMode: SaveModeRebuild,
	})
	if err != nil || !res.RolledOut || !slices.Equal(rebuilt, []string{"srv-web"}) {
		t.Fatalf("rebuild = %+v, %v, rebuilt %v; want one rebuild of web", res, err, rebuilt)
	}
	if a := getApp(t, svc.Client, "web"); !apiequality.Semantic.DeepEqual(a.Spec, linked.Spec) || len(history.recordingDeploys.rows) != 1 {
		t.Fatalf("rebuild mode patched the App itself (releaseConfig %+v) or opened %d rows; want only RebuildService's deploy",
			a.Spec.ReleaseConfig, len(history.recordingDeploys.rows)-1)
	}
}

// What keeps building on a group deploy: a static site (its environment is
// build-time only), a Git-backed service with no live build (nothing to rerun,
// and the group write must still succeed), and one with a deploy in flight (the
// release waits for it, and rerunning the build serving now would roll that
// deploy's code back).
func TestEnvGroupDeploysStillBuildWithNothingToRerun(t *testing.T) {
	inFlight := store.Deploy{ID: "dep-building", Status: store.DeployBuildInProgress, Commit: "5d946be30"}
	for _, tc := range []struct {
		name        string
		serviceType string
		rows        []store.Deploy
	}{
		{name: "a static site", serviceType: appv1alpha1.TypeStaticSite, rows: []store.Deploy{servedBuild}},
		{name: "no live build", serviceType: appv1alpha1.TypeWebService},
		{name: "a deploy in flight", serviceType: appv1alpha1.TypeWebService, rows: []store.Deploy{servedBuild, inFlight}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testCtx()
			svc, history := sourceBuiltGroupService(tc.serviceType, tc.rows...)
			g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
				t.Fatalf("link: %v", err)
			}
			svc.Base.Clock = func() time.Time { return time.Unix(2_000_000, 0).UTC() }
			res, err := svc.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
				EnvVars: []EnvVarPatch{{Key: "MESSAGE", Value: "v2", ValueSet: true}}, SaveMode: SaveModeDeploy,
			})
			if err != nil || !res.RolledOut || len(res.FailedServiceIDs) != 0 {
				t.Fatalf("Save and deploy = %+v, %v", res, err)
			}
			a := getApp(t, svc.Client, "web")
			if a.Spec.ReleaseConfig != nil || a.Spec.RestartedAt == "" {
				t.Fatalf("selected %+v (restartedAt %q); want a plain build", a.Spec.ReleaseConfig, a.Spec.RestartedAt)
			}
			for _, row := range history.recordingDeploys.rows {
				if row.Image != "" {
					t.Fatalf("row %+v records an image; a build records none", row)
				}
			}
			if len(history.recordingDeploys.rows) != 2 {
				t.Fatalf("rows = %d, want the link and the save", len(history.recordingDeploys.rows))
			}
		})
	}
}
