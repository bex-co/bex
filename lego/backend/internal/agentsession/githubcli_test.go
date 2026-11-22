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

package agentsession

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func boolp(v bool) *bool { return &v }

var (
	on  = boolp(true)
	off = boolp(false)
)

// w10/m561: the binding chain's precedence, pinned pair by pair in both
// directions, with the mandatory floor and a child's chain.
func TestGitHubCLIChainResolve(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chain GitHubCLIChain
		on    bool
		level GitHubCLILevel
	}{
		{"nothing bound: on", GitHubCLIChain{}, true, ""},
		{"workspace off", GitHubCLIChain{WorkspaceOff: true}, false, GitHubCLIWorkspace},
		{"mandatory has no effect while the workspace is on", GitHubCLIChain{Mandatory: true}, true, ""},

		// workspace ↔ session
		{"a session turns gh off under a workspace on", GitHubCLIChain{Sessions: []*bool{off}}, false, GitHubCLISession},
		{"a session turns gh on under a workspace off", GitHubCLIChain{WorkspaceOff: true, Sessions: []*bool{on}}, true, ""},
		{"a session cannot turn gh on under a mandatory off", GitHubCLIChain{WorkspaceOff: true, Mandatory: true, Sessions: []*bool{on}}, false, GitHubCLIWorkspace},
		{"a session can turn gh off under a mandatory on", GitHubCLIChain{Mandatory: true, Sessions: []*bool{off}}, false, GitHubCLISession},

		// workspace ↔ automations default
		{"the automations default turns gh off for an automation's session", GitHubCLIChain{Automated: true, AutomationsDefault: off}, false, GitHubCLIAutomationsDefault},
		{"the automations default turns gh on for an automation's session", GitHubCLIChain{WorkspaceOff: true, Automated: true, AutomationsDefault: on}, true, ""},
		{"the automations default cannot turn gh on under a mandatory off", GitHubCLIChain{WorkspaceOff: true, Mandatory: true, Automated: true, AutomationsDefault: on}, false, GitHubCLIWorkspace},
		{"the automations default binds no session a person started", GitHubCLIChain{AutomationsDefault: off}, true, ""},
		{"the automations default's on binds no session a person started", GitHubCLIChain{WorkspaceOff: true, AutomationsDefault: on}, false, GitHubCLIWorkspace},

		// automations default ↔ automation
		{"an automation turns gh off under an automations default on", GitHubCLIChain{Automated: true, AutomationsDefault: on, Automation: off}, false, GitHubCLIAutomation},
		{"an automation turns gh on under an automations default off", GitHubCLIChain{Automated: true, AutomationsDefault: off, Automation: on}, true, ""},
		{"an automation inherits the automations default", GitHubCLIChain{Automated: true, AutomationsDefault: off, Automation: nil}, false, GitHubCLIAutomationsDefault},

		// workspace ↔ automation
		{"an automation turns gh off under a workspace on", GitHubCLIChain{Automated: true, Automation: off}, false, GitHubCLIAutomation},
		{"an automation turns gh on under a workspace off", GitHubCLIChain{WorkspaceOff: true, Automated: true, Automation: on}, true, ""},
		{"an automation cannot turn gh on under a mandatory off", GitHubCLIChain{WorkspaceOff: true, Mandatory: true, Automated: true, Automation: on}, false, GitHubCLIWorkspace},

		// automation ↔ session
		{"a session turns gh off under its automation's on", GitHubCLIChain{Automated: true, Automation: on, Sessions: []*bool{off}}, false, GitHubCLISession},
		{"a session turns gh on under its automation's off", GitHubCLIChain{Automated: true, Automation: off, Sessions: []*bool{on}}, true, ""},
		{"a session inherits its automation", GitHubCLIChain{Automated: true, Automation: off, Sessions: []*bool{nil}}, false, GitHubCLIAutomation},

		// automations default ↔ session
		{"a session turns gh off under the automations default on", GitHubCLIChain{Automated: true, AutomationsDefault: on, Sessions: []*bool{off}}, false, GitHubCLISession},
		{"a session turns gh on under the automations default off", GitHubCLIChain{Automated: true, AutomationsDefault: off, Sessions: []*bool{on}}, true, ""},

		// child sessions: the tree's chain, and a child only turns gh off
		{"a child inherits its root's off", GitHubCLIChain{Sessions: []*bool{off, nil}}, false, GitHubCLISession},
		{"a child cannot turn gh on under its root's off", GitHubCLIChain{Sessions: []*bool{off, on}}, false, GitHubCLISession},
		{"a child cannot turn gh on under its workspace's off", GitHubCLIChain{WorkspaceOff: true, Sessions: []*bool{nil, on}}, false, GitHubCLIWorkspace},
		{"a child turns gh off under its root's on", GitHubCLIChain{WorkspaceOff: true, Sessions: []*bool{on, off}}, false, GitHubCLISession},
		{"a child inherits its root's on", GitHubCLIChain{WorkspaceOff: true, Sessions: []*bool{on, nil}}, true, ""},
		{"a grandchild inherits its parent's off", GitHubCLIChain{Sessions: []*bool{nil, off, nil}}, false, GitHubCLISession},
		{"a child of an automation's session inherits the automation's off", GitHubCLIChain{Automated: true, Automation: off, Sessions: []*bool{nil, on}}, false, GitHubCLIAutomation},
	} {
		got := tc.chain.Resolve()
		if (got == "") != tc.on || got != tc.level {
			t.Errorf("%s: Resolve = %q, want on=%t level=%q", tc.name, got, tc.on, tc.level)
		}
	}
}

