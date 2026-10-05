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
	"github.com/bex-co/bex/lego/backend/internal/postgres"
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
	// authoredPaths maps normalized alias paths back to the customer's YAML.
	authoredPaths map[string]string
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
	if problems := normalizeBlueprintAliases(source, registry); len(problems) > 0 {
		return source, sortBlueprintSourceProblems(problems)
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
		problems = append(problems, blueprintSchemaProblems(err, source.Locations, source.Value)...)
	}
	problems = append(problems, blueprintCapabilityProblems(source.Value, nil, source.Locations, registry)...)
	for i := range problems {
		if authored := source.authoredPath(problems[i].Path); authored != problems[i].Path {
			problems[i].Path = authored
			location := lookupBlueprintLocation(authored, source.Locations)
			problems[i].Line, problems[i].Column = location.Line, location.Column
		}
	}
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

// blueprintEnvironmentPropertyPrefix is the schema pointer prefix for a
// project environment's own (non-resource) properties.
const blueprintEnvironmentPropertyPrefix = "#/definitions/environment/allOf/1/properties/"

const (
	blueprintCapabilityRoot               blueprintCapabilityKind = "root"
	blueprintCapabilityResources          blueprintCapabilityKind = "resources"
	blueprintCapabilityProject            blueprintCapabilityKind = "project"
	blueprintCapabilityEnvironment        blueprintCapabilityKind = "environment"
	blueprintCapabilityServices           blueprintCapabilityKind = "services"
	blueprintCapabilityServer             blueprintCapabilityKind = "serverService"
	blueprintCapabilityCron               blueprintCapabilityKind = "cronService"
	blueprintCapabilityStatic             blueprintCapabilityKind = "staticService"
	blueprintCapabilityWorkflow           blueprintCapabilityKind = "workflowService"
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
	if problem, refused := blueprintWholeEntryProblem(path, locations, context); refused {
		return []BlueprintSourceProblem{problem}
	}
	var problems []BlueprintSourceProblem
	problems = append(problems, blueprintProjectEnvironmentsProblems(object, path, locations, context)...)
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
			problem("BLUEPRINT_CAPABILITY_UNSUPPORTED", blueprintUnsupportedCapabilityMessage(registry, field, capabilityPointer))
			continue
		}
		if enumPointer := blueprintFieldEnumCapabilityPointer(context, field); enumPointer != "" && blueprintEnumCapabilityUnsupported(registry, enumPointer, blueprintEncodedValue(child)) {
			problem("BLUEPRINT_CAPABILITY_UNSUPPORTED", blueprintUnsupportedEnumMessage(registry, field, enumPointer, blueprintEncodedValue(child)))
			continue
		}
		childContext := blueprintChildCapabilityContext(context, field, child)
		problems = append(problems, blueprintCapabilityProblemsAt(child, fieldPath, locations, registry, childContext)...)
	}
	return problems
}

// blueprintWholeEntryProblem refuses a services[] entry whose kind bex does
// not offer at all with ONE diagnostic on its type, instead of one per field:
// every workflowService field is classified unsupported, but five "X is not
// available" lines for a single declaration read as five independent edits.
func blueprintWholeEntryProblem(path []string, locations map[string]BlueprintSourceLocation, context blueprintCapabilityContext) (BlueprintSourceProblem, bool) {
	if context.kind != blueprintCapabilityWorkflow {
		return BlueprintSourceProblem{}, false
	}
	pointer := renderSchemaPointer(append(append([]string(nil), path...), "type"))
	location := lookupBlueprintLocation(pointer, locations)
	return BlueprintSourceProblem{
		Code:    "BLUEPRINT_CAPABILITY_UNSUPPORTED",
		Path:    pointer,
		Message: "Render Workflows (type: workflow) are not available on bex",
		Line:    location.Line,
		Column:  location.Column,
	}, true
}

// blueprintProjectEnvironmentsProblems keeps a project declaring at least one
// environment. Render's 2026-10 schema stopped requiring projects[].environments
// so a project can carry only buildSources, but the Blueprint spec still says a
// project "defines one or more environments", and bex materializes a project
// from its environments: an environment-less project would apply as a silent
// no-op, the false success ADR049 exists to prevent.
func blueprintProjectEnvironmentsProblems(object map[string]any, path []string, locations map[string]BlueprintSourceLocation, context blueprintCapabilityContext) []BlueprintSourceProblem {
	if context.kind != blueprintCapabilityProject {
		return nil
	}
	if _, declared := object["environments"]; declared {
		return nil
	}
	pointer := renderSchemaPointer(path)
	location := lookupBlueprintLocation(pointer, locations)
	return []BlueprintSourceProblem{{
		Code:    "BLUEPRINT_PROJECT_ENVIRONMENTS_REQUIRED",
		Path:    pointer,
		Message: "a project must define at least one environment",
		Line:    location.Line,
		Column:  location.Column,
	}}
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
	// declaredInCreate probes the direct create API's spelling of the field;
	// nil marks a Blueprint-only field the create API does not expose.
	declaredInCreate func(CreateRequest) bool
}

