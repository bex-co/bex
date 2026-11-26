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
	"errors"
	"fmt"
	gobuild "go/build"
	"maps"
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

	"github.com/bex-co/bex/lego/types/toolchains"
)

const nativeUtilityOwnerLabel = "co.bex.native-qualification"

func TestNativeCommonQualificationTracksActualGeneratorAndFixtureChanges(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", ".github", "workflows", "native-common-image.yml"))
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
		{".github/scripts/native-oci-mirror.py", true},
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

// The qualify jobs compile this package (GOWORK=off) from a closed sparse
// checkout, and qualification-source.json records the qualifier's
// SOURCE_PACKAGES. A local package this package or its tests reach, directly
// or through another local package, that either list omits fails `go test
// ./internal/build` with "no required module provides package" before any case
// runs: 020e4198c's lego/types/deploybudgets import did, and this workflow
// only triggers on native*.go, so operator-test is where that has to show up.
func TestNativeCommonQualificationChecksOutEveryLocalPackageTheBuildCompiles(t *testing.T) {
	const module = "github.com/bex-co/bex/"
	repo := filepath.Join("..", "..", "..", "..")
	platform := gobuild.Default
	platform.GOOS, platform.GOARCH = "linux", "amd64" // the qualify job's platform
	local := map[string]bool{}
	var walk func(dir string, tests bool)
	walk = func(dir string, tests bool) {
		pkg, err := platform.ImportDir(filepath.Join(repo, filepath.FromSlash(dir)), 0)
		must(t, err)
		imports := slices.Clone(pkg.Imports)
		if tests {
			imports = append(append(imports, pkg.TestImports...), pkg.XTestImports...)
		}
		for _, imported := range imports {
			if rel, ok := strings.CutPrefix(imported, module); ok && !local[rel] {
				local[rel] = true
				walk(rel, false)
			}
		}
	}
	walk("lego/operator/internal/build", true)
	walk("lego/operator/cmd/native-init", false)
	if !local["lego/types/v1alpha1"] {
		t.Fatal("import walk missed lego/types/v1alpha1; it is not reading the build package")
	}

	helper, err := os.ReadFile(filepath.Join(repo, ".github", "scripts", "native-common-qualify.py"))
	must(t, err)
	declared := regexp.MustCompile(`(?s)\nSOURCE_PACKAGES = \((.*?)\n\)`).FindSubmatch(helper)
	if declared == nil {
		t.Fatal("native-common-qualify.py no longer declares SOURCE_PACKAGES")
	}
	recorded := map[string]bool{}
	for _, name := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(declared[1], -1) {
		recorded["lego/"+string(name[1])] = true
	}

	raw, err := os.ReadFile(filepath.Join(repo, ".github", "workflows", "native-common-image.yml"))
	must(t, err)
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Uses string
				With map[string]any
			}
		}
	}
	must(t, yaml.Unmarshal(raw, &workflow))
	closed := map[string][]string{}
	for name, job := range workflow.Jobs {
		for _, step := range job.Steps {
			sparse, _ := step.With["sparse-checkout"].(string)
			rules := strings.Fields(sparse)
			// Open checkouts ("/*" then "!/.pm/") already carry the whole tree.
			if strings.HasPrefix(step.Uses, "actions/checkout@") && len(rules) > 0 && rules[0] == "!/**" {
				closed[name] = rules
			}
		}
	}
	checkouts := slices.Sorted(maps.Keys(closed))
	if len(checkouts) == 0 {
		t.Fatal("native-common-image.yml has no closed sparse checkout left to check")
	}
	selects := func(rules []string, dir string) bool {
		for _, rule := range rules {
			if prefix, ok := strings.CutSuffix(rule, "**"); ok && !strings.HasPrefix(rule, "!") && strings.HasPrefix("/"+dir+"/", prefix) {
				return true
			}
		}
		return false
	}
	for _, dir := range slices.Sorted(maps.Keys(local)) {
		if strings.HasPrefix(dir+"/", "lego/operator/internal/build/") {
			continue // the build tree is recorded and checked out whole
		}
		if !recorded[dir] {
			t.Errorf("native-common-qualify.py SOURCE_PACKAGES omits %s, which the build package compiles", strings.TrimPrefix(dir, "lego/"))
		}
		for _, job := range checkouts {
			if !selects(closed[job], dir) {
				t.Errorf("native-common-image.yml job %s's sparse checkout omits /%s/**, which the build package compiles", job, dir)
			}
		}
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

// prerequisiteBuildArgs is buildArgs for a recorded build/export that fetches
// test prerequisites. A build-level --network none also binds every
// RUN --network=default, so the wheel download could never resolve a host;
// this build's default network is the daemon's, and every RUN that must stay
// offline names --network=none itself.
func (d nativeUtilityDocker) prerequisiteBuildArgs(extra []string, source string, load bool) []string {
	args := d.buildArgs(extra, source, load)
	for i := range args[:len(args)-1] {
		if args[i] == "--network" && args[i+1] == "none" {
			args[i+1] = "default"
			break
		}
	}
	return args
}

// nativeUtilityPrerequisiteRecipe opens only the caller-validated, checksum-
// bound platform RUN. All other RUN instructions become explicitly offline
// before the CLI default is opened. Generated COPY heredocs are data, including
// the env loader handled by runNativeBuildShell, and remain byte-for-byte.
func nativeUtilityPrerequisiteRecipe(canonical, selected string) (string, error) {
	if !strings.HasPrefix(selected, "RUN ") || strings.Contains(selected, "\n") {
		return "", fmt.Errorf("prerequisite must be one complete canonical RUN")
	}
	lines := strings.Split(canonical, "\n")
	heredoc, found := "", 0
	for i, line := range lines {
		if heredoc != "" {
			if line == heredoc {
				heredoc = ""
			}
			continue
		}
		if strings.HasPrefix(line, "COPY ") && strings.Contains(line, "<<") {
			_, tail, quoted := strings.Cut(line, "<<'")
			end, _, closed := strings.Cut(tail, "'")
			if !quoted || !closed || !regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`).MatchString(end) {
				return "", fmt.Errorf("unexpected canonical COPY heredoc")
			}
			heredoc = end
			continue
		}
		command, run := strings.CutPrefix(line, "RUN ")
		if !run {
			continue
		}
		hasNetwork := false
		for flags := command; strings.HasPrefix(flags, "--"); {
			flag, tail, hasCommand := strings.Cut(flags, " ")
			if !hasCommand {
				return "", fmt.Errorf("canonical RUN has no command")
			}
			if strings.HasPrefix(flag, "--network=") {
				if hasNetwork || flag != "--network=none" {
					return "", fmt.Errorf("unexpected canonical RUN network flag")
				}
				hasNetwork = true
			}
			flags = tail
		}
		if line == selected {
			if hasNetwork {
				return "", fmt.Errorf("prerequisite already has a network flag")
			}
			lines[i] = "RUN --network=default " + command
			found++
		} else if !hasNetwork {
			lines[i] = "RUN --network=none " + command
		}
	}
	if heredoc != "" || found != 1 {
		return "", fmt.Errorf("prerequisite requires one RUN and complete canonical heredocs")
	}
	return strings.Join(lines, "\n"), nil
}

func nativeUtilityWithoutRunNetwork(recipe string) string {
	return strings.ReplaceAll(strings.ReplaceAll(recipe, "\nRUN --network=default ", "\nRUN "), "\nRUN --network=none ", "\nRUN ")
}

func (d nativeUtilityDocker) runImage(t *testing.T, tc nativeUtilityCase, tag, evidence string, prerequisites bool, runtimeEnv ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	createArgs := []string{"create", "--network", "none", "--label", d.label()}
	for _, entry := range runtimeEnv {
		createArgs = append(createArgs, "--env", entry)
	}
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
	wormhole := !tc.static && nativeCommonBase != ""
	if wormhole {
		d.probeWormhole(ctx, t, tag, evidence, runtimeEnv)
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
	receipt, err := json.MarshalIndent(map[string]any{"case": tc.name, "container_id": container, "image_config_id": states[0].Image, "image_platform": actualPlatform, "owner": d.owner, "builder": d.builder, "common_base": nativeCommonBase, "test_prerequisite_overlay": prerequisites, "runtime_environment": runtimeEnv, "network": "none", "runtime_command_executed": !tc.static, "static_artifacts_extracted": tc.static, "wormhole_probed": wormhole, "scope": "local generated image; recorded qualification overlays and optional generated exact-default SPDX; no hosted, Kubernetes pre-deploy or publication claim"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "receipt.json"), receipt, 0o600))
	t.Logf("%s passed actual generated network-off commands; image %s", tc.name, states[0].Image)
}

// nativeWormholeProbe is what a tenant's Shell on a native service runs:
// Render pre-installs magic-wormhole on every native runtime (w6/m549). The
// runtime copied over the common image must neither shadow the packaged
// client nor change its interpreter, and the client's crypto must load. The
// factory check sends a real file through a relay; this proves the generated
// image of every family still carries that client.
const nativeWormholeProbe = `set -eu
test "$(command -v wormhole)" = /usr/bin/wormhole
test "$(head -n 1 /usr/bin/wormhole | sed 's/^#![[:space:]]*//')" = /usr/bin/python3
wormhole --version
cd / && /usr/bin/python3 -c '
import nacl.secret, spake2, wormhole.cli.cmd_receive, wormhole.cli.cmd_send
box = nacl.secret.SecretBox(bytes(32))
assert box.decrypt(box.encrypt(b"bex")) == b"bex"
a, b = spake2.SPAKE2_Symmetric(b"code"), spake2.SPAKE2_Symmetric(b"code")
sa, sb = a.start(), b.start()
assert a.finish(sb) == b.finish(sa)
print("wormhole client loads on the system interpreter")'
`

func (d nativeUtilityDocker) probeWormhole(ctx context.Context, t *testing.T, tag, evidence string, runtimeEnv []string) {
	t.Helper()
	args := make([]string, 0, 11+2*len(runtimeEnv))
	args = append(args, "run", "--rm", "--network", "none", "--label", d.label(), "--entrypoint", "/bin/sh")
	for _, entry := range runtimeEnv {
		args = append(args, "--env", entry)
	}
	out, err := exec.CommandContext(ctx, "docker", append(args, tag, "-c", nativeWormholeProbe)...).CombinedOutput()
	must(t, os.WriteFile(filepath.Join(evidence, "wormhole.log"), out, 0o600))
	if err != nil || !strings.Contains(string(out), "magic-wormhole ") {
		t.Fatalf("packaged wormhole in the generated image: %v\n%s", err, out)
	}
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
	offlinePoetry := strings.Replace(poetry, "\nRUN ", "\nRUN --network=none ", 1)
	if strings.Count(prefix, poetry)+strings.Count(prefix, offlinePoetry) != 1 {
		return "", fmt.Errorf("wheel preparation requires the canonical modern Python Poetry bootstrap")
	}
	if strings.Contains(prefix, offlinePoetry) {
		poetry = offlinePoetry
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
	prepared, err := nativeUtilityWheelRecipe(string(recipe), common)
	must(t, err)
	return nativeUtilityExportWheels(t, d, prepared, bundle, source, evidence)
}

func nativeUtilityExportWheels(t *testing.T, d nativeUtilityDocker, prepared, bundle, source, evidence string) string {
	t.Helper()
	if d.builder == "" {
		t.Fatal("wheel preparation requires an explicitly owned Buildx builder and platform")
	}
	recipePath := filepath.Join(evidence, "Dockerfile.prerequisites")
	must(t, os.WriteFile(recipePath, []byte(prepared), 0o600))
	wheels := filepath.Join(evidence, "prepared-wheels")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	args := d.prerequisiteBuildArgs([]string{"--file", recipePath, "--secret", "id=render-env,src=" + bundle, "--output", "type=local,dest=" + wheels}, source, false)
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
	prerequisites := d.prerequisiteBuildArgs([]string{"--output", "type=local,dest=wheels"}, "source", false)
	if slices.Contains(prerequisites, "--load") {
		t.Fatal("prerequisite export loaded an unnecessary image")
	}
	loadedPrerequisite := d.prerequisiteBuildArgs([]string{"-t", "unique-owned-image"}, "source", true)
	if !slices.Equal(loadedPrerequisite[:7], args[:7]) {
		t.Fatalf("prerequisite image lost its explicit --load/builder/platform: %v", loadedPrerequisite)
	}
	if i := slices.Index(loadedPrerequisite, "--network"); i < 0 || loadedPrerequisite[i+1] != "default" {
		t.Fatalf("prerequisite image cannot fetch its checksum-bound platform source: %v", loadedPrerequisite)
	}
	// Only recorded prerequisite builds may reach the network; ordinary image builds stay offline.
	if i := slices.Index(prerequisites, "--network"); i < 0 || prerequisites[i+1] != "default" {
		t.Fatalf("prerequisite export cannot fetch its wheels: %v", prerequisites)
	}
	if i := slices.Index(args, "--network"); i < 0 || args[i+1] != "none" {
		t.Fatalf("image build left the network open: %v", args)
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
		{"default3148", "3.14.8", toolchains.RenderDefaults20261012.Add(time.Second), nil},
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

func TestNativeUtilityExactDefaultSelectionKeepsLookupEvidence(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	reviewed, ok := nativeToolchains[nativePythonRuntime].find("3.14")
	if !ok || reviewed.version == "3.14.8" {
		t.Fatal("test requires a reviewed line distinct from the exact creation-date default")
	}
	for _, tc := range []struct {
		name, note string
		found      bool
		lookupErr  error
	}{
		{name: "exact image", found: true},
		{name: "missing tag", note: "No official image python:3.14.8-bookworm"},
		{name: "lookup failure", note: "controlled lookup transport failure", lookupErr: errors.New("controlled lookup transport failure")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeExact{digests: map[string]string{}, err: tc.lookupErr,
				env: []string{"PYTHON_VERSION=3.14.8", "PATH=/usr/local/bin:/usr/bin"}, envByImage: map[string]fakeExactMetadata{}}
			if tc.found {
				fake.digests["python:3.14.8-bookworm"] = exactDigest
			}
			for runtime, toolchain := range nativeToolchains {
				for _, line := range toolchain.lines {
					fake.envByImage[line.image] = commonReviewedFixtureMetadata(runtime, line)
				}
			}
			o := nativeOptions()
			o.Runtime = nativePythonRuntime
			bundle, path, source := writeResolveFixture(t, o, nil, nil)
			choices, err := ResolveNativeBuildExact(t.Context(), fake, toolchains.RenderDefaults20261012.Add(time.Second), o.Runtime, bundle, path, source)
			must(t, err) // Existing exact-lookup fallback still resolves a build.
			if !slices.Contains(fake.asked, "python:3.14.8-bookworm") || choices[0].Exact != "3.14.8" {
				t.Fatalf("creation-date exact lookup changed: %+v; queries=%v", choices[0], fake.asked)
			}
			data, selectionErr := nativeUtilitySelectionReceipt(choices, "3.14.8")
			var record struct {
				Choices []struct{ Image, Narration string }
				Passed  bool `json:"selection_precondition_passed"`
			}
			must(t, json.Unmarshal(data, &record))
			if len(record.Choices) != len(choices) || record.Choices[0].Image != choices[0].Image {
				t.Fatalf("selected identities were lost: %s", data)
			}
			if tc.found {
				if selectionErr != nil || !record.Passed || choices[0].Line != "3.14.8" {
					t.Fatalf("exact default rejected: %v; %s", selectionErr, data)
				}
			} else {
				if choices[0].Image != reviewed.image || selectionErr == nil || record.Passed ||
					!strings.Contains(selectionErr.Error(), reviewed.version) || !strings.Contains(record.Choices[0].Narration, tc.note) {
					t.Fatalf("fallback was changed or counted as exact-default qualification: %v; %s", selectionErr, data)
				}
			}
		})
	}
}

func TestNativeUtilityPrerequisiteRecipePreservesCommandsAndHeredocs(t *testing.T) {
	selected := "RUN --mount=type=bind,from=trusted,source=/reference,target=/reference,readonly /usr/libexec/build-source 28.0 " + strings.Repeat("a", 64)
	canonical := "FROM trusted\nRUN prepare-platform\n" + selected + "\n" +
		"COPY <<'BEX_LOAD_ENV' /opt/bex/load-env\nRUN --network=host literal-heredoc-data\nBEX_LOAD_ENV\n" +
		"COPY --chmod=755 <<'BEX_PNPM' /usr/local/bin/pnpm\nRUN literal-pnpm-data\nBEX_PNPM\n" +
		"RUN --network=none already-offline\n" +
		"RUN --mount=type=secret,id=render-env [\"/bin/bash\",\"-c\",\". /opt/bex/load-env\\nkeep --network=host literal\"]\n" +
		"CMD [\"unchanged --network=default runtime\"]\n"
	expected := "FROM trusted\nRUN --network=none prepare-platform\nRUN --network=default " + strings.TrimPrefix(selected, "RUN ") + "\n" +
		"COPY <<'BEX_LOAD_ENV' /opt/bex/load-env\nRUN --network=host literal-heredoc-data\nBEX_LOAD_ENV\n" +
		"COPY --chmod=755 <<'BEX_PNPM' /usr/local/bin/pnpm\nRUN literal-pnpm-data\nBEX_PNPM\n" +
		"RUN --network=none already-offline\n" +
		"RUN --network=none --mount=type=secret,id=render-env [\"/bin/bash\",\"-c\",\". /opt/bex/load-env\\nkeep --network=host literal\"]\n" +
		"CMD [\"unchanged --network=default runtime\"]\n"
	actual, err := nativeUtilityPrerequisiteRecipe(canonical, selected)
	must(t, err)
	if actual != expected {
		t.Fatalf("network flags changed canonical commands or heredoc data:\n%s", actual)
	}
	for name, invalid := range map[string]string{
		"missing":           strings.ReplaceAll(canonical, selected, "RUN different-source"),
		"duplicate":         canonical + selected + "\n",
		"other-open-RUN":    canonical + "RUN --network=default fetch-unreviewed\n",
		"hidden-open-flag":  canonical + "RUN --mount=type=secret,id=x --network=host fetch-unreviewed\n",
		"duplicate-network": canonical + "RUN --network=none --network=host fetch-unreviewed\n",
		"unterminated-data": canonical + "COPY <<'BEX_DATA' /x\nRUN ignored\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := nativeUtilityPrerequisiteRecipe(invalid, selected); err == nil {
				t.Fatal("changed or ambiguous prerequisite accepted")
			}
		})
	}
}

func TestNativeUtilityWheelPreparationKeepsOfflineBootstrap(t *testing.T) {
	const common = "ghcr.io/bex-co/bex-cnb-run@" + otherDigest
	canonical := "FROM trusted-runtime AS source\nFROM " + common + "\nRUN --network=none /usr/local/bin/python -m venv /opt/bex/poetry && /opt/bex/poetry/bin/pip install poetry==2.1.3\nCOPY . .\nRUN --network=none original-build\n"
	prepared, err := nativeUtilityWheelRecipe(canonical, common)
	must(t, err)
	if !strings.Contains(prepared, "RUN --network=none /usr/local/bin/python -m venv") || strings.Contains(prepared, "original-build") {
		t.Fatal("already-offline bootstrap lost its flag or authored source reached wheel export")
	}
	if strings.Count(prepared, "RUN --network=default ") != 1 {
		t.Fatal("wheel acquisition network boundary changed")
	}
}
