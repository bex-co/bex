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
	"compress/flate"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"hash/maphash"
	"io"
	"log"
	"maps"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	files "github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

type Revalidator interface {
	RevalidateFiles(context.Context, files.Claims) error
}

// ExecRevalidator preserves the existing sandbox exec authorization boundary:
// create-like access and the stronger agent-session sensitive relation.
type ExecRevalidator struct{ *core.Base }

func (r *ExecRevalidator) RevalidateFiles(ctx context.Context, c files.Claims) error {
	return (&sshgateway.ExecRevalidator{Base: r.Base}).RevalidateExec(ctx, sandboxexec.Claims{
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
		writeJSONError(w, http.StatusServiceUnavailable, errors.New("sandbox file transfer unavailable"))
		return
	}
	c, err := files.Verify(s.Secret, r.Header.Get(files.TicketHeader), time.Now())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, errors.New("invalid file ticket"))
		return
	}
	if (c.Operation == files.OperationUpload && r.Method != http.MethodPut) ||
		(c.Operation == files.OperationDownload && r.Method != http.MethodGet) {
		writeJSONError(w, http.StatusForbidden, errors.New("file operation does not match ticket"))
		return
	}
	if !c.PlatformUploadAllowed(sandboxexec.SystemSubject) {
		writeJSONError(w, http.StatusForbidden, errors.New("file operation does not match ticket"))
		return
	}
	if !s.Nonces.Consume(r.Context(), c.Nonce, c.NonceExpiry(), time.Now()) {
		writeJSONError(w, http.StatusConflict, errors.New("file ticket already used"))
		return
	}
	limitKey := c.LimitKey(sandboxexec.SystemSubject)
	if acquired, scope := s.Limits.Acquire(limitKey); !acquired {
		s.Metrics.LimitRejected(scope)
		writeJSONError(w, http.StatusTooManyRequests, errors.New("session limit reached"))
		return
	}
	defer s.Limits.Release(limitKey)
	s.Metrics.SessionStarted()
	started := time.Now()
	result := "closed"
	defer func() { s.Metrics.SessionEnded(result, time.Since(started)) }()
	check := func(ctx context.Context) error {
		if s.Revalidator == nil {
			return nil
		}
		return s.Revalidator.RevalidateFiles(ctx, c)
	}
	if err := check(r.Context()); err != nil {
		writeJSONError(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}
	timed, cancelTimeout := context.WithTimeout(r.Context(), s.TransferTimeout)
	defer cancelTimeout()
	ctx, cancel := sshgateway.WithRevalidation(timed, s.RevalidateInterval, check)
	defer cancel()
	// The watchdog ends a transfer by canceling ctx under a live timed context;
	// the client and the cap end timed itself. A transfer the watchdog ended was
	// revoked: answer, log and count it as the exec stream does (w5/139).
	// It also counts a transfer refused for gateway capacity, which both
	// directions pass through here.
	revoked := func(err error) error {
		switch {
		case errors.Is(err, errGatewayBusy):
			s.Metrics.LimitRejected("archive_index_memory")
		case errors.Is(err, errWorkspaceBusy):
			s.Metrics.LimitRejected("archive_index_workspace")
		}
		if err == nil || ctx.Err() == nil || timed.Err() != nil {
			return err
		}
		result = "revoked"
		log.Printf("sandbox file transfer revoked mid-stream (sandbox=%s subject=%s)", c.SandboxID, c.Subject)
		return errRevoked
	}
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
		if err := revoked(s.upload(ctx, target, c.Path, r)); err != nil {
			writeError(w, target.ServiceID, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.download(ctx, target, c.Path, w, revoked)
}

var (
	errTooLarge   = errors.New("sandbox transfer exceeds 256 MiB")
	errBadArchive = errors.New("invalid or unsupported sandbox archive")
	errBadGzip    = errors.New("invalid gzip upload")
	errMissingEnd = errors.New("archive has no complete end marker")
)

type statusError struct {
	status  int
	message string
	// code is Render's machine-readable error code, where one exists: the
	// Render SDK tells a missing remote path (file_not_found) from a gone
	// sandbox only by it.
	code string
}

func (e *statusError) Error() string { return e.message }

// codeFileNotFound is Render's code for a missing remote path; the Render SDK
// raises SandboxFileNotFoundError on it.
const codeFileNotFound = "file_not_found"

var (
	errOperationFailed = &statusError{status: http.StatusServiceUnavailable, message: "sandbox file operation failed"}
	errRevoked         = &statusError{status: http.StatusForbidden, message: "access was revoked during this transfer"}
	// errNotReady is a target Pod that is not Ready (a NotReady or unreachable
	// node): out of reach for now, never a missing sandbox.
	errNotReady = &statusError{status: http.StatusServiceUnavailable, message: "sandbox is not reachable right now: its pod is not ready"}
	// errUploadContentType answers an upload in neither of the two content
	// types Render documents (connect-sandbox-files: "Use
	// application/octet-stream for a single file or application/x-tar for a
	// directory archive. For gzip compression, also set Content-Encoding:
	// gzip."). That includes application/gzip, which Render's TypeScript SDK
	// still lists from before Render moved compression into Content-Encoding
	// (w32/041).
	errUploadContentType = &statusError{status: http.StatusUnsupportedMediaType,
		message: "unsupported upload Content-Type: send a file as application/octet-stream or a directory as an application/x-tar archive, with Content-Encoding: gzip when either is gzipped"}
	// errUploadEncoding answers an upload in an encoding other than gzip, the
	// one Render documents for both types (w32/045).
	errUploadEncoding = &statusError{status: http.StatusUnsupportedMediaType,
		message: "unsupported upload Content-Encoding: send the upload gzipped with Content-Encoding: gzip, or with no Content-Encoding"}
	// errGatewayBusy is a directory transfer past archiveIndexMemory, and
	// errWorkspaceBusy one past its workspace's share of it.
	errGatewayBusy   = &statusError{status: http.StatusServiceUnavailable, message: "the sandbox file gateway is at capacity; retry the transfer"}
	errWorkspaceBusy = &statusError{status: http.StatusServiceUnavailable, message: "this workspace's directory transfers hold its share of the sandbox file gateway; retry when one of them ends"}
)

// hideCause answers errOperationFailed in place of an unclassified failure: a
// Kubernetes exec error can name the API server, the pod and the node, and a
// failed read of the upload the gateway's own addresses (w5/131). The cause is
// logged instead, unless the caller canceled. core.HideCause does the same for
// bex-api, but logs as bex-api.
func hideCause(sandbox string, cause error) error {
	if !errors.Is(cause, context.Canceled) {
		log.Printf("sandbox files: %v (sandbox=%s): %v", errOperationFailed, sandbox, cause)
	}
	return errOperationFailed
}

func writeError(w http.ResponseWriter, sandbox string, err error) {
	var status int
	var se *statusError
	switch {
	case errors.As(err, &se):
		status = se.status
	case errors.Is(err, errTooLarge):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, errBadArchive), errors.Is(err, errBadGzip), errors.Is(err, errMissingEnd), errors.Is(err, io.ErrUnexpectedEOF):
		status = http.StatusBadRequest
	case errors.Is(err, sshgateway.ErrTargetTerminated):
		status = http.StatusNotFound
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	default:
		err, status = hideCause(sandbox, err), errOperationFailed.status
	}
	writeJSONError(w, status, err)
}

// writeJSONError answers bex's REST error body (core.WriteErrStatus), plus
// Render's machine code where one exists. The pinned Render CLI prints its
// message and the Render SDK reads its code; a plain-text body left the SDK
// unable to tell a missing file from a missing sandbox.
func writeJSONError(w http.ResponseWriter, status int, err error) {
	var se *statusError
	if errors.As(err, &se) && se.code != "" {
		core.WriteJSON(w, status, map[string]any{"error": se.message, "message": se.message, "code": se.code, "id": se.code})
		return
	}
	core.WriteErrStatus(w, status, err.Error())
}

// archiveError marks err as the client's bad archive when the archive's own
// format failed it; anything else passes through. A failed read of the upload
// is not the archive's fault, and its text, a network error, must not reach
// the client inside one (w5/131).
func archiveError(err error) error {
	if malformed(err) || errors.Is(err, errMissingEnd) || errors.Is(err, tar.ErrHeader) {
		return fmt.Errorf("%w: %w", errBadArchive, err)
	}
	return err
}

// gzipError does the same for a gzipped single file (w32/045).
func gzipError(err error) error {
	if malformed(err) {
		return fmt.Errorf("%w: %w", errBadGzip, err)
	}
	return err
}

// malformed reports a stream that ends early or whose gzip header, data or
// trailer (its CRC-32 and length) does not check out.
func malformed(err error) bool {
	var corrupt flate.CorruptInputError
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, gzip.ErrHeader) || errors.Is(err, gzip.ErrChecksum) || errors.As(err, &corrupt)
}

