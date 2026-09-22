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

// Package sandboxfiles streams path-bound files through the existing privileged
// Executor. Neither API nor gateway buffers an artifact or extracts it locally.
package sandboxfiles

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	files "github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxsse"
)

type Revalidator interface {
	RevalidateFiles(context.Context, files.Claims) error
}

// ExecRevalidator preserves the existing sandbox exec authorization boundary:
// create-like access and the stronger agent-session sensitive relation.
type ExecRevalidator struct{ *core.Base }

func (r *ExecRevalidator) RevalidateFiles(ctx context.Context, c files.Claims) error {
	return (&sandboxsse.ExecRevalidator{Base: r.Base}).RevalidateExec(ctx, sandboxexec.Claims{
		Subject: c.Subject, Workspace: c.Workspace, AgentSessionID: c.AgentSessionID,
	})
}

type Server struct {
	Secret             []byte
	Executor           sshgateway.Executor
	Metrics            *sshgateway.Metrics
	Limits             *sshgateway.SessionLimiter
	Nonces             *sshgateway.NonceGuard
	Revalidator        Revalidator
	TransferTimeout    time.Duration
	RevalidateInterval time.Duration
}

func (s *Server) Enabled() bool { return len(s.Secret) > 0 }

func (s *Server) Handler() http.Handler {
	if s.TransferTimeout <= 0 {
		s.TransferTimeout = files.DefaultTransferTimeout
	}
	if s.RevalidateInterval == 0 {
		s.RevalidateInterval = sshgateway.DefaultRevalidateInterval
	}
	if s.Limits == nil {
		s.Limits = sshgateway.NewSessionLimiter(0, 0)
	}
	return http.HandlerFunc(s.serve)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if !s.Enabled() || s.Executor == nil || s.Nonces == nil || s.Nonces.Store == nil {
		http.Error(w, "sandbox file transfer unavailable", http.StatusServiceUnavailable)
		return
	}
	c, err := files.Verify(s.Secret, r.Header.Get(files.TicketHeader), time.Now())
	if err != nil {
		http.Error(w, "invalid file ticket", http.StatusUnauthorized)
		return
	}
	if (c.Operation == files.OperationUpload && r.Method != http.MethodPut) ||
		(c.Operation == files.OperationDownload && r.Method != http.MethodGet) {
		http.Error(w, "file operation does not match ticket", http.StatusForbidden)
		return
	}
	if !s.Nonces.Consume(r.Context(), c.Nonce, c.NonceExpiry(), time.Now()) {
		http.Error(w, "file ticket already used", http.StatusConflict)
		return
	}
	if acquired, scope := s.Limits.Acquire(c.Subject); !acquired {
		s.Metrics.LimitRejected(scope)
		http.Error(w, "session limit reached", http.StatusTooManyRequests)
		return
	}
	defer s.Limits.Release(c.Subject)
	s.Metrics.SessionStarted()
	started := time.Now()
	defer func() { s.Metrics.SessionEnded("closed", time.Since(started)) }()
	check := func(ctx context.Context) error {
		if s.Revalidator == nil {
			return nil
		}
		return s.Revalidator.RevalidateFiles(ctx, c)
	}
	if err := check(r.Context()); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	timed, cancelTimeout := context.WithTimeout(r.Context(), s.TransferTimeout)
	defer cancelTimeout()
	ctx, cancel := sshgateway.WithRevalidation(timed, s.RevalidateInterval, check)
	defer cancel()
	deadline := time.Now().Add(s.TransferTimeout)
	_ = http.NewResponseController(w).SetReadDeadline(deadline)
	_ = http.NewResponseController(w).SetWriteDeadline(deadline)
	defer func() {
		_ = http.NewResponseController(w).SetReadDeadline(time.Time{})
		_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	}()
	stopClose := context.AfterFunc(ctx, func() { _ = r.Body.Close() })
	defer stopClose()
	target := apps.SSHInstanceTarget{PodName: c.PodName(), Namespace: c.Namespace,
		Container: sandboxexec.SandboxContainer, ServiceID: c.SandboxID, OwnerID: c.Workspace}
	if c.Operation == files.OperationUpload {
		err = s.upload(ctx, target, c.Path, r)
		if err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.download(ctx, target, c.Path, w)
}

var (
	errTooLarge   = errors.New("sandbox transfer exceeds 256 MiB")
	errBadArchive = errors.New("invalid or unsupported sandbox archive")
	errMissingEnd = errors.New("archive has no complete end marker")
)

type statusError struct {
	status  int
	message string
}

func (e *statusError) Error() string { return e.message }

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusServiceUnavailable
	var se *statusError
	switch {
	case errors.As(err, &se):
		status = se.status
	case errors.Is(err, errTooLarge):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, errBadArchive), errors.Is(err, errMissingEnd), errors.Is(err, io.ErrUnexpectedEOF):
		status = http.StatusBadRequest
	case errors.Is(err, sshgateway.ErrTargetTerminated):
		status = http.StatusNotFound
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	}
	http.Error(w, err.Error(), status)
}

