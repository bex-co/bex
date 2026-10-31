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
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

type nativeInitQualificationBinary struct {
	SHA256     string `json:"sha256"`
	Machine    uint16 `json:"machine"`
	Executable bool   `json:"executable"`
	Bytes      int    `json:"bytes"`
}

type nativeInitQualificationExpected struct {
	Run  string                        `json:"run"`
	Node string                        `json:"node"`
	Arch string                        `json:"arch"`
	Env  map[string]string             `json:"env"`
	Init nativeInitQualificationBinary `json:"init"`
}

type nativeInitQualificationIdentity struct {
	Node, Executable string
	Env              map[string]string
	Init             struct {
		SHA256     string
		Machine    uint16
		Executable bool
		PID1       string
	}
}

type nativeInitQualificationProof struct {
	Phase, Run                string
	Identity                  nativeInitQualificationIdentity
	Events                    []string
	Holds, Completed, Signals int
	Pass                      bool
}

func nativeInitQualificationTarget(platform string) (string, elf.Machine, error) {
	switch platform {
	case "linux/amd64":
		return "amd64", elf.EM_X86_64, nil
	case "linux/arm64":
		return "arm64", elf.EM_AARCH64, nil
	default:
		return "", 0, fmt.Errorf("native-init qualification requires explicit linux/amd64 or linux/arm64")
	}
}

