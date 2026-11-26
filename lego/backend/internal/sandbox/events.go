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
	"cmp"
	"context"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A sandbox's event timeline (w1/m658), as Render's sandbox details drawer
// reads it through `sandboxEvents`: its lifecycle transitions, the commands
// and file transfers run in it, and the snapshots taken of it, newest first.
// docs/render-artifacts/sandbox-groups.md § The dashboard records Render's
// shape and what bex can report.

// LifecycleStore keeps each sandbox's lifecycle events; *store.PGStore
// implements it.
type LifecycleStore interface {
	RecordSandboxLifecycleEvent(ctx context.Context, e store.SandboxLifecycleEvent) error
	SandboxLifecycleEvents(ctx context.Context, workspaceID, sandboxID string) ([]store.SandboxLifecycleEvent, error)
	SandboxStartEvents(ctx context.Context, workspaceID string, sandboxIDs []string) ([]store.SandboxLifecycleEvent, error)
	ExpireSandboxLifecycleEvents(ctx context.Context, now time.Time) error
}

// Render's SandboxLifecycleEventType.
const (
	LifecycleCreated    = "CREATED"
	LifecycleReady      = "READY"
	LifecycleSuspending = "SUSPENDING"
	LifecycleSuspended  = "SUSPENDED"
	LifecycleResuming   = "RESUMING"
	LifecycleTerminated = "TERMINATED"
	LifecycleErrored    = "ERRORED"
)

// phaseLifecycle is the event a lifecycle verb's phase records.
var phaseLifecycle = map[Status]string{
	StatusSuspended:  LifecycleSuspended,
	StatusResuming:   LifecycleResuming,
	StatusTerminated: LifecycleTerminated,
}

// lifecycleLabels are the English labels Render's events carry. The dashboard
// words each type itself, in the reader's language.
var lifecycleLabels = map[string]string{
	LifecycleCreated:    "Created",
	LifecycleReady:      "Ready",
	LifecycleSuspending: "Suspending",
	LifecycleSuspended:  "Suspended",
	LifecycleResuming:   "Resuming",
	LifecycleTerminated: "Terminated",
	LifecycleErrored:    "Errored",
}

// Render's lifecycle triggers: who or what made the transition.
const (
	TriggerSDK         = "SDK"
	TriggerCLI         = "CLI"
	TriggerAPI         = "API"
	TriggerDashboard   = "DASHBOARD"
	TriggerIdleTimeout = "IDLE_TIMEOUT"
	TriggerSystem      = "SYSTEM"
	// TriggerConnection is a resume a command or a file transfer started.
	TriggerConnection = "CONNECTION"
)

// sdkUserAgents are the User-Agent prefixes of Render's SDKs.
var sdkUserAgents = []string{"render-sdk", "render-python", "render-typescript", "renderinc-sdk"}

// lifecycleTrigger is the trigger of a transition the request in ctx made:
// the surface it came through (core.ProductSurface), or SYSTEM for one no
// request made.
func lifecycleTrigger(ctx context.Context) string {
	switch core.ProductSurface(ctx) {
	case core.SurfaceDashboard:
		return TriggerDashboard
	case core.SurfaceCLI:
		return TriggerCLI
	case core.SurfaceAPI, core.SurfaceMCP:
		if origin, ok := core.RequestOriginFrom(ctx); ok {
			agent := strings.ToLower(origin.UserAgent)
			if slices.ContainsFunc(sdkUserAgents, func(p string) bool { return strings.HasPrefix(agent, p) }) {
				return TriggerSDK
			}
		}
		return TriggerAPI
	default:
		return TriggerSystem
	}
}

// lifecycleEventWriteTimeout bounds an event write, which never fails the verb.
const lifecycleEventWriteTimeout = 5 * time.Second

// recordLifecycle records one transition of a sandbox, now. Best effort, like
// the record writes: a failed write only leaves the event out of the timeline.
func (s *Service) recordLifecycle(ctx context.Context, ws, sandboxID, lifecycleType, trigger string) {
	s.writeLifecycle(ctx, store.SandboxLifecycleEvent{WorkspaceID: ws, SandboxID: sandboxID, LifecycleType: lifecycleType, Trigger: trigger, At: s.Now()})
}

// recordCreated records a created sandbox's first two events: created, with
// who started it, and ready, since a create answers once the sandbox runs.
func (s *Service) recordCreated(ctx context.Context, ws string, box Sandbox) {
	created := store.SandboxLifecycleEvent{WorkspaceID: ws, SandboxID: box.ID, LifecycleType: LifecycleCreated, Trigger: lifecycleTrigger(ctx), At: s.Now()}
	if box.CreatedAt != nil {
		created.At = *box.CreatedAt
	}
	if identity, ok := core.IdentityFrom(ctx); ok && !identity.IsAPIKey() {
		created.UserEmail = identity.Email
	}
	s.writeLifecycle(ctx, created)
	if box.Status == StatusRunning {
		s.recordLifecycle(ctx, ws, box.ID, LifecycleReady, created.Trigger)
	}
}