func executionError(code int, err error) error {
	if err != nil {
		return err
	}
	switch code {
	case 0:
		return nil
	case 66:
		return &statusError{http.StatusNotFound, "sandbox source or destination parent does not exist"}
	case 73:
		return &statusError{http.StatusConflict, "sandbox destination exists or is not a supported file target"}
	default:
		return &statusError{http.StatusServiceUnavailable, "sandbox file operation failed"}
	}
}

func (s *Server) execute(ctx context.Context, target apps.SSHInstanceTarget, command []string, input io.Reader, output io.Writer) error {
	code, err := s.Executor.Execute(ctx, target, command, false, nil, input, output, io.Discard)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return executionError(code, err)
}

// Both supported images supply POSIX sh, tar, cat, wc, rm, mkdir and Linux mv.
// Untrusted paths are positional argv, never interpolated into shell source.
// Extraction always starts in an empty private staging tree; existing target
// trees are never merged, so their symlinks cannot redirect archive writes.
const stageScript = `set -eu
cd -P "$1" || exit 66
[ "$(pwd -P)" = "$1" ] || exit 73
name=$2 stage=$3 kind=$4 size=$5
if [ "$kind" = directory ]; then
  [ ! -e "$name" ] && [ ! -L "$name" ] || exit 73
else
  [ ! -L "$name" ] && { [ ! -e "$name" ] || [ -f "$name" ]; } || exit 73
fi
umask 077
mkdir "$stage" || exit 73
complete=0
trap 'if [ "$complete" != 1 ]; then rm -rf -- "$stage"; fi' EXIT
trap 'exit 1' HUP INT TERM
if [ "$kind" = directory ]; then
  mkdir "$stage/tree"
  tar -xf - -C "$stage/tree"
else
  cat > "$stage/data"
  [ "$(wc -c < "$stage/data" | tr -d ' ')" = "$size" ] || exit 65
fi
complete=1
`

const publishScript = `set -eu
cd -P "$1" || exit 66
[ "$(pwd -P)" = "$1" ] || exit 73
name=$2 stage=$3 kind=$4
[ -d "$stage" ] && [ ! -L "$stage" ] || exit 73
if [ "$kind" = directory ]; then
  [ ! -e "$name" ] && [ ! -L "$name" ] || exit 73
  mv -T -- "$stage/tree" "$name"
else
  [ ! -L "$name" ] && { [ ! -e "$name" ] || [ -f "$name" ]; } || exit 73
  mv -T -- "$stage/data" "$name"
fi
rmdir "$stage"
`

func shell(script string, args ...string) []string {
	return append([]string{"/bin/sh", "-c", script, "bex-sandbox-files"}, args...)
}

