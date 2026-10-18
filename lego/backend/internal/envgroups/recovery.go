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

package envgroups

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Operation kinds and phases of an interrupted content save or clone source
// lock (w4/m98). The operation is recorded on the group's row together with
// its busy claim; the prior and proposed maps it needs to restore or finish
// are kept in OpenBao beside the group's own maps, since they are values.
const (
	opKindPatch = "patch"
	opKindClone = "clone"

	opPhaseAdmitted     = "admitted"      // busy claimed; no live map mutation yet
	opPhaseEnvWritten   = "env_written"   // live env (+ projection) may be new
	opPhaseFilesWritten = "files_written" // live files (+ projection) may be new; treat as commit
)

// Default operation lease. A different instance may reclaim only after this
// wall-clock bound elapses; clearing busy solely because a timer fired without
// restoring or finalizing from durable evidence is not recovery.
const defaultEnvGroupOpLease = 2 * time.Minute

const groupCleanupTimeout = 30 * time.Second

type groupCleanupKey struct{}

// Detached cleanup gets one deadline shared by compensation, row release and
// value-map deletion. Nested helpers preserve the budget rather than renewing it.
func groupCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Value(groupCleanupKey{}) != nil {
		return ctx, func() {}
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), groupCleanupTimeout)
	return context.WithValue(cleanup, groupCleanupKey{}, true), cancel
}

func opDir(gid string) string { return "env-groups/" + gid + "/op" }

func opOldEnvPath(gid, opID string) string   { return opDir(gid) + "/" + opID + "/old-env" }
func opOldFilesPath(gid, opID string) string { return opDir(gid) + "/" + opID + "/old-files" }
func opNewEnvPath(gid, opID string) string   { return opDir(gid) + "/" + opID + "/new-env" }
func opNewFilesPath(gid, opID string) string { return opDir(gid) + "/" + opID + "/new-files" }

// groupContents is a group's two maps.
type groupContents struct{ env, files map[string]string }

func newGroupOpID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "ego-" + hex.EncodeToString(b[:]), nil
}

func (s *Service) opLease() time.Duration {
	if s.OpLease > 0 {
		return s.OpLease
	}
	return defaultEnvGroupOpLease
}

// clearOpMaps removes one operation's recovery maps.
func (s *Service) clearOpMaps(ctx context.Context, workspace, gid, opID string) error {
	var errs []error
	for _, path := range []string{
		opOldEnvPath(gid, opID), opOldFilesPath(gid, opID), opNewEnvPath(gid, opID), opNewFilesPath(gid, opID),
	} {
		errs = append(errs, s.deleteGroupPath(ctx, workspace, path))
	}
	return errors.Join(errs...)
}

// preparePatchOperation persists, after the busy claim, the prior and
// proposed map of each side the save changes, so a crash during the later
// writes leaves another instance enough to restore or finish. A crash while
// preparing remains admitted and only
// needs cleanup; no live map can have changed yet.
func (s *Service) preparePatchOperation(ctx context.Context, op store.EnvGroupOp, workspace, gid string, prior, next groupContents) error {
	opID := op.ID
	writes := map[string]map[string]string{}
	if op.EnvChanged {
		writes[opOldEnvPath(gid, opID)], writes[opNewEnvPath(gid, opID)] = prior.env, next.env
	}
	if op.FilesChanged {
		writes[opOldFilesPath(gid, opID)], writes[opNewFilesPath(gid, opID)] = prior.files, next.files
	}
	for path, data := range writes {
		if err := s.putGroupMap(ctx, workspace, path, data); err != nil {
			return err
		}
	}
	return nil
}

// contentErr maps a content claim, advance or release error: a lost race is
// ENV_GROUP_REVISION_CONFLICT, a missing group core.ErrNotFound, and anything
// else fallback.
func contentErr(err, fallback error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrEnvGroupChanged):
		return envGroupRevisionConflict()
	case errors.Is(err, store.ErrNotFound):
		return core.ErrNotFound
	}
	return fallback
}

// claimContent marks the group busy for one operation at the content version
// the caller read.
func (s *Service) claimContent(ctx context.Context, g store.EnvGroup, op store.EnvGroupOp) (int64, error) {
	op.Phase, op.LeaseUntil = opPhaseAdmitted, s.nowTime().Add(s.opLease())
	version, err := s.Groups.ClaimEnvGroupContent(ctx, g.ID, g.Content.Version, op)
	return version, contentErr(err, core.ErrSecretsUnavailable)
}

func (s *Service) advanceOpPhase(ctx context.Context, gid, opID, phase string) error {
	err := s.Groups.AdvanceEnvGroupOp(ctx, gid, opID, phase, s.nowTime().Add(s.opLease()))
	return contentErr(err, core.ErrSecretsUnavailable)
}

// releaseContent uses the bounded cleanup context its caller owns.
func (s *Service) releaseContent(ctx context.Context, gid string, claimVersion int64, rel store.EnvGroupRelease) error {
	return s.Groups.ReleaseEnvGroupContent(ctx, gid, claimVersion, rel)
}

// commit is the release of a save that changed content to contents.
func (s *Service) commit(contents groupContents) store.EnvGroupRelease {
	return store.EnvGroupRelease{Commit: &store.EnvGroupCommit{
		EnvKeys: slices.Sorted(maps.Keys(contents.env)), FileNames: slices.Sorted(maps.Keys(contents.files)),
		UpdatedAt: s.nowTime(),
	}}
}

