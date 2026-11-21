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
	"compress/gzip"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"golang.org/x/net/http/httpguts"
)

// Automatic Brotli (w8/m79, Render's documented static-site behavior) and gzip
// (w8/m132, what Render's live static sites also serve to a client that
// accepts gzip but not br): an eligible text-like object is served br-encoded
// to a visitor that accepts br, else gzip-encoded to one that accepts gzip.
// As on Render (w8/359), br has no size floor and neither coding a savings
// test. The encoded variant is computed once per immutable object key and
// cached beside the identity object under the same shared byte budget, so the
// CPU cost is paid per published object, not per request. Everything here
// degrades to the identity representation — never to an error — when a bound
// is reached.
const (
	// brotliQuality trades ratio for encode speed. 5 is the usual on-the-fly
	// sweet spot (most of quality 11's ratio at a fraction of the CPU).
	// Changing it, the window or the library re-encodes br bodies under their
	// unchanged strong ETag (w8/361), which only a resumed br range notices.
	brotliQuality = 5
	// brotliWindowBits is the encoder's LZ window (1 MiB). The library default
	// (4 MiB) roughly triples encoder memory for little gain on web assets.
	brotliWindowBits = 20
	// minGzipBytes is the smallest body served gzip. Render's static sites
	// gzip a 67-byte robots.txt (into 78 bytes) but send a 27-byte JSON file
	// identity (captured 2026-11-20, w8/359), matching the 48-byte gzip floor
	// of the Cloudflare edge in front of them. br has no floor: Render sends a
	// 10-byte body as 14 br bytes.
	minGzipBytes = 48
	// maxBrotliBytes bounds the input of one encode so a single request cannot
	// pin a core (or its share of the live-body budget) for long. Larger
	// eligible objects are served identity.
	maxBrotliBytes = 8 << 20 // 8 MiB
	// maxRangedBrotliBytes is the largest br body a Range addresses. Render's
	// edge sends a 206 of br bodies up to 2,641 bytes and the whole br body
	// from 4,573 bytes up (captured 2026-11-21 on seven sites, w8/362); the
	// boundary between those two is not measured.
	maxRangedBrotliBytes = 4 << 10
	// defaultMaxConcurrentEncodes bounds encode CPU on the shared server, and
	// defaultMaxSiteEncodes keeps one site from holding every slot. A request
	// that finds no slot is served identity, not queued.
	defaultMaxConcurrentEncodes = 2
	defaultMaxSiteEncodes       = 1
	// gzipLevel is the usual on-the-fly speed/ratio balance (stdlib default 6).
	gzipLevel = 6
	// brotliEncoderStateBytes is the encoder's ring buffer + hash tables at
	// brotliQuality/brotliWindowBits, with headroom over the ~14 MiB measured
	// for an 8 MiB input (TestBrotliEncoderStateBound).
	brotliEncoderStateBytes = 24 << 20 // 24 MiB
)

// encodedBound caps one encode's output for n input bytes. It sits well above
// either encoder's worst-case expansion (Brotli adds a few bytes per 16 KiB,
// gzip 5 per stored block plus an 18-byte frame), so it only stops an encode
// that has gone wrong.
func encodedBound(n int) int { return n + n/64 + 64 }

// encodeLease is the live-body budget one encode of n input bytes holds: the
// output buffer and its exact-size copy (each at most encodedBound(n)) plus
// encoder state. Leasing it from the live-body budget means encoding adds no
// memory ceiling of its own.
func encodeLease(n int) int64 { return 2*int64(encodedBound(n)) + brotliEncoderStateBytes }

// Content codings a static object can be served in, in server preference order.
const (
	codingBrotli = "br"
	codingGzip   = "gzip"
)

// errEncodeBusy reports that no encode slot or live-body capacity was free; the
// caller serves identity and does not cache the outcome.
var errEncodeBusy = errors.New("staticserver: brotli encode capacity reached")

// brotliVariantKey names the cached br variant of an object key. The leading
// NUL can never begin a key built by Site.keyFor, so no request path can name
// (and be served) a variant's bytes as if they were an identity object.
func brotliVariantKey(key string) string { return variantKey(codingBrotli, key) }

// variantKey names the cached variant of key in coding (same NUL-prefix rule).
func variantKey(coding, key string) string { return "\x00" + coding + "\x00" + key }

// compression is where Render compresses a media type, which decides its tag
// strength and its ranges (captured 2026-11-21, w8/361, w8/362, w8/363).
type compression int

