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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// w22/m311 t005: REST, GraphQL and MCP answer every group behavior the same
// way. Each scenario runs once per surface, each on its own fresh workspace,
// and the machine code of a refusal and the normalized fields of a success
// must be identical across the three.

// outcome is what a surface answered: the machine code of a refusal (empty on
// success), or the fields of the result with every generated id and timestamp
// normalized away.
type outcome struct {
	Code   string
	Fields map[string]any
}

const unsupportedSurface = "unsupported-by-this-surface"

type groupDriver interface {
	createGroup(groupName string) outcome
	getGroup(id string) outcome
	// listGroups reads one page: after is the cursor ("" = the first page) and
	// limit the page size (0 = the surface's default).
	listGroups(after string, limit int) outcome
	updateGroup(id string, limit int) outcome
	createSandbox(groupID string) outcome
	listSandboxes(groupID string) outcome
	// execOn runs a command in a sandbox, which wakes it when it is suspended.
	execOn(id string) outcome
}

func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			switch k {
			case "id", "ownerId", "createdAt", "updatedAt", "cursor", "workspace", "owner", "image", "networkPolicy", "executionId":
				continue
			}
			out[k] = normalize(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalize(val)
		}
		return out
	case string:
		for _, prefix := range []string{"sbg-", "sbx-"} {
			if strings.HasPrefix(x, prefix) {
				return "<" + prefix + "id>"
			}
		}
		return x
	case float64:
		return x
	}
	return v
}

func fieldsOf(v any) map[string]any {
	raw, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	norm, _ := normalize(m).(map[string]any)
	return norm
}

// ---- REST ----

type restDriver struct {
	t *testing.T
	f *groupFixture
}

func (d restDriver) call(method, path, body string) outcome {
	d.t.Helper()
	code, resp := d.f.do(d.t, method, path, body)
	if code >= 400 {
		return outcome{Code: errCode(resp)}
	}
	var v any
	_ = json.Unmarshal([]byte(resp), &v)
	return outcome{Fields: fieldsOf(map[string]any{"result": v})}
}

