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

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// The same caller can operate in their personal workspace but is only a viewer
// in the shared one. A projection must use the selected workspace's grants.
type actionWorkspaceChecker struct{ sharedAllowed bool }

func (c actionWorkspaceChecker) Check(_ context.Context, _, relation, object string) (bool, error) {
	return object == core.WorkspaceObject("tea-a") ||
		(object == core.WorkspaceObject("tea-b") && (relation == core.RelCanView || c.sharedAllowed)), nil
}

func TestActionCapabilitiesGraphQLWorkspaceSelection(t *testing.T) {
	home, shared := ownedApp("home", "tea-a"), ownedApp("shared", "tea-b")
	home.Labels[core.LabelAppID], shared.Labels[core.LabelAppID] = ids.New(ids.Service), ids.New(ids.Service)
	base := &core.Base{
		Client: fakeClient(home, shared), Namespace: "default",
		Workspace: twoWorkspaceResolver{}, Authz: actionWorkspaceChecker{},
	}
	handler, _ := serverWith(t, base, Deps{DeployStore: &conformDeployStore{}})
	for _, projection := range []struct{ field, key, action string }{
		{"serverActions", "id", core.ActionSuspend},
		{"deployActions", "serviceId", core.ActionDeploy},
	} {
		t.Run(projection.field, func(t *testing.T) {
			for _, tc := range []struct {
				name, target, owner, outcome, wantError string
				sharedAllowed                           bool
			}{
				{name: "omitted owner keeps personal default", target: home.Labels[core.LabelAppID], outcome: core.DecisionAllowed},
				{name: "selected shared viewer", target: shared.Labels[core.LabelAppID], owner: "tea-b", outcome: core.DecisionDenied},
				{name: "selected shared operator", target: shared.Labels[core.LabelAppID], owner: "tea-b", outcome: core.DecisionAllowed, sharedAllowed: true},
				{name: "omitted owner cannot imply shared workspace", target: shared.Labels[core.LabelAppID], wantError: core.ErrNotFound.Error()},
				{name: "wrong selected workspace cannot read accessible target", target: shared.Labels[core.LabelAppID], owner: "tea-a", wantError: core.ErrForbidden.Error()},
				{name: "missing target has same absence", target: ids.New(ids.Service), owner: "tea-a", wantError: core.ErrNotFound.Error()},
				{name: "nonmember selection never falls back", target: home.Labels[core.LabelAppID], owner: "tea-foreign", wantError: core.ErrForbidden.Error()},
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
