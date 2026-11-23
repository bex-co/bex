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
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// renderFieldProbe is one served operation and a body the composed stack
// already accepts; the guard adds each property Render declares, one at a time.
type renderFieldProbe struct {
	op, method, path string
	base             map[string]any
	// nested names an object property whose schema (or oneOf variant when
	// variant != "") is probed field by field inside base[nested].
	// When nested == "" and variant != "", variant selects a top-level oneOf
	// branch (or an array item's oneOf branch when isArray is true).
	nested, variant string
	// isArray wraps the probed object inside a single-element JSON array for
	// operations whose request body is an array of objects.
	isArray bool
	// oneOfIndex selects an unnamed inline oneOf branch (1-based) when the
	// pinned schema does not use a named component $ref (e.g. envVarInputArray).
	oneOfIndex int
}

// checkBodyInventory verifies that got covers every served JSON-body operation
// in want with no omissions and no stale entries.
func checkBodyInventory(t *testing.T, label string, want []string, got map[string]bool) {
	t.Helper()
	for _, op := range want {
		if !got[op] {
			t.Errorf("served Render JSON-body operation %q is missing from %s", op, label)
		}
	}
	for op := range got {
		if !slices.Contains(want, op) {
			t.Errorf("%s entry %q is not a served JSON-body operation", label, op)
		}
	}
}

// declaredFieldAllowlist records Render-declared request fields bex answers
// with something other than "handled" or a named refusal, with the reason.
// A generic `unknown field` not listed here fails the guard (w5/m146).
var declaredFieldAllowlist = map[string]string{}

// servedJSONBodyOperations returns the sorted list of served Render operation
// IDs that declare an application/json request body in doc (excluding
// documented renderOperationOmissions).
func servedJSONBodyOperations(doc *openapi3.T) []string {
	var ops []string
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if op.OperationID == "" {
				continue
			}
			if _, omitted := renderOperationOmissions[op.OperationID]; omitted {
				continue
			}
			if op.RequestBody == nil || op.RequestBody.Value == nil || op.RequestBody.Value.Content.Get("application/json") == nil {
				continue
			}
			ops = append(ops, op.OperationID)
		}
	}
	slices.Sort(ops)
	return ops
}

// renderBodyGateServer returns a composed server with optional REST route
// families mounted (sandboxes, registry credentials, build-source env routes)
// and a seeded build-source carrier so every served JSON-body operation reaches
// its strict decoder.
func renderBodyGateServer(t *testing.T, ownerID, buildSourceID string) http.Handler {
	t.Helper()
	bsCarrier := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildSourceID,
			Namespace: "default",
			Labels: map[string]string{
				core.LabelTenant:    ownerID,
				core.LabelWorkspace: ownerID,
				store.LabelAppID:    buildSourceID,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeBuildSource,
			DisplayName: "seeded-bsr",
			Repo:        "https://github.com/render-examples/express-hello-world",
		},
	}
	h, _ := serverWith(t, &core.Base{
		Client:    fakeClient(bsCarrier),
		Namespace: "default",
		Workspace: fakeWorkspace{"client-1": ownerID},
	}, Deps{
		SandboxClient:      &sandbox.Client{},
		RegistryCredsStore: newFakeRCStore(),
		Secrets:            newFakeRCSecretKV(),
	})
	return h
}

