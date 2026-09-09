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
	"fmt"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Operation record phases for an interrupted content save or clone source lock
// (w4/m98). Values are stored in OpenBao under env-groups/<gid>/op (non-secret)
// with prior/proposed maps beside them so another API instance can recover.
const (
	opKindPatch = "patch"
	opKindClone = "clone"

	opPhaseAdmitted     = "admitted"      // recovery written; busy claimed; no live map mutation yet
	opPhaseEnvWritten   = "env_written"   // live env (+ projection) may be new
	opPhaseFilesWritten = "files_written" // live files (+ projection) may be new; treat as commit
	opPhaseCommitted    = "committed"     // maps done; revision release still pending
)

// Default operation lease. A different instance may reclaim only after this
// wall-clock bound elapses; clearing busy solely because a timer fired without
// restoring or finalizing from durable evidence is not recovery.
const defaultEnvGroupOpLease = 2 * time.Minute

func opRecordPath(gid string) string { return "env-groups/" + gid + "/op" }
func opOldEnvPath(gid, opID string) string {
	return "env-groups/" + gid + "/op/" + opID + "/old-env"
}
func opOldFilesPath(gid, opID string) string {
	return "env-groups/" + gid + "/op/" + opID + "/old-files"
}
func opNewEnvPath(gid, opID string) string {
	return "env-groups/" + gid + "/op/" + opID + "/new-env"
}
func opNewFilesPath(gid, opID string) string {
	return "env-groups/" + gid + "/op/" + opID + "/new-files"
}

type groupOpRecord struct {
	id          string
	kind        string
	phase       string
	generation  uint64
	leaseUntil  time.Time
	envChanged  bool
	filesChanged bool
}

func encodeOpRecord(op groupOpRecord) map[string]string {
	data := map[string]string{
		"id":         op.id,
		"kind":       op.kind,
		"phase":      op.phase,
		"generation": strconv.FormatUint(op.generation, 10),
		"leaseUntil": op.leaseUntil.UTC().Format(time.RFC3339Nano),
	}
	if op.envChanged {
		data["envChanged"] = "1"
	}
	if op.filesChanged {
		data["filesChanged"] = "1"
	}
	return data
}

func decodeOpRecord(raw map[string]string) (groupOpRecord, error) {
	if raw["id"] == "" || raw["kind"] == "" || raw["phase"] == "" {
		return groupOpRecord{}, fmt.Errorf("incomplete operation record")
	}
	generation, _ := strconv.ParseUint(raw["generation"], 10, 64)
	leaseUntil, err := time.Parse(time.RFC3339Nano, raw["leaseUntil"])
	if err != nil {
		leaseUntil, err = time.Parse(time.RFC3339, raw["leaseUntil"])
		if err != nil {
			return groupOpRecord{}, fmt.Errorf("invalid leaseUntil")
		}
	}
	return groupOpRecord{
		id:           raw["id"],
		kind:         raw["kind"],
		phase:        raw["phase"],
		generation:   generation,
		leaseUntil:   leaseUntil,
		envChanged:   raw["envChanged"] == "1",
		filesChanged: raw["filesChanged"] == "1",
	}, nil
}

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

func (s *Service) writeOpRecord(ctx context.Context, workspace, gid string, op groupOpRecord) error {
	return s.putGroupMap(ctx, workspace, opRecordPath(gid), encodeOpRecord(op))
}

func (s *Service) readOpRecord(ctx context.Context, workspace, gid string) (groupOpRecord, bool, error) {
	raw, err := s.getGroupMap(ctx, workspace, opRecordPath(gid))
	if err != nil {
		return groupOpRecord{}, false, err
	}
	if len(raw) == 0 {
		return groupOpRecord{}, false, nil
	}
	op, err := decodeOpRecord(raw)
	if err != nil {
		return groupOpRecord{}, false, err
	}
	return op, true, nil
}

func (s *Service) clearOpArtifacts(ctx context.Context, workspace, gid, opID string) error {
	var errs []error
	if opID != "" {
		for _, path := range []string{
			opOldEnvPath(gid, opID), opOldFilesPath(gid, opID), opNewEnvPath(gid, opID), opNewFilesPath(gid, opID),
		} {
			errs = append(errs, s.deleteGroupPath(ctx, workspace, path))
		}
	}
	op, ok, err := s.readOpRecord(ctx, workspace, gid)
	if err != nil {
		errs = append(errs, err)
	} else if ok && (opID == "" || op.id == opID) {
		errs = append(errs, s.deleteGroupPath(ctx, workspace, opRecordPath(gid)))
	}
	return errors.Join(errs...)
}

