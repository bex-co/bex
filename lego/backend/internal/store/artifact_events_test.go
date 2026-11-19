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

package store

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestObservedImagePullFailureSplitsBuildSourceDeploys(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Generation: 4},
		Status: appv1alpha1.AppStatus{
			Conditions: []metav1.Condition{{
				Type:               appv1alpha1.ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             "ImagePullBackOff",
				Message:            "image pull failed: manifest unknown",
				ObservedGeneration: 4,
			}},
		},
	}

	// Build-source deploy -> artifact_fetch_failed with buildId + message.
	bsDeploy := Deploy{ID: "dep-bs", AppID: "srv-1", Generation: 4, BuildID: "bld-0123456789abcdefghij", Image: "registry.local/bs:1"}
	fact, ok := observedImagePullFailure(bsDeploy, app)
	if !ok {
		t.Fatal("expected artifact_fetch_failed fact")
	}
	if fact.Type != EventFactArtifactFetchFailed || fact.SourceKey != "deploy:dep-bs:artifact_fetch_failed" ||
		fact.BuildID != bsDeploy.BuildID || fact.Message != "image pull failed: manifest unknown" || fact.Image != "" {
		t.Fatalf("build-source pull failure fact = %+v, want artifact_fetch_failed with buildId+message", fact)
	}

	// Empty condition message falls back to DefaultArtifactFetchFailedMessage.
	emptyMsgApp := app.DeepCopy()
	emptyMsgApp.Status.Conditions[0].Message = ""
	fact, ok = observedImagePullFailure(bsDeploy, emptyMsgApp)
	if !ok || fact.Message != DefaultArtifactFetchFailedMessage {
		t.Fatalf("empty condition message fact = %+v (%v), want fallback %q", fact, ok, DefaultArtifactFetchFailedMessage)
	}

	// Regular image deploy -> image_pull_failed unchanged.
	imgDeploy := Deploy{ID: "dep-img", AppID: "srv-1", Generation: 4, Image: "docker.io/library/nginx:missing"}
	fact, ok = observedImagePullFailure(imgDeploy, app)
	if !ok || fact.Type != EventFactImagePullFailed || fact.SourceKey != "deploy:dep-img:image_pull_failed" ||
		fact.BuildID != "" || fact.Image != imgDeploy.Image {
		t.Fatalf("image deploy pull failure fact = %+v (%v), want image_pull_failed", fact, ok)
	}
}

func TestServedReleasePullFailureSplitsBuildSource(t *testing.T) {
	at := time.Date(2026, 11, 19, 2, 0, 0, 0, time.UTC)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Generation: 3},
		Status: appv1alpha1.AppStatus{
			Image:       "registry.local/bs:1",
			ImageDigest: &appv1alpha1.ImageDigestStatus{Generation: 3},
			Conditions: []metav1.Condition{{
				Type:               appv1alpha1.ConditionRollout,
				Status:             metav1.ConditionFalse,
				Reason:             appv1alpha1.ReasonImagePullFailed,
				Message:            "manifest unknown",
				ObservedGeneration: 3,
				LastTransitionTime: metav1.NewTime(at),
			}},
		},
	}

	fact, ok := servedReleasePullFailureWithBuild("srv-1", app, "bld-served")
	if !ok || fact.Type != EventFactArtifactFetchFailed || fact.BuildID != "bld-served" ||
		fact.Message != "manifest unknown" || fact.SourceKey != "release:srv-1:3:artifact_fetch_failed:1795053600" {
		t.Fatalf("served build-source pull failure = %+v (%v)", fact, ok)
	}
	imgFact, ok := servedReleasePullFailure("srv-1", app)
	if !ok || imgFact.Type != EventFactImagePullFailed || imgFact.BuildID != "" {
		t.Fatalf("served image pull failure = %+v (%v)", imgFact, ok)
	}
}

