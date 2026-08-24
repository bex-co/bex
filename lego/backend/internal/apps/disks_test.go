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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// diskApp is a store-managed, paid, single-instance service — the only shape
// that may carry a disk.
func diskEligibleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "default",
			Labels: map[string]string{
				store.LabelManagedBy: store.ManagedByValue,
				store.LabelAppID:     "srv-" + name,
				core.LabelTenant:     "tea-disks",
			},
		},
		Spec:   appv1alpha1.AppSpec{Image: name + ":v1", Replicas: 1, Tier: "starter"},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning},
	}
}

func newDiskService(app *appv1alpha1.App) (*Service, client.Client, *recordingStore) {
	st := &recordingStore{disks: map[string]store.Disk{}}
	svc, cl := newService(st, app)
	return svc, cl, st
}

func TestAddDiskWritesTheRowAndProjectsTheSpec(t *testing.T) {
	svc, cl, st := newDiskService(diskEligibleApp("web"))

	view, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 25)
	if err != nil {
		t.Fatalf("AddDisk: %v", err)
	}
	if view.SizeGB != 25 || view.MountPath != "/var/data" || view.ServiceID != "srv-web" {
		t.Fatalf("view = %+v, want the disk attached to srv-web at /var/data", view)
	}
	if len(st.disks) != 1 {
		t.Fatalf("store holds %d disks, want the one row that is the billing record", len(st.disks))
	}
	// The spec is what the operator acts on: without it the row would bill for
	// a volume that was never attached.
	spec := getApp(t, cl, "web").Spec.Disk
	if spec == nil || spec.MountPath != "/var/data" || spec.SizeGB != 25 {
		t.Fatalf("spec.disk = %+v, want the projected disk", spec)
	}
}

func TestAddDiskDefaultsToRendersTenGB(t *testing.T) {
	svc, _, _ := newDiskService(diskEligibleApp("web"))

	view, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 0)
	if err != nil {
		t.Fatalf("AddDisk: %v", err)
	}
	if view.SizeGB != diskDefaultSizeGB {
		t.Fatalf("sizeGB = %d, want Render's %d default", view.SizeGB, diskDefaultSizeGB)
	}
}

func TestAddDiskRefusesASecondDisk(t *testing.T) {
	svc, _, _ := newDiskService(diskEligibleApp("web"))
	if _, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10); err != nil {
		t.Fatalf("first AddDisk: %v", err)
	}

	_, err := svc.AddDisk(context.Background(), "web", "more", "/var/more", 10)
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("second AddDisk error = %v, want a conflict — a service may have at most one disk", err)
	}
}

// Each refusal mirrors a CRD-level CEL rule. They are asserted here too because
// an API that accepted them would write a billing row for a disk the API server
// then rejects, leaving the tenant charged for a volume that never existed.
func TestAddDiskRefusesIneligibleServices(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*appv1alpha1.App)
		wantMsg string
	}{
		{"cron job", func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeCronJob }, "cannot have a disk"},
		{"static site", func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeStaticSite }, "cannot have a disk"},
		{"free tier", func(a *appv1alpha1.App) { a.Spec.Tier = "free" }, "paid instance type"},
		{"two instances", func(a *appv1alpha1.App) { a.Spec.Replicas = 2 }, "more than one instance"},
		{"autoscaling on", func(a *appv1alpha1.App) {
			a.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{Enabled: true, MaxReplicas: 3}
		}, "autoscaling"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := diskEligibleApp("web")
			tc.mutate(app)
			svc, _, st := newDiskService(app)

			_, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("AddDisk error = %v, want bad request", err)
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error %q does not explain the refusal (%q)", err, tc.wantMsg)
			}
			if len(st.disks) != 0 {
				t.Fatal("a refused disk still wrote a billable row")
			}
		})
	}
}

func TestAddDiskRefusesReservedAndMalformedMountPaths(t *testing.T) {
	for _, tc := range []struct{ name, path, wantMsg string }{
		{"relative", "data", "absolute"},
		{"root", "/", "root directory"},
		{"trailing slash", "/var/data/", "root directory"},
		{"build output", "/opt/render/project/src", "reserved"},
		{"projected secrets", "/etc/secrets", "reserved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := newDiskService(diskEligibleApp("web"))
			_, err := svc.AddDisk(context.Background(), "web", "data", tc.path, 10)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("AddDisk(%q) error = %v, want bad request", tc.path, err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error %q does not explain the refusal (%q)", err, tc.wantMsg)
			}
		})
	}
	// A subdirectory of a reserved path is legal — it is mounting OVER the
	// directory that breaks the platform, not mounting inside it.
	svc, _, _ := newDiskService(diskEligibleApp("web"))
	if _, err := svc.AddDisk(context.Background(), "web", "data", "/opt/render/project/src/uploads", 10); err != nil {
		t.Fatalf("subdirectory of a reserved path refused: %v", err)
	}
}

