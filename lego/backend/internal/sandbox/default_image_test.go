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
	"net/http/httptest"
	"slices"
	"testing"
)

// w22/m447 t005: changing the default image (BEX_SANDBOX_IMAGE, the base
// template's image) changes only the sandboxes created afterwards. One created
// before keeps the image it was created from across suspend and resume:
// resume names no image, and OpenSandbox resumes its BatchSandbox from its own
// committed root filesystem. Its commands still run through the exec wrapper,
// which falls back to /bin/sh where that image has no bash
// (TestExecFallsBackToShWhenTheRootFilesystemHasNoBash).
func TestASandboxKeepsItsImageWhenTheDefaultChanges(t *testing.T) {
	const (
		before = "docker.io/library/bash:5.3@sha256:589adbc588d8ef3c1ec3c68333faf4baf5dccb65d3cb50f6cf3d78cad2d6e1b0"
		after  = "ghcr.io/bex-co/bex-operator:sandbox-base-0000000000000000000000000000000000000000@sha256:1111111111111111111111111111111111111111111111111111111111111111"
	)
	base := func(image string) map[string]Template {
		return map[string]Template{"base": {Image: image, Entrypoint: []string{"/bin/sh", "-c", "exec sleep infinity"}, CPU: "1", Memory: "2Gi"}}
	}
	f := newIdleFixture(t)
	f.svc.Templates, f.svc.DefaultTemplate = base(before), "base"
	old, err := f.svc.Create(callerCtx(), CreateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	osID := f.os.idOf(old.ID)
	if osID == "" || f.os.box(osID).image != before {
		t.Fatalf("created %q from %q, want the default of the day, %q", osID, f.os.box(osID).image, before)
	}

	// bex-api rolls out with a new default; the sandbox is suspended and
	// resumed under it.
	f.svc.Templates = base(after)
	// The real server moves Pausing to Paused and Resuming to Running on its
	// own; the fake moves when told.
	settle := func(want Status) {
		t.Helper()
		f.os.set(osID, func(b *fakeBox) { b.state, b.next = b.next, "" })
		box, err := f.svc.Get(callerCtx(), old.ID)
		if err != nil || box.Status != want {
			t.Fatalf("sandbox = %s (%v), want %s", box.Status, err, want)
		}
	}
	if err := f.svc.Suspend(callerCtx(), old.ID); err != nil {
		t.Fatal(err)
	}
	settle(StatusSuspended)
	if err := f.svc.Resume(callerCtx(), old.ID); err != nil {
		t.Fatal(err)
	}
	settle(StatusRunning)
	if got := f.os.box(osID).image; got != before || len(f.os.resumeImages) != 0 {
		t.Fatalf("after resume the sandbox runs %q (resume named %q), want its own %q", got, f.os.resumeImages, before)
	}

	// Its commands still reach it through the exec wrapper.
	rr := httptest.NewRecorder()
	if err := f.svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: old.ID, Command: "echo resumed"}, rr, rr.Flush); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	argv := slices.Clone(f.gwArgv)
	f.mu.Unlock()
	if len(argv) != 1 || len(argv[0]) < 3 || !slices.Equal(argv[0][:3], []string{"/bin/sh", "-c", homeExecScript}) || argv[0][len(argv[0])-1] != "echo resumed" {
		t.Fatalf("exec after resume ran %q, want the home wrapper with the command last", argv)
	}

	// A sandbox created now starts from the new default.
	fresh, err := f.svc.Create(callerCtx(), CreateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.os.box(f.os.idOf(fresh.ID)).image; got != after {
		t.Fatalf("a new sandbox runs %q, want the new default %q", got, after)
	}
}
