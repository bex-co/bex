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
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
)

// RenderBlueprintSchemaSHA256 is the digest of the reviewed Render schema
// snapshot fetched on 2026-08-02. Production never downloads this unversioned
// upstream document: a deliberate update must change both the bytes and this
// constant, then pass capability-registry exhaustiveness tests.
const RenderBlueprintSchemaSHA256 = "665539cb0c191856ba38d292b985a963880bb69b030d666e5fe7788e78e7e696"

//go:embed schema/render.yaml.json
var renderBlueprintSchemaSource []byte

//go:embed schema/capabilities.json
var renderBlueprintCapabilitiesSource []byte

//go:embed schema/x-bex.schema.json
var bexBlueprintExtensionSchemaSource []byte

// BlueprintCapabilityState states how bex treats one reviewed Render
// Blueprint field or enum value. No state means "unclassified", which the
// registry validator rejects.
type BlueprintCapabilityState string

const (
	BlueprintCapabilityEquivalent  BlueprintCapabilityState = "equivalent"
	BlueprintCapabilityTranslated  BlueprintCapabilityState = "translated"
	BlueprintCapabilityUnsupported BlueprintCapabilityState = "unsupported"
	BlueprintCapabilityDeprecated  BlueprintCapabilityState = "deprecated"
	BlueprintCapabilityExtension   BlueprintCapabilityState = "extension"
)

// BlueprintCapabilityHandler is a checked identifier for the canonical stage
// that implements an accepted Blueprint construct. Registry entries are not
// free-form audit prose: startup rejects a handler that is not part of this
// compiler/apply pipeline.
type BlueprintCapabilityHandler string

const (
	blueprintHandlerCompileSource BlueprintCapabilityHandler = "CompileBlueprintSource"
	blueprintHandlerNormalizeIR   BlueprintCapabilityHandler = "NormalizeBlueprintIR"
	blueprintHandlerManifestType  BlueprintCapabilityHandler = "manifestType"
	blueprintHandlerParseStack    BlueprintCapabilityHandler = "parseCompiledStack"
	blueprintHandlerParseKeyValue BlueprintCapabilityHandler = "parseKeyValue"
	blueprintHandlerParseService  BlueprintCapabilityHandler = "parseService"
)

var blueprintCapabilityHandlers = map[BlueprintCapabilityHandler]struct{}{
	blueprintHandlerCompileSource: {},
	blueprintHandlerNormalizeIR:   {},
	blueprintHandlerManifestType:  {},
	blueprintHandlerParseStack:    {},
	blueprintHandlerParseKeyValue: {},
	blueprintHandlerParseService:  {},
}

// BlueprintCapability is deliberately small. The registry is the canonical
// field-by-field ledger; implementation tasks replace conservative
// unsupported entries with the handler and fixture that prove equivalence.
type BlueprintCapability struct {
	State   BlueprintCapabilityState   `json:"state"`
	Handler BlueprintCapabilityHandler `json:"handler,omitempty"`
	Fixture string                     `json:"fixture,omitempty"`
	Reason  string                     `json:"reason"`
}

type blueprintCapabilitySchema struct {
	Source      string `json:"source"`
	RetrievedAt string `json:"retrievedAt"`
	SHA256      string `json:"sha256"`
}

type blueprintExtension struct {
	State     BlueprintCapabilityState `json:"state"`
	Schema    string                   `json:"schema"`
	AllowedAt []string                 `json:"allowedAt"`
	Reason    string                   `json:"reason"`
}

// BlueprintCapabilityRegistry is the machine-readable reviewed contract. The
// map keys are JSON Pointers into the pinned schema. EnumValues is indexed by
// the pointer to an enum array and canonical JSON encoding of each value, so
// string, number, and boolean enum members cannot collide.
type BlueprintCapabilityRegistry struct {
	Version    int                                       `json:"version"`
	Schema     blueprintCapabilitySchema                 `json:"schema"`
	Fields     map[string]BlueprintCapability            `json:"fields"`
	EnumValues map[string]map[string]BlueprintCapability `json:"enumValues"`
	Extensions map[string]blueprintExtension             `json:"extensions"`
}