// prebuiltImageSourceFields is the shared policy for settings that only have
// meaning while a service builds from Git. The Blueprint compiler rejects enabled
// automation and other declared build fields; the direct create API checks the subset it exposes.
var prebuiltImageSourceFields = []prebuiltImageSourceField{
	{blueprintName: "repo", createName: "repo", declaredInCreate: func(req CreateRequest) bool { return req.Repo != "" }},
	{blueprintName: "branch", createName: "branch", declaredInCreate: func(req CreateRequest) bool { return req.Branch != "" }},
	{blueprintName: "rootDir", createName: "rootDirectory", declaredInCreate: func(req CreateRequest) bool { return req.RootDir != "" }},
	{blueprintName: "buildFilter", createName: "buildFilter", declaredInCreate: func(req CreateRequest) bool { return req.BuildFilter != nil }},
	{blueprintName: "buildCommand", createName: "buildCommand", declaredInCreate: func(req CreateRequest) bool { return req.BuildCommand != "" }},
	{blueprintName: "dockerfilePath", createName: "dockerfilePath", declaredInCreate: func(req CreateRequest) bool { return req.DockerfilePath != "" }},
	{blueprintName: "autoDeploy", createName: "autoDeploy", declaredInCreate: func(req CreateRequest) bool { return req.AutoDeploy != nil && *req.AutoDeploy }},
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
	// A static site has no prebuilt-image path at all (ADR029): the schema's
	// runtime enum is shared, so `runtime: static` + image validates as a
	// server service — the image field itself is the incompatibility here,
	// not the build-from-git settings beside it (repo is required for static).
	if context.kind == blueprintCapabilityStatic {
		fieldPath := append(append([]string(nil), path...), "image")
		pointer := renderSchemaPointer(fieldPath)
		location := lookupBlueprintLocation(pointer, locations)
		return []BlueprintSourceProblem{{
			Code:    "BLUEPRINT_CAPABILITY_INCOMPATIBLE",
			Path:    pointer,
			Message: "a static site builds from a Git repo; a prebuilt image is not supported",
			Line:    location.Line,
			Column:  location.Column,
		}}
	}
	problems := make([]BlueprintSourceProblem, 0, len(prebuiltImageSourceFields))
	for _, sourceField := range prebuiltImageSourceFields {
		if _, declared := object[sourceField.blueprintName]; !declared {
			continue
		}
		// Explicitly disabled automation is meaningful for image services too.
		if sourceField.blueprintName == "autoDeploy" && object["autoDeploy"] == false {
			continue
		}
		// Only the enabled, otherwise-supported trigger is incompatible.
		// checksPass and an invalid enum already have a more
		// specific registry/schema diagnosis at this same path; emitting both
		// would make one edit look like two independently actionable errors.
		if sourceField.blueprintName == "autoDeployTrigger" {
			trigger, _ := object[sourceField.blueprintName].(string)
			if trigger != "commit" {
				continue
			}
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
		case "buildSources", "previews", "previewsEnabled", "previewsExpireAfterDays", "projects", "ungrouped", "version":
			return "#/allOf/1/properties/" + field
		}
	case blueprintCapabilityResources:
		// The resources kind is the ungrouped block, whose own buildSources
		// property sits beside the shared resource collections.
		if field == "buildSources" {
			return "#/allOf/1/properties/ungrouped/properties/" + field
		}
		return "#/definitions/resources/properties/" + field
	case blueprintCapabilityEnvironment:
		switch field {
		case "services", "databases", "envVarGroups":
			return "#/definitions/resources/properties/" + field
		default:
			return blueprintEnvironmentPropertyPrefix + field
		}
	case blueprintCapabilityIPAllowList:
		return "#/definitions/ipAllowList/items/properties/" + field
	}
	if context.kind == "" {
		return ""
	}
	return "#/definitions/" + string(context.kind) + "/properties/" + field
}

// blueprintResourceChildContexts maps a resources-family collection field to
// its child capability context — the same triple under the root, the ungrouped
// resources block, and a project environment.
var blueprintResourceChildContexts = map[string]blueprintCapabilityContext{
	"services":     {kind: blueprintCapabilityServices},
	"databases":    {kind: blueprintCapabilityDatabase},
	"envVarGroups": {kind: blueprintCapabilityEnvGroup},
}

// blueprintEnvVarChildContexts is shared by the four env-var entry kinds:
// which reference form an entry uses is decided per item, so every kind
// resolves the same two reference objects.
var blueprintEnvVarChildContexts = map[string]blueprintCapabilityContext{
	"fromDatabase": {base: "#/definitions/envVarFromDatabase/properties/fromDatabase"},
	"fromService":  {base: "#/definitions/envVarFromService/properties/fromService"},
}

// blueprintServiceChildContexts builds the shared service child-context row.
// scaling and maintenanceMode are inline schema objects under the service
// kind's own definition, and the previews child kind differs per service kind.
func blueprintServiceChildContexts(kind, previews blueprintCapabilityKind) map[string]blueprintCapabilityContext {
	return map[string]blueprintCapabilityContext{
		"image":              {kind: blueprintCapabilityImage},
		"registryCredential": {kind: blueprintCapabilityRegistryCredential},
		"envVars":            {kind: blueprintCapabilityEnvKeyValue},
		"buildFilter":        {kind: blueprintCapabilityBuildFilter},
		"disk":               {kind: blueprintCapabilityDisk},
		"ipAllowList":        {kind: blueprintCapabilityIPAllowList},
		"previews":           {kind: previews},
		"scaling":            {base: "#/definitions/" + string(kind) + "/properties/scaling"},
		"maintenanceMode":    {base: "#/definitions/" + string(kind) + "/properties/maintenanceMode"},
		"headers":            {kind: blueprintCapabilityHeader},
		"routes":             {kind: blueprintCapabilityRoute},
	}
}

