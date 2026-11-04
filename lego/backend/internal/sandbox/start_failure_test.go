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

package sandbox

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// timeoutWithListing answers every create with the OpenSandbox pod-ready
// timeout and every list with `listed` live sandboxes.
func timeoutWithListing(t *testing.T, listed int, cpu string) *Service {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/sandboxes" {
			items := make([]string, listed)
			for i := range items {
				items[i] = fmt.Sprintf(`{"id":"sbx-%d","status":{"state":"Running"}}`, i)
			}
			_, _ = io.WriteString(w, "["+strings.Join(items, ",")+"]")
			return
		}
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = io.WriteString(w, `{"code":"KUBERNETES::POD_READY_TIMEOUT","message":"sandbox pod not ready within 300s"}`)
	}))
	t.Cleanup(upstream.Close)
	return &Service{
		Base:            &core.Base{Namespace: "default"},
		Client:          NewClient(upstream.URL),
		Templates:       map[string]Template{"node": {Image: "node:20", CPU: cpu, Memory: "1Gi"}},
		DefaultTemplate: "node",
	}
}

func restCreate(t *testing.T, svc *Service) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(`{}`)).WithContext(callerCtx()))
	return rec
}

// A workspace with room under its plan's sandbox limits cannot have been
// refused by the quota: the timeout is a platform failure, never a plan limit
// (w9/074 — a node without gVisor failed every session as "capacity reached").
func TestPodReadyTimeoutWithRoomIsAPlatformFailure(t *testing.T) {
	rec := restCreate(t, timeoutWithListing(t, 1, "2"))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), CodeSandboxStartFailed) {
		t.Fatalf("create = %d %s, want 503 + %s", rec.Code, rec.Body.String(), CodeSandboxStartFailed)
	}
	if strings.Contains(rec.Body.String(), "upgrade") {
		t.Fatalf("a platform failure must not tell the customer to upgrade: %s", rec.Body.String())
	}
}

// When the live sandboxes already fill the plan's limits, capacity is the
// honest answer.
func TestPodReadyTimeoutAtTheLimitStaysACapacityRefusal(t *testing.T) {
	rec := restCreate(t, timeoutWithListing(t, 2, "100"))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), CodeSandboxCapacityLimit) {
		t.Fatalf("create = %d %s, want 409 + %s", rec.Code, rec.Body.String(), CodeSandboxCapacityLimit)
	}
}

func TestStartFailureCodeIsRecognized(t *testing.T) {
	if !IsStartFailure(fmt.Errorf("wrapped: %w", sandboxStartError())) || IsStartFailure(sandboxCapacityError()) {
		t.Fatal("IsStartFailure must match only the start-failure code")
	}
}
