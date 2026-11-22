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
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
)

// w8/398: Render resolves every Node request with node-version-alias, the
// newest release in nodejs.org's index that satisfies it, odd lines included
// (render.com/docs/node-version). Selection names that release when a line's
// final release settles it, or the line whose tags the resolve phase lists.
func TestNodeRequestResolvesOddLinesLikeNodeVersionAlias(t *testing.T) {
	engines := func(rng string) map[string]string {
		return map[string]string{"package.json": `{"engines":{"node":"` + rng + `"}}`}
	}
	for _, tc := range []struct {
		name                      string
		env, files                map[string]string
		line, exact, release, rng string
		unreviewed                bool
	}{
		{name: ".nvmrc odd release", files: map[string]string{".nvmrc": "25.8.1\n"}, line: "24", exact: "25.8.1", unreviewed: true},
		{name: "odd major", env: map[string]string{"NODE_VERSION": "v25"}, line: "24", exact: "25.9.0", unreviewed: true},
		{name: "range below an even line", env: map[string]string{"NODE_VERSION": ">=20 <22"}, line: "24", exact: "21.7.3", unreviewed: true},
		{name: "engines range below the newest line", files: engines(">=24 <26"), line: "24", exact: "25.9.0", unreviewed: true},
		{name: ".node-version 19", files: map[string]string{".node-version": "19"}, line: "24", exact: "19.9.0", unreviewed: true},
		{name: "range below 16", files: engines(">=14 <16"), line: "24", exact: "15.14.0", unreviewed: true},
		{name: "older minor of an odd line", env: map[string]string{"NODE_VERSION": "25.8"}, line: "24", release: "25", rng: "25.8", unreviewed: true},
		{name: "tilde range an odd final misses", files: engines("~21.6.0"), line: "24", release: "21", rng: "~21.6.0", unreviewed: true},
		// A release named exactly from Node 14 on is the registry's to confirm.
		{name: "a future major", env: map[string]string{"NODE_VERSION": "27.0.0"}, line: "24", exact: "27.0.0", unreviewed: true},
		{name: "no such release", env: map[string]string{"NODE_VERSION": "99.99.99"}, line: "24", exact: "99.99.99", unreviewed: true},
		// The reviewed lines are unchanged.
		{name: "open range", files: engines(">=20"), line: "26"},
		{name: "caret on a reviewed line", files: engines("^22"), line: "22"},
		{name: "range reaching an even line first", files: engines(">=20 <23"), line: "22"},
		{name: "exact on a reviewed line", env: map[string]string{"NODE_VERSION": "22.11.0"}, line: "22", exact: "22.11.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveNativeVersion(nativeNodeRuntime, tc.env, files(tc.files))
			if err != nil {
				t.Fatal(err)
			}
			if got.Line != tc.line || got.Exact != tc.exact || got.releaseLine != tc.release || got.rangeRequest != tc.rng || got.unreviewed != tc.unreviewed {
				t.Fatalf("got line %q exact %q release line %q range %q unreviewed %t; want %q %q %q %q %t",
					got.Line, got.Exact, got.releaseLine, got.rangeRequest, got.unreviewed, tc.line, tc.exact, tc.release, tc.rng, tc.unreviewed)
			}
			if line, _ := nativeToolchains[nativeNodeRuntime].find(tc.line); got.Image != line.image {
				t.Errorf("image %q is not the %s line's pin", got.Image, tc.line)
			}
		})
	}

	// What no Node release can be stays refused at version selection, and the
	// refusal says that any other release installs.
	for _, tc := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"NODE_VERSION": "12.22.12"}, "NODE_VERSION=12.22.12 is not a supported Node version; supported Node lines are 14, 16, 18, 20, 22, 24, 26, and " + nativeNodeAnyRelease},
		{map[string]string{"NODE_VERSION": "27"}, "NODE_VERSION=27 is not a supported Node version"},
		{map[string]string{"NODE_VERSION": ">=27"}, "NODE_VERSION=>=27 is not a supported Node version"},
		{map[string]string{"NODE_VERSION": "<14"}, "NODE_VERSION=<14 is not a supported Node version"},
	} {
		_, err := ResolveNativeVersion(nativeNodeRuntime, tc.env, files(nil))
		if _, tenant := errors.AsType[*UnsupportedVersionError](err); !tenant || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: err = %v, want a tenant error containing %q", tc.env, err, tc.want)
		}
	}
}

