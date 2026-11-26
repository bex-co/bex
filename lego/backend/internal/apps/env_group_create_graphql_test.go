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

package apps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/envgroups/envgroupstest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// appWriteCounter records the first App a create writes and counts every
// later write to an App's spec, each of which would roll another release.
type appWriteCounter struct {
	client.Client
	first  *appv1alpha1.App
	writes int
}

func (c *appWriteCounter) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if a, ok := obj.(*appv1alpha1.App); ok && !isDryRunCreate(opts) && c.first == nil {
		c.first = a.DeepCopy()
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *appWriteCounter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*appv1alpha1.App); ok {
		c.writes++
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *appWriteCounter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*appv1alpha1.App); ok {
		c.writes++
	}
	return c.Client.Update(ctx, obj, opts...)
}

func isDryRunCreate(opts []client.CreateOption) bool {
	var o client.CreateOptions
	o.ApplyOptions(opts)
	return len(o.DryRun) > 0
}

// newEnvGroupCreateService is the store-backed create path with the real
// env-groups seam and one group, "shared", in the caller's workspace.
func newEnvGroupCreateService(t *testing.T) (*Service, *recordingStore, *appWriteCounter, *envgroups.Service, string, context.Context) {
	t.Helper()
	svc, st, ctx := newDiskStoreService()
	counter := &appWriteCounter{Client: svc.Client}
	svc.Client = counter
	groups := &envgroups.Service{Base: svc.Base, Store: newMemKV(), Groups: envgroupstest.New()}
	svc.EnvGroups = groups
	group, err := groups.CreateEnvGroup(ctx, envgroups.CreateEnvGroupRequest{Name: "shared",
		EnvVars: []envgroups.CreateEnvVarInput{{Key: "SHARED_KEY", Value: "from-the-group"}}})
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, counter, groups, group.ID, ctx
}

// GraphQL createService(envGroups:) links the chosen groups as the service is
// written (w4/731): its first App already carries the group's values, so the
// first deploy has them and no second release is rolled for the link.
func TestGraphQLCreateServiceLinksEnvGroupsBeforeTheFirstDeploy(t *testing.T) {
	svc, st, counter, groups, gid, ctx := newEnvGroupCreateService(t)
	res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: ctx, RequestString: `mutation {
		createService(name: "web", repo: "https://github.com/o/r", plan: "starter", envGroups: ["` + gid + `"]) { id latestDeployId } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService(envGroups:): %v", res.Errors)
	}
	created := res.Data.(map[string]any)["createService"].(map[string]any)
	serviceID := created["id"].(string)
	// The create still answers its first deploy, which the dashboard opens.
	if created["latestDeployId"] == nil {
		t.Fatal("createService answered no latestDeployId: the dashboard would not open the first deploy")
	}
	if counter.first == nil || !reflect.DeepEqual(counter.first.Spec.EnvFromSecrets, []string{gid + "-env"}) ||
		!reflect.DeepEqual(counter.first.Spec.FilesFromSecrets, []string{gid + "-files"}) {
		t.Fatalf("the first App write does not carry the group: %+v", counter.first)
	}
	if counter.writes != 0 {
		t.Fatalf("the create wrote the App %d more times, rolling a release for the link", counter.writes)
	}
	if len(st.appCreates) != 1 || !st.appCreates[0].CreationPending {
		t.Fatalf("store rows = %+v, want one row held pending until the link commits", st.appCreates)
	}
	got, err := groups.GetEnvGroup(ctx, gid)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.ServiceLinks, []string{serviceID}) {
		t.Fatalf("group links = %v, want [%s]", got.ServiceLinks, serviceID)
	}
	read := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: ctx,
		RequestString: `{ server(id: "` + serviceID + `") { linkedEnvGroupIds } }`})
	if len(read.Errors) > 0 {
		t.Fatalf("server { linkedEnvGroupIds }: %v", read.Errors)
	}
	if ids := read.Data.(map[string]any)["server"].(map[string]any)["linkedEnvGroupIds"]; !reflect.DeepEqual(ids, []any{gid}) {
		t.Fatalf("server { linkedEnvGroupIds } = %v, want [%s]", ids, gid)
	}
}

// A group the create cannot link refuses the whole create before its first
// write, a dry-run included, so a preview never answers "would succeed".
func TestGraphQLCreateServiceRefusesAnUnknownEnvGroupBeforeWriting(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "dryRun"}[dryRun], func(t *testing.T) {
			svc, st, counter, groups, gid, ctx := newEnvGroupCreateService(t)
			const unknown = "evg-d0000000000000000000"
			dry := ""
			if dryRun {
				dry = ", dryRun: true"
			}
			res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: ctx, RequestString: `mutation {
				createService(name: "web", repo: "https://github.com/o/r", plan: "starter", envGroups: ["` + gid + `", "` + unknown + `"]` + dry + `) { id } }`})
			if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, `env group "`+unknown+`" does not exist`) {
				t.Fatalf("errors = %v, want the unknown group named", res.Errors)
			}
			if len(st.appCreates) != 0 || counter.first != nil {
				t.Fatalf("a refused create wrote a row (%d) or an App (%v)", len(st.appCreates), counter.first != nil)
			}
			got, err := groups.GetEnvGroup(ctx, gid)
			if err != nil || len(got.ServiceLinks) != 0 {
				t.Fatalf("a refused create linked the known group: %v (%v)", got.ServiceLinks, err)
			}
		})
	}
}

// The create form's groups ride GraphQL only: Render's POST /v1/services has
// no environment-group field, so REST's strict decoding (the composed
// server's) refuses one as any field Render does not declare.
func TestRESTCreateServiceTakesNoEnvGroups(t *testing.T) {
	svc, st, _, _, gid, ctx := newEnvGroupCreateService(t)
	rec := httptest.NewRecorder()
	body := `{"name":"web","type":"web_service","repo":"https://github.com/o/r","envGroups":["` + gid + `"]}`
	validated(t, diskRESTMux(t, svc)).ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)).WithContext(core.WithStrictJSONDecoding(ctx)))
	if rec.Code != http.StatusBadRequest || len(st.appCreates) != 0 {
		t.Fatalf("REST create with envGroups => %d %s (rows %d), want a 400 and nothing written", rec.Code, rec.Body, len(st.appCreates))
	}
}
