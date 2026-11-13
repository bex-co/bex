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
	"fmt"
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Concurrency admission (w22/m311, docs/ADR042-sandbox-cluster-substrate.md
// § Sandbox groups). A group at its concurrencyLimit answers 429 to a sandbox
// create and to every resume: the explicit one, and the implicit one an exec,
// upload or download starts on a suspended sandbox. Suspended sandboxes do not
// count, and lowering the limit stops nothing that is running.

const (
	// createLease is how long the slot reserved before a create stays counted
	// while the sandbox is not yet visible in OpenSandbox's list. It outlasts the
	// synchronous create (NewClient's 330 s ceiling).
	createLease = 7 * time.Minute
	// settleLease covers the lag between OpenSandbox accepting a create or a
	// resume and its list showing the sandbox active. It only matters for a
	// sandbox the list shows absent or still suspended.
	settleLease = 2 * time.Minute
)

// admissionListTimeout bounds the live list read while the admission lock is
// held, so a hung OpenSandbox cannot hold every create of one workspace. A var
// only so a test can shorten it.
var admissionListTimeout = 10 * time.Second

// holdsSlotStatus reports whether a sandbox in this status counts toward a
// concurrency limit. Render: "suspended sandboxes do not count"
// (render.com/docs/sandboxes-limits, read 2026-11-10) and it names no other
// status, so bex counts the ones that use or are about to use capacity (creating,
// running, suspending, resuming) and not the ones that cannot run: suspended,
// errored and terminated. A suspending sandbox still holds its pod until the
// commit finishes, so it counts until it is suspended.
func holdsSlotStatus(status Status) bool {
	switch status {
	case StatusCreating, StatusRunning, StatusSuspending, StatusResuming:
		return true
	}
	return false
}

// slotLedger is one workspace's count of sandboxes holding a concurrency slot.
type slotLedger struct {
	byGroup   map[string]int
	workspace int
	holders   map[string]bool
}

// countSlots counts, per group and across the workspace, the Render sandboxes
// holding a slot. A sandbox holds one when OpenSandbox reports it in a holding
// status, or when it is absent or still reads suspended but has an unexpired
// lease (an admission took its slot and the list has not caught up). A sandbox
// with no binding belongs to the workspace's default group. Agent-session
// sandboxes are not Render sandboxes and are never counted: their feature has
// its own cap (BEX_AGENT_MAX_LIVE_SANDBOXES_PER_WORKSPACE).
func countSlots(live []osSandbox, workspace string, members map[string]store.SandboxGroupMember, defaultGroup string, now time.Time) slotLedger {
	ledger := slotLedger{byGroup: map[string]int{}, holders: map[string]bool{}}
	leased := func(m store.SandboxGroupMember) bool { return m.LeaseUntil != nil && m.LeaseUntil.After(now) }
	hold := func(id, group string) {
		ledger.byGroup[group]++
		ledger.workspace++
		ledger.holders[id] = true
	}
	seen := make(map[string]bool, len(live))
	for _, raw := range live {
		if !validOwnedSandbox(raw, workspace) || !idleTracked(raw) {
			continue
		}
		id := canonicalID(raw)
		seen[id] = true
		member, bound := members[id]
		group := defaultGroup
		if bound {
			group = member.GroupID
		}
		status := mapOpenSandboxStatus(raw.Status.State)
		if holdsSlotStatus(status) || (status == StatusSuspended && bound && leased(member)) {
			hold(id, group)
		}
	}
	for id, member := range members {
		if !seen[id] && leased(member) {
			hold(id, member.GroupID)
		}
	}
	return ledger
}

func (s *Service) groupConcurrencyError(group store.SandboxGroup, limit, active int) error {
	return core.NewTooManyRequestsError(CodeSandboxGroupConcurrencyLimit,
		fmt.Sprintf("sandbox group %q is at its concurrency limit of %d active sandboxes (%d active); suspend or terminate one, raise the limit with PATCH /v1/sandbox-groups/%s, or use another group", group.Name, limit, active, group.ID),
		map[string]any{"sandboxGroupId": group.ID, "concurrencyLimit": limit, "active": active})
}

func (s *Service) workspaceConcurrencyError(limit, active int) error {
	return core.NewTooManyRequestsError(CodeSandboxWorkspaceConcurrencyLimit,
		fmt.Sprintf("this workspace is at its limit of %d active sandboxes across all groups (%d active); suspend or terminate one, then retry", limit, active),
		map[string]any{"limit": limit, "active": active})
}

// Codes of the two 429s. Render documents none for a concurrency refusal.
const (
	// CodeSandboxGroupConcurrencyLimit is a create or resume refused because the
	// sandbox's group is at its concurrencyLimit.
	CodeSandboxGroupConcurrencyLimit = "SANDBOX_GROUP_CONCURRENCY_LIMIT"
	// CodeSandboxWorkspaceConcurrencyLimit is a create or resume refused because
	// the workspace is at its cap on active sandboxes across all its groups, bex's
	// own bound (ADR104 D11, BEX_SANDBOX_MAX_ACTIVE_PER_WORKSPACE).
	CodeSandboxWorkspaceConcurrencyLimit = "SANDBOX_WORKSPACE_CONCURRENCY_LIMIT"
)

// admit takes a slot for sandboxID in group, or refuses with a 429, as one
// critical section under the workspace's admission lock: the live OpenSandbox
// list, the stored bindings and leases, the count and the binding write all
// happen while no other create or resume of the workspace, on any bex-api
// replica, can count. So parallel requests cannot each see room for one more.
// A sandbox that already holds a slot (a repeated resume while the first still
// reads suspended) is admitted without counting itself twice. acquired is true
// when this call took a new slot, which the caller must give back if the work
// that slot was for then fails.
func (s *Service) admit(ctx context.Context, key, workspace string, group, defaultGroup store.SandboxGroup, sandboxID, op string, lease time.Duration) (acquired bool, err error) {
	err = s.Groups.WithSandboxAdmission(ctx, workspace, func(adm store.SandboxAdmission) error {
		members, err := adm.Members(ctx)
		if err != nil {
			return err
		}
		listCtx, cancel := context.WithTimeout(ctx, admissionListTimeout)
		defer cancel()
		// Sent once: the workspace's admission lock, its transaction and a
		// pooled connection are held across this read.
		live, err := s.Client.withoutReadRetry().List(listCtx, key)
		if err != nil {
			return fmt.Errorf("sandbox concurrency: list sandboxes: %w", err)
		}
		now := s.Now()
		ledger := countSlots(live, workspace, members, defaultGroup.ID, now)
		if !ledger.holders[sandboxID] {
			if limit, active := s.effectiveLimit(group), ledger.byGroup[group.ID]; active >= limit {
				s.Metrics.observeRefusal("group", op)
				return s.groupConcurrencyError(group, limit, active)
			}
			if limit := s.MaxActive; limit > 0 && ledger.workspace >= limit {
				s.Metrics.observeRefusal("workspace", op)
				return s.workspaceConcurrencyError(limit, ledger.workspace)
			}
			acquired = true
		}
		return adm.Bind(ctx, sandboxID, group.ID, now.Add(lease))
	})
	if errors.Is(err, store.ErrSandboxAdmissionBusy) {
		return false, core.NewUnavailableError(CodeSandboxAdmissionBusy,
			"this workspace's sandbox creates and resumes are queued behind a slow request; retry in a few seconds", nil)
	}
	if errors.Is(err, store.ErrNotFound) {
		return false, groupNotFound(group.ID) // the group's delete started meanwhile (Bind)
	}
	if err != nil {
		return false, err
	}
	return acquired, nil
}

// CodeSandboxAdmissionBusy is the retryable 503 of an admission that waited too
// long for its workspace's lock.
const CodeSandboxAdmissionBusy = "SANDBOX_ADMISSION_BUSY"

// resolveGroup is the group a create names, or the workspace's default one when
// it names none: created on first use, as Render does for a sandbox create
// before any group exists.
func (s *Service) resolveGroup(ctx context.Context, workspace, groupID string) (store.SandboxGroup, error) {
	if groupID == "" {
		return s.defaultGroup(ctx, workspace)
	}
	return s.loadGroup(ctx, workspace, groupID)
}

func (s *Service) defaultGroup(ctx context.Context, workspace string) (store.SandboxGroup, error) {
	g, err := s.Groups.EnsureDefaultSandboxGroup(ctx, workspace, s.newGroupLimit())
	if err != nil {
		return store.SandboxGroup{}, store.MapError(err)
	}
	return g, nil
}

// admitCreate reserves a slot for a sandbox about to be created in the group the
// request names, or the default group. With no group store (store off) it is a
// no-op and enforces nothing; naming a group then is refused.
func (s *Service) admitCreate(ctx context.Context, workspace, groupID, sandboxID string) (store.SandboxGroup, error) {
	if s.Groups == nil {
		if groupID != "" {
			return store.SandboxGroup{}, s.requireGroups()
		}
		return store.SandboxGroup{}, nil
	}
	group, err := s.resolveGroup(ctx, workspace, groupID)
	if err != nil {
		return store.SandboxGroup{}, err
	}
	defaultGroup := group
	if !group.IsDefault {
		if defaultGroup, err = s.defaultGroup(ctx, workspace); err != nil {
			return store.SandboxGroup{}, err
		}
	}
	// The workspace's OpenSandbox tenant key (agentSessionKey is the shared,
	// authorization-free lookup; the caller was authorized by Create).
	key, err := s.agentSessionKey(ctx, workspace)
	if err != nil {
		return store.SandboxGroup{}, err
	}
	if _, err := s.admit(ctx, key, workspace, group, defaultGroup, sandboxID, "create", createLease); err != nil {
		return store.SandboxGroup{}, err
	}
	return group, nil
}

// admitResume takes a slot for a suspended sandbox about to be resumed, by any
// path. It returns a release that gives the slot back when the resume then
// fails (a no-op when nothing was newly taken). A sandbox that is not a Render
// sandbox (an agent-session one) and a deployment with no group store pass
// through.
func (s *Service) admitResume(ctx context.Context, key, workspace string, raw osSandbox) (release func(), err error) {
	release = func() {}
	if s.Groups == nil || !idleTracked(raw) {
		return release, nil
	}
	id := canonicalID(raw)
	defaultGroup, err := s.defaultGroup(ctx, workspace)
	if err != nil {
		return release, err
	}
	members, err := s.Groups.SandboxGroupMembers(ctx, workspace)
	if err != nil {
		return release, store.MapError(err)
	}
	group := defaultGroup
	if m, bound := members[id]; bound && m.GroupID != defaultGroup.ID {
		if group, err = s.Groups.SandboxGroup(ctx, workspace, m.GroupID); err != nil {
			return release, store.MapError(err)
		}
	}
	acquired, err := s.admit(ctx, key, workspace, group, defaultGroup, id, "resume", settleLease)
	if err != nil {
		return release, err
	}
	if acquired {
		release = func() { s.clearSlotLease(ctx, workspace, id) }
	}
	return release, nil
}

// slotWrite runs one slot bookkeeping write on a context that survives the
// request's own cancellation, bounded by egressCleanupTimeout: releasing or
// settling a slot must land even after the client hung up, and must never hang
// the response. A failure is logged, not returned: the lease ends on its own. It
// does nothing without a group store.
func (s *Service) slotWrite(ctx context.Context, what, workspace, sandboxID string, write func(context.Context) error) {
	if s.Groups == nil {
		return
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), egressCleanupTimeout)
	defer cancel()
	if err := write(cleanup); err != nil {
		log.Printf("sandbox groups: %s workspace=%s sandbox=%s: %v", what, workspace, sandboxID, err)
	}
}

