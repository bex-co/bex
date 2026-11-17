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
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// reviewedBlueprintAlias is one documented historical spelling the compiler
// renames to its canonical field before schema validation.
type reviewedBlueprintAlias struct {
	kind          blueprintCapabilityKind
	field, target string
	documentation string
	// exclusive refuses a service that declares both spellings, whatever
	// their values. Otherwise both may be declared when they agree.
	exclusive bool
	// notice is the plan warning a use of the alias earns, "" for none.
	notice string
}

// pointer is the alias's own JSON Pointer in the capability registry.
func (a reviewedBlueprintAlias) pointer() string {
	return "#/definitions/" + string(a.kind) + "/properties/" + a.field
}

func (a reviewedBlueprintAlias) targetPointer() string {
	return "#/definitions/" + string(a.kind) + "/properties/" + a.target
}

// reviewedBlueprintAliases is every alias the registry may carry: Render's
// documented env -> runtime spelling on each service definition, and
// afterFirstDeployCommand -> initialDeployHook where the hook itself is
// accepted (web and private services and background workers; w33/m435).
var reviewedBlueprintAliases = func() []reviewedBlueprintAlias {
	aliases := make([]reviewedBlueprintAlias, 0, 4)
	for _, kind := range []blueprintCapabilityKind{blueprintCapabilityServer, blueprintCapabilityCron, blueprintCapabilityStatic} {
		aliases = append(aliases, reviewedBlueprintAlias{
			kind: kind, field: "env", target: "runtime",
			documentation: "https://render.com/docs/blueprint-spec#runtime",
		})
	}
	return append(aliases, reviewedBlueprintAlias{
		kind: blueprintCapabilityServer, field: blueprintHookAlias, target: "initialDeployHook",
		documentation: "https://render.com/docs/blueprint-spec#initialdeployhook",
		exclusive:     true,
		notice:        blueprintHookAlias + " is deprecated; use initialDeployHook",
	})
}()

// blueprintHookAlias is Render's deprecated alias for initialDeployHook.
const blueprintHookAlias = "afterFirstDeployCommand"

// reviewedBlueprintAliasAt is the reviewed alias registered at pointer.
func reviewedBlueprintAliasAt(pointer string) (reviewedBlueprintAlias, bool) {
	for _, alias := range reviewedBlueprintAliases {
		if alias.pointer() == pointer {
			return alias, true
		}
	}
	return reviewedBlueprintAlias{}, false
}

// Aliases are reviewed separately from the immutable upstream schema: the
// registry must carry exactly reviewedBlueprintAliases, each naming its
// canonical field and Render's documentation of it.
func validateBlueprintAliases(registry *BlueprintCapabilityRegistry) error {
	for _, reviewed := range reviewedBlueprintAliases {
		pointer := reviewed.pointer()
		alias, ok := registry.Aliases[pointer]
		if !ok {
			return fmt.Errorf("Render Blueprint capability registry is missing alias %s", pointer)
		}
		if err := validateBlueprintCapability(pointer, alias.BlueprintCapability); err != nil {
			return err
		}
		if alias.State != BlueprintCapabilityDeprecated || alias.Handler != blueprintHandlerCompileSource ||
			alias.Target != reviewed.targetPointer() || alias.Documentation != reviewed.documentation {
			return fmt.Errorf("Render Blueprint capability registry has invalid alias %s", pointer)
		}
		if _, ok := registry.Fields[alias.Target]; !ok {
			return fmt.Errorf("Render Blueprint alias %s has unknown target %s", pointer, alias.Target)
		}
	}
	if len(registry.Aliases) != len(reviewedBlueprintAliases) {
		return fmt.Errorf("Render Blueprint capability registry has unreviewed aliases")
	}
	return nil
}

