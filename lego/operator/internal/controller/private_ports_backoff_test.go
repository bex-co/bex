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

package controller

import (
	"context"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A sweep dials thousands of ports of a pod for its whole 15 s timeout, and
// sweeping every service every two minutes kept the production manager at 0.4
// to 0.5 cores, throttled at its old limit (w21/m421 t002: the profile put 75%
// of its CPU in portscan.scan). A service whose listeners hold still is
// rescanned at doubling intervals up to 16 minutes; any change, or a new
// revision, brings it back to two.
func TestPrivatePortRescanBacksOffWhileListenersHoldStill(t *testing.T) {
	r := &AppReconciler{}
	const key = "tea-a/db"
	held := &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9187}}
	sweepFound := func(next *appv1alpha1.RuntimePortsStatus) {
		r.notePrivateSweep(key, "rev-2", held, next)
	}
	if got := r.privatePortRescanEvery(key, "rev-2"); got != 2*time.Minute {
		t.Fatalf("before any sweep the spacing is %v, want 2m", got)
	}
	for i, want := range []time.Duration{4, 8, 16, 16, 16} {
		sweepFound(&appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9187}})
		if got := r.privatePortRescanEvery(key, "rev-2"); got != want*time.Minute {
			t.Fatalf("after %d unchanged sweeps the spacing is %v, want %vm", i+1, got, int64(want))
		}
	}
	// A sweep that finds another listener set resets it.
	sweepFound(&appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9187, 7000}})
	if got := r.privatePortRescanEvery(key, "rev-2"); got != 2*time.Minute {
		t.Fatalf("after a changed listener set the spacing is %v, want 2m", got)
	}
	// A failed sweep proves nothing: it neither backs off nor resets.
	sweepFound(held)
	r.notePrivateSweep(key, "rev-2", held, nil)
	if got := r.privatePortRescanEvery(key, "rev-2"); got != 4*time.Minute {
		t.Fatalf("a failed sweep moved the spacing to %v, want it to stay 4m", got)
	}
	// Another revision starts over.
	if got := r.privatePortRescanEvery(key, "rev-3"); got != 2*time.Minute {
		t.Fatalf("a new revision is spaced %v, want 2m", got)
	}
}

// Rescans of known revisions share two slots, so the manager's CPU for sweeps
// has a ceiling however many services run; a pass that finds none leaves the
// rescan to its next one, and a new revision's first sweep never waits.
func TestPrivatePortRescansAreBounded(t *testing.T) {
	scanner := &cannedScanner{open: []int{5432}}
	r, app, dep := privateScanFixture(scanner, true, time.Nanosecond)
	app.UID = "uid-1"
	app.Status.RuntimePorts = &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432}}

	held := make([]func(), 0, privatePortRescanSlots)
	for range privatePortRescanSlots {
		release, ok := r.acquireRescanSlot()
		if !ok {
			t.Fatal("a free slot was refused")
		}
		held = append(held, release)
	}
	if _, ok := r.acquireRescanSlot(); ok {
		t.Fatalf("a %dth concurrent rescan got a slot", privatePortRescanSlots+1)
	}

	if got := r.scanPrivatePorts(context.Background(), app, dep); got != nil || awaitPrivateSweep(r, app) || scanner.calls != 0 {
		t.Fatalf("with every slot taken a rescan started (record %+v, %d scans)", got, scanner.calls)
	}
	if _, stamped := r.privatePortScanned.Load("tea-a/db"); stamped {
		t.Fatal("a rescan that got no slot still counted as scanned, so its next pass would wait out the interval")
	}

	// A new revision is swept at once, slots or not.
	app.Status.RuntimePorts = &appv1alpha1.RuntimePortsStatus{Revision: "rev-1", Ports: []int32{5432}}
	if got := r.scanPrivatePorts(context.Background(), app, dep); got != nil || !awaitPrivateSweep(r, app) || scanner.calls != 1 {
		t.Fatalf("a new revision's first sweep was held back (record %+v, %d scans)", got, scanner.calls)
	}
	got := r.scanPrivatePorts(context.Background(), app, dep)
	if got == nil || got.Revision != "rev-2" {
		t.Fatalf("the woken pass took %+v, want rev-2's record", got)
	}
	app.Status.RuntimePorts = got

	// The revision is known now, so its rescan needs a slot: none is free.
	if again := r.scanPrivatePorts(context.Background(), app, dep); again != nil || awaitPrivateSweep(r, app) || scanner.calls != 1 {
		t.Fatalf("a rescan of a known revision ran without a slot (record %+v, %d scans)", again, scanner.calls)
	}

	// Once a slot frees, the rescan runs.
	held[0]()
	if got := r.scanPrivatePorts(context.Background(), app, dep); got != nil || !awaitPrivateSweep(r, app) || scanner.calls != 2 {
		t.Fatalf("after a slot freed the rescan did not run (record %+v, %d scans)", got, scanner.calls)
	}
	for _, release := range held[1:] {
		release()
	}
}

// The pass that takes a finished sweep's record is what feeds the backoff: a
// sweep that found the listeners the record already held doubles the spacing.
func TestAFinishedSweepFeedsTheRescanBackoff(t *testing.T) {
	scanner := &cannedScanner{open: []int{5432, 9187}}
	r, app, dep := privateScanFixture(scanner, true, time.Nanosecond)
	app.UID = "uid-1"
	app.Status.RuntimePorts = &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9187}}
	if got := sweptPrivatePorts(r, app, dep); got == nil || scanner.calls != 1 {
		t.Fatalf("the rescan took %+v after %d scans, want its record", got, scanner.calls)
	}
	if got := r.privatePortRescanEvery("tea-a/db", "rev-2"); got != 2*time.Nanosecond {
		t.Fatalf("after a sweep that found the same listeners the spacing is %v, want it doubled", got)
	}
	scanner.open = []int{5432}
	app.Status.RuntimePorts = &appv1alpha1.RuntimePortsStatus{Revision: "rev-2", Ports: []int32{5432, 9187}}
	if got := sweptPrivatePorts(r, app, dep); got == nil {
		t.Fatal("the next rescan took no record")
	}
	if got := r.privatePortRescanEvery("tea-a/db", "rev-2"); got != time.Nanosecond {
		t.Fatalf("after a sweep that found a closed listener the spacing is %v, want it reset", got)
	}
}
