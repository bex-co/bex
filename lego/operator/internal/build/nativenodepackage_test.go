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

package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func enginesNode(rng string) string { return `{"engines":{"node":"` + rng + `"}}` }

// w8/399: with no package.json at the root, Render's Node "uses the first
// package.json file it finds in a subdirectory" (render.com/docs/node-version).
// bex's order is the shallowest first, each directory's entries lexically.
func TestNodePackageJSONSearch(t *testing.T) {
	for name, tc := range map[string]struct {
		files map[string]string
		links map[string]string // name → target, relative to the root
		want  string
	}{
		"none":                         {want: ""},
		"root":                         {files: map[string]string{"package.json": `{"name":"x"}`, "backend/package.json": "{}"}, want: "package.json"},
		"one subdirectory":             {files: map[string]string{"backend/package.json": "{}"}, want: "backend/package.json"},
		"lexical among siblings":       {files: map[string]string{"frontend/package.json": "{}", "backend/package.json": "{}"}, want: "backend/package.json"},
		"shallowest first":             {files: map[string]string{"a/b/package.json": "{}", "z/package.json": "{}"}, want: "z/package.json"},
		"three levels down":            {files: map[string]string{"a/b/c/package.json": "{}"}, want: "a/b/c/package.json"},
		"past the depth bound":         {files: map[string]string{"a/b/c/d/package.json": "{}"}, want: ""},
		"node_modules never":           {files: map[string]string{"node_modules/x/package.json": "{}"}, want: ""},
		"dot-directories never":        {files: map[string]string{".github/package.json": "{}", ".git/x/package.json": "{}"}, want: ""},
		"under node_modules' siblings": {files: map[string]string{"node_modules/package.json": "{}", "web/package.json": "{}"}, want: "web/package.json"},
		"no file link":                 {files: map[string]string{"b/package.json": "{}", "target.json": "{}"}, links: map[string]string{"a/package.json": "../target.json"}, want: "b/package.json"},
		"a package.json directory":     {files: map[string]string{"a/package.json/x": "", "b/package.json": "{}"}, want: "b/package.json"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tc.files {
				must(t, os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755))
				must(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
			}
			for name, target := range tc.links {
				must(t, os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755))
				must(t, os.Symlink(target, filepath.Join(root, name)))
			}
			if got := nodePackageJSON(root); got != tc.want {
				t.Errorf("nodePackageJSON = %q, want %q", got, tc.want)
			}
		})
	}
}

// A directory link, here out of the checkout, is never searched.
func TestNodePackageJSONSearchNeverFollowsADirectoryLink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	must(t, os.WriteFile(filepath.Join(outside, "package.json"), []byte(enginesNode(">=26")), 0o644))
	must(t, os.Symlink(outside, filepath.Join(root, "app")))
	if got := nodePackageJSON(root); got != "" {
		t.Errorf("nodePackageJSON = %q through a link, want none", got)
	}
}

// The package.json the search finds is the Node signal, its request line and
// the path the log names, for a Node service and for another runtime's Node
// companion alike; a root .nvmrc, a root package.json without engines and a
// node_modules one keep their meaning.
func TestNodeReadsTheFirstSubdirectoryPackageJSON(t *testing.T) {
	newest, _ := nativeToolchains[nativeNodeRuntime].find("26")
	twentyTwo, _ := nativeToolchains[nativeNodeRuntime].find("22")
	for name, tc := range map[string]struct {
		runtime string
		files   map[string]string
		source  string
		line    string
		want    string // the Node choice's narration, when checked
	}{
		// render-examples/docs-api-agent-template@5f73445: no root package.json.
		"monorepo": {runtime: "node", files: map[string]string{
			"backend/package.json": enginesNode(">=22.0.0"), "frontend/package.json": enginesNode(">=22.0.0"),
		}, source: "backend/package.json engines.node", line: "26",
			want: "==> Requesting Node.js version >=22.0.0\n==> Using Node.js version " + newest.version + " via /opt/render/project/src/backend/package.json" + nodeDocs},
		"python companion": {runtime: "python", files: map[string]string{"web/package.json": enginesNode("^22")},
			source: "web/package.json engines.node", line: "22",
			want: "==> Requesting Node.js version ^22\n==> Using Node.js version " + twentyTwo.version + " via /opt/render/project/src/web/package.json" + nodeDocs},
		"root .nvmrc wins": {runtime: "node", files: map[string]string{".nvmrc": "22\n", "backend/package.json": enginesNode(">=26")},
			source: ".nvmrc", line: "22"},
		"root package.json without engines": {runtime: "node", files: map[string]string{"package.json": `{"name":"x"}`, "backend/package.json": enginesNode(">=26")},
			source: "", line: "24"},
		"first one without engines": {runtime: "node", files: map[string]string{"api/package.json": `{"name":"api"}`, "web/package.json": enginesNode(">=26")},
			source: "", line: "24"},
		"node_modules": {runtime: "node", files: map[string]string{"node_modules/x/package.json": enginesNode(">=26")},
			source: "", line: "24"},
	} {
		t.Run(name, func(t *testing.T) {
			choices, _ := resolveExact(t, &fakeExact{}, tc.runtime, nil, tc.files)
			var node NativeChoice
			for _, c := range choices {
				if c.Label == "Node" {
					node = c
				}
			}
			if node.Source != tc.source || node.Line != tc.line {
				t.Fatalf("Node choice = %+v; want line %s from %q", node, tc.line, tc.source)
			}
			if tc.want != "" && node.Narration() != tc.want {
				t.Errorf("narration = %q, want %q", node.Narration(), tc.want)
			}
		})
	}
}

