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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type fakeExact struct {
	digests    map[string]string
	err        error
	asked      []string
	env        []string
	envErr     error
	history    []string
	envByImage map[string]fakeExactMetadata
}

type fakeExactMetadata struct{ env, history []string }

func (f *fakeExact) NodeSourceDigest(_ context.Context, version string) (string, error) {
	f.asked = append(f.asked, "node source "+version)
	return strings.Repeat("b", 64), f.err
}

func (f *fakeExact) ImageMetadata(_ context.Context, image string) (env, history []string, err error) {
	f.asked = append(f.asked, "env "+image)
	if metadata, ok := f.envByImage[image]; ok {
		return metadata.env, metadata.history, f.envErr
	}
	return f.env, f.history, f.envErr
}

func (f *fakeExact) ResolveTag(_ context.Context, repo, tag string) (string, bool, error) {
	f.asked = append(f.asked, repo+":"+tag)
	d, ok := f.digests[repo+":"+tag]
	return d, ok, f.err
}

// ListTags lists the repo's tags among digests, as the registry would.
func (f *fakeExact) ListTags(_ context.Context, repo, prefix string) ([]string, error) {
	f.asked = append(f.asked, "list "+repo+":"+prefix)
	var tags []string
	for ref := range f.digests {
		if r, tag, _ := strings.Cut(ref, ":"); r == repo && strings.HasPrefix(tag, prefix) {
			tags = append(tags, tag)
		}
	}
	return tags, f.err
}

const exactDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000001"

func TestPlatformBackportPreservesExactLookupAndVerifiedFallback(t *testing.T) {
	const platform = "ghcr.io/bex-co/bex-native:node-14-bullseye@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	r := resolver{t: nativeToolchain{label: "Node", lines: []nativeLine{{line: "14", image: platform, upstream: "node:14-bullseye@" + exactDigest}}}}
	choice, err := r.choice("14", "NODE_VERSION=14.21.2")
	if err != nil {
		t.Fatal(err)
	}
	choice.Exact = "14.21.2"
	for name, registry := range map[string]*fakeExact{
		"published exact":  {digests: map[string]string{"node:14.21.2-bullseye": exactDigest}},
		"missing exact":    {},
		"registry failure": {err: errors.New("registry unavailable")},
	} {
		t.Run(name, func(t *testing.T) {
			got := pinExactVersion(context.Background(), registry, choice)
			if len(registry.asked) != 1 || registry.asked[0] != "node:14.21.2-bullseye" {
				t.Fatalf("exact lookup escaped the official source: %v", registry.asked)
			}
			if name == "published exact" {
				if got.Image != "node:14.21.2-bullseye@"+exactDigest || got.Line != "14.21.2" {
					t.Fatalf("explicit version was not preserved: %+v", got)
				}
			} else if got.Image != platform {
				t.Fatalf("failed lookup lost the verified platform fallback: %+v", got)
			}
		})
	}
}

func resolveExact(t *testing.T, fake *fakeExact, runtime string, env, files map[string]string) ([]NativeChoice, string) {
	t.Helper()
	o := nativeOptions()
	o.Runtime = runtime
	bundle, dockerfile, src := writeResolveFixture(t, o, env, files)
	choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, runtime, bundle, dockerfile, src)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(dockerfile)
	return choices, string(out)
}

