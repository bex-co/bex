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

package keyvalue

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/m168: Render's GET /v1/key-value takes ownerId as an array.

type memberships map[string][]string

func (m memberships) Tenant(_ context.Context, id core.Identity) (string, bool) {
	if ws := m[id.Subject]; len(ws) > 0 {
		return ws[0], true
	}
	return "", false
}

func (m memberships) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	return slices.Contains(m[id.Subject], tenantID), nil
}

func tenantKeyValue(id, tenant string) *appv1alpha1.KeyValue {
	kv := sampleKeyValue(id)
	kv.Namespace = tenant
	kv.Labels = map[string]string{core.LabelTenant: tenant, core.LabelWorkspace: tenant}
	return kv
}

func TestKeyValueListHonorsOwnerArrays(t *testing.T) {
	svc, _ := newService(
		tenantKeyValue("red-aaaaaaaaaaaaaaaaaaaa", "tea-1"),
		tenantKeyValue("red-bbbbbbbbbbbbbbbbbbbb", "tea-2"),
		tenantKeyValue("red-cccccccccccccccccccc", "tea-3"),
	)
	svc.Authz = &fakeChecker{allow: true}
	svc.Workspace = memberships{"user-a": {"tea-1", "tea-2"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	list := func(query string) (int, []string, string) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/key-value?"+query, nil).WithContext(ctxAs("user-a")))
		var out []struct {
			KeyValue struct {
				ID string `json:"id"`
			} `json:"keyValue"`
		}
		var got []string
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			for _, o := range out {
				got = append(got, o.KeyValue.ID)
			}
			slices.Sort(got)
		}
		return rec.Code, got, rec.Body.String()
	}

	want := []string{"red-aaaaaaaaaaaaaaaaaaaa", "red-bbbbbbbbbbbbbbbbbbbb"}
	for _, query := range []string{"ownerId=tea-1&ownerId=tea-2", "ownerId=tea-1,tea-2", "ownerId=tea-2,tea-1&limit=5"} {
		if code, got, body := list(query); code != http.StatusOK || !slices.Equal(got, want) {
			t.Errorf("%s = %d %v: %s", query, code, got, body)
		}
	}
	if code, _, body := list("ownerId=tea-1,tea-3"); code != http.StatusForbidden || strings.Contains(body, "red-") {
		t.Errorf("a forbidden owner in the mix = %d, want 403 with nothing disclosed: %s", code, body)
	}
}
