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

package sandbox

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w22/m603: Render's SDK v1.10.0 (TypeScript and Python) and CLI v2.38.0 put a
// sandbox group in a project environment, move it and take it out with the
// requests testdata/group-environment-contract.json records from their source.
// Each sequence is replayed here, in the order a user would run them, over
// bex-api's REST routes: every status must be one the clients accept and every
// read what they then expect. The fixture's handler also checks each response
// against the pinned Render schema, which the Python client parses.

type contractStep struct {
	Method  string          `json:"method"`
	Path    string          `json:"path"`
	Body    json.RawMessage `json:"body"`
	Success []int           `json:"success"`
	Expect  map[string]any  `json:"expect"`
}

type groupEnvironmentContract struct {
	Sequences map[string][]contractStep `json:"sequences"`
	Refusals  map[string]struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"refusals"`
}

func loadGroupEnvironmentContract(t *testing.T) groupEnvironmentContract {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "group-environment-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c groupEnvironmentContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"create", "createTypeScript", "set", "move", "clear", "clearWhenInNone"} {
		if len(c.Sequences[name]) == 0 {
			t.Fatalf("the contract has no %q sequence", name)
		}
	}
	return c
}

// replay runs one recorded sequence with its placeholders bound and returns the
// last response body.
func (f *groupFixture) replay(t *testing.T, name string, steps []contractStep, vars map[string]string) string {
	t.Helper()
	bind := func(s string) string { return bindVars(s, vars) }
	var body string
	for i, step := range steps {
		var code int
		code, body = f.do(t, step.Method, "/v1"+bind(step.Path), bind(string(step.Body)))
		if !slices.Contains(step.Success, code) {
			t.Fatalf("%s step %d: %s %s = %d %s, the clients accept %v", name, i+1, step.Method, bind(step.Path), code, body, step.Success)
		}
		if step.Expect == nil {
			continue
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("%s step %d: %s is not a group: %v", name, i+1, body, err)
		}
		for key, want := range step.Expect {
			if s, ok := want.(string); ok {
				want = bind(s)
			}
			if got[key] != want {
				t.Fatalf("%s step %d: %s = %v, the clients then expect %v", name, i+1, key, got[key], want)
			}
		}
	}
	return body
}

func TestRenderClientsMoveAGroupBetweenEnvironments(t *testing.T) {
	c := loadGroupEnvironmentContract(t)
	f := newGroupFixture(t)
	vars := map[string]string{
		"$OWNER": f.ws,
		"$ENV_A": f.environment(t, "staging", false),
		"$ENV_B": f.environment(t, "production", false),
	}

	// createGroup / create_group / `sandbox-groups create --environment`, and
	// the TypeScript SDK's body, which also sends its default region.
	var created Group
	if err := json.Unmarshal([]byte(f.replay(t, "create", c.Sequences["create"], vars)), &created); err != nil {
		t.Fatal(err)
	}
	f.replay(t, "createTypeScript", c.Sequences["createTypeScript"], vars)

	// setGroupEnvironment(g, A), then (g, B), then (g, null), and null again on
	// a group in none; the CLI's `environment set` and `remove` send the same.
	vars["$GROUP"] = f.createGroup(t, "mover").ID
	for _, name := range []string{"set", "move", "clear", "clearWhenInNone"} {
		f.replay(t, name, c.Sequences[name], vars)
	}
	// The group the create placed was not touched by the other group's moves.
	code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+created.ID, "")
	var still Group
	if err := json.Unmarshal([]byte(body), &still); code != http.StatusOK || err != nil || environmentOfGroup(still) != vars["$ENV_A"] {
		t.Fatalf("the created group after the moves = %d %s, want it still in %s", code, body, vars["$ENV_A"])
	}
}

// The refusals the clients read: an unknown group is 404 "sandbox group not
// found" on add and on remove, the CLI fake's own answer, and a Developer's
// create in, or move into or out of, a protected environment is a 403.
func TestRenderClientsReadTheGroupMoveRefusals(t *testing.T) {
	c := loadGroupEnvironmentContract(t)
	f := newGroupFixture(t)
	staging := f.environment(t, "staging", false)
	unknown := ids.SandboxGroup.Prefix() + "-" + strings.Repeat("a", 20)

	notFound := c.Refusals["unknownGroup"]
	for _, req := range []struct{ method, path, body string }{
		{http.MethodPost, "/v1/environments/" + staging + "/resources", `{"resourceIds":["` + unknown + `"]}`},
		{http.MethodDelete, "/v1/environments/" + staging + "/resources?resourceIds=" + unknown, ""},
	} {
		code, body := f.do(t, req.method, req.path, req.body)
		var refusal struct{ Message, Code string }
		_ = json.Unmarshal([]byte(body), &refusal)
		if code != notFound.Status || refusal.Message != notFound.Message || refusal.Code != CodeSandboxGroupNotFound {
			t.Errorf("%s of an unknown group = %d %s, want %d %q", req.method, code, body, notFound.Status, notFound.Message)
		}
	}

	prod := f.environment(t, "production", true)
	f.svc.Base.Authz = managerChecker{admin: true}
	g := f.mustCreateGroupIn(t, "in-prod", prod)
	f.svc.Base.Authz = managerChecker{admin: false}
	forbidden := c.Refusals["protectedEnvironment"]
	// SDK v1.10.0: creating a group in a protected environment needs an Admin,
	// in either SDK's body, and the refused create leaves no group.
	before := len(f.listGroups(t))
	for _, name := range []string{"create", "createTypeScript"} {
		step := c.Sequences[name][0]
		body := bindVars(string(step.Body), map[string]string{"$OWNER": f.ws, "$ENV_A": prod})
		if code, out := f.do(t, step.Method, "/v1"+step.Path, body); code != forbidden.Status || errCode(out) != core.ProtectedPlacementCode {
			t.Errorf("a Developer's %s in a protected environment = %d %s, want %d %s", name, code, out, forbidden.Status, core.ProtectedPlacementCode)
		}
	}
	if after := len(f.listGroups(t)); after != before {
		t.Errorf("the refused creates left %d groups, want %d", after, before)
	}
	for _, req := range []struct{ method, path, body string }{
		{http.MethodPost, "/v1/environments/" + staging + "/resources", `{"resourceIds":["` + g.ID + `"]}`},
		{http.MethodDelete, "/v1/environments/" + prod + "/resources?resourceIds=" + g.ID, ""},
	} {
		if code, body := f.do(t, req.method, req.path, req.body); code != forbidden.Status || errCode(body) != core.ProtectedPlacementCode {
			t.Errorf("a Developer's %s out of a protected environment = %d %s, want %d %s", req.method, code, body, forbidden.Status, core.ProtectedPlacementCode)
		}
	}
	if code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+g.ID, ""); code != http.StatusOK || !strings.Contains(body, prod) {
		t.Fatalf("after the refusals the group reads %d %s, want it still in %s", code, body, prod)
	}
}
