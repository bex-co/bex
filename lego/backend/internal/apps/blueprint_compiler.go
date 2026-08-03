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
	return blueprintCapabilityProblemsAt(value, path, locations, registry, blueprintCapabilityContext{kind: blueprintCapabilityRoot})
}

// blueprintCapabilityContext identifies the reviewed schema object currently
// being visited. It is deliberately independent of the Go decoder structs: a
// field must pass this gate before an adapter is allowed to approximate it.
// base is used for inline schema objects whose registry entries are not a
// named definition (for example serverService.scaling).
type blueprintCapabilityContext struct {
	kind blueprintCapabilityKind
	base string
}

type blueprintCapabilityKind string

const (
	blueprintCapabilityRoot               blueprintCapabilityKind = "root"
	blueprintCapabilityResources          blueprintCapabilityKind = "resources"
	blueprintCapabilityProject            blueprintCapabilityKind = "project"
	blueprintCapabilityEnvironment        blueprintCapabilityKind = "environment"
	blueprintCapabilityServices           blueprintCapabilityKind = "services"
	blueprintCapabilityServer             blueprintCapabilityKind = "serverService"
	blueprintCapabilityCron               blueprintCapabilityKind = "cronService"
	blueprintCapabilityStatic             blueprintCapabilityKind = "staticService"
	blueprintCapabilityDatabase           blueprintCapabilityKind = "database"
	blueprintCapabilityKeyValue           blueprintCapabilityKind = "redisServer"
	blueprintCapabilityEnvGroup           blueprintCapabilityKind = "envVarGroup"
	blueprintCapabilityEnvKeyValue        blueprintCapabilityKind = "envVarFromKeyValue"
	blueprintCapabilityEnvDatabase        blueprintCapabilityKind = "envVarFromDatabase"
	blueprintCapabilityEnvService         blueprintCapabilityKind = "envVarFromService"
	blueprintCapabilityEnvGroupReference  blueprintCapabilityKind = "envVarFromGroup"
	blueprintCapabilityImage              blueprintCapabilityKind = "image"
	blueprintCapabilityRegistryCredential blueprintCapabilityKind = "registryCredential"
	blueprintCapabilityBuildFilter        blueprintCapabilityKind = "buildFilter"
	blueprintCapabilityDisk               blueprintCapabilityKind = "disk"
	blueprintCapabilityIPAllowList        blueprintCapabilityKind = "ipAllowList"
	blueprintCapabilityReadReplica        blueprintCapabilityKind = "readReplica"
	blueprintCapabilityHeader             blueprintCapabilityKind = "header"
	blueprintCapabilityRoute              blueprintCapabilityKind = "route"
	blueprintCapabilityRootPreviews       blueprintCapabilityKind = "rootPreviews"
	blueprintCapabilityServicePreviews    blueprintCapabilityKind = "servicePreviews"
	blueprintCapabilityStaticPreviews     blueprintCapabilityKind = "staticServicePreviews"
)

