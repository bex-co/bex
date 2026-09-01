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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// internalMarkers is what w6/m123 exists to keep out of the tenant's sight:
// Kubernetes Job condition text leaked the internal build namespace, the Job
// naming scheme (with the workspace id embedded), a bare exit code, and bex's
// own PodFailurePolicy configuration.
var internalMarkers = []string{"PodFailurePolicy", "bex-build/", "bld-tea-", "FailJob rule", "exit code 90"}

// TestBuildFailureMessageNeverLeaksJobInternals pins the w6/m123 defect: every
// fault class, with and without a captured tail, must compose a tenant-facing
// sentence free of Kubernetes Job internals — even when the raw Job message is
// present on the observation (it is; it belongs to the operator log only).
func TestBuildFailureMessageNeverLeaksJobInternals(t *testing.T) {
	rawJobMessage := "PodFailurePolicy: Container buildkit for pod bex-build/bld-tea-d98210cbbpdc73dcrkvg-x-gen-2-hhtxp failed with exit code 90 matching FailJob rule at index 1"
	tail := "error: failed to solve: failed to read dockerfile: open NoSuchDockerfile: no such file or directory"

	for _, fault := range []build.Fault{build.FaultTenant, build.FaultInfra, build.FaultTimeout, build.FaultNone} {
		for _, withTail := range []bool{true, false} {
			obs := build.Observation{Phase: build.PhaseFailed, Fault: fault, Message: rawJobMessage}
			if withTail {
				obs.Tail = tail
				obs.FailedStep = "docker build"
			}
			msg := buildFailureMessage(viewForBuildFault(fault), obs)
			if msg == "" {
				t.Fatalf("%s/tail=%v: empty message", fault, withTail)
			}
			for _, marker := range internalMarkers {
				if strings.Contains(msg, marker) {
					t.Errorf("%s/tail=%v: message leaks %q: %q", fault, withTail, marker, msg)
				}
			}
		}
	}
}

// TestBuildFailureMessageQuotesTheFailingStep is the missing-Dockerfile
// fixture's acceptance shape: a tenant fault with a captured tail names the
// failing step and carries the step's own output — the sentence the tenant can
// act on. It asserts structure (step named, tail present), not exact wording.
func TestBuildFailureMessageQuotesTheFailingStep(t *testing.T) {
	obs := build.Observation{
		Phase: build.PhaseFailed, Fault: build.FaultTenant,
		FailedStep: "docker build",
		Tail:       "failed to read dockerfile: open NoSuchDockerfile: no such file or directory",
	}
	msg := buildFailureMessage(viewForBuildFault(build.FaultTenant), obs)
	if !strings.Contains(msg, "docker build") {
		t.Errorf("message does not name the failing step: %q", msg)
	}
	if !strings.Contains(msg, "NoSuchDockerfile") {
		t.Errorf("message does not carry the step's own error: %q", msg)
	}

	// Without a tail (reaped pod), the message must still stand on its own and
	// point somewhere actionable rather than trail off.
	obs.Tail = ""
	bare := buildFailureMessage(viewForBuildFault(build.FaultTenant), obs)
	if !strings.Contains(bare, "build logs") {
		t.Errorf("tail-less message gives no place to look: %q", bare)
	}

	// A timeout has no failing container by construction; its message carries
	// the budget that expired instead of quoting output.
	timeout := buildFailureMessage(viewForBuildFault(build.FaultTimeout), build.Observation{Fault: build.FaultTimeout})
	if !strings.Contains(timeout, "time limit") {
		t.Errorf("timeout message does not name the expired budget: %q", timeout)
	}
}

// TestBuildFailureClassesStayDistinct guards w7/m82/t003's split: a tenant
// fault and an infra fault must keep producing different condition reasons and
// different tenant-facing sentences — m123 improves the message without
// collapsing the classification.
func TestBuildFailureClassesStayDistinct(t *testing.T) {
	tenant := viewForBuildFault(build.FaultTenant)
	infra := viewForBuildFault(build.FaultInfra)
	if tenant.reason == infra.reason {
		t.Fatalf("tenant and infra faults share condition reason %q", tenant.reason)
	}
	tenantMsg := buildFailureMessage(tenant, build.Observation{Fault: build.FaultTenant})
	infraMsg := buildFailureMessage(infra, build.Observation{Fault: build.FaultInfra})
	if tenantMsg == infraMsg {
		t.Fatalf("tenant and infra faults read identically: %q", tenantMsg)
	}
}

// TestStampBuildRun pins the failure-path timing projection (w6/m123): a
// reported window lands on the App status attributed to the release
// generation; an unreported one (zero times) writes nothing, so bex-api leaves
// the deploy row's started_at honestly null instead of inheriting a stale or
// fabricated window.
func TestStampBuildRun(t *testing.T) {
	start := time.Date(2026, 8, 27, 19, 56, 10, 0, time.UTC)
	finish := start.Add(68 * time.Second)

	app := &appv1alpha1.App{}
	app.Generation = 4
	app.Status.ReleaseGeneration = 3

	stampBuildRun(app, build.Observation{StartedAt: start, FinishedAt: finish})
	br := app.Status.BuildRun
	if br == nil || br.Generation != 3 {
		t.Fatalf("BuildRun = %+v, want release-generation 3 attribution", br)
	}
	gotStart, _ := time.Parse(time.RFC3339, br.StartedAt)
	gotFinish, _ := time.Parse(time.RFC3339, br.FinishedAt)
	if !gotStart.Equal(start) || !gotFinish.Equal(finish) {
		t.Fatalf("window = %s..%s, want %s..%s", br.StartedAt, br.FinishedAt, start, finish)
	}

	// An unknown window must not clobber nothing onto the status either way.
	fresh := &appv1alpha1.App{}
	stampBuildRun(fresh, build.Observation{})
	if fresh.Status.BuildRun != nil {
		t.Fatalf("zero window stamped %+v, want nil", fresh.Status.BuildRun)
	}
}