// TestPGArtifactEventsRoundTrip pins w4/m510 against real Postgres:
//   - artifact_source_changed persists BuildSourceFrom/BuildSourceTo on audit_events,
//     reads them back on ListServiceEvents, GetServiceEvent and ListWebhookEvents,
//     and drops pre-migration apps.LinkBuildSource rows with both sides NULL.
//   - artifact_fetch_failed lands once when observedImagePullFailure and
//     TransitionDeploy both fire for a build-source deploy (no image_pull_failed),
//     reads BuildID and Message back on list/get, and leaves image deploys on
//     image_pull_failed.
func TestPGArtifactEventsRoundTrip(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)

	tenant, err := st.CreateWorkspace(ctx, "artifact-events-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})

	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "web", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	// Pre-migration apps.LinkBuildSource row (both sides empty) must be excluded.
	if err := st.Record(ctx, core.AuditEvent{
		Caller: "fixture-owner", Verb: core.AuditVerbLinkBuildSource,
		Resource: core.WorkspaceObject(tenant.ID), Target: core.ServiceTarget(app.ID),
		Outcome: core.AuditAllowed, At: now.Add(-4 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	// Attach, switch, detach rows.
	for _, ev := range []core.AuditEvent{
		{Caller: "fixture-owner", Verb: core.AuditVerbLinkBuildSource, Resource: core.WorkspaceObject(tenant.ID), Target: core.ServiceTarget(app.ID), Outcome: core.AuditAllowed, At: now.Add(-3 * time.Minute), BuildSourceTo: "bds-one"},
		{Caller: "fixture-owner", Verb: core.AuditVerbLinkBuildSource, Resource: core.WorkspaceObject(tenant.ID), Target: core.ServiceTarget(app.ID), Outcome: core.AuditAllowed, At: now.Add(-2 * time.Minute), BuildSourceFrom: "bds-one", BuildSourceTo: "bds-two"},
		{Caller: "fixture-owner", Verb: core.AuditVerbLinkBuildSource, Resource: core.WorkspaceObject(tenant.ID), Target: core.ServiceTarget(app.ID), Outcome: core.AuditAllowed, At: now.Add(-time.Minute), BuildSourceFrom: "bds-two"},
	} {
		if err := st.Record(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}

	// ListServiceEvents returns the 3 new link rows (not the legacy NULL/NULL row).
	linkRows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: []string{core.AuditVerbLinkBuildSource}})
	if err != nil {
		t.Fatal(err)
	}
	if len(linkRows) != 3 {
		t.Fatalf("linkRows = %d, want 3 (legacy NULL/NULL row excluded)", len(linkRows))
	}
	strPtr := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	if strPtr(linkRows[0].BuildSourceFrom) != "bds-two" || linkRows[0].BuildSourceTo != nil {
		t.Errorf("detach row = %v -> %v, want bds-two -> nil", linkRows[0].BuildSourceFrom, linkRows[0].BuildSourceTo)
	}
	if strPtr(linkRows[1].BuildSourceFrom) != "bds-one" || strPtr(linkRows[1].BuildSourceTo) != "bds-two" {
		t.Errorf("relink row = %v -> %v, want bds-one -> bds-two", linkRows[1].BuildSourceFrom, linkRows[1].BuildSourceTo)
	}
	if linkRows[2].BuildSourceFrom != nil || strPtr(linkRows[2].BuildSourceTo) != "bds-one" {
		t.Errorf("attach row = %v -> %v, want nil -> bds-one", linkRows[2].BuildSourceFrom, linkRows[2].BuildSourceTo)
	}
	// GetServiceEvent reads the same sides by evt- id.
	gotLink, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, linkRows[1].Key))
	if err != nil {
		t.Fatal(err)
	}
	if strPtr(gotLink.Event.BuildSourceFrom) != "bds-one" || strPtr(gotLink.Event.BuildSourceTo) != "bds-two" {
		t.Errorf("GetServiceEvent relink = %v -> %v, want bds-one -> bds-two", gotLink.Event.BuildSourceFrom, gotLink.Event.BuildSourceTo)
	}

	// Build-source deploy pull failure: observed fact + TransitionDeploy close -> exactly one artifact_fetch_failed, zero image_pull_failed.
	bsDep, err := st.CreateDeploy(ctx, app.ID, TriggerAPI, "zot/bs:1", 2, CommitInfo{BuildID: "bld-0123456789abcdefghij"}, "fixture-owner", false)
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := st.InsertServiceEventFact(ctx, ServiceEventFact{
		SourceKey:  "deploy:" + bsDep.ID + ":artifact_fetch_failed",
		AppID:      app.ID,
		Type:       EventFactArtifactFetchFailed,
		At:         now,
		DeployID:   bsDep.ID,
		BuildID:    bsDep.BuildID,
		Message:    "manifest unknown for bld-0123456789abcdefghij",
		ReasonCode: EventReasonImagePullBackoff,
	})
	if err != nil || !inserted {
		t.Fatalf("insert observed artifact_fetch_failed: inserted=%v err=%v", inserted, err)
	}
	if ok, err := st.TransitionDeploy(ctx, bsDep.ID, DeployUpdateFailed, RanImage{}, "manifest unknown for bld-0123456789abcdefghij", EventReasonImagePullBackoff, "", nil); err != nil || !ok {
		t.Fatalf("TransitionDeploy bsDep: ok=%v err=%v", ok, err)
	}

	// Build-source deploy that closes via TransitionDeploy directly (no prior observed fact).
	bsDep2, err := st.CreateDeploy(ctx, app.ID, TriggerAPI, "zot/bs:2", 3, CommitInfo{BuildID: "bld-second00000000000000"}, "fixture-owner", false)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.TransitionDeploy(ctx, bsDep2.ID, DeployUpdateFailed, RanImage{}, "pull access denied", EventReasonImagePullBackoff, "", nil); err != nil || !ok {
		t.Fatalf("TransitionDeploy bsDep2: ok=%v err=%v", ok, err)
	}

	// Regular image deploy pull failure still records image_pull_failed.
	imgDep, err := st.CreateDeploy(ctx, app.ID, TriggerAPI, "docker.io/library/nginx:missing", 4, CommitInfo{}, "fixture-owner", false)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.TransitionDeploy(ctx, imgDep.ID, DeployUpdateFailed, RanImage{}, "manifest unknown", EventReasonImagePullBackoff, "", nil); err != nil || !ok {
		t.Fatalf("TransitionDeploy imgDep: ok=%v err=%v", ok, err)
	}

	// Verify fact split and details on list and get.
	artifactRows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{FactTypes: []string{string(EventFactArtifactFetchFailed)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifactRows) != 2 {
		t.Fatalf("artifact_fetch_failed rows = %d, want 2: %+v", len(artifactRows), artifactRows)
	}
	imgRows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{FactTypes: []string{string(EventFactImagePullFailed)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(imgRows) != 1 || imgRows[0].DeployID != imgDep.ID {
		t.Fatalf("image_pull_failed rows = %+v, want only %s", imgRows, imgDep.ID)
	}
	byBuild := map[string]ServiceEventRow{}
	for _, r := range artifactRows {
		byBuild[r.ArtifactBuildID] = r
	}
	if r := byBuild[bsDep.BuildID]; r.ArtifactMessage != "manifest unknown for bld-0123456789abcdefghij" {
		t.Errorf("observed artifact row = %+v", r)
	}
	if r := byBuild[bsDep2.BuildID]; r.ArtifactMessage != "pull access denied" {
		t.Errorf("transition artifact row = %+v", r)
	}
	gotFact, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, byBuild[bsDep.BuildID].Key))
	if err != nil {
		t.Fatal(err)
	}
	if gotFact.Event.ArtifactBuildID != bsDep.BuildID || gotFact.Event.ArtifactMessage != "manifest unknown for bld-0123456789abcdefghij" {
		t.Errorf("GetServiceEvent artifact_fetch_failed = %+v", gotFact.Event)
	}

	// LiveDeployBuildID resolves the build-source deploy's buildId by generation.
	if gotID, err := st.LiveDeployBuildID(ctx, app.ID, 2); err != nil || gotID != bsDep.BuildID {
		t.Errorf("LiveDeployBuildID(gen 2) = %q, %v; want %q", gotID, err, bsDep.BuildID)
	}

	// ListWebhookEvents sees the 3 new LinkBuildSource rows (not the legacy one) and both artifact_fetch_failed facts.
	hooks, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().UTC().Add(time.Hour), []string{core.AuditVerbLinkBuildSource}, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	var linkHooks, fetchHooks int
	for _, h := range hooks {
		if h.Source == EventSourceAudit && h.Verb == core.AuditVerbLinkBuildSource {
			linkHooks++
		}
		if h.Source == EventSourceFact && h.FactType == string(EventFactArtifactFetchFailed) {
			fetchHooks++
		}
	}
	if linkHooks != 3 || fetchHooks != 2 {
		t.Errorf("webhook feed: linkHooks=%d fetchHooks=%d, want 3 and 2", linkHooks, fetchHooks)
	}
}
