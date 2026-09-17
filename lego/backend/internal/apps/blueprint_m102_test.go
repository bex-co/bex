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
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestShippedExampleBlueprintsValidate(t *testing.T) {
	svc := newBlueprintValidator(t)
	root := repoRoot(t)
	// Only files git actually tracks. The walk used to cover the whole repo root
	// and skip four directory names by hand, which swept up any gitignored
	// sibling checkout a developer happened to keep there — on 2026-09-16 an
	// unrelated project's bex.yml failed this test on a local machine while CI,
	// with its clean checkout, stayed green. A false failure that only fires
	// locally is the worst kind: it teaches people to ignore a red suite. The
	// test's own name is the right scope — the blueprints *we ship*.
	tracked := gitTrackedFiles(t, root)
	var found int
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == ".output" || base == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if name != "render.yaml" && name != "bex.yml" {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || !tracked[filepath.ToSlash(rel)] {
			return nil // untracked or ignored — not ours to validate
		}
		found++
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("read %s: %v", path, readErr)
			return nil
		}
		_, problems := CompileBlueprintSource(string(raw))
		if len(problems) > 0 {
			t.Errorf("%s compiler problems = %+v", rel, problems)
			return nil
		}
		validation, valErr := svc.ValidateBlueprint(context.Background(), "", string(raw))
		if valErr != nil {
			t.Errorf("%s ValidateBlueprint: %v", rel, valErr)
			return nil
		}
		if !validation.Valid || validation.Plan == nil {
			t.Errorf("%s valid=%v plan=%v errors=%+v", rel, validation.Valid, validation.Plan != nil, validation.Errors)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found < 7 {
		t.Fatalf("found %d shipped manifests, want at least 7", found)
	}
}

func TestStaticSitePlanIsTheNamedOffender(t *testing.T) {
	manifest := staticSiteManifest("    staticPublishPath: .\n    plan: free\n")
	problems := mustCompileProblems(t, manifest)
	joined := problemText(problems)
	if !strings.Contains(joined, "plan") {
		t.Fatalf("problems = %+v, want the extra property named plan", problems)
	}
	if strings.Contains(joined, "staticPublishPath") {
		t.Fatalf("problems = %+v, must not blame the legal staticPublishPath key", problems)
	}
}

func TestObeyingStaticSiteErrorsConvergesByRemovingPlan(t *testing.T) {
	svc := newBlueprintValidator(t)
	start := staticSiteManifest("    staticPublishPath: .\n    plan: free\n")
	first, err := svc.ValidateBlueprint(context.Background(), "", start)
	if err != nil {
		t.Fatal(err)
	}
	if first.Valid || !strings.Contains(problemErrors(first), "plan") || strings.Contains(problemErrors(first), "staticPublishPath") {
		t.Fatalf("step 1 = %+v, want plan named and staticPublishPath omitted", first.Errors)
	}
	fixed := staticSiteManifest("    staticPublishPath: .\n")
	second, err := svc.ValidateBlueprint(context.Background(), "", fixed)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Valid || second.Plan == nil {
		t.Fatalf("removing plan should validate, got %+v", second)
	}
}

func TestBlueprintMissingPublishDirectorySpeaksManifestKey(t *testing.T) {
	svc := newBlueprintValidator(t)
	validation, err := svc.ValidateBlueprint(context.Background(), "", staticSiteManifest(""))
	if err != nil {
		t.Fatal(err)
	}
	got := problemErrors(validation)
	if validation.Valid || !strings.Contains(got, "staticPublishPath is required for a static_site") {
		t.Fatalf("missing publish dir = %+v, want staticPublishPath", validation.Errors)
	}
	if strings.Contains(got, "publishPath is required") {
		t.Fatalf("blueprint error still used REST field name: %s", got)
	}
	withPath := staticSiteManifest("    staticPublishPath: .\n")
	ok, err := svc.ValidateBlueprint(context.Background(), "", withPath)
	if err != nil {
		t.Fatal(err)
	}
	if !ok.Valid {
		t.Fatalf("adding staticPublishPath should validate, got %+v", ok.Errors)
	}
}

func TestCreateStaticSiteErrorKeepsRESTFieldName(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "site", Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/acme/site",
	})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "publishPath is required for a static_site") {
		t.Fatalf("REST create = %v, want publishPath", err)
	}
	if strings.Contains(err.Error(), "staticPublishPath") {
		t.Fatalf("REST create must not use the manifest key: %v", err)
	}
}

