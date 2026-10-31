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
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// Synthetic identity is restricted to source-only tests; no production pin.
const commonFixtureImage = "ghcr.io/bex-co/bex-cnb-run:common-fixture@" + otherDigest

func TestCommonFoundationPreservesDefaultAndExactSelections(t *testing.T) {
	original, originalSupplied := nativeCommonBase, nativeSuppliedBases
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases = original, originalSupplied })
	nativeCommonBase = commonFixtureImage
	// The common route cannot silently adopt an unrelated C derivative.
	nativeSuppliedBases = map[string]string{"unrelated": "invalid-C-candidate"}
	for _, tc := range []struct {
		name, runtime, line, version, tag string
		env                               map[string]string
		created                           time.Time
		missing                           bool
	}{
		{name: "node-default", runtime: nativeNodeRuntime, line: "24", version: "24.22.1"},
		{name: "python-explicit-line", runtime: nativePythonRuntime, line: "3.13", version: "3.13.18", env: map[string]string{"PYTHON_VERSION": "3.13"}},
		{name: "python-creation-default", runtime: nativePythonRuntime, line: "3.14.8", version: "3.14.8", tag: "python:3.14.8-bookworm", created: pythonCreationCutoff.Add(time.Second)},
		{name: "node-exact", runtime: nativeNodeRuntime, line: "24.10.0", version: "24.10.0", tag: "node:24.10.0-bookworm", env: map[string]string{"NODE_VERSION": "24.10.0", "BUN_VERSION": "1.3"}},
		{name: "python-exact", runtime: nativePythonRuntime, line: "3.13.1", version: "3.13.1", tag: "python:3.13.1-bookworm", env: map[string]string{"PYTHON_VERSION": "3.13.1"}},
		{name: "existing-exact-fallback", runtime: nativeNodeRuntime, line: "24", version: "24.22.1", tag: "node:24.10.0-bookworm", env: map[string]string{"NODE_VERSION": "24.10.0"}, missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = tc.runtime
			bundle, path, source := writeResolveFixture(t, o, tc.env, nil)
			fake := &fakeExact{digests: map[string]string{}}
			if tc.tag != "" && !tc.missing {
				fake.digests[tc.tag] = exactDigest
			}
			versionKey := "NODE_VERSION"
			if tc.runtime == nativePythonRuntime {
				versionKey = "PYTHON_VERSION"
			}
			fake.env = []string{versionKey + "=" + tc.version, "PATH=/usr/local/bin:/usr/bin", "LANG=C.UTF-8", `LITERAL=$HOME\path"quoted`}
			if tc.runtime == nativePythonRuntime {
				node, _ := nativeToolchains[nativeNodeRuntime].find("24")
				fake.envByImage = map[string]fakeExactMetadata{node.image: {env: []string{"NODE_VERSION=" + node.version, "PATH=/usr/local/bin:/usr/bin"}}}
			}
			choices, err := ResolveNativeBuildExact(context.Background(), fake, tc.created, tc.runtime, bundle, path, source)
			must(t, err)
			data, err := os.ReadFile(path)
			must(t, err)
			recipe := string(data)
			reviewed, err := ResolveNativeVersionAt(tc.created, tc.runtime, tc.env, files(nil))
			must(t, err)
			primary := reviewed.Image
			if tc.tag != "" && !tc.missing {
				primary = tc.tag + "@" + exactDigest
			}
			if choices[0].Image != primary || choices[0].Line != tc.line {
				t.Fatalf("common base changed selected primary: %+v", choices[0])
			}
			if !strings.Contains(recipe, "\nFROM "+commonFixtureImage+"\n") {
				t.Fatalf("not using the one common FROM:\n%s", recipe)
			}
			assertCommonPrimaryOrder(t, recipe, primary, versionKey, tc.version, tc.runtime == nativePythonRuntime)
			if !slices.Contains(fake.asked, "env "+primary) {
				t.Fatalf("default/exact source metadata not read: %v", fake.asked)
			}
			if tc.missing && !strings.Contains(choices[0].Narration(), "No official image "+tc.tag) {
				t.Fatalf("existing fallback explanation lost: %s", choices[0].Narration())
			}
			if strings.Contains(recipe, "apt-get") {
				t.Fatal("common tools are being installed during a tenant build")
			}
			for _, command := range []string{o.BuildCommand, o.StartCommand} {
				if !strings.Contains(recipe, command) {
					t.Fatalf("authored command changed: %s", command)
				}
			}
		})
	}
}

