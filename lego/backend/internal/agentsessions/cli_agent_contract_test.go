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
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "agent-cli-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]any
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	for section, typ := range map[string]reflect.Type{
		"session":        reflect.TypeFor[View](),
		"attachment":     reflect.TypeFor[Attachment](),
		"transcriptPage": reflect.TypeFor[TranscriptPage](),
	} {
		value, ok := golden[section]
		if !ok {
			t.Fatalf("the fixture lost its %q section", section)
		}
		assertFieldsExist(t, section, value, typ)
	}

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
