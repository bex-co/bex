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
		name    string
		workspaces map[string]bool
		request    CreateRequest
		want       string
	}{
		{"disabled", nil, CreateRequest{Name: "app", Image: "nginx:1"}, ""},
		{"image", map[string]bool{"*": true}, CreateRequest{Name: "app", Image: "nginx:1"}, appv1alpha1.ContainerPolicyImageV1},
		{"docker", map[string]bool{"*": true}, CreateRequest{Name: "app", Repo: "https://github.com/example/app", Runtime: "docker"}, appv1alpha1.ContainerPolicyImageV1},
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
			t.Run(fmt.Sprintf("%d/dryRun=%t", port, dryRun), func(t *testing.T) {
				st := &recordingStore{}
				service, cl := newTenantStoreService(fakeWorkspace{"qa": "tea-qa"}, st)
				service.ImageCompatibilityWorkspaces = map[string]bool{"tea-qa": true}
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

func TestBlueprintSyncPreservesContainerPolicy(t *testing.T) {
	for _, policy := range []string{"", appv1alpha1.ContainerPolicyStrictV1, appv1alpha1.ContainerPolicyImageV1} {
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
	if app.Spec.ContainerPolicy != appv1alpha1.ContainerPolicyImageV1 || app.Spec.PortMode != appv1alpha1.PortModeImageV1 {
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
	for _, tc := range []struct {
		workspaces map[string]bool
		policy     string
		mode       string
	}{
		{map[string]bool{"tea-qa": true}, appv1alpha1.ContainerPolicyImageV1, appv1alpha1.PortModeImageV1},
		// Another workspace's admission grants this one nothing.
		{map[string]bool{"tea-other": true}, "", ""},
		{nil, "", ""},
	} {
		st := &recordingStore{disks: map[string]store.Disk{}}
		service, client := newTenantStoreService(fakeWorkspace{"qa": "tea-qa"}, st)
		service.ImageCompatibilityWorkspaces = tc.workspaces
		ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "qa", Method: "session"})
		request := DeployRequest{Manifest: string(manifest), Repo: "https://github.com/render-examples/clickhouse", Branch: "master"}
		if _, err := service.DeployStack(ctx, request); err != nil {
			t.Fatal(err)
		}
		app := getTenantApp(t, client, "tea-qa", "clickhouse")
		if app.Spec.ContainerPolicy != tc.policy || app.Spec.PortMode != tc.mode {
			t.Fatalf("admitted %v: policy=%q mode=%q, want %q/%q", tc.workspaces, app.Spec.ContainerPolicy, app.Spec.PortMode, tc.policy, tc.mode)
		}
	}
}