func assertCommonPrimaryOrder(t *testing.T, recipe, primary, key, version string, python bool) {
	t.Helper()
	markers := []string{"rm -rf /usr/local", "COPY --link --from=" + primary + " /usr/local/ /usr/local/\n", "RUN --network=none /sbin/ldconfig\n", "ENV " + key + "=\"" + version + "\"\n"}
	if python {
		markers = append(markers, "RUN --network=none /usr/bin/bex-native-python-tools\n")
	}
	previous := -1
	for _, marker := range markers {
		at := strings.Index(recipe, marker)
		if at < 0 || at <= previous {
			t.Fatalf("primary/loader/environment/packaging order lost %q:\n%s", marker, recipe)
		}
		previous = at
	}
	for _, marker := range []string{" /usr/local/bin/bun /usr/local/bin/bun", "/opt/bex/poetry", " /usr/local/bin/node /usr/local/bin/node", "\nCOPY . .\n"} {
		if at := strings.Index(recipe, marker); at >= 0 && at < previous {
			t.Fatalf("primary replacement erased a later layer %q:\n%s", marker, recipe)
		}
	}
	for _, marker := range []string{`primary_os="$VERSION_CODENAME"`, `ENV LITERAL="\$HOME\\path\"quoted"`, `ENV PATH="/usr/local/bin:/usr/bin"`, `ENV LANG="C.UTF-8"`} {
		if !strings.Contains(recipe, marker) {
			t.Fatalf("suite guard or exact ENV escaping lost %q:\n%s", marker, recipe)
		}
	}
}

func TestCommonMetadataFailureNeverSubstitutesAnotherRuntime(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	o := nativeOptions()
	o.Runtime = nativePythonRuntime
	bundle, path, source := writeResolveFixture(t, o, nil, nil)
	before, err := os.ReadFile(path)
	must(t, err)
	fake := &fakeExact{digests: map[string]string{"python:3.14.8-bookworm": exactDigest}, envErr: errors.New("metadata transport unavailable")}
	_, err = ResolveNativeBuildExact(context.Background(), fake, pythonCreationCutoff.Add(time.Second), o.Runtime, bundle, path, source)
	if err == nil || !strings.Contains(err.Error(), "read selected native primary environment") {
		t.Fatalf("metadata failure was hidden: %v", err)
	}
	after, readErr := os.ReadFile(path)
	must(t, readErr)
	if string(after) != string(before) {
		t.Fatal("failed metadata resolution wrote a downgraded or partial recipe")
	}
}

