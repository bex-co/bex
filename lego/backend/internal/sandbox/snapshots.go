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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Sandbox snapshots (w32/m285, docs/ADR104-sandbox-render-parity.md D4). A
// filesystem snapshot is a running sandbox's root filesystem, committed by the
// substrate while the sandbox keeps running and then copied into a repository
// of its own in the sandbox registry. A sandbox create that names the snapshot
// (snapshotId) or its name (snapshotName) starts from that copy and inherits
// the source's plan, network policy and timeouts.
//
// Render's default kind, `runtime`, also saves memory and running processes.
// The substrate cannot checkpoint those yet (w2/m322), so a runtime capture is
// refused by name and never downgraded to a filesystem one.

// Codes. Render's errorCode vocabulary names some of these refusals; the rest
// are bex's.
const (
	// CodeSnapshotNotFound is Render's `snapshot_not_found`: no such snapshot
	// in the group, or none of that name.
	CodeSnapshotNotFound = "snapshot_not_found"
	// CodeSnapshotNotAvailable is Render's 409 for a create from a snapshot
	// that is still creating or has failed.
	CodeSnapshotNotAvailable = "snapshot_not_available"
	// CodeSnapshotCreating is Render's 409 for deleting a snapshot whose
	// capture is in flight.
	CodeSnapshotCreating = "snapshot_creating"
	// CodeInvalidSnapshotName is Render's `invalid_snapshot_name`.
	CodeInvalidSnapshotName = "invalid_snapshot_name"
	// CodeInvalidSnapshotID is Render's `invalid_snapshot_id`.
	CodeInvalidSnapshotID = "invalid_snapshot_id"
	// CodeInvalidSnapshotStatus is Render's `invalid_status` on the snapshot list.
	CodeInvalidSnapshotStatus = "invalid_status"
	// CodeSnapshotRuntimeUnsupported refuses Render's default kind, `runtime`,
	// until the substrate can checkpoint memory and processes (w2/m322).
	CodeSnapshotRuntimeUnsupported = "SANDBOX_SNAPSHOT_RUNTIME_UNSUPPORTED"
	// CodeSandboxSnapshotInvalid refuses a capture or restore Render refuses
	// with a message only.
	CodeSandboxSnapshotInvalid = "SANDBOX_SNAPSHOT_INVALID"
	// CodeSandboxSnapshotLimit is the 429 of a capture past a rate or count
	// limit.
	CodeSandboxSnapshotLimit = "SANDBOX_SNAPSHOT_LIMIT"
	// CodeSandboxSnapshotsUnavailable is the 503 of a deployment with no
	// sandbox registry or store to keep snapshots in.
	CodeSandboxSnapshotsUnavailable = "SANDBOX_SNAPSHOTS_UNAVAILABLE"
)

const (
	maxSnapshotName = 64
	// defaultSnapshotTTL is Render's snapshot lifetime: 30 days.
	defaultSnapshotTTL = 30 * 24 * time.Hour
	// maxSnapshotTTL keeps an expiry inside what a timestamp holds; a longer
	// life is `ttlSeconds: 0`.
	maxSnapshotTTL = 100 * 365 * 24 * time.Hour
	// DefaultSnapshotCapturesPerMinute is Render's capture rate limit, per
	// user (render.com/docs/sandboxes-limits).
	DefaultSnapshotCapturesPerMinute = 10
	// DefaultSnapshotCapturesInFlight is how many captures one workspace may
	// have creating at once: the substrate commits two at a time for every
	// workspace together, so one workspace must not hold the queue.
	DefaultSnapshotCapturesInFlight = 5
	// DefaultSnapshotsPerGroup caps a group's snapshots (ADR104 D11).
	DefaultSnapshotsPerGroup = 100
	// defaultMaxSnapshotBytes caps one snapshot's compressed size: a sandbox's
	// 40 GB writable filesystem.
	defaultMaxSnapshotBytes = 40 * store.BytesPerDecimalGB
	// snapshotsSegment is the path under a workspace's snapshot namespace that
	// holds its snapshots, one repository per snapshot, and snapshotImageTag
	// the one tag each holds (registry retention keeps a repository's newest
	// tags and reaps untagged manifests).
	snapshotsSegment = "/snapshots/"
	snapshotImageTag = "snapshot"
	// snapshotCaptureTimeout fails a capture still creating: the substrate
	// gives its commit 15 minutes (snapshot_create_timeout_seconds).
	snapshotCaptureTimeout = 20 * time.Minute
	// snapshotCopyTimeout bounds the copy into the snapshot's repository, and
	// snapshotClaimHold, longer, is how long one replica holds a capture it is
	// settling.
	snapshotCopyTimeout = 5 * time.Minute
	snapshotClaimHold   = 6 * time.Minute
	snapshotSweepEvery  = 3 * time.Second
	snapshotSweepBatch  = 20
	// snapshotCaptureAttempts bounds the captures requested for one snapshot
	// when the substrate cannot start them (recapture).
	snapshotCaptureAttempts = 3
	// snapshotReapTimeout bounds the cleanup a delete does after its answer
	// is decided.
	snapshotReapTimeout = 30 * time.Second
)

// snapshotNoExpiry is the expiresAt Render reports for `ttlSeconds: 0`.
var snapshotNoExpiry = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

var snapshotStatuses = []string{store.SandboxSnapshotCreating, store.SandboxSnapshotAvailable, store.SandboxSnapshotFailed}

// SnapshotStatuses is the snapshot list's status filter vocabulary, for
// refusals the REST gate answers before the Service.
func SnapshotStatuses() []string { return slices.Clone(snapshotStatuses) }

// Snapshot is Render's sandbox snapshot, the shape of the pinned OpenAPI schema.
type Snapshot struct {
	ID              string  `json:"id"`
	SandboxGroupID  string  `json:"sandboxGroupId"`
	SourceSandboxID string  `json:"sourceSandboxId"`
	Name            *string `json:"name"`
	Kind            string  `json:"kind"`
	Status          string  `json:"status"`
	// Plan is the source sandbox's plan when the snapshot was captured.
	Plan        string     `json:"plan"`
	RequestedAt time.Time  `json:"requestedAt"`
	CapturedAt  *time.Time `json:"capturedAt"`
	// ExpiresAt is when the snapshot can no longer be read or restored;
	// 9999-12-31T23:59:59Z for one that never expires.
	ExpiresAt  time.Time `json:"expiresAt"`
	TTLSeconds int64     `json:"ttlSeconds"`
	SizeBytes  *int64    `json:"sizeBytes"`
	Error      *string   `json:"error"`
}