// TestRenderDeclaredRequestFieldsAreHandledOrRefusedByName is w5/m146's and
// w5/m582's guard: every request property the pinned OpenAPI declares on every
// served JSON-body operation (and all five serviceDetailsPOST/PATCH variants),
// sent alongside a body the stack accepts, either reaches the handler or
// answers a named refusal — never the strict decoder's generic `unknown field`,
// which tells a Render client nothing.
func TestRenderDeclaredRequestFieldsAreHandledOrRefusedByName(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData(renderOpenAPISource)
	if err != nil {
		t.Fatal(err)
	}
	const (
		srv = "srv-00000000000000000000"
		dpg = "dpg-00000000000000000000"
		red = "red-00000000000000000000"
		dsk = "dsk-00000000000000000000"
		whk = "whk-00000000000000000000"
		blp = "exs-00000000000000000000"
		prj = "prj-00000000000000000000"
		evm = "evm-00000000000000000000"
		evg = "evg-00000000000000000000"
		rgc = "rgc-00000000000000000000"
		rte = "rte-00000000000000000000"
		bsr = "bsr-00000000000000000000"
		sbg = "sbg-00000000000000000000"
		sbx = "sbx-00000000000000000000"
		usr = "usr-00000000000000000000"
		own = "tea-00000000000000000000"
	)
	image := map[string]any{"imagePath": "docker.io/library/nginx:alpine", "ownerId": ""}
	repo := "https://github.com/render-examples/express-hello-world"
	probes := []renderFieldProbe{
		// Services + all 5 serviceDetailsPOST and all 5 serviceDetailsPATCH variants
		{op: "create-service", method: http.MethodPost, path: "/v1/services", base: map[string]any{"type": "web_service", "name": "web2", "ownerId": own, "image": image}},
		{op: "create-service", method: http.MethodPost, path: "/v1/services", base: map[string]any{"type": "web_service", "name": "web-img", "ownerId": own, "image": map[string]any{"imagePath": "docker.io/library/nginx:alpine", "ownerId": ""}}, nested: "image"},
		{op: "create-service", method: http.MethodPost, path: "/v1/services", base: map[string]any{"type": "web_service", "name": "web3", "ownerId": own, "image": image, "serviceDetails": map[string]any{"runtime": "image"}}, nested: "serviceDetails", variant: "webServiceDetailsPOST"},
		{op: "create-service", method: http.MethodPost, path: "/v1/services", base: map[string]any{"type": "private_service", "name": "pserv", "ownerId": own, "image": image, "serviceDetails": map[string]any{"runtime": "image", "plan": "starter"}}, nested: "serviceDetails", variant: "privateServiceDetailsPOST"},
		{op: "create-service", method: http.MethodPost, path: "/v1/services", base: map[string]any{"type": "background_worker", "name": "worker", "ownerId": own, "image": image, "serviceDetails": map[string]any{"runtime": "image"}}, nested: "serviceDetails", variant: "backgroundWorkerDetailsPOST"},
		{op: "create-service", method: http.MethodPost, path: "/v1/services", base: map[string]any{"type": "static_site", "name": "site", "ownerId": own, "repo": repo, "serviceDetails": map[string]any{"publishPath": "public"}}, nested: "serviceDetails", variant: "staticSiteDetailsPOST"},
		{op: "create-service", method: http.MethodPost, path: "/v1/services", base: map[string]any{"type": "cron_job", "name": "cron", "ownerId": own, "image": image, "serviceDetails": map[string]any{"runtime": "image", "schedule": "0 * * * *"}}, nested: "serviceDetails", variant: "cronJobDetailsPOST"},
		{op: "update-service", method: http.MethodPatch, path: "/v1/services/" + srv, base: map[string]any{"name": "web4"}},
		{op: "update-service", method: http.MethodPatch, path: "/v1/services/" + srv, base: map[string]any{"image": map[string]any{"imagePath": "docker.io/library/nginx:alpine", "ownerId": ""}}, nested: "image"},
		{op: "update-service", method: http.MethodPatch, path: "/v1/services/" + srv, base: map[string]any{"serviceDetails": map[string]any{}}, nested: "serviceDetails", variant: "webServiceDetailsPATCH"},
		{op: "update-service", method: http.MethodPatch, path: "/v1/services/" + srv, base: map[string]any{"serviceDetails": map[string]any{}}, nested: "serviceDetails", variant: "privateServiceDetailsPATCH"},
		{op: "update-service", method: http.MethodPatch, path: "/v1/services/" + srv, base: map[string]any{"serviceDetails": map[string]any{}}, nested: "serviceDetails", variant: "backgroundWorkerDetailsPATCH"},
		{op: "update-service", method: http.MethodPatch, path: "/v1/services/" + srv, base: map[string]any{"serviceDetails": map[string]any{}}, nested: "serviceDetails", variant: "staticSiteDetailsPATCH"},
		{op: "update-service", method: http.MethodPatch, path: "/v1/services/" + srv, base: map[string]any{"serviceDetails": map[string]any{}}, nested: "serviceDetails", variant: "cronJobDetailsPATCH"},
		{op: "scale-service", method: http.MethodPost, path: "/v1/services/" + srv + "/scale", base: map[string]any{"numInstances": 2}},
		{op: "autoscale-service", method: http.MethodPut, path: "/v1/services/" + srv + "/autoscaling", base: map[string]any{"enabled": true, "min": 1, "max": 3, "criteria": map[string]any{"cpu": map[string]any{"enabled": true, "percentage": 70}, "memory": map[string]any{"enabled": false, "percentage": 80}}}},
		{op: "create-deploy", method: http.MethodPost, path: "/v1/services/" + srv + "/deploys", base: map[string]any{}},
		{op: "rollback-deploy", method: http.MethodPost, path: "/v1/services/" + srv + "/rollback", base: map[string]any{"deployId": "dep-00000000000000000000"}},
		{op: "create-custom-domain", method: http.MethodPost, path: "/v1/services/" + srv + "/custom-domains", base: map[string]any{"name": "app.example.com"}},
		{op: "post-job", method: http.MethodPost, path: "/v1/services/" + srv + "/jobs", base: map[string]any{"startCommand": "echo hi"}},

		// Service headers, routes, env vars, secret files
		{op: "add-headers", method: http.MethodPost, path: "/v1/services/" + srv + "/headers", base: map[string]any{"path": "/*", "name": "X-Test", "value": "1"}},
		{op: "update-headers", method: http.MethodPut, path: "/v1/services/" + srv + "/headers", base: map[string]any{"path": "/*", "name": "X-Test", "value": "1"}, isArray: true},
		{op: "add-route", method: http.MethodPost, path: "/v1/services/" + srv + "/routes", base: map[string]any{"type": "redirect", "source": "/a", "destination": "/b"}},
		{op: "patch-route", method: http.MethodPatch, path: "/v1/services/" + srv + "/routes/" + rte, base: map[string]any{"priority": 0}},
		{op: "put-routes", method: http.MethodPut, path: "/v1/services/" + srv + "/routes", base: map[string]any{"type": "redirect", "source": "/a", "destination": "/b"}, isArray: true},
		{op: "update-env-var", method: http.MethodPut, path: "/v1/services/" + srv + "/env-vars/FOO", base: map[string]any{"value": "bar"}, variant: "envVarValue"},
		{op: "update-env-var", method: http.MethodPut, path: "/v1/services/" + srv + "/env-vars/FOO", base: map[string]any{"generateValue": true}, variant: "envVarGenerateValue"},
		{op: "update-env-vars-for-service", method: http.MethodPut, path: "/v1/services/" + srv + "/env-vars", base: map[string]any{"key": "FOO", "value": "bar"}, isArray: true, oneOfIndex: 1},
		{op: "update-env-vars-for-service", method: http.MethodPut, path: "/v1/services/" + srv + "/env-vars", base: map[string]any{"key": "FOO", "generateValue": true}, isArray: true, oneOfIndex: 2},
		{op: "add-or-update-secret-file", method: http.MethodPut, path: "/v1/services/" + srv + "/secret-files/ca.pem", base: map[string]any{}},
		{op: "update-secret-files-for-service", method: http.MethodPut, path: "/v1/services/" + srv + "/secret-files", base: map[string]any{"name": "ca.pem", "content": "pem"}, isArray: true},

		// Build sources + build-source env vars, secret files, builds
		{op: "create-build-source", method: http.MethodPost, path: "/v1/build-sources", base: map[string]any{"ownerId": own, "name": "src"}},
		{op: "create-build-source", method: http.MethodPost, path: "/v1/build-sources", base: map[string]any{"ownerId": own, "name": "src-git", "git": map[string]any{"runtime": "docker", "repoUrl": repo}}, nested: "git"},
		{op: "create-build-source", method: http.MethodPost, path: "/v1/build-sources", base: map[string]any{"ownerId": own, "name": "src-img", "image": map[string]any{"ownerId": own, "imageUrl": "docker.io/library/nginx:alpine"}}, nested: "image"},
		{op: "update-build-source", method: http.MethodPatch, path: "/v1/build-sources/" + bsr, base: map[string]any{"name": "src2"}},
		{op: "update-build-source", method: http.MethodPatch, path: "/v1/build-sources/" + bsr, base: map[string]any{"git": map[string]any{}}, nested: "git"},
		{op: "update-build-source", method: http.MethodPatch, path: "/v1/build-sources/" + bsr, base: map[string]any{"image": map[string]any{}}, nested: "image"},
		{op: "trigger-build-source-build", method: http.MethodPost, path: "/v1/build-sources/" + bsr + "/builds", base: map[string]any{}},
		{op: "update-build-source-env-var", method: http.MethodPut, path: "/v1/build-sources/" + bsr + "/env-vars/FOO", base: map[string]any{"value": "bar"}, variant: "envVarValue"},
		{op: "update-build-source-env-var", method: http.MethodPut, path: "/v1/build-sources/" + bsr + "/env-vars/FOO", base: map[string]any{"generateValue": true}, variant: "envVarGenerateValue"},
		{op: "update-env-vars-for-build-source", method: http.MethodPut, path: "/v1/build-sources/" + bsr + "/env-vars", base: map[string]any{"key": "FOO", "value": "bar"}, isArray: true, oneOfIndex: 1},
		{op: "update-env-vars-for-build-source", method: http.MethodPut, path: "/v1/build-sources/" + bsr + "/env-vars", base: map[string]any{"key": "FOO", "generateValue": true}, isArray: true, oneOfIndex: 2},
		{op: "add-or-update-build-source-secret-file", method: http.MethodPut, path: "/v1/build-sources/" + bsr + "/secret-files/ca.pem", base: map[string]any{}},
		{op: "update-secret-files-for-build-source", method: http.MethodPut, path: "/v1/build-sources/" + bsr + "/secret-files", base: map[string]any{"name": "ca.pem", "content": "pem"}, isArray: true},

		// Postgres
		{op: "create-postgres", method: http.MethodPost, path: "/v1/postgres", base: map[string]any{"name": "pg", "plan": "free", "version": "16", "ownerId": own}},
		{op: "update-postgres", method: http.MethodPatch, path: "/v1/postgres/" + dpg, base: map[string]any{"name": "pg2"}},
		{op: "recover-postgres", method: http.MethodPost, path: "/v1/postgres/" + dpg + "/recovery", base: map[string]any{"restoreName": "pg-restored"}},
		{op: "create-postgres-user", method: http.MethodPost, path: "/v1/postgres/" + dpg + "/credentials", base: map[string]any{"username": "appuser"}},

		// Key Value + legacy Redis aliases
		{op: "create-key-value", method: http.MethodPost, path: "/v1/key-value", base: map[string]any{"name": "kv", "plan": "free", "ownerId": own}},
		{op: "update-key-value", method: http.MethodPatch, path: "/v1/key-value/" + red, base: map[string]any{"name": "kv2"}},
		{op: "create-redis", method: http.MethodPost, path: "/v1/redis", base: map[string]any{"name": "kv", "plan": "free", "ownerId": own}},
		{op: "update-redis", method: http.MethodPatch, path: "/v1/redis/" + red, base: map[string]any{"name": "kv2"}},

		// Disks + snapshots
		{op: "add-disk", method: http.MethodPost, path: "/v1/disks", base: map[string]any{"name": "data", "sizeGB": 10, "mountPath": "/data", "serviceId": srv}},
		{op: "update-disk", method: http.MethodPatch, path: "/v1/disks/" + dsk, base: map[string]any{"sizeGB": 20}},
		{op: "restore-snapshot", method: http.MethodPost, path: "/v1/disks/" + dsk + "/snapshots/restore", base: map[string]any{"snapshotKey": "snap-1"}},

		// Environment groups
		{op: "create-env-group", method: http.MethodPost, path: "/v1/env-groups", base: map[string]any{"name": "shared", "ownerId": own}},
		{op: "update-env-group", method: http.MethodPatch, path: "/v1/env-groups/" + evg, base: map[string]any{"name": "shared2"}},
		{op: "update-env-group-env-var", method: http.MethodPut, path: "/v1/env-groups/" + evg + "/env-vars/KEY", base: map[string]any{"value": "v"}, variant: "envVarValue"},
		{op: "update-env-group-env-var", method: http.MethodPut, path: "/v1/env-groups/" + evg + "/env-vars/KEY", base: map[string]any{"generateValue": true}, variant: "envVarGenerateValue"},
		{op: "update-env-group-secret-file", method: http.MethodPut, path: "/v1/env-groups/" + evg + "/secret-files/ca.pem", base: map[string]any{}},

		// Projects + environments
		{op: "create-project", method: http.MethodPost, path: "/v1/projects", base: map[string]any{"name": "proj", "ownerId": own}},
		{op: "update-project", method: http.MethodPatch, path: "/v1/projects/" + prj, base: map[string]any{"name": "proj2"}},
		{op: "create-environment", method: http.MethodPost, path: "/v1/environments", base: map[string]any{"name": "staging", "projectId": prj}},
		{op: "update-environment", method: http.MethodPatch, path: "/v1/environments/" + evm, base: map[string]any{"name": "prod"}},
		{op: "add-resources-to-environment", method: http.MethodPost, path: "/v1/environments/" + evm + "/resources", base: map[string]any{"resourceIds": []string{srv}}},

		// Registry credentials, webhooks, blueprints, notifications, members
		{op: "create-registry-credential", method: http.MethodPost, path: "/v1/registrycredentials", base: map[string]any{"name": "ghcr", "ownerId": own, "host": "ghcr.io", "username": "alice", "authToken": "hunter2"}},
		{op: "update-registry-credential", method: http.MethodPatch, path: "/v1/registrycredentials/" + rgc, base: map[string]any{"name": "renamed"}},
		{op: "create-webhook", method: http.MethodPost, path: "/v1/webhooks", base: map[string]any{"ownerId": own, "name": "hook", "url": "https://example.com/hook", "enabled": true, "eventFilter": []string{}}},
		{op: "update-webhook", method: http.MethodPatch, path: "/v1/webhooks/" + whk, base: map[string]any{"name": "hook2"}},
		{op: "update-blueprint", method: http.MethodPatch, path: "/v1/blueprints/" + blp, base: map[string]any{"name": "stack"}},
		{op: "patch-owner-notification-settings", method: http.MethodPatch, path: "/v1/notification-settings/owners/" + own, base: map[string]any{}},
		{op: "patch-service-notification-overrides", method: http.MethodPatch, path: "/v1/notification-settings/overrides/services/" + srv, base: map[string]any{}},
		{op: "update-workspace-member", method: http.MethodPatch, path: "/v1/owners/" + own + "/members/" + usr, base: map[string]any{"role": "ADMIN"}},

		// Sandboxes, sandbox groups, builds, snapshots, credentials
		{op: "create-sandbox", method: http.MethodPost, path: "/v1/sandboxes", base: map[string]any{"ownerId": own}},
		{op: "create-sandbox", method: http.MethodPost, path: "/v1/sandboxes", base: map[string]any{"ownerId": own, "networkPolicy": map[string]any{"type": "allow-list"}}, nested: "networkPolicy"},
		{op: "create-sandbox-group", method: http.MethodPost, path: "/v1/sandbox-groups", base: map[string]any{"ownerId": own, "name": "grp"}},
		{op: "update-sandbox-group", method: http.MethodPatch, path: "/v1/sandbox-groups/" + sbg, base: map[string]any{"concurrencyLimit": 10}},
		{op: "create-sandbox-build", method: http.MethodPost, path: "/v1/sandbox-groups/" + sbg + "/builds", base: map[string]any{"name": "img", "sourceImage": "alpine:3.20"}},
		{op: "create-sandbox-build", method: http.MethodPost, path: "/v1/sandbox-groups/" + sbg + "/builds", base: map[string]any{"name": "img", "sourceImage": "alpine:3.20", "registryCredential": map[string]any{"username": "u", "password": "p"}}, nested: "registryCredential"},
		{op: "create-sandbox-snapshot", method: http.MethodPost, path: "/v1/sandboxes/" + sbx + "/snapshots", base: map[string]any{}},
		{op: "create-sandbox-group-credential", method: http.MethodPost, path: "/v1/sandbox-groups/" + sbg + "/credentials", base: map[string]any{"id": "cred-1", "domain": "api.example.com", "value": "secret", "injection": map[string]any{"type": "header", "name": "Authorization"}}},
		{op: "create-sandbox-group-credential", method: http.MethodPost, path: "/v1/sandbox-groups/" + sbg + "/credentials", base: map[string]any{"id": "cred-1", "domain": "api.example.com", "value": "secret", "injection": map[string]any{"type": "header"}}, nested: "injection"},
		{op: "update-sandbox-group-credential", method: http.MethodPatch, path: "/v1/sandbox-groups/" + sbg + "/credentials/cred-1", base: map[string]any{}},
		{op: "update-sandbox-group-credential", method: http.MethodPatch, path: "/v1/sandbox-groups/" + sbg + "/credentials/cred-1", base: map[string]any{"injection": map[string]any{"type": "header"}}, nested: "injection"},
		{op: "restore-sandbox-group-credential", method: http.MethodPost, path: "/v1/sandbox-groups/" + sbg + "/credentials/cred-1/restore", base: map[string]any{"versionId": 1}},
	}

	// Inventory guard: every served Render JSON-body operation in the pinned spec
	// must be probed, and every create-service / update-service serviceDetails
	// oneOf variant must be probed.
	servedOps := servedJSONBodyOperations(doc)
	probedOps := map[string]bool{}
	probedVariants := map[string]map[string]bool{}
	for _, p := range probes {
		probedOps[p.op] = true
		if p.nested != "" && p.variant != "" {
			k := p.op + "." + p.nested
			if probedVariants[k] == nil {
				probedVariants[k] = map[string]bool{}
			}
			probedVariants[k][p.variant] = true
		}
	}
	checkBodyInventory(t, "TestRenderDeclaredRequestFieldsAreHandledOrRefusedByName", servedOps, probedOps)
	for _, op := range []string{"create-service", "update-service"} {
		details := renderRequestSchema(t, doc, op).Properties["serviceDetails"].Value
		for _, ref := range details.OneOf {
			parts := strings.Split(ref.Ref, "/")
			variant := parts[len(parts)-1]
			if !probedVariants[op+".serviceDetails"][variant] {
				t.Errorf("%s.serviceDetails variant %q has no probe", op, variant)
			}
		}
	}

	h := renderBodyGateServer(t, own, bsr)

	unprobed := 0
	checkResponse := func(t *testing.T, key string, raw []byte, wCode int, got string) {
		t.Helper()
		switch {
		case strings.Contains(got, "does not match the Render API schema") ||
			strings.Contains(got, "invalid request body at") ||
			strings.Contains(got, "invalid path parameter") ||
			strings.Contains(got, "invalid query parameter") ||
			strings.Contains(got, "OWNER_REQUIRED"):
			// The generated example did not satisfy the validator, so the
			// handler was never reached; counted, not silently passed.
			unprobed++
			t.Logf("unprobed (validator refused %s): %d %s", raw, wCode, got)
		case strings.Contains(got, "unknown field"):
			if reason, ok := declaredFieldAllowlist[key]; ok {
				t.Logf("allowlisted: %s", reason)
				return
			}
			t.Errorf("Render declares %s but bex answers a generic %d %s — handle it or refuse it by name", key, wCode, got)
		}
	}

	// Verify every probe's base body actually reaches the handler's strict JSON
	// decoder by checking a synthetic unknown-field canary on each probe.
	for _, p := range probes {
		schema := renderRequestSchema(t, doc, p.op)
		if p.isArray {
			schema = schema.Items.Value
		}
		if p.nested != "" {
			prop := schema.Properties[p.nested].Value
			if p.variant != "" {
				schema = oneOfVariant(t, doc, prop, p.variant)
			} else {
				schema = prop
			}
		} else if p.variant != "" {
			schema = oneOfVariant(t, doc, schema, p.variant)
		} else if p.oneOfIndex > 0 {
			schema = schema.OneOf[p.oneOfIndex-1].Value
		}

		// Always probe the base body itself so operations whose only properties
		// are required (and therefore present in base) are still verified, and
		// confirm a synthetic unknown field is rejected (proving the handler's
		// strict JSON decoder actually ran).
		baseKey := p.op + ".<base>"
		if p.nested != "" {
			baseKey = p.op + "." + p.nested
			if p.variant != "" {
				baseKey += "(" + p.variant + ")"
			}
			baseKey += ".<base>"
		} else if p.variant != "" {
			baseKey = p.op + "(" + p.variant + ").<base>"
		} else if p.oneOfIndex > 0 {
			baseKey = p.op + "[oneOf]." + string(rune('0'+p.oneOfIndex)) + ".<base>"
		}
		t.Run(baseKey, func(t *testing.T) {
			raw := marshalProbeBody(p.base, p.isArray)
			w := do(t, h, p.method, p.path, testToken, string(raw))
			checkResponse(t, baseKey, raw, w.Code, w.Body.String())

			canary := cloneJSON(p.base)
			dest := canary
			if p.nested != "" {
				dest = canary[p.nested].(map[string]any)
			}
			dest["__bex_declared_field_canary__"] = 1
			canaryRaw := marshalProbeBody(canary, p.isArray)
			cw := do(t, h, p.method, p.path, testToken, string(canaryRaw))
			if cw.Code != http.StatusBadRequest || !strings.Contains(cw.Body.String(), "unknown field") {
				t.Errorf("canary unknown field on %s = %d %s (want 400 naming unknown field — handler did not reach strict JSON decoder!)", baseKey, cw.Code, cw.Body.String())
			}
		})

		for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
			target := p.base
			if p.nested != "" {
				target, _ = p.base[p.nested].(map[string]any)
			}
			if _, inBase := target[name]; inBase {
				continue
			}
			key := p.op + "." + name
			if p.nested != "" {
				if p.variant != "" {
					key = p.op + "." + p.nested + "(" + p.variant + ")." + name
				} else {
					key = p.op + "." + p.nested + "." + name
				}
			} else if p.variant != "" {
				key = p.op + "(" + p.variant + ")." + name
			}
			t.Run(key, func(t *testing.T) {
				body := cloneJSON(p.base)
				dest := body
				if p.nested != "" {
					dest = body[p.nested].(map[string]any)
				}
				dest[name] = exampleFor(schema.Properties[name].Value)
				raw := marshalProbeBody(body, p.isArray)
				w := do(t, h, p.method, p.path, testToken, string(raw))
				checkResponse(t, key, raw, w.Code, w.Body.String())
			})
		}
	}
	// An example the validator refuses never reaches the handler, so a rising
	// count would let the guard pass while checking nothing.
	if unprobed > 0 {
		t.Fatalf("%d declared properties were never probed; fix exampleFor or allowlist them", unprobed)
	}
}

