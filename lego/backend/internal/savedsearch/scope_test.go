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

package savedsearch

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type stubScopeLister struct {
	workspace string
	resources []Resource
	err       error
}

func (s stubScopeLister) ProjectScopeResources(_ context.Context, projectID, envID string) (string, []Resource, error) {
	if s.err != nil {
		return "", nil, s.err
	}
	if projectID == "prj-missing" {
		return "", nil, core.NotFound("project")
	}
	if envID == "evm-missing" {
		return "", nil, core.NotFound("environment")
	}
	if envID == "evm-wrong" && projectID != "" {
		return "", nil, fmt.Errorf("%w: environment does not belong to project", core.ErrBadRequest)
	}
	if envID == "" {
		return s.workspace, s.resources, nil
	}
	var out []Resource
	for _, r := range s.resources {
		if r.EnvironmentID == envID {
			out = append(out, r)
		}
	}
	return s.workspace, out, nil
}

func TestResolveScopeRules(t *testing.T) {
	lister := stubScopeLister{
		workspace: "tea-1",
		resources: []Resource{
			{ID: "srv-web", Name: "web", Type: "web_service", EnvironmentID: "evm-prod"},
			{ID: "srv-static", Name: "docs", Type: "static_site", EnvironmentID: "evm-prod"},
			{ID: "dpg-db", Name: "db", Type: KindPostgres, EnvironmentID: "evm-staging"},
		},
	}
	logsPolicy := ScopePolicy{
		RequireProjectForEnvironment: true,
		MaxResources:                 20,
		Keep:                         func(r Resource) bool { return r.ResourceType() != "" },
	}
	metricsPolicy := ScopePolicy{
		RequireProjectForEnvironment: false,
		MaxResources:                 50,
		Keep:                         func(r Resource) bool { return r.Type != "static_site" },
	}

	// Logs: environment or region without project -> 400.
	if _, err := ResolveScope(context.Background(), lister, ScopeRequest{Environment: "evm-prod"}, logsPolicy); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("logs env without project = %v, want 400", err)
	}
	if _, err := ResolveScope(context.Background(), lister, ScopeRequest{Region: "oregon"}, logsPolicy); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("logs region without project = %v, want 400", err)
	}
	if _, err := ResolveScope(context.Background(), lister, ScopeRequest{Project: "prj-1", Region: "fsn1"}, logsPolicy); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("invalid region fsn1 = %v, want 400", err)
	}

	// Logs: project alone skips static site.
	got, err := ResolveScope(context.Background(), lister, ScopeRequest{Project: "prj-1", Region: "oregon"}, logsPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Scoped || !slices.Equal(got.IDs, []string{"srv-web", "dpg-db"}) {
		t.Fatalf("logs project scope = %+v", got)
	}

	// Logs: project + environment narrows to evm-prod.
	got, err = ResolveScope(context.Background(), lister, ScopeRequest{Project: "prj-1", Environment: "evm-prod"}, logsPolicy)
	if err != nil || !slices.Equal(got.IDs, []string{"srv-web"}) {
		t.Fatalf("logs project+env = %+v, %v", got, err)
	}

	// Logs: project + explicit resource narrows; outside resource is 400.
	got, err = ResolveScope(context.Background(), lister, ScopeRequest{Project: "prj-1", Resources: []string{"dpg-db", "dpg-db"}}, logsPolicy)
	if err != nil || !slices.Equal(got.IDs, []string{"dpg-db"}) {
		t.Fatalf("logs project+resource = %+v, %v", got, err)
	}
	if _, err := ResolveScope(context.Background(), lister, ScopeRequest{Project: "prj-1", Resources: []string{"srv-outside"}}, logsPolicy); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("outside resource = %v, want 400", err)
	}

	// Metrics: environment alone is allowed, region with environment is allowed.
	got, err = ResolveScope(context.Background(), lister, ScopeRequest{Environment: "evm-staging", Region: "frankfurt"}, metricsPolicy)
	if err != nil || !slices.Equal(got.IDs, []string{"dpg-db"}) {
		t.Fatalf("metrics env alone = %+v, %v", got, err)
	}
	// Metrics: region without project or environment -> 400.
	if _, err := ResolveScope(context.Background(), lister, ScopeRequest{Region: "frankfurt"}, metricsPolicy); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("metrics region alone = %v, want 400", err)
	}
	// Metrics: unknown project/environment -> 404.
	if _, err := ResolveScope(context.Background(), lister, ScopeRequest{Project: "prj-missing"}, metricsPolicy); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("missing project = %v, want 404", err)
	}
	if _, err := ResolveScope(context.Background(), lister, ScopeRequest{Environment: "evm-missing"}, metricsPolicy); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("missing env = %v, want 404", err)
	}
	// Metrics: environment outside project -> 400.
	if _, err := ResolveScope(context.Background(), lister, ScopeRequest{Project: "prj-1", Environment: "evm-wrong"}, metricsPolicy); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("env outside project = %v, want 400", err)
	}

	// Cap emits structured too_many_resources code.
	many := make([]Resource, 21)
	for i := range many {
		many[i] = Resource{ID: fmt.Sprintf("srv-%02d", i), Type: "web_service"}
	}
	_, err = ResolveScope(context.Background(), stubScopeLister{workspace: "tea-1", resources: many}, ScopeRequest{Project: "prj-1"}, logsPolicy)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != TooManyResourcesCode {
		t.Fatalf("over cap = %v, want coded %s", err, TooManyResourcesCode)
	}
}