func nativeInitQualificationELF(path, platform string) (nativeInitQualificationBinary, error) {
	_, machine, err := nativeInitQualificationTarget(platform)
	if err != nil {
		return nativeInitQualificationBinary{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nativeInitQualificationBinary{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || info.Size() > 16<<20 {
		return nativeInitQualificationBinary{}, fmt.Errorf("native-init must be a bounded executable regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nativeInitQualificationBinary{}, err
	}
	file, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return nativeInitQualificationBinary{}, err
	}
	if file.Class != elf.ELFCLASS64 || file.Data != elf.ELFDATA2LSB || file.Type != elf.ET_EXEC || file.Machine != machine {
		return nativeInitQualificationBinary{}, fmt.Errorf("native-init ELF differs from requested platform %s", platform)
	}
	return nativeInitQualificationBinary{fmt.Sprintf("%x", sha256.Sum256(data)), uint16(machine), true, len(data)}, nil
}

func nativeInitQualificationFixture(t *testing.T) (Options, map[string]string) {
	t.Helper()
	o := nativeInitOptions() // The resolver-image sentinel enables staging only; it is not image provenance.
	o.BuildEnv, o.RuntimeEnvSecret = nil, ""
	o.BuildCommand, o.StartCommand = "node build.js", "npm start"
	files := map[string]string{}
	for _, name := range []string{".node-version", "package.json", "identity.js", "build.js", "server.js"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "native-common-init", name))
		must(t, err)
		files[name] = string(raw)
	}
	return o, files
}

func nativeInitQualificationExpect(choices []NativeChoice, arch, run string, staged nativeInitQualificationBinary) (nativeInitQualificationExpected, error) {
	if len(choices) == 0 || choices[0].Line != "24" || choices[0].declined != "" {
		return nativeInitQualificationExpected{}, fmt.Errorf("native-init profile requires selected Node24")
	}
	environment := map[string]string{}
	for _, entry := range choices[0].imageEnv {
		key, value, valid := strings.Cut(entry, "=")
		if _, duplicate := environment[key]; !valid || !nativeImageEnvName.MatchString(key) || duplicate {
			return nativeInitQualificationExpected{}, fmt.Errorf("invalid selected Node primary ENV")
		}
		environment[key] = value
	}
	node := environment["NODE_VERSION"]
	if !fullVersion.MatchString(node) || !strings.HasPrefix(node, "24.") {
		return nativeInitQualificationExpected{}, fmt.Errorf("selected Node24 exact patch unavailable")
	}
	return nativeInitQualificationExpected{run, node, arch, environment, staged}, nil
}

func nativeInitQualificationValidate(expected nativeInitQualificationExpected, proof nativeInitQualificationProof, drained bool) error {
	actual := proof.Identity
	if proof.Run != expected.Run || actual.Node != expected.Node || actual.Executable != "/usr/local/bin/node" || !reflect.DeepEqual(actual.Env, expected.Env) ||
		actual.Init.SHA256 != expected.Init.SHA256 || actual.Init.Machine != expected.Init.Machine || !actual.Init.Executable || actual.Init.PID1 != nativeInitPath {
		return fmt.Errorf("actual primary/PID1 identity differs from selected Node/native-init")
	}
	if drained && (proof.Phase != "runtime" || !proof.Pass || proof.Holds != 1 || proof.Completed != 1 || proof.Signals != 1 ||
		!slices.Equal(proof.Events, []string{"listening", "hold_begin", "SIGTERM", "hold_end", "closed"})) {
		return fmt.Errorf("held request did not drain after exactly one SIGTERM before exit")
	}
	return nil
}

// This extra profile has a localhost HTTP transport on explicit bridge networking.
// Its canonical authored build stays network-off; generic integration skips it.
func TestCommonNativeInitQualification(t *testing.T) {
	if os.Getenv("BEX_NATIVE_TEST_INIT_PROFILE") != "1" {
		t.Skip("set BEX_NATIVE_TEST_INIT_PROFILE=1 for separate native-init qualification")
	}
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") != "1" {
		t.Fatal("native-init profile also requires BEX_BUILDKIT_INTEGRATION=1")
	}
	common, _, _ := nativeUtilityTestImages(t)
	docker := nativeUtilityDockerConfig(t)
	if common == "" || docker.builder == "" {
		t.Fatal("native-init profile requires an immutable reviewed common and explicit owned builder/platform")
	}
	arch, _, err := nativeInitQualificationTarget(docker.platform)
	must(t, err)
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = common
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		output, err = os.MkdirTemp(requested, "native-common-init-")
		must(t, err)
	}
	t.Logf("retained native-init evidence: %s", output)
	o, files := nativeInitQualificationFixture(t)
	bundle, path, source := writeResolveFixture(t, o, nil, files)
	choices, err := ResolveNativeBuildExact(t.Context(), NewDockerHubResolver(), time.Time{}, nativeNodeRuntime, bundle, path, source)
	must(t, err)
	compiled := compileNativeInitForArch(t, arch)
	before, err := nativeInitQualificationELF(compiled, docker.platform)
	must(t, err)
	initDirectory := filepath.Join(output, "init")
	stagedPath := filepath.Join(initDirectory, nativeInitFile)
	must(t, StageNativeInit(compiled, stagedPath))
	staged, err := nativeInitQualificationELF(stagedPath, docker.platform)
	must(t, err)
	if before != staged {
		t.Fatal("staged native-init differs from the compiled executable")
	}
	run := fmt.Sprintf("bex-init-%d-%d", os.Getpid(), time.Now().UnixNano())
	expected, err := nativeInitQualificationExpect(choices, arch, run, staged)
	must(t, err)
	raw, err := json.MarshalIndent(expected, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(source, "expected.json"), raw, 0o600))
	recipe, err := os.ReadFile(path)
	must(t, err)
	must(t, os.WriteFile(filepath.Join(output, "Dockerfile.canonical"), recipe, 0o600))
	must(t, os.Rename(path, filepath.Join(source, "Dockerfile")))
	custody, err := json.MarshalIndent(map[string]any{"common": common, "choices": choices, "expected": expected, "platform": docker.platform,
		"resolver_image_field": o.NativeResolverImage, "resolver_image_role": "test-only nativeInitOptions staging sentinel; not queried, executed or claimed as image provenance",
		"named_context": nativeInitContext + "=" + initDirectory, "build_network": "none", "runtime_network": "bridge; published127.0.0.1 ephemeral port only",
		"scope": "separate Node24/common/native-init local drain profile; original13/28 and first cohort unchanged; no hosted/Kubernetes/publication claim"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(output, "source-custody.json"), custody, 0o600))
	tag := run + ":qualification"
	t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	args := docker.buildArgs([]string{"--build-context", nativeInitContext + "=" + initDirectory, "--secret", "id=render-env,src=" + bundle, "-t", tag}, source, true)
	log, buildErr := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	must(t, os.WriteFile(filepath.Join(output, "build.log"), log, 0o600))
	if buildErr != nil {
		t.Fatalf("canonical Node24 native-init build: %v\n%s", buildErr, log)
	}
	image, err := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{.Id}}\n{{.Os}}/{{.Architecture}}\n{{json .Config.Entrypoint}}", tag).Output()
	must(t, err)
	must(t, os.WriteFile(filepath.Join(output, "image-identity.txt"), image, 0o600))
	identity := strings.Split(strings.TrimSpace(string(image)), "\n")
	if len(identity) != 3 || !digestPattern.MatchString(identity[0]) || identity[1] != docker.platform || identity[2] != `["/native-init"]` {
		t.Fatalf("built platform/entrypoint differs from requested native-init image: %s", image)
	}
	for _, launch := range []struct {
		name    string
		command []string
	}{{"npm", nil}, {"yarn", []string{"/bin/bash", "-c", "yarn start"}}, {"pnpm", []string{"/bin/bash", "-c", "pnpm start"}}, {"direct", []string{"/bin/bash", "-c", "node server.js"}}} {
		if !t.Run(launch.name, func(t *testing.T) {
			nativeInitQualificationDrain(t, docker, identity[0], expected, output, launch.name, launch.command)
		}) {
			break
		}
	}
}