// SnapshotEnvelope wraps a Snapshot for Render's cursor-paginated list.
type SnapshotEnvelope struct {
	Snapshot Snapshot `json:"snapshot"`
	Cursor   string   `json:"cursor"`
}

// CreateSnapshotRequest is the body of Render's create-sandbox-snapshot.
type CreateSnapshotRequest struct {
	// Kind is `filesystem`; empty is Render's default, `runtime`.
	Kind string  `json:"kind,omitempty"`
	Name *string `json:"name,omitempty"`
	// TTLSeconds is the lifetime after the last use (0 keeps the snapshot
	// until it is deleted); ExpiresAt is its deprecated form.
	TTLSeconds *int64     `json:"ttlSeconds,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
}

// SnapshotStore is where snapshots and their storage meter live;
// *store.PGStore implements it.
type SnapshotStore interface {
	CreateSandboxSnapshot(ctx context.Context, in store.NewSandboxSnapshot, limits store.SandboxSnapshotLimits, now time.Time) (store.SandboxSnapshot, error)
	SandboxSnapshot(ctx context.Context, workspaceID, groupID, id string, now time.Time) (store.SandboxSnapshot, error)
	SandboxSnapshots(ctx context.Context, workspaceID, groupID, name string, statuses []string, now time.Time) ([]store.SandboxSnapshot, error)
	NewestAvailableSandboxSnapshot(ctx context.Context, workspaceID, groupID, name string, now time.Time) (store.SandboxSnapshot, error)
	SetSandboxSnapshotRuntimeID(ctx context.Context, id, runtimeID string) (bool, error)
	TakeSandboxSnapshotCaptureAttempt(ctx context.Context, id string, maxAttempts int) (int, error)
	ClaimCreatingSandboxSnapshots(ctx context.Context, now time.Time, hold time.Duration, limit int) ([]store.SandboxSnapshot, error)
	ReleaseSandboxSnapshotClaim(ctx context.Context, id string) error
	MarkSandboxSnapshotAvailable(ctx context.Context, id, imageDigest string, sizeBytes int64, capturedAt time.Time) (bool, error)
	FailSandboxSnapshot(ctx context.Context, id, reason string) (bool, error)
	ClearSandboxSnapshotRuntimeID(ctx context.Context, id string) error
	UseSandboxSnapshot(ctx context.Context, id string, at time.Time) error
	DeleteSandboxSnapshot(ctx context.Context, workspaceID, groupID, id string, now time.Time) (store.SandboxSnapshot, store.SandboxSnapshotDeletion, error)
	EndGroupSandboxSnapshots(ctx context.Context, workspaceID, groupID, reason string, now time.Time) ([]store.SandboxSnapshot, error)
	ReapableSandboxSnapshots(ctx context.Context, now time.Time, limit int) ([]store.SandboxSnapshot, error)
	MarkSandboxSnapshotReaped(ctx context.Context, id string, at time.Time) error
	StaleCreatingSandboxSnapshots(ctx context.Context, olderThan time.Time, reason string) ([]store.SandboxSnapshot, error)

	StartSandboxStorage(ctx context.Context, workspaceID, objectID, kind, ref string, at time.Time) error
	SetSandboxStorageBytes(ctx context.Context, workspaceID, objectID string, startedAt time.Time, bytes int64) error
	EndSandboxStorage(ctx context.Context, workspaceID, objectID string, at time.Time) error
}

// SnapshotReaper deletes a snapshot's image, and the tag a capture's commit
// left in its source sandbox's repository, from the registry;
// *RegistryReaper implements it.
type SnapshotReaper interface {
	DeleteSnapshotImage(ctx context.Context, workspaceID, snapshotID string) error
	DeleteCommitTag(ctx context.Context, workspaceID, ref, tag string) error
}

// Snapshots keeps sandbox snapshots. nil on the Service answers every
// snapshot verb 503. The copy into a snapshot's repository uses the Service's
// Images (the sandbox registry and the importer's credential).
type Snapshots struct {
	Store SnapshotStore
	// Reaper deletes snapshot images from the registry; nil leaves them, as a
	// deployment without the reaper credential does.
	Reaper SnapshotReaper
	// MaxBytes caps one snapshot's compressed size (0: defaultMaxSnapshotBytes).
	MaxBytes int64
	// Limits bound captures: a requester's per minute, a workspace's in
	// flight and a group's snapshots. A zero field is no limit; the
	// composition root sets the Default* constants or the operator's values.
	Limits store.SandboxSnapshotLimits
}

func (sn *Snapshots) maxBytes() int64 {
	if sn.MaxBytes > 0 {
		return sn.MaxBytes
	}
	return defaultMaxSnapshotBytes
}

// requireSnapshots is the 503 of a deployment that cannot keep snapshots.
func (s *Service) requireSnapshots() error {
	if s.Snapshots == nil || s.Images == nil || s.Groups == nil {
		return core.NewUnavailableError(CodeSandboxSnapshotsUnavailable,
			"sandbox snapshots are not available on this deployment: they need the control-plane store and the sandbox registry", nil)
	}
	return nil
}

// snapshotRepository is a snapshot's repository path in the sandbox registry,
// inside the workspace's snapshot namespace so a restored sandbox's pod pulls
// it with the namespace's own read-only credential.
func (r *RegistryConfig) snapshotRepository(workspaceID, snapshotID string) string {
	return r.namespacePath(workspaceID) + snapshotsSegment + snapshotID
}

// commitTag is the tag the substrate's commit pushes for a capture: `snap-`
// and the capture id's 32 hex digits (OpenSandbox build_public_snapshot_tag;
// a capture id that is not a UUID is hashed).
func commitTag(runtimeID string) string {
	hexID := strings.ToLower(strings.ReplaceAll(runtimeID, "-", ""))
	if !publicCommitHex.MatchString(hexID) {
		sum := sha256.Sum256([]byte(runtimeID))
		hexID = hex.EncodeToString(sum[:])[:32]
	}
	return "snap-" + hexID
}

var publicCommitHex = regexp.MustCompile(`^[0-9a-f]{32}$`)

// publicCommitTag matches commitTag's output: what a capture leaves in its
// source sandbox's repository until the snapshot's own copy exists.
var publicCommitTag = regexp.MustCompile(`^snap-[0-9a-f]{32}$`)

// restoredImage matches the image a sandbox restored from a snapshot runs, so
// a read and a charge show the snapshot's id instead of the sandbox
// registry's internal address.
var restoredImage = regexp.MustCompile(regexp.QuoteMeta(snapshotsSegment) + `(snp-[0-9a-v]{20})@sha256:[0-9a-f]{64}$`)

func validateSnapshotID(id string) error {
	if kind, ok := ids.KindOf(id); ok && kind == ids.SandboxSnapshot {
		return nil
	}
	return core.NewBadRequestError(CodeInvalidSnapshotID,
		fmt.Sprintf("invalid snapshot ID %q; they look like %s-…", id, ids.SandboxSnapshot.Prefix()),
		map[string]any{"field": "snapshotId"})
}

// validateSnapshotName applies Render's name rule: 1 to 64 characters, not
// beginning with the snapshot id prefix (a name must never read as an id).
func validateSnapshotName(n, field string) error {
	if labelOK(n, maxSnapshotName) && !strings.HasPrefix(n, ids.SandboxSnapshot.Prefix()+"-") {
		return nil
	}
	return InvalidSnapshotNameError(field)
}

// InvalidSnapshotNameError is Render's invalid_snapshot_name for the named
// field, which the REST gate also answers for a name its schema refuses.
func InvalidSnapshotNameError(field string) error {
	return core.NewBadRequestError(CodeInvalidSnapshotName,
		fmt.Sprintf("a snapshot name must be 1 to %d characters, with no leading or trailing space and no control characters, and must not start with %s-",
			maxSnapshotName, ids.SandboxSnapshot.Prefix()),
		map[string]any{"field": field, "maxLength": maxSnapshotName})
}

// ValidateSnapshotListQuery is the part of Render's list-sandbox-snapshots
// request check the generic OpenAPI gate cannot make: Render requires ownerId
// exactly once (`invalid_owner_id`), as the group list does, and a cursor is a
// well-formed `snp-` id.
func ValidateSnapshotListQuery(q url.Values) error {
	if owners := q["ownerId"]; len(owners) != 1 || owners[0] == "" {
		return core.NewBadRequestError(CodeInvalidOwnerID, "ownerId is required exactly once",
			map[string]any{"field": "ownerId"})
	}
	return validateListCursor(q, ids.SandboxSnapshot)
}

func snapshotNotFound(id string) error {
	return core.NewNotFoundError(CodeSnapshotNotFound, "sandbox snapshot not found in this sandbox group", map[string]any{"id": id})
}

func snapshotView(s store.SandboxSnapshot) Snapshot {
	expires := snapshotNoExpiry
	if s.ExpiresAt != nil {
		expires = *s.ExpiresAt
	}
	return Snapshot{
		ID: s.ID, SandboxGroupID: s.SandboxGroupID, SourceSandboxID: s.SourceSandboxID, Name: s.Name,
		Kind: s.Kind, Status: s.Status, Plan: s.Plan, RequestedAt: s.RequestedAt, CapturedAt: s.CapturedAt,
		ExpiresAt: expires, TTLSeconds: s.TTLSeconds, SizeBytes: s.SizeBytes, Error: s.Error,
	}
}

// snapshotTTL validates a capture request and returns the snapshot's lifetime
// in seconds (0: it never expires).
func snapshotTTL(req CreateSnapshotRequest, now time.Time) (int64, error) {
	switch req.Kind {
	case store.SandboxSnapshotFilesystem:
	case "", store.SandboxSnapshotRuntime:
		return 0, core.NewBadRequestError(CodeSnapshotRuntimeUnsupported,
			"bex cannot capture a runtime snapshot (memory and running processes) yet; capture the filesystem instead with kind filesystem (--kind filesystem)",
			map[string]any{"field": "kind", "supported": []string{store.SandboxSnapshotFilesystem}})
	default:
		return 0, core.NewBadRequestError(CodeSandboxSnapshotInvalid,
			fmt.Sprintf("%q is not a snapshot kind; use %s", req.Kind, store.SandboxSnapshotFilesystem),
			map[string]any{"field": "kind", "supported": []string{store.SandboxSnapshotFilesystem}})
	}
	if req.Name != nil {
		if err := validateSnapshotName(*req.Name, "name"); err != nil {
			return 0, err
		}
	}
	ttl := defaultSnapshotTTL
	switch {
	case req.TTLSeconds != nil && req.ExpiresAt != nil:
		return 0, core.NewBadRequestError(CodeSandboxSnapshotInvalid, "set a snapshot's lifetime with ttlSeconds or with expiresAt, not both",
			map[string]any{"fields": []string{"ttlSeconds", "expiresAt"}})
	case req.TTLSeconds != nil:
		if *req.TTLSeconds < 0 || *req.TTLSeconds > int64(maxSnapshotTTL/time.Second) {
			return 0, core.NewBadRequestError(CodeSandboxSnapshotInvalid,
				fmt.Sprintf("ttlSeconds must be between 0 (never expires) and %d", int64(maxSnapshotTTL/time.Second)),
				map[string]any{"field": "ttlSeconds", "max": int64(maxSnapshotTTL / time.Second)})
		}
		ttl = time.Duration(*req.TTLSeconds) * time.Second
	case req.ExpiresAt != nil:
		ttl = req.ExpiresAt.Sub(now)
		if ttl <= 0 || ttl > maxSnapshotTTL {
			return 0, core.NewBadRequestError(CodeSandboxSnapshotInvalid, "expiresAt must be in the future",
				map[string]any{"field": "expiresAt"})
		}
		// Whole seconds, never shorter than asked.
		ttl = (ttl + time.Second - 1).Truncate(time.Second)
	}
	return int64(ttl / time.Second), nil
}

// CreateSnapshot starts capturing a filesystem snapshot of a running sandbox
// and returns it in `creating`. The sandbox keeps running: the substrate
// freezes it only while its root filesystem is committed.
func (s *Service) CreateSnapshot(ctx context.Context, sandboxID string, req CreateSnapshotRequest) (Snapshot, error) {
	ctx, err := s.scopeSandbox(ctx, "", sandboxID)
	if err != nil {
		return Snapshot{}, err
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return Snapshot{}, err
	}
	if !s.enabled() {
		return Snapshot{}, core.ErrSandboxesUnavailable
	}
	if err := s.requireSnapshots(); err != nil {
		return Snapshot{}, err
	}
	now := s.Now()
	ttl, err := snapshotTTL(req, now)
	if err != nil {
		return Snapshot{}, err
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	ws := s.WorkspaceOrDefault(ctx)
	raw, err := s.ownedSandbox(ctx, key, ws, sandboxID)
	if err != nil {
		box, endedErr := s.orEnded(ctx, ws, sandboxID, err)
		if endedErr != nil {
			return Snapshot{}, endedErr
		}
		return Snapshot{}, sandboxEndedError(box)
	}
	if status := mapOpenSandboxStatus(raw.Status.State); status != StatusRunning {
		return Snapshot{}, snapshotSourceNotRunning(canonicalID(raw), status)
	}
	// A snapshot stores up to a sandbox's whole filesystem in the platform
	// registry, so a capture takes the billing gates a sandbox create takes
	// and the workspace's storage cap a suspend takes.
	if err := s.RequireBillingFor(ctx, ws, true); err != nil {
		return Snapshot{}, err
	}
	if err := s.checkStorageCap(ctx, raw, "snapshot"); err != nil {
		return Snapshot{}, err
	}
	box, err := s.withRules(ctx, ws, s.sandboxView(raw, ws))
	if err != nil {
		return Snapshot{}, err
	}
	grouped, err := s.withGroups(ctx, ws, []Sandbox{box})
	if err != nil {
		return Snapshot{}, err
	}
	box = grouped[0]
	policy, err := json.Marshal(box.NetworkPolicy)
	if err != nil {
		return Snapshot{}, err
	}
	snap, err := s.Snapshots.Store.CreateSandboxSnapshot(ctx, store.NewSandboxSnapshot{
		WorkspaceID: ws, SandboxGroupID: box.SandboxGroupID, SourceSandboxID: box.ID, SourceRef: raw.ID,
		Name: req.Name, Kind: store.SandboxSnapshotFilesystem, Plan: string(box.Plan), NetworkPolicy: policy,
		TimeoutSeconds: box.TimeoutSeconds, IdleTimeoutSeconds: box.IdleTimeoutSeconds, TTLSeconds: ttl,
		RequestedBy: ownerID(ctx),
	}, s.Snapshots.Limits, now)
	if err != nil {
		return Snapshot{}, s.snapshotRefusal(err, box)
	}
	capture, err := s.Client.CreateSnapshot(ctx, key, raw.ID, snap.ID)
	if err != nil {
		return Snapshot{}, s.captureNotStarted(ctx, snap, box, err)
	}
	if _, err := s.Snapshots.Store.SetSandboxSnapshotRuntimeID(ctx, snap.ID, capture.ID); err != nil {
		// The capture runs with nothing to find it by. The stale sweep fails
		// the row; the substrate's object is the one thing left behind.
		log.Printf("sandbox snapshots: record the capture %s of %s: %v", capture.ID, snap.ID, err)
	}
	return snapshotView(snap), nil
}

func snapshotSourceNotRunning(id string, status Status) error {
	return core.NewConflictError(CodeSandboxNotRunning,
		fmt.Sprintf("sandbox %s is %s; a snapshot can only be captured from a running sandbox", id, status),
		map[string]any{"id": id, "status": string(status)})
}

// snapshotRefusal names a capture the store refused.
func (s *Service) snapshotRefusal(err error, box Sandbox) error {
	limits := s.Snapshots.Limits
	var busy *store.SandboxSnapshotBusyError
	switch {
	case errors.As(err, &busy):
		s.Metrics.observeRefusal("sandbox", "snapshot")
		return core.NewTooManyRequestsError(CodeSandboxSnapshotLimit,
			fmt.Sprintf("snapshot %s is still being captured from sandbox %s; wait for it to finish, then capture again", busy.ID, box.ID),
			map[string]any{"snapshotId": busy.ID, "sandboxId": box.ID})
	case errors.Is(err, store.ErrSandboxSnapshotRate):
		s.Metrics.observeRefusal("rate", "snapshot")
		return core.NewTooManyRequestsError(CodeSandboxSnapshotLimit,
			fmt.Sprintf("you have requested %d snapshots in the last minute, the limit; try again in a minute", limits.PerRequesterPerMinute),
			map[string]any{"limit": limits.PerRequesterPerMinute, "windowSeconds": 60})
	case errors.Is(err, store.ErrSandboxSnapshotInFlight):
		s.Metrics.observeRefusal("workspace", "snapshot")
		return core.NewTooManyRequestsError(CodeSandboxSnapshotLimit,
			fmt.Sprintf("this workspace already has %d snapshots being captured, its limit; wait for one to finish, then capture again", limits.InFlight),
			map[string]any{"limit": limits.InFlight})
	case errors.Is(err, store.ErrSandboxSnapshotGroupLimit):
		s.Metrics.observeRefusal("group", "snapshot")
		return core.NewTooManyRequestsError(CodeSandboxSnapshotLimit,
			fmt.Sprintf("sandbox group %s already holds %d snapshots, its limit; delete snapshots you no longer need, then capture again", box.SandboxGroupID, limits.PerGroup),
			map[string]any{"limit": limits.PerGroup, "sandboxGroupId": box.SandboxGroupID})
	case errors.Is(err, store.ErrNotFound):
		return core.SandboxGroupNotFound(box.SandboxGroupID)
	}
	return store.MapError(err)
}

// captureNotStarted fails the snapshot whose capture the substrate did not
// accept and names why, so no `creating` row waits on a capture that never
// began.
func (s *Service) captureNotStarted(ctx context.Context, snap store.SandboxSnapshot, box Sandbox, cause error) error {
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordWriteTimeout)
	defer cancel()
	if _, err := s.Snapshots.Store.FailSandboxSnapshot(failCtx, snap.ID, "the capture could not be started"); err != nil {
		log.Printf("sandbox snapshots: fail %s, whose capture did not start: %v", snap.ID, err)
	}
	switch {
	case errors.Is(cause, errOpenSandboxConflict):
		// The sandbox left `running` between the read and the capture.
		return core.NewConflictError(CodeSandboxNotRunning,
			fmt.Sprintf("sandbox %s is no longer running; a snapshot can only be captured from a running sandbox", box.ID),
			map[string]any{"id": box.ID})
	case errors.Is(cause, errOpenSandboxNotFound):
		return sandboxNotFound(box.ID)
	}
	return cause
}

// snapshotGroup scopes a group verb, authorizes rel there and loads the group.
func (s *Service) snapshotGroup(ctx context.Context, groupID, rel string) (context.Context, store.SandboxGroup, error) {
	ctx, err := s.scopeGroup(ctx, groupID)
	if err != nil {
		return ctx, store.SandboxGroup{}, err
	}
	if err := s.Authorize(ctx, rel); err != nil {
		return ctx, store.SandboxGroup{}, err
	}
	if err := s.requireSnapshots(); err != nil {
		return ctx, store.SandboxGroup{}, err
	}
	g, err := s.storedGroup(ctx, groupID)
	return ctx, g, err
}

// GetSnapshot returns one snapshot of a group. A deleted or expired one, or
// one of another group, is not found, as on Render.
func (s *Service) GetSnapshot(ctx context.Context, groupID, snapshotID string) (Snapshot, error) {
	ctx, group, err := s.snapshotGroup(ctx, groupID, core.RelCanView)
	if err != nil {
		return Snapshot{}, err
	}
	if err := validateSnapshotID(snapshotID); err != nil {
		return Snapshot{}, err
	}
	snap, err := s.Snapshots.Store.SandboxSnapshot(ctx, group.WorkspaceID, group.ID, snapshotID, s.Now())
	if errors.Is(err, store.ErrNotFound) {
		return Snapshot{}, snapshotNotFound(snapshotID)
	}
	if err != nil {
		return Snapshot{}, store.MapError(err)
	}
	return snapshotView(snap), nil
}

// ListSnapshots lists a group's snapshots newest first, without the deleted
// and expired ones, filtered by name and status, one page after the cursor.
func (s *Service) ListSnapshots(ctx context.Context, groupID, snapshotName string, statuses []string, after string, limit int) ([]Snapshot, error) {
	ctx, group, err := s.snapshotGroup(ctx, groupID, core.RelCanView)
	if err != nil {
		return nil, err
	}
	if err := validateSnapshotStatuses(statuses); err != nil {
		return nil, err
	}
	rows, err := s.Snapshots.Store.SandboxSnapshots(ctx, group.WorkspaceID, group.ID, snapshotName, statuses, s.Now())
	if err != nil {
		return nil, store.MapError(err)
	}
	out := make([]Snapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, snapshotView(row))
	}
	return core.Page(out, after, core.PageLimitOrDefault(limit), func(sn Snapshot) string { return sn.ID }), nil
}

// validateSnapshotStatuses refuses a status filter value that is not a
// snapshot status, by name.
func validateSnapshotStatuses(statuses []string) error {
	for _, st := range statuses {
		if !slices.Contains(snapshotStatuses, st) {
			return core.NewBadRequestError(CodeInvalidSnapshotStatus,
				fmt.Sprintf("%q is not a snapshot status; use %s", st, strings.Join(snapshotStatuses, ", ")),
				map[string]any{"field": "status", "supported": snapshotStatuses})
		}
	}
	return nil
}

// snapshotTimedOutReason is the failure a capture past snapshotCaptureTimeout
// records; GraphQL reads it as Render's DEADLINE_EXCEEDED.
const snapshotTimedOutReason = "the capture did not finish in time"

// DeleteSnapshot deletes one snapshot of a group, as Render's
// delete-sandbox-snapshot: it leaves lists and reads, a create can no longer
// start from it, its registry image goes, and deleting a deleted or expired
// one succeeds. Sandboxes already restored from it are not affected: each has
// pulled the image, and a suspend commits its own. A capture still in flight
// is refused.
func (s *Service) DeleteSnapshot(ctx context.Context, groupID, snapshotID string) error {
	ctx, group, err := s.snapshotGroup(ctx, groupID, core.RelCanCreate)
	if err != nil {
		return err
	}
	if err := validateSnapshotID(snapshotID); err != nil {
		return err
	}
	snap, did, err := s.Snapshots.Store.DeleteSandboxSnapshot(ctx, group.WorkspaceID, group.ID, snapshotID, s.Now())
	if errors.Is(err, store.ErrNotFound) {
		return snapshotNotFound(snapshotID)
	}
	if err != nil {
		return store.MapError(err)
	}
	switch did {
	case store.SandboxSnapshotStillCreating:
		return core.NewConflictError(CodeSnapshotCreating,
			fmt.Sprintf("snapshot %s is still being captured; delete it once it is available or failed", snapshotID),
			map[string]any{"id": snapshotID, "status": snap.Status})
	case store.SandboxSnapshotGone:
		return nil // already deleted, or expired: Render answers 204 again
	}
	// The row is deleted; what it holds outside the database goes now, and the
	// sweep retries whatever fails here.
	reapCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), snapshotReapTimeout)
	defer cancel()
	if err := s.reapSnapshot(reapCtx, snap); err != nil && !errors.Is(err, errCaptureInFlight) {
		log.Printf("sandbox snapshots: reap deleted %s (the sweep retries): %v", snapshotID, err)
	}
	return nil
}

// restoreSource resolves a create's snapshotId or snapshotName, in the group
// the sandbox is created in, to the snapshot and the stored image the sandbox
// starts from; nil when the create names neither. A name is the group's most
// recently requested available snapshot of it, as on Render.
func (s *Service) restoreSource(ctx context.Context, workspace string, req CreateRequest) (*store.SandboxSnapshot, string, error) {
	if req.SnapshotID == "" && req.SnapshotName == "" {
		return nil, "", nil
	}
	if err := s.requireSnapshots(); err != nil {
		return nil, "", err
	}
	switch {
	case req.SnapshotID != "" && req.SnapshotName != "":
		return nil, "", core.NewBadRequestError(CodeSandboxSnapshotInvalid, "name a snapshot by snapshotId or by snapshotName, not both",
			map[string]any{"fields": []string{"snapshotId", "snapshotName"}})
	case req.BuildID != "" || req.BuildName != "":
		return nil, "", core.NewBadRequestError(CodeSandboxSnapshotInvalid, "start a sandbox from a snapshot or from an image, not both",
			map[string]any{"fields": []string{"snapshotId", "snapshotName", "buildId", "buildName"}})
	}
	group, err := s.resolveGroup(ctx, workspace, req.SandboxGroupID)
	if err != nil {
		return nil, "", err
	}
	now := s.Now()
	var snap store.SandboxSnapshot
	if req.SnapshotID != "" {
		if err := validateSnapshotID(req.SnapshotID); err != nil {
			return nil, "", err
		}
		snap, err = s.Snapshots.Store.SandboxSnapshot(ctx, workspace, group.ID, req.SnapshotID, now)
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", snapshotNotFound(req.SnapshotID)
		}
	} else {
		if err := validateSnapshotName(req.SnapshotName, "snapshotName"); err != nil {
			return nil, "", err
		}
		snap, err = s.Snapshots.Store.NewestAvailableSandboxSnapshot(ctx, workspace, group.ID, req.SnapshotName, now)
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", s.noAvailableSnapshot(ctx, workspace, group.ID, req.SnapshotName, now)
		}
	}
	if err != nil {
		return nil, "", store.MapError(err)
	}
	if snap.Status != store.SandboxSnapshotAvailable || snap.ImageDigest == nil {
		return nil, "", snapshotNotAvailable(snap)
	}
	repo, err := s.Images.repository(s.Images.Registry.snapshotRepository(workspace, snap.ID))
	if err != nil {
		return nil, "", err
	}
	return &snap, repo.Digest(*snap.ImageDigest).String(), nil
}

func snapshotNotAvailable(snap store.SandboxSnapshot) error {
	return core.NewConflictError(CodeSnapshotNotAvailable,
		fmt.Sprintf("snapshot %s is %s; a sandbox can start from it once it is available", snap.ID, snap.Status),
		map[string]any{"snapshotId": snap.ID, "status": snap.Status})
}

// noAvailableSnapshot is Render's answer for a snapshotName with no available
// snapshot: 409 snapshot_not_available when the group has snapshots of that
// name, and 404 snapshot_not_found when it has none.
func (s *Service) noAvailableSnapshot(ctx context.Context, workspace, groupID, name string, now time.Time) error {
	named, err := s.Snapshots.Store.SandboxSnapshots(ctx, workspace, groupID, name, nil, now)
	if err != nil {
		return store.MapError(err)
	}
	if len(named) == 0 {
		return core.NewNotFoundError(CodeSnapshotNotFound, fmt.Sprintf("sandbox group %s has no snapshot named %q", groupID, name),
			map[string]any{"snapshotName": name, "sandboxGroupId": groupID})
	}
	return snapshotNotAvailable(named[0])
}

// inheritFromSnapshot fills what a create from a snapshot left out with the
// source sandbox's values, as Render does: the plan, the network policy
// (allow-list rules included), the runtime timeout and the idle timeout, each
// overridable. Environment variables are never inherited.
func inheritFromSnapshot(req CreateRequest, snap store.SandboxSnapshot) (CreateRequest, error) {
	if req.Plan == "" {
		req.Plan = Plan(snap.Plan)
	}
	if req.TimeoutSeconds == nil {
		req.TimeoutSeconds = &snap.TimeoutSeconds
	}
	if req.IdleTimeoutSeconds == nil {
		req.IdleTimeoutSeconds = &snap.IdleTimeoutSeconds
	}
	if req.NetworkPolicy == nil && len(snap.NetworkPolicy) > 0 {
		var policy *NetworkPolicy
		if err := json.Unmarshal(snap.NetworkPolicy, &policy); err != nil {
			return req, fmt.Errorf("snapshot %s: its network policy is unreadable: %w", snap.ID, err)
		}
		req.NetworkPolicy = policy
	}
	return req, nil
}

// snapshotUsed restarts a snapshot's lifetime when a sandbox was created from
// it. A failure costs the snapshot one extension, never the create.
func (s *Service) snapshotUsed(ctx context.Context, snap *store.SandboxSnapshot) {
	if snap == nil {
		return
	}
	useCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordWriteTimeout)
	defer cancel()
	if err := s.Snapshots.Store.UseSandboxSnapshot(useCtx, snap.ID, s.Now()); err != nil {
		log.Printf("sandbox snapshots: extend %s after a restore: %v", snap.ID, err)
	}
}

// RunSnapshotSweeper settles the captures in flight and reaps the images of
// deleted and expired snapshots, until ctx ends.
func (s *Service) RunSnapshotSweeper(ctx context.Context) {
	if s.Snapshots == nil || s.Images == nil || !s.enabled() {
		return
	}
	core.Poll(ctx, "sandbox-snapshots", snapshotSweepEvery, s.sweepSnapshots)
}

// sweepSnapshots is one pass. Every step is idempotent, so several replicas
// may run it; a capture is settled by the one that holds its claim.
func (s *Service) sweepSnapshots(ctx context.Context) error {
	st := s.Snapshots.Store
	now := s.Now()
	var errs []error
	stale, err := st.StaleCreatingSandboxSnapshots(ctx, now.Add(-snapshotCaptureTimeout), snapshotTimedOutReason)
	if err != nil {
		errs = append(errs, err)
	}
	for _, snap := range stale {
		log.Printf("sandbox snapshots: failed %s, still creating after %s", snap.ID, snapshotCaptureTimeout)
	}
	claimed, err := st.ClaimCreatingSandboxSnapshots(ctx, now, snapshotClaimHold, snapshotSweepBatch)
	if err != nil {
		errs = append(errs, err)
	}
	for _, snap := range claimed {
		if err := s.settleSnapshot(ctx, snap); err != nil {
			errs = append(errs, fmt.Errorf("settle %s: %w", snap.ID, err))
		}
	}
	reapable, err := st.ReapableSandboxSnapshots(ctx, now, snapshotSweepBatch)
	if err != nil {
		errs = append(errs, err)
	}
	for _, snap := range reapable {
		if err := s.reapSnapshot(ctx, snap); err != nil && !errors.Is(err, errCaptureInFlight) {
			errs = append(errs, fmt.Errorf("reap %s: %w", snap.ID, err))
		}
	}
	return errors.Join(errs...)
}

// snapshotKey is the tenant key of a snapshot's workspace, for a call made
// outside a request.
func (s *Service) snapshotKey(ctx context.Context, workspace string) (string, error) {
	if s.Keys == nil {
		return "", nil
	}
	return s.Keys.WorkspaceKey(ctx, workspace)
}

// settleSnapshot moves one claimed capture on: it reads the substrate's
// state, and once the commit is pushed copies the image into the snapshot's
// own repository, opens its storage meter and marks it available. A capture
// still running, or a step that failed, goes back to the next sweep.
func (s *Service) settleSnapshot(ctx context.Context, snap store.SandboxSnapshot) error {
	st := s.Snapshots.Store
	release := func(err error) error {
		return errors.Join(err, st.ReleaseSandboxSnapshotClaim(ctx, snap.ID))
	}
	if snap.RuntimeID == nil {
		return release(nil) // the create is about to record it, or died: the stale sweep fails it
	}
	key, err := s.snapshotKey(ctx, snap.WorkspaceID)
	if err != nil {
		return release(err)
	}
	capture, err := s.Client.Snapshot(ctx, key, *snap.RuntimeID)
	switch {
	case errors.Is(err, errOpenSandboxNotFound):
		_, err := st.FailSandboxSnapshot(ctx, snap.ID, "the capture was lost before it finished")
		return err
	case err != nil:
		return release(err)
	}
	switch capture.Status.State {
	case osSnapshotFailed:
		log.Printf("sandbox snapshots: capture of %s failed: %s: %s", snap.ID, capture.Status.Reason, capture.Status.Message)
		if capture.Status.Reason == osSnapshotNotStarted {
			if again, err := s.recapture(ctx, key, snap); again || err != nil {
				return release(err)
			}
		}
		_, err := st.FailSandboxSnapshot(ctx, snap.ID, "the sandbox's filesystem could not be committed")
		return err
	case osSnapshotReady:
	default:
		return release(nil)
	}
	copyCtx, cancel := context.WithTimeout(ctx, snapshotCopyTimeout)
	defer cancel()
	digest, size, err := s.storeSnapshotImage(copyCtx, snap)
	var tooLarge *snapshotTooLargeError
	switch {
	case errors.As(err, &tooLarge):
		_, err := st.FailSandboxSnapshot(ctx, snap.ID, tooLarge.Error())
		return err
	case err != nil:
		return release(err)
	}
	// The meter opens before the snapshot reads `available`, so no available
	// snapshot is ever unmetered; a snapshot settled or gone meanwhile closes
	// it again and drops the copy.
	capturedAt := s.Now().UTC().Truncate(time.Second)
	if err := st.StartSandboxStorage(ctx, snap.WorkspaceID, snap.ID, store.StorageKindSnapshot, "", capturedAt); err != nil {
		return release(err)
	}
	if err := st.SetSandboxStorageBytes(ctx, snap.WorkspaceID, snap.ID, capturedAt, size); err != nil {
		return release(err)
	}
	settled, err := st.MarkSandboxSnapshotAvailable(ctx, snap.ID, digest, size, capturedAt)
	if err != nil {
		return release(err)
	}
	if !settled {
		return s.Snapshots.dropImage(ctx, snap, capturedAt)
	}
	return nil
}

// recapture requests a capture again in place of one the server could not
// start, while the snapshot has attempts left, and reports whether it did.
// false with no error means the capture is to fail: its attempts are spent, or
// the sandbox is no longer running. An error leaves the failed capture
// recorded, so the next sweep tries again with the attempts that remain.
func (s *Service) recapture(ctx context.Context, key string, snap store.SandboxSnapshot) (bool, error) {
	st := s.Snapshots.Store
	attempt, err := st.TakeSandboxSnapshotCaptureAttempt(ctx, snap.ID, snapshotCaptureAttempts)
	if err != nil || attempt == 0 {
		return false, err
	}
	next, err := s.Client.CreateSnapshot(ctx, key, snap.SourceRef, snap.ID)
	switch {
	case errors.Is(err, errOpenSandboxConflict), errors.Is(err, errOpenSandboxNotFound):
		return false, nil
	case err != nil:
		return true, err
	}
	log.Printf("sandbox snapshots: capture of %s requested again as %s (attempt %d of %d)", snap.ID, next.ID, attempt, snapshotCaptureAttempts)
	if _, err := st.SetSandboxSnapshotRuntimeID(ctx, snap.ID, next.ID); err != nil {
		// The row still names the failed capture, deleted below, so the next
		// sweep fails it as lost rather than start a second commit of the
		// sandbox while this one runs unrecorded, as in CreateSnapshot.
		log.Printf("sandbox snapshots: record the capture %s of %s: %v", next.ID, snap.ID, err)
	}
	// The failed capture made nothing but the server's record of it.
	if err := s.Client.DeleteSnapshot(ctx, key, *snap.RuntimeID); err != nil {
		log.Printf("sandbox snapshots: delete the failed capture %s of %s: %v", *snap.RuntimeID, snap.ID, err)
	}
	return true, nil
}

// snapshotTooLargeError is a capture whose image is past the size cap.
type snapshotTooLargeError struct{ size, max int64 }

func (e *snapshotTooLargeError) Error() string {
	return fmt.Sprintf("the sandbox's filesystem is %s, more than the %s a snapshot may hold", formatGB(e.size), formatGB(e.max))
}

// storeSnapshotImage copies a capture's committed image from its source
// sandbox's repository into the snapshot's own and returns its digest and
// compressed size. Its layers are mounted, not uploaded again: they are the
// ones the substrate's commit wrote into this workspace's own snapshot
// namespace, so no digest here names another workspace's blob. The copy is
// what keeps the snapshot: the registry keeps only a repository's newest
// tags, and the source repository gets one more at every suspend.
func (s *Service) storeSnapshotImage(ctx context.Context, snap store.SandboxSnapshot) (string, int64, error) {
	im := s.Images
	sourcePath, err := im.Registry.repository(snap.WorkspaceID, snap.SourceRef)
	if err != nil {
		return "", 0, err
	}
	source, err := im.repository(sourcePath)
	if err != nil {
		return "", 0, err
	}
	target, err := im.repository(im.Registry.snapshotRepository(snap.WorkspaceID, snap.ID))
	if err != nil {
		return "", 0, err
	}
	opts, err := im.registryOptions(ctx)
	if err != nil {
		return "", 0, err
	}
	img, err := remote.Image(source.Tag(commitTag(*snap.RuntimeID)), opts...)
	if err != nil {
		return "", 0, fmt.Errorf("read the committed image: %w", err)
	}
	size, err := imageBytes(img)
	if err != nil {
		return "", 0, fmt.Errorf("size the committed image: %w", err)
	}
	if limit := s.Snapshots.maxBytes(); size > limit {
		return "", 0, &snapshotTooLargeError{size: size, max: limit}
	}
	if err := remote.Write(target.Tag(snapshotImageTag), img, opts...); err != nil {
		return "", 0, fmt.Errorf("copy the committed image: %w", err)
	}
	digest, err := img.Digest()
	if err != nil {
		return "", 0, err
	}
	return digest.String(), size, nil
}

// dropImage closes a snapshot's storage meter at endedAt and deletes its image
// from the registry. Both are idempotent.
func (sn *Snapshots) dropImage(ctx context.Context, snap store.SandboxSnapshot, endedAt time.Time) error {
	if err := sn.Store.EndSandboxStorage(ctx, snap.WorkspaceID, snap.ID, endedAt); err != nil {
		return err
	}
	if sn.Reaper == nil {
		return nil
	}
	return sn.Reaper.DeleteSnapshotImage(ctx, snap.WorkspaceID, snap.ID)
}

// reapImage drops the image of a snapshot that is deleted, or expired at now,
// and records it.
func (sn *Snapshots) reapImage(ctx context.Context, snap store.SandboxSnapshot, now time.Time) error {
	ended := snap.DeletedAt
	if ended == nil && snap.ExpiresAt != nil && !snap.ExpiresAt.After(now) {
		ended = snap.ExpiresAt
	}
	if ended == nil || snap.ImageDigest == nil || snap.ReapedAt != nil {
		return nil
	}
	if err := sn.dropImage(ctx, snap, *ended); err != nil {
		return err
	}
	return sn.Store.MarkSandboxSnapshotReaped(ctx, snap.ID, now)
}

// purgeWorkspace deletes the images of every snapshot of a deleted workspace
// while their rows still name them. The captures' own objects go with the
// workspace's sandboxes and namespace, and a tag a capture left in a
// sandbox's repository is the orphan sweep's.
func (sn *Snapshots) purgeWorkspace(ctx context.Context, workspaceID string, now time.Time) error {
	snaps, err := sn.Store.EndGroupSandboxSnapshots(ctx, workspaceID, "", "its workspace was deleted", now)
	if err != nil {
		return err
	}
	var errs []error
	for _, snap := range snaps {
		if err := sn.reapImage(ctx, snap, now); err != nil {
			errs = append(errs, fmt.Errorf("delete the image of snapshot %s: %w", snap.ID, err))
		}
	}
	return errors.Join(errs...)
}

// errCaptureInFlight is a capture the substrate is still working on, whose
// record it will not delete yet.
var errCaptureInFlight = errors.New("the snapshot's capture is still in flight")

// reapSnapshot removes what a settled snapshot still holds outside the
// database: the substrate's object of its capture with the tag that capture
// pushed, and, once the snapshot is deleted or expired, its image and its
// open storage meter. Each step is idempotent; a failure, or a capture the
// substrate has not finished, leaves the rest for the next pass.
func (s *Service) reapSnapshot(ctx context.Context, snap store.SandboxSnapshot) error {
	var captureErr error
	if snap.RuntimeID != nil {
		captureErr = s.cleanCapture(ctx, snap)
	}
	return errors.Join(captureErr, s.Snapshots.reapImage(ctx, snap, s.Now()))
}

// cleanCapture deletes the substrate's record of a settled capture (with the
// SandboxSnapshot object and the commit Jobs it owns) and the tag the commit
// pushed into the source sandbox's repository.
func (s *Service) cleanCapture(ctx context.Context, snap store.SandboxSnapshot) error {
	key, err := s.snapshotKey(ctx, snap.WorkspaceID)
	if err != nil {
		return err
	}
	if err := s.Client.DeleteSnapshot(ctx, key, *snap.RuntimeID); err != nil {
		if errors.Is(err, errOpenSandboxConflict) {
			return errCaptureInFlight
		}
		return err
	}
	if s.Snapshots.Reaper != nil {
		if err := s.Snapshots.Reaper.DeleteCommitTag(ctx, snap.WorkspaceID, snap.SourceRef, commitTag(*snap.RuntimeID)); err != nil {
			return err
		}
	}
	return s.Snapshots.Store.ClearSandboxSnapshotRuntimeID(ctx, snap.ID)
}

// endGroupSnapshots deletes every snapshot of a deleted group with its image,
// its meter and its capture. A capture the substrate has not finished is an
// error, so the group's delete is retried until nothing of it is left behind.
func (s *Service) endGroupSnapshots(ctx context.Context, g store.SandboxGroup) error {
	if s.Snapshots == nil {
		return nil
	}
	snaps, err := s.Snapshots.Store.EndGroupSandboxSnapshots(ctx, g.WorkspaceID, g.ID, "its sandbox group was deleted", s.Now())
	if err != nil {
		return err
	}
	var errs []error
	for _, snap := range snaps {
		if err := s.reapSnapshot(ctx, snap); err != nil {
			errs = append(errs, fmt.Errorf("reap snapshot %s: %w", snap.ID, err))
		}
	}
	return errors.Join(errs...)
}
