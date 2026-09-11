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

package code

import (
	"io"
	"path/filepath"
	"testing"
)

// TestExecClaudeEmitsBeforeReplacingTheProcess pins the ordering the whole
// launcher-native telemetry path depends on: once syscall.Exec succeeds no bex
// code runs again, so the hook must fire first. A deliberately unusable binary
// path makes Exec fail, which is the only way to observe both sides.
func TestExecClaudeEmitsBeforeReplacingTheProcess(t *testing.T) {
	calls := 0
	err := execClaude(filepath.Join(t.TempDir(), "definitely-not-a-binary"), nil, nil, func() { calls++ })
	if err == nil {
		t.Fatal("exec of a nonexistent binary succeeded")
	}
	if calls != 1 {
		t.Fatalf("beforeExec ran %d times, want exactly once before the exec", calls)
	}
}

// TestLaunchDoesNotEmitWhenClaudeIsMissing is the no-double-count half: when the
// launch fails early the process is never replaced, so Cobra completes normally
// and upstream's own post-run hook reports the execution. Emitting here too
// would count that invocation twice.
func TestLaunchDoesNotEmitWhenClaudeIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	calls := 0
	err := launch(Provider{Name: "glm"}, nil, io.Discard, func() { calls++ })
	if err == nil {
		t.Fatal("launch succeeded with no claude on PATH")
	}
	if calls != 0 {
		t.Fatalf("beforeExec ran %d times, want none when the process is never replaced", calls)
	}
}
