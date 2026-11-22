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
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A session's `gh` resolves through Devin's security-profile binding chain
// (w10/m561, ADR047 D2): the workspace's GitHub CLI setting, then for a
// session an automation started the automations default and that
// automation's own binding, then the session's own. The most specific binding
// wins, unless the workspace's off is mandatory: that is a floor nothing below
// turns back on. A child session (ADR102) is governed by its tree's chain,
// and its own binding can only turn `gh` off.

// GitHubCLILevel names the binding that turned a session's `gh` off. It
// crosses the internal mint hop in RefusalLevelHeader and reaches the driver
// as BEX_AGENT_GITHUB_CLI_LEVEL.
type GitHubCLILevel string

const (
	// GitHubCLIWorkspace is the agent environment's githubCliEnabled, and
	// what an older bex-api or an unknown level reads as.
	GitHubCLIWorkspace GitHubCLILevel = "workspace"
	// GitHubCLIAutomationsDefault is the agent environment's
	// automationsGithubCliEnabled, for sessions an automation started.
	GitHubCLIAutomationsDefault GitHubCLILevel = "automations-default"
	// GitHubCLIAutomation is the automation template's agentConfig.githubCli.
	GitHubCLIAutomation GitHubCLILevel = "automation"
	// GitHubCLISession is the session's own agentConfig.githubCli, or that
	// of a session above it in its tree.
	GitHubCLISession GitHubCLILevel = "session"
)

// ParseGitHubCLILevel reads a level from the wire: an empty or unknown value
// is the workspace level, so a component that does not know a newer level
// still refuses, with the workspace's words.
func ParseGitHubCLILevel(value string) GitHubCLILevel {
	switch level := GitHubCLILevel(value); level {
	case GitHubCLIAutomationsDefault, GitHubCLIAutomation, GitHubCLISession:
		return level
	}
	return GitHubCLIWorkspace
}

// GitHubCLIChain is every binding of one session's `gh`. A nil binding
// inherits.
type GitHubCLIChain struct {
	// WorkspaceOff is the workspace's GitHub CLI setting turned off, and
	// Mandatory makes that off a floor.
	WorkspaceOff bool
	Mandatory    bool
	// Automated is a session whose tree an automation started: the
	// automations default and the automation's own binding apply to it.
	Automated          bool
	AutomationsDefault *bool
	Automation         *bool
	// Sessions are the session bindings of the tree from its root down to
	// the session itself. The root's turns `gh` on or off; one below it only
	// turns it off, since a child cannot loosen its parent's chain.
	Sessions []*bool
}

// Resolve applies the precedence: the workspace's mandatory off first, then
// the most specific binding of workspace, automations default, automation and
// root session, then any off below the root. It returns the level that
// turned `gh` off, "" when the session gets `gh`.
func (c GitHubCLIChain) Resolve() GitHubCLILevel {
	if c.WorkspaceOff && c.Mandatory {
		return GitHubCLIWorkspace
	}
	on, level := !c.WorkspaceOff, GitHubCLIWorkspace
	bind := func(binding *bool, at GitHubCLILevel) {
		if binding != nil {
			on, level = *binding, at
		}
	}
	if c.Automated {
		bind(c.AutomationsDefault, GitHubCLIAutomationsDefault)
		bind(c.Automation, GitHubCLIAutomation)
	}
	for i, binding := range c.Sessions {
		if i == 0 || (binding != nil && !*binding) {
			bind(binding, GitHubCLISession)
		}
	}
	if on {
		return ""
	}
	return level
}

// ResolveGitHubCLI is the level that turned session's `gh` off under env,
// "" when it gets `gh`: the one resolver of both `gh` mints and every
// dispatch. A mandatory off needs nothing below it read.
func ResolveGitHubCLI(ctx context.Context, sessions SessionStore, automations AutomationStore, env store.AgentEnvironment, session store.AgentSession) (GitHubCLILevel, error) {
	if env.GitHubCLIDisabled && env.GitHubCLIMandatory {
		return GitHubCLIWorkspace, nil
	}
	chain, err := LoadGitHubCLIChain(ctx, sessions, automations, env, session)
	if err != nil {
		return "", err
	}
	return chain.Resolve(), nil
}

// AutomationStore reads the automation that started a session's tree, for
// its template's `gh` binding.
type AutomationStore interface {
	GetAgentAutomation(ctx context.Context, id string) (store.AgentAutomation, error)
}

