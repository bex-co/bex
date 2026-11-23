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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// protectedDatastores is a Postgres and a Key Value of workspace tea-a in
// environment env-1, each with its connection Secret, served by a bex-api whose
// environments read status for every environment.
func protectedDatastores(t *testing.T, authz core.Checker, status string) (http.Handler, *Server, *appv1alpha1.Database, *appv1alpha1.KeyValue) {
	t.Helper()
	db, kv := ownedDB(ids.New(ids.Postgres), "tea-a"), ownedKV(ids.New(ids.KeyValue), "tea-a")
	db.Labels[core.LabelEnvironment], kv.Labels[core.LabelEnvironment] = "env-1", "env-1"
	db.Status = appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady, SecretName: db.Name + "-app"}
	kv.Status = appv1alpha1.KeyValueStatus{Phase: appv1alpha1.KVPhaseReady, SecretName: kv.Name}
	secret := func(name, uri string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Data: map[string][]byte{"password": []byte("s3cret"), "uri": []byte(uri)}}
	}
	base := &core.Base{
		Client: fakeClient(db, kv,
			secret(db.Name+"-app", "postgresql://u:s3cret@"+db.Name+"-rw.default:5432/app"),
			secret(kv.Name, "redis://default:s3cret@"+kv.Name+":6379")),
		Namespace: "default", Workspace: twoWorkspaceResolver{}, Authz: authz,
	}
	handler, srv := serverWith(t, base, Deps{DeployStore: &conformDeployStore{}, Store: protectedLifecycleStore{status: status}})
	return handler, srv, db, kv
}

func graphQL(t *testing.T, handler http.Handler, query string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"query": query})
	response := do(t, handler, http.MethodPost, "/graphql", testToken, string(body))
	var out map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s: %v %s", query, err, response.Body.String())
	}
	return out
}

// TestProtectedCredentialsAndIPRulesRefuseADeveloperOnEverySurface (w4/m594):
// a Developer's datastore credential read and IP-rule change on a member of a
// protected environment answer the same coded refusal over GraphQL and MCP, and
// a 403 naming the action over REST, where Render's operations carry only
// Render's error codes (w8/411). Nothing the refusal says carries the password.
func TestProtectedCredentialsAndIPRulesRefuseADeveloperOnEverySurface(t *testing.T) {
	handler, srv, db, kv := protectedDatastores(t, developerChecker{}, core.ProtectedStatusProtected)

	t.Run("GraphQL", func(t *testing.T) {
		for _, query := range []string{
			fmt.Sprintf(`{databaseConnectionInfo(id:%q){password}}`, db.Name),
			fmt.Sprintf(`{keyValueConnectionInfo(id:%q){internalConnectionString}}`, kv.Name),
			fmt.Sprintf(`mutation{setDatabaseIpAllowList(id:%q,cidrs:["198.51.100.0/24"]){id}}`, db.Name),
		} {
			out := graphQL(t, handler, query)
			if code := graphqlCode(out); code != core.ProtectedPlacementCode {
				t.Errorf("%s answered code %q (%v), want %s", query, code, out, core.ProtectedPlacementCode)
			}
			if raw, _ := json.Marshal(out); strings.Contains(string(raw), "s3cret") {
				t.Errorf("%s leaked the password: %s", query, raw)
			}
		}
	})

	t.Run("REST", func(t *testing.T) {
		for _, path := range []string{"/v1/postgres/" + db.Name + "/connection-info", "/v1/key-value/" + kv.Name + "/connection-info"} {
			res := do(t, handler, http.MethodGet, path, testToken, "")
			if res.Code != http.StatusForbidden || !strings.Contains(res.Body.String(), "only workspace Admins can view credentials") ||
				strings.Contains(res.Body.String(), "s3cret") {
				t.Errorf("GET %s = %d %s, want 403 naming the action", path, res.Code, res.Body)
			}
		}
	})

	t.Run("MCP", func(t *testing.T) {
		id := core.Identity{Subject: "dana", Method: "session"}
		for tool, args := range map[string]map[string]any{
			"update_postgres":  {"postgresId": db.Name, "ipAllowList": []map[string]string{{"cidrBlock": "198.51.100.0/24", "description": "vpn"}}},
			"update_key_value": {"keyValueId": kv.Name, "ipAllowList": []map[string]string{{"cidrBlock": "198.51.100.0/24", "description": "vpn"}}},
		} {
			res := mcpCall(t, srv, id, tool, args)
			if text := mcpText(res); !res.IsError || !strings.HasPrefix(text, core.ProtectedPlacementCode+":") {
				t.Errorf("%s = %q (error %t), want the %s refusal", tool, text, res.IsError, core.ProtectedPlacementCode)
			}
		}
	})
}