func TestExactVersionPinsTheRequestedPatch(t *testing.T) {
	cases := []struct {
		name, runtime string
		env           map[string]string
		files         map[string]string
		tag           string
		metadata      []string
	}{
		{"python env", "python", map[string]string{"PYTHON_VERSION": "3.11.9"}, nil, "python:3.11.9-bookworm", []string{"PYTHON_VERSION=3.11.9"}},
		{"node env", "node", map[string]string{"NODE_VERSION": "18.18.0"}, nil, "node:18.18.0-bookworm", []string{"NODE_VERSION=18.18.0"}},
		{"nvmrc", "node", nil, map[string]string{".nvmrc": "v20.1.0\n"}, "node:20.1.0-bookworm", []string{"NODE_VERSION=20.1.0"}},
		{"python file", "python", nil, map[string]string{".python-version": "3.12.1\n"}, "python:3.12.1-bookworm", []string{"PYTHON_VERSION=3.12.1"}},
		{"ruby file", "ruby", nil, map[string]string{".ruby-version": "3.3.1\n"}, "ruby:3.3.1-bookworm", []string{"RUBY_VERSION=3.3.1"}},
		{"elixir env", "elixir", map[string]string{"ELIXIR_VERSION": "1.17.0"}, nil, "elixir:1.17.0", []string{"ELIXIR_VERSION=v1.17.0", "OTP_VERSION=27.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeExact{digests: map[string]string{tc.tag: exactDigest}, env: tc.metadata,
				history: []string{"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'"}}
			choices, dockerfile := resolveExact(t, fake, tc.runtime, tc.env, tc.files)
			wantAsked := []string{tc.tag}
			if len(nativeSuppliedBases) != 0 {
				wantAsked = append(wantAsked, "env "+tc.tag+"@"+exactDigest)
			}
			if !slices.Equal(fake.asked, wantAsked) {
				t.Fatalf("asked %v, want %v", fake.asked, wantAsked)
			}
			reviewed, err := ResolveNativeVersion(tc.runtime, tc.env, files(tc.files))
			if err != nil {
				t.Fatal(err)
			}
			assertExactNativePrimary(t, dockerfile, reviewed.Image, tc.tag+"@"+exactDigest, tc.metadata)
			_, version, _ := strings.Cut(strings.SplitN(tc.tag, "-", 2)[0], ":")
			if choices[0].Line != version || choices[0].Image != tc.tag+"@"+exactDigest || !strings.Contains(choices[0].Narration(), version) {
				t.Errorf("narration %q does not name %s", choices[0].Narration(), version)
			}
		})
	}
}

func TestExactVersionLeavesLineRequestsAlone(t *testing.T) {
	reviewed, ok := nativeToolchains["python"].find("3.11")
	if !ok {
		t.Fatal("missing reviewed Python 3.11 toolchain")
	}
	for name, tc := range map[string]struct {
		runtime string
		env     map[string]string
		files   map[string]string
	}{
		"line only":       {"python", nil, map[string]string{".python-version": "3.11\n"}},
		"reviewed patch":  {"python", map[string]string{"PYTHON_VERSION": reviewed.version}, nil},
		"node major":      {"node", map[string]string{"NODE_VERSION": "18"}, nil},
		"node range":      {"node", map[string]string{"NODE_VERSION": ">=18 <19"}, nil},
		"no signal":       {"python", nil, nil},
		"elixir with otp": {"elixir", map[string]string{"ELIXIR_VERSION": "1.17"}, nil},
		"otp major only":  {"elixir", map[string]string{"ELIXIR_VERSION": "1.19", "ERLANG_VERSION": "28"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeExact{}
			resolveExact(t, fake, tc.runtime, tc.env, tc.files)
			if len(fake.asked) != 0 {
				t.Errorf("registry consulted: %v", fake.asked)
			}
		})
	}
}

func TestExactVersionFallsBackToTheReviewedPatch(t *testing.T) {
	for name, fake := range map[string]*fakeExact{
		"no such tag": {},
		"registry":    {err: errors.New("registry answered 500")},
	} {
		t.Run(name, func(t *testing.T) {
			choices, dockerfile := resolveExact(t, fake, "python", map[string]string{"PYTHON_VERSION": "3.11.9"}, nil)
			line, _ := nativeToolchains["python"].find("3.11")
			if choices[0].Image != line.image || !strings.Contains(dockerfile, "FROM "+expectedNativeBase(line.image)+"\n") || strings.Contains(dockerfile, "rm -rf /usr/local") {
				t.Errorf("Dockerfile left the reviewed image:\n%s", dockerfile)
			}
			narration := choices[0].Narration()
			if !strings.Contains(narration, "Using Python 3.11 ") || !strings.Contains(narration, "python:3.11.9-bookworm") {
				t.Errorf("narration %q does not explain the fallback", narration)
			}
		})
	}
}

