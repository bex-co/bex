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
	"slices"
	"testing"
)

// accessRecordFixture is a stored access record as production logged it
// (w4/m325's finding), with the client fields the edge logs since.
const accessRecordFixture = `{"DownstreamContentSize":311,"DownstreamStatus":503,"Duration":8384760,` +
	`"KubernetesIngressName":"tea-daif693dqjvc73e7as3g-qa-20261111-lg89","KubernetesIngressNamespace":"tea-daif693dqjvc73e7as3g",` +
	`"RequestHost":"qa-20261111-lg89.onbex.co","RequestMethod":"GET","RequestPath":"/qa89-err",` +
	`"ServiceName":"tea-daif693dqjvc73e7as3g-tea-daif693dqjvc73e7as3g-qa-20261111-lg89-8080@kubernetes",` +
	`"downstream_Rndr-Id":"ad8c2d92-90ac-42a8-8c03-c8b6f8776c79","level":"info","msg":"",` +
	`"origin_Rndr-Id":"ad8c2d92-90ac-42a8-8c03-c8b6f8776c79","request_Rndr-Id":"ad8c2d92-90ac-42a8-8c03-c8b6f8776c79",` +
	`"request_True-Client-Ip":"198.51.100.3","request_User-Agent":"curl/8.7.1 \"quoted\"","time":"2026-11-11T08:55:49Z"}`

// accessRecordSummary is that record as Render's request line reads
// (docs/render-artifacts/request-logs.md).
const accessRecordSummary = `clientIP="198.51.100.3" requestID="ad8c2d92-90ac-42a8-8c03-c8b6f8776c79" ` +
	`responseTimeMS=8 responseBytes=311 userAgent="curl/8.7.1 \"quoted\""`

// TestRequestLinesReadLikeRenders (w4/m325): a request line's message is
// Render's summary and its labels are Render's (method, statusCode, host,
// path, level), on REST, GraphQL and MCP; the requestID is in the message,
// and only GraphQL also has it as a field. An app line is untouched.
func TestRequestLinesReadLikeRenders(t *testing.T) {
	const id = "ad8c2d92-90ac-42a8-8c03-c8b6f8776c79"
	records, _ := json.Marshal([][]string{{"1760950000000000000", accessRecordFixture}})
	f := newFakeLoki(lokiResp(
		map[string]any{
			"stream": `{"app":"web","type":"request","method":"GET","status":"503"}`,
			"values": string(records),
		},
		map[string]any{
			"stream": `{"app":"web","pod":"web-1","container":"app","level":"info"}`,
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
	if got := entries[0].Message; got != accessRecordSummary {
		t.Errorf("request message =\n%s\nwant\n%s", got, accessRecordSummary)
	}
	if got := entries[1].Message; got != "handled "+id {
		t.Errorf("app message = %q", got)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/logs?resource=web", nil))
	var page struct {
		Logs []struct {
			Message string                         `json:"message"`
			Labels  []struct{ Name, Value string } `json:"labels"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || len(page.Logs) != 2 {
		t.Fatalf("REST = %d %s (err %v)", rec.Code, rec.Body, err)
	}
	wantLabels := map[string][]string{
		accessRecordSummary: {"resource=web", "method=GET", "statusCode=503", "host=qa-20261111-lg89.onbex.co", "path=/qa89-err", "level=error", "type=request"},
		// The instance is the pod's public id; its value is not under test.
		"handled " + id: {"resource=web", "instance=*", "level=info", "type=app"},
	}
	for _, row := range page.Logs {
		var got []string
		for _, l := range row.Labels {
			if l.Name == LabelInstance {
				l.Value = "*"
			}
			got = append(got, l.Name+"="+l.Value)
		}
		if want := wantLabels[row.Message]; !slices.Equal(got, want) {
			t.Errorf("REST %q labels = %v, want %v", row.Message, got, want)
		}
	}

	// text matches what a request line reads, not the stored record.
	for search, want := range map[string]int{"responseTimeMS=8": 1, "KubernetesIngressName": 0, "qa-20261111-lg89.onbex.co": 0} {
		got, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeRequest}, Search: []string{search}})
		if err != nil || len(got) != want {
			t.Errorf("text=%s matched %d lines (err %v), want %d", search, len(got), err, want)
		}
	}

	for _, e := range mcpLogEntries(entries) {
		if _, ok := e.Labels[LabelRequestID]; ok {
			t.Errorf("MCP labels carry requestID: %v", e.Labels)
		}
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web") { logs { type message requestId host path level } } }`)
	for _, row := range data["logs"].(map[string]any)["logs"].([]any) {
		r := row.(map[string]any)
		want := map[string]any{"type": "app", "message": "handled " + id, "requestId": "", "host": "", "path": "", "level": "info"}
		if r["type"] == LogTypeRequest {
			want = map[string]any{"type": "request", "message": accessRecordSummary, "requestId": id, "host": "qa-20261111-lg89.onbex.co", "path": "/qa89-err", "level": "error"}
		}
		for k, v := range want {
			if r[k] != v {
				t.Errorf("GraphQL %v row %s = %v, want %v", r["type"], k, r[k], v)
			}
		}
	}
}

// A request line shipped before the edge logged its client, size or request
// id shows what it has, and keeps its host, path and level labels; one that is
// not an access record is returned as stored.
func TestRequestLineKeepsWhatItCannotProject(t *testing.T) {
	for _, tc := range []struct{ stored, status, want, path, level string }{
		{`{"DownstreamStatus":404,"Duration":1999999,"RequestPath":"/old"}`, "404", `responseTimeMS=1`, "/old", "warning"},
		{`{"Duration":5000000,"downstream_Rndr-Id":"r-1"}`, "200", `requestID="r-1" responseTimeMS=5`, "", "info"},
		{`{"RequestPath":"/no-fields"}`, "502", `{"RequestPath":"/no-fields"}`, "/no-fields", "error"},
		{`not an access record`, "200", `not an access record`, "", ""},
	} {
		labels := map[string]string{LabelStatusCode: tc.status}
		got := requestLine(tc.stored, labels)
		if got != tc.want || labels[LabelPath] != tc.path || labels[LabelLevel] != tc.level {
			t.Errorf("requestLine(%s) = %q, path %q, level %q; want %q, %q, %q",
				tc.stored, got, labels[LabelPath], labels[LabelLevel], tc.want, tc.path, tc.level)
		}
	}
}
