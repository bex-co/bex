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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	smithy "github.com/aws/smithy-go"
)

func TestS3OriginCheckVerifiesSignedListAccess(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Errorf("S3 check request was not SigV4 signed")
		}
		if r.Method == http.MethodHead && r.URL.Path == "/bex-static/site/rev-1/index.html" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/bex-static" ||
			r.URL.Query().Get("list-type") != "2" || r.URL.Query().Get("max-keys") != "1" {
			t.Errorf("unexpected S3 check request: %s %s", r.Method, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/xml")
		if _, err := fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>bex-static</Name><KeyCount>1</KeyCount><MaxKeys>1</MaxKeys><IsTruncated>false</IsTruncated>
  <Contents><Key>site/rev-1/index.html</Key><Size>5</Size></Contents>
</ListBucketResult>`); err != nil {
			t.Errorf("write S3 list response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	origin, err := NewS3Origin(context.Background(), server.URL, "us-east-1", "bex-static")
	if err != nil {
		t.Fatal(err)
	}
	if err := origin.Check(context.Background()); err != nil {
		t.Fatalf("Check() = %v, want success", err)
	}
	if requests != 2 {
		t.Fatalf("Check() made %d requests, want signed ListObjects + HeadObject", requests)
	}
}

func TestS3OriginCheckFailsClosedWhenObjectReadIsDenied(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusForbidden)
			if _, err := fmt.Fprint(w, `<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`); err != nil {
				t.Errorf("write S3 denial response: %v", err)
			}
			return
		}
		if _, err := fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>bex-static</Name><KeyCount>1</KeyCount><MaxKeys>1</MaxKeys><IsTruncated>false</IsTruncated>
  <Contents><Key>site/rev-1/index.html</Key><Size>5</Size></Contents>
</ListBucketResult>`); err != nil {
			t.Errorf("write S3 list response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	origin, err := NewS3Origin(context.Background(), server.URL, "us-east-1", "bex-static")
	if err != nil {
		t.Fatal(err)
	}
	if err := origin.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "object read access") {
		t.Fatalf("Check() = %v, want actionable GetObject failure", err)
	}
}

func TestS3OriginCheckFailsClosedOnDeniedCredential(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		if _, err := fmt.Fprint(w, `<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`); err != nil {
			t.Errorf("write S3 denial response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	origin, err := NewS3Origin(context.Background(), server.URL, "us-east-1", "bex-static")
	if err != nil {
		t.Fatal(err)
	}
	if err := origin.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "verify read access") {
		t.Fatalf("Check() = %v, want actionable access failure", err)
	}
}

func TestIsNotFoundRecognizesKeyTooLong(t *testing.T) {
	// Defense in depth for w6/047: the handler rejects over-long keys up front,
	// but should one ever reach the store, its KeyTooLongError client error means
	// "can never exist" and must map to ErrNotFound (404), not a 502.
	err := fmt.Errorf("get %q: %w", "key", &smithy.GenericAPIError{Code: "KeyTooLongError", Message: "your key is too long"})
	if !isNotFound(err) {
		t.Errorf("isNotFound(KeyTooLongError) = false, want true")
	}
}