const (
	// uncompressed: one representation, one strong tag, ranges in every coding
	// (PNG, woff, woff2, PDF).
	uncompressed compression = iota
	// originCompressed: Render's origin encodes it, and its edge decodes br for
	// other clients. The tag is strong on br only (encodeBody).
	originCompressed
	// edgeCompressed: Render's origin serves it identity and the Cloudflare
	// edge in front encodes it for a br or gzip client. The tag is weak on
	// those bodies only, and a Range gets the identity bytes (encodeBody).
	edgeCompressed
)

// encodingCandidate reports how the response being prepared in hdr can be
// encoded: by its media type's compression, when it is at most maxBrotliBytes
// with no authored Content-Encoding (a precompressed asset is served exactly
// as published, w4/204) and no Cache-Control: no-transform.
func encodingCandidate(hdr http.Header, size int) compression {
	if size > maxBrotliBytes {
		return uncompressed
	}
	if _, authored := hdr["Content-Encoding"]; authored {
		return uncompressed
	}
	if httpguts.HeaderValuesContainsToken(hdr.Values("Cache-Control"), "no-transform") {
		return uncompressed
	}
	return compressionOf(hdr.Get("Content-Type"))
}

// compressionOf is where Render compresses a media type. Measured read-only
// on Render static sites (w8/363, 2026-11-21): the icon types are edge
// compressed on six sites. Origin compressed are text, JS, JSON, XML and SVG
// (w8/361), font/ttf for .ttf and .otf files, vnd.ms-fontobject and wasm
// (three sites). image/bmp, application/yaml and the x-font types had no
// Render sample and stay origin compressed. Already-compressed formats
// (images, video, archives, woff, woff2, PDF) are uncompressed.
func compressionOf(contentType string) compression {
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if strings.HasPrefix(mediaType, "text/") ||
		strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml") {
		return originCompressed
	}
	switch mediaType {
	case "application/javascript", "application/x-javascript", "application/ecmascript",
		"application/json", "application/xml", "application/wasm", "application/yaml",
		"image/svg+xml", "image/bmp",
		"font/ttf", "font/otf", "application/vnd.ms-fontobject",
		"application/x-font-ttf", "application/x-font-otf":
		return originCompressed
	case "image/x-icon", "image/vnd.microsoft.icon":
		return edgeCompressed
	}
	return uncompressed
}

// negotiateCoding picks the content coding for the request's Accept-Encoding
// (RFC 9110 §12.5.3): br or gzip, whichever the client weights higher, br at
// equal weight; "" for identity. An explicit entry decides for its coding; else
// a "*" entry does. An unparseable weight counts as zero, so a malformed offer
// gets the always-acceptable identity representation.
func negotiateCoding(header http.Header) string {
	brQ, gzipQ, starQ := -1.0, -1.0, -1.0 // -1: not listed
	for _, line := range header.Values("Accept-Encoding") {
		for member := range strings.SplitSeq(line, ",") {
			coding, params, _ := strings.Cut(member, ";")
			switch strings.ToLower(strings.TrimSpace(coding)) {
			case codingBrotli:
				brQ = qvalue(params)
			case codingGzip, "x-gzip":
				gzipQ = qvalue(params)
			case "*":
				starQ = qvalue(params)
			}
		}
	}
	if brQ < 0 {
		brQ = starQ
	}
	if gzipQ < 0 {
		gzipQ = starQ
	}
	switch {
	case brQ > 0 && brQ >= gzipQ:
		return codingBrotli
	case gzipQ > 0:
		return codingGzip
	}
	return ""
}

// qvalue extracts the weight from an Accept-Encoding member's parameters:
// 1 when absent, 0 when malformed or out of range.
func qvalue(params string) float64 {
	for param := range strings.SplitSeq(params, ";") {
		name, value, _ := strings.Cut(param, "=")
		if !strings.EqualFold(strings.TrimSpace(name), "q") {
			continue
		}
		q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || !(q >= 0 && q <= 1) {
			return 0
		}
		return q
	}
	return 1
}

// varyAcceptEncoding records that the response depends on Accept-Encoding,
// keeping any authored Vary (a "*" already covers it).
func varyAcceptEncoding(hdr http.Header) {
	vary := hdr.Values("Vary")
	if httpguts.HeaderValuesContainsToken(vary, "Accept-Encoding") || httpguts.HeaderValuesContainsToken(vary, "*") {
		return
	}
	hdr.Add("Vary", "Accept-Encoding")
}

