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
	_ "embed"
	"strings"
)

// This same script belongs to the deploy/native-common Docker context. Go
// embed cannot reach outside its package; node14_tools_test.py guards the two
// copies byte-for-byte so historical and common images run one implementation.
//
//go:embed nativenodepython.py
var nativeNodePythonHelper string

func nativeNodeAddonPythonLines(line string, helperInBase bool) (string, error) {
	if !nativeLegacyPnpmLine(line) {
		return "", nil
	}
	python, source, err := commonNodeBuildPython()
	if err != nil {
		return "", err
	}
	lines := "COPY --link --from=" + source + " /usr/local/ /opt/bex/native-node-python/\n"
	if !helperInBase {
		lines += "COPY <<'BEX_NODE_PYTHON' /usr/libexec/bex-native/prepare-node14-tools.py\n" +
			nativeNodePythonHelper + "BEX_NODE_PYTHON\n"
	}
	return lines + "RUN --network=none /opt/bex/native-node-python/bin/python3.9 -I /usr/libexec/bex-native/prepare-node14-tools.py " + python.version + "\n", nil
}

// Raw Bullseye companions already have compatible Python. A Bookworm donor
// must not be copied onto that older libc; the common base handles its own
// reviewed legacy trees separately.
func nativeHistoricalNodePythonLines(hostImage, line string) (string, error) {
	if nativeCommonBase != "" || !nativeLegacyPnpmLine(line) || !strings.Contains(hostImage, "-bookworm@") {
		return "", nil
	}
	return nativeNodeAddonPythonLines(line, false)
}