func TestParseGitHubCLILevel(t *testing.T) {
	for in, want := range map[string]GitHubCLILevel{
		"": GitHubCLIWorkspace, "workspace": GitHubCLIWorkspace, "enterprise": GitHubCLIWorkspace,
		"automations-default": GitHubCLIAutomationsDefault, "automation": GitHubCLIAutomation, "session": GitHubCLISession,
	} {
		if got := ParseGitHubCLILevel(in); got != want {
			t.Errorf("ParseGitHubCLILevel(%q) = %q, want %q", in, got, want)
		}
	}
}

// treeSessions is a SessionStore holding several sessions by id.
type treeSessions map[string]store.AgentSession

func (f treeSessions) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	if s, ok := f[id]; ok {
		return s, nil
	}
	return store.AgentSession{}, store.ErrNotFound
}

type fakeAutomations struct {
	rows map[string]store.AgentAutomation
	err  error
}

func (f fakeAutomations) GetAgentAutomation(_ context.Context, id string) (store.AgentAutomation, error) {
	if f.err != nil {
		return store.AgentAutomation{}, f.err
	}
	if a, ok := f.rows[id]; ok {
		return a, nil
	}
	return store.AgentAutomation{}, store.ErrNotFound
}

func sessionConfig(binding *bool) json.RawMessage {
	config := map[string]any{"agent": "claude", "task": "t"}
	if binding != nil {
		config["githubCli"] = *binding
	}
	out, _ := json.Marshal(config)
	return out
}

func automationTemplate(binding *bool) json.RawMessage {
	config := map[string]any{"agent": "claude"}
	if binding != nil {
		config["githubCli"] = *binding
	}
	out, _ := json.Marshal(map[string]any{"task": "t", "agentConfig": config})
	return out
}

