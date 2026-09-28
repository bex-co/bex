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
	"add-headers":                          "ADR018 \u00a7 Render operation omissions \u2014 Individual static rule mutations",
	"add-resources-to-environment":         "ADR018 \u00a7 Render operation omissions \u2014 Environment resource mutation paths",
	"add-route":                            "ADR018 \u00a7 Render operation omissions \u2014 Individual static rule mutations",
	"cancelTaskRun":                        "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"create-dedicated-ip":                  "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"create-postgres-user":                 "ADR018 \u00a7 Render operation omissions \u2014 Postgres default credential rotation",
	"create-redis":                         "ADR018 \u00a7 Render operation omissions \u2014 Legacy Redis spelling",
	"createTask":                           "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"createWorkflow":                       "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"createWorkflowVersion":                "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"delete-dedicated-ip":                  "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"delete-header":                        "ADR018 \u00a7 Render operation omissions \u2014 Individual static rule mutations",
	"delete-owner-log-stream":              "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"delete-redis":                         "ADR018 \u00a7 Render operation omissions \u2014 Legacy Redis spelling",
	"delete-resource-log-stream":           "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"delete-route":                         "ADR018 \u00a7 Render operation omissions \u2014 Individual static rule mutations",
	"deleteOwnerMetricsStream":             "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"deleteWorkflow":                       "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"get-owner-log-stream":                 "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"get-resource-log-stream":              "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"get-task-runs-completed":              "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"get-task-runs-queued":                 "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getOwnerMetricsStream":                "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"getTask":                              "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getTaskRun":                           "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getWorkflow":                          "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"getWorkflowVersion":                   "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"list-dedicated-ips":                   "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"list-maintenance":                     "ADR018 \u00a7 Render operation omissions \u2014 Maintenance scheduling",
	"list-organization-audit-logs":         "ADR018 \u00a7 Render operation omissions \u2014 Render audit-log paths",
	"list-redis":                           "ADR018 \u00a7 Render operation omissions \u2014 Legacy Redis spelling",
	"list-resource-log-streams":            "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"listTaskRuns":                         "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"listTasks":                            "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"listWorkflowVersions":                 "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"patch-owner-notification-settings":    "ADR018 \u00a7 Render operation omissions \u2014 Owner notification policy",
	"patch-route":                          "ADR018 \u00a7 Render operation omissions \u2014 Individual static rule mutations",
	"preview-service":                      "ADR018 \u00a7 Render operation omissions \u2014 PR previews",
	"purge-cache":                          "ADR018 \u00a7 Render operation omissions \u2014 Static cache purge",
	"remove-resources-from-environment":    "ADR018 \u00a7 Render operation omissions \u2014 Environment resource mutation paths",
	"remove-workspace-member":              "ADR018 \u00a7 Render operation omissions \u2014 Render member mutation paths",
	"resume-redis":                         "ADR018 \u00a7 Render operation omissions \u2014 Legacy Redis spelling",
	"retrieve-dedicated-ip":                "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"retrieve-maintenance":                 "ADR018 \u00a7 Render operation omissions \u2014 Maintenance scheduling",
	"retrieve-owner-notification-settings": "ADR018 \u00a7 Render operation omissions \u2014 Owner notification policy",
	"retrieve-redis":                       "ADR018 \u00a7 Render operation omissions \u2014 Legacy Redis spelling",
	"retrieve-redis-connection-info":       "ADR018 \u00a7 Render operation omissions \u2014 Legacy Redis spelling",
	"streamTaskRunsEvents":                 "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"suspend-redis":                        "ADR018 \u00a7 Render operation omissions \u2014 Legacy Redis spelling",
	"trigger-maintenance":                  "ADR018 \u00a7 Render operation omissions \u2014 Maintenance scheduling",
	"update-dedicated-ip":                  "ADR018 \u00a7 Render operation omissions \u2014 Dedicated outbound IPs",
	"update-maintenance":                   "ADR018 \u00a7 Render operation omissions \u2014 Maintenance scheduling",
	"update-owner-log-stream":              "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"update-redis":                         "ADR018 \u00a7 Render operation omissions \u2014 Legacy Redis spelling",
	"update-resource-log-stream":           "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
	"update-secret-files-for-service":      "ADR018 \u00a7 Render operation omissions \u2014 Bulk secret-file replacement",
	"update-workspace-member":              "ADR018 \u00a7 Render operation omissions \u2014 Render member mutation paths",
	"updateWorkflow":                       "ADR018 \u00a7 Render operation omissions \u2014 Render Workflows",
	"upsertOwnerMetricsStream":             "ADR018 \u00a7 Render operation omissions \u2014 External telemetry streams",
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
			if pattern != "" {
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
