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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Every Render-gated body operation bex serves accepts bex's own dialect
// through the composed stack: the pinned Render request validator, then the
// real handler's strict decoder (w8/136, expanded across every served JSON-body
// operation in w5/m582). w8/135 found create/update registry-credential
// unreachable because the two demanded contradictory bodies, and only
// production composed them. The handler may still answer 404 or 503 for its
// own reasons here; what must never come back is the validator's schema
// refusal, or the strict decoder refusing a bex field.
//
// The inventory guard derives the served JSON-body set from the runtime
// contract so a newly mounted body operation cannot be omitted.
func TestRenderGatedBexBodiesReachTheirHandlers(t *testing.T) {
	const (
		srv = "srv-00000000000000000000"
		dpg = "dpg-00000000000000000000"
		red = "red-00000000000000000000"
		dsk = "dsk-00000000000000000000"
		whk = "whk-00000000000000000000"
		blp = "blp-00000000000000000000"
		prj = "prj-00000000000000000000"
		evm = "evm-00000000000000000000"
		evg = "evg-00000000000000000000"
		rte = "rte-00000000000000000000"
		bsr = "bsr-00000000000000000000"
		sbg = "sbg-00000000000000000000"
		sbx = "sbx-00000000000000000000"
		usr = "usr-00000000000000000000"
		own = "tea-00000000000000000000"
	)
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ op, method, path, body string }{
		// Blueprints
		{"update-blueprint", http.MethodPatch, "/v1/blueprints/" + blp, `{"name":"stack"}`},
		{"validate-blueprint", http.MethodPost, "/v1/blueprints/validate", `{"bexYaml":"services: []","ownerId":"` + own + `"}`},

		// Disks + snapshots
		{"add-disk", http.MethodPost, "/v1/disks", `{"name":"data","sizeGB":10,"mountPath":"/data","serviceId":"` + srv + `"}`},
		{"update-disk", http.MethodPatch, "/v1/disks/" + dsk, `{"sizeGB":20}`},
		{"restore-snapshot", http.MethodPost, "/v1/disks/" + dsk + "/snapshots/restore", `{"snapshotKey":"snap-1"}`},

		// Notifications + members
		{"patch-owner-notification-settings", http.MethodPatch, "/v1/notification-settings/owners/" + own, `{"emailEnabled":true,"notificationsToSend":"failure"}`},
		{"patch-service-notification-overrides", http.MethodPatch, "/v1/notification-settings/overrides/services/" + srv, `{"notificationsToSend":"default"}`},
		{"update-workspace-member", http.MethodPatch, "/v1/owners/" + own + "/members/" + usr, `{"role":"ADMIN"}`},

		// Registry credentials
		{"create-registry-credential", http.MethodPost, "/v1/registrycredentials", `{"name":"ghcr","ownerId":"` + own + `","host":"ghcr.io","username":"alice","authToken":"hunter2"}`},
		{"update-registry-credential", http.MethodPatch, "/v1/registrycredentials/rgc-ghcr.io", `{"name":"renamed"}`},

		// Services, deploys, scaling, domains, jobs, routes, headers, env vars, secret files
		// image.ownerId is Render-required; the official CLI always sends it, empty.
		{"create-service", http.MethodPost, "/v1/services", `{"type":"web_service","name":"web2","ownerId":"` + own + `","image":{"imagePath":"docker.io/library/nginx:alpine","ownerId":""}}`},
		{"update-service", http.MethodPatch, "/v1/services/" + srv, `{"name":"web3"}`},
		{"scale-service", http.MethodPost, "/v1/services/" + srv + "/scale", `{"numInstances":2}`},
		{"autoscale-service", http.MethodPut, "/v1/services/" + srv + "/autoscaling", `{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":75},"memory":{"enabled":false,"percentage":80}}}`},
		{"create-deploy", http.MethodPost, "/v1/services/" + srv + "/deploys", `{}`},
		{"rollback-deploy", http.MethodPost, "/v1/services/" + srv + "/rollback", `{"deployId":"dep-1"}`},
		{"create-custom-domain", http.MethodPost, "/v1/services/" + srv + "/custom-domains", `{"name":"app.example.com"}`},
		{"post-job", http.MethodPost, "/v1/services/" + srv + "/jobs", `{"startCommand":"echo hi"}`},
		{"add-route", http.MethodPost, "/v1/services/" + srv + "/routes", `{"type":"redirect","source":"/a","destination":"/b"}`},
		{"patch-route", http.MethodPatch, "/v1/services/" + srv + "/routes/" + rte, `{"priority":0}`},
		{"put-routes", http.MethodPut, "/v1/services/" + srv + "/routes", `[{"type":"redirect","source":"/a","destination":"/b"}]`},
		{"add-headers", http.MethodPost, "/v1/services/" + srv + "/headers", `{"path":"/*","name":"X-Test","value":"1"}`},
		{"update-headers", http.MethodPut, "/v1/services/" + srv + "/headers", `[{"path":"/*","name":"X-Test","value":"1"}]`},
		{"update-env-var", http.MethodPut, "/v1/services/" + srv + "/env-vars/FOO", `{"value":"bar"}`},
		{"update-env-vars-for-service", http.MethodPut, "/v1/services/" + srv + "/env-vars", `[{"key":"FOO","value":"bar"}]`},
		{"add-or-update-secret-file", http.MethodPut, "/v1/services/" + srv + "/secret-files/ca.pem", `{"content":"pem"}`},
		{"update-secret-files-for-service", http.MethodPut, "/v1/services/" + srv + "/secret-files", `[{"name":"ca.pem","content":"pem"}]`},

		// Build sources
		{"create-build-source", http.MethodPost, "/v1/build-sources", `{"ownerId":"` + own + `","name":"src","git":{"runtime":"docker","repoUrl":"https://github.com/render-examples/express-hello-world"}}`},
		{"update-build-source", http.MethodPatch, "/v1/build-sources/" + bsr, `{"name":"src2"}`},
		{"trigger-build-source-build", http.MethodPost, "/v1/build-sources/" + bsr + "/builds", `{"clearCache":"do_not_clear"}`},
		{"update-build-source-env-var", http.MethodPut, "/v1/build-sources/" + bsr + "/env-vars/FOO", `{"value":"bar"}`},
		{"update-env-vars-for-build-source", http.MethodPut, "/v1/build-sources/" + bsr + "/env-vars", `[{"key":"FOO","value":"bar"}]`},
		{"add-or-update-build-source-secret-file", http.MethodPut, "/v1/build-sources/" + bsr + "/secret-files/ca.pem", `{"content":"pem"}`},
		{"update-secret-files-for-build-source", http.MethodPut, "/v1/build-sources/" + bsr + "/secret-files", `[{"name":"ca.pem","content":"pem"}]`},

		// Key Value + legacy Redis aliases
		{"create-key-value", http.MethodPost, "/v1/key-value", `{"name":"kv","ownerId":"` + own + `","plan":"free","public":true}`},
		{"update-key-value", http.MethodPatch, "/v1/key-value/" + red, `{"name":"kv2"}`},
		{"create-redis", http.MethodPost, "/v1/redis", `{"name":"kvr","ownerId":"` + own + `","plan":"free","public":true}`},
		{"update-redis", http.MethodPatch, "/v1/redis/" + red, `{"name":"kvr2"}`},

		// Postgres
		{"create-postgres", http.MethodPost, "/v1/postgres", `{"name":"pg","ownerId":"` + own + `","plan":"free","version":"16","public":true}`},
		{"update-postgres", http.MethodPatch, "/v1/postgres/" + dpg, `{"name":"pg2"}`},
		{"recover-postgres", http.MethodPost, "/v1/postgres/" + dpg + "/recovery", `{"restoreName":"pg-restored","environmentId":"` + evm + `"}`},
		{"create-postgres-user", http.MethodPost, "/v1/postgres/" + dpg + "/credentials", `{"username":"appuser"}`},

		// Projects + environments + env groups
		{"create-project", http.MethodPost, "/v1/projects", `{"name":"proj","ownerId":"` + own + `"}`},
		{"update-project", http.MethodPatch, "/v1/projects/" + prj, `{"name":"proj2"}`},
		{"create-environment", http.MethodPost, "/v1/environments", `{"name":"staging","projectId":"` + prj + `"}`},
		{"update-environment", http.MethodPatch, "/v1/environments/" + evm, `{"name":"prod"}`},
		{"add-resources-to-environment", http.MethodPost, "/v1/environments/" + evm + "/resources", `{"resourceIds":["` + srv + `"]}`},
		{"create-env-group", http.MethodPost, "/v1/env-groups", `{"name":"shared","ownerId":"` + own + `"}`},
		{"update-env-group", http.MethodPatch, "/v1/env-groups/" + evg, `{"name":"shared2"}`},
		{"update-env-group-env-var", http.MethodPut, "/v1/env-groups/" + evg + "/env-vars/KEY", `{"value":"v"}`},
		{"update-env-group-secret-file", http.MethodPut, "/v1/env-groups/" + evg + "/secret-files/ca.pem", `{"content":"c"}`},

		// Webhooks
		{"create-webhook", http.MethodPost, "/v1/webhooks", `{"name":"hook","url":"https://example.com/hook"}`},
		{"update-webhook", http.MethodPatch, "/v1/webhooks/" + whk, `{"name":"hook2"}`},

		// Sandboxes, sandbox groups, builds, snapshots, credentials, connect-run
		{"create-sandbox", http.MethodPost, "/v1/sandboxes", `{"ownerId":"` + own + `","plan":"1c-2g"}`},
		{"connect-sandbox-run", http.MethodPost, "/v1/sandboxes/" + sbx + "/runs/exec/token", `{"command":"echo hi","ownerId":"` + own + `"}`},
		{"create-sandbox-group", http.MethodPost, "/v1/sandbox-groups", `{"ownerId":"` + own + `","name":"grp","environmentId":"` + evm + `"}`},
		{"update-sandbox-group", http.MethodPatch, "/v1/sandbox-groups/" + sbg, `{"concurrencyLimit":10}`},
		{"create-sandbox-build", http.MethodPost, "/v1/sandbox-groups/" + sbg + "/builds", `{"name":"img","sourceImage":"alpine:3.20"}`},
		{"create-sandbox-snapshot", http.MethodPost, "/v1/sandboxes/" + sbx + "/snapshots", `{"kind":"filesystem","name":"snap-1"}`},
		{"create-sandbox-group-credential", http.MethodPost, "/v1/sandbox-groups/" + sbg + "/credentials", `{"id":"cred-1","domain":"api.example.com","value":"secret","injection":{"type":"header","name":"Authorization"}}`},
		{"update-sandbox-group-credential", http.MethodPatch, "/v1/sandbox-groups/" + sbg + "/credentials/cred-1", `{"enabled":false}`},
		{"restore-sandbox-group-credential", http.MethodPost, "/v1/sandbox-groups/" + sbg + "/credentials/cred-1/restore", `{"versionId":1}`},
	}

	// Inventory guard: every served Render JSON-body operation in the runtime
	// contract (including body extensions like validate-blueprint JSON and
	// connect-sandbox-run) must appear in the table.
	servedOps := servedJSONBodyOperations(contract.document)
	covered := map[string]bool{}
	for _, tc := range cases {
		covered[tc.op] = true
	}
	checkBodyInventory(t, "TestRenderGatedBexBodiesReachTheirHandlers", servedOps, covered)

	h := renderBodyGateServer(t, own, bsr)
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			w := do(t, h, tc.method, tc.path, testToken, tc.body)
			body := w.Body.String()
			for _, refusal := range []string{
				"invalid request body at",
				"does not match the Render API schema",
				"request body is not allowed",
				"invalid path parameter",
				"invalid query parameter",
				"unknown field",
				"OWNER_REQUIRED",
			} {
				if strings.Contains(body, refusal) {
					t.Fatalf("%s %s with a bex body = %d %s: the composed stack refused it (%q)", tc.method, tc.path, w.Code, body, refusal)
				}
			}
		})
	}
}

