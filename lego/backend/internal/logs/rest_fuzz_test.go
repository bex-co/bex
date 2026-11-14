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
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// logQueryFuzzSeeds are the raw query strings every logs REST surface accepts,
// harvested from the package's own tests: the documented filters, the class
// shorthand, wildcard and `/…/` text values, malformed timestamps and limits,
// and the shapes validate() must refuse. Seeds live as source so a reviewer can
// see the untrusted surface being probed; the on-disk testdata/fuzz cache is
// never committed.
func logQueryFuzzSeeds(f *testing.F) {
	f.Helper()
	seeds := []string{
		"",
		"resource=web",
		"resource=web&resource=worker",
		"resource=web&type=build&direction=forward",
		"resource=web&type=predeploy",
		"resource=web&type=predeploy&type=build",
		"resource=web&type=all&type=request",
		"resource=web&type=request&statusCode=2xx&level=warning&level=error",
		"resource=web&text=/foo.*/&text=bar*&text=plain",
		"resource=web&text=/[",
		"resource=web&textMode=display&direction=backward",
		"resource=web&startTime=2026-01-02T15:04:05Z&endTime=2026-01-02T15:04:06Z",
		"resource=web&startTime=bad&endTime=worse",
		"resource=web&limit=0",
		"resource=web&limit=-1",
		"resource=web&limit=999999999999999999999",
		"resource=web&host=/.*/&path=/x/&method=GET&instance=web-1",
		"resource=web&deploy=dpl-1&type=build",
		"resource=web&sandbox=a&task=b",
		"resource=" + strings.Repeat("x", 5000),
		"resource=%00&text=%00",
		"&&&",
		"resource=web&unknown=1",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
}

// FuzzParseLogParams drives the shared logs REST query parser with arbitrary
// client-supplied query strings: parseLogParams, the verb-level validate(), the
// normalizer, and the LogQL selector/query builders every read goes through. It
// must never panic; a query it accepts must produce a selector that starts with
// a `{` and closes with `}` so no filter value can break out of a matcher.
func FuzzParseLogParams(f *testing.F) {
	logQueryFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, rawQuery string) {
		// Build the request without httptest.NewRequest: an arbitrary query can
		// be an unparseable URL, which that helper panics on. The handlers read
		// r.URL.Query(), which parses the raw string the same way for any input.
		req := &http.Request{URL: &url.URL{Path: "/v1/logs", RawQuery: rawQuery}, Header: http.Header{}}
		_, q, err := parseLogParams(req)
		if err != nil {
			return
		}
		if err := q.validate(); err != nil {
			return
		}
		nq := q.normalized()
		selector := lokiSelectorFor("default", nq)
		if !strings.HasPrefix(selector, "{") || !strings.HasSuffix(selector, "}") {
			t.Fatalf("selector %q for %q is not a balanced stream selector", selector, rawQuery)
		}
		_ = lokiQueryFor("default", nq)
	})
}
