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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// repoApp is a repo-backed service, the only shape where Auto-Deploy is a real
// user-facing toggle (apps/service.go defaults it on for a repo, off for an
// image). autoDeploy=false on one of these is an owner opting out of releases.
func repoApp(name string, autoDeploy bool) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Image = ""
	a.Spec.Repo = "https://github.com/bex-co/" + name
	a.Spec.Branch = "main"
	a.Spec.AutoDeploy = autoDeploy
	return a
}

// restartedAt is the field that actually rolls the pod: the operator treats it
// as release identity, so "no deploy" means this string does not move.
func restartedAt(t *testing.T, svc *Service, name string) string {
	t.Helper()
	return getApp(t, svc.Client, name).Spec.RestartedAt
}

// Render: "If you make changes to an environment group (including deleting
// it), Render kicks off a new deploy for every linked service that has
// autodeploys enabled" (render.com/docs/configure-environment-variables,
// fetched 2026-09-14). Before w2/m94 every Render-shaped group write rolled
// every linked service unconditionally (w1/092): a live probe on 2026-09-14
// saw a config_change deploy open one second after a PUT to a group variable on
// a service whose autoDeploy had just been set to "no", and go live.
func TestGroupWriteSkipsTheDeployOnAnAutoDeployOffService(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore(), repoApp("on", true), repoApp("off", false))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", ServiceIDs: []string{"on", "off"},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	beforeOff := restartedAt(t, svc, "off")

	// PatchEnvironment in deploy mode is the shared tail every Render-shaped
	// group verb funnels through (SetEnvGroupVar(s), DeleteEnvGroupVar,
	// SetEnvGroupFile, DeleteEnvGroupFile, ApplyEnvGroup all call it with
	// SaveModeDeploy), and it is the one surface that can carry a non-Render
	// field — see the m94 README Decisions on why the per-key verbs keep
	// Render's exact response object.
	result, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		EnvVars: []EnvVarPatch{{Key: "MESSAGE", Value: "v2", ValueSet: true}}, SaveMode: SaveModeDeploy,
	})
	if err != nil {
		t.Fatalf("group write: %v", err)
	}

	if !slices.Contains(result.PendingServiceIDs, "off") {
		t.Fatalf("pendingServiceIds = %v, want it to name the auto-deploy-off service", result.PendingServiceIDs)
	}
	if slices.Contains(result.PendingServiceIDs, "on") {
		t.Fatalf("pendingServiceIds = %v, must not name the auto-deploy-on service", result.PendingServiceIDs)
	}
	// Pending is a deliberate skip, not a failure: retrying it would ship the
	// release the owner opted out of.
	if !result.RolledOut || len(result.FailedServiceIDs) != 0 {
		t.Fatalf("a skipped service is not a rollout failure: rolledOut=%v failed=%v", result.RolledOut, result.FailedServiceIDs)
	}
	if !slices.Contains(result.AffectedServiceIDs, "off") {
		t.Fatalf("affectedServiceIds = %v, want the skipped service still reported as affected", result.AffectedServiceIDs)
	}
	if got := restartedAt(t, svc, "off"); got != beforeOff {
		t.Fatalf("auto-deploy-off service rolled: restartedAt %q -> %q", beforeOff, got)
	}
	if restartedAt(t, svc, "on") == "" {
		t.Fatal("auto-deploy-on service should still have been rolled")
	}
	// The value itself is current for both — only the release is deferred.
	value, err := svc.GetEnvGroupVar(ctx, group.ID, "MESSAGE")
	if err != nil || value.Value != "v2" {
		t.Fatalf("group value = %+v, %v; want the write to have landed for every linked service", value, err)
	}

	// The per-key Render verb funnels through the same tail, so it is gated
	// too even though its Render-shaped response cannot report the skip.
	stillOff := restartedAt(t, svc, "off")
	if _, err := svc.SetEnvGroupVar(ctx, group.ID, "MESSAGE", "v3"); err != nil {
		t.Fatalf("set group var: %v", err)
	}
	if got := restartedAt(t, svc, "off"); got != stillOff {
		t.Fatalf("PUT /env-vars/{key} rolled an auto-deploy-off service: %q -> %q", stillOff, got)
	}
	if err := svc.DeleteEnvGroupVar(ctx, group.ID, "MESSAGE"); err != nil {
		t.Fatalf("delete group var: %v", err)
	}
	if got := restartedAt(t, svc, "off"); got != stillOff {
		t.Fatalf("DELETE /env-vars/{key} rolled an auto-deploy-off service: %q -> %q", stillOff, got)
	}
}

