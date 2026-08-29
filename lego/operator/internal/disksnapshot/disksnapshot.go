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

// Package disksnapshot is the byte pipeline behind persistent-disk snapshots
// (docs/ADR082-persistent-disks.md D5): a directory tree in, one encrypted
// stream out, and back again.
//
// Hetzner has no volume snapshots at any level — server snapshots exclude
// attached volumes and the CSI driver has no snapshot capability — so Render's
// disk-snapshot semantics have to be reproduced at the file level. This package
// is that mechanism, deliberately separated from both S3 and Kubernetes so the
// part that can silently lose a tenant's data is testable as pure bytes: every
// round trip below runs against a real temporary directory in unit tests, with
// no cluster and no object store involved.
//
// The stream is tar → gzip → age, in one pass with nothing staged on disk. The
// KeyValue backup pipeline stages its RDB through an EmptyDir, which is fine
// for a 5 GB datastore and impossible for a disk that may be 10 TB.
package disksnapshot

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

// Backup writes root's contents to w as a gzip-compressed tar encrypted to
// recipient. The recipient is an age X25519 public key, so the object that
// lands in the store decrypts with the stock `age` CLI as well as with Restore.
//
// Symlinks are archived as symlinks rather than followed: a tenant's volume may
// contain a link pointing outside the mount, and following it would copy data
// the disk does not own into the disk's own snapshot.
func Backup(root string, w io.Writer, recipient string) error {
	id, err := age.ParseX25519Recipient(recipient)
	if err != nil {
		return fmt.Errorf("parse recipient: %w", err)
	}
	encrypted, err := age.Encrypt(w, id)
	if err != nil {
		return fmt.Errorf("open age writer: %w", err)
	}
	gz := gzip.NewWriter(encrypted)
	archive := tar.NewWriter(gz)

	if err := writeTree(archive, root); err != nil {
		return err
	}
	// Close in order, innermost first: each layer flushes into the next, and a
	// dropped error here is a truncated snapshot that still looks like a
	// success — the exact failure a backup must never have.
	if err := archive.Close(); err != nil {
		return fmt.Errorf("finish tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("finish gzip: %w", err)
	}
	if err := encrypted.Close(); err != nil {
		return fmt.Errorf("finish age: %w", err)
	}
	return nil
}

func writeTree(archive *tar.Writer, root string) error {
	return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			// Directories, symlinks, and anything else carry no payload. Device
			// nodes and sockets are archived as their header alone rather than
			// skipped, so a restore reproduces the tree's shape.
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(archive, file)
		// A read error matters; a close error on a read-only handle does not,
		// but it must not mask the copy's.
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		return err
	})
}

// Restore replaces root's contents with the snapshot in r, decrypting with
// identity (an age X25519 secret key).
//
// It is destructive by design — Render's restore discards everything written
// after the snapshot — but it is also re-runnable: the snapshot stays in the
// object store, so a restore interrupted halfway is fixed by running it again
// rather than by having kept a copy of data the caller asked to replace.
func Restore(root string, r io.Reader, identity string) error {
	key, err := age.ParseX25519Identity(strings.TrimSpace(identity))
	if err != nil {
		return fmt.Errorf("parse identity: %w", err)
	}
	decrypted, err := age.Decrypt(r, key)
	if err != nil {
		return fmt.Errorf("open age reader: %w", err)
	}
	gz, err := gzip.NewReader(decrypted)
	if err != nil {
		return fmt.Errorf("open gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	if err := clearTree(root); err != nil {
		return err
	}
	return extractTree(tar.NewReader(gz), root)
}

// clearTree empties root without removing root itself — it is the mount point,
// and removing it would detach the volume from under the pod.
func clearTree(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read %s: %w", root, err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return fmt.Errorf("clear %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func extractTree(archive *tar.Reader, root string) error {
	rooted, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open extraction root: %w", err)
	}
	defer func() { _ = rooted.Close() }()
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		target, err := safeJoin(root, header.Name)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, target)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := rooted.MkdirAll(name, header.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := rooted.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				return err
			}
			// os.Root rejects absolute link targets and any relative target that
			// escapes root. Later writes through a symlink are constrained by the
			// same rooted handle, closing both forms of tar extraction escape.
			if err := rooted.Symlink(header.Linkname, name); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(rooted, name, archive, header.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		default:
			// Anything else (fifo, device, hard link) is reproduced only as far
			// as its parent directory; a tenant filesystem that needs them is
			// outside what a file-level snapshot can promise, and inventing a
			// device node from an archive is worse than not having one.
			if err := rooted.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				return err
			}
		}
	}
}

func writeFile(root *os.Root, name string, src io.Reader, mode fs.FileMode) error {
	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	file, err := root.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, src); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// safeJoin resolves an archive entry against root and refuses anything that
// would land outside it. A snapshot is tenant-controlled data — a crafted
// "../../etc/passwd" entry would otherwise let a restore write anywhere the
// Job's filesystem allows, which is the classic tar-extraction escape.
func safeJoin(root, name string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry %q escapes the disk", name)
	}
	target := filepath.Join(root, cleaned)
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry %q escapes the disk", name)
	}
	return target, nil
}