// blueprintChildContexts resolves (parent kind, field) to the child's
// capability context. A field absent from its kind's row — including every
// field under the previews kinds — yields the zero context.
var blueprintChildContexts = map[blueprintCapabilityKind]map[string]blueprintCapabilityContext{
	blueprintCapabilityRoot: mergedBlueprintMaps(blueprintResourceChildContexts, map[string]blueprintCapabilityContext{
		"ungrouped": {kind: blueprintCapabilityResources},
		"projects":  {kind: blueprintCapabilityProject},
		"previews":  {kind: blueprintCapabilityRootPreviews},
	}),
	blueprintCapabilityResources: blueprintResourceChildContexts,
	blueprintCapabilityProject: {
		"environments": {kind: blueprintCapabilityEnvironment},
	},
	blueprintCapabilityEnvironment: mergedBlueprintMaps(blueprintResourceChildContexts, map[string]blueprintCapabilityContext{
		"networking":  {base: blueprintEnvironmentPropertyPrefix + "networking"},
		"permissions": {base: blueprintEnvironmentPropertyPrefix + "permissions"},
	}),
	blueprintCapabilityServer: blueprintServiceChildContexts(blueprintCapabilityServer, blueprintCapabilityServicePreviews),
	blueprintCapabilityCron:   blueprintServiceChildContexts(blueprintCapabilityCron, blueprintCapabilityServicePreviews),
	blueprintCapabilityStatic: blueprintServiceChildContexts(blueprintCapabilityStatic, blueprintCapabilityStaticPreviews),
	blueprintCapabilityDatabase: {
		"ipAllowList":      {kind: blueprintCapabilityIPAllowList},
		"readReplicas":     {kind: blueprintCapabilityReadReplica},
		"highAvailability": {base: "#/definitions/database/properties/highAvailability"},
	},
	blueprintCapabilityKeyValue: {
		"ipAllowList": {kind: blueprintCapabilityIPAllowList},
	},
	blueprintCapabilityEnvGroup: {
		"envVars": {kind: blueprintCapabilityEnvKeyValue},
	},
	blueprintCapabilityEnvKeyValue:       blueprintEnvVarChildContexts,
	blueprintCapabilityEnvDatabase:       blueprintEnvVarChildContexts,
	blueprintCapabilityEnvService:        blueprintEnvVarChildContexts,
	blueprintCapabilityEnvGroupReference: blueprintEnvVarChildContexts,
}

func blueprintChildCapabilityContext(context blueprintCapabilityContext, field string, _ any) blueprintCapabilityContext {
	if context.base != "" {
		return blueprintCapabilityContext{base: context.base + "/properties/" + field}
	}
	return blueprintChildContexts[context.kind][field]
}

