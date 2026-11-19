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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/s3client"
)

// The AWS SDK for Go v2 against bex-s3 (ADR112 D11, layer 1): a real client library, its
// defaults untouched (request checksums included), over a socket, on real Postgres and a fake
// Wasabi. What the SDK does here is what any Go program using it does.

type sdkRig struct {
	*pgRig
	c   *s3.Client
	k   testKey
	srv *httptest.Server
}

func newSDKRig(t *testing.T) *sdkRig {
	t.Helper()
	p := newWideRig(t)
	p.gw.cfg.Now = time.Now
	p.keys.Now = time.Now
	srv := httptest.NewServer(p.gw)
	t.Cleanup(srv.Close)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	return &sdkRig{pgRig: p, k: k, srv: srv, c: s3client.New(s3client.Config{Endpoint: srv.URL, Region: "oregon", AccessKey: k.id, SecretKey: k.secret, MaxAttempts: 1})}
}

// code is the S3 error code of an SDK error and its HTTP status.
func code(err error) (string, int) {
	if err == nil {
		return "", 0
	}
	var coded interface{ ErrorCode() string }
	var resp *awshttp.ResponseError
	c, status := "", 0
	if errors.As(err, &coded) {
		c = coded.ErrorCode()
	}
	if errors.As(err, &resp) {
		status = resp.HTTPStatusCode()
	}
	return c, status
}

func wantCode(t *testing.T, what string, err error, wantCode string, wantStatus int) {
	t.Helper()
	if got, status := code(err); got != wantCode || (wantStatus != 0 && status != wantStatus) {
		t.Errorf("%s = %q (HTTP %d), want %s (%d); err %v", what, got, status, wantCode, wantStatus, err)
	}
}

func (r *sdkRig) put(bucket, key string, data []byte) {
	r.t.Helper()
	if _, err := r.c.PutObject(context.Background(), &s3.PutObjectInput{Bucket: &bucket, Key: &key, Body: bytes.NewReader(data)}); err != nil {
		r.t.Fatalf("PutObject %s/%s: %v", bucket, key, err)
	}
}