var renderBlueprintRegistryOnce = sync.OnceValues(loadRenderBlueprintCapabilityRegistry)

// RenderBlueprintSchema returns a copy of the reviewed Render schema bytes.
// Copying prevents callers from invalidating the integrity check for a later
// request in the same process.
func RenderBlueprintSchema() []byte {
	return append([]byte(nil), renderBlueprintSchemaSource...)
}

// RenderBlueprintCapabilityRegistry returns the checked, immutable-by-
// convention capability registry. Callers must treat its maps as read-only.
func RenderBlueprintCapabilityRegistry() (*BlueprintCapabilityRegistry, error) {
	return renderBlueprintRegistryOnce()
}

func loadRenderBlueprintCapabilityRegistry() (*BlueprintCapabilityRegistry, error) {
	return loadRenderBlueprintCapabilityRegistryData(renderBlueprintSchemaSource, renderBlueprintCapabilitiesSource)
}

func loadRenderBlueprintCapabilityRegistryData(schemaData, registryData []byte) (*BlueprintCapabilityRegistry, error) {
	digest := fmt.Sprintf("%x", sha256.Sum256(schemaData))
	if digest != RenderBlueprintSchemaSHA256 {
		return nil, fmt.Errorf("Render Blueprint schema integrity mismatch: got %s, want %s", digest, RenderBlueprintSchemaSHA256)
	}

	var registry BlueprintCapabilityRegistry
	if err := json.Unmarshal(registryData, &registry); err != nil {
		return nil, fmt.Errorf("decode Render Blueprint capability registry: %w", err)
	}
	if err := validateRenderBlueprintCapabilityRegistry(schemaData, &registry); err != nil {
		return nil, err
	}
	return &registry, nil
}

func validateRenderBlueprintCapabilityRegistry(schemaData []byte, registry *BlueprintCapabilityRegistry) error {
	if registry == nil {
		return fmt.Errorf("Render Blueprint capability registry is nil")
	}
	if registry.Version != 1 {
		return fmt.Errorf("Render Blueprint capability registry version = %d, want 1", registry.Version)
	}
	if registry.Schema.Source != "https://render.com/schema/render.yaml.json" {
		return fmt.Errorf("Render Blueprint capability registry source = %q", registry.Schema.Source)
	}
	if registry.Schema.RetrievedAt == "" {
		return fmt.Errorf("Render Blueprint capability registry has no retrieval date")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(schemaData))
	if registry.Schema.SHA256 != digest {
		return fmt.Errorf("Render Blueprint capability registry digest = %s, want %s", registry.Schema.SHA256, digest)
	}

	fields, enumValues, err := renderSchemaVocabulary(schemaData)
	if err != nil {
		return err
	}
	if err := validateCapabilityMap("field", fields, registry.Fields); err != nil {
		return err
	}
	if err := validateEnumCapabilityMap(enumValues, registry.EnumValues); err != nil {
		return err
	}

	extension, ok := registry.Extensions["x-bex"]
	if !ok || extension.State != BlueprintCapabilityExtension || extension.Schema == "" || len(extension.AllowedAt) == 0 || extension.Reason == "" {
		return fmt.Errorf("Render Blueprint capability registry has no complete x-bex extension entry")
	}
	return nil
}

func validateCapabilityMap(kind string, want map[string]struct{}, got map[string]BlueprintCapability) error {
	for pointer := range want {
		capability, ok := got[pointer]
		if !ok {
			return fmt.Errorf("Render Blueprint capability registry is missing %s %s", kind, pointer)
		}
		if err := validateBlueprintCapability(pointer, capability); err != nil {
			return err
		}
	}
	for pointer := range got {
		if _, ok := want[pointer]; !ok {
			return fmt.Errorf("Render Blueprint capability registry has unknown %s %s", kind, pointer)
		}
	}
	return nil
}

