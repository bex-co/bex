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

package apps

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestRenderBlueprintCapabilityRegistryExhaustive(t *testing.T) {
	registry, err := RenderBlueprintCapabilityRegistry()
	if err != nil {
		t.Fatalf("RenderBlueprintCapabilityRegistry(): %v", err)
	}
	if got, want := registry.Schema.SHA256, RenderBlueprintSchemaSHA256; got != want {
		t.Fatalf("registry schema digest = %q, want %q", got, want)
	}
	if got, want := len(registry.Fields), 163; got != want {
		t.Fatalf("registered field count = %d, want %d", got, want)
	}
	if got, want := len(registry.EnumValues), 19; got != want {
		t.Fatalf("registered enum count = %d, want %d", got, want)
	}
	valueCount := 0
	for _, values := range registry.EnumValues {
		valueCount += len(values)
	}
	if got, want := valueCount, 106; got != want {
		t.Fatalf("registered enum value count = %d, want %d", got, want)
	}
	if !json.Valid(bexBlueprintExtensionSchemaSource) {
		t.Fatal("x-bex extension schema is not JSON")
	}
}

func TestRenderBlueprintCapabilityRegistryFixturesExist(t *testing.T) {
	registry, err := RenderBlueprintCapabilityRegistry()
	if err != nil {
		t.Fatalf("RenderBlueprintCapabilityRegistry(): %v", err)
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	backendRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	fixtures := map[string]struct{}{}
	for _, capability := range registry.Fields {
		if capability.State != BlueprintCapabilityUnsupported {
			fixtures[capability.Fixture] = struct{}{}
		}
	}
	for _, values := range registry.EnumValues {
		for _, capability := range values {
			if capability.State != BlueprintCapabilityUnsupported {
				fixtures[capability.Fixture] = struct{}{}
			}
		}
	}
	ordered := make([]string, 0, len(fixtures))
	for fixture := range fixtures {
		ordered = append(ordered, fixture)
	}
	sort.Strings(ordered)
	for _, fixture := range ordered {
		path, testName, ok := strings.Cut(fixture, ":")
		if !ok || path == "" || testName == "" || filepath.IsAbs(path) {
			t.Errorf("invalid capability fixture %q", fixture)
			continue
		}
		data, err := os.ReadFile(filepath.Join(backendRoot, filepath.Clean(path)))
		if err != nil {
			t.Errorf("read capability fixture %q: %v", fixture, err)
			continue
		}
		if !strings.Contains(string(data), "func "+testName+"(") {
			t.Errorf("capability fixture %q has no test function %s", fixture, testName)
		}
	}
}

func TestRenderBlueprintCapabilityRegistryDetectsIntegrityMismatch(t *testing.T) {
	mutated := append([]byte(nil), renderBlueprintSchemaSource...)
	mutated[0] ^= 1
	_, err := loadRenderBlueprintCapabilityRegistryData(mutated, renderBlueprintCapabilitiesSource)
	if err == nil || !strings.Contains(err.Error(), "integrity mismatch") {
		t.Fatalf("integrity error = %v, want mismatch", err)
	}
}

func TestRenderBlueprintCapabilityRegistryRejectsMissingField(t *testing.T) {
	registry := decodeCapabilityRegistry(t)
	delete(registry.Fields, "#/definitions/database/properties/name")
	err := validateRenderBlueprintCapabilityRegistry(renderBlueprintSchemaSource, &registry)
	if err == nil || !strings.Contains(err.Error(), "missing field #/definitions/database/properties/name") {
		t.Fatalf("missing-field error = %v", err)
	}
}

func TestRenderBlueprintCapabilityRegistryRejectsUnclassifiedUpstreamEnum(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(renderBlueprintSchemaSource, &document); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	definitions := document["definitions"].(map[string]any)
	autoDeploy := definitions["autoDeployTrigger"].(map[string]any)
	autoDeploy["enum"] = append(autoDeploy["enum"].([]any), "afterChecksPass")
	mutated, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode schema: %v", err)
	}
	registry := decodeCapabilityRegistry(t)
	registry.Schema.SHA256 = fmtSHA256(mutated)
	err = validateRenderBlueprintCapabilityRegistry(mutated, &registry)
	if err == nil || !strings.Contains(err.Error(), "missing enum value #/definitions/autoDeployTrigger/enum \"afterChecksPass\"") {
		t.Fatalf("unclassified-enum error = %v", err)
	}
}

func decodeCapabilityRegistry(t *testing.T) BlueprintCapabilityRegistry {
	t.Helper()
	var registry BlueprintCapabilityRegistry
	if err := json.Unmarshal(renderBlueprintCapabilitiesSource, &registry); err != nil {
		t.Fatalf("decode capability registry: %v", err)
	}
	return registry
}

func fmtSHA256(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
