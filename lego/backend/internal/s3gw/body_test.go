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

package s3gw

import (
	"bytes"
	"crypto/md5" //nolint:gosec
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCRC64NVMEMatchesThePublishedCheckValue(t *testing.T) {
	h := newCRC64NVME()
	h.Write([]byte("123456789"))
	if got := h.Sum64(); got != 0xAE8B14860A799888 {
		t.Errorf("CRC-64/NVME of 123456789 = %016X, want AE8B14860A799888", got)
	}
}

// The slicing-by-8 path (eight bytes at a time) and the byte-at-a-time tail agree with
// the bitwise definition, whatever the write sizes.
func TestCRC64NVMEAgreesWithTheBitwiseDefinitionForEveryWriteSplit(t *testing.T) {
	data := payload(5000)
	bitwise := func(p []byte) uint64 {
		crc := ^uint64(0)
		for _, b := range p {
			crc ^= uint64(b)
			for range 8 {
				if crc&1 == 1 {
					crc = crc>>1 ^ crc64NVMEPoly
				} else {
					crc >>= 1
				}
			}
		}
		return ^crc
	}
	want := bitwise(data)
	for _, step := range []int{1, 3, 7, 8, 9, 64, 2047, 2048, 5000} {
		h := newCRC64NVME()
		for off := 0; off < len(data); off += step {
			h.Write(data[off:min(off+step, len(data))])
		}
		if got := h.Sum64(); got != want {
			t.Errorf("writes of %d bytes: %016X, want %016X", step, got, want)
		}
	}
	h := newCRC64NVME()
	h.Write(data)
	h.Reset()
	h.Write([]byte("123456789"))
	if h.Sum64() != 0xAE8B14860A799888 {
		t.Error("a reset hash does not start over")
	}
}

func sum(algorithm string, data []byte) string {
	h := checksumAlgorithms[algorithm]()
	h.Write(data)
	return encodeChecksum(h)
}

// chunkedBody frames data the way botocore and the Go SDK do for
// STREAMING-UNSIGNED-PAYLOAD-TRAILER.
func chunkedBody(data []byte, chunk int, trailerName, trailerValue string) []byte {
	var b bytes.Buffer
	for off := 0; off < len(data); off += chunk {
		end := min(off+chunk, len(data))
		fmt.Fprintf(&b, "%x\r\n", end-off)
		b.Write(data[off:end])
		b.WriteString("\r\n")
	}
	b.WriteString("0\r\n")
	if trailerName != "" {
		fmt.Fprintf(&b, "%s:%s\r\n", trailerName, trailerValue)
	}
	b.WriteString("\r\n")
	return b.Bytes()
}

// trailerRequest is a put the way the AWS CLI v2 sends it: chunked transfer, no
// Content-Length, aws-chunked, the decoded length and the trailer's name.
func trailerRequest(data []byte, framed []byte, trailer string) *request {
	r := httptest.NewRequest("PUT", "https://objects.onbex.co/b/k", bytes.NewReader(framed))
	r.ContentLength = -1
	r.Header.Set("X-Amz-Content-Sha256", streamingUnsignedTrailer)
	r.Header.Set("Content-Encoding", "aws-chunked")
	r.Header.Set("X-Amz-Decoded-Content-Length", fmt.Sprint(len(data)))
	r.Header.Set("X-Amz-Trailer", trailer)
	return &request{r: r}
}

func plainRequest(body []byte, mode string, mutate ...func(*http.Request)) *request {
	r := httptest.NewRequest("PUT", "https://objects.onbex.co/b/k", bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("X-Amz-Content-Sha256", mode)
	for _, m := range mutate {
		m(r)
	}
	return &request{r: r}
}

// drain reads the body the way the store call does and reports what it delivered.
func drain(pb *putBody) (delivered int, err error) {
	n, err := io.Copy(io.Discard, pb.Body)
	return int(n), err
}

func payload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + 3)
	}
	return b
}