func (r *sdkRig) get(bucket, key string) ([]byte, error) {
	out, err := r.c.GetObject(context.Background(), &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func ptr[T any](v T) *T { return &v }

func TestSDKPutGetRangeHeadAndMetadataWithDefaultChecksums(t *testing.T) {
	r := newSDKRig(t)
	ctx := context.Background()
	r.bucket("sdk", objectstorage.Oregon)
	data := salted(100_000, 9)
	if _, err := r.c.PutObject(ctx, &s3.PutObjectInput{
		Bucket: ptr("sdk"), Key: ptr("dir/o.bin"), Body: bytes.NewReader(data), ContentType: ptr("application/x-sdk"),
		Metadata: map[string]string{"mtime": "42", "owner": "alice"}, CacheControl: ptr("max-age=60"),
	}); err != nil {
		t.Fatalf("PutObject with the SDK's default checksum: %v", err)
	}
	head, err := r.c.HeadObject(ctx, &s3.HeadObjectInput{Bucket: ptr("sdk"), Key: ptr("dir/o.bin")})
	if err != nil || *head.ContentLength != int64(len(data)) || *head.ContentType != "application/x-sdk" || head.Metadata["mtime"] != "42" || head.Metadata["owner"] != "alice" || *head.CacheControl != "max-age=60" {
		t.Fatalf("HeadObject = %+v, %v", head, err)
	}
	if got, err := r.get("sdk", "dir/o.bin"); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("GetObject: %d bytes, %v", len(got), err)
	}
	part, err := r.c.GetObject(ctx, &s3.GetObjectInput{Bucket: ptr("sdk"), Key: ptr("dir/o.bin"), Range: ptr("bytes=10-19")})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(part.Body)
	part.Body.Close()
	if !bytes.Equal(body, data[10:20]) || *part.ContentRange != "bytes 10-19/100000" {
		t.Errorf("a ranged get = %q range %v", body, part.ContentRange)
	}
	_, err = r.c.GetObject(ctx, &s3.GetObjectInput{Bucket: ptr("sdk"), Key: ptr("dir/o.bin"), Range: ptr("bytes=200000-")})
	wantCode(t, "a range past the end", err, "InvalidRange", 416)
	// An explicit checksum the body does not have stores nothing and leaves the object.
	_, err = r.c.PutObject(ctx, &s3.PutObjectInput{Bucket: ptr("sdk"), Key: ptr("dir/o.bin"), Body: bytes.NewReader([]byte("replacement")), ChecksumSHA256: ptr("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")})
	wantCode(t, "a put with a wrong SHA-256", err, "BadDigest", 400)
	if got, _ := r.get("sdk", "dir/o.bin"); !bytes.Equal(got, data) {
		t.Error("a put that failed its checksum changed the stored object")
	}
	for _, alg := range []types.ChecksumAlgorithm{types.ChecksumAlgorithmCrc32, types.ChecksumAlgorithmCrc32c, types.ChecksumAlgorithmCrc64nvme, types.ChecksumAlgorithmSha1, types.ChecksumAlgorithmSha256} {
		if _, err := r.c.PutObject(ctx, &s3.PutObjectInput{Bucket: ptr("sdk"), Key: ptr("alg/" + string(alg)), Body: bytes.NewReader(data), ChecksumAlgorithm: alg}); err != nil {
			t.Errorf("PutObject with %s: %v", alg, err)
		}
	}
	_, err = r.c.GetObject(ctx, &s3.GetObjectInput{Bucket: ptr("sdk"), Key: ptr("missing")})
	wantCode(t, "a missing key", err, "NoSuchKey", 404)
}

func TestSDKMultipartCopyAndPartCopy(t *testing.T) {
	r := newSDKRig(t)
	ctx := context.Background()
	r.bucket("src", objectstorage.Oregon)
	r.bucket("dst", objectstorage.Oregon)
	parts := [][]byte{salted(5*mib, 1), salted(5*mib, 2), salted(2048, 3)}
	up, err := r.c.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{Bucket: ptr("src"), Key: ptr("big.bin"), ContentType: ptr("application/x-big")})
	if err != nil {
		t.Fatal(err)
	}
	var done []types.CompletedPart
	for i, p := range parts {
		out, err := r.c.UploadPart(ctx, &s3.UploadPartInput{Bucket: ptr("src"), Key: ptr("big.bin"), UploadId: up.UploadId, PartNumber: ptr(int32(i + 1)), Body: bytes.NewReader(p)})
		if err != nil {
			t.Fatalf("UploadPart %d: %v", i+1, err)
		}
		done = append(done, types.CompletedPart{PartNumber: ptr(int32(i + 1)), ETag: out.ETag})
	}
	listed, err := r.c.ListParts(ctx, &s3.ListPartsInput{Bucket: ptr("src"), Key: ptr("big.bin"), UploadId: up.UploadId, MaxParts: ptr(int32(2))})
	if err != nil || len(listed.Parts) != 2 || !*listed.IsTruncated || *listed.NextPartNumberMarker != "2" {
		t.Fatalf("ListParts max 2 = %+v, %v", listed, err)
	}
	if _, err := r.c.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{Bucket: ptr("src"), Key: ptr("big.bin"), UploadId: up.UploadId, MultipartUpload: &types.CompletedMultipartUpload{Parts: done}}); err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}
	whole := bytes.Join(parts, nil)
	if got, err := r.get("src", "big.bin"); err != nil || !bytes.Equal(got, whole) {
		t.Fatalf("the assembled object: %d bytes, %v", len(got), err)
	}
	if head, _ := r.c.HeadObject(ctx, &s3.HeadObjectInput{Bucket: ptr("src"), Key: ptr("big.bin")}); head == nil || *head.ContentType != "application/x-big" {
		t.Errorf("the upload lost its Content-Type: %+v", head)
	}

	// CopyObject, then UploadPartCopy of ranges of it into another bucket.
	if _, err := r.c.CopyObject(ctx, &s3.CopyObjectInput{Bucket: ptr("dst"), Key: ptr("copy.bin"), CopySource: ptr("src/big.bin")}); err != nil {
		t.Fatalf("CopyObject: %v", err)
	}
	if got, _ := r.get("dst", "copy.bin"); !bytes.Equal(got, whole) {
		t.Error("CopyObject is not byte-identical")
	}
	cp, err := r.c.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{Bucket: ptr("dst"), Key: ptr("parts.bin")})
	if err != nil {
		t.Fatal(err)
	}
	var copied []types.CompletedPart
	for i, rng := range []string{fmt.Sprintf("bytes=0-%d", 5*mib-1), fmt.Sprintf("bytes=%d-", 5*mib)} {
		out, err := r.c.UploadPartCopy(ctx, &s3.UploadPartCopyInput{Bucket: ptr("dst"), Key: ptr("parts.bin"), UploadId: cp.UploadId, PartNumber: ptr(int32(i + 1)), CopySource: ptr("src/big.bin"), CopySourceRange: ptr(rng)})
		if err != nil {
			t.Fatalf("UploadPartCopy %s: %v", rng, err)
		}
		copied = append(copied, types.CompletedPart{PartNumber: ptr(int32(i + 1)), ETag: out.CopyPartResult.ETag})
	}
	if _, err := r.c.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{Bucket: ptr("dst"), Key: ptr("parts.bin"), UploadId: cp.UploadId, MultipartUpload: &types.CompletedMultipartUpload{Parts: copied}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.get("dst", "parts.bin"); !bytes.Equal(got, whole) {
		t.Error("an object copied in two ranges is not the source")
	}
	// An abort leaves nothing at Wasabi or in the listing.
	ab, _ := r.c.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{Bucket: ptr("dst"), Key: ptr("never")})
	if _, err := r.c.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: ptr("dst"), Key: ptr("never"), UploadId: ab.UploadId}); err != nil {
		t.Fatal(err)
	}
	lu, err := r.c.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{Bucket: ptr("dst")})
	if err != nil || len(lu.Uploads) != 0 || r.Fakes[objectstorage.Oregon].OpenUploads() != 0 {
		t.Errorf("after the abort: %d uploads listed (%v), %d open at Wasabi", len(lu.Uploads), err, r.Fakes[objectstorage.Oregon].OpenUploads())
	}
	_, err = r.c.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: ptr("dst"), Key: ptr("never"), UploadId: ab.UploadId})
	wantCode(t, "a second abort", err, "NoSuchUpload", 404)
}

