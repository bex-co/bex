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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type commonElixirCase struct{ name, version, elixir, otp string }

func commonElixirCases() []commonElixirCase {
	var cases []commonElixirCase
	for _, line := range nativeToolchains[nativeElixirRuntime].lines {
		cases = append(cases, commonElixirCase{name: "elixir-" + line.line, version: line.line, elixir: line.version})
	}
	return append(cases, commonElixirCase{name: "elixir-exact-otp280", version: "1.18.4", elixir: "1.18.4", otp: "28.0"})
}

func commonElixirEnvironment(tc commonElixirCase) map[string]string {
	// Elixir selection uses the env contract, not a version file. A full
	// reviewed patch with its image's v prefix preserves the same value in
	// the authored command's env bundle and the final image environment.
	env := map[string]string{"ELIXIR_VERSION": "v" + tc.elixir}
	if tc.otp != "" {
		env["ERLANG_VERSION"] = tc.otp
	}
	return env
}

func commonElixirExpectations(choices []NativeChoice, tc commonElixirCase) ([]byte, error) {
	if len(choices) != 2 {
		return nil, fmt.Errorf("Elixir qualification requires the primary and one Node companion")
	}
	if choices[0].Line != tc.version {
		return nil, fmt.Errorf("Elixir fixture selected %s instead of %s", choices[0].Line, tc.version)
	}
	env := map[string]string{"ELIXIR_VERSION": "v" + tc.elixir, "MIX_ENV": "prod"}
	for _, key := range []string{"OTP_VERSION", "REBAR_VERSION", "REBAR3_VERSION"} {
		value, err := commonImageVariable(choices[0].imageEnv, key)
		if err != nil {
			return nil, err
		}
		env[key] = value
	}
	if tc.otp != "" && (!otpRelease.MatchString(env["OTP_VERSION"]) || !otpMatches(env["OTP_VERSION"], tc.otp)) {
		return nil, fmt.Errorf("resolved OTP %s does not satisfy fixture request %s", env["OTP_VERSION"], tc.otp)
	}
	node, err := commonImageVariable(choices[1].imageEnv, "NODE_VERSION")
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(map[string]any{"elixir": tc.elixir, "otp": env["OTP_VERSION"], "node": node, "env": env}, "", "  ")
}

func commonElixirFixtureFiles(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("testdata", "native-elixir-qualification")
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
		if !entry.Type().IsRegular() {
			return fmt.Errorf("Elixir fixture must be regular: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[relative] = string(raw)
		return nil
	}))
	files[".node-version"] = "24\n"
	return files
}

func commonElixirQualificationRecipe(recipe string, primary NativeChoice) (string, bool, error) {
	if primary.commonOTP == "" {
		return recipe, false, nil
	}
	// Acquiring the checksum-bound platform OTP source is a prerequisite, not
	// an authored project command. Only that canonical builder RUN may use
	// network access; the actual fixture build/runtime retain network=none.
	line := "RUN --mount=type=bind,from=" + primary.Image + ",source=/usr/local/lib/erlang,target=/opt/bex-otp-reference,readonly /usr/libexec/bex-native/build-otp " + primary.commonOTP + " "
	var selected string
	for candidate := range strings.SplitSeq(primary.primaryStage, "\n") {
		if checksum, found := strings.CutPrefix(candidate, line); found {
			if !digestPattern.MatchString("sha256:" + checksum) {
				return "", false, fmt.Errorf("OTP source builder checksum changed")
			}
			selected = candidate
		}
	}
	if selected == "" || strings.Count(primary.primaryStage, line) != 1 || strings.Count(recipe, "\n"+selected+"\n") != 1 || strings.Count(recipe, " /usr/libexec/bex-native/build-otp ") != 1 {
		return "", false, fmt.Errorf("OTP prerequisite must have one canonical source-builder command")
	}
	executed, err := nativeUtilityPrerequisiteRecipe(recipe, selected)
	return executed, true, err
}

