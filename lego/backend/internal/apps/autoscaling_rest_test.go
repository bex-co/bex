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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func putAutoscaling(t *testing.T, svc *Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodPut, "/v1/services/web/autoscaling", strings.NewReader(body))
	req = req.WithContext(core.WithStrictJSONDecoding(core.WithIdentity(req.Context(), core.Identity{
		Subject: "tester", Method: "session", Human: true,
	})))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

// TestPUTAutoscalingAcceptsRenderBody pins w4/m104: the handler decodes
// Render's pinned schema and stores the same config GraphQL would.
func TestPUTAutoscalingAcceptsRenderBody(t *testing.T) {
	svc, _ := newService(nil, freeWebApp("web"))

	body := `{
		"enabled": true,
		"min": 1,
		"max": 1,
		"criteria": {
			"cpu": {"enabled": true, "percentage": 60},
			"memory": {"enabled": false, "percentage": 0}
		}
	}`
	w := putAutoscaling(t, svc, body)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body = %s", w.Code, w.Body.String())
	}
	var got renderAutoscalingConfig
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.Enabled || got.Min != 1 || got.Max != 1 || !got.Criteria.CPU.Enabled || got.Criteria.CPU.Percentage != 60 {
		t.Fatalf("response = %+v", got)
	}

	view, err := svc.GetAutoscaling(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	if !view.Enabled || view.MinInstances != 1 || view.MaxInstances != 1 || view.TargetCPUPercent == nil || *view.TargetCPUPercent != 60 {
		t.Fatalf("stored view = %+v", view)
	}

	// GraphQL-shaped write of the same intent stays consistent.
	cpu := int32(60)
	gql, err := svc.SetAutoscaling(context.Background(), "web", SetAutoscalingRequest{
		MinInstances: 1, MaxInstances: 1, TargetCPUPercent: &cpu,
	})
	if err != nil {
		t.Fatalf("GraphQL-shaped SetAutoscaling: %v", err)
	}
	if gql.MinInstances != got.Min || gql.MaxInstances != got.Max || gql.TargetCPUPercent == nil || *gql.TargetCPUPercent != 60 {
		t.Fatalf("GraphQL view = %+v vs REST %+v", gql, got)
	}
}

func TestPUTAutoscalingRejectsBexDialectWithNamedError(t *testing.T) {
	svc, _ := newService(nil, freeWebApp("web"))
	// Without the OpenAPI gate, the handler's strict decoder names the unknown
	// fields rather than answering a bare bad request (w4/m104 t003).
	w := putAutoscaling(t, svc, `{"minInstances":1,"maxInstances":1,"targetCPUPercent":60}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	msg := w.Body.String()
	if !strings.Contains(msg, "unknown field") {
		t.Fatalf("body = %q, want named unknown-field detail", msg)
	}
	if msg == `{"error":"bad request","id":"bad_request","message":"bad request"}`+"\n" {
		t.Fatal("bare bad request is unreachable for body rejection")
	}
}

func TestPUTAutoscalingEnabledFalseDisables(t *testing.T) {
	svc, _ := newService(nil, freeWebApp("web"))
	cpu := int32(50)
	if _, err := svc.SetAutoscaling(context.Background(), "web", SetAutoscalingRequest{
		MinInstances: 1, MaxInstances: 1, TargetCPUPercent: &cpu,
	}); err != nil {
		t.Fatal(err)
	}
	body := `{"enabled":false,"min":0,"max":1,"criteria":{"cpu":{"enabled":false,"percentage":0},"memory":{"enabled":false,"percentage":0}}}`
	w := putAutoscaling(t, svc, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	view, err := svc.GetAutoscaling(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	if view.Enabled {
		t.Fatalf("still enabled: %+v", view)
	}
}

func TestPUTAutoscalingUnionBodyNamesUnknownFields(t *testing.T) {
	// The production anomaly: a body carrying both dialects passed OpenAPI then
	// failed DecodeBody. After the DecodeBody detail fix the refusal names a
	// field rather than returning bare "bad request".
	r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte(`{
		"enabled":true,"min":1,"max":1,
		"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":0}},
		"minInstances":1,"maxInstances":1,"targetCPUPercent":60
	}`)))
	r = r.WithContext(core.WithStrictJSONDecoding(context.Background()))
	_, err := core.DecodeBody[renderAutoscalingConfig](r)
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("err = %v, want unknown-field detail (union anomaly explanation)", err)
	}
}
