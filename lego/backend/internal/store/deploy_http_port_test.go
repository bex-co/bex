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
	"fmt"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestReleaseDetectedHTTPPortMatchesItsImageAndRelease(t *testing.T) {
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1}, Status: appv1alpha1.AppStatus{
		ServingNetwork: &appv1alpha1.ImageNetworkStatus{Image: "old@sha256:a", Revision: "rev-1", PrimaryPort: 80, HTTPSelection: "http"},
		ImageNetwork:   &appv1alpha1.ImageNetworkStatus{Image: "new@sha256:b", Revision: "rev-2", PrimaryPort: 15672, HTTPSelection: "http"},
	}}
	for _, tc := range []struct {
		name       string
		generation int64
		image      string
		want       int32
	}{
		{"old serving", 1, "old@sha256:a", 80},
		{"new candidate", 2, "new@sha256:b", 15672},
		{"candidate cannot narrate older release", 1, "new@sha256:b", 0},
		{"old serving cannot narrate new release", 2, "old@sha256:a", 0},
		{"other image", 2, "unrelated@sha256:c", 0},
		{"unknown release", 0, "new@sha256:b", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := releaseDetectedHTTPPort(app, tc.generation, tc.image); got != tc.want {
				t.Fatalf("port=%d, want%d", got, tc.want)
			}
		})
	}
	for _, selection := range []string{"", "configured", "retained", "fallback"} {
		app.Status.ImageNetwork.HTTPSelection = selection
		if got := releaseDetectedHTTPPort(app, 2, "new@sha256:b"); got != 0 {
			t.Fatalf("selection %q narrated detectedHTTP port%d", selection, got)
		}
	}
	app.Status.ImageNetwork.HTTPSelection = "http"
	app.Spec.Type = appv1alpha1.TypePrivateService
	app.Spec.PortMode = appv1alpha1.PortModeImageV1
	if got := releaseDetectedHTTPPort(app, 2, "new@sha256:b"); got != 0 {
		t.Fatalf("private service narrated HTTP port%d", got)
	}
}

func TestLiveDeployRecordsItsOwnHTTPPort(t *testing.T) {
	ctx := context.Background()
	reconciler, st, cl := newTestReconciler(t)
	tenant, _ := st.CreateTenant(ctx, "http-port", "free")
	row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "rabbitmq:management", Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1, Port: 10000, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	history, _ := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if len(history) != 1 {
		t.Fatalf("create deploys=%v", history)
	}
	first := history[0]
	setStatus := func(deploy Deploy, image string, port int32) {
		t.Helper()
		app := getApp(t, cl)
		app.Status.Phase, app.Status.Image, app.Status.ReleaseGeneration = appv1alpha1.PhaseRunning, image, deploy.Generation
		app.Status.ActiveRevision = fmt.Sprintf("rev-%d", deploy.Generation)
		app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: image, Revision: app.Status.ActiveRevision, PrimaryPort: port, HTTPSelection: "http"}
		app.Status.ImageNetwork = app.Status.ServingNetwork.DeepCopy()
		if err := cl.Status().Update(ctx, app); err != nil {
			t.Fatal(err)
		}
		if err := reconciler.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	setStatus(first, "rabbitmq@sha256:first", 15672)
	first = mustGetDeploy(t, st, row.ID, first.ID)
	if first.Status != DeployLive || first.DetectedHTTPPort != 15672 {
		t.Fatalf("first=%+v", first)
	}
	second, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "nginx:alpine", first.Generation+1, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	setStatus(second, "nginx@sha256:second", 80)
	first = mustGetDeploy(t, st, row.ID, first.ID)
	second = mustGetDeploy(t, st, row.ID, second.ID)
	if first.Status != DeployDeactivated || first.DetectedHTTPPort != 15672 || second.DetectedHTTPPort != 80 {
		t.Fatalf("old/new port narration drifted: first=%+v second=%+v", first, second)
	}
}

func TestPGDetectedHTTPPortSurvivesReplacement(t *testing.T) {
	ctx := context.Background()
	pool, st, appID := startedAtPGApp(t, "http-port")
	first, ok, err := openDeployFor(ctx, st, appID)
	if err != nil || !ok {
		t.Fatalf("open=%v %v", ok, err)
	}
	if first.DetectedHTTPPort != 0 {
		t.Fatal("old rows invent a detected port")
	}
	if changed, err := st.TransitionDeploy(ctx, first.ID, DeployLive, RanImage{Ref: "rabbitmq@sha256:first", DetectedHTTPPort: 15672}, "", "", "", nil); err != nil || !changed {
		t.Fatalf("first live=%v %v", changed, err)
	}
	second, err := st.CreateDeploy(ctx, appID, TriggerAPI, "nginx:alpine", first.Generation+1, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := st.TransitionDeploy(ctx, second.ID, DeployLive, RanImage{Ref: "nginx@sha256:second", DetectedHTTPPort: 80}, "", "", "", nil); err != nil || !changed {
		t.Fatalf("second live=%v %v", changed, err)
	}
	history, err := st.ListDeploys(ctx, appID, DeployFilter{})
	if err != nil || len(history) != 2 {
		t.Fatalf("history=%+v %v", history, err)
	}
	ports := map[string]int32{}
	for _, deploy := range history {
		ports[deploy.ID] = deploy.DetectedHTTPPort
	}
	if ports[first.ID] != 15672 || ports[second.ID] != 80 {
		t.Fatalf("stored ports=%v", ports)
	}
	reloaded := mustGetDeploy(t, st, appID, first.ID)
	if reloaded.Status != DeployDeactivated || reloaded.DetectedHTTPPort != 15672 {
		t.Fatalf("old history lost its port: %+v", reloaded)
	}
	// A terminal retry or a failed candidate cannot rewrite another release's narration.
	if changed, err := st.TransitionDeploy(ctx, first.ID, DeployLive, RanImage{DetectedHTTPPort: 9999}, "", "", "", nil); err != nil || changed {
		t.Fatalf("old terminal rewrite=%v %v", changed, err)
	}
	third, err := st.CreateDeploy(ctx, appID, TriggerAPI, "broken:latest", second.Generation+1, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := st.TransitionDeploy(ctx, third.ID, DeployUpdateFailed, RanImage{DetectedHTTPPort: 9999}, "failed", "", "", nil); err != nil || !changed {
		t.Fatalf("failed close=%v %v", changed, err)
	}
	if failed := mustGetDeploy(t, st, appID, third.ID); failed.DetectedHTTPPort != 0 {
		t.Fatal("failed deploy received detectedHTTP narration")
	}
	if _, err := pool.Exec(ctx, `UPDATE deploys SET detected_http_port=65536 WHERE id=$1`, second.ID); err == nil {
		t.Fatal("DB accepted invalid HTTP port")
	}
}
