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

package main_test

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// groupEnvironmentContract mirrors the "environment" section of
// lego/cli/testdata/sandbox-groups-contract.json (w22/m604 t004): the objects
// bex-api answers with and, for each v2.38.0 command line, the requests the
// pinned client sends. lego/backend/internal/sandbox/group_environment_cli_contract_test.go
// replays the same requests against bex-api and checks every status and shape.
type groupEnvironmentContract struct {
	Group        string           `json:"group"`
	Owner        map[string]any   `json:"owner"`
	Project      map[string]any   `json:"project"`
	Environments []map[string]any `json:"environments"`
	Commands     map[string]struct {
		Args []string `json:"args"`
		// Before is the environment the group is in when the command starts
		// (null: none); After is where the command leaves it.
		Before   *string           `json:"before"`
		After    *string           `json:"after"`
		Requests []contractRequest `json:"requests"`
	} `json:"commands"`
}

type contractRequest struct {
	Method string         `json:"method"`
	Path   string         `json:"path"`
	Query  string         `json:"query,omitempty"`
	Body   map[string]any `json:"body,omitempty"`
	Status int            `json:"status"`
}

func loadGroupEnvironmentContract(t *testing.T) (sandboxGroupsContract, groupEnvironmentContract) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sandbox-groups-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Environment groupEnvironmentContract `json:"environment"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return loadSandboxGroupsContract(t), doc.Environment
}

// groupEnvironmentServer answers the routes the three commands reach with the
// contract's objects (and any more projects of the workspace), keeps the
// group's environment as bex-api does, and records each request with the
// status it was given.
func groupEnvironmentServer(t *testing.T, groups sandboxGroupsContract, c groupEnvironmentContract, placed *string, more ...map[string]any) (*httptest.Server, func() []contractRequest) {
	t.Helper()
	projects := append([]map[string]any{c.Project}, more...)
	project := func(id string) map[string]any {
		for _, p := range projects {
			if p["id"] == id {
				return p
			}
		}
		return nil
	}
	var mu sync.Mutex
	var seen []contractRequest
	group := maps.Clone(groups.Create.Response)
	group["id"] = c.Group
	setPlacement := func(env *string) {
		if env == nil {
			group["environmentId"] = nil
		} else {
			group["environmentId"] = *env
		}
	}
	setPlacement(placed)
	environment := func(id string) map[string]any {
		for _, e := range c.Environments {
			if e["id"] == id {
				return e
			}
		}
		return nil
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		status, answer := http.StatusNotFound, any(map[string]any{"id": "not_found", "message": "not found"})
		q := r.URL.Query()
		segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/"), "/")
		switch {
		case r.Method == http.MethodGet && len(segments) == 2 && segments[0] == "owners" && segments[1] == c.Owner["id"]:
			status, answer = http.StatusOK, c.Owner
		case r.Method == http.MethodGet && r.URL.Path == "/v1/projects":
			page := []any{}
			if slices.Contains(q["ownerId"], c.Owner["id"].(string)) {
				for _, p := range projects {
					page = append(page, map[string]any{"project": p, "cursor": p["id"]})
				}
			}
			status, answer = http.StatusOK, page
		case r.Method == http.MethodGet && len(segments) == 2 && segments[0] == "projects" && project(segments[1]) != nil:
			status, answer = http.StatusOK, project(segments[1])
		case r.Method == http.MethodGet && r.URL.Path == "/v1/environments":
			page := []any{}
			for _, e := range c.Environments {
				if slices.Contains(queryList(q, "projectId"), e["projectId"].(string)) && (!q.Has("name") || slices.Contains(queryList(q, "name"), e["name"].(string))) {
					page = append(page, map[string]any{"environment": e, "cursor": e["id"]})
				}
			}
			status, answer = http.StatusOK, page
		case r.Method == http.MethodGet && len(segments) == 2 && segments[0] == "environments" && environment(segments[1]) != nil:
			status, answer = http.StatusOK, environment(segments[1])
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sandbox-groups":
			created := maps.Clone(group)
			created["name"] = body["name"]
			created["environmentId"] = body["environmentId"]
			status, answer = http.StatusCreated, created
		case r.Method == http.MethodGet && r.URL.Path == "/v1/sandbox-groups/"+c.Group:
			status, answer = http.StatusOK, group
		case len(segments) == 3 && segments[0] == "environments" && segments[2] == "resources" && environment(segments[1]) != nil:
			ids := q["resourceIds"]
			if r.Method == http.MethodPost {
				ids = nil
				listed, _ := body["resourceIds"].([]any)
				for _, id := range listed {
					s, _ := id.(string)
					ids = append(ids, s)
				}
			}
			if !slices.Equal(ids, []string{c.Group}) {
				status, answer = http.StatusNotFound, map[string]any{"id": "not_found", "message": "sandbox group not found"}
				break
			}
			switch r.Method {
			case http.MethodPost:
				env := segments[1]
				setPlacement(&env)
				status, answer = http.StatusCreated, environment(env)
			case http.MethodDelete:
				if group["environmentId"] == segments[1] {
					setPlacement(nil)
				}
				status, answer = http.StatusNoContent, nil
			}
		}
		seen = append(seen, contractRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body, Status: status})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if answer != nil {
			_ = json.NewEncoder(w).Encode(answer)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []contractRequest {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(seen)
	}
}

// v2.38.0's `sandbox-groups create --environment`, `environment set` (an
// environment id or a name) and `environment remove` send exactly the requests
// the contract records, read bex's answers, and print the group where the
// command left it.
func TestPinnedSandboxGroupEnvironmentCommandsSendTheContract(t *testing.T) {
	groups, c := loadGroupEnvironmentContract(t)
	for _, name := range []string{"createIn", "setByID", "setByName", "move", "remove", "removeWhenInNone"} {
		command, ok := c.Commands[name]
		if !ok || len(command.Requests) == 0 {
			t.Fatalf("the contract has no %q command", name)
		}
		t.Run(name, func(t *testing.T) {
			srv, calls := groupEnvironmentServer(t, groups, c, command.Before)
			run := bexAgainst(t, srv.URL, append(command.Args, "-o", "json")...)
			run.Env = append(run.Env, "BEX_WORKSPACE="+c.Owner["id"].(string))
			var stdout, stderr strings.Builder
			run.Stdout, run.Stderr = &stdout, &stderr
			if err := run.Run(); err != nil {
				t.Fatalf("bex %v: %v\n%s", command.Args, err, stderr.String())
			}
			var printed map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &printed); err != nil {
				t.Fatalf("bex %v printed %q: %v", command.Args, stdout.String(), err)
			}
			want := any(nil)
			if command.After != nil {
				want = *command.After
			}
			if printed["environmentId"] != want {
				t.Errorf("bex %v printed environmentId %v, want %v", command.Args, printed["environmentId"], want)
			}

			got := calls()
			if !slices.EqualFunc(got, command.Requests, sameRequest) {
				pretty, _ := json.MarshalIndent(got, "", "  ")
				t.Fatalf("bex %v sent:\n%s\nthe contract records:\n%+v", command.Args, pretty, command.Requests)
			}
		})
	}
}

// queryList reads a list parameter as bex-api does (core.QueryList): repeated,
// comma-separated, or both.
func queryList(q url.Values, key string) []string {
	var out []string
	for _, raw := range q[key] {
		out = append(out, strings.Split(raw, ",")...)
	}
	return out
}

// sameRequest compares a recorded request with the contract's. A body is
// compared whole; the query is compared as a set of values.
func sameRequest(got, want contractRequest) bool {
	if got.Method != want.Method || got.Path != want.Path || got.Status != want.Status {
		return false
	}
	if !sameQuery(got.Query, want.Query) {
		return false
	}
	gotBody, _ := json.Marshal(got.Body)
	wantBody, _ := json.Marshal(want.Body)
	return string(gotBody) == string(wantBody)
}

func sameQuery(a, b string) bool {
	split := func(s string) []string {
		if s == "" {
			return nil
		}
		parts := strings.Split(s, "&")
		slices.Sort(parts)
		return parts
	}
	return slices.Equal(split(a), split(b))
}

// An environment name the workspace's projects share is refused by the client
// before it moves anything: bex-api's list answers every project's environment
// of that name, as Render's does, and the client asks for the id instead.
func TestPinnedEnvironmentSetRefusesANameTwoProjectsShare(t *testing.T) {
	groups, c := loadGroupEnvironmentContract(t)
	other := maps.Clone(c.Project)
	other["id"], other["name"] = "prj-cph1rs3idesc73a2b2n0", "another"
	twin := maps.Clone(c.Environments[0])
	twin["id"], twin["projectId"] = "evm-cph1rs3idesc73a2b2o0", other["id"]
	c.Environments = append(c.Environments, twin)
	srv, calls := groupEnvironmentServer(t, groups, c, nil, other)

	name := c.Environments[0]["name"].(string)
	run := bexAgainst(t, srv.URL, "sandbox-groups", "environment", "set", c.Group, name, "-o", "json")
	run.Env = append(run.Env, "BEX_WORKSPACE="+c.Owner["id"].(string))
	out, err := run.CombinedOutput()
	if err == nil || !strings.Contains(string(out), `multiple environments found with name "`+name+`"`) {
		t.Fatalf("bex sandbox-groups environment set %s %s = %v:\n%s\nwant the client's refusal of a shared name", c.Group, name, err, out)
	}
	for _, req := range calls() {
		if strings.HasSuffix(req.Path, "/resources") {
			t.Fatalf("the refused set still sent %s %s", req.Method, req.Path)
		}
		if req.Path == "/v1/environments" {
			q, _ := url.ParseQuery(req.Query)
			if !slices.Contains(queryList(q, "projectId"), other["id"].(string)) {
				t.Errorf("the name lookup %s?%s did not ask the second project", req.Path, req.Query)
			}
		}
	}
}
