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

package envgroups

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's trigger for a group's linked services is a change: "If you make
// changes to an environment group (including deleting it), Render kicks off a
// new deploy for every linked service that has autodeploys enabled"
// (render.com/docs/configure-environment-variables). Its API pages for the
// group's env-var and secret-file PUTs document no deploy at all, and the
// dashboard offers no Save while nothing is edited. A PUT of what the group
// already holds is no change, yet every single-item verb funnelled it into the
// rollout that PatchEnvironment keeps on purpose for an empty patch (the retry of
// a failed rollout), so it bumped restartedAt and opened a config_change deploy
// on every linked service (w21/111).

// groupWrite is one verb's write of the value the group already holds ("1") or
// of another ("2"), through each surface.
type groupWrite struct {
	name    string
	service func(ctx context.Context, svc *Service, gid, value string) error
	path    func(gid string) string
	body    func(value string) string
	gql     func(gid, value string) string
	mcpTool string
	mcpArgs func(gid, value string) map[string]any
}

var groupWrites = []groupWrite{
	{
		name: "one variable",
		service: func(ctx context.Context, svc *Service, gid, value string) error {
			_, err := svc.SetEnvGroupVar(ctx, gid, "KEEP", value)
			return err
		},
		path: func(gid string) string { return "/v1/env-groups/" + gid + "/env-vars/KEEP" },
		body: func(value string) string { return `{"value":"` + value + `"}` },
		gql: func(gid, value string) string {
			return `mutation { setEnvGroupVar(id:"` + gid + `", key:"KEEP", value:"` + value + `") }`
		},
		mcpTool: "set_env_group_var",
		mcpArgs: func(gid, value string) map[string]any {
			return map[string]any{"id": gid, "key": "KEEP", "value": value}
		},
	},
	{
		name: "the whole variable set",
		service: func(ctx context.Context, svc *Service, gid, value string) error {
			_, err := svc.SetEnvGroupVars(ctx, gid, []EnvVarView{{Key: "KEEP", Value: value}})
			return err
		},
		path: func(gid string) string { return "/v1/env-groups/" + gid + "/env-vars" },
		body: func(value string) string { return `[{"key":"KEEP","value":"` + value + `"}]` },
		gql: func(gid, value string) string {
			return `mutation { setEnvGroupVars(id:"` + gid + `", envVars:[{key:"KEEP", value:"` + value + `"}]) }`
		},
		mcpTool: "update_env_group_vars",
		mcpArgs: func(gid, value string) map[string]any {
			return map[string]any{"id": gid, "envVars": []map[string]any{{"key": "KEEP", "value": value}}}
		},
	},
	{
		name: "one secret file",
		service: func(ctx context.Context, svc *Service, gid, value string) error {
			_, err := svc.SetEnvGroupFile(ctx, gid, "keep.txt", value)
			return err
		},
		path: func(gid string) string { return "/v1/env-groups/" + gid + "/secret-files/keep.txt" },
		body: func(value string) string { return `{"content":"` + value + `"}` },
		gql: func(gid, value string) string {
			return `mutation { setEnvGroupSecretFile(id:"` + gid + `", name:"keep.txt", content:"` + value + `") }`
		},
		mcpTool: "set_env_group_secret_file",
		mcpArgs: func(gid, value string) map[string]any {
			return map[string]any{"id": gid, "name": "keep.txt", "content": value}
		},
	},
}

// groupRig is a control-plane group "shared" holding KEEP=1 and keep.txt=1,
// linked to two services, with a deploy recorder and a clock that moves, so a
// roll shows as a new restartedAt, a deploy row and a new App resourceVersion.
type groupRig struct {
	svc  *Service
	rec  *recordingDeploys
	gid  string
	tick int
}