func TestDockerHubResolver(t *testing.T) {
	var scope string
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		scope = r.URL.Query().Get("scope")
		_, _ = w.Write([]byte(`{"token":"tok"}`))
	})
	mux.HandleFunc("/v2/library/python/manifests/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.Method != http.MethodHead {
			http.Error(w, "bad", http.StatusUnauthorized)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/3.11.9-bookworm") {
			w.Header().Set("Docker-Content-Digest", exactDigest)
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d := &DockerHubResolver{Client: srv.Client(), AuthURL: srv.URL + "/token", RegistryURL: srv.URL}

	digest, ok, err := d.ResolveTag(context.Background(), "python", "3.11.9-bookworm")
	if err != nil || !ok || digest != exactDigest || scope != "repository:library/python:pull" {
		t.Fatalf("got %q %v %v scope %q", digest, ok, err, scope)
	}
	if _, ok, err := d.ResolveTag(context.Background(), "python", "3.11.99-bookworm"); ok || err != nil {
		t.Errorf("missing tag: ok=%v err=%v", ok, err)
	}
	for _, bad := range [][2]string{{"evil", "1.0"}, {"python", "../x"}, {"python", "a b"}} {
		if _, _, err := d.ResolveTag(context.Background(), bad[0], bad[1]); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}

// w8/198: a Ruby service created since the cutoff with no version signal builds
// on Render's default 3.4.4, and the log names it; an older or unknown-age
// service, and any explicit signal, keep the line's reviewed patch.
func TestRubyDefaultIsRendersExactPatch(t *testing.T) {
	fake := &fakeExact{digests: map[string]string{"ruby:3.4.4-bookworm": exactDigest}, env: []string{"RUBY_VERSION=3.4.4"}}
	reviewed, _ := nativeToolchains[nativeRubyRuntime].find("3.4")
	run := func(created time.Time, files map[string]string) NativeChoice {
		t.Helper()
		o := nativeOptions()
		o.Runtime = "ruby"
		bundle, dockerfile, src := writeResolveFixture(t, o, nil, files)
		choices, err := ResolveNativeBuildExact(context.Background(), fake, created, "ruby", bundle, dockerfile, src)
		if err != nil {
			t.Fatal(err)
		}
		if choices[0].Image == "ruby:3.4.4-bookworm@"+exactDigest {
			out, err := os.ReadFile(dockerfile)
			must(t, err)
			assertExactNativePrimary(t, string(out), reviewed.image, choices[0].Image, fake.env)
		}
		return choices[0]
	}

	got := run(rubyDefaultCutoff.Add(time.Hour), nil)
	if got.Line != "3.4.4" || !strings.Contains(got.Image, "ruby:3.4.4-bookworm@"+exactDigest) || got.Narration() != "==> Using Ruby 3.4.4 (default)" {
		t.Errorf("new service: %+v narration %q", got, got.Narration())
	}
	// w4/620: Render's default moved to 3.4.12, the line's reviewed patch, for
	// services created since 2026-11-03; one created before keeps 3.4.4.
	if c := run(renderDefaults20261103.Add(time.Hour), nil); c.Line != "3.4" || c.Image != reviewed.image || reviewed.version != "3.4.12" {
		t.Errorf("service created after 2026-11-03 must build on the reviewed 3.4.12: %+v (reviewed %s)", c, reviewed.version)
	}
	if c := run(renderDefaults20261103.Add(-time.Hour), nil); c.Line != "3.4.4" {
		t.Errorf("service created just before 2026-11-03 must keep 3.4.4: %+v", c)
	}
	for name, c := range map[string]NativeChoice{
		"older service":   run(rubyDefaultCutoff.Add(-time.Hour), nil),
		"unknown age":     run(time.Time{}, nil),
		"explicit signal": run(rubyDefaultCutoff.Add(time.Hour), map[string]string{".ruby-version": "3.4\n"}),
	} {
		if c.Line != "3.4" && c.Line != "3.4.4" || strings.Contains(c.Image, "ruby:3.4.4-bookworm@") {
			t.Errorf("%s must keep the reviewed 3.4 patch: %+v", name, c)
		}
	}

	missing := &fakeExact{digests: map[string]string{}}
	o := nativeOptions()
	o.Runtime = "ruby"
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
	choices, err := ResolveNativeBuildExact(context.Background(), missing, rubyDefaultCutoff.Add(time.Hour), "ruby", bundle, dockerfile, src)
	if err != nil || choices[0].Line != "3.4" || !strings.Contains(choices[0].Narration(), "No official image ruby:3.4.4-bookworm") {
		t.Errorf("a missing tag must fall back to the reviewed patch and say so: %+v %v", choices[0], err)
	}
}

// w8/217: a Python service created since the cutoff with no version signal
// builds on Render's default 3.14.8, not the 3.14 line's newest reviewed patch;
// PYTHON_VERSION=3.14 keeps the reviewed patch, and an older or unknown-age
// service keeps its 3.13 default.
func TestPythonDefaultIsRendersExactPatch(t *testing.T) {
	fake := &fakeExact{digests: map[string]string{"python:3.14.8-bookworm": exactDigest}, env: []string{"PYTHON_VERSION=3.14.8"}}
	reviewed, _ := nativeToolchains[nativePythonRuntime].find("3.14")
	run := func(created time.Time, env map[string]string) NativeChoice {
		t.Helper()
		o := nativeOptions()
		o.Runtime = "python"
		bundle, dockerfile, src := writeResolveFixture(t, o, env, nil)
		choices, err := ResolveNativeBuildExact(context.Background(), fake, created, "python", bundle, dockerfile, src)
		if err != nil {
			t.Fatal(err)
		}
		if choices[0].Image == "python:3.14.8-bookworm@"+exactDigest {
			out, err := os.ReadFile(dockerfile)
			must(t, err)
			assertExactNativePrimary(t, string(out), reviewed.image, choices[0].Image, fake.env)
		}
		return choices[0]
	}

	got := run(pythonCreationCutoff, nil)
	if got.Line != "3.14.8" || !strings.Contains(got.Image, "python:3.14.8-bookworm@"+exactDigest) || got.Narration() != "==> Using Python 3.14.8 (default)" {
		t.Errorf("new service: %+v narration %q", got, got.Narration())
	}
	// w4/620: Render's default moved to 3.14.10, the line's reviewed patch, for
	// services created since 2026-11-03; one created before keeps 3.14.8.
	if c := run(renderDefaults20261103.Add(time.Hour), nil); c.Line != "3.14" || c.Image != reviewed.image || reviewed.version != "3.14.10" {
		t.Errorf("service created after 2026-11-03 must build on the reviewed 3.14.10: %+v (reviewed %s)", c, reviewed.version)
	}
	if c := run(renderDefaults20261103.Add(-time.Hour), nil); c.Line != "3.14.8" {
		t.Errorf("service created just before 2026-11-03 must keep 3.14.8: %+v", c)
	}
	if c := run(pythonCreationCutoff.Add(time.Hour), map[string]string{"PYTHON_VERSION": "3.14"}); c.Line != "3.14" || c.Image != reviewed.image {
		t.Errorf("PYTHON_VERSION=3.14 must keep the reviewed %s: %+v", reviewed.version, c)
	}
	for name, c := range map[string]NativeChoice{
		"older service": run(pythonCreationCutoff.Add(-time.Hour), nil),
		"unknown age":   run(time.Time{}, nil),
	} {
		if c.Line != "3.13" || strings.Contains(c.Image, "python:3.14.8-bookworm@") {
			t.Errorf("%s must keep the 3.13 default: %+v", name, c)
		}
	}

	missing := &fakeExact{digests: map[string]string{}}
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
	choices, err := ResolveNativeBuildExact(context.Background(), missing, pythonCreationCutoff.Add(time.Hour), "python", bundle, dockerfile, src)
	if err != nil || choices[0].Line != "3.14" || !strings.Contains(choices[0].Narration(), "No official image python:3.14.8-bookworm") {
		t.Errorf("a missing tag must fall back to the reviewed patch and say so: %+v %v", choices[0], err)
	}
}

// w8/307: a range the line's reviewed patch misses (`~18.17.0` against
// 18.20.8) builds on the newest official release inside it, as Render installs
// one, from the env, a version file or package.json engines.
func TestNodeRangePinsTheNewestOfficialReleaseInside(t *testing.T) {
	for name, tc := range map[string]struct {
		env   map[string]string
		files map[string]string
	}{
		"env":     {env: map[string]string{"NODE_VERSION": "~18.17.0"}},
		"nvmrc":   {files: map[string]string{".nvmrc": "~18.17.0\n"}},
		"engines": {files: map[string]string{"package.json": `{"engines":{"node":"~18.17.0"}}`}},
		// A partial version is a range, as node-version-alias reads it.
		"partial": {env: map[string]string{"NODE_VERSION": "v18.17"}},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeExact{
				digests: map[string]string{
					"node:18.17.0-bookworm": exactDigest, "node:18.17.1-bookworm": exactDigest,
					"node:18.17.1-bullseye": exactDigest, "node:18.18.0-bookworm": exactDigest,
					// Other variants of a newer patch are not the line's image.
					"node:18.17.2-bookworm-slim": exactDigest, "node:18.17.3-alpine": exactDigest,
				},
				env:     []string{"NODE_VERSION=18.17.1"},
				history: []string{"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'"},
			}
			choices, _ := resolveExact(t, fake, "node", tc.env, tc.files)
			if choices[0].Image != "node:18.17.1-bookworm@"+exactDigest || !strings.HasPrefix(choices[0].Narration(), "==> Using Node 18.17.1 (") {
				t.Fatalf("choice = %+v, narration %q; want node:18.17.1-bookworm", choices[0], choices[0].Narration())
			}
			if !slices.Contains(fake.asked, "list node:18.") || !slices.Contains(fake.asked, "node:18.17.1-bookworm") {
				t.Errorf("asked %v, want the listing and the chosen tag", fake.asked)
			}
		})
	}
}