func (s *Service) ensureGroupOperableLocked(ctx context.Context, g store.EnvGroup) error {
	op := g.Content.Op
	switch {
	case op == nil:
		// repair_required without recovery evidence — fail honestly.
		return envGroupRestorationFailed()
	case g.Content.State == store.EnvGroupBusy && s.nowTime().Before(op.LeaseUntil):
		return envGroupRevisionConflict()
	case op.Kind == opKindClone:
		// Clone never mutates source contents — release the source lock.
		return s.finishRecovery(ctx, g, store.EnvGroupRelease{})
	case op.Kind == opKindPatch:
		return s.recoverPatchOperation(ctx, g)
	}
	return envGroupRestorationFailed()
}

// withGroupContent acquires the cross-instance side-effect lock before reading
// the row, recovering an expired operation, or touching either live map. A row
// read before the lock can no longer authorize a stale writer or recoverer.
func (s *Service) withGroupContent(ctx context.Context, gid string, fn func(context.Context, store.EnvGroup) error) error {
	if err := s.requireGroups(ctx); err != nil {
		return err
	}
	err := s.Groups.WithEnvGroupContentLock(ctx, gid, func(ctx context.Context) error {
		g, err := s.readGroup(ctx, gid)
		if err != nil {
			return err
		}
		if g.Content.State != store.EnvGroupIdle {
			if err := s.ensureGroupOperableLocked(ctx, g); err != nil {
				return err
			}
			g, err = s.readGroup(ctx, gid)
			if err != nil {
				return err
			}
		}
		if g.Content.Op != nil {
			if err := s.cleanupIdleOperation(ctx, g); err != nil {
				return err
			}
			g, err = s.readGroup(ctx, gid)
			if err != nil {
				return err
			}
		}
		return fn(ctx, g)
	})
	return contentErr(err, err)
}

// recoverPatchOperation restores or finishes an interrupted save from the
// phase it reached.
func (s *Service) recoverPatchOperation(ctx context.Context, g store.EnvGroup) error {
	op, workspace, gid := g.Content.Op, g.TenantID, g.ID
	switch op.Phase {
	case opPhaseAdmitted:
		// Busy claimed, no live map mutation yet — drop the claim.
		return s.finishRecovery(ctx, g, store.EnvGroupRelease{})
	case opPhaseEnvWritten:
		// Env may be new while files are still old — restore prior env.
		if op.EnvChanged {
			prior, err := s.getGroupMap(ctx, workspace, opOldEnvPath(gid, op.ID))
			if err != nil || s.writeSlot(ctx, workspace, gid, envSlot, prior) != nil {
				return envGroupRestorationFailed()
			}
		}
		return s.finishRecovery(ctx, g, store.EnvGroupRelease{})
	case opPhaseFilesWritten:
		// Maps reflect the acknowledged new configuration — finalize. A side
		// the save left alone keeps its committed names.
		if !op.EnvChanged && !op.FilesChanged {
			return s.finishRecovery(ctx, g, store.EnvGroupRelease{})
		}
		release := s.commit(groupContents{})
		release.Commit.EnvKeys, release.Commit.FileNames = g.EnvKeys, g.FileNames
		for _, side := range []struct {
			changed bool
			slot    contentSlot
			path    string
			names   *[]string
		}{
			{op.EnvChanged, envSlot, opNewEnvPath(gid, op.ID), &release.Commit.EnvKeys},
			{op.FilesChanged, filesSlot, opNewFilesPath(gid, op.ID), &release.Commit.FileNames},
		} {
			if !side.changed {
				continue
			}
			next, err := s.getGroupMap(ctx, workspace, side.path)
			if err != nil || s.writeSlot(ctx, workspace, gid, side.slot, next) != nil {
				return envGroupRestorationFailed()
			}
			*side.names = slices.Sorted(maps.Keys(next))
		}
		return s.finishRecovery(ctx, g, release)
	}
	return envGroupRestorationFailed()
}

// finishRecovery releases a recovered group at the version the recoverer read,
// then removes the operation's recovery maps.
func (s *Service) finishRecovery(ctx context.Context, g store.EnvGroup, rel store.EnvGroupRelease) error {
	ctx, cancel := groupCleanupContext(ctx)
	defer cancel()
	rel.RetainOperation = true
	if err := contentErr(s.releaseContent(ctx, g.ID, g.Content.Version, rel), envGroupRestorationFailed()); err != nil {
		return err
	}
	s.tryOperationCleanup(ctx, g.ID)
	return nil
}

// cleanupIdleOperation never changes committed contents or generation. An idle
// row retaining its op already committed or restored; only its recovery maps
// remain. Keep that evidence until every delete succeeds so another instance
// can retry, and no later writer replaces an uncleared operation.
func (s *Service) cleanupIdleOperation(ctx context.Context, g store.EnvGroup) error {
	if g.Content.State != store.EnvGroupIdle {
		return envGroupRevisionConflict()
	}
	if g.Content.Op == nil {
		return nil
	}
	ctx, cancel := groupCleanupContext(ctx)
	defer cancel()
	if err := s.clearOpMaps(ctx, g.TenantID, g.ID, g.Content.Op.ID); err != nil {
		return core.ErrSecretsUnavailable
	}
	return contentErr(s.releaseContent(ctx, g.ID, g.Content.Version, store.EnvGroupRelease{}), core.ErrSecretsUnavailable)
}

// Content already committed successfully. Cleanup failure leaves the row's op
// as its retry anchor, without turning a committed save into a failed save.
func (s *Service) tryOperationCleanup(ctx context.Context, gid string) {
	ctx, cancel := groupCleanupContext(ctx)
	defer cancel()
	if g, err := s.readGroup(ctx, gid); err == nil {
		_ = s.cleanupIdleOperation(ctx, g)
	}
}
