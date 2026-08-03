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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"gopkg.in/yaml.v3"
)

// BlueprintSourceLocation identifies an input value rather than a transformed
// Go struct field. It survives schema validation so REST, GraphQL, MCP, and
// the dashboard can show the same actionable source location.
type BlueprintSourceLocation struct {
	Line   int
	Column int
}

// BlueprintSourceProblem is the source/schema compiler's stable diagnostic.
// The path is an RFC 6901 JSON Pointer; it is never populated with a secret
// value. Adapter-specific error envelopes translate this one type later.
type BlueprintSourceProblem struct {
	Code    string
	Path    string
	Message string
	Line    int
	Column  int
}

// BlueprintSource is the source-preserving JSON-compatible representation of
// a YAML document. Locations map every explicit JSON Pointer to its YAML node.
// Value is safe to hand to the pinned JSON Schema validator.
type BlueprintSource struct {
	Value     any
	Locations map[string]BlueprintSourceLocation
}

var renderBlueprintSchemaOnce = sync.OnceValues(compileRenderBlueprintSchema)

// CompileBlueprintSource parses exactly one YAML document, rejects YAML forms
// that have no unambiguous JSON meaning, validates it against the reviewed
// Render schema plus the local x-bex overlay, and returns every independently
// actionable parse/schema error it can discover. It performs no auth, state
// lookup, or mutation.
func CompileBlueprintSource(manifest string) (*BlueprintSource, []BlueprintSourceProblem) {
	source, problems := parseBlueprintSource(manifest)
	if len(problems) > 0 {
		return source, sortBlueprintSourceProblems(problems)
	}
	registry, err := RenderBlueprintCapabilityRegistry()
	if err != nil {
		return source, []BlueprintSourceProblem{{
			Code:    "BLUEPRINT_SCHEMA_UNAVAILABLE",
			Path:    "#",
			Message: "the reviewed Render Blueprint capability registry could not be loaded",
		}}
	}
	schema, err := renderBlueprintSchemaOnce()
	if err != nil {
		return source, []BlueprintSourceProblem{{
			Code:    "BLUEPRINT_SCHEMA_UNAVAILABLE",
			Path:    "#",
			Message: "the reviewed Render Blueprint schema could not be loaded",
		}}
	}
	if err := schema.Validate(source.Value); err != nil {
		problems = append(problems, blueprintSchemaProblems(err, source.Locations)...)
	}
	problems = append(problems, blueprintCapabilityProblems(source.Value, nil, source.Locations, registry)...)
	return source, sortBlueprintSourceProblems(problems)
}

// blueprintSourceProblemsError adapts compiler diagnostics to stack callers
// that still use an error return. Validation adapters consume the complete
// problem list directly; mutation callers receive the first deterministic
// diagnostic and never reach a write.
func blueprintSourceProblemsError(problems []BlueprintSourceProblem) error {
	if len(problems) == 0 {
		return nil
	}
	problem := problems[0]
	return fmt.Errorf("%w: %s", core.ErrBadRequest, problem.Message)
}