func (s *Service) writeLifecycle(ctx context.Context, e store.SandboxLifecycleEvent) {
	if s.Lifecycle == nil || e.SandboxID == "" {
		return
	}
	e.At = e.At.UTC()
	e.ID = ids.Derive(ids.Event, e.SandboxID, e.LifecycleType, strconv.FormatInt(e.At.UnixNano(), 10))
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lifecycleEventWriteTimeout)
	defer cancel()
	if err := s.Lifecycle.RecordSandboxLifecycleEvent(writeCtx, e); err != nil {
		log.Printf("sandbox: record %s event of %s: %v", e.LifecycleType, e.SandboxID, err)
	}
}

// Render's SandboxEventOutcome.
const (
	OutcomeSuccess    = "SUCCESS"
	OutcomeError      = "ERROR"
	OutcomeInProgress = "IN_PROGRESS"
)

// Event kinds, Render's three SandboxEvent implementations.
const (
	EventKindLifecycle = "lifecycle"
	EventKindExecution = "execution"
	EventKindSnapshot  = "snapshot"
)

// Event is one entry of a sandbox's timeline. Kind says which of the
// kind-specific parts is set.
type Event struct {
	Kind      string
	ID        string
	Timestamp time.Time
	Outcome   string
	Label     string
	// Lifecycle events.
	LifecycleType string
	Trigger       string
	UserEmail     string
	// Execution and snapshot events.
	Execution *Execution
	Snapshot  *Snapshot
}

// EventPage is one page of a timeline, newest first.
type EventPage struct {
	HasMore bool
	Events  []Event
}

// Render's default and largest page of events.
const (
	defaultEventPage = 50
	maxEventPage     = 100
)

// newestEventFirst orders a timeline as Render's drawer does: newest first,
// and by id, descending, between events of the same instant.
func newestEventFirst(a, b Event) int {
	if c := b.Timestamp.Compare(a.Timestamp); c != 0 {
		return c
	}
	return strings.Compare(b.ID, a.ID)
}

// beforeCursor reports whether e comes after the cursor (before, beforeID)
// in timeline order, so belongs to the page that follows it.
func beforeCursor(e Event, before *time.Time, beforeID string) bool {
	if before == nil {
		return true
	}
	if c := e.Timestamp.Compare(*before); c != 0 {
		return c < 0
	}
	return beforeID != "" && e.ID < beforeID
}

// ListEvents is a sandbox's timeline, newest first, one page after the
// cursor: the events older than before, or as old with a smaller id than
// beforeID. A sandbox of another workspace is not found.
func (s *Service) ListEvents(ctx context.Context, sandboxID string, before *time.Time, beforeID string, limit int) (EventPage, error) {
	ctx, err := s.scopeSandbox(ctx, "", sandboxID)
	if err != nil {
		return EventPage{}, err
	}
	box, err := s.Get(ctx, sandboxID)
	if err != nil {
		return EventPage{}, err
	}
	ws := s.WorkspaceOrDefault(ctx)
	if limit <= 0 {
		limit = defaultEventPage
	}
	limit = min(limit, maxEventPage)
	events, err := s.lifecycleEvents(ctx, ws, box)
	if err != nil {
		return EventPage{}, err
	}
	executions, err := s.executionEvents(ctx, ws, box.ID, before, limit+1)
	if err != nil {
		return EventPage{}, err
	}
	snapshots, err := s.snapshotEvents(ctx, ws, box)
	if err != nil {
		return EventPage{}, err
	}
	events = slices.Concat(events, executions, snapshots)
	events = slices.DeleteFunc(events, func(e Event) bool { return !beforeCursor(e, before, beforeID) })
	slices.SortFunc(events, newestEventFirst)
	page := EventPage{HasMore: len(events) > limit}
	page.Events = events[:min(limit, len(events))]
	return page, nil
}