// weakenETag marks a strong ETag, generated or authored, weak.
func weakenETag(hdr http.Header) {
	if tag := hdr.Get("Etag"); strings.HasPrefix(tag, `"`) {
		hdr.Set("Etag", "W/"+tag)
	}
}

// encodeBody picks the bytes a file response sends for body, whose headers so
// far are in hdr: br for a client that prefers it, at any size; gzip, when
// gzipOK, from minGzipBytes up; else identity. Anything short of a ready
// variant within budget is identity. release returns the encoded body's
// live-body lease once the response is written.
//
// A compressed file's strong tag names one of its bodies and its other bodies
// send the weak form, as Render's edge sends them. So one strong validator
// never describes two byte sequences (RFC 9110 §8.8.1), and If-None-Match, a
// weak comparison, matches either form on any coding. An uncompressed file
// has one representation and keeps its tag.
//
//   - An origin-compressed file's strong tag belongs to its br body, and its
//     identity and gzip bodies send the weak form, as Render's edge sends them
//     (captured 2026-11-21 on three sites, w8/361). A Range addresses only a
//     br body of at most maxRangedBrotliBytes (seven sites, w8/362): the
//     identity and gzip bodies and a larger br body answer it with the whole
//     body.
//   - An edge-compressed file's strong tag belongs to its identity body, and
//     its br and gzip bodies send the weak form (six sites, w8/363). A
//     request with a Range gets the identity bytes in every coding, as does a
//     client that sent no Accept-Encoding (noAcceptEncodingHeader).
//
// ranged reports whether a Range addresses out.
func (h *Handler) encodeBody(r *http.Request, hdr http.Header, site Site, servedPath string, body []byte, gzipOK bool) (out []byte, ranged bool, release func()) {
	class := encodingCandidate(hdr, len(body))
	if class == uncompressed ||
		class == edgeCompressed && (r.Header.Get("Range") != "" || r.Header.Get(noAcceptEncodingHeader) != "") {
		return body, true, func() {}
	}
	out, coding, release := h.negotiatedBody(r, site, servedPath, body, gzipOK)
	if coding != "" {
		hdr.Set("Content-Encoding", coding)
	}
	if class == edgeCompressed {
		if coding != "" {
			weakenETag(hdr)
		}
		return out, coding == "", release
	}
	if coding != codingBrotli {
		weakenETag(hdr)
	}
	return out, coding == codingBrotli && len(out) <= maxRangedBrotliBytes, release
}

// noAcceptEncodingHeader marks a request whose client sent no
// Accept-Encoding. The edge plugin's staticSite instance sets it on every
// static site's route (deploy/gitops/charts/traefik-plugins/clientipheaders,
// w8/363): Traefik's forwarder asks for gzip on such a request and decodes the
// answer itself, so the server would otherwise take that client for a gzip
// one. A forged marker only gets its sender the identity body.
const noAcceptEncodingHeader = "X-Bex-No-Accept-Encoding"

// negotiatedBody is encodeBody's choice of bytes for a candidate: the cached
// variant in the coding the request negotiates, or body and "" for identity.
func (h *Handler) negotiatedBody(r *http.Request, site Site, servedPath string, body []byte, gzipOK bool) (out []byte, coding string, release func()) {
	release = func() {}
	coding = negotiateCoding(r.Header)
	if coding == "" || coding == codingGzip && (!gzipOK || len(body) < minGzipBytes) {
		return body, "", release
	}
	enc, ok := h.encodedVariant(coding, site.AppID, site.cacheKey(servedPath), body)
	if !ok {
		return body, "", release
	}
	// The encoded body leases its own bytes on top of the identity lease (the
	// identity body stays referenced); HEAD writes none. Without room, serve
	// identity rather than shed.
	lease := int64(len(enc))
	if r.Method == http.MethodHead {
		lease = 0
	}
	if !h.liveBodies.TryAcquire(lease) {
		return body, "", release
	}
	return enc, coding, func() { h.liveBodies.Release(lease) }
}

