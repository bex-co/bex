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

package upgrade

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadCappedEnforcesLimit(t *testing.T) {
	if _, err := readCapped(strings.NewReader("hello"), 5); err != nil {
		t.Errorf("exactly at the cap must succeed: %v", err)
	}
	got, err := readCapped(strings.NewReader("hi"), 5)
	if err != nil || string(got) != "hi" {
		t.Errorf("under the cap: got %q err %v", got, err)
	}
	if _, err := readCapped(strings.NewReader("too many bytes"), 5); err == nil {
		t.Error("over the cap must error rather than allocate past it")
	}
}

func TestArchiveName(t *testing.T) {
	got := archiveName("1.4.2", "darwin", "arm64")
	if got != "bex-1.4.2-darwin-arm64.tar.gz" {
		t.Errorf("archiveName = %q", got)
	}
}

func TestChecksumFor(t *testing.T) {
	checksums := []byte(
		"aaaa1111  bex-1.0.0-linux-amd64.tar.gz\n" +
			"BBBB2222 *bex-1.0.0-darwin-arm64.tar.gz\n" + // binary-mode marker
			"\n" +
			"# comment line ignored\n",
	)
	cases := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"bex-1.0.0-linux-amd64.tar.gz", "aaaa1111", false},
		{"bex-1.0.0-darwin-arm64.tar.gz", "bbbb2222", false}, // lowercased, * stripped
		{"bex-1.0.0-windows-amd64.tar.gz", "", true},
	}
	for _, c := range cases {
		got, err := checksumFor(checksums, c.name)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: want error", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestExtractBinary(t *testing.T) {
	content := []byte("ELF-ish binary bytes")
	archive := makeArchive(t, "3.1.0", content)

	got, err := extractBinary(archive, binaryName)
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("extracted %q, want %q", got, content)
	}

	if _, err := extractBinary(archive, "not-there"); err == nil {
		t.Error("want error for a missing binary in the archive")
	}
	if _, err := extractBinary([]byte("not a gzip"), binaryName); err == nil {
		t.Error("want error for a non-gzip archive")
	}
}
