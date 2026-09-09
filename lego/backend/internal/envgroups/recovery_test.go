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
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestRecoverExpiredPatchFinalizesCommittedMaps(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc := newService(store)
	svc.Base.Clock = time.Now
	svc.OpLease = time.Millisecond
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	oldEnv, _ := svc.getGroupMap(ctx, "", envPath(group.ID))
	newEnv := map[string]string{"TOKEN": "after"}
	snap, _ := store.GetVersioned(ctx, revisionPath(group.ID))
	gen := revisionGeneration(snap.Data)
	opID, err := svc.preparePatchOperation(ctx, "", group.ID, gen, oldEnv, map[string]string{}, newEnv, map[string]string{}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutCAS(ctx, revisionPath(group.ID), revisionClaimData("busy", gen, opID), snap.Version); err != nil {
		t.Fatal(err)
	}
	if err := svc.commitOpRecord(ctx, "", group.ID, groupOpRecord{
		id: opID, kind: opKindPatch, phase: opPhaseAdmitted, generation: gen,
		leaseUntil: time.Now().UTC().Add(svc.opLease()), envChanged: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.advanceOpPhase(ctx, "", group.ID, opID, opPhaseFilesWritten); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Millisecond)
	if err := svc.ensureGroupOperable(ctx, "", group.ID); err != nil {
		t.Fatalf("recover: %v", err)
	}
	got, err := svc.GetEnvGroupVar(ctx, group.ID, "TOKEN")
	if err != nil || got.Value != "after" {
		t.Fatalf("TOKEN after recover = %+v err=%v", got, err)
	}
	view, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil || view.Availability != "" {
		t.Fatalf("view after recover = %+v err=%v", view, err)
	}
}

func TestRecoverExpiredPatchRestoresAdmittedOperation(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc := newService(store)
	svc.Base.Clock = time.Now
	svc.OpLease = time.Millisecond
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	oldEnv, _ := svc.getGroupMap(ctx, "", envPath(group.ID))
	snap, _ := store.GetVersioned(ctx, revisionPath(group.ID))
	gen := revisionGeneration(snap.Data)
	opID, err := svc.preparePatchOperation(ctx, "", group.ID, gen, oldEnv, map[string]string{}, map[string]string{"TOKEN": "partial"}, map[string]string{}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutCAS(ctx, revisionPath(group.ID), revisionClaimData("busy", gen, opID), snap.Version); err != nil {
		t.Fatal(err)
	}
	if err := svc.commitOpRecord(ctx, "", group.ID, groupOpRecord{
		id: opID, kind: opKindPatch, phase: opPhaseAdmitted, generation: gen,
		leaseUntil: time.Now().UTC().Add(svc.opLease()), envChanged: true,
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Millisecond)
	if err := svc.ensureGroupOperable(ctx, "", group.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetEnvGroupVar(ctx, group.ID, "TOKEN")
	if got.Value != "before" {
		t.Fatalf("admitted recovery should restore before, got %q", got.Value)
	}
}

func TestActiveLeaseRejectsConcurrentWriter(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc := newService(store)
	svc.OpLease = time.Hour
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "before"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	oldEnv, _ := svc.getGroupMap(ctx, "", envPath(group.ID))
	opID, err := svc.preparePatchOperation(ctx, "", group.ID, 0, oldEnv, map[string]string{}, oldEnv, map[string]string{}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := store.GetVersioned(ctx, revisionPath(group.ID))
	gen := revisionGeneration(snap.Data)
	if _, err := store.PutCAS(ctx, revisionPath(group.ID), revisionClaimData("busy", gen, opID), snap.Version); err != nil {
		t.Fatal(err)
	}
	if err := svc.commitOpRecord(ctx, "", group.ID, groupOpRecord{
		id: opID, kind: opKindPatch, phase: opPhaseAdmitted, generation: gen,
		leaseUntil: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ensureGroupOperable(ctx, "", group.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("active lease = %v, want conflict", err)
	}
	view, err := svc.GetEnvGroup(ctx, group.ID)
	if err != nil || view.Availability != "busy" {
		t.Fatalf("busy view = %+v err=%v", view, err)
	}
}

func TestListEnvGroupsSurfacesBusyWithoutFailingHealthyPeers(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc := newService(store)
	svc.OpLease = time.Hour
	healthy, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "healthy"})
	if err != nil {
		t.Fatal(err)
	}
	busy, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "busy-one"})
	if err != nil {
		t.Fatal(err)
	}
	opID, err := svc.preparePatchOperation(ctx, "", busy.ID, 0, map[string]string{}, map[string]string{}, map[string]string{}, map[string]string{}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := store.GetVersioned(ctx, revisionPath(busy.ID))
	gen := revisionGeneration(snap.Data)
	if _, err := store.PutCAS(ctx, revisionPath(busy.ID), revisionClaimData("busy", gen, opID), snap.Version); err != nil {
		t.Fatal(err)
	}
	if err := svc.commitOpRecord(ctx, "", busy.ID, groupOpRecord{
		id: opID, kind: opKindPatch, phase: opPhaseAdmitted, generation: gen,
		leaseUntil: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
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
	ctx := context.Background()
	store := newFakeStore()
	svc := newService(store)
	svc.Base.Clock = time.Now
	svc.OpLease = time.Millisecond
	source, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "src", EnvVars: []CreateEnvVarInput{{Key: "A", Value: "1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := store.GetVersioned(ctx, revisionPath(source.ID))
	gen := revisionGeneration(snap.Data)
	opID, err := svc.prepareCloneOperation(ctx, "", source.ID, gen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutCAS(ctx, revisionPath(source.ID), revisionClaimData("busy", gen, opID), snap.Version); err != nil {
		t.Fatal(err)
	}
	if err := svc.commitOpRecord(ctx, "", source.ID, groupOpRecord{
		id: opID, kind: opKindClone, phase: opPhaseAdmitted, generation: gen,
		leaseUntil: time.Now().UTC().Add(svc.opLease()),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Millisecond)
	if err := svc.ensureGroupOperable(ctx, "", source.ID); err != nil {
		t.Fatal(err)
	}
	view, err := svc.GetEnvGroup(ctx, source.ID)
	if err != nil || view.Availability != "" || view.Name != "src" {
		t.Fatalf("source after clone recover = %+v err=%v", view, err)
	}
	got, _ := svc.GetEnvGroupVar(ctx, source.ID, "A")
	if got.Value != "1" {
		t.Fatalf("clone recover mutated source value to %q", got.Value)
	}
}
