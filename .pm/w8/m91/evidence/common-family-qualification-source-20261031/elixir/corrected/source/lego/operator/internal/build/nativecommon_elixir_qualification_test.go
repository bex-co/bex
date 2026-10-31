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
	if tc.otp != "" && env["OTP_VERSION"] != tc.otp {
		return nil, fmt.Errorf("exact OTP fixture resolved %s instead of %s", env["OTP_VERSION"], tc.otp)
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
	if strings.Count(recipe, "\n"+line) != 1 {
		return "", false, fmt.Errorf("OTP prerequisite must have one canonical source-builder command")
	}
	return strings.Replace(recipe, "\n"+line, "\nRUN --network=default "+strings.TrimPrefix(line, "RUN "), 1), true, nil
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
			executed, prerequisites, err := commonElixirQualificationRecipe(string(recipe), choices[0])
			must(t, err)
			must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile"), []byte(executed), 0o600))
			must(t, os.WriteFile(path, []byte(executed), 0o600))
			must(t, os.Rename(path, filepath.Join(source, "Dockerfile")))
			tag := fmt.Sprintf("bex-native-%s:%d-%d", tc.name, os.Getpid(), time.Now().UnixNano())
			t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, "docker", docker.buildArgs([]string{"--secret", "id=render-env,src=" + bundle, "-t", tag}, source, true)...)
			log, buildErr := command.CombinedOutput()
			must(t, os.WriteFile(filepath.Join(evidence, "build.log"), log, 0o600))
			if buildErr != nil {
				t.Fatalf("actual common Elixir/OTP build: %v\n%s", buildErr, log)
			}
			docker.runImage(t, nativeUtilityCase{name: tc.name}, tag, evidence, prerequisites)
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
				if strings.Replace(executed, "RUN --network=default --mount=type=bind", "RUN --mount=type=bind", 1) != string(recipe) {
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