func newGroupRig(t *testing.T) *groupRig {
	t.Helper()
	rec := &recordingDeploys{}
	svc := newService(newFakeStore(), managedApp("web"), managedApp("worker"))
	svc.Rollout = &rollout.Tracker{Store: rec}
	rig := &groupRig{svc: svc, rec: rec}
	svc.Base.Clock = func() time.Time {
		rig.tick++
		return time.Unix(2_000_000+int64(rig.tick), 0).UTC()
	}
	group, err := svc.CreateEnvGroup(testCtx(), CreateEnvGroupRequest{
		Name:        "shared",
		EnvVars:     []CreateEnvVarInput{{Key: "KEEP", Value: "1", ValueSet: true}},
		SecretFiles: []SecretFileView{{Name: "keep.txt", Content: "1"}},
		ServiceIDs:  []string{"web", "worker"},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	rig.gid = group.ID
	rec.rows = nil
	return rig
}

// groupState is everything a roll or a store write moves.
type groupState struct {
	revision        string
	rows            int
	web, worker     string // App resourceVersions
	webAt, workerAt string
}

func (g *groupRig) state(t *testing.T) groupState {
	t.Helper()
	view, err := g.svc.GetEnvGroup(testCtx(), g.gid)
	if err != nil {
		t.Fatalf("get group: %v", err)
	}
	web, worker := getApp(t, g.svc.Client, "web"), getApp(t, g.svc.Client, "worker")
	return groupState{
		revision: view.Revision, rows: len(g.rec.rows),
		web: web.ResourceVersion, worker: worker.ResourceVersion,
		webAt: web.Spec.RestartedAt, workerAt: worker.Spec.RestartedAt,
	}
}

// surfaces runs one write through the Service and each adapter.
func (w groupWrite) surfaces(t *testing.T, rig *groupRig) map[string]func(value string) {
	t.Helper()
	cs := mcpSession(t, rig.svc)
	return map[string]func(value string){
		"service": func(value string) {
			if err := w.service(testCtx(), rig.svc, rig.gid, value); err != nil {
				t.Fatalf("service write: %v", err)
			}
		},
		"REST": func(value string) {
			rec := serveREST(rig.svc, http.MethodPut, w.path(rig.gid), w.body(value))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST write = %d %s, want 200", rec.Code, rec.Body.String())
			}
		},
		"GraphQL": func(value string) {
			res := graphql.Do(graphql.Params{Schema: *envGroupSchema(t, rig.svc), Context: testCtx(), RequestString: w.gql(rig.gid, value)})
			if len(res.Errors) > 0 {
				t.Fatalf("GraphQL write: %v", res.Errors)
			}
		},
		"MCP": func(value string) {
			res, err := cs.CallTool(testCtx(), &mcp.CallToolParams{Name: w.mcpTool, Arguments: w.mcpArgs(rig.gid, value)})
			if err != nil || res.IsError {
				t.Fatalf("MCP write: err=%v result=%+v", err, res)
			}
		},
	}
}

// A write of the value the group holds answers as before and changes nothing: no
// store write (the revision stands), no App patch, no deploy row, on the Service
// and on REST, GraphQL and MCP. A write of another value still rolls every linked
// service once.
func TestAGroupWriteOfTheStoredValueRollsNoLinkedService(t *testing.T) {
	for _, w := range groupWrites {
		for _, surface := range []string{"service", "REST", "GraphQL", "MCP"} {
			t.Run(w.name+"/"+surface, func(t *testing.T) {
				rig := newGroupRig(t)
				write := w.surfaces(t, rig)[surface]

				before := rig.state(t)
				write("1")
				if after := rig.state(t); after != before {
					t.Fatalf("a write of the stored value changed the group or its services:\nbefore %+v\nafter  %+v", before, after)
				}
				write("1")
				if after := rig.state(t); after != before {
					t.Fatalf("the same write again changed something:\nbefore %+v\nafter  %+v", before, after)
				}

				write("2")
				changed := rig.state(t)
				if changed.revision == before.revision {
					t.Fatalf("a write of another value left the revision %q", changed.revision)
				}
				if changed.rows != 2 || changed.webAt == before.webAt || changed.workerAt == before.workerAt {
					t.Fatalf("a write of another value must roll both linked services once, got %+v from %+v", changed, before)
				}
				for _, row := range rig.rec.rows {
					if row.Trigger != store.TriggerConfigChange {
						t.Fatalf("deploy row trigger = %q, want %q", row.Trigger, store.TriggerConfigChange)
					}
				}

				// And the new value stored is again no change.
				write("2")
				if after := rig.state(t); after != changed {
					t.Fatalf("a write of the value just stored changed something:\nbefore %+v\nafter  %+v", changed, after)
				}
			})
		}
	}
}

// A delete of a variable or a file the group holds is a change and rolls; the
// delete of one it does not hold answers 404 and changes nothing, as before.
func TestAGroupDeleteStillRollsWhenItRemovesSomething(t *testing.T) {
	for name, path := range map[string]string{"variable": "/env-vars/KEEP", "file": "/secret-files/keep.txt"} {
		t.Run(name, func(t *testing.T) {
			rig := newGroupRig(t)
			before := rig.state(t)
			rec := serveREST(rig.svc, http.MethodDelete, "/v1/env-groups/"+rig.gid+path, "")
			if rec.Code != http.StatusNoContent {
				t.Fatalf("DELETE = %d %s, want 204", rec.Code, rec.Body.String())
			}
			removed := rig.state(t)
			if removed.revision == before.revision || removed.rows != 2 {
				t.Fatalf("a delete that removed something must store and roll both services, got %+v from %+v", removed, before)
			}
			rec = serveREST(rig.svc, http.MethodDelete, "/v1/env-groups/"+rig.gid+path, "")
			if rec.Code != http.StatusNotFound {
				t.Fatalf("DELETE of the absent name = %d, want 404", rec.Code)
			}
			if after := rig.state(t); after != removed {
				t.Fatalf("a delete of an absent name changed something:\nbefore %+v\nafter  %+v", removed, after)
			}
		})
	}
}

// The retry of a failed rollout is a different request from an identical write:
// the empty patch rolls every linked service at the group's revision whatever
// the group holds, on REST, GraphQL and MCP, while an identical single-item write
// made in between rolls none.
func TestTheEmptyPatchStillRetriesAFailedRollout(t *testing.T) {
	for _, surface := range []string{"service", "REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			rig := newGroupRig(t)
			failing := true
			rig.svc.Base.Client = interceptor.NewClient(rig.svc.Client.(client.WithWatch), interceptor.Funcs{
				Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					if a, ok := obj.(*appv1alpha1.App); ok && a.Name == "worker" && failing {
						return errors.New("worker is unreachable")
					}
					return c.Patch(ctx, obj, patch, opts...)
				},
			})

			saved, err := rig.svc.PatchEnvironment(testCtx(), rig.gid, EnvironmentPatch{
				EnvVars: []EnvVarPatch{{Key: "KEEP", Value: "2", ValueSet: true}}, SaveMode: SaveModeDeploy,
			})
			if err != nil || saved.RolledOut || !slices.Equal(saved.FailedServiceIDs, []string{"srv-worker"}) {
				t.Fatalf("the faulted save = %+v err=%v, want srv-worker failed", saved, err)
			}

			// The same value again, through the single-item verb: nothing rolls, so
			// it cannot stand in for the retry.
			failing = false
			before := rig.state(t)
			if err := groupWrites[0].service(testCtx(), rig.svc, rig.gid, "2"); err != nil {
				t.Fatalf("identical write: %v", err)
			}
			if after := rig.state(t); after != before {
				t.Fatalf("an identical write after the failed rollout changed something:\nbefore %+v\nafter  %+v", before, after)
			}

			// The retry is the empty patch at the revision the save returned.
			var rolledOut bool
			switch surface {
			case "service":
				retry, err := rig.svc.PatchEnvironment(testCtx(), rig.gid, EnvironmentPatch{SaveMode: SaveModeDeploy, ExpectedRevision: &saved.Revision})
				if err != nil {
					t.Fatalf("retry: %v", err)
				}
				rolledOut = retry.RolledOut
			case "REST":
				rec := serveREST(rig.svc, http.MethodPatch, "/v1/env-groups/"+rig.gid+"/contents", `{"saveMode":"deploy","expectedRevision":"`+saved.Revision+`"}`)
				rolledOut = rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), `"rolledOut":true`)
			case "GraphQL":
				res := graphql.Do(graphql.Params{Schema: *envGroupSchema(t, rig.svc), Context: testCtx(), RequestString: `mutation { patchEnvGroupEnvironment(id:"` + rig.gid + `", saveMode:"deploy", expectedRevision:"` + saved.Revision + `") { rolledOut } }`})
				if len(res.Errors) > 0 {
					t.Fatalf("retry: %v", res.Errors)
				}
				rolledOut = res.Data.(map[string]any)["patchEnvGroupEnvironment"].(map[string]any)["rolledOut"] == true
			case "MCP":
				res, err := mcpSession(t, rig.svc).CallTool(testCtx(), &mcp.CallToolParams{Name: "patch_env_group_environment", Arguments: map[string]any{
					"id": rig.gid, "saveMode": "deploy", "expectedRevision": saved.Revision,
				}})
				if err != nil || res.IsError {
					t.Fatalf("retry: err=%v result=%+v", err, res)
				}
				structured, _ := json.Marshal(res.StructuredContent)
				rolledOut = strings.Contains(string(structured), `"rolledOut":true`)
			}
			if !rolledOut {
				t.Fatal("the empty patch did not report a completed rollout")
			}
			retried := rig.state(t)
			if retried.workerAt == before.workerAt || retried.webAt == before.webAt {
				t.Fatalf("the empty patch must roll every linked service again, got %+v from %+v", retried, before)
			}
			if retried.revision != before.revision {
				t.Fatalf("the retry moved the revision %q -> %q", before.revision, retried.revision)
			}
		})
	}
}
