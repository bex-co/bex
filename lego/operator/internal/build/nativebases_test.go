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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const suppliedFixtureImage = "ghcr.io/bex-co/bex-native:test@" + otherDigest

// The resolver returns the upstream language identity even after its final
// FROM adopts a supplied derivative. Neighbor tests must check both contracts.
func expectedNativeBase(reviewed string) string {
	key := reviewed
	for _, toolchain := range nativeToolchains {
		for _, line := range toolchain.lines {
			if line.image == reviewed {
				key = cmp.Or(line.upstream, line.image)
			}
		}
	}
	if supplied, ok := nativeSuppliedBases[key]; ok {
		return supplied
	}
	return reviewed
}

func assertExactNativePrimary(t *testing.T, dockerfile, reviewed, exact string, env []string) {
	t.Helper()
	base := exact
	if len(nativeSuppliedBases) != 0 {
		base = expectedNativeBase(reviewed)
		copyAt := strings.Index(dockerfile, "COPY --link --from="+exact+" /usr/local/ /usr/local/\n")
		resetAt := strings.Index(dockerfile, "rm -rf /usr/local")
		refreshAt := strings.Index(dockerfile, "\nRUN --network=none /sbin/ldconfig\n")
		if resetAt < 0 || copyAt < resetAt || refreshAt < copyAt {
			t.Fatalf("exact primary or its loader cache was not replaced:\n%s", dockerfile)
		}
		for _, item := range env {
			key, value, _ := strings.Cut(item, "=")
			if !strings.Contains(dockerfile, "ENV "+key+"=\""+value+"\"\n") {
				t.Errorf("exact primary environment %q was not restored:\n%s", item, dockerfile)
			}
		}
	}
	if !strings.Contains(dockerfile, "\nFROM "+base+"\n") {
		t.Errorf("primary FROM must be %s:\n%s", base, dockerfile)
	}
}

func TestSuppliedResolverPreservesEveryReviewedPrimary(t *testing.T) {
	original := nativeSuppliedBases
	t.Cleanup(func() { nativeSuppliedBases = original })
	catalog := map[string]string{}
	for runtime, toolchain := range nativeToolchains {
		for _, line := range toolchain.lines {
			catalog[cmp.Or(line.upstream, line.image)] = "ghcr.io/bex-co/bex-native:test-" + runtime + "-" + line.line + "@" + otherDigest
		}
	}
	// After adoption, exercise the real catalog for every reviewed line. An
	// incomplete catalog must fail here before an unselected runtime reaches users.
	if len(original) != 0 {
		if len(original) != len(catalog) {
			t.Fatalf("supplied catalog has %d entries; reviewed toolchains require %d", len(original), len(catalog))
		}
		for base := range catalog {
			if _, ok := original[base]; !ok {
				t.Fatalf("supplied catalog lacks reviewed base %s", base)
			}
		}
		catalog = original
	}
	for runtime, toolchain := range nativeToolchains {
		for _, line := range toolchain.lines {
			t.Run(runtime+"/"+line.line, func(t *testing.T) {
				env, source := map[string]string{}, map[string]string{}
				switch runtime {
				case nativePythonRuntime:
					env["PYTHON_VERSION"] = line.line
				case nativeNodeRuntime:
					env["NODE_VERSION"] = line.line
				case nativeElixirRuntime:
					env["ELIXIR_VERSION"] = line.line
				case nativeRubyRuntime:
					source[".ruby-version"] = line.line
				}
				nativeSuppliedBases = nil
				beforeChoices, before := resolveExact(t, &fakeExact{}, runtime, env, source)
				nativeSuppliedBases = catalog
				afterChoices, after := resolveExact(t, &fakeExact{}, runtime, env, source)
				for _, choices := range [][]NativeChoice{beforeChoices, afterChoices} {
					if choices[0].Line != line.line || choices[0].Image != line.image {
						t.Fatalf("supplier changed the reviewed language identity: %+v", choices)
					}
				}
				want := strings.Replace(before, "\nFROM "+line.image+"\n", "\nFROM "+catalog[cmp.Or(line.upstream, line.image)]+"\n", 1)
				if !strings.Contains(before, "\nFROM "+line.image+"\n") || after != want || strings.Contains(after, "rm -rf /usr/local") {
					t.Fatalf("supplier changed the reviewed primary or companion/user layers:\n%s", after)
				}
			})
		}
	}
}

