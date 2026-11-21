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
	"compress/gzip"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/andybalholm/brotli"
	"golang.org/x/net/http/httpguts"
	"golang.org/x/sync/semaphore"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m79: Render serves eligible static text/JS/CSS br-encoded; every one of
// these used to be served identity with no Content-Encoding.

var brAssets = []struct {
	name, path, ctype, text string
}{
	{"text", "/payload.txt", "text/plain", "ordinary static content\n"},
	{"javascript", "/app.js", "text/javascript", "console.log('ordinary asset');\n"},
	{"css", "/app.css", "text/css", ".example { color: blue; padding: 1px; }\n"},
}

func repeated(text string) []byte { return []byte(strings.Repeat(text, 4096)) }

// decodeBrotli independently decodes a br body.
func decodeBrotli(t *testing.T, encoded []byte) []byte {
	t.Helper()
	decoded, err := io.ReadAll(brotli.NewReader(bytes.NewReader(encoded)))
	if err != nil {
		t.Fatalf("decode br body: %v", err)
	}
	return decoded
}

func hasVaryAcceptEncoding(h http.Header) bool {
	return httpguts.HeaderValuesContainsToken(h.Values("Vary"), "Accept-Encoding")
}

// decodeBody independently decodes a body in coding.
func decodeBody(t *testing.T, coding string, encoded []byte) []byte {
	t.Helper()
	if coding == codingBrotli {
		return decodeBrotli(t, encoded)
	}
	zr, err := gzip.NewReader(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("gzip header: %v", err)
	}
	decoded, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decode gzip body: %v", err)
	}
	return decoded
}

// assertEncoded checks a coding response decodes to want, varies by
// Accept-Encoding, is honestly framed whatever its size against identity's
// (w8/359), and carries a strong tag on br and a weak one on gzip, as Render's
// edge sends them (w8/361).
func assertEncoded(t *testing.T, label string, rec *httptest.ResponseRecorder, coding string, want []byte) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%s => %d, want 200", label, rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != coding {
		t.Fatalf("%s Content-Encoding = %q, want %s (%d bytes sent)", label, got, coding, rec.Body.Len())
	}
	if !hasVaryAcceptEncoding(rec.Header()) {
		t.Fatalf("%s Vary = %q, want Accept-Encoding", label, rec.Header().Values("Vary"))
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(rec.Body.Len()) {
		t.Fatalf("%s Content-Length = %q, want the encoded length %d", label, got, rec.Body.Len())
	}
	if !bytes.Equal(decodeBody(t, coding, rec.Body.Bytes()), want) {
		t.Fatalf("%s %s body does not decode to the published bytes", label, coding)
	}
	wantTag := `W/"`
	if coding == codingBrotli {
		wantTag = `"`
	}
	if tag := rec.Header().Get("Etag"); !strings.HasPrefix(tag, wantTag) {
		t.Fatalf("%s Etag = %q, want a %s… tag on %s", label, tag, wantTag, coding)
	}
}

// assertBrotli is assertEncoded for br on a compressible body, which must
// shrink.
func assertBrotli(t *testing.T, label string, rec *httptest.ResponseRecorder, want []byte) {
	t.Helper()
	assertEncoded(t, label, rec, codingBrotli, want)
	if rec.Body.Len() >= len(want) {
		t.Fatalf("%s sent %d br bytes for %d identity bytes, want smaller", label, rec.Body.Len(), len(want))
	}
}

// assertIdentity checks an identity response carries exactly the published bytes.
func assertIdentity(t *testing.T, label string, rec *httptest.ResponseRecorder, want []byte) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%s => %d, want 200", label, rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("%s Content-Encoding = %q, want identity", label, got)
	}
	if !bytes.Equal(rec.Body.Bytes(), want) {
		t.Fatalf("%s body changed: %d bytes, want %d", label, rec.Body.Len(), len(want))
	}
}

func brHeader(offer string) http.Header { return http.Header{"Accept-Encoding": {offer}} }

// offerHeader is an Accept-Encoding offer, none when offer is empty, plus the
// given header name/value pairs.
func offerHeader(offer string, kv ...string) http.Header {
	hdr := http.Header{}
	if offer != "" {
		hdr.Set("Accept-Encoding", offer)
	}
	for i := 0; i < len(kv); i += 2 {
		hdr.Set(kv[i], kv[i+1])
	}
	return hdr
}

// holdSiteEncode takes the test site's only encode slot, as a concurrent encode
// would; the returned func gives it back.
func holdSiteEncode(t *testing.T, h *Handler) func() {
	t.Helper()
	if !h.encodes.acquire(appID) {
		t.Fatal("encode slot unexpectedly busy")
	}
	return func() { h.encodes.release(appID) }
}

func TestAutomaticBrotliForEligibleAssets(t *testing.T) {
	for _, tc := range brAssets {
		body := repeated(tc.text)
		h, _ := newTestHandler(t, Site{}, map[string]Object{
			key(strings.TrimPrefix(tc.path, "/")): {Body: body, ContentType: tc.ctype},
		})
		for _, offer := range []string{"br", "gzip, deflate, br, zstd", "br;q=1, gzip;q=0.5", "BR", "*"} {
			assertBrotli(t, tc.name+" offered "+offer, doWith(h, http.MethodGet, tc.path, brHeader(offer)), body)
		}
		for _, offer := range []string{"", "identity", "deflate", "zstd", "br;q=0", "br;q=0, gzip;q=0", "*;q=0", "br;q=bogus", "br;q=2"} {
			rec := doWith(h, http.MethodGet, tc.path, brHeader(offer))
			assertIdentity(t, tc.name+" offered "+strconv.Quote(offer), rec, body)
			if !hasVaryAcceptEncoding(rec.Header()) {
				t.Fatalf("%s identity response for an encodable asset lacks Vary: Accept-Encoding", tc.name)
			}
		}
	}
}

