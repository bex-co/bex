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
	"encoding/json"
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestRenderInsightViews(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromFile("../api/openapi/render-public-api-1.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		got  any
		want string
	}{
		{"processes", toRenderProcesses([]ProcessView{{PID: 7, UserName: "alice", ApplicationName: "psql", State: "active", Query: "SELECT 1", DurationSeconds: 12}}), `{"processes":[{"pid":7,"username":"alice","applicationName":"psql","state":"active","query":"SELECT 1","duration":12}]}`},
		{"top-queries", toRenderTopQueries([]TopQueryView{{Query: "SELECT $1", Calls: 2, TotalTimeMs: 9.5, MeanTimeMs: 4.75, Rows: 3, SharedHitBlks: 7, SharedReadBlks: 8}}), `{"topQueries":[{"query":"SELECT $1","calls":2,"totalTimeMs":9.5,"meanTimeMs":4.75,"rows":3,"sharedBlocksHit":7,"sharedBlocksRead":8}]}`},
		{"sizes", toRenderSizes(SizesView{Database: DatabaseSizeView{Name: "app", SizeBytes: 100}, Tables: []TableSizeView{{Schema: "public", Name: "widgets", SizeBytes: 40}}}), `{"sizes":[{"Database":"app","Bytes":100},{"Database":"app","Schema":"public","Table":"widgets","Bytes":40}]}`},
		{"table-scans", toRenderTableScans([]TableScanView{{Schema: "public", Name: "widgets", SeqScans: 5, IndexScans: 9}}), `{"tableScans":[{"Schema":"public","Table":"widgets","Scans":5}]}`},
		{"processes", toRenderProcesses(nil), `{"processes":[]}`},
		{"top-queries", toRenderTopQueries(nil), `{"topQueries":[]}`},
		{"table-scans", toRenderTableScans(nil), `{"tableScans":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.got)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %s, want %s", raw, tc.want)
			}
			schema := spec.Paths.Value("/postgres/{postgresId}/query/" + tc.name).Get.Responses.Status(200).Value.Content.Get("application/json").Schema.Value
			if err := schema.VisitJSON(got); err != nil {
				t.Fatal(err)
			}
		})
	}
}