type nativeInitQualificationContainer struct {
	ID, Image, Network string
	Running            bool
	ExitCode           int
	Labels             map[string]string
	Ports              map[string][]struct{ HostIP, HostPort string }
}

const nativeInitQualificationInspect = `{"ID":{{json .Id}},"Image":{{json .Image}},"Network":{{json .HostConfig.NetworkMode}},"Running":{{json .State.Running}},"ExitCode":{{json .State.ExitCode}},"Labels":{{json .Config.Labels}},"Ports":{{json .NetworkSettings.Ports}}}`

func nativeInitQualificationAddress(state nativeInitQualificationContainer, id, image, owner string) (string, error) {
	ports := state.Ports["10000/tcp"]
	if state.ID != id || state.Image != image || state.Labels[nativeUtilityOwnerLabel] != owner || state.Network != "bridge" || !state.Running ||
		len(state.Ports) != 1 || len(ports) != 1 || ports[0].HostIP != "127.0.0.1" {
		return "", fmt.Errorf("owned drain container must expose exactly one localhost bridge port")
	}
	port, err := strconv.Atoi(ports[0].HostPort)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid ephemeral localhost port")
	}
	return "http://" + net.JoinHostPort("127.0.0.1", ports[0].HostPort), nil
}

func nativeInitQualificationExit(launch string, status int) bool {
	// native-init preserves a wrapper's SIGTERM status while orphaned Node drains.
	return status == 0 || (launch != "direct" && status == 143)
}

