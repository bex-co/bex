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
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/oauthgrant"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestOperatorRevocationIsRefusedOnEveryReplicaAtTheNextRequest pins w21/090:
// an operator who cuts off one human's grant with the revoke-oauth-grant
// command (internal/oauthgrant) makes every bex-api replica refuse the token at
// its next request, with no wait for the introspection cache window.
//
// Two replicas, with the revocation marker as production has it (one shared
// store), each hold the token warm. The window is never crossed: the clock is
// frozen, so a refusal can only come from the marker. The control is a token
// revoked directly at Hydra, which no marker follows: both replicas still
// accept it, the gap the command closes.
func TestOperatorRevocationIsRefusedOnEveryReplicaAtTheNextRequest(t *testing.T) {
	const cli, otherCLI = "429024F5E608930E2A65EF92591A25CC", "bex-mobile"
	t.Run("in-memory marker", func(t *testing.T) {
		shared, trail := &memoryRevocations{}, &fakeAuditSink{}
		runOperatorRevocation(t, "human-op", "human-other", "human-direct", cli, otherCLI,
			[]RevocationStore{shared, shared}, shared, trail, "tea-ops",
			func(t *testing.T) []core.AuditEvent { return trail.events })
	})
	t.Run("postgres marker", func(t *testing.T) {
		ctx, pool, st := ownerGuardPG(t)
		run := uniqueRunSuffix()
		ws := "tea-op-" + run
		// One PGStore per replica, as in production, over one database.
		replicas := []RevocationStore{store.NewPGStore(pool), store.NewPGStore(pool)}
		runOperatorRevocation(t, "human-op-"+run, "human-other-"+run, "human-direct-"+run, "cli-"+run, "mobile-"+run,
			replicas, st, st, ws,
			func(t *testing.T) []core.AuditEvent {
				rows, err := st.ListAuditEvents(ctx, ws, store.AuditFilter{})
				if err != nil {
					t.Fatalf("list the audit trail: %v", err)
				}
				var evs []core.AuditEvent
				for _, row := range rows {
					evs = append(evs, core.AuditEvent{Caller: row.Caller, CallerMethod: row.CallerMethod, Verb: row.Verb,
						Resource: core.WorkspaceObject(row.WorkspaceID), Target: row.Target, TargetName: row.TargetName,
						Outcome: core.AuditOutcome(row.Outcome)})
				}
				return evs
			})
	})
}

func runOperatorRevocation(t *testing.T, human, other, direct, cli, otherCLI string,
	replicaMarkers []RevocationStore, operatorMarkers oauthgrant.Markers, audit core.AuditSink, workspace string,
	auditRows func(*testing.T) []core.AuditEvent) {
	t.Helper()
	hydra := newRevocableHydra(t)
	hydra.issue(cli, human, "access-target", "refresh-target")
	hydra.issue(cli, other, "access-same-client-other-human", "refresh-x")
	hydra.issue(otherCLI, human, "access-same-human-other-client", "refresh-y")
	hydra.issue(cli, direct, "access-revoked-at-hydra-only", "refresh-z")

	// Frozen well before every marker, so the cache window never ends in this
	// test and a refusal can only be the marker's work.
	clock := &stepClock{t: time.Unix(1_700_000_000, 0)}
	tokens := []string{"access-target", "access-same-client-other-human", "access-same-human-other-client", "access-revoked-at-hydra-only"}
	handlers := make([]http.Handler, len(replicaMarkers))
	for i, markers := range replicaMarkers {
		gate := newOryAuth(hydra.URL, "", "", "", "", false, nil, nil, nil, "")
		gate.revocations, gate.now, gate.cache.Now = markers, clock.Now, clock.Now
		handlers[i] = gate.middleware(echoIdentity)
		for _, token := range tokens {
			if got := do(t, handlers[i], http.MethodGet, "/probe", token, "").Code; got != http.StatusOK {
				t.Fatalf("replica %d warming %s: %d", i, token, got)
			}
		}
	}

	// The control: revoked at Hydra directly, as an operator holding only its
	// admin API would, with no marker behind it.
	hydra.revoke(t, http.MethodDelete, "/admin/oauth2/auth/sessions/consent?subject="+direct+"&client="+cli, nil)

	// The operator's command, applied.
	revoker := &oauthgrant.Revoker{HydraAdminURL: hydra.URL, Markers: operatorMarkers, Audit: audit, AuditWorkspace: workspace}
	res, err := revoker.Run(context.Background(), oauthgrant.Request{Subject: human, ClientID: cli, Apply: true})
	if err != nil {
		t.Fatalf("revoke-oauth-grant: %v (%+v)", err, res)
	}
	if len(res.Revoked) != 1 || res.Revoked[0].Hydra != "deleted" || !res.Revoked[0].Marker || !res.Revoked[0].Audited {
		t.Fatalf("result = %+v", res)
	}

	// The very next request, on every replica, with no wait.
	for i, handler := range handlers {
		want := map[string]int{
			"access-target":                  http.StatusUnauthorized,
			"access-same-client-other-human": http.StatusOK,
			"access-same-human-other-client": http.StatusOK,
			"access-revoked-at-hydra-only":   http.StatusOK, // the gap the command closes
		}
		for _, token := range tokens {
			if got := do(t, handler, http.MethodGet, "/probe", token, "").Code; got != want[token] {
				t.Errorf("replica %d, %s: %d, want %d", i, token, got, want[token])
			}
		}
	}
	// A replica with a cold cache refuses it too: Hydra no longer knows the grant.
	cold := newOryAuth(hydra.URL, "", "", "", "", false, nil, nil, nil, "")
	cold.revocations, cold.now, cold.cache.Now = replicaMarkers[0], clock.Now, clock.Now
	if got := do(t, cold.middleware(echoIdentity), http.MethodGet, "/probe", "access-target", "").Code; got != http.StatusUnauthorized {
		t.Errorf("cold replica: %d, want 401", got)
	}

	rows := auditRows(t)
	if len(rows) != 1 {
		t.Fatalf("audit rows = %+v, want exactly the revoked grant", rows)
	}
	ev := rows[0]
	if ev.Verb != oauthgrant.AuditVerb || ev.Caller != oauthgrant.AuditCaller || ev.CallerMethod != oauthgrant.AuditCallerMethod ||
		ev.Resource != core.WorkspaceObject(workspace) || ev.Target != "oauth-grant:"+cli || ev.TargetName != human ||
		ev.Outcome != core.AuditAllowed {
		t.Errorf("audit row = %+v", ev)
	}
}
