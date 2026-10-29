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
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Exact Erlang/OTP (w8/197). Render installs the Erlang/OTP release a service
// pins with ERLANG_VERSION (28.3), not just its major. The official elixir
// images carry whatever OTP of the major they were built on, so an exact
// release builds on two other images, both pinned by digest for the build:
//
//   - the official erlang image of that release, which is the official elixir
//     image's own base (the same Debian buildpack-deps toolchain, OTP and
//     rebar3 under /usr/local), and
//   - Elixir's tree from hexpm's build of the Elixir release on that same OTP
//     release, so no Elixir code runs on an OTP older than it was compiled on.
//
// A minor (28.3) takes its newest patch both repositories publish. When none
// exists the build fails by name; it never builds on another OTP, and the log
// names the OTP the image installs.

// hexpmElixirTag is a hexpm Debian build: <elixir>-erlang-<otp>-debian-<os>-<date>.
var hexpmElixirTag = regexp.MustCompile(`^(\d+\.\d+\.\d+)-erlang-(\d+(?:\.\d+){1,3})-debian-[a-z]+-(\d{8}(?:\.\d+)?)$`)

// pinErlangVersion swaps an Elixir choice's image for the official erlang image
// of the requested OTP release, with Elixir from hexpm's build on that release.
func pinErlangVersion(ctx context.Context, exact ExactImageResolver, choice NativeChoice) (NativeChoice, error) {
	elixir := choice.Exact
	if elixir == "" {
		line, _ := nativeToolchains[nativeElixirRuntime].find(choice.Line)
		elixir = line.version
	}
	request := choice.erlang
	tags, err := exact.ListTags(ctx, hexpmElixirRepo, elixir+"-erlang-"+otpName(request))
	if err != nil {
		return NativeChoice{}, fmt.Errorf("look up Elixir %s on Erlang/OTP %s: %w", elixir, request, err)
	}
	// The newest hexpm build of each OTP release the request names.
	type build struct{ date, tag string }
	builds := map[string]build{}
	for _, tag := range tags {
		m := hexpmElixirTag.FindStringSubmatch(tag)
		if m == nil || m[1] != elixir || !otpMatches(m[2], request) {
			continue
		}
		// The newest date wins; the tag breaks a tie, so the choice is stable.
		if b, ok := builds[m[2]]; !ok || m[3] > b.date || m[3] == b.date && tag > b.tag {
			builds[m[2]] = build{m[3], tag}
		}
	}
	releases := make([]string, 0, len(builds))
	for otp := range builds {
		releases = append(releases, otp)
	}
	slices.SortFunc(releases, func(a, b string) int { return slices.Compare(otpParts(b), otpParts(a)) })
	for _, otp := range releases {
		erlangTag := otpTag(otp)
		erlangDigest, found, err := exact.ResolveTag(ctx, erlangRepo, erlangTag)
		if err != nil {
			return NativeChoice{}, fmt.Errorf("look up %s:%s: %w", erlangRepo, erlangTag, err)
		}
		if !found {
			continue
		}
		elixirTag := builds[otp].tag
		elixirDigest, found, err := exact.ResolveTag(ctx, hexpmElixirRepo, elixirTag)
		if err != nil {
			return NativeChoice{}, fmt.Errorf("look up %s:%s: %w", hexpmElixirRepo, elixirTag, err)
		}
		if !found {
			continue
		}
		choice.Image = erlangRepo + ":" + erlangTag + "@" + erlangDigest
		choice.elixirImage = hexpmElixirRepo + ":" + elixirTag + "@" + elixirDigest
		choice.Line, choice.otp = elixir, otp
		return choice, nil
	}
	return NativeChoice{}, &UnsupportedVersionError{msg: fmt.Sprintf(
		"ERLANG_VERSION=%[1]s is not available with Elixir %[2]s: no published image carries Elixir %[2]s on Erlang/OTP %[1]s; set ERLANG_VERSION to another OTP %[3]d release (or %[3]d for the line's own) or choose another ELIXIR_VERSION",
		request, elixir, otpParts(request)[0])}
}

// otpName is how OTP names a release, without trailing zero parts past the
// minor (28.3.0 → 28.3): the form hexpm tags carry, so every tag of the
// requested release and its patches begins with it.
func otpName(release string) string {
	parts := strings.Split(release, ".")
	for len(parts) > 2 && strings.Trim(parts[len(parts)-1], "0") == "" {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, ".")
}

// nativeElixirTreeLines installs Elixir on the erlang base exactly as the
// official elixir image does on its own: the tree in /usr/local/lib/elixir, its
// commands linked into /usr/local/bin, and the image's LANG and ELIXIR_VERSION.
func nativeElixirTreeLines(elixirImage, version string) string {
	return "COPY --link --from=" + elixirImage + " /usr/local/lib/elixir /usr/local/lib/elixir\n" +
		"RUN ln -s ../lib/elixir/bin/elixir ../lib/elixir/bin/elixirc ../lib/elixir/bin/iex ../lib/elixir/bin/mix /usr/local/bin/\n" +
		"ENV LANG=C.UTF-8 ELIXIR_VERSION=v" + version + "\n"
}

// otpParts is an OTP release's numbers, padded to four (28.3 → 28.3.0.0).
func otpParts(release string) []int {
	parts := []int{0, 0, 0, 0}
	for i, p := range strings.SplitN(release, ".", 4) {
		parts[i], _ = strconv.Atoi(p)
	}
	return parts
}

// otpMatches reports whether release is the requested one or one of its
// patches: 28.3 matches 28.3, 28.3.1 and 28.3.3; 28.3.0 matches 28.3 and its
// maintenance patches (28.3.0.x), not 28.3.1.
func otpMatches(release, request string) bool {
	n := strings.Count(request, ".") + 1
	return slices.Equal(otpParts(release)[:n], otpParts(request)[:n])
}

// otpTag is the official erlang tag that names exactly one OTP release: the
// four-part one (28.3.3.0). The shorter tags (28.3, 28.3.3) follow the newest
// release beneath them.
func otpTag(release string) string {
	p := otpParts(release)
	return fmt.Sprintf("%d.%d.%d.%d", p[0], p[1], p[2], p[3])
}
