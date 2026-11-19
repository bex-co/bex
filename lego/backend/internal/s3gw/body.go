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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// The payload modes a stock client sends in x-amz-content-sha256 (ADR112 D3):
// UNSIGNED-PAYLOAD, the body's hex SHA-256, and STREAMING-UNSIGNED-PAYLOAD-TRAILER
// (an aws-chunked body whose digest follows the bytes). The two signed-chunk modes
// answer 501 (D10): over HTTPS the AWS CLI and the current SDKs send the unsigned
// ones.
const (
	streamingUnsignedTrailer = "STREAMING-UNSIGNED-PAYLOAD-TRAILER"
	streamingSignedPrefix    = "STREAMING-AWS4-"
)

// errBadDigest and errBadSHA256 are a body that did not match a digest the client
// sent: no byte of it was committed.
var (
	errBadDigest = errors.New("the body does not match the checksum the client sent")
	errBadSHA256 = errors.New("the body does not match the x-amz-content-sha256 the client sent")
	errBodyShort = errors.New("the body is shorter than its declared length")
	errBodyLong  = errors.New("the body is longer than its declared length")
)

// putBody is a request's body ready to stream to the object store: its declared
// length, the reader to hand to it and, once it was read to its end, the checksums
// it verified. The reader withholds the body's last byte until every digest the
// client sent has been verified, so a mismatch ends the upstream request short of
// its signed Content-Length, which the store refuses: nothing is stored, a previous
// object is untouched and the corrupt bytes are never readable.
type putBody struct {
	Size int64
	Body io.Reader
	v    *verifyingReader
}

// Verified is the x-amz-checksum-<algorithm> values bex verified, to echo.
func (b *putBody) Verified() map[string]string { return b.v.verified }

// Failure is the error to answer with when the store call failed because the body
// did not verify, nil when the body is not what failed.
func (b *putBody) Failure() *Error {
	switch {
	case errors.Is(b.v.err, errBadSHA256):
		return newError(http.StatusBadRequest, "XAmzContentSHA256Mismatch", "The provided 'x-amz-content-sha256' header does not match what was computed.")
	case errors.Is(b.v.err, errBadDigest):
		return newError(http.StatusBadRequest, "BadDigest", "The Content-MD5 or checksum value that you specified did not match what we received.")
	case errors.Is(b.v.err, errBodyShort), errors.Is(b.v.err, io.ErrUnexpectedEOF):
		return newError(http.StatusBadRequest, "IncompleteBody", "You did not provide the number of bytes specified by the Content-Length HTTP header.")
	case errors.Is(b.v.err, errBodyLong):
		return errInvalidRequest("The body is longer than the Content-Length or x-amz-decoded-content-length the request declared.")
	case errors.Is(b.v.err, errMalformedChunks):
		return errInvalidArgument("The aws-chunked body is malformed: " + b.v.err.Error())
	}
	return nil
}

// openBody checks the request's payload mode, length and digests and returns the
// body to stream. Everything that can refuse does so here, before the first byte is
// read, so a client that sent Expect: 100-continue is refused without sending one.
func (rq *request) openBody(maxSize int64) (*putBody, *Error) {
	r := rq.r
	mode := rq.payloadMode()
	if strings.HasPrefix(mode, streamingSignedPrefix) {
		return nil, errNotImplemented("Signed-chunk payloads (" + mode + ") are not implemented; send an unsigned payload over HTTPS")
	}
	chunked := false
	switch {
	case mode == streamingUnsignedTrailer:
		chunked = true
	case mode == unsignedPayload, isHex64(mode):
	default:
		return nil, errInvalidArgument("x-amz-content-sha256 must be UNSIGNED-PAYLOAD, STREAMING-UNSIGNED-PAYLOAD-TRAILER or the hex SHA-256 of the body")
	}
	if hasToken(r.Header.Values("Content-Encoding"), "aws-chunked") != chunked {
		return nil, errInvalidArgument("Content-Encoding must be aws-chunked exactly when x-amz-content-sha256 is " + streamingUnsignedTrailer)
	}
	size, e := declaredLength(r, chunked)
	if e != nil {
		return nil, e
	}
	if size < 0 || size > maxSize {
		return nil, newError(http.StatusBadRequest, "EntityTooLarge", fmt.Sprintf("Your proposed upload exceeds the maximum allowed object size of %d bytes.", maxSize))
	}
	v := &verifyingReader{size: size, verified: map[string]string{}}
	var src io.Reader = r.Body
	if chunked {
		v.chunks = newChunkedReader(r.Body)
		src = v.chunks
		trailer := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Amz-Trailer")))
		if trailer == "" {
			return nil, errInvalidArgument("x-amz-trailer is required with " + streamingUnsignedTrailer)
		}
		name, ok := checksumName(trailer)
		if _, known := checksumAlgorithms[name]; !ok || !known {
			return nil, errNotImplemented("The trailer " + trailer + " is not implemented; bex verifies the CRC32, CRC32C, CRC64NVME, SHA1 and SHA256 checksums")
		}
		v.trailerAlgorithm = name
		v.add(name)
	}
	v.src = src
	if isHex64(mode) {
		v.wantSHA256 = mode
		v.sha = sha256.New()
	}
	if e := v.expectHeaders(r); e != nil {
		return nil, e
	}
	pb := &putBody{Size: size, Body: v, v: v}
	if size == 0 {
		// There is no byte to withhold: an empty put is verified before the store is
		// asked to create the empty object.
		if err := v.finish(); err != nil {
			if f := pb.Failure(); f != nil {
				return nil, f
			}
			return nil, errInvalidArgument(err.Error())
		}
		pb.Body = http.NoBody
	}
	return pb, nil
}