func TestATrailerBodyWithNoContentLengthIsStoredAndItsChecksumEchoed(t *testing.T) {
	data := payload(100_000)
	for _, algorithm := range []string{"crc64nvme", "crc32", "crc32c", "sha1", "sha256"} {
		framed := chunkedBody(data, 8192, "x-amz-checksum-"+algorithm, sum(algorithm, data))
		pb, e := trailerRequest(data, framed, "x-amz-checksum-"+algorithm).openBody(maxSingleObject)
		if e != nil {
			t.Fatalf("%s: openBody = %v", algorithm, e)
		}
		var got bytes.Buffer
		if _, err := io.Copy(&got, pb.Body); err != nil {
			t.Fatalf("%s: %v", algorithm, err)
		}
		if !bytes.Equal(got.Bytes(), data) || pb.Size != int64(len(data)) {
			t.Errorf("%s: the decoded body differs from what was sent", algorithm)
		}
		if pb.Verified()[algorithm] != sum(algorithm, data) {
			t.Errorf("%s: verified = %v, want the checksum echoed", algorithm, pb.Verified())
		}
	}
}

func TestAWrongTrailerChecksumDeliversEveryByteButTheLastAndFails(t *testing.T) {
	data := payload(50_000)
	framed := chunkedBody(data, 4096, "x-amz-checksum-crc64nvme", sum("crc64nvme", payload(50_001)))
	pb, e := trailerRequest(data, framed, "x-amz-checksum-crc64nvme").openBody(maxSingleObject)
	if e != nil {
		t.Fatal(e)
	}
	delivered, err := drain(pb)
	if !errors.Is(err, errBadDigest) {
		t.Fatalf("read error = %v, want errBadDigest", err)
	}
	// The store call that read this stopped one byte short of its signed
	// Content-Length, which it refuses: nothing is stored.
	if delivered != len(data)-1 {
		t.Errorf("delivered %d of %d bytes before the failure, want all but the last", delivered, len(data))
	}
	if f := pb.Failure(); f == nil || f.Code != "BadDigest" || f.Status != 400 {
		t.Errorf("Failure = %v, want 400 BadDigest", f)
	}
	// The failure is sticky: nothing more can be read.
	if n, err := pb.Body.Read(make([]byte, 8)); n != 0 || err == nil {
		t.Errorf("a read after the failure returned %d, %v", n, err)
	}
}

func TestEveryHeaderDigestIsVerifiedBeforeTheLastByteIsReleased(t *testing.T) {
	data := payload(70_000)
	md5sum := md5.Sum(data) //nolint:gosec
	hexSum := sha256.Sum256(data)
	cases := []struct {
		name    string
		mode    string
		headers map[string]string
		want    string // the failure code for a corrupted digest
	}{
		{"content-md5", unsignedPayload, map[string]string{"Content-MD5": base64.StdEncoding.EncodeToString(md5sum[:])}, "BadDigest"},
		{"x-amz-checksum-crc32", unsignedPayload, map[string]string{"X-Amz-Checksum-Crc32": sum("crc32", data)}, "BadDigest"},
		{"x-amz-checksum-crc64nvme", unsignedPayload, map[string]string{"X-Amz-Checksum-Crc64nvme": sum("crc64nvme", data)}, "BadDigest"},
		{"x-amz-checksum-sha256", unsignedPayload, map[string]string{"X-Amz-Checksum-Sha256": sum("sha256", data)}, "BadDigest"},
		{"explicit sha-256", hex.EncodeToString(hexSum[:]), nil, "XAmzContentSHA256Mismatch"},
	}
	for _, c := range cases {
		good := plainRequest(data, c.mode, func(r *http.Request) {
			for k, v := range c.headers {
				r.Header.Set(k, v)
			}
		})
		pb, e := good.openBody(maxSingleObject)
		if e != nil {
			t.Fatalf("%s: %v", c.name, e)
		}
		if n, err := drain(pb); err != nil || n != len(data) {
			t.Errorf("%s: a correct digest delivered %d bytes with %v", c.name, n, err)
		}
		// The same request with the last byte of the body corrupted.
		bad := append([]byte(nil), data...)
		bad[len(bad)-1] ^= 0xff
		corrupt := plainRequest(bad, c.mode, func(r *http.Request) {
			for k, v := range c.headers {
				r.Header.Set(k, v)
			}
		})
		pb, e = corrupt.openBody(maxSingleObject)
		if e != nil {
			t.Fatalf("%s: %v", c.name, e)
		}
		delivered, err := drain(pb)
		if err == nil || delivered != len(bad)-1 {
			t.Errorf("%s: a corrupted body delivered %d of %d bytes with %v, want all but the last and an error", c.name, delivered, len(bad), err)
		}
		if f := pb.Failure(); f == nil || f.Code != c.want {
			t.Errorf("%s: Failure = %v, want %s", c.name, f, c.want)
		}
	}
}

