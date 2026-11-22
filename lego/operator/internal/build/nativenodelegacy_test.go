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
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// w8/401: Render installs Node releases before 14. A Node service logged
// "==> Using Node version 12.18.3 via environment variable NODE_VERSION" and
// built (community.render.com thread 15142), and every request resolves with
// node-version-alias (render.com/docs/node-version). bex installs each such
// release from its official image, back to 0.10.45, the oldest whose image a
// build can pull, and the request resolves as it does from 14 on.
func TestNodeRequestBefore14ResolvesLikeNodeVersionAlias(t *testing.T) {
	for _, tc := range []struct {
		name                string
		env, files          map[string]string
		exact, release, rng string
	}{
		{name: "NODE_VERSION exact", env: map[string]string{"NODE_VERSION": "12.18.3"}, exact: "12.18.3"},
		{name: ".nvmrc exact", files: map[string]string{".nvmrc": "v10.24.1\n"}, exact: "10.24.1"},
		{name: "major", env: map[string]string{"NODE_VERSION": "12"}, exact: "12.22.12"},
		{name: "x range", files: map[string]string{"package.json": enginesNode("12.x")}, exact: "12.22.12"},
		{name: "caret", files: map[string]string{"package.json": enginesNode("^12.18.0")}, exact: "12.22.12"},
		{name: "below 14", files: map[string]string{"package.json": enginesNode("<14")}, exact: "13.14.0"},
		{name: "across old lines", env: map[string]string{"NODE_VERSION": ">=8 <11"}, exact: "10.24.1"},
		{name: "lts codename", env: map[string]string{"NODE_VERSION": "lts/erbium"}, exact: "12.22.12"},
		{name: "oldest lts codename", env: map[string]string{"NODE_VERSION": "lts/Argon"}, exact: "4.9.1"},
		{name: "0.x line", env: map[string]string{"NODE_VERSION": "0.12"}, exact: "0.12.18"},
		{name: "0.x exact", files: map[string]string{".node-version": "0.10.48\n"}, exact: "0.10.48"},
		// A final release outside the request leaves the line for the
		// resolve phase to list.
		{name: "older minor", files: map[string]string{".node-version": "12.18\n"}, release: "12", rng: "12.18"},
		{name: "0.x range a final misses", files: map[string]string{"package.json": enginesNode("~0.10.40 <0.10.46")}, release: "0.10", rng: "~0.10.40 <0.10.46"},
		// Releases nodejs.org has with no image still reach the registry,
		// which refuses them by name.
		{name: "a release no image carries", env: map[string]string{"NODE_VERSION": "0.8.6"}, exact: "0.8.6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveNativeVersion(nativeNodeRuntime, tc.env, files(tc.files))
			if err != nil {
				t.Fatal(err)
			}
			if got.Line != "24" || !got.unreviewed || got.Exact != tc.exact || got.releaseLine != tc.release || got.rangeRequest != tc.rng {
				t.Fatalf("got line %q exact %q release line %q range %q unreviewed %t; want 24 %q %q %q true",
					got.Line, got.Exact, got.releaseLine, got.rangeRequest, got.unreviewed, tc.exact, tc.release, tc.rng)
			}
		})
	}
	// What names no Node release stays refused at version selection.
	for _, request := range []string{"3", "<0.10", "lts/notacodename"} {
		_, err := ResolveNativeVersion(nativeNodeRuntime, map[string]string{"NODE_VERSION": request}, files(nil))
		if _, tenant := errors.AsType[*UnsupportedVersionError](err); !tenant || !strings.Contains(err.Error(), "NODE_VERSION="+request+" is not a supported Node version") {
			t.Errorf("NODE_VERSION=%s: err = %v, want the selection refusal", request, err)
		}
	}
}

// nodeBefore14Images are official images of releases before 14 in the variant
// Docker Hub publishes each in (read 2026-11-22): bullseye from 12.22.5,
// buster, stretch from 4.8.4 and wheezy from 0.10.45.
var nodeBefore14Images = []string{
	"0.10.45-wheezy", "0.10.48-wheezy", "0.12.17-wheezy", "0.12.18-wheezy", "4.8.3-wheezy", "4.9.1-stretch", "6.17.1-stretch",
	"8.17.0-buster", "10.15.3-stretch", "10.24.1-buster", "12.18.3-buster", "12.22.11-bullseye", "12.22.12-bullseye", "13.14.0-buster",
}