// payloadMode is the x-amz-content-sha256 the request was signed with: UNSIGNED-PAYLOAD
// for a presigned URL, which carries no such header.
func (rq *request) payloadMode() string {
	if v := rq.r.Header.Get("X-Amz-Content-Sha256"); v != "" {
		return v
	}
	return unsignedPayload
}

// declaredLength is the length of the body: Content-Length, or
// x-amz-decoded-content-length for an aws-chunked body (whose own length, with the
// framing, is not the object's). 411 MissingContentLength is for neither, which is
// what a chunked transfer encoding with no decoded length is.
func declaredLength(r *http.Request, chunked bool) (int64, *Error) {
	missing := newError(http.StatusLengthRequired, "MissingContentLength", "You must provide the Content-Length HTTP header.")
	if chunked {
		v := r.Header.Get("X-Amz-Decoded-Content-Length")
		if v == "" {
			return 0, missing
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			return 0, errInvalidArgument("x-amz-decoded-content-length must be a number of bytes")
		}
		return n, nil
	}
	if r.ContentLength < 0 {
		return 0, missing
	}
	return r.ContentLength, nil
}

// storedEncoding is Content-Encoding without the aws-chunked token, which is only
// the framing the client used to deliver the body (any other encoding stays); ""
// stores none.
func storedEncoding(values []string) string {
	var kept []string
	for _, v := range values {
		for _, tok := range strings.Split(v, ",") {
			if tok = strings.TrimSpace(tok); tok != "" && !strings.EqualFold(tok, "aws-chunked") {
				kept = append(kept, tok)
			}
		}
	}
	return strings.Join(kept, ", ")
}

func hasToken(values []string, want string) bool {
	for _, v := range values {
		for _, tok := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(tok), want) {
				return true
			}
		}
	}
	return false
}

func isHex64(s string) bool { return len(s) == 64 && isHex(s) }

// verifyingReader streams the body, hashes what it hands out, and verifies every
// digest the client sent when the body's last byte is read.
type verifyingReader struct {
	src  io.Reader
	size int64
	pos  int64
	// chunks is the aws-chunked decoder, whose trailer holds a trailing checksum.
	chunks           *chunkedReader
	trailerAlgorithm string
	hashers          map[string]hash.Hash // by algorithm
	wantHeaders      map[string]string    // algorithm -> base64 from a header
	md5              hash.Hash
	wantMD5          string
	sha              hash.Hash
	wantSHA256       string

	last     [1]byte
	lastSent bool
	finished bool
	err      error
	verified map[string]string
}

func (v *verifyingReader) add(algorithm string) {
	if v.hashers == nil {
		v.hashers = map[string]hash.Hash{}
	}
	if _, ok := v.hashers[algorithm]; !ok {
		v.hashers[algorithm] = checksumAlgorithms[algorithm]()
	}
}

// expectHeaders registers the digests the request carries as headers: Content-MD5
// and x-amz-checksum-<algorithm>. A checksum header bex cannot verify is a 501, so
// integrity is never skipped silently.
func (v *verifyingReader) expectHeaders(r *http.Request) *Error {
	if md5v := r.Header.Get("Content-Md5"); md5v != "" {
		raw, err := base64.StdEncoding.DecodeString(md5v)
		if err != nil || len(raw) != 16 {
			return newError(http.StatusBadRequest, "InvalidDigest", "The Content-MD5 you specified was invalid.")
		}
		v.md5, v.wantMD5 = md5Hash(), md5v
	}
	for name, values := range r.Header {
		algorithm, ok := checksumName(name)
		if !ok {
			continue
		}
		if _, known := checksumAlgorithms[algorithm]; !known {
			return errNotImplemented("The checksum header " + name + " is not implemented; bex verifies the CRC32, CRC32C, CRC64NVME, SHA1 and SHA256 checksums")
		}
		if v.wantHeaders == nil {
			v.wantHeaders = map[string]string{}
		}
		v.wantHeaders[algorithm] = values[0]
		v.add(algorithm)
	}
	return nil
}