func TestSuppliedExactPrimaryPrecedesCompanions(t *testing.T) {
	for _, tc := range []struct{ runtime, env, version, tag string }{
		{"node", "NODE_VERSION", "24.10.0", "node:24.10.0-bookworm"},
		{"python", "PYTHON_VERSION", "3.14.8", "python:3.14.8-bookworm"},
		{"ruby", "RUBY_VERSION", "3.3.1", "ruby:3.3.1-bookworm"},
		{"elixir", "ELIXIR_VERSION", "1.17.0", "elixir:1.17.0"},
	} {
		t.Run(tc.runtime, func(t *testing.T) {
			original := nativeSuppliedBases
			t.Cleanup(func() { nativeSuppliedBases = original })
			env := map[string]string{tc.env: tc.version, "BUN_VERSION": "1.3"}
			files := map[string]string{}
			if tc.runtime == "ruby" {
				delete(env, "RUBY_VERSION")
				files[".ruby-version"] = tc.version
			}
			reviewed, err := ResolveNativeVersion(tc.runtime, env, func(name string) ([]byte, bool) { v, ok := files[name]; return []byte(v), ok })
			if err != nil {
				t.Fatal(err)
			}
			nativeSuppliedBases = map[string]string{reviewed.Image: suppliedFixtureImage}
			fake := &fakeExact{digests: map[string]string{tc.tag: exactDigest},
				env:     []string{tc.env + "=" + tc.version, "PATH=/usr/local/bin:/usr/bin", "LITERAL=$HOME\\path\"quoted"},
				history: []string{"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'"}}
			if tc.runtime == "elixir" {
				fake.env = append(fake.env, "OTP_VERSION=27.0")
			}
			choices, after := resolveExact(t, fake, tc.runtime, env, files)
			choice := choices[0]
			reset := strings.Index(after, "rm -rf /usr/local")
			primaryCopy := strings.Index(after, "COPY --link --from="+choice.Image+" /usr/local/")
			refresh := strings.Index(after, "\nRUN --network=none /sbin/ldconfig\n")
			if reset < 0 || primaryCopy < reset || refresh < primaryCopy {
				t.Fatalf("exact primary was merged instead of replaced:\n%s", after)
			}
			for _, marker := range []string{" /usr/local/bin/bun /usr/local/bin/bun", "/opt/bex/poetry", " /usr/local/bin/node /usr/local/bin/node", "\nCOPY . .\n"} {
				if at := strings.Index(after, marker); at >= 0 && at < refresh {
					t.Fatalf("exact primary erased later installation %q:\n%s", marker, after)
				}
			}
			for _, want := range []string{"primary_os=\"$VERSION_CODENAME\"", "ENV " + tc.env + "=\"" + tc.version + "\"", `ENV LITERAL="\$HOME\\path\"quoted"`} {
				if !strings.Contains(after, want) {
					t.Fatalf("exact source environment/compatibility check missing %q:\n%s", want, after)
				}
			}
			if strings.Contains(after, "apt-get") {
				t.Fatal("supplied tools are being installed in a tenant build")
			}
			hook := strings.Index(after, "RUN --network=none /usr/bin/bex-native-python-tools\n")
			if tc.runtime == nativePythonRuntime {
				if hook < primaryCopy || hook > strings.Index(after, "RUN /usr/local/bin/python -m venv") {
					t.Fatalf("primary Python tools are not restored offline before its companions:\n%s", after)
				}
			} else if hook >= 0 {
				t.Fatal("Python packaging hook reached another primary runtime")
			}
		})
	}
}

