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

package sandboxfiles

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	files "github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
)

// member is one archive entry as a test reads it back.
type member struct {
	name     string
	typeflag byte
	linkname string
	body     string
}

func (m member) String() string {
	return fmt.Sprintf("%s %c %q %q", m.name, m.typeflag, m.linkname, m.body)
}

func members(t *testing.T, archive []byte) []member {
	t.Helper()
	var out []member
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, member{h.Name, h.Typeflag, h.Linkname, string(body)})
	}
}

func equalMembers(t *testing.T, got []member, want ...member) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("archive members:\n got %v\nwant %v", got, want)
	}
}

// w32/m380: a symlink is kept with its target byte for byte, a hard link to a
// file earlier in the archive is kept with its target cleaned like an entry
// name, a hard link to a symlink is a copy of it, and special entries (and a
// hard link to one) are skipped, as Render documents: "Symlinks are preserved,
// and sockets, FIFOs, and other special files are skipped".
func TestArchiveKeepsLinksAndSkipsSpecialEntries(t *testing.T) {
	var out bytes.Buffer
	_, err := copyArchive(&out, bytes.NewReader(archive(t,
		&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "./top.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 2},
		&tar.Header{Name: "./rel-link", Typeflag: tar.TypeSymlink, Linkname: "top.txt"},
		&tar.Header{Name: "./abs-link", Typeflag: tar.TypeSymlink, Linkname: "/etc/hostname"},
		&tar.Header{Name: "./dangling", Typeflag: tar.TypeSymlink, Linkname: "missing target"},
		&tar.Header{Name: "./out-link", Typeflag: tar.TypeSymlink, Linkname: "../../outside"},
		&tar.Header{Name: "./sub/", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "./dirlink", Typeflag: tar.TypeSymlink, Linkname: "sub"},
		&tar.Header{Name: "./hard.txt", Typeflag: tar.TypeLink, Linkname: "./top.txt"},
		&tar.Header{Name: "./sub/hard", Typeflag: tar.TypeLink, Linkname: "hard.txt"},
		&tar.Header{Name: "./rel-link-2", Typeflag: tar.TypeLink, Linkname: "./rel-link"},
		&tar.Header{Name: "./pipe", Typeflag: tar.TypeFifo},
		&tar.Header{Name: "./pipe-2", Typeflag: tar.TypeLink, Linkname: "./pipe"},
		&tar.Header{Name: "./tty", Typeflag: tar.TypeChar, Devmajor: 5},
		&tar.Header{Name: "./disk", Typeflag: tar.TypeBlock, Devmajor: 8},
		&tar.Header{Name: "./last.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1},
	)))
	if err != nil {
		t.Fatal(err)
	}
	equalMembers(t, members(t, out.Bytes()),
		member{"top.txt", tar.TypeReg, "", "\x00\x00"},
		member{"rel-link", tar.TypeSymlink, "top.txt", ""},
		member{"abs-link", tar.TypeSymlink, "/etc/hostname", ""},
		member{"dangling", tar.TypeSymlink, "missing target", ""},
		member{"out-link", tar.TypeSymlink, "../../outside", ""},
		member{"sub", tar.TypeDir, "", ""},
		member{"dirlink", tar.TypeSymlink, "sub", ""},
		member{"hard.txt", tar.TypeLink, "top.txt", ""},
		member{"sub/hard", tar.TypeLink, "hard.txt", ""},
		member{"rel-link-2", tar.TypeSymlink, "top.txt", ""},
		member{"last.txt", tar.TypeReg, "", "\x00"},
	)
}

// Nothing can be written through a link, and a hard link never reaches outside
// the archive's own regular files.
func TestArchiveStillRefusesWritesThroughLinksAndEscapingHardLinks(t *testing.T) {
	sub := &tar.Header{Name: "sub/", Typeflag: tar.TypeDir}
	file := &tar.Header{Name: "file", Typeflag: tar.TypeReg}
	for name, headers := range map[string][]*tar.Header{
		"a child under a symlink":            {{Name: "l", Typeflag: tar.TypeSymlink, Linkname: "/etc"}, {Name: "l/passwd", Typeflag: tar.TypeReg}},
		"a child under a skipped fifo":       {{Name: "p", Typeflag: tar.TypeFifo}, {Name: "p/x", Typeflag: tar.TypeReg}},
		"a name reused by a symlink":         {file, {Name: "file", Typeflag: tar.TypeSymlink, Linkname: "x"}},
		"a hard link before its target":      {{Name: "h", Typeflag: tar.TypeLink, Linkname: "file"}, file},
		"a hard link out of the archive":     {file, {Name: "h", Typeflag: tar.TypeLink, Linkname: "../file"}},
		"a hard link to an absolute path":    {file, {Name: "h", Typeflag: tar.TypeLink, Linkname: "/etc/shadow"}},
		"a hard link to a directory":         {sub, {Name: "h", Typeflag: tar.TypeLink, Linkname: "sub"}},
		"a hard link to itself":              {{Name: "h", Typeflag: tar.TypeLink, Linkname: "h"}},
		"a hard link with no target":         {file, {Name: "h", Typeflag: tar.TypeLink}},
		"a symlink with no target":           {{Name: "s", Typeflag: tar.TypeSymlink}},
		"a symlink target with a control":    {{Name: "s", Typeflag: tar.TypeSymlink, Linkname: "a\nb"}},
		"a symlink target over PATH_MAX":     {{Name: "s", Typeflag: tar.TypeSymlink, Linkname: strings.Repeat("a", files.MaxPathBytes+1)}},
		"a traversal name on a skipped fifo": {{Name: "../p", Typeflag: tar.TypeFifo}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := copyArchive(io.Discard, bytes.NewReader(archive(t, headers...))); !errors.Is(err, errBadArchive) {
				t.Fatalf("copyArchive = %v, want a bad archive", err)
			}
		})
	}
}

