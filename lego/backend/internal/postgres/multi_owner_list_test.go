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

package postgres

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/m168: Render's GET /v1/postgres takes ownerId as an array.

func tenantDatabase(id, tenant string) *appv1alpha1.Database {
	return &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: id, Namespace: tenant,
		Labels: map[string]string{core.LabelTenant: tenant, core.LabelWorkspace: tenant},
	}, Spec: appv1alpha1.DatabaseSpec{Name: "db"}}
}

func listPostgresIDs(t *testing.T, svc *Service, query string) (int, []string, string) {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/postgres?"+query, nil).WithContext(ctxAs("user-a")))
	var out []struct {
		Postgres struct {
			ID string `json:"id"`
		} `json:"postgres"`
	}
	var got []string
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		for _, o := range out {
			got = append(got, o.Postgres.ID)
		}
		slices.Sort(got)
	}
	return rec.Code, got, rec.Body.String()
}

func TestPostgresListHonorsOwnerArrays(t *testing.T) {
	svc, _ := newService(tenantDatabase("dpg-1", "tea-1"), tenantDatabase("dpg-2", "tea-2"), tenantDatabase("dpg-3", "tea-3"))
	svc.Authz = &fakeChecker{allow: true}
	svc.Workspace = coretest.Members{"user-a": {"tea-1", "tea-2"}}

	for _, query := range []string{"ownerId=tea-1&ownerId=tea-2", "ownerId=tea-1,tea-2", "ownerId=tea-1,tea-2&limit=5"} {
		if code, got, body := listPostgresIDs(t, svc, query); code != http.StatusOK || !slices.Equal(got, []string{"dpg-1", "dpg-2"}) {
			t.Errorf("%s = %d %v: %s", query, code, got, body)
		}
	}
	code, _, body := listPostgresIDs(t, svc, "ownerId=tea-1,tea-3")
	if code != http.StatusForbidden || strings.Contains(body, "dpg-") {
		t.Errorf("a forbidden owner in the mix = %d, want 403 with nothing disclosed: %s", code, body)
	}
}
