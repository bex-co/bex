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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type auditRows struct {
	mu     sync.Mutex
	events []core.AuditEvent
}

func (a *auditRows) Record(_ context.Context, ev core.AuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, ev)
	return nil
}

func (a *auditRows) verbs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []string
	for _, ev := range a.events {
		out = append(out, string(ev.Outcome)+" "+ev.Verb)
	}
	return out
}

// allowFirst allows the first n checks and refuses the rest: the admission of a
// live tail passes, and a follower's own check, made afterwards in a goroutine of
// its own, is refused (a membership revoked in between).
type allowFirst struct {
	n     int64
	calls atomic.Int64
}

func (c *allowFirst) Check(context.Context, string, string, string) (bool, error) {
	return c.calls.Add(1) <= c.n, nil
}

// w21/093: GET /v1/logs/subscribe is mounted as a method value, and a tail of
// several resources follows each in a goroutine, so a refusal on either path
// recorded the name of the frame it ran on (http.HandlerFunc.ServeHTTP,
// logs.followAll.func1). Both record the verb the other surfaces' tail records.
func TestLogsSubscribeRecordsFollowLogsOnEveryPath(t *testing.T) {
	for _, tc := range []struct {
		name      string
		resources string
		allowed   int64
		want      []string
	}{
		{"the admission is refused", "web", 0, []string{"denied " + AuditVerbFollowLogs}},
		{"a follower's own check is refused", "web&resource=api", 2, []string{"denied " + AuditVerbFollowLogs, "denied " + AuditVerbFollowLogs}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(map[string][]string{}, sampleApp("web"), sampleApp("api"), podFor("web", "web-1"), podFor("api", "api-1"))
			rows := &auditRows{}
			svc.Base.Audit = rows
			svc.Base.Authz = &allowFirst{n: tc.allowed}
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?resource="+tc.resources, nil)
			req.Header.Set("Accept", "text/event-stream")
			req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user-1", Method: core.MethodSession}))
			serveSubscribe(mux, rec, req)

			got := rows.verbs()
			if len(got) != len(tc.want) {
				t.Fatalf("recorded %v, want %v", got, tc.want)
			}
			for i, row := range got {
				if row != tc.want[i] || strings.Contains(row, "func") {
					t.Fatalf("recorded %v, want %v", got, tc.want)
				}
			}
		})
	}
}
