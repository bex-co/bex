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

package s3

import (
	"bufio"
	"bytes"
	"crypto/md5"
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
	"sync/atomic"
	"testing"
	"time"
)

// fakeS3 is bex's S3 API in memory, over TLS the way the gateway is reached: path
// style, SigV4 headers or query, the checksum trailer, multipart uploads, ranged
// reads, CORS and lifecycle documents, and the error codes the gateway answers
// with. It records every request, so a test can say what the client sent and how
// many at once.
type fakeS3 struct {
	t   *testing.T
	srv *httptest.Server

	mu      sync.Mutex
	buckets map[string]*fakeBucket
	uploads map[string]*fakeUpload
	log     []string // "METHOD /path?query" of every request, in order
	nextID  int

	// hold makes every object request take this long, so requests overlap.
	hold time.Duration
	// accessKey is the only access key the fake accepts; others get a 403
	// InvalidAccessKeyId. Empty accepts any.
	accessKey string
	// intercept may answer a request itself: ok reports it did.
	intercept func(r *http.Request, n int) (status int, code, message string, ok bool)
	// pageSize is the most keys a listing returns, 1,000 as S3 does.
	pageSize int

	inflight, peak atomic.Int32
	requestCount   atomic.Int32
	now            func() time.Time
	auth           []string // every Authorization header and presigned credential seen
	agents         []string // every User-Agent
}

type fakeBucket struct {
	created   time.Time
	location  string
	objects   map[string]*fakeObject
	cors      []byte
	lifecycle []byte
}

type fakeObject struct {
	data        []byte
	modified    time.Time
	contentType string
	metadata    map[string]string
}

type fakeUpload struct {
	bucket, key string
	parts       map[int][]byte
	contentType string
	metadata    map[string]string
}

