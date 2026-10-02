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
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

func datastoreRegionFixture(t *testing.T, region string, composed bool) http.Handler {
	t.Helper()
	var objects []client.Object
	for i, name := range []string{"alpha", "bravo", "charlie"} {
		db, kv := twinObjects(name)
		for _, object := range []client.Object{db, kv} {
			object.SetCreationTimestamp(metav1.NewTime(time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC)))
			object.SetAnnotations(map[string]string{resourcemeta.UpdatedAtAnnotation: time.Date(2026, 2, i+1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)})
			object.GetLabels()[core.LabelEnvironment] = "evm-prod"
			if i == 0 {
				object.GetLabels()[core.LabelEnvironment] = "evm-staging"
			}
		}
		db.Spec.Name, kv.Spec.Name = name, name
		db.Spec.Plan, kv.Spec.Plan = "free", "free"
		objects = append(objects, db, kv)
	}
	base := &core.Base{Client: fakeClient(objects...), Namespace: "default"}
	if composed {
		h, _ := serverWith(t, base, Deps{Region: region})
		return h
	}
	srv := NewServer(base, Deps{Region: region})
	mux := http.NewServeMux()
	srv.Postgres.RegisterREST(mux)
	srv.KeyValue.RegisterREST(mux)
	return mux
}

type datastoreRegionRow struct {
	ID, Name, Region, Cursor string
}

func readDatastoreRegionPage(t *testing.T, h http.Handler, path, key, query string) []datastoreRegionRow {
	t.Helper()
	res := do(t, h, http.MethodGet, path+"?"+query, testToken, "")
	if res.Code != http.StatusOK {
		t.Fatalf("GET %s?%s = %d: %s", path, query, res.Code, res.Body.String())
	}
	var page []map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil || page == nil {
		t.Fatalf("list response = %s, error %v; want an array", res.Body.String(), err)
	}
	rows := make([]datastoreRegionRow, 0, len(page))
	for _, entry := range page {
		var row datastoreRegionRow
		if err := json.Unmarshal(entry[key], &row); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(entry["cursor"], &row.Cursor); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func TestDatastoreRegionFiltersThroughREST(t *testing.T) {
	for _, composed := range []bool{false, true} {
		mode := "handler"
		if composed {
			mode = "composed"
		}
		t.Run(mode, func(t *testing.T) {
			h := datastoreRegionFixture(t, "oregon", composed)
			for _, route := range []struct{ path, key string }{{"/v1/postgres", "postgres"}, {"/v1/key-value", "keyValue"}} {
				t.Run(route.key, func(t *testing.T) {
					all := []string{"alpha", "bravo", "charlie"}
					for _, tc := range []struct {
						query string
						want  []string
					}{
						{"", all},
						{"region=oregon", all},
						{"region=frankfurt", nil},
						{"region=frankfurt,oregon", all},
						{"region=oregon,frankfurt", all},
						{"region=frankfurt&region=oregon", all},
						{"region=oregon&region=frankfurt", all},
						{"region=frankfurt,singapore", nil},
						{"region=oregon&name=bravo", []string{"bravo"}},
						{"region=oregon&environmentId=evm-staging", []string{"alpha"}},
						{"region=oregon&createdAfter=2026-01-02T12:00:00Z", []string{"charlie"}},
						{"region=oregon&updatedBefore=2026-02-02T12:00:00Z", []string{"alpha", "bravo"}},
						{"region=frankfurt&name=bravo&environmentId=evm-prod&limit=1", nil},
					} {
						t.Run(tc.query, func(t *testing.T) {
							rows := readDatastoreRegionPage(t, h, route.path, route.key, tc.query)
							var names []string
							for _, row := range rows {
								names = append(names, row.Name)
								if row.Region != "oregon" {
									t.Errorf("%s region = %q, want configured oregon", row.Name, row.Region)
								}
							}
							slices.Sort(names)
							if !slices.Equal(names, tc.want) {
								t.Fatalf("names = %v, want %v", names, tc.want)
							}
						})
					}
					// The first unfiltered resource is excluded by the combined
					// predicates. Applying LIMIT before those predicates loses page 1.
					query := url.Values{
						"region": {"frankfurt,oregon"}, "name": {"bravo,charlie"}, "environmentId": {"evm-prod"},
						"createdAfter": {"2026-01-01T12:00:00Z"}, "updatedAfter": {"2026-02-01T12:00:00Z"}, "limit": {"1"},
					}
					for _, want := range []string{"bravo", "charlie", ""} {
						page := readDatastoreRegionPage(t, h, route.path, route.key, query.Encode())
						if want == "" {
							if len(page) != 0 {
								t.Fatalf("final page = %+v, want empty", page)
							}
							break
						}
						if len(page) != 1 || page[0].Name != want || page[0].Cursor == "" {
							t.Fatalf("page = %+v, want only %s with a cursor", page, want)
						}
						query.Set("cursor", page[0].Cursor)
					}
				})
			}
		})
	}
}

func TestDatastoreRegionUsesLiteralPlacement(t *testing.T) {
	for _, region := range []string{"fsn1", ""} {
		t.Run("configured="+region, func(t *testing.T) {
			direct := datastoreRegionFixture(t, region, false)
			composed := datastoreRegionFixture(t, region, true)
			for _, route := range []struct{ path, key string }{{"/v1/postgres", "postgres"}, {"/v1/key-value", "keyValue"}} {
				t.Run(route.key, func(t *testing.T) {
					for _, h := range []http.Handler{direct, composed} {
						page := readDatastoreRegionPage(t, h, route.path, route.key, "")
						if len(page) != 3 || page[0].Region != region {
							t.Fatalf("unfiltered placement = %+v, want 3 resources in %q", page, region)
						}
						if page := readDatastoreRegionPage(t, h, route.path, route.key, "region=frankfurt"); len(page) != 0 {
							t.Fatalf("frankfurt matched configured %q: %+v", region, page)
						}
					}
					page := readDatastoreRegionPage(t, direct, route.path, route.key, "region=fsn1")
					want := 0
					if region == "fsn1" {
						want = 3
					}
					if len(page) != want {
						t.Fatalf("literal handler filter returned %d resources, want %d", len(page), want)
					}
					// The pinned Render enum remains the public gate. A real bex
					// placement is not silently admitted or aliased to Frankfurt.
					for _, query := range []string{
						"region=fsn1",
						"region=frankfurt,fsn1",
						"region=frankfurt&region=fsn1",
						"region=fsn1&region=frankfurt",
						"region=frankfurt&region=bogus",
					} {
						res := do(t, composed, http.MethodGet, route.path+"?"+query, testToken, "")
						if res.Code != http.StatusBadRequest {
							t.Fatalf("%s = %d, want enum rejection: %s", query, res.Code, res.Body.String())
						}
					}
				})
			}
		})
	}
}
