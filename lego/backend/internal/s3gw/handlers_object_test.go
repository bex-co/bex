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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// object makes a bucket for the key's workspace and returns the physical Wasabi
// prefix its objects live under.
func (p *pgRig) bucket(name string, region objectstorage.Region) string {
	p.t.Helper()
	b, err := p.Svc.CreateBucket(context.Background(), p.ws, name, region, "")
	if err != nil {
		p.t.Fatal(err)
	}
	return objectstorage.Prefix(p.ws, b.ID)
}

func (p *pgRig) stored(region objectstorage.Region, prefix, key string) ([]byte, bool) {
	return p.Fakes[region].Get(region.Bucket(), prefix+key)
}

func TestAnObjectRoundTripsByteIdenticalWithItsHeadersAndMetadata(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	prefix := p.bucket("photos", objectstorage.Oregon)
	data := payload(300_000)
	put := p.signed(k, "PUT", "/photos/dir/cat.jpg", bytes.NewReader(data), func(r *http.Request) {
		r.ContentLength = int64(len(data))
		r.Header.Set("Content-Type", "image/jpeg")
		r.Header.Set("Cache-Control", "max-age=60")
		r.Header.Set("Content-Language", "en")
		r.Header.Set("Content-Disposition", "inline; filename=cat.jpg")
		r.Header.Set("x-amz-meta-mtime", "1700000000.5")
		r.Header.Set("x-amz-meta-owner", "alice")
	})
	rec, doc := p.do(put)
	if rec.Code != 200 || rec.Header().Get("ETag") == "" {
		t.Fatalf("PutObject = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
	// The physical key is <tea>/<obk>/<key>, and the bytes are exactly the ones sent.
	if got, ok := p.stored(objectstorage.Oregon, prefix, "dir/cat.jpg"); !ok || !bytes.Equal(got, data) {
		t.Fatal("the object is not in the shared bucket under the workspace's prefix, byte for byte")
	}
	get, _ := p.do(p.signed(k, "GET", "/photos/dir/cat.jpg", nil))
	h := get.Header()
	if get.Code != 200 || !bytes.Equal(get.Body.Bytes(), data) {
		t.Fatalf("GetObject = %d, %d body bytes", get.Code, get.Body.Len())
	}
	for name, want := range map[string]string{
		"Content-Type": "image/jpeg", "Cache-Control": "max-age=60", "Content-Language": "en", "Content-Disposition": "inline; filename=cat.jpg",
		"x-amz-meta-mtime": "1700000000.5", "x-amz-meta-owner": "alice", "Accept-Ranges": "bytes", "X-Content-Type-Options": "nosniff",
		"Content-Security-Policy": "default-src 'none'; sandbox", "Content-Length": fmt.Sprint(len(data)), "ETag": rec.Header().Get("ETag"),
	} {
		if got := h.Get(name); got != want {
			t.Errorf("GET %s = %q, want %q", name, got, want)
		}
	}
	if h.Get("Last-Modified") == "" {
		t.Error("GET has no Last-Modified")
	}
	head, _ := p.do(p.signed(k, "HEAD", "/photos/dir/cat.jpg", nil))
	if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") != fmt.Sprint(len(data)) || head.Header().Get("x-amz-meta-owner") != "alice" {
		t.Errorf("HeadObject = %d with %d body bytes, headers %v", head.Code, head.Body.Len(), head.Header())
	}
	// An object stored with no disposition downloads as an attachment, never rendered on bex's origin.
	p.do(p.signed(k, "PUT", "/photos/page.html", strings.NewReader("<script>alert(1)</script>"), func(r *http.Request) {
		r.ContentLength = 25
		r.Header.Set("Content-Type", "text/html")
	}))
	page, _ := p.do(p.signed(k, "GET", "/photos/page.html", nil))
	if page.Header().Get("Content-Disposition") != "attachment" || page.Header().Get("Content-Type") != "text/html" {
		t.Errorf("a stored HTML page = %q, %q; want an attachment", page.Header().Get("Content-Disposition"), page.Header().Get("Content-Type"))
	}
	if rec, _ := p.do(p.signed(k, "DELETE", "/photos/dir/cat.jpg", nil)); rec.Code != 204 {
		t.Errorf("DeleteObject = %d", rec.Code)
	}
	if _, ok := p.stored(objectstorage.Oregon, prefix, "dir/cat.jpg"); ok {
		t.Error("the object is still stored after a delete")
	}
	if rec, _ := p.do(p.signed(k, "DELETE", "/photos/dir/cat.jpg", nil)); rec.Code != 204 {
		t.Errorf("DeleteObject of a missing key = %d, want 204 as on S3", rec.Code)
	}
	if rec, doc := p.do(p.signed(k, "GET", "/photos/dir/cat.jpg", nil)); rec.Code != 404 || doc.Code != "NoSuchKey" {
		t.Errorf("GetObject of a missing key = %d %q", rec.Code, doc.Code)
	}
	if rec, doc := p.do(p.signed(k, "GET", "/nope/x", nil)); rec.Code != 404 || doc.Code != "NoSuchBucket" {
		t.Errorf("GetObject in a missing bucket = %d %q", rec.Code, doc.Code)
	}
}

// putChunked sends an object the way the AWS CLI v2 does: aws-chunked, no
// Content-Length, the checksum as a trailer.
func (p *pgRig) putChunked(k testKey, target string, data []byte, trailerAlg, trailerValue string) (*httptest.ResponseRecorder, errorDoc) {
	framed := chunkedBody(data, 8192, "x-amz-checksum-"+trailerAlg, trailerValue)
	return p.do(p.signed(k, "PUT", target, bytes.NewReader(framed), func(r *http.Request) {
		r.ContentLength = -1
		r.Header.Set("X-Amz-Content-Sha256", streamingUnsignedTrailer)
		r.Header.Set("Content-Encoding", "aws-chunked")
		r.Header.Set("X-Amz-Decoded-Content-Length", fmt.Sprint(len(data)))
		r.Header.Set("X-Amz-Trailer", "x-amz-checksum-"+trailerAlg)
	}))
}

func TestAChunkedBodyWithNoContentLengthIsStoredAndItsChecksumEchoed(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	prefix := p.bucket("bkt", objectstorage.Oregon)
	data := payload(120_000)
	for _, alg := range []string{"crc64nvme", "crc32"} {
		rec, doc := p.putChunked(k, "/bkt/"+alg, data, alg, sum(alg, data))
		if rec.Code != 200 || rec.Header().Get("x-amz-checksum-"+alg) != sum(alg, data) {
			t.Fatalf("%s: PutObject = %d %q %s (checksum header %q)", alg, rec.Code, doc.Code, doc.Message, rec.Header().Get("x-amz-checksum-"+alg))
		}
		if got, ok := p.stored(objectstorage.Oregon, prefix, alg); !ok || !bytes.Equal(got, data) {
			t.Errorf("%s: the decoded object is not stored byte for byte", alg)
		}
	}
	// The aws-chunked token is only framing: what is stored reads back with none.
	get, _ := p.do(p.signed(k, "GET", "/bkt/crc32", nil))
	if enc := get.Header().Get("Content-Encoding"); enc != "" {
		t.Errorf("Content-Encoding = %q, want the framing token dropped", enc)
	}
}

func TestAWrongChecksumStoresNothingAndLeavesAPreviousObjectUntouched(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	prefix := p.bucket("bkt", objectstorage.Oregon)
	v1 := payload(50_000)
	if rec, doc := p.putChunked(k, "/bkt/obj", v1, "crc32", sum("crc32", v1)); rec.Code != 200 {
		t.Fatalf("v1 = %d %q", rec.Code, doc.Code)
	}
	v2 := append([]byte(nil), payload(60_000)...)
	for name, send := range map[string]func() (*httptest.ResponseRecorder, errorDoc){
		"a wrong trailer": func() (*httptest.ResponseRecorder, errorDoc) {
			return p.putChunked(k, "/bkt/obj", v2, "crc32", sum("crc32", v1))
		},
		"a wrong crc64nvme trailer": func() (*httptest.ResponseRecorder, errorDoc) {
			return p.putChunked(k, "/bkt/obj", v2, "crc64nvme", sum("crc64nvme", v1))
		},
		"a wrong header checksum": func() (*httptest.ResponseRecorder, errorDoc) {
			return p.do(p.signed(k, "PUT", "/bkt/obj", bytes.NewReader(v2), func(r *http.Request) {
				r.ContentLength = int64(len(v2))
				r.Header.Set("X-Amz-Checksum-Crc32", sum("crc32", v1))
			}))
		},
		"a wrong content-md5": func() (*httptest.ResponseRecorder, errorDoc) {
			return p.do(p.signed(k, "PUT", "/bkt/obj", bytes.NewReader(v2), func(r *http.Request) {
				r.ContentLength = int64(len(v2))
				r.Header.Set("Content-MD5", "AAAAAAAAAAAAAAAAAAAAAA==")
			}))
		},
	} {
		rec, doc := send()
		if rec.Code != 400 || doc.Code != "BadDigest" {
			t.Errorf("%s = %d %q %s, want 400 BadDigest", name, rec.Code, doc.Code, doc.Message)
		}
		if got, ok := p.stored(objectstorage.Oregon, prefix, "obj"); !ok || !bytes.Equal(got, v1) {
			t.Fatalf("%s: the previous object is not byte-identical afterwards", name)
		}
	}
	wrongSHA := sha256.Sum256(v1)
	rec, doc := p.do(p.signed(k, "PUT", "/bkt/new", bytes.NewReader(v2), func(r *http.Request) {
		r.ContentLength = int64(len(v2))
		r.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(wrongSHA[:]))
	}))
	if rec.Code != 400 || doc.Code != "XAmzContentSHA256Mismatch" {
		t.Errorf("an explicit SHA-256 mismatch = %d %q, want 400 XAmzContentSHA256Mismatch", rec.Code, doc.Code)
	}
	if _, ok := p.stored(objectstorage.Oregon, prefix, "new"); ok {
		t.Error("a mismatched put created an object")
	}
	// An unknown trailer is 501 before a byte is read.
	if rec, doc := p.putChunked(k, "/bkt/x", v1, "xxh3", "AAAA"); rec.Code != 501 || doc.Code != "NotImplemented" {
		t.Errorf("an unknown trailer = %d %q, want 501", rec.Code, doc.Code)
	}
	// A body shorter than its declared length stores nothing either.
	short := p.signed(k, "PUT", "/bkt/short", bytes.NewReader(v1[:100]), func(r *http.Request) { r.ContentLength = 1000 })
	short.ContentLength = 1000
	if rec, doc := p.do(short); rec.Code != 400 || doc.Code != "IncompleteBody" {
		t.Errorf("a short body = %d %q, want 400 IncompleteBody", rec.Code, doc.Code)
	}
	if _, ok := p.stored(objectstorage.Oregon, prefix, "short"); ok {
		t.Error("a short body created an object")
	}
}

