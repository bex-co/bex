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
	"slices"
	"strings"
	"testing"
)

// Every native build, on every reviewed toolchain line and for a static site,
// copies the pinned static ffmpeg and ffprobe right after its FROM and before
// the source and the authored build (w8/m82). The layer is the same line
// everywhere, so it is one shared, --link-cached layer, and the resolve
// phase's FROM rewrite and companions keep it in place.
func TestNativeDockerfileCarriesFFmpeg(t *testing.T) {
	const ffmpeg = nativeMediaTools
	runtimes := make([]string, 0, len(nativeToolchains))
	for runtime := range nativeToolchains {
		runtimes = append(runtimes, runtime)
	}
	slices.Sort(runtimes)
	versionEnv := map[string]string{"python": "PYTHON_VERSION", nativeNodeRuntime: "NODE_VERSION", "elixir": "ELIXIR_VERSION"}
	check := func(t *testing.T, name, df, from string) {
		t.Helper()
		at := strings.Index(df, "\n"+ffmpeg)
		if !strings.Contains(df, "\nFROM "+from+"\n") || at < strings.Index(df, "\nFROM ") || at > strings.Index(df, "\nCOPY . .\n") ||
			strings.Count(df, ffmpeg) != 1 {
			t.Fatalf("%s: ffmpeg is not copied once between FROM and the source:\n%s", name, df)
		}
	}
	for _, runtime := range runtimes {
		o := nativeOptions()
		o.Runtime = runtime
		check(t, runtime, nativeDockerfile(o), nativeToolchains[runtime].defaultImage())
		for _, line := range nativeToolchains[runtime].lines {
			env := map[string]string{}
			if key := versionEnv[runtime]; key != "" {
				env[key] = line.line
			}
			files := map[string]string{"uv.lock": "", "bun.lock": ""}
			if runtime == "ruby" {
				files[".ruby-version"] = line.version
			}
			bundle, dockerfile, src := writeResolveFixture(t, o, env, files)
			if _, err := ResolveNativeBuild(runtime, bundle, dockerfile, src); err != nil {
				t.Fatalf("%s %s: %v", runtime, line.line, err)
			}
			df, err := os.ReadFile(dockerfile)
			if err != nil {
				t.Fatal(err)
			}
			check(t, runtime+" "+line.line+" resolved", string(df), line.image)
		}
	}
	static := nativeOptions()
	static.StaticSite, static.Runtime, static.StartCommand = true, "", ""
	check(t, "static site", nativeDockerfile(static), nativeToolchains[nativeNodeRuntime].defaultImage())
}