const commonElixirEmulationEnv = "ERL_FLAGS=+JMsingle true"

func commonElixirSingleMapping(daemon, target string) bool {
	return daemon == "linux/arm64" && target == "linux/amd64"
}

// Only the qualification's known generated stage prefixes gain a build ARG.
// The authored commands/heredocs stay byte-exact, and ARG is not an image ENV.
func commonElixirEmulationRecipe(canonical, common string, primary NativeChoice) (string, error) {
	const header = "# syntax=docker/dockerfile:1.7\n"
	stage, emulatedStage := primary.primaryStage, primary.primaryStage
	if stage != "" {
		first, rest, found := strings.Cut(stage, "\n")
		if !found || !strings.HasPrefix(first, "FROM "+common+" AS ") || strings.Contains(rest, "\nFROM ") {
			return "", fmt.Errorf("emulation ARG requires the known common OTP stage")
		}
		emulatedStage = first + "\nARG ERL_FLAGS\n" + rest
	}
	prefix, replacement := header, header
	if stage != "" {
		prefix += stage + "\n"
		replacement += emulatedStage + "\n"
	}
	prefix += "FROM " + common + "\n"
	replacement += "FROM " + common + "\nARG ERL_FLAGS\n"
	rest, found := strings.CutPrefix(canonical, prefix)
	if !found {
		return "", fmt.Errorf("emulation ARG requires the exact canonical common stage prefix")
	}
	return replacement + rest, nil
}

// Separate from the frozen original 13 tool replays and 28 hosted cases.
// Explicit opt-in prevents this expensive full-OTP profile from joining the
// first common-image cohort or a generic BuildKit integration invocation.
func TestCommonElixirOfflineQualification(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" || os.Getenv("BEX_NATIVE_ELIXIR_INTEGRATION") != "1" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 and BEX_NATIVE_ELIXIR_INTEGRATION=1 for actual common Elixir/OTP qualification")
	}
	common, _, _ := nativeUtilityTestImages(t)
	if common == "" {
		t.Fatal("Elixir qualification requires reviewed immutable BEX_NATIVE_TEST_COMMON_IMAGE")
	}
	docker := nativeUtilityDockerConfig(t)
	if docker.builder == "" {
		t.Fatal("Elixir qualification requires an explicitly owned Buildx builder and platform")
	}
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = common, nil, nil
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		var err error
		output, err = os.MkdirTemp(requested, "native-elixir-")
		must(t, err)
	}
	t.Logf("retained Elixir/OTP qualification evidence: %s", output)
	// Record the actual daemon, rather than inferring it from this test's Go ELF.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	daemon, daemonErr := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Os}}/{{.Server.Arch}}").CombinedOutput()
	cancel()
	must(t, os.WriteFile(filepath.Join(output, "daemon-platform.txt"), daemon, 0o600))
	must(t, daemonErr)
	emulatedAMD := commonElixirSingleMapping(strings.TrimSpace(string(daemon)), docker.platform)
	var runtimeEnv []string
	if emulatedAMD {
		runtimeEnv = []string{commonElixirEmulationEnv}
	}
	observation, err := json.MarshalIndent(map[string]any{"daemon": strings.TrimSpace(string(daemon)), "target": docker.platform, "single_mapping": emulatedAMD, "runtime_environment": runtimeEnv, "scope": "qualification emulator compatibility only; no customer or final-image ENV change"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(output, "emulation.json"), observation, 0o600))
	for _, tc := range commonElixirCases() {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = nativeElixirRuntime
			o.BuildCommand, o.StartCommand = "/usr/bin/python3 check.py build", "/usr/bin/python3 check.py runtime"
			bundle, path, source := writeResolveFixture(t, o, commonElixirEnvironment(tc), commonElixirFixtureFiles(t))
			choices, err := ResolveNativeBuildAt(time.Time{}, o.Runtime, bundle, path, source)
			must(t, err)
			expected, err := commonElixirExpectations(choices, tc)
			must(t, err)
			must(t, os.WriteFile(filepath.Join(source, "expected.json"), expected, 0o600))
			evidence := filepath.Join(output, tc.name)
			must(t, os.Mkdir(evidence, 0o700))
			must(t, os.WriteFile(filepath.Join(evidence, "expected.json"), expected, 0o600))
			recipe, err := os.ReadFile(path)
			must(t, err)
			must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile.canonical"), recipe, 0o600))
			executed := string(recipe)
			if emulatedAMD {
				executed, err = commonElixirEmulationRecipe(executed, common, choices[0])
				must(t, err)
			}
			executed, prerequisites, err := commonElixirQualificationRecipe(executed, choices[0])
			must(t, err)
			must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile"), []byte(executed), 0o600))
			must(t, os.WriteFile(path, []byte(executed), 0o600))
			must(t, os.Rename(path, filepath.Join(source, "Dockerfile")))
			tag := fmt.Sprintf("bex-native-%s:%d-%d", tc.name, os.Getpid(), time.Now().UnixNano())
			t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Minute)
			defer cancel()
			extra := []string{"--secret", "id=render-env,src=" + bundle, "-t", tag}
			if emulatedAMD {
				extra = append(extra, "--build-arg", commonElixirEmulationEnv)
			}
			args := docker.buildArgs(extra, source, true)
			if prerequisites {
				args = docker.prerequisiteBuildArgs(extra, source, true)
			}
			command := exec.CommandContext(ctx, "docker", args...)
			log, buildErr := command.CombinedOutput()
			must(t, os.WriteFile(filepath.Join(evidence, "build.log"), log, 0o600))
			if buildErr != nil {
				t.Fatalf("actual common Elixir/OTP build: %v\n%s", buildErr, log)
			}
			if emulatedAMD {
				imageEnv, err := exec.CommandContext(ctx, "docker", "image", "inspect", tag, "--format", "{{json .Config.Env}}").Output()
				must(t, os.WriteFile(filepath.Join(evidence, "image-env.json"), imageEnv, 0o600))
				must(t, err)
				var entries []string
				must(t, json.Unmarshal(imageEnv, &entries))
				for _, entry := range entries {
					if strings.HasPrefix(entry, "ERL_FLAGS=") {
						t.Fatal("qualification VM setting escaped into the final OCI environment")
					}
				}
			}
			docker.runImage(t, nativeUtilityCase{name: tc.name}, tag, evidence, prerequisites || emulatedAMD, runtimeEnv...)
		})
	}
}