// assertGzip is assertEncoded for gzip on a compressible body, which must
// shrink.
func assertGzip(t *testing.T, label string, rec *httptest.ResponseRecorder, want []byte) {
	t.Helper()
	assertEncoded(t, label, rec, codingGzip, want)
	if rec.Body.Len() >= len(want) {
		t.Fatalf("%s sent %d gzip bytes for %d identity bytes, want smaller", label, rec.Body.Len(), len(want))
	}
}

// TestGzipForGzipOnlyClients (w8/m132, Render serves gzip to a client that
// does not offer br): gzip is chosen when br is not accepted, br stays
// preferred at equal weight, the br and gzip variants are cached apart, and a
// gzip response revalidates against its own encoded bytes. A Range gets the
// whole gzip body, as on Render's edge (captured 2026-11-21, w8/362).
func TestGzipForGzipOnlyClients(t *testing.T) {
	body := repeated("console.log('hello');\n")
	h, _ := newTestHandler(t, Site{}, map[string]Object{key("big.js"): {Body: body, ContentType: "application/javascript"}})
	for _, offer := range []string{"gzip", "gzip, deflate", "GZIP", "gzip;q=0.5, br;q=0.1", "br;q=0, gzip", "*;q=0.2, br;q=0"} {
		assertGzip(t, "offered "+offer, doWith(h, http.MethodGet, "/big.js", brHeader(offer)), body)
	}
	for _, offer := range []string{"gzip, br", "br, gzip;q=0.5", "*"} {
		assertBrotli(t, "offered "+offer, doWith(h, http.MethodGet, "/big.js", brHeader(offer)), body)
	}
	for _, offer := range []string{"gzip;q=0", "deflate", "zstd", "identity"} {
		assertIdentity(t, "offered "+offer, doWith(h, http.MethodGet, "/big.js", brHeader(offer)), body)
	}
	for _, coding := range []string{codingBrotli, codingGzip} {
		if v, ok := h.cache.get(variantKey(coding, cachedKey("big.js"))); !ok || v.negative {
			t.Fatalf("%s variant not cached apart from the other coding (hit %v)", coding, ok)
		}
	}

	first := doWith(h, http.MethodGet, "/big.js", brHeader("gzip"))
	etag := first.Header().Get("Etag")
	if rec := doWith(h, http.MethodGet, "/big.js", http.Header{"Accept-Encoding": {"gzip"}, "If-None-Match": {etag}}); rec.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match %s => %d, want 304", etag, rec.Code)
	}
	rec := doWith(h, http.MethodGet, "/big.js", http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-9"}})
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), first.Body.Bytes()) || rec.Header().Get("Content-Encoding") != codingGzip {
		t.Fatalf("Range on gzip => %d %d bytes CE %q, want 200 with the whole gzip body", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Encoding"))
	}
	head := doWith(h, http.MethodHead, "/big.js", brHeader("gzip"))
	if head.Header().Get("Content-Encoding") != "gzip" || head.Body.Len() != 0 {
		t.Fatalf("HEAD gzip = %q with %d body bytes", head.Header().Get("Content-Encoding"), head.Body.Len())
	}
}

// w8/359, captured on Render 2026-11-20 with Accept-Encoding "gzip, br": a
// 67-byte robots.txt went out as 62 br bytes, a 27-byte version.json as 29
// and the bare 404 as 14, each with Vary: Accept-Encoding. A gzip-only client
// got robots.txt gzipped (78 bytes), version.json identity, and every 404
// identity. bex skipped bodies under 1 KiB and any encode that saved under
// 1/16, and sent the bare 404 identity, all without Vary.
const (
	renderRobots  = "# https://www.robotstxt.org/robotstxt.html\nUser-agent: *\nDisallow:\n"
	renderVersion = `{"version":"1763672406123"}`
)

