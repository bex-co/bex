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
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/graphql-go/graphql"
)

// Fake Kubernetes intentionally has no CRD CEL validation: refusal here proves
// the API checks disk invariants before either durable-row or CR writes.
func TestDiskInvariantsRefusedAcrossSurfaces(t *testing.T) {
	cases := []struct {
		name, method, path, body, mutation, tool string
		args                                     map[string]any
	}{
		{"scale", http.MethodPost, "/v1/services/web/scale", `{"numInstances":3}`, `mutation {scaleService(id:"web",numInstances:3){id}}`, "scale_service", map[string]any{"serviceId": "web", "numInstances": 3}},
		{"free plan", http.MethodPatch, "/v1/services/web", `{"serviceDetails":{"plan":"free"}}`, `mutation {updateServicePlan(id:"web",plan:"free"){id}}`, "update_service", map[string]any{"serviceId": "web", "plan": "free"}},
		{"free plan preview", http.MethodPatch, "/v1/services/web", `{"serviceDetails":{"plan":"free"},"dryRun":true}`, `mutation {updateServicePlan(id:"web",plan:"free",dryRun:true){id}}`, "update_service", map[string]any{"serviceId": "web", "plan": "free", "dryRun": true}},
		{"autoscaling", http.MethodPut, "/v1/services/web/autoscaling", `{"enabled":true,"min":1,"max":1,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":0}}}`, `mutation {setAutoscaling(id:"web",minInstances:1,maxInstances:1,targetCPUPercent:60){enabled}}`, "update_service", map[string]any{"serviceId": "web", "autoscaling": map[string]any{"minInstances": 1, "maxInstances": 1, "targetCPUPercent": 60}}},
	}
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, tc := range cases {
			t.Run(surface+"/"+tc.name, func(t *testing.T) {
				app := diskEligibleApp("web")
				app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
				svc, cl, st := newDiskService(app)
				before := getApp(t, cl, "web").DeepCopy()
				var message string
				switch surface {
				case "REST":
					mux := http.NewServeMux()
					svc.RegisterREST(mux)
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
					if rec.Code != http.StatusBadRequest {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
					}
					message = rec.Body.String()
				case "GraphQL":
					result := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: t.Context(), RequestString: tc.mutation})
					if len(result.Errors) != 1 {
						t.Fatalf("expected one domain refusal: %+v", result)
					}
					message = result.Errors[0].Message
				case "MCP":
					message = callAppsMCPError(t, svc, tc.tool, tc.args)
				}
				if !strings.Contains(strings.ToLower(message), "disk") {
					t.Fatalf("expected disk-specific refusal: %s", message)
				}
				if len(st.tierCalls) != 0 || len(st.replicasCalls) != 0 || len(st.deployCalls) != 0 {
					t.Fatalf("refused operation wrote row/deploy: tier=%d replicas=%d deploy=%d", len(st.tierCalls), len(st.replicasCalls), len(st.deployCalls))
				}
				if after := getApp(t, cl, "web"); !reflect.DeepEqual(before, after) {
					t.Fatal("refused operation modified CR")
				}
			})
		}
	}
}
