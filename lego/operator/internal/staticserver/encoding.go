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
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"golang.org/x/net/http/httpguts"
)

// Automatic Brotli (w8/m79, Render's documented static-site behavior): an
// eligible text-like object is served br-encoded to a visitor that accepts br.
// The encoded variant is computed once per immutable object key and cached
// beside the identity object under the same shared byte budget, so the CPU cost
// is paid per published object, not per request. Everything here degrades to
// the identity representation — never to an error — when a bound is reached.
const (
	// brotliQuality trades ratio for encode speed. 5 is the usual on-the-fly
	// sweet spot (most of quality 11's ratio at a fraction of the CPU).
	brotliQuality = 5
	// brotliWindowBits is the encoder's LZ window (1 MiB). The library default
	// (4 MiB) roughly triples encoder memory for little gain on web assets.
	brotliWindowBits = 20
	// minBrotliBytes skips bodies too small for encoding to pay for itself.
	minBrotliBytes = 1024
	// maxBrotliBytes bounds the input of one encode so a single request cannot
	// pin a core (or its share of the live-body budget) for long. Larger
	// eligible objects are served identity.
	maxBrotliBytes = 8 << 20 // 8 MiB
	// defaultMaxConcurrentEncodes bounds encode CPU on the shared server, and
	// defaultMaxSiteEncodes keeps one site from holding every slot. A request
	// that finds no slot is served identity, not queued.
	defaultMaxConcurrentEncodes = 2
	defaultMaxSiteEncodes       = 1
	// brotliEncoderStateBytes is the encoder's ring buffer + hash tables at
	// brotliQuality/brotliWindowBits, with headroom over the ~14 MiB measured
	// for an 8 MiB input (TestBrotliEncoderStateBound).
	brotliEncoderStateBytes = 24 << 20 // 24 MiB
)

// encodeLease is the live-body budget one encode of n input bytes holds: the
// output buffer and its exact-size copy (each at most n) plus encoder state.
// Leasing it from the live-body budget means encoding adds no memory ceiling
// of its own.
func encodeLease(n int) int64 { return 2*int64(n) + brotliEncoderStateBytes }

// errEncodeBusy reports that no encode slot or live-body capacity was free; the
// caller serves identity and does not cache the outcome.
var errEncodeBusy = errors.New("staticserver: brotli encode capacity reached")

// brotliVariantKey names the cached br variant of an object key. The leading
// NUL can never begin a key built by Site.keyFor, so no request path can name
// (and be served) a variant's bytes as if they were an identity object.
func brotliVariantKey(key string) string { return "\x00br\x00" + key }

// brotliCandidate reports whether the response being prepared in hdr could be
// served br-encoded: an eligible media type and size, no authored
// Content-Encoding (a precompressed asset is served exactly as published, w4/204)
// and no Cache-Control: no-transform. It depends only on the object and its
// header rules — not on the request — so it also decides Vary.
func brotliCandidate(hdr http.Header, size int) bool {
	if size < minBrotliBytes || size > maxBrotliBytes {
		return false
	}
	if _, authored := hdr["Content-Encoding"]; authored {
		return false
	}
	if httpguts.HeaderValuesContainsToken(hdr.Values("Cache-Control"), "no-transform") {
		return false
	}
	return compressibleType(hdr.Get("Content-Type"))
}

// compressibleType reports whether a media type is text-like enough for Brotli
// to help. Already-compressed formats (images, video, archives, woff2) are not.
func compressibleType(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if strings.HasPrefix(mediaType, "text/") ||
		strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml") {
		return true
	}
	switch mediaType {
	case "application/javascript", "application/x-javascript", "application/ecmascript",
		"application/json", "application/xml", "application/wasm",
		"image/svg+xml", "image/x-icon", "image/vnd.microsoft.icon", "image/bmp",
		"font/ttf", "font/otf", "application/vnd.ms-fontobject",
		"application/x-font-ttf", "application/x-font-otf":
		return true
	}
	return false
}

// acceptsBrotli reports whether the request's Accept-Encoding (RFC 9110
// §12.5.3) admits br with a nonzero weight: an explicit br entry decides; else
// a "*" entry does. An unparseable weight counts as zero, so a malformed offer
// gets the always-acceptable identity representation.
func acceptsBrotli(header http.Header) bool {
	brQ, starQ := -1.0, -1.0 // -1: not listed
	for _, line := range header.Values("Accept-Encoding") {
		for member := range strings.SplitSeq(line, ",") {
			coding, params, _ := strings.Cut(member, ";")
			switch strings.ToLower(strings.TrimSpace(coding)) {
			case "br":
				brQ = qvalue(params)
			case "*":
				starQ = qvalue(params)
			}
		}
	}
	if brQ >= 0 {
		return brQ > 0
	}
	return starQ > 0
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

// weakenETag marks an authored strong ETag weak on the br representation: the
// encoded bytes differ from the identity bytes, so one strong validator must
// not describe both (RFC 9110 §8.8.1). Revalidation (weak comparison) keeps
// working; If-Match/If-Range, which demand strong equality, fail closed.
func weakenETag(hdr http.Header) {
	if tag := hdr.Get("Etag"); strings.HasPrefix(tag, `"`) {
		hdr.Set("Etag", "W/"+tag)
	}
}

// brotliVariant returns the useful br encoding of the identity body stored
// under key, computing it at most once per key: concurrent requests collapse
// onto one encode (singleflight) and the result — including "not useful", so an
// incompressible asset is not re-tried every request — is cached against the
// site's budget. ok=false means serve identity.
func (h *Handler) brotliVariant(appID, key string, body []byte) (encoded []byte, ok bool) {
	vkey := brotliVariantKey(key)
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
		enc, useful := encodeBrotli(body)
		variant := Object{Body: enc, negative: !useful}
		h.cache.put(appID, vkey, variant)
		return variant, nil
	})
	if err != nil {
		return nil, false
	}
	variant := v.(Object)
	return variant.Body, !variant.negative
}

// encodeBrotli compresses src, reporting useful=false (and no bytes) unless the
// result saves at least 1/16 of the size. The encoder writes into a buffer
// capped at that threshold, so an incompressible input stops early instead of
// growing an output as large as itself.
func encodeBrotli(src []byte) (encoded []byte, useful bool) {
	out := &cappedBuffer{buf: make([]byte, 0, len(src)-len(src)/16)}
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

// errBufferFull aborts an encode whose output would not be useful.
var errBufferFull = errors.New("staticserver: encoded output not smaller")

// cappedBuffer is an io.Writer that refuses to grow past its initial capacity.
type cappedBuffer struct{ buf []byte }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(b.buf)+len(p) > cap(b.buf) {
		return 0, errBufferFull
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// brotliResponseWriter frames a br body that http.ServeContent serves.
// ServeContent omits Content-Length on a full response whenever
// Content-Encoding is set (it assumes an on-the-fly compressing wrapper); this
// body is fully encoded up front, so the exact length is known and sent. A
// status other than 200/206 carries no br body, so the coding is dropped (304
// and 416 already strip it; this covers the bodyless 412).
type brotliResponseWriter struct {
	http.ResponseWriter
	size int
}

func (w *brotliResponseWriter) WriteHeader(code int) {
	hdr := w.Header()
	switch code {
	case http.StatusOK:
		hdr.Set("Content-Length", strconv.Itoa(w.size))
	case http.StatusPartialContent:
		// ServeContent sets the range's own Content-Length.
	default:
		hdr.Del("Content-Encoding")
	}
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (w *brotliResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
