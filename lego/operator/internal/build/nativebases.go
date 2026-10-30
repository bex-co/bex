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
	"fmt"
	"regexp"
	"strings"
)

// nativeSuppliedBases maps reviewed official bases to their reviewed,
// immutable supplied derivatives. Publication alone never changes this map.
// Keep official toolchain pins separate: exact selection and companions must
// continue to resolve the upstream primary rather than a derivative tag.
var nativeSuppliedBases = map[string]string{}

var nativeImageEnvName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var nativeDebianHistory = regexp.MustCompile(`^# debian\.sh --arch '(?:amd64|arm64)' out/ '(bullseye|bookworm|trixie)' '@[0-9]+'$`)
var nativeLegacyDebianHistory = regexp.MustCompile(`^(?:/bin/sh -c #\(nop\) )?ADD file:[a-f0-9]{64} in /$`)

// nativeDebianSuite accepts only Debian's own root-image provenance format.
// The legacy ADD format names no suite; callers must not infer one from dates.
func nativeDebianSuite(history []string) (string, error) {
	if len(history) == 0 || len(history) > 1024 {
		return "", fmt.Errorf("native image history has an invalid entry count")
	}
	total := 0
	for _, entry := range history {
		total += len(entry)
		if len(entry) > 64<<10 || total > maxVersionFileBytes {
			return "", fmt.Errorf("native image history exceeds the provenance limit")
		}
	}
	root := strings.TrimSpace(history[0])
	for _, entry := range history[1:] {
		if strings.HasPrefix(strings.TrimSpace(entry), "# debian.sh ") {
			return "", fmt.Errorf("native image history has ambiguous Debian roots")
		}
	}
	if m := nativeDebianHistory.FindStringSubmatch(root); m != nil {
		return m[1], nil
	}
	if nativeLegacyDebianHistory.MatchString(root) {
		return "", nil
	}
	return "", fmt.Errorf("native image history does not identify a reviewed Debian root")
}

// suppliedNativeImage keeps exact OTP on the same Debian suite as its source.
// Older Elixir/OTP images used Debian's legacy ADD history: the reviewed line
// carrying that actual OTP major is the compatible candidate, and the generated
// filesystem check still verifies its suite before any primary is replaced.
func suppliedNativeImage(reviewedImage string, choice NativeChoice, supplied map[string]string) (string, error) {
	reviewedSource := cmp.Or(choice.upstream, reviewedImage)
	selected := reviewedSource
	if choice.Image != reviewedImage && strings.HasPrefix(reviewedSource, "elixir:") {
		wantedOTP := ""
		if choice.imageSuite == "" {
			for _, entry := range choice.imageEnv {
				if value, found := strings.CutPrefix(entry, "OTP_VERSION="); found {
					if !otpRelease.MatchString(value) {
						return "", fmt.Errorf("legacy native image has an invalid OTP version")
					}
					wantedOTP, _, _ = strings.Cut(value, ".")
				}
			}
			if wantedOTP == "" {
				return "", fmt.Errorf("legacy native image has no verified OTP version")
			}
		}
		matched := false
		for _, line := range nativeToolchains[nativeElixirRuntime].lines {
			if (choice.imageSuite != "" && line.debian == choice.imageSuite) || (wantedOTP != "" && line.otp == wantedOTP) {
				if !matched {
					selected, matched = cmp.Or(line.upstream, line.image), true
				}
				if line.image == reviewedImage {
					selected = reviewedSource
					break
				}
			}
		}
		if !matched {
			return "", fmt.Errorf("no reviewed supplied Elixir base matches the exact image's Debian/OTP provenance")
		}
	}
	image, ok := supplied[selected]
	if !ok {
		return "", fmt.Errorf("no reviewed supplied base for %s", selected)
	}
	return image, nil
}