func (s *Server) upload(ctx context.Context, target apps.SSHInstanceTarget, destination string, r *http.Request) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return &statusError{http.StatusUnsupportedMediaType, "expected application/octet-stream or application/x-tar"}
	}
	kind := "file"
	encoding := r.Header.Get("Content-Encoding")
	expectedSize := r.ContentLength
	switch mediaType {
	case "application/octet-stream":
		if encoding != "" && encoding != "identity" {
			return &statusError{http.StatusUnsupportedMediaType, "raw uploads do not accept Content-Encoding"}
		}
		if expectedSize < 0 {
			// The pinned CLI passes an open os.File even for an empty upload.
			// Go encodes that zero ContentLength as chunked. Admit its empty
			// stream, while nonempty raw bodies still need an exact length.
			var first [1]byte
			if _, err := io.ReadFull(r.Body, first[:]); !errors.Is(err, io.EOF) {
				if err != nil {
					return err
				}
				return &statusError{http.StatusLengthRequired, "nonempty raw uploads require Content-Length"}
			}
			expectedSize = 0
		}
	case "application/x-tar":
		kind = "directory"
		if encoding != "" && encoding != "identity" && encoding != "gzip" {
			return &statusError{http.StatusUnsupportedMediaType, "unsupported archive Content-Encoding"}
		}
	default:
		return &statusError{http.StatusUnsupportedMediaType, "unsupported upload Content-Type"}
	}
	if r.ContentLength > files.MaxTransferBytes {
		return errTooLarge
	}
	input := io.Reader(&boundedReader{reader: r.Body, remaining: files.MaxTransferBytes})
	if encoding == "gzip" {
		gz, err := gzip.NewReader(input)
		if err != nil {
			return fmt.Errorf("%w: %v", errBadArchive, err)
		}
		defer gz.Close()
		input = gz
	}
	input = &boundedReader{reader: input, remaining: files.MaxTransferBytes}
	nonce, err := hmacticket.Nonce()
	if err != nil {
		return err
	}
	stage := ".bex-copy-" + nonce
	parent, name := path.Dir(destination), path.Base(destination)
	published := false
	// Cleanup is the only work permitted past request cancellation, is joined,
	// and has its own short cap. A vanished pod may retain the hidden stage;
	// an incomplete upload is never installed at the requested path.
	defer func() {
		if published {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.execute(cleanupCtx, target, shell(`cd -P "$1" && rm -rf -- "$2"`, parent, stage), nil, io.Discard)
	}()
	reader, writer := io.Pipe()
	stopPipe := context.AfterFunc(ctx, func() {
		_ = reader.CloseWithError(ctx.Err())
		_ = writer.CloseWithError(ctx.Err())
	})
	defer stopPipe()
	validated := make(chan error, 1)
	go func() {
		var err error
		if kind == "directory" {
			err = copyArchive(writer, input)
		} else {
			var n int64
			n, err = io.Copy(writer, input)
			if err == nil && n != expectedSize {
				err = io.ErrUnexpectedEOF
			}
		}
		_ = writer.CloseWithError(err)
		validated <- err
	}()
	err = s.execute(ctx, target, shell(stageScript, parent, name, stage, kind, strconv.FormatInt(expectedSize, 10)), reader, io.Discard)
	_ = reader.CloseWithError(err)
	// An executor can refuse before reading any input. Close the request body
	// too so a stalled uploader cannot retain the validation goroutine.
	_ = r.Body.Close()
	validationErr := <-validated
	if err != nil && errors.Is(validationErr, io.ErrClosedPipe) {
		return err
	}
	if validationErr != nil {
		return validationErr
	}
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	err = s.execute(ctx, target, shell(publishScript, parent, name, stage, kind), nil, io.Discard)
	published = err == nil
	return err
}

const downloadScript = `set -eu
parent=$1 name=$2
cd -P "$parent" || exit 66
[ "$(pwd -P)" = "$parent" ] || exit 73
[ ! -L "$name" ] || exit 73
if [ -d "$name" ]; then
  printf 'directory\n'
  tar -cf - -C "$name" .
elif [ -f "$name" ]; then
  printf 'file\n'
  cat -- "$name"
else
  exit 66
fi
`

