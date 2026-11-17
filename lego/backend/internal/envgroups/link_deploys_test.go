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
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render: "That's it! Render kicks off a new deploy of your service to
// incorporate the values from the linked environment group"
// (render.com/docs/configure-environment-variables § Linking a group to a
// service, read 2026-11-16). Render names no Auto-Deploy condition for a link;
// only a change to the group, "including deleting it", waits for a service
// with autodeploys off. w4/m239 had read the modify rule as the link rule, so
// bex held a link on an Auto-Deploy-off service until its owner deployed
// (w32/m423).

// trackedRepoApp is repoApp with the control-plane labels that give a service
// deploy history, so a test can count the deploy rows a write opens.
func trackedRepoApp(name string, autoDeploy bool) *appv1alpha1.App {
	a := repoApp(name, autoDeploy)
	a.Labels = managedApp(name).Labels
	return a
}

// trackedStaticSite is a store-managed repo-backed static site.
func trackedStaticSite(name string, autoDeploy bool) *appv1alpha1.App {
	a := trackedRepoApp(name, autoDeploy)
	a.Spec.Type = appv1alpha1.TypeStaticSite
	a.Spec.PublishPath = "dist"
	return a
}

// recordedService is a service over apps whose rollouts land in rec.
func recordedService(rec *recordingDeploys, apps ...*appv1alpha1.App) *Service {
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	svc := newService(newFakeStore(), objs...)
	svc.Rollout = &rollout.Tracker{Store: rec}
	return svc
}

// wantRunsGroup fails unless the service's running spec mounts gid, with
// nothing left staged for it.
func wantRunsGroup(t *testing.T, a *appv1alpha1.App, gid string) {
	t.Helper()
	if !slices.Contains(a.Spec.EnvFromSecrets, envSecretName(gid)) || !slices.Contains(a.Spec.FilesFromSecrets, filesSecretName(gid)) {
		t.Fatalf("spec does not mount group %s: env %v files %v", gid, a.Spec.EnvFromSecrets, a.Spec.FilesFromSecrets)
	}
	if staged := a.Annotations[appv1alpha1.AnnotationPendingGroupLinks]; staged != "" {
		t.Fatalf("the link is still staged for the next deploy: %s", staged)
	}
}

// An owner's link deploys a repo-backed web service and static site whose
// Auto-Deploy is off: the refs and spec.restartedAt move in one recorded
// rollout. The unlink that follows still waits for the owner's next deploy.
func TestAnOwnersLinkDeploysAnAutoDeployOffService(t *testing.T) {
	for name, app := range map[string]*appv1alpha1.App{
		"web service": trackedRepoApp("off", false),
		"static site": trackedStaticSite("off", false),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := testCtx()
			rec := &recordingDeploys{}
			svc := recordedService(rec, app)
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "MESSAGE", Value: "grp-v1", ValueSet: true}},
			})
			if err != nil {
				t.Fatalf("create group: %v", err)
			}

			if err := svc.LinkService(ctx, group.ID, "off"); err != nil {
				t.Fatalf("link: %v", err)
			}
			linked := getApp(t, svc.Client, "off")
			wantRunsGroup(t, linked, group.ID)
			if linked.Spec.RestartedAt == "" {
				t.Fatal("the link did not roll the service: spec.restartedAt unset")
			}
			if len(rec.rows) != 1 || rec.rows[0].Trigger != store.TriggerConfigChange || rec.rows[0].AppID != "srv-off" {
				t.Fatalf("link opened rows %+v, want one config_change row for srv-off", rec.rows)
			}
			if got := linked.Annotations[appv1alpha1.AnnotationReleaseGeneration]; got != strconv.FormatInt(rec.rows[0].Generation, 10) {
				t.Fatalf("release generation %q, row generation %d: the row must name the release the operator adopts", got, rec.rows[0].Generation)
			}

			// Unlink is not Render's link rule; it keeps the Auto-Deploy gate.
			if err := svc.UnlinkService(ctx, group.ID, "off"); err != nil {
				t.Fatalf("unlink: %v", err)
			}
			unlinked := getApp(t, svc.Client, "off")
			if len(rec.rows) != 1 {
				t.Fatalf("unlink opened %d more deploy rows, want none", len(rec.rows)-1)
			}
			if unlinked.Spec.RestartedAt != linked.Spec.RestartedAt || !slices.Contains(unlinked.Spec.EnvFromSecrets, envSecretName(group.ID)) {
				t.Fatalf("unlink changed the running release: %+v", unlinked.Spec)
			}
			if mountsGroup(unlinked, group.ID) {
				t.Fatal("a staged unlink must read as unmounted")
			}
		})
	}
}

