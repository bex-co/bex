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

package api

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// conformanceExemptions lists served Render operations that are excluded from
// 2xx JSON response-conformance coverage in internal/api. Every other served
// operation (pinned spec minus renderOperationOmissions) must have at least one
// validated response in a full package run.
var conformanceExemptions = map[string]string{
	"subscribe-logs": "SSE stream (text/event-stream), excluded from JSON response validation by rendertest.isStream",
}

// conformanceRefusalOnly lists served Render operations that intentionally
// answer a Render error envelope rather than 2xx (per DO_NOT_DO.md / ADR018).
// They must still be exercised and validated against Render's error schema.
var conformanceRefusalOnly = map[string]string{
	"post-job": "DO_NOT_DO.md: one-off jobs are unsupported (410 ONE_OFF_JOBS_UNSUPPORTED)",
}

// TestMain fails a full run that leaves an internal/api Allowlist entry
// unexercised (rendertest.RunSuite) or any served non-streaming Render
// operation unvalidated: every test building its handler with buildHandler
// feeds the tally.
func TestMain(m *testing.M) {
	code := rendertest.RunSuite(m, rendertest.SuiteAPI)
	if code == 0 && rendertest.FullPackageRun() {
		if problems := checkRenderConformanceCoverage(); len(problems) > 0 {
			fmt.Fprintf(os.Stderr, "FAIL: Render response-conformance coverage gaps (%d):\n  %s\n",
				len(problems), strings.Join(problems, "\n  "))
			code = 1
		}
	}
	os.Exit(code)
}

func checkRenderConformanceCoverage() []string {
	contract, err := renderContractOnce()
	if err != nil {
		return []string{"load pinned contract: " + err.Error()}
	}
	return checkRenderConformanceCoverageWith(contract.document, rendertest.ValidatedStatuses(), conformanceExemptions, conformanceRefusalOnly)
}

func checkRenderConformanceCoverageWith(doc *openapi3.T, validated map[string]map[int]bool, exemptions, refusalOnly map[string]string) []string {
	var problems []string
	served := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			id := op.OperationID
			if id == "" {
				continue
			}
			if _, omitted := renderOperationOmissions[id]; omitted {
				continue
			}
			served[id] = true
			if _, exempt := exemptions[id]; exempt {
				continue
			}
			statuses := validated[id]
			if len(statuses) == 0 {
				problems = append(problems, fmt.Sprintf("%s (%s %s): never validated", id, strings.ToUpper(method), path))
				continue
			}
			if _, isRefusal := refusalOnly[id]; isRefusal {
				continue
			}
			has2xx := false
			for st := range statuses {
				if st >= 200 && st < 300 {
					has2xx = true
					break
				}
			}
			if !has2xx {
				var got []int
				for st := range statuses {
					got = append(got, st)
				}
				slices.Sort(got)
				problems = append(problems, fmt.Sprintf("%s (%s %s): only non-2xx validated (%v)", id, strings.ToUpper(method), path, got))
			}
		}
	}
	for id := range exemptions {
		if !served[id] {
			problems = append(problems, fmt.Sprintf("stale conformanceExemptions entry %q", id))
		}
	}
	for id := range refusalOnly {
		if !served[id] {
			problems = append(problems, fmt.Sprintf("stale conformanceRefusalOnly entry %q", id))
		}
	}
	slices.Sort(problems)
	return problems
}

func TestRenderConformanceCoverageGuard(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	all200 := map[string]map[int]bool{}
	for _, item := range contract.document.Paths.Map() {
		for _, op := range item.Operations() {
			if op.OperationID == "" {
				continue
			}
			if _, omitted := renderOperationOmissions[op.OperationID]; omitted {
				continue
			}
			if _, exempt := conformanceExemptions[op.OperationID]; exempt {
				continue
			}
			if _, refusal := conformanceRefusalOnly[op.OperationID]; refusal {
				all200[op.OperationID] = map[int]bool{410: true}
				continue
			}
			all200[op.OperationID] = map[int]bool{200: true}
		}
	}
	if probs := checkRenderConformanceCoverageWith(contract.document, all200, conformanceExemptions, conformanceRefusalOnly); len(probs) != 0 {
		t.Fatalf("full validated map reported problems: %v", probs)
	}

	// Missing operation is caught.
	missing := make(map[string]map[int]bool, len(all200))
	for k, v := range all200 {
		missing[k] = v
	}
	delete(missing, "delete-build-source")
	probs := checkRenderConformanceCoverageWith(contract.document, missing, conformanceExemptions, conformanceRefusalOnly)
	if len(probs) != 1 || !strings.Contains(probs[0], "delete-build-source") || !strings.Contains(probs[0], "never validated") {
		t.Fatalf("missing op problems = %v, want single never-validated failure for delete-build-source", probs)
	}

	// Non-2xx-only operation (not in refusalOnly) is caught.
	missing["delete-build-source"] = map[int]bool{404: true}
	probs = checkRenderConformanceCoverageWith(contract.document, missing, conformanceExemptions, conformanceRefusalOnly)
	if len(probs) != 1 || !strings.Contains(probs[0], "delete-build-source") || !strings.Contains(probs[0], "only non-2xx validated") {
		t.Fatalf("non-2xx-only problems = %v, want single non-2xx failure for delete-build-source", probs)
	}

	// Stale exemption / refusalOnly entries are caught.
	badExempt := map[string]string{"subscribe-logs": "sse", "not-a-real-op": "stale"}
	probs = checkRenderConformanceCoverageWith(contract.document, all200, badExempt, conformanceRefusalOnly)
	if len(probs) != 1 || !strings.Contains(probs[0], "stale conformanceExemptions") {
		t.Fatalf("stale exemption problems = %v, want stale entry error", probs)
	}
}
