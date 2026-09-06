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

package controller

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

const annotationProjectedSpec = "app.bex.co/projected-spec"

// projectUnstructuredSpec retains admission defaults while enforcing bex's
// desired fields and deleting fields withdrawn since the previous projection.
// Existing children are adopted with one legacy whole-spec replacement, then
// remember only bex's input, never the admission-defaulted result. This also
// tracks arbitrary tenant PostgreSQL parameter names without owning CNPG's
// own parameter defaults. The annotation contains spec only, not credentials.
func projectUnstructuredSpec(object *unstructured.Unstructured, desired map[string]any) error {
	encoded, err := json.Marshal(desired)
	if err != nil {
		return fmt.Errorf("encoding projected spec: %w", err)
	}
	annotations := object.GetAnnotations()
	var previous map[string]any
	if raw := annotations[annotationProjectedSpec]; raw != "" {
		// Use Kubernetes' JSON decoder to preserve integer types in unstructured
		// fields; encoding/json's float64 conversion would itself prevent convergence.
		if err := json.Unmarshal([]byte(raw), &previous); err != nil {
			return fmt.Errorf("decoding previous projected spec: %w", err)
		}
	}
	if previous == nil {
		object.Object["spec"] = runtime.DeepCopyJSONValue(desired)
	} else {
		live, _, err := unstructured.NestedMap(object.Object, "spec")
		if err != nil {
			return err
		}
		object.Object["spec"] = mergeProjectedMap(live, previous, desired)
	}
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[annotationProjectedSpec] = string(encoded)
	object.SetAnnotations(annotations)
	return nil
}

func mergeProjectedMap(live, previous, desired map[string]any) map[string]any {
	result := runtime.DeepCopyJSONValue(live).(map[string]any)
	if result == nil {
		result = map[string]any{}
	}
	for key := range previous {
		if _, keep := desired[key]; !keep {
			delete(result, key)
		}
	}
	for key, value := range desired {
		if next, ok := value.(map[string]any); ok {
			current, _ := live[key].(map[string]any)
			old, _ := previous[key].(map[string]any)
			result[key] = mergeProjectedMap(current, old, next)
			continue
		}
		// CNPG defaults fields inside managed roles, plugins, and external
		// clusters. These lists use unique names; retain defaults within a
		// surviving entry, but replace membership and order with bex intent.
		if next, ok := value.([]any); ok && (key == "roles" || key == "plugins" || key == "externalClusters") {
			current, _ := live[key].([]any)
			old, _ := previous[key].([]any)
			result[key] = mergeNamedProjectionList(current, old, next)
			continue
		}
		result[key] = runtime.DeepCopyJSONValue(value)
	}
	return result
}

// Only called for the explicitly named CNPG lists above. Other lists are
// atomic: retaining an omitted CIDR, SQL statement, or certificate DNS name
// would change tenant intent.
func mergeNamedProjectionList(live, previous, desired []any) []any {
	byName := func(items []any) map[string]map[string]any {
		result := map[string]map[string]any{}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil
			}
			name, ok := entry["name"].(string)
			if !ok || name == "" {
				return nil
			}
			if _, duplicate := result[name]; duplicate {
				return nil
			}
			result[name] = entry
		}
		return result
	}
	current, old, next := byName(live), byName(previous), byName(desired)
	if current == nil || old == nil || next == nil {
		return runtime.DeepCopyJSONValue(desired).([]any)
	}
	result := make([]any, 0, len(desired))
	for _, item := range desired {
		entry := item.(map[string]any)
		name := entry["name"].(string)
		result = append(result, mergeProjectedMap(current[name], old[name], entry))
	}
	return result
}
