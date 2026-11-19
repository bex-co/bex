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
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// Render's object routes (Contract A, `render ea objects`) and the S3 surface work in the same
// bucket, render-<region>, so what one writes the other lists, reads and deletes (ADR112 D1,
// D3). Contract A's routes are thin adapters over the Service's Mint*, ListObjects and
// DeleteObject, which this drives with the same presigned URLs the CLI is handed; the bytes
// of a Render upload and download go to the object store, as they do in production.
func TestObjectsWrittenWithRendersRoutesAreListedAndReadThroughS3AndTheOtherWayRound(t *testing.T) {
	r := newSDKRig(t)
	ctx := context.Background()
	region := objectstorage.Oregon
	system := objectstorage.SystemBucketName(region)

	// Render side: mint a PUT URL and upload to it, as `render ea objects put` does.
	fromRender := salted(30_000, 11)
	mint, err := r.Svc.MintPut(ctx, r.ws, region, "dir/from-render.bin", int64(len(fromRender)))
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPut, mint.URL, bytes.NewReader(fromRender))
	req.ContentLength = int64(len(fromRender))
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("the upload to the minted URL = %v, %v", resp, err)
	}
	resp.Body.Close()

	// S3 side: the system bucket is a bucket like any other, and the object is in it.
	if _, err := r.c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &system}); err != nil {
		t.Fatalf("HeadBucket %s: %v", system, err)
	}
	listed, err := r.c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &system})
	if err != nil || len(listed.Contents) != 1 || *listed.Contents[0].Key != "dir/from-render.bin" || *listed.Contents[0].Size != int64(len(fromRender)) {
		t.Fatalf("S3 lists %+v (%v) in %s, want the object Render's route wrote", listed, err, system)
	}
	if got, err := r.get(system, "dir/from-render.bin"); err != nil || !bytes.Equal(got, fromRender) {
		t.Fatalf("S3 reads %d bytes of the Render upload, %v", len(got), err)
	}

	// S3 writes, Render lists and reads.
	fromS3 := salted(1234, 12)
	r.put(system, "dir/from-s3.bin", fromS3)
	page, err := r.Svc.ListObjects(ctx, r.ws, region, "", 50)
	if err != nil || len(page.Items) != 2 || page.Items[0].Key != "dir/from-render.bin" || page.Items[1].Key != "dir/from-s3.bin" || page.Items[1].SizeBytes != int64(len(fromS3)) {
		t.Fatalf("Render's listing = %+v, %v, want both objects", page.Items, err)
	}
	get, err := r.Svc.MintGet(ctx, r.ws, region, "dir/from-s3.bin")
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.Get(get.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Equal(body, fromS3) {
		t.Fatalf("Render's minted GET of the S3 upload = %d, %d bytes", resp.StatusCode, len(body))
	}

	// A delete on either side is a delete on the other.
	if err := r.Svc.DeleteObject(ctx, r.ws, region, "dir/from-render.bin"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.get(system, "dir/from-render.bin"); err == nil {
		t.Error("S3 still reads an object Render's route deleted")
	}
	if _, err := r.c.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &system, Key: ptr("dir/from-s3.bin")}); err != nil {
		t.Fatal(err)
	}
	if page, _ := r.Svc.ListObjects(ctx, r.ws, region, "", 50); len(page.Items) != 0 {
		t.Errorf("Render still lists %+v after S3 deleted the last object", page.Items)
	}
	// The system bucket cannot be deleted through S3, as Render's namespace cannot be.
	_, err = r.c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &system})
	wantCode(t, "DeleteBucket on the system bucket", err, "AccessDenied", 403)
}