// busyBoxLinks is BusyBox v1.37.0's `tar -cf - -C dl .` of the tree a sandbox
// download carries (node:22-alpine, the base image's BusyBox, 2026-11-14):
//
//	top.txt ("top\n"), hard.txt (a second link to it), sub/inner.txt ("inner\n")
//	and sub/hard-inner.txt (its second link), rel-link -> top.txt and
//	rel-link-2 (its second link), abs-link -> /etc/hostname, dirlink -> sub,
//	dangling -> missing-target, and a FIFO with two names, pipe and pipe-hard.
//
// BusyBox archives the first name it reads for an inode with its data and each
// later one as a hard link to it, so a link always follows its target, even
// for a symlink or a FIFO.
const busyBoxLinks = "testdata/busybox-links.tar"

// downloadLinksGolden is the gateway's archive of busyBoxLinks, before gzip.
// lego/cli's TestPinnedSandboxDownloadRestoresLinks has the pinned client
// extract it. Regenerate with BEX_UPDATE_GOLDEN=1 when the archive rule moves.
const downloadLinksGolden = "../../../../cli/testdata/sandbox-download-links.tar"

func TestABusyBoxTreeOfLinksDownloadsWithItsLinks(t *testing.T) {
	sandboxTar, err := os.ReadFile(busyBoxLinks)
	if err != nil {
		t.Fatal(err)
	}
	s := serverFor(func(_ context.Context, _ []string, _ io.Reader, out io.Writer) (int, error) {
		_, _ = io.WriteString(out, "directory\n")
		_, _ = out.Write(sandboxTar)
		return 0, nil
	})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, requestFor(t, http.MethodGet, nil))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/x-tar" {
		t.Fatalf("download = %d %q: %s", w.Code, w.Header().Get("Content-Type"), w.Body)
	}
	gz, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	equalMembers(t, members(t, got),
		member{"abs-link", tar.TypeSymlink, "/etc/hostname", ""},
		member{"dangling", tar.TypeSymlink, "missing-target", ""},
		member{"hard.txt", tar.TypeReg, "", "top\n"},
		member{"dirlink", tar.TypeSymlink, "sub", ""},
		member{"rel-link", tar.TypeSymlink, "top.txt", ""},
		member{"sub", tar.TypeDir, "", ""},
		member{"sub/inner.txt", tar.TypeReg, "", "inner\n"},
		member{"sub/hard-inner.txt", tar.TypeLink, "sub/inner.txt", ""},
		member{"top.txt", tar.TypeLink, "hard.txt", ""},
		member{"rel-link-2", tar.TypeSymlink, "top.txt", ""},
	)
	if os.Getenv("BEX_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(downloadLinksGolden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if golden, err := os.ReadFile(downloadLinksGolden); err != nil || !bytes.Equal(golden, got) {
		t.Fatalf("%s is not the gateway's archive of %s (err %v); regenerate it with BEX_UPDATE_GOLDEN=1", downloadLinksGolden, busyBoxLinks, err)
	}
}