func marshalProbeBody(body map[string]any, isArray bool) []byte {
	if isArray {
		raw, _ := json.Marshal([]any{body})
		return raw
	}
	raw, _ := json.Marshal(body)
	return raw
}

func renderRequestSchema(t *testing.T, doc *openapi3.T, opID string) *openapi3.Schema {
	t.Helper()
	_, op := findRenderOperation(t, doc, opID)
	if op.RequestBody == nil || op.RequestBody.Value.Content.Get("application/json") == nil {
		t.Fatalf("no JSON request body for %s", opID)
	}
	return op.RequestBody.Value.Content.Get("application/json").Schema.Value
}

func oneOfVariant(t *testing.T, doc *openapi3.T, s *openapi3.Schema, name string) *openapi3.Schema {
	t.Helper()
	for _, ref := range s.OneOf {
		if strings.HasSuffix(ref.Ref, "/"+name) {
			return ref.Value
		}
	}
	if len(s.OneOf) == 0 && len(s.Properties) > 0 {
		return s
	}
	t.Fatalf("no oneOf variant %s", name)
	return nil
}

// exampleFor builds a value the pinned schema accepts: its example, its first
// enum value, or a typed placeholder, recursing into objects and arrays.
func exampleFor(s *openapi3.Schema) any {
	if s == nil {
		return "x"
	}
	if s.Example != nil {
		return s.Example
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	if len(s.OneOf) > 0 {
		return exampleFor(s.OneOf[0].Value)
	}
	if len(s.AllOf) > 0 {
		return exampleFor(s.AllOf[0].Value)
	}
	switch {
	case s.Type.Is("boolean"):
		return true
	case s.Type.Is("integer"), s.Type.Is("number"):
		if s.Min != nil {
			return *s.Min
		}
		return 1
	case s.Type.Is("array"):
		if s.Items == nil {
			return []any{}
		}
		return []any{exampleFor(s.Items.Value)}
	case s.Type.Is("object") || len(s.Properties) > 0:
		out := map[string]any{}
		for _, r := range s.Required {
			if prop, ok := s.Properties[r]; ok {
				out[r] = exampleFor(prop.Value)
			}
		}
		return out
	case s.Format == "date-time":
		return "2026-10-20T00:00:00Z"
	case s.Format == "uri":
		return "https://example.com"
	default:
		return "x"
	}
}

func cloneJSON(m map[string]any) map[string]any {
	raw, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