// blueprintCapabilityProblems rejects source constructs that the upstream
// schema intentionally permits but bex cannot represent truthfully. Keeping
// this immediately after schema validation means every caller fails before a
// state lookup or write, with the source path that needs changing.
func blueprintCapabilityProblems(value any, path []string, locations map[string]BlueprintSourceLocation, registry *BlueprintCapabilityRegistry) []BlueprintSourceProblem {
	object, ok := value.(map[string]any)
	if !ok {
		switch child := value.(type) {
		case []any:
			var problems []BlueprintSourceProblem
			for index, item := range child {
				problems = append(problems, blueprintCapabilityProblems(item, append(path, strconv.Itoa(index)), locations, registry)...)
			}
			return problems
		}
		return nil
	}
	var problems []BlueprintSourceProblem
	problem := func(field, code, message string) {
		fieldPath := append(append([]string(nil), path...), field)
		pointer := renderSchemaPointer(fieldPath)
		location := lookupBlueprintLocation(pointer, locations)
		problems = append(problems, BlueprintSourceProblem{Code: code, Path: pointer, Message: message, Line: location.Line, Column: location.Column})
	}
	for field, child := range object {
		switch field {
		case "builder":
			problem(field, "BLUEPRINT_EXTENSION_REQUIRED", "bex build strategy must be written as x-bex.builder; builder is not a Render Blueprint field")
		case "region":
			if !blueprintCapabilityUnsupported(registry, "#/definitions/serverService/properties/region", "#/definitions/cronService/properties/region", "#/definitions/staticService/properties/region", "#/definitions/database/properties/region", "#/definitions/redisServer/properties/region") {
				break
			}
			problem(field, "BLUEPRINT_CAPABILITY_UNSUPPORTED", "per-resource region placement is not available on bex")
		case "disk":
			if !blueprintCapabilityUnsupported(registry, "#/definitions/serverService/properties/disk") {
				break
			}
			problem(field, "BLUEPRINT_CAPABILITY_UNSUPPORTED", "persistent service disks are not available on bex")
		case "dockerContext":
			if !blueprintCapabilityUnsupported(registry, "#/definitions/serverService/properties/dockerContext", "#/definitions/cronService/properties/dockerContext") {
				break
			}
			problem(field, "BLUEPRINT_CAPABILITY_UNSUPPORTED", "dockerContext is not available on bex because its build-context semantics cannot be represented exactly")
		case "registryCredential", "creds":
			if !blueprintCapabilityUnsupported(registry, "#/definitions/serverService/properties/registryCredential", "#/definitions/cronService/properties/registryCredential", "#/definitions/image/properties/creds", "#/definitions/registryCredential/properties/fromRegistryCreds") {
				break
			}
			problem(field, "BLUEPRINT_CAPABILITY_UNSUPPORTED", "Blueprint registry credentials are not available on bex; bind an authorized registry credential through the service API")
		case "previews", "previewsEnabled", "previewsExpireAfterDays", "previewPlan", "previewDiskSizeGB", "pullRequestPreviewsEnabled":
			if !blueprintCapabilityUnsupported(registry,
				"#/allOf/1/properties/previews", "#/allOf/1/properties/previewsEnabled", "#/allOf/1/properties/previewsExpireAfterDays",
				"#/definitions/serverService/properties/previews", "#/definitions/serverService/properties/previewPlan", "#/definitions/serverService/properties/pullRequestPreviewsEnabled",
				"#/definitions/staticService/properties/previews", "#/definitions/staticService/properties/pullRequestPreviewsEnabled",
				"#/definitions/database/properties/previewPlan", "#/definitions/database/properties/previewDiskSizeGB", "#/definitions/redisServer/properties/previewPlan",
			) {
				break
			}
			problem(field, "BLUEPRINT_CAPABILITY_UNSUPPORTED", "preview environments are not available on bex")
		case "previewValue":
			if !blueprintCapabilityUnsupported(registry, "#/definitions/envVarFromKeyValue/properties/previewValue") {
				break
			}
			problem(field, "BLUEPRINT_CAPABILITY_UNSUPPORTED", "previewValue is not available on bex because preview environments are not available")
		case "autoDeployTrigger":
			if trigger, _ := child.(string); trigger == "checksPass" && blueprintEnumCapabilityUnsupported(registry, "#/definitions/autoDeployTrigger/enum", `"checksPass"`) {
				problem(field, "BLUEPRINT_CAPABILITY_UNSUPPORTED", "autoDeployTrigger: checksPass requires CI-check gating, which is not available on bex")
			}
		}
		if field == "x-bex" {
			continue // local extension vocabulary is validated by its own schema.
		}
		problems = append(problems, blueprintCapabilityProblems(child, append(path, field), locations, registry)...)
	}
	return problems
}

