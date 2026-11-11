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
	"net/http"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/565: a site that published its repository's .git (every direct publish of
// the repository root before the fix) never serves it, and no rule reaches it.
func TestGitMetadataIsNeverServed(t *testing.T) {
	objs := map[string]Object{
		key("index.html"):                  {Body: []byte("HOME"), ContentType: "text/html"},
		key("README"):                      {Body: []byte("Hello World!")},
		key(".git/HEAD"):                   {Body: []byte("7fd1a60b01f91b314f59955a4e4d4e80d8edf11d\n")},
		key(".git/config"):                 {Body: []byte("[remote \"origin\"]\n")},
		key(".git/index.html"):             {Body: []byte("index under .git")},
		key("sub/.git/config"):             {Body: []byte("[core]\n")},
		key("theme/.git"):                  {Body: []byte("gitdir: ../.git/modules/theme\n")},
		key(".well-known/assetlinks.json"): {Body: []byte("[]")},
		key(".gitignore"):                  {Body: []byte("node_modules\n")},
	}
	for _, site := range []Site{
		{},
		{NoSPAFallback: true, Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}}},
	} {
		h, origin := newTestHandler(t, site, objs)
		for _, p := range []string{"/.git/HEAD", "/.git/config", "/.git/", "/.git", "/sub/.git/config", "/theme/.git", "/x/../.git/HEAD"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				if rec := do(h, method, p); rec.Code != http.StatusNotFound {
					t.Errorf("%s %s => %d %q, want 404", method, p, rec.Code, rec.Body)
				}
			}
		}
		for _, k := range []string{".git/HEAD", ".git/config", ".git/index.html", "sub/.git/config", "theme/.git"} {
			if origin.gets[key(k)] != 0 {
				t.Errorf("%s was read from the origin %d times, want never", k, origin.gets[key(k)])
			}
		}
		for p, want := range map[string]string{"/README": "Hello World!", "/.well-known/assetlinks.json": "[]", "/.gitignore": "node_modules\n"} {
			if rec := do(h, http.MethodGet, p); rec.Code != http.StatusOK || rec.Body.String() != want {
				t.Errorf("GET %s => %d %q, want 200 %q", p, rec.Code, rec.Body, want)
			}
		}
	}

	// A rewrite whose destination is git metadata misses like any absent file.
	h, _ := newTestHandler(t, Site{
		NoSPAFallback: true,
		Routes:        []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/head", Destination: "/.git/HEAD"}},
	}, objs)
	if rec := do(h, http.MethodGet, "/head"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /head (rewritten to /.git/HEAD) => %d %q, want 404", rec.Code, rec.Body)
	}
}