func TestCommonFoundationCoversEveryExistingReviewedChoice(t *testing.T) {
	original, artifacts := nativeCommonBase, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeRuntimeArtifacts = original, artifacts })
	nativeCommonBase = commonFixtureImage
	nativeRuntimeArtifacts = nil
	fake := &fakeExact{envByImage: map[string]fakeExactMetadata{}}
	for runtime, toolchain := range nativeToolchains {
		for _, line := range toolchain.lines {
			fake.envByImage[line.image] = commonReviewedFixtureMetadata(runtime, line)
		}
	}
	count := 0
	for runtime, toolchain := range nativeToolchains {
		for _, line := range toolchain.lines {
			count++
			t.Run(runtime+"/"+line.line, func(t *testing.T) {
				env, sourceFiles := map[string]string{}, map[string]string{}
				switch runtime {
				case nativeNodeRuntime:
					env["NODE_VERSION"] = line.line
				case nativePythonRuntime:
					env["PYTHON_VERSION"] = line.line
				case nativeElixirRuntime:
					env["ELIXIR_VERSION"] = line.line
				case nativeRubyRuntime:
					sourceFiles[".ruby-version"] = line.line
				}
				o := nativeOptions()
				o.Runtime = runtime
				bundle, path, source := writeResolveFixture(t, o, env, sourceFiles)
				choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, runtime, bundle, path, source)
				must(t, err)
				raw, err := os.ReadFile(path)
				must(t, err)
				recipe := string(raw)
				if choices[0].Image != line.image || choices[0].Line != line.line {
					t.Fatalf("existing selection changed: %+v", choices[0])
				}
				if !strings.Contains(recipe, "\nFROM "+commonFixtureImage+"\n") || strings.Contains(recipe, "apt-get") {
					t.Fatalf("not one common final base without tenant apt:\n%s", recipe)
				}
				if strings.Contains(line.image, "-bullseye@") && !strings.Contains(recipe, `[ "$primary_os" = bullseye ] && [ "$VERSION_CODENAME" = bookworm ]`) {
					t.Fatalf("legacy tree has no bounded compatibility route:\n%s", recipe)
				}
				if runtime == "rust" && (!strings.Contains(recipe, `ENV CARGO_HOME="/usr/local/cargo"`) || !strings.Contains(recipe, `ENV RUSTUP_HOME="/usr/local/rustup"`)) {
					t.Fatal("Rust installation roots lost")
				}
				if runtime == "go" && !strings.Contains(recipe, `ENV GOTOOLCHAIN="local"`) {
					t.Fatal("Go toolchain policy lost")
				}
				if nativeJSCompanionRuntimes[runtime] && !strings.Contains(recipe, " /usr/local/include/node /usr/local/include/node\n") {
					t.Fatal("Node companion matching headers missing")
				}
			})
		}
	}
	if count != 27 {
		t.Fatalf("reviewed matrix changed; qualify all %d choices", count)
	}
}

func TestCommonRuntimeArtifactsNeverChangeExactIdentity(t *testing.T) {
	original, artifacts := nativeCommonBase, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeRuntimeArtifacts = original, artifacts })
	nativeCommonBase = commonFixtureImage
	line, _ := nativeToolchains[nativeNodeRuntime].find("14")
	tree := "ghcr.io/bex-co/bex-cnb-run@" + otherDigest
	nativeRuntimeArtifacts = map[string]string{line.image: tree}
	for _, exact := range []bool{false, true} {
		t.Run(map[bool]string{true: "other-exact", false: "approved-tree"}[exact], func(t *testing.T) {
			version := "14.21.3"
			primary := line.image
			fake := &fakeExact{digests: map[string]string{}}
			env := map[string]string{"NODE_VERSION": "14"}
			if exact {
				version = "14.21.2"
				env["NODE_VERSION"] = version
				fake.digests["node:14.21.2-bullseye"] = exactDigest
				primary = "node:14.21.2-bullseye@" + exactDigest
			}
			fake.env = []string{"NODE_VERSION=" + version, "PATH=/usr/local/bin:/usr/bin"}
			o := nativeOptions()
			bundle, path, source := writeResolveFixture(t, o, env, nil)
			choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, o.Runtime, bundle, path, source)
			must(t, err)
			raw, err := os.ReadFile(path)
			must(t, err)
			recipe := string(raw)
			donor := tree
			if exact {
				donor = "bex-node-bookworm"
				if !strings.Contains(recipe, "COPY --link --from="+primary+" /usr/local/ /usr/local/\n") {
					t.Fatal("exact source was not retained in its same-version repair stage")
				}
			}
			if choices[0].Image != primary || !strings.Contains(recipe, "COPY --link --from="+donor+" /usr/local/ /usr/local/\n") || strings.Contains(recipe, "\nFROM "+tree+"\n") {
				t.Fatalf("runtime tree changed exact identity or adopted C base:\n%s", recipe)
			}
			if !strings.Contains(recipe, " = "+version+" ]\n") {
				t.Fatal("actual installed runtime version is not checked")
			}
		})
	}
}

