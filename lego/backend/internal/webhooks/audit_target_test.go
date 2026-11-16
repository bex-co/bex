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

package webhooks

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type auditSink struct{ events []core.AuditEvent }

func (s *auditSink) Record(_ context.Context, ev core.AuditEvent) error {
	s.events = append(s.events, ev)
	return nil
}

func (s *auditSink) allowed(verb string) []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range s.events {
		if ev.Verb == verb && ev.Outcome == core.AuditAllowed {
			out = append(out, ev)
		}
	}
	return out
}

// w4/m321: a webhook create records one row once the endpoint exists, naming
// it, and a refused create records none; a delete names the endpoint.
func TestWebhookCreateAndDeleteRowsNameTheEndpoint(t *testing.T) {
	s, _ := newTestService()
	sink := &auditSink{}
	s.Audit = sink
	ctx := context.Background()

	if _, err := s.Create(ctx, CreateRequest{Name: "x", URL: "not-a-url", EventTypes: []string{TypeDeployStarted}}); err == nil {
		t.Fatal("a bad URL must be refused")
	}
	if got := sink.allowed("webhooks.Create"); len(got) != 0 {
		t.Fatalf("a refused create recorded %+v", got)
	}
	created, err := s.Create(ctx, CreateRequest{Name: "primary", URL: "https://example.com/hook", EventTypes: []string{TypeDeployStarted}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	rows := sink.allowed("webhooks.Create")
	if len(rows) != 1 || rows[0].Target != core.WebhookTarget(created.ID) || rows[0].TargetName != "primary" {
		t.Fatalf("create rows = %+v, want one naming %s primary", rows, created.ID)
	}

	if err := s.Delete(ctx, "", created.ID); err != nil {
		t.Fatal(err)
	}
	rows = sink.allowed("webhooks.Delete")
	if len(rows) != 1 || rows[0].Target != core.WebhookTarget(created.ID) {
		t.Fatalf("delete rows = %+v, want one targeting %s", rows, created.ID)
	}
}