// encodedVariant returns the coding encoding of the identity body stored
// under key, computing it at most once per key and coding: concurrent requests
// collapse onto one encode (singleflight) and the result — including a failed
// encode, so it is not re-tried every request — is cached against the site's
// budget. ok=false means serve identity.
func (h *Handler) encodedVariant(coding, appID, key string, body []byte) (encoded []byte, ok bool) {
	vkey := variantKey(coding, key)
	if v, hit := h.cache.get(vkey); hit {
		return v.Body, !v.negative
	}
	v, err, _ := h.group.Do(vkey, func() (any, error) {
		if v, hit := h.cache.get(vkey); hit {
			return v, nil
		}
		if !h.encodes.acquire(appID) {
			return nil, errEncodeBusy
		}
		defer h.encodes.release(appID)
		lease := encodeLease(len(body))
		if !h.liveBodies.TryAcquire(lease) {
			return nil, errEncodeBusy
		}
		defer h.liveBodies.Release(lease)
		encode := encodeBrotli
		if coding == codingGzip {
			encode = encodeGzip
		}
		enc, encoded := encode(body)
		variant := Object{Body: enc, negative: !encoded}
		h.cache.put(appID, vkey, variant)
		return variant, nil
	})
	if err != nil {
		return nil, false
	}
	variant := v.(Object)
	return variant.Body, !variant.negative
}

// encodeBrotli compresses src whatever the result's size: Render serves br
// even where it is larger than identity (a 27-byte JSON file as 29 br bytes,
// w8/359). ok=false (and no bytes) means the encode failed or outgrew
// encodedBound.
func encodeBrotli(src []byte) (encoded []byte, ok bool) {
	out := &cappedBuffer{buf: make([]byte, 0, encodedBound(len(src)))}
	w := brotli.NewWriterOptions(out, brotli.WriterOptions{Quality: brotliQuality, LGWin: brotliWindowBits})
	if _, err := w.Write(src); err != nil {
		return nil, false
	}
	if err := w.Close(); err != nil {
		return nil, false
	}
	// Copy to an exact-size slice: the cache charges capacity, and the
	// worst-case buffer would otherwise stay allocated behind a small result.
	return append([]byte(nil), out.buf...), true
}

// encodeGzip is encodeBrotli for gzip, with the same bound: Render gzips a
// 67-byte robots.txt into 78 bytes (w8/359).
func encodeGzip(src []byte) (encoded []byte, ok bool) {
	out := &cappedBuffer{buf: make([]byte, 0, encodedBound(len(src)))}
	w, err := gzip.NewWriterLevel(out, gzipLevel)
	if err != nil {
		return nil, false
	}
	if _, err := w.Write(src); err != nil {
		return nil, false
	}
	if err := w.Close(); err != nil {
		return nil, false
	}
	return append([]byte(nil), out.buf...), true
}

// errBufferFull aborts an encode whose output outgrew encodedBound.
var errBufferFull = errors.New("staticserver: encoded output past its bound")

// cappedBuffer is an io.Writer that refuses to grow past its initial capacity.
type cappedBuffer struct{ buf []byte }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(b.buf)+len(p) > cap(b.buf) {
		return 0, errBufferFull
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// fileResponseWriter finishes the file response http.ServeContent writes for
// the size-byte body encodeBody chose.
//   - A 200 sends the exact length. ServeContent omits Content-Length on a full
//     response whenever Content-Encoding is set (it assumes an on-the-fly
//     compressing wrapper), but an encoded body here is encoded up front.
//   - A status other than 200/206 carries no encoded body, so the coding is
//     dropped (304 and 416 already strip it).
//   - A 304 carries notModifiedETag, the tag before encodeBody weakened it,
//     whatever the coding: Render's edge answers every revalidation with the
//     strong tag (captured 2026-11-21, w8/361).
//   - Accept-Ranges, which ServeContent sets on every 200 and 206, goes only
//     on a 200 whose body a Range addresses (ranged), as on Render's edge:
//     none on the other 200s (w8/362) and none on a 206 (captured 2026-11-21
//     on an icon, PNG, woff, PDF, JSON, XML and text, w8/363).
type fileResponseWriter struct {
	http.ResponseWriter
	size            int
	notModifiedETag string
	ranged          bool
}

func (w *fileResponseWriter) WriteHeader(code int) {
	hdr := w.Header()
	if !w.ranged || code == http.StatusPartialContent {
		hdr.Del("Accept-Ranges")
	}
	switch code {
	case http.StatusOK:
		hdr.Set("Content-Length", strconv.Itoa(w.size))
	case http.StatusPartialContent:
		// ServeContent sets the range's own Content-Length.
	case http.StatusNotModified:
		if w.notModifiedETag != "" {
			hdr.Set("Etag", w.notModifiedETag)
		}
		fallthrough
	default:
		hdr.Del("Content-Encoding")
	}
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (w *fileResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