func TestCommonElixirQualificationCanonicalRecipes(t *testing.T) {
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = commonFixtureImage, nil, nil
	for _, tc := range commonElixirCases() {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeExact{digests: map[string]string{}, envByImage: map[string]fakeExactMetadata{}}
			for runtime, chain := range nativeToolchains {
				for _, line := range chain.lines {
					fake.envByImage[line.image] = commonReviewedFixtureMetadata(runtime, line)
				}
			}
			if tc.otp != "" {
				tag := tc.elixir + "-erlang-28.0-debian-bookworm-20260610"
				fake.digests["erlang:28.0.0.0"], fake.digests[hexpmElixirRepo+":"+tag] = exactDigest, otherDigest
				fake.envByImage["erlang:28.0.0.0@"+exactDigest] = fakeExactMetadata{env: []string{"OTP_VERSION=28.0", "REBAR_VERSION=2.6.4", "REBAR3_VERSION=3.27.0"}, history: commonOTPSyntheticHistory("bookworm")}
			}
			o := nativeOptions()
			o.Runtime, o.BuildCommand, o.StartCommand = nativeElixirRuntime, "/usr/bin/python3 check.py build", "/usr/bin/python3 check.py runtime"
			bundle, path, source := writeResolveFixture(t, o, commonElixirEnvironment(tc), commonElixirFixtureFiles(t))
			choices, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, o.Runtime, bundle, path, source)
			must(t, err)
			_, err = commonElixirExpectations(choices, tc)
			must(t, err)
			recipe, err := os.ReadFile(path)
			must(t, err)
			for _, required := range []string{"\nFROM " + commonFixtureImage + "\n", "check.py build", "check.py runtime", "MIX_ENV=prod"} {
				if !strings.Contains(string(recipe), required) {
					t.Fatalf("qualification lost canonical behavior %q", required)
				}
			}
			executed, overlay, err := commonElixirQualificationRecipe(string(recipe), choices[0])
			must(t, err)
			if overlay != (choices[0].commonOTP != "") || strings.Count(executed, "RUN --network=default ") != map[bool]int{false: 0, true: 1}[overlay] {
				t.Fatal("OTP source preparation changed another network boundary")
			}
			if overlay {
				if nativeUtilityWithoutRunNetwork(executed) != nativeUtilityWithoutRunNetwork(string(recipe)) {
					t.Fatal("OTP test overlay changed the canonical selected payload or authored commands")
				}
				for _, bad := range []string{strings.ReplaceAll(string(recipe), " /usr/libexec/bex-native/build-otp ", " unrelated "), string(recipe) + string(recipe)} {
					if _, _, err := commonElixirQualificationRecipe(bad, choices[0]); err == nil {
						t.Fatal("ambiguous or changed OTP prerequisite accepted")
					}
				}
			}
		})
	}
}

