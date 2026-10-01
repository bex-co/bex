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
	"fmt"
	"strconv"
	"strings"
)

// Aliases are reviewed separately from the immutable upstream schema. The
// current overlay accepts only Render's documented env -> runtime spelling.
func validateBlueprintAliases(registry *BlueprintCapabilityRegistry) error {
	for _, kind := range []blueprintCapabilityKind{blueprintCapabilityServer, blueprintCapabilityCron, blueprintCapabilityStatic} {
		pointer := "#/definitions/" + string(kind) + "/properties/env"
		alias, ok := registry.Aliases[pointer]
		if !ok {
			return fmt.Errorf("Render Blueprint capability registry is missing alias %s", pointer)
		}
		if err := validateBlueprintCapability(pointer, alias.BlueprintCapability); err != nil {
			return err
		}
		if alias.State != BlueprintCapabilityDeprecated || alias.Handler != blueprintHandlerCompileSource ||
			alias.Target != strings.TrimSuffix(pointer, "env")+"runtime" ||
			alias.Documentation != "https://render.com/docs/blueprint-spec#runtime" {
			return fmt.Errorf("Render Blueprint capability registry has invalid alias %s", pointer)
		}
		if _, ok := registry.Fields[alias.Target]; !ok {
			return fmt.Errorf("Render Blueprint alias %s has unknown target %s", pointer, alias.Target)
		}
	}
	if len(registry.Aliases) != 3 {
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
			alias, ok := registry.Aliases[blueprintFieldCapabilityPointer(context, field)]
			if !ok {
				continue
			}
			canonical := alias.Target[strings.LastIndex(alias.Target, "/")+1:]
			authoredPath := renderSchemaPointer(append(append([]string(nil), path...), field))
			canonicalPath := renderSchemaPointer(append(append([]string(nil), path...), canonical))
			if current, exists := object[canonical]; exists {
				if blueprintEncodedValue(current) != blueprintEncodedValue(value) {
					location := lookupBlueprintLocation(authoredPath, source.Locations)
					problems = append(problems, BlueprintSourceProblem{
						Code: "BLUEPRINT_ALIAS_CONFLICT", Path: authoredPath,
						Message: "env and runtime must have the same value when both are declared",
						Line:    location.Line, Column: location.Column,
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
