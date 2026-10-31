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
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

type nativeRubyQualificationCase struct {
	name, line, version string
	created             time.Time
	static              bool
}

func nativeRubyQualificationCases() []nativeRubyQualificationCase {
	var cases []nativeRubyQualificationCase
	for _, line := range nativeToolchains[nativeRubyRuntime].lines {
		cases = append(cases, nativeRubyQualificationCase{name: "ruby" + strings.ReplaceAll(line.line, ".", ""), line: line.line, version: line.version})
	}
	ruby, _ := nativeToolchains[nativeRubyRuntime].find("3.4")
	return append(cases,
		nativeRubyQualificationCase{name: "ruby344-creation-default", version: rubyDefaultVersion, created: rubyDefaultCutoff.Add(time.Second)},
		nativeRubyQualificationCase{name: "node-static-ruby34", line: ruby.line, version: ruby.version, static: true})
}

func nativeRubyQualificationFixture(t *testing.T, tc nativeRubyQualificationCase) (Options, map[string]string) {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join("testdata", "native-ruby-qualification"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name, err := filepath.Rel(filepath.Join("testdata", "native-ruby-qualification"), path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		files[name] = string(raw)
		return err
	})
	must(t, err)
	// Files select lines without overwriting the selected image's exact version
	// ENV during the authored command. The creation-default case has no Ruby signal.
	files[".node-version"] = "24\n"
	if tc.line != "" {
		files[".ruby-version"] = tc.line + "\n"
	}
	node, _ := nativeToolchains[nativeNodeRuntime].find("24")
	o := nativeOptions()
	o.Runtime = nativeRubyRuntime
	o.BuildCommand = fmt.Sprintf("ruby build.rb %s %s primary", tc.version, node.version)
	o.StartCommand = fmt.Sprintf("ruby check.rb %s %s primary runtime", tc.version, node.version)
	if tc.static {
		o.Runtime, o.StaticSite, o.StaticPublishPath = nativeNodeRuntime, true, "public"
		o.BuildCommand, o.StartCommand = fmt.Sprintf("node static.js %s %s", tc.version, node.version), ""
	}
	return o, files
}

// This separate profile exercises canonical recipes without a prerequisite
// overlay. Generic BuildKit integration invocations must skip it. It needs an
// explicitly owned existing builder/platform; it does not create a builder,
// publish, select production pins, or exercise Kubernetes/hosted execution.
func TestCommonRubyGeneratedImages(t *testing.T) {
	if os.Getenv("BEX_NATIVE_TEST_RUBY_PROFILE") != "1" {
		t.Skip("set BEX_NATIVE_TEST_RUBY_PROFILE=1 for the separate Ruby qualification profile")
	}
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") != "1" {
		t.Fatal("Ruby profile also requires BEX_BUILDKIT_INTEGRATION=1")
	}
	common, _, _ := nativeUtilityTestImages(t)
	if common == "" {
		t.Fatal("Ruby profile requires immutable BEX_NATIVE_TEST_COMMON_IMAGE")
	}
	docker := nativeUtilityDockerConfig(t)
	if docker.builder == "" {
		t.Fatal("Ruby profile requires an explicitly owned builder and platform")
	}
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		var err error
		output, err = os.MkdirTemp(requested, "native-ruby-qualification-")
		must(t, err)
	}
	t.Logf("Ruby qualification evidence: %s", output)
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = common
	for _, tc := range nativeRubyQualificationCases() {
		t.Run(tc.name, func(t *testing.T) {
			nativeRubyQualificationImage(t, docker, output, tc)
		})
	}
}