func TestSmallBodiesAreEncodedAsOnRender(t *testing.T) {
	if len(renderRobots) < minGzipBytes || len(renderVersion) >= minGzipBytes {
		t.Fatalf("fixtures are %d and %d bytes, want either side of minGzipBytes %d", len(renderRobots), len(renderVersion), minGzipBytes)
	}
	tiny := []byte("ok\n")
	floor := []byte(strings.Repeat("a", minGzipBytes))
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("robots.txt"):        {Body: []byte(renderRobots), ContentType: "text/plain"},
		key("_app/version.json"): {Body: []byte(renderVersion), ContentType: "application/json"},
		key("tiny.txt"):          {Body: tiny, ContentType: "text/plain"},
		key("floor.txt"):         {Body: floor, ContentType: "text/plain"},
		key("under.txt"):         {Body: floor[1:], ContentType: "text/plain"},
	})
	for path, body := range map[string][]byte{
		"/robots.txt": []byte(renderRobots), "/_app/version.json": []byte(renderVersion),
		"/tiny.txt": tiny, "/floor.txt": floor, "/under.txt": floor[1:],
	} {
		assertEncoded(t, path+" offered gzip, br", doWith(h, http.MethodGet, path, brHeader("gzip, br")), codingBrotli, body)
		rec := doWith(h, http.MethodGet, path, nil)
		assertIdentity(t, path+" identity", rec, body)
		if !hasVaryAcceptEncoding(rec.Header()) {
			t.Fatalf("%s identity lacks Vary: Accept-Encoding", path)
		}
		gz := doWith(h, http.MethodGet, path, brHeader("gzip"))
		if len(body) >= minGzipBytes {
			assertEncoded(t, path+" offered gzip", gz, codingGzip, body)
			continue
		}
		assertIdentity(t, path+" offered gzip", gz, body)
		if !hasVaryAcceptEncoding(gz.Header()) {
			t.Fatalf("%s gzip-client identity lacks Vary: Accept-Encoding", path)
		}
	}
	// No savings test: the encoded body may be the larger one, as Render's are.
	if rec := doWith(h, http.MethodGet, "/tiny.txt", brHeader("br")); rec.Body.Len() <= len(tiny) {
		t.Fatalf("a %d-byte body went out as %d br bytes, want the larger br body Render sends", len(tiny), rec.Body.Len())
	}
}

// TestSmallBodyValidatorsHold: the w4/341 rules hold below the old 1 KiB
// floor too. The br variant carries the strong tag and identity its weak form
// (w8/361), and either tag revalidates either representation with a bodyless
// 304 that keeps Vary and carries the strong tag.
func TestSmallBodyValidatorsHold(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("robots.txt"): {Body: []byte(renderRobots), ContentType: "text/plain"},
	})
	br := doWith(h, http.MethodGet, "/robots.txt", brHeader("br"))
	identity := doWith(h, http.MethodGet, "/robots.txt", nil)
	strong, weak := br.Header().Get("Etag"), identity.Header().Get("Etag")
	if !strings.HasPrefix(strong, `"`) || weak != "W/"+strong {
		t.Fatalf("br ETag %q, identity ETag %q, want a strong tag and its weak form", strong, weak)
	}
	for _, offer := range []string{"br", "", "gzip"} {
		for _, tag := range []string{strong, weak} {
			rec := doWith(h, http.MethodGet, "/robots.txt", http.Header{"Accept-Encoding": {offer}, "If-None-Match": {tag}})
			if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 || rec.Header().Get("Content-Encoding") != "" ||
				!hasVaryAcceptEncoding(rec.Header()) || rec.Header().Get("Etag") != strong {
				t.Fatalf("offer %q If-None-Match %s => %d %d bytes CE %q Vary %v ETag %q, want a bodyless 304 with Vary and %s",
					offer, tag, rec.Code, rec.Body.Len(), rec.Header().Get("Content-Encoding"), rec.Header().Values("Vary"), rec.Header().Get("Etag"), strong)
			}
		}
	}
}

// TestNotFoundIsBrotliEncodedForBrClients: Render sends the bare 404 and a
// site's 404.html br to a br client and identity to a gzip-only one, both
// with Vary: Accept-Encoding (w8/359).
func TestNotFoundIsBrotliEncodedForBrClients(t *testing.T) {
	page := repeated("<p>custom 404</p>\n")
	bare, _ := newTestHandler(t, Site{NoSPAFallback: true}, nil)
	withPage, _ := newTestHandler(t, Site{NoSPAFallback: true}, map[string]Object{key("404.html"): {Body: page, ContentType: "text/html"}})
	for _, tc := range []struct {
		name string
		h    *Handler
		want []byte
	}{{"bare 404", bare, []byte("Not Found\n")}, {"404.html", withPage, page}} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := doWith(tc.h, method, "/qa-nope-x", brHeader("gzip, br"))
			hdr := rec.Header()
			if rec.Code != http.StatusNotFound || hdr.Get("Content-Encoding") != "br" || !hasVaryAcceptEncoding(hdr) {
				t.Fatalf("%s %s offered br => %d CE %q Vary %v, want a br 404 with Vary", method, tc.name, rec.Code, hdr.Get("Content-Encoding"), hdr.Values("Vary"))
			}
			if method == http.MethodHead {
				if rec.Body.Len() != 0 || hdr.Get("Content-Length") == "" {
					t.Fatalf("HEAD %s => %d body bytes, Content-Length %q", tc.name, rec.Body.Len(), hdr.Get("Content-Length"))
				}
				continue
			}
			if hdr.Get("Content-Length") != strconv.Itoa(rec.Body.Len()) || !bytes.Equal(decodeBrotli(t, rec.Body.Bytes()), tc.want) {
				t.Fatalf("%s br body (%d bytes, Content-Length %q) does not decode to the page", tc.name, rec.Body.Len(), hdr.Get("Content-Length"))
			}
			for _, offer := range []string{"gzip", ""} {
				rec := doWith(tc.h, method, "/qa-nope-x", brHeader(offer))
				if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Encoding") != "" || !bytes.Equal(rec.Body.Bytes(), tc.want) || !hasVaryAcceptEncoding(rec.Header()) {
					t.Fatalf("%s offered %q => %d CE %q, want the identity 404 with Vary", tc.name, offer, rec.Code, rec.Header().Get("Content-Encoding"))
				}
			}
		}
	}
	if rec := doWith(bare, http.MethodGet, "/qa-nope-x", brHeader("br")); rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
		rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("bare br 404 headers = %v, want text/plain with nosniff", rec.Header())
	}
}