// The odd lines' final releases are frozen facts of nodejs.org's index: each
// is an odd major no reviewed line carries, and none moves.
func TestNodeOddLinesAreFinalReleasesOfUnreviewedMajors(t *testing.T) {
	node := nativeToolchains[nativeNodeRuntime]
	for _, l := range node.released {
		v, err := semver.StrictNewVersion(l.version)
		if err != nil || v.Major()%2 == 0 || l.line != strings.Split(l.version, ".")[0] || l.image != "" {
			t.Errorf("%+v is not an odd line at its final release", l)
		}
		if _, reviewed := node.find(l.line); reviewed {
			t.Errorf("line %s is reviewed and released both", l.line)
		}
	}
	if !slices.IsSortedFunc(node.rangeLines(), func(a, b nativeLine) int {
		return semver.MustParse(a.version).Compare(semver.MustParse(b.version))
	}) || len(node.rangeLines()) != len(node.lines)+len(node.released) {
		t.Errorf("range lines %+v are not every line in release order", node.rangeLines())
	}
}

// nodeIndexFixture is a slice of nodejs.org's release index: every reviewed
// line's pinned patch (the newest release of its line, which the toolchain
// refresh keeps), each odd line's final release, and some older releases.
func nodeIndexFixture() []string {
	lines := nativeToolchains[nativeNodeRuntime].lines
	pins := make([]string, len(lines))
	for i, l := range lines {
		pins[i] = l.version
	}
	return slices.Concat(pins, []string{"14.17.0", "18.17.1", "20.11.0", "22.0.0", "24.22.1",
		"15.0.1", "17.9.1", "19.9.0", "21.0.0", "21.6.0", "21.6.2", "21.7.3", "23.11.1", "25.8.0", "25.8.1", "25.9.0", "15.14.0"})
}

// node-version-alias's answer: semver.maxSatisfying over the index.
func maxSatisfying(t *testing.T, index []string, request string) string {
	t.Helper()
	c, err := semver.NewConstraint(strings.TrimPrefix(request, "v"))
	if err != nil {
		t.Fatal(err)
	}
	var best *semver.Version
	for _, release := range index {
		if v := semver.MustParse(release); c.Check(v) && (best == nil || v.GreaterThan(best)) {
			best = v
		}
	}
	return best.Original()
}

// The release a build installs is the one node-version-alias resolves against
// the same index, for each Render signal, the reviewed lines included.
func TestNodeBuildInstallsTheReleaseNodeVersionAliasResolves(t *testing.T) {
	index := nodeIndexFixture()
	digests := map[string]string{}
	for _, release := range index {
		digests["node:"+release+"-bookworm"] = exactDigest
	}
	for _, request := range []string{"25.8.1", "v25", "25", "25.8", ">=20 <22", ">=24 <26", ">=20", "^22", "22", "~21.6.0",
		">=14 <16", ">=16 <18", "19", "^23.10", "21.0.0", "<20", "^20 || ^21"} {
		want := maxSatisfying(t, index, request)
		for name, signal := range map[string]struct{ env, files map[string]string }{
			"NODE_VERSION": {env: map[string]string{"NODE_VERSION": request}},
			".nvmrc":       {files: map[string]string{".nvmrc": request + "\n"}},
			"engines.node": {files: map[string]string{"package.json": `{"engines":{"node":"` + request + `"}}`}},
		} {
			t.Run(request+"/"+name, func(t *testing.T) {
				choices, _ := resolveExact(t, &fakeExact{digests: digests}, nativeNodeRuntime, signal.env, signal.files)
				got := choices[0].Line
				if line, reviewed := nativeToolchains[nativeNodeRuntime].find(got); reviewed {
					got = line.version
				}
				if got != want {
					t.Fatalf("installed %q (%+v), want node-version-alias's %s", got, choices[0], want)
				}
			})
		}
	}
}

