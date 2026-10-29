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

// Package apps is the App-lifecycle feature: the list/get read side and the
// restart/suspend/resume write side, projected as Render's "service" shape. The
// Service holds the business logic once; the rest/graphql/mcp files are thin
// registration fragments over it, so the three surfaces cannot drift.
package apps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestImageCompatibilityCreationPolicy(t *testing.T) {
	for _, tc := range []struct {
		name       string
		workspaces map[string]bool
		request    CreateRequest
		want       string
	}{
		{"disabled", nil, CreateRequest{Name: "app", Image: "nginx:1"}, ""},
		{"image", map[string]bool{"*": true}, CreateRequest{Name: "app", Image: "nginx:1"}, appv1alpha1.ContainerPolicyRenderV1},
		{"docker", map[string]bool{"*": true}, CreateRequest{Name: "app", Repo: "https://github.com/example/app", Runtime: "docker"}, appv1alpha1.ContainerPolicyRenderV1},
		{"native", map[string]bool{"*": true}, CreateRequest{Name: "app", Repo: "https://github.com/example/app", Runtime: "go", BuildCommand: "go build .", StartCommand: "./app"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, client := newService(nil)
			service.ImageCompatibilityWorkspaces = tc.workspaces
			if _, err := service.Create(context.Background(), tc.request); err != nil {
				t.Fatal(err)
			}
			if got := getApp(t, client, "app").Spec.ContainerPolicy; got != tc.want {
				t.Fatalf("policy = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPrivateImagePortCreationIntent(t *testing.T) {
	for _, port := range []int32{0, 3000, 9000} {
		service, client := newService(nil)
		service.ImageCompatibilityWorkspaces = map[string]bool{"*": true}
		_, err := service.Create(t.Context(), CreateRequest{
			Name: "clickhouse", Type: appv1alpha1.TypePrivateService,
			Repo: "https://github.com/render-examples/clickhouse", Runtime: "docker", Port: port,
		})
		if err != nil {
			t.Fatal(err)
		}
		want := appv1alpha1.PortModeImageV1
		if port != 0 {
			want = appv1alpha1.PortModeImageConfiguredV1
		}
		if got := getApp(t, client, "clickhouse").Spec.PortMode; got != want {
			t.Fatalf("explicit port %d selected mode %s, want %s", port, got, want)
		}
	}
}

func TestPrivateImagePortRejectsReservedCreationWithoutWrites(t *testing.T) {
	for _, port := range []int32{18012, 18013, 19099} {
		for _, dryRun := range []bool{false, true} {
			// Image ports are a default in every workspace now, so the refusal
			// must not depend on the capability allowlist (w6/m148).
			for _, admitted := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/dryRun=%t/admitted=%t", port, dryRun, admitted), func(t *testing.T) {
					st := &recordingStore{}
					service, cl := newTenantStoreService(fakeWorkspace{"qa": "tea-qa"}, st)
					if admitted {
						service.ImageCompatibilityWorkspaces = map[string]bool{"tea-qa": true}
					}
					ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "qa", Method: "session"})
					_, err := service.Create(ctx, CreateRequest{
						Name: "clickhouse", Type: appv1alpha1.TypePrivateService, DryRun: dryRun,
						Repo: "https://github.com/render-examples/clickhouse", Runtime: "docker", Port: port,
					})
					if !errors.Is(err, core.ErrBadRequest) {
						t.Fatalf("reserved port %d: got %v, want bad request", port, err)
					}
					if len(st.appCreates) != 0 {
						t.Fatal("invalid port created a durable service")
					}
					var apps appv1alpha1.AppList
					if err := cl.List(ctx, &apps); err != nil || len(apps.Items) != 0 {
						t.Fatalf("invalid port created an App: count=%d, err=%v", len(apps.Items), err)
					}
				})
			}
		}
	}
}

func TestBlueprintSyncPreservesContainerPolicy(t *testing.T) {
	for _, policy := range []string{"", appv1alpha1.ContainerPolicyStrictV1, appv1alpha1.ContainerPolicyImageV1, appv1alpha1.ContainerPolicyRenderV1} {
		t.Run(policy, func(t *testing.T) {
			existing := sampleApp("app")
			existing.Spec.ContainerPolicy = policy
			service, client := newService(nil, existing)
			// Enabling the new-creation default cannot migrate existing workloads.
			service.ImageCompatibilityWorkspaces = map[string]bool{"*": true}
			manifest := "services: [{name: app, type: web, env: image, image: {url: 'nginx:1'}}]"
			if _, err := service.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
				t.Fatal(err)
			}
			if got := getApp(t, client, "app").Spec.ContainerPolicy; got != policy {
				t.Fatalf("sync migrated %q to %q", policy, got)
			}
			// Turning the creation gate back off also preserves compatible services.
			service.ImageCompatibilityWorkspaces = nil
			if _, err := service.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
				t.Fatal(err)
			}
			if got := getApp(t, client, "app").Spec.ContainerPolicy; got != policy {
				t.Fatalf("disabled gate migrated %q to %q", policy, got)
			}
		})
	}
}

