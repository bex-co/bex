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
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/utils/ptr"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m548: as on Render ("You can't change the shutdown delay for a service
// with a persistent disk", render.com/docs/disks), a service with a disk keeps
// the 30-second default on every surface.

func diskShutdownDelayApp(delay *int32) *appv1alpha1.App {
	app := diskEligibleApp("web")
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	app.Spec.MaxShutdownDelaySeconds = delay
	return app
}

// readMaxShutdownDelay is serviceDetails.maxShutdownDelaySeconds as REST
// GET /v1/services/{id} reports it.
func readMaxShutdownDelay(t *testing.T, mux *http.ServeMux, id string) float64 {
	t.Helper()
	rec := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/"+id, nil))
	var out struct {
		ServiceDetails map[string]any `json:"serviceDetails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); rec.Code != http.StatusOK || err != nil {
		t.Fatalf("GET %s => %d %v: %s", id, rec.Code, err, rec.Body)
	}
	got, _ := out.ServiceDetails["maxShutdownDelaySeconds"].(float64)
	return got
}

func TestDiskServiceRefusesACustomShutdownDelayOnEverySurface(t *testing.T) {
	// A dry-run answers from the patch's preflight alone, so it is the one
	// call that shows the preflight refuses too.
	for _, surface := range []string{"REST", "REST dry-run", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			svc, cl, _ := newDiskService(diskShutdownDelayApp(nil))
			sink := &captureAuditSink{}
			svc.Audit = sink
			before := getApp(t, cl, "web").DeepCopy()
			var message string
			switch surface {
			case "REST", "REST dry-run":
				body := `{"serviceDetails":{"maxShutdownDelaySeconds":120}}`
				if surface == "REST dry-run" {
					body = `{"serviceDetails":{"maxShutdownDelaySeconds":120},"dryRun":true}`
				}
				rec := httptest.NewRecorder()
				validated(t, diskRESTMux(t, svc)).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web",
					strings.NewReader(body)))
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("PATCH => %d: %s", rec.Code, rec.Body)
				}
				message = rec.Body.String()
			case "GraphQL":
				res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: context.Background(),
					RequestString: `mutation { setMaxShutdownDelay(id: "web", seconds: 120) { id } }`})
				if len(res.Errors) != 1 {
					t.Fatalf("setMaxShutdownDelay = %+v, want one refusal", res)
				}
				message = res.Errors[0].Message
			case "MCP":
				message = callAppsMCPError(t, svc, "update_service", map[string]any{"serviceId": "web", "maxShutdownDelaySeconds": 120})
			}
			if !strings.Contains(message, diskShutdownDelayRefusal) {
				t.Fatalf("refusal = %s, want %q", message, diskShutdownDelayRefusal)
			}
			if after := getApp(t, cl, "web"); !reflect.DeepEqual(before, after) {
				t.Fatalf("refused write changed the CR: %+v", after.Spec)
			}
			if n := sink.countVerb(core.AuditVerbSetMaxShutdownDelay); n != 0 {
				t.Fatalf("refused write recorded %d shutdown-delay changes", n)
			}
		})
	}
}

// bex reports 30 for a disk service, so a client writing back what it read is
// not refused for a value it cannot change. The write stores nothing.
func TestDiskServiceTakesOnlyTheDefaultShutdownDelay(t *testing.T) {
	ctx := context.Background()
	t.Run("the default on a disk service is a no-op", func(t *testing.T) {
		svc, cl, _ := newDiskService(diskShutdownDelayApp(nil))
		sink := &captureAuditSink{}
		svc.Audit = sink
		before := getApp(t, cl, "web").DeepCopy()
		view, err := svc.SetMaxShutdownDelay(ctx, "web", 30)
		if err != nil || view.MaxShutdownDelaySeconds != 30 {
			t.Fatalf("set 30 = %d, %v; want 30 accepted", view.MaxShutdownDelaySeconds, err)
		}
		if after := getApp(t, cl, "web"); !reflect.DeepEqual(before, after) {
			t.Fatalf("a no-op wrote the CR: %+v", after.Spec)
		}
		if n := sink.countVerb(core.AuditVerbSetMaxShutdownDelay); n != 0 {
			t.Fatalf("a no-op recorded %d shutdown-delay changes", n)
		}
	})
	t.Run("the default clears a delay held from before the rule", func(t *testing.T) {
		svc, cl, _ := newDiskService(diskShutdownDelayApp(ptr.To(int32(120))))
		sink := &captureAuditSink{}
		svc.Audit = sink
		mux := diskRESTMux(t, svc)
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web",
			strings.NewReader(`{"serviceDetails":{"maxShutdownDelaySeconds":30}}`)))
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH 30 => %d: %s", rec.Code, rec.Body)
		}
		if got := getApp(t, cl, "web").Spec.MaxShutdownDelaySeconds; got != nil {
			t.Fatalf("spec.maxShutdownDelaySeconds = %d, want none stored", *got)
		}
		if n := sink.countVerb(core.AuditVerbSetMaxShutdownDelay); n != 1 {
			t.Fatalf("recorded %d shutdown-delay changes, want 1", n)
		}
	})
	t.Run("a service without a disk keeps the whole range", func(t *testing.T) {
		app := diskShutdownDelayApp(nil)
		app.Spec.Disk = nil
		for _, seconds := range []int32{1, 300} {
			svc, cl, _ := newDiskService(app.DeepCopy())
			if _, err := svc.SetMaxShutdownDelay(ctx, "web", seconds); err != nil {
				t.Fatalf("set %d without a disk: %v", seconds, err)
			}
			if got := getApp(t, cl, "web").Spec.MaxShutdownDelaySeconds; got == nil || *got != seconds {
				t.Fatalf("spec.maxShutdownDelaySeconds = %v, want %d", got, seconds)
			}
		}
	})
}

// A delay the spec still holds (a CR written past bex-api, a rollback to a
// release recorded before the disk) reads as the default the operator runs.
func TestDiskServiceReadsTheDefaultShutdownDelay(t *testing.T) {
	svc, _, _ := newDiskService(diskShutdownDelayApp(ptr.To(int32(120))))
	if got := readMaxShutdownDelay(t, diskRESTMux(t, svc), "web"); got != 30 {
		t.Fatalf("REST maxShutdownDelaySeconds = %v, want 30", got)
	}
	res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: context.Background(),
		RequestString: `{ server(id: "web") { maxShutdownDelaySeconds } }`})
	if len(res.Errors) > 0 || res.Data.(map[string]any)["server"].(map[string]any)["maxShutdownDelaySeconds"] != 30 {
		t.Fatalf("GraphQL server = %+v, want maxShutdownDelaySeconds 30", res)
	}
}

func TestCreateWithADiskRefusesACustomShutdownDelay(t *testing.T) {
	create := func(t *testing.T, delay string) (*httptest.ResponseRecorder, *Service) {
		t.Helper()
		svc, _ := newService(nil)
		rec := httptest.NewRecorder()
		body := `{"name":"web","type":"web_service","repo":"https://github.com/o/r","serviceDetails":{"plan":"starter",` +
			`"maxShutdownDelaySeconds":` + delay + `,"disk":{"name":"data","mountPath":"/var/data","sizeGB":10}}}`
		validated(t, diskRESTMux(t, svc)).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
		return rec, svc
	}
	rec, svc := create(t, "120")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), diskShutdownDelayRefusal) {
		t.Fatalf("create with a disk and 120 s => %d: %s", rec.Code, rec.Body)
	}
	if _, err := svc.Get(context.Background(), "web"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("a refused create left a service: %v", err)
	}

	rec, svc = create(t, "30")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create with a disk and 30 s => %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, svc.Client, "web").Spec.MaxShutdownDelaySeconds; got != nil {
		t.Fatalf("spec.maxShutdownDelaySeconds = %d, want none stored", *got)
	}
}

// Every attach path returns a custom delay to the default in the write that
// attaches the disk, and records it as the shutdown-delay setting change.
func TestAttachingADiskReturnsACustomShutdownDelayToTheDefault(t *testing.T) {
	attach := map[string]func(t *testing.T, svc *Service){
		"REST": func(t *testing.T, svc *Service) {
			rec := httptest.NewRecorder()
			validated(t, diskRESTMux(t, svc)).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/disks",
				strings.NewReader(`{"serviceId":"web","name":"data","mountPath":"/var/data","sizeGB":10}`)))
			if rec.Code != http.StatusCreated {
				t.Fatalf("POST /v1/disks => %d: %s", rec.Code, rec.Body)
			}
		},
		"GraphQL": func(t *testing.T, svc *Service) {
			res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: context.Background(),
				RequestString: `mutation { addDisk(serviceId: "web", name: "data", mountPath: "/var/data", sizeGB: 10) { id } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("addDisk: %v", res.Errors)
			}
		},
		"MCP": func(t *testing.T, svc *Service) {
			res, err := displayNameMCPSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "add_disk",
				Arguments: map[string]any{"serviceId": "web", "name": "data", "mountPath": "/var/data", "sizeGB": 10}})
			if err != nil || res.IsError {
				t.Fatalf("add_disk: err=%v result=%+v", err, res)
			}
		},
	}
	for surface, run := range attach {
		for _, tc := range []struct {
			name     string
			delay    *int32
			recorded int
		}{
			{"a custom delay", ptr.To(int32(120)), 1},
			{"an explicit default", ptr.To(int32(30)), 0},
			{"no delay", nil, 0},
		} {
			t.Run(surface+"/"+tc.name, func(t *testing.T) {
				app := diskShutdownDelayApp(tc.delay)
				app.Spec.Disk = nil
				svc, cl, _ := newDiskService(app)
				sink := &captureAuditSink{}
				svc.Audit = sink
				run(t, svc)
				after := getApp(t, cl, "web")
				if after.Spec.Disk == nil || after.Spec.MaxShutdownDelaySeconds != nil {
					t.Fatalf("after attach: disk %+v, delay %v; want the disk and no stored delay",
						after.Spec.Disk, after.Spec.MaxShutdownDelaySeconds)
				}
				if got := readMaxShutdownDelay(t, diskRESTMux(t, svc), "web"); got != 30 {
					t.Fatalf("reads back %v, want 30", got)
				}
				if n := sink.countVerb(core.AuditVerbSetMaxShutdownDelay); n != tc.recorded {
					t.Fatalf("recorded %d shutdown-delay changes, want %d", n, tc.recorded)
				}
			})
		}
	}
}

