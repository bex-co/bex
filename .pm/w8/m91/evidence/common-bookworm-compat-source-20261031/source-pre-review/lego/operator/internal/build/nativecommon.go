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
	"cmp"
	"context"
	"fmt"
	"strings"
)

func commonNativeRuntimeChoice(ctx context.Context, exact ExactImageResolver, runtime string, choice NativeChoice) (NativeChoice, error) {
	if artifact, ok := nativeRuntimeArtifacts[choice.Image]; ok {
		if err := validateNativeBase(artifact); err != nil {
			return NativeChoice{}, err
		}
		if !strings.HasPrefix(artifact, "ghcr.io/bex-co/bex-cnb-run@") {
			return NativeChoice{}, fmt.Errorf("runtime artifact is outside the reviewed public package")
		}
		choice.primaryImage = artifact
	}
	key, command := "", ""
	switch runtime {
	case nativeNodeRuntime:
		key = "NODE_VERSION"
		command = `/usr/local/bin/node -p 'process.versions.node'`
	case nativePythonRuntime:
		key = "PYTHON_VERSION"
		command = `/usr/local/bin/python3 -I -c 'import sys; print(sys.version.split()[0])'`
	case nativeRubyRuntime:
		key = "RUBY_VERSION"
		command = `/usr/local/bin/ruby -e 'puts RUBY_VERSION'`
	default:
		return choice, nil
	}
	version, err := commonImageVariable(choice.imageEnv, key)
	if err != nil || !fullVersion.MatchString(version) || (version != choice.Line && !strings.HasPrefix(version, choice.Line+".")) {
		return NativeChoice{}, fmt.Errorf("selected %s runtime environment does not match its selected release", runtime)
	}
	choice.runtimeCheck = "RUN --network=none [ \"$(" + command + ")\" = " + version + " ]\n"
	if runtime == nativeNodeRuntime && commonNodeBackport(version) {
		if choice.primaryImage == "" {
			checksum, err := exact.NodeSourceDigest(ctx, version)
			if err != nil {
				return NativeChoice{}, fmt.Errorf("read exact Node source identity: %w", err)
			}
			if !digestPattern.MatchString("sha256:" + checksum) {
				return NativeChoice{}, fmt.Errorf("invalid exact Node source digest")
			}
			choice.primaryImage = "bex-node-bookworm"
			choice.primaryStage = "FROM " + nativeCommonBase + " AS bex-node-bookworm\nCOPY --link --from=" + choice.Image + " /usr/local/ /usr/local/\nRUN /usr/libexec/bex-native/build-node --common-bookworm " + version + " " + checksum + "\n"
		}
		choice.runtimeCheck += "RUN --network=none /usr/local/bin/node /usr/share/bex-security/node-check.js\n"
	}
	if strings.Contains(choice.Image, "-bullseye@") {
		choice.legacySource = (runtime == nativeNodeRuntime && strings.HasPrefix(version, "14.")) ||
			(runtime == nativePythonRuntime && strings.HasPrefix(version, "3.7.")) ||
			(runtime == nativeRubyRuntime && strings.HasPrefix(version, "3.0."))
	}
	return choice, nil
}

func commonNodeBackport(version string) bool {
	major, _, _ := strings.Cut(version, ".")
	return major == "14" || major == "16" || major == "18" || major == "20"
}

func commonRuntimeProvenance(choice NativeChoice) string {
	if !strings.HasPrefix(choice.Image, "node:") || choice.primaryImage == "" {
		return ""
	}
	version, _ := commonImageVariable(choice.imageEnv, "NODE_VERSION")
	if !commonNodeBackport(version) {
		return ""
	}
	return "COPY --link --from=" + choice.primaryImage + " /usr/share/bex-node-brotli /usr/share/bex-node-brotli\n"
}

// Only the three existing legacy families may use their old runtime tree on
// Bookworm. Their distinct libssl1.1/libffi7 SONAMEs are genuine coinstalled
// packages in the common factory; libc and current headers are never copied.
func nativeCommonSourceGuard(source string, legacy, reset bool) string {
	compatible := `[ "$primary_os" = "$VERSION_CODENAME" ]`
	if legacy {
		compatible += ` || { [ "$primary_os" = bullseye ] && [ "$VERSION_CODENAME" = bookworm ]; }`
	}
	lines := "COPY --link --from=" + source + " /usr/lib/os-release /opt/bex-primary-os-release\n" +
		"RUN . /opt/bex-primary-os-release && [ -n \"$VERSION_CODENAME\" ] && primary_os=\"$VERSION_CODENAME\" && . /etc/os-release && [ -n \"$VERSION_CODENAME\" ] && " +
		"{ " + compatible + " || { echo 'exact native primary and supplied base have different Debian distributions' >&2; exit 1; }; } && rm -f /opt/bex-primary-os-release"
	if reset {
		lines += " && rm -rf /usr/local && mkdir -p /usr/local"
	}
	return lines + "\n"
}

func commonNativeJSToolLines(choice NativeChoice) string {
	image := cmp.Or(choice.primaryImage, choice.Image)
	return nativeCommonSourceGuard(image, choice.legacySource, false) + nativeJSToolLines(image) + commonRuntimeProvenance(choice) + "COPY --link --from=" + image + " /usr/local/include/node /usr/local/include/node\n" + choice.runtimeCheck
}

func commonNativeRubyToolLines(choice NativeChoice) string {
	image := cmp.Or(choice.primaryImage, choice.Image)
	return nativeCommonSourceGuard(image, choice.legacySource, false) + nativeRubyToolLines(image) + "RUN --network=none /sbin/ldconfig\n" + choice.runtimeCheck
}

func resolveNativeCompanion(ctx context.Context, exact ExactImageResolver, runtime string, env map[string]string, readFile func(string) ([]byte, bool)) (NativeChoice, string, error) {
	choice, err := ResolveNativeVersion(runtime, env, readFile)
	if err != nil {
		return NativeChoice{}, "", err
	}
	if nativeCommonBase != "" {
		pinned, err := pinNativePrimary(ctx, exact, runtime, choice)
		if err != nil {
			return NativeChoice{}, "", err
		}
		if runtime == nativeRubyRuntime {
			return pinned, commonNativeRubyToolLines(pinned), nil
		}
		return pinned, commonNativeJSToolLines(pinned), nil
	}
	if runtime == nativeRubyRuntime {
		return choice, nativeRubyToolLines(choice.Image), nil
	}
	return choice, nativeJSToolLines(choice.Image), nil
}