// pinNativePrimary keeps exact-version fallback behavior while collecting the
// verified metadata needed to transplant that primary onto a supplied base.
func pinNativePrimary(ctx context.Context, exact ExactImageResolver, runtime string, reviewed NativeChoice) (NativeChoice, error) {
	choice := reviewed
	var err error
	if choice.erlang != "" {
		choice, err = pinErlangVersion(ctx, exact, choice)
		if err != nil {
			return NativeChoice{}, err
		}
	} else {
		choice = pinExactVersion(ctx, exact, choice)
	}
	if len(nativeSuppliedBases) == 0 || choice.Image == reviewed.Image {
		return choice, nil
	}
	requestedImage := choice.Image
	var history []string
	choice.imageEnv, history, err = exact.ImageMetadata(ctx, choice.Image)
	if err == nil && runtime == nativeElixirRuntime {
		choice.imageSuite, err = nativeDebianSuite(history)
	}
	if err == nil {
		return choice, nil
	}
	if choice.erlang != "" {
		return NativeChoice{}, fmt.Errorf("read exact Erlang image environment: %w", err)
	}
	reviewed.note = fmt.Sprintf("==> Could not read the exact image %s environment (%v); using the reviewed %s %s image", requestedImage, err, reviewed.Label, reviewed.Line)
	return reviewed, nil
}

// applyNativeSuppliedBase runs after all language and companion selection.
// Its primary replacement is prepended ahead of companions so an exact
// primary cannot erase Node, Ruby, Bun, Poetry, or their links.
func applyNativeSuppliedBase(dockerfile, reviewedImage string, choice NativeChoice, supplied map[string]string) (string, error) {
	if len(supplied) == 0 {
		return dockerfile, nil
	}
	image, err := suppliedNativeImage(reviewedImage, choice, supplied)
	if err != nil {
		return "", err
	}
	ref, digest, pinned := strings.Cut(image, "@")
	if !pinned || ref == "" || strings.ContainsAny(ref, " \t\r\n") || !digestPattern.MatchString(digest) {
		return "", fmt.Errorf("supplied native base is not pinned: %s", image)
	}
	out, err := rewriteNativeBase(dockerfile, choice.Image, image)
	if err != nil || choice.Image == reviewedImage {
		return out, err
	}
	if len(choice.imageEnv) == 0 {
		return "", fmt.Errorf("exact primary %s has no verified environment", choice.Image)
	}
	// COPY merges directories. Remove the reviewed primary first so a newer
	// patch's modules or extension files cannot survive beside the requested
	// interpreter. Debian packages and supplied tools live outside /usr/local.
	var lines strings.Builder
	lines.WriteString("COPY --link --from=" + choice.Image + " /usr/lib/os-release /opt/bex-primary-os-release\n" +
		"RUN . /opt/bex-primary-os-release && [ -n \"$VERSION_CODENAME\" ] && primary_os=\"$VERSION_CODENAME\" && . /etc/os-release && [ -n \"$VERSION_CODENAME\" ] && " +
		"{ [ \"$primary_os\" = \"$VERSION_CODENAME\" ] || { echo 'exact native primary and supplied base have different Debian distributions' >&2; exit 1; }; } && " +
		"rm -f /opt/bex-primary-os-release && rm -rf /usr/local && mkdir -p /usr/local\n" +
		"COPY --link --from=" + choice.Image + " /usr/local/ /usr/local/\n" +
		"RUN --network=none /sbin/ldconfig\n")
	escaper := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`)
	for _, item := range choice.imageEnv {
		key, value, valid := strings.Cut(item, "=")
		if !valid || !nativeImageEnvName.MatchString(key) || strings.ContainsAny(value, "\x00\r\n") {
			return "", fmt.Errorf("exact primary %s has an invalid environment entry", choice.Image)
		}
		escaped := escaper.Replace(value)
		fmt.Fprintf(&lines, "ENV %s=\"%s\"\n", key, escaped)
	}
	if strings.HasPrefix(cmp.Or(choice.upstream, reviewedImage), "python:") {
		lines.WriteString("RUN --network=none /usr/bin/bex-native-python-tools\n")
	}
	return afterNativeBase(out, image, lines.String())
}
