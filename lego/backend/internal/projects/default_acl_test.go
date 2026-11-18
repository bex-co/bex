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

package projects

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// savedACL is one SetEnvironmentACL call inside the create transaction.
type savedACL struct {
	status    string
	isolated  bool
	allowList []core.IPAllowListEntry
}

// aclRecordingTxStore is txProjectStore that keeps the access settings the
// create saves for each inline environment.
type aclRecordingTxStore struct {
	*fakeProjectStore
	saved []savedACL
}

func (s *aclRecordingTxStore) RunGroupingTx(ctx context.Context, fn func(store.GroupingStore) error) error {
	return fn(aclRecordingGroupings{fakeProjectGroupings{s.fakeProjectStore}, s})
}

type aclRecordingGroupings struct {
	fakeProjectGroupings
	s *aclRecordingTxStore
}

func (g aclRecordingGroupings) SetEnvironmentACL(_ context.Context, _, status string, isolated bool, allowList []core.IPAllowListEntry) error {
	g.s.saved = append(g.s.saved, savedACL{status, isolated, allowList})
	return nil
}

// w9/106: Render's Terraform provider creates every render_project with
// protectedStatus and networkIsolationEnabled on each environment, "unprotected"
// and false unless configured. A developer, and so every API key, gets the 201
// Render answers; an environment that restricts access still needs an
// administrator.
func TestDeveloperCreatesAProjectWhoseEnvironmentsStateTheDefaults(t *testing.T) {
	for _, tc := range []struct {
		name        string
		environment string
		want        int
	}{
		{"the provider's defaults", `{"name":"prod","protectedStatus":"unprotected","networkIsolationEnabled":false}`, http.StatusCreated},
		{"an allow-all list", `{"name":"prod","protectedStatus":"unprotected","networkIsolationEnabled":false,"ipAllowList":[{"cidrBlock":"0.0.0.0/0","description":"everywhere"}]}`, http.StatusCreated},
		{"protected", `{"name":"prod","protectedStatus":"protected","networkIsolationEnabled":false}`, http.StatusForbidden},
		{"isolated", `{"name":"prod","protectedStatus":"unprotected","networkIsolationEnabled":true}`, http.StatusForbidden},
		{"a narrowing list", `{"name":"prod","ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"office"}]}`, http.StatusForbidden},
		{"deny-all", `{"name":"prod","ipAllowList":[]}`, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &aclRecordingTxStore{fakeProjectStore: newFakeProjectStore()}
			svc := &Service{Base: &core.Base{Authz: denyManageChecker{}}, Store: st}
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			body := `{"name":"terraform","ownerId":"tea-a","environments":[` + tc.environment + `]}`
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(body))
			mux.ServeHTTP(rec, req.WithContext(core.WithStrictJSONDecoding(ctxAs("user-a"))))
			if rec.Code != tc.want {
				t.Fatalf("POST /v1/projects = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			ps, err := st.ListProjects(ctxAs("user-a"), "tea-a")
			if err != nil {
				t.Fatal(err)
			}
			if tc.want != http.StatusCreated {
				if len(ps) != 0 || len(st.saved) != 0 {
					t.Fatalf("a refused create wrote projects %+v and settings %+v", ps, st.saved)
				}
				return
			}
			if len(ps) != 1 || len(st.saved) != 1 {
				t.Fatalf("created projects %+v with settings %+v, want one of each", ps, st.saved)
			}
			got := st.saved[0]
			if got.status != core.ProtectedStatusUnprotected || got.isolated || core.EnvironmentACLBearing(got.status, got.isolated, got.allowList) {
				t.Fatalf("saved %+v, want unprotected and open", got)
			}
			if !strings.Contains(tc.environment, "ipAllowList") && !slices.Equal(got.allowList, core.DefaultEnvironmentAllowList()) {
				t.Fatalf("an omitted list saved %+v, want the seeded allow-all rules", got.allowList)
			}
		})
	}
}
