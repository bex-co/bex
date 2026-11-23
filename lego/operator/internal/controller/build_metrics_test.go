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
// recorded only where there was a layout to measure. w9/m493: a native
// build's $XDG_CACHE_HOME archive lands on series of its own kind, so an
// archive's hit never reads as the layer cache's.
func TestRecordBuildSignalsMetersEachCachePhase(t *testing.T) {
	layer, xdg := build.CacheKindLayer, build.CacheKindXDG
	restoreHit := buildCachePhaseSeconds.WithLabelValues("cache-restore", layer, build.CacheHit)
	saveError := buildCachePhaseSeconds.WithLabelValues("cache-save", layer, build.CacheError)
	xdgRestoreHit := buildCachePhaseSeconds.WithLabelValues("cache-restore", xdg, build.CacheHit)
	xdgSaveSaved := buildCachePhaseSeconds.WithLabelValues("cache-save", xdg, build.CacheSaved)
	restoreBytes := buildCacheBytes.WithLabelValues("cache-restore", layer)
	saveBytes := buildCacheBytes.WithLabelValues("cache-save", layer)
	xdgRestoreBytes := buildCacheBytes.WithLabelValues("cache-restore", xdg)
	hits, errs := histogramCount(t, restoreHit), histogramCount(t, saveError)
	xdgHits, xdgSaves := histogramCount(t, xdgRestoreHit), histogramCount(t, xdgSaveSaved)
	restored, saved, xdgRestored := histogramCount(t, restoreBytes), histogramCount(t, saveBytes), histogramCount(t, xdgRestoreBytes)

	recordBuildSignals(build.Signals{CachePhases: []build.CachePhaseSignal{
		{Phase: "cache-restore", Kind: layer, Outcome: build.CacheHit, Seconds: 3, Bytes: 1 << 20},
		{Phase: "cache-restore", Kind: xdg, Outcome: build.CacheHit, Seconds: 2, Bytes: 4 << 20},
		{Phase: "cache-save", Kind: layer, Outcome: build.CacheError, Seconds: 1},
		{Phase: "cache-save", Kind: xdg, Outcome: build.CacheSaved, Seconds: 1, Bytes: 5 << 20},
	}})

	for _, c := range []struct {
		name   string
		o      prometheus.Observer
		before uint64
		want   uint64
	}{
		{"layer restore hit", restoreHit, hits, 1},
		{"layer save error", saveError, errs, 1},
		{"xdg restore hit", xdgRestoreHit, xdgHits, 1},
		{"xdg save saved", xdgSaveSaved, xdgSaves, 1},
		{"layer restored bytes", restoreBytes, restored, 1},
		{"xdg restored bytes", xdgRestoreBytes, xdgRestored, 1},
		// A failed save recorded no size.
		{"layer saved bytes", saveBytes, saved, 0},
	} {
		if got := histogramCount(t, c.o); got != c.before+c.want {
			t.Errorf("%s count = %d, want %d", c.name, got, c.before+c.want)
		}
	}
}

// w33/m571: each part of a native build's export the build plane read lands
// on its own series, and a build that reported none observes nothing.
func TestRecordBuildSignalsMetersEachExportPart(t *testing.T) {
	parts := []string{build.ExportImage, build.ExportLayers, build.ExportCache, build.ExportXDG, build.ExportTotal}
	before := map[string]uint64{}
	for _, part := range parts {
		before[part] = histogramCount(t, buildExportSeconds.WithLabelValues(part))
	}
	recordBuildSignals(build.Signals{})
	recordBuildSignals(build.Signals{Export: []build.ExportPart{
		{Part: build.ExportImage, Seconds: 434.1}, {Part: build.ExportLayers, Seconds: 412.6},
		{Part: build.ExportCache, Seconds: 12.3}, {Part: build.ExportTotal, Seconds: 452},
	}})
	for _, part := range parts {
		want := before[part] + 1
		if part == build.ExportXDG {
			want = before[part]
		}
		if got := histogramCount(t, buildExportSeconds.WithLabelValues(part)); got != want {
			t.Errorf("%s count = %d, want %d", part, got, want)
		}
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