// A refused request names the file it came from.
func TestNodeSubdirectoryPackageJSONRefusalNamesIt(t *testing.T) {
	o := nativeOptions()
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, map[string]string{"backend/package.json": enginesNode("<14")})
	_, err := ResolveNativeBuildExact(context.Background(), &fakeExact{}, time.Time{}, nativeNodeRuntime, bundle, dockerfile, src)
	var unsupported *UnsupportedVersionError
	if !errors.As(err, &unsupported) || !strings.HasPrefix(unsupported.Error(), `backend/package.json engines.node "<14" is not a supported Node version`) {
		t.Fatalf("err = %v, want a refusal naming backend/package.json", err)
	}
}

// w8/400: another runtime's Node never fails its build on a request bex cannot
// install. Render installs any Node release, 12.18.3 included (community
// thread 15142), so a Python service with a stray docs/package.json asking for
// 12.x builds there; bex builds it on the dated default, with a line naming
// the request, whatever the signal. A Node service still refuses it (w8/398).
func TestNodeCompanionFallsBackOnARequestBexCannotInstall(t *testing.T) {
	const image = "node:24.22.1-bookworm@" + exactDigest
	created := renderNodeDefault20261021.Add(time.Hour)
	common := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = common })
	for name, tc := range map[string]struct{ env, files map[string]string }{
		"nested package.json":   {files: map[string]string{"app.py": "", "docs/package.json": enginesNode("12.x")}},
		"root package.json":     {files: map[string]string{"package.json": enginesNode("<14")}},
		"NODE_VERSION":          {env: map[string]string{"NODE_VERSION": "12.22.12"}},
		"release with no image": {files: map[string]string{".nvmrc": "99.99.99\n"}},
	} {
		fake := &fakeExact{digests: map[string]string{"node:24.22.1-bookworm": exactDigest}}
		o := nativeOptions()
		o.Runtime = nativeNodeRuntime
		bundle, dockerfile, src := writeResolveFixture(t, o, tc.env, tc.files)
		_, refusal := ResolveNativeBuildExact(context.Background(), fake, created, nativeNodeRuntime, bundle, dockerfile, src)
		if _, tenant := errors.AsType[*UnsupportedVersionError](refusal); !tenant {
			t.Fatalf("%s: a Node service resolved with err %v, want the w8/398 refusal", name, refusal)
		}
		want := "==> Using Node.js version 24.22.1 (default)" + nodeDocs + "\n==> " + refusal.Error() + "; using the default Node.js version instead"
		for _, base := range []string{common, ""} {
			nativeCommonBase = base
			for runtime := range nativeJSCompanionRuntimes {
				t.Run(name+"/"+runtime+"/common base "+boolName(base != ""), func(t *testing.T) {
					choices, out := resolveExactAt(t, fake, created, runtime, tc.env, tc.files)
					i := slices.IndexFunc(choices, func(c NativeChoice) bool { return c.Label == "Node" })
					if i < 0 || choices[i].Image != image || choices[i].Source != "" || choices[i].Narration() != want || !strings.Contains(out, "COPY --link --from="+image+" ") {
						t.Fatalf("choices %+v; want the default Node with %q\n%s", choices, want, out)
					}
				})
			}
		}
	}
}

// An unreadable registry is still a platform error for the companion: no
// fallback hides it.
func TestNodeCompanionRegistryErrorStillFails(t *testing.T) {
	o := nativeOptions()
	o.Runtime = nativePythonRuntime
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"NODE_VERSION": "25.8.1"}, nil)
	_, err := ResolveNativeBuildExact(context.Background(), &fakeExact{err: errors.New("registry down")}, time.Time{}, nativePythonRuntime, bundle, dockerfile, src)
	if _, tenant := errors.AsType[*UnsupportedVersionError](err); err == nil || tenant || !strings.Contains(err.Error(), "registry down") {
		t.Fatalf("err = %v, want the platform error", err)
	}
}
