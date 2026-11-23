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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m563: a finished turn's sandbox closes 5 minutes after the turn of a
// session a person started, as a Devin session sleeps after 5 minutes of
// inactivity, and at the harvest for an automation's or a child session. An
// Enterprise workspace sets its own timeout, 5 to 120 minutes.

// origins are the three ways a session is started, as the row records them.
var origins = map[string]func(*store.AgentSession){
	"person":     func(*store.AgentSession) {},
	"automation": func(r *store.AgentSession) { r.AutomationID = "agt-auto-1" },
	"child":      func(r *store.AgentSession) { r.ParentSessionID = "ags-parent-1" },
}

// finishedFixture is a session of the given origin whose turn the Completer
// finalized at the tick it returns, with the default 5-minute grace.
func finishedFixture(t *testing.T, origin func(*store.AgentSession)) (*Completer, *fakeStore, *fakeLifecycle, string, time.Time) {
	t.Helper()
	c, st, lc, id := completerFixture(succeededStatus(true), nil)
	c.IdleTTL = DefaultIdleTimeout
	row := st.rows[id]
	origin(&row)
	st.rows[id] = row
	start := st.now
	c.Now = func() time.Time { return start }
	c.Reconcile(context.Background())
	if st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("phase = %s, want the turn finalized", st.rows[id].Phase)
	}
	return c, st, lc, id, st.rows[id].UpdatedAt
}

// A person's session is kept until 5 minutes after its turn ended; an
// automation's and a child's are closed at the very tick that harvested
// them. An open editor keeps every one of them.
func TestTheIdleCloseDependsOnWhoStartedTheSession(t *testing.T) {
	t.Run("person", func(t *testing.T) {
		c, st, lc, id, ended := finishedFixture(t, origins["person"])
		c.Now = func() time.Time { return ended.Add(5*time.Minute - time.Second) }
		c.Reconcile(context.Background())
		if lc.canceled != 0 || st.rows[id].SandboxID == "" {
			t.Fatalf("closed at 4m59s (canceled=%d)", lc.canceled)
		}
		c.Now = func() time.Time { return ended.Add(5*time.Minute + time.Second) }
		c.Reconcile(context.Background())
		if lc.canceled != 1 || st.rows[id].SandboxID != "" {
			t.Fatalf("not closed at 5m01s (canceled=%d sandbox=%q)", lc.canceled, st.rows[id].SandboxID)
		}
	})
	for _, name := range []string{"automation", "child"} {
		t.Run(name, func(t *testing.T) {
			_, st, lc, id, _ := finishedFixture(t, origins[name])
			if lc.canceled != 1 || st.rows[id].SandboxID != "" {
				t.Fatalf("not closed at the harvest (canceled=%d sandbox=%q)", lc.canceled, st.rows[id].SandboxID)
			}
			if lc.readTranscript == 0 || !lc.readTranscriptBeforeCancel {
				t.Fatalf("closed without harvesting first (reads=%d)", lc.readTranscript)
			}
		})
	}
	for name, origin := range origins {
		t.Run(name+" with an editor open", func(t *testing.T) {
			c, st, lc, id := completerFixture(succeededStatus(true), nil)
			c.IdleTTL = DefaultIdleTimeout
			row := st.rows[id]
			origin(&row)
			st.rows[id] = row
			st.openSSH[id] = st.now
			c.Now = func() time.Time { return st.now.Add(time.Hour) }
			c.Reconcile(context.Background())
			if lc.canceled != 0 || st.rows[id].SandboxID == "" {
				t.Fatalf("closed under an open editor (canceled=%d)", lc.canceled)
			}
		})
	}
}

// fakeIdleTimeouts is a workspace timeout source that counts its reads.
type fakeIdleTimeouts struct {
	timeout time.Duration
	set     bool
	err     error
	reads   int
}

func (f *fakeIdleTimeouts) WorkspaceIdleTimeout(context.Context, string) (time.Duration, bool, error) {
	f.reads++
	return f.timeout, f.set, f.err
}

