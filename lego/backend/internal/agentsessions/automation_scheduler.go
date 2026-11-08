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
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The automation scheduler (ADR101 D3): a loop on every bex-api replica that
// claims due automations and stale runs from Postgres and fires each one as a
// normal session create, as the automation's creator.
const (
	automationTick        = 15 * time.Second
	automationLease       = time.Minute
	automationBatch       = 20
	automationConcurrency = 4
	// automationFailureLimit consecutive failed_create runs pause the
	// automation (ADR101 D7).
	automationFailureLimit = 3
	maxRunErrorLen         = 500
)

// AutomationScheduler fires due agent automations. Run it on every replica:
// the claim is FOR UPDATE SKIP LOCKED, the schedule advances in the claiming
// transaction, the run row's key is (automation, fire_at) and the session
// create is idempotent, so replicas never share or double a fire.
type AutomationScheduler struct {
	Service *Service
	// Interval is the tick (default 15 s); Now the clock (default Service.Now).
	Interval time.Duration
	Now      func() time.Time
}

// AutomationsRunnable reports whether this deployment can fire automations: the
// automation table and everything a session create needs are wired.
func (s *Service) AutomationsRunnable() bool {
	return s != nil && s.Base != nil && s.Automations != nil && s.enabled() && s.modelProxyEnabled() && s.ticketEnabled()
}

// Run ticks until ctx ends.
func (w *AutomationScheduler) Run(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = automationTick
	}
	core.Poll(ctx, "agent-automations", interval, w.RunOnce)
}

func (w *AutomationScheduler) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return w.Service.Now()
}

// RunOnce is one tick: claim due automations, claim stale runs (a replica that
// died after claiming), and fire every run it now owns, automationConcurrency
// at a time. Exported so tests step the scheduler deterministically.
func (w *AutomationScheduler) RunOnce(ctx context.Context) error {
	st := w.Service.Automations
	now := w.now().UTC()
	claimed, claimErr := st.ClaimDueAutomations(ctx, now, automationBatch, automationLease, scheduleAdvance(now))
	for _, f := range claimed.Failures {
		w.Service.retireUnschedulable(ctx, f)
	}
	stale, staleErr := st.ClaimStaleRuns(ctx, now, automationLease, automationBatch)

	runs := append(claimed.Runs, stale...)
	sem := make(chan struct{}, automationConcurrency)
	var wg sync.WaitGroup
	for _, run := range runs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := w.Service.fireAutomationRun(ctx, run); err != nil {
				log.Printf("agent-automations: %s fire_at=%s left claimed for retry: %v",
					run.Run.AutomationID, run.Run.FireAt.UTC().Format(time.RFC3339Nano), err)
			}
		}()
	}
	wg.Wait()
	return errors.Join(claimErr, staleErr)
}

// errScheduleFinished is a due automation whose schedule has nothing left to run.
var errScheduleFinished = errors.New("the schedule is finished")

// scheduleAdvance is ClaimDueAutomations' advance function: it decodes the
// stored trigger and asks the schedule which tick to run now, how many were
// missed and when the next one is (ADR101 D3: only the latest missed tick fires).
func scheduleAdvance(now time.Time) func(a store.AgentAutomation) (time.Time, *time.Time, int, error) {
	return func(a store.AgentAutomation) (time.Time, *time.Time, int, error) {
		var triggers []AutomationTrigger
		if err := json.Unmarshal(a.Triggers, &triggers); err != nil || len(triggers) != 1 || triggers[0].Schedule == nil {
			return time.Time{}, nil, 0, fmt.Errorf("stored triggers are not one schedule trigger")
		}
		if a.NextFireAt == nil {
			return time.Time{}, nil, 0, fmt.Errorf("the automation is enabled with no next fire")
		}
		fireAt, next, missed := triggers[0].Schedule.Advance(*a.NextFireAt, now)
		if fireAt.IsZero() {
			return time.Time{}, nil, 0, errScheduleFinished
		}
		return fireAt, next, missed, nil
	}
}