// LoadGitHubCLIChain walks a session's tree to its root, reads the root's
// automation, and treats a deleted automation or ancestor as binding nothing.
func TestLoadGitHubCLIChainWalksTheTreeAndTheAutomation(t *testing.T) {
	ctx := context.Background()
	env := store.AgentEnvironment{WorkspaceID: "tea-a", GitHubCLIDisabled: true, GitHubCLIMandatory: true, AutomationsGitHubCLI: on}
	sessions := treeSessions{
		"ags-root":   {ID: "ags-root", WorkspaceID: "tea-a", AgentConfig: sessionConfig(nil), AutomationID: "aut-1"},
		"ags-child":  {ID: "ags-child", WorkspaceID: "tea-a", AgentConfig: sessionConfig(off), ParentSessionID: "ags-root"},
		"ags-other":  {ID: "ags-other", WorkspaceID: "tea-b", AgentConfig: sessionConfig(off)},
		"ags-orphan": {ID: "ags-orphan", WorkspaceID: "tea-a", AgentConfig: sessionConfig(nil), ParentSessionID: "ags-gone"},
	}
	automations := fakeAutomations{rows: map[string]store.AgentAutomation{
		"aut-1": {ID: "aut-1", WorkspaceID: "tea-a", Template: automationTemplate(off)},
	}}
	grandchild := store.AgentSession{ID: "ags-grand", WorkspaceID: "tea-a", AgentConfig: sessionConfig(on), ParentSessionID: "ags-child"}
	chain, err := LoadGitHubCLIChain(ctx, sessions, automations, env, grandchild)
	if err != nil {
		t.Fatal(err)
	}
	if !chain.WorkspaceOff || !chain.Mandatory || !chain.Automated || chain.AutomationsDefault != on || chain.Automation == nil || *chain.Automation ||
		len(chain.Sessions) != 3 || chain.Sessions[0] != nil || chain.Sessions[1] == nil || *chain.Sessions[1] || chain.Sessions[2] == nil || !*chain.Sessions[2] {
		t.Fatalf("grandchild chain = %+v", chain)
	}

	// A deleted automation binds nothing, but its session is still an
	// automation's, so the automations default applies.
	chain, err = LoadGitHubCLIChain(ctx, sessions, fakeAutomations{}, store.AgentEnvironment{AutomationsGitHubCLI: off}, sessions["ags-root"])
	if err != nil || !chain.Automated || chain.Automation != nil {
		t.Fatalf("deleted automation chain = %+v err=%v", chain, err)
	}
	if got := chain.Resolve(); got != GitHubCLIAutomationsDefault {
		t.Fatalf("deleted automation resolves %q, want the automations default's off", got)
	}
	// No automation store reads every automation as binding nothing.
	if chain, err = LoadGitHubCLIChain(ctx, sessions, nil, store.AgentEnvironment{}, sessions["ags-root"]); err != nil || !chain.Automated || chain.Automation != nil {
		t.Fatalf("no automation store chain = %+v err=%v", chain, err)
	}
	// An automation of another workspace never binds this one's session.
	foreign := fakeAutomations{rows: map[string]store.AgentAutomation{"aut-1": {ID: "aut-1", WorkspaceID: "tea-b", Template: automationTemplate(off)}}}
	if chain, err = LoadGitHubCLIChain(ctx, sessions, foreign, store.AgentEnvironment{}, sessions["ags-root"]); err != nil || chain.Automation != nil {
		t.Fatalf("foreign automation chain = %+v err=%v", chain, err)
	}

	// A deleted parent ends the walk: the child is top-level from then on.
	if chain, err = LoadGitHubCLIChain(ctx, sessions, automations, store.AgentEnvironment{}, sessions["ags-orphan"]); err != nil || len(chain.Sessions) != 1 || chain.Automated {
		t.Fatalf("orphan chain = %+v err=%v", chain, err)
	}
	// A parent of another workspace is not walked.
	stray := store.AgentSession{ID: "ags-stray", WorkspaceID: "tea-a", ParentSessionID: "ags-other"}
	if chain, err = LoadGitHubCLIChain(ctx, sessions, automations, store.AgentEnvironment{}, stray); err != nil || len(chain.Sessions) != 1 {
		t.Fatalf("stray chain = %+v err=%v", chain, err)
	}

	// A read failure, or a stored config it cannot read, fails closed.
	if _, err := LoadGitHubCLIChain(ctx, sessions, fakeAutomations{err: errors.New("connection reset")}, env, sessions["ags-root"]); err == nil {
		t.Fatal("an unreadable automation resolved")
	}
	if _, err := LoadGitHubCLIChain(ctx, sessions, automations, env, store.AgentSession{WorkspaceID: "tea-a", AgentConfig: json.RawMessage(`{"githubCli":"yes"}`)}); err == nil {
		t.Fatal("an unreadable session config resolved")
	}
}

