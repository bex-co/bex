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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeExact struct {
	digests map[string]string
	err     error
	asked   []string
}

func (f *fakeExact) ResolveOfficialTag(_ context.Context, repo, tag string) (string, bool, error) {
	f.asked = append(f.asked, repo+":"+tag)
	d, ok := f.digests[repo+":"+tag]
	return d, ok, f.err
}

const exactDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000001"

func resolveExact(t *testing.T, fake *fakeExact, runtime string, env, files map[string]string) ([]NativeChoice, string) {
	t.Helper()
	o := nativeOptions()
	o.Runtime = runtime
	bundle, dockerfile, src := writeResolveFixture(t, o, env, files)
	choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, runtime, bundle, dockerfile, src)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(dockerfile)
	return choices, string(out)
}

func TestExactVersionPinsTheRequestedPatch(t *testing.T) {
	cases := []struct {
		name, runtime string
		env           map[string]string
		files         map[string]string
		tag           string
	}{
		{"python env", "python", map[string]string{"PYTHON_VERSION": "3.11.9"}, nil, "python:3.11.9-bookworm"},
		{"node env", "node", map[string]string{"NODE_VERSION": "18.18.0"}, nil, "node:18.18.0-bookworm"},
		{"nvmrc", "node", nil, map[string]string{".nvmrc": "v20.1.0\n"}, "node:20.1.0-bookworm"},
		{"python file", "python", nil, map[string]string{".python-version": "3.12.1\n"}, "python:3.12.1-bookworm"},
		{"ruby file", "ruby", nil, map[string]string{".ruby-version": "3.3.1\n"}, "ruby:3.3.1-bookworm"},
		{"elixir env", "elixir", map[string]string{"ELIXIR_VERSION": "1.17.0"}, nil, "elixir:1.17.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeExact{digests: map[string]string{tc.tag: exactDigest}}
			choices, dockerfile := resolveExact(t, fake, tc.runtime, tc.env, tc.files)
			if len(fake.asked) != 1 || fake.asked[0] != tc.tag {
				t.Fatalf("asked %v, want [%s]", fake.asked, tc.tag)
			}
			if !strings.Contains(dockerfile, "FROM "+tc.tag+"@"+exactDigest) {
				t.Errorf("Dockerfile does not build on the exact image:\n%s", dockerfile)
			}
			_, version, _ := strings.Cut(strings.SplitN(tc.tag, "-", 2)[0], ":")
			if choices[0].Line != version || !strings.Contains(choices[0].Narration(), version) {
				t.Errorf("narration %q does not name %s", choices[0].Narration(), version)
			}
		})
	}
}

func TestExactVersionLeavesLineRequestsAlone(t *testing.T) {
	for name, tc := range map[string]struct {
		runtime string
		env     map[string]string
		files   map[string]string
	}{
		"line only":       {"python", nil, map[string]string{".python-version": "3.11\n"}},
		"reviewed patch":  {"python", map[string]string{"PYTHON_VERSION": "3.11.17"}, nil},
		"node major":      {"node", map[string]string{"NODE_VERSION": "18"}, nil},
		"node range":      {"node", map[string]string{"NODE_VERSION": ">=18 <19"}, nil},
		"no signal":       {"python", nil, nil},
		"elixir with otp": {"elixir", map[string]string{"ELIXIR_VERSION": "1.17"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeExact{}
			resolveExact(t, fake, tc.runtime, tc.env, tc.files)
			if len(fake.asked) != 0 {
				t.Errorf("registry consulted: %v", fake.asked)
			}
		})
	}
}

func TestExactVersionFallsBackToTheReviewedPatch(t *testing.T) {
	for name, fake := range map[string]*fakeExact{
		"no such tag": {},
		"registry":    {err: errors.New("registry answered 500")},
	} {
		t.Run(name, func(t *testing.T) {
			choices, dockerfile := resolveExact(t, fake, "python", map[string]string{"PYTHON_VERSION": "3.11.9"}, nil)
			line, _ := nativeToolchains["python"].find("3.11")
			if !strings.Contains(dockerfile, "FROM "+line.image) {
				t.Errorf("Dockerfile left the reviewed image:\n%s", dockerfile)
			}
			narration := choices[0].Narration()
			if !strings.Contains(narration, "Using Python 3.11 ") || !strings.Contains(narration, "python:3.11.9-bookworm") {
				t.Errorf("narration %q does not explain the fallback", narration)
			}
		})
	}
}

func TestDockerHubResolver(t *testing.T) {
	var scope string
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		scope = r.URL.Query().Get("scope")
		_, _ = w.Write([]byte(`{"token":"tok"}`))
	})
	mux.HandleFunc("/v2/library/python/manifests/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.Method != http.MethodHead {
			http.Error(w, "bad", http.StatusUnauthorized)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/3.11.9-bookworm") {
			w.Header().Set("Docker-Content-Digest", exactDigest)
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d := &DockerHubResolver{Client: srv.Client(), AuthURL: srv.URL + "/token", RegistryURL: srv.URL}

	digest, ok, err := d.ResolveOfficialTag(context.Background(), "python", "3.11.9-bookworm")
	if err != nil || !ok || digest != exactDigest || scope != "repository:library/python:pull" {
		t.Fatalf("got %q %v %v scope %q", digest, ok, err, scope)
	}
	if _, ok, err := d.ResolveOfficialTag(context.Background(), "python", "3.11.99-bookworm"); ok || err != nil {
		t.Errorf("missing tag: ok=%v err=%v", ok, err)
	}
	for _, bad := range [][2]string{{"evil", "1.0"}, {"python", "../x"}, {"python", "a b"}} {
		if _, _, err := d.ResolveOfficialTag(context.Background(), bad[0], bad[1]); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}
