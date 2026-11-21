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
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/types/toolchains"
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
// names the OTP the image installs. An Erlang/OTP major other than the line's
// (Elixir 1.18.4 on 27), and an Elixir no line carries (1.16.3, on the newest
// major it supports unless ERLANG_VERSION names one), build the same way: a
// major takes its newest release both repositories publish (w8/373).

// hexpmElixirTag is a hexpm Debian build: <elixir>-erlang-<otp>-debian-<os>-<date>.
var hexpmElixirTag = regexp.MustCompile(`^(\d+\.\d+\.\d+)-erlang-(\d+(?:\.\d+){1,3})-debian-[a-z]+-(\d{8}(?:\.\d+)?)$`)

// maxOTPCandidates bounds the official releases a partial ERLANG_VERSION
// probes, newest first, for hexpm's build of the Elixir. hexpm trails the
// official images by a few maintenance patches: on 2026-11-21 Elixir 1.18.4's
// newest OTP 27 build was 27.3.4.16, three releases behind erlang:27.3.4.19.
const maxOTPCandidates = 12

// otpFourPart is an official erlang tag that names exactly one release.
var otpFourPart = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)

// pinErlangVersion swaps an Elixir choice's image for the official erlang image
// of the requested OTP release, with Elixir from hexpm's build on that release.
// The candidates are the official releases the request names, newest first
// (otpReleases); each is probed for hexpm's Debian builds of that one release.
// hexpm's own listing is no candidate source: an old Elixir and OTP minor
// (1.16.3 on 26.2) has more builds than a listing reads (w8/373).
func pinErlangVersion(ctx context.Context, exact ExactImageResolver, choice NativeChoice) (NativeChoice, error) {
	elixir := choice.Exact
	if elixir == "" {
		line, _ := nativeToolchains[nativeElixirRuntime].find(choice.Line)
		elixir = line.version
	}
	request := choice.erlang
	releases, err := otpReleases(ctx, exact, request)
	if err != nil {
		return NativeChoice{}, fmt.Errorf("look up Elixir %s on Erlang/OTP %s: %w", elixir, request, err)
	}
	for _, release := range releases {
		otp := otpName(release)
		tags, err := exact.ListTags(ctx, hexpmElixirRepo, elixir+"-erlang-"+otp+"-debian-")
		if err != nil {
			return NativeChoice{}, fmt.Errorf("look up Elixir %s on Erlang/OTP %s: %w", elixir, request, err)
		}
		// The newest build wins; the tag breaks a tie, so the choice is stable.
		elixirTag, date := "", ""
		for _, tag := range tags {
			m := hexpmElixirTag.FindStringSubmatch(tag)
			if m != nil && m[1] == elixir && m[2] == otp && (m[3] > date || m[3] == date && tag > elixirTag) {
				elixirTag, date = tag, m[3]
			}
		}
		if elixirTag == "" {
			continue
		}
		erlangTag := otpTag(release)
		erlangDigest, found, err := exact.ResolveTag(ctx, erlangRepo, erlangTag)
		if err != nil {
			return NativeChoice{}, fmt.Errorf("look up %s:%s: %w", erlangRepo, erlangTag, err)
		}
		if !found {
			continue
		}
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
	major := otpParts(request)[0]
	if !strings.Contains(choice.Source, "ERLANG_VERSION=") {
		// No ERLANG_VERSION: the OTP is the newest major an Elixir no line
		// carries supports (w8/373).
		return NativeChoice{}, &UnsupportedVersionError{Msg: fmt.Sprintf(
			"%s is not available: no published image carries Elixir %s on Erlang/OTP %d, the newest it supports; set ELIXIR_VERSION to a published Elixir release, or ERLANG_VERSION to another compatible release",
			toolchains.BoundedRequest(choice.Source), elixir, major)}
	}
	hint := ""
	if line, _ := nativeToolchains[nativeElixirRuntime].find(choice.Line); !choice.unreviewed && strconv.Itoa(major) == line.otp {
		hint = fmt.Sprintf(" (or %d for the line's own)", major)
	}
	return NativeChoice{}, &UnsupportedVersionError{Msg: fmt.Sprintf(
		"ERLANG_VERSION=%[1]s is not available with Elixir %[2]s: no published image carries Elixir %[2]s on Erlang/OTP %[1]s; set ERLANG_VERSION to another OTP %[3]d release%[4]s or choose another ELIXIR_VERSION",
		request, elixir, major, hint)}
}

// otpReleases is the official erlang releases a request names, newest first
// and at most maxOTPCandidates: a four-part release names itself, and anything
// shorter its four-part tags (28.3: 28.3.3.0, 28.3.1.0, 28.3.0.0). The official
// repository has a few hundred tags per major.
func otpReleases(ctx context.Context, exact ExactImageResolver, request string) ([]string, error) {
	if strings.Count(request, ".") == 3 {
		return []string{request}, nil
	}
	tags, err := exact.ListTags(ctx, erlangRepo, otpName(request)+".")
	if err != nil {
		return nil, err
	}
	releases := slices.DeleteFunc(tags, func(tag string) bool { return !otpFourPart.MatchString(tag) || !otpMatches(tag, request) })
	slices.SortFunc(releases, func(a, b string) int { return slices.Compare(otpParts(b), otpParts(a)) })
	return releases[:min(len(releases), maxOTPCandidates)], nil
}

// pinDefaultErlang installs Render's default OTP under the default Elixir
// (w8/333) the way pinErlangVersion installs a requested one. It is a default,
// as an exact default patch is: when no image carries it or the registry cannot
// be read, the build keeps the default Elixir's own image, with the OTP that
// image was built on, and the log says so.
func pinDefaultErlang(ctx context.Context, exact ExactImageResolver, choice NativeChoice) NativeChoice {
	pinned, err := pinErlangVersion(ctx, exact, choice)
	if err == nil {
		return pinned
	}
	reason := err.Error()
	if _, unavailable := errors.AsType[*UnsupportedVersionError](err); unavailable {
		reason = "no published image carries it"
	}
	note := fmt.Sprintf("==> Could not install the default Erlang/OTP %s (%s); using the Erlang/OTP of the Elixir image", otpName(choice.erlang), reason)
	choice.erlang = ""
	choice = pinExactVersion(ctx, exact, choice)
	choice.note = strings.TrimSuffix(note+"\n"+choice.note, "\n")
	return choice
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
