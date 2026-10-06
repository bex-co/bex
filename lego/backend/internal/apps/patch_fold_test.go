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

package apps

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestServicePatchFoldsMatchTheirVerbs (w5/m116): each row's check folds into
// the preflight's probe exactly the spec change its verb then writes. The probe
// is what admission judges before the patch's first write and what a dry-run
// answers with, so a fold that drifted from its verb would let either one lie.
// One single-row patch per case, on a service that row applies to: the probe's
// spec must equal the spec the real patch leaves, restartedAt aside (the verbs
// bump it to roll the change out).
func TestServicePatchFoldsMatchTheirVerbs(t *testing.T) {
	str := func(s string) *string { return &s }
	i32 := func(v int32) *int32 { return &v }
	repo := func() *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Labels = nil                   // storeless: every verb writes the CR directly
		a.Spec.CloneSecret = "web-clone" // scoped to this repo; a repo change clears it
		return a
	}
	docker := func() *appv1alpha1.App {
		a := repo()
		a.Spec.Runtime = "docker"
		return a
	}
	static := func() *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/site.git", Branch: "main"},
		}
	}
	cron := func() *appv1alpha1.App { return cronApp("web") }
	cases := []struct {
		name  string
		app   func() *appv1alpha1.App
		patch ServicePatch
	}{
		{"display name", repo, ServicePatch{DisplayName: str(" Storefront ")}},
		{"source", repo, ServicePatch{Repo: str("https://github.com/bex-co/other.git")}},
		{"plan", repo, ServicePatch{Plan: str("standard")}},
		{"idle timeout", repo, ServicePatch{IdleTTLSeconds: i32(900)}},
		{"shutdown delay", repo, ServicePatch{MaxShutdownDelaySeconds: i32(45)}},
		{"root dir", repo, ServicePatch{RootDir: str("services/api")}},
		{"build filter", repo, ServicePatch{BuildFilter: &BuildFilterView{Paths: []string{"src/**"}}}},
		{"auto-deploy", repo, ServicePatch{AutoDeploy: new(bool)}},
		{"health check", repo, ServicePatch{HealthCheckPath: str(" /healthz ")}},
		{"pre-deploy command", repo, ServicePatch{PreDeployCommand: str(" ./migrate ")}},
		{"commands", repo, ServicePatch{BuildCommand: str(" make "), StartCommand: str(" ./serve ")}},
		{"dockerfile path", docker, ServicePatch{DockerfilePath: str("docker/Dockerfile")}},
		{"port", repo, ServicePatch{Port: i32(9090)}},
		{"notify on fail", repo, ServicePatch{NotifyOnFail: str("ignore")}},
		{"notifications to send", repo, ServicePatch{NotificationsToSend: str("none")}},
		{"subdomain policy", repo, ServicePatch{RenderSubdomainPolicy: str("enabled")}},
		{"ip allow list", repo, ServicePatch{IPAllowList: &[]core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}}}},
		{"maintenance mode", repo, ServicePatch{MaintenanceMode: &MaintenanceModeView{Enabled: true}}},
		{"autoscaling", repo, ServicePatch{Autoscaling: &SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3, TargetCPUPercent: i32(70)}}},
		{"cron schedule and command", cron, ServicePatch{Schedule: str(" 0 * * * * "), Command: str(" ./job ")}},
		{"publish path", static, ServicePatch{PublishPath: str(" dist ")}},
	}
	covered := map[string]bool{}
	for _, tc := range cases {
		for _, row := range presentServicePatchRows(tc.patch) {
			for _, f := range row.fields {
				covered[f] = true
			}
		}
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil, tc.app())
			ctx := context.Background()
			_, probe, err := svc.preflightServicePatch(withRequestMemo(ctx), "web", tc.patch, true)
			if err != nil {
				t.Fatalf("dry-run preflight: %v", err)
			}
			if _, err := svc.ApplyServicePatch(ctx, "web", tc.patch); err != nil {
				t.Fatalf("real patch: %v", err)
			}
			got, want := getApp(t, cl, "web").Spec, probe.Spec
			got.RestartedAt, want.RestartedAt = "", ""
			if !reflect.DeepEqual(got, want) {
				g, _ := json.Marshal(got)
				w, _ := json.Marshal(want)
				t.Fatalf("the real patch wrote\n  %s\nthe probe folded\n  %s", g, w)
			}
		})
	}
	for _, row := range servicePatchTable {
		for _, f := range row.fields {
			// Rows whose fields ride along with another's (the image's owner,
			// the reorder flag) are covered by their trigger field.
			if !covered[f] && f != "ImageOwnerID" && f != "Image" && f != "Branch" && f != "RegistryCredentialID" && f != "MaintenanceBeforeFreeDowngrade" {
				t.Errorf("no case patches %s: its fold is unchecked", f)
			}
		}
	}
}