func TestSDKListingContinuationDelimiterAndBatchDelete(t *testing.T) {
	r := newSDKRig(t)
	ctx := context.Background()
	r.bucket("lst", objectstorage.Oregon)
	var keys []string
	for i := 0; i < 25; i++ {
		key := fmt.Sprintf("d%d/k%02d", i%3, i)
		keys = append(keys, key)
		r.put("lst", key, []byte{byte(i)})
	}
	seen := map[string]bool{}
	pages := 0
	pager := s3.NewListObjectsV2Paginator(r.c, &s3.ListObjectsV2Input{Bucket: ptr("lst"), MaxKeys: ptr(int32(7))})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, o := range page.Contents {
			if seen[*o.Key] {
				t.Errorf("%s was listed twice", *o.Key)
			}
			seen[*o.Key] = true
		}
	}
	if len(seen) != 25 || pages != 4 {
		t.Errorf("a ListObjectsV2 paginator saw %d keys in %d pages, want 25 in 4", len(seen), pages)
	}
	prefixes := map[string]bool{}
	pager = s3.NewListObjectsV2Paginator(r.c, &s3.ListObjectsV2Input{Bucket: ptr("lst"), Delimiter: ptr("/"), MaxKeys: ptr(int32(1))})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range page.CommonPrefixes {
			prefixes[*p.Prefix] = true
		}
	}
	if len(prefixes) != 3 || !prefixes["d0/"] || !prefixes["d1/"] || !prefixes["d2/"] {
		t.Errorf("the common prefixes are %v, want d0/ d1/ d2/", prefixes)
	}
	// ListObjects (V1) pages by NextMarker when a delimiter is given and by the last key otherwise.
	var marker *string
	v1 := 0
	for {
		page, err := r.c.ListObjects(ctx, &s3.ListObjectsInput{Bucket: ptr("lst"), MaxKeys: ptr(int32(10)), Marker: marker})
		if err != nil {
			t.Fatal(err)
		}
		v1 += len(page.Contents)
		if !*page.IsTruncated {
			break
		}
		marker = page.Contents[len(page.Contents)-1].Key
	}
	if v1 != 25 {
		t.Errorf("ListObjects by marker saw %d keys, want 25", v1)
	}
	var ids []types.ObjectIdentifier
	for _, k := range keys {
		ids = append(ids, types.ObjectIdentifier{Key: ptr(k)})
	}
	del, err := r.c.DeleteObjects(ctx, &s3.DeleteObjectsInput{Bucket: ptr("lst"), Delete: &types.Delete{Objects: ids}})
	if err != nil || len(del.Deleted) != 25 || len(del.Errors) != 0 {
		t.Fatalf("DeleteObjects: %d deleted, %d errors, %v", len(del.Deleted), len(del.Errors), err)
	}
	if out, _ := r.c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: ptr("lst")}); len(out.Contents) != 0 {
		t.Errorf("%d keys remain after the batch delete", len(out.Contents))
	}
}

