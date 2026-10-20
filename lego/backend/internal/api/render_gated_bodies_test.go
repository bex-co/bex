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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Every Render-gated body operation bex serves accepts bex's own dialect
// through the composed stack: the pinned Render request validator, then the
// real handler's strict decoder (w8/136). w8/135 found create/update
// registry-credential unreachable because the two demanded contradictory
// bodies, and only production composed them. The handler may still answer
// 404 or 503 for its own reasons here; what must never come back is the
// validator's schema refusal, or the strict decoder refusing a bex field.
//
// The table is every Render operation with a JSON request body that bex serves
// (28 after w8/137). A body operation bex starts serving belongs here too.
func TestRenderGatedBexBodiesReachTheirHandlers(t *testing.T) {
	const (
		srv = "srv-00000000000000000000"
		dsk = "dsk-00000000000000000000"
		whk = "whk-00000000000000000000"
		blp = "blp-00000000000000000000"
	)
	h, _ := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"},
		Deps{RegistryCredsStore: newFakeRCStore(), Secrets: newFakeRCSecretKV()})
	for _, tc := range []struct{ op, method, path, body string }{
		{"update-blueprint", http.MethodPatch, "/v1/blueprints/" + blp, `{"name":"stack"}`},
		{"add-disk", http.MethodPost, "/v1/disks", `{"name":"data","sizeGB":10,"mountPath":"/data","serviceId":"` + srv + `"}`},
		{"update-disk", http.MethodPatch, "/v1/disks/" + dsk, `{"sizeGB":20}`},
		{"restore-snapshot", http.MethodPost, "/v1/disks/" + dsk + "/snapshots/restore", `{"snapshotKey":"snap-1"}`},
		{"patch-service-notification-overrides", http.MethodPatch, "/v1/notification-settings/overrides/services/" + srv, `{"notificationsToSend":"default"}`},
		{"create-registry-credential", http.MethodPost, "/v1/registrycredentials", `{"name":"ghcr","host":"ghcr.io","username":"alice","authToken":"hunter2"}`},
		{"update-registry-credential", http.MethodPatch, "/v1/registrycredentials/rgc-ghcr.io", `{"name":"renamed"}`},
		// image.ownerId is Render-required; the official CLI always sends it, empty.
		{"create-service", http.MethodPost, "/v1/services", `{"type":"web_service","name":"web2","image":{"imagePath":"docker.io/library/nginx:alpine","ownerId":""}}`},
		{"update-service", http.MethodPatch, "/v1/services/" + srv, `{"name":"web3"}`},
		{"create-deploy", http.MethodPost, "/v1/services/" + srv + "/deploys", `{}`},
		{"rollback-deploy", http.MethodPost, "/v1/services/" + srv + "/rollback", `{"deployId":"dep-1"}`},
		{"create-key-value", http.MethodPost, "/v1/key-value", `{"name":"kv","plan":"free"}`},
		{"update-key-value", http.MethodPatch, "/v1/key-value/red-00000000000000000000", `{"name":"kv2"}`},
		{"create-postgres", http.MethodPost, "/v1/postgres", `{"name":"pg","plan":"free","version":"16"}`},
		{"update-postgres", http.MethodPatch, "/v1/postgres/dpg-00000000000000000000", `{"name":"pg2"}`},
		{"create-project", http.MethodPost, "/v1/projects", `{"name":"proj","ownerId":"tea-a"}`},
		{"update-project", http.MethodPatch, "/v1/projects/prj-00000000000000000000", `{"name":"proj2"}`},
		{"create-environment", http.MethodPost, "/v1/environments", `{"name":"staging","projectId":"prj-00000000000000000000"}`},
		{"update-environment", http.MethodPatch, "/v1/environments/evm-00000000000000000000", `{"name":"prod"}`},
		{"create-env-group", http.MethodPost, "/v1/env-groups", `{"name":"shared"}`},
		{"update-env-group", http.MethodPatch, "/v1/env-groups/evg-00000000000000000000", `{"name":"shared2"}`},
		{"update-env-group-env-var", http.MethodPut, "/v1/env-groups/evg-00000000000000000000/env-vars/KEY", `{"value":"v"}`},
		{"update-env-group-secret-file", http.MethodPut, "/v1/env-groups/evg-00000000000000000000/secret-files/ca.pem", `{"content":"c"}`},
		{"add-route", http.MethodPost, "/v1/services/" + srv + "/routes", `{"type":"redirect","source":"/a","destination":"/b"}`},
		{"patch-route", http.MethodPatch, "/v1/services/" + srv + "/routes/rte-00000000000000000000", `{"priority":0}`},
		{"add-headers", http.MethodPost, "/v1/services/" + srv + "/headers", `{"path":"/*","name":"X-Test","value":"1"}`},
		{"create-webhook", http.MethodPost, "/v1/webhooks", `{"name":"hook","url":"https://example.com/hook"}`},
		{"update-webhook", http.MethodPatch, "/v1/webhooks/" + whk, `{"name":"hook2"}`},
	} {
		t.Run(tc.op, func(t *testing.T) {
			w := do(t, h, tc.method, tc.path, testToken, tc.body)
			body := w.Body.String()
			for _, refusal := range []string{"invalid request body at", "does not match the Render API schema", "request body is not allowed", "unknown field"} {
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
	created := read(do(t, h, http.MethodPost, "/v1/key-value", testToken, `{"name":"kvh","plan":"free","maxmemoryPolicy":"allkeys-lru"}`), http.StatusCreated)
	if created.Options.MaxmemoryPolicy != "allkeys_lru" {
		t.Fatalf("created maxmemoryPolicy = %q, want Render's allkeys_lru", created.Options.MaxmemoryPolicy)
	}
	read(do(t, h, http.MethodPost, "/v1/key-value", testToken, `{"name":"kvu","plan":"free","maxmemoryPolicy":"allkeys_lru"}`), http.StatusCreated)
	patched := read(do(t, h, http.MethodPatch, "/v1/key-value/"+created.ID, testToken, `{"persistenceMode":"journal-snapshot","maxmemoryPolicy":"volatile-ttl"}`), http.StatusOK)
	if patched.Options.PersistenceMode != "journal_snapshot" || patched.Options.MaxmemoryPolicy != "volatile_ttl" {
		t.Fatalf("patched = %+v, want journal_snapshot and volatile_ttl", patched)
	}
	for _, body := range []string{`{"name":"kvb","plan":"free","maxmemoryPolicy":"bogus-w8-139"}`, `{"name":"kvb","plan":"free","persistenceMode":"journal-bogus"}`} {
		if w := do(t, h, http.MethodPost, "/v1/key-value", testToken, body); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid request body at") {
			t.Errorf("POST %s = %d %s, want the validator's 400", body, w.Code, w.Body.String())
		}
	}
}
