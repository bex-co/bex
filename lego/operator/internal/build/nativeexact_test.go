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

	"github.com/bex-co/bex/lego/types/toolchains"
)

type fakeExact struct {
	digests map[string]string
	// v1 are tags whose only manifest is in Docker's retired v1 format.
	v1         map[string]bool
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
	if f.env == nil && f.history == nil && f.envErr == nil {
		env, history = officialImageMetadata(image)
		return env, history, nil
	}
	return f.env, f.history, f.envErr
}

// officialImageMetadata is what Docker Hub serves for an official runtime
// image. Common-mode resolution reads it for every selected primary, so a fake
// without explicit metadata answers as the registry would: a reviewed line
// image carries its line's metadata, an exact tag its tag's version and suite.
func officialImageMetadata(image string) (env, history []string) {
	for runtime, toolchain := range nativeToolchains {
		for _, line := range toolchain.lines {
			if line.image == image || line.upstream == image {
				metadata := commonReviewedFixtureMetadata(runtime, line)
				return metadata.env, metadata.history
			}
		}
	}
	ref, _, _ := strings.Cut(image, "@")
	repo, tag, _ := strings.Cut(ref, ":")
	version, variant, _ := strings.Cut(tag, "-")
	suite := "bookworm"
	for _, older := range []string{"bullseye", "buster", "trixie"} {
		if strings.Contains(variant, older) {
			suite = older
		}
	}
	env = []string{"PATH=/usr/local/bin:/usr/bin"}
	switch repo {
	case nativeNodeRuntime:
		env = append(env, "NODE_VERSION="+version)
	case nativePythonRuntime:
		env = append(env, "PYTHON_VERSION="+version)
	case nativeRubyRuntime:
		env = append(env, "RUBY_VERSION="+version)
	}
	return env, commonOTPSyntheticHistory(suite)
}

func (f *fakeExact) ResolveTag(_ context.Context, repo, tag string) (string, bool, error) {
	f.asked = append(f.asked, repo+":"+tag)
	if f.v1[repo+":"+tag] {
		return "", false, fmt.Errorf("%s:%s: %w", repo, tag, errV1Manifest)
	}
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
	return resolveExactAt(t, fake, time.Time{}, runtime, env, files)
}