func executionError(code int, err error) error {
	if err != nil {
		return err
	}
	switch code {
	case 0:
		return nil
	case 66:
		return &statusError{status: http.StatusNotFound, message: "sandbox source or destination parent does not exist", code: codeFileNotFound}
	case 73:
		return &statusError{status: http.StatusConflict, message: "sandbox destination exists or is not a supported file target"}
	case 74:
		return &statusError{status: http.StatusConflict, message: "sandbox upload path passes through a symlink"}
	default:
		return errOperationFailed
	}
}

func (s *Server) execute(ctx context.Context, target apps.SSHInstanceTarget, command []string, input io.Reader, output io.Writer) error {
	code, err := s.Executor.Execute(ctx, target, command, false, nil, input, output, io.Discard)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, sshgateway.ErrTargetNotReady) {
		return errNotReady
	}
	if err != nil && !errors.Is(err, sshgateway.ErrTargetTerminated) {
		// The executor's own failure answers as one whatever it wraps, never as
		// the client's bad upload (io.ErrUnexpectedEOF) or a timeout.
		return hideCause(target.ServiceID, err)
	}
	return executionError(code, err)
}

// cdParent enters parent directory $1; a relative $1 is under the
// (physically resolved) home directory. An upload refuses a symlinked
// ancestor (exit 74): following it would move the write to a path its ticket
// did not name. A download passes "follow" as $2 and resolves its source as
// the sandbox's own filesystem does (w32/024): it only reads, inside a
// sandbox whose tenant is root and can already cat the same file through
// exec. An argument, not a variable, so no environment can turn it on.
const cdParent = `cd_parent() {
  case $1 in
  /*) dir=$1 ;;
  *)
    [ -n "${HOME:-}" ] || exit 66
    home=$(cd -P "$HOME" && pwd -P) || exit 66
    if [ "$1" = . ]; then dir=$home
    elif [ "$home" = / ]; then dir=/$1
    else dir=$home/$1
    fi ;;
  esac
  cd -P "$dir" || exit 66
  [ "${2:-}" = follow ] || [ "$(pwd -P)" = "$dir" ] || exit 74
}
`

