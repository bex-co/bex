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
	"sort"
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

// writeTree walks root through an os.Root so every path component is confined to
// the volume. The tenant application keeps a writable mount on the same PVC while
// this read-only backup runs, so the tree is a live, hostile filesystem: the old
// filepath.Walk + os.Open pair collected lstat metadata and later re-resolved the
// pathname, and a tenant could race a checked regular file into a symlink to
// /proc/self/environ between those steps — capturing the backup process's own S3
// credential into the encrypted snapshot (codex finding-3, CWE-367). Resolving
// through os.Root closes that gap two ways: an escaping symlink swapped in before
// the open is rejected rather than followed, and each regular file's tar header
// is built from the OPENED descriptor's fstat, so the archived type and size
// describe the bytes actually copied.
func writeTree(archive *tar.Writer, root string) error {
	rooted, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open backup root: %w", err)
	}
	defer func() { _ = rooted.Close() }()
	return walkTree(archive, rooted, ".")
}

// walkTree archives dir (a slash path relative to the backup root) and, depth
// first, everything beneath it. It never descends a symlink — a link is recorded
// as a link, whatever it points at — mirroring the original's "a tenant's volume
// may contain a link pointing outside the mount, and following it would copy data
// the disk does not own into the disk's own snapshot" contract.
func walkTree(archive *tar.Writer, root *os.Root, dir string) error {
	d, err := root.Open(dir)
	if err != nil {
		return err
	}
	entries, err := d.ReadDir(-1)
	_ = d.Close()
	if err != nil {
		return err
	}
	// Stable order keeps snapshots of the same tree byte-reproducible, the way
	// filepath.Walk's lexical order did.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := entry.Name()
		if dir != "." {
			name = dir + "/" + name
		}
		// Lstat, not Stat: the final component is described as itself, so a
		// symlink is seen as a symlink rather than resolved.
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			link, err := root.Readlink(name)
			if err != nil {
				return err
			}
			if err := writeHeader(archive, info, link, name); err != nil {
				return err
			}
		case info.IsDir():
			if err := writeHeader(archive, info, "", name); err != nil {
				return err
			}
			if err := walkTree(archive, root, name); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := writeRegularFile(archive, root, name); err != nil {
				return err
			}
		default:
			// Device nodes, fifos, and sockets carry no payload and are archived
			// as their header alone, so a restore reproduces the tree's shape.
			if err := writeHeader(archive, info, "", name); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeRegularFile opens name through the root, builds the tar header from the
// opened descriptor (not the earlier Lstat), and copies exactly the header's
// worth of bytes.
func writeRegularFile(archive *tar.Writer, root *os.Root, name string) error {
	file, err := root.Open(name)
	if err != nil {
		// An escaping symlink raced in after the Lstat above resolves outside the
		// root and lands here as an error — fail the snapshot loudly rather than
		// follow it. The Job's backoff retries; a tenant only ever races its own.
		return err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		// The path became a non-regular inode (an in-root symlink to a directory,
		// say) between the Lstat and this open. Archive the header alone; a
		// snapshot never copies payload from an inode it did not verify as regular.
		return writeHeader(archive, info, "", name)
	}
	if err := writeHeader(archive, info, "", name); err != nil {
		return err
	}
	size := info.Size()
	n, err := io.Copy(archive, io.LimitReader(file, size))
	if err != nil {
		return err
	}
	if n < size {
		// A live volume can truncate the file between the fstat above and this
		// copy. tar requires exactly header.Size bytes, so pad the shortfall
		// rather than emit a malformed member or fail a benign concurrent write.
		if _, err := io.CopyN(archive, zeroReader{}, size-n); err != nil {
			return err
		}
	}
	return nil
}

func writeHeader(archive *tar.Writer, info fs.FileInfo, link, name string) error {
	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	header.Name = name
	return archive.WriteHeader(header)
}

// zeroReader yields an unbounded run of NUL bytes — padding for a tar member
// whose file shrank between its fstat and copy.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
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
