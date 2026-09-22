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
	"strings"
	"testing"
)

// TestMCPToolsNeverCallBexRender is w4/113. The consumer of an MCP tool schema
// is an agent deciding what product it is driving, and one unannotated constant
// told 180 of 188 tools that the caller's workspace was a "Render workspace".
//
// The line this pins: bex may REFER to Render when explaining compatibility —
// "bex extension over Render's MCP" tells an agent something true and useful —
// and must never CALL ITSELF Render. A possessive ("Render's MCP", "Render's
// schema") is a reference; a bare noun phrase naming the caller's own resource
// is a mislabel.
func TestMCPToolsNeverCallBexRender(t *testing.T) {
	// Each phrase names something belonging to the CALLER. None of them can be
	// read as bex describing Render's product.
	mislabels := []string{
		"render workspace",
		"render account",
		"your render",
		"the render service",
	}

	for _, tool := range listBexTools(t, fullyWiredServer()).Tools {
		text := tool.Description
		if tool.InputSchema != nil {
			raw, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatalf("marshal input schema for %s: %v", tool.Name, err)
			}
			text += " " + string(raw)
		}
		lower := strings.ToLower(text)
		for _, phrase := range mislabels {
			if strings.Contains(lower, phrase) {
				t.Errorf("tool %s names the caller's own resource as Render (%q) — bex may refer to Render, never call itself Render", tool.Name, phrase)
			}
		}
	}
}

// TestMCPMayStillReferToRender keeps the guard above from being read as "purge
// the word Render". The compatibility descriptions are deliberate and stay.
func TestMCPMayStillReferToRender(t *testing.T) {
	var referring int
	for _, tool := range listBexTools(t, fullyWiredServer()).Tools {
		if strings.Contains(tool.Description, "Render's") {
			referring++
		}
	}
	if referring == 0 {
		t.Fatal("no tool explains its relationship to Render any more — the compatibility context is deliberate (w4/113)")
	}
}

// TestMCPImageCreateIsReachable is w4/114: create_web_service and
// create_cron_job required buildCommand and startCommand — inherited verbatim
// from Render's tool, which is coherent there because Render's tool is git-only
// — while bex's handler refuses both for a prebuilt image. No value satisfied
// both, so every image-backed service type (web, private, worker, cron) was
// unreachable over MCP while REST and GraphQL created them happily.
//
// The requirement was never the schema's to enforce: resolveBuildStrategy
// refuses a native runtime without both commands on every surface, which is
// what the apps-package half of this pins.
func TestMCPImageCreateIsReachable(t *testing.T) {
	required := map[string]map[string]bool{}
	for _, tool := range listBexTools(t, fullyWiredServer()).Tools {
		if tool.InputSchema == nil {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal input schema for %s: %v", tool.Name, err)
		}
		var sch struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(raw, &sch); err != nil {
			t.Fatalf("parse input schema for %s: %v", tool.Name, err)
		}
		set := map[string]bool{}
		for _, r := range sch.Required {
			set[r] = true
		}
		required[tool.Name] = set
	}

	for tool, stillRequired := range map[string][]string{
		"create_web_service": {"name"},
		"create_cron_job":    {"name", "schedule"},
	} {
		set, ok := required[tool]
		if !ok {
			t.Errorf("%s is not registered", tool)
			continue
		}
		for _, field := range []string{"buildCommand", "startCommand"} {
			if set[field] {
				t.Errorf("%s still requires %s, so a prebuilt image cannot be created: the schema refuses its absence and the handler refuses its presence", tool, field)
			}
		}
		// The relaxation is exactly two fields wide.
		for _, field := range stillRequired {
			if !set[field] {
				t.Errorf("%s no longer requires %s — the fix was meant to be two fields wide", tool, field)
			}
		}
	}
}