func nativeRubyQualificationImage(t *testing.T, docker nativeUtilityDocker, output string, tc nativeRubyQualificationCase) {
	t.Helper()
	o, files := nativeRubyQualificationFixture(t, tc)
	bundle, path, source := writeResolveFixture(t, o, nil, files)
	choices, err := ResolveNativeBuildExact(t.Context(), NewDockerHubResolver(), tc.created, o.Runtime, bundle, path, source)
	must(t, err)
	rubyIndex := 0
	if tc.static {
		rubyIndex = 1
	}
	if len(choices) != 2 || choices[rubyIndex].declined != "" {
		t.Fatalf("Ruby selection missing: %+v", choices)
	}
	evidence := filepath.Join(output, tc.name)
	must(t, os.Mkdir(evidence, 0o700))
	recipe, err := os.ReadFile(path)
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile.canonical"), recipe, 0o600))
	selection, err := json.MarshalIndent(choices, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "selection.json"), selection, 0o600))
	must(t, os.Rename(path, filepath.Join(source, "Dockerfile")))
	tag := fmt.Sprintf("bex-native-ruby-%s:%d-%d", tc.name, os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", docker.buildArgs([]string{"--secret", "id=render-env,src=" + bundle, "-t", tag}, source, true)...)
	log, buildErr := command.CombinedOutput()
	must(t, os.WriteFile(filepath.Join(evidence, "build.log"), log, 0o600))
	if buildErr != nil {
		t.Fatalf("canonical offline Ruby build: %v\n%s", buildErr, log)
	}
	docker.runImage(t, nativeUtilityCase{name: tc.name, static: tc.static}, tag, evidence, false)
	if tc.static {
		must(t, nativeRubyQualificationProof(filepath.Join(evidence, "public", "ruby-proof.json"), tc, "build"))
		node, _ := nativeToolchains[nativeNodeRuntime].find("24")
		page, err := os.ReadFile(filepath.Join(evidence, "public", "index.html"))
		must(t, err)
		if !strings.Contains(string(page), "Node "+node.version+" + Ruby "+tc.version+": 雪") {
			t.Fatalf("static Node output did not retain the selected companion: %s", page)
		}
		return
	}
	for _, phase := range []string{"build", "runtime"} {
		must(t, nativeRubyQualificationProof(filepath.Join(evidence, "qa-output", phase+".json"), tc, phase))
	}
}

type nativeRubyProof struct {
	Pass            bool              `json:"pass"`
	Ruby            string            `json:"ruby"`
	Node            string            `json:"node"`
	Mode            string            `json:"mode"`
	Phase           string            `json:"phase"`
	CompiledAPI     []int             `json:"compiled_api"`
	OpenSSL         string            `json:"openssl"`
	OpenSSLLibrary  string            `json:"openssl_library"`
	Bundler         string            `json:"bundler"`
	Env             map[string]string `json:"env"`
	Arch            string            `json:"arch"`
	NativeExtension bool              `json:"native_extension"`
	Stdlib          bool              `json:"stdlib"`
	SourceRoot      string            `json:"source_root"`
}

func nativeRubyQualificationProof(path string, tc nativeRubyQualificationCase, phase string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var proof nativeRubyProof
	if err := json.Unmarshal(raw, &proof); err != nil {
		return err
	}
	mode := "primary"
	node, _ := nativeToolchains[nativeNodeRuntime].find("24")
	versionKey, version := "RUBY_VERSION", tc.version
	if tc.static {
		mode = "static"
		versionKey, version = "NODE_VERSION", node.version
	}
	parts := strings.Split(tc.version, ".")
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	if !proof.Pass || proof.Ruby != tc.version || proof.Node != node.version || proof.Mode != mode || proof.Phase != phase ||
		majorErr != nil || minorErr != nil || !slices.Equal(proof.CompiledAPI, []int{major, minor}) ||
		!proof.NativeExtension || !proof.Stdlib || !strings.HasPrefix(proof.OpenSSL, "OpenSSL ") || !strings.HasPrefix(proof.OpenSSLLibrary, "OpenSSL ") ||
		!fullVersion.MatchString(proof.Bundler) || proof.Arch == "" || proof.SourceRoot != "/opt/render/project/src" {
		return fmt.Errorf("incomplete or incorrect Ruby %s %s proof: %s", tc.name, phase, raw)
	}
	if proof.Env["LANG"] != "C.UTF-8" || proof.Env["BUNDLE_PATH"] != nativeRubyGems || proof.Env["BUNDLE_BIN"] != nativeRubyGems+"/bin" || proof.Env[versionKey] != version {
		return fmt.Errorf("selected Ruby/Node or Bundler environment changed: %s", raw)
	}
	return nil
}

