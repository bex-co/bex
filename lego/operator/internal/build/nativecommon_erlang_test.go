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
	"os"
	"strings"
	"testing"
	"time"
)

func commonOTPSyntheticHistory(suite string) []string {
	return []string{"# debian.sh --arch 'amd64' out/ '" + suite + "' '@1793059200'", `RUN /bin/sh -c set -xe && OTP_DOWNLOAD_URL="https://github.com/erlang/otp/releases/download/OTP-${OTP_VERSION}/otp_src_${OTP_VERSION}.tar.gz" && OTP_DOWNLOAD_SHA256="` + strings.Repeat("a", 64) + `" && curl -fSL -o otp-src.tar.gz "$OTP_DOWNLOAD_URL"`}
}

func TestCommonElixirKeepsExactOTPAndFullRuntimeFeatures(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	for _, tc := range []struct {
		line, elixir, otp string
		missing           bool
	}{
		{"1.18", "1.18.4", "28.5.0.8", false}, {"1.20", "1.20.4", "29.1.2", false}, {"1.18", "1.18.4", "28.5.0.8", true},
	} {
		t.Run(tc.elixir+"/"+tc.otp+map[bool]string{true: "/source", false: "/donor"}[tc.missing], func(t *testing.T) {
			env := map[string]string{"ELIXIR_VERSION": tc.line}
			reviewed, err := ResolveNativeVersion(nativeElixirRuntime, env, files(nil))
			must(t, err)
			tag := tc.otp + "-debian-bookworm-20261027"
			fake := &fakeExact{digests: map[string]string{}, envByImage: map[string]fakeExactMetadata{}}
			sourceEnv := []string{"OTP_VERSION=" + tc.otp, "ELIXIR_VERSION=v" + tc.elixir, "REBAR_VERSION=2.6.4", "REBAR3_VERSION=3.27.0", "PATH=/usr/local/bin:/usr/bin"}
			fake.envByImage[reviewed.Image] = fakeExactMetadata{sourceEnv, commonOTPSyntheticHistory("trixie")}
			nodeLine, _ := nativeToolchains[nativeNodeRuntime].find("24")
			fake.envByImage[nodeLine.image] = fakeExactMetadata{env: []string{"NODE_VERSION=" + nodeLine.version, "PATH=/usr/local/bin:/usr/bin"}}
			for _, bad := range []string{tc.otp + "-debian-trixie-20261029", tc.otp + "-debian-bookworm-20261030-slim", tc.otp + "-debian-bookworm-invalid", "29.9-debian-bookworm-20261030"} {
				fake.digests[hexpmErlangRepo+":"+bad] = otherDigest
			}
			if !tc.missing {
				fake.digests[hexpmErlangRepo+":"+tag] = otherDigest
				fake.digests[hexpmErlangRepo+":"+tc.otp+"-debian-bookworm-20261020"] = exactDigest
				fake.envByImage[hexpmErlangRepo+":"+tag+"@"+otherDigest] = fakeExactMetadata{[]string{"LANG=C.UTF-8"}, commonOTPSyntheticHistory("bookworm")}
			}
			o := nativeOptions()
			o.Runtime = nativeElixirRuntime
			bundle, path, source := writeResolveFixture(t, o, env, nil)
			choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, o.Runtime, bundle, path, source)
			must(t, err)
			raw, err := os.ReadFile(path)
			must(t, err)
			recipe := string(raw)
			choice := choices[0]
			if choice.Image != reviewed.Image || choice.Line != tc.line || choice.commonOTP != tc.otp {
				t.Fatalf("selected version was changed: %+v", choice)
			}
			for _, want := range []string{
				"FROM " + commonFixtureImage + " AS bex-otp-bookworm\n",
				"source=/usr/local/lib/erlang,target=/opt/bex-otp-reference,readonly /usr/libexec/bex-native/build-otp " + tc.otp + " " + strings.Repeat("a", 64),
				"COPY --link --from=bex-otp-bookworm /usr/local/ /usr/local/\n",
				"COPY --link --from=" + reviewed.Image + " /usr/local/lib/elixir /usr/local/lib/elixir\n",
				"COPY --link --from=" + reviewed.Image + " /usr/local/bin/rebar3 /usr/local/bin/rebar3\n",
				"COPY --link --from=" + reviewed.Image + " /usr/local/bin/rebar /usr/local/bin/rebar\n",
				"ENV OTP_VERSION=\"" + tc.otp + "\"", "ENV LANG=C.UTF-8 ELIXIR_VERSION=v" + tc.elixir,
			} {
				if !strings.Contains(recipe, want) {
					t.Fatalf("missing exact/common runtime component %q:\n%s", want, recipe)
				}
			}
			donorCopy := "COPY --link --from=" + hexpmErlangRepo + ":" + tag + "@" + otherDigest + " /usr/local/ /usr/local/\n"
			if strings.Contains(recipe, donorCopy) == tc.missing {
				t.Fatalf("donor/source fallback routing wrong:\n%s", recipe)
			}
			final := strings.Index(recipe, "\nFROM "+commonFixtureImage+"\n")
			primary := strings.Index(recipe, "COPY --link --from=bex-otp-bookworm /usr/local/")
			rebar := strings.Index(recipe, " /usr/local/bin/rebar3 /usr/local/bin/rebar3")
			node := strings.Index(recipe, " /usr/local/bin/node /usr/local/bin/node")
			elixir := strings.Index(recipe, " /usr/local/lib/elixir /usr/local/lib/elixir")
			if final < 0 || primary < final || rebar < primary || node < rebar || elixir < primary {
				t.Fatalf("primary/tool/companion order lost:\n%s", recipe)
			}
			if strings.Contains(recipe, "apt-get") {
				t.Fatal("OTP compatibility installed tenant apt packages")
			}
		})
	}
}