// Key Value's maxmemoryPolicy and persistenceMode take either separator: the
// handler normalizes Render's underscore and bex's hyphen alike, as GraphQL and
// MCP do. The pinned enums admitted only underscores, so REST alone refused
// `allkeys-lru` (w8/139). Responses stay underscore, and a genuinely unknown
// value is still refused.
func TestKeyValueEnumsAcceptBothSeparatorsOverREST(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	type kv struct {
		ID      string
		Options struct{ MaxmemoryPolicy, PersistenceMode string }
	}
	read := func(w *httptest.ResponseRecorder, want int) kv {
		t.Helper()
		var got kv
		if err := json.Unmarshal(w.Body.Bytes(), &got); w.Code != want || err != nil {
			t.Fatalf("= %d %s, want %d", w.Code, w.Body.String(), want)
		}
		return got
	}
	created := read(do(t, h, http.MethodPost, "/v1/key-value", testToken, `{"name":"kvh","ownerId":"default","plan":"starter","maxmemoryPolicy":"allkeys-lru"}`), http.StatusCreated)
	if created.Options.MaxmemoryPolicy != "allkeys_lru" {
		t.Fatalf("created maxmemoryPolicy = %q, want Render's allkeys_lru", created.Options.MaxmemoryPolicy)
	}
	read(do(t, h, http.MethodPost, "/v1/key-value", testToken, `{"name":"kvu","ownerId":"default","plan":"starter","maxmemoryPolicy":"allkeys_lru"}`), http.StatusCreated)
	patched := read(do(t, h, http.MethodPatch, "/v1/key-value/"+created.ID, testToken, `{"persistenceMode":"journal-snapshot","maxmemoryPolicy":"volatile-ttl"}`), http.StatusOK)
	if patched.Options.PersistenceMode != "journal_snapshot" || patched.Options.MaxmemoryPolicy != "volatile_ttl" {
		t.Fatalf("patched = %+v, want journal_snapshot and volatile_ttl", patched)
	}
	// A value neither spelling names is refused with the Service's own 400,
	// which lists Render's spellings (w32/051): the gate named only the field.
	const policies = "noeviction | allkeys_lru | allkeys_lfu | volatile_lru | volatile_lfu | allkeys_random | volatile_random | volatile_ttl"
	for _, tc := range []struct{ method, path, body, want string }{
		{http.MethodPost, "/v1/key-value", `{"name":"kvb","ownerId":"default","plan":"free","maxmemoryPolicy":"bogus-w8-139"}`,
			`bad request: unknown maxmemoryPolicy "bogus-w8-139" (valid: ` + policies + `)`},
		{http.MethodPost, "/v1/key-value", `{"name":"kvb","ownerId":"default","plan":"free","persistenceMode":"journal-bogus"}`,
			`bad request: unknown persistenceMode "journal-bogus" (valid: journal_snapshot | snapshot | off)`},
		{http.MethodPatch, "/v1/key-value/" + created.ID, `{"persistenceMode":"aof"}`,
			`bad request: unknown persistenceMode "aof" (valid: journal_snapshot | snapshot | off)`},
		{http.MethodPatch, "/v1/key-value/" + created.ID, `{"maxmemoryPolicy":"lru"}`,
			`bad request: unknown maxmemoryPolicy "lru" (valid: ` + policies + `)`},
		// Render's legacy /v1/redis routes validate as the key-value operations.
		{http.MethodPatch, "/v1/redis/" + created.ID, `{"persistenceMode":""}`,
			`bad request: unknown persistenceMode "" (valid: journal_snapshot | snapshot | off)`},
		// A value that is not a string keeps the gate's pointer.
		{http.MethodPost, "/v1/key-value", `{"name":"kvb","ownerId":"default","plan":"free","persistenceMode":5}`,
			`bad request: invalid request body at /persistenceMode`},
	} {
		w := do(t, h, tc.method, tc.path, testToken, tc.body)
		var refusal struct{ Message string }
		if err := json.Unmarshal(w.Body.Bytes(), &refusal); w.Code != http.StatusBadRequest || err != nil || refusal.Message != tc.want {
			t.Errorf("%s %s %s = %d %s, want 400 %q", tc.method, tc.path, tc.body, w.Code, w.Body.String(), tc.want)
		}
	}
}

