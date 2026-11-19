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

// Package fakes3 is an in-memory S3 over HTTP, for tests of the object store
// (ADR112). It serves the path-style subset bex uses (PUT, GET with Range,
// HEAD, DELETE, multi-object delete and ListObjectsV2) so tests exercise the
// real AWS client and real presigned URLs against it.
//
// It evaluates Range and the four conditional headers of a GET or HEAD as S3
// does (206, 304, 412 and 416), which the public gateway hands through. It does
// not check signatures. It does refuse a chunked PUT against a URL
// that signs a Content-Length, which is what Wasabi does (ADR112 § Not
// verified, settled 2026-11-15); it does not compare a declared length with the
// bytes sent, so tests read what was sent from the recorded requests.
package fakes3

import (
	"bytes"
	"crypto/md5" //nolint:gosec // S3 ETags are MD5, not a security use
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Object is one stored object.
type Object struct {
	Body         []byte
	Header       http.Header // the entity headers the PUT carried
	ETag         string
	LastModified time.Time
}

// Request is one request the server saw, kept for assertions.
type Request struct {
	Method string
	Bucket string
	Key    string
	Query  url.Values
	Header http.Header
}

// Server is the fake. Its URL is the endpoint to give the S3 client.
type Server struct {
	*httptest.Server

	mu       sync.Mutex
	buckets  map[string]map[string]*Object
	uploads  map[string]*upload
	nextID   int
	requests []Request
	down     bool
	// rangeQuirk makes a Range past the end answer 200 with the whole object, as
	// Wasabi does on a presigned GET (ADR112 § Not verified, 2026-11-16), instead of
	// S3's 416.
	rangeQuirk bool
}

// SetRangeQuirk makes a Range past an object's end answer 200 with the whole object
// (Wasabi's measured behavior) instead of 416.
func (s *Server) SetRangeQuirk(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rangeQuirk = on
}

// New starts a server holding the named, empty buckets.
func New(buckets ...string) *Server {
	s := &Server{buckets: map[string]map[string]*Object{}, uploads: map[string]*upload{}}
	for _, b := range buckets {
		s.buckets[b] = map[string]*Object{}
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// SetDown makes every request answer 503 until it is called with false.
func (s *Server) SetDown(down bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.down = down
}

// Put seeds or replaces an object.
func (s *Server) Put(bucket, key string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.put(bucket, key, body, http.Header{})
}

func (s *Server) put(bucket, key string, body []byte, header http.Header) {
	if s.buckets[bucket] == nil {
		s.buckets[bucket] = map[string]*Object{}
	}
	sum := md5.Sum(body) //nolint:gosec
	s.buckets[bucket][key] = &Object{
		Body: append([]byte(nil), body...), Header: header, ETag: `"` + hex.EncodeToString(sum[:]) + `"`,
		LastModified: time.Now().UTC().Truncate(time.Second),
	}
}

// Delete removes an object, as a lifecycle rule or a test cleanup would.
func (s *Server) Delete(bucket, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.buckets[bucket], key)
}

// SetLastModified sets a stored object's last-modified time, so a lifecycle
// test can make an object any age. It reports whether the object exists.
func (s *Server) SetLastModified(bucket, key string, t time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.buckets[bucket][key]
	if ok {
		o.LastModified = t.UTC().Truncate(time.Second)
	}
	return ok
}

// Get returns a stored object's body.
func (s *Server) Get(bucket, key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.buckets[bucket][key]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), o.Body...), true
}