func TestSuppliedExactErlangKeepsTheMatchedElixir(t *testing.T) {
	original := nativeSuppliedBases
	t.Cleanup(func() { nativeSuppliedBases = original })
	fake := erlangFake("28.3.3.0")
	env := map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.3"}
	reviewed, err := ResolveNativeVersion("elixir", env, func(string) ([]byte, bool) { return nil, false })
	if err != nil {
		t.Fatal(err)
	}
	nativeSuppliedBases = map[string]string{reviewed.Image: suppliedFixtureImage}
	fake.env = []string{"OTP_VERSION=28.3.3.0", "PATH=/usr/local/bin:/usr/bin"}
	fake.history = []string{"# debian.sh --arch 'amd64' out/ 'trixie' '@1749513600'"}
	choices, after := resolveExact(t, fake, "elixir", env, nil)
	choice := choices[0]
	erlang := strings.Index(after, "COPY --link --from="+choice.Image+" /usr/local/")
	elixir := strings.Index(after, "COPY --link --from="+choice.elixirImage+" /usr/local/lib/elixir")
	if erlang < 0 || elixir < erlang || !strings.Contains(after, "ENV OTP_VERSION=\"28.3.3.0\"") || !strings.Contains(after, "ELIXIR_VERSION=v1.19.5") {
		t.Fatalf("matched Elixir/OTP was changed:\n%s", after)
	}
}

func TestSuppliedBaseRefusesUnreviewedInputs(t *testing.T) {
	line, _ := nativeToolchains["node"].find("24")
	before := nativeDockerfile(nativeOptions())
	choice := NativeChoice{Image: line.image}
	for name, pins := range map[string]map[string]string{
		"missing reviewed base": {"another": suppliedFixtureImage},
		"mutable tag":           {line.image: "ghcr.io/bex-co/bex-native:latest"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := applyNativeSuppliedBase(before, line.image, choice, pins); err == nil {
				t.Fatal("accepted unreviewed supplied base")
			}
		})
	}
	choice.Image = "node:24.10.0-bookworm@" + exactDigest
	before, err := rewriteNativeBase(before, line.image, choice.Image)
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range [][]string{nil, {"BAD KEY=value"}, {"PATH=value\nRUN injected"}} {
		choice.imageEnv = env
		if _, err := applyNativeSuppliedBase(before, line.image, choice, map[string]string{line.image: suppliedFixtureImage}); err == nil {
			t.Fatalf("accepted invalid exact primary environment %v", env)
		}
	}
}

func TestSuppliedResolverKeepsTheRequestedExactMetadata(t *testing.T) {
	original := nativeSuppliedBases
	t.Cleanup(func() { nativeSuppliedBases = original })
	line, _ := nativeToolchains["python"].find("3.14")
	nativeSuppliedBases = map[string]string{line.image: suppliedFixtureImage}
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": "3.14.8"}, nil)
	fake := &fakeExact{digests: map[string]string{"python:3.14.8-bookworm": exactDigest}, env: []string{"PYTHON_VERSION=3.14.8", "PYTHON_PIP_VERSION=25.2", "PATH=/usr/local/bin:/usr/bin"}}
	choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, "python", bundle, dockerfile, src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(dockerfile)
	if err != nil {
		t.Fatal(err)
	}
	if choices[0].Line != "3.14.8" || !strings.Contains(string(out), "ENV PYTHON_PIP_VERSION=\"25.2\"") || !strings.Contains(string(out), "FROM "+suppliedFixtureImage) {
		t.Fatalf("exact metadata or supplied base lost: %+v\n%s", choices, out)
	}
}