// The release a build installs is the one node-version-alias resolves against
// the same index, for each Render signal.
func TestNodeBuildBefore14InstallsTheReleaseNodeVersionAliasResolves(t *testing.T) {
	index := nodeIndexFixture()
	digests := map[string]string{}
	for _, release := range index {
		digests["node:"+release+"-bookworm"] = exactDigest
	}
	for _, tag := range nodeBefore14Images {
		digests["node:"+tag] = exactDigest
		release, _, _ := strings.Cut(tag, "-")
		index = append(index, release)
	}
	for _, request := range []string{"12.18.3", "12", "v12", "^12.18.0", "~12.22.0", "12.22", "<14", ">=8 <11", "10.15",
		"^0.12.0", "0.10", "~0.10.45", "4", "<5", "4.8", "6 || 8"} {
		want := maxSatisfying(t, index, request)
		for name, signal := range map[string]struct{ env, files map[string]string }{
			"NODE_VERSION": {env: map[string]string{"NODE_VERSION": request}},
			".nvmrc":       {files: map[string]string{".nvmrc": request + "\n"}},
			"engines.node": {files: map[string]string{"package.json": enginesNode(request)}},
		} {
			t.Run(request+"/"+name, func(t *testing.T) {
				choices, _ := resolveExact(t, &fakeExact{digests: digests}, nativeNodeRuntime, signal.env, signal.files)
				if got := choices[0].Line; got != want {
					t.Fatalf("installed %q (%+v), want node-version-alias's %s", got, choices[0], want)
				}
			})
		}
	}
}

// Each release installs from its own older-variant image, as w8/309's do: on
// the common base its tree replaces the default's behind a guard that reads
// that Debian's os-release as it is, and off it its node, npm and headers are
// copied onto the default line's image. pnpm 10 needs Node 18.12, so each
// gets the standalone pnpm.
func TestResolveNativeBuildInstallsNodeBefore14(t *testing.T) {
	common := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = common })
	reviewed, _ := nativeToolchains[nativeNodeRuntime].find("24")
	for _, base := range []string{common, ""} {
		nativeCommonBase = base
		for _, tc := range []struct {
			name, donor, log, osRelease string
			env, files                  map[string]string
		}{
			{"NODE_VERSION", "node:12.18.3-buster", "==> Using Node.js version 12.18.3 via environment variable NODE_VERSION", "/usr/lib/os-release",
				map[string]string{"NODE_VERSION": "12.18.3"}, nil},
			{"engines range", "node:12.22.12-bullseye", "==> Requesting Node.js version 12.x\n==> Using Node.js version 12.22.12 via /opt/render/project/src/package.json", "/usr/lib/os-release",
				nil, map[string]string{"package.json": enginesNode("12.x")}},
			{"stretch", "node:10.15.3-stretch", "==> Using Node.js version 10.15.3 via /opt/render/project/src/.nvmrc", "/usr/lib/os-release",
				nil, map[string]string{".nvmrc": "10.15.3\n"}},
			{"wheezy", "node:0.10.48-wheezy", "==> Using Node.js version 0.10.48 via environment variable NODE_VERSION", "/etc/os-release",
				map[string]string{"NODE_VERSION": "0.10"}, nil},
		} {
			t.Run(tc.name+"/common base "+boolName(base != ""), func(t *testing.T) {
				fake := &fakeExact{digests: map[string]string{tc.donor: exactDigest}}
				choices, dockerfile := resolveExact(t, fake, nativeNodeRuntime, tc.env, tc.files)
				donor := tc.donor + "@" + exactDigest
				release := strings.TrimPrefix(strings.Split(tc.donor, "-")[0], "node:")
				narration := choices[0].Narration()
				if choices[0].Line != release || !strings.HasPrefix(narration, tc.log+nodeDocs+"\n") || !strings.Contains(narration, "node:"+release+"-bookworm does not exist") {
					t.Fatalf("choice %+v narration %q; want Node %s from %s", choices[0], narration, release, tc.donor)
				}
				if !strings.Contains(dockerfile, nativeLegacyPnpmInstall()) || strings.Contains(dockerfile, nativePnpmInstall()) {
					t.Errorf("Node %s lacks the standalone pnpm:\n%s", release, dockerfile)
				}
				if nativeCommonBase == "" {
					if !strings.Contains(dockerfile, nativeNodeDonorLines(donor, release)) {
						t.Errorf("Node %s was not copied onto the default line's image:\n%s", release, dockerfile)
					}
					return
				}
				suite := strings.SplitN(tc.donor, "-", 2)[1]
				guard := "COPY --link --from=" + donor + " " + tc.osRelease + " /opt/bex-primary-os-release\n"
				if !strings.Contains(dockerfile, guard) || !strings.Contains(dockerfile, `[ "$primary_os" = `+suite+` ] && [ "$VERSION_CODENAME" = bookworm ]`) {
					t.Errorf("no %s guard for %s:\n%s", suite, release, dockerfile)
				}
				assertExactNativePrimary(t, dockerfile, reviewed.image, donor, []string{"NODE_VERSION=" + release})
			})
		}
	}
}