// TestProtectedAdminRowsGraphQL (w4/m594): over GraphQL, databaseActions and
// keyValueActions carry view_connection_info and update_ip_allow_list, denied
// with protected_environment_admin_required for a Developer on a member of a
// protected environment, and allowed for an Admin and on an unprotected one.
func TestProtectedAdminRowsGraphQL(t *testing.T) {
	for _, tc := range []struct {
		name       string
		authz      core.Checker
		status     string
		wantReason string
	}{
		{"developer, protected", developerChecker{}, core.ProtectedStatusProtected, core.ReasonProtectedEnvironmentAdmin},
		{"admin, protected", allowOwnChecker{}, core.ProtectedStatusProtected, ""},
		{"developer, unprotected", developerChecker{}, core.ProtectedStatusUnprotected, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, _, db, kv := protectedDatastores(t, tc.authz, tc.status)
			for field, target := range map[string]string{"databaseActions": db.Name, "keyValueActions": kv.Name} {
				raw, _ := json.Marshal(graphQL(t, handler, fmt.Sprintf(`{%s(id:%q,ownerId:"tea-a"){action outcome reason precondition}}`, field, target)))
				var result struct {
					Data   map[string][]core.ActionDecision `json:"data"`
					Errors []struct{ Message string }       `json:"errors"`
				}
				if err := json.Unmarshal(raw, &result); err != nil || len(result.Errors) != 0 {
					t.Fatalf("%s: %v %s", field, err, raw)
				}
				seen := 0
				for _, row := range result.Data[field] {
					if row.Action != core.ActionViewConnectionInfo && row.Action != core.ActionUpdateIPAllowList {
						continue
					}
					seen++
					want := core.ActionDecision{Action: row.Action, Outcome: core.DecisionAllowed}
					if tc.wantReason != "" {
						want.Outcome, want.Reason = core.DecisionDenied, tc.wantReason
					}
					if row != want {
						t.Errorf("%s %s = %+v, want %+v", field, row.Action, row, want)
					}
				}
				if seen != 2 {
					t.Errorf("%s: saw %d of the two rows in %+v", field, seen, result.Data[field])
				}
			}
		})
	}
}

// TestProtectedLifecycleVerbsRefuseADeveloperWithTheCode (w4/728): suspend,
// resume, delete, turning maintenance mode on and opening a Shell on a member
// of a protected environment answer a Developer the coded refusal naming that
// environment over GraphQL and MCP, as credentials and IP rules do, so every
// client tells "only an Admin may do this here" from any other refusal by one
// code.
func TestProtectedLifecycleVerbsRefuseADeveloperWithTheCode(t *testing.T) {
	serve := func() (http.Handler, *Server, string, string, string) {
		web := ownedApp("web", "tea-a")
		web.Labels[core.LabelAppID] = ids.New(ids.Service)
		web.Labels[store.LabelManagedBy] = store.ManagedByValue
		web.Spec.Type, web.Spec.Tier = appv1alpha1.TypeWebService, "standard"
		db, kv := ownedDB(ids.New(ids.Postgres), "tea-a"), ownedKV(ids.New(ids.KeyValue), "tea-a")
		for _, labels := range []map[string]string{web.Labels, db.Labels, kv.Labels} {
			labels[core.LabelEnvironment] = "env-1"
		}
		base := &core.Base{Client: fakeClient(web, db, kv), Namespace: "default", Workspace: twoWorkspaceResolver{}, Authz: developerChecker{}}
		handler, srv := serverWith(t, base, Deps{DeployStore: &conformDeployStore{}, Store: protectedLifecycleStore{status: core.ProtectedStatusProtected}})
		return handler, srv, web.Labels[core.LabelAppID], db.Name, kv.Name
	}
	mutations := func(service, db, kv string) []string {
		return []string{
			fmt.Sprintf(`mutation{suspendService(id:%q){id}}`, service),
			fmt.Sprintf(`mutation{resumeService(id:%q){id}}`, service),
			fmt.Sprintf(`mutation{deleteService(id:%q)}`, service),
			fmt.Sprintf(`mutation{setMaintenanceMode(id:%q,maintenanceMode:{enabled:true}){id}}`, service),
			fmt.Sprintf(`mutation{createShellSession(id:%q){ticket}}`, service),
			fmt.Sprintf(`mutation{suspendDatabase(id:%q){id}}`, db),
			fmt.Sprintf(`mutation{resumeDatabase(id:%q){id}}`, db),
			fmt.Sprintf(`mutation{deleteDatabase(id:%q)}`, db),
			fmt.Sprintf(`mutation{suspendKeyValue(id:%q){id}}`, kv),
			fmt.Sprintf(`mutation{resumeKeyValue(id:%q){id}}`, kv),
			fmt.Sprintf(`mutation{deleteKeyValue(id:%q)}`, kv),
		}
	}

	t.Run("GraphQL", func(t *testing.T) {
		handler, _, service, db, kv := serve()
		for _, query := range mutations(service, db, kv) {
			out := graphQL(t, handler, query)
			errs, _ := out["errors"].([]any)
			var ext map[string]any
			if len(errs) > 0 {
				ext, _ = errs[0].(map[string]any)["extensions"].(map[string]any)
			}
			if code := graphqlCode(out); code != core.ProtectedPlacementCode || ext["environmentId"] != "env-1" {
				t.Errorf("%s answered %v, want %s naming env-1", query, out, core.ProtectedPlacementCode)
			}
		}
	})

	t.Run("MCP", func(t *testing.T) {
		_, srv, service, db, kv := serve()
		id := core.Identity{Subject: "dana", Method: "session"}
		for _, call := range []struct {
			tool string
			args map[string]any
		}{
			{"suspend_service", map[string]any{"serviceId": service}},
			{"resume_service", map[string]any{"serviceId": service}},
			{"delete_service", map[string]any{"serviceId": service}},
			{"update_service", map[string]any{"serviceId": service, "maintenanceMode": map[string]any{"enabled": true}}},
			{"suspend_postgres", map[string]any{"postgresId": db}},
			{"resume_postgres", map[string]any{"postgresId": db}},
			{"suspend_keyvalue", map[string]any{"keyValueId": kv}},
			{"resume_keyvalue", map[string]any{"keyValueId": kv}},
		} {
			res := mcpCall(t, srv, id, call.tool, call.args)
			if text := mcpText(res); !res.IsError || !strings.HasPrefix(text, core.ProtectedPlacementCode+":") {
				t.Errorf("%s = %q (error %t), want the %s refusal", call.tool, text, res.IsError, core.ProtectedPlacementCode)
			}
		}
	})
}