func validateEnumCapabilityMap(want map[string]map[string]struct{}, got map[string]map[string]BlueprintCapability) error {
	for pointer, values := range want {
		registered, ok := got[pointer]
		if !ok {
			return fmt.Errorf("Render Blueprint capability registry is missing enum %s", pointer)
		}
		for value := range values {
			capability, ok := registered[value]
			if !ok {
				return fmt.Errorf("Render Blueprint capability registry is missing enum value %s %s", pointer, value)
			}
			if err := validateBlueprintCapability(pointer+"/"+value, capability); err != nil {
				return err
			}
		}
		for value := range registered {
			if _, ok := values[value]; !ok {
				return fmt.Errorf("Render Blueprint capability registry has unknown enum value %s %s", pointer, value)
			}
		}
	}
	for pointer := range got {
		if _, ok := want[pointer]; !ok {
			return fmt.Errorf("Render Blueprint capability registry has unknown enum %s", pointer)
		}
	}
	return nil
}

func validateBlueprintCapability(pointer string, capability BlueprintCapability) error {
	switch capability.State {
	case BlueprintCapabilityEquivalent, BlueprintCapabilityTranslated, BlueprintCapabilityUnsupported, BlueprintCapabilityDeprecated, BlueprintCapabilityExtension:
	default:
		return fmt.Errorf("Render Blueprint capability registry has unclassified entry %s", pointer)
	}
	if capability.Reason == "" {
		return fmt.Errorf("Render Blueprint capability registry has no reason for %s", pointer)
	}
	if capability.State != BlueprintCapabilityUnsupported && (capability.Handler == "" || capability.Fixture == "") {
		return fmt.Errorf("Render Blueprint capability registry has no handler/fixture evidence for %s", pointer)
	}
	if capability.Handler != "" {
		if _, ok := blueprintCapabilityHandlers[capability.Handler]; !ok {
			return fmt.Errorf("Render Blueprint capability registry has unknown handler %q for %s", capability.Handler, pointer)
		}
	}
	return nil
}

func renderSchemaVocabulary(schemaData []byte) (map[string]struct{}, map[string]map[string]struct{}, error) {
	var document any
	if err := json.Unmarshal(schemaData, &document); err != nil {
		return nil, nil, fmt.Errorf("decode Render Blueprint schema: %w", err)
	}
	fields := map[string]struct{}{}
	enumValues := map[string]map[string]struct{}{}
	collectRenderSchemaVocabulary(document, nil, fields, enumValues)
	return fields, enumValues, nil
}

func collectRenderSchemaVocabulary(node any, path []string, fields map[string]struct{}, enumValues map[string]map[string]struct{}) {
	switch value := node.(type) {
	case []any:
		for i, child := range value {
			collectRenderSchemaVocabulary(child, append(path, fmt.Sprintf("%d", i)), fields, enumValues)
		}
	case map[string]any:
		if properties, ok := value["properties"].(map[string]any); ok {
			for _, name := range slices.Sorted(maps.Keys(properties)) {
				fields[renderSchemaPointer(append(path, "properties", name))] = struct{}{}
			}
		}
		if values, ok := value["enum"].([]any); ok {
			pointer := renderSchemaPointer(append(path, "enum"))
			registered := enumValues[pointer]
			if registered == nil {
				registered = map[string]struct{}{}
				enumValues[pointer] = registered
			}
			for _, member := range values {
				encoded, err := json.Marshal(member)
				if err != nil {
					panic(fmt.Sprintf("marshal Render Blueprint enum: %v", err))
				}
				registered[string(encoded)] = struct{}{}
			}
		}
		for _, key := range slices.Sorted(maps.Keys(value)) {
			collectRenderSchemaVocabulary(value[key], append(path, key), fields, enumValues)
		}
	}
}

// blueprintPointerEscaper is the RFC 6901 token escaper. It is package-level
// because renderSchemaPointer runs twice per YAML node walked, and the walk
// admits up to blueprintMaxNodes of them per Blueprint parse.
var blueprintPointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

func renderSchemaPointer(parts []string) string {
	if len(parts) == 0 {
		return "#"
	}
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = blueprintPointerEscaper.Replace(part)
	}
	return "#/" + strings.Join(escaped, "/")
}