func TestARangeIsServedPartialAndOnePastTheEndIs416EvenWhereWasabiAnswers200(t *testing.T) {
	for _, quirk := range []bool{false, true} {
		t.Run(fmt.Sprintf("wasabi-quirk=%v", quirk), func(t *testing.T) {
			p := newPGRig(t)
			p.Fakes[objectstorage.Oregon].SetRangeQuirk(quirk)
			k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
			p.bucket("bkt", objectstorage.Oregon)
			data := payload(1000)
			p.do(p.signed(k, "PUT", "/bkt/o", bytes.NewReader(data), func(r *http.Request) { r.ContentLength = 1000 }))
			for _, c := range []struct {
				rng, want string
				from, to  int
			}{
				{"bytes=0-9", "bytes 0-9/1000", 0, 10}, {"bytes=990-", "bytes 990-999/1000", 990, 1000}, {"bytes=-16", "bytes 984-999/1000", 984, 1000},
				{"bytes=500-501", "bytes 500-501/1000", 500, 502}, {"bytes=999-999", "bytes 999-999/1000", 999, 1000},
			} {
				rec, _ := p.do(p.signed(k, "GET", "/bkt/o", nil, func(r *http.Request) { r.Header.Set("Range", c.rng) }))
				if rec.Code != 206 || rec.Header().Get("Content-Range") != c.want || !bytes.Equal(rec.Body.Bytes(), data[c.from:c.to]) {
					t.Errorf("Range %s = %d %q (%d bytes), want 206 %s", c.rng, rec.Code, rec.Header().Get("Content-Range"), rec.Body.Len(), c.want)
				}
			}
			// A resume at bytes=<size>- must not download everything again.
			for _, rng := range []string{"bytes=1000-", "bytes=1500-2000", "bytes=1000-1000"} {
				rec, doc := p.do(p.signed(k, "GET", "/bkt/o", nil, func(r *http.Request) { r.Header.Set("Range", rng) }))
				if rec.Code != 416 || doc.Code != "InvalidRange" || rec.Body.Len() > 1000 && !strings.Contains(rec.Body.String(), "InvalidRange") {
					t.Errorf("Range %s = %d %q, want 416 InvalidRange and not the object", rng, rec.Code, doc.Code)
				}
			}
			// A header that is no single byte range is S3's to ignore: the whole object.
			if rec, _ := p.do(p.signed(k, "GET", "/bkt/o", nil, func(r *http.Request) { r.Header.Set("Range", "bytes=0-1,5-6") })); rec.Code != 200 || rec.Body.Len() != 1000 {
				t.Errorf("a multi-range = %d with %d bytes, want the whole object", rec.Code, rec.Body.Len())
			}
		})
	}
}