func TestBlueprintSchemaControlMessages(t *testing.T) {
	cases := []struct {
		name, manifest, want string
	}{
		{"cron validates", `services:
  - type: cron
    name: job
    runtime: docker
    repo: https://github.com/bex-co/bex
    branch: main
    schedule: "*/5 * * * *"
    plan: free
`, ""},
		{"nonsense type lists cron", `services:
  - type: nonsense
    name: job
    runtime: docker
    repo: https://github.com/bex-co/bex
    branch: main
`, "value must be one of 'web', 'worker', 'pserv', 'cron', 'keyvalue', 'redis'"},
		{"duplicate names", `services:
  - type: web
    name: qa-dup
    runtime: image
    image: {url: nginx:1.27}
  - type: web
    name: qa-dup
    runtime: image
    image: {url: nginx:1.27}
`, `duplicate name "qa-dup" for service (first declared at #/services/0)`},
		{"missing name", `services:
  - type: web
    runtime: image
    image: {url: nginx:1.27}
`, "missing property 'name'"},
		{"unknown property", `services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
    totallyUnknownField: true
`, "additional properties 'totallyUnknownField' not allowed"},
		{"non-YAML", `{`, "Blueprint is not valid YAML"},
		{"empty document", ``, "Blueprint must contain one YAML document"},
		{"unknown root", "foo: bar\n", "additional properties 'foo' not allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, problems := CompileBlueprintIR(tc.manifest)
			if tc.want == "" {
				if len(problems) != 0 {
					t.Fatalf("problems = %+v, want valid", problems)
				}
				return
			}
			if !strings.Contains(problemText(problems), tc.want) {
				t.Fatalf("problems = %+v, want %q", problems, tc.want)
			}
		})
	}
}

func TestAnyOfReportsTheWrongKeyOnEachServiceKind(t *testing.T) {
	cases := []struct {
		name, manifest, want, not string
	}{
		{"static plan", staticSiteManifest("    staticPublishPath: .\n    plan: free\n"), "plan", "staticPublishPath"},
		{"cron extra", `services:
  - type: cron
    name: job
    runtime: docker
    repo: https://github.com/bex-co/bex
    branch: main
    schedule: "*/5 * * * *"
    totallyUnknownField: true
`, "totallyUnknownField", "ipAllowList"},
		{"server extra", `services:
  - type: web
    name: api
    runtime: docker
    repo: https://github.com/bex-co/bex
    branch: main
    plan: free
    totallyUnknownField: true
`, "totallyUnknownField", "ipAllowList"},
		{"redis extra", `services:
  - type: keyvalue
    name: cache
    ipAllowList: [{source: 0.0.0.0/0}]
    totallyUnknownField: true
`, "totallyUnknownField", "runtime"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := mustCompileProblems(t, tc.manifest)
			got := problemText(problems)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("problems = %+v, want %q", problems, tc.want)
			}
			if tc.not != "" && strings.Contains(got, tc.not) {
				t.Fatalf("problems = %+v, must not mention %q", problems, tc.not)
			}
		})
	}
}

func TestAnyOfDoesNotCollapseIndependentErrors(t *testing.T) {
	twoKeys := staticSiteManifest("    staticPublishPath: .\n    plan: free\n    totallyUnknownField: true\n")
	problems := mustCompileProblems(t, twoKeys)
	got := problemText(problems)
	if !strings.Contains(got, "plan") || !strings.Contains(got, "totallyUnknownField") {
		t.Fatalf("two extra keys collapsed: %+v", problems)
	}

	twoServices := `services:
  - type: web
    name: api
    runtime: docker
    repo: https://github.com/bex-co/bex
    branch: main
    plan: free
    totallyUnknownField: true
  - type: web
    name: site
    runtime: static
    repo: https://github.com/bex-co/bex
    branch: main
    staticPublishPath: .
    plan: free
`
	problems = mustCompileProblems(t, twoServices)
	got = problemText(problems)
	if !strings.Contains(got, "totallyUnknownField") || !strings.Contains(got, "plan") {
		t.Fatalf("two services collapsed: %+v", problems)
	}
}