// bex defaults spec.autoDeploy to false for an image-backed service because
// there is no branch to watch, not because its owner declined releases.
// Treating that default as an opt-out would strand every image-backed service
// on stale group values forever — a worse bug than the one the gate fixes.
func TestGroupWriteStillDeploysAnImageBackedService(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore(), sampleApp("image"))
	if svc.Client != nil && getApp(t, svc.Client, "image").Spec.AutoDeploy {
		t.Fatal("fixture should be an image-backed service with autoDeploy false")
	}
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", ServiceIDs: []string{"image"},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	result, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		EnvVars: []EnvVarPatch{{Key: "MESSAGE", Value: "v2", ValueSet: true}}, SaveMode: SaveModeDeploy,
	})
	if err != nil {
		t.Fatalf("group write: %v", err)
	}
	if len(result.PendingServiceIDs) != 0 {
		t.Fatalf("image-backed service must not be gated, got pending %v", result.PendingServiceIDs)
	}
	if restartedAt(t, svc, "image") == "" {
		t.Fatal("image-backed service should have been rolled")
	}
}

// Linking is still honored for a gated service — the refs land, so the group's
// values are there for its next deploy — but it does not force a release.
func TestLinkAndUnlinkSkipTheDeployOnAnAutoDeployOffService(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore(), repoApp("off", false))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	before := restartedAt(t, svc, "off")

	if err := svc.LinkService(ctx, group.ID, "off"); err != nil {
		t.Fatalf("link: %v", err)
	}
	linked := getApp(t, svc.Client, "off")
	if !slices.Contains(linked.Spec.EnvFromSecrets, envSecretName(group.ID)) {
		t.Fatalf("link must still attach the group's env Secret: %v", linked.Spec.EnvFromSecrets)
	}
	if !slices.Contains(linked.Spec.FilesFromSecrets, filesSecretName(group.ID)) {
		t.Fatalf("link must still attach the group's files Secret: %v", linked.Spec.FilesFromSecrets)
	}
	if linked.Spec.RestartedAt != before {
		t.Fatalf("link rolled an auto-deploy-off service: restartedAt %q -> %q", before, linked.Spec.RestartedAt)
	}

	if err := svc.UnlinkService(ctx, group.ID, "off"); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	unlinked := getApp(t, svc.Client, "off")
	if slices.Contains(unlinked.Spec.EnvFromSecrets, envSecretName(group.ID)) {
		t.Fatalf("unlink must still detach the group's env Secret: %v", unlinked.Spec.EnvFromSecrets)
	}
	if unlinked.Spec.RestartedAt != before {
		t.Fatalf("unlink rolled an auto-deploy-off service: restartedAt %q -> %q", before, unlinked.Spec.RestartedAt)
	}
}

// Create-with-services links at create time, so it needs the same gate, and its
// result is the group view rather than a patch result.
func TestCreateWithServicesNamesTheAutoDeployOffServices(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore(), repoApp("on", true), repoApp("off", false))
	before := restartedAt(t, svc, "off")

	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", ServiceIDs: []string{"on", "off"},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if !slices.Equal(group.PendingServiceIDs, []string{"off"}) {
		t.Fatalf("create pendingServiceIds = %v, want [off]", group.PendingServiceIDs)
	}
	if got := restartedAt(t, svc, "off"); got != before {
		t.Fatalf("create-with-services rolled an auto-deploy-off service: %q -> %q", before, got)
	}
	if restartedAt(t, svc, "on") == "" {
		t.Fatal("create-with-services should still roll an auto-deploy-on service")
	}
	// A read of the group must not carry the write-only field.
	read, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("get group: %v", err)
	}
	if len(read.PendingServiceIDs) != 0 {
		t.Fatalf("a group READ must not carry pendingServiceIds, got %v", read.PendingServiceIDs)
	}
}

// The explicit save-mode callers (REST /contents, GraphQL, MCP, the dashboard
// editor) are a caller's deliberate choice and stay ungated — the control that
// proves the gate is scoped to Render's implicit fan-out.
func TestExplicitSaveModeIsNotGated(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeStore(), repoApp("off", false))
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", ServiceIDs: []string{"off"},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// SaveModeOnly never rolls anything, gate or no gate.
	result, err := svc.PatchEnvironment(ctx, group.ID, EnvironmentPatch{
		EnvVars: []EnvVarPatch{{Key: "MESSAGE", Value: "v2", ValueSet: true}}, SaveMode: SaveModeOnly,
	})
	if err != nil {
		t.Fatalf("save-only patch: %v", err)
	}
	if len(result.PendingServiceIDs) != 0 {
		t.Fatalf("save-only must not report pending services, got %v", result.PendingServiceIDs)
	}
	_ = result
}
