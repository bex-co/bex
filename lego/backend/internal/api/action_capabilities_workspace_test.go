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
	"fmt"
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The same caller can operate in their personal workspace but is only a viewer
// in the shared one. A projection must use the selected workspace's grants.
type actionWorkspaceChecker struct{ sharedAllowed bool }

func (c actionWorkspaceChecker) Check(_ context.Context, _, relation, object string) (bool, error) {
	return object == core.WorkspaceObject("tea-a") ||
		(object == core.WorkspaceObject("tea-b") && (relation == core.RelCanView || c.sharedAllowed)), nil
}

// TestActionCapabilitiesGraphQLWorkspaceSelection pins every per-resource
// action projection (gqlutil.OwnedIDVerb, ADR087): an explicit ownerId binds
// the acting workspace, and without one the projection stays on the caller's
// default rather than following the resource (w4/m172).
func TestActionCapabilitiesGraphQLWorkspaceSelection(t *testing.T) {
	home, shared := ownedApp("home", "tea-a"), ownedApp("shared", "tea-b")
	home.Labels[core.LabelAppID], shared.Labels[core.LabelAppID] = ids.New(ids.Service), ids.New(ids.Service)
	homeDB, sharedDB := ownedDB(ids.New(ids.Postgres), "tea-a"), ownedDB(ids.New(ids.Postgres), "tea-b")
	homeKV, sharedKV := ownedKV(ids.New(ids.KeyValue), "tea-a"), ownedKV(ids.New(ids.KeyValue), "tea-b")
	base := &core.Base{
		Client: fakeClient(home, shared, homeDB, sharedDB, homeKV, sharedKV), Namespace: "default",
		Workspace: twoWorkspaceResolver{}, Authz: actionWorkspaceChecker{},
	}
	handler, _ := serverWith(t, base, Deps{DeployStore: &conformDeployStore{}})
	for _, projection := range []struct {
		field, key, action string
		home, shared       string
		kind               ids.Kind
	}{
		{"serverActions", "id", core.ActionSuspend, home.Labels[core.LabelAppID], shared.Labels[core.LabelAppID], ids.Service},
		{"deployActions", "serviceId", core.ActionDeploy, home.Labels[core.LabelAppID], shared.Labels[core.LabelAppID], ids.Service},
		{"databaseActions", "id", core.ActionSuspend, homeDB.Name, sharedDB.Name, ids.Postgres},
		{"keyValueActions", "id", core.ActionSuspend, homeKV.Name, sharedKV.Name, ids.KeyValue},
	} {
		t.Run(projection.field, func(t *testing.T) {
			for _, tc := range []struct {
				name, target, owner, outcome, wantError string
				sharedAllowed                           bool
			}{
				{name: "omitted owner keeps personal default", target: projection.home, outcome: core.DecisionAllowed},
				{name: "selected shared viewer", target: projection.shared, owner: "tea-b", outcome: core.DecisionDenied},
				{name: "selected shared operator", target: projection.shared, owner: "tea-b", outcome: core.DecisionAllowed, sharedAllowed: true},
				{name: "omitted owner cannot imply shared workspace", target: projection.shared, wantError: core.ErrNotFound.Error()},
				{name: "wrong selected workspace cannot read accessible target", target: projection.shared, owner: "tea-a", wantError: core.ErrNotFound.Error()}, // not in the named workspace (w5/086)
				{name: "missing target has same absence", target: ids.New(projection.kind), owner: "tea-a", wantError: core.ErrNotFound.Error()},
				{name: "nonmember selection never falls back", target: projection.home, owner: "tea-foreign", wantError: core.ErrForbidden.Error()},
			} {
				t.Run(tc.name, func(t *testing.T) {
					base.Authz = actionWorkspaceChecker{sharedAllowed: tc.sharedAllowed}
					ownerArg := ""
					if tc.owner != "" {
						ownerArg = fmt.Sprintf(",ownerId:%q", tc.owner)
					}
					query := fmt.Sprintf(`{%s(%s:%q%s){action outcome reason precondition}}`, projection.field, projection.key, tc.target, ownerArg)
					body, _ := json.Marshal(map[string]string{"query": query})
					response := do(t, handler, http.MethodPost, "/graphql", testToken, string(body))
					if response.Code != http.StatusOK {
						t.Fatalf("GraphQL status %d: %s", response.Code, response.Body.String())
					}
					var result struct {
						Data   map[string][]core.ActionDecision `json:"data"`
						Errors []struct{ Message string }       `json:"errors"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					rows := result.Data[projection.field]
					if tc.wantError != "" {
						if len(result.Errors) != 1 || result.Errors[0].Message != tc.wantError || len(rows) != 0 {
							t.Fatalf("response = %s, want %q without decisions", response.Body.String(), tc.wantError)
						}
						return
					}
					if len(result.Errors) != 0 || len(rows) == 0 {
						t.Fatalf("expected selected-workspace decisions: %s", response.Body.String())
					}
					for _, row := range rows {
						if row.Action != projection.action {
							continue
						}
						if row.Outcome != tc.outcome || row.Precondition != "" {
							t.Fatalf("selected action = %+v, want %s without a precondition", row, tc.outcome)
						}
						if tc.outcome == core.DecisionDenied && row.Reason != core.ReasonInsufficientPermission {
							t.Fatalf("viewer decision = %+v, want insufficient_permission", row)
						}
						return
					}
					t.Fatalf("missing action %q in %+v", projection.action, rows)
				})
			}
		})
	}
}

// protectedLifecycleStore answers the two protection lookups the lifecycle
// projections make, a service's and a datastore's environment, with one
// status. Every other IntentStore method is the nil interface's: a projection
// must never reach one.
type protectedLifecycleStore struct {
	apps.IntentStore
	status string
}

func (p protectedLifecycleStore) GetAppProtectedStatus(context.Context, string) (string, error) {
	return p.status, nil
}

func (p protectedLifecycleStore) GetEnvironmentProtectedStatus(context.Context, string) (string, error) {
	return p.status, nil
}

// TestLifecycleProjectionsGraphQLProtectedAdminRule (w4/m578): over GraphQL,
// serverActions, databaseActions and keyValueActions report suspend and resume
// as denied with protected_environment_admin_required for a Developer on a
// member of a protected environment, the reason the dashboard shows as
// Render's "This action is limited to admins.", and allowed for an Admin and
// on an unprotected member. Restart, which Render leaves open, stays allowed.
// serverActions' shell row (w4/726) and maintenance toggle row (w4/725) read
// the same way, so the Shell tab and the Settings switch say so before the
// click.
func TestLifecycleProjectionsGraphQLProtectedAdminRule(t *testing.T) {
	web := ownedApp("web", "tea-a")
	web.Labels[core.LabelAppID] = ids.New(ids.Service)
	web.Labels[store.LabelManagedBy] = store.ManagedByValue
	db, kv := ownedDB(ids.New(ids.Postgres), "tea-a"), ownedKV(ids.New(ids.KeyValue), "tea-a")
	db.Labels[core.LabelEnvironment], kv.Labels[core.LabelEnvironment] = "env-1", "env-1"
	for _, tc := range []struct {
		name       string
		authz      core.Checker
		status     string
		wantReason string // "" when suspend and resume are allowed
	}{
		{"developer, protected", developerChecker{}, core.ProtectedStatusProtected, core.ReasonProtectedEnvironmentAdmin},
		{"admin, protected", allowOwnChecker{}, core.ProtectedStatusProtected, ""},
		{"developer, unprotected", developerChecker{}, core.ProtectedStatusUnprotected, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := &core.Base{
				Client: fakeClient(web, db, kv), Namespace: "default",
				Workspace: twoWorkspaceResolver{}, Authz: tc.authz,
			}
			handler, _ := serverWith(t, base, Deps{DeployStore: &conformDeployStore{}, Store: protectedLifecycleStore{status: tc.status}})
			for field, target := range map[string]string{
				"serverActions": web.Labels[core.LabelAppID], "databaseActions": db.Name, "keyValueActions": kv.Name,
			} {
				query := fmt.Sprintf(`{%s(id:%q,ownerId:"tea-a"){action outcome reason precondition}}`, field, target)
				body, _ := json.Marshal(map[string]string{"query": query})
				response := do(t, handler, http.MethodPost, "/graphql", testToken, string(body))
				var result struct {
					Data   map[string][]core.ActionDecision `json:"data"`
					Errors []struct{ Message string }       `json:"errors"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result.Errors) != 0 {
					t.Fatalf("%s: %v %s", field, err, response.Body.String())
				}
				seen := 0
				for _, row := range result.Data[field] {
					switch row.Action {
					case core.ActionSuspend, core.ActionResume, core.ActionShell, core.ActionToggleMaintenanceMode:
						seen++
						if tc.wantReason == "" {
							if row.Outcome != core.DecisionAllowed {
								t.Errorf("%s %s = %+v, want allowed", field, row.Action, row)
							}
						} else if row.Outcome != core.DecisionDenied || row.Reason != tc.wantReason || row.Precondition != "" {
							t.Errorf("%s %s = %+v, want denied/%s without a precondition", field, row.Action, row, tc.wantReason)
						}
					case core.ActionRestart:
						if row.Outcome != core.DecisionAllowed {
							t.Errorf("%s restart = %+v, want allowed", field, row)
						}
					}
				}
				want := 2
				if field == "serverActions" {
					want = 4 // and shell and the maintenance toggle
				}
				if seen != want {
					t.Errorf("%s: saw %d of its %d restricted rows in %+v", field, seen, want, result.Data[field])
				}
			}
		})
	}
}
