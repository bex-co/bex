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

package staticserver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/andybalholm/brotli"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The QA a895 file: 57 UTF-8 bytes the uploader stored as a bare text/plain.
const utf8Text = "QA_A895_UNICODE café 雪 + percent% newline\nsecond line\n"

// s3Object is what the fake bucket answers for one key.
type s3Object struct {
	contentType, contentEncoding, body string
}

// fakeS3Origin is the real S3Origin against a bucket that answers GetObject
// from objects with the metadata the publisher stored, and counts reads per
// key.
func fakeS3Origin(t *testing.T, objects map[string]s3Object) (*S3Origin, func(key string) int) {
	t.Helper()
	var mu sync.Mutex
	gets := map[string]int{}
	origin := newTestS3Origin(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/bex-static/")
		mu.Lock()
		gets[key]++
		mu.Unlock()
		obj, ok := objects[key]
		if r.Method != http.MethodGet || !ok {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code></Error>`))
			return
		}
		// A nil type stops net/http sniffing one, as an object stored
		// without a type has none.
		w.Header()["Content-Type"] = nil
		if obj.contentType != "" {
			w.Header().Set("Content-Type", obj.contentType)
		}
		if obj.contentEncoding != "" {
			w.Header().Set("Content-Encoding", obj.contentEncoding)
		}
		_, _ = w.Write([]byte(obj.body))
	}))
	return origin, func(key string) int {
		mu.Lock()
		defer mu.Unlock()
		return gets[key]
	}
}

// A valid UTF-8 text/plain object that names no charset is declared UTF-8;
// every other stored type, an explicit charset and bytes that are not UTF-8
// keep what the origin sent. The bytes never change (w4/335).
func TestS3OriginDeclaresUTF8OnlyOnUndeclaredUTF8PlainText(t *testing.T) {
	latin1 := "caf\xe9\n"
	cases := []struct {
		name string
		obj  s3Object
		want string
	}{
		{"UTF-8 text", s3Object{contentType: "text/plain", body: utf8Text}, "text/plain; charset=utf-8"},
		{"ASCII text", s3Object{contentType: "text/plain", body: "hello\n"}, "text/plain; charset=utf-8"},
		{"empty text", s3Object{contentType: "text/plain"}, "text/plain; charset=utf-8"},
		{"respelled type", s3Object{contentType: "Text/Plain", body: utf8Text}, "text/plain; charset=utf-8"},
		{"other parameters", s3Object{contentType: "text/plain; format=flowed", body: utf8Text}, "text/plain; charset=utf-8; format=flowed"},
		{"declared UTF-8", s3Object{contentType: "text/plain; charset=UTF-8", body: utf8Text}, "text/plain; charset=UTF-8"},
		{"declared legacy charset", s3Object{contentType: "text/plain; charset=iso-8859-1", body: latin1}, "text/plain; charset=iso-8859-1"},
		{"invalid UTF-8", s3Object{contentType: "text/plain", body: latin1}, "text/plain"},
		{"encoded body", s3Object{contentType: "text/plain", contentEncoding: "gzip", body: "hello\n"}, "text/plain"},
		{"JSON", s3Object{contentType: "application/json", body: `{"name":"café"}`}, "application/json"},
		{"other text type", s3Object{contentType: "text/csv", body: "café,1\n"}, "text/csv"},
		{"binary", s3Object{contentType: "image/png", body: "\x89PNG\r\n"}, "image/png"},
		{"octet-stream", s3Object{contentType: "application/octet-stream", body: utf8Text}, "application/octet-stream"},
		{"malformed type", s3Object{contentType: "text/plain; =bad", body: utf8Text}, "text/plain; =bad"},
		{"no stored type", s3Object{body: utf8Text}, ""},
	}
	objects := map[string]s3Object{}
	for _, tc := range cases {
		objects[tc.name] = tc.obj
	}
	origin, _ := fakeS3Origin(t, objects)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := origin.Get(context.Background(), tc.name)
			if err != nil {
				t.Fatal(err)
			}
			if got.ContentType != tc.want || !bytes.Equal(got.Body, []byte(tc.obj.body)) {
				t.Errorf("Get = %q with %d bytes, want %q with the %d stored bytes", got.ContentType, len(got.Body), tc.want, len(tc.obj.body))
			}
		})
	}
}

// A visitor reading the bare text/plain upload, directly or through a
// rewrite, by GET or HEAD, is told it is UTF-8; so is one reading a text file
// stored with no type or as octet-stream, through the extension fallback. The
// object is read once, so no request rescans it, and an owner's Content-Type
// rule still wins.
func TestStaticUTF8TextIsServedAsUTF8(t *testing.T) {
	origin, gets := fakeS3Origin(t, map[string]s3Object{
		key("files/hello.txt"):   {contentType: "text/plain", body: utf8Text},
		key("files/untyped.txt"): {body: utf8Text},
		key("files/octet.txt"):   {contentType: "application/octet-stream", body: utf8Text},
		key("override/note.txt"): {contentType: "text/plain", body: utf8Text},
	})
	h := New(staticResolver{host: testHost, site: Site{
		AppID: appID, Revision: rev,
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/friendly", Destination: "/files/hello.txt"}},
		Headers: []appv1alpha1.StaticHeader{
			{Path: "/override/*", Name: "Content-Type", Value: "text/plain; charset=iso-8859-1"},
		},
	}}, origin, 1<<20)
	served := func(path, wantType string) {
		t.Helper()
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := do(h, method, path)
			wantBody := utf8Text
			if method == http.MethodHead {
				wantBody = ""
			}
			if ct := rec.Header().Get("Content-Type"); rec.Code != http.StatusOK || ct != wantType || rec.Body.String() != wantBody {
				t.Errorf("%s %s = %d %q with %d bytes, want 200 %q with %d bytes", method, path, rec.Code, ct, rec.Body.Len(), wantType, len(wantBody))
			}
		}
	}
	for _, path := range []string{"/files/hello.txt", "/friendly", "/files/untyped.txt", "/files/octet.txt"} {
		served(path, "text/plain; charset=utf-8")
	}
	served("/override/note.txt", "text/plain; charset=iso-8859-1")
	if n := gets(key("files/hello.txt")); n != 1 {
		t.Errorf("origin read the file %d times, want once", n)
	}
}

// A declared UTF-8 type changes neither a byte range nor automatic Brotli:
// both still serve the stored bytes, under the declared type.
func TestStaticUTF8TextKeepsRangesAndCompression(t *testing.T) {
	long := strings.Repeat(utf8Text, 20) // past minBrotliBytes
	origin, _ := fakeS3Origin(t, map[string]s3Object{key("files/long.txt"): {contentType: "text/plain", body: long}})
	h := New(staticResolver{host: testHost, site: Site{AppID: appID, Revision: rev}}, origin, 1<<20)

	rec := doWith(h, http.MethodGet, "/files/long.txt", http.Header{"Range": {"bytes=0-14"}})
	if ct := rec.Header().Get("Content-Type"); rec.Code != http.StatusPartialContent || ct != "text/plain; charset=utf-8" || rec.Body.String() != long[:15] {
		t.Errorf("range = %d %q %q, want 206 text/plain; charset=utf-8 %q", rec.Code, ct, rec.Body, long[:15])
	}

	rec = doWith(h, http.MethodGet, "/files/long.txt", http.Header{"Accept-Encoding": {"br"}})
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "br" || rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("br = %d encoding %q type %q, want 200 br text/plain; charset=utf-8", rec.Code, rec.Header().Get("Content-Encoding"), rec.Header().Get("Content-Type"))
	}
	if body, err := io.ReadAll(brotli.NewReader(rec.Body)); err != nil || string(body) != long {
		t.Errorf("br body decodes to %d bytes (%v), want the %d stored bytes", len(body), err, len(long))
	}
}
