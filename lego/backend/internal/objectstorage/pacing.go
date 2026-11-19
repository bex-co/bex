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

package objectstorage

import (
	"context"
	"io"
	"time"

	"golang.org/x/time/rate"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Pacing (ADR112 D3): a workspace's S3 downloads leave through a rate-limited writer and
// its uploads are read through a rate-limited reader, each at a fixed bytes-per-second
// kept per workspace on each replica. Pacing, not debt: D4's debt-and-429 scheme was
// built for anonymous readers and would fail the ten parallel 8 MiB ranges of a 1 GiB
// `aws s3 cp`, and 429 is not a throttle S3 clients retry.

// paceChunk is the most one paced read or write moves at once, which bounds how long a
// single wait is and keeps the limiter's burst a whole number of chunks.
const paceChunk = 256 << 10

// pacer keeps one token bucket per workspace.
type pacer struct {
	byWorkspace *core.KeyedRateLimiter[string]
	bytesPerSec int64
}

func newPacer(bytesPerSecond int64) *pacer {
	if bytesPerSecond <= 0 {
		return &pacer{}
	}
	// KeyedRateLimiter takes requests per minute; a "request" here is a byte, and the
	// burst is one second's worth. A bucket is looked up once per stream, so it must
	// outlive the longest stream: one swept while a stream still reads through it would
	// hand the workspace's next stream a full bucket.
	return &pacer{
		byWorkspace: core.NewKeyedRateLimiter[string](float64(bytesPerSecond)*60, int(bytesPerSecond), StreamDeadline+time.Minute, time.Minute),
		bytesPerSec: bytesPerSecond,
	}
}

func (p *pacer) limiter(workspaceID string) *rate.Limiter {
	if p == nil || p.byWorkspace == nil {
		return nil
	}
	return p.byWorkspace.Bucket(workspaceID)
}

// SetPacing replaces the S3 read and write rates (bytes per second, per workspace on
// this replica). Zero turns a direction's pacing off. Tests set small or large values.
func (s *Service) SetPacing(readBytesPerSecond, writeBytesPerSecond int64) {
	s.readPace, s.writePace = newPacer(readBytesPerSecond), newPacer(writeBytesPerSecond)
}

type pacedReader struct {
	ctx context.Context
	r   io.Reader
	lim *rate.Limiter
}

func (p *pacedReader) Read(b []byte) (int, error) {
	if len(b) > paceChunk {
		b = b[:paceChunk]
	}
	n, err := p.r.Read(b)
	if n > 0 {
		if werr := p.lim.WaitN(p.ctx, n); werr != nil && err == nil {
			err = werr
		}
	}
	return n, err
}

// PaceUpload wraps the body an upload reads from the client, so it is taken at the
// workspace's write rate.
func (s *Service) PaceUpload(ctx context.Context, workspaceID string, r io.Reader) io.Reader {
	lim := s.writePace.limiter(workspaceID)
	if lim == nil {
		return r
	}
	return &pacedReader{ctx: ctx, r: r, lim: lim}
}

type pacedWriter struct {
	ctx context.Context
	w   io.Writer
	lim *rate.Limiter
}

func (p *pacedWriter) Write(b []byte) (int, error) {
	written := 0
	for len(b) > 0 {
		chunk := b
		if len(chunk) > paceChunk {
			chunk = chunk[:paceChunk]
		}
		if err := p.lim.WaitN(p.ctx, len(chunk)); err != nil {
			return written, err
		}
		n, err := p.w.Write(chunk)
		written += n
		if err != nil {
			return written, err
		}
		b = b[len(chunk):]
	}
	return written, nil
}

// PaceDownload wraps the writer an object is streamed to, so it leaves at the
// workspace's read rate.
func (s *Service) PaceDownload(ctx context.Context, workspaceID string, w io.Writer) io.Writer {
	lim := s.readPace.limiter(workspaceID)
	if lim == nil {
		return w
	}
	return &pacedWriter{ctx: ctx, w: w, lim: lim}
}
