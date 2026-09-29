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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// multi_owner_list_test.go is w1/m168: Render's GET /v1/services takes ownerId
// as an array (repeated or comma-separated), and bex read one value.

// memberships is a caller that belongs to several workspaces (the first is
// its default).
type memberships map[string][]string

func (m memberships) Tenant(_ context.Context, id core.Identity) (string, bool) {
	ws := m[id.Subject]
	if len(ws) == 0 {
		return "", false
	}
	return ws[0], true
}

func (m memberships) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	return slices.Contains(m[id.Subject], tenantID), nil
}

func withAppID(a *appv1alpha1.App, id string) *appv1alpha1.App {
	a.Labels[core.LabelAppID] = id
	return a
}

type listedService struct {
	Service struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		OwnerID string `json:"ownerId"`
	} `json:"service"`
	Cursor string `json:"cursor"`
}

func listServices(t *testing.T, svc *Service, subject, query string) (int, []listedService, string) {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/services?"+query, nil).WithContext(ctxAs(subject))
	mux.ServeHTTP(rec, req)
	var out []listedService
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v (%s)", err, rec.Body.String())
		}
	}
	return rec.Code, out, rec.Body.String()
}

func multiOwnerService(ws memberships) *Service {
	svc, _ := newTenantService(ws,
		withAppID(tenantApp("web", "tea-1"), "srv-1"),
		withAppID(tenantApp("api", "tea-1"), "srv-2"),
		withAppID(tenantApp("api", "tea-2"), "srv-3"),
		withAppID(tenantApp("secret", "tea-3"), "srv-4"),
	)
	svc.Authz = &fakeChecker{allow: true}
	return svc
}

func listedIDs(items []listedService) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Service.ID)
	}
	slices.Sort(out)
	return out
}

func TestServiceListUnionsEveryRequestedWorkspace(t *testing.T) {
	svc := multiOwnerService(memberships{"user-a": {"tea-1", "tea-2"}})
	want := []string{"srv-1", "srv-2", "srv-3"}
	for _, query := range []string{"ownerId=tea-1&ownerId=tea-2", "ownerId=tea-1,tea-2", "ownerId=tea-2,tea-1&ownerId=tea-1"} {
		code, got, body := listServices(t, svc, "user-a", query)
		if code != http.StatusOK || !slices.Equal(listedIDs(got), want) {
			t.Errorf("%s = %d %v, want %v: %s", query, code, listedIDs(got), want, body)
		}
	}
	// Filters apply across the union.
	if _, got, _ := listServices(t, svc, "user-a", "ownerId=tea-1,tea-2&name=api"); !slices.Equal(listedIDs(got), []string{"srv-2", "srv-3"}) {
		t.Errorf("name filter across workspaces = %v", listedIDs(got))
	}
}

// A workspace the caller cannot see fails the whole request: nothing from the
// workspaces it CAN see leaks out alongside the refusal.
func TestServiceListFailsClosedOnAnyForbiddenWorkspace(t *testing.T) {
	svc := multiOwnerService(memberships{"user-a": {"tea-1", "tea-2"}})
	for _, query := range []string{"ownerId=tea-1,tea-3", "ownerId=tea-3&ownerId=tea-1"} {
		code, _, body := listServices(t, svc, "user-a", query)
		if code != http.StatusForbidden {
			t.Errorf("%s = %d, want 403: %s", query, code, body)
		}
		if strings.Contains(body, "srv-") {
			t.Errorf("%s disclosed resources with its refusal: %s", query, body)
		}
	}
}

// Names are unique within a workspace, not across them. Walking a
// multi-workspace list one item per page must visit every service exactly once.
func TestServiceListPagesAcrossEqualNamesWithoutGapsOrDuplicates(t *testing.T) {
	svc := multiOwnerService(memberships{"user-a": {"tea-1", "tea-2"}})
	var seen []string
	cursor := ""
	for range 10 {
		query := "ownerId=tea-1,tea-2&limit=1"
		if cursor != "" {
			query += "&cursor=" + cursor
		}
		code, page, body := listServices(t, svc, "user-a", query)
		if code != http.StatusOK {
			t.Fatalf("page = %d: %s", code, body)
		}
		if len(page) == 0 {
			break
		}
		seen = append(seen, page[0].Service.ID)
		cursor = page[0].Cursor
	}
	if want := []string{"srv-1", "srv-2", "srv-3"}; !slices.Equal(seen, want) {
		t.Fatalf("walk visited %v, want each of %v exactly once in id order", seen, want)
	}
}

// A single or omitted owner keeps its existing name cursor and order.
func TestServiceListSingleOwnerPagingUnchanged(t *testing.T) {
	svc := multiOwnerService(memberships{"user-a": {"tea-1", "tea-2"}})
	for _, query := range []string{"ownerId=tea-1&limit=1", "limit=1"} {
		_, page, body := listServices(t, svc, "user-a", query)
		if len(page) != 1 || page[0].Cursor != page[0].Service.Name {
			t.Errorf("%s: cursor is no longer the name: %s", query, body)
		}
	}
}