const diskRuleDelay = "    maxShutdownDelaySeconds: "

// Render's validator refuses the field beside a disk at any value, the
// default included, at the field's own path.
func TestBlueprintRefusesADiskWithAShutdownDelay(t *testing.T) {
	ctx := context.Background()
	const want = `service "qa-bp-web": max shutdown delay is not supported for services with a disk`
	for _, seconds := range []string{"30", "120"} {
		t.Run(seconds, func(t *testing.T) {
			svc, cl := newService(nil)
			manifest := diskRuleWeb + diskRuleDelay + seconds + "\n" + diskRuleDisk
			validation, err := svc.ValidateBlueprint(ctx, "", manifest, "")
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if validation.Valid || len(validation.Errors) != 1 {
				t.Fatalf("validation = valid %v, errors %+v, want the one refusal", validation.Valid, validation.Errors)
			}
			got := validation.Errors[0]
			if got.Error != want || got.Path == nil || *got.Path != "services[0].maxShutdownDelaySeconds" {
				t.Fatalf("error = %q at %v, want %q at services[0].maxShutdownDelaySeconds", got.Error, got.Path, want)
			}
			if validation.Plan != nil && len(validation.Plan.Actions) != 0 {
				t.Fatalf("a refused file planned %+v", validation.Plan.Actions)
			}
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("sync = %v, want the same refusal", err)
			}
			var apps appv1alpha1.AppList
			if err := cl.List(ctx, &apps); err != nil || len(apps.Items) != 0 {
				t.Fatalf("a refused sync wrote %d Apps (%v)", len(apps.Items), err)
			}
		})
	}
	// Each half alone is a valid service.
	svc, _ := newService(nil)
	for name, manifest := range map[string]string{"disk": diskRuleWeb + diskRuleDisk, "delay": diskRuleWeb + diskRuleDelay + "120\n"} {
		if v, err := svc.ValidateBlueprint(ctx, "", manifest, ""); err != nil || !v.Valid {
			t.Fatalf("a service with only the %s = valid %v, errors %+v, err %v", name, v.Valid, v.Errors, err)
		}
	}
}

