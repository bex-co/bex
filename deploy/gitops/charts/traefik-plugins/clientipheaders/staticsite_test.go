// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bex_client_ip_headers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// w8/363: a static site's route marks exactly the requests whose client sent
// no Accept-Encoding, whatever the caller sent as the marker itself.
func TestStaticSiteMarksNoAcceptEncoding(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header http.Header
		want   string
	}{
		{"no Accept-Encoding", http.Header{}, "1"},
		{"an empty Accept-Encoding", http.Header{"Accept-Encoding": {""}}, "1"},
		{"a forged marker with no Accept-Encoding", http.Header{noAcceptEncodingHeader: {"0"}}, "1"},
		{"br", http.Header{"Accept-Encoding": {"br"}}, ""},
		{"gzip", http.Header{"Accept-Encoding": {"gzip"}}, ""},
		{"identity", http.Header{"Accept-Encoding": {"identity"}}, ""},
		{"a forged marker with gzip", http.Header{"Accept-Encoding": {"gzip"}, noAcceptEncodingHeader: {"1"}}, ""},
	} {
		var got []string
		reached := false
		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			reached = true
			got = r.Header.Values(noAcceptEncodingHeader)
		})
		h, err := New(context.Background(), next, &Config{StaticSite: true}, "static-site")
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodGet, "https://site.onbex.co/favicon.ico", nil)
		r.Header = tc.header
		h.ServeHTTP(httptest.NewRecorder(), r)
		want := []string(nil)
		if tc.want != "" {
			want = []string{tc.want}
		}
		if !reached || len(got) != len(want) || len(want) == 1 && got[0] != want[0] {
			t.Errorf("%s: reached %v, marker %q, want %q", tc.name, reached, got, want)
		}
	}
}
