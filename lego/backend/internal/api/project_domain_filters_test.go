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

package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type listFilterDomainStore struct {
	*store.PGStore
	rows []store.Domain
}

func (s listFilterDomainStore) ListDomainClaims(context.Context, string) ([]store.Domain, error) {
	return s.rows, nil
}

func TestGatedProjectAndDomainListFilters(t *testing.T) {
	epoch := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	app := sampleApp("web")
	app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-web"}
	base := &core.Base{Client: fakeClient(app), Namespace: "default"}
	h, srv := serverWith(t, base, Deps{ProjectsStore: newConformProjectStore(
		store.Project{ID: "prj-a", TenantID: "tea-1", Name: "alpha", CreatedAt: epoch, UpdatedAt: epoch.Add(24 * time.Hour)},
		store.Project{ID: "prj-b", TenantID: "tea-1", Name: "beta", CreatedAt: epoch.Add(48 * time.Hour), UpdatedAt: epoch.Add(72 * time.Hour)},
	)})
	srv.Apps.Store = listFilterDomainStore{rows: []store.Domain{
		{Host: "alpha.example.com", ClaimState: "pending", CreatedAt: epoch},
		{Host: "beta.example.com", ClaimState: "pending", CreatedAt: epoch.Add(48 * time.Hour)},
	}}
	for _, tc := range []struct {
		path, query string
		want        int
	}{
		{"/v1/projects?ownerId=tea-1", "", 2},
		{"/v1/projects?ownerId=tea-1", "&name=missing", 0},
		{"/v1/projects?ownerId=tea-1", "&name=alpha", 1},
		{"/v1/projects?ownerId=tea-1", "&name=alpha&name=beta", 2},
		{"/v1/projects?ownerId=tea-1", "&name=alph", 0},
		{"/v1/projects?ownerId=tea-1", "&createdAfter=2026-01-02T00:00:00Z", 1},
		{"/v1/projects?ownerId=tea-1", "&createdBefore=2026-01-04T00:00:00Z", 1},
		{"/v1/projects?ownerId=tea-1", "&updatedAfter=2026-01-03T00:00:00Z", 1},
		{"/v1/projects?ownerId=tea-1", "&updatedBefore=2026-01-05T00:00:00Z", 1},
		{"/v1/projects?ownerId=tea-1", "&name=alpha&createdAfter=2030-01-01T00:00:00Z", 0},
		{"/v1/projects?ownerId=tea-1", "&name=alpha&limit=1", 1},
		{"/v1/services/web/custom-domains?", "", 2},
		{"/v1/services/web/custom-domains?", "name=missing.example.com", 0},
		{"/v1/services/web/custom-domains?", "name=alpha.example.com", 1},
		{"/v1/services/web/custom-domains?", "name=alpha.example.com&name=beta.example.com", 2},
		{"/v1/services/web/custom-domains?", "name=alpha", 0},
		{"/v1/services/web/custom-domains?", "createdAfter=2026-01-02T00:00:00Z", 1},
		{"/v1/services/web/custom-domains?", "createdBefore=2026-01-04T00:00:00Z", 1},
		{"/v1/services/web/custom-domains?", "createdAfter=2030-01-01T00:00:00Z", 0},
		{"/v1/services/web/custom-domains?", "name=beta.example.com&limit=1", 1},
	} {
		t.Run(tc.path+tc.query, func(t *testing.T) {
			w := do(t, h, "GET", tc.path+tc.query, testToken, "")
			if w.Code != 200 {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var rows []json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != tc.want {
				t.Fatalf("got %d rows, want %d: %s", len(rows), tc.want, w.Body.String())
			}
		})
	}
	for _, endpoint := range []struct {
		path string
		keys []string
	}{
		{"/v1/projects?ownerId=tea-1&", []string{"createdBefore", "createdAfter", "updatedBefore", "updatedAfter"}},
		{"/v1/services/web/custom-domains?", []string{"createdBefore", "createdAfter"}},
	} {
		for _, key := range endpoint.keys {
			w := do(t, h, "GET", endpoint.path+key+"=yesterday", testToken, "")
			if w.Code != 400 || !strings.Contains(w.Body.String(), key) {
				t.Fatalf("%s: %d %s", key, w.Code, w.Body.String())
			}
		}
	}
	// Legacy storeless domains have no per-host creation timestamp. A window
	// must preserve the shared missing-timestamp semantics rather than invent one.
	srv.Apps.Store = nil
	app.Spec.Hosts = []string{"legacy.example.com"}
	if err := base.Client.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	w := do(t, h, "GET", "/v1/services/web/custom-domains?createdAfter=2030-01-01T00:00:00Z", testToken, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "legacy.example.com") {
		t.Fatalf("legacy domain: %d %s", w.Code, w.Body.String())
	}
}
