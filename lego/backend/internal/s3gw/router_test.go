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
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// relationOf is ADR112 D6 as a table: what each S3 operation asks of the key's
// creator on the workspace. A new route is a decision, not a default.
var relationOf = map[string]string{
	"ListBuckets": core.RelCanView, "HeadBucket": core.RelCanView, "GetBucketLocation": core.RelCanView, "GetBucketCors": core.RelCanView,
	"GetBucketLifecycleConfiguration": core.RelCanView, "ListObjects": core.RelCanView, "ListObjectsV2": core.RelCanView,
	"ListMultipartUploads": core.RelCanView, "HeadObject": core.RelCanView, "GetObjectTagging": core.RelCanView, "ListParts": core.RelCanView,
	"GetObject":    core.RelCanViewSensitive,
	"CreateBucket": core.RelCanCreate, "DeleteBucket": core.RelCanCreate, "PutBucketCors": core.RelCanCreate, "DeleteBucketCors": core.RelCanCreate,
	"PutBucketLifecycleConfiguration": core.RelCanCreate, "DeleteBucketLifecycle": core.RelCanCreate, "DeleteObjects": core.RelCanCreate,
	"PutObject": core.RelCanCreate, "DeleteObject": core.RelCanCreate, "CreateMultipartUpload": core.RelCanCreate, "UploadPart": core.RelCanCreate,
	"CompleteMultipartUpload": core.RelCanCreate, "AbortMultipartUpload": core.RelCanCreate,
}

func TestEveryRouteAsksForExactlyTheRelationADR112D6Names(t *testing.T) {
	seen := map[string]bool{}
	for _, rt := range routes {
		want, ok := relationOf[rt.op]
		if !ok {
			t.Errorf("route %s has no entry in relationOf: decide what it needs (ADR112 D6)", rt.op)
			continue
		}
		seen[rt.op] = true
		if rt.relation != want {
			t.Errorf("%s asks for %s, want %s", rt.op, rt.relation, want)
		}
		if rt.relation == core.RelCanManage {
			t.Errorf("%s asks for can_manage, which no key holds", rt.op)
		}
		if !strings.HasPrefix(rt.verb, "objectstorage.") {
			t.Errorf("%s records the verb %q, want an objectstorage.<Action> verb", rt.op, rt.verb)
		}
		// Whatever stores bytes or rules is refused the unimplemented headers.
		stores := rt.relation == core.RelCanCreate && (rt.method == http.MethodPut || rt.method == http.MethodPost)
		if stores != rt.write {
			t.Errorf("%s: write = %v, but it %s", rt.op, rt.write, map[bool]string{true: "stores", false: "does not store"}[stores])
		}
	}
	for op := range relationOf {
		if !seen[op] {
			t.Errorf("relationOf lists %s, which is not a route", op)
		}
	}
}

func TestSubresourcesAreMatchedFirstSoNoUnsupportedOneReachesAnObjectOrBucketHandler(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionReadWrite, 0)
	reached := map[string]bool{}
	for _, rt := range routes {
		op := rt.op
		spyOn(t, op, func(*request) *Error { reached[op] = true; return nil })
	}
	for _, c := range []struct{ method, target string }{
		// A tag write must never be a put, and a tag delete never a delete.
		{"PUT", "/b/k?tagging"}, {"DELETE", "/b/k?tagging"}, {"PUT", "/b?tagging"}, {"GET", "/b?tagging"},
		{"PUT", "/b/k?acl"}, {"GET", "/b/k?acl"}, {"GET", "/b?acl"}, {"PUT", "/b?acl"},
		{"PUT", "/b?policy"}, {"GET", "/b?policy"}, {"DELETE", "/b?policy"},
		{"GET", "/b?versioning"}, {"PUT", "/b?versioning"}, {"GET", "/b?versions"}, {"GET", "/b/k?versionId=1"}, {"DELETE", "/b/k?versionId=1"},
		{"PUT", "/b/k?retention"}, {"PUT", "/b/k?legal-hold"}, {"PUT", "/b?object-lock"}, {"GET", "/b?encryption"}, {"PUT", "/b?replication"},
		{"PUT", "/b?notification"}, {"PUT", "/b?website"}, {"PUT", "/b?accelerate"}, {"POST", "/b/k?select&select-type=2"}, {"POST", "/b/k?restore"},
		{"GET", "/b/k?torrent"}, {"PUT", "/b?logging"}, {"PUT", "/b?ownershipControls"}, {"PUT", "/b?publicAccessBlock"},
		// A browser form upload to a bucket, and a POST that is not a batch delete.
		{"POST", "/b"}, {"POST", "/b/k"},
		// An override or a parameter bex has no meaning for is never silently ignored.
		{"GET", "/b/k?response-content-type=text/html"}, {"GET", "/b/k?response-content-disposition=inline"}, {"GET", "/b?list-type=1"},
		{"GET", "/b?max-keys=1&unknown-param=1"}, {"PUT", "/b/k?unknown-param=1"},
		// A multipart subresource on the wrong level or verb is not another operation.
		{"GET", "/b/k?uploads"}, {"PUT", "/b/k?uploads"}, {"DELETE", "/b/k?uploads"}, {"PUT", "/b/k?uploadId=x"}, {"GET", "/b/k?partNumber=1"},
		{"PUT", "/?cors"},
	} {
		rec, doc := g.do(g.signed(k, c.method, c.target, nil))
		if rec.Code != http.StatusNotImplemented || doc.Code != "NotImplemented" {
			t.Errorf("%s %s = %d %q, want 501 NotImplemented", c.method, c.target, rec.Code, doc.Code)
		}
	}
	if len(reached) != 0 {
		t.Errorf("an unsupported subresource reached handlers %v", reached)
	}
}

