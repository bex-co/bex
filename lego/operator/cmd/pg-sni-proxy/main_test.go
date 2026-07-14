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

package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildClientHello constructs a minimal TLS 1.3-shaped ClientHello record
// containing a single SNI extension for the given hostname. Used to exercise
// the extractSNI and readTLSRecord paths.
func buildClientHello(sni string) []byte {
	// SNI extension data: list_len(2) + name_type(1) + name_len(2) + name
	sniData := []byte{0x00, byte(len(sni) >> 8), byte(len(sni))}
	sniData = append(sniData, []byte(sni)...)
	listLen := len(sniData)
	sniExt := []byte{byte(listLen >> 8), byte(listLen)}
	sniExt = append(sniExt, sniData...)
	extLen := len(sniExt)
	// Extension: type(2) + len(2) + data
	ext := []byte{0x00, 0x00, byte(extLen >> 8), byte(extLen)}
	ext = append(ext, sniExt...)

	// Extensions block: total_len(2) + extensions
	exts := []byte{byte(len(ext) >> 8), byte(len(ext))}
	exts = append(exts, ext...)

	// Minimal ClientHello body:
	// legacy_version(2) + random(32) + session_id_len(1) +
	// cipher_suites_len(2) + cipher_suite(2) + comp_methods_len(1) + comp_method(1) + extensions
	chBody := make([]byte, 34) // version + random
	chBody[0] = 0x03
	chBody[1] = 0x03
	chBody = append(chBody, 0x00)       // session_id_len = 0
	chBody = append(chBody, 0x00, 0x02) // cipher_suites_len = 2
	chBody = append(chBody, 0x00, 0x2f) // TLS_RSA_WITH_AES_128_CBC_SHA
	chBody = append(chBody, 0x01, 0x00) // compression_methods: 1 byte, null
	chBody = append(chBody, exts...)

	// Handshake header: type(1) + length(3)
	hsLen := len(chBody)
	hs := []byte{0x01, byte(hsLen >> 16), byte(hsLen >> 8), byte(hsLen)}
	hs = append(hs, chBody...)

	// TLS record header: content_type(1) + version(2) + length(2)
	rec := make([]byte, 5+len(hs))
	rec[0] = 0x16 // Handshake
	rec[1] = 0x03
	rec[2] = 0x01
	binary.BigEndian.PutUint16(rec[3:5], uint16(len(hs)))
	copy(rec[5:], hs)
	return rec
}

func TestExtractSNI(t *testing.T) {
	tests := []string{
		"mydb.db.bex.co",
		"mydb-pool.db.bex.co",
		"mydb-ro-east.db.bex.co",
	}
	for _, sni := range tests {
		t.Run(sni, func(t *testing.T) {
			record := buildClientHello(sni)
			got, err := extractSNI(record)
			if err != nil {
				t.Fatalf("extractSNI() error: %v", err)
			}
			if got != sni {
				t.Errorf("extractSNI() = %q, want %q", got, sni)
			}
		})
	}
}

func TestExtractSNI_InvalidInputs(t *testing.T) {
	if sni, _ := extractSNI([]byte{0x15, 0, 0, 0, 0}); sni != "" {
		t.Error("expected empty SNI for non-handshake record")
	}
	if sni, err := extractSNI(nil); err == nil || sni != "" {
		t.Error("expected error for nil record")
	}
}

func TestRouterResolve(t *testing.T) {
	r := newRouter("db.bex.co")
	r.set("mydb", "tenant-ns")

	tests := []struct {
		sni    string
		want   string
		wantOk bool
	}{
		{"mydb.db.bex.co", "mydb-rw.tenant-ns.svc.cluster.local:5432", true},
		{"mydb-pool.db.bex.co", "mydb-pooler.tenant-ns.svc.cluster.local:5432", true},
		{"mydb-ro-east.db.bex.co", "mydb-ro.tenant-ns.svc.cluster.local:5432", true},
		{"mydb-ro-reader-a.db.bex.co", "mydb-ro.tenant-ns.svc.cluster.local:5432", true},
		{"unknown.db.bex.co", "", false},
		{"mydb.wrong.domain", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.sni, func(t *testing.T) {
			got, ok := r.resolve(tt.sni)
			if ok != tt.wantOk {
				t.Fatalf("resolve(%q) ok = %v, want %v", tt.sni, ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("resolve(%q) = %q, want %q", tt.sni, got, tt.want)
			}
		})
	}

	// Deletion removes the entry.
	r.delete("mydb")
	if _, ok := r.resolve("mydb.db.bex.co"); ok {
		t.Error("expected resolve to fail after delete")
	}
}

func TestReadTLSRecord_DirectTLS(t *testing.T) {
	// Simulate the direct-TLS path: first 8 bytes already peeked, rest from reader.
	record := buildClientHello("test.db.bex.co")
	initial := record[:8]
	rest := record[8:]

	got, err := readTLSRecord(bytes.NewReader(rest), initial)
	if err != nil {
		t.Fatalf("readTLSRecord() error: %v", err)
	}
	if !bytes.Equal(got, record) {
		t.Errorf("readTLSRecord() = %d bytes, want %d", len(got), len(record))
	}

	sni, err := extractSNI(got)
	if err != nil {
		t.Fatalf("extractSNI() after readTLSRecord: %v", err)
	}
	if sni != "test.db.bex.co" {
		t.Errorf("SNI from reassembled record = %q, want %q", sni, "test.db.bex.co")
	}
}

func TestReadTLSRecord_FreshStart(t *testing.T) {
	// Simulate the SSLRequest path: no bytes pre-peeked, read from scratch.
	record := buildClientHello("fresh.db.bex.co")

	got, err := readTLSRecord(bytes.NewReader(record), nil)
	if err != nil {
		t.Fatalf("readTLSRecord() error: %v", err)
	}
	if !bytes.Equal(got, record) {
		t.Errorf("readTLSRecord() bytes mismatch: got %d, want %d", len(got), len(record))
	}
}

func TestRouterResolve_EmptyDomain(t *testing.T) {
	r := newRouter("")
	r.set("mydb", "ns")
	if _, ok := r.resolve("mydb.db.bex.co"); ok {
		t.Error("resolve should fail when domain is empty")
	}
}
