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

package webhooks

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type renderVocabularyFixture struct {
	CapturedAt                          string            `json:"capturedAt"`
	RenderDashboardCapturedAt           string            `json:"renderDashboardCapturedAt"`
	RenderOpenAPI                       []string          `json:"renderOpenAPI"`
	RenderDashboard                     []string          `json:"renderDashboard"`
	APIOnly                             []string          `json:"apiOnly"`
	DashboardOnly                       []string          `json:"dashboardOnly"`
	BexSupported                        []string          `json:"bexSupported"`
	BexExtensions                       []string          `json:"bexExtensions"`
	RenderDocumentedExtensionsSupported []string          `json:"renderDocumentedExtensionsSupported"`
	RenderAliasesUnsupported            map[string]string `json:"renderAliasesUnsupported"`
	AntiGoalUnsupported                 []string          `json:"antiGoalUnsupported"`
	SourceBoundUnsupported              []string          `json:"sourceBoundUnsupported"`
	// PlannedUnsupported maps a Render value bex does not emit yet to the
	// board item that will add it.
	PlannedUnsupported map[string]string `json:"plannedUnsupported"`
}

func loadRenderVocabularyFixture(t *testing.T) renderVocabularyFixture {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve vocabulary test source")
	}
	path := filepath.Join(filepath.Dir(source), "..", "..", "..", "..", "docs", "render-artifacts", "fixtures", "render-webhook-vocabulary-2026-11-02.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture renderVocabularyFixture
	if err := json.Unmarshal(contents, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func sortedSet(values ...[]string) []string {
	set := make(map[string]bool)
	for _, list := range values {
		for _, value := range list {
			set[value] = true
		}
	}
	return slices.Sorted(maps.Keys(set))
}

func difference(left, right []string) []string {
	rightSet := make(map[string]bool, len(right))
	for _, value := range right {
		rightSet[value] = true
	}
	var out []string
	for _, value := range left {
		if !rightSet[value] {
			out = append(out, value)
		}
	}
	slices.Sort(out)
	return out
}

func TestRenderDashboardAndAPIVocabulariesStayDistinct(t *testing.T) {
	fixture := loadRenderVocabularyFixture(t)
	// The public enum was re-captured by the round-5 drift (w2/m251); the
	// dashboard picker needs an authenticated walk and keeps its 2026-08-17 set.
	if fixture.CapturedAt != "2026-11-02" || fixture.RenderDashboardCapturedAt != "2026-08-17" ||
		len(fixture.RenderOpenAPI) != 68 || len(fixture.RenderDashboard) != 64 {
		t.Fatalf("dated Render counts = %s API=%d dashboard(%s)=%d", fixture.CapturedAt, len(fixture.RenderOpenAPI),
			fixture.RenderDashboardCapturedAt, len(fixture.RenderDashboard))
	}
	if got, want := difference(fixture.RenderOpenAPI, fixture.RenderDashboard), sortedSet(fixture.APIOnly); !slices.Equal(got, want) {
		t.Fatalf("API-only event types = %v, fixture says %v", got, want)
	}
	if got, want := difference(fixture.RenderDashboard, fixture.RenderOpenAPI), sortedSet(fixture.DashboardOnly); !slices.Equal(got, want) {
		t.Fatalf("dashboard-only event types = %v, fixture says %v", got, want)
	}
}

func TestBexWebhookVocabularyHasOneTruthfulDispositionPerValue(t *testing.T) {
	fixture := loadRenderVocabularyFixture(t)
	if got, want := sortedSet(EventTypes), sortedSet(fixture.BexSupported); !slices.Equal(got, want) {
		t.Fatalf("served Bex event types = %v, fixture says %v", got, want)
	}

	// Bex values absent from the public API are either an explicit Bex event or
	// a value Render's authenticated picker/prose still exposes. Nothing else
	// can enter the served picker to make a parity count look green.
	if got, want := difference(EventTypes, fixture.RenderOpenAPI), sortedSet(fixture.BexExtensions, fixture.RenderDocumentedExtensionsSupported); !slices.Equal(got, want) {
		t.Fatalf("Bex non-OpenAPI values = %v, disposition = %v", got, want)
	}

	aliasValues := make([]string, 0, len(fixture.RenderAliasesUnsupported))
	for source, target := range fixture.RenderAliasesUnsupported {
		aliasValues = append(aliasValues, source)
		if !slices.Contains(EventTypes, target) {
			t.Fatalf("Render alias %q points to unsupported Bex value %q", source, target)
		}
	}
	renderUnion := sortedSet(fixture.RenderOpenAPI, fixture.RenderDashboard)
	unsupported := difference(renderUnion, EventTypes)
	planned := slices.Collect(maps.Keys(fixture.PlannedUnsupported))
	for value, item := range fixture.PlannedUnsupported {
		if !strings.HasPrefix(item, "w") || !strings.Contains(item, "/") {
			t.Fatalf("planned value %q names no board item: %q", value, item)
		}
	}
	dispositioned := sortedSet(aliasValues, fixture.AntiGoalUnsupported, fixture.SourceBoundUnsupported, planned)
	if !slices.Equal(unsupported, dispositioned) {
		t.Fatalf("Render-only values = %v, disposition ledger = %v", unsupported, dispositioned)
	}
	if len(dispositioned) != len(aliasValues)+len(fixture.AntiGoalUnsupported)+len(fixture.SourceBoundUnsupported)+len(planned) {
		t.Fatal("unsupported disposition categories overlap")
	}
}
