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

package envgroups

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type auditRows struct{ events []core.AuditEvent }

func (a *auditRows) Record(_ context.Context, ev core.AuditEvent) error {
	a.events = append(a.events, ev)
	return nil
}

// refuseRelation allows every check but one relation: a viewer holds can_view and
// not can_view_sensitive.
type refuseRelation struct{ relation string }

func (c refuseRelation) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != c.relation, nil
}

// w21/093: REST shapes a group's reply with its values, which is a separate
// can_view_sensitive decision made from the route's closure; a viewer's refusal
// recorded "envgroups.RegisterREST.func8". And the Blueprint protected-placement
// check reads the groups through a closure of its own.
func TestEnvGroupClosurePathsRecordNamedVerbs(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))
	created := serveREST(svc, http.MethodPost, "/v1/env-groups", `{"name":"shared"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", created.Code, created.Body)
	}
	var group EnvGroupView
	if err := json.Unmarshal(created.Body.Bytes(), &group); err != nil || group.ID == "" {
		t.Fatalf("created group: %s (%v)", created.Body, err)
	}
	id := group.ID

	rows := &auditRows{}
	svc.Audit = rows
	svc.Authz = refuseRelation{relation: core.RelCanViewSensitive}

	if rec := serveREST(svc, http.MethodGet, "/v1/env-groups/"+id, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a viewer's GET = %d %s, want 403", rec.Code, rec.Body)
	}
	denied := ""
	for _, ev := range rows.events {
		if ev.Outcome == core.AuditDenied {
			denied = ev.Verb
		}
	}
	if denied != AuditVerbRevealEnvGroup {
		t.Fatalf("the refused reveal recorded %q (rows %+v), want %s", denied, rows.events, AuditVerbRevealEnvGroup)
	}

	rows.events = nil
	svc.Authz = refuseRelation{relation: core.RelCanView}
	if _, err := GroupPlacements(svc)(testCtx()); err == nil {
		t.Fatal("a refused placement read succeeded")
	}
	if len(rows.events) != 1 || rows.events[0].Verb != AuditVerbGroupPlacements {
		t.Fatalf("the refused placement read recorded %+v, want one %s", rows.events, AuditVerbGroupPlacements)
	}
}