func TestCommonRubyQualificationCanonicalRecipes(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	cases := nativeRubyQualificationCases()
	if len(cases) != len(nativeToolchains[nativeRubyRuntime].lines)+2 {
		t.Fatal("profile must cover every reviewed Ruby line, creation default and static companion")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, files := nativeRubyQualificationFixture(t, tc)
			bundle, path, source := writeResolveFixture(t, o, nil, files)
			fake := &fakeExact{digests: map[string]string{}, envByImage: map[string]fakeExactMetadata{}}
			for _, runtime := range []string{nativeRubyRuntime, nativeNodeRuntime} {
				for _, line := range nativeToolchains[runtime].lines {
					fake.envByImage[line.image] = commonReviewedFixtureMetadata(runtime, line)
				}
			}
			var primary string
			if tc.line != "" {
				line, _ := nativeToolchains[nativeRubyRuntime].find(tc.line)
				primary = line.image
			} else {
				primary = "ruby:" + rubyDefaultVersion + "-bookworm@" + exactDigest
				fake.digests["ruby:"+rubyDefaultVersion+"-bookworm"] = exactDigest
				fake.envByImage[primary] = fakeExactMetadata{env: []string{"RUBY_VERSION=" + rubyDefaultVersion, "PATH=/usr/local/bin:/usr/bin"}, history: commonOTPSyntheticHistory("bookworm")}
			}
			choices, err := ResolveNativeBuildExact(context.Background(), fake, tc.created, o.Runtime, bundle, path, source)
			must(t, err)
			raw, err := os.ReadFile(path)
			must(t, err)
			assertRubyQualificationRecipe(t, tc, o, choices, primary, string(raw))
		})
	}
}

func assertRubyQualificationRecipe(t *testing.T, tc nativeRubyQualificationCase, o Options, choices []NativeChoice, ruby, recipe string) {
	t.Helper()
	node, _ := nativeToolchains[nativeNodeRuntime].find("24")
	if len(choices) != 2 || !strings.Contains(recipe, "\nFROM "+commonFixtureImage+"\n") || !strings.Contains(recipe, o.BuildCommand) {
		t.Fatalf("canonical selection/command lost: %+v\n%s", choices, recipe)
	}
	rubyAt := strings.Index(recipe, "COPY --link --from="+ruby+" /usr/local/ /usr/local/\n")
	commandAt := strings.LastIndex(recipe, o.BuildCommand)
	if rubyAt < 0 || commandAt <= rubyAt || !strings.Contains(recipe, "[ \"$(/usr/local/bin/ruby -e 'puts RUBY_VERSION')\" = "+tc.version+" ]") {
		t.Fatalf("selected Ruby tree/version check missing before authored command:\n%s", recipe)
	}
	primary, key, version := ruby, "RUBY_VERSION", tc.version
	if tc.static {
		primary, key, version = node.image, "NODE_VERSION", node.version
	}
	previous := -1
	for _, marker := range []string{"rm -rf /usr/local", "COPY --link --from=" + primary + " /usr/local/ /usr/local/\n", "RUN --network=none /sbin/ldconfig\n", "ENV " + key + "=\"" + version + "\"\n", "\nCOPY . .\n"} {
		at := strings.Index(recipe, marker)
		if at < 0 || at <= previous {
			t.Fatalf("selected primary/loader/ENV/source order lost %q:\n%s", marker, recipe)
		}
		previous = at
	}
	if tc.static {
		if choices[0].Image != node.image || choices[1].Image != ruby || strings.Contains(recipe, "\nCMD ") || strings.Contains(recipe, "BUNDLE_DEPLOYMENT=true") {
			t.Fatalf("static companion replaced Node or acquired compute defaults: %+v\n%s", choices, recipe)
		}
		return
	}
	if choices[0].Image != ruby || choices[1].Image != node.image || !strings.Contains(recipe, o.StartCommand) || !strings.Contains(recipe, "ENV BUNDLE_DEPLOYMENT=true BUNDLE_PATH="+nativeRubyGems) {
		t.Fatalf("primary Ruby defaults, Node companion or runtime command lost: %+v\n%s", choices, recipe)
	}
	if tc.line == "" && (choices[0].Line != rubyDefaultVersion || !strings.Contains(choices[0].Narration(), "(default)")) {
		t.Fatalf("creation default was replaced by a reviewed-line patch: %+v", choices)
	}
}

