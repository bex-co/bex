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
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// secretFileFixture writes files into a fresh "Secret volume" directory and
// returns Options naming them with their sizes, as the controller does.
func secretFileFixture(t *testing.T, o Options, sizes map[string]int) (Options, string, map[string][]byte) {
	t.Helper()
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	must(t, os.Mkdir(secret, 0o700))
	rng := rand.New(rand.NewPCG(uint64(len(sizes)), 699))
	contents := map[string][]byte{}
	o.NativeFileSizes = map[string]int{}
	o.NativeFiles = nil
	for _, name := range slices.Sorted(maps.Keys(sizes)) {
		content := make([]byte, sizes[name])
		for i := range content {
			content[i] = byte(rng.UintN(256))
		}
		must(t, os.WriteFile(filepath.Join(secret, name), content, 0o600))
		contents[name] = content
		o.NativeFiles = append(o.NativeFiles, name)
		o.NativeFileSizes[name] = len(content)
	}
	o.NativeFilesSecrets = []string{"bld-web-native-files"}
	return o, dir, contents
}

// runPreparerIn executes o's native preparer with /native and /native-files
// under dir.
func runPreparerIn(t *testing.T, o Options, dir string) ([]byte, error) {
	t.Helper()
	prep := nativeBuildPreparer(o)
	script := strings.NewReplacer("/native-files", filepath.Join(dir, "secret"), "/native", dir,
		"/runtime-env", filepath.Join(dir, "absent")).Replace(prep.Args[0])
	cmd := exec.Command("sh", "-eu", "-c", script)
	cmd.Env = os.Environ()
	for name, value := range expandedBuildValues(prep.Env) { // as the kubelet resolves them
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	return cmd.CombinedOutput()
}

var secretMountFlag = regexp.MustCompile(`--mount=type=secret,id=([^,]+),target=([^, ]+),required=true`)

// The w4/699 regression: a 614,400-byte secret file failed every repo build
// with "secret bex-file-0 too big. max size 500KiB", because buildctl refuses
// any --secret source over 500 KiB. The native build now carries such a file
// in parts no larger than that and reassembles it at /etc/secrets/<name>. The
// test runs the real preparer, checks every --secret source buildctl would
// open, and replays the RUN's assembly over the parts to get the exact bytes.
func TestNativeSecretFilesOverBuildKitLimitTravelInParts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		sizes map[string]int
		split bool
	}{
		{"one 614,400-byte file", map[string]int{"qa-build.txt": 614400}, true},
		{"1 MiB in two files beside a small one", map[string]int{"a.bin": 524288, "b.bin": 524288, ".npmrc": 21}, true},
		{"a file of exactly 500 KiB", map[string]int{"edge.bin": buildkitMaxSecretSize}, false},
		{"one byte over", map[string]int{"edge.bin": buildkitMaxSecretSize + 1}, true},
		{"1 MiB in one file", map[string]int{"max.bin": 1 << 20}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.StaticSite, o.Runtime, o.StartCommand = true, "", ""
			o, dir, contents := secretFileFixture(t, o, tc.sizes)
			if err := validateNativeOptions(o); err != nil {
				t.Fatal(err)
			}
			if out, err := runPreparerIn(t, o, dir); err != nil {
				t.Fatalf("prepare: %v: %s", err, out)
			}

			sources := map[string]string{} // secret id -> src on disk
			args := buildctlRecipeArgs(o, "/workspace/source")
			for i := 0; i+1 < len(args); i++ {
				if args[i] != "--secret" || strings.HasPrefix(args[i+1], "id=render-env,") {
					continue
				}
				id, src, ok := strings.Cut(strings.TrimPrefix(args[i+1], "id="), ",src=")
				if !ok || !strings.HasPrefix(src, "/native/") {
					t.Fatalf("unexpected --secret %q", args[i+1])
				}
				local := filepath.Join(dir, strings.TrimPrefix(src, "/native/"))
				info, err := os.Stat(local)
				if err != nil {
					t.Fatalf("--secret %s source missing after the preparer: %v", id, err)
				}
				if info.Size() > buildkitMaxSecretSize {
					t.Fatalf("--secret %s is %d bytes, over BuildKit's %d", id, info.Size(), buildkitMaxSecretSize)
				}
				sources[id] = local
			}

			dockerfile := nativeDockerfile(o)
			mounts := secretMountFlag.FindAllStringSubmatch(dockerfile, -1)
			if len(mounts) != len(sources) {
				t.Fatalf("Dockerfile mounts %d file secrets, buildctl passes %d:\n%s", len(mounts), len(sources), dockerfile)
			}
			if got := strings.Contains(dockerfile, "--mount=type=tmpfs,target=/etc/secrets "); got != tc.split {
				t.Fatalf("tmpfs /etc/secrets = %v, want %v:\n%s", got, tc.split, dockerfile)
			}

			etc := filepath.Join(dir, "etc-secrets")
			must(t, os.Mkdir(etc, 0o700))
			for name, want := range contents {
				var got []byte
				if !tc.split {
					// Direct mounts: BuildKit puts the one secret at the target.
					for _, m := range mounts {
						if m[2] == "/etc/secrets/"+name {
							got = mustRead(t, sources[m[1]])
						}
					}
				} else {
					// Longest target first, so .../1/1 never rewrites part of .../1/10.
					slices.SortFunc(mounts, func(a, b []string) int { return len(b[2]) - len(a[2]) })
					replace := []string{"/etc/secrets", etc}
					for _, m := range mounts {
						replace = append(replace, m[2], sources[m[1]])
					}
					var assemble []string
					for line := range strings.SplitSeq(dockerfile, "\n") {
						if strings.HasPrefix(line, "cat "+nativeFilePartsMount+"/") {
							assemble = append(assemble, line)
						}
					}
					script := strings.NewReplacer(replace...).Replace(strings.Join(assemble, "\n"))
					if out, err := exec.Command("sh", "-eu", "-c", script).CombinedOutput(); err != nil {
						t.Fatalf("assemble: %v: %s\n%s", err, out, script)
					}
					got = mustRead(t, filepath.Join(etc, name))
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("%s: build reads %d bytes, want the exact %d", name, len(got), len(want))
				}
			}
		})
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Files that all fit one secret keep the Job and Dockerfile they had before
// w4/699: the same ids, sources and direct /etc/secrets mounts, no tmpfs and no
// parts list for the preparer.
func TestNativeSecretFilesUnderTheLimitAreUnchanged(t *testing.T) {
	o := nativeOptions()
	o.NativeFilesSecrets, o.NativeFiles = []string{"bld-web-native-files"}, []string{".npmrc", "qa-r59.txt"}
	before := nativeDockerfile(o)
	o.NativeFileSizes = map[string]int{".npmrc": 21, "qa-r59.txt": buildkitMaxSecretSize}
	if after := nativeDockerfile(o); after != before {
		t.Fatalf("sizes under the limit changed the Dockerfile:\n%s\nwant\n%s", after, before)
	}
	if strings.Contains(before, "tmpfs") || strings.Contains(before, nativeFilePartsMount) {
		t.Fatalf("no split file, yet the Dockerfile assembles:\n%s", before)
	}
	for _, env := range nativeBuildPreparer(o).Env {
		if env.Name == "BEX_NATIVE_FILE_PARTS" {
			t.Fatalf("no split file, yet the preparer gets a parts list: %q", env.Value)
		}
	}
}