func blueprintCapabilityProblemsAt(value any, path []string, locations map[string]BlueprintSourceLocation, registry *BlueprintCapabilityRegistry, context blueprintCapabilityContext) []BlueprintSourceProblem {
	if list, ok := value.([]any); ok {
		var problems []BlueprintSourceProblem
		for index, item := range list {
			itemContext := context
			if context.kind == blueprintCapabilityServices {
				itemContext = blueprintServiceCapabilityContext(item)
			} else if context.kind == blueprintCapabilityEnvKeyValue || context.kind == blueprintCapabilityEnvDatabase || context.kind == blueprintCapabilityEnvService || context.kind == blueprintCapabilityEnvGroupReference {
				itemContext = blueprintEnvVarCapabilityContext(item)
			}
			problems = append(problems, blueprintCapabilityProblemsAt(item, append(path, strconv.Itoa(index)), locations, registry, itemContext)...)
		}
		return problems
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	var problems []BlueprintSourceProblem
	problems = append(problems, blueprintPrebuiltImageProblems(object, path, locations, context)...)
	problems = append(problems, blueprintServiceRuntimeProblems(object, path, locations, context)...)
	for field, child := range object {
		fieldPath := append(append([]string(nil), path...), field)
		pointer := renderSchemaPointer(fieldPath)
		location := lookupBlueprintLocation(pointer, locations)
		problem := func(code, message string) {
			problems = append(problems, BlueprintSourceProblem{Code: code, Path: pointer, Message: message, Line: location.Line, Column: location.Column})
		}
		if field == "builder" {
			problem("BLUEPRINT_EXTENSION_REQUIRED", "bex build strategy must be written as x-bex.builder; builder is not a Render Blueprint field")
			continue
		}
		if field == "x-bex" {
			continue // local extension vocabulary is validated by its own schema.
		}
		capabilityPointer := blueprintFieldCapabilityPointer(context, field)
		if blueprintCapabilityUnsupported(registry, capabilityPointer) {
			problem("BLUEPRINT_CAPABILITY_UNSUPPORTED", blueprintUnsupportedCapabilityMessage(field, capabilityPointer))
			continue
		}
		if enumPointer := blueprintFieldEnumCapabilityPointer(context, field); enumPointer != "" && blueprintEnumCapabilityUnsupported(registry, enumPointer, blueprintEncodedValue(child)) {
			problem("BLUEPRINT_CAPABILITY_UNSUPPORTED", blueprintUnsupportedEnumMessage(field, enumPointer))
			continue
		}
		childContext := blueprintChildCapabilityContext(context, field, child)
		problems = append(problems, blueprintCapabilityProblemsAt(child, fieldPath, locations, registry, childContext)...)
	}
	return problems
}

// blueprintServiceRuntimeProblems covers fields whose meaning depends on a
// selected build strategy. The registry classifies the field as translated
// because native runtimes implement it; this check prevents a Dockerfile or
// prebuilt-image service from retaining the field without an effect.
func blueprintServiceRuntimeProblems(object map[string]any, path []string, locations map[string]BlueprintSourceLocation, context blueprintCapabilityContext) []BlueprintSourceProblem {
	if context.kind != blueprintCapabilityServer && context.kind != blueprintCapabilityCron {
		return nil
	}
	runtime, _ := object["runtime"].(string)
	runtime = strings.ToLower(strings.TrimSpace(runtime))
	_, hasImage := object["image"]
	var problems []BlueprintSourceProblem
	for _, rule := range []struct {
		field   string
		invalid bool
		message string
	}{
		{
			field:   "buildCommand",
			invalid: !hasImage && !blueprintNativeRuntime(runtime),
			message: "buildCommand requires a native runtime (elixir, go, node, python, ruby, or rust); bex does not run it for Dockerfile or prebuilt-image services",
		},
		{
			field:   "dockerCommand",
			invalid: runtime != "docker",
			message: "dockerCommand requires runtime: docker",
		},
	} {
		if _, declared := object[rule.field]; !declared || !rule.invalid {
			continue
		}
		fieldPath := append(append([]string(nil), path...), rule.field)
		pointer := renderSchemaPointer(fieldPath)
		location := lookupBlueprintLocation(pointer, locations)
		problems = append(problems, BlueprintSourceProblem{
			Code:    "BLUEPRINT_CAPABILITY_INCOMPATIBLE",
			Path:    pointer,
			Message: rule.message,
			Line:    location.Line,
			Column:  location.Column,
		})
	}
	return problems
}

func blueprintNativeRuntime(runtime string) bool {
	switch runtime {
	case "elixir", "go", "node", "python", "ruby", "rust":
		return true
	default:
		return false
	}
}

type prebuiltImageSourceField struct {
	blueprintName string
	createName    string
}

// prebuiltImageSourceFields is the shared policy for settings that only have
// meaning while a service builds from Git. The Blueprint compiler reports every
// declared field; the direct create API checks the subset it exposes.
var prebuiltImageSourceFields = []prebuiltImageSourceField{
	{blueprintName: "repo", createName: "repo"},
	{blueprintName: "branch", createName: "branch"},
	{blueprintName: "rootDir", createName: "rootDirectory"},
	{blueprintName: "buildFilter", createName: "buildFilter"},
	{blueprintName: "buildCommand", createName: "buildCommand"},
	{blueprintName: "dockerfilePath", createName: "dockerfilePath"},
	{blueprintName: "autoDeploy", createName: "autoDeploy"},
	{blueprintName: "autoDeployTrigger"},
}

func prebuiltImageSourceFieldMessage(field string) string {
	if field == "repo" {
		return "a service cannot declare both image and repo; remove repo or deploy from repo instead"
	}
	return "prebuilt image services cannot declare " + field + "; remove it or deploy from repo instead"
}

// blueprintPrebuiltImageProblems rejects source-build settings on a service
// that supplies a prebuilt image. The App controller intentionally skips all
// build and git-push behavior for such a service, so retaining these settings
// in the CR would make a successful Blueprint apply a silent no-op.
func blueprintPrebuiltImageProblems(object map[string]any, path []string, locations map[string]BlueprintSourceLocation, context blueprintCapabilityContext) []BlueprintSourceProblem {
	if context.kind != blueprintCapabilityServer && context.kind != blueprintCapabilityCron && context.kind != blueprintCapabilityStatic {
		return nil
	}
	runtime, _ := object["runtime"].(string)
	_, hasImage := object["image"]
	if !hasImage && !strings.EqualFold(runtime, "image") {
		return nil
	}
	problems := make([]BlueprintSourceProblem, 0, len(prebuiltImageSourceFields))
	for _, sourceField := range prebuiltImageSourceFields {
		if _, declared := object[sourceField.blueprintName]; !declared {
			continue
		}
		fieldPath := append(append([]string(nil), path...), sourceField.blueprintName)
		pointer := renderSchemaPointer(fieldPath)
		location := lookupBlueprintLocation(pointer, locations)
		problems = append(problems, BlueprintSourceProblem{
			Code:    "BLUEPRINT_CAPABILITY_INCOMPATIBLE",
			Path:    pointer,
			Message: prebuiltImageSourceFieldMessage(sourceField.blueprintName),
			Line:    location.Line,
			Column:  location.Column,
		})
	}
	return problems
}

func blueprintFieldCapabilityPointer(context blueprintCapabilityContext, field string) string {
	if context.base != "" {
		return context.base + "/properties/" + field
	}
	switch context.kind {
	case blueprintCapabilityRoot:
		switch field {
		case "services", "databases", "envVarGroups":
			return "#/definitions/resources/properties/" + field
		case "previews", "previewsEnabled", "previewsExpireAfterDays", "projects", "ungrouped", "version":
			return "#/allOf/1/properties/" + field
		}
	case blueprintCapabilityResources:
		return "#/definitions/resources/properties/" + field
	case blueprintCapabilityEnvironment:
		switch field {
		case "services", "databases", "envVarGroups":
			return "#/definitions/resources/properties/" + field
		default:
			return "#/definitions/environment/allOf/1/properties/" + field
		}
	case blueprintCapabilityIPAllowList:
		return "#/definitions/ipAllowList/items/properties/" + field
	}
	if context.kind == "" {
		return ""
	}
	return "#/definitions/" + string(context.kind) + "/properties/" + field
}

func blueprintChildCapabilityContext(context blueprintCapabilityContext, field string, child any) blueprintCapabilityContext {
	if context.base != "" {
		return blueprintCapabilityContext{base: context.base + "/properties/" + field}
	}
	switch context.kind {
	case blueprintCapabilityRoot:
		switch field {
		case "services":
			return blueprintCapabilityContext{kind: blueprintCapabilityServices}
		case "databases":
			return blueprintCapabilityContext{kind: blueprintCapabilityDatabase}
		case "envVarGroups":
			return blueprintCapabilityContext{kind: blueprintCapabilityEnvGroup}
		case "ungrouped":
			return blueprintCapabilityContext{kind: blueprintCapabilityResources}
		case "projects":
			return blueprintCapabilityContext{kind: blueprintCapabilityProject}
		case "previews":
			return blueprintCapabilityContext{kind: blueprintCapabilityRootPreviews}
		}
	case blueprintCapabilityResources:
		switch field {
		case "services":
			return blueprintCapabilityContext{kind: blueprintCapabilityServices}
		case "databases":
			return blueprintCapabilityContext{kind: blueprintCapabilityDatabase}
		case "envVarGroups":
			return blueprintCapabilityContext{kind: blueprintCapabilityEnvGroup}
		}
	case blueprintCapabilityProject:
		if field == "environments" {
			return blueprintCapabilityContext{kind: blueprintCapabilityEnvironment}
		}
	case blueprintCapabilityEnvironment:
		switch field {
		case "services":
			return blueprintCapabilityContext{kind: blueprintCapabilityServices}
		case "databases":
			return blueprintCapabilityContext{kind: blueprintCapabilityDatabase}
		case "envVarGroups":
			return blueprintCapabilityContext{kind: blueprintCapabilityEnvGroup}
		case "networking", "permissions":
			return blueprintCapabilityContext{base: "#/definitions/environment/allOf/1/properties/" + field}
		}
	case blueprintCapabilityServer, blueprintCapabilityCron, blueprintCapabilityStatic:
		switch field {
		case "image":
			return blueprintCapabilityContext{kind: blueprintCapabilityImage}
		case "registryCredential":
			return blueprintCapabilityContext{kind: blueprintCapabilityRegistryCredential}
		case "envVars":
			return blueprintCapabilityContext{kind: blueprintCapabilityEnvKeyValue}
		case "buildFilter":
			return blueprintCapabilityContext{kind: blueprintCapabilityBuildFilter}
		case "disk":
			return blueprintCapabilityContext{kind: blueprintCapabilityDisk}
		case "ipAllowList":
			return blueprintCapabilityContext{kind: blueprintCapabilityIPAllowList}
		case "previews":
			if context.kind == blueprintCapabilityStatic {
				return blueprintCapabilityContext{kind: blueprintCapabilityStaticPreviews}
			}
			return blueprintCapabilityContext{kind: blueprintCapabilityServicePreviews}
		case "scaling", "maintenanceMode":
			return blueprintCapabilityContext{base: "#/definitions/" + string(context.kind) + "/properties/" + field}
		case "headers":
			return blueprintCapabilityContext{kind: blueprintCapabilityHeader}
		case "routes":
			return blueprintCapabilityContext{kind: blueprintCapabilityRoute}
		}
	case blueprintCapabilityDatabase:
		switch field {
		case "ipAllowList":
			return blueprintCapabilityContext{kind: blueprintCapabilityIPAllowList}
		case "readReplicas":
			return blueprintCapabilityContext{kind: blueprintCapabilityReadReplica}
		case "highAvailability":
			return blueprintCapabilityContext{base: "#/definitions/database/properties/highAvailability"}
		}
	case blueprintCapabilityKeyValue:
		if field == "ipAllowList" {
			return blueprintCapabilityContext{kind: blueprintCapabilityIPAllowList}
		}
	case blueprintCapabilityEnvGroup:
		if field == "envVars" {
			return blueprintCapabilityContext{kind: blueprintCapabilityEnvKeyValue}
		}
	case blueprintCapabilityEnvKeyValue, blueprintCapabilityEnvDatabase, blueprintCapabilityEnvService, blueprintCapabilityEnvGroupReference:
		switch field {
		case "fromDatabase":
			return blueprintCapabilityContext{base: "#/definitions/envVarFromDatabase/properties/fromDatabase"}
		case "fromService":
			return blueprintCapabilityContext{base: "#/definitions/envVarFromService/properties/fromService"}
		}
	case blueprintCapabilityRootPreviews, blueprintCapabilityServicePreviews, blueprintCapabilityStaticPreviews:
		return blueprintCapabilityContext{}
	}
	return blueprintCapabilityContext{}
}

func blueprintServiceCapabilityContext(value any) blueprintCapabilityContext {
	service, _ := value.(map[string]any)
	serviceType, _ := service["type"].(string)
	switch serviceType {
	case "keyvalue", "redis":
		return blueprintCapabilityContext{kind: blueprintCapabilityKeyValue}
	case "cron":
		return blueprintCapabilityContext{kind: blueprintCapabilityCron}
	}
	if runtime, _ := service["runtime"].(string); runtime == "static" {
		return blueprintCapabilityContext{kind: blueprintCapabilityStatic}
	}
	return blueprintCapabilityContext{kind: blueprintCapabilityServer}
}

func blueprintEnvVarCapabilityContext(value any) blueprintCapabilityContext {
	entry, _ := value.(map[string]any)
	switch {
	case entry["fromDatabase"] != nil:
		return blueprintCapabilityContext{kind: blueprintCapabilityEnvDatabase}
	case entry["fromService"] != nil:
		return blueprintCapabilityContext{kind: blueprintCapabilityEnvService}
	case entry["fromGroup"] != nil:
		return blueprintCapabilityContext{kind: blueprintCapabilityEnvGroupReference}
	default:
		return blueprintCapabilityContext{kind: blueprintCapabilityEnvKeyValue}
	}
}

func blueprintFieldEnumCapabilityPointer(context blueprintCapabilityContext, field string) string {
	if context.base != "" {
		switch context.base {
		case "#/definitions/environment/allOf/1/properties/networking":
			if field == "isolation" {
				return context.base + "/properties/isolation/enum"
			}
		case "#/definitions/environment/allOf/1/properties/permissions":
			if field == "protection" {
				return context.base + "/properties/protection/enum"
			}
		case "#/definitions/envVarFromDatabase/properties/fromDatabase":
			if field == "property" {
				return "#/definitions/databaseEnvVarProperty/enum"
			}
		case "#/definitions/envVarFromService/properties/fromService":
			if field == "property" {
				return "#/definitions/serviceEnvVarProperty/enum"
			}
			if field == "type" {
				return "#/definitions/serviceType/enum"
			}
		}
	}
	switch context.kind {
	case blueprintCapabilityServer:
		switch field {
		case "type", "renderSubdomainPolicy":
			return "#/definitions/serverService/properties/" + field + "/enum"
		case "runtime":
			return "#/definitions/runtime/enum"
		case "autoDeployTrigger":
			return "#/definitions/autoDeployTrigger/enum"
		case "plan":
			return "#/definitions/plan/enum"
		}
	case blueprintCapabilityCron:
		switch field {
		case "runtime":
			return "#/definitions/runtime/enum"
		case "autoDeployTrigger":
			return "#/definitions/autoDeployTrigger/enum"
		case "plan":
			return "#/definitions/plan/enum"
		}
	case blueprintCapabilityStatic:
		switch field {
		case "renderSubdomainPolicy":
			return "#/definitions/staticService/properties/renderSubdomainPolicy/enum"
		case "autoDeployTrigger":
			return "#/definitions/autoDeployTrigger/enum"
		}
	case blueprintCapabilityDatabase:
		switch field {
		case "plan":
			return "#/definitions/plan/enum"
		case "postgresMajorVersion":
			return "#/definitions/database/properties/postgresMajorVersion/enum"
		case "connectionPool":
			return "#/definitions/connectionPool/enum"
		case "region":
			return "#/definitions/region/enum"
		}
	case blueprintCapabilityKeyValue:
		switch field {
		case "type", "maxmemoryPolicy", "persistenceMode":
			return "#/definitions/redisServer/properties/" + field + "/enum"
		case "plan":
			return "#/definitions/plan/enum"
		case "region":
			return "#/definitions/region/enum"
		}
	case blueprintCapabilityRoute:
		if field == "type" {
			return "#/definitions/route/properties/type/enum"
		}
	case blueprintCapabilityRootPreviews, blueprintCapabilityServicePreviews, blueprintCapabilityStaticPreviews:
		if field == "generation" {
			return "#/definitions/previewsGeneration/enum"
		}
	}
	return ""
}

func blueprintEncodedValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func blueprintUnsupportedCapabilityMessage(field, pointer string) string {
	switch pointer {
	case "#/definitions/serverService/properties/disk":
		return "persistent service disks are not available on bex"
	case "#/definitions/serverService/properties/dockerContext", "#/definitions/cronService/properties/dockerContext":
		return "dockerContext is not available on bex because its build-context semantics cannot be represented exactly"
	case "#/definitions/serverService/properties/registryCredential", "#/definitions/cronService/properties/registryCredential", "#/definitions/image/properties/creds", "#/definitions/registryCredential/properties/fromRegistryCreds":
		return "Blueprint registry credentials are not available on bex; bind an authorized registry credential through the service API"
	case "#/allOf/1/properties/previews", "#/allOf/1/properties/previewsEnabled", "#/allOf/1/properties/previewsExpireAfterDays", "#/definitions/serverService/properties/previews", "#/definitions/serverService/properties/previewPlan", "#/definitions/serverService/properties/pullRequestPreviewsEnabled", "#/definitions/staticService/properties/previews", "#/definitions/staticService/properties/pullRequestPreviewsEnabled", "#/definitions/database/properties/previewPlan", "#/definitions/database/properties/previewDiskSizeGB", "#/definitions/redisServer/properties/previewPlan":
		return "preview environments are not available on bex"
	case "#/definitions/envVarFromKeyValue/properties/previewValue":
		return "previewValue is not available on bex because preview environments are not available"
	case "#/definitions/serverService/properties/region", "#/definitions/cronService/properties/region", "#/definitions/database/properties/region", "#/definitions/redisServer/properties/region":
		return "per-resource region placement is not available on bex"
	default:
		return fmt.Sprintf("%s is not available on bex", field)
	}
}

func blueprintUnsupportedEnumMessage(field, pointer string) string {
	if pointer == "#/definitions/autoDeployTrigger/enum" {
		return "autoDeployTrigger: checksPass requires CI-check gating, which is not available on bex"
	}
	return fmt.Sprintf("%s uses an unsupported Render Blueprint value", field)
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