func TestUpdateDiskGrowsButNeverShrinks(t *testing.T) {
	svc, cl, _ := newDiskService(diskEligibleApp("web"))
	created, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10)
	if err != nil {
		t.Fatalf("AddDisk: %v", err)
	}

	grown, err := svc.UpdateDisk(context.Background(), created.ID, nil, nil, ptr.To(int32(50)))
	if err != nil {
		t.Fatalf("grow: %v", err)
	}
	if grown.SizeGB != 50 {
		t.Fatalf("sizeGB = %d, want 50", grown.SizeGB)
	}
	if got := getApp(t, cl, "web").Spec.Disk; got == nil || got.SizeGB != 50 {
		t.Fatalf("spec.disk = %+v, want the grow projected so the operator expands the volume", got)
	}

	_, err = svc.UpdateDisk(context.Background(), created.ID, nil, nil, ptr.To(int32(20)))
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("shrink error = %v, want bad request — shrinking destroys the filesystem", err)
	}
	if got := getApp(t, cl, "web").Spec.Disk; got.SizeGB != 50 {
		t.Fatalf("a refused shrink still changed spec.disk to %dGB", got.SizeGB)
	}
}

func TestDeleteDiskDetachesAndStopsBilling(t *testing.T) {
	svc, cl, st := newDiskService(diskEligibleApp("web"))
	created, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10)
	if err != nil {
		t.Fatalf("AddDisk: %v", err)
	}

	if err := svc.DeleteDisk(context.Background(), created.ID); err != nil {
		t.Fatalf("DeleteDisk: %v", err)
	}
	// Clearing the spec is what releases the volume; closing the row is what
	// stops the meter. Both must happen or the tenant keeps paying.
	if got := getApp(t, cl, "web").Spec.Disk; got != nil {
		t.Fatalf("spec.disk = %+v, want nil after delete", got)
	}
	if st.disks[created.ID].DeletedAt == nil {
		t.Fatal("the disk row is still open; the meter would keep billing a deleted volume")
	}
	if _, err := svc.GetDisk(context.Background(), created.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetDisk after delete = %v, want not found", err)
	}
}

func TestServiceViewCarriesTheDisk(t *testing.T) {
	svc, _, _ := newDiskService(diskEligibleApp("web"))
	if _, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10); err != nil {
		t.Fatalf("AddDisk: %v", err)
	}

	view, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.Disk == nil || view.Disk.MountPath != "/var/data" || view.Disk.SizeGB != 10 {
		t.Fatalf("service view disk = %+v, want the attached disk", view.Disk)
	}
}

// Render's schema nests the attached disk inside serviceDetails (its
// `serviceDisk`), so a Render client reads serviceDetails.disk. bex exposed it
// only on the GraphQL sibling view until the w1/m86 parity audit: REST and MCP
// both render through renderService, which knew nothing about disks, so every
// Render-shaped client saw a diskless service no matter what was attached.
func TestRenderedServiceDetailsCarryTheDisk(t *testing.T) {
	svc, _, _ := newDiskService(diskEligibleApp("web"))
	if _, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10); err != nil {
		t.Fatalf("AddDisk: %v", err)
	}
	view, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	details := toRenderService(view).ServiceDetails
	disk, ok := details["disk"].(map[string]any)
	if !ok {
		t.Fatalf("serviceDetails.disk absent; a Render client reads it there. got keys %v", details)
	}
	if disk["mountPath"] != "/var/data" || disk["sizeGB"] != int32(10) || disk["name"] != "data" {
		t.Errorf("serviceDetails.disk = %+v, want the attached disk's three Render fields", disk)
	}
}

// A diskless service must OMIT the key rather than carry a null or a zero
// disk — every other optional serviceDetails field behaves that way, and a
// present-but-empty disk would read as "there is one, sized 0".
func TestRenderedServiceDetailsOmitDiskWhenThereIsNone(t *testing.T) {
	svc, _, _ := newDiskService(diskEligibleApp("web"))
	view, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, present := toRenderService(view).ServiceDetails["disk"]; present {
		t.Error("serviceDetails.disk present on a diskless service")
	}
}

// --- w1/m88: Render REST parity ---

func diskRESTMux(t *testing.T, svc *Service) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return mux
}