func TestAnEmptyBodyIsVerifiedBeforeTheStoreIsAsked(t *testing.T) {
	good := trailerRequest(nil, chunkedBody(nil, 1, "x-amz-checksum-crc32", sum("crc32", nil)), "x-amz-checksum-crc32")
	pb, e := good.openBody(maxSingleObject)
	if e != nil || pb.Size != 0 || pb.Body != http.NoBody {
		t.Fatalf("an empty trailer put = %+v, %v", pb, e)
	}
	bad := trailerRequest(nil, chunkedBody(nil, 1, "x-amz-checksum-crc32", "AAAAAQ=="), "x-amz-checksum-crc32")
	if _, e := bad.openBody(maxSingleObject); e == nil || e.Code != "BadDigest" {
		t.Errorf("an empty put with a wrong trailer = %v, want BadDigest before any store call", e)
	}
	badMD5 := plainRequest(nil, unsignedPayload, func(r *http.Request) {
		r.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(make([]byte, 16)))
	})
	if _, e := badMD5.openBody(maxSingleObject); e == nil || e.Code != "BadDigest" {
		t.Errorf("an empty put with the wrong MD5 = %v, want BadDigest", e)
	}
}

func TestPayloadModesBexDoesNotServeAnswer501AndBadValuesAnswer400(t *testing.T) {
	data := payload(10)
	for _, mode := range []string{"STREAMING-AWS4-HMAC-SHA256-PAYLOAD", "STREAMING-AWS4-HMAC-SHA256-PAYLOAD-TRAILER", "STREAMING-AWS4-ECDSA-P256-SHA256-PAYLOAD"} {
		if _, e := plainRequest(data, mode).openBody(maxSingleObject); e == nil || e.Status != 501 {
			t.Errorf("%s = %v, want 501", mode, e)
		}
	}
	for _, mode := range []string{"", "nonsense", strings.Repeat("g", 64), strings.Repeat("a", 63)} {
		rq := plainRequest(data, mode)
		if mode == "" {
			rq.r.Header.Del("X-Amz-Content-Sha256") // a presigned request: unsigned
			if _, e := rq.openBody(maxSingleObject); e != nil {
				t.Errorf("no x-amz-content-sha256 (presigned) = %v, want an unsigned payload", e)
			}
			continue
		}
		if _, e := rq.openBody(maxSingleObject); e == nil || e.Status != 400 {
			t.Errorf("%q = %v, want 400", mode, e)
		}
	}
	// An x-amz-trailer bex cannot verify is 501, never ignored; so is a checksum header.
	if _, e := trailerRequest(data, chunkedBody(data, 4, "x-amz-checksum-xxh3", "AAAA"), "x-amz-checksum-xxh3").openBody(maxSingleObject); e == nil || e.Status != 501 {
		t.Errorf("an unknown trailer = %v, want 501", e)
	}
	if _, e := trailerRequest(data, chunkedBody(data, 4, "x-custom", "1"), "x-custom").openBody(maxSingleObject); e == nil || e.Status != 501 {
		t.Errorf("a non-checksum trailer = %v, want 501", e)
	}
	withUnknown := plainRequest(data, unsignedPayload, func(r *http.Request) { r.Header.Set("X-Amz-Checksum-Xxh3", "AAAA") })
	if _, e := withUnknown.openBody(maxSingleObject); e == nil || e.Status != 501 {
		t.Errorf("an unknown x-amz-checksum-* header = %v, want 501", e)
	}
	noTrailerHeader := trailerRequest(data, chunkedBody(data, 4, "x-amz-checksum-crc32", sum("crc32", data)), "")
	if _, e := noTrailerHeader.openBody(maxSingleObject); e == nil || e.Status != 400 {
		t.Errorf("aws-chunked with no x-amz-trailer = %v, want 400", e)
	}
	// The encoding and the mode agree.
	mismatch := plainRequest(data, unsignedPayload, func(r *http.Request) { r.Header.Set("Content-Encoding", "aws-chunked") })
	if _, e := mismatch.openBody(maxSingleObject); e == nil || e.Status != 400 {
		t.Errorf("aws-chunked with an unsigned payload = %v, want 400", e)
	}
}