func TestCommonRubyQualificationProofRejectsWrongOrMissingControls(t *testing.T) {
	tc := nativeRubyQualificationCases()[0]
	node, _ := nativeToolchains[nativeNodeRuntime].find("24")
	base := nativeRubyProof{Pass: true, Ruby: tc.version, Node: node.version, Mode: "primary", Phase: "runtime", CompiledAPI: []int{3, 0},
		OpenSSL: "OpenSSL 1.1.1 test", OpenSSLLibrary: "OpenSSL 1.1.1 test", Bundler: "2.2.33", Arch: "test-linux", NativeExtension: true, Stdlib: true, SourceRoot: "/opt/render/project/src",
		Env: map[string]string{"LANG": "C.UTF-8", "BUNDLE_PATH": nativeRubyGems, "BUNDLE_BIN": nativeRubyGems + "/bin", "RUBY_VERSION": tc.version}}
	for name, change := range map[string]func(*nativeRubyProof){
		"healthy": func(*nativeRubyProof) {}, "wrong Ruby": func(p *nativeRubyProof) { p.Ruby = "3.4.4" },
		"healthy Bundler4": func(p *nativeRubyProof) { p.Bundler = "4.0.11" }, "no pass": func(p *nativeRubyProof) { p.Pass = false },
		"wrong Node": func(p *nativeRubyProof) { p.Node = "14.21.3" }, "wrong headers": func(p *nativeRubyProof) { p.CompiledAPI = []int{3, 1} },
		"no native import": func(p *nativeRubyProof) { p.NativeExtension = false }, "no stdlib": func(p *nativeRubyProof) { p.Stdlib = false },
		"wrong phase": func(p *nativeRubyProof) { p.Phase = "build" }, "wrong mode": func(p *nativeRubyProof) { p.Mode = "static" },
		"no OpenSSL": func(p *nativeRubyProof) { p.OpenSSL = "" }, "no Bundler": func(p *nativeRubyProof) { p.Bundler = "" },
		"no dynamic OpenSSL": func(p *nativeRubyProof) { p.OpenSSLLibrary = "" },
		"no platform":        func(p *nativeRubyProof) { p.Arch = "" }, "wrong source root": func(p *nativeRubyProof) { p.SourceRoot = "/app" },
		"wrong locale": func(p *nativeRubyProof) { p.Env["LANG"] = "C" }, "wrong version ENV": func(p *nativeRubyProof) { p.Env["RUBY_VERSION"] = "3.0" },
		"wrong gem path": func(p *nativeRubyProof) { p.Env["BUNDLE_PATH"] = "/tmp/gems" },
		"wrong bin path": func(p *nativeRubyProof) { p.Env["BUNDLE_BIN"] = "/tmp/bin" },
	} {
		t.Run(name, func(t *testing.T) {
			proof := base
			proof.Env = maps.Clone(base.Env)
			change(&proof)
			raw, err := json.Marshal(proof)
			must(t, err)
			path := filepath.Join(t.TempDir(), "proof.json")
			must(t, os.WriteFile(path, raw, 0o600))
			if err := nativeRubyQualificationProof(path, tc, "runtime"); (err == nil) != strings.HasPrefix(name, "healthy") {
				t.Fatalf("proof validation = %v", err)
			}
		})
	}
	static := nativeRubyQualificationCases()[len(nativeRubyQualificationCases())-1]
	for _, versionENV := range []string{node.version, "24"} {
		proof := base
		proof.Ruby, proof.Mode, proof.Phase, proof.CompiledAPI = static.version, "static", "build", []int{3, 4}
		proof.Env = maps.Clone(base.Env)
		delete(proof.Env, "RUBY_VERSION")
		proof.Env["NODE_VERSION"] = versionENV
		raw, err := json.Marshal(proof)
		must(t, err)
		path := filepath.Join(t.TempDir(), "static-proof.json")
		must(t, os.WriteFile(path, raw, 0o600))
		if err := nativeRubyQualificationProof(path, static, "build"); (err == nil) != (versionENV == node.version) {
			t.Fatalf("static proof must preserve exact Node ENV: %v", err)
		}
	}
}

func TestCommonRubyProfileSkipsGenericBuildkitOptIn(t *testing.T) {
	binary, err := os.Executable()
	must(t, err)
	for _, tc := range []struct{ name, profile, integration, want string }{
		{"generic integration skips", "", "1", "--- SKIP: TestCommonRubyGeneratedImages"},
		{"profile needs integration opt-in", "1", "", "Ruby profile also requires BEX_BUILDKIT_INTEGRATION=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), binary, "-test.run=^TestCommonRubyGeneratedImages$", "-test.v")
			for _, env := range os.Environ() {
				if !strings.HasPrefix(env, "BEX_NATIVE_TEST_RUBY_PROFILE=") && !strings.HasPrefix(env, "BEX_BUILDKIT_INTEGRATION=") {
					command.Env = append(command.Env, env)
				}
			}
			command.Env = append(command.Env, "BEX_NATIVE_TEST_RUBY_PROFILE="+tc.profile, "BEX_BUILDKIT_INTEGRATION="+tc.integration)
			output, err := command.CombinedOutput()
			if (err == nil) != (tc.profile == "") || !strings.Contains(string(output), tc.want) {
				t.Fatalf("profile guard result %v: %s", err, output)
			}
		})
	}
}
