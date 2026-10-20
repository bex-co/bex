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
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// swappableResolver serves one host from whichever Site is current, as the
// cluster resolver does across a delete and recreate.
type swappableResolver struct{ site *Site }

func (r swappableResolver) Resolve(host string) (Site, bool) { return *r.site, host == testHost }

// TestRecreatedSiteNeverServesItsPredecessorsCache (w8/146): a site deleted
// and recreated under its name publishes to the same "<ws>/<name>/rev-1/" key.
// The successor must read its own bytes, encodings and misses, never what the
// cache kept for the predecessor.
func TestRecreatedSiteNeverServesItsPredecessorsCache(t *testing.T) {
	const prefix = "tea-aaaaaaaaaaaaaaaaaaaa/site/rev-1/"
	old := bytes.Repeat([]byte("<p>withdrawn content</p>"), 200)
	origin := newFakeOrigin(map[string]Object{prefix + "index.html": {Body: old, ContentType: "text/html"}})
	site := Site{AppID: "site", Revision: "rev-1", Prefix: prefix, Incarnation: "uid-old"}
	h := New(swappableResolver{site: &site}, origin, 1<<20)
	br := http.Header{"Accept-Encoding": {"br"}}

	if rec := do(h, http.MethodGet, "/"); !bytes.Equal(rec.Body.Bytes(), old) {
		t.Fatalf("setup: predecessor body = %q", rec.Body.String())
	}
	doWith(h, http.MethodGet, "/", br)
	if rec := do(h, http.MethodGet, "/missing.html"); rec.Code != http.StatusNotFound {
		t.Fatalf("setup: predecessor miss = %d", rec.Code)
	}

	oldEncoded, _ := encodeBrotli(old)

	// Delete + recreate: same key, new object, new incarnation.
	fresh := bytes.Repeat([]byte("<p>the new site</p>"), 200)
	origin.objs[prefix+"index.html"] = Object{Body: fresh, ContentType: "text/html"}
	origin.objs[prefix+"missing.html"] = Object{Body: []byte("now present"), ContentType: "text/html"}
	site.Incarnation = "uid-new"

	if rec := do(h, http.MethodGet, "/"); !bytes.Equal(rec.Body.Bytes(), fresh) {
		t.Fatalf("successor body = %.40q…, want its own page", rec.Body.String())
	}
	if rec := doWith(h, http.MethodGet, "/", br); rec.Header().Get("Content-Encoding") != "br" || bytes.Equal(rec.Body.Bytes(), oldEncoded) {
		t.Fatalf("successor br variant = %q encoding, want its own encoding", rec.Header().Get("Content-Encoding"))
	}
	if rec := do(h, http.MethodGet, "/missing.html"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "now present") {
		t.Fatalf("successor reads the predecessor's cached miss: %d %q", rec.Code, rec.Body.String())
	}
}