// A range no official image meets in any Debian variant keeps the reviewed
// patch, and the build log says so.
func TestNodeRangeWithoutAnOfficialReleaseSaysSo(t *testing.T) {
	fake := &fakeExact{digests: map[string]string{"node:16.20.2-bookworm": exactDigest}}
	choices, _ := resolveExact(t, fake, "node", map[string]string{"NODE_VERSION": "~16.13.0"}, nil)
	line, _ := nativeToolchains["node"].find("16")
	narration := choices[0].Narration()
	if choices[0].Image != line.image || !strings.Contains(narration, "No official node:16.x (bookworm, bullseye or buster) image satisfies ~16.13.0") || !strings.Contains(narration, "(16.20.2)") {
		t.Fatalf("choice image %q, narration %q; want the reviewed image and a line naming the substitution", choices[0].Image, narration)
	}
}

// A Python service's Node companion takes NODE_VERSION's exact release too.
func TestNodeCompanionTakesTheExactRelease(t *testing.T) {
	fake := &fakeExact{digests: map[string]string{"node:18.17.1-bookworm": exactDigest}}
	choices, dockerfile := resolveExact(t, fake, "python", map[string]string{"NODE_VERSION": "18.17.1"}, nil)
	var node NativeChoice
	for _, c := range choices {
		if c.Label == "Node" {
			node = c
		}
	}
	if node.Image != "node:18.17.1-bookworm@"+exactDigest || !strings.Contains(dockerfile, "COPY --link --from=node:18.17.1-bookworm@"+exactDigest+" /usr/local/bin/node") {
		t.Fatalf("companion = %+v; want node:18.17.1-bookworm copied in\n%s", node, dockerfile)
	}
}

