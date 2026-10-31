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
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

const nativeUtilityOwnerLabel = "co.bex.native-qualification"

func TestNativeCommonQualificationTracksActualGeneratorAndFixtureChanges(t *testing.T) {
	raw, err := os.ReadFile("/private/tmp/bex-w8-common-trigger-source-20261031-1qcjkybt/pre-fix-common-workflow.yml")
	must(t, err)
	var workflow struct {
		On struct {
			Push struct{ Paths []string }
		}
		Jobs map[string]struct {
			Uses string
			With map[string]string
		}
	}
	must(t, yaml.Unmarshal(raw, &workflow))
	// These are the two glob forms used by this actual workflow. Match native
	// file globs by path segment; a trailing /** also includes nested fixtures.
	selected := func(file string) bool {
		for _, pattern := range workflow.On.Push.Paths {
			if strings.HasSuffix(pattern, "/**") && strings.HasPrefix(file, strings.TrimSuffix(pattern, "**")) {
				return true
			}
			match, err := path.Match(pattern, file)
			must(t, err)
			if match {
				return true
			}
		}
		return false
	}
	for _, tc := range []struct {
		file string
		want bool
	}{
		{"lego/operator/internal/build/native.go", true},
		{"lego/operator/internal/build/nativeresolve.go", true},
		{"lego/operator/internal/build/nativecommon_erlang.go", true},
		{"lego/operator/internal/build/nativeutility_integration_test.go", true},
		{"lego/operator/internal/build/testdata/native-utilities/original-cases.py", true},
		{"lego/operator/internal/build/testdata/native-utilities/cases/m103/http.py", true},
		{"deploy/native-common/Dockerfile", true},
		{"lego/operator/internal/controller/disk.go", false},
		{"lego/backend/internal/apps/service.go", false},
		{".pm/w8/m91/README.md", false},
	} {
		if selected(tc.file) != tc.want {
			t.Errorf("actual common push path selection for %s = %v, want %v", tc.file, !tc.want, tc.want)
		}
	}
	publications := 0
	for _, job := range workflow.Jobs {
		if job.Uses == "./.github/workflows/native-base-image.yml" {
			publications++
			if job.With["runtime"] != "common-bookworm" {
				t.Fatal("generator/fixture change would publish a C runtime instead of one common foundation")
			}
		}
	}
	if publications != 1 {
		t.Fatalf("common workflow has %d publication calls, want one", publications)
	}
}

type nativeUtilityDocker struct{ builder, platform, owner string }

