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

// Package executiontest models Kubernetes field expansion for projection tests.
package executiontest

import "strings"

// Expand models the kubelet's one-pass expansion of EnvVar.Value, Command and
// Args. Mapping results are appended verbatim and never recursively expanded.
// Reviewed kubelet expansion contract:
// https://github.com/kubernetes/kubernetes/blob/v1.36.1/third_party/forked/golang/expansion/expand.go
func Expand(input string, env map[string]string) string {
	var result strings.Builder
	for cursor := 0; cursor < len(input); {
		if input[cursor] != '$' || cursor+1 == len(input) {
			result.WriteByte(input[cursor])
			cursor++
			continue
		}
		switch input[cursor+1] {
		case '$':
			result.WriteByte('$')
			cursor += 2
		case '(':
			closing := strings.IndexByte(input[cursor+2:], ')')
			if closing < 0 {
				// Only the opener is consumed: dollars later in an incomplete reference
				// still follow the ordinary one-pass rules.
				result.WriteString("$(")
				cursor += 2
				continue
			}
			end := cursor + 2 + closing
			if value, ok := env[input[cursor+2:end]]; ok {
				result.WriteString(value)
			} else {
				result.WriteString(input[cursor : end+1])
			}
			cursor = end + 1
		default:
			result.WriteString(input[cursor : cursor+2])
			cursor += 2
		}
	}
	return result.String()
}
