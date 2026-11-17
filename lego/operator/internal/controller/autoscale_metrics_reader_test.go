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

package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// w4/696: the discovery REST client does not decode an error response's
// Status body, so a refused metrics read logged `"error":"unknown"` and a lost
// RBAC grant read the same as a metrics-server outage. The reader's error now
// names the HTTP status and the server's reason for each.
func TestMetricsServerReaderErrorNamesStatusAndReason(t *testing.T) {
	const forbidden = `{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure",` +
		`"message":"pods.metrics.k8s.io is forbidden: User \"system:serviceaccount:bex-system:bex-controller-manager\" cannot list resource \"pods\" in API group \"metrics.k8s.io\" in the namespace \"tenant\"",` +
		`"reason":"Forbidden","details":{"group":"metrics.k8s.io","kind":"pods"},"code":403}`
	cases := []struct {
		name        string
		status      int
		contentType string
		body        string
		want        []string
		check       func(error) bool
	}{
		{
			name: "rbac refusal", status: http.StatusForbidden, contentType: "application/json", body: forbidden,
			want:  []string{"metrics.k8s.io pods list: 403 Forbidden: ", "cannot list resource \"pods\" in API group \"metrics.k8s.io\""},
			check: apierrors.IsForbidden,
		},
		{
			name: "metrics-server outage", status: http.StatusServiceUnavailable, contentType: "text/plain; charset=utf-8",
			body:  "service unavailable\n",
			want:  []string{"metrics.k8s.io pods list: 503 Service Unavailable: service unavailable"},
			check: apierrors.IsServiceUnavailable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/apis/metrics.k8s.io/v1beta1/namespaces/tenant/pods" {
					t.Errorf("path = %q", r.URL.Path)
				}
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			cs, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			_, err = NewMetricsServerReader(cs)(context.Background(), "tenant", "web")
			if err == nil {
				t.Fatal("want an error")
			}
			msg := err.Error()
			for _, w := range tc.want {
				if !strings.Contains(msg, w) {
					t.Errorf("error %q does not contain %q", msg, w)
				}
			}
			if strings.Contains(msg, "unknown") {
				t.Errorf("error %q still reads unknown", msg)
			}
			if !tc.check(err) {
				t.Errorf("error %q lost its API status", msg)
			}
		})
	}
}
