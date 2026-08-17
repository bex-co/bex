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
	"fmt"
	"io"
	"path"
	"strings"
)

const (
	// binaryName is the executable inside each release archive.
	binaryName = "bex"
	// checksumsAsset and signatureAsset are the shared verification assets
	// scripts/bex-cli-build.sh + cli-release.yml attach to every release.
	checksumsAsset = "checksums.txt"
	signatureAsset = "checksums.txt.sigstore.json"
)

// archiveName mirrors cli-release.yml's `bex-${VERSION}-${os}-${arch}.tar.gz`
// naming (VERSION is the bare X.Y.Z the release tag normalizes to).
func archiveName(version, goos, goarch string) string {
	return fmt.Sprintf("bex-%s-%s-%s.tar.gz", version, goos, goarch)
}

// checksumFor returns the lowercase hex SHA-256 that checksums.txt records for
// filename. The format is `sha256sum`'s: `<hash>  <name>` (text mode) or
// `<hash> *<name>` (binary mode); both are accepted.
func checksumFor(checksums []byte, filename string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == filename {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", filename)
}

// extractBinary returns the bytes of the named executable inside a .tar.gz
// release archive (cli-release.yml packs it under a per-target directory).
func extractBinary(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || path.Base(hdr.Name) != name {
			continue
		}
		data, err := readCapped(tr, maxDownloadBytes)
		if err != nil {
			return nil, fmt.Errorf("read %s from archive: %w", name, err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("archive did not contain a %q binary", name)
}