func TestSDKBucketsAndEachConfigurationRoundTrip(t *testing.T) {
	r := newSDKRig(t)
	ctx := context.Background()
	if _, err := r.c.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: ptr("cfg"), CreateBucketConfiguration: &types.CreateBucketConfiguration{LocationConstraint: "frankfurt"}}); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if loc, err := r.c.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: ptr("cfg")}); err != nil || string(loc.LocationConstraint) != "frankfurt" {
		t.Errorf("GetBucketLocation = %v, %v", loc, err)
	}
	if _, err := r.c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: ptr("cfg")}); err != nil {
		t.Errorf("HeadBucket: %v", err)
	}
	if lb, err := r.c.ListBuckets(ctx, &s3.ListBucketsInput{}); err != nil || len(lb.Buckets) != 1 || *lb.Buckets[0].Name != "cfg" {
		t.Errorf("ListBuckets = %+v, %v", lb, err)
	}
	_, err := r.c.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: ptr("cfg")})
	wantCode(t, "creating a bucket the workspace has", err, "BucketAlreadyOwnedByYou", 409)

	cors := &types.CORSConfiguration{CORSRules: []types.CORSRule{
		{AllowedOrigins: []string{"https://app.example"}, AllowedMethods: []string{"GET", "PUT"}, AllowedHeaders: []string{"*"}, ExposeHeaders: []string{"ETag"}, MaxAgeSeconds: ptr(int32(600))},
		{AllowedOrigins: []string{"*"}, AllowedMethods: []string{"GET"}},
	}}
	if _, err := r.c.PutBucketCors(ctx, &s3.PutBucketCorsInput{Bucket: ptr("cfg"), CORSConfiguration: cors}); err != nil {
		t.Fatalf("PutBucketCors: %v", err)
	}
	gotCORS, err := r.c.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: ptr("cfg")})
	if err != nil || len(gotCORS.CORSRules) != 2 || gotCORS.CORSRules[0].AllowedOrigins[0] != "https://app.example" ||
		len(gotCORS.CORSRules[0].AllowedMethods) != 2 || *gotCORS.CORSRules[0].MaxAgeSeconds != 600 || gotCORS.CORSRules[0].ExposeHeaders[0] != "ETag" || gotCORS.CORSRules[1].AllowedOrigins[0] != "*" {
		t.Errorf("GetBucketCors = %+v, %v", gotCORS, err)
	}
	if _, err := r.c.DeleteBucketCors(ctx, &s3.DeleteBucketCorsInput{Bucket: ptr("cfg")}); err != nil {
		t.Fatal(err)
	}
	_, err = r.c.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: ptr("cfg")})
	wantCode(t, "GetBucketCors with none", err, "NoSuchCORSConfiguration", 404)

	life := &types.BucketLifecycleConfiguration{Rules: []types.LifecycleRule{
		{ID: ptr("tmp"), Status: types.ExpirationStatusEnabled, Filter: &types.LifecycleRuleFilter{Prefix: ptr("tmp/")}, Expiration: &types.LifecycleExpiration{Days: ptr(int32(3))},
			AbortIncompleteMultipartUpload: &types.AbortIncompleteMultipartUpload{DaysAfterInitiation: ptr(int32(2))}},
	}}
	if _, err := r.c.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{Bucket: ptr("cfg"), LifecycleConfiguration: life}); err != nil {
		t.Fatalf("PutBucketLifecycleConfiguration: %v", err)
	}
	gotLife, err := r.c.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: ptr("cfg")})
	if err != nil || len(gotLife.Rules) != 1 || *gotLife.Rules[0].ID != "tmp" || *gotLife.Rules[0].Expiration.Days != 3 ||
		*gotLife.Rules[0].AbortIncompleteMultipartUpload.DaysAfterInitiation != 2 || gotLife.Rules[0].Status != types.ExpirationStatusEnabled {
		t.Errorf("GetBucketLifecycleConfiguration = %+v, %v", gotLife, err)
	}
	life.Rules[0].Expiration.Days = ptr(int32(40_000))
	_, err = r.c.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{Bucket: ptr("cfg"), LifecycleConfiguration: life})
	wantCode(t, "a lifecycle rule of 40,000 days", err, "InvalidArgument", 400)
	if _, err := r.c.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{Bucket: ptr("cfg")}); err != nil {
		t.Fatal(err)
	}
	// The calls bex does not implement answer 501, not silence.
	_, err = r.c.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{Bucket: ptr("cfg"), Policy: ptr(`{"Version":"2012-10-17","Statement":[]}`)})
	wantCode(t, "PutBucketPolicy", err, "NotImplemented", 501)
	_, err = r.c.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{Bucket: ptr("cfg"), VersioningConfiguration: &types.VersioningConfiguration{Status: types.BucketVersioningStatusEnabled}})
	wantCode(t, "PutBucketVersioning", err, "NotImplemented", 501)
	if _, err := r.c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: ptr("cfg")}); err != nil {
		t.Errorf("DeleteBucket: %v", err)
	}
}

