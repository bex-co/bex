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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"k8s.io/utils/ptr"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/732: a disk stops at 1 TB on Render. Its dashboard caps both disk forms at
// 1000 GB and maps its server's "disk size cannot exceed 1TB" refusal, and its
// Terraform provider refuses size_gb above 1000; larger disks are a support
// request. bex had accepted up to Hetzner's 10 000 GB on every surface.

const diskCapRefusal = "disk size cannot exceed 1TB (1000GB)"

func restDiskCall(t *testing.T, svc *Service, method, path, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	validated(t, diskRESTMux(t, svc)).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	var out struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out.Message
}

func gqlDiskError(t *testing.T, svc *Service, mutation string) string {
	t.Helper()
	res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: context.Background(), RequestString: mutation})
	if len(res.Errors) != 1 {
		t.Fatalf("%s: errors = %v, want one refusal", mutation, res.Errors)
	}
	return res.Errors[0].Message
}

// Adding a disk above 1 TB is refused on REST, GraphQL and MCP in Render's
// words, before any row or spec is written; 1000 GB itself is accepted.
func TestAddingADiskAboveOneTBIsRefusedOnEverySurface(t *testing.T) {
	refusals := map[string]func(*Service) string{
		"REST": func(svc *Service) string {
			code, msg := restDiskCall(t, svc, "POST", "/v1/disks", `{"serviceId":"web","name":"data","mountPath":"/var/data","sizeGB":1001}`)
			if code != http.StatusBadRequest {
				t.Fatalf("POST /v1/disks sizeGB 1001 => %d %s, want 400", code, msg)
			}
			return msg
		},
		"GraphQL": func(svc *Service) string {
			return gqlDiskError(t, svc, `mutation { addDisk(serviceId: "web", name: "data", mountPath: "/var/data", sizeGB: 1001) { id } }`)
		},
		"MCP": func(svc *Service) string {
			return callAppsMCPError(t, svc, "add_disk", map[string]any{"serviceId": "web", "name": "data", "mountPath": "/var/data", "sizeGB": 1001})
		},
	}
	for surface, refuse := range refusals {
		t.Run(surface, func(t *testing.T) {
			svc, cl, st := newDiskService(diskEligibleApp("web"))
			if msg := refuse(svc); !strings.Contains(msg, diskCapRefusal) {
				t.Fatalf("refusal = %q, want %q", msg, diskCapRefusal)
			}
			if len(st.disks) != 0 || getApp(t, cl, "web").Spec.Disk != nil {
				t.Fatal("a refused 1001 GB disk still wrote a row or spec.disk")
			}
		})
	}

	svc, _, _ := newDiskService(diskEligibleApp("web"))
	if code, msg := restDiskCall(t, svc, "POST", "/v1/disks", `{"serviceId":"web","name":"data","mountPath":"/var/data","sizeGB":1000}`); code != http.StatusCreated {
		t.Fatalf("POST /v1/disks sizeGB 1000 => %d %s, want 201", code, msg)
	}
}

// A service created with a disk above 1 TB is refused on REST and GraphQL with
// the same sentence, and nothing is created.
func TestCreatingAServiceWithADiskAboveOneTBIsRefused(t *testing.T) {
	restSvc, restStore, restCtx := newDiskStoreService()
	rec := httptest.NewRecorder()
	validated(t, diskRESTMux(t, restSvc)).ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(
		`{"name":"web","type":"web_service","repo":"https://github.com/o/r","serviceDetails":{"plan":"starter","disk":{"name":"data","mountPath":"/var/data","sizeGB":1001}}}`,
	)).WithContext(restCtx))
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusBadRequest || !strings.Contains(body.Message, diskCapRefusal) {
		t.Fatalf("REST create with a 1001 GB disk => %d %s, want 400 %q", rec.Code, rec.Body, diskCapRefusal)
	}

	gqlSvc, gqlStore, gqlCtx := newDiskStoreService()
	res := graphql.Do(graphql.Params{Schema: mustSchema(t, gqlSvc), Context: gqlCtx, RequestString: `mutation {
		createService(name: "web", type: "web_service", repo: "https://github.com/o/r", plan: "starter",
			disk: {name: "data", mountPath: "/var/data", sizeGB: 1001}) { id } }`})
	if len(res.Errors) != 1 || res.Errors[0].Message != body.Message {
		t.Fatalf("GraphQL create errors = %v, want REST's %q", res.Errors, body.Message)
	}
	if len(restStore.appCreates)+len(gqlStore.appCreates)+len(restStore.disks)+len(gqlStore.disks) != 0 {
		t.Fatal("a refused create wrote a service or a disk")
	}
}

