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

package objectgw

import (
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

func i64(n int64) *int64 { return &n }

func corsRule(origins, methods, headers, expose []string, maxAge *int64) objectstorage.CORSRule {
	return objectstorage.CORSRule{
		Allowed:       objectstorage.CORSAllowed{Origins: origins, Methods: methods, Headers: headers},
		ExposeHeaders: expose, MaxAgeSeconds: maxAge,
	}
}

func TestAnAllowedOriginGetsTheCORSHeadersAndADeniedOneDoesNot(t *testing.T) {
	e := newGW(t,
		corsRule([]string{"https://app.example"}, []string{"GET", "HEAD"}, nil, []string{"ETag", "Content-Length"}, nil),
	)
	e.put("o", []byte("x"), objectstorage.ObjectHeaders{})

	rec := e.do(http.MethodGet, "/o", "Origin", "https://app.example")
	h := rec.Header()
	if h.Get("Access-Control-Allow-Origin") != "https://app.example" || h.Get("Access-Control-Expose-Headers") != "ETag, Content-Length" {
		t.Errorf("allowed origin: %v", h)
	}
	if h.Get("Vary") != "Origin" || h.Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("Vary %q, credentials %q: the answer varies by origin and never allows credentials", h.Get("Vary"), h.Get("Access-Control-Allow-Credentials"))
	}

	// Not the allowed origin, a look-alike, another scheme, another port, no Origin.
	for _, origin := range []string{"https://evil.example", "https://app.example.evil.test", "http://app.example", "https://app.example:8443", "null", ""} {
		rec := e.do(http.MethodGet, "/o", "Origin", origin)
		if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "" || rec.Header().Get("Access-Control-Expose-Headers") != "" {
			t.Errorf("origin %q: %d with CORS headers %v; the object is served and the browser is not told it may read it", origin, rec.Code, rec.Header())
		}
		if rec.Header().Get("Vary") != "Origin" {
			t.Errorf("origin %q: Vary = %q; every answer of a bucket with rules varies by origin", origin, rec.Header().Get("Vary"))
		}
	}
	// HEAD is allowed by the rule, a 304 still carries the headers.
	etag := rec.Header().Get("ETag")
	if rec := e.do(http.MethodHead, "/o", "Origin", "https://app.example"); rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("HEAD from an allowed origin has no CORS header")
	}
	if rec := e.do(http.MethodGet, "/o", "Origin", "https://app.example", "If-None-Match", etag); rec.Code != http.StatusNotModified || rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Errorf("a 304 to an allowed origin = %d %v", rec.Code, rec.Header())
	}
}

func TestACorsWildcardOriginAnswersStarAndNoRulesAnswerNothing(t *testing.T) {
	e := newGW(t, corsRule([]string{"*"}, []string{"GET"}, nil, nil, nil))
	e.put("o", []byte("x"), objectstorage.ObjectHeaders{})
	rec := e.do(http.MethodGet, "/o", "Origin", "https://anyone.example")
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("a * rule = %q, want *", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	// A rule that allows only GET gives nothing to a HEAD.
	if rec := e.do(http.MethodHead, "/o", "Origin", "https://anyone.example"); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("a rule that allows GET only answered a HEAD with CORS headers")
	}

	bare := newGW(t)
	bare.put("o", []byte("x"), objectstorage.ObjectHeaders{})
	rec = bare.do(http.MethodGet, "/o", "Origin", "https://anyone.example")
	for name := range rec.Header() {
		if len(name) > 14 && name[:14] == "Access-Control" {
			t.Errorf("a bucket with no rules sent %s", name)
		}
	}
	if rec.Header().Get("Vary") != "" {
		t.Errorf("a bucket with no rules sent Vary %q", rec.Header().Get("Vary"))
	}
}

func TestASubdomainWildcardOriginMatchesOnlyItsSubdomains(t *testing.T) {
	e := newGW(t, corsRule([]string{"https://*.example.com"}, []string{"GET"}, nil, nil, nil))
	e.put("o", []byte("x"), objectstorage.ObjectHeaders{})
	for origin, want := range map[string]bool{
		"https://app.example.com": true, "https://a.b.example.com": true, "https://APP.example.com": true,
		"https://example.com": false, "http://app.example.com": false, "https://app.example.com.evil.test": false, "https://evil-example.com": false,
	} {
		got := e.do(http.MethodGet, "/o", "Origin", origin).Header().Get("Access-Control-Allow-Origin") != ""
		if got != want {
			t.Errorf("origin %q allowed = %v, want %v", origin, got, want)
		}
	}
}

