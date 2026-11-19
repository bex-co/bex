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

package main_test

// ADR113 D7, coverage contract rule 3: a Bex-native command that replaces the
// process cannot ship unreported. Upstream's post-run hook never runs for a
// replaced process, so the command must call telemetry.EmitLaunch just before
// the replacement, and a test against the built binary must prove that one
// event is stored.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// processReplacers is the reviewed list of the module's process-replacing call
// sites, keyed by "<file> <enclosing function>", each with the binary-level
// tests (telemetry_contract_test.go, main_test.go) that expect exactly one
// stored event from the commands that reach it.
var processReplacers = map[string][]string{
	// bex code's provider launchers: bex glm, muse, kimi, deepseek.
	"internal/code/launch.go execClaude": {"TestBexProviderLaunchEmitsExactlyOneEvent"},
	// The desktop launcher, which bex edit, bex <path> and bex agent open reach.
	"internal/desktop/desktop.go osSystem": {"TestBexEditLaunchEmitsExactlyOneEvent", "TestBexAgentOpenLaunchEmitsExactlyOneEvent"},
	// bex agent ssh.
	"internal/agent/ssh.go execSystemSSH": {"TestBexAgentSSHLaunchEmitsExactlyOneEvent"},
	// bex s3 and bex s3api, which run the AWS CLI against bex object storage.
	"internal/s3/launch.go execAWS": {"TestBexS3LaunchEmitsExactlyOneEvent"},
}

// replacingPackages are the import paths whose Exec replaces the process.
var replacingPackages = map[string]bool{"syscall": true, "golang.org/x/sys/unix": true}

// replacingSite is one reference to a process-replacing Exec.
type replacingSite struct {
	key  string // "<file> <enclosing function>", the processReplacers key
	line int
}

