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
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func storeBucket(workspaceID, name string) store.ObjectBucket {
	return store.ObjectBucket{
		ID: ids.New(ids.ObjectBucket), WorkspaceID: workspaceID, Name: name, Region: "oregon",
	}
}

func storeUpload(workspaceID, bucketID, key, wasabiID string) store.ObjectMultipartUpload {
	return store.ObjectMultipartUpload{
		ID: ids.New(ids.ObjectMultipartUpload), WorkspaceID: workspaceID, BucketID: bucketID, Region: "oregon", Key: key,
		WasabiUploadID: wasabiID, CreatedBy: "s3k-test",
	}
}

const mib = 1 << 20

// salted is n bytes that differ from salted(n, other) at every position of a different seed,
// so an assembled object that took its parts in the wrong order or the wrong version of one
// differs from the expected.
func salted(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7+3) ^ seed ^ byte(i>>10)
	}
	return b
}

type partRef struct {
	n    int
	etag string
}

func (p *pgRig) createUpload(k testKey, bucket, key string) string {
	p.t.Helper()
	rec, doc := p.do(p.signed(k, "POST", "/"+bucket+"/"+uriEncode(key, false)+"?uploads", nil))
	if rec.Code != 200 {
		p.t.Fatalf("CreateMultipartUpload = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
	var out initiateMultipartXML
	if err := xml.Unmarshal(rec.Body.Bytes(), &out); err != nil || !strings.HasPrefix(out.UploadID, "mpu-") || out.Bucket != bucket || out.Key != key {
		p.t.Fatalf("CreateMultipartUpload answered %s (%v)", rec.Body, err)
	}
	return out.UploadID
}

func (p *pgRig) uploadPart(k testKey, bucket, key, id string, n int, data []byte, mutate ...func(*http.Request)) (*httptest.ResponseRecorder, errorDoc) {
	target := fmt.Sprintf("/%s/%s?partNumber=%d&uploadId=%s", bucket, uriEncode(key, false), n, url.QueryEscape(id))
	return p.do(p.signed(k, "PUT", target, bytes.NewReader(data), append([]func(*http.Request){func(r *http.Request) { r.ContentLength = int64(len(data)) }}, mutate...)...))
}

// part uploads a part that must succeed and returns it as the complete request lists it.
func (p *pgRig) part(k testKey, bucket, key, id string, n int, data []byte) partRef {
	p.t.Helper()
	rec, doc := p.uploadPart(k, bucket, key, id, n, data)
	if rec.Code != 200 || rec.Header().Get("ETag") == "" {
		p.t.Fatalf("UploadPart %d = %d %q %s", n, rec.Code, doc.Code, doc.Message)
	}
	return partRef{n, rec.Header().Get("ETag")}
}

func completeBody(parts ...partRef) *bytes.Reader {
	var b strings.Builder
	b.WriteString("<CompleteMultipartUpload>")
	for _, p := range parts {
		fmt.Fprintf(&b, "<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", p.n, p.etag)
	}
	b.WriteString("</CompleteMultipartUpload>")
	return bytes.NewReader([]byte(b.String()))
}

func (p *pgRig) complete(k testKey, bucket, key, id string, parts ...partRef) (*httptest.ResponseRecorder, errorDoc) {
	body := completeBody(parts...)
	return p.do(p.signed(k, "POST", "/"+bucket+"/"+uriEncode(key, false)+"?uploadId="+url.QueryEscape(id), body,
		func(r *http.Request) { r.ContentLength = int64(body.Len()) }))
}

func (p *pgRig) abort(k testKey, bucket, key, id string) (*httptest.ResponseRecorder, errorDoc) {
	return p.do(p.signed(k, "DELETE", "/"+bucket+"/"+uriEncode(key, false)+"?uploadId="+url.QueryEscape(id), nil))
}

func (p *pgRig) listParts(k testKey, bucket, key, id string) listPartsResultXML {
	p.t.Helper()
	rec, doc := p.do(p.signed(k, "GET", "/"+bucket+"/"+uriEncode(key, false)+"?uploadId="+url.QueryEscape(id), nil))
	if rec.Code != 200 {
		p.t.Fatalf("ListParts = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
	var out listPartsResultXML
	if err := xml.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		p.t.Fatal(err)
	}
	return out
}

func TestAMultipartUploadRoundTripsInPartsAndTheObjectIsByteIdentical(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	prefix := p.bucket("mpu", objectstorage.Oregon)
	one, two, three := salted(5*mib, 1), salted(5*mib, 2), salted(1500, 3)
	id := p.createUpload(k, "mpu", "big/obj.bin")
	// Parts may arrive in any order, and the ones that follow need not be sent by the same call.
	r3 := p.part(k, "mpu", "big/obj.bin", id, 3, three)
	r1 := p.part(k, "mpu", "big/obj.bin", id, 1, one)
	r2 := p.part(k, "mpu", "big/obj.bin", id, 2, two)
	got := p.listParts(k, "mpu", "big/obj.bin", id)
	if len(got.Parts) != 3 || got.Parts[0].PartNumber != 1 || got.Parts[2].PartNumber != 3 || got.Parts[0].Size != 5*mib || got.Parts[2].Size != 1500 || got.IsTruncated {
		t.Fatalf("ListParts = %+v", got)
	}
	if got.Parts[1].ETag != r2.etag || got.UploadID != id || got.Key != "big/obj.bin" {
		t.Errorf("ListParts shows %q for part 2, want %q (upload %q)", got.Parts[1].ETag, r2.etag, got.UploadID)
	}
	rec, doc := p.complete(k, "mpu", "big/obj.bin", id, r1, r2, r3)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<CompleteMultipartUploadResult") || !strings.Contains(rec.Body.String(), "-3") {
		t.Fatalf("CompleteMultipartUpload = %d %q %s", rec.Code, doc.Code, rec.Body)
	}
	want := append(append(append([]byte(nil), one...), two...), three...)
	if stored, ok := p.stored(objectstorage.Oregon, prefix, "big/obj.bin"); !ok || !bytes.Equal(stored, want) {
		t.Fatal("the assembled object is not the parts in order, byte for byte")
	}
	if rec, _ := p.do(p.signed(k, "GET", "/mpu/big/obj.bin", nil)); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), want) {
		t.Errorf("a GET of the assembled object = %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if n := p.Fakes[objectstorage.Oregon].OpenUploads(); n != 0 {
		t.Errorf("%d uploads are still open at Wasabi after the complete", n)
	}
	// The upload is gone: a second complete, a part and an abort of it are NoSuchUpload.
	for name, call := range map[string]func() (*httptest.ResponseRecorder, errorDoc){
		"complete": func() (*httptest.ResponseRecorder, errorDoc) { return p.complete(k, "mpu", "big/obj.bin", id, r1) },
		"part": func() (*httptest.ResponseRecorder, errorDoc) {
			return p.uploadPart(k, "mpu", "big/obj.bin", id, 4, one)
		},
		"abort": func() (*httptest.ResponseRecorder, errorDoc) { return p.abort(k, "mpu", "big/obj.bin", id) },
	} {
		if rec, doc := call(); rec.Code != 404 || doc.Code != "NoSuchUpload" {
			t.Errorf("a %s of a completed upload = %d %q, want 404 NoSuchUpload", name, rec.Code, doc.Code)
		}
	}
	// What completed counts against the cap before the next sweep, as a put does.
	b, _ := p.Svc.GetBucket(context.Background(), p.ws, "mpu")
	p.observeUsage(objectstorage.Oregon, b.ID, 0, 0, time.Now().Add(-time.Hour))
	if bytesWritten, objects, _ := p.PG.ObjectStorageWriteTotals(context.Background(), p.ws, "oregon"); bytesWritten < int64(len(want)) || objects < 1 {
		t.Errorf("the write-through counter holds %d bytes in %d objects, want at least the %d the upload completed", bytesWritten, objects, len(want))
	}
}

func TestAPartSentAgainReplacesTheEarlierOneAndAnUnlistedPartIsDropped(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	prefix := p.bucket("mpu", objectstorage.Oregon)
	id := p.createUpload(k, "mpu", "o")
	first, second, extra, last := salted(5*mib, 1), salted(5*mib+10, 9), salted(5*mib, 4), salted(77, 5)
	p.part(k, "mpu", "o", id, 1, first)
	r1 := p.part(k, "mpu", "o", id, 1, second) // replaces
	p.part(k, "mpu", "o", id, 2, extra)        // never listed
	r3 := p.part(k, "mpu", "o", id, 3, last)
	parts := p.listParts(k, "mpu", "o", id).Parts
	if len(parts) != 3 || parts[0].Size != 5*mib+10 || parts[0].ETag != r1.etag {
		t.Fatalf("ListParts after a replaced part = %+v", parts)
	}
	if rec, doc := p.complete(k, "mpu", "o", id, r1, r3); rec.Code != 200 {
		t.Fatalf("complete without part 2 = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
	if stored, _ := p.stored(objectstorage.Oregon, prefix, "o"); !bytes.Equal(stored, append(append([]byte(nil), second...), last...)) {
		t.Error("the object is not the replaced part 1 followed by part 3")
	}
}

func TestCompleteRefusesWhatS3RefusesAndLeavesTheUploadOpen(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	prefix := p.bucket("mpu", objectstorage.Oregon)
	id := p.createUpload(k, "mpu", "o")
	r1 := p.part(k, "mpu", "o", id, 1, salted(5*mib, 1))
	r2 := p.part(k, "mpu", "o", id, 2, salted(5*mib, 2))
	small := p.part(k, "mpu", "o", id, 3, salted(100, 3))
	r4 := p.part(k, "mpu", "o", id, 4, salted(5*mib, 4))
	for _, c := range []struct {
		name   string
		parts  []partRef
		status int
		code   string
	}{
		{"a part with the wrong ETag", []partRef{r1, {2, `"00000000000000000000000000000000"`}}, 400, "InvalidPart"},
		{"a part never uploaded", []partRef{r1, {9, r2.etag}}, 400, "InvalidPart"},
		{"parts out of order", []partRef{r2, r1}, 400, "InvalidPartOrder"},
		{"a repeated part", []partRef{r1, r1}, 400, "InvalidPartOrder"},
		{"a part under 5 MiB that is not the last", []partRef{r1, small, r4}, 400, "EntityTooSmall"},
		{"no parts", nil, 400, "MalformedXML"},
	} {
		if rec, doc := p.complete(k, "mpu", "o", id, c.parts...); rec.Code != c.status || doc.Code != c.code {
			t.Errorf("%s = %d %q, want %d %s", c.name, rec.Code, doc.Code, c.status, c.code)
		}
	}
	if _, ok := p.stored(objectstorage.Oregon, prefix, "o"); ok {
		t.Error("a refused complete stored an object")
	}
	if p.Fakes[objectstorage.Oregon].OpenUploads() != 1 {
		t.Error("a refused complete closed the upload")
	}
	// A client's ETag may carry the quotes S3 shows or not.
	if rec, doc := p.complete(k, "mpu", "o", id, partRef{1, strings.Trim(r1.etag, `"`)}, r2, r4); rec.Code != 200 {
		t.Errorf("a valid complete after the refused ones = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
}

func TestAnInterruptedPartRecordsNothingAndTheUploadStaysUsable(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mpu", objectstorage.Oregon)
	id := p.createUpload(k, "mpu", "o")
	short := salted(6*mib, 1)
	rec, doc := p.uploadPart(k, "mpu", "o", id, 1, short[:mib], func(r *http.Request) { r.ContentLength = int64(len(short)) })
	if rec.Code != 400 {
		t.Fatalf("a part whose body ended early = %d %q, want 400", rec.Code, doc.Code)
	}
	if parts := p.listParts(k, "mpu", "o", id).Parts; len(parts) != 0 {
		t.Errorf("an interrupted part was recorded: %+v", parts)
	}
	if got := p.Fakes[objectstorage.Oregon].UploadParts(p.uploadWasabiID(id)); len(got) != 0 {
		t.Errorf("Wasabi holds parts %v of the interrupted upload", got)
	}
	whole := salted(5*mib, 2)
	r1 := p.part(k, "mpu", "o", id, 1, whole)
	if rec, doc := p.complete(k, "mpu", "o", id, r1); rec.Code != 200 {
		t.Errorf("the upload after an interrupted part = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
}

// uploadWasabiID is the id Wasabi gave an upload, which only the row holds.
func (p *pgRig) uploadWasabiID(id string) string {
	p.t.Helper()
	u, err := p.PG.GetObjectMultipartUpload(context.Background(), p.ws, id)
	if err != nil {
		p.t.Fatal(err)
	}
	return u.WasabiUploadID
}

func TestUnfinishedPartsCountAgainstTheQuotaUntilTheUploadAbortsOrCompletes(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mpu", objectstorage.Oregon)
	b, _ := p.Svc.GetBucket(context.Background(), p.ws, "mpu")
	// The last sweep saw the workspace 8 MiB under its cap.
	p.observeUsage(objectstorage.Oregon, b.ID, objectstorage.MaxWorkspaceBytes-8*mib, 5, time.Now().Add(-time.Minute))
	id := p.createUpload(k, "mpu", "o")
	p.part(k, "mpu", "o", id, 1, salted(5*mib, 1))
	rec, doc := p.uploadPart(k, "mpu", "o", id, 2, salted(5*mib, 2))
	if rec.Code != 403 || doc.Code != "QuotaExceeded" {
		t.Fatalf("a second part past the cap = %d %q, want 403 QuotaExceeded", rec.Code, doc.Code)
	}
	// A single put sees the unfinished bytes too.
	if rec, doc := p.put(k, "/mpu/other", 4*mib); rec.Code != 403 || doc.Code != "QuotaExceeded" {
		t.Errorf("a put past the cap beside an unfinished upload = %d %q, want 403", rec.Code, doc.Code)
	}
	if rec, doc := p.abort(k, "mpu", "o", id); rec.Code != 204 {
		t.Fatalf("AbortMultipartUpload = %d %q", rec.Code, doc.Code)
	}
	if rec, doc := p.put(k, "/mpu/other", 4*mib); rec.Code != 200 {
		t.Errorf("a put after the abort freed the parts = %d %q, want 200", rec.Code, doc.Code)
	}
	if n := p.Fakes[objectstorage.Oregon].OpenUploads(); n != 0 {
		t.Errorf("%d uploads are still open at Wasabi after the abort", n)
	}
}

func TestAPartThatWouldTakeTheUploadPast5GiBIsRefusedWhenItStarts(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mpu", objectstorage.Oregon)
	id := p.createUpload(k, "mpu", "o")
	// An earlier part of 5 GiB, recorded as if it had been uploaded.
	if err := p.PG.UpsertObjectMultipartPart(context.Background(), id, 1, objectstorage.MaxPartBytes, `"abc"`); err != nil {
		t.Fatal(err)
	}
	rec, doc := p.uploadPart(k, "mpu", "o", id, 2, salted(10, 1))
	if rec.Code != 400 || doc.Code != "EntityTooLarge" {
		t.Errorf("a part past the 5 GiB total = %d %q, want 400 EntityTooLarge", rec.Code, doc.Code)
	}
	if got := p.Fakes[objectstorage.Oregon].UploadParts(p.uploadWasabiID(id)); len(got) != 0 {
		t.Errorf("a refused part reached Wasabi: %v", got)
	}
	// Part 1 sent again replaces its 5 GiB: it is not added to itself.
	if rec, doc := p.uploadPart(k, "mpu", "o", id, 1, salted(10, 1), func(r *http.Request) { r.ContentLength = 10 }); rec.Code != 200 {
		t.Errorf("a part sent again under the cap = %d %q", rec.Code, doc.Code)
	}
	// A single part over 5 GiB is refused on its declared length, before a byte is read.
	rec, doc = p.uploadPart(k, "mpu", "o", id, 3, nil, func(r *http.Request) { r.ContentLength = objectstorage.MaxPartBytes + 1 })
	if rec.Code != 400 || doc.Code != "EntityTooLarge" {
		t.Errorf("a part of 5 GiB + 1 = %d %q, want 400 EntityTooLarge", rec.Code, doc.Code)
	}
	for _, n := range []int{0, 10001, -1} {
		rec, doc := p.do(p.signed(k, "PUT", fmt.Sprintf("/mpu/o?partNumber=%d&uploadId=%s", n, id), bytes.NewReader([]byte("x")), func(r *http.Request) { r.ContentLength = 1 }))
		if rec.Code != 400 || doc.Code != "InvalidArgument" {
			t.Errorf("part number %d = %d %q, want 400 InvalidArgument", n, rec.Code, doc.Code)
		}
	}
}

func TestAnUploadIdOfAnotherWorkspaceBucketOrKeyIsNoSuchUpload(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mpu", objectstorage.Oregon)
	p.bucket("other", objectstorage.Oregon)
	id := p.createUpload(k, "mpu", "o")
	// Another workspace's upload, which exists, with its own Wasabi upload.
	foreign := p.otherWorkspace()
	foreignBucket, err := p.PG.CreateObjectBucket(context.Background(), storeBucket(foreign, "theirs"), 10)
	if err != nil {
		t.Fatal(err)
	}
	row, err := p.PG.CreateObjectMultipartUpload(context.Background(), storeUpload(foreign, foreignBucket.ID, "o", "up-1"), 10)
	if err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{
		"another bucket":    "/other/o",
		"another key":       "/mpu/different",
		"another workspace": "/mpu/o",
	} {
		uploadID := id
		if name == "another workspace" {
			uploadID = row.ID
		}
		for op, call := range map[string]func() (*httptest.ResponseRecorder, errorDoc){
			"UploadPart": func() (*httptest.ResponseRecorder, errorDoc) {
				return p.do(p.signed(k, "PUT", target+"?partNumber=1&uploadId="+uploadID, bytes.NewReader([]byte("x")), func(r *http.Request) { r.ContentLength = 1 }))
			},
			"Complete": func() (*httptest.ResponseRecorder, errorDoc) {
				return p.do(p.signed(k, "POST", target+"?uploadId="+uploadID, completeBody(partRef{1, `"a"`})))
			},
			"Abort": func() (*httptest.ResponseRecorder, errorDoc) {
				return p.do(p.signed(k, "DELETE", target+"?uploadId="+uploadID, nil))
			},
			"ListParts": func() (*httptest.ResponseRecorder, errorDoc) {
				return p.do(p.signed(k, "GET", target+"?uploadId="+uploadID, nil))
			},
		} {
			if rec, doc := call(); rec.Code != 404 || doc.Code != "NoSuchUpload" {
				t.Errorf("%s with the upload id of %s = %d %q, want 404 NoSuchUpload", op, name, rec.Code, doc.Code)
			}
		}
	}
	// The other workspace's upload is untouched, and ours is still open.
	if _, err := p.PG.GetObjectMultipartUpload(context.Background(), foreign, row.ID); err != nil {
		t.Errorf("another workspace's upload was touched: %v", err)
	}
	if _, err := p.PG.GetObjectMultipartUpload(context.Background(), p.ws, id); err != nil {
		t.Errorf("this workspace's upload was touched by the probes: %v", err)
	}
	// A listing of the bucket shows this bucket's uploads only.
	rec, _ := p.do(p.signed(k, "GET", "/other?uploads", nil))
	if strings.Contains(rec.Body.String(), "<Upload>") {
		t.Errorf("a bucket with no upload lists %s", rec.Body)
	}
}

func TestListMultipartUploadsPagesByKeyAndUploadIdAndFiltersByPrefix(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mpu", objectstorage.Oregon)
	want := map[string]bool{}
	for _, key := range []string{"a/1", "a/2", "a/2", "b/1", "é"} {
		want[key+"|"+p.createUpload(k, "mpu", key)] = true
	}
	var seen []string
	keyMarker, idMarker := "", ""
	for pages := 0; pages < 10; pages++ {
		q := url.Values{"uploads": {""}, "max-uploads": {"2"}}
		if keyMarker != "" {
			q.Set("key-marker", keyMarker)
			q.Set("upload-id-marker", idMarker)
		}
		rec, doc := p.do(p.signed(k, "GET", "/mpu?"+q.Encode(), nil))
		if rec.Code != 200 {
			t.Fatalf("ListMultipartUploads = %d %q", rec.Code, doc.Code)
		}
		var res listUploadsResultXML
		if err := xml.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		for _, u := range res.Uploads {
			seen = append(seen, u.Key+"|"+u.UploadID)
		}
		if !res.IsTruncated {
			break
		}
		keyMarker, idMarker = res.NextKeyMarker, res.NextUploadIDMarker
	}
	if len(seen) != 5 {
		t.Fatalf("the pages listed %v, want 5 uploads", seen)
	}
	for i, s := range seen {
		if !want[s] || (i > 0 && s < seen[i-1] && strings.SplitN(s, "|", 2)[0] == strings.SplitN(seen[i-1], "|", 2)[0]) {
			t.Errorf("upload %q is unexpected or out of order in %v", s, seen)
		}
	}
	rec, _ := p.do(p.signed(k, "GET", "/mpu?uploads&prefix=a/", nil))
	if n := strings.Count(rec.Body.String(), "<Upload>"); n != 3 {
		t.Errorf("a listing under a/ shows %d uploads, want 3", n)
	}
	if rec, doc := p.do(p.signed(k, "GET", "/mpu?uploads&delimiter=/", nil)); rec.Code != 501 {
		t.Errorf("a delimiter on ListMultipartUploads = %d %q, want 501", rec.Code, doc.Code)
	}
}

func TestOnlyOneHundredUploadsMayBeOpenAndABucketWithOneCannotBeDeleted(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mpu", objectstorage.Oregon)
	var first string
	for i := 0; i < objectstorage.MaxOpenMultipartUploads; i++ {
		id := p.createUpload(k, "mpu", fmt.Sprintf("o%03d", i))
		if first == "" {
			first = id
		}
	}
	rec, doc := p.do(p.signed(k, "POST", "/mpu/one-too-many?uploads", nil))
	if rec.Code != 403 || doc.Code != "QuotaExceeded" {
		t.Fatalf("the 101st open upload = %d %q, want 403 QuotaExceeded", rec.Code, doc.Code)
	}
	if n := p.Fakes[objectstorage.Oregon].OpenUploads(); n != objectstorage.MaxOpenMultipartUploads {
		t.Errorf("Wasabi holds %d uploads, want the refused one aborted: %d", n, objectstorage.MaxOpenMultipartUploads)
	}
	if rec, doc := p.do(p.signed(k, "DELETE", "/mpu", nil)); rec.Code != 409 || doc.Code != "BucketNotEmpty" {
		t.Errorf("DeleteBucket with unfinished uploads = %d %q, want 409 BucketNotEmpty", rec.Code, doc.Code)
	}
	p.abort(k, "mpu", "o000", first)
	p.createUpload(k, "mpu", "after-abort") // the slot is free again
}

func TestAPartCopyTakesARangeOfASourceInsideWasabiAndNeedsTheSourceRead(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	src := p.bucket("src", objectstorage.Oregon)
	dst := p.bucket("dst", objectstorage.Oregon)
	p.bucket("eur", objectstorage.Frankfurt)
	data := salted(12*mib+345, 7)
	if rec, doc := p.put(k, "/src/big.bin", len(data)); rec.Code != 200 {
		t.Fatalf("put = %d %q", rec.Code, doc.Code)
	}
	p.Fakes[objectstorage.Oregon].Put(objectstorage.Oregon.Bucket(), src+"big.bin", data)
	id := p.createUpload(k, "dst", "copy.bin")
	copyPart := func(n int, rng string, mutate ...func(*http.Request)) (*httptest.ResponseRecorder, errorDoc) {
		return p.do(p.signed(k, "PUT", fmt.Sprintf("/dst/copy.bin?partNumber=%d&uploadId=%s", n, id), nil, append([]func(*http.Request){func(r *http.Request) {
			r.ContentLength = 0
			r.Header.Set("x-amz-copy-source", "/src/big.bin")
			if rng != "" {
				r.Header.Set("x-amz-copy-source-range", rng)
			}
		}}, mutate...)...))
	}
	var parts []partRef
	for i, rng := range []string{fmt.Sprintf("bytes=0-%d", 5*mib-1), fmt.Sprintf("bytes=%d-%d", 5*mib, 10*mib-1), fmt.Sprintf("bytes=%d-", 10*mib)} {
		rec, doc := copyPart(i+1, rng)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<CopyPartResult") {
			t.Fatalf("UploadPartCopy %s = %d %q %s", rng, rec.Code, doc.Code, rec.Body)
		}
		var res copyPartResultXML
		_ = xml.Unmarshal(rec.Body.Bytes(), &res)
		parts = append(parts, partRef{i + 1, res.ETag})
	}
	if got := p.listParts(k, "dst", "copy.bin", id).Parts; len(got) != 3 || got[0].Size != 5*mib || got[2].Size != 2*mib+345 {
		t.Fatalf("the copied parts are %+v", got)
	}
	if rec, doc := p.complete(k, "dst", "copy.bin", id, parts...); rec.Code != 200 {
		t.Fatalf("complete = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
	if got, ok := p.stored(objectstorage.Oregon, dst, "copy.bin"); !ok || !bytes.Equal(got, data) {
		t.Error("the object copied in parts is not the source, byte for byte")
	}
	// The probes that follow use a new upload: the first completed.
	id = p.createUpload(k, "dst", "copy.bin")
	if rec, doc := copyPart(1, fmt.Sprintf("bytes=%d-%d", len(data), len(data)+9)); rec.Code != 416 {
		t.Errorf("a range past the source's end = %d %q, want 416", rec.Code, doc.Code)
	}
	// S3 defines the range only as bytes=first-last or bytes=first-: a suffix range, several ranges
	// and garbage are refused instead of being read some other way (a suffix would have counted
	// one byte too many against the quota).
	for _, bad := range []string{"bytes=-5", "bytes=0-1,3-4", "bytes=abc", "bytes=5-2", "0-5", "bytes=-"} {
		if rec, doc := copyPart(1, bad); rec.Code != 400 || doc.Code != "InvalidArgument" {
			t.Errorf("a part copy with the range %q = %d %q, want 400 InvalidArgument", bad, rec.Code, doc.Code)
		}
	}
	if rec, doc := copyPart(1, "", func(r *http.Request) { r.Header.Set("x-amz-copy-source", "/eur/x") }); rec.Code != 501 || doc.Code != "NotImplemented" {
		t.Errorf("a part copy from a bucket of another region = %d %q, want 501", rec.Code, doc.Code)
	}
	if rec, doc := copyPart(1, "", func(r *http.Request) { r.Header.Set("x-amz-copy-source", "/src/missing") }); rec.Code != 404 || doc.Code != "NoSuchKey" {
		t.Errorf("a missing source = %d %q, want 404 NoSuchKey", rec.Code, doc.Code)
	}
	// The conditions on the source, which `aws s3 cp` sends so a source that changed under a
	// multipart copy is not stitched together.
	head, _ := p.do(p.signed(k, "HEAD", "/src/big.bin", nil))
	etag := head.Header().Get("ETag")
	past, future := "Sat, 01 Jan 2000 00:00:00 GMT", "Sat, 01 Jan 2100 00:00:00 GMT"
	for _, c := range []struct {
		header, value string
		status        int
	}{
		{"x-amz-copy-source-if-match", etag, 200},
		{"x-amz-copy-source-if-match", `"00000000000000000000000000000000"`, 412},
		{"x-amz-copy-source-if-none-match", etag, 412},
		{"x-amz-copy-source-if-none-match", `"00000000000000000000000000000000"`, 200},
		{"x-amz-copy-source-if-unmodified-since", future, 200},
		{"x-amz-copy-source-if-unmodified-since", past, 412},
		{"x-amz-copy-source-if-modified-since", past, 200},
		{"x-amz-copy-source-if-modified-since", future, 412},
	} {
		rec, doc := copyPart(1, "", func(r *http.Request) { r.Header.Set(c.header, c.value) })
		if rec.Code != c.status || (c.status == 412 && doc.Code != "PreconditionFailed") {
			t.Errorf("a part copy with %s: %s = %d %q, want %d", c.header, c.value, rec.Code, doc.Code, c.status)
		}
	}
	// Reading the source is the creator's can_view_sensitive, as for CopyObject.
	p.authz.held = map[string]bool{core.RelCanView: true, core.RelCanCreate: true}
	if rec, doc := copyPart(2, ""); rec.Code != 403 || doc.Code != "AccessDenied" {
		t.Errorf("a part copy by a creator who cannot read the source = %d %q, want 403", rec.Code, doc.Code)
	}
}

func TestAMultipartUploadWritesItsCreateAndCompleteRowsAndOneRowPerPartNever(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mpu", objectstorage.Oregon)
	id := p.createUpload(k, "mpu", "o")
	var parts []partRef
	for i := 1; i <= 3; i++ {
		parts = append(parts, p.part(k, "mpu", "o", id, i, salted(5*mib, byte(i))))
	}
	p.complete(k, "mpu", "o", id, parts...)
	id2 := p.createUpload(k, "mpu", "gone")
	p.abort(k, "mpu", "gone", id2)
	count := func(verb string) int { return len(p.Audit.byVerb(verb)) }
	if count(objectstorage.AuditVerbCreateMultipartUpload) != 2 || count(objectstorage.AuditVerbCompleteMultipartUpload) != 1 || count(objectstorage.AuditVerbAbortMultipartUpload) != 1 {
		t.Errorf("rows: create %d, complete %d, abort %d, want 2, 1 and 1", count(objectstorage.AuditVerbCreateMultipartUpload),
			count(objectstorage.AuditVerbCompleteMultipartUpload), count(objectstorage.AuditVerbAbortMultipartUpload))
	}
	if n := count(objectstorage.AuditVerbPutObject); n != 0 {
		t.Errorf("%d PutObject rows for the parts of one upload, want none", n)
	}
	for _, row := range p.Audit.byVerb(objectstorage.AuditVerbCompleteMultipartUpload) {
		if strings.Contains(row.TargetName, "uploadId") || strings.Contains(row.TargetName, id) {
			t.Errorf("an audit row carries the upload's query: %q", row.TargetName)
		}
	}
	// A read key's denied parts are one row per window, not one per part.
	reader := p.newKey(objectstorage.KeyPermissionRead, 0)
	for i := 1; i <= 5; i++ {
		if rec, _ := p.uploadPart(reader, "mpu", "o", id, i, salted(10, 1)); rec.Code != 403 {
			t.Fatalf("a read key's part = %d, want 403", rec.Code)
		}
	}
	if n := count(objectstorage.AuditVerbPutObject); n != 1 {
		t.Errorf("%d denied rows for five refused parts of one key, want one", n)
	}
}

func TestTheSweepAbortsAnUploadWithNoPartForSevenDaysAndALifecycleRuleShortensIt(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mpu", objectstorage.Oregon)
	p.bucket("rules", objectstorage.Oregon)
	stale := p.createUpload(k, "mpu", "stale")
	fresh := p.createUpload(k, "mpu", "fresh")
	p.part(k, "mpu", "fresh", fresh, 1, salted(10, 1))
	ruled := p.createUpload(k, "rules", "tmp/x")
	kept := p.createUpload(k, "rules", "keep/y")
	ctx := context.Background()
	// Six days on, nothing is stale; eight days on, the upload with no part is, and the one
	// whose part came later is not.
	if n, err := p.Svc.AbortStaleUploads(ctx, time.Now().Add(6*24*time.Hour)); err != nil || n != 0 {
		t.Fatalf("after 6 days the sweep aborted %d (%v), want none", n, err)
	}
	if _, err := p.PG.Pool.Exec(ctx, `UPDATE object_multipart_uploads SET last_part_at = now() - interval '8 days' WHERE id = ANY($1)`, []string{stale, ruled, kept}); err != nil {
		t.Fatal(err)
	}
	if n, err := p.Svc.AbortStaleUploads(ctx, time.Now()); err != nil || n != 3 {
		t.Fatalf("the sweep aborted %d (%v), want the 3 stale uploads", n, err)
	}
	for _, id := range []string{stale, ruled, kept} {
		if _, err := p.PG.GetObjectMultipartUpload(ctx, p.ws, id); err == nil {
			t.Errorf("upload %s outlived the sweep", id)
		}
	}
	if _, err := p.PG.GetObjectMultipartUpload(ctx, p.ws, fresh); err != nil {
		t.Errorf("an upload that had a part was aborted: %v", err)
	}
	if n := p.Fakes[objectstorage.Oregon].OpenUploads(); n != 1 {
		t.Errorf("Wasabi holds %d uploads after the sweep, want the one that is fresh", n)
	}

	// A lifecycle AbortIncompleteMultipartUpload rule aborts an upload by its age and prefix.
	rules := `<LifecycleConfiguration><Rule><ID>r</ID><Status>Enabled</Status><Filter><Prefix>tmp/</Prefix></Filter>` +
		`<AbortIncompleteMultipartUpload><DaysAfterInitiation>2</DaysAfterInitiation></AbortIncompleteMultipartUpload></Rule></LifecycleConfiguration>`
	if rec, doc := p.do(p.signed(k, "PUT", "/rules?lifecycle", strings.NewReader(rules), func(r *http.Request) { r.ContentLength = int64(len(rules)) })); rec.Code != 200 {
		t.Fatalf("PutBucketLifecycleConfiguration = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
	ruled = p.createUpload(k, "rules", "tmp/x")
	kept = p.createUpload(k, "rules", "keep/y")
	b, _ := p.Svc.GetBucket(ctx, p.ws, "rules")
	logf := func(string, ...any) {}
	if n, err := p.Svc.AbortBucketUploads(ctx, b, time.Now().Add(36*time.Hour), logf); err != nil || n != 0 {
		t.Fatalf("a rule of 2 days aborted %d (%v) after 1.5 days", n, err)
	}
	if n, err := p.Svc.AbortBucketUploads(ctx, b, time.Now().Add(72*time.Hour), logf); err != nil || n != 1 {
		t.Fatalf("a rule of 2 days aborted %d (%v) after 3 days, want the one under tmp/", n, err)
	}
	if _, err := p.PG.GetObjectMultipartUpload(ctx, p.ws, ruled); err == nil {
		t.Error("the upload under the rule's prefix survived")
	}
	if _, err := p.PG.GetObjectMultipartUpload(ctx, p.ws, kept); err != nil {
		t.Errorf("an upload outside the rule's prefix was aborted: %v", err)
	}
}

func TestDeletingAWorkspaceAbortsItsUploadsAtWasabi(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("aaa", objectstorage.Oregon)
	p.bucket("bbb", objectstorage.Frankfurt)
	p.part(k, "aaa", "x", p.createUpload(k, "aaa", "x"), 1, salted(100, 1))
	p.createUpload(k, "bbb", "y")
	if p.Fakes[objectstorage.Oregon].OpenUploads()+p.Fakes[objectstorage.Frankfurt].OpenUploads() != 2 {
		t.Fatal("the uploads are not open")
	}
	purger := &objectstorage.WorkspacePurger{Service: p.Svc, Repo: p.PG}
	if err := purger.PurgeWorkspace(context.Background(), p.ws); err != nil {
		t.Fatal(err)
	}
	if n := p.Fakes[objectstorage.Oregon].OpenUploads() + p.Fakes[objectstorage.Frankfurt].OpenUploads(); n != 0 {
		t.Errorf("%d multipart uploads are left at Wasabi after the workspace's purge", n)
	}
	if rows, err := p.PG.ObjectMultipartUploadsOfWorkspace(context.Background(), p.ws, 10); err != nil || len(rows) != 0 {
		t.Errorf("the purge left upload rows %v (%v)", rows, err)
	}
}

func TestAnAbortThatFailsAtWasabiKeepsTheRowSoTheClientOrTheNextPassRetries(t *testing.T) {
	p := newWideRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("mpu", objectstorage.Oregon)
	idle := p.createUpload(k, "mpu", "idle")
	live := p.createUpload(k, "mpu", "live")
	ctx := context.Background()
	if _, err := p.Pool.Exec(ctx, `UPDATE object_multipart_uploads SET last_part_at = now() - interval '8 days' WHERE id = $1`, idle); err != nil {
		t.Fatal(err)
	}
	fake := p.Fakes[objectstorage.Oregon]
	fake.SetDown(true)
	if n, err := p.Svc.AbortStaleUploads(ctx, time.Now()); err == nil || n != 0 {
		t.Fatalf("the sweep with Wasabi down aborted %d (%v), want a failure and none", n, err)
	}
	if rec, _ := p.abort(k, "mpu", "live", live); rec.Code < 500 {
		t.Errorf("an abort with Wasabi down = %d, want a 5xx the SDK retries", rec.Code)
	}
	for _, id := range []string{idle, live} {
		if _, err := p.PG.GetObjectMultipartUpload(ctx, p.ws, id); err != nil {
			t.Errorf("upload %s lost its row while Wasabi still holds it: %v", id, err)
		}
	}
	fake.SetDown(false)
	if n, err := p.Svc.AbortStaleUploads(ctx, time.Now()); err != nil || n != 1 {
		t.Errorf("the next pass aborted %d (%v), want the one stale upload", n, err)
	}
	if rec, doc := p.abort(k, "mpu", "live", live); rec.Code != 204 {
		t.Errorf("the client's retry = %d %q, want 204", rec.Code, doc.Code)
	}
	if fake.OpenUploads() != 0 {
		t.Errorf("%d uploads are still open at Wasabi", fake.OpenUploads())
	}
}