// destinationFree succeeds when entry $name may be published as $kind: a
// directory only where nothing is, a file over nothing or a regular file, and
// never through a symlink.
const destinationFree = `destination_free() {
  if [ "$kind" = directory ]; then
    [ ! -e "$name" ] && [ ! -L "$name" ]
  else
    [ ! -L "$name" ] && { [ ! -e "$name" ] || [ -f "$name" ]; }
  fi
}
`

// checkScript refuses an upload's destination before any of its body is sent:
// it runs without stdin, so a refusal never leaves an upload streaming into an
// exec that will not read it (w8/313).
const checkScript = cdParent + destinationFree + `set -eu
cd_parent "$1"
name=$2 kind=$3
destination_free || exit 73
`

// Both supported images supply POSIX sh, tar, cat, wc, tr, rm, rmdir, mkdir,
// chmod and Linux mv.
// Untrusted paths are positional argv, never interpolated into shell source.
// Extraction always starts in an empty private staging tree; existing target
// trees are never merged, so their symlinks cannot redirect archive writes.
// Every exit reads the rest of stdin first: the upload streams into it, and a
// pods/exec stdin stream the remote stops reading stalls the exec rather than
// ending it (w8/313).
const stageScript = cdParent + destinationFree + `set -eu
trap 'cat >/dev/null || :' EXIT
cd_parent "$1"
name=$2 stage=$3 kind=$4
destination_free || exit 73
umask 077
mkdir "$stage" || exit 73
complete=0
trap 'cat >/dev/null || :; [ "$complete" = 1 ] || rm -rf -- "$stage" || :' EXIT
trap 'exit 1' HUP INT TERM
if [ "$kind" = directory ]; then
  mkdir "$stage/tree"
  tar -xf - -C "$stage/tree"
else
  cat > "$stage/data"
fi
complete=1
`