func TestConditionalReadsAnswer304And412AsS3Does(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("bkt", objectstorage.Oregon)
	rec, _ := p.do(p.signed(k, "PUT", "/bkt/o", strings.NewReader("hello"), func(r *http.Request) { r.ContentLength = 5 }))
	etag := rec.Header().Get("ETag")
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	for _, c := range []struct {
		name, header, value string
		want                int
	}{
		{"If-None-Match matches", "If-None-Match", etag, 304}, {"If-None-Match differs", "If-None-Match", `"other"`, 200},
		{"If-Match matches", "If-Match", etag, 200}, {"If-Match differs", "If-Match", `"other"`, 412},
		{"If-Modified-Since in the future", "If-Modified-Since", future, 304}, {"If-Modified-Since in the past", "If-Modified-Since", past, 200},
		{"If-Unmodified-Since in the past", "If-Unmodified-Since", past, 412}, {"If-Unmodified-Since in the future", "If-Unmodified-Since", future, 200},
	} {
		get, _ := p.do(p.signed(k, "GET", "/bkt/o", nil, func(r *http.Request) { r.Header.Set(c.header, c.value) }))
		head, _ := p.do(p.signed(k, "HEAD", "/bkt/o", nil, func(r *http.Request) { r.Header.Set(c.header, c.value) }))
		if get.Code != c.want || head.Code != c.want {
			t.Errorf("%s: GET %d, HEAD %d, want %d", c.name, get.Code, head.Code, c.want)
		}
		if c.want == 304 && get.Body.Len() != 0 {
			t.Errorf("%s: a 304 carries a body", c.name)
		}
	}
}

