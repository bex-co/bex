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

package objectstorage_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func upload(t *testing.T, putURL string, body []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT to the presigned URL = %v, %v", resp, err)
	}
}

func download(t *testing.T, getURL string) (string, http.Header) {
	t.Helper()
	resp, err := http.Get(getURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET of the presigned URL = %v, %v", resp, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.Header
}

func TestRenderObjectRoundTrip(t *testing.T) {
	e := newPGEnv(t, "objects-roundtrip")
	ctx := context.Background()
	body := []byte("hello render")

	put, err := e.Svc.MintPut(ctx, e.WS, objectstorage.Oregon, "dir/a.txt", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if put.MaxSizeBytes != int64(len(body)) {
		t.Fatalf("MaxSizeBytes = %d, want the size asked for (%d): the SDKs require it and the URL signs it", put.MaxSizeBytes, len(body))
	}
	if d := time.Until(put.ExpiresAt); d < 14*time.Minute || d > 15*time.Minute+time.Second {
		t.Fatalf("a put URL expires in %v, want about 15 minutes", d)
	}
	if u, _ := url.Parse(put.URL); !strings.Contains(u.Query().Get("X-Amz-SignedHeaders"), "content-length") {
		t.Fatalf("the put URL does not sign content-length: %s", put.URL)
	}
	upload(t, put.URL, body)

	// The object sits under <workspace>/<system bucket id>/ and nowhere else.
	sys, err := e.Svc.SystemBucket(ctx, e.WS, objectstorage.Oregon)
	if err != nil {
		t.Fatal(err)
	}
	wantKey := e.WS + "/" + sys.ID + "/dir/a.txt"
	if keys := e.Fake.Keys(objectstorage.Oregon.Bucket(), ""); len(keys) != 1 || keys[0] != wantKey {
		t.Fatalf("stored keys = %v, want exactly %s", keys, wantKey)
	}

	get, err := e.Svc.MintGet(ctx, e.WS, objectstorage.Oregon, "dir/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got, header := download(t, get.URL); got != string(body) || header.Get("Content-Disposition") != "attachment" {
		t.Fatalf("downloaded %q with disposition %q", got, header.Get("Content-Disposition"))
	}

	page, err := e.Svc.ListObjects(ctx, e.WS, objectstorage.Oregon, "", 100)
	if err != nil || len(page.Items) != 1 || page.Items[0].Key != "dir/a.txt" || page.Items[0].SizeBytes != int64(len(body)) || page.HasNext {
		t.Fatalf("ListObjects = %+v, %v", page, err)
	}

	if err := e.Svc.DeleteObject(ctx, e.WS, objectstorage.Oregon, "dir/a.txt"); err != nil {
		t.Fatal(err)
	}
	if page, _ := e.Svc.ListObjects(ctx, e.WS, objectstorage.Oregon, "", 100); len(page.Items) != 0 {
		t.Fatalf("the object is still listed after a delete: %+v", page.Items)
	}
}

func TestMissingKeysAreNotFoundOnGetAndDelete(t *testing.T) {
	e := newPGEnv(t, "objects-missing")
	ctx := context.Background()
	if _, err := e.Svc.MintGet(ctx, e.WS, objectstorage.Oregon, "nope"); !errors.Is(err, objectstorage.ErrObjectNotFound) {
		t.Errorf("MintGet of a missing key = %v, want ErrObjectNotFound (a URL for it would only fail later)", err)
	}
	if err := e.Svc.DeleteObject(ctx, e.WS, objectstorage.Oregon, "nope"); !errors.Is(err, objectstorage.ErrObjectNotFound) {
		t.Errorf("DeleteObject of a missing key = %v, want ErrObjectNotFound (S3 would answer 204)", err)
	}
}

func TestRenderObjectsAreScopedToTheWorkspaceAndTheSystemBucket(t *testing.T) {
	e := newPGEnv(t, "objects-scope")
	ctx := context.Background()
	put, err := e.Svc.MintPut(ctx, e.WS, objectstorage.Oregon, "secret.txt", 3)
	if err != nil {
		t.Fatal(err)
	}
	upload(t, put.URL, []byte("abc"))

	other := ids.New(ids.Workspace)
	e.cleanup(t, other)
	if _, err := e.Svc.MintGet(ctx, other, objectstorage.Oregon, "secret.txt"); !errors.Is(err, objectstorage.ErrObjectNotFound) {
		t.Fatalf("another workspace reads this key: %v", err)
	}
	if page, _ := e.Svc.ListObjects(ctx, other, objectstorage.Oregon, "", 10); len(page.Items) != 0 {
		t.Fatalf("another workspace lists %d of this workspace's objects", len(page.Items))
	}

	// An object in a user bucket of the same workspace is not in the Render namespace.
	photos, err := e.Svc.CreateBucket(ctx, e.WS, "photos", objectstorage.Oregon, "")
	if err != nil {
		t.Fatal(err)
	}
	userKey, _ := objectstorage.ObjectKey(e.WS, photos.ID, "cat.jpg")
	e.Fake.Put(objectstorage.Oregon.Bucket(), userKey, []byte("x"))
	page, err := e.Svc.ListObjects(ctx, e.WS, objectstorage.Oregon, "", 100)
	if err != nil || len(page.Items) != 1 || page.Items[0].Key != "secret.txt" {
		t.Fatalf("the Render listing = %+v, %v; want only the system bucket's object", page.Items, err)
	}
	if _, err := e.Svc.MintGet(ctx, e.WS, objectstorage.Oregon, "cat.jpg"); !errors.Is(err, objectstorage.ErrObjectNotFound) {
		t.Fatalf("a user bucket's object is readable through the Render namespace: %v", err)
	}
}

func TestMintPutRefusesBeforeCreatingAnything(t *testing.T) {
	e := newPGEnv(t, "objects-refuse")
	ctx := context.Background()
	cases := []struct {
		name string
		key  string
		size int64
		want error
	}{
		{"empty key", "", 1, objectstorage.ErrInvalidKey},
		{"dot dot", "a/../b", 1, objectstorage.ErrInvalidKey},
		{"negative size", "k", -1, objectstorage.ErrObjectTooLarge},
		{"one byte over 5 GiB", "k", objectstorage.MaxObjectBytesPresigned + 1, objectstorage.ErrObjectTooLarge},
	}
	for _, c := range cases {
		if _, err := e.Svc.MintPut(ctx, e.WS, objectstorage.Oregon, c.key, c.size); !errors.Is(err, c.want) {
			t.Errorf("%s: MintPut = %v, want %v", c.name, err, c.want)
		}
	}
	var notOffered *objectstorage.RegionNotOfferedError
	if _, err := e.Svc.MintPut(ctx, e.WS, objectstorage.Frankfurt, "k", 1); !errors.As(err, &notOffered) {
		t.Errorf("a region with no store = %v", err)
	}
	if list, _ := e.Svc.ListBuckets(ctx, e.WS); len(list) != 0 {
		t.Fatalf("refused mints created %d bucket rows", len(list))
	}
	// Exactly the 5 GiB cap is allowed: the URL only signs the size.
	if _, err := e.Svc.MintPut(ctx, e.WS, objectstorage.Oregon, "big", objectstorage.MaxObjectBytesPresigned); err != nil {
		t.Errorf("a 5 GiB object was refused: %v", err)
	}
	if _, err := e.Svc.MintPut(ctx, e.WS, objectstorage.Oregon, "empty", 0); err != nil {
		t.Errorf("a 0 B object was refused: %v", err)
	}
}

func TestMintPutIsRefusedAtTheQuota(t *testing.T) {
	e := newPGEnv(t, "objects-quota")
	ctx := context.Background()
	sys, err := e.Svc.SystemBucket(ctx, e.WS, objectstorage.Oregon)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.PG.UpsertObjectStorageUsage(ctx, store.ObjectStorageUsage{
		BucketID: sys.ID, WorkspaceID: e.WS, Region: "oregon", Bytes: objectstorage.MaxWorkspaceBytes - 10, Objects: 5, ObservedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	var q *objectstorage.QuotaError
	if _, err := e.Svc.MintPut(ctx, e.WS, objectstorage.Oregon, "k", 11); !errors.As(err, &q) || q.Resource != "bytes" {
		t.Fatalf("an 11-byte put with 10 bytes of headroom = %v, want a bytes QuotaError", err)
	}
	if _, err := e.Svc.MintPut(ctx, e.WS, objectstorage.Oregon, "k", 10); err != nil {
		t.Fatalf("a put that exactly fits was refused: %v", err)
	}
}

func TestMintsAreRateLimitedPerWorkspace(t *testing.T) {
	e := newPGEnv(t, "objects-rate")
	ctx := context.Background()
	// The bucket refills MintPerSecond tokens a second on the real clock, and
	// each mint reads Postgres, so a slow run earns a few tokens back while it
	// drains the burst: mint until the first refusal and allow what the
	// elapsed time refilled.
	granted := 0
	var last error
	start := time.Now()
	for range objectstorage.MintBurst + 100 {
		if _, last = e.Svc.MintPut(ctx, e.WS, objectstorage.Oregon, "k", 1); last != nil {
			break
		}
		granted++
	}
	refilled := int(time.Since(start).Seconds()*objectstorage.MintPerSecond) + 1
	if !errors.Is(last, objectstorage.ErrRateLimited) || granted < objectstorage.MintBurst || granted > objectstorage.MintBurst+refilled {
		t.Fatalf("granted %d mints, then %v; want %d (+ up to %d refilled) then ErrRateLimited", granted, last, objectstorage.MintBurst, refilled)
	}
	// A read mint draws on the same bucket, and another workspace is unaffected.
	if _, err := e.Svc.MintGet(ctx, e.WS, objectstorage.Oregon, "k"); !errors.Is(err, objectstorage.ErrRateLimited) {
		t.Fatalf("a get mint past the rate = %v, want ErrRateLimited", err)
	}
	other := ids.New(ids.Workspace)
	e.cleanup(t, other)
	if _, err := e.Svc.MintPut(ctx, other, objectstorage.Oregon, "k", 1); err != nil {
		t.Fatalf("another workspace was limited: %v", err)
	}
}

func TestListObjectsPagesWithCursorsAndHonorsALimitOf200(t *testing.T) {
	e := newPGEnv(t, "objects-list")
	ctx := context.Background()
	sys, err := e.Svc.SystemBucket(ctx, e.WS, objectstorage.Oregon)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 250 {
		e.Fake.Put(objectstorage.Oregon.Bucket(), fmt.Sprintf("%s/%s/k/%04d", e.WS, sys.ID, i), []byte("v"))
	}

	// The CLI's own help example asks for 200; a smaller cap would truncate silently.
	first, err := e.Svc.ListObjects(ctx, e.WS, objectstorage.Oregon, "", 200)
	if err != nil || len(first.Items) != 200 || !first.HasNext || first.NextCursor == "" {
		t.Fatalf("first page = %d items, hasNext %v, next %q, %v", len(first.Items), first.HasNext, first.NextCursor, err)
	}
	if first.NextCursor != first.Items[199].Cursor || first.Items[0].Key != "k/0000" {
		t.Fatalf("cursor %q does not resume after the last item %q", first.NextCursor, first.Items[199].Key)
	}
	second, err := e.Svc.ListObjects(ctx, e.WS, objectstorage.Oregon, first.NextCursor, 200)
	if err != nil || len(second.Items) != 50 || second.HasNext || second.NextCursor != "" || second.Items[0].Key != "k/0200" {
		t.Fatalf("second page = %d items from %q, hasNext %v, %v", len(second.Items), second.Items[0].Key, second.HasNext, err)
	}
	// A limit of zero or less is a page of one, and a huge limit is capped at the S3 page.
	if p, _ := e.Svc.ListObjects(ctx, e.WS, objectstorage.Oregon, "", 0); len(p.Items) != 1 {
		t.Errorf("limit 0 returned %d items", len(p.Items))
	}
	if p, _ := e.Svc.ListObjects(ctx, e.WS, objectstorage.Oregon, "", 1_000_000); len(p.Items) != 250 {
		t.Errorf("a huge limit returned %d items, want all 250 (under the cap of %d)", len(p.Items), objectstorage.ListMaxLimit)
	}
}

func TestListObjectsRefusesACursorItDidNotMint(t *testing.T) {
	e := newPGEnv(t, "objects-cursor")
	ctx := context.Background()
	for _, bad := range []string{
		"!!!not base64!!!",
		base64.RawURLEncoding.EncodeToString([]byte("a/../b")),
		base64.RawURLEncoding.EncodeToString([]byte("/leading")),
	} {
		if _, err := e.Svc.ListObjects(ctx, e.WS, objectstorage.Oregon, bad, 10); !errors.Is(err, objectstorage.ErrInvalidCursor) {
			t.Errorf("cursor %q = %v, want ErrInvalidCursor", bad, err)
		}
	}
}

func TestObjectOperationsAnswerNotConfiguredWithoutACredential(t *testing.T) {
	s := objectstorage.NewService(nil, nil)
	ctx := context.Background()
	if _, err := s.MintPut(ctx, "tea-x", objectstorage.Oregon, "k", 1); !errors.Is(err, objectstorage.ErrNotConfigured) {
		t.Errorf("MintPut = %v", err)
	}
	if _, err := s.MintGet(ctx, "tea-x", objectstorage.Oregon, "k"); !errors.Is(err, objectstorage.ErrNotConfigured) {
		t.Errorf("MintGet = %v", err)
	}
	if err := s.DeleteObject(ctx, "tea-x", objectstorage.Oregon, "k"); !errors.Is(err, objectstorage.ErrNotConfigured) {
		t.Errorf("DeleteObject = %v", err)
	}
	if _, err := s.ListObjects(ctx, "tea-x", objectstorage.Oregon, "", 10); !errors.Is(err, objectstorage.ErrNotConfigured) {
		t.Errorf("ListObjects = %v", err)
	}
}
