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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/m149: Render's lists answer for every workspace the caller belongs to
// when ownerId is omitted, and for exactly the named workspaces when it is
// given. /v1/disks ignored ownerId and answered from the default workspace.
func TestRESTListsScopeByOwnerID(t *testing.T) {
	a1, b1 := webApp("tea-a", "web", "nginx:1"), webApp("tea-b", "api", "nginx:1")
	rec := &recordingStore{disks: map[string]store.Disk{
		"dsk-aaaaaaaaaaaaaaaaaaaa": {ID: "dsk-aaaaaaaaaaaaaaaaaaaa", TenantID: "tea-a", AppID: "srv-tea-a-web", Name: "a"},
		"dsk-bbbbbbbbbbbbbbbbbbbb": {ID: "dsk-bbbbbbbbbbbbbbbbbbbb", TenantID: "tea-b", AppID: "srv-tea-b-api", Name: "b"},
	}}
	newMux := func(spanning bool) http.Handler {
		svc, _ := newTenantStoreService(coretest.Workspaces{"tea-a", "tea-b"}, rec, a1, b1)
		if spanning {
			svc.MemberWorkspaceIDs = func(context.Context, core.Identity) ([]string, error) { return []string{"tea-a", "tea-b"}, nil }
		}
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		return validated(t, mux)
	}
	get := func(h http.Handler, path string) (int, []string) {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctxAs("id-a")))
		if w.Code != http.StatusOK {
			return w.Code, nil
		}
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
			t.Fatalf("GET %s: %v %s", path, err, w.Body)
		}
		var ids []string
		for _, row := range rows {
			for _, key := range []string{"service", "disk"} {
				var item struct {
					ID string `json:"id"`
				}
				if raw, ok := row[key]; ok && json.Unmarshal(raw, &item) == nil {
					ids = append(ids, item.ID)
				}
			}
		}
		slices.Sort(ids)
		return w.Code, ids
	}

	spanning := newMux(true)
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/v1/services", []string{"srv-tea-a-web", "srv-tea-b-api"}},
		{"/v1/services?ownerId=tea-b", []string{"srv-tea-b-api"}},
		{"/v1/disks", []string{"dsk-aaaaaaaaaaaaaaaaaaaa", "dsk-bbbbbbbbbbbbbbbbbbbb"}},
		{"/v1/disks?ownerId=tea-b", []string{"dsk-bbbbbbbbbbbbbbbbbbbb"}},
		{"/v1/disks?ownerId=tea-b&diskId=dsk-bbbbbbbbbbbbbbbbbbbb", []string{"dsk-bbbbbbbbbbbbbbbbbbbb"}},
		{"/v1/disks?ownerId=tea-a&diskId=dsk-bbbbbbbbbbbbbbbbbbbb", nil},
		{"/v1/disks?ownerId=tea-a&ownerId=tea-b", []string{"dsk-aaaaaaaaaaaaaaaaaaaa", "dsk-bbbbbbbbbbbbbbbbbbbb"}},
		{"/v1/disks?ownerId=tea-b&serviceId=srv-tea-a-web", nil},
		{"/v1/disks?ownerId=tea-a&ownerId=tea-b&serviceId=srv-tea-a-web", []string{"dsk-aaaaaaaaaaaaaaaaaaaa"}},
		{"/v1/disks?serviceId=srv-tea-b-api", []string{"dsk-bbbbbbbbbbbbbbbbbbbb"}},
	} {
		if code, got := get(spanning, tc.path); code != http.StatusOK || !slices.Equal(got, tc.want) {
			t.Errorf("GET %s = %d %v, want %v", tc.path, code, got, tc.want)
		}
	}
	// A workspace the caller is not in is refused on disks, as on services.
	for _, path := range []string{"/v1/disks?ownerId=tea-zzz", "/v1/services?ownerId=tea-zzz"} {
		if code, _ := get(spanning, path); code != http.StatusForbidden {
			t.Errorf("GET %s = %d, want 403", path, code)
		}
	}

	// A caller with no membership lookup keeps its single workspace.
	if code, got := get(newMux(false), "/v1/disks"); code != http.StatusOK || !slices.Equal(got, []string{"dsk-aaaaaaaaaaaaaaaaaaaa"}) {
		t.Errorf("GET /v1/disks without membership = %d %v, want the default workspace's disk", code, got)
	}
}
