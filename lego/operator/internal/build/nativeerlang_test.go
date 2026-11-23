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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// hexpmTags are hexpm builds of Elixir 1.19.5 and 1.18.4 across OTP 28
// releases, operating systems and build dates, as Docker Hub lists them.
var hexpmTags = []string{
	"1.19.5-erlang-28.3-debian-trixie-20260610",
	"1.19.5-erlang-28.3.1-debian-trixie-20260610",
	"1.19.5-erlang-28.3.3-debian-trixie-20260713",
	"1.19.5-erlang-28.3.3-debian-trixie-20260824",
	"1.19.5-erlang-28.3.3-debian-bookworm-20260824-slim",
	"1.19.5-erlang-28.3.3-alpine-3.22.5",
	"1.19.5-erlang-28.30.1-debian-trixie-20260824",
	"1.19.5-erlang-28.5.0.6-debian-trixie-20260824",
	"1.18.4-erlang-28.3.3-debian-bookworm-20260824",
	// w8/373: an Elixir no line carries and another OTP major than the line's.
	"1.16.3-erlang-26.2.5.20-debian-bookworm-20260610",
	"1.16.3-erlang-26.2.5.20-debian-trixie-20261104",
	"1.12.3-erlang-24.3.4.17-debian-bookworm-20260610",
	"1.18.4-erlang-27.3.4.16-debian-bookworm-20260824",
}

const otherDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000002"

// erlangFake publishes hexpmTags and the given official erlang tags.
func erlangFake(erlangTags ...string) *fakeExact {
	digests := map[string]string{}
	for _, tag := range hexpmTags {
		digests[hexpmElixirRepo+":"+tag] = otherDigest
	}
	for _, tag := range erlangTags {
		digests["erlang:"+tag] = exactDigest
	}
	return &fakeExact{digests: digests}
}