// preparePatchOperation persists recovery evidence before the busy claim so a
// crash during later writes has enough state for another instance to restore or
// finalize. Returns the operation id embedded in the revision claim.
func (s *Service) preparePatchOperation(ctx context.Context, workspace, gid string, generation uint64, oldEnv, oldFiles, newEnv, newFiles map[string]string, envChanged, filesChanged bool) (string, error) {
	opID, err := newGroupOpID()
	if err != nil {
		return "", err
	}
	_ = generation
	_ = envChanged
	_ = filesChanged
	if err := s.putGroupMap(ctx, workspace, opOldEnvPath(gid, opID), oldEnv); err != nil {
		return "", err
	}
	if err := s.putGroupMap(ctx, workspace, opOldFilesPath(gid, opID), oldFiles); err != nil {
		_ = s.clearOpArtifacts(ctx, workspace, gid, opID)
		return "", err
	}
	if err := s.putGroupMap(ctx, workspace, opNewEnvPath(gid, opID), newEnv); err != nil {
		_ = s.clearOpArtifacts(ctx, workspace, gid, opID)
		return "", err
	}
	if err := s.putGroupMap(ctx, workspace, opNewFilesPath(gid, opID), newFiles); err != nil {
		_ = s.clearOpArtifacts(ctx, workspace, gid, opID)
		return "", err
	}
	// Defer the shared op record until after the busy claim wins so concurrent
	// prepares cannot overwrite each other's ownership pointer.
	return opID, nil
}

// commitOpRecord publishes the shared operation pointer after the busy claim
// succeeds. Callers that lose the claim must clear only their opID-scoped paths.
func (s *Service) commitOpRecord(ctx context.Context, workspace, gid string, op groupOpRecord) error {
	return s.writeOpRecord(ctx, workspace, gid, op)
}

func (s *Service) advanceOpPhase(ctx context.Context, workspace, gid, opID, phase string) error {
	op, ok, err := s.readOpRecord(ctx, workspace, gid)
	if err != nil {
		return err
	}
	if !ok || op.id != opID {
		return envGroupMetadataConflict()
	}
	op.phase = phase
	op.leaseUntil = s.Now().UTC().Add(s.opLease())
	return s.writeOpRecord(ctx, workspace, gid, op)
}

func (s *Service) prepareCloneOperation(ctx context.Context, workspace, gid string, generation uint64) (string, error) {
	opID, err := newGroupOpID()
	if err != nil {
		return "", err
	}
	_ = workspace
	_ = gid
	_ = generation
	return opID, nil
}

func revisionClaimData(state string, generation uint64, opID string) map[string]string {
	data := map[string]string{
		"state": state, "generation": strconv.FormatUint(generation, 10),
	}
	if opID != "" {
		data["op"] = opID
	}
	return data
}

// ensureGroupOperable recovers an expired or orphaned busy/repair_required
// revision when durable evidence allows, then reports whether the group may
// accept a new writer. Active leases are left alone.
func (s *Service) ensureGroupOperable(ctx context.Context, workspace, gid string) error {
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return core.ErrSecretsUnavailable
	}
	snapshot, err := s.getRevisionSnapshot(ctx, versioned, workspace, gid)
	if err != nil {
		return core.ErrSecretsUnavailable
	}
	state := snapshot.Data["state"]
	if state == "" || state == "idle" {
		return nil
	}
	op, hasOp, err := s.readOpRecord(ctx, workspace, gid)
	if err != nil {
		return err
	}
	if state == "busy" && hasOp && s.Now().UTC().Before(op.leaseUntil) {
		return envGroupRevisionConflict()
	}
	if !hasOp {
		// Legacy busy/repair without recovery evidence — fail honestly.
		if state == "repair_required" {
			return envGroupRestorationFailed()
		}
		return envGroupRevisionConflict()
	}
	return s.recoverGroupOperation(ctx, workspace, gid, versioned, snapshot, op)
}

func (s *Service) recoverGroupOperation(ctx context.Context, workspace, gid string, versioned core.VersionedSecretKV, snapshot core.SecretKVSnapshot, op groupOpRecord) error {
	switch op.kind {
	case opKindClone:
		return s.recoverCloneOperation(ctx, workspace, gid, versioned, snapshot, op)
	case opKindPatch:
		return s.recoverPatchOperation(ctx, workspace, gid, versioned, snapshot, op)
	default:
		return envGroupRestorationFailed()
	}
}