// A tag listing that fails keeps the reviewed patch and says why.
func TestNodeRangeListingFailureSaysSo(t *testing.T) {
	fake := &fakeExact{err: errors.New("hub answered 503")}
	choices, _ := resolveExact(t, fake, "node", map[string]string{"NODE_VERSION": "~18.17.0"}, nil)
	line, _ := nativeToolchains["node"].find("18")
	if choices[0].Image != line.image || !strings.Contains(choices[0].Narration(), "Could not list node releases for ~18.17.0 (hub answered 503)") {
		t.Fatalf("choice image %q, narration %q", choices[0].Image, choices[0].Narration())
	}
}

// w8/309: Node's bookworm images start at 16.20 and 18.16. An older release,
// asked for exactly or through a range, keeps the line's bookworm image and
// takes the release's node and node_modules from its bullseye image, before
// the legacy node-gyp patch edits that npm.
func TestNodeReleaseOlderThanBookwormIsCopiedOntoTheLine(t *testing.T) {
	const donor = "node:16.13.2-bullseye@" + exactDigest
	for name, env := range map[string]map[string]string{
		"exact": {"NODE_VERSION": "16.13.2"},
		"range": {"NODE_VERSION": "~16.13.0"},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeExact{digests: map[string]string{"node:16.13.1-bullseye": exactDigest, "node:16.13.2-bullseye": exactDigest, "node:16.20.2-bookworm": exactDigest}}
			choices, dockerfile := resolveExact(t, fake, "node", env, nil)
			line, _ := nativeToolchains["node"].find("16")
			narration := choices[0].Narration()
			if choices[0].Image != line.image || !strings.HasPrefix(narration, "==> Using Node 16.13.2 (") || !strings.Contains(narration, "copying Node 16.13.2 from node:16.13.2-bullseye onto the Node 16 image") {
				t.Fatalf("choice image %q, narration %q", choices[0].Image, narration)
			}
			copyNode := strings.Index(dockerfile, "COPY --link --from="+donor+" /usr/local/bin/node /usr/local/bin/node")
			patch := strings.Index(dockerfile, "prepare-node14-tools.py")
			if copyNode < 0 || !strings.Contains(dockerfile, "FROM "+expectedNativeBase(line.image)+"\n") || patch < copyNode {
				t.Fatalf("Dockerfile must keep the line base and copy the release before patching npm:\n%s", dockerfile)
			}
		})
	}
}