func (s *Server) download(ctx context.Context, target apps.SSHInstanceTarget, source string, w http.ResponseWriter) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := s.execute(ctx, target, shell(downloadScript, path.Dir(source), path.Base(source)), nil, writer)
		done <- err
		_ = writer.CloseWithError(err)
	}()
	stopPipe := context.AfterFunc(ctx, func() {
		_ = reader.CloseWithError(ctx.Err())
		_ = writer.CloseWithError(ctx.Err())
	})
	defer stopPipe()
	input := bufio.NewReaderSize(reader, 32)
	marker, err := input.ReadSlice('\n')
	kind := string(marker)
	if err != nil || (kind != "file\n" && kind != "directory\n") {
		cancel()
		_ = reader.Close()
		execErr := <-done
		if execErr != nil {
			err = execErr
		}
		if err == nil {
			err = errors.New("invalid sandbox transfer response")
		}
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	if kind == "directory\n" {
		w.Header().Set("Content-Type", "application/x-tar")
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(source)}))
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Cache-Control", "no-store")
	gz := gzip.NewWriter(&boundedWriter{writer: w, remaining: files.MaxTransferBytes})
	bounded := &boundedReader{reader: input, remaining: files.MaxTransferBytes}
	if kind == "directory\n" {
		err = copyArchive(gz, bounded)
	} else {
		_, err = io.Copy(gz, bounded)
	}
	if err != nil {
		cancel()
	}
	_ = reader.CloseWithError(err)
	execErr := <-done
	if err == nil {
		err = execErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = gz.Close()
	}
	if err != nil {
		// Never close gzip after failure: its trailer is the pinned client's
		// proof that even a zero-byte file completed. Abort HTTP too; returning
		// normally would turn a partial raw stream into a success-shaped EOF.
		panic(http.ErrAbortHandler)
	}
}

type boundedReader struct {
	reader    io.Reader
	remaining int64
}

type boundedWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		n, err := w.writer.Write(p[:w.remaining])
		w.remaining -= int64(n)
		if err != nil {
			return n, err
		}
		return n, errTooLarge
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

func (r *boundedReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n != 0 {
			return 0, errTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}

type terminatedReader struct{ io.Reader }

func (r terminatedReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if errors.Is(err, io.EOF) {
		err = errMissingEnd
	}
	return n, err
}

// copyArchive validates before forwarding each member, strips owners and
// special permission bits, and supplies end markers only after the complete
// source (including its gzip checksum, when present) has been verified.
func copyArchive(dst io.Writer, src io.Reader) error {
	tr := tar.NewReader(terminatedReader{src})
	tw := tar.NewWriter(&boundedWriter{writer: dst, remaining: files.MaxTransferBytes})
	seen := map[string]byte{}
	entries := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: %w", errBadArchive, err)
		}
		entries++
		if entries > files.MaxArchiveEntries {
			return fmt.Errorf("%w: too many entries", errBadArchive)
		}
		if (h.Name == "." || h.Name == "./") && h.Typeflag == tar.TypeDir {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(h.Name, "./"), "/")
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			return fmt.Errorf("%w: links and special entries are unsupported", errBadArchive)
		}
		if h.Size > files.MaxTransferBytes {
			return errTooLarge
		}
		if name == "" || name == "." || path.IsAbs(name) || path.Clean(name) != name ||
			name == ".." || strings.HasPrefix(name, "../") || strings.ContainsRune(name, '\\') ||
			strings.IndexFunc(name, unicode.IsControl) >= 0 || len(name) > files.MaxPathBytes ||
			strings.Count(name, "/") >= files.MaxPathDepth || h.Size < 0 {
			return fmt.Errorf("%w: unsafe entry", errBadArchive)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("%w: duplicate entry", errBadArchive)
		}
		if parent := path.Dir(name); parent != "." && seen[parent] != tar.TypeDir {
			return fmt.Errorf("%w: parent directory must precede its children", errBadArchive)
		}
		seen[name] = h.Typeflag
		clean := &tar.Header{Name: name, Typeflag: h.Typeflag, Size: h.Size, Mode: h.Mode & 0777}
		if h.Typeflag == tar.TypeDir {
			clean.Mode |= 0700
			clean.Size = 0
		} else {
			clean.Mode |= 0600
		}
		if err := tw.WriteHeader(clean); err != nil {
			return err
		}
		if _, err := io.Copy(tw, tr); err != nil {
			return err
		}
	}
	// Reading through EOF also verifies a gzip footer. Reject non-padding data
	// after tar's terminator instead of silently ignoring another archive.
	buf := make([]byte, 32<<10)
	for {
		n, err := src.Read(buf)
		for _, b := range buf[:n] {
			if b != 0 {
				return fmt.Errorf("%w: trailing data", errBadArchive)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	return tw.Close()
}
