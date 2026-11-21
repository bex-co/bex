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

package s3

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/term"
)

// result is one thing a command did, or with --dryrun would do.
type result struct {
	Action string `json:"action"`
	// Target is what a single-sided action acted on: a deleted object, a made
	// or removed bucket.
	Target      string `json:"target,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
	DryRun      bool   `json:"dryRun,omitempty"`
}

// line is the result as one line of text: `upload: ./a to s3://b/a`,
// `delete: s3://b/k`, `make_bucket: photos`.
func (r result) line() string {
	text := r.Action + ": " + r.Target
	if r.Source != "" || r.Destination != "" {
		text = r.Action + ": " + r.Source + " to " + r.Destination
	}
	if r.DryRun {
		return "(dryrun) " + text
	}
	return text
}

// reporter prints results as they happen (text) or collects them for one
// document at the end (JSON and YAML).
type reporter struct {
	mu      sync.Mutex
	out     io.Writer
	format  format
	prog    *progress
	results []result
}

func (r *reporter) add(res result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.format != formatText {
		r.results = append(r.results, res)
		return
	}
	r.prog.clear()
	_, _ = fmt.Fprintln(r.out, res.line())
}

// finish prints the collected document: every result, in the order they
// finished, under "results".
func (r *reporter) finish() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.format == formatText {
		return nil
	}
	results := r.results
	if results == nil {
		results = []result{}
	}
	return writeValue(r.out, r.format, map[string]any{"results": results})
}

// progress is the status line of a transfer: bytes done of bytes known, the
// rate, and the files still to go. It goes to stderr and only to a terminal,
// so a pipe or a log never holds it.
type progress struct {
	w       io.Writer
	enabled bool
	started time.Time
	done    atomic.Int64
	total   atomic.Int64
	files   atomic.Int64 // files not finished yet
	stopped chan struct{}
	wg      sync.WaitGroup
	shown   atomic.Bool
}

func newProgress(w io.Writer, enabled bool) *progress {
	return &progress{w: w, enabled: enabled, started: time.Now(), stopped: make(chan struct{})}
}

// expect adds a file of size bytes to what is to be done.
func (p *progress) expect(size int64) {
	if p == nil {
		return
	}
	p.total.Add(size)
	p.files.Add(1)
}

// advance records n more bytes moved.
func (p *progress) advance(n int64) {
	if p != nil {
		p.done.Add(n)
	}
}

// finished records a file as done.
func (p *progress) finished() {
	if p != nil {
		p.files.Add(-1)
	}
}

func (p *progress) start() {
	if p == nil || !p.enabled {
		return
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		tick := time.NewTicker(250 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-p.stopped:
				return
			case <-tick.C:
				p.draw()
			}
		}
	}()
}

func (p *progress) draw() {
	done, total := min(p.done.Load(), p.total.Load()), p.total.Load()
	rate := float64(done) / max(time.Since(p.started).Seconds(), 0.001)
	_, _ = fmt.Fprintf(p.w, "\r\x1b[2KCompleted %s/%s (%s/s) with %d file(s) remaining", humanBytes(done), humanBytes(total), humanBytes(int64(rate)), p.files.Load())
	p.shown.Store(true)
}

// clear wipes the status line so a result line can print over it.
func (p *progress) clear() {
	if p != nil && p.enabled && p.shown.CompareAndSwap(true, false) {
		_, _ = fmt.Fprint(p.w, "\r\x1b[2K")
	}
}

func (p *progress) stop() {
	if p == nil || !p.enabled {
		return
	}
	close(p.stopped)
	p.wg.Wait()
	p.clear()
}

// isTerminal reports whether w is a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