func TestExactErlangFullReleaseLimitsTheTagSearch(t *testing.T) {
	fake := erlangFake("28.5.0.6")
	fake.envByImage = map[string]fakeExactMetadata{"erlang:28.5.0.6@" + exactDigest: {
		[]string{"OTP_VERSION=28.5.0.6", "PATH=/usr/local/bin:/usr/bin"}, commonOTPSyntheticHistory("trixie")}}
	resolveExact(t, fake, nativeElixirRuntime, map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.5.0.6"}, nil)
	if len(fake.asked) == 0 || fake.asked[0] != "list hexpm/elixir:1.19.5-erlang-28.5.0.6-debian-" {
		t.Fatalf("a fully specified release searched unrelated image families: %v", fake.asked)
	}
}

// w8/197: ERLANG_VERSION finer than a major builds on the official erlang image
// of the newest matching OTP release both registries publish, with Elixir from
// hexpm's build on that release, and the log names the OTP installed.
func TestExactErlangBuildsOnTheRequestedOTP(t *testing.T) {
	cases := []struct {
		name          string
		env           map[string]string
		erlangTags    []string
		erlang, hexpm string
		narration     string
	}{
		{"newest patch of a minor", map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.3"},
			[]string{"28.3.3.0", "28.3.1.0", "28.3.0.0", "28.30.1.0"}, "28.3.3.0", "1.19.5-erlang-28.3.3-debian-trixie-20260824",
			"==> Using Elixir 1.19.5 on Erlang/OTP 28.3.3 (from ELIXIR_VERSION=1.19.5, ERLANG_VERSION=28.3)"},
		{"an exact patch", map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "OTP-28.3.1"},
			[]string{"28.3.3.0", "28.3.1.0"}, "28.3.1.0", "1.19.5-erlang-28.3.1-debian-trixie-20260610",
			"==> Using Elixir 1.19.5 on Erlang/OTP 28.3.1 (from ELIXIR_VERSION=1.19.5, ERLANG_VERSION=OTP-28.3.1)"},
		{"the minor's first release", map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.3.0"},
			[]string{"28.3.3.0", "28.3.0.0"}, "28.3.0.0", "1.19.5-erlang-28.3-debian-trixie-20260610",
			"==> Using Elixir 1.19.5 on Erlang/OTP 28.3 (from ELIXIR_VERSION=1.19.5, ERLANG_VERSION=28.3.0)"},
		{"a maintenance patch", map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.5.0.6"},
			[]string{"28.5.0.6"}, "28.5.0.6", "1.19.5-erlang-28.5.0.6-debian-trixie-20260824",
			"==> Using Elixir 1.19.5 on Erlang/OTP 28.5.0.6 (from ELIXIR_VERSION=1.19.5, ERLANG_VERSION=28.5.0.6)"},
		{"newest patch without an official image", map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.3"},
			[]string{"28.3.1.0", "28.3.0.0"}, "28.3.1.0", "1.19.5-erlang-28.3.1-debian-trixie-20260610",
			"==> Using Elixir 1.19.5 on Erlang/OTP 28.3.1 (from ELIXIR_VERSION=1.19.5, ERLANG_VERSION=28.3)"},
		{"default Elixir line", map[string]string{"ERLANG_VERSION": "28.3"},
			[]string{"28.3.3.0"}, "28.3.3.0", "1.18.4-erlang-28.3.3-debian-bookworm-20260824",
			"==> Using Elixir 1.18.4 on Erlang/OTP 28.3.3 (from ERLANG_VERSION=28.3)"},
		// w8/373: an Elixir no line carries builds on the newest OTP major it
		// supports, at the newest release hexpm built it on.
		{"an Elixir no line carries", map[string]string{"ELIXIR_VERSION": "1.16.3"},
			[]string{"26.2.5.21", "26.2.5.20", "26.2.4.0", "27.3.4.19"}, "26.2.5.20", "1.16.3-erlang-26.2.5.20-debian-trixie-20261104",
			"==> Using Elixir 1.16.3 on Erlang/OTP 26.2.5.20 (from ELIXIR_VERSION=1.16.3)"},
		{"a bare minor no line carries", map[string]string{"ELIXIR_VERSION": "1.12", "ERLANG_VERSION": "24.3.4"},
			[]string{"24.3.4.17", "24.3.4.0", "24.3.3.0"}, "24.3.4.17", "1.12.3-erlang-24.3.4.17-debian-bookworm-20260610",
			"==> Using Elixir 1.12.3 on Erlang/OTP 24.3.4.17 (from ELIXIR_VERSION=1.12, ERLANG_VERSION=24.3.4)"},
		{"another OTP major than the line's", map[string]string{"ELIXIR_VERSION": "1.18.4", "ERLANG_VERSION": "27.3"},
			[]string{"27.3.4.19", "27.3.4.18", "27.3.4.16", "27.3.0.0"}, "27.3.4.16", "1.18.4-erlang-27.3.4.16-debian-bookworm-20260824",
			"==> Using Elixir 1.18.4 on Erlang/OTP 27.3.4.16 (from ELIXIR_VERSION=1.18.4, ERLANG_VERSION=27.3)"},
		{"another OTP major alone", map[string]string{"ELIXIR_VERSION": "1.18.4", "ERLANG_VERSION": "27"},
			[]string{"27.3.4.19", "27.3.4.16", "26.2.5.21"}, "27.3.4.16", "1.18.4-erlang-27.3.4.16-debian-bookworm-20260824",
			"==> Using Elixir 1.18.4 on Erlang/OTP 27.3.4.16 (from ELIXIR_VERSION=1.18.4, ERLANG_VERSION=27)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := erlangFake(tc.erlangTags...)
			_, release, _ := strings.Cut(tc.hexpm, "-erlang-")
			release, _, _ = strings.Cut(release, "-debian-")
			otpEnv := []string{"OTP_VERSION=" + release, "PATH=/usr/local/bin:/usr/bin"}
			fake.envByImage = map[string]fakeExactMetadata{"erlang:" + tc.erlang + "@" + exactDigest: {otpEnv, commonOTPSyntheticHistory("trixie")}}
			// A real service has a creation date, which selects Render's
			// default Elixir when only ERLANG_VERSION is set (w6/m291).
			choices, dockerfile := resolveExactAt(t, fake, renderElixirDefault20250612.Add(time.Hour), "elixir", tc.env, nil)
			reviewed, err := ResolveNativeVersion(nativeElixirRuntime, tc.env, files(nil))
			if err != nil {
				t.Fatal(err)
			}
			// A trixie OTP is rebuilt on the common base in its own stage, which the
			// final image then copies like any exact primary.
			assertExactNativePrimary(t, dockerfile, reviewed.Image, cmp.Or(choices[0].primaryImage, "erlang:"+tc.erlang+"@"+exactDigest), otpEnv)
			for _, want := range []string{
				"COPY --link --from=hexpm/elixir:" + tc.hexpm + "@" + otherDigest + " /usr/local/lib/elixir /usr/local/lib/elixir\n",
				"/usr/local/bin/\nENV LANG=C.UTF-8 ELIXIR_VERSION=v" + choices[0].Line + "\n",
			} {
				if !strings.Contains(dockerfile, want) {
					t.Errorf("Dockerfile lacks %q:\n%s", want, dockerfile)
				}
			}
			if strings.Contains(dockerfile, "FROM elixir:") {
				t.Errorf("Dockerfile still builds on an official elixir image:\n%s", dockerfile)
			}
			if choices[0].Image != "erlang:"+tc.erlang+"@"+exactDigest {
				t.Errorf("exact OTP identity changed: %+v", choices[0])
			}
			if got := shown(choices[0].Narration()); got != tc.narration+elixirDocs {
				t.Errorf("narration %q, want %q", got, tc.narration+elixirDocs)
			}
			for _, asked := range fake.asked {
				if strings.HasPrefix(asked, "elixir:") {
					t.Errorf("looked up the official elixir image %s, which the build does not use", asked)
				}
			}
		})
	}
}