func TestSuppliedResolverEnvironmentLookupFailureKeepsVersionFallbackRules(t *testing.T) {
	original := nativeSuppliedBases
	t.Cleanup(func() { nativeSuppliedBases = original })
	for _, runtime := range []string{"python", "elixir"} {
		t.Run(runtime, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = runtime
			env := map[string]string{"PYTHON_VERSION": "3.14.8"}
			fake := &fakeExact{digests: map[string]string{"python:3.14.8-bookworm": exactDigest}}
			if runtime == "elixir" {
				env = map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.3"}
				fake = erlangFake("28.3.3.0")
			}
			fake.envErr = errors.New("config unavailable")
			reviewed, err := ResolveNativeVersion(runtime, env, func(string) ([]byte, bool) { return nil, false })
			if err != nil {
				t.Fatal(err)
			}
			nativeSuppliedBases = map[string]string{reviewed.Image: suppliedFixtureImage}
			bundle, dockerfile, src := writeResolveFixture(t, o, env, nil)
			choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, runtime, bundle, dockerfile, src)
			if runtime == "elixir" {
				if err == nil || !strings.Contains(err.Error(), "exact Erlang image environment") {
					t.Fatalf("exact Erlang silently fell back: %v", err)
				}
				return
			}
			if err != nil || choices[0].Image != reviewed.Image || !strings.Contains(choices[0].Narration(), "config unavailable") {
				t.Fatalf("ordinary exact version lost its narrated reviewed fallback: %+v, %v", choices, err)
			}
			out, err := os.ReadFile(dockerfile)
			if err != nil || !strings.Contains(string(out), "FROM "+suppliedFixtureImage) || strings.Contains(string(out), "rm -rf /usr/local") {
				t.Fatalf("fallback did not retain the supplied reviewed primary: %v\n%s", err, out)
			}
		})
	}
}

func TestNativeDebianSuiteAcceptsOnlyBoundedRootProvenance(t *testing.T) {
	for _, suite := range []string{"bullseye", "bookworm", "trixie"} {
		for _, arch := range []string{"amd64", "arm64"} {
			root := "# debian.sh --arch '" + arch + "' out/ '" + suite + "' '@1749513600'"
			got, err := nativeDebianSuite([]string{root, "RUN apt-get update"})
			if err != nil || got != suite {
				t.Fatalf("suite %q/%q = %q, %v", arch, suite, got, err)
			}
		}
	}
	legacy := "/bin/sh -c #(nop) ADD file:" + strings.Repeat("a", 64) + " in / "
	if got, err := nativeDebianSuite([]string{legacy}); err != nil || got != "" {
		t.Fatalf("legacy root invented a suite: %q, %v", got, err)
	}
	cases := map[string][]string{
		"empty":             nil,
		"untrusted text":    {"FROM debian:bookworm"},
		"unknown suite":     {"# debian.sh --arch 'amd64' out/ 'forky' '@1749513600'"},
		"command suffix":    {"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'; extra"},
		"newline":           {"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'\nRUN injected"},
		"later provenance":  {"RUN true", "# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'"},
		"ambiguous roots":   {"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'", "# debian.sh --arch 'amd64' out/ 'trixie' '@1749513600'"},
		"oversized entry":   {strings.Repeat("a", (64<<10)+1)},
		"too many entries":  make([]string, 1025),
		"oversized history": make([]string, 17),
	}
	for i := range cases["oversized history"] {
		cases["oversized history"][i] = strings.Repeat("a", 64<<10)
	}
	for name, history := range cases {
		t.Run(name, func(t *testing.T) {
			if suite, err := nativeDebianSuite(history); err == nil {
				t.Fatalf("accepted %s provenance as %q", name, suite)
			}
		})
	}
}

func TestExactOTPSuppliedBaseMatchesSourceSuite(t *testing.T) {
	reviewed, _ := nativeToolchains[nativeElixirRuntime].find("1.18")
	bookworm, _ := nativeToolchains[nativeElixirRuntime].find("1.17")
	trixie, _ := nativeToolchains[nativeElixirRuntime].find("1.19")
	const bookwormSupplier = "ghcr.io/bex-co/bex-native:bookworm-test@" + exactDigest
	pins := map[string]string{reviewed.image: suppliedFixtureImage, bookworm.image: bookwormSupplier, trixie.image: suppliedFixtureImage}
	for _, tc := range []struct{ name, suite, otp, want string }{
		{"old OTP28.0", "bookworm", "28.0", bookwormSupplier},
		{"current OTP28.3.3", "trixie", "28.3.3", suppliedFixtureImage},
		{"legacy OTP27 source", "", "27.0", bookwormSupplier},
	} {
		t.Run(tc.name, func(t *testing.T) {
			choice := NativeChoice{Image: "erlang:" + tc.otp + "@" + exactDigest, imageSuite: tc.suite, imageEnv: []string{"OTP_VERSION=" + tc.otp}}
			got, err := suppliedNativeImage(reviewed.image, choice, pins)
			if err != nil || got != tc.want {
				t.Fatalf("selected %q, %v; want source-suite supplier %q", got, err, tc.want)
			}
		})
	}
	for _, choice := range []NativeChoice{
		{Image: "erlang:28.0@" + exactDigest, imageSuite: "forky"},
		{Image: "erlang:28.0@" + exactDigest, imageEnv: []string{"OTP_VERSION=bad"}},
		{Image: "erlang:28.0@" + exactDigest},
		{Image: "erlang:28.0@" + exactDigest, imageEnv: []string{"OTP_VERSION=26.0"}},
	} {
		if got, err := suppliedNativeImage(reviewed.image, choice, pins); err == nil {
			t.Fatalf("invented suite supplier %q from %+v", got, choice)
		}
	}
}

