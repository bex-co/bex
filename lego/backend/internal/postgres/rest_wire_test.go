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

package postgres

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
)

// w8/057: every bex Postgres tier renders on REST as a value of Render's own
// REST plan enum (basic_256mb, not the Blueprint spelling basic-256mb).
func TestRESTPostgresPlanIsInRendersEnum(t *testing.T) {
	raw, err := os.ReadFile("../api/openapi/render-public-api-1.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	enum := spec.Components.Schemas["postgresPOSTInput"].Properties["plan"].Enum
	if len(enum) < 10 {
		t.Fatalf("pinned plan enum looks wrong: %v", enum)
	}
	for _, id := range tiers.Postgres.IDs() {
		if got := renderRESTPostgresPlan(id); !slices.Contains(enum, got) {
			t.Errorf("tier %q renders as %q, which is not in Render's REST enum", id, got)
		}
	}
	if got := renderRESTPostgresPlan("0.1c-256mb"); got != "0.1c-256mb" {
		t.Errorf("spec-based id rewritten to %q", got)
	}
}

// Create refuses the retired parameterOverrides by name even under the strict
// decoder production runs (w2/066), and a malformed body names what is wrong
// instead of a bare "bad request body".
func TestRESTCreatePostgresParameterOverridesAndDecodeErrors(t *testing.T) {
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/v1/postgres", strings.NewReader(body))
		mux.ServeHTTP(rec, req.WithContext(core.WithStrictJSONDecoding(req.Context())))
		return rec
	}
	for _, body := range []string{
		`{"name":"qa-pg","plan":"basic_256mb","parameterOverrides":{"work_mem":"64MB"},"dryRun":true}`,
		`{"name":"qa-pg","plan":"basic_256mb","parameterOverrides":{},"dryRun":true}`,
	} {
		if rec := post(body); rec.Code != http.StatusBadRequest ||
			!strings.Contains(rec.Body.String(), "parameterOverrides is no longer supported") ||
			!strings.Contains(rec.Body.String(), "ALTER DATABASE") {
			t.Fatalf("create with parameterOverrides %s => %d %s, want the named refusal", body, rec.Code, rec.Body)
		}
	}
	if rec := post(`{"name":"qa-pg","plan":"free","nope":1,"dryRun":true}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), `nope`) {
		t.Fatalf("unknown field => %d %s, want it named", rec.Code, rec.Body)
	}
}

// w4/697: Render's pinned spec declares suspend, resume and restart as a 202
// with no content, so bex answers each with an empty body (as failover does),
// while the verb still takes effect and an error keeps its JSON shape.
func TestRESTPostgresLifecycleVerbsAnswerBodiless202(t *testing.T) {
	raw, err := os.ReadFile("../api/openapi/render-public-api-1.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	verbs := []string{"suspend", "resume", "restart"}
	for _, verb := range verbs {
		var op struct {
			Responses map[string]map[string]json.RawMessage `json:"responses"`
		}
		if err := json.Unmarshal(spec.Paths["/postgres/{postgresId}/"+verb]["post"], &op); err != nil {
			t.Fatalf("spec %s: %v", verb, err)
		}
		accepted, ok := op.Responses["202"]
		if !ok {
			t.Fatalf("spec has no 202 for %s", verb)
		}
		if _, hasContent := accepted["content"]; hasContent {
			t.Fatalf("spec now declares a %s body; revisit w4/697", verb)
		}
	}

	svc, _ := newService()
	created := serveREST(svc, http.MethodPost, "/v1/postgres", `{"name":"pg-life","plan":"free"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", created.Code, created.Body.String())
	}
	var pg struct{ ID string }
	if err := json.Unmarshal(created.Body.Bytes(), &pg); err != nil || pg.ID == "" {
		t.Fatalf("create body has no id: %s", created.Body.String())
	}
	for _, verb := range verbs {
		rec := serveREST(svc, http.MethodPost, "/v1/postgres/"+pg.ID+"/"+verb, "")
		if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
			t.Fatalf("%s => %d %q, want 202 with an empty body", verb, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "" {
			t.Fatalf("%s Content-Type = %q, want none for an empty body", verb, ct)
		}
		if verb == "suspend" {
			got := serveREST(svc, http.MethodGet, "/v1/postgres/"+pg.ID, "")
			if !strings.Contains(got.Body.String(), `"suspended":"suspended"`) {
				t.Fatalf("suspend did not take effect: %s", got.Body.String())
			}
		}
	}

	missing := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-missing/suspend", "")
	if missing.Code != http.StatusNotFound || !json.Valid(missing.Body.Bytes()) {
		t.Fatalf("missing suspend => %d %q, want a JSON 404", missing.Code, missing.Body.String())
	}
}