// Keys lists a bucket's keys under prefix, in order.
func (s *Server) Keys(bucket, prefix string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for k := range s.buckets[bucket] {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// Requests returns every request seen so far, oldest first.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	s.mu.Lock()
	s.requests = append(s.requests, Request{Method: r.Method, Bucket: bucket, Key: key, Query: r.URL.Query(), Header: r.Header.Clone()})
	s.mu.Unlock()
	// A body is read before the lock is taken: a slow upload must not stop the
	// fake serving everything else, which is what real streaming tests need.
	var body []byte
	var bodyErr error
	if (r.Method == http.MethodPut || r.Method == http.MethodPost) && !chunkedRefusal(r) {
		body, bodyErr = io.ReadAll(r.Body)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.down {
		s.fail(w, r, http.StatusServiceUnavailable, "ServiceUnavailable", "the fake is down")
		return
	}
	objects, ok := s.buckets[bucket]
	if !ok {
		s.fail(w, r, http.StatusNotFound, "NoSuchBucket", "the bucket does not exist")
		return
	}
	q := r.URL.Query()
	switch {
	case key == "" && r.Method == http.MethodGet && q.Get("list-type") == "2":
		s.list(w, bucket, objects, q)
	case key == "" && r.Method == http.MethodPost && q.Has("delete"):
		s.deleteMany(w, r, objects, body)
	case key == "":
		s.fail(w, r, http.StatusNotImplemented, "NotImplemented", "the fake serves list-type=2 and delete only")
	case r.Method == http.MethodPost && q.Has("uploads"):
		s.createUpload(w, r, bucket, key)
	case r.Method == http.MethodPut && q.Has("partNumber") && q.Has("uploadId"):
		s.uploadPart(w, r, bucket, key, body, bodyErr)
	case r.Method == http.MethodPost && q.Has("uploadId"):
		s.completeUpload(w, r, bucket, key, body)
	case r.Method == http.MethodDelete && q.Has("uploadId"):
		s.abortUpload(w, r, bucket, key)
	case r.Method == http.MethodPut && r.Header.Get("X-Amz-Copy-Source") != "":
		s.copyObject(w, r, bucket, key)
	case r.Method == http.MethodPut:
		s.putObject(w, r, bucket, key, body, bodyErr)
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		s.getObject(w, r, objects, key)
	case r.Method == http.MethodDelete:
		delete(objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		s.fail(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed", r.Method)
	}
}

func (s *Server) putObject(w http.ResponseWriter, r *http.Request, bucket, key string, body []byte, bodyErr error) {
	if strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") || r.Header.Get("X-Amz-Decoded-Content-Length") != "" {
		s.fail(w, r, http.StatusNotImplemented, "NotImplemented", "aws-chunked bodies are not served; use a presigned URL")
		return
	}
	// Wasabi refuses a PUT sent chunked, with no Content-Length, when the
	// presigned URL signs one (ADR112 § Not verified, settled 2026-11-15).
	if chunkedRefusal(r) {
		s.fail(w, r, http.StatusForbidden, "SignatureDoesNotMatch", "a chunked body does not match the signed Content-Length")
		return
	}
	if bodyErr != nil {
		s.fail(w, r, http.StatusBadRequest, "IncompleteBody", bodyErr.Error())
		return
	}
	s.put(bucket, key, body, entityHeaders(r.Header))
	w.Header().Set("ETag", s.buckets[bucket][key].ETag)
	w.WriteHeader(http.StatusOK)
}

// entityHeaders are the headers an object keeps: the entity headers and the user
// metadata (x-amz-meta-*), which a GET and a HEAD return.
func entityHeaders(from http.Header) http.Header {
	header := http.Header{}
	for _, name := range []string{"Content-Type", "Cache-Control", "Content-Disposition", "Content-Encoding", "Content-Language", "Expires"} {
		if v := from.Get(name); v != "" {
			header.Set(name, v)
		}
	}
	for name, values := range from {
		if strings.HasPrefix(strings.ToLower(name), "x-amz-meta-") {
			header[name] = append([]string(nil), values...)
		}
	}
	return header
}

// copyObject serves CopyObject: the source is "<bucket>/<key>" in x-amz-copy-source,
// and the metadata directive says whether the destination keeps the source's headers
// (COPY, the default) or takes the request's (REPLACE). A missing source is 404.
func (s *Server) copyObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	raw, err := url.PathUnescape(r.Header.Get("X-Amz-Copy-Source"))
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, "InvalidArgument", "x-amz-copy-source")
		return
	}
	srcBucket, srcKey, _ := strings.Cut(strings.TrimPrefix(raw, "/"), "/")
	src, ok := s.buckets[srcBucket][srcKey]
	if !ok {
		s.fail(w, r, http.StatusNotFound, "NoSuchKey", "the copy source does not exist")
		return
	}
	header := src.Header.Clone()
	if strings.EqualFold(r.Header.Get("X-Amz-Metadata-Directive"), "REPLACE") {
		header = entityHeaders(r.Header)
	}
	s.put(bucket, key, src.Body, header)
	dst := s.buckets[bucket][key]
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><CopyObjectResult><ETag>%s</ETag><LastModified>%s</LastModified></CopyObjectResult>`,
		strings.ReplaceAll(dst.ETag, `"`, "&quot;"), dst.LastModified.Format("2006-01-02T15:04:05.000Z"))
}

