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
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w22/m604 t004: Render CLI v2.38.0's `sandbox-groups create --environment`,
// `environment set` and `environment remove` send the requests the "environment"
// section of lego/cli/testdata/sandbox-groups-contract.json records, and
// lego/cli/sandbox_group_environment_contract_test.go proves the pinned client
// sends exactly those and reads these answers. Here every request is replayed,
// command by command, against bex-api's routes: each must answer the status the
// contract records with an object of the contract's shape, and the group must
// end where the command leaves it.

type cliEnvironmentContract struct {
	Group        string           `json:"group"`
	Owner        map[string]any   `json:"owner"`
	Project      map[string]any   `json:"project"`
	Environments []map[string]any `json:"environments"`
	Commands     map[string]struct {
		Before   *string `json:"before"`
		After    *string `json:"after"`
		Requests []struct {
			Method string         `json:"method"`
			Path   string         `json:"path"`
			Query  string         `json:"query"`
			Body   map[string]any `json:"body"`
			Status int            `json:"status"`
		} `json:"requests"`
	} `json:"commands"`
}

// The owner and project reads are Render's general routes, which the pinned
// CLI's --environment resolution reads for every command that takes one; their
// answers are proven by the workspaces and projects packages (w8/m149), whose
// routes this fixture does not mount.
var cliEnvironmentProvenElsewhere = []string{"/v1/owners/", "/v1/projects"}

func TestPinnedCLIGroupEnvironmentCommandsReadBexAnswers(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "sandbox-groups-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Create struct {
			Response map[string]any `json:"response"`
		} `json:"create"`
		Environment cliEnvironmentContract `json:"environment"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	c := golden.Environment
	order := []string{"createIn", "setByID", "setByName", "move", "remove", "removeWhenInNone"}
	if len(c.Commands) != len(order) {
		t.Fatalf("the contract has commands %v, this test replays %v", slices.Sorted(maps.Keys(c.Commands)), order)
	}

	f := newGroupFixture(t)
	ctx := context.Background()
	project, err := f.st.CreateProject(ctx, f.ws, c.Project["name"].(string))
	if err != nil {
		t.Fatal(err)
	}
	bound := map[string]string{
		c.Owner["id"].(string):   f.ws,
		c.Project["id"].(string): project.ID,
	}
	for _, e := range c.Environments {
		env, err := f.st.CreateEnvironment(ctx, project.ID, f.ws, e["name"].(string))
		if err != nil {
			t.Fatal(err)
		}
		bound[e["id"].(string)] = ids.EnvironmentPublicID(env.ID)
	}
	group := f.createGroup(t, "mover")
	bound[c.Group] = group.ID
	pairs := make([]string, 0, 2*len(bound))
	for from, to := range bound {
		pairs = append(pairs, from, to)
	}
	bind := strings.NewReplacer(pairs...).Replace

	environmentShape := c.Environments[0]
	groupShape := func(environmentID *string) map[string]any {
		shape := maps.Clone(golden.Create.Response)
		if environmentID != nil {
			shape["environmentId"] = bind(*environmentID)
		}
		return shape
	}
	placement := func() *string {
		_, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+group.ID, "")
		var g Group
		_ = json.Unmarshal([]byte(body), &g)
		return g.EnvironmentID
	}
	place := func(want *string) {
		now := placement()
		switch {
		case want == nil && now != nil:
			f.do(t, http.MethodDelete, "/v1/environments/"+*now+"/resources?resourceIds="+group.ID, "")
		case want != nil && (now == nil || *now != bind(*want)):
			f.do(t, http.MethodPost, "/v1/environments/"+bind(*want)+"/resources", `{"resourceIds":["`+group.ID+`"]}`)
		}
		if got := placementText(placement()); got != bind(placementText(want)) {
			t.Fatalf("could not put the group in %q before the command: it reads %q", placementText(want), got)
		}
	}

	for _, name := range order {
		command := c.Commands[name]
		t.Run(name, func(t *testing.T) {
			if name != "createIn" {
				place(command.Before)
			}
			var last map[string]any
			for i, req := range command.Requests {
				if provenElsewhere(req.Path) {
					continue
				}
				path := bind(req.Path)
				if req.Query != "" {
					path += "?" + bind(req.Query)
				}
				body := ""
				if req.Body != nil {
					rawBody, _ := json.Marshal(req.Body)
					body = bind(string(rawBody))
				}
				code, out := f.do(t, req.Method, path, body)
				if code != req.Status {
					t.Fatalf("request %d: %s %s = %d %s, the pinned CLI was given %d", i+1, req.Method, path, code, out, req.Status)
				}
				if out == "" {
					continue
				}
				switch {
				case strings.HasPrefix(req.Path, "/v1/sandbox-groups"):
					last = nil
					_ = json.Unmarshal([]byte(out), &last)
					want := command.After
					if req.Method == http.MethodGet && i < len(command.Requests)-1 {
						want = command.Before // the remove's first read
					}
					sameShape(t, name+" group", last, groupShape(want))
				case req.Path == "/v1/environments":
					var page []map[string]any
					if err := json.Unmarshal([]byte(out), &page); err != nil || len(page) != 1 {
						t.Fatalf("request %d: %s = %s, the pinned CLI needs exactly one environment of that name", i+1, path, out)
					}
					sameShape(t, name+" environment list item", page[0], map[string]any{"environment": map[string]any{}, "cursor": ""})
					sameShape(t, name+" listed environment", page[0]["environment"].(map[string]any), environmentShape)
				default:
					var env map[string]any
					_ = json.Unmarshal([]byte(out), &env)
					sameShape(t, name+" environment", env, environmentShape)
				}
			}
			if last == nil {
				t.Fatalf("%s read no group", name)
			}
			// The create's group is the one it made; every other command
			// moves the fixture's group.
			got, _ := last["environmentId"].(string)
			if name != "createIn" {
				got = placementText(placement())
			}
			if want := placementText(command.After); got != bind(want) {
				t.Fatalf("after %s the group reads environmentId %q, the contract says %q", name, got, want)
			}
		})
	}
}

// placementText is a placement as text, "" for none.
func placementText(environmentID *string) string {
	if environmentID == nil {
		return ""
	}
	return *environmentID
}

func provenElsewhere(path string) bool {
	for _, prefix := range cliEnvironmentProvenElsewhere {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
