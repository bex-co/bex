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
	"slices"
	"testing"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The service read must carry which linked group WINS a collision, because
// nothing else on the wire can express it: the workspace group list the
// dashboard already has records links as an unordered set (w1/091 — the
// Environment page listed the losing group first, by accident rather than by
// rule, and marked neither key).
//
// Precedence is spec.envFromSecrets order: the operator emits those Secrets as
// envFrom sources in order and Kubernetes lets the LAST win, so last linked
// wins. This is deliberate divergence from Render, which uses the most recently
// CREATED group (docs/ADR018-render-parity.md).
func TestLinkedEnvGroupIDsCarryPrecedenceOrderOnEveryAdapter(t *testing.T) {
	a := sampleApp("web")
	// Linked B first, then A — so A wins, and A must come LAST.
	a.Spec.EnvFromSecrets = []string{"evg-b-env", "evg-a-env"}
	// The service's own Secret is a different field and must never appear.
	a.Spec.EnvFromSecret = "web-env"
	svc, _ := newService(nil, a)
	want := []string{"evg-b", "evg-a"}

	// REST / MCP: both render the same renderService JSON shape.
	view, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !slices.Equal(view.LinkedEnvGroupIDs, want) {
		t.Fatalf("AppView.LinkedEnvGroupIDs = %v, want %v", view.LinkedEnvGroupIDs, want)
	}
	if got := svc.renderService(view).LinkedEnvGroupIDs; !slices.Equal(got, want) {
		t.Fatalf("renderService.linkedEnvGroupIds = %v, want %v", got, want)
	}

	// GraphQL: same field, same order.
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: `{ service(id:"web") { linkedEnvGroupIds } }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql errors: %v", res.Errors)
	}
	raw := res.Data.(map[string]any)["service"].(map[string]any)["linkedEnvGroupIds"].([]any)
	got := make([]string, 0, len(raw))
	for _, v := range raw {
		got = append(got, v.(string))
	}
	if !slices.Equal(got, want) {
		t.Fatalf("graphql linkedEnvGroupIds = %v, want %v", got, want)
	}
}

// A service with no linked groups reports nothing, and the service's own env
// Secret never leaks into the list even though it also ends in "-env".
func TestLinkedEnvGroupIDsOmitsTheServicesOwnSecret(t *testing.T) {
	a := sampleApp("web")
	a.Spec.EnvFromSecret = "web-env"
	if got := linkedEnvGroupIDs(a); len(got) != 0 {
		t.Fatalf("unlinked service reported groups: %v", got)
	}
	// A ref that is not a group projection at all is ignored rather than
	// producing a bogus id.
	if got := linkedEnvGroupIDs(&appv1alpha1.App{
		Spec: appv1alpha1.AppSpec{EnvFromSecrets: []string{"-env", "evg-1-env", "unrelated"}},
	}); !slices.Equal(got, []string{"evg-1"}) {
		t.Fatalf("linkedEnvGroupIDs = %v, want [evg-1]", got)
	}
}