// A workspace's own timeout replaces the 5 minutes for a person's session;
// it is read only once the turn has been idle for the 5 minutes no timeout
// goes below, and a failed read keeps the sandbox for the next tick.
func TestAWorkspaceTimeoutKeepsAPersonsSession(t *testing.T) {
	c, st, lc, id, ended := finishedFixture(t, origins["person"])
	timeouts := &fakeIdleTimeouts{timeout: 30 * time.Minute, set: true}
	c.IdleTimeouts = timeouts
	c.Now = func() time.Time { return ended.Add(4 * time.Minute) }
	c.Reconcile(context.Background())
	if timeouts.reads != 0 || lc.canceled != 0 {
		t.Fatalf("reads %d canceled %d inside the 5 minutes, want neither", timeouts.reads, lc.canceled)
	}
	c.Now = func() time.Time { return ended.Add(29 * time.Minute) }
	c.Reconcile(context.Background())
	if timeouts.reads != 1 || lc.canceled != 0 {
		t.Fatalf("reads %d canceled %d at 29m, want the 30m timeout read and kept", timeouts.reads, lc.canceled)
	}
	timeouts.err = errors.New("store down")
	c.Now = func() time.Time { return ended.Add(31 * time.Minute) }
	c.Reconcile(context.Background())
	if lc.canceled != 0 || st.rows[id].SandboxID == "" {
		t.Fatalf("closed on a failed read (canceled=%d)", lc.canceled)
	}
	timeouts.err = nil
	c.Reconcile(context.Background())
	if lc.canceled != 1 {
		t.Fatalf("not closed at 31m of a 30m timeout (canceled=%d)", lc.canceled)
	}

	// None set: the 5 minutes; an automation never asks.
	c, _, lc, _, ended = finishedFixture(t, origins["person"])
	c.IdleTimeouts = &fakeIdleTimeouts{}
	c.Now = func() time.Time { return ended.Add(5*time.Minute + time.Second) }
	c.Reconcile(context.Background())
	if lc.canceled != 1 {
		t.Fatalf("a workspace with no timeout kept the sandbox past 5m (canceled=%d)", lc.canceled)
	}
	auto := &fakeIdleTimeouts{timeout: time.Hour, set: true}
	c, st, lc, id = completerFixture(succeededStatus(true), nil)
	c.IdleTTL, c.IdleTimeouts = DefaultIdleTimeout, auto
	row := st.rows[id]
	row.AutomationID = "agt-auto-1"
	st.rows[id] = row
	c.Reconcile(context.Background())
	if auto.reads != 0 || lc.canceled != 1 {
		t.Fatalf("an automation's session read %d timeouts, canceled %d, want none read and closed", auto.reads, lc.canceled)
	}
}

// planService is an environment service whose workspace is on plan, with the
// Completer's 5-minute default.
func planService(plan string) (*Service, *fakeEnvironments) {
	envs := newFakeEnvironments()
	svc := environmentService("tea-a", envs)
	svc.Base.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return plan, nil })
	svc.Completer = &Completer{IdleTTL: DefaultIdleTimeout}
	return svc, envs
}

// The Enterprise setting on every surface: REST, GraphQL and MCP read and
// write the same idleTimeoutMinutes, and the Completer's read sees it.
func TestAnEnterpriseWorkspaceSetsItsIdleTimeoutOnEverySurface(t *testing.T) {
	svc, envs := planService(store.PlanEnterprise)
	read, err := svc.Environment(caller("viewer"), "tea-a")
	if err != nil || read.IdleTimeoutMinutes != 5 || !read.IdleTimeoutConfigurable {
		t.Fatalf("default = %d configurable %t err=%v, want 5 and settable", read.IdleTimeoutMinutes, read.IdleTimeoutConfigurable, err)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(`{"idleTimeoutMinutes":30}`)).WithContext(caller("admin")))
	var view Environment
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &view) != nil || view.IdleTimeoutMinutes != 30 {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
	}
	if got, ok, err := (IdleTimeouts{Service: svc}).WorkspaceIdleTimeout(context.Background(), "tea-a"); err != nil || !ok || got != 30*time.Minute {
		t.Fatalf("the Completer reads %v %t %v, want 30m", got, ok, err)
	}

	result := graphql.Do(graphql.Params{Schema: environmentSchema(t, svc), Context: caller("admin"),
		RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {idleTimeoutMinutes: 120}) { idleTimeoutMinutes idleTimeoutConfigurable } }`})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL: %v", result.Errors)
	}
	if got := result.Data.(map[string]any)["updateAgentEnvironment"].(map[string]any); got["idleTimeoutMinutes"] != 120 || got["idleTimeoutConfigurable"] != true {
		t.Fatalf("GraphQL = %v", got)
	}

	client := newMCPClient(t, caller("admin"), svc)
	res, err := client.CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "idleTimeoutMinutes": 5}})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"idleTimeoutMinutes":5`) {
		t.Fatalf("MCP = %+v err=%v", res, err)
	}
	if got := *envs.rows["tea-a"].IdleTimeoutMinutes; got != 5 {
		t.Fatalf("stored %d, want the last write", got)
	}

	// An update of another field keeps it.
	if got, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{NetworkTier: ptr(NetworkTierFull)}); err != nil || got.IdleTimeoutMinutes != 5 {
		t.Fatalf("a tier update = %d err=%v, want the timeout kept", got.IdleTimeoutMinutes, err)
	}
	// A member who is not an admin cannot set it.
	if _, err := svc.UpdateEnvironment(caller("dev"), "tea-a", EnvironmentInput{IdleTimeoutMinutes: ptr(60)}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a developer's write = %v, want forbidden", err)
	}
}

