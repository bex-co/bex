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

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type mapEnvironmentResolver struct {
	envs   map[string]core.EnvironmentAssignment
	errs   map[string]error
	admin  bool
	placed []string
}

func (r *mapEnvironmentResolver) ResolveForCreate(_ context.Context, environmentID, workspaceID string) (core.EnvironmentAssignment, error) {
	if err, ok := r.errs[environmentID]; ok {
		return core.EnvironmentAssignment{}, err
	}
	env, ok := r.envs[environmentID]
	if !ok {
		return core.EnvironmentAssignment{}, core.ErrNotFound
	}
	if env.WorkspaceID != "" && env.WorkspaceID != workspaceID {
		return core.EnvironmentAssignment{}, core.ErrForbidden
	}
	return env, nil
}

func (r *mapEnvironmentResolver) RequirePlacement(_ context.Context, assignment core.EnvironmentAssignment) error {
	r.placed = append(r.placed, assignment.ID)
	if !assignment.Protected || r.admin {
		return nil
	}
	return core.PlacementRefusal(assignment.ID, core.ErrForbidden)
}

func TestRecoverEnvironmentID(t *testing.T) {
	const (
		srcEnvID    = "evm-source00000000000000"
		targetEnvID = "evm-target00000000000000"
		protEnvID   = "evm-prot0000000000000000"
		foreignEnv  = "evm-foreign0000000000000"
	)

	makeResolver := func(admin bool) *mapEnvironmentResolver {
		return &mapEnvironmentResolver{
			admin: admin,
			envs: map[string]core.EnvironmentAssignment{
				srcEnvID: {
					ID:          srcEnvID,
					ProjectID:   "prj-src",
					WorkspaceID: "tea-a",
					IPAllowList: []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}},
				},
				targetEnvID: {
					ID:          targetEnvID,
					ProjectID:   "prj-target",
					WorkspaceID: "tea-a",
					IPAllowList: []core.IPAllowListEntry{{CIDRBlock: "192.0.2.0/24"}},
				},
				// Legacy env- spelling resolves to canonical evm- assignment.
				"env-target00000000000000": {
					ID:          targetEnvID,
					ProjectID:   "prj-target",
					WorkspaceID: "tea-a",
					IPAllowList: []core.IPAllowListEntry{{CIDRBlock: "192.0.2.0/24"}},
				},
				protEnvID: {
					ID:          protEnvID,
					ProjectID:   "prj-target",
					WorkspaceID: "tea-a",
					Protected:   true,
				},
				foreignEnv: {
					ID:          foreignEnv,
					ProjectID:   "prj-other",
					WorkspaceID: "tea-b",
				},
			},
		}
	}

	seedSourceInEnv := func(t *testing.T, admin bool) (*Service, *mapEnvironmentResolver) {
		t.Helper()
		svc, src := backedUpSource(t, "basic-256mb")
		svc.Payment = &switchablePaymentGate{bound: true}
		src.Labels[core.LabelEnvironment] = srcEnvID
		src.Labels[core.LabelProject] = "prj-src"
		if err := svc.Client.Update(context.Background(), src); err != nil {
			t.Fatal(err)
		}
		res := makeResolver(admin)
		svc.Environments = res
		return svc, res
	}

	t.Run("explicit environmentId overrides source environment on every surface", func(t *testing.T) {
		for _, surface := range []string{"REST /recovery", "REST /recover", "GraphQL", "MCP"} {
			t.Run(surface, func(t *testing.T) {
				svc, _ := seedSourceInEnv(t, true)
				var gotID, gotEnvID string
				switch surface {
				case "REST /recovery":
					w := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-src/recovery",
						`{"restoreName":"restored","environmentId":"`+targetEnvID+`"}`)
					if w.Code != http.StatusOK {
						t.Fatalf("POST /recovery = %d %s", w.Code, w.Body.String())
					}
					var body struct {
						ID            string `json:"id"`
						EnvironmentID string `json:"environmentId"`
					}
					if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					gotID, gotEnvID = body.ID, body.EnvironmentID
				case "REST /recover":
					w := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-src/recover",
						`{"name":"restored","environmentId":"env-target00000000000000"}`)
					if w.Code != http.StatusCreated {
						t.Fatalf("POST /recover = %d %s", w.Code, w.Body.String())
					}
					var body struct {
						ID            string `json:"id"`
						EnvironmentID string `json:"environmentId"`
					}
					if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					gotID, gotEnvID = body.ID, body.EnvironmentID
				case "GraphQL":
					schema, err := pgGQLSchema(svc)
					if err != nil {
						t.Fatal(err)
					}
					res := graphql.Do(graphql.Params{
						Schema:         schema,
						Context:        ctxAs("user-a"),
						VariableValues: map[string]any{"e": targetEnvID},
						RequestString:  `mutation($e:String) { recoverDatabase(id:"dpg-src", name:"restored", environmentId:$e) { id environmentId } }`,
					})
					if len(res.Errors) > 0 {
						t.Fatalf("recoverDatabase errors = %v", res.Errors)
					}
					rec := res.Data.(map[string]any)["recoverDatabase"].(map[string]any)
					gotID, gotEnvID = rec["id"].(string), rec["environmentId"].(string)
				case "MCP":
					session, cleanup := pgMCPSession(t, svc)
					defer cleanup()
					res, err := session.CallTool(ctxAs("user-a"), &mcp.CallToolParams{
						Name: "recover_postgres",
						Arguments: map[string]any{
							"postgresId":    "dpg-src",
							"name":          "restored",
							"environmentId": targetEnvID,
						},
					})
					if err != nil || res.IsError {
						t.Fatalf("recover_postgres = %+v, err=%v", res, err)
					}
					var view PostgresView
					if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &view); err != nil {
						t.Fatal(err)
					}
					gotID, gotEnvID = view.ID, view.EnvironmentID
				}
				if gotEnvID != id.EnvironmentPublicID(targetEnvID) {
					t.Fatalf("response environmentId = %q, want %q", gotEnvID, targetEnvID)
				}
				var persisted appv1alpha1.Database
				if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: gotID}, &persisted); err != nil {
					t.Fatal(err)
				}
				if persisted.Labels[core.LabelEnvironment] != targetEnvID || persisted.Labels[core.LabelProject] != "prj-target" {
					t.Fatalf("labels = %v, want env=%s project=prj-target", persisted.Labels, targetEnvID)
				}
				if len(persisted.Spec.EnvironmentIPAllowList) != 1 || persisted.Spec.EnvironmentIPAllowList[0] != "192.0.2.0/24" {
					t.Fatalf("EnvironmentIPAllowList = %v, want [192.0.2.0/24]", persisted.Spec.EnvironmentIPAllowList)
				}
			})
		}
	})

	t.Run("omitted environmentId inherits source environment", func(t *testing.T) {
		svc, _ := seedSourceInEnv(t, true)
		view, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored"})
		if err != nil {
			t.Fatal(err)
		}
		if view.EnvironmentID != srcEnvID || view.ProjectID != "prj-src" {
			t.Fatalf("view = %+v, want source env %q and project prj-src", view, srcEnvID)
		}
	})

	t.Run("foreign and unknown environmentId are refused identically to CreatePostgres", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			envID string
			want  error
		}{
			{name: "foreign", envID: foreignEnv, want: core.ErrForbidden},
			{name: "unknown", envID: "evm-missing0000000000000", want: core.ErrNotFound},
		} {
			t.Run(tc.name, func(t *testing.T) {
				svc, _ := seedSourceInEnv(t, true)
				_, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored", EnvironmentID: tc.envID})
				if !errors.Is(err, tc.want) {
					t.Fatalf("Recover(env=%s) = %v, want %v", tc.envID, err, tc.want)
				}
				var list appv1alpha1.DatabaseList
				if err := svc.Client.List(context.Background(), &list); err != nil || len(list.Items) != 1 {
					t.Fatalf("refused recovery wrote a database: count=%d err=%v", len(list.Items), err)
				}
			})
		}
	})

	t.Run("protected target environment requires Admin", func(t *testing.T) {
		for _, admin := range []bool{false, true} {
			svc, res := seedSourceInEnv(t, admin)
			view, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored", EnvironmentID: protEnvID})
			if len(res.placed) != 1 || res.placed[0] != protEnvID {
				t.Fatalf("admin=%v: placed = %v, want [%s]", admin, res.placed, protEnvID)
			}
			var list appv1alpha1.DatabaseList
			if listErr := svc.Client.List(context.Background(), &list); listErr != nil {
				t.Fatal(listErr)
			}
			if !admin {
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Code != core.ProtectedPlacementCode || !errors.Is(err, core.ErrForbidden) {
					t.Fatalf("non-admin recovery into protected target = %v, want %s", err, core.ProtectedPlacementCode)
				}
				if len(list.Items) != 1 {
					t.Fatalf("refused recovery wrote a database: count=%d", len(list.Items))
				}
				continue
			}
			if err != nil || view.EnvironmentID != protEnvID {
				t.Fatalf("admin recovery into protected target = %+v, err=%v", view, err)
			}
		}
	})

	t.Run("datadog recovery fields stay refused by name", func(t *testing.T) {
		svc, _ := seedSourceInEnv(t, true)
		for _, body := range []struct {
			payload string
			field   string
		}{
			{`{"restoreName":"restored","datadogApiKey":"k"}`, "datadogApiKey"},
			{`{"restoreName":"restored","datadogSite":"US1"}`, "datadogSite"},
		} {
			w := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-src/recovery", body.payload)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "recovery field "+body.field+" is unsupported") {
				t.Fatalf("POST /recovery %s = %d %s, want named 400 for %s", body.payload, w.Code, w.Body.String(), body.field)
			}
		}
	})
}
