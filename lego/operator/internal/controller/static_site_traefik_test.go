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

package controller

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/staticserver"
)

// probeSite serves its objects at any host.
type probeSite map[string]staticserver.Object

func (p probeSite) Resolve(string) (staticserver.Site, bool) {
	return staticserver.Site{AppID: "probe", Revision: "rev-1"}, true
}

func (p probeSite) Get(_ context.Context, key string) (staticserver.Object, error) {
	if obj, ok := p[strings.TrimPrefix(key, "probe/rev-1/")]; ok {
		return obj, nil
	}
	return staticserver.Object{}, staticserver.ErrNotFound
}

// TestStaticSiteOnTheDeployedTraefik is w8/363's actual-proxy gate: the
// static server behind the deployed Traefik release, with the edge plugin
// interpreted on the entrypoint and a static site's chain on the router.
// Traefik's forwarder asks for gzip on a GET with no Accept-Encoding, yet that
// client gets an icon identity under its strong tag with Content-Length and
// Accept-Ranges, as on Render. br and gzip clients get the weak tag, and a
// script still reaches a header-less client through the forwarder's decode,
// under the weak tag Render sends it. Opt-in: needs Docker.
func TestStaticSiteOnTheDeployedTraefik(t *testing.T) {
	if os.Getenv("BEX_TRAEFIK_INTEGRATION") == "" {
		t.Skip("set BEX_TRAEFIK_INTEGRATION=1 to run the static site chain on the deployed Traefik release")
	}
	ico := append([]byte{0, 0, 1, 0}, make([]byte, 15000)...)
	js := []byte(strings.Repeat("console.log('static');\n", 512))
	site := probeSite{
		"favicon.ico": {Body: ico, ContentType: "image/vnd.microsoft.icon"},
		"app.js":      {Body: js, ContentType: "text/javascript"},
	}
	port := startEdgeOrigin(t, staticserver.New(site, site, 1<<20))
	dockerArgs, traefikArgs := edgePluginArgs(t, clientIPHeadersPlugin)
	base, _ := runEdgeTraefik(t, port, edgeTraefikConfig{
		label: "static-site", chain: []string{"probe-static-site"}, readyPath: "/favicon.ico",
		middlewares: tenantEdgeMiddlewares(map[string]any{"probe-static-site": staticSiteSpec}),
		dockerArgs:  dockerArgs,
		traefikArgs: append(traefikArgs, "--entrypoints.web.http.middlewares=edge-headers@file,client-ip@file"),
	})
	// The client's own transport must not ask for gzip either.
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	get := func(path, offer string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if offer != "" {
			req.Header.Set("Accept-Encoding", offer)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp, body
	}

	resp, body := get("/favicon.ico", "")
	strong := resp.Header.Get("ETag")
	if strings.HasPrefix(strong, "W/") || resp.Header.Get("Content-Encoding") != "" ||
		resp.Header.Get("Content-Length") != strconv.Itoa(len(ico)) || resp.Header.Get("Accept-Ranges") != "bytes" || len(body) != len(ico) {
		t.Fatalf("icon with no Accept-Encoding: %v (%d bytes), want identity with a strong tag, Content-Length and Accept-Ranges", resp.Header, len(body))
	}
	for _, offer := range []string{"br", "gzip"} {
		if resp, _ := get("/favicon.ico", offer); resp.Header.Get("Content-Encoding") != offer || resp.Header.Get("ETag") != "W/"+strong {
			t.Errorf("icon offered %s: CE %q ETag %q, want %s under W/%s", offer, resp.Header.Get("Content-Encoding"), resp.Header.Get("ETag"), offer, strong)
		}
	}
	resp, body = get("/app.js", "")
	if !strings.HasPrefix(resp.Header.Get("ETag"), "W/") || resp.Header.Get("Content-Encoding") != "" || string(body) != string(js) {
		t.Errorf("script with no Accept-Encoding: %v (%d bytes), want the decoded identity bytes under the weak tag", resp.Header, len(body))
	}
}