// Below Enterprise the write is refused on every surface with the plan
// error and writes nothing; a timeout stored before a downgrade stops
// applying, and the reads say 5 minutes.
func TestTheIdleTimeoutIsEnterpriseOnly(t *testing.T) {
	for _, plan := range []string{store.PlanHobby, store.PlanPro, store.PlanScale} {
		t.Run(plan, func(t *testing.T) {
			svc, envs := planService(plan)
			_, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{IdleTimeoutMinutes: ptr(30)})
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode || coded.Params["feature"] != "agentIdleTimeout" ||
				coded.Params["requiredPlan"] != store.PlanEnterprise || coded.Params["field"] != idleTimeoutField {
				t.Fatalf("service = %#v, want %s naming the Enterprise plan", err, core.PlanFeatureRequiredCode)
			}
			if _, written := envs.rows["tea-a"]; written {
				t.Fatal("a refused write was stored")
			}

			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(`{"idleTimeoutMinutes":30}`)).WithContext(caller("admin")))
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), core.PlanFeatureRequiredCode) {
				t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
			}
			result := graphql.Do(graphql.Params{Schema: environmentSchema(t, svc), Context: caller("admin"),
				RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {idleTimeoutMinutes: 30}) { idleTimeoutMinutes } }`})
			if len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != core.PlanFeatureRequiredCode {
				t.Fatalf("GraphQL = %#v", result.Errors)
			}
			res, err := newMCPClient(t, caller("admin"), svc).CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "idleTimeoutMinutes": 30}})
			if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, core.PlanFeatureRequiredCode) {
				t.Fatalf("MCP = %+v err=%v", res, err)
			}

			// Another field still saves on this plan.
			if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{NetworkTier: ptr(NetworkTierFull)}); err != nil {
				t.Fatalf("a tier update below Enterprise: %v", err)
			}
			// A downgraded workspace's stored timeout no longer applies.
			row := envs.rows["tea-a"]
			row.IdleTimeoutMinutes = ptr(90)
			envs.rows["tea-a"] = row
			read, err := svc.Environment(caller("viewer"), "tea-a")
			if err != nil || read.IdleTimeoutMinutes != 5 || read.IdleTimeoutConfigurable {
				t.Fatalf("read = %d configurable %t err=%v, want 5 and not settable", read.IdleTimeoutMinutes, read.IdleTimeoutConfigurable, err)
			}
			if got, ok, err := (IdleTimeouts{Service: svc}).WorkspaceIdleTimeout(context.Background(), "tea-a"); err != nil || ok {
				t.Fatalf("the Completer reads %v %t %v, want the platform's", got, ok, err)
			}
		})
	}
}

// A plan read that fails refuses the read and the write rather than guess.
func TestTheIdleTimeoutFailsClosedOnAPlanReadFailure(t *testing.T) {
	svc, _ := planService(store.PlanEnterprise)
	svc.Base.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "", errors.New("db down") })
	if _, err := svc.Environment(caller("viewer"), "tea-a"); err == nil {
		t.Fatal("the read succeeded without the plan")
	}
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{IdleTimeoutMinutes: ptr(30)}); err == nil {
		t.Fatal("the write succeeded without the plan")
	}
	if _, _, err := (IdleTimeouts{Service: svc}).WorkspaceIdleTimeout(context.Background(), "tea-a"); err != nil {
		t.Fatalf("a workspace with no row read the plan: %v", err)
	}
}
