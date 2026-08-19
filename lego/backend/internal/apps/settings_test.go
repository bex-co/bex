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

package apps

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// settings_test.go covers w1/m78's collapse of the twice-maintained service
// patch op tables (rest.go patchService + mcp.go applyServicePatch) into the
// single servicePatchTable behind ApplyServicePatch: the order pin, the
// field-completeness guard, and the cross-surface equivalence the old
// structure made impossible to assert.

// TestServicePatchTableOrderIsRESTApplicationOrder pins servicePatchTable to
// REST's application order — the order MCP's tool always declared canonical.
// Each row is identified by the ServicePatch fields it owns. Reordering,
// dropping, or merging rows fails here; this is the field-order guard both
// adapters rely on (w1/m78/t001).
func TestServicePatchTableOrderIsRESTApplicationOrder(t *testing.T) {
	want := [][]string{
		{"DisplayName"},
		{"Repo", "Image", "ImageOwnerID", "Branch", "RegistryCredentialID"},
		{"MaintenanceBeforeFreeDowngrade"}, // early maintenance on a free-downgrade (both surfaces)
		{"Plan"},
		{"IdleTTLSeconds"},
		{"MaxShutdownDelaySeconds"},
		{"RootDir"},
		{"BuildFilter"},
		{"AutoDeploy"},
		{"Schedule", "Command"},
		{"HealthCheckPath"},
		{"PreDeployCommand"},
		{"PublishPath"},
		{"BuildCommand", "StartCommand"},
		{"DockerfilePath"},
		{"NotifyOnFail"},
		{"NotificationsToSend"}, // MCP-only fill
		{"RenderSubdomainPolicy"},
		{"IPAllowList"},
		{"MaintenanceMode"}, // the normal (late) maintenance position
		{"Autoscaling"},     // MCP-only fill
	}
	if len(servicePatchTable) != len(want) {
		t.Fatalf("servicePatchTable has %d rows, want %d", len(servicePatchTable), len(want))
	}
	for i, row := range servicePatchTable {
		if !slices.Equal(row.fields, want[i]) {
			t.Errorf("servicePatchTable[%d] owns %v, want %v — the order is REST's application order and is part of the cross-surface contract",
				i, row.fields, want[i])
		}
	}
}

