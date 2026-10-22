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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// w8/191: Render's unfiltered GET /notification-settings/overrides lists "all
// notification overrides for all workspaces the user belongs to"; ownerId
// narrows it to one workspace.
func TestUnfilteredOverridesSpanTheCallersWorkspaces(t *testing.T) {
	a1, b1 := webApp("tea-a", "web", "nginx:1"), webApp("tea-b", "api", "nginx:1")
	svc, _ := newTenantStoreService(coretest.Workspaces{"tea-a", "tea-b"}, &recordingStore{}, a1, b1)
	svc.MemberWorkspaceIDs = func(context.Context, core.Identity) ([]string, error) { return []string{"tea-a", "tea-b"}, nil }
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	list := func(query string) []string {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/notification-settings/overrides"+query, nil).WithContext(ctxAs("id-a")))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET overrides%s = %d %s", query, rec.Code, rec.Body)
		}
		var entries []struct {
			Override struct {
				ServiceID string `json:"serviceId"`
			} `json:"override"`
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, e := range entries {
			ids = append(ids, e.Override.ServiceID)
		}
		return ids
	}
	if got := list(""); len(got) != 2 {
		t.Errorf("unfiltered overrides = %v, want both workspaces' services", got)
	}
	if got := list("?ownerId=tea-a"); len(got) != 1 {
		t.Errorf("ownerId=tea-a overrides = %v, want one", got)
	}
}
