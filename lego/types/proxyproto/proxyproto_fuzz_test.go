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

package proxyproto

import (
	"bufio"
	"bytes"
	"io"
	"net/netip"
	"testing"
)

// proxyProtoFuzzSeeds are the header shapes the package's tests exercise — a
// v1 IPv4 and IPv6 line, an UNKNOWN transport, a v2 address block, and the
// near-miss protocols whose first byte collides with a header — plus the edges
// the hand-rolled parser must survive: a lone "P", a bare v2 signature, a
// truncated length field, and a length longer than the buffer. Seeds live as
// source so a reviewer sees exactly what the untrusted surface is probed with;
// the on-disk testdata/fuzz cache is never committed.
func proxyProtoFuzzSeeds(f *testing.F) {
	f.Helper()
	for _, seed := range []string{
		"",
		"P",
		"PROXY",
		"PROXY ",
		"PROXY UNKNOWN\r\n",
		"PROXY TCP4 203.0.113.9 49.12.20.236 49152 6379\r\n",
		"PROXY TCP6 2001:db8::9 2a01:4f8:c01e:3d1f::1 49152 5432\r\n",
		"PROXY TCP4 203.0.113.9 49.12.20.236 0 6379\r\n",
		"PROXY TCP4 2001:db8::9 49.12.20.236 49152 6379\r\n",
		"PROXY TCP4 203.0.113.9 49.12.20.236 49152\r\n",
		"PROXY TCP4 203.0.113.9 49.12.20.236 49152 6379\n",
		"POST / HTTP/1.1\r\n",
		"\r\nhello",
		string(proxyV2Signature),
		string(proxyV2Signature) + "\x21\x11\x00\x0c" + "seven\x00\x01\x00\x02",
		string(proxyV2Signature) + "\x20\x12\x00\x00",
		string(proxyV2Signature) + "\x20\x11\x00\x24" + "short",
	} {
		f.Add(seed)
	}
}

// FuzzReadProxySource drives the single PROXY protocol parser every SSH gateway
// and SNI-proxy connection enters through. The peer is a trusted edge, the
// bytes are whatever the connection sends, so a header from that edge is
// consumed and anything else must pass through byte for byte. It must never
// panic, and on an untrusted peer a real header must always be refused.
func FuzzReadProxySource(f *testing.F) {
	proxyProtoFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, header string) {
		raw := []byte(header)
		trusted := tcpAddr("10.10.0.7")
		reader := bufio.NewReader(bytes.NewReader(raw))
		got, err := ReadProxySource(reader, trusted, trustedEdge())
		if err == nil && !got.IsValid() {
			t.Fatalf("ReadProxySource(%q) accepted with an invalid address", header)
		}

		// A headerless protocol is passed through untouched: the immediate peer
		// is the source and every byte stays buffered for the caller. An empty
		// connection is the one non-header input with no byte to peek, and it
		// is an error by design.
		if len(raw) == 0 {
			return
		}
		v1Header := len(raw) >= len(proxyV1Prefix) && bytes.HasPrefix(raw, []byte(proxyV1Prefix))
		v2Header := len(raw) >= len(proxyV2Signature) && bytes.HasPrefix(raw, proxyV2Signature)
		if !v1Header && !v2Header {
			if err != nil {
				t.Fatalf("ReadProxySource(%q) rejected a headerless protocol: %v", header, err)
			}
			if got != netip.MustParseAddr("10.10.0.7") {
				t.Fatalf("ReadProxySource(%q) = %s, want the immediate peer", header, got)
			}
			remaining, readErr := io.ReadAll(reader)
			if readErr != nil || !bytes.Equal(remaining, raw) {
				t.Fatalf("ReadProxySource(%q) lost headerless bytes: %q, %v", header, remaining, readErr)
			}
			return
		}

		// The same header from an untrusted peer is refused rather than honored.
		untrustedReader := bufio.NewReader(bytes.NewReader(raw))
		if _, err := ReadProxySource(untrustedReader, tcpAddr("198.51.100.9"), trustedEdge()); err == nil {
			t.Fatalf("ReadProxySource(%q) honored a PROXY header from an untrusted peer", header)
		}
	})
}