func TestSDKPresignedURLsWorkAndAreBoundToTheirKeyAndExpiry(t *testing.T) {
	r := newSDKRig(t)
	ctx := context.Background()
	r.bucket("pre", objectstorage.Oregon)
	r.put("pre", "a", []byte("object A"))
	r.put("pre", "b", []byte("object B"))
	ps := s3.NewPresignClient(r.c)
	get := func(url string) (int, string) {
		resp, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}
	a, err := ps.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: ptr("pre"), Key: ptr("a")}, s3.WithPresignExpires(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if status, body := get(a.URL); status != 200 || body != "object A" {
		t.Fatalf("a presigned GET = %d %q", status, body)
	}
	// The URL signed for a is not valid for b, and a changed query invalidates it.
	if status, _ := get(strings.Replace(a.URL, "/pre/a?", "/pre/b?", 1)); status != 403 {
		t.Errorf("a URL signed for one key, used for another = %d, want 403", status)
	}
	if status, _ := get(strings.Replace(a.URL, "X-Amz-Expires=60", "X-Amz-Expires=3600", 1)); status != 403 {
		t.Errorf("a URL with its expiry changed = %d, want 403", status)
	}
	short, err := ps.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: ptr("pre"), Key: ptr("a")}, s3.WithPresignExpires(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2200 * time.Millisecond)
	if status, body := get(short.URL); status != 403 || !strings.Contains(body, "expired") {
		t.Errorf("a URL past its expiry = %d %q, want 403 AccessDenied (expired)", status, body)
	}
	put, err := ps.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: ptr("pre"), Key: ptr("uploaded")}, s3.WithPresignExpires(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPut, put.URL, strings.NewReader("by presigned PUT"))
	for name, values := range put.SignedHeader {
		if !strings.EqualFold(name, "host") {
			req.Header[name] = values
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("a presigned PUT = %v, %v", resp, err)
	}
	resp.Body.Close()
	if got, err := r.get("pre", "uploaded"); err != nil || string(got) != "by presigned PUT" {
		t.Errorf("the presigned PUT stored %q, %v", got, err)
	}
	// A presigned URL is the key's authority and the creator's: once the creator no longer holds the
	// relation, the URL stops working.
	r.authz.held = map[string]bool{}
	if status, _ := get(a.URL); status != 403 {
		t.Errorf("a presigned GET by a key whose creator lost access = %d, want 403", status)
	}
}

