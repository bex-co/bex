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
			if _, err := ResolveNativeBuild(runtime, bundle, dockerfile, src); err != nil {
				t.Fatalf("%s NODE_VERSION=%s: %v", runtime, line, err)
			}
			df, _ := os.ReadFile(dockerfile)
			legacy := strings.Count(string(df), nativeLegacyPnpmInstall())
			pkg := strings.Count(string(df), nativePnpmInstall())
			want := line == "14" || line == "16"
			if (want && (legacy != 1 || pkg != 0)) || (!want && (legacy != 0 || pkg != 1)) {
				t.Fatalf("%s NODE_VERSION=%s: standalone=%d package=%d\n%s", runtime, line, legacy, pkg, df)
			}
		}
	}
}

func TestNativeLegacyPnpmInstallIsPinnedPerArchitecture(t *testing.T) {
	lines := nativeLegacyPnpmInstall()
	for _, want := range []string{
		"amd64) arch=x64 sum=" + strings.TrimPrefix(nativeLegacyPnpm.amd64, "sha256:"),
		"arm64) arch=arm64 sum=" + strings.TrimPrefix(nativeLegacyPnpm.arm64, "sha256:"),
		"linux-$arch-" + nativePnpm.version + ".tgz",
		"sha256sum -c -",
		"exit 1 ;; esac",
	} {
		if !strings.Contains(lines, want) {
			t.Fatalf("legacy pnpm install lacks %q:\n%s", want, lines)
		}
	}
	// A build ARG would reach the authored build's environment.
	if strings.Contains(lines, "ARG ") || strings.Contains(lines, "TARGETARCH") {
		t.Fatalf("legacy pnpm install must not declare build args:\n%s", lines)
	}
}