func TestCommonBookwormExactOTP280NeedsNoReplacement(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	tag := "1.18.4-erlang-28.0-debian-bookworm-20260610"
	image := "erlang:28.0.0.0@" + exactDigest
	fake := &fakeExact{digests: map[string]string{"erlang:28.0.0.0": exactDigest, hexpmElixirRepo + ":" + tag: otherDigest}, envByImage: map[string]fakeExactMetadata{image: {[]string{"OTP_VERSION=28.0", "PATH=/usr/local/bin:/usr/bin"}, commonOTPSyntheticHistory("bookworm")}}}
	nodeLine, _ := nativeToolchains[nativeNodeRuntime].find("24")
	fake.envByImage[nodeLine.image] = fakeExactMetadata{env: []string{"NODE_VERSION=" + nodeLine.version, "PATH=/usr/local/bin:/usr/bin"}}
	o := nativeOptions()
	o.Runtime = nativeElixirRuntime
	bundle, path, source := writeResolveFixture(t, o, map[string]string{"ELIXIR_VERSION": "1.18.4", "ERLANG_VERSION": "28.0"}, nil)
	choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, o.Runtime, bundle, path, source)
	must(t, err)
	raw, err := os.ReadFile(path)
	must(t, err)
	recipe := string(raw)
	if choices[0].Image != image || choices[0].otp != "28.0" || strings.Contains(recipe, "bex-otp-bookworm") || !strings.Contains(recipe, "COPY --link --from="+image+" /usr/local/ /usr/local/\n") {
		t.Fatalf("existing exact Bookworm OTP changed:\n%s", recipe)
	}
}

func TestCommonOTPSourceChecksumRequiresOneBoundedOfficialSource(t *testing.T) {
	good := commonOTPSyntheticHistory("trixie")
	if got, err := commonOTPSourceChecksum(good); err != nil || got != strings.Repeat("a", 64) {
		t.Fatalf("good source: %q %v", got, err)
	}
	for name, history := range map[string][]string{
		"missing": good[:1], "duplicate": append(append([]string{}, good...), good[1]),
		"other host": {good[0], strings.Replace(good[1], "github.com/erlang/otp", "example.invalid/otp", 1)},
		"bad hash":   {good[0], strings.Replace(good[1], strings.Repeat("a", 64), "not-a-digest", 1)},
		"oversize":   {good[0], good[1] + strings.Repeat("x", 65<<10)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := commonOTPSourceChecksum(history); err == nil {
				t.Fatal("invalid/ambiguous source checksum accepted")
			}
		})
	}
}