func TestCommonExactCompanionsPreservePatchAndHeaders(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	o := nativeOptions()
	o.Runtime = nativePythonRuntime
	python, _ := nativeToolchains[nativePythonRuntime].find("3.13")
	node := "node:14.21.2-bullseye@" + exactDigest
	fake := &fakeExact{digests: map[string]string{"node:14.21.2-bullseye": exactDigest}, envByImage: map[string]fakeExactMetadata{
		python.image: {env: []string{"PYTHON_VERSION=" + python.version, "PATH=/usr/local/bin:/usr/bin"}},
		node:         {env: []string{"NODE_VERSION=14.21.2", "PATH=/usr/local/bin:/usr/bin"}},
	}}
	bundle, path, source := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": "3.13", "NODE_VERSION": "14.21.2"}, nil)
	choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, o.Runtime, bundle, path, source)
	must(t, err)
	raw, err := os.ReadFile(path)
	must(t, err)
	recipe := string(raw)
	if len(choices) < 3 || choices[2].Image != node || !strings.Contains(recipe, "COPY --link --from=bex-node-bookworm /usr/local/include/node /usr/local/include/node\n") || !strings.Contains(recipe, "COPY --link --from="+node+" /usr/local/ /usr/local/\n") {
		t.Fatalf("companion exact patch or headers changed: %+v\n%s", choices, recipe)
	}
}

func commonReviewedFixtureMetadata(runtime string, line nativeLine) fakeExactMetadata {
	env := []string{"PATH=/usr/local/bin:/usr/bin"}
	switch runtime {
	case nativeNodeRuntime:
		env = append(env, "NODE_VERSION="+line.version)
	case nativePythonRuntime:
		env = append(env, "PYTHON_VERSION="+line.version)
	case nativeRubyRuntime:
		env = append(env, "RUBY_VERSION="+line.version)
	case "go":
		env = append(env, "GOTOOLCHAIN=local")
	case "rust":
		env = append(env, "CARGO_HOME=/usr/local/cargo", "RUSTUP_HOME=/usr/local/rustup")
	case nativeElixirRuntime:
		otp := "28.5.0.8"
		if line.otp == "27" {
			otp = "27.3.4.8"
		}
		if line.otp == "29" {
			otp = "29.1.2"
		}
		env = append(env, "OTP_VERSION="+otp, "ELIXIR_VERSION=v"+line.version, "REBAR_VERSION=2.6.4", "REBAR3_VERSION=3.27.0")
	}
	suite := line.debian
	if suite == "" {
		suite = "bookworm"
	}
	if strings.Contains(line.image, "-bullseye@") {
		suite = "bullseye"
	}
	return fakeExactMetadata{env: env, history: commonOTPSyntheticHistory(suite)}
}