// blueprintCapabilityUnsupported makes reviewed registry state authoritative
// for a refusal. A field can occur in several schema definitions; it remains
// unavailable only while every applicable definition is classified unsupported.
// Per-resource handlers refine these candidates as their equivalence work lands.
func blueprintCapabilityUnsupported(registry *BlueprintCapabilityRegistry, pointers ...string) bool {
	if registry == nil || len(pointers) == 0 {
		return false
	}
	for _, pointer := range pointers {
		capability, ok := registry.Fields[pointer]
		if !ok || capability.State != BlueprintCapabilityUnsupported {
			return false
		}
	}
	return true
}

func blueprintEnumCapabilityUnsupported(registry *BlueprintCapabilityRegistry, pointer, encodedValue string) bool {
	if registry == nil {
		return false
	}
	capability, ok := registry.EnumValues[pointer][encodedValue]
	return ok && capability.State == BlueprintCapabilityUnsupported
}

func parseBlueprintSource(manifest string) (*BlueprintSource, []BlueprintSourceProblem) {
	decoder := yaml.NewDecoder(strings.NewReader(manifest))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return &BlueprintSource{Locations: map[string]BlueprintSourceLocation{}}, []BlueprintSourceProblem{{
				Code: "BLUEPRINT_YAML_EMPTY", Path: "#", Message: "Blueprint must contain one YAML document",
			}}
		}
		line, column := yamlSyntaxLocation(err)
		return &BlueprintSource{Locations: map[string]BlueprintSourceLocation{}}, []BlueprintSourceProblem{{
			Code: "BLUEPRINT_YAML_SYNTAX", Path: "#", Message: "Blueprint is not valid YAML", Line: line, Column: column,
		}}
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return &BlueprintSource{Locations: map[string]BlueprintSourceLocation{}}, []BlueprintSourceProblem{{
			Code: "BLUEPRINT_YAML_MULTIDOC", Path: "#", Message: "Blueprint must contain exactly one YAML document", Line: extra.Line, Column: extra.Column,
		}}
	}

	source := &BlueprintSource{Locations: map[string]BlueprintSourceLocation{}}
	if len(document.Content) != 1 {
		return source, []BlueprintSourceProblem{{Code: "BLUEPRINT_YAML_EMPTY", Path: "#", Message: "Blueprint must contain one YAML document"}}
	}
	value, problems := yamlNodeToBlueprintValue(document.Content[0], nil, source.Locations)
	source.Value = value
	return source, problems
}

var yamlSyntaxLine = regexp.MustCompile(`(?i)line\s+(\d+)`)

func yamlSyntaxLocation(err error) (line, column int) {
	match := yamlSyntaxLine.FindStringSubmatch(err.Error())
	if len(match) != 2 {
		return 0, 0
	}
	line, _ = strconv.Atoi(match[1])
	return line, 1
}

func yamlNodeToBlueprintValue(node *yaml.Node, path []string, locations map[string]BlueprintSourceLocation) (any, []BlueprintSourceProblem) {
	pointer := renderSchemaPointer(path)
	locations[pointer] = BlueprintSourceLocation{Line: node.Line, Column: node.Column}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return nil, []BlueprintSourceProblem{{Code: "BLUEPRINT_YAML_EMPTY", Path: pointer, Message: "Blueprint must contain one YAML document", Line: node.Line, Column: node.Column}}
		}
		return yamlNodeToBlueprintValue(node.Content[0], path, locations)
	case yaml.AliasNode:
		return nil, []BlueprintSourceProblem{{Code: "BLUEPRINT_YAML_ALIAS", Path: pointer, Message: "YAML aliases are not supported in Blueprints", Line: node.Line, Column: node.Column}}
	case yaml.SequenceNode:
		values := make([]any, len(node.Content))
		var problems []BlueprintSourceProblem
		for i, child := range node.Content {
			value, childProblems := yamlNodeToBlueprintValue(child, append(path, strconv.Itoa(i)), locations)
			values[i] = value
			problems = append(problems, childProblems...)
		}
		return values, problems
	case yaml.MappingNode:
		values := map[string]any{}
		seen := map[string]BlueprintSourceLocation{}
		var problems []BlueprintSourceProblem
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				problems = append(problems, BlueprintSourceProblem{Code: "BLUEPRINT_YAML_MAPPING_KEY", Path: pointer, Message: "Blueprint mapping keys must be strings", Line: key.Line, Column: key.Column})
				continue
			}
			childPath := append(path, key.Value)
			childPointer := renderSchemaPointer(childPath)
			if first, duplicate := seen[key.Value]; duplicate {
				problems = append(problems, BlueprintSourceProblem{
					Code: "BLUEPRINT_DUPLICATE_KEY", Path: childPointer,
					Message: fmt.Sprintf("duplicate key %q (first declared at line %d, column %d)", key.Value, first.Line, first.Column),
					Line:    key.Line, Column: key.Column,
				})
				continue
			}
			seen[key.Value] = BlueprintSourceLocation{Line: key.Line, Column: key.Column}
			child, childProblems := yamlNodeToBlueprintValue(value, childPath, locations)
			values[key.Value] = child
			problems = append(problems, childProblems...)
		}
		return values, problems
	case yaml.ScalarNode:
		return blueprintJSONScalar(node, pointer)
	default:
		return nil, []BlueprintSourceProblem{{Code: "BLUEPRINT_YAML_NODE", Path: pointer, Message: "Blueprint contains an unsupported YAML node", Line: node.Line, Column: node.Column}}
	}
}

