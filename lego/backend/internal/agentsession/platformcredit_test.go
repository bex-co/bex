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

package agentsession

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// The monitor reads the platform key's own limit and usage from OpenRouter's
// GET /api/v1/key with the key as Bearer, exports usage against the limit,
// and never puts the key in an error.
func TestPlatformCreditMonitorExportsLimitAndUsage(t *testing.T) {
	var body string
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		if body == "" {
			http.Error(w, "nope "+testPlatformKey, http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	m := NewPlatformCreditMonitor(prometheus.NewRegistry(), testPlatformKey)
	m.URL, m.HTTP = srv.URL, srv.Client()

	for _, tc := range []struct {
		body         string
		limit, usage float64
	}{
		// A monthly-reset limit: usage is what counts against it.
		{`{"data":{"label":"x","limit":1000,"limit_remaining":150,"limit_reset":"monthly","usage":4321.5}}`, 1000, 850},
		{`{"data":{"limit":500,"usage":420}}`, 500, 420},
		{`{"data":{"limit":null,"limit_remaining":null,"usage":12.5}}`, 0, 12.5},
	} {
		body = tc.body
		if err := m.Poll(context.Background()); err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		if got := testutil.ToFloat64(m.limit); got != tc.limit {
			t.Errorf("%s: limit = %v, want %v", tc.body, got, tc.limit)
		}
		if got := testutil.ToFloat64(m.usage); got != tc.usage {
			t.Errorf("%s: usage = %v, want %v", tc.body, got, tc.usage)
		}
	}
	if sawAuth != "Bearer "+testPlatformKey {
		t.Fatal("the key must be sent as a Bearer token")
	}
	for _, bad := range []string{"", `{"data":{}}`, `not json`} {
		body = bad
		err := m.Poll(context.Background())
		if err == nil || strings.Contains(err.Error(), testPlatformKey) {
			t.Fatalf("%q: err = %v, want a key-free error", bad, err)
		}
	}
	if got := testutil.ToFloat64(m.usage); got != 12.5 {
		t.Fatalf("a failed poll changed the last usage: %v", got)
	}
}
