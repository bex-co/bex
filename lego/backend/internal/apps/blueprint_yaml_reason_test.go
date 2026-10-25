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
	"strings"
	"testing"
)

// w4/397: a YAML syntax refusal keeps the parser's reason, not only the line.
func TestBlueprintYAMLSyntaxKeepsReason(t *testing.T) {
	_, _, problems := CompileBlueprintIR("services:\n\t- type: web\n")
	if len(problems) != 1 || problems[0].Code != "BLUEPRINT_YAML_SYNTAX" {
		t.Fatalf("problems = %+v", problems)
	}
	if !strings.Contains(problems[0].Message, "tab character") {
		t.Fatalf("message = %q, want the tab reason", problems[0].Message)
	}
	// The reason never echoes manifest content (a value could be a secret).
	_, _, leaked := CompileBlueprintIR("services:\n  - type: web\n   name: \"s3cr3t-value\n")
	for _, p := range leaked {
		if strings.Contains(p.Message, "s3cr3t-value") {
			t.Fatalf("message echoes manifest content: %q", p.Message)
		}
	}
}