func TestEnvVarAnyOfNamesTheEnvVarFault(t *testing.T) {
	base := func(env string) string {
		return `services:
  - type: web
    name: api
    runtime: docker
    repo: https://github.com/bex-co/bex
    branch: main
    plan: free
    envVars:
` + env
	}
	cases := []struct {
		name, env, want, not string
	}{
		{"bogus inside fromDatabase", `      - key: DATABASE_URL
        fromDatabase: {name: db, property: connectionString, extra: true}
`, "extra", "ipAllowList"},
		{"invalid fromDatabase property", `      - key: DATABASE_URL
        fromDatabase: {name: db, property: notAProperty}
`, "connectionString", "ipAllowList"},
		{"bogus beside value", `      - key: MESSAGE
        value: hello
        extra: true
`, "extra", "ipAllowList"},
		{"fromService missing type", `      - key: HOST
        fromService: {name: other}
`, "type", "ipAllowList"},
		{"value and fromDatabase", `      - key: DATABASE_URL
        value: ignored
        fromDatabase: {name: db, property: connectionString}
`, "value", "ipAllowList"},
	}
	seen := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := mustCompileProblems(t, base(tc.env))
			got := problemText(problems)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("problems = %+v, want %q", problems, tc.want)
			}
			if strings.Contains(got, tc.not) {
				t.Fatalf("problems = %+v, must not mention %q", problems, tc.not)
			}
			seen[tc.name] = got
		})
	}
	unique := map[string]string{}
	for name, text := range seen {
		if first, dup := unique[text]; dup {
			t.Fatalf("env-var faults %q and %q produced identical errors:\n%s", first, name, text)
		}
		unique[text] = name
	}

	controls := []string{
		base("      - key: MESSAGE\n        value: hello\n"),
		`services:
  - type: web
    name: api
    runtime: docker
    repo: https://github.com/bex-co/bex
    branch: main
    plan: free
`,
	}
	svc := newBlueprintValidator(t)
	for _, manifest := range controls {
		validation, err := svc.ValidateBlueprint(context.Background(), "", manifest)
		if err != nil {
			t.Fatal(err)
		}
		if !validation.Valid {
			t.Fatalf("control invalid: %+v", validation.Errors)
		}
	}
}

func TestGraphQLAndMCPShareValidateBlueprintProblems(t *testing.T) {
	svc := newBlueprintValidator(t)
	validation, err := svc.ValidateBlueprint(context.Background(), "", staticSiteManifest("    staticPublishPath: .\n    plan: free\n"))
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || len(validation.Errors) == 0 {
		t.Fatalf("want shared invalid result, got %+v", validation)
	}
	messages := make([]string, len(validation.Errors))
	for i, item := range validation.Errors {
		messages[i] = item.Error
	}
	// GraphQL `errors` and MCP validate_bex_yml both return these strings
	// untransformed from ValidateBlueprint (graphql.go errors field, mcp.go
	// validate_bex_yml handler).
	if !strings.Contains(strings.Join(messages, "\n"), "plan") {
		t.Fatalf("shared messages = %v", messages)
	}
}

func newBlueprintValidator(t *testing.T) *Service {
	t.Helper()
	return &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "..", ".."))
}

// gitTrackedFiles is the set of repo-relative paths git tracks, used to keep the
// shipped-blueprint walk from validating files that are not part of the repo.
// A checkout without git is a hard failure rather than a silent pass: a walk that
// quietly validated nothing would be worse than not having the test.
func gitTrackedFiles(t *testing.T, root string) map[string]bool {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files in %s: %v", root, err)
	}
	tracked := make(map[string]bool)
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			tracked[name] = true
		}
	}
	return tracked
}

func staticSiteManifest(extra string) string {
	return `services:
  - name: qa-static-bp
    type: web
    runtime: static
    repo: https://github.com/bex-co/bex
    rootDir: examples/static-site
    branch: main
` + extra
}

func mustCompileProblems(t *testing.T, manifest string) []BlueprintSourceProblem {
	t.Helper()
	_, problems := CompileBlueprintSource(manifest)
	if len(problems) == 0 {
		t.Fatal("want schema problems, got none")
	}
	return problems
}

func problemText(problems []BlueprintSourceProblem) string {
	var b strings.Builder
	for _, problem := range problems {
		b.WriteString(problem.Path)
		b.WriteByte(' ')
		b.WriteString(problem.Message)
		b.WriteByte('\n')
	}
	return b.String()
}

func problemErrors(v BlueprintValidation) string {
	var b strings.Builder
	for _, item := range v.Errors {
		b.WriteString(item.Error)
		b.WriteByte('\n')
	}
	return b.String()
}