// A group delete is Render's modify rule ("including deleting it"): a linked
// Auto-Deploy-off service keeps serving its release, and the next deploy
// drops the group.
func TestGroupDeleteStillWaitsOnAnAutoDeployOffService(t *testing.T) {
	ctx := testCtx()
	rec := &recordingDeploys{}
	svc := recordedService(rec, trackedRepoApp("off", false), trackedRepoApp("on", true))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", ServiceIDs: []string{"off", "on"}})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if len(rec.rows) != 2 {
		t.Fatalf("create with both services opened %d rows, want one each", len(rec.rows))
	}
	before := getApp(t, svc.Client, "off")

	if err := svc.DeleteEnvGroup(ctx, group.ID); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	if len(rec.rows) != 3 || rec.rows[2].AppID != "srv-on" {
		t.Fatalf("delete opened rows %+v, want one more, for the Auto-Deploy-on service only", rec.rows[2:])
	}
	after := getApp(t, svc.Client, "off")
	if after.Spec.RestartedAt != before.Spec.RestartedAt || mountsGroup(after, group.ID) {
		t.Fatalf("delete rolled the Auto-Deploy-off service or left it mounted: %+v %v", after.Spec, after.Annotations)
	}
	rollout.PrepareGroupRefsForRelease(after)
	if len(after.Spec.EnvFromSecrets)+len(after.Spec.FilesFromSecrets) != 0 {
		t.Fatalf("the next release must drop the deleted group: %+v", after.Spec)
	}
}

// An owner who unlinks a group from an Auto-Deploy-off service and then links
// it again gets the group, deployed: the staged unlink must not ride the
// link's release and drop the group it just linked.
func TestRelinkingAfterAStagedUnlinkKeepsTheGroup(t *testing.T) {
	ctx := testCtx()
	rec := &recordingDeploys{}
	svc := recordedService(rec, trackedRepoApp("off", false))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", ServiceIDs: []string{"off"}})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := svc.UnlinkService(ctx, group.ID, "off"); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if staged := getApp(t, svc.Client, "off").Annotations[appv1alpha1.AnnotationPendingGroupLinks]; staged == "" {
		t.Fatal("fixture: the unlink should be staged")
	}

	svc.Base.Clock = func() time.Time { return time.Unix(2_000_000, 0).UTC() } // a later request
	if err := svc.LinkService(ctx, group.ID, "off"); err != nil {
		t.Fatalf("relink: %v", err)
	}
	wantRunsGroup(t, getApp(t, svc.Client, "off"), group.ID)
	if len(rec.rows) != 2 {
		t.Fatalf("deploy rows = %d, want the create's and the relink's", len(rec.rows))
	}
}

// Another group's staged change still rides the link's release, as it rides
// any release the owner starts.
func TestALinksReleaseCarriesOtherStagedGroupChanges(t *testing.T) {
	ctx := testCtx()
	svc := recordedService(&recordingDeploys{}, trackedRepoApp("off", false))
	older, newer := twoGroups(t, svc)
	if err := svc.LinkEnvGroup(ctx, "a", "off"); err != nil { // a Blueprint sync's link waits
		t.Fatalf("sync link: %v", err)
	}
	if err := svc.LinkService(ctx, newer, "off"); err != nil {
		t.Fatalf("link: %v", err)
	}
	a := getApp(t, svc.Client, "off")
	wantCreationOrder(t, a.Spec.EnvFromSecrets, a.Spec.FilesFromSecrets, older, newer)
	if staged := a.Annotations[appv1alpha1.AnnotationPendingGroupLinks]; staged != "" {
		t.Fatalf("the release left a change staged: %s", staged)
	}
}

