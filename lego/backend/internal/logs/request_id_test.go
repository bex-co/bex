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

package logs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequestLinesCarryRequestID (w8/m125): a request line's edge-minted
// Rndr-Id is its requestID on the entry, the REST labels and GraphQL; an app
// line has none.
func TestRequestLinesCarryRequestID(t *testing.T) {
	const id = "a657a6df-c727-4ff4-adaf-5bbe8a02e05c"
	f := newFakeLoki(lokiResp(
		map[string]any{
			"stream": `{"app":"web","type":"request","method":"GET","status":"200"}`,
			"values": `[["1760950000000000000","{\"RequestPath\":\"/x\",\"request_Rndr-Id\":\"` + id + `\"}"]]`,
		},
		map[string]any{
			"stream": `{"app":"web","pod":"web-1","container":"app"}`,
			"values": `[["1760950001000000000","handled ` + id + `"]]`,
		},
	))
	defer f.srv.Close()
	svc := newService(nil, sampleApp("web"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())

	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web"})
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries = %+v (err %v)", entries, err)
	}
	for _, e := range entries {
		want := ""
		if e.Labels[LabelType] == LogTypeRequest {
			want = id
		}
		if got := e.Labels[LabelRequestID]; got != want {
			t.Errorf("%s entry requestID = %q, want %q", e.Labels[LabelType], got, want)
		}
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/logs?resource=web&type=request", nil))
	var page struct {
		Logs []struct {
			Labels []struct{ Name, Value string } `json:"labels"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || len(page.Logs) == 0 {
		t.Fatalf("REST = %d %s (err %v)", rec.Code, rec.Body, err)
	}
	found := false
	for _, row := range page.Logs {
		for _, l := range row.Labels {
			found = found || (l.Name == LabelRequestID && l.Value == id)
		}
	}
	if !found {
		t.Errorf("REST logs = %+v, want a request row labelled requestID %s", page.Logs, id)
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web") { logs { type requestId } } }`)
	for _, row := range data["logs"].(map[string]any)["logs"].([]any) {
		r := row.(map[string]any)
		want := ""
		if r["type"] == LogTypeRequest {
			want = id
		}
		if r["requestId"] != want {
			t.Errorf("GraphQL %v row requestId = %v, want %q", r["type"], r["requestId"], want)
		}
	}
}
