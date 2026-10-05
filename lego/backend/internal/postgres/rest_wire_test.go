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

// Create accepts Render's parameterOverrides under PATCH's rule, and a
// malformed body names what is wrong instead of a bare "bad request body".
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
	rec := post(`{"name":"qa-pg","plan":"basic_256mb","parameterOverrides":{"work_mem":"64MB"},"dryRun":true}`)
	var got PostgresView
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusOK || got.ParameterOverrides["work_mem"] != "64MB" || got.Plan != "basic_256mb" {
		t.Fatalf("create with overrides => %d %s", rec.Code, rec.Body)
	}
	if rec := post(`{"name":"qa-pg","plan":"free","parameterOverrides":{"wal_level":"minimal"},"dryRun":true}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "wal_level") {
		t.Fatalf("operator-managed override => %d %s", rec.Code, rec.Body)
	}
	if rec := post(`{"name":"qa-pg","plan":"free","nope":1,"dryRun":true}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), `nope`) {
		t.Fatalf("unknown field => %d %s, want it named", rec.Code, rec.Body)
	}
}
