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

// Package upgrade implements `bex upgrade`: an in-place self-update for
// raw-binary installs. It resolves the newest bex-cli/v* GitHub release,
// downloads the per-target archive, verifies it (the release's cosign
// signature over checksums.txt, then that archive's checksum), and atomically
// replaces the running binary — refusing when a package manager owns the file.
package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bex-co/bex/cli/internal/update"
	"github.com/spf13/cobra"
)

// maxDownloadBytes caps the archive download; the archives are tens of MiB, so
// this is a generous ceiling that still refuses a hostile unbounded body.
// maxMetaBytes bounds the tiny verification assets (checksums + signature) far
// more tightly so a misdirected URL can't force a 256 MiB allocation for them.
const (
	maxDownloadBytes = 256 << 20
	maxMetaBytes     = 1 << 20
)

// downloadClient is separate from update's short-fused checker: fetching an
// archive legitimately takes longer than listing releases.
var downloadClient = &http.Client{Timeout: 2 * time.Minute}

// upgrader holds the self-update flow with every side-effecting dependency
// behind a field, so tests drive it without network, disk, or sigstore.
type upgrader struct {
	currentVersion string
	goos, goarch   string
	execPath       string
	checkOnly      bool
	stdout         io.Writer
	stderr         io.Writer

	fetchRelease func() (update.FullRelease, error)
	download     func(url string, max int64) ([]byte, error)
	verifySig    func(checksums, sigBundle []byte) error
}

// Command returns the `bex upgrade` cobra command. currentVersion is main's
// build-injected bexVersion.
func Command(currentVersion string) *cobra.Command {
	checkOnly := false
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Update bex to the latest release",
		Long: "Downloads the newest bex-cli release, verifies its cosign signature and\n" +
			"checksum, and replaces the running binary in place. Installs managed by a\n" +
			"package manager (e.g. Homebrew) are left untouched with an upgrade hint.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, _ []string) error {
			execPath, err := runningBinaryPath()
			if err != nil {
				return err
			}
			u := &upgrader{
				currentVersion: currentVersion,
				goos:           runtime.GOOS,
				goarch:         runtime.GOARCH,
				execPath:       execPath,
				checkOnly:      checkOnly,
				stdout:         c.OutOrStdout(),
				stderr:         c.ErrOrStderr(),
				fetchRelease:   update.NewChecker(os.LookupEnv).LatestRelease,
				download:       httpDownload,
				verifySig:      verifySignature,
			}
			return u.run()
		},
	}
	cmd.Flags().BoolVarP(&checkOnly, "check", "n", false, "report whether an upgrade is available without installing it")
	return cmd
}

func (u *upgrader) run() error {
	if !update.IsReleaseBuild(u.currentVersion) {
		return fmt.Errorf("self-update is unavailable for a dev build; install a released bex-cli/v* binary")
	}

	// A package-manager-owned binary must never be overwritten out from under
	// the manager — deflect before any network work.
	if hint, owned := packageManagerHint(u.execPath); owned {
		fmt.Fprint(u.stdout, hint)
		return nil
	}

	release, err := u.fetchRelease()
	if err != nil {
		return fmt.Errorf("resolve latest bex-cli release: %w", err)
	}
	if !update.Newer(u.currentVersion, release.Version) {
		fmt.Fprintf(u.stdout, "bex is already up to date (v%s)\n", u.currentVersion)
		return nil
	}
	if u.checkOnly {
		fmt.Fprintf(u.stdout, "A new release of bex is available: v%s → v%s\n%s\nRun `bex upgrade` to install it.\n", u.currentVersion, release.Version, release.URL)
		return nil
	}

	archiveName := archiveName(release.Version, u.goos, u.goarch)
	archiveURL, err := requireAsset(release, archiveName)
	if err != nil {
		return err
	}
	checksumsURL, err := requireAsset(release, checksumsAsset)
	if err != nil {
		return err
	}
	sigURL, err := requireAsset(release, signatureAsset)
	if err != nil {
		return err
	}

	fmt.Fprintf(u.stderr, "Downloading bex v%s (%s/%s)...\n", release.Version, u.goos, u.goarch)
	archive, err := u.download(archiveURL, maxDownloadBytes)
	if err != nil {
		return fmt.Errorf("download %s: %w", archiveName, err)
	}
	checksums, err := u.download(checksumsURL, maxMetaBytes)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	sigBundle, err := u.download(sigURL, maxMetaBytes)
	if err != nil {
		return fmt.Errorf("download signature: %w", err)
	}

	// Verify the signature over checksums.txt first (it makes checksums.txt
	// trustworthy), then confirm the archive matches its signed checksum.
	// Either failure aborts before the running binary is touched.
	if err := u.verifySig(checksums, sigBundle); err != nil {
		return fmt.Errorf("signature verification failed, refusing to upgrade: %w", err)
	}
	want, err := checksumFor(checksums, archiveName)
	if err != nil {
		return err
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch for %s, refusing to upgrade", archiveName)
	}

	binary, err := extractBinary(archive, binaryName)
	if err != nil {
		return err
	}
	if err := replaceBinary(u.execPath, binary); err != nil {
		return err
	}
	fmt.Fprintf(u.stdout, "Upgraded bex v%s → v%s\n", u.currentVersion, release.Version)
	return nil
}

// runningBinaryPath resolves the on-disk path of the current executable,
// following symlinks so a Homebrew bin symlink resolves to its Cellar target.
func runningBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve running binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

func requireAsset(release update.FullRelease, name string) (string, error) {
	url, ok := release.Asset(name)
	if !ok {
		return "", fmt.Errorf("release %s is missing asset %q", release.Tag, name)
	}
	return url, nil
}

// httpDownload fetches url in full, bounded by max so a hostile or truncated
// body can't exhaust memory.
func httpDownload(url string, max int64) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return readCapped(resp.Body, max)
}

// readCapped reads r fully but errors rather than allocating past max bytes.
func readCapped(r io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("exceeds %d bytes", max)
	}
	return data, nil
}