// The common base's guard reads each old Debian's os-release as it is: a
// stretch point release with VERSION_CODENAME, an earlier one with only
// VERSION="9 (stretch)", and wheezy's; it still refuses a tree from another
// suite, or one whose file names none.
func TestNodeCommonGuardReadsOldDebianOSRelease(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	for name, tc := range map[string]struct {
		source, primary string
		ok              bool
	}{
		"late stretch":     {"node:10.24.1-stretch", "VERSION=\"9 (stretch)\"\nVERSION_CODENAME=stretch\n", true},
		"early stretch":    {"node:6.17.1-stretch", "PRETTY_NAME=\"Debian GNU/Linux 9 (stretch)\"\nVERSION_ID=\"9\"\nVERSION=\"9 (stretch)\"\nID=debian\n", true},
		"wheezy":           {"node:0.10.48-wheezy", "VERSION_ID=\"7\"\nVERSION=\"7 (wheezy)\"\nID=debian\n", true},
		"another suite":    {"node:6.17.1-stretch", "VERSION=\"8 (jessie)\"\n", false},
		"no suite named":   {"node:0.10.48-wheezy", "VERSION_ID=\"7\"\nVERSION=\"7\"\n", false},
		"buster unchanged": {"node:12.18.3-buster", "VERSION=\"10 (buster)\"\nVERSION_CODENAME=buster\n", true},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			primary, base := filepath.Join(dir, "primary"), filepath.Join(dir, "base")
			must(t, os.WriteFile(primary, []byte(tc.primary), 0o644))
			must(t, os.WriteFile(base, []byte("VERSION=\"12 (bookworm)\"\nVERSION_CODENAME=bookworm\n"), 0o644))
			guard := nativeCommonSourceGuard(tc.source+"@"+exactDigest, true, false)
			_, run, found := strings.Cut(guard, "\nRUN ")
			if !found {
				t.Fatalf("no RUN in %q", guard)
			}
			run = strings.ReplaceAll(strings.ReplaceAll(run, "/opt/bex-primary-os-release", primary), ". /etc/os-release", ". "+base)
			out, err := exec.Command("sh", "-c", run).CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("guard ok = %t (%v %s), want %t:\n%s", err == nil, err, out, tc.ok, run)
			}
		})
	}
}

// Another runtime's Node installs such a release too, where w8/400 fell back
// to the default: a stray docs/package.json asking for 12.x builds a Python
// service on Node 12, as on Render.
func TestNodeCompanionInstallsANodeReleaseBefore14(t *testing.T) {
	common := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = common })
	for _, base := range []string{common, ""} {
		nativeCommonBase = base
		for _, tc := range []struct {
			name, donor, log string
			env, files       map[string]string
		}{
			{"nested package.json", "node:12.22.12-bullseye", "==> Requesting Node.js version 12.x\n==> Using Node.js version 12.22.12 via /opt/render/project/src/docs/package.json",
				nil, map[string]string{"app.py": "", "docs/package.json": enginesNode("12.x")}},
			{"NODE_VERSION", "node:12.18.3-buster", "==> Using Node.js version 12.18.3 via environment variable NODE_VERSION",
				map[string]string{"NODE_VERSION": "12.18.3"}, nil},
		} {
			for primary := range nativeJSCompanionRuntimes {
				t.Run(tc.name+"/"+primary+"/common base "+boolName(base != ""), func(t *testing.T) {
					fake := &fakeExact{digests: map[string]string{tc.donor: exactDigest}}
					choices, dockerfile := resolveExact(t, fake, primary, tc.env, tc.files)
					i := slices.IndexFunc(choices, func(c NativeChoice) bool { return c.Label == "Node" })
					donor := tc.donor + "@" + exactDigest
					if i < 0 || choices[i].Image != donor && choices[i].nodeDonor != donor || !strings.HasPrefix(choices[i].Narration(), tc.log+nodeDocs) ||
						strings.Contains(choices[i].Narration(), "using the default Node.js version instead") {
						t.Fatalf("choices %+v; want the companion Node from %s", choices, tc.donor)
					}
					if !strings.Contains(dockerfile, "COPY --link --from="+donor+" /usr/local/bin/node /usr/local/bin/node\n") || !strings.Contains(dockerfile, nativeLegacyPnpmInstall()) {
						t.Errorf("the companion Node or its standalone pnpm is missing:\n%s", dockerfile)
					}
				})
			}
		}
	}
}