// A Blueprint resync never routes an existing service through the creation
// defaults: turning image ports on (or off) must not migrate its port mode,
// which would change its published ports and roll it (w6/m148).
func TestBlueprintSyncPreservesPortMode(t *testing.T) {
	manifest := "services: [{name: app, type: pserv, runtime: docker, repo: 'https://github.com/example/app'}]"
	for _, mode := range []string{"", appv1alpha1.PortModeImageV1, appv1alpha1.PortModeImageConfiguredV1} {
		t.Run(mode, func(t *testing.T) {
			existing := sampleApp("app")
			existing.Spec.Type = appv1alpha1.TypePrivateService
			existing.Spec.Repo, existing.Spec.Runtime = "https://github.com/example/app", "docker"
			existing.Spec.PortMode = mode
			service, client := newService(nil, existing)
			for _, disabled := range []bool{false, true} {
				service.PrivateImagePortsDisabled = disabled
				if _, err := service.DeployStack(t.Context(), DeployRequest{Manifest: manifest}); err != nil {
					t.Fatal(err)
				}
				if got := getApp(t, client, "app").Spec.PortMode; got != mode {
					t.Fatalf("sync with ports disabled=%t migrated %q to %q", disabled, mode, got)
				}
			}
		})
	}
}

func TestClickHouseBlueprintCreatesDurablePrivateImagePolicy(t *testing.T) {
	manifest, err := os.ReadFile("testdata/render-clickhouse/render.yaml")
	if err != nil {
		t.Fatal(err)
	}
	st := &recordingStore{disks: map[string]store.Disk{}}
	service, client := newTenantStoreService(fakeWorkspace{"qa": "tea-qa"}, st)
	ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "qa", Method: "session"})
	service.ImageCompatibilityWorkspaces = map[string]bool{"tea-qa": true}
	request := DeployRequest{
		Manifest: string(manifest), Repo: "https://github.com/render-examples/clickhouse", Branch: "master",
	}
	if _, err := service.DeployStack(ctx, request); err != nil {
		t.Fatal(err)
	}
	app := getTenantApp(t, client, "tea-qa", "clickhouse")
	if app.Spec.ContainerPolicy != appv1alpha1.ContainerPolicyRenderV1 || app.Spec.PortMode != appv1alpha1.PortModeImageV1 {
		t.Fatalf("compatibility policy not selected: %+v", app.Spec)
	}
	if app.Spec.Expose || app.Spec.Type != appv1alpha1.TypePrivateService || app.Spec.Replicas != 1 {
		t.Fatalf("private single-instance intent lost: %+v", app.Spec)
	}
	if app.Spec.Disk == nil || app.Spec.Disk.MountPath != "/var/lib/clickhouse" || app.Spec.Disk.SizeGB != 10 {
		t.Fatalf("disk not projected: %+v", app.Spec.Disk)
	}
	if len(st.appCreates) != 1 || st.appCreates[0].ContainerPolicy != app.Spec.ContainerPolicy || st.appCreates[0].PortMode != app.Spec.PortMode {
		t.Fatalf("policies missing from durable creation: %+v", st.appCreates)
	}
	if app.InternalAddress() != "" {
		t.Fatal("unresolved image returned an invented address")
	}
	// An unchanged sync, including after rollback of the creation flag, must
	// keep both persisted policies and avoid starting another release.
	service.ImageCompatibilityWorkspaces = nil
	if _, err := service.DeployStack(ctx, request); err != nil {
		t.Fatal(err)
	}
	after := getTenantApp(t, client, "tea-qa", "clickhouse")
	if !reflect.DeepEqual(app.Spec, after.Spec) || app.ResourceVersion != after.ResourceVersion {
		t.Fatal("unchanged Blueprint sync mutated the service")
	}
	if len(st.appCreates) != 1 || len(st.disks) != 1 || len(st.deployCalls) != 0 {
		t.Fatalf("unchanged sync created resources: apps=%d disks=%d deploys=%d", len(st.appCreates), len(st.disks), len(st.deployCalls))
	}
}