func nativeUtilityDockerConfig(t *testing.T) nativeUtilityDocker {
	t.Helper()
	d := nativeUtilityDocker{os.Getenv("BEX_NATIVE_TEST_BUILDER"), os.Getenv("BEX_NATIVE_TEST_PLATFORM"), os.Getenv("BEX_NATIVE_TEST_OWNER")}
	if d.owner == "" {
		d.owner = fmt.Sprintf("bex-nq-%d-%d", os.Getpid(), time.Now().UnixNano())
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,79}$`).MatchString(d.owner) {
		t.Fatal("invalid test-only resource owner")
	}
	if d.builder != "" && d.builder != d.owner {
		t.Fatal("explicit qualification builder must belong to this resource owner")
	}
	if (d.builder == "") != (d.platform == "") || (d.platform != "" && !slices.Contains([]string{"linux/amd64", "linux/arm64"}, d.platform)) {
		t.Fatal("explicit builder and linux/amd64 or linux/arm64 platform must be supplied together")
	}
	return d
}

func (d nativeUtilityDocker) label() string { return nativeUtilityOwnerLabel + "=" + d.owner }

func nativeUtilityCleanup(args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "docker", args...).Run()
}

func (d nativeUtilityDocker) buildArgs(extra []string, source string, load bool) []string {
	args := []string{"build"}
	if d.builder != "" {
		args = []string{"buildx", "build", "--builder", d.builder, "--platform", d.platform}
		if load {
			args = append(args, "--load")
		}
	}
	args = append(args, "--network", "none", "--progress", "plain", "--label", d.label())
	return append(append(args, extra...), source)
}

func (d nativeUtilityDocker) runImage(t *testing.T, tc nativeUtilityCase, tag, evidence string, prerequisites bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	createArgs := []string{"create", "--network", "none", "--label", d.label()}
	if tc.static {
		createArgs = append(createArgs, "--entrypoint", "/bin/true")
	}
	create := exec.CommandContext(ctx, "docker", append(createArgs, tag)...)
	id, err := create.Output()
	must(t, err)
	container := strings.TrimSpace(string(id))
	t.Cleanup(func() { nativeUtilityCleanup("rm", "-f", container) })
	if !tc.static {
		start, runErr := exec.CommandContext(ctx, "docker", "start", "--attach", container).CombinedOutput()
		must(t, os.WriteFile(filepath.Join(evidence, "runtime.log"), start, 0o600))
		if runErr != nil || !strings.Contains(string(start), `"pass": true`) {
			t.Fatalf("generated network-off runtime: %v\n%s", runErr, start)
		}
		if copied, err := exec.CommandContext(ctx, "docker", "cp", container+":/opt/render/project/src/qa-output", filepath.Join(evidence, "qa-output")).CombinedOutput(); err != nil {
			t.Fatalf("export actual original-program proofs: %v %s", err, copied)
		}
	} else if copied, err := exec.CommandContext(ctx, "docker", "cp", container+":/opt/render/project/src/public", filepath.Join(evidence, "public")).CombinedOutput(); err != nil {
		t.Fatalf("export static artifacts: %v %s", err, copied)
	}
	inspected, err := exec.CommandContext(ctx, "docker", "inspect", container).Output()
	must(t, err)
	var states []struct {
		Image      string
		State      struct{ ExitCode int }
		HostConfig struct{ NetworkMode string }
	}
	must(t, json.Unmarshal(inspected, &states))
	if len(states) != 1 || states[0].HostConfig.NetworkMode != "none" || states[0].State.ExitCode != 0 {
		t.Fatalf("unexpected owned container state: %s", inspected)
	}
	platform, err := exec.CommandContext(ctx, "docker", "image", "inspect", states[0].Image, "--format", "{{.Os}}/{{.Architecture}}").Output()
	must(t, err)
	actualPlatform := strings.TrimSpace(string(platform))
	if d.platform != "" && actualPlatform != d.platform {
		t.Fatalf("generated platform %s differs from requested %s", actualPlatform, d.platform)
	}
	if tc.name == "python3148-default" && os.Getenv("BEX_NATIVE_TEST_SYFT") != "" {
		nativeUtilityGeneratedSBOM(t, ctx, states[0].Image, actualPlatform, evidence)
	}
	receipt, err := json.MarshalIndent(map[string]any{"case": tc.name, "container_id": container, "image_config_id": states[0].Image, "image_platform": actualPlatform, "owner": d.owner, "builder": d.builder, "common_base": nativeCommonBase, "test_prerequisite_overlay": prerequisites, "network": "none", "runtime_command_executed": !tc.static, "static_artifacts_extracted": tc.static, "scope": "local generated image; recorded test-only wheel overlay and optional generated exact-default SPDX; no hosted, Kubernetes pre-deploy or publication claim"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "receipt.json"), receipt, 0o600))
	t.Logf("%s passed actual generated network-off commands; image %s", tc.name, states[0].Image)
}

func nativeUtilityWheelRecipe(recipe, common string) (string, error) {
	const boundary = "\nCOPY . .\n"
	const stage = "bex-test-prerequisites"
	marker := "\nFROM " + common + "\n"
	if strings.Count(recipe, boundary) != 1 || strings.Count(recipe, marker) != 1 {
		return "", fmt.Errorf("wheel preparation requires one canonical common runtime and source boundary")
	}
	prefix, _, _ := strings.Cut(recipe, boundary)
	prefix = strings.Replace(prefix, marker, "\nFROM "+common+" AS "+stage+"\n", 1)
	// Only this recorded prerequisite stage has network access. The actual
	// canonical authored build and runtime commands remain network-off.
	poetry := "\nRUN /usr/local/bin/python -m venv /opt/bex/poetry && "
	if strings.Count(prefix, poetry) != 1 {
		return "", fmt.Errorf("wheel preparation requires the canonical modern Python Poetry bootstrap")
	}
	prepare := "\nRUN --network=default /usr/local/bin/python3 -m pip wheel --disable-pip-version-check --wheel-dir /opt/bex-test-wheels pyvips==3.2.0 Django==5.2.18 poetry==2.1.3\n" +
		"ENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n"
	// Canonical Poetry bootstrapping also needs the recorded wheel closure;
	// acquire it first, then exercise the unchanged runtime/tool prefix offline.
	prefix = strings.Replace(prefix, poetry, prepare+poetry, 1)
	return prefix + "\n" +
		"FROM scratch\nCOPY --from=" + stage + " /opt/bex-test-wheels/ /\n", nil
}

func nativeUtilityPrepareWheels(t *testing.T, d nativeUtilityDocker, recipe []byte, common, bundle, source, evidence string) string {
	t.Helper()
	if d.builder == "" {
		t.Fatal("wheel preparation requires an explicitly owned Buildx builder and platform")
	}
	prepared, err := nativeUtilityWheelRecipe(string(recipe), common)
	must(t, err)
	path := filepath.Join(evidence, "Dockerfile.prerequisites")
	must(t, os.WriteFile(path, []byte(prepared), 0o600))
	wheels := filepath.Join(evidence, "prepared-wheels")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	args := d.buildArgs([]string{"--file", path, "--secret", "id=render-env,src=" + bundle, "--output", "type=local,dest=" + wheels}, source, false)
	command := exec.CommandContext(ctx, "docker", args...)
	log, err := command.CombinedOutput()
	must(t, os.WriteFile(filepath.Join(evidence, "prerequisites.log"), log, 0o600))
	if err != nil {
		t.Fatalf("prepare real target-platform/runtime wheels: %v\n%s", err, log)
	}
	return wheels
}

func nativeUtilityGeneratedSBOM(t *testing.T, ctx context.Context, image, platform, evidence string) {
	t.Helper()
	helper := filepath.Join("..", "..", "..", "..", ".github", "scripts", "native-common-qualify.py")
	command := exec.CommandContext(ctx, "python3", helper, "sbom", "--syft", os.Getenv("BEX_NATIVE_TEST_SYFT"), "--image", image, "--platform", platform, "--evidence", evidence)
	log, err := command.CombinedOutput()
	must(t, os.WriteFile(filepath.Join(evidence, "sbom.log"), log, 0o600))
	if err != nil {
		t.Fatalf("capture/bind actual exact-default SPDX before image cleanup: %v\n%s", err, log)
	}
}

func TestNativeUtilityWheelPreparationPreservesExactRuntimePrefix(t *testing.T) {
	const common = "ghcr.io/bex-co/bex-cnb-run@" + otherDigest
	const primary = "python:3.14.8-bookworm@" + exactDigest
	canonical := "# syntax=docker/dockerfile:1.7\nFROM " + common + "\nCOPY --from=" + primary + " /usr/local /usr/local\nENV PYTHON_VERSION=3.14.8\nRUN /usr/local/bin/python -m venv /opt/bex/poetry && /opt/bex/poetry/bin/pip install poetry==2.1.3\nWORKDIR /opt/render/project/src\nCOPY . .\nRUN --network=none unchanged-original-command\n"
	prepared, err := nativeUtilityWheelRecipe(canonical, common)
	must(t, err)
	for _, required := range []string{primary, "ENV PYTHON_VERSION=3.14.8", "--network=default /usr/local/bin/python3 -m pip wheel", "poetry==2.1.3", "FROM scratch\nCOPY --from=bex-test-prerequisites"} {
		if !strings.Contains(prepared, required) {
			t.Fatalf("prerequisite recipe lost exact selected runtime/input: %s", required)
		}
	}
	if strings.Contains(prepared, "unchanged-original-command") || strings.Contains(prepared, "COPY . .") {
		t.Fatal("prerequisite preparation executed authored source commands")
	}
	if strings.Index(prepared, "pip wheel") > strings.Index(prepared, " -m venv /opt/bex/poetry") || strings.Index(prepared, "ENV PIP_NO_INDEX=1") > strings.Index(prepared, " -m venv /opt/bex/poetry") {
		t.Fatal("Poetry bootstrap ran before its offline prerequisite wheels")
	}
	for _, invalid := range []string{strings.ReplaceAll(canonical, "COPY . .", "COPY source ."), canonical + "\nCOPY . .\n", strings.ReplaceAll(canonical, common, "unrelated-base"), strings.ReplaceAll(canonical, " -m venv /opt/bex/poetry", " -m venv unrelated")} {
		if _, err := nativeUtilityWheelRecipe(invalid, common); err == nil {
			t.Fatal("ambiguous runtime/source boundary accepted")
		}
	}
}

func TestNativeUtilityBuildUsesExplicitBuilderPlatformAndOwner(t *testing.T) {
	d := nativeUtilityDocker{"unique-owned-builder", "linux/amd64", "unique-owned-builder"}
	args := d.buildArgs([]string{"-t", "unique-owned-image"}, "source", true)
	if !slices.Equal(args[:7], []string{"buildx", "build", "--builder", d.builder, "--platform", d.platform, "--load"}) || !slices.Contains(args, d.label()) {
		t.Fatalf("build escaped explicit builder/platform/owner: %v", args)
	}
	if slices.Contains(d.buildArgs([]string{"--output", "type=local,dest=wheels"}, "source", false), "--load") {
		t.Fatal("prerequisite export loaded an unnecessary image")
	}
}

func TestNativeUtilityWheelPreparationAcceptsActualResolvedPythonRecipes(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	for _, tc := range []struct {
		name, version string
		created       time.Time
		env           map[string]string
	}{
		{"explicit313", "3.13", time.Time{}, map[string]string{"PYTHON_VERSION": "3.13"}},
		{"default3148", "3.14.8", pythonCreationCutoff.Add(time.Second), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeExact{digests: map[string]string{"python:3.14.8-bookworm": exactDigest}, env: []string{"PYTHON_VERSION=3.14.8", "PATH=/usr/local/bin:/usr/bin"}, envByImage: map[string]fakeExactMetadata{}}
			for runtime, toolchain := range nativeToolchains {
				for _, line := range toolchain.lines {
					fake.envByImage[line.image] = commonReviewedFixtureMetadata(runtime, line)
				}
			}
			o := nativeOptions()
			o.Runtime = nativePythonRuntime
			bundle, path, source := writeResolveFixture(t, o, tc.env, nil)
			choices, err := ResolveNativeBuildExact(t.Context(), fake, tc.created, o.Runtime, bundle, path, source)
			must(t, err)
			if choices[0].Line != tc.version {
				t.Fatalf("fixture changed its selected runtime: %+v", choices[0])
			}
			canonical, err := os.ReadFile(path)
			must(t, err)
			prepared, err := nativeUtilityWheelRecipe(string(canonical), nativeCommonBase)
			must(t, err)
			if !strings.Contains(prepared, choices[0].Image) || !strings.Contains(prepared, "poetry=="+defaultPoetryVersion) {
				t.Fatal("prerequisite recipe did not retain the actual selected primary and tool")
			}
		})
	}
}
