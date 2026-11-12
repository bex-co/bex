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

package logs

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// buildTail is one live type=build tail over a build pod whose output the
// test writes line by line, and a deploy row the test moves through its
// statuses the way the reconciler does.
type buildTail struct {
	t   *testing.T
	out *io.PipeWriter
	mu  sync.Mutex
	row DeployProgress
	// newer is a deploy created after this one, listed before it as the
	// source lists rows (newest first).
	newer *DeployProgress
	got   chan string
	done  chan error
}

const (
	buildingFrom = "==> Building from https://github.com/x/y.git@abc1234"
	buildFailed  = "==> Build failed"
)

func startBuildTail(t *testing.T) *buildTail {
	t.Helper()
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	pod := buildPodFor("web", "bld-web-gen-3-a", "builds", "buildkit", time.Unix(3, 0))
	svc := newService(nil, app, pod)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 5 * time.Millisecond
	pr, pw := io.Pipe()
	svc.PodLogsFollow = func(context.Context, string, string, string, time.Time) (io.ReadCloser, error) {
		return pr, nil
	}
	b := &buildTail{
		t:   t,
		out: pw,
		row: DeployProgress{
			ID: "dep-3", Status: deployStatusQueued, Commit: "abc1234def5678",
			CreatedAt: time.Date(2026, 11, 12, 12, 58, 29, 0, time.UTC),
		},
		got:  make(chan string, 32),
		done: make(chan error, 1),
	}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.newer != nil {
			return []DeployProgress{*b.newer, b.row}, nil
		}
		return []DeployProgress{b.row}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	go func() {
		b.done <- svc.FollowLogs(ctx, LogQuery{App: "web", Types: []string{LogTypeBuild}}, func(e LogEntry) error {
			b.got <- e.Message
			return nil
		})
	}()
	b.expect("==> Build queued")
	return b
}

// update changes the deploy row, as a reconciler pass does.
func (b *buildTail) update(change func(*DeployProgress)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	change(&b.row)
}

// print writes one line of the build's own output.
func (b *buildTail) print(msg string) {
	b.t.Helper()
	if _, err := fmt.Fprintf(b.out, "%s %s\n", time.Now().UTC().Format(time.RFC3339Nano), msg); err != nil {
		b.t.Fatalf("write build output: %v", err)
	}
}

func (b *buildTail) expect(want string) {
	b.t.Helper()
	select {
	case got := <-b.got:
		if got != want {
			b.t.Fatalf("next line = %q, want %q", got, want)
		}
	case err := <-b.done:
		b.t.Fatalf("tail ended (%v) before %q", err, want)
	case <-time.After(5 * time.Second):
		b.t.Fatalf("no %q within 5s", want)
	}
}

// quiet asserts that nothing arrives and the tail stays open for many polls.
func (b *buildTail) quiet() {
	b.t.Helper()
	select {
	case got := <-b.got:
		b.t.Fatalf("unexpected line %q", got)
	case err := <-b.done:
		b.t.Fatalf("tail ended early: %v", err)
	case <-time.After(60 * time.Millisecond):
	}
}

func (b *buildTail) ends() {
	b.t.Helper()
	select {
	case err := <-b.done:
		if err != nil {
			b.t.Fatalf("tail ended with %v, want a clean end", err)
		}
	case got := <-b.got:
		b.t.Fatalf("unexpected line %q, want the tail to end", got)
	case <-time.After(5 * time.Second):
		b.t.Fatal("the tail did not end")
	}
}

// w8/m159 t009: production's tail printed `==> Building from` after `==> Build
// successful`, because the narration was read only before the pod and after
// its streams ended, and a failing build's stream ended 16 s before its row
// closed, so `==> Build failed` never arrived. The live order must be
// history's: the start line when the row records the start, the verdict last.
func TestBuildTailNarratesInHistorysOrder(t *testing.T) {
	b := startBuildTail(t)
	b.print("==> Using Node 24")
	b.expect("==> Using Node 24")
	b.update(func(d *DeployProgress) {
		d.Status = deployStatusBuildInProgress
		d.StartedAt = d.CreatedAt.Add(29 * time.Second)
	})
	b.expect(buildingFrom)
	b.print("==> Running build command 'yarn install && exit 3'...")
	b.expect("==> Running build command 'yarn install && exit 3'...")
	b.print("error: build command exited with code 3")
	b.expect("error: build command exited with code 3")
	if err := b.out.Close(); err != nil {
		t.Fatal(err)
	}
	b.quiet() // the build's output is over, and its row has not closed yet
	b.update(func(d *DeployProgress) {
		d.Status = "build_failed"
		d.Built = true
		d.FinishedAt = d.CreatedAt.Add(90 * time.Second)
	})
	b.expect(buildFailed)
	b.ends()
}

// A row first seen closed while the build's output is still arriving (one
// reconciler pass recorded its start and its failure) gets its start line at
// once and keeps its verdict until the last line.
func TestBuildTailHoldsTheVerdictUntilTheBuildsLastLine(t *testing.T) {
	b := startBuildTail(t)
	b.print("==> Using Node 24")
	b.expect("==> Using Node 24")
	b.update(func(d *DeployProgress) {
		d.Status = "build_failed"
		d.Built = true
		d.StartedAt = d.CreatedAt.Add(time.Second)
		d.FinishedAt = d.CreatedAt.Add(time.Minute)
	})
	b.expect(buildingFrom)
	b.quiet()
	b.print("error: build command exited with code 3")
	b.expect("error: build command exited with code 3")
	if err := b.out.Close(); err != nil {
		t.Fatal(err)
	}
	b.expect(buildFailed)
	b.ends()
}

// A successful build's row moves on to its rollout: the build stream ends
// there, without waiting for the rollout's own closing line.
func TestBuildTailEndsWhenTheRowMovesOnToItsRollout(t *testing.T) {
	b := startBuildTail(t)
	b.update(func(d *DeployProgress) {
		d.Status = deployStatusBuildInProgress
		d.StartedAt = d.CreatedAt.Add(time.Second)
	})
	b.expect(buildingFrom)
	b.print("==> Build successful 🎉")
	b.expect("==> Build successful 🎉")
	if err := b.out.Close(); err != nil {
		t.Fatal(err)
	}
	b.quiet()
	b.update(func(d *DeployProgress) { d.Status = "update_in_progress" })
	b.ends()
}

// A deploy created while this build runs is not narrated on its stream; the
// tail still reads its own deploy's start and verdict.
func TestBuildTailFollowsItsOwnDeployPastANewerOne(t *testing.T) {
	b := startBuildTail(t)
	b.mu.Lock()
	b.newer = &DeployProgress{ID: "dep-4", Status: deployStatusQueued, CreatedAt: b.row.CreatedAt.Add(time.Second)}
	b.mu.Unlock()
	b.print("==> Using Node 24")
	b.expect("==> Using Node 24")
	b.update(func(d *DeployProgress) {
		d.Status = deployStatusBuildInProgress
		d.StartedAt = d.CreatedAt.Add(2 * time.Second)
	})
	b.expect(buildingFrom)
	if err := b.out.Close(); err != nil {
		t.Fatal(err)
	}
	b.quiet()
	b.update(func(d *DeployProgress) {
		d.Status = "build_failed"
		d.Built = true
		d.FinishedAt = d.CreatedAt.Add(time.Minute)
	})
	b.expect(buildFailed)
	b.ends()
}