// A link a Blueprint sync makes is not one Render documents a deploy for, so
// it keeps the Auto-Deploy gate (w4/m239).
func TestABlueprintSyncLinkStillWaitsOnAnAutoDeployOffService(t *testing.T) {
	ctx := testCtx()
	rec := &recordingDeploys{}
	svc := recordedService(rec, trackedRepoApp("off", false))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	before := getApp(t, svc.Client, "off")

	if err := svc.LinkEnvGroup(ctx, "shared", "off"); err != nil {
		t.Fatalf("sync link: %v", err)
	}
	after := getApp(t, svc.Client, "off")
	if len(rec.rows) != 0 || after.Spec.RestartedAt != before.Spec.RestartedAt || len(after.Spec.EnvFromSecrets) != 0 {
		t.Fatalf("a sync link deployed: rows %d, spec %+v", len(rec.rows), after.Spec)
	}
	if !mountsGroup(after, group.ID) {
		t.Fatal("the sync link must stay staged for the next deploy")
	}
}

// A suspended service deploys nothing (w4/m234): an owner's link is held for
// its resume and next deploy, whatever its Auto-Deploy setting, and a create
// names it pending.
func TestALinkOnASuspendedServiceWaitsAndIsReportedPending(t *testing.T) {
	for name, app := range map[string]*appv1alpha1.App{
		"auto-deploy off": trackedRepoApp("asleep", false),
		"auto-deploy on":  trackedRepoApp("asleep", true),
		"image-backed":    managedApp("asleep"),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := testCtx()
			app.Spec.Suspended = true
			rec := &recordingDeploys{}
			svc := recordedService(rec, app, trackedRepoApp("awake", false))

			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", ServiceIDs: []string{"asleep", "awake"}})
			if err != nil {
				t.Fatalf("create group: %v", err)
			}
			if !slices.Equal(group.PendingServiceIDs, []string{"srv-asleep"}) {
				t.Fatalf("pendingServiceIds = %v, want only the suspended service", group.PendingServiceIDs)
			}
			if len(rec.rows) != 1 || rec.rows[0].AppID != "srv-awake" {
				t.Fatalf("rows %+v, want one, for the running service only", rec.rows)
			}
			asleep := getApp(t, svc.Client, "asleep")
			if asleep.Annotations[appv1alpha1.AnnotationReleaseGeneration] != "" {
				t.Fatalf("the suspended service's link was stamped as a release: %v", asleep.Annotations)
			}
			if !mountsGroup(asleep, group.ID) {
				t.Fatal("the held link must still read as mounted")
			}
		})
	}
}

// A link to a build source waits for its next build (w2/m254), on the owner's
// link path too.
func TestALinkToABuildSourceWaits(t *testing.T) {
	carrier := trackedRepoApp("bds", true)
	carrier.Spec.Type = appv1alpha1.TypeBuildSource
	rec := &recordingDeploys{}
	svc := recordedService(rec, carrier)
	before := getApp(t, svc.Client, "bds")

	pending, err := svc.rollLinked(testCtx(), before.DeepCopy(), "stamp", "evg-1", ownerLink)
	if err != nil || !pending {
		t.Fatalf("rollLinked = (%v, %v), want the link to wait", pending, err)
	}
	after := getApp(t, svc.Client, "bds")
	if len(rec.rows) != 0 || after.Spec.RestartedAt != before.Spec.RestartedAt || len(after.Spec.EnvFromSecrets) != 0 {
		t.Fatalf("a build source link rolled: rows %d, spec %+v", len(rec.rows), after.Spec)
	}
}

