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
	"strings"
)

var commonErlangTag = regexp.MustCompile(`^(\d+(?:\.\d+){1,3})-debian-bookworm-(\d{8}(?:\.\d+)?)$`)
var otpSourceChecksum = regexp.MustCompile(`(?:^|[\t ;&])OTP_DOWNLOAD_SHA256="([a-f0-9]{64})"(?:[\t ;&]|$)`)

func pinCommonNativePrimary(ctx context.Context, exact ExactImageResolver, runtime string, choice NativeChoice) (NativeChoice, error) {
	var history []string
	var err error
	choice.imageEnv, history, err = exact.ImageMetadata(ctx, choice.Image)
	if err != nil {
		return NativeChoice{}, fmt.Errorf("read selected native primary environment: %w", err)
	}
	if runtime != nativeElixirRuntime {
		return commonNativeRuntimeChoice(ctx, exact, runtime, choice)
	}
	suite, err := nativeDebianSuite(history)
	if err != nil {
		return NativeChoice{}, err
	}
	if suite == "bookworm" {
		return choice, nil
	}
	otp, err := commonImageVariable(choice.imageEnv, "OTP_VERSION")
	if err != nil || !otpRelease.MatchString(otp) {
		return NativeChoice{}, fmt.Errorf("selected Elixir has no valid exact OTP environment")
	}
	if choice.otp != "" && !slices.Equal(otpParts(choice.otp), otpParts(otp)) {
		return NativeChoice{}, fmt.Errorf("selected exact OTP differs from its verified environment")
	}
	if choice.elixirImage == "" {
		version, err := commonImageVariable(choice.imageEnv, "ELIXIR_VERSION")
		if err != nil || !fullVersion.MatchString(strings.TrimPrefix(version, "v")) {
			return NativeChoice{}, fmt.Errorf("selected Elixir has no valid release environment")
		}
		choice.elixirImage = choice.Image
		choice.elixirVersion = strings.TrimPrefix(version, "v")
	}
	choice.commonOTP = otp
	donor, err := commonBookwormOTP(ctx, exact, otp)
	if err != nil {
		return NativeChoice{}, err
	}
	checksum, err := commonOTPSourceChecksum(history)
	if err != nil {
		return NativeChoice{}, err
	}
	// BuildKit caches this immutable common-base/version/checksum tuple. No
	// apt operation, moving release, or new unavailable-version result.
	choice.primaryImage = "bex-otp-bookworm"
	choice.primaryStage = "FROM " + nativeCommonBase + " AS bex-otp-bookworm\n"
	if donor != "" {
		choice.primaryStage += "COPY --link --from=" + donor + " /usr/local/ /usr/local/\n"
	}
	// Some providers omit wx and native headers. Compare the same OTP's
	// official application/NIF/header inventory; fill it by building the exact
	// source release rather than silently losing those applications.
	choice.primaryStage += "RUN --mount=type=bind,from=" + choice.Image + ",source=/usr/local/lib/erlang,target=/opt/bex-otp-reference,readonly /usr/libexec/bex-native/build-otp " + otp + " " + checksum + "\n"
	return choice, nil
}

func commonImageVariable(env []string, key string) (string, error) {
	value := ""
	found := false
	for _, entry := range env {
		if candidate, ok := strings.CutPrefix(entry, key+"="); ok {
			if found || candidate == "" {
				return "", fmt.Errorf("ambiguous image environment %s", key)
			}
			value, found = candidate, true
		}
	}
	if !found {
		return "", fmt.Errorf("image environment lacks %s", key)
	}
	return value, nil
}

func commonBookwormOTP(ctx context.Context, exact ExactImageResolver, otp string) (string, error) {
	prefix := otp + "-debian-bookworm-"
	tags, err := exact.ListTags(ctx, hexpmErlangRepo, prefix)
	if err != nil {
		return "", fmt.Errorf("look up exact Bookworm OTP %s: %w", otp, err)
	}
	candidates := make([]string, 0, len(tags))
	for _, tag := range tags {
		m := commonErlangTag.FindStringSubmatch(tag)
		if m != nil && m[1] == otp {
			candidates = append(candidates, tag)
		}
	}
	slices.Sort(candidates)
	slices.Reverse(candidates)
	for _, tag := range candidates {
		digest, found, err := exact.ResolveTag(ctx, hexpmErlangRepo, tag)
		if err != nil {
			return "", err
		}
		if !found {
			continue
		}
		if !digestPattern.MatchString(digest) {
			return "", fmt.Errorf("bookworm OTP donor is not digest pinned")
		}
		image := hexpmErlangRepo + ":" + tag + "@" + digest
		_, history, err := exact.ImageMetadata(ctx, image)
		if err != nil {
			return "", err
		}
		suite, err := nativeDebianSuite(history)
		if err != nil || suite != "bookworm" {
			return "", fmt.Errorf("OTP donor does not have verified Bookworm provenance")
		}
		return image, nil
	}
	return "", nil
}

func commonOTPSourceChecksum(history []string) (string, error) {
	if _, err := nativeDebianSuite(history); err != nil {
		return "", err
	}
	checksum := ""
	for _, entry := range history {
		for _, match := range otpSourceChecksum.FindAllStringSubmatch(entry, -1) {
			if checksum != "" {
				return "", fmt.Errorf("ambiguous exact OTP source checksum")
			}
			if !strings.Contains(entry, `OTP_DOWNLOAD_URL="https://github.com/erlang/otp/releases/download/OTP-${OTP_VERSION}/otp_src_${OTP_VERSION}.tar.gz"`) {
				return "", fmt.Errorf("OTP source checksum is not bound to the official release URL")
			}
			checksum = match[1]
		}
	}
	if checksum == "" {
		return "", fmt.Errorf("selected OTP image lacks a verified source checksum")
	}
	return checksum, nil
}

func commonErlangToolLines(choice NativeChoice) string {
	if choice.commonOTP == "" {
		return ""
	}
	return "COPY --link --from=" + choice.Image + " /usr/local/bin/rebar /usr/local/bin/rebar\n" +
		"COPY --link --from=" + choice.Image + " /usr/local/bin/rebar3 /usr/local/bin/rebar3\n" +
		"RUN --network=none erl -noshell -eval '{ok,B}=file:read_file(filename:join([code:root_dir(),\"releases\",erlang:system_info(otp_release),\"OTP_VERSION\"])), \"" + choice.commonOTP + "\"=string:trim(binary_to_list(B)), halt(0).' && rebar --version && rebar3 --version\n"
}
