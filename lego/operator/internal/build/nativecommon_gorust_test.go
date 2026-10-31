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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type nativeGoRustPrimary struct {
	Runtime string   `json:"runtime"`
	Line    string   `json:"line"`
	Image   string   `json:"image"`
	Version string   `json:"version"`
	Env     []string `json:"env"`
}

func nativeGoRustProfileEnabled() bool { return os.Getenv("BEX_NATIVE_TEST_GORUST_PROFILE") == "1" }

func nativeGoRustPrimaryContract(runtime string, choice NativeChoice) (nativeGoRustPrimary, error) {
	toolchain, found := nativeToolchains[runtime]
	if !found || (runtime != "go" && runtime != "rust") || len(toolchain.lines) != 1 || choice.Line != toolchain.defaultLine {
		return nativeGoRustPrimary{}, fmt.Errorf("Go/Rust profile requires its single reviewed line")
	}
	if err := validateNativeBase(choice.Image); err != nil {
		return nativeGoRustPrimary{}, err
	}
	key, required, binaryPath := "GOLANG_VERSION", map[string]string{"GOTOOLCHAIN": "local", "GOPATH": "/go"}, "/usr/local/go/bin"
	if runtime == "rust" {
		key, required, binaryPath = "RUST_VERSION", map[string]string{"CARGO_HOME": "/usr/local/cargo", "RUSTUP_HOME": "/usr/local/rustup"}, "/usr/local/cargo/bin"
	}
	environment := map[string]string{}
	for _, item := range choice.imageEnv {
		name, value, ok := strings.Cut(item, "=")
		if _, duplicate := environment[name]; !ok || !nativeImageEnvName.MatchString(name) || duplicate {
			return nativeGoRustPrimary{}, fmt.Errorf("invalid selected primary environment")
		}
		environment[name] = value
	}
	version := environment[key]
	if !fullVersion.MatchString(version) || !strings.HasPrefix(version, choice.Line+".") || !slices.Contains(strings.Split(environment["PATH"], ":"), binaryPath) {
		return nativeGoRustPrimary{}, fmt.Errorf("selected primary version/PATH does not match its reviewed line")
	}
	for name, value := range required {
		if environment[name] != value {
			return nativeGoRustPrimary{}, fmt.Errorf("selected primary %s changed", name)
		}
	}
	return nativeGoRustPrimary{runtime, choice.Line, choice.Image, version, append([]string(nil), choice.imageEnv...)}, nil
}

func nativeGoRustFiles(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("testdata", "native-common-gorust")
	files := map[string]string{}
	must(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("qualification fixture must not contain symlinks")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err == nil {
			files[relative] = string(data)
		}
		return err
	}))
	return files
}

func nativeGoRustTreeRecipe(canonical, primary, runtime string) (string, error) {
	if err := validateNativeBase(primary); err != nil {
		return "", err
	}
	const boundary = "\nCOPY . .\n"
	if (runtime != "go" && runtime != "rust") || strings.Contains(primary, ",") || strings.Count(canonical, boundary) != 1 || !strings.Contains(canonical, "COPY --link --from="+primary+" /usr/local/ /usr/local/\n") {
		return "", fmt.Errorf("primary tree proof requires one canonical source boundary and selected donor")
	}
	probe := "RUN --network=none --mount=type=bind,from=" + primary + ",source=/usr/local,target=/opt/bex-test-primary,readonly /usr/bin/python3 check.py tree " + runtime + " /opt/bex-test-primary\n"
	return strings.Replace(canonical, boundary, boundary+probe, 1), nil
}

