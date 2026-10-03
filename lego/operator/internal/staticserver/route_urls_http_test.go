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

package staticserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func routeURLRequest(t *testing.T, server *httptest.Server, method, target string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, server.URL+target, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = testHost
	cl := server.Client()
	cl.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := cl.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return response, string(body)
}

func TestRouteURLRedirectPreservesCapturedFilename(t *testing.T) {
	for _, destination := range []string{"/docs/:splat", "/docs/*"} {
		t.Run(destination, func(t *testing.T) {
			objects := map[string]Object{}
			cases := []struct{ encoded, decoded string }{
				{"a%3Fb.yaml", "a?b.yaml"}, {"a%23b.yaml", "a#b.yaml"}, {"a%25b.yaml", "a%b.yaml"}, {"a%20b.yaml", "a b.yaml"}, {"caf%C3%A9.yaml", "café.yaml"}, {"nested/a%3Fb.yaml", "nested/a?b.yaml"}, {"a%253Fb.yaml", "a%3Fb.yaml"}, {"plain.yaml", "plain.yaml"},
			}
			for _, tc := range cases {
				objects[key("docs/"+tc.decoded)] = Object{Body: []byte(tc.decoded), ContentType: "application/yaml"}
			}
			h, _ := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/jump/*", Destination: destination}}, Headers: []appv1alpha1.StaticHeader{{Path: "/jump/*", Name: "X-Original", Value: "yes"}}}, objects)
			server := httptest.NewServer(h)
			defer server.Close()
			for _, tc := range cases {
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					t.Run(method+tc.encoded, func(t *testing.T) {
						response, _ := routeURLRequest(t, server, method, "/jump/"+tc.encoded)
						want := "/docs/" + tc.encoded
						if response.StatusCode != 301 || response.Header.Get("Location") != want {
							t.Fatalf("redirect=%d Location=%q want%q", response.StatusCode, response.Header.Get("Location"), want)
						}
						if response.Header.Get("X-Original") != "yes" {
							t.Fatal("redirect lost request headers")
						}
						target, err := url.Parse(response.Header.Get("Location"))
						if err != nil {
							t.Fatal(err)
						}
						if target.Path != "/docs/"+tc.decoded || target.RawQuery != "" || target.Fragment != "" {
							t.Fatalf("captured filename became URL syntax: %#v", target)
						}
						next, body := routeURLRequest(t, server, method, target.RequestURI())
						if next.StatusCode != 200 {
							t.Fatalf("followed redirect=%d", next.StatusCode)
						}
						if method == http.MethodGet && body != tc.decoded {
							t.Fatalf("wrong object body %q want%q", body, tc.decoded)
						}
						if method == http.MethodHead && body != "" {
							t.Fatal("HEAD sent body")
						}
					})
				}
			}
		})
	}
}

func TestRouteURLRewriteResolvesConfiguredURLPath(t *testing.T) {
	for _, tc := range []struct{ destination, object string }{
		{"/render.yaml?from=rule#frag", "render.yaml"}, {"/%72ender.yaml", "render.yaml"}, {"/render.yaml", "render.yaml"},
		{"/docs/a%3Fb.yaml?from=rule#frag", "docs/a?b.yaml"}, {"/docs/a%23b.yaml", "docs/a#b.yaml"}, {"/docs/a%25b.yaml", "docs/a%b.yaml"}, {"/docs/a%20b.yaml", "docs/a b.yaml"}, {"/docs/caf%C3%A9.yaml", "docs/café.yaml"}, {"/docs/a%253Fb.yaml", "docs/a%3Fb.yaml"},
		{"/docs/a%2Fb.yaml", "docs/a/b.yaml"},
		{"/docs/%2E%2E/render.yaml", "render.yaml"},
		{"/docs/a%255Cb.yaml", "docs/a%5Cb.yaml"},
	} {
		t.Run(tc.destination, func(t *testing.T) {
			h, origin := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/rewrite.yaml", Destination: tc.destination}}, Headers: []appv1alpha1.StaticHeader{{Path: "/rewrite.yaml", Name: "X-Original", Value: "yes"}, {Path: "/docs/*", Name: "X-Destination", Value: "no"}}}, map[string]Object{key(tc.object): {Body: []byte("original " + tc.object), ContentType: "application/yaml"}})
			server := httptest.NewServer(h)
			defer server.Close()
			parsed, err := url.Parse(tc.destination)
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				direct, directBody := routeURLRequest(t, server, method, parsed.RequestURI())
				response, body := routeURLRequest(t, server, method, "/rewrite.yaml?incoming=one")
				if direct.StatusCode != 200 || response.StatusCode != 200 || body != directBody {
					t.Fatalf("%s direct=%d %q rewrite=%d %q", method, direct.StatusCode, directBody, response.StatusCode, body)
				}
				if response.Header.Get("Location") != "" || response.Header.Get("X-Original") != "yes" || response.Header.Get("X-Destination") != "" {
					t.Fatalf("rewrite headers=%v", response.Header)
				}
				if response.Header.Get("Content-Type") != "application/yaml" {
					t.Fatal("rewrite changed object MIME")
				}
			}
			if origin.gets[key(tc.object)] != 1 {
				t.Fatal("direct and rewrite did not share published object cache")
			}
		})
	}
}