func TestTheLengthIsContentLengthOrTheDecodedLengthAnd411IsForNeither(t *testing.T) {
	data := payload(100)
	// No Content-Length and no chunked decoded length: 411.
	rq := plainRequest(data, unsignedPayload)
	rq.r.ContentLength = -1
	if _, e := rq.openBody(maxSingleObject); e == nil || e.Status != 411 || e.Code != "MissingContentLength" {
		t.Errorf("no length = %v, want 411 MissingContentLength", e)
	}
	noDecoded := trailerRequest(data, chunkedBody(data, 64, "x-amz-checksum-crc32", sum("crc32", data)), "x-amz-checksum-crc32")
	noDecoded.r.Header.Del("X-Amz-Decoded-Content-Length")
	if _, e := noDecoded.openBody(maxSingleObject); e == nil || e.Status != 411 {
		t.Errorf("aws-chunked with no decoded length = %v, want 411", e)
	}
	// The decoded length, not the framed one, is the object's size: an SDK that
	// also sends the framed Content-Length gets the same answer.
	framed := chunkedBody(data, 64, "x-amz-checksum-crc32", sum("crc32", data))
	both := trailerRequest(data, framed, "x-amz-checksum-crc32")
	both.r.ContentLength = int64(len(framed))
	pb, e := both.openBody(maxSingleObject)
	if e != nil || pb.Size != 100 {
		t.Fatalf("framed Content-Length with a decoded length = %+v, %v, want size 100", pb, e)
	}
	// A bad or negative length, and one above the cap.
	for _, v := range []string{"abc", "-1", "1.5"} {
		rq := trailerRequest(data, framed, "x-amz-checksum-crc32")
		rq.r.Header.Set("X-Amz-Decoded-Content-Length", v)
		if _, e := rq.openBody(maxSingleObject); e == nil || e.Status != 400 {
			t.Errorf("decoded length %q = %v, want 400", v, e)
		}
	}
	tooBig := trailerRequest(data, framed, "x-amz-checksum-crc32")
	tooBig.r.Header.Set("X-Amz-Decoded-Content-Length", fmt.Sprint(maxSingleObject+1))
	if _, e := tooBig.openBody(maxSingleObject); e == nil || e.Code != "EntityTooLarge" || e.Status != 400 {
		t.Errorf("5 GiB + 1 = %v, want 400 EntityTooLarge", e)
	}
	if _, e := plainRequest(data, unsignedPayload).openBody(50); e == nil || e.Code != "EntityTooLarge" {
		t.Errorf("a body over the caller's cap = %v, want EntityTooLarge", e)
	}
}

func TestABodyThatDoesNotMatchItsDeclaredLengthFails(t *testing.T) {
	data := payload(1000)
	// Shorter than declared.
	short := plainRequest(data[:900], unsignedPayload)
	short.r.ContentLength = 1000
	pb, e := short.openBody(maxSingleObject)
	if e != nil {
		t.Fatal(e)
	}
	if _, err := drain(pb); err == nil {
		t.Error("a short body was read without an error")
	}
	if f := pb.Failure(); f == nil || f.Code != "IncompleteBody" {
		t.Errorf("a short body: %v, want IncompleteBody", f)
	}
	// Longer than declared (an aws-chunked body that carries more than it said).
	framed := chunkedBody(payload(1100), 256, "x-amz-checksum-crc32", sum("crc32", payload(1100)))
	long := trailerRequest(data, framed, "x-amz-checksum-crc32")
	pb, e = long.openBody(maxSingleObject)
	if e != nil {
		t.Fatal(e)
	}
	delivered, err := drain(pb)
	if err == nil || delivered != len(data)-1 {
		t.Errorf("a long body delivered %d bytes with %v, want all but the last and an error", delivered, err)
	}
	if f := pb.Failure(); f == nil {
		t.Error("a long body has no failure to answer with")
	}
}

