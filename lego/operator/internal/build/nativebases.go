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

	"github.com/bex-co/bex/lego/types/toolchains"
)

// nativeCommonBase is the user-selected single Bookworm system/tools image
// (2026-10-29): every native build's final stage starts from it, with the
// selected runtime copied on top. Pin only a digest that one
// native-common-image.yml families run published and passed on every family
// case; this one is run 45839946482 (w3/m635, on buildpack-deps:bookworm's
// 2026-11-24 rebuild; w6/m549 adopted run 44906752907's, the first with
// magic-wormhole, w8/335 run 43900534897's, w8/m91 t008 run 43883107194's).
var nativeCommonBase = "ghcr.io/bex-co/bex-cnb-run@sha256:9c3fac2fc2a764c83434eb28e9821bd734f64bbce68efe42d6593cff29280df5"

// nativeRuntimeArtifacts reuses approved runtime TREES, never their final
// system base. Keys are exact selected upstream identities; a line artifact
// must not substitute a newer patch for another exact request. These are the
// published Node 14/16/18/20 and exact 14.21.2 trees with the Brotli backport
// (2026-11-04 donor route); the families run qualifies exactly this map
// (commonNodeQualificationArtifacts).
var nativeRuntimeArtifacts = map[string]string{
	"node:14-bullseye@sha256:c0bff0d29a742f40650d5f0305dd581351c10954e6cb6676fc96f47590b9666e":      "ghcr.io/bex-co/bex-cnb-run@sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc",
	"node:14.21.2-bullseye@sha256:cc748803fe49871926c433ae6aafbfd59b765eed6005dc0843b916eb74f67b12": "ghcr.io/bex-co/bex-cnb-run@sha256:332869e644cdd45bf1f9e7c42ed3df91fedb95c4d0cee6bf2abf771c9eb760c7",
	"node:16-bookworm@sha256:73e677f85fbc56d22c817b4e5d7f72086c5343c0859f3bf8aaadab22aab29ce6":      "ghcr.io/bex-co/bex-cnb-run@sha256:2802078e18496838c15159ec99c01c9b260128cef8e1d1ba29cd0a1836165113",
	"node:18-bookworm@sha256:c6ae79e38498325db67193d391e6ec1d224d96c693a8a4d943498556716d3783":      "ghcr.io/bex-co/bex-cnb-run@sha256:6c40aaf4af55979e9b923e5949704f562d8e334e7370022f0c3584341742ddd1",
	"node:20-bookworm@sha256:8f693eaa7e0a8e71560c9a82b55fd54c2ae920a2ba5d2cde28bac7d1c01c9ba5":      "ghcr.io/bex-co/bex-cnb-run@sha256:2c69d3e2913703a634016cc24338f9532f24a750ddb831351e1374d274846a4d",
}

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
		if len(entry) > 64<<10 || total > toolchains.MaxVersionFileBytes {
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

// pinNativePrimary keeps exact-version fallback behavior while collecting the
// verified metadata needed to transplant that primary onto the common base.
// A Node release no reviewed line carries has no fallback: it installs or
// fails by name (w8/398). readFile reads the service's version files.
func pinNativePrimary(ctx context.Context, exact ExactImageResolver, runtime string, reviewed NativeChoice, readFile func(string) ([]byte, bool)) (NativeChoice, error) {
	choice := reviewed
	var err error
	switch {
	case runtime == nativeNodeRuntime && choice.unreviewed:
		if choice, err = pinNodeRelease(ctx, exact, choice, readFile); err != nil {
			return NativeChoice{}, err
		}
	case choice.erlang == "":
		choice = pinExactVersion(ctx, exact, choice)
	case choice.Source == "":
		// No version signal: the OTP is Render's default, which falls back
		// where a requested one fails by name.
		choice = pinDefaultErlang(ctx, exact, choice)
	default:
		if choice, err = pinErlangVersion(ctx, exact, choice); err != nil {
			return NativeChoice{}, err
		}
	}
	if nativeCommonBase == "" {
		return choice, nil
	}
	// The common image has no primary, including when selection kept the
	// reviewed default. Never substitute another language patch because its
	// environment could not be read; preserve identity and report the error.
	return pinCommonNativePrimary(ctx, exact, runtime, choice)
}

// applyNativeCommonBase runs after all language and companion selection. It
// moves the final stage onto the common image and installs the selected
// primary, including a reviewed default, ahead of companions and authored
// commands, so the primary cannot erase Node, Ruby, Bun, Poetry, or their
// links. Without a common base the recipe keeps the selected official image.
func applyNativeCommonBase(dockerfile, reviewedImage string, choice NativeChoice) (string, error) {
	image := nativeCommonBase
	if image == "" {
		return dockerfile, nil
	}
	if err := validateNativeBase(image); err != nil {
		return "", err
	}
	out, err := rewriteNativeBase(dockerfile, choice.Image, image)
	if err != nil {
		return "", err
	}
	if len(choice.imageEnv) == 0 {
		return "", fmt.Errorf("exact primary %s has no verified environment", choice.Image)
	}
	// COPY merges directories. Remove the reviewed primary first so a newer
	// patch's modules or extension files cannot survive beside the requested
	// interpreter. Debian packages and supplied tools live outside /usr/local.
	var lines strings.Builder
	primary := cmp.Or(choice.primaryImage, choice.Image)
	lines.WriteString(nativeCommonSourceGuard(primary, choice.legacySource, true) +
		"COPY --link --from=" + primary + " /usr/local/ /usr/local/\n" + commonRuntimeProvenance(choice) +
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
	lines.WriteString(choice.runtimeCheck)
	if strings.HasPrefix(cmp.Or(choice.upstream, reviewedImage), "python:") {
		lines.WriteString("RUN --network=none /usr/bin/bex-native-python-tools\n")
	}
	lines.WriteString(commonErlangToolLines(choice))
	out, err = afterNativeBase(out, image, lines.String())
	if err == nil {
		out, err = prependNativeRuntimeStage(out, image, choice.primaryStage)
	}
	return out, err
}

func validateNativeBase(image string) error {
	ref, digest, pinned := strings.Cut(image, "@")
	if !pinned || ref == "" || strings.ContainsAny(ref, " \t\r\n") || !digestPattern.MatchString(digest) {
		return fmt.Errorf("native base image is not pinned by digest: %s", image)
	}
	return nil
}

func prependNativeRuntimeStage(dockerfile, finalImage, stage string) (string, error) {
	if stage == "" {
		return dockerfile, nil
	}
	from := "\nFROM " + finalImage + "\n"
	if strings.Count(dockerfile, from) != 1 {
		return "", fmt.Errorf("generated runtime stage lacks its unique final base")
	}
	return strings.Replace(dockerfile, from, "\n"+stage+from, 1), nil
}