// retireUnschedulable stops a due automation the claim could not advance, so it
// is not retried every tick: a finished schedule just disables it; stored
// triggers that no longer decode pause it template_invalid.
func (s *Service) retireUnschedulable(ctx context.Context, f store.ClaimFailure) {
	reason := AutomationPausedTemplateInvalid
	if errors.Is(f.Err, errScheduleFinished) {
		reason = ""
	}
	if _, err := s.Automations.SetAgentAutomationPaused(ctx, f.Automation.ID, true, reason); err != nil {
		log.Printf("agent-automations: %s cannot be scheduled (%v) and pausing it failed: %v", f.Automation.ID, f.Err, err)
		return
	}
	log.Printf("agent-automations: %s stopped: %v", f.Automation.ID, f.Err)
}

// automationIdempotencyKey is the create key of one fire: the same on every
// retry, recovery and replica, and distinct for every fire_at. Microseconds,
// the resolution of the run table's fire_at, so two run-now fires in one
// second never share a key.
func automationIdempotencyKey(automationID string, fireAt time.Time) string {
	return "aut:" + automationID + ":" + strconv.FormatInt(fireAt.UnixMicro(), 10)
}

// fireAutomationRun is ADR101 D3 steps 3 and 4 for a run the caller owns:
// re-authorize the creator (D4), skip on overlap (D6), create the session
// (idempotently) and settle the run. A definite answer settles the run; a
// transient failure (authorization or database unavailable, a 5xx-class
// create error, a canceled context) settles nothing and is returned, so the
// run stays claimed and the recovery pass retries it after the lease.
func (s *Service) fireAutomationRun(ctx context.Context, c store.ClaimedRun) error {
	outcome, sessionID, errText, err := s.decideAutomationRun(ctx, c)
	if err != nil {
		return err
	}
	return s.settleAutomationRun(ctx, c, outcome, sessionID, errText)
}

// decideAutomationRun returns the run's definite outcome, or a transient error.
func (s *Service) decideAutomationRun(ctx context.Context, c store.ClaimedRun) (outcome, sessionID, errText string, err error) {
	a, run := c.Automation, c.Run
	key := automationIdempotencyKey(a.ID, run.FireAt)

	// A retry of a fire that already created its session (the replica died
	// before settling) finishes that fire, whatever has changed since.
	if existing, _, err := s.Store.FindAgentSessionByIdempotencyKey(ctx, a.WorkspaceID, key); err == nil {
		return store.AutomationRunStarted, existing.ID, "", nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", "", "", err
	}

	var tpl AutomationTemplate
	if err := json.Unmarshal(a.Template, &tpl); err != nil {
		return store.AutomationRunFailedCreate, "", "the stored template cannot be read: " + err.Error(), nil
	}

	// D4: the creator's identity, through the same can_create check a manual
	// create runs.
	if a.CreatedBy == "" {
		return store.AutomationRunDenied, "", "the automation has no creator", nil
	}
	ctx = core.WithIdentity(ctx, core.Identity{Subject: a.CreatedBy, Method: "session"})
	ctx = core.WithWorkspace(ctx, a.WorkspaceID)
	if _, err := s.AuthorizeWorkspace(ctx, core.RelCanCreate, a.WorkspaceID); err != nil {
		if errors.Is(err, core.ErrForbidden) {
			return store.AutomationRunDenied, "", "the creator can no longer create sessions in this workspace", nil
		}
		return "", "", "", err
	}

	// D6: skip while the previous run's session is still working.
	if previous, found, err := s.Automations.LatestStartedSession(ctx, a.ID); err != nil {
		return "", "", "", err
	} else if found {
		record, err := s.Store.GetAgentSession(ctx, previous)
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			return "", "", "", err
		case stateOf(record.Phase) == StateWorking:
			return store.AutomationRunSkippedOverlap, "", "the previous run's session " + previous + " is still working", nil
		}
	}

	view, err := s.Create(ctx, tpl.createRequest(a, key))
	switch {
	case err == nil:
		return store.AutomationRunStarted, view.ID, "", nil
	case isDefiniteRefusal(err):
		return store.AutomationRunFailedCreate, "", truncateRunError(err.Error()), nil
	}
	return "", "", "", err
}

