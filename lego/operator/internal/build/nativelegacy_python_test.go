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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacyPoetrySolve = "poetry==2.1.3 'virtualenv<20.33.0'"

func nativeLegacyPythonProfileEnabled() bool {
	return os.Getenv("BEX_BUILDKIT_INTEGRATION") != "" && os.Getenv("BEX_NATIVE_LEGACY_PYTHON_INTEGRATION") == "1"
}

// The default legacy tool uses only the common image's system Python. Prepare
// that one closure once for both app versions, never cp313/cp314 wheels. Then
// exercise the unchanged canonical tool bootstrap against it without network.
func nativeLegacyPythonWheelRecipe(canonical, common string) (string, error) {
	if err := validateNativeBase(common); err != nil {
		return "", err
	}
	if strings.Count(canonical, "\nFROM "+common+"\n") != 1 || strings.Count(canonical, "\nCOPY . .\n") != 1 {
		return "", fmt.Errorf("legacy wheels require one canonical common base/source boundary")
	}
	var bootstrap string
	for line := range strings.SplitSeq(canonical, "\n") {
		if strings.HasPrefix(line, "RUN /usr/bin/python3 -m venv /opt/bex/poetry && ") {
			if bootstrap != "" || !strings.Contains(line, "--no-input "+legacyPoetrySolve+" && ") {
				return "", fmt.Errorf("legacy wheels require the canonical default Poetry dependency solve")
			}
			bootstrap = line
		}
	}
	if bootstrap == "" {
		return "", fmt.Errorf("legacy wheels require system-Python default Poetry")
	}
	return "# syntax=docker/dockerfile:1.7\nFROM " + common + " AS bex-test-prerequisites\n" +
		"RUN --network=default /usr/bin/python3 -m venv /opt/bex/test-wheel-builder && /opt/bex/test-wheel-builder/bin/pip wheel --disable-pip-version-check --only-binary=:all: --wheel-dir /opt/bex-test-wheels " + legacyPoetrySolve + "\n" +
		"ENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n" +
		strings.Replace(bootstrap, "RUN ", "RUN --network=none ", 1) + "\n" +
		"COPY check-tool.py /opt/bex-test-check-tool.py\n" +
		"RUN --network=none /opt/bex/poetry/bin/python -I /opt/bex-test-check-tool.py > /opt/bex-test-tool-python.json\n" +
		"FROM scratch\nCOPY --from=bex-test-prerequisites /opt/bex-test-wheels/ /wheels/\n" +
		"COPY --from=bex-test-prerequisites /opt/bex-test-tool-python.json /tool-python.json\n", nil
}

func nativeLegacyWheelPlatform(data []byte, platform string) error {
	var actual struct {
		Python, Poetry, OS, Machine string
		BasePrefix                  string `json:"base_prefix"`
		Prefix                      string
		Cap                         bool `json:"virtualenv_cap_satisfied"`
	}
	if err := json.Unmarshal(data, &actual); err != nil {
		return err
	}
	arch := map[string]string{"linux/amd64": "x86_64", "linux/arm64": "aarch64"}[platform]
	if arch == "" || actual.Machine != arch || actual.OS != "Linux" || !strings.HasPrefix(actual.Python, "3.11.") ||
		actual.BasePrefix != "/usr" || actual.Prefix != "/opt/bex/poetry" || actual.Poetry != "2.1.3" || !actual.Cap {
		return fmt.Errorf("legacy wheel closure does not match the common system Python/tool/platform")
	}
	return nil
}

func nativeLegacyPythonFiles(t *testing.T, version string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, name := range []string{"pyproject.toml", "setup.py", "legacy_probe.c", "check.py", "check-tool.py"} {
		data, err := os.ReadFile(filepath.Join("testdata", "native-legacy-python", name))
		must(t, err)
		files[name] = strings.ReplaceAll(string(data), "@APP_VERSION@", version)
	}
	return files
}

// This extra profile is never selected by the existing first-cohort workflow
// or by a generic BEX_BUILDKIT_INTEGRATION invocation without its own opt-in.
func TestNativeLegacyPythonGeneratedImages(t *testing.T) {
	if !nativeLegacyPythonProfileEnabled() {
		t.Skip("set BEX_BUILDKIT_INTEGRATION and BEX_NATIVE_LEGACY_PYTHON_INTEGRATION=1 for the separate legacy family profile")
	}
	common, _, _ := nativeUtilityTestImages(t)
	docker := nativeUtilityDockerConfig(t)
	if common == "" || docker.builder == "" {
		t.Fatal("legacy profile requires an immutable common image and explicitly owned builder/platform")
	}
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = common
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		var err error
		output, err = os.MkdirTemp(requested, "native-legacy-python-")
		must(t, err)
	}
	t.Logf("retained legacy family evidence: %s", output)
	inputs, err := json.MarshalIndent(map[string]any{"common": common, "builder": docker.builder, "platform": docker.platform,
		"owner": docker.owner, "profile": "legacy-python37-python38", "tool_solve": legacyPoetrySolve,
		"scope": "opt-in generated images only; original13/28 and first-cohort selector unchanged"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(output, "inputs.json"), inputs, 0o600))
	wheelExport := ""
	for _, line := range []string{"3.7", "3.8"} {
		if !t.Run(line, func(t *testing.T) {
			selected, found := nativeToolchains[nativePythonRuntime].find(line)
			if !found {
				t.Fatal("missing reviewed legacy Python choice")
			}
			legacyPythonGeneratedCase(t, docker, common, selected.version, output, &wheelExport)
		}) {
			break // Preserve the failed case before attempting another family.
		}
	}
}

func legacyPythonGeneratedCase(t *testing.T, docker nativeUtilityDocker, common, version, output string, wheelExport *string) {
	t.Helper()
	q := prepareNativePythonQualification(t, "legacy", version, output, nativeLegacyPythonFiles(t, version))
	prepared, err := nativeLegacyPythonWheelRecipe(string(q.canonical), common)
	must(t, err)
	if *wheelExport == "" {
		*wheelExport = nativeUtilityExportWheels(t, docker, prepared, q.bundle, q.source, q.evidence)
	}
	metadata, err := os.ReadFile(filepath.Join(*wheelExport, "tool-python.json"))
	must(t, err)
	must(t, nativeLegacyWheelPlatform(metadata, docker.platform))
	q.run(t, docker, common, prepared, *wheelExport, legacyPoetrySolve, metadata)
}
