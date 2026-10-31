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

var nativeModernPythonLines = []string{"3.9", "3.10", "3.11", "3.12", "3.14"}

const modernPoetrySolve = "poetry==2.1.3"

func nativeModernPythonVersion(version string) bool {
	for _, name := range nativeModernPythonLines {
		line, found := nativeToolchains[nativePythonRuntime].find(name)
		if found && line.version == version {
			return true
		}
	}
	return false
}

// Only Poetry's closure is acquired here. Keep the selected target Python's
// canonical prefix and tool bootstrap; the original 3.13 utility helper's
// Django/pyvips closure stays unchanged and cannot serve Python 3.9.
func nativeModernPythonWheelRecipe(canonical, common, version string) (string, error) {
	if err := validateNativeBase(common); err != nil {
		return "", err
	}
	const boundary = "\nCOPY . .\n"
	marker := "\nFROM " + common + "\n"
	if !nativeModernPythonVersion(version) || strings.Count(canonical, marker) != 1 || strings.Count(canonical, boundary) != 1 ||
		strings.Count(canonical, "\nENV PYTHON_VERSION=\""+version+"\"\n") != 1 {
		return "", fmt.Errorf("modern wheels require one canonical selected runtime/base/source boundary")
	}
	prefix, _, _ := strings.Cut(canonical, boundary)
	const bootstrap = "\nRUN /usr/local/bin/python -m venv /opt/bex/poetry && "
	if strings.Count(prefix, bootstrap) != 1 || !strings.Contains(prefix, "--no-input "+modernPoetrySolve+" && ") || strings.Contains(prefix, "virtualenv<") {
		return "", fmt.Errorf("modern wheels require selected-app Python/default Poetry without the legacy cap")
	}
	prefix = strings.Replace(prefix, marker, "\nFROM "+common+" AS bex-test-prerequisites\n", 1)
	acquire := "\nRUN --network=default /usr/bin/python3 -m pip wheel --disable-pip-version-check --wheel-dir /opt/bex-test-wheels " + modernPoetrySolve + "\n" +
		"ENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n"
	prefix = strings.Replace(prefix, bootstrap, acquire+strings.Replace(bootstrap, "RUN ", "RUN --network=none ", 1), 1)
	return prefix + "\nCOPY check-tool.py /opt/bex-test-check-tool.py\n" +
		"RUN --network=none /opt/bex/poetry/bin/python -I /opt/bex-test-check-tool.py > /opt/bex-test-tool-python.json\n" +
		"FROM scratch\nCOPY --from=bex-test-prerequisites /opt/bex-test-wheels/ /wheels/\n" +
		"COPY --from=bex-test-prerequisites /opt/bex-test-tool-python.json /tool-python.json\n", nil
}

func nativeModernWheelPlatform(data []byte, platform, version string) error {
	var actual struct {
		Python, Poetry, OS, Machine, Prefix, Executable, Virtualenv string
		PythonEnv                                                   string `json:"python_env"`
		BasePrefix                                                  string `json:"base_prefix"`
		Checked                                                     bool   `json:"dependency_check_succeeded"`
	}
	if err := json.Unmarshal(data, &actual); err != nil {
		return err
	}
	arch := map[string]string{"linux/amd64": "x86_64", "linux/arm64": "aarch64"}[platform]
	if !nativeModernPythonVersion(version) || arch == "" || actual.Machine != arch || actual.OS != "Linux" ||
		actual.Python != version || actual.PythonEnv != version || actual.BasePrefix != "/usr/local" || actual.Prefix != "/opt/bex/poetry" ||
		!strings.HasPrefix(actual.Executable, "/opt/bex/poetry/bin/") || actual.Poetry != "2.1.3" || actual.Virtualenv == "" || !actual.Checked {
		return fmt.Errorf("modern wheel closure does not match selected application Python/tool/platform")
	}
	return nil
}

func nativeModernPythonFiles(t *testing.T, version string) map[string]string {
	t.Helper()
	// The unchanged legacy observer already checks selected headers, C import,
	// libpython SONAME, SSL/FFI/Unicode and unforced automatic Poetry projects.
	files := nativeLegacyPythonFiles(t, version)
	raw, err := os.ReadFile(filepath.Join("testdata", "native-modern-python", "check-tool.py"))
	must(t, err)
	files["check-tool.py"] = strings.ReplaceAll(string(raw), "@APP_VERSION@", version)
	files[".node-version"] = "24\n"
	return files
}

// Separate from the original 3.13 utilities and exact-default 3.14.8 gate.
// This profile never joins generic integration or the first-cohort selector.
func TestNativeModernPythonGeneratedImages(t *testing.T) {
	if os.Getenv("BEX_NATIVE_TEST_MODERN_PYTHON_PROFILE") != "1" {
		t.Skip("set BEX_NATIVE_TEST_MODERN_PYTHON_PROFILE=1 for the separate modern Python profile")
	}
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") != "1" {
		t.Fatal("modern Python profile also requires BEX_BUILDKIT_INTEGRATION=1")
	}
	common, _, _ := nativeUtilityTestImages(t)
	docker := nativeUtilityDockerConfig(t)
	if common == "" || docker.builder == "" {
		t.Fatal("modern profile requires an immutable common image and explicitly owned builder/platform")
	}
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = common
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		var err error
		output, err = os.MkdirTemp(requested, "native-modern-python-")
		must(t, err)
	}
	t.Logf("retained modern Python family evidence: %s", output)
	inputs, err := json.MarshalIndent(map[string]any{"common": common, "builder": docker.builder, "platform": docker.platform,
		"owner": docker.owner, "profile": "modern-python39-python310-python311-python312-python314-table", "tool_solve": modernPoetrySolve,
		"shared_application_observer": "unchanged legacy fixture; existing qa-output/legacy-*.json filenames retained",
		"scope":                       "opt-in generated images only; original13/28/default3.14.8/3.13utilities/first-cohort unchanged"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(output, "inputs.json"), inputs, 0o600))
	for _, name := range nativeModernPythonLines {
		if !t.Run(name, func(t *testing.T) {
			selected, found := nativeToolchains[nativePythonRuntime].find(name)
			if !found {
				t.Fatal("missing reviewed modern Python choice")
			}
			modernPythonGeneratedCase(t, docker, common, selected.version, output)
		}) {
			break
		}
	}
}

func modernPythonGeneratedCase(t *testing.T, docker nativeUtilityDocker, common, version, output string) {
	t.Helper()
	q := prepareNativePythonQualification(t, "modern", version, output, nativeModernPythonFiles(t, version))
	prepared, err := nativeModernPythonWheelRecipe(string(q.canonical), common, version)
	must(t, err)
	// Every version/platform prepares its own real wheel closure. Neither the
	// legacy system interpreter's export nor the original 3.13 cache is reused.
	wheels := nativeUtilityExportWheels(t, docker, prepared, q.bundle, q.source, q.evidence)
	metadata, err := os.ReadFile(filepath.Join(wheels, "tool-python.json"))
	must(t, err)
	must(t, nativeModernWheelPlatform(metadata, docker.platform, version))
	q.run(t, docker, common, prepared, wheels, modernPoetrySolve, metadata)
}