func normalizeBlueprintAliases(source *BlueprintSource, registry *BlueprintCapabilityRegistry) []BlueprintSourceProblem {
	var problems []BlueprintSourceProblem
	var visit func(any, []string, blueprintCapabilityContext)
	visit = func(value any, path []string, context blueprintCapabilityContext) {
		if list, ok := value.([]any); ok {
			for index, item := range list {
				itemContext := context
				if context.kind == blueprintCapabilityServices {
					// Classify legacy static services before their runtime is normalized.
					object, _ := item.(map[string]any)
					runtime, present := object["runtime"]
					if !present {
						runtime = object["env"]
					}
					itemContext = blueprintServiceCapabilityContext(map[string]any{"type": object["type"], "runtime": runtime})
				}
				visit(item, append(append([]string(nil), path...), strconv.Itoa(index)), itemContext)
			}
			return
		}
		object, ok := value.(map[string]any)
		if !ok {
			return
		}
		for field, value := range object {
			pointer := blueprintFieldCapabilityPointer(context, field)
			alias, ok := registry.Aliases[pointer]
			if !ok {
				continue
			}
			reviewed, _ := reviewedBlueprintAliasAt(pointer)
			canonical := pointerLeaf(alias.Target)
			authoredPath := renderSchemaPointer(append(append([]string(nil), path...), field))
			canonicalPath := renderSchemaPointer(append(append([]string(nil), path...), canonical))
			if current, exists := object[canonical]; exists {
				message := ""
				switch {
				case reviewed.exclusive:
					message = fmt.Sprintf("%s is the deprecated alias of %s; declare only one of them", field, canonical)
				case blueprintEncodedValue(current) != blueprintEncodedValue(value):
					message = fmt.Sprintf("%s and %s must have the same value when both are declared", field, canonical)
				}
				if message != "" {
					location := lookupBlueprintLocation(authoredPath, source.Locations)
					problems = append(problems, BlueprintSourceProblem{
						Code: "BLUEPRINT_ALIAS_CONFLICT", Path: authoredPath, Message: message,
						Line: location.Line, Column: location.Column,
					})
					continue
				}
			} else {
				object[canonical] = value
				source.Locations[canonicalPath] = source.Locations[authoredPath]
				if source.authoredPaths == nil {
					source.authoredPaths = make(map[string]string)
				}
				source.authoredPaths[canonicalPath] = authoredPath
			}
			delete(object, field)
		}
		for field, child := range object {
			visit(child, append(append([]string(nil), path...), field), blueprintChildCapabilityContext(context, field, child))
		}
	}
	visit(source.Value, nil, blueprintCapabilityContext{kind: blueprintCapabilityRoot})
	return problems
}

// blueprintAliasWarnings locates each use of an alias that earns a plan
// warning (reviewedBlueprintAlias.notice), in source order.
func blueprintAliasWarnings(source *BlueprintSource) []BlueprintValidationError {
	if source == nil {
		return nil
	}
	notices := map[string]string{}
	for _, alias := range reviewedBlueprintAliases {
		if alias.notice != "" {
			notices[alias.field] = alias.notice
		}
	}
	var authored []string
	for _, path := range source.authoredPaths {
		if notices[pointerLeaf(path)] != "" {
			authored = append(authored, path)
		}
	}
	slices.SortFunc(authored, func(a, b string) int {
		la, lb := lookupBlueprintLocation(a, source.Locations), lookupBlueprintLocation(b, source.Locations)
		return cmp.Or(cmp.Compare(la.Line, lb.Line), cmp.Compare(la.Column, lb.Column), cmp.Compare(a, b))
	})
	warnings := make([]BlueprintValidationError, 0, len(authored))
	for _, path := range authored {
		warnings = append(warnings, blueprintLocatedError(source, notices[pointerLeaf(path)], path))
	}
	return warnings
}

// pointerLeaf is the last segment of a JSON Pointer: the field it names.
func pointerLeaf(pointer string) string {
	return pointer[strings.LastIndex(pointer, "/")+1:]
}

func (source *BlueprintSource) authoredPath(path string) string {
	for prefix := path; prefix != ""; {
		if authored, ok := source.authoredPaths[prefix]; ok {
			return authored + strings.TrimPrefix(path, prefix)
		}
		last := strings.LastIndex(prefix, "/")
		if last < 0 {
			break
		}
		prefix = prefix[:last]
	}
	return path
}