func TestCopyObjectRunsInsideTheStoreKeepsOrReplacesMetadataAndNeedsTheSourceRead(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	src := p.bucket("src", objectstorage.Oregon)
	dst := p.bucket("dst", objectstorage.Oregon)
	p.bucket("eu-data", objectstorage.Frankfurt)
	data := payload(70_000)
	p.do(p.signed(k, "PUT", "/src/a/o.bin", bytes.NewReader(data), func(r *http.Request) {
		r.ContentLength = int64(len(data))
		r.Header.Set("Content-Type", "application/x-thing")
		r.Header.Set("x-amz-meta-mtime", "42")
	}))
	copyTo := func(target, source string, mutate ...func(*http.Request)) (*httptest.ResponseRecorder, errorDoc) {
		return p.do(p.signed(k, "PUT", target, nil, append([]func(*http.Request){func(r *http.Request) {
			r.ContentLength = 0
			r.Header.Set("x-amz-copy-source", source)
		}}, mutate...)...))
	}
	rec, doc := copyTo("/dst/copy.bin", "/src/a/o.bin")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<CopyObjectResult") {
		t.Fatalf("CopyObject = %d %q %s", rec.Code, doc.Code, rec.Body)
	}
	if got, ok := p.stored(objectstorage.Oregon, dst, "copy.bin"); !ok || !bytes.Equal(got, data) {
		t.Fatal("the copy is not the source, byte for byte")
	}
	head, _ := p.do(p.signed(k, "HEAD", "/dst/copy.bin", nil))
	if head.Header().Get("Content-Type") != "application/x-thing" || head.Header().Get("x-amz-meta-mtime") != "42" {
		t.Errorf("COPY lost the source's headers: %v", head.Header())
	}
	rec, _ = copyTo("/dst/replaced.bin", "src/a/o.bin", func(r *http.Request) {
		r.Header.Set("x-amz-metadata-directive", "REPLACE")
		r.Header.Set("Content-Type", "text/plain")
		r.Header.Set("x-amz-meta-mtime", "99")
	})
	head, _ = p.do(p.signed(k, "HEAD", "/dst/replaced.bin", nil))
	if rec.Code != 200 || head.Header().Get("Content-Type") != "text/plain" || head.Header().Get("x-amz-meta-mtime") != "99" {
		t.Errorf("REPLACE = %d, headers %v", rec.Code, head.Header())
	}
	if rec, doc := copyTo("/src/a/o.bin", "/src/a/o.bin"); rec.Code != 400 || doc.Code != "InvalidRequest" {
		t.Errorf("a copy onto itself = %d %q, want 400 InvalidRequest", rec.Code, doc.Code)
	}
	if rec, _ := copyTo("/src/a/o.bin", "/src/a/o.bin", func(r *http.Request) { r.Header.Set("x-amz-metadata-directive", "REPLACE") }); rec.Code != 200 {
		t.Errorf("a copy onto itself that replaces the metadata = %d, want 200", rec.Code)
	}
	if rec, doc := copyTo("/dst/x", "/src/missing"); rec.Code != 404 || doc.Code != "NoSuchKey" {
		t.Errorf("a missing source = %d %q, want 404 NoSuchKey", rec.Code, doc.Code)
	}
	if rec, doc := copyTo("/dst/cond-ok", "/src/a/o.bin", func(r *http.Request) {
		r.Header.Set("x-amz-copy-source-if-match", head.Header().Get("ETag"))
	}); rec.Code != 200 {
		t.Errorf("a copy whose source meets If-Match = %d %q, want 200", rec.Code, doc.Code)
	}
	if rec, doc := copyTo("/dst/cond-no", "/src/a/o.bin", func(r *http.Request) {
		r.Header.Set("x-amz-copy-source-if-match", `"00000000000000000000000000000000"`)
	}); rec.Code != 412 || doc.Code != "PreconditionFailed" {
		t.Errorf("a copy whose source fails If-Match = %d %q, want 412 PreconditionFailed", rec.Code, doc.Code)
	}
	if _, ok := p.stored(objectstorage.Oregon, dst, "cond-no"); ok {
		t.Error("a copy that failed its condition stored an object")
	}
	if rec, doc := copyTo("/dst/x", "/nope/a"); rec.Code != 404 || doc.Code != "NoSuchBucket" {
		t.Errorf("a missing source bucket = %d %q", rec.Code, doc.Code)
	}
	if rec, doc := copyTo("/eu-data/x", "/src/a/o.bin"); rec.Code != 501 || doc.Code != "NotImplemented" {
		t.Errorf("a copy across regions = %d %q, want 501", rec.Code, doc.Code)
	}
	if rec, doc := copyTo("/dst/x", "/src/a/o.bin?versionId=1"); rec.Code != 501 {
		t.Errorf("a copy of a version = %d %q, want 501", rec.Code, doc.Code)
	}
	if rec, doc := copyTo("/dst/x", "/src/a/../o.bin"); rec.Code != 400 || doc.Code != "InvalidArgument" {
		t.Errorf("a copy source with .. = %d %q, want 400", rec.Code, doc.Code)
	}
	// Reading the source is the creator's can_view_sensitive: without it nothing is copied.
	p.authz.held = map[string]bool{core.RelCanView: true, core.RelCanCreate: true}
	before := len(p.Fakes[objectstorage.Oregon].Keys(objectstorage.Oregon.Bucket(), ""))
	rec, doc = copyTo("/dst/denied.bin", "/src/a/o.bin")
	if rec.Code != 403 || doc.Code != "AccessDenied" || len(p.Fakes[objectstorage.Oregon].Keys(objectstorage.Oregon.Bucket(), "")) != before {
		t.Errorf("a copy by a creator who cannot read the source = %d %q, objects %d -> %d", rec.Code, doc.Code, before, len(p.Fakes[objectstorage.Oregon].Keys(objectstorage.Oregon.Bucket(), "")))
	}
	_ = src
}