// Render's postgresPATCHInput.readReplicas reaches bex's handler through the
// composed stack and replaces the set (w5/m138). The pinned item schema also
// allows a per-replica parameterOverrides, which bex cannot honor — its named
// replicas share one reader pool — so the strict decoder refuses it by name
// rather than dropping it, and a name bex cannot build a hostname from is a
// named 400.
func TestPostgresReadReplicasPatchThroughComposedStack(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	w := do(t, h, http.MethodPost, "/v1/postgres", testToken, `{"name":"pgr","ownerId":"default","plan":"basic_1gb","diskSizeGB":10}`)
	var created struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &created); w.Code != http.StatusCreated || err != nil || created.ID == "" {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	path := "/v1/postgres/" + created.ID
	w = do(t, h, http.MethodPatch, path, testToken, `{"readReplicas":[{"name":"r1"}]}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"name":"r1"`) {
		t.Fatalf("PATCH readReplicas = %d %s", w.Code, w.Body.String())
	}
	w = do(t, h, http.MethodPatch, path, testToken, `{"readReplicas":[{"name":"r1","parameterOverrides":{"work_mem":"4MB"}}]}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `unknown field \"parameterOverrides\"`) {
		t.Fatalf("per-replica parameterOverrides = %d %s, want the strict decoder's named 400", w.Code, w.Body.String())
	}
	w = do(t, h, http.MethodPatch, path, testToken, `{"readReplicas":[{"name":"Reader_2"}]}`)
	// Render's error body carries no bex code (w8/411); the message names it.
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `read replica name \"Reader_2\" must be`) ||
		strings.Contains(w.Body.String(), `"code"`) {
		t.Fatalf("invalid replica name = %d %s, want a named 400 without a code", w.Code, w.Body.String())
	}
}