func blueprintServiceCapabilityContext(value any) blueprintCapabilityContext {
	service, _ := value.(map[string]any)
	serviceType, _ := service["type"].(string)
	if isKeyValueType(serviceType) {
		return blueprintCapabilityContext{kind: blueprintCapabilityKeyValue}
	}
	if serviceType == "cron" {
		return blueprintCapabilityContext{kind: blueprintCapabilityCron}
	}
	if serviceType == "workflow" {
		return blueprintCapabilityContext{kind: blueprintCapabilityWorkflow}
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

// blueprintCommonServiceEnumPointers covers the enum fields the server and
// cron service kinds share; static deliberately has no plan/runtime enum
// pointer. plan is per kind: Render split its one shared plan enum into
// serverPlan/cronPlan/keyValuePlan/postgresPlan (schema pin 2026-10-02).
var blueprintCommonServiceEnumPointers = map[string]string{
	"runtime":           "#/definitions/runtime/enum",
	"autoDeployTrigger": "#/definitions/autoDeployTrigger/enum",
}

var blueprintPreviewsEnumPointers = map[string]string{
	"generation": "#/definitions/previewsGeneration/enum",
}

// blueprintBaseEnumPointers resolves (inline-object base, field) to the enum
// pointer the registry classifies that field's values under.
var blueprintBaseEnumPointers = map[string]map[string]string{
	blueprintEnvironmentPropertyPrefix + "networking": {
		"isolation": blueprintEnvironmentPropertyPrefix + "networking/properties/isolation/enum",
	},
	blueprintEnvironmentPropertyPrefix + "permissions": {
		"protection": blueprintEnvironmentPropertyPrefix + "permissions/properties/protection/enum",
	},
	"#/definitions/envVarFromDatabase/properties/fromDatabase": {
		"property": "#/definitions/databaseEnvVarProperty/enum",
	},
	"#/definitions/envVarFromService/properties/fromService": {
		"property": "#/definitions/serviceEnvVarProperty/enum",
		"type":     "#/definitions/serviceType/enum",
	},
}

// blueprintKindEnumPointers is blueprintBaseEnumPointers' counterpart for
// named-definition contexts.
var blueprintKindEnumPointers = map[blueprintCapabilityKind]map[string]string{
	blueprintCapabilityServer: mergedBlueprintMaps(blueprintCommonServiceEnumPointers, map[string]string{
		"type":                  "#/definitions/serverService/properties/type/enum",
		"renderSubdomainPolicy": "#/definitions/serverService/properties/renderSubdomainPolicy/enum",
		"plan":                  "#/definitions/serverPlan/enum",
	}),
	blueprintCapabilityCron: mergedBlueprintMaps(blueprintCommonServiceEnumPointers, map[string]string{
		"plan": "#/definitions/cronPlan/enum",
	}),
	blueprintCapabilityStatic: {
		"renderSubdomainPolicy": "#/definitions/staticService/properties/renderSubdomainPolicy/enum",
		"autoDeployTrigger":     "#/definitions/autoDeployTrigger/enum",
	},
	blueprintCapabilityDatabase: {
		"plan":                 "#/definitions/postgresPlan/enum",
		"postgresMajorVersion": "#/definitions/database/properties/postgresMajorVersion/enum",
		"connectionPool":       "#/definitions/connectionPool/enum",
		"region":               "#/definitions/region/enum",
	},
	blueprintCapabilityKeyValue: {
		"type":            "#/definitions/redisServer/properties/type/enum",
		"maxmemoryPolicy": "#/definitions/redisServer/properties/maxmemoryPolicy/enum",
		"persistenceMode": "#/definitions/redisServer/properties/persistenceMode/enum",
		"plan":            "#/definitions/keyValuePlan/enum",
		"region":          "#/definitions/region/enum",
	},
	blueprintCapabilityRoute: {
		"type": "#/definitions/route/properties/type/enum",
	},
	blueprintCapabilityRootPreviews:    blueprintPreviewsEnumPointers,
	blueprintCapabilityServicePreviews: blueprintPreviewsEnumPointers,
	blueprintCapabilityStaticPreviews:  blueprintPreviewsEnumPointers,
}

func blueprintFieldEnumCapabilityPointer(context blueprintCapabilityContext, field string) string {
	if pointer := blueprintBaseEnumPointers[context.base][field]; pointer != "" {
		return pointer
	}
	return blueprintKindEnumPointers[context.kind][field]
}

func blueprintEncodedValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func blueprintUnsupportedCapabilityMessage(registry *BlueprintCapabilityRegistry, field, pointer string) string {
	switch pointer {
	case "#/definitions/cronService/properties/preDeployCommand":
		return "preDeployCommand: cron jobs do not run a pre-deploy phase on bex; put the command at the start of startCommand instead"
	case "#/definitions/staticService/properties/preDeployCommand":
		return "preDeployCommand: static sites do not run a pre-deploy phase; put the command in buildCommand instead"
	case "#/definitions/serverService/properties/disk":
		return "persistent service disks are not available on bex"
	case "#/allOf/1/properties/previews", "#/allOf/1/properties/previewsEnabled", "#/allOf/1/properties/previewsExpireAfterDays", "#/definitions/serverService/properties/previews", "#/definitions/serverService/properties/previewPlan", "#/definitions/serverService/properties/pullRequestPreviewsEnabled", "#/definitions/staticService/properties/previews", "#/definitions/staticService/properties/pullRequestPreviewsEnabled", "#/definitions/database/properties/previewPlan", "#/definitions/database/properties/previewDiskSizeGB", "#/definitions/redisServer/properties/previewPlan":
		return "preview environments are not available on bex"
	case "#/definitions/envVarFromKeyValue/properties/previewValue":
		return "previewValue is not available on bex because preview environments are not available"
	case "#/definitions/serverService/properties/region", "#/definitions/cronService/properties/region", "#/definitions/database/properties/region", "#/definitions/redisServer/properties/region":
		return "per-resource region placement is not available on bex"
	case "#/allOf/1/properties/buildSources", "#/allOf/1/properties/ungrouped/properties/buildSources", "#/definitions/project/properties/buildSources", "#/definitions/serverService/properties/buildSource":
		return field + ": Render Build Sources (shared build reuse) are not available on bex; declare the repo or image and build settings on each service instead"
	default:
		if reason := tenantFacingReason(registry.Fields[pointer].Reason); reason != "" {
			return fmt.Sprintf("%s is not available on bex: %s", field, reason)
		}
		return fmt.Sprintf("%s is not available on bex", field)
	}
}

func blueprintUnsupportedEnumMessage(registry *BlueprintCapabilityRegistry, field, pointer, encodedValue string) string {
	var value string
	if json.Unmarshal([]byte(encodedValue), &value) != nil {
		value = encodedValue
	}
	switch pointer {
	case "#/definitions/database/properties/postgresMajorVersion/enum":
		// The create API's wording and supported list, from one source (w8/054).
		return field + ": " + postgres.UnsupportedVersionMessage(value)
	case "#/definitions/autoDeployTrigger/enum":
		return "autoDeployTrigger: checksPass requires CI-check gating, which is not available on bex"
	case "#/definitions/serverPlan/enum", "#/definitions/cronPlan/enum", "#/definitions/keyValuePlan/enum", "#/definitions/postgresPlan/enum":
		return "plan names a Render compute plan size that bex does not offer"
	case "#/definitions/serviceType/enum":
		return "fromService type workflow refers to Render Workflows, which are not available on bex"
	case "#/definitions/serviceEnvVarProperty/enum":
		return "fromService property slug is defined only for Render Workflows, which are not available on bex"
	}
	if reason := tenantFacingReason(registry.EnumValues[pointer][encodedValue].Reason); reason != "" {
		return fmt.Sprintf("%s %s is not available on bex: %s", field, encodedValue, reason)
	}
	return fmt.Sprintf("%s uses an unsupported Render Blueprint value", field)
}

// tenantFacingReason is a registry reason cut before its first parenthetical
// or em dash — where reviewed entries cite ADRs, milestones and DO_NOT_DO
// lines a tenant cannot act on (w8/054). The guard test pins that no
// unsupported entry's message leaks such a reference.
func tenantFacingReason(reason string) string {
	// Whole families already have a tenant sentence; reuse it rather than the
	// maintainer's rationale for the classification.
	for _, family := range []struct{ marker, sentence string }{
		{"PR preview environments", "preview environments are not available on bex"},
		{"Render Workflows", "Render Workflows are not available on bex"},
		{"buildSource", "Render Build Sources (shared build reuse) are not available on bex"},
		{"single-region", "per-resource region placement is not available on bex"},
	} {
		if strings.Contains(reason, family.marker) {
			return family.sentence
		}
	}
	for _, cut := range []string{" (", " — "} {
		if i := strings.Index(reason, cut); i >= 0 {
			reason = reason[:i]
		}
	}
	return strings.TrimRight(strings.TrimSpace(reason), ".;:,")
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
	// The byte cap must run BEFORE yaml.Decode: the walk budgets below bound
	// what the walk materializes, but the decoder has already built the full
	// node graph by then — compact flow YAML amplifies ~100× into yaml.Node
	// structs, so an uncapped 2 MiB body (BEX_MAX_BODY_BYTES) allocates
	// ~200 MB before the first budget check (codex r7 #9). 512 KiB is far
	// above any real Blueprint while keeping the pre-walk allocation bounded.
	if len(manifest) > blueprintMaxManifestBytes {
		return &BlueprintSource{Locations: map[string]BlueprintSourceLocation{}}, []BlueprintSourceProblem{{
			Code: "BLUEPRINT_YAML_TOO_LARGE", Path: "#",
			Message: ErrBlueprintTooLarge.Error(),
		}}
	}
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
	value, problems := yamlNodeToBlueprintValue(document.Content[0], nil, source.Locations, 0, &blueprintWalkBudget{})
	source.Value = value
	return source, problems
}

// Structural budgets for Blueprint YAML materialization. The manifest byte
// cap (blueprintMaxManifestBytes, checked before yaml.Decode) bounds input
// bytes, but bytes do not bound the amplified in-memory shape: aliases are
// already rejected, yet a small document can still spread into millions of
// tiny nodes — each materializing a Go value plus one locations-map entry —
// or nest pathologically deep. Every budget is checked during the walk,
// before the allocation it guards, and the first breach refuses the whole
// document. The limits are far above any real Blueprint (hundreds of nodes)
// and far below yaml.v3's own 10k nesting ceiling.
const (
	// blueprintMaxManifestBytes is the pre-decode byte cap (see
	// parseBlueprintSource); the budgets below govern the post-decode walk.
	blueprintMaxManifestBytes = 512 << 10 // 512 KiB

	blueprintMaxDepth             = 100
	blueprintMaxNodes             = 100_000
	blueprintMaxCollectionEntries = 10_000
	// blueprintMaxResources bounds the aggregate declarations that the
	// Blueprint compiler can normalize and plan. It is deliberately above the
	// paid plan's normal resource quotas, but prevents a byte-small manifest
	// from fanning out thousands of downstream actions.
	blueprintMaxResources = 500
	// Kept below the manifest byte cap so a single oversized scalar is
	// still reported as a scalar breach, not a document-size one.
	blueprintMaxScalarBytes = 256 << 10 // 256 KiB
	// The locations map holds one pointer-string entry per value node — the
	// most expensive per-node allocation — so it is capped tighter than raw
	// nodes (whose count also includes mapping keys): a keyless document
	// trips this budget first, a key-heavy one trips blueprintMaxNodes first.
	blueprintMaxLocations = 75_000
)

// ErrBlueprintTooLarge keeps transport refusals and compiler diagnostics on
// the same manifest byte limit, including bodies refused before YAML decoding.
var ErrBlueprintTooLarge = fmt.Errorf("Blueprint manifests are limited to %d KiB", blueprintMaxManifestBytes>>10)

// blueprintWalkBudget enforces the structural budgets above while a Blueprint
// YAML document is materialized: it counts nodes as the walk visits them and,
// on the first breach, bails the whole walk out fail-fast so an over-budget
// document stops allocating immediately instead of after full amplification.
type blueprintWalkBudget struct {
	nodes     int
	resources int
	bailed    bool
}

// fail marks the walk bailed — every later yamlNodeToBlueprintValue call
// returns at once without allocating — and returns the breach diagnostic.
func (b *blueprintWalkBudget) fail(code, pointer, message string, node *yaml.Node) []BlueprintSourceProblem {
	b.bailed = true
	return []BlueprintSourceProblem{{Code: code, Path: pointer, Message: message, Line: node.Line, Column: node.Column}}
}

// countNode charges one node against the walk budget, returning nil while the
// document stays inside it and the breach diagnostic once it does not. Both the
// value walk and the mapping-key loop charge through here so the two cannot
// drift on the limit or its message.
func (b *blueprintWalkBudget) countNode(pointer string, node *yaml.Node) []BlueprintSourceProblem {
	b.nodes++
	if b.nodes <= blueprintMaxNodes {
		return nil
	}
	return b.fail("BLUEPRINT_YAML_NODES", pointer, fmt.Sprintf("Blueprint contains more than %d YAML nodes", blueprintMaxNodes), node)
}

// countCollection checks one sequence's or mapping's entry count against the
// per-collection budget.
func (b *blueprintWalkBudget) countCollection(pointer string, node *yaml.Node, entries int) []BlueprintSourceProblem {
	if entries <= blueprintMaxCollectionEntries {
		return nil
	}
	return b.fail("BLUEPRINT_YAML_COLLECTION", pointer, fmt.Sprintf("Blueprint collections are limited to %d entries", blueprintMaxCollectionEntries), node)
}

// countResources checks the aggregate declaration count before the sequence is
// materialized into []any. Resource collections are identified by their schema
// field name; all valid root, ungrouped, and project-environment placements use
// one of these three names.
func (b *blueprintWalkBudget) countResources(path []string, pointer string, node *yaml.Node, entries int) []BlueprintSourceProblem {
	if len(path) == 0 {
		return nil
	}
	switch path[len(path)-1] {
	case "services", "databases", "envVarGroups":
		b.resources += entries
		if b.resources > blueprintMaxResources {
			return b.fail("BLUEPRINT_RESOURCE_COUNT", pointer, fmt.Sprintf("Blueprints are limited to %d resource declarations", blueprintMaxResources), node)
		}
	}
	return nil
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

func yamlNodeToBlueprintValue(node *yaml.Node, path []string, locations map[string]BlueprintSourceLocation, depth int, budget *blueprintWalkBudget) (any, []BlueprintSourceProblem) {
	pointer := renderSchemaPointer(path)
	if budget.bailed {
		return nil, nil
	}
	if depth > blueprintMaxDepth {
		return nil, budget.fail("BLUEPRINT_YAML_DEPTH", pointer, fmt.Sprintf("Blueprint nests deeper than the supported %d levels", blueprintMaxDepth), node)
	}
	if breach := budget.countNode(pointer, node); breach != nil {
		return nil, breach
	}
	if len(locations) >= blueprintMaxLocations {
		return nil, budget.fail("BLUEPRINT_YAML_LOCATIONS", pointer, fmt.Sprintf("Blueprint maps more than %d source locations", blueprintMaxLocations), node)
	}
	locations[pointer] = BlueprintSourceLocation{Line: node.Line, Column: node.Column}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return nil, []BlueprintSourceProblem{{Code: "BLUEPRINT_YAML_EMPTY", Path: pointer, Message: "Blueprint must contain one YAML document", Line: node.Line, Column: node.Column}}
		}
		return yamlNodeToBlueprintValue(node.Content[0], path, locations, depth, budget)
	case yaml.AliasNode:
		return nil, []BlueprintSourceProblem{{Code: "BLUEPRINT_YAML_ALIAS", Path: pointer, Message: "YAML aliases are not supported in Blueprints", Line: node.Line, Column: node.Column}}
	case yaml.SequenceNode:
		if breach := budget.countCollection(pointer, node, len(node.Content)); breach != nil {
			return nil, breach
		}
		if breach := budget.countResources(path, pointer, node, len(node.Content)); breach != nil {
			return nil, breach
		}
		values := make([]any, len(node.Content))
		var problems []BlueprintSourceProblem
		for i, child := range node.Content {
			if budget.bailed {
				break
			}
			value, childProblems := yamlNodeToBlueprintValue(child, append(path, strconv.Itoa(i)), locations, depth+1, budget)
			values[i] = value
			problems = append(problems, childProblems...)
		}
		return values, problems
	case yaml.MappingNode:
		entries := len(node.Content) / 2
		if breach := budget.countCollection(pointer, node, entries); breach != nil {
			return nil, breach
		}
		values := make(map[string]any, entries)
		seen := make(map[string]BlueprintSourceLocation, entries)
		var problems []BlueprintSourceProblem
		for i := 0; i < len(node.Content); i += 2 {
			if budget.bailed {
				break
			}
			key, value := node.Content[i], node.Content[i+1]
			if breach := budget.countNode(pointer, key); breach != nil {
				problems = append(problems, breach...)
				break
			}
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				problems = append(problems, BlueprintSourceProblem{Code: "BLUEPRINT_YAML_MAPPING_KEY", Path: pointer, Message: "Blueprint mapping keys must be strings", Line: key.Line, Column: key.Column})
				continue
			}
			if len(key.Value) > blueprintMaxScalarBytes {
				problems = append(problems, budget.fail("BLUEPRINT_YAML_SCALAR_SIZE", pointer, fmt.Sprintf("Blueprint mapping keys are limited to %d bytes", blueprintMaxScalarBytes), key)...)
				break
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
			child, childProblems := yamlNodeToBlueprintValue(value, childPath, locations, depth+1, budget)
			values[key.Value] = child
			problems = append(problems, childProblems...)
		}
		return values, problems
	case yaml.ScalarNode:
		if len(node.Value) > blueprintMaxScalarBytes {
			return nil, budget.fail("BLUEPRINT_YAML_SCALAR_SIZE", pointer, fmt.Sprintf("Blueprint scalar values are limited to %d bytes", blueprintMaxScalarBytes), node)
		}
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
	if err := admitBexFreeCronPlan(document); err != nil {
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
	document["properties"] = mergedBlueprintMaps(resourceProperties, rootProperties)
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
	environment["properties"] = mergedBlueprintMaps(resourceProperties, environmentProperties)
	environment["additionalProperties"] = false
	delete(environment, "allOf")
	delete(environment, "unevaluatedProperties")

	rootProperties, _ = document["properties"].(map[string]any)
	ungrouped, ok := rootProperties["ungrouped"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no ungrouped definition")
	}
	ungroupedProperties, _ := ungrouped["properties"].(map[string]any)
	ungrouped["properties"] = mergedBlueprintMaps(resourceProperties, ungroupedProperties)
	ungrouped["additionalProperties"] = false
	delete(ungrouped, "allOf")
	delete(ungrouped, "unevaluatedProperties")
	return nil
}

// admitBexFreeCronPlan is a documented bex divergence (ADR049 "Schema re-pin
// 2026-10-02"): Render's cronPlan has no free rung, but bex keeps a free cron
// tier (also the omitted-plan default), and tenant Blueprints already declare
// `plan: free` on cron jobs. Rejecting it would break their next sync for no
// gain, so the compiled schema — never the pinned upstream bytes — admits it.
// The capability registry stays exhaustive over the pinned bytes only; this
// value resolves through normalizeTierForType like any other plan.
func admitBexFreeCronPlan(document map[string]any) error {
	definitions, _ := document["definitions"].(map[string]any)
	cronPlan, ok := definitions["cronPlan"].(map[string]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema has no cronPlan definition")
	}
	values, ok := cronPlan["enum"].([]any)
	if !ok {
		return fmt.Errorf("embedded Render Blueprint schema cronPlan has no enum")
	}
	for _, value := range values {
		if value == "free" {
			return nil // upstream re-admitted it; nothing to overlay
		}
	}
	cronPlan["enum"] = append([]any{"free"}, values...)
	return nil
}

func mergedBlueprintMaps[V any](groups ...map[string]V) map[string]V {
	merged := map[string]V{}
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
//
// Conditional subschemas (if/then/else) are left exactly as published: they
// constrain a few named fields of an object whose own definition is already
// closed, so closing them too would reject every OTHER field of that object
// whenever the condition held (serverService's buildSource branch lists only
// the fields it forbids).
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
	for key, child := range object {
		if key == "if" || key == "then" || key == "else" {
			continue
		}
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

func blueprintSchemaProblems(err error, locations map[string]BlueprintSourceLocation, instance any) []BlueprintSourceProblem {
	var validation *jsonschema.ValidationError
	if !errors.As(err, &validation) {
		return []BlueprintSourceProblem{{Code: "BLUEPRINT_SCHEMA_INVALID", Path: "#", Message: "Blueprint does not satisfy the reviewed Render schema"}}
	}
	var problems []BlueprintSourceProblem
	appendBlueprintSchemaProblems(validation, locations, instance, &problems)
	return problems
}

func appendBlueprintSchemaProblems(validation *jsonschema.ValidationError, locations map[string]BlueprintSourceLocation, instance any, problems *[]BlueprintSourceProblem) {
	if len(validation.Causes) > 0 {
		// A failed anyOf reports every alternative. Report the branch the
		// instance actually declared (type/runtime const, or which env-var
		// key is present). Leaf count is last resort: it prefers the
		// shallowest failure and otherwise names an unrelated kind.
		if _, isAnyOf := validation.ErrorKind.(*kind.AnyOf); isAnyOf && len(validation.Causes) > 1 {
			appendBlueprintSchemaProblems(selectBlueprintAnyOfCause(validation.Causes, blueprintInstanceAt(instance, validation.InstanceLocation)), locations, instance, problems)
			return
		}
		for _, cause := range validation.Causes {
			appendBlueprintSchemaProblems(cause, locations, instance, problems)
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
		message = blueprintSchemaLeafMessage(validation)
	}
	*problems = append(*problems, BlueprintSourceProblem{Code: "BLUEPRINT_SCHEMA_INVALID", Path: path, Message: message, Line: location.Line, Column: location.Column})
}

// blueprintServiceTypeValues is the union of services[] discriminators.
const blueprintServiceTypeValues = "'web', 'worker', 'pserv', 'cron', 'keyvalue', 'redis'"

func blueprintSchemaLeafMessage(validation *jsonschema.ValidationError) string {
	message := validation.Error()
	if loc := validation.InstanceLocation; blueprintServiceTypeLocation(loc) {
		if got, ok := blueprintSchemaGotValue(validation.ErrorKind); ok && !blueprintKnownServiceType(got) {
			if i := strings.LastIndex(message, ": "); i >= 0 {
				return message[:i+2] + "value must be one of " + blueprintServiceTypeValues
			}
			return "value must be one of " + blueprintServiceTypeValues
		}
	}
	return message
}

func blueprintServiceTypeLocation(location []string) bool {
	if len(location) < 3 || location[len(location)-1] != "type" {
		return false
	}
	for i, token := range location[:len(location)-1] {
		if token == "services" && i+1 < len(location)-1 {
			if _, err := strconv.Atoi(location[i+1]); err == nil {
				return true
			}
		}
	}
	return false
}

func blueprintSchemaGotValue(errorKind jsonschema.ErrorKind) (string, bool) {
	switch k := errorKind.(type) {
	case *kind.Enum:
		s, ok := k.Got.(string)
		return s, ok
	case *kind.Const:
		s, ok := k.Got.(string)
		return s, ok
	default:
		return "", false
	}
}

func blueprintKnownServiceType(value string) bool {
	switch value {
	case "web", "worker", "pserv", "cron", "keyvalue", "redis", "workflow":
		return true
	default:
		return false
	}
}

func selectBlueprintAnyOfCause(causes []*jsonschema.ValidationError, instance any) *jsonschema.ValidationError {
	object, _ := instance.(map[string]any)
	bestScore := -1
	var matched []*jsonschema.ValidationError
	for _, cause := range causes {
		score := blueprintAnyOfDiscriminatorScore(cause, object)
		if score < 0 {
			continue
		}
		if score > bestScore {
			bestScore = score
			matched = []*jsonschema.ValidationError{cause}
			continue
		}
		if score == bestScore {
			matched = append(matched, cause)
		}
	}
	pool := causes
	if len(matched) > 0 {
		pool = matched
	}
	closest := pool[0]
	for _, cause := range pool[1:] {
		if blueprintSchemaLeafCount(cause) < blueprintSchemaLeafCount(closest) {
			closest = cause
		}
	}
	return closest
}

func blueprintAnyOfDiscriminatorScore(cause *jsonschema.ValidationError, object map[string]any) int {
	if object == nil {
		return 0
	}
	switch blueprintSchemaBranchName(cause) {
	case "redisServer":
		return blueprintEnumMatch(blueprintObjectString(object, "type"), "keyvalue", "redis")
	case "cronService":
		return blueprintConstMatch(blueprintObjectString(object, "type"), "cron")
	case "workflowService":
		return blueprintConstMatch(blueprintObjectString(object, "type"), "workflow")
	case "staticService":
		typeScore := blueprintConstMatch(blueprintObjectString(object, "type"), "web")
		runtimeScore := blueprintConstMatch(blueprintObjectString(object, "runtime"), "static")
		if typeScore < 0 || runtimeScore < 0 {
			return -1
		}
		return typeScore + runtimeScore
	case "serverService":
		if blueprintObjectString(object, "type") == "web" && blueprintObjectString(object, "runtime") == "static" {
			return -1
		}
		return blueprintEnumMatch(blueprintObjectString(object, "type"), "web", "worker", "pserv")
	case "envVarFromDatabase":
		return blueprintPresenceMatch(object, "fromDatabase")
	case "envVarFromService":
		return blueprintPresenceMatch(object, "fromService")
	case "envVarFromGroup":
		return blueprintPresenceMatch(object, "fromGroup")
	case "envVarFromKeyValue":
		if _, fromRef := object["fromDatabase"]; fromRef {
			return -1
		}
		if _, fromRef := object["fromService"]; fromRef {
			return -1
		}
		if _, fromRef := object["fromGroup"]; fromRef {
			return -1
		}
		if _, ok := object["value"]; ok {
			return 1
		}
		if _, ok := object["generateValue"]; ok {
			return 1
		}
		if _, ok := object["key"]; ok {
			return 0
		}
		return -1
	default:
		return 0
	}
}

func blueprintSchemaBranchName(cause *jsonschema.ValidationError) string {
	url := cause.SchemaURL
	if ref, ok := cause.ErrorKind.(*kind.Reference); ok && ref.URL != "" {
		url = ref.URL
	}
	const marker = "#/definitions/"
	if i := strings.Index(url, marker); i >= 0 {
		name := url[i+len(marker):]
		if j := strings.IndexAny(name, "/?#"); j >= 0 {
			name = name[:j]
		}
		return name
	}
	return ""
}

func blueprintObjectString(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return value
}

func blueprintConstMatch(got, want string) int {
	if got == "" {
		return 0
	}
	if got == want {
		return 2
	}
	return -1
}

func blueprintEnumMatch(got string, want ...string) int {
	if got == "" {
		return 0
	}
	for _, allowed := range want {
		if got == allowed {
			return 1
		}
	}
	return -1
}

func blueprintPresenceMatch(object map[string]any, key string) int {
	if _, ok := object[key]; ok {
		return 2
	}
	return -1
}

func blueprintInstanceAt(root any, location []string) any {
	current := root
	for _, token := range location {
		switch node := current.(type) {
		case map[string]any:
			current = node[token]
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node) {
				return nil
			}
			current = node[index]
		default:
			return nil
		}
	}
	return current
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