func nativeInitQualificationDrain(t *testing.T, docker nativeUtilityDocker, image string, expected nativeInitQualificationExpected, output, launch string, command []string) {
	t.Helper()
	evidence := filepath.Join(output, launch)
	must(t, os.Mkdir(evidence, 0o700))
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	run := func(name string, args ...string) ([]byte, error) {
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		must(t, os.WriteFile(filepath.Join(evidence, name+".log"), out, 0o600))
		return out, err
	}
	name := expected.Run + "-" + launch
	intent, err := json.MarshalIndent(map[string]any{"name": name, "image": image, "platform": docker.platform, "owner": docker.owner, "network": "bridge", "publication": "127.0.0.1::10000", "state": "before one create; ambiguous failure must be reconciled, never retried"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "create-intent.json"), intent, 0o600))
	args := make([]string, 0, 20+len(command))
	args = append(args, "create", "--name", name, "--network", "bridge", "--platform", docker.platform, "--label", docker.label(), "--publish", "127.0.0.1::10000", "--stop-timeout", "20", "--memory", "512m", "--cpus", "1", "--pids-limit", "128", image)
	create := exec.CommandContext(ctx, "docker", append(args, command...)...)
	var createStderr bytes.Buffer
	create.Stderr = &createStderr
	created, createErr := create.Output()
	must(t, os.WriteFile(filepath.Join(evidence, "create.stdout"), created, 0o600))
	must(t, os.WriteFile(filepath.Join(evidence, "create.stderr"), createStderr.Bytes(), 0o600))
	id := strings.TrimSpace(string(created))
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(id) {
		t.Fatalf("container create returned no exact owned ID; preserve intent, no retry: %v %q %s", createErr, created, &createStderr)
	}
	t.Cleanup(func() {
		logsCtx, stopLogs := context.WithTimeout(context.Background(), 20*time.Second)
		logs, _ := exec.CommandContext(logsCtx, "docker", "logs", "--tail", "100", id).CombinedOutput()
		stopLogs()
		must(t, os.WriteFile(filepath.Join(evidence, "cleanup-runtime.log"), logs, 0o600))
		nativeUtilityCleanup("rm", "-f", id)
		cleanupCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		check, inspectErr := exec.CommandContext(cleanupCtx, "docker", "inspect", "--format", "{{.Id}}", id).CombinedOutput()
		gone := inspectErr != nil && strings.Contains(strings.ToLower(string(check)), "no such object: "+id)
		receipt, marshalErr := json.MarshalIndent(map[string]any{"container_id": id, "owner": docker.owner, "absence_proved": gone, "inspection": string(check), "scope": "exact own returned ID only"}, "", "  ")
		must(t, marshalErr)
		must(t, os.WriteFile(filepath.Join(evidence, "cleanup.json"), receipt, 0o600))
		if !gone {
			t.Errorf("own drain container absence unproved: %v %s", inspectErr, check)
		}
	})
	must(t, createErr)
	_, err = run("start", "start", id)
	must(t, err)
	inspected, err := run("running", "inspect", "--format", nativeInitQualificationInspect, id)
	must(t, err)
	var state nativeInitQualificationContainer
	must(t, json.Unmarshal(inspected, &state))
	address, err := nativeInitQualificationAddress(state, id, image, docker.owner)
	must(t, err)
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(path string) ([]byte, int, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, address+path, nil)
		if err != nil {
			return nil, 0, err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, 0, err
		}
		defer func() { _ = response.Body.Close() }()
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if len(body) > 65536 {
			return nil, 0, fmt.Errorf("drain fixture response exceeds64KiB")
		}
		return body, response.StatusCode, err
	}
	wait := func(path string, condition func(nativeInitQualificationProof) bool) nativeInitQualificationProof {
		deadline := time.Now().Add(30 * time.Second)
		for {
			raw, status, readErr := get(path)
			var proof nativeInitQualificationProof
			if readErr == nil && status == 200 && json.Unmarshal(raw, &proof) == nil && nativeInitQualificationValidate(expected, proof, false) == nil && condition(proof) {
				return proof
			}
			if time.Now().After(deadline) || ctx.Err() != nil {
				t.Fatalf("fixture precondition %s unavailable before deadline: %v %s", path, readErr, raw)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	wait("/ready", func(proof nativeInitQualificationProof) bool {
		return slices.Equal(proof.Events, []string{"listening"})
	})
	type result struct {
		body   string
		status int
		err    error
	}
	held := make(chan result, 1)
	go func() {
		// The held response lasts4s; it must outlive the termination signal.
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, address+"/hold", nil)
		if err != nil {
			held <- result{err: err}
			return
		}
		heldClient := *client
		heldClient.Timeout = 30 * time.Second
		response, err := heldClient.Do(request)
		if err != nil {
			held <- result{err: err}
			return
		}
		defer func() { _ = response.Body.Close() }()
		body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		held <- result{string(body), response.StatusCode, err}
	}()
	before := wait("/state", func(proof nativeInitQualificationProof) bool {
		return slices.Equal(proof.Events, []string{"listening", "hold_begin"})
	})
	_, err = run("stop", "stop", "--time", "20", id)
	must(t, err)
	var response result
	select {
	case response = <-held:
	case <-ctx.Done():
		t.Fatal("held request did not finish before drain context expired")
	}
	if response.err != nil || response.status != 200 || response.body != "held 雪 λ🙂" {
		t.Fatalf("in-flight response was lost: %+v", response)
	}
	logs, err := run("runtime", "logs", id)
	must(t, err)
	if !strings.Contains(string(logs), `"pass": true`) {
		t.Fatal("runtime did not emit a passing drain proof")
	}
	_, err = run("export", "cp", id+":/opt/render/project/src/qa-output", filepath.Join(evidence, "qa-output"))
	must(t, err)
	raw, err := os.ReadFile(filepath.Join(evidence, "qa-output", "runtime.json"))
	must(t, err)
	var proof nativeInitQualificationProof
	must(t, json.Unmarshal(raw, &proof))
	must(t, nativeInitQualificationValidate(expected, proof, true))
	inspected, err = run("stopped", "inspect", "--format", nativeInitQualificationInspect, id)
	must(t, err)
	must(t, json.Unmarshal(inspected, &state))
	if state.ID != id || state.Image != image || state.Running || !nativeInitQualificationExit(launch, state.ExitCode) || state.Network != "bridge" || state.Labels[nativeUtilityOwnerLabel] != docker.owner {
		t.Fatalf("owned drain container did not preserve a successful/terminated wrapper status: %s", inspected)
	}
	receipt, err := json.MarshalIndent(map[string]any{"launch": launch, "command_override": command, "container": state, "platform": docker.platform, "owner": docker.owner, "common": nativeCommonBase, "pre_stop": before, "response": response.body, "proof": proof, "network_scope": "bridge with127.0.0.1-only ephemeral publication; build remains network=none", "pass": true}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "receipt.json"), receipt, 0o600))
}