func TestCommonElixirOutputOracle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", "testdata/native-elixir-qualification/check_test.py", "-v")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Elixir fixture output-oracle controls: %v\n%s", err, output)
	}
}

func TestCommonElixirRequestedOTPMatchesResolvedMetadata(t *testing.T) {
	for _, tc := range []struct {
		request, resolved string
		accept            bool
	}{
		{"28.0", "28.0.4", true},
		{"28.0.0", "28.0.0.1", true},
		{"28.0.2", "28.0.2.1", true},
		{"28.0.2.1", "28.0.2.1", true},
		{"28.0.0.0", "28.0", true},
		{"28.0", "28.1", false},
		{"28.0.2", "28.0.3", false},
		{"28.0.2.1", "28.0.2.2", false},
		{"28.0.0.0", "28.0.0.1", false},
		{"28.0", "27.0.4", false},
		{"28.0", "28.0-rc.1", false},
		{"28.0", "28.0.invalid", false},
	} {
		t.Run(tc.request+"/"+tc.resolved, func(t *testing.T) {
			fixture := commonElixirCase{name: "resolved-metadata", version: "1.18.4", elixir: "1.18.4", otp: tc.request}
			choices := []NativeChoice{
				{Line: fixture.version, imageEnv: []string{"OTP_VERSION=" + tc.resolved, "REBAR_VERSION=2.6.4", "REBAR3_VERSION=3.27.0"}},
				{imageEnv: []string{"NODE_VERSION=24.22.1"}},
			}
			raw, err := commonElixirExpectations(choices, fixture)
			if !tc.accept {
				if err == nil || raw != nil {
					t.Fatalf("wrong resolved OTP accepted: request=%s resolved=%s", tc.request, tc.resolved)
				}
				return
			}
			must(t, err)
			var expected struct {
				OTP string            `json:"otp"`
				Env map[string]string `json:"env"`
			}
			must(t, json.Unmarshal(raw, &expected))
			if expected.OTP != tc.resolved || expected.Env["OTP_VERSION"] != tc.resolved {
				t.Fatal("runtime/header/NIF expectation lost the exact selected metadata version")
			}
			if commonElixirEnvironment(fixture)["ERLANG_VERSION"] != tc.request {
				t.Fatal("fixture rewrote the authored OTP request to satisfy its expectation")
			}
		})
	}
}