// w8/197: an OTP release no image carries for the Elixir version fails the
// build by name, and a registry failure fails it too, rather than building on
// another OTP.
func TestExactErlangUnavailableFailsByName(t *testing.T) {
	for name, tc := range map[string]struct {
		env        map[string]string
		erlangTags []string
		want       string
	}{
		"no such OTP": {map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.9"}, []string{"28.3.3.0"},
			"ERLANG_VERSION=28.9 is not available with Elixir 1.19.5: no published image carries Elixir 1.19.5 on Erlang/OTP 28.9"},
		"no official erlang image": {map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.3.3"}, nil,
			"no published image carries Elixir 1.19.5 on Erlang/OTP 28.3.3"},
		"no hexpm build of that Elixir": {map[string]string{"ELIXIR_VERSION": "1.19.4", "ERLANG_VERSION": "28.3"}, []string{"28.3.3.0"},
			"ERLANG_VERSION=28.3 is not available with Elixir 1.19.4"},
		// w8/373: an Elixir no line carries never builds on another Elixir.
		"no such Elixir": {map[string]string{"ELIXIR_VERSION": "1.16.9"}, []string{"26.2.5.20"},
			"ELIXIR_VERSION=1.16.9 is not available: no published image carries Elixir 1.16.9 on Erlang/OTP 26, the newest it supports"},
		"another major without a build": {map[string]string{"ELIXIR_VERSION": "1.18.4", "ERLANG_VERSION": "26"}, []string{"26.2.5.20"},
			"ERLANG_VERSION=26 is not available with Elixir 1.18.4: no published image carries Elixir 1.18.4 on Erlang/OTP 26; set ERLANG_VERSION to another OTP 26 release or choose another ELIXIR_VERSION"},
	} {
		t.Run(name, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = "elixir"
			bundle, dockerfile, src := writeResolveFixture(t, o, tc.env, nil)
			before, _ := os.ReadFile(dockerfile)
			_, err := ResolveNativeBuildExact(context.Background(), erlangFake(tc.erlangTags...), time.Time{}, "elixir", bundle, dockerfile, src)
			var unsupported *UnsupportedVersionError
			if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want an UnsupportedVersionError containing %q", err, tc.want)
			}
			if after, _ := os.ReadFile(dockerfile); string(after) != string(before) {
				t.Errorf("a refused build rewrote its Dockerfile")
			}
		})
	}

	t.Run("registry failure", func(t *testing.T) {
		o := nativeOptions()
		o.Runtime = "elixir"
		bundle, dockerfile, src := writeResolveFixture(t, o, map[string]string{"ELIXIR_VERSION": "1.19.5", "ERLANG_VERSION": "28.3"}, nil)
		fake := erlangFake("28.3.3.0")
		fake.err = errors.New("registry answered 500")
		_, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, "elixir", bundle, dockerfile, src)
		var unsupported *UnsupportedVersionError
		if err == nil || errors.As(err, &unsupported) || !strings.Contains(err.Error(), "Erlang/OTP 28.3") {
			t.Fatalf("err = %v, want a platform error naming the lookup", err)
		}
	})
}