func TestNativeGoRustGeneratedImages(t *testing.T) {
	if !nativeGoRustProfileEnabled() {
		t.Skip("separate Go/Rust profile requires BEX_NATIVE_TEST_GORUST_PROFILE=1")
	}
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") != "1" {
		t.Fatal("Go/Rust profile also requires BEX_BUILDKIT_INTEGRATION=1")
	}
	common, _, _ := nativeUtilityTestImages(t)
	docker := nativeUtilityDockerConfig(t)
	if common == "" || docker.builder == "" {
		t.Fatal("Go/Rust profile requires an immutable common image and explicitly owned builder/platform")
	}
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = common
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		var err error
		output, err = os.MkdirTemp(requested, "native-common-gorust-")
		must(t, err)
	}
	t.Logf("retained Go/Rust profile evidence: %s", output)
	for _, runtime := range []string{"go", "rust"} {
		if !t.Run(runtime, func(t *testing.T) { nativeGoRustGeneratedCase(t, docker, common, runtime, output) }) {
			break
		}
	}
}

func nativeGoRustGeneratedCase(t *testing.T, docker nativeUtilityDocker, common, runtime, output string) {
	t.Helper()
	evidence := filepath.Join(output, runtime)
	must(t, os.Mkdir(evidence, 0o700))
	o := nativeOptions()
	o.Runtime, o.BuildEnv, o.RuntimeEnvSecret = runtime, nil, ""
	o.BuildCommand, o.StartCommand = "/usr/bin/python3 check.py build "+runtime, "/usr/bin/python3 check.py runtime "+runtime
	files := nativeGoRustFiles(t)
	bundle, path, source := writeResolveFixture(t, o, nil, files)
	choices, err := ResolveNativeBuildAt(time.Time{}, runtime, bundle, path, source)
	must(t, err)
	primary, err := nativeGoRustPrimaryContract(runtime, choices[0])
	must(t, err)
	expected, err := json.MarshalIndent(primary, "", "  ")
	must(t, err)
	files["expected-primary.json"] = string(expected) + "\n"
	must(t, os.WriteFile(filepath.Join(source, "expected-primary.json"), []byte(files["expected-primary.json"]), 0o600))
	canonical, err := os.ReadFile(path)
	must(t, err)
	recipe, err := nativeGoRustTreeRecipe(string(canonical), primary.Image, runtime)
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile.canonical"), canonical, 0o600))
	must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile"), []byte(recipe), 0o600))
	must(t, os.WriteFile(filepath.Join(source, "Dockerfile"), []byte(recipe), 0o600))
	hashes := map[string]string{}
	for name, content := range files {
		hashes[name] = fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	}
	receipt, err := json.MarshalIndent(map[string]any{"common": common, "primary": primary, "choices": choices, "platform": docker.platform,
		"canonical_sha256": fmt.Sprintf("%x", sha256.Sum256(canonical)), "executed_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(recipe))),
		"test_only_overlay": "readonly pinned-donor tree comparison after COPY; authored build/start unchanged", "fixture_sha256": hashes,
		"scope": "separate common Go/Rust family gate; original13/28 unchanged; no native-init/Kubernetes/hosted or publication claim"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "source-custody.json"), receipt, 0o600))
	tag := fmt.Sprintf("bex-native-gorust-%s:%d-%d", runtime, os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", docker.buildArgs([]string{"--secret", "id=render-env,src=" + bundle, "-t", tag}, source, true)...)
	log, buildErr := command.CombinedOutput()
	must(t, os.WriteFile(filepath.Join(evidence, "build.log"), log, 0o600))
	if buildErr != nil {
		t.Fatalf("actual offline Go/Rust family build: %v\n%s", buildErr, log)
	}
	docker.runImage(t, nativeUtilityCase{name: "common-" + runtime}, tag, evidence, true)
}

func TestNativeGoRustProfileRequiresSeparateOptIn(t *testing.T) {
	t.Setenv("BEX_BUILDKIT_INTEGRATION", "1")
	for _, value := range []string{"", "0", "true", "1 "} {
		t.Setenv("BEX_NATIVE_TEST_GORUST_PROFILE", value)
		if nativeGoRustProfileEnabled() {
			t.Fatalf("generic integration selected extra profile with %q", value)
		}
	}
	t.Setenv("BEX_NATIVE_TEST_GORUST_PROFILE", "1")
	if !nativeGoRustProfileEnabled() {
		t.Fatal("explicit profile not selected")
	}
}

