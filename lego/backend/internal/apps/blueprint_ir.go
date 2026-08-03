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
	"context"
	"fmt"
	"reflect"
	"sort"
)

// BlueprintResourceKind is the resource family a normalized declaration
// creates or synchronizes. Key Value remains a Render service-list entry in
// source, but gets its own target kind before planning.
type BlueprintResourceKind string

const (
	BlueprintResourceService     BlueprintResourceKind = "service"
	BlueprintResourcePostgres    BlueprintResourceKind = "postgres"
	BlueprintResourceKeyValue    BlueprintResourceKind = "key_value"
	BlueprintResourceEnvVarGroup BlueprintResourceKind = "env_var_group"
)

// BlueprintField preserves the distinction the old typed decoder loses:
// declared with null, declared with a zero/empty value, and absent (not in the
// resource's Fields map at all). Value is compiler-internal and must not be
// serialized into user-visible plans because it can contain an env secret.
type BlueprintField struct {
	Null     bool
	Value    any
	Location BlueprintSourceLocation
}

// BlueprintResourceIR is one normalized declaration. SourcePath always points
// at the original YAML object; Project/Environment are declaration placement,
// not inferred current state.
type BlueprintResourceIR struct {
	Kind        BlueprintResourceKind
	Name        string
	SourcePath  string
	Project     string
	Environment string
	Ungrouped   bool
	Fields      map[string]BlueprintField
}

// BlueprintIR is an order-independent, source-preserving intermediate form.
// Parsing and validation happen once; subsequent planners and adapters consume
// this instead of re-unmarshalling the manifest.
type BlueprintIR struct {
	Resources []BlueprintResourceIR
}

// BlueprintExtensionBuilder reads the reviewed local extension after schema
// validation. It intentionally has no legacy builder fallback: accepting the
// unnamespaced spelling would make a filename-based dialect ambiguous.
func BlueprintExtensionBuilder(resource BlueprintResourceIR) (string, bool) {
	extension, ok := resource.Fields["x-bex"]
	if !ok || extension.Null {
		return "", false
	}
	object, ok := extension.Value.(map[string]any)
	if !ok {
		return "", false
	}
	builder, ok := object["builder"].(string)
	return builder, ok
}

// BlueprintPlanOperation is intentionally small at this stage. Field-specific
// policies add conflict/unsupported actions without forcing adapters to invent
// their own action vocabulary.
type BlueprintPlanOperation string

const (
	BlueprintPlanCreate BlueprintPlanOperation = "create"
	BlueprintPlanUpdate BlueprintPlanOperation = "update"
	BlueprintPlanNoop   BlueprintPlanOperation = "noop"
	BlueprintPlanError  BlueprintPlanOperation = "error"
)

// BlueprintFieldChange carries only the changed field path. It deliberately
// omits values: plan output must remain safe for `value`, generated env vars,
// and secret references.
type BlueprintFieldChange struct {
	Path string
}

// BlueprintPlanAction is a deterministic authorized-current-state decision.
type BlueprintPlanAction struct {
	Operation     BlueprintPlanOperation
	Kind          BlueprintResourceKind
	Name          string
	SourcePath    string
	ResourceID    string
	ChangedFields []BlueprintFieldChange
	Message       string
}

// BlueprintPlan is the shared planning result. It is intentionally distinct
// from the older declaration-count validation summary.
type BlueprintPlan struct {
	Actions []BlueprintPlanAction
}

// BlueprintCurrentResource is the non-secret readable snapshot required for
// planning. A resolver must scope lookup to the authorized workspace; the IR
// never contains an owner id guessed from a resource name.
type BlueprintCurrentResource struct {
	ID     string
	Fields map[string]any
}

// BlueprintStateResolver is the only current-state seam the generic planner
// needs. The production implementation arrives with the entrypoint migration;
// tests and feature-specific planners can supply a minimal authorized view.
type BlueprintStateResolver interface {
	ResolveBlueprintResource(context.Context, BlueprintResourceKind, string) (BlueprintCurrentResource, bool, error)
}