// createRequest is the session create an automation's template describes.
func (t AutomationTemplate) createRequest(a store.AgentAutomation, key string) CreateRequest {
	return CreateRequest{
		OwnerID: a.WorkspaceID, Repo: t.Repo, Branch: t.Branch,
		AgentConfig:     AgentConfig{Agent: t.AgentConfig.Agent, Model: t.AgentConfig.Model, Task: t.Task, Template: t.AgentConfig.Template},
		EgressAllowlist: t.EgressAllowlist, IdempotencyKey: key, AutomationID: a.ID, Title: t.Title, Tags: t.Tags,
		OutputSchema: t.OutputSchema, OutputRequired: t.OutputRequired, MaxSpendUSD: t.MaxSpendUSD,
		SessionSecretsInput: SessionSecretsInput{SecretNames: t.SecretNames}, Skills: t.Skills,
	}
}

// isDefiniteRefusal is a create error that is the request's own fault (the
// 4xx classes): retrying the same template cannot succeed until something
// changes, so the run settles failed_create. Every other error (503 classes,
// 500s, a canceled context) is transient.
func isDefiniteRefusal(err error) bool {
	for _, class := range []error{core.ErrBadRequest, core.ErrForbidden, core.ErrConflict, core.ErrNotFound,
		core.ErrGone, core.ErrTooLarge, core.ErrPaymentRequired, core.ErrBillingEnforced} {
		if errors.Is(err, class) {
			return true
		}
	}
	return false
}

func truncateRunError(msg string) string {
	if len(msg) > maxRunErrorLen {
		return msg[:maxRunErrorLen]
	}
	return msg
}

// settleAutomationRun books the outcome and applies the D7 auto-pause: a denied
// run pauses the automation at once (creator_lost_access) and the third
// consecutive failed_create pauses it (repeated_failures). A run somebody else
// already settled is left as stored.
func (s *Service) settleAutomationRun(ctx context.Context, c store.ClaimedRun, outcome, sessionID, errText string) error {
	a, run := c.Automation, c.Run
	failures, err := s.Automations.SettleRun(ctx, a.ID, run.FireAt, outcome, sessionID, errText)
	if errors.Is(err, store.ErrAgentAutomationRunSettled) || errors.Is(err, store.ErrNotFound) {
		log.Printf("agent-automations: %s fire_at=%s was already settled or deleted", a.ID, run.FireAt.UTC().Format(time.RFC3339Nano))
		return nil
	}
	if err != nil {
		return err
	}
	log.Printf("agent-automations: %s fire_at=%s workspace=%s outcome=%s session=%s missed=%d",
		a.ID, run.FireAt.UTC().Format(time.RFC3339Nano), a.WorkspaceID, outcome, sessionID, run.MissedTicks)

	reason := ""
	switch {
	case outcome == store.AutomationRunDenied:
		reason = AutomationPausedCreatorLostAccess
	case outcome == store.AutomationRunFailedCreate && failures >= automationFailureLimit:
		reason = AutomationPausedRepeatedFailures
	}
	if reason != "" {
		if _, err := s.Automations.SetAgentAutomationPaused(ctx, a.ID, true, reason); err != nil {
			// The run is booked; the next failed run pauses again.
			log.Printf("agent-automations: %s run settled %s but pausing (%s) failed: %v", a.ID, outcome, reason, err)
		} else {
			log.Printf("agent-automations: %s paused: %s", a.ID, reason)
		}
	}
	return nil
}