// publishScript's $5 is the published entry's octal mode, chosen by the
// gateway (the archive root's, or plainFileMode): staging runs under umask 077.
// $6 is how many bytes the gateway sent a file's stage, after decoding: the
// file publishes only if its stage holds exactly those, so a stream the exec
// cut short is never installed. The stage cannot check it: a gzipped file's
// length is known only once its trailer has been read (w32/045).
const publishScript = cdParent + destinationFree + `set -eu
cd_parent "$1"
name=$2 stage=$3 kind=$4 mode=$5 size=$6
[ -d "$stage" ] && [ ! -L "$stage" ] || exit 73
destination_free || exit 73
if [ "$kind" = directory ]; then
  chmod -- "$mode" "$stage/tree"
  mv -T -- "$stage/tree" "$name"
else
  [ "$(wc -c < "$stage/data" | tr -d ' ')" = "$size" ] || exit 65
  chmod -- "$mode" "$stage/data"
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
		return errUploadContentType
	}
	kind := "file"
	switch mediaType {
	case "application/octet-stream":
	case "application/x-tar":
		kind = "directory"
	default:
		return errUploadContentType
	}
	// Render's contract gzips either type the same way (w32/045).
	encoding := r.Header.Get("Content-Encoding")
	gzipped := encoding == "gzip"
	if !gzipped && encoding != "" && encoding != "identity" {
		return errUploadEncoding
	}
	// A plain file's Content-Length proves it arrived whole. A gzipped file's
	// trailer and an archive's end marker prove theirs, so either may be sent
	// chunked.
	expectedSize := r.ContentLength
	if kind == "file" && !gzipped && expectedSize < 0 {
		// The pinned CLI passes an open os.File even for an empty upload.
		// Go encodes that zero ContentLength as chunked. Admit its empty
		// stream, while nonempty plain files still need an exact length.
		var first [1]byte
		if _, err := io.ReadFull(r.Body, first[:]); !errors.Is(err, io.EOF) {
			if err != nil {
				return err
			}
			return &statusError{status: http.StatusLengthRequired, message: "a nonempty file sent without Content-Encoding: gzip requires Content-Length"}
		}
		expectedSize = 0
	}
	if r.ContentLength > files.MaxTransferBytes {
		return errTooLarge
	}
	parent, name := path.Dir(destination), path.Base(destination)
	if err := s.execute(ctx, target, shell(checkScript, parent, name, kind), nil, io.Discard); err != nil {
		return err
	}
	input := io.Reader(&boundedReader{reader: r.Body, remaining: files.MaxTransferBytes})
	if gzipped {
		gz, err := gzip.NewReader(input)
		if err != nil {
			if kind == "file" {
				return gzipError(err)
			}
			return archiveError(err)
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
		// Its outcome is discarded, so it skips execute, whose log would repeat
		// the transfer's own failure.
		_, _ = s.Executor.Execute(cleanupCtx, target, shell(cdParent+`cd_parent "$1" && rm -rf -- "$2"`, parent, stage), false, nil, nil, io.Discard, io.Discard)
	}()
	reader, writer := io.Pipe()
	stopPipe := context.AfterFunc(ctx, func() {
		_ = reader.CloseWithError(ctx.Err())
		_ = writer.CloseWithError(ctx.Err())
	})
	defer stopPipe()
	validated := make(chan error, 1)
	mode := int64(plainFileMode)
	var sent int64 // a file's bytes, decoded
	go func() {
		var err error
		if kind == "directory" {
			mode, err = copyArchive(writer, input, target.OwnerID)
		} else {
			// Reading a gzipped file through EOF checks its trailer's CRC-32
			// and length.
			sent, err = io.Copy(writer, input)
			if gzipped {
				err = gzipError(err)
			} else if err == nil && sent != expectedSize {
				err = io.ErrUnexpectedEOF
			}
		}
		_ = writer.CloseWithError(err)
		validated <- err
	}()
	err = s.execute(ctx, target, shell(stageScript, parent, name, stage, kind), reader, io.Discard)
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
	err = s.execute(ctx, target, shell(publishScript, parent, name, stage, kind, strconv.FormatInt(mode, 8), strconv.FormatInt(sent, 10)), nil, io.Discard)
	published = err == nil
	return err
}

const downloadScript = cdParent + `set -eu
name=$2
cd_parent "$1" follow
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