func blueprintJSONScalar(node *yaml.Node, pointer string) (any, []BlueprintSourceProblem) {
	problem := func(code, message string) (any, []BlueprintSourceProblem) {
		return nil, []BlueprintSourceProblem{{Code: code, Path: pointer, Message: message, Line: node.Line, Column: node.Column}}
	}
	switch node.Tag {
	case "!!str":
		return node.Value, nil
	case "!!null":
		return nil, nil
	case "!!bool":
		value, err := strconv.ParseBool(node.Value)
		if err != nil {
			return problem("BLUEPRINT_YAML_SCALAR", "Blueprint contains an invalid boolean")
		}
		return value, nil
	case "!!int":
		value := strings.ReplaceAll(node.Value, "_", "")
		if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") || strings.HasPrefix(value, "0o") || strings.HasPrefix(value, "0O") || strings.HasPrefix(value, "0b") || strings.HasPrefix(value, "0B") {
			return problem("BLUEPRINT_YAML_SCALAR", "Blueprint numbers must use JSON decimal syntax")
		}
		integer, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return problem("BLUEPRINT_YAML_SCALAR", "Blueprint contains an invalid integer")
		}
		return integer, nil
	case "!!float":
		value := strings.ReplaceAll(node.Value, "_", "")
		if strings.EqualFold(value, ".nan") || strings.Contains(strings.ToLower(value), "inf") {
			return problem("BLUEPRINT_YAML_SCALAR", "Blueprint numbers must be finite JSON numbers")
		}
		floating, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return problem("BLUEPRINT_YAML_SCALAR", "Blueprint contains an invalid number")
		}
		return floating, nil
	case "!!timestamp":
		return problem("BLUEPRINT_YAML_SCALAR", "quote timestamp-looking values so their string type is explicit")
	default:
		return problem("BLUEPRINT_YAML_SCALAR", "Blueprint contains an unsupported YAML scalar type")
	}
}

func compileRenderBlueprintSchema() (*jsonschema.Schema, error) {
	data, err := renderBlueprintValidationSchema()
	if err != nil {
		return nil, err
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode Render Blueprint validation schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://render.com/schema/render.yaml.json", document); err != nil {
		return nil, fmt.Errorf("register Render Blueprint validation schema: %w", err)
	}
	schema, err := compiler.Compile("https://render.com/schema/render.yaml.json")
	if err != nil {
		return nil, fmt.Errorf("compile Render Blueprint validation schema: %w", err)
	}
	return schema, nil
}