func listDisksREST(t *testing.T, mux *http.ServeMux, query string) []DiskView {
	t.Helper()
	rec := httptest.NewRecorder()
	// The workspace-wide branch (no serviceId, or several) reads the caller's
	// workspace, so the request has to carry the one the fixture App belongs to
	// — otherwise it lists a different tenant's disks, which is correctly none.
	req := httptest.NewRequest("GET", "/v1/disks?"+query, nil)
	req = req.WithContext(core.WithWorkspace(req.Context(), "tea-disks"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/disks?%s => 200, got %d: %s", query, rec.Code, rec.Body)
	}
	var out []struct {
		Disk DiskView `json:"disk"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode list: %v (%s)", err, rec.Body)
	}
	disks := make([]DiskView, 0, len(out))
	for _, row := range out {
		disks = append(disks, row.Disk)
	}
	return disks
}

// Render documents diskId, name and two time windows on list-disks, and the
// pinned spec declares them — so before w1/m88 they passed the OpenAPI gate and
// were then silently dropped, handing the caller an UNFILTERED list with no
// error and no way to tell. A wrong answer delivered confidently is worse than
// a refusal; these assertions fail the moment a filter stops filtering.
func TestListDisksHonorsRendersDocumentedFilters(t *testing.T) {
	svc, _, _ := newDiskService(diskEligibleApp("web"))
	disk, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10)
	if err != nil {
		t.Fatalf("AddDisk: %v", err)
	}
	mux := diskRESTMux(t, svc)

	if got := listDisksREST(t, mux, "serviceId=srv-web"); len(got) != 1 {
		t.Fatalf("service-scoped list => 1 disk, got %d", len(got))
	}
	// A filter that matches keeps the disk; one that does not must exclude it.
	for _, tc := range []struct {
		query string
		want  int
	}{
		{"serviceId=srv-web&diskId=" + disk.ID, 1},
		{"serviceId=srv-web&diskId=dsk-nope", 0},
		{"serviceId=srv-web&name=data", 1},
		{"serviceId=srv-web&name=other", 0},
		{"serviceId=srv-web", 1},
		{"serviceId=srv-web&createdBefore=2000-01-01T00:00:00Z", 0},
		{"serviceId=srv-web&createdAfter=2000-01-01T00:00:00Z", 1},
		{"serviceId=srv-web&updatedBefore=2000-01-01T00:00:00Z", 0},
	} {
		if got := listDisksREST(t, mux, tc.query); len(got) != tc.want {
			t.Errorf("?%s => %d disks, want %d", tc.query, len(got), tc.want)
		}
	}
}

// Render types serviceId as an ARRAY. Reading only the first value silently
// narrows a two-service query to one service's disks.
func TestListDisksUnionsRepeatedServiceID(t *testing.T) {
	svc, _, _ := newDiskService(diskEligibleApp("web"))
	if _, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10); err != nil {
		t.Fatalf("AddDisk: %v", err)
	}
	mux := diskRESTMux(t, svc)

	// The real service plus one that does not exist: the union must still
	// return the real one rather than resolving only the first value.
	if got := listDisksREST(t, mux, "serviceId=srv-nope&serviceId=srv-web"); len(got) != 1 {
		t.Errorf("repeated serviceId => 1 disk, got %d (only the first value was honored?)", len(got))
	}
}

// Render's create bodies carry serviceDetails.disk. Because Render-matched
// routes decode strictly, the field's absence was not "ignored" — it was a 400
// on `unknown field "disk"`, so a Render client could not create a
// disk-bearing service at all.
func TestCreateServiceAcceptsRendersServiceDetailsDisk(t *testing.T) {
	svc, _ := newService(nil)
	mux := diskRESTMux(t, svc)

	body := `{"name":"web","type":"web_service","repo":"https://github.com/o/r","serviceDetails":{"plan":"starter","disk":{"name":"data","mountPath":"/var/data","sizeGB":25}}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create with serviceDetails.disk => 201, got %d: %s", rec.Code, rec.Body)
	}

	var out struct {
		Service struct {
			ServiceDetails struct {
				Disk *ServiceDiskView `json:"disk"`
			} `json:"serviceDetails"`
		} `json:"service"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode create: %v (%s)", err, rec.Body)
	}
	got := out.Service.ServiceDetails.Disk
	if got == nil || got.MountPath != "/var/data" || got.SizeGB != 25 {
		t.Fatalf("created service's disk = %+v, want the declared disk", got)
	}
}

// Render's serviceDisk requires only name + mountPath at create — sizeGB is
// optional there, while the standalone POST /v1/disks requires it. That
// asymmetry is Render's; bex reproduces it rather than smoothing it out, so the
// create path must fill the default instead of refusing.
func TestCreateServiceDiskDefaultsSizeWhenOmitted(t *testing.T) {
	svc, _ := newService(nil)
	mux := diskRESTMux(t, svc)

	body := `{"name":"web","type":"web_service","repo":"https://github.com/o/r","serviceDetails":{"plan":"starter","disk":{"name":"data","mountPath":"/var/data"}}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create with sizeGB omitted => 201, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		Service struct {
			ServiceDetails struct {
				Disk *ServiceDiskView `json:"disk"`
			} `json:"serviceDetails"`
		} `json:"service"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Service.ServiceDetails.Disk == nil || out.Service.ServiceDetails.Disk.SizeGB != diskDefaultSizeGB {
		t.Fatalf("omitted sizeGB => default %d, got %+v", diskDefaultSizeGB, out.Service.ServiceDetails.Disk)
	}
}
