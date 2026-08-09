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

func TestBlueprintCompilerRejectsPrebuiltImageBuildInputsAtTheirSourcePaths(t *testing.T) {
	_, problems := CompileBlueprintSource(`
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    repo: https://github.com/bex-co/api
    branch: main
    rootDir: cmd/api
    buildFilter: {paths: [cmd/api/**]}
    buildCommand: go build ./cmd/api
    dockerfilePath: Dockerfile
    autoDeploy: false
    autoDeployTrigger: off
`)
	want := map[string]bool{
		"#/services/0/repo":              false,
		"#/services/0/branch":            false,
		"#/services/0/rootDir":           false,
		"#/services/0/buildFilter":       false,
		"#/services/0/buildCommand":      false,
		"#/services/0/dockerfilePath":    false,
		"#/services/0/autoDeploy":        false,
		"#/services/0/autoDeployTrigger": false,
	}
	for _, problem := range problems {
		if problem.Code == "BLUEPRINT_CAPABILITY_INCOMPATIBLE" {
			if _, ok := want[problem.Path]; ok {
				want[problem.Path] = true
			}
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("missing prebuilt-image incompatibility at %s: %+v", path, problems)
		}
	}
	for _, problem := range problems {
		if problem.Path == "#/services/0/buildCommand" && problem.Code == "BLUEPRINT_CAPABILITY_INCOMPATIBLE" && !strings.Contains(problem.Message, "prebuilt image services") {
			t.Fatalf("buildCommand should have exactly the prebuilt-image diagnosis, got %+v", problems)
		}
	}
}

func TestBlueprintCompilerRejectsRuntimeIncompatibleBuildCommands(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		path     string
	}{
		{
			name: "Dockerfile build command",
			manifest: `services:
  - {type: web, name: api, runtime: docker, repo: https://github.com/bex-co/api, buildCommand: go build ./cmd/api}
`,
			path: "#/services/0/buildCommand",
		},
		{
			name: "image docker command",
			manifest: `services:
  - {type: web, name: api, runtime: image, image: {url: nginx:1.27}, dockerCommand: nginx -g 'daemon off;'}
`,
			path: "#/services/0/dockerCommand",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, problems := CompileBlueprintSource(tc.manifest)
			problem := findBlueprintProblem(problems, "BLUEPRINT_CAPABILITY_INCOMPATIBLE")
			if problem == nil || problem.Path != tc.path {
				t.Fatalf("CompileBlueprintSource() problems = %+v, want incompatibility at %s", problems, tc.path)
			}
		})
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

func TestBlueprintCompilerEnforcesCapabilityRegistryAtNestedPaths(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		manifest string
		path     string
	}{
		"image registry credential": {
			manifest: `services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27, creds: private-registry}
`,
			path: "#/services/0/image/creds",
		},
		"persistent disk": {
			manifest: `services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    disk: {name: data, mountPath: /data, sizeGB: 10}
`,
			path: "#/services/0/disk",
		},
		"nested database region": {
			manifest: `projects:
  - name: product
    environments:
      - name: production
        databases:
          - name: data
            region: oregon
`,
			path: "#/projects/0/environments/0/databases/0/region",
		},
		"unsupported postgres version": {
			manifest: `databases:
  - name: data
    postgresMajorVersion: "12"
`,
			path: "#/databases/0/postgresMajorVersion",
		},
		"cron pre-deploy command": {
			manifest: `services:
  - type: cron
    name: nightly
    runtime: image
    image: {url: nginx:1.27}
    schedule: "0 0 * * *"
    preDeployCommand: ./migrate
`,
			path: "#/services/0/preDeployCommand",
		},
		"static pre-deploy command": {
			manifest: `services:
  - type: web
    name: site
    runtime: static
    repo: https://github.com/bex/site
    staticPublishPath: dist
    preDeployCommand: ./migrate
`,
			path: "#/services/0/preDeployCommand",
		},
		"static build command": {
			manifest: `services:
  - type: web
    name: site
    runtime: static
    repo: https://github.com/bex/site
    staticPublishPath: dist
    buildCommand: npm run build
`,
			path: "#/services/0/buildCommand",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, problems := CompileBlueprintSource(tc.manifest)
			problem := findBlueprintProblem(problems, "BLUEPRINT_CAPABILITY_UNSUPPORTED")
			if problem == nil || problem.Path != tc.path {
				t.Fatalf("CompileBlueprintSource() problems = %+v, want unsupported path %s", problems, tc.path)
			}
		})
	}
}

func TestBlueprintCapabilityStateControlsRuntimeRefusal(t *testing.T) {
	t.Parallel()
	unsupported := &BlueprintCapabilityRegistry{Fields: map[string]BlueprintCapability{
		"#/definitions/serverService/properties/disk": {State: BlueprintCapabilityUnsupported},
	}}
	if !blueprintCapabilityUnsupported(unsupported, "#/definitions/serverService/properties/disk") {
		t.Fatal("unsupported registry field did not refuse the capability")
	}
	unsupported.Fields["#/definitions/serverService/properties/disk"] = BlueprintCapability{State: BlueprintCapabilityTranslated}
	if blueprintCapabilityUnsupported(unsupported, "#/definitions/serverService/properties/disk") {
		t.Fatal("translated registry field still refused the capability")
	}

	enums := &BlueprintCapabilityRegistry{EnumValues: map[string]map[string]BlueprintCapability{
		"#/definitions/autoDeployTrigger/enum": {`"checksPass"`: {State: BlueprintCapabilityUnsupported}},
	}}
	if !blueprintEnumCapabilityUnsupported(enums, "#/definitions/autoDeployTrigger/enum", `"checksPass"`) {
		t.Fatal("unsupported enum did not refuse the capability")
	}
}