func TestImageCompatibilityIsScopedToListedWorkspaces(t *testing.T) {
	manifest, err := os.ReadFile("testdata/render-clickhouse/render.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// w6/m148 split the gate: image-derived private ports are the creation
	// default in every workspace (rollback: BEX_PRIVATE_IMAGE_PORTS=off), while
	// the container policy (render-v1 since w6/m149) follows the listed
	// workspaces.
	for _, tc := range []struct {
		workspaces    map[string]bool
		portsDisabled bool
		policy        string
		mode          string
	}{
		{map[string]bool{"tea-qa": true}, false, appv1alpha1.ContainerPolicyRenderV1, appv1alpha1.PortModeImageV1},
		// Another workspace's admission grants this one no capabilities.
		{map[string]bool{"tea-other": true}, false, "", appv1alpha1.PortModeImageV1},
		{nil, false, "", appv1alpha1.PortModeImageV1},
		// The rollback switch restores configured ports without touching policy.
		{nil, true, "", ""},
		{map[string]bool{"tea-qa": true}, true, appv1alpha1.ContainerPolicyRenderV1, ""},
	} {
		st := &recordingStore{disks: map[string]store.Disk{}}
		service, client := newTenantStoreService(fakeWorkspace{"qa": "tea-qa"}, st)
		service.ImageCompatibilityWorkspaces = tc.workspaces
		service.PrivateImagePortsDisabled = tc.portsDisabled
		ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "qa", Method: "session"})
		request := DeployRequest{Manifest: string(manifest), Repo: "https://github.com/render-examples/clickhouse", Branch: "master"}
		if _, err := service.DeployStack(ctx, request); err != nil {
			t.Fatal(err)
		}
		app := getTenantApp(t, client, "tea-qa", "clickhouse")
		if app.Spec.ContainerPolicy != tc.policy || app.Spec.PortMode != tc.mode {
			t.Fatalf("admitted %v, ports disabled %t: policy=%q mode=%q, want %q/%q", tc.workspaces, tc.portsDisabled, app.Spec.ContainerPolicy, app.Spec.PortMode, tc.policy, tc.mode)
		}
		if len(st.appCreates) != 1 || st.appCreates[0].ContainerPolicy != tc.policy || st.appCreates[0].PortMode != tc.mode {
			t.Fatalf("durable creation did not record the split decision: %+v", st.appCreates)
		}
	}
}

// A new Docker private service, Git-built (w6/m148) or prebuilt (w6/095),
// takes image ports by default; native builds and other types keep their
// configured port in every workspace.
func TestPrivateImagePortsDefaultForDockerAndPrebuiltPrivateServices(t *testing.T) {
	repo := "https://github.com/example/app"
	for _, tc := range []struct {
		name    string
		request CreateRequest
		want    string
	}{
		{"docker pserv", CreateRequest{Name: "app", Type: appv1alpha1.TypePrivateService, Repo: repo, Runtime: "docker"}, appv1alpha1.PortModeImageV1},
		{"docker pserv explicit port", CreateRequest{Name: "app", Type: appv1alpha1.TypePrivateService, Repo: repo, Runtime: "docker", Port: 8080}, appv1alpha1.PortModeImageConfiguredV1},
		{"docker web", CreateRequest{Name: "app", Repo: repo, Runtime: "docker"}, appv1alpha1.PortModeWebImageV1},
		{"docker worker", CreateRequest{Name: "app", Type: appv1alpha1.TypeBackgroundWorker, Repo: repo, Runtime: "docker"}, ""},
		{"native pserv", CreateRequest{Name: "app", Type: appv1alpha1.TypePrivateService, Repo: repo, Runtime: "go", BuildCommand: "go build .", StartCommand: "./app"}, ""},
		{"prebuilt pserv", CreateRequest{Name: "app", Type: appv1alpha1.TypePrivateService, Image: "clickhouse/clickhouse-server:23.8"}, appv1alpha1.PortModeImageV1},
		{"prebuilt pserv explicit port", CreateRequest{Name: "app", Type: appv1alpha1.TypePrivateService, Image: "clickhouse/clickhouse-server:23.8", Port: 8123}, appv1alpha1.PortModeImageConfiguredV1},
		{"prebuilt worker", CreateRequest{Name: "app", Type: appv1alpha1.TypeBackgroundWorker, Image: "nginx:1"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, client := newService(nil)
			if _, err := service.Create(t.Context(), tc.request); err != nil {
				t.Fatal(err)
			}
			app := getApp(t, client, "app")
			if app.Spec.PortMode != tc.want || app.Spec.ContainerPolicy != "" {
				t.Fatalf("mode=%q policy=%q, want mode %q and no policy", app.Spec.PortMode, app.Spec.ContainerPolicy, tc.want)
			}
		})
	}
}