// chunkedRefusal reports a PUT with no Content-Length against a URL that signs one.
func chunkedRefusal(r *http.Request) bool {
	return r.ContentLength < 0 && strings.Contains(r.URL.Query().Get("X-Amz-SignedHeaders"), "content-length")
}

func (s *Server) getObject(w http.ResponseWriter, r *http.Request, objects map[string]*Object, key string) {
	o, ok := objects[key]
	if !ok {
		s.fail(w, r, http.StatusNotFound, "NoSuchKey", "the key does not exist")
		return
	}
	for name, values := range o.Header {
		w.Header()[name] = values
	}
	if w.Header().Get("Content-Type") == "" {
		// S3 types an object stored with no Content-Type as an opaque binary; left
		// unset, Go's own server would sniff the bytes.
		w.Header().Set("Content-Type", "binary/octet-stream")
	}
	// Like S3, a response-content-* query parameter overrides the stored header.
	if v := r.URL.Query().Get("response-content-disposition"); v != "" {
		w.Header().Set("Content-Disposition", v)
	}
	if v := r.URL.Query().Get("response-content-type"); v != "" {
		w.Header().Set("Content-Type", v)
	}
	w.Header().Set("ETag", o.ETag)
	w.Header().Set("Last-Modified", o.LastModified.Format(http.TimeFormat))
	switch conditionalStatus(r, o) {
	case http.StatusPreconditionFailed:
		s.fail(w, r, http.StatusPreconditionFailed, "PreconditionFailed", "a condition of the request was not met")
		return
	case http.StatusNotModified:
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := o.Body
	status := http.StatusOK
	// S3 serves one byte range and ignores a header that asks for several.
	if rng := r.Header.Get("Range"); rng != "" && !strings.Contains(rng, ",") {
		start, end, ok := parseRange(rng, int64(len(body)))
		switch {
		case ok:
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
			body, status = body[start:end+1], http.StatusPartialContent
		case s.rangeQuirk:
			// Wasabi answers a Range past the end with the whole object and a 200.
		default:
			s.fail(w, r, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", rng)
			return
		}
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
}

// conditionalStatus evaluates the four conditional headers of a GET or HEAD as
// S3 does, in RFC 7232's order: 412 when If-Match or If-Unmodified-Since fails,
// 304 when If-None-Match matches or, without it, If-Modified-Since says the
// object is unchanged, and 0 when the request is unconditional or passes.
func conditionalStatus(r *http.Request, o *Object) int {
	etagIn := func(list string) bool {
		for _, tag := range strings.Split(list, ",") {
			if tag = strings.TrimSpace(tag); tag == "*" || tag == o.ETag {
				return true
			}
		}
		return false
	}
	if m := r.Header.Get("If-Match"); m != "" && !etagIn(m) {
		return http.StatusPreconditionFailed
	}
	if t, err := http.ParseTime(r.Header.Get("If-Unmodified-Since")); err == nil && o.LastModified.After(t) {
		return http.StatusPreconditionFailed
	}
	if n := r.Header.Get("If-None-Match"); n != "" {
		if etagIn(n) {
			return http.StatusNotModified
		}
		return 0
	}
	if t, err := http.ParseTime(r.Header.Get("If-Modified-Since")); err == nil && !o.LastModified.After(t) {
		return http.StatusNotModified
	}
	return 0
}

func parseRange(h string, size int64) (start, end int64, ok bool) {
	spec, found := strings.CutPrefix(h, "bytes=")
	if !found || strings.Contains(spec, ",") {
		return 0, 0, false
	}
	from, to, _ := strings.Cut(spec, "-")
	switch {
	case from == "":
		n, err := strconv.ParseInt(to, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		start, end = max(size-n, 0), size-1
	default:
		var err error
		if start, err = strconv.ParseInt(from, 10, 64); err != nil {
			return 0, 0, false
		}
		end = size - 1
		if to != "" {
			if end, err = strconv.ParseInt(to, 10, 64); err != nil {
				return 0, 0, false
			}
			end = min(end, size-1)
		}
	}
	return start, end, start >= 0 && start <= end && start < size
}

type listResult struct {
	XMLName     xml.Name       `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name        string         `xml:"Name"`
	Prefix      string         `xml:"Prefix"`
	KeyCount    int            `xml:"KeyCount"`
	MaxKeys     int            `xml:"MaxKeys"`
	IsTruncated bool           `xml:"IsTruncated"`
	Contents    []listContents `xml:"Contents"`
	// CommonPrefixes are the keys folded by a delimiter.
	CommonPrefixes []listPrefix `xml:"CommonPrefixes"`
}

type listPrefix struct {
	Prefix string `xml:"Prefix"`
}

type listContents struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int    `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

func (s *Server) list(w http.ResponseWriter, bucket string, objects map[string]*Object, q url.Values) {
	prefix, after, delimiter := q.Get("prefix"), q.Get("start-after"), q.Get("delimiter")
	maxKeys := 1000
	if v, err := strconv.Atoi(q.Get("max-keys")); err == nil && v > 0 {
		maxKeys = min(v, 1000)
	}
	var keys []string
	for k := range objects {
		if strings.HasPrefix(k, prefix) && k > after {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	res := listResult{Name: bucket, Prefix: prefix, MaxKeys: maxKeys}
	// One entry is a key or, with a delimiter, the common prefix that folds every key
	// under it; the page ends after maxKeys entries.
	seenPrefix := map[string]bool{}
	entries := 0
	for _, k := range keys {
		if delimiter != "" {
			if at := strings.Index(k[len(prefix):], delimiter); at >= 0 {
				common := k[:len(prefix)+at+len(delimiter)]
				if seenPrefix[common] {
					continue
				}
				if entries == maxKeys {
					res.IsTruncated = true
					break
				}
				seenPrefix[common] = true
				res.CommonPrefixes = append(res.CommonPrefixes, listPrefix{Prefix: common})
				entries++
				continue
			}
		}
		if entries == maxKeys {
			res.IsTruncated = true
			break
		}
		o := objects[k]
		res.Contents = append(res.Contents, listContents{
			Key: k, LastModified: o.LastModified.Format("2006-01-02T15:04:05.000Z"), ETag: o.ETag, Size: len(o.Body), StorageClass: "STANDARD",
		})
		entries++
	}
	res.KeyCount = entries
	w.Header().Set("Content-Type", "application/xml")
	_, _ = io.WriteString(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(res)
}

func (s *Server) deleteMany(w http.ResponseWriter, r *http.Request, objects map[string]*Object, body []byte) {
	var in struct {
		Objects []struct {
			Key string `xml:"Key"`
		} `xml:"Object"`
	}
	if err := xml.Unmarshal(body, &in); err != nil {
		s.fail(w, r, http.StatusBadRequest, "MalformedXML", err.Error())
		return
	}
	for _, o := range in.Objects {
		delete(objects, o.Key)
	}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = io.WriteString(w, xml.Header+`<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></DeleteResult>`)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if r.Method == http.MethodHead {
		w.WriteHeader(status)
		return
	}
	var msg bytes.Buffer
	_ = xml.EscapeText(&msg, []byte(message))
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `%s<Error><Code>%s</Code><Message>%s</Message></Error>`, xml.Header, code, msg.String())
}

// upload is one open multipart upload.
type upload struct {
	bucket, key string
	header      http.Header
	parts       map[int]*part
}

type part struct {
	body []byte
	etag string
}

// OpenUploads is the number of multipart uploads started and neither completed nor
// aborted.
func (s *Server) OpenUploads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.uploads)
}

// UploadParts is the part numbers an open upload holds, in order, or nil for one that is
// not open.
func (s *Server) UploadParts(uploadID string) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.uploads[uploadID]
	if !ok {
		return nil
	}
	var numbers []int
	for n := range u.parts {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	return numbers
}

func (s *Server) createUpload(w http.ResponseWriter, r *http.Request, bucket, key string) {
	s.nextID++
	id := fmt.Sprintf("fake-upload-%d", s.nextID)
	s.uploads[id] = &upload{bucket: bucket, key: key, header: entityHeaders(r.Header), parts: map[int]*part{}}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><InitiateMultipartUploadResult><Bucket>%s</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>`, bucket, key, id)
}

func (s *Server) openUpload(w http.ResponseWriter, r *http.Request, bucket, key string) (*upload, bool) {
	u, ok := s.uploads[r.URL.Query().Get("uploadId")]
	if !ok || u.bucket != bucket || u.key != key {
		s.fail(w, r, http.StatusNotFound, "NoSuchUpload", "the upload does not exist")
		return nil, false
	}
	return u, true
}

func partETag(body []byte) string {
	sum := md5.Sum(body) //nolint:gosec
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func (s *Server) uploadPart(w http.ResponseWriter, r *http.Request, bucket, key string, body []byte, bodyErr error) {
	u, ok := s.openUpload(w, r, bucket, key)
	if !ok {
		return
	}
	n, err := strconv.Atoi(r.URL.Query().Get("partNumber"))
	if err != nil || n < 1 || n > 10000 {
		s.fail(w, r, http.StatusBadRequest, "InvalidArgument", "part number")
		return
	}
	if r.Header.Get("X-Amz-Copy-Source") != "" {
		s.uploadPartCopy(w, r, u, n)
		return
	}
	if chunkedRefusal(r) {
		s.fail(w, r, http.StatusForbidden, "SignatureDoesNotMatch", "a chunked body does not match the signed Content-Length")
		return
	}
	if bodyErr != nil {
		s.fail(w, r, http.StatusBadRequest, "IncompleteBody", bodyErr.Error())
		return
	}
	u.parts[n] = &part{body: append([]byte(nil), body...), etag: partETag(body)}
	w.Header().Set("ETag", u.parts[n].etag)
	w.WriteHeader(http.StatusOK)
}

// uploadPartCopy serves UploadPartCopy: the whole source, or the bytes of its
// x-amz-copy-source-range.
func (s *Server) uploadPartCopy(w http.ResponseWriter, r *http.Request, u *upload, n int) {
	raw, err := url.PathUnescape(r.Header.Get("X-Amz-Copy-Source"))
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, "InvalidArgument", "x-amz-copy-source")
		return
	}
	srcBucket, srcKey, _ := strings.Cut(strings.TrimPrefix(raw, "/"), "/")
	src, ok := s.buckets[srcBucket][srcKey]
	if !ok {
		s.fail(w, r, http.StatusNotFound, "NoSuchKey", "the copy source does not exist")
		return
	}
	body := src.Body
	if rng := r.Header.Get("X-Amz-Copy-Source-Range"); rng != "" {
		start, end, ok := parseRange(rng, int64(len(body)))
		if !ok {
			s.fail(w, r, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", rng)
			return
		}
		body = body[start : end+1]
	}
	u.parts[n] = &part{body: append([]byte(nil), body...), etag: partETag(body)}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><CopyPartResult><ETag>%s</ETag><LastModified>%s</LastModified></CopyPartResult>`,
		strings.ReplaceAll(u.parts[n].etag, `"`, "&quot;"), time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
}

// completeUpload assembles the object from the parts the request lists, in the order it
// lists them. A part it names that the upload lacks, or whose ETag differs, is InvalidPart.
func (s *Server) completeUpload(w http.ResponseWriter, r *http.Request, bucket, key string, body []byte) {
	u, ok := s.openUpload(w, r, bucket, key)
	if !ok {
		return
	}
	var in struct {
		Parts []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		} `xml:"Part"`
	}
	if err := xml.Unmarshal(body, &in); err != nil || len(in.Parts) == 0 {
		s.fail(w, r, http.StatusBadRequest, "MalformedXML", "the part list")
		return
	}
	var whole []byte
	var sums []byte
	for _, p := range in.Parts {
		have, ok := u.parts[p.PartNumber]
		if !ok || strings.Trim(p.ETag, `"`) != strings.Trim(have.etag, `"`) {
			s.fail(w, r, http.StatusBadRequest, "InvalidPart", fmt.Sprintf("part %d", p.PartNumber))
			return
		}
		whole = append(whole, have.body...)
		raw, _ := hex.DecodeString(strings.Trim(have.etag, `"`))
		sums = append(sums, raw...)
	}
	s.put(bucket, key, whole, u.header)
	final := md5.Sum(sums) //nolint:gosec
	s.buckets[bucket][key].ETag = fmt.Sprintf(`"%s-%d"`, hex.EncodeToString(final[:]), len(in.Parts))
	delete(s.uploads, r.URL.Query().Get("uploadId"))
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><CompleteMultipartUploadResult><Bucket>%s</Bucket><Key>%s</Key><ETag>%s</ETag></CompleteMultipartUploadResult>`,
		bucket, key, strings.ReplaceAll(s.buckets[bucket][key].ETag, `"`, "&quot;"))
}

func (s *Server) abortUpload(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if _, ok := s.openUpload(w, r, bucket, key); !ok {
		return
	}
	delete(s.uploads, r.URL.Query().Get("uploadId"))
	w.WriteHeader(http.StatusNoContent)
}
