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

const compilerValidBlueprint = `
services:
  - type: web
    name: api
    runtime: image
    image:
      url: nginx:1.27
`

func TestBlueprintCompilerAcceptsReviewedRenderBlueprint(t *testing.T) {
	source, problems := CompileBlueprintSource(compilerValidBlueprint)
	if len(problems) != 0 {
		t.Fatalf("CompileBlueprintSource(valid) problems = %+v", problems)
	}
	if source == nil || source.Value == nil {
		t.Fatal("CompileBlueprintSource(valid) returned no source value")
	}
	location, ok := source.Locations["#/services/0/image/url"]
	if !ok || location.Line == 0 || location.Column == 0 {
		t.Fatalf("image URL location = %+v, present=%v", location, ok)
	}
}

func TestBlueprintCompilerRejectsDuplicateKeysBeforeSchema(t *testing.T) {
	_, problems := CompileBlueprintSource(`
services: []
services: []
`)
	problem := findBlueprintProblem(problems, "BLUEPRINT_DUPLICATE_KEY")
	if problem == nil {
		t.Fatalf("duplicate-key problems = %+v", problems)
	}
	if problem.Path != "#/services" || problem.Line != 3 || problem.Column != 1 {
		t.Fatalf("duplicate-key problem = %+v", *problem)
	}
}

func TestBlueprintCompilerRejectsUnknownFieldsWithLocation(t *testing.T) {
	_, problems := CompileBlueprintSource(`
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    notARealRenderField: true
`)
	problem := findBlueprintProblem(problems, "BLUEPRINT_SCHEMA_INVALID")
	if problem == nil {
		t.Fatalf("unknown-field problems = %+v", problems)
	}
	if problem.Path != "#/services/0/notARealRenderField" || problem.Line != 7 || problem.Column != 26 {
		t.Fatalf("unknown-field location = %+v", *problem)
	}
	if !strings.Contains(problem.Message, "notARealRenderField") {
		t.Fatalf("unknown-field message = %q", problem.Message)
	}
}

func TestBlueprintCompilerAcceptsOnlyStructuredXBexExtension(t *testing.T) {
	manifest := `
x-bex:
  builder: auto
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    x-bex:
      builder: dockerfile
`
	if _, problems := CompileBlueprintSource(manifest); len(problems) != 0 {
		t.Fatalf("x-bex problems = %+v", problems)
	}
	source, problems := CompileBlueprintSource(manifest)
	if len(problems) != 0 {
		t.Fatalf("x-bex source problems = %+v", problems)
	}
	ir, irProblems := NormalizeBlueprintIR(source)
	if len(irProblems) != 0 || len(ir.Resources) != 1 {
		t.Fatalf("x-bex normalization = resources %#v problems %+v", ir.Resources, irProblems)
	}
	if builder, ok := BlueprintExtensionBuilder(ir.Resources[0]); !ok || builder != "dockerfile" {
		t.Fatalf("x-bex builder = %q, %v", builder, ok)
	}

	_, problems = CompileBlueprintSource(strings.Replace(manifest, "builder: auto", "unknown: true", 1))
	if findBlueprintProblem(problems, "BLUEPRINT_SCHEMA_INVALID") == nil {
		t.Fatalf("unknown x-bex problems = %+v", problems)
	}
}

func TestBlueprintCompilerRejectsPreviewOnlyEnvValue(t *testing.T) {
	_, problems := CompileBlueprintSource(`
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    envVars:
      - key: API_URL
        value: https://api.example.com
        previewValue: https://preview-api.example.com
`)
	problem := findBlueprintProblem(problems, "BLUEPRINT_CAPABILITY_UNSUPPORTED")
	if problem == nil || problem.Path != "#/services/0/envVars/0/previewValue" {
		t.Fatalf("previewValue problems = %+v", problems)
	}
}

func TestBlueprintCompilerRejectsUnsupportedRenderCapabilitiesAtSourcePath(t *testing.T) {
	t.Parallel()
	_, problems := CompileBlueprintSource(`
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    autoDeployTrigger: checksPass
`)
	problem := findBlueprintProblem(problems, "BLUEPRINT_CAPABILITY_UNSUPPORTED")
	if problem == nil || problem.Path != "#/services/0/autoDeployTrigger" {
		t.Fatalf("checksPass problem = %+v; all = %+v", problem, problems)
	}

	_, problems = CompileBlueprintSource(`
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    builder: dockerfile
`)
	problem = findBlueprintProblem(problems, "BLUEPRINT_EXTENSION_REQUIRED")
	if problem == nil || problem.Path != "#/services/0/builder" {
		t.Fatalf("legacy builder problem = %+v; all = %+v", problem, problems)
	}
}

func TestBlueprintCompilerRejectsPreviewFieldsEvenWhenFalseOrEmpty(t *testing.T) {
	t.Parallel()
	for name, manifest := range map[string]string{
		"service preview plan": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx}
    previewPlan: free
`,
		"key-value preview plan": `services:
  - name: cache
    type: keyvalue
    previewPlan: free
`,
		"preview toggle false": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx}
    pullRequestPreviewsEnabled: false
`,
	} {
		t.Run(name, func(t *testing.T) {
			_, problems := CompileBlueprintSource(manifest)
			problem := findBlueprintProblem(problems, "BLUEPRINT_CAPABILITY_UNSUPPORTED")
			if problem == nil {
				t.Fatalf("CompileBlueprintSource() problems = %#v, want preview capability refusal", problems)
			}
			if problem.Path == "" || problem.Line == 0 || problem.Column == 0 {
				t.Errorf("preview refusal = %#v, want source path and location", problem)
			}
		})
	}
}

func TestBlueprintCompilerAcceptsRootVersionAsGrammarMetadata(t *testing.T) {
	_, problems := CompileBlueprintSource(`
version: "1"
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
`)
	if len(problems) != 0 {
		t.Fatalf("version metadata problems = %+v", problems)
	}
}

func TestBlueprintCompilerRejectsAmbiguousYAMLValues(t *testing.T) {
	_, aliasProblems := CompileBlueprintSource(`
services:
  - &service
    type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
ungrouped:
  services: [*service]
`)
	if findBlueprintProblem(aliasProblems, "BLUEPRINT_YAML_ALIAS") == nil {
		t.Fatalf("alias problems = %+v", aliasProblems)
	}

	_, timestampProblems := CompileBlueprintSource(`
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    branch: 2026-08-02
`)
	if findBlueprintProblem(timestampProblems, "BLUEPRINT_YAML_SCALAR") == nil {
		t.Fatalf("timestamp problems = %+v", timestampProblems)
	}
}

func TestBlueprintCompilerReportsSeveralIndependentSchemaProblems(t *testing.T) {
	_, problems := CompileBlueprintSource(`
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    notARealRenderField: true
databases:
  - name: data
    connectionPool: not-a-pool
`)
	if got := len(problems); got < 2 {
		t.Fatalf("schema problems = %+v, want at least two independent errors", problems)
	}
}

func findBlueprintProblem(problems []BlueprintSourceProblem, code string) *BlueprintSourceProblem {
	for i := range problems {
		if problems[i].Code == code {
			return &problems[i]
		}
	}
	return nil
}
