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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// Path discovery (w3/084): a path is a request line's field, not a stream
// label, so the store answers it from a sample of the newest request lines,
// each projected to its path.

// The read asks Loki for the newest lokiPathSample lines of the query's own
// streams, projected to the path, and answers the paths most often seen,
// sorted: a service whose paths carry ids lists its common routes.
func TestLokiPathValuesSampleTheNewestRequestLines(t *testing.T) {
	// /zz sorts last but is seen most; / is seen twice; the rest once.
	values := []string{`["3","/"]`, `["2","/"]`, `["1","/zz"]`, `["4","/zz"]`, `["5","/zz"]`}
	for i := range 2 * maxPathValues {
		values = append(values, fmt.Sprintf(`["%d","/users/%03d"]`, 10+i, i))
	}
	f := newFakeLoki(lokiResp(
		map[string]any{"stream": `{"app":"web","request_path":"/"}`, "values": "[" + strings.Join(values[:2], ",") + "]"},
		map[string]any{"stream": `{"app":"web","request_path":"many"}`, "values": "[" + strings.Join(values[2:], ",") + "]"},
	))
	defer f.srv.Close()

	since := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	end := since.Add(time.Hour)
	got, err := NewLokiLabelValuesSource(f.srv.URL, f.srv.Client())(context.Background(), "ws", LabelPath,
		LogQuery{App: "web", Types: []string{LogTypeRequest}, Host: []string{"shop.example.com"}, Since: since, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastPath != "/loki/api/v1/query_range" {
		t.Fatalf("path discovery read %s, want a query_range", f.lastPath)
	}
	want := `{namespace="ws", app="web", type="request"} | json request_path="RequestPath", request_host="RequestHost" | request_host="shop.example.com" | request_path!="" | line_format "{{.request_path}}"`
	if q := f.lastValues.Get("query"); q != want {
		t.Errorf("query = %s\nwant      %s", q, want)
	}
	if f.lastValues.Get("limit") != strconv.Itoa(lokiPathSample) || f.lastValues.Get("direction") != DirectionBackward {
		t.Errorf("sample = limit %s, direction %s; want the newest %d lines", f.lastValues.Get("limit"), f.lastValues.Get("direction"), lokiPathSample)
	}
	if f.lastValues.Get("start") != strconv.FormatInt(since.UnixNano(), 10) {
		t.Errorf("start = %s, want the query's", f.lastValues.Get("start"))
	}

	if len(got) != maxPathValues || !slices.IsSorted(got) {
		t.Fatalf("paths = %d values (sorted %v), want the %d most frequent, sorted", len(got), slices.IsSorted(got), maxPathValues)
	}
	// The two seen more than once are kept, and the first one-offs in order
	// fill the bound.
	if got[0] != "/" || got[1] != "/users/000" || got[len(got)-2] != fmt.Sprintf("/users/%03d", maxPathValues-3) || got[len(got)-1] != "/zz" {
		t.Errorf("paths = %v…%v", got[:2], got[len(got)-2:])
	}
}

// On the pinned Loki (BEX_TEST_LOKI_URL), path discovery lists the paths of
// the service's own request lines in the window: not another service's, not
// an app line that happens to carry a RequestPath, not a request line with
// none, and not one a negated path filter excludes.
func TestPathValuesOnThePinnedLoki(t *testing.T) {
	base := os.Getenv("BEX_TEST_LOKI_URL")
	if base == "" {
		testenv.Skip(t, "BEX_TEST_LOKI_URL not set")
	}
	base = strings.TrimRight(base, "/")
	ns := fmt.Sprintf("w3-084-%d", time.Now().UnixNano())
	now := time.Now()
	since := now.Add(-time.Minute)
	n := 0
	push := func(app string, labels map[string]string, record map[string]any) {
		t.Helper()
		n++
		line, _ := json.Marshal(record)
		labels["namespace"], labels["app"] = ns, app
		values := [][2]string{{strconv.FormatInt(since.Add(time.Duration(n)*time.Second).UnixNano(), 10), string(line)}}
		body, _ := json.Marshal(map[string]any{"streams": []any{map[string]any{"stream": labels, "values": values}}})
		resp, err := http.Post(base+"/loki/api/v1/push", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("push: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			t.Fatalf("push: HTTP %d", resp.StatusCode)
		}
	}
	request := func(app, status, path string) {
		record := map[string]any{"RequestHost": app + ".onbex.co"}
		if path != "" {
			record["RequestPath"] = path
		}
		push(app, map[string]string{"type": LogTypeRequest, "method": "GET", "status": status, "level": statusLevel(status)}, record)
	}
	request("web", "200", "/healthz")
	request("web", "200", "/healthz")
	request("web", "404", "/api/users?id=7")
	request("web", "500", "/api/orders")
	request("web", "200", "") // an access record with no path
	request("other", "200", "/admin")
	push("web", map[string]string{"container": "app", "type": LogTypeApp, "pod": "web-1", "level": "info"},
		map[string]any{"RequestPath": "/from-an-app-line"})

	discover := func(q LogQuery) []string {
		t.Helper()
		q.App, q.Types, q.Since, q.End = "web", []string{LogTypeRequest}, since, now
		var got []string
		// The store indexes pushed lines within a moment; ask until they are in.
		for range 50 {
			paths, err := NewLokiLabelValuesSource(base, nil)(context.Background(), ns, LabelPath, q)
			if err != nil {
				t.Fatal(err)
			}
			if got = paths; len(got) > 0 {
				return got
			}
			time.Sleep(100 * time.Millisecond)
		}
		return got
	}
	if got, want := discover(LogQuery{}), []string{"/api/orders", "/api/users?id=7", "/healthz"}; !slices.Equal(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
	q := LogQuery{Path: []string{"/healthz"}, negated: []string{LabelPath}}
	if got, want := discover(q), []string{"/api/orders", "/api/users?id=7"}; !slices.Equal(got, want) {
		t.Errorf("paths not /healthz = %v, want %v", got, want)
	}
}