// A file that grew past its planned part count between the Job being made and
// the preparer running fails the preparer by name, not BuildKit with an id.
func TestNativeSecretFileGrownPastItsPartsFailsThePreparer(t *testing.T) {
	o := nativeOptions()
	o, dir, _ := secretFileFixture(t, o, map[string]int{"grow.bin": 600000})
	must(t, os.WriteFile(filepath.Join(dir, "secret", "grow.bin"), make([]byte, 2*buildkitMaxSecretSize+1), 0o600))
	out, err := runPreparerIn(t, o, dir)
	if err == nil || !strings.Contains(string(out), "secret file grow.bin changed") {
		t.Fatalf("grown file: err=%v out=%s", err, out)
	}
}

// w4/699, Dockerfile builds: the tenant's Dockerfile names the secret id, so a
// file over BuildKit's limit cannot be split. It is left out of the build
// secrets, which would otherwise fail every build, stays in the context root,
// and the build log says why it is not mountable.
func TestDockerfileBuildLeavesAnOversizedSecretFileOutOfItsSecrets(t *testing.T) {
	o := dockerOpts()
	o.NativeFileSizes = map[string]int{o.NativeFiles[0]: buildkitMaxSecretSize + 1}
	big, small := o.NativeFiles[0], o.NativeFiles[1]
	args := dockerBuildSecretArgs(o)
	if strings.Contains(strings.Join(args, " "), "id="+renderSecretFileID(big)+",") {
		t.Fatalf("oversized %s still passed as a build secret: %v", big, args)
	}
	if !containsPair(args, "--secret", "id="+renderSecretFileID(small)+",src="+dockerBuildInputsMount+"/files/"+small) {
		t.Fatalf("small %s lost its build secret: %v", small, args)
	}
	script := dockerBuildInputsScript(o, BuildLiterals(o.BuildEnv))
	notice := fmt.Sprintf("Secret file %s is over the 500 KiB a Docker build secret can hold", big)
	if !strings.Contains(script, notice) || strings.Contains(script, "Secret file "+small+" is over") {
		t.Fatalf("notice wrong:\n%s", script)
	}
	out, err := exec.Command("sh", "-c", dockerOversizedSecretNotice(o)).CombinedOutput()
	if err != nil || !strings.Contains(string(out), notice) {
		t.Fatalf("notice does not run: %v %s", err, out)
	}
}

