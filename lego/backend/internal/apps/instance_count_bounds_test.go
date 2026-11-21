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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// instanceCountRefusal is the refusal for an instance count outside Render's
// 1-100 (pinned numInstances minimum 1; Render's scaling cap), under the name
// the caller sent it (w8/364).
func instanceCountRefusal(field string) string {
	return fmt.Sprintf("%s must be 1-%d", field, store.MaxReplicas)
}

// outOfRangeInstanceCounts are below Render's minimum (an explicit 0
// included: only an omitted count defaults to 1) and above its cap.
var outOfRangeInstanceCounts = []int{-1, 0, int(store.MaxReplicas) + 1}

// A REST create (and its dry run) refuses serviceDetails.numInstances outside
// 1-100 by Render's name, creating nothing; 1 and an omitted count create one
// instance.
func TestRESTCreateRefusesNumInstancesOutsideRendersRange(t *testing.T) {
	post := func(t *testing.T, query, details string) (*httptest.ResponseRecorder, client.Client) {
		svc, cl := newService(nil)
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		body := `{"name":"web","type":"web_service","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"starter"` + details + `}}`
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services"+query, strings.NewReader(body)))
		return rec, cl
	}
	for _, n := range outOfRangeInstanceCounts {
		for _, query := range []string{"", "?dryRun=true"} {
			rec, cl := post(t, query, fmt.Sprintf(`,"numInstances":%d`, n))
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), instanceCountRefusal("numInstances")) {
				t.Errorf("numInstances %d%s => %d %s, want 400 %q", n, query, rec.Code, rec.Body, instanceCountRefusal("numInstances"))
			}
			if got := countApps(t, cl); got != 0 {
				t.Errorf("numInstances %d%s created %d services, want none", n, query, got)
			}
		}
	}
	for _, details := range []string{`,"numInstances":1`, ""} {
		rec, cl := post(t, "", details)
		if rec.Code != http.StatusCreated {
			t.Fatalf("serviceDetails %q => %d %s, want 201", details, rec.Code, rec.Body)
		}
		if got := getApp(t, cl, "web").Spec.Replicas; got != 1 {
			t.Errorf("serviceDetails %q => replicas %d, want 1", details, got)
		}
	}
}

// GraphQL's createService names its count replicas, and is held to the same
// range under that name.
func TestGraphQLCreateServiceRefusesReplicasOutsideRendersRange(t *testing.T) {
	for _, n := range outOfRangeInstanceCounts {
		svc, cl := newService(nil)
		res := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: context.Background(),
			RequestString: fmt.Sprintf(`mutation { createService(name: "web", image: "nginx:1", replicas: %d) { name } }`, n)})
		if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, instanceCountRefusal("replicas")) {
			t.Errorf("replicas %d => %v, want %q", n, res.Errors, instanceCountRefusal("replicas"))
		}
		if got := countApps(t, cl); got != 0 {
			t.Errorf("replicas %d created %d services, want none", n, got)
		}
	}
	svc, cl := newService(nil)
	res := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: context.Background(),
		RequestString: `mutation { createService(name: "web", image: "nginx:1", replicas: 1) { name } }`})
	if len(res.Errors) != 0 || getApp(t, cl, "web").Spec.Replicas != 1 {
		t.Fatalf("replicas 1 => %v, want one instance", res.Errors)
	}
}

// MCP's create_web_service names its count replicas, likewise.
func TestMCPCreateWebServiceRefusesReplicasOutsideRendersRange(t *testing.T) {
	for _, n := range outOfRangeInstanceCounts {
		svc, cl := newService(nil)
		res := callMCPTool(t, svc, "create_web_service", map[string]any{"name": "web", "image": "nginx:1", "runtime": "image", "replicas": n})
		if !res.IsError || !strings.Contains(mcpResultText(res), instanceCountRefusal("replicas")) {
			t.Errorf("replicas %d => %q (error %v), want %q", n, mcpResultText(res), res.IsError, instanceCountRefusal("replicas"))
		}
		if got := countApps(t, cl); got != 0 {
			t.Errorf("replicas %d created %d services, want none", n, got)
		}
	}
	svc, cl := newService(nil)
	if res := callMCPTool(t, svc, "create_web_service", map[string]any{"name": "web", "image": "nginx:1", "runtime": "image", "replicas": 1}); res.IsError {
		t.Fatalf("replicas 1 => %s", mcpResultText(res))
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 1 {
		t.Errorf("replicas 1 => %d instances", got)
	}
}

// A Blueprint's numInstances above Render's cap fails validation at the field
// in Render's words, and an apply refuses it the same way for a new service
// and for one the file already manages (the pinned schema refuses 0 and
// below before either).
func TestBlueprintNumInstancesAboveRendersCap(t *testing.T) {
	manifest := func(n int) string {
		return fmt.Sprintf("services:\n  - type: web\n    name: web\n    runtime: image\n    image:\n      url: docker.io/library/nginx:1.27-alpine\n    plan: starter\n    numInstances: %d\n", n)
	}
	over := manifest(int(store.MaxReplicas) + 1)
	svc, cl := newService(nil)
	validation, err := svc.ValidateBlueprint(context.Background(), "", over, "")
	if err != nil || validation.Valid || len(validation.Errors) != 1 {
		t.Fatalf("validation = %+v, %v", validation, err)
	}
	problem := validation.Errors[0]
	if problem.Path == nil || *problem.Path != "services[0].numInstances" || !strings.Contains(problem.Error, instanceCountRefusal("numInstances")) {
		t.Fatalf("want %q at services[0].numInstances, got %q at %v", instanceCountRefusal("numInstances"), problem.Error, ptrValue(problem.Path))
	}
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: over}); err == nil || !strings.Contains(err.Error(), instanceCountRefusal("numInstances")) {
		t.Fatalf("apply of a new service = %v, want %q", err, instanceCountRefusal("numInstances"))
	}
	if got := countApps(t, cl); got != 0 {
		t.Fatalf("a refused apply created %d services", got)
	}
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest(2)}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: over}); err == nil || !strings.Contains(err.Error(), instanceCountRefusal("numInstances")) {
		t.Fatalf("apply to a managed service = %v, want %q", err, instanceCountRefusal("numInstances"))
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 2 {
		t.Errorf("a refused apply left replicas %d, want 2", got)
	}
}