func TestSDKIsolationBetweenWorkspacesBucketsAndKeys(t *testing.T) {
	r := newSDKRig(t)
	ctx := context.Background()
	mine := r.bucket("mine", objectstorage.Oregon)
	r.put("mine", "secret.txt", []byte("mine"))
	// Another workspace's bucket, with an object at Wasabi and an unfinished upload.
	foreign := r.otherWorkspace()
	theirs, err := r.PG.CreateObjectBucket(ctx, storeBucket(foreign, "theirs"), 10)
	if err != nil {
		t.Fatal(err)
	}
	r.Fakes[objectstorage.Oregon].Put(objectstorage.Oregon.Bucket(), foreign+"/"+theirs.ID+"/theirs.txt", []byte("theirs"))
	upload, err := r.PG.CreateObjectMultipartUpload(ctx, storeUpload(foreign, theirs.ID, "theirs.txt", "foreign-upload"), 10)
	if err != nil {
		t.Fatal(err)
	}
	physical := map[string]string{
		"another workspace's bucket name":      "theirs",
		"another workspace's obk- id":          theirs.ID,
		"this workspace's own obk- id":         mine[strings.Index(mine, "obk-") : strings.Index(mine, "obk-")+len("obk-")+20],
		"the workspace id":                     r.ws,
		"a bucket of the other workspace's id": foreign,
	}
	for what, bucket := range physical {
		b := bucket
		for op, call := range map[string]func() error{
			"GetObject": func() error { _, err := r.get(b, "theirs.txt"); return err },
			"PutObject": func() error {
				_, err := r.c.PutObject(ctx, &s3.PutObjectInput{Bucket: &b, Key: ptr("x"), Body: strings.NewReader("x")})
				return err
			},
			"ListObjectsV2": func() error { _, err := r.c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &b}); return err },
			"HeadObject": func() error {
				_, err := r.c.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &b, Key: ptr("theirs.txt")})
				return err
			},
			"DeleteObject": func() error {
				_, err := r.c.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &b, Key: ptr("theirs.txt")})
				return err
			},
			"GetBucketCors": func() error { _, err := r.c.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: &b}); return err },
		} {
			err := call()
			if c, status := code(err); c != "NoSuchBucket" && !(op == "HeadObject" && status == 404) {
				t.Errorf("%s on %s = %q (HTTP %d), want NoSuchBucket", op, what, c, status)
			}
		}
	}
	if got, ok := r.Fakes[objectstorage.Oregon].Get(objectstorage.Oregon.Bucket(), foreign+"/"+theirs.ID+"/theirs.txt"); !ok || string(got) != "theirs" {
		t.Error("another workspace's object was touched")
	}
	// An upload id of another workspace is no upload here, on any bucket and key.
	for _, k := range []struct{ bucket, key string }{{"mine", "theirs.txt"}, {"mine", "secret.txt"}} {
		_, err := r.c.UploadPart(ctx, &s3.UploadPartInput{Bucket: &k.bucket, Key: &k.key, UploadId: &upload.ID, PartNumber: ptr(int32(1)), Body: strings.NewReader("x")})
		wantCode(t, "UploadPart with another workspace's upload id", err, "NoSuchUpload", 404)
		_, err = r.c.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{Bucket: &k.bucket, Key: &k.key, UploadId: &upload.ID, MultipartUpload: &types.CompletedMultipartUpload{Parts: []types.CompletedPart{{PartNumber: ptr(int32(1)), ETag: ptr(`"x"`)}}}})
		wantCode(t, "CompleteMultipartUpload with another workspace's upload id", err, "NoSuchUpload", 404)
		_, err = r.c.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: &k.bucket, Key: &k.key, UploadId: &upload.ID})
		wantCode(t, "AbortMultipartUpload with another workspace's upload id", err, "NoSuchUpload", 404)
	}
	if _, err := r.PG.GetObjectMultipartUpload(ctx, foreign, upload.ID); err != nil {
		t.Errorf("another workspace's upload was removed: %v", err)
	}
	// A forged continuation token and a prefix that tries to leave the bucket.
	_, err = r.c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: ptr("mine"), ContinuationToken: ptr(objectstorage.EncodeListToken(false, "../"+theirs.ID+"/theirs.txt"))})
	if err != nil {
		t.Errorf("a forged but well-formed token only moves the start inside the bucket: %v", err)
	}
	out, err := r.c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: ptr("mine"), ContinuationToken: ptr(objectstorage.EncodeListToken(false, "../"+theirs.ID+"/theirs.txt"))})
	if err == nil {
		for _, o := range out.Contents {
			if strings.Contains(*o.Key, "theirs") {
				t.Errorf("a forged token listed %q", *o.Key)
			}
		}
	}
	_, err = r.c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: ptr("mine"), ContinuationToken: ptr("not a token")})
	wantCode(t, "a malformed continuation token", err, "InvalidArgument", 400)
	_, err = r.c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: ptr("mine"), Prefix: ptr("../" + theirs.ID + "/")})
	wantCode(t, "a listing prefix with ..", err, "InvalidArgument", 400)
	// Keys: the shapes D7 refuses are refused, and Unicode and 1,024 bytes are kept intact.
	for _, bad := range []string{"a/../b", "../x", "a//b", "/lead", ".", "a/./b", "tab\tkey", "back\\slash", strings.Repeat("k", 1025)} {
		_, err := r.c.PutObject(ctx, &s3.PutObjectInput{Bucket: ptr("mine"), Key: ptr(bad), Body: strings.NewReader("x")})
		if err == nil {
			t.Errorf("PutObject accepted the key %q", bad)
		}
	}
	for _, good := range []string{"ünï/çødé/日本語.txt", "with space+plus%25 and=sign&amp", "emoji-\U0001F600", strings.Repeat("k", 1024)} {
		r.put("mine", good, []byte(good))
		if got, err := r.get("mine", good); err != nil || string(got) != good {
			t.Errorf("the key %q did not round trip: %q, %v", good, got, err)
		}
	}
}

