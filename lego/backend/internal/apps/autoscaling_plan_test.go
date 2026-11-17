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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w7/m429 t001: autoscaling needs the Pro plan (render.com/docs/scaling:
// "Available only for Pro workspaces and higher"). Turning it on below Pro is
// refused on every write path with PLAN_FEATURE_REQUIRED before any write; a
// policy already on keeps running, re-sends unchanged and can be turned off.

const autoscalingRefusal = "bad request: autoscaling requires the Pro plan or higher; this workspace is on Hobby"

func tenantPaidWebApp(name string) *appv1alpha1.App {
	a := paidWebApp(name)
	a.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelServiceName: name}
	return a
}

func autoscalingOnAt(a *appv1alpha1.App, minReplicas, maxReplicas, cpu int32) *appv1alpha1.App {
	a.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: minReplicas, MaxReplicas: maxReplicas, TargetCPUPercent: &cpu}
	return a
}

func wantAutoscalingRefusal(t *testing.T, err error) {
	t.Helper()
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode || err.Error() != autoscalingRefusal ||
		coded.Params["feature"] != "autoscaling" || coded.Params["requiredPlan"] != "pro" || coded.Params["plan"] != "hobby" {
		t.Fatalf("want the coded Pro-plan refusal, got %v", err)
	}
}

func TestAutoscalingNeedsProOnEverySurface(t *testing.T) {
	const (
		restBody = `{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":0}}}`
		mutation = `mutation {setAutoscaling(id:"web",minInstances:1,maxInstances:3,targetCPUPercent:60){enabled}}`
	)
	mcpArgs := map[string]any{"serviceId": "web", "autoscaling": map[string]any{"minInstances": 1, "maxInstances": 3, "targetCPUPercent": 60}}
	for _, plan := range []string{"hobby", "pro", "scale", "enterprise"} {
		for _, surface := range []string{"REST", "GraphQL", "MCP"} {
			t.Run(plan+"/"+surface, func(t *testing.T) {
				svc, cl := newService(nil, tenantPaidWebApp("web"))
				withWorkspacePlan(svc, plan)
				var (
					refused bool
					code    string
					params  map[string]any
					message string
				)
				switch surface {
				case "REST":
					w := putAutoscaling(t, svc, restBody)
					refused = w.Code != http.StatusOK
					var body struct {
						Code    string         `json:"code"`
						Message string         `json:"message"`
						Params  map[string]any `json:"params"`
					}
					_ = json.Unmarshal(w.Body.Bytes(), &body)
					if refused && w.Code != http.StatusBadRequest {
						t.Fatalf("status = %d, want 400: %s", w.Code, w.Body)
					}
					code, params, message = body.Code, body.Params, body.Message
				case "GraphQL":
					result := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: t.Context(), RequestString: mutation})
					if refused = len(result.Errors) > 0; refused {
						message = result.Errors[0].Message
						code, _ = result.Errors[0].Extensions["code"].(string)
						params = map[string]any{"feature": result.Errors[0].Extensions["feature"], "requiredPlan": result.Errors[0].Extensions["requiredPlan"]}
					}
				case "MCP":
					if plan != "hobby" {
						if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{Autoscaling: &SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3, TargetCPUPercent: int32p(60)}}); err != nil {
							t.Fatal(err)
						}
						break
					}
					refused = true
					text := callAppsMCPError(t, svc, "update_service", mcpArgs)
					code, message, _ = strings.Cut(text, ": ")
					params = map[string]any{"feature": "autoscaling", "requiredPlan": "pro"}
				}
				as := getApp(t, cl, "web").Spec.Autoscaling
				if plan != "hobby" {
					if refused || as == nil || !as.Enabled || as.MaxReplicas != 3 {
						t.Fatalf("%s must turn autoscaling on: refused=%v %s, spec %+v", plan, refused, message, as)
					}
					return
				}
				if !refused || code != core.PlanFeatureRequiredCode || message != autoscalingRefusal ||
					params["feature"] != "autoscaling" || params["requiredPlan"] != "pro" {
					t.Fatalf("hobby: want %s %q, got refused=%v %s %q %v", core.PlanFeatureRequiredCode, autoscalingRefusal, refused, code, message, params)
				}
				if as != nil {
					t.Fatalf("a refused enable wrote spec.autoscaling: %+v", as)
				}
			})
		}
	}
}

func TestAutoscalingOnHobbyKeepsAnExistingPolicy(t *testing.T) {
	svc, cl := newService(nil, autoscalingOnAt(tenantPaidWebApp("web"), 1, 3, 60))
	withWorkspacePlan(svc, "hobby")
	ctx := context.Background()
	stored := getApp(t, cl, "web").Spec.Autoscaling.DeepCopy()

	if v, err := svc.GetAutoscaling(ctx, "web"); err != nil || !v.Enabled || v.MaxInstances != 3 {
		t.Fatalf("the existing policy must read back unchanged: %+v %v", v, err)
	}
	if _, err := svc.SetAutoscaling(ctx, "web", SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3, TargetCPUPercent: int32p(60)}); err != nil {
		t.Fatalf("re-sending the stored policy must pass: %v", err)
	}
	if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Autoscaling: &SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3, TargetCPUPercent: int32p(60)}}); err != nil {
		t.Fatalf("a PATCH round-tripping the stored policy must pass: %v", err)
	}
	_, err := svc.SetAutoscaling(ctx, "web", SetAutoscalingRequest{MinInstances: 1, MaxInstances: 5, TargetCPUPercent: int32p(60)})
	wantAutoscalingRefusal(t, err)
	if got := getApp(t, cl, "web").Spec.Autoscaling; !reflect.DeepEqual(got, stored) {
		t.Fatalf("a refused change moved the policy: %+v", got)
	}
	if err := svc.DeleteAutoscaling(ctx, "web"); err != nil {
		t.Fatalf("turning the policy off must stay allowed: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Autoscaling; got == nil || got.Enabled {
		t.Fatalf("disable left %+v", got)
	}
	// Turned off, it is a new enable: that needs Pro, even with the kept
	// settings.
	_, err = svc.SetAutoscaling(ctx, "web", SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3, TargetCPUPercent: int32p(60)})
	wantAutoscalingRefusal(t, err)
}