func TestNativeGoRustCanonicalRecipesAndContracts(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	for _, runtime := range []string{"go", "rust"} {
		t.Run(runtime, func(t *testing.T) {
			fake := &fakeExact{envByImage: map[string]fakeExactMetadata{}}
			for family, toolchain := range nativeToolchains {
				for _, line := range toolchain.lines {
					fake.envByImage[line.image] = commonReviewedFixtureMetadata(family, line)
				}
			}
			line := nativeToolchains[runtime].lines[0]
			metadata := fake.envByImage[line.image]
			metadata.env = []string{"PATH=/go/bin:/usr/local/go/bin:/usr/local/bin:/usr/bin", "GOLANG_VERSION=1.27.3", "GOTOOLCHAIN=local", "GOPATH=/go"}
			if runtime == "rust" {
				metadata.env = []string{"PATH=/usr/local/cargo/bin:/usr/local/bin:/usr/bin", "RUST_VERSION=1.96.0", "CARGO_HOME=/usr/local/cargo", "RUSTUP_HOME=/usr/local/rustup"}
			}
			fake.envByImage[line.image] = metadata
			o := nativeOptions()
			o.Runtime, o.BuildEnv, o.RuntimeEnvSecret = runtime, nil, ""
			o.BuildCommand, o.StartCommand = "/usr/bin/python3 check.py build "+runtime, "/usr/bin/python3 check.py runtime "+runtime
			bundle, path, source := writeResolveFixture(t, o, nil, nativeGoRustFiles(t))
			choices, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, runtime, bundle, path, source)
			must(t, err)
			primary, err := nativeGoRustPrimaryContract(runtime, choices[0])
			must(t, err)
			if primary.Image != line.image || primary.Line != line.line || !slices.Equal(primary.Env, metadata.env) {
				t.Fatal("profile substituted selected primary or exact metadata")
			}
			canonical, err := os.ReadFile(path)
			must(t, err)
			recipe, err := nativeGoRustTreeRecipe(string(canonical), primary.Image, runtime)
			must(t, err)
			for _, required := range []string{"\nFROM " + commonFixtureImage + "\n", "from=" + primary.Image + ",source=/usr/local,target=/opt/bex-test-primary,readonly", shellJSON(nativeEnvLoaderPrefix + o.BuildCommand), shellJSON(o.StartCommand)} {
				if !strings.Contains(recipe, required) {
					t.Fatalf("canonical primary/command boundary lost %q", required)
				}
			}
			for _, bad := range []string{string(canonical) + "\nCOPY . .\n", strings.ReplaceAll(string(canonical), "COPY . .", "COPY other .")} {
				if _, err := nativeGoRustTreeRecipe(bad, primary.Image, runtime); err == nil {
					t.Fatal("ambiguous source boundary accepted")
				}
			}
			for _, bad := range [][]string{nil, append(append([]string(nil), metadata.env...), metadata.env[0]), strings.Split(strings.ReplaceAll(strings.Join(metadata.env, "\n"), primary.Version, "2.0.0"), "\n"), strings.Split(strings.ReplaceAll(strings.Join(metadata.env, "\n"), "/usr/local/", "/foreign/"), "\n")} {
				changed := choices[0]
				changed.imageEnv = bad
				if _, err := nativeGoRustPrimaryContract(runtime, changed); err == nil {
					t.Fatal("missing, duplicate or substituted primary metadata accepted")
				}
			}
		})
	}
}

func TestNativeGoRustFixtureNegativeControls(t *testing.T) {
	command := exec.CommandContext(t.Context(), "python3", "-m", "unittest", "-v", "check_test.py")
	command.Dir = filepath.Join("testdata", "native-common-gorust")
	command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("offline primary/tree/output controls: %v\n%s", err, output)
	}
}