// resolveExactAt is resolveExact for a service created at created.
func resolveExactAt(t *testing.T, fake *fakeExact, created time.Time, runtime string, env, files map[string]string) ([]NativeChoice, string) {
	t.Helper()
	o := nativeOptions()
	o.Runtime = runtime
	bundle, dockerfile, src := writeResolveFixture(t, o, env, files)
	choices, err := ResolveNativeBuildExact(context.Background(), fake, created, runtime, bundle, dockerfile, src)
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
		// Bundler refuses any other patch than the one these name (w9/133).
		{"ruby lockfile", "ruby", nil, map[string]string{"Gemfile.lock": "RUBY VERSION\n   ruby 3.3.1p55\n", "Gemfile": "ruby '3.3.1'"}, "ruby:3.3.1-bookworm", []string{"RUBY_VERSION=3.3.1"}},
		{"ruby Gemfile", "ruby", nil, map[string]string{"Gemfile": "ruby '3.3.1'"}, "ruby:3.3.1-bookworm", []string{"RUBY_VERSION=3.3.1"}},
		{"ruby .tool-versions", "ruby", nil, map[string]string{".tool-versions": "nodejs 24.1.0\nruby 3.3.1\n"}, "ruby:3.3.1-bookworm", []string{"RUBY_VERSION=3.3.1"}},
		{"elixir env", "elixir", map[string]string{"ELIXIR_VERSION": "1.17.0"}, nil, "elixir:1.17.0", []string{"ELIXIR_VERSION=v1.17.0", "OTP_VERSION=27.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			history := []string{"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'"}
			if tc.runtime == nativeElixirRuntime {
				history = commonOTPSyntheticHistory("bookworm")
			}
			fake := &fakeExact{digests: map[string]string{tc.tag: exactDigest},
				envByImage: map[string]fakeExactMetadata{tc.tag + "@" + exactDigest: {tc.metadata, history}}}
			choices, dockerfile := resolveExact(t, fake, tc.runtime, tc.env, tc.files)
			// The exact tag is looked up first. A supplied or common base then
			// reads that image's environment; companions read their own after.
			wantAsked := []string{tc.tag}
			if nativeCommonBase != "" {
				wantAsked = append(wantAsked, "env "+tc.tag+"@"+exactDigest)
			}
			asked := fake.asked
			if nativeCommonBase != "" && len(asked) > len(wantAsked) {
				asked = asked[:len(wantAsked)]
			}
			if !slices.Equal(asked, wantAsked) {
				t.Fatalf("asked %v, want %v", fake.asked, wantAsked)
			}
			reviewed, err := ResolveNativeVersion(tc.runtime, tc.env, files(tc.files))
			if err != nil {
				t.Fatal(err)
			}
			assertExactNativePrimary(t, dockerfile, reviewed.Image, tc.tag+"@"+exactDigest, tc.metadata)
			_, version, _ := strings.Cut(strings.SplitN(tc.tag, "-", 2)[0], ":")
			if choices[0].Line != version || choices[0].Image != tc.tag+"@"+exactDigest || !strings.Contains(shown(choices[0].Narration()), version) {
				t.Errorf("narration %q does not name %s", shown(choices[0].Narration()), version)
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
			asked := fake.asked
			if nativeCommonBase != "" {
				// The common base reads each selected image's environment and
				// looks for a Bookworm OTP; neither is an exact-tag lookup.
				asked = slices.DeleteFunc(slices.Clone(asked), func(entry string) bool {
					return strings.HasPrefix(entry, "env ") || strings.HasPrefix(entry, "list "+hexpmErlangRepo+":")
				})
			}
			if len(asked) != 0 {
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
			// On the common base the reviewed tree is the one copied in.
			kept := !strings.Contains(dockerfile, "rm -rf /usr/local")
			if nativeCommonBase != "" {
				kept = strings.Contains(dockerfile, "COPY --link --from="+line.image+" /usr/local/ /usr/local/\n")
			}
			if choices[0].Image != line.image || !strings.Contains(dockerfile, "FROM "+expectedNativeBase(line.image)+"\n") || !kept {
				t.Errorf("Dockerfile left the reviewed image:\n%s", dockerfile)
			}
			narration := shown(choices[0].Narration())
			if !strings.HasPrefix(narration, "==> Using Python version "+line.version+" via environment variable PYTHON_VERSION\n") || !strings.Contains(narration, "python:3.11.9-bookworm") {
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

// w8/198, w9/138, w3/m636: a Ruby service created since a dated step with no
// version signal builds on Render's default patch for that date, and the log
// names it; an unknown-age service (or one older than Render's 2025-06-12 row,
// which no bex service is), and any explicit signal, keep the line's reviewed
// patch. Whether a step is that reviewed patch (no swap) or behind it
// (its own official image) is read from the table, so refreshing the 3.4 line
// needs no edit here.
func TestRubyDefaultIsRendersExactPatch(t *testing.T) {
	exactEnv := map[string][]string{"3.4.4": {"RUBY_VERSION=3.4.4"}, "3.4.12": {"RUBY_VERSION=3.4.12"}, "3.4.13": {"RUBY_VERSION=3.4.13"}}
	fake := &fakeExact{digests: map[string]string{}, envByImage: map[string]fakeExactMetadata{}}
	for version, env := range exactEnv {
		fake.digests["ruby:"+version+"-bookworm"] = exactDigest
		fake.envByImage["ruby:"+version+"-bookworm@"+exactDigest] = fakeExactMetadata{env, []string{"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'"}}
	}
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
		if env, ok := exactEnv[choices[0].Line]; ok {
			out, err := os.ReadFile(dockerfile)
			must(t, err)
			assertExactNativePrimary(t, string(out), reviewed.image, choices[0].Image, env)
		}
		return choices[0]
	}
	wantDefault := func(name string, c NativeChoice, version string) {
		t.Helper()
		line, image := version, "ruby:"+version+"-bookworm@"+exactDigest
		if _, ok := rubyReviewedDefault(version); ok {
			line, image = reviewed.line, reviewed.image
		}
		if c.Line != line || c.Image != image || shown(c.Narration()) != "==> Using Ruby version "+version+" (default)"+rubyDocs {
			t.Errorf("%s must build on %s (reviewed %s): %+v narration %q", name, version, reviewed.version, c, shown(c.Narration()))
		}
	}

	// w3/m636: Render's 3.4.4 row runs from 2025-06-12, so every service bex
	// created before 2026-11-03 gets it, not the line's pin.
	wantDefault("service created at Render's 2025-06-12 row", run(renderDefaults20250612, nil), "3.4.4")
	wantDefault("service created 2026-08-01", run(on2026(time.August, 1), nil), "3.4.4")
	// w4/620: Render's default moved to 3.4.12 for services created since
	// 2026-11-03; one created before keeps 3.4.4.
	wantDefault("service created after 2026-11-03", run(toolchains.RenderDefaults20261103.Add(time.Hour), nil), "3.4.12")
	wantDefault("service created 2026-11-05", run(on2026(time.November, 5), nil), "3.4.12")
	wantDefault("service created just before 2026-11-03", run(toolchains.RenderDefaults20261103.Add(-time.Hour), nil), "3.4.4")
	// w9/138: Render's default moved to 3.4.13 for services created since
	// 2026-11-19; one created the hour before keeps 3.4.12.
	wantDefault("service created after 2026-11-19", run(renderRubyDefault20261119.Add(time.Hour), nil), "3.4.13")
	wantDefault("service created 2026-11-20", run(on2026(time.November, 20), nil), "3.4.13")
	wantDefault("service created just before 2026-11-19", run(renderRubyDefault20261119.Add(-time.Hour), nil), "3.4.12")
	for name, c := range map[string]NativeChoice{
		"created before Render's 2025-06-12 row": run(renderDefaults20250612.Add(-time.Hour), nil),
		"unknown age":                            run(time.Time{}, nil),
		"explicit signal":                        run(renderDefaults20250612.Add(time.Hour), map[string]string{".ruby-version": "3.4\n"}),
		"explicit signal after 2026-11-19":       run(renderRubyDefault20261119.Add(time.Hour), map[string]string{".ruby-version": "3.4\n"}),
	} {
		if c.Line != "3.4" || c.Image != reviewed.image {
			t.Errorf("%s must keep the reviewed 3.4 patch: %+v", name, c)
		}
	}

	// A step behind the reviewed patch whose tag is missing falls back to the
	// reviewed patch and says so; 3.4.4 always is one.
	missing := &fakeExact{digests: map[string]string{}}
	steps := map[time.Time]string{renderDefaults20250612.Add(time.Hour): "3.4.4", toolchains.RenderDefaults20261103.Add(time.Hour): "3.4.12", renderRubyDefault20261119.Add(time.Hour): "3.4.13"}
	for created, version := range steps {
		if _, ok := rubyReviewedDefault(version); ok {
			continue
		}
		o := nativeOptions()
		o.Runtime = "ruby"
		bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
		choices, err := ResolveNativeBuildExact(context.Background(), missing, created, "ruby", bundle, dockerfile, src)
		if err != nil || choices[0].Line != "3.4" || choices[0].Image != reviewed.image || !strings.Contains(shown(choices[0].Narration()), "No official image ruby:"+version+"-bookworm") {
			t.Errorf("a missing %s tag must fall back to the reviewed patch and say so: %+v %v", version, choices[0], err)
		}
	}
}

// w6/m291: Render's default Node (24.22.1 since 2026-10-21) and Elixir (1.18.4
// since 2025-06-12) are exact steps, so refreshing a line to a newer patch
// leaves a new service's default where Render has it. An explicit line request,
// an unknown-age service, one older than the runtime's first step and a
// missing tag keep the reviewed patch.
//
// An Elixir service with no signal also gets Render's default Erlang/OTP
// 28.0.2 (w8/333), so its exact image is that release's.
func TestNodeAndElixirDefaultsAreRendersExactPatch(t *testing.T) {
	for _, tc := range []struct {
		runtime, line, exact, tag string
		from, first               time.Time
		explicit                  map[string]string
		published                 *fakeExact
		image                     string   // the new service's
		imageEnv                  []string // its image's environment
		narration                 string   // Render's line (w9/139)
	}{
		{nativeNodeRuntime, "24", "24.22.1", "node:24.22.1-bookworm", renderNodeDefault20261021, renderNodeDefault20260421, map[string]string{"NODE_VERSION": "24"},
			&fakeExact{digests: map[string]string{"node:24.22.1-bookworm": exactDigest}}, "node:24.22.1-bookworm@" + exactDigest, nil,
			"==> Using Node.js version 24.22.1 (default)" + nodeDocs},
		{nativeElixirRuntime, "1.18", "1.18.4", "elixir:1.18.4", renderDefaults20250612, renderDefaults20250612, map[string]string{"ELIXIR_VERSION": "1.18"},
			defaultOTPPublished(), defaultOTPImage, defaultOTPEnv,
			"==> Using Erlang version 28.0.2 (default)\n==> Using Elixir version 1.18.4 (default)" + elixirDocs},
	} {
		t.Run(tc.runtime, func(t *testing.T) {
			reviewed, _ := nativeToolchains[tc.runtime].find(tc.line)
			if reviewed.version == tc.exact {
				t.Fatalf("the %s line's reviewed patch is Render's default %s again; this test no longer proves a pin bump leaves the default alone", tc.line, tc.exact)
			}
			run := func(created time.Time, env map[string]string) NativeChoice {
				t.Helper()
				choices, dockerfile := resolveExactAt(t, tc.published, created, tc.runtime, env, nil)
				if choices[0].Image == tc.image {
					assertExactNativePrimary(t, dockerfile, reviewed.image, choices[0].Image, tc.imageEnv)
				}
				return choices[0]
			}

			got := run(tc.from.Add(time.Hour), nil)
			if got.Line != tc.exact || got.Image != tc.image || shown(got.Narration()) != tc.narration {
				t.Errorf("new service: %+v narration %q, want %q", got, shown(got.Narration()), tc.narration)
			}
			for name, c := range map[string]NativeChoice{
				"unknown age":           run(time.Time{}, nil),
				"explicit line":         run(tc.from.Add(time.Hour), tc.explicit),
				"before the first step": run(tc.first.Add(-time.Hour), nil),
			} {
				if c.Line != tc.line || c.Image != reviewed.image {
					t.Errorf("%s must keep the reviewed %s: %+v", name, reviewed.version, c)
				}
			}

			missing := &fakeExact{digests: map[string]string{}}
			choices, _ := resolveExactAt(t, missing, tc.from.Add(time.Hour), tc.runtime, nil, nil)
			if choices[0].Line != tc.line || !strings.Contains(shown(choices[0].Narration()), "No official image "+tc.tag) {
				t.Errorf("a missing tag must fall back to the reviewed patch and say so: %+v", choices[0])
			}
		})
	}
}

// w8/217, w3/m636: a Python service created since Render's 2026-02-11 row
// with no version signal builds on Render's default for its date (3.14.3, then
// 3.14.8 from 2026-10-12), not the 3.14 line's newest reviewed patch;
// PYTHON_VERSION=3.14 keeps the reviewed patch, and an unknown-age service
// (or one older than the row, which no bex service is) keeps its 3.13 default.
func TestPythonDefaultIsRendersExactPatch(t *testing.T) {
	exactEnv := map[string][]string{"3.14.3": {"PYTHON_VERSION=3.14.3"}, "3.14.8": {"PYTHON_VERSION=3.14.8"}}
	fake := &fakeExact{digests: map[string]string{}, envByImage: map[string]fakeExactMetadata{}}
	for version, env := range exactEnv {
		fake.digests["python:"+version+"-bookworm"] = exactDigest
		fake.envByImage["python:"+version+"-bookworm@"+exactDigest] = fakeExactMetadata{env, []string{"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'"}}
	}
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
		if env, ok := exactEnv[choices[0].Line]; ok {
			out, err := os.ReadFile(dockerfile)
			must(t, err)
			assertExactNativePrimary(t, string(out), reviewed.image, choices[0].Image, env)
		}
		return choices[0]
	}
	wantExact := func(name string, c NativeChoice, version string) {
		t.Helper()
		if c.Line != version || c.Image != "python:"+version+"-bookworm@"+exactDigest || shown(c.Narration()) != "==> Using Python version "+version+" (default)"+pythonDocs {
			t.Errorf("%s must build on %s: %+v narration %q", name, version, c, shown(c.Narration()))
		}
	}

	wantExact("service created at Render's 2026-10-12 row", run(toolchains.RenderDefaults20261012, nil), "3.14.8")
	wantExact("service created 2026-10-15", run(on2026(time.October, 15), nil), "3.14.8")
	wantExact("service created just before 2026-10-12", run(toolchains.RenderDefaults20261012.Add(-time.Hour), nil), "3.14.3")
	wantExact("service created 2026-05-01", run(on2026(time.May, 1), nil), "3.14.3")
	wantExact("service created at Render's 2026-02-11 row", run(toolchains.RenderDefaults20260211, nil), "3.14.3")
	// w4/620: Render's default moved to 3.14.10, the line's reviewed patch, for
	// services created since 2026-11-03; one created before keeps 3.14.8.
	if c := run(toolchains.RenderDefaults20261103.Add(time.Hour), nil); c.Line != "3.14" || c.Image != reviewed.image || reviewed.version != "3.14.10" {
		t.Errorf("service created after 2026-11-03 must build on the reviewed 3.14.10: %+v (reviewed %s)", c, reviewed.version)
	}
	// …and its log names that exact patch, not the bare line (w4/668).
	if c := run(toolchains.RenderDefaults20261103.Add(time.Hour), nil); shown(c.Narration()) != "==> Using Python version 3.14.10 (default)"+pythonDocs {
		t.Errorf("narration = %q, want the exact 3.14.10", shown(c.Narration()))
	}
	if c := run(toolchains.RenderDefaults20261103.Add(-time.Hour), nil); c.Line != "3.14.8" {
		t.Errorf("service created just before 2026-11-03 must keep 3.14.8: %+v", c)
	}
	if c := run(toolchains.RenderDefaults20261012.Add(time.Hour), map[string]string{"PYTHON_VERSION": "3.14"}); c.Line != "3.14" || c.Image != reviewed.image {
		t.Errorf("PYTHON_VERSION=3.14 must keep the reviewed %s: %+v", reviewed.version, c)
	}
	legacy, _ := nativeToolchains[nativePythonRuntime].find("3.13")
	for name, c := range map[string]NativeChoice{
		"created before Render's 2026-02-11 row": run(toolchains.RenderDefaults20260211.Add(-time.Hour), nil),
		"unknown age":                            run(time.Time{}, nil),
	} {
		if c.Line != "3.13" || c.Image != legacy.image {
			t.Errorf("%s must keep the 3.13 default: %+v", name, c)
		}
	}

	missing := &fakeExact{digests: map[string]string{}}
	o := nativeOptions()
	o.Runtime = "python"
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
	choices, err := ResolveNativeBuildExact(context.Background(), missing, toolchains.RenderDefaults20261012.Add(time.Hour), "python", bundle, dockerfile, src)
	if err != nil || choices[0].Line != "3.14" || !strings.Contains(shown(choices[0].Narration()), "No official image python:3.14.8-bookworm") {
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
		via   string // Render's words for the signal (w9/139)
		// requesting is the line Render's log opens with for a range (w9/141).
		requesting string
	}{
		"env":     {env: map[string]string{"NODE_VERSION": "~18.17.0"}, via: "environment variable NODE_VERSION", requesting: "==> Requesting Node.js version ~18.17.0\n"},
		"nvmrc":   {files: map[string]string{".nvmrc": "~18.17.0\n"}, via: "/opt/render/project/src/.nvmrc", requesting: "==> Requesting Node.js version ~18.17.0\n"},
		"engines": {files: map[string]string{"package.json": `{"engines":{"node":"~18.17.0"}}`}, via: "/opt/render/project/src/package.json", requesting: "==> Requesting Node.js version ~18.17.0\n"},
		// A partial version is a range, as node-version-alias reads it. Whether
		// Render's log echoes one is unmeasured, so bex does not.
		"partial": {env: map[string]string{"NODE_VERSION": "v18.17"}, via: "environment variable NODE_VERSION"},
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
			if choices[0].Image != "node:18.17.1-bookworm@"+exactDigest || shown(choices[0].Narration()) != tc.requesting+"==> Using Node.js version 18.17.1 via "+tc.via+nodeDocs {
				t.Fatalf("choice = %+v, narration %q; want node:18.17.1-bookworm", choices[0], shown(choices[0].Narration()))
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
	narration := shown(choices[0].Narration())
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
	if choices[0].Image != line.image || !strings.Contains(shown(choices[0].Narration()), "Could not list node releases for ~18.17.0 (hub answered 503)") {
		t.Fatalf("choice image %q, narration %q", choices[0].Image, shown(choices[0].Narration()))
	}
}

// w8/309: Node's bookworm images start at 16.20 and 18.16. An older release,
// asked for exactly or through a range, keeps the line's bookworm image and
// takes the release's node and node_modules from its bullseye image, before
// the legacy node-gyp patch edits that npm.
func TestNodeReleaseOlderThanBookwormIsCopiedOntoTheLine(t *testing.T) {
	const donor = "node:16.13.2-bullseye@" + exactDigest
	for name, tc := range map[string]struct {
		env        map[string]string
		requesting string
	}{
		"exact": {env: map[string]string{"NODE_VERSION": "16.13.2"}},
		"range": {env: map[string]string{"NODE_VERSION": "~16.13.0"}, requesting: "==> Requesting Node.js version ~16.13.0\n"},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeExact{digests: map[string]string{"node:16.13.1-bullseye": exactDigest, "node:16.13.2-bullseye": exactDigest, "node:16.20.2-bookworm": exactDigest}}
			choices, dockerfile := resolveExact(t, fake, "node", tc.env, nil)
			line, _ := nativeToolchains["node"].find("16")
			narration := shown(choices[0].Narration())
			// On the common base the release's own image is the copied primary;
			// on the official line image only its node is copied in.
			wantImage, wantNote, wantCopy := line.image, "copying Node 16.13.2 from node:16.13.2-bullseye onto the Node 16 image", " /usr/local/bin/node /usr/local/bin/node"
			if nativeCommonBase != "" {
				wantImage, wantNote, wantCopy = donor, "using Node 16.13.2 from node:16.13.2-bullseye", " /usr/local/ /usr/local/"
			}
			if choices[0].Image != wantImage || !strings.HasPrefix(narration, tc.requesting+"==> Using Node.js version 16.13.2 via environment variable NODE_VERSION"+nodeDocs+"\n") || !strings.Contains(narration, wantNote) {
				t.Fatalf("choice image %q, narration %q", choices[0].Image, narration)
			}
			copyNode := strings.Index(dockerfile, "COPY --link --from="+donor+wantCopy)
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

// w8/370: a companion Node with no version signal takes the dated default a
// Node service created at the same time gets, as Render's Python, Ruby and Go
// builds name theirs. An unknown-age service (or one older than the table's
// first step) keeps the reviewed patch.
func TestNodeCompanionDefaultFollowsTheServiceCreationDate(t *testing.T) {
	reviewed, _ := nativeToolchains[nativeNodeRuntime].find("24")
	if reviewed.version == "24.22.1" {
		t.Fatal("the 24 line's reviewed patch is Render's default 24.22.1 again; this test no longer separates the two")
	}
	const image = "node:24.22.1-bookworm@" + exactDigest
	fake := &fakeExact{digests: map[string]string{"node:24.22.1-bookworm": exactDigest}}
	companion := func(t *testing.T, runtime string, created time.Time) (NativeChoice, string) {
		t.Helper()
		choices, dockerfile := resolveExactAt(t, fake, created, runtime, nil, nil)
		for _, c := range choices[1:] {
			if c.Label == "Node" {
				return c, dockerfile
			}
		}
		t.Fatalf("no Node companion in %+v", choices)
		return NativeChoice{}, ""
	}
	common := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = common })
	for _, base := range []string{common, ""} {
		nativeCommonBase = base
		for runtime := range nativeJSCompanionRuntimes {
			t.Run(fmt.Sprintf("%s/common base %t", runtime, base != ""), func(t *testing.T) {
				node, dockerfile := companion(t, runtime, renderNodeDefault20261021.Add(time.Hour))
				if node.Image != image || shown(node.Narration()) != "==> Using Node.js version 24.22.1 (default)"+nodeDocs || !strings.Contains(dockerfile, "COPY --link --from="+image+" ") {
					t.Errorf("new service: %+v narration %q\n%s", node, shown(node.Narration()), dockerfile)
				}
				for name, created := range map[string]time.Time{"unknown age": {}, "before the first step": renderNodeDefault20260421.Add(-time.Hour)} {
					if node, _ := companion(t, runtime, created); node.Image != reviewed.image || shown(node.Narration()) != "==> Using Node.js version "+reviewed.version+" (default)"+nodeDocs {
						t.Errorf("%s must keep the reviewed %s: %+v narration %q", name, reviewed.version, node, shown(node.Narration()))
					}
				}
			})
		}
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
	t.Log(shown(choices[0].Narration()))
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
	wants := []string{
		"COPY --link --from=node:16.8.0-buster@" + exactDigest + " /usr/local/bin/node /usr/local/bin/node",
		"COPY --link --from=node:16.8.0-buster@" + exactDigest + " /usr/local/include/node /usr/local/include/node",
		"else rm -f /usr/local/bin/corepack; fi",
		"ENV NODE_VERSION=16.8.0\n",
	}
	if nativeCommonBase != "" {
		// The common base copies the whole buster tree, Corepack link and all,
		// behind a guard that admits exactly a buster primary on bookworm.
		wants = []string{
			"COPY --link --from=node:16.8.0-buster@" + exactDigest + " /usr/local/ /usr/local/",
			`[ "$primary_os" = buster ] && [ "$VERSION_CODENAME" = bookworm ]`,
			`ENV NODE_VERSION="16.8.0"`,
		}
	}
	for _, want := range wants {
		if !strings.Contains(dockerfile, want) {
			t.Errorf("Dockerfile lacks %q:\n%s", want, dockerfile)
		}
	}
}

// w9/133: Ruby's bookworm images start at 3.1.4 and 3.2.2. An older release,
// such as render-examples/sidekiq's locked 3.2.0, takes its tree from its
// bullseye image onto the common base, which carries that image's libssl1.1
// and libffi7 for the 3.0 line; the log says so. With no image in either variant the build keeps the
// reviewed patch and says that instead, as every exact path does.
func TestRubyReleaseOlderThanBookwormComesFromBullseye(t *testing.T) {
	const donor = "ruby:3.2.0-bullseye@" + exactDigest
	sidekiq := map[string]string{"Gemfile.lock": sidekiqLock, "Gemfile": "source 'https://rubygems.org'\nruby '3.2.0'\n"}
	line, _ := nativeToolchains[nativeRubyRuntime].find("3.2")
	fake := &fakeExact{digests: map[string]string{"ruby:3.2.0-bullseye": exactDigest, "ruby:3.2.0-buster": exactDigest}}
	choices, dockerfile := resolveExact(t, fake, nativeRubyRuntime, nil, sidekiq)
	want := "==> Using Ruby version 3.2.0 via /opt/render/project/src/Gemfile.lock" + rubyDocs + "\n" +
		"==> ruby:3.2.0-bookworm does not exist; using Ruby 3.2.0 from ruby:3.2.0-bullseye"
	if choices[0].Image != donor || choices[0].Line != "3.2.0" || shown(choices[0].Narration()) != want {
		t.Fatalf("choice %+v, narration %q; want %s and %q", choices[0], shown(choices[0].Narration()), donor, want)
	}
	if slices.Contains(fake.asked, "ruby:3.2.0-buster") {
		t.Errorf("a buster Ruby needs libffi6, which no base carries; asked %v", fake.asked)
	}
	// The final base stays the common bookworm image. Its guard admits exactly
	// a bullseye primary on it, and the build checks the interpreter before
	// any authored command.
	for _, w := range []string{
		"\nFROM " + nativeCommonBase + "\n",
		"COPY --link --from=" + donor + " /usr/local/ /usr/local/\n",
		`[ "$primary_os" = bullseye ] && [ "$VERSION_CODENAME" = bookworm ]`,
		`ENV RUBY_VERSION="3.2.0"`,
		`[ "$(/usr/local/bin/ruby -e 'puts RUBY_VERSION')" = 3.2.0 ]`,
	} {
		if !strings.Contains(dockerfile, w) {
			t.Errorf("Dockerfile lacks %q:\n%s", w, dockerfile)
		}
	}
	if strings.Contains(dockerfile, "COPY --link --from="+line.image+" /usr/local/ /usr/local/") {
		t.Errorf("the reviewed %s tree is still copied:\n%s", line.version, dockerfile)
	}

	missing, _ := resolveExact(t, &fakeExact{digests: map[string]string{}}, nativeRubyRuntime, nil, sidekiq)
	want = "==> Using Ruby version " + line.version + " via /opt/render/project/src/Gemfile.lock" + rubyDocs + "\n" +
		"==> No official image ruby:3.2.0-bookworm; using the reviewed Ruby 3.2 image"
	if missing[0].Image != line.image || shown(missing[0].Narration()) != want {
		t.Errorf("no image in any variant: %+v narration %q, want %q", missing[0], shown(missing[0].Narration()), want)
	}
}

// w9/138, w3/m635: with the 3.4 line on 3.4.13 (refreshed 2026-11-26), a Ruby
// service created from 2026-11-03 until 2026-11-19 stays on exactly 3.4.12, as
// Render's table does, and one created since 2026-11-19 builds on the line's
// patch with no swap.
func TestRubyDatedDefaultsSurviveALineRefreshToTheirNewestStep(t *testing.T) {
	withLineVersion(t, nativeRubyRuntime, "3.4", "3.4.13")
	for created, want := range map[time.Time]string{
		renderRubyDefault20261119.Add(-time.Hour): "3.4.12",
		renderRubyDefault20261119:                 "",
		renderRubyDefault20261119.Add(time.Hour):  "",
	} {
		if got := defaultExact(created, nativeRubyRuntime, NativeChoice{Line: "3.4"}); got != want {
			t.Errorf("defaultExact(%s) after a 3.4.13 line refresh = %q, want %q", created, got, want)
		}
	}
}

// w3/m636: a Node service, and the companion Node of every other runtime,
// created on any date since bex's first service (2026-06-05) builds on the
// patch Render's node-version table gives that date: 24.14.1 from 2026-04-21,
// 24.21.0 from 2026-09-17 and 24.22.1 from 2026-10-21, never the 24 line's pin.
func TestNodeDefaultFollowsRendersDatedRows(t *testing.T) {
	fake := &fakeExact{digests: map[string]string{
		"node:24.14.1-bookworm": exactDigest, "node:24.21.0-bookworm": exactDigest, "node:24.22.1-bookworm": exactDigest}}
	for _, tc := range []struct {
		name    string
		created time.Time
		want    string
	}{
		{"created when bex's first service could be", on2026(time.June, 5), "24.14.1"},
		{"created 2026-08-01", on2026(time.August, 1), "24.14.1"},
		{"created just before 2026-09-17", renderNodeDefault20260917.Add(-time.Hour), "24.14.1"},
		{"created at 2026-09-17", renderNodeDefault20260917, "24.21.0"},
		{"created 2026-10-01", on2026(time.October, 1), "24.21.0"},
		{"created just before 2026-10-21", renderNodeDefault20261021.Add(-time.Hour), "24.21.0"},
		{"created at 2026-10-21", renderNodeDefault20261021, "24.22.1"},
		{"created 2026-11-20", on2026(time.November, 20), "24.22.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			image := "node:" + tc.want + "-bookworm@" + exactDigest
			narration := "==> Using Node.js version " + tc.want + " (default)" + nodeDocs
			choices, _ := resolveExactAt(t, fake, tc.created, nativeNodeRuntime, nil, nil)
			if c := choices[0]; c.Line != tc.want || c.Image != image || shown(c.Narration()) != narration {
				t.Errorf("node service: %+v narration %q, want %s", c, shown(c.Narration()), tc.want)
			}
			for runtime := range nativeJSCompanionRuntimes {
				choices, _ := resolveExactAt(t, fake, tc.created, runtime, nil, nil)
				i := slices.IndexFunc(choices[1:], func(c NativeChoice) bool { return c.Label == "Node" })
				if i < 0 {
					t.Fatalf("%s: no Node companion in %+v", runtime, choices)
				}
				if c := choices[1+i]; c.Image != image || shown(c.Narration()) != narration {
					t.Errorf("%s companion: %+v narration %q, want %s", runtime, c, shown(c.Narration()), tc.want)
				}
			}
		})
	}
}

// w3/m636 (the w8/217 regression class): refreshing a line's reviewed patch
// never moves a dated default. With each runtime's default line bumped past
// all of its steps, a service created at or inside each step still asks for
// exactly that step's patch, and only an unknown-age service follows the
// bumped pin. Each step is on its runtime's default line, the line a service
// with no signal resolves to, or defaultExact would drop it.
func TestDatedDefaultsSurviveALinePinBump(t *testing.T) {
	for runtime, steps := range exactDefaults {
		t.Run(runtime, func(t *testing.T) {
			line := nativeToolchains[runtime].defaultLine
			bumped := line + ".999"
			if !strings.Contains(line, ".") {
				bumped += ".0"
			}
			withLineVersion(t, runtime, line, bumped)
			assertAscending(t, runtime, steps)
			for _, step := range steps {
				if !strings.HasPrefix(step.Version, line+".") {
					t.Errorf("step %s is not on the %s default line %s", step.Version, runtime, line)
				}
				for _, created := range []time.Time{step.From, step.From.Add(time.Hour)} {
					if got := defaultExact(created, runtime, NativeChoice{Line: line}); got != step.Version {
						t.Errorf("a service created %s after the %s line moved to %s gets %q, want exactly %s", created, line, bumped, got, step.Version)
					}
				}
			}
			if got := defaultExact(time.Time{}, runtime, NativeChoice{Line: line}); got != "" {
				t.Errorf("an unknown-age service gets %q, want the bumped pin %s", got, bumped)
			}
		})
	}
}

// w3/m636: Erlang/OTP's and uv's dated defaults have no line pin to follow,
// but their tables keep the same shape: ascending, Erlang/OTP in the
// four-part form that names one release, and uv an exact release.
func TestErlangAndUVDatedDefaultsAreWellFormed(t *testing.T) {
	assertAscending(t, "erlang", erlangDefaults)
	for _, step := range erlangDefaults {
		if strings.Count(step.Version, ".") != 3 {
			t.Errorf("erlang step %s does not name exactly one release", step.Version)
		}
	}
	assertAscending(t, "uv", uvDefaults)
	for _, step := range uvDefaults {
		if !uvRelease.MatchString(step.Version) {
			t.Errorf("uv step %s is not an exact release", step.Version)
		}
	}
	if newest := uvDefaults[len(uvDefaults)-1].Version; newest != defaultUVVersion {
		t.Errorf("uv's newest dated default %s is not defaultUVVersion %s, which refusals suggest", newest, defaultUVVersion)
	}
}

// on2026 is midnight UTC on a day of 2026, a service's creation time in tests.
func on2026(month time.Month, day int) time.Time {
	return time.Date(2026, month, day, 0, 0, 0, 0, time.UTC)
}

// assertAscending fails for a dated table whose steps are not in date order,
// which DatedDefaultAt relies on.
func assertAscending(t *testing.T, name string, steps []toolchains.DatedDefault) {
	t.Helper()
	for i := 1; i < len(steps); i++ {
		if !steps[i-1].From.Before(steps[i].From) {
			t.Errorf("%s step %s (%s) is not after step %s (%s)", name, steps[i].Version, steps[i].From, steps[i-1].Version, steps[i-1].From)
		}
	}
}

// withLineVersion sets the reviewed patch of one of runtime's lines for the
// rest of the test, as a toolchain refresh would.
func withLineVersion(t *testing.T, runtime, line, version string) {
	t.Helper()
	saved := nativeToolchains[runtime]
	t.Cleanup(func() { nativeToolchains[runtime] = saved })
	bumped := saved
	bumped.lines = slices.Clone(saved.lines)
	for i := range bumped.lines {
		if bumped.lines[i].line == line {
			bumped.lines[i].version = version
		}
	}
	nativeToolchains[runtime] = bumped
}