// The mints refuse a session whose own, automation or automations-default
// binding has gh off while its workspace has gh on, naming the level, and the
// level crosses the internal hop to the gateway's client.
func TestMintGitHubAPICredentialsFollowTheBindingChain(t *testing.T) {
	connections := fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "octo"}}
	ownOff := activeSession()
	ownOff.AgentConfig = sessionConfig(off)
	automated := activeSession()
	automated.AutomationID = "aut-1"
	for _, tc := range []struct {
		name        string
		session     store.AgentSession
		env         EnvironmentStore
		automations AutomationStore
		level       GitHubCLILevel
	}{
		{"the session's own off", ownOff, noEnvironment, nil, GitHubCLISession},
		{"the automation's off", automated, noEnvironment, fakeAutomations{rows: map[string]store.AgentAutomation{"aut-1": {ID: "aut-1", WorkspaceID: "tea-a", Template: automationTemplate(off)}}}, GitHubCLIAutomation},
		{"the automations default's off", automated, fakeEnvironments{row: store.AgentEnvironment{WorkspaceID: "tea-a", AutomationsGitHubCLI: off}}, fakeAutomations{}, GitHubCLIAutomationsDefault},
		{"the workspace's mandatory off over the automation's on", automated, fakeEnvironments{row: store.AgentEnvironment{WorkspaceID: "tea-a", GitHubCLIDisabled: true, GitHubCLIMandatory: true}}, fakeAutomations{rows: map[string]store.AgentAutomation{"aut-1": {ID: "aut-1", WorkspaceID: "tea-a", Template: automationTemplate(on)}}}, GitHubCLIWorkspace},
	} {
		for _, kind := range githubAPIMints {
			gh := &fakeSessionGitHub{}
			m := &Minter{GitHub: gh, Connections: connections, Sessions: fakeSessions{session: tc.session}, Environments: tc.env, Automations: tc.automations}
			_, err := kind.mint(m, context.Background(), validMintRequest())
			level, ok := GitHubCLIOffLevel(err)
			if !ok || level != tc.level || !errors.Is(err, ErrForbidden) || kind.minted(gh) != 0 {
				t.Fatalf("%s, %s: err=%v level=%q fake=%+v, want ErrGitHubCLIOff at %q and no token", tc.name, kind.name, err, level, gh, tc.level)
			}
			if got, err := m.Mint(context.Background(), validMintRequest()); err != nil || got.Token != "ghs_scoped" {
				t.Fatalf("%s: the Git mint = %+v %v, want the push token", tc.name, got, err)
			}

			server := httptest.NewServer(&GitHubReadHandler{Secret: []byte("secret"), Minter: m})
			_, err = (&Client{URL: server.URL, Secret: []byte("secret"), HTTP: server.Client()}).Mint(context.Background(), validMintRequest())
			server.Close()
			if level, ok := GitHubCLIOffLevel(err); !ok || level != tc.level {
				t.Fatalf("%s across the hop: err=%v level=%q, want %q", tc.name, err, level, tc.level)
			}
		}
	}

	// A session's own on wins over its workspace's off that is not mandatory.
	ownOn := activeSession()
	ownOn.AgentConfig = sessionConfig(on)
	gh := &fakeSessionGitHub{}
	m := &Minter{GitHub: gh, Connections: connections, Sessions: fakeSessions{session: ownOn}, Environments: fakeEnvironments{row: store.AgentEnvironment{WorkspaceID: "tea-a", GitHubCLIDisabled: true}}}
	if got, err := m.MintGitHubRead(context.Background(), validMintRequest()); err != nil || got.Token != "ghs_api_read" {
		t.Fatalf("a session's on under a workspace off = %+v %v, want the token", got, err)
	}
	// An automation that cannot be read refuses instead of guessing.
	m = &Minter{GitHub: gh, Connections: connections, Sessions: fakeSessions{session: automated}, Environments: noEnvironment, Automations: fakeAutomations{err: errors.New("connection reset")}}
	if _, err := m.MintGitHubRead(context.Background(), validMintRequest()); err == nil || errors.Is(err, ErrForbidden) {
		t.Fatalf("an unreadable automation: err=%v, want an unavailable error", err)
	}
}

// The level header degrades both ways: a gateway's client reads an absent
// level (an older bex-api) or an unknown one (a newer bex-api) as the
// workspace's, so the call is refused either way; a bare github-cli-off
// error from bex-api answers the workspace level.
func TestGitHubCLIOffLevelCrossesTheHopAndDegrades(t *testing.T) {
	for header, want := range map[string]GitHubCLILevel{
		"": GitHubCLIWorkspace, "enterprise": GitHubCLIWorkspace, "session": GitHubCLISession, "automation": GitHubCLIAutomation, "automations-default": GitHubCLIAutomationsDefault,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(RefusalHeader, RefusalGitHubCLIOff)
			if header != "" {
				w.Header().Set(RefusalLevelHeader, header)
			}
			w.WriteHeader(http.StatusForbidden)
		}))
		_, err := (&Client{URL: server.URL, Secret: []byte("secret"), HTTP: server.Client()}).Mint(context.Background(), validMintRequest())
		server.Close()
		level, ok := GitHubCLIOffLevel(err)
		if !ok || level != want || !errors.Is(err, ErrForbidden) {
			t.Fatalf("level header %q: err=%v level=%q, want ErrGitHubCLIOff at %q", header, err, level, want)
		}
	}
	if level, ok := GitHubCLIOffLevel(ErrGitHubCLIOff); !ok || level != GitHubCLIWorkspace {
		t.Fatalf("a bare ErrGitHubCLIOff = %q %t, want the workspace level", level, ok)
	}
	if _, ok := GitHubCLIOffLevel(ErrForbidden); ok {
		t.Fatal("a plain ErrForbidden read as gh off")
	}
}