// settleSlot shortens a created sandbox's slot reservation to the lag window
// once the create has succeeded.
func (s *Service) settleSlot(ctx context.Context, workspace, sandboxID string) {
	s.slotWrite(ctx, "settle slot", workspace, sandboxID, func(c context.Context) error {
		until := s.Now().Add(settleLease)
		return s.Groups.SetSandboxSlotLease(c, workspace, sandboxID, &until)
	})
}

// clearSlotLease ends a sandbox's slot reservation: it is suspended, or its
// resume failed, so it holds nothing the list does not show.
func (s *Service) clearSlotLease(ctx context.Context, workspace, sandboxID string) {
	s.slotWrite(ctx, "clear slot lease", workspace, sandboxID, func(c context.Context) error {
		return s.Groups.SetSandboxSlotLease(c, workspace, sandboxID, nil)
	})
}

// releaseSlot removes a sandbox's group binding: its create failed, or it was
// terminated.
func (s *Service) releaseSlot(ctx context.Context, workspace, sandboxID string) {
	s.slotWrite(ctx, "release slot", workspace, sandboxID, func(c context.Context) error {
		return s.Groups.ReleaseSandboxSlot(c, workspace, sandboxID)
	})
}

// releaseFailedCreate gives back the slot of a create that failed. A timeout
// waiting for the pod leaves the slot (and its binding) alone: the sandbox may
// still come up, and it must then count in the group it was created in.
func (s *Service) releaseFailedCreate(ctx context.Context, workspace, sandboxID string, err error) {
	if IsCapacityLimit(err) || IsStartFailure(err) {
		return
	}
	s.releaseSlot(ctx, workspace, sandboxID)
}