// This gate uses real, independently published or locally served derivatives;
// no synthetic digest can stand in for a supply-chain artifact.
func TestNativeSuppliedImageRunsOldExactOTP(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 and supply a pinned Bookworm Elixir derivative reference")
	}
	bookwormRef := os.Getenv("BEX_NATIVE_TEST_BOOKWORM_IMAGE")
	if bookwormRef == "" {
		t.Fatal("BEX_NATIVE_TEST_BOOKWORM_IMAGE must name an actual immutable Bookworm Elixir derivative")
	}
	bookworm, _ := nativeToolchains[nativeElixirRuntime].find("1.17")
	original := nativeSuppliedBases
	t.Cleanup(func() { nativeSuppliedBases = original })
	nativeSuppliedBases = map[string]string{bookworm.image: bookwormRef}
	archive, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "native-base", "testdata", "archive.dump.b64"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "native-base", "testdata", "rows.txt"))
	if err != nil {
		t.Fatal(err)
	}
	const check = `
set -eu
base64 -d archive.dump.b64 > archive.dump
pg_restore --file=archive.sql archive.dump
awk '/^COPY public.native_archive / { copying=1; next } copying && $0=="\\." { exit } copying { print }' archive.sql > actual-rows.txt
cmp rows.txt actual-rows.txt
! pg_restore --file=invalid.sql /dev/null
psql --version
pg_dump --version
node --version
npm --version
tsc --version
find /usr/local/lib/erlang -type f -name '*.so' -exec sh -ec 'for f; do result=$(ldd "$f" 2>&1) || { printf "%s\n" "$result"; exit 1; }; case $result in *"not found"*) printf "%s\n" "$result"; exit 1;; esac; done' sh {} +
elixir -e 'unless System.version() == "1.18.4" and System.get_env("OTP_VERSION") == "28.0" and System.otp_release() == "28", do: raise("wrong exact runtime"); release = File.read!(Path.join([to_string(:code.root_dir()), "releases", System.otp_release(), "OTP_VERSION"])) |> String.trim(); unless release == "28.0", do: raise("wrong installed OTP release"); unless Base.encode16(:crypto.hash(:sha256, "abc"), case: :lower) == "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", do: raise("crypto"); :ok = :ssl.start(); t = :ets.new(:probe, []); true = :ets.insert(t, {:answer, 42}); [{:answer, 42}] = :ets.lookup(t, :answer); IO.puts("exact-otp-runtime-and-libraries-ok")'
`
	o := nativeOptions()
	o.Runtime = nativeElixirRuntime
	o.BuildCommand = check + "\necho native-supplied-build-ok"
	o.StartCommand = check + "\necho native-supplied-runtime-ok"
	bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"ELIXIR_VERSION": "1.18.4", "ERLANG_VERSION": "28.0.0.0"}, map[string]string{"archive.dump.b64": string(archive), "rows.txt": string(rows)})
	choices, err := ResolveNativeBuildExact(t.Context(), NewDockerHubResolver(), time.Time{}, nativeElixirRuntime, bundle, dockerfile, src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(dockerfile)
	if err != nil {
		t.Fatal(err)
	}
	if choices[0].otp != "28.0" || choices[0].imageSuite != "bookworm" || !strings.Contains(string(out), "FROM "+bookwormRef) {
		t.Fatalf("old exact OTP did not choose its source suite: %+v\n%s", choices, out)
	}
	must(t, os.Rename(dockerfile, filepath.Join(src, "Dockerfile")))
	tag := fmt.Sprintf("bex-native-supplied-old-otp:%d", os.Getpid())
	t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", "-f", tag).Run() })
	build := exec.Command("docker", "build", "--progress=plain", "-t", tag, "--secret", "id=render-env,src="+bundle, src)
	build.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	if output, err := build.CombinedOutput(); err != nil || !strings.Contains(string(output), "native-supplied-build-ok") {
		t.Fatalf("real generated-image build: %v\n%s", err, output)
	}
	output, err := exec.Command("docker", "run", "--rm", "--network", "none", tag).CombinedOutput()
	if err != nil || !strings.Contains(string(output), "native-supplied-runtime-ok") || !strings.Contains(string(output), "exact-otp-runtime-and-libraries-ok") {
		t.Fatalf("offline exact runtime: %v\n%s", err, output)
	}
	t.Logf("offline runtime:\n%s", output)
	t.Logf("source %s on %s, supplied base %s; build/runtime real PG18 archive, complete Erlang shared-library closure, crypto/SSL/ETS and Node companion pass", choices[0].Image, choices[0].imageSuite, bookwormRef)
}

