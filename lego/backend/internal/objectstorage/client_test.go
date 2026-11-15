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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
)

const testBucket = "bex-objects-oregon"

func newTestStore(t *testing.T) (objectstorage.Store, *fakes3.Server) {
	t.Helper()
	fake := fakes3.New(testBucket)
	t.Cleanup(fake.Close)
	store := objectstorage.NewStore(objectstorage.Config{
		Endpoint: fake.URL, Region: "us-west-1", Bucket: testBucket, AccessKey: "AKIATEST", SecretKey: "secret", MaxAttempts: 1,
	})
	return store, fake
}

func TestRegionConfigUsesTheProductionConstants(t *testing.T) {
	cfg := objectstorage.Frankfurt.Config("ak", "sk")
	if cfg.Endpoint != "https://s3.eu-central-2.wasabisys.com" || cfg.Region != "eu-central-2" || cfg.Bucket != "bex-objects-frankfurt" ||
		cfg.AccessKey != "ak" || cfg.SecretKey != "sk" {
		t.Fatalf("Config = %+v", cfg)
	}
}

func TestPresignPutSignsContentLength(t *testing.T) {
	store, _ := newTestStore(t)
	raw, err := store.PresignPut(context.Background(), "tea-x/obk-y/cat.jpg", 1234, objectstorage.PresignTTL)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	signed := strings.Split(u.Query().Get("X-Amz-SignedHeaders"), ";")
	if !contains(signed, "content-length") {
		t.Fatalf("a presigned PUT must sign content-length so the upload cannot exceed what was reserved; signed headers = %v (url %s)", signed, raw)
	}
	if got := u.Query().Get("X-Amz-Expires"); got != "900" {
		t.Fatalf("X-Amz-Expires = %q, want 900 (15 minutes)", got)
	}
	if !strings.HasSuffix(u.Path, "/"+testBucket+"/tea-x/obk-y/cat.jpg") {
		t.Fatalf("path = %q: want the path-style bucket and the full key", u.Path)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestPresignedPutThenGetRoundTripsThroughTheStore(t *testing.T) {
	store, fake := newTestStore(t)
	ctx := context.Background()
	body := []byte("hello object storage")
	putURL, err := store.PresignPut(ctx, "tea-x/obk-y/dir/a.txt", int64(len(body)), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT to the presigned URL = %v, %v", resp, err)
	}
	if got, ok := fake.Get(testBucket, "tea-x/obk-y/dir/a.txt"); !ok || !bytes.Equal(got, body) {
		t.Fatalf("stored body = %q, %v", got, ok)
	}

	getURL, err := store.PresignGet(ctx, "tea-x/obk-y/dir/a.txt", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(getURL, "response-content-disposition=attachment") {
		t.Fatalf("a presigned GET must pin content-disposition: attachment, got %s", getURL)
	}
	resp, err := http.Get(getURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) || resp.Header.Get("Content-Disposition") != "attachment" {
		t.Fatalf("GET = %q with disposition %q", got, resp.Header.Get("Content-Disposition"))
	}
}

func TestHead(t *testing.T) {
	store, fake := newTestStore(t)
	fake.Put(testBucket, "tea-x/obk-y/a", []byte("12345"))
	info, err := store.Head(context.Background(), "tea-x/obk-y/a")
	if err != nil || info.Size != 5 || info.Key != "tea-x/obk-y/a" || info.ETag == "" {
		t.Fatalf("Head = %+v, %v", info, err)
	}
	if _, err := store.Head(context.Background(), "tea-x/obk-y/missing"); !errors.Is(err, objectstorage.ErrObjectNotFound) {
		t.Fatalf("Head of a missing key = %v, want ErrObjectNotFound", err)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	store, fake := newTestStore(t)
	fake.Put(testBucket, "tea-x/obk-y/a", []byte("x"))
	for i := range 2 {
		if err := store.Delete(context.Background(), "tea-x/obk-y/a"); err != nil {
			t.Fatalf("delete #%d: %v", i+1, err)
		}
	}
	if _, ok := fake.Get(testBucket, "tea-x/obk-y/a"); ok {
		t.Fatal("object still stored")
	}
}

func TestListPagesWithStartAfter(t *testing.T) {
	store, fake := newTestStore(t)
	for i := range 7 {
		fake.Put(testBucket, fmt.Sprintf("tea-x/obk-y/k%02d", i), []byte("v"))
	}
	fake.Put(testBucket, "tea-x/obk-other/k", []byte("v"))
	fake.Put(testBucket, "tea-z/obk-y/k", []byte("v"))

	var got []string
	start := ""
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("listing does not end")
		}
		page, err := store.List(context.Background(), "tea-x/obk-y/", start, 3)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range page.Objects {
			got = append(got, o.Key)
		}
		if page.Next == "" {
			break
		}
		start = page.Next
	}
	if len(got) != 7 || got[0] != "tea-x/obk-y/k00" || got[6] != "tea-x/obk-y/k06" {
		t.Fatalf("paged keys = %v, want the seven of the prefix in order and nothing from the neighbours", got)
	}
	for _, bad := range []int{0, -1, objectstorage.ListMaxLimit + 1} {
		if _, err := store.List(context.Background(), "p/", "", bad); err == nil {
			t.Errorf("List with limit %d did not fail", bad)
		}
	}
}

func TestDeletePrefixRemovesEverythingUnderItAndNothingElse(t *testing.T) {
	store, fake := newTestStore(t)
	for i := range 2500 { // more than one 1000-key page
		fake.Put(testBucket, fmt.Sprintf("tea-x/obk-y/k%04d", i), []byte("v"))
	}
	fake.Put(testBucket, "tea-x/obk-other/keep", []byte("v"))
	fake.Put(testBucket, "tea-xx/obk-y/keep", []byte("v")) // shares a string prefix, not a path prefix
	n, err := store.DeletePrefix(context.Background(), "tea-x/")
	if err != nil || n != 2501 {
		t.Fatalf("DeletePrefix = %d, %v; want 2501", n, err)
	}
	if left := fake.Keys(testBucket, ""); len(left) != 1 || left[0] != "tea-xx/obk-y/keep" {
		t.Fatalf("left behind: %v", left)
	}
}

func TestStoreSurfacesAnUnavailableBackend(t *testing.T) {
	store, fake := newTestStore(t)
	fake.Put(testBucket, "tea-x/obk-y/a", []byte("x"))
	fake.SetDown(true)
	ctx := context.Background()
	if _, err := store.Head(ctx, "tea-x/obk-y/a"); err == nil || errors.Is(err, objectstorage.ErrObjectNotFound) {
		t.Fatalf("Head while down = %v, want a real error, not not-found", err)
	}
	if _, err := store.List(ctx, "tea-x/", "", 10); err == nil {
		t.Fatal("List while down did not fail")
	}
	if n, err := store.DeletePrefix(ctx, "tea-x/"); err == nil || n != 0 {
		t.Fatalf("DeletePrefix while down = %d, %v", n, err)
	}
	if err := store.Delete(ctx, "tea-x/obk-y/a"); err == nil {
		t.Fatal("Delete while down did not fail")
	}
	fake.SetDown(false)
	if _, err := store.Head(ctx, "tea-x/obk-y/a"); err != nil {
		t.Fatalf("Head after recovery: %v", err)
	}
}