func renderBlueprintValidationSchema() ([]byte, error) {
	var document map[string]any
	if err := json.Unmarshal(renderBlueprintSchemaSource, &document); err != nil {
		return nil, fmt.Errorf("decode embedded Render Blueprint schema: %w", err)
	}
	var extension map[string]any
	if err := json.Unmarshal(bexBlueprintExtensionSchemaSource, &extension); err != nil {
		return nil, fmt.Errorf("decode x-bex extension schema: %w", err)
	}
	delete(extension, "$id")
	if err := normalizeRenderResourceContainers(document); err != nil {
		return nil, err
	}
	if err := addBlueprintExtension(document, extension); err != nil {
		return nil, err
	}
	closeBlueprintSchemaObjects(document)
	return json.Marshal(document)
}

// normalizeRenderResourceContainers expands the Render schema's three
// resource containers before compiling it. The reviewed schema expresses them
// with $ref + allOf + unevaluatedProperties. jsonschema/v6 validates the
// subschemas correctly but does not carry evaluated-property annotations
// through that combination, causing valid services/databases to be reported
// as a "false schema". Expanding the equivalent closed objects keeps the
// upstream vocabulary and closed-world behavior intact without accepting a
// second dialect.
func normalizeRenderResourceContainers(document map[string]any) error {
	definitions, ok := document["definitions"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no definitions")
	}
	resources, ok := definitions["resources"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no resources definition")
	}
	resourceProperties, ok := resources["properties"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no resource properties")
	}

	rootAllOf, ok := document["allOf"].([]any)
	if !ok || len(rootAllOf) < 2 {
		return fmt.Errorf("embedded Render Blueprint schema has no root resource container")
	}
	rootTail, ok := rootAllOf[1].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has invalid root resource container")
	}
	rootProperties, ok := rootTail["properties"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no root properties")
	}
	document["properties"] = mergedBlueprintSchemaProperties(resourceProperties, rootProperties)
	document["additionalProperties"] = false
	delete(document, "allOf")
	delete(document, "unevaluatedProperties")

	environment, ok := definitions["environment"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no environment definition")
	}
	environmentAllOf, ok := environment["allOf"].([]any)
	if !ok || len(environmentAllOf) < 2 {
		return fmt.Errorf("embedded Render Blueprint schema has invalid environment resource container")
	}
	environmentTail, ok := environmentAllOf[1].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has invalid environment properties")
	}
	environmentProperties, ok := environmentTail["properties"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no environment properties")
	}
	environment["properties"] = mergedBlueprintSchemaProperties(resourceProperties, environmentProperties)
	environment["additionalProperties"] = false
	delete(environment, "allOf")
	delete(environment, "unevaluatedProperties")

	rootProperties, _ = document["properties"].(map[string]any)
	ungrouped, ok := rootProperties["ungrouped"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no ungrouped definition")
	}
	ungrouped["properties"] = mergedBlueprintSchemaProperties(resourceProperties, nil)
	ungrouped["additionalProperties"] = false
	delete(ungrouped, "allOf")
	delete(ungrouped, "unevaluatedProperties")
	return nil
}

func mergedBlueprintSchemaProperties(groups ...map[string]any) map[string]any {
	merged := map[string]any{}
	for _, group := range groups {
		for key, value := range group {
			merged[key] = value
		}
	}
	return merged
}

// Render's published schema leaves a few nested object definitions open (for
// example scaling). Blueprint parity is intentionally closed-world: a typo is
// never allowed to disappear before an adapter sees it. Closing only schema
// nodes that already declare named properties preserves all documented fields
// while making the compiler's unknown-field contract uniform.
func closeBlueprintSchemaObjects(value any) {
	object, ok := value.(map[string]any)
	if !ok {
		switch list := value.(type) {
		case []any:
			for _, item := range list {
				closeBlueprintSchemaObjects(item)
			}
		}
		return
	}
	if _, hasProperties := object["properties"].(map[string]any); hasProperties {
		if _, hasAdditionalProperties := object["additionalProperties"]; !hasAdditionalProperties {
			object["additionalProperties"] = false
		}
	}
	for _, child := range object {
		closeBlueprintSchemaObjects(child)
	}
}

