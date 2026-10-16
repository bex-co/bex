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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A primary Node serving image defaults NODE_ENV=production after its build
// RUN (w8/111) on every reviewed line; a static image, other primary runtimes
// (whose Node companion stays as it was) and the build itself get nothing.
func TestNativeNodeServingImageDefaultsProduction(t *testing.T) {
	const want = "ENV NODE_ENV=production\n"
	o := nativeOptions()
	for _, line := range nativeToolchains[nativeNodeRuntime].lines {
		bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"NODE_VERSION": line.line}, nil)
		if _, err := ResolveNativeBuild(nativeNodeRuntime, bundle, dockerfile, src); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(dockerfile)
		if err != nil {
			t.Fatal(err)
		}
		df := string(raw)
		at, run, cmd := strings.Index(df, "\n"+want), strings.Index(df, "\nRUN --mount=type=secret,id=render-env"), strings.Index(df, "\nCMD ")
		if strings.Count(df, want) != 1 || at < run || at > cmd {
			t.Fatalf("node %s: NODE_ENV default is not once between the build RUN and CMD:\n%s", line.line, df)
		}
	}
	static := nativeOptions()
	static.StaticSite, static.Runtime, static.StartCommand = true, "", ""
	if df := nativeDockerfile(static); strings.Contains(df, "NODE_ENV=production") {
		t.Errorf("static image gained the Node default:\n%s", df)
	}
	// Python and Ruby gain their Node companion in the resolve phase; the
	// default must not ride along.
	for _, runtime := range []string{"python", "ruby"} {
		other := nativeOptions()
		other.Runtime = runtime
		bundle, dockerfile, src := writeResolveFixture(t, other, nil, nil)
		if _, err := ResolveNativeBuild(runtime, bundle, dockerfile, src); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(dockerfile)
		if err != nil {
			t.Fatal(err)
		}
		if df := string(raw); !strings.Contains(df, "/usr/local/bin/node") || strings.Contains(df, "NODE_ENV=production") {
			t.Errorf("%s: want the Node companion and no Node default:\n%s", runtime, df)
		}
	}
}

// TestNativeNodeImageRuntimeEnvironment is w8/111's execution gate: the
// generated Node image builds with NODE_ENV unset and starts in production by
// default, while a value the service configures — staging, or explicitly
// empty, as a literal or from an env file the way a linked Secret arrives —
// reaches the process exactly. Opt-in: needs Docker.
func TestNativeNodeImageRuntimeEnvironment(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	o := nativeOptions()
	report := `node -e 'console.log(JSON.stringify({present: "NODE_ENV" in process.env, value: process.env.NODE_ENV ?? null}))'`
	o.BuildCommand = report + ` > build.json`
	o.StartCommand = `echo "build $(cat build.json) runtime $(` + report + `)"`
	o.BuildEnv = nil
	tag := buildResolvedImage(t, o, nil, nil, fmt.Sprintf("bex-native-node-env-it:%d", os.Getpid()))
	envFile := filepath.Join(t.TempDir(), "secret.env")
	must(t, os.WriteFile(envFile, []byte("NODE_ENV=\n"), 0o600))
	for _, tc := range []struct {
		name    string
		args    []string
		runtime string
	}{
		{"default", nil, `{"present":true,"value":"production"}`},
		{"configured literal", []string{"-e", "NODE_ENV=staging"}, `{"present":true,"value":"staging"}`},
		{"explicit empty literal", []string{"-e", "NODE_ENV="}, `{"present":true,"value":""}`},
		{"empty from a Secret env file", []string{"--env-file", envFile}, `{"present":true,"value":""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{"run", "--rm", "--network", "none"}, tc.args...), tag)
			out, err := exec.Command("docker", args...).CombinedOutput()
			want := `build {"present":false,"value":null} runtime ` + tc.runtime
			if err != nil || !strings.Contains(string(out), want) {
				t.Fatalf("run = %v\n%s; want %q", err, out, want)
			}
		})
	}
}