func newFakeS3(t *testing.T) *fakeS3 {
	t.Helper()
	f := &fakeS3{t: t, buckets: map[string]*fakeBucket{}, uploads: map[string]*fakeUpload{}, pageSize: 1000,
		now: func() time.Time { return time.Now().UTC().Truncate(time.Second) }}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// bucket makes (or returns) a bucket.
func (f *fakeS3) bucket(name string) *fakeBucket {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bucketLocked(name)
}

func (f *fakeS3) bucketLocked(name string) *fakeBucket {
	b := f.buckets[name]
	if b == nil {
		b = &fakeBucket{created: f.now(), location: "oregon", objects: map[string]*fakeObject{}}
		f.buckets[name] = b
	}
	return b
}

// put stores an object directly, with the time a test chooses.
func (f *fakeS3) put(bucket, key string, data []byte, modified time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bucketLocked(bucket).objects[key] = &fakeObject{data: data, modified: modified, metadata: map[string]string{}}
}

// object is a stored object's bytes, ok false when there is none.
func (f *fakeS3) object(bucket, key string) (*fakeObject, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.buckets[bucket]
	if b == nil {
		return nil, false
	}
	o, ok := b.objects[key]
	return o, ok
}

// keys are the keys of a bucket, sorted.
func (f *fakeS3) keys(bucket string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	if b := f.buckets[bucket]; b != nil {
		for k := range b.objects {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// requests are the recorded requests, in order, that start with a method and
// path prefix ("GET /photos").
func (f *fakeS3) requests(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, l := range f.log {
		if strings.HasPrefix(l, prefix) {
			out = append(out, l)
		}
	}
	return out
}

// count is how many recorded requests contain substr.
func (f *fakeS3) count(substr string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, l := range f.log {
		if strings.Contains(l, substr) {
			n++
		}
	}
	return n
}

func (f *fakeS3) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log, f.auth = nil, nil
	f.peak.Store(0)
}

func (f *fakeS3) fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amz-request-id", "REQ0001")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message><Resource>%s</Resource><RequestId>REQ0001</RequestId></Error>`,
		code, message, r.URL.Path)
}

func (f *fakeS3) serve(w http.ResponseWriter, r *http.Request) {
	n := int(f.requestCount.Add(1))
	cur := f.inflight.Add(1)
	defer f.inflight.Add(-1)
	for {
		peak := f.peak.Load()
		if cur <= peak || f.peak.CompareAndSwap(peak, cur) {
			break
		}
	}
	f.mu.Lock()
	f.log = append(f.log, r.Method+" "+r.URL.RequestURI())
	auth := r.Header.Get("Authorization")
	if auth == "" {
		auth = "query:" + r.URL.Query().Get("X-Amz-Credential")
	}
	f.auth = append(f.auth, auth)
	f.agents = append(f.agents, r.UserAgent())
	f.mu.Unlock()
	if f.hold > 0 && r.URL.Path != "/" {
		time.Sleep(f.hold)
	}
	if f.accessKey != "" && !strings.Contains(auth, "Credential="+f.accessKey+"/") && !strings.HasPrefix(auth, "query:"+f.accessKey+"/") {
		_, _ = io.Copy(io.Discard, r.Body)
		f.fail(w, r, http.StatusForbidden, "InvalidAccessKeyId", "The access key Id you provided does not exist in our records.")
		return
	}
	if f.intercept != nil {
		if status, code, message, ok := f.intercept(r, n); ok {
			_, _ = io.Copy(io.Discard, r.Body)
			f.fail(w, r, status, code, message)
			return
		}
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key, _ := strings.Cut(path, "/")
	q := r.URL.Query()
	switch {
	case path == "":
		f.listBuckets(w, r)
	case key == "":
		f.bucketOp(w, r, bucket, q)
	default:
		f.objectOp(w, r, bucket, key, q)
	}
}

func (f *fakeS3) listBuckets(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	var names []string
	for n := range f.buckets {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListAllMyBucketsResult><Owner><ID>tea-1</ID><DisplayName>tea-1</DisplayName></Owner><Buckets>`)
	for _, n := range names {
		fmt.Fprintf(&b, "<Bucket><Name>%s</Name><CreationDate>%s</CreationDate></Bucket>", n, f.buckets[n].created.Format("2006-01-02T15:04:05.000Z"))
	}
	b.WriteString("</Buckets></ListAllMyBucketsResult>")
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	_, _ = io.WriteString(w, b.String())
}

func (f *fakeS3) bucketOp(w http.ResponseWriter, r *http.Request, name string, q url.Values) {
	f.mu.Lock()
	b := f.buckets[name]
	f.mu.Unlock()
	if r.Method == http.MethodPut && !q.Has("cors") && !q.Has("lifecycle") {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		exists := f.buckets[name] != nil
		if !exists {
			var cfg struct {
				LocationConstraint string `xml:"LocationConstraint"`
			}
			_ = xml.Unmarshal(body, &cfg)
			f.bucketLocked(name).location = cfg.LocationConstraint
		}
		f.mu.Unlock()
		if exists {
			f.fail(w, r, http.StatusConflict, "BucketAlreadyOwnedByYou", "Your previous request to create the named bucket succeeded and you already own it.")
		}
		return
	}
	if b == nil {
		_, _ = io.Copy(io.Discard, r.Body)
		f.fail(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
		return
	}
	switch {
	case q.Has("cors"):
		f.configDoc(w, r, name, "cors", "NoSuchCORSConfiguration", "The CORS configuration does not exist")
	case q.Has("lifecycle"):
		f.configDoc(w, r, name, "lifecycle", "NoSuchLifecycleConfiguration", "The lifecycle configuration does not exist")
	case r.Method == http.MethodDelete:
		f.mu.Lock()
		busy := len(b.objects) > 0
		for _, u := range f.uploads {
			busy = busy || u.bucket == name
		}
		if !busy {
			delete(f.buckets, name)
		}
		f.mu.Unlock()
		if busy {
			f.fail(w, r, http.StatusConflict, "BucketNotEmpty", "The bucket you tried to delete is not empty")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && q.Has("delete"):
		f.deleteObjects(w, r, b)
	case r.Method == http.MethodGet && q.Get("list-type") == "2":
		f.listObjects(w, b, name, q)
	case r.Method == http.MethodHead:
		w.WriteHeader(http.StatusOK)
	default:
		f.fail(w, r, http.StatusNotImplemented, "NotImplemented", "A header you provided implies functionality that is not implemented")
	}
}

// configDoc serves one kind of bucket configuration document, kept as sent.
func (f *fakeS3) configDoc(w http.ResponseWriter, r *http.Request, bucket, kind, missingCode, missingMessage string) {
	f.mu.Lock()
	b := f.buckets[bucket]
	slot := &b.cors
	if kind == "lifecycle" {
		slot = &b.lifecycle
	}
	switch r.Method {
	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		*slot = body
		f.mu.Unlock()
		if status, code, message, bad := f.badRules(kind, body); bad {
			f.mu.Lock()
			*slot = nil
			f.mu.Unlock()
			f.fail(w, r, status, code, message)
		}
	case http.MethodGet:
		doc := *slot
		f.mu.Unlock()
		if doc == nil {
			f.fail(w, r, http.StatusNotFound, missingCode, missingMessage)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write(doc)
	case http.MethodDelete:
		*slot = nil
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		f.mu.Unlock()
	}
}

// badRules is what the gateway refuses in a rules document: it stores neither
// transitions nor tag filters.
func (f *fakeS3) badRules(kind string, body []byte) (int, string, string, bool) {
	if kind == "lifecycle" && (bytes.Contains(body, []byte("<Transition>")) || bytes.Contains(body, []byte("<Tag>"))) {
		return http.StatusNotImplemented, "NotImplemented", "Transition and tag filters are not implemented", true
	}
	return 0, "", "", false
}

type listKey struct {
	key string
	o   *fakeObject
}

func (f *fakeS3) listObjects(w http.ResponseWriter, b *fakeBucket, name string, q url.Values) {
	prefix, delimiter := q.Get("prefix"), q.Get("delimiter")
	after := ""
	if tok := q.Get("continuation-token"); tok != "" {
		raw, _ := hex.DecodeString(tok)
		after = string(raw)
	}
	limit := f.pageSize
	if mk, err := strconv.Atoi(q.Get("max-keys")); err == nil && mk > 0 && mk < limit {
		limit = mk
	}
	f.mu.Lock()
	var keys []string
	for k := range b.objects {
		if strings.HasPrefix(k, prefix) && k > after {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var contents []listKey
	var prefixes []string
	seen := map[string]bool{}
	truncated, last := false, ""
	for _, k := range keys {
		entry := k
		if delimiter != "" {
			if i := strings.Index(k[len(prefix):], delimiter); i >= 0 {
				entry = k[:len(prefix)+i+len(delimiter)]
			}
		}
		if len(contents)+len(prefixes) == limit {
			truncated = true
			break
		}
		last = k
		if entry != k {
			if !seen[entry] {
				seen[entry] = true
				prefixes = append(prefixes, entry)
			}
			continue
		}
		contents = append(contents, listKey{k, b.objects[k]})
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<?xml version="1.0" encoding="UTF-8"?><ListBucketResult><Name>%s</Name><Prefix>%s</Prefix><KeyCount>%d</KeyCount><MaxKeys>%d</MaxKeys><IsTruncated>%v</IsTruncated>`,
		name, xmlEscape(prefix), len(contents)+len(prefixes), limit, truncated)
	if delimiter != "" {
		fmt.Fprintf(&out, "<Delimiter>%s</Delimiter>", xmlEscape(delimiter))
	}
	if truncated {
		// A page that ends on a common prefix resumes after everything below it.
		resume := last
		if len(prefixes) > 0 && delimiter != "" && strings.HasPrefix(last, prefixes[len(prefixes)-1]) {
			resume = prefixes[len(prefixes)-1] + "\U0010ffff"
		}
		fmt.Fprintf(&out, "<NextContinuationToken>%s</NextContinuationToken>", hex.EncodeToString([]byte(resume)))
	}
	for _, c := range contents {
		fmt.Fprintf(&out, `<Contents><Key>%s</Key><LastModified>%s</LastModified><ETag>"%s"</ETag><Size>%d</Size><StorageClass>STANDARD</StorageClass></Contents>`,
			xmlEscape(c.key), c.o.modified.Format("2006-01-02T15:04:05.000Z"), etagOf(c.o.data), len(c.o.data))
	}
	for _, p := range prefixes {
		fmt.Fprintf(&out, "<CommonPrefixes><Prefix>%s</Prefix></CommonPrefixes>", xmlEscape(p))
	}
	out.WriteString("</ListBucketResult>")
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	_, _ = io.WriteString(w, out.String())
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func etagOf(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func (f *fakeS3) deleteObjects(w http.ResponseWriter, r *http.Request, b *fakeBucket) {
	var req struct {
		Objects []struct {
			Key string `xml:"Key"`
		} `xml:"Object"`
	}
	body, _ := io.ReadAll(r.Body)
	if err := xml.Unmarshal(body, &req); err != nil || len(req.Objects) > 1000 {
		f.fail(w, r, http.StatusBadRequest, "MalformedXML", "The XML you provided was not well-formed")
		return
	}
	f.mu.Lock()
	for _, o := range req.Objects {
		delete(b.objects, o.Key)
	}
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><DeleteResult></DeleteResult>`)
}

func (f *fakeS3) objectOp(w http.ResponseWriter, r *http.Request, bucket, key string, q url.Values) {
	f.mu.Lock()
	b := f.buckets[bucket]
	f.mu.Unlock()
	if b == nil {
		_, _ = io.Copy(io.Discard, r.Body)
		f.fail(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
		return
	}
	switch r.Method {
	case http.MethodPut:
		switch {
		case q.Has("uploadId"):
			f.uploadPart(w, r, q)
		case r.Header.Get("x-amz-copy-source") != "":
			f.copyObject(w, r, b, key)
		default:
			f.putObject(w, r, b, key)
		}
	case http.MethodPost:
		switch {
		case q.Has("uploads"):
			f.createUpload(w, r, bucket, key)
		case q.Has("uploadId"):
			f.completeUpload(w, r, b, q.Get("uploadId"))
		}
	case http.MethodDelete:
		if id := q.Get("uploadId"); id != "" {
			f.mu.Lock()
			delete(f.uploads, id)
			f.mu.Unlock()
		} else {
			f.mu.Lock()
			delete(b.objects, key)
			f.mu.Unlock()
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodGet, http.MethodHead:
		f.getObject(w, r, b, key)
	}
}

// readBody is a request's payload: the bytes, with the framing of the
// checksum-trailer mode (`aws-chunked`) removed.
func readBody(r *http.Request) ([]byte, error) {
	if !strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") && !strings.HasPrefix(r.Header.Get("x-amz-content-sha256"), "STREAMING-") {
		return io.ReadAll(r.Body)
	}
	br := bufio.NewReader(r.Body)
	var out bytes.Buffer
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.ParseInt(strings.TrimSpace(strings.SplitN(line, ";", 2)[0]), 16, 64)
		if err != nil {
			return nil, fmt.Errorf("chunk size %q: %w", line, err)
		}
		if size == 0 {
			_, _ = io.Copy(io.Discard, br) // the trailer
			return out.Bytes(), nil
		}
		if _, err := io.CopyN(&out, br, size); err != nil {
			return nil, err
		}
		if _, err := br.Discard(2); err != nil {
			return nil, err
		}
	}
}

func userMetadata(r *http.Request) map[string]string {
	md := map[string]string{}
	for k, v := range r.Header {
		if name, ok := strings.CutPrefix(strings.ToLower(k), "x-amz-meta-"); ok {
			md[name] = v[0] // names are stored in lowercase, as the gateway does (w21/113)
		}
	}
	return md
}

func (f *fakeS3) putObject(w http.ResponseWriter, r *http.Request, b *fakeBucket, key string) {
	body, err := readBody(r)
	if err != nil {
		f.fail(w, r, http.StatusBadRequest, "IncompleteBody", err.Error())
		return
	}
	f.mu.Lock()
	b.objects[key] = &fakeObject{data: body, modified: f.now(), contentType: r.Header.Get("Content-Type"), metadata: userMetadata(r)}
	f.mu.Unlock()
	w.Header().Set("ETag", `"`+etagOf(body)+`"`)
}

func (f *fakeS3) copyObject(w http.ResponseWriter, r *http.Request, b *fakeBucket, key string) {
	raw, _ := url.PathUnescape(r.Header.Get("x-amz-copy-source"))
	srcBucket, srcKey, _ := strings.Cut(strings.TrimPrefix(raw, "/"), "/")
	f.mu.Lock()
	var src *fakeObject
	if sb := f.buckets[srcBucket]; sb != nil {
		src = sb.objects[srcKey]
	}
	if src == nil {
		f.mu.Unlock()
		f.fail(w, r, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
		return
	}
	dst := &fakeObject{data: src.data, modified: f.now(), contentType: src.contentType, metadata: src.metadata}
	if r.Header.Get("x-amz-metadata-directive") == "REPLACE" {
		dst.contentType, dst.metadata = r.Header.Get("Content-Type"), userMetadata(r)
	}
	b.objects[key] = dst
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><CopyObjectResult><ETag>"%s"</ETag><LastModified>%s</LastModified></CopyObjectResult>`,
		etagOf(src.data), dst.modified.Format("2006-01-02T15:04:05.000Z"))
}

func (f *fakeS3) createUpload(w http.ResponseWriter, r *http.Request, bucket, key string) {
	f.mu.Lock()
	f.nextID++
	id := fmt.Sprintf("upload-%d", f.nextID)
	f.uploads[id] = &fakeUpload{bucket: bucket, key: key, parts: map[int][]byte{}, contentType: r.Header.Get("Content-Type"), metadata: userMetadata(r)}
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><InitiateMultipartUploadResult><Bucket>%s</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>`, bucket, xmlEscape(key), id)
}

func (f *fakeS3) uploadPart(w http.ResponseWriter, r *http.Request, q url.Values) {
	body, err := readBody(r)
	if err != nil {
		f.fail(w, r, http.StatusBadRequest, "IncompleteBody", err.Error())
		return
	}
	n, _ := strconv.Atoi(q.Get("partNumber"))
	f.mu.Lock()
	u := f.uploads[q.Get("uploadId")]
	if u != nil {
		u.parts[n] = body
	}
	f.mu.Unlock()
	if u == nil {
		f.fail(w, r, http.StatusNotFound, "NoSuchUpload", "The specified upload does not exist.")
		return
	}
	w.Header().Set("ETag", `"`+etagOf(body)+`"`)
}

func (f *fakeS3) completeUpload(w http.ResponseWriter, r *http.Request, b *fakeBucket, id string) {
	_, _ = io.Copy(io.Discard, r.Body)
	f.mu.Lock()
	u := f.uploads[id]
	if u == nil {
		f.mu.Unlock()
		f.fail(w, r, http.StatusNotFound, "NoSuchUpload", "The specified upload does not exist.")
		return
	}
	var nums []int
	for n := range u.parts {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var data []byte
	for _, n := range nums {
		data = append(data, u.parts[n]...)
	}
	b.objects[u.key] = &fakeObject{data: data, modified: f.now(), contentType: u.contentType, metadata: u.metadata}
	delete(f.uploads, id)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><CompleteMultipartUploadResult><Bucket>x</Bucket><Key>%s</Key><ETag>"%s-%d"</ETag></CompleteMultipartUploadResult>`, xmlEscape(u.key), etagOf(data), len(nums))
}

func (f *fakeS3) getObject(w http.ResponseWriter, r *http.Request, b *fakeBucket, key string) {
	f.mu.Lock()
	o := b.objects[key]
	f.mu.Unlock()
	if o == nil {
		f.fail(w, r, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
		return
	}
	h := w.Header()
	h.Set("ETag", `"`+etagOf(o.data)+`"`)
	h.Set("Last-Modified", o.modified.UTC().Format(http.TimeFormat))
	h.Set("Accept-Ranges", "bytes")
	if o.contentType != "" {
		h.Set("Content-Type", o.contentType)
	}
	for k, v := range o.metadata {
		h.Set("x-amz-meta-"+k, v)
	}
	data := o.data
	status := http.StatusOK
	if rng := r.Header.Get("Range"); rng != "" {
		var start, end int64
		if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil || start >= int64(len(data)) {
			f.fail(w, r, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", "The requested range is not satisfiable")
			return
		}
		end = min(end, int64(len(data))-1)
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		data, status = data[start:end+1], http.StatusPartialContent
	}
	h.Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}
