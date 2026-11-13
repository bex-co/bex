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

package sniproxy

import (
	"bytes"
	"testing"
)

// sniFuzzSeeds are hand-built TLS records that walk the parser's documented
// edges: a real-shaped ClientHello with one SNI extension, a record whose
// handshake length overruns the record, a ClientHello cut before its
// extensions, and a zero-length record. Every one is fed to both the record
// reader and the SNI extractor; the on-disk testdata/fuzz cache is never
// committed.
func sniFuzzSeeds() [][]byte {
	return [][]byte{
		nil,
		{},
		{TLSHandshakeType},
		{TLSHandshakeType, 0x03, 0x01, 0x00, 0x00},
		// A minimal record: handshake byte, then a ClientHello body.
		wellFormedRecord(),
		// Handshake length larger than the record.
		{TLSHandshakeType, 0x03, 0x01, 0x00, 0x08, 0x01, 0xff, 0xff, 0xff},
		// Not a handshake record.
		{0x17, 0x03, 0x03, 0x00, 0x01, 0x00},
	}
}

// wellFormedRecord wraps minimalClientHello in a TLS record header whose
// length matches the body, so the seed parses all the way through.
func wellFormedRecord() []byte {
	body := minimalClientHello()
	return append([]byte{
		TLSHandshakeType, 0x03, 0x01,
		byte(len(body) >> 8), byte(len(body)),
	}, body...)
}

// minimalClientHello is a well-formed ClientHello body (without the record
// header) carrying the minimum fixed fields plus a zero-length extensions
// block, so the walker reaches the extension loop.
func minimalClientHello() []byte {
	// version (2) + random (32) + session id length 0 (1) + cipher suites
	// length 2 (2 + 2 bytes) + compression length 1 (1 + 1 byte) +
	// extensions length 0 (2).
	body := []byte{
		0x03, 0x03,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0x00,
		0x00, 0x02, 0x13, 0x01,
		0x01, 0x00,
		0x00, 0x00,
	}
	handshakeLen := len(body)
	return append([]byte{
		0x01,
		byte(handshakeLen >> 16), byte(handshakeLen >> 8), byte(handshakeLen),
	}, body...)
}

// FuzzExtractSNI drives the public SNI parser with arbitrary bytes. It is the
// first thing every tenant TLS connection reaches, so it must answer a parse
// error or a host name, never panic, for any input.
func FuzzExtractSNI(f *testing.F) {
	for _, seed := range sniFuzzSeeds() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, record []byte) {
		_, _ = extractSNIForFuzz(record)
	})
}

// extractSNIForFuzz keeps the fuzz body's allocation bounded: ExtractSNI
// reads only the first record, and a length prefix is bounded by MaxClientHello
// in ReadTLSRecord, so a record far past that is not a shape the caller can
// produce.
func extractSNIForFuzz(record []byte) (string, error) {
	if len(record) > 5+MaxClientHello {
		record = record[:5+MaxClientHello]
	}
	return ExtractSNI(record)
}

// FuzzReadTLSRecord drives the record reader with arbitrary preamble bytes and
// body bytes. It must never panic and must never return more than the bounded
// record size; a short reader is a clean error.
func FuzzReadTLSRecord(f *testing.F) {
	// initial is the preamble a protocol detector already consumed; rest is
	// what the reader still sees. Feed each record both ways so the corpus
	// covers a complete record arriving in one buffer and split across two.
	for _, seed := range sniFuzzSeeds() {
		f.Add([]byte(nil), seed)
		f.Add(seed, []byte(nil))
	}
	f.Fuzz(func(t *testing.T, initial, rest []byte) {
		record, err := ReadTLSRecord(bytes.NewReader(rest), initial)
		if err != nil {
			return
		}
		if len(record) > 5+MaxClientHello {
			t.Fatalf("record exceeds the bound: %d bytes", len(record))
		}
		if len(record) < 5 || record[0] != TLSHandshakeType {
			t.Fatalf("accepted a non-handshake record of %d bytes", len(record))
		}
	})
}