func TestNativeInitQualificationELFAndStagingControls(t *testing.T) {
	for _, platform := range []string{"linux/amd64", "linux/arm64"} {
		t.Run(platform, func(t *testing.T) {
			_, machine, err := nativeInitQualificationTarget(platform)
			must(t, err)
			data := make([]byte, 64, 65)
			copy(data, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
			binary.LittleEndian.PutUint16(data[16:], uint16(elf.ET_EXEC))
			binary.LittleEndian.PutUint16(data[18:], uint16(machine))
			binary.LittleEndian.PutUint32(data[20:], 1)
			binary.LittleEndian.PutUint16(data[52:], 64)
			source := filepath.Join(t.TempDir(), nativeInitFile)
			must(t, os.WriteFile(source, data, 0o755))
			before, err := nativeInitQualificationELF(source, platform)
			must(t, err)
			staged := filepath.Join(t.TempDir(), "init", nativeInitFile)
			must(t, StageNativeInit(source, staged))
			after, err := nativeInitQualificationELF(staged, platform)
			must(t, err)
			if before != after {
				t.Fatal("StageNativeInit changed synthetic ELF bytes/identity")
			}
			other := "linux/arm64"
			if platform == other {
				other = "linux/amd64"
			}
			if _, err := nativeInitQualificationELF(staged, other); err == nil {
				t.Fatal("ELF for the wrong requested platform was accepted")
			}
			must(t, os.Chmod(staged, 0o644))
			if _, err := nativeInitQualificationELF(staged, platform); err == nil {
				t.Fatal("nonexecutable native-init accepted")
			}
			must(t, os.Chmod(staged, 0o755))
			must(t, os.WriteFile(staged, []byte("not an ELF"), 0o755))
			if _, err := nativeInitQualificationELF(staged, platform); err == nil {
				t.Fatal("invalid ELF accepted")
			}
			data = append(data, 0)
			must(t, os.WriteFile(staged, data, 0o755))
			changed, err := nativeInitQualificationELF(staged, platform)
			must(t, err)
			if changed.SHA256 == before.SHA256 {
				t.Fatal("different executable bytes retained their digest")
			}
		})
	}
	for _, invalid := range []string{"", "linux/386", "darwin/arm64", "arm64"} {
		if _, _, err := nativeInitQualificationTarget(invalid); err == nil {
			t.Fatal("unrequested platform accepted")
		}
	}
}

func TestNativeInitQualificationCanonicalRecipe(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	o, files := nativeInitQualificationFixture(t)
	fake := &fakeExact{envByImage: map[string]fakeExactMetadata{}}
	for runtime, toolchain := range nativeToolchains {
		for _, line := range toolchain.lines {
			fake.envByImage[line.image] = commonReviewedFixtureMetadata(runtime, line)
		}
	}
	bundle, path, source := writeResolveFixture(t, o, nil, files)
	choices, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, nativeNodeRuntime, bundle, path, source)
	must(t, err)
	expected, err := nativeInitQualificationExpect(choices, "arm64", "synthetic-only", nativeInitQualificationBinary{})
	must(t, err)
	if expected.Env["NODE_VERSION"] != expected.Node || files[".node-version"] != "24\n" || o.RuntimeEnvSecret != "" || len(o.BuildEnv) != 0 {
		t.Fatal("fixture selected or overrode a different primary version")
	}
	recipe, err := os.ReadFile(path)
	must(t, err)
	for _, required := range []string{"\nFROM " + commonFixtureImage + "\n", "COPY --link --from=bex-init /native-init /\nENTRYPOINT [\"/native-init\"]", shellJSON(nativeEnvLoaderPrefix + "node build.js"), "CMD " + shellJSON("npm start")} {
		if !strings.Contains(string(recipe), required) {
			t.Fatalf("canonical recipe lost %q", required)
		}
	}
	docker := nativeUtilityDocker{"own-builder", "linux/arm64", "own-builder"}
	args := docker.buildArgs([]string{"--build-context", "bex-init=/own/staged/init", "--secret", "id=render-env,src=" + bundle}, source, true)
	if !strings.Contains(strings.Join(args, " "), "--builder own-builder --platform linux/arm64 --load --network none") || !slices.Contains(args, "bex-init=/own/staged/init") {
		t.Fatalf("requested platform/named context lost: %v", args)
	}
	for _, choices := range [][]NativeChoice{nil, {{Line: "22"}}, {{Line: "24", imageEnv: []string{"NODE_VERSION=24"}}}, {{Line: "24", imageEnv: []string{"NODE_VERSION=24.1.0", "NODE_VERSION=24.1.0"}}}} {
		if _, err := nativeInitQualificationExpect(choices, "arm64", "synthetic-only", nativeInitQualificationBinary{}); err == nil {
			t.Fatal("unbound/changed primary metadata accepted")
		}
	}
}

