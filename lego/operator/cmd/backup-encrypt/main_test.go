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
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// backup is a stand-in for a compressed RDB: incompressible bytes larger than
// age's 64 KiB chunk size, so the multi-chunk path and its final
// authentication tag are actually exercised rather than a single short chunk.
func backup(t *testing.T) []byte {
	t.Helper()
	buf := make([]byte, 200*1024)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	return buf
}

func writeBackup(t *testing.T, dir string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, "dump.rdb.gz")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestEncryptRoundTripsThroughTheAgeFormat is the load-bearing assertion of
// w7/m85's t003: replacing the downloaded `age` binary with a compiled-in
// library must not change the object the restore path consumes.
// scripts/lib/restore.sh decrypts with the stock `age -d -i <identity>`, so the
// output has to be a standard age file for one X25519 recipient — decrypted
// here through the same public format, byte-for-byte.
func TestEncryptRoundTripsThroughTheAgeFormat(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	plaintext := backup(t)
	in := writeBackup(t, dir, plaintext)
	out := filepath.Join(dir, "dump.rdb.gz.age")

	if err := run([]string{in, out}, identity.Recipient().String()); err != nil {
		t.Fatal(err)
	}

	ciphertext, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// The v1 header is what makes the file readable by the `age` CLI at all;
	// assert it before decrypting so a format change fails with a clear reason.
	if !bytes.HasPrefix(ciphertext, []byte("age-encryption.org/v1\n")) {
		t.Fatalf("output is not an age v1 file: %q", ciphertext[:min(len(ciphertext), 32)])
	}
	if bytes.Contains(ciphertext, plaintext[:512]) {
		t.Fatal("plaintext survives in the encrypted object")
	}

	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("decrypted %d bytes, want the original %d", len(got), len(plaintext))
	}

	// The upload stage shares this volume; the plaintext must be gone.
	if _, err := os.Stat(in); !os.IsNotExist(err) {
		t.Fatalf("plaintext RDB survived the encrypt stage: %v", err)
	}

	// The upload stage runs as a different uid with DAC_OVERRIDE dropped, so an
	// owner-only object is invisible to it and the backup silently fails (the
	// 2026-08-23 production kvbak outage). The ciphertext must be world-readable.
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o044 != 0o044 {
		t.Fatalf("ciphertext mode %v is not readable by the upload stage's uid", info.Mode().Perm())
	}
}

// TestEncryptIsNotArmored guards the object's shape against a silently doubled
// size: the pipeline uploads binary, and armor would inflate an already-large
// backup by a third for no benefit.
func TestEncryptIsNotArmored(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "dump.rdb.gz.age")
	if err := run([]string{writeBackup(t, dir, backup(t)), out}, identity.Recipient().String()); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(ciphertext, []byte(armor.Header)) {
		t.Fatal("output is ASCII-armored; the pipeline uploads binary age objects")
	}
}

// TestEncryptRejectsUnusableInput covers the ways the stage can be misconfigured
// or fed the wrong thing. Every one must fail loudly AND leave no output file:
// the upload stage ships whatever is at the ciphertext path, so a partial or
// absent-recipient artifact left behind would be uploaded as if it were a
// backup, and the plaintext must survive for the retried Job.
func TestEncryptRejectsUnusableInput(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		recipient string
		args      func(dir, in, out string) []string
		want      string
	}{
		{"no recipient", "", func(_, in, out string) []string { return []string{in, out} }, "AGE_PUBLIC_KEY"},
		{"malformed recipient", "not-an-age-key",
			func(_, in, out string) []string { return []string{in, out} }, "parse recipient"},
		{"ssh recipient", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample",
			func(_, in, out string) []string { return []string{in, out} }, "parse recipient"},
		{"missing input", identity.Recipient().String(),
			func(dir, _, out string) []string { return []string{filepath.Join(dir, "absent.gz"), out} },
			"no such file"},
		{"wrong arity", identity.Recipient().String(),
			func(_, in, _ string) []string { return []string{in} }, "usage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			in := writeBackup(t, dir, backup(t))
			out := filepath.Join(dir, "dump.rdb.gz.age")

			err := run(tc.args(dir, in, out), tc.recipient)
			if err == nil {
				t.Fatal("expected a failure")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v does not mention %q", err, tc.want)
			}
			if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
				t.Fatalf("a failed encrypt left an object the upload stage would ship: %v", statErr)
			}
			if _, statErr := os.Stat(in); statErr != nil {
				t.Fatalf("a failed encrypt destroyed the snapshot the retry needs: %v", statErr)
			}
		})
	}
}
