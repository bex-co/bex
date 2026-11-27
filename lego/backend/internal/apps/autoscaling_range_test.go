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
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/739: Render's Blueprint schema (serverService.scaling) requires
// minInstances ≥ 1 and bounds targetCPUPercent/targetMemoryPercent at 90, and
// its dashboard's sliders run 1–100 instances and 1–90%. bex accepted
// minInstances 0 (which the operator runs as 1, so the read did not match
// what ran) and targets up to 100 on REST, GraphQL and MCP.

// autoscalingWrite is one surface's autoscaling write of min instances and a
// CPU target; it returns the refusal message, or "" when the write succeeded.
type autoscalingWrite func(t *testing.T, svc *Service, minInstances, cpu int) string

func restAutoscalingWrite(t *testing.T, svc *Service, minInstances, cpu int) string {
	rec := putAutoscaling(t, svc, fmt.Sprintf(`{"enabled":true,"min":%d,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":%d},"memory":{"enabled":false,"percentage":0}}}`, minInstances, cpu))
	if rec.Code == http.StatusOK {
		return ""
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT …/autoscaling min %d cpu %d = %d %s, want 200 or 400", minInstances, cpu, rec.Code, rec.Body.String())
	}
	var out struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Message
}

func gqlAutoscalingWrite(t *testing.T, svc *Service, minInstances, cpu int) string {
	res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: context.Background(), RequestString: fmt.Sprintf(
		`mutation { setAutoscaling(id: "web", minInstances: %d, maxInstances: 3, targetCPUPercent: %d) { enabled } }`, minInstances, cpu)})
	if len(res.Errors) == 0 {
		return ""
	}
	return res.Errors[0].Message
}

func mcpAutoscalingWrite(t *testing.T, svc *Service, minInstances, cpu int) string {
	res := callMCPTool(t, svc, "update_service", map[string]any{"serviceId": "web", "autoscaling": map[string]any{
		"minInstances": minInstances, "maxInstances": 3, "targetCPUPercent": cpu,
	}})
	if !res.IsError {
		return ""
	}
	return mcpResultText(res)
}

// A minimum below 1 or a target above 90 is refused on REST, GraphQL and MCP
// with a message naming Render's range, and stores nothing; Render's own
// bounds, 1 instance and 90%, are accepted and read back as written.
func TestAutoscalingRangesAreRendersOnEverySurface(t *testing.T) {
	surfaces := map[string]autoscalingWrite{"REST": restAutoscalingWrite, "GraphQL": gqlAutoscalingWrite, "MCP": mcpAutoscalingWrite}
	refusals := []struct {
		name         string
		minInstances int
		cpu          int
		want         string
	}{
		{"minInstances 0", 0, 60, "minInstances must be 1–100"},
		{"minInstances -1", -1, 60, "minInstances must be 1–100"},
		{"target 95", 1, 95, "targetCPUPercent must be 1–90"},
		{"target 91", 1, 91, "targetCPUPercent must be 1–90"},
	}
	for surface, write := range surfaces {
		for _, tc := range refusals {
			t.Run(surface+"/"+tc.name, func(t *testing.T) {
				svc, cl := newService(nil, paidWebApp("web"))
				if msg := write(t, svc, tc.minInstances, tc.cpu); !strings.Contains(msg, tc.want) {
					t.Fatalf("refusal = %q, want it to name %q", msg, tc.want)
				}
				if got := getApp(t, cl, "web").Spec.Autoscaling; got != nil {
					t.Fatalf("a refused write stored %+v", got)
				}
			})
		}
		t.Run(surface+"/Render's bounds", func(t *testing.T) {
			svc, _ := newService(nil, paidWebApp("web"))
			if msg := write(t, svc, 1, 90); msg != "" {
				t.Fatalf("minInstances 1 with a 90%% target refused: %s", msg)
			}
			view, err := svc.GetAutoscaling(context.Background(), "web")
			if err != nil {
				t.Fatal(err)
			}
			if !view.Enabled || view.MinInstances != 1 || view.TargetCPUPercent == nil || *view.TargetCPUPercent != 90 {
				t.Fatalf("read after the write = %+v, want min 1 and cpu 90", view)
			}
		})
	}
}

// The ceiling holds for memory as for CPU, for the percentage a turned-off
// criterion keeps (the slider value Render's dashboard keeps is at most 90),
// and for a create carrying serviceDetails.autoscaling, which shares the
// validation.
func TestAutoscalingMemoryKeptAndCreateRanges(t *testing.T) {
	for body, want := range map[string]string{
		`{"enabled":true,"min":1,"max":2,"criteria":{"cpu":{"enabled":false,"percentage":0},"memory":{"enabled":true,"percentage":95}}}`:  "targetMemoryPercent must be 1–90",
		`{"enabled":true,"min":1,"max":2,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":91}}}`: "criteria.memory.percentage must be 0–90",
	} {
		svc, _ := newService(nil, paidWebApp("web"))
		if rec := putAutoscaling(t, svc, body); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("PUT %s = %d %s, want 400 naming %q", body, rec.Code, rec.Body.String(), want)
		}
	}
	svc, _ := newService(nil, paidWebApp("web"))
	if rec := putAutoscaling(t, svc, `{"enabled":true,"min":1,"max":2,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":90}}}`); rec.Code != http.StatusOK {
		t.Errorf("a turned-off memory criterion kept at 90 = %d %s, want 200", rec.Code, rec.Body.String())
	}

	cpu := int32(60)
	create := CreateRequest{Name: "web2", Type: appv1alpha1.TypeWebService, Image: "nginx:1", Plan: "starter", Replicas: 1,
		Autoscaling: &SetAutoscalingRequest{MinInstances: 0, MaxInstances: 3, TargetCPUPercent: &cpu}}
	if _, err := specFromCreate(create); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "minInstances must be 1–100") {
		t.Errorf("create with minInstances 0 = %v, want the range refusal", err)
	}
}

// A Blueprint's scaling block meets the same ranges at validate, located at
// the field: minInstances 0 and a target of 95 are refused, and a block that
// leaves out minInstances (which Render's Blueprint spec marks Required) is
// refused rather than stored as 0.
func TestBlueprintScalingRangesAreRenders(t *testing.T) {
	const head = "services:\n  - type: web\n    name: auto\n    runtime: image\n    image:\n      url: docker.io/traefik/whoami:v1.10.3\n    plan: starter\n    scaling:\n"
	for name, tc := range map[string]struct{ scaling, path, want string }{
		"minInstances 0":       {"      minInstances: 0\n      maxInstances: 3\n      targetCPUPercent: 60\n", "services[0].scaling.minInstances", "want 1"},
		"target 95":            {"      minInstances: 1\n      maxInstances: 3\n      targetMemoryPercent: 95\n", "services[0].scaling.targetMemoryPercent", "want 90"},
		"minInstances omitted": {"      maxInstances: 3\n      targetCPUPercent: 60\n", "services[0]", "minInstances must be 1–100"},
	} {
		t.Run(name, func(t *testing.T) {
			svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
			validation, err := svc.ValidateBlueprint(context.Background(), "", head+tc.scaling, "")
			if err != nil || validation.Valid || len(validation.Errors) != 1 {
				t.Fatalf("validation = %+v, %v; want one refusal", validation, err)
			}
			problem := validation.Errors[0]
			if ptrValue(problem.Path) != tc.path || !strings.Contains(problem.Error, tc.want) {
				t.Fatalf("refusal at %v: %q, want %s naming %q", ptrValue(problem.Path), problem.Error, tc.path, tc.want)
			}
		})
	}
}
