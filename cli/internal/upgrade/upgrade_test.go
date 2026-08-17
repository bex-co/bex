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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/cli/internal/update"
)

const (
	testGOOS   = "linux"
	testGOARCH = "amd64"
)

// makeArchive builds a .tar.gz shaped like a real release archive: a single
// per-target directory holding the bex binary.
func makeArchive(t *testing.T, version string, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	inner := fmt.Sprintf("bex-%s-%s-%s/%s", version, testGOOS, testGOARCH, binaryName)
	if err := tw.WriteHeader(&tar.Header{Name: inner, Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatalf("write tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// release fixture: a newer 2.0.0 release with the three verification-relevant
// assets. The download map keys are the asset URLs.
type fixture struct {
	release   update.FullRelease
	downloads map[string][]byte
	archive   []byte
	newBinary []byte
}

func newFixture(t *testing.T, newBinary []byte) *fixture {
	t.Helper()
	const version = "2.0.0"
	archive := makeArchive(t, version, newBinary)
	archiveAsset := archiveName(version, testGOOS, testGOARCH)
	checksums := []byte(sha256Hex(archive) + "  " + archiveAsset + "\n" +
		sha256Hex([]byte("unrelated")) + "  bex-2.0.0-darwin-arm64.tar.gz\n")
	sig := []byte(`{"sigstore":"bundle"}`)
	return &fixture{
		release: update.FullRelease{
			Version: version,
			Tag:     "bex-cli/v" + version,
			URL:     "https://example.test/releases/bex-cli/v" + version,
			Assets: []update.Asset{
				{Name: archiveAsset, URL: "asset://archive"},
				{Name: checksumsAsset, URL: "asset://checksums"},
				{Name: signatureAsset, URL: "asset://sig"},
			},
		},
		downloads: map[string][]byte{
			"asset://archive":   archive,
			"asset://checksums": checksums,
			"asset://sig":       sig,
		},
		archive:   archive,
		newBinary: newBinary,
	}
}

func (f *fixture) download(url string, _ int64) ([]byte, error) {
	b, ok := f.downloads[url]
	if !ok {
		return nil, fmt.Errorf("unexpected download %q", url)
	}
	return b, nil
}

// newUpgrader wires an upgrader against a temp-dir "current binary" and the
// fixture's seams. verifySig defaults to accepting.
func newUpgrader(t *testing.T, f *fixture, current string) (*upgrader, string, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	execPath := filepath.Join(dir, "bex")
	if err := os.WriteFile(execPath, []byte("OLD-BINARY"), 0o755); err != nil {
		t.Fatalf("seed current binary: %v", err)
	}
	out := &bytes.Buffer{}
	u := &upgrader{
		currentVersion: current,
		goos:           testGOOS,
		goarch:         testGOARCH,
		execPath:       execPath,
		stdout:         out,
		stderr:         &bytes.Buffer{},
		fetchRelease:   func() (update.FullRelease, error) { return f.release, nil },
		download:       f.download,
		verifySig:      func(_, _ []byte) error { return nil },
	}
	return u, execPath, out
}

func TestRunUpgradesToNewerRelease(t *testing.T) {
	f := newFixture(t, []byte("NEW-BINARY-CONTENT"))
	u, execPath, out := newUpgrader(t, f, "1.0.0")

	var gotChecksums, gotSig []byte
	u.verifySig = func(checksums, sig []byte) error {
		gotChecksums, gotSig = checksums, sig
		return nil
	}

	if err := u.run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if !bytes.Equal(got, f.newBinary) {
		t.Errorf("binary not replaced: got %q", got)
	}
	if !strings.Contains(out.String(), "Upgraded bex v1.0.0 → v2.0.0") {
		t.Errorf("missing success line: %q", out.String())
	}
	// The signature is verified over the checksums bytes, and the bundle is
	// the downloaded signature asset.
	if !bytes.Equal(gotChecksums, f.downloads["asset://checksums"]) {
		t.Error("verifySig did not receive the checksums.txt bytes")
	}
	if !bytes.Equal(gotSig, f.downloads["asset://sig"]) {
		t.Error("verifySig did not receive the signature bundle bytes")
	}
	// Installed binary is executable.
	fi, err := os.Stat(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("installed binary not executable: %v", fi.Mode())
	}
}

func TestRunNoopWhenCurrent(t *testing.T) {
	f := newFixture(t, []byte("NEW"))
	u, execPath, out := newUpgrader(t, f, "2.0.0")
	downloadCalled := false
	u.download = func(url string, max int64) ([]byte, error) { downloadCalled = true; return f.download(url, max) }

	if err := u.run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if downloadCalled {
		t.Error("must not download when already current")
	}
	if !strings.Contains(out.String(), "already up to date (v2.0.0)") {
		t.Errorf("missing up-to-date line: %q", out.String())
	}
	if got, _ := os.ReadFile(execPath); string(got) != "OLD-BINARY" {
		t.Errorf("binary must be untouched: %q", got)
	}
}

func TestRunCheckOnlyDoesNotInstall(t *testing.T) {
	f := newFixture(t, []byte("NEW"))
	u, execPath, out := newUpgrader(t, f, "1.0.0")
	u.checkOnly = true
	downloadCalled := false
	u.download = func(url string, max int64) ([]byte, error) { downloadCalled = true; return f.download(url, max) }

	if err := u.run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if downloadCalled {
		t.Error("--check must not download")
	}
	if !strings.Contains(out.String(), "v1.0.0 → v2.0.0") {
		t.Errorf("missing availability line: %q", out.String())
	}
	if got, _ := os.ReadFile(execPath); string(got) != "OLD-BINARY" {
		t.Errorf("binary must be untouched: %q", got)
	}
}

func TestRunAbortsOnBadSignatureLeavingBinaryIntact(t *testing.T) {
	f := newFixture(t, []byte("MALICIOUS"))
	u, execPath, _ := newUpgrader(t, f, "1.0.0")
	u.verifySig = func(_, _ []byte) error { return fmt.Errorf("identity mismatch") }

	err := u.run()
	if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
		t.Fatalf("want signature failure, got %v", err)
	}
	if got, _ := os.ReadFile(execPath); string(got) != "OLD-BINARY" {
		t.Errorf("binary must be untouched after signature failure: %q", got)
	}
}

func TestRunAbortsOnChecksumMismatchLeavingBinaryIntact(t *testing.T) {
	f := newFixture(t, []byte("TAMPERED"))
	// Corrupt the checksums entry for the archive so the SHA-256 no longer
	// matches, even though the (faked) signature over it "verifies".
	f.downloads["asset://checksums"] = []byte(sha256Hex([]byte("wrong")) + "  " + archiveName("2.0.0", testGOOS, testGOARCH) + "\n")
	u, execPath, _ := newUpgrader(t, f, "1.0.0")

	err := u.run()
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum mismatch, got %v", err)
	}
	if got, _ := os.ReadFile(execPath); string(got) != "OLD-BINARY" {
		t.Errorf("binary must be untouched after checksum mismatch: %q", got)
	}
}

func TestRunDevBuildRefuses(t *testing.T) {
	f := newFixture(t, []byte("NEW"))
	u, _, _ := newUpgrader(t, f, "dev")
	err := u.run()
	if err == nil || !strings.Contains(err.Error(), "dev build") {
		t.Fatalf("want dev-build refusal, got %v", err)
	}
}

func TestRunBrewInstallDeflects(t *testing.T) {
	f := newFixture(t, []byte("NEW"))
	u, _, out := newUpgrader(t, f, "1.0.0")
	u.execPath = "/opt/homebrew/Cellar/bex/1.0.0/bin/bex"
	fetchCalled := false
	u.fetchRelease = func() (update.FullRelease, error) { fetchCalled = true; return f.release, nil }

	if err := u.run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if fetchCalled {
		t.Error("brew-owned install must deflect before any network work")
	}
	if !strings.Contains(out.String(), "brew upgrade bex") {
		t.Errorf("missing brew hint: %q", out.String())
	}
}

func TestRunMissingAssetIsAnError(t *testing.T) {
	f := newFixture(t, []byte("NEW"))
	// Drop the per-target archive asset from the release.
	f.release.Assets = f.release.Assets[1:]
	u, _, _ := newUpgrader(t, f, "1.0.0")
	err := u.run()
	if err == nil || !strings.Contains(err.Error(), "missing asset") {
		t.Fatalf("want missing-asset error, got %v", err)
	}
}