func TestMalformedChunkFramingIsRefusedAndNeverParsedAsAnotherRequest(t *testing.T) {
	data := payload(40)
	good := chunkedBody(data, 16, "x-amz-checksum-crc32", sum("crc32", data))
	for name, framed := range map[string][]byte{
		"a non-hex size":        []byte("zz\r\n" + string(data) + "\r\n0\r\n\r\n"),
		"no CRLF after a chunk": []byte("28\r\n" + string(data) + "XX0\r\n\r\n"),
		"a bare LF":             []byte("28\n" + string(data) + "\r\n0\r\n\r\n"),
		"a chunk extension":     []byte("28;chunk-signature=abc\r\n" + string(data) + "\r\n0\r\n\r\n"),
		"truncated mid-chunk":   good[:20],
		"no final chunk":        good[:len(good)-len("0\r\nx-amz-checksum-crc32:xxxxxxxx\r\n\r\n")],
		"an endless size line":  []byte(strings.Repeat("f", maxChunkLine+10)),
		"too many trailers":     append(append([]byte("28\r\n"), append(data, []byte("\r\n0\r\n")...)...), []byte(strings.Repeat("x-a:b\r\n", 20)+"\r\n")...),
	} {
		pb, e := trailerRequest(data, framed, "x-amz-checksum-crc32").openBody(maxSingleObject)
		if e != nil {
			t.Fatalf("%s: %v", name, e)
		}
		if _, err := drain(pb); err == nil {
			t.Errorf("%s: the body was accepted", name)
		}
		if f := pb.Failure(); f == nil || f.Status != 400 {
			t.Errorf("%s: Failure = %v, want a 400", name, f)
		}
	}
}

func TestTheStoredEncodingDropsOnlyTheFramingToken(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want string
	}{
		{[]string{"aws-chunked"}, ""}, {[]string{"aws-chunked,gzip"}, "gzip"}, {[]string{"gzip, aws-chunked"}, "gzip"},
		{[]string{"gzip"}, "gzip"}, {[]string{"AWS-CHUNKED", "br"}, "br"}, {nil, ""}, {[]string{"gzip, br"}, "gzip, br"},
	} {
		if got := storedEncoding(c.in); got != c.want {
			t.Errorf("storedEncoding(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestChecksumsAreBase64OfTheBigEndianDigest(t *testing.T) {
	// CRC32 of "hello world" is 0x0D4A1185, which S3 shows as DUoRhQ==.
	if got := sum("crc32", []byte("hello world")); got != "DUoRhQ==" {
		t.Errorf("crc32 = %q, want DUoRhQ==", got)
	}
	if want := base64.StdEncoding.EncodeToString([]byte{0x0D, 0x4A, 0x11, 0x85}); want != "DUoRhQ==" {
		t.Fatal("the test's own expectation is wrong")
	}
	if got, want := sum("crc32c", []byte("123456789")), base64.StdEncoding.EncodeToString([]byte{0xE3, 0x06, 0x92, 0x83}); got != want {
		t.Errorf("crc32c(123456789) = %q, want %q (0xE3069283)", got, want)
	}
	if crc32.ChecksumIEEE([]byte("hello world")) != 0x0D4A1185 {
		t.Fatal("the reference value changed")
	}
	if got, want := sum("crc64nvme", []byte("123456789")), base64.StdEncoding.EncodeToString([]byte{0xAE, 0x8B, 0x14, 0x86, 0x0A, 0x79, 0x98, 0x88}); got != want {
		t.Errorf("crc64nvme(123456789) = %q, want %q", got, want)
	}
}

func FuzzChunkedReader(f *testing.F) {
	data := payload(300)
	f.Add(chunkedBody(data, 64, "x-amz-checksum-crc32", sum("crc32", data)))
	f.Add([]byte("0\r\n\r\n"))
	f.Add([]byte("5\r\nhello\r\n0\r\nx-amz-checksum-crc32:AAAA\r\n\r\n"))
	f.Add([]byte("zz\r\n"))
	f.Add([]byte(strings.Repeat("9", 5000)))
	f.Fuzz(func(t *testing.T, in []byte) {
		c := newChunkedReader(bytes.NewReader(in))
		n, err := io.Copy(io.Discard, c)
		if err == nil && n > int64(len(in)) {
			t.Fatalf("decoded %d bytes from %d", n, len(in))
		}
		// The parser must never hand out more than the stream held, nor panic.
	})
}