// NormalizeBlueprintIR turns a compiler-validated source tree into all root,
// environment-scoped, and ungrouped resource declarations. It does not apply
// field defaults or resolve references; those require the capability handlers
// and current state added by subsequent tasks.
func NormalizeBlueprintIR(source *BlueprintSource) (BlueprintIR, []BlueprintSourceProblem) {
	if source == nil {
		return BlueprintIR{}, []BlueprintSourceProblem{{Code: "BLUEPRINT_IR_SOURCE", Path: "#", Message: "Blueprint source is required"}}
	}
	root, ok := source.Value.(map[string]any)
	if !ok {
		return BlueprintIR{}, []BlueprintSourceProblem{{Code: "BLUEPRINT_IR_ROOT", Path: "#", Message: "Blueprint root must be an object"}}
	}
	ir := BlueprintIR{}
	var problems []BlueprintSourceProblem
	normalizeBlueprintResourceLists(&ir, &problems, source, root, nil, "", "", false)

	if ungrouped, ok := root["ungrouped"].(map[string]any); ok {
		normalizeBlueprintResourceLists(&ir, &problems, source, ungrouped, []string{"ungrouped"}, "", "", true)
	}
	if projects, ok := root["projects"].([]any); ok {
		for projectIndex, rawProject := range projects {
			project, ok := rawProject.(map[string]any)
			if !ok {
				continue
			}
			projectName, _ := project["name"].(string)
			environments, _ := project["environments"].([]any)
			for environmentIndex, rawEnvironment := range environments {
				environment, ok := rawEnvironment.(map[string]any)
				if !ok {
					continue
				}
				environmentName, _ := environment["name"].(string)
				path := []string{"projects", fmt.Sprintf("%d", projectIndex), "environments", fmt.Sprintf("%d", environmentIndex)}
				normalizeBlueprintResourceLists(&ir, &problems, source, environment, path, projectName, environmentName, false)
			}
		}
	}

	seen := map[BlueprintResourceKind]map[string]BlueprintResourceIR{}
	for _, resource := range ir.Resources {
		byName := seen[resource.Kind]
		if byName == nil {
			byName = map[string]BlueprintResourceIR{}
			seen[resource.Kind] = byName
		}
		if first, duplicate := byName[resource.Name]; duplicate {
			location := source.Locations[resource.SourcePath]
			problems = append(problems, BlueprintSourceProblem{
				Code: "BLUEPRINT_DUPLICATE_RESOURCE", Path: resource.SourcePath,
				Message: fmt.Sprintf("duplicate name %q for %s (first declared at %s)", resource.Name, resource.Kind, first.SourcePath),
				Line:    location.Line, Column: location.Column,
			})
			continue
		}
		byName[resource.Name] = resource
	}
	sort.SliceStable(ir.Resources, func(i, j int) bool {
		return blueprintResourceOrder(ir.Resources[i], ir.Resources[j])
	})
	return ir, sortBlueprintSourceProblems(problems)
}

func normalizeBlueprintResourceLists(ir *BlueprintIR, problems *[]BlueprintSourceProblem, source *BlueprintSource, container map[string]any, basePath []string, project, environment string, ungrouped bool) {
	for _, declaration := range []struct {
		Field string
		Kind  BlueprintResourceKind
	}{
		{Field: "services", Kind: BlueprintResourceService},
		{Field: "databases", Kind: BlueprintResourcePostgres},
		{Field: "envVarGroups", Kind: BlueprintResourceEnvVarGroup},
	} {
		entries, _ := container[declaration.Field].([]any)
		for index, rawEntry := range entries {
			entry, ok := rawEntry.(map[string]any)
			if !ok {
				continue
			}
			path := append(append([]string(nil), basePath...), declaration.Field, fmt.Sprintf("%d", index))
			resource := blueprintResourceFromMap(source, declaration.Kind, entry, path, project, environment, ungrouped)
			if declaration.Field == "services" {
				if kind, _ := entry["type"].(string); kind == "keyvalue" || kind == "redis" {
					resource.Kind = BlueprintResourceKeyValue
				}
			}
			if resource.Name == "" {
				location := source.Locations[resource.SourcePath]
				*problems = append(*problems, BlueprintSourceProblem{Code: "BLUEPRINT_IR_NAME", Path: resource.SourcePath + "/name", Message: "Blueprint resource is missing its name", Line: location.Line, Column: location.Column})
				continue
			}
			ir.Resources = append(ir.Resources, resource)
		}
	}
}