// w32/m380 t003: a download that fails before its first byte answers its
// cause. Aborting it reached the client as bex-api's "503 sandbox file gateway
// unavailable", or as "409 file ticket already used" once the GET was
// replayed. Once a byte is out the stream still aborts (see
// TestDownloadGzipFinalizesOnlyAfterExecutorSuccess).
func TestADownloadRefusedBeforeItsFirstByteAnswersItsCause(t *testing.T) {
	for name, tc := range map[string]struct {
		output  string
		exit    int
		status  int
		message string
	}{
		"an unsafe first entry": {
			output: "directory\n" + string(archive(t, &tar.Header{Name: "./", Typeflag: tar.TypeDir}, &tar.Header{Name: "../escape", Typeflag: tar.TypeReg})),
			status: http.StatusBadRequest, message: "invalid or unsupported sandbox archive: unsafe entry",
		},
		"a file the sandbox cannot read": {output: "file\n", exit: 1, status: http.StatusServiceUnavailable, message: "sandbox file operation failed"},
	} {
		t.Run(name, func(t *testing.T) {
			s := serverFor(func(_ context.Context, _ []string, _ io.Reader, out io.Writer) (int, error) {
				_, _ = io.WriteString(out, tc.output)
				return tc.exit, nil
			})
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, requestFor(t, http.MethodGet, nil))
			var body struct{ Message string }
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != tc.status || body.Message != tc.message {
				t.Fatalf("download = %d %s (%v), want %d %q", w.Code, w.Body, err, tc.status, tc.message)
			}
			for _, header := range []string{"Content-Encoding", "Content-Disposition", "Cache-Control"} {
				if value := w.Header().Get(header); value != "" {
					t.Errorf("the refusal kept the download's %s: %q", header, value)
				}
			}
		})
	}
}

// An upload's links arrive as links: the archive the pinned client's writeTar
// sends (symlinks stored, not followed) plus a hard link, as Python's tarfile
// writes for a second name of one inode, extracted by the real stage script
// and published by the real publish script (w32/m380 t001).
func TestUploadedLinksArriveAsLinks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("publishScript needs GNU coreutils (mv -T); runs on Linux CI")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var upload bytes.Buffer
	tw := tar.NewWriter(&upload)
	for _, h := range []*tar.Header{
		{Name: "./", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "with space.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 4},
		{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "with space.txt"},
		{Name: "abs-link", Typeflag: tar.TypeSymlink, Linkname: "/etc/hostname"},
		{Name: "dangling", Typeflag: tar.TypeSymlink, Linkname: "nowhere"},
		{Name: "lib/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "lib64", Typeflag: tar.TypeSymlink, Linkname: "lib"},
		{Name: "hard.txt", Typeflag: tar.TypeLink, Linkname: "with space.txt"},
		{Name: "pipe", Typeflag: tar.TypeFifo, Mode: 0o644},
	} {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			_, _ = io.WriteString(tw, "data")
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var clean bytes.Buffer
	mode, err := copyArchive(&clean, &upload)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{
		shell(stageScript, root, "up", ".bex-copy-links", "directory", "0"),
		shell(publishScript, root, "up", ".bex-copy-links", "directory", strconv.FormatInt(mode, 8)),
	} {
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Env = []string{"HOME=" + root, "PATH=" + os.Getenv("PATH")}
		if command[2] == stageScript {
			cmd.Stdin = &clean
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%q: %v %s", command[4:], err, out)
		}
	}
	up := filepath.Join(root, "up")
	for name, target := range map[string]string{"link": "with space.txt", "abs-link": "/etc/hostname", "dangling": "nowhere", "lib64": "lib"} {
		if got, err := os.Readlink(filepath.Join(up, name)); err != nil || got != target {
			t.Errorf("%s -> %q (%v), want a symlink to %q", name, got, err, target)
		}
	}
	original, err := os.Lstat(filepath.Join(up, "with space.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if hard, err := os.Lstat(filepath.Join(up, "hard.txt")); err != nil || !os.SameFile(original, hard) {
		t.Errorf("hard.txt = %v (%v), want a second link to with space.txt", hard, err)
	}
	if _, err := os.Lstat(filepath.Join(up, "pipe")); !os.IsNotExist(err) {
		t.Errorf("the FIFO was published: %v", err)
	}
}
