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

package agentsessions

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m563 against the real store: the idle close of a finished turn depends
// on who started the session, an open editor still keeps every one, a
// completed automation turn's undelivered work keeps its disk at that close,
// and an Enterprise workspace's own timeout keeps a person's session longer.

// finishTurn settles the session's running turn as a clean success and
// returns its row as finalized: UpdatedAt is the turn's end.
func finishTurn(t *testing.T, c *Completer, pg *store.PGStore, lc *fakeLifecycle, id string) store.AgentSession {
	t.Helper()
	ctx := context.Background()
	row, err := pg.GetAgentSession(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	lc.status = succeededStatus(false)
	c.Now = time.Now
	c.finalize(ctx, row)
	if row, err = pg.GetAgentSession(ctx, id); err != nil || row.Phase != PhaseCompleted {
		t.Fatalf("finalized = %+v err=%v, want completed", row, err)
	}
	return row
}

// sandboxAt runs the reaper at at and reports the session's sandbox id.
func sandboxAt(t *testing.T, c *Completer, pg *store.PGStore, id string, at time.Time) string {
	t.Helper()
	c.Now = func() time.Time { return at }
	c.reapIdleSandboxes(context.Background())
	row, err := pg.GetAgentSession(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return row.SandboxID
}

func TestTheIdleCloseDependsOnWhoStartedTheSessionPG(t *testing.T) {
	ctx := context.Background()
	e := newLineageEnv(t)
	// started is a running session of the named origin.
	started := func(t *testing.T, origin string) string {
		t.Helper()
		switch origin {
		case "child":
			return e.create(t, e.create(t, "").ID).ID
		case "automation":
			id := e.create(t, "").ID
			if _, err := e.pg.Pool.Exec(ctx, `UPDATE agent_sessions SET automation_id = 'agt-idle-close' WHERE id = $1`, id); err != nil {
				t.Fatal(err)
			}
			return id
		}
		return e.create(t, "").ID
	}
	completer := func() *Completer {
		return &Completer{Store: e.pg, Sandbox: e.lc, IdleTTL: DefaultIdleTimeout}
	}

	t.Run("person: kept at 4m59s, closed at 5m01s", func(t *testing.T) {
		c, id := completer(), started(t, "person")
		ended := finishTurn(t, c, e.pg, e.lc, id).UpdatedAt
		if sandboxAt(t, c, e.pg, id, ended.Add(5*time.Minute-time.Second)) == "" {
			t.Fatal("closed at 4m59s")
		}
		if got := sandboxAt(t, c, e.pg, id, ended.Add(5*time.Minute+time.Second)); got != "" {
			t.Fatalf("sandbox %q at 5m01s, want closed", got)
		}
	})
	for _, origin := range []string{"automation", "child"} {
		t.Run(origin+": closed at the harvest", func(t *testing.T) {
			c, id := completer(), started(t, origin)
			if row := finishTurn(t, c, e.pg, e.lc, id); row.SandboxID != "" {
				t.Fatalf("sandbox %q after the harvest, want closed", row.SandboxID)
			}
		})
	}
	for _, origin := range []string{"person", "automation", "child"} {
		t.Run(origin+": kept by an open editor", func(t *testing.T) {
			c, id := completer(), started(t, origin)
			if err := e.pg.StartSSHSession(ctx, store.SSHSessionAudit{
				ID: fmt.Sprintf("ssh-idle-%s-%d", origin, time.Now().UnixNano()), Subject: "user:alice",
				WorkspaceID: e.ws, ServiceID: id, InstanceID: "sandbox", StartedAt: time.Now(),
			}); err != nil {
				t.Fatal(err)
			}
			ended := finishTurn(t, c, e.pg, e.lc, id)
			if ended.SandboxID == "" || sandboxAt(t, c, e.pg, id, ended.UpdatedAt.Add(time.Hour)) == "" {
				t.Fatal("closed under an open editor")
			}
		})
	}
}

// An automation's completed turn that leaves an uncommitted file keeps its
// disk at the harvest close, as a person's does at the idle close (w10/m491),
// and the next message continues from it.
func TestAnAutomationsUndeliveredWorkIsKeptAtTheHarvestClose(t *testing.T) {
	e := newRetainEnv(t)
	e.c.IdleTTL = time.Hour // a person's session would still be inside it
	if _, err := e.pg.Pool.Exec(context.Background(), `UPDATE agent_sessions SET automation_id = 'agt-idle-close' WHERE id = $1`, e.id); err != nil {
		t.Fatal(err)
	}
	e.finished(t, gitStatus(undeliveredGit))
	img := e.live(t)
	turn := e.turn(t, 1)
	if img.State != store.RetainedImageKept || turn.Retention != store.TurnRetentionKept || turn.RetentionCause != store.TurnRetentionCauseIdleClosed {
		t.Fatalf("image %+v turn %+v, want kept by the close at the harvest", img, turn)
	}
	if e.lc.canceled != 0 || e.session(t).SandboxID != "" {
		t.Fatalf("canceled %d sandbox %q, want the save to have ended it", e.lc.canceled, e.session(t).SandboxID)
	}
	e.send(t, "alice", "carry on")
	e.assertContinuedFrom(t, img, store.TurnRetentionCauseIdleClosed)
}

// An Enterprise workspace set to 30 minutes keeps a person's session for 30
// minutes; a Pro workspace cannot set one.
func TestAnEnterpriseIdleTimeoutKeepsAPersonsSessionPG(t *testing.T) {
	ctx := context.Background()
	e := newLineageEnv(t)
	if _, err := e.pg.UpdateTenantPlan(ctx, e.ws, store.PlanEnterprise); err != nil {
		t.Fatal(err)
	}
	admin := environmentService(e.ws, e.pg)
	admin.Base.Plans = e.pg
	if got, err := admin.UpdateEnvironment(caller("admin"), e.ws, EnvironmentInput{IdleTimeoutMinutes: ptr(30)}); err != nil || got.IdleTimeoutMinutes != 30 {
		t.Fatalf("set 30 minutes = %d err=%v", got.IdleTimeoutMinutes, err)
	}
	c := &Completer{Store: e.pg, Sandbox: e.lc, IdleTTL: DefaultIdleTimeout, IdleTimeouts: IdleTimeouts{Service: admin}}
	id := e.create(t, "").ID
	ended := finishTurn(t, c, e.pg, e.lc, id).UpdatedAt
	if sandboxAt(t, c, e.pg, id, ended.Add(29*time.Minute)) == "" {
		t.Fatal("closed at 29m of a 30m timeout")
	}
	if got := sandboxAt(t, c, e.pg, id, ended.Add(31*time.Minute)); got != "" {
		t.Fatalf("sandbox %q at 31m, want closed", got)
	}

	if _, err := e.pg.UpdateTenantPlan(ctx, e.ws, store.PlanPro); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.UpdateEnvironment(caller("admin"), e.ws, EnvironmentInput{IdleTimeoutMinutes: ptr(60)}); !isCode(err, "PLAN_FEATURE_REQUIRED") {
		t.Fatalf("a Pro workspace's write = %v, want PLAN_FEATURE_REQUIRED", err)
	}
	if got, ok, err := c.IdleTimeouts.WorkspaceIdleTimeout(ctx, e.ws); err != nil || ok {
		t.Fatalf("after the downgrade the Completer reads %v %t %v, want the platform's", got, ok, err)
	}
}