// The rule as a table: which link writes wait for the service's next deploy.
func TestWhichGroupLinkWritesWait(t *testing.T) {
	buildSource := appv1alpha1.AppSpec{Type: appv1alpha1.TypeBuildSource, Repo: "r", AutoDeploy: true}
	off := appv1alpha1.AppSpec{Repo: "r"}
	on := appv1alpha1.AppSpec{Repo: "r", AutoDeploy: true}
	image := appv1alpha1.AppSpec{Image: "nginx"}
	for _, tc := range []struct {
		name  string
		write groupLinkWrite
		spec  appv1alpha1.AppSpec
		waits bool
	}{
		{"owner link, auto-deploy off", ownerLink, off, false},
		{"owner link, auto-deploy on", ownerLink, on, false},
		{"owner link, image-backed", ownerLink, image, false},
		{"owner link, build source", ownerLink, buildSource, true},
		{"sync link, auto-deploy off", syncLink, off, true},
		{"sync link, auto-deploy on", syncLink, on, false},
		{"sync link, image-backed", syncLink, image, false},
		{"sync link, build source", syncLink, buildSource, true},
		{"unlink, auto-deploy off", unlinkGroup, off, true},
		{"unlink, auto-deploy on", unlinkGroup, on, false},
		{"unlink, image-backed", unlinkGroup, image, false},
		{"unlink, build source", unlinkGroup, buildSource, true},
	} {
		if got := tc.write.waits(&appv1alpha1.App{Spec: tc.spec}); got != tc.waits {
			t.Errorf("%s: waits = %v, want %v", tc.name, got, tc.waits)
		}
	}
}

// Every surface shares the one link path: REST's link and create, GraphQL's
// link mutation and MCP's link and create tools each deploy an Auto-Deploy-off
// service once, and no create names it pending.
func TestEveryLinkSurfaceDeploysAnAutoDeployOffService(t *testing.T) {
	for _, surface := range []struct {
		name string
		link func(t *testing.T, svc *Service, gid string) (pending []string)
	}{
		{"REST link", func(t *testing.T, svc *Service, gid string) []string {
			if rec := serveREST(svc, http.MethodPost, "/v1/env-groups/"+gid+"/services/off", ""); rec.Code != http.StatusOK {
				t.Fatalf("POST link = %d %s", rec.Code, rec.Body)
			}
			return nil
		}},
		{"REST create", func(t *testing.T, svc *Service, _ string) []string {
			rec := serveREST(svc, http.MethodPost, "/v1/env-groups", `{"name":"created","serviceIds":["off"]}`)
			if rec.Code != http.StatusCreated {
				t.Fatalf("POST create = %d %s", rec.Code, rec.Body)
			}
			var body struct {
				PendingServiceIDs []string `json:"pendingServiceIds"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			return body.PendingServiceIDs
		}},
		{"GraphQL link", func(t *testing.T, svc *Service, gid string) []string {
			res := graphql.Do(graphql.Params{
				Schema: *envGroupSchema(t, svc), Context: testCtx(),
				RequestString: `mutation { linkEnvGroup(id: "` + gid + `", serviceId: "off") }`,
			})
			if len(res.Errors) > 0 {
				t.Fatalf("linkEnvGroup: %v", res.Errors)
			}
			return nil
		}},
		{"MCP link", func(t *testing.T, svc *Service, gid string) []string {
			callTool(t, mcpSession(t, svc), "link_env_group", map[string]any{"id": gid, "serviceId": "off"}, nil)
			return nil
		}},
		{"MCP create", func(t *testing.T, svc *Service, _ string) []string {
			var created EnvGroupView
			callTool(t, mcpSession(t, svc), "create_env_group", map[string]any{"name": "created", "serviceIds": []string{"off"}}, &created)
			return created.PendingServiceIDs
		}},
	} {
		t.Run(surface.name, func(t *testing.T) {
			rec := &recordingDeploys{}
			svc := recordedService(rec, trackedRepoApp("off", false))
			group, err := svc.CreateEnvGroup(testCtx(), CreateEnvGroupRequest{Name: "shared"})
			if err != nil {
				t.Fatalf("create group: %v", err)
			}
			if pending := surface.link(t, svc, group.ID); len(pending) != 0 {
				t.Fatalf("pendingServiceIds = %v, want none", pending)
			}
			if len(rec.rows) != 1 || rec.rows[0].AppID != "srv-off" {
				t.Fatalf("rows %+v, want one deploy of srv-off", rec.rows)
			}
			a := getApp(t, svc.Client, "off")
			if a.Annotations[appv1alpha1.AnnotationPendingGroupLinks] != "" || len(a.Spec.EnvFromSecrets) != 1 {
				t.Fatalf("the link was staged instead of deployed: %+v %v", a.Spec, a.Annotations)
			}
		})
	}
}