func (v *verifyingReader) hashAll(p []byte) {
	for _, h := range v.hashers {
		_, _ = h.Write(p)
	}
	if v.md5 != nil {
		_, _ = v.md5.Write(p)
	}
	if v.sha != nil {
		_, _ = v.sha.Write(p)
	}
}

// Read serves every byte but the last as it arrives. The last is read, the body is
// checked to end there, every digest is verified, and only then is it handed on;
// on a mismatch the read fails and the upstream request ends one byte short.
func (v *verifyingReader) Read(p []byte) (int, error) {
	if v.err != nil {
		return 0, v.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	if v.pos < v.size-1 {
		want := min(int64(len(p)), v.size-1-v.pos)
		n, err := v.src.Read(p[:want])
		v.hashAll(p[:n])
		v.pos += int64(n)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// The body ended before its declared length.
				err = errBodyShort
			}
			v.err = err
			return n, err
		}
		return n, nil
	}
	if v.lastSent {
		return 0, io.EOF
	}
	if err := v.readLast(); err != nil {
		v.err = err
		return 0, err
	}
	p[0] = v.last[0]
	v.lastSent = true
	return 1, nil
}

// readLast reads the final byte, then verifies the body is complete.
func (v *verifyingReader) readLast() error {
	if _, err := io.ReadFull(v.src, v.last[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return errBodyShort
		}
		return err
	}
	v.hashAll(v.last[:])
	v.pos++
	return v.finish()
}

// finish checks that nothing follows the declared length and verifies every digest.
func (v *verifyingReader) finish() error {
	if v.finished {
		return v.err
	}
	v.finished = true
	v.err = v.check()
	return v.err
}

func (v *verifyingReader) check() error {
	var extra [1]byte
	n, err := v.src.Read(extra[:])
	switch {
	case n > 0:
		return errBodyLong
	case err != nil && !errors.Is(err, io.EOF):
		return err
	}
	return v.verify()
}

func (v *verifyingReader) verify() error {
	if v.sha != nil && hex.EncodeToString(v.sha.Sum(nil)) != v.wantSHA256 {
		return errBadSHA256
	}
	if v.md5 != nil && base64.StdEncoding.EncodeToString(v.md5.Sum(nil)) != v.wantMD5 {
		return errBadDigest
	}
	for algorithm, want := range v.wantHeaders {
		got := encodeChecksum(v.hashers[algorithm])
		if got != want {
			return errBadDigest
		}
		v.verified[algorithm] = got
	}
	if v.trailerAlgorithm != "" {
		want, ok := v.chunks.trailer[checksumHeaderPrefix+v.trailerAlgorithm]
		if !ok {
			return fmt.Errorf("%w: the trailer %s%s is missing", errMalformedChunks, checksumHeaderPrefix, v.trailerAlgorithm)
		}
		got := encodeChecksum(v.hashers[v.trailerAlgorithm])
		if got != want {
			return errBadDigest
		}
		v.verified[v.trailerAlgorithm] = got
	}
	return nil
}

// readSmall reads a small request body (an XML document) completely, verifying
// the same digests, and refuses one longer than max. It is for the operations
// whose body is a document and not an object.
func (rq *request) readSmall(max int64) ([]byte, *Error) {
	pb, e := rq.openBody(max)
	if e != nil {
		return nil, e
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, pb.Body); err != nil {
		if f := pb.Failure(); f != nil {
			return nil, f
		}
		return nil, newError(http.StatusBadRequest, "IncompleteBody", "The request body could not be read: "+err.Error())
	}
	return buf.Bytes(), nil
}

// echoChecksums sets the x-amz-checksum-* headers of the digests bex verified.
func echoChecksums(w http.ResponseWriter, verified map[string]string) {
	for algorithm, value := range verified {
		w.Header().Set(checksumHeaderPrefix+algorithm, value)
	}
}

// maxSingleObject is the largest single put and the largest multipart total
// (ADR112 D3: the S3 single-PUT ceiling).
const maxSingleObject = objectstorage.MaxObjectBytesPresigned
