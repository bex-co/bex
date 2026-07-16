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
	"net/netip"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// buildClientHello constructs a minimal TLS 1.3-shaped ClientHello record
// containing a single SNI extension for the given hostname. Used to exercise
// the shared SNI parser and TLS record reader paths.
func buildClientHello(sni string) []byte {
	// SNI extension data: list_len(2) + name_type(1) + name_len(2) + name
	sniData := make([]byte, 0, 3+len(sni))
	sniData = append(sniData, 0x00, byte(len(sni)>>8), byte(len(sni)))
	sniData = append(sniData, []byte(sni)...)
	listLen := len(sniData)
	sniExt := make([]byte, 0, 2+len(sniData))
	sniExt = append(sniExt, byte(listLen>>8), byte(listLen))
	sniExt = append(sniExt, sniData...)
	extLen := len(sniExt)
	// Extension: type(2) + len(2) + data
	ext := make([]byte, 0, 4+len(sniExt))
	ext = append(ext, 0x00, 0x00, byte(extLen>>8), byte(extLen))
	ext = append(ext, sniExt...)

	// Extensions block: total_len(2) + extensions
	exts := make([]byte, 0, 2+len(ext))
	exts = append(exts, byte(len(ext)>>8), byte(len(ext)))
	exts = append(exts, ext...)

	// Minimal ClientHello body:
	// legacy_version(2) + random(32) + session_id_len(1) +
	// cipher_suites_len(2) + cipher_suite(2) + comp_methods_len(1) + comp_method(1) + extensions
	chBody := make([]byte, 34, 41+len(exts)) // version + random
	chBody[0] = 0x03
	chBody[1] = 0x03
	chBody = append(chBody, 0x00)       // session_id_len = 0
	chBody = append(chBody, 0x00, 0x02) // cipher_suites_len = 2
	chBody = append(chBody, 0x00, 0x2f) // TLS_RSA_WITH_AES_128_CBC_SHA
	chBody = append(chBody, 0x01, 0x00) // compression_methods: 1 byte, null
	chBody = append(chBody, exts...)

	// Handshake header: type(1) + length(3)
	hsLen := len(chBody)
	hs := make([]byte, 0, 4+len(chBody))
	hs = append(hs, 0x01, byte(hsLen>>16), byte(hsLen>>8), byte(hsLen))
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
			got, err := sniproxy.ExtractSNI(record)
			if err != nil {
				t.Fatalf("ExtractSNI() error: %v", err)
			}
			if got != sni {
				t.Errorf("ExtractSNI() = %q, want %q", got, sni)
			}
		})
	}
}

func TestExtractSNI_InvalidInputs(t *testing.T) {
	if sni, _ := sniproxy.ExtractSNI([]byte{0x15, 0, 0, 0, 0}); sni != "" {
		t.Error("expected empty SNI for non-handshake record")
	}
	if sni, err := sniproxy.ExtractSNI(nil); err == nil || sni != "" {
		t.Error("expected error for nil record")
	}
}

func TestRouterResolve(t *testing.T) {
	r := newRouter("db.bex.co")
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "mydb", Namespace: "tenant-ns"},
		Spec: appv1alpha1.DatabaseSpec{
			Public: true, Pooler: true,
			ReadReplicas:           []appv1alpha1.DatabaseReadReplica{{Name: "east"}, {Name: "reader-a"}},
			IPAllowList:            []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24"}},
			EnvironmentIPAllowList: []string{"203.0.113.0/28"},
		},
	}
	if err := r.set(db); err != nil {
		t.Fatal(err)
	}

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
			got, ok := r.resolve(tt.sni, netip.MustParseAddr("203.0.113.9"))
			if ok != tt.wantOk {
				t.Fatalf("resolve(%q) ok = %v, want %v", tt.sni, ok, tt.wantOk)
			}
			if got.Backend != tt.want {
				t.Errorf("resolve(%q) backend = %q, want %q", tt.sni, got.Backend, tt.want)
			}
			if ok && got.Database != "mydb" {
				t.Errorf("resolve(%q) Database = %q, want parent mydb", tt.sni, got.Database)
			}
		})
	}

	// Deletion removes the entry.
	r.delete("mydb")
	if _, ok := r.resolve("mydb.db.bex.co", netip.MustParseAddr("203.0.113.9")); ok {
		t.Error("expected resolve to fail after delete")
	}
	if err := r.set(db); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.resolve("mydb.db.bex.co", netip.MustParseAddr("198.51.100.9")); ok {
		t.Error("non-allowlisted source resolved")
	}
	if _, ok := r.resolve("mydb.db.bex.co", netip.MustParseAddr("203.0.113.20")); ok {
		t.Error("source outside the environment allowlist resolved")
	}
	db.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "broken"}}
	if err := r.set(db); err == nil {
		t.Error("invalid allowlist must fail source health")
	}
	valid := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "valid", Namespace: "ns"},
		Spec:       appv1alpha1.DatabaseSpec{Public: true},
	}
	if err := r.set(valid); err != nil {
		t.Fatal(err)
	}
	if r.healthy() {
		t.Error("a later valid reconcile masked another Database's invalid route")
	}
	r.delete(db.Name)
	if !r.healthy() {
		t.Error("deleting the invalid Database did not restore source health")
	}
}

func TestReadTLSRecord_DirectTLS(t *testing.T) {
	// Simulate the direct-TLS path: first 8 bytes already peeked, rest from reader.
	record := buildClientHello("test.db.bex.co")
	initial := record[:8]
	rest := record[8:]

	got, err := sniproxy.ReadTLSRecord(bytes.NewReader(rest), initial)
	if err != nil {
		t.Fatalf("ReadTLSRecord() error: %v", err)
	}
	if !bytes.Equal(got, record) {
		t.Errorf("ReadTLSRecord() = %d bytes, want %d", len(got), len(record))
	}

	sni, err := sniproxy.ExtractSNI(got)
	if err != nil {
		t.Fatalf("ExtractSNI() after ReadTLSRecord: %v", err)
	}
	if sni != "test.db.bex.co" {
		t.Errorf("SNI from reassembled record = %q, want %q", sni, "test.db.bex.co")
	}
}

func TestReadTLSRecord_FreshStart(t *testing.T) {
	// Simulate the SSLRequest path: no bytes pre-peeked, read from scratch.
	record := buildClientHello("fresh.db.bex.co")

	got, err := sniproxy.ReadTLSRecord(bytes.NewReader(record), nil)
	if err != nil {
		t.Fatalf("ReadTLSRecord() error: %v", err)
	}
	if !bytes.Equal(got, record) {
		t.Errorf("ReadTLSRecord() bytes mismatch: got %d, want %d", len(got), len(record))
	}
}

func TestRouterResolve_EmptyDomain(t *testing.T) {
	r := newRouter("")
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "mydb", Namespace: "ns"},
		Spec:       appv1alpha1.DatabaseSpec{Public: true},
	}
	if err := r.set(db); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.resolve("mydb.db.bex.co", netip.MustParseAddr("1.1.1.1")); ok {
		t.Error("resolve should fail when domain is empty")
	}
}
