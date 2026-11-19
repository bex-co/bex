// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package s3metadatacase is a Traefik local HTTP middleware on the S3 gateway's
// route (w21/113). Amazon S3 names a user-metadata header in lowercase
// (`x-amz-meta-mtime`), and botocore, so boto3 and the AWS CLI, hands a caller
// the part of the header name after the prefix exactly as it came, so
// `head["Metadata"]["mtime"]` is S3's answer and `{"Mtime": ...}` is not.
//
// bex-s3 cannot send the lowercase name through the edge on its own: Traefik
// reads a backend's response with net/http, which canonicalises every header
// name on the way in (`X-Amz-Meta-Mtime`), and copies it out in that form. A
// header map key that is not canonical is written to an HTTP/1.1 client as it
// stands (net/http: "to use non-canonical keys, assign to the map directly"),
// so this middleware, which runs on the way out, renames every
// `X-Amz-Meta-*` key in the response's header map to lowercase just before
// the head is written. An HTTP/2 client got lowercase names already.
package bex_s3_metadata_case

import (
	"context"
	"net/http"
	"strings"
)

// metadataPrefix is the lowercase prefix of S3's user-metadata response headers.
const metadataPrefix = "x-amz-meta-"

type Config struct{}

func CreateConfig() *Config { return &Config{} }

type lowercaser struct{ next http.Handler }

func New(_ context.Context, next http.Handler, _ *Config, _ string) (http.Handler, error) {
	return &lowercaser{next: next}, nil
}

func (l *lowercaser) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.next.ServeHTTP(&writer{ResponseWriter: w}, r)
}

// writer renames the metadata headers once, just before the response head is
// written: by WriteHeader, which the proxy always calls, or by the first Write
// of a handler that never did.
//
// Only WriteHeader and Write are overridden. Under Traefik's Yaegi
// interpreter compiled code sees an interpreted ResponseWriter without Flush
// (see clientipheaders' originServerWriter), so the proxy behind this writer
// cannot flush. That costs an S3 response nothing: a get has a Content-Length,
// and net/http flushes its own 4 KiB buffer as it fills and the rest when the
// handler returns.
type writer struct {
	http.ResponseWriter
	renamed bool
}

func (w *writer) rename() {
	w.renamed = true
	h := w.ResponseWriter.Header()
	for key, values := range h {
		if len(key) <= len(metadataPrefix) || !strings.EqualFold(key[:len(metadataPrefix)], metadataPrefix) {
			continue
		}
		if lower := strings.ToLower(key); lower != key {
			delete(h, key)
			h[lower] = append(h[lower], values...)
		}
	}
}

func (w *writer) WriteHeader(code int) {
	// Every call renames: an informational answer (100, 103) precedes the real
	// one, and the headers set after it are the final head's.
	w.rename()
	w.ResponseWriter.WriteHeader(code)
}

func (w *writer) Write(b []byte) (int, error) {
	if !w.renamed {
		w.rename()
	}
	return w.ResponseWriter.Write(b)
}