func TestNativeInitQualificationDrainAndTransportControls(t *testing.T) {
	for _, test := range []struct {
		launch string
		status int
		accept bool
	}{{"direct", 0, true}, {"direct", 143, false}, {"npm", 143, true}, {"yarn", 143, true}, {"pnpm", 143, true}, {"pnpm", 0, true}, {"npm", 1, false}, {"yarn", 137, false}} {
		if nativeInitQualificationExit(test.launch, test.status) != test.accept {
			t.Fatalf("unexpected stopped-wrapper status policy: %+v", test)
		}
	}
	expected := nativeInitQualificationExpected{Run: "synthetic", Node: "24.1.0", Env: map[string]string{"NODE_VERSION": "24.1.0"}, Init: nativeInitQualificationBinary{SHA256: strings.Repeat("a", 64), Machine: 183, Executable: true}}
	proof := nativeInitQualificationProof{Phase: "runtime", Run: expected.Run, Events: []string{"listening", "hold_begin", "SIGTERM", "hold_end", "closed"}, Holds: 1, Completed: 1, Signals: 1, Pass: true}
	proof.Identity.Node, proof.Identity.Executable, proof.Identity.Env = expected.Node, "/usr/local/bin/node", expected.Env
	proof.Identity.Init.SHA256, proof.Identity.Init.Machine, proof.Identity.Init.Executable, proof.Identity.Init.PID1 = expected.Init.SHA256, 183, true, nativeInitPath
	must(t, nativeInitQualificationValidate(expected, proof, true))
	for _, mutate := range []func(*nativeInitQualificationProof){
		func(p *nativeInitQualificationProof) { p.Run = "foreign" },
		func(p *nativeInitQualificationProof) { p.Identity.Node = "22.1.0" },
		func(p *nativeInitQualificationProof) { p.Identity.Init.SHA256 = strings.Repeat("b", 64) },
		func(p *nativeInitQualificationProof) { p.Identity.Init.Machine = 62 },
		func(p *nativeInitQualificationProof) { p.Identity.Init.PID1 = "/bin/bash" },
		func(p *nativeInitQualificationProof) { p.Identity.Init.Executable = false },
		func(p *nativeInitQualificationProof) { p.Identity.Env = map[string]string{"NODE_VERSION": "24"} },
		func(p *nativeInitQualificationProof) {
			p.Events = []string{"listening", "hold_begin", "hold_end", "SIGTERM", "closed"}
		},
		func(p *nativeInitQualificationProof) { p.Signals = 2 },
		func(p *nativeInitQualificationProof) { p.Completed = 0 },
		func(p *nativeInitQualificationProof) { p.Pass = false },
	} {
		bad := proof
		mutate(&bad)
		if nativeInitQualificationValidate(expected, bad, true) == nil {
			t.Fatal("changed identity or incomplete drain accepted")
		}
	}
	var state nativeInitQualificationContainer
	must(t, json.Unmarshal([]byte(`{"ID":"own","Image":"exact","Network":"bridge","Running":true,"Labels":{"co.bex.native-qualification":"owner"},"Ports":{"10000/tcp":[{"HostIp":"127.0.0.1","HostPort":"49152"}]}}`), &state))
	address, err := nativeInitQualificationAddress(state, "own", "exact", "owner")
	must(t, err)
	if address != "http://127.0.0.1:49152" {
		t.Fatalf("unexpected localhost address %s", address)
	}
	for _, mutate := range []func(*nativeInitQualificationContainer){
		func(s *nativeInitQualificationContainer) { s.ID = "foreign" },
		func(s *nativeInitQualificationContainer) { s.Image = "changed" },
		func(s *nativeInitQualificationContainer) { s.Network = "host" },
		func(s *nativeInitQualificationContainer) { s.Running = false },
		func(s *nativeInitQualificationContainer) { s.Labels = map[string]string{} },
		func(s *nativeInitQualificationContainer) { s.Ports = nil },
	} {
		bad := state
		mutate(&bad)
		if _, err := nativeInitQualificationAddress(bad, "own", "exact", "owner"); err == nil {
			t.Fatal("foreign or nonlocalhost transport accepted")
		}
	}
	state.Ports["10000/tcp"][0].HostIP = "0.0.0.0"
	if _, err := nativeInitQualificationAddress(state, "own", "exact", "owner"); err == nil {
		t.Fatal("wildcard publication accepted")
	}
}