// TestBrotliOnEveryServingCaller covers the three serveObject callers: a direct
// object (and a directory index), a rewritten target, and the SPA fallback.
// The content type comes from the served object while header rules still
// match the visitor's path.
func TestBrotliOnEveryServingCaller(t *testing.T) {
	page := repeated("<p>index</p>\n")
	docs := repeated("<p>docs</p>\n")
	h, _ := newTestHandler(t, Site{
		Routes:  []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/alias.js", Destination: "/app.js"}},
		Headers: []appv1alpha1.StaticHeader{{Path: "/client/*", Name: "X-Route", Value: "spa"}},
	}, map[string]Object{
		key("index.html"):      {Body: page},
		key("docs/index.html"): {Body: docs},
		key("app.js"):          {Body: repeated("console.log(1);\n")}, // type inferred from the extension
	})
	assertBrotli(t, "root index", doWith(h, http.MethodGet, "/", brHeader("br")), page)
	assertBrotli(t, "directory index", doWith(h, http.MethodGet, "/docs/", brHeader("br")), docs)
	assertBrotli(t, "direct", doWith(h, http.MethodGet, "/app.js", brHeader("br")), repeated("console.log(1);\n"))
	assertBrotli(t, "rewrite", doWith(h, http.MethodGet, "/alias.js", brHeader("br")), repeated("console.log(1);\n"))
	spa := doWith(h, http.MethodGet, "/client/route", brHeader("br"))
	assertBrotli(t, "spa fallback", spa, page)
	if spa.Header().Get("X-Route") != "spa" || !strings.HasPrefix(spa.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("spa fallback headers = %v", spa.Header())
	}

	// A miss is the bare 404, which Render also sends br (w8/359).
	for _, p := range []string{"/missing.css", "/client/../../missing.js"} {
		rec := doWith(h, http.MethodGet, p, brHeader("br"))
		if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Encoding") != "br" ||
			string(decodeBrotli(t, rec.Body.Bytes())) != "Not Found\n" {
			t.Fatalf("GET %s => %d %q, want the bare 404 in br", p, rec.Code, rec.Header().Get("Content-Encoding"))
		}
	}
}

