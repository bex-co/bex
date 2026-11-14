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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m364, Render's scale-service reference: "Render ignores this value as
// long as autoscaling is enabled for the service." REST, GraphQL and MCP share
// Service.Scale: with autoscaling on the count is accepted and stored as
// numInstances (the operator runs it once autoscaling is off), and no
// instance_count_changed is recorded, because the running count, which the
// autoscaler owns, did not change.
func TestScaleUnderAutoscalingStoresTheCountWithoutAnEvent(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			sink := &captureAuditSink{}
			cl := fakeClient(mkAutoscaledApp())
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Audit: sink}, DomainTraffic: allowDomainTraffic{}}
			switch surface {
			case "REST":
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services/web/scale", strings.NewReader(`{"numInstances":3}`)))
				if rec.Code != http.StatusAccepted {
					t.Fatalf("scale => %d, want 202: %s", rec.Code, rec.Body)
				}
				var out renderService
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Replicas != 3 {
					t.Fatalf("the scale answered replicas %d (%v), want the stored 3", out.Replicas, err)
				}
			case "GraphQL":
				res := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: t.Context(),
					RequestString: `mutation { scaleService(id: "web", numInstances: 3) { replicas } }`})
				if len(res.Errors) > 0 {
					t.Fatalf("gql: %v", res.Errors)
				}
			case "MCP":
				if res := callMCPTool(t, svc, "scale_service", map[string]any{"serviceId": "web", "numInstances": 3}); res.IsError {
					t.Fatalf("scale_service: %s", mcpResultText(res))
				}
			}
			if got := getApp(t, cl, "web").Spec.Replicas; got != 3 {
				t.Fatalf("spec.replicas = %d, want the stored 3", got)
			}
			if n := sink.countVerb(core.AuditVerbScale); n != 0 {
				t.Fatalf("a scale under autoscaling recorded %d instance_count_changed rows, want 0", n)
			}
		})
	}
}

// The same scale with autoscaling turned off changes the running count and
// records it, as before.
func TestScaleWithAutoscalingOffStillRecordsTheChange(t *testing.T) {
	app := mkAutoscaledApp()
	app.Spec.Autoscaling.Enabled = false // kept by a disable (w4/597)
	sink := &captureAuditSink{}
	svc := &Service{Base: &core.Base{Client: fakeClient(app), Namespace: "default", Audit: sink}}
	if _, err := svc.Scale(t.Context(), "web", 3); err != nil {
		t.Fatalf("Scale: %v", err)
	}
	if n := sink.countVerb(core.AuditVerbScale); n != 1 {
		t.Fatalf("recorded %d instance_count_changed rows, want 1", n)
	}
}

// w4/m364: the operator marks an autoscaled App AutoscalingMetricsAvailable=
// False while it cannot read utilization. The autoscaling read reports that as
// metricsUnavailable (a bex extension) on REST and GraphQL, so the Scaling
// page can say autoscaling is paused instead of implying it acts.
func TestAutoscalingReadReportsAPausedAutoscaler(t *testing.T) {
	paused := func(enabled bool) *appv1alpha1.App {
		app := mkAutoscaledApp()
		app.Spec.Autoscaling.Enabled = enabled
		app.Status.Conditions = []metav1.Condition{{
			Type: appv1alpha1.ConditionAutoscalingMetricsAvailable, Status: metav1.ConditionFalse,
			Reason: appv1alpha1.AutoscalingMetricsUnavailableReason, LastTransitionTime: metav1.Now(),
		}}
		return app
	}
	for name, tc := range map[string]struct {
		app  *appv1alpha1.App
		want bool
	}{
		"paused":          {app: paused(true), want: true},
		"metrics read":    {app: mkAutoscaledApp(), want: false},
		"autoscaling off": {app: paused(false), want: false},
	} {
		t.Run(name, func(t *testing.T) {
			svc, _ := newService(nil, tc.app)

			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/web/autoscaling", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET autoscaling => %d: %s", rec.Code, rec.Body)
			}
			var wire map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
				t.Fatal(err)
			}
			if got, present := wire["metricsUnavailable"]; tc.want != present || (present && got != true) {
				t.Fatalf("REST metricsUnavailable = %v (present %v), want %v; body %s", got, present, tc.want, rec.Body)
			}

			res := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: t.Context(),
				RequestString: `{ autoscalingConfig(id: "web") { metricsUnavailable } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			if got := res.Data.(map[string]any)["autoscalingConfig"].(map[string]any)["metricsUnavailable"]; got != tc.want {
				t.Fatalf("GraphQL metricsUnavailable = %v, want %v", got, tc.want)
			}
		})
	}
}