// TestNativeImageRunsFFmpeg is w8/m82's execution gate: on the Python 3.12
// line (the reproduced failure, through Pydub too) and on Node 24, the
// authored build writes a WAV, encodes it to MP3 with ffmpeg and checks it
// with ffprobe, and the started image, with the network off, decodes that MP3
// again from the line's own runtime and probes the result — so ffmpeg, its MP3
// codec and ffprobe work at build and at runtime on PATH, beside the line's
// toolchain. Opt-in: needs Docker and network for PyPI.
func TestNativeImageRunsFFmpeg(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	const encode = `ffmpeg -hide_banner -loglevel error -f lavfi -i sine=frequency=440:duration=1 -ar 44100 tone.wav` +
		` && ffmpeg -hide_banner -loglevel error -i tone.wav -codec:a libmp3lame -b:a 128k tone.mp3` +
		` && echo "built $(ffprobe -v error -show_entries stream=codec_name,sample_rate -of csv=p=0 tone.mp3)" > built.txt`
	// The reproduced application: Pydub's WAV→MP3 export, which shells out to
	// ffmpeg (and reads the result back through ffprobe's mediainfo).
	const pydub = ` && pip install --no-cache-dir --disable-pip-version-check -q pydub==0.25.1` +
		` && python -c 'from pydub import AudioSegment, utils; AudioSegment.from_wav("tone.wav").export("pydub.mp3", format="mp3"); print("pydub", utils.mediainfo("pydub.mp3")["codec_name"])' >> built.txt`
	for _, tc := range []struct{ runtime, env, line, build, decode string }{
		{"python", "PYTHON_VERSION", "3.12", encode + pydub, `python -c 'import subprocess; subprocess.run(["ffmpeg","-hide_banner","-loglevel","error","-y","-i","tone.mp3","back.wav"], check=True); print("runtime", subprocess.run(["ffprobe","-v","error","-show_entries","stream=codec_name","-of","csv=p=0","back.wav"], check=True, capture_output=True, text=True).stdout.strip())'`},
		{"node", "NODE_VERSION", "24", encode, `node -e 'const {execFileSync: x} = require("node:child_process"); x("ffmpeg", ["-hide_banner","-loglevel","error","-y","-i","tone.mp3","back.wav"]); console.log("runtime", x("ffprobe", ["-v","error","-show_entries","stream=codec_name","-of","csv=p=0","back.wav"]).toString().trim())'`},
	} {
		t.Run(tc.runtime, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = tc.runtime
			o.BuildCommand = tc.build
			o.StartCommand = `cat built.txt; ` + tc.decode
			tag := buildResolvedImage(t, o, map[string]string{tc.env: tc.line}, nil, fmt.Sprintf("bex-native-ffmpeg-it-%s:%d", tc.runtime, os.Getpid()))
			out, err := exec.Command("docker", "run", "--rm", "--network", "none", tag).CombinedOutput()
			wants := []string{"built mp3,44100", "runtime pcm_s16le"}
			if tc.runtime == "python" {
				wants = append(wants, "pydub mp3")
			}
			for _, want := range wants {
				if err != nil || !strings.Contains(string(out), want) {
					t.Fatalf("run = %v\n%s; want %q", err, out, want)
				}
			}
		})
	}
}

// TestNativeFFmpegLayerIsShared pins why the binaries are copied to /: cold
// builds of two different native images both carry the exact layer the copy
// makes on its own, so the registry and every node keep one copy of it.
// Opt-in: needs Docker.
func TestNativeFFmpegLayerIsShared(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build the generated images")
	}
	layers := func(t *testing.T, tag, dir string) []string {
		t.Helper()
		t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", "-f", tag).Run() })
		cmd := exec.Command("docker", "build", "-q", "--no-cache", "-t", tag, dir)
		cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", tag, err, out)
		}
		out, err := exec.Command("docker", "image", "inspect", "--format", "{{range .RootFS.Layers}}{{println .}}{{end}}", tag).Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.Fields(string(out))
	}
	alone := t.TempDir()
	must(t, os.WriteFile(filepath.Join(alone, "Dockerfile"), []byte("FROM scratch\n"+strings.SplitAfter(nativeMediaTools, "\n")[0]), 0o644))
	want := layers(t, fmt.Sprintf("bex-native-ffmpeg-layer-alone:%d", os.Getpid()), alone)
	if len(want) != 1 {
		t.Fatalf("the copy alone made layers %q, want one", want)
	}
	for _, runtime := range []string{"python", nativeNodeRuntime} {
		o := nativeOptions()
		o.Runtime, o.BuildCommand = runtime, "true"
		bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
		if _, err := ResolveNativeBuild(runtime, bundle, dockerfile, src); err != nil {
			t.Fatal(err)
		}
		df, err := os.ReadFile(dockerfile)
		if err != nil {
			t.Fatal(err)
		}
		// Up to the source: the layers a service's code cannot change.
		head := string(df)[:strings.Index(string(df), "\nCOPY . .\n")+1]
		must(t, os.WriteFile(filepath.Join(src, "Dockerfile"), []byte(head), 0o644))
		if got := layers(t, fmt.Sprintf("bex-native-ffmpeg-layer-%s:%d", runtime, os.Getpid()), src); !slices.Contains(got, want[0]) {
			t.Errorf("%s image layers %q lack the shared ffmpeg layer %s", runtime, got, want[0])
		}
	}
}
