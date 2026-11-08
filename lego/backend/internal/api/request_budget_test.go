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

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w5/m286: a REST handler waiting on a slow dependency is stopped at the
// budget and answers REQUEST_TIMEOUT, inside the write timeout, without the
// dependency's (internal) error text.
func TestRequestBudgetStopsASlowRESTHandler(t *testing.T) {
	s := &Server{restBudget: 50 * time.Millisecond}
	stopped := make(chan struct{})
	h := s.withRequestBudget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(stopped)
		core.WriteErr(w, fmt.Errorf("list apps: Get https://10.0.0.1:6443/apis: %w", r.Context().Err()))
	}))
	rec := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services", nil))
	<-stopped
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("handler ran %s, want it stopped at the budget", took)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusServiceUnavailable || body["code"] != core.RequestTimeoutCode || body["message"] != core.RequestTimedOutMessage {
		t.Fatalf("response = %d %v, want 503 REQUEST_TIMEOUT", rec.Code, body)
	}
	if got := rec.Body.String(); strings.Contains(got, "10.0.0.1") {
		t.Errorf("timeout body leaks the dependency's error: %s", got)
	}
}

// Streaming and long-wait routes keep their own deadlines.
func TestRequestBudgetExemptsStreamingRoutes(t *testing.T) {
	s := &Server{restBudget: time.Millisecond}
	for _, c := range []struct {
		method, path string
		exempt       bool
	}{
		{http.MethodGet, "/v1/logs/subscribe", true},
		{http.MethodGet, "/v1/logs", false},
		{http.MethodPost, "/v1/sandboxes/sbx-1/exec", true},
		{http.MethodPost, "/v1/agent-attachments", true},
		{http.MethodGet, "/v1/agent-attachments/att-1/content", true},
		{http.MethodPost, "/v1/services/srv-1/restart", false},
	} {
		var deadline bool
		h := s.withRequestBudget(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			_, deadline = r.Context().Deadline()
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(c.method, c.path, nil))
		if deadline == c.exempt {
			t.Errorf("%s %s: deadline=%v, want exempt=%v", c.method, c.path, deadline, c.exempt)
		}
	}
}

// The budget must end before the write timeout (or the answer is a 502) and
// after wait_agent_sessions' 25 s wait (or its timedOut answer is lost).
func TestRequestBudgetSitsInsideTheWriteTimeout(t *testing.T) {
	if core.RequestBudget >= HTTPWriteTimeout || core.RequestBudget <= 25*time.Second {
		t.Fatalf("RequestBudget = %s, want between 25s and HTTPWriteTimeout %s", core.RequestBudget, HTTPWriteTimeout)
	}
}