func TestPreflight(t *testing.T) {
	e := newGW(t,
		corsRule([]string{"https://app.example"}, []string{"GET", "HEAD"}, []string{"Range", "If-None-Match"}, []string{"ETag"}, i64(600)),
		corsRule([]string{"https://other.example"}, []string{"GET"}, []string{"*"}, nil, nil),
	)
	e.put("o", []byte("x"), objectstorage.ObjectHeaders{})
	preflight := func(origin, method, headers string) *responseView {
		args := []string{"Origin", origin, "Access-Control-Request-Method", method}
		if headers != "" {
			args = append(args, "Access-Control-Request-Headers", headers)
		}
		rec := e.do(http.MethodOptions, "/o", args...)
		return &responseView{rec.Code, rec.Header(), rec.Body.String()}
	}

	t.Run("allowed", func(t *testing.T) {
		r := preflight("https://app.example", "GET", "range, if-none-match")
		if r.code != http.StatusNoContent || r.h.Get("Access-Control-Allow-Origin") != "https://app.example" ||
			r.h.Get("Access-Control-Allow-Methods") != "GET, HEAD" || r.h.Get("Access-Control-Max-Age") != "600" ||
			r.h.Get("Access-Control-Allow-Headers") != "range, if-none-match" || r.h.Get("Access-Control-Expose-Headers") != "ETag" {
			t.Errorf("allowed preflight = %d %v", r.code, r.h)
		}
		for _, v := range []string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"} {
			if !hasVary(r.h, v) {
				t.Errorf("a preflight answer must vary by %s: %v", v, r.h["Vary"])
			}
		}
	})
	t.Run("a wildcard header rule allows any header", func(t *testing.T) {
		if r := preflight("https://other.example", "GET", "X-Anything, Authorization"); r.code != http.StatusNoContent {
			t.Errorf("preflight = %d %v", r.code, r.h)
		}
	})
	t.Run("denied", func(t *testing.T) {
		for name, r := range map[string]*responseView{
			"origin":         preflight("https://evil.example", "GET", ""),
			"method":         preflight("https://app.example", "PUT", ""),
			"method of rule": preflight("https://other.example", "HEAD", ""), // second rule has GET only
			"header":         preflight("https://app.example", "GET", "Authorization"),
			"one bad header": preflight("https://app.example", "GET", "Range, X-Evil"),
		} {
			if r.code != http.StatusForbidden || r.h.Get("Access-Control-Allow-Origin") != "" || r.h.Get("Access-Control-Allow-Methods") != "" {
				t.Errorf("%s: %d %v, want 403 with no CORS header", name, r.code, r.h)
			}
		}
	})
	t.Run("an OPTIONS that is not a preflight", func(t *testing.T) {
		for _, headers := range [][]string{nil, {"Origin", "https://app.example"}, {"Access-Control-Request-Method", "GET"}} {
			rec := e.do(http.MethodOptions, "/o", headers...)
			if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") == "" {
				t.Errorf("OPTIONS with %v = %d, want 405 with Allow", headers, rec.Code)
			}
		}
	})
	t.Run("a bucket with no rules refuses every preflight", func(t *testing.T) {
		bare := newGW(t)
		rec := bare.do(http.MethodOptions, "/o", "Origin", "https://app.example", "Access-Control-Request-Method", "GET")
		if rec.Code != http.StatusForbidden || rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("preflight on a bucket with no rules = %d %v", rec.Code, rec.Header())
		}
	})
	t.Run("a preflight never reads the object", func(t *testing.T) {
		before := e.objectRequests(http.MethodGet) + e.objectRequests(http.MethodHead)
		preflight("https://app.example", "GET", "")
		if got := e.objectRequests(http.MethodGet) + e.objectRequests(http.MethodHead); got != before {
			t.Errorf("a preflight sent %d requests to Wasabi", got-before)
		}
	})
}

func TestTheFirstMatchingRuleWins(t *testing.T) {
	e := newGW(t,
		corsRule([]string{"https://app.example"}, []string{"GET"}, nil, []string{"First"}, nil),
		corsRule([]string{"https://app.example"}, []string{"GET"}, nil, []string{"Second"}, nil),
	)
	e.put("o", []byte("x"), objectstorage.ObjectHeaders{})
	if got := e.do(http.MethodGet, "/o", "Origin", "https://app.example").Header().Get("Access-Control-Expose-Headers"); got != "First" {
		t.Errorf("Expose-Headers = %q, want First", got)
	}
}

type responseView struct {
	code int
	h    http.Header
	body string
}

func hasVary(h http.Header, want string) bool {
	for _, line := range h.Values("Vary") {
		if line == want {
			return true
		}
	}
	return false
}
