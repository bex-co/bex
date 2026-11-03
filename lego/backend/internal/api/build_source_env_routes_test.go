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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m254 t004: a build source's env-var, secret-file and env-group routes
// reach only build source carriers — a service id answers 404 — and a carrier
// id passes through to the same handlers a service's routes use (here the
// secrets store is off, so they answer its 503).
func TestBuildSourceEnvironmentRoutesReachOnlyCarriers(t *testing.T) {
	carrier := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "bds-1", Namespace: "default",
			Labels: map[string]string{store.LabelAppID: "bds-1", core.LabelTenant: "tea-a"}},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeBuildSource, Repo: "https://github.com/acme/mono"},
	}
	base := &core.Base{Client: fakeClient(eventsApp(), carrier), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, _ := serverWith(t, base, Deps{})
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/v1/build-sources/%s/env-vars", ""},
		{"PUT", "/v1/build-sources/%s/env-vars", `[]`},
		{"GET", "/v1/build-sources/%s/env-vars/FOO", ""},
		{"DELETE", "/v1/build-sources/%s/env-vars/FOO", ""},
		{"GET", "/v1/build-sources/%s/secret-files", ""},
		{"DELETE", "/v1/build-sources/%s/secret-files/cfg", ""},
		{"POST", "/v1/build-sources/%s/env-groups/evg-1", ""},
		{"DELETE", "/v1/build-sources/%s/env-groups/evg-1", ""},
	} {
		for id, want := range map[string]int{"srv-1": http.StatusNotFound, "web": http.StatusNotFound, "bds-1": 0} {
			res := do(t, h, route.method, fmt.Sprintf(route.path, id), testToken, route.body)
			if want != 0 && res.Code != want {
				t.Errorf("%s %s = %d %s, want %d", route.method, fmt.Sprintf(route.path, id), res.Code, res.Body.String(), want)
			}
			if want == 0 && res.Code == http.StatusNotFound {
				t.Errorf("%s %s = 404 for a build source: %s", route.method, fmt.Sprintf(route.path, id), res.Body.String())
			}
		}
	}
}

// w2/m254 t007: GraphQL reaches a build source's build-time environment
// through the service env verbs with its bds- id as serviceId, while REST's
// /v1/services routes still never reach a carrier.
func TestBuildSourceEnvironmentOverGraphQL(t *testing.T) {
	id := ids.New(ids.BuildSource)
	carrier := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: id, Namespace: "default",
			Labels: map[string]string{store.LabelAppID: id, core.LabelTenant: "tea-a"}},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeBuildSource, Repo: "https://github.com/acme/mono"},
	}
	base := &core.Base{Client: fakeClient(eventsApp(), carrier), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, _ := serverWith(t, base, Deps{})
	query := func(serviceID string) string {
		body, _ := json.Marshal(map[string]string{"query": `{ envVars(serviceId:"` + serviceID + `") { cursor } }`})
		return do(t, h, "POST", "/graphql", testToken, string(body)).Body.String()
	}
	// The secrets store is off in this harness, so reaching the carrier
	// answers its SECRETS_UNAVAILABLE, as REST's build-source route does.
	if got := query(id); !strings.Contains(got, "SECRETS_UNAVAILABLE") {
		t.Errorf("envVars(serviceId: %s) = %s; a build source id reaches its carrier", id, got)
	}
	if got := query(ids.New(ids.BuildSource)); !strings.Contains(got, "not found") {
		t.Errorf("envVars of an unknown build source = %s, want not found", got)
	}
	if res := do(t, h, "GET", "/v1/services/"+id+"/env-vars", testToken, ""); res.Code != http.StatusNotFound {
		t.Errorf("GET /v1/services/%s/env-vars = %d %s, want 404: a service route never reaches a build source", id, res.Code, res.Body.String())
	}
}
