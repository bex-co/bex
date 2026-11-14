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

import "testing"

// Render's list operations take limitParam ("Defaults to 20", 1-100), and an
// unpaged call answers that first page with a cursor to continue (user
// decision 2026-11-13, w22/007, w10/m377). Every operation in the pinned
// contract that accepts `limit` and that bex serves is named in
// unpagedListPages with where its unpaged page of 20 comes from, or in
// unpagedListExemptions; renderOperationOmissions covers the ones bex does not
// serve, so a re-pin that adds a list fails here until it is classified. Only
// the six lists w10/m377 switched cite a behavior test; the rest cite the
// helper the m377 inventory found. GraphQL and MCP keep their own defaults
// (ADR006).
var unpagedListPages = map[string]string{
	// Switched by w10/m377, each with a behavior test.
	"list-sandboxes":   "sandbox TestListLimitIsClampedToOneHundredAndAnUnpagedListGetsRendersFirstPage",
	"list-routes":      "apps TestRESTUnpagedStaticRulesAreRendersFirstPage",
	"list-headers":     "apps TestRESTUnpagedStaticRulesAreRendersFirstPage",
	"list-deploys":     "deploys TestRESTUnpagedDeployListIsRendersFirstPage",
	"list-job":         "jobs TestListJobsFilterEdgeCases (an absent limit reaches the store as 20)",
	"list-logs-values": "logs TestRESTRequestLogsAndDiscovery (20 of 25 values unpaged)",
	// Already Render's page of 20 (core.PageParams, PageLimitOrDefault,
	// PageLimitOrAbsent or the store's default page), w10/m377 t001 inventory.
	"list-services":                      "apps registerServiceRoutes, core.PageParams",
	"list-blueprints":                    "apps registerBlueprintRoutes, core.PageParams",
	"list-blueprint-syncs":               "apps ListBlueprintSyncs, core.PageLimitOrAbsent",
	"list-disks":                         "apps disks REST, core.PageParams",
	"list-owners":                        "workspaces RegisterREST, core.PageParams",
	"list-owner-audit-logs":              "audit RegisterREST, store.DefaultAuditPageSize (auditLogLimitParam, max 1000)",
	"list-notification-overrides":        "apps notification overrides, core.PageParams",
	"list-registry-credentials":          "registrycreds RegisterREST, core.PageParams",
	"get-env-vars-for-service":           "secrets listPaged",
	"list-secret-files-for-service":      "secrets listPaged",
	"get-env-vars-for-build-source":      "secrets listPaged",
	"list-secret-files-for-build-source": "secrets listPaged",
	"list-events":                        "events RegisterREST, core.PageParams",
	"list-custom-domains":                "apps registerDomainRoutes, core.PageParams",
	"list-logs":                          "logs Query, the default page",
	"list-key-value":                     "keyvalue handleListKeyValues, core.PageParams",
	"list-redis":                         "keyvalue handleListKeyValues, core.PageParams",
	"list-postgres":                      "postgres RegisterREST, core.PageParams",
	"list-projects":                      "projects RegisterREST, core.PageParams",
	"list-environments":                  "environments RegisterREST, core.PageParams",
	"list-env-groups":                    "envgroups RegisterREST, core.PageParams",
	"list-build-sources":                 "apps build sources REST, core.PageParams",
	"list-builds-in-build-source":        "deploys build sources REST, core.PageParams",
	"list-webhooks":                      "webhooks RegisterREST, core.PageParams",
	"list-webhook-events":                "webhooks RegisterREST, core.PageParams",
	"list-sandbox-executions":            "sandbox RegisterREST, core.PageParams",
	"list-sandbox-builds":                "sandbox images REST, core.PageParams",
	"list-sandbox-groups":                "sandbox groups REST, core.PageParams",
}

// unpagedListExemptions accept `limit` but answer no page.
var unpagedListExemptions = map[string]string{
	"subscribe-logs": "a log stream: limit bounds the backlog, not a page",
	"listWorkflows":  "a stub that answers [] (Render Workflows are an omission)",
}

func TestEveryServedRenderListIsClassified(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	// Every operation that accepts `limit`, whether by the shared limitParam,
	// auditLogLimitParam or an inline definition (list-sandbox-groups).
	pinned := map[string]bool{}
	for op, query := range contract.allowedQuery {
		if _, ok := query["limit"]; ok && op.OperationID != "" {
			pinned[op.OperationID] = true
		}
	}
	if len(pinned) == 0 {
		t.Fatal("found no list operation in the pinned contract")
	}
	for op := range pinned {
		_, omitted := renderOperationOmissions[op]
		_, paged := unpagedListPages[op]
		_, exempt := unpagedListExemptions[op]
		classified := paged || exempt
		switch {
		case omitted && classified:
			t.Errorf("%s is both an omission and classified here", op)
		case !omitted && !classified:
			t.Errorf("served list %s has no unpaged-page evidence: an unpaged call must answer Render's first page of 20 (w10/m377)", op)
		}
	}
	for _, table := range []map[string]string{unpagedListPages, unpagedListExemptions} {
		for op := range table {
			if !pinned[op] {
				t.Errorf("%s does not take a limit in the pinned contract any more; drop it", op)
			}
		}
	}
}