// TestNativeSecretFilesInPartsPinnedBuildKit runs w4/699's repro through the
// pinned production BuildKit image with the exact Job buildctl args: a static
// build reads a 614,400-byte and a 21-byte secret file whole at /etc/secrets,
// and neither the files nor their parts are left in the image. Opt-in: needs a
// privileged Docker daemon and pulls the node toolchain.
func TestNativeSecretFilesInPartsPinnedBuildKit(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to run against the pinned BuildKit image")
	}
	o := nativeOptions()
	o.StaticSite, o.Runtime, o.StartCommand, o.BuildEnv = true, "", "", nil
	o.BuildCommand = `wc -c < /etc/secrets/qa-build.txt | tr -d " " > check.txt && cat /etc/secrets/qa-build.txt /etc/secrets/.npmrc > joined.bin`
	o, dir, contents := secretFileFixture(t, o, map[string]int{"qa-build.txt": 614400, ".npmrc": 21})
	if err := validateNativeOptions(o); err != nil {
		t.Fatal(err)
	}
	if out, err := runPreparerIn(t, o, dir); err != nil {
		t.Fatalf("prepare: %v: %s", err, out)
	}
	for _, sub := range []string{"src", "out"} {
		must(t, os.Mkdir(filepath.Join(dir, sub), 0o755))
	}
	must(t, os.WriteFile(filepath.Join(dir, "src", "README"), []byte("fixture\n"), 0o644))
	runJobBuildctl(t, o, filepath.Join(dir, "src"), dir, filepath.Join(dir, "out"))
	_, layer := lastOCILayer(t, filepath.Join(dir, "out", "image.tar"))
	for name := range layer {
		if strings.HasPrefix(name, "etc/secrets") || strings.HasPrefix(name, "run/") {
			t.Fatalf("secret mount committed into the RUN layer: %s", name)
		}
	}
	if got := string(layer["opt/render/project/src/check.txt"]); got != "614400\n" {
		t.Fatalf("check.txt = %q, want 614400", got)
	}
	want := append(slices.Clone(contents["qa-build.txt"]), contents[".npmrc"]...)
	if !bytes.Equal(layer["opt/render/project/src/joined.bin"], want) {
		t.Fatalf("the build read %d bytes, not the files' exact %d", len(layer["opt/render/project/src/joined.bin"]), len(want))
	}
}
