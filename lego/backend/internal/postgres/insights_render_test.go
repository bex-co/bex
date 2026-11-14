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
		{"processes", toRenderProcesses([]ProcessView{{PID: 7, UserName: "alice", ApplicationName: "psql", State: "active", Query: "SELECT 1", DurationSeconds: 12,
			DatabaseName: "app", ClientAddr: "10.0.0.7", BackendStart: "2026-11-13T11:00:00.000000Z", QueryStart: "2026-11-13T11:25:00.000000Z"}}),
			`{"processes":[{"pid":7,"databaseName":"app","username":"alice","applicationName":"psql","clientAddr":"10.0.0.7","backendStart":"2026-11-13T11:00:00.000000Z","queryStart":"2026-11-13T11:25:00.000000Z","state":"active","query":"SELECT 1","duration":12,"isLeader":false}]}`},
		{"top-queries", toRenderTopQueries([]TopQueryView{{QueryID: "-42", Query: "SELECT $1", Calls: 2, TotalTimeMs: 9.5, MinTimeMs: 1, MaxTimeMs: 8.5, MeanTimeMs: 4.75, StddevTimeMs: 3.75, Rows: 3,
			SharedHitBlks: 7, SharedReadBlks: 8, SharedDirtiedBlks: 1, SharedWrittenBlks: 2, LocalHitBlks: 3, LocalReadBlks: 4, LocalDirtiedBlks: 5, LocalWrittenBlks: 6, TempReadBlks: 9, TempWrittenBlks: 10}}),
			`{"topQueries":[{"queryId":"-42","query":"SELECT $1","calls":2,"totalTimeMs":9.5,"minTimeMs":1,"maxTimeMs":8.5,"meanTimeMs":4.75,"stddevTimeMs":3.75,"rows":3,"sharedBlocksHit":7,"sharedBlocksRead":8,"sharedBlocksDirtied":1,"sharedBlocksWritten":2,"localBlocksHit":3,"localBlocksRead":4,"localBlocksDirtied":5,"localBlocksWritten":6,"tempBlocksRead":9,"tempBlocksWritten":10}]}`},
		// w4/634: indexes are rows of their own, ranked with the tables by size.
		{"sizes", toRenderSizes(SizesView{Database: DatabaseSizeView{Name: "app", SizeBytes: 100}, Tables: []TableSizeView{{Schema: "public", Name: "widgets", SizeBytes: 40}, {Schema: "public", Name: "tags", SizeBytes: 8}},
			Indexes: []IndexSizeView{{Schema: "public", Table: "widgets", Name: "widgets_pkey", SizeBytes: 16}}}),
			`{"sizes":[{"Database":"app","Bytes":100},{"Database":"app","Schema":"public","Table":"widgets","Bytes":40},{"Database":"app","Schema":"public","Table":"widgets","Index":"widgets_pkey","Bytes":16},{"Database":"app","Schema":"public","Table":"tags","Bytes":8}]}`},
		{"table-scans", toRenderTableScans([]TableScanView{{Database: "app", Schema: "public", Name: "widgets", SeqScans: 5, IndexScans: 9}}), `{"tableScans":[{"Database":"app","Schema":"public","Table":"widgets","Scans":5}]}`},
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
