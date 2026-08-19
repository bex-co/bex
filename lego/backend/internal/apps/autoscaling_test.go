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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// autoscaling_test.go covers the autoscaling surface (w1/m20):
// SetAutoscaling / GetAutoscaling / DeleteAutoscaling across service, REST,
// GraphQL, and MCP.

func int32p(v int32) *int32 { return &v }

// mkAutoscaledApp creates an App with autoscaling already configured.
func mkAutoscaledApp() *appv1alpha1.App {
	a := sampleApp("web")
	a.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{
		Enabled:          true,
		MinReplicas:      1,
		MaxReplicas:      5,
		TargetCPUPercent: int32p(80),
	}
	return a
}

func TestGetAutoscalingReturnsSpec(t *testing.T) {
	svc, _ := newService(nil, mkAutoscaledApp())
	v, err := svc.GetAutoscaling(context.Background(), "web")
	if err != nil {
		t.Fatalf("GetAutoscaling: %v", err)
	}
	if !v.Enabled || v.MinInstances != 1 || v.MaxInstances != 5 {
		t.Errorf("unexpected view: %+v", v)
	}
	if v.TargetCPUPercent == nil || *v.TargetCPUPercent != 80 {
		t.Errorf("TargetCPUPercent = %v, want 80", v.TargetCPUPercent)
	}
}

func TestGetAutoscalingNotFound(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.GetAutoscaling(context.Background(), "nonexistent")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("expected core.ErrNotFound, got %v", err)
	}
}

func TestSetAutoscalingWritesToCR(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	req := SetAutoscalingRequest{
		MinInstances:     1,
		MaxInstances:     4,
		TargetCPUPercent: int32p(75),
	}
	v, err := svc.SetAutoscaling(context.Background(), "web", req)
	if err != nil {
		t.Fatalf("SetAutoscaling: %v", err)
	}
	if !v.Enabled {
		t.Fatal("expected enabled=true after SetAutoscaling")
	}
	if v.MaxInstances != 4 {
		t.Errorf("MaxInstances = %d, want 4", v.MaxInstances)
	}

	// Verify the CR was actually patched.
	app := getApp(t, cl, "web")
	if app.Spec.Autoscaling == nil {
		t.Fatal("spec.autoscaling is nil after set")
	}
	if app.Spec.Autoscaling.MaxReplicas != 4 {
		t.Errorf("spec.autoscaling.maxReplicas = %d, want 4", app.Spec.Autoscaling.MaxReplicas)
	}
}

func TestSetAutoscalingValidatesMinLeMax(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	req := SetAutoscalingRequest{
		MinInstances:     5,
		MaxInstances:     3, // min > max
		TargetCPUPercent: int32p(80),
	}
	_, err := svc.SetAutoscaling(context.Background(), "web", req)
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("expected bad-request, got %v", err)
	}
}

func TestSetAutoscalingRequiresAtLeastOneTarget(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	req := SetAutoscalingRequest{
		MinInstances: 1,
		MaxInstances: 3,
		// no CPU or memory target
	}
	_, err := svc.SetAutoscaling(context.Background(), "web", req)
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("expected bad-request (no target), got %v", err)
	}
}

func TestSetAutoscalingValidatesPercentRange(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	req := SetAutoscalingRequest{
		MinInstances:     1,
		MaxInstances:     3,
		TargetCPUPercent: int32p(0), // out of range
	}
	_, err := svc.SetAutoscaling(context.Background(), "web", req)
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("expected bad-request (percent=0), got %v", err)
	}
}

func TestSetAutoscalingRejectsAbovePlatformCeiling(t *testing.T) {
	// codex round-15 #1: autoscaling min/max must share the explicit-scale
	// ceiling; without it a can_operate caller could persist int32-max bounds
	// the operator would then project into Deployment.spec.replicas.
	svc, _ := newService(nil, sampleApp("web"))
	over := store.MaxReplicas + 1
	cpu := int32p(80)
	for _, req := range []SetAutoscalingRequest{
		{MinInstances: over, MaxInstances: over, TargetCPUPercent: cpu},
		{MinInstances: 1, MaxInstances: over, TargetCPUPercent: cpu},
	} {
		_, err := svc.SetAutoscaling(context.Background(), "web", req)
		if !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("SetAutoscaling(%+v) = %v, want bad-request at ceiling %d", req, err, store.MaxReplicas)
		}
	}
	// The ceiling itself is still accepted — the bound is inclusive.
	if _, err := svc.SetAutoscaling(context.Background(), "web", SetAutoscalingRequest{
		MinInstances: store.MaxReplicas, MaxInstances: store.MaxReplicas, TargetCPUPercent: cpu,
	}); err != nil {
		t.Fatalf("ceiling %d must be accepted: %v", store.MaxReplicas, err)
	}
}