// Node 18 before 18.12 gets the legacy standalone pnpm: pnpm 10 refuses it.
func TestNodeReleaseBefore1812GetsTheLegacyPnpm(t *testing.T) {
	fake := &fakeExact{digests: map[string]string{"node:18.10.0-bullseye": exactDigest}}
	_, dockerfile := resolveExact(t, fake, "node", map[string]string{"NODE_VERSION": "18.10.0"}, nil)
	if !strings.Contains(dockerfile, nativeLegacyPnpmInstall()) || strings.Contains(dockerfile, nativePnpmInstall()) {
		t.Fatalf("Node 18.10.0 did not get the legacy pnpm:\n%s", dockerfile)
	}
	if strings.Contains(dockerfile, "prepare-node14-tools.py") {
		t.Errorf("Node 18's node-gyp needs no legacy Python patch:\n%s", dockerfile)
	}
}

// A companion Node copies an older-variant release the same way.
func TestNodeCompanionCopiesAnOlderVariantRelease(t *testing.T) {
	fake := &fakeExact{digests: map[string]string{"node:16.13.2-bullseye": exactDigest}}
	_, dockerfile := resolveExact(t, fake, "python", map[string]string{"NODE_VERSION": "16.13.2"}, nil)
	if !strings.Contains(dockerfile, "COPY --link --from=node:16.13.2-bullseye@"+exactDigest+" /usr/local/bin/node /usr/local/bin/node") {
		t.Fatalf("companion did not copy the release:\n%s", dockerfile)
	}
}