func TestBlueprintServiceCapabilityContextUsesDeclarationKind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value map[string]any
		want  string
	}{
		{name: "key value", value: map[string]any{"type": "keyvalue"}, want: "redisServer"},
		{name: "cron", value: map[string]any{"type": "cron"}, want: "cronService"},
		{name: "static", value: map[string]any{"runtime": "static"}, want: "staticService"},
		{name: "server", value: map[string]any{"type": "web"}, want: "serverService"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(blueprintServiceCapabilityContext(tc.value).kind); got != tc.want {
				t.Errorf("blueprintServiceCapabilityContext() = %q, want %q", got, tc.want)
			}
		})
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

func TestBlueprintCompilerEnforcesStructuralBudgets(t *testing.T) {
	// Deeper than blueprintMaxDepth: 110 levels of single-key mappings.
	var deep strings.Builder
	for i := 0; i < 110; i++ {
		deep.WriteString(strings.Repeat("  ", i))
		deep.WriteString("k:\n")
	}
	deep.WriteString(strings.Repeat("  ", 110))
	deep.WriteString(`"leaf"`)

	// More than blueprintMaxNodes: key-heavy mappings (each entry costs a key
	// node and a value node) while staying under the locations budget.
	var nodes strings.Builder
	nodes.WriteString("root:\n")
	for i := 0; i < 10_000; i++ {
		fmt.Fprintf(&nodes, "  k%d: {a: 1, b: 2, c: 3, d: 4, e: 5}\n", i)
	}

	// More than blueprintMaxLocations: keyless nested sequences, so value
	// nodes (each with a locations entry) outnumber the locations budget
	// while total nodes stay under blueprintMaxNodes.
	var locations strings.Builder
	for i := 0; i < 7_500; i++ {
		locations.WriteString("- [0, 0, 0, 0, 0, 0, 0, 0, 0, 0]\n")
	}

	// More than blueprintMaxCollectionEntries in one sequence.
	var collection strings.Builder
	for i := 0; i < blueprintMaxCollectionEntries+1; i++ {
		collection.WriteString("- x\n")
	}

	// Longer than blueprintMaxScalarBytes.
	scalar := "k: " + strings.Repeat("x", blueprintMaxScalarBytes+1)

	for _, tc := range []struct {
		name     string
		manifest string
		code     string
	}{
		{"depth", deep.String(), "BLUEPRINT_YAML_DEPTH"},
		{"nodes", nodes.String(), "BLUEPRINT_YAML_NODES"},
		{"locations", locations.String(), "BLUEPRINT_YAML_LOCATIONS"},
		{"collection entries", collection.String(), "BLUEPRINT_YAML_COLLECTION"},
		{"scalar length", scalar, "BLUEPRINT_YAML_SCALAR_SIZE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, problems := CompileBlueprintSource(tc.manifest)
			problem := findBlueprintProblem(problems, tc.code)
			if problem == nil {
				t.Fatalf("CompileBlueprintSource() problems = %+v, want %s", problems, tc.code)
			}
			if problem.Path == "" || problem.Line == 0 {
				t.Errorf("budget problem = %+v, want source path and location", problem)
			}
		})
	}
}

func TestBlueprintCompilerAcceptsDocumentNearStructuralBudgets(t *testing.T) {
	// Deepest accepted nesting: exactly blueprintMaxDepth levels.
	var deep strings.Builder
	for i := 0; i < blueprintMaxDepth; i++ {
		deep.WriteString(strings.Repeat("  ", i))
		deep.WriteString("k:\n")
	}
	deep.WriteString(strings.Repeat("  ", blueprintMaxDepth))
	deep.WriteString(`"leaf"`)
	if _, problems := parseBlueprintSource(deep.String()); len(problems) != 0 {
		t.Fatalf("nesting at the depth budget problems = %+v", problems)
	}

	// Largest accepted collection: exactly blueprintMaxCollectionEntries.
	var collection strings.Builder
	for i := 0; i < blueprintMaxCollectionEntries; i++ {
		collection.WriteString("- x\n")
	}
	if _, problems := parseBlueprintSource(collection.String()); len(problems) != 0 {
		t.Fatalf("collection at the entry budget problems = %+v", problems)
	}

	// Longest accepted scalar: exactly blueprintMaxScalarBytes.
	scalar := "k: " + strings.Repeat("x", blueprintMaxScalarBytes)
	if _, problems := parseBlueprintSource(scalar); len(problems) != 0 {
		t.Fatalf("scalar at the length budget problems = %+v", problems)
	}

	// Breadth just under the node and locations budgets: 6800 sequences of 10
	// items => 74,801 located value nodes (budget 75,000).
	var wide strings.Builder
	for i := 0; i < 6_800; i++ {
		wide.WriteString("- [0, 0, 0, 0, 0, 0, 0, 0, 0, 0]\n")
	}
	if _, problems := parseBlueprintSource(wide.String()); len(problems) != 0 {
		t.Fatalf("breadth just under the node/location budgets problems = %+v", problems)
	}

	// A large realistic Blueprint well under every budget compiles clean.
	var blueprint strings.Builder
	blueprint.WriteString("services:\n")
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&blueprint, "  - type: web\n    name: api-%d\n    runtime: image\n    image: {url: nginx:1.27}\n", i)
	}
	if _, problems := CompileBlueprintSource(blueprint.String()); len(problems) != 0 {
		t.Fatalf("500-service Blueprint problems = %+v", problems)
	}
}