func TestEveryProcessReplacingCallSiteIsReviewed(t *testing.T) {
	sites, err := processReplacingSites(".")
	if err != nil {
		t.Fatal(err)
	}
	tests, err := testFunctions(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range coverageProblems(sites, processReplacers, tests) {
		t.Error(problem)
	}
}

// TestCoverageScanFindsAnUnreviewedExec plants the call sites the contract
// exists to catch, in a new file of a scratch module, and checks that each is
// reported with what to add.
func TestCoverageScanFindsAnUnreviewedExec(t *testing.T) {
	root := t.TempDir()
	write := func(path, source string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/newcmd/run.go", `package newcmd

import (
	sys "syscall"

	"golang.org/x/sys/unix"
)

func handOff(path string) error { return sys.Exec(path, nil, nil) }

var replace = unix.Exec

func notAnExec(sys struct{ Exec func() }) { sys.Exec() }
`)
	// Ignored: a test file, and testdata.
	write("internal/newcmd/run_test.go", "package newcmd\n\nimport \"syscall\"\n\nvar _ = syscall.Exec\n")
	write("internal/newcmd/testdata/fixture.go", "package fixture\n\nimport \"syscall\"\n\nvar _ = syscall.Exec\n")

	sites, err := processReplacingSites(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []replacingSite{{"internal/newcmd/run.go handOff", 9}, {"internal/newcmd/run.go", 11}}
	if !slices.Equal(sites, want) {
		t.Fatalf("sites = %v, want %v", sites, want)
	}

	problems := coverageProblems(sites, map[string][]string{}, nil)
	if len(problems) != 2 {
		t.Fatalf("problems = %q, want one per site", problems)
	}
	for _, problem := range problems {
		for _, says := range []string{"internal/newcmd/run.go:", "telemetry.EmitLaunch", "exactly one stored event", "processReplacers"} {
			if !strings.Contains(problem, says) {
				t.Errorf("problem %q does not say %q", problem, says)
			}
		}
	}
}

// TestCoverageScanRefusesADotImport: `import . "syscall"` would turn the call
// into a bare Exec the scan cannot attribute, so the file is refused instead.
func TestCoverageScanRefusesADotImport(t *testing.T) {
	root := t.TempDir()
	source := "package hidden\n\nimport . \"syscall\"\n\nfunc run(path string) error { return Exec(path, nil, nil) }\n"
	if err := os.WriteFile(filepath.Join(root, "hidden.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := processReplacingSites(root)
	if err == nil || !strings.Contains(err.Error(), "hidden.go dot-imports syscall") {
		t.Fatalf("err = %v, want the dot import refused", err)
	}
}

// TestCoverageListCannotGoStale: an entry whose call site is gone, or that
// names no test or a test that does not exist, is a failure too, so the list
// stays a reviewed record of the code.
func TestCoverageListCannotGoStale(t *testing.T) {
	sites := []replacingSite{{"internal/a.go run", 3}, {"internal/b.go run", 5}}
	reviewed := map[string][]string{
		"internal/a.go run":  {"TestARuns"},
		"internal/b.go run":  nil,
		"internal/gone.go x": {"TestGone"},
	}
	problems := coverageProblems(sites, reviewed, []string{"TestARuns", "TestGone"})
	want := []string{
		"internal/b.go run names no contract test in processReplacers",
		"internal/gone.go x is listed in processReplacers but no longer replaces the process: remove the entry",
	}
	if !slices.Equal(problems, want) {
		t.Errorf("problems = %q, want %q", problems, want)
	}
	problems = coverageProblems(sites[:1], map[string][]string{"internal/a.go run": {"TestMissing"}}, []string{"TestARuns"})
	if len(problems) != 1 || !strings.Contains(problems[0], "TestMissing, which does not exist") {
		t.Errorf("problems = %q, want the missing test named", problems)
	}
}

// coverageProblems compares the call sites found with the reviewed list and
// the test functions that exist.
func coverageProblems(sites []replacingSite, reviewed map[string][]string, tests []string) []string {
	var problems []string
	found := map[string]bool{}
	for _, site := range sites {
		found[site.key] = true
		if _, ok := reviewed[site.key]; ok {
			continue
		}
		file, _, _ := strings.Cut(site.key, " ")
		problems = append(problems, fmt.Sprintf(
			"%s:%d replaces the process, so upstream's telemetry hook never reports the command (ADR113 D7). "+
				"Call telemetry.EmitLaunch(c.CommandPath(), startedAt, c.ErrOrStderr()) immediately before it, "+
				"add a test against the built binary that expects exactly one stored event "+
				"(see TestBexAgentSSHLaunchEmitsExactlyOneEvent), and list %q with that test in processReplacers",
			file, site.line, site.key))
	}
	for _, key := range slices.Sorted(maps.Keys(reviewed)) {
		if !found[key] {
			problems = append(problems, fmt.Sprintf("%s is listed in processReplacers but no longer replaces the process: remove the entry", key))
			continue
		}
		if len(reviewed[key]) == 0 {
			problems = append(problems, fmt.Sprintf("%s names no contract test in processReplacers", key))
		}
		for _, test := range reviewed[key] {
			if !slices.Contains(tests, test) {
				problems = append(problems, fmt.Sprintf("%s names %s, which does not exist in package main_test", key, test))
			}
		}
	}
	return problems
}

// processReplacingSites finds every reference to syscall.Exec or unix.Exec in
// the non-test Go files under root, testdata excluded. A reference, not only a
// call: a function value passed on replaces the process just the same.
func processReplacingSites(root string) ([]replacingSite, error) {
	var sites []replacingSite
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); path != root && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found, err := fileReplacingSites(path, filepath.ToSlash(rel))
		sites = append(sites, found...)
		return err
	})
	return sites, err
}

func fileReplacingSites(path, rel string) ([]replacingSite, error) {
	fset := token.NewFileSet()
	// Object resolution stays on: an identifier the file declares itself (a
	// parameter or variable named syscall) resolves to that declaration, and
	// only one left unresolved can name the import.
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, spec := range file.Imports {
		importPath := strings.Trim(spec.Path.Value, `"`)
		if !replacingPackages[importPath] {
			continue
		}
		name := importPath[strings.LastIndex(importPath, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "." {
			return nil, fmt.Errorf("%s dot-imports %s, which hides its Exec from the telemetry coverage scan", rel, importPath)
		}
		names[name] = true
	}
	if len(names) == 0 {
		return nil, nil
	}
	var sites []replacingSite
	for _, decl := range file.Decls {
		key := rel
		if fn, ok := decl.(*ast.FuncDecl); ok {
			key += " " + fn.Name.Name
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Exec" {
				return true
			}
			if pkg, ok := selector.X.(*ast.Ident); ok && names[pkg.Name] && pkg.Obj == nil {
				sites = append(sites, replacingSite{key, fset.Position(selector.Pos()).Line})
			}
			return true
		})
	}
	return sites, nil
}

// testFunctions are the Test functions of package main_test in dir.
func testFunctions(dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, err
	}
	var tests []string
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
				tests = append(tests, fn.Name.Name)
			}
		}
	}
	return tests, nil
}