func TestTheOperationsBexServesAreMatchedByMethodAndSubresource(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionReadWrite, 0)
	var got string
	for _, rt := range routes {
		op := rt.op
		spyOn(t, op, func(*request) *Error { got = op; return nil })
	}
	for _, c := range []struct{ method, target, want string }{
		{"GET", "/", "ListBuckets"},
		{"PUT", "/b", "CreateBucket"}, {"HEAD", "/b", "HeadBucket"}, {"DELETE", "/b", "DeleteBucket"},
		{"GET", "/b?location", "GetBucketLocation"}, {"GET", "/b?cors", "GetBucketCors"}, {"PUT", "/b?cors", "PutBucketCors"}, {"DELETE", "/b?cors", "DeleteBucketCors"},
		{"GET", "/b?lifecycle", "GetBucketLifecycleConfiguration"}, {"PUT", "/b?lifecycle", "PutBucketLifecycleConfiguration"}, {"DELETE", "/b?lifecycle", "DeleteBucketLifecycle"},
		{"GET", "/b", "ListObjects"}, {"GET", "/b?prefix=a&delimiter=%2F&max-keys=5&marker=m&encoding-type=url", "ListObjects"},
		{"GET", "/b?list-type=2&prefix=a&continuation-token=t&start-after=s&fetch-owner=true", "ListObjectsV2"},
		{"GET", "/b?uploads&prefix=a&key-marker=k&upload-id-marker=u&max-uploads=3", "ListMultipartUploads"},
		{"POST", "/b?delete", "DeleteObjects"},
		{"PUT", "/b/k", "PutObject"}, {"GET", "/b/k", "GetObject"}, {"HEAD", "/b/k", "HeadObject"}, {"DELETE", "/b/k", "DeleteObject"},
		{"GET", "/b/k?tagging", "GetObjectTagging"}, {"POST", "/b/k?uploads", "CreateMultipartUpload"},
		{"PUT", "/b/k?partNumber=1&uploadId=u", "UploadPart"}, {"POST", "/b/k?uploadId=u", "CompleteMultipartUpload"},
		{"DELETE", "/b/k?uploadId=u", "AbortMultipartUpload"}, {"GET", "/b/k?uploadId=u&max-parts=2&part-number-marker=1", "ListParts"},
		// The SDKs of Go and JS add x-id to every request.
		{"GET", "/b/k?x-id=GetObject", "GetObject"}, {"PUT", "/b/k?x-id=PutObject", "PutObject"}, {"GET", "/b?list-type=2&x-id=ListObjectsV2", "ListObjectsV2"},
		// A key may contain slashes, a trailing one included, and a bucket path may end in one.
		{"GET", "/b/", "ListObjects"}, {"GET", "/b/a/b/c.txt", "GetObject"}, {"PUT", "/b/dir/", "PutObject"},
	} {
		got = ""
		rec, doc := g.do(g.signed(k, c.method, c.target, nil))
		if rec.Code != http.StatusOK || got != c.want {
			t.Errorf("%s %s = %d %q, handler %q; want %q", c.method, c.target, rec.Code, doc.Code, got, c.want)
		}
	}
	if rec, doc := g.do(g.signed(k, "PATCH", "/b", nil)); rec.Code != http.StatusMethodNotAllowed || doc.Code != "MethodNotAllowed" {
		t.Errorf("PATCH /b = %d %q, want 405 MethodNotAllowed", rec.Code, doc.Code)
	}
}