// A release no reviewed line carries installs from its official image pinned
// by digest, on the common base and on the official image alike, and the log
// names it as Render's does.
func TestResolveNativeBuildInstallsAnyNodeRelease(t *testing.T) {
	common := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = common })
	reviewed, _ := nativeToolchains[nativeNodeRuntime].find("24")
	for _, base := range []string{common, ""} {
		nativeCommonBase = base
		for _, tc := range []struct {
			name       string
			env, files map[string]string
			tags       []string
			image, log string
		}{
			{".nvmrc", nil, map[string]string{".nvmrc": "25.8.1\n"}, []string{"25.8.1-bookworm"}, "node:25.8.1-bookworm",
				"==> Using Node.js version 25.8.1 via /opt/render/project/src/.nvmrc"},
			{"a range", map[string]string{"NODE_VERSION": ">=20 <22"}, nil, []string{"21.7.3-bookworm"}, "node:21.7.3-bookworm",
				"==> Requesting Node.js version >=20 <22\n==> Using Node.js version 21.7.3 via environment variable NODE_VERSION"},
			{"a listed range", nil, map[string]string{"package.json": `{"engines":{"node":"~25.8.0"}}`},
				[]string{"25.8.0-bookworm", "25.8.1-bookworm", "25.8.1-bookworm-slim", "25.9.0-bookworm"}, "node:25.8.1-bookworm",
				"==> Requesting Node.js version ~25.8.0\n==> Using Node.js version 25.8.1 via /opt/render/project/src/package.json"},
		} {
			t.Run(tc.name+"/common base "+boolName(base != ""), func(t *testing.T) {
				fake := &fakeExact{digests: map[string]string{}}
				for _, tag := range tc.tags {
					fake.digests["node:"+tag] = exactDigest
				}
				choices, dockerfile := resolveExact(t, fake, nativeNodeRuntime, tc.env, tc.files)
				image := tc.image + "@" + exactDigest
				if choices[0].Image != image || choices[0].Narration() != tc.log+nodeDocs {
					t.Fatalf("choice %+v narration %q; want %s and %q", choices[0], choices[0].Narration(), image, tc.log+nodeDocs)
				}
				assertExactNativePrimary(t, dockerfile, reviewed.image, image, []string{"NODE_VERSION=" + choices[0].Line})
				if !strings.Contains(dockerfile, nativePnpmInstall()) {
					t.Errorf("Node %s lacks the pinned pnpm:\n%s", choices[0].Line, dockerfile)
				}
			})
		}
	}
}

func boolName(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// Node 15, 17 and 19 have no bookworm images: the release comes from its
// older-variant image as w8/309's do, and Node 15 and 17 get the standalone
// pnpm, which pnpm 10 needs before Node 18.12.
func TestNodeReleaseOnAnOddLineWithoutBookwormImages(t *testing.T) {
	common := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = common })
	for _, base := range []string{common, ""} {
		nativeCommonBase = base
		for _, tc := range []struct {
			request, donor string
			legacyPnpm     bool
		}{
			{"19", "node:19.9.0-bullseye", false},
			{"17.9.1", "node:17.9.1-bullseye", true},
			{">=15 <16", "node:15.14.0-buster", true},
		} {
			t.Run(tc.request+"/common base "+boolName(base != ""), func(t *testing.T) {
				fake := &fakeExact{digests: map[string]string{tc.donor: exactDigest}}
				choices, dockerfile := resolveExact(t, fake, nativeNodeRuntime, map[string]string{"NODE_VERSION": tc.request}, nil)
				donor := tc.donor + "@" + exactDigest
				copyFrom := "COPY --link --from=" + donor + " /usr/local/bin/node /usr/local/bin/node"
				if nativeCommonBase != "" {
					copyFrom = "COPY --link --from=" + donor + " /usr/local/ /usr/local/"
				}
				release, _ := strings.CutPrefix(strings.Split(tc.donor, "-")[0], "node:")
				if choices[0].Line != release || !strings.Contains(dockerfile, copyFrom) || !strings.Contains(choices[0].Narration(), "does not exist") {
					t.Fatalf("choice %+v narration %q; want Node %s from %s\n%s", choices[0], choices[0].Narration(), release, tc.donor, dockerfile)
				}
				if got := strings.Contains(dockerfile, nativeLegacyPnpmInstall()); got != tc.legacyPnpm || got == strings.Contains(dockerfile, nativePnpmInstall()) {
					t.Errorf("legacy pnpm = %t, want %t:\n%s", got, tc.legacyPnpm, dockerfile)
				}
			})
		}
	}
}

// The Node companion of every other native runtime installs such a release
// the same way, on the common base and on the official image alike.
func TestNodeCompanionInstallsAnyNodeRelease(t *testing.T) {
	common := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = common })
	const image = "node:25.8.1-bookworm@" + exactDigest
	for _, base := range []string{common, ""} {
		nativeCommonBase = base
		for runtime := range nativeJSCompanionRuntimes {
			t.Run(runtime+"/common base "+boolName(base != ""), func(t *testing.T) {
				fake := &fakeExact{digests: map[string]string{"node:25.8.1-bookworm": exactDigest}}
				choices, dockerfile := resolveExact(t, fake, runtime, map[string]string{"NODE_VERSION": "25.8.1"}, nil)
				i := slices.IndexFunc(choices, func(c NativeChoice) bool { return c.Label == "Node" })
				if i < 0 || choices[i].Image != image || choices[i].Narration() != "==> Using Node.js version 25.8.1 via environment variable NODE_VERSION"+nodeDocs ||
					!strings.Contains(dockerfile, "COPY --link --from="+image+" /usr/local/bin/node /usr/local/bin/node") {
					t.Fatalf("choices %+v; want the companion Node 25.8.1 copied in\n%s", choices, dockerfile)
				}
			})
		}
	}
}