// TestOlderNodeReleaseRunsOnTheLineImage is w8/309's real-image check: it
// resolves NODE_VERSION=16.13.2 against Docker Hub, builds the Dockerfile and
// runs node, npm and pnpm from the image. Opt-in: needs Docker and the network.
func TestOlderNodeReleaseRunsOnTheLineImage(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to resolve, build and run the generated image")
	}
	o := nativeOptions()
	o.Runtime = "node"
	o.BuildCommand = "/usr/local/bin/node -v"
	o.StartCommand = `/bin/sh -c 'echo node=$(node -v) npm=$(npm -v) npx=$(npx -v) pnpm=$(pnpm -v) corepack=$(corepack -v) env=$NODE_VERSION'`
	env := map[string]string{"NODE_VERSION": "16.13.2"}
	bundle, dockerfile, src := writeResolveFixture(t, o, env, map[string]string{"package.json": `{"name":"it","version":"1.0.0"}`})
	choices, err := ResolveNativeBuildExact(context.Background(), NewDockerHubResolver(), time.Time{}, "node", bundle, dockerfile, src)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(choices[0].Narration())
	// A toolchain container without buildx hands the resolved context to the
	// host's docker instead, and checks nothing itself.
	if root := os.Getenv("BEX_NATIVE_TEST_HANDOFF"); root != "" {
		out, err := os.MkdirTemp(root, "node-donor-")
		must(t, err)
		for name, from := range map[string]string{"Dockerfile": dockerfile, "render-env": bundle, "src/package.json": filepath.Join(src, "package.json")} {
			data, err := os.ReadFile(from)
			must(t, err)
			must(t, os.MkdirAll(filepath.Dir(filepath.Join(out, name)), 0o755))
			must(t, os.WriteFile(filepath.Join(out, name), data, 0o600))
		}
		t.Skipf("resolved build context written to %s; build and run it with the host's docker", out)
	}
	tag := fmt.Sprintf("bex-native-node-donor-it:%d", os.Getpid())
	t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", "-f", tag).Run() })
	build := exec.Command("docker", "build", "-q", "-t", tag, "--secret", "id=render-env,src="+bundle, "-f", dockerfile, src)
	build.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	out, err := exec.Command("docker", "run", "--rm", tag).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "node=v16.13.2 npm=8.1.2 npx=8.1.2 pnpm=") || !strings.Contains(string(out), "env=16.13.2") {
		t.Fatalf("run = %v\n%s; want node v16.13.2 with its npm 8.1.2 and a working pnpm", err, out)
	}
	t.Log(strings.TrimSpace(string(out)))
}

// A release with no bullseye image either (Node 14.17 predates bullseye)
// takes its buster image; the copy names the release, links Corepack only
// where it exists, and sets the image's NODE_VERSION.
func TestNodeReleaseFallsBackToBuster(t *testing.T) {
	fake := &fakeExact{digests: map[string]string{"node:16.8.0-buster": exactDigest}}
	_, dockerfile := resolveExact(t, fake, "node", map[string]string{"NODE_VERSION": "16.8.0"}, nil)
	for _, want := range []string{
		"COPY --link --from=node:16.8.0-buster@" + exactDigest + " /usr/local/bin/node /usr/local/bin/node",
		"COPY --link --from=node:16.8.0-buster@" + exactDigest + " /usr/local/include/node /usr/local/include/node",
		"else rm -f /usr/local/bin/corepack; fi",
		"ENV NODE_VERSION=16.8.0\n",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Errorf("Dockerfile lacks %q:\n%s", want, dockerfile)
		}
	}
}

// w4/620: a later bump of a line's reviewed patch must not move a dated
// default: a service created since 2026-11-03 still asks for exactly 3.14.10.
func TestDatedDefaultSurvivesALinePinBump(t *testing.T) {
	saved := nativeToolchains[nativePythonRuntime]
	t.Cleanup(func() { nativeToolchains[nativePythonRuntime] = saved })
	bumped := saved
	bumped.lines = append([]nativeLine(nil), saved.lines...)
	for i := range bumped.lines {
		if bumped.lines[i].line == "3.14" {
			bumped.lines[i].version = "3.14.11"
		}
	}
	nativeToolchains[nativePythonRuntime] = bumped
	got := defaultExact(renderDefaults20261103.Add(time.Hour), nativePythonRuntime, NativeChoice{Line: "3.14"})
	if got != "3.14.10" {
		t.Fatalf("defaultExact after a 3.14.11 line bump = %q, want 3.14.10", got)
	}
}