func addBlueprintExtension(document, extension map[string]any) error {
	rootProperties, ok := document["properties"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no root properties")
	}
	rootProperties["x-bex"] = extension

	definitions, ok := document["definitions"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no definitions")
	}
	for _, name := range []string{"serverService", "staticService", "cronService", "redisServer"} {
		definition, ok := definitions[name].(map[string]any)
		if !ok {
			return fmt.Errorf("embedded Render Blueprint schema has no %s definition", name)
		}
		properties, ok := definition["properties"].(map[string]any)
		if !ok {
			return fmt.Errorf("embedded Render Blueprint schema has no %s properties", name)
		}
		properties["x-bex"] = extension
	}
	return nil
}

func blueprintSchemaProblems(err error, locations map[string]BlueprintSourceLocation) []BlueprintSourceProblem {
	var validation *jsonschema.ValidationError
	if !errors.As(err, &validation) {
		return []BlueprintSourceProblem{{Code: "BLUEPRINT_SCHEMA_INVALID", Path: "#", Message: "Blueprint does not satisfy the reviewed Render schema"}}
	}
	var problems []BlueprintSourceProblem
	appendBlueprintSchemaProblems(validation, locations, &problems)
	return problems
}

func appendBlueprintSchemaProblems(validation *jsonschema.ValidationError, locations map[string]BlueprintSourceLocation, problems *[]BlueprintSourceProblem) {
	if len(validation.Causes) > 0 {
		// A failed anyOf reports every alternative (Redis, cron, server, static;
		// likewise each env-var form). Only the closest alternative is useful to
		// a Blueprint author. Selecting the branch with the fewest leaf failures
		// retains the schema's strictness while avoiding misleading errors from
		// unrelated service kinds.
		if _, isAnyOf := validation.ErrorKind.(*kind.AnyOf); isAnyOf && len(validation.Causes) > 1 {
			closest := validation.Causes[0]
			for _, cause := range validation.Causes[1:] {
				if blueprintSchemaLeafCount(cause) < blueprintSchemaLeafCount(closest) {
					closest = cause
				}
			}
			appendBlueprintSchemaProblems(closest, locations, problems)
			return
		}
		for _, cause := range validation.Causes {
			appendBlueprintSchemaProblems(cause, locations, problems)
		}
		return
	}
	path := renderSchemaPointer(validation.InstanceLocation)
	if additional, ok := validation.ErrorKind.(*kind.AdditionalProperties); ok && len(additional.Properties) == 1 {
		path = renderSchemaPointer(append(append([]string(nil), validation.InstanceLocation...), additional.Properties[0]))
	}
	location := lookupBlueprintLocation(path, locations)
	message := "Blueprint does not satisfy the reviewed Render schema"
	if validation.ErrorKind != nil {
		message = validation.Error()
	}
	*problems = append(*problems, BlueprintSourceProblem{Code: "BLUEPRINT_SCHEMA_INVALID", Path: path, Message: message, Line: location.Line, Column: location.Column})
}

func blueprintSchemaLeafCount(validation *jsonschema.ValidationError) int {
	if len(validation.Causes) == 0 {
		return 1
	}
	count := 0
	for _, cause := range validation.Causes {
		count += blueprintSchemaLeafCount(cause)
	}
	return count
}

func lookupBlueprintLocation(path string, locations map[string]BlueprintSourceLocation) BlueprintSourceLocation {
	for path != "" {
		if location, ok := locations[path]; ok {
			return location
		}
		if path == "#" {
			break
		}
		path = path[:strings.LastIndex(path, "/")]
	}
	return BlueprintSourceLocation{}
}

func sortBlueprintSourceProblems(problems []BlueprintSourceProblem) []BlueprintSourceProblem {
	if len(problems) < 2 {
		return problems
	}
	sort.SliceStable(problems, func(i, j int) bool {
		if problems[i].Path != problems[j].Path {
			return problems[i].Path < problems[j].Path
		}
		if problems[i].Line != problems[j].Line {
			return problems[i].Line < problems[j].Line
		}
		if problems[i].Column != problems[j].Column {
			return problems[i].Column < problems[j].Column
		}
		return problems[i].Code < problems[j].Code
	})
	return problems
}
