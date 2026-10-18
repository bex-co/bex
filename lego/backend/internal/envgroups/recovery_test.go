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
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// strand leaves group gid exactly as a save that crashed in phase would: its
// recovery maps written, its row claimed busy for the operation, then the
// process gone. It returns the operation id.
func strand(t *testing.T, svc *Service, gid, kind, phase string, oldEnv, newEnv map[string]string) string {
	t.Helper()
	ctx := testCtx()
	g, err := svc.readGroup(ctx, gid)
	if err != nil {
		t.Fatal(err)
	}
	op := store.EnvGroupOp{Kind: kind, EnvChanged: !mapsEqual(oldEnv, newEnv)}
	opID, err := svc.preparePatchOperation(ctx, op, g.TenantID, gid,
		groupContents{env: oldEnv, files: map[string]string{}}, groupContents{env: newEnv, files: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	op.ID = opID
	if _, err := svc.claimContent(ctx, g, op); err != nil {
		t.Fatal(err)
	}
	if phase != opPhaseAdmitted {
		if err := svc.advanceOpPhase(ctx, gid, opID, phase); err != nil {
			t.Fatal(err)
		}
	}
	return opID
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// recover runs recovery on the group's current row.
func recoverGroup(t *testing.T, svc *Service, gid string) error {
	t.Helper()
	g, err := svc.readGroup(testCtx(), gid)
	if err != nil {
		t.Fatal(err)
	}
	return svc.ensureGroupOperable(testCtx(), g)
}

// opMapsLeft counts the recovery maps still stored for opID.
func opMapsLeft(svc *Service, workspace, gid, opID string) int {
	n := 0
	for _, path := range []string{opOldEnvPath(gid, opID), opOldFilesPath(gid, opID), opNewEnvPath(gid, opID), opNewFilesPath(gid, opID)} {
		if hasPath(svc.Store.(*fakeStore), workspace, path) {
			n++
		}
	}
	return n
}

func hasPath(f *fakeStore, workspace, path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.m[f.key(groupCtx(testCtx(), workspace), path)]
	return ok
}

func TestRecoverExpiredPatchFinalizesCommittedMaps(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	svc.Base.Clock = time.Now
	svc.OpLease = time.Millisecond
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	opID := strand(t, svc, group.ID, opKindPatch, opPhaseFilesWritten, map[string]string{"TOKEN": "before"}, map[string]string{"TOKEN": "after", "NEW": "1"})
	time.Sleep(3 * time.Millisecond)
	if err := recoverGroup(t, svc, group.ID); err != nil {
		t.Fatalf("recover: %v", err)
	}
	got, err := svc.GetEnvGroupVar(ctx, group.ID, "TOKEN")
	if err != nil || got.Value != "after" {
		t.Fatalf("TOKEN after recover = %+v err=%v", got, err)
	}
	view, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil || view.Availability != "" || len(view.EnvVars) != 2 || view.Revision != encodeEnvGroupRevision(2) {
		t.Fatalf("view after recover = %+v err=%v; want both keys at generation 2", view, err)
	}
	if n := opMapsLeft(svc, view.OwnerID, group.ID, opID); n != 0 {
		t.Fatalf("%d recovery maps survived finalization", n)
	}
}

func TestRecoverExpiredPatchRestoresAdmittedOperation(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	svc.Base.Clock = time.Now
	svc.OpLease = time.Millisecond
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	opID := strand(t, svc, group.ID, opKindPatch, opPhaseAdmitted, map[string]string{"TOKEN": "before"}, map[string]string{"TOKEN": "partial"})
	time.Sleep(3 * time.Millisecond)
	if err := recoverGroup(t, svc, group.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetEnvGroupVar(ctx, group.ID, "TOKEN")
	if got.Value != "before" {
		t.Fatalf("admitted recovery should restore before, got %q", got.Value)
	}
	if view, _ := svc.GetEnvGroup(ctx, group.ID); view.Revision != encodeEnvGroupRevision(1) {
		t.Fatalf("an abandoned save moved the revision to %s", view.Revision)
	}
	if n := opMapsLeft(svc, group.OwnerID, group.ID, opID); n != 0 {
		t.Fatalf("%d recovery maps survived the restore", n)
	}
}

// A save that crashed after writing env but before files restores the old env
// map and its projection.
func TestRecoverExpiredPatchRestoresEnvWritten(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	svc.Base.Clock = time.Now
	svc.OpLease = time.Millisecond
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	strand(t, svc, group.ID, opKindPatch, opPhaseEnvWritten, map[string]string{"TOKEN": "before"}, map[string]string{"TOKEN": "half"})
	// The crashed save had already written its new env map and projection.
	_ = svc.storeMap(ctx, group.OwnerID, envPath(group.ID), map[string]string{"TOKEN": "half"})
	_ = svc.upsertSecret(ctx, group.OwnerID, envSecretName(group.ID), map[string]string{"TOKEN": "half"})
	time.Sleep(3 * time.Millisecond)
	if err := recoverGroup(t, svc, group.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.GetEnvGroupVar(ctx, group.ID, "TOKEN"); got.Value != "before" {
		t.Fatalf("TOKEN after env_written recovery = %q, want before", got.Value)
	}
	if v := string(secret(t, svc.Client, envSecretName(group.ID)).Data["TOKEN"]); v != "before" {
		t.Fatalf("projection after env_written recovery = %q, want before", v)
	}
}

func TestActiveLeaseRejectsConcurrentWriter(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	svc.OpLease = time.Hour
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	strand(t, svc, group.ID, opKindPatch, opPhaseAdmitted, map[string]string{"TOKEN": "before"}, map[string]string{"TOKEN": "before"})
	if err := recoverGroup(t, svc, group.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("active lease = %v, want conflict", err)
	}
	view, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil || view.Availability != "busy" || len(view.EnvVars) != 0 || view.Revision != "" {
		t.Fatalf("busy view = %+v err=%v; want availability busy with no content names", view, err)
	}
	if _, err := svc.SetEnvGroupVar(ctx, group.ID, "TOKEN", "racing"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("write during an active lease = %v, want conflict", err)
	}
}

func TestListEnvGroupsSurfacesBusyWithoutFailingHealthyPeers(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	svc.OpLease = time.Hour
	healthy, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "healthy"})
	if err != nil {
		t.Fatal(err)
	}
	busy, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "busy-one"})
	if err != nil {
		t.Fatal(err)
	}
	strand(t, svc, busy.ID, opKindPatch, opPhaseAdmitted, map[string]string{}, map[string]string{})
	list, err := svc.ListEnvGroups(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	var sawHealthy, sawBusy bool
	for _, g := range list {
		switch g.ID {
		case healthy.ID:
			sawHealthy = true
			if g.Availability != "" {
				t.Fatalf("healthy availability = %q", g.Availability)
			}
		case busy.ID:
			sawBusy = true
			if g.Availability != "busy" {
				t.Fatalf("busy availability = %q", g.Availability)
			}
		}
	}
	if !sawHealthy || !sawBusy {
		t.Fatalf("list missing peers: healthy=%v busy=%v (%d groups)", sawHealthy, sawBusy, len(list))
	}
}

func TestRecoverExpiredCloneReleasesSourceLock(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	svc.Base.Clock = time.Now
	svc.OpLease = time.Millisecond
	source, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "src", EnvVars: []CreateEnvVarInput{{Key: "A", Value: "1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	row, _ := svc.readGroup(ctx, source.ID)
	if _, err := svc.claimContent(ctx, row, store.EnvGroupOp{ID: "ego-clone", Kind: opKindClone}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Millisecond)
	if err := recoverGroup(t, svc, source.ID); err != nil {
		t.Fatal(err)
	}
	view, err := svc.GetEnvGroup(ctx, source.ID)
	if err != nil || view.Availability != "" || view.Name != "src" || view.Revision != encodeEnvGroupRevision(1) {
		t.Fatalf("source after clone recover = %+v err=%v", view, err)
	}
	got, _ := svc.GetEnvGroupVar(ctx, source.ID, "A")
	if got.Value != "1" {
		t.Fatalf("clone recover mutated source value to %q", got.Value)
	}
}

// repair_required with no recorded operation cannot be recovered and says so.
func TestRepairRequiredWithoutOperationFailsHonestly(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "stuck"})
	if err != nil {
		t.Fatal(err)
	}
	row, _ := svc.fakeGroups().Row(group.ID)
	row.Content.State = store.EnvGroupRepairRequired
	svc.fakeGroups().Set(row)
	view, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil || view.Availability != "repair_required" {
		t.Fatalf("view = %+v err=%v, want repair_required", view, err)
	}
	if _, err := svc.SetEnvGroupVar(ctx, group.ID, "A", "1"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("write to a repair_required group = %v, want conflict", err)
	}
}

// A save that changed only env and crashed at files_written finishes with the
// new env names and the files side's names untouched: its recovery maps hold
// only the side it changed.
func TestRecoverFinalizeKeepsTheUnchangedSidesNames(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	svc.Base.Clock = time.Now
	svc.OpLease = time.Millisecond
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "A", Value: "1"}},
		SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "pem"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	opID := strand(t, svc, group.ID, opKindPatch, opPhaseFilesWritten, map[string]string{"A": "1"}, map[string]string{"B": "2"})
	if hasPath(svc.Store.(*fakeStore), group.OwnerID, opOldFilesPath(group.ID, opID)) {
		t.Fatal("a save that left files alone wrote files recovery maps")
	}
	time.Sleep(3 * time.Millisecond)
	if err := recoverGroup(t, svc, group.ID); err != nil {
		t.Fatal(err)
	}
	view, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil || len(view.EnvVars) != 1 || view.EnvVars[0].Key != "B" ||
		len(view.SecretFiles) != 1 || view.SecretFiles[0].Name != "cert.pem" {
		t.Fatalf("view after finalize = %+v err=%v; want env B and the untouched cert.pem", view, err)
	}
}
