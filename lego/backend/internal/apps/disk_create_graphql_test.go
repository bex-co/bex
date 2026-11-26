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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// GraphQL createService(disk:) creates the service with its disk in one write
// (w4/m652): the disk row the meter bills and the service's disk read back,
// for each type Render attaches disks to.
func TestGraphQLCreateServiceAttachesADisk(t *testing.T) {
	for _, svcType := range []string{"web_service", "private_service", "background_worker"} {
		t.Run(svcType, func(t *testing.T) {
			svc, st, ctx := newDiskStoreService()
			schema := mustSchema(t, svc)
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `mutation {
				createService(name: "web", type: "` + svcType + `", repo: "https://github.com/o/r", plan: "starter",
					disk: {name: "data", mountPath: "/var/data", sizeGB: 5}) { id disk { name mountPath sizeGB } } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("createService(disk:): %v", res.Errors)
			}
			created := res.Data.(map[string]any)["createService"].(map[string]any)
			want := map[string]any{"name": "data", "mountPath": "/var/data", "sizeGB": 5}
			if got := created["disk"]; !reflect.DeepEqual(got, want) {
				t.Fatalf("createService.disk = %v, want %v", got, want)
			}
			if len(st.disks) != 1 {
				t.Fatalf("store holds %d disks, want the one row the meter bills", len(st.disks))
			}
			for _, d := range st.disks {
				if d.Name != "data" || d.MountPath != "/var/data" || d.SizeGB != 5 {
					t.Fatalf("disk row = %+v, want data at /var/data, 5 GB", d)
				}
			}
			read := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ server(id: "` + created["id"].(string) + `") { disk { name mountPath sizeGB } } }`})
			if len(read.Errors) > 0 {
				t.Fatalf("server { disk }: %v", read.Errors)
			}
			if got := read.Data.(map[string]any)["server"].(map[string]any)["disk"]; !reflect.DeepEqual(got, want) {
				t.Fatalf("server { disk } = %v, want %v", got, want)
			}
		})
	}
}

// sizeGB is optional, as in Render's serviceDisk and REST's
// serviceDetails.disk: the create fills Render's default.
func TestGraphQLCreateServiceDiskDefaultsItsSize(t *testing.T) {
	svc, _, ctx := newDiskStoreService()
	res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: ctx, RequestString: `mutation {
		createService(name: "web", repo: "https://github.com/o/r", plan: "starter",
			disk: {name: "data", mountPath: "/var/data"}) { disk { sizeGB } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService(disk:) without sizeGB: %v", res.Errors)
	}
	if got := res.Data.(map[string]any)["createService"].(map[string]any)["disk"].(map[string]any)["sizeGB"]; got != int(diskDefaultSizeGB) {
		t.Fatalf("omitted sizeGB => %v, want Render's default %d", got, diskDefaultSizeGB)
	}
}

// Every create GraphQL refuses for its disk, REST refuses with the same 400
// message, so the two surfaces cannot drift. Autoscaling, the fifth rule,
// cannot be sent to GraphQL's createService, which takes no autoscaling.
func TestGraphQLCreateServiceRefusesADiskAsRESTDoes(t *testing.T) {
	const disk = `{"name":"data","mountPath":"/var/data","sizeGB":5}`
	cases := []struct {
		name, rest, gql, want string
	}{{
		name: "static site",
		rest: `{"name":"site","type":"static_site","repo":"https://github.com/o/r","serviceDetails":{"publishPath":"dist","disk":` + disk + `}}`,
		gql:  `name: "site", type: "static_site", repo: "https://github.com/o/r", publishPath: "dist"`,
		want: "a static_site cannot have a disk",
	}, {
		name: "cron job",
		rest: `{"name":"job","type":"cron_job","repo":"https://github.com/o/r","serviceDetails":{"plan":"starter","schedule":"0 * * * *","disk":` + disk + `}}`,
		gql:  `name: "job", type: "cron_job", repo: "https://github.com/o/r", plan: "starter", schedule: "0 * * * *"`,
		want: "a cron_job cannot have a disk",
	}, {
		name: "free plan",
		rest: `{"name":"web","type":"web_service","repo":"https://github.com/o/r","serviceDetails":{"plan":"free","disk":` + disk + `}}`,
		gql:  `name: "web", type: "web_service", repo: "https://github.com/o/r", plan: "free"`,
		want: diskPaidPlanRefusal,
	}, {
		name: "two instances",
		rest: `{"name":"web","type":"web_service","repo":"https://github.com/o/r","serviceDetails":{"plan":"starter","numInstances":2,"disk":` + disk + `}}`,
		gql:  `name: "web", type: "web_service", repo: "https://github.com/o/r", plan: "starter", replicas: 2`,
		want: diskInstancesRefusal(2),
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restSvc, restStore, restCtx := newDiskStoreService()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/services", strings.NewReader(tc.rest)).WithContext(restCtx)
			validated(t, diskRESTMux(t, restSvc)).ServeHTTP(rec, req)
			var body struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusBadRequest {
				t.Fatalf("REST create => %d %s, want 400", rec.Code, rec.Body)
			}

			gqlSvc, gqlStore, gqlCtx := newDiskStoreService()
			res := graphql.Do(graphql.Params{Schema: mustSchema(t, gqlSvc), Context: gqlCtx,
				RequestString: `mutation { createService(` + tc.gql + `, disk: {name: "data", mountPath: "/var/data", sizeGB: 5}) { id } }`})
			if len(res.Errors) != 1 {
				t.Fatalf("GraphQL create errors = %v, want one refusal", res.Errors)
			}
			if got := res.Errors[0].Message; got != body.Message || !strings.Contains(got, tc.want) {
				t.Fatalf("GraphQL refusal %q, REST %q; want both to state %q", got, body.Message, tc.want)
			}
			if len(restStore.appCreates)+len(gqlStore.appCreates)+len(restStore.disks)+len(gqlStore.disks) != 0 {
				t.Fatal("a refused create wrote a service or a disk")
			}
		})
	}
}

// newDiskStoreService is the store-backed create path (a resolved workspace
// and a store that keeps disk rows), so a create's disk lands where the meter
// reads it; ctx is the caller signed in to that workspace.
func newDiskStoreService() (*Service, *recordingStore, context.Context) {
	st := &recordingStore{disks: map[string]store.Disk{}}
	svc, _ := newTenantStoreService(fakeWorkspace{"qa": "tea-qa"}, st)
	return svc, st, core.WithIdentity(context.Background(), core.Identity{Subject: "qa", Method: "session"})
}
