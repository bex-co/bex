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

package disksnapshot

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func newKeypair(t *testing.T) (recipient, identity string) {
	t.Helper()
	key, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key.Recipient().String(), key.String()
}

// tree writes a directory shape and returns it. The shapes here are the ones a
// real tenant volume has: nested directories, an empty file, a large binary
// file that crosses buffer boundaries, unusual permissions, and a symlink.
func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "app.conf"), []byte("key = value\n"), 0o644)
	mustWrite(t, filepath.Join(root, "nested", "deep", "data.json"), []byte(`{"a":1}`), 0o600)
	mustWrite(t, filepath.Join(root, "empty"), nil, 0o644)

	big := make([]byte, 1<<20) // 1 MiB of incompressible bytes
	if _, err := rand.Read(big); err != nil {
		t.Fatalf("random: %v", err)
	}
	mustWrite(t, filepath.Join(root, "blob.bin"), big, 0o644)

	if err := os.Symlink("app.conf", filepath.Join(root, "current.conf")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "emptydir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return root
}

func mustWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// snapshot returns a directory's contents as a comparable map, so a round trip
// can be asserted on content and mode rather than on "no error".
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			out[rel] = "symlink->" + link
		case info.IsDir():
			out[rel] = "dir"
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[rel] = string(data) + "|" + info.Mode().Perm().String()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// The defining test: whatever was on the volume comes back byte for byte, with
// its permissions and symlinks, through a real encrypted stream.
func TestRoundTripRestoresTheTreeExactly(t *testing.T) {
	recipient, identity := newKeypair(t)
	src := tree(t)
	want := snapshot(t, src)

	var archive bytes.Buffer
	if err := Backup(src, &archive, recipient); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	dst := t.TempDir()
	if err := Restore(dst, bytes.NewReader(archive.Bytes()), identity); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got := snapshot(t, dst)
	if len(got) != len(want) {
		t.Fatalf("restored %d entries, want %d\n got: %v\nwant: %v", len(got), len(want), keys(got), keys(want))
	}
	for name, wantEntry := range want {
		if got[name] != wantEntry {
			t.Errorf("%s: restored %q, want %q", name, got[name], wantEntry)
		}
	}
}

// Restore discards post-snapshot writes — that is Render's documented behavior
// and the reason it warns before running one.
func TestRestoreDiscardsEverythingWrittenAfterTheSnapshot(t *testing.T) {
	recipient, identity := newKeypair(t)
	volume := tree(t)
	var archive bytes.Buffer
	if err := Backup(volume, &archive, recipient); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// The tenant keeps working after the snapshot: a new file, an edit, a delete.
	mustWrite(t, filepath.Join(volume, "written-later.txt"), []byte("new"), 0o644)
	mustWrite(t, filepath.Join(volume, "app.conf"), []byte("edited\n"), 0o644)
	if err := os.RemoveAll(filepath.Join(volume, "nested")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if err := Restore(volume, bytes.NewReader(archive.Bytes()), identity); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if _, err := os.Stat(filepath.Join(volume, "written-later.txt")); !os.IsNotExist(err) {
		t.Error("a file created after the snapshot survived the restore")
	}
	data, err := os.ReadFile(filepath.Join(volume, "app.conf"))
	if err != nil || string(data) != "key = value\n" {
		t.Errorf("app.conf = %q (%v), want the snapshot's contents back", data, err)
	}
	if _, err := os.Stat(filepath.Join(volume, "nested", "deep", "data.json")); err != nil {
		t.Errorf("a directory deleted after the snapshot was not restored: %v", err)
	}
}

// Restoring in place must not remove the mount point itself; deleting it would
// detach the volume from under the pod.
func TestRestoreKeepsTheMountPointItself(t *testing.T) {
	recipient, identity := newKeypair(t)
	src := tree(t)
	var archive bytes.Buffer
	if err := Backup(src, &archive, recipient); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	volume := t.TempDir()
	before, err := os.Stat(volume)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if err := Restore(volume, bytes.NewReader(archive.Bytes()), identity); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	after, err := os.Stat(volume)
	if err != nil {
		t.Fatalf("mount point is gone after restore: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Error("the mount point was recreated rather than emptied; the volume would be detached")
	}
}

// A snapshot is tenant-controlled data. A crafted path must not let a restore
// write outside the volume it is restoring.
func TestRestoreRefusesPathsThatEscapeTheDisk(t *testing.T) {
	recipient, identity := newKeypair(t)
	for _, name := range []string{"../escaped", "../../etc/passwd", "/etc/passwd", "nested/../../escaped"} {
		t.Run(name, func(t *testing.T) {
			var archive bytes.Buffer
			writeHostileArchive(t, &archive, recipient, name)

			volume := t.TempDir()
			err := Restore(volume, bytes.NewReader(archive.Bytes()), identity)
			if err == nil {
				t.Fatalf("Restore accepted %q", name)
			}
			if !strings.Contains(err.Error(), "escapes the disk") {
				t.Fatalf("error = %v, want an escape refusal", err)
			}
		})
	}
}

func writeHostileArchive(t *testing.T, w io.Writer, recipient, name string) {
	t.Helper()
	id, err := age.ParseX25519Recipient(recipient)
	if err != nil {
		t.Fatalf("recipient: %v", err)
	}
	encrypted, err := age.Encrypt(w, id)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	gz := gzip.NewWriter(encrypted)
	archive := tar.NewWriter(gz)
	body := []byte("owned")
	if err := archive.WriteHeader(&tar.Header{
		Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("header: %v", err)
	}
	if _, err := archive.Write(body); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, closer := range []io.Closer{archive, gz, encrypted} {
		if err := closer.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

// The wrong key must fail loudly rather than producing an empty volume: a
// restore that "succeeds" by wiping and extracting nothing is data loss.
func TestRestoreWithTheWrongKeyFailsBeforeTouchingTheVolume(t *testing.T) {
	recipient, _ := newKeypair(t)
	_, otherIdentity := newKeypair(t)
	src := tree(t)
	var archive bytes.Buffer
	if err := Backup(src, &archive, recipient); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	volume := tree(t)
	before := snapshot(t, volume)
	if err := Restore(volume, bytes.NewReader(archive.Bytes()), otherIdentity); err == nil {
		t.Fatal("Restore accepted a snapshot it could not decrypt")
	}
	if got := snapshot(t, volume); len(got) != len(before) {
		t.Fatalf("a failed decrypt still cleared the volume: %d entries left of %d", len(got), len(before))
	}
}

// Truncation is the failure mode a streamed backup has to be honest about: a
// cut-off object must not restore as a partial volume that looks complete.
func TestRestoreRejectsATruncatedSnapshot(t *testing.T) {
	recipient, identity := newKeypair(t)
	src := tree(t)
	var archive bytes.Buffer
	if err := Backup(src, &archive, recipient); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	truncated := archive.Bytes()[:archive.Len()/2]

	if err := Restore(t.TempDir(), bytes.NewReader(truncated), identity); err == nil {
		t.Fatal("Restore accepted a truncated snapshot")
	}
}

func TestBackupRefusesAnInvalidRecipient(t *testing.T) {
	if err := Backup(t.TempDir(), io.Discard, "not-an-age-key"); err == nil {
		t.Fatal("Backup accepted an invalid recipient; the snapshot would be unencrypted or unreadable")
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
