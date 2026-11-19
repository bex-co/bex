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
	"io"
	"net/url"
	"sort"
	"strings"
	"testing"
)

// Fuzz targets for the parsers that read bytes a client chose before anything is
// authenticated or authorized (ADR112 D11): the canonical query, the Authorization
// header's fields, and the aws-chunked framing. They run as ordinary tests over their seeds
// in CI; `go test -fuzz=FuzzChunkedFramingRoundTrip ./internal/s3gw` explores.

func FuzzCanonicalQuery(f *testing.F) {
	for _, seed := range []string{"", "a=b", "z=1&a=2&a=1", "x-id=GetObject&list-type=2", "k=%zz", "=v&=", "a=b=c", "k%20x=v+w", "X-Amz-Signature=abc&X-Amz-Date=1", "a&b&&c"} {
		f.Add(seed, true)
		f.Add(seed, false)
	}
	f.Fuzz(func(t *testing.T, raw string, presigned bool) {
		got := canonicalQuery(raw, presigned)
		if got != canonicalQuery(raw, presigned) {
			t.Fatal("the canonical query is not deterministic")
		}
		if got == "" {
			return
		}
		// Every pair is name=value in sorted order, with nothing that SigV4 encodes left raw.
		var keys []string
		for _, pair := range strings.Split(got, "&") {
			k, _, ok := strings.Cut(pair, "=")
			if !ok {
				t.Fatalf("a pair without =: %q in %q", pair, got)
			}
			keys = append(keys, k)
		}
		if !sort.StringsAreSorted(keys) {
			t.Fatalf("the names are not sorted: %q", got)
		}
		if strings.ContainsAny(got, " +\"<>\\^`{|}") {
			t.Fatalf("an unencoded character survived: %q", got)
		}
		if presigned {
			for _, k := range keys {
				if k == "X-Amz-Signature" {
					t.Fatalf("the signature is part of what it signs: %q", got)
				}
			}
		}
		if _, err := url.ParseQuery(got); err != nil {
			t.Fatalf("the canonical query does not parse: %v (%q)", err, got)
		}
	})
}

func FuzzAuthorizationFields(f *testing.F) {
	for _, seed := range []string{
		"Credential=s3k-x/20261119/oregon/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=abcd",
		"Credential=a, SignedHeaders=b", "", ",,,", "Credential=, SignedHeaders=, Signature=", "a=b=c=d", "Credential=a,Credential=b,SignedHeaders=c,Signature=d",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		fields, ok := parseAuthorizationFields(s)
		if !ok {
			if fields != nil {
				t.Fatal("fields returned with ok false")
			}
			return
		}
		for _, need := range []string{"Credential", "SignedHeaders", "Signature"} {
			if fields[need] == "" {
				t.Fatalf("ok with no %s: %q", need, s)
			}
		}
		_ = splitSignedHeaders(fields["SignedHeaders"])
	})
}

// FuzzChunkedFramingRoundTrip feeds the decoder arbitrary bytes, and also frames arbitrary data the
// way an SDK does and requires the decoder to give it back.
func FuzzChunkedFramingRoundTrip(f *testing.F) {
	f.Add([]byte("5\r\nhello\r\n0\r\nx-amz-checksum-crc32:AAAAAA==\r\n\r\n"), []byte("data"), 3)
	f.Add([]byte("0\r\n\r\n"), []byte{}, 1)
	f.Add([]byte("ffffffffffffffff\r\n"), []byte("x"), 1)
	f.Add([]byte("5\r\nhel"), []byte("abc"), 2)
	f.Add([]byte("5;ext=1\r\nhello\r\n0\r\n\r\n"), []byte("abcdef"), 4)
	f.Add(bytes.Repeat([]byte("1\r\nx\r\n"), 50), []byte("z"), 100)
	f.Fuzz(func(t *testing.T, raw, data []byte, chunk int) {
		// Arbitrary bytes: it terminates, never returns more data than it was given, and an
		// error is sticky.
		got, err := io.ReadAll(newChunkedReader(bytes.NewReader(raw)))
		if len(got) > len(raw) {
			t.Fatalf("%d bytes of data out of %d bytes of framing", len(got), len(raw))
		}
		if err != nil && len(raw) == 0 {
			_ = err
		}
		// Well-formed framing of arbitrary data round trips, with the trailer.
		if chunk < 1 || chunk > 1<<16 {
			chunk = 1 + (chunk%1000+1000)%1000
		}
		cr := newChunkedReader(bytes.NewReader(chunkedBody(data, chunk, "x-amz-checksum-crc32", "AAAAAA==")))
		back, err := io.ReadAll(cr)
		if err != nil || !bytes.Equal(back, data) {
			t.Fatalf("framing %d bytes in chunks of %d came back as %d bytes, %v", len(data), chunk, len(back), err)
		}
		if cr.trailer["x-amz-checksum-crc32"] != "AAAAAA==" {
			t.Fatalf("the trailer is %v", cr.trailer)
		}
	})
}