func TestCommonElixirSingleMappingRequiresActualEmulation(t *testing.T) {
	for _, tc := range []struct {
		daemon, target string
		want           bool
	}{
		{"linux/arm64", "linux/amd64", true},
		{"linux/amd64", "linux/amd64", false},
		{"linux/arm64", "linux/arm64", false},
		{"linux/amd64", "linux/arm64", false},
		{"", "linux/amd64", false},
		{"linux/arm64", "", false},
		{"windows/arm64", "linux/amd64", false},
		{"linux/arm64\nwarning", "linux/amd64", false},
	} {
		if actual := commonElixirSingleMapping(tc.daemon, tc.target); actual != tc.want {
			t.Errorf("daemon=%q target=%q: %v, want %v", tc.daemon, tc.target, actual, tc.want)
		}
	}
}

func TestCommonElixirEmulationRecipePreservesAuthoredBytes(t *testing.T) {
	const common = "ghcr.io/bex-co/bex-cnb-run@" + exactDigest
	const header = "# syntax=docker/dockerfile:1.7\n"
	for _, stage := range []string{"", "FROM " + common + " AS bex-otp-bookworm\nCOPY --from=reviewed /usr/local/ /usr/local/\nRUN exact-otp-source\n"} {
		primary := NativeChoice{primaryStage: stage}
		prefix := header
		expected := header
		if stage != "" {
			prefix += stage + "\n"
			first, rest, _ := strings.Cut(stage, "\n")
			expected += first + "\nARG ERL_FLAGS\n" + rest + "\n"
		}
		prefix += "FROM " + common + "\n"
		expected += "FROM " + common + "\nARG ERL_FLAGS\n"
		body := "COPY <<'AUTHORED' /app/data\nFROM " + common + "\nARG ERL_FLAGS\nENV ERL_FLAGS=literal-data\nAUTHORED\nRUN --network=none original-build\nCMD [\"original-runtime\"]\n"
		executed, err := commonElixirEmulationRecipe(prefix+body, common, primary)
		must(t, err)
		if executed != expected+body {
			t.Fatal("emulation ARG changed authored command, data or stage bytes")
		}
		for _, changed := range []string{strings.Replace(prefix+body, header, "# changed syntax\n", 1), strings.Replace(prefix+body, "FROM "+common, "FROM unrelated", 1)} {
			if _, err := commonElixirEmulationRecipe(changed, common, primary); err == nil {
				t.Fatal("unrecognized stage prefix accepted")
			}
		}
	}
	if _, err := commonElixirEmulationRecipe(header+"FROM "+common+"\n", common, NativeChoice{primaryStage: "FROM unrelated AS otp\n"}); err == nil {
		t.Fatal("unreviewed source-stage prefix accepted")
	}
}

func TestCommonElixirEmulationComposesWithSourcePrerequisite(t *testing.T) {
	const common = "ghcr.io/bex-co/bex-cnb-run@" + exactDigest
	primary := NativeChoice{Image: "erlang:28.0@" + otherDigest, commonOTP: "28.0"}
	selected := "RUN --mount=type=bind,from=" + primary.Image + ",source=/usr/local/lib/erlang,target=/opt/bex-otp-reference,readonly /usr/libexec/bex-native/build-otp 28.0 " + strings.Repeat("a", 64)
	primary.primaryStage = "FROM " + common + " AS bex-otp-bookworm\n" + selected + "\n"
	body := "RUN --network=none original-authored-build\nCMD [\"original-authored-runtime\"]\n"
	canonical := "# syntax=docker/dockerfile:1.7\n" + primary.primaryStage + "\nFROM " + common + "\n" + body
	emulated, err := commonElixirEmulationRecipe(canonical, common, primary)
	must(t, err)
	executed, prerequisites, err := commonElixirQualificationRecipe(emulated, primary)
	must(t, err)
	if !prerequisites || strings.Count(executed, "\nARG ERL_FLAGS\n") != 2 || !strings.Contains(executed, "RUN --network=default "+strings.TrimPrefix(selected, "RUN ")) || !strings.HasSuffix(executed, body) {
		t.Fatal("emulation build ARG changed the checksum-bound prerequisite or authored commands")
	}
}