func TestDeleteObjectsRemovesABatchAndReportsWhatItCouldNot(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	prefix := p.bucket("bkt", objectstorage.Oregon)
	for _, key := range []string{"a", "b", "c", "keep"} {
		p.do(p.signed(k, "PUT", "/bkt/"+key, strings.NewReader(key), func(r *http.Request) { r.ContentLength = int64(len(key)) }))
	}
	doc := `<Delete><Object><Key>a</Key></Object><Object><Key>b</Key></Object><Object><Key>missing</Key></Object><Object><Key>../escape</Key></Object></Delete>`
	rec, e := p.call(k, "POST", "/bkt?delete", doc)
	if rec.Code != 200 {
		t.Fatalf("DeleteObjects = %d %q", rec.Code, e.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"<Deleted><Key>a</Key></Deleted>", "<Deleted><Key>b</Key></Deleted>", "<Deleted><Key>missing</Key></Deleted>", "<Error><Key>../escape</Key><Code>InvalidArgument</Code>"} {
		if !strings.Contains(body, want) {
			t.Errorf("DeleteResult lacks %s: %s", want, body)
		}
	}
	for key, want := range map[string]bool{"a": false, "b": false, "c": true, "keep": true} {
		if _, ok := p.stored(objectstorage.Oregon, prefix, key); ok != want {
			t.Errorf("after the batch, %q stored = %v, want %v", key, ok, want)
		}
	}
	quiet, _ := p.call(k, "POST", "/bkt?delete", `<Delete><Quiet>true</Quiet><Object><Key>c</Key></Object></Delete>`)
	if quiet.Code != 200 || strings.Contains(quiet.Body.String(), "<Deleted>") {
		t.Errorf("a quiet delete = %d %s, want no Deleted entries", quiet.Code, quiet.Body)
	}
	var many strings.Builder
	many.WriteString("<Delete>")
	for i := 0; i < 1001; i++ {
		fmt.Fprintf(&many, "<Object><Key>k%d</Key></Object>", i)
	}
	many.WriteString("</Delete>")
	if rec, e := p.call(k, "POST", "/bkt?delete", many.String()); rec.Code != 400 || e.Code != "MalformedXML" {
		t.Errorf("1,001 keys = %d %q, want 400 MalformedXML", rec.Code, e.Code)
	}
	if rec, e := p.call(k, "POST", "/bkt?delete", `<Delete><Object><Key>a</Key><VersionId>1</VersionId></Object></Delete>`); rec.Code != 501 {
		t.Errorf("a version id = %d %q, want 501", rec.Code, e.Code)
	}
}

func TestObjectTaggingReadsAsAnEmptySetAndMetadataIsBounded(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("bkt", objectstorage.Oregon)
	p.do(p.signed(k, "PUT", "/bkt/o", strings.NewReader("x"), func(r *http.Request) { r.ContentLength = 1 }))
	rec, _ := p.call(k, "GET", "/bkt/o?tagging", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<Tagging") || strings.Contains(rec.Body.String(), "<Tag>") {
		t.Errorf("GetObjectTagging = %d %s, want an empty tag set", rec.Code, rec.Body)
	}
	if rec, e := p.call(k, "GET", "/bkt/missing?tagging", ""); rec.Code != 404 || e.Code != "NoSuchKey" {
		t.Errorf("GetObjectTagging of a missing key = %d %q", rec.Code, e.Code)
	}
	big := strings.Repeat("v", 2100)
	if rec, e := p.do(p.signed(k, "PUT", "/bkt/big-meta", strings.NewReader("x"), func(r *http.Request) {
		r.ContentLength = 1
		r.Header.Set("x-amz-meta-big", big)
	})); rec.Code != 400 || e.Code != "MetadataTooLarge" {
		t.Errorf("2,100 bytes of metadata = %d %q, want 400 MetadataTooLarge", rec.Code, e.Code)
	}
}

func TestAGetIsAuditedOncePerKeyBucketAndKeyAndDenialsOncePerWindow(t *testing.T) {
	p := newPGRig(t)
	rw := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	ro := p.newKey(objectstorage.KeyPermissionRead, 0)
	p.bucket("bkt", objectstorage.Oregon)
	data := payload(5000)
	p.do(p.signed(rw, "PUT", "/bkt/o", bytes.NewReader(data), func(r *http.Request) { r.ContentLength = 5000 }))
	p.do(p.signed(rw, "PUT", "/bkt/other", bytes.NewReader(data), func(r *http.Request) { r.ContentLength = 5000 }))
	views := func() int { return len(p.Audit.byVerb(core.AuditVerbGetObject)) }
	// `aws s3 cp` reads in ranges, ten at a time: ten ranged gets are one row.
	for i := 0; i < 10; i++ {
		p.do(p.signed(rw, "GET", "/bkt/o", nil, func(r *http.Request) { r.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", i*100, i*100+99)) }))
	}
	if views() != 1 {
		t.Fatalf("ten ranged gets wrote %d rows, want 1", views())
	}
	row := p.Audit.byVerb(core.AuditVerbGetObject)[0]
	if row.Target != "object:bkt/o" || row.Caller != rw.id || row.CallerMethod != core.MethodS3Key {
		t.Errorf("the download row = %+v, want object:bkt/o by key %s", row, rw.id)
	}
	// A HEAD, a missing object and a missing bucket record no success.
	p.do(p.signed(rw, "HEAD", "/bkt/o", nil))
	p.do(p.signed(rw, "GET", "/bkt/missing", nil))
	p.do(p.signed(rw, "GET", "/nope/o", nil))
	if views() != 1 {
		t.Errorf("a head, a miss and a missing bucket wrote rows: %d", views())
	}
	// Another object and another key are rows of their own; after the window, another.
	p.do(p.signed(rw, "GET", "/bkt/other", nil))
	p.do(p.signed(ro, "GET", "/bkt/o", nil))
	if views() != 3 {
		t.Errorf("a second object and a second key wrote %d rows in all, want 3", views())
	}
	p.now = p.now.Add(AuditWindow + time.Second)
	p.gw.cfg.Now = func() time.Time { return p.now }
	p.do(p.signed(rw, "GET", "/bkt/o", nil))
	if views() != 4 {
		t.Errorf("a get after the window wrote %d rows in all, want 4", views())
	}
	// A leaked read key's denied puts are one row per key, verb and bucket per window.
	for i := 0; i < 50; i++ {
		p.do(p.signed(ro, "PUT", fmt.Sprintf("/bkt/denied%d", i), strings.NewReader("x"), func(r *http.Request) { r.ContentLength = 1 }))
	}
	denied := 0
	for _, r := range p.Audit.byVerb(objectstorage.AuditVerbPutObject) {
		if r.Outcome == core.AuditDenied {
			denied++
		}
	}
	// The ceiling refuses a read key before OpenFGA; its denial is still a row, once.
	if denied != 1 {
		t.Errorf("50 denied puts wrote %d rows, want 1 per window", denied)
	}
	// An authorization the creator's role refuses is a denied row, once per window.
	p.authz.held = map[string]bool{core.RelCanView: true, core.RelCanViewSensitive: true}
	for i := 0; i < 50; i++ {
		p.do(p.signed(rw, "PUT", fmt.Sprintf("/bkt/refused%d", i), strings.NewReader("x"), func(r *http.Request) { r.ContentLength = 1 }))
	}
	denied = 0
	for _, r := range p.Audit.byVerb(objectstorage.AuditVerbPutObject) {
		if r.Outcome == core.AuditDenied && r.Caller == rw.id {
			denied++
		}
	}
	if denied != 1 {
		t.Errorf("50 puts refused by the creator's role wrote %d denied rows, want 1 per window", denied)
	}
	_ = io.EOF
}

func TestACopyRowNamesItsSourceAndABatchDeleteIsOneRow(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("src", objectstorage.Oregon)
	p.bucket("dst", objectstorage.Oregon)
	p.do(p.signed(k, "PUT", "/src/o", strings.NewReader("x"), func(r *http.Request) { r.ContentLength = 1 }))
	p.do(p.signed(k, "PUT", "/dst/c", nil, func(r *http.Request) { r.ContentLength = 0; r.Header.Set("x-amz-copy-source", "/src/o") }))
	rows := p.Audit.byVerb(objectstorage.AuditVerbCopyObject)
	if len(rows) != 1 || rows[0].Target != "object:dst/c" || !strings.Contains(rows[0].TargetName, "object:src/o") {
		t.Fatalf("copy rows = %+v, want one naming dst/c and its source src/o", rows)
	}
	if puts := p.Audit.byVerb(objectstorage.AuditVerbPutObject); len(puts) != 1 {
		t.Errorf("a copy also wrote a PutObject row: %d put rows, want only the plain put's", len(puts))
	}
	p.call(k, "POST", "/src?delete", `<Delete><Object><Key>o</Key></Object><Object><Key>p</Key></Object><Object><Key>q</Key></Object></Delete>`)
	del := p.Audit.byVerb(objectstorage.AuditVerbDeleteObjects)
	if len(del) != 1 || del[0].Target != "object:src/o" || del[0].TargetName != "3 keys" {
		t.Errorf("batch delete rows = %+v, want one naming the bucket, the first key and 3 keys", del)
	}
	// No row ever carries a query string, a header or a signature.
	for _, r := range p.Audit.rows {
		for _, field := range []string{r.Target, r.TargetName, r.Resource} {
			if strings.Contains(field, "X-Amz-") || strings.Contains(field, "Signature") || strings.Contains(field, "?") {
				t.Errorf("an audit row carries a signed query or header text: %+v", r)
			}
		}
	}
}