// A sync judges the service it leaves: a disk declared for a service with a
// custom delay returns the delay to the default, and a delay declared for a
// service whose disk was attached outside the file is refused.
func TestBlueprintSyncKeepsADiskServicesDefaultShutdownDelay(t *testing.T) {
	disk := &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	for name, tc := range map[string]struct {
		existing, want, result appv1alpha1.AppSpec
		fields                 []string
		refused                bool
	}{
		"a disk declared for a service with a custom delay": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 1, MaxShutdownDelaySeconds: ptr.To(int32(120))},
			want:     appv1alpha1.AppSpec{Disk: disk},
			fields:   []string{"disk"},
			result:   appv1alpha1.AppSpec{Tier: "starter", Replicas: 1, Disk: disk},
		},
		"a delay declared for a service that has a disk": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 1, Disk: disk},
			want:     appv1alpha1.AppSpec{MaxShutdownDelaySeconds: ptr.To(int32(30))},
			fields:   []string{"maxShutdownDelaySeconds"},
			refused:  true,
		},
		"a delay declared for a service without a disk": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 1},
			want:     appv1alpha1.AppSpec{MaxShutdownDelaySeconds: ptr.To(int32(120))},
			fields:   []string{"maxShutdownDelaySeconds"},
			result:   appv1alpha1.AppSpec{Tier: "starter", Replicas: 1, MaxShutdownDelaySeconds: ptr.To(int32(120))},
		},
	} {
		t.Run(name, func(t *testing.T) {
			fields := map[string]BlueprintField{}
			for _, field := range tc.fields {
				fields[field] = BlueprintField{}
			}
			dst := *tc.existing.DeepCopy()
			_, err := ApplyBlueprintServiceSpec(&dst, tc.want, fields)
			if tc.refused {
				var conflict *BlueprintFieldConflictError
				if !errors.As(err, &conflict) || conflict.Path != "maxShutdownDelaySeconds" || conflict.Message != diskShutdownDelayRefusal {
					t.Fatalf("sync = %v, want maxShutdownDelaySeconds: %s", err, diskShutdownDelayRefusal)
				}
				if !reflect.DeepEqual(dst, tc.existing) {
					t.Fatalf("a refused sync still changed the spec: %+v", dst)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(dst, tc.result) {
				t.Fatalf("sync = %+v, %v; want %+v", dst, err, tc.result)
			}
		})
	}
}

// Through a managed sync, the reset lands with the disk and is recorded as the
// shutdown-delay setting change.
func TestBlueprintDiskSyncRecordsTheShutdownDelayReset(t *testing.T) {
	rec := &recordingStore{}
	svc, base, sink, ctx := managedStackService(t, rec)
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: imageService + diskRuleDelay + "120\n"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	sink.events = nil
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: imageService + "    disk: {name: data, mountPath: /var/data, sizeGB: 10}\n"}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	app := getTenantApp(t, base, "tea-a", "web")
	if app.Spec.Disk == nil || app.Spec.MaxShutdownDelaySeconds != nil {
		t.Fatalf("after the sync: disk %+v, delay %v; want the disk and no stored delay", app.Spec.Disk, app.Spec.MaxShutdownDelaySeconds)
	}
	if got := serviceRows(sink); !slices.Contains(got, core.AuditVerbSetMaxShutdownDelay+" by id-a") {
		t.Fatalf("the sync recorded %v, want the shutdown-delay change", got)
	}
}

// An export never writes the field beside a disk, so it re-imports.
func TestGenerateBlueprintOmitsADiskServicesShutdownDelay(t *testing.T) {
	svc, _, _ := newDiskService(diskShutdownDelayApp(ptr.To(int32(120))))
	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{ServiceIDs: []string{"web"}})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if strings.Contains(out.Manifest, "maxShutdownDelaySeconds") || !strings.Contains(out.Manifest, "mountPath: /var/data") {
		t.Fatalf("manifest carries the delay or lost the disk:\n%s", out.Manifest)
	}
}