func (d restDriver) createGroup(name string) outcome {
	return d.call(http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"name":%q}`, name))
}

func (d restDriver) getGroup(id string) outcome {
	return d.call(http.MethodGet, "/v1/sandbox-groups/"+id, "")
}

func (d restDriver) listGroups(after string, limit int) outcome {
	query := ""
	if after != "" {
		query += "&cursor=" + after
	}
	if limit > 0 {
		query += fmt.Sprintf("&limit=%d", limit)
	}
	out := d.call(http.MethodGet, "/v1/sandbox-groups?ownerId="+d.f.ws+query, "")
	if out.Code != "" {
		return out
	}
	// REST wraps each group as {sandboxGroup, cursor}; the other surfaces list groups.
	items, _ := out.Fields["result"].([]any)
	var groups []any
	for _, item := range items {
		groups = append(groups, item.(map[string]any)["sandboxGroup"])
	}
	return outcome{Fields: map[string]any{"result": groups}}
}

func (d restDriver) updateGroup(id string, limit int) outcome {
	return d.call(http.MethodPatch, "/v1/sandbox-groups/"+id, fmt.Sprintf(`{"concurrencyLimit":%d}`, limit))
}

func (d restDriver) createSandbox(groupID string) outcome {
	body := `{}`
	if groupID != "" {
		body = fmt.Sprintf(`{"sandboxGroupId":%q}`, groupID)
	}
	return d.call(http.MethodPost, "/v1/sandboxes", body)
}

func (d restDriver) listSandboxes(groupID string) outcome {
	path := "/v1/sandboxes"
	if groupID != "" {
		path += "?sandboxGroupId=" + groupID
	}
	out := d.call(http.MethodGet, path, "")
	if out.Code != "" {
		return out
	}
	// REST wraps each item as {sandbox, cursor}; the other surfaces list sandboxes.
	items, _ := out.Fields["result"].([]any)
	var boxes []any
	for _, item := range items {
		boxes = append(boxes, item.(map[string]any)["sandbox"])
	}
	return outcome{Fields: map[string]any{"result": boxes}}
}

func (d restDriver) execOn(id string) outcome {
	return d.call(http.MethodPost, "/v1/sandboxes/"+id+"/exec", `{"command":"true"}`)
}

// ---- GraphQL ----

type gqlDriver struct {
	t      *testing.T
	schema graphql.Schema
}

func newGQLDriver(t *testing.T, svc *Service) gqlDriver {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return gqlDriver{t: t, schema: schema}
}

func (d gqlDriver) run(root, query string, vars map[string]any) outcome {
	d.t.Helper()
	res := graphql.Do(graphql.Params{Schema: d.schema, RequestString: query, VariableValues: vars, Context: callerCtx()})
	if len(res.Errors) > 0 {
		code, _ := res.Errors[0].Extensions["code"].(string)
		if code == "" {
			d.t.Fatalf("GraphQL refusal without a code: %v", res.Errors[0])
		}
		return outcome{Code: code}
	}
	data, _ := res.Data.(map[string]any)
	return outcome{Fields: fieldsOf(map[string]any{"result": data[root]})}
}

const gqlGroupFields = `id ownerId name region isDefault environmentId concurrencyLimit createdAt updatedAt`
const gqlSandboxFields = `id name plan status region timeoutSeconds idleTimeoutSeconds sandboxGroupId`

func (d gqlDriver) createGroup(name string) outcome {
	return d.run("createSandboxGroup", `mutation($n: String!) { createSandboxGroup(name: $n) { `+gqlGroupFields+` } }`, map[string]any{"n": name})
}

func (d gqlDriver) getGroup(id string) outcome {
	return d.run("sandboxGroup", `query($id: String!) { sandboxGroup(id: $id) { `+gqlGroupFields+` } }`, map[string]any{"id": id})
}

func (d gqlDriver) listGroups(after string, limit int) outcome {
	vars := map[string]any{}
	if after != "" {
		vars["c"] = after
	}
	if limit > 0 {
		vars["l"] = limit
	}
	return d.run("sandboxGroups", `query($c: String, $l: Int) { sandboxGroups(cursor: $c, limit: $l) { `+gqlGroupFields+` } }`, vars)
}

func (d gqlDriver) updateGroup(id string, limit int) outcome {
	return d.run("updateSandboxGroup", `mutation($id: String!, $l: Int!) { updateSandboxGroup(id: $id, concurrencyLimit: $l) { `+gqlGroupFields+` } }`,
		map[string]any{"id": id, "l": limit})
}

func (d gqlDriver) createSandbox(groupID string) outcome {
	args := ""
	vars := map[string]any{}
	if groupID != "" {
		args, vars = `(sandboxGroupId: $g)`, map[string]any{"g": groupID}
	}
	decl := ""
	if groupID != "" {
		decl = `($g: String)`
	}
	return d.run("createSandbox", `mutation`+decl+` { createSandbox`+args+` { `+gqlSandboxFields+` } }`, vars)
}

func (d gqlDriver) listSandboxes(groupID string) outcome {
	args, decl, vars := "", "", map[string]any{}
	if groupID != "" {
		args, decl, vars = `(sandboxGroupId: $g)`, `($g: String)`, map[string]any{"g": groupID}
	}
	return d.run("sandboxes", `query`+decl+` { sandboxes`+args+` { `+gqlSandboxFields+` } }`, vars)
}

func (gqlDriver) execOn(string) outcome { return outcome{Code: unsupportedSurface} }

// ---- MCP ----

type mcpDriver struct {
	t      *testing.T
	client *mcp.ClientSession
}

func (d mcpDriver) call(tool string, args map[string]any) outcome {
	d.t.Helper()
	res, err := d.client.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		d.t.Fatalf("%s: %v", tool, err)
	}
	if res.IsError {
		text := res.Content[0].(*mcp.TextContent).Text
		code, _, found := strings.Cut(text, ": ")
		if !found {
			d.t.Fatalf("MCP refusal without a code: %q", text)
		}
		return outcome{Code: code}
	}
	return outcome{Fields: fieldsOf(map[string]any{"result": res.StructuredContent})}
}

func (d mcpDriver) createGroup(name string) outcome {
	return d.call("create_sandbox_group", map[string]any{"name": name})
}

func (d mcpDriver) getGroup(id string) outcome {
	return d.call("get_sandbox_group", map[string]any{"id": id})
}

func (d mcpDriver) listGroups(after string, limit int) outcome {
	args := map[string]any{}
	if after != "" {
		args["cursor"] = after
	}
	if limit > 0 {
		args["limit"] = limit
	}
	out := d.call("list_sandbox_groups", args)
	if out.Code != "" {
		return out
	}
	// MCP wraps the list as {sandboxGroups: [...]}.
	inner, _ := out.Fields["result"].(map[string]any)
	return outcome{Fields: map[string]any{"result": inner["sandboxGroups"]}}
}

func (d mcpDriver) updateGroup(id string, limit int) outcome {
	return d.call("update_sandbox_group", map[string]any{"id": id, "concurrencyLimit": limit})
}

func (d mcpDriver) createSandbox(groupID string) outcome {
	args := map[string]any{}
	if groupID != "" {
		args["sandboxGroupId"] = groupID
	}
	return d.call("spawn_sandbox", args)
}

func (d mcpDriver) listSandboxes(groupID string) outcome {
	args := map[string]any{}
	if groupID != "" {
		args["sandboxGroupId"] = groupID
	}
	out := d.call("list_sandboxes", args)
	if out.Code != "" {
		return out
	}
	// MCP wraps the list as {sandboxes: [...]}.
	inner, _ := out.Fields["result"].(map[string]any)
	return outcome{Fields: map[string]any{"result": inner["sandboxes"]}}
}

func (d mcpDriver) execOn(id string) outcome {
	return d.call("sandbox_exec", map[string]any{"id": id, "command": "true"})
}

// canonical makes a list outcome comparable: an empty or null list is one thing,
// and, unless the list is ordered, the order of items is not part of the contract.
func canonical(o outcome, ordered bool) outcome {
	if o.Fields == nil {
		return o
	}
	list, ok := o.Fields["result"].([]any)
	if o.Fields["result"] == nil {
		list, ok = []any{}, true
	}
	if ok {
		sorted := append([]any{}, list...) // never nil: an empty list is one thing
		if !ordered {
			// OpenSandbox's list is a map, so a sandbox list has no order.
			slices.SortFunc(sorted, func(a, b any) int {
				ja, _ := json.Marshal(a)
				jb, _ := json.Marshal(b)
				return strings.Compare(string(ja), string(jb))
			})
		}
		o.Fields = map[string]any{"result": sorted}
	}
	return o
}

// groupNames is the names in a group list outcome, in order.
func groupNames(o outcome) []string {
	names := []string{}
	list, _ := o.Fields["result"].([]any)
	for _, item := range list {
		names = append(names, item.(map[string]any)["name"].(string))
	}
	return names
}

func TestGroupBehaviorsAgreeAcrossRESTGraphQLAndMCP(t *testing.T) {
	type env struct {
		f *groupFixture
		d groupDriver
	}
	scenarios := []struct {
		name string
		// want is the machine code every surface must answer ("" = a success
		// whose normalized fields must match across surfaces).
		want string
		run  func(e env) outcome
	}{
		{"create a group", "", func(e env) outcome { return e.d.createGroup("CI agents") }},
		{"a name another group has", CodeSandboxGroupNameTaken, func(e env) outcome {
			e.f.createGroup(t, "taken")
			return e.d.createGroup("taken")
		}},
		{"an invalid name", CodeSandboxGroupNameInvalid, func(e env) outcome { return e.d.createGroup(" padded") }},
		{"past the group cap", CodeSandboxGroupLimitReached, func(e env) outcome {
			for i := range e.f.svc.MaxGroups {
				e.f.createGroup(t, fmt.Sprintf("g%d", i))
			}
			return e.d.createGroup("one-too-many")
		}},
		{"a limit above the ceiling is capped", "", func(e env) outcome {
			return e.d.updateGroup(e.f.createGroup(t, "cap").ID, 5000)
		}},
		{"a limit below one", CodeSandboxGroupLimitInvalid, func(e env) outcome {
			return e.d.updateGroup(e.f.createGroup(t, "low").ID, 0)
		}},
		{"updating an unknown group", CodeSandboxGroupNotFound, func(e env) outcome {
			return e.d.updateGroup("sbg-"+strings.Repeat("a", 20), 5)
		}},
		{"updating a malformed group id", CodeInvalidSandboxGroupID, func(e env) outcome {
			return e.d.updateGroup("nope", 5)
		}},
		{"creating in an unknown group", CodeSandboxGroupNotFound, func(e env) outcome {
			return e.d.createSandbox("sbg-" + strings.Repeat("a", 20))
		}},
		{"creating in a malformed group id", CodeInvalidSandboxGroupID, func(e env) outcome {
			return e.d.createSandbox("nope")
		}},
		{"creating in the group", "", func(e env) outcome {
			return e.d.createSandbox(e.f.createGroup(t, "target").ID)
		}},
		{"listing a group's sandboxes", "", func(e env) outcome {
			a, b := e.f.createGroup(t, "a"), e.f.createGroup(t, "b")
			e.f.mustCreate(t, a.ID)
			e.f.mustCreate(t, b.ID)
			e.f.mustCreate(t, b.ID)
			return e.d.listSandboxes(b.ID)
		}},
		{"listing an unknown group", CodeSandboxGroupNotFound, func(e env) outcome {
			return e.d.listSandboxes("sbg-" + strings.Repeat("a", 20))
		}},
		{"a create at the group's limit", CodeSandboxGroupConcurrencyLimit, func(e env) outcome {
			g := e.f.createGroup(t, "full")
			e.f.setLimit(t, g, 1)
			e.f.mustCreate(t, g.ID)
			return e.d.createSandbox(g.ID)
		}},
		{"a create at the workspace cap", CodeSandboxWorkspaceConcurrencyLimit, func(e env) outcome {
			e.f.svc.MaxActive = 1
			g := e.f.createGroup(t, "wide")
			e.f.mustCreate(t, g.ID)
			return e.d.createSandbox(g.ID)
		}},
		{"waking a suspended sandbox at the group's limit", CodeSandboxGroupConcurrencyLimit, func(e env) outcome {
			g := e.f.createGroup(t, "wake")
			e.f.setLimit(t, g, 1)
			asleep := e.f.mustCreate(t, g.ID)
			e.f.suspend(t, asleep)
			e.f.mustCreate(t, g.ID)
			return e.d.execOn(asleep.ID)
		}},
		{"retrieving a group", "", func(e env) outcome {
			return e.d.getGroup(e.f.createGroup(t, "one").ID)
		}},
		{"retrieving an unknown group", CodeSandboxGroupNotFound, func(e env) outcome {
			return e.d.getGroup("sbg-" + strings.Repeat("a", 20))
		}},
		{"retrieving a malformed group id", CodeInvalidSandboxGroupID, func(e env) outcome {
			return e.d.getGroup("nope")
		}},
		{"listing a workspace's groups", "", func(e env) outcome {
			for _, name := range []string{"g0", "g1", "g2"} {
				e.f.createGroup(t, name)
			}
			return e.d.listGroups("", 0)
		}},
		{"listing a page of groups after a cursor", "", func(e env) outcome {
			e.f.svc.MaxGroups = 0 // the fixture's cap of 3 is below the 4 this needs
			for _, name := range []string{"g0", "g1", "g2", "g3"} {
				e.f.createGroup(t, name)
			}
			newestFirst, err := e.f.st.SandboxGroups(context.Background(), e.f.ws)
			if err != nil {
				t.Fatal(err)
			}
			return e.d.listGroups(newestFirst[1].ID, 1) // after g2, one group: g1
		}},
		{"listing a workspace with no groups", "", func(e env) outcome {
			return e.d.listGroups("", 0)
		}},
	}
	// A group list is ordered, newest first, and these are the names each must
	// carry; every other scenario's list is unordered.
	groupLists := map[string][]string{
		"listing a workspace's groups":            {"g2", "g1", "g0"},
		"listing a page of groups after a cursor": {"g1"},
		"listing a workspace with no groups":      {},
	}
	// Only these surfaces have the operation at all: GraphQL has no exec verb, so
	// waking a sandbox by exec is REST and MCP.
	only := map[string][]string{"waking a suspended sandbox at the group's limit": {"MCP", "REST"}}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			got := map[string]outcome{}
			for _, driver := range []string{"REST", "GraphQL", "MCP"} {
				f := newGroupFixture(t)
				e := env{f: f}
				switch driver {
				case "REST":
					e.d = restDriver{t: t, f: f}
				case "GraphQL":
					e.d = newGQLDriver(t, f.svc)
				case "MCP":
					e.d = mcpDriver{t: t, client: mcpClient(t, f.svc)}
				}
				_, ordered := groupLists[sc.name]
				got[driver] = canonical(sc.run(e), ordered)
			}
			var surfaces []string
			for driver, out := range got {
				if out.Code == unsupportedSurface {
					continue
				}
				surfaces = append(surfaces, driver)
			}
			slices.Sort(surfaces)
			if len(surfaces) < 2 {
				t.Fatalf("%s: only %v answered", sc.name, surfaces)
			}
			ref := got[surfaces[0]]
			if ref.Code != sc.want {
				t.Fatalf("%s answered code %q, want %q", surfaces[0], ref.Code, sc.want)
			}
			if want, listed := groupLists[sc.name]; listed {
				if names := groupNames(ref); !slices.Equal(names, want) {
					t.Errorf("%s listed %v, want %v", surfaces[0], names, want)
				}
			}
			for _, driver := range surfaces[1:] {
				if got[driver].Code != ref.Code {
					t.Errorf("%s answered code %q, %s answered %q", driver, got[driver].Code, surfaces[0], ref.Code)
				}
				if ref.Code == "" && !reflect.DeepEqual(got[driver].Fields, ref.Fields) {
					t.Errorf("%s answered %v, %s answered %v", driver, got[driver].Fields, surfaces[0], ref.Fields)
				}
			}
			wantSurfaces := only[sc.name]
			if wantSurfaces == nil {
				wantSurfaces = []string{"GraphQL", "MCP", "REST"}
			}
			if !slices.Equal(surfaces, wantSurfaces) {
				t.Errorf("answered by %v, want %v", surfaces, wantSurfaces)
			}
		})
	}
}