func blueprintResourceFromMap(source *BlueprintSource, kind BlueprintResourceKind, entry map[string]any, path []string, project, environment string, ungrouped bool) BlueprintResourceIR {
	resourcePath := renderSchemaPointer(path)
	fields := make(map[string]BlueprintField, len(entry))
	for name, value := range entry {
		fieldPath := resourcePath + "/" + renderSchemaPointer([]string{name})[2:]
		fields[name] = BlueprintField{Null: value == nil, Value: value, Location: source.Locations[fieldPath]}
	}
	name, _ := entry["name"].(string)
	return BlueprintResourceIR{Kind: kind, Name: name, SourcePath: resourcePath, Project: project, Environment: environment, Ungrouped: ungrouped, Fields: fields}
}

// PlanBlueprintIR resolves each normalized declaration exactly once under the
// caller-provided context. It records only field names in an update action,
// retaining the no-secret plan invariant while field-specific handlers decide
// actual patch semantics later.
func PlanBlueprintIR(ctx context.Context, ir BlueprintIR, resolver BlueprintStateResolver) (BlueprintPlan, error) {
	if resolver == nil {
		return BlueprintPlan{}, fmt.Errorf("Blueprint current-state resolver is required")
	}
	resources := append([]BlueprintResourceIR(nil), ir.Resources...)
	sort.SliceStable(resources, func(i, j int) bool { return blueprintResourceOrder(resources[i], resources[j]) })
	plan := BlueprintPlan{Actions: make([]BlueprintPlanAction, 0, len(resources))}
	for _, resource := range resources {
		current, exists, err := resolver.ResolveBlueprintResource(ctx, resource.Kind, resource.Name)
		if err != nil {
			return BlueprintPlan{}, fmt.Errorf("resolve %s %q: %w", resource.Kind, resource.Name, err)
		}
		action := BlueprintPlanAction{Kind: resource.Kind, Name: resource.Name, SourcePath: resource.SourcePath}
		if !exists {
			action.Operation = BlueprintPlanCreate
			plan.Actions = append(plan.Actions, action)
			continue
		}
		action.ResourceID = current.ID
		action.ChangedFields = changedBlueprintFields(resource.Fields, current.Fields)
		if len(action.ChangedFields) == 0 {
			action.Operation = BlueprintPlanNoop
		} else {
			action.Operation = BlueprintPlanUpdate
		}
		plan.Actions = append(plan.Actions, action)
	}
	return plan, nil
}

func changedBlueprintFields(desired map[string]BlueprintField, current map[string]any) []BlueprintFieldChange {
	var changes []BlueprintFieldChange
	for name, field := range desired {
		want := field.Value
		if field.Null {
			want = nil
		}
		if got, ok := current[name]; !ok || !reflect.DeepEqual(want, got) {
			changes = append(changes, BlueprintFieldChange{Path: name})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

func blueprintResourceOrder(left, right BlueprintResourceIR) bool {
	leftRank, rightRank := blueprintResourceRank(left.Kind), blueprintResourceRank(right.Kind)
	if leftRank != rightRank {
		return leftRank < rightRank
	}
	if left.Name != right.Name {
		return left.Name < right.Name
	}
	return left.SourcePath < right.SourcePath
}

func blueprintResourceRank(kind BlueprintResourceKind) int {
	switch kind {
	case BlueprintResourceEnvVarGroup:
		return 0
	case BlueprintResourcePostgres:
		return 1
	case BlueprintResourceKeyValue:
		return 2
	case BlueprintResourceService:
		return 3
	default:
		return 4
	}
}
