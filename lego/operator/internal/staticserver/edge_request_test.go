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
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"testing"
)

// hexText is n hex digits from a fixed seed: text br shrinks only by about
// half, so 16 KiB of it has a br body past maxRangedBrotliBytes.
func hexText(n int) []byte {
	const digits = "0123456789abcdef"
	rng := rand.New(rand.NewPCG(362, 362))
	out := make([]byte, n)
	for i := range out {
		out[i] = digits[rng.IntN(len(digits))]
	}
	return out
}

// TestRangesAndIfMatchFollowRendersEdge (captured on Render 2026-11-21 on
// seven sites, w8/362): a Range gets a 206 on a file with one representation
// in every coding, and on a compressible file's br body up to 2,641 bytes,
// each with Accept-Ranges. A compressible file's identity and gzip bodies, and
// its br body from 4,573 bytes up, answer with 200, the whole body and no
// Accept-Ranges, If-Range or not. If-Match is ignored on every file and
// coding: a stale tag gets 200 and the body.
func TestRangesAndIfMatchFollowRendersEdge(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("small.js"): {Body: repeated("console.log('small');\n"), ContentType: "text/javascript"},
		key("large.js"): {Body: hexText(16 << 10), ContentType: "text/javascript"},
		key("logo.png"): {Body: repeated("\x89PNG\r\n"), ContentType: "image/png"},
	})
	cases := []struct {
		path, offer string
		ranged      bool
	}{
		{"/small.js", "br", true},
		{"/small.js", "gzip", false},
		{"/small.js", "", false},
		{"/large.js", "br", false},
		{"/large.js", "gzip", false},
		{"/large.js", "", false},
		{"/logo.png", "br", true},
		{"/logo.png", "gzip", true},
		{"/logo.png", "", true},
	}
	for _, tc := range cases {
		full := doWith(h, http.MethodGet, tc.path, offerHeader(tc.offer))
		label := fmt.Sprintf("%s offered %q", tc.path, tc.offer)
		if tc.path == "/large.js" && tc.offer == "br" && full.Body.Len() <= maxRangedBrotliBytes {
			t.Fatalf("%s: br body %d bytes, want past %d", label, full.Body.Len(), maxRangedBrotliBytes)
		}
		wantAcceptRanges := ""
		if tc.ranged {
			wantAcceptRanges = "bytes"
		}
		if got := full.Header().Get("Accept-Ranges"); full.Code != http.StatusOK || got != wantAcceptRanges {
			t.Errorf("%s => %d Accept-Ranges %q, want 200 with %q", label, full.Code, got, wantAcceptRanges)
		}
		tag := full.Header().Get("ETag")
		for _, req := range []struct {
			name   string
			kv     []string
			ranges bool // asks for bytes 0-9
		}{
			{"Range", []string{"Range", "bytes=0-9"}, true},
			{"Range + If-Range", []string{"Range", "bytes=0-9", "If-Range", tag}, true},
			{"stale If-Match + Range", []string{"If-Match", `"stale"`, "Range", "bytes=0-9"}, true},
			{"stale If-Match", []string{"If-Match", `"stale"`}, false},
			{"current If-Match", []string{"If-Match", tag}, false},
			{"unsatisfiable Range", []string{"Range", "bytes=999999-"}, false},
		} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				rec := doWith(h, method, tc.path, offerHeader(tc.offer, req.kv...))
				l := method + " " + label + " " + req.name
				wantBody := full.Body.Bytes()
				switch {
				case req.name == "unsatisfiable Range" && tc.ranged:
					if rec.Code != http.StatusRequestedRangeNotSatisfiable {
						t.Errorf("%s => %d, want 416", l, rec.Code)
					}
					continue
				case req.ranges && tc.ranged:
					wantRange := "bytes 0-9/" + strconv.Itoa(full.Body.Len())
					if rec.Code != http.StatusPartialContent || rec.Header().Get("Content-Range") != wantRange ||
						rec.Header().Get("Content-Encoding") != full.Header().Get("Content-Encoding") {
						t.Errorf("%s => %d Content-Range %q CE %q, want 206 %q", l, rec.Code, rec.Header().Get("Content-Range"), rec.Header().Get("Content-Encoding"), wantRange)
						continue
					}
					wantBody = wantBody[:10]
				default:
					if rec.Code != http.StatusOK || rec.Header().Get("Content-Range") != "" ||
						rec.Header().Get("Content-Length") != strconv.Itoa(full.Body.Len()) ||
						rec.Header().Get("Content-Encoding") != full.Header().Get("Content-Encoding") ||
						rec.Header().Get("ETag") != tag || rec.Header().Get("Accept-Ranges") != wantAcceptRanges {
						t.Errorf("%s => %d Content-Range %q Content-Length %q CE %q ETag %q Accept-Ranges %q, want the 200 a plain GET gets",
							l, rec.Code, rec.Header().Get("Content-Range"), rec.Header().Get("Content-Length"),
							rec.Header().Get("Content-Encoding"), rec.Header().Get("ETag"), rec.Header().Get("Accept-Ranges"))
						continue
					}
				}
				if method == http.MethodHead {
					wantBody = nil
				}
				if !bytes.Equal(rec.Body.Bytes(), wantBody) {
					t.Errorf("%s sent %d body bytes, want %d", l, rec.Body.Len(), len(wantBody))
				}
			}
		}
	}
}

// TestIfMatchNeverHidesIfNoneMatch: with If-Match ignored, a matching
// If-None-Match still gets its bodyless 304 under the strong tag (w8/361),
// whatever If-Match says.
func TestIfMatchNeverHidesIfNoneMatch(t *testing.T) {
	h := conditionalSite(t)
	for _, offer := range []string{"br", "gzip", ""} {
		rec := doWith(h, http.MethodGet, "/version.txt", offerHeader(offer, "If-Match", `"stale"`, "If-None-Match", savedETag))
		if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 || rec.Header().Get("ETag") != savedETag {
			t.Errorf("offered %q: stale If-Match + matching If-None-Match => %d (%d bytes, ETag %q), want bodyless 304 under %s",
				offer, rec.Code, rec.Body.Len(), rec.Header().Get("ETag"), savedETag)
		}
	}
}

// TestEdgeRequestLeavesTheCallersRequest: dropping the headers Render's edge
// ignores never changes the request the caller still holds.
func TestEdgeRequestLeavesTheCallersRequest(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("If-Match", `"stale"`)
	r.Header.Set("Range", "bytes=0-9")
	got := edgeRequest(r, false)
	if got.Header.Get("If-Match") != "" || got.Header.Get("Range") != "" {
		t.Errorf("edge request kept If-Match %q Range %q", got.Header.Get("If-Match"), got.Header.Get("Range"))
	}
	if r.Header.Get("If-Match") == "" || r.Header.Get("Range") == "" {
		t.Error("edgeRequest changed the caller's headers")
	}
}