// A static site's Node installs such a release the same way, from
// NODE_VERSION or from package.json.
func TestStaticSiteInstallsANodeReleaseBefore14(t *testing.T) {
	for _, tc := range []struct {
		name, donor, log string
		env, files       map[string]string
	}{
		{"NODE_VERSION", "node:12.18.3-buster", "==> Using Node.js version 12.18.3 via environment variable NODE_VERSION",
			map[string]string{"NODE_VERSION": "12.18.3"}, nil},
		{"engines range", "node:12.22.12-bullseye", "==> Using Node.js version 12.22.12 via /opt/render/project/src/package.json",
			nil, map[string]string{"package.json": enginesNode("^12.18.0")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.StaticSite, o.Runtime, o.StartCommand = true, "", ""
			bundle, dockerfile, src := writeResolveFixture(t, o, tc.env, tc.files)
			fake := &fakeExact{digests: map[string]string{tc.donor: exactDigest}}
			choices, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, nativeRuntime(o), bundle, dockerfile, src)
			release := strings.TrimPrefix(strings.Split(tc.donor, "-")[0], "node:")
			if err != nil || choices[0].Line != release || !strings.Contains(choices[0].Narration(), tc.log) {
				t.Fatalf("choices %+v, %v; want the static site's Node %s from %s", choices, err, release, tc.donor)
			}
			out, _ := os.ReadFile(dockerfile)
			reviewed, _ := nativeToolchains[nativeNodeRuntime].find("24")
			assertExactNativePrimary(t, string(out), reviewed.image, tc.donor+"@"+exactDigest, []string{"NODE_VERSION=" + release})
		})
	}
}

// A release before 14 no build can install fails at version selection by
// name, with the Dockerfile untouched: one whose only image is in Docker's
// retired v1 format (6.0.0, and the newest 5.10 release a range reaches), and
// one no image carries at all (nodejs.org has releases before 0.10.45 that no
// image does).
func TestNodeReleaseBefore14ThatCannotBePulledFailsByName(t *testing.T) {
	o := nativeOptions()
	o.Runtime = nativeNodeRuntime
	supported := "a supported Node line (14, 16, 18, 20, 22, 24, 26)"
	v1 := "is in Docker's retired v1 image format, which the builder cannot pull; set another Node release or " + supported
	for _, tc := range []struct {
		name       string
		env, files map[string]string
		want       string
	}{
		{"v1 image", map[string]string{"NODE_VERSION": "6.0.0"}, nil,
			"NODE_VERSION=6.0.0 cannot be installed: Node 6.0.0's only official image, node:6.0.0-wheezy, " + v1},
		{"range reaching a v1 image", nil, map[string]string{"package.json": enginesNode("~5.10.0")},
			`package.json engines.node "~5.10.0" cannot be installed: Node 5.10.1's only official image, node:5.10.1-wheezy, ` + v1},
		{"no image", map[string]string{"NODE_VERSION": "0.8.6"}, nil,
			"NODE_VERSION=0.8.6 cannot be installed: no official image node:0.8.6-bookworm (or its bullseye, buster, stretch or wheezy variant) exists; set a published Node release or " + supported},
		{"range with no image", nil, map[string]string{".nvmrc": "~0.10.20 <0.10.45\n"},
			`.nvmrc "~0.10.20 <0.10.45" matches no Node release: no official node:0.10.x image (bookworm, bullseye, buster, stretch or wheezy) satisfies it; set a published Node release or ` + supported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle, dockerfile, src := writeResolveFixture(t, o, tc.env, tc.files)
			before, _ := os.ReadFile(dockerfile)
			fake := &fakeExact{
				digests: map[string]string{"node:5.10.0-wheezy": exactDigest, "node:5.10.1-wheezy": exactDigest, "node:0.10.48-wheezy": exactDigest},
				v1:      map[string]bool{"node:6.0.0-wheezy": true, "node:5.10.1-wheezy": true},
			}
			_, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, nativeNodeRuntime, bundle, dockerfile, src)
			if _, tenant := errors.AsType[*UnsupportedVersionError](err); !tenant || err.Error() != tc.want {
				t.Fatalf("err = %v, want the tenant error %q", err, tc.want)
			}
			if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
				t.Error("a release no build can install rewrote the Dockerfile")
			}
		})
	}
}