func TestAutoscalingPlanGateDegrades(t *testing.T) {
	req := SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3, TargetCPUPercent: int32p(60)}
	t.Run("unreadable plan fails the write", func(t *testing.T) {
		svc, cl := newService(nil, tenantPaidWebApp("web"))
		boom := errors.New("tenants table unavailable")
		svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "", boom })
		if _, err := svc.SetAutoscaling(context.Background(), "web", req); !errors.Is(err, boom) {
			t.Fatalf("want the plan read error, got %v", err)
		}
		if getApp(t, cl, "web").Spec.Autoscaling != nil {
			t.Fatal("a write whose plan could not be read changed the App")
		}
	})
	t.Run("no plan reader leaves it open", func(t *testing.T) {
		svc, _ := newService(nil, tenantPaidWebApp("web"))
		if _, err := svc.SetAutoscaling(context.Background(), "web", req); err != nil {
			t.Fatalf("store off: %v", err)
		}
	})
}

func TestCreateWithAutoscalingNeedsPro(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
	body := `{"name":"app","type":"web_service","image":{"imagePath":"nginx:v1"},"serviceDetails":{"plan":"starter",
		"autoscaling":{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":0}}}}}`
	for _, plan := range []string{"hobby", "pro"} {
		t.Run(plan, func(t *testing.T) {
			svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{})
			withWorkspacePlan(svc, plan)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)).WithContext(ctx))
			var list appv1alpha1.AppList
			if err := cl.List(ctx, &list); err != nil {
				t.Fatal(err)
			}
			if plan == "pro" {
				if rec.Code != http.StatusCreated || len(list.Items) != 1 || !autoscalingOn(list.Items[0].Spec.Autoscaling) {
					t.Fatalf("pro create => %d %s (%d Apps)", rec.Code, rec.Body, len(list.Items))
				}
				return
			}
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"PLAN_FEATURE_REQUIRED"`) || !strings.Contains(rec.Body.String(), autoscalingRefusal) {
				t.Fatalf("hobby create => %d %s", rec.Code, rec.Body)
			}
			if len(list.Items) != 0 {
				t.Fatalf("a refused create left %d Apps", len(list.Items))
			}
		})
	}
}

const scaledBlueprint = `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    plan: starter
    scaling: {minInstances: 1, maxInstances: 3, targetCPUPercent: 60}
`

// The Blueprint reports the refusal at the service's scaling path in validate,
// and the sync refuses it with the same code before writing; a service that
// already autoscales exactly so stays valid on Hobby.
func TestBlueprintScalingNeedsPro(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
	for _, tc := range []struct {
		name     string
		plan     string
		existing *appv1alpha1.App
		refused  bool
	}{
		{"hobby new service", "hobby", nil, true},
		{"hobby turns it on", "hobby", tenantPaidWebApp("web"), true},
		{"hobby changes it", "hobby", autoscalingOnAt(tenantPaidWebApp("web"), 1, 2, 60), true},
		{"hobby re-declares it", "hobby", autoscalingOnAt(tenantPaidWebApp("web"), 1, 3, 60), false},
		{"pro", "pro", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var apps []*appv1alpha1.App
			if tc.existing != nil {
				tc.existing.Namespace = "tea-a"
				apps = append(apps, tc.existing)
			}
			svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{}, apps...)
			withWorkspacePlan(svc, tc.plan)
			before := map[string]*appv1alpha1.AutoscalingSpec{}
			for _, a := range apps {
				before[a.Name] = a.Spec.Autoscaling.DeepCopy()
			}

			v, err := svc.ValidateBlueprint(ctx, "tea-a", scaledBlueprint, "")
			if err != nil {
				t.Fatal(err)
			}
			if !tc.refused {
				if !v.Valid {
					t.Fatalf("want valid, got %+v", v.Errors)
				}
				return
			}
			if v.Valid || len(v.Errors) != 1 {
				t.Fatalf("want one plan entry, got %+v", v)
			}
			got := v.Errors[0]
			if got.Code != core.PlanFeatureRequiredCode || got.Path == nil || *got.Path != "services[0].scaling" ||
				got.Error != `service "web": scaling: autoscaling requires the Pro plan or higher; this workspace is on Hobby` || got.Line == nil {
				t.Fatalf("entry = %+v (path %v)", got, *got.Path)
			}

			_, err = svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: scaledBlueprint})
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode || coded.Params["feature"] != "autoscaling" {
				t.Fatalf("sync: want the coded refusal, got %v", err)
			}
			var list appv1alpha1.AppList
			if err := cl.List(ctx, &list); err != nil {
				t.Fatal(err)
			}
			if len(list.Items) != len(apps) {
				t.Fatalf("a refused sync left %d Apps, want %d", len(list.Items), len(apps))
			}
			for _, a := range list.Items {
				if !reflect.DeepEqual(a.Spec.Autoscaling, before[a.Name]) {
					t.Fatalf("a refused sync changed %s's autoscaling: %+v", a.Name, a.Spec.Autoscaling)
				}
			}
		})
	}
}
