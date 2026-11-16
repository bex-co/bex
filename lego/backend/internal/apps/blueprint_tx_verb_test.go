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

package apps

import (
	"context"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// passThroughGroupingTx runs the grouping apply's callback as the pool-backed
// store does, without a database: the callback is a closure, one frame above the
// authorization inside it.
type passThroughGroupingTx struct{ groups store.GroupingStore }

func (p passThroughGroupingTx) RunGroupingTx(_ context.Context, fn func(store.GroupingStore) error) error {
	return fn(p.groups)
}

// w21/093: production applies the groupings in a transaction, so the can_manage
// check a Blueprint apply makes before it changes an existing environment's
// protection ran inside the callback's closure and recorded
// "apps.applyBlueprintGroupings.func1". It records the verb of the apply that
// asked, as the apply without a transaction does.
func TestBlueprintGroupingTransactionRecordsTheApplyVerb(t *testing.T) {
	const manifest = `version: "1"
projects:
  - name: platform
    environments:
      - name: production
        permissions:
          protection: disabled
        services:
          - type: web
            name: web
            runtime: image
            image:
              url: nginx:alpine
`
	for _, transactional := range []bool{false, true} {
		name := "without a transaction"
		if transactional {
			name = "in a transaction"
		}
		t.Run(name, func(t *testing.T) {
			st, _ := seededProtectedEnvironmentStore()
			svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
			svc.BlueprintGroups, svc.Environments = st, st
			if transactional {
				svc.BlueprintGroupsTx = passThroughGroupingTx{groups: st}
			}
			sink := &captureAuditSink{}
			svc.Audit = sink
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})

			if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
				t.Fatal(err)
			}
			if st.aclWrites != 1 {
				t.Fatalf("the apply wrote the environment's controls %d times, want once", st.aclWrites)
			}

			// A change to an existing environment's protection is admin-classified:
			// the apply's can_manage check is a write the log records.
			var manage []string
			for _, ev := range sink.events {
				if strings.Contains(ev.Verb, "func") {
					t.Errorf("recorded %q, a closure's name", ev.Verb)
				}
				if ev.Relation == core.RelCanManage {
					manage = append(manage, string(ev.Outcome)+" "+ev.Verb)
				}
			}
			if len(manage) != 1 || manage[0] != "allowed apps.DeployStack" {
				t.Fatalf("the can_manage check recorded %v, want [allowed apps.DeployStack]", manage)
			}
		})
	}
}
