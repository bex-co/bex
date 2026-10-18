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

// preparePatchOperation persists, before the busy claim, the prior and
// proposed map of each side the save changes, so a crash during the later
// writes leaves another instance enough to restore or finish. Returns the
// operation id the claim records.
func (s *Service) preparePatchOperation(ctx context.Context, op store.EnvGroupOp, workspace, gid string, prior, next groupContents) (string, error) {
	opID, err := newGroupOpID()
	if err != nil {
		return "", err
	}
	writes := map[string]map[string]string{}
	if op.EnvChanged {
		writes[opOldEnvPath(gid, opID)], writes[opNewEnvPath(gid, opID)] = prior.env, next.env
	}
	if op.FilesChanged {
		writes[opOldFilesPath(gid, opID)], writes[opNewFilesPath(gid, opID)] = prior.files, next.files
	}
	for path, data := range writes {
		if err := s.putGroupMap(ctx, workspace, path, data); err != nil {
			_ = s.clearOpMaps(context.WithoutCancel(ctx), workspace, gid, opID)
			return "", err
		}
	}
	return opID, nil
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

// releaseContent ends the claim held at claimVersion, even for a caller that
// has gone away.
func (s *Service) releaseContent(ctx context.Context, gid string, claimVersion int64, rel store.EnvGroupRelease) error {
	return s.Groups.ReleaseEnvGroupContent(context.WithoutCancel(ctx), gid, claimVersion, rel)
}

// commit is the release of a save that changed content to contents.
func (s *Service) commit(contents groupContents) store.EnvGroupRelease {
	return store.EnvGroupRelease{Commit: &store.EnvGroupCommit{
		EnvKeys: slices.Sorted(maps.Keys(contents.env)), FileNames: slices.Sorted(maps.Keys(contents.files)),
		UpdatedAt: s.nowTime(),
	}}
}

// operableGroup returns a fresh idle row for a writer: g itself when idle,
// otherwise after offering it to recovery. A group still held, or stranded,
// answers its conflict.
func (s *Service) operableGroup(ctx context.Context, g store.EnvGroup) (store.EnvGroup, error) {
	if g.Content.State == store.EnvGroupIdle {
		return g, nil
	}
	if err := s.ensureGroupOperable(ctx, g); err != nil {
		return store.EnvGroup{}, err
	}
	fresh, err := s.readGroup(ctx, g.ID)
	if err != nil {
		return store.EnvGroup{}, err
	}
	switch fresh.Content.State {
	case store.EnvGroupIdle:
		return fresh, nil
	case store.EnvGroupRepairRequired:
		return store.EnvGroup{}, envGroupRestorationFailed()
	}
	return store.EnvGroup{}, envGroupRevisionConflict() // another save claimed it meanwhile
}

// ensureGroupOperable recovers an expired busy or a stranded repair_required
// group when its recorded operation allows, then reports whether the group may
// accept a new writer. An active lease is left alone.
func (s *Service) ensureGroupOperable(ctx context.Context, g store.EnvGroup) error {
	op := g.Content.Op
	switch {
	case g.Content.State == store.EnvGroupIdle:
		return nil
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
	if err := contentErr(s.releaseContent(ctx, g.ID, g.Content.Version, rel), envGroupRestorationFailed()); err != nil {
		return err
	}
	_ = s.clearOpMaps(ctx, g.TenantID, g.ID, g.Content.Op.ID)
	return nil
}