// pnpm 10 runs on Node 18.12 and later; every earlier release or line gets the
// standalone, which carries its own Node.
func TestNativeLegacyPnpmCoversEveryReleaseBefore1812(t *testing.T) {
	for version, want := range map[string]bool{
		"0.10.48": true, "4.9.1": true, "12": true, "12.18.3": true, "13.14.0": true, "14": true, "15.14.0": true,
		"17.9.1": true, "18.11.0": true, "18": false, "18.12.0": false, "22": false, "26.13.1": false, "": false, "canary": false,
	} {
		if got := nativeLegacyPnpmRelease(version); got != want {
			t.Errorf("nativeLegacyPnpmRelease(%q) = %t, want %t", version, got, want)
		}
	}
}

// Docker Hub answers a tag whose only manifest is in the retired v1 format
// with that manifest whatever Accept asks for; ResolveTag reports it so the
// build refuses it by name rather than handing BuildKit a pull it cannot do.
func TestDockerHubResolverReportsV1Manifests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"token":"tok"}`)) })
	mux.HandleFunc("/v2/library/node/manifests/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", exactDigest)
		switch {
		case strings.HasSuffix(r.URL.Path, "/6.0.0-wheezy"):
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v1+prettyjws")
		case strings.HasSuffix(r.URL.Path, "/0.10.48-wheezy"):
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		default:
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.list.v2+json")
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d := &DockerHubResolver{Client: srv.Client(), AuthURL: srv.URL + "/token", RegistryURL: srv.URL}
	if _, ok, err := d.ResolveTag(t.Context(), "node", "6.0.0-wheezy"); ok || !errors.Is(err, errV1Manifest) {
		t.Errorf("v1 manifest: ok=%t err=%v, want errV1Manifest", ok, err)
	}
	for _, tag := range []string{"0.10.48-wheezy", "12.18.3-buster"} {
		if digest, ok, err := d.ResolveTag(t.Context(), "node", tag); !ok || err != nil || digest != exactDigest {
			t.Errorf("%s: %q %t %v, want its digest", tag, digest, ok, err)
		}
	}
}

// TestNativeImageRunsAnOptedInNodeBefore14 is w8/401's execution gate: the
// resolved Dockerfile for a release before 14, from a buster, a stretch with
// VERSION_CODENAME and an early stretch without it, builds with the real
// registry; the authored build runs its node, npm, Yarn and the standalone
// pnpm and installs a package, and the started image serves on the same
// release. The common base is amd64 only, so another host builds the
// official-image path. Opt-in: needs Docker and the network.
func TestNativeImageRunsAnOptedInNodeBefore14(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	common := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = common })
	if runtime.GOARCH != "amd64" {
		nativeCommonBase = ""
	}
	for _, release := range []string{"12.18.3", "10.15.3", "6.17.1"} {
		t.Run(release, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = nativeNodeRuntime
			o.BuildCommand = `echo '{"name":"qa","version":"1.0.0"}' > package.json && npm install --no-audit ms@2.1.3 && ` +
				`echo "built node=$(node -v) npm=$(npm -v) yarn=$(yarn -v 2>&1 | tail -1) pnpm=$(pnpm -v) ms=$(node -p 'require("ms")(60000)')" > built.txt`
			o.StartCommand = `cat built.txt; node -e 'console.log("runtime " + process.version)'`
			bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"NODE_VERSION": release}, nil)
			if _, err := resolveNativeBuild(nativeNodeRuntime, bundle, dockerfile, src); err != nil {
				t.Fatal(err)
			}
			must(t, os.Rename(dockerfile, filepath.Join(src, "Dockerfile")))
			tag := "bex-native-node-it-" + release + ":" + strconv.Itoa(os.Getpid())
			t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", "-f", tag).Run() })
			build := exec.Command("docker", "build", "-q", "-t", tag, "--secret", "id=render-env,src="+bundle, src)
			build.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, out)
			}
			out, err := exec.Command("docker", "run", "--rm", tag).CombinedOutput()
			want := []string{"built node=v" + release + " npm=", " ms=1m", "runtime v" + release}
			for _, w := range want {
				if err != nil || !strings.Contains(string(out), w) {
					t.Fatalf("run = %v\n%s; want %q", err, out, w)
				}
			}
			t.Logf("%s", out)
		})
	}
}
