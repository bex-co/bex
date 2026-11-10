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
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// Every absent Render operation is a named, documented omission; new upstream
// operations cannot silently fall out of the route intersection inventory.
var renderOperationOmissions = map[string]string{
	"cancelFlowRun":                "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"cancelTaskRun":                "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"create-dedicated-ip":          "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"create-sandbox-build":         "ADR018 \u00a7 Render operation omissions \u2014 Render sandbox groups, snapshots and executions",
	"create-sandbox-snapshot":      "ADR018 \u00a7 Render operation omissions \u2014 Render sandbox groups, snapshots and executions",
	"createTask":                   "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"createWorkflow":               "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"createWorkflowVersion":        "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"delete-dedicated-ip":          "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"delete-owner-log-stream":      "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"delete-resource-log-stream":   "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"delete-sandbox-snapshot":      "ADR018 \u00a7 Render operation omissions \u2014 Render sandbox groups, snapshots and executions",
	"deleteOwnerMetricsStream":     "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"deleteWorkflow":               "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"get-owner-log-stream":         "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"get-resource-log-stream":      "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"get-task-runs-completed":      "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"get-task-runs-queued":         "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getFlow":                      "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getFlowRun":                   "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getFlowRunStep":               "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getOwnerMetricsStream":        "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"getTask":                      "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getTaskRun":                   "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getWorkflow":                  "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getWorkflowVersion":           "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"list-dedicated-ips":           "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"list-maintenance":             "ADR018 \u00a7 Render operation omissions \u2014 Maintenance scheduling",
	"list-organization-audit-logs": "ADR018 \u00a7 Render operation omissions \u2014 Render audit-log paths",
	"list-resource-log-streams":    "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"list-sandbox-builds":          "ADR018 \u00a7 Render operation omissions \u2014 Render sandbox groups, snapshots and executions",
	"list-sandbox-snapshots":       "ADR018 \u00a7 Render operation omissions \u2014 Render sandbox groups, snapshots and executions",
	"listFlowRunSteps":             "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"listFlowRuns":                 "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"listFlows":                    "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"listTaskRuns":                 "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"listTasks":                    "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"listWorkflowVersions":         "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"preview-service":              "ADR018 \u00a7 Render operation omissions \u2014 PR previews",
	"retrieve-dedicated-ip":        "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"retrieve-maintenance":         "ADR018 \u00a7 Render operation omissions \u2014 Maintenance scheduling",
	"retrieve-sandbox-build":       "ADR018 \u00a7 Render operation omissions \u2014 Render sandbox groups, snapshots and executions",
	"retrieve-sandbox-snapshot":    "ADR018 \u00a7 Render operation omissions \u2014 Render sandbox groups, snapshots and executions",
	"startFlowRun":                 "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"streamTaskRunsEvents":         "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"trigger-maintenance":          "ADR018 \u00a7 Render operation omissions \u2014 Maintenance scheduling",
	"update-dedicated-ip":          "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"update-maintenance":           "ADR018 \u00a7 Render operation omissions \u2014 Maintenance scheduling",
	"update-owner-log-stream":      "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"update-resource-log-stream":   "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"updateWorkflow":               "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"upsertOwnerMetricsStream":     "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
}

func TestEveryRenderOperationMountedOrDocumented(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	mux := matrixServer(t).restHandler()
	parameter := regexp.MustCompile(`\{[^}]+\}`)
	remaining := make(map[string]string, len(renderOperationOmissions))
	for operation, citation := range renderOperationOmissions {
		remaining[operation] = citation
	}
	for path, item := range contract.document.Paths.Map() {
		for method, operation := range item.Operations() {
			req := httptest.NewRequest(strings.ToUpper(method), "/v1"+parameter.ReplaceAllString(path, "fixture"), nil)
			_, pattern := mux.Handler(req)
			citation, omitted := renderOperationOmissions[operation.OperationID]
			delete(remaining, operation.OperationID)
			// A not-offered route is mounted only to name its refusal; it is
			// still an omission and must keep its ADR018 citation.
			if pattern != "" && !notOfferedRoutes[pattern] {
				if omitted {
					t.Errorf("stale omission for mounted %s", operation.OperationID)
				}
				continue
			}
			if !omitted || !strings.Contains(citation, "ADR018") {
				t.Errorf("missing: %s %s %s", operation.OperationID, method, path)
			}
		}
	}
	for operation := range remaining {
		t.Errorf("omission no longer in Render spec: %s", operation)
	}
}