// A pre-existing platform backport must remain a fallback while the supplier
// catalog and exact lookup continue to name the reviewed official input.
func TestSuppliedBasePreservesPlatformBackportProvenance(t *testing.T) {
	const platform = "ghcr.io/bex-co/bex-native:python-backport@" + otherDigest
	originalToolchain, originalCatalog := nativeToolchains[nativePythonRuntime], nativeSuppliedBases
	t.Cleanup(func() {
		nativeToolchains[nativePythonRuntime], nativeSuppliedBases = originalToolchain, originalCatalog
	})
	official, _ := originalToolchain.find("3.14")
	upstream := cmp.Or(official.upstream, official.image)
	backported := originalToolchain
	backported.lines = slices.Clone(originalToolchain.lines)
	for i := range backported.lines {
		if backported.lines[i].line == "3.14" {
			backported.lines[i].image, backported.lines[i].upstream = platform, upstream
		}
	}
	nativeToolchains[nativePythonRuntime] = backported
	for _, supplied := range []bool{false, true} {
		for _, found := range []bool{false, true} {
			t.Run(fmt.Sprintf("supplied-%v/exact-found-%v", supplied, found), func(t *testing.T) {
				nativeSuppliedBases = nil
				if supplied {
					nativeSuppliedBases = map[string]string{upstream: suppliedFixtureImage}
				}
				fake := &fakeExact{env: []string{"PYTHON_VERSION=3.14.8", "PATH=/usr/local/bin:/usr/bin"}}
				if found {
					fake.digests = map[string]string{"python:3.14.8-bookworm": exactDigest}
				}
				choices, after := resolveExact(t, fake, nativePythonRuntime, map[string]string{"PYTHON_VERSION": "3.14.8"}, nil)
				choice := choices[0]
				base := platform
				if found {
					base = "python:3.14.8-bookworm@" + exactDigest
					if choice.Image != base {
						t.Fatalf("exact source identity changed: %+v", choice)
					}
					assertExactNativePrimary(t, after, platform, base, fake.env)
				}
				if supplied {
					base = suppliedFixtureImage
				}
				if !strings.Contains(after, "\nFROM "+base+"\n") || choice.upstream != upstream {
					t.Fatalf("resolver lost the upstream key or existing derivative: %+v\n%s", choice, after)
				}
				if len(fake.asked) == 0 || fake.asked[0] != "python:3.14.8-bookworm" {
					t.Fatalf("exact lookup escaped official input: %v", fake.asked)
				}
				hook := strings.Contains(after, "RUN --network=none /usr/bin/bex-native-python-tools")
				if hook != (found && supplied) || (!found && (choice.Image != platform || strings.Contains(after, "rm -rf /usr/local"))) {
					t.Fatalf("exact transplant or verified fallback changed: %+v\n%s", choice, after)
				}
			})
		}
	}
}