func TestAWriteWithAnUnimplementedHeaderIsRefusedAndNeverIgnored(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionReadWrite, 0)
	ran := false
	for _, op := range []string{"PutObject", "CreateMultipartUpload", "UploadPart", "CreateBucket"} {
		spyOn(t, op, func(*request) *Error { ran = true; return nil })
	}
	refused := []struct{ method, target, header, value string }{
		{"PUT", "/b/k", "If-None-Match", "*"}, // Terraform's state lock
		{"PUT", "/b/k", "If-Match", `"etag"`},
		{"PUT", "/b/k", "x-amz-tagging", "a=b"},
		{"PUT", "/b/k", "x-amz-copy-source-if-match", `"e"`},
		{"PUT", "/b/k", "x-amz-copy-source-if-modified-since", "Sat, 01 Jan 2000 00:00:00 GMT"},
		{"PUT", "/b/k", "x-amz-storage-class", "GLACIER"},
		{"PUT", "/b/k", "x-amz-server-side-encryption", "AES256"},
		{"PUT", "/b/k", "x-amz-server-side-encryption-aws-kms-key-id", "k"},
		{"PUT", "/b/k", "x-amz-server-side-encryption-customer-algorithm", "AES256"},
		{"PUT", "/b/k", "x-amz-acl", "public-read"},
		{"PUT", "/b/k", "x-amz-grant-read", "id=1"},
		{"PUT", "/b/k", "x-amz-object-lock-mode", "GOVERNANCE"},
		{"PUT", "/b/k", "x-amz-website-redirect-location", "/x"},
		{"POST", "/b/k?uploads", "x-amz-storage-class", "STANDARD_IA"},
		{"PUT", "/b/k?partNumber=1&uploadId=u", "If-None-Match", "*"},
		{"PUT", "/b", "x-amz-acl", "public-read"},
		{"PUT", "/b", "x-amz-bucket-object-lock-enabled", "true"},
	}
	for _, c := range refused {
		rec, doc := g.do(g.signed(k, c.method, c.target, nil, func(r *http.Request) { r.Header.Set(c.header, c.value) }))
		if rec.Code != http.StatusNotImplemented || doc.Code != "NotImplemented" {
			t.Errorf("%s %s with %s: %s = %d %q, want 501", c.method, c.target, c.header, c.value, rec.Code, doc.Code)
		}
	}
	if ran {
		t.Error("a write with an unimplemented header reached its handler")
	}
	// What is implemented, or harmless, passes.
	for _, c := range []struct{ method, target, header, value string }{
		{"PUT", "/b/k", "x-amz-storage-class", "STANDARD"}, {"PUT", "/b/k", "x-amz-acl", "private"},
		{"PUT", "/b/k", "x-amz-meta-mtime", "1700000000"}, {"PUT", "/b/k", "x-amz-sdk-checksum-algorithm", "CRC32"},
		{"PUT", "/b/k", "x-amz-user-agent", "aws-sdk"},
	} {
		ran = false
		rec, doc := g.do(g.signed(k, c.method, c.target, nil, func(r *http.Request) { r.Header.Set(c.header, c.value) }))
		if rec.Code != http.StatusOK || !ran {
			t.Errorf("%s %s with %s: %s = %d %q (handler ran: %v), want it served", c.method, c.target, c.header, c.value, rec.Code, doc.Code, ran)
		}
	}
	// A conditional READ is S3's own and bex serves it: only writes refuse.
	spyOn(t, "GetObject", func(*request) *Error { ran = true; return nil })
	ran = false
	if rec, _ := g.do(g.signed(k, "GET", "/b/k", nil, func(r *http.Request) { r.Header.Set("If-None-Match", `"e"`) })); rec.Code != 200 || !ran {
		t.Errorf("a conditional GET = %d (ran %v), want it served", rec.Code, ran)
	}
}

func TestPathsSplitIntoABucketAndAKeyAsPathStyleS3DoesIt(t *testing.T) {
	for _, c := range []struct{ path, bucket, key string }{
		{"/", "", ""}, {"/b", "b", ""}, {"/b/", "b", ""}, {"/b/k", "b", "k"}, {"/b/a/b/", "b", "a/b/"}, {"/b/a//b", "b", "a//b"}, {"/b/é", "b", "é"},
	} {
		if bucket, key := splitPath(c.path); bucket != c.bucket || key != c.key {
			t.Errorf("splitPath(%q) = %q, %q; want %q, %q", c.path, bucket, key, c.bucket, c.key)
		}
	}
	// The subresource list never drops a route's own parameter.
	for _, rt := range routes {
		if rt.sub != "" && !slices.Contains(unsupportedSubresources, rt.sub) {
			t.Errorf("subresource %q of %s is missing from unsupportedSubresources, so a wrong-verb request for it would not be a 501", rt.sub, rt.op)
		}
	}
}
