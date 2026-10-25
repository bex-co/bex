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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// failingStore models the intermittent post-create content read: writes land,
// reads fail.
type readFailStore struct{ core.SecretKV }

func (readFailStore) Get(context.Context, string) (map[string]string, error) {
	return nil, core.ErrNotFound
}

func TestCreateResponseRevealsWrittenContentsWhenReadFails(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))
	view, err := svc.CreateEnvGroup(testCtx(), CreateEnvGroupRequest{
		Name:    "shared",
		EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "lit-value"}, {Key: "GEN", GenerateValue: true}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	svc.Store = readFailStore{svc.Store}
	out, err := svc.committedRevealed(testCtx())(view, nil)
	if err != nil {
		t.Fatalf("committed response: %v", err)
	}
	body, _ := json.Marshal(out)
	var group EnvGroupView
	if err := json.Unmarshal(body, &group); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, v := range group.EnvVars {
		values[v.Key] = v.Value
	}
	if values["TOKEN"] != "lit-value" {
		t.Fatalf("create response TOKEN = %q, want the written value (body %s)", values["TOKEN"], body)
	}
	if len(values["GEN"]) != 44 {
		t.Fatalf("create response GEN = %q, want a 44-char generated value", values["GEN"])
	}
}

func TestMaskedFallbackOmitsValue(t *testing.T) {
	resolver := coretest.Members{"dana": {"tea-a"}}
	svc := withGroups(&Service{
		Base:  &core.Base{Client: fakeClient(ownedApp("web", "tea-a")), Namespace: "default", Workspace: resolver},
		Store: newFakeStore(),
	})
	ctx := core.WithIdentity(testCtx(), core.Identity{Subject: "dana", Method: "session"})
	view, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name: "shared", EnvVars: []CreateEnvVarInput{{Key: "TOKEN", Value: "topsecret"}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	svc.Authz = sensitiveRevokedChecker{}
	out, err := svc.committedRevealed(ctx)(view, nil)
	if err != nil {
		t.Fatalf("committed response: %v", err)
	}
	body, _ := json.Marshal(out)
	if strings.Contains(string(body), "topsecret") || strings.Contains(string(body), `"value":""`) {
		t.Fatalf("masked fallback must omit values, got %s", body)
	}
}

func TestRESTPerKeyWritesReturnEnvGroup(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))
	w := serveREST(svc, "POST", "/v1/env-groups", `{"name":"shared"}`)
	if w.Code != 201 {
		t.Fatalf("create => %d: %s", w.Code, w.Body.String())
	}
	var g EnvGroupView
	_ = json.Unmarshal(w.Body.Bytes(), &g)

	w = serveREST(svc, "PUT", "/v1/env-groups/"+g.ID+"/env-vars/TOKEN", `{"value":"abc123"}`)
	if w.Code != 200 {
		t.Fatalf("PUT var => %d: %s", w.Code, w.Body.String())
	}
	var afterVar EnvGroupView
	if err := json.Unmarshal(w.Body.Bytes(), &afterVar); err != nil || afterVar.ID != g.ID {
		t.Fatalf("PUT var body is not the envGroup: %s", w.Body.String())
	}
	found := false
	for _, v := range afterVar.EnvVars {
		if v.Key == "TOKEN" && v.Value == "abc123" {
			found = true
		}
	}
	if !found {
		t.Fatalf("PUT var response missing the written value: %s", w.Body.String())
	}

	w = serveREST(svc, "PUT", "/v1/env-groups/"+g.ID+"/env-vars/GEN", `{"generateValue":true}`)
	if w.Code != 200 {
		t.Fatalf("PUT generate => %d: %s", w.Code, w.Body.String())
	}
	var afterGen EnvGroupView
	_ = json.Unmarshal(w.Body.Bytes(), &afterGen)
	found = false
	for _, v := range afterGen.EnvVars {
		if v.Key == "GEN" && len(v.Value) == 44 {
			found = true
		}
	}
	if !found {
		t.Fatalf("PUT generate response missing a 44-char value: %s", w.Body.String())
	}

	w = serveREST(svc, "PUT", "/v1/env-groups/"+g.ID+"/secret-files/cert.pem", `{"content":"CERTDATA"}`)
	if w.Code != 200 {
		t.Fatalf("PUT file => %d: %s", w.Code, w.Body.String())
	}
	var afterFile EnvGroupView
	if err := json.Unmarshal(w.Body.Bytes(), &afterFile); err != nil || afterFile.ID != g.ID {
		t.Fatalf("PUT file body is not the envGroup: %s", w.Body.String())
	}
	found = false
	for _, f := range afterFile.SecretFiles {
		if f.Name == "cert.pem" && f.Content == "CERTDATA" {
			found = true
		}
	}
	if !found {
		t.Fatalf("PUT file response missing the written content: %s", w.Body.String())
	}
}

// w4/396: the REST list is Render's envGroupMeta — no values anywhere —
// while retrieve keeps them.
func TestRESTListIsMetadataOnly(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))
	if w := serveREST(svc, "POST", "/v1/env-groups", `{"name":"shared","envVars":[{"key":"TOKEN","value":"secret-v1"}]}`); w.Code != 201 {
		t.Fatalf("create => %d: %s", w.Code, w.Body.String())
	}
	w := serveREST(svc, "GET", "/v1/env-groups", "")
	if w.Code != 200 {
		t.Fatalf("list => %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "secret-v1") || strings.Contains(w.Body.String(), `"envVars"`) {
		t.Fatalf("list leaked values: %s", w.Body.String())
	}
	var page []struct {
		EnvGroup struct {
			ID string `json:"id"`
		} `json:"envGroup"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page) != 1 {
		t.Fatalf("list page = %s (%v)", w.Body.String(), err)
	}
	w = serveREST(svc, "GET", "/v1/env-groups/"+page[0].EnvGroup.ID, "")
	if !strings.Contains(w.Body.String(), "secret-v1") {
		t.Fatalf("retrieve lost values: %s", w.Body.String())
	}
}
