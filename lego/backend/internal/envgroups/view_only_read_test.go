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

package envgroups

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// w8/m442: Render's 2026-11-16 envGroup omits envVars and secretFiles "if the
// caller doesn't have the permission to read the environment group's secret
// values". A member who may view the group but not its secrets reads its
// metadata; it used to be refused the whole read.

// viewOnlyGroup is a group with one value and one secret file, read back by
// the caller authz allows.
func viewOnlyGroup(t *testing.T, authz core.Checker) (svc *Service, id string, sink *viewAuditSink) {
	t.Helper()
	svc = newService(newFakeStore(), sampleApp("web"))
	created := serveREST(svc, http.MethodPost, "/v1/env-groups",
		`{"name":"shared","envVars":[{"key":"API_KEY","value":"s3cret-value"}],"secretFiles":[{"name":"cert.pem","content":"PEM-BODY"}]}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", created.Code, created.Body)
	}
	var group struct{ ID string }
	if err := json.Unmarshal(created.Body.Bytes(), &group); err != nil || group.ID == "" {
		t.Fatalf("created group: %s (%v)", created.Body, err)
	}
	sink = &viewAuditSink{}
	svc.Audit = sink
	svc.Authz = authz
	return svc, group.ID, sink
}

func TestAViewOnlyReadOmitsTheValuesAndRecordsNothing(t *testing.T) {
	svc, id, sink := viewOnlyGroup(t, refuseRelation{relation: core.RelCanViewSensitive})

	rec := serveREST(svc, http.MethodGet, "/v1/env-groups/"+id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("a viewer's GET = %d %s, want 200", rec.Code, rec.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"envVars", "secretFiles"} {
		if _, present := body[key]; present {
			t.Errorf("the viewer's group carries %q: %s", key, rec.Body)
		}
	}
	if body["id"] != id || body["name"] != "shared" || body["ownerId"] == "" {
		t.Errorf("the viewer's group lacks its metadata: %s", rec.Body)
	}
	if _, ok := body["serviceLinks"].([]any); !ok {
		t.Errorf("serviceLinks = %v, want Render's required array", body["serviceLinks"])
	}
	for _, secret := range []string{"s3cret-value", "PEM-BODY", "API_KEY", "cert.pem"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("the viewer's group reveals %q: %s", secret, rec.Body)
		}
	}
	// Nothing was revealed, and nothing was refused: no value view, no denial.
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbViewEnvVarValues || ev.Verb == AuditVerbRevealEnvGroup || ev.Outcome == core.AuditDenied {
			t.Errorf("the viewer's read recorded %+v", ev)
		}
	}
}

func TestASecretReaderStillReadsTheValuesAndTheReadIsRecorded(t *testing.T) {
	svc, id, sink := viewOnlyGroup(t, nil)

	rec := serveREST(svc, http.MethodGet, "/v1/env-groups/"+id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	var group struct {
		EnvVars     []struct{ Key, Value string }
		SecretFiles []struct{ Name, Content string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}
	if len(group.EnvVars) != 1 || group.EnvVars[0].Value != "s3cret-value" ||
		len(group.SecretFiles) != 1 || group.SecretFiles[0].Content != "PEM-BODY" {
		t.Fatalf("the reader's group = %s, want its value and file content", rec.Body)
	}
	views := 0
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbViewEnvVarValues && ev.Outcome == core.AuditAllowed {
			views++
		}
	}
	if views != 1 {
		t.Fatalf("recorded %+v, want one ViewEnvVarValuesEvent", sink.events)
	}
}

func TestACallerWhoMayNotViewTheGroupIsStillRefused(t *testing.T) {
	svc, id, _ := viewOnlyGroup(t, refuseRelation{relation: core.RelCanView})

	rec := serveREST(svc, http.MethodGet, "/v1/env-groups/"+id, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("GET without can_view = %d %s, want 403", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "shared") {
		t.Errorf("the refusal names the group: %s", rec.Body)
	}
}

// w9/m496: a scoped API key reads a group's values, on its read and on a
// write's response alike, only when granted Render's "View environment group
// variable and secret file values", whatever its creator may see.
func TestAScopedKeyReadsGroupValuesOnlyWhenGranted(t *testing.T) {
	svc, id, _ := viewOnlyGroup(t, nil)
	grants := coretest.KeyGrants{"key-1": core.KeyGrantFromEntries([]string{"env_groups:read_write"})}
	svc.KeyGrants = grants
	key := core.WithIdentity(context.Background(), core.Identity{Subject: "key-1", ClientID: "key-1", Method: core.MethodOAuth2})
	reveals := func(rec *httptest.ResponseRecorder) bool {
		t.Helper()
		if rec.Code >= 300 {
			t.Fatalf("answer = %d %s", rec.Code, rec.Body)
		}
		return strings.Contains(rec.Body.String(), "s3cret-value") || strings.Contains(rec.Body.String(), "PEM-BODY")
	}
	if reveals(serveRESTContext(key, svc, http.MethodGet, "/v1/env-groups/"+id, "")) {
		t.Error("a key without the values read them on GET")
	}
	if reveals(serveRESTContext(key, svc, http.MethodPatch, "/v1/env-groups/"+id, `{"name":"renamed"}`)) {
		t.Error("a key without the values read them in a rename's response")
	}
	grants["key-1"] = core.KeyGrantFromEntries([]string{"env_groups:read_write", "env_group_values:read"})
	if !reveals(serveRESTContext(key, svc, http.MethodGet, "/v1/env-groups/"+id, "")) {
		t.Error("a key granted the values did not read them")
	}
}