func TestSDKABucketNameReusedAfterDeleteIsANewEmptyBucket(t *testing.T) {
	r := newSDKRig(t)
	ctx := context.Background()
	first := r.bucket("reuse", objectstorage.Oregon)
	r.put("reuse", "old.txt", []byte("old"))
	if _, err := r.c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: ptr("reuse")}); err == nil {
		t.Fatal("a bucket with an object was deleted")
	}
	if _, err := r.c.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: ptr("reuse"), Key: ptr("old.txt")}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: ptr("reuse")}); err != nil {
		t.Fatal(err)
	}
	// An object left behind under the old bucket's prefix (a delete that raced a put, say)
	// must not appear in a bucket created later under the same name.
	r.Fakes[objectstorage.Oregon].Put(objectstorage.Oregon.Bucket(), first+"orphan.txt", []byte("orphan"))
	r.bucket("reuse", objectstorage.Oregon)
	out, err := r.c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: ptr("reuse")})
	if err != nil || len(out.Contents) != 0 {
		t.Errorf("the recreated bucket lists %d objects, %v", len(out.Contents), err)
	}
	_, err = r.get("reuse", "orphan.txt")
	wantCode(t, "an orphan of the old bucket", err, "NoSuchKey", 404)
}

func TestSDKAKeyWhoseCreatorWasRemovedIsRefusedEverywhere(t *testing.T) {
	r := newSDKRig(t)
	ctx := context.Background()
	r.bucket("gone", objectstorage.Oregon)
	r.put("gone", "o", []byte("x"))
	r.authz.held = map[string]bool{} // the creator left the workspace: no relation holds
	for op, call := range map[string]func() error{
		"GetObject": func() error { _, err := r.get("gone", "o"); return err },
		"PutObject": func() error {
			_, err := r.c.PutObject(ctx, &s3.PutObjectInput{Bucket: ptr("gone"), Key: ptr("y"), Body: strings.NewReader("y")})
			return err
		},
		"ListObjects": func() error {
			_, err := r.c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: ptr("gone")})
			return err
		},
		"ListBuckets": func() error { _, err := r.c.ListBuckets(ctx, &s3.ListBucketsInput{}); return err },
		"CreateBucket": func() error {
			_, err := r.c.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: ptr("another")})
			return err
		},
		"Multipart": func() error {
			_, err := r.c.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{Bucket: ptr("gone"), Key: ptr("m")})
			return err
		},
	} {
		wantCode(t, op+" by a key whose creator left", call(), "AccessDenied", 403)
	}
	// The creator comes back: the same key works again, so nothing was revoked by the refusals.
	r.authz.held = map[string]bool{core.RelCanView: true, core.RelCanViewSensitive: true, core.RelCanCreate: true}
	if got, err := r.get("gone", "o"); err != nil || string(got) != "x" {
		t.Errorf("after the creator returned: %q, %v", got, err)
	}
}