func TestExactNodeSourceMetadataIsBoundedAndVersionSpecific(t *testing.T) {
	checksum := strings.Repeat("c", 64)
	for name, body := range map[string]string{
		"good":      checksum + "  node-v14.21.2.tar.xz\n" + strings.Repeat("d", 64) + "  node-v14.21.2-linux-x64.tar.xz\n",
		"newer":     checksum + "  node-v14.21.3.tar.xz\n",
		"invalid":   "not-a-sha node-v14.21.2.tar.xz\n",
		"duplicate": strings.Repeat(checksum+" node-v14.21.2.tar.xz\n", 2),
		"oversize":  checksum + " node-v14.21.2.tar.xz\n" + strings.Repeat("x", maxTokenBytes),
	} {
		t.Run(name, func(t *testing.T) {
			requested := ""
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requested = r.URL.Path; _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			registry := &DockerHubResolver{Client: server.Client(), NodeDistURL: server.URL}
			got, err := registry.NodeSourceDigest(context.Background(), "14.21.2")
			if requested != "/v14.21.2/SHASUMS256.txt" {
				t.Fatalf("source request escaped exact release: %s", requested)
			}
			if name == "good" {
				if err != nil || got != checksum {
					t.Fatalf("valid exact source %q %v", got, err)
				}
			} else if err == nil {
				t.Fatal("invalid or wrong-version metadata accepted")
			}
		})
	}
}

func TestCommonNodeBuildPythonNeverReachesRuntimeTree(t *testing.T) {
	original, artifacts := nativeCommonBase, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeRuntimeArtifacts = original, artifacts })
	nativeCommonBase = commonFixtureImage
	nativeRuntimeArtifacts = nil
	python, _ := nativeToolchains[nativePythonRuntime].find("3.9")
	for _, version := range []string{"14.21.3", "14.21.2"} {
		t.Run(version, func(t *testing.T) {
			fake := &fakeExact{digests: map[string]string{"node:" + version + "-bullseye": exactDigest}, env: []string{"NODE_VERSION=" + version, "PATH=/usr/local/bin:/usr/bin"}}
			o := nativeOptions()
			bundle, path, source := writeResolveFixture(t, o, map[string]string{"NODE_VERSION": version}, nil)
			_, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, nativeNodeRuntime, bundle, path, source)
			must(t, err)
			raw, err := os.ReadFile(path)
			must(t, err)
			recipe := string(raw)
			buildAt := strings.Index(recipe, "FROM "+commonFixtureImage+" AS bex-node-build\n")
			exportAt := strings.Index(recipe, "FROM scratch AS bex-node-bookworm\n")
			finalAt := strings.Index(recipe, "\nFROM "+commonFixtureImage+"\n")
			if buildAt < 0 || exportAt < buildAt || finalAt < exportAt {
				t.Fatalf("build-only/runtime export boundary missing:\n%s", recipe)
			}
			builder, exported := recipe[buildAt:exportAt], recipe[exportAt:finalAt]
			for _, want := range []string{"COPY --link --from=" + python.image + " /usr/local/ /usr/local/\n", "RUN /sbin/ldconfig && /usr/local/bin/python3.9 -c", "build-node --common-bookworm " + version} {
				if !strings.Contains(builder, want) {
					t.Fatalf("compatible build interpreter missing %q:\n%s", want, builder)
				}
			}
			if strings.Contains(exported, "python") || strings.Contains(exported, "--from=bex-node-build /usr/local/ /usr/local/") {
				t.Fatalf("build interpreter leaked into exported runtime:\n%s", exported)
			}
			if !strings.Contains(exported, "COPY --link --from=bex-node-build /usr/local/bin/node /usr/local/bin/node\n") {
				t.Fatal("repaired executable missing from runtime export")
			}
		})
	}
}

func TestCommonNodeBuilderRefusesSystemPythonSubstitution(t *testing.T) {
	original := nativeToolchains[nativePythonRuntime]
	t.Cleanup(func() { nativeToolchains[nativePythonRuntime] = original })
	changed := original
	changed.lines = append([]nativeLine(nil), original.lines...)
	for i := range changed.lines {
		if changed.lines[i].line == "3.9" {
			changed.lines[i].version = "3.11.0"
		}
	}
	nativeToolchains[nativePythonRuntime] = changed
	if _, err := commonNodeBuildStage("node:14-bullseye@"+exactDigest, "14.21.3", strings.Repeat("b", 64)); err == nil {
		t.Fatal("incompatible build Python silently replaced reviewed3.9")
	}
}
