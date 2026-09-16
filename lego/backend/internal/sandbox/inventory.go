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

package sandbox

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

// Teardown failure reasons (closed label set for bex_sandbox_teardown_failures_total).
const (
	TeardownReasonPreviousSandbox = "previous_sandbox"
	TeardownReasonIdleReap        = "idle_reap"
	TeardownReasonInventory       = "inventory"
)

// InventoryClass is a closed label set for live-sandbox classification.
// Never put tenant/sandbox/session ids on the series.
const (
	InventoryClassClaimed        = "claimed"
	InventoryClassTerminalOrphan = "terminal_orphan"
	InventoryClassNoRowOrphan    = "no_row_orphan"
	InventoryClassTimed          = "timed"
	InventoryClassInconclusive   = "inconclusive"
)

// InventoryCounts is one reconcile pass's classification tallies.
type InventoryCounts struct {
	Claimed        int
	TerminalOrphan int
	NoRowOrphan    int
	Timed          int
	Inconclusive   int
	OldestAge      map[string]float64 // class → seconds
	Terminated     int
}

// Merge folds another pass's tallies into c (max age per class).
func (c *InventoryCounts) Merge(o InventoryCounts) {
	c.Claimed += o.Claimed
	c.TerminalOrphan += o.TerminalOrphan
	c.NoRowOrphan += o.NoRowOrphan
	c.Timed += o.Timed
	c.Inconclusive += o.Inconclusive
	c.Terminated += o.Terminated
	if c.OldestAge == nil {
		c.OldestAge = map[string]float64{}
	}
	for class, age := range o.OldestAge {
		noteOldest(c.OldestAge, class, age)
	}
}

// SessionClaim is the durable session state the inventory reconcile needs to
// decide whether a live sandbox still has an owner.
type SessionClaim struct {
	Phase     string
	SandboxID string
	Found     bool
}

// SessionClaimLookup resolves an agent-session id stamped on sandbox metadata.
// err ≠ nil means the read was inconclusive — terminate nothing that tick.
type SessionClaimLookup func(ctx context.Context, sessionID string) (SessionClaim, error)

// liveAgentPhases are phases that may still own a running sandbox — active
// turns plus hibernation (snapshot still needs the pod until reclaim finishes).
var liveAgentPhases = map[string]struct{}{
	"running":       {},
	"creating":      {},
	"resuming":      {},
	"redispatching": {},
	"hibernating":   {},
	"hibernated":    {},
}

// ReconcileWorkspaceInventory lists the workspace's live sandboxes and
// terminates those whose agent session is terminal, absent, or past the
// lifetime bound. Conservative: a list/lookup error terminates nothing for
// that sandbox; a workspace list error aborts the whole pass.
// Non-agent sandboxes (no LabelSession) are only reaped by the lifetime bound.
// apiKey, when non-empty, is the OpenSandbox tenant key already listed by the
// meter/inventory walker — avoids a second WorkspaceKey round-trip.
func (l *AgentSessionLifecycle) ReconcileWorkspaceInventory(ctx context.Context, workspaceID, apiKey string, now time.Time, lookup SessionClaimLookup) (InventoryCounts, error) {
	counts := InventoryCounts{OldestAge: map[string]float64{}}
	s := l.service
	if s == nil || !s.enabled() || lookup == nil {
		return counts, nil
	}
	key := apiKey
	if key == "" {
		var err error
		key, err = s.agentSessionKey(ctx, workspaceID)
		if err != nil {
			return counts, err
		}
	}
	rows, err := s.Client.List(ctx, key)
	if err != nil {
		return counts, err
	}
	var terminateErr error
	for _, raw := range rows {
		if err := ctx.Err(); err != nil {
			return counts, errors.Join(terminateErr, err)
		}
		if !validOwnedSandbox(raw, workspaceID) {
			continue
		}
		if mapOpenSandboxStatus(raw.Status.State) == StatusTerminated {
			continue
		}
		timeout, _ := strconv.Atoi(raw.Metadata[metadataTimeout])
		age := 0.0
		if raw.Created != nil {
			age = now.Sub(*raw.Created).Seconds()
			if age < 0 {
				age = 0
			}
		}
		sessionID := raw.Metadata[agentsession.LabelSession]
		class := InventoryClassTimed
		shouldStop := false

		switch {
		case LifetimeExpired(raw.Created, timeout, now):
			class = InventoryClassTimed
			shouldStop = true
		case sessionID == "":
			// Ordinary EA sandbox still inside its bound — leave it.
			class = InventoryClassTimed
			noteOldest(counts.OldestAge, class, age)
			counts.Timed++
			continue
		default:
			claim, err := lookup(ctx, sessionID)
			if err != nil {
				class = InventoryClassInconclusive
				counts.Inconclusive++
				noteOldest(counts.OldestAge, class, age)
				continue
			}
			class, shouldStop = classifySessionSandbox(claim, raw.ID)
		}

		switch class {
		case InventoryClassClaimed:
			counts.Claimed++
		case InventoryClassTerminalOrphan:
			counts.TerminalOrphan++
		case InventoryClassNoRowOrphan:
			counts.NoRowOrphan++
		case InventoryClassTimed:
			counts.Timed++
		case InventoryClassInconclusive:
			counts.Inconclusive++
		}
		noteOldest(counts.OldestAge, class, age)

		if !shouldStop {
			continue
		}
		if err := s.terminateObserved(ctx, key, &raw); err != nil {
			terminateErr = errors.Join(terminateErr, err)
			continue
		}
		counts.Terminated++
	}
	return counts, terminateErr
}

// classifySessionSandbox decides whether a live sandbox is still owned by its
// stamped session. A blank or matching sandbox_id on a live phase stays claimed
// (steer may have blanked the column while the durable previous-sandbox intent
// finishes); anything else is an orphan.
func classifySessionSandbox(claim SessionClaim, sandboxID string) (class string, shouldStop bool) {
	if !claim.Found {
		return InventoryClassNoRowOrphan, true
	}
	_, live := liveAgentPhases[claim.Phase]
	ownsThis := claim.SandboxID == sandboxID || claim.SandboxID == ""
	if ownsThis && live {
		return InventoryClassClaimed, false
	}
	return InventoryClassTerminalOrphan, true
}

func noteOldest(oldest map[string]float64, class string, age float64) {
	if cur, ok := oldest[class]; !ok || age > cur {
		oldest[class] = age
	}
}
