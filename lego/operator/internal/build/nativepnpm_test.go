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
	"os"
	"strings"
	"testing"
)

// w8/226: pnpm 10's JavaScript entrypoint refuses Node < 18.12, so a Node 14
// or 16 selection — primary or companion — carries the standalone build of the
// same release instead, and every other line keeps the package unchanged.
func TestNativePnpmFollowsTheSelectedNodeLine(t *testing.T) {
	for _, runtime := range []string{nativeNodeRuntime, nativePythonRuntime} {
		for _, line := range []string{"14", "16", "18", "20", "22", "24"} {
			o := nativeOptions()
			o.Runtime = runtime
			bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"NODE_VERSION": line}, nil)
			if _, err := resolveNativeBuild(runtime, bundle, dockerfile, src); err != nil {
				t.Fatalf("%s NODE_VERSION=%s: %v", runtime, line, err)
			}
			df, _ := os.ReadFile(dockerfile)
			legacy := strings.Count(string(df), nativeLegacyPnpmInstall())
			stages := strings.Count(string(df), nativeLegacyPnpmStages())
			pkg := strings.Count(string(df), nativePnpmInstall())
			want := line == "14" || line == "16"
			if (want && (legacy != 1 || stages != 1 || pkg != 0)) || (!want && (legacy != 0 || stages != 0 || pkg != 1)) {
				t.Fatalf("%s NODE_VERSION=%s: standalone=%d stages=%d package=%d\n%s", runtime, line, legacy, stages, pkg, df)
			}
		}
	}
}

func TestNativeLegacyPnpmIsFetchedByChecksumPerArchitecture(t *testing.T) {
	stages := nativeLegacyPnpmStages()
	for _, want := range []string{
		"FROM scratch AS bex-pnpm-amd64\nADD --checksum=" + nativeLegacyPnpm.amd64 + " https://registry.npmjs.org/@pnpm/linux-x64/-/linux-x64-" + nativePnpm.version + ".tgz",
		"FROM scratch AS bex-pnpm-arm64\nADD --checksum=" + nativeLegacyPnpm.arm64 + " https://registry.npmjs.org/@pnpm/linux-arm64/-/linux-arm64-" + nativePnpm.version + ".tgz",
		"FROM bex-pnpm-${TARGETARCH} AS bex-pnpm-tarball",
		"FROM " + nativePreparerImage + " AS bex-pnpm",
	} {
		if !strings.Contains(stages, want) {
			t.Fatalf("legacy pnpm stages lack %q:\n%s", want, stages)
		}
	}
	// No download inside the image build, and no build ARG reaches the
	// authored build's environment.
	lines := nativeLegacyPnpmInstall()
	if !strings.HasPrefix(lines, "COPY --link --from=bex-pnpm ") || strings.Contains(lines, "RUN ") || strings.Contains(lines, "ARG ") {
		t.Fatalf("legacy pnpm install must only copy:\n%s", lines)
	}
}
