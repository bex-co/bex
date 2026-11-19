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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestBuildFailureMappingsAreTotalAndDistinct pins both halves of the
// classification surface (w7/m82 t003): the metrics outcome and the App
// condition reason. Both must map each fault to its own value and must fall
// back to the unclassified value rather than guessing — filing an unknown class
// under either side would corrupt the infra-success SLO in one direction.
func TestBuildFailureMappingsAreTotalAndDistinct(t *testing.T) {
	cases := []struct {
		fault       build.Fault
		wantOutcome string
		wantReason  string
	}{
		{build.FaultTenant, buildOutcomeUserFailed, appv1alpha1.ReasonBuildFailedUserError},
		{build.FaultInfra, buildOutcomeInfraFailed, appv1alpha1.ReasonBuildFailedInfrastructure},
		{build.FaultTimeout, buildOutcomeTimeout, appv1alpha1.ReasonBuildFailed},
		{build.FaultNone, buildOutcomeFailed, appv1alpha1.ReasonBuildFailed},
		{build.Fault("something-new"), buildOutcomeFailed, appv1alpha1.ReasonBuildFailed},
	}
	seenOutcome, seenReason := map[string]bool{}, map[string]bool{}
	for _, c := range cases {
		v := viewForBuildFault(c.fault)
		if v.outcome != c.wantOutcome {
			t.Errorf("view(%q).outcome = %q, want %q", c.fault, v.outcome, c.wantOutcome)
		}
		if v.reason != c.wantReason {
			t.Errorf("view(%q).reason = %q, want %q", c.fault, v.reason, c.wantReason)
		}
		if v.message == "" {
			t.Errorf("view(%q) has no message; the tenant would see a bare colon", c.fault)
		}
		seenOutcome[c.wantOutcome] = true
		seenReason[c.wantReason] = true
	}
	// tenant and infra must be separable in both surfaces.
	// tenant, infra, timeout and unclassified must all be separable by outcome;
	// reason collapses timeout and unclassified onto ReasonBuildFailed by design.
	if len(seenOutcome) != 4 {
		t.Errorf("expected four distinct outcomes, got %d", len(seenOutcome))
	}
	if len(seenReason) != 3 {
		t.Errorf("expected three distinct reasons, got %d", len(seenReason))
	}
	// Every classified reason must be recognised by the shared contract, or
	// bex-api will file it as a generic update failure.
	for _, r := range []string{appv1alpha1.ReasonBuildFailed, appv1alpha1.ReasonBuildFailedUserError, appv1alpha1.ReasonBuildFailedInfrastructure} {
		if !appv1alpha1.IsBuildFailureReason(r) {
			t.Errorf("IsBuildFailureReason(%q) = false; bex-api would misclassify this deploy", r)
		}
	}
	if appv1alpha1.IsBuildFailureReason("IngressFailed") {
		t.Error("IsBuildFailureReason must not be over-broad")
	}
}

// w9/m492: each S3 cache phase's own outcome reaches the fleet, which is the
// only place a store outage shows (every phase exits 0), and a size is
// recorded only where there was a layout to measure.
func TestRecordBuildSignalsMetersEachCachePhase(t *testing.T) {
	restoreHit := buildCachePhaseSeconds.WithLabelValues("cache-restore", build.CacheHit)
	saveError := buildCachePhaseSeconds.WithLabelValues("cache-save", build.CacheError)
	restoreBytes := buildCacheBytes.WithLabelValues("cache-restore")
	saveBytes := buildCacheBytes.WithLabelValues("cache-save")
	hits, errs := histogramCount(t, restoreHit), histogramCount(t, saveError)
	restored, saved := histogramCount(t, restoreBytes), histogramCount(t, saveBytes)

	recordBuildSignals(build.Signals{CachePhases: []build.CachePhaseSignal{
		{Phase: "cache-restore", Outcome: build.CacheHit, Seconds: 3, Bytes: 1 << 20},
		{Phase: "cache-save", Outcome: build.CacheError, Seconds: 1},
	}})

	if got := histogramCount(t, restoreHit); got != hits+1 {
		t.Errorf("restore hit count = %d, want %d", got, hits+1)
	}
	if got := histogramCount(t, saveError); got != errs+1 {
		t.Errorf("save error count = %d, want %d", got, errs+1)
	}
	if got := histogramCount(t, restoreBytes); got != restored+1 {
		t.Errorf("restored bytes count = %d, want %d", got, restored+1)
	}
	if got := histogramCount(t, saveBytes); got != saved {
		t.Errorf("a failed save recorded a size (%d → %d)", saved, got)
	}
}

func histogramCount(t *testing.T, o prometheus.Observer) uint64 {
	t.Helper()
	var m dto.Metric
	if err := o.(prometheus.Metric).Write(&m); err != nil {
		t.Fatal(err)
	}
	return m.GetHistogram().GetSampleCount()
}