func TestDeleteAutoscalingClearsSpec(t *testing.T) {
	svc, cl := newService(nil, mkAutoscaledApp())
	if err := svc.DeleteAutoscaling(context.Background(), "web"); err != nil {
		t.Fatalf("DeleteAutoscaling: %v", err)
	}
	app := getApp(t, cl, "web")
	if app.Spec.Autoscaling != nil {
		t.Errorf("spec.autoscaling should be nil after delete, got %+v", app.Spec.Autoscaling)
	}
}

func TestDeleteAutoscalingIdempotent(t *testing.T) {
	// App has no autoscaling configured — delete should succeed silently.
	svc, _ := newService(nil, sampleApp("web"))
	if err := svc.DeleteAutoscaling(context.Background(), "web"); err != nil {
		t.Fatalf("DeleteAutoscaling (idempotent): %v", err)
	}
}

func TestRESTGetAutoscaling(t *testing.T) {
	svc, _ := newService(nil, mkAutoscaledApp())
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/autoscaling", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET autoscaling => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		Enabled      bool  `json:"enabled"`
		MaxInstances int32 `json:"maxInstances"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.Enabled || out.MaxInstances != 5 {
		t.Errorf("unexpected body: %+v", out)
	}
}

func TestRESTPutAutoscaling(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"minInstances":2,"maxInstances":6,"targetCPUPercent":70}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PUT", "/v1/services/web/autoscaling", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT autoscaling => 200, got %d: %s", rec.Code, rec.Body)
	}
	app := getApp(t, cl, "web")
	if app.Spec.Autoscaling == nil || app.Spec.Autoscaling.MaxReplicas != 6 {
		t.Errorf("unexpected spec: %+v", app.Spec.Autoscaling)
	}
}

func TestRESTDeleteAutoscaling(t *testing.T) {
	svc, cl := newService(nil, mkAutoscaledApp())
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/v1/services/web/autoscaling", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE autoscaling => 204, got %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "web").Spec.Autoscaling != nil {
		t.Fatal("spec.autoscaling not cleared after DELETE")
	}
}

func TestGraphQLAutoscalingConfigQuery(t *testing.T) {
	svc, _ := newService(nil, mkAutoscaledApp())
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:  schema,
		Context: context.Background(), // graphql-go v0.8.1 Do does not default a nil Context; the resolver dereferences it
		RequestString: `{ autoscalingConfig(id:"web") {
			enabled minInstances maxInstances targetCPUPercent
		}}`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	cfg := res.Data.(map[string]any)["autoscalingConfig"].(map[string]any)
	if cfg["enabled"] != true {
		t.Errorf("enabled = %v, want true", cfg["enabled"])
	}
	// graphql-go resolves Int fields as int
	if cfg["maxInstances"] != 5 {
		t.Errorf("maxInstances = %v, want 5", cfg["maxInstances"])
	}
}

func TestGraphQLSetAutoscalingMutation(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:  schema,
		Context: context.Background(), // graphql-go v0.8.1 Do does not default a nil Context; the resolver dereferences it
		RequestString: `mutation {
			setAutoscaling(id:"web",minInstances:1,maxInstances:4,targetCPUPercent:80) {
				enabled maxInstances
			}
		}`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	app := getApp(t, cl, "web")
	if app.Spec.Autoscaling == nil || app.Spec.Autoscaling.MaxReplicas != 4 {
		t.Errorf("unexpected spec after mutation: %+v", app.Spec.Autoscaling)
	}
}

func TestGraphQLDisableAutoscalingMutation(t *testing.T) {
	svc, cl := newService(nil, mkAutoscaledApp())
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(), // graphql-go v0.8.1 Do does not default a nil Context; the resolver dereferences it
		RequestString: `mutation { disableAutoscaling(id:"web") }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	if getApp(t, cl, "web").Spec.Autoscaling != nil {
		t.Fatal("spec.autoscaling not cleared")
	}
}