// TestServicePatchTableCoversEveryFieldExactlyOnce is the completeness guard
// (w1/m78/t006): every ServicePatch field must be owned by exactly one table
// row, so a field added to the type cannot be forgotten in the table (it would
// silently apply on neither surface), and no row can claim a field that no
// longer exists.
func TestServicePatchTableCoversEveryFieldExactlyOnce(t *testing.T) {
	owned := map[string]int{}
	for i, row := range servicePatchTable {
		if len(row.fields) == 0 {
			t.Errorf("servicePatchTable[%d] owns no fields — every row must name what it consumes", i)
		}
		if row.present == nil || row.apply == nil {
			t.Errorf("servicePatchTable[%d] (%v) is missing its present/apply func", i, row.fields)
		}
		for _, f := range row.fields {
			owned[f]++
		}
	}
	typ := reflect.TypeOf(ServicePatch{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		switch owned[name] {
		case 1: // covered exactly once
		case 0:
			t.Errorf("ServicePatch.%s is not owned by any servicePatchTable row — a patch setting that no surface can apply", name)
		default:
			t.Errorf("ServicePatch.%s is owned by %d servicePatchTable rows, want exactly 1", name, owned[name])
		}
		delete(owned, name)
	}
	for name := range owned {
		t.Errorf("servicePatchTable owns %q, which is not a ServicePatch field", name)
	}
}

// TestServicePatchCrossSurfaceEquivalence drives the same multi-field patch
// through PATCH /v1/services/{id} and update_service on identical fixtures and
// requires (a) the identical resulting CR spec and (b) the identical verb
// APPLICATION ORDER, observed through the audit trail each write verb records.
// The old twin-table structure could only compare specs field by field; the
// order comparison is what w1/m78's single table makes assertable (t006).
func TestServicePatchCrossSurfaceEquivalence(t *testing.T) {
	// Every field both surfaces can spell (the three documented routing
	// divergences are single-surface by design and therefore excluded —
	// repo/image REST-only, notificationsToSend/autoscaling MCP-only):
	// identity, source, billing plan, runtime, build, delivery, networking,
	// notifications, and maintenance in one call.
	mcpArgs := map[string]any{
		"serviceId":               "web",
		"displayName":             "Combined",
		"branch":                  "release",
		"plan":                    "standard",
		"maxShutdownDelaySeconds": 90,
		"rootDir":                 "services/other",
		"buildFilter":             map[string]any{"paths": []string{"api/**"}, "ignoredPaths": []string{"docs/**"}},
		"autoDeploy":              false,
		"healthCheckPath":         "/ready",
		"preDeployCommand":        "bin/seed",
		"buildCommand":            "make ci",
		"startCommand":            "bin/combined",
		"dockerfilePath":          "docker/Dockerfile.ci",
		"notifyOnFail":            "ignore",
		"renderSubdomainPolicy":   "disabled",
		"ipAllowList":             []map[string]any{{"cidrBlock": "198.51.100.0/24", "description": "vpn"}},
		"maintenanceMode":         map[string]any{"enabled": true, "uri": "https://status.example.com/down"},
	}
	restBody := `{
		"name": "Combined",
		"branch": "release",
		"rootDir": "services/other",
		"buildFilter": {"paths": ["api/**"], "ignoredPaths": ["docs/**"]},
		"autoDeploy": "no",
		"notifyOnFail": "ignore",
		"renderSubdomainPolicy": "disabled",
		"serviceDetails": {
			"plan": "standard",
			"maxShutdownDelaySeconds": 90,
			"healthCheckPath": "/ready",
			"preDeployCommand": "bin/seed",
			"maintenanceMode": {"enabled": true, "uri": "https://status.example.com/down"},
			"ipAllowList": [{"cidrBlock": "198.51.100.0/24", "description": "vpn"}],
			"envSpecificDetails": {"buildCommand": "make ci", "dockerCommand": "bin/combined", "dockerfilePath": "docker/Dockerfile.ci"}
		}
	}`

	mcpSvc, _ := populatedService(t)
	mcpSink := &captureAuditSink{}
	mcpSvc.Base.Audit = mcpSink
	call, cleanup := appsMCPClient(t, mcpSvc)
	defer cleanup()
	call("update_service", mcpArgs)
	viaMCP := getApp(t, mcpSvc.Client, "web").Spec

	restSvc, _ := populatedService(t)
	restSink := &captureAuditSink{}
	restSvc.Base.Audit = restSink
	mux := http.NewServeMux()
	restSvc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(restBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST PATCH => %d: %s", rec.Code, rec.Body)
	}
	viaREST := getApp(t, restSvc.Client, "web").Spec

	// (a) Identical resulting spec. restartedAt is a timestamp; compare its
	// emptiness (did the patch trigger a build?) rather than the instant.
	mcpRebuilt, restRebuilt := viaMCP.RestartedAt != "", viaREST.RestartedAt != ""
	viaMCP.RestartedAt, viaREST.RestartedAt = "", ""
	if mcpRebuilt != restRebuilt {
		t.Errorf("build trigger differs: MCP rebuilt=%v, REST rebuilt=%v", mcpRebuilt, restRebuilt)
	}
	if !reflect.DeepEqual(viaMCP, viaREST) {
		mcpJSON, _ := json.Marshal(viaMCP)
		restJSON, _ := json.Marshal(viaREST)
		t.Errorf("MCP and REST disagree on the resulting spec:\n  MCP:  %s\n  REST: %s", mcpJSON, restJSON)
	}

	// (b) Identical application order. Every queued verb records its allowed
	// write on the audit trail synchronously on the verb's own path, so the
	// verb sequence IS the application order.
	verbs := func(events []core.AuditEvent) []string {
		out := make([]string, len(events))
		for i, ev := range events {
			out[i] = ev.Verb
		}
		return out
	}
	mcpVerbs, restVerbs := verbs(mcpSink.events), verbs(restSink.events)
	if !slices.Equal(mcpVerbs, restVerbs) {
		t.Errorf("application order differs between surfaces:\n  MCP:  %v\n  REST: %v", mcpVerbs, restVerbs)
	}

	// And the shared order is the table's order, not merely mutually equal:
	// the fifteen ops queued above must appear as an ordered subsequence
	// (maintenance additionally records its field-level effects, which ride
	// between — tolerated, as long as both surfaces agree above).
	expected := []string{
		"apps.SetDisplayName",
		"apps.SetSourceAndRegistryCredential",
		"apps.SetPlan",
		"apps.SetMaxShutdownDelay",
		"apps.SetRootDir",
		"apps.SetBuildFilter",
		"apps.SetAutoDeploy",
		"apps.SetHealthCheckPath",
		"apps.SetPreDeployCommand",
		"apps.SetCommands",
		"apps.SetDockerfilePath",
		"apps.SetNotifyOnFail",
		"apps.SetSubdomainPolicy",
		"apps.SetIPAllowList",
		"apps.SetMaintenanceMode",
	}
	next := 0
	for _, v := range mcpVerbs {
		if next < len(expected) && v == expected[next] {
			next++
		}
	}
	if next != len(expected) {
		t.Errorf("audited verb order %v does not contain the table order %v (matched %d/%d)",
			mcpVerbs, expected, next, len(expected))
	}
}
