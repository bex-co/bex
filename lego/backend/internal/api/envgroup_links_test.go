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
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w5/m120 through the wired server: a group links a service by its id, and
// deleting the service over REST removes it from the group.
func TestEnvGroups_DeletingAServiceUnlinksIt(t *testing.T) {
	webID := ids.New(ids.Service)
	web := sampleApp("web")
	web.Labels = map[string]string{core.LabelAppID: webID, core.LabelServiceName: "web", core.LabelTenant: "tea-cli"}
	base := &core.Base{Client: fakeClient(web), Namespace: "default", Workspace: fakeWorkspace{"client-1": "tea-cli"},
		Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}
	h, _ := serverWith(t, base, Deps{Secrets: newMemSecretStore()})

	created := do(t, h, "POST", "/v1/env-groups", testToken, `{"name":"shared"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create group = %d %s", created.Code, created.Body)
	}
	group := decodeJSON[envgroups.EnvGroupView](t, created.Result())
	// The stored link set, which GraphQL lists: REST names only the services
	// that still exist, so it could not show a deleted service left linked.
	links := func() []string {
		t.Helper()
		data := gql(t, h, `{ envGroup(id: "`+group.ID+`") { serviceLinks } }`)
		ids := []string{}
		for _, link := range data["envGroup"].(map[string]any)["serviceLinks"].([]any) {
			ids = append(ids, link.(string))
		}
		return ids
	}
	if res := do(t, h, "POST", "/v1/env-groups/"+group.ID+"/services/web", testToken, ""); res.Code != http.StatusOK {
		t.Fatalf("link by name = %d %s", res.Code, res.Body)
	} else if errs := loadRenderSpec(t).validate("link-service-to-env-group", res.Body.Bytes()); len(errs) > 0 {
		// Render answers a link with the updated envGroup (w8/063).
		t.Fatalf("link response diverges from envGroup: %v\n%s", errs, res.Body)
	}
	if got := links(); !slices.Equal(got, []string{webID}) {
		t.Fatalf("serviceLinks = %v, want the service id %s", got, webID)
	}
	if res := do(t, h, "DELETE", "/v1/services/"+webID, testToken, ""); res.Code >= 300 {
		t.Fatalf("delete service = %d %s", res.Code, res.Body)
	}
	if got := links(); len(got) != 0 {
		t.Fatalf("serviceLinks after the service was deleted = %v, want none", got)
	}
}