// w8/333: Render's default Elixir 1.18.4 runs on its default Erlang/OTP
// 28.0.2, not on the newer 28.x the official elixir image was built with. A
// service created since the cutoff with neither ELIXIR_VERSION nor
// ERLANG_VERSION builds on erlang:28.0.2.0 with hexpm's Elixir 1.18.4 for that
// release, and the log names both. An explicit version, an older or
// unknown-age service, and a default no image provides keep what they had.
func TestDefaultElixirBuildsOnRendersDefaultOTP(t *testing.T) {
	const (
		erlangImage = "erlang:28.0.2.0@" + exactDigest
		newest      = "1.18.4-erlang-28.0.2-debian-trixie-20260610"
	)
	otpEnv := []string{"OTP_VERSION=28.0.2", "PATH=/usr/local/bin:/usr/bin"}
	published := func() *fakeExact {
		fake := &fakeExact{digests: map[string]string{
			"erlang:28.0.2.0": exactDigest, "erlang:28.0.3.0": exactDigest, "elixir:1.18.4": exactDigest,
			hexpmElixirRepo + ":" + newest:                                          otherDigest,
			hexpmElixirRepo + ":1.18.4-erlang-28.0.2-debian-bookworm-20260518":      otherDigest,
			hexpmElixirRepo + ":1.18.4-erlang-28.0.2-debian-bookworm-20260610-slim": otherDigest,
			hexpmElixirRepo + ":1.18.4-erlang-28.0.3-debian-bookworm-20260824":      otherDigest,
			hexpmElixirRepo + ":1.18.4-erlang-28.5.0.8-debian-trixie-20260824":      otherDigest,
		}}
		fake.envByImage = map[string]fakeExactMetadata{erlangImage: {otpEnv, commonOTPSyntheticHistory("bookworm")}}
		return fake
	}
	created := erlangDefaultCutoff.Add(time.Hour)
	reviewed, err := ResolveNativeVersion(nativeElixirRuntime, nil, files(nil))
	must(t, err)
	other, _ := nativeToolchains[nativeElixirRuntime].find("1.19")

	fake := published()
	choices, dockerfile := resolveExactAt(t, fake, created, nativeElixirRuntime, nil, nil)
	// Render names the default Erlang on a line of its own, before Elixir
	// (w9/139).
	if got, want := shown(choices[0].Narration()), "==> Using Erlang version 28.0.2 (default)\n==> Using Elixir version 1.18.4 (default)"+elixirDocs; got != want {
		t.Errorf("narration %q, want %q", got, want)
	}
	if choices[0].Image != erlangImage || choices[0].Line != "1.18.4" {
		t.Errorf("default build is not on Erlang/OTP 28.0.2: %+v", choices[0])
	}
	assertExactNativePrimary(t, dockerfile, reviewed.Image, erlangImage, otpEnv)
	for _, want := range []string{
		"COPY --link --from=" + hexpmElixirRepo + ":" + newest + "@" + otherDigest + " /usr/local/lib/elixir /usr/local/lib/elixir\n",
		"ENV LANG=C.UTF-8 ELIXIR_VERSION=v1.18.4\n",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Errorf("Dockerfile lacks %q:\n%s", want, dockerfile)
		}
	}
	if !slices.Equal(fake.asked[:1], []string{"list " + hexpmElixirRepo + ":1.18.4-erlang-28.0.2-debian-"}) || slices.Contains(fake.asked, "elixir:1.18.4") {
		t.Errorf("the default looked beyond Erlang/OTP 28.0.2 or at the official elixir image: %v", fake.asked)
	}

	for name, tc := range map[string]struct {
		created     time.Time
		env         map[string]string
		image, line string
	}{
		"created before the cutoff": {erlangDefaultCutoff.Add(-time.Hour), nil, "elixir:1.18.4@" + exactDigest, "1.18.4"},
		"unknown age":               {time.Time{}, nil, reviewed.Image, reviewed.Line},
		"explicit default line":     {created, map[string]string{"ELIXIR_VERSION": "1.18"}, reviewed.Image, reviewed.Line},
		"explicit other Elixir":     {created, map[string]string{"ELIXIR_VERSION": "1.19"}, other.image, "1.19"},
		"ERLANG_VERSION major only": {created, map[string]string{"ERLANG_VERSION": "28"}, "elixir:1.18.4@" + exactDigest, "1.18.4"},
	} {
		t.Run(name, func(t *testing.T) {
			choices, _ := resolveExactAt(t, published(), tc.created, nativeElixirRuntime, tc.env, nil)
			// Only Render's docs line, which names Elixir and Erlang's page,
			// may mention Erlang.
			narration, docs := strings.CutSuffix(shown(choices[0].Narration()), elixirDocs)
			if c := choices[0]; c.Image != tc.image || c.Line != tc.line || c.otp != "" || !docs || strings.Contains(narration, "Erlang") {
				t.Errorf("must keep %s %s with the image's own OTP: %+v", tc.image, tc.line, c)
			}
		})
	}
	t.Run("explicit ERLANG_VERSION", func(t *testing.T) {
		fake := published()
		fake.envByImage["erlang:28.0.3.0@"+exactDigest] = fakeExactMetadata{[]string{"OTP_VERSION=28.0.3"}, commonOTPSyntheticHistory("bookworm")}
		choices, _ := resolveExactAt(t, fake, created, nativeElixirRuntime, map[string]string{"ERLANG_VERSION": "28.0"}, nil)
		if got, want := shown(choices[0].Narration()), "==> Using Elixir 1.18.4 on Erlang/OTP 28.0.3 (from ERLANG_VERSION=28.0)"+elixirDocs; got != want {
			t.Errorf("narration %q, want %q", got, want)
		}
	})

	// The default is a default, like an exact default patch: when no image
	// carries it, or the registry cannot be read, the build keeps the Elixir
	// default and says why, instead of failing.
	t.Run("no image carries it", func(t *testing.T) {
		choices, _ := resolveExactAt(t, &fakeExact{digests: map[string]string{"elixir:1.18.4": exactDigest}}, created, nativeElixirRuntime, nil, nil)
		want := "==> Using Elixir version 1.18.4 (default)" + elixirDocs + "\n==> Could not install the default Erlang/OTP 28.0.2 (no published image carries it); using the Erlang/OTP of the Elixir image"
		if c := choices[0]; c.Image != "elixir:1.18.4@"+exactDigest || c.otp != "" || shown(c.Narration()) != want {
			t.Errorf("fallback %+v narration %q, want %q", c, shown(c.Narration()), want)
		}
	})
	t.Run("registry failure", func(t *testing.T) {
		choice, err := ResolveNativeVersionAt(created, nativeElixirRuntime, nil, files(nil), packageJSON)
		must(t, err)
		got := pinDefaultErlang(t.Context(), &fakeExact{err: errors.New("registry answered 500")}, choice)
		want := "==> Using Elixir version " + reviewed.lineVersion + " (default)" + elixirDocs + "\n" +
			"==> Could not install the default Erlang/OTP 28.0.2 (look up Elixir 1.18.4 on Erlang/OTP 28.0.2.0: registry answered 500); using the Erlang/OTP of the Elixir image\n" +
			"==> Could not look up elixir:1.18.4 (registry answered 500); using the reviewed Elixir 1.18 image"
		if got.Image != reviewed.Image || got.erlang != "" || shown(got.Narration()) != want {
			t.Errorf("fallback %+v narration %q, want %q", got, shown(got.Narration()), want)
		}
	})
}

