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

// Render's existingResources: create_new (w2/m261): "creates a separate set of
// resources, leaving existing resources unchanged … via a random suffix that is
// assigned to this Blueprint and appended to the name of all resources this
// Blueprint manages". The suffix is applied to the decoded source between
// schema validation and normalization, so every later step — references,
// adoption by name, the plan, requiredManualEnvVars — sees the suffixed names,
// and source locations (keyed by JSON pointer) still point at the customer's
// YAML. Projects and environments group resources rather than being managed
// copies of them, so they keep their names.

// compileBlueprintIRFor compiles manifest as the Blueprint with suffix manages
// it; an empty suffix is CompileBlueprintIR.
func compileBlueprintIRFor(manifest, suffix string) (*BlueprintSource, BlueprintIR, []BlueprintSourceProblem) {
	source, problems := CompileBlueprintSource(manifest)
	if len(problems) > 0 {
		return source, BlueprintIR{}, problems
	}
	suffixBlueprintResources(source.Value, suffix)
	ir, problems := NormalizeBlueprintIR(source)
	if len(problems) > 0 {
		return source, BlueprintIR{}, problems
	}
	return source, ir, nil
}

// suffixBlueprintResources appends "-<suffix>" to every resource the document
// declares and to every reference to one, in place. A reference to a resource
// the document does not declare names an existing workspace resource (an env
// group or a uniquely named database the Blueprint links to), so it keeps its
// name.
func suffixBlueprintResources(root any, suffix string) {
	doc, ok := root.(map[string]any)
	if suffix == "" || !ok {
		return
	}
	declared := map[string]map[string]bool{"services": {}, "databases": {}, "envVarGroups": {}}
	forEachBlueprintContainer(doc, func(c blueprintContainer) {
		for _, declaration := range blueprintResourceDeclarations {
			entries, _ := c.value[declaration.Field].([]any)
			for _, raw := range entries {
				entry, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				if name, ok := entry["name"].(string); ok && name != "" {
					declared[declaration.Field][name] = true
					entry["name"] = name + "-" + suffix
				}
			}
		}
	})
	suffixReferences(doc, suffix, declared)
}

// referenceTargets maps each reference key to the declaration list its target
// lives in.
var referenceTargets = map[string]string{"fromService": "services", "fromDatabase": "databases", "fromGroup": "envVarGroups"}

// suffixReferences renames every fromService/fromDatabase target and fromGroup
// link that names a declared resource, anywhere in the document.
func suffixReferences(node any, suffix string, declared map[string]map[string]bool) {
	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			list, isReference := referenceTargets[key]
			if !isReference {
				suffixReferences(child, suffix, declared)
				continue
			}
			switch ref := child.(type) {
			case map[string]any: // fromService / fromDatabase: {name, …}
				if name, ok := ref["name"].(string); ok && declared[list][name] {
					ref["name"] = name + "-" + suffix
				}
			case string: // fromGroup: <name>
				if declared[list][ref] {
					v[key] = ref + "-" + suffix
				}
			}
		}
	case []any:
		for _, child := range v {
			suffixReferences(child, suffix, declared)
		}
	}
}
