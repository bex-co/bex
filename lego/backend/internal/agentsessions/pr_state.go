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
	"log"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A session's pull request state (Devin's pr_state) is read from GitHub by a
// reconciler on every replica, so it needs no webhook event the App may not be
// subscribed to; a pull_request delivery, when the App sends one, updates it
// at once (ADR094 D7).
const (
	prStateTick        = time.Minute
	prStateBatch       = 50
	prStateConcurrency = 4
	// prStateRetry is when a read GitHub refused (a deleted repository, a
	// removed installation, a rate limit) is tried again.
	prStateRetry = time.Hour
)

// nextPRCheck is when to read a pull request again, given its state and when
// that state last changed; nil means never. An open one is read every five
// minutes for a week, hourly for a month, then daily, so abandoned pull
// requests stop spending the installation's API budget; a closed one hourly
// for a week, so a reopen is seen; a merged one never.
func nextPRCheck(state string, changedAt, now time.Time) *time.Time {
	age := now.Sub(changedAt)
	var every time.Duration
	switch {
	case state == store.PRStateMerged:
		return nil
	case state == store.PRStateClosed && age >= 7*24*time.Hour:
		return nil
	case state == store.PRStateClosed:
		every = time.Hour
	case age < 7*24*time.Hour:
		every = 5 * time.Minute
	case age < 30*24*time.Hour:
		every = time.Hour
	default:
		every = 24 * time.Hour
	}
	next := now.Add(every)
	return &next
}

// PRStateStore is the durable seam (*store.PGStore).
type PRStateStore interface {
	ClaimAgentSessionPRChecks(ctx context.Context, now time.Time, retry time.Duration, limit int) ([]store.AgentSessionPRCheck, error)
	RecordAgentSessionPRState(ctx context.Context, sessionID, state string, at time.Time, next *time.Time) (bool, error)
	RecordPullRequestState(ctx context.Context, workspaceIDs []string, repo string, number int, state string, at time.Time, next *time.Time) (int, error)
	GetGitConnectionByOwner(ctx context.Context, workspaceID, accountLogin string) (store.GitConnection, error)
}

// PRStateGitHub reads one pull request (*github.Client).
type PRStateGitHub interface {
	GetPullRequest(ctx context.Context, installationID int64, owner, repo string, number int) (github.PullRequestDetail, error)
}

// PRStates keeps every session's pull request state current.
type PRStates struct {
	Store  PRStateStore
	GitHub PRStateGitHub
	Now    func() time.Time
}

func (p *PRStates) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// Run ticks until ctx ends.
func (p *PRStates) Run(ctx context.Context) {
	core.Poll(ctx, "agent-pr-states", prStateTick, p.RunOnce)
}

// prStateOf maps GitHub's state and merged flag onto Devin's three states.
func prStateOf(state string, merged bool) string {
	switch {
	case merged:
		return store.PRStateMerged
	case state == "closed":
		return store.PRStateClosed
	default:
		return store.PRStateOpen
	}
}

// prRead is one claimed pull request with what reading it needs.
type prRead struct {
	check          store.AgentSessionPRCheck
	installationID int64
	owner, name    string
}

// RunOnce claims the pull requests due a read and records what GitHub says,
// prStateConcurrency at a time. One that cannot be read keeps its last known
// state until the claim's retry. Exported so tests step it deterministically.
func (p *PRStates) RunOnce(ctx context.Context) error {
	now := p.now().UTC()
	checks, err := p.Store.ClaimAgentSessionPRChecks(ctx, now, prStateRetry, prStateBatch)
	if err != nil {
		return err
	}
	// Installations resolve once per workspace and owner, before the reads.
	installations := map[string]int64{}
	reads := make([]prRead, 0, len(checks))
	for _, check := range checks {
		repo, err := agentsession.NormalizeRepository(check.Repo)
		if err != nil {
			continue
		}
		owner, name, _ := strings.Cut(repo, "/")
		key := check.WorkspaceID + "/" + owner
		id, ok := installations[key]
		if !ok {
			connection, err := p.Store.GetGitConnectionByOwner(ctx, check.WorkspaceID, owner)
			if err != nil {
				log.Printf("agent-pr-states: session=%s %s#%d: no installation: %v", check.SessionID, repo, check.PRNumber, err)
			}
			id = connection.InstallationID
			installations[key] = id
		}
		if id != 0 {
			reads = append(reads, prRead{check: check, installationID: id, owner: owner, name: name})
		}
	}
	var g errgroup.Group
	g.SetLimit(prStateConcurrency)
	for _, read := range reads {
		g.Go(func() error {
			if err := p.read(ctx, read); err != nil {
				log.Printf("agent-pr-states: session=%s %s/%s#%d left as %q: %v", read.check.SessionID, read.owner, read.name, read.check.PRNumber, read.check.PRState, err)
			}
			return nil
		})
	}
	return g.Wait()
}

func (p *PRStates) read(ctx context.Context, read prRead) error {
	pr, err := p.GitHub.GetPullRequest(ctx, read.installationID, read.owner, read.name, read.check.PRNumber)
	if err != nil {
		return err
	}
	now := p.now().UTC()
	state := prStateOf(pr.State, pr.Merged)
	changedAt := now
	if state == read.check.PRState && read.check.PRStateAt != nil {
		changedAt = *read.check.PRStateAt
	}
	_, err = p.Store.RecordAgentSessionPRState(ctx, read.check.SessionID, state, now, nextPRCheck(state, changedAt, now))
	return err
}

// HandlePullRequestEvent applies one GitHub App pull_request delivery to the
// sessions in workspaceIDs (the workspaces its installation is bound to)
// whose pull request it names. The webhook forwards only the actions that can
// change a state.
func (p *PRStates) HandlePullRequestEvent(ctx context.Context, body []byte, workspaceIDs []string) error {
	var ev feedbackEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return err
	}
	if ev.PullRequest.Number <= 0 || len(workspaceIDs) == 0 {
		return nil
	}
	repo, err := agentsession.NormalizeRepository(ev.Repository.FullName)
	if err != nil {
		return nil
	}
	now := p.now().UTC()
	state := prStateOf(ev.PullRequest.State, ev.PullRequest.Merged)
	_, err = p.Store.RecordPullRequestState(ctx, workspaceIDs, repo, ev.PullRequest.Number, state, now, nextPRCheck(state, now, now))
	return err
}