func TestDockerHubResolverListsTagsByPrefix(t *testing.T) {
	all := []string{"1.19.4-erlang-28.3-debian-trixie-1", "1.19.5-erlang-27.3-debian-trixie-1", "11.19.5-erlang-28.3-debian-trixie-1",
		"1.19.5-erlang-28.3-debian-trixie-1", "1.19.5-erlang-28.3.3-debian-trixie-1", "1.19.5-erlang-28.5-debian-trixie-1",
		"1.19.5-erlang-29.0-debian-trixie-1", "1.19.6-erlang-28.3-debian-trixie-1"}
	pages := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/repositories/hexpm/elixir/tags", func(w http.ResponseWriter, r *http.Request) {
		pages++
		// The Hub's substring search, two results a page.
		var matches []map[string]string
		for _, tag := range all {
			if strings.Contains(tag, r.URL.Query().Get("name")) {
				matches = append(matches, map[string]string{"name": tag})
			}
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		lo, hi := min(2*(page-1), len(matches)), min(2*page, len(matches))
		var next *string
		if hi < len(matches) {
			n := "https://hub.example/next"
			next = &n
		}
		body, _ := json.Marshal(map[string]any{"count": len(matches), "next": next, "results": matches[lo:hi]})
		_, _ = w.Write(body)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d := &DockerHubResolver{Client: srv.Client(), HubURL: srv.URL}

	got, err := d.ListTags(context.Background(), hexpmElixirRepo, "1.19.5-erlang-28.")
	want := []string{"1.19.5-erlang-28.3-debian-trixie-1", "1.19.5-erlang-28.3.3-debian-trixie-1", "1.19.5-erlang-28.5-debian-trixie-1"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("got %v %v, want %v", got, err, want)
	}
	if pages != 2 {
		t.Errorf("read %d pages, want 2", pages)
	}
	if _, err := d.ListTags(context.Background(), "evil/elixir", "1."); err == nil {
		t.Error("an unreviewed repository was listed")
	}
}

// w8/373: a partial request's candidates come from the official erlang
// repository, newest first, and each is probed for hexpm's builds of that one
// release. An old Elixir and OTP minor has more hexpm builds than a listing
// reads, so no hexpm search spans more than one release; a release hexpm never
// built is passed over, and a failed listing stays a lookup failure.
func TestExactErlangCandidatesComeFromTheOfficialRepository(t *testing.T) {
	const official = "28.0."
	const missing = "1.18.4-erlang-28.0.3-debian-"
	const built = "1.18.4-erlang-28.0.2-debian-"
	const selected = built + "bookworm-20260824"
	for _, failed := range []string{"", official, built} {
		name := "complete lookup"
		if failed != "" {
			name = "listing failure " + failed
		}
		t.Run(name, func(t *testing.T) {
			queries := make(chan string, 32)
			heads := make(chan string, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var names []string
				switch r.URL.Path {
				case "/v2/repositories/library/erlang/tags", "/v2/repositories/hexpm/elixir/tags":
					search := r.URL.Query().Get("name")
					queries <- search
					if search == failed {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					switch search {
					case official:
						names = []string{"28.0", "28.0.0", "28.0.0.0", "28.0.2", "28.0.2.0", "28.0.2.0-slim", "28.0.3.0", "28.0.10.0-alpine"}
					case missing:
					case built:
						names = []string{built + "bookworm-20260610", selected, selected + "-slim", built + "trixie-20260610"}
					default:
						t.Errorf("unexpected tag query %q", search)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					results := make([]map[string]string, 0, len(names))
					for _, tag := range names {
						results = append(results, map[string]string{"name": tag})
					}
					must(t, json.NewEncoder(w).Encode(map[string]any{"results": results}))
				case "/token":
					must(t, json.NewEncoder(w).Encode(map[string]string{"token": "public-test-token"}))
				case "/v2/library/erlang/manifests/28.0.2.0":
					heads <- r.URL.Path
					w.Header().Set("Docker-Content-Digest", exactDigest)
				case "/v2/hexpm/elixir/manifests/" + selected:
					heads <- r.URL.Path
					w.Header().Set("Docker-Content-Digest", otherDigest)
				default:
					t.Errorf("unexpected metadata request %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(server.Close)
			resolver := &DockerHubResolver{Client: server.Client(), AuthURL: server.URL + "/token", RegistryURL: server.URL, HubURL: server.URL}
			choice, err := pinErlangVersion(t.Context(), resolver, NativeChoice{Exact: "1.18.4", erlang: "28.0", Source: "ERLANG_VERSION=28.0"})
			if failed != "" {
				var unsupported *UnsupportedVersionError
				if err == nil || errors.As(err, &unsupported) || !strings.Contains(err.Error(), "tag listing answered 503") || choice.Image != "" || len(heads) != 0 {
					t.Fatalf("a failed listing did not remain a lookup failure: choice=%+v err=%v", choice, err)
				}
				return
			}
			must(t, err)
			if choice.otp != "28.0.2" || choice.Image != "erlang:28.0.2.0@"+exactDigest || choice.elixirImage != hexpmElixirRepo+":"+selected+"@"+otherDigest {
				t.Fatalf("lookup changed the newest shared OTP/build selection: %+v", choice)
			}
			if len(queries) != 3 || <-queries != official || <-queries != missing || <-queries != built || len(heads) != 2 {
				t.Fatal("lookup searched beyond one release at a time or bypassed the selected image digest lookups")
			}
		})
	}
}

func TestExactErlangPreservesMaintenanceSelection(t *testing.T) {
	catalog := []string{
		"1.18.4-erlang-28.0-debian-bookworm-20260610",
		"1.18.4-erlang-28.0-debian-bookworm-20260824",
		"1.18.4-erlang-28.0.0.1-debian-bookworm-20260824",
		"1.18.4-erlang-28.0.2-debian-bookworm-20260824",
		"1.18.4-erlang-28.0.2.1-debian-bookworm-20260610",
		"1.18.4-erlang-28.0.2.1-debian-bookworm-20260824",
		"1.18.4-erlang-28.0.2.2-debian-bookworm-20260824", // no official image
		"1.18.4-erlang-28.0-rc.1-alpine-3.22",
		"1.18.4-erlang-28.0-alpine-3.22",
		"1.18.4-erlang-28.0-ubuntu-noble-20260824",
		"1.18.4-erlang-28.01-debian-bookworm-20260824",
		"1.18.4-erlang-28.0.2.1-debian-bookworm-20261031-slim",
		"11.18.4-erlang-28.0.2.1-debian-bookworm-20261031",
	}
	for request, want := range map[string]string{"28.0": "28.0.2.1", "28.0.0": "28.0.0.1", "28.0.2": "28.0.2.1", "28.0.0.0": "28.0"} {
		t.Run(request, func(t *testing.T) {
			fake := &fakeExact{digests: map[string]string{}}
			for _, tag := range catalog {
				fake.digests[hexpmElixirRepo+":"+tag] = otherDigest
			}
			for _, release := range []string{"28.0", "28.0.0.1", "28.0.2", "28.0.2.1"} {
				fake.digests[erlangRepo+":"+otpTag(release)] = exactDigest
			}
			choice, err := pinErlangVersion(t.Context(), fake, NativeChoice{Exact: "1.18.4", erlang: request})
			must(t, err)
			if choice.otp != want || choice.Image != erlangRepo+":"+otpTag(want)+"@"+exactDigest || !strings.Contains(choice.elixirImage, "-debian-bookworm-20260824@") {
				t.Fatalf("changed matching release/newest date/shared-image intersection for %s: %+v", request, choice)
			}
		})
	}
}