// TestBrotliIneligibleResponsesStayExact: authored precompressed bodies,
// no-transform and non-text media are served exactly as published, and
// still vary by Accept-Encoding as every Render file does (w8/359).
func TestBrotliIneligibleResponsesStayExact(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(repeated("precompressed asset\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), repeated("pixels")...)
	h, _ := newTestHandler(t, Site{Headers: []appv1alpha1.StaticHeader{
		{Path: "/pre/*", Name: "Content-Encoding", Value: "gzip"},
		{Path: "/raw/*", Name: "Cache-Control", Value: "public, max-age=60, No-Transform"},
	}}, map[string]Object{
		key("pre/app.js"):   {Body: gz.Bytes(), ContentType: "text/javascript"},
		key("raw/app.css"):  {Body: repeated("a{}\n"), ContentType: "text/css"},
		key("img/logo.png"): {Body: png, ContentType: "image/png"},
	})
	cases := []struct {
		path, encoding string
		want           []byte
	}{
		{"/pre/app.js", "gzip", gz.Bytes()},
		{"/raw/app.css", "", repeated("a{}\n")},
		{"/img/logo.png", "", png},
	}
	for _, tc := range cases {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			// "gzip" alone must not re-encode either (w8/m132).
			if rec := doWith(h, method, tc.path, brHeader("gzip")); rec.Header().Get("Content-Encoding") != tc.encoding || !hasVaryAcceptEncoding(rec.Header()) {
				t.Fatalf("%s %s offered gzip => Content-Encoding %q Vary %v, want the fixed representation", method, tc.path, rec.Header().Get("Content-Encoding"), rec.Header().Values("Vary"))
			}
			rec := doWith(h, method, tc.path, brHeader("br, gzip"))
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != tc.encoding {
				t.Fatalf("%s %s => %d Content-Encoding %q, want 200 %q", method, tc.path, rec.Code, rec.Header().Get("Content-Encoding"), tc.encoding)
			}
			if !hasVaryAcceptEncoding(rec.Header()) {
				t.Fatalf("%s %s lacks Vary: Accept-Encoding on a fixed representation", method, tc.path)
			}
			want := tc.want
			if method == http.MethodHead {
				want = nil
			}
			if !bytes.Equal(rec.Body.Bytes(), want) {
				t.Fatalf("%s %s body changed", method, tc.path)
			}
		}
	}
}

// TestBrotliKeepsAuthoredVary merges Accept-Encoding into an authored Vary and
// leaves "Vary: *" alone.
func TestBrotliKeepsAuthoredVary(t *testing.T) {
	h, _ := newTestHandler(t, Site{Headers: []appv1alpha1.StaticHeader{
		{Path: "/a.css", Name: "Vary", Value: "Origin"},
		{Path: "/b.css", Name: "Vary", Value: "*"},
		{Path: "/c.css", Name: "Vary", Value: "origin, accept-encoding"},
	}}, map[string]Object{
		key("a.css"): {Body: repeated("a{}\n"), ContentType: "text/css"},
		key("b.css"): {Body: repeated("b{}\n"), ContentType: "text/css"},
		key("c.css"): {Body: repeated("c{}\n"), ContentType: "text/css"},
	})
	for path, want := range map[string]string{
		"/a.css": "Origin,Accept-Encoding",
		"/b.css": "*",
		"/c.css": "origin, accept-encoding",
	} {
		rec := doWith(h, http.MethodGet, path, brHeader("br"))
		if got := strings.Join(rec.Header().Values("Vary"), ","); got != want || rec.Header().Get("Content-Encoding") != "br" {
			t.Fatalf("GET %s Vary = %q Content-Encoding %q, want %q br", path, got, rec.Header().Get("Content-Encoding"), want)
		}
	}
}

// TestIncompressibleBodiesAreStillEncoded: Render applies no savings test
// (w8/359), so an eligible type whose bytes do not shrink is served br and
// gzip all the same, from a variant cached so later requests need no encode
// slot.
func TestIncompressibleBodiesAreStillEncoded(t *testing.T) {
	noise := make([]byte, 64<<10)
	if _, err := rand.Read(noise); err != nil {
		t.Fatal(err)
	}
	h, _ := newTestHandler(t, Site{}, map[string]Object{key("noise.txt"): {Body: noise, ContentType: "text/plain"}})
	first := doWith(h, http.MethodGet, "/noise.txt", brHeader("br"))
	assertEncoded(t, "incompressible br", first, codingBrotli, noise)
	if first.Body.Len() < len(noise) {
		t.Fatalf("random bytes shrank to %d br bytes from %d", first.Body.Len(), len(noise))
	}
	assertEncoded(t, "incompressible gzip", doWith(h, http.MethodGet, "/noise.txt", brHeader("gzip")), codingGzip, noise)
	if v, ok := h.cache.get(brotliVariantKey(cachedKey("noise.txt"))); !ok || v.negative {
		t.Fatalf("incompressible br variant cache = %v %v, want the cached variant", ok, v.negative)
	}
	defer holdSiteEncode(t, h)() // a further encode attempt would have no slot
	assertEncoded(t, "incompressible again", doWith(h, http.MethodGet, "/noise.txt", brHeader("br")), codingBrotli, noise)
}

// TestBrotliVariantCachedOncePerKey: the first br request encodes and caches
// the variant against the site budget; later requests are served from it even
// when no encode slot is free.
func TestBrotliVariantCachedOncePerKey(t *testing.T) {
	body := repeated("console.log('cached');\n")
	h, origin := newTestHandler(t, Site{}, map[string]Object{key("app.js"): {Body: body, ContentType: "text/javascript"}})
	first := doWith(h, http.MethodGet, "/app.js", brHeader("br"))
	assertBrotli(t, "first", first, body)

	vkey := brotliVariantKey(cachedKey("app.js"))
	v, ok := h.cache.get(vkey)
	if !ok || !bytes.Equal(v.Body, first.Body.Bytes()) {
		t.Fatal("encoded variant was not cached")
	}
	h.cache.mu.Lock()
	charged, used := h.cache.siteUsed[appID], entryCharge(cachedKey("app.js"), Object{Body: body, ContentType: "text/javascript", etag: contentETag(body, "text/javascript")})+entryCharge(vkey, v)
	h.cache.mu.Unlock()
	if charged != used {
		t.Fatalf("site cache charge = %d, want identity + variant = %d", charged, used)
	}
	if cap(v.Body) > len(v.Body)+64 {
		t.Fatalf("cached variant retains capacity %d for %d bytes", cap(v.Body), len(v.Body))
	}

	defer holdSiteEncode(t, h)()
	assertBrotli(t, "cached", doWith(h, http.MethodGet, "/app.js", brHeader("br")), body)
	if origin.gets[key("app.js")] != 1 {
		t.Fatalf("origin gets = %d, want 1", origin.gets[key("app.js")])
	}
}

// TestBrotliDegradesToIdentityUnderBudget: with every encode slot taken, or the
// live-body budget too small for an encode's working set, an eligible request
// is served identity (still 200, still Vary) — never shed — and the busy
// outcome is not cached, so a later request gets br.
func TestBrotliDegradesToIdentityUnderBudget(t *testing.T) {
	body := repeated(".busy { color: red; }\n")
	h, _ := newTestHandler(t, Site{}, map[string]Object{key("app.css"): {Body: body, ContentType: "text/css"}})

	release := holdSiteEncode(t, h)
	rec := doWith(h, http.MethodGet, "/app.css", brHeader("br"))
	assertIdentity(t, "no encode slot", rec, body)
	if !hasVaryAcceptEncoding(rec.Header()) {
		t.Fatal("busy fallback lacks Vary: Accept-Encoding")
	}
	release()

	h.liveBodies = semaphore.NewWeighted(int64(len(body)) + 1) // identity fits, encode working set does not
	assertIdentity(t, "no encode memory", doWith(h, http.MethodGet, "/app.css", brHeader("br")), body)
	if _, cached := h.cache.get(brotliVariantKey(cachedKey("app.css"))); cached {
		t.Fatal("a capacity fallback was cached as the variant outcome")
	}

	h.liveBodies = semaphore.NewWeighted(defaultMaxLiveBodyBytes)
	assertBrotli(t, "after capacity returns", doWith(h, http.MethodGet, "/app.css", brHeader("br")), body)

	// Every lease is returned: the whole budget is acquirable again.
	if !h.liveBodies.TryAcquire(defaultMaxLiveBodyBytes) || !h.encodes.acquire(appID) || !h.encodes.acquire("other-site") {
		t.Fatal("br serving leaked live-body or encode capacity")
	}
}

// TestBrotliEncodeSlotsAreFairAcrossSites: another site's in-flight encode does
// not take this site's slot, while the global bound still caps the total.
func TestBrotliEncodeSlotsAreFairAcrossSites(t *testing.T) {
	body := repeated("console.log('fair');\n")
	h, _ := newTestHandler(t, Site{}, map[string]Object{key("app.js"): {Body: body, ContentType: "text/javascript"}})
	if !h.encodes.acquire("busy-site") {
		t.Fatal("first site slot")
	}
	if h.encodes.acquire("busy-site") {
		t.Fatal("one site took two encode slots")
	}
	assertBrotli(t, "beside another site's encode", doWith(h, http.MethodGet, "/app.js", brHeader("br")), body)
	if !h.encodes.acquire("second-site") || h.encodes.acquire("third-site") {
		t.Fatalf("global encode bound is not %d", defaultMaxConcurrentEncodes)
	}
}

// TestBrotliSkipsObjectsPastTheEncodeBound: an eligible object larger than
// maxBrotliBytes is served identity rather than spending CPU/memory on it.
func TestBrotliSkipsObjectsPastTheEncodeBound(t *testing.T) {
	big := bytes.Repeat([]byte("a"), maxBrotliBytes+1)
	h, _ := newTestHandler(t, Site{}, map[string]Object{key("big.txt"): {Body: big, ContentType: "text/plain"}})
	rec := doWith(h, http.MethodGet, "/big.txt", brHeader("br"))
	assertIdentity(t, "oversize", rec, big)
	if !hasVaryAcceptEncoding(rec.Header()) {
		t.Fatal("oversize identity lacks Vary: Accept-Encoding")
	}
}

// TestBrotliConcurrentRequestsStayCorrect: a burst of br and identity requests
// for one asset each get exact, honestly labeled bytes (run under -race), from
// one origin fetch and one cached variant.
func TestBrotliConcurrentRequestsStayCorrect(t *testing.T) {
	body := repeated("console.log('burst');\n")
	origin := &blockingOrigin{release: make(chan struct{}), entered: make(chan string, 64), body: body}
	close(origin.release)
	h := newConcurrentHandler(origin, nil)
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := range 32 {
		wg.Go(func() {
			offer := "identity"
			if i%2 == 0 {
				offer = "br"
			}
			rec := doWith(h, http.MethodGet, "/burst.html", brHeader(offer))
			// A br offer may be served identity while every encode slot is
			// busy; either way the bytes must be exact and labeled honestly.
			got := rec.Body.Bytes()
			switch rec.Header().Get("Content-Encoding") {
			case "br":
				d, err := io.ReadAll(brotli.NewReader(bytes.NewReader(got)))
				if err != nil {
					errs <- err
					return
				}
				got = d
				if offer != "br" {
					errs <- fmt.Errorf("%s request was served br", offer)
				}
			case "": // identity
			default:
				errs <- fmt.Errorf("unexpected coding %q", rec.Header().Get("Content-Encoding"))
			}
			if rec.Code != http.StatusOK || !bytes.Equal(got, body) {
				errs <- fmt.Errorf("%s request => %d, %d bytes", offer, rec.Code, len(got))
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	assertBrotli(t, "after burst", doWith(h, http.MethodGet, "/burst.html", brHeader("br")), body)
	if n := origin.total.Load(); n != 1 {
		t.Fatalf("origin fetches = %d, want 1", n)
	}
	if _, ok := h.cache.get(brotliVariantKey(cachedKey("burst.html"))); !ok {
		t.Fatal("burst left no cached variant")
	}
}

// TestBrotliVariantIsRevisionScoped: a republish (new revision prefix) serves
// the new revision's bytes br-encoded, never the previous revision's cached
// variant, and rolling back serves the old variant again.
func TestBrotliVariantIsRevisionScoped(t *testing.T) {
	v1, v2 := repeated("console.log('rev-1');\n"), repeated("console.log('rev-2');\n")
	origin := newFakeOrigin(map[string]Object{
		"ws/site/rev-1/app.js": {Body: v1, ContentType: "text/javascript"},
		"ws/site/rev-2/app.js": {Body: v2, ContentType: "text/javascript"},
	})
	site := Site{AppID: appID, Prefix: "ws/site/rev-1/"}
	h := New(publishResolver{site: &site}, origin, 1<<20)
	assertBrotli(t, "rev-1", doWith(h, http.MethodGet, "/app.js", brHeader("br")), v1)

	site.Prefix = "ws/site/rev-2/"
	assertBrotli(t, "rev-2", doWith(h, http.MethodGet, "/app.js", brHeader("br")), v2)
	assertIdentity(t, "rev-2 identity", doWith(h, http.MethodGet, "/app.js", nil), v2)

	site.Prefix = "ws/site/rev-1/"
	assertBrotli(t, "rollback", doWith(h, http.MethodGet, "/app.js", brHeader("br")), v1)
}

// TestBrotliVariantKeyIsNotAddressable: every object key a request can derive
// starts with the site prefix, so none can collide with a variant key (and be
// served a variant's bytes as identity) — even for an empty legacy site.
func TestBrotliVariantKeyIsNotAddressable(t *testing.T) {
	for _, site := range []Site{{}, {AppID: appID, Revision: rev}, {Prefix: "ws/site/rev-1/"}} {
		for _, p := range []string{"/", "/\x00br\x00app.js", normalizePath("/%00br%00x")} {
			if k := site.keyFor(p); strings.HasPrefix(k, brotliVariantKey("")) {
				t.Fatalf("keyFor(%+v, %q) = %q collides with the variant namespace", site, p, k)
			}
		}
	}
}

// TestBrotliPreconditionsAndHead: the br representation carries the saved
// ETag as it was authored, strong (w8/361), so revalidation yields a bodyless
// 304 that claims no coding. If-Match is ignored, as on Render's edge
// (captured 2026-11-21, w8/362), so any tag gets the body in either coding.
// HEAD reports the same representation metadata as GET. Identity sends the
// saved tag's weak form.
func TestBrotliPreconditionsAndHead(t *testing.T) {
	body := repeated(versionBytes)
	h, _ := newTestHandler(t, Site{Headers: []appv1alpha1.StaticHeader{{Path: "/*", Name: "ETag", Value: savedETag}}},
		map[string]Object{key("version.txt"): {Body: body, ContentType: "text/plain"}})

	get := doWith(h, http.MethodGet, "/version.txt", offerHeader("br"))
	assertBrotli(t, "GET", get, body)
	if got := get.Header().Get("ETag"); got != savedETag {
		t.Fatalf("br ETag = %q, want %q", got, savedETag)
	}
	head := doWith(h, http.MethodHead, "/version.txt", offerHeader("br"))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Encoding") != "br" ||
		head.Header().Get("Content-Length") != get.Header().Get("Content-Length") || head.Header().Get("ETag") != savedETag {
		t.Fatalf("HEAD br = %d %d bytes %v, want GET's metadata without a body", head.Code, head.Body.Len(), head.Header())
	}

	for _, inm := range []string{savedETag, "W/" + savedETag, "*"} {
		rec := doWith(h, http.MethodGet, "/version.txt", offerHeader("br", "If-None-Match", inm))
		if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 || rec.Header().Get("Content-Encoding") != "" {
			t.Fatalf("br If-None-Match %s => %d %d bytes CE %q, want bodyless 304", inm, rec.Code, rec.Body.Len(), rec.Header().Get("Content-Encoding"))
		}
		if !hasVaryAcceptEncoding(rec.Header()) {
			t.Fatalf("br 304 lacks Vary: %v", rec.Header())
		}
	}
	for _, im := range []string{savedETag, `"other"`, "W/" + savedETag} {
		assertBrotli(t, "br If-Match "+im, doWith(h, http.MethodGet, "/version.txt", offerHeader("br", "If-Match", im)), body)
	}

	identity := doWith(h, http.MethodGet, "/version.txt", offerHeader("identity"))
	assertIdentity(t, "identity", identity, body)
	if identity.Header().Get("ETag") != "W/"+savedETag {
		t.Fatalf("identity ETag = %q, want W/%s", identity.Header().Get("ETag"), savedETag)
	}
	for _, im := range []string{savedETag, `"other"`} {
		assertIdentity(t, "identity If-Match "+im, doWith(h, http.MethodGet, "/version.txt", offerHeader("identity", "If-Match", im)), body)
	}
}

// TestBrotliFramingOverHTTP exercises real connections: full br responses are
// Content-Length framed, a range addresses the small br body's bytes and an
// unsatisfiable one is a plain 416, an identity range gets the whole framed
// identity body (w8/362), and HEAD/304 carry no body bytes.
func TestBrotliFramingOverHTTP(t *testing.T) {
	body := repeated("console.log('framed');\n")
	h, _ := newTestHandler(t, Site{}, map[string]Object{key("app.js"): {Body: body, ContentType: "text/javascript"}})
	server := httptest.NewServer(h)
	defer server.Close()

	roundTrip := func(method string, header http.Header) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+"/app.js", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = testHost
		req.Header = header
		res, err := http.DefaultTransport.RoundTrip(req) // never decodes br
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		got, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return res, got
	}

	res, encoded := roundTrip(http.MethodGet, brHeader("br"))
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Encoding") != "br" ||
		res.ContentLength != int64(len(encoded)) || len(res.TransferEncoding) != 0 {
		t.Fatalf("br GET => %d CE %q length %d TE %v for %d bytes", res.StatusCode, res.Header.Get("Content-Encoding"), res.ContentLength, res.TransferEncoding, len(encoded))
	}
	if !bytes.Equal(decodeBrotli(t, encoded), body) || len(encoded) >= len(body) {
		t.Fatalf("br GET body: %d encoded bytes do not decode to the %d published bytes", len(encoded), len(body))
	}

	res, got := roundTrip(http.MethodHead, brHeader("br"))
	if res.StatusCode != http.StatusOK || len(got) != 0 || res.ContentLength != int64(len(encoded)) {
		t.Fatalf("br HEAD => %d %d bytes length %d, want %d", res.StatusCode, len(got), res.ContentLength, len(encoded))
	}

	res, got = roundTrip(http.MethodGet, http.Header{"Accept-Encoding": {"br"}, "Range": {"bytes=0-9"}})
	wantRange := fmt.Sprintf("bytes 0-9/%d", len(encoded))
	if res.StatusCode != http.StatusPartialContent || res.Header.Get("Content-Encoding") != "br" ||
		res.Header.Get("Content-Range") != wantRange || !bytes.Equal(got, encoded[:10]) {
		t.Fatalf("br range => %d CE %q %q %d bytes, want 206 %q of the encoded bytes", res.StatusCode, res.Header.Get("Content-Encoding"), res.Header.Get("Content-Range"), len(got), wantRange)
	}

	res, got = roundTrip(http.MethodGet, http.Header{"Accept-Encoding": {"identity"}, "Range": {"bytes=0-9"}})
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Encoding") != "" || res.ContentLength != int64(len(body)) ||
		res.Header.Get("Content-Range") != "" || !bytes.Equal(got, body) {
		t.Fatalf("identity range => %d %q %q length %d, want 200 with the whole identity body", res.StatusCode, res.Header.Get("Content-Encoding"), res.Header.Get("Content-Range"), res.ContentLength)
	}

	res, _ = roundTrip(http.MethodGet, http.Header{"Accept-Encoding": {"br"}, "Range": {fmt.Sprintf("bytes=%d-", len(body)*2)}})
	if res.StatusCode != http.StatusRequestedRangeNotSatisfiable || res.Header.Get("Content-Encoding") != "" {
		t.Fatalf("unsatisfiable br range => %d CE %q, want 416 with no coding", res.StatusCode, res.Header.Get("Content-Encoding"))
	}

	res, got = roundTrip(http.MethodGet, http.Header{"Accept-Encoding": {"br"}, "If-None-Match": {"*"}})
	if res.StatusCode != http.StatusNotModified || len(got) != 0 {
		t.Fatalf("br If-None-Match * => %d %d bytes, want bodyless 304", res.StatusCode, len(got))
	}
}

func TestNegotiateCoding(t *testing.T) {
	for offer, want := range map[string]string{
		"br": "br", " br ": "br", "Br": "br", "gzip, br": "br", "br;q=0.001": "br", "br; q=1.0": "br",
		"br;level=1;q=0.5": "br", "*": "br", "gzip;q=0.5, br;q=0.4": "gzip", "gzip, br;q=0.9": "gzip",
		"gzip": "gzip", "GZIP": "gzip", "x-gzip": "gzip", "gzip, deflate": "gzip", "gzip;q=0.001": "gzip",
		"br;q=0, gzip": "gzip", "br;q=0, *": "gzip", "gzip, *;q=0.1": "gzip", "deflate, gzip;q=0.8": "gzip",
		"": "", "identity": "", "deflate": "", "zstd": "", "br;q=0": "", "br;Q=0.000": "", "gzip;q=0": "",
		"*;q=0": "", "br;q=": "", "br;q=NaN": "", "br;q=-1": "", "gzip;q=NaN": "", "brotli": "",
		"br;q=0, gzip;q=0": "", "gzip;q=0, *": "br", "gzip;q=0, deflate": "",
	} {
		if got := negotiateCoding(http.Header{"Accept-Encoding": {offer}}); got != want {
			t.Errorf("negotiateCoding(%q) = %q, want %q", offer, got, want)
		}
	}
	if got := negotiateCoding(http.Header{"Accept-Encoding": {"deflate", "gzip"}}); got != "gzip" {
		t.Errorf("gzip offered on a second Accept-Encoding line = %q, want gzip", got)
	}
	if got := negotiateCoding(http.Header{"Accept-Encoding": {"gzip", "br"}}); got != "br" {
		t.Errorf("br offered on a second Accept-Encoding line = %q, want br", got)
	}
}

func TestCompressibleType(t *testing.T) {
	for ct, want := range map[string]bool{
		"text/html; charset=utf-8": true, "TEXT/CSS": true, "text/javascript": true, "application/javascript": true,
		"application/json": true, "application/manifest+json": true, "image/svg+xml": true, "application/wasm": true,
		"font/ttf": true, "application/xml": true,
		"image/png": false, "image/webp": false, "font/woff2": false, "application/zip": false,
		"application/octet-stream": false, "video/mp4": false, "": false,
	} {
		if got := compressibleType(ct); got != want {
			t.Errorf("compressibleType(%q) = %v, want %v", ct, got, want)
		}
	}
}

// TestBrotliEncoderStateBound measures one encode's total allocation at the
// largest admitted input and holds it within the working set leased from the
// live-body budget (encodeLease).
func TestBrotliEncoderStateBound(t *testing.T) {
	src := make([]byte, maxBrotliBytes)
	if _, err := rand.Read(src[:maxBrotliBytes/2]); err != nil { // half noise, half zeros
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	if _, ok := encodeBrotli(src); !ok {
		t.Fatal("half-zero input did not compress")
	}
	runtime.ReadMemStats(&after)
	allocated := int64(after.TotalAlloc - before.TotalAlloc)
	lease := encodeLease(len(src))
	if allocated > lease {
		t.Fatalf("encode allocated %d MiB, beyond its %d MiB live-body lease", allocated>>20, lease>>20)
	}
	t.Logf("encode of %d MiB allocated %d MiB (lease %d MiB)", len(src)>>20, allocated>>20, lease>>20)
}