// download streams source to w. revoked classifies a failure the watchdog
// caused; once the stream has started, a failure can only abort it.
func (s *Server) download(ctx context.Context, target apps.SSHInstanceTarget, source string, w http.ResponseWriter, revoked func(error) error) {
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
		// The cancel above ends a stream still running, so its Canceled is ours.
		if execErr := <-done; execErr != nil && !errors.Is(execErr, context.Canceled) {
			err = execErr
		}
		if err == nil {
			err = errors.New("invalid sandbox transfer response")
		}
		writeError(w, target.ServiceID, revoked(err))
		return
	}
	contentType := "application/octet-stream"
	if kind == "directory\n" {
		contentType = "application/x-tar"
	}
	sent := &startWriter{writer: w, header: http.Header{
		"Content-Type":        {contentType},
		"Content-Disposition": {mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(source)})},
		"Content-Encoding":    {"gzip"},
		// no-transform keeps the edge from decoding the gzip for a client that
		// did not ask for it: a decoded body ends cleanly where the gateway
		// aborts, so such a client would read a cut archive as complete
		// (w32/023).
		"Cache-Control": {"no-store, no-transform"},
	}}
	gz := gzip.NewWriter(&boundedWriter{writer: sent, remaining: files.MaxTransferBytes})
	bounded := &boundedReader{reader: input, remaining: files.MaxTransferBytes}
	if kind == "directory\n" {
		_, err = copyArchive(gz, bounded, target.OwnerID)
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
	if err == nil {
		return
	}
	err = revoked(err) // a revocation is logged and counted either way
	if !sent.started {
		// Nothing has reached the client, so it gets the cause; an abort reaches
		// it only as bex-api's generic 503 (w32/m380).
		writeError(w, target.ServiceID, err)
		return
	}
	// Never close gzip after failure: its trailer is the pinned client's proof
	// that even a zero-byte file completed. Abort HTTP too; returning normally
	// would turn a partial raw stream into a success-shaped EOF.
	panic(http.ErrAbortHandler)
}

// startWriter sets the download's headers at its first write, which commits
// them with the 200. Until then a failure can still answer its own status.
type startWriter struct {
	writer  http.ResponseWriter
	header  http.Header
	started bool
}