// A static site's Node, the primary of Render's static build environment,
// installs such a release the same way.
func TestStaticSiteInstallsAnyNodeRelease(t *testing.T) {
	o := nativeOptions()
	o.StaticSite, o.Runtime, o.StartCommand = true, "", ""
	const image = "node:21.7.3-bookworm@" + exactDigest
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, map[string]string{"package.json": `{"engines":{"node":">=20 <22"}}`})
	fake := &fakeExact{digests: map[string]string{"node:21.7.3-bookworm": exactDigest}}
	choices, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, nativeRuntime(o), bundle, dockerfile, src)
	if err != nil || choices[0].Image != image || !strings.Contains(choices[0].Narration(), "==> Using Node.js version 21.7.3 via /opt/render/project/src/package.json") {
		t.Fatalf("choices %+v, %v; want the static site's Node 21.7.3", choices, err)
	}
	out, _ := os.ReadFile(dockerfile)
	reviewed, _ := nativeToolchains[nativeNodeRuntime].find("24")
	assertExactNativePrimary(t, string(out), reviewed.image, image, []string{"NODE_VERSION=21.7.3"})
}

// What nodejs.org does not have fails at version selection by name, with the
// Dockerfile untouched, and an unreadable registry fails as a platform error:
// no reviewed line is the same Node.
func TestNodeReleaseWithoutAnImageFailsByName(t *testing.T) {
	o := nativeOptions()
	o.Runtime = nativeNodeRuntime
	supported := "set a published Node release or a supported Node line (14, 16, 18, 20, 22, 24, 26)"
	for _, tc := range []struct {
		name       string
		env, files map[string]string
		want       string
	}{
		{"exact", map[string]string{"NODE_VERSION": "99.99.99"}, nil,
			"NODE_VERSION=99.99.99 is not a Node release: no official image node:99.99.99-bookworm (or its bullseye or buster variant) exists; " + supported},
		{"exact past an odd line's final", nil, map[string]string{".nvmrc": "v25.99.0\n"},
			`.nvmrc "v25.99.0" is not a Node release: no official image node:25.99.0-bookworm (or its bullseye or buster variant) exists; ` + supported},
		{"range", nil, map[string]string{"package.json": `{"engines":{"node":"~21.6.5"}}`},
			`package.json engines.node "~21.6.5" matches no Node release: no official node:21.x image (bookworm, bullseye or buster) satisfies it; ` + supported},
		{"a version no tag can name", map[string]string{"NODE_VERSION": "25.8." + strings.Repeat("9", 200)}, nil,
			"NODE_VERSION=25.8." + strings.Repeat("9", 102) + "… is not a Node release; " + supported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle, dockerfile, src := writeResolveFixture(t, o, tc.env, tc.files)
			before, _ := os.ReadFile(dockerfile)
			fake := &fakeExact{digests: map[string]string{"node:21.6.2-bookworm": exactDigest, "node:21.7.3-bookworm": exactDigest}}
			_, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, nativeNodeRuntime, bundle, dockerfile, src)
			if _, tenant := errors.AsType[*UnsupportedVersionError](err); !tenant || err.Error() != tc.want {
				t.Fatalf("err = %v, want the tenant error %q", err, tc.want)
			}
			if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
				t.Error("a release that does not exist rewrote the Dockerfile")
			}
		})
	}
	for name, env := range map[string]map[string]string{
		"exact": {"NODE_VERSION": "25.8.1"},
		"range": {"NODE_VERSION": "~25.8.0"},
	} {
		t.Run("registry down/"+name, func(t *testing.T) {
			bundle, dockerfile, src := writeResolveFixture(t, o, env, nil)
			_, err := ResolveNativeBuildExact(t.Context(), &fakeExact{err: errors.New("registry down")}, time.Time{}, nativeNodeRuntime, bundle, dockerfile, src)
			if _, tenant := errors.AsType[*UnsupportedVersionError](err); err == nil || tenant || !strings.Contains(err.Error(), "registry down") || !strings.Contains(err.Error(), "Node 25") {
				t.Fatalf("err = %v, want a platform error naming the lookup", err)
			}
		})
	}
}