// lifecycleEvents are the sandbox's recorded transitions, with its creation
// and its end read from the sandbox itself when no event records them (a
// sandbox created before events were, or ended by a sweep).
func (s *Service) lifecycleEvents(ctx context.Context, ws string, box Sandbox) ([]Event, error) {
	var stored []store.SandboxLifecycleEvent
	if s.Lifecycle != nil {
		var err error
		if stored, err = s.Lifecycle.SandboxLifecycleEvents(ctx, ws, box.ID); err != nil {
			return nil, err
		}
	}
	has := func(t string) bool {
		return slices.ContainsFunc(stored, func(e store.SandboxLifecycleEvent) bool { return e.LifecycleType == t })
	}
	if !has(LifecycleCreated) && box.CreatedAt != nil {
		stored = append(stored, store.SandboxLifecycleEvent{SandboxID: box.ID, LifecycleType: LifecycleCreated, At: *box.CreatedAt})
	}
	if box.Status == StatusTerminated && box.TerminatedAt != nil && !has(LifecycleTerminated) {
		stored = append(stored, store.SandboxLifecycleEvent{SandboxID: box.ID, LifecycleType: LifecycleTerminated, Trigger: TriggerSystem, At: *box.TerminatedAt})
	}
	out := make([]Event, 0, len(stored))
	for _, e := range stored {
		id := cmp.Or(e.ID, ids.Derive(ids.Event, box.ID, e.LifecycleType))
		out = append(out, Event{
			Kind: EventKindLifecycle, ID: id, Timestamp: e.At.UTC(), Outcome: OutcomeSuccess,
			Label: lifecycleLabels[e.LifecycleType], LifecycleType: e.LifecycleType, Trigger: e.Trigger, UserEmail: e.UserEmail,
		})
	}
	return out, nil
}

// executionTimeline is the execution read a timeline page needs;
// *store.PGStore implements it.
type executionTimeline interface {
	SandboxExecutionsUntil(ctx context.Context, workspaceID, sandboxID string, until *time.Time, limit int) ([]store.SandboxExecution, error)
}

// executionEvents are the newest executions that started at or before
// before, enough to fill a page.
func (s *Service) executionEvents(ctx context.Context, ws, sandboxID string, before *time.Time, limit int) ([]Event, error) {
	timeline, ok := s.Executions.(executionTimeline)
	if !ok {
		return nil, nil
	}
	// The cursor's own instant may hold events already shown, so read past it.
	rows, err := timeline.SandboxExecutionsUntil(ctx, ws, sandboxID, before, limit+maxEventPage)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(rows))
	for _, r := range rows {
		ex := Execution{ID: r.ID, SandboxID: r.SandboxID, Type: r.Type, Operation: r.Operation, Command: r.Command, StartedAt: r.StartedAt, StoppedAt: r.StoppedAt, ExitCode: r.ExitCode}
		out = append(out, Event{Kind: EventKindExecution, ID: r.ID, Timestamp: r.StartedAt.UTC(), Outcome: executionOutcome(ex), Label: executionLabel(ex), Execution: &ex})
	}
	return out, nil
}

// executionOutcome is a command's outcome: its exit code once it has one, in
// progress until it stops, and an error for a command stopped with none (its
// sandbox ended under it). A file transfer that stopped succeeded.
func executionOutcome(e Execution) string {
	switch {
	case e.ExitCode != nil && *e.ExitCode == 0:
		return OutcomeSuccess
	case e.ExitCode != nil:
		return OutcomeError
	case e.StoppedAt == nil:
		return OutcomeInProgress
	case e.Type == "runs":
		return OutcomeError
	default:
		return OutcomeSuccess
	}
}

// executionLabel is an execution's English label: its command, or the path a
// transfer moved.
func executionLabel(e Execution) string {
	if e.Command != "" {
		return e.Command
	}
	return e.Operation
}

// snapshotEvents are the snapshots taken of the sandbox that are still kept.
func (s *Service) snapshotEvents(ctx context.Context, ws string, box Sandbox) ([]Event, error) {
	if s.Snapshots == nil || s.Snapshots.Store == nil || box.SandboxGroupID == "" {
		return nil, nil
	}
	rows, err := s.Snapshots.Store.SandboxSnapshots(ctx, ws, box.SandboxGroupID, "", nil, s.Now())
	if err != nil {
		return nil, store.MapError(err)
	}
	var out []Event
	for _, row := range rows {
		if row.SourceSandboxID != box.ID {
			continue
		}
		snap := snapshotView(row)
		out = append(out, Event{Kind: EventKindSnapshot, ID: snap.ID, Timestamp: snap.RequestedAt.UTC(), Outcome: snapshotOutcome(snap.Status), Label: "Snapshot", Snapshot: &snap})
	}
	return out, nil
}

func snapshotOutcome(status string) string {
	switch status {
	case store.SandboxSnapshotAvailable:
		return OutcomeSuccess
	case store.SandboxSnapshotFailed:
		return OutcomeError
	default:
		return OutcomeInProgress
	}
}