func (w *startWriter) Write(p []byte) (int, error) {
	if !w.started {
		w.started = true
		maps.Copy(w.writer.Header(), w.header)
	}
	return w.writer.Write(p)
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

// ValidateArchive applies the extraction rules copyArchive enforces without
// writing anything, so a producer of upload archives can prove its output is
// one the gateway accepts. Every caller's index draws on the share of one
// workspace, "".
func ValidateArchive(src io.Reader) error {
	_, err := copyArchive(io.Discard, &boundedReader{reader: src, remaining: files.MaxTransferBytes}, "")
	return err
}

// plainFileMode is a single uploaded file's mode (an overwritten file takes it
// too), and plainDirMode an archive root's when the archive names none: what
// a plain create gives under the default umask, as the pinned client's own
// download path chmods a file (render-oss/cli pkg/sandbox/files.go:103-105).
// Render documents no server-side mode.
const (
	plainFileMode = 0o644
	plainDirMode  = 0o755
)

// archiveModTime keeps an entry's modification time (w32/018), in whole
// seconds within the range a USTAR header holds, which BusyBox tar reads.
// BusyBox restores it on files; a directory's is its extraction time.
func archiveModTime(t time.Time) time.Time {
	const ustarMaxSeconds = 1<<33 - 1
	return time.Unix(min(max(t.Unix(), 0), ustarMaxSeconds), 0)
}

// entryMode is an entry's permission bits without owners or special bits,
// plus the owner access its extraction needs.
func entryMode(h *tar.Header) int64 {
	if h.Typeflag == tar.TypeDir {
		return h.Mode&0777 | 0700
	}
	return h.Mode&0777 | 0600
}

// archiveIndex is what copyArchive remembers of the entries it has passed:
// each name's type, and the targets of the symlinks a later hard link may
// name. A name is held as a 128-bit digest under seeds chosen per archive, so
// it costs a few tens of bytes however long it is. Every entry costs at least
// one 512-byte header against MaxTransferBytes, so the byte bound also bounds
// the index, and there is no entry-count cap (w32/023): Render documents none
// (render.com/docs/sandboxes-files, sandboxes-limits, read 2026-11-14), and a
// project's node_modules or site-packages routinely holds tens of thousands
// of entries. Targets are bytes the tenant chose, so at most
// maxKeptSymlinkTargets of them are kept. Every index in the process also
// draws on archiveIndexMemory, within its workspace's share.
type archiveIndex struct {
	seeds     [2]maphash.Seed
	kinds     map[nameKey]byte
	symlinks  map[nameKey]string
	keptBytes int
	budget    *indexBudget // archiveIndexMemory when the index began
	workspace string       // whose share the index draws on
	used      int64        // bytes the index holds, as charged
	reserved  int64        // bytes taken from budget
}

type nameKey [2]uint64

// maxKeptSymlinkTargets bounds the symlink targets one archive keeps, each
// charged keptSymlinkCost. A hard link to a symlink past it is refused. Tar
// writes one only for a symlink with a second name, which is rare.
const maxKeptSymlinkTargets = 8 << 20

// keptSymlinkCost is the heap a kept target takes: its copy, which the
// allocator rounds up to a size class (at most a quarter more from 64 bytes
// up, as 3,457 bytes take 4,096; symlinkSlotCost absorbs the few bytes below
// that), and its symlinks slot.
func keptSymlinkCost(target string) int {
	return len(target) + len(target)/4 + symlinkSlotCost
}

// archiveIndexMemory bounds what the indexes of every archive this gateway
// process copies hold at once (w32/025). One archive's index stays under
// about 40 MiB, but up to BEX_SSH_MAX_SESSIONS transfers share the pod's
// 512Mi with the exec drain buffer (128Mi) and every other session, so a few
// worst-case archives from one tenant could end every session on the pod in
// an out-of-memory kill. Past the budget an archive fails with errGatewayBusy
// instead. A 25,000-entry tree takes under 2 MiB, so the budget engages only
// under pressure; Render documents no transfer concurrency limit.
//
// One workspace's archives hold at most workspaceIndexShare of it (w32/030),
// so one tenant's slow transfers always leave the others 16 MiB, about
// 250,000 entries, while a lone worst-case archive still fits. Past its share
// an archive fails with errWorkspaceBusy. A fixed fraction, like the sandbox
// import budget's quarter (w5/m383), would refuse a lone worst-case tree.
var archiveIndexMemory = newIndexBudget(archiveIndexBudget, workspaceIndexShare)

const (
	archiveIndexBudget  = 64 << 20
	workspaceIndexShare = 48 << 20
	// indexEntryCost is one kinds slot with its share of map growth: 524,288
	// digests took 29.9 MiB of heap (measured 2026-11-14).
	indexEntryCost = 64
	// symlinkSlotCost is one symlinks slot, a 32-byte key and string header
	// against a kinds slot's 24, scaled like indexEntryCost.
	symlinkSlotCost = 96
	// indexReserveChunk is how much an index takes from archiveIndexMemory at
	// a time, so the shared semaphore is not touched per entry.
	indexReserveChunk = 64 << 10
)

// indexBudget lends archive indexes memory from one process-wide total, and
// at most share of it to one workspace's indexes together.
type indexBudget struct {
	mu    sync.Mutex
	free  int64
	share int64
	held  map[string]int64 // by workspace
}

func newIndexBudget(total, share int64) *indexBudget {
	return &indexBudget{free: total, share: share, held: map[string]int64{}}
}

// take lends workspace n bytes. A workspace past its share is told so even
// when the process is full too: its own transfers are what to wait for.
func (b *indexBudget) take(workspace string, n int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.held[workspace]+n > b.share {
		return errWorkspaceBusy
	}
	if n > b.free {
		return errGatewayBusy
	}
	b.free -= n
	b.held[workspace] += n
	return nil
}

func (b *indexBudget) give(workspace string, n int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.free += n
	if b.held[workspace] -= n; b.held[workspace] == 0 {
		delete(b.held, workspace)
	}
}

func newArchiveIndex(workspace string) *archiveIndex {
	return &archiveIndex{
		seeds:     [2]maphash.Seed{maphash.MakeSeed(), maphash.MakeSeed()},
		kinds:     map[nameKey]byte{},
		symlinks:  map[nameKey]string{},
		budget:    archiveIndexMemory,
		workspace: workspace,
	}
}

func (x *archiveIndex) key(name string) nameKey {
	return nameKey{maphash.String(x.seeds[0], name), maphash.String(x.seeds[1], name)}
}

// charge accounts n more bytes, reserving from archiveIndexMemory as needed.
func (x *archiveIndex) charge(n int) error {
	x.used += int64(n)
	if x.used <= x.reserved {
		return nil
	}
	need := (x.used - x.reserved + indexReserveChunk - 1) / indexReserveChunk * indexReserveChunk
	if err := x.budget.take(x.workspace, need); err != nil {
		return err
	}
	x.reserved += need
	return nil
}

func (x *archiveIndex) release() {
	x.budget.give(x.workspace, x.reserved)
	x.reserved = 0
}

// keepSymlink copies target: archive/tar hands back a PAX linkpath as a
// substring of its whole PAX block, which can hold a megabyte of padding.
func (x *archiveIndex) keepSymlink(key nameKey, target string) error {
	cost := keptSymlinkCost(target)
	if x.keptBytes+cost > maxKeptSymlinkTargets {
		return nil
	}
	if err := x.charge(cost); err != nil {
		return err
	}
	x.keptBytes += cost
	x.symlinks[key] = strings.Clone(target)
	return nil
}

// entryName is an archive member's name relative to its root, as BusyBox's
// `tar -cf - -C dir .` writes it (`./sub/f`) and as a hard link names its
// target (`./f`).
func entryName(name string) string {
	return strings.TrimSuffix(strings.TrimPrefix(name, "./"), "/")
}

// copyArchive validates before forwarding each member, keeps only its
// permission bits and modification time, and supplies end markers only after
// the complete source (including its gzip checksum, when present) has been
// verified. It returns the mode the archive gives its root (its `.` entry,
// plainDirMode when it has none), for the published directory.
//
// The same rule holds in both directions (w32/m380), as on Render
// (render.com/docs/sandboxes-files, read 2026-11-13: "Symlinks are preserved,
// and sockets, FIFOs, and other special files are skipped"). A symlink keeps
// its target byte for byte, relative, absolute or dangling: the tenant is root
// in its own sandbox. It can never be a parent, so nothing is written through
// it. A hard link is another name for an entry earlier in the same archive, so
// it stays inside the tree: tar archives the first name it reads for an inode
// and each later one as a link to it. It counts as what it names: a file stays
// a hard link, a symlink is copied, and a special entry is skipped too.
//
// workspace is whose share of archiveIndexMemory the archive's index draws on.
func copyArchive(dst io.Writer, src io.Reader, workspace string) (int64, error) {
	tr := tar.NewReader(terminatedReader{src})
	tw := tar.NewWriter(&boundedWriter{writer: dst, remaining: files.MaxTransferBytes})
	index := newArchiveIndex(workspace)
	defer index.release()
	// tar's Reader and Writer hide their WriterTo and ReaderFrom, so io.Copy
	// would allocate 32 KiB for every entry.
	buf := make([]byte, 32<<10)
	root := int64(plainDirMode)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, archiveError(err)
		}
		if (h.Name == "." || h.Name == "./") && h.Typeflag == tar.TypeDir {
			root = entryMode(h)
			continue
		}
		name := entryName(h.Name)
		if h.Size > files.MaxTransferBytes {
			return 0, errTooLarge
		}
		if name == "" || name == "." || path.IsAbs(name) || path.Clean(name) != name ||
			name == ".." || strings.HasPrefix(name, "../") || strings.ContainsRune(name, '\\') ||
			strings.IndexFunc(name, unicode.IsControl) >= 0 || len(name) > files.MaxPathBytes ||
			strings.Count(name, "/") >= files.MaxPathDepth || h.Size < 0 {
			return 0, fmt.Errorf("%w: unsafe entry", errBadArchive)
		}
		key := index.key(name)
		if _, exists := index.kinds[key]; exists {
			return 0, fmt.Errorf("%w: duplicate entry", errBadArchive)
		}
		if parent := path.Dir(name); parent != "." && index.kinds[index.key(parent)] != tar.TypeDir {
			return 0, fmt.Errorf("%w: parent directory must precede its children", errBadArchive)
		}
		clean := &tar.Header{Name: name, Typeflag: h.Typeflag, Mode: entryMode(h), ModTime: archiveModTime(h.ModTime)}
		kind := h.Typeflag
		switch h.Typeflag {
		case tar.TypeReg:
			clean.Size = h.Size
		case tar.TypeSymlink:
			if h.Linkname == "" || len(h.Linkname) > files.MaxPathBytes || strings.IndexFunc(h.Linkname, unicode.IsControl) >= 0 {
				return 0, fmt.Errorf("%w: unsafe symlink target", errBadArchive)
			}
			clean.Linkname = h.Linkname
			if err := index.keepSymlink(key, h.Linkname); err != nil {
				return 0, err
			}
		case tar.TypeLink:
			target := index.key(entryName(h.Linkname))
			var ok bool
			if kind, ok = index.kinds[target]; !ok || kind == tar.TypeDir {
				return 0, fmt.Errorf("%w: a hard link must name a file earlier in the archive", errBadArchive)
			}
			switch kind {
			case tar.TypeReg:
				clean.Linkname = entryName(h.Linkname)
			case tar.TypeSymlink: // BusyBox archives a second name for a symlink this way
				if clean.Linkname, ok = index.symlinks[target]; !ok {
					return 0, fmt.Errorf("%w: too many symlink targets to copy a hard link to one", errBadArchive)
				}
				clean.Typeflag = tar.TypeSymlink
				if err := index.keepSymlink(key, clean.Linkname); err != nil {
					return 0, err
				}
			}
		}
		if err := index.charge(indexEntryCost); err != nil {
			return 0, err
		}
		// Recorded even when skipped, so a child under it is refused.
		index.kinds[key] = kind
		if kind != tar.TypeReg && kind != tar.TypeDir && kind != tar.TypeSymlink {
			continue
		}
		if err := tw.WriteHeader(clean); err != nil {
			return 0, err
		}
		if _, err := io.CopyBuffer(tw, tr, buf); err != nil {
			return 0, archiveError(err)
		}
	}
	// Reading through EOF also verifies a gzip footer. Reject non-padding data
	// after tar's terminator instead of silently ignoring another archive.
	for {
		n, err := src.Read(buf)
		for _, b := range buf[:n] {
			if b != 0 {
				return 0, fmt.Errorf("%w: trailing data", errBadArchive)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, archiveError(err)
		}
	}
	return root, tw.Close()
}
