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

package agentsessions

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestShapesMatchCLIAgentContract is the backend twin of the `bex agent`
// fixture (lego/cli/testdata/agent-cli-contract.json, w9/m177): the CLI
// imports no sibling module and decodes these field names by hand, so every
// key the fixture uses must still be a JSON field of the type that serves it.
// Field names come from the struct tags, so omitempty fields count.
func TestShapesMatchCLIAgentContract(t *testing.T) {
	golden := loadCLIFixture(t, "agent-cli-contract.json")
	assertSections(t, golden, map[string]reflect.Type{
		"session":           reflect.TypeFor[View](),
		"attachment":        reflect.TypeFor[Attachment](),
		"transcriptPage":    reflect.TypeFor[TranscriptPage](),
		"tree":              reflect.TypeFor[Tree](),
		"modelCost":         reflect.TypeFor[ModelCost](),
		"secretList":        reflect.TypeFor[AgentSecretList](),
		"skill":             reflect.TypeFor[AgentSkill](),
		"skillVersionList":  reflect.TypeFor[AgentSkillVersionList](),
		"environment":       reflect.TypeFor[Environment](),
		"capabilities":      reflect.TypeFor[Capabilities](),
		"queuedMessage":     reflect.TypeFor[QueuedMessage](),
		"queuedMessageList": reflect.TypeFor[QueuedMessageList](),
	})

	// The error envelope is written by core.WriteErr, not a struct.
	rec := httptest.NewRecorder()
	core.WriteErr(rec, mapStoreError("ags-x", store.ErrNotFound))
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	for key := range golden["error"].(map[string]any) {
		if _, ok := envelope[key]; !ok {
			t.Errorf("the REST error envelope no longer carries %q, which bex agent reads", key)
		}
	}
}

// TestShapesMatchCLIAutomationContract is the backend twin of the
// `bex agent automations` fixture (lego/cli/testdata/
// agent-automation-cli-contract.json, ADR106 D9): every key the CLI reads must
// still be a JSON field of the type that serves it, and the run outcomes and
// error codes it branches on must still be the ones the service emits.
func TestShapesMatchCLIAutomationContract(t *testing.T) {
	golden := loadCLIFixture(t, "agent-automation-cli-contract.json")
	assertSections(t, golden, map[string]reflect.Type{
		"automation":     reflect.TypeFor[AgentAutomation](),
		"automationList": reflect.TypeFor[AgentAutomationList](),
		"run":            reflect.TypeFor[AgentAutomationRun](),
		"runList":        reflect.TypeFor[AgentAutomationRunList](),
	})
	outcomes := []any{store.AutomationRunClaimed, store.AutomationRunStarted, store.AutomationRunSkippedOverlap,
		store.AutomationRunFailedCreate, store.AutomationRunDenied}
	if !reflect.DeepEqual(golden["outcomes"], outcomes) {
		t.Errorf("run outcomes %v, the CLI expects %v", outcomes, golden["outcomes"])
	}
	codes := golden["errorCodes"].(map[string]any)
	for key, code := range map[string]string{
		"conflict": CodeAutomationConflict, "notCreator": CodeAutomationNotCreator,
	} {
		if codes[key] != code {
			t.Errorf("error code %s is %q, the CLI branches on %q", key, code, codes[key])
		}
	}
}

// loadCLIFixture reads a lego/cli/testdata fixture the CLI decodes by hand.
func loadCLIFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]any
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	return golden
}

// assertSections checks each named fixture section against the type that
// serves it.
func assertSections(t *testing.T, golden map[string]any, sections map[string]reflect.Type) {
	t.Helper()
	for section, typ := range sections {
		value, ok := golden[section]
		if !ok {
			t.Fatalf("the fixture lost its %q section", section)
		}
		assertFieldsExist(t, section, value, typ)
	}
}

// assertFieldsExist walks a fixture value against a Go type: every object key
// must be a json-tagged field, recursing into structs, slices and pointers.
// Free-form values (map[string]any, json.RawMessage, any) end the walk.
func assertFieldsExist(t *testing.T, path string, value any, typ reflect.Type) {
	t.Helper()
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		typ = typ.Elem()
		if items, ok := value.([]any); ok {
			for i, item := range items {
				assertFieldsExist(t, path+"["+strconv.Itoa(i)+"]", item, typ)
			}
			return
		}
	}
	object, ok := value.(map[string]any)
	if !ok || typ.Kind() != reflect.Struct || typ == reflect.TypeFor[json.RawMessage]() {
		return
	}
	fields := jsonFields(typ)
	for key, child := range object {
		field, ok := fields[key]
		if !ok {
			t.Errorf("%s.%s: %s has no such JSON field, but bex agent reads it", path, key, typ.Name())
			continue
		}
		assertFieldsExist(t, path+"."+key, child, field)
	}
}

func jsonFields(typ reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for i := range typ.NumField() {
		f := typ.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.Anonymous && name == "" {
			embedded := f.Type
			if embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			for k, v := range jsonFields(embedded) {
				out[k] = v
			}
			continue
		}
		if name == "-" || !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out[name] = f.Type
	}
	return out
}