// TestRecoverPostgresEnvironmentIDThroughComposedStack exercises Render's
// POST /v1/postgres/{id}/recovery with environmentId through the full OpenAPI
// request gate, strict JSON decoder, and response conformance wrapper (w5/m582).
func TestRecoverPostgresEnvironmentIDThroughComposedStack(t *testing.T) {
	const (
		ws = "tea-recover000000000000"
		db = "dpg-recover000000000000"
	)
	peStore := newConformProjectEnvStore()
	src := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:        db,
			Namespace:   "default",
			Labels:      core.TenantLabels(ws),
			Annotations: map[string]string{resourcemeta.AnnotRegion: "frankfurt"},
		},
		Spec: appv1alpha1.DatabaseSpec{
			Name:    "orders",
			Plan:    "basic-256mb",
			Version: "16",
		},
		Status: appv1alpha1.DatabaseStatus{
			Phase:          appv1alpha1.DBPhaseReady,
			BackupsEnabled: true,
		},
	}
	objStore := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "barmancloud.cnpg.io/v1",
			"kind":       "ObjectStore",
			"metadata": map[string]any{
				"name":      "bex-tenant-postgres",
				"namespace": "default",
			},
			"status": map[string]any{
				"serverRecoveryWindow": map[string]any{
					db: map[string]any{
						"firstRecoverabilityPoint": "2026-08-01T00:00:00Z",
						"lastSuccessfulBackupTime": "2026-08-02T00:00:00Z",
					},
				},
			},
		},
	}
	objStore.SetGroupVersionKind(schema.GroupVersionKind{Group: "barmancloud.cnpg.io", Version: "v1", Kind: "ObjectStore"})

	wsStore := newFakeWSStore()
	wsStore.tenants = []store.Tenant{{ID: ws, Name: "Acme", Plan: store.PlanPro}}
	wsStore.members[ws] = []store.TenantMember{{TenantID: ws, Subject: "client-1", Role: "admin"}}

	h, _ := serverWith(t,
		&core.Base{
			Client:    fakeClient(src, objStore),
			Namespace: "default",
			Workspace: fakeWorkspace{"client-1": ws},
			Plans:     proPlan(),
		},
		Deps{
			ProjectsStore:     peStore,
			EnvironmentsStore: peStore,
			WorkspaceStore:    wsStore,
			Identities:        metadataIdentities{},
			Region:            "fsn1",
		},
	)

	// Create a project and target environment through the composed server.
	w := do(t, h, http.MethodPost, "/v1/projects", testToken, `{"name":"proj","ownerId":"`+ws+`"}`)
	var proj struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &proj); w.Code != http.StatusCreated || err != nil || proj.ID == "" {
		t.Fatalf("create project = %d %s", w.Code, w.Body.String())
	}
	w = do(t, h, http.MethodPost, "/v1/environments", testToken, `{"name":"staging","projectId":"`+proj.ID+`"}`)
	var env struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &env); w.Code != http.StatusCreated || err != nil || env.ID == "" {
		t.Fatalf("create environment = %d %s", w.Code, w.Body.String())
	}

	// Recover into that environment through Render's POST /v1/postgres/{id}/recovery.
	w = do(t, h, http.MethodPost, "/v1/postgres/"+db+"/recovery", testToken,
		`{"restoreName":"orders-restored","environmentId":"`+env.ID+`"}`)
	var recovered struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Region        string `json:"region"`
		EnvironmentID string `json:"environmentId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &recovered); w.Code != http.StatusOK || err != nil {
		t.Fatalf("POST /recovery = %d %s", w.Code, w.Body.String())
	}
	if recovered.Name != "orders-restored" || recovered.EnvironmentID != env.ID || recovered.Region != "frankfurt" {
		t.Fatalf("recovered = %+v, want name=orders-restored environmentId=%q region=frankfurt", recovered, env.ID)
	}

	// Datadog fields remain refused by name through the composed stack.
	w = do(t, h, http.MethodPost, "/v1/postgres/"+db+"/recovery", testToken,
		`{"restoreName":"orders-dd","datadogApiKey":"k"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "recovery field datadogApiKey is unsupported") {
		t.Fatalf("datadogApiKey = %d %s, want named 400", w.Code, w.Body.String())
	}
}
