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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := erlangFake(tc.erlangTags...)
			choices, dockerfile := resolveExact(t, fake, "elixir", tc.env, nil)
			for _, want := range []string{
				"\nFROM erlang:" + tc.erlang + "@" + exactDigest + "\n",
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
			if got := choices[0].Narration(); got != tc.narration {
				t.Errorf("narration %q, want %q", got, tc.narration)
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
