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
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
)

// withoutCommonBase clears the common-base pin for one test: the recipe then
// keeps each selection's official image, the route the official-image
// fallbacks and the qualification controls exercise. Production carries the
// common base.
func withoutCommonBase(t *testing.T) {
	t.Helper()
	base, artifacts := nativeCommonBase, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeRuntimeArtifacts = base, artifacts })
	nativeCommonBase, nativeRuntimeArtifacts = "", map[string]string{}
}

// expectedNativeBase is the final stage's FROM for a selection of the reviewed
// line image: the common image when it is pinned, else that official image.
func expectedNativeBase(reviewed string) string {
	if nativeCommonBase != "" {
		// Every selection's final stage starts from the common image; the
		// selected runtime is copied onto it.
		return nativeCommonBase
	}
	return reviewed
}

func assertExactNativePrimary(t *testing.T, dockerfile, reviewed, exact string, env []string) {
	t.Helper()
	base := exact
	if nativeCommonBase != "" {
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

// The common base must be pinned, and a primary's image environment is copied
// only when every entry is a plain NAME=value line.
func TestCommonBaseRefusesUnreviewedInputs(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	line, _ := nativeToolchains["node"].find("24")
	before := nativeDockerfile(nativeOptions())
	choice := NativeChoice{Image: line.image, imageEnv: []string{"NODE_VERSION=" + line.version}}
	nativeCommonBase = "ghcr.io/bex-co/bex-cnb-run:latest"
	if _, err := applyNativeCommonBase(before, line.image, choice); err == nil {
		t.Fatal("accepted a mutable common base")
	}
	nativeCommonBase = commonFixtureImage
	choice.Image = "node:24.10.0-bookworm@" + exactDigest
	before, err := rewriteNativeBase(before, line.image, choice.Image)
	must(t, err)
	choice.imageEnv = []string{"NODE_VERSION=24.10.0", "PATH=/usr/local/bin:/usr/bin"}
	if _, err := applyNativeCommonBase(before, line.image, choice); err != nil {
		t.Fatalf("refused a valid exact primary environment: %v", err)
	}
	for _, env := range [][]string{nil, {"BAD KEY=value"}, {"PATH=value\nRUN injected"}} {
		choice.imageEnv = env
		if _, err := applyNativeCommonBase(before, line.image, choice); err == nil {
			t.Fatalf("accepted invalid exact primary environment %v", env)
		}
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

// A pre-existing platform backport stays the fallback on the official-image
// route, and the exact lookup keeps naming the reviewed official input.
func TestPlatformBackportKeepsOfficialExactLookup(t *testing.T) {
	withoutCommonBase(t)
	const platform = "ghcr.io/bex-co/bex-native:python-backport@" + otherDigest
	originalToolchain := nativeToolchains[nativePythonRuntime]
	t.Cleanup(func() { nativeToolchains[nativePythonRuntime] = originalToolchain })
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
	for _, found := range []bool{false, true} {
		t.Run(fmt.Sprintf("exact-found-%v", found), func(t *testing.T) {
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
			if !strings.Contains(after, "\nFROM "+base+"\n") || choice.upstream != upstream {
				t.Fatalf("resolver lost the upstream key or existing derivative: %+v\n%s", choice, after)
			}
			if len(fake.asked) == 0 || fake.asked[0] != "python:3.14.8-bookworm" {
				t.Fatalf("exact lookup escaped official input: %v", fake.asked)
			}
			if strings.Contains(after, "RUN --network=none /usr/bin/bex-native-python-tools") || (!found && (choice.Image != platform || strings.Contains(after, "rm -rf /usr/local"))) {
				t.Fatalf("exact selection or verified fallback changed: %+v\n%s", choice, after)
			}
		})
	}
}

// Common adoption is atomic for the bounded reviewed Node catalog. Neighboring
// exact requests retain their official images and are outside that verdict.
func nativeCommonAdoptionDataError(common string, artifacts map[string]string) error {
	if common == "" {
		if len(artifacts) != 0 {
			return fmt.Errorf("runtime artifact catalog cannot precede common adoption")
		}
		return nil
	}
	if err := validateNativeBase(common); err != nil {
		return err
	}
	ref, _, _ := strings.Cut(common, "@")
	packageName, _, _ := strings.Cut(ref, ":")
	if packageName != "ghcr.io/bex-co/bex-cnb-run" {
		return fmt.Errorf("common foundation is outside the reviewed public package")
	}
	reviewed, err := commonNodeQualificationArtifacts("1")
	if err != nil {
		return err
	}
	for _, requested := range []string{"14", "16", "18", "20"} {
		line, found := nativeToolchains[nativeNodeRuntime].find(requested)
		if !found || reviewed[line.image] == "" {
			return fmt.Errorf("reviewed Node line %s lacks an approved runtime artifact", requested)
		}
	}
	const exact14212 = "node:14.21.2-bullseye@sha256:cc748803fe49871926c433ae6aafbfd59b765eed6005dc0843b916eb74f67b12"
	const donor14212 = "ghcr.io/bex-co/bex-cnb-run@sha256:332869e644cdd45bf1f9e7c42ed3df91fedb95c4d0cee6bf2abf771c9eb760c7"
	if len(reviewed) != 7 || reviewed[exact14212] != donor14212 || len(artifacts) != len(reviewed) {
		return fmt.Errorf("common adoption requires all four reviewed lines, exact14.21.2 and Render's dated Node 24.14.1 and 24.21.0")
	}
	for source, donor := range reviewed {
		if artifacts[source] != donor {
			return fmt.Errorf("common adoption lacks the exact approved runtime artifact for %s", source)
		}
	}
	return nil
}

func TestNativeCommonAdoptionDataIsAtomic(t *testing.T) {
	must(t, nativeCommonAdoptionDataError(nativeCommonBase, nativeRuntimeArtifacts))
}

func TestNativeCommonAdoptionDataRejectsPartialCatalog(t *testing.T) {
	reviewed, err := commonNodeQualificationArtifacts("1")
	must(t, err)
	for _, tc := range []struct {
		name, common string
		artifacts    map[string]string
		valid        bool
	}{
		{"not-adopted", "", nil, true},
		{"complete", commonFixtureImage, reviewed, true},
		{"catalog-before-common", "", reviewed, false},
		{"common-without-catalog", commonFixtureImage, nil, false},
		{"mutable-common", "ghcr.io/bex-co/bex-cnb-run:latest", reviewed, false},
		{"other-package", "example.invalid/common@" + otherDigest, reviewed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := nativeCommonAdoptionDataError(tc.common, tc.artifacts); (err == nil) != tc.valid {
				t.Fatalf("atomic common/catalog validation: %v", err)
			}
		})
	}
	for source := range reviewed {
		t.Run("missing-"+source, func(t *testing.T) {
			changed := maps.Clone(reviewed)
			delete(changed, source)
			if nativeCommonAdoptionDataError(commonFixtureImage, changed) == nil {
				t.Fatal("common adoption silently omitted a covered catalog identity")
			}
		})
		t.Run("wrong-donor-"+source, func(t *testing.T) {
			changed := maps.Clone(reviewed)
			changed[source] = "ghcr.io/bex-co/bex-cnb-run@" + otherDigest
			if nativeCommonAdoptionDataError(commonFixtureImage, changed) == nil {
				t.Fatal("common adoption substituted an unapproved runtime artifact")
			}
		})
	}
	changed := maps.Clone(reviewed)
	changed["node:14.21.1-bullseye@"+exactDigest] = "ghcr.io/bex-co/bex-cnb-run@" + otherDigest
	if nativeCommonAdoptionDataError(commonFixtureImage, changed) == nil {
		t.Fatal("common adoption added an unreviewed neighboring patch")
	}
	original := nativeToolchains[nativeNodeRuntime]
	t.Cleanup(func() { nativeToolchains[nativeNodeRuntime] = original })
	updated := original
	updated.lines = slices.Clone(original.lines)
	for i := range updated.lines {
		if updated.lines[i].line == "14" {
			updated.lines[i].image = "node:14-bullseye@" + exactDigest
		}
	}
	nativeToolchains[nativeNodeRuntime] = updated
	if nativeCommonAdoptionDataError(commonFixtureImage, reviewed) == nil {
		t.Fatal("updated production catalog silently reused stale artifact source identities")
	}
}

// nodeFirstBrotliFix is each Node line's first release whose bundled decoder
// carries Node's own CVE-2026-111497 fix (w1/135; nodejs/node@49dff6f57aa7
// on the 24 line, read from each tag's src/node_zlib.cc on 2026-11-26).
var nodeFirstBrotliFix = map[string]string{"22": "22.23.4", "24": "24.22.1", "26": "26.12.1"}

type nodeDonor struct{ source, tree string }

// nodeDonorsByVersion keys nativeRuntimeArtifacts' Node entries by the
// version in their official tag (a line image's key is its line).
func nodeDonorsByVersion() map[string]nodeDonor {
	donors := map[string]nodeDonor{}
	for source, tree := range nativeRuntimeArtifacts {
		ref, _, _ := strings.Cut(source, "@")
		if tag, ok := strings.CutPrefix(ref, "node:"); ok {
			version, _, _ := strings.Cut(tag, "-")
			donors[version] = nodeDonor{source, tree}
		}
	}
	return donors
}

// TestDatedNodeDefaultsHaveAFixedDecoder: every patch Render's dated
// node-version rows give a no-signal Node service, or another runtime's
// companion Node, carries Node's own Brotli fix or has a reviewed backport
// donor (w3/m672). A new Render row below its line's first fixed release
// fails here instead of reaching production unrepaired: publish its donor
// with deploy/native-base/node-brotli/patch.sh first.
func TestDatedNodeDefaultsHaveAFixedDecoder(t *testing.T) {
	donors := nodeDonorsByVersion()
	for _, step := range exactDefaults[nativeNodeRuntime] {
		line, _, _ := strings.Cut(step.Version, ".")
		fixed, recorded := nodeFirstBrotliFix[line]
		if !recorded {
			t.Errorf("Node %s, Render's default from %s: nodeFirstBrotliFix records no first Brotli-fixed release for the %s line", step.Version, step.From.Format(time.DateOnly), line)
			continue
		}
		if !semver.MustParse(step.Version).LessThan(semver.MustParse(fixed)) {
			continue
		}
		if _, ok := donors[step.Version]; !ok {
			t.Errorf("Node %s, Render's default from %s, predates the %s line's first Brotli-fixed release %s and has no reviewed backport donor in nativeRuntimeArtifacts", step.Version, step.From.Format(time.DateOnly), line, fixed)
		}
	}
}

// w3/m672: a no-signal Node service, and another runtime's companion Node,
// created in Render's 24.14.1 or 24.21.0 band copies that release's published
// Brotli donor, keeps its source record and probes the decoder at build time.
// One created from 2026-10-21 gets 24.22.1, which carries Node's own fix,
// from its official image with neither.
func TestDatedNode24DefaultsCopyTheirBrotliDonors(t *testing.T) {
	donors := nodeDonorsByVersion()
	fake := &fakeExact{digests: map[string]string{"node:24.22.1-bookworm": exactDigest}}
	for _, version := range []string{"24.14.1", "24.21.0"} {
		donor, ok := donors[version]
		if !ok {
			t.Fatalf("Render's dated Node %s has no donor in nativeRuntimeArtifacts", version)
		}
		ref, digest, _ := strings.Cut(donor.source, "@")
		fake.digests[ref] = digest
	}
	const probe = "RUN --network=none /usr/local/bin/node /usr/share/bex-security/node-check.js\n"
	for _, tc := range []struct {
		name, runtime, version string
		created                time.Time
	}{
		{"node created 2026-05-01", nativeNodeRuntime, "24.14.1", on2026(time.May, 1)},
		{"node created 2026-10-01", nativeNodeRuntime, "24.21.0", on2026(time.October, 1)},
		{"node created 2026-11-01", nativeNodeRuntime, "24.22.1", on2026(time.November, 1)},
		{"python companion created 2026-05-01", nativePythonRuntime, "24.14.1", on2026(time.May, 1)},
		{"python companion created 2026-10-01", nativePythonRuntime, "24.21.0", on2026(time.October, 1)},
		{"python companion created 2026-11-01", nativePythonRuntime, "24.22.1", on2026(time.November, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			choices, dockerfile := resolveExactAt(t, fake, tc.created, tc.runtime, nil, nil)
			i := slices.IndexFunc(choices, func(c NativeChoice) bool { return c.Label == "Node" })
			if i < 0 {
				t.Fatalf("no Node selection in %+v", choices)
			}
			node := choices[i]
			if version, _ := commonImageVariable(node.imageEnv, "NODE_VERSION"); version != tc.version {
				t.Fatalf("selected Node %q (%s), want Render's dated %s", version, node.Image, tc.version)
			}
			donor, repaired := donors[tc.version]
			if !repaired {
				if node.primaryImage != "" || strings.Contains(dockerfile, "node-check.js") || strings.Contains(dockerfile, "/usr/share/bex-node-brotli") {
					t.Fatalf("Node %s carries Node's own fix, yet the build copies a donor or claims a backport:\n%s", tc.version, dockerfile)
				}
				return
			}
			provenance := "COPY --link --from=" + donor.tree + " /usr/share/bex-node-brotli /usr/share/bex-node-brotli\n"
			if node.Image != donor.source || node.primaryImage != donor.tree ||
				!strings.Contains(dockerfile, "COPY --link --from="+donor.tree+" /usr/local/") ||
				!strings.Contains(dockerfile, provenance) || !strings.Contains(dockerfile, probe) {
				t.Fatalf("Node %s did not copy its donor %s with its source record and build-time probe:\n%s", tc.version, donor.tree, dockerfile)
			}
			if strings.Contains(dockerfile, "build-node") || strings.Contains(dockerfile, "--from="+donor.source+" /usr/local/") {
				t.Fatalf("Node %s compiled in the tenant build or copied the unrepaired official tree:\n%s", tc.version, dockerfile)
			}
		})
	}
}
