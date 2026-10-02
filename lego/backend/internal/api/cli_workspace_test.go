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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Exercise the actual auth/router stack: the active CLI workspace selects a
// same-named service, while ids stay global and foreign names stay hidden.
func TestCLIWorkspaceServicePaths(t *testing.T) {
	st := newFakeWSStore()
	base := serverBase(t, st)
	workspaces := []string{ids.New(ids.Workspace), ids.New(ids.Workspace), ids.New(ids.Workspace)}
	services := []string{ids.New(ids.Service), ids.New(ids.Service), ids.New(ids.Service)}
	for i, name := range []string{"home", "other", "foreign"} {
		ws := workspaces[i]
		st.tenants = append(st.tenants, store.Tenant{ID: ws, Name: name, Plan: "hobby"})
		subject := "client-1"
		if i == 2 {
			subject = "someone-else"
		}
		st.members[ws] = []store.TenantMember{{TenantID: ws, Subject: subject, Role: "admin"}}
		a := sampleApp(core.CRName(ws, "old"))
		a.Namespace = ws
		a.Labels = map[string]string{core.LabelTenant: ws, core.LabelServiceName: "old", core.LabelAppID: services[i]}
		a.Spec.DisplayName = "web"
		if err := base.Client.Create(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
	unique := sampleApp(core.CRName(workspaces[1], "unique"))
	unique.Namespace = workspaces[1]
	unique.Labels = map[string]string{core.LabelTenant: workspaces[1], core.LabelServiceName: "unique", core.LabelAppID: ids.New(ids.Service)}
	if err := base.Client.Create(context.Background(), unique); err != nil {
		t.Fatal(err)
	}
	h, _ := serverWith(t, base, Deps{WorkspaceStore: st, DeployStore: &conformDeployStore{}})
	for _, tc := range []struct {
		name, selected, target string
		status                 int
		wantID                 string
	}{
		{"saved workspace id", workspaces[1], "web", 200, services[1]},
		{"environment workspace name", "other", "web", 200, services[1]},
		{"home selection", "home", "web", 200, services[0]},
		{"no selection refuses ambiguity", "", "web", 409, ""},
		{"unknown selection fails closed", "missing", "web", 404, ""},
		{"foreign id", workspaces[2], "web", 403, ""},
		{"foreign name hidden", "foreign", "web", 404, ""},
		{"typed id ignores selection", workspaces[0], services[1], 200, services[1]},
		{"typed id ignores stale selection", "missing", services[1], 200, services[1]},
		{"typed id still authorizes", workspaces[0], services[2], 403, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/services/"+tc.target, nil)
			req.Header.Set("Authorization", "Bearer "+testToken)
			req.Header.Set(cliWorkspaceHeader, tc.selected)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("HTTP %d: %s; want %d", rec.Code, rec.Body, tc.status)
			}
			if tc.wantID != "" {
				var got struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.ID != tc.wantID {
					t.Fatalf("service id = %q, want %q; %s", got.ID, tc.wantID, rec.Body)
				}
			}
		})
	}
	// The target exists only in the other workspace: skipping scope would
	// return its subresources, so these 404s discriminate against that bug.
	for _, suffix := range []string{"/deploys", "/instances"} {
		for _, selected := range []string{"", "home", "other"} {
			req := httptest.NewRequest("GET", "/v1/services/unique"+suffix, nil)
			req.Header.Set("Authorization", "Bearer "+testToken)
			req.Header.Set(cliWorkspaceHeader, selected)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			want := http.StatusOK
			if selected == "home" {
				want = http.StatusNotFound
			}
			if rec.Code != want {
				t.Fatalf("nested %s selected %q: %d %s, want %d", suffix, selected, rec.Code, rec.Body, want)
			}
		}
	}
}

func TestCLIWorkspaceHeaderNameIsPinned(t *testing.T) {
	// CLI imports no backend packages; transport_test.go pins the other half.
	if cliWorkspaceHeader != "X-Bex-Workspace" {
		t.Fatal("update CLI and backend header together")
	}
}