// Growing a disk past 1 TB is refused on every surface; growing it to 1000 GB
// is accepted.
func TestGrowingADiskPastOneTBIsRefusedOnEverySurface(t *testing.T) {
	refusals := map[string]func(*Service, string) string{
		"REST": func(svc *Service, id string) string {
			code, msg := restDiskCall(t, svc, "PATCH", "/v1/disks/"+id, `{"sizeGB":1001}`)
			if code != http.StatusBadRequest {
				t.Fatalf("PATCH sizeGB 1001 => %d %s, want 400", code, msg)
			}
			return msg
		},
		"GraphQL": func(svc *Service, id string) string {
			return gqlDiskError(t, svc, `mutation { updateDisk(id: "`+id+`", sizeGB: 1001) { id } }`)
		},
		"MCP": func(svc *Service, id string) string {
			return callAppsMCPError(t, svc, "update_disk", map[string]any{"diskId": id, "sizeGB": 1001})
		},
	}
	for surface, refuse := range refusals {
		t.Run(surface, func(t *testing.T) {
			svc, cl, st := newDiskService(diskEligibleApp("web"))
			disk, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10)
			if err != nil {
				t.Fatal(err)
			}
			if msg := refuse(svc, disk.ID); !strings.Contains(msg, diskCapRefusal) {
				t.Fatalf("refusal = %q, want %q", msg, diskCapRefusal)
			}
			if st.disks[disk.ID].SizeGB != 10 || getApp(t, cl, "web").Spec.Disk.SizeGB != 10 {
				t.Fatal("a refused grow past 1 TB still changed the row or spec.disk")
			}
			grown, err := svc.UpdateDisk(context.Background(), disk.ID, nil, nil, ptr.To(int32(1000)))
			if err != nil || grown.SizeGB != 1000 {
				t.Fatalf("grow to 1000 GB = %+v, %v; want accepted", grown, err)
			}
		})
	}
}

// A disk already above 1 TB (support raised it, or it predates the cap) keeps
// its size: a PATCH that repeats it goes through, a grow is refused with the
// cap, and a shrink is still a shrink.
func TestADiskAboveOneTBKeepsItsSize(t *testing.T) {
	app := diskEligibleApp("web")
	app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1500}
	svc, cl, st := newDiskService(app)
	id := ids.New(ids.Disk)
	st.disks[id] = store.Disk{ID: id, TenantID: "tea-disks", AppID: "srv-web", Name: "data", MountPath: "/var/data", SizeGB: 1500}

	mountPath := "/var/data2"
	if updated, err := svc.UpdateDisk(context.Background(), id, nil, &mountPath, ptr.To(int32(1500))); err != nil || updated.SizeGB != 1500 {
		t.Fatalf("remount repeating sizeGB 1500 = %+v, %v; want accepted", updated, err)
	}
	if _, err := svc.UpdateDisk(context.Background(), id, nil, nil, ptr.To(int32(1501))); !errors.Is(err, core.ErrBadRequest) ||
		!strings.Contains(err.Error(), diskCapRefusal) {
		t.Fatalf("grow 1500 → 1501 = %v, want %q", err, diskCapRefusal)
	}
	if _, err := svc.UpdateDisk(context.Background(), id, nil, nil, ptr.To(int32(1200))); !errors.Is(err, core.ErrBadRequest) ||
		!strings.Contains(err.Error(), "can only grow") {
		t.Fatalf("shrink 1500 → 1200 = %v, want the grow-only refusal", err)
	}
	if st.disks[id].SizeGB != 1500 || getApp(t, cl, "web").Spec.Disk.SizeGB != 1500 {
		t.Fatal("a refused resize changed the 1500 GB disk")
	}
}

// A Blueprint declaring a disk above 1 TB is invalid and its apply writes
// nothing; a sync that re-declares the size of a disk already above 1 TB stays
// accepted, and only growing it is refused.
func TestBlueprintDiskAboveOneTB(t *testing.T) {
	v, err := workspaceValidator(t).ValidateBlueprint(ownershipCtx(), connOwner, templateSmallDisk(1001), "")
	if err != nil || v.Valid || len(v.Errors) == 0 || !strings.Contains(v.Errors[0].Error, diskCapRefusal) {
		t.Fatalf("validate a 1001 GB disk = %+v, %v; want invalid with %q", v, err, diskCapRefusal)
	}
	if v, err := workspaceValidator(t).ValidateBlueprint(ownershipCtx(), connOwner, templateSmallDisk(1000), ""); err != nil || !v.Valid {
		t.Fatalf("validate a 1000 GB disk = %+v, %v; want valid", v, err)
	}

	rec := &recordingStore{}
	svc, ctx := ownSourceService(newFakeBlueprintStore(), rec)
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: templateSmallDisk(1001)}); err == nil || !strings.Contains(err.Error(), diskCapRefusal) {
		t.Fatalf("apply a 1001 GB disk = %v, want %q", err, diskCapRefusal)
	}
	var apps appv1alpha1.AppList
	if err := svc.Client.List(ctx, &apps); err != nil || len(apps.Items) != 0 || len(rec.disks) != 0 {
		t.Fatalf("a refused apply left %d apps and %d disk rows (%v)", len(apps.Items), len(rec.disks), err)
	}

	existing := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Tier: "starter",
		Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1500}}}
	declare := func(sizeGB int32, current *appv1alpha1.App) error {
		_, err := specFromCreate(withCurrentSettings(CreateRequest{
			Name: "web", Type: appv1alpha1.TypeWebService, Image: "nginx:1", Plan: "starter",
			Disk: &ServiceDiskView{Name: "data", MountPath: "/var/data", SizeGB: sizeGB},
		}, current))
		return err
	}
	if err := declare(1500, existing); err != nil {
		t.Fatalf("re-declaring the existing 1500 GB disk = %v, want accepted", err)
	}
	if err := declare(1501, existing); err == nil || !strings.Contains(err.Error(), diskCapRefusal) {
		t.Fatalf("growing the 1500 GB disk = %v, want %q", err, diskCapRefusal)
	}
	if err := declare(1500, nil); err == nil || !strings.Contains(err.Error(), diskCapRefusal) {
		t.Fatalf("a new service's 1500 GB disk = %v, want %q", err, diskCapRefusal)
	}
}