// maxTreeWalk bounds the walk up a session's tree: ADR102 allows three
// levels below the root.
const maxTreeWalk = 3

// LoadGitHubCLIChain reads session's chain under its workspace's environment
// row env (the zero value for a workspace with none): the sessions above it
// up to its tree's root and, when an automation started that root, the
// automation's current template. Both are read live, so an automation edit
// applies from a running session's next `gh` call. A deleted automation binds
// nothing, and a deleted ancestor ends the walk, as ADR102 makes the child of
// a deleted parent a top-level session. automations nil reads every
// automation as binding nothing.
func LoadGitHubCLIChain(ctx context.Context, sessions SessionStore, automations AutomationStore, env store.AgentEnvironment, session store.AgentSession) (GitHubCLIChain, error) {
	chain := GitHubCLIChain{
		WorkspaceOff: env.GitHubCLIDisabled, Mandatory: env.GitHubCLIMandatory, AutomationsDefault: env.AutomationsGitHubCLI,
	}
	own, err := SessionGitHubCLI(session.AgentConfig)
	if err != nil {
		return GitHubCLIChain{}, err
	}
	chain.Sessions = []*bool{own}
	root := session
	for range maxTreeWalk {
		if root.ParentSessionID == "" || sessions == nil {
			break
		}
		parent, err := sessions.GetAgentSession(ctx, root.ParentSessionID)
		if errors.Is(err, store.ErrNotFound) {
			break
		}
		if err != nil {
			return GitHubCLIChain{}, err
		}
		if parent.WorkspaceID != session.WorkspaceID {
			break
		}
		binding, err := SessionGitHubCLI(parent.AgentConfig)
		if err != nil {
			return GitHubCLIChain{}, err
		}
		chain.Sessions = append([]*bool{binding}, chain.Sessions...)
		root = parent
	}
	if root.AutomationID == "" {
		return chain, nil
	}
	chain.Automated = true
	if automations == nil {
		return chain, nil
	}
	automation, err := automations.GetAgentAutomation(ctx, root.AutomationID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return chain, nil
	case err != nil:
		return GitHubCLIChain{}, err
	}
	if automation.WorkspaceID != session.WorkspaceID {
		return chain, nil
	}
	chain.Automation, err = AutomationGitHubCLI(automation.Template)
	return chain, err
}

// SessionGitHubCLI is the githubCli binding a session's stored agentConfig
// carries; nil inherits.
func SessionGitHubCLI(agentConfig json.RawMessage) (*bool, error) {
	if len(agentConfig) == 0 {
		return nil, nil
	}
	var config struct {
		GitHubCLI *bool `json:"githubCli"`
	}
	if err := json.Unmarshal(agentConfig, &config); err != nil {
		return nil, fmt.Errorf("agent session config: %w", err)
	}
	return config.GitHubCLI, nil
}

// AutomationGitHubCLI is the githubCli binding a stored automation template
// carries in its agentConfig; nil inherits.
func AutomationGitHubCLI(template json.RawMessage) (*bool, error) {
	if len(template) == 0 {
		return nil, nil
	}
	var tpl struct {
		AgentConfig struct {
			GitHubCLI *bool `json:"githubCli"`
		} `json:"agentConfig"`
	}
	if err := json.Unmarshal(template, &tpl); err != nil {
		return nil, fmt.Errorf("agent automation template: %w", err)
	}
	return tpl.AgentConfig.GitHubCLI, nil
}

// GitHubCLIOffError is ErrGitHubCLIOff naming the level that turned `gh` off.
type GitHubCLIOffError struct {
	Level GitHubCLILevel
}

func (e *GitHubCLIOffError) Error() string {
	return fmt.Sprintf("%v (the %s binding)", ErrGitHubCLIOff, e.Level)
}

func (e *GitHubCLIOffError) Unwrap() error { return ErrGitHubCLIOff }

// GitHubCLIOff is the `gh` route's refusal naming level.
func GitHubCLIOff(level GitHubCLILevel) error { return &GitHubCLIOffError{Level: level} }

// GitHubCLIOffLevel is the level err's `gh` refusal names: the workspace for
// an ErrGitHubCLIOff that names none. ok is false when err is not one.
func GitHubCLIOffLevel(err error) (level GitHubCLILevel, ok bool) {
	if !errors.Is(err, ErrGitHubCLIOff) {
		return "", false
	}
	var off *GitHubCLIOffError
	if errors.As(err, &off) {
		return off.Level, true
	}
	return GitHubCLIWorkspace, true
}