func (s *Service) recoverCloneOperation(ctx context.Context, workspace, gid string, versioned core.VersionedSecretKV, snapshot core.SecretKVSnapshot, op groupOpRecord) error {
	// Clone never mutates source contents — release the source lock and clear
	// the operation record.
	if _, err := versioned.PutCAS(groupCtx(ctx, workspace), revisionPath(gid), revisionClaimData("idle", op.generation, ""), snapshot.Version); err != nil {
		if errors.Is(err, core.ErrConflict) {
			return envGroupRevisionConflict()
		}
		return envGroupRestorationFailed()
	}
	_ = s.clearOpArtifacts(ctx, workspace, gid, op.id)
	return nil
}

func (s *Service) recoverPatchOperation(ctx context.Context, workspace, gid string, versioned core.VersionedSecretKV, snapshot core.SecretKVSnapshot, op groupOpRecord) error {
	switch op.phase {
	case opPhaseAdmitted:
		// Busy claimed, no live map mutation yet — drop the claim.
		if _, err := versioned.PutCAS(groupCtx(ctx, workspace), revisionPath(gid), revisionClaimData("idle", op.generation, ""), snapshot.Version); err != nil {
			if errors.Is(err, core.ErrConflict) {
				return envGroupRevisionConflict()
			}
			return envGroupRestorationFailed()
		}
		_ = s.clearOpArtifacts(ctx, workspace, gid, op.id)
		return nil
	case opPhaseEnvWritten:
		// Env may be new while files are still old — restore prior env.
		if op.envChanged {
			oldEnv, err := s.getGroupMap(ctx, workspace, opOldEnvPath(gid, op.id))
			if err != nil {
				return envGroupRestorationFailed()
			}
			if err := s.storeMap(ctx, workspace, envPath(gid), oldEnv); err != nil {
				return envGroupRestorationFailed()
			}
			if err := s.upsertSecret(ctx, workspace, envSecretName(gid), oldEnv); err != nil {
				return envGroupRestorationFailed()
			}
		}
		if _, err := versioned.PutCAS(groupCtx(ctx, workspace), revisionPath(gid), revisionClaimData("idle", op.generation, ""), snapshot.Version); err != nil {
			if errors.Is(err, core.ErrConflict) {
				return envGroupRevisionConflict()
			}
			return envGroupRestorationFailed()
		}
		_ = s.clearOpArtifacts(ctx, workspace, gid, op.id)
		return nil
	case opPhaseFilesWritten, opPhaseCommitted:
		// Maps reflect the acknowledged new configuration — finalize.
		newGeneration := op.generation
		if op.envChanged || op.filesChanged {
			newGeneration++
		}
		if op.envChanged {
			newEnv, err := s.getGroupMap(ctx, workspace, opNewEnvPath(gid, op.id))
			if err != nil {
				return envGroupRestorationFailed()
			}
			if err := s.storeMap(ctx, workspace, envPath(gid), newEnv); err != nil {
				return envGroupRestorationFailed()
			}
			if err := s.upsertSecret(ctx, workspace, envSecretName(gid), newEnv); err != nil {
				return envGroupRestorationFailed()
			}
		}
		if op.filesChanged {
			newFiles, err := s.getGroupMap(ctx, workspace, opNewFilesPath(gid, op.id))
			if err != nil {
				return envGroupRestorationFailed()
			}
			if err := s.storeMap(ctx, workspace, filesPath(gid), newFiles); err != nil {
				return envGroupRestorationFailed()
			}
			if err := s.upsertSecret(ctx, workspace, filesSecretName(gid), newFiles); err != nil {
				return envGroupRestorationFailed()
			}
		}
		if _, err := s.mutateMetaCAS(ctx, gid, workspace, func(cur meta) (meta, error) {
			cur.updatedAt = s.now()
			return cur, nil
		}); err != nil && !errors.Is(err, core.ErrNotFound) {
			return envGroupRestorationFailed()
		}
		if _, err := versioned.PutCAS(groupCtx(ctx, workspace), revisionPath(gid), revisionClaimData("idle", newGeneration, ""), snapshot.Version); err != nil {
			if errors.Is(err, core.ErrConflict) {
				return envGroupRevisionConflict()
			}
			return envGroupRestorationFailed()
		}
		_ = s.clearOpArtifacts(ctx, workspace, gid, op.id)
		return nil
	default:
		return envGroupRestorationFailed()
	}
}
